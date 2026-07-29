package providerjobs

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/nevermore222/sangyu-record/internal/providers"
)

var (
	ErrNotFound         = errors.New("provider job not found")
	ErrTerminalConflict = errors.New("provider terminal state conflicts with update")
)

type Job struct {
	ID                   uuid.UUID
	RequestID            uuid.UUID
	ProjectID            uuid.UUID
	WorkflowRunID        uuid.UUID
	WorkflowNode         string
	ProviderKind         providers.Kind
	TaskType             providers.TaskType
	IdempotencyKey       string
	ProviderJobID        string
	State                providers.State
	Input                json.RawMessage
	NormalizedOutput     json.RawMessage
	RawResponseObjectKey string
	ErrorCode            string
	ErrorMessage         string
	Deadline             time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type CreateInput struct {
	RequestID      uuid.UUID
	ProjectID      uuid.UUID
	WorkflowRunID  uuid.UUID
	WorkflowNode   string
	ProviderKind   providers.Kind
	TaskType       providers.TaskType
	IdempotencyKey string
	Input          json.RawMessage
	Deadline       time.Time
}

type Attempt struct {
	ID                   uuid.UUID
	ProviderJobID        uuid.UUID
	Attempt              int
	Operation            string
	State                providers.State
	HTTPStatus           int
	RawResponseObjectKey string
	ErrorCode            string
	ElapsedMS            int64
	CreatedAt            time.Time
}

type Outcome struct {
	JobID         uuid.UUID
	ProjectID     uuid.UUID
	WorkflowRunID uuid.UUID
	WorkflowNode  string
	State         providers.State
	Output        json.RawMessage
	ErrorCode     string
}

type Repository interface {
	CreateOrGet(context.Context, CreateInput) (Job, bool, error)
	MarkSubmitted(context.Context, uuid.UUID, providers.JobRef) error
	Get(context.Context, uuid.UUID) (Job, error)
	StartAttempt(context.Context, uuid.UUID, string, providers.State) (Attempt, error)
	FinishAttempt(context.Context, Attempt) error
	ApplySnapshot(context.Context, uuid.UUID, providers.Snapshot, string) error
	ConsumeTerminal(context.Context, uuid.UUID) (Outcome, bool, error)
}
