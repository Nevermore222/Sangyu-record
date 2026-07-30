package visitanalysis

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/nevermore222/sangyu-record/internal/providers"
)

var (
	ErrNotFound           = errors.New("visit analysis not found")
	ErrValidation         = errors.New("visit analysis is invalid")
	ErrInsufficientAssets = errors.New("visit requires at least one uploaded audio or photo")
	ErrInvalidState       = errors.New("visit state does not allow analysis")
)

type Analysis struct {
	ID                uuid.UUID                    `json:"id"`
	VisitID           uuid.UUID                    `json:"visit_id"`
	WorkflowRunID     uuid.UUID                    `json:"workflow_run_id"`
	Summary           string                       `json:"summary"`
	CoveredItems      []providers.CoveredItem      `json:"covered_items"`
	Gaps              []providers.MaterialGap      `json:"gaps"`
	FollowupQuestions []providers.FollowupQuestion `json:"followup_questions"`
	CreatedAt         time.Time                    `json:"created_at"`
	UpdatedAt         time.Time                    `json:"updated_at"`
}
