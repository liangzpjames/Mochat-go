package dashboard

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type WorkMessageStaffMode string

const (
	WorkMessageStaffModeAll      WorkMessageStaffMode = "all"
	WorkMessageStaffModeFocused  WorkMessageStaffMode = "focused"
	WorkMessageStaffModeArchived WorkMessageStaffMode = "archived"
	WorkMessageStaffModeDeparted WorkMessageStaffMode = "departed"
)

type WorkMessageStaffDirectoryFilter struct {
	TenantID            int
	CorpID              int
	UserID              int
	Mode                WorkMessageStaffMode
	Keyword             string
	DepartmentID        int
	Page                int
	PageSize            int
	RestrictEmployeeIDs bool
	EmployeeIDs         []int
}

type WorkMessageStaffDepartment struct {
	ID            int                          `json:"id"`
	ParentID      int                          `json:"parentId"`
	Name          string                       `json:"name"`
	EmployeeCount int                          `json:"employeeCount"`
	Children      []WorkMessageStaffDepartment `json:"children"`
}

type WorkMessageStaffEmployee struct {
	ID                       int    `json:"id"`
	Name                     string `json:"name"`
	Avatar                   string `json:"avatar"`
	Status                   int    `json:"status"`
	DepartmentIDs            []int  `json:"departmentIds"`
	Archived                 bool   `json:"archived"`
	ConversationCount        int    `json:"conversationCount"`
	FocusedConversationCount int    `json:"focusedConversationCount"`
	LastConversationAt       string `json:"lastConversationAt"`
}

type WorkMessageStaffCounts struct {
	All      int `json:"all"`
	Focused  int `json:"focused"`
	Archived int `json:"archived"`
	Departed int `json:"departed"`
}

type WorkMessageStaffLimitation struct {
	Key    string `json:"key"`
	Reason string `json:"reason"`
}

type WorkMessageStaffDirectoryPage struct {
	Departments  []WorkMessageStaffDepartment `json:"departments"`
	Employees    []WorkMessageStaffEmployee   `json:"employees"`
	Counts       WorkMessageStaffCounts       `json:"counts"`
	Page         int                          `json:"page"`
	PageSize     int                          `json:"pageSize"`
	Total        int                          `json:"total"`
	Limitations  []WorkMessageStaffLimitation `json:"limitations"`
	Capabilities []WorkMessageCapability      `json:"capabilities"`
}

type WorkMessageStaffDetailFilter struct {
	TenantID            int
	CorpID              int
	UserID              int
	EmployeeID          int
	ToUserType          int
	ToUserID            int
	Keyword             string
	Date                string
	Before              string
	MessageTypes        []int
	EmployeeIDs         []int
	RestrictEmployeeIDs bool
	PageSize            int
}

type WorkMessageStaffStats struct {
	CommunicationDays int `json:"communicationDays"`
	MessageTotal      int `json:"messageTotal"`
	InboundTotal      int `json:"inboundTotal"`
	OutboundTotal     int `json:"outboundTotal"`
}

type WorkMessageStaffMessage struct {
	ID              string `json:"id"`
	SenderName      string `json:"senderName"`
	SenderAvatar    string `json:"senderAvatar"`
	Direction       string `json:"direction"`
	SentAt          string `json:"sentAt"`
	ArchiveSource   string `json:"archiveSource"`
	ArchiveSourceID string `json:"archiveSourceId"`
	Type            int    `json:"type"`
	Content         any    `json:"content"`
}

type WorkMessageStaffDetail struct {
	ConversationID string                    `json:"conversationId"`
	EmployeeID     int                       `json:"employeeId"`
	TargetID       int                       `json:"targetId"`
	EmployeeName   string                    `json:"employeeName"`
	TargetType     string                    `json:"targetType"`
	TargetName     string                    `json:"targetName"`
	Focused        bool                      `json:"focused"`
	Stats          WorkMessageStaffStats     `json:"stats"`
	Messages       []WorkMessageStaffMessage `json:"messages"`
	NextBefore     string                    `json:"nextBefore"`
	HasMore        bool                      `json:"hasMore"`
	Capabilities   []WorkMessageCapability   `json:"capabilities"`
}

type WorkMessageStaffCursor struct {
	SentAt     string `json:"sentAt"`
	TableIndex int    `json:"tableIndex"`
	Seq        int64  `json:"seq"`
	ID         int    `json:"id"`
}

type WorkMessageStaffStore interface {
	StaffDirectory(context.Context, WorkMessageStaffDirectoryFilter) (WorkMessageStaffDirectoryPage, error)
	StaffDetail(context.Context, WorkMessageStaffDetailFilter) (WorkMessageStaffDetail, error)
}

func EncodeWorkMessageStaffCursor(cursor WorkMessageStaffCursor) string {
	raw, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func DecodeWorkMessageStaffCursor(raw string) (WorkMessageStaffCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return WorkMessageStaffCursor{}, err
	}
	var cursor WorkMessageStaffCursor
	if err := json.Unmarshal(decoded, &cursor); err != nil {
		return WorkMessageStaffCursor{}, err
	}
	if cursor.SentAt == "" || cursor.TableIndex < 1 || cursor.TableIndex > WorkMessageArchiveMessageTableCount || cursor.ID <= 0 {
		return WorkMessageStaffCursor{}, errors.New("invalid staff message cursor")
	}
	return cursor, nil
}

func (h *AutoTagHandler) WorkMessageStaffDirectory(w http.ResponseWriter, r *http.Request) {
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
	filter, ok := workMessageStaffDirectoryFilter(w, r, principalScope.Principal.TenantID, corpID, userID, access)
	if !ok {
		return
	}
	store, ok := h.store.(WorkMessageStaffStore)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "员工会话目录数据能力未接入", nil)
		return
	}
	page, err := store.StaffDirectory(r.Context(), filter)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if page.Departments == nil {
		page.Departments = []WorkMessageStaffDepartment{}
	}
	if page.Employees == nil {
		page.Employees = []WorkMessageStaffEmployee{}
	}
	for index := range page.Employees {
		if page.Employees[index].DepartmentIDs == nil {
			page.Employees[index].DepartmentIDs = []int{}
		}
	}
	if page.Limitations == nil {
		page.Limitations = []WorkMessageStaffLimitation{}
	}
	if page.Capabilities == nil {
		page.Capabilities = []WorkMessageCapability{}
	}
	writeEnvelope(w, http.StatusOK, 200, "success", page)
}

func (h *AutoTagHandler) WorkMessageStaffDetail(w http.ResponseWriter, r *http.Request) {
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
	filter, ok := workMessageStaffDetailFilter(w, r, principalScope.Principal.TenantID, corpID, userID, access)
	if !ok {
		return
	}
	if filter.RestrictEmployeeIDs && !containsInt(filter.EmployeeIDs, filter.EmployeeID) {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, ErrWorkMessageConversationNotFound.Error(), nil)
		return
	}
	store, ok := h.store.(WorkMessageStaffStore)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "员工会话详情数据能力未接入", nil)
		return
	}
	detail, err := store.StaffDetail(r.Context(), filter)
	if errors.Is(err, ErrWorkMessageConversationNotFound) {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, err.Error(), nil)
		return
	}
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if detail.Messages == nil {
		detail.Messages = []WorkMessageStaffMessage{}
	}
	if detail.Capabilities == nil {
		detail.Capabilities = []WorkMessageCapability{}
	}
	writeEnvelope(w, http.StatusOK, 200, "success", detail)
}

func workMessageStaffDirectoryFilter(w http.ResponseWriter, r *http.Request, tenantID, corpID, userID int, access AccessContext) (WorkMessageStaffDirectoryFilter, bool) {
	mode := WorkMessageStaffMode(strings.TrimSpace(r.URL.Query().Get("mode")))
	if mode == "" {
		mode = WorkMessageStaffModeAll
	}
	switch mode {
	case WorkMessageStaffModeAll, WorkMessageStaffModeFocused, WorkMessageStaffModeArchived, WorkMessageStaffModeDeparted:
	default:
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid mode", nil)
		return WorkMessageStaffDirectoryFilter{}, false
	}
	departmentID, ok := optionalPositiveInt(w, r, "departmentId")
	if !ok {
		return WorkMessageStaffDirectoryFilter{}, false
	}
	page, ok := workMessageStrictPositiveQueryInt(w, r, "page", 1)
	if !ok {
		return WorkMessageStaffDirectoryFilter{}, false
	}
	pageSize, ok := workMessageStrictPositiveQueryInt(w, r, "pageSize", 50)
	if !ok || pageSize != 50 {
		if ok {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "pageSize must be 50", nil)
		}
		return WorkMessageStaffDirectoryFilter{}, false
	}
	return WorkMessageStaffDirectoryFilter{
		TenantID: tenantID, CorpID: corpID, UserID: userID, Mode: mode,
		Keyword: strings.TrimSpace(r.URL.Query().Get("keyword")), DepartmentID: departmentID,
		Page: page, PageSize: pageSize, RestrictEmployeeIDs: access.DataPermission != DataPermissionAll,
		EmployeeIDs: workMessageUniquePositiveInts(access.DeptEmployeeIDs),
	}, true
}

func workMessageStaffDetailFilter(w http.ResponseWriter, r *http.Request, tenantID, corpID, userID int, access AccessContext) (WorkMessageStaffDetailFilter, bool) {
	employeeID, toUserType, toUserID, valid := parseWorkMessageConversationID(strings.TrimSpace(r.URL.Query().Get("conversationId")))
	if !valid {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid conversationId", nil)
		return WorkMessageStaffDetailFilter{}, false
	}
	pageSize, ok := workMessageStrictPositiveQueryInt(w, r, "pageSize", 50)
	if !ok || pageSize != 50 {
		if ok {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "pageSize must be 50", nil)
		}
		return WorkMessageStaffDetailFilter{}, false
	}
	types, ok := parseWorkMessageTypes(w, r.URL.Query()["messageTypes"])
	if !ok {
		return WorkMessageStaffDetailFilter{}, false
	}
	date := strings.TrimSpace(r.URL.Query().Get("date"))
	if date != "" {
		if _, err := time.Parse("2006-01-02", date); err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid date", nil)
			return WorkMessageStaffDetailFilter{}, false
		}
	}
	before := strings.TrimSpace(r.URL.Query().Get("before"))
	if before != "" {
		if _, err := DecodeWorkMessageStaffCursor(before); err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid before", nil)
			return WorkMessageStaffDetailFilter{}, false
		}
	}
	return WorkMessageStaffDetailFilter{
		TenantID: tenantID, CorpID: corpID, UserID: userID, EmployeeID: employeeID, ToUserType: toUserType, ToUserID: toUserID,
		Keyword: strings.TrimSpace(r.URL.Query().Get("keyword")), Date: date, Before: before, MessageTypes: types,
		EmployeeIDs: workMessageUniquePositiveInts(access.DeptEmployeeIDs), RestrictEmployeeIDs: access.DataPermission != DataPermissionAll, PageSize: pageSize,
	}, true
}

func optionalPositiveInt(w http.ResponseWriter, r *http.Request, key string) (int, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return 0, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid "+key, nil)
		return 0, false
	}
	return value, true
}
