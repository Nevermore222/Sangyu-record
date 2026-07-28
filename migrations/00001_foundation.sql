-- +goose Up
CREATE TYPE project_state AS ENUM (
    'collecting',
    'processing',
    'needs_material',
    'generating',
    'quality_check',
    'exception',
    'pdf_rendering',
    'completed'
);

CREATE TYPE plan_item_status AS ENUM ('pending', 'collected', 'insufficient', 'not_needed');
CREATE TYPE asset_kind AS ENUM ('audio', 'photo', 'staff_note');
CREATE TYPE asset_state AS ENUM ('pending_upload', 'uploaded', 'verified', 'rejected');
CREATE TYPE node_state AS ENUM ('queued', 'running', 'succeeded', 'failed');

CREATE TABLE projects (
    id uuid PRIMARY KEY,
    display_name text NOT NULL,
    birth_year integer NOT NULL CHECK (birth_year >= 1900),
    birth_place text NOT NULL,
    long_term_residence text NOT NULL,
    primary_occupation text NOT NULL DEFAULT '',
    target_edition text NOT NULL CHECK (target_edition IN ('brief', 'standard', 'long')),
    state project_state NOT NULL DEFAULT 'collecting',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE collection_plan_items (
    id uuid PRIMARY KEY,
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    category text NOT NULL,
    prompt text NOT NULL,
    required boolean NOT NULL,
    status plan_item_status NOT NULL DEFAULT 'pending',
    position integer NOT NULL CHECK (position >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (project_id, position)
);

CREATE TABLE assets (
    id uuid PRIMARY KEY,
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    kind asset_kind NOT NULL,
    filename text NOT NULL,
    content_type text NOT NULL,
    size_bytes bigint NOT NULL CHECK (size_bytes > 0),
    object_key text NOT NULL UNIQUE,
    sha256 char(64),
    state asset_state NOT NULL DEFAULT 'pending_upload',
    created_at timestamptz NOT NULL DEFAULT now(),
    uploaded_at timestamptz,
    CHECK ((state = 'pending_upload' AND sha256 IS NULL) OR state <> 'pending_upload')
);

CREATE TABLE workflow_runs (
    id uuid PRIMARY KEY,
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    state node_state NOT NULL DEFAULT 'queued',
    error_code text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE workflow_nodes (
    id uuid PRIMARY KEY,
    run_id uuid NOT NULL REFERENCES workflow_runs(id) ON DELETE CASCADE,
    node_name text NOT NULL,
    state node_state NOT NULL DEFAULT 'queued',
    output jsonb,
    error_code text,
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (run_id, node_name)
);

CREATE TABLE artifacts (
    id uuid PRIMARY KEY,
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    workflow_run_id uuid NOT NULL REFERENCES workflow_runs(id) ON DELETE CASCADE,
    version integer NOT NULL CHECK (version > 0),
    kind text NOT NULL CHECK (kind IN ('manuscript', 'pdf')),
    object_key text NOT NULL UNIQUE,
    content_type text NOT NULL,
    size_bytes bigint NOT NULL CHECK (size_bytes > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (project_id, kind, version)
);

CREATE INDEX assets_project_id_idx ON assets(project_id);
CREATE INDEX workflow_runs_project_id_idx ON workflow_runs(project_id);
CREATE INDEX artifacts_project_id_idx ON artifacts(project_id);

-- +goose Down
DROP TABLE IF EXISTS artifacts;
DROP TABLE IF EXISTS workflow_nodes;
DROP TABLE IF EXISTS workflow_runs;
DROP TABLE IF EXISTS assets;
DROP TABLE IF EXISTS collection_plan_items;
DROP TABLE IF EXISTS projects;
DROP TYPE IF EXISTS node_state;
DROP TYPE IF EXISTS asset_state;
DROP TYPE IF EXISTS asset_kind;
DROP TYPE IF EXISTS plan_item_status;
DROP TYPE IF EXISTS project_state;
