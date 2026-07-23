package dashboard

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	SaaSReleaseEvidenceStatusMissing           = "missing"
	SaaSReleaseEvidenceStatusProgress          = "in_progress"
	SaaSReleaseEvidenceStatusPassed            = "passed"
	SaaSReleaseEvidenceStatusFailed            = "failed"
	SaaSReleaseCandidateStatusBlocked          = "blocked"
	SaaSReleaseCandidateStatusReady            = "ready"
	SaaSReleaseCandidateStatusStale            = "stale"
	SaaSReleaseEvidenceRequiredCount           = 6
	SaaSAdminOperationActionEvidenceSave       = "saas.admin.release.evidence.save"
	SaaSAdminOperationActionEvidenceActionSave = "saas.admin.release.evidence.action.save"
	SaaSAdminOperationActionCandidateGate      = "saas.admin.release.candidate.gate"
	SaaSAdminOperationTargetEvidence           = "saas_release_evidence"
	SaaSAdminOperationTargetEvidenceAction     = "saas_release_evidence_action"
	SaaSAdminOperationTargetCandidate          = "saas_release_candidate"
	SaaSReleaseEvidenceActionStateUnassigned   = "unassigned"
	SaaSReleaseEvidenceActionStateProgress     = "in_progress"
	SaaSReleaseEvidenceActionStateBlocked      = "blocked"
	SaaSReleaseEvidenceActionStateResolved     = "resolved"
	SaaSReleaseEvidenceActionDueNoDate         = "no_date"
	SaaSReleaseEvidenceActionDueScheduled      = "scheduled"
	SaaSReleaseEvidenceActionDueSoon           = "due_soon"
	SaaSReleaseEvidenceActionDueOverdue        = "overdue"
	SaaSReleaseEvidenceActionDueResolved       = "resolved"
	SaaSReleaseEvidenceActionDueSoonHours      = 72
)

var (
	saasReleaseFingerprintPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
	saasReleaseVersionPattern     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	saasReleaseSecretPattern      = regexp.MustCompile(`(?i)(authorization\s*:|bearer\s+|access[_-]?token\s*[=:]|refresh[_-]?token\s*[=:]|password\s*[=:]|passwd\s*[=:]|secret\s*[=:]|encodingaeskey\s*[=:])`)
)

type SaaSReleaseEvidence struct {
	ID                int64
	Key               string
	Title             string
	Category          string
	Required          bool
	Status            string
	EvidenceURL       string
	Environment       string
	SourceFingerprint string
	ArtifactSHA256    string
	ArtifactSizeBytes int64
	CheckedAt         string
	CheckedBy         int
	Note              string
	Version           int
	CreatedAt         string
	UpdatedAt         string
}

type SaaSReleaseEvidenceUpdate struct {
	Key                  string                           `json:"key"`
	Status               string                           `json:"status"`
	EvidenceURL          string                           `json:"evidenceUrl"`
	Environment          string                           `json:"environment"`
	SourceFingerprint    string                           `json:"sourceFingerprint"`
	ArtifactSHA256       string                           `json:"artifactSha256"`
	ArtifactSizeBytes    int64                            `json:"artifactSizeBytes"`
	Note                 string                           `json:"note"`
	ExpectedVersion      int                              `json:"expectedVersion"`
	ActorUserID          int                              `json:"-"`
	ActorTenantID        int                              `json:"-"`
	ArtifactVerification *SaaSReleaseArtifactVerification `json:"-"`
}

type SaaSReleaseEvidenceUpdateResult struct {
	Evidence             SaaSReleaseEvidence
	ArtifactVerification *SaaSReleaseArtifactVerification
	OperationID          int64
}

type SaaSReleaseEvidenceAction struct {
	ID          int64
	Key         string
	Title       string
	OwnerUserID int
	OwnerName   string
	OwnerPhone  string
	OwnerActive bool
	DueAt       string
	NextAction  string
	Note        string
	Version     int
	CreatedBy   int
	UpdatedBy   int
	CreatedAt   string
	UpdatedAt   string
}

type SaaSReleaseEvidenceActionUpdate struct {
	Key             string `json:"key"`
	OwnerUserID     int    `json:"ownerUserId"`
	DueAt           string `json:"dueAt"`
	NextAction      string `json:"nextAction"`
	Note            string `json:"note"`
	ExpectedVersion int    `json:"expectedVersion"`
	ActorUserID     int    `json:"-"`
	ActorTenantID   int    `json:"-"`
}

type SaaSReleaseEvidenceActionUpdateResult struct {
	Action      SaaSReleaseEvidenceAction
	OperationID int64
}

type SaaSReleaseCandidate struct {
	ID                int64
	CandidateNo       string
	ReleaseVersion    string
	SourceFingerprint string
	Status            string
	RequiredCount     int
	PassedCount       int
	MatchedCount      int
	SnapshotJSON      string
	GateMessage       string
	CreatedBy         int
	OperationID       int64
	CreatedAt         string
}

type SaaSReleaseCandidateCreate struct {
	CandidateNo              string
	ReleaseVersion           string                                     `json:"releaseVersion"`
	SourceFingerprint        string                                     `json:"sourceFingerprint"`
	ActorUserID              int                                        `json:"-"`
	ActorTenantID            int                                        `json:"-"`
	ApprovalExecutionID      int64                                      `json:"-"`
	ApprovalExecutionVersion int                                        `json:"-"`
	ArtifactVerifications    map[string]SaaSReleaseArtifactVerification `json:"-"`
}

type SaaSReleaseCandidateApprovalPayload struct {
	ReleaseVersion    string `json:"releaseVersion"`
	SourceFingerprint string `json:"sourceFingerprint"`
}

type SaaSReleaseCandidateCreateResult struct {
	Candidate   SaaSReleaseCandidate
	OperationID int64
}

type saasReleaseCandidateEvidenceSnapshot struct {
	ID                   int64                            `json:"id"`
	Key                  string                           `json:"key"`
	Status               string                           `json:"status"`
	EvidenceURL          string                           `json:"evidenceUrl"`
	Environment          string                           `json:"environment"`
	SourceFingerprint    string                           `json:"sourceFingerprint"`
	ArtifactSHA256       string                           `json:"artifactSha256"`
	ArtifactSizeBytes    int64                            `json:"artifactSizeBytes"`
	CheckedAt            string                           `json:"checkedAt"`
	CheckedBy            int                              `json:"checkedBy"`
	Note                 string                           `json:"note"`
	Version              int                              `json:"version"`
	ArtifactVerification *SaaSReleaseArtifactVerification `json:"artifactVerification"`
}

type saasReleaseCandidateEvaluation struct {
	SnapshotValid       bool
	EvidenceCurrent     bool
	EffectiveStatus     string
	DriftedEvidenceKeys []string
}

type SaaSReleaseReadinessStore interface {
	SaaSReleaseEvidence(ctx context.Context) ([]SaaSReleaseEvidence, error)
	UpdateSaaSReleaseEvidence(ctx context.Context, input SaaSReleaseEvidenceUpdate) (SaaSReleaseEvidenceUpdateResult, error)
	SaaSReleaseCandidates(ctx context.Context, limit int) ([]SaaSReleaseCandidate, error)
	CreateSaaSReleaseCandidate(ctx context.Context, input SaaSReleaseCandidateCreate) (SaaSReleaseCandidateCreateResult, error)
}

type SaaSReleaseEvidenceActionStore interface {
	SaaSReleaseEvidenceActions(ctx context.Context, platformTenantID int) ([]SaaSReleaseEvidenceAction, error)
	UpdateSaaSReleaseEvidenceAction(ctx context.Context, input SaaSReleaseEvidenceActionUpdate) (SaaSReleaseEvidenceActionUpdateResult, error)
}

func (h *SaaSAdminHandler) ReleaseReadiness(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	store, ok := h.saasReleaseReadinessStore(w)
	if !ok {
		return
	}
	requestedFingerprint := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sourceFingerprint")))
	if requestedFingerprint != "" && !saasReleaseFingerprintPattern.MatchString(requestedFingerprint) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "sourceFingerprint 必须是 64 位 SHA-256", nil)
		return
	}
	fingerprint := requestedFingerprint
	runtimeFingerprint, fingerprintSource, authoritative := h.authoritativeReleaseSourceFingerprint()
	if authoritative {
		if requestedFingerprint != "" && requestedFingerprint != runtimeFingerprint {
			writeEnvelope(w, http.StatusConflict, http.StatusConflict, "sourceFingerprint 与当前运行版本不一致", nil)
			return
		}
		fingerprint = runtimeFingerprint
	}
	items, err := store.SaaSReleaseEvidence(r.Context())
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	candidates, err := store.SaaSReleaseCandidates(r.Context(), 20)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	actionStore, ok := h.saasReleaseEvidenceActionStore(w)
	if !ok {
		return
	}
	actions, err := actionStore.SaaSReleaseEvidenceActions(r.Context(), user.TenantID)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	owners, err := h.saasReleaseEvidenceActionOwners(r.Context(), user.TenantID)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	now := time.Now()
	writeEnvelope(w, http.StatusOK, http.StatusOK, "ok", map[string]any{
		"summary":       saasReleaseReadinessSummaryPayload(items, candidates, fingerprint, authoritative, fingerprintSource, h.releaseEvidenceVerifierStatus()),
		"evidence":      saasReleaseEvidenceItemsPayload(items),
		"actionSummary": saasReleaseEvidenceActionSummaryPayload(actions, items, now),
		"actions":       saasReleaseEvidenceActionsPayload(actions, items, now),
		"owners":        owners,
		"candidates":    saasReleaseCandidatesPayload(candidates, items),
	})
}

func (h *SaaSAdminHandler) ReleaseEvidence(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	store, ok := h.saasReleaseReadinessStore(w)
	if !ok {
		return
	}
	var input SaaSReleaseEvidenceUpdate
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid JSON body", nil)
		return
	}
	if err := normalizeSaaSReleaseEvidenceUpdate(&input); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	runtimeFingerprint, _, authoritative := h.authoritativeReleaseSourceFingerprint()
	if input.Status == SaaSReleaseEvidenceStatusPassed && !authoritative {
		writeEnvelope(w, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "当前运行版本未内置源码指纹，不能将证据标记为通过", nil)
		return
	}
	if authoritative && input.SourceFingerprint != "" && input.SourceFingerprint != runtimeFingerprint {
		writeEnvelope(w, http.StatusConflict, http.StatusConflict, "sourceFingerprint 与当前运行版本不一致", nil)
		return
	}
	if input.Status == SaaSReleaseEvidenceStatusPassed {
		if h.releaseEvidenceVerifier == nil || !h.releaseEvidenceVerifier.Status().Configured {
			writeEnvelope(w, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "远端证据工件校验器未配置，不能将证据标记为通过", nil)
			return
		}
		items, err := store.SaaSReleaseEvidence(r.Context())
		if err != nil {
			writeSaaSAdminError(w, err)
			return
		}
		current, found := findSaaSReleaseEvidence(items, input.Key)
		if !found {
			writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "发布证据项不存在", nil)
			return
		}
		if current.Version != input.ExpectedVersion {
			writeEnvelope(w, http.StatusConflict, http.StatusConflict, "发布证据版本已变化，请刷新后重试", nil)
			return
		}
		verification, verifyErr := h.releaseEvidenceVerifier.Verify(r.Context(), input.EvidenceURL, input.ArtifactSHA256, input.ArtifactSizeBytes)
		verificationEvidence := current
		verificationEvidence.EvidenceURL = input.EvidenceURL
		verificationEvidence.ArtifactSHA256 = input.ArtifactSHA256
		verificationEvidence.ArtifactSizeBytes = input.ArtifactSizeBytes
		verification = bindSaaSReleaseArtifactVerification(verificationEvidence, verification)
		if verifyErr != nil || !verification.Verified {
			if verification.Error == "" {
				verification.Error = "远端工件未通过完整性校验"
			}
			writeEnvelope(w, http.StatusUnprocessableEntity, http.StatusUnprocessableEntity, "远端证据工件校验失败："+verification.Error, map[string]any{"artifactVerification": verification})
			return
		}
		input.ArtifactVerification = &verification
	}
	input.ActorUserID, input.ActorTenantID = user.ID, user.TenantID
	result, err := store.UpdateSaaSReleaseEvidence(r.Context(), input)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "ok", map[string]any{
		"evidence": saasReleaseEvidencePayload(result.Evidence), "artifactVerification": result.ArtifactVerification, "operationId": result.OperationID,
	})
}

func (h *SaaSAdminHandler) ReleaseEvidenceAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	store, ok := h.saasReleaseEvidenceActionStore(w)
	if !ok {
		return
	}
	var input SaaSReleaseEvidenceActionUpdate
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid JSON body", nil)
		return
	}
	if err := normalizeSaaSReleaseEvidenceActionUpdate(&input); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	input.ActorUserID, input.ActorTenantID = user.ID, user.TenantID
	result, err := store.UpdateSaaSReleaseEvidenceAction(r.Context(), input)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "ok", map[string]any{
		"action":      saasReleaseEvidenceActionPayload(result.Action, SaaSReleaseEvidence{}, time.Now()),
		"operationId": result.OperationID,
	})
}

func (h *SaaSAdminHandler) ReleaseCandidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	var input SaaSReleaseCandidateCreate
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid JSON body", nil)
		return
	}
	if err := h.normalizeSaaSReleaseCandidateRequest(&input); err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	if h.rejectDirectHighRiskAction(r.Context(), w, SaaSAdminApprovalActionReleaseCandidateGate, 0) {
		return
	}
	result, err := h.createSaaSReleaseCandidate(r.Context(), input, user)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "ok", map[string]any{
		"candidate": saasReleaseCandidatePayload(result.Candidate), "operationId": result.OperationID,
	})
}

func (h *SaaSAdminHandler) normalizeSaaSReleaseCandidateRequest(input *SaaSReleaseCandidateCreate) error {
	runtimeFingerprint, _, authoritative := h.authoritativeReleaseSourceFingerprint()
	if !authoritative {
		return &SaaSAdminOperationError{Status: http.StatusServiceUnavailable, Message: "当前运行版本未内置源码指纹，不能生成发布候选"}
	}
	if strings.TrimSpace(input.SourceFingerprint) == "" {
		input.SourceFingerprint = runtimeFingerprint
	}
	if err := normalizeSaaSReleaseCandidateCreate(input); err != nil {
		return NewSaaSAdminBadRequest(err.Error())
	}
	if input.SourceFingerprint != runtimeFingerprint {
		return &SaaSAdminOperationError{Status: http.StatusConflict, Message: "sourceFingerprint 与当前运行版本不一致"}
	}
	if h.releaseEvidenceVerifier == nil || !h.releaseEvidenceVerifier.Status().Configured {
		return &SaaSAdminOperationError{Status: http.StatusServiceUnavailable, Message: "远端证据工件校验器未配置，不能生成发布候选"}
	}
	return nil
}

func (h *SaaSAdminHandler) createSaaSReleaseCandidate(ctx context.Context, input SaaSReleaseCandidateCreate, user User) (SaaSReleaseCandidateCreateResult, error) {
	if err := h.normalizeSaaSReleaseCandidateRequest(&input); err != nil {
		return SaaSReleaseCandidateCreateResult{}, err
	}
	store, ok := h.store.(SaaSReleaseReadinessStore)
	if !ok || store == nil {
		return SaaSReleaseCandidateCreateResult{}, errors.New("SaaS release readiness store is not configured")
	}
	items, err := store.SaaSReleaseEvidence(ctx)
	if err != nil {
		return SaaSReleaseCandidateCreateResult{}, err
	}
	input.ArtifactVerifications = h.verifySaaSReleaseEvidenceArtifacts(ctx, items)
	candidateNo, err := newSaaSReleaseCandidateNo()
	if err != nil {
		return SaaSReleaseCandidateCreateResult{}, err
	}
	input.CandidateNo = candidateNo
	input.ActorUserID, input.ActorTenantID = user.ID, user.TenantID
	return store.CreateSaaSReleaseCandidate(ctx, input)
}

func (h *SaaSAdminHandler) saasReleaseReadinessStore(w http.ResponseWriter) (SaaSReleaseReadinessStore, bool) {
	store, ok := h.store.(SaaSReleaseReadinessStore)
	if !ok || store == nil {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "SaaS release readiness store is not configured", nil)
		return nil, false
	}
	return store, true
}

func (h *SaaSAdminHandler) saasReleaseEvidenceActionStore(w http.ResponseWriter) (SaaSReleaseEvidenceActionStore, bool) {
	store, ok := h.store.(SaaSReleaseEvidenceActionStore)
	if !ok || store == nil {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "SaaS release evidence action store is not configured", nil)
		return nil, false
	}
	return store, true
}

func (h *SaaSAdminHandler) saasReleaseEvidenceActionOwners(ctx context.Context, platformTenantID int) ([]map[string]any, error) {
	store, ok := h.store.(SaaSAdminAccessStore)
	if !ok || store == nil {
		return []map[string]any{}, nil
	}
	items, err := store.SaaSAdminAccessAssignments(ctx, platformTenantID, SaaSAdminAccessAssignmentOptions{Limit: 100})
	if err != nil {
		return nil, err
	}
	owners := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if item.Status != 1 {
			continue
		}
		owners = append(owners, map[string]any{
			"userId": item.UserID, "name": item.UserName, "phone": item.Phone, "isSuperAdmin": item.IsSuperAdmin,
		})
	}
	return owners, nil
}

func (h *SaaSAdminHandler) authoritativeReleaseSourceFingerprint() (string, string, bool) {
	fingerprint := strings.ToLower(strings.TrimSpace(h.releaseSourceFingerprint))
	if !saasReleaseFingerprintPattern.MatchString(fingerprint) {
		return "", "", false
	}
	source := strings.ToLower(strings.TrimSpace(h.releaseSourceFingerprintSource))
	if source != "build" && source != "environment" {
		source = "runtime"
	}
	return fingerprint, source, true
}

func (h *SaaSAdminHandler) releaseEvidenceVerifierStatus() SaaSReleaseEvidenceVerifierStatus {
	if h == nil || h.releaseEvidenceVerifier == nil {
		return SaaSReleaseEvidenceVerifierStatus{}
	}
	return h.releaseEvidenceVerifier.Status()
}

func (h *SaaSAdminHandler) verifySaaSReleaseEvidenceArtifacts(ctx context.Context, items []SaaSReleaseEvidence) map[string]SaaSReleaseArtifactVerification {
	type verificationResult struct {
		key          string
		verification SaaSReleaseArtifactVerification
	}
	required := make([]SaaSReleaseEvidence, 0, SaaSReleaseEvidenceRequiredCount)
	for _, item := range items {
		if item.Required {
			required = append(required, item)
		}
	}
	results := make(chan verificationResult, len(required))
	for _, item := range required {
		item := item
		go func() {
			verification := SaaSReleaseArtifactVerification{AttemptedAt: time.Now().UTC().Format(time.RFC3339)}
			if !SaaSReleaseEvidenceComplete(item) {
				verification.Error = "证据元数据不完整或尚未通过"
			} else {
				verified, err := h.releaseEvidenceVerifier.Verify(ctx, item.EvidenceURL, item.ArtifactSHA256, item.ArtifactSizeBytes)
				verification = verified
				if err != nil && verification.Error == "" {
					verification.Error = truncateSaaSReleaseVerificationText(err.Error())
				}
				if err != nil {
					verification.Verified = false
				}
				if err == nil && !verification.Verified && verification.Error == "" {
					verification.Error = "远端工件未通过完整性校验"
				}
			}
			results <- verificationResult{key: item.Key, verification: bindSaaSReleaseArtifactVerification(item, verification)}
		}()
	}
	verifications := make(map[string]SaaSReleaseArtifactVerification, len(required))
	for range required {
		result := <-results
		verifications[result.key] = result.verification
	}
	return verifications
}

func findSaaSReleaseEvidence(items []SaaSReleaseEvidence, key string) (SaaSReleaseEvidence, bool) {
	for _, item := range items {
		if item.Key == key {
			return item, true
		}
	}
	return SaaSReleaseEvidence{}, false
}

func bindSaaSReleaseArtifactVerification(item SaaSReleaseEvidence, verification SaaSReleaseArtifactVerification) SaaSReleaseArtifactVerification {
	verification.EvidenceKey = item.Key
	verification.EvidenceVersion = item.Version
	verification.EvidenceURL = item.EvidenceURL
	verification.ExpectedSHA256 = strings.ToLower(strings.TrimSpace(item.ArtifactSHA256))
	verification.ExpectedSizeBytes = item.ArtifactSizeBytes
	return verification
}

func normalizeSaaSReleaseEvidenceUpdate(input *SaaSReleaseEvidenceUpdate) error {
	input.Key = strings.ToLower(strings.TrimSpace(input.Key))
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	input.EvidenceURL = strings.TrimSpace(input.EvidenceURL)
	input.Environment = strings.TrimSpace(input.Environment)
	input.SourceFingerprint = strings.ToLower(strings.TrimSpace(input.SourceFingerprint))
	input.ArtifactSHA256 = strings.ToLower(strings.TrimSpace(input.ArtifactSHA256))
	input.Note = strings.TrimSpace(input.Note)
	if input.Key == "" || input.ExpectedVersion <= 0 {
		return errors.New("key 和 expectedVersion 必填")
	}
	if len([]rune(input.Environment)) > 80 || len([]rune(input.Note)) > 1000 {
		return errors.New("environment 最多 80 个字符，note 最多 1000 个字符")
	}
	if saasReleaseSecretPattern.MatchString(input.Environment) || saasReleaseSecretPattern.MatchString(input.Note) {
		return errors.New("发布证据元数据不能包含凭据或密钥")
	}
	switch input.Status {
	case SaaSReleaseEvidenceStatusMissing:
		input.EvidenceURL, input.Environment, input.SourceFingerprint, input.ArtifactSHA256, input.ArtifactSizeBytes = "", "", "", "", 0
	case SaaSReleaseEvidenceStatusProgress:
		if input.EvidenceURL != "" {
			if err := validateSaaSReleaseEvidenceURL(input.EvidenceURL); err != nil {
				return err
			}
		}
		if input.SourceFingerprint != "" && !saasReleaseFingerprintPattern.MatchString(input.SourceFingerprint) {
			return errors.New("sourceFingerprint 必须是 64 位 SHA-256")
		}
		if err := validateSaaSReleaseArtifact(input.ArtifactSHA256, input.ArtifactSizeBytes, false); err != nil {
			return err
		}
	case SaaSReleaseEvidenceStatusPassed:
		if input.EvidenceURL == "" || input.Environment == "" || !saasReleaseFingerprintPattern.MatchString(input.SourceFingerprint) {
			return errors.New("证据通过时 evidenceUrl、environment 和 64 位 sourceFingerprint 必填")
		}
		if err := validateSaaSReleaseEvidenceURL(input.EvidenceURL); err != nil {
			return err
		}
		if err := validateSaaSReleaseArtifact(input.ArtifactSHA256, input.ArtifactSizeBytes, true); err != nil {
			return err
		}
	case SaaSReleaseEvidenceStatusFailed:
		if input.Note == "" {
			return errors.New("证据失败时 note 必填")
		}
		if input.EvidenceURL != "" {
			if err := validateSaaSReleaseEvidenceURL(input.EvidenceURL); err != nil {
				return err
			}
		}
		if input.SourceFingerprint != "" && !saasReleaseFingerprintPattern.MatchString(input.SourceFingerprint) {
			return errors.New("sourceFingerprint 必须是 64 位 SHA-256")
		}
		if err := validateSaaSReleaseArtifact(input.ArtifactSHA256, input.ArtifactSizeBytes, false); err != nil {
			return err
		}
	default:
		return errors.New("status 必须是 missing/in_progress/passed/failed")
	}
	return nil
}

func normalizeSaaSReleaseEvidenceActionUpdate(input *SaaSReleaseEvidenceActionUpdate) error {
	input.Key = strings.ToLower(strings.TrimSpace(input.Key))
	input.DueAt = strings.TrimSpace(input.DueAt)
	input.NextAction = strings.TrimSpace(input.NextAction)
	input.Note = strings.TrimSpace(input.Note)
	if input.Key == "" || len(input.Key) > 64 || input.ExpectedVersion <= 0 {
		return errors.New("key 和 expectedVersion 必填")
	}
	if input.OwnerUserID < 0 {
		return errors.New("ownerUserId 不能为负数")
	}
	if len([]rune(input.NextAction)) == 0 || len([]rune(input.NextAction)) > 500 || len([]rune(input.Note)) > 1000 {
		return errors.New("nextAction 必填且最多 500 个字符，note 最多 1000 个字符")
	}
	if saasReleaseSecretPattern.MatchString(input.NextAction) || saasReleaseSecretPattern.MatchString(input.Note) {
		return errors.New("补证行动不能包含凭据或密钥")
	}
	if input.DueAt != "" {
		input.DueAt = strings.Replace(input.DueAt, "T", " ", 1)
		if len(input.DueAt) == 16 {
			input.DueAt += ":00"
		}
		normalized, err := normalizeSaaSAdminOptionalDateTime(input.DueAt)
		if err != nil {
			return errors.New("dueAt 格式无效或超出 MySQL 时间范围")
		}
		input.DueAt = normalized
	}
	if input.OwnerUserID == 0 && input.DueAt != "" {
		return errors.New("未分配负责人时不能设置截止时间")
	}
	if input.OwnerUserID > 0 && input.DueAt == "" {
		return errors.New("分配负责人时 dueAt 必填")
	}
	return nil
}

func validateSaaSReleaseArtifact(sha256 string, sizeBytes int64, required bool) error {
	if sizeBytes < 0 {
		return errors.New("artifactSizeBytes 不能为负数")
	}
	if sha256 == "" && sizeBytes == 0 {
		if required {
			return errors.New("证据通过时 64 位 artifactSha256 和大于 0 的 artifactSizeBytes 必填")
		}
		return nil
	}
	if !saasReleaseFingerprintPattern.MatchString(sha256) || sizeBytes <= 0 {
		return errors.New("artifactSha256 必须是 64 位 SHA-256，且 artifactSizeBytes 必须大于 0")
	}
	return nil
}

func normalizeSaaSReleaseCandidateCreate(input *SaaSReleaseCandidateCreate) error {
	input.ReleaseVersion = strings.TrimSpace(input.ReleaseVersion)
	input.SourceFingerprint = strings.ToLower(strings.TrimSpace(input.SourceFingerprint))
	if !saasReleaseVersionPattern.MatchString(input.ReleaseVersion) {
		return errors.New("releaseVersion 必须是 1 至 64 位版本标识")
	}
	if !saasReleaseFingerprintPattern.MatchString(input.SourceFingerprint) {
		return errors.New("sourceFingerprint 必须是 64 位 SHA-256")
	}
	return nil
}

func validateSaaSReleaseEvidenceURL(value string) error {
	if len(value) > 1000 {
		return errors.New("evidenceUrl 最多 1000 个字符")
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("evidenceUrl 必须是不含凭据、查询参数和片段的 HTTPS 地址")
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".test") || strings.HasSuffix(host, ".invalid") || host == "example.com" || host == "example.org" || host == "example.net" || strings.HasSuffix(host, ".example.com") || strings.HasSuffix(host, ".example.org") || strings.HasSuffix(host, ".example.net") {
		return errors.New("evidenceUrl 必须指向真实生产证据地址")
	}
	return nil
}

func newSaaSReleaseCandidateNo() (string, error) {
	random := make([]byte, 6)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return "REL-" + time.Now().UTC().Format("20060102T150405") + "-" + strings.ToUpper(hex.EncodeToString(random)), nil
}

func saasReleaseReadinessSummaryPayload(items []SaaSReleaseEvidence, candidates []SaaSReleaseCandidate, fingerprint string, authoritative bool, fingerprintSource string, verifier SaaSReleaseEvidenceVerifierStatus) map[string]any {
	required, passed, progress, failed, matched := 0, 0, 0, 0, 0
	for _, item := range items {
		if !item.Required {
			continue
		}
		required++
		switch item.Status {
		case SaaSReleaseEvidenceStatusPassed:
			if SaaSReleaseEvidenceComplete(item) {
				passed++
				if fingerprint != "" && item.SourceFingerprint == fingerprint {
					matched++
				}
			} else {
				failed++
			}
		case SaaSReleaseEvidenceStatusProgress:
			progress++
		case SaaSReleaseEvidenceStatusFailed:
			failed++
		}
	}
	metadataReady := authoritative && saasReleaseEvidenceMetadataReady(items, fingerprint)
	var latest *SaaSReleaseCandidate
	for index := range candidates {
		if candidates[index].SourceFingerprint == fingerprint {
			latest = &candidates[index]
			break
		}
	}
	latestEvaluation := saasReleaseCandidateEvaluation{}
	ready := false
	remoteVerifiedCount, latestCandidateNo, latestCandidateStatus, latestCandidateEffectiveStatus := 0, "", "", ""
	if latest != nil {
		remoteVerifiedCount, latestCandidateNo, latestCandidateStatus = latest.PassedCount, latest.CandidateNo, latest.Status
		latestEvaluation = evaluateSaaSReleaseCandidate(*latest, items)
		latestCandidateEffectiveStatus = latestEvaluation.EffectiveStatus
		ready = authoritative && verifier.Configured && metadataReady && latestEvaluation.EffectiveStatus == SaaSReleaseCandidateStatusReady
	}
	return map[string]any{
		"requiredCount": required, "passedCount": passed, "missingCount": required - passed - progress - failed,
		"inProgressCount": progress, "failedCount": failed, "fingerprintMatchedCount": matched,
		"targetSourceFingerprint": fingerprint, "sourceFingerprintAuthoritative": authoritative,
		"sourceFingerprintSource": fingerprintSource, "candidateGateEnabled": authoritative && verifier.Configured && metadataReady,
		"metadataReady": metadataReady, "ready": ready, "remoteVerifiedCount": remoteVerifiedCount,
		"latestCandidateNo": latestCandidateNo, "latestCandidateStatus": latestCandidateStatus,
		"latestCandidateEffectiveStatus":     latestCandidateEffectiveStatus,
		"latestCandidateSnapshotValid":       latestEvaluation.SnapshotValid,
		"latestCandidateEvidenceCurrent":     latestEvaluation.EvidenceCurrent,
		"latestCandidateDriftedEvidenceKeys": latestEvaluation.DriftedEvidenceKeys,
		"artifactVerifier":                   verifier,
	}
}

func saasReleaseEvidenceMetadataReady(items []SaaSReleaseEvidence, fingerprint string) bool {
	fingerprint = strings.ToLower(strings.TrimSpace(fingerprint))
	if !saasReleaseFingerprintPattern.MatchString(fingerprint) {
		return false
	}
	required, passed, matched := 0, 0, 0
	for _, item := range items {
		if !item.Required {
			continue
		}
		required++
		if SaaSReleaseEvidenceComplete(item) {
			passed++
			if item.SourceFingerprint == fingerprint {
				matched++
			}
		}
	}
	return required == SaaSReleaseEvidenceRequiredCount && passed == required && matched == required
}

func SaaSReleaseEvidenceComplete(item SaaSReleaseEvidence) bool {
	return item.Status == SaaSReleaseEvidenceStatusPassed &&
		strings.TrimSpace(item.Environment) != "" &&
		saasReleaseFingerprintPattern.MatchString(strings.ToLower(strings.TrimSpace(item.SourceFingerprint))) &&
		saasReleaseFingerprintPattern.MatchString(strings.ToLower(strings.TrimSpace(item.ArtifactSHA256))) &&
		item.ArtifactSizeBytes > 0 &&
		strings.TrimSpace(item.CheckedAt) != "" && item.CheckedBy > 0 &&
		validateSaaSReleaseEvidenceURL(strings.TrimSpace(item.EvidenceURL)) == nil
}

func SaaSReleaseArtifactVerificationMatchesEvidence(item SaaSReleaseEvidence, verification SaaSReleaseArtifactVerification) bool {
	return verification.EvidenceKey == item.Key && verification.EvidenceVersion == item.Version &&
		verification.EvidenceURL == item.EvidenceURL && verification.ExpectedSHA256 == item.ArtifactSHA256 &&
		verification.ExpectedSizeBytes == item.ArtifactSizeBytes
}

func SaaSReleaseArtifactVerificationPassed(item SaaSReleaseEvidence, verification SaaSReleaseArtifactVerification) bool {
	return SaaSReleaseArtifactVerificationMatchesEvidence(item, verification) && verification.Verified && verification.Error == "" &&
		verification.ActualSHA256 == item.ArtifactSHA256 && verification.ActualSizeBytes == item.ArtifactSizeBytes &&
		verification.HTTPStatus == http.StatusOK && strings.TrimSpace(verification.AttemptedAt) != ""
}

func evaluateSaaSReleaseCandidate(candidate SaaSReleaseCandidate, items []SaaSReleaseEvidence) saasReleaseCandidateEvaluation {
	evaluation := saasReleaseCandidateEvaluation{EffectiveStatus: candidate.Status, DriftedEvidenceKeys: []string{}}
	if candidate.Status != SaaSReleaseCandidateStatusReady {
		return evaluation
	}
	evaluation.EffectiveStatus = SaaSReleaseCandidateStatusStale
	var snapshots []saasReleaseCandidateEvidenceSnapshot
	if err := json.Unmarshal([]byte(candidate.SnapshotJSON), &snapshots); err != nil || len(snapshots) == 0 {
		return evaluation
	}
	snapshotByKey := make(map[string]saasReleaseCandidateEvidenceSnapshot, len(snapshots))
	duplicateKeys := make(map[string]bool)
	for _, snapshot := range snapshots {
		key := strings.TrimSpace(snapshot.Key)
		if key == "" {
			continue
		}
		if _, exists := snapshotByKey[key]; exists {
			duplicateKeys[key] = true
			continue
		}
		snapshotByKey[key] = snapshot
	}
	requiredCount, validSnapshotCount := 0, 0
	for _, current := range items {
		if !current.Required {
			continue
		}
		requiredCount++
		snapshot, found := snapshotByKey[current.Key]
		if !found || duplicateKeys[current.Key] || snapshot.ArtifactVerification == nil {
			evaluation.DriftedEvidenceKeys = appendUniqueSaaSReleaseEvidenceKey(evaluation.DriftedEvidenceKeys, current.Key)
			continue
		}
		snapshotEvidence := snapshot.saasReleaseEvidence()
		if !SaaSReleaseEvidenceComplete(snapshotEvidence) || snapshotEvidence.SourceFingerprint != candidate.SourceFingerprint ||
			!SaaSReleaseArtifactVerificationPassed(snapshotEvidence, *snapshot.ArtifactVerification) {
			evaluation.DriftedEvidenceKeys = appendUniqueSaaSReleaseEvidenceKey(evaluation.DriftedEvidenceKeys, current.Key)
			continue
		}
		validSnapshotCount++
		if !sameSaaSReleaseEvidenceIdentity(current, snapshotEvidence) {
			evaluation.DriftedEvidenceKeys = appendUniqueSaaSReleaseEvidenceKey(evaluation.DriftedEvidenceKeys, current.Key)
		}
	}
	evaluation.SnapshotValid = requiredCount == SaaSReleaseEvidenceRequiredCount &&
		candidate.RequiredCount == requiredCount && candidate.PassedCount == requiredCount && candidate.MatchedCount == requiredCount &&
		validSnapshotCount == requiredCount
	evaluation.EvidenceCurrent = evaluation.SnapshotValid && len(evaluation.DriftedEvidenceKeys) == 0
	if evaluation.EvidenceCurrent {
		evaluation.EffectiveStatus = SaaSReleaseCandidateStatusReady
	}
	return evaluation
}

func (snapshot saasReleaseCandidateEvidenceSnapshot) saasReleaseEvidence() SaaSReleaseEvidence {
	return SaaSReleaseEvidence{
		ID: snapshot.ID, Key: snapshot.Key, Required: true, Status: snapshot.Status,
		EvidenceURL: snapshot.EvidenceURL, Environment: snapshot.Environment, SourceFingerprint: snapshot.SourceFingerprint,
		ArtifactSHA256: snapshot.ArtifactSHA256, ArtifactSizeBytes: snapshot.ArtifactSizeBytes,
		CheckedAt: snapshot.CheckedAt, CheckedBy: snapshot.CheckedBy, Note: snapshot.Note, Version: snapshot.Version,
	}
}

func sameSaaSReleaseEvidenceIdentity(current SaaSReleaseEvidence, snapshot SaaSReleaseEvidence) bool {
	return current.ID == snapshot.ID && current.Key == snapshot.Key && current.Status == snapshot.Status &&
		current.EvidenceURL == snapshot.EvidenceURL && current.Environment == snapshot.Environment &&
		current.SourceFingerprint == snapshot.SourceFingerprint && current.ArtifactSHA256 == snapshot.ArtifactSHA256 &&
		current.ArtifactSizeBytes == snapshot.ArtifactSizeBytes && current.CheckedAt == snapshot.CheckedAt &&
		current.CheckedBy == snapshot.CheckedBy && current.Note == snapshot.Note && current.Version == snapshot.Version
}

func appendUniqueSaaSReleaseEvidenceKey(keys []string, key string) []string {
	for _, existing := range keys {
		if existing == key {
			return keys
		}
	}
	return append(keys, key)
}

func saasReleaseEvidenceActionState(action SaaSReleaseEvidenceAction, evidence SaaSReleaseEvidence) string {
	if SaaSReleaseEvidenceComplete(evidence) {
		return SaaSReleaseEvidenceActionStateResolved
	}
	if evidence.Status == SaaSReleaseEvidenceStatusFailed {
		return SaaSReleaseEvidenceActionStateBlocked
	}
	if action.OwnerUserID <= 0 || !action.OwnerActive {
		return SaaSReleaseEvidenceActionStateUnassigned
	}
	return SaaSReleaseEvidenceActionStateProgress
}

func saasReleaseEvidenceActionDueState(action SaaSReleaseEvidenceAction, evidence SaaSReleaseEvidence, now time.Time) string {
	if SaaSReleaseEvidenceComplete(evidence) {
		return SaaSReleaseEvidenceActionDueResolved
	}
	dueAt, ok := parseSaaSAdminNormalizedDateTime(action.DueAt)
	if !ok {
		return SaaSReleaseEvidenceActionDueNoDate
	}
	if !dueAt.After(now) {
		return SaaSReleaseEvidenceActionDueOverdue
	}
	if !dueAt.After(now.Add(SaaSReleaseEvidenceActionDueSoonHours * time.Hour)) {
		return SaaSReleaseEvidenceActionDueSoon
	}
	return SaaSReleaseEvidenceActionDueScheduled
}

func saasReleaseEvidenceActionPayload(action SaaSReleaseEvidenceAction, evidence SaaSReleaseEvidence, now time.Time) map[string]any {
	title := strings.TrimSpace(action.Title)
	if title == "" {
		title = evidence.Title
	}
	return map[string]any{
		"id": action.ID, "key": action.Key, "title": title,
		"evidenceStatus": evidence.Status, "evidenceComplete": SaaSReleaseEvidenceComplete(evidence),
		"state": saasReleaseEvidenceActionState(action, evidence), "dueState": saasReleaseEvidenceActionDueState(action, evidence, now),
		"ownerUserId": action.OwnerUserID, "ownerName": action.OwnerName, "ownerPhone": action.OwnerPhone, "ownerActive": action.OwnerActive,
		"dueAt": action.DueAt, "nextAction": action.NextAction, "note": action.Note, "version": action.Version,
		"createdBy": action.CreatedBy, "updatedBy": action.UpdatedBy, "createdAt": action.CreatedAt, "updatedAt": action.UpdatedAt,
	}
}

func saasReleaseEvidenceActionsPayload(actions []SaaSReleaseEvidenceAction, evidence []SaaSReleaseEvidence, now time.Time) []map[string]any {
	evidenceByKey := make(map[string]SaaSReleaseEvidence, len(evidence))
	for _, item := range evidence {
		evidenceByKey[item.Key] = item
	}
	result := make([]map[string]any, 0, len(actions))
	for _, action := range actions {
		result = append(result, saasReleaseEvidenceActionPayload(action, evidenceByKey[action.Key], now))
	}
	return result
}

func saasReleaseEvidenceActionSummaryPayload(actions []SaaSReleaseEvidenceAction, evidence []SaaSReleaseEvidence, now time.Time) map[string]any {
	evidenceByKey := make(map[string]SaaSReleaseEvidence, len(evidence))
	for _, item := range evidence {
		evidenceByKey[item.Key] = item
	}
	resolved, blocked, assigned, unassigned, overdue, dueSoon := 0, 0, 0, 0, 0, 0
	for _, action := range actions {
		item := evidenceByKey[action.Key]
		state := saasReleaseEvidenceActionState(action, item)
		dueState := saasReleaseEvidenceActionDueState(action, item, now)
		if state == SaaSReleaseEvidenceActionStateResolved {
			resolved++
			continue
		}
		if state == SaaSReleaseEvidenceActionStateBlocked {
			blocked++
		}
		if action.OwnerUserID > 0 && action.OwnerActive {
			assigned++
		} else {
			unassigned++
		}
		if dueState == SaaSReleaseEvidenceActionDueOverdue {
			overdue++
		}
		if dueState == SaaSReleaseEvidenceActionDueSoon {
			dueSoon++
		}
	}
	return map[string]any{
		"totalCount": len(actions), "resolvedCount": resolved, "unresolvedCount": len(actions) - resolved,
		"blockedCount": blocked, "assignedCount": assigned, "unassignedCount": unassigned,
		"overdueCount": overdue, "dueSoonCount": dueSoon,
	}
}

func saasReleaseEvidencePayload(item SaaSReleaseEvidence) map[string]any {
	return map[string]any{
		"id": item.ID, "key": item.Key, "title": item.Title, "category": item.Category, "required": item.Required,
		"status": item.Status, "evidenceUrl": item.EvidenceURL, "environment": item.Environment,
		"sourceFingerprint": item.SourceFingerprint, "artifactSha256": item.ArtifactSHA256,
		"artifactSizeBytes": item.ArtifactSizeBytes, "checkedAt": item.CheckedAt, "checkedBy": item.CheckedBy,
		"note": item.Note, "version": item.Version, "createdAt": item.CreatedAt, "updatedAt": item.UpdatedAt,
	}
}

func saasReleaseEvidenceItemsPayload(items []SaaSReleaseEvidence) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, saasReleaseEvidencePayload(item))
	}
	return result
}

func saasReleaseCandidatePayload(item SaaSReleaseCandidate) map[string]any {
	return map[string]any{
		"id": item.ID, "candidateNo": item.CandidateNo, "releaseVersion": item.ReleaseVersion,
		"sourceFingerprint": item.SourceFingerprint, "status": item.Status, "requiredCount": item.RequiredCount,
		"passedCount": item.PassedCount, "matchedCount": item.MatchedCount, "gateMessage": item.GateMessage,
		"createdBy": item.CreatedBy, "operationId": item.OperationID, "createdAt": item.CreatedAt,
	}
}

func saasReleaseCandidatesPayload(items []SaaSReleaseCandidate, evidence []SaaSReleaseEvidence) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payload := saasReleaseCandidatePayload(item)
		evaluation := evaluateSaaSReleaseCandidate(item, evidence)
		payload["effectiveStatus"] = evaluation.EffectiveStatus
		payload["snapshotValid"] = evaluation.SnapshotValid
		payload["evidenceCurrent"] = evaluation.EvidenceCurrent
		payload["driftedEvidenceKeys"] = evaluation.DriftedEvidenceKeys
		result = append(result, payload)
	}
	return result
}
