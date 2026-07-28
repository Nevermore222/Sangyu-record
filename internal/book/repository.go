package book

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrArtifactNotFound   = errors.New("artifact not found")
	ErrManuscriptNotFound = errors.New("manuscript not found")
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) LoadManuscript(ctx context.Context, runID uuid.UUID) (Manuscript, error) {
	var encoded []byte
	err := r.pool.QueryRow(ctx, `
		SELECT output FROM workflow_nodes
		WHERE run_id = $1 AND node_name = 'write_book' AND state = 'succeeded'`, runID).Scan(&encoded)
	if errors.Is(err, pgx.ErrNoRows) {
		return Manuscript{}, ErrManuscriptNotFound
	}
	if err != nil {
		return Manuscript{}, err
	}
	var wrapper struct {
		Output json.RawMessage `json:"output"`
	}
	if err := json.Unmarshal(encoded, &wrapper); err != nil {
		return Manuscript{}, err
	}
	var manuscript Manuscript
	if err := json.Unmarshal(wrapper.Output, &manuscript); err != nil {
		return Manuscript{}, err
	}
	return manuscript, nil
}

func (r *PostgresRepository) NextVersion(ctx context.Context, projectID uuid.UUID, kind string) (int, error) {
	var version int
	err := r.pool.QueryRow(ctx, `SELECT coalesce(max(version), 0) + 1 FROM artifacts WHERE project_id = $1 AND kind = $2`, projectID, kind).Scan(&version)
	return version, err
}

func (r *PostgresRepository) Save(ctx context.Context, artifact Artifact) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO artifacts (
			id, project_id, workflow_run_id, version, kind, object_key,
			content_type, size_bytes, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		artifact.ID, artifact.ProjectID, artifact.WorkflowRunID, artifact.Version,
		artifact.Kind, artifact.ObjectKey, artifact.ContentType, artifact.SizeBytes, artifact.CreatedAt,
	)
	return err
}

func (r *PostgresRepository) Latest(ctx context.Context, projectID uuid.UUID) (Artifact, error) {
	var artifact Artifact
	err := r.pool.QueryRow(ctx, `
		SELECT id, project_id, workflow_run_id, version, kind, object_key,
		       content_type, size_bytes, created_at
		FROM artifacts WHERE project_id = $1 AND kind = 'pdf'
		ORDER BY version DESC LIMIT 1`, projectID).Scan(
		&artifact.ID, &artifact.ProjectID, &artifact.WorkflowRunID, &artifact.Version,
		&artifact.Kind, &artifact.ObjectKey, &artifact.ContentType, &artifact.SizeBytes, &artifact.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Artifact{}, ErrArtifactNotFound
	}
	return artifact, err
}
