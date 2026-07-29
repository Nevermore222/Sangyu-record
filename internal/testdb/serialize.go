package testdb

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

const sharedDatabaseLockID int64 = 7_302_026_073_000_001

// Serialize keeps tests that mutate the shared integration database from
// truncating tables underneath tests running in another Go package.
func Serialize(t testing.TB, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", sharedDatabaseLockID); err != nil {
		conn.Release()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", sharedDatabaseLockID)
		conn.Release()
	})
}
