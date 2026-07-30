package assets

import (
	"context"
	"net/url"
	"time"

	"github.com/google/uuid"
)

const sourceURLTTL = 15 * time.Minute

type SourceAssetRepository interface {
	ListUploadedByKind(context.Context, uuid.UUID, Kind) ([]Asset, error)
	ListUploadedByVisitAndKind(context.Context, uuid.UUID, Kind) ([]Asset, error)
}

func (r *SourceReader) URLsByVisit(ctx context.Context, visitID uuid.UUID, kind Kind) ([]string, error) {
	items, err := r.repo.ListUploadedByVisitAndKind(ctx, visitID, kind)
	if err != nil {
		return nil, err
	}
	urls := make([]string, 0, len(items))
	for _, asset := range items {
		signed, err := r.store.PresignGet(ctx, r.bucket, asset.ObjectKey, sourceURLTTL)
		if err != nil {
			return nil, err
		}
		urls = append(urls, signed.String())
	}
	return urls, nil
}

type SourceURLStore interface {
	PresignGet(context.Context, string, string, time.Duration) (*url.URL, error)
}

type SourceReader struct {
	repo   SourceAssetRepository
	store  SourceURLStore
	bucket string
}

func NewSourceReader(repo SourceAssetRepository, store SourceURLStore, bucket string) *SourceReader {
	return &SourceReader{repo: repo, store: store, bucket: bucket}
}

func (r *SourceReader) URLs(ctx context.Context, projectID uuid.UUID, kind Kind) ([]string, error) {
	assets, err := r.repo.ListUploadedByKind(ctx, projectID, kind)
	if err != nil {
		return nil, err
	}
	urls := make([]string, 0, len(assets))
	for _, asset := range assets {
		signed, err := r.store.PresignGet(ctx, r.bucket, asset.ObjectKey, sourceURLTTL)
		if err != nil {
			return nil, err
		}
		urls = append(urls, signed.String())
	}
	return urls, nil
}
