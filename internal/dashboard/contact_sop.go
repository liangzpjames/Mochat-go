package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

type ContactSOPContent struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type ContactSOPContact struct {
	ID               int
	Name             string
	Avatar           string
	WXExternalUserID string
	UpdatedAt        string
}

type ContactSOPItem struct {
	ID           int
	ContactSOPID int
	Creator      string
	Time         string
	TipTime      string
	TaskRaw      string
	Contact      ContactSOPContact
}

type ContactSOPStore interface {
	SidebarEmployeeByID(ctx context.Context, employeeID int) (SidebarEmployee, bool, error)
	ContactSOPTips(ctx context.Context, employeeID int, contactID int) ([]ContactSOPItem, error)
	ContactSOPInfo(ctx context.Context, employeeID int, corpID int, id int) (ContactSOPItem, bool, error)
}

type ContactSOPHandler struct {
	store      ContactSOPStore
	sidebar    UserIDResolver
	apiBaseURL string
}

func NewContactSOPHandler(store ContactSOPStore, sidebar UserIDResolver, apiBaseURL string) *ContactSOPHandler {
	return &ContactSOPHandler{store: store, sidebar: sidebar, apiBaseURL: strings.TrimRight(apiBaseURL, "/")}
}

func (h *ContactSOPHandler) GetSOPTipInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	employee, ok := h.resolveSidebarAccess(w, r)
	if !ok {
		return
	}
	contactID, err := positiveQueryIntRequired(r, "contactId")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "客户id必传", nil)
		return
	}
	items, err := h.store.ContactSOPTips(r.Context(), employee.ID, contactID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	payload := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payload = append(payload, h.itemPayload(item))
	}
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *ContactSOPHandler) GetSOPInfo(w http.ResponseWriter, r *http.Request) {
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
	item, found, err := h.store.ContactSOPInfo(r.Context(), employee.ID, employee.CorpID, id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "个人SOP不存在", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", h.itemPayload(item))
}

func (h *ContactSOPHandler) itemPayload(item ContactSOPItem) map[string]any {
	contact := map[string]any{
		"id":               item.Contact.ID,
		"name":             item.Contact.Name,
		"avatar":           h.fileFullURL(item.Contact.Avatar),
		"wxExternalUserid": item.Contact.WXExternalUserID,
		"updatedAt":        item.Contact.UpdatedAt,
	}
	return map[string]any{
		"id":           item.ID,
		"contactSopId": item.ContactSOPID,
		"creator":      item.Creator,
		"time":         item.Time,
		"tipTime":      item.TipTime,
		"task":         contactSOPTaskPayload(item.TaskRaw, h.fileFullURL),
		"contact":      contact,
	}
}

func (h *ContactSOPHandler) resolveSidebarAccess(w http.ResponseWriter, r *http.Request) (SidebarEmployee, bool) {
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

func (h *ContactSOPHandler) fileFullURL(path string) string {
	if path == "" || strings.Contains(path, "http") || h.apiBaseURL == "" {
		return path
	}
	return h.apiBaseURL + "/static/" + strings.TrimLeft(path, "/")
}

func contactSOPTaskPayload(raw string, fullURL func(string) string) map[string]any {
	var decoded any
	if strings.TrimSpace(raw) != "" && json.Unmarshal([]byte(raw), &decoded) == nil {
		return normalizeContactSOPTask(decoded, fullURL)
	}
	return map[string]any{"content": []any{}}
}

func normalizeContactSOPTask(decoded any, fullURL func(string) string) map[string]any {
	switch value := decoded.(type) {
	case map[string]any:
		if content, ok := value["content"]; ok {
			value["content"] = normalizeContactSOPContent(content, fullURL)
		} else {
			value["content"] = []any{}
		}
		return value
	case []any:
		return map[string]any{"content": normalizeContactSOPContent(value, fullURL)}
	case string:
		if strings.TrimSpace(value) == "" {
			return map[string]any{"content": []any{}}
		}
		return map[string]any{"content": []any{map[string]any{"type": "text", "value": value}}}
	default:
		return map[string]any{"content": []any{}}
	}
}

func normalizeContactSOPContent(raw any, fullURL func(string) string) []any {
	items, ok := raw.([]any)
	if !ok {
		return []any{}
	}
	content := make([]any, 0, len(items))
	for _, rawItem := range items {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		itemType, _ := item["type"].(string)
		itemValue, _ := item["value"].(string)
		if itemType != "text" && itemValue != "" {
			item["value"] = fullURL(itemValue)
		}
		content = append(content, item)
	}
	return content
}
