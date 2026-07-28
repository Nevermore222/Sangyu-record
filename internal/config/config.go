package config

import (
	"errors"
	"os"
)

type Config struct {
	HTTPAddress string
	DatabaseURL string
	RedisURL    string
	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	S3Bucket    string
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddress: envOr("HTTP_ADDRESS", ":8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    os.Getenv("REDIS_URL"),
		S3Endpoint:  os.Getenv("S3_ENDPOINT"),
		S3AccessKey: os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey: os.Getenv("S3_SECRET_KEY"),
		S3Bucket:    envOr("S3_BUCKET", "sangyu-private"),
	}
	if cfg.DatabaseURL == "" || cfg.RedisURL == "" || cfg.S3Endpoint == "" {
		return Config{}, errors.New("DATABASE_URL, REDIS_URL, and S3_ENDPOINT are required")
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
