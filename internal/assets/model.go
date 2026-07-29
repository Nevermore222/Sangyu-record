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
	ErrInvalidState           = errors.New("asset state does not allow this operation")
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

type Source string

const (
	SourceDirect     Source = "direct"
	SourceWeChatFile Source = "wechat_file"
	SourceAlbum      Source = "album"
	SourceCamera     Source = "camera"
)

type InitiateInput struct {
	ProjectID   uuid.UUID   `json:"-"`
	VisitID     *uuid.UUID  `json:"visit_id,omitempty"`
	Kind        Kind        `json:"kind"`
	Source      Source      `json:"source,omitempty"`
	Filename    string      `json:"filename"`
	DisplayName string      `json:"display_name,omitempty"`
	ContentType string      `json:"content_type"`
	SizeBytes   int64       `json:"size_bytes"`
	PlanItemIDs []uuid.UUID `json:"plan_item_ids,omitempty"`
}

type Asset struct {
	ID          uuid.UUID   `json:"id"`
	ProjectID   uuid.UUID   `json:"project_id"`
	VisitID     *uuid.UUID  `json:"visit_id,omitempty"`
	Kind        Kind        `json:"kind"`
	Source      Source      `json:"source,omitempty"`
	Filename    string      `json:"filename"`
	DisplayName string      `json:"display_name"`
	ContentType string      `json:"content_type"`
	SizeBytes   int64       `json:"size_bytes"`
	ObjectKey   string      `json:"object_key"`
	SHA256      string      `json:"sha256,omitempty"`
	State       State       `json:"state"`
	CreatedAt   time.Time   `json:"created_at"`
	UploadedAt  *time.Time  `json:"uploaded_at,omitempty"`
	PlanItemIDs []uuid.UUID `json:"plan_item_ids,omitempty"`
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
