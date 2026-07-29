package staff

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrUnauthorized = errors.New("staff session is unauthorized")
	ErrForbidden    = errors.New("staff access is forbidden")
	ErrValidation   = errors.New("staff request is invalid")
)

type State string

const (
	StateActive   State = "active"
	StateDisabled State = "disabled"
)

type Staff struct {
	ID           uuid.UUID `json:"id"`
	WeChatOpenID string    `json:"-"`
	DisplayName  string    `json:"display_name"`
	TeamName     string    `json:"team_name"`
	State        State     `json:"state"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Session struct {
	ID         uuid.UUID
	StaffID    uuid.UUID
	TokenHash  string
	ExpiresAt  time.Time
	CreatedAt  time.Time
	LastSeenAt time.Time
}

type LoginResult struct {
	Token string `json:"token"`
	Staff Staff  `json:"staff"`
}
