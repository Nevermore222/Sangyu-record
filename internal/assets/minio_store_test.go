package assets

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/nevermore222/sangyu-record/internal/platform"
)

func TestMinioObjectStorePresignAndStat(t *testing.T) {
	endpoint := os.Getenv("TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("TEST_S3_ENDPOINT is not set")
	}
	client, err := platform.NewObjectStore(platform.ObjectStoreConfig{
		Endpoint: endpoint, AccessKey: "sangyu", SecretKey: "sangyu-local-secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	store := NewMinioObjectStore(client)
	key := "tests/" + uuid.NewString() + "/photo.jpg"
	ctx := context.Background()
	defer func() { _ = client.RemoveObject(ctx, "sangyu-private", key, minio.RemoveObjectOptions{}) }()

	uploadURL, err := store.PresignPut(ctx, "sangyu-private", key, "image/jpeg", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL.String(), bytes.NewReader([]byte("image-bytes")))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "image/jpeg")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode/100 != 2 {
		t.Fatalf("upload status = %d", response.StatusCode)
	}

	info, err := store.Stat(ctx, "sangyu-private", key)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size != int64(len("image-bytes")) || info.ContentType != "image/jpeg" {
		t.Fatalf("info = %#v", info)
	}
}
