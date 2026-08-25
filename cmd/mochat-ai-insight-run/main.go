// Command mochat-ai-insight-run performs one explicit AI insight backfill.
// It does not enable the daily scheduler or run-on-start behavior.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	aiinsight "jiyi/mochat-go/internal/modules/ai-insight"
	"jiyi/mochat-go/internal/modules/providers"
	openai "jiyi/mochat-go/internal/modules/providers/ai/openai"

	_ "github.com/go-sql-driver/mysql"
)

type runtimeConfig struct {
	DSN             string
	BaseURL         string
	APIKey          string
	Model           string
	ProviderTimeout time.Duration
	RunTimeout      time.Duration
	LookbackDays    int
}

func main() {
	config, err := loadRuntimeConfig(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), config.RunTimeout)
	defer cancel()
	if err := run(ctx, config); err != nil {
		log.Fatal(err)
	}
}

func loadRuntimeConfig(getenv func(string) string) (runtimeConfig, error) {
	dsn := strings.TrimSpace(getenv("MOCHAT_MYSQL_DSN"))
	if dsn == "" {
		return runtimeConfig{}, errors.New("MOCHAT_MYSQL_DSN is required")
	}
	keyFile := strings.TrimSpace(getenv("MOCHAT_GO_AI_PROVIDER_KEY_FILE"))
	if keyFile == "" {
		return runtimeConfig{}, errors.New("MOCHAT_GO_AI_PROVIDER_KEY_FILE is required for one-shot analysis")
	}
	contents, err := os.ReadFile(keyFile)
	if err != nil {
		return runtimeConfig{}, fmt.Errorf("read protected AI provider key file: %w", err)
	}
	apiKey := strings.TrimSpace(string(contents))
	if apiKey == "" {
		return runtimeConfig{}, errors.New("protected AI provider key file is empty")
	}
	return runtimeConfig{
		DSN:             dsn,
		BaseURL:         strings.TrimSpace(getenv("MOCHAT_GO_AI_PROVIDER_BASE_URL")),
		APIKey:          apiKey,
		Model:           strings.TrimSpace(getenv("MOCHAT_GO_AI_PROVIDER_MODEL")),
		ProviderTimeout: positiveDuration(getenv("MOCHAT_GO_AI_PROVIDER_TIMEOUT_SECONDS"), time.Second, 120*time.Second),
		RunTimeout:      positiveDuration(getenv("MOCHAT_GO_AI_RUN_TIMEOUT_MINUTES"), time.Minute, 30*time.Minute),
		LookbackDays:    positiveInt(getenv("MOCHAT_GO_AI_RUN_LOOKBACK_DAYS"), 30),
	}, nil
}

func positiveInt(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func positiveDuration(raw string, unit, fallback time.Duration) time.Duration {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return fallback
	}
	return time.Duration(value) * unit
}

func run(ctx context.Context, config runtimeConfig) error {
	provider, err := openai.New(openai.Config{
		BaseURL: config.BaseURL,
		APIKey:  config.APIKey,
		Model:   config.Model,
		Timeout: config.ProviderTimeout,
	})
	if err != nil {
		return err
	}
	if provider.Status().State != providers.StateReady {
		return errors.New("AI provider is not ready")
	}
	db, err := sql.Open("mysql", config.DSN)
	if err != nil {
		return errors.New("open AI insight database connection failed")
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return errors.New("AI insight database is unavailable")
	}

	startedAt := time.Now().Add(-2 * time.Second)
	metadata := provider.Metadata()
	log.Printf("one-shot AI insight analysis started provider=%s model=%s", metadata.Provider, metadata.Model)
	runner := aiinsight.NewDailyAnalysisRunner(db, provider, log.Default())
	endAt := time.Now()
	startAt := endAt.AddDate(0, 0, -config.LookbackDays)
	if err := runner.RunBackfill(ctx, startAt, endAt); err != nil {
		return fmt.Errorf("one-shot AI insight analysis failed: %w", err)
	}
	var runCount, failedRuns, failedCandidates, successfulCandidates int
	err = db.QueryRowContext(ctx, `SELECT COUNT(*),
COALESCE(SUM(CASE WHEN status<>'succeeded' THEN 1 ELSE 0 END),0),
COALESCE(SUM(failure_count),0),
COALESCE(SUM(success_count),0)
FROM mochat_go_ai_insight_runs WHERE created_at>=?`, startedAt).Scan(&runCount, &failedRuns, &failedCandidates, &successfulCandidates)
	if err != nil {
		return errors.New("audit one-shot AI insight runs failed")
	}
	if runCount == 0 {
		return errors.New("one-shot AI insight analysis created no auditable runs")
	}
	if failedRuns > 0 || failedCandidates > 0 {
		return fmt.Errorf("one-shot AI insight analysis incomplete: runs=%d failed_runs=%d successful_candidates=%d failed_candidates=%d", runCount, failedRuns, successfulCandidates, failedCandidates)
	}
	log.Printf("one-shot AI insight analysis completed runs=%d successful_candidates=%d", runCount, successfulCandidates)
	return nil
}
