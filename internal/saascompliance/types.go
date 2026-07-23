package saascompliance

import (
	"context"
	"io"
	"time"
)

const (
	InventoryVersion = "v1"

	PolicyStatusActive   = "active"
	PolicyStatusDisabled = "disabled"

	ExportStatusPending   = "pending"
	ExportStatusRunning   = "running"
	ExportStatusSucceeded = "succeeded"
	ExportStatusFailed    = "failed"
	ExportStatusExpired   = "expired"
	ExportStatusDeleted   = "deleted"

	ExportDeletionStatusPending   = "pending"
	ExportDeletionStatusRunning   = "running"
	ExportDeletionStatusSucceeded = "succeeded"
	ExportDeletionStatusFailed    = "failed"

	ExportDeletionStepPending = "pending"
	ExportDeletionStepDeleted = "deleted"
	ExportDeletionStepMissing = "missing"
	ExportDeletionStepFailed  = "failed"

	HoldStatusActive   = "active"
	HoldStatusReleased = "released"
	HoldStatusExpired  = "expired"

	ErasureStatusPendingApproval = "pending_approval"
	ErasureStatusApproved        = "approved"
	ErasureStatusWaiting         = "waiting"
	ErasureStatusRunning         = "running"
	ErasureStatusSucceeded       = "succeeded"
	ErasureStatusFailed          = "failed"
	ErasureStatusBlocked         = "blocked"
	ErasureStatusCanceled        = "canceled"

	StepStatusPending   = "pending"
	StepStatusRunning   = "running"
	StepStatusSucceeded = "succeeded"
	StepStatusFailed    = "failed"

	DatasetActionDelete    = "delete"
	DatasetActionRedact    = "redact"
	DatasetActionRetain    = "retain"
	DatasetActionTombstone = "tombstone"
	DatasetActionVerify    = "verify"
	DatasetActionArtifacts = "artifacts"
	DatasetActionFiles     = "files"

	RetentionOperational = "operational"
	RetentionFinancial   = "financial"
	RetentionAudit       = "audit"

	ArtifactFormat = "tar.gz.mgce"

	OperationActionPolicyUpdate      = "saas.admin.compliance.policy.update"
	OperationActionHoldCreate        = "saas.admin.compliance.hold.create"
	OperationActionHoldRelease       = "saas.admin.compliance.hold.release"
	OperationActionExportRequest     = "saas.admin.compliance.export.request"
	OperationActionExportComplete    = "saas.admin.compliance.export.complete"
	OperationActionExportDownload    = "saas.admin.compliance.export.download"
	OperationActionExportDeletePlan  = "saas.admin.compliance.export.delete.schedule"
	OperationActionExportDeleteRetry = "saas.admin.compliance.export.delete.retry"
	OperationActionExportDelete      = "saas.admin.compliance.export.delete"
	OperationActionErasureRequest    = "saas.admin.compliance.erasure.request"
	OperationActionErasureAuthorize  = "saas.admin.compliance.erasure.authorize"
	OperationActionErasureCancel     = "saas.admin.compliance.erasure.cancel"
	OperationActionErasureComplete   = "saas.admin.compliance.erasure.complete"
)

type OperationError struct {
	Code    string
	Message string
}

func (e *OperationError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func Invalid(message string) error     { return &OperationError{Code: "invalid", Message: message} }
func NotFound(message string) error    { return &OperationError{Code: "not_found", Message: message} }
func Conflict(message string) error    { return &OperationError{Code: "conflict", Message: message} }
func Unavailable(message string) error { return &OperationError{Code: "unavailable", Message: message} }

type Actor struct {
	UserID   int
	TenantID int
}

type Policy struct {
	ID                               int64
	Status                           string
	ExportRetentionDays              int
	ErasureGraceDays                 int
	RequireRecentExport              bool
	RecentExportMaxAgeDays           int
	BillingRetentionDays             int
	AuditRetentionDays               int
	ServiceAccountUsageRetentionDays int
	Version                          int
	UpdatedBy                        int
	CreatedAt                        string
	UpdatedAt                        string
}

type PolicyUpdate struct {
	Status                           string `json:"status"`
	ExportRetentionDays              int    `json:"exportRetentionDays"`
	ErasureGraceDays                 int    `json:"erasureGraceDays"`
	RequireRecentExport              bool   `json:"requireRecentExport"`
	RecentExportMaxAgeDays           int    `json:"recentExportMaxAgeDays"`
	BillingRetentionDays             int    `json:"billingRetentionDays"`
	AuditRetentionDays               int    `json:"auditRetentionDays"`
	ServiceAccountUsageRetentionDays int    `json:"serviceAccountUsageRetentionDays"`
	ExpectedVersion                  int    `json:"expectedVersion"`
	Actor                            Actor  `json:"-"`
	ApprovalExecutionID              int64  `json:"-"`
	ApprovalExecutionVersion         int    `json:"-"`
}

type Tenant struct {
	ID        int
	Name      string
	Status    int
	CreatedAt string
	UpdatedAt string
	DeletedAt string
}

type LegalHold struct {
	ID            int64
	HoldNo        string
	TenantID      int
	TenantName    string
	Status        string
	Reason        string
	StartsAt      string
	StartsAtValue time.Time
	ExpiresAt     string
	ExpiresValue  time.Time
	ReleasedAt    string
	ReleasedBy    int
	ReleaseReason string
	Version       int
	CreatedBy     int
	UpdatedBy     int
	CreatedAt     string
	UpdatedAt     string
	OperationID   int64
}

type LegalHoldCreate struct {
	HoldNo    string
	TenantID  int
	Reason    string
	StartsAt  time.Time
	ExpiresAt time.Time
	Actor     Actor
}

type LegalHoldRelease struct {
	HoldID                   int64
	Reason                   string
	ExpectedVersion          int
	Actor                    Actor
	ApprovalExecutionID      int64
	ApprovalExecutionVersion int
	ApprovalPlan             *LegalHoldReleasePlan
}

type LegalHoldReleasePlan struct {
	HoldID          int64  `json:"holdId"`
	HoldNo          string `json:"holdNo"`
	TenantID        int    `json:"tenantId"`
	TenantName      string `json:"tenantName"`
	Status          string `json:"status"`
	HoldReason      string `json:"holdReason"`
	StartsAt        string `json:"startsAt"`
	ExpiresAt       string `json:"expiresAt"`
	ExpectedVersion int    `json:"expectedVersion"`
	ReleaseReason   string `json:"releaseReason"`
}

type DataExport struct {
	ID                     int64
	ExportNo               string
	TenantID               int
	TenantName             string
	Status                 string
	ArtifactName           string
	ArtifactFormat         string
	Encrypted              bool
	EncryptionKeyID        string
	EncryptionKeyReady     bool
	SHA256                 string
	SizeBytes              int64
	ManifestSHA256         string
	InventoryVersion       string
	TableCount             int
	RowCount               int64
	FileCount              int
	FileSizeBytes          int64
	RequestReason          string
	RequestedBy            int
	ActorTenantID          int
	StartedAt              string
	FinishedAt             string
	FinishedAtValue        time.Time
	ExpiresAt              string
	ExpiresAtValue         time.Time
	ErrorMessage           string
	DownloadCount          int
	LastDownloadedAt       string
	LastDownloadedBy       int
	OperationID            int64
	DeletionStatus         string
	DeletionArtifactStatus string
	DeletionRecordStatus   string
	DeletionAttempts       int
	DeletionApprovalID     int64
	DeletionLeaseExpiresAt string
	DeletionLeaseValue     time.Time
	DeletionRequestedAt    string
	DeletionStartedAt      string
	DeletionFinishedAt     string
	DeletionLastError      string
	DeletionRequestedBy    int
	DeletionActorTenantID  int
	Version                int
	CreatedAt              string
	UpdatedAt              string
	DeletedAt              string
}

type DataExportCreate struct {
	ExportNo         string
	TenantID         int
	TenantName       string
	ArtifactName     string
	EncryptionKeyID  string
	InventoryVersion string
	Reason           string
	Actor            Actor
}

type DataExportCompletion struct {
	ExportID       int64
	Status         string
	SHA256         string
	SizeBytes      int64
	ManifestSHA256 string
	TableCount     int
	RowCount       int64
	FileCount      int
	FileSizeBytes  int64
	ErrorMessage   string
	FinishedAt     time.Time
	ExpiresAt      time.Time
	Actor          Actor
}

type DataExportDeletionPlan struct {
	ExportID     int64  `json:"exportId"`
	ExportNo     string `json:"exportNo"`
	TenantID     int    `json:"tenantId"`
	TenantName   string `json:"tenantName"`
	Status       string `json:"status"`
	ArtifactName string `json:"artifactName"`
	SHA256       string `json:"sha256"`
	SizeBytes    int64  `json:"sizeBytes"`
	FinishedAt   string `json:"finishedAt"`
	ExpiresAt    string `json:"expiresAt"`
}

type DataExportDeletionSchedule struct {
	Plan                     DataExportDeletionPlan
	Actor                    Actor
	ApprovalExecutionID      int64
	ApprovalExecutionVersion int
}

type DataExportDeletionCheckpoint struct {
	ExportID     int64
	Attempt      int
	Status       string
	ErrorMessage string
	Actor        Actor
}

type DataExportDeletionCompletion struct {
	ExportID int64
	Attempt  int
	Actor    Actor
}

type ErasureRequest struct {
	ID                 int64
	RequestNo          string
	TenantID           int
	TenantName         string
	Status             string
	Reason             string
	EligibleAt         string
	EligibleAtValue    time.Time
	LatestExportID     int64
	ApprovalID         int64
	ApprovedAt         string
	ApprovedBy         int
	InventoryVersion   string
	TotalSteps         int
	CompletedSteps     int
	DeletedRows        int64
	RedactedRows       int64
	VerificationSHA256 string
	ReportJSON         string
	LastError          string
	RequestedBy        int
	ActorTenantID      int
	StartedAt          string
	FinishedAt         string
	OperationID        int64
	Version            int
	CreatedAt          string
	UpdatedAt          string
}

type ErasureCreate struct {
	RequestNo        string
	TenantID         int
	TenantName       string
	Reason           string
	ConfirmationSHA  string
	EligibleAt       time.Time
	LatestExportID   int64
	InventoryVersion string
	Actor            Actor
}

type ErasureAuthorize struct {
	RequestID       int64
	ApprovalID      int64
	ApprovalVersion int
	ApprovalUserID  int
	Actor           Actor
}

type ErasureStep struct {
	ID           int64  `json:"id"`
	RequestID    int64  `json:"requestId"`
	StepOrder    int    `json:"stepOrder"`
	StepKey      string `json:"stepKey"`
	TableName    string `json:"tableName"`
	Action       string `json:"action"`
	Status       string `json:"status"`
	AffectedRows int64  `json:"affectedRows"`
	StartedAt    string `json:"startedAt"`
	FinishedAt   string `json:"finishedAt"`
	ErrorMessage string `json:"errorMessage"`
}

type DatasetSpec struct {
	Key            string
	Table          string
	Predicate      string
	OrderBy        string
	Action         string
	RetentionClass string
	DeleteOrder    int
	Export         bool
}

type DatasetRowIterator interface {
	Columns() []string
	Next() bool
	Values() ([]any, error)
	Err() error
	Close() error
}

type StorageFile struct {
	ID           int64
	RelativePath string
	OriginalName string
	SizeBytes    int64
}

type InventoryCoverage struct {
	OwnedTables   []string
	CoveredTables []string
	UnknownTables []string
}

type RetentionBlock struct {
	Category    string
	Table       string
	RecordCount int64
	RetainUntil string
}

type ConfigStatus struct {
	ArtifactRootConfigured bool
	FileStorageConfigured  bool
	EncryptionConfigured   bool
	EncryptionKeyID        string
	EncryptionKeyCount     int
	InventoryVersion       string
	InventoryTableCount    int
	InventoryUnknownTables []string
}

type Overview struct {
	Policy   Policy
	Holds    []LegalHold
	Exports  []DataExport
	Erasures []ErasureRequest
	Config   ConfigStatus
}

type ProcessResult struct {
	ExportsProcessed         int
	ExportDeletionsProcessed int
	ErasuresProcessed        int
	ArtifactsDeleted         int
	ExportIDs                []int64
	ExportDeletionIDs        []int64
	ErasureIDs               []int64
}

type ExportDownload struct {
	Export   DataExport
	Filename string
	Reader   io.ReadCloser
}

type Store interface {
	CompliancePolicy(context.Context) (Policy, error)
	UpdateCompliancePolicy(context.Context, PolicyUpdate) (Policy, error)
	ComplianceTenant(context.Context, int) (Tenant, error)
	ComplianceInventoryCoverage(context.Context, []DatasetSpec) (InventoryCoverage, error)
	OpenComplianceDataset(context.Context, DatasetSpec, int) (DatasetRowIterator, error)
	ComplianceStorageFiles(context.Context, int) ([]StorageFile, error)

	ComplianceLegalHolds(context.Context, int, int) ([]LegalHold, error)
	ComplianceLegalHold(context.Context, int64) (LegalHold, error)
	CreateComplianceLegalHold(context.Context, LegalHoldCreate) (LegalHold, error)
	ReleaseComplianceLegalHold(context.Context, LegalHoldRelease) (LegalHold, error)
	ActiveComplianceLegalHold(context.Context, int, time.Time) (LegalHold, bool, error)

	CreateComplianceExport(context.Context, DataExportCreate) (DataExport, error)
	ClaimComplianceExport(context.Context, int64, time.Time) (DataExport, error)
	CompleteComplianceExport(context.Context, DataExportCompletion) (DataExport, error)
	ComplianceExport(context.Context, int64) (DataExport, error)
	ComplianceExports(context.Context, int, int) ([]DataExport, error)
	NextPendingComplianceExport(context.Context) (DataExport, bool, error)
	RecordComplianceExportDownload(context.Context, int64, Actor) (DataExport, error)
	MarkComplianceExportDeleted(context.Context, int64, Actor) (DataExport, error)
	ScheduleComplianceExportDeletion(context.Context, DataExportDeletionSchedule) (DataExport, error)
	RunnableComplianceExportDeletions(context.Context, int) ([]DataExport, error)
	ClaimComplianceExportDeletion(context.Context, int64, bool, Actor) (DataExport, error)
	CheckpointComplianceExportDeletion(context.Context, DataExportDeletionCheckpoint) (DataExport, error)
	CompleteComplianceExportDeletion(context.Context, DataExportDeletionCompletion) (DataExport, error)

	CreateComplianceErasure(context.Context, ErasureCreate) (ErasureRequest, error)
	AuthorizeComplianceErasure(context.Context, ErasureAuthorize) (ErasureRequest, error)
	CancelComplianceErasure(context.Context, int64, string, Actor) (ErasureRequest, error)
	ComplianceErasure(context.Context, int64) (ErasureRequest, error)
	ComplianceErasures(context.Context, int, int) ([]ErasureRequest, error)
	NextDueComplianceErasure(context.Context, time.Time) (ErasureRequest, bool, error)
	ComplianceRetentionBlocks(context.Context, int, Policy, time.Time) ([]RetentionBlock, error)
	PrepareComplianceErasure(context.Context, int64, []DatasetSpec, time.Time) (ErasureRequest, error)
	ComplianceErasureSteps(context.Context, int64) ([]ErasureStep, error)
	ApplyComplianceErasureStep(context.Context, ErasureRequest, DatasetSpec, time.Time) (ErasureStep, error)
	ApplyComplianceErasureStorageFiles(context.Context, ErasureRequest, int, time.Time) (ErasureStep, error)
	ApplyComplianceErasureArtifacts(context.Context, ErasureRequest, int, time.Time) (ErasureStep, error)
	FinalizeComplianceErasure(context.Context, ErasureRequest, string, string, string, string, Actor, time.Time) (ErasureRequest, error)
	BlockComplianceErasure(context.Context, int64, string, time.Time) error
	FailComplianceErasure(context.Context, int64, string, time.Time) error
}
