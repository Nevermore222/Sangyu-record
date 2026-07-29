package providerjobs

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
)

type recordingPutter struct {
	bucket string
	key    string
	body   []byte
}

func (p *recordingPutter) PutObject(_ context.Context, bucket, key string, reader io.Reader, _ int64, _ minio.PutObjectOptions) (minio.UploadInfo, error) {
	p.bucket, p.key = bucket, key
	p.body, _ = io.ReadAll(reader)
	return minio.UploadInfo{}, nil
}

func TestRawStoreUsesProjectScopedAttemptPath(t *testing.T) {
	putter := &recordingPutter{}
	store := NewMinioRawStore(putter, "private")
	job := Job{ID: uuid.New(), ProjectID: uuid.New()}
	raw := []byte(`{"provider_specific":true}`)
	key, err := store.Put(context.Background(), job, 2, raw)
	if err != nil {
		t.Fatal(err)
	}
	want := "projects/" + job.ProjectID.String() + "/provider-jobs/" + job.ID.String() + "/attempts/2.json"
	if key != want || putter.bucket != "private" || !bytes.Equal(putter.body, raw) {
		t.Fatalf("key=%q bucket=%q body=%s", key, putter.bucket, putter.body)
	}
}
