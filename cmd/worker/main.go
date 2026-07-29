package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/hibiken/asynq"
	"github.com/nevermore222/sangyu-record/internal/assets"
	"github.com/nevermore222/sangyu-record/internal/book"
	"github.com/nevermore222/sangyu-record/internal/config"
	"github.com/nevermore222/sangyu-record/internal/platform"
	"github.com/nevermore222/sangyu-record/internal/providerjobs"
	"github.com/nevermore222/sangyu-record/internal/providers"
	"github.com/nevermore222/sangyu-record/internal/workflow"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	providerObjectClient, err := platform.NewObjectStore(platform.ObjectStoreConfig{
		Endpoint: cfg.S3PublicEndpoint, AccessKey: cfg.S3AccessKey, SecretKey: cfg.S3SecretKey, Region: cfg.S3Region,
	})
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := platform.OpenPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		log.Fatal(err)
	}
	client, err := platform.NewAsynqClient(cfg.RedisURL)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = client.Close() }()
	server, err := platform.NewAsynqServer(cfg.RedisURL, 4)
	if err != nil {
		log.Fatal(err)
	}
	repo := workflow.NewPostgresRepository(pool)
	queue := workflow.NewAsynqEnqueuer(client)
	objectClient, err := platform.NewObjectStore(platform.ObjectStoreConfig{
		Endpoint: cfg.S3Endpoint, AccessKey: cfg.S3AccessKey, SecretKey: cfg.S3SecretKey, Region: cfg.S3Region,
	})
	if err != nil {
		log.Fatal(err)
	}
	providerRegistry, err := newProviderRegistry(cfg)
	if err != nil {
		log.Fatal(err)
	}
	providerJobService := providerjobs.NewService(
		providerjobs.NewPostgresRepository(pool),
		providerjobs.NewMinioRawStore(objectClient, cfg.S3Bucket),
		providerRegistry,
		time.Now,
	)
	assetRepo := assets.NewPostgresRepository(pool)
	sourceReader := assets.NewSourceReader(assetRepo, assets.NewMinioObjectStore(objectClient, providerObjectClient), cfg.S3Bucket)
	bookRepo := book.NewPostgresRepository(pool)
	artifactStore := book.NewMinioArtifactStore(objectClient, objectClient)
	renderer := book.NewService(book.NewChromiumEngine(cfg.ChromiumURL), artifactStore, bookRepo, cfg.S3Bucket)
	processors := workflow.ProviderProcessors(providerJobService, sourceReader, cfg.ProviderCallbackBaseURL)
	processors[workflow.NodeRenderPDF] = book.NewWorkflowProcessor(bookRepo, renderer)
	worker := workflow.NewWorker(repo, processors, queue, providerJobService, cfg.ProviderPollInterval)
	mux := asynq.NewServeMux()
	mux.Handle(workflow.TaskWorkflowNode, workflow.NewAsynqHandler(worker))
	mux.Handle(workflow.TaskProviderPoll, workflow.NewProviderPollAsynqHandler(worker))
	reconcileCtx, stopReconciler := context.WithCancel(context.Background())
	defer stopReconciler()
	go runProviderReconciler(reconcileCtx, worker, 30*time.Second)
	log.Print("workflow worker started")
	if err := server.Run(mux); err != nil {
		log.Fatal(err)
	}
}

func runProviderReconciler(ctx context.Context, worker *workflow.Worker, interval time.Duration) {
	reconcile := func(before time.Time) {
		if err := worker.ReconcileProviderJobs(ctx, before); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("reconcile provider jobs: %v", err)
		}
	}
	reconcile(time.Now())

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			reconcile(now.Add(-interval))
		}
	}
}

func newProviderRegistry(cfg config.Config) (providers.Registry, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	media, err := providers.NewHTTPClient(providers.HTTPConfig{
		BaseURL: cfg.MediaProviderURL, Token: cfg.MediaProviderToken, AllowedHosts: cfg.ProviderAllowedHosts,
	}, client)
	if err != nil {
		return providers.Registry{}, err
	}
	knowledge, err := providers.NewHTTPClient(providers.HTTPConfig{
		BaseURL: cfg.KnowledgeProviderURL, Token: cfg.KnowledgeProviderToken, AllowedHosts: cfg.ProviderAllowedHosts,
	}, client)
	if err != nil {
		return providers.Registry{}, err
	}
	agent, err := providers.NewHTTPClient(providers.HTTPConfig{
		BaseURL: cfg.AgentProviderURL, Token: cfg.AgentProviderToken, AllowedHosts: cfg.ProviderAllowedHosts,
	}, client)
	if err != nil {
		return providers.Registry{}, err
	}
	return providers.Registry{Media: media, Knowledge: knowledge, Agent: agent}, nil
}
