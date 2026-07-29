package staff

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) UpsertStaff(ctx context.Context, value Staff) (Staff, error) {
	var state string
	err := r.pool.QueryRow(ctx, `
		INSERT INTO staff (
			id, wechat_openid, display_name, team_name, state, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (wechat_openid) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			updated_at = EXCLUDED.updated_at
		RETURNING id, wechat_openid, display_name, team_name, state, created_at, updated_at`,
		value.ID, value.WeChatOpenID, value.DisplayName, value.TeamName, value.State,
		value.CreatedAt, value.UpdatedAt,
	).Scan(
		&value.ID, &value.WeChatOpenID, &value.DisplayName, &value.TeamName,
		&state, &value.CreatedAt, &value.UpdatedAt,
	)
	if err != nil {
		return Staff{}, err
	}
	value.State = State(state)
	return value, nil
}

func (r *PostgresRepository) CreateSession(ctx context.Context, session Session) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO staff_sessions (
			id, staff_id, token_hash, expires_at, created_at, last_seen_at
		) VALUES ($1, $2, $3, $4, $5, $6)`,
		session.ID, session.StaffID, session.TokenHash, session.ExpiresAt,
		session.CreatedAt, session.LastSeenAt,
	)
	return err
}

func (r *PostgresRepository) Authenticate(ctx context.Context, tokenHash string, now time.Time) (Staff, error) {
	var value Staff
	var state string
	err := r.pool.QueryRow(ctx, `
		WITH matched AS (
			SELECT staff.id, staff.wechat_openid, staff.display_name, staff.team_name,
			       staff.state, staff.created_at, staff.updated_at
			FROM staff_sessions AS sessions
			JOIN staff ON staff.id = sessions.staff_id
			WHERE sessions.token_hash = $1 AND sessions.expires_at > $2
		), touched AS (
			UPDATE staff_sessions
			SET last_seen_at = $2
			WHERE token_hash = $1
			  AND EXISTS (SELECT 1 FROM matched WHERE state = 'active')
			RETURNING id
		)
		SELECT matched.id, matched.wechat_openid, matched.display_name, matched.team_name,
		       matched.state, matched.created_at, matched.updated_at
		FROM matched
		LEFT JOIN touched ON true`,
		tokenHash, now,
	).Scan(
		&value.ID, &value.WeChatOpenID, &value.DisplayName, &value.TeamName,
		&state, &value.CreatedAt, &value.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Staff{}, ErrUnauthorized
	}
	if err != nil {
		return Staff{}, err
	}
	value.State = State(state)
	if value.State == StateDisabled {
		return Staff{}, ErrForbidden
	}
	return value, nil
}

func (r *PostgresRepository) RevokeSession(ctx context.Context, tokenHash string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM staff_sessions WHERE token_hash = $1`, tokenHash)
	return err
}
