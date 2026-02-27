package repository

import (
	"fmt"
	"os"
	"path/filepath"
)

type FSStorage struct{ baseDir string }

func NewFSStorage(baseDir string) *FSStorage { return &FSStorage{baseDir: baseDir} }

func (s *FSStorage) Write(storagePath string, data []byte) error {
	full := filepath.Join(s.baseDir, storagePath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("creating dirs: %w", err)
	}
	return os.WriteFile(full, data, 0o644)
}

func (s *FSStorage) Read(storagePath string) ([]byte, error) {
	return os.ReadFile(filepath.Join(s.baseDir, storagePath))
}
