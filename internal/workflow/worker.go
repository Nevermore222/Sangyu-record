package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/nevermore222/sangyu-record/internal/providerjobs"
	"github.com/nevermore222/sangyu-record/internal/providers"
)

type NodeRepository interface {
	ClaimNode(context.Context, NodePayload) (bool, error)
	SucceedNode(context.Context, NodePayload, json.RawMessage) (*NodePayload, error)
	FailNode(context.Context, NodePayload, string) error
}

type Enqueuer interface {
	EnqueueNode(context.Context, NodePayload) error
	EnqueueProviderPoll(context.Context, ProviderPollPayload, time.Duration) error
}

type ProviderJobs interface {
	Refresh(context.Context, uuid.UUID) (providerjobs.Job, error)
	FindUnconsumedByWorkflowNode(context.Context, uuid.UUID, uuid.UUID, string) (providerjobs.Job, error)
	ListUnconsumedDue(context.Context, time.Time, int) ([]providerjobs.Job, error)
	PeekTerminal(context.Context, uuid.UUID) (providerjobs.Outcome, bool, error)
	MarkConsumed(context.Context, uuid.UUID) error
}

func (w *Worker) ReconcileProviderJobs(ctx context.Context, before time.Time) error {
	if w.jobs == nil || w.queue == nil {
		return nil
	}
	jobs, err := w.jobs.ListUnconsumedDue(ctx, before, 100)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if err := w.queue.EnqueueProviderPoll(ctx, ProviderPollPayload{JobID: job.ID}, 0); err != nil {
			return err
		}
	}
	return nil
}

type ProcessResult struct {
	Output        json.RawMessage
	ProviderJobID uuid.UUID
}

func Completed(output json.RawMessage) ProcessResult {
	return ProcessResult{Output: output}
}

func Waiting(jobID uuid.UUID) ProcessResult {
	return ProcessResult{ProviderJobID: jobID}
}

func (r ProcessResult) IsWaiting() bool {
	return r.ProviderJobID != uuid.Nil
}

type Processor interface {
	Process(context.Context, NodePayload) (ProcessResult, error)
}

type Worker struct {
	repo       NodeRepository
	processors map[NodeName]Processor
	queue      Enqueuer
	jobs       ProviderJobs
	pollEvery  time.Duration
}

func NewWorker(repo NodeRepository, processors map[NodeName]Processor, queue Enqueuer, jobs ProviderJobs, pollInterval time.Duration) *Worker {
	if pollInterval <= 0 {
		pollInterval = 2 * time.Second
	}
	return &Worker{repo: repo, processors: processors, queue: queue, jobs: jobs, pollEvery: pollInterval}
}

func (w *Worker) Process(ctx context.Context, payload NodePayload) error {
	claimed, err := w.repo.ClaimNode(ctx, payload)
	if err != nil {
		return err
	}
	if !claimed {
		return w.recoverProviderPoll(ctx, payload)
	}
	processor, ok := w.processors[payload.Node]
	if !ok {
		_ = w.repo.FailNode(ctx, payload, "processor_missing")
		return ErrProcessorMissing
	}
	result, err := processor.Process(ctx, payload)
	if err != nil {
		code := "processing_failed"
		if errors.Is(err, ErrRendererUnavailable) {
			code = "renderer_unavailable"
		}
		_ = w.repo.FailNode(ctx, payload, code)
		return err
	}
	if result.IsWaiting() {
		if w.queue == nil {
			return nil
		}
		return w.queue.EnqueueProviderPoll(ctx, ProviderPollPayload{JobID: result.ProviderJobID}, w.pollEvery)
	}
	next, err := w.repo.SucceedNode(ctx, payload, result.Output)
	if err != nil || next == nil || w.queue == nil {
		return err
	}
	return w.queue.EnqueueNode(ctx, *next)
}

func (w *Worker) recoverProviderPoll(ctx context.Context, payload NodePayload) error {
	if w.jobs == nil || w.queue == nil {
		return nil
	}
	job, err := w.jobs.FindUnconsumedByWorkflowNode(ctx, payload.ProjectID, payload.RunID, string(payload.Node))
	if errors.Is(err, providerjobs.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return w.queue.EnqueueProviderPoll(ctx, ProviderPollPayload{JobID: job.ID}, 0)
}

func (w *Worker) ProcessProviderPoll(ctx context.Context, payload ProviderPollPayload) error {
	if w.jobs == nil {
		return ErrProcessorMissing
	}
	job, err := w.jobs.Refresh(ctx, payload.JobID)
	if err != nil {
		return err
	}
	if !job.State.Terminal() {
		if w.queue == nil {
			return nil
		}
		return w.queue.EnqueueProviderPoll(ctx, payload, w.pollEvery)
	}
	outcome, available, err := w.jobs.PeekTerminal(ctx, payload.JobID)
	if err != nil || !available {
		return err
	}
	node := NodePayload{
		RunID: outcome.WorkflowRunID, ProjectID: outcome.ProjectID, Node: NodeName(outcome.WorkflowNode),
	}
	if outcome.State != providers.StateSucceeded {
		code := outcome.ErrorCode
		if code == "" {
			code = "provider_failed"
		}
		if err := w.repo.FailNode(ctx, node, code); err != nil {
			return err
		}
		return w.jobs.MarkConsumed(ctx, payload.JobID)
	}
	next, err := w.repo.SucceedNode(ctx, node, outcome.Output)
	if err != nil {
		return err
	}
	if next != nil && w.queue != nil {
		if err := w.queue.EnqueueNode(ctx, *next); err != nil {
			return err
		}
	}
	return w.jobs.MarkConsumed(ctx, payload.JobID)
}
