package book

import (
	"bytes"
	"context"
	"time"

	"github.com/minio/minio-go/v7"
)

type MinioArtifactStore struct {
	client        *minio.Client
	presignClient *minio.Client
}

func NewMinioArtifactStore(client, presignClient *minio.Client) *MinioArtifactStore {
	return &MinioArtifactStore{client: client, presignClient: presignClient}
}

func (s *MinioArtifactStore) Put(ctx context.Context, bucket, key string, data []byte, contentType string) error {
	_, err := s.client.PutObject(ctx, bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (s *MinioArtifactStore) PresignGet(ctx context.Context, bucket, key string) (string, error) {
	result, err := s.presignClient.PresignedGetObject(ctx, bucket, key, 15*time.Minute, nil)
	if err != nil {
		return "", err
	}
	return result.String(), nil
}
