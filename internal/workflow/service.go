package workflow

import (
	"context"

	"github.com/google/uuid"
)

type RunRepository interface {
	CreateRun(context.Context, CreateRunInput) (Run, error)
	LatestRun(context.Context, uuid.UUID) (Run, error)
}

type Service struct {
	repo  RunRepository
	queue Enqueuer
}

func NewService(repo RunRepository, queue Enqueuer) *Service {
	return &Service{repo: repo, queue: queue}
}

func (s *Service) Start(ctx context.Context, projectID uuid.UUID) (Run, error) {
	return s.StartRun(ctx, CreateRunInput{
		ProjectID: projectID,
		Kind:      RunKindBook,
		Nodes:     NodeSequence,
	})
}

func (s *Service) StartRun(ctx context.Context, input CreateRunInput) (Run, error) {
	if len(input.Nodes) == 0 {
		return Run{}, ErrSequenceEmpty
	}
	run, err := s.repo.CreateRun(ctx, input)
	if err != nil {
		return Run{}, err
	}
	err = s.queue.EnqueueNode(ctx, NodePayload{
		RunID: run.ID, ProjectID: run.ProjectID, VisitID: run.VisitID,
		Kind: run.Kind, Node: input.Nodes[0],
	})
	return run, err
}

func (s *Service) Latest(ctx context.Context, projectID uuid.UUID) (Run, error) {
	return s.repo.LatestRun(ctx, projectID)
}
