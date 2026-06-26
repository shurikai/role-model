package api

import (
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shurikai/role-model/internal/api/handlers"
	"github.com/shurikai/role-model/internal/api/middleware"
	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/generation"
)

func NewRouter(pool *pgxpool.Pool, queries *db.Queries, genSvc *generation.Service, jwtSecret string) *chi.Mux {
	r := chi.NewRouter()
	r.Use(chimiddleware.Logger)
	r.Use(middleware.Recoverer)

	healthHandler := handlers.NewHealthHandler(pool)
	r.Get("/health", healthHandler.Health)

	r.Route("/api/v1", func(r chi.Router) {
		authHandler := handlers.NewAuthHandler(queries, jwtSecret)
		r.Post("/auth/signup", authHandler.Signup)
		r.Post("/auth/login", authHandler.Login)

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth(jwtSecret))

			employerHandler := handlers.NewEmployerHandler(queries)
			r.Get("/employers", employerHandler.List)
			r.Get("/employers/{id}", employerHandler.Get)
			r.Post("/employers", employerHandler.Create)
			r.Patch("/employers/{id}", employerHandler.Update)
			r.Delete("/employers/{id}", employerHandler.Delete)

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

			generationHandler := handlers.NewGenerationHandler(queries, genSvc)
			r.Post("/applications/{id}/extract-signals", generationHandler.ExtractSignals)
			r.Post("/applications/{id}/generate", generationHandler.Generate)

			resumeVersionHandler := handlers.NewResumeVersionHandler(queries)
			r.Get("/applications/{id}/versions", resumeVersionHandler.ListByApplication)
			r.Get("/resume-versions/{id}", resumeVersionHandler.Get)
		})
	})

	return r
}
