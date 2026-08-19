package dashboard

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type WorkMessageCustomerMode string

const (
	WorkMessageCustomerModeAll     WorkMessageCustomerMode = "all"
	WorkMessageCustomerModeFocused WorkMessageCustomerMode = "focused"
	WorkMessageCustomerModeActive  WorkMessageCustomerMode = "active"
	WorkMessageCustomerModeLost    WorkMessageCustomerMode = "lost"
)

type WorkMessageCustomerConversationMode string

const (
	WorkMessageCustomerConversationModeDirect WorkMessageCustomerConversationMode = "direct"
	WorkMessageCustomerConversationModeGroup  WorkMessageCustomerConversationMode = "group"
)

type WorkMessageCustomerDirectoryFilter struct {
	TenantID            int
	CorpID              int
	UserID              int
	Mode                WorkMessageCustomerMode
	Keyword             string
	Page                int
	PageSize            int
	RestrictEmployeeIDs bool
	EmployeeIDs         []int
}

type WorkMessageCustomerConversationFilter struct {
	TenantID            int
	CorpID              int
	UserID              int
	CustomerID          int
	Mode                WorkMessageCustomerConversationMode
	Page                int
	PageSize            int
	RestrictEmployeeIDs bool
	EmployeeIDs         []int
}

type WorkMessageCustomerDetailFilter struct {
	TenantID            int
	CorpID              int
	UserID              int
	CustomerID          int
	EmployeeID          int
	ToUserType          int
	ToUserID            int
	Keyword             string
	MessageTypes        []int
	Date                string
	PageSize            int
	Before              string
	RestrictEmployeeIDs bool
	EmployeeIDs         []int
}

type WorkMessageCustomerProfile struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Avatar        string `json:"avatar"`
	ProfileStatus string `json:"profileStatus"`
}

type WorkMessageCustomerDirectoryItem struct {
	ID                       int    `json:"id"`
	Name                     string `json:"name"`
	Avatar                   string `json:"avatar"`
	ProfileStatus            string `json:"profileStatus"`
	ActiveRelationCount      int    `json:"activeRelationCount"`
	LostRelationCount        int    `json:"lostRelationCount"`
	DirectConversationCount  int    `json:"directConversationCount"`
	GroupConversationCount   int    `json:"groupConversationCount"`
	FocusedConversationCount int    `json:"focusedConversationCount"`
	LastConversationAt       string `json:"lastConversationAt"`
}

type WorkMessageCustomerCounts struct {
	All     int `json:"all"`
	Focused int `json:"focused"`
	Active  int `json:"active"`
	Lost    int `json:"lost"`
}

type WorkMessageCustomerLimitation struct {
	Key    string `json:"key"`
	Reason string `json:"reason"`
}

type WorkMessageCustomerDirectoryPage struct {
	Customers    []WorkMessageCustomerDirectoryItem `json:"customers"`
	Counts       WorkMessageCustomerCounts          `json:"counts"`
	Page         int                                `json:"page"`
	PageSize     int                                `json:"pageSize"`
	Total        int                                `json:"total"`
	Limitations  []WorkMessageCustomerLimitation    `json:"limitations"`
	Capabilities []WorkMessageCapability            `json:"capabilities"`
}

type WorkMessageCustomerConversation struct {
	WorkMessageGlobalConversation
	RelationStatus   string `json:"relationStatus,omitempty"`
	MembershipStatus string `json:"membershipStatus,omitempty"`
}

type WorkMessageCustomerConversationPage struct {
	Customer     WorkMessageCustomerProfile          `json:"customer"`
	Mode         WorkMessageCustomerConversationMode `json:"mode"`
	List         []WorkMessageCustomerConversation   `json:"list"`
	Total        int                                 `json:"total"`
	Page         int                                 `json:"page"`
	PageSize     int                                 `json:"pageSize"`
	Capabilities []WorkMessageCapability             `json:"capabilities"`
}

type WorkMessageCustomerDetail struct {
	WorkMessageStaffDetail
	CustomerID   int                        `json:"customerId"`
	CustomerName string                     `json:"customerName"`
	Profile      WorkMessageCustomerProfile `json:"profile"`
}

type WorkMessageCustomerDirectoryStore interface {
	WorkMessageCustomerDirectory(context.Context, WorkMessageCustomerDirectoryFilter) (WorkMessageCustomerDirectoryPage, error)
}

type WorkMessageCustomerConversationStore interface {
	WorkMessageCustomerConversations(context.Context, WorkMessageCustomerConversationFilter) (WorkMessageCustomerConversationPage, error)
}

type WorkMessageCustomerDetailStore interface {
	WorkMessageCustomerDetail(context.Context, WorkMessageCustomerDetailFilter) (WorkMessageCustomerDetail, error)
}

func WorkMessageCustomerCapabilities() []WorkMessageCapability {
	return []WorkMessageCapability{
		{Key: "groupMemberIdentity", Available: false, Reason: "当前归档数据无法稳定识别群聊入站消息的具体外部成员"},
		{Key: "conversationDownload", Available: false, Reason: "当前系统未提供完整的单会话下载能力"},
	}
}

func (h *AutoTagHandler) WorkMessageCustomerDirectory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principal, access, ok := h.resolveAuthorized(w, r, workMessageConversationPermissionKey)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok || !h.workMessageArchiveAllowed(w, r, principal.Principal.TenantID, corpID) {
		return
	}
	filter, ok := workMessageCustomerDirectoryFilter(w, r, principal.Principal.TenantID, corpID, userID, access)
	if !ok {
		return
	}
	store, ok := h.store.(WorkMessageCustomerDirectoryStore)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "客户目录能力未接入", nil)
		return
	}
	page, err := store.WorkMessageCustomerDirectory(r.Context(), filter)
	if err != nil {
		writeWorkMessageCustomerError(w, err)
		return
	}
	if page.Customers == nil {
		page.Customers = []WorkMessageCustomerDirectoryItem{}
	}
	if page.Limitations == nil {
		page.Limitations = []WorkMessageCustomerLimitation{}
	}
	if page.Capabilities == nil {
		page.Capabilities = []WorkMessageCapability{}
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", page)
}

func (h *AutoTagHandler) WorkMessageCustomerConversations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principal, access, ok := h.resolveAuthorized(w, r, workMessageConversationPermissionKey)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok || !h.workMessageArchiveAllowed(w, r, principal.Principal.TenantID, corpID) {
		return
	}
	filter, ok := workMessageCustomerConversationFilter(w, r, principal.Principal.TenantID, corpID, userID, access)
	if !ok {
		return
	}
	store, ok := h.store.(WorkMessageCustomerConversationStore)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "客户关联会话能力未接入", nil)
		return
	}
	page, err := store.WorkMessageCustomerConversations(r.Context(), filter)
	if err != nil {
		writeWorkMessageCustomerError(w, err)
		return
	}
	if page.List == nil {
		page.List = []WorkMessageCustomerConversation{}
	}
	if page.Capabilities == nil {
		page.Capabilities = []WorkMessageCapability{}
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", page)
}

func (h *AutoTagHandler) WorkMessageCustomerDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principal, access, ok := h.resolveAuthorized(w, r, workMessageConversationPermissionKey)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok || !h.workMessageArchiveAllowed(w, r, principal.Principal.TenantID, corpID) {
		return
	}
	filter, ok := workMessageCustomerDetailFilter(w, r, principal.Principal.TenantID, corpID, userID, access)
	if !ok {
		return
	}
	if filter.RestrictEmployeeIDs && !containsInt(filter.EmployeeIDs, filter.EmployeeID) {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, ErrWorkMessageConversationNotFound.Error(), nil)
		return
	}
	store, ok := h.store.(WorkMessageCustomerDetailStore)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "客户会话详情能力未接入", nil)
		return
	}
	detail, err := store.WorkMessageCustomerDetail(r.Context(), filter)
	if err != nil {
		writeWorkMessageCustomerError(w, err)
		return
	}
	if detail.Messages == nil {
		detail.Messages = []WorkMessageStaffMessage{}
	}
	if detail.Capabilities == nil {
		detail.Capabilities = []WorkMessageCapability{}
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", detail)
}

func workMessageCustomerDirectoryFilter(w http.ResponseWriter, r *http.Request, tenantID, corpID, userID int, access AccessContext) (WorkMessageCustomerDirectoryFilter, bool) {
	mode := WorkMessageCustomerMode(strings.TrimSpace(r.URL.Query().Get("mode")))
	if mode == "" {
		mode = WorkMessageCustomerModeAll
	}
	switch mode {
	case WorkMessageCustomerModeAll, WorkMessageCustomerModeFocused, WorkMessageCustomerModeActive, WorkMessageCustomerModeLost:
	default:
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid mode", nil)
		return WorkMessageCustomerDirectoryFilter{}, false
	}
	page, ok := workMessageStrictPositiveQueryInt(w, r, "page", 1)
	if !ok {
		return WorkMessageCustomerDirectoryFilter{}, false
	}
	pageSize, ok := workMessageCustomerPageSize(w, r, 50)
	if !ok {
		return WorkMessageCustomerDirectoryFilter{}, false
	}
	filter := WorkMessageCustomerDirectoryFilter{TenantID: tenantID, CorpID: corpID, UserID: userID, Mode: mode, Keyword: strings.TrimSpace(r.URL.Query().Get("keyword")), Page: page, PageSize: pageSize}
	return filter.withAccess(access), true
}

func workMessageCustomerConversationFilter(w http.ResponseWriter, r *http.Request, tenantID, corpID, userID int, access AccessContext) (WorkMessageCustomerConversationFilter, bool) {
	customerID, ok := requiredPositiveCustomerID(w, r)
	if !ok {
		return WorkMessageCustomerConversationFilter{}, false
	}
	mode := WorkMessageCustomerConversationMode(strings.TrimSpace(r.URL.Query().Get("mode")))
	if mode == "" {
		mode = WorkMessageCustomerConversationModeDirect
	}
	switch mode {
	case WorkMessageCustomerConversationModeDirect, WorkMessageCustomerConversationModeGroup:
	default:
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid mode", nil)
		return WorkMessageCustomerConversationFilter{}, false
	}
	page, ok := workMessageStrictPositiveQueryInt(w, r, "page", 1)
	if !ok {
		return WorkMessageCustomerConversationFilter{}, false
	}
	pageSize, ok := workMessageCustomerPageSize(w, r, 20)
	if !ok {
		return WorkMessageCustomerConversationFilter{}, false
	}
	filter := WorkMessageCustomerConversationFilter{TenantID: tenantID, CorpID: corpID, UserID: userID, CustomerID: customerID, Mode: mode, Page: page, PageSize: pageSize}
	if access.DataPermission != DataPermissionAll {
		filter.RestrictEmployeeIDs = true
		filter.EmployeeIDs = workMessageUniquePositiveInts(access.DeptEmployeeIDs)
	}
	return filter, true
}

func workMessageCustomerDetailFilter(w http.ResponseWriter, r *http.Request, tenantID, corpID, userID int, access AccessContext) (WorkMessageCustomerDetailFilter, bool) {
	customerID, ok := requiredPositiveCustomerID(w, r)
	if !ok {
		return WorkMessageCustomerDetailFilter{}, false
	}
	employeeID, toUserType, toUserID, valid := parseWorkMessageConversationID(strings.TrimSpace(r.URL.Query().Get("conversationId")))
	if !valid {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid conversationId", nil)
		return WorkMessageCustomerDetailFilter{}, false
	}
	pageSize, ok := workMessageCustomerPageSize(w, r, 50)
	if !ok {
		return WorkMessageCustomerDetailFilter{}, false
	}
	types, ok := parseWorkMessageTypes(w, r.URL.Query()["messageTypes"])
	if !ok {
		return WorkMessageCustomerDetailFilter{}, false
	}
	date := strings.TrimSpace(r.URL.Query().Get("date"))
	if date != "" {
		if _, err := time.Parse("2006-01-02", date); err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid date", nil)
			return WorkMessageCustomerDetailFilter{}, false
		}
	}
	before := strings.TrimSpace(r.URL.Query().Get("before"))
	if before != "" {
		if _, err := DecodeWorkMessageStaffCursor(before); err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid before", nil)
			return WorkMessageCustomerDetailFilter{}, false
		}
	}
	filter := WorkMessageCustomerDetailFilter{TenantID: tenantID, CorpID: corpID, UserID: userID, CustomerID: customerID, EmployeeID: employeeID, ToUserType: toUserType, ToUserID: toUserID, Keyword: strings.TrimSpace(r.URL.Query().Get("keyword")), MessageTypes: types, Date: date, PageSize: pageSize, Before: before}
	if access.DataPermission != DataPermissionAll {
		filter.RestrictEmployeeIDs = true
		filter.EmployeeIDs = workMessageUniquePositiveInts(access.DeptEmployeeIDs)
	}
	return filter, true
}

func (filter WorkMessageCustomerDirectoryFilter) withAccess(access AccessContext) WorkMessageCustomerDirectoryFilter {
	if access.DataPermission != DataPermissionAll {
		filter.RestrictEmployeeIDs = true
		filter.EmployeeIDs = workMessageUniquePositiveInts(access.DeptEmployeeIDs)
	}
	return filter
}

func requiredPositiveCustomerID(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get("customerId"))
	if raw == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "customerId is required", nil)
		return 0, false
	}
	customerID, err := strconv.Atoi(raw)
	if err != nil || customerID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid customerId", nil)
		return 0, false
	}
	return customerID, true
}

func workMessageCustomerPageSize(w http.ResponseWriter, r *http.Request, expected int) (int, bool) {
	pageSize, ok := workMessageStrictPositiveQueryInt(w, r, "pageSize", expected)
	if !ok {
		return 0, false
	}
	if pageSize != expected {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "pageSize must be "+strconv.Itoa(expected), nil)
		return 0, false
	}
	return pageSize, true
}

func writeWorkMessageCustomerError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrWorkMessageConversationNotFound) {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
}
