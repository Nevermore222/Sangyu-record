package providers

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrKindNotConfigured = errors.New("provider kind is not configured")
	ErrInvalidOutput     = errors.New("provider output is invalid")
)

type Kind string

const (
	KindMedia     Kind = "media"
	KindKnowledge Kind = "knowledge"
	KindAgent     Kind = "agent"
)

type TaskType string

const (
	TaskAudioTranscription    TaskType = "audio_transcription"
	TaskSpeakerDiarization    TaskType = "speaker_diarization"
	TaskPhotoOCR              TaskType = "photo_ocr"
	TaskPhotoUnderstanding    TaskType = "photo_understanding"
	TaskCollectionPlan        TaskType = "collection_plan"
	TaskMaterialAssessment    TaskType = "material_assessment"
	TaskFollowupPlan          TaskType = "followup_plan"
	TaskMemoirPositioning     TaskType = "memoir_positioning"
	TaskTimelineBuilder       TaskType = "timeline_builder"
	TaskChapterPlanner        TaskType = "chapter_planner"
	TaskChapterWriter         TaskType = "chapter_writer"
	TaskChapterReview         TaskType = "chapter_review"
	TaskBookConsistencyReview TaskType = "book_consistency_review"
	TaskSharedMemoryRetrieval TaskType = "shared_memory_retrieval"
)

type State string

const (
	StatePendingSubmission State = "pending_submission"
	StateSubmitted         State = "submitted"
	StateProcessing        State = "processing"
	StateSucceeded         State = "succeeded"
	StateRetryableFailed   State = "retryable_failed"
	StatePermanentlyFailed State = "permanently_failed"
	StateTimedOut          State = "timed_out"
	StateCancelled         State = "cancelled"
)

func (s State) Terminal() bool {
	switch s {
	case StateSucceeded, StatePermanentlyFailed, StateTimedOut, StateCancelled:
		return true
	default:
		return false
	}
}

type SubmitRequest struct {
	RequestID           string          `json:"request_id"`
	IdempotencyKey      string          `json:"idempotency_key"`
	ContractVersion     string          `json:"contract_version"`
	TaskType            TaskType        `json:"task_type"`
	InputSchemaVersion  string          `json:"input_schema_version"`
	OutputSchemaVersion string          `json:"output_schema_version"`
	Input               json.RawMessage `json:"input"`
	ResourceURLs        []string        `json:"resource_urls"`
	CallbackURL         string          `json:"callback_url"`
	Deadline            time.Time       `json:"deadline"`
}

type JobRef struct {
	ProviderJobID string          `json:"provider_job_id"`
	State         State           `json:"state"`
	Raw           json.RawMessage `json:"-"`
}

type Snapshot struct {
	RequestID     string          `json:"request_id"`
	ProviderJobID string          `json:"provider_job_id"`
	State         State           `json:"state"`
	Output        json.RawMessage `json:"output,omitempty"`
	ErrorCode     string          `json:"error_code,omitempty"`
	ErrorMessage  string          `json:"error_message,omitempty"`
	Raw           json.RawMessage `json:"-"`
}

type Provider interface {
	Submit(context.Context, SubmitRequest) (JobRef, error)
	Status(context.Context, string) (Snapshot, error)
	Cancel(context.Context, string) error
}

type MediaProvider interface{ Provider }
type KnowledgeProvider interface{ Provider }
type AgentProvider interface{ Provider }

type Registry struct {
	Media     MediaProvider
	Knowledge KnowledgeProvider
	Agent     AgentProvider
}

func (r Registry) For(kind Kind) (Provider, error) {
	switch kind {
	case KindMedia:
		if r.Media != nil {
			return r.Media, nil
		}
	case KindKnowledge:
		if r.Knowledge != nil {
			return r.Knowledge, nil
		}
	case KindAgent:
		if r.Agent != nil {
			return r.Agent, nil
		}
	}
	return nil, ErrKindNotConfigured
}
