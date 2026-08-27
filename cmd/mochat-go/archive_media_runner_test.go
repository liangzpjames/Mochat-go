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

func TestDurableArchiveMediaBatchContinuesAfterOneObjectFailsAndSanitizesResult(t *testing.T) {
	const secret = "SECRET-UPSTREAM-MEDIA-LOCATOR"
	runner := &fakeDurableArchiveMediaBatchRunner{results: []fakeMediaRunResult{
		{worked: true, err: errors.New(secret)},
		{worked: true},
		{worked: false},
	}}
	err := runDurableArchiveMediaBatch(context.Background(), runner, 10, log.New(&bytes.Buffer{}, "", 0))
	if err == nil || strings.Contains(err.Error(), secret) || !strings.Contains(err.Error(), "failed items") {
		t.Fatalf("batch error=%v", err)
	}
	if runner.runCalls != 3 {
		t.Fatalf("RunOne calls=%d, want 3", runner.runCalls)
	}
}

type fakeMediaRunResult struct {
	worked bool
	err    error
}

type fakeDurableArchiveMediaBatchRunner struct {
	cleanupErr error
	runCalls   int
	results    []fakeMediaRunResult
}

func (r *fakeDurableArchiveMediaBatchRunner) CleanupStaleAttempts(context.Context) (int, error) {
	return 0, r.cleanupErr
}

func (r *fakeDurableArchiveMediaBatchRunner) RunOne(context.Context) (bool, error) {
	r.runCalls++
	if len(r.results) > 0 {
		result := r.results[0]
		r.results = r.results[1:]
		return result.worked, result.err
	}
	return false, nil
}
