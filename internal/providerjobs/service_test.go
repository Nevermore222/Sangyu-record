package providerjobs

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nevermore222/sangyu-record/internal/providers"
)

type memoryJobsRepository struct {
	jobs       map[uuid.UUID]Job
	byKey      map[string]uuid.UUID
	consumed   map[uuid.UUID]bool
	attemptSeq map[uuid.UUID]int
}

func newMemoryJobsRepository() *memoryJobsRepository {
	return &memoryJobsRepository{
		jobs: map[uuid.UUID]Job{}, byKey: map[string]uuid.UUID{}, consumed: map[uuid.UUID]bool{}, attemptSeq: map[uuid.UUID]int{},
	}
}

func (r *memoryJobsRepository) CreateOrGet(_ context.Context, input CreateInput) (Job, bool, error) {
	if id, ok := r.byKey[input.IdempotencyKey]; ok {
		return r.jobs[id], false, nil
	}
	job := Job{ID: uuid.New(), RequestID: input.RequestID, ProjectID: input.ProjectID, WorkflowRunID: input.WorkflowRunID,
		WorkflowNode: input.WorkflowNode, ProviderKind: input.ProviderKind, TaskType: input.TaskType,
		IdempotencyKey: input.IdempotencyKey, State: providers.StatePendingSubmission, Input: input.Input, Deadline: input.Deadline}
	r.jobs[job.ID], r.byKey[input.IdempotencyKey] = job, job.ID
	return job, true, nil
}

func (r *memoryJobsRepository) MarkSubmitted(_ context.Context, id uuid.UUID, ref providers.JobRef) error {
	job := r.jobs[id]
	job.ProviderJobID, job.State = ref.ProviderJobID, ref.State
	r.jobs[id] = job
	return nil
}

func (r *memoryJobsRepository) Get(_ context.Context, id uuid.UUID) (Job, error) {
	return r.jobs[id], nil
}

func (r *memoryJobsRepository) FindUnconsumedByWorkflowNode(
	_ context.Context,
	projectID uuid.UUID,
	runID uuid.UUID,
	node string,
) (Job, error) {
	for id, job := range r.jobs {
		if job.ProjectID == projectID && job.WorkflowRunID == runID && job.WorkflowNode == node && !r.consumed[id] {
			return job, nil
		}
	}
	return Job{}, ErrNotFound
}

func (r *memoryJobsRepository) LeaseUnconsumedDue(_ context.Context, before, _ time.Time, limit int) ([]Job, error) {
	jobs := make([]Job, 0)
	for id, job := range r.jobs {
		if r.consumed[id] || job.UpdatedAt.After(before) {
			continue
		}
		jobs = append(jobs, job)
	}
	if limit > 0 && len(jobs) > limit {
		jobs = jobs[:limit]
	}
	return jobs, nil
}

func (r *memoryJobsRepository) StartAttempt(_ context.Context, id uuid.UUID, operation string, state providers.State) (Attempt, error) {
	r.attemptSeq[id]++
	return Attempt{ID: uuid.New(), ProviderJobID: id, Attempt: r.attemptSeq[id], Operation: operation, State: state}, nil
}

func (r *memoryJobsRepository) FinishAttempt(_ context.Context, _ Attempt) error { return nil }

func (r *memoryJobsRepository) ApplySnapshot(_ context.Context, id uuid.UUID, snapshot providers.Snapshot, rawKey string) error {
	job := r.jobs[id]
	if job.State.Terminal() {
		return nil
	}
	job.State, job.NormalizedOutput, job.RawResponseObjectKey = snapshot.State, snapshot.Output, rawKey
	job.ErrorCode, job.ErrorMessage = snapshot.ErrorCode, snapshot.ErrorMessage
	r.jobs[id] = job
	return nil
}

func (r *memoryJobsRepository) ConsumeTerminal(_ context.Context, id uuid.UUID) (Outcome, bool, error) {
	job := r.jobs[id]
	if !job.State.Terminal() || r.consumed[id] {
		return Outcome{}, false, nil
	}
	r.consumed[id] = true
	return Outcome{JobID: id, ProjectID: job.ProjectID, WorkflowRunID: job.WorkflowRunID, WorkflowNode: job.WorkflowNode,
		State: job.State, Output: job.NormalizedOutput, ErrorCode: job.ErrorCode}, true, nil
}

func (r *memoryJobsRepository) PeekTerminal(_ context.Context, id uuid.UUID) (Outcome, bool, error) {
	job := r.jobs[id]
	if !job.State.Terminal() || r.consumed[id] {
		return Outcome{}, false, nil
	}
	return Outcome{JobID: id, ProjectID: job.ProjectID, WorkflowRunID: job.WorkflowRunID, WorkflowNode: job.WorkflowNode,
		State: job.State, Output: job.NormalizedOutput, ErrorCode: job.ErrorCode}, true, nil
}

func (r *memoryJobsRepository) MarkConsumed(_ context.Context, id uuid.UUID) error {
	r.consumed[id] = true
	return nil
}

type memoryRawStore struct{}

func (memoryRawStore) Put(_ context.Context, job Job, attempt int, _ []byte) (string, error) {
	return "raw/" + job.ID.String() + "/" + string(rune(attempt+'0')), nil
}

type fakeProvider struct {
	ref         providers.JobRef
	snapshot    providers.Snapshot
	submitErr   error
	statusErr   error
	submitCalls int
	statusCalls int
}

func (p *fakeProvider) Submit(_ context.Context, _ providers.SubmitRequest) (providers.JobRef, error) {
	p.submitCalls++
	return p.ref, p.submitErr
}
func (p *fakeProvider) Status(_ context.Context, _ string) (providers.Snapshot, error) {
	p.statusCalls++
	return p.snapshot, p.statusErr
}
func (p *fakeProvider) Cancel(_ context.Context, _ string) error { return nil }

func TestSubmitUsesPersistedIdempotencyKey(t *testing.T) {
	provider := &fakeProvider{ref: providers.JobRef{ProviderJobID: "external-1", State: providers.StateSubmitted, Raw: json.RawMessage(`{"provider_job_id":"external-1","state":"submitted"}`)}}
	repo := newMemoryJobsRepository()
	service := NewService(repo, memoryRawStore{}, providers.Registry{Media: provider}, func() time.Time { return time.Now().UTC() })
	input := validSubmitInput(providers.KindMedia, providers.TaskAudioTranscription)

	first, err := service.Submit(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Submit(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || provider.submitCalls != 1 {
		t.Fatalf("jobs = %s/%s, submit calls = %d", first.ID, second.ID, provider.submitCalls)
	}
}

func TestSubmitRejectsTerminalProviderResponse(t *testing.T) {
	provider := &fakeProvider{ref: providers.JobRef{
		ProviderJobID: "external-1",
		State:         providers.StateSucceeded,
		Raw:           json.RawMessage(`{"provider_job_id":"external-1","state":"succeeded"}`),
	}}
	repo := newMemoryJobsRepository()
	service := NewService(repo, memoryRawStore{}, providers.Registry{Media: provider}, time.Now)

	job, err := service.Submit(context.Background(), validSubmitInput(providers.KindMedia, providers.TaskAudioTranscription))
	if err != nil {
		t.Fatal(err)
	}
	if job.State != providers.StatePermanentlyFailed || job.ErrorCode != "invalid_provider_response" {
		t.Fatalf("job = %#v", job)
	}
}

func TestSubmitRequiresExplicitProviderState(t *testing.T) {
	provider := &fakeProvider{ref: providers.JobRef{ProviderJobID: "external-1"}}
	repo := newMemoryJobsRepository()
	service := NewService(repo, memoryRawStore{}, providers.Registry{Media: provider}, time.Now)

	job, err := service.Submit(context.Background(), validSubmitInput(providers.KindMedia, providers.TaskAudioTranscription))
	if err != nil {
		t.Fatal(err)
	}
	if job.State != providers.StatePermanentlyFailed || job.ErrorCode != "invalid_provider_response" {
		t.Fatalf("job = %#v", job)
	}
}

func TestUnstructuredHTTPFailureRemainsRetryable(t *testing.T) {
	provider := &fakeProvider{submitErr: &providers.RemoteError{StatusCode: 503, Body: "temporarily unavailable"}}
	repo := newMemoryJobsRepository()
	service := NewService(repo, memoryRawStore{}, providers.Registry{Media: provider}, time.Now)

	job, err := service.Submit(context.Background(), validSubmitInput(providers.KindMedia, providers.TaskAudioTranscription))
	if err != nil {
		t.Fatal(err)
	}
	if job.State != providers.StateRetryableFailed || job.ErrorCode != "provider_http_503" {
		t.Fatalf("job = %#v", job)
	}
}

func TestDuplicateTerminalCallbackIsConsumedOnce(t *testing.T) {
	provider := &fakeProvider{ref: providers.JobRef{ProviderJobID: "external-1", State: providers.StateSubmitted}}
	repo := newMemoryJobsRepository()
	service := NewService(repo, memoryRawStore{}, providers.Registry{Media: provider}, func() time.Time { return time.Now().UTC() })
	job, err := service.Submit(context.Background(), validSubmitInput(providers.KindMedia, providers.TaskAudioTranscription))
	if err != nil {
		t.Fatal(err)
	}
	snapshot := providers.Snapshot{
		RequestID: job.RequestID.String(), ProviderJobID: "external-1", State: providers.StateSucceeded,
		Output: json.RawMessage(`{"segments":[{"start_seconds":0,"end_seconds":1,"text":"测试","source_ref":"audio#0-1"}]}`),
		Raw:    json.RawMessage(`{"provider_job_id":"external-1","state":"succeeded"}`),
	}
	if err := service.ApplyCallback(context.Background(), job.ID, snapshot); err != nil {
		t.Fatal(err)
	}
	if err := service.ApplyCallback(context.Background(), job.ID, snapshot); err != nil {
		t.Fatal(err)
	}
	if repo.attemptSeq[job.ID] != 2 {
		t.Fatalf("attempts after duplicate callback = %d, want submit plus one callback", repo.attemptSeq[job.ID])
	}
	_, ok, err := service.ConsumeTerminal(context.Background(), job.ID)
	if err != nil || !ok {
		t.Fatalf("first consume = %v, %v", ok, err)
	}
	_, ok, err = service.ConsumeTerminal(context.Background(), job.ID)
	if err != nil || ok {
		t.Fatalf("second consume = %v, %v", ok, err)
	}
}

func TestDuplicateProcessingCallbackDoesNotCreateAnotherAttempt(t *testing.T) {
	provider := &fakeProvider{ref: providers.JobRef{ProviderJobID: "external-1", State: providers.StateSubmitted}}
	repo := newMemoryJobsRepository()
	service := NewService(repo, memoryRawStore{}, providers.Registry{Media: provider}, time.Now)
	job, err := service.Submit(context.Background(), validSubmitInput(providers.KindMedia, providers.TaskAudioTranscription))
	if err != nil {
		t.Fatal(err)
	}
	snapshot := providers.Snapshot{
		RequestID: job.RequestID.String(), ProviderJobID: "external-1", State: providers.StateProcessing,
	}
	if err := service.ApplyCallback(context.Background(), job.ID, snapshot); err != nil {
		t.Fatal(err)
	}
	if err := service.ApplyCallback(context.Background(), job.ID, snapshot); err != nil {
		t.Fatal(err)
	}
	if repo.attemptSeq[job.ID] != 2 {
		t.Fatalf("attempts after duplicate callback = %d, want submit plus one callback", repo.attemptSeq[job.ID])
	}
}

func TestCallbackRejectsFailureWithoutErrorCode(t *testing.T) {
	provider := &fakeProvider{ref: providers.JobRef{ProviderJobID: "external-1", State: providers.StateSubmitted}}
	repo := newMemoryJobsRepository()
	service := NewService(repo, memoryRawStore{}, providers.Registry{Media: provider}, time.Now)
	job, err := service.Submit(context.Background(), validSubmitInput(providers.KindMedia, providers.TaskAudioTranscription))
	if err != nil {
		t.Fatal(err)
	}
	err = service.ApplyCallback(context.Background(), job.ID, providers.Snapshot{
		RequestID: job.RequestID.String(), ProviderJobID: "external-1", State: providers.StatePermanentlyFailed,
	})
	if err == nil {
		t.Fatal("callback failure without error code was accepted")
	}
	if repo.attemptSeq[job.ID] != 1 {
		t.Fatalf("invalid callback created an attempt: %d", repo.attemptSeq[job.ID])
	}
}

func TestCancelledCallbackDoesNotRequireErrorCode(t *testing.T) {
	provider := &fakeProvider{ref: providers.JobRef{ProviderJobID: "external-1", State: providers.StateSubmitted}}
	repo := newMemoryJobsRepository()
	service := NewService(repo, memoryRawStore{}, providers.Registry{Media: provider}, time.Now)
	job, err := service.Submit(context.Background(), validSubmitInput(providers.KindMedia, providers.TaskAudioTranscription))
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ApplyCallback(context.Background(), job.ID, providers.Snapshot{
		RequestID: job.RequestID.String(), ProviderJobID: "external-1", State: providers.StateCancelled,
	}); err != nil {
		t.Fatal(err)
	}
	updated, err := service.Get(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.State != providers.StateCancelled {
		t.Fatalf("job state = %s", updated.State)
	}
}

func TestInvalidStatusSnapshotArchivesRawResponse(t *testing.T) {
	provider := &fakeProvider{
		ref: providers.JobRef{ProviderJobID: "external-1", State: providers.StateSubmitted},
		snapshot: providers.Snapshot{
			RequestID: "placeholder", ProviderJobID: "external-1", State: providers.State("unknown"),
			Raw: json.RawMessage(`{"provider_job_id":"external-1","state":"unknown"}`),
		},
	}
	repo := newMemoryJobsRepository()
	service := NewService(repo, memoryRawStore{}, providers.Registry{Media: provider}, time.Now)
	job, err := service.Submit(context.Background(), validSubmitInput(providers.KindMedia, providers.TaskAudioTranscription))
	if err != nil {
		t.Fatal(err)
	}
	provider.snapshot.RequestID = job.RequestID.String()
	updated, err := service.Refresh(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.State != providers.StatePermanentlyFailed || updated.ErrorCode != "invalid_provider_response" || updated.RawResponseObjectKey == "" {
		t.Fatalf("job = %#v", updated)
	}
}

func TestCallbackRequiresExactProviderJobIdentity(t *testing.T) {
	provider := &fakeProvider{ref: providers.JobRef{ProviderJobID: "external-1", State: providers.StateSubmitted}}
	repo := newMemoryJobsRepository()
	service := NewService(repo, memoryRawStore{}, providers.Registry{Media: provider}, time.Now)
	job, err := service.Submit(context.Background(), validSubmitInput(providers.KindMedia, providers.TaskAudioTranscription))
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ApplyCallback(context.Background(), job.ID, providers.Snapshot{
		ProviderJobID: "external-1", State: providers.StateProcessing,
	}); err == nil {
		t.Fatal("callback without request ID was accepted")
	}
	if err := service.ApplyCallback(context.Background(), job.ID, providers.Snapshot{
		RequestID: job.RequestID.String(), ProviderJobID: "wrong-job", State: providers.StateProcessing,
	}); err == nil {
		t.Fatal("callback for a different Provider job was accepted")
	}
}

func TestRefreshRejectsSnapshotForDifferentJob(t *testing.T) {
	provider := &fakeProvider{
		ref: providers.JobRef{ProviderJobID: "external-1", State: providers.StateSubmitted},
		snapshot: providers.Snapshot{
			RequestID: "wrong-request", ProviderJobID: "external-1", State: providers.StateProcessing,
		},
	}
	repo := newMemoryJobsRepository()
	service := NewService(repo, memoryRawStore{}, providers.Registry{Media: provider}, time.Now)
	job, err := service.Submit(context.Background(), validSubmitInput(providers.KindMedia, providers.TaskAudioTranscription))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Refresh(context.Background(), job.ID); err == nil {
		t.Fatal("status snapshot for a different request was accepted")
	}
	unchanged, err := service.Get(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.State != providers.StateSubmitted {
		t.Fatalf("job state = %s, want submitted", unchanged.State)
	}
}

func validSubmitInput(kind providers.Kind, task providers.TaskType) SubmitInput {
	return SubmitInput{
		ProjectID: uuid.New(), WorkflowRunID: uuid.New(), WorkflowNode: "transcribe", ProviderKind: kind, TaskType: task,
		Input: json.RawMessage(`{"project_id":"test"}`), CallbackBaseURL: "http://api:8080", Deadline: time.Now().Add(time.Minute),
	}
}
