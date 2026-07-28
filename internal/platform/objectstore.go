package platform

import (
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type ObjectStoreConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Secure    bool
	Region    string
}

func NewObjectStore(cfg ObjectStoreConfig) (*minio.Client, error) {
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	return minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.Secure,
		Region: cfg.Region,
	})
}
