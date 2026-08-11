package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
)

const statisticMaxRange = 30 * 24 * time.Hour

type StatisticContactSummary struct {
	Total int
	Add   int
	Loss  int
	Net   int
}

type StatisticContactDay struct {
	Date  string
	Total int
	Add   int
	Loss  int
	Net   int
}

type StatisticTopEmployee struct {
	Name  string
	Total int
}

type StatisticEmployee struct {
	ID       int
	WXUserID string
	Name     string
	Avatar   string
}

type StatisticEmployeeMetric struct {
	ChatCnt         int
	MessageCnt      int
	ReplyPercentage float64
	AvgReplyTime    float64
}

type StatisticBehaviorData struct {
	StatTime            int64   `json:"stat_time"`
	ChatCnt             int     `json:"chat_cnt"`
	MessageCnt          int     `json:"message_cnt"`
	ReplyPercentage     float64 `json:"reply_percentage"`
	AvgReplyTime        float64 `json:"avg_reply_time"`
	NewApplyCnt         int     `json:"new_apply_cnt,omitempty"`
	NewContactCnt       int     `json:"new_contact_cnt,omitempty"`
	NegativeFeedbackCnt int     `json:"negative_feedback_cnt,omitempty"`
}

type StatisticStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	StatisticToday(ctx context.Context, corpID int, now time.Time) (StatisticContactSummary, error)
	StatisticContactTrend(ctx context.Context, corpID int, employeeIDs []int, start time.Time, end time.Time) ([]StatisticContactDay, error)
	StatisticTopEmployees(ctx context.Context, corpID int, limit int) (int, []StatisticTopEmployee, error)
	StatisticEmployeesByCorp(ctx context.Context, corpID int) ([]StatisticEmployee, error)
	StatisticEmployeesByIDs(ctx context.Context, corpID int, employeeIDs []int) ([]StatisticEmployee, error)
	StatisticEmployeeLocalSummary(ctx context.Context, corpID int, employeeIDs []int, start time.Time, end time.Time) (StatisticEmployeeMetric, error)
	StatisticEmployeeLocalMetricsByEmployee(ctx context.Context, corpID int, employeeIDs []int, start time.Time, end time.Time) (map[int]StatisticEmployeeMetric, error)
	StatisticEmployeeLocalTrend(ctx context.Context, corpID int, employeeIDs []int, start time.Time, end time.Time) ([]StatisticBehaviorData, error)
	RoomWelcomeCorpCredentialByID(ctx context.Context, corpID int) (RoomWelcomeCorpCredential, bool, error)
}

type StatisticBehaviorClient interface {
	UserBehavior(ctx context.Context, credential RoomWelcomeCorpCredential, userIDs []string, start time.Time, end time.Time) ([]StatisticBehaviorData, error)
}

type StatisticHandler struct {
	store      StatisticStore
	cache      LoginCache
	resolver   UserIDResolver
	authorizer CorpAdminAuthorizer
	apiBaseURL string
	client     StatisticBehaviorClient
	now        func() time.Time
}

func NewStatisticHandler(store StatisticStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer, apiBaseURL string, client StatisticBehaviorClient) *StatisticHandler {
	apiBaseURL = strings.TrimRight(strings.TrimSpace(apiBaseURL), "/")
	return &StatisticHandler{store: store, cache: cache, resolver: resolver, authorizer: authorizer, apiBaseURL: apiBaseURL, client: client}
}

func (h *StatisticHandler) Index(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, corpID, ok := h.resolveAuthorized(w, r, "/dashboard/statistic/index#get")
	if !ok {
		return
	}
	start, end, ok := h.queryDateRange(w, r, false)
	if !ok {
		return
	}
	mode := positiveQueryInt(r, "mode", 0)
	employeeIDs := statisticEmployeeIDsFromQuery(r, "employeeId")

	today, err := h.store.StatisticToday(r.Context(), corpID, h.currentTime())
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	days, err := h.store.StatisticContactTrend(r.Context(), corpID, employeeIDs, start, end)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	table := map[string]int{}
	series := make([]map[string]any, 0, len(days))
	for _, day := range days {
		series = append(series, map[string]any{
			"date":  day.Date,
			"total": day.Total,
			"add":   day.Add,
			"loss":  day.Loss,
			"net":   day.Net,
		})
		if value, ok := statisticContactModeValue(day, mode); ok {
			table[day.Date] = value
		}
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"today": map[string]any{
			"total": today.Total,
			"add":   today.Add,
			"loss":  today.Loss,
			"net":   today.Net,
		},
		"table": table,
		"any":   series,
	})
}

func (h *StatisticHandler) TopList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, corpID, ok := h.resolveAuthorized(w, r, "/dashboard/statistic/topList#get")
	if !ok {
		return
	}
	total, list, err := h.store.StatisticTopEmployees(r.Context(), corpID, 10)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, item := range list {
		out = append(out, map[string]any{"name": item.Name, "total": item.Total})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"total": total,
		"list":  out,
	})
}

func (h *StatisticHandler) Employees(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, corpID, ok := h.resolveAuthorized(w, r, "/dashboard/statistic/employees#get")
	if !ok {
		return
	}
	start, end, ok := h.queryDateRange(w, r, true)
	if !ok {
		return
	}
	employees, err := h.store.StatisticEmployeesByCorp(r.Context(), corpID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	metric, err := h.behaviorSummary(r.Context(), corpID, employees, start, end)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", statisticMetricPayload(metric))
}

func (h *StatisticHandler) EmployeeCounts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, corpID, ok := h.resolveAuthorized(w, r, "/dashboard/statistic/employeeCounts#get")
	if !ok {
		return
	}
	start, end, ok := h.queryDateRange(w, r, true)
	if !ok {
		return
	}
	page := positiveQueryInt(r, "page", 1)
	employees, err := h.store.StatisticEmployeesByCorp(r.Context(), corpID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	total := len(employees)
	paged := paginateStatisticEmployees(employees, page, 5)
	metrics, err := h.employeeMetrics(r.Context(), corpID, paged, start, end)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	table := make([]map[string]any, 0, len(paged))
	for _, employee := range paged {
		metric := metrics[employee.ID]
		item := statisticMetricPayload(metric)
		item["id"] = employee.ID
		item["name"] = employee.Name
		item["avatar"] = h.fileFullURL(employee.Avatar)
		table = append(table, item)
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"total": total,
		"table": table,
	})
}

func (h *StatisticHandler) EmployeesTrend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, corpID, ok := h.resolveAuthorized(w, r, "/dashboard/statistic/employeesTrend#get")
	if !ok {
		return
	}
	start, end, ok := h.queryDateRange(w, r, true)
	if !ok {
		return
	}
	mode := positiveQueryInt(r, "mode", 0)
	employeeIDs := statisticEmployeeIDsFromQuery(r, "employees")
	employees, err := h.statisticEmployees(r.Context(), corpID, employeeIDs)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	behavior, err := h.behaviorTrend(r.Context(), corpID, employees, start, end)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(behavior))
	table := map[string]any{}
	for _, item := range behavior {
		date := time.Unix(item.StatTime, 0).Format("2006-01-02")
		row := map[string]any{
			"date":             date,
			"chat_cnt":         item.ChatCnt,
			"message_cnt":      item.MessageCnt,
			"reply_percentage": roundFloat(item.ReplyPercentage, 2),
			"avg_reply_time":   roundFloat(item.AvgReplyTime, 2),
		}
		list = append(list, row)
		if value, ok := statisticBehaviorModeValue(row, mode); ok {
			table[date] = value
		}
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"list":  list,
		"table": table,
	})
}

func (h *StatisticHandler) behaviorSummary(ctx context.Context, corpID int, employees []StatisticEmployee, start time.Time, end time.Time) (StatisticEmployeeMetric, error) {
	behavior, ok := h.weComBehavior(ctx, corpID, employees, start, end)
	if ok {
		return summarizeStatisticBehavior(behavior), nil
	}
	employeeIDs := statisticEmployeeIDs(employees)
	return h.store.StatisticEmployeeLocalSummary(ctx, corpID, employeeIDs, start, end)
}

func (h *StatisticHandler) employeeMetrics(ctx context.Context, corpID int, employees []StatisticEmployee, start time.Time, end time.Time) (map[int]StatisticEmployeeMetric, error) {
	result := make(map[int]StatisticEmployeeMetric, len(employees))
	local, err := h.store.StatisticEmployeeLocalMetricsByEmployee(ctx, corpID, statisticEmployeeIDs(employees), start, end)
	if err != nil {
		return nil, err
	}
	for _, employee := range employees {
		result[employee.ID] = local[employee.ID]
		if behavior, ok := h.weComBehavior(ctx, corpID, []StatisticEmployee{employee}, start, end); ok {
			result[employee.ID] = summarizeStatisticBehavior(behavior)
		}
	}
	return result, nil
}

func (h *StatisticHandler) behaviorTrend(ctx context.Context, corpID int, employees []StatisticEmployee, start time.Time, end time.Time) ([]StatisticBehaviorData, error) {
	if behavior, ok := h.weComBehavior(ctx, corpID, employees, start, end); ok {
		sort.Slice(behavior, func(i, j int) bool { return behavior[i].StatTime < behavior[j].StatTime })
		return behavior, nil
	}
	return h.store.StatisticEmployeeLocalTrend(ctx, corpID, statisticEmployeeIDs(employees), start, end)
}

func (h *StatisticHandler) weComBehavior(ctx context.Context, corpID int, employees []StatisticEmployee, start time.Time, end time.Time) ([]StatisticBehaviorData, bool) {
	if h.client == nil {
		return nil, false
	}
	userIDs := statisticWXUserIDs(employees)
	if len(userIDs) == 0 {
		return nil, false
	}
	credential, ok, err := h.store.RoomWelcomeCorpCredentialByID(ctx, corpID)
	if err != nil || !ok || credential.WXCorpID == "" || credential.ContactSecret == "" {
		return nil, false
	}
	behavior, err := h.client.UserBehavior(ctx, credential, userIDs, start, end)
	if err != nil {
		return nil, false
	}
	return behavior, true
}

func (h *StatisticHandler) statisticEmployees(ctx context.Context, corpID int, employeeIDs []int) ([]StatisticEmployee, error) {
	if len(employeeIDs) > 0 {
		return h.store.StatisticEmployeesByIDs(ctx, corpID, employeeIDs)
	}
	return h.store.StatisticEmployeesByCorp(ctx, corpID)
}

func (h *StatisticHandler) queryDateRange(w http.ResponseWriter, r *http.Request, enforceMax bool) (time.Time, time.Time, bool) {
	start, err := parseStatisticTime(r.URL.Query().Get("startTime"))
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "开始时间格式错误", nil)
		return time.Time{}, time.Time{}, false
	}
	end, err := parseStatisticTime(r.URL.Query().Get("endTime"))
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "结束时间格式错误", nil)
		return time.Time{}, time.Time{}, false
	}
	if end.Before(start) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "结束时间不能早于开始时间", nil)
		return time.Time{}, time.Time{}, false
	}
	if enforceMax && end.Sub(start) > statisticMaxRange {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "日期范围不能超过30天", nil)
		return time.Time{}, time.Time{}, false
	}
	return start, end, true
}

func (h *StatisticHandler) resolveAuthorized(w http.ResponseWriter, r *http.Request, permissionKey string) (int, User, int, bool) {
	userID, user, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return 0, User{}, 0, false
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return 0, User{}, 0, false
	}
	if h.authorizer != nil {
		if _, err := h.authorizer.Resolve(r.Context(), userID, permissionKey, corpID, principalScope.WorkEmployeeID); err != nil {
			writeAccessError(w, err)
			return 0, User{}, 0, false
		}
	}
	return userID, user, corpID, true
}

func (h *StatisticHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (int, User, DashboardRequestScope, bool) {
	requestPrincipal, err := DashboardPrincipalFromContext(r.Context())
	userID := requestPrincipal.UserID
	if err != nil || userID <= 0 {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return 0, User{}, DashboardRequestScope{}, false
	}
	user, found, err := h.store.UserByID(r.Context(), userID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return 0, User{}, DashboardRequestScope{}, false
	}
	if !found {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "user not found", nil)
		return 0, User{}, DashboardRequestScope{}, false
	}
	principalScope, err := DashboardRequestScopeFromContext(r.Context())
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return 0, User{}, DashboardRequestScope{}, false
	}
	return userID, user, DashboardRequestScope(principalScope), true
}

func (h *StatisticHandler) currentTime() time.Time {
	if h.now != nil {
		return h.now()
	}
	return time.Now()
}

func (h *StatisticHandler) fileFullURL(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") || h.apiBaseURL == "" {
		return path
	}
	return h.apiBaseURL + "/static/" + strings.TrimLeft(path, "/")
}

func parseStatisticTime(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		"2006/01/02 15:04:05",
		"2006/01/02 15:04",
		"2006/01/02",
		time.RFC3339,
	}
	for _, layout := range layouts {
		if value, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return value, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid time %q", raw)
}

func statisticEmployeeIDsFromQuery(r *http.Request, key string) []int {
	rawValues, ok := r.URL.Query()[key]
	if !ok || len(rawValues) == 0 {
		return []int{}
	}
	values := make([]any, 0, len(rawValues))
	for _, raw := range rawValues {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if strings.HasPrefix(raw, "[") {
			var decoded []any
			if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
				values = append(values, decoded...)
				continue
			}
		}
		for _, part := range strings.Split(raw, ",") {
			values = append(values, strings.TrimSpace(part))
		}
	}
	ids, _ := intSliceParam(map[string]any{key: values}, key)
	return ids
}

func statisticEmployeeIDs(employees []StatisticEmployee) []int {
	ids := make([]int, 0, len(employees))
	for _, employee := range employees {
		ids = append(ids, employee.ID)
	}
	return ids
}

func statisticWXUserIDs(employees []StatisticEmployee) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(employees))
	for _, employee := range employees {
		value := strings.TrimSpace(employee.WXUserID)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func paginateStatisticEmployees(employees []StatisticEmployee, page int, perPage int) []StatisticEmployee {
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 5
	}
	start := (page - 1) * perPage
	if start >= len(employees) {
		return []StatisticEmployee{}
	}
	end := start + perPage
	if end > len(employees) {
		end = len(employees)
	}
	return employees[start:end]
}

func summarizeStatisticBehavior(items []StatisticBehaviorData) StatisticEmployeeMetric {
	if len(items) == 0 {
		return StatisticEmployeeMetric{}
	}
	var metric StatisticEmployeeMetric
	for _, item := range items {
		metric.ChatCnt += item.ChatCnt
		metric.MessageCnt += item.MessageCnt
		metric.ReplyPercentage += item.ReplyPercentage
		metric.AvgReplyTime += item.AvgReplyTime
	}
	metric.ReplyPercentage = roundFloat(metric.ReplyPercentage/float64(len(items)), 2)
	metric.AvgReplyTime = roundFloat(metric.AvgReplyTime/float64(len(items)), 2)
	return metric
}

func statisticMetricPayload(metric StatisticEmployeeMetric) map[string]any {
	return map[string]any{
		"chat_cnt":         metric.ChatCnt,
		"message_cnt":      metric.MessageCnt,
		"reply_percentage": roundFloat(metric.ReplyPercentage, 2),
		"avg_reply_time":   roundFloat(metric.AvgReplyTime, 2),
	}
}

func statisticContactModeValue(day StatisticContactDay, mode int) (int, bool) {
	switch mode {
	case 1:
		return day.Total, true
	case 2:
		return day.Add, true
	case 3:
		return day.Loss, true
	case 4:
		return day.Net, true
	default:
		return 0, false
	}
}

func statisticBehaviorModeValue(row map[string]any, mode int) (any, bool) {
	switch mode {
	case 1:
		return row["chat_cnt"], true
	case 2:
		return row["message_cnt"], true
	case 3:
		return row["reply_percentage"], true
	case 4:
		return row["avg_reply_time"], true
	default:
		return nil, false
	}
}

func roundFloat(value float64, places int) float64 {
	if places < 0 {
		return value
	}
	scale := math.Pow(10, float64(places))
	return math.Round(value*scale) / scale
}
