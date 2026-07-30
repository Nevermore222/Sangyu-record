package workflow

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type memoryRunRepository struct {
	run             Run
	input           CreateRunInput
	finalizeCreated bool
	finalizeErr     error
}

func (r *memoryRunRepository) CreateRun(_ context.Context, input CreateRunInput) (Run, error) {
	r.input = input
	r.run = Run{
		ID: uuid.New(), ProjectID: input.ProjectID, VisitID: input.VisitID, Kind: input.Kind,
		State: NodeQueued, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	for position, name := range input.Nodes {
		r.run.Nodes = append(r.run.Nodes, Node{Name: name, State: NodeQueued, Position: position})
	}
	return r.run, nil
}

func (r *memoryRunRepository) ProjectOwned(_ context.Context, _, _ uuid.UUID, _ bool) error {
	return nil
}

func (r *memoryRunRepository) LatestRun(_ context.Context, _, _ uuid.UUID, _ bool) (Run, error) {
	return r.run, nil
}

func (r *memoryRunRepository) FinalizeBook(_ context.Context, input FinalizeBookRequest) (Run, bool, error) {
	if r.finalizeErr != nil {
		return Run{}, false, r.finalizeErr
	}
	if r.run.ID != uuid.Nil && !r.finalizeCreated {
		return r.run, false, nil
	}
	r.finalizeCreated = false
	r.input = CreateRunInput{ProjectID: input.ProjectID, Kind: RunKindBook, Nodes: input.Nodes}
	r.run = Run{ID: uuid.New(), ProjectID: input.ProjectID, Kind: RunKindBook, State: NodeQueued}
	for position, name := range input.Nodes {
		r.run.Nodes = append(r.run.Nodes, Node{Name: name, State: NodeQueued, Position: position})
	}
	return r.run, true, nil
}

func TestStartPersistsBeforeEnqueueingFirstNode(t *testing.T) {
	repo := &memoryRunRepository{}
	queue := &memoryQueue{}
	service := NewService(repo, queue)
	projectID := uuid.New()

	run, err := service.Start(context.Background(), projectID, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if run.ID == uuid.Nil {
		t.Fatal("run was not persisted")
	}
	if run.Kind != RunKindBook || len(run.Nodes) != 7 || len(repo.input.Nodes) != 7 {
		t.Fatalf("book run = %#v, input = %#v", run, repo.input)
	}
	if len(queue.nodePayloads) != 1 || queue.nodePayloads[0].Node != NodeTranscribe || queue.nodePayloads[0].RunID != run.ID {
		t.Fatalf("queued = %#v", queue.nodePayloads)
	}
}

func TestFinalizeRequiresExplicitConfirmation(t *testing.T) {
	service := NewService(&memoryRunRepository{}, &memoryQueue{})
	_, err := service.Finalize(context.Background(), uuid.New(), uuid.New(), FinalizeInput{})
	if !errors.Is(err, ErrConfirmationRequired) {
		t.Fatalf("err = %v", err)
	}
}

func TestFinalizePropagatesReadinessFailure(t *testing.T) {
	repo := &memoryRunRepository{finalizeErr: ErrDraftVisitExists}
	service := NewService(repo, &memoryQueue{})
	_, err := service.Finalize(context.Background(), uuid.New(), uuid.New(), FinalizeInput{ConfirmMaterialsReady: true})
	if !errors.Is(err, ErrDraftVisitExists) {
		t.Fatalf("err = %v", err)
	}
}

func TestFinalizeReturnsExistingActiveBookRun(t *testing.T) {
	repo := &memoryRunRepository{finalizeCreated: true}
	queue := &memoryQueue{}
	service := NewService(repo, queue)
	input := FinalizeInput{ConfirmMaterialsReady: true}
	projectID, staffID := uuid.New(), uuid.New()

	first, err := service.Finalize(context.Background(), projectID, staffID, input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Finalize(context.Background(), projectID, staffID, input)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("run IDs = %s and %s", first.ID, second.ID)
	}
	if len(queue.nodePayloads) != 1 {
		t.Fatalf("queued payloads = %d, want 1", len(queue.nodePayloads))
	}
}
