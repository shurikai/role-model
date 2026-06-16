package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shurikai/role-model/internal/api/handlers"
	"github.com/shurikai/role-model/internal/db"
)

func NewRouter(pool *pgxpool.Pool, queries *db.Queries) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	healthHandler := handlers.NewHealthHandler(pool)
	r.Get("/health", healthHandler.Health)

	r.Route("/api/v1", func(r chi.Router) {
		employerHandler := handlers.NewEmployerHandler(queries)
		r.Get("/employers", employerHandler.List)
		r.Get("/employers/{id}", employerHandler.Get)
	})

	return r
}
