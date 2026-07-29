package assets

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	maxAudioSize = int64(2 * 1024 * 1024 * 1024)
	maxPhotoSize = int64(30 * 1024 * 1024)
	uploadTTL    = 30 * time.Minute
)

var sha256Pattern = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

type Repository interface {
	Create(context.Context, Asset) error
	Get(context.Context, uuid.UUID) (Asset, error)
	MarkUploaded(context.Context, uuid.UUID, string, time.Time) (Asset, error)
	ListByVisit(context.Context, uuid.UUID) ([]Asset, error)
	DeletePending(context.Context, uuid.UUID) (Asset, error)
}

type ObjectStore interface {
	PresignPut(context.Context, string, string, string, time.Duration) (*url.URL, error)
	Stat(context.Context, string, string) (ObjectInfo, error)
	Remove(context.Context, string, string) error
}

type Service struct {
	repo         Repository
	store        ObjectStore
	bucket       string
	requireVisit bool
}

func NewService(repo Repository, store ObjectStore, bucket string) *Service {
	return &Service{repo: repo, store: store, bucket: bucket}
}

func NewServiceWithConfig(repo Repository, store ObjectStore, bucket string, requireVisit bool) *Service {
	return &Service{repo: repo, store: store, bucket: bucket, requireVisit: requireVisit}
}

func (s *Service) Initiate(ctx context.Context, input InitiateInput) (UploadTicket, error) {
	if err := validateInitiateInput(input); err != nil {
		return UploadTicket{}, err
	}
	if s.requireVisit && input.VisitID == nil {
		return UploadTicket{}, fmt.Errorf("%w: visit_id is required", ErrValidation)
	}

	now := time.Now().UTC()
	assetID := uuid.New()
	filename := sanitizeFilename(input.Filename)
	displayName := strings.TrimSpace(input.DisplayName)
	if displayName == "" {
		displayName = strings.TrimSpace(input.Filename)
	}
	asset := Asset{
		ID:          assetID,
		ProjectID:   input.ProjectID,
		VisitID:     input.VisitID,
		Kind:        input.Kind,
		Source:      input.Source,
		Filename:    filename,
		DisplayName: displayName,
		ContentType: input.ContentType,
		SizeBytes:   input.SizeBytes,
		ObjectKey:   fmt.Sprintf("projects/%s/source/%s/%s", input.ProjectID, assetID, filename),
		State:       StatePendingUpload,
		CreatedAt:   now,
		PlanItemIDs: uniqueAssetIDs(input.PlanItemIDs),
	}
	if err := s.repo.Create(ctx, asset); err != nil {
		return UploadTicket{}, err
	}
	uploadURL, err := s.store.PresignPut(ctx, s.bucket, asset.ObjectKey, asset.ContentType, uploadTTL)
	if err != nil {
		return UploadTicket{}, err
	}
	return UploadTicket{AssetID: asset.ID, UploadURL: uploadURL.String(), ExpiresAt: now.Add(uploadTTL)}, nil
}

func (s *Service) Get(ctx context.Context, assetID uuid.UUID) (Asset, error) {
	return s.repo.Get(ctx, assetID)
}

func (s *Service) RenewUpload(ctx context.Context, assetID uuid.UUID) (UploadTicket, error) {
	asset, err := s.repo.Get(ctx, assetID)
	if err != nil {
		return UploadTicket{}, err
	}
	if asset.State != StatePendingUpload {
		return UploadTicket{}, ErrInvalidState
	}
	now := time.Now().UTC()
	uploadURL, err := s.store.PresignPut(ctx, s.bucket, asset.ObjectKey, asset.ContentType, uploadTTL)
	if err != nil {
		return UploadTicket{}, err
	}
	return UploadTicket{AssetID: asset.ID, UploadURL: uploadURL.String(), ExpiresAt: now.Add(uploadTTL)}, nil
}

func (s *Service) DeletePending(ctx context.Context, assetID uuid.UUID) error {
	asset, err := s.repo.DeletePending(ctx, assetID)
	if err != nil {
		return err
	}
	_ = s.store.Remove(ctx, s.bucket, asset.ObjectKey)
	return nil
}

func (s *Service) ListByVisit(ctx context.Context, visitID uuid.UUID) ([]Asset, error) {
	if visitID == uuid.Nil {
		return nil, fmt.Errorf("%w: visit ID is required", ErrValidation)
	}
	return s.repo.ListByVisit(ctx, visitID)
}

func (s *Service) Complete(ctx context.Context, assetID uuid.UUID, sha256 string) (Asset, error) {
	if !sha256Pattern.MatchString(sha256) {
		return Asset{}, fmt.Errorf("%w: sha256 must contain 64 hexadecimal characters", ErrValidation)
	}
	sha256 = strings.ToLower(sha256)
	asset, err := s.repo.Get(ctx, assetID)
	if err != nil {
		return Asset{}, err
	}
	if asset.State == StateUploaded {
		if asset.SHA256 != sha256 {
			return Asset{}, ErrHashConflict
		}
		return asset, nil
	}

	info, err := s.store.Stat(ctx, s.bucket, asset.ObjectKey)
	if err != nil {
		return Asset{}, err
	}
	if info.Size != asset.SizeBytes || info.ContentType != asset.ContentType {
		return Asset{}, ErrUploadMismatch
	}
	return s.repo.MarkUploaded(ctx, assetID, sha256, time.Now().UTC())
}

func validateInitiateInput(input InitiateInput) error {
	if input.ProjectID == uuid.Nil || strings.TrimSpace(input.Filename) == "" || input.SizeBytes <= 0 {
		return fmt.Errorf("%w: project_id, filename, and positive size_bytes are required", ErrValidation)
	}
	allowed := map[Kind]map[string]bool{
		KindAudio: {"audio/mpeg": true, "audio/mp4": true, "audio/wav": true},
		KindPhoto: {"image/jpeg": true, "image/png": true},
	}
	if !allowed[input.Kind][input.ContentType] {
		return ErrUnsupportedContentType
	}
	if input.VisitID != nil {
		if *input.VisitID == uuid.Nil {
			return fmt.Errorf("%w: visit_id cannot be empty", ErrValidation)
		}
		if !validSource(input.Source) {
			return fmt.Errorf("%w: source is required for visit assets", ErrValidation)
		}
	} else if input.Source != "" && !validSource(input.Source) {
		return fmt.Errorf("%w: unknown asset source", ErrValidation)
	}
	if input.Kind == KindAudio && input.SizeBytes > maxAudioSize {
		return fmt.Errorf("%w: audio exceeds 2 GiB", ErrValidation)
	}
	if input.Kind == KindPhoto && input.SizeBytes > maxPhotoSize {
		return fmt.Errorf("%w: photo exceeds 30 MiB", ErrValidation)
	}
	return nil
}

func validSource(source Source) bool {
	switch source {
	case SourceDirect, SourceWeChatFile, SourceAlbum, SourceCamera:
		return true
	default:
		return false
	}
}

func uniqueAssetIDs(values []uuid.UUID) []uuid.UUID {
	result := make([]uuid.UUID, 0, len(values))
	seen := make(map[uuid.UUID]struct{}, len(values))
	for _, value := range values {
		if value == uuid.Nil {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func sanitizeFilename(filename string) string {
	filename = path.Base(strings.ReplaceAll(strings.TrimSpace(filename), "\\", "/"))
	var builder strings.Builder
	for _, character := range filename {
		if character == '.' || character == '-' || character == '_' ||
			character >= '0' && character <= '9' ||
			character >= 'A' && character <= 'Z' ||
			character >= 'a' && character <= 'z' {
			builder.WriteRune(character)
		} else {
			builder.WriteRune('_')
		}
	}
	if builder.Len() == 0 {
		return "asset"
	}
	return builder.String()
}
