package config

import "testing"

func TestLoadRequiresServiceEndpoints(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("REDIS_URL", "")
	t.Setenv("S3_ENDPOINT", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want required-variable error")
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("REDIS_URL", "redis://example")
	t.Setenv("S3_ENDPOINT", "minio:9000")
	t.Setenv("HTTP_ADDRESS", "")
	t.Setenv("S3_BUCKET", "")
	t.Setenv("S3_PUBLIC_ENDPOINT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddress != ":8080" {
		t.Fatalf("HTTPAddress = %q", cfg.HTTPAddress)
	}
	if cfg.S3Bucket != "sangyu-private" {
		t.Fatalf("S3Bucket = %q", cfg.S3Bucket)
	}
	if cfg.S3PublicEndpoint != "minio:9000" {
		t.Fatalf("S3PublicEndpoint = %q", cfg.S3PublicEndpoint)
	}
	if cfg.S3Region != "us-east-1" {
		t.Fatalf("S3Region = %q", cfg.S3Region)
	}
}
