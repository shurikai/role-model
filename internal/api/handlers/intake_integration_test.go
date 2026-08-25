//go:build integration

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/httputil"
	"github.com/shurikai/role-model/internal/intake"
)

// These run against a real database because what they are testing is a
// database guard. The pending-only WHERE clause on MarkEntityDraftRejected and
// UpdateEntityDraftPayload cannot be exercised by a fake: the whole point of
// putting the check in SQL rather than in Go is that it holds when a resolve
// lands between a read and a write.

type intakeFixture struct {
	t       *testing.T
	ctx     context.Context
	pool    *pgxpool.Pool
	q       *db.Queries
	handler *IntakeHandler
	userID  uuid.UUID
	batchID uuid.UUID
}

func newIntakeFixture(t *testing.T, extractor intake.Extractor) *intakeFixture {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL required for intake handler integration tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)

	q := db.New(pool)
	userID := uuid.New()
	if _, err := q.CreateUser(ctx, db.CreateUserParams{
		ID: userID, Email: "intake-handler-" + userID.String() + "@test.local",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	batch, err := q.CreateImportBatch(ctx, db.CreateImportBatchParams{
		ID: uuid.New(), UserID: userID, RawText: "pasted career text", Status: "review",
	})
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}

	return &intakeFixture{
		t: t, ctx: ctx, pool: pool, q: q,
		handler: NewIntakeHandler(q, intake.NewService(pool, q), extractor),
		userID:  userID, batchID: batch.ID,
	}
}

// draft stages one entity draft in whatever status the test needs.
func (f *intakeFixture) draft(kind, status string, payload any, deps ...uuid.UUID) db.EntityDraft {
	f.t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		f.t.Fatalf("marshal payload: %v", err)
	}
	msg := json.RawMessage(raw)
	// depends_on is NOT NULL, so a draft with no parents carries an empty
	// array rather than nil. Worth knowing on the read side too: a client
	// never sees null here.
	if deps == nil {
		deps = []uuid.UUID{}
	}
	row, err := f.q.CreateEntityDraft(f.ctx, db.CreateEntityDraftParams{
		ID: uuid.New(), UserID: f.userID, BatchID: f.batchID, Kind: kind,
		Payload: &msg, DependsOn: deps, Status: status,
	})
	if err != nil {
		f.t.Fatalf("create %s draft: %v", kind, err)
	}
	return row
}

// call routes one request through chi so {draftID} resolves the way it does in
// the real router, with the user already in context as RequireAuth leaves it.
func (f *intakeFixture) call(method, pattern, path string, body any, h http.HandlerFunc) *httptest.ResponseRecorder {
	f.t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			f.t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}

	r := chi.NewRouter()
	r.Method(method, pattern, h)

	req := httptest.NewRequest(method, path, reader)
	req = req.WithContext(httputil.WithUserID(req.Context(), f.userID))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func (f *intakeFixture) statusOf(id uuid.UUID) string {
	f.t.Helper()
	row, err := f.q.GetEntityDraft(f.ctx, db.GetEntityDraftParams{ID: id, UserID: f.userID})
	if err != nil {
		f.t.Fatalf("reload draft: %v", err)
	}
	return row.Status
}

func TestRejectEntityDraft(t *testing.T) {
	f := newIntakeFixture(t, nil)

	t.Run("a pending draft is rejected", func(t *testing.T) {
		d := f.draft(intake.KindEmployer, "pending", map[string]any{"name": "Nimbus Health"})
		rec := f.call(http.MethodPost, "/import/entities/{draftID}/reject",
			"/import/entities/"+d.ID.String()+"/reject", nil, f.handler.RejectEntityDraft)

		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body)
		}
		if got := f.statusOf(d.ID); got != "rejected" {
			t.Fatalf("want status rejected, got %q", got)
		}
	})

	// The regression test for the pre-existing gap. Before the WHERE clause
	// carried `AND status = 'pending'`, this rejected a draft that had already
	// become a row — the entity_drafts row said rejected while the employer it
	// created sat in the account, and nothing reported the disagreement.
	t.Run("an approved draft cannot be rejected", func(t *testing.T) {
		d := f.draft(intake.KindEmployer, "pending", map[string]any{"name": "Already Real"})
		rowID := uuid.New()
		if _, err := f.q.MarkEntityDraftResolved(f.ctx, db.MarkEntityDraftResolvedParams{
			ID: d.ID, UserID: f.userID, ResolvedID: &rowID,
		}); err != nil {
			t.Fatalf("mark resolved: %v", err)
		}

		rec := f.call(http.MethodPost, "/import/entities/{draftID}/reject",
			"/import/entities/"+d.ID.String()+"/reject", nil, f.handler.RejectEntityDraft)

		if rec.Code != http.StatusConflict {
			t.Fatalf("want 409, got %d: %s", rec.Code, rec.Body)
		}
		if got := f.statusOf(d.ID); got != "approved" {
			t.Fatalf("the draft must still be approved, got %q", got)
		}
	})

	t.Run("the query itself refuses a non-pending draft", func(t *testing.T) {
		// Directly, without the handler's read-first check — this is the half
		// that holds when the status changes underneath a request.
		d := f.draft(intake.KindEmployer, "rejected", map[string]any{"name": "Already Rejected"})
		if _, err := f.q.MarkEntityDraftRejected(f.ctx, db.MarkEntityDraftRejectedParams{
			ID: d.ID, UserID: f.userID,
		}); err == nil {
			t.Fatal("expected the guarded query to match no rows")
		}
	})

	t.Run("someone else's draft is not found", func(t *testing.T) {
		rec := f.call(http.MethodPost, "/import/entities/{draftID}/reject",
			"/import/entities/"+uuid.New().String()+"/reject", nil, f.handler.RejectEntityDraft)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("want 404, got %d: %s", rec.Code, rec.Body)
		}
	})
}

func TestUpdateEntityDraftPayload(t *testing.T) {
	f := newIntakeFixture(t, nil)

	t.Run("a pending draft's payload is replaced", func(t *testing.T) {
		d := f.draft(intake.KindEmployer, "pending", map[string]any{"name": "Nimbus Helth"})
		rec := f.call(http.MethodPut, "/import/entities/{draftID}",
			"/import/entities/"+d.ID.String(),
			map[string]any{"name": "Nimbus Health", "industry": "healthcare", "notes": nil},
			f.handler.UpdateEntityDraftPayload)

		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body)
		}
		row, err := f.q.GetEntityDraft(f.ctx, db.GetEntityDraftParams{ID: d.ID, UserID: f.userID})
		if err != nil {
			t.Fatalf("reload: %v", err)
		}
		var got map[string]any
		if err := json.Unmarshal(*row.Payload, &got); err != nil {
			t.Fatalf("parse stored payload: %v", err)
		}
		if got["name"] != "Nimbus Health" || got["industry"] != "healthcare" {
			t.Fatalf("payload was not replaced: %v", got)
		}
	})

	t.Run("a payload that could not resolve is refused at edit time", func(t *testing.T) {
		d := f.draft(intake.KindEmployer, "pending", map[string]any{"name": "Nimbus Health"})
		rec := f.call(http.MethodPut, "/import/entities/{draftID}",
			"/import/entities/"+d.ID.String(),
			map[string]any{"name": ""},
			f.handler.UpdateEntityDraftPayload)

		// 422, not 400: the JSON parsed fine, it just cannot become a row.
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("want 422, got %d: %s", rec.Code, rec.Body)
		}
	})

	t.Run("a field the kind does not have is refused, not dropped", func(t *testing.T) {
		d := f.draft(intake.KindEmployer, "pending", map[string]any{"name": "Nimbus Health"})
		rec := f.call(http.MethodPut, "/import/entities/{draftID}",
			"/import/entities/"+d.ID.String(),
			map[string]any{"name": "Nimbus Health", "industy": "healthcare"},
			f.handler.UpdateEntityDraftPayload)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("want 422, got %d: %s", rec.Code, rec.Body)
		}
	})

	t.Run("an approved draft cannot be edited", func(t *testing.T) {
		d := f.draft(intake.KindEmployer, "pending", map[string]any{"name": "Already Real"})
		rowID := uuid.New()
		if _, err := f.q.MarkEntityDraftResolved(f.ctx, db.MarkEntityDraftResolvedParams{
			ID: d.ID, UserID: f.userID, ResolvedID: &rowID,
		}); err != nil {
			t.Fatalf("mark resolved: %v", err)
		}

		rec := f.call(http.MethodPut, "/import/entities/{draftID}",
			"/import/entities/"+d.ID.String(),
			map[string]any{"name": "Renamed After The Fact"},
			f.handler.UpdateEntityDraftPayload)

		if rec.Code != http.StatusConflict {
			t.Fatalf("want 409, got %d: %s", rec.Code, rec.Body)
		}
	})

	t.Run("a draft that does not exist is not found", func(t *testing.T) {
		rec := f.call(http.MethodPut, "/import/entities/{draftID}",
			"/import/entities/"+uuid.New().String(),
			map[string]any{"name": "Nobody"}, f.handler.UpdateEntityDraftPayload)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("want 404, got %d: %s", rec.Code, rec.Body)
		}
	})
}

func TestApproveEntityDraft(t *testing.T) {
	f := newIntakeFixture(t, nil)

	t.Run("a draft with no dependencies becomes a row", func(t *testing.T) {
		d := f.draft(intake.KindEmployer, "pending", map[string]any{"name": "Nimbus Health"})
		rec := f.call(http.MethodPost, "/import/entities/{draftID}/approve",
			"/import/entities/"+d.ID.String()+"/approve", nil, f.handler.ApproveEntityDraft)

		if rec.Code != http.StatusCreated {
			t.Fatalf("want 201, got %d: %s", rec.Code, rec.Body)
		}
		var body approveEntityDraftResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("parse response: %v", err)
		}
		employer, err := f.q.GetEmployer(f.ctx, db.GetEmployerParams{ID: body.ResolvedID, UserID: f.userID})
		if err != nil {
			t.Fatalf("the resolved id must name a real employer: %v", err)
		}
		if employer.Name != "Nimbus Health" {
			t.Fatalf("wrong employer created: %q", employer.Name)
		}
	})

	// The rule this endpoint exists to enforce. Approving the parent on the
	// reviewer's behalf would write a row they never looked at, which is the
	// one thing a review queue is for.
	t.Run("a draft whose parent is still pending is refused, and nothing is written", func(t *testing.T) {
		employer := f.draft(intake.KindEmployer, "pending", map[string]any{"name": "Unreviewed Employer"})
		position := f.draft(intake.KindPosition, "pending", map[string]any{
			"employer_draft": employer.ID, "title": "Staff Nurse", "started_on": "2019-04",
		}, employer.ID)

		rec := f.call(http.MethodPost, "/import/entities/{draftID}/approve",
			"/import/entities/"+position.ID.String()+"/approve", nil, f.handler.ApproveEntityDraft)

		if rec.Code != http.StatusConflict {
			t.Fatalf("want 409, got %d: %s", rec.Code, rec.Body)
		}
		// The message has to name the parent — "approve the parent first" is
		// useless without saying which card that is.
		if !bytes.Contains(rec.Body.Bytes(), []byte(employer.ID.String())) {
			t.Fatalf("the 409 must name the unresolved dependency, got: %s", rec.Body)
		}
		if got := f.statusOf(position.ID); got != "pending" {
			t.Fatalf("the position must still be pending, got %q", got)
		}
		if got := f.statusOf(employer.ID); got != "pending" {
			t.Fatalf("the employer must NOT have been auto-approved, got %q", got)
		}
	})

	t.Run("the same draft resolves once its parent has", func(t *testing.T) {
		employer := f.draft(intake.KindEmployer, "pending", map[string]any{"name": "Sequenced Health"})
		position := f.draft(intake.KindPosition, "pending", map[string]any{
			"employer_draft": employer.ID, "title": "Charge Nurse", "started_on": "2020-01",
		}, employer.ID)

		first := f.call(http.MethodPost, "/import/entities/{draftID}/approve",
			"/import/entities/"+employer.ID.String()+"/approve", nil, f.handler.ApproveEntityDraft)
		if first.Code != http.StatusCreated {
			t.Fatalf("approving the employer: want 201, got %d: %s", first.Code, first.Body)
		}

		second := f.call(http.MethodPost, "/import/entities/{draftID}/approve",
			"/import/entities/"+position.ID.String()+"/approve", nil, f.handler.ApproveEntityDraft)
		if second.Code != http.StatusCreated {
			t.Fatalf("approving the position: want 201, got %d: %s", second.Code, second.Body)
		}

		var body approveEntityDraftResponse
		if err := json.Unmarshal(second.Body.Bytes(), &body); err != nil {
			t.Fatalf("parse response: %v", err)
		}
		pos, err := f.q.GetPosition(f.ctx, db.GetPositionParams{ID: body.ResolvedID, UserID: f.userID})
		if err != nil {
			t.Fatalf("the resolved id must name a real position: %v", err)
		}
		// Attached to the employer the FIRST approval created, not to a
		// second copy of it.
		employerRow, err := f.q.GetEntityDraft(f.ctx, db.GetEntityDraftParams{ID: employer.ID, UserID: f.userID})
		if err != nil {
			t.Fatalf("reload employer draft: %v", err)
		}
		if employerRow.ResolvedID == nil || pos.EmployerID != *employerRow.ResolvedID {
			t.Fatalf("position is attached to %s, want %v", pos.EmployerID, employerRow.ResolvedID)
		}
	})

	t.Run("an already-approved draft is a conflict, not a second row", func(t *testing.T) {
		d := f.draft(intake.KindEmployer, "pending", map[string]any{"name": "Twice Approved"})
		if rec := f.call(http.MethodPost, "/import/entities/{draftID}/approve",
			"/import/entities/"+d.ID.String()+"/approve", nil, f.handler.ApproveEntityDraft); rec.Code != http.StatusCreated {
			t.Fatalf("first approve: want 201, got %d: %s", rec.Code, rec.Body)
		}
		rec := f.call(http.MethodPost, "/import/entities/{draftID}/approve",
			"/import/entities/"+d.ID.String()+"/approve", nil, f.handler.ApproveEntityDraft)
		if rec.Code != http.StatusConflict {
			t.Fatalf("second approve: want 409, got %d: %s", rec.Code, rec.Body)
		}
	})
}

// stubExtractor stands in for the model. The staging half of extraction is
// what this test is about; the model half is exercised by cmd/intakerun
// against a real API, because there is nothing to assert about a response that
// differs every time.
type stubExtractor struct {
	response string
	err      error
	calls    int
}

func (s *stubExtractor) Complete(context.Context, string, string, int64) (string, error) {
	s.calls++
	return s.response, s.err
}

func TestStartCareerImport(t *testing.T) {
	t.Run("stages a career and leaves the batch ready for review", func(t *testing.T) {
		extractor := &stubExtractor{response: `{
			"employers":[{"ref":"e1","name":"Nimbus Health","industry":"healthcare"}],
			"positions":[{"ref":"p1","employer_ref":"e1","title":"Staff Nurse","started_on":"2019-04"}],
			"contributions":[{"position_ref":"p1","summary":"Ran the floor","full_description":"Charge duties on a 24-bed unit.","tags":[]}],
			"skills":[{"category":"Clinical","tag":"ACLS","proficiency":"expert"}]
		}`}
		f := newIntakeFixture(t, extractor)

		rec := f.call(http.MethodPost, "/import/career", "/import/career",
			map[string]any{"raw_text": "a whole career, pasted"}, f.handler.StartCareerImport)

		if rec.Code != http.StatusCreated {
			t.Fatalf("want 201, got %d: %s", rec.Code, rec.Body)
		}
		if extractor.calls != 1 {
			t.Fatalf("expected exactly one model call, got %d", extractor.calls)
		}

		var body startCareerImportResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("parse response: %v", err)
		}
		if body.DraftCount != 4 {
			t.Fatalf("want 4 drafts, got %d (%v)", body.DraftCount, body.ByKind)
		}
		// 'review' is what the review screen tests for. Leaving it on
		// 'extracting' shows a finished import as still running, forever.
		if body.Status != "review" {
			t.Fatalf("want the batch left at review, got %q", body.Status)
		}

		drafts, err := f.q.ListEntityDraftsByBatch(f.ctx, db.ListEntityDraftsByBatchParams{
			BatchID: body.ID, UserID: f.userID,
		})
		if err != nil {
			t.Fatalf("list drafts: %v", err)
		}
		if len(drafts) != 4 {
			t.Fatalf("want 4 staged drafts, got %d", len(drafts))
		}
	})

	t.Run("a failed extraction keeps the batch and records why", func(t *testing.T) {
		f := newIntakeFixture(t, &stubExtractor{response: "not json at all"})

		rec := f.call(http.MethodPost, "/import/career", "/import/career",
			map[string]any{"raw_text": "a whole career, pasted"}, f.handler.StartCareerImport)

		if rec.Code != http.StatusBadGateway {
			t.Fatalf("want 502, got %d: %s", rec.Code, rec.Body)
		}
		var body startCareerImportResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("parse response: %v", err)
		}
		if body.Status != "failed" {
			t.Fatalf("want the batch marked failed, got %q", body.Status)
		}
		// The batch survives carrying the reason. Deleting it would leave the
		// person with a spinner that stopped and nothing to read.
		batch, err := f.q.GetImportBatch(f.ctx, db.GetImportBatchParams{ID: body.ID, UserID: f.userID})
		if err != nil {
			t.Fatalf("the failed batch must still exist: %v", err)
		}
		if batch.ErrorText == nil || *batch.ErrorText == "" {
			t.Fatal("the failed batch must carry an error_text")
		}
	})

	t.Run("empty text is refused before anything is created", func(t *testing.T) {
		extractor := &stubExtractor{response: "{}"}
		f := newIntakeFixture(t, extractor)

		rec := f.call(http.MethodPost, "/import/career", "/import/career",
			map[string]any{"raw_text": "   "}, f.handler.StartCareerImport)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("want 400, got %d: %s", rec.Code, rec.Body)
		}
		if extractor.calls != 0 {
			t.Fatal("an empty import must not reach the model")
		}
	})
}
