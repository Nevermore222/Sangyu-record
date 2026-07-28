package main

import (
	"context"
	"log"
	"time"

	"github.com/hibiken/asynq"
	"github.com/nevermore222/sangyu-record/internal/config"
	"github.com/nevermore222/sangyu-record/internal/platform"
	"github.com/nevermore222/sangyu-record/internal/workflow"
)

func main() {
	cfg, err := config.Load()
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
	worker := workflow.NewWorker(repo, workflow.DeterministicProcessors(), queue)
	mux := asynq.NewServeMux()
	mux.Handle(workflow.TaskWorkflowNode, workflow.NewAsynqHandler(worker))
	log.Print("workflow worker started")
	if err := server.Run(mux); err != nil {
		log.Fatal(err)
	}
}
