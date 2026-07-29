-- +goose Up
ALTER TABLE projects
    ADD COLUMN owner_staff_id uuid REFERENCES staff(id);

CREATE TABLE consents (
    id uuid PRIMARY KEY,
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    confirmed_by text NOT NULL CHECK (confirmed_by IN ('elder', 'guardian')),
    confirmation_method text NOT NULL CHECK (confirmation_method = 'onsite'),
    staff_id uuid NOT NULL REFERENCES staff(id),
    confirmed_at timestamptz NOT NULL,
    UNIQUE (project_id, confirmed_by, confirmation_method)
);

CREATE INDEX projects_owner_updated_idx
    ON projects(owner_staff_id, updated_at DESC, id DESC);

-- +goose Down
DROP INDEX IF EXISTS projects_owner_updated_idx;
DROP TABLE IF EXISTS consents;
ALTER TABLE projects DROP COLUMN IF EXISTS owner_staff_id;
