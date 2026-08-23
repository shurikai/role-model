//go:build integration

package intake

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shurikai/role-model/internal/db"
)

// The whole point of Phase 8, exercised end to end: an account with NOTHING —
// no employers, no positions, no tags, no categories — takes a batch of drafts
// and comes out with a career. Before entity_drafts this was impossible;
// ApproveDraft required a position_id that already existed, so the import was
// unusable by every new user.
func TestResolveBatchBuildsACareerFromNothing(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL required for intake integration tests")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	q := db.New(pool)
	svc := NewService(pool, q)

	userID := uuid.New()
	if _, err := q.CreateUser(ctx, db.CreateUserParams{
		ID: userID, Email: "intake-" + userID.String() + "@test.local",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	batch, err := q.CreateImportBatch(ctx, db.CreateImportBatchParams{
		ID: uuid.New(), UserID: userID, RawText: "pasted career text", Status: "review",
	})
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}

	// A clinician's career, drafted child-first on purpose: the input order is
	// whatever the extractor emitted, and depending on it is the bug.
	employerID, positionID := uuid.New(), uuid.New()
	contributionID, skillID := uuid.New(), uuid.New()

	mk := func(id uuid.UUID, kind string, payload any, deps ...uuid.UUID) {
		t.Helper()
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal %s payload: %v", kind, err)
		}
		msg := json.RawMessage(raw)
		if deps == nil {
			deps = []uuid.UUID{}
		}
		if _, err := q.CreateEntityDraft(ctx, db.CreateEntityDraftParams{
			ID: id, UserID: userID, BatchID: batch.ID, Kind: kind,
			Payload: &msg, DependsOn: deps, Status: "pending",
		}); err != nil {
			t.Fatalf("create %s draft: %v", kind, err)
		}
	}

	mk(skillID, KindSkill, map[string]any{
		"category": "Clinical", "tag": "ACLS", "proficiency": "expert", "years_experience": 8.0,
	})
	mk(contributionID, KindContribution, map[string]any{
		"position_draft":   positionID,
		"summary":          "Rebuilt referral intake across six sites.",
		"full_description": "Cut average wait from three weeks to four days by rerouting triage.",
		"tags":             []map[string]string{{"category": "Clinical", "name": "Triage"}},
	}, positionID)
	mk(positionID, KindPosition, map[string]any{
		"employer_draft": employerID,
		"title":          "Charge Nurse",
		"industry_level": "senior",
		"started_on":     "2019-03",
	}, employerID)
	mk(employerID, KindEmployer, map[string]any{
		"name": "County Health Network", "industry": "public health",
	})

	result, err := svc.ResolveBatch(ctx, userID, batch.ID)
	if err != nil {
		t.Fatalf("resolve batch: %v (unresolved: %v)", err, result.Unresolved)
	}
	if len(result.Resolved) != 4 {
		t.Fatalf("resolved %d drafts, want 4: %v", len(result.Resolved), result.Resolved)
	}

	// The chain the resolve-or-create service exists for: a skill needed a tag,
	// the tag needed a category, and neither existed.
	cats, err := q.ListTagCategories(ctx, userID)
	if err != nil {
		t.Fatalf("list categories: %v", err)
	}
	if len(cats) != 1 || cats[0].Name != "Clinical" {
		t.Fatalf("categories = %v, want exactly one named Clinical", cats)
	}

	tags, err := q.ListTags(ctx, userID)
	if err != nil {
		t.Fatalf("list tags: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("tags = %v, want ACLS and Triage", tags)
	}

	// The position really is attached to the employer the draft named by id,
	// not to a second employer created along the way.
	employers, err := q.GetEmployers(ctx, userID)
	if err != nil {
		t.Fatalf("list employers: %v", err)
	}
	if len(employers) != 1 {
		t.Fatalf("employers = %v, want exactly one", employers)
	}
	positions, err := q.GetPositionsByEmployer(ctx, db.GetPositionsByEmployerParams{
		EmployerID: employers[0].ID, UserID: userID,
	})
	if err != nil {
		t.Fatalf("list positions: %v", err)
	}
	if len(positions) != 1 || positions[0].Title != "Charge Nurse" {
		t.Fatalf("positions = %v, want one Charge Nurse under the employer", positions)
	}
}

// A batch that fails partway must leave nothing behind. A half-resolved import
// is the worst outcome: the employer exists, the positions do not, and the
// drafts that named them are marked approved with no row behind them.
func TestResolveBatchIsAllOrNothing(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL required for intake integration tests")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	q := db.New(pool)
	svc := NewService(pool, q)

	userID := uuid.New()
	if _, err := q.CreateUser(ctx, db.CreateUserParams{
		ID: userID, Email: "intake-fail-" + userID.String() + "@test.local",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	batch, err := q.CreateImportBatch(ctx, db.CreateImportBatchParams{
		ID: uuid.New(), UserID: userID, RawText: "x", Status: "review",
	})
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}

	good, bad := uuid.New(), uuid.New()
	mk := func(id uuid.UUID, kind string, payload any) {
		raw, _ := json.Marshal(payload)
		msg := json.RawMessage(raw)
		if _, err := q.CreateEntityDraft(ctx, db.CreateEntityDraftParams{
			ID: id, UserID: userID, BatchID: batch.ID, Kind: kind,
			Payload: &msg, DependsOn: []uuid.UUID{}, Status: "pending",
		}); err != nil {
			t.Fatalf("create draft: %v", err)
		}
	}
	mk(good, KindEmployer, map[string]any{"name": "Valid Employer"})
	// A position with no employer_draft: resolvable in order, invalid in
	// content, so the failure happens after the employer has been written.
	mk(bad, KindPosition, map[string]any{"title": "Orphan", "started_on": "2020-01"})

	if _, err := svc.ResolveBatch(ctx, userID, batch.ID); err == nil {
		t.Fatal("a batch with an invalid draft resolved successfully")
	}

	employers, err := q.GetEmployers(ctx, userID)
	if err != nil {
		t.Fatalf("list employers: %v", err)
	}
	if len(employers) != 0 {
		t.Errorf("employers = %v, want none — the failed batch left a row behind", employers)
	}

	drafts, err := q.ListEntityDraftsByBatch(ctx, db.ListEntityDraftsByBatchParams{
		BatchID: batch.ID, UserID: userID,
	})
	if err != nil {
		t.Fatalf("list drafts: %v", err)
	}
	for _, d := range drafts {
		if d.Status != "pending" {
			t.Errorf("draft %s is %q after a failed batch; it should still be pending", d.ID, d.Status)
		}
	}
}

// The whole loop, from a canned extraction to rows: plan, stage, resolve. This
// is what makes entity_drafts a path rather than a table — Phase 8 shipped the
// substrate and the resolver, and nothing populated it but a test.
func TestPlanStageResolveRoundTrip(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL required for intake integration tests")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	q := db.New(pool)
	svc := NewService(pool, q)

	userID := uuid.New()
	if _, err := q.CreateUser(ctx, db.CreateUserParams{
		ID: userID, Email: "roundtrip-" + userID.String() + "@test.local",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	batch, err := q.CreateImportBatch(ctx, db.CreateImportBatchParams{
		ID: uuid.New(), UserID: userID, RawText: "pasted career text", Status: "review",
	})
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}

	var x CareerExtraction
	if err := json.Unmarshal([]byte(clinicalExtraction), &x); err != nil {
		t.Fatalf("parse canned extraction: %v", err)
	}
	planned, err := PlanDrafts(x)
	if err != nil {
		t.Fatalf("PlanDrafts: %v", err)
	}
	if _, err := svc.StageDrafts(ctx, userID, batch.ID, planned); err != nil {
		t.Fatalf("StageDrafts: %v", err)
	}

	result, err := svc.ResolveBatch(ctx, userID, batch.ID)
	if err != nil {
		t.Fatalf("ResolveBatch: %v (unresolved: %v)", err, result.Unresolved)
	}
	if len(result.Resolved) != len(planned) {
		t.Fatalf("resolved %d of %d drafts", len(result.Resolved), len(planned))
	}

	// An account that had nothing now has a career, and none of it is software.
	employers, _ := q.GetEmployers(ctx, userID)
	if len(employers) != 2 {
		t.Errorf("employers = %d, want 2", len(employers))
	}
	cats, _ := q.ListTagCategories(ctx, userID)
	if len(cats) != 3 {
		t.Errorf("categories = %v, want Clinical, Certifications, Charting Systems", cats)
	}
	skills, _ := q.ListActiveSkillsByUser(ctx, userID)
	if len(skills) != 2 {
		t.Errorf("skills = %d, want 2", len(skills))
	}
}
