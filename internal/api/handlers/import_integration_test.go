//go:build integration

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/generation"
	"github.com/shurikai/role-model/internal/httputil"
	"github.com/shurikai/role-model/internal/stage0"
	"github.com/shurikai/role-model/internal/testenv"
)

// These exercise the tag_ids path on POST /import/drafts/{id}/approve against a
// real database: the ownership check, the dedupe, and that the links are
// written in the same transaction as the contribution.

type importFixture struct {
	t          *testing.T
	ctx        context.Context
	q          *db.Queries
	handler    *ImportHandler
	userID     uuid.UUID
	batchID    uuid.UUID
	positionID uuid.UUID
}

func newImportFixture(t *testing.T) *importFixture {
	t.Helper()
	dsn := testenv.DatabaseURL(t)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)

	q := db.New(pool)
	userID := uuid.New()
	if _, err := q.CreateUser(ctx, db.CreateUserParams{
		ID: userID, Email: "import-handler-" + userID.String() + "@test.local",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	employer, err := q.CreateEmployer(ctx, db.CreateEmployerParams{
		ID: uuid.New(), UserID: userID, Name: "Acme",
	})
	if err != nil {
		t.Fatalf("create employer: %v", err)
	}
	position, err := q.CreatePosition(ctx, db.CreatePositionParams{
		ID: uuid.New(), UserID: userID, EmployerID: employer.ID, Title: "Engineer",
		StartedOn: pgtype.Date{Time: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true},
	})
	if err != nil {
		t.Fatalf("create position: %v", err)
	}

	batch, err := q.CreateImportBatch(ctx, db.CreateImportBatchParams{
		ID: uuid.New(), UserID: userID, RawText: "pasted career text", Status: "review",
	})
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}

	// stage0.Service's LLM client is unused on the approve path.
	svc := stage0.NewService(pool, q, generation.NewClient(""))

	return &importFixture{
		t: t, ctx: ctx, q: q,
		handler:    NewImportHandler(q, svc),
		userID:     userID,
		batchID:    batch.ID,
		positionID: position.ID,
	}
}

// pendingDraft stages one approvable contribution_draft.
func (f *importFixture) pendingDraft() db.ContributionDraft {
	f.t.Helper()
	summary := "Decomposed a monolith into services."
	full := "Longer description of the same work."
	d, err := f.q.CreateContributionDraft(f.ctx, db.CreateContributionDraftParams{
		ID: uuid.New(), UserID: f.userID, BatchID: f.batchID,
		EmployerName: "Acme", PositionTitle: "Engineer",
		Summary: &summary, FullDescription: &full,
	})
	if err != nil {
		f.t.Fatalf("create draft: %v", err)
	}
	return d
}

// makeTag creates a tag (and its category) for a user.
func (f *importFixture) makeTag(userID uuid.UUID, category, name string) uuid.UUID {
	f.t.Helper()
	if _, err := f.q.CreateTagCategory(f.ctx, db.CreateTagCategoryParams{
		ID: uuid.New(), UserID: userID, Name: category, SortOrder: 0,
	}); err != nil {
		// A category is per-user unique; a second tag in the same category is fine.
		f.t.Logf("create category %q (may already exist): %v", category, err)
	}
	tag, err := f.q.CreateTag(f.ctx, db.CreateTagParams{
		ID: uuid.New(), UserID: userID, Name: name, Aliases: nil, Category: category, SortOrder: 0,
	})
	if err != nil {
		f.t.Fatalf("create tag %q: %v", name, err)
	}
	return tag.ID
}

func (f *importFixture) approve(draftID uuid.UUID, body any) *httptest.ResponseRecorder {
	f.t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		f.t.Fatalf("marshal body: %v", err)
	}
	r := chi.NewRouter()
	r.Post("/import/drafts/{draftID}/approve", f.handler.Approve)

	req := httptest.NewRequest(http.MethodPost,
		"/import/drafts/"+draftID.String()+"/approve", bytes.NewReader(raw))
	req = req.WithContext(httputil.WithUserID(req.Context(), f.userID))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func (f *importFixture) draftStatus(id uuid.UUID) string {
	f.t.Helper()
	row, err := f.q.GetContributionDraft(f.ctx, db.GetContributionDraftParams{ID: id, UserID: f.userID})
	if err != nil {
		f.t.Fatalf("reload draft: %v", err)
	}
	return row.Status
}

func (f *importFixture) contributionTagIDs(contributionID uuid.UUID) []uuid.UUID {
	f.t.Helper()
	rows, err := f.q.GetTagsByContribution(f.ctx, db.GetTagsByContributionParams{
		ContributionID: contributionID, UserID: f.userID,
	})
	if err != nil {
		f.t.Fatalf("get tags by contribution: %v", err)
	}
	out := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		out[i] = r.ID
	}
	return out
}

func contributionIDFrom(t *testing.T, rec *httptest.ResponseRecorder) uuid.UUID {
	t.Helper()
	var out struct {
		ContributionID uuid.UUID `json:"contribution_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode approve response: %v (%s)", err, rec.Body)
	}
	return out.ContributionID
}

func TestApproveDraftWithTagIDs(t *testing.T) {
	f := newImportFixture(t)
	d := f.pendingDraft()
	t1 := f.makeTag(f.userID, "Languages", "Go")
	t2 := f.makeTag(f.userID, "Protocols & Messaging", "Kafka")

	rec := f.approve(d.ID, map[string]any{
		"position_id": f.positionID,
		"tag_ids":     []uuid.UUID{t1, t2},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d: %s", rec.Code, rec.Body)
	}

	got := f.contributionTagIDs(contributionIDFrom(t, rec))
	if len(got) != 2 {
		t.Fatalf("want 2 tag links, got %d: %v", len(got), got)
	}
	seen := map[uuid.UUID]bool{got[0]: true, got[1]: true}
	if !seen[t1] || !seen[t2] {
		t.Fatalf("links %v do not cover %v / %v", got, t1, t2)
	}
	if s := f.draftStatus(d.ID); s != "approved" {
		t.Fatalf("draft status = %q, want approved", s)
	}
}

func TestApproveDraftUnknownTagRollsBack(t *testing.T) {
	f := newImportFixture(t)
	d := f.pendingDraft()

	rec := f.approve(d.ID, map[string]any{
		"position_id": f.positionID,
		"tag_ids":     []uuid.UUID{uuid.New()},
	})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d: %s", rec.Code, rec.Body)
	}
	var errBody struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &errBody)
	if errBody.Code != "tag_not_found" {
		t.Fatalf("want code tag_not_found, got %q (%s)", errBody.Code, rec.Body)
	}
	if s := f.draftStatus(d.ID); s != "pending" {
		t.Fatalf("draft status = %q, want pending (nothing should have been written)", s)
	}
	if rows, _ := f.q.GetContributionsByPosition(f.ctx, db.GetContributionsByPositionParams{
		PositionID: &f.positionID, UserID: f.userID,
	}); len(rows) != 0 {
		t.Fatalf("want no contribution written, got %d", len(rows))
	}
}

func TestApproveDraftForeignTag(t *testing.T) {
	f := newImportFixture(t)
	d := f.pendingDraft()

	other := uuid.New()
	if _, err := f.q.CreateUser(f.ctx, db.CreateUserParams{
		ID: other, Email: "other-" + other.String() + "@test.local",
	}); err != nil {
		t.Fatalf("create other user: %v", err)
	}
	foreignTag := f.makeTag(other, "Languages", "Rust")

	rec := f.approve(d.ID, map[string]any{
		"position_id": f.positionID,
		"tag_ids":     []uuid.UUID{foreignTag},
	})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d: %s", rec.Code, rec.Body)
	}
	if s := f.draftStatus(d.ID); s != "pending" {
		t.Fatalf("draft status = %q, want pending", s)
	}
}

func TestApproveDraftDuplicateTagIDs(t *testing.T) {
	f := newImportFixture(t)
	d := f.pendingDraft()
	t1 := f.makeTag(f.userID, "Languages", "Go")

	rec := f.approve(d.ID, map[string]any{
		"position_id": f.positionID,
		"tag_ids":     []uuid.UUID{t1, t1, t1},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d: %s", rec.Code, rec.Body)
	}
	got := f.contributionTagIDs(contributionIDFrom(t, rec))
	if len(got) != 1 || got[0] != t1 {
		t.Fatalf("want exactly one link to %v, got %v", t1, got)
	}
}

func TestApproveDraftNoTagIDs(t *testing.T) {
	f := newImportFixture(t)
	d := f.pendingDraft()

	rec := f.approve(d.ID, map[string]any{"position_id": f.positionID})
	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d: %s", rec.Code, rec.Body)
	}
	if got := f.contributionTagIDs(contributionIDFrom(t, rec)); len(got) != 0 {
		t.Fatalf("want no tag links, got %v", got)
	}
	if s := f.draftStatus(d.ID); s != "approved" {
		t.Fatalf("draft status = %q, want approved", s)
	}
}
