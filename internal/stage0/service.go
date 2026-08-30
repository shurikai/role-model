// Package stage0 implements the LLM-assisted import pipeline: extracting
// structured career entries from pasted resume text and enriching them with
// review flags before a human approves anything into contributions.
package stage0

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/generation"
)

const (
	extractionMaxTokens = 4096
	// Stage 0b now returns tag suggestions alongside flags. A response truncated
	// at the token cap is invalid JSON and loses both, so this carries more
	// headroom than the flags-only pass needed.
	enrichmentMaxTokens = 2048
)

var (
	ErrDraftNotPending  = errors.New("draft is not pending")
	ErrDraftIncomplete  = errors.New("draft summary and full_description must be set before approval")
	ErrPositionNotFound = errors.New("position not found")
	ErrTagNotFound      = errors.New("tag not found")
)

// completer is the slice of generation.Client this package calls. It exists so
// RunEnrichment can be exercised against a canned response without a live LLM;
// *generation.Client satisfies it.
type completer interface {
	Complete(ctx context.Context, systemPrompt, userContent string, maxTokens int64) (string, error)
}

// Service owns the Stage 0 lifecycle: extraction, enrichment, and the status
// transitions of import_batches / contribution_drafts. It does not write to
// contributions directly — that only happens on human-approved draft approval.
type Service struct {
	pool   *pgxpool.Pool
	q      *db.Queries
	client completer
}

func NewService(pool *pgxpool.Pool, q *db.Queries, client *generation.Client) *Service {
	return &Service{pool: pool, q: q, client: client}
}

type extractedEntry struct {
	EmployerName    string  `json:"employer_name"`
	PositionTitle   string  `json:"position_title"`
	Summary         *string `json:"summary"`
	FullDescription *string `json:"full_description"`
	Outcomes        *string `json:"outcomes"`
	ScaleContext    *string `json:"scale_context"`
}

// enrichmentTagOption is one row of the user's existing tag vocabulary as the
// enrichment prompt sees it. The id is withheld deliberately: the model copies
// names, and resolveSuggestedTags maps them back to real rows in Go.
type enrichmentTagOption struct {
	Name     string `json:"name"`
	Category string `json:"category"`
}

type enrichmentInput struct {
	EmployerName    string                `json:"employer_name"`
	PositionTitle   string                `json:"position_title"`
	Summary         *string               `json:"summary"`
	FullDescription *string               `json:"full_description"`
	Outcomes        *string               `json:"outcomes"`
	ScaleContext    *string               `json:"scale_context"`
	AvailableTags   []enrichmentTagOption `json:"available_tags"`
}

type enrichmentResult struct {
	Flags         []json.RawMessage `json:"flags"`
	SuggestedTags []string          `json:"suggested_tags"`
}

// suggestedTag is one resolved tag suggestion as persisted on
// contribution_drafts.suggested_tags. Denormalised: the review screen renders
// name and category directly, and the narrow import path has no tag fetch of
// its own.
type suggestedTag struct {
	TagID    uuid.UUID `json:"tag_id"`
	Name     string    `json:"name"`
	Category string    `json:"category"`
}

// resolveSuggestedTags maps model-returned tag names to the user's real tag
// rows. Matching is exact and case-insensitive on a trimmed name — no alias or
// fuzzy match, because the model is handed the canonical names to copy. A name
// that resolves to nothing is dropped (Stage 0b never invents a tag); a name
// that resolves to an already-picked tag is dropped. Order follows the model's.
// The result is always non-nil so it marshals to [] rather than null.
func resolveSuggestedTags(available []db.Tag, names []string) []suggestedTag {
	byName := make(map[string]db.Tag, len(available))
	for _, t := range available {
		byName[strings.ToLower(strings.TrimSpace(t.Name))] = t
	}

	out := make([]suggestedTag, 0, len(names))
	seen := make(map[uuid.UUID]bool, len(names))
	for _, n := range names {
		t, ok := byName[strings.ToLower(strings.TrimSpace(n))]
		if !ok || seen[t.ID] {
			continue
		}
		seen[t.ID] = true
		out = append(out, suggestedTag{TagID: t.ID, Name: t.Name, Category: t.Category})
	}
	return out
}

// RunExtraction drives Stage 0a (extraction) followed by Stage 0b (per-draft
// enrichment) for a pending batch. It runs synchronously.
func (s *Service) RunExtraction(ctx context.Context, batchID, userID uuid.UUID) error {
	batch, err := s.q.GetImportBatch(ctx, db.GetImportBatchParams{ID: batchID, UserID: userID})
	if err != nil {
		return fmt.Errorf("run extraction: get batch: %w", err)
	}
	if batch.Status != "pending" {
		return fmt.Errorf("run extraction: batch %s is not pending (status=%s)", batchID, batch.Status)
	}

	if _, err := s.setStatus(ctx, batchID, userID, "extracting", nil); err != nil {
		return fmt.Errorf("run extraction: set extracting: %w", err)
	}

	prompt, err := generation.RawPrompt("stage0a_extraction.txt")
	if err != nil {
		return s.fail(ctx, batchID, userID, fmt.Errorf("load extraction prompt: %w", err))
	}

	raw, err := s.client.Complete(ctx, prompt, batch.RawText, extractionMaxTokens)
	if err != nil {
		return s.fail(ctx, batchID, userID, fmt.Errorf("extraction call: %w", err))
	}

	var entries []extractedEntry
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		return s.fail(ctx, batchID, userID, fmt.Errorf("parse extraction response: %w (raw: %s)", err, raw))
	}

	drafts, err := s.insertDrafts(ctx, batchID, userID, entries)
	if err != nil {
		return s.fail(ctx, batchID, userID, fmt.Errorf("insert drafts: %w", err))
	}

	if _, err := s.setStatus(ctx, batchID, userID, "enriching", nil); err != nil {
		return fmt.Errorf("run extraction: set enriching: %w", err)
	}

	for _, d := range drafts {
		if err := s.RunEnrichment(ctx, d); err != nil {
			log.Printf("stage0: enrichment failed for draft %s: %v", d.ID, err)
		}
	}

	if _, err := s.setStatus(ctx, batchID, userID, "review", nil); err != nil {
		return fmt.Errorf("run extraction: set review: %w", err)
	}

	return nil
}

// insertDrafts inserts all extracted entries as contribution_drafts inside a
// single transaction, so a mid-loop failure leaves no partial drafts behind.
func (s *Service) insertDrafts(ctx context.Context, batchID, userID uuid.UUID, entries []extractedEntry) ([]db.ContributionDraft, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after a successful commit

	qtx := s.q.WithTx(tx)

	drafts := make([]db.ContributionDraft, 0, len(entries))
	for _, e := range entries {
		if e.EmployerName == "" || e.PositionTitle == "" {
			return nil, fmt.Errorf("extracted entry missing employer_name or position_title")
		}

		d, err := qtx.CreateContributionDraft(ctx, db.CreateContributionDraftParams{
			ID:              uuid.New(),
			UserID:          userID,
			BatchID:         batchID,
			EmployerName:    e.EmployerName,
			PositionTitle:   e.PositionTitle,
			Summary:         e.Summary,
			FullDescription: e.FullDescription,
			Outcomes:        e.Outcomes,
			ScaleContext:    e.ScaleContext,
		})
		if err != nil {
			return nil, fmt.Errorf("create draft: %w", err)
		}
		drafts = append(drafts, d)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return drafts, nil
}

// ApproveDraft verifies the target position belongs to userID, writes a new
// contribution from the draft's fields, links any tagIDs to it, and marks the
// draft approved. tagIDs is optional; every id must name a tag the user owns,
// and a bad one aborts before anything is written.
func (s *Service) ApproveDraft(ctx context.Context, userID, draftID, positionID uuid.UUID, tagIDs []uuid.UUID) (db.Contribution, error) {
	draft, err := s.q.GetContributionDraft(ctx, db.GetContributionDraftParams{ID: draftID, UserID: userID})
	if err != nil {
		return db.Contribution{}, err
	}
	if draft.Status != "pending" {
		return db.Contribution{}, ErrDraftNotPending
	}
	if draft.Summary == nil || *draft.Summary == "" {
		return db.Contribution{}, ErrDraftIncomplete
	}
	if draft.FullDescription == nil || *draft.FullDescription == "" {
		return db.Contribution{}, ErrDraftIncomplete
	}

	// Parent-ownership check: the position must exist AND belong to this user.
	if _, err := s.q.GetPosition(ctx, db.GetPositionParams{
		ID:     positionID,
		UserID: userID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Contribution{}, ErrPositionNotFound
		}
		return db.Contribution{}, fmt.Errorf("approve draft: verify position: %w", err)
	}

	// Same double-ownership rule the tag CRUD path applies: every tag must exist
	// and belong to this user. Checked before the transaction opens so a bad id
	// is a clean 404 rather than a rolled-back write. Deduped so the assign loop
	// below runs once per tag.
	uniqueTagIDs := make([]uuid.UUID, 0, len(tagIDs))
	seenTag := make(map[uuid.UUID]bool, len(tagIDs))
	for _, id := range tagIDs {
		if seenTag[id] {
			continue
		}
		seenTag[id] = true
		if _, err := s.q.GetTag(ctx, db.GetTagParams{ID: id, UserID: userID}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return db.Contribution{}, ErrTagNotFound
			}
			return db.Contribution{}, fmt.Errorf("approve draft: verify tag: %w", err)
		}
		uniqueTagIDs = append(uniqueTagIDs, id)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return db.Contribution{}, fmt.Errorf("approve draft: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after a successful commit

	qtx := s.q.WithTx(tx)

	contribution, err := qtx.CreateContribution(ctx, db.CreateContributionParams{
		ID:              uuid.New(),
		UserID:          userID,
		PositionID:      &positionID,
		Summary:         *draft.Summary,
		FullDescription: *draft.FullDescription,
		Outcomes:        draft.Outcomes,
		ScaleContext:    draft.ScaleContext,
		IsActive:        true,
	})
	if err != nil {
		return db.Contribution{}, fmt.Errorf("approve draft: create contribution: %w", err)
	}

	if _, err := qtx.UpdateContributionDraftStatus(ctx, db.UpdateContributionDraftStatusParams{
		ID:     draftID,
		UserID: userID,
		Status: "approved",
	}); err != nil {
		return db.Contribution{}, fmt.Errorf("approve draft: update draft status: %w", err)
	}

	for _, id := range uniqueTagIDs {
		if err := qtx.AssignTagToContribution(ctx, db.AssignTagToContributionParams{
			ContributionID: contribution.ID,
			TagID:          id,
		}); err != nil {
			return db.Contribution{}, fmt.Errorf("approve draft: assign tag: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return db.Contribution{}, fmt.Errorf("approve draft: commit tx: %w", err)
	}

	return contribution, nil
}

// RunEnrichment runs Stage 0b for a single draft: review flags plus tag
// suggestions drawn from the user's existing vocabulary. Failures are returned
// to the caller to log — a draft without enrichment is acceptable, a failed
// batch is not.
func (s *Service) RunEnrichment(ctx context.Context, draft db.ContributionDraft) error {
	prompt, err := generation.RawPrompt("stage0b_enrichment.txt")
	if err != nil {
		return fmt.Errorf("load enrichment prompt: %w", err)
	}

	tags, err := s.q.ListTags(ctx, draft.UserID)
	if err != nil {
		return fmt.Errorf("list tags: %w", err)
	}
	options := make([]enrichmentTagOption, 0, len(tags))
	for _, t := range tags {
		options = append(options, enrichmentTagOption{Name: t.Name, Category: t.Category})
	}

	input, err := json.Marshal(enrichmentInput{
		EmployerName:    draft.EmployerName,
		PositionTitle:   draft.PositionTitle,
		Summary:         draft.Summary,
		FullDescription: draft.FullDescription,
		Outcomes:        draft.Outcomes,
		ScaleContext:    draft.ScaleContext,
		AvailableTags:   options,
	})
	if err != nil {
		return fmt.Errorf("marshal draft input: %w", err)
	}

	raw, err := s.client.Complete(ctx, prompt, string(input), enrichmentMaxTokens)
	if err != nil {
		return fmt.Errorf("enrichment call: %w", err)
	}

	var result enrichmentResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return fmt.Errorf("parse enrichment response: %w (raw: %s)", err, raw)
	}

	resolved := resolveSuggestedTags(tags, result.SuggestedTags)
	if len(resolved) < len(result.SuggestedTags) {
		log.Printf("stage0: enrichment draft %s: dropped %d unresolved tag name(s)",
			draft.ID, len(result.SuggestedTags)-len(resolved))
	}

	flagsJSON, err := json.Marshal(result.Flags)
	if err != nil {
		return fmt.Errorf("marshal flags: %w", err)
	}
	flagsRaw := json.RawMessage(flagsJSON)

	suggestedJSON, err := json.Marshal(resolved)
	if err != nil {
		return fmt.Errorf("marshal suggested tags: %w", err)
	}
	suggestedRaw := json.RawMessage(suggestedJSON)

	if _, err := s.q.UpdateContributionDraftEnrichment(ctx, db.UpdateContributionDraftEnrichmentParams{
		ID:            draft.ID,
		UserID:        draft.UserID,
		Flags:         &flagsRaw,
		SuggestedTags: &suggestedRaw,
	}); err != nil {
		return fmt.Errorf("update draft enrichment: %w", err)
	}

	return nil
}

func (s *Service) setStatus(ctx context.Context, batchID, userID uuid.UUID, status string, errorText *string) (db.ImportBatch, error) {
	return s.q.UpdateImportBatchStatus(ctx, db.UpdateImportBatchStatusParams{
		ID:        batchID,
		UserID:    userID,
		Status:    status,
		ErrorText: errorText,
	})
}

// fail marks the batch failed with the cause's message and returns the cause
// unchanged, so callers can propagate the original error.
func (s *Service) fail(ctx context.Context, batchID, userID uuid.UUID, cause error) error {
	msg := cause.Error()
	if _, err := s.setStatus(ctx, batchID, userID, "failed", &msg); err != nil {
		log.Printf("stage0: failed to mark batch %s failed: %v", batchID, err)
	}
	return cause
}
