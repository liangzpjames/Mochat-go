package dashboard

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"time"

	"jiyi/mochat-go/internal/authjwt"
)

type UserAdminFilter struct {
	TenantID       int
	UserIDs        []int
	RestrictUserID bool
	Phone          string
	Status         *int
	Page           int
	PerPage        int
}

type UserAdminItem struct {
	ID           int
	Phone        string
	Name         string
	Gender       int
	Department   string
	Position     string
	LoginTime    string
	Status       int
	TenantID     int
	IsSuperAdmin int
	CreatedAt    string
	UpdatedAt    string
}

type UserAdminPage struct {
	Items     []UserAdminItem
	Total     int
	TotalPage int
	PerPage   int
}

type UserAdminStatusCounts struct {
	NotEnabled int
	Normal     int
	Disable    int
}

type UserAdminRoleInfo struct {
	RoleID   int
	RoleName string
}

type UserAdminDepartment struct {
	DepartmentID   int
	DepartmentName string
}

type UserAdminWrite struct {
	Name       string
	Phone      string
	Gender     int
	Department string
	Status     int
	TenantID   int
	RoleID     int
	Password   string
}

type UserAdminStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	UserAdminIDsByTenant(ctx context.Context, tenantID int) ([]int, error)
	UserAdminLogUserIDsByCorp(ctx context.Context, corpID int) ([]int, error)
	UserAdminLogUserIDsByEmployees(ctx context.Context, employeeIDs []int) ([]int, error)
	UserAdminStatusCounts(ctx context.Context, userIDs []int) (UserAdminStatusCounts, error)
	UserAdminPage(ctx context.Context, filter UserAdminFilter) (UserAdminPage, error)
	UserAdminByID(ctx context.Context, userID int) (UserAdminItem, bool, error)
	UserAdminRolesByUserIDs(ctx context.Context, userIDs []int) (map[int]UserAdminRoleInfo, error)
	UserAdminDepartmentsByUserIDs(ctx context.Context, corpID int, userIDs []int) (map[int][]UserAdminDepartment, error)
	UserAdminIDsByPhone(ctx context.Context, phone string) ([]int, error)
	CreateUserAdmin(ctx context.Context, values UserAdminWrite, corpID int) (int, error)
	UpdateUserAdmin(ctx context.Context, userID int, values UserAdminWrite, corpID int) (bool, error)
	UserAdminItemsByIDs(ctx context.Context, userIDs []int) ([]UserAdminItem, error)
	UpdateUserAdminStatuses(ctx context.Context, userIDs []int, status int) error
	UpdateUserAdminPassword(ctx context.Context, userID int, passwordHash string) (bool, error)
}

type UserAdminHandler struct {
	store        UserAdminStore
	cache        LoginCache
	resolver     UserIDResolver
	authorizer   CorpAdminAuthorizer
	passwordSalt string
	logoutStore  LogoutStore
	parser       authjwt.Parser
	blacklistTTL time.Duration
}

func NewUserAdminHandler(
	store UserAdminStore,
	cache LoginCache,
	resolver UserIDResolver,
	authorizer CorpAdminAuthorizer,
	passwordSalt string,
	logoutStore LogoutStore,
	parser authjwt.Parser,
	blacklistTTL time.Duration,
) *UserAdminHandler {
	return &UserAdminHandler{
		store:        store,
		cache:        cache,
		resolver:     resolver,
		authorizer:   authorizer,
		passwordSalt: passwordSalt,
		logoutStore:  logoutStore,
		parser:       parser,
		blacklistTTL: blacklistTTL,
	}
}

func (h *UserAdminHandler) Index(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, user, _, access, ok := h.resolveAuthorized(w, r, "/dashboard/user/index#get")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	allowedUserIDs, err := h.accessibleUserIDs(r.Context(), user, corpID, access)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	statusCounts, err := h.store.UserAdminStatusCounts(r.Context(), allowedUserIDs)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	filter := UserAdminFilter{
		TenantID:       user.TenantID,
		UserIDs:        allowedUserIDs,
		RestrictUserID: true,
		Phone:          strings.TrimSpace(r.URL.Query().Get("phone")),
		Page:           positiveQueryInt(r, "page", 1),
		PerPage:        positiveQueryInt(r, "perPage", 10),
	}
	if rawStatus := strings.TrimSpace(r.URL.Query().Get("status")); rawStatus != "" && rawStatus != "no" {
		status, err := parsePositiveOrZeroInt(rawStatus)
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "状态 必需为整数", nil)
			return
		}
		filter.Status = &status
	}
	page, err := h.store.UserAdminPage(r.Context(), filter)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	userIDs := make([]int, 0, len(page.Items))
	for _, item := range page.Items {
		userIDs = append(userIDs, item.ID)
	}
	roles, departments, err := h.listLookups(r.Context(), corpID, userIDs)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, userAdminListPayload(item, roles[item.ID], departments[item.ID]))
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"page": map[string]any{
			"perPage":   page.PerPage,
			"total":     page.Total,
			"totalPage": page.TotalPage,
		},
		"notEnabledNum": statusCounts.NotEnabled,
		"normalNum":     statusCounts.Normal,
		"disableNum":    statusCounts.Disable,
		"list":          list,
	})
}

func (h *UserAdminHandler) Show(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/user/show#get")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	userID, err := positiveQueryIntRequired(r, "userId")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "用户ID 必填", nil)
		return
	}
	item, found, err := h.store.UserAdminByID(r.Context(), userID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "当前账户不存在", nil)
		return
	}
	roles, departments, err := h.listLookups(r.Context(), corpID, []int{userID})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", userAdminDetailPayload(item, roles[item.ID], departments[item.ID]))
}

func (h *UserAdminHandler) Store(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, currentUser, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/user/store#post")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	values, ok := h.userWriteFromParams(w, params, currentUser.TenantID, true)
	if !ok {
		return
	}
	existingIDs, err := h.store.UserAdminIDsByPhone(r.Context(), values.Phone)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if len(existingIDs) > 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "手机号已存在，不可重复创建", nil)
		return
	}
	if !enforceSaaSQuota(r.Context(), w, h.store, currentUser.TenantID, SaaSMetricUsers, 1) {
		return
	}
	hash, err := authjwt.GeneratePasswordHash(h.passwordSalt, values.Password)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "账户创建失败", nil)
		return
	}
	values.Password = hash
	if _, err := h.store.CreateUserAdmin(r.Context(), values, corpID); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "账户创建失败", nil)
		return
	}
	if err := refreshSaaSUsageCounter(r.Context(), h.store, currentUser.TenantID, SaaSMetricUsers); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "账户用量刷新失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *UserAdminHandler) Update(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, currentUser, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/user/update#put")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	userID, okInt, err := intParam(params, "userId")
	if err != nil || !okInt || userID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "用户ID 必填", nil)
		return
	}
	if _, found, err := h.store.UserAdminByID(r.Context(), userID); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	} else if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "当前账户不存在，不可操作", nil)
		return
	}
	values, ok := h.userWriteFromParams(w, params, currentUser.TenantID, false)
	if !ok {
		return
	}
	existingIDs, err := h.store.UserAdminIDsByPhone(r.Context(), values.Phone)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	for _, existingID := range existingIDs {
		if existingID != userID {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "手机号已存在，不可重复创建", nil)
			return
		}
	}
	updated, err := h.store.UpdateUserAdmin(r.Context(), userID, values, corpID)
	if err != nil || !updated {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "账户更新失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *UserAdminHandler) StatusUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, _, _, ok := h.resolveAccess(w, r); !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	userIDs, err := intSliceParam(params, "userId")
	if err != nil || len(userIDs) == 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "用户ID 必填", nil)
		return
	}
	status, okInt, err := intParam(params, "status")
	if err != nil || !okInt || (status != 1 && status != 2) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "状态 必填", nil)
		return
	}
	users, err := h.store.UserAdminItemsByIDs(r.Context(), userIDs)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if len(users) != len(userIDs) {
		found := map[int]struct{}{}
		for _, user := range users {
			found[user.ID] = struct{}{}
		}
		missing := []string{}
		for _, id := range userIDs {
			if _, ok := found[id]; !ok {
				missing = append(missing, intToString(id))
			}
		}
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "部分账户信息不存在，账户ID："+strings.Join(missing, "、"), nil)
		return
	}
	repeatedNames := []string{}
	for _, user := range users {
		if user.Status == status {
			repeatedNames = append(repeatedNames, user.Name)
		}
	}
	if len(repeatedNames) > 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, strings.Join(repeatedNames, "、")+"当前状态："+userStatusText(status)+"，不可重复操作", nil)
		return
	}
	if err := h.store.UpdateUserAdminStatuses(r.Context(), userIDs, status); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "账户状态更新失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *UserAdminHandler) PasswordReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, _, _, ok := h.resolveAccess(w, r); !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	userID, okInt, err := intParam(params, "id")
	if err != nil || !okInt || userID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "用户id 必填", nil)
		return
	}
	newPassword := stringParam(params, "newPassword")
	if !validAlphaNumPassword(newPassword) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "新密码 组成必须是字母或数字", nil)
		return
	}
	if _, found, err := h.store.UserAdminByID(r.Context(), userID); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	} else if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "当前账户不存在，不可操作", nil)
		return
	}
	hash, err := authjwt.GeneratePasswordHash(h.passwordSalt, newPassword)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "账户更新密码失败", nil)
		return
	}
	updated, err := h.store.UpdateUserAdminPassword(r.Context(), userID, hash)
	if err != nil || !updated {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "账户更新密码失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *UserAdminHandler) PasswordUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, _, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	oldPassword := stringParam(params, "oldPassword")
	newPassword := stringParam(params, "newPassword")
	againPassword := stringParam(params, "againNewPassword")
	if !validAlphaNumPassword(oldPassword) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "旧密码 组成必须是字母或数字", nil)
		return
	}
	if !validAlphaNumPassword(newPassword) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "新密码 组成必须是字母或数字", nil)
		return
	}
	if againPassword != newPassword {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "确认新密码 必须和新密码一致", nil)
		return
	}
	authUser, found, err := h.store.UserAdminByID(r.Context(), userID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "当前账户不存在，不可操作", nil)
		return
	}
	passwordHash, err := h.userPasswordHash(r.Context(), userID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !authjwt.CheckPasswordHash(h.passwordSalt, oldPassword, passwordHash) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "密码错误，不可操作", nil)
		return
	}
	newHash, err := authjwt.GeneratePasswordHash(h.passwordSalt, newPassword)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "账户更新密码失败", nil)
		return
	}
	updated, err := h.store.UpdateUserAdminPassword(r.Context(), authUser.ID, newHash)
	if err != nil || !updated {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "账户更新密码失败", nil)
		return
	}
	h.logoutCurrentToken(r)
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *UserAdminHandler) userPasswordHash(ctx context.Context, userID int) (string, error) {
	if store, ok := h.store.(interface {
		UserAuthByID(ctx context.Context, userID int) (AuthUser, bool, error)
	}); ok {
		authUser, found, err := store.UserAuthByID(ctx, userID)
		if err != nil || !found {
			return "", err
		}
		return authUser.Password, nil
	}
	return "", nil
}

func (h *UserAdminHandler) userWriteFromParams(w http.ResponseWriter, params map[string]any, tenantID int, requirePassword bool) (UserAdminWrite, bool) {
	name := stringParam(params, "userName")
	if name == "" {
		name = stringParam(params, "name")
	}
	if name == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "用户名称 必填", nil)
		return UserAdminWrite{}, false
	}
	phone := stringParam(params, "phone")
	if len(phone) != 11 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "手机号码 字符串长度为固定值：11", nil)
		return UserAdminWrite{}, false
	}
	gender, okInt, err := intParam(params, "gender")
	if err != nil || !okInt || (gender != 1 && gender != 2) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "性别 必填", nil)
		return UserAdminWrite{}, false
	}
	status, okInt, err := intParam(params, "status")
	if err != nil || !okInt || status < 0 || status > 2 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "状态 必填", nil)
		return UserAdminWrite{}, false
	}
	roleID, _, err := intParam(params, "roleId")
	if err != nil || roleID < 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "角色ID 必需为整数", nil)
		return UserAdminWrite{}, false
	}
	password := stringParam(params, "password")
	if requirePassword && password == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "密码 必填", nil)
		return UserAdminWrite{}, false
	}
	return UserAdminWrite{
		Name:       name,
		Phone:      phone,
		Gender:     gender,
		Department: stringParam(params, "department"),
		Status:     status,
		TenantID:   tenantID,
		RoleID:     roleID,
		Password:   password,
	}, true
}

func (h *UserAdminHandler) accessibleUserIDs(ctx context.Context, user User, corpID int, access AccessContext) ([]int, error) {
	if user.IsSuperAdmin == 1 {
		return h.store.UserAdminIDsByTenant(ctx, user.TenantID)
	}
	if access.DataPermission == DataPermissionAll {
		return h.store.UserAdminLogUserIDsByCorp(ctx, corpID)
	}
	return h.store.UserAdminLogUserIDsByEmployees(ctx, access.DeptEmployeeIDs)
}

func (h *UserAdminHandler) listLookups(ctx context.Context, corpID int, userIDs []int) (map[int]UserAdminRoleInfo, map[int][]UserAdminDepartment, error) {
	roles, err := h.store.UserAdminRolesByUserIDs(ctx, userIDs)
	if err != nil {
		return nil, nil, err
	}
	departments, err := h.store.UserAdminDepartmentsByUserIDs(ctx, corpID, userIDs)
	if err != nil {
		return nil, nil, err
	}
	return roles, departments, nil
}

func (h *UserAdminHandler) resolveAuthorized(w http.ResponseWriter, r *http.Request, permissionKey string) (int, User, DashboardRequestScope, AccessContext, bool) {
	userID, user, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return 0, User{}, DashboardRequestScope{}, AccessContext{}, false
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return 0, User{}, DashboardRequestScope{}, AccessContext{}, false
	}
	access := AccessContext{User: user, CorpID: corpID, WorkEmployeeID: principalScope.WorkEmployeeID, DataPermission: DataPermissionAll}
	if h.authorizer != nil {
		var err error
		access, err = h.authorizer.Resolve(r.Context(), userID, permissionKey, corpID, principalScope.WorkEmployeeID)
		if err != nil {
			writeAccessError(w, err)
			return 0, User{}, DashboardRequestScope{}, AccessContext{}, false
		}
	}
	return userID, user, principalScope, access, true
}

func (h *UserAdminHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (int, User, DashboardRequestScope, bool) {
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
	if !requireTenantSuperAdmin(w, user) {
		return 0, User{}, DashboardRequestScope{}, false
	}
	principalScope, err := DashboardRequestScopeFromContext(r.Context())
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return 0, User{}, DashboardRequestScope{}, false
	}
	return userID, user, DashboardRequestScope(principalScope), true
}

func (h *UserAdminHandler) logoutCurrentToken(r *http.Request) {
	if h.logoutStore == nil || h.parser.Secret == "" {
		return
	}
	token := authjwt.TokenFromRequest(r)
	if token == "" {
		return
	}
	payload, err := h.parser.Parse(r.Context(), token)
	if err != nil {
		return
	}
	if userID, ok := authjwt.UserIDFromPayload(payload); ok && userID > 0 {
		_ = h.logoutStore.DeleteUserCorpCache(r.Context(), userID)
	}
	ttl := h.blacklistTTL
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	_ = h.logoutStore.AddJWTBlacklist(r.Context(), authjwt.BlacklistKey(h.parser.Prefix, payload, token), ttl)
}

func userAdminListPayload(item UserAdminItem, role UserAdminRoleInfo, departments []UserAdminDepartment) map[string]any {
	return map[string]any{
		"userId":       item.ID,
		"userName":     item.Name,
		"phone":        item.Phone,
		"gender":       item.Gender,
		"department":   userAdminDepartmentPayloads(departments),
		"loginTime":    item.LoginTime,
		"status":       item.Status,
		"statusText":   userStatusText(item.Status),
		"tenantId":     item.TenantID,
		"isSuperAdmin": item.IsSuperAdmin,
		"roleName":     role.RoleName,
		"createdAt":    item.CreatedAt,
	}
}

func userAdminDetailPayload(item UserAdminItem, role UserAdminRoleInfo, departments []UserAdminDepartment) map[string]any {
	return map[string]any{
		"userId":       item.ID,
		"userName":     item.Name,
		"phone":        item.Phone,
		"gender":       item.Gender,
		"department":   userAdminDepartmentPayloads(departments),
		"status":       item.Status,
		"statusText":   userStatusText(item.Status),
		"tenantId":     item.TenantID,
		"isSuperAdmin": item.IsSuperAdmin,
		"roleId":       role.RoleID,
		"roleName":     role.RoleName,
	}
}

func userAdminDepartmentPayloads(departments []UserAdminDepartment) []map[string]any {
	out := make([]map[string]any, 0, len(departments))
	for _, department := range departments {
		out = append(out, map[string]any{
			"departmentId":   department.DepartmentID,
			"departmentName": department.DepartmentName,
		})
	}
	return out
}

func userStatusText(status int) string {
	switch status {
	case 0:
		return "未启用"
	case 1:
		return "正常"
	case 2:
		return "禁用"
	default:
		return ""
	}
}

var alphaNumPasswordPattern = regexp.MustCompile(`^[A-Za-z0-9]+$`)

func validAlphaNumPassword(password string) bool {
	return password != "" && alphaNumPasswordPattern.MatchString(password)
}

func intToString(value int) string {
	return strings.TrimSpace(toString(value))
}
