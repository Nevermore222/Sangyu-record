package projects

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound   = errors.New("project not found")
	ErrValidation = errors.New("project validation failed")
)

type State string

const (
	StateCollecting    State = "collecting"
	StateProcessing    State = "processing"
	StateNeedsMaterial State = "needs_material"
	StateGenerating    State = "generating"
	StateQualityCheck  State = "quality_check"
	StateException     State = "exception"
	StatePDFRendering  State = "pdf_rendering"
	StateCompleted     State = "completed"
)

type PlanItemStatus string

const PlanPending PlanItemStatus = "pending"

type CreateInput struct {
	OwnerStaffID      uuid.UUID `json:"-"`
	DisplayName       string    `json:"display_name"`
	BirthYear         int       `json:"birth_year"`
	BirthPlace        string    `json:"birth_place"`
	LongTermResidence string    `json:"long_term_residence"`
	PrimaryOccupation string    `json:"primary_occupation"`
	TargetEdition     string    `json:"target_edition"`
}

type Project struct {
	ID                uuid.UUID `json:"id"`
	OwnerStaffID      uuid.UUID `json:"owner_staff_id,omitempty"`
	DisplayName       string    `json:"display_name"`
	BirthYear         int       `json:"birth_year"`
	BirthPlace        string    `json:"birth_place"`
	LongTermResidence string    `json:"long_term_residence"`
	PrimaryOccupation string    `json:"primary_occupation"`
	TargetEdition     string    `json:"target_edition"`
	State             State     `json:"state"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type PlanItem struct {
	ID        uuid.UUID      `json:"id"`
	ProjectID uuid.UUID      `json:"project_id"`
	Category  string         `json:"category"`
	Prompt    string         `json:"prompt"`
	Required  bool           `json:"required"`
	Status    PlanItemStatus `json:"status"`
	GapReason string         `json:"gap_reason,omitempty"`
	Position  int            `json:"position"`
	CreatedAt time.Time      `json:"created_at"`
}

type ProjectDetail struct {
	Project
	CollectionPlan []PlanItem `json:"collection_plan"`
	Consent        *Consent   `json:"consent,omitempty"`
}

type Consent struct {
	ID                 uuid.UUID `json:"id"`
	ProjectID          uuid.UUID `json:"project_id"`
	ConfirmedBy        string    `json:"confirmed_by"`
	ConfirmationMethod string    `json:"confirmation_method"`
	StaffID            uuid.UUID `json:"staff_id"`
	ConfirmedAt        time.Time `json:"confirmed_at"`
}

type ConfirmConsentInput struct {
	ConfirmedBy string `json:"confirmed_by"`
}

type ListInput struct {
	OwnerStaffID   uuid.UUID
	Query          string
	State          State
	Cursor         string
	Limit          int
	IncludeUnowned bool
}

type ProjectSummary struct {
	ID                uuid.UUID `json:"id"`
	OwnerStaffID      uuid.UUID `json:"owner_staff_id,omitempty"`
	DisplayName       string    `json:"display_name"`
	BirthYear         int       `json:"birth_year"`
	BirthPlace        string    `json:"birth_place"`
	LongTermResidence string    `json:"long_term_residence"`
	PrimaryOccupation string    `json:"primary_occupation"`
	TargetEdition     string    `json:"target_edition"`
	State             State     `json:"state"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type Page struct {
	Items      []ProjectSummary `json:"items"`
	NextCursor string           `json:"next_cursor,omitempty"`
}

type DashboardCounts struct {
	Collecting    int `json:"collecting"`
	NeedsMaterial int `json:"needs_material"`
	Processing    int `json:"processing"`
	Completed     int `json:"completed"`
}

type Dashboard struct {
	Counts     DashboardCounts  `json:"counts"`
	Actionable []ProjectSummary `json:"actionable"`
	Recent     []ProjectSummary `json:"recent"`
}
