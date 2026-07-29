package visits

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound        = errors.New("visit not found")
	ErrValidation      = errors.New("visit validation failed")
	ErrConsentRequired = errors.New("project consent is required")
	ErrInvalidState    = errors.New("visit state does not allow this operation")
)

type State string

const (
	StateDraft     State = "draft"
	StateSubmitted State = "submitted"
	StateAnalyzing State = "analyzing"
	StateCompleted State = "completed"
	StateFailed    State = "failed"
)

type Visit struct {
	ID          uuid.UUID   `json:"id"`
	ProjectID   uuid.UUID   `json:"project_id"`
	Sequence    int         `json:"sequence"`
	StaffID     uuid.UUID   `json:"staff_id"`
	VisitedAt   time.Time   `json:"visited_at"`
	Location    string      `json:"location"`
	Notes       string      `json:"notes"`
	State       State       `json:"state"`
	ErrorCode   string      `json:"error_code,omitempty"`
	PlanItemIDs []uuid.UUID `json:"plan_item_ids"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

type CreateInput struct {
	ProjectID   uuid.UUID   `json:"-"`
	StaffID     uuid.UUID   `json:"-"`
	VisitedAt   time.Time   `json:"visited_at"`
	Location    string      `json:"location"`
	Notes       string      `json:"notes"`
	PlanItemIDs []uuid.UUID `json:"plan_item_ids"`
}

type UpdateInput struct {
	VisitedAt   *time.Time   `json:"visited_at,omitempty"`
	Location    *string      `json:"location,omitempty"`
	Notes       *string      `json:"notes,omitempty"`
	PlanItemIDs *[]uuid.UUID `json:"plan_item_ids,omitempty"`
}
