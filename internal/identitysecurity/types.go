package identitysecurity

import (
	"context"
	"errors"
	"net/http"
	"time"
)

const (
	PolicyStatusActive   = "active"
	PolicyStatusDisabled = "disabled"

	MFAStatusPending  = "pending"
	MFAStatusActive   = "active"
	MFAStatusDisabled = "disabled"

	ChallengeStatusPending  = "pending"
	ChallengeStatusConsumed = "consumed"
	ChallengeStatusExpired  = "expired"
	ChallengeStatusLocked   = "locked"

	SessionStatusActive  = "active"
	SessionStatusRevoked = "revoked"
	SessionStatusExpired = "expired"

	IncidentStatusOpen         = "open"
	IncidentStatusAcknowledged = "acknowledged"
	IncidentStatusResolved     = "resolved"

	EventPasswordFailed   = "password_failed"
	EventPasswordVerified = "password_verified"
	EventLoginBlocked     = "login_blocked"
	EventMFARequired      = "mfa_required"
	EventMFAFailed        = "mfa_failed"
	EventLoginSucceeded   = "login_succeeded"
	EventSessionRevoked   = "session_revoked"
	EventMFABegan         = "mfa_enrollment_began"
	EventMFAEnabled       = "mfa_enabled"
	EventMFAReset         = "mfa_reset"

	AuthMethodPassword = "password"
	AuthMethodTOTP     = "password+totp"
	AuthMethodRecovery = "password+recovery"

	RiskNormal   = "normal"
	RiskWarning  = "warning"
	RiskCritical = "critical"

	OperationPolicyUpdate   = "saas.admin.identity.policy.update"
	OperationSessionRevoke  = "saas.admin.identity.session.revoke"
	OperationUserUnlock     = "saas.admin.identity.user.unlock"
	OperationIncidentUpdate = "saas.admin.identity.incident.update"
	OperationMFABegin       = "saas.admin.identity.mfa.begin"
	OperationMFAEnable      = "saas.admin.identity.mfa.enable"
	OperationMFAReset       = "saas.admin.identity.mfa.reset"
)

var (
	ErrSessionNotFound = errors.New("identity session not found")
	ErrSessionRevoked  = errors.New("identity session revoked")
	ErrSessionExpired  = errors.New("identity session expired")
)

type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string { return e.Message }

func Invalid(message string) error { return &Error{Status: http.StatusBadRequest, Message: message} }
func Unauthorized(message string) error {
	return &Error{Status: http.StatusUnauthorized, Message: message}
}
func Forbidden(message string) error { return &Error{Status: http.StatusForbidden, Message: message} }
func NotFound(message string) error  { return &Error{Status: http.StatusNotFound, Message: message} }
func Conflict(message string) error  { return &Error{Status: http.StatusConflict, Message: message} }
func Unavailable(message string) error {
	return &Error{Status: http.StatusServiceUnavailable, Message: message}
}

func StatusCode(err error) int {
	var typed *Error
	if errors.As(err, &typed) {
		return typed.Status
	}
	return http.StatusInternalServerError
}

type Actor struct {
	UserID   int
	TenantID int
}

type Policy struct {
	TenantID                int      `json:"tenantId"`
	TenantName              string   `json:"tenantName"`
	Status                  string   `json:"status"`
	MaxFailedAttempts       int      `json:"maxFailedAttempts"`
	LockoutMinutes          int      `json:"lockoutMinutes"`
	SessionTTLMinutes       int      `json:"sessionTtlMinutes"`
	IdleTimeoutMinutes      int      `json:"idleTimeoutMinutes"`
	MaxConcurrentSessions   int      `json:"maxConcurrentSessions"`
	RequireMFA              bool     `json:"requireMfa"`
	AllowedIPCIDRs          []string `json:"allowedIpCidrs"`
	LoginEventRetentionDays int      `json:"loginEventRetentionDays"`
	SessionRetentionDays    int      `json:"sessionRetentionDays"`
	ActiveUserCount         int      `json:"activeUserCount"`
	MFAEnrolledUserCount    int      `json:"mfaEnrolledUserCount"`
	MFAMissingUserCount     int      `json:"mfaMissingUserCount"`
	Version                 int      `json:"version"`
	UpdatedBy               int      `json:"updatedBy"`
	CreatedAt               string   `json:"createdAt"`
	UpdatedAt               string   `json:"updatedAt"`
}

type PolicyUpdate struct {
	TenantID                 int      `json:"tenantId"`
	Status                   string   `json:"status"`
	MaxFailedAttempts        int      `json:"maxFailedAttempts"`
	LockoutMinutes           int      `json:"lockoutMinutes"`
	SessionTTLMinutes        int      `json:"sessionTtlMinutes"`
	IdleTimeoutMinutes       int      `json:"idleTimeoutMinutes"`
	MaxConcurrentSessions    int      `json:"maxConcurrentSessions"`
	RequireMFA               bool     `json:"requireMfa"`
	AllowedIPCIDRs           []string `json:"allowedIpCidrs"`
	LoginEventRetentionDays  int      `json:"loginEventRetentionDays"`
	SessionRetentionDays     int      `json:"sessionRetentionDays"`
	ExpectedVersion          int      `json:"expectedVersion"`
	Actor                    Actor    `json:"-"`
	ApprovalExecutionID      int64    `json:"-"`
	ApprovalExecutionVersion int      `json:"-"`
}

type UserState struct {
	UserID           int       `json:"userId"`
	TenantID         int       `json:"tenantId"`
	UserName         string    `json:"userName"`
	Phone            string    `json:"phone"`
	UserStatus       int       `json:"userStatus"`
	FailedAttempts   int       `json:"failedAttempts"`
	LockedUntil      string    `json:"lockedUntil"`
	LastFailedAt     string    `json:"lastFailedAt"`
	LastFailedIP     string    `json:"lastFailedIp"`
	LastSuccessAt    string    `json:"lastSuccessAt"`
	LastSuccessIP    string    `json:"lastSuccessIp"`
	MFAStatus        string    `json:"mfaStatus"`
	MFARecoveryCodes int       `json:"mfaRecoveryCodes"`
	MFAVersion       int       `json:"mfaVersion"`
	ActiveSessions   int       `json:"activeSessions"`
	Version          int       `json:"version"`
	UpdatedAt        string    `json:"updatedAt"`
	LockedUntilValue time.Time `json:"-"`
}

type Principal struct {
	UserID   int
	TenantID int
	Phone    string
	Name     string
}

type RequestMeta struct {
	IP        string
	UserAgent string
}

type MFACredential struct {
	ID                     int64
	UserID                 int
	TenantID               int
	Status                 string
	SecretCiphertext       string
	EncryptionKeyID        string
	RecoveryCodeHashesJSON string
	RecoveryCodesRemaining int
	VerifiedAt             string
	LastUsedAt             string
	LastTOTPStep           int64
	DisabledAt             string
	DisabledBy             int
	DisabledReason         string
	Version                int
	CreatedAt              string
	UpdatedAt              string
}

type MFAEnrollment struct {
	UserID          int      `json:"userId"`
	TenantID        int      `json:"tenantId"`
	Secret          string   `json:"secret"`
	ProvisioningURI string   `json:"provisioningUri"`
	RecoveryCodes   []string `json:"recoveryCodes"`
	KeyID           string   `json:"keyId"`
	Version         int      `json:"version"`
}

type MFABegin struct {
	Principal Principal
	Actor     Actor
}

type MFAVerify struct {
	Principal       Principal
	Code            string
	ExpectedVersion int
	Actor           Actor
}

type MFAReset struct {
	UserID                   int    `json:"userId"`
	TenantID                 int    `json:"tenantId"`
	ExpectedVersion          int    `json:"expectedVersion"`
	Reason                   string `json:"reason"`
	Actor                    Actor  `json:"-"`
	ApprovalExecutionID      int64  `json:"-"`
	ApprovalExecutionVersion int    `json:"-"`
}

type MFAResetResult struct {
	Credential      MFACredential
	RevokedSessions int
	OperationID     int64
}

type Challenge struct {
	ID          int64
	UserID      int
	TenantID    int
	Status      string
	Attempts    int
	MaxAttempts int
	IP          string
	UserAgent   string
	ExpiresAt   time.Time
	ConsumedAt  string
	Credential  MFACredential
}

type AuthDecision struct {
	Policy          Policy
	MFARequired     bool
	ChallengeToken  string
	ChallengeExpiry time.Time
	AuthMethod      string
	NewIP           bool
}

type MFACompletion struct {
	Principal  Principal
	Policy     Policy
	AuthMethod string
}

type Session struct {
	ID               int64  `json:"id"`
	SessionJTI       string `json:"sessionJti"`
	UserID           int    `json:"userId"`
	UserName         string `json:"userName"`
	Phone            string `json:"phone"`
	TenantID         int    `json:"tenantId"`
	TenantName       string `json:"tenantName"`
	Status           string `json:"status"`
	AuthMethod       string `json:"authMethod"`
	IP               string `json:"ipAddress"`
	UserAgent        string `json:"userAgent"`
	IssuedAt         string `json:"issuedAt"`
	ExpiresAt        string `json:"expiresAt"`
	IdleExpiresAt    string `json:"idleExpiresAt"`
	LastSeenAt       string `json:"lastSeenAt"`
	RevokedAt        string `json:"revokedAt"`
	RevokedBy        int    `json:"revokedBy"`
	RevocationReason string `json:"revocationReason"`
	Version          int    `json:"version"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
}

type SessionCreate struct {
	Principal             Principal
	SessionJTI            string
	TokenSHA256           string
	AuthMethod            string
	Meta                  RequestMeta
	IssuedAt              time.Time
	ExpiresAt             time.Time
	IdleExpiresAt         time.Time
	MaxConcurrentSessions int
}

type SessionClaims struct {
	JTI       string
	UserID    int
	IssuedAt  time.Time
	ExpiresAt time.Time
}

type SessionOptions struct {
	TenantID int
	UserID   int
	Status   string
	Keyword  string
	Limit    int
}

type SessionRevoke struct {
	SessionID       int64  `json:"sessionId"`
	UserID          int    `json:"userId"`
	ExpectedVersion int    `json:"expectedVersion"`
	Reason          string `json:"reason"`
	Actor           Actor  `json:"-"`
}

type LoginEvent struct {
	ID         int64          `json:"id"`
	UserID     int            `json:"userId"`
	UserName   string         `json:"userName"`
	TenantID   int            `json:"tenantId"`
	TenantName string         `json:"tenantName"`
	EventType  string         `json:"eventType"`
	Result     string         `json:"result"`
	RiskLevel  string         `json:"riskLevel"`
	ReasonCode string         `json:"reasonCode"`
	IP         string         `json:"ipAddress"`
	UserAgent  string         `json:"userAgent"`
	Metadata   map[string]any `json:"metadata"`
	OccurredAt string         `json:"occurredAt"`
}

type EventOptions struct {
	TenantID int
	UserID   int
	Risk     string
	Result   string
	Keyword  string
	Limit    int
}

type LoginFailure struct {
	Principal Principal
	PhoneHash string
	Meta      RequestMeta
	Policy    Policy
	Reason    string
	Now       time.Time
}

type LoginSuccess struct {
	Principal Principal
	PhoneHash string
	Meta      RequestMeta
	EventType string
	Risk      string
	Reason    string
	Now       time.Time
}

type Incident struct {
	ID              int64  `json:"id"`
	IncidentNo      string `json:"incidentNo"`
	TenantID        int    `json:"tenantId"`
	TenantName      string `json:"tenantName"`
	UserID          int    `json:"userId"`
	UserName        string `json:"userName"`
	IncidentType    string `json:"incidentType"`
	Severity        string `json:"severity"`
	Status          string `json:"status"`
	Title           string `json:"title"`
	LatestDetail    string `json:"latestDetail"`
	OccurrenceCount int    `json:"occurrenceCount"`
	FirstOccurredAt string `json:"firstOccurredAt"`
	LastOccurredAt  string `json:"lastOccurredAt"`
	AssignedTo      string `json:"assignedTo"`
	AcknowledgedAt  string `json:"acknowledgedAt"`
	AcknowledgedBy  int    `json:"acknowledgedBy"`
	ResolvedAt      string `json:"resolvedAt"`
	ResolvedBy      int    `json:"resolvedBy"`
	Resolution      string `json:"resolution"`
	Version         int    `json:"version"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}

type IncidentOptions struct {
	TenantID int
	UserID   int
	Status   string
	Severity string
	Keyword  string
	Limit    int
}

type IncidentUpdate struct {
	ID              int64  `json:"id"`
	Action          string `json:"action"`
	AssignedTo      string `json:"assignedTo"`
	Resolution      string `json:"resolution"`
	ExpectedVersion int    `json:"expectedVersion"`
	Actor           Actor  `json:"-"`
}

type Summary struct {
	ActiveSessions      int `json:"activeSessions"`
	LockedUsers         int `json:"lockedUsers"`
	MFAEnrolledUsers    int `json:"mfaEnrolledUsers"`
	ActiveUsers         int `json:"activeUsers"`
	OpenIncidents       int `json:"openIncidents"`
	CriticalIncidents   int `json:"criticalIncidents"`
	FailedLogins24h     int `json:"failedLogins24h"`
	SuccessfulLogins24h int `json:"successfulLogins24h"`
}

type ConfigStatus struct {
	Enabled               bool     `json:"enabled"`
	SessionEnforced       bool     `json:"sessionEnforced"`
	MFAEncryptionReady    bool     `json:"mfaEncryptionReady"`
	MFAEncryptionKeyID    string   `json:"mfaEncryptionKeyId"`
	MFAEncryptionKeys     int      `json:"mfaEncryptionKeyCount"`
	TrustedProxyHeaders   bool     `json:"trustedProxyHeaders"`
	TrustedProxyCIDRs     []string `json:"trustedProxyCidrs"`
	TrustedProxyCIDRCount int      `json:"trustedProxyCidrCount"`
}

type Overview struct {
	Policy    Policy       `json:"policy"`
	Summary   Summary      `json:"summary"`
	Users     []UserState  `json:"users"`
	Sessions  []Session    `json:"sessions"`
	Events    []LoginEvent `json:"events"`
	Incidents []Incident   `json:"incidents"`
	Config    ConfigStatus `json:"config"`
}

type CleanupResult struct {
	ExpiredChallenges int `json:"expiredChallenges"`
	ExpiredSessions   int `json:"expiredSessions"`
	DeletedChallenges int `json:"deletedChallenges"`
	DeletedSessions   int `json:"deletedSessions"`
	DeletedEvents     int `json:"deletedEvents"`
}

type Store interface {
	IdentityPolicy(ctx context.Context, tenantID int) (Policy, bool, error)
	UpdateIdentityPolicy(ctx context.Context, input PolicyUpdate) (Policy, error)
	IdentityMFAEnrollmentCoverage(ctx context.Context, tenantID int) (activeUsers, enrolledUsers int, err error)
	IdentityUserState(ctx context.Context, userID int) (UserState, bool, error)
	RecordIdentityLoginFailure(ctx context.Context, input LoginFailure) (UserState, error)
	RecordIdentityLoginEvent(ctx context.Context, event LoginEvent, phoneHash string, now time.Time) error
	RecordIdentityLoginSuccess(ctx context.Context, input LoginSuccess) (newIP bool, err error)
	IdentityMFACredential(ctx context.Context, userID int) (MFACredential, bool, error)
	SavePendingIdentityMFA(ctx context.Context, credential MFACredential, actor Actor) (MFACredential, error)
	ActivateIdentityMFA(ctx context.Context, userID, expectedVersion int, recoveryHashesJSON string, recoveryCount int, totpStep int64, actor Actor, now time.Time) (MFACredential, error)
	UseIdentityMFA(ctx context.Context, userID, expectedVersion int, recoveryHashesJSON string, recoveryCount int, totpStep int64, now time.Time) (MFACredential, error)
	ResetIdentityMFA(ctx context.Context, input MFAReset, now time.Time) (MFAResetResult, error)
	CreateIdentityChallenge(ctx context.Context, challengeHash string, principal Principal, meta RequestMeta, expiresAt time.Time) (Challenge, error)
	IdentityChallenge(ctx context.Context, challengeHash string) (Challenge, error)
	FailIdentityChallenge(ctx context.Context, challengeID int64, reason string, now time.Time) (Challenge, error)
	ConsumeIdentityChallenge(ctx context.Context, challengeID int64, now time.Time) (Challenge, error)
	CreateIdentitySession(ctx context.Context, input SessionCreate) (Session, int, error)
	ValidateIdentitySession(ctx context.Context, claims SessionClaims, now time.Time) error
	IdentityOverview(ctx context.Context, tenantID, limit int) (Summary, []UserState, []Session, []LoginEvent, []Incident, error)
	IdentitySessions(ctx context.Context, options SessionOptions) ([]Session, error)
	RevokeIdentitySession(ctx context.Context, input SessionRevoke, now time.Time) (Session, error)
	RevokeCurrentIdentitySession(ctx context.Context, jti string, userID int, reason string, now time.Time) error
	UnlockIdentityUser(ctx context.Context, userID, expectedVersion int, reason string, actor Actor, now time.Time) (UserState, error)
	IdentityLoginEvents(ctx context.Context, options EventOptions) ([]LoginEvent, error)
	IdentityIncidents(ctx context.Context, options IncidentOptions) ([]Incident, error)
	UpdateIdentityIncident(ctx context.Context, input IncidentUpdate, now time.Time) (Incident, error)
	CleanupIdentitySecurity(ctx context.Context, now time.Time, limit int) (CleanupResult, error)
}
