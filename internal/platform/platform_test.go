package platform

import (
	"context"
	"os"
	"testing"
)

func TestPostgresConnection(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	pool, err := OpenPostgres(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestNewAsynqClientRejectsInvalidURL(t *testing.T) {
	if _, err := NewAsynqClient("not-a-redis-url"); err == nil {
		t.Fatal("NewAsynqClient() error = nil, want invalid URL error")
	}
}

func TestNewObjectStoreRejectsEndpointWithScheme(t *testing.T) {
	_, err := NewObjectStore(ObjectStoreConfig{
		Endpoint:  "http://localhost:9000",
		AccessKey: "key",
		SecretKey: "secret",
	})
	if err == nil {
		t.Fatal("NewObjectStore() error = nil, want endpoint format error")
	}
}
