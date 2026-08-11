package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

type WorkEmployeeSyncCredential struct {
	CorpID         int
	TenantID       int
	WXCorpID       string
	EmployeeSecret string
	ContactSecret  string
}

type WorkEmployeeSyncDepartment struct {
	WXDepartmentID int
	Name           string
	WXParentID     int
	Order          int
}

type WorkDepartmentEventDepartment struct {
	WXDepartmentID int
	Name           string
	HasName        bool
	WXParentID     int
	HasParent      bool
	Order          int
	HasOrder       bool
}

type WorkEmployeeSyncEmployee struct {
	WXUserID             string
	Name                 string
	Mobile               string
	Position             string
	Gender               int
	Email                string
	Avatar               string
	ThumbAvatar          string
	Telephone            string
	Alias                string
	ExtAttr              json.RawMessage
	Status               int
	QRCode               string
	ExternalProfile      json.RawMessage
	ExternalPosition     json.RawMessage
	Address              string
	OpenUserID           string
	WXMainDepartmentID   int
	DepartmentIDs        []int
	IsLeaderInDepartment []int
	DepartmentOrders     []int
}

type WorkEmployeeSyncResult struct {
	DepartmentsCreated int
	DepartmentsUpdated int
	EmployeesCreated   int
	EmployeesUpdated   int
	UsersCreated       int
	RelationsCreated   int
	RelationsUpdated   int
	RelationsDeleted   int
}

type WorkEmployeeSyncClient interface {
	Departments(ctx context.Context, credential WorkEmployeeSyncCredential) ([]WorkEmployeeSyncDepartment, error)
	DepartmentUsers(ctx context.Context, credential WorkEmployeeSyncCredential, wxDepartmentID int) ([]WorkEmployeeSyncEmployee, error)
	FollowUsers(ctx context.Context, credential WorkEmployeeSyncCredential) ([]string, error)
}

func (h *WorkReadHandler) WithWorkEmployeeSyncClient(client WorkEmployeeSyncClient, passwordKey string) *WorkReadHandler {
	h.workEmployeeSync = client
	h.workEmployeePasswordKey = passwordKey
	return h
}

func (h *WorkReadHandler) WorkEmployeeSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, user, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if h.workEmployeeSync == nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "企业微信成员同步客户端未配置", nil)
		return
	}
	corpIDs, err := h.workEmployeeSyncCorpIDs(r.Context(), userID, user, principalScope)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if len(corpIDs) == 0 {
		writeEnvelope(w, http.StatusOK, 200, "success", []any{})
		return
	}
	credentials, err := h.store.WorkEmployeeSyncCredentials(r.Context(), corpIDs)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	credentialsByCorp := make(map[int]WorkEmployeeSyncCredential, len(credentials))
	for _, credential := range credentials {
		credentialsByCorp[credential.CorpID] = credential
	}
	for _, corpID := range corpIDs {
		credential, ok := credentialsByCorp[corpID]
		if !ok || strings.TrimSpace(credential.WXCorpID) == "" || strings.TrimSpace(credential.EmployeeSecret) == "" {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "企业授权信息错误", nil)
			return
		}
		departments, err := h.workEmployeeSync.Departments(r.Context(), credential)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		employees, err := h.workEmployeeSyncUsers(r.Context(), credential, departments)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		followUserIDs, _ := h.workEmployeeSync.FollowUsers(r.Context(), credential)
		if _, err := h.store.SyncWorkEmployees(r.Context(), credential, departments, employees, followUserIDs, ""); err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *WorkReadHandler) workEmployeeSyncCorpIDs(ctx context.Context, userID int, user User, principalScope DashboardRequestScope) ([]int, error) {
	if user.IsSuperAdmin == 1 && user.TenantID > 0 {
		corpIDs, err := h.store.CorpIDsByTenant(ctx, user.TenantID)
		if err != nil {
			return nil, err
		}
		return uniquePositiveIntsLocal(corpIDs), nil
	}
	corpIDs, err := h.store.CorpIDsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(corpIDs) == 0 {
		corpIDs = principalScope.CorpIDs
	}
	return uniquePositiveIntsLocal(corpIDs), nil
}

func (h *WorkReadHandler) workEmployeeSyncUsers(ctx context.Context, credential WorkEmployeeSyncCredential, departments []WorkEmployeeSyncDepartment) ([]WorkEmployeeSyncEmployee, error) {
	merged := map[string]WorkEmployeeSyncEmployee{}
	for _, department := range departments {
		if department.WXDepartmentID <= 0 {
			continue
		}
		users, err := h.workEmployeeSync.DepartmentUsers(ctx, credential, department.WXDepartmentID)
		if err != nil {
			return nil, err
		}
		for _, user := range users {
			wxUserID := strings.TrimSpace(user.WXUserID)
			if wxUserID == "" {
				continue
			}
			user.WXUserID = wxUserID
			if current, ok := merged[wxUserID]; ok {
				merged[wxUserID] = mergeWorkEmployeeSyncEmployee(current, user)
				continue
			}
			merged[wxUserID] = user
		}
	}
	keys := make([]string, 0, len(merged))
	for wxUserID := range merged {
		keys = append(keys, wxUserID)
	}
	sort.Strings(keys)
	result := make([]WorkEmployeeSyncEmployee, 0, len(keys))
	for _, wxUserID := range keys {
		result = append(result, merged[wxUserID])
	}
	return result, nil
}

func mergeWorkEmployeeSyncEmployee(left WorkEmployeeSyncEmployee, right WorkEmployeeSyncEmployee) WorkEmployeeSyncEmployee {
	if right.Name != "" {
		left.Name = right.Name
	}
	if right.Mobile != "" {
		left.Mobile = right.Mobile
	}
	if right.Position != "" {
		left.Position = right.Position
	}
	if right.Gender != 0 {
		left.Gender = right.Gender
	}
	if right.Email != "" {
		left.Email = right.Email
	}
	if right.Avatar != "" {
		left.Avatar = right.Avatar
	}
	if right.ThumbAvatar != "" {
		left.ThumbAvatar = right.ThumbAvatar
	}
	if right.Telephone != "" {
		left.Telephone = right.Telephone
	}
	if right.Alias != "" {
		left.Alias = right.Alias
	}
	if len(right.ExtAttr) > 0 {
		left.ExtAttr = right.ExtAttr
	}
	if right.Status != 0 {
		left.Status = right.Status
	}
	if right.QRCode != "" {
		left.QRCode = right.QRCode
	}
	if len(right.ExternalProfile) > 0 {
		left.ExternalProfile = right.ExternalProfile
	}
	if len(right.ExternalPosition) > 0 {
		left.ExternalPosition = right.ExternalPosition
	}
	if right.Address != "" {
		left.Address = right.Address
	}
	if right.OpenUserID != "" {
		left.OpenUserID = right.OpenUserID
	}
	if right.WXMainDepartmentID != 0 {
		left.WXMainDepartmentID = right.WXMainDepartmentID
	}
	left.DepartmentIDs, left.IsLeaderInDepartment, left.DepartmentOrders = mergeWorkEmployeeDepartments(
		left.DepartmentIDs, left.IsLeaderInDepartment, left.DepartmentOrders,
		right.DepartmentIDs, right.IsLeaderInDepartment, right.DepartmentOrders,
	)
	return left
}

func mergeWorkEmployeeDepartments(leftIDs []int, leftLeaders []int, leftOrders []int, rightIDs []int, rightLeaders []int, rightOrders []int) ([]int, []int, []int) {
	seen := map[int]int{}
	ids := make([]int, 0, len(leftIDs)+len(rightIDs))
	leaders := make([]int, 0, len(leftIDs)+len(rightIDs))
	orders := make([]int, 0, len(leftIDs)+len(rightIDs))
	appendOne := func(id int, leader int, order int) {
		if id <= 0 {
			return
		}
		if index, ok := seen[id]; ok {
			leaders[index] = leader
			orders[index] = order
			return
		}
		seen[id] = len(ids)
		ids = append(ids, id)
		leaders = append(leaders, leader)
		orders = append(orders, order)
	}
	for index, id := range leftIDs {
		appendOne(id, indexedInt(leftLeaders, index), indexedInt(leftOrders, index))
	}
	for index, id := range rightIDs {
		appendOne(id, indexedInt(rightLeaders, index), indexedInt(rightOrders, index))
	}
	return ids, leaders, orders
}

func indexedInt(values []int, index int) int {
	if index < 0 || index >= len(values) {
		return 0
	}
	return values[index]
}

func (c *RoomWelcomeWeComClient) Departments(ctx context.Context, credential WorkEmployeeSyncCredential) ([]WorkEmployeeSyncDepartment, error) {
	token, err := c.accessToken(ctx, RoomWelcomeCorpCredential{WXCorpID: credential.WXCorpID, ContactSecret: credential.EmployeeSecret})
	if err != nil {
		return nil, err
	}
	var response struct {
		weComBaseResponse
		Departments []struct {
			ID       int    `json:"id"`
			Name     string `json:"name"`
			ParentID int    `json:"parentid"`
			Order    int    `json:"order"`
		} `json:"department"`
	}
	if err := c.getJSON(ctx, "cgi-bin/department/list", token, nil, &response); err != nil {
		return nil, err
	}
	departments := make([]WorkEmployeeSyncDepartment, 0, len(response.Departments))
	for _, department := range response.Departments {
		departments = append(departments, WorkEmployeeSyncDepartment{
			WXDepartmentID: department.ID,
			Name:           department.Name,
			WXParentID:     department.ParentID,
			Order:          department.Order,
		})
	}
	return departments, nil
}

func (c *RoomWelcomeWeComClient) DepartmentUsers(ctx context.Context, credential WorkEmployeeSyncCredential, wxDepartmentID int) ([]WorkEmployeeSyncEmployee, error) {
	token, err := c.accessToken(ctx, RoomWelcomeCorpCredential{WXCorpID: credential.WXCorpID, ContactSecret: credential.EmployeeSecret})
	if err != nil {
		return nil, err
	}
	values := url.Values{}
	values.Set("department_id", strconv.Itoa(wxDepartmentID))
	values.Set("fetch_child", "0")
	var response struct {
		weComBaseResponse
		UserList []workEmployeeSyncWeComUser `json:"userlist"`
	}
	if err := c.getJSON(ctx, "cgi-bin/user/list", token, values, &response); err != nil {
		return nil, err
	}
	users := make([]WorkEmployeeSyncEmployee, 0, len(response.UserList))
	for _, user := range response.UserList {
		users = append(users, user.toSyncEmployee())
	}
	return users, nil
}

func (c *RoomWelcomeWeComClient) User(ctx context.Context, credential WorkEmployeeSyncCredential, wxUserID string) (WorkEmployeeSyncEmployee, error) {
	token, err := c.accessToken(ctx, RoomWelcomeCorpCredential{WXCorpID: credential.WXCorpID, ContactSecret: credential.EmployeeSecret})
	if err != nil {
		return WorkEmployeeSyncEmployee{}, err
	}
	values := url.Values{}
	values.Set("userid", wxUserID)
	var response workEmployeeSyncWeComUserResponse
	if err := c.getJSON(ctx, "cgi-bin/user/get", token, values, &response); err != nil {
		return WorkEmployeeSyncEmployee{}, err
	}
	return response.workEmployeeSyncWeComUser.toSyncEmployee(), nil
}

func (c *RoomWelcomeWeComClient) FollowUsers(ctx context.Context, credential WorkEmployeeSyncCredential) ([]string, error) {
	secret := strings.TrimSpace(credential.ContactSecret)
	if secret == "" {
		secret = strings.TrimSpace(credential.EmployeeSecret)
	}
	if secret == "" {
		return []string{}, nil
	}
	token, err := c.accessToken(ctx, RoomWelcomeCorpCredential{WXCorpID: credential.WXCorpID, ContactSecret: secret})
	if err != nil {
		return nil, err
	}
	var response struct {
		weComBaseResponse
		FollowUser []string `json:"follow_user"`
	}
	if err := c.getJSON(ctx, "cgi-bin/externalcontact/get_follow_user_list", token, nil, &response); err != nil {
		return nil, err
	}
	return response.FollowUser, nil
}

type workEmployeeSyncWeComUser struct {
	UserID               string          `json:"userid"`
	Name                 string          `json:"name"`
	Mobile               string          `json:"mobile"`
	Position             string          `json:"position"`
	Gender               any             `json:"gender"`
	Email                string          `json:"email"`
	Avatar               string          `json:"avatar"`
	ThumbAvatar          string          `json:"thumb_avatar"`
	Telephone            string          `json:"telephone"`
	Alias                string          `json:"alias"`
	ExtAttr              json.RawMessage `json:"extattr"`
	Status               int             `json:"status"`
	QRCode               string          `json:"qr_code"`
	ExternalProfile      json.RawMessage `json:"external_profile"`
	ExternalPosition     json.RawMessage `json:"external_position"`
	Address              string          `json:"address"`
	OpenUserID           string          `json:"open_userid"`
	MainDepartment       int             `json:"main_department"`
	Departments          []int           `json:"department"`
	IsLeaderInDepartment []int           `json:"is_leader_in_dept"`
	Orders               []int           `json:"order"`
}

type workEmployeeSyncWeComUserResponse struct {
	weComBaseResponse
	workEmployeeSyncWeComUser
}

func (u workEmployeeSyncWeComUser) toSyncEmployee() WorkEmployeeSyncEmployee {
	return WorkEmployeeSyncEmployee{
		WXUserID:             strings.TrimSpace(u.UserID),
		Name:                 u.Name,
		Mobile:               u.Mobile,
		Position:             u.Position,
		Gender:               intFromFlexibleJSON(u.Gender),
		Email:                u.Email,
		Avatar:               u.Avatar,
		ThumbAvatar:          u.ThumbAvatar,
		Telephone:            u.Telephone,
		Alias:                u.Alias,
		ExtAttr:              u.ExtAttr,
		Status:               u.Status,
		QRCode:               u.QRCode,
		ExternalProfile:      u.ExternalProfile,
		ExternalPosition:     u.ExternalPosition,
		Address:              u.Address,
		OpenUserID:           u.OpenUserID,
		WXMainDepartmentID:   u.MainDepartment,
		DepartmentIDs:        append([]int{}, u.Departments...),
		IsLeaderInDepartment: append([]int{}, u.IsLeaderInDepartment...),
		DepartmentOrders:     append([]int{}, u.Orders...),
	}
}

func intFromFlexibleJSON(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case string:
		parsed, _ := strconv.Atoi(strings.TrimSpace(typed))
		return parsed
	case nil:
		return 0
	default:
		return 0
	}
}

func (r WorkEmployeeSyncCredential) String() string {
	return fmt.Sprintf("corp=%d tenant=%d wx=%s", r.CorpID, r.TenantID, r.WXCorpID)
}
