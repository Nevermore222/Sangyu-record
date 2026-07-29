package assets

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
)

type memoryRepository struct {
	items map[uuid.UUID]Asset
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{items: make(map[uuid.UUID]Asset)}
}

func (r *memoryRepository) Create(_ context.Context, asset Asset) error {
	r.items[asset.ID] = asset
	return nil
}

func (r *memoryRepository) Get(_ context.Context, id uuid.UUID) (Asset, error) {
	asset, ok := r.items[id]
	if !ok {
		return Asset{}, ErrNotFound
	}
	return asset, nil
}

func (r *memoryRepository) MarkUploaded(_ context.Context, id uuid.UUID, sha256 string, uploadedAt time.Time) (Asset, error) {
	asset, ok := r.items[id]
	if !ok {
		return Asset{}, ErrNotFound
	}
	if asset.State == StateUploaded && asset.SHA256 != sha256 {
		return Asset{}, ErrHashConflict
	}
	asset.State = StateUploaded
	asset.SHA256 = sha256
	asset.UploadedAt = &uploadedAt
	r.items[id] = asset
	return asset, nil
}

func (r *memoryRepository) ListByVisit(_ context.Context, visitID uuid.UUID) ([]Asset, error) {
	items := make([]Asset, 0)
	for _, asset := range r.items {
		if asset.VisitID != nil && *asset.VisitID == visitID {
			items = append(items, asset)
		}
	}
	return items, nil
}

func (r *memoryRepository) DeletePending(_ context.Context, id uuid.UUID) (Asset, error) {
	asset, ok := r.items[id]
	if !ok {
		return Asset{}, ErrNotFound
	}
	if asset.State != StatePendingUpload {
		return Asset{}, ErrInvalidState
	}
	delete(r.items, id)
	return asset, nil
}

type fakeObjectStore struct {
	info ObjectInfo
}

func (s *fakeObjectStore) PresignPut(_ context.Context, _ string, _ string, _ string, _ time.Duration) (*url.URL, error) {
	return url.Parse("http://object-store/upload")
}

func (s *fakeObjectStore) Stat(_ context.Context, _ string, _ string) (ObjectInfo, error) {
	return s.info, nil
}

func (s *fakeObjectStore) Remove(_ context.Context, _ string, _ string) error {
	return nil
}

func TestInitiateRejectsUnsupportedContentType(t *testing.T) {
	service := NewService(newMemoryRepository(), &fakeObjectStore{}, "private")
	_, err := service.Initiate(context.Background(), InitiateInput{
		ProjectID:   uuid.New(),
		Kind:        KindAudio,
		Filename:    "interview.exe",
		ContentType: "application/octet-stream",
		SizeBytes:   100,
	})
	if !errors.Is(err, ErrUnsupportedContentType) {
		t.Fatalf("err = %v, want ErrUnsupportedContentType", err)
	}
}

func TestConfiguredServiceRequiresVisit(t *testing.T) {
	service := NewServiceWithConfig(newMemoryRepository(), &fakeObjectStore{}, "private", true)
	_, err := service.Initiate(context.Background(), InitiateInput{
		ProjectID: uuid.New(), Kind: KindAudio, Source: SourceDirect,
		Filename: "interview.wav", ContentType: "audio/wav", SizeBytes: 100,
	})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("err = %v, want ErrValidation", err)
	}
}

func TestCompleteIsIdempotentAndKeepsObjectKey(t *testing.T) {
	repo := newMemoryRepository()
	store := &fakeObjectStore{info: ObjectInfo{Size: 100, ContentType: "audio/wav"}}
	service := NewService(repo, store, "private")
	ticket, err := service.Initiate(context.Background(), InitiateInput{
		ProjectID: uuid.New(), Kind: KindAudio, Filename: "interview.wav",
		ContentType: "audio/wav", SizeBytes: 100,
	})
	if err != nil {
		t.Fatal(err)
	}

	hash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	first, err := service.Complete(context.Background(), ticket.AssetID, hash)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Complete(context.Background(), ticket.AssetID, hash)
	if err != nil {
		t.Fatal(err)
	}
	if first.ObjectKey != second.ObjectKey {
		t.Fatal("completion changed immutable object key")
	}
}

func TestCompleteRejectsDifferentHash(t *testing.T) {
	repo := newMemoryRepository()
	store := &fakeObjectStore{info: ObjectInfo{Size: 100, ContentType: "audio/wav"}}
	service := NewService(repo, store, "private")
	ticket, err := service.Initiate(context.Background(), InitiateInput{
		ProjectID: uuid.New(), Kind: KindAudio, Filename: "interview.wav",
		ContentType: "audio/wav", SizeBytes: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Complete(context.Background(), ticket.AssetID, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); err != nil {
		t.Fatal(err)
	}
	_, err = service.Complete(context.Background(), ticket.AssetID, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if !errors.Is(err, ErrHashConflict) {
		t.Fatalf("err = %v, want ErrHashConflict", err)
	}
}

func TestRenewUploadKeepsAssetIdentityAndObjectKey(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(repo, &fakeObjectStore{}, "private")
	ticket, err := service.Initiate(context.Background(), InitiateInput{
		ProjectID: uuid.New(), Kind: KindAudio, Filename: "interview.wav",
		ContentType: "audio/wav", SizeBytes: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	before := repo.items[ticket.AssetID]
	renewed, err := service.RenewUpload(context.Background(), ticket.AssetID)
	if err != nil {
		t.Fatal(err)
	}
	after := repo.items[ticket.AssetID]
	if renewed.AssetID != ticket.AssetID || after.ObjectKey != before.ObjectKey {
		t.Fatalf("renewed = %#v, before = %#v, after = %#v", renewed, before, after)
	}
}

func TestDeletePendingRejectsUploadedAsset(t *testing.T) {
	repo := newMemoryRepository()
	store := &fakeObjectStore{info: ObjectInfo{Size: 100, ContentType: "audio/wav"}}
	service := NewService(repo, store, "private")
	ticket, err := service.Initiate(context.Background(), InitiateInput{
		ProjectID: uuid.New(), Kind: KindAudio, Filename: "interview.wav",
		ContentType: "audio/wav", SizeBytes: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Complete(context.Background(), ticket.AssetID, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); err != nil {
		t.Fatal(err)
	}
	if err := service.DeletePending(context.Background(), ticket.AssetID); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("err = %v, want ErrInvalidState", err)
	}
}
