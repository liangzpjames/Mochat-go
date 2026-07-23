package dashboard

import (
	"context"
	"net/http"
	"strconv"
)

type ContactBatchAddContact struct {
	ID     int
	Phone  string
	Status int
}

type ContactBatchAddDetail struct {
	EmployeeName string
	List         []ContactBatchAddContact
}

type ContactBatchAddStore interface {
	SidebarEmployeeByID(ctx context.Context, employeeID int) (SidebarEmployee, bool, error)
	ContactBatchAddDetail(ctx context.Context, employeeID int, batchID int, status int) (ContactBatchAddDetail, error)
}

type ContactBatchAddHandler struct {
	store   ContactBatchAddStore
	sidebar UserIDResolver
}

func NewContactBatchAddHandler(store ContactBatchAddStore, sidebar UserIDResolver) *ContactBatchAddHandler {
	return &ContactBatchAddHandler{store: store, sidebar: sidebar}
}

func (h *ContactBatchAddHandler) Detail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	employee, ok := h.resolveSidebarAccess(w, r)
	if !ok {
		return
	}
	batchID, err := positiveQueryIntRequired(r, "batchId")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "批次id必传", nil)
		return
	}
	status, err := contactBatchAddStatusParam(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "状态参数错误", nil)
		return
	}
	if status < 0 || status > 4 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "状态参数错误", nil)
		return
	}
	detail, err := h.store.ContactBatchAddDetail(r.Context(), employee.ID, batchID, status)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(detail.List))
	for _, item := range detail.List {
		list = append(list, map[string]any{
			"id":     item.ID,
			"phone":  item.Phone,
			"status": contactBatchAddStatusText(item.Status),
		})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"employeeName": detail.EmployeeName,
		"list":         list,
	})
}

func (h *ContactBatchAddHandler) resolveSidebarAccess(w http.ResponseWriter, r *http.Request) (SidebarEmployee, bool) {
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

func contactBatchAddStatusText(status int) string {
	switch status {
	case 0:
		return "待分配"
	case 1:
		return "待添加"
	case 2:
		return "待通过"
	case 3:
		return "已添加"
	default:
		return ""
	}
}

func contactBatchAddStatusParam(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("status")
	if raw == "" {
		return 4, nil
	}
	return strconv.Atoi(raw)
}
