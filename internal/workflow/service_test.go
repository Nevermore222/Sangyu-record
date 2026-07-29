package workflow

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

type memoryRunRepository struct {
	run   Run
	input CreateRunInput
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

func (r *memoryRunRepository) LatestRun(_ context.Context, _ uuid.UUID) (Run, error) {
	return r.run, nil
}

func TestStartPersistsBeforeEnqueueingFirstNode(t *testing.T) {
	repo := &memoryRunRepository{}
	queue := &memoryQueue{}
	service := NewService(repo, queue)
	projectID := uuid.New()

	run, err := service.Start(context.Background(), projectID)
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
