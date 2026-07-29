package visits

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Create(ctx context.Context, value Visit, includeUnowned bool) (Visit, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Visit{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var projectID uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT id FROM projects
		WHERE id = $1 AND (owner_staff_id = $2 OR ($3 AND owner_staff_id IS NULL))
		FOR UPDATE`, value.ProjectID, value.StaffID, includeUnowned).Scan(&projectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Visit{}, ErrNotFound
	}
	if err != nil {
		return Visit{}, err
	}
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(sequence), 0) + 1 FROM visits WHERE project_id = $1`,
		value.ProjectID,
	).Scan(&value.Sequence); err != nil {
		return Visit{}, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO visits (
			id, project_id, sequence, staff_id, visited_at, location, notes,
			state, error_code, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULLIF($9, ''), $10, $11)`,
		value.ID, value.ProjectID, value.Sequence, value.StaffID, value.VisitedAt,
		value.Location, value.Notes, value.State, value.ErrorCode, value.CreatedAt, value.UpdatedAt,
	)
	if err != nil {
		return Visit{}, err
	}
	if err := replacePlanItems(ctx, tx, value.ID, value.ProjectID, value.PlanItemIDs); err != nil {
		return Visit{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Visit{}, err
	}
	return value, nil
}

func (r *PostgresRepository) Get(ctx context.Context, id, staffID uuid.UUID, includeUnowned bool) (Visit, error) {
	value, err := scanVisit(r.pool.QueryRow(ctx, `
		SELECT visits.id, visits.project_id, visits.sequence, visits.staff_id,
		       visits.visited_at, visits.location, visits.notes, visits.state,
		       COALESCE(visits.error_code, ''), visits.created_at, visits.updated_at
		FROM visits
		JOIN projects ON projects.id = visits.project_id
		WHERE visits.id = $1
		  AND (projects.owner_staff_id = $2 OR ($3 AND projects.owner_staff_id IS NULL))`,
		id, staffID, includeUnowned,
	))
	if err != nil {
		return Visit{}, err
	}
	value.PlanItemIDs, err = loadPlanItems(ctx, r.pool, value.ID)
	if err != nil {
		return Visit{}, err
	}
	return value, nil
}

func (r *PostgresRepository) List(ctx context.Context, projectID, staffID uuid.UUID, includeUnowned bool) ([]Visit, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT visits.id, visits.project_id, visits.sequence, visits.staff_id,
		       visits.visited_at, visits.location, visits.notes, visits.state,
		       COALESCE(visits.error_code, ''), visits.created_at, visits.updated_at
		FROM visits
		JOIN projects ON projects.id = visits.project_id
		WHERE visits.project_id = $1
		  AND (projects.owner_staff_id = $2 OR ($3 AND projects.owner_staff_id IS NULL))
		ORDER BY visits.sequence DESC`, projectID, staffID, includeUnowned)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Visit, 0)
	for rows.Next() {
		value, err := scanVisit(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for index := range items {
		items[index].PlanItemIDs, err = loadPlanItems(ctx, r.pool, items[index].ID)
		if err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (r *PostgresRepository) Update(ctx context.Context, value Visit, includeUnowned bool) (Visit, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Visit{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var state string
	err = tx.QueryRow(ctx, `
		SELECT visits.state
		FROM visits
		JOIN projects ON projects.id = visits.project_id
		WHERE visits.id = $1
		  AND (projects.owner_staff_id = $2 OR ($3 AND projects.owner_staff_id IS NULL))
		FOR UPDATE OF visits`, value.ID, value.StaffID, includeUnowned).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) {
		return Visit{}, ErrNotFound
	}
	if err != nil {
		return Visit{}, err
	}
	if State(state) != StateDraft {
		return Visit{}, ErrInvalidState
	}
	_, err = tx.Exec(ctx, `
		UPDATE visits SET visited_at=$2, location=$3, notes=$4, updated_at=$5
		WHERE id=$1`, value.ID, value.VisitedAt, value.Location, value.Notes, value.UpdatedAt)
	if err != nil {
		return Visit{}, err
	}
	if err := replacePlanItems(ctx, tx, value.ID, value.ProjectID, value.PlanItemIDs); err != nil {
		return Visit{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Visit{}, err
	}
	return value, nil
}

type rowScanner interface {
	Scan(...any) error
}

type queryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func scanVisit(row rowScanner) (Visit, error) {
	var value Visit
	var state string
	err := row.Scan(
		&value.ID, &value.ProjectID, &value.Sequence, &value.StaffID,
		&value.VisitedAt, &value.Location, &value.Notes, &state,
		&value.ErrorCode, &value.CreatedAt, &value.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Visit{}, ErrNotFound
	}
	if err != nil {
		return Visit{}, err
	}
	value.State = State(state)
	return value, nil
}

func loadPlanItems(ctx context.Context, queryer queryer, visitID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := queryer.Query(ctx, `
		SELECT plan_item_id FROM visit_plan_items
		WHERE visit_id=$1 ORDER BY plan_item_id`, visitID)
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

func replacePlanItems(ctx context.Context, tx pgx.Tx, visitID, projectID uuid.UUID, planItemIDs []uuid.UUID) error {
	if _, err := tx.Exec(ctx, `DELETE FROM visit_plan_items WHERE visit_id=$1`, visitID); err != nil {
		return err
	}
	for _, planItemID := range planItemIDs {
		result, err := tx.Exec(ctx, `
			INSERT INTO visit_plan_items (visit_id, plan_item_id)
			SELECT $1, id FROM collection_plan_items
			WHERE id=$2 AND project_id=$3`, visitID, planItemID, projectID)
		if err != nil {
			return err
		}
		if result.RowsAffected() != 1 {
			return fmt.Errorf("%w: plan item does not belong to project", ErrValidation)
		}
	}
	return nil
}
