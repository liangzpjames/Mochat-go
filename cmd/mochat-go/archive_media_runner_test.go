package main

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"
)

func TestDurableArchiveMediaBatchContinuesAfterSanitizedCleanupError(t *testing.T) {
	const secret = "SECRET-PATH-OR-LOCATOR"
	runner := &fakeDurableArchiveMediaBatchRunner{cleanupErr: errors.New(secret)}
	var output bytes.Buffer
	logger := log.New(&output, "", 0)
	if err := runDurableArchiveMediaBatch(context.Background(), runner, 1, logger); err != nil {
		t.Fatal(err)
	}
	if runner.runCalls != 1 {
		t.Fatalf("RunOne calls=%d", runner.runCalls)
	}
	if strings.Contains(output.String(), secret) || !strings.Contains(output.String(), "cleanup failed") {
		t.Fatalf("unsafe or missing cleanup log=%q", output.String())
	}
}

type fakeDurableArchiveMediaBatchRunner struct {
	cleanupErr error
	runCalls   int
}

func (r *fakeDurableArchiveMediaBatchRunner) CleanupStaleAttempts(context.Context) (int, error) {
	return 0, r.cleanupErr
}

func (r *fakeDurableArchiveMediaBatchRunner) RunOne(context.Context) (bool, error) {
	r.runCalls++
	return false, nil
}
