package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

type RoomRemindFilter struct {
	CorpID  int
	Name    string
	Keyword string
	Status  int
	Page    int
	PerPage int
}

type RoomRemindItem struct {
	ID             int
	Name           string
	RoomsRaw       string
	IsQrcode       int
	IsLink         int
	IsMiniprogram  int
	IsCard         int
	IsKeyword      int
	Keyword        string
	Status         int
	TenantID       int
	CorpID         int
	CreateUserID   int
	CreateUserName string
	RoomNum        int
	RecordNum      int
	CreatedAt      string
	UpdatedAt      string
}

type RoomRemindPage struct {
	Items     []RoomRemindItem
	Total     int
	TotalPage int
	Page      int
	PerPage   int
}

type RoomRemindWrite struct {
	CorpID           int
	CreateUserID     int
	Name             string
	HasName          bool
	RoomsRaw         string
	HasRooms         bool
	IsQrcode         int
	HasIsQrcode      bool
	IsLink           int
	HasIsLink        bool
	IsMiniprogram    int
	HasIsMiniprogram bool
	IsCard           int
	HasIsCard        bool
	IsKeyword        int
	HasIsKeyword     bool
	Keyword          string
	HasKeyword       bool
	Status           int
	HasStatus        bool
}

type RoomRemindStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	RoomRemindPage(ctx context.Context, filter RoomRemindFilter) (RoomRemindPage, error)
	RoomRemindByID(ctx context.Context, corpID int, id int) (RoomRemindItem, bool, error)
	CreateRoomRemind(ctx context.Context, values RoomRemindWrite) (int, error)
	UpdateRoomRemind(ctx context.Context, corpID int, id int, values RoomRemindWrite) (bool, error)
	UpdateRoomRemindStatus(ctx context.Context, corpID int, id int, status int) (bool, error)
	DeleteRoomRemind(ctx context.Context, corpID int, id int) (bool, error)
}

type RoomRemindHandler struct {
	store      RoomRemindStore
	cache      LoginCache
	resolver   UserIDResolver
	authorizer CorpAdminAuthorizer
}

func NewRoomRemindHandler(store RoomRemindStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer) *RoomRemindHandler {
	return &RoomRemindHandler{store: store, cache: cache, resolver: resolver, authorizer: authorizer}
}

func (h *RoomRemindHandler) Index(w http.ResponseWriter, r *http.Request) {
	h.writePage(w, r, "/dashboard/roomRemind/index#get", true, roomRemindFilterFromRequest(r, -1))
}

func (h *RoomRemindHandler) Task(w http.ResponseWriter, r *http.Request) {
	h.writePage(w, r, "", false, roomRemindFilterFromRequest(r, 1))
}

func (h *RoomRemindHandler) Store(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, user, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/roomRemind/store#post", true)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, 400, err.Error(), nil)
		return
	}
	values, err := roomRemindWriteFromParams(params, corpID, userID, true)
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
	if !enforceSaaSQuota(r.Context(), w, h.store, user.TenantID, SaaSMetricRoomReminds, 1) {
		return
	}
	id, err := h.store.CreateRoomRemind(r.Context(), values)
	if err != nil {
		if writeSaaSQuotaError(w, err) {
			return
		}
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if err := refreshSaaSUsageCounter(r.Context(), h.store, user.TenantID, SaaSMetricRoomReminds); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{id})
}

func (h *RoomRemindHandler) Update(w http.ResponseWriter, r *http.Request) {
	h.writeMutation(w, r, http.MethodPut, "/dashboard/roomRemind/update#put", func(ctx context.Context, userID int, corpID int, params map[string]any) (any, error) {
		id, err := roomRemindIDFromParams(params)
		if err != nil || id <= 0 {
			return nil, badRequestError("roomRemindId required")
		}
		values, err := roomRemindWriteFromParams(params, corpID, userID, false)
		if err != nil {
			return nil, err
		}
		updated, err := h.store.UpdateRoomRemind(ctx, corpID, id, values)
		if err != nil {
			return nil, err
		}
		if !updated {
			return nil, badRequestError("room remind not found")
		}
		return []any{id}, nil
	})
}

func (h *RoomRemindHandler) Status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/roomRemind/status#get", true)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	id := roomRemindIDFromQuery(r)
	if id <= 0 {
		writeEnvelope(w, http.StatusBadRequest, 400, "roomRemindId required", nil)
		return
	}
	status := lotteryQueryIntAllowZero(r, "status", -1)
	if status < 0 {
		writeEnvelope(w, http.StatusBadRequest, 400, "status required", nil)
		return
	}
	updated, err := h.store.UpdateRoomRemindStatus(r.Context(), corpID, id, status)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !updated {
		writeEnvelope(w, http.StatusBadRequest, 400, "room remind not found", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{id})
}

func (h *RoomRemindHandler) Destroy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, user, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/roomRemind/destroy#delete", true)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, 400, err.Error(), nil)
		return
	}
	id, err := roomRemindIDFromParams(params)
	if err != nil || id <= 0 {
		writeEnvelope(w, http.StatusBadRequest, 400, "roomRemindId required", nil)
		return
	}
	deleted, err := h.store.DeleteRoomRemind(r.Context(), corpID, id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !deleted {
		writeEnvelope(w, http.StatusBadRequest, 400, "room remind not found", nil)
		return
	}
	if err := refreshSaaSUsageCounter(r.Context(), h.store, user.TenantID, SaaSMetricRoomReminds); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{id})
}

func (h *RoomRemindHandler) Info(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/roomRemind/info#get", true)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	id := roomRemindIDFromQuery(r)
	if id <= 0 {
		writeEnvelope(w, http.StatusBadRequest, 400, "roomRemindId required", nil)
		return
	}
	item, found, err := h.store.RoomRemindByID(r.Context(), corpID, id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusNotFound, 404, "room remind not found", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", roomRemindPayload(item))
}

func (h *RoomRemindHandler) writePage(w http.ResponseWriter, r *http.Request, permission string, checkRBAC bool, filter RoomRemindFilter) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, _, ok := h.resolveAuthorized(w, r, permission, checkRBAC)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	filter.CorpID = corpID
	page, err := h.store.RoomRemindPage(r.Context(), filter)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, roomRemindPayload(item))
	}
	payload := contactBatchAddPagination(r, page.Page, page.PerPage, page.Total, list)
	payload["list"] = list
	payload["page"] = map[string]any{"perPage": page.PerPage, "total": page.Total, "totalPage": page.TotalPage}
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *RoomRemindHandler) writeMutation(w http.ResponseWriter, r *http.Request, method string, permission string, mutate func(context.Context, int, int, map[string]any) (any, error)) {
	if r.Method != method {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, _, _, ok := h.resolveAuthorized(w, r, permission, true)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
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

func (h *RoomRemindHandler) resolveAuthorized(w http.ResponseWriter, r *http.Request, permissionKey string, checkRBAC bool) (int, User, DashboardRequestScope, AccessContext, bool) {
	userID, user, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return 0, User{}, DashboardRequestScope{}, AccessContext{}, false
	}
	corpID, ok := principalCorpID(w, r)
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
	if checkRBAC && h.authorizer != nil {
		resolved, err := h.authorizer.Resolve(r.Context(), userID, permissionKey, corpID, employeeID)
		if err != nil {
			writeAccessError(w, err)
			return 0, User{}, DashboardRequestScope{}, AccessContext{}, false
		}
		access = resolved
	}
	return userID, user, principalScope, access, true
}

func (h *RoomRemindHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (int, User, DashboardRequestScope, bool) {
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

func roomRemindFilterFromRequest(r *http.Request, defaultStatus int) RoomRemindFilter {
	return RoomRemindFilter{
		Name:    lotteryQueryString(r, "name", "keyword"),
		Keyword: lotteryQueryString(r, "remindKeyword", "remind_keyword"),
		Status:  lotteryQueryIntAllowZero(r, "status", defaultStatus),
		Page:    positiveQueryInt(r, "page", 1),
		PerPage: positiveQueryInt(r, "perPage", 15),
	}
}

func roomRemindWriteFromParams(params map[string]any, corpID int, userID int, requireName bool) (RoomRemindWrite, error) {
	if nested, ok := roomRemindMapParam(params, "remind"); ok {
		for key, value := range params {
			if _, exists := nested[key]; !exists && key != "remind" {
				nested[key] = value
			}
		}
		params = nested
	}
	values := RoomRemindWrite{CorpID: corpID, CreateUserID: userID}
	if name, ok := firstStringParam(params, "name", "ruleName", "rule_name"); ok {
		values.Name = name
		values.HasName = true
	}
	if requireName && strings.TrimSpace(values.Name) == "" {
		return values, badRequestError("name required")
	}
	if raw, err := roomRemindRoomsFromParams(params); err != nil {
		return values, err
	} else if strings.TrimSpace(raw) != "" {
		values.RoomsRaw = raw
		values.HasRooms = true
	}
	if value, ok, err := firstIntParam(params, "isQrcode", "is_qrcode", "qrcode"); err != nil {
		return values, err
	} else if ok {
		values.IsQrcode = value
		values.HasIsQrcode = true
	}
	if value, ok, err := firstIntParam(params, "isLink", "is_link", "link"); err != nil {
		return values, err
	} else if ok {
		values.IsLink = value
		values.HasIsLink = true
	}
	if value, ok, err := firstIntParam(params, "isMiniprogram", "is_miniprogram", "miniprogram"); err != nil {
		return values, err
	} else if ok {
		values.IsMiniprogram = value
		values.HasIsMiniprogram = true
	}
	if value, ok, err := firstIntParam(params, "isCard", "is_card", "card"); err != nil {
		return values, err
	} else if ok {
		values.IsCard = value
		values.HasIsCard = true
	}
	if value, ok, err := firstIntParam(params, "isKeyword", "is_keyword", "keywordStatus", "keyword_status"); err != nil {
		return values, err
	} else if ok {
		values.IsKeyword = value
		values.HasIsKeyword = true
	}
	if keyword, ok := firstStringParam(params, "keyword", "remindKeyword", "remind_keyword"); ok {
		values.Keyword = keyword
		values.HasKeyword = true
		if strings.TrimSpace(keyword) != "" && !values.HasIsKeyword {
			values.IsKeyword = 1
			values.HasIsKeyword = true
		}
	}
	if status, ok, err := firstIntParam(params, "status"); err != nil {
		return values, err
	} else if ok {
		values.Status = status
		values.HasStatus = true
	}
	if requireName {
		if !values.HasRooms {
			values.RoomsRaw = "[]"
			values.HasRooms = true
		}
		if !values.HasKeyword {
			values.Keyword = ""
			values.HasKeyword = true
		}
		if !values.HasStatus {
			values.Status = 1
			values.HasStatus = true
		}
	}
	return values, nil
}

func roomRemindRoomsFromParams(params map[string]any) (string, error) {
	if raw, ok := firstJSONRawParam(params, "rooms", "roomIds", "room_ids"); ok {
		return raw, nil
	}
	if nested, ok := roomRemindMapParam(params, "room"); ok {
		raw, err := json.Marshal(nested)
		return string(raw), err
	}
	if value, ok := firstStringParam(params, "roomId", "room_id", "chatid", "wxChatId"); ok {
		raw, _ := json.Marshal([]string{value})
		return string(raw), nil
	}
	return "", nil
}

func roomRemindIDFromParams(params map[string]any) (int, error) {
	if id, has, err := firstPositiveIntParam(params, "roomRemindId", "room_remind_id", "remindId", "remind_id", "id"); err != nil || has {
		return id, err
	}
	if nested, ok := roomRemindMapParam(params, "remind"); ok {
		id, _, err := firstPositiveIntParam(nested, "roomRemindId", "room_remind_id", "remindId", "remind_id", "id")
		return id, err
	}
	return 0, nil
}

func roomRemindIDFromQuery(r *http.Request) int {
	for _, key := range []string{"roomRemindId", "room_remind_id", "remindId", "remind_id", "id"} {
		if value := positiveQueryInt(r, key, 0); value > 0 {
			return value
		}
	}
	return 0
}

func roomRemindMapParam(params map[string]any, key string) (map[string]any, bool) {
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

func roomRemindPayload(item RoomRemindItem) map[string]any {
	return map[string]any{
		"id":               item.ID,
		"roomRemindId":     item.ID,
		"room_remind_id":   item.ID,
		"remindId":         item.ID,
		"name":             item.Name,
		"rooms":            jsonPayload(item.RoomsRaw),
		"isQrcode":         item.IsQrcode,
		"is_qrcode":        item.IsQrcode,
		"isLink":           item.IsLink,
		"is_link":          item.IsLink,
		"isMiniprogram":    item.IsMiniprogram,
		"is_miniprogram":   item.IsMiniprogram,
		"isCard":           item.IsCard,
		"is_card":          item.IsCard,
		"isKeyword":        item.IsKeyword,
		"is_keyword":       item.IsKeyword,
		"keyword":          item.Keyword,
		"status":           item.Status,
		"statusText":       roomRemindStatusText(item.Status),
		"tenantId":         item.TenantID,
		"corpId":           item.CorpID,
		"createUserId":     item.CreateUserID,
		"createUserName":   item.CreateUserName,
		"roomNum":          item.RoomNum,
		"recordNum":        item.RecordNum,
		"messageRecordNum": item.RecordNum,
		"enabledTypes":     roomRemindEnabledTypes(item),
		"createdAt":        item.CreatedAt,
		"updatedAt":        item.UpdatedAt,
	}
}

func roomRemindEnabledTypes(item RoomRemindItem) []string {
	types := make([]string, 0, 5)
	if item.IsQrcode == 1 {
		types = append(types, "qrcode")
	}
	if item.IsLink == 1 {
		types = append(types, "link")
	}
	if item.IsMiniprogram == 1 {
		types = append(types, "miniprogram")
	}
	if item.IsCard == 1 {
		types = append(types, "card")
	}
	if item.IsKeyword == 1 {
		types = append(types, "keyword")
	}
	return types
}

func roomRemindStatusText(status int) string {
	if status == 0 {
		return "已停用"
	}
	return "已启用"
}
