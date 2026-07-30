package assets

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeSourceAssetRepository struct {
	projectID uuid.UUID
	kind      Kind
	assets    []Asset
}

func (r *fakeSourceAssetRepository) ListUploadedByKind(_ context.Context, projectID uuid.UUID, kind Kind) ([]Asset, error) {
	r.projectID, r.kind = projectID, kind
	return r.assets, nil
}

func (r *fakeSourceAssetRepository) ListUploadedByVisitAndKind(_ context.Context, _ uuid.UUID, kind Kind) ([]Asset, error) {
	r.projectID = uuid.Nil
	r.kind = kind
	return r.assets, nil
}

type fakeSourceURLStore struct {
	keys   []string
	expiry time.Duration
}

func (s *fakeSourceURLStore) PresignGet(_ context.Context, _, key string, expiry time.Duration) (*url.URL, error) {
	s.keys = append(s.keys, key)
	s.expiry = expiry
	return url.Parse("https://objects.example/" + key)
}

func TestSourceReaderSignsRequestedUploadedAssets(t *testing.T) {
	projectID := uuid.New()
	repo := &fakeSourceAssetRepository{assets: []Asset{
		{ObjectKey: "projects/test/source/one.wav"},
		{ObjectKey: "projects/test/source/two.wav"},
	}}
	store := &fakeSourceURLStore{}
	reader := NewSourceReader(repo, store, "private")

	urls, err := reader.URLs(context.Background(), projectID, KindAudio)
	if err != nil {
		t.Fatal(err)
	}
	if repo.projectID != projectID || repo.kind != KindAudio {
		t.Fatalf("query = %s/%s", repo.projectID, repo.kind)
	}
	if len(urls) != 2 || len(store.keys) != 2 || store.expiry != 15*time.Minute {
		t.Fatalf("urls=%#v keys=%#v expiry=%s", urls, store.keys, store.expiry)
	}
}
