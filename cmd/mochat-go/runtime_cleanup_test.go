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

	"jiyi/mochat-go/internal/store"
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

func TestPostWorkerMySQLInitializationFailureCancelsRootAndWaits(t *testing.T) {
	for _, initErr := range []error{
		errors.New("open mysql: DSN is empty"),
		errors.New("open mysql: connection refused"),
	} {
		t.Run(initErr.Error(), func(t *testing.T) {
			rootCtx, cancelRoot := context.WithCancel(context.Background())
			waited := false
			var target *store.MySQLStore
			run := func() (runtimeErr error) {
				defer backgroundTaskCleanup(cancelRoot, func(context.Context) error {
					if rootCtx.Err() == nil {
						t.Fatal("worker wait started before root cancellation")
					}
					waited = true
					return nil
				}, time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))()
				if err := initializeRuntimeMySQLStore(&target, func() (*store.MySQLStore, error) {
					return nil, initErr
				}); err != nil {
					runtimeErr = err
					return
				}
				return nil
			}
			if err := run(); !errors.Is(err, initErr) {
				t.Fatalf("runtime error = %v, want %v", err, initErr)
			}
			if !waited {
				t.Fatal("worker cleanup was skipped on MySQL initialization failure")
			}
			if target != nil {
				t.Fatal("failed MySQL store was published")
			}
		})
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
	if strings.Contains(postStart, "fatal(") || strings.Contains(postStart, "fatalf(") || strings.Contains(postStart, "os.Exit(") {
		t.Fatalf("post-worker startup still contains fatal/os.Exit path:\n%s", postStart)
	}
	if strings.Contains(postStart, "getMySQLStore(") {
		t.Fatalf("post-worker startup still calls the fatal compatibility getter:\n%s", postStart)
	}
	initialize := strings.Index(postStart, "initializeRuntimeMySQLStore")
	moduleBuild := strings.Index(postStart, "newSCRMModuleRouter")
	if initialize < 0 || moduleBuild <= initialize {
		t.Fatalf("API MySQL dependency is not initialized through the error-returning path before post-start module construction")
	}
}
