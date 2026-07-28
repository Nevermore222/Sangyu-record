package assets

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound               = errors.New("asset not found")
	ErrValidation             = errors.New("asset validation failed")
	ErrUnsupportedContentType = errors.New("unsupported content type")
	ErrUploadMismatch         = errors.New("uploaded object does not match ticket")
	ErrHashConflict           = errors.New("asset hash conflicts with completed upload")
)

type Kind string

const (
	KindAudio     Kind = "audio"
	KindPhoto     Kind = "photo"
	KindStaffNote Kind = "staff_note"
)

type State string

const (
	StatePendingUpload State = "pending_upload"
	StateUploaded      State = "uploaded"
)

type InitiateInput struct {
	ProjectID   uuid.UUID `json:"-"`
	Kind        Kind      `json:"kind"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
}

type Asset struct {
	ID          uuid.UUID  `json:"id"`
	ProjectID   uuid.UUID  `json:"project_id"`
	Kind        Kind       `json:"kind"`
	Filename    string     `json:"filename"`
	ContentType string     `json:"content_type"`
	SizeBytes   int64      `json:"size_bytes"`
	ObjectKey   string     `json:"object_key"`
	SHA256      string     `json:"sha256,omitempty"`
	State       State      `json:"state"`
	CreatedAt   time.Time  `json:"created_at"`
	UploadedAt  *time.Time `json:"uploaded_at,omitempty"`
}

type UploadTicket struct {
	AssetID   uuid.UUID `json:"asset_id"`
	UploadURL string    `json:"upload_url"`
	ExpiresAt time.Time `json:"expires_at"`
}

type ObjectInfo struct {
	Size        int64
	ContentType string
}
