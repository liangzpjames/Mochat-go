package reporting

import "time"

type ReportKind string

const (
	CustomerReport   ReportKind = "customer"
	EmployeeReport   ReportKind = "employee"
	ConversionReport ReportKind = "conversion"
	BehaviorReport   ReportKind = "behavior"
	DetailReport     ReportKind = "report"
	OverviewReport   ReportKind = "overview"
)

func ParseKind(value string) (ReportKind, bool) {
	kind := ReportKind(value)
	switch kind {
	case CustomerReport, EmployeeReport, ConversionReport, BehaviorReport, DetailReport, OverviewReport:
		return kind, true
	default:
		return "", false
	}
}

type ReportQuery struct {
	TenantID                int64
	CorpID                  int64
	Timezone                string
	StartAt                 time.Time
	EndAt                   time.Time
	TrendStartAt            *time.Time
	TrendEndAt              *time.Time
	DepartmentIDs           []int64
	EmployeeIDs             []int64
	AllowedEmployeeIDs      []int64
	EmployeeScopeRestricted bool
	Stage                   string
	Page                    int
	PageSize                int
}

type SeriesPoint struct {
	At    time.Time `json:"at"`
	Value float64   `json:"value"`
}

type Dimension struct {
	Key   string  `json:"key"`
	Label string  `json:"label"`
	Value float64 `json:"value"`
}

type Pagination struct {
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
	Total    int `json:"total"`
}

type Freshness struct {
	DataThrough time.Time `json:"dataThrough"`
	Provider    string    `json:"provider"`
	Status      string    `json:"status"`
}

type Limitation struct {
	Provider string `json:"provider"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

type ReportResult struct {
	Summary         map[string]*float64          `json:"summary"`
	Series          []SeriesPoint                `json:"series"`
	Dimensions      []Dimension                  `json:"dimensions"`
	Items           []map[string]any             `json:"items"`
	Pagination      Pagination                   `json:"pagination"`
	Freshness       Freshness                    `json:"freshness"`
	Limitations     []Limitation                 `json:"limitations"`
	AIInsight       *AIInsightSummary            `json:"aiInsight,omitempty"`
	AIMetrics       *AIMetrics                   `json:"aiMetrics,omitempty"`
	Conversation    *ConversationStats           `json:"conversation,omitempty"`
	Quality         *QualityStats                `json:"quality,omitempty"`
	EmployeeRanking []EmployeeRankingItem        `json:"employeeRanking,omitempty"`
	Trajectory      []ConversationTrajectoryItem `json:"trajectory,omitempty"`
}

type AIInsightSummary struct {
	Capability  string `json:"capability"`
	Provider    string `json:"provider"`
	Summary     string `json:"summary"`
	GeneratedAt string `json:"generatedAt"`
}

type AIMetrics struct {
	AnalysisCount           *int     `json:"analysisCount"`
	CustomerNegativeEmotion *int     `json:"customerNegativeEmotion"`
	AverageEmployeeScore    *float64 `json:"averageEmployeeScore"`
	KeywordCount            *int     `json:"keywordCount"`
	AnalyzedEmployeeCount   *int     `json:"analyzedEmployeeCount"`
	AnalyzedCustomerCount   *int     `json:"analyzedCustomerCount"`
}

type QualityStats struct {
	SensitiveWords *int                `json:"sensitiveWords"`
	RiskBehavior   *int                `json:"riskBehavior"`
	CustomerLoss   *int                `json:"customerLoss"`
	TimeoutWarning *int                `json:"timeoutWarning"`
	Trend          []QualityTrendPoint `json:"trend"`
}

type QualityTrendPoint struct {
	Date           string `json:"date"`
	SensitiveWords *int   `json:"sensitiveWords"`
	RiskBehavior   *int   `json:"riskBehavior"`
	CustomerLoss   *int   `json:"customerLoss"`
	TimeoutWarning *int   `json:"timeoutWarning"`
}

type EmployeeRankingItem struct {
	EmployeeID   int64  `json:"employeeId"`
	EmployeeName string `json:"employeeName"`
	Sessions     int    `json:"sessions"`
	Messages     int    `json:"messages"`
}

type ConversationTrajectoryItem struct {
	ID           string `json:"id"`
	TargetType   string `json:"targetType"`
	TargetID     string `json:"targetId"`
	EmployeeName string `json:"employeeName"`
	MessageCount int    `json:"messageCount"`
	LatestAt     string `json:"latestAt"`
}

type ConversationGroupStats struct {
	Sessions         int `json:"sessions"`
	EmployeeMessages int `json:"employeeMessages"`
	CustomerMessages int `json:"customerMessages"`
}

type ConversationTrendPoint struct {
	Date                     string `json:"date"`
	CustomerSessions         int    `json:"customerSessions"`
	CustomerEmployeeMessages int    `json:"customerEmployeeMessages"`
	CustomerCustomerMessages int    `json:"customerCustomerMessages"`
	RoomSessions             int    `json:"roomSessions"`
	RoomEmployeeMessages     int    `json:"roomEmployeeMessages"`
	RoomCustomerMessages     int    `json:"roomCustomerMessages"`
}

type ConversationStats struct {
	Customer ConversationGroupStats   `json:"customer"`
	Room     ConversationGroupStats   `json:"room"`
	Trend    []ConversationTrendPoint `json:"trend"`
}

func Ratio(numerator, denominator float64) *float64 {
	if denominator == 0 {
		return nil
	}
	value := numerator / denominator
	return &value
}
