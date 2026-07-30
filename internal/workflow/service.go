package workflow

import (
	"context"

	"github.com/google/uuid"
)

type RunRepository interface {
	CreateRun(context.Context, CreateRunInput) (Run, error)
	ProjectOwned(context.Context, uuid.UUID, uuid.UUID, bool) error
	LatestRun(context.Context, uuid.UUID, uuid.UUID, bool) (Run, error)
	FinalizeBook(context.Context, FinalizeBookRequest) (Run, bool, error)
}

type Service struct {
	repo         RunRepository
	queue        Enqueuer
	allowUnowned bool
}

func NewService(repo RunRepository, queue Enqueuer) *Service {
	return &Service{repo: repo, queue: queue}
}

func NewServiceWithConfig(repo RunRepository, queue Enqueuer, allowUnowned bool) *Service {
	return &Service{repo: repo, queue: queue, allowUnowned: allowUnowned}
}

func (s *Service) Start(ctx context.Context, projectID, staffID uuid.UUID) (Run, error) {
	if projectID == uuid.Nil || staffID == uuid.Nil {
		return Run{}, ErrInvalidRun
	}
	if err := s.repo.ProjectOwned(ctx, projectID, staffID, s.allowUnowned); err != nil {
		return Run{}, err
	}
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

func (s *Service) Latest(ctx context.Context, projectID, staffID uuid.UUID) (Run, error) {
	return s.repo.LatestRun(ctx, projectID, staffID, s.allowUnowned)
}

func (s *Service) Finalize(ctx context.Context, projectID, staffID uuid.UUID, input FinalizeInput) (Run, error) {
	if !input.ConfirmMaterialsReady {
		return Run{}, ErrConfirmationRequired
	}
	if projectID == uuid.Nil || staffID == uuid.Nil {
		return Run{}, ErrInvalidRun
	}
	run, created, err := s.repo.FinalizeBook(ctx, FinalizeBookRequest{
		ProjectID: projectID, StaffID: staffID, IncludeUnowned: s.allowUnowned, Nodes: NodeSequence,
	})
	if err != nil || !created {
		return run, err
	}
	err = s.queue.EnqueueNode(ctx, NodePayload{
		RunID: run.ID, ProjectID: run.ProjectID, Kind: run.Kind, Node: NodeSequence[0],
	})
	return run, err
}
