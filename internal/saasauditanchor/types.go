package saasauditanchor

import (
	"context"
	"errors"
	"time"
)

const (
	SchemaVersion          = "mochat-go.saas-audit-anchor.v1"
	SignatureAlgorithm     = "hmac-sha256"
	ArtifactStatusPending  = "pending"
	ArtifactStatusExported = "exported"
	ArtifactStatusFailed   = "failed"
	RemoteStatusDisabled   = "disabled"
	RemoteStatusPending    = "pending"
	RemoteStatusExported   = "exported"
	RemoteStatusFailed     = "failed"
	VerifyStatusPending    = "pending"
	VerifyStatusPassed     = "passed"
	VerifyStatusFailed     = "failed"
	TriggerManual          = "admin_manual"
	TriggerCron            = "cron"
	TriggerMaintenance     = "maintenance"
	OperationActionCreate  = "saas.admin.audit.anchor.create"
	OperationActionVerify  = "saas.admin.audit.anchor.verify"
	OperationTarget        = "saas_admin_audit_anchor"
)

var ErrRemoteArtifactNotFound = errors.New("remote audit anchor artifact not found")

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

type Config struct {
	ArtifactRoot  string
	HMACKey       string
	HMACKeys      string
	HMACKeyID     string
	Remote        RemoteArtifactStore
	RequireRemote bool
}

type ConfigStatus struct {
	ArtifactRoot               string
	ArtifactRootConfigured     bool
	HMACConfigured             bool
	HMACKeyID                  string
	HMACKeyCount               int
	MissingKeyIDs              []string
	IndependentStorageRequired bool
	RemoteConfigured           bool
	RemoteRequired             bool
	RemoteProvider             string
	RemoteBucket               string
	RemotePrefix               string
	RemoteRetentionMode        string
	RemoteRetentionDays        int
}

type Actor struct {
	UserID   int
	TenantID int
}

type Chain struct {
	TenantID       int
	TenantName     string
	ChainVersion   int
	AnchorLogID    int64
	AnchorHash     string
	LegacyLogCount int64
	ChainHeadLogID int64
	ChainHeadHash  string
	SignedLogCount int64
	Status         string
}

type AuditVerification struct {
	ScannedChains int
	HealthyChains int
	FailedChains  int
	Chains        []Chain
}

type Checkpoint struct {
	ID                  int64
	CheckpointNo        string
	Fingerprint         string
	TenantID            int
	TenantName          string
	ChainVersion        int
	AnchorLogID         int64
	AnchorHash          string
	LegacyLogCount      int64
	ChainHeadLogID      int64
	ChainHeadHash       string
	SignedLogCount      int64
	SignatureAlgorithm  string
	KeyID               string
	PayloadSHA256       string
	Signature           string
	ArtifactName        string
	ArtifactSHA256      string
	ArtifactStatus      string
	ArtifactError       string
	RemoteStatus        string
	RemoteProvider      string
	RemoteBucket        string
	RemoteObjectKey     string
	RemoteETag          string
	RemoteVersionID     string
	RemoteSHA256        string
	RemoteSizeBytes     int64
	RemoteRetentionMode string
	RemoteRetainUntil   string
	RemoteError         string
	RemoteExportedAt    string
	RemoteVerifiedAt    string
	VerificationStatus  string
	VerificationError   string
	Source              string
	ActorUserID         int
	ActorTenantID       int
	SignedAt            string
	ExportedAt          string
	LastVerifiedAt      string
	CreatedAt           string
}

type CheckpointCreate struct {
	Checkpoint Checkpoint
}

type CheckpointSummary struct {
	CheckpointCount          int
	ExportedCount            int
	ArtifactFailedCount      int
	VerificationPassedCount  int
	VerificationFailedCount  int
	PendingVerificationCount int
	OrphanArtifactCount      int
	RemoteExportedCount      int
	RemoteFailedCount        int
	RemotePendingCount       int
	OrphanRemoteCount        int
	LatestSignedAt           string
	LatestVerifiedAt         string
	LatestRemoteVerifiedAt   string
}

type Overview struct {
	Config      ConfigStatus
	Summary     CheckpointSummary
	Checkpoints []Checkpoint
}

type CreateOptions struct {
	TenantID int
	Limit    int
	Source   string
	Actor    Actor
}

type CreateResult struct {
	ScannedChains           int
	CreatedCheckpoints      int
	ExistingCheckpoints     int
	BackfilledCheckpoints   int
	ExportedArtifacts       int
	FailedArtifacts         int
	RemoteExportedArtifacts int
	RemoteFailedArtifacts   int
	OperationID             int64
	CreatedAt               string
	Checkpoints             []Checkpoint
}

type VerifyOptions struct {
	TenantID int
	Limit    int
	Source   string
	Actor    Actor
}

type VerifyResult struct {
	ScannedCheckpoints   int
	PassedCheckpoints    int
	FailedCheckpoints    int
	MissingKeyCount      int
	MissingArtifactCount int
	OrphanArtifactCount  int
	MissingRemoteCount   int
	OrphanRemoteCount    int
	OperationID          int64
	VerifiedAt           string
	Checkpoints          []Checkpoint
}

type ArtifactUpdate struct {
	ID             int64
	Status         string
	ArtifactName   string
	ArtifactSHA256 string
	ErrorMessage   string
	ExportedAt     time.Time
}

type VerificationUpdate struct {
	ID           int64
	Status       string
	ErrorMessage string
	VerifiedAt   time.Time
}

type RemoteArtifact struct {
	Provider      string
	Bucket        string
	ObjectKey     string
	ETag          string
	VersionID     string
	SHA256        string
	SizeBytes     int64
	RetentionMode string
	RetainUntil   string
	ExportedAt    time.Time
	VerifiedAt    time.Time
}

type RemoteArtifactUpdate struct {
	ID           int64
	Status       string
	Artifact     RemoteArtifact
	ErrorMessage string
}

type RemoteArtifactStore interface {
	Provider() string
	Bucket() string
	Prefix() string
	RetentionMode() string
	RetentionDays() int
	Probe(context.Context) error
	Ensure(context.Context, string, []byte, string, time.Time, RemoteArtifact) (RemoteArtifact, error)
	Verify(context.Context, RemoteArtifact, []byte, string) (RemoteArtifact, error)
	ListObjectKeys(context.Context) ([]string, error)
}

type OperationRecord struct {
	Action   string
	TenantID int
	Source   string
	Affected int
	Failed   int
	Actor    Actor
}

type Store interface {
	VerifyAuditAnchorChains(context.Context, CreateOptions) (AuditVerification, error)
	AuditAnchorCheckpointSummary(context.Context, int) (CheckpointSummary, error)
	AuditAnchorCheckpoints(context.Context, int, int) ([]Checkpoint, error)
	AuditAnchorCheckpointsPendingRemote(context.Context, int, int) ([]Checkpoint, error)
	AuditAnchorCheckpointKeyIDs(context.Context) ([]string, error)
	AuditAnchorCheckpointNos(context.Context) ([]string, error)
	AuditAnchorRemoteObjectKeys(context.Context) ([]string, error)
	AuditAnchorCheckpointByFingerprint(context.Context, string) (Checkpoint, bool, error)
	CreateAuditAnchorCheckpoint(context.Context, CheckpointCreate) (Checkpoint, bool, error)
	UpdateAuditAnchorArtifact(context.Context, ArtifactUpdate) (Checkpoint, error)
	UpdateAuditAnchorRemoteArtifact(context.Context, RemoteArtifactUpdate) (Checkpoint, error)
	UpdateAuditAnchorVerification(context.Context, VerificationUpdate) (Checkpoint, error)
	CheckAuditAnchorCheckpointChain(context.Context, Checkpoint) error
	RecordAuditAnchorOperation(context.Context, OperationRecord) (int64, error)
}
