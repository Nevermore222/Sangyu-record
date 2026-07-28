package book

import (
	"context"

	"github.com/google/uuid"
)

type LatestArtifactRepository interface {
	Latest(context.Context, uuid.UUID) (Artifact, error)
}

type DownloadStore interface {
	PresignGet(context.Context, string, string) (string, error)
}

type Catalog struct {
	repo   LatestArtifactRepository
	store  DownloadStore
	bucket string
}

func NewCatalog(repo LatestArtifactRepository, store DownloadStore, bucket string) *Catalog {
	return &Catalog{repo: repo, store: store, bucket: bucket}
}

func (c *Catalog) Latest(ctx context.Context, projectID uuid.UUID) (Artifact, error) {
	artifact, err := c.repo.Latest(ctx, projectID)
	if err != nil {
		return Artifact{}, err
	}
	artifact.DownloadURL, err = c.store.PresignGet(ctx, c.bucket, artifact.ObjectKey)
	return artifact, err
}
