package visitanalysis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nevermore222/sangyu-record/internal/providers"
	"github.com/nevermore222/sangyu-record/internal/workflow"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) PrepareSubmit(ctx context.Context, visitID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var state string
	err = tx.QueryRow(ctx, `SELECT state FROM visits WHERE id=$1 FOR UPDATE`, visitID).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if state != "draft" {
		return ErrInvalidState
	}
	var count int
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FROM assets
		WHERE visit_id=$1 AND state='uploaded' AND kind IN ('audio','photo')`, visitID).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		return ErrInsufficientAssets
	}
	if _, err := tx.Exec(ctx, `
		UPDATE visits SET state='submitted', error_code=NULL, updated_at=now() WHERE id=$1`, visitID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) AttachRun(ctx context.Context, visitID, runID uuid.UUID) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE visits SET state='analyzing', error_code=NULL, updated_at=now()
		WHERE id=$1 AND EXISTS(
			SELECT 1 FROM workflow_runs WHERE id=$2 AND visit_id=$1 AND kind='visit_analysis'
		)`, visitID, runID)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) RevertSubmit(ctx context.Context, visitID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE visits SET state='draft', updated_at=now() WHERE id=$1 AND state='submitted'`, visitID)
	return err
}

func (r *PostgresRepository) RetryPayload(ctx context.Context, visitID uuid.UUID) (workflow.NodePayload, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return workflow.NodePayload{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var payload workflow.NodePayload
	var kind, node string
	err = tx.QueryRow(ctx, `
		SELECT runs.id, runs.project_id, runs.visit_id, runs.kind, nodes.node_name
		FROM workflow_runs AS runs
		JOIN workflow_nodes AS nodes ON nodes.run_id=runs.id
		WHERE runs.visit_id=$1 AND runs.kind='visit_analysis'
		  AND nodes.state IN ('failed','queued')
		ORDER BY runs.created_at DESC, nodes.position
		LIMIT 1`, visitID).Scan(
		&payload.RunID, &payload.ProjectID, &payload.VisitID, &kind, &node,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return workflow.NodePayload{}, ErrInvalidState
	}
	if err != nil {
		return workflow.NodePayload{}, err
	}
	payload.Kind, payload.Node = workflow.RunKind(kind), workflow.NodeName(node)
	if _, err := tx.Exec(ctx, `
		UPDATE workflow_runs SET state='queued', error_code=NULL, updated_at=now() WHERE id=$1`, payload.RunID); err != nil {
		return workflow.NodePayload{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE visits SET state='analyzing', error_code=NULL, updated_at=now() WHERE id=$1`, visitID); err != nil {
		return workflow.NodePayload{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return workflow.NodePayload{}, err
	}
	return payload, nil
}

func (r *PostgresRepository) GetByVisit(ctx context.Context, visitID uuid.UUID) (Analysis, error) {
	return scanAnalysis(r.pool.QueryRow(ctx, `
		SELECT id, visit_id, workflow_run_id, summary, covered_items, gaps,
		       followup_questions, created_at, updated_at
		FROM visit_analyses WHERE visit_id=$1`, visitID))
}

func (r *PostgresRepository) BuildProviderInput(ctx context.Context, payload workflow.NodePayload) (json.RawMessage, error) {
	var project struct {
		DisplayName       string `json:"display_name"`
		BirthYear         int    `json:"birth_year"`
		BirthPlace        string `json:"birth_place"`
		LongTermResidence string `json:"long_term_residence"`
		PrimaryOccupation string `json:"primary_occupation"`
	}
	var visit struct {
		ID        uuid.UUID `json:"id"`
		Sequence  int       `json:"sequence"`
		VisitedAt time.Time `json:"visited_at"`
		Location  string    `json:"location"`
		Notes     string    `json:"notes"`
	}
	err := r.pool.QueryRow(ctx, `
		SELECT projects.display_name, projects.birth_year, projects.birth_place,
		       projects.long_term_residence, projects.primary_occupation,
		       visits.id, visits.sequence, visits.visited_at, visits.location, visits.notes
		FROM visits JOIN projects ON projects.id=visits.project_id
		WHERE visits.id=$1 AND projects.id=$2`, payload.VisitID, payload.ProjectID).Scan(
		&project.DisplayName, &project.BirthYear, &project.BirthPlace,
		&project.LongTermResidence, &project.PrimaryOccupation,
		&visit.ID, &visit.Sequence, &visit.VisitedAt, &visit.Location, &visit.Notes,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	type planItem struct {
		ID       uuid.UUID `json:"id"`
		Category string    `json:"category"`
		Prompt   string    `json:"prompt"`
	}
	selected := make([]planItem, 0)
	rows, err := r.pool.Query(ctx, `
		SELECT items.id, items.category, items.prompt
		FROM visit_plan_items AS selected
		JOIN collection_plan_items AS items ON items.id=selected.plan_item_id
		WHERE selected.visit_id=$1 ORDER BY items.position`, payload.VisitID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var item planItem
		if err := rows.Scan(&item.ID, &item.Category, &item.Prompt); err != nil {
			rows.Close()
			return nil, err
		}
		selected = append(selected, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	outputs := make(map[string]json.RawMessage)
	rows, err = r.pool.Query(ctx, `
		SELECT node_name, output FROM workflow_nodes
		WHERE run_id=$1 AND output IS NOT NULL ORDER BY position`, payload.RunID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var name string
		var output json.RawMessage
		if err := rows.Scan(&name, &output); err != nil {
			rows.Close()
			return nil, err
		}
		outputs[name] = output
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	return json.Marshal(map[string]any{
		"project_id":          payload.ProjectID,
		"visit_id":            payload.VisitID,
		"workflow_run_id":     payload.RunID,
		"project":             project,
		"visit":               visit,
		"selected_plan_items": selected,
		"upstream_outputs":    outputs,
	})
}

func (r *PostgresRepository) Persist(ctx context.Context, payload workflow.NodePayload) (Analysis, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Analysis{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	existing, err := scanAnalysis(tx.QueryRow(ctx, `
		SELECT id, visit_id, workflow_run_id, summary, covered_items, gaps,
		       followup_questions, created_at, updated_at
		FROM visit_analyses WHERE visit_id=$1`, payload.VisitID))
	if err == nil {
		return existing, tx.Commit(ctx)
	}
	if !errors.Is(err, ErrNotFound) {
		return Analysis{}, err
	}

	var assessmentRaw, followupRaw json.RawMessage
	if err := tx.QueryRow(ctx, `
		SELECT
			(SELECT output FROM workflow_nodes WHERE run_id=$1 AND node_name='visit_assess_material'),
			(SELECT output FROM workflow_nodes WHERE run_id=$1 AND node_name='visit_plan_followup')`,
		payload.RunID).Scan(&assessmentRaw, &followupRaw); err != nil {
		return Analysis{}, err
	}
	var assessment providers.MaterialAssessment
	var followup providers.FollowupPlan
	if err := json.Unmarshal(assessmentRaw, &assessment); err != nil {
		return Analysis{}, fmt.Errorf("%w: decode material assessment: %v", ErrValidation, err)
	}
	if err := json.Unmarshal(followupRaw, &followup); err != nil {
		return Analysis{}, fmt.Errorf("%w: decode follow-up plan: %v", ErrValidation, err)
	}
	var projectID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT project_id FROM visits WHERE id=$1 FOR UPDATE`, payload.VisitID).Scan(&projectID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Analysis{}, ErrNotFound
		}
		return Analysis{}, err
	}
	for _, item := range assessment.CoveredItems {
		id, err := uuid.Parse(item.PlanItemID)
		if err != nil {
			return Analysis{}, fmt.Errorf("%w: covered plan item ID", ErrValidation)
		}
		result, err := tx.Exec(ctx, `
			UPDATE collection_plan_items SET status='collected', gap_reason=''
			WHERE id=$1 AND project_id=$2`, id, projectID)
		if err != nil || result.RowsAffected() != 1 {
			return Analysis{}, fmt.Errorf("%w: covered plan item does not belong to project", ErrValidation)
		}
	}
	for _, gap := range assessment.Gaps {
		id, err := uuid.Parse(gap.PlanItemID)
		if err != nil {
			return Analysis{}, fmt.Errorf("%w: gap plan item ID", ErrValidation)
		}
		result, err := tx.Exec(ctx, `
			UPDATE collection_plan_items SET status='insufficient', gap_reason=$3
			WHERE id=$1 AND project_id=$2`, id, projectID, gap.Reason)
		if err != nil || result.RowsAffected() != 1 {
			return Analysis{}, fmt.Errorf("%w: gap plan item does not belong to project", ErrValidation)
		}
	}
	coveredRaw, _ := json.Marshal(assessment.CoveredItems)
	gapsRaw, _ := json.Marshal(assessment.Gaps)
	questionsRaw, _ := json.Marshal(followup.Questions)
	now := time.Now().UTC()
	analysis := Analysis{
		ID: uuid.New(), VisitID: payload.VisitID, WorkflowRunID: payload.RunID,
		Summary: followup.Summary, CoveredItems: assessment.CoveredItems,
		Gaps: assessment.Gaps, FollowupQuestions: followup.Questions,
		CreatedAt: now, UpdatedAt: now,
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO visit_analyses (
			id, visit_id, workflow_run_id, summary, covered_items, gaps,
			followup_questions, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$8)`,
		analysis.ID, analysis.VisitID, analysis.WorkflowRunID, analysis.Summary,
		coveredRaw, gapsRaw, questionsRaw, now)
	if err != nil {
		return Analysis{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE visits SET state='completed', error_code=NULL, updated_at=$2 WHERE id=$1`, payload.VisitID, now); err != nil {
		return Analysis{}, err
	}
	projectState := "collecting"
	if len(assessment.Gaps) > 0 {
		projectState = "needs_material"
	}
	if _, err := tx.Exec(ctx, `UPDATE projects SET state=$2, updated_at=$3 WHERE id=$1`, projectID, projectState, now); err != nil {
		return Analysis{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Analysis{}, err
	}
	return analysis, nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanAnalysis(row rowScanner) (Analysis, error) {
	var value Analysis
	var coveredRaw, gapsRaw, questionsRaw json.RawMessage
	err := row.Scan(
		&value.ID, &value.VisitID, &value.WorkflowRunID, &value.Summary,
		&coveredRaw, &gapsRaw, &questionsRaw, &value.CreatedAt, &value.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Analysis{}, ErrNotFound
	}
	if err != nil {
		return Analysis{}, err
	}
	if err := json.Unmarshal(coveredRaw, &value.CoveredItems); err != nil {
		return Analysis{}, err
	}
	if err := json.Unmarshal(gapsRaw, &value.Gaps); err != nil {
		return Analysis{}, err
	}
	if err := json.Unmarshal(questionsRaw, &value.FollowupQuestions); err != nil {
		return Analysis{}, err
	}
	return value, nil
}
