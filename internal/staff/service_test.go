package staff

import (
	"context"
	"errors"
	"testing"
	"time"
)

type memoryRepository struct {
	staffByOpenID map[string]Staff
	sessions      map[string]Session
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{staffByOpenID: make(map[string]Staff), sessions: make(map[string]Session)}
}

func (r *memoryRepository) UpsertStaff(_ context.Context, value Staff) (Staff, error) {
	if existing, ok := r.staffByOpenID[value.WeChatOpenID]; ok {
		return existing, nil
	}
	r.staffByOpenID[value.WeChatOpenID] = value
	return value, nil
}

func (r *memoryRepository) CreateSession(_ context.Context, session Session) error {
	r.sessions[session.TokenHash] = session
	return nil
}

func (r *memoryRepository) Authenticate(_ context.Context, tokenHash string, now time.Time) (Staff, error) {
	session, ok := r.sessions[tokenHash]
	if !ok || !session.ExpiresAt.After(now) {
		return Staff{}, ErrUnauthorized
	}
	for _, value := range r.staffByOpenID {
		if value.ID == session.StaffID && value.State == StateActive {
			return value, nil
		}
	}
	return Staff{}, ErrUnauthorized
}

func (r *memoryRepository) RevokeSession(_ context.Context, tokenHash string) error {
	delete(r.sessions, tokenHash)
	return nil
}

type fakeExchanger struct {
	openID string
	err    error
}

func (e fakeExchanger) Exchange(_ context.Context, _ string) (string, error) {
	return e.openID, e.err
}

func TestDevLoginCreatesReusableSession(t *testing.T) {
	now := time.Date(2026, 7, 30, 8, 0, 0, 0, time.UTC)
	repo := newMemoryRepository()
	service := NewService(repo, nil, Config{
		Mode: "dev", SessionTTL: 12 * time.Hour, SessionSecret: []byte("test-session-secret"),
	}, func() time.Time { return now })

	login, err := service.LoginDev(context.Background(), "本地采集员")
	if err != nil {
		t.Fatal(err)
	}
	if login.Token == "" {
		t.Fatal("login token is empty")
	}
	authenticated, err := service.Authenticate(context.Background(), login.Token)
	if err != nil {
		t.Fatal(err)
	}
	if authenticated.ID != login.Staff.ID || authenticated.DisplayName != "本地采集员" {
		t.Fatalf("authenticated = %#v, login = %#v", authenticated, login)
	}
}

func TestWechatLoginRejectsOpenIDOutsideAllowlist(t *testing.T) {
	service := NewService(newMemoryRepository(), fakeExchanger{openID: "not-allowed"}, Config{
		Mode: "wechat", SessionTTL: time.Hour, SessionSecret: []byte("test-session-secret"),
		AllowedOpenIDs: map[string]struct{}{"allowed": {}},
	}, time.Now)

	_, err := service.LoginWechat(context.Background(), "temporary-code")
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}

func TestLogoutRevokesSession(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(repo, nil, Config{
		Mode: "dev", SessionTTL: time.Hour, SessionSecret: []byte("test-session-secret"),
	}, time.Now)
	login, err := service.LoginDev(context.Background(), "采集员")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Logout(context.Background(), login.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(context.Background(), login.Token); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
}
