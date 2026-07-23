package saasbackup

import (
	"context"
	"io"
	"time"
)

const (
	PolicyStatusActive   = "active"
	PolicyStatusDisabled = "disabled"

	TriggerManual      = "manual"
	TriggerCron        = "cron"
	TriggerMaintenance = "maintenance"

	RunStatusRunning   = "running"
	RunStatusSucceeded = "succeeded"
	RunStatusFailed    = "failed"
	RunStatusDeleted   = "deleted"

	VerificationPending = "pending"
	VerificationPassed  = "passed"
	VerificationFailed  = "failed"

	ReplicaStatusDisabled  = "disabled"
	ReplicaStatusPending   = "pending"
	ReplicaStatusUploading = "uploading"
	ReplicaStatusSucceeded = "succeeded"
	ReplicaStatusFailed    = "failed"
	ReplicaStatusDeleted   = "deleted"

	DrillStatusRunning   = "running"
	DrillStatusSucceeded = "succeeded"
	DrillStatusFailed    = "failed"

	RestoreTargetLifecyclePreconfigured = "preconfigured"
	RestoreTargetLifecycleEphemeral     = "ephemeral"
	RestoreCleanupNotRequired           = "not_required"
	RestoreCleanupPending               = "pending"
	RestoreCleanupSucceeded             = "succeeded"
	RestoreCleanupFailed                = "failed"
	RestoreCleanupRetained              = "retained"

	ArtifactFormatEncrypted = "sql.gz.mgbk"
	ArtifactFormatPlain     = "sql.gz"

	OperationActionPolicyUpdate    = "saas.admin.backup.policy.update"
	OperationActionCreate          = "saas.admin.backup.create"
	OperationActionVerify          = "saas.admin.backup.verify"
	OperationActionReplicate       = "saas.admin.backup.replicate"
	OperationActionDelete          = "saas.admin.backup.delete"
	OperationActionRestoreDrill    = "saas.admin.backup.restore_drill"
	OperationActionCleanupSchedule = "saas.admin.backup.cleanup.schedule"
	OperationActionCleanupRetry    = "saas.admin.backup.cleanup.retry"
	OperationActionCleanupFinish   = "saas.admin.backup.cleanup.finish"

	CleanupStatusPending   = "pending"
	CleanupStatusRunning   = "running"
	CleanupStatusSucceeded = "succeeded"
	CleanupStatusPartial   = "partial"
	CleanupStatusFailed    = "failed"

	CleanupItemStatusPending   = "pending"
	CleanupItemStatusRunning   = "running"
	CleanupItemStatusSucceeded = "succeeded"
	CleanupItemStatusFailed    = "failed"

	CleanupStepPending     = "pending"
	CleanupStepNotRequired = "not_required"
	CleanupStepDeleted     = "deleted"
	CleanupStepMissing     = "missing"
	CleanupStepFailed      = "failed"

	CleanupStepReplica = "replica"
	CleanupStepLocal   = "local"
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
	ID                       int64
	Status                   string
	IntervalMinutes          int
	RetentionDays            int
	MinSuccessfulBackups     int
	MaxBackupAgeMinutes      int
	RestoreDrillIntervalDays int
	RequireEncryption        bool
	RequireOffsiteReplica    bool
	Version                  int
	UpdatedBy                int
	CreatedAt                string
	UpdatedAt                string
}

type PolicyUpdate struct {
	Status                   string `json:"status"`
	IntervalMinutes          int    `json:"intervalMinutes"`
	RetentionDays            int    `json:"retentionDays"`
	MinSuccessfulBackups     int    `json:"minSuccessfulBackups"`
	MaxBackupAgeMinutes      int    `json:"maxBackupAgeMinutes"`
	RestoreDrillIntervalDays int    `json:"restoreDrillIntervalDays"`
	RequireEncryption        bool   `json:"requireEncryption"`
	RequireOffsiteReplica    bool   `json:"requireOffsiteReplica"`
	ExpectedVersion          int    `json:"expectedVersion"`
	Actor                    Actor  `json:"-"`
	ApprovalExecutionID      int64  `json:"-"`
	ApprovalExecutionVersion int    `json:"-"`
}

type SourceMetadata struct {
	DatabaseName     string
	MigrationVersion string
	MigrationCount   int
	TableCount       int
}

type BackupRun struct {
	ID                 int64
	BackupNo           string
	TriggerType        string
	Status             string
	ArtifactName       string
	ArtifactFormat     string
	Encrypted          bool
	EncryptionKeyID    string
	SHA256             string
	SizeBytes          int64
	ReplicaStatus      string
	ReplicaProvider    string
	ReplicaBucket      string
	ReplicaObjectKey   string
	ReplicaETag        string
	ReplicaVersionID   string
	ReplicaSHA256      string
	ReplicaSizeBytes   int64
	ReplicatedAt       string
	ReplicaVerifiedAt  string
	ReplicaError       string
	EncryptionKeyReady bool
	DatabaseName       string
	MigrationVersion   string
	MigrationCount     int
	TableCount         int
	VerificationStatus string
	VerifiedAt         string
	VerificationError  string
	ErrorMessage       string
	StartedAt          string
	FinishedAt         string
	CreatedBy          int
	ActorTenantID      int
	OperationID        int64
	CleanupRunID       int64
	Version            int
	CreatedAt          string
	UpdatedAt          string
	DeletedAt          string
}

type BackupStart struct {
	BackupNo        string
	TriggerType     string
	ArtifactName    string
	ArtifactFormat  string
	Encrypted       bool
	EncryptionKeyID string
	ReplicaStatus   string
	ReplicaProvider string
	Metadata        SourceMetadata
	StartedAt       time.Time
	Actor           Actor
}

type BackupCompletion struct {
	RunID        int64
	Status       string
	SHA256       string
	SizeBytes    int64
	ErrorMessage string
	FinishedAt   time.Time
	Actor        Actor
}

type BackupVerification struct {
	RunID        int64
	Status       string
	ErrorMessage string
	VerifiedAt   time.Time
	Actor        Actor
}

type BackupReplicaStart struct {
	RunID     int64
	Provider  string
	Bucket    string
	ObjectKey string
	StartedAt time.Time
	Actor     Actor
}

type BackupReplicaCompletion struct {
	RunID        int64
	Status       string
	ETag         string
	VersionID    string
	SHA256       string
	SizeBytes    int64
	ReplicatedAt time.Time
	VerifiedAt   time.Time
	ErrorMessage string
	Actor        Actor
}

type ReplicaObject struct {
	Provider  string
	Bucket    string
	ObjectKey string
	ETag      string
	VersionID string
	SHA256    string
	SizeBytes int64
}

type ReplicaStore interface {
	Provider() string
	Bucket() string
	ObjectKey(artifactName string, createdAt time.Time) (string, error)
	Probe(ctx context.Context) error
	Put(ctx context.Context, objectKey, localPath, sha256 string, sizeBytes int64) (ReplicaObject, error)
	Verify(ctx context.Context, objectKey, versionID, sha256 string, sizeBytes int64) (ReplicaObject, error)
	Open(ctx context.Context, objectKey, versionID string) (ReplicaObject, io.ReadCloser, error)
	Delete(ctx context.Context, objectKey, versionID string) error
}

type RestoreDrill struct {
	ID                       int64
	DrillNo                  string
	BackupRunID              int64
	TriggerType              string
	Status                   string
	TargetFingerprint        string
	TargetDatabase           string
	TargetLifecycle          string
	TargetCleanupStatus      string
	TargetCleanedAt          string
	TargetCleanupError       string
	ExpectedMigrationVersion string
	ActualMigrationVersion   string
	ExpectedMigrationCount   int
	ActualMigrationCount     int
	ExpectedTableCount       int
	ActualTableCount         int
	ChecksJSON               string
	ErrorMessage             string
	StartedAt                string
	FinishedAt               string
	DurationMS               int64
	CreatedBy                int
	ActorTenantID            int
	OperationID              int64
	CreatedAt                string
}

type RestoreDrillStart struct {
	DrillNo             string
	BackupRun           BackupRun
	TriggerType         string
	TargetFingerprint   string
	TargetDatabase      string
	TargetLifecycle     string
	TargetCleanupStatus string
	StartedAt           time.Time
	Actor               Actor
}

type RestoreDrillCompletion struct {
	DrillID                int64
	Status                 string
	ActualMigrationVersion string
	ActualMigrationCount   int
	ActualTableCount       int
	Checks                 map[string]any
	ErrorMessage           string
	FinishedAt             time.Time
	DurationMS             int64
	TargetCleanupStatus    string
	TargetCleanedAt        time.Time
	TargetCleanupError     string
	Actor                  Actor
}

type RetentionResult struct {
	Scanned         int
	Deleted         int
	ReplicasDeleted int
	Preserved       int
	MissingFiles    int
	RunIDs          []int64
}

type CleanupPlanItem struct {
	BackupRunID      int64  `json:"backupRunId"`
	BackupRunVersion int    `json:"backupRunVersion"`
	BackupNo         string `json:"backupNo"`
	Status           string `json:"status"`
	FinishedAt       string `json:"finishedAt"`
	ArtifactName     string `json:"artifactName"`
	SHA256           string `json:"sha256"`
	SizeBytes        int64  `json:"sizeBytes"`
	ReplicaStatus    string `json:"replicaStatus"`
	ReplicaProvider  string `json:"replicaProvider"`
	ReplicaBucket    string `json:"replicaBucket"`
	ReplicaObjectKey string `json:"replicaObjectKey"`
	ReplicaVersionID string `json:"replicaVersionId"`
}

type CleanupPlan struct {
	PolicyVersion int               `json:"policyVersion"`
	CutoffAt      time.Time         `json:"cutoffAt"`
	Scanned       int               `json:"scanned"`
	Preserved     int               `json:"preserved"`
	Items         []CleanupPlanItem `json:"items"`
}

type CleanupSchedule struct {
	CleanupNo                string
	Plan                     CleanupPlan
	Actor                    Actor
	ApprovalExecutionID      int64
	ApprovalExecutionVersion int
}

type CleanupRun struct {
	ID                   int64
	CleanupNo            string
	Status               string
	PolicyVersion        int
	CutoffAt             string
	ScannedCount         int
	CandidateCount       int
	PreservedCount       int
	DeletedCount         int
	FailedCount          int
	ReplicasDeletedCount int
	MissingFilesCount    int
	Attempts             int
	ApprovalID           int64
	LeaseExpiresAt       string
	StartedAt            string
	FinishedAt           string
	LastError            string
	CreatedBy            int
	ActorTenantID        int
	OperationID          int64
	Version              int
	CreatedAt            string
	UpdatedAt            string
}

type CleanupItem struct {
	ID               int64
	CleanupRunID     int64
	BackupRunID      int64
	BackupRunVersion int
	BackupNo         string
	ArtifactName     string
	ReplicaObjectKey string
	ReplicaVersionID string
	Status           string
	ReplicaStatus    string
	LocalStatus      string
	RecordStatus     string
	Attempts         int
	LastError        string
	ReplicaDeletedAt string
	LocalDeletedAt   string
	RecordDeletedAt  string
	OperationID      int64
	CreatedAt        string
	UpdatedAt        string
}

type CleanupCheckpoint struct {
	CleanupRunID   int64
	CleanupAttempt int
	ItemID         int64
	Step           string
	Status         string
	ErrorMessage   string
	Actor          Actor
}

type CleanupItemCompletion struct {
	CleanupRunID   int64
	CleanupAttempt int
	ItemID         int64
	Actor          Actor
}

type CleanupRunCompletion struct {
	CleanupRunID   int64
	CleanupAttempt int
	Actor          Actor
}

type ConfigStatus struct {
	SourceConfigured       bool
	BackupRootConfigured   bool
	EncryptionConfigured   bool
	EncryptionKeyCount     int
	CronEnabled            bool
	CronIntervalSeconds    int64
	CronRunOnStart         bool
	RestoreConfigured      bool
	RestoreAutoProvision   bool
	RestoreAdminConfigured bool
	DumpToolReady          bool
	RestoreToolReady       bool
	ReplicaConfigured      bool
	ReplicaProvider        string
	ReplicaBucket          string
	EncryptionKeyID        string
	RestoreDatabasePrefix  string
}

type Overview struct {
	Policy      Policy
	Runs        []BackupRun
	Drills      []RestoreDrill
	CleanupRuns []CleanupRun
	Config      ConfigStatus
}

type Store interface {
	BackupPolicy(ctx context.Context) (Policy, error)
	UpdateBackupPolicy(ctx context.Context, input PolicyUpdate) (Policy, error)
	BackupSourceMetadata(ctx context.Context) (SourceMetadata, error)
	StartBackupRun(ctx context.Context, input BackupStart) (BackupRun, error)
	CompleteBackupRun(ctx context.Context, input BackupCompletion) (BackupRun, error)
	RecordBackupVerification(ctx context.Context, input BackupVerification) (BackupRun, error)
	StartBackupReplica(ctx context.Context, input BackupReplicaStart) (BackupRun, error)
	CompleteBackupReplica(ctx context.Context, input BackupReplicaCompletion) (BackupRun, error)
	BackupRun(ctx context.Context, id int64) (BackupRun, error)
	BackupRuns(ctx context.Context, limit int) ([]BackupRun, error)
	StartRestoreDrill(ctx context.Context, input RestoreDrillStart) (RestoreDrill, error)
	CompleteRestoreDrill(ctx context.Context, input RestoreDrillCompletion) (RestoreDrill, error)
	RestoreDrills(ctx context.Context, limit int) ([]RestoreDrill, error)
	MarkBackupDeleted(ctx context.Context, runID int64, actor Actor) (BackupRun, error)
	ScheduleBackupCleanup(ctx context.Context, input CleanupSchedule) (CleanupRun, error)
	BackupCleanupRun(ctx context.Context, id int64) (CleanupRun, error)
	BackupCleanupRuns(ctx context.Context, limit int) ([]CleanupRun, error)
	BackupCleanupItems(ctx context.Context, cleanupRunID int64) ([]CleanupItem, error)
	ClaimBackupCleanup(ctx context.Context, cleanupRunID int64, retry bool, actor Actor) (CleanupRun, []CleanupItem, error)
	ClaimNextBackupCleanup(ctx context.Context, actor Actor) (CleanupRun, []CleanupItem, bool, error)
	CheckpointBackupCleanupItem(ctx context.Context, input CleanupCheckpoint) (CleanupItem, error)
	CompleteBackupCleanupItem(ctx context.Context, input CleanupItemCompletion) (CleanupItem, error)
	FinishBackupCleanup(ctx context.Context, input CleanupRunCompletion) (CleanupRun, error)
}
