package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRunKeygenWritesConfiguration(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "secrets")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"keygen", "--output", dir, "--public-url", "http://139.196.34.133:19090"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "wecom-fill.txt")); err != nil {
		t.Fatal(err)
	}
	if stdout.String() == "" {
		t.Fatal("keygen did not report its output directory")
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := run([]string{"unknown"}, &stdout, &stderr); code == 0 {
		t.Fatal("unknown command succeeded")
	}
}
