package providerjobs

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
)

type ObjectPutter interface {
	PutObject(context.Context, string, string, io.Reader, int64, minio.PutObjectOptions) (minio.UploadInfo, error)
}

type RawStore interface {
	Put(context.Context, Job, int, []byte) (string, error)
}

type MinioRawStore struct {
	client ObjectPutter
	bucket string
}

func NewMinioRawStore(client ObjectPutter, bucket string) *MinioRawStore {
	return &MinioRawStore{client: client, bucket: bucket}
}

func (s *MinioRawStore) Put(ctx context.Context, job Job, attempt int, raw []byte) (string, error) {
	key := fmt.Sprintf("projects/%s/provider-jobs/%s/attempts/%d.json", job.ProjectID, job.ID, attempt)
	_, err := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(raw), int64(len(raw)), minio.PutObjectOptions{ContentType: "application/json"})
	return key, err
}
