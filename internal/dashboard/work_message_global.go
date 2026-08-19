package dashboard

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type WorkMessageBucket string

const (
	WorkMessageBucketAll     WorkMessageBucket = "all"
	WorkMessageBucketTimeout WorkMessageBucket = "timeout"
	WorkMessageBucketRisk    WorkMessageBucket = "risk"
	WorkMessageBucketToday   WorkMessageBucket = "today"
	WorkMessageBucketFocused WorkMessageBucket = "focused"
)

type WorkMessageMetricValue struct {
	Value      *int64   `json:"value"`
	ChangeRate *float64 `json:"changeRate"`
	Status     string   `json:"status"`
	Reason     string   `json:"reason,omitempty"`
}

type WorkMessageCapability struct {
	Key       string `json:"key"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

type WorkMessageGlobalFilter struct {
	TenantID            int
	CorpID              int
	UserID              int
	EmployeeIDs         []int
	RestrictEmployeeIDs bool
	ToUserID            int
	Keyword             string
	ConversationType    string
	MessageTypes        []int
	StartAt             string
	EndAt               string
	ArchiveSource       string
	Bucket              WorkMessageBucket
	Page                int
	PageSize            int
}

type WorkMessageGlobalConversation struct {
	ID              string `json:"id"`
	ConversationID  string `json:"conversationId"`
	EmployeeID      int    `json:"employeeId"`
	EmployeeName    string `json:"employeeName"`
	EmployeeAvatar  string `json:"employeeAvatar"`
	TargetType      string `json:"targetType"`
	TargetID        int    `json:"targetId"`
	TargetName      string `json:"targetName"`
	TargetAvatar    string `json:"targetAvatar"`
	LastMessage     string `json:"lastMessage"`
	LastMessageType int    `json:"lastMessageType"`
	LastDirection   string `json:"lastDirection"`
	SentAt          string `json:"sentAt"`
	MessageTotal    int    `json:"messageTotal"`
	RiskCount       int    `json:"riskCount"`
	TimeoutCount    int    `json:"timeoutCount"`
	Focused         bool   `json:"focused"`
	ArchiveSource   string `json:"archiveSource"`
	ArchiveSourceID string `json:"archiveSourceId"`
}

type WorkMessageGlobalPage struct {
	Items    []WorkMessageGlobalConversation
	Total    int
	Page     int
	PageSize int
}

type WorkMessageGlobalOverview struct {
	Metrics      map[string]WorkMessageMetricValue `json:"metrics"`
	Capabilities []WorkMessageCapability           `json:"capabilities"`
}

type WorkMessageFocusInput struct {
	TenantID       int
	CorpID         int
	UserID         int
	WorkEmployeeID int
	ToUserType     int
	ToUserID       int
}

var ErrWorkMessageConversationNotFound = errors.New("work message conversation not found")

type WorkMessageGlobalStore interface {
	WorkMessageGlobalPage(context.Context, WorkMessageGlobalFilter) (WorkMessageGlobalPage, error)
	WorkMessageGlobalOverview(context.Context, WorkMessageGlobalFilter) (WorkMessageGlobalOverview, error)
	SetWorkMessageFocus(context.Context, WorkMessageFocusInput) error
	DeleteWorkMessageFocus(context.Context, WorkMessageFocusInput) error
	WorkMessageGlobalConversationExists(context.Context, WorkMessageGlobalFilter) (bool, error)
}

func (h *AutoTagHandler) WorkMessageGlobalOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, access, ok := h.resolveAuthorized(w, r, workMessageConversationPermissionKey)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	if !h.workMessageArchiveAllowed(w, r, principalScope.Principal.TenantID, corpID) {
		return
	}
	filter, ok := workMessageGlobalFilterFromRequest(w, r, principalScope.Principal.TenantID, corpID, userID, access)
	if !ok {
		return
	}
	store, ok := h.store.(WorkMessageGlobalStore)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "全局消息概览数据能力未接入", nil)
		return
	}
	overview, err := store.WorkMessageGlobalOverview(r.Context(), filter)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", overview)
}

func (h *AutoTagHandler) WorkMessageFocus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodDelete {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, access, ok := h.resolveAuthorized(w, r, workMessageConversationPermissionKey)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	if !h.workMessageArchiveAllowed(w, r, principalScope.Principal.TenantID, corpID) {
		return
	}
	var body struct {
		ConversationID string `json:"conversationId"`
	}
	if err := decodeAuthJSONBody(r, &body); err != nil || strings.TrimSpace(body.ConversationID) == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "conversationId is required", nil)
		return
	}
	employeeID, toUserType, toUserID, valid := parseWorkMessageConversationID(body.ConversationID)
	if !valid {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid conversationId", nil)
		return
	}
	filter := WorkMessageGlobalFilter{
		TenantID:            principalScope.Principal.TenantID,
		CorpID:              corpID,
		EmployeeIDs:         []int{employeeID},
		RestrictEmployeeIDs: true,
		ConversationType:    workMessageTargetType(toUserType),
		Page:                1,
		PageSize:            1,
	}
	store, ok := h.store.(WorkMessageGlobalStore)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "全局消息关注能力未接入", nil)
		return
	}
	exists, err := store.WorkMessageGlobalConversationExists(r.Context(), filterForConversation(filter, employeeID, toUserType, toUserID, access))
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !exists {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, ErrWorkMessageConversationNotFound.Error(), nil)
		return
	}
	input := WorkMessageFocusInput{TenantID: filter.TenantID, CorpID: filter.CorpID, UserID: userID, WorkEmployeeID: employeeID, ToUserType: toUserType, ToUserID: toUserID}
	if r.Method == http.MethodPut {
		err = store.SetWorkMessageFocus(r.Context(), input)
	} else {
		err = store.DeleteWorkMessageFocus(r.Context(), input)
	}
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{"conversationId": body.ConversationID, "focused": r.Method == http.MethodPut})
}

func filterForConversation(filter WorkMessageGlobalFilter, employeeID, toUserType, toUserID int, access AccessContext) WorkMessageGlobalFilter {
	filter.ConversationType = workMessageTargetType(toUserType)
	filter.EmployeeIDs = []int{employeeID}
	filter.RestrictEmployeeIDs = true
	filter.Page = 1
	filter.PageSize = 1
	if access.DataPermission != DataPermissionAll {
		filter.EmployeeIDs = workMessageEmployeeIntersection(filter.EmployeeIDs, access.DeptEmployeeIDs)
	}
	filter.ToUserID = toUserID
	return filter
}

func workMessageGlobalFilterFromRequest(w http.ResponseWriter, r *http.Request, tenantID, corpID, userID int, access AccessContext) (WorkMessageGlobalFilter, bool) {
	legacy, ok := workMessageGlobalFilter(w, r, corpID, access)
	if !ok {
		return WorkMessageGlobalFilter{}, false
	}
	bucket := WorkMessageBucket(strings.TrimSpace(r.URL.Query().Get("bucket")))
	if bucket == "" {
		bucket = WorkMessageBucketAll
	}
	switch bucket {
	case WorkMessageBucketAll, WorkMessageBucketTimeout, WorkMessageBucketRisk, WorkMessageBucketToday, WorkMessageBucketFocused:
	default:
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid bucket", nil)
		return WorkMessageGlobalFilter{}, false
	}
	types, ok := parseWorkMessageTypes(w, r.URL.Query()["messageTypes"])
	if !ok {
		return WorkMessageGlobalFilter{}, false
	}
	return WorkMessageGlobalFilter{
		TenantID:            tenantID,
		CorpID:              corpID,
		UserID:              userID,
		EmployeeIDs:         append([]int(nil), legacy.EmployeeIDs...),
		RestrictEmployeeIDs: legacy.RestrictEmployeeIDs,
		ToUserID:            legacy.ToUserID,
		Keyword:             legacy.Keyword,
		ConversationType:    workMessageTargetType(legacy.ToUserType),
		MessageTypes:        types,
		StartAt:             legacy.DateTimeStart,
		EndAt:               legacy.DateTimeEnd,
		ArchiveSource:       legacy.ArchiveSource,
		Bucket:              bucket,
		Page:                legacy.Page,
		PageSize:            legacy.PerPage,
	}, true
}

func parseWorkMessageTypes(w http.ResponseWriter, values []string) ([]int, bool) {
	if len(values) == 0 {
		return nil, true
	}
	result := make([]int, 0, len(values))
	seen := map[int]struct{}{}
	for _, raw := range values {
		for _, value := range strings.Split(raw, ",") {
			value = strings.TrimSpace(strings.ToLower(value))
			if value == "" {
				continue
			}
			mapped := map[string][]int{"text": {1, 100}, "image": {2}, "voice": {3}, "video": {4}, "file": {5}, "link": {6}, "location": {7}, "emotion": {8}, "mixed": {9}, "markdown": {10}, "meeting": {11}, "doc": {13}, "calendar": {14}, "vote": {15}, "redpacket": {17}, "card": {18}}
			typeIDs, ok := mapped[value]
			if !ok {
				parsed, err := strconv.Atoi(value)
				if err != nil || parsed <= 0 {
					writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, fmt.Sprintf("invalid messageTypes: %s", value), nil)
					return nil, false
				}
				typeIDs = []int{parsed}
			}
			for _, typeID := range typeIDs {
				if _, exists := seen[typeID]; !exists {
					seen[typeID] = struct{}{}
					result = append(result, typeID)
				}
			}
		}
	}
	return result, true
}
