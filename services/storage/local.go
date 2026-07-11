package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LocalStorage saves files under a directory and returns /uploads/ URLs.
type LocalStorage struct {
	dir string
}

func NewLocal(dir string) *LocalStorage {
	if strings.TrimSpace(dir) == "" {
		dir = "./uploads"
	}
	return &LocalStorage{dir: dir}
}

func (s *LocalStorage) Driver() string { return DriverLocal }

func (s *LocalStorage) Save(key string, body io.Reader, _ string, _ int64) (string, error) {
	key = strings.TrimPrefix(strings.ReplaceAll(key, "\\", "/"), "/")
	if key == "" || strings.Contains(key, "..") {
		return "", fmt.Errorf("invalid storage key")
	}

	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return "", fmt.Errorf("prepare upload directory: %w", err)
	}

	dst := filepath.Join(s.dir, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", fmt.Errorf("prepare upload path: %w", err)
	}

	f, err := os.Create(dst)
	if err != nil {
		return "", fmt.Errorf("create upload file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, body); err != nil {
		return "", fmt.Errorf("write upload file: %w", err)
	}

	return "/uploads/" + key, nil
}
