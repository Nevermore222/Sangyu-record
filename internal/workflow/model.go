package workflow

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrProcessorMissing    = errors.New("workflow processor is not registered")
	ErrRendererUnavailable = errors.New("renderer is not available")
	ErrInsufficientAssets  = errors.New("workflow requires uploaded audio and photo assets")
	ErrRunNotFound         = errors.New("workflow run not found")
	ErrNodeNotFound        = errors.New("workflow node not found")
)

type NodeName string

const (
	NodeTranscribe      NodeName = "transcribe"
	NodeUnderstandPhoto NodeName = "understand_photo"
	NodeBuildMemory     NodeName = "build_memory"
	NodePlanBook        NodeName = "plan_book"
	NodeWriteBook       NodeName = "write_book"
	NodeRenderPDF       NodeName = "render_pdf"
)

var NodeSequence = []NodeName{
	NodeTranscribe,
	NodeUnderstandPhoto,
	NodeBuildMemory,
	NodePlanBook,
	NodeWriteBook,
	NodeRenderPDF,
}

type NodeState string

const (
	NodeQueued    NodeState = "queued"
	NodeRunning   NodeState = "running"
	NodeSucceeded NodeState = "succeeded"
	NodeFailed    NodeState = "failed"
)

type NodePayload struct {
	RunID     uuid.UUID `json:"run_id"`
	ProjectID uuid.UUID `json:"project_id"`
	Node      NodeName  `json:"node"`
}

type Run struct {
	ID        uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"project_id"`
	State     NodeState `json:"state"`
	ErrorCode string    `json:"error_code,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Nodes     []Node    `json:"nodes"`
}

type Node struct {
	Name      NodeName  `json:"name"`
	State     NodeState `json:"state"`
	ErrorCode string    `json:"error_code,omitempty"`
	Attempts  int       `json:"attempts"`
}
