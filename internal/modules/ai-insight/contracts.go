package aiinsight

import (
	"context"
	"encoding/json"
	"time"
)

// AnalysisType identifies one of the two conversation-scoped AI workspaces.
type AnalysisType string

const (
	AnalysisTypeSession               AnalysisType = "session"
	AnalysisTypeSmart                 AnalysisType = "smart"
	DefaultSmartAnalysisRuleSystemKey              = "default-smart-analysis"
)

type AnalysisStatus string

const (
	AnalysisStatusPending   AnalysisStatus = "pending"
	AnalysisStatusRunning   AnalysisStatus = "running"
	AnalysisStatusSucceeded AnalysisStatus = "succeeded"
	AnalysisStatusFailed    AnalysisStatus = "failed"
)

// EvidenceAssessment is a bounded assessment whose evidence must refer to
// source messages supplied in the same model request.
type EvidenceAssessment struct {
	Level              string                `json:"level"`
	Score              *int                  `json:"score"`
	Reason             string                `json:"reason"`
	EvidenceMessageIDs []string              `json:"evidenceMessageIds"`
	Dimensions         []QuantifiedDimension `json:"dimensions,omitempty"`
}

type QuantifiedDimension struct {
	Name               string   `json:"name"`
	Weight             float64  `json:"weight"`
	Score              *float64 `json:"score"`
	Reason             string   `json:"reason"`
	EvidenceMessageIDs []string `json:"evidenceMessageIds"`
}

type CustomerEmotion struct {
	Label              string   `json:"label"`
	Reason             string   `json:"reason"`
	EvidenceMessageIDs []string `json:"evidenceMessageIds"`
}

type CustomerAnalysis struct {
	QualityLevel     string             `json:"qualityLevel"`
	QualityScore     *float64           `json:"qualityScore,omitempty"`
	QualityReason    string             `json:"qualityReason"`
	PurchaseIntent   EvidenceAssessment `json:"purchaseIntent"`
	ChurnRisk        EvidenceAssessment `json:"churnRisk"`
	Keywords         []string           `json:"keywords"`
	ExplicitNeeds    []string           `json:"explicitNeeds"`
	ImplicitNeeds    []string           `json:"implicitNeeds"`
	Emotion          CustomerEmotion    `json:"emotion"`
	RecommendedReply string             `json:"recommendedReply"`
	Actions          []string           `json:"actions"`
	Notes            []string           `json:"notes"`
}

type UnresolvedIssue struct {
	Title              string   `json:"title"`
	Reason             string   `json:"reason"`
	EvidenceMessageIDs []string `json:"evidenceMessageIds"`
}

type EmployeeQADimension struct {
	Name    string `json:"name"`
	Score   int    `json:"score"`
	Comment string `json:"comment"`
}

type EmployeeQA struct {
	Score                    int                   `json:"score"`
	Dimensions               []EmployeeQADimension `json:"dimensions"`
	Strengths                []string              `json:"strengths"`
	Issues                   []string              `json:"issues"`
	Suggestions              []string              `json:"suggestions"`
	UnresolvedCustomerIssues []UnresolvedIssue     `json:"unresolvedCustomerIssues,omitempty"`
	UnresolvedObjections     []UnresolvedIssue     `json:"unresolvedObjections,omitempty"`
}

type SessionAnalysisResult struct {
	SchemaVersion int              `json:"schemaVersion"`
	Summary       string           `json:"summary"`
	Customer      CustomerAnalysis `json:"customer"`
	EmployeeQA    EmployeeQA       `json:"employeeQa"`
}

func (result SessionAnalysisResult) MarshalJSON() ([]byte, error) {
	type wire SessionAnalysisResult
	encoded, err := json.Marshal(wire(result))
	if err != nil {
		return nil, err
	}
	var root map[string]any
	if err := json.Unmarshal(encoded, &root); err != nil {
		return nil, err
	}
	customer, _ := root["customer"].(map[string]any)
	employeeQA, _ := root["employeeQa"].(map[string]any)
	if result.SchemaVersion == insightSchemaVersionV1 {
		delete(customer, "qualityScore")
		for _, name := range []string{"purchaseIntent", "churnRisk"} {
			assessment, _ := customer[name].(map[string]any)
			delete(assessment, "dimensions")
		}
		delete(employeeQA, "unresolvedCustomerIssues")
		delete(employeeQA, "unresolvedObjections")
	} else if result.SchemaVersion == insightSchemaVersionV2 {
		customer["qualityScore"] = result.Customer.QualityScore
	}
	return json.Marshal(root)
}

type SmartAnalysisResult struct {
	SchemaVersion         int                   `json:"schemaVersion"`
	Conclusion            string                `json:"conclusion"`
	Matched               bool                  `json:"matched"`
	Confidence            *float64              `json:"confidence,omitempty"`
	MatchScore            *float64              `json:"matchScore,omitempty"`
	ConfidenceScore       *float64              `json:"confidenceScore,omitempty"`
	EvidenceCoverageScore *float64              `json:"evidenceCoverageScore,omitempty"`
	PriorityScore         *float64              `json:"priorityScore,omitempty"`
	PriorityLevel         string                `json:"priorityLevel,omitempty"`
	Dimensions            []QuantifiedDimension `json:"dimensions,omitempty"`
	EvidenceMessageIDs    []string              `json:"evidenceMessageIds"`
	Recommendations       []string              `json:"recommendations"`
}

func (result SmartAnalysisResult) MarshalJSON() ([]byte, error) {
	type wire SmartAnalysisResult
	encoded, err := json.Marshal(wire(result))
	if err != nil {
		return nil, err
	}
	var root map[string]any
	if err := json.Unmarshal(encoded, &root); err != nil {
		return nil, err
	}
	if result.SchemaVersion == insightSchemaVersionV1 {
		for _, name := range []string{"matchScore", "confidenceScore", "evidenceCoverageScore", "priorityScore", "priorityLevel", "dimensions"} {
			delete(root, name)
		}
		root["confidence"] = result.Confidence
	} else if result.SchemaVersion == insightSchemaVersionV2 {
		delete(root, "confidence")
		root["matchScore"] = result.MatchScore
		root["confidenceScore"] = result.ConfidenceScore
		root["evidenceCoverageScore"] = result.EvidenceCoverageScore
		root["priorityScore"] = result.PriorityScore
		root["dimensions"] = result.Dimensions
	}
	return json.Marshal(root)
}

type SourceMessage struct {
	ID              string
	ConversationKey string
	MessageTime     time.Time
	Direction       string
	SenderID        string
	SenderName      string
	Content         string
	TableIndex      int
	Sequence        int64
}

type ConversationCandidate struct {
	ConversationKey    string
	EmployeeID         int64
	EmployeeName       string
	EmployeeAvatar     string
	TargetType         string
	TargetID           string
	TargetName         string
	TargetAvatar       string
	SourceStartedAt    time.Time
	SourceEndedAt      time.Time
	SourceMessageCount int
	SourceFingerprint  string
}

type ConversationWindowQuery struct {
	TenantID           int64
	CorpID             int64
	ConversationKey    string
	StartAt            time.Time
	EndAt              time.Time
	Limit              int
	AllowedEmployeeIDs []int64
	Restricted         bool
}

type CandidateQuery struct {
	TenantID           int64
	CorpID             int64
	StartAt            time.Time
	EndAt              time.Time
	Limit              int
	AnalysisType       AnalysisType
	RuleVersionID      int64
	AllowedEmployeeIDs []int64
	Restricted         bool
}

type ConversationInsight struct {
	ID                 int64
	TenantID           int64
	CorpID             int64
	AnalysisType       AnalysisType
	RuleID             int64
	RuleVersionID      int64
	RuleNameSnapshot   string
	RuleVersion        int
	ConversationKey    string
	EmployeeID         int64
	EmployeeName       string
	EmployeeAvatar     string
	TargetType         string
	TargetID           string
	TargetName         string
	TargetAvatar       string
	SourceStartedAt    time.Time
	SourceEndedAt      time.Time
	SourceMessageCount int
	SourceFingerprint  string
	Status             AnalysisStatus
	Summary            string
	SessionResult      *SessionAnalysisResult
	SmartResult        *SmartAnalysisResult
	ResultJSON         []byte
	ErrorSummary       string
	Provider           string
	Model              string
	PromptVersion      string
	GeneratedAt        *time.Time
	CreatedAt          time.Time
}

type AnalysisRule struct {
	ID                int64
	TenantID          int64
	CorpID            int64
	Name              string
	Objective         string
	ConversationTypes []string
	TargetScope       string
	TargetIDs         []int64
	LookbackDays      int
	MinimumMessages   int
	Status            string
	CurrentVersion    int
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         *time.Time
}

type AnalysisRuleVersion struct {
	ID                     int64
	TenantID               int64
	CorpID                 int64
	RuleID                 int64
	Version                int
	Name                   string
	Objective              string
	CustomerAnalysisPrompt string
	EmployeeQAPrompt       string
	ConversationTypes      []string
	TargetScope            string
	TargetIDs              []int64
	LookbackDays           int
	MinimumMessages        int
	CreatedAt              time.Time
}

type InsightRun struct {
	ID             int64
	TenantID       int64
	CorpID         int64
	AnalysisType   AnalysisType
	RuleVersionID  int64
	Status         AnalysisStatus
	PlannedAt      *time.Time
	StartedAt      *time.Time
	FinishedAt     *time.Time
	CandidateCount int
	SuccessCount   int
	FailureCount   int
	BacklogCount   int
	ErrorSummary   string
	CreatedAt      time.Time
}

type InsightRunResult struct {
	Status         AnalysisStatus
	CandidateCount int
	SuccessCount   int
	FailureCount   int
	BacklogCount   int
	ErrorSummary   string
	FinishedAt     time.Time
}

type InsightFilter struct {
	TenantID           int64
	CorpID             int64
	AnalysisType       AnalysisType
	Page               int
	PageSize           int
	EmployeeID         int64
	ConversationType   string
	TargetID           string
	RuleVersionID      int64
	Status             AnalysisStatus
	Keyword            string
	CustomerName       string
	StartAt            *time.Time
	EndAt              *time.Time
	AllowedEmployeeIDs []int64
	Restricted         bool
}

type EmployeeOptionFilter struct {
	TenantID           int64
	CorpID             int64
	AnalysisType       AnalysisType
	EmployeeKeyword    string
	Limit              int
	AllowedEmployeeIDs []int64
	Restricted         bool
}

type EmployeeOption struct {
	ID     int64
	Name   string
	Avatar string
}

type InsightPage struct {
	Items       []ConversationInsight
	Page        int
	PageSize    int
	Total       int
	GeneratedAt *time.Time
}

type InsightDetailFilter struct {
	TenantID           int64
	CorpID             int64
	AnalysisType       AnalysisType
	ID                 int64
	AllowedEmployeeIDs []int64
	Restricted         bool
}

type RuleFilter struct {
	TenantID int64
	CorpID   int64
	Page     int
	PageSize int
	Status   string
	Keyword  string
}

type RulePage struct {
	Items    []AnalysisRule
	Page     int
	PageSize int
	Total    int
}

type RuleWrite struct {
	TenantID          int64
	CorpID            int64
	ID                int64
	Name              string
	Objective         string
	ConversationTypes []string
	TargetScope       string
	TargetIDs         []int64
	LookbackDays      int
	MinimumMessages   int
	Status            string
	ActorID           int64
}

type RuleStatusWrite struct {
	TenantID int64
	CorpID   int64
	ID       int64
	Status   string
	ActorID  int64
}

type RuleDelete struct {
	TenantID int64
	CorpID   int64
	ID       int64
	ActorID  int64
}

type Repository interface {
	ConversationCandidates(context.Context, CandidateQuery) ([]ConversationCandidate, error)
	ConversationMessages(context.Context, ConversationWindowQuery) ([]SourceMessage, error)
	LatestSucceededFingerprint(context.Context, int64, int64, AnalysisType, int64, string) (string, error)
	SaveInsight(context.Context, ConversationInsight) error
	CreateRun(context.Context, InsightRun) (int64, error)
	FinishRun(context.Context, int64, InsightRunResult) error
	InsightPage(context.Context, InsightFilter) (InsightPage, error)
	EmployeeOptions(context.Context, EmployeeOptionFilter) ([]EmployeeOption, error)
	InsightDetail(context.Context, InsightDetailFilter) (ConversationInsight, error)
	LatestRun(context.Context, int64, int64, AnalysisType) (*InsightRun, error)
	RulePage(context.Context, RuleFilter) (RulePage, error)
	RuleByID(context.Context, int64, int64, int64) (AnalysisRule, error)
	CreateRule(context.Context, RuleWrite) (AnalysisRule, error)
	UpdateRule(context.Context, RuleWrite) (AnalysisRule, error)
	SetRuleStatus(context.Context, RuleStatusWrite) error
	DeleteRule(context.Context, RuleDelete) error
	EnabledRuleVersions(context.Context, int64, int64) ([]AnalysisRuleVersion, error)
	CurrentEnabledRuleVersion(context.Context, int64, int64, string) (*AnalysisRuleVersion, error)
}
