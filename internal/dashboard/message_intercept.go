package dashboard

import (
	"context"
	"fmt"
	"strings"
)

type KeywordLibrary struct {
	ID               int64  `json:"id"`
	TenantID         int64  `json:"tenantId"`
	CorpID           int64  `json:"corpId"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	MatchMode        string `json:"matchMode"`
	Status           string `json:"status"`
	DraftVersion     int    `json:"draftVersion"`
	PublishedVersion int    `json:"publishedVersion"`
	EntryCount       int64  `json:"entryCount"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
}
type KeywordEntry struct {
	ID        int64  `json:"id"`
	LibraryID int64  `json:"libraryId"`
	Keyword   string `json:"keyword"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}
type KeywordLibraryFilter struct {
	TenantID, CorpID int
	Name, Status     string
	Page, PerPage    int
}
type KeywordEntryFilter struct {
	TenantID, CorpID int
	LibraryID        int64
	Keyword, Status  string
	Page, PerPage    int
}
type KeywordLibraryPage struct {
	Items   []KeywordLibrary `json:"items"`
	Total   int              `json:"total"`
	Page    int              `json:"page"`
	PerPage int              `json:"perPage"`
}
type KeywordEntryPage struct {
	Items   []KeywordEntry `json:"items"`
	Total   int            `json:"total"`
	Page    int            `json:"page"`
	PerPage int            `json:"perPage"`
}

type MessageInterceptRule struct {
	ID                 int64    `json:"id"`
	TenantID           int64    `json:"tenantId"`
	CorpID             int64    `json:"corpId"`
	Name               string   `json:"name"`
	LibraryID          int64    `json:"libraryId"`
	LibraryName        string   `json:"libraryName"`
	LibraryVersion     int      `json:"libraryVersion"`
	ConversationScopes []string `json:"conversationScopes"`
	Decision           string   `json:"decision"`
	Status             string   `json:"status"`
	TriggerCount       int64    `json:"triggerCount"`
	CreatedAt          string   `json:"createdAt"`
	UpdatedAt          string   `json:"updatedAt"`
}
type MessageInterceptRecord struct {
	ID               int64    `json:"id"`
	RuleID           int64    `json:"ruleId"`
	RuleName         string   `json:"ruleName"`
	LibraryID        int64    `json:"libraryId"`
	LibraryVersion   int      `json:"libraryVersion"`
	ConversationType string   `json:"conversationType"`
	ConversationID   string   `json:"conversationId"`
	MessageID        string   `json:"messageId"`
	SenderID         string   `json:"senderId"`
	SenderName       string   `json:"senderName"`
	MessageContent   string   `json:"messageContent"`
	MatchedKeywords  []string `json:"matchedKeywords"`
	Decision         string   `json:"decision"`
	Explanation      string   `json:"explanation"`
	AuditStatus      string   `json:"auditStatus"`
	OccurredAt       string   `json:"occurredAt"`
}
type MessageInterceptRuleFilter struct {
	TenantID, CorpID int
	Name, Status     string
	Page, PerPage    int
}
type MessageInterceptRecordFilter struct {
	TenantID, CorpID               int
	Keyword, Decision, AuditStatus string
	RuleID                         int64
	Page, PerPage                  int
}
type MessageInterceptRulePage struct {
	Items   []MessageInterceptRule `json:"items"`
	Total   int                    `json:"total"`
	Page    int                    `json:"page"`
	PerPage int                    `json:"perPage"`
}
type MessageInterceptRecordPage struct {
	Items   []MessageInterceptRecord `json:"items"`
	Total   int                      `json:"total"`
	Page    int                      `json:"page"`
	PerPage int                      `json:"perPage"`
}
type MessageInterceptEvaluation struct {
	ConversationType string `json:"conversationType"`
	ConversationID   string `json:"conversationId"`
	MessageID        string `json:"messageId"`
	SenderID         string `json:"senderId"`
	SenderName       string `json:"senderName"`
	Content          string `json:"content"`
	OccurredAt       string `json:"occurredAt"`
}
type MessageInterceptDecision struct {
	Decision        string   `json:"decision"`
	Matched         bool     `json:"matched"`
	RecordIDs       []int64  `json:"recordIds"`
	MatchedKeywords []string `json:"matchedKeywords"`
	Explanation     string   `json:"explanation"`
}

type MessageInterceptProvider interface {
	KeywordLibraryPage(context.Context, KeywordLibraryFilter) (KeywordLibraryPage, error)
	KeywordEntryPage(context.Context, KeywordEntryFilter) (KeywordEntryPage, error)
	MessageInterceptRulePage(context.Context, MessageInterceptRuleFilter) (MessageInterceptRulePage, error)
	MessageInterceptRecordPage(context.Context, MessageInterceptRecordFilter) (MessageInterceptRecordPage, error)
}
type KeywordLibraryWriter interface {
	SaveKeywordLibrary(context.Context, KeywordLibrary) (int64, error)
	SetKeywordLibraryStatus(context.Context, int, int, int64, string) (bool, error)
	DeleteKeywordLibrary(context.Context, int, int, int64) (bool, error)
	SaveKeywordEntry(context.Context, int, int, KeywordEntry) (int64, error)
	SetKeywordEntryStatus(context.Context, int, int, int64, string) (bool, error)
	DeleteKeywordEntry(context.Context, int, int, int64) (bool, error)
	PublishKeywordLibrary(context.Context, int, int, int64, int64) (int, error)
}
type MessageInterceptWriter interface {
	SaveMessageInterceptRule(context.Context, MessageInterceptRule) (int64, error)
	SetMessageInterceptRuleStatus(context.Context, int, int, int64, string) (bool, error)
	DeleteMessageInterceptRule(context.Context, int, int, int64) (bool, error)
	EvaluateMessageIntercept(context.Context, int, int, MessageInterceptEvaluation) (MessageInterceptDecision, error)
	AuditMessageInterceptRecords(context.Context, int, int, int64, []int64, string, string) (int64, error)
}

func ValidateKeywordLibrary(v KeywordLibrary) error {
	if strings.TrimSpace(v.Name) == "" || len([]rune(v.Name)) > 80 {
		return fmt.Errorf("词库名称不能为空且不能超过 80 个字符")
	}
	if v.MatchMode != "contains" && v.MatchMode != "exact" {
		return fmt.Errorf("匹配方式无效")
	}
	if v.Status != "enabled" && v.Status != "disabled" {
		return fmt.Errorf("词库状态无效")
	}
	return nil
}
func ValidateKeywordEntry(v KeywordEntry) error {
	if v.LibraryID <= 0 {
		return fmt.Errorf("请选择词库")
	}
	if strings.TrimSpace(v.Keyword) == "" || len([]rune(v.Keyword)) > 120 {
		return fmt.Errorf("关键词不能为空且不能超过 120 个字符")
	}
	if v.Status != "enabled" && v.Status != "disabled" {
		return fmt.Errorf("关键词状态无效")
	}
	return nil
}
func ValidateMessageInterceptRule(v MessageInterceptRule) error {
	if strings.TrimSpace(v.Name) == "" || len([]rune(v.Name)) > 80 {
		return fmt.Errorf("规则名称不能为空且不能超过 80 个字符")
	}
	if v.LibraryID <= 0 || v.LibraryVersion <= 0 {
		return fmt.Errorf("规则必须关联已发布词库版本")
	}
	if v.Decision != "blocked" && v.Decision != "review_required" && v.Decision != "allowed" {
		return fmt.Errorf("拦截决定无效")
	}
	if v.Status != "enabled" && v.Status != "disabled" {
		return fmt.Errorf("规则状态无效")
	}
	if len(v.ConversationScopes) == 0 {
		return fmt.Errorf("请选择会话范围")
	}
	for _, s := range v.ConversationScopes {
		if s != "single" && s != "group" {
			return fmt.Errorf("会话范围无效")
		}
	}
	return nil
}

func MatchKeywords(content, mode string, keywords []string) []string {
	content = strings.TrimSpace(content)
	out := []string{}
	seen := map[string]bool{}
	for _, k := range keywords {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		hit := false
		if mode == "exact" {
			hit = strings.EqualFold(content, k)
		} else {
			hit = strings.Contains(strings.ToLower(content), strings.ToLower(k))
		}
		if hit && !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	return out
}
