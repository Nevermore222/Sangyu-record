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

const StateCollecting State = "collecting"

type PlanItemStatus string

const PlanPending PlanItemStatus = "pending"

type CreateInput struct {
	DisplayName       string `json:"display_name"`
	BirthYear         int    `json:"birth_year"`
	BirthPlace        string `json:"birth_place"`
	LongTermResidence string `json:"long_term_residence"`
	PrimaryOccupation string `json:"primary_occupation"`
	TargetEdition     string `json:"target_edition"`
}

type Project struct {
	ID                uuid.UUID `json:"id"`
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
	Position  int            `json:"position"`
	CreatedAt time.Time      `json:"created_at"`
}

type ProjectDetail struct {
	Project
	CollectionPlan []PlanItem `json:"collection_plan"`
}
