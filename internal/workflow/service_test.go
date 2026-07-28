package workflow

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

type memoryRunRepository struct {
	run Run
}

func (r *memoryRunRepository) CreateRun(_ context.Context, projectID uuid.UUID) (Run, error) {
	r.run = Run{ID: uuid.New(), ProjectID: projectID, State: NodeQueued, CreatedAt: time.Now(), UpdatedAt: time.Now()}
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
	if len(queue.payloads) != 1 || queue.payloads[0].Node != NodeTranscribe || queue.payloads[0].RunID != run.ID {
		t.Fatalf("queued = %#v", queue.payloads)
	}
}
