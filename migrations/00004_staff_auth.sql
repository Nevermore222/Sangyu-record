-- +goose Up
CREATE TYPE staff_state AS ENUM ('active', 'disabled');

CREATE TABLE staff (
    id uuid PRIMARY KEY,
    wechat_openid text NOT NULL UNIQUE,
    display_name text NOT NULL,
    team_name text NOT NULL DEFAULT '',
    state staff_state NOT NULL DEFAULT 'active',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE staff_sessions (
    id uuid PRIMARY KEY,
    staff_id uuid NOT NULL REFERENCES staff(id) ON DELETE CASCADE,
    token_hash char(64) NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX staff_sessions_expiry_idx ON staff_sessions(expires_at);

-- +goose Down
DROP TABLE IF EXISTS staff_sessions;
DROP TABLE IF EXISTS staff;
DROP TYPE IF EXISTS staff_state;
