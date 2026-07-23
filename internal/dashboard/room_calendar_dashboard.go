package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

type RoomCalendarFilter struct {
	CorpID  int
	Name    string
	OnOff   int
	Page    int
	PerPage int
}

type RoomCalendarItem struct {
	ID             int
	Name           string
	RoomsRaw       string
	OnOff          int
	TenantID       int
	CorpID         int
	CreateUserID   int
	CreateUserName string
	RoomNum        int
	PushNum        int
	CreatedAt      string
	UpdatedAt      string
	Pushes         []RoomCalendarPushItem
}

type RoomCalendarPushItem struct {
	ID             int
	RoomCalendarID int
	Name           string
	Day            string
	PushContentRaw string
	OnOff          int
	Status         int
	CreatedAt      string
	UpdatedAt      string
}

type RoomCalendarPage struct {
	Items     []RoomCalendarItem
	Total     int
	TotalPage int
	Page      int
	PerPage   int
}

type RoomCalendarWrite struct {
	CorpID       int
	CreateUserID int
	Name         string
	HasName      bool
	RoomsRaw     string
	HasRooms     bool
	OnOff        int
	HasOnOff     bool
	Pushes       []RoomCalendarPushWrite
	HasPushes    bool
}

type RoomCalendarPushWrite struct {
	ID             int
	Name           string
	Day            string
	PushContentRaw string
	OnOff          int
	HasOnOff       bool
	Status         int
	HasStatus      bool
}

type RoomCalendarStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	RoomCalendarPage(ctx context.Context, filter RoomCalendarFilter) (RoomCalendarPage, error)
	RoomCalendarByID(ctx context.Context, corpID int, id int) (RoomCalendarItem, bool, error)
	CreateRoomCalendar(ctx context.Context, values RoomCalendarWrite) (int, error)
	UpdateRoomCalendar(ctx context.Context, corpID int, id int, values RoomCalendarWrite) (bool, error)
	DeleteRoomCalendar(ctx context.Context, corpID int, id int) (bool, error)
	AddRoomCalendarRooms(ctx context.Context, corpID int, id int, roomsRaw string) (bool, error)
	RemoveRoomCalendarRoom(ctx context.Context, corpID int, id int, roomKey string) (bool, error)
}

type RoomCalendarHandler struct {
	store      RoomCalendarStore
	cache      LoginCache
	resolver   UserIDResolver
	authorizer CorpAdminAuthorizer
}

func NewRoomCalendarHandler(store RoomCalendarStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer) *RoomCalendarHandler {
	return &RoomCalendarHandler{store: store, cache: cache, resolver: resolver, authorizer: authorizer}
}

func (h *RoomCalendarHandler) Index(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/roomCalendar/index#get")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	page, err := h.store.RoomCalendarPage(r.Context(), RoomCalendarFilter{
		CorpID:  corpID,
		Name:    lotteryQueryString(r, "name", "keyword"),
		OnOff:   lotteryQueryIntAllowZero(r, "onOff", -1),
		Page:    positiveQueryInt(r, "page", 1),
		PerPage: positiveQueryInt(r, "perPage", 15),
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, roomCalendarPayload(item))
	}
	payload := contactBatchAddPagination(r, page.Page, page.PerPage, page.Total, list)
	payload["list"] = list
	payload["page"] = map[string]any{"perPage": page.PerPage, "total": page.Total, "totalPage": page.TotalPage}
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *RoomCalendarHandler) Store(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, user, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/roomCalendar/store#post")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, 400, err.Error(), nil)
		return
	}
	values, err := roomCalendarWriteFromParams(params, corpID, userID, true)
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
	if !enforceSaaSQuota(r.Context(), w, h.store, user.TenantID, SaaSMetricRoomCalendars, 1) {
		return
	}
	id, err := h.store.CreateRoomCalendar(r.Context(), values)
	if err != nil {
		if writeSaaSQuotaError(w, err) {
			return
		}
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if err := refreshSaaSUsageCounter(r.Context(), h.store, user.TenantID, SaaSMetricRoomCalendars); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{id})
}

func (h *RoomCalendarHandler) Update(w http.ResponseWriter, r *http.Request) {
	h.writeMutation(w, r, http.MethodPut, "/dashboard/roomCalendar/update#put", func(ctx context.Context, userID int, corpID int, params map[string]any) (any, error) {
		id, err := roomCalendarIDFromParams(params)
		if err != nil || id <= 0 {
			return nil, badRequestError("roomCalendarId required")
		}
		values, err := roomCalendarWriteFromParams(params, corpID, userID, false)
		if err != nil {
			return nil, err
		}
		updated, err := h.store.UpdateRoomCalendar(ctx, corpID, id, values)
		if err != nil {
			return nil, err
		}
		if !updated {
			return nil, badRequestError("room calendar not found")
		}
		return []any{id}, nil
	})
}

func (h *RoomCalendarHandler) Destroy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, user, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/roomCalendar/destroy#delete")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, 400, err.Error(), nil)
		return
	}
	id, err := roomCalendarIDFromParams(params)
	if err != nil || id <= 0 {
		writeEnvelope(w, http.StatusBadRequest, 400, "roomCalendarId required", nil)
		return
	}
	deleted, err := h.store.DeleteRoomCalendar(r.Context(), corpID, id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !deleted {
		writeEnvelope(w, http.StatusBadRequest, 400, "room calendar not found", nil)
		return
	}
	if err := refreshSaaSUsageCounter(r.Context(), h.store, user.TenantID, SaaSMetricRoomCalendars); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{id})
}

func (h *RoomCalendarHandler) Show(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/roomCalendar/show#get")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	id := roomCalendarIDFromQuery(r)
	if id <= 0 {
		writeEnvelope(w, http.StatusBadRequest, 400, "roomCalendarId required", nil)
		return
	}
	item, found, err := h.store.RoomCalendarByID(r.Context(), corpID, id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusNotFound, 404, "room calendar not found", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", roomCalendarPayload(item))
}

func (h *RoomCalendarHandler) AddRoom(w http.ResponseWriter, r *http.Request) {
	h.writeMutation(w, r, http.MethodPost, "/dashboard/roomCalendar/addRoom#post", func(ctx context.Context, _ int, corpID int, params map[string]any) (any, error) {
		id, err := roomCalendarIDFromParams(params)
		if err != nil || id <= 0 {
			return nil, badRequestError("roomCalendarId required")
		}
		roomsRaw, err := roomCalendarRoomsFromParams(params)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(roomsRaw) == "" {
			return nil, badRequestError("room required")
		}
		updated, err := h.store.AddRoomCalendarRooms(ctx, corpID, id, roomsRaw)
		if err != nil {
			return nil, err
		}
		if !updated {
			return nil, badRequestError("room calendar not found")
		}
		return []any{id}, nil
	})
}

func (h *RoomCalendarHandler) DestroyRoom(w http.ResponseWriter, r *http.Request) {
	h.writeMutation(w, r, http.MethodDelete, "/dashboard/roomCalendar/destroyRoom#delete", func(ctx context.Context, _ int, corpID int, params map[string]any) (any, error) {
		id, err := roomCalendarIDFromParams(params)
		if err != nil || id <= 0 {
			return nil, badRequestError("roomCalendarId required")
		}
		roomKey, ok := firstStringParam(params, "chatid", "wxChatId", "roomId", "room_id", "idRoom")
		if !ok {
			if nested, ok := roomCalendarMapParam(params, "room"); ok {
				roomKey, _ = firstStringParam(nested, "chatid", "wxChatId", "roomId", "room_id", "id")
			}
		}
		roomKey = strings.TrimSpace(roomKey)
		if roomKey == "" {
			return nil, badRequestError("room required")
		}
		updated, err := h.store.RemoveRoomCalendarRoom(ctx, corpID, id, roomKey)
		if err != nil {
			return nil, err
		}
		if !updated {
			return nil, badRequestError("room calendar not found")
		}
		return []any{id}, nil
	})
}

func (h *RoomCalendarHandler) writeMutation(w http.ResponseWriter, r *http.Request, method string, permission string, mutate func(context.Context, int, int, map[string]any) (any, error)) {
	if r.Method != method {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, loginInfo, _, ok := h.resolveAuthorized(w, r, permission)
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
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

func (h *RoomCalendarHandler) resolveAuthorized(w http.ResponseWriter, r *http.Request, permissionKey string) (int, User, LoginCorpInfo, AccessContext, bool) {
	userID, user, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return 0, User{}, LoginCorpInfo{}, AccessContext{}, false
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return 0, User{}, LoginCorpInfo{}, AccessContext{}, false
	}
	employeeID := loginInfo.WorkEmployeeID
	if employeeID <= 0 {
		resolved, err := h.store.EmployeeIDByUserCorp(r.Context(), userID, corpID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return 0, User{}, LoginCorpInfo{}, AccessContext{}, false
		}
		employeeID = resolved
	}
	access := AccessContext{User: user, PermissionKey: permissionKey, CorpID: corpID, WorkEmployeeID: employeeID, DataPermission: DataPermissionAll}
	if h.authorizer != nil {
		resolved, err := h.authorizer.Resolve(r.Context(), userID, permissionKey, corpID, employeeID)
		if err != nil {
			writeAccessError(w, err)
			return 0, User{}, LoginCorpInfo{}, AccessContext{}, false
		}
		access = resolved
	}
	return userID, user, loginInfo, access, true
}

func (h *RoomCalendarHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (int, User, LoginCorpInfo, bool) {
	if h.resolver == nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "user resolver not configured", nil)
		return 0, User{}, LoginCorpInfo{}, false
	}
	userID, err := h.resolver.UserID(r)
	if err != nil || userID <= 0 {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return 0, User{}, LoginCorpInfo{}, false
	}
	user, found, err := h.store.UserByID(r.Context(), userID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return 0, User{}, LoginCorpInfo{}, false
	}
	if !found {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "user not found", nil)
		return 0, User{}, LoginCorpInfo{}, false
	}
	cacheValue := ""
	if h.cache != nil {
		cacheValue, err = h.cache.UserCorpCache(r.Context(), userID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return 0, User{}, LoginCorpInfo{}, false
		}
	}
	loginInfo, err := ResolveValidatedLoginCorpInfoFromStore(r.Context(), r.Header, user, cacheValue, h.store)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return 0, User{}, LoginCorpInfo{}, false
	}
	return userID, user, loginInfo, true
}

func roomCalendarWriteFromParams(params map[string]any, corpID int, userID int, requireName bool) (RoomCalendarWrite, error) {
	if nested, ok := roomCalendarMapParam(params, "calendar"); ok {
		for key, value := range params {
			if _, exists := nested[key]; !exists && key != "calendar" {
				nested[key] = value
			}
		}
		params = nested
	}
	values := RoomCalendarWrite{CorpID: corpID, CreateUserID: userID}
	if name, ok := firstStringParam(params, "name", "calendarName", "calendar_name"); ok {
		values.Name = name
		values.HasName = true
	}
	if requireName && strings.TrimSpace(values.Name) == "" {
		return values, badRequestError("name required")
	}
	if raw, ok := firstJSONRawParam(params, "rooms", "roomIds", "room_ids"); ok {
		values.RoomsRaw = raw
		values.HasRooms = true
	}
	if onOff, ok, err := firstIntParam(params, "onOff", "on_off", "status"); err != nil {
		return values, err
	} else if ok {
		values.OnOff = onOff
		values.HasOnOff = true
	}
	pushes, hasPushes, err := roomCalendarPushesFromParams(params)
	if err != nil {
		return values, err
	}
	values.Pushes = pushes
	values.HasPushes = hasPushes
	if requireName {
		if !values.HasRooms {
			values.RoomsRaw = "[]"
			values.HasRooms = true
		}
		if !values.HasOnOff {
			values.OnOff = 1
			values.HasOnOff = true
		}
	}
	return values, nil
}

func roomCalendarPushesFromParams(params map[string]any) ([]RoomCalendarPushWrite, bool, error) {
	if form, ok := roomCalendarMapParam(params, "form"); ok {
		raw, hasList := firstJSONRawParam(params, "list", "pushContent", "push_content")
		if !hasList {
			raw = "[]"
		}
		push := RoomCalendarPushWrite{PushContentRaw: raw, HasOnOff: true, OnOff: 1, HasStatus: true, Status: 1}
		if id, has, err := firstPositiveIntParam(form, "id", "pushId", "push_id"); err != nil {
			return nil, false, err
		} else if has {
			push.ID = id
		}
		if name, ok := firstStringParam(form, "name", "title"); ok {
			push.Name = name
		}
		date, _ := firstStringParam(form, "date", "day")
		timeText, _ := firstStringParam(form, "time")
		push.Day = strings.TrimSpace(strings.TrimSpace(date) + " " + strings.TrimSpace(timeText))
		return []RoomCalendarPushWrite{push}, true, nil
	}
	raw, ok := firstJSONRawParam(params, "pushes", "pushList", "push", "push_list")
	if !ok {
		return nil, false, nil
	}
	var decoded any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return nil, true, err
	}
	items, ok := decoded.([]any)
	if !ok {
		items = []any{decoded}
	}
	pushes := make([]RoomCalendarPushWrite, 0, len(items))
	for _, item := range items {
		mapped, ok := item.(map[string]any)
		if !ok {
			continue
		}
		push, err := roomCalendarPushFromMap(mapped)
		if err != nil {
			return nil, true, err
		}
		pushes = append(pushes, push)
	}
	return pushes, true, nil
}

func roomCalendarPushFromMap(params map[string]any) (RoomCalendarPushWrite, error) {
	push := RoomCalendarPushWrite{HasOnOff: true, OnOff: 1, HasStatus: true, Status: 1}
	if id, has, err := firstPositiveIntParam(params, "id", "pushId", "push_id"); err != nil {
		return push, err
	} else if has {
		push.ID = id
	}
	if name, ok := firstStringParam(params, "name", "title"); ok {
		push.Name = name
	}
	date, _ := firstStringParam(params, "date")
	timeText, _ := firstStringParam(params, "time")
	if day, ok := firstStringParam(params, "day", "sendTime", "send_time"); ok {
		push.Day = day
	} else {
		push.Day = strings.TrimSpace(strings.TrimSpace(date) + " " + strings.TrimSpace(timeText))
	}
	if raw, ok := firstJSONRawParam(params, "pushContent", "push_content", "content", "list"); ok {
		push.PushContentRaw = raw
	} else {
		push.PushContentRaw = "[]"
	}
	if onOff, ok, err := firstIntParam(params, "onOff", "on_off"); err != nil {
		return push, err
	} else if ok {
		push.OnOff = onOff
		push.HasOnOff = true
	}
	if status, ok, err := firstIntParam(params, "status"); err != nil {
		return push, err
	} else if ok {
		push.Status = status
		push.HasStatus = true
	}
	return push, nil
}

func roomCalendarRoomsFromParams(params map[string]any) (string, error) {
	if raw, ok := firstJSONRawParam(params, "rooms", "roomIds", "room_ids"); ok {
		return raw, nil
	}
	if nested, ok := roomCalendarMapParam(params, "room"); ok {
		raw, err := json.Marshal(nested)
		return string(raw), err
	}
	if value, ok := firstStringParam(params, "roomId", "room_id", "chatid", "wxChatId"); ok {
		raw, _ := json.Marshal([]string{value})
		return string(raw), nil
	}
	return "", nil
}

func roomCalendarIDFromParams(params map[string]any) (int, error) {
	if id, has, err := firstPositiveIntParam(params, "roomCalendarId", "room_calendar_id", "calendarId", "calendar_id", "id"); err != nil || has {
		return id, err
	}
	if nested, ok := roomCalendarMapParam(params, "calendar"); ok {
		id, _, err := firstPositiveIntParam(nested, "roomCalendarId", "room_calendar_id", "calendarId", "calendar_id", "id")
		return id, err
	}
	return 0, nil
}

func roomCalendarIDFromQuery(r *http.Request) int {
	for _, key := range []string{"roomCalendarId", "room_calendar_id", "calendarId", "calendar_id", "id"} {
		if value := positiveQueryInt(r, key, 0); value > 0 {
			return value
		}
	}
	return 0
}

func roomCalendarMapParam(params map[string]any, key string) (map[string]any, bool) {
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

func roomCalendarPayload(item RoomCalendarItem) map[string]any {
	pushes := make([]map[string]any, 0, len(item.Pushes))
	for _, push := range item.Pushes {
		pushes = append(pushes, roomCalendarPushPayload(push))
	}
	return map[string]any{
		"id":               item.ID,
		"roomCalendarId":   item.ID,
		"room_calendar_id": item.ID,
		"calendarId":       item.ID,
		"name":             item.Name,
		"rooms":            jsonPayload(item.RoomsRaw),
		"onOff":            item.OnOff,
		"on_off":           item.OnOff,
		"tenantId":         item.TenantID,
		"corpId":           item.CorpID,
		"createUserId":     item.CreateUserID,
		"createUserName":   item.CreateUserName,
		"roomNum":          item.RoomNum,
		"pushNum":          item.PushNum,
		"push":             pushes,
		"pushList":         pushes,
		"createdAt":        item.CreatedAt,
		"updatedAt":        item.UpdatedAt,
	}
}

func roomCalendarPushPayload(item RoomCalendarPushItem) map[string]any {
	return map[string]any{
		"id":             item.ID,
		"pushId":         item.ID,
		"roomCalendarId": item.RoomCalendarID,
		"name":           item.Name,
		"day":            item.Day,
		"date":           roomCalendarDatePart(item.Day),
		"time":           roomCalendarTimePart(item.Day),
		"pushContent":    jsonPayload(item.PushContentRaw),
		"push_content":   jsonPayload(item.PushContentRaw),
		"onOff":          item.OnOff,
		"on_off":         item.OnOff,
		"status":         item.Status,
		"createdAt":      item.CreatedAt,
		"updatedAt":      item.UpdatedAt,
	}
}

func roomCalendarDatePart(day string) string {
	fields := strings.Fields(day)
	if len(fields) > 0 {
		return fields[0]
	}
	return day
}

func roomCalendarTimePart(day string) string {
	fields := strings.Fields(day)
	if len(fields) > 1 {
		return fields[1]
	}
	return ""
}
