// Package stage0 implements the LLM-assisted import pipeline: extracting
// structured career entries from pasted resume text and enriching them with
// review flags before a human approves anything into contributions.
package stage0

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/generation"
)

const (
	extractionMaxTokens = 4096
	enrichmentMaxTokens = 1024
)

// Service owns the Stage 0 lifecycle: extraction, enrichment, and the status
// transitions of import_batches / contribution_drafts. It does not write to
// contributions directly — that only happens on human-approved draft approval.
type Service struct {
	pool   *pgxpool.Pool
	q      *db.Queries
	client *generation.Client
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

type draftInput struct {
	EmployerName    string  `json:"employer_name"`
	PositionTitle   string  `json:"position_title"`
	Summary         *string `json:"summary"`
	FullDescription *string `json:"full_description"`
	Outcomes        *string `json:"outcomes"`
	ScaleContext    *string `json:"scale_context"`
}

type enrichmentResult struct {
	Flags []json.RawMessage `json:"flags"`
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

// RunEnrichment runs Stage 0b for a single draft. Failures are returned to the
// caller to log — a missing flags column is acceptable, a failed batch is not.
func (s *Service) RunEnrichment(ctx context.Context, draft db.ContributionDraft) error {
	prompt, err := generation.RawPrompt("stage0b_enrichment.txt")
	if err != nil {
		return fmt.Errorf("load enrichment prompt: %w", err)
	}

	input, err := json.Marshal(draftInput{
		EmployerName:    draft.EmployerName,
		PositionTitle:   draft.PositionTitle,
		Summary:         draft.Summary,
		FullDescription: draft.FullDescription,
		Outcomes:        draft.Outcomes,
		ScaleContext:    draft.ScaleContext,
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

	flagsJSON, err := json.Marshal(result.Flags)
	if err != nil {
		return fmt.Errorf("marshal flags: %w", err)
	}
	flagsRaw := json.RawMessage(flagsJSON)

	if _, err := s.q.UpdateContributionDraft(ctx, db.UpdateContributionDraftParams{
		ID:              draft.ID,
		UserID:          draft.UserID,
		Summary:         draft.Summary,
		FullDescription: draft.FullDescription,
		Outcomes:        draft.Outcomes,
		ScaleContext:    draft.ScaleContext,
		Flags:           &flagsRaw,
	}); err != nil {
		return fmt.Errorf("update draft flags: %w", err)
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
