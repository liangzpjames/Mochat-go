package dashboardadmin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strings"

	"jiyi/mochat-go/internal/wecomcredentials"
)

const (
	PermissionIntegrationsRead     = "platform.integrations.read"
	PermissionIntegrationsManage   = "platform.integrations.manage"
	WeComVerificationLocalContract = "local_contract"
)

var (
	ErrWeComOnlineVerificationUnavailable = errors.New("WeCom online verification is unavailable")
	ErrWeComCorpMismatch                  = errors.New("verified WeCom corp does not match authoritative binding")
	ErrWeComMissingCapabilities           = errors.New("required WeCom capabilities are missing")
	ErrWeComCredentialDecrypt             = errors.New("WeCom integration credential cannot be decrypted")
	ErrWeComActiveMediaLease              = errors.New("active archive media lease blocks integration switch")
	ErrWeComCandidateNotVerified          = errors.New("WeCom integration candidate is not verified")
)

type WeComIntegration struct {
	ID                   string   `json:"id"`
	Mode                 string   `json:"mode"`
	Slot                 string   `json:"slot"`
	Status               string   `json:"status"`
	VerifiedWXCorpID     string   `json:"verifiedWxCorpId"`
	AgentID              string   `json:"agentId"`
	ProviderAppID        string   `json:"providerAppId"`
	CredentialConfigured bool     `json:"credentialConfigured"`
	CredentialHint       string   `json:"credentialHint"`
	Scope                []string `json:"scope"`
	ScopeDigest          string   `json:"scopeDigest"`
	MissingCapabilities  []string `json:"missingCapabilities"`
	Generation           uint64   `json:"generation"`
	Version              uint64   `json:"version"`
	VerificationLevel    string   `json:"verificationLevel"`
	VerifiedAt           string   `json:"verifiedAt"`
	LastErrorCode        string   `json:"lastErrorCode"`
	UpdatedAt            string   `json:"updatedAt"`
	CredentialCiphertext string   `json:"-"`
	CredentialKeyID      string   `json:"-"`
}

type WeComIntegrationView struct {
	TenantID  int               `json:"tenantId"`
	CorpID    int               `json:"corpId"`
	Current   *WeComIntegration `json:"current"`
	Candidate *WeComIntegration `json:"candidate"`
}

type WeComIntegrationCandidateInput struct {
	Mode           string   `json:"mode"`
	AgentID        string   `json:"agentId"`
	ProviderAppID  string   `json:"providerAppId"`
	EmployeeSecret string   `json:"employeeSecret"`
	ContactSecret  string   `json:"contactSecret"`
	AgentSecret    string   `json:"agentSecret"`
	ChatSecret     string   `json:"chatSecret"`
	PermanentCode  string   `json:"permanentCode"`
	Scope          []string `json:"scope"`
	Version        uint64   `json:"version"`
}

type WeComIntegrationVersionInput struct {
	Version uint64 `json:"version"`
}

type WeComVerificationCandidate struct {
	Integration           WeComIntegration
	TenantID              int
	CorpID                int
	AuthoritativeWXCorpID string
	Credentials           wecomcredentials.AuthorizationCredential
}

type WeComVerificationRequest struct {
	TenantID              int
	CorpID                int
	AuthoritativeWXCorpID string
	Mode                  string
	AgentID               string
	ProviderAppID         string
	Scope                 []string
	Credentials           wecomcredentials.AuthorizationCredential
}

type WeComVerificationResult struct {
	VerifiedWXCorpID    string   `json:"verifiedWxCorpId"`
	Scope               []string `json:"scope"`
	MissingCapabilities []string `json:"missingCapabilities"`
	VerificationLevel   string   `json:"verificationLevel"`
}

type WeComIntegrationAudit struct {
	ID          int64  `json:"id"`
	Action      string `json:"action"`
	TargetID    string `json:"targetId"`
	BeforeJSON  string `json:"before"`
	AfterJSON   string `json:"after"`
	ActorUserID int    `json:"actorUserId"`
	CreatedAt   string `json:"createdAt"`
}

type WeComIntegrationVerifier interface {
	Verify(context.Context, WeComVerificationRequest) (WeComVerificationResult, error)
}
type WeComIntegrationVerifierFunc func(context.Context, WeComVerificationRequest) (WeComVerificationResult, error)

func (f WeComIntegrationVerifierFunc) Verify(ctx context.Context, request WeComVerificationRequest) (WeComVerificationResult, error) {
	return f(ctx, request)
}

type WeComIntegrationStore interface {
	WeComIntegration(context.Context, Actor, int) (WeComIntegrationView, error)
	SaveWeComIntegrationCandidate(context.Context, Actor, int, WeComIntegrationCandidateInput) (WeComIntegration, error)
	WeComIntegrationVerificationCandidate(context.Context, Actor, int, uint64) (WeComVerificationCandidate, error)
	CompleteWeComIntegrationVerification(context.Context, Actor, int, uint64, WeComVerificationResult, string) (WeComIntegration, error)
	SwitchWeComIntegration(context.Context, Actor, int, uint64) (WeComIntegrationView, error)
	RollbackWeComIntegration(context.Context, Actor, int, uint64) (WeComIntegrationView, error)
	WeComIntegrationAudits(context.Context, Actor, int) ([]WeComIntegrationAudit, error)
}

type WeComIntegrationService struct {
	store    WeComIntegrationStore
	verifier WeComIntegrationVerifier
}

func NewWeComIntegrationService(store WeComIntegrationStore, verifier WeComIntegrationVerifier) *WeComIntegrationService {
	return &WeComIntegrationService{store: store, verifier: verifier}
}
func (s *WeComIntegrationService) Get(ctx context.Context, actor Actor, tenantID int) (WeComIntegrationView, error) {
	if s == nil || s.store == nil || tenantID <= 0 {
		return WeComIntegrationView{}, ErrInvalidRequest
	}
	return s.store.WeComIntegration(ctx, actor, tenantID)
}
func (s *WeComIntegrationService) SaveCandidate(ctx context.Context, actor Actor, tenantID int, input WeComIntegrationCandidateInput) (WeComIntegration, error) {
	if s == nil || s.store == nil || tenantID <= 0 {
		return WeComIntegration{}, ErrInvalidRequest
	}
	input = normalizeWeComCandidateInput(input)
	if err := ValidateWeComIntegrationCandidate(input, true); err != nil && !(errors.Is(err, ErrInvalidRequest) && weComCandidateMayRetain(input)) {
		return WeComIntegration{}, err
	}
	return s.store.SaveWeComIntegrationCandidate(ctx, actor, tenantID, input)
}
func (s *WeComIntegrationService) VerifyCandidate(ctx context.Context, actor Actor, tenantID int, version uint64) (WeComIntegration, error) {
	if s == nil || s.store == nil || tenantID <= 0 || version == 0 {
		return WeComIntegration{}, ErrInvalidRequest
	}
	candidate, err := s.store.WeComIntegrationVerificationCandidate(ctx, actor, tenantID, version)
	if err != nil {
		return WeComIntegration{}, err
	}
	fail := func(code string, cause error) (WeComIntegration, error) {
		_, recordErr := s.store.CompleteWeComIntegrationVerification(ctx, actor, tenantID, version, WeComVerificationResult{}, code)
		if recordErr != nil {
			return WeComIntegration{}, recordErr
		}
		return WeComIntegration{}, cause
	}
	if s.verifier == nil {
		return fail("WECOM_ONLINE_VERIFICATION_UNAVAILABLE", ErrWeComOnlineVerificationUnavailable)
	}
	result, err := s.verifier.Verify(ctx, WeComVerificationRequest{TenantID: candidate.TenantID, CorpID: candidate.CorpID, AuthoritativeWXCorpID: candidate.AuthoritativeWXCorpID, Mode: candidate.Integration.Mode, AgentID: candidate.Integration.AgentID, ProviderAppID: candidate.Integration.ProviderAppID, Scope: append([]string{}, candidate.Integration.Scope...), Credentials: candidate.Credentials})
	if err != nil {
		return fail("WECOM_CONTRACT_VERIFICATION_FAILED", ErrWeComOnlineVerificationUnavailable)
	}
	result.VerifiedWXCorpID = strings.TrimSpace(result.VerifiedWXCorpID)
	result.Scope = normalizeWeComStrings(result.Scope)
	result.MissingCapabilities = normalizeWeComStrings(result.MissingCapabilities)
	result.VerificationLevel = strings.TrimSpace(result.VerificationLevel)
	if result.VerificationLevel != WeComVerificationLocalContract {
		return fail("WECOM_VERIFICATION_LEVEL_INVALID", ErrInvalidRequest)
	}
	if result.VerifiedWXCorpID == "" || result.VerifiedWXCorpID != candidate.AuthoritativeWXCorpID {
		return fail("WECOM_CORP_MISMATCH", ErrWeComCorpMismatch)
	}
	if len(result.MissingCapabilities) > 0 {
		return fail("WECOM_MISSING_CAPABILITIES", ErrWeComMissingCapabilities)
	}
	return s.store.CompleteWeComIntegrationVerification(ctx, actor, tenantID, version, result, "")
}
func (s *WeComIntegrationService) Switch(ctx context.Context, actor Actor, tenantID int, version uint64) (WeComIntegrationView, error) {
	if s == nil || s.store == nil || tenantID <= 0 || version == 0 {
		return WeComIntegrationView{}, ErrInvalidRequest
	}
	return s.store.SwitchWeComIntegration(ctx, actor, tenantID, version)
}
func (s *WeComIntegrationService) Rollback(ctx context.Context, actor Actor, tenantID int, version uint64) (WeComIntegrationView, error) {
	if s == nil || s.store == nil || tenantID <= 0 || version == 0 {
		return WeComIntegrationView{}, ErrInvalidRequest
	}
	return s.store.RollbackWeComIntegration(ctx, actor, tenantID, version)
}
func (s *WeComIntegrationService) Audits(ctx context.Context, actor Actor, tenantID int) ([]WeComIntegrationAudit, error) {
	if s == nil || s.store == nil || tenantID <= 0 {
		return nil, ErrInvalidRequest
	}
	return s.store.WeComIntegrationAudits(ctx, actor, tenantID)
}

func ValidateWeComIntegrationCandidate(input WeComIntegrationCandidateInput, credentialExists bool) error {
	input = normalizeWeComCandidateInput(input)
	self := input.EmployeeSecret != "" || input.ContactSecret != "" || input.AgentSecret != "" || input.ChatSecret != ""
	switch input.Mode {
	case "self_built":
		if input.ProviderAppID != "" || input.PermanentCode != "" || (!self && !credentialExists) {
			return ErrInvalidRequest
		}
	case "third_party_delegated":
		if input.ProviderAppID == "" || self || (!credentialExists && input.PermanentCode == "") {
			return ErrInvalidRequest
		}
	default:
		return ErrInvalidRequest
	}
	return nil
}
func weComCandidateMayRetain(input WeComIntegrationCandidateInput) bool {
	return (input.Mode == "self_built" && !hasWeComSelfSecret(input)) || (input.Mode == "third_party_delegated" && input.ProviderAppID != "" && input.PermanentCode == "" && !hasWeComSelfSecret(input))
}
func hasWeComSelfSecret(input WeComIntegrationCandidateInput) bool {
	return input.EmployeeSecret != "" || input.ContactSecret != "" || input.AgentSecret != "" || input.ChatSecret != ""
}
func normalizeWeComCandidateInput(input WeComIntegrationCandidateInput) WeComIntegrationCandidateInput {
	input.Mode = strings.TrimSpace(input.Mode)
	input.AgentID = strings.TrimSpace(input.AgentID)
	input.ProviderAppID = strings.TrimSpace(input.ProviderAppID)
	input.EmployeeSecret = strings.TrimSpace(input.EmployeeSecret)
	input.ContactSecret = strings.TrimSpace(input.ContactSecret)
	input.AgentSecret = strings.TrimSpace(input.AgentSecret)
	input.ChatSecret = strings.TrimSpace(input.ChatSecret)
	input.PermanentCode = strings.TrimSpace(input.PermanentCode)
	input.Scope = normalizeWeComStrings(input.Scope)
	return input
}
func normalizeWeComStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
func WeComScopeDigest(scope []string) string {
	sum := sha256.Sum256([]byte(strings.Join(normalizeWeComStrings(scope), "\n")))
	return hex.EncodeToString(sum[:])
}
