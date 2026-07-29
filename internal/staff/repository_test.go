package staff

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nevermore222/sangyu-record/internal/testdb"
)

func TestPostgresRepositoryAuthenticatesAndRevokesSession(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	testdb.Serialize(t, pool)

	repo := NewPostgresRepository(pool)
	now := time.Now().UTC()
	value, err := repo.UpsertStaff(context.Background(), Staff{
		ID: uuid.New(), WeChatOpenID: "repository-test-" + uuid.NewString(), DisplayName: "仓库测试员",
		State: StateActive, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	session := Session{
		ID: uuid.New(), StaffID: value.ID, TokenHash: uniqueTokenHash(),
		ExpiresAt: now.Add(time.Hour), CreatedAt: now, LastSeenAt: now,
	}
	if err := repo.CreateSession(context.Background(), session); err != nil {
		t.Fatal(err)
	}
	authenticated, err := repo.Authenticate(context.Background(), session.TokenHash, now)
	if err != nil || authenticated.ID != value.ID {
		t.Fatalf("authenticated = %#v, err = %v", authenticated, err)
	}
	if err := repo.RevokeSession(context.Background(), session.TokenHash); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Authenticate(context.Background(), session.TokenHash, now); err == nil {
		t.Fatal("revoked session authenticated")
	}
	_, _ = pool.Exec(context.Background(), "DELETE FROM staff WHERE id=$1", value.ID)
}

func TestPostgresRepositoryRejectsDisabledStaffAsForbidden(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	testdb.Serialize(t, pool)

	repo := NewPostgresRepository(pool)
	now := time.Now().UTC()
	value, err := repo.UpsertStaff(context.Background(), Staff{
		ID: uuid.New(), WeChatOpenID: "disabled-test-" + uuid.NewString(), DisplayName: "Disabled Tester",
		State: StateDisabled, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	session := Session{
		ID: uuid.New(), StaffID: value.ID, TokenHash: uniqueTokenHash(),
		ExpiresAt: now.Add(time.Hour), CreatedAt: now, LastSeenAt: now,
	}
	if err := repo.CreateSession(context.Background(), session); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Authenticate(context.Background(), session.TokenHash, now); !errors.Is(err, ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
	_, _ = pool.Exec(context.Background(), "DELETE FROM staff WHERE id=$1", value.ID)
}

func uniqueTokenHash() string {
	digest := sha256.Sum256([]byte(uuid.NewString()))
	return hex.EncodeToString(digest[:])
}
