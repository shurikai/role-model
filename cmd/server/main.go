package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/shurikai/role-model/internal/api"
	"github.com/shurikai/role-model/internal/config"
	"github.com/shurikai/role-model/internal/contribution"
	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/fitgate"
	"github.com/shurikai/role-model/internal/generation"
	"github.com/shurikai/role-model/internal/intake"
	"github.com/shurikai/role-model/internal/project"
	"github.com/shurikai/role-model/internal/renderer"
	"github.com/shurikai/role-model/internal/stage0"
)

func main() {
	cfg := config.Load()

	// Refuse to start on a misconfiguration that would otherwise be silent —
	// an empty JWT_SECRET above all, which makes every token forgeable with no
	// symptom at all (#36).
	if err := cfg.Validate(); err != nil {
		log.Fatal(err)
	}

	pool, err := db.NewPool(context.Background(), &cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	queries := db.New(pool)

	genClient := generation.NewClient(cfg.AnthropicAPIKey)
	genSvc := generation.NewService(queries, genClient)
	stage0Svc := stage0.NewService(pool, queries, genClient)
	intakeSvc := intake.NewService(pool, queries)
	fitSvc := fitgate.NewService(queries, genClient)
	contribSvc := contribution.NewService(pool, queries)
	projectSvc := project.NewService(pool, queries)
	rendererClient := renderer.NewClient(cfg.RendererURL)
	router := api.NewRouter(api.RouterDeps{
		Pool:           pool,
		Queries:        queries,
		GenSvc:         genSvc,
		Stage0Svc:      stage0Svc,
		IntakeSvc:      intakeSvc,
		GenClient:      genClient,
		FitSvc:         fitSvc,
		ContribSvc:     contribSvc,
		ProjectSvc:     projectSvc,
		RendererClient: rendererClient,
		JWTSecret:      cfg.JWTSecret,
		AllowedOrigins: cfg.AllowedOrigins,
		SignupEnabled:  cfg.SignupEnabled,
	})

	// Timeouts, all of which were absent. Without ReadHeaderTimeout a client
	// can hold a connection open indefinitely by dribbling headers, and
	// without WriteTimeout the request context carries no deadline at all —
	// which is what let a hung renderer occupy a request forever.
	//
	// WriteTimeout is deliberately the LOOSEST of the three deadlines that can
	// end a slow request. The Anthropic client gives up at
	// generation.RequestTimeout and the renderer client at
	// renderer.RequestTimeout, both well inside this, so an upstream that
	// hangs produces a specific error from the client that hung rather than a
	// connection the server killed with nothing to say about why. Whenever one
	// of those is raised, raise this too, or it silently becomes the thing
	// that fires first.
	//
	// It is large because generation legitimately takes minutes: career
	// extraction is capped at 16384 output tokens and 2a/2b are two calls in
	// series. A conventional 30s would break the pipeline's normal path.
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       120 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("server listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-quit
	log.Println("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("server forced to shutdown: %v", err)
	}

	log.Println("server stopped")
}
