-- +goose Up
CREATE TYPE workflow_kind AS ENUM ('book', 'visit_analysis');

ALTER TABLE workflow_runs ADD COLUMN kind workflow_kind NOT NULL DEFAULT 'book';
ALTER TABLE workflow_runs ADD COLUMN visit_id uuid REFERENCES visits(id) ON DELETE CASCADE;
ALTER TABLE workflow_nodes ADD COLUMN position integer;

UPDATE workflow_nodes SET position = CASE node_name
    WHEN 'transcribe' THEN 0
    WHEN 'understand_photo' THEN 1
    WHEN 'build_memory' THEN 2
    WHEN 'retrieve_shared_memory' THEN 3
    WHEN 'plan_book' THEN 4
    WHEN 'write_book' THEN 5
    WHEN 'render_pdf' THEN 6
END;

ALTER TABLE workflow_nodes ALTER COLUMN position SET NOT NULL;
ALTER TABLE workflow_nodes ADD CONSTRAINT workflow_nodes_position_nonnegative CHECK (position >= 0);
CREATE UNIQUE INDEX workflow_nodes_position_idx ON workflow_nodes(run_id, position);
CREATE INDEX workflow_runs_visit_created_idx ON workflow_runs(visit_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS workflow_runs_visit_created_idx;
DROP INDEX IF EXISTS workflow_nodes_position_idx;
ALTER TABLE workflow_nodes DROP CONSTRAINT IF EXISTS workflow_nodes_position_nonnegative;
ALTER TABLE workflow_nodes DROP COLUMN IF EXISTS position;
ALTER TABLE workflow_runs DROP COLUMN IF EXISTS visit_id;
ALTER TABLE workflow_runs DROP COLUMN IF EXISTS kind;
DROP TYPE IF EXISTS workflow_kind;
