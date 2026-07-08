package api

import (
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shurikai/role-model/internal/api/handlers"
	"github.com/shurikai/role-model/internal/api/middleware"
	"github.com/shurikai/role-model/internal/contribution"
	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/fitgate"
	"github.com/shurikai/role-model/internal/generation"
	"github.com/shurikai/role-model/internal/stage0"
)

type RouterDeps struct {
	Pool           *pgxpool.Pool
	Queries        *db.Queries
	GenSvc         *generation.Service
	Stage0Svc      *stage0.Service
	FitSvc         *fitgate.Service
	ContribSvc     *contribution.Service
	JWTSecret      string
	AllowedOrigins []string
}

func NewRouter(deps RouterDeps) *chi.Mux {
	r := chi.NewRouter()
	// Mounted before Recoverer/Logger so CORS preflight (OPTIONS) requests —
	// which never carry the Authorization header — are answered here instead
	// of falling through to RequireAuth and being rejected as unauthenticated.
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   deps.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
	r.Use(chimiddleware.Logger)
	r.Use(middleware.Recoverer)

	healthHandler := handlers.NewHealthHandler(deps.Pool)
	r.Get("/health", healthHandler.Health)

	r.Route("/api/v1", func(r chi.Router) {
		authHandler := handlers.NewAuthHandler(deps.Queries, deps.JWTSecret)
		r.Post("/auth/signup", authHandler.Signup)
		r.Post("/auth/login", authHandler.Login)

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth(deps.JWTSecret))

			r.Get("/auth/me", authHandler.Me)

			employerHandler := handlers.NewEmployerHandler(deps.Queries)
			r.Get("/employers", employerHandler.List)
			r.Get("/employers/{id}", employerHandler.Get)
			r.Post("/employers", employerHandler.Create)
			r.Patch("/employers/{id}", employerHandler.Update)
			r.Delete("/employers/{id}", employerHandler.Delete)

			positionHandler := handlers.NewPositionHandler(deps.Queries)
			r.Get("/employers/{employerID}/positions", positionHandler.ListByEmployer)
			r.Get("/positions/{id}", positionHandler.Get)
			r.Post("/positions", positionHandler.Create)
			r.Patch("/positions/{id}", positionHandler.Update)
			r.Delete("/positions/{id}", positionHandler.Delete)

			contributionHandler := handlers.NewContributionHandler(deps.Queries, deps.ContribSvc)
			r.Get("/positions/{positionID}/contributions", contributionHandler.ListByPosition)
			r.Get("/contributions/{id}", contributionHandler.Get)
			r.Post("/contributions", contributionHandler.Create)
			r.Patch("/contributions/{id}", contributionHandler.Update)
			r.Delete("/contributions/{id}", contributionHandler.Delete)

			applicationHandler := handlers.NewApplicationHandler(deps.Queries)
			r.Get("/applications", applicationHandler.List)
			r.Post("/applications", applicationHandler.Create)
			r.Get("/applications/{id}", applicationHandler.Get)
			r.Patch("/applications/{id}", applicationHandler.Update)

			generationHandler := handlers.NewGenerationHandler(deps.Queries, deps.GenSvc)
			r.Post("/applications/{id}/extract-signals", generationHandler.ExtractSignals)
			r.Post("/applications/{id}/generate", generationHandler.Generate)

			resumeVersionHandler := handlers.NewResumeVersionHandler(deps.Queries)
			r.Get("/applications/{id}/versions", resumeVersionHandler.ListByApplication)
			r.Get("/resume-versions/{id}", resumeVersionHandler.Get)

			fitHandler := handlers.NewFitHandler(deps.Queries, deps.FitSvc)
			r.Post("/applications/{applicationID}/fit", fitHandler.Run)
			r.Get("/applications/{applicationID}/fit", fitHandler.ListByApplication)
			r.Get("/applications/{applicationID}/fit/{reportID}", fitHandler.Get)

			importHandler := handlers.NewImportHandler(deps.Queries, deps.Stage0Svc)
			r.Post("/import", importHandler.Create)
			r.Get("/import/{batchID}", importHandler.GetBatch)
			r.Get("/import/{batchID}/drafts", importHandler.ListDrafts)
			r.Get("/import/drafts/{draftID}", importHandler.GetDraft)
			r.Put("/import/drafts/{draftID}", importHandler.UpdateDraft)
			r.Post("/import/drafts/{draftID}/approve", importHandler.Approve)
			r.Post("/import/drafts/{draftID}/reject", importHandler.Reject)

			educationHandler := handlers.NewEducationHandler(deps.Queries)
			r.Post("/education", educationHandler.Create)
			r.Patch("/education/{id}", educationHandler.Update)
			r.Delete("/education/{id}", educationHandler.Delete)

			credentialHandler := handlers.NewCredentialHandler(deps.Queries)
			r.Post("/credentials", credentialHandler.Create)
			r.Patch("/credentials/{id}", credentialHandler.Update)
			r.Delete("/credentials/{id}", credentialHandler.Delete)

			tagHandler := handlers.NewTagHandler(deps.Queries)
			r.Post("/tags", tagHandler.Create)
			r.Get("/tags", tagHandler.List)
			r.Delete("/tags/{id}", tagHandler.Delete)

			r.Post("/tag-categories", tagHandler.CreateCategory)
			r.Get("/tag-categories", tagHandler.ListCategories)
			r.Delete("/tag-categories/{id}", tagHandler.DeleteCategory)

			r.Post("/contributions/{id}/tags", tagHandler.AssignToContribution)
			r.Delete("/contributions/{id}/tags/{tagId}", tagHandler.UnassignFromContribution)
		})
	})

	return r
}
