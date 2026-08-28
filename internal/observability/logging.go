package observability

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const redactedValue = "[REDACTED]"

var (
	bearerPattern              = regexp.MustCompile(`(?i)(\bbearer\s+)([^\s,;]+)`)
	sensitiveAssignmentPattern = regexp.MustCompile(`(?i)\b(password|passwd|token|secret|api[_-]?key|cookie|authorization|private[_-]?key|credential)\s*=\s*([^\s,;]+)`)
	urlUserInfoPattern         = regexp.MustCompile(`(?i)([a-z][a-z0-9+.-]*://)[^/\s:@]+:[^@/\s]+@`)
	jwtPattern                 = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}(?:\.[A-Za-z0-9_-]{8,})?\b`)
	privateKeyPattern          = regexp.MustCompile(`(?s)-----BEGIN [^-\r\n]*PRIVATE KEY-----.*?-----END [^-\r\n]*PRIVATE KEY-----`)
)

type Config struct {
	Level     string
	Format    string
	Output    io.Writer
	Service   string
	AddSource bool
}

func New(cfg Config) (*slog.Logger, error) {
	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}
	if cfg.Output == nil {
		cfg.Output = os.Stdout
	}
	options := &slog.HandlerOptions{Level: level, AddSource: cfg.AddSource}
	var handler slog.Handler
	switch strings.ToLower(strings.TrimSpace(cfg.Format)) {
	case "", "json":
		handler = slog.NewJSONHandler(cfg.Output, options)
	case "text":
		handler = slog.NewTextHandler(cfg.Output, options)
	default:
		return nil, fmt.Errorf("unsupported log format %q", cfg.Format)
	}
	handler = &redactingHandler{next: handler}
	logger := slog.New(handler)
	if service := strings.TrimSpace(cfg.Service); service != "" {
		logger = logger.With("service", service)
	}
	return logger, nil
}

func ConfigureFromEnv(service string) (*slog.Logger, error) {
	addSource := false
	if raw := strings.TrimSpace(os.Getenv("MOCHAT_LOG_SOURCE")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("MOCHAT_LOG_SOURCE must be a boolean")
		}
		addSource = parsed
	}
	return New(Config{
		Level:     envDefault("MOCHAT_LOG_LEVEL", "info"),
		Format:    envDefault("MOCHAT_LOG_FORMAT", "json"),
		Output:    os.Stdout,
		Service:   service,
		AddSource: addSource,
	})
}

func BridgeStandardLog(logger *slog.Logger) {
	if logger == nil {
		logger = slog.Default()
	}
	log.SetFlags(0)
	log.SetPrefix("")
	log.SetOutput(&legacyWriter{logger: logger})
}

func SanitizeText(value string) string {
	if value == "" {
		return ""
	}
	value = privateKeyPattern.ReplaceAllString(value, redactedValue)
	value = bearerPattern.ReplaceAllString(value, `${1}`+redactedValue)
	value = sensitiveAssignmentPattern.ReplaceAllString(value, `${1}=`+redactedValue)
	value = urlUserInfoPattern.ReplaceAllString(value, `${1}`+redactedValue+`@`)
	value = jwtPattern.ReplaceAllString(value, redactedValue)
	return value
}

func parseLevel(value string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unsupported log level %q", value)
	}
}

func envDefault(name string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

type redactingHandler struct {
	next slog.Handler
}

func (h *redactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *redactingHandler) Handle(ctx context.Context, record slog.Record) error {
	redacted := slog.NewRecord(record.Time, record.Level, SanitizeText(record.Message), record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		redacted.AddAttrs(redactAttr(attr))
		return true
	})
	return h.next.Handle(ctx, redacted)
}

func (h *redactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, 0, len(attrs))
	for _, attr := range attrs {
		redacted = append(redacted, redactAttr(attr))
	}
	return &redactingHandler{next: h.next.WithAttrs(redacted)}
}

func (h *redactingHandler) WithGroup(name string) slog.Handler {
	return &redactingHandler{next: h.next.WithGroup(name)}
}

func redactAttr(attr slog.Attr) slog.Attr {
	attr.Value = attr.Value.Resolve()
	if attr.Value.Kind() == slog.KindGroup {
		members := attr.Value.Group()
		redacted := make([]slog.Attr, 0, len(members))
		for _, member := range members {
			redacted = append(redacted, redactAttr(member))
		}
		attr.Value = slog.GroupValue(redacted...)
		return attr
	}
	if isSensitiveKey(attr.Key) && attr.Value.Kind() != slog.KindBool {
		attr.Value = slog.StringValue(redactedValue)
		return attr
	}
	switch attr.Value.Kind() {
	case slog.KindString:
		attr.Value = slog.StringValue(SanitizeText(attr.Value.String()))
	case slog.KindAny:
		if err, ok := attr.Value.Any().(error); ok {
			attr.Value = slog.StringValue(SanitizeText(err.Error()))
		}
	}
	return attr
}

func isSensitiveKey(key string) bool {
	normalized := strings.NewReplacer("-", "_", ".", "_").Replace(strings.ToLower(strings.TrimSpace(key)))
	for _, marker := range []string{"password", "passwd", "token", "secret", "cookie", "authorization", "api_key", "private_key", "credential"} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

type legacyWriter struct {
	logger *slog.Logger
}

func (w *legacyWriter) Write(payload []byte) (int, error) {
	message := strings.TrimSpace(string(payload))
	level := slog.LevelInfo
	attrs := make([]any, 0, 2)
	for {
		field, rest, found := strings.Cut(message, " ")
		if !found {
			field, rest = message, ""
		}
		key, value, ok := strings.Cut(field, "=")
		if !ok {
			break
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "level":
			if parsed, err := parseLevel(value); err == nil {
				level = parsed
			}
		case "event":
			if event := strings.TrimSpace(value); event != "" {
				attrs = append(attrs, "event", event)
			}
		default:
			break
		}
		if key != "level" && key != "event" {
			break
		}
		message = strings.TrimSpace(rest)
		if message == "" {
			break
		}
	}
	if message == "" {
		message = "legacy log"
	}
	logger := w.logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Log(context.Background(), level, message, attrs...)
	return len(payload), nil
}
