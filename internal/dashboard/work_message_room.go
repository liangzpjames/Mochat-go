package dashboard

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type WorkMessageRoomMode string

const (
	WorkMessageRoomModeActive    WorkMessageRoomMode = "active"
	WorkMessageRoomModeDissolved WorkMessageRoomMode = "dissolved"
)

type WorkMessageRoomMemberMode string

const (
	WorkMessageRoomMemberModeAll      WorkMessageRoomMemberMode = "all"
	WorkMessageRoomMemberModeEmployee WorkMessageRoomMemberMode = "employee"
	WorkMessageRoomMemberModeCustomer WorkMessageRoomMemberMode = "customer"
	WorkMessageRoomMemberModeLeft     WorkMessageRoomMemberMode = "left"
)

type WorkMessageRoomDirectoryFilter struct {
	TenantID            int
	CorpID              int
	UserID              int
	RoomMode            WorkMessageRoomMode
	Keyword             string
	Page                int
	PageSize            int
	CustomerIDs         []int
	RoomGroupIDs        []int
	RestrictEmployeeIDs bool
	EmployeeIDs         []int
}

type WorkMessageRoomDirectoryItem struct {
	ID            int    `json:"id"`
	ExternalID    string `json:"externalId"`
	Name          string `json:"name"`
	Avatar        string `json:"avatar"`
	OwnerID       int    `json:"ownerId"`
	OwnerName     string `json:"ownerName"`
	MemberCount   int    `json:"memberCount"`
	EmployeeCount int    `json:"employeeCount"`
	CustomerCount int    `json:"customerCount"`
	MessageCount  int    `json:"messageCount"`
	LastMessage   string `json:"lastMessage"`
	LastMessageAt string `json:"lastMessageAt"`
	Focused       bool   `json:"focused"`
	RiskCount     int    `json:"riskCount"`
	TimeoutCount  int    `json:"timeoutCount"`
	Dissolved     bool   `json:"dissolved"`
}

type WorkMessageRoomDirectoryPage struct {
	Items        []WorkMessageRoomDirectoryItem `json:"items"`
	Total        int                            `json:"total"`
	Page         int                            `json:"page"`
	PageSize     int                            `json:"pageSize"`
	Capabilities []WorkMessageCapability        `json:"capabilities"`
	Limitations  []WorkMessageStaffLimitation   `json:"limitations"`
}

type WorkMessageRoomProfileFilter struct {
	TenantID            int
	CorpID              int
	UserID              int
	RoomID              int
	RestrictEmployeeIDs bool
	EmployeeIDs         []int
}

type WorkMessageRoomProfile struct {
	ID            int                          `json:"id"`
	ExternalID    string                       `json:"externalId"`
	Name          string                       `json:"name"`
	Avatar        string                       `json:"avatar"`
	OwnerID       int                          `json:"ownerId"`
	OwnerName     string                       `json:"ownerName"`
	MemberCount   int                          `json:"memberCount"`
	EmployeeCount int                          `json:"employeeCount"`
	CustomerCount int                          `json:"customerCount"`
	CreatedAt     string                       `json:"createdAt"`
	Status        string                       `json:"status"`
	Dissolved     bool                         `json:"dissolved"`
	Focused       bool                         `json:"focused"`
	RiskCount     int                          `json:"riskCount"`
	TimeoutCount  int                          `json:"timeoutCount"`
	Capabilities  []WorkMessageCapability      `json:"capabilities"`
	Limitations   []WorkMessageStaffLimitation `json:"limitations"`
}

type WorkMessageRoomMessagesFilter struct {
	TenantID            int
	CorpID              int
	UserID              int
	RoomID              int
	Keyword             string
	Date                string
	Before              string
	MessageTypes        []int
	RestrictEmployeeIDs bool
	EmployeeIDs         []int
	PageSize            int
}

type WorkMessageRoomMessage struct {
	ID              string `json:"id"`
	SenderID        int    `json:"senderId"`
	SenderName      string `json:"senderName"`
	SenderAvatar    string `json:"senderAvatar"`
	SenderKind      string `json:"senderKind"`
	Direction       string `json:"direction"`
	SentAt          string `json:"sentAt"`
	ArchiveSource   string `json:"archiveSource"`
	ArchiveSourceID string `json:"archiveSourceId"`
	Type            int    `json:"type"`
	Content         any    `json:"content"`
}

type WorkMessageRoomMessageStats struct {
	MessageTotal  int `json:"messageTotal"`
	EmployeeTotal int `json:"employeeTotal"`
	CustomerTotal int `json:"customerTotal"`
	RiskTotal     int `json:"riskTotal"`
	TimeoutTotal  int `json:"timeoutTotal"`
}

type WorkMessageRoomMessages struct {
	RoomID       int                         `json:"roomId"`
	Stats        WorkMessageRoomMessageStats `json:"stats"`
	Messages     []WorkMessageRoomMessage    `json:"messages"`
	NextBefore   string                      `json:"nextBefore"`
	HasMore      bool                        `json:"hasMore"`
	Capabilities []WorkMessageCapability     `json:"capabilities"`
}

type WorkMessageRoomMembersFilter struct {
	TenantID            int
	CorpID              int
	UserID              int
	RoomID              int
	Mode                WorkMessageRoomMemberMode
	Keyword             string
	Page                int
	PageSize            int
	RestrictEmployeeIDs bool
	EmployeeIDs         []int
}

type WorkMessageRoomMember struct {
	ID         int    `json:"id"`
	ExternalID string `json:"externalId"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Avatar     string `json:"avatar"`
	JoinedAt   string `json:"joinedAt"`
	LeftAt     string `json:"leftAt"`
	Status     string `json:"status"`
	EmployeeID int    `json:"employeeId"`
	CustomerID int    `json:"customerId"`
}

type WorkMessageRoomMemberPage struct {
	Items        []WorkMessageRoomMember `json:"items"`
	Total        int                     `json:"total"`
	Page         int                     `json:"page"`
	PageSize     int                     `json:"pageSize"`
	Capabilities []WorkMessageCapability `json:"capabilities"`
}

type WorkMessageRoomFilterOptionsFilter struct {
	TenantID            int
	CorpID              int
	UserID              int
	Kind                string
	RestrictEmployeeIDs bool
	EmployeeIDs         []int
}

type WorkMessageRoomOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

type WorkMessageRoomFilterOptions struct {
	Employees    []WorkMessageRoomOption `json:"employees"`
	Customers    []WorkMessageRoomOption `json:"customers"`
	Groups       []WorkMessageRoomOption `json:"groups"`
	Capabilities []WorkMessageCapability `json:"capabilities"`
}

type WorkMessageRoomStore interface {
	RoomDirectory(context.Context, WorkMessageRoomDirectoryFilter) (WorkMessageRoomDirectoryPage, error)
	RoomProfile(context.Context, WorkMessageRoomProfileFilter) (WorkMessageRoomProfile, error)
	RoomMessages(context.Context, WorkMessageRoomMessagesFilter) (WorkMessageRoomMessages, error)
	RoomMembers(context.Context, WorkMessageRoomMembersFilter) (WorkMessageRoomMemberPage, error)
	RoomFilterOptions(context.Context, WorkMessageRoomFilterOptionsFilter) (WorkMessageRoomFilterOptions, error)
}

func (h *AutoTagHandler) WorkMessageRoomDirectory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, access, ok := h.resolveAuthorized(w, r, workMessageConversationPermissionKey)
	if !ok || !h.workMessageArchiveAllowed(w, r, principalScope.Principal.TenantID, principalScope.Principal.CorpID) {
		return
	}
	filter, ok := workMessageRoomDirectoryFilter(w, r, principalScope.Principal.TenantID, principalScope.Principal.CorpID, userID, access)
	if !ok {
		return
	}
	store, ok := h.store.(WorkMessageRoomStore)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "群会话目录数据能力未接入", nil)
		return
	}
	page, err := store.RoomDirectory(r.Context(), filter)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	normalizeWorkMessageRoomDirectoryPage(&page)
	writeEnvelope(w, http.StatusOK, 200, "success", page)
}

func (h *AutoTagHandler) WorkMessageRoomProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, access, ok := h.resolveAuthorized(w, r, workMessageConversationPermissionKey)
	if !ok || !h.workMessageArchiveAllowed(w, r, principalScope.Principal.TenantID, principalScope.Principal.CorpID) {
		return
	}
	roomID, ok := requiredRoomID(w, r)
	if !ok {
		return
	}
	store, ok := h.store.(WorkMessageRoomStore)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "群会话资料数据能力未接入", nil)
		return
	}
	profile, err := store.RoomProfile(r.Context(), WorkMessageRoomProfileFilter{
		TenantID: principalScope.Principal.TenantID, CorpID: principalScope.Principal.CorpID, UserID: userID, RoomID: roomID,
		RestrictEmployeeIDs: access.DataPermission != DataPermissionAll, EmployeeIDs: workMessageUniquePositiveInts(access.DeptEmployeeIDs),
	})
	if errors.Is(err, ErrWorkMessageConversationNotFound) {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, err.Error(), nil)
		return
	}
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	normalizeWorkMessageRoomProfile(&profile)
	writeEnvelope(w, http.StatusOK, 200, "success", profile)
}

func (h *AutoTagHandler) WorkMessageRoomMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, access, ok := h.resolveAuthorized(w, r, workMessageConversationPermissionKey)
	if !ok || !h.workMessageArchiveAllowed(w, r, principalScope.Principal.TenantID, principalScope.Principal.CorpID) {
		return
	}
	roomID, ok := requiredRoomID(w, r)
	if !ok {
		return
	}
	filter, ok := workMessageRoomMessagesFilter(w, r, principalScope.Principal.TenantID, principalScope.Principal.CorpID, userID, roomID, access)
	if !ok {
		return
	}
	store, ok := h.store.(WorkMessageRoomStore)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "群会话消息数据能力未接入", nil)
		return
	}
	messages, err := store.RoomMessages(r.Context(), filter)
	if errors.Is(err, ErrWorkMessageConversationNotFound) {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, err.Error(), nil)
		return
	}
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if projector, supported := h.store.(WorkMessageMediaProjector); supported {
		if err := projector.ProjectWorkMessageRoomMedia(r.Context(), principalScope.Principal.TenantID, principalScope.Principal.CorpID, &messages); err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "会话媒体读取失败", nil)
			return
		}
	}
	normalizeWorkMessageRoomMessages(&messages)
	writeEnvelope(w, http.StatusOK, 200, "success", messages)
}

func (h *AutoTagHandler) WorkMessageRoomMembers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, access, ok := h.resolveAuthorized(w, r, workMessageConversationPermissionKey)
	if !ok || !h.workMessageArchiveAllowed(w, r, principalScope.Principal.TenantID, principalScope.Principal.CorpID) {
		return
	}
	roomID, ok := requiredRoomID(w, r)
	if !ok {
		return
	}
	filter, ok := workMessageRoomMembersFilter(w, r, principalScope.Principal.TenantID, principalScope.Principal.CorpID, userID, roomID, access)
	if !ok {
		return
	}
	store, ok := h.store.(WorkMessageRoomStore)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "群成员数据能力未接入", nil)
		return
	}
	page, err := store.RoomMembers(r.Context(), filter)
	if errors.Is(err, ErrWorkMessageConversationNotFound) {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, err.Error(), nil)
		return
	}
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	normalizeWorkMessageRoomMemberPage(&page)
	writeEnvelope(w, http.StatusOK, 200, "success", page)
}

func (h *AutoTagHandler) WorkMessageRoomFilterOptions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, access, ok := h.resolveAuthorized(w, r, workMessageConversationPermissionKey)
	if !ok || !h.workMessageArchiveAllowed(w, r, principalScope.Principal.TenantID, principalScope.Principal.CorpID) {
		return
	}
	kind := strings.TrimSpace(r.URL.Query().Get("kind"))
	if kind != "" && kind != "employee" && kind != "customer" && kind != "group" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid kind", nil)
		return
	}
	store, ok := h.store.(WorkMessageRoomStore)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "群会话筛选数据能力未接入", nil)
		return
	}
	options, err := store.RoomFilterOptions(r.Context(), WorkMessageRoomFilterOptionsFilter{
		TenantID: principalScope.Principal.TenantID, CorpID: principalScope.Principal.CorpID, UserID: userID, Kind: kind,
		RestrictEmployeeIDs: access.DataPermission != DataPermissionAll, EmployeeIDs: workMessageUniquePositiveInts(access.DeptEmployeeIDs),
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	normalizeWorkMessageRoomFilterOptions(&options)
	writeEnvelope(w, http.StatusOK, 200, "success", options)
}

func workMessageRoomDirectoryFilter(w http.ResponseWriter, r *http.Request, tenantID, corpID, userID int, access AccessContext) (WorkMessageRoomDirectoryFilter, bool) {
	roomMode := WorkMessageRoomMode(strings.TrimSpace(r.URL.Query().Get("roomMode")))
	if roomMode == "" {
		roomMode = WorkMessageRoomModeActive
	}
	if roomMode != WorkMessageRoomModeActive && roomMode != WorkMessageRoomModeDissolved {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid roomMode", nil)
		return WorkMessageRoomDirectoryFilter{}, false
	}
	page, ok := workMessageStrictPositiveQueryInt(w, r, "page", 1)
	if !ok {
		return WorkMessageRoomDirectoryFilter{}, false
	}
	pageSize, ok := workMessageStrictPositiveQueryInt(w, r, "pageSize", 50)
	if !ok || pageSize != 50 {
		if ok {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "pageSize must be 50", nil)
		}
		return WorkMessageRoomDirectoryFilter{}, false
	}
	employeeIDs, ok := workMessageEmployeeIDs(w, r)
	if !ok {
		return WorkMessageRoomDirectoryFilter{}, false
	}
	customerIDs, ok := workMessageRoomPositiveIDs(w, r, "customerIds")
	if !ok {
		return WorkMessageRoomDirectoryFilter{}, false
	}
	roomGroupIDs, ok := workMessageRoomPositiveIDs(w, r, "roomGroupIds")
	if !ok {
		return WorkMessageRoomDirectoryFilter{}, false
	}
	if access.DataPermission != DataPermissionAll {
		if len(r.URL.Query()["employeeIds"]) > 0 || r.URL.Query().Has("employeeId") {
			employeeIDs = workMessageEmployeeIntersection(employeeIDs, access.DeptEmployeeIDs)
		} else {
			employeeIDs = workMessageUniquePositiveInts(access.DeptEmployeeIDs)
		}
	}
	return WorkMessageRoomDirectoryFilter{
		TenantID: tenantID, CorpID: corpID, UserID: userID, RoomMode: roomMode,
		Keyword: strings.TrimSpace(r.URL.Query().Get("keyword")), Page: page, PageSize: pageSize,
		RestrictEmployeeIDs: access.DataPermission != DataPermissionAll,
		EmployeeIDs:         employeeIDs, CustomerIDs: customerIDs, RoomGroupIDs: roomGroupIDs,
	}, true
}

func workMessageRoomPositiveIDs(w http.ResponseWriter, r *http.Request, key string) ([]int, bool) {
	rawValues := r.URL.Query()[key]
	ids := make([]int, 0, len(rawValues))
	for _, rawValue := range rawValues {
		for _, raw := range strings.Split(rawValue, ",") {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			value, err := strconv.Atoi(raw)
			if err != nil || value <= 0 {
				writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid "+key, nil)
				return nil, false
			}
			ids = append(ids, value)
		}
	}
	return workMessageUniquePositiveInts(ids), true
}

func workMessageRoomMessagesFilter(w http.ResponseWriter, r *http.Request, tenantID, corpID, userID, roomID int, access AccessContext) (WorkMessageRoomMessagesFilter, bool) {
	pageSize, ok := workMessageStrictPositiveQueryInt(w, r, "pageSize", 50)
	if !ok || pageSize != 50 {
		if ok {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "pageSize must be 50", nil)
		}
		return WorkMessageRoomMessagesFilter{}, false
	}
	types, ok := parseWorkMessageTypes(w, r.URL.Query()["messageTypes"])
	if !ok {
		return WorkMessageRoomMessagesFilter{}, false
	}
	date := strings.TrimSpace(r.URL.Query().Get("date"))
	if date != "" {
		if _, err := time.Parse("2006-01-02", date); err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid date", nil)
			return WorkMessageRoomMessagesFilter{}, false
		}
	}
	before := strings.TrimSpace(r.URL.Query().Get("before"))
	if before != "" {
		if _, err := DecodeWorkMessageStaffCursor(before); err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid before", nil)
			return WorkMessageRoomMessagesFilter{}, false
		}
	}
	return WorkMessageRoomMessagesFilter{
		TenantID: tenantID, CorpID: corpID, UserID: userID, RoomID: roomID,
		Keyword: strings.TrimSpace(r.URL.Query().Get("keyword")), Date: date, Before: before,
		MessageTypes: types, PageSize: pageSize, RestrictEmployeeIDs: access.DataPermission != DataPermissionAll,
		EmployeeIDs: workMessageUniquePositiveInts(access.DeptEmployeeIDs),
	}, true
}

func workMessageRoomMembersFilter(w http.ResponseWriter, r *http.Request, tenantID, corpID, userID, roomID int, access AccessContext) (WorkMessageRoomMembersFilter, bool) {
	mode := WorkMessageRoomMemberMode(strings.TrimSpace(r.URL.Query().Get("mode")))
	if mode == "" {
		mode = WorkMessageRoomMemberModeAll
	}
	switch mode {
	case WorkMessageRoomMemberModeAll, WorkMessageRoomMemberModeEmployee, WorkMessageRoomMemberModeCustomer, WorkMessageRoomMemberModeLeft:
	default:
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid mode", nil)
		return WorkMessageRoomMembersFilter{}, false
	}
	page, ok := workMessageStrictPositiveQueryInt(w, r, "page", 1)
	if !ok {
		return WorkMessageRoomMembersFilter{}, false
	}
	pageSize, ok := workMessageStrictPositiveQueryInt(w, r, "pageSize", 50)
	if !ok || pageSize != 50 {
		if ok {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "pageSize must be 50", nil)
		}
		return WorkMessageRoomMembersFilter{}, false
	}
	return WorkMessageRoomMembersFilter{
		TenantID: tenantID, CorpID: corpID, UserID: userID, RoomID: roomID, Mode: mode,
		Keyword: strings.TrimSpace(r.URL.Query().Get("keyword")), Page: page, PageSize: pageSize,
		RestrictEmployeeIDs: access.DataPermission != DataPermissionAll,
		EmployeeIDs:         workMessageUniquePositiveInts(access.DeptEmployeeIDs),
	}, true
}

func requiredRoomID(w http.ResponseWriter, r *http.Request) (int, bool) {
	roomID, ok := workMessageStrictPositiveQueryInt(w, r, "roomId", 0)
	if !ok {
		return 0, false
	}
	if roomID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid roomId", nil)
		return 0, false
	}
	return roomID, true
}

func normalizeWorkMessageRoomDirectoryPage(page *WorkMessageRoomDirectoryPage) {
	if page.Items == nil {
		page.Items = []WorkMessageRoomDirectoryItem{}
	}
	if page.Capabilities == nil {
		page.Capabilities = []WorkMessageCapability{}
	}
	if page.Limitations == nil {
		page.Limitations = []WorkMessageStaffLimitation{}
	}
}

func normalizeWorkMessageRoomProfile(profile *WorkMessageRoomProfile) {
	if profile.Capabilities == nil {
		profile.Capabilities = []WorkMessageCapability{}
	}
	if profile.Limitations == nil {
		profile.Limitations = []WorkMessageStaffLimitation{}
	}
}

func normalizeWorkMessageRoomMessages(messages *WorkMessageRoomMessages) {
	if messages.Messages == nil {
		messages.Messages = []WorkMessageRoomMessage{}
	}
	if messages.Capabilities == nil {
		messages.Capabilities = []WorkMessageCapability{}
	}
}

func normalizeWorkMessageRoomMemberPage(page *WorkMessageRoomMemberPage) {
	if page.Items == nil {
		page.Items = []WorkMessageRoomMember{}
	}
	if page.Capabilities == nil {
		page.Capabilities = []WorkMessageCapability{}
	}
}

func normalizeWorkMessageRoomFilterOptions(options *WorkMessageRoomFilterOptions) {
	if options.Employees == nil {
		options.Employees = []WorkMessageRoomOption{}
	}
	if options.Customers == nil {
		options.Customers = []WorkMessageRoomOption{}
	}
	if options.Groups == nil {
		options.Groups = []WorkMessageRoomOption{}
	}
	if options.Capabilities == nil {
		options.Capabilities = []WorkMessageCapability{}
	}
}
