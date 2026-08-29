package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/store"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, dashboard.SanitizeWeWorkCallbackFailure(err.Error()))
		os.Exit(1)
	}
}

func run() error {
	if os.Getenv("MOCHAT_GO_ALLOW_WEWORK_CALLBACK_LEGACY_CUTOVER") != "1" {
		return fmt.Errorf("legacy callback cutover is disabled; set MOCHAT_GO_ALLOW_WEWORK_CALLBACK_LEGACY_CUTOVER=1 only in a maintenance window")
	}
	dsn := flag.String("dsn", "", "MySQL DSN containing migration 0172")
	redisAddr := flag.String("redis-addr", "", "legacy Redis address")
	redisPassword := flag.String("redis-password", "", "legacy Redis password")
	redisDB := flag.Int("redis-db", 0, "legacy Redis database")
	confirmStopped := flag.Bool("confirm-producers-stopped", false, "confirm every old callback Redis producer has been stopped")
	timeout := flag.Duration("timeout", 5*time.Minute, "cutover timeout")
	flag.Parse()
	if strings.TrimSpace(*dsn) == "" || strings.TrimSpace(*redisAddr) == "" {
		return fmt.Errorf("dsn and redis-addr are required")
	}
	if !*confirmStopped {
		return fmt.Errorf("confirm-producers-stopped is required; stop every old callback Redis producer before cutover")
	}
	if *timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}

	db, err := sql.Open("mysql", *dsn)
	if err != nil {
		return fmt.Errorf("open mysql: %w", err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping mysql: %w", err)
	}
	cutoverStore := store.NewMySQLStore(db)
	completed, err := cutoverStore.WeWorkCallbackLegacyCutoverCompleted(ctx)
	if err != nil {
		return fmt.Errorf("read durable cutover marker: %w", err)
	}
	if completed {
		fmt.Printf("wework callback legacy cutover %s already completed\n", dashboard.LegacyWeWorkCallbackCutoverName)
		return nil
	}

	legacy := store.NewRedisStore(store.RedisConfig{Addr: *redisAddr, Password: *redisPassword, DB: *redisDB})
	defer legacy.Close()
	if err := legacy.Ping(ctx); err != nil {
		return fmt.Errorf("ping legacy redis: %w", err)
	}
	imported, err := dashboard.ImportLegacyWeWorkCallbackBacklog(ctx, legacy, cutoverStore, log.Default())
	if err != nil {
		if markerErr := cutoverStore.FailWeWorkCallbackLegacyCutover(ctx, imported, err.Error()); markerErr != nil {
			return fmt.Errorf("legacy cutover failed: %w; durable failure marker: %v", err, markerErr)
		}
		return err
	}
	if err := cutoverStore.CompleteWeWorkCallbackLegacyCutover(ctx, imported); err != nil {
		return fmt.Errorf("complete durable cutover marker: %w", err)
	}
	fmt.Printf("wework callback legacy cutover %s completed imported=%d\n", dashboard.LegacyWeWorkCallbackCutoverName, imported)
	return nil
}
