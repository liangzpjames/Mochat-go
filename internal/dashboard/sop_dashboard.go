package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type ContactSOPDashboardFilter struct {
	CorpID  int
	Name    string
	Page    int
	PerPage int
}

type ContactSOPDashboardItem struct {
	ID             int
	CorpID         int
	CreatorID      int
	CreatorName    string
	Name           string
	SettingRaw     string
	EmployeeIDsRaw string
	ContactIDsRaw  string
	State          int
	CreatedAt      string
	UpdatedAt      string
}

type ContactSOPDashboardPage struct {
	Items     []ContactSOPDashboardItem
	Total     int
	TotalPage int
	PerPage   int
	Page      int
}

type ContactSOPDashboardWrite struct {
	CorpID         int
	CreatorID      int
	Name           string
	HasName        bool
	SettingRaw     string
	HasSetting     bool
	EmployeeIDsRaw string
	HasEmployeeIDs bool
	ContactIDsRaw  string
	HasContactIDs  bool
	State          int
	HasState       bool
}

type RoomSOPDashboardFilter struct {
	CorpID  int
	Name    string
	Page    int
	PerPage int
}

type RoomSOPDashboardItem struct {
	ID          int
	CorpID      int
	CreatorID   int
	CreatorName string
	Name        string
	SettingRaw  string
	RoomIDsRaw  string
	State       int
	CreatedAt   string
	UpdatedAt   string
}

type RoomSOPDashboardPage struct {
	Items     []RoomSOPDashboardItem
	Total     int
	TotalPage int
	PerPage   int
	Page      int
}

type RoomSOPDashboardWrite struct {
	CorpID     int
	CreatorID  int
	Name       string
	HasName    bool
	SettingRaw string
	HasSetting bool
	RoomIDsRaw string
	HasRoomIDs bool
	State      int
	HasState   bool
}

type SOPDashboardStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	ContactSOPDashboardPage(ctx context.Context, filter ContactSOPDashboardFilter) (ContactSOPDashboardPage, error)
	ContactSOPDashboardByID(ctx context.Context, corpID int, id int) (ContactSOPDashboardItem, bool, error)
	CreateContactSOPDashboard(ctx context.Context, values ContactSOPDashboardWrite) (int, error)
	UpdateContactSOPDashboard(ctx context.Context, corpID int, id int, values ContactSOPDashboardWrite) (bool, error)
	UpdateContactSOPDashboardEmployees(ctx context.Context, corpID int, id int, employeeIDsRaw string) (bool, error)
	UpdateContactSOPDashboardState(ctx context.Context, corpID int, id int, state int) (bool, error)
	DeleteContactSOPDashboard(ctx context.Context, corpID int, id int) (bool, error)
	RoomSOPDashboardPage(ctx context.Context, filter RoomSOPDashboardFilter) (RoomSOPDashboardPage, error)
	RoomSOPDashboardByID(ctx context.Context, corpID int, id int) (RoomSOPDashboardItem, bool, error)
	CreateRoomSOPDashboard(ctx context.Context, values RoomSOPDashboardWrite) (int, error)
	UpdateRoomSOPDashboard(ctx context.Context, corpID int, id int, values RoomSOPDashboardWrite) (bool, error)
	UpdateRoomSOPDashboardRooms(ctx context.Context, corpID int, id int, roomIDsRaw string) (bool, error)
	UpdateRoomSOPDashboardState(ctx context.Context, corpID int, id int, state int) (bool, error)
	DeleteRoomSOPDashboard(ctx context.Context, corpID int, id int) (bool, error)
}

type SOPDashboardHandler struct {
	store      SOPDashboardStore
	cache      LoginCache
	resolver   UserIDResolver
	authorizer CorpAdminAuthorizer
}

func NewSOPDashboardHandler(store SOPDashboardStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer) *SOPDashboardHandler {
	return &SOPDashboardHandler{store: store, cache: cache, resolver: resolver, authorizer: authorizer}
}

func (h *SOPDashboardHandler) ContactIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/contactSop/index#get")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	page, err := h.store.ContactSOPDashboardPage(r.Context(), ContactSOPDashboardFilter{
		CorpID:  corpID,
		Name:    sopQueryName(r),
		Page:    positiveQueryInt(r, "page", 1),
		PerPage: positiveQueryInt(r, "perPage", 15),
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, contactSOPDashboardPayload(item))
	}
	payload := contactBatchAddPagination(r, page.Page, page.PerPage, page.Total, list)
	payload["list"] = list
	payload["page"] = map[string]any{"perPage": page.PerPage, "total": page.Total, "totalPage": page.TotalPage}
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *SOPDashboardHandler) ContactStore(w http.ResponseWriter, r *http.Request) {
	h.writeContactMutation(w, r, http.MethodPost, "/dashboard/contactSop/store#post", func(ctx context.Context, user User, corpID int, params map[string]any) (any, error) {
		values, err := contactSOPWriteFromParams(params, corpID, user.ID, true)
		if err != nil {
			return nil, err
		}
		if err := requireSaaSQuota(ctx, h.store, user.TenantID, SaaSMetricContactSOPs, 1); err != nil {
			return nil, err
		}
		id, err := h.store.CreateContactSOPDashboard(ctx, values)
		if err != nil {
			return nil, err
		}
		if err := refreshSaaSUsageCounter(ctx, h.store, user.TenantID, SaaSMetricContactSOPs); err != nil {
			return nil, err
		}
		return []any{id}, nil
	})
}

func (h *SOPDashboardHandler) ContactSetEmployee(w http.ResponseWriter, r *http.Request) {
	h.writeContactMutation(w, r, http.MethodPut, "/dashboard/contactSop/setEmployee#put", func(ctx context.Context, _ User, corpID int, params map[string]any) (any, error) {
		id, err := contactSOPIDFromRequest(r, params)
		if err != nil {
			return nil, err
		}
		employeeIDsRaw, found, err := sopRawJSONParam(params, "employeeIds", "employee_ids", "employeeId", "employee_id", "employees", "employeeList", "employee")
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, badRequestError("employeeIds required")
		}
		ok, err := h.store.UpdateContactSOPDashboardEmployees(ctx, corpID, id, employeeIDsRaw)
		return nil, sopRequireFound(ok, err, "个人SOP不存在")
	})
}

func (h *SOPDashboardHandler) ContactState(w http.ResponseWriter, r *http.Request) {
	h.writeContactMutation(w, r, http.MethodPut, "/dashboard/contactSop/state#put", func(ctx context.Context, _ User, corpID int, params map[string]any) (any, error) {
		id, err := contactSOPIDFromRequest(r, params)
		if err != nil {
			return nil, err
		}
		state, found, err := sopStateParam(params, -1)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, badRequestError("state required")
		}
		ok, err := h.store.UpdateContactSOPDashboardState(ctx, corpID, id, state)
		return nil, sopRequireFound(ok, err, "个人SOP不存在")
	})
}

func (h *SOPDashboardHandler) ContactInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/contactSop/info#get")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	id := positiveQueryInt(r, "contactSopId", 0)
	if id <= 0 {
		id = positiveQueryInt(r, "id", 0)
	}
	if id <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "contactSopId required", nil)
		return
	}
	item, found, err := h.store.ContactSOPDashboardByID(r.Context(), corpID, id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "个人SOP不存在", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", contactSOPDashboardPayload(item))
}

func (h *SOPDashboardHandler) ContactDestroy(w http.ResponseWriter, r *http.Request) {
	h.writeContactMutation(w, r, http.MethodDelete, "/dashboard/contactSop/destroy#delete", func(ctx context.Context, user User, corpID int, params map[string]any) (any, error) {
		id, err := contactSOPIDFromRequest(r, params)
		if err != nil {
			return nil, err
		}
		ok, err := h.store.DeleteContactSOPDashboard(ctx, corpID, id)
		if err := sopRequireFound(ok, err, "个人SOP不存在"); err != nil {
			return nil, err
		}
		if err := refreshSaaSUsageCounter(ctx, h.store, user.TenantID, SaaSMetricContactSOPs); err != nil {
			return nil, err
		}
		return nil, nil
	})
}

func (h *SOPDashboardHandler) ContactUpdate(w http.ResponseWriter, r *http.Request) {
	h.writeContactMutation(w, r, http.MethodPut, "/dashboard/contactSop/update#put", func(ctx context.Context, user User, corpID int, params map[string]any) (any, error) {
		id, err := contactSOPIDFromRequest(r, params)
		if err != nil {
			return nil, err
		}
		values, err := contactSOPWriteFromParams(params, corpID, user.ID, false)
		if err != nil {
			return nil, err
		}
		ok, err := h.store.UpdateContactSOPDashboard(ctx, corpID, id, values)
		return nil, sopRequireFound(ok, err, "个人SOP不存在")
	})
}

func (h *SOPDashboardHandler) RoomIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/roomSop/index#get")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	page, err := h.store.RoomSOPDashboardPage(r.Context(), RoomSOPDashboardFilter{
		CorpID:  corpID,
		Name:    sopQueryName(r),
		Page:    positiveQueryInt(r, "page", 1),
		PerPage: positiveQueryInt(r, "perPage", 15),
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, roomSOPDashboardPayload(item))
	}
	payload := contactBatchAddPagination(r, page.Page, page.PerPage, page.Total, list)
	payload["list"] = list
	payload["page"] = map[string]any{"perPage": page.PerPage, "total": page.Total, "totalPage": page.TotalPage}
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *SOPDashboardHandler) RoomStore(w http.ResponseWriter, r *http.Request) {
	h.writeRoomMutation(w, r, http.MethodPost, "/dashboard/roomSop/store#post", func(ctx context.Context, user User, corpID int, params map[string]any) (any, error) {
		values, err := roomSOPWriteFromParams(params, corpID, user.ID, true)
		if err != nil {
			return nil, err
		}
		if err := requireSaaSQuota(ctx, h.store, user.TenantID, SaaSMetricRoomSOPs, 1); err != nil {
			return nil, err
		}
		id, err := h.store.CreateRoomSOPDashboard(ctx, values)
		if err != nil {
			return nil, err
		}
		if err := refreshSaaSUsageCounter(ctx, h.store, user.TenantID, SaaSMetricRoomSOPs); err != nil {
			return nil, err
		}
		return []any{id}, nil
	})
}

func (h *SOPDashboardHandler) RoomSetRoom(w http.ResponseWriter, r *http.Request) {
	h.writeRoomMutation(w, r, http.MethodPut, "/dashboard/roomSop/setRoom#put", func(ctx context.Context, _ User, corpID int, params map[string]any) (any, error) {
		id, err := roomSOPIDFromRequest(r, params)
		if err != nil {
			return nil, err
		}
		roomIDsRaw, found, err := sopRawJSONParam(params, "roomIds", "room_ids", "roomId", "room_id", "rooms", "roomList", "room")
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, badRequestError("roomIds required")
		}
		ok, err := h.store.UpdateRoomSOPDashboardRooms(ctx, corpID, id, roomIDsRaw)
		return nil, sopRequireFound(ok, err, "群SOP不存在")
	})
}

func (h *SOPDashboardHandler) RoomState(w http.ResponseWriter, r *http.Request) {
	h.writeRoomMutation(w, r, http.MethodPut, "/dashboard/roomSop/state#put", func(ctx context.Context, _ User, corpID int, params map[string]any) (any, error) {
		id, err := roomSOPIDFromRequest(r, params)
		if err != nil {
			return nil, err
		}
		state, found, err := sopStateParam(params, -1)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, badRequestError("state required")
		}
		ok, err := h.store.UpdateRoomSOPDashboardState(ctx, corpID, id, state)
		return nil, sopRequireFound(ok, err, "群SOP不存在")
	})
}

func (h *SOPDashboardHandler) RoomInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/roomSop/info#get")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	id := positiveQueryInt(r, "roomSopId", 0)
	if id <= 0 {
		id = positiveQueryInt(r, "id", 0)
	}
	if id <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "roomSopId required", nil)
		return
	}
	item, found, err := h.store.RoomSOPDashboardByID(r.Context(), corpID, id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "群SOP不存在", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", roomSOPDashboardPayload(item))
}

func (h *SOPDashboardHandler) RoomDestroy(w http.ResponseWriter, r *http.Request) {
	h.writeRoomMutation(w, r, http.MethodDelete, "/dashboard/roomSop/destroy#delete", func(ctx context.Context, user User, corpID int, params map[string]any) (any, error) {
		id, err := roomSOPIDFromRequest(r, params)
		if err != nil {
			return nil, err
		}
		ok, err := h.store.DeleteRoomSOPDashboard(ctx, corpID, id)
		if err := sopRequireFound(ok, err, "群SOP不存在"); err != nil {
			return nil, err
		}
		if err := refreshSaaSUsageCounter(ctx, h.store, user.TenantID, SaaSMetricRoomSOPs); err != nil {
			return nil, err
		}
		return nil, nil
	})
}

func (h *SOPDashboardHandler) RoomUpdate(w http.ResponseWriter, r *http.Request) {
	h.writeRoomMutation(w, r, http.MethodPut, "/dashboard/roomSop/update#put", func(ctx context.Context, user User, corpID int, params map[string]any) (any, error) {
		id, err := roomSOPIDFromRequest(r, params)
		if err != nil {
			return nil, err
		}
		values, err := roomSOPWriteFromParams(params, corpID, user.ID, false)
		if err != nil {
			return nil, err
		}
		ok, err := h.store.UpdateRoomSOPDashboard(ctx, corpID, id, values)
		return nil, sopRequireFound(ok, err, "群SOP不存在")
	})
}

func (h *SOPDashboardHandler) writeContactMutation(w http.ResponseWriter, r *http.Request, method string, permissionKey string, action func(context.Context, User, int, map[string]any) (any, error)) {
	h.writeSOPMutation(w, r, method, permissionKey, action)
}

func (h *SOPDashboardHandler) writeRoomMutation(w http.ResponseWriter, r *http.Request, method string, permissionKey string, action func(context.Context, User, int, map[string]any) (any, error)) {
	h.writeSOPMutation(w, r, method, permissionKey, action)
}

func (h *SOPDashboardHandler) writeSOPMutation(w http.ResponseWriter, r *http.Request, method string, permissionKey string, action func(context.Context, User, int, map[string]any) (any, error)) {
	if r.Method != method {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, user, _, _, ok := h.resolveAuthorized(w, r, permissionKey)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "参数错误", nil)
		return
	}
	data, err := action(r.Context(), user, corpID, params)
	if err != nil {
		if isBadRequestError(err) {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
			return
		}
		if writeSaaSQuotaError(w, err) {
			return
		}
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if data == nil {
		data = []any{}
	}
	writeEnvelope(w, http.StatusOK, 200, "success", data)
}

func (h *SOPDashboardHandler) resolveAuthorized(w http.ResponseWriter, r *http.Request, permissionKey string) (int, User, DashboardRequestScope, AccessContext, bool) {
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

func (h *SOPDashboardHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (int, User, DashboardRequestScope, bool) {
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
	return userID, user, DashboardRequestScope(principalScope), true
}

func contactSOPWriteFromParams(params map[string]any, corpID int, creatorID int, requireName bool) (ContactSOPDashboardWrite, error) {
	values := ContactSOPDashboardWrite{CorpID: corpID, CreatorID: creatorID}
	if name, found := sopStringParam(params, "name", "sopName", "title"); found {
		values.Name = name
		values.HasName = true
	}
	if requireName && strings.TrimSpace(values.Name) == "" {
		return ContactSOPDashboardWrite{}, badRequestError("name required")
	}
	if raw, found, err := sopRawJSONParam(params, "setting", "settings", "rule", "rules", "content", "list"); err != nil {
		return ContactSOPDashboardWrite{}, err
	} else if found {
		values.SettingRaw = raw
		values.HasSetting = true
	}
	if raw, found, err := sopRawJSONParam(params, "employeeIds", "employee_ids", "employeeId", "employee_id", "employees", "employeeList", "employee"); err != nil {
		return ContactSOPDashboardWrite{}, err
	} else if found {
		values.EmployeeIDsRaw = raw
		values.HasEmployeeIDs = true
	}
	if raw, found, err := sopRawJSONParam(params, "contactIds", "contact_ids", "contactId", "contact_id", "contacts", "contactList", "contact"); err != nil {
		return ContactSOPDashboardWrite{}, err
	} else if found {
		values.ContactIDsRaw = raw
		values.HasContactIDs = true
	}
	defaultState := -1
	if requireName {
		defaultState = 1
	}
	if state, found, err := sopStateParam(params, defaultState); err != nil {
		return ContactSOPDashboardWrite{}, err
	} else if found {
		values.State = state
		values.HasState = true
	}
	return values, nil
}

func roomSOPWriteFromParams(params map[string]any, corpID int, creatorID int, requireName bool) (RoomSOPDashboardWrite, error) {
	values := RoomSOPDashboardWrite{CorpID: corpID, CreatorID: creatorID}
	if name, found := sopStringParam(params, "name", "sopName", "title"); found {
		values.Name = name
		values.HasName = true
	}
	if requireName && strings.TrimSpace(values.Name) == "" {
		return RoomSOPDashboardWrite{}, badRequestError("name required")
	}
	if raw, found, err := sopRawJSONParam(params, "setting", "settings", "rule", "rules", "content", "list"); err != nil {
		return RoomSOPDashboardWrite{}, err
	} else if found {
		values.SettingRaw = raw
		values.HasSetting = true
	}
	if raw, found, err := sopRawJSONParam(params, "roomIds", "room_ids", "roomId", "room_id", "rooms", "roomList", "room"); err != nil {
		return RoomSOPDashboardWrite{}, err
	} else if found {
		values.RoomIDsRaw = raw
		values.HasRoomIDs = true
	}
	defaultState := -1
	if requireName {
		defaultState = 1
	}
	if state, found, err := sopStateParam(params, defaultState); err != nil {
		return RoomSOPDashboardWrite{}, err
	} else if found {
		values.State = state
		values.HasState = true
	}
	return values, nil
}

func contactSOPIDFromRequest(r *http.Request, params map[string]any) (int, error) {
	id, _, err := firstPositiveIntParam(params, "contactSopId", "contact_sop_id", "sopId", "id")
	if err != nil {
		return 0, badRequestError("contactSopId invalid")
	}
	if id <= 0 {
		id = positiveQueryInt(r, "contactSopId", 0)
	}
	if id <= 0 {
		id = positiveQueryInt(r, "id", 0)
	}
	if id <= 0 {
		return 0, badRequestError("contactSopId required")
	}
	return id, nil
}

func roomSOPIDFromRequest(r *http.Request, params map[string]any) (int, error) {
	id, _, err := firstPositiveIntParam(params, "roomSopId", "room_sop_id", "sopId", "id")
	if err != nil {
		return 0, badRequestError("roomSopId invalid")
	}
	if id <= 0 {
		id = positiveQueryInt(r, "roomSopId", 0)
	}
	if id <= 0 {
		id = positiveQueryInt(r, "id", 0)
	}
	if id <= 0 {
		return 0, badRequestError("roomSopId required")
	}
	return id, nil
}

func sopStateParam(params map[string]any, defaultValue int) (int, bool, error) {
	for _, key := range []string{"state", "status"} {
		state, found, err := intParam(params, key)
		if err != nil {
			return 0, found, badRequestError(key + " invalid")
		}
		if found {
			return state, true, nil
		}
	}
	if defaultValue >= 0 {
		return defaultValue, true, nil
	}
	return 0, false, nil
}

func sopStringParam(params map[string]any, keys ...string) (string, bool) {
	for _, key := range keys {
		if _, ok := params[key]; !ok {
			continue
		}
		return stringParam(params, key), true
	}
	return "", false
}

func sopRawJSONParam(params map[string]any, keys ...string) (string, bool, error) {
	for _, key := range keys {
		value, ok := params[key]
		if !ok {
			continue
		}
		raw, err := sopRawJSONValue(value)
		return raw, true, err
	}
	return "", false, nil
}

func sopRawJSONValue(value any) (string, error) {
	if value == nil {
		return "[]", nil
	}
	switch typed := value.(type) {
	case string:
		raw := strings.TrimSpace(typed)
		if raw == "" {
			return "[]", nil
		}
		if json.Valid([]byte(raw)) {
			return raw, nil
		}
		if ints, ok := sopCSVInts(raw); ok {
			encoded, _ := json.Marshal(ints)
			return string(encoded), nil
		}
		encoded, err := json.Marshal(raw)
		return string(encoded), err
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return "", badRequestError("JSON 格式错误")
		}
		return string(encoded), nil
	}
}

func sopCSVInts(raw string) ([]int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []int{}, true
	}
	parts := strings.Split(raw, ",")
	values := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		value, err := strconv.Atoi(part)
		if err != nil || value <= 0 {
			return nil, false
		}
		values = append(values, value)
	}
	return values, true
}

func sopRequireFound(found bool, err error, message string) error {
	if err != nil {
		return err
	}
	if !found {
		return badRequestError(message)
	}
	return nil
}

func sopQueryName(r *http.Request) string {
	for _, key := range []string{"name", "searchKey", "keyword", "keyWords"} {
		if value := strings.TrimSpace(r.URL.Query().Get(key)); value != "" {
			return value
		}
	}
	return ""
}

func contactSOPDashboardPayload(item ContactSOPDashboardItem) map[string]any {
	setting := sopJSONPayload(item.SettingRaw)
	employeeIDs := sopJSONPayload(item.EmployeeIDsRaw)
	contactIDs := sopJSONPayload(item.ContactIDsRaw)
	return map[string]any{
		"id":             item.ID,
		"contactSopId":   item.ID,
		"contact_sop_id": item.ID,
		"corpId":         item.CorpID,
		"corp_id":        item.CorpID,
		"creatorId":      item.CreatorID,
		"creator_id":     item.CreatorID,
		"creatorName":    item.CreatorName,
		"creator":        item.CreatorName,
		"name":           item.Name,
		"setting":        setting,
		"settingRaw":     item.SettingRaw,
		"employeeIds":    employeeIDs,
		"employee_ids":   employeeIDs,
		"contactIds":     contactIDs,
		"contact_ids":    contactIDs,
		"employeeNum":    sopJSONArrayLen(item.EmployeeIDsRaw),
		"contactNum":     sopJSONArrayLen(item.ContactIDsRaw),
		"state":          item.State,
		"status":         item.State,
		"createdAt":      item.CreatedAt,
		"created_at":     item.CreatedAt,
		"updatedAt":      item.UpdatedAt,
		"updated_at":     item.UpdatedAt,
	}
}

func roomSOPDashboardPayload(item RoomSOPDashboardItem) map[string]any {
	setting := sopJSONPayload(item.SettingRaw)
	roomIDs := sopJSONPayload(item.RoomIDsRaw)
	return map[string]any{
		"id":          item.ID,
		"roomSopId":   item.ID,
		"room_sop_id": item.ID,
		"corpId":      item.CorpID,
		"corp_id":     item.CorpID,
		"creatorId":   item.CreatorID,
		"creator_id":  item.CreatorID,
		"creatorName": item.CreatorName,
		"creator":     item.CreatorName,
		"name":        item.Name,
		"setting":     setting,
		"settingRaw":  item.SettingRaw,
		"roomIds":     roomIDs,
		"room_ids":    roomIDs,
		"roomNum":     sopJSONArrayLen(item.RoomIDsRaw),
		"state":       item.State,
		"status":      item.State,
		"createdAt":   item.CreatedAt,
		"created_at":  item.CreatedAt,
		"updatedAt":   item.UpdatedAt,
		"updated_at":  item.UpdatedAt,
	}
}

func sopJSONPayload(raw string) any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []any{}
	}
	var decoded any
	if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
		return decoded
	}
	return raw
}

func sopJSONArrayLen(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	var values []any
	if err := json.Unmarshal([]byte(raw), &values); err == nil {
		return len(values)
	}
	return 0
}
