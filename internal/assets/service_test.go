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

type fakeObjectStore struct {
	info ObjectInfo
}

func (s *fakeObjectStore) PresignPut(_ context.Context, _ string, _ string, _ string, _ time.Duration) (*url.URL, error) {
	return url.Parse("http://object-store/upload")
}

func (s *fakeObjectStore) Stat(_ context.Context, _ string, _ string) (ObjectInfo, error) {
	return s.info, nil
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
