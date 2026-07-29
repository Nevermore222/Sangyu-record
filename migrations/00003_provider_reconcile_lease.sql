-- +goose Up
ALTER TABLE provider_jobs
    ADD COLUMN next_reconcile_at timestamptz NOT NULL DEFAULT '-infinity';

CREATE INDEX provider_jobs_reconcile_idx
    ON provider_jobs(next_reconcile_at, updated_at)
    WHERE consumed_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS provider_jobs_reconcile_idx;
ALTER TABLE provider_jobs DROP COLUMN IF EXISTS next_reconcile_at;
