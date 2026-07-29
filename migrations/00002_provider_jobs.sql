-- +goose Up
CREATE TYPE provider_kind AS ENUM ('media', 'knowledge', 'agent');
CREATE TYPE provider_job_state AS ENUM (
    'pending_submission',
    'submitted',
    'processing',
    'succeeded',
    'retryable_failed',
    'permanently_failed',
    'timed_out',
    'cancelled'
);

CREATE TABLE provider_jobs (
    id uuid PRIMARY KEY,
    request_id uuid NOT NULL UNIQUE,
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    workflow_run_id uuid NOT NULL REFERENCES workflow_runs(id) ON DELETE CASCADE,
    workflow_node_name text NOT NULL,
    provider_kind provider_kind NOT NULL,
    task_type text NOT NULL,
    idempotency_key text NOT NULL UNIQUE,
    provider_job_id text,
    state provider_job_state NOT NULL DEFAULT 'pending_submission',
    input jsonb NOT NULL,
    normalized_output jsonb,
    raw_response_object_key text,
    error_code text,
    error_message text,
    consumed_at timestamptz,
    deadline timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider_kind, provider_job_id)
);

CREATE TABLE provider_attempts (
    id uuid PRIMARY KEY,
    provider_job_id uuid NOT NULL REFERENCES provider_jobs(id) ON DELETE CASCADE,
    attempt integer NOT NULL CHECK (attempt > 0),
    operation text NOT NULL CHECK (operation IN ('submit', 'status', 'callback', 'cancel')),
    state provider_job_state NOT NULL,
    http_status integer,
    raw_response_object_key text,
    error_code text,
    elapsed_ms bigint NOT NULL CHECK (elapsed_ms >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider_job_id, attempt)
);

CREATE INDEX provider_jobs_workflow_node_idx ON provider_jobs(workflow_run_id, workflow_node_name);
CREATE INDEX provider_jobs_poll_idx ON provider_jobs(state, updated_at);

-- +goose Down
DROP TABLE IF EXISTS provider_attempts;
DROP TABLE IF EXISTS provider_jobs;
DROP TYPE IF EXISTS provider_job_state;
DROP TYPE IF EXISTS provider_kind;
