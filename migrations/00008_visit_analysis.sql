-- +goose Up
CREATE TABLE visit_analyses (
    id uuid PRIMARY KEY,
    visit_id uuid NOT NULL UNIQUE REFERENCES visits(id) ON DELETE CASCADE,
    workflow_run_id uuid NOT NULL UNIQUE REFERENCES workflow_runs(id) ON DELETE CASCADE,
    summary text NOT NULL,
    covered_items jsonb NOT NULL,
    gaps jsonb NOT NULL,
    followup_questions jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE collection_plan_items
    ADD COLUMN gap_reason text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE collection_plan_items DROP COLUMN IF EXISTS gap_reason;
DROP TABLE IF EXISTS visit_analyses;
