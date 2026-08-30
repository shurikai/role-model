//go:build integration

package stage0

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/testenv"
)

// fakeCompleter records the last user message and returns a canned response.
type fakeCompleter struct {
	response string
	lastUser string
}

func (f *fakeCompleter) Complete(_ context.Context, _, userContent string, _ int64) (string, error) {
	f.lastUser = userContent
	return f.response, nil
}

func TestRunEnrichmentSuggestsAndPersistsTags(t *testing.T) {
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
		ID: userID, Email: "enrich-" + userID.String() + "@test.local",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if _, err := q.CreateTagCategory(ctx, db.CreateTagCategoryParams{
		ID: uuid.New(), UserID: userID, Name: "Languages", SortOrder: 0,
	}); err != nil {
		t.Fatalf("create category: %v", err)
	}
	goTag, err := q.CreateTag(ctx, db.CreateTagParams{
		ID: uuid.New(), UserID: userID, Name: "Go", Category: "Languages", SortOrder: 0,
	})
	if err != nil {
		t.Fatalf("create tag: %v", err)
	}

	batch, err := q.CreateImportBatch(ctx, db.CreateImportBatchParams{
		ID: uuid.New(), UserID: userID, RawText: "text", Status: "review",
	})
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}
	summary := "Built services in Go."
	full := "More detail."
	draft, err := q.CreateContributionDraft(ctx, db.CreateContributionDraftParams{
		ID: uuid.New(), UserID: userID, BatchID: batch.ID,
		EmployerName: "Acme", PositionTitle: "Engineer",
		Summary: &summary, FullDescription: &full,
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}

	// The model returns one real tag and one it invented; the invented one is dropped.
	fc := &fakeCompleter{response: `{"flags":[{"type":"gap","field":"outcomes","message":"thin"}],"suggested_tags":["go","Rust"]}`}
	svc := &Service{pool: pool, q: q, client: fc}

	if err := svc.RunEnrichment(ctx, draft); err != nil {
		t.Fatalf("RunEnrichment: %v", err)
	}

	if !strings.Contains(fc.lastUser, `"available_tags"`) || !strings.Contains(fc.lastUser, `"Go"`) {
		t.Fatalf("prompt input did not carry the tag vocabulary: %s", fc.lastUser)
	}

	reloaded, err := q.GetContributionDraft(ctx, db.GetContributionDraftParams{ID: draft.ID, UserID: userID})
	if err != nil {
		t.Fatalf("reload draft: %v", err)
	}
	if reloaded.SuggestedTags == nil {
		t.Fatal("suggested_tags was not written")
	}
	var got []suggestedTag
	if err := json.Unmarshal(*reloaded.SuggestedTags, &got); err != nil {
		t.Fatalf("parse suggested_tags: %v", err)
	}
	if len(got) != 1 || got[0].TagID != goTag.ID || got[0].Name != "Go" || got[0].Category != "Languages" {
		t.Fatalf("want one resolved Go tag, got %+v", got)
	}
	if reloaded.Flags == nil {
		t.Fatal("flags should still be written alongside suggested_tags")
	}
}
