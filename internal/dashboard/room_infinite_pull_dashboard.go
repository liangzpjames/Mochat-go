package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type RoomInfinitePullFilter struct {
	CorpID  int
	Name    string
	Page    int
	PerPage int
}

type RoomInfinitePullItem struct {
	ID             int
	Name           string
	Avatar         string
	TitleStatus    int
	Title          string
	DescribeStatus int
	Describe       string
	Logo           string
	QwCodeRaw      string
	TotalNum       int
	TenantID       int
	CorpID         int
	CreateUserID   int
	CreateUserName string
	CreatedAt      string
	UpdatedAt      string
}

type RoomInfinitePullPage struct {
	Items     []RoomInfinitePullItem
	Total     int
	TotalPage int
	Page      int
	PerPage   int
}

type RoomInfinitePullWrite struct {
	CorpID            int
	CreateUserID      int
	Name              string
	HasName           bool
	Avatar            string
	HasAvatar         bool
	TitleStatus       int
	HasTitleStatus    bool
	Title             string
	HasTitle          bool
	DescribeStatus    int
	HasDescribeStatus bool
	Describe          string
	HasDescribe       bool
	Logo              string
	HasLogo           bool
	QwCodeRaw         string
	HasQwCode         bool
	TotalNum          int
	HasTotalNum       bool
}

type RoomInfinitePullStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	RoomInfinitePullPage(ctx context.Context, filter RoomInfinitePullFilter) (RoomInfinitePullPage, error)
	RoomInfinitePullByID(ctx context.Context, corpID int, id int) (RoomInfinitePullItem, bool, error)
	CreateRoomInfinitePull(ctx context.Context, values RoomInfinitePullWrite) (int, error)
	UpdateRoomInfinitePull(ctx context.Context, corpID int, id int, values RoomInfinitePullWrite) (bool, error)
	DeleteRoomInfinitePull(ctx context.Context, corpID int, id int) (bool, error)
}

type RoomInfinitePullHandler struct {
	store      RoomInfinitePullStore
	cache      LoginCache
	resolver   UserIDResolver
	authorizer CorpAdminAuthorizer
}

func NewRoomInfinitePullHandler(store RoomInfinitePullStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer) *RoomInfinitePullHandler {
	return &RoomInfinitePullHandler{store: store, cache: cache, resolver: resolver, authorizer: authorizer}
}

func (h *RoomInfinitePullHandler) Index(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/roomInfinitePull/index#get")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	page, err := h.store.RoomInfinitePullPage(r.Context(), RoomInfinitePullFilter{
		CorpID:  corpID,
		Name:    lotteryQueryString(r, "name", "keyword"),
		Page:    positiveQueryInt(r, "page", 1),
		PerPage: positiveQueryInt(r, "perPage", 15),
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, roomInfinitePullPayload(r, item))
	}
	payload := contactBatchAddPagination(r, page.Page, page.PerPage, page.Total, list)
	payload["list"] = list
	payload["page"] = map[string]any{"perPage": page.PerPage, "total": page.Total, "totalPage": page.TotalPage}
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *RoomInfinitePullHandler) Store(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, user, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/roomInfinitePull/store#post")
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
	values, err := roomInfinitePullWriteFromParams(params, corpID, userID, true)
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
	if !enforceSaaSQuota(r.Context(), w, h.store, user.TenantID, SaaSMetricRoomInfinitePulls, 1) {
		return
	}
	id, err := h.store.CreateRoomInfinitePull(r.Context(), values)
	if err != nil {
		if writeSaaSQuotaError(w, err) {
			return
		}
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if err := refreshSaaSUsageCounter(r.Context(), h.store, user.TenantID, SaaSMetricRoomInfinitePulls); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{id})
}

func (h *RoomInfinitePullHandler) Update(w http.ResponseWriter, r *http.Request) {
	h.writeMutation(w, r, http.MethodPut, "/dashboard/roomInfinitePull/update#put", func(ctx context.Context, userID int, corpID int, params map[string]any) (any, error) {
		id, err := roomInfinitePullIDFromParams(params)
		if err != nil || id <= 0 {
			return nil, badRequestError("roomInfinitePullId required")
		}
		values, err := roomInfinitePullWriteFromParams(params, corpID, userID, false)
		if err != nil {
			return nil, err
		}
		updated, err := h.store.UpdateRoomInfinitePull(ctx, corpID, id, values)
		if err != nil {
			return nil, err
		}
		if !updated {
			return nil, badRequestError("room infinite pull not found")
		}
		return []any{id}, nil
	})
}

func (h *RoomInfinitePullHandler) Destroy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, user, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/roomInfinitePull/destroy#delete")
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
	id, err := roomInfinitePullIDFromParams(params)
	if err != nil || id <= 0 {
		writeEnvelope(w, http.StatusBadRequest, 400, "roomInfinitePullId required", nil)
		return
	}
	deleted, err := h.store.DeleteRoomInfinitePull(r.Context(), corpID, id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !deleted {
		writeEnvelope(w, http.StatusBadRequest, 400, "room infinite pull not found", nil)
		return
	}
	if err := refreshSaaSUsageCounter(r.Context(), h.store, user.TenantID, SaaSMetricRoomInfinitePulls); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{id})
}

func (h *RoomInfinitePullHandler) Info(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/roomInfinitePull/info#get")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	id := roomInfinitePullIDFromQuery(r)
	if id <= 0 {
		writeEnvelope(w, http.StatusBadRequest, 400, "roomInfinitePullId required", nil)
		return
	}
	item, found, err := h.store.RoomInfinitePullByID(r.Context(), corpID, id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusNotFound, 404, "room infinite pull not found", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", roomInfinitePullPayload(r, item))
}

func (h *RoomInfinitePullHandler) writeMutation(w http.ResponseWriter, r *http.Request, method string, permission string, mutate func(context.Context, int, int, map[string]any) (any, error)) {
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

func (h *RoomInfinitePullHandler) resolveAuthorized(w http.ResponseWriter, r *http.Request, permissionKey string) (int, User, DashboardRequestScope, AccessContext, bool) {
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

func (h *RoomInfinitePullHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (int, User, DashboardRequestScope, bool) {
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

func roomInfinitePullWriteFromParams(params map[string]any, corpID int, userID int, requireName bool) (RoomInfinitePullWrite, error) {
	if nested, ok := roomInfinitePullMapParam(params, "infinite"); ok {
		for key, value := range params {
			if _, exists := nested[key]; !exists && key != "infinite" {
				nested[key] = value
			}
		}
		params = nested
	}
	values := RoomInfinitePullWrite{CorpID: corpID, CreateUserID: userID}
	if name, ok := firstStringParam(params, "name", "titleName", "title_name"); ok {
		values.Name = name
		values.HasName = true
	}
	if requireName && strings.TrimSpace(values.Name) == "" {
		return values, badRequestError("name required")
	}
	if avatar, ok := firstStringParam(params, "avatar", "qrcodeAvatar", "qrcode_avatar"); ok {
		values.Avatar = avatar
		values.HasAvatar = true
	}
	if status, ok, err := firstIntParam(params, "titleStatus", "title_status"); err != nil {
		return values, err
	} else if ok {
		values.TitleStatus = status
		values.HasTitleStatus = true
	}
	if title, ok := firstStringParam(params, "title", "roomTitle", "room_title"); ok {
		values.Title = title
		values.HasTitle = true
	}
	if status, ok, err := firstIntParam(params, "describeStatus", "describe_status"); err != nil {
		return values, err
	} else if ok {
		values.DescribeStatus = status
		values.HasDescribeStatus = true
	}
	if describe, ok := firstStringParam(params, "describe", "description"); ok {
		values.Describe = describe
		values.HasDescribe = true
	}
	if logo, ok := firstStringParam(params, "logo"); ok {
		values.Logo = logo
		values.HasLogo = true
	}
	if raw, ok := firstJSONRawParam(params, "qwCode", "qw_code", "qrcode", "rooms"); ok {
		values.QwCodeRaw = raw
		values.HasQwCode = true
	}
	if total, ok, err := firstIntParam(params, "totalNum", "total_num"); err != nil {
		return values, err
	} else if ok {
		values.TotalNum = total
		values.HasTotalNum = true
	}
	if requireName {
		if !values.HasAvatar {
			values.Avatar = ""
			values.HasAvatar = true
		}
		if !values.HasTitleStatus {
			values.TitleStatus = 1
			values.HasTitleStatus = true
		}
		if !values.HasTitle {
			values.Title = ""
			values.HasTitle = true
		}
		if !values.HasDescribeStatus {
			values.DescribeStatus = 1
			values.HasDescribeStatus = true
		}
		if !values.HasDescribe {
			values.Describe = ""
			values.HasDescribe = true
		}
		if !values.HasLogo {
			values.Logo = ""
			values.HasLogo = true
		}
		if !values.HasQwCode {
			values.QwCodeRaw = "[]"
			values.HasQwCode = true
		}
	}
	return values, nil
}

func roomInfinitePullIDFromParams(params map[string]any) (int, error) {
	if id, has, err := firstPositiveIntParam(params, "roomInfinitePullId", "room_infinite_pull_id", "infiniteId", "infinite_id", "id"); err != nil || has {
		return id, err
	}
	if nested, ok := roomInfinitePullMapParam(params, "infinite"); ok {
		id, _, err := firstPositiveIntParam(nested, "roomInfinitePullId", "room_infinite_pull_id", "infiniteId", "infinite_id", "id")
		return id, err
	}
	return 0, nil
}

func roomInfinitePullIDFromQuery(r *http.Request) int {
	for _, key := range []string{"roomInfinitePullId", "room_infinite_pull_id", "infiniteId", "infinite_id", "id"} {
		if value := positiveQueryInt(r, key, 0); value > 0 {
			return value
		}
	}
	return 0
}

func roomInfinitePullMapParam(params map[string]any, key string) (map[string]any, bool) {
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

func roomInfinitePullPayload(r *http.Request, item RoomInfinitePullItem) map[string]any {
	return map[string]any{
		"id":                    item.ID,
		"roomInfinitePullId":    item.ID,
		"room_infinite_pull_id": item.ID,
		"infiniteId":            item.ID,
		"name":                  item.Name,
		"avatar":                item.Avatar,
		"titleStatus":           item.TitleStatus,
		"title_status":          item.TitleStatus,
		"title":                 item.Title,
		"describeStatus":        item.DescribeStatus,
		"describe_status":       item.DescribeStatus,
		"describe":              item.Describe,
		"logo":                  item.Logo,
		"qwCode":                jsonPayload(item.QwCodeRaw),
		"qw_code":               jsonPayload(item.QwCodeRaw),
		"totalNum":              item.TotalNum,
		"total_num":             item.TotalNum,
		"tenantId":              item.TenantID,
		"corpId":                item.CorpID,
		"createUserId":          item.CreateUserID,
		"createUserName":        item.CreateUserName,
		"createdAt":             item.CreatedAt,
		"created_at":            item.CreatedAt,
		"updatedAt":             item.UpdatedAt,
		"updated_at":            item.UpdatedAt,
		"link":                  roomInfinitePullLink(r, item.ID),
	}
}

func roomInfinitePullLink(r *http.Request, id int) string {
	path := "/roomInfinitePull?id=" + strconv.Itoa(id)
	if r == nil || strings.TrimSpace(r.Host) == "" {
		return path
	}
	scheme := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if scheme == "" {
		scheme = "http"
		if r.TLS != nil {
			scheme = "https"
		}
	}
	return scheme + "://" + r.Host + path
}
