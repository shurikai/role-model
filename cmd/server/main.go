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
	"github.com/shurikai/role-model/internal/project"
	"github.com/shurikai/role-model/internal/renderer"
	"github.com/shurikai/role-model/internal/stage0"
)

func main() {
	cfg := config.Load()

	pool, err := db.NewPool(context.Background(), &cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	queries := db.New(pool)

	genClient := generation.NewClient(cfg.AnthropicAPIKey)
	genSvc := generation.NewService(queries, genClient)
	stage0Svc := stage0.NewService(pool, queries, genClient)
	fitSvc := fitgate.NewService(queries, genClient)
	contribSvc := contribution.NewService(pool, queries)
	projectSvc := project.NewService(pool, queries)
	rendererClient := renderer.NewClient(cfg.RendererURL)
	router := api.NewRouter(api.RouterDeps{
		Pool:           pool,
		Queries:        queries,
		GenSvc:         genSvc,
		Stage0Svc:      stage0Svc,
		FitSvc:         fitSvc,
		ContribSvc:     contribSvc,
		ProjectSvc:     projectSvc,
		RendererClient: rendererClient,
		JWTSecret:      cfg.JWTSecret,
		AllowedOrigins: cfg.AllowedOrigins,
	})

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
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
