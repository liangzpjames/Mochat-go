package dashboard

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type WorkMessageTrajectoryType string

const (
	WorkMessageTrajectoryAll      WorkMessageTrajectoryType = "all"
	WorkMessageTrajectoryEmployee WorkMessageTrajectoryType = "employee"
	WorkMessageTrajectoryCustomer WorkMessageTrajectoryType = "customer"
	WorkMessageTrajectoryRoom     WorkMessageTrajectoryType = "room"
)

type WorkMessageTrajectoryFilter struct {
	TenantID            int
	CorpID              int
	UserID              int
	EmployeeID          int
	Date                string
	StartAt             string
	EndAt               string
	ConversationType    WorkMessageTrajectoryType
	RestrictEmployeeIDs bool
	EmployeeIDs         []int
}

type WorkMessageTrajectoryMetric struct {
	Status       string `json:"status"`
	SubjectTotal *int64 `json:"subjectTotal"`
	MessageTotal *int64 `json:"messageTotal"`
	Reason       string `json:"reason,omitempty"`
}

type WorkMessageTrajectoryEvent struct {
	ID             string `json:"id"`
	ConversationID string `json:"conversationId"`
	Hour           string `json:"hour"`
	TargetType     string `json:"targetType"`
	TargetID       int    `json:"targetId"`
	TargetName     string `json:"targetName"`
	TargetAvatar   string `json:"targetAvatar"`
	TargetStatus   string `json:"targetStatus"`
	MessageTotal   int64  `json:"messageTotal"`
	FirstMessageAt string `json:"firstMessageAt"`
	LastMessageAt  string `json:"lastMessageAt"`
}

type WorkMessageTrajectoryDay struct {
	Employee struct {
		ID     int    `json:"id"`
		Name   string `json:"name"`
		Avatar string `json:"avatar"`
	} `json:"employee"`
	Date                    string                                 `json:"date"`
	Timezone                string                                 `json:"timezone"`
	Metrics                 map[string]WorkMessageTrajectoryMetric `json:"metrics"`
	Events                  []WorkMessageTrajectoryEvent           `json:"events"`
	UnmatchedTargetMessages int64                                  `json:"unmatchedTargetMessages"`
	Limitations             []WorkMessageStaffLimitation           `json:"limitations"`
	Capabilities            []WorkMessageCapability                `json:"capabilities"`
}

type WorkMessageTrajectoryStore interface {
	TrajectoryDay(context.Context, WorkMessageTrajectoryFilter) (WorkMessageTrajectoryDay, error)
}

func workMessageTrajectoryFilter(w http.ResponseWriter, r *http.Request, tenantID, corpID, userID int, access AccessContext, now time.Time) (WorkMessageTrajectoryFilter, bool) {
	employeeID, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("employeeId")))
	if err != nil || employeeID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid employeeId", nil)
		return WorkMessageTrajectoryFilter{}, false
	}
	date := strings.TrimSpace(r.URL.Query().Get("date"))
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return WorkMessageTrajectoryFilter{}, false
	}
	day, err := time.ParseInLocation("2006-01-02", date, location)
	if err != nil || day.Format("2006-01-02") != date {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid date", nil)
		return WorkMessageTrajectoryFilter{}, false
	}
	today := now.In(location).Truncate(24 * time.Hour)
	// Truncate is safe for the fixed Asia/Shanghai location only when now has
	// the same offset; construct the local midnight explicitly for all callers.
	nowLocal := now.In(location)
	today = time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, location)
	if day.After(today) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "date cannot be in the future", nil)
		return WorkMessageTrajectoryFilter{}, false
	}
	kind := WorkMessageTrajectoryType(strings.TrimSpace(r.URL.Query().Get("conversationType")))
	if kind == "" {
		kind = WorkMessageTrajectoryAll
	}
	switch kind {
	case WorkMessageTrajectoryAll, WorkMessageTrajectoryEmployee, WorkMessageTrajectoryCustomer, WorkMessageTrajectoryRoom:
	default:
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid conversationType", nil)
		return WorkMessageTrajectoryFilter{}, false
	}
	start := day.Format("2006-01-02") + " 00:00:00"
	end := day.AddDate(0, 0, 1).Format("2006-01-02") + " 00:00:00"
	ids := workMessageUniquePositiveInts(access.DeptEmployeeIDs)
	return WorkMessageTrajectoryFilter{
		TenantID: tenantID, CorpID: corpID, UserID: userID, EmployeeID: employeeID,
		Date: date, StartAt: start, EndAt: end, ConversationType: kind,
		RestrictEmployeeIDs: access.DataPermission != DataPermissionAll, EmployeeIDs: ids,
	}, true
}

func (h *AutoTagHandler) WorkMessageTrajectoryDay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, access, ok := h.resolveAuthorized(w, r, workMessageConversationPermissionKey)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok || !h.workMessageArchiveAllowed(w, r, principalScope.Principal.TenantID, corpID) {
		return
	}
	filter, ok := workMessageTrajectoryFilter(w, r, principalScope.Principal.TenantID, corpID, userID, access, time.Now())
	if !ok {
		return
	}
	if filter.RestrictEmployeeIDs && !containsInt(filter.EmployeeIDs, filter.EmployeeID) {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, ErrWorkMessageConversationNotFound.Error(), nil)
		return
	}
	store, ok := h.store.(WorkMessageTrajectoryStore)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "会话轨迹数据能力未接入", nil)
		return
	}
	day, err := store.TrajectoryDay(r.Context(), filter)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if day.Metrics == nil {
		day.Metrics = map[string]WorkMessageTrajectoryMetric{}
	}
	if day.Events == nil {
		day.Events = []WorkMessageTrajectoryEvent{}
	}
	if day.Limitations == nil {
		day.Limitations = []WorkMessageStaffLimitation{}
	}
	if day.Capabilities == nil {
		day.Capabilities = []WorkMessageCapability{}
	}
	writeEnvelope(w, http.StatusOK, 200, "success", day)
}
