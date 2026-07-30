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
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if asset.VisitID != nil {
		var exists bool
		err := tx.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM visits
				WHERE id=$1 AND project_id=$2 AND state='draft'
			)`, *asset.VisitID, asset.ProjectID).Scan(&exists)
		if err != nil {
			return err
		}
		if !exists {
			return ErrInvalidState
		}
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO assets (
			id, project_id, visit_id, kind, source, filename, display_name,
			content_type, size_bytes, object_key, state, created_at
		) VALUES ($1, $2, $3, $4, NULLIF($5, '')::asset_source, $6, $7, $8, $9, $10, $11, $12)`,
		asset.ID, asset.ProjectID, asset.VisitID, asset.Kind, asset.Source,
		asset.Filename, asset.DisplayName, asset.ContentType, asset.SizeBytes,
		asset.ObjectKey, asset.State, asset.CreatedAt,
	)
	if err != nil {
		return err
	}
	for _, planItemID := range asset.PlanItemIDs {
		result, err := tx.Exec(ctx, `
			INSERT INTO asset_plan_items (asset_id, plan_item_id)
			SELECT $1, id FROM collection_plan_items
			WHERE id=$2 AND project_id=$3`, asset.ID, planItemID, asset.ProjectID)
		if err != nil {
			return err
		}
		if result.RowsAffected() != 1 {
			return ErrValidation
		}
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) Get(ctx context.Context, id uuid.UUID) (Asset, error) {
	return scanAsset(r.pool.QueryRow(ctx, `
		SELECT id, project_id, visit_id, kind, source, filename, display_name, content_type, size_bytes,
		       object_key, sha256, state, created_at, uploaded_at
		FROM assets WHERE id = $1`, id))
}

func (r *PostgresRepository) ListUploadedByKind(ctx context.Context, projectID uuid.UUID, kind Kind) ([]Asset, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, project_id, visit_id, kind, source, filename, display_name, content_type, size_bytes,
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

func (r *PostgresRepository) ListUploadedByVisitAndKind(ctx context.Context, visitID uuid.UUID, kind Kind) ([]Asset, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, project_id, visit_id, kind, source, filename, display_name, content_type, size_bytes,
		       object_key, sha256, state, created_at, uploaded_at
		FROM assets
		WHERE visit_id=$1 AND kind=$2 AND state='uploaded'
		ORDER BY created_at, id`, visitID, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Asset, 0)
	for rows.Next() {
		asset, err := scanAsset(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, asset)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) MarkUploaded(ctx context.Context, id uuid.UUID, sha256 string, uploadedAt time.Time) (Asset, error) {
	asset, err := scanAsset(r.pool.QueryRow(ctx, `
		UPDATE assets
		SET sha256 = $2, state = 'uploaded', uploaded_at = $3
		WHERE id = $1
		  AND (state = 'pending_upload' OR (state = 'uploaded' AND sha256 = $2))
		RETURNING id, project_id, visit_id, kind, source, filename, display_name, content_type, size_bytes,
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

func (r *PostgresRepository) ListByVisit(ctx context.Context, visitID uuid.UUID) ([]Asset, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, project_id, visit_id, kind, source, filename, display_name, content_type, size_bytes,
		       object_key, sha256, state, created_at, uploaded_at
		FROM assets WHERE visit_id=$1 ORDER BY created_at, id`, visitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Asset, 0)
	for rows.Next() {
		asset, err := scanAsset(rows)
		if err != nil {
			return nil, err
		}
		asset.PlanItemIDs, err = r.loadPlanItems(ctx, asset.ID)
		if err != nil {
			return nil, err
		}
		items = append(items, asset)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) DeletePending(ctx context.Context, id uuid.UUID) (Asset, error) {
	asset, err := scanAsset(r.pool.QueryRow(ctx, `
		DELETE FROM assets WHERE id=$1 AND state='pending_upload'
		RETURNING id, project_id, visit_id, kind, source, filename, display_name, content_type, size_bytes,
		          object_key, sha256, state, created_at, uploaded_at`, id))
	if errors.Is(err, ErrNotFound) {
		if _, getErr := r.Get(ctx, id); errors.Is(getErr, ErrNotFound) {
			return Asset{}, ErrNotFound
		}
		return Asset{}, ErrInvalidState
	}
	return asset, err
}

func (r *PostgresRepository) loadPlanItems(ctx context.Context, assetID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `SELECT plan_item_id FROM asset_plan_items WHERE asset_id=$1 ORDER BY plan_item_id`, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		items = append(items, id)
	}
	return items, rows.Err()
}

type rowScanner interface {
	Scan(...any) error
}

func scanAsset(row rowScanner) (Asset, error) {
	var asset Asset
	var kind, state string
	var sha256 pgtype.Text
	var uploadedAt pgtype.Timestamptz
	var visitID pgtype.UUID
	var source pgtype.Text
	err := row.Scan(
		&asset.ID, &asset.ProjectID, &visitID, &kind, &source, &asset.Filename, &asset.DisplayName, &asset.ContentType,
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
	if visitID.Valid {
		value := uuid.UUID(visitID.Bytes)
		asset.VisitID = &value
	}
	if source.Valid {
		asset.Source = Source(source.String)
	}
	if sha256.Valid {
		asset.SHA256 = sha256.String
	}
	if uploadedAt.Valid {
		value := uploadedAt.Time
		asset.UploadedAt = &value
	}
	return asset, nil
}
