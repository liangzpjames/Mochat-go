package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

type User struct {
	ID                              int
	Phone                           string
	Name                            string
	Gender                          int
	Department                      string
	Position                        string
	LoginTime                       string
	Status                          int
	TenantID                        int
	TenantStatus                    int
	TenantPackageExpired            bool
	TenantPackageExpiresAt          string
	TenantSubscriptionManaged       bool
	TenantSubscriptionStatus        string
	TenantSubscriptionAccessAllowed bool
	TenantSubscriptionGraceEndsAt   string
	IsSuperAdmin                    int
}

type Employee struct {
	ID               int
	Name             string
	Mobile           string
	Position         string
	Gender           int
	Email            string
	Avatar           string
	ThumbAvatar      string
	Telephone        string
	Alias            string
	Status           int
	QRCode           string
	ExternalPosition string
	Address          string
}

type Corp struct {
	ID   int
	Name string
}

type Store interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	EmployeeByID(ctx context.Context, employeeID int) (Employee, bool, error)
	CorpByID(ctx context.Context, corpID int) (Corp, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
}

type LoginCache interface {
	UserCorpCache(ctx context.Context, userID int) (string, error)
}

type UserIDResolver interface {
	UserID(r *http.Request) (int, error)
}

type LoginShowHandler struct {
	store    Store
	cache    LoginCache
	resolver UserIDResolver
}

type envelope struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

var ErrUnauthorized = errors.New("unauthorized")

func NewLoginShowHandler(store Store, cache LoginCache, resolver UserIDResolver) *LoginShowHandler {
	return &LoginShowHandler{store: store, cache: cache, resolver: resolver}
}

func (h *LoginShowHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}

	userID, err := h.resolver.UserID(r)
	if err != nil || userID <= 0 {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	user, ok, err := h.store.UserByID(r.Context(), userID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !ok {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "user not found", nil)
		return
	}
	if user.TenantStatus == 2 {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, "租户已停用", nil)
		return
	}
	if user.TenantSubscriptionManaged && !user.TenantSubscriptionAccessAllowed {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, SaaSAdminSubscriptionAccessReason(user.TenantSubscriptionStatus), nil)
		return
	}
	if user.TenantPackageExpired {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, "租户套餐已到期", nil)
		return
	}

	cacheValue := ""
	if h.cache != nil {
		cacheValue, err = h.cache.UserCorpCache(r.Context(), userID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
	}

	loginInfo, err := ResolveValidatedLoginCorpInfoFromStore(r.Context(), r.Header, user, cacheValue, h.store)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	payload := h.payload(r.Context(), user, loginInfo.CorpIDs, loginInfo.WorkEmployeeID)
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *LoginShowHandler) payload(ctx context.Context, user User, corpIDs []int, employeeID int) map[string]any {
	var employee Employee
	if employeeID > 0 {
		if value, ok, err := h.store.EmployeeByID(ctx, employeeID); err == nil && ok {
			employee = value
		}
	}

	var corp Corp
	if len(corpIDs) == 1 && corpIDs[0] > 0 {
		if value, ok, err := h.store.CorpByID(ctx, corpIDs[0]); err == nil && ok {
			corp = value
		}
	}

	return map[string]any{
		"userId":                          user.ID,
		"userPhone":                       user.Phone,
		"userName":                        user.Name,
		"userGender":                      user.Gender,
		"userDepartment":                  user.Department,
		"userPosition":                    user.Position,
		"userLoginTime":                   user.LoginTime,
		"userStatus":                      user.Status,
		"tenantStatus":                    user.TenantStatus,
		"tenantPackageExpired":            user.TenantPackageExpired,
		"tenantPackageExpiresAt":          user.TenantPackageExpiresAt,
		"tenantSubscriptionManaged":       user.TenantSubscriptionManaged,
		"tenantSubscriptionStatus":        user.TenantSubscriptionStatus,
		"tenantSubscriptionAccessAllowed": user.TenantSubscriptionAccessAllowed,
		"tenantSubscriptionGraceEndsAt":   user.TenantSubscriptionGraceEndsAt,
		"employeeId":                      employeeID,
		"employeeName":                    employee.Name,
		"employeeMobile":                  employee.Mobile,
		"employeePosition":                employee.Position,
		"employeeGender":                  employee.Gender,
		"employeeEmail":                   employee.Email,
		"employeeAvatar":                  employee.Avatar,
		"employeeThumbAvatar":             employee.ThumbAvatar,
		"employeeTelephone":               employee.Telephone,
		"employeeAlias":                   employee.Alias,
		"employeeStatus":                  employee.Status,
		"employeeQrCode":                  employee.QRCode,
		"employeeExternalPosition":        employee.ExternalPosition,
		"employeeAddress":                 employee.Address,
		"corpId":                          corp.ID,
		"corpName":                        corp.Name,
	}
}

type HeaderUserIDResolver struct {
	HeaderName string
}

func (r HeaderUserIDResolver) UserID(req *http.Request) (int, error) {
	header := r.HeaderName
	if header == "" {
		header = "X-Mochat-Go-User-ID"
	}
	raw := req.Header.Get(header)
	userID, err := strconv.Atoi(raw)
	if err != nil || userID <= 0 {
		return 0, ErrUnauthorized
	}
	return userID, nil
}

func writeEnvelope(w http.ResponseWriter, httpStatus int, code int, msg string, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(httpStatus)
	_ = json.NewEncoder(w).Encode(envelope{Code: code, Msg: msg, Data: data})
}
