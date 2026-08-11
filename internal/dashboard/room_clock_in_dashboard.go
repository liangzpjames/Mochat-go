package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type RoomClockInFilter struct {
	CorpID             int
	CreateUserID       int
	RestrictCreateUser bool
	ActiveName         string
	Status             int
	Page               int
	PerPage            int
}

type RoomClockInItem struct {
	ID                int
	OfficialAccountID int
	ActiveName        string
	Description       string
	Type              int
	StartTime         string
	EndTime           string
	TasksRaw          string
	EmployeeQRCode    string
	ContactTagsRaw    string
	CorpCardStatus    int
	CorpCardRaw       string
	Status            int
	TenantID          int
	CorpID            int
	CreateUserID      int
	CreateUserName    string
	ContactNum        int
	ClockInNum        int
	ReceiveNum        int
	CreatedAt         string
	UpdatedAt         string
}

type RoomClockInPage struct {
	Items     []RoomClockInItem
	Total     int
	TotalPage int
	Page      int
	PerPage   int
}

type RoomClockInWrite struct {
	CorpID               int
	CreateUserID         int
	OfficialAccountID    int
	HasOfficialAccountID bool
	ActiveName           string
	HasActiveName        bool
	Description          string
	HasDescription       bool
	Type                 int
	HasType              bool
	StartTime            string
	HasStartTime         bool
	EndTime              string
	HasEndTime           bool
	TasksRaw             string
	HasTasks             bool
	EmployeeQRCode       string
	HasEmployeeQRCode    bool
	ContactTagsRaw       string
	HasContactTags       bool
	CorpCardStatus       int
	HasCorpCardStatus    bool
	CorpCardRaw          string
	HasCorpCard          bool
	Status               int
	HasStatus            bool
}

type RoomClockInContactFilter struct {
	CorpID    int
	ClockInID int
	Nickname  string
	Status    int
	WriteOff  int
	Page      int
	PerPage   int
}

type RoomClockInContactItem struct {
	ID             int
	ClockInID      int
	UnionID        string
	OpenID         string
	Nickname       string
	Avatar         string
	City           string
	ContactID      int
	EmployeeIDsRaw string
	ContactTagsRaw string
	DayCount       int
	Status         int
	ReceiveLevel   int
	WriteOff       int
	FirstClockAt   string
	LastClockAt    string
	CreatedAt      string
	UpdatedAt      string
}

type RoomClockInContactPage struct {
	Items     []RoomClockInContactItem
	Total     int
	TotalPage int
	Page      int
	PerPage   int
}

type RoomClockInDayRecord struct {
	ID        int
	ClockInID int
	ContactID int
	UnionID   string
	Day       string
	CreatedAt string
	UpdatedAt string
}

type RoomClockInDayPage struct {
	Items     []RoomClockInDayRecord
	Total     int
	TotalPage int
	Page      int
	PerPage   int
}

type RoomClockInStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	RoomClockInPage(ctx context.Context, filter RoomClockInFilter) (RoomClockInPage, error)
	RoomClockInByID(ctx context.Context, corpID int, id int) (RoomClockInItem, bool, error)
	CreateRoomClockIn(ctx context.Context, values RoomClockInWrite) (int, error)
	UpdateRoomClockIn(ctx context.Context, corpID int, id int, values RoomClockInWrite) (bool, error)
	DeleteRoomClockIn(ctx context.Context, corpID int, id int) (bool, error)
	RoomClockInContactPage(ctx context.Context, filter RoomClockInContactFilter) (RoomClockInContactPage, error)
	RoomClockInDayPage(ctx context.Context, corpID int, clockInID int, contactID int, unionID string, page int, perPage int) (RoomClockInDayPage, error)
	BatchTagRoomClockInContacts(ctx context.Context, corpID int, clockInID int, contactIDs []int, tagIDs []int) (int, error)
}

type RoomClockInHandler struct {
	store            RoomClockInStore
	cache            LoginCache
	resolver         UserIDResolver
	authorizer       CorpAdminAuthorizer
	operationBaseURL string
}

func NewRoomClockInHandler(store RoomClockInStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer, operationBaseURL string) *RoomClockInHandler {
	return &RoomClockInHandler{store: store, cache: cache, resolver: resolver, authorizer: authorizer, operationBaseURL: strings.TrimRight(strings.TrimSpace(operationBaseURL), "/")}
}

func (h *RoomClockInHandler) Index(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, user, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/roomClockIn/index#get")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	page, err := h.store.RoomClockInPage(r.Context(), RoomClockInFilter{
		CorpID:             corpID,
		CreateUserID:       userID,
		RestrictCreateUser: user.IsSuperAdmin == 0,
		ActiveName:         lotteryQueryString(r, "activeName", "active_name", "name", "keyword"),
		Status:             lotteryQueryIntAllowZero(r, "status", -1),
		Page:               positiveQueryInt(r, "page", 1),
		PerPage:            positiveQueryInt(r, "perPage", 15),
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, roomClockInPayload(item, h.shareURL(item.ID), true))
	}
	payload := contactBatchAddPagination(r, page.Page, page.PerPage, page.Total, list)
	payload["list"] = list
	payload["page"] = map[string]any{"perPage": page.PerPage, "total": page.Total, "totalPage": page.TotalPage}
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *RoomClockInHandler) Store(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, user, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/roomClockIn/store#post")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, 400, err.Error(), nil)
		return
	}
	values, err := roomClockInWriteFromParams(params, corpID, userID, true)
	if err != nil {
		status := http.StatusInternalServerError
		code := http.StatusInternalServerError
		if isBadRequestError(err) {
			status = http.StatusBadRequest
			code = 400
		}
		writeEnvelope(w, status, code, err.Error(), nil)
		return
	}
	if !enforceSaaSQuota(r.Context(), w, h.store, user.TenantID, SaaSMetricRoomClockIns, 1) {
		return
	}
	id, err := h.store.CreateRoomClockIn(r.Context(), values)
	if err != nil {
		if writeSaaSQuotaError(w, err) {
			return
		}
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if err := refreshSaaSUsageCounter(r.Context(), h.store, user.TenantID, SaaSMetricRoomClockIns); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{id})
}

func (h *RoomClockInHandler) Update(w http.ResponseWriter, r *http.Request) {
	h.writeMutation(w, r, http.MethodPut, "/dashboard/roomClockIn/update#put", func(ctx context.Context, userID int, corpID int, params map[string]any) (any, error) {
		id, err := roomClockInIDFromParams(params)
		if err != nil || id <= 0 {
			return nil, badRequestError("clockInId required")
		}
		values, err := roomClockInWriteFromParams(params, corpID, userID, false)
		if err != nil {
			return nil, err
		}
		updated, err := h.store.UpdateRoomClockIn(ctx, corpID, id, values)
		if err != nil {
			return nil, err
		}
		if !updated {
			return nil, badRequestError("room clock in not found")
		}
		return []any{id}, nil
	})
}

func (h *RoomClockInHandler) Destroy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, user, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/roomClockIn/destroy#delete")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, 400, err.Error(), nil)
		return
	}
	id, err := roomClockInIDFromParams(params)
	if err != nil || id <= 0 {
		writeEnvelope(w, http.StatusBadRequest, 400, "clockInId required", nil)
		return
	}
	deleted, err := h.store.DeleteRoomClockIn(r.Context(), corpID, id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !deleted {
		writeEnvelope(w, http.StatusBadRequest, 400, "room clock in not found", nil)
		return
	}
	if err := refreshSaaSUsageCounter(r.Context(), h.store, user.TenantID, SaaSMetricRoomClockIns); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{id})
}

func (h *RoomClockInHandler) Show(w http.ResponseWriter, r *http.Request) {
	h.show(w, r, "/dashboard/roomClockIn/show#get")
}

func (h *RoomClockInHandler) Info(w http.ResponseWriter, r *http.Request) {
	h.show(w, r, "/dashboard/roomClockIn/info#get")
}

func (h *RoomClockInHandler) ShowContact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/roomClockIn/showContact#get")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	page, err := h.store.RoomClockInContactPage(r.Context(), RoomClockInContactFilter{
		CorpID:    corpID,
		ClockInID: roomClockInIDFromQuery(r),
		Nickname:  lotteryQueryString(r, "nickname", "name", "contactName"),
		Status:    lotteryQueryIntAllowZero(r, "status", -1),
		WriteOff:  lotteryQueryIntAllowZero(r, "writeOff", -1),
		Page:      positiveQueryInt(r, "page", 1),
		PerPage:   positiveQueryInt(r, "perPage", 15),
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, roomClockInContactPayload(item))
	}
	payload := contactBatchAddPagination(r, page.Page, page.PerPage, page.Total, list)
	payload["list"] = list
	payload["page"] = map[string]any{"perPage": page.PerPage, "total": page.Total, "totalPage": page.TotalPage}
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *RoomClockInHandler) DayDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/roomClockIn/dayDetail#get")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	contactID := positiveQueryInt(r, "contactId", 0)
	if contactID <= 0 {
		contactID = positiveQueryInt(r, "contact_id", 0)
	}
	if contactID <= 0 {
		contactID = positiveQueryInt(r, "id", 0)
	}
	page, err := h.store.RoomClockInDayPage(r.Context(), corpID, roomClockInIDFromQuery(r), contactID, lotteryQueryString(r, "unionId", "union_id"), positiveQueryInt(r, "page", 1), positiveQueryInt(r, "perPage", 31))
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	days := make([]string, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, roomClockInDayPayload(item))
		if strings.TrimSpace(item.Day) != "" {
			days = append(days, item.Day)
		}
	}
	payload := contactBatchAddPagination(r, page.Page, page.PerPage, page.Total, list)
	payload["list"] = list
	payload["days"] = days
	payload["page"] = map[string]any{"perPage": page.PerPage, "total": page.Total, "totalPage": page.TotalPage}
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *RoomClockInHandler) BatchContactTags(w http.ResponseWriter, r *http.Request) {
	h.writeMutation(w, r, http.MethodPut, "/dashboard/roomClockIn/batchContactTags#put", func(ctx context.Context, _ int, corpID int, params map[string]any) (any, error) {
		clockInID, _, err := firstPositiveIntParam(params, "clockInId", "clock_in_id", "activityId", "activity_id", "id")
		if err != nil || clockInID <= 0 {
			return nil, badRequestError("clockInId required")
		}
		contactIDs, err := firstIntSliceParam(params, "contactIds", "contact_ids", "contactId", "contact_id")
		if err != nil || len(contactIDs) == 0 {
			return nil, badRequestError("contactIds required")
		}
		tagIDs, err := firstIntSliceParam(params, "tagIds", "tag_ids", "tags", "contactTags")
		if err != nil || len(tagIDs) == 0 {
			return nil, badRequestError("tagIds required")
		}
		affected, err := h.store.BatchTagRoomClockInContacts(ctx, corpID, clockInID, contactIDs, tagIDs)
		if err != nil {
			return nil, err
		}
		return map[string]any{"affected": affected}, nil
	})
}

func (h *RoomClockInHandler) show(w http.ResponseWriter, r *http.Request, permission string) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, _, ok := h.resolveAuthorized(w, r, permission)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	id := roomClockInIDFromQuery(r)
	if id <= 0 {
		writeEnvelope(w, http.StatusBadRequest, 400, "clockInId required", nil)
		return
	}
	item, found, err := h.store.RoomClockInByID(r.Context(), corpID, id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusNotFound, 404, "room clock in not found", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", roomClockInPayload(item, h.shareURL(id), false))
}

func (h *RoomClockInHandler) writeMutation(w http.ResponseWriter, r *http.Request, method string, permission string, mutate func(context.Context, int, int, map[string]any) (any, error)) {
	if r.Method != method {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, _, _, ok := h.resolveAuthorized(w, r, permission)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, 400, err.Error(), nil)
		return
	}
	data, err := mutate(r.Context(), userID, corpID, params)
	if err != nil {
		status := http.StatusInternalServerError
		code := http.StatusInternalServerError
		if isBadRequestError(err) {
			status = http.StatusBadRequest
			code = 400
		}
		writeEnvelope(w, status, code, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", data)
}

func (h *RoomClockInHandler) resolveAuthorized(w http.ResponseWriter, r *http.Request, permissionKey string) (int, User, DashboardRequestScope, AccessContext, bool) {
	userID, user, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return 0, User{}, DashboardRequestScope{}, AccessContext{}, false
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return 0, User{}, DashboardRequestScope{}, AccessContext{}, false
	}
	employeeID := principalScope.WorkEmployeeID
	if employeeID <= 0 {
		resolved, err := h.store.EmployeeIDByUserCorp(r.Context(), userID, corpID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return 0, User{}, DashboardRequestScope{}, AccessContext{}, false
		}
		employeeID = resolved
	}
	access := AccessContext{User: user, PermissionKey: permissionKey, CorpID: corpID, WorkEmployeeID: employeeID, DataPermission: DataPermissionAll}
	if h.authorizer != nil {
		resolved, err := h.authorizer.Resolve(r.Context(), userID, permissionKey, corpID, employeeID)
		if err != nil {
			writeAccessError(w, err)
			return 0, User{}, DashboardRequestScope{}, AccessContext{}, false
		}
		access = resolved
	}
	return userID, user, principalScope, access, true
}

func (h *RoomClockInHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (int, User, DashboardRequestScope, bool) {
	if h.resolver == nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "user resolver not configured", nil)
		return 0, User{}, DashboardRequestScope{}, false
	}
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
	return userID, user, principalScope, true
}

func (h *RoomClockInHandler) shareURL(id int) string {
	base := h.operationBaseURL
	if base == "" {
		base = "/operation"
	}
	return base + "/roomClockIn?id=" + strconv.Itoa(id)
}

func roomClockInWriteFromParams(params map[string]any, corpID int, userID int, requireName bool) (RoomClockInWrite, error) {
	if nested, ok := roomClockInMapParam(params, "clockIn"); ok {
		for key, value := range params {
			if _, exists := nested[key]; !exists && key != "clockIn" {
				nested[key] = value
			}
		}
		params = nested
	} else if nested, ok := roomClockInMapParam(params, "activity"); ok {
		for key, value := range params {
			if _, exists := nested[key]; !exists && key != "activity" {
				nested[key] = value
			}
		}
		params = nested
	}
	values := RoomClockInWrite{CorpID: corpID, CreateUserID: userID}
	if id, ok, err := firstIntParam(params, "officialAccountId", "official_account_id"); err != nil {
		return values, err
	} else if ok {
		values.OfficialAccountID = id
		values.HasOfficialAccountID = true
	}
	if name, ok := firstStringParam(params, "activeName", "active_name", "name", "activityName"); ok {
		values.ActiveName = name
		values.HasActiveName = true
	}
	if requireName && strings.TrimSpace(values.ActiveName) == "" {
		return values, badRequestError("activeName required")
	}
	if description, ok := firstStringParam(params, "description", "desc"); ok {
		values.Description = description
		values.HasDescription = true
	}
	if typ, ok, err := firstIntParam(params, "type", "clockInType", "clock_in_type"); err != nil {
		return values, err
	} else if ok {
		values.Type = typ
		values.HasType = true
	}
	if !values.HasType && requireName {
		values.Type = 1
		values.HasType = true
	}
	if startTime, ok := firstStringParam(params, "startTime", "start_time"); ok {
		values.StartTime = startTime
		values.HasStartTime = true
	}
	if endTime, ok := firstStringParam(params, "endTime", "end_time"); ok {
		values.EndTime = endTime
		values.HasEndTime = true
	}
	if raw, ok := firstJSONRawParam(params, "tasks", "task", "prizeSet", "prize_set"); ok {
		values.TasksRaw = raw
		values.HasTasks = true
	}
	if qrcode, ok := firstStringParam(params, "employeeQrcode", "employeeQRCode", "employee_qrcode", "qrcode", "qrCode"); ok {
		values.EmployeeQRCode = qrcode
		values.HasEmployeeQRCode = true
	}
	if raw, ok := firstJSONRawParam(params, "contactTags", "contact_tags", "tags"); ok {
		values.ContactTagsRaw = raw
		values.HasContactTags = true
	}
	if status, ok, err := roomFissionFirstBoolIntParam(params, "corpCardStatus", "corp_card_status"); err != nil {
		return values, err
	} else if ok {
		values.CorpCardStatus = status
		values.HasCorpCardStatus = true
	}
	if raw, ok := firstJSONRawParam(params, "corpCard", "corp_card", "corpInfo", "corp_info"); ok {
		values.CorpCardRaw = raw
		values.HasCorpCard = true
	}
	if status, ok, err := firstIntParam(params, "status"); err != nil {
		return values, err
	} else if ok {
		values.Status = status
		values.HasStatus = true
	}
	if !values.HasStatus && requireName {
		values.Status = 1
		values.HasStatus = true
	}
	if !values.HasTasks && requireName {
		values.TasksRaw = "[]"
		values.HasTasks = true
	}
	return values, nil
}

func roomClockInIDFromParams(params map[string]any) (int, error) {
	if id, has, err := firstPositiveIntParam(params, "clockInId", "clock_in_id", "activityId", "activity_id", "id"); err != nil || has {
		return id, err
	}
	for _, key := range []string{"clockIn", "activity"} {
		if nested, ok := roomClockInMapParam(params, key); ok {
			id, _, err := firstPositiveIntParam(nested, "clockInId", "clock_in_id", "activityId", "activity_id", "id")
			return id, err
		}
	}
	return 0, nil
}

func roomClockInIDFromQuery(r *http.Request) int {
	for _, key := range []string{"clockInId", "clock_in_id", "activityId", "activity_id", "id"} {
		if value := positiveQueryInt(r, key, 0); value > 0 {
			return value
		}
	}
	return 0
}

func roomClockInMapParam(params map[string]any, key string) (map[string]any, bool) {
	value, ok := params[key]
	if !ok || value == nil {
		return nil, false
	}
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case string:
		var decoded map[string]any
		if err := json.Unmarshal([]byte(typed), &decoded); err == nil {
			return decoded, true
		}
	}
	return nil, false
}

func roomClockInPayload(item RoomClockInItem, link string, list bool) map[string]any {
	payload := map[string]any{
		"id":                  item.ID,
		"clockInId":           item.ID,
		"clock_in_id":         item.ID,
		"activityId":          item.ID,
		"activity_id":         item.ID,
		"officialAccountId":   item.OfficialAccountID,
		"official_account_id": item.OfficialAccountID,
		"activeName":          item.ActiveName,
		"active_name":         item.ActiveName,
		"name":                item.ActiveName,
		"description":         item.Description,
		"type":                item.Type,
		"startTime":           item.StartTime,
		"start_time":          item.StartTime,
		"endTime":             item.EndTime,
		"end_time":            item.EndTime,
		"tasks":               jsonPayload(item.TasksRaw),
		"employeeQrcode":      item.EmployeeQRCode,
		"employeeQRCode":      item.EmployeeQRCode,
		"employee_qrcode":     item.EmployeeQRCode,
		"contactTags":         jsonPayload(item.ContactTagsRaw),
		"contact_tags":        jsonPayload(item.ContactTagsRaw),
		"corpCardStatus":      item.CorpCardStatus,
		"corp_card_status":    item.CorpCardStatus,
		"corpCard":            jsonPayload(item.CorpCardRaw),
		"corp_card":           jsonPayload(item.CorpCardRaw),
		"corpInfo":            jsonPayload(item.CorpCardRaw),
		"corp_info":           jsonPayload(item.CorpCardRaw),
		"status":              item.Status,
		"statusText":          roomClockInStatusText(item.Status),
		"tenantId":            item.TenantID,
		"corpId":              item.CorpID,
		"createUserId":        item.CreateUserID,
		"createUserName":      item.CreateUserName,
		"contactNum":          item.ContactNum,
		"clockInNum":          item.ClockInNum,
		"receiveNum":          item.ReceiveNum,
		"link":                link,
		"shareUrl":            link,
		"url":                 link,
		"qrcodeUrl":           link,
		"createdAt":           item.CreatedAt,
		"updatedAt":           item.UpdatedAt,
	}
	if !list {
		payload["clockIn"] = map[string]any{
			"id":                  item.ID,
			"officialAccountId":   item.OfficialAccountID,
			"official_account_id": item.OfficialAccountID,
			"activeName":          item.ActiveName,
			"active_name":         item.ActiveName,
			"description":         item.Description,
			"type":                item.Type,
			"startTime":           item.StartTime,
			"start_time":          item.StartTime,
			"endTime":             item.EndTime,
			"end_time":            item.EndTime,
			"tasks":               jsonPayload(item.TasksRaw),
			"employeeQrcode":      item.EmployeeQRCode,
			"employee_qrcode":     item.EmployeeQRCode,
			"contactTags":         jsonPayload(item.ContactTagsRaw),
			"contact_tags":        jsonPayload(item.ContactTagsRaw),
			"corpCardStatus":      item.CorpCardStatus,
			"corp_card_status":    item.CorpCardStatus,
			"corpCard":            jsonPayload(item.CorpCardRaw),
			"corp_card":           jsonPayload(item.CorpCardRaw),
			"status":              item.Status,
		}
	}
	return payload
}

func roomClockInContactPayload(item RoomClockInContactItem) map[string]any {
	return map[string]any{
		"id":              item.ID,
		"contactRecordId": item.ID,
		"clockInId":       item.ClockInID,
		"clock_in_id":     item.ClockInID,
		"unionId":         item.UnionID,
		"union_id":        item.UnionID,
		"openid":          item.OpenID,
		"nickname":        item.Nickname,
		"name":            item.Nickname,
		"avatar":          item.Avatar,
		"city":            item.City,
		"contactId":       item.ContactID,
		"contact_id":      item.ContactID,
		"employeeIds":     jsonPayload(item.EmployeeIDsRaw),
		"employee_ids":    jsonPayload(item.EmployeeIDsRaw),
		"contactTags":     jsonPayload(item.ContactTagsRaw),
		"contact_tags":    jsonPayload(item.ContactTagsRaw),
		"dayCount":        item.DayCount,
		"day_count":       item.DayCount,
		"status":          item.Status,
		"statusText":      roomClockInContactStatusText(item.Status),
		"receiveLevel":    item.ReceiveLevel,
		"receive_level":   item.ReceiveLevel,
		"writeOff":        item.WriteOff,
		"write_off":       item.WriteOff,
		"firstClockAt":    item.FirstClockAt,
		"first_clock_at":  item.FirstClockAt,
		"lastClockAt":     item.LastClockAt,
		"last_clock_at":   item.LastClockAt,
		"createdAt":       item.CreatedAt,
		"updatedAt":       item.UpdatedAt,
	}
}

func roomClockInDayPayload(item RoomClockInDayRecord) map[string]any {
	return map[string]any{
		"id":          item.ID,
		"clockInId":   item.ClockInID,
		"clock_in_id": item.ClockInID,
		"contactId":   item.ContactID,
		"contact_id":  item.ContactID,
		"unionId":     item.UnionID,
		"union_id":    item.UnionID,
		"day":         item.Day,
		"createdAt":   item.CreatedAt,
		"updatedAt":   item.UpdatedAt,
	}
}

func roomClockInStatusText(status int) string {
	switch status {
	case 0:
		return "已停用"
	case 2:
		return "已结束"
	default:
		return "进行中"
	}
}

func roomClockInContactStatusText(status int) string {
	switch status {
	case 1:
		return "已完成"
	default:
		return "未完成"
	}
}
