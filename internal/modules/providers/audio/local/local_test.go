package local

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/modules/providers"
)

func TestStoragePutOpenDelete(t *testing.T) {
	root := t.TempDir()
	storage, err := New(Config{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	content := []byte("RIFF....WAVE")
	if err := storage.Put(ctx, "audio/1/2026/08/sample.wav", bytes.NewReader(content), providers.PutOptions{ContentType: "audio/wav", SizeBytes: int64(len(content))}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	reader, size, err := storage.Open(ctx, "audio/1/2026/08/sample.wav")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if size != int64(len(content)) {
		t.Fatalf("size = %d, want %d", size, len(content))
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	buffered, err := os.ReadFile(filepath.Join(root, "audio", "1", "2026", "08", "sample.wav"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buffered, content) {
		t.Fatal("stored content mismatch")
	}
	if err := storage.Delete(ctx, "audio/1/2026/08/sample.wav"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, _, err := storage.Open(ctx, "audio/1/2026/08/sample.wav"); !os.IsNotExist(err) {
		t.Fatalf("Open after delete err = %v, want not exist", err)
	}
}

func TestStorageRejectsPathTraversal(t *testing.T) {
	storage, err := New(Config{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"../escape.wav", "audio/../../escape.wav", "/absolute/path.wav", "", ".."} {
		if err := storage.Put(context.Background(), key, strings.NewReader("x"), providers.PutOptions{}); err == nil {
			t.Fatalf("Put(%q) error = nil, want rejection", key)
		}
	}
}

func TestStorageStatusReady(t *testing.T) {
	storage, err := New(Config{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if status := storage.Status(); status.State != providers.StateReady {
		t.Fatalf("status = %#v, want ready", status)
	}
}

func TestStorageStatusUsesFilesystemEvidence(t *testing.T) {
	parent := t.TempDir()
	missingRoot := filepath.Join(parent, "not-created")
	missing, err := New(Config{Root: missingRoot})
	if err != nil {
		t.Fatal(err)
	}
	if status := missing.Status(); status.State != providers.StateLimited || status.Code != "audio_storage.root_missing" {
		t.Fatalf("missing root status = %#v, want limited/root_missing", status)
	}
	if _, err := os.Stat(missingRoot); !os.IsNotExist(err) {
		t.Fatalf("Status created missing root: stat err=%v", err)
	}

	fileRoot := filepath.Join(parent, "root-file")
	if err := os.WriteFile(fileRoot, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	fileStorage, err := New(Config{Root: fileRoot})
	if err != nil {
		t.Fatal(err)
	}
	if status := fileStorage.Status(); status.State != providers.StateUnavailable || status.Code != "audio_storage.root_not_directory" {
		t.Fatalf("file root status = %#v, want unavailable/root_not_directory", status)
	}

	parentFile := filepath.Join(parent, "parent-file")
	if err := os.WriteFile(parentFile, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	childStorage, err := New(Config{Root: filepath.Join(parentFile, "child")})
	if err != nil {
		t.Fatal(err)
	}
	if status := childStorage.Status(); status.State != providers.StateUnavailable {
		t.Fatalf("file parent status = %#v, want unavailable", status)
	}
}

func TestStorageStatusRejectsNonWritableDirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "read-only")
	if err := os.Mkdir(root, 0o555); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o555); err != nil {
		t.Fatal(err)
	}
	storage, err := New(Config{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if status := storage.Status(); status.State != providers.StateUnavailable || status.Code != "audio_storage.root_not_writable" {
		t.Fatalf("read-only root status = %#v, want unavailable/root_not_writable", status)
	}

	parent := filepath.Join(t.TempDir(), "read-only-parent")
	if err := os.Mkdir(parent, 0o555); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(parent, 0o555); err != nil {
		t.Fatal(err)
	}
	missingChild, err := New(Config{Root: filepath.Join(parent, "missing-child")})
	if err != nil {
		t.Fatal(err)
	}
	if status := missingChild.Status(); status.State != providers.StateUnavailable || status.Code != "audio_storage.parent_not_writable" {
		t.Fatalf("read-only parent status = %#v, want unavailable/parent_not_writable", status)
	}
}
