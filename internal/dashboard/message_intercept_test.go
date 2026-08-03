package dashboard

import "testing"

func TestMatchKeywords(t *testing.T) {
	got := MatchKeywords("请联系微信 ABC", "contains", []string{"微信", "abc", "无"})
	if len(got) != 2 {
		t.Fatalf("got %#v", got)
	}
	if len(MatchKeywords("微信客服", "exact", []string{"微信"})) != 0 {
		t.Fatal("exact should not partially match")
	}
}
func TestValidateMessageInterceptRuleRequiresPublishedVersion(t *testing.T) {
	if ValidateMessageInterceptRule(MessageInterceptRule{Name: "规则", LibraryID: 1, LibraryVersion: 0, Decision: "blocked", Status: "enabled", ConversationScopes: []string{"single"}}) == nil {
		t.Fatal("expected version error")
	}
}
