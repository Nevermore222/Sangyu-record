package assets

import (
	"context"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
)

type MinioObjectStore struct {
	client *minio.Client
}

func NewMinioObjectStore(client *minio.Client) *MinioObjectStore {
	return &MinioObjectStore{client: client}
}

func (s *MinioObjectStore) PresignPut(ctx context.Context, bucket, key, _ string, expiry time.Duration) (*url.URL, error) {
	return s.client.PresignedPutObject(ctx, bucket, key, expiry)
}

func (s *MinioObjectStore) Stat(ctx context.Context, bucket, key string) (ObjectInfo, error) {
	info, err := s.client.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return ObjectInfo{}, err
	}
	return ObjectInfo{Size: info.Size, ContentType: info.ContentType}, nil
}
