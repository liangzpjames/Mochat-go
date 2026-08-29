package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/store"

	_ "github.com/go-sql-driver/mysql"
)

const legacyCallbackKeyNamespace = "mochat-go:wework-callback|mochat-go:wework-callback:processing|mochat-go:wework-callback:dead"

type cutoverOptions struct {
	dsn               string
	redisAddr         string
	redisPassword     string
	redisDB           int
	timeout           time.Duration
	sourceFingerprint string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, dashboard.SanitizeWeWorkCallbackFailure(err.Error()))
		os.Exit(1)
	}
}

func run() error {
	options, err := parseCutoverOptions(os.Args[1:], os.Getenv, os.Stderr)
	if err != nil {
		return err
	}
	return executeCutover(options, os.Stdout)
}

func parseCutoverOptions(args []string, getenv func(string) string, output io.Writer) (cutoverOptions, error) {
	if getenv("MOCHAT_GO_ALLOW_WEWORK_CALLBACK_LEGACY_CUTOVER") != "1" {
		return cutoverOptions{}, fmt.Errorf("legacy callback cutover is disabled; authorize the maintenance operation explicitly")
	}
	flags := flag.NewFlagSet("mochat-callback-legacy-cutover", flag.ContinueOnError)
	flags.SetOutput(output)
	confirmTrafficStopped := flags.Bool("confirm-legacy-traffic-stopped", false, "confirm legacy producers, consumers, and retry writers are stopped")
	timeout := flags.Duration("timeout", 5*time.Minute, "cutover timeout")
	if err := flags.Parse(args); err != nil {
		return cutoverOptions{}, err
	}
	if flags.NArg() != 0 {
		return cutoverOptions{}, fmt.Errorf("unexpected positional arguments")
	}
	if getenv("MOCHAT_GO_WEWORK_CALLBACK_LEGACY_TRAFFIC_STOPPED") != "1" {
		return cutoverOptions{}, fmt.Errorf("maintenance token is required after all legacy callback producers, consumers, and retry writers are stopped")
	}
	if !*confirmTrafficStopped {
		return cutoverOptions{}, fmt.Errorf("confirm-legacy-traffic-stopped is required after stopping all legacy callback producers, consumers, and retry writers")
	}
	if *timeout <= 0 {
		return cutoverOptions{}, fmt.Errorf("timeout must be positive")
	}
	dsn := strings.TrimSpace(getenv("MOCHAT_GO_MYSQL_DSN"))
	redisAddr := strings.TrimSpace(getenv("MOCHAT_REDIS_ADDR"))
	if dsn == "" || redisAddr == "" {
		return cutoverOptions{}, fmt.Errorf("MOCHAT_GO_MYSQL_DSN and MOCHAT_REDIS_ADDR are required")
	}
	redisDB := 0
	if rawDB := strings.TrimSpace(getenv("MOCHAT_REDIS_DB")); rawDB != "" {
		parsedDB, err := strconv.Atoi(rawDB)
		if err != nil || parsedDB < 0 {
			return cutoverOptions{}, fmt.Errorf("MOCHAT_REDIS_DB must be a non-negative integer")
		}
		redisDB = parsedDB
	}
	fingerprint, err := legacySourceFingerprint(redisAddr, redisDB)
	if err != nil {
		return cutoverOptions{}, err
	}
	return cutoverOptions{
		dsn: dsn, redisAddr: redisAddr, redisPassword: getenv("MOCHAT_REDIS_PASSWORD"),
		redisDB: redisDB, timeout: *timeout, sourceFingerprint: fingerprint,
	}, nil
}

func legacySourceFingerprint(redisAddr string, redisDB int) (string, error) {
	redisAddr = strings.ToLower(strings.TrimSpace(redisAddr))
	if redisAddr == "" || redisDB < 0 {
		return "", fmt.Errorf("invalid legacy Redis source identity")
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("addr=%s\ndb=%d\nkeys=%s", redisAddr, redisDB, legacyCallbackKeyNamespace)))
	return hex.EncodeToString(sum[:]), nil
}

func executeCutover(options cutoverOptions, output io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), options.timeout)
	defer cancel()
	db, err := sql.Open("mysql", options.dsn)
	if err != nil {
		return fmt.Errorf("open mysql: %w", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping mysql: %w", err)
	}

	legacy := store.NewRedisStore(store.RedisConfig{Addr: options.redisAddr, Password: options.redisPassword, DB: options.redisDB})
	defer legacy.Close()
	if err := legacy.Ping(ctx); err != nil {
		return fmt.Errorf("ping legacy redis: %w", err)
	}
	cutoverStore := store.NewMySQLStore(db)
	state, err := cutoverStore.WeWorkCallbackLegacyCutover(ctx)
	if err != nil {
		return fmt.Errorf("read durable cutover marker: %w", err)
	}
	if state.SourceFingerprint != "" && state.SourceFingerprint != options.sourceFingerprint {
		return dashboard.ErrLegacyWeWorkCallbackSourceMismatch
	}
	if state.Status == "completed" {
		stats, err := legacy.PreflightLegacyWeWorkCallbackBacklog(ctx)
		if err != nil {
			return fmt.Errorf("preflight completed legacy source: %w", err)
		}
		if stats.Pending != 0 || stats.Processing != 0 || stats.Dead != 0 {
			return fmt.Errorf("completed legacy cutover source is not empty: pending=%d processing=%d dead=%d", stats.Pending, stats.Processing, stats.Dead)
		}
		fmt.Fprintf(output, "wework callback legacy cutover %s already completed and source is empty\n", dashboard.LegacyWeWorkCallbackCutoverName)
		return nil
	}
	if err := cutoverStore.BeginWeWorkCallbackLegacyCutover(ctx, options.sourceFingerprint); err != nil {
		return fmt.Errorf("bind durable cutover source before import: %w", err)
	}
	imported, err := dashboard.ImportLegacyWeWorkCallbackBacklog(ctx, legacy, cutoverStore, log.Default())
	if err != nil {
		if markerErr := cutoverStore.FailWeWorkCallbackLegacyCutover(ctx, options.sourceFingerprint, imported, err.Error()); markerErr != nil {
			return fmt.Errorf("legacy cutover failed: %w; durable failure marker: %v", err, markerErr)
		}
		return err
	}
	if err := cutoverStore.CompleteWeWorkCallbackLegacyCutover(ctx, options.sourceFingerprint, imported); err != nil {
		return fmt.Errorf("complete durable cutover marker: %w", err)
	}
	fmt.Fprintf(output, "wework callback legacy cutover %s completed imported=%d\n", dashboard.LegacyWeWorkCallbackCutoverName, imported)
	return nil
}
