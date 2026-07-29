package assets

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Create(ctx context.Context, asset Asset) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO assets (
			id, project_id, kind, filename, content_type, size_bytes,
			object_key, state, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		asset.ID, asset.ProjectID, asset.Kind, asset.Filename, asset.ContentType,
		asset.SizeBytes, asset.ObjectKey, asset.State, asset.CreatedAt,
	)
	return err
}

func (r *PostgresRepository) Get(ctx context.Context, id uuid.UUID) (Asset, error) {
	return scanAsset(r.pool.QueryRow(ctx, `
		SELECT id, project_id, kind, filename, content_type, size_bytes,
		       object_key, sha256, state, created_at, uploaded_at
		FROM assets WHERE id = $1`, id))
}

func (r *PostgresRepository) ListUploadedByKind(ctx context.Context, projectID uuid.UUID, kind Kind) ([]Asset, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, project_id, kind, filename, content_type, size_bytes,
		       object_key, sha256, state, created_at, uploaded_at
		FROM assets
		WHERE project_id = $1 AND kind = $2 AND state = 'uploaded'
		ORDER BY created_at, id`, projectID, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var assets []Asset
	for rows.Next() {
		asset, err := scanAsset(rows)
		if err != nil {
			return nil, err
		}
		assets = append(assets, asset)
	}
	return assets, rows.Err()
}

func (r *PostgresRepository) MarkUploaded(ctx context.Context, id uuid.UUID, sha256 string, uploadedAt time.Time) (Asset, error) {
	asset, err := scanAsset(r.pool.QueryRow(ctx, `
		UPDATE assets
		SET sha256 = $2, state = 'uploaded', uploaded_at = $3
		WHERE id = $1
		  AND (state = 'pending_upload' OR (state = 'uploaded' AND sha256 = $2))
		RETURNING id, project_id, kind, filename, content_type, size_bytes,
		          object_key, sha256, state, created_at, uploaded_at`,
		id, sha256, uploadedAt,
	))
	if errors.Is(err, ErrNotFound) {
		if _, getErr := r.Get(ctx, id); errors.Is(getErr, ErrNotFound) {
			return Asset{}, ErrNotFound
		}
		return Asset{}, ErrHashConflict
	}
	return asset, err
}

type rowScanner interface {
	Scan(...any) error
}

func scanAsset(row rowScanner) (Asset, error) {
	var asset Asset
	var kind, state string
	var sha256 pgtype.Text
	var uploadedAt pgtype.Timestamptz
	err := row.Scan(
		&asset.ID, &asset.ProjectID, &kind, &asset.Filename, &asset.ContentType,
		&asset.SizeBytes, &asset.ObjectKey, &sha256, &state, &asset.CreatedAt, &uploadedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Asset{}, ErrNotFound
	}
	if err != nil {
		return Asset{}, err
	}
	asset.Kind = Kind(kind)
	asset.State = State(state)
	if sha256.Valid {
		asset.SHA256 = sha256.String
	}
	if uploadedAt.Valid {
		value := uploadedAt.Time
		asset.UploadedAt = &value
	}
	return asset, nil
}
