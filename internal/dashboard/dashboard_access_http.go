package dashboard

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type DashboardAccessHTTP struct {
	service  *DashboardAccessAdminService
	resolver UserIDResolver
}

func NewDashboardAccessHTTP(service *DashboardAccessAdminService, resolver UserIDResolver) *DashboardAccessHTTP {
	return &DashboardAccessHTTP{service: service, resolver: resolver}
}

func (handler *DashboardAccessHTTP) ServeHTTP(w http.ResponseWriter, request *http.Request) {
	if handler == nil || handler.service == nil || handler.resolver == nil {
		writeMachineEnvelope(w, http.StatusInternalServerError, "DASHBOARD_ACCESS_ERROR", "dashboard access unavailable", nil)
		return
	}
	actorUserID, err := handler.resolver.UserID(request)
	if err != nil || actorUserID <= 0 {
		writeMachineEnvelope(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized", nil)
		return
	}

	path := request.URL.Path
	switch path {
	case "/dashboard/access/profile":
		if request.Method != http.MethodGet {
			writeDashboardAccessMethodNotAllowed(w)
			return
		}
		corpID := 0
		if access, ok := DashboardAccessFromContext(request.Context()); ok {
			corpID = access.CorpID
		}
		result, err := handler.service.Profile(request.Context(), actorUserID, corpID)
		handler.writeResult(w, http.StatusOK, result, err)
		return
	case "/dashboard/access/catalog":
		if request.Method != http.MethodGet {
			writeDashboardAccessMethodNotAllowed(w)
			return
		}
		result, err := handler.service.Catalog(request.Context(), actorUserID)
		handler.writeResult(w, http.StatusOK, result, err)
		return
	case "/dashboard/access/users":
		handler.serveUsers(w, request, actorUserID)
		return
	case "/dashboard/access/roles":
		handler.serveRoles(w, request, actorUserID)
		return
	case "/dashboard/access/audits":
		handler.serveAudits(w, request, actorUserID)
		return
	}

	if id, suffix, ok := dashboardAccessPathID(path, "/dashboard/access/users/"); ok {
		if suffix != "" {
			writeDashboardAccessNotFound(w)
			return
		}
		handler.serveUser(w, request, actorUserID, id)
		return
	}
	if id, suffix, ok := dashboardAccessPathID(path, "/dashboard/access/roles/"); ok {
		handler.serveRole(w, request, actorUserID, id, suffix)
		return
	}
	writeDashboardAccessNotFound(w)
}

func (handler *DashboardAccessHTTP) serveUsers(w http.ResponseWriter, request *http.Request, actorUserID int) {
	if request.Method != http.MethodGet {
		writeDashboardAccessMethodNotAllowed(w)
		return
	}
	page, perPage, err := dashboardAccessPagination(request)
	if err != nil {
		handler.writeResult(w, http.StatusOK, nil, ErrDashboardAccessAdminInvalid)
		return
	}
	result, err := handler.service.Users(request.Context(), actorUserID, page, perPage)
	handler.writeResult(w, http.StatusOK, result, err)
}

func (handler *DashboardAccessHTTP) serveUser(w http.ResponseWriter, request *http.Request, actorUserID, targetUserID int) {
	switch request.Method {
	case http.MethodGet:
		result, err := handler.service.User(request.Context(), actorUserID, targetUserID)
		handler.writeResult(w, http.StatusOK, result, err)
	case http.MethodPut:
		var input ReplaceUserDashboardAccessInput
		if err := decodeDashboardAccessJSON(request, &input); err != nil {
			handler.writeResult(w, http.StatusOK, nil, ErrDashboardAccessAdminInvalid)
			return
		}
		input.RequestID = dashboardAccessRequestID(request, input.RequestID)
		result, err := handler.service.ReplaceUserAccess(request.Context(), actorUserID, targetUserID, input)
		handler.writeResult(w, http.StatusOK, result, err)
	default:
		writeDashboardAccessMethodNotAllowed(w)
	}
}

func (handler *DashboardAccessHTTP) serveRoles(w http.ResponseWriter, request *http.Request, actorUserID int) {
	switch request.Method {
	case http.MethodGet:
		page, perPage, err := dashboardAccessPagination(request)
		if err != nil {
			handler.writeResult(w, http.StatusOK, nil, ErrDashboardAccessAdminInvalid)
			return
		}
		result, err := handler.service.Roles(request.Context(), actorUserID, page, perPage)
		handler.writeResult(w, http.StatusOK, result, err)
	case http.MethodPost:
		var input CreateDashboardRoleInput
		if err := decodeDashboardAccessJSON(request, &input); err != nil {
			handler.writeResult(w, http.StatusCreated, nil, ErrDashboardAccessAdminInvalid)
			return
		}
		input.RequestID = dashboardAccessRequestID(request, input.RequestID)
		result, err := handler.service.CreateRole(request.Context(), actorUserID, input)
		handler.writeResult(w, http.StatusCreated, result, err)
	default:
		writeDashboardAccessMethodNotAllowed(w)
	}
}

func (handler *DashboardAccessHTTP) serveRole(w http.ResponseWriter, request *http.Request, actorUserID, roleID int, suffix string) {
	if suffix == "/status" {
		if request.Method != http.MethodPut {
			writeDashboardAccessMethodNotAllowed(w)
			return
		}
		var input UpdateDashboardRoleStatusInput
		if err := decodeDashboardAccessJSON(request, &input); err != nil {
			handler.writeResult(w, http.StatusOK, nil, ErrDashboardAccessAdminInvalid)
			return
		}
		input.RequestID = dashboardAccessRequestID(request, input.RequestID)
		result, err := handler.service.UpdateRoleStatus(request.Context(), actorUserID, roleID, input)
		handler.writeResult(w, http.StatusOK, result, err)
		return
	}
	if suffix != "" {
		writeDashboardAccessNotFound(w)
		return
	}
	switch request.Method {
	case http.MethodPut:
		var input UpdateDashboardRoleInput
		if err := decodeDashboardAccessJSON(request, &input); err != nil {
			handler.writeResult(w, http.StatusOK, nil, ErrDashboardAccessAdminInvalid)
			return
		}
		input.RequestID = dashboardAccessRequestID(request, input.RequestID)
		result, err := handler.service.UpdateRole(request.Context(), actorUserID, roleID, input)
		handler.writeResult(w, http.StatusOK, result, err)
	case http.MethodDelete:
		var input DeleteDashboardRoleInput
		if err := decodeDashboardAccessJSON(request, &input); err != nil {
			handler.writeResult(w, http.StatusOK, nil, ErrDashboardAccessAdminInvalid)
			return
		}
		input.RequestID = dashboardAccessRequestID(request, input.RequestID)
		err := handler.service.DeleteRole(request.Context(), actorUserID, roleID, input)
		handler.writeResult(w, http.StatusOK, map[string]uint64{"version": input.ExpectedVersion + 1}, err)
	default:
		writeDashboardAccessMethodNotAllowed(w)
	}
}

func (handler *DashboardAccessHTTP) serveAudits(w http.ResponseWriter, request *http.Request, actorUserID int) {
	if request.Method != http.MethodGet {
		writeDashboardAccessMethodNotAllowed(w)
		return
	}
	page, perPage, err := dashboardAccessPagination(request)
	if err != nil {
		handler.writeResult(w, http.StatusOK, nil, ErrDashboardAccessAdminInvalid)
		return
	}
	filter := DashboardPermissionAuditFilter{
		TargetType: strings.TrimSpace(request.URL.Query().Get("targetType")),
		TargetID:   strings.TrimSpace(request.URL.Query().Get("targetId")),
		Action:     strings.TrimSpace(request.URL.Query().Get("action")),
		Page:       page,
		PerPage:    perPage,
	}
	if raw := strings.TrimSpace(request.URL.Query().Get("actorUserId")); raw != "" {
		filter.ActorUserID, err = strconv.Atoi(raw)
		if err != nil || filter.ActorUserID <= 0 {
			handler.writeResult(w, http.StatusOK, nil, ErrDashboardAccessAdminInvalid)
			return
		}
	}
	if filter.StartedAt, err = dashboardAccessQueryTime(request, "startedAt"); err != nil {
		handler.writeResult(w, http.StatusOK, nil, ErrDashboardAccessAdminInvalid)
		return
	}
	if filter.EndedAt, err = dashboardAccessQueryTime(request, "endedAt"); err != nil {
		handler.writeResult(w, http.StatusOK, nil, ErrDashboardAccessAdminInvalid)
		return
	}
	result, err := handler.service.Audits(request.Context(), actorUserID, filter)
	handler.writeResult(w, http.StatusOK, result, err)
}

func (handler *DashboardAccessHTTP) writeResult(w http.ResponseWriter, successStatus int, data any, err error) {
	if err == nil {
		writeEnvelope(w, successStatus, http.StatusOK, "success", data)
		return
	}
	switch {
	case errors.Is(err, ErrUnauthorized):
		writeMachineEnvelope(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized", nil)
	case errors.Is(err, ErrDashboardAccessAdminForbidden):
		writeMachineEnvelope(w, http.StatusForbidden, DashboardPermissionDeniedCode, "dashboard permission denied", nil)
	case errors.Is(err, ErrDashboardAccessAdminNotFound):
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "not found", nil)
	case errors.Is(err, ErrDashboardAccessAdminConflict):
		writeEnvelope(w, http.StatusConflict, http.StatusConflict, "version conflict", nil)
	case errors.Is(err, ErrDashboardAccessRoleHasMembers):
		writeEnvelope(w, http.StatusConflict, http.StatusConflict, "role has members", nil)
	case errors.Is(err, ErrDashboardAccessAdminInvalid):
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request", nil)
	default:
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "dashboard access unavailable", nil)
	}
}

func decodeDashboardAccessJSON(request *http.Request, target any) error {
	if request == nil || request.Body == nil {
		return io.ErrUnexpectedEOF
	}
	decoder := json.NewDecoder(io.LimitReader(request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain exactly one JSON object")
	}
	return nil
}

func dashboardAccessPathID(path, prefix string) (int, string, bool) {
	if !strings.HasPrefix(path, prefix) {
		return 0, "", false
	}
	rest := strings.TrimPrefix(path, prefix)
	idText, suffix, _ := strings.Cut(rest, "/")
	id, err := strconv.Atoi(idText)
	if err != nil || id <= 0 {
		return 0, "", false
	}
	if suffix != "" {
		suffix = "/" + suffix
	}
	return id, suffix, true
}

func dashboardAccessPagination(request *http.Request) (int, int, error) {
	page, err := dashboardAccessPositiveQueryInt(request, "page")
	if err != nil {
		return 0, 0, err
	}
	perPage, err := dashboardAccessPositiveQueryInt(request, "perPage")
	return page, perPage, err
}

func dashboardAccessPositiveQueryInt(request *http.Request, key string) (int, error) {
	raw := strings.TrimSpace(request.URL.Query().Get(key))
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, ErrDashboardAccessAdminInvalid
	}
	return value, nil
}

func dashboardAccessQueryTime(request *http.Request, key string) (time.Time, error) {
	raw := strings.TrimSpace(request.URL.Query().Get(key))
	if raw == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, raw)
}

func dashboardAccessRequestID(request *http.Request, bodyValue string) string {
	if value := strings.TrimSpace(request.Header.Get("X-Request-ID")); value != "" {
		return value
	}
	return strings.TrimSpace(bodyValue)
}

func writeDashboardAccessMethodNotAllowed(w http.ResponseWriter) {
	writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
}

func writeDashboardAccessNotFound(w http.ResponseWriter) {
	writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "not found", nil)
}

func requireTenantSuperAdmin(w http.ResponseWriter, user User) bool {
	if user.ID > 0 && user.TenantID > 0 && user.IsSuperAdmin == 1 {
		return true
	}
	writeDashboardPermissionDenied(w)
	return false
}
