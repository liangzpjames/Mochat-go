package aisettings

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"

	"jiyi/mochat-go/internal/modules/ai-settings/ports"
)

type PrivateStorage struct {
	root string
}

func NewPrivateStorage(fileStorageRoot string) (*PrivateStorage, error) {
	fileStorageRoot = strings.TrimSpace(fileStorageRoot)
	if fileStorageRoot == "" {
		return nil, ports.ErrUnsafeObjectKey
	}
	staticRoot, err := filepath.Abs(fileStorageRoot)
	if err != nil {
		return nil, err
	}
	root := filepath.Join(filepath.Dir(staticRoot), "ai-knowledge-private")
	if err := os.MkdirAll(filepath.Join(root, ".tmp"), 0o700); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(root, ".trash"), 0o700); err != nil {
		return nil, err
	}
	return &PrivateStorage{root: root}, nil
}

func (s *PrivateStorage) Root() string {
	if s == nil {
		return ""
	}
	return s.root
}

func (s *PrivateStorage) Resolve(objectKey string) (string, error) {
	if s == nil || s.root == "" || !safeObjectKey(objectKey) {
		return "", ports.ErrUnsafeObjectKey
	}
	target := filepath.Join(s.root, filepath.FromSlash(objectKey))
	if !withinRoot(s.root, target) {
		return "", ports.ErrUnsafeObjectKey
	}
	return target, nil
}

func (s *PrivateStorage) Stage(reader io.Reader) (string, int64, string, error) {
	if s == nil || s.root == "" || reader == nil {
		return "", 0, "", ports.ErrDocumentUnreadable
	}
	file, err := os.CreateTemp(filepath.Join(s.root, ".tmp"), "upload-*")
	if err != nil {
		return "", 0, "", err
	}
	path := file.Name()
	hash := sha256.New()
	size, copyErr := io.Copy(io.MultiWriter(file, hash), io.LimitReader(reader, MaxUploadBytes+1))
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(path)
		return "", 0, "", ports.ErrDocumentUnreadable
	}
	if size > MaxUploadBytes {
		_ = os.Remove(path)
		return "", 0, "", ports.ErrDocumentTooLarge
	}
	return path, size, hex.EncodeToString(hash.Sum(nil)), nil
}

func (s *PrivateStorage) Commit(stagedPath, objectKey string) (string, error) {
	target, err := s.Resolve(objectKey)
	if err != nil {
		return "", err
	}
	if !withinRoot(filepath.Join(s.root, ".tmp"), stagedPath) {
		return "", ports.ErrUnsafeObjectKey
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return "", err
	}
	if err := os.Rename(stagedPath, target); err != nil {
		return "", err
	}
	return target, nil
}

func (s *PrivateStorage) Quarantine(objectKey string) (string, error) {
	source, err := s.Resolve(objectKey)
	if err != nil {
		return "", err
	}
	file, err := os.CreateTemp(filepath.Join(s.root, ".trash"), "delete-*")
	if err != nil {
		return "", err
	}
	quarantine := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(quarantine)
		return "", err
	}
	if err := os.Remove(quarantine); err != nil {
		return "", err
	}
	if err := os.Rename(source, quarantine); err != nil {
		return "", err
	}
	return quarantine, nil
}

func (s *PrivateStorage) Restore(quarantinePath, objectKey string) error {
	if !withinRoot(filepath.Join(s.root, ".trash"), quarantinePath) {
		return ports.ErrUnsafeObjectKey
	}
	target, err := s.Resolve(objectKey)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	return os.Rename(quarantinePath, target)
}

func (s *PrivateStorage) Delete(objectKey string) error {
	target, err := s.Resolve(objectKey)
	if err != nil {
		return err
	}
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *PrivateStorage) PurgeQuarantine(path string) error {
	if !withinRoot(filepath.Join(s.root, ".trash"), path) {
		return ports.ErrUnsafeObjectKey
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *PrivateStorage) RemoveStaged(path string) {
	if s != nil && withinRoot(filepath.Join(s.root, ".tmp"), path) {
		_ = os.Remove(path)
	}
}

func safeObjectKey(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || filepath.IsAbs(value) || filepath.VolumeName(value) != "" || strings.HasPrefix(value, "/") || strings.Contains(value, "\\") {
		return false
	}
	clean := filepath.Clean(filepath.FromSlash(value))
	return clean != "." && clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator))
}

func withinRoot(root, target string) bool {
	rootAbs, rootErr := filepath.Abs(root)
	targetAbs, targetErr := filepath.Abs(target)
	if rootErr != nil || targetErr != nil {
		return false
	}
	relative, err := filepath.Rel(rootAbs, targetAbs)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
