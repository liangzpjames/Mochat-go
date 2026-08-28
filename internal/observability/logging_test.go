package observability

import (
	"bytes"
	"errors"
	"log"
	"log/slog"
	"strings"
	"testing"
)

func TestNewEmitsConfiguredJSONAndRedactsSensitiveValues(t *testing.T) {
	var output bytes.Buffer
	logger, err := New(Config{Level: "debug", Format: "json", Output: &output, Service: "test"})
	if err != nil {
		t.Fatal(err)
	}
	logger.Info("provider token=plain-token", "event", "redaction_check", "api_key", "plain-key", "configured", true)
	line := output.String()
	for _, forbidden := range []string{"plain-token", "plain-key"} {
		if strings.Contains(line, forbidden) {
			t.Fatalf("log leaked %q: %s", forbidden, line)
		}
	}
	for _, required := range []string{`"level":"INFO"`, `"event":"redaction_check"`, `"service":"test"`, `[REDACTED]`} {
		if !strings.Contains(line, required) {
			t.Fatalf("log missing %q: %s", required, line)
		}
	}
}

func TestNewHonorsLevelAndValidatesFormat(t *testing.T) {
	var output bytes.Buffer
	logger, err := New(Config{Level: "warn", Format: "text", Output: &output, Service: "test"})
	if err != nil {
		t.Fatal(err)
	}
	logger.Info("hidden info")
	logger.Warn("visible warning")
	if strings.Contains(output.String(), "hidden info") || !strings.Contains(output.String(), "visible warning") {
		t.Fatalf("level filtering failed: %s", output.String())
	}
	if _, err := New(Config{Level: "verbose", Format: "json", Output: &output}); err == nil {
		t.Fatal("expected invalid level error")
	}
	if _, err := New(Config{Level: "info", Format: "xml", Output: &output}); err == nil {
		t.Fatal("expected invalid format error")
	}
}

func TestRedactionHandlesErrorsGroupsAndConfiguredBooleans(t *testing.T) {
	var output bytes.Buffer
	logger, err := New(Config{Level: "info", Format: "json", Output: &output, Service: "test"})
	if err != nil {
		t.Fatal(err)
	}
	logger.Error("request failed Authorization: Bearer raw-bearer; upstream said Bearer standalone-bearer", "error", errors.New("password=hunter2"), "auth", slog.GroupValue(
		slog.String("private_key", "-----BEGIN PRIVATE KEY-----raw-----END PRIVATE KEY-----"),
		slog.Bool("token_configured", true),
	))
	line := output.String()
	for _, forbidden := range []string{"raw-bearer", "standalone-bearer", "hunter2", "BEGIN PRIVATE KEY", "raw-----"} {
		if strings.Contains(line, forbidden) {
			t.Fatalf("log leaked %q: %s", forbidden, line)
		}
	}
	if !strings.Contains(line, `"token_configured":true`) {
		t.Fatalf("configured boolean should remain observable: %s", line)
	}
}

func TestSanitizeTextRedactsURLUserInfoAndJWT(t *testing.T) {
	input := "mysql://db-user:db-password@database:3306/mochat jwt=eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ1c2VyLTEifQ.signature123"
	redacted := SanitizeText(input)
	for _, forbidden := range []string{"db-user", "db-password", "eyJhbGciOiJIUzI1NiJ9", "eyJzdWIiOiJ1c2VyLTEifQ", "signature123"} {
		if strings.Contains(redacted, forbidden) {
			t.Fatalf("sanitized text leaked %q: %s", forbidden, redacted)
		}
	}
	if strings.Count(redacted, redactedValue) < 2 {
		t.Fatalf("sanitized text did not redact URL and JWT: %s", redacted)
	}
}

func TestBridgeStandardLogRecognizesExplicitLevelPrefix(t *testing.T) {
	originalWriter, originalFlags, originalPrefix := log.Writer(), log.Flags(), log.Prefix()
	t.Cleanup(func() {
		log.SetOutput(originalWriter)
		log.SetFlags(originalFlags)
		log.SetPrefix(originalPrefix)
	})

	var output bytes.Buffer
	logger, err := New(Config{Level: "debug", Format: "json", Output: &output, Service: "test"})
	if err != nil {
		t.Fatal(err)
	}
	BridgeStandardLog(logger)
	log.Print("level=WARN event=queue_retry queue item will retry token=hidden")
	line := output.String()
	if !strings.Contains(line, `"level":"WARN"`) || !strings.Contains(line, `"event":"queue_retry"`) {
		t.Fatalf("bridged log = %s", line)
	}
	if strings.Contains(line, "hidden") {
		t.Fatalf("bridged log leaked secret: %s", line)
	}
}

func TestConfigureFromEnvUsesDocumentedVariables(t *testing.T) {
	t.Setenv("MOCHAT_LOG_LEVEL", "error")
	t.Setenv("MOCHAT_LOG_FORMAT", "text")
	t.Setenv("MOCHAT_LOG_SOURCE", "1")
	logger, err := ConfigureFromEnv("migration-test")
	if err != nil {
		t.Fatal(err)
	}
	if logger == nil {
		t.Fatal("logger is nil")
	}
	t.Setenv("MOCHAT_LOG_SOURCE", "sometimes")
	if _, err := ConfigureFromEnv("migration-test"); err == nil {
		t.Fatal("expected invalid source flag error")
	}
}
