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
	Summary     map[string]*float64 `json:"summary"`
	Series      []SeriesPoint       `json:"series"`
	Dimensions  []Dimension         `json:"dimensions"`
	Items       []map[string]any    `json:"items"`
	Pagination  Pagination          `json:"pagination"`
	Freshness   Freshness           `json:"freshness"`
	Limitations []Limitation        `json:"limitations"`
}

func Ratio(numerator, denominator float64) *float64 {
	if denominator == 0 {
		return nil
	}
	value := numerator / denominator
	return &value
}
