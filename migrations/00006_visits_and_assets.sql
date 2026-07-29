-- +goose Up
CREATE TYPE visit_state AS ENUM ('draft', 'submitted', 'analyzing', 'completed', 'failed');
CREATE TYPE asset_source AS ENUM ('direct', 'wechat_file', 'album', 'camera');

CREATE TABLE visits (
    id uuid PRIMARY KEY,
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    sequence integer NOT NULL CHECK (sequence > 0),
    staff_id uuid NOT NULL REFERENCES staff(id),
    visited_at timestamptz NOT NULL,
    location text NOT NULL DEFAULT '',
    notes text NOT NULL DEFAULT '',
    state visit_state NOT NULL DEFAULT 'draft',
    error_code text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (project_id, sequence)
);

CREATE TABLE visit_plan_items (
    visit_id uuid NOT NULL REFERENCES visits(id) ON DELETE CASCADE,
    plan_item_id uuid NOT NULL REFERENCES collection_plan_items(id) ON DELETE CASCADE,
    PRIMARY KEY (visit_id, plan_item_id)
);

ALTER TABLE assets ADD COLUMN visit_id uuid REFERENCES visits(id) ON DELETE CASCADE;
ALTER TABLE assets ADD COLUMN source asset_source;
ALTER TABLE assets ADD COLUMN display_name text NOT NULL DEFAULT '';

CREATE TABLE asset_plan_items (
    asset_id uuid NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    plan_item_id uuid NOT NULL REFERENCES collection_plan_items(id) ON DELETE CASCADE,
    PRIMARY KEY (asset_id, plan_item_id)
);

CREATE INDEX visits_project_sequence_idx ON visits(project_id, sequence DESC);
CREATE INDEX assets_visit_created_idx ON assets(visit_id, created_at, id);

-- +goose Down
DROP INDEX IF EXISTS assets_visit_created_idx;
DROP INDEX IF EXISTS visits_project_sequence_idx;
DROP TABLE IF EXISTS asset_plan_items;
ALTER TABLE assets DROP COLUMN IF EXISTS display_name;
ALTER TABLE assets DROP COLUMN IF EXISTS source;
ALTER TABLE assets DROP COLUMN IF EXISTS visit_id;
DROP TABLE IF EXISTS visit_plan_items;
DROP TABLE IF EXISTS visits;
DROP TYPE IF EXISTS asset_source;
DROP TYPE IF EXISTS visit_state;
