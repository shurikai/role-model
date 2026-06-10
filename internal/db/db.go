package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shurikai/role-model/internal/config"
)

func New(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, cfg.DatabaseURL)
}
