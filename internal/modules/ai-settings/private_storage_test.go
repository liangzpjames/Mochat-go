package aisettings

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/modules/ai-settings/ports"
)

func TestPrivateStorageUsesNonStaticSiblingAndRejectsUnsafeKeys(t *testing.T) {
	staticRoot := filepath.Join(t.TempDir(), "upload", "static")
	storage, err := NewPrivateStorage(staticRoot)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(filepath.ToSlash(storage.Root()), "/static/") || filepath.Base(storage.Root()) != "ai-knowledge-private" {
		t.Fatalf("private root = %q", storage.Root())
	}
	for _, key := range []string{"../outside.txt", "/absolute.txt", `..\outside.txt`, ""} {
		if _, err := storage.Resolve(key); err != ports.ErrUnsafeObjectKey {
			t.Fatalf("Resolve(%q) error = %v", key, err)
		}
	}
}

func TestPrivateStorageStagesCommitsQuarantinesAndRestores(t *testing.T) {
	storage, err := NewPrivateStorage(filepath.Join(t.TempDir(), "upload", "static"))
	if err != nil {
		t.Fatal(err)
	}
	staged, size, checksum, err := storage.Stage(bytes.NewBufferString("private knowledge"))
	if err != nil {
		t.Fatal(err)
	}
	if size != int64(len("private knowledge")) || len(checksum) != 64 {
		t.Fatalf("size=%d checksum=%q", size, checksum)
	}
	key := "tenant-1/corp-2/kb-3/doc-4.txt"
	if _, err := storage.Commit(staged, key); err != nil {
		t.Fatal(err)
	}
	path, _ := storage.Resolve(key)
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	quarantine, err := storage.Quarantine(key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("committed path still exists: %v", err)
	}
	if err := storage.Restore(quarantine, key); err != nil {
		t.Fatal(err)
	}
	if err := storage.Delete(key); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("deleted path still exists: %v", err)
	}
}

func TestPrivateStorageRejectsFileOverLimit(t *testing.T) {
	storage, err := NewPrivateStorage(filepath.Join(t.TempDir(), "static"))
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, err = storage.Stage(bytes.NewReader(make([]byte, MaxUploadBytes+1)))
	if err != ports.ErrDocumentTooLarge {
		t.Fatalf("error = %v", err)
	}
}
