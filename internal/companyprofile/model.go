package companyprofile

import (
	"context"
	"errors"
	"time"

	"jiyi/mochat-go/internal/dashboardprincipal"
)

const (
	CodeInvalidRequest       = "INVALID_REQUEST"
	CodePermissionDenied     = "PERMISSION_DENIED"
	CodeTenantAccessDenied   = "TENANT_ACCESS_DENIED"
	CodeNotFound             = "NOT_FOUND"
	CodeVersionConflict      = "VERSION_CONFLICT"
	CodeCorpIDImmutable      = "CORP_ID_IMMUTABLE"
	CodeWeComCredentialError = "WECOM_CREDENTIAL_INVALID"
	CodeInternal             = "INTERNAL_ERROR"
)

var (
	ErrInvalidRequest      = errors.New("company profile request is invalid")
	ErrPermissionDenied    = errors.New("company profile permission denied")
	ErrTenantAccessDenied  = errors.New("company profile tenant access denied")
	ErrNotFound            = errors.New("company profile not found")
	ErrVersionConflict     = errors.New("company profile version conflict")
	ErrCorpIDImmutable     = errors.New("company profile CorpID is immutable")
	ErrCredentialInvalid   = errors.New("company profile WeCom credential invalid")
	ErrStoreUnavailable    = errors.New("company profile store unavailable")
	ErrVerifierUnavailable = errors.New("company profile verifier unavailable")
)

type Profile struct {
	TenantID              int                `json:"tenantId"`
	CorpID                int                `json:"corpId"`
	DisplayName           string             `json:"displayName"`
	AuthoritativeCorpName string             `json:"authoritativeCorpName,omitempty"`
	WXCorpID              string             `json:"wxCorpId,omitempty"`
	BindingStatus         string             `json:"bindingStatus"`
	BindingVersion        uint64             `json:"bindingVersion"`
	VerifiedAt            *time.Time         `json:"verifiedAt,omitempty"`
	Credentials           CredentialStatuses `json:"credentials"`
	UpdatedAt             time.Time          `json:"updatedAt"`
}

type CredentialStatuses struct {
	WeCom   CredentialStatus `json:"wecom"`
	Agent   CredentialStatus `json:"agent"`
	Archive CredentialStatus `json:"archive"`
}

type CredentialStatus struct {
	Configured bool       `json:"configured"`
	KeyID      string     `json:"keyId,omitempty"`
	UpdatedAt  *time.Time `json:"updatedAt,omitempty"`
}

type UpdateProfileInput struct {
	DisplayName     string `json:"displayName"`
	ExpectedVersion uint64 `json:"expectedVersion"`
	RequestID       string `json:"requestId,omitempty"`
}

type WeComCredentialsInput struct {
	WXCorpID        *string `json:"wxCorpId,omitempty"`
	EmployeeSecret  *string `json:"employeeSecret"`
	ContactSecret   *string `json:"contactSecret"`
	CallbackToken   *string `json:"callbackToken"`
	EncodingAESKey  *string `json:"encodingAESKey"`
	ChatSecret      *string `json:"chatSecret"`
	ExpectedVersion uint64  `json:"expectedVersion"`
	RequestID       string  `json:"requestId,omitempty"`
}

type AgentCredentialsInput struct {
	AgentID         int     `json:"agentId"`
	WXAgentID       string  `json:"wxAgentId,omitempty"`
	WXSecret        *string `json:"wxSecret"`
	ExpectedVersion uint64  `json:"expectedVersion"`
	RequestID       string  `json:"requestId,omitempty"`
}

type ArchiveCredentialsInput struct {
	ChatSecret      *string `json:"chatSecret"`
	ExpectedVersion uint64  `json:"expectedVersion"`
	RequestID       string  `json:"requestId,omitempty"`
}

type VerifyInput struct {
	WXCorpID        string `json:"wxCorpId,omitempty"`
	ExpectedVersion uint64 `json:"expectedVersion"`
	RequestID       string `json:"requestId,omitempty"`
}

type VerificationCredentials struct {
	EmployeeSecret string
	ContactSecret  string
	CallbackToken  string
	EncodingAESKey string
	ChatSecret     string
}

type VerificationSnapshot struct {
	Verified       bool
	WXCorpID       string
	BindingVersion uint64
	Credentials    VerificationCredentials
}

type VerificationRequest struct {
	TenantID    int
	CorpID      int
	WXCorpID    string
	Credentials VerificationCredentials
}

type VerificationResult struct {
	WXCorpID string
	CorpName string
}

type WeComVerifier interface {
	Verify(context.Context, VerificationRequest) (VerificationResult, error)
}

type AuditFilter struct {
	Page    int
	PerPage int
}

type AuditPage struct {
	Items   []Audit `json:"items"`
	Page    int     `json:"page"`
	PerPage int     `json:"perPage"`
	Total   int     `json:"total"`
}

type Audit struct {
	ID              int64     `json:"id"`
	Action          string    `json:"action"`
	TargetType      string    `json:"targetType"`
	TargetID        string    `json:"targetId"`
	ChangedFields   []string  `json:"changedFields,omitempty"`
	ExpectedVersion *uint64   `json:"expectedVersion,omitempty"`
	ResultVersion   *uint64   `json:"resultVersion,omitempty"`
	RequestID       string    `json:"requestId,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
}

type Store interface {
	GetProfile(context.Context, dashboardprincipal.DashboardPrincipal) (Profile, error)
	UpdateProfile(context.Context, dashboardprincipal.DashboardPrincipal, UpdateProfileInput) (Profile, error)
	GetVerificationSnapshot(context.Context, dashboardprincipal.DashboardPrincipal) (VerificationSnapshot, error)
	CommitVerification(context.Context, dashboardprincipal.DashboardPrincipal, VerifyInput, VerificationResult) (Profile, error)
	RotateWeComCredentials(context.Context, dashboardprincipal.DashboardPrincipal, WeComCredentialsInput) (Profile, error)
	RotateAgentCredentials(context.Context, dashboardprincipal.DashboardPrincipal, AgentCredentialsInput) (Profile, error)
	RotateArchiveCredentials(context.Context, dashboardprincipal.DashboardPrincipal, ArchiveCredentialsInput) (Profile, error)
	ListAudits(context.Context, dashboardprincipal.DashboardPrincipal, AuditFilter) (AuditPage, error)
}

type SyncResult struct {
	Status             string    `json:"status"`
	Cursor             string    `json:"cursor,omitempty"`
	DepartmentsCreated int       `json:"departmentsCreated"`
	DepartmentsUpdated int       `json:"departmentsUpdated"`
	EmployeesCreated   int       `json:"employeesCreated"`
	EmployeesUpdated   int       `json:"employeesUpdated"`
	StartedAt          time.Time `json:"startedAt"`
	FinishedAt         time.Time `json:"finishedAt"`
	ErrorCode          string    `json:"errorCode,omitempty"`
}

type SyncStatus struct {
	Status      string     `json:"status"`
	Cursor      string     `json:"cursor,omitempty"`
	Departments int        `json:"departments"`
	Employees   int        `json:"employees"`
	StartedAt   *time.Time `json:"startedAt,omitempty"`
	FinishedAt  *time.Time `json:"finishedAt,omitempty"`
	ErrorCode   string     `json:"errorCode,omitempty"`
}

type SyncDepartment struct {
	WXDepartmentID int
	Name           string
	WXParentID     int
	Order          int
}

type SyncEmployee struct {
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
	Status               int
	QRCode               string
	Address              string
	OpenUserID           string
	WXMainDepartmentID   int
	DepartmentIDs        []int
	IsLeaderInDepartment []int
	DepartmentOrders     []int
}

type EmployeeSyncData struct {
	Departments []SyncDepartment
	Employees   []SyncEmployee
}

type EmployeeSyncScheduler interface {
	EnqueueEmployeeSync(context.Context, int) (string, error)
}

type EmployeeSyncQueueResult struct {
	Cursor        string
	AlreadyQueued bool
}

type SyncStore interface {
	GetSyncStatus(context.Context, dashboardprincipal.DashboardPrincipal) (SyncStatus, error)
}

type EmployeeSyncQueueStore interface {
	QueueEmployeeSync(context.Context, dashboardprincipal.DashboardPrincipal) (EmployeeSyncQueueResult, error)
	RecordEmployeeSyncFailure(context.Context, dashboardprincipal.DashboardPrincipal) error
}
