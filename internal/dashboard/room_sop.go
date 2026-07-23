package dashboard

import (
	"context"
	"net/http"
	"strings"
)

type RoomSOPRoom struct {
	ID         int
	Name       string
	WXChatID   string
	CreateTime string
}

type RoomSOPItem struct {
	ID        int
	RoomSOPID int
	Creator   string
	Time      string
	State     int
	TaskRaw   string
	Room      RoomSOPRoom
}

type RoomSOPStore interface {
	SidebarEmployeeByID(ctx context.Context, employeeID int) (SidebarEmployee, bool, error)
	RoomSOPInfo(ctx context.Context, employeeID int, id int) (RoomSOPItem, bool, error)
	MarkRoomSOPDone(ctx context.Context, employeeID int, id int) (bool, error)
}

type RoomSOPHandler struct {
	store      RoomSOPStore
	sidebar    UserIDResolver
	apiBaseURL string
}

func NewRoomSOPHandler(store RoomSOPStore, sidebar UserIDResolver, apiBaseURL string) *RoomSOPHandler {
	return &RoomSOPHandler{store: store, sidebar: sidebar, apiBaseURL: strings.TrimRight(apiBaseURL, "/")}
}

func (h *RoomSOPHandler) GetSOPInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	employee, ok := h.resolveSidebarAccess(w, r)
	if !ok {
		return
	}
	id, err := positiveQueryIntRequired(r, "id")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "ID 必填", nil)
		return
	}
	item, found, err := h.store.RoomSOPInfo(r.Context(), employee.ID, id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "群SOP不存在", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", h.itemPayload(item))
}

func (h *RoomSOPHandler) LogState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	employee, ok := h.resolveSidebarAccess(w, r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "参数错误", nil)
		return
	}
	id, ok, err := intParam(params, "id")
	if err != nil || !ok || id <= 0 {
		if queryID, queryErr := positiveQueryIntRequired(r, "id"); queryErr == nil {
			id = queryID
		} else {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "ID 必填", nil)
			return
		}
	}
	found, err := h.store.MarkRoomSOPDone(r.Context(), employee.ID, id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "群SOP不存在", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *RoomSOPHandler) itemPayload(item RoomSOPItem) map[string]any {
	return map[string]any{
		"id":        item.ID,
		"roomSopId": item.RoomSOPID,
		"creator":   item.Creator,
		"time":      item.Time,
		"state":     item.State,
		"task":      contactSOPTaskPayload(item.TaskRaw, h.fileFullURL),
		"room": map[string]any{
			"id":         item.Room.ID,
			"name":       item.Room.Name,
			"wxChatId":   item.Room.WXChatID,
			"createTime": item.Room.CreateTime,
		},
	}
}

func (h *RoomSOPHandler) resolveSidebarAccess(w http.ResponseWriter, r *http.Request) (SidebarEmployee, bool) {
	if h.sidebar == nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "sidebar employee resolver not configured", nil)
		return SidebarEmployee{}, false
	}
	employeeID, err := h.sidebar.UserID(r)
	if err != nil || employeeID <= 0 {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return SidebarEmployee{}, false
	}
	employee, found, err := h.store.SidebarEmployeeByID(r.Context(), employeeID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return SidebarEmployee{}, false
	}
	if !found {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "employee not found", nil)
		return SidebarEmployee{}, false
	}
	return employee, true
}

func (h *RoomSOPHandler) fileFullURL(path string) string {
	if path == "" || strings.Contains(path, "http") || h.apiBaseURL == "" {
		return path
	}
	return h.apiBaseURL + "/static/" + strings.TrimLeft(path, "/")
}
