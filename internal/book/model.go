package book

import (
	"time"

	"github.com/google/uuid"
)

type Manuscript struct {
	Title    string    `json:"title"`
	Subtitle string    `json:"subtitle,omitempty"`
	Chapters []Chapter `json:"chapters"`
}

type Chapter struct {
	Title      string      `json:"title"`
	Paragraphs []Paragraph `json:"paragraphs"`
}

type Paragraph struct {
	Text         string   `json:"text"`
	EvidenceRefs []string `json:"evidence_refs"`
}

type Artifact struct {
	ID            uuid.UUID `json:"id"`
	ProjectID     uuid.UUID `json:"project_id"`
	WorkflowRunID uuid.UUID `json:"workflow_run_id"`
	Version       int       `json:"version"`
	Kind          string    `json:"kind"`
	ObjectKey     string    `json:"object_key"`
	ContentType   string    `json:"content_type"`
	SizeBytes     int64     `json:"size_bytes"`
	CreatedAt     time.Time `json:"created_at"`
	DownloadURL   string    `json:"download_url,omitempty"`
}
