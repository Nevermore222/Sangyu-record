package workflow

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nevermore222/sangyu-record/internal/providerjobs"
	"github.com/nevermore222/sangyu-record/internal/providers"
)

type memoryNodeRepository struct {
	states       map[string]NodeState
	next         *NodePayload
	succeedCalls int
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
	r.succeedCalls++
	r.states[nodeKey(payload)] = NodeSucceeded
	return r.next, nil
}

type memoryQueue struct {
	nodePayloads  []NodePayload
	providerPolls []ProviderPollPayload
}

func (q *memoryQueue) EnqueueNode(_ context.Context, payload NodePayload) error {
	q.nodePayloads = append(q.nodePayloads, payload)
	return nil
}

func (q *memoryQueue) EnqueueProviderPoll(_ context.Context, payload ProviderPollPayload, _ time.Duration) error {
	q.providerPolls = append(q.providerPolls, payload)
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
	worker := NewWorker(repo, map[NodeName]Processor{NodeTranscribe: &countingProcessor{}}, queue, nil, time.Second)
	payload := NodePayload{RunID: next.RunID, ProjectID: next.ProjectID, Node: NodeTranscribe}

	if err := worker.Process(context.Background(), payload); err != nil {
		t.Fatal(err)
	}
	if len(queue.nodePayloads) != 1 || queue.nodePayloads[0].Node != NodeUnderstandPhoto {
		t.Fatalf("queued = %#v", queue.nodePayloads)
	}
}

type countingProcessor struct {
	calls int
}

func (p *countingProcessor) Process(_ context.Context, _ NodePayload) (ProcessResult, error) {
	p.calls++
	return Completed(json.RawMessage(`{"status":"ok"}`)), nil
}

func TestNodeSuccessIsIdempotent(t *testing.T) {
	repo := newMemoryNodeRepository()
	processor := &countingProcessor{}
	worker := NewWorker(repo, map[NodeName]Processor{NodeTranscribe: processor}, nil, nil, time.Second)
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

type processorFunc func(context.Context, NodePayload) (ProcessResult, error)

func (f processorFunc) Process(ctx context.Context, payload NodePayload) (ProcessResult, error) {
	return f(ctx, payload)
}

type memoryProviderJobs struct {
	job        providerjobs.Job
	outcome    providerjobs.Outcome
	consumable bool
}

func (j *memoryProviderJobs) Refresh(_ context.Context, _ uuid.UUID) (providerjobs.Job, error) {
	return j.job, nil
}

func (j *memoryProviderJobs) ConsumeTerminal(_ context.Context, _ uuid.UUID) (providerjobs.Outcome, bool, error) {
	if !j.consumable {
		return providerjobs.Outcome{}, false, nil
	}
	j.consumable = false
	return j.outcome, true, nil
}

func TestPendingProviderJobLeavesNodeRunningAndQueuesPoll(t *testing.T) {
	repo := newMemoryNodeRepository()
	queue := &memoryQueue{}
	jobID := uuid.New()
	worker := NewWorker(repo, map[NodeName]Processor{
		NodeTranscribe: processorFunc(func(context.Context, NodePayload) (ProcessResult, error) {
			return Waiting(jobID), nil
		}),
	}, queue, nil, time.Second)
	payload := NodePayload{RunID: uuid.New(), ProjectID: uuid.New(), Node: NodeTranscribe}

	if err := worker.Process(context.Background(), payload); err != nil {
		t.Fatal(err)
	}
	if repo.states[nodeKey(payload)] != NodeRunning {
		t.Fatal("node did not stay running")
	}
	if len(queue.providerPolls) != 1 || queue.providerPolls[0].JobID != jobID {
		t.Fatalf("polls = %#v", queue.providerPolls)
	}
}

func TestConsumedProviderSuccessAdvancesWorkflowOnce(t *testing.T) {
	payload := NodePayload{RunID: uuid.New(), ProjectID: uuid.New(), Node: NodeTranscribe}
	next := NodePayload{RunID: payload.RunID, ProjectID: payload.ProjectID, Node: NodeUnderstandPhoto}
	repo := newMemoryNodeRepository()
	repo.next = &next
	queue := &memoryQueue{}
	jobs := &memoryProviderJobs{
		job: providerjobs.Job{ID: uuid.New(), State: providers.StateSucceeded},
		outcome: providerjobs.Outcome{
			ProjectID: payload.ProjectID, WorkflowRunID: payload.RunID, WorkflowNode: string(payload.Node),
			State:  providers.StateSucceeded,
			Output: json.RawMessage(`{"segments":[{"source_ref":"audio#0-1"}]}`),
		},
		consumable: true,
	}
	worker := NewWorker(repo, nil, queue, jobs, time.Second)
	poll := ProviderPollPayload{JobID: jobs.job.ID}

	if err := worker.ProcessProviderPoll(context.Background(), poll); err != nil {
		t.Fatal(err)
	}
	if err := worker.ProcessProviderPoll(context.Background(), poll); err != nil {
		t.Fatal(err)
	}
	if repo.succeedCalls != 1 {
		t.Fatalf("succeed calls = %d", repo.succeedCalls)
	}
	if len(queue.nodePayloads) != 1 || queue.nodePayloads[0] != next {
		t.Fatalf("next nodes = %#v", queue.nodePayloads)
	}
}
