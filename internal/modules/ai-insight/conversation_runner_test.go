package aiinsight

import (
	"strings"
	"testing"
	"time"
)

func TestConversationRunnerPromptContainsSourceContract(t *testing.T) {
	prompt := buildConversationPrompt(AnalysisTypeSession, "session-v1", "请分析客户采购意向", []SourceMessage{{ID: "msg:1", Direction: "inbound", MessageTime: time.Date(2026, 8, 21, 9, 0, 0, 0, time.UTC), SenderName: "客户", Content: "想了解价格"}})
	for _, fragment := range []string{"msg:1", "inbound", "采购意向", "schemaVersion", "evidenceMessageIds"} {
		if !strings.Contains(prompt, fragment) {
			t.Fatalf("prompt missing %q: %s", fragment, prompt)
		}
	}
}

func TestConversationRunnerConfigDefaults(t *testing.T) {
	config := normalizeRunnerConfig(RunnerConfig{})
	if config.BatchLimit != 200 || config.Concurrency != 2 || config.SessionDays != 30 || config.SessionLimit != 200 {
		t.Fatalf("defaults = %#v", config)
	}
}
