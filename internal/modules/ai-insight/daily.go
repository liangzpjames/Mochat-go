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

// DailyConfig configures the once-per-day AI insight analysis job.
type DailyConfig struct {
	DB         *sql.DB
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
	resolver     providers.AIProviderResolver
	analysis     transporthttp.AnalysisStore
	conversation *ConversationAnalysisRunner
	logger       *log.Logger
}

// NewDailyAnalysisRunnerWithResolver is the production construction path. It
// never accepts a process-wide AI client; each corp run resolves its own
// database-backed provider once inside the conversation runner.
func NewDailyAnalysisRunnerWithResolver(db *sql.DB, repo Repository, resolver providers.AIProviderResolver, logger *log.Logger, assistants ...any) *DailyAnalysisRunner {
	if logger == nil {
		logger = log.Default()
	}
	var assistant any
	if len(assistants) > 0 {
		assistant = assistants[0]
	} else {
		assistant, _ = aisettingsmysql.NewAgentRepository(db)
	}
	return &DailyAnalysisRunner{db: db, resolver: resolver, analysis: transporthttp.NewSQLAnalysisStore(db), conversation: NewConversationAnalysisRunner(repo, resolver, RunnerConfig{}, logger, assistant), logger: logger}
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
				r.logger.Printf("AI insight conversation analysis failed for corp %d: %s", corp.corpID, safeRunFailureCode(conversationErr, "AI_ANALYSIS_FAILED"))
			}
		}
	}
	return nil
}

// RunDailyLoop schedules RunOnce at the configured local hour (default 00:00,
// i.e. 每日 24 点) in Asia/Shanghai and reschedules after each run.
func RunDailyLoop(ctx context.Context, config DailyConfig) {
	if config.Logger == nil {
		config.Logger = log.Default()
	}
	if config.DB == nil || config.Resolver == nil {
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
	}
	run := func() {
		started := time.Now()
		config.Logger.Printf("AI insight daily analysis started at %s", started.In(location).Format(time.RFC3339))
		if err := runner.RunOnce(ctx); err != nil {
			logDailyRunFailure(config.Logger, err)
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

func logDailyRunFailure(logger *log.Logger, err error) {
	logger.Printf("AI insight daily analysis failed: %s", safeRunFailureCode(err, "AI_DAILY_RUN_FAILED"))
}
