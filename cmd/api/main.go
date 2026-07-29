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
	"github.com/google/uuid"
	"github.com/nevermore222/sangyu-record/internal/assets"
	"github.com/nevermore222/sangyu-record/internal/book"
	"github.com/nevermore222/sangyu-record/internal/config"
	"github.com/nevermore222/sangyu-record/internal/httpapi"
	"github.com/nevermore222/sangyu-record/internal/platform"
	"github.com/nevermore222/sangyu-record/internal/projects"
	"github.com/nevermore222/sangyu-record/internal/providerjobs"
	"github.com/nevermore222/sangyu-record/internal/providers"
	"github.com/nevermore222/sangyu-record/internal/staff"
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
	staffRepo := staff.NewPostgresRepository(pool)
	var codeExchanger staff.CodeExchanger
	if cfg.AuthMode == "wechat" {
		codeExchanger = staff.NewHTTPExchanger(
			staff.WeChatCode2SessionURL,
			cfg.WeChatAppID,
			cfg.WeChatAppSecret,
			&http.Client{Timeout: 10 * time.Second},
		)
	}
	allowedOpenIDs := make(map[string]struct{}, len(cfg.StaffOpenIDAllowlist))
	for _, openID := range cfg.StaffOpenIDAllowlist {
		allowedOpenIDs[openID] = struct{}{}
	}
	staffService := staff.NewService(staffRepo, codeExchanger, staff.Config{
		Mode:           cfg.AuthMode,
		AllowedOpenIDs: allowedOpenIDs,
		SessionTTL:     cfg.SessionTTL,
		SessionSecret:  []byte(cfg.SessionSecret),
	}, time.Now)
	staffHandler := staff.NewHandler(staffService)
	staffMiddleware := staff.NewMiddleware(staffService)

	projectRepo := projects.NewPostgresRepository(pool)
	projectService := projects.NewServiceWithConfig(projectRepo, projects.DeterministicPlanner{}, cfg.AuthMode == "dev")
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
	workflowQueue := workflow.NewAsynqEnqueuer(asynqClient)
	callbackHandler := providerjobs.NewCallbackHandler(
		providerJobService,
		callbackPollEnqueuer{queue: workflowQueue},
		providerjobs.NewCallbackVerifier([]byte(cfg.ProviderCallbackSecret), 5*time.Minute, nil),
	)
	workflowRepo := workflow.NewPostgresRepository(pool)
	workflowService := workflow.NewService(workflowRepo, workflowQueue)
	workflowHandler := workflow.NewHandler(workflowService)
	bookRepo := book.NewPostgresRepository(pool)
	artifactStore := book.NewMinioArtifactStore(objectClient, publicObjectClient)
	bookHandler := book.NewHandler(book.NewCatalog(bookRepo, artifactStore, cfg.S3Bucket))

	server := &http.Server{
		Addr: cfg.HTTPAddress,
		Handler: httpapi.NewRouter(httpapi.Dependencies{
			RegisterAuthRoutes:     staffHandler.RegisterAuth,
			RegisterProviderRoutes: callbackHandler.Register,
			StaffMiddleware:        staffMiddleware.Handle,
			RegisterStaffRoutes: func(router chi.Router) {
				staffHandler.RegisterStaff(router)
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

type callbackPollEnqueuer struct {
	queue *workflow.AsynqEnqueuer
}

func (q callbackPollEnqueuer) EnqueueProviderPoll(ctx context.Context, jobID uuid.UUID, delay time.Duration) error {
	return q.queue.EnqueueProviderPoll(ctx, workflow.ProviderPollPayload{JobID: jobID}, delay)
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
