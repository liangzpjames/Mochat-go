package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"jiyi/mochat-go/internal/archivebridge"
	"jiyi/mochat-go/internal/observability"
)

type commandConfig struct {
	address            string
	bridgeBearer       string
	fixtureEnabled     bool
	sdkEnabled         bool
	fixtureAdminBearer string
	fixtureStatePath   string
}

func main() {
	logger, err := observability.ConfigureFromEnv("mochat-archive-bridge")
	if err != nil {
		fmt.Fprintln(os.Stderr, "日志配置无效；请检查 MOCHAT_LOG_LEVEL 和 MOCHAT_LOG_FORMAT")
		os.Exit(1)
	}
	slog.SetDefault(logger)
	observability.BridgeStandardLog(logger)
	if err := run(os.Getenv); err != nil {
		logger.Error("会话存档 bridge 无法继续运行；请检查配置、监听端口和存储",
			"event", "archive_bridge_failed", "component", "archive", "step", "serve", "result", "failed",
			"error_code", "ARCHIVE_BRIDGE_FAILED", "error", observability.SanitizeText(err.Error()))
		os.Exit(1)
	}
}

func loadConfig(getenv func(string) string) (commandConfig, error) {
	config := commandConfig{
		address:            strings.TrimSpace(getenv("MOCHAT_ARCHIVE_BRIDGE_ADDR")),
		bridgeBearer:       strings.TrimSpace(getenv("MOCHAT_ARCHIVE_BRIDGE_BEARER")),
		fixtureEnabled:     strings.EqualFold(strings.TrimSpace(getenv("MOCHAT_ARCHIVE_FIXTURE_ENABLED")), "true"),
		sdkEnabled:         strings.EqualFold(strings.TrimSpace(getenv("MOCHAT_ARCHIVE_SDK_ENABLED")), "true"),
		fixtureAdminBearer: strings.TrimSpace(getenv("MOCHAT_ARCHIVE_FIXTURE_ADMIN_BEARER")),
		fixtureStatePath:   strings.TrimSpace(getenv("MOCHAT_ARCHIVE_FIXTURE_STATE_PATH")),
	}
	if config.address == "" {
		config.address = ":8083"
	}
	if len(config.bridgeBearer) < 40 {
		return commandConfig{}, errors.New("MOCHAT_ARCHIVE_BRIDGE_BEARER must contain at least 40 characters")
	}
	if config.fixtureEnabled && config.sdkEnabled {
		return commandConfig{}, errors.New("fixture and production SDK modes are mutually exclusive")
	}
	if config.fixtureEnabled {
		if len(config.fixtureAdminBearer) < 40 || config.fixtureAdminBearer == config.bridgeBearer {
			return commandConfig{}, errors.New("fixture admin bearer must be independent and contain at least 40 characters")
		}
		if config.fixtureStatePath == "" {
			return commandConfig{}, errors.New("MOCHAT_ARCHIVE_FIXTURE_STATE_PATH is required when fixtures are enabled")
		}
	}
	return config, nil
}

func run(getenv func(string) string) error {
	config, err := loadConfig(getenv)
	if err != nil {
		return err
	}
	store := archivebridge.NewStore()
	bridgeConfig := archivebridge.Config{BearerToken: config.bridgeBearer}
	if config.fixtureEnabled {
		manager, err := archivebridge.NewFixtureManager(config.fixtureStatePath, store)
		if err != nil {
			return fmt.Errorf("load fixture state: %w", err)
		}
		bridgeConfig.FixtureEnabled = true
		bridgeConfig.FixtureAdminToken = config.fixtureAdminBearer
		bridgeConfig.FixtureManager = manager
	}
	handler, err := archivebridge.NewHandler(bridgeConfig, store)
	if err != nil {
		return err
	}
	server := &http.Server{Addr: config.address, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 35 * time.Second, WriteTimeout: 35 * time.Second, IdleTimeout: 60 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	shutdownDone := make(chan struct{})
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		close(shutdownDone)
	}()
	log.Printf("archive bridge listening on %s; local fixtures enabled=%t; production SDK enabled=%t", config.address, config.fixtureEnabled, config.sdkEnabled)
	err = server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	if ctx.Err() != nil {
		<-shutdownDone
	}
	return nil
}
