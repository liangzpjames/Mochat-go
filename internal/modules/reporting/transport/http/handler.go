package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/modules/reporting"
)

const ReportsPath = "/dashboard/reports/{kind}"

type Principal struct {
	UserID                  int64
	TenantID                int64
	CorpID                  int64
	WorkEmployeeID          int64
	AllowedEmployeeIDs      []int64
	EmployeeScopeRestricted bool
}

type PrincipalResolver interface {
	Resolve(*http.Request) (Principal, error)
}
type Authorizer interface {
	Authorize(context.Context, Principal, int64, string) error
}
type ReportService interface {
	Query(context.Context, reporting.ReportKind, reporting.ReportQuery) (reporting.ReportResult, error)
}

type Handler struct {
	service    ReportService
	resolver   PrincipalResolver
	authorizer Authorizer
}

func NewHandler(service ReportService, resolver PrincipalResolver, authorizer Authorizer) *Handler {
	return &Handler{service: service, resolver: resolver, authorizer: authorizer}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	kindValue := r.PathValue("kind")
	if kindValue == "" {
		kindValue = strings.TrimPrefix(r.URL.Path, "/dashboard/reports/")
	}
	kind, ok := reporting.ParseKind(kindValue)
	if !ok {
		write(w, http.StatusUnprocessableEntity, "unknown report kind", nil)
		return
	}
	if kind == reporting.BehaviorReport {
		if event := r.URL.Query().Get("eventType"); event != "" && event != "lead.created" && event != "contact.updated" && event != "order.created" {
			write(w, http.StatusUnprocessableEntity, "unknown behavior event type", nil)
			return
		}
	}
	principal, err := h.resolver.Resolve(r)
	if err != nil || principal.UserID <= 0 || principal.TenantID <= 0 || principal.CorpID <= 0 {
		write(w, http.StatusUnauthorized, "unauthorized", nil)
		return
	}
	query, err := parseQuery(r, principal)
	if err != nil {
		write(w, http.StatusBadRequest, "invalid report query", nil)
		return
	}
	permission := "/data/" + string(kind) + "#get"
	if kind == reporting.OverviewReport {
		permission = "/dashboard/corpData/index#get"
	}
	if h.authorizer != nil {
		if err := h.authorizer.Authorize(r.Context(), principal, query.CorpID, permission); err != nil {
			write(w, http.StatusForbidden, "forbidden", nil)
			return
		}
	}
	result, err := h.service.Query(r.Context(), kind, query)
	if err != nil {
		if errors.Is(err, reporting.ErrInvalidQuery) {
			write(w, http.StatusUnprocessableEntity, err.Error(), nil)
		} else {
			write(w, http.StatusInternalServerError, "report query failed", nil)
		}
		return
	}
	write(w, http.StatusOK, "success", result)
}

func parseQuery(r *http.Request, principal Principal) (reporting.ReportQuery, error) {
	values := r.URL.Query()
	corpID := principal.CorpID
	if raw := values.Get("corpId"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 || parsed != principal.CorpID {
			return reporting.ReportQuery{}, errors.New("corpId does not match dashboard principal")
		}
		corpID = parsed
	}
	startAt, err := time.Parse(time.RFC3339, values.Get("startAt"))
	if err != nil {
		return reporting.ReportQuery{}, err
	}
	endAt, err := time.Parse(time.RFC3339, values.Get("endAt"))
	if err != nil {
		return reporting.ReportQuery{}, err
	}
	var trendStartAt, trendEndAt *time.Time
	if raw := values.Get("trendStartAt"); raw != "" {
		parsed, parseErr := time.Parse(time.RFC3339, raw)
		if parseErr != nil {
			return reporting.ReportQuery{}, parseErr
		}
		trendStartAt = &parsed
	}
	if raw := values.Get("trendEndAt"); raw != "" {
		parsed, parseErr := time.Parse(time.RFC3339, raw)
		if parseErr != nil {
			return reporting.ReportQuery{}, parseErr
		}
		trendEndAt = &parsed
	}
	page, pageSize := parsePositive(values.Get("page"), 1), parsePositive(values.Get("pageSize"), 20)
	employeeIDs := parseIDs(values["employeeIds"])
	if len(employeeIDs) == 0 && values.Get("employeeId") != "" {
		employeeIDs = parseIDs([]string{values.Get("employeeId")})
	}
	departmentIDs := parseIDs(values["departmentIds"])
	if len(departmentIDs) == 0 && values.Get("departmentId") != "" {
		departmentIDs = parseIDs([]string{values.Get("departmentId")})
	}
	return reporting.ReportQuery{TenantID: principal.TenantID, CorpID: corpID, Timezone: values.Get("timezone"), StartAt: startAt, EndAt: endAt, TrendStartAt: trendStartAt, TrendEndAt: trendEndAt, DepartmentIDs: departmentIDs, EmployeeIDs: employeeIDs, AllowedEmployeeIDs: principal.AllowedEmployeeIDs, EmployeeScopeRestricted: principal.EmployeeScopeRestricted, Stage: values.Get("stage"), Page: page, PageSize: pageSize}, nil
}

func parsePositive(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
func parseIDs(values []string) []int64 {
	var result []int64
	for _, value := range values {
		for _, item := range strings.Split(value, ",") {
			if id, err := strconv.ParseInt(strings.TrimSpace(item), 10, 64); err == nil && id > 0 {
				result = append(result, id)
			}
		}
	}
	return result
}

func write(w http.ResponseWriter, status int, message string, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// Dashboard ApiClient validates the shared {code,msg,data} envelope.
	_ = json.NewEncoder(w).Encode(map[string]any{"code": status, "msg": message, "data": data})
}
