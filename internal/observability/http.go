package observability

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

type requestIDContextKey struct{}

var safeRequestID = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

// RequestID returns the request identifier installed by HTTPMiddleware.
func RequestID(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDContextKey{}).(string)
	return requestID
}

// HTTPMiddleware records only write operations and abnormal HTTP outcomes.
// It deliberately ignores request bodies, query strings, and headers.
func HTTPMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get("X-Request-ID")
			if !safeRequestID.MatchString(requestID) {
				requestID = uuid.NewString()
			}
			w.Header().Set("X-Request-ID", requestID)
			ctx := context.WithValue(r.Context(), requestIDContextKey{}, requestID)
			response := &statusResponseWriter{ResponseWriter: w, status: http.StatusOK}
			startedAt := time.Now()
			defer func() {
				if recovered := recover(); recovered != nil {
					if !response.wroteHeader {
						response.WriteHeader(http.StatusInternalServerError)
					}
					logger.Error("关键请求发生未处理异常；请按请求标识检查对应处理步骤",
						"event", "critical_request_panicked", "component", httpComponent(r.URL.Path), "request_id", requestID,
						"method", r.Method, "path", r.URL.Path, "step", "request", "result", "failed",
						"error_code", "HTTP_HANDLER_PANIC", "status_code", http.StatusInternalServerError,
						"duration_ms", time.Since(startedAt).Milliseconds())
					return
				}
				logHTTPOutcome(logger, r.Method, r.URL.Path, requestID, response.status, time.Since(startedAt))
			}()
			next.ServeHTTP(response, r.WithContext(ctx))
		})
	}
}

type statusResponseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusResponseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.status = status
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusResponseWriter) Write(body []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func (w *statusResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func logHTTPOutcome(logger *slog.Logger, method string, path string, requestID string, status int, elapsed time.Duration) {
	if path == "/wecom/suite/callback" {
		return
	}
	component := httpComponent(path)
	attrs := []any{
		"component", component,
		"request_id", requestID,
		"method", method,
		"path", path,
		"step", "request",
		"status_code", status,
		"duration_ms", elapsed.Milliseconds(),
	}

	switch {
	case path == "/readyz" && status >= http.StatusInternalServerError:
		logger.Error("服务未就绪；请检查数据库、配置和外部依赖", append([]any{"event", "readiness_failed", "result", "failed", "error_code", "HTTP_READY_FAILED"}, attrs...)...)
	case path == "/healthz" && status >= http.StatusInternalServerError:
		logger.Error("服务存活检查失败；请检查进程状态", append([]any{"event", "liveness_failed", "result", "failed", "error_code", "HTTP_HEALTH_FAILED"}, attrs...)...)
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		logger.Warn("请求被认证或权限规则拒绝；请检查登录态、租户和权限配置", append([]any{"event", "request_rejected", "result", "rejected", "error_code", httpErrorCode(status)}, attrs...)...)
	case status >= http.StatusInternalServerError:
		logger.Error("关键请求失败；请结合请求标识检查对应业务步骤", append([]any{"event", "critical_request_failed", "result", "failed", "error_code", httpErrorCode(status)}, attrs...)...)
	case isMutatingMethod(method):
		logger.Info("关键写操作完成", append([]any{"event", "critical_request_completed", "result", httpResult(status)}, attrs...)...)
	}
}

func httpErrorCode(status int) string {
	return fmt.Sprintf("HTTP_%d", status)
}

func isMutatingMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func httpResult(status int) string {
	if status >= http.StatusBadRequest {
		return "rejected"
	}
	return "success"
}

func httpComponent(path string) string {
	lowerPath := strings.ToLower(path)
	switch {
	case lowerPath == "/healthz" || lowerPath == "/readyz":
		return "runtime"
	case strings.Contains(lowerPath, "suite/callback") || strings.Contains(lowerPath, "webhook"):
		return "callback"
	case strings.Contains(lowerPath, "archive") || strings.Contains(lowerPath, "work-message"):
		return "archive"
	case strings.Contains(lowerPath, "employee-sync") || strings.Contains(lowerPath, "contact-sync") || strings.Contains(lowerPath, "room-sync"):
		return "wecom_sync"
	case strings.Contains(lowerPath, "/auth") || strings.Contains(lowerPath, "login") || strings.Contains(lowerPath, "activate"):
		return "auth"
	case strings.Contains(lowerPath, "saas"):
		return "saas"
	case strings.Contains(lowerPath, "/ai") || strings.Contains(lowerPath, "provider"):
		return "ai_provider"
	case strings.HasPrefix(lowerPath, "/dashboard"):
		return "dashboard"
	default:
		return "http"
	}
}
