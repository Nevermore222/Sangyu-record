package workflow

import (
	"context"
	"encoding/json"
	"errors"
)

type NodeRepository interface {
	ClaimNode(context.Context, NodePayload) (bool, error)
	SucceedNode(context.Context, NodePayload, json.RawMessage) (*NodePayload, error)
	FailNode(context.Context, NodePayload, string) error
}

type Enqueuer interface {
	Enqueue(context.Context, NodePayload) error
}

type Processor interface {
	Process(context.Context, NodePayload) (json.RawMessage, error)
}

type Worker struct {
	repo       NodeRepository
	processors map[NodeName]Processor
	queue      Enqueuer
}

func NewWorker(repo NodeRepository, processors map[NodeName]Processor, queues ...Enqueuer) *Worker {
	worker := &Worker{repo: repo, processors: processors}
	if len(queues) > 0 {
		worker.queue = queues[0]
	}
	return worker
}

func (w *Worker) Process(ctx context.Context, payload NodePayload) error {
	claimed, err := w.repo.ClaimNode(ctx, payload)
	if err != nil || !claimed {
		return err
	}
	processor, ok := w.processors[payload.Node]
	if !ok {
		_ = w.repo.FailNode(ctx, payload, "processor_missing")
		return ErrProcessorMissing
	}
	output, err := processor.Process(ctx, payload)
	if err != nil {
		code := "processing_failed"
		if errors.Is(err, ErrRendererUnavailable) {
			code = "renderer_unavailable"
		}
		_ = w.repo.FailNode(ctx, payload, code)
		return err
	}
	next, err := w.repo.SucceedNode(ctx, payload, output)
	if err != nil || next == nil || w.queue == nil {
		return err
	}
	return w.queue.Enqueue(ctx, *next)
}
