package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"jiyi/mochat-go/internal/observability"
)

func configureLogging() {
	logger, err := observability.ConfigureFromEnv("mochat-go")
	if err != nil {
		fmt.Fprintln(os.Stderr, "日志配置无效；请检查 MOCHAT_LOG_LEVEL 和 MOCHAT_LOG_FORMAT")
		os.Exit(1)
	}
	slog.SetDefault(logger)
	observability.BridgeStandardLog(logger)
	logger.Info("服务开始启动；如启动失败请检查随后出现的 ERROR 日志",
		"event", "runtime_starting",
		"component", "runtime",
		"step", "startup",
		"result", "started",
	)
}

func withHTTPLogging(next http.Handler) http.Handler {
	return observability.HTTPMiddleware(slog.Default())(next)
}

func structuredLogger() *slog.Logger {
	return slog.Default()
}

func logRuntimeListening(listener, addr string, phpFallbackEnabled bool) {
	slog.Default().Info("服务已开始监听；请求失败时请按 request_id 关联排查",
		"event", "runtime_listening",
		"component", "runtime",
		"step", "serve",
		"result", "ready",
		"listener", listener,
		"listen_addr", addr,
		"php_fallback_enabled", phpFallbackEnabled,
	)
}

func debugf(format string, args ...any) {
	slog.Default().Debug(fmt.Sprintf(format, args...),
		"event", "runtime_debug_detail",
		"component", "runtime",
		"step", "startup_detail",
		"result", "observed",
	)
}

func routeDebugf(format string, args ...any) {
	slog.Default().Debug(fmt.Sprintf(format, args...),
		"event", "route_registered",
		"component", "runtime",
		"step", "route_registration",
		"result", "enabled",
	)
}

func fatal(args ...any) {
	slog.Default().Error("服务无法继续运行；请按错误内容检查配置或依赖",
		"event", "runtime_failed",
		"component", "runtime",
		"step", "startup_or_serve",
		"result", "failed",
		"error_code", "RUNTIME_FAILED",
		"error", fmt.Sprint(args...),
	)
	os.Exit(1)
}

func fatalf(format string, args ...any) {
	fatal(fmt.Sprintf(format, args...))
}
