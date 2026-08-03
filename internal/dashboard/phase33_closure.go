package dashboard

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type SilentCustomerRule struct {
	ID           int64  `json:"id"`
	TenantID     int64  `json:"tenantId"`
	CorpID       int64  `json:"corpId"`
	Name         string `json:"name"`
	SilentDays   int    `json:"silentDays"`
	Status       string `json:"status"`
	TriggerCount int64  `json:"triggerCount"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}
type SilentCustomerRecord struct {
	ID                 int64  `json:"id"`
	RuleID             int64  `json:"ruleId"`
	RuleName           string `json:"ruleName"`
	CustomerID         string `json:"customerId"`
	CustomerName       string `json:"customerName"`
	EmployeeID         int64  `json:"employeeId"`
	EmployeeName       string `json:"employeeName"`
	LastInteractionAt  string `json:"lastInteractionAt"`
	SilentDays         int    `json:"silentDays"`
	Status             string `json:"status"`
	AssignedEmployeeID int64  `json:"assignedEmployeeId"`
	FollowUpNote       string `json:"followUpNote"`
	UpdatedAt          string `json:"updatedAt"`
}
type SilentCustomerActivity struct {
	CustomerID        string `json:"customerId"`
	CustomerName      string `json:"customerName"`
	EmployeeID        int64  `json:"employeeId"`
	EmployeeName      string `json:"employeeName"`
	LastInteractionAt string `json:"lastInteractionAt"`
}
type RefuseArchiveRecord struct {
	ID                  int64  `json:"id"`
	SubjectType         string `json:"subjectType"`
	SubjectID           string `json:"subjectId"`
	SubjectName         string `json:"subjectName"`
	EmployeeID          int64  `json:"employeeId"`
	EmployeeName        string `json:"employeeName"`
	AuthorizationStatus string `json:"authorizationStatus"`
	Source              string `json:"source"`
	RefusedAt           string `json:"refusedAt"`
	AuthorizedAt        string `json:"authorizedAt"`
	LastFollowUpAt      string `json:"lastFollowUpAt"`
	FollowUpStatus      string `json:"followUpStatus"`
	FollowUpNote        string `json:"followUpNote"`
	UpdatedAt           string `json:"updatedAt"`
}
type SilentRuleFilter struct {
	TenantID, CorpID int
	Name, Status     string
	Page, PerPage    int
}
type SilentRecordFilter struct {
	TenantID, CorpID int
	Customer, Status string
	RuleID           int64
	Page, PerPage    int
}
type RefuseArchiveFilter struct {
	TenantID, CorpID                             int
	Subject, AuthorizationStatus, FollowUpStatus string
	Page, PerPage                                int
}
type SilentRulePage struct {
	Items                []SilentCustomerRule `json:"items"`
	Total, Page, PerPage int
}
type SilentRecordPage struct {
	Items                []SilentCustomerRecord `json:"items"`
	Total, Page, PerPage int
}
type RefuseArchivePage struct {
	Items                []RefuseArchiveRecord `json:"items"`
	Total, Page, PerPage int
}
type Phase33ClosureProvider interface {
	SilentRulePage(context.Context, SilentRuleFilter) (SilentRulePage, error)
	SilentRecordPage(context.Context, SilentRecordFilter) (SilentRecordPage, error)
	RefuseArchivePage(context.Context, RefuseArchiveFilter) (RefuseArchivePage, error)
}
type Phase33ClosureWriter interface {
	SaveSilentRule(context.Context, SilentCustomerRule) (int64, error)
	SetSilentRuleStatus(context.Context, int, int, int64, string) (bool, error)
	DeleteSilentRule(context.Context, int, int, int64) (bool, error)
	EvaluateSilentCustomer(context.Context, int, int, SilentCustomerActivity) (int64, error)
	ActSilentRecords(context.Context, int, int, int64, []int64, string, int64, string) (int64, error)
	UpsertRefuseArchive(context.Context, int, int, int64, RefuseArchiveRecord) (int64, error)
	FollowUpRefuseArchive(context.Context, int, int, int64, int64, string, string) (bool, error)
}

func ValidateSilentRule(v SilentCustomerRule) error {
	if strings.TrimSpace(v.Name) == "" || len([]rune(v.Name)) > 80 {
		return fmt.Errorf("规则名称不能为空且不能超过 80 个字符")
	}
	if v.SilentDays < 1 || v.SilentDays > 365 {
		return fmt.Errorf("沉默天数必须为 1 到 365 天")
	}
	if v.Status != "enabled" && v.Status != "disabled" {
		return fmt.Errorf("规则状态无效")
	}
	return nil
}
func ValidateSilentActivity(v SilentCustomerActivity) error {
	if strings.TrimSpace(v.CustomerID) == "" {
		return fmt.Errorf("客户 ID 不能为空")
	}
	if _, e := time.Parse(time.RFC3339, v.LastInteractionAt); e != nil {
		return fmt.Errorf("最后互动时间无效")
	}
	return nil
}
func ValidateRefuseArchive(v RefuseArchiveRecord) error {
	if v.SubjectType != "customer" && v.SubjectType != "employee" && v.SubjectType != "group_member" {
		return fmt.Errorf("主体类型无效")
	}
	if strings.TrimSpace(v.SubjectID) == "" {
		return fmt.Errorf("主体 ID 不能为空")
	}
	if v.AuthorizationStatus != "refused" && v.AuthorizationStatus != "authorized" && v.AuthorizationStatus != "pending" && v.AuthorizationStatus != "expired" {
		return fmt.Errorf("授权状态无效")
	}
	return nil
}
