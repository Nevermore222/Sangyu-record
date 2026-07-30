package workflow

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrProcessorMissing     = errors.New("workflow processor is not registered")
	ErrRendererUnavailable  = errors.New("renderer is not available")
	ErrInsufficientAssets   = errors.New("workflow requires uploaded audio and photo assets")
	ErrRunNotFound          = errors.New("workflow run not found")
	ErrNodeNotFound         = errors.New("workflow node not found")
	ErrSequenceEmpty        = errors.New("workflow node sequence is empty")
	ErrInvalidRun           = errors.New("workflow run is invalid")
	ErrConfirmationRequired = errors.New("finalization requires explicit material confirmation")
	ErrConsentRequired      = errors.New("project consent is required before finalization")
	ErrDraftVisitExists     = errors.New("project has an unfinished draft visit")
	ErrProjectNotFound      = errors.New("project was not found")
)

type RunKind string

const (
	RunKindBook          RunKind = "book"
	RunKindVisitAnalysis RunKind = "visit_analysis"
)

type NodeName string

const (
	NodeTranscribe           NodeName = "transcribe"
	NodeUnderstandPhoto      NodeName = "understand_photo"
	NodeBuildMemory          NodeName = "build_memory"
	NodeRetrieveSharedMemory NodeName = "retrieve_shared_memory"
	NodePlanBook             NodeName = "plan_book"
	NodeWriteBook            NodeName = "write_book"
	NodeRenderPDF            NodeName = "render_pdf"
	NodeVisitTranscribe      NodeName = "visit_transcribe"
	NodeVisitUnderstandPhoto NodeName = "visit_understand_photo"
	NodeVisitAssessMaterial  NodeName = "visit_assess_material"
	NodeVisitPlanFollowup    NodeName = "visit_plan_followup"
	NodeVisitPersistAnalysis NodeName = "visit_persist_analysis"
)

var NodeSequence = []NodeName{
	NodeTranscribe,
	NodeUnderstandPhoto,
	NodeBuildMemory,
	NodeRetrieveSharedMemory,
	NodePlanBook,
	NodeWriteBook,
	NodeRenderPDF,
}

var VisitAnalysisSequence = []NodeName{
	NodeVisitTranscribe,
	NodeVisitUnderstandPhoto,
	NodeVisitAssessMaterial,
	NodeVisitPlanFollowup,
	NodeVisitPersistAnalysis,
}

type CreateRunInput struct {
	ProjectID uuid.UUID
	VisitID   uuid.UUID
	Kind      RunKind
	Nodes     []NodeName
}

type FinalizeInput struct {
	ConfirmMaterialsReady bool `json:"confirm_materials_ready"`
}

type FinalizeBookRequest struct {
	ProjectID      uuid.UUID
	StaffID        uuid.UUID
	IncludeUnowned bool
	Nodes          []NodeName
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
	VisitID   uuid.UUID `json:"visit_id,omitempty"`
	Kind      RunKind   `json:"kind,omitempty"`
	Node      NodeName  `json:"node"`
}

type Run struct {
	ID        uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"project_id"`
	VisitID   uuid.UUID `json:"visit_id,omitempty"`
	Kind      RunKind   `json:"kind"`
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
	Position  int       `json:"position"`
}
