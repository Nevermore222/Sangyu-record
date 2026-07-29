package providerjobs

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nevermore222/sangyu-record/internal/providers"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) CreateOrGet(ctx context.Context, input CreateInput) (Job, bool, error) {
	job := Job{
		ID: uuid.New(), RequestID: input.RequestID, ProjectID: input.ProjectID, WorkflowRunID: input.WorkflowRunID,
		WorkflowNode: input.WorkflowNode, ProviderKind: input.ProviderKind, TaskType: input.TaskType,
		IdempotencyKey: input.IdempotencyKey, State: providers.StatePendingSubmission, Input: input.Input, Deadline: input.Deadline,
	}
	result, err := r.pool.Exec(ctx, `
		INSERT INTO provider_jobs (
			id, request_id, project_id, workflow_run_id, workflow_node_name, provider_kind,
			task_type, idempotency_key, state, input, deadline
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (idempotency_key) DO NOTHING`,
		job.ID, job.RequestID, job.ProjectID, job.WorkflowRunID, job.WorkflowNode, job.ProviderKind,
		job.TaskType, job.IdempotencyKey, job.State, job.Input, job.Deadline)
	if err != nil {
		return Job{}, false, err
	}
	if result.RowsAffected() == 1 {
		createdJob, err := r.Get(ctx, job.ID)
		return createdJob, true, err
	}
	existing, err := r.getByIdempotencyKey(ctx, input.IdempotencyKey)
	return existing, false, err
}

func (r *PostgresRepository) MarkSubmitted(ctx context.Context, id uuid.UUID, ref providers.JobRef) error {
	if ref.ProviderJobID == "" {
		return errors.New("provider job ID is required")
	}
	result, err := r.pool.Exec(ctx, `
		UPDATE provider_jobs SET provider_job_id=$2, state=$3, error_code=NULL, error_message=NULL, updated_at=now()
		WHERE id=$1 AND state IN ('pending_submission','retryable_failed') AND provider_job_id IS NULL`,
		id, ref.ProviderJobID, ref.State)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 1 {
		return nil
	}
	job, err := r.Get(ctx, id)
	if err == nil && job.ProviderJobID == ref.ProviderJobID {
		return nil
	}
	if err != nil {
		return err
	}
	return ErrTerminalConflict
}

func (r *PostgresRepository) Get(ctx context.Context, id uuid.UUID) (Job, error) {
	return scanJob(r.pool.QueryRow(ctx, selectJob+" WHERE id=$1", id))
}

func (r *PostgresRepository) FindUnconsumedByWorkflowNode(
	ctx context.Context,
	projectID uuid.UUID,
	runID uuid.UUID,
	node string,
) (Job, error) {
	return scanJob(r.pool.QueryRow(ctx, selectJob+`
		WHERE project_id=$1 AND workflow_run_id=$2 AND workflow_node_name=$3 AND consumed_at IS NULL`,
		projectID, runID, node))
}

func (r *PostgresRepository) ListUnconsumedDue(ctx context.Context, before time.Time, limit int) ([]Job, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, selectJob+`
		WHERE consumed_at IS NULL AND updated_at <= $1
		ORDER BY updated_at, id
		LIMIT $2`, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := make([]Job, 0)
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (r *PostgresRepository) getByIdempotencyKey(ctx context.Context, key string) (Job, error) {
	return scanJob(r.pool.QueryRow(ctx, selectJob+" WHERE idempotency_key=$1", key))
}

func (r *PostgresRepository) StartAttempt(ctx context.Context, jobID uuid.UUID, operation string, state providers.State) (Attempt, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Attempt{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := tx.QueryRow(ctx, "SELECT id FROM provider_jobs WHERE id=$1 FOR UPDATE", jobID).Scan(&jobID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Attempt{}, ErrNotFound
		}
		return Attempt{}, err
	}
	attempt := Attempt{ID: uuid.New(), ProviderJobID: jobID, Operation: operation, State: state, CreatedAt: time.Now().UTC()}
	if err := tx.QueryRow(ctx, "SELECT coalesce(max(attempt),0)+1 FROM provider_attempts WHERE provider_job_id=$1", jobID).Scan(&attempt.Attempt); err != nil {
		return Attempt{}, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO provider_attempts (id, provider_job_id, attempt, operation, state, elapsed_ms, created_at)
		VALUES ($1,$2,$3,$4,$5,0,$6)`, attempt.ID, attempt.ProviderJobID, attempt.Attempt, attempt.Operation, attempt.State, attempt.CreatedAt)
	if err != nil {
		return Attempt{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Attempt{}, err
	}
	return attempt, nil
}

func (r *PostgresRepository) FinishAttempt(ctx context.Context, attempt Attempt) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE provider_attempts SET state=$2, http_status=NULLIF($3,0),
			raw_response_object_key=NULLIF($4,''), error_code=NULLIF($5,''), elapsed_ms=$6
		WHERE id=$1`, attempt.ID, attempt.State, attempt.HTTPStatus, attempt.RawResponseObjectKey, attempt.ErrorCode, attempt.ElapsedMS)
	return err
}

func (r *PostgresRepository) ApplySnapshot(ctx context.Context, id uuid.UUID, snapshot providers.Snapshot, rawKey string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var current string
	if err := tx.QueryRow(ctx, "SELECT state FROM provider_jobs WHERE id=$1 FOR UPDATE", id).Scan(&current); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	currentState := providers.State(current)
	if currentState.Terminal() {
		if currentState == snapshot.State {
			return tx.Commit(ctx)
		}
		return ErrTerminalConflict
	}
	_, err = tx.Exec(ctx, `
		UPDATE provider_jobs SET state=$2, normalized_output=$3, raw_response_object_key=NULLIF($4,''),
			error_code=NULLIF($5,''), error_message=NULLIF($6,''),
			provider_job_id=coalesce(provider_job_id, NULLIF($7,'')), updated_at=now()
		WHERE id=$1`, id, snapshot.State, nullJSON(snapshot.Output), rawKey, snapshot.ErrorCode, snapshot.ErrorMessage, snapshot.ProviderJobID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) ConsumeTerminal(ctx context.Context, id uuid.UUID) (Outcome, bool, error) {
	var outcome Outcome
	var state string
	var output []byte
	err := r.pool.QueryRow(ctx, `
		UPDATE provider_jobs SET consumed_at=now(), updated_at=now()
		WHERE id=$1 AND consumed_at IS NULL
		  AND state IN ('succeeded','permanently_failed','timed_out','cancelled')
		RETURNING id, project_id, workflow_run_id, workflow_node_name, state,
		          coalesce(normalized_output,'null'::jsonb), coalesce(error_code,'')`, id,
	).Scan(&outcome.JobID, &outcome.ProjectID, &outcome.WorkflowRunID, &outcome.WorkflowNode, &state, &output, &outcome.ErrorCode)
	if errors.Is(err, pgx.ErrNoRows) {
		return Outcome{}, false, nil
	}
	if err != nil {
		return Outcome{}, false, err
	}
	outcome.State, outcome.Output = providers.State(state), output
	return outcome, true, nil
}

func (r *PostgresRepository) PeekTerminal(ctx context.Context, id uuid.UUID) (Outcome, bool, error) {
	var outcome Outcome
	var state string
	var output []byte
	err := r.pool.QueryRow(ctx, `
		SELECT id, project_id, workflow_run_id, workflow_node_name, state,
		       coalesce(normalized_output,'null'::jsonb), coalesce(error_code,'')
		FROM provider_jobs
		WHERE id=$1 AND consumed_at IS NULL
		  AND state IN ('succeeded','permanently_failed','timed_out','cancelled')`, id,
	).Scan(&outcome.JobID, &outcome.ProjectID, &outcome.WorkflowRunID, &outcome.WorkflowNode, &state, &output, &outcome.ErrorCode)
	if errors.Is(err, pgx.ErrNoRows) {
		return Outcome{}, false, nil
	}
	if err != nil {
		return Outcome{}, false, err
	}
	outcome.State, outcome.Output = providers.State(state), output
	return outcome, true, nil
}

func (r *PostgresRepository) MarkConsumed(ctx context.Context, id uuid.UUID) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE provider_jobs SET consumed_at=coalesce(consumed_at, now()), updated_at=now()
		WHERE id=$1 AND state IN ('succeeded','permanently_failed','timed_out','cancelled')`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrTerminalConflict
	}
	return nil
}

const selectJob = `SELECT id, request_id, project_id, workflow_run_id, workflow_node_name,
	provider_kind, task_type, idempotency_key, coalesce(provider_job_id,''), state, input,
	coalesce(normalized_output,'null'::jsonb), coalesce(raw_response_object_key,''),
	coalesce(error_code,''), coalesce(error_message,''), deadline, created_at, updated_at
	FROM provider_jobs`

type rowScanner interface {
	Scan(...any) error
}

func scanJob(row rowScanner) (Job, error) {
	var job Job
	var kind, task, state string
	err := row.Scan(
		&job.ID, &job.RequestID, &job.ProjectID, &job.WorkflowRunID, &job.WorkflowNode,
		&kind, &task, &job.IdempotencyKey, &job.ProviderJobID, &state, &job.Input,
		&job.NormalizedOutput, &job.RawResponseObjectKey, &job.ErrorCode, &job.ErrorMessage,
		&job.Deadline, &job.CreatedAt, &job.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrNotFound
	}
	job.ProviderKind, job.TaskType, job.State = providers.Kind(kind), providers.TaskType(task), providers.State(state)
	return job, err
}

func nullJSON(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return value
}
