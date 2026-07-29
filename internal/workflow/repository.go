package workflow

import (
	"context"
	"encoding/json"
	"errors"
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

func (r *PostgresRepository) CreateRun(ctx context.Context, projectID uuid.UUID) (Run, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Run{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var audioCount, photoCount int
	err = tx.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE kind = 'audio'), count(*) FILTER (WHERE kind = 'photo')
		FROM assets WHERE project_id = $1 AND state = 'uploaded'`, projectID,
	).Scan(&audioCount, &photoCount)
	if err != nil {
		return Run{}, err
	}
	if audioCount == 0 || photoCount == 0 {
		return Run{}, ErrInsufficientAssets
	}

	now := time.Now().UTC()
	run := Run{ID: uuid.New(), ProjectID: projectID, State: NodeQueued, CreatedAt: now, UpdatedAt: now}
	_, err = tx.Exec(ctx, `
		INSERT INTO workflow_runs (id, project_id, state, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $4)`, run.ID, run.ProjectID, run.State, now)
	if err != nil {
		return Run{}, err
	}
	for _, nodeName := range NodeSequence {
		_, err = tx.Exec(ctx, `
			INSERT INTO workflow_nodes (id, run_id, node_name, state, created_at, updated_at)
			VALUES ($1, $2, $3, 'queued', $4, $4)`, uuid.New(), run.ID, nodeName, now)
		if err != nil {
			return Run{}, err
		}
		run.Nodes = append(run.Nodes, Node{Name: nodeName, State: NodeQueued})
	}
	if _, err := tx.Exec(ctx, "UPDATE projects SET state = 'processing', updated_at = $2 WHERE id = $1", projectID, now); err != nil {
		return Run{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Run{}, err
	}
	return run, nil
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
	result, err := tx.Exec(ctx, `
		UPDATE workflow_nodes SET state = 'succeeded', output = $3, error_code = NULL, updated_at = now()
		WHERE run_id = $1 AND node_name = $2 AND state = 'running'`, payload.RunID, payload.Node, output)
	if err != nil {
		return nil, err
	}
	if result.RowsAffected() == 0 {
		return nil, ErrNodeNotFound
	}

	nextName, hasNext := nextNode(payload.Node)
	if !hasNext {
		if _, err := tx.Exec(ctx, "UPDATE workflow_runs SET state = 'succeeded', updated_at = now() WHERE id = $1", payload.RunID); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, "UPDATE projects SET state = 'completed', updated_at = now() WHERE id = $1", payload.ProjectID); err != nil {
			return nil, err
		}
		return nil, tx.Commit(ctx)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &NodePayload{RunID: payload.RunID, ProjectID: payload.ProjectID, Node: nextName}, nil
}

func (r *PostgresRepository) FailNode(ctx context.Context, payload NodePayload, code string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
		UPDATE workflow_nodes SET state = 'failed', error_code = $3, updated_at = now()
		WHERE run_id = $1 AND node_name = $2`, payload.RunID, payload.Node, code); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "UPDATE workflow_runs SET state = 'failed', error_code = $2, updated_at = now() WHERE id = $1", payload.RunID, code); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "UPDATE projects SET state = 'exception', updated_at = now() WHERE id = $1", payload.ProjectID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) LatestRun(ctx context.Context, projectID uuid.UUID) (Run, error) {
	var run Run
	var state string
	err := r.pool.QueryRow(ctx, `
		SELECT id, project_id, state, coalesce(error_code, ''), created_at, updated_at
		FROM workflow_runs WHERE project_id = $1 ORDER BY created_at DESC LIMIT 1`, projectID,
	).Scan(&run.ID, &run.ProjectID, &state, &run.ErrorCode, &run.CreatedAt, &run.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Run{}, ErrRunNotFound
	}
	if err != nil {
		return Run{}, err
	}
	run.State = NodeState(state)
	rows, err := r.pool.Query(ctx, `
		SELECT node_name, state, coalesce(error_code, ''), attempts
		FROM workflow_nodes WHERE run_id = $1
		ORDER BY CASE node_name
			WHEN 'transcribe' THEN 1 WHEN 'understand_photo' THEN 2 WHEN 'build_memory' THEN 3
			WHEN 'retrieve_shared_memory' THEN 4 WHEN 'plan_book' THEN 5
			WHEN 'write_book' THEN 6 WHEN 'render_pdf' THEN 7 END`, run.ID)
	if err != nil {
		return Run{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var node Node
		var name, nodeState string
		if err := rows.Scan(&name, &nodeState, &node.ErrorCode, &node.Attempts); err != nil {
			return Run{}, err
		}
		node.Name, node.State = NodeName(name), NodeState(nodeState)
		run.Nodes = append(run.Nodes, node)
	}
	return run, rows.Err()
}

func nextNode(current NodeName) (NodeName, bool) {
	for index, node := range NodeSequence {
		if node == current && index+1 < len(NodeSequence) {
			return NodeSequence[index+1], true
		}
	}
	return "", false
}
