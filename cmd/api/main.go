package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nevermore222/sangyu-record/internal/assets"
	"github.com/nevermore222/sangyu-record/internal/config"
	"github.com/nevermore222/sangyu-record/internal/httpapi"
	"github.com/nevermore222/sangyu-record/internal/platform"
	"github.com/nevermore222/sangyu-record/internal/projects"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelStartup()
	pool, err := platform.OpenPostgres(startupCtx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(startupCtx); err != nil {
		log.Fatal(err)
	}

	projectRepo := projects.NewPostgresRepository(pool)
	projectService := projects.NewService(projectRepo, projects.DeterministicPlanner{})
	projectHandler := projects.NewHandler(projectService)
	objectClient, err := platform.NewObjectStore(platform.ObjectStoreConfig{
		Endpoint: cfg.S3Endpoint, AccessKey: cfg.S3AccessKey, SecretKey: cfg.S3SecretKey,
	})
	if err != nil {
		log.Fatal(err)
	}
	assetRepo := assets.NewPostgresRepository(pool)
	assetService := assets.NewService(assetRepo, assets.NewMinioObjectStore(objectClient), cfg.S3Bucket)
	assetHandler := assets.NewHandler(assetService)

	server := &http.Server{
		Addr: cfg.HTTPAddress,
		Handler: httpapi.NewRouter(httpapi.Dependencies{
			RegisterStaffRoutes: func(router chi.Router) {
				projectHandler.Register(router)
				assetHandler.Register(router)
			},
		}),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}()

	log.Printf("API listening on %s", cfg.HTTPAddress)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
