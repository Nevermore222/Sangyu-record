package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	HTTPAddress             string
	DatabaseURL             string
	RedisURL                string
	S3Endpoint              string
	S3PublicEndpoint        string
	S3AccessKey             string
	S3SecretKey             string
	S3Bucket                string
	S3Region                string
	ChromiumURL             string
	MediaProviderURL        string
	MediaProviderToken      string
	KnowledgeProviderURL    string
	KnowledgeProviderToken  string
	AgentProviderURL        string
	AgentProviderToken      string
	ProviderAllowedHosts    []string
	ProviderCallbackBaseURL string
	ProviderCallbackSecret  string
	ProviderPollInterval    time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddress:             envOr("HTTP_ADDRESS", ":8080"),
		DatabaseURL:             os.Getenv("DATABASE_URL"),
		RedisURL:                os.Getenv("REDIS_URL"),
		S3Endpoint:              os.Getenv("S3_ENDPOINT"),
		S3AccessKey:             os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey:             os.Getenv("S3_SECRET_KEY"),
		S3Bucket:                envOr("S3_BUCKET", "sangyu-private"),
		S3Region:                envOr("S3_REGION", "us-east-1"),
		ChromiumURL:             envOr("CHROMIUM_URL", "http://localhost:9222"),
		MediaProviderURL:        os.Getenv("MEDIA_PROVIDER_URL"),
		MediaProviderToken:      os.Getenv("MEDIA_PROVIDER_TOKEN"),
		KnowledgeProviderURL:    os.Getenv("KNOWLEDGE_PROVIDER_URL"),
		KnowledgeProviderToken:  os.Getenv("KNOWLEDGE_PROVIDER_TOKEN"),
		AgentProviderURL:        os.Getenv("AGENT_PROVIDER_URL"),
		AgentProviderToken:      os.Getenv("AGENT_PROVIDER_TOKEN"),
		ProviderCallbackBaseURL: os.Getenv("PROVIDER_CALLBACK_BASE_URL"),
		ProviderCallbackSecret:  os.Getenv("PROVIDER_CALLBACK_SECRET"),
	}
	cfg.S3PublicEndpoint = envOr("S3_PUBLIC_ENDPOINT", cfg.S3Endpoint)
	if cfg.DatabaseURL == "" || cfg.RedisURL == "" || cfg.S3Endpoint == "" {
		return Config{}, errors.New("DATABASE_URL, REDIS_URL, and S3_ENDPOINT are required")
	}
	if cfg.MediaProviderURL == "" || cfg.MediaProviderToken == "" ||
		cfg.KnowledgeProviderURL == "" || cfg.KnowledgeProviderToken == "" ||
		cfg.AgentProviderURL == "" || cfg.AgentProviderToken == "" ||
		cfg.ProviderCallbackBaseURL == "" || cfg.ProviderCallbackSecret == "" {
		return Config{}, errors.New("Provider URLs, tokens, callback base URL, and callback secret are required")
	}
	for _, host := range strings.Split(os.Getenv("PROVIDER_ALLOWED_HOSTS"), ",") {
		if host = strings.TrimSpace(host); host != "" {
			cfg.ProviderAllowedHosts = append(cfg.ProviderAllowedHosts, host)
		}
	}
	if len(cfg.ProviderAllowedHosts) == 0 {
		return Config{}, errors.New("PROVIDER_ALLOWED_HOSTS is required")
	}
	pollInterval, err := time.ParseDuration(os.Getenv("PROVIDER_POLL_INTERVAL"))
	if err != nil || pollInterval <= 0 {
		return Config{}, fmt.Errorf("PROVIDER_POLL_INTERVAL must be a positive duration")
	}
	cfg.ProviderPollInterval = pollInterval
	return cfg, nil
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
