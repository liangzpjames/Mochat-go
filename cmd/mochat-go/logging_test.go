package main

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/observability"
)

func TestLogRuntimeListeningIdentifiesEveryListener(t *testing.T) {
	var output bytes.Buffer
	logger, err := observability.New(observability.Config{
		Level:   "info",
		Format:  "json",
		Output:  &output,
		Service: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	original := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(original) })

	logRuntimeListening("sidebar", "127.0.0.1:8081", false)
	logRuntimeListening("operation", "127.0.0.1:8082", false)
	logRuntimeListening("main", "127.0.0.1:8080", true)

	logs := output.String()
	if got := strings.Count(logs, `"event":"runtime_listening"`); got != 3 {
		t.Fatalf("runtime listening log count = %d, want 3: %s", got, logs)
	}
	for _, required := range []string{
		`"listener":"sidebar"`,
		`"listener":"operation"`,
		`"listener":"main"`,
		`"listen_addr":"127.0.0.1:8080"`,
		`"php_fallback_enabled":true`,
	} {
		if !strings.Contains(logs, required) {
			t.Fatalf("runtime listening logs missing %s: %s", required, logs)
		}
	}
}
