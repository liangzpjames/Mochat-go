package local

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"jiyi/mochat-go/internal/modules/providers"
)

// Config configures the local file system audio provider.
type Config struct {
	Root string
}

// Storage implements providers.AudioProvider on the local file system.
type Storage struct {
	root string
}

var _ providers.AudioProvider = (*Storage)(nil)

func New(config Config) (*Storage, error) {
	root := strings.TrimSpace(config.Root)
	if root == "" {
		root = "storage/upload/static"
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve audio storage root: %w", err)
	}
	return &Storage{root: absolute}, nil
}

func (s *Storage) Root() string {
	return s.root
}

func (s *Storage) Status() providers.Status {
	base := providers.Status{Kind: "audio_storage", Source: providers.SourceLocal, Capabilities: []string{"audio_object_storage"}}
	if s == nil || strings.TrimSpace(s.root) == "" {
		base.State = providers.StateUnavailable
		base.Code = "audio_storage.root_unavailable"
		return base
	}
	info, err := os.Stat(s.root)
	if err == nil {
		if !info.IsDir() {
			base.State = providers.StateUnavailable
			base.Code = "audio_storage.root_not_directory"
			return base
		}
		if !directoryWritable(info) {
			base.State = providers.StateUnavailable
			base.Code = "audio_storage.root_not_writable"
			return base
		}
		base.State = providers.StateReady
		base.Code = "audio_storage.ready"
		return base
	}
	if !os.IsNotExist(err) {
		base.State = providers.StateUnavailable
		base.Code = "audio_storage.root_stat_failed"
		return base
	}
	_, parentInfo, parentErr := nearestExistingDirectory(filepath.Dir(s.root))
	if parentErr != nil || parentInfo == nil || !parentInfo.IsDir() || !directoryWritable(parentInfo) {
		base.State = providers.StateUnavailable
		base.Code = "audio_storage.parent_not_writable"
		return base
	}
	base.State = providers.StateLimited
	base.Code = "audio_storage.root_missing"
	base.Action = "create the configured local audio storage directory before uploads"
	return base
}

func directoryWritable(info os.FileInfo) bool {
	return info.Mode().Perm()&0o222 != 0
}

func nearestExistingDirectory(path string) (string, os.FileInfo, error) {
	for {
		info, err := os.Stat(path)
		if err == nil {
			return path, info, nil
		}
		if !os.IsNotExist(err) {
			return "", nil, err
		}
		next := filepath.Dir(path)
		if next == path {
			return "", nil, os.ErrNotExist
		}
		path = next
	}
}

func (s *Storage) Put(ctx context.Context, key string, reader io.Reader, opts providers.PutOptions) error {
	if ctx == nil {
		return errors.New("context is required")
	}
	if reader == nil {
		return errors.New("reader is required")
	}
	target, err := s.resolve(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create audio directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(target), ".upload-*")
	if err != nil {
		return fmt.Errorf("create temporary audio file: %w", err)
	}
	temporaryName := temporary.Name()
	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(temporary, hash), reader)
	closeErr := temporary.Close()
	if err != nil {
		_ = os.Remove(temporaryName)
		return fmt.Errorf("write audio file: %w", err)
	}
	if closeErr != nil {
		_ = os.Remove(temporaryName)
		return fmt.Errorf("close audio file: %w", closeErr)
	}
	if opts.SizeBytes > 0 && written > opts.SizeBytes {
		_ = os.Remove(temporaryName)
		return fmt.Errorf("audio file exceeds declared size %d", opts.SizeBytes)
	}
	if err := os.Rename(temporaryName, target); err != nil {
		_ = os.Remove(temporaryName)
		return fmt.Errorf("commit audio file: %w", err)
	}
	_ = hash
	return nil
}

func (s *Storage) Open(_ context.Context, key string) (io.ReadCloser, int64, error) {
	target, err := s.resolve(key)
	if err != nil {
		return nil, 0, err
	}
	file, err := os.Open(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, os.ErrNotExist
		}
		return nil, 0, fmt.Errorf("open audio file: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, 0, fmt.Errorf("stat audio file: %w", err)
	}
	return file, info.Size(), nil
}

func (s *Storage) Delete(_ context.Context, key string) error {
	target, err := s.resolve(key)
	if err != nil {
		return err
	}
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete audio file: %w", err)
	}
	return nil
}

func (s *Storage) Sha256Of(ctx context.Context, key string) (string, error) {
	reader, _, err := s.Open(ctx, key)
	if err != nil {
		return "", err
	}
	defer reader.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, reader); err != nil {
		return "", fmt.Errorf("hash audio file: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (s *Storage) resolve(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", errors.New("audio key is required")
	}
	if strings.HasPrefix(key, "/") || strings.HasPrefix(key, "\\") || strings.Contains(key, ":") {
		return "", fmt.Errorf("unsafe audio key %q", key)
	}
	cleanKey := filepath.Clean(filepath.FromSlash(key))
	if cleanKey == "." || filepath.IsAbs(cleanKey) || strings.HasPrefix(cleanKey, ".."+string(filepath.Separator)) || cleanKey == ".." {
		return "", fmt.Errorf("unsafe audio key %q", key)
	}
	target := filepath.Join(s.root, cleanKey)
	relative, err := filepath.Rel(s.root, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe audio path %q", key)
	}
	return target, nil
}
