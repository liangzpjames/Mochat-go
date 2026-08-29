package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
)

func TestPostWorkerBuildFailureCancelsRootAndWaitsForWorkers(t *testing.T) {
	rootCtx, cancelRoot := context.WithCancel(context.Background())
	waited := make(chan struct{})
	wait := func(context.Context) error {
		if rootCtx.Err() == nil {
			t.Fatal("worker wait started before root cancellation")
		}
		close(waited)
		return nil
	}
	buildErr := errors.New("build server failed")
	run := func() (err error) {
		defer backgroundTaskCleanup(cancelRoot, wait, time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))()
		return buildErr
	}
	if err := run(); !errors.Is(err, buildErr) {
		t.Fatalf("run error = %v", err)
	}
	select {
	case <-waited:
	default:
		t.Fatal("worker wait was skipped on post-start build error")
	}
}

func TestNoFatalExitAfterBackgroundWorkersStart(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	start := strings.Index(text, "defer backgroundTaskCleanup")
	end := strings.Index(text, "httpServices.Run(shutdownCtx)")
	if start < 0 || end <= start {
		t.Fatalf("could not locate worker-start/serve range")
	}
	postStart := text[start:end]
	if strings.Contains(postStart, "fatal(") || strings.Contains(postStart, "fatalf(") {
		t.Fatalf("post-worker startup still contains fatal/os.Exit path:\n%s", postStart)
	}
}
