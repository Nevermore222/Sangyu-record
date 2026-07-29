package projects

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

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
			id, owner_staff_id, display_name, birth_year, birth_place, long_term_residence,
			primary_occupation, target_edition, state, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		detail.ID, nullableUUID(detail.OwnerStaffID), detail.DisplayName, detail.BirthYear, detail.BirthPlace,
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
	return r.get(ctx, id, uuid.Nil, false, false)
}

func (r *PostgresRepository) GetOwned(ctx context.Context, id, ownerStaffID uuid.UUID, includeUnowned bool) (ProjectDetail, error) {
	return r.get(ctx, id, ownerStaffID, includeUnowned, true)
}

func (r *PostgresRepository) get(ctx context.Context, id, ownerStaffID uuid.UUID, includeUnowned, checkOwner bool) (ProjectDetail, error) {
	var detail ProjectDetail
	var state string
	query := `
		SELECT id, COALESCE(owner_staff_id, '00000000-0000-0000-0000-000000000000'::uuid),
		       display_name, birth_year, birth_place, long_term_residence,
		       primary_occupation, target_edition, state, created_at, updated_at
		FROM projects
		WHERE id = $1`
	args := []any{id}
	if checkOwner {
		query += ` AND (owner_staff_id = $2 OR ($3 AND owner_staff_id IS NULL))`
		args = append(args, ownerStaffID, includeUnowned)
	}
	err := r.pool.QueryRow(ctx, `
		`+query, args...,
	).Scan(
		&detail.ID, &detail.OwnerStaffID, &detail.DisplayName, &detail.BirthYear, &detail.BirthPlace,
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
	var consent Consent
	err = r.pool.QueryRow(ctx, `
		SELECT id, project_id, confirmed_by, confirmation_method, staff_id, confirmed_at
		FROM consents WHERE project_id = $1
		ORDER BY confirmed_at DESC LIMIT 1`, id,
	).Scan(
		&consent.ID, &consent.ProjectID, &consent.ConfirmedBy,
		&consent.ConfirmationMethod, &consent.StaffID, &consent.ConfirmedAt,
	)
	if err == nil {
		detail.Consent = &consent
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return ProjectDetail{}, err
	}
	return detail, nil
}

type cursorPayload struct {
	UpdatedAt time.Time `json:"updated_at"`
	ID        uuid.UUID `json:"id"`
}

func (r *PostgresRepository) List(ctx context.Context, input ListInput) (Page, error) {
	limit := input.Limit
	if limit < 1 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	where := []string{"(owner_staff_id = $1 OR ($2 AND owner_staff_id IS NULL))"}
	args := []any{input.OwnerStaffID, input.IncludeUnowned}
	if query := strings.TrimSpace(input.Query); query != "" {
		args = append(args, "%"+query+"%")
		position := len(args)
		where = append(where, fmt.Sprintf(`(
			display_name ILIKE $%d OR birth_place ILIKE $%d OR
			long_term_residence ILIKE $%d OR primary_occupation ILIKE $%d
		)`, position, position, position, position))
	}
	if input.State != "" {
		args = append(args, input.State)
		where = append(where, fmt.Sprintf("state = $%d", len(args)))
	}
	if input.Cursor != "" {
		cursor, err := decodeCursor(input.Cursor)
		if err != nil {
			return Page{}, err
		}
		args = append(args, cursor.UpdatedAt, cursor.ID)
		where = append(where, fmt.Sprintf("(updated_at, id) < ($%d, $%d)", len(args)-1, len(args)))
	}
	args = append(args, limit+1)
	query := `
		SELECT id, COALESCE(owner_staff_id, '00000000-0000-0000-0000-000000000000'::uuid),
		       display_name, birth_year, birth_place, long_term_residence,
		       primary_occupation, target_edition, state, updated_at
		FROM projects
		WHERE ` + strings.Join(where, " AND ") + fmt.Sprintf(`
		ORDER BY updated_at DESC, id DESC
		LIMIT $%d`, len(args))
	items, err := r.querySummaries(ctx, query, args...)
	if err != nil {
		return Page{}, err
	}
	page := Page{Items: items}
	if len(items) > limit {
		page.Items = items[:limit]
		last := page.Items[len(page.Items)-1]
		page.NextCursor = encodeCursor(cursorPayload{UpdatedAt: last.UpdatedAt, ID: last.ID})
	}
	return page, nil
}

func (r *PostgresRepository) Dashboard(ctx context.Context, ownerStaffID uuid.UUID, includeUnowned bool) (Dashboard, error) {
	var result Dashboard
	err := r.pool.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE state = 'collecting'),
			count(*) FILTER (WHERE state = 'needs_material'),
			count(*) FILTER (WHERE state IN ('processing', 'generating', 'quality_check', 'pdf_rendering')),
			count(*) FILTER (WHERE state = 'completed')
		FROM projects
		WHERE owner_staff_id = $1 OR ($2 AND owner_staff_id IS NULL)`,
		ownerStaffID, includeUnowned,
	).Scan(
		&result.Counts.Collecting, &result.Counts.NeedsMaterial,
		&result.Counts.Processing, &result.Counts.Completed,
	)
	if err != nil {
		return Dashboard{}, err
	}
	baseSelect := `
		SELECT id, COALESCE(owner_staff_id, '00000000-0000-0000-0000-000000000000'::uuid),
		       display_name, birth_year, birth_place, long_term_residence,
		       primary_occupation, target_edition, state, updated_at
		FROM projects
		WHERE (owner_staff_id = $1 OR ($2 AND owner_staff_id IS NULL))`
	result.Actionable, err = r.querySummaries(ctx, baseSelect+`
		  AND state <> 'completed'
		ORDER BY CASE state
			WHEN 'needs_material' THEN 0 WHEN 'exception' THEN 1
			WHEN 'collecting' THEN 2 ELSE 3 END,
			updated_at DESC, id DESC LIMIT 5`, ownerStaffID, includeUnowned)
	if err != nil {
		return Dashboard{}, err
	}
	result.Recent, err = r.querySummaries(ctx, baseSelect+`
		ORDER BY updated_at DESC, id DESC LIMIT 5`, ownerStaffID, includeUnowned)
	if err != nil {
		return Dashboard{}, err
	}
	return result, nil
}

func (r *PostgresRepository) UpsertConsent(ctx context.Context, value Consent) (Consent, error) {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO consents (
			id, project_id, confirmed_by, confirmation_method, staff_id, confirmed_at
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (project_id, confirmed_by, confirmation_method) DO UPDATE SET
			staff_id = EXCLUDED.staff_id,
			confirmed_at = EXCLUDED.confirmed_at
		RETURNING id, project_id, confirmed_by, confirmation_method, staff_id, confirmed_at`,
		value.ID, value.ProjectID, value.ConfirmedBy, value.ConfirmationMethod,
		value.StaffID, value.ConfirmedAt,
	).Scan(
		&value.ID, &value.ProjectID, &value.ConfirmedBy, &value.ConfirmationMethod,
		&value.StaffID, &value.ConfirmedAt,
	)
	return value, err
}

func (r *PostgresRepository) HasConsent(ctx context.Context, projectID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM consents WHERE project_id = $1)`, projectID).Scan(&exists)
	return exists, err
}

func (r *PostgresRepository) querySummaries(ctx context.Context, query string, args ...any) ([]ProjectSummary, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ProjectSummary, 0)
	for rows.Next() {
		var item ProjectSummary
		var state string
		if err := rows.Scan(
			&item.ID, &item.OwnerStaffID, &item.DisplayName, &item.BirthYear,
			&item.BirthPlace, &item.LongTermResidence, &item.PrimaryOccupation,
			&item.TargetEdition, &state, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		item.State = State(state)
		items = append(items, item)
	}
	return items, rows.Err()
}

func encodeCursor(cursor cursorPayload) string {
	encoded, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(encoded)
}

func decodeCursor(value string) (cursorPayload, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return cursorPayload{}, fmt.Errorf("%w: invalid cursor", ErrValidation)
	}
	var cursor cursorPayload
	if err := json.Unmarshal(decoded, &cursor); err != nil || cursor.ID == uuid.Nil || cursor.UpdatedAt.IsZero() {
		return cursorPayload{}, fmt.Errorf("%w: invalid cursor", ErrValidation)
	}
	return cursor, nil
}

func nullableUUID(value uuid.UUID) any {
	if value == uuid.Nil {
		return nil
	}
	return value
}
