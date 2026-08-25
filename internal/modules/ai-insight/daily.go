package aiinsight

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"

	transporthttp "jiyi/mochat-go/internal/modules/ai-insight/transport/http"
	aisettingsmysql "jiyi/mochat-go/internal/modules/ai-settings/adapters/mysql"
	"jiyi/mochat-go/internal/modules/providers"
)

var dailyAnalysisPages = []string{
	"emotion",
	"employee-score",
	"communication-keyword",
}

// DailyConfig configures the once-per-day AI insight analysis job.
type DailyConfig struct {
	DB         *sql.DB
	AI         providers.AIProvider // compatibility-only test construction
	Resolver   providers.AIProviderResolver
	Hour       int
	RunOnStart bool
	Logger     *log.Logger
}

// DailyAnalysisRunner generates and persists analysis results for every
// active corp. Page-open reads never call the model; this job is the only
// writer so each analysis page is refreshed at most once per day.
type DailyAnalysisRunner struct {
	db           *sql.DB
	ai           providers.AIProvider
	resolver     providers.AIProviderResolver
	analysis     transporthttp.AnalysisStore
	conversation *ConversationAnalysisRunner
	logger       *log.Logger
}

func NewDailyAnalysisRunner(db *sql.DB, ai providers.AIProvider, logger *log.Logger) *DailyAnalysisRunner {
	if logger == nil {
		logger = log.Default()
	}
	repo := NewSQLRepository(db)
	assistantRepo, _ := aisettingsmysql.NewAgentRepository(db)
	return &DailyAnalysisRunner{db: db, ai: ai, resolver: providers.StaticAIProviderResolver{Provider: ai}, analysis: transporthttp.NewSQLAnalysisStore(db), conversation: NewConversationAnalysisRunner(repo, ai, RunnerConfig{}, logger, assistantRepo), logger: logger}
}

// NewDailyAnalysisRunnerWithResolver is the production construction path. It
// never accepts a process-wide AI client; each corp run resolves its own
// database-backed provider once inside the conversation runner.
func NewDailyAnalysisRunnerWithResolver(db *sql.DB, repo Repository, resolver providers.AIProviderResolver, logger *log.Logger) *DailyAnalysisRunner {
	if logger == nil {
		logger = log.Default()
	}
	return &DailyAnalysisRunner{db: db, resolver: resolver, analysis: transporthttp.NewSQLAnalysisStore(db), conversation: NewConversationAnalysisRunnerWithResolver(repo, resolver, RunnerConfig{}, logger), logger: logger}
}

// RunOnce analyzes archive texts for every active corp and persists the
// results. A failed corp or page never aborts the whole run.
func (r *DailyAnalysisRunner) RunOnce(ctx context.Context) error {
	return r.run(ctx, nil)
}

// RunBackfill executes the same persisted analysis flows over an explicit
// operator-authorized window without enabling any recurring scheduler.
func (r *DailyAnalysisRunner) RunBackfill(ctx context.Context, startAt, endAt time.Time) error {
	if startAt.IsZero() || endAt.IsZero() || !startAt.Before(endAt) {
		return errors.New("AI insight backfill window is invalid")
	}
	return r.run(ctx, &analysisWindow{startAt: startAt, endAt: endAt})
}

func (r *DailyAnalysisRunner) run(ctx context.Context, window *analysisWindow) error {
	if r == nil || r.db == nil {
		return errors.New("AI insight daily analysis database is unavailable")
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT c.tenant_id, c.id
		FROM mc_corp c
		JOIN mochat_go_tenant_corp_bindings b ON b.tenant_id = c.tenant_id AND b.corp_id = c.id
		WHERE c.deleted_at IS NULL AND b.status = 2
		ORDER BY c.id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type corpRef struct{ tenantID, corpID int64 }
	corps := []corpRef{}
	for rows.Next() {
		var ref corpRef
		if rows.Scan(&ref.tenantID, &ref.corpID) == nil {
			corps = append(corps, ref)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(corps) == 0 {
		r.logger.Printf("AI insight daily analysis: no active corps, nothing to analyze")
		return nil
	}
	for _, corp := range corps {
		if r.conversation != nil {
			var conversationErr error
			if window == nil {
				conversationErr = r.conversation.RunCorp(ctx, corp.tenantID, corp.corpID)
			} else {
				conversationErr = r.conversation.RunCorpWindow(ctx, corp.tenantID, corp.corpID, window.startAt, window.endAt)
			}
			if conversationErr != nil {
				r.logger.Printf("AI insight conversation analysis failed for corp %d: %v", corp.corpID, conversationErr)
			}
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
	if config.DB == nil || (config.Resolver == nil && config.AI == nil) {
		config.Logger.Printf("AI insight daily loop skipped: dependencies missing")
		return
	}
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		location = time.Local
	}
	var runner *DailyAnalysisRunner
	if config.Resolver != nil {
		repo := NewSQLRepository(config.DB)
		runner = NewDailyAnalysisRunnerWithResolver(config.DB, repo, config.Resolver, config.Logger)
	} else {
		runner = NewDailyAnalysisRunner(config.DB, config.AI, config.Logger)
	}
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
