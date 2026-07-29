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
