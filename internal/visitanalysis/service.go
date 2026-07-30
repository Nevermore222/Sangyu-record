package visitanalysis

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/nevermore222/sangyu-record/internal/visits"
	"github.com/nevermore222/sangyu-record/internal/workflow"
)

type VisitAccess interface {
	Get(context.Context, uuid.UUID, uuid.UUID) (visits.Visit, error)
}

type WorkflowStarter interface {
	StartRun(context.Context, workflow.CreateRunInput) (workflow.Run, error)
}

type NodeQueue interface {
	EnqueueNode(context.Context, workflow.NodePayload) error
}

type AnalysisRepository interface {
	PrepareSubmit(context.Context, uuid.UUID) error
	AttachRun(context.Context, uuid.UUID, uuid.UUID) error
	RevertSubmit(context.Context, uuid.UUID) error
	RetryPayload(context.Context, uuid.UUID) (workflow.NodePayload, error)
	GetByVisit(context.Context, uuid.UUID) (Analysis, error)
}

type Service struct {
	repo      AnalysisRepository
	visits    VisitAccess
	workflows WorkflowStarter
	queue     NodeQueue
}

func NewService(repo AnalysisRepository, visits VisitAccess, workflows WorkflowStarter, queue NodeQueue) *Service {
	return &Service{repo: repo, visits: visits, workflows: workflows, queue: queue}
}

func (s *Service) Submit(ctx context.Context, visitID, staffID uuid.UUID) (workflow.Run, error) {
	visit, err := s.visits.Get(ctx, visitID, staffID)
	if err != nil {
		return workflow.Run{}, err
	}
	if visit.State != visits.StateDraft {
		return workflow.Run{}, ErrInvalidState
	}
	if err := s.repo.PrepareSubmit(ctx, visitID); err != nil {
		return workflow.Run{}, err
	}
	run, startErr := s.workflows.StartRun(ctx, workflow.CreateRunInput{
		ProjectID: visit.ProjectID,
		VisitID:   visit.ID,
		Kind:      workflow.RunKindVisitAnalysis,
		Nodes:     workflow.VisitAnalysisSequence,
	})
	if run.ID == uuid.Nil {
		_ = s.repo.RevertSubmit(ctx, visitID)
		return workflow.Run{}, startErr
	}
	if err := s.repo.AttachRun(ctx, visitID, run.ID); err != nil {
		return workflow.Run{}, err
	}
	return run, startErr
}

func (s *Service) Retry(ctx context.Context, visitID, staffID uuid.UUID) (workflow.NodePayload, error) {
	visit, err := s.visits.Get(ctx, visitID, staffID)
	if err != nil {
		return workflow.NodePayload{}, err
	}
	if visit.State != visits.StateFailed {
		return workflow.NodePayload{}, ErrInvalidState
	}
	payload, err := s.repo.RetryPayload(ctx, visitID)
	if err != nil {
		return workflow.NodePayload{}, err
	}
	if err := s.queue.EnqueueNode(ctx, payload); err != nil {
		return workflow.NodePayload{}, err
	}
	return payload, nil
}

func (s *Service) Get(ctx context.Context, visitID, staffID uuid.UUID) (Analysis, error) {
	if _, err := s.visits.Get(ctx, visitID, staffID); err != nil {
		return Analysis{}, err
	}
	return s.repo.GetByVisit(ctx, visitID)
}

func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound) || errors.Is(err, visits.ErrNotFound)
}
