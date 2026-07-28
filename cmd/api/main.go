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
	"github.com/nevermore222/sangyu-record/internal/book"
	"github.com/nevermore222/sangyu-record/internal/config"
	"github.com/nevermore222/sangyu-record/internal/httpapi"
	"github.com/nevermore222/sangyu-record/internal/platform"
	"github.com/nevermore222/sangyu-record/internal/projects"
	"github.com/nevermore222/sangyu-record/internal/workflow"
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
		Endpoint: cfg.S3Endpoint, AccessKey: cfg.S3AccessKey, SecretKey: cfg.S3SecretKey, Region: cfg.S3Region,
	})
	if err != nil {
		log.Fatal(err)
	}
	publicObjectClient, err := platform.NewObjectStore(platform.ObjectStoreConfig{
		Endpoint: cfg.S3PublicEndpoint, AccessKey: cfg.S3AccessKey, SecretKey: cfg.S3SecretKey, Region: cfg.S3Region,
	})
	if err != nil {
		log.Fatal(err)
	}
	assetRepo := assets.NewPostgresRepository(pool)
	assetService := assets.NewService(assetRepo, assets.NewMinioObjectStore(objectClient, publicObjectClient), cfg.S3Bucket)
	assetHandler := assets.NewHandler(assetService)
	asynqClient, err := platform.NewAsynqClient(cfg.RedisURL)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = asynqClient.Close() }()
	workflowRepo := workflow.NewPostgresRepository(pool)
	workflowService := workflow.NewService(workflowRepo, workflow.NewAsynqEnqueuer(asynqClient))
	workflowHandler := workflow.NewHandler(workflowService)
	bookRepo := book.NewPostgresRepository(pool)
	artifactStore := book.NewMinioArtifactStore(objectClient, publicObjectClient)
	bookHandler := book.NewHandler(book.NewCatalog(bookRepo, artifactStore, cfg.S3Bucket))

	server := &http.Server{
		Addr: cfg.HTTPAddress,
		Handler: httpapi.NewRouter(httpapi.Dependencies{
			RegisterStaffRoutes: func(router chi.Router) {
				projectHandler.Register(router)
				assetHandler.Register(router)
				workflowHandler.Register(router)
				bookHandler.Register(router)
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
