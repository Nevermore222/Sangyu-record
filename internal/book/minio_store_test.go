package book

import (
	"context"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/nevermore222/sangyu-record/internal/platform"
)

func TestMinioArtifactStorePutAndPresignGet(t *testing.T) {
	endpoint := os.Getenv("TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("TEST_S3_ENDPOINT is not set")
	}
	internalClient, err := platform.NewObjectStore(platform.ObjectStoreConfig{Endpoint: endpoint, AccessKey: "sangyu", SecretKey: "sangyu-local-secret"})
	if err != nil {
		t.Fatal(err)
	}
	publicClient, err := platform.NewObjectStore(platform.ObjectStoreConfig{Endpoint: "localhost:9000", AccessKey: "sangyu", SecretKey: "sangyu-local-secret"})
	if err != nil {
		t.Fatal(err)
	}
	store := NewMinioArtifactStore(internalClient, publicClient)
	key := "tests/" + uuid.NewString() + "/memoir.pdf"
	ctx := context.Background()
	defer func() { _ = internalClient.RemoveObject(ctx, "sangyu-private", key, minio.RemoveObjectOptions{}) }()
	data := []byte("%PDF-1.4\nfixture")
	if err := store.Put(ctx, "sangyu-private", key, data, "application/pdf"); err != nil {
		t.Fatal(err)
	}
	downloadURL, err := store.PresignGet(ctx, "sangyu-private", key)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.Get(downloadURL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	got, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Fatalf("downloaded = %q", got)
	}
}
