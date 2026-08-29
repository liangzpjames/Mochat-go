package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"jiyi/mochat-go/internal/archivebridge"
	"jiyi/mochat-go/internal/mysqlconn"
	"jiyi/mochat-go/internal/observability"
	"jiyi/mochat-go/internal/wecomcredentials"
)

type commandConfig struct {
	address            string
	bridgeBearer       string
	fixtureEnabled     bool
	sdkEnabled         bool
	fixtureAdminBearer string
	fixtureStatePath   string
}

type driverRegistrar interface {
	RegisterAll(context.Context) (int, error)
	Close() error
}

type registrarFactory func(*archivebridge.Store) (driverRegistrar, error)

type bridgeServer interface {
	ListenAndServe() error
	Shutdown(context.Context) error
	Close() error
}

type databaseOpener func(string) (*sql.DB, error)

type managedProductionRegistrar struct {
	driverRegistrar
	db        *sql.DB
	closeOnce sync.Once
	closeErr  error
}

func (r *managedProductionRegistrar) Close() error {
	if r == nil {
		return nil
	}
	r.closeOnce.Do(func() {
		var driverErr error
		if r.driverRegistrar != nil {
			driverErr = r.driverRegistrar.Close()
		}
		var dbErr error
		if r.db != nil {
			if err := r.db.Close(); err != nil {
				dbErr = &archivebridge.BridgeError{Code: "ARCHIVE_DRIVER_CLOSE_FAILED", Cause: err}
			}
		}
		r.closeErr = errors.Join(driverErr, dbErr)
	})
	return r.closeErr
}

func main() {
	logger, err := observability.ConfigureFromEnv("mochat-archive-bridge")
	if err != nil {
		fmt.Fprintln(os.Stderr, "日志配置无效；请检查 MOCHAT_LOG_LEVEL 和 MOCHAT_LOG_FORMAT")
		os.Exit(1)
	}
	slog.SetDefault(logger)
	observability.BridgeStandardLog(logger)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Getenv, defaultRegistrarFactory); err != nil {
		logger.Error("会话存档 bridge 无法继续运行；请检查配置、监听端口和存储",
			"event", "archive_bridge_failed", "component", "archive", "step", "serve", "result", "failed",
			"error_code", "ARCHIVE_BRIDGE_FAILED", "error", observability.SanitizeText(err.Error()))
		os.Exit(1)
	}
}

func defaultRegistrarFactory(store *archivebridge.Store) (driverRegistrar, error) {
	return newProductionRegistrar(os.Getenv, mysqlconn.Open, store)
}

func newProductionRegistrar(getenv func(string) string, openDatabase databaseOpener, bridgeStore *archivebridge.Store) (driverRegistrar, error) {
	if getenv == nil || openDatabase == nil || bridgeStore == nil {
		return nil, &archivebridge.BridgeError{Code: "ARCHIVE_DRIVER_UNAVAILABLE"}
	}
	dsn := strings.TrimSpace(getenv("MOCHAT_MYSQL_DSN"))
	stateRoot := strings.TrimSpace(getenv("MOCHAT_ARCHIVE_BRIDGE_STATE_ROOT"))
	if stateRoot == "" {
		stateRoot = "/app/storage/archive-bridge/finance"
	}
	encryption, dedicated := wecomcredentials.SelectEncryptionSource(
		wecomcredentials.EncryptionSource{
			Key: getenv("MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY"), Keys: getenv("MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEYS"), KeyID: getenv("MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY_ID"),
		},
		wecomcredentials.EncryptionSource{
			Key: getenv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY"), Keys: getenv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEYS"), KeyID: getenv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY_ID"),
		},
		wecomcredentials.EncryptionSource{
			Key: getenv("MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY"), Keys: getenv("MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEYS"), KeyID: getenv("MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY_ID"),
		},
		wecomcredentials.EncryptionSource{
			Key: getenv("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY"), Keys: getenv("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEYS"), KeyID: getenv("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY_ID"),
		},
	)
	if dsn == "" || (strings.TrimSpace(encryption.Key) == "" && strings.TrimSpace(encryption.Keys) == "") {
		return nil, &archivebridge.BridgeError{Code: "ARCHIVE_DRIVER_UNAVAILABLE"}
	}
	credentials, err := wecomcredentials.NewManager(wecomcredentials.Config{
		EncryptionKey: encryption.Key, EncryptionKeys: encryption.Keys, EncryptionKeyID: encryption.KeyID,
		RequireEncryption: true, DedicatedConfigured: dedicated,
	})
	if err != nil {
		return nil, &archivebridge.BridgeError{Code: "ARCHIVE_DRIVER_UNAVAILABLE", Cause: err}
	}
	db, err := openDatabase(dsn)
	if err != nil {
		return nil, &archivebridge.BridgeError{Code: "ARCHIVE_BINDING_SOURCE_UNAVAILABLE", Cause: err}
	}
	pingCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		return nil, errors.Join(&archivebridge.BridgeError{Code: "ARCHIVE_BINDING_SOURCE_UNAVAILABLE", Cause: err}, db.Close())
	}
	provider, err := archivebridge.NewMySQLProductionProvider(db, credentials, stateRoot, nil)
	if err != nil {
		return nil, errors.Join(err, db.Close())
	}
	registrar, err := archivebridge.NewDriverRegistrar(provider, provider, bridgeStore)
	if err != nil {
		return nil, errors.Join(err, db.Close())
	}
	return &managedProductionRegistrar{driverRegistrar: registrar, db: db}, nil
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

func run(ctx context.Context, getenv func(string) string, newRegistrar registrarFactory) error {
	if ctx == nil {
		return errors.New("archive bridge context is required")
	}
	config, err := loadConfig(getenv)
	if err != nil {
		return err
	}
	store := archivebridge.NewStore()
	var registrar driverRegistrar
	if config.sdkEnabled {
		if newRegistrar == nil {
			return &archivebridge.BridgeError{Code: "ARCHIVE_DRIVER_UNAVAILABLE"}
		}
		registrar, err = newRegistrar(store)
		if err != nil {
			return err
		}
		if registrar == nil {
			return &archivebridge.BridgeError{Code: "ARCHIVE_DRIVER_UNAVAILABLE"}
		}
		count, registerErr := registrar.RegisterAll(ctx)
		if registerErr != nil {
			return errors.Join(registerErr, registrar.Close())
		}
		if count == 0 || store.DriverCount() == 0 {
			return errors.Join(&archivebridge.BridgeError{Code: "ARCHIVE_DRIVER_UNAVAILABLE"}, registrar.Close())
		}
	}
	bridgeConfig := archivebridge.Config{BearerToken: config.bridgeBearer}
	if config.fixtureEnabled {
		manager, err := archivebridge.NewFixtureManager(config.fixtureStatePath, store)
		if err != nil {
			return closeRegistrar(registrar, fmt.Errorf("load fixture state: %w", err))
		}
		bridgeConfig.FixtureEnabled = true
		bridgeConfig.FixtureAdminToken = config.fixtureAdminBearer
		bridgeConfig.FixtureManager = manager
	}
	handler, err := archivebridge.NewHandler(bridgeConfig, store)
	if err != nil {
		return closeRegistrar(registrar, err)
	}
	server := &http.Server{Addr: config.address, Handler: newReadinessHandler(handler, store, config.sdkEnabled), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 35 * time.Second, WriteTimeout: 35 * time.Second, IdleTimeout: 60 * time.Second}
	log.Printf("archive bridge listening on %s; local fixtures enabled=%t; production SDK enabled=%t", config.address, config.fixtureEnabled, config.sdkEnabled)
	return serve(ctx, server, registrar)
}

func newReadinessHandler(next http.Handler, store *archivebridge.Store, production bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/readyz" {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if production && store.DriverCount() == 0 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"unavailable","errcode":"ARCHIVE_DRIVER_UNAVAILABLE"}` + "\n"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"status":"ready","bindingCount":%d,"production":%t}`+"\n", store.DriverCount(), production)
	})
}

func serve(ctx context.Context, server bridgeServer, registrar driverRegistrar) error {
	listenDone := make(chan error, 1)
	go func() {
		listenDone <- server.ListenAndServe()
	}()
	var listenErr, shutdownErr, forceCloseErr error
	listenReturned := false
	select {
	case <-ctx.Done():
	case listenErr = <-listenDone:
		listenReturned = true
	}
	if !listenReturned || !errors.Is(listenErr, http.ErrServerClosed) {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		shutdownErr = server.Shutdown(shutdownCtx)
		cancel()
		if shutdownErr != nil {
			forceCloseErr = server.Close()
		}
	}
	if !listenReturned {
		listenErr = <-listenDone
	}
	if errors.Is(listenErr, http.ErrServerClosed) {
		listenErr = nil
	}
	var closeErr error
	if registrar != nil {
		closeErr = registrar.Close()
	}
	return errors.Join(listenErr, shutdownErr, forceCloseErr, closeErr)
}

func closeRegistrar(registrar driverRegistrar, runErr error) error {
	if registrar == nil {
		return runErr
	}
	return errors.Join(runErr, registrar.Close())
}
