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
