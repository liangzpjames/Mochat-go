package aiinsight

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"jiyi/mochat-go/internal/modules/providers"
)

type RunnerConfig struct {
	BatchLimit    int
	Concurrency   int
	PromptVersion string
	SessionDays   int
	SessionLimit  int
}

type ConversationAnalysisRunner struct {
	repo   Repository
	ai     providers.AIProvider
	config RunnerConfig
	logger *log.Logger
	now    func() time.Time
}

func NewConversationAnalysisRunner(repo Repository, ai providers.AIProvider, config RunnerConfig, logger *log.Logger) *ConversationAnalysisRunner {
	if logger == nil {
		logger = log.Default()
	}
	return &ConversationAnalysisRunner{repo: repo, ai: ai, config: normalizeRunnerConfig(config), logger: logger, now: time.Now}
}

func normalizeRunnerConfig(config RunnerConfig) RunnerConfig {
	if config.BatchLimit <= 0 {
		config.BatchLimit = 200
	}
	if config.Concurrency <= 0 {
		config.Concurrency = 2
	}
	if config.PromptVersion == "" {
		config.PromptVersion = "conversation-v1"
	}
	if config.SessionDays <= 0 {
		config.SessionDays = 30
	}
	if config.SessionLimit <= 0 {
		config.SessionLimit = 200
	}
	return config
}

func (r *ConversationAnalysisRunner) RunCorp(ctx context.Context, tenantID, corpID int64) error {
	if r == nil || r.repo == nil {
		return errors.New("AI insight conversation repository is unavailable")
	}
	if r.ai == nil || r.ai.Status().State != providers.StateReady {
		return r.recordUnavailableRun(ctx, tenantID, corpID, AnalysisTypeSession, 0, "AI provider is not ready")
	}
	now := r.now()
	start := now.AddDate(0, 0, -r.config.SessionDays)
	if err := r.runType(ctx, tenantID, corpID, AnalysisTypeSession, 0, start, now, nil); err != nil {
		r.logger.Printf("AI conversation session analysis failed for corp %d: %v", corpID, err)
	}
	rules, err := r.repo.EnabledRuleVersions(ctx, tenantID, corpID)
	if err != nil {
		return err
	}
	for _, rule := range rules {
		days := rule.LookbackDays
		if days <= 0 {
			days = r.config.SessionDays
		}
		if err := r.runType(ctx, tenantID, corpID, AnalysisTypeSmart, rule.ID, now.AddDate(0, 0, -days), now, &rule); err != nil {
			r.logger.Printf("AI conversation smart analysis failed for corp %d rule %d: %v", corpID, rule.RuleID, err)
		}
	}
	return nil
}

func (r *ConversationAnalysisRunner) runType(ctx context.Context, tenantID, corpID int64, analysisType AnalysisType, ruleVersionID int64, startAt, endAt time.Time, rule *AnalysisRuleVersion) error {
	run := InsightRun{TenantID: tenantID, CorpID: corpID, AnalysisType: analysisType, RuleVersionID: ruleVersionID, Status: AnalysisStatusRunning, PlannedAt: &startAt, StartedAt: &endAt}
	runID, err := r.repo.CreateRun(ctx, run)
	if err != nil {
		return err
	}
	candidates, err := r.repo.ConversationCandidates(ctx, CandidateQuery{TenantID: tenantID, CorpID: corpID, AnalysisType: analysisType, RuleVersionID: ruleVersionID, StartAt: startAt, EndAt: endAt, Limit: r.config.BatchLimit})
	if err != nil {
		_ = r.repo.FinishRun(ctx, runID, InsightRunResult{Status: AnalysisStatusFailed, ErrorSummary: err.Error(), FinishedAt: r.now()})
		return err
	}
	filtered := candidates[:0]
	for _, candidate := range candidates {
		if rule != nil && !ruleAllowsCandidate(*rule, candidate) {
			continue
		}
		if rule != nil && candidate.SourceMessageCount < rule.MinimumMessages {
			continue
		}
		filtered = append(filtered, candidate)
	}
	counts := r.processCandidates(ctx, tenantID, corpID, analysisType, ruleVersionID, filtered, rule)
	counts.CandidateCount = len(filtered)
	if len(candidates) > len(filtered) {
		counts.BacklogCount = len(candidates) - len(filtered)
	}
	counts.Status = AnalysisStatusSucceeded
	counts.FinishedAt = r.now()
	return r.repo.FinishRun(ctx, runID, counts)
}

func (r *ConversationAnalysisRunner) processCandidates(ctx context.Context, tenantID, corpID int64, analysisType AnalysisType, ruleVersionID int64, candidates []ConversationCandidate, rule *AnalysisRuleVersion) InsightRunResult {
	var result InsightRunResult
	if len(candidates) == 0 {
		return result
	}
	type job struct{ candidate ConversationCandidate }
	jobs := make(chan job)
	var wg sync.WaitGroup
	var mu sync.Mutex
	worker := func() {
		defer wg.Done()
		for item := range jobs {
			err := r.processCandidate(ctx, tenantID, corpID, analysisType, ruleVersionID, item.candidate, rule)
			mu.Lock()
			if err != nil {
				result.FailureCount++
				if result.ErrorSummary == "" {
					result.ErrorSummary = err.Error()
				}
			} else {
				result.SuccessCount++
			}
			mu.Unlock()
		}
	}
	for i := 0; i < r.config.Concurrency; i++ {
		wg.Add(1)
		go worker()
	}
	for _, candidate := range candidates {
		select {
		case jobs <- job{candidate: candidate}:
		case <-ctx.Done():
			mu.Lock()
			result.FailureCount++
			mu.Unlock()
		}
	}
	close(jobs)
	wg.Wait()
	return result
}

func (r *ConversationAnalysisRunner) processCandidate(ctx context.Context, tenantID, corpID int64, analysisType AnalysisType, ruleVersionID int64, candidate ConversationCandidate, rule *AnalysisRuleVersion) error {
	previous, err := r.repo.LatestSucceededFingerprint(ctx, tenantID, corpID, analysisType, ruleVersionID, candidate.ConversationKey)
	if err != nil {
		return err
	}
	if previous != "" && previous == candidate.SourceFingerprint {
		return nil
	}
	messages, err := r.repo.ConversationMessages(ctx, ConversationWindowQuery{TenantID: tenantID, CorpID: corpID, ConversationKey: candidate.ConversationKey, StartAt: candidate.SourceStartedAt, EndAt: candidate.SourceEndedAt, Limit: r.config.SessionLimit})
	if err != nil {
		return err
	}
	allowed := make(map[string]struct{}, len(messages))
	for _, message := range messages {
		allowed[message.ID] = struct{}{}
	}
	request := providers.ChatRequest{System: "你是企业微信会话分析助手，只能依据消息证据回答，必须返回合法 JSON，不得输出 Markdown。", Prompt: buildConversationPrompt(analysisType, r.config.PromptVersion, ruleObjective(rule), messages)}
	raw, err := r.ai.Chat(ctx, request)
	if err != nil {
		return r.saveFailedInsight(ctx, tenantID, corpID, candidate, analysisType, ruleVersionID, rule, err)
	}
	insight := ConversationInsight{TenantID: tenantID, CorpID: corpID, AnalysisType: analysisType, RuleVersionID: ruleVersionID, ConversationKey: candidate.ConversationKey, EmployeeID: candidate.EmployeeID, EmployeeName: candidate.EmployeeName, EmployeeAvatar: candidate.EmployeeAvatar, TargetType: candidate.TargetType, TargetID: candidate.TargetID, TargetName: candidate.TargetName, TargetAvatar: candidate.TargetAvatar, SourceStartedAt: candidate.SourceStartedAt, SourceEndedAt: candidate.SourceEndedAt, SourceMessageCount: candidate.SourceMessageCount, SourceFingerprint: candidate.SourceFingerprint, Status: AnalysisStatusSucceeded, Provider: "ai", PromptVersion: r.config.PromptVersion, GeneratedAt: timePtr(r.now())}
	if rule != nil {
		insight.RuleID = rule.RuleID
		insight.RuleVersion = rule.Version
	}
	if analysisType == AnalysisTypeSession {
		parsed, parseErr := ParseSessionAnalysisResult(raw, allowed)
		if parseErr != nil {
			return r.saveFailedInsight(ctx, tenantID, corpID, candidate, analysisType, ruleVersionID, rule, parseErr)
		}
		insight.Summary = parsed.Summary
		insight.SessionResult = &parsed
		insight.ResultJSON, _ = json.Marshal(parsed)
	} else {
		parsed, parseErr := ParseSmartAnalysisResult(raw, allowed)
		if parseErr != nil {
			return r.saveFailedInsight(ctx, tenantID, corpID, candidate, analysisType, ruleVersionID, rule, parseErr)
		}
		insight.Summary = parsed.Conclusion
		insight.SmartResult = &parsed
		insight.ResultJSON, _ = json.Marshal(parsed)
	}
	return r.repo.SaveInsight(ctx, insight)
}

func (r *ConversationAnalysisRunner) saveFailedInsight(ctx context.Context, tenantID, corpID int64, candidate ConversationCandidate, analysisType AnalysisType, ruleVersionID int64, rule *AnalysisRuleVersion, cause error) error {
	insight := ConversationInsight{TenantID: tenantID, CorpID: corpID, AnalysisType: analysisType, RuleVersionID: ruleVersionID, ConversationKey: candidate.ConversationKey, EmployeeID: candidate.EmployeeID, EmployeeName: candidate.EmployeeName, EmployeeAvatar: candidate.EmployeeAvatar, TargetType: candidate.TargetType, TargetID: candidate.TargetID, TargetName: candidate.TargetName, TargetAvatar: candidate.TargetAvatar, SourceStartedAt: candidate.SourceStartedAt, SourceEndedAt: candidate.SourceEndedAt, SourceMessageCount: candidate.SourceMessageCount, SourceFingerprint: candidate.SourceFingerprint, Status: AnalysisStatusFailed, ErrorSummary: cause.Error(), ResultJSON: []byte(`{}`), PromptVersion: r.config.PromptVersion}
	if rule != nil {
		insight.RuleID = rule.RuleID
		insight.RuleVersion = rule.Version
	}
	if err := r.repo.SaveInsight(ctx, insight); err != nil {
		return err
	}
	return cause
}

func (r *ConversationAnalysisRunner) recordUnavailableRun(ctx context.Context, tenantID, corpID int64, analysisType AnalysisType, ruleVersionID int64, message string) error {
	now := r.now()
	runID, err := r.repo.CreateRun(ctx, InsightRun{TenantID: tenantID, CorpID: corpID, AnalysisType: analysisType, RuleVersionID: ruleVersionID, Status: AnalysisStatusFailed, StartedAt: &now})
	if err != nil {
		return err
	}
	return r.repo.FinishRun(ctx, runID, InsightRunResult{Status: AnalysisStatusFailed, ErrorSummary: message, FinishedAt: now})
}

func buildConversationPrompt(analysisType AnalysisType, promptVersion, objective string, messages []SourceMessage) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "promptVersion=%s\n", promptVersion)
	builder.WriteString("来源消息（仅可引用其中的 evidenceMessageIds）：\n")
	for _, message := range messages {
		fmt.Fprintf(&builder, "- id=%s time=%s direction=%s sender=%s content=%s\n", message.ID, message.MessageTime.Format(time.RFC3339), message.Direction, message.SenderName, message.Content)
	}
	if analysisType == AnalysisTypeSession {
		builder.WriteString("任务：输出会话分析，重点识别客户采购意向、流失风险与员工服务质量。\nJSON Schema：{\"schemaVersion\":1,\"summary\":string,\"customer\":{\"qualityLevel\":\"low|medium|high|insufficient\",\"qualityReason\":string,\"purchaseIntent\":{\"level\":\"low|medium|high|insufficient\",\"score\":0-100|null,\"reason\":string,\"evidenceMessageIds\":string[]},\"churnRisk\":{...},\"keywords\":string[],\"explicitNeeds\":string[],\"implicitNeeds\":string[],\"emotion\":{\"label\":\"positive|neutral|negative|mixed|unknown\",\"reason\":string,\"evidenceMessageIds\":string[]},\"recommendedReply\":string,\"actions\":string[],\"notes\":string[]},\"employeeQa\":{\"score\":0-100,\"dimensions\":object[],\"strengths\":string[],\"issues\":string[],\"suggestions\":string[]}}\n")
	} else {
		fmt.Fprintf(&builder, "任务：%s\n", objective)
		builder.WriteString("JSON Schema：{\"schemaVersion\":1,\"conclusion\":string,\"matched\":boolean,\"confidence\":0-1|null,\"evidenceMessageIds\":string[],\"recommendations\":string[]}\n")
	}
	return builder.String()
}

func ruleObjective(rule *AnalysisRuleVersion) string {
	if rule == nil {
		return "识别客户意向、沟通质量和跟进动作"
	}
	return rule.Objective
}
func ruleAllowsCandidate(rule AnalysisRuleVersion, candidate ConversationCandidate) bool {
	if len(rule.ConversationTypes) == 0 {
		return true
	}
	target := "direct"
	if candidate.TargetType == "2" {
		target = "group"
	}
	for _, kind := range rule.ConversationTypes {
		if kind == target {
			return true
		}
	}
	return false
}
func timePtr(value time.Time) *time.Time { return &value }
