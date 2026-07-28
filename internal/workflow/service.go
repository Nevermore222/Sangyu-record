package workflow

import (
	"context"

	"github.com/google/uuid"
)

type RunRepository interface {
	CreateRun(context.Context, uuid.UUID) (Run, error)
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
	run, err := s.repo.CreateRun(ctx, projectID)
	if err != nil {
		return Run{}, err
	}
	err = s.queue.Enqueue(ctx, NodePayload{RunID: run.ID, ProjectID: projectID, Node: NodeTranscribe})
	return run, err
}

func (s *Service) Latest(ctx context.Context, projectID uuid.UUID) (Run, error) {
	return s.repo.LatestRun(ctx, projectID)
}
