package book

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

type fakePDFEngine struct {
	data []byte
}

func (e fakePDFEngine) Render(_ context.Context, _ string) ([]byte, error) {
	return e.data, nil
}

type memoryArtifactStore struct {
	key  string
	data []byte
}

func (s *memoryArtifactStore) Put(_ context.Context, _ string, key string, data []byte, _ string) error {
	s.key, s.data = key, append([]byte(nil), data...)
	return nil
}

type memoryArtifactRepository struct {
	saved Artifact
}

func (r *memoryArtifactRepository) NextVersion(_ context.Context, _ uuid.UUID, _ string) (int, error) {
	return 1, nil
}

func (r *memoryArtifactRepository) Save(_ context.Context, artifact Artifact) error {
	r.saved = artifact
	return nil
}

func (r *memoryArtifactRepository) Latest(_ context.Context, _ uuid.UUID) (Artifact, error) {
	return r.saved, nil
}

func (s *memoryArtifactStore) PresignGet(_ context.Context, _ string, _ string) (string, error) {
	return "http://localhost:9000/download", nil
}

func TestServiceStoresValidPDFBeforeSavingArtifact(t *testing.T) {
	store := &memoryArtifactStore{}
	repo := &memoryArtifactRepository{}
	service := NewService(fakePDFEngine{data: []byte("%PDF-1.4\nfixture")}, store, repo, "private")
	projectID, runID := uuid.New(), uuid.New()
	artifact, err := service.Render(context.Background(), projectID, runID, Manuscript{Title: "岁月留声"})
	if err != nil {
		t.Fatal(err)
	}
	if artifact.Version != 1 || repo.saved.ID != artifact.ID {
		t.Fatalf("artifact = %#v, saved = %#v", artifact, repo.saved)
	}
	wantKey := "projects/" + projectID.String() + "/artifacts/1/memoir.pdf"
	if store.key != wantKey {
		t.Fatalf("object key = %q, want %q", store.key, wantKey)
	}
}
