package dashboard

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"jiyi/mochat-go/internal/authpassword"
)

var (
	ErrDashboardAccessAdminForbidden = errors.New("dashboard access administration forbidden")
	ErrDashboardAccessAdminNotFound  = errors.New("dashboard access administration target not found")
	ErrDashboardAccessAdminConflict  = errors.New("dashboard access administration version conflict")
	ErrDashboardAccessRoleHasMembers = errors.New("dashboard access role has members")
	ErrDashboardAccessAdminInvalid   = errors.New("invalid dashboard access administration input")
)

const (
	dashboardAccessSystemRoleRemark       = "系统预置全权限角色"
	dashboardAccessLegacySystemRoleRemark = "bootstrap full-access role"
)

func IsReservedDashboardRoleRemark(remark string) bool {
	remark = strings.TrimSpace(remark)
	return remark == dashboardAccessSystemRoleRemark || remark == dashboardAccessLegacySystemRoleRemark
}

type DashboardAccessPage struct {
	Page      int `json:"page"`
	PerPage   int `json:"perPage"`
	Total     int `json:"total"`
	TotalPage int `json:"totalPage"`
}

type DashboardAccessRoleSummary struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Status  int    `json:"status"`
	Version uint64 `json:"version"`
}

type DashboardPermissionAssignment struct {
	Code  string    `json:"code"`
	Scope DataScope `json:"scope"`
}

type DashboardAccessUserSummary struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Phone        string `json:"phone"`
	Status       int    `json:"status"`
	IsSuperAdmin bool   `json:"isSuperAdmin"`
	Version      uint64 `json:"version"`
}

type DashboardAccessUserDetail struct {
	ID                   int                             `json:"id"`
	TenantID             int                             `json:"-"`
	Name                 string                          `json:"name"`
	Phone                string                          `json:"phone"`
	Status               int                             `json:"status"`
	IsSuperAdmin         bool                            `json:"isSuperAdmin"`
	Roles                []DashboardAccessRoleSummary    `json:"roles"`
	DirectPermissions    []DashboardPermissionAssignment `json:"directPermissions"`
	InheritedPermissions []EffectivePermission           `json:"inheritedPermissions"`
	EffectivePermissions []EffectivePermission           `json:"effectivePermissions"`
	Version              uint64                          `json:"version"`
}

type DashboardAccessUserPage struct {
	List []DashboardAccessUserSummary `json:"list"`
	Page DashboardAccessPage          `json:"page"`
}

type DashboardEmployeeAccount struct {
	UserID             int    `json:"userId"`
	LoginIdentifier    string `json:"loginIdentifier"`
	Status             int    `json:"status"`
	MustRotatePassword bool   `json:"mustRotatePassword"`
	AuthVersion        uint64 `json:"authVersion"`
}

type DashboardAccessEmployee struct {
	ID       int                       `json:"id"`
	WXUserID string                    `json:"wxUserId"`
	Name     string                    `json:"name"`
	Mobile   string                    `json:"mobile"`
	Status   int                       `json:"status"`
	Account  *DashboardEmployeeAccount `json:"account"`
}

type DashboardAccessEmployeePage struct {
	List []DashboardAccessEmployee `json:"list"`
	Page DashboardAccessPage       `json:"page"`
}

type ProvisionDashboardEmployeeAccountInput struct {
	LoginIdentifier string `json:"loginIdentifier"`
	RoleIDs         []int  `json:"roleIds"`
	RequestID       string `json:"requestId"`
}

type UpdateDashboardEmployeeAccountStatusInput struct {
	Status    int    `json:"status"`
	RequestID string `json:"requestId"`
}

type ResetDashboardEmployeePasswordInput struct {
	RequestID string `json:"requestId"`
}

type DashboardEmployeeAccountMutationResult struct {
	Employee          DashboardAccessEmployee `json:"employee"`
	TemporaryPassword string                  `json:"temporaryPassword,omitempty"`
}

type DashboardAccessRoleDetail struct {
	ID          int                             `json:"id"`
	TenantID    int                             `json:"-"`
	Name        string                          `json:"name"`
	Remark      string                          `json:"remark"`
	Status      int                             `json:"status"`
	IsSystem    bool                            `json:"isSystem"`
	MemberCount int                             `json:"memberCount"`
	Permissions []DashboardPermissionAssignment `json:"permissions"`
	Version     uint64                          `json:"version"`
}

type DashboardAccessRolePage struct {
	List []DashboardAccessRoleDetail `json:"list"`
	Page DashboardAccessPage         `json:"page"`
}

type DashboardPermissionAudit struct {
	ID              int64           `json:"id"`
	ActorUserID     *int            `json:"actorUserId"`
	ActorName       string          `json:"actorName"`
	Action          string          `json:"action"`
	TargetType      string          `json:"targetType"`
	TargetID        string          `json:"targetId"`
	BeforeJSON      json.RawMessage `json:"before"`
	AfterJSON       json.RawMessage `json:"after"`
	ExpectedVersion *uint64         `json:"expectedVersion"`
	ResultVersion   *uint64         `json:"resultVersion"`
	RequestID       string          `json:"requestId"`
	CreatedAt       time.Time       `json:"time"`
}

type DashboardPermissionAuditFilter struct {
	ActorUserID int
	TargetType  string
	TargetID    string
	Action      string
	StartedAt   time.Time
	EndedAt     time.Time
	Page        int
	PerPage     int
}

type DashboardPermissionAuditPage struct {
	List []DashboardPermissionAudit `json:"list"`
	Page DashboardAccessPage        `json:"page"`
}

type ReplaceUserDashboardAccessInput struct {
	RoleIDs           []int                           `json:"roleIds"`
	DirectPermissions []DashboardPermissionAssignment `json:"directPermissions"`
	ExpectedVersion   uint64                          `json:"expectedVersion"`
	RequestID         string                          `json:"requestId"`
}

type CreateDashboardRoleInput struct {
	Name        string                          `json:"name"`
	Remark      string                          `json:"remark"`
	Status      int                             `json:"status"`
	Permissions []DashboardPermissionAssignment `json:"permissions"`
	RequestID   string                          `json:"requestId"`
}

type UpdateDashboardRoleInput struct {
	Name            string                          `json:"name"`
	Remark          string                          `json:"remark"`
	Permissions     []DashboardPermissionAssignment `json:"permissions"`
	ExpectedVersion uint64                          `json:"expectedVersion"`
	RequestID       string                          `json:"requestId"`
}

type UpdateDashboardRoleStatusInput struct {
	Status          int    `json:"status"`
	ExpectedVersion uint64 `json:"expectedVersion"`
	RequestID       string `json:"requestId"`
}

type DeleteDashboardRoleInput struct {
	ExpectedVersion uint64 `json:"expectedVersion"`
	RequestID       string `json:"requestId"`
}

type ReplaceUserDashboardAccessCommand struct {
	TenantID          int
	ActorUserID       int
	ActorName         string
	TargetUserID      int
	RoleIDs           []int
	DirectPermissions []DashboardPermissionAssignment
	ExpectedVersion   uint64
	RequestID         string
}

type CreateDashboardRoleCommand struct {
	TenantID    int
	ActorUserID int
	ActorName   string
	Name        string
	Remark      string
	Status      int
	Permissions []DashboardPermissionAssignment
	RequestID   string
}

type UpdateDashboardRoleCommand struct {
	TenantID        int
	ActorUserID     int
	ActorName       string
	RoleID          int
	Name            string
	Remark          string
	Permissions     []DashboardPermissionAssignment
	ExpectedVersion uint64
	RequestID       string
}

type UpdateDashboardRoleStatusCommand struct {
	TenantID        int
	ActorUserID     int
	ActorName       string
	RoleID          int
	Status          int
	ExpectedVersion uint64
	RequestID       string
}

type DeleteDashboardRoleCommand struct {
	TenantID        int
	ActorUserID     int
	ActorName       string
	RoleID          int
	ExpectedVersion uint64
	RequestID       string
}

type ProvisionDashboardEmployeeAccountCommand struct {
	TenantID        int
	ActorUserID     int
	ActorName       string
	EmployeeID      int
	LoginIdentifier string
	RoleIDs         []int
	PasswordHash    string
	RequestID       string
}

type UpdateDashboardEmployeeAccountStatusCommand struct {
	TenantID    int
	ActorUserID int
	ActorName   string
	EmployeeID  int
	Status      int
	RequestID   string
}

type ResetDashboardEmployeePasswordCommand struct {
	TenantID     int
	ActorUserID  int
	ActorName    string
	EmployeeID   int
	PasswordHash string
	RequestID    string
}

type DashboardAccessAdminStore interface {
	DashboardAccessStore
	DashboardAccessUsers(ctx context.Context, tenantID, page, perPage int) (DashboardAccessUserPage, error)
	DashboardAccessUser(ctx context.Context, tenantID, userID int) (DashboardAccessUserDetail, bool, error)
	DashboardAccessRoles(ctx context.Context, tenantID, page, perPage int) (DashboardAccessRolePage, error)
	DashboardAccessRole(ctx context.Context, tenantID, roleID int) (DashboardAccessRoleDetail, bool, error)
	DashboardPermissionAudits(ctx context.Context, tenantID int, filter DashboardPermissionAuditFilter) (DashboardPermissionAuditPage, error)
	ReplaceUserDashboardAccess(ctx context.Context, command ReplaceUserDashboardAccessCommand) (DashboardAccessUserDetail, error)
	CreateDashboardRole(ctx context.Context, command CreateDashboardRoleCommand) (DashboardAccessRoleDetail, error)
	UpdateDashboardRole(ctx context.Context, command UpdateDashboardRoleCommand) (DashboardAccessRoleDetail, error)
	UpdateDashboardRoleStatus(ctx context.Context, command UpdateDashboardRoleStatusCommand) (DashboardAccessRoleDetail, error)
	DeleteDashboardRole(ctx context.Context, command DeleteDashboardRoleCommand) error
	DashboardAccessEmployees(ctx context.Context, tenantID, page, perPage int) (DashboardAccessEmployeePage, error)
	ProvisionDashboardEmployeeAccount(ctx context.Context, command ProvisionDashboardEmployeeAccountCommand) (DashboardAccessEmployee, error)
	UpdateDashboardEmployeeAccountStatus(ctx context.Context, command UpdateDashboardEmployeeAccountStatusCommand) (DashboardAccessEmployee, error)
	ResetDashboardEmployeePassword(ctx context.Context, command ResetDashboardEmployeePasswordCommand) (DashboardAccessEmployee, error)
}

type DashboardAccessAdminService struct {
	store             DashboardAccessAdminStore
	access            *DashboardAccessService
	temporaryPassword func() (string, error)
}

func NewDashboardAccessAdminService(store DashboardAccessAdminStore, accessServices ...*DashboardAccessService) *DashboardAccessAdminService {
	access := NewDashboardAccessService(store)
	if len(accessServices) > 0 && accessServices[0] != nil {
		access = accessServices[0]
	}
	return &DashboardAccessAdminService{store: store, access: access, temporaryPassword: generateDashboardTemporaryPassword}
}

func (service *DashboardAccessAdminService) Employees(ctx context.Context, actorUserID, page, perPage int) (DashboardAccessEmployeePage, error) {
	actor, err := service.adminIdentity(ctx, actorUserID)
	if err != nil {
		return DashboardAccessEmployeePage{}, err
	}
	page, perPage = normalizeDashboardAccessPage(page, perPage)
	return service.store.DashboardAccessEmployees(ctx, actor.TenantID, page, perPage)
}

func (service *DashboardAccessAdminService) ProvisionEmployeeAccount(ctx context.Context, actorUserID, employeeID int, input ProvisionDashboardEmployeeAccountInput) (DashboardEmployeeAccountMutationResult, error) {
	actor, err := service.adminIdentity(ctx, actorUserID)
	if err != nil {
		return DashboardEmployeeAccountMutationResult{}, err
	}
	loginIdentifier := strings.TrimSpace(input.LoginIdentifier)
	if employeeID <= 0 || !validDashboardLoginIdentifier(loginIdentifier) {
		return DashboardEmployeeAccountMutationResult{}, ErrDashboardAccessAdminInvalid
	}
	password, err := service.temporaryPassword()
	if err != nil {
		return DashboardEmployeeAccountMutationResult{}, err
	}
	hash, err := authpassword.Hash(password)
	if err != nil {
		return DashboardEmployeeAccountMutationResult{}, err
	}
	employee, err := service.store.ProvisionDashboardEmployeeAccount(ctx, ProvisionDashboardEmployeeAccountCommand{
		TenantID: actor.TenantID, ActorUserID: actor.UserID, ActorName: actor.UserName,
		EmployeeID: employeeID, LoginIdentifier: loginIdentifier, RoleIDs: uniquePositiveInts(input.RoleIDs), PasswordHash: hash,
		RequestID: strings.TrimSpace(input.RequestID),
	})
	if err != nil {
		return DashboardEmployeeAccountMutationResult{}, err
	}
	return DashboardEmployeeAccountMutationResult{Employee: employee, TemporaryPassword: password}, nil
}

func (service *DashboardAccessAdminService) UpdateEmployeeAccountStatus(ctx context.Context, actorUserID, employeeID int, input UpdateDashboardEmployeeAccountStatusInput) (DashboardEmployeeAccountMutationResult, error) {
	actor, err := service.adminIdentity(ctx, actorUserID)
	if err != nil {
		return DashboardEmployeeAccountMutationResult{}, err
	}
	if employeeID <= 0 || (input.Status != 1 && input.Status != 2) {
		return DashboardEmployeeAccountMutationResult{}, ErrDashboardAccessAdminInvalid
	}
	employee, err := service.store.UpdateDashboardEmployeeAccountStatus(ctx, UpdateDashboardEmployeeAccountStatusCommand{
		TenantID: actor.TenantID, ActorUserID: actor.UserID, ActorName: actor.UserName,
		EmployeeID: employeeID, Status: input.Status, RequestID: strings.TrimSpace(input.RequestID),
	})
	return DashboardEmployeeAccountMutationResult{Employee: employee}, err
}

func (service *DashboardAccessAdminService) ResetEmployeePassword(ctx context.Context, actorUserID, employeeID int, input ResetDashboardEmployeePasswordInput) (DashboardEmployeeAccountMutationResult, error) {
	actor, err := service.adminIdentity(ctx, actorUserID)
	if err != nil {
		return DashboardEmployeeAccountMutationResult{}, err
	}
	if employeeID <= 0 {
		return DashboardEmployeeAccountMutationResult{}, ErrDashboardAccessAdminInvalid
	}
	password, err := service.temporaryPassword()
	if err != nil {
		return DashboardEmployeeAccountMutationResult{}, err
	}
	hash, err := authpassword.Hash(password)
	if err != nil {
		return DashboardEmployeeAccountMutationResult{}, err
	}
	employee, err := service.store.ResetDashboardEmployeePassword(ctx, ResetDashboardEmployeePasswordCommand{
		TenantID: actor.TenantID, ActorUserID: actor.UserID, ActorName: actor.UserName,
		EmployeeID: employeeID, PasswordHash: hash, RequestID: strings.TrimSpace(input.RequestID),
	})
	if err != nil {
		return DashboardEmployeeAccountMutationResult{}, err
	}
	return DashboardEmployeeAccountMutationResult{Employee: employee, TemporaryPassword: password}, nil
}

func validDashboardLoginIdentifier(value string) bool {
	if len(value) != 11 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func generateDashboardTemporaryPassword() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"
	bytes := make([]byte, 12)
	random := make([]byte, len(bytes))
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate temporary password: %w", err)
	}
	for index := range bytes {
		bytes[index] = alphabet[int(random[index])%len(alphabet)]
	}
	return string(bytes), nil
}

func (service *DashboardAccessAdminService) Profile(ctx context.Context, userID, corpID int) (DashboardAccessProfile, error) {
	return service.access.Resolve(ctx, userID, corpID)
}

func (service *DashboardAccessAdminService) Catalog(ctx context.Context, actorUserID int) ([]DashboardPermissionDefinition, error) {
	if _, err := service.adminIdentity(ctx, actorUserID); err != nil {
		return nil, err
	}
	return service.store.DashboardPermissionCatalog(ctx)
}

func (service *DashboardAccessAdminService) Users(ctx context.Context, actorUserID, page, perPage int) (DashboardAccessUserPage, error) {
	actor, err := service.adminIdentity(ctx, actorUserID)
	if err != nil {
		return DashboardAccessUserPage{}, err
	}
	page, perPage = normalizeDashboardAccessPage(page, perPage)
	return service.store.DashboardAccessUsers(ctx, actor.TenantID, page, perPage)
}

func (service *DashboardAccessAdminService) User(ctx context.Context, actorUserID, targetUserID int) (DashboardAccessUserDetail, error) {
	actor, err := service.adminIdentity(ctx, actorUserID)
	if err != nil {
		return DashboardAccessUserDetail{}, err
	}
	item, found, err := service.store.DashboardAccessUser(ctx, actor.TenantID, targetUserID)
	if err != nil {
		return DashboardAccessUserDetail{}, err
	}
	if !found || item.TenantID != actor.TenantID {
		return DashboardAccessUserDetail{}, ErrDashboardAccessAdminNotFound
	}
	return item, nil
}

func (service *DashboardAccessAdminService) Roles(ctx context.Context, actorUserID, page, perPage int) (DashboardAccessRolePage, error) {
	actor, err := service.adminIdentity(ctx, actorUserID)
	if err != nil {
		return DashboardAccessRolePage{}, err
	}
	page, perPage = normalizeDashboardAccessPage(page, perPage)
	return service.store.DashboardAccessRoles(ctx, actor.TenantID, page, perPage)
}

func (service *DashboardAccessAdminService) Role(ctx context.Context, actorUserID, roleID int) (DashboardAccessRoleDetail, error) {
	actor, err := service.adminIdentity(ctx, actorUserID)
	if err != nil {
		return DashboardAccessRoleDetail{}, err
	}
	item, found, err := service.store.DashboardAccessRole(ctx, actor.TenantID, roleID)
	if err != nil {
		return DashboardAccessRoleDetail{}, err
	}
	if !found || item.TenantID != actor.TenantID {
		return DashboardAccessRoleDetail{}, ErrDashboardAccessAdminNotFound
	}
	return item, nil
}

func (service *DashboardAccessAdminService) Audits(ctx context.Context, actorUserID int, filter DashboardPermissionAuditFilter) (DashboardPermissionAuditPage, error) {
	actor, err := service.adminIdentity(ctx, actorUserID)
	if err != nil {
		return DashboardPermissionAuditPage{}, err
	}
	filter.Page, filter.PerPage = normalizeDashboardAccessPage(filter.Page, filter.PerPage)
	return service.store.DashboardPermissionAudits(ctx, actor.TenantID, filter)
}

func (service *DashboardAccessAdminService) ReplaceUserAccess(ctx context.Context, actorUserID, targetUserID int, input ReplaceUserDashboardAccessInput) (DashboardAccessUserDetail, error) {
	actor, err := service.adminIdentity(ctx, actorUserID)
	if err != nil {
		return DashboardAccessUserDetail{}, err
	}
	permissions, err := service.validateAssignments(ctx, input.DirectPermissions)
	if err != nil {
		return DashboardAccessUserDetail{}, err
	}
	if targetUserID <= 0 || input.ExpectedVersion == 0 {
		return DashboardAccessUserDetail{}, ErrDashboardAccessAdminInvalid
	}
	return service.store.ReplaceUserDashboardAccess(ctx, ReplaceUserDashboardAccessCommand{
		TenantID: actor.TenantID, ActorUserID: actor.UserID, ActorName: actor.UserName, TargetUserID: targetUserID,
		RoleIDs: uniquePositiveInts(input.RoleIDs), DirectPermissions: permissions,
		ExpectedVersion: input.ExpectedVersion, RequestID: strings.TrimSpace(input.RequestID),
	})
}

func (service *DashboardAccessAdminService) CreateRole(ctx context.Context, actorUserID int, input CreateDashboardRoleInput) (DashboardAccessRoleDetail, error) {
	actor, err := service.adminIdentity(ctx, actorUserID)
	if err != nil {
		return DashboardAccessRoleDetail{}, err
	}
	permissions, err := service.validateAssignments(ctx, input.Permissions)
	if err != nil {
		return DashboardAccessRoleDetail{}, err
	}
	if strings.TrimSpace(input.Name) == "" || !validDashboardRoleStatus(input.Status) || IsReservedDashboardRoleRemark(input.Remark) {
		return DashboardAccessRoleDetail{}, ErrDashboardAccessAdminInvalid
	}
	return service.store.CreateDashboardRole(ctx, CreateDashboardRoleCommand{
		TenantID: actor.TenantID, ActorUserID: actor.UserID, ActorName: actor.UserName,
		Name: strings.TrimSpace(input.Name), Remark: strings.TrimSpace(input.Remark), Status: input.Status,
		Permissions: permissions, RequestID: strings.TrimSpace(input.RequestID),
	})
}

func (service *DashboardAccessAdminService) UpdateRole(ctx context.Context, actorUserID, roleID int, input UpdateDashboardRoleInput) (DashboardAccessRoleDetail, error) {
	actor, err := service.adminIdentity(ctx, actorUserID)
	if err != nil {
		return DashboardAccessRoleDetail{}, err
	}
	permissions, err := service.validateAssignments(ctx, input.Permissions)
	if err != nil {
		return DashboardAccessRoleDetail{}, err
	}
	if roleID <= 0 || strings.TrimSpace(input.Name) == "" || input.ExpectedVersion == 0 || IsReservedDashboardRoleRemark(input.Remark) {
		return DashboardAccessRoleDetail{}, ErrDashboardAccessAdminInvalid
	}
	return service.store.UpdateDashboardRole(ctx, UpdateDashboardRoleCommand{
		TenantID: actor.TenantID, ActorUserID: actor.UserID, ActorName: actor.UserName, RoleID: roleID,
		Name: strings.TrimSpace(input.Name), Remark: strings.TrimSpace(input.Remark), Permissions: permissions,
		ExpectedVersion: input.ExpectedVersion, RequestID: strings.TrimSpace(input.RequestID),
	})
}

func (service *DashboardAccessAdminService) UpdateRoleStatus(ctx context.Context, actorUserID, roleID int, input UpdateDashboardRoleStatusInput) (DashboardAccessRoleDetail, error) {
	actor, err := service.adminIdentity(ctx, actorUserID)
	if err != nil {
		return DashboardAccessRoleDetail{}, err
	}
	if roleID <= 0 || input.ExpectedVersion == 0 || !validDashboardRoleStatus(input.Status) {
		return DashboardAccessRoleDetail{}, ErrDashboardAccessAdminInvalid
	}
	return service.store.UpdateDashboardRoleStatus(ctx, UpdateDashboardRoleStatusCommand{
		TenantID: actor.TenantID, ActorUserID: actor.UserID, ActorName: actor.UserName, RoleID: roleID,
		Status: input.Status, ExpectedVersion: input.ExpectedVersion, RequestID: strings.TrimSpace(input.RequestID),
	})
}

func (service *DashboardAccessAdminService) DeleteRole(ctx context.Context, actorUserID, roleID int, input DeleteDashboardRoleInput) error {
	actor, err := service.adminIdentity(ctx, actorUserID)
	if err != nil {
		return err
	}
	if roleID <= 0 || input.ExpectedVersion == 0 {
		return ErrDashboardAccessAdminInvalid
	}
	return service.store.DeleteDashboardRole(ctx, DeleteDashboardRoleCommand{
		TenantID: actor.TenantID, ActorUserID: actor.UserID, ActorName: actor.UserName, RoleID: roleID,
		ExpectedVersion: input.ExpectedVersion, RequestID: strings.TrimSpace(input.RequestID),
	})
}

func (service *DashboardAccessAdminService) adminIdentity(ctx context.Context, actorUserID int) (DashboardAccessIdentity, error) {
	if service == nil || service.store == nil || actorUserID <= 0 {
		return DashboardAccessIdentity{}, ErrUnauthorized
	}
	identity, found, err := service.store.DashboardAccessIdentity(ctx, actorUserID)
	if err != nil {
		return DashboardAccessIdentity{}, err
	}
	if !found || identity.UserID != actorUserID || identity.TenantID <= 0 || identity.Status != 1 {
		return DashboardAccessIdentity{}, ErrUnauthorized
	}
	if !identity.IsSuperAdmin {
		return DashboardAccessIdentity{}, ErrDashboardAccessAdminForbidden
	}
	return identity, nil
}

func (service *DashboardAccessAdminService) validateAssignments(ctx context.Context, assignments []DashboardPermissionAssignment) ([]DashboardPermissionAssignment, error) {
	catalog, err := service.store.DashboardPermissionCatalog(ctx)
	if err != nil {
		return nil, err
	}
	byCode := make(map[string]DashboardPermissionDefinition, len(catalog))
	for _, permission := range catalog {
		byCode[permission.Code] = permission
	}
	resultByCode := make(map[string]DashboardPermissionAssignment, len(assignments))
	for _, assignment := range assignments {
		assignment.Code = strings.TrimSpace(assignment.Code)
		permission, found := byCode[assignment.Code]
		if !found || permission.SuperadminOnly {
			return nil, ErrDashboardAccessAdminInvalid
		}
		if assignment.Scope == "" {
			assignment.Scope = DataScopeSelf
		}
		if assignment.Scope != DataScopeSelf && assignment.Scope != DataScopeDepartment && assignment.Scope != DataScopeTenant {
			return nil, ErrDashboardAccessAdminInvalid
		}
		resultByCode[assignment.Code] = assignment
	}
	result := make([]DashboardPermissionAssignment, 0, len(resultByCode))
	for _, assignment := range resultByCode {
		result = append(result, assignment)
	}
	sort.Slice(result, func(left, right int) bool { return result[left].Code < result[right].Code })
	return result, nil
}

func normalizeDashboardAccessPage(page, perPage int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}
	return page, perPage
}

func validDashboardRoleStatus(status int) bool { return status == 1 || status == 2 }
