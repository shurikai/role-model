package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shurikai/role-model/internal/api/handlers"
	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/generation"
)

func NewRouter(pool *pgxpool.Pool, queries *db.Queries, genClient *generation.Client) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	healthHandler := handlers.NewHealthHandler(pool)
	r.Get("/health", healthHandler.Health)

	r.Route("/api/v1", func(r chi.Router) {
		employerHandler := handlers.NewEmployerHandler(queries)
		r.Get("/employers", employerHandler.List)
		r.Get("/employers/{id}", employerHandler.Get)

		positionHandler := handlers.NewPositionHandler(queries)
		r.Get("/employers/{employerID}/positions", positionHandler.ListByEmployer)
		r.Get("/positions/{id}", positionHandler.Get)

		contributionHandler := handlers.NewContributionHandler(queries)
		r.Get("/positions/{positionID}/contributions", contributionHandler.ListByPosition)
		r.Get("/contributions/{id}", contributionHandler.Get)

		applicationHandler := handlers.NewApplicationHandler(queries)
		r.Get("/applications", applicationHandler.List)
		r.Post("/applications", applicationHandler.Create)
		r.Get("/applications/{id}", applicationHandler.Get)
		r.Patch("/applications/{id}", applicationHandler.Update)

		generationHandler := handlers.NewGenerationHandler(queries, genClient)
		r.Post("/applications/{id}/extract-signals", generationHandler.ExtractSignals)
	})

	return r
}
