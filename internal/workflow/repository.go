package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

func (r *PostgresRepository) CreateRun(ctx context.Context, input CreateRunInput) (Run, error) {
	if input.ProjectID == uuid.Nil || len(input.Nodes) == 0 {
		return Run{}, ErrInvalidRun
	}
	if input.Kind != RunKindBook && input.Kind != RunKindVisitAnalysis {
		return Run{}, ErrInvalidRun
	}
	if input.Kind == RunKindVisitAnalysis && input.VisitID == uuid.Nil {
		return Run{}, ErrInvalidRun
	}
	seen := make(map[NodeName]struct{}, len(input.Nodes))
	for _, name := range input.Nodes {
		if name == "" {
			return Run{}, ErrInvalidRun
		}
		if _, exists := seen[name]; exists {
			return Run{}, fmt.Errorf("%w: duplicate workflow node %s", ErrInvalidRun, name)
		}
		seen[name] = struct{}{}
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Run{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if input.Kind == RunKindBook {
		var audioCount, photoCount int
		err = tx.QueryRow(ctx, `
			SELECT count(*) FILTER (WHERE kind = 'audio'), count(*) FILTER (WHERE kind = 'photo')
			FROM assets WHERE project_id = $1 AND state = 'uploaded'`, input.ProjectID,
		).Scan(&audioCount, &photoCount)
		if err != nil {
			return Run{}, err
		}
		if audioCount == 0 || photoCount == 0 {
			return Run{}, ErrInsufficientAssets
		}
	} else {
		var exists bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS(SELECT 1 FROM visits WHERE id=$1 AND project_id=$2)`,
			input.VisitID, input.ProjectID,
		).Scan(&exists); err != nil {
			return Run{}, err
		}
		if !exists {
			return Run{}, ErrInvalidRun
		}
	}

	run, err := insertRun(ctx, tx, input)
	if err != nil {
		return Run{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Run{}, err
	}
	return run, nil
}

func (r *PostgresRepository) FinalizeBook(ctx context.Context, input FinalizeBookRequest) (Run, bool, error) {
	if input.ProjectID == uuid.Nil || input.StaffID == uuid.Nil || len(input.Nodes) == 0 {
		return Run{}, false, ErrInvalidRun
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Run{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var projectID uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT id FROM projects
		WHERE id=$1 AND (owner_staff_id=$2 OR ($3 AND owner_staff_id IS NULL))
		FOR UPDATE`, input.ProjectID, input.StaffID, input.IncludeUnowned).Scan(&projectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Run{}, false, ErrProjectNotFound
	}
	if err != nil {
		return Run{}, false, err
	}

	active, err := latestActiveBookRun(ctx, tx, input.ProjectID)
	if err == nil {
		if err := tx.Commit(ctx); err != nil {
			return Run{}, false, err
		}
		return active, false, nil
	}
	if !errors.Is(err, ErrRunNotFound) {
		return Run{}, false, err
	}

	var hasConsent, hasDraftVisit bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM consents WHERE project_id=$1)`, input.ProjectID).Scan(&hasConsent); err != nil {
		return Run{}, false, err
	}
	if !hasConsent {
		return Run{}, false, ErrConsentRequired
	}
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM visits WHERE project_id=$1 AND state='draft')`, input.ProjectID).Scan(&hasDraftVisit); err != nil {
		return Run{}, false, err
	}
	if hasDraftVisit {
		return Run{}, false, ErrDraftVisitExists
	}

	var audioCount, photoCount int
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE kind='audio'), count(*) FILTER (WHERE kind='photo')
		FROM assets WHERE project_id=$1 AND state='uploaded'`, input.ProjectID).Scan(&audioCount, &photoCount); err != nil {
		return Run{}, false, err
	}
	if audioCount == 0 || photoCount == 0 {
		return Run{}, false, ErrInsufficientAssets
	}

	run, err := insertRun(ctx, tx, CreateRunInput{
		ProjectID: input.ProjectID, Kind: RunKindBook, Nodes: input.Nodes,
	})
	if err != nil {
		return Run{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Run{}, false, err
	}
	return run, true, nil
}

func insertRun(ctx context.Context, tx pgx.Tx, input CreateRunInput) (Run, error) {
	now := time.Now().UTC()
	run := Run{
		ID: uuid.New(), ProjectID: input.ProjectID, VisitID: input.VisitID,
		Kind: input.Kind, State: NodeQueued, CreatedAt: now, UpdatedAt: now,
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO workflow_runs (id, project_id, kind, visit_id, state, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)`,
		run.ID, run.ProjectID, run.Kind, nullableUUID(run.VisitID), run.State, now)
	if err != nil {
		return Run{}, err
	}
	for position, nodeName := range input.Nodes {
		if _, err := tx.Exec(ctx, `
			INSERT INTO workflow_nodes (id, run_id, node_name, position, state, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'queued', $5, $5)`,
			uuid.New(), run.ID, nodeName, position, now); err != nil {
			return Run{}, err
		}
		run.Nodes = append(run.Nodes, Node{Name: nodeName, State: NodeQueued, Position: position})
	}
	if input.Kind == RunKindBook {
		if _, err := tx.Exec(ctx, "UPDATE projects SET state='processing', updated_at=$2 WHERE id=$1", input.ProjectID, now); err != nil {
			return Run{}, err
		}
	}
	return run, nil
}

func latestActiveBookRun(ctx context.Context, tx pgx.Tx, projectID uuid.UUID) (Run, error) {
	var run Run
	var kind, state string
	err := tx.QueryRow(ctx, `
		SELECT id, project_id, kind, state, COALESCE(error_code, ''), created_at, updated_at
		FROM workflow_runs
		WHERE project_id=$1 AND kind='book' AND state IN ('queued','running')
		ORDER BY created_at DESC LIMIT 1`, projectID).Scan(
		&run.ID, &run.ProjectID, &kind, &state, &run.ErrorCode, &run.CreatedAt, &run.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Run{}, ErrRunNotFound
	}
	if err != nil {
		return Run{}, err
	}
	run.Kind, run.State = RunKind(kind), NodeState(state)
	rows, err := tx.Query(ctx, `
		SELECT node_name, state, COALESCE(error_code, ''), attempts, position
		FROM workflow_nodes WHERE run_id=$1 ORDER BY position`, run.ID)
	if err != nil {
		return Run{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var node Node
		var name, nodeState string
		if err := rows.Scan(&name, &nodeState, &node.ErrorCode, &node.Attempts, &node.Position); err != nil {
			return Run{}, err
		}
		node.Name, node.State = NodeName(name), NodeState(nodeState)
		run.Nodes = append(run.Nodes, node)
	}
	return run, rows.Err()
}

func (r *PostgresRepository) ClaimNode(ctx context.Context, payload NodePayload) (bool, error) {
	var state string
	err := r.pool.QueryRow(ctx, `
		UPDATE workflow_nodes
		SET state = 'running', attempts = attempts + 1, updated_at = now()
		WHERE run_id = $1 AND node_name = $2 AND state IN ('queued', 'failed')
		RETURNING state`, payload.RunID, payload.Node).Scan(&state)
	if err == nil {
		_, _ = r.pool.Exec(ctx, "UPDATE workflow_runs SET state = 'running', error_code = NULL, updated_at = now() WHERE id = $1", payload.RunID)
		return true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	err = r.pool.QueryRow(ctx, "SELECT state FROM workflow_nodes WHERE run_id = $1 AND node_name = $2", payload.RunID, payload.Node).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrNodeNotFound
	}
	return false, err
}

func (r *PostgresRepository) SucceedNode(ctx context.Context, payload NodePayload, output json.RawMessage) (*NodePayload, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var position int
	err = tx.QueryRow(ctx, `
		UPDATE workflow_nodes
		SET state='succeeded', output=$3, error_code=NULL, updated_at=now()
		WHERE run_id=$1 AND node_name=$2 AND state='running'
		RETURNING position`, payload.RunID, payload.Node, output).Scan(&position)
	if errors.Is(err, pgx.ErrNoRows) {
		var state string
		err = tx.QueryRow(ctx, `
			SELECT state, position FROM workflow_nodes
			WHERE run_id=$1 AND node_name=$2 FOR UPDATE`,
			payload.RunID, payload.Node).Scan(&state, &position)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && state != string(NodeSucceeded)) {
			return nil, ErrNodeNotFound
		}
	}
	if err != nil {
		return nil, err
	}

	var kind string
	var visitID uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT kind, COALESCE(visit_id, '00000000-0000-0000-0000-000000000000'::uuid)
		FROM workflow_runs WHERE id=$1`, payload.RunID).Scan(&kind, &visitID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRunNotFound
	}
	if err != nil {
		return nil, err
	}

	var nextName string
	err = tx.QueryRow(ctx, `
		SELECT node_name FROM workflow_nodes
		WHERE run_id=$1 AND position>$2
		ORDER BY position LIMIT 1`, payload.RunID, position).Scan(&nextName)
	if err == nil {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return &NodePayload{
			RunID: payload.RunID, ProjectID: payload.ProjectID, VisitID: visitID,
			Kind: RunKind(kind), Node: NodeName(nextName),
		}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if _, err := tx.Exec(ctx, "UPDATE workflow_runs SET state='succeeded', updated_at=now() WHERE id=$1", payload.RunID); err != nil {
		return nil, err
	}
	if RunKind(kind) == RunKindBook {
		if _, err := tx.Exec(ctx, "UPDATE projects SET state='completed', updated_at=now() WHERE id=$1", payload.ProjectID); err != nil {
			return nil, err
		}
	}
	return nil, tx.Commit(ctx)
}

func (r *PostgresRepository) FailNode(ctx context.Context, payload NodePayload, code string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := tx.Exec(ctx, `
		UPDATE workflow_nodes SET state='failed', error_code=$3, updated_at=now()
		WHERE run_id=$1 AND node_name=$2 AND state IN ('running','failed')`,
		payload.RunID, payload.Node, code)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNodeNotFound
	}
	if _, err := tx.Exec(ctx, `
		UPDATE workflow_runs SET state='failed', error_code=$2, updated_at=now() WHERE id=$1`,
		payload.RunID, code); err != nil {
		return err
	}
	var kind string
	var visitID uuid.UUID
	if err := tx.QueryRow(ctx, `
		SELECT kind, COALESCE(visit_id, '00000000-0000-0000-0000-000000000000'::uuid)
		FROM workflow_runs WHERE id=$1`, payload.RunID).Scan(&kind, &visitID); err != nil {
		return err
	}
	if RunKind(kind) == RunKindBook {
		if _, err := tx.Exec(ctx, "UPDATE projects SET state='exception', updated_at=now() WHERE id=$1", payload.ProjectID); err != nil {
			return err
		}
	} else if visitID != uuid.Nil {
		if _, err := tx.Exec(ctx, "UPDATE visits SET state='failed', error_code=$2, updated_at=now() WHERE id=$1", visitID, code); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) ProjectOwned(ctx context.Context, projectID, staffID uuid.UUID, includeUnowned bool) error {
	var exists bool
	if err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM projects
			WHERE id=$1 AND (owner_staff_id=$2 OR ($3 AND owner_staff_id IS NULL))
		)`, projectID, staffID, includeUnowned).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrProjectNotFound
	}
	return nil
}

func (r *PostgresRepository) LatestRun(ctx context.Context, projectID, staffID uuid.UUID, includeUnowned bool) (Run, error) {
	var run Run
	var state, kind string
	err := r.pool.QueryRow(ctx, `
		SELECT workflow_runs.id, workflow_runs.project_id,
		       COALESCE(workflow_runs.visit_id, '00000000-0000-0000-0000-000000000000'::uuid),
		       workflow_runs.kind, workflow_runs.state, COALESCE(workflow_runs.error_code, ''),
		       workflow_runs.created_at, workflow_runs.updated_at
		FROM workflow_runs
		JOIN projects ON projects.id=workflow_runs.project_id
		WHERE workflow_runs.project_id=$1 AND workflow_runs.kind='book'
		  AND (projects.owner_staff_id=$2 OR ($3 AND projects.owner_staff_id IS NULL))
		ORDER BY workflow_runs.created_at DESC LIMIT 1`, projectID, staffID, includeUnowned).Scan(
		&run.ID, &run.ProjectID, &run.VisitID, &kind, &state,
		&run.ErrorCode, &run.CreatedAt, &run.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Run{}, ErrRunNotFound
	}
	if err != nil {
		return Run{}, err
	}
	run.Kind, run.State = RunKind(kind), NodeState(state)
	rows, err := r.pool.Query(ctx, `
		SELECT node_name, state, COALESCE(error_code, ''), attempts, position
		FROM workflow_nodes WHERE run_id=$1 ORDER BY position`, run.ID)
	if err != nil {
		return Run{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var node Node
		var name, nodeState string
		if err := rows.Scan(&name, &nodeState, &node.ErrorCode, &node.Attempts, &node.Position); err != nil {
			return Run{}, err
		}
		node.Name, node.State = NodeName(name), NodeState(nodeState)
		run.Nodes = append(run.Nodes, node)
	}
	return run, rows.Err()
}

func nullableUUID(value uuid.UUID) any {
	if value == uuid.Nil {
		return nil
	}
	return value
}
