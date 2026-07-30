package book

import (
	"context"

	"github.com/google/uuid"
)

type LatestArtifactRepository interface {
	LatestOwned(context.Context, uuid.UUID, uuid.UUID, bool) (Artifact, error)
}

type DownloadStore interface {
	PresignGet(context.Context, string, string) (string, error)
}

type Catalog struct {
	repo         LatestArtifactRepository
	store        DownloadStore
	bucket       string
	allowUnowned bool
}

func NewCatalog(repo LatestArtifactRepository, store DownloadStore, bucket string) *Catalog {
	return &Catalog{repo: repo, store: store, bucket: bucket}
}

func NewCatalogWithConfig(repo LatestArtifactRepository, store DownloadStore, bucket string, allowUnowned bool) *Catalog {
	return &Catalog{repo: repo, store: store, bucket: bucket, allowUnowned: allowUnowned}
}

func (c *Catalog) Latest(ctx context.Context, projectID, staffID uuid.UUID) (Artifact, error) {
	artifact, err := c.repo.LatestOwned(ctx, projectID, staffID, c.allowUnowned)
	if err != nil {
		return Artifact{}, err
	}
	artifact.DownloadURL, err = c.store.PresignGet(ctx, c.bucket, artifact.ObjectKey)
	return artifact, err
}
