package aiinsight

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"

	"jiyi/mochat-go/internal/modules/providers"
	transporthttp "jiyi/mochat-go/internal/modules/ai-insight/transport/http"
)

var dailyAnalysisPages = []string{
	"session-analysis",
	"smart-analysis",
	"emotion",
	"employee-score",
	"communication-keyword",
}

// DailyConfig configures the once-per-day AI insight analysis job.
type DailyConfig struct {
	DB         *sql.DB
	AI         providers.AIProvider
	Hour       int
	RunOnStart bool
	Logger     *log.Logger
}

// DailyAnalysisRunner generates and persists analysis results for every
// active corp. Page-open reads never call the model; this job is the only
// writer so each analysis page is refreshed at most once per day.
type DailyAnalysisRunner struct {
	db       *sql.DB
	ai       providers.AIProvider
	analysis transporthttp.AnalysisStore
	logger   *log.Logger
}

func NewDailyAnalysisRunner(db *sql.DB, ai providers.AIProvider, logger *log.Logger) *DailyAnalysisRunner {
	if logger == nil {
		logger = log.Default()
	}
	return &DailyAnalysisRunner{db: db, ai: ai, analysis: transporthttp.NewSQLAnalysisStore(db), logger: logger}
}

// RunOnce analyzes archive texts for every active corp and persists the
// results. A failed corp or page never aborts the whole run.
func (r *DailyAnalysisRunner) RunOnce(ctx context.Context) error {
	if r == nil || r.db == nil {
		return errors.New("AI insight daily analysis database is unavailable")
	}
	if r.ai == nil || r.ai.Status().State != providers.StateReady {
		return errors.New("AI insight daily analysis skipped: AI provider is not ready")
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id
		FROM mc_corp c
		JOIN mochat_go_tenant_corp_bindings b ON b.tenant_id = c.tenant_id AND b.corp_id = c.id
		WHERE c.deleted_at IS NULL AND b.status = 2
		ORDER BY c.id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	corpIDs := []int64{}
	for rows.Next() {
		var corpID int64
		if rows.Scan(&corpID) == nil {
			corpIDs = append(corpIDs, corpID)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(corpIDs) == 0 {
		r.logger.Printf("AI insight daily analysis: no active corps, nothing to analyze")
		return nil
	}
	for _, corpID := range corpIDs {
		if err := r.runCorp(ctx, corpID); err != nil {
			r.logger.Printf("AI insight daily analysis failed for corp %d: %v", corpID, err)
		}
	}
	return nil
}

func (r *DailyAnalysisRunner) runCorp(ctx context.Context, corpID int64) error {
	texts, err := transporthttp.FetchArchiveTexts(ctx, r.db, corpID, 20, nil, false)
	if err != nil {
		return err
	}
	if len(texts) == 0 {
		r.logger.Printf("AI insight daily analysis: corp %d has no archive texts", corpID)
		return nil
	}
	now := time.Now()
	for _, page := range dailyAnalysisPages {
		system, prompt := transporthttp.BuildAnalysisPrompt(page, texts)
		summary, chatErr := r.ai.Chat(ctx, providers.ChatRequest{System: system, Prompt: prompt})
		if chatErr != nil {
			r.logger.Printf("AI insight daily analysis: corp %d page %s chat failed: %v", corpID, page, chatErr)
			continue
		}
		payload := map[string]any{
			"summary":     summary,
			"keywords":    []any{},
			"generatedAt": now.Format(time.RFC3339),
		}
		if saveErr := r.analysis.Save(ctx, corpID, page, "succeeded", payload, ""); saveErr != nil {
			r.logger.Printf("AI insight daily analysis: corp %d page %s save failed: %v", corpID, page, saveErr)
			continue
		}
		r.logger.Printf("AI insight daily analysis: corp %d page %s saved", corpID, page)
	}
	return nil
}

// RunDailyLoop schedules RunOnce at the configured local hour (default 00:00,
// i.e. 每日 24 点) in Asia/Shanghai and reschedules after each run.
func RunDailyLoop(ctx context.Context, config DailyConfig) {
	if config.Logger == nil {
		config.Logger = log.Default()
	}
	if config.DB == nil || config.AI == nil {
		config.Logger.Printf("AI insight daily loop skipped: dependencies missing")
		return
	}
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		location = time.Local
	}
	runner := NewDailyAnalysisRunner(config.DB, config.AI, config.Logger)
	run := func() {
		started := time.Now()
		config.Logger.Printf("AI insight daily analysis started at %s", started.In(location).Format(time.RFC3339))
		if err := runner.RunOnce(ctx); err != nil {
			config.Logger.Printf("AI insight daily analysis failed: %v", err)
			return
		}
		config.Logger.Printf("AI insight daily analysis finished in %s", time.Since(started).Round(time.Second))
	}
	if config.RunOnStart {
		run()
	}
	for {
		now := time.Now().In(location)
		next := time.Date(now.Year(), now.Month(), now.Day(), config.Hour, 0, 0, 0, location)
		if !next.After(now) {
			next = next.AddDate(0, 0, 1)
		}
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		run()
	}
}
