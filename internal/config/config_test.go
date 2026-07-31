package config

import (
	"testing"
	"time"
)

func setRequiredBaseEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("REDIS_URL", "redis://example")
	t.Setenv("S3_ENDPOINT", "minio:9000")
	t.Setenv("MEDIA_PROVIDER_URL", "http://mock-provider:8090")
	t.Setenv("MEDIA_PROVIDER_TOKEN", "media-token")
	t.Setenv("KNOWLEDGE_PROVIDER_URL", "http://mock-provider:8090")
	t.Setenv("KNOWLEDGE_PROVIDER_TOKEN", "knowledge-token")
	t.Setenv("AGENT_PROVIDER_URL", "http://mock-provider:8090")
	t.Setenv("AGENT_PROVIDER_TOKEN", "agent-token")
	t.Setenv("PROVIDER_ALLOWED_HOSTS", " mock-provider:8090, backup-provider:8090 ")
	t.Setenv("PROVIDER_CALLBACK_BASE_URL", "http://api:8080")
	t.Setenv("PROVIDER_CALLBACK_SECRET", "local-callback-secret")
	t.Setenv("PROVIDER_POLL_INTERVAL", "2s")
	t.Setenv("AUTH_MODE", "dev")
	t.Setenv("SESSION_SECRET", "local-session-secret")
	t.Setenv("SESSION_TTL", "12h")
}

func TestLoadRequiresWechatCredentialsInWechatMode(t *testing.T) {
	setRequiredBaseEnvironment(t)
	t.Setenv("AUTH_MODE", "wechat")
	t.Setenv("WECHAT_APP_ID", "")
	t.Setenv("WECHAT_APP_SECRET", "")
	t.Setenv("STAFF_OPENID_ALLOWLIST", "collector-openid")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want WeChat credential error")
	}
}

func TestLoadAllowsWechatAutoEnrollWithoutAllowlist(t *testing.T) {
	setRequiredBaseEnvironment(t)
	t.Setenv("AUTH_MODE", "wechat")
	t.Setenv("WECHAT_APP_ID", "wx-test-app")
	t.Setenv("WECHAT_APP_SECRET", "test-secret")
	t.Setenv("STAFF_OPENID_ALLOWLIST", "")
	t.Setenv("STAFF_AUTO_ENROLL", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.StaffAutoEnroll {
		t.Fatal("StaffAutoEnroll = false, want true")
	}
}

func TestLoadParsesStaffAuthentication(t *testing.T) {
	setRequiredBaseEnvironment(t)
	t.Setenv("STAFF_OPENID_ALLOWLIST", " collector-1,collector-2 ")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AuthMode != "dev" || cfg.SessionTTL != 12*time.Hour || len(cfg.StaffOpenIDAllowlist) != 2 {
		t.Fatalf("auth config = %#v", cfg)
	}
}

func TestLoadRequiresServiceEndpoints(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("REDIS_URL", "")
	t.Setenv("S3_ENDPOINT", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want required-variable error")
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	setRequiredBaseEnvironment(t)
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
	if cfg.ChromiumURL != "http://localhost:9222" {
		t.Fatalf("ChromiumURL = %q", cfg.ChromiumURL)
	}
}

func TestLoadParsesSecurePublicObjectEndpoint(t *testing.T) {
	setRequiredBaseEnvironment(t)
	t.Setenv("S3_PUBLIC_ENDPOINT", "files.example.com")
	t.Setenv("S3_PUBLIC_SECURE", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.S3PublicSecure {
		t.Fatal("S3PublicSecure = false, want true")
	}
}

func TestLoadRequiresProviderConfiguration(t *testing.T) {
	setRequiredBaseEnvironment(t)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AgentProviderURL != "http://mock-provider:8090" || cfg.ProviderPollInterval.String() != "2s" {
		t.Fatalf("cfg = %#v", cfg)
	}
	if len(cfg.ProviderAllowedHosts) != 2 || cfg.ProviderAllowedHosts[1] != "backup-provider:8090" {
		t.Fatalf("allowed hosts = %#v", cfg.ProviderAllowedHosts)
	}
}

func TestLoadRejectsEmptyProviderCallbackSecret(t *testing.T) {
	setRequiredBaseEnvironment(t)
	t.Setenv("PROVIDER_CALLBACK_SECRET", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want callback-secret error")
	}
}
