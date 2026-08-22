package dashboard

import (
	"context"
	"net/http"
	"strconv"
	"strings"
)

type SidebarEmployeeProfile struct {
	ID              int      `json:"id"`
	Name            string   `json:"name"`
	Avatar          string   `json:"avatar"`
	DepartmentNames []string `json:"departmentNames"`
	CorpName        string   `json:"corpName"`
}

type SidebarCustomerMetrics struct {
	Total          int `json:"total"`
	AddedToday     int `json:"addedToday"`
	TaggedTotal    int `json:"taggedTotal"`
	OwnedRoomTotal int `json:"ownedRoomTotal"`
}

type SidebarTaskMetrics struct {
	ContactSOPPending int `json:"contactSopPending"`
	RoomSOPPending    int `json:"roomSopPending"`
	BatchAddPending   int `json:"batchAddPending"`
}

type SidebarWorkbenchSummary struct {
	Employee  SidebarEmployeeProfile `json:"employee"`
	Customers SidebarCustomerMetrics `json:"customers"`
	Tasks     SidebarTaskMetrics     `json:"tasks"`
}

type SidebarContactListItem struct {
	ID               int      `json:"id"`
	WXExternalUserID string   `json:"wxExternalUserid"`
	Name             string   `json:"name"`
	Avatar           string   `json:"avatar"`
	Remark           string   `json:"remark"`
	Status           int      `json:"status"`
	AddedAt          string   `json:"addedAt"`
	Tags             []string `json:"tags"`
}

type SidebarContactFilter struct {
	Keyword string
	Page    int
	PerPage int
}

type SidebarContactPage struct {
	Page      int                      `json:"page"`
	PerPage   int                      `json:"perPage"`
	Total     int                      `json:"total"`
	TotalPage int                      `json:"totalPage"`
	Items     []SidebarContactListItem `json:"items"`
}

type SidebarTaskFilter struct {
	Kind    string
	State   string
	Page    int
	PerPage int
}

type SidebarTaskListItem struct {
	ID          int    `json:"id"`
	Kind        string `json:"kind"`
	Title       string `json:"title"`
	SubjectName string `json:"subjectName"`
	ScheduledAt string `json:"scheduledAt"`
	State       string `json:"state"`
}

type SidebarTaskPage struct {
	Page      int                   `json:"page"`
	PerPage   int                   `json:"perPage"`
	Total     int                   `json:"total"`
	TotalPage int                   `json:"totalPage"`
	Items     []SidebarTaskListItem `json:"items"`
}

type SidebarWorkbenchStore interface {
	SidebarEmployeeByID(context.Context, int) (SidebarEmployee, bool, error)
	SidebarWorkbenchSummary(context.Context, SidebarEmployee) (SidebarWorkbenchSummary, error)
	SidebarContacts(context.Context, SidebarEmployee, SidebarContactFilter) (SidebarContactPage, error)
	SidebarTasks(context.Context, SidebarEmployee, SidebarTaskFilter) (SidebarTaskPage, error)
}

type SidebarWorkbenchHandler struct {
	store   SidebarWorkbenchStore
	sidebar UserIDResolver
}

func NewSidebarWorkbenchHandler(store SidebarWorkbenchStore, sidebar UserIDResolver) *SidebarWorkbenchHandler {
	return &SidebarWorkbenchHandler{store: store, sidebar: sidebar}
}

func (h *SidebarWorkbenchHandler) Summary(w http.ResponseWriter, r *http.Request) {
	if !sidebarWorkbenchGET(w, r) {
		return
	}
	employee, ok := h.resolveEmployee(w, r)
	if !ok {
		return
	}
	summary, err := h.store.SidebarWorkbenchSummary(r.Context(), employee)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", summary)
}

func (h *SidebarWorkbenchHandler) Contacts(w http.ResponseWriter, r *http.Request) {
	if !sidebarWorkbenchGET(w, r) {
		return
	}
	employee, ok := h.resolveEmployee(w, r)
	if !ok {
		return
	}
	page, perPage, valid := sidebarWorkbenchPagination(r)
	if !valid {
		writeEnvelope(w, http.StatusUnprocessableEntity, http.StatusUnprocessableEntity, "分页参数错误", nil)
		return
	}
	filter := SidebarContactFilter{Keyword: strings.TrimSpace(r.URL.Query().Get("keyword")), Page: page, PerPage: perPage}
	result, err := h.store.SidebarContacts(r.Context(), employee, filter)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", result)
}

func (h *SidebarWorkbenchHandler) Tasks(w http.ResponseWriter, r *http.Request) {
	if !sidebarWorkbenchGET(w, r) {
		return
	}
	employee, ok := h.resolveEmployee(w, r)
	if !ok {
		return
	}
	kind := strings.TrimSpace(r.URL.Query().Get("kind"))
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	if !validSidebarTaskKind(kind) || !validSidebarTaskState(state) {
		writeEnvelope(w, http.StatusUnprocessableEntity, http.StatusUnprocessableEntity, "任务筛选参数错误", nil)
		return
	}
	page, perPage, valid := sidebarWorkbenchPagination(r)
	if !valid {
		writeEnvelope(w, http.StatusUnprocessableEntity, http.StatusUnprocessableEntity, "分页参数错误", nil)
		return
	}
	filter := SidebarTaskFilter{Kind: kind, State: state, Page: page, PerPage: perPage}
	result, err := h.store.SidebarTasks(r.Context(), employee, filter)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", result)
}

func (h *SidebarWorkbenchHandler) resolveEmployee(w http.ResponseWriter, r *http.Request) (SidebarEmployee, bool) {
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
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "employee not found", nil)
		return SidebarEmployee{}, false
	}
	return employee, true
}

func sidebarWorkbenchGET(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet {
		return true
	}
	writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
	return false
}

func sidebarWorkbenchPagination(r *http.Request) (int, int, bool) {
	page, ok := strictPositiveIntDefault(r.URL.Query().Get("page"), 1)
	if !ok {
		return 0, 0, false
	}
	perPage, ok := strictPositiveIntDefault(r.URL.Query().Get("perPage"), 20)
	if !ok {
		return 0, 0, false
	}
	if perPage > 50 {
		perPage = 50
	}
	return page, perPage, true
}

func strictPositiveIntDefault(raw string, fallback int) (int, bool) {
	if strings.TrimSpace(raw) == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	return value, err == nil && value > 0
}

func validSidebarTaskKind(kind string) bool {
	return kind == "contactSop" || kind == "roomSop" || kind == "batchAdd"
}

func validSidebarTaskState(state string) bool {
	return state == "pending" || state == "done"
}
