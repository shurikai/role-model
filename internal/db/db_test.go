//go:build integration

package db_test

import (
	"context"
	"os"
	"testing"

	"github.com/shurikai/role-model/internal/config"
	"github.com/shurikai/role-model/internal/db"
)

func TestNewPool_Connect(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set, skipping DB pool integration test.")
	}

	cfg := &config.Config{DatabaseURL: dsn}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg)
	if err != nil {
		t.Fatalf("NewPool: expected success, got error: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("Ping: expected success, got error: %v", err)
	}
}

func TestNewPool_BadDSN(t *testing.T) {
	cfg := &config.Config{
		DatabaseURL: "postgres://bad:bad@localhost:1/nonexistent?sslmode=disable",
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg)
	if err == nil {
		pool.Close()
		t.Fatal("expected error for unreachable database, got nil")
	}
}
