package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

type RoomQualityFilter struct {
	CorpID  int
	Name    string
	Status  int
	Page    int
	PerPage int
}

type RoomQualityItem struct {
	ID             int
	Name           string
	Description    string
	RuleRaw        string
	RoomsRaw       string
	Status         int
	TenantID       int
	CorpID         int
	CreateUserID   int
	CreateUserName string
	RoomNum        int
	ContactNum     int
	CreatedAt      string
	UpdatedAt      string
}

type RoomQualityPage struct {
	Items     []RoomQualityItem
	Total     int
	TotalPage int
	Page      int
	PerPage   int
}

type RoomQualityWrite struct {
	CorpID         int
	CreateUserID   int
	Name           string
	HasName        bool
	Description    string
	HasDescription bool
	RuleRaw        string
	HasRule        bool
	RoomsRaw       string
	HasRooms       bool
	Status         int
	HasStatus      bool
}

type RoomQualityContactFilter struct {
	CorpID    int
	QualityID int
	RoomID    int
	Nickname  string
	Status    int
	Page      int
	PerPage   int
}

type RoomQualityContactItem struct {
	ID             int
	QualityID      int
	RoomID         int
	RoomName       string
	ContactID      int
	ExternalUserID string
	Nickname       string
	Avatar         string
	EmployeeIDsRaw string
	Content        string
	MsgType        string
	TriggerAt      string
	Status         int
	CreatedAt      string
	UpdatedAt      string
}

type RoomQualityContactPage struct {
	Items     []RoomQualityContactItem
	Total     int
	TotalPage int
	Page      int
	PerPage   int
}

type RoomQualityStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	RoomQualityPage(ctx context.Context, filter RoomQualityFilter) (RoomQualityPage, error)
	RoomQualityByID(ctx context.Context, corpID int, id int) (RoomQualityItem, bool, error)
	CreateRoomQuality(ctx context.Context, values RoomQualityWrite) (int, error)
	UpdateRoomQuality(ctx context.Context, corpID int, id int, values RoomQualityWrite) (bool, error)
	UpdateRoomQualityStatus(ctx context.Context, corpID int, id int, status int) (bool, error)
	DeleteRoomQuality(ctx context.Context, corpID int, id int) (bool, error)
	RoomQualityContactPage(ctx context.Context, filter RoomQualityContactFilter) (RoomQualityContactPage, error)
	RoomQualityContactByID(ctx context.Context, corpID int, id int) (RoomQualityContactItem, bool, error)
}

type RoomQualityHandler struct {
	store      RoomQualityStore
	cache      LoginCache
	resolver   UserIDResolver
	authorizer CorpAdminAuthorizer
}

func NewRoomQualityHandler(store RoomQualityStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer) *RoomQualityHandler {
	return &RoomQualityHandler{store: store, cache: cache, resolver: resolver, authorizer: authorizer}
}

func (h *RoomQualityHandler) Index(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/roomQuality/index#get")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	page, err := h.store.RoomQualityPage(r.Context(), RoomQualityFilter{
		CorpID:  corpID,
		Name:    lotteryQueryString(r, "name", "keyword"),
		Status:  lotteryQueryIntAllowZero(r, "status", -1),
		Page:    positiveQueryInt(r, "page", 1),
		PerPage: positiveQueryInt(r, "perPage", 15),
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, roomQualityPayload(item))
	}
	payload := contactBatchAddPagination(r, page.Page, page.PerPage, page.Total, list)
	payload["list"] = list
	payload["page"] = map[string]any{"perPage": page.PerPage, "total": page.Total, "totalPage": page.TotalPage}
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *RoomQualityHandler) Store(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, user, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/roomQuality/store#post")
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
	values, err := roomQualityWriteFromParams(params, corpID, userID, true)
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
	if !enforceSaaSQuota(r.Context(), w, h.store, user.TenantID, SaaSMetricRoomQualities, 1) {
		return
	}
	id, err := h.store.CreateRoomQuality(r.Context(), values)
	if err != nil {
		if writeSaaSQuotaError(w, err) {
			return
		}
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if err := refreshSaaSUsageCounter(r.Context(), h.store, user.TenantID, SaaSMetricRoomQualities); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{id})
}

func (h *RoomQualityHandler) Update(w http.ResponseWriter, r *http.Request) {
	h.writeMutation(w, r, http.MethodPut, "/dashboard/roomQuality/update#put", func(ctx context.Context, userID int, corpID int, params map[string]any) (any, error) {
		id, err := roomQualityIDFromParams(params)
		if err != nil || id <= 0 {
			return nil, badRequestError("roomQualityId required")
		}
		values, err := roomQualityWriteFromParams(params, corpID, userID, false)
		if err != nil {
			return nil, err
		}
		updated, err := h.store.UpdateRoomQuality(ctx, corpID, id, values)
		if err != nil {
			return nil, err
		}
		if !updated {
			return nil, badRequestError("room quality not found")
		}
		return []any{id}, nil
	})
}

func (h *RoomQualityHandler) Status(w http.ResponseWriter, r *http.Request) {
	h.writeMutation(w, r, http.MethodPut, "/dashboard/roomQuality/status#put", func(ctx context.Context, _ int, corpID int, params map[string]any) (any, error) {
		id, err := roomQualityIDFromParams(params)
		if err != nil || id <= 0 {
			return nil, badRequestError("roomQualityId required")
		}
		status, ok, err := firstIntParam(params, "status")
		if err != nil || !ok {
			return nil, badRequestError("status required")
		}
		updated, err := h.store.UpdateRoomQualityStatus(ctx, corpID, id, status)
		if err != nil {
			return nil, err
		}
		if !updated {
			return nil, badRequestError("room quality not found")
		}
		return []any{id}, nil
	})
}

func (h *RoomQualityHandler) Destroy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, user, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/roomQuality/destroy#delete")
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
	id, err := roomQualityIDFromParams(params)
	if err != nil || id <= 0 {
		writeEnvelope(w, http.StatusBadRequest, 400, "roomQualityId required", nil)
		return
	}
	deleted, err := h.store.DeleteRoomQuality(r.Context(), corpID, id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !deleted {
		writeEnvelope(w, http.StatusBadRequest, 400, "room quality not found", nil)
		return
	}
	if err := refreshSaaSUsageCounter(r.Context(), h.store, user.TenantID, SaaSMetricRoomQualities); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{id})
}

func (h *RoomQualityHandler) Info(w http.ResponseWriter, r *http.Request) {
	h.show(w, r, "/dashboard/roomQuality/info#get")
}

func (h *RoomQualityHandler) ShowContact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/roomQuality/showContact#get")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	page, err := h.store.RoomQualityContactPage(r.Context(), RoomQualityContactFilter{
		CorpID:    corpID,
		QualityID: roomQualityIDFromQuery(r),
		RoomID:    positiveQueryInt(r, "roomId", 0),
		Nickname:  lotteryQueryString(r, "nickname", "name", "contactName"),
		Status:    lotteryQueryIntAllowZero(r, "status", -1),
		Page:      positiveQueryInt(r, "page", 1),
		PerPage:   positiveQueryInt(r, "perPage", 15),
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, roomQualityContactPayload(item))
	}
	payload := contactBatchAddPagination(r, page.Page, page.PerPage, page.Total, list)
	payload["list"] = list
	payload["page"] = map[string]any{"perPage": page.PerPage, "total": page.Total, "totalPage": page.TotalPage}
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *RoomQualityHandler) ContactDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/roomQuality/contactDetail#get")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	id := positiveQueryInt(r, "contactRecordId", 0)
	if id <= 0 {
		id = positiveQueryInt(r, "id", 0)
	}
	if id <= 0 {
		writeEnvelope(w, http.StatusBadRequest, 400, "contactRecordId required", nil)
		return
	}
	item, found, err := h.store.RoomQualityContactByID(r.Context(), corpID, id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusNotFound, 404, "room quality contact not found", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", roomQualityContactPayload(item))
}

func (h *RoomQualityHandler) show(w http.ResponseWriter, r *http.Request, permission string) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, _, ok := h.resolveAuthorized(w, r, permission)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	id := roomQualityIDFromQuery(r)
	if id <= 0 {
		writeEnvelope(w, http.StatusBadRequest, 400, "roomQualityId required", nil)
		return
	}
	item, found, err := h.store.RoomQualityByID(r.Context(), corpID, id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusNotFound, 404, "room quality not found", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", roomQualityPayload(item))
}

func (h *RoomQualityHandler) writeMutation(w http.ResponseWriter, r *http.Request, method string, permission string, mutate func(context.Context, int, int, map[string]any) (any, error)) {
	if r.Method != method {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, _, _, ok := h.resolveAuthorized(w, r, permission)
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

func (h *RoomQualityHandler) resolveAuthorized(w http.ResponseWriter, r *http.Request, permissionKey string) (int, User, DashboardRequestScope, AccessContext, bool) {
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

func (h *RoomQualityHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (int, User, DashboardRequestScope, bool) {
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

func roomQualityWriteFromParams(params map[string]any, corpID int, userID int, requireName bool) (RoomQualityWrite, error) {
	if nested, ok := roomQualityMapParam(params, "quality"); ok {
		for key, value := range params {
			if _, exists := nested[key]; !exists && key != "quality" {
				nested[key] = value
			}
		}
		params = nested
	}
	values := RoomQualityWrite{CorpID: corpID, CreateUserID: userID}
	if name, ok := firstStringParam(params, "name", "ruleName", "rule_name"); ok {
		values.Name = name
		values.HasName = true
	}
	if requireName && strings.TrimSpace(values.Name) == "" {
		return values, badRequestError("name required")
	}
	if description, ok := firstStringParam(params, "description", "desc"); ok {
		values.Description = description
		values.HasDescription = true
	}
	if raw, ok := firstJSONRawParam(params, "rule", "rules"); ok {
		values.RuleRaw = raw
		values.HasRule = true
	}
	if raw, ok := firstJSONRawParam(params, "rooms", "roomIds", "room_ids"); ok {
		values.RoomsRaw = raw
		values.HasRooms = true
	}
	if status, ok, err := firstIntParam(params, "status"); err != nil {
		return values, err
	} else if ok {
		values.Status = status
		values.HasStatus = true
	}
	if requireName {
		if !values.HasRule {
			values.RuleRaw = "[]"
			values.HasRule = true
		}
		if !values.HasRooms {
			values.RoomsRaw = "[]"
			values.HasRooms = true
		}
		if !values.HasStatus {
			values.Status = 1
			values.HasStatus = true
		}
	}
	return values, nil
}

func roomQualityIDFromParams(params map[string]any) (int, error) {
	if id, has, err := firstPositiveIntParam(params, "roomQualityId", "room_quality_id", "qualityId", "quality_id", "id"); err != nil || has {
		return id, err
	}
	if nested, ok := roomQualityMapParam(params, "quality"); ok {
		id, _, err := firstPositiveIntParam(nested, "roomQualityId", "room_quality_id", "qualityId", "quality_id", "id")
		return id, err
	}
	return 0, nil
}

func roomQualityIDFromQuery(r *http.Request) int {
	for _, key := range []string{"roomQualityId", "room_quality_id", "qualityId", "quality_id", "id"} {
		if value := positiveQueryInt(r, key, 0); value > 0 {
			return value
		}
	}
	return 0
}

func roomQualityMapParam(params map[string]any, key string) (map[string]any, bool) {
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

func roomQualityPayload(item RoomQualityItem) map[string]any {
	return map[string]any{
		"id":              item.ID,
		"roomQualityId":   item.ID,
		"room_quality_id": item.ID,
		"qualityId":       item.ID,
		"name":            item.Name,
		"description":     item.Description,
		"rule":            jsonPayload(item.RuleRaw),
		"rooms":           jsonPayload(item.RoomsRaw),
		"status":          item.Status,
		"statusText":      roomQualityStatusText(item.Status),
		"tenantId":        item.TenantID,
		"corpId":          item.CorpID,
		"createUserId":    item.CreateUserID,
		"createUserName":  item.CreateUserName,
		"roomNum":         item.RoomNum,
		"contactNum":      item.ContactNum,
		"createdAt":       item.CreatedAt,
		"updatedAt":       item.UpdatedAt,
	}
}

func roomQualityContactPayload(item RoomQualityContactItem) map[string]any {
	return map[string]any{
		"id":              item.ID,
		"contactRecordId": item.ID,
		"roomQualityId":   item.QualityID,
		"qualityId":       item.QualityID,
		"roomId":          item.RoomID,
		"roomName":        item.RoomName,
		"contactId":       item.ContactID,
		"externalUserId":  item.ExternalUserID,
		"nickname":        item.Nickname,
		"name":            item.Nickname,
		"avatar":          item.Avatar,
		"employeeIds":     jsonPayload(item.EmployeeIDsRaw),
		"content":         item.Content,
		"msgType":         item.MsgType,
		"triggerAt":       item.TriggerAt,
		"status":          item.Status,
		"createdAt":       item.CreatedAt,
		"updatedAt":       item.UpdatedAt,
	}
}

func roomQualityStatusText(status int) string {
	if status == 0 {
		return "已停用"
	}
	return "已启用"
}
