package projects

import (
	"context"
	"errors"

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

func (r *PostgresRepository) Create(ctx context.Context, detail ProjectDetail) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `
		INSERT INTO projects (
			id, display_name, birth_year, birth_place, long_term_residence,
			primary_occupation, target_edition, state, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		detail.ID, detail.DisplayName, detail.BirthYear, detail.BirthPlace,
		detail.LongTermResidence, detail.PrimaryOccupation, detail.TargetEdition,
		detail.State, detail.CreatedAt, detail.UpdatedAt,
	)
	if err != nil {
		return err
	}

	for _, item := range detail.CollectionPlan {
		_, err = tx.Exec(ctx, `
			INSERT INTO collection_plan_items (
				id, project_id, category, prompt, required, status, position, created_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			item.ID, item.ProjectID, item.Category, item.Prompt, item.Required,
			item.Status, item.Position, item.CreatedAt,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *PostgresRepository) Get(ctx context.Context, id uuid.UUID) (ProjectDetail, error) {
	var detail ProjectDetail
	var state string
	err := r.pool.QueryRow(ctx, `
		SELECT id, display_name, birth_year, birth_place, long_term_residence,
		       primary_occupation, target_edition, state, created_at, updated_at
		FROM projects
		WHERE id = $1`, id,
	).Scan(
		&detail.ID, &detail.DisplayName, &detail.BirthYear, &detail.BirthPlace,
		&detail.LongTermResidence, &detail.PrimaryOccupation, &detail.TargetEdition,
		&state, &detail.CreatedAt, &detail.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ProjectDetail{}, ErrNotFound
	}
	if err != nil {
		return ProjectDetail{}, err
	}
	detail.State = State(state)

	rows, err := r.pool.Query(ctx, `
		SELECT id, project_id, category, prompt, required, status, position, created_at
		FROM collection_plan_items
		WHERE project_id = $1
		ORDER BY position`, id)
	if err != nil {
		return ProjectDetail{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var item PlanItem
		var status string
		if err := rows.Scan(
			&item.ID, &item.ProjectID, &item.Category, &item.Prompt,
			&item.Required, &status, &item.Position, &item.CreatedAt,
		); err != nil {
			return ProjectDetail{}, err
		}
		item.Status = PlanItemStatus(status)
		detail.CollectionPlan = append(detail.CollectionPlan, item)
	}
	if err := rows.Err(); err != nil {
		return ProjectDetail{}, err
	}
	return detail, nil
}
