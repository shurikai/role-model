// Package contribution owns multi-step database orchestration for
// contributions that doesn't belong in an HTTP handler. Single-query
// CRUD (create/update/get/list) stays in the handler; only the
// cascading-delete transaction lives here.
package contribution

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shurikai/role-model/internal/db"
)

var ErrNotFound = errors.New("contribution not found")

type Service struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewService(pool *pgxpool.Pool, queries *db.Queries) *Service {
	return &Service{pool: pool, queries: queries}
}

// Delete removes a contribution and its dependent tag/project-link rows
// atomically, after verifying it belongs to userID.
func (s *Service) Delete(ctx context.Context, userID, contributionID uuid.UUID) error {
	if _, err := s.queries.GetContribution(ctx, db.GetContributionParams{
		ID:     contributionID,
		UserID: userID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("delete: verify contribution: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("delete: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after a successful commit

	qtx := s.queries.WithTx(tx)

	if err := qtx.DeleteContributionTags(ctx, contributionID); err != nil {
		return fmt.Errorf("delete: remove tag links: %w", err)
	}
	if err := qtx.DeleteContributionProjectLinks(ctx, contributionID); err != nil {
		return fmt.Errorf("delete: remove project links: %w", err)
	}
	if err := qtx.DeleteContribution(ctx, db.DeleteContributionParams{
		ID:     contributionID,
		UserID: userID,
	}); err != nil {
		return fmt.Errorf("delete: delete contribution: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("delete: commit tx: %w", err)
	}

	return nil
}
