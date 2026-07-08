// Package project owns multi-step database orchestration for projects that
// doesn't belong in an HTTP handler. Single-query CRUD (create/update/get/list)
// stays in the handler; only the cascading-delete transaction lives here.
package project

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shurikai/role-model/internal/db"
)

var ErrNotFound = errors.New("project not found")

type Service struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewService(pool *pgxpool.Pool, queries *db.Queries) *Service {
	return &Service{pool: pool, queries: queries}
}

// Delete removes a project and its dependent contribution/tag-link rows
// atomically, after verifying it belongs to userID.
func (s *Service) Delete(ctx context.Context, userID, projectID uuid.UUID) error {
	if _, err := s.queries.GetProject(ctx, db.GetProjectParams{
		ID:     projectID,
		UserID: userID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("delete: verify project: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("delete: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after a successful commit

	qtx := s.queries.WithTx(tx)

	if err := qtx.DeleteProjectContributions(ctx, projectID); err != nil {
		return fmt.Errorf("delete: remove contribution links: %w", err)
	}
	if err := qtx.DeleteProjectTags(ctx, projectID); err != nil {
		return fmt.Errorf("delete: remove tag links: %w", err)
	}
	if _, err := qtx.DeleteProject(ctx, db.DeleteProjectParams{
		ID:     projectID,
		UserID: userID,
	}); err != nil {
		return fmt.Errorf("delete: delete project: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("delete: commit tx: %w", err)
	}

	return nil
}
