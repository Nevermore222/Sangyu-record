package assets

import (
	"context"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
)

type MinioObjectStore struct {
	client        *minio.Client
	presignClient *minio.Client
}

func NewMinioObjectStore(client *minio.Client, presignClients ...*minio.Client) *MinioObjectStore {
	presignClient := client
	if len(presignClients) > 0 {
		presignClient = presignClients[0]
	}
	return &MinioObjectStore{client: client, presignClient: presignClient}
}

func (s *MinioObjectStore) PresignPut(ctx context.Context, bucket, key, _ string, expiry time.Duration) (*url.URL, error) {
	return s.presignClient.PresignedPutObject(ctx, bucket, key, expiry)
}

func (s *MinioObjectStore) PresignGet(ctx context.Context, bucket, key string, expiry time.Duration) (*url.URL, error) {
	return s.presignClient.PresignedGetObject(ctx, bucket, key, expiry, nil)
}

func (s *MinioObjectStore) Stat(ctx context.Context, bucket, key string) (ObjectInfo, error) {
	info, err := s.client.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return ObjectInfo{}, err
	}
	return ObjectInfo{Size: info.Size, ContentType: info.ContentType}, nil
}
