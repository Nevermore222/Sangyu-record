package providerjobs

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nevermore222/sangyu-record/internal/providers"
	"github.com/nevermore222/sangyu-record/internal/testdb"
)

func TestTerminalProviderJobIsConsumedOnce(t *testing.T) {
	pool := openProviderJobsTestPool(t)
	projectID, runID := insertProviderJobParents(t, pool)
	repo := NewPostgresRepository(pool)
	job, created, err := repo.CreateOrGet(context.Background(), CreateInput{
		RequestID: uuid.New(), ProjectID: projectID, WorkflowRunID: runID,
		WorkflowNode: "transcribe", ProviderKind: providers.KindMedia,
		TaskType: providers.TaskAudioTranscription, IdempotencyKey: runID.String() + ":transcribe",
		Input: json.RawMessage(`{"project_id":"test"}`), Deadline: time.Now().Add(time.Minute),
	})
	if err != nil || !created {
		t.Fatalf("create = %#v, %v, %v", job, created, err)
	}
	if err := repo.MarkSubmitted(context.Background(), job.ID, providers.JobRef{ProviderJobID: "external-" + job.ID.String(), State: providers.StateSubmitted}); err != nil {
		t.Fatal(err)
	}
	if err := repo.ApplySnapshot(context.Background(), job.ID, providers.Snapshot{
		RequestID: job.RequestID.String(), ProviderJobID: "external-" + job.ID.String(), State: providers.StateSucceeded,
		Output: json.RawMessage(`{"segments":[{"start_seconds":0,"end_seconds":1,"text":"测试","source_ref":"audio#0-1"}]}`),
	}, "raw/provider-response.json"); err != nil {
		t.Fatal(err)
	}

	outcome, consumed, err := repo.ConsumeTerminal(context.Background(), job.ID)
	if err != nil || !consumed || outcome.WorkflowNode != "transcribe" {
		t.Fatalf("first consume = %#v, %v, %v", outcome, consumed, err)
	}
	_, consumed, err = repo.ConsumeTerminal(context.Background(), job.ID)
	if err != nil || consumed {
		t.Fatalf("second consume = %v, %v", consumed, err)
	}
}

func TestTerminalOutcomeRemainsUntilAcknowledged(t *testing.T) {
	pool := openProviderJobsTestPool(t)
	projectID, runID := insertProviderJobParents(t, pool)
	repo := NewPostgresRepository(pool)
	job, _, err := repo.CreateOrGet(context.Background(), CreateInput{
		RequestID: uuid.New(), ProjectID: projectID, WorkflowRunID: runID,
		WorkflowNode: "transcribe", ProviderKind: providers.KindMedia,
		TaskType: providers.TaskAudioTranscription, IdempotencyKey: runID.String() + ":transcribe",
		Input: json.RawMessage(`{}`), Deadline: time.Now().Add(time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.ApplySnapshot(context.Background(), job.ID, providers.Snapshot{
		ProviderJobID: "external-ack", State: providers.StateSucceeded,
		Output: json.RawMessage(`{"segments":[{"source_ref":"audio#0-1"}]}`),
	}, "raw/status.json"); err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if _, available, err := repo.PeekTerminal(context.Background(), job.ID); err != nil || !available {
			t.Fatalf("peek %d = %v, %v", attempt, available, err)
		}
	}
	due, err := repo.ListUnconsumedDue(context.Background(), time.Now().Add(time.Second), 100)
	if err != nil || !containsJob(due, job.ID) {
		t.Fatalf("due before acknowledgement = %#v, %v", due, err)
	}
	if err := repo.MarkConsumed(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
	if _, available, err := repo.PeekTerminal(context.Background(), job.ID); err != nil || available {
		t.Fatalf("peek after acknowledgement = %v, %v", available, err)
	}
	due, err = repo.ListUnconsumedDue(context.Background(), time.Now().Add(time.Second), 100)
	if err != nil || containsJob(due, job.ID) {
		t.Fatalf("due after acknowledgement = %#v, %v", due, err)
	}
}

func containsJob(jobs []Job, id uuid.UUID) bool {
	for _, job := range jobs {
		if job.ID == id {
			return true
		}
	}
	return false
}

func TestCreateOrGetUsesIdempotencyKey(t *testing.T) {
	pool := openProviderJobsTestPool(t)
	projectID, runID := insertProviderJobParents(t, pool)
	repo := NewPostgresRepository(pool)
	input := CreateInput{
		RequestID: uuid.New(), ProjectID: projectID, WorkflowRunID: runID, WorkflowNode: "transcribe",
		ProviderKind: providers.KindMedia, TaskType: providers.TaskAudioTranscription,
		IdempotencyKey: runID.String() + ":transcribe", Input: json.RawMessage(`{}`), Deadline: time.Now().Add(time.Minute),
	}
	first, created, err := repo.CreateOrGet(context.Background(), input)
	if err != nil || !created {
		t.Fatalf("first = %#v, %v, %v", first, created, err)
	}
	input.RequestID = uuid.New()
	second, created, err := repo.CreateOrGet(context.Background(), input)
	if err != nil || created || first.ID != second.ID {
		t.Fatalf("second = %#v, %v, %v", second, created, err)
	}
}

func TestRetryableSubmissionCanStoreExternalJobID(t *testing.T) {
	pool := openProviderJobsTestPool(t)
	projectID, runID := insertProviderJobParents(t, pool)
	repo := NewPostgresRepository(pool)
	job, _, err := repo.CreateOrGet(context.Background(), CreateInput{
		RequestID: uuid.New(), ProjectID: projectID, WorkflowRunID: runID, WorkflowNode: "transcribe",
		ProviderKind: providers.KindMedia, TaskType: providers.TaskAudioTranscription,
		IdempotencyKey: runID.String() + ":transcribe", Input: json.RawMessage(`{}`), Deadline: time.Now().Add(time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.ApplySnapshot(context.Background(), job.ID, providers.Snapshot{
		State: providers.StateRetryableFailed, ErrorCode: "provider_busy",
	}, ""); err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkSubmitted(context.Background(), job.ID, providers.JobRef{
		ProviderJobID: "external-retry", State: providers.StateSubmitted,
	}); err != nil {
		t.Fatal(err)
	}
	updated, err := repo.Get(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ProviderJobID != "external-retry" || updated.State != providers.StateSubmitted {
		t.Fatalf("updated job = %#v", updated)
	}
}

func TestCallbackCanArriveBeforeSubmitResponseIsPersisted(t *testing.T) {
	pool := openProviderJobsTestPool(t)
	projectID, runID := insertProviderJobParents(t, pool)
	repo := NewPostgresRepository(pool)
	job, _, err := repo.CreateOrGet(context.Background(), CreateInput{
		RequestID: uuid.New(), ProjectID: projectID, WorkflowRunID: runID, WorkflowNode: "transcribe",
		ProviderKind: providers.KindMedia, TaskType: providers.TaskAudioTranscription,
		IdempotencyKey: runID.String() + ":transcribe", Input: json.RawMessage(`{}`), Deadline: time.Now().Add(time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	snapshot := providers.Snapshot{
		RequestID: job.RequestID.String(), ProviderJobID: "external-fast", State: providers.StateSucceeded,
		Output: json.RawMessage(`{"segments":[{"start_seconds":0,"end_seconds":1,"text":"test","source_ref":"audio#0-1"}]}`),
	}
	if err := repo.ApplySnapshot(context.Background(), job.ID, snapshot, "raw/callback.json"); err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkSubmitted(context.Background(), job.ID, providers.JobRef{
		ProviderJobID: "external-fast", State: providers.StateSubmitted,
	}); err != nil {
		t.Fatal(err)
	}
	updated, err := repo.Get(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ProviderJobID != "external-fast" || updated.State != providers.StateSucceeded {
		t.Fatalf("updated job = %#v", updated)
	}
}

func openProviderJobsTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	testdb.Serialize(t, pool)
	return pool
}

func insertProviderJobParents(t *testing.T, pool *pgxpool.Pool) (uuid.UUID, uuid.UUID) {
	t.Helper()
	projectID, runID := uuid.New(), uuid.New()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO projects (id, display_name, birth_year, birth_place, long_term_residence, target_edition)
		VALUES ($1, 'Provider Test', 1948, 'Suzhou', 'Suzhou', 'brief')`, projectID)
	if err == nil {
		_, err = pool.Exec(context.Background(), `INSERT INTO workflow_runs (id, project_id) VALUES ($1, $2)`, runID, projectID)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DELETE FROM projects WHERE id = $1", projectID) })
	return projectID, runID
}
