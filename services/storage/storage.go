package storage

import (
	"fmt"
	"io"
	"strings"

	"github.com/zaidejaz/saaf-islamabad-backend/config"
)

const (
	DriverLocal = "local"
	DriverR2    = "r2"
)

// Uploader persists binary image data and returns a public URL.
type Uploader interface {
	Driver() string
	Save(key string, body io.Reader, contentType string, size int64) (publicURL string, err error)
}

// New builds the configured image uploader.
func New(cfg *config.Config) (Uploader, error) {
	driver := strings.ToLower(strings.TrimSpace(cfg.StorageDriver))
	if driver == "" {
		driver = DriverLocal
	}

	switch driver {
	case DriverLocal:
		return NewLocal(cfg.LocalUploadsDir), nil
	case DriverR2:
		return NewR2(R2Config{
			AccountID:       cfg.R2AccountID,
			AccessKeyID:     cfg.R2AccessKeyID,
			SecretAccessKey: cfg.R2SecretAccessKey,
			Bucket:          cfg.R2Bucket,
			PublicBaseURL:   cfg.R2PublicBaseURL,
		})
	default:
		return nil, fmt.Errorf("unsupported STORAGE_DRIVER %q (use local or r2)", driver)
	}
}
