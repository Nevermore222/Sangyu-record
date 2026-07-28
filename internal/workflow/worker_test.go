package workflow

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

type memoryNodeRepository struct {
	states map[string]NodeState
	next   *NodePayload
}

func newMemoryNodeRepository() *memoryNodeRepository {
	return &memoryNodeRepository{states: make(map[string]NodeState)}
}

func nodeKey(payload NodePayload) string {
	return payload.RunID.String() + ":" + string(payload.Node)
}

func (r *memoryNodeRepository) ClaimNode(_ context.Context, payload NodePayload) (bool, error) {
	key := nodeKey(payload)
	if r.states[key] == NodeSucceeded || r.states[key] == NodeRunning {
		return false, nil
	}
	r.states[key] = NodeRunning
	return true, nil
}

func (r *memoryNodeRepository) SucceedNode(_ context.Context, payload NodePayload, _ json.RawMessage) (*NodePayload, error) {
	r.states[nodeKey(payload)] = NodeSucceeded
	return r.next, nil
}

type memoryQueue struct {
	payloads []NodePayload
}

func (q *memoryQueue) Enqueue(_ context.Context, payload NodePayload) error {
	q.payloads = append(q.payloads, payload)
	return nil
}

func (r *memoryNodeRepository) FailNode(_ context.Context, payload NodePayload, _ string) error {
	r.states[nodeKey(payload)] = NodeFailed
	return nil
}

func TestNodeSuccessEnqueuesNextAfterPersistence(t *testing.T) {
	next := NodePayload{RunID: uuid.New(), ProjectID: uuid.New(), Node: NodeUnderstandPhoto}
	repo := newMemoryNodeRepository()
	repo.next = &next
	queue := &memoryQueue{}
	worker := NewWorker(repo, map[NodeName]Processor{NodeTranscribe: &countingProcessor{}}, queue)
	payload := NodePayload{RunID: next.RunID, ProjectID: next.ProjectID, Node: NodeTranscribe}

	if err := worker.Process(context.Background(), payload); err != nil {
		t.Fatal(err)
	}
	if len(queue.payloads) != 1 || queue.payloads[0].Node != NodeUnderstandPhoto {
		t.Fatalf("queued = %#v", queue.payloads)
	}
}

type countingProcessor struct {
	calls int
}

func (p *countingProcessor) Process(_ context.Context, _ NodePayload) (json.RawMessage, error) {
	p.calls++
	return json.RawMessage(`{"status":"ok"}`), nil
}

func TestNodeSuccessIsIdempotent(t *testing.T) {
	repo := newMemoryNodeRepository()
	processor := &countingProcessor{}
	worker := NewWorker(repo, map[NodeName]Processor{NodeTranscribe: processor})
	payload := NodePayload{RunID: uuid.New(), ProjectID: uuid.New(), Node: NodeTranscribe}

	if err := worker.Process(context.Background(), payload); err != nil {
		t.Fatal(err)
	}
	if err := worker.Process(context.Background(), payload); err != nil {
		t.Fatal(err)
	}
	if processor.calls != 1 {
		t.Fatalf("processor calls = %d, want 1", processor.calls)
	}
}
