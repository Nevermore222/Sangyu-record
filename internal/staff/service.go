package staff

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	UpsertStaff(context.Context, Staff) (Staff, error)
	CreateSession(context.Context, Session) error
	Authenticate(context.Context, string, time.Time) (Staff, error)
	RevokeSession(context.Context, string) error
}

type CodeExchanger interface {
	Exchange(context.Context, string) (string, error)
}

type Config struct {
	Mode           string
	AllowedOpenIDs map[string]struct{}
	SessionTTL     time.Duration
	SessionSecret  []byte
}

type Service struct {
	repo      Repository
	exchanger CodeExchanger
	config    Config
	now       func() time.Time
}

func NewService(repo Repository, exchanger CodeExchanger, config Config, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, exchanger: exchanger, config: config, now: now}
}

func (s *Service) LoginDev(ctx context.Context, displayName string) (LoginResult, error) {
	if s.config.Mode != "dev" {
		return LoginResult{}, ErrForbidden
	}
	return s.login(ctx, "dev-local", displayName)
}

func (s *Service) LoginWechat(ctx context.Context, code string) (LoginResult, error) {
	if s.config.Mode != "wechat" || s.exchanger == nil {
		return LoginResult{}, ErrForbidden
	}
	if strings.TrimSpace(code) == "" {
		return LoginResult{}, fmt.Errorf("%w: code is required", ErrValidation)
	}
	openID, err := s.exchanger.Exchange(ctx, code)
	if err != nil {
		return LoginResult{}, err
	}
	if _, allowed := s.config.AllowedOpenIDs[openID]; !allowed {
		return LoginResult{}, ErrForbidden
	}
	return s.login(ctx, openID, "微信采集员")
}

func (s *Service) login(ctx context.Context, openID, displayName string) (LoginResult, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return LoginResult{}, fmt.Errorf("%w: display name is required", ErrValidation)
	}
	now := s.now().UTC()
	value, err := s.repo.UpsertStaff(ctx, Staff{
		ID: uuid.New(), WeChatOpenID: openID, DisplayName: displayName,
		State: StateActive, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return LoginResult{}, err
	}
	if value.State != StateActive {
		return LoginResult{}, ErrForbidden
	}
	token, err := randomToken()
	if err != nil {
		return LoginResult{}, err
	}
	session := Session{
		ID: uuid.New(), StaffID: value.ID, TokenHash: s.hashToken(token),
		ExpiresAt: now.Add(s.config.SessionTTL), CreatedAt: now, LastSeenAt: now,
	}
	if err := s.repo.CreateSession(ctx, session); err != nil {
		return LoginResult{}, err
	}
	return LoginResult{Token: token, Staff: value}, nil
}

func (s *Service) Authenticate(ctx context.Context, token string) (Staff, error) {
	if strings.TrimSpace(token) == "" {
		return Staff{}, ErrUnauthorized
	}
	return s.repo.Authenticate(ctx, s.hashToken(token), s.now().UTC())
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if strings.TrimSpace(token) == "" {
		return ErrUnauthorized
	}
	return s.repo.RevokeSession(ctx, s.hashToken(token))
}

func (s *Service) hashToken(token string) string {
	mac := hmac.New(sha256.New, s.config.SessionSecret)
	_, _ = mac.Write([]byte(token))
	return hex.EncodeToString(mac.Sum(nil))
}

func randomToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
