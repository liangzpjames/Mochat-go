package store

import (
	"os"
	"strings"
	"testing"
)

func TestEmployeeApplyAckUsesDedicatedAtomicIdempotencyCleanup(t *testing.T) {
	sourceBytes, err := os.ReadFile("redis.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	ackStart := strings.Index(source, "func (s *RedisStore) AckEmployeeApply")
	if ackStart < 0 {
		t.Fatal("AckEmployeeApply implementation not found")
	}
	ackEnd := strings.Index(source[ackStart:], "func (s *RedisStore) RetryEmployeeApply")
	if ackEnd < 0 {
		t.Fatal("AckEmployeeApply boundary not found")
	}
	ackSection := source[ackStart : ackStart+ackEnd]
	if !strings.Contains(ackSection, "ackEmployeeApplyQueueItem") {
		t.Fatalf("employee ack does not use a dedicated cleanup helper:\n%s", ackSection)
	}
	if strings.Contains(ackSection, "ackReliableQueueItem(") {
		t.Fatalf("employee ack still uses generic completion semantics:\n%s", ackSection)
	}

	helperStart := strings.Index(source, "func (s *RedisStore) ackEmployeeApplyQueueItem")
	if helperStart < 0 {
		t.Fatal("employee ack cleanup helper not found")
	}
	helperEnd := strings.Index(source[helperStart:], "func (s *RedisStore) retryReliableQueueItem")
	if helperEnd < 0 {
		t.Fatal("employee ack cleanup helper boundary not found")
	}
	helperSection := source[helperStart : helperStart+helperEnd]
	for _, required := range []string{"IdempotencyKey", "Eval"} {
		if !strings.Contains(helperSection, required) {
			t.Errorf("employee ack helper missing %q:\n%s", required, helperSection)
		}
	}
	scriptStart := strings.Index(source, "const employeeApplyAckScript")
	if scriptStart < 0 {
		t.Fatal("employee ack script not found")
	}
	scriptSection := source[scriptStart:helperStart]
	for _, required := range []string{"LREM", "DEL", "removed > 0"} {
		if !strings.Contains(scriptSection, required) {
			t.Errorf("employee ack script missing %q:\n%s", required, scriptSection)
		}
	}
}

func TestEmployeeApplyAckKeepsOtherQueueCompletionSemantics(t *testing.T) {
	sourceBytes, err := os.ReadFile("redis.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	for _, method := range []string{"AckWeWorkCallback", "AckContactWelcome"} {
		start := strings.Index(source, "func (s *RedisStore) "+method)
		if start < 0 {
			t.Fatalf("%s implementation not found", method)
		}
		nextStart := start + len("func (s *RedisStore) ")
		end := strings.Index(source[nextStart:], "func (s *RedisStore) ")
		if end < 0 {
			end = len(source) - nextStart
		} else {
			end += nextStart - start
		}
		section := source[start : start+end]
		if !strings.Contains(section, "ackReliableQueueItem(") {
			t.Fatalf("%s was changed to employee-specific completion semantics:\n%s", method, section)
		}
	}
}
