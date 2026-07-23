package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeSaaSReleaseArtifactVerifier struct {
	mu       sync.Mutex
	failures map[string]string
	calls    int
}

func (verifier *fakeSaaSReleaseArtifactVerifier) Verify(_ context.Context, evidenceURL string, expectedSHA256 string, expectedSizeBytes int64) (SaaSReleaseArtifactVerification, error) {
	verifier.mu.Lock()
	defer verifier.mu.Unlock()
	verifier.calls++
	result := SaaSReleaseArtifactVerification{
		EvidenceURL: evidenceURL, ExpectedSHA256: expectedSHA256, ExpectedSizeBytes: expectedSizeBytes,
		ActualSHA256: expectedSHA256, ActualSizeBytes: expectedSizeBytes, HTTPStatus: http.StatusOK,
		AttemptedAt: time.Now().UTC().Format(time.RFC3339), Verified: true,
	}
	if message := verifier.failures[evidenceURL]; message != "" {
		result.Verified = false
		result.Error = message
		return result, errors.New(message)
	}
	return result, nil
}

func (verifier *fakeSaaSReleaseArtifactVerifier) Status() SaaSReleaseEvidenceVerifierStatus {
	return SaaSReleaseEvidenceVerifierStatus{
		Configured: true, VerifyOnPass: true, VerifyOnCandidateGate: true,
		TimeoutSeconds: 30, MaxBytes: 64 << 20, RequireHTTPS: true,
		PrivateNetworksBlocked: true, MetadataAddressesBlocked: true, DNSPinningEnabled: true,
		SameOriginRedirectsOnly: true, EnvironmentProxyDisabled: true,
	}
}

type fakeSaaSReleaseReadinessStore struct {
	*fakeSaaSAdminStore
	evidence        []SaaSReleaseEvidence
	candidates      []SaaSReleaseCandidate
	updateResult    SaaSReleaseEvidenceUpdateResult
	actions         []SaaSReleaseEvidenceAction
	actionResult    SaaSReleaseEvidenceActionUpdateResult
	candidateResult SaaSReleaseCandidateCreateResult
	lastUpdate      SaaSReleaseEvidenceUpdate
	lastAction      SaaSReleaseEvidenceActionUpdate
	lastCandidate   SaaSReleaseCandidateCreate
	updateCalls     int
	actionCalls     int
	candidateCalls  int
}

func mustSaaSReleaseCandidateSnapshot(t *testing.T, items []SaaSReleaseEvidence) string {
	t.Helper()
	snapshots := make([]map[string]any, 0, len(items))
	for _, item := range items {
		verification := SaaSReleaseArtifactVerification{
			EvidenceKey: item.Key, EvidenceVersion: item.Version, EvidenceURL: item.EvidenceURL,
			ExpectedSHA256: item.ArtifactSHA256, ExpectedSizeBytes: item.ArtifactSizeBytes,
			Verified: true, AttemptedAt: "2026-07-14T10:00:00Z", ActualSHA256: item.ArtifactSHA256,
			ActualSizeBytes: item.ArtifactSizeBytes, HTTPStatus: http.StatusOK,
		}
		snapshots = append(snapshots, map[string]any{
			"id": item.ID, "key": item.Key, "status": item.Status, "evidenceUrl": item.EvidenceURL,
			"environment": item.Environment, "sourceFingerprint": item.SourceFingerprint,
			"artifactSha256": item.ArtifactSHA256, "artifactSizeBytes": item.ArtifactSizeBytes,
			"checkedAt": item.CheckedAt, "checkedBy": item.CheckedBy, "note": item.Note, "version": item.Version,
			"artifactVerification": verification,
		})
	}
	raw, err := json.Marshal(snapshots)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func (store *fakeSaaSReleaseReadinessStore) SaaSReleaseEvidence(context.Context) ([]SaaSReleaseEvidence, error) {
	return store.evidence, nil
}

func (store *fakeSaaSReleaseReadinessStore) UpdateSaaSReleaseEvidence(_ context.Context, input SaaSReleaseEvidenceUpdate) (SaaSReleaseEvidenceUpdateResult, error) {
	store.updateCalls++
	store.lastUpdate = input
	return store.updateResult, nil
}

func (store *fakeSaaSReleaseReadinessStore) SaaSReleaseEvidenceActions(context.Context, int) ([]SaaSReleaseEvidenceAction, error) {
	return store.actions, nil
}

func (store *fakeSaaSReleaseReadinessStore) UpdateSaaSReleaseEvidenceAction(_ context.Context, input SaaSReleaseEvidenceActionUpdate) (SaaSReleaseEvidenceActionUpdateResult, error) {
	store.actionCalls++
	store.lastAction = input
	return store.actionResult, nil
}

func (store *fakeSaaSReleaseReadinessStore) SaaSReleaseCandidates(context.Context, int) ([]SaaSReleaseCandidate, error) {
	return store.candidates, nil
}

func (store *fakeSaaSReleaseReadinessStore) CreateSaaSReleaseCandidate(_ context.Context, input SaaSReleaseCandidateCreate) (SaaSReleaseCandidateCreateResult, error) {
	store.candidateCalls++
	store.lastCandidate = input
	return store.candidateResult, nil
}

func TestSaaSReleaseEvidenceValidationRequiresProductionURLAndArtifactIntegrity(t *testing.T) {
	fingerprint := strings.Repeat("a", 64)
	artifactSHA256 := strings.Repeat("d", 64)
	valid := SaaSReleaseEvidenceUpdate{
		Key: "real_wecom", Status: "passed", EvidenceURL: "https://evidence.company.cn/wecom/run-1",
		Environment: "production", SourceFingerprint: fingerprint, ArtifactSHA256: artifactSHA256,
		ArtifactSizeBytes: 4096, ExpectedVersion: 1,
	}
	if err := normalizeSaaSReleaseEvidenceUpdate(&valid); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{
		"http://evidence.company.cn/run", "https://localhost/run",
		"https://evidence.example.com/run", "https://evidence.company.cn/run?token=secret",
	} {
		input := valid
		input.EvidenceURL = value
		if err := normalizeSaaSReleaseEvidenceUpdate(&input); err == nil {
			t.Fatalf("expected URL %q to be rejected", value)
		}
	}
	valid.Note = "Authorization: Bearer abcdefghijklmnopqrstuvwxyz"
	if err := normalizeSaaSReleaseEvidenceUpdate(&valid); err == nil {
		t.Fatal("expected secret-like note to be rejected")
	}
	for _, input := range []SaaSReleaseEvidenceUpdate{
		{Key: "real_wecom", Status: "passed", EvidenceURL: "https://evidence.company.cn/run", Environment: "production", SourceFingerprint: fingerprint, ExpectedVersion: 1},
		{Key: "real_wecom", Status: "in_progress", ArtifactSHA256: artifactSHA256, ExpectedVersion: 1},
		{Key: "real_wecom", Status: "failed", Note: "校验失败", ArtifactSHA256: artifactSHA256, ArtifactSizeBytes: -1, ExpectedVersion: 1},
	} {
		if err := normalizeSaaSReleaseEvidenceUpdate(&input); err == nil {
			t.Fatalf("expected invalid artifact metadata to be rejected: %+v", input)
		}
	}
}

func TestSaaSReleaseEvidenceActionValidationRequiresTrackableAssignment(t *testing.T) {
	valid := SaaSReleaseEvidenceActionUpdate{
		Key: "real_wecom", OwnerUserID: 7, DueAt: "2026-07-22T09:30",
		NextAction: "完成真实企微授权与回调联调", Note: "由发布运营跟进", ExpectedVersion: 1,
	}
	if err := normalizeSaaSReleaseEvidenceActionUpdate(&valid); err != nil {
		t.Fatal(err)
	}
	if valid.DueAt != "2026-07-22 09:30:00" {
		t.Fatalf("dueAt=%q", valid.DueAt)
	}
	for _, input := range []SaaSReleaseEvidenceActionUpdate{
		{Key: "real_wecom", OwnerUserID: 7, NextAction: "已分配但无截止时间", ExpectedVersion: 1},
		{Key: "real_wecom", DueAt: "2026-07-22 09:30:00", NextAction: "无负责人却有截止时间", ExpectedVersion: 1},
		{Key: "real_wecom", OwnerUserID: 7, DueAt: "invalid", NextAction: "无效时间", ExpectedVersion: 1},
		{Key: "real_wecom", OwnerUserID: 7, DueAt: "2026-07-22", NextAction: "Authorization: Bearer hidden", ExpectedVersion: 1},
		{Key: "real_wecom", OwnerUserID: 7, DueAt: "2026-07-22", ExpectedVersion: 1},
	} {
		if err := normalizeSaaSReleaseEvidenceActionUpdate(&input); err == nil {
			t.Fatalf("expected invalid action to be rejected: %+v", input)
		}
	}
}

func TestSaaSReleaseEvidenceActionSummaryTracksOwnershipAndDeadlines(t *testing.T) {
	now := time.Date(2026, 7, 19, 10, 0, 0, 0, time.Local)
	fingerprint := strings.Repeat("a", 64)
	artifact := strings.Repeat("b", 64)
	complete := SaaSReleaseEvidence{
		Key: "mysql57_amd64", Title: "MySQL 5.7", Status: SaaSReleaseEvidenceStatusPassed,
		EvidenceURL: "https://evidence.company.cn/mysql57", Environment: "production", SourceFingerprint: fingerprint,
		ArtifactSHA256: artifact, ArtifactSizeBytes: 10, CheckedAt: "2026-07-19 09:00:00", CheckedBy: 7,
	}
	evidence := []SaaSReleaseEvidence{
		complete,
		{Key: "real_wecom", Status: SaaSReleaseEvidenceStatusFailed},
		{Key: "real_wechat_open", Status: SaaSReleaseEvidenceStatusMissing},
		{Key: "real_saas_tenants", Status: SaaSReleaseEvidenceStatusProgress},
		{Key: "production_frontend", Status: SaaSReleaseEvidenceStatusProgress},
		{Key: "stability", Status: SaaSReleaseEvidenceStatusProgress},
	}
	actions := []SaaSReleaseEvidenceAction{
		{Key: "mysql57_amd64", OwnerUserID: 7, OwnerActive: true},
		{Key: "real_wecom", OwnerUserID: 7, OwnerActive: true, DueAt: now.Add(96 * time.Hour).Format("2006-01-02 15:04:05")},
		{Key: "real_wechat_open"},
		{Key: "real_saas_tenants", OwnerUserID: 8, OwnerActive: true, DueAt: now.Add(-time.Hour).Format("2006-01-02 15:04:05")},
		{Key: "production_frontend", OwnerUserID: 9, OwnerActive: true, DueAt: now.Add(24 * time.Hour).Format("2006-01-02 15:04:05")},
		{Key: "stability", OwnerUserID: 10, OwnerActive: true, DueAt: now.Add(10 * 24 * time.Hour).Format("2006-01-02 15:04:05")},
	}
	summary := saasReleaseEvidenceActionSummaryPayload(actions, evidence, now)
	if summary["totalCount"] != 6 || summary["resolvedCount"] != 1 || summary["blockedCount"] != 1 ||
		summary["assignedCount"] != 4 || summary["unassignedCount"] != 1 || summary["overdueCount"] != 1 || summary["dueSoonCount"] != 1 {
		t.Fatalf("summary=%+v", summary)
	}
	payloads := saasReleaseEvidenceActionsPayload(actions, evidence, now)
	if payloads[0]["state"] != SaaSReleaseEvidenceActionStateResolved || payloads[1]["state"] != SaaSReleaseEvidenceActionStateBlocked ||
		payloads[2]["state"] != SaaSReleaseEvidenceActionStateUnassigned || payloads[3]["dueState"] != SaaSReleaseEvidenceActionDueOverdue ||
		payloads[4]["dueState"] != SaaSReleaseEvidenceActionDueSoon || payloads[5]["dueState"] != SaaSReleaseEvidenceActionDueScheduled {
		t.Fatalf("payloads=%+v", payloads)
	}
}

func TestSaaSReleaseEvidenceCompleteRejectsBypassedIncompleteRows(t *testing.T) {
	item := SaaSReleaseEvidence{
		Status: SaaSReleaseEvidenceStatusPassed, EvidenceURL: "https://evidence.company.cn/run",
		Environment: "production", SourceFingerprint: strings.Repeat("c", 64),
		ArtifactSHA256: strings.Repeat("d", 64), ArtifactSizeBytes: 4096,
		CheckedAt: "2026-07-12 02:00:00", CheckedBy: 9,
	}
	if !SaaSReleaseEvidenceComplete(item) {
		t.Fatal("complete evidence should pass")
	}
	item.CheckedBy = 0
	if SaaSReleaseEvidenceComplete(item) {
		t.Fatal("evidence without verifier must not pass")
	}
	item.CheckedBy = 9
	item.ArtifactSizeBytes = 0
	if SaaSReleaseEvidenceComplete(item) {
		t.Fatal("evidence without artifact size must not pass")
	}
	item.ArtifactSizeBytes = 4096
	item.EvidenceURL = "https://localhost/run"
	if SaaSReleaseEvidenceComplete(item) {
		t.Fatal("local evidence URL must not pass")
	}
}

func TestSaaSReleaseReadinessHandlersPreserveActorAndGateResult(t *testing.T) {
	fingerprint := strings.Repeat("b", 64)
	artifactSHA256 := strings.Repeat("c", 64)
	evidence := make([]SaaSReleaseEvidence, 0, SaaSReleaseEvidenceRequiredCount)
	for index, key := range []string{"mysql57_amd64", "real_wecom", "real_wechat_open", "real_saas_tenants", "production_frontend", "stability"} {
		evidence = append(evidence, SaaSReleaseEvidence{
			ID: int64(index + 1), Key: key, Title: key, Required: true, Status: SaaSReleaseEvidenceStatusPassed,
			EvidenceURL: "https://evidence.company.cn/" + key, Environment: "production", SourceFingerprint: fingerprint,
			ArtifactSHA256: artifactSHA256, ArtifactSizeBytes: int64(4096 + index),
			CheckedAt: "2026-07-12 02:00:00", CheckedBy: 7, Note: "验收通过", Version: 1,
		})
	}
	snapshotJSON := mustSaaSReleaseCandidateSnapshot(t, evidence)
	store := &fakeSaaSReleaseReadinessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{7: {ID: 7, TenantID: 1, IsSuperAdmin: 1}}},
		evidence:           evidence,
		candidates: []SaaSReleaseCandidate{{
			ID: 8, CandidateNo: "REL-CURRENT", ReleaseVersion: "v0.9.9", SourceFingerprint: fingerprint,
			Status: SaaSReleaseCandidateStatusReady, RequiredCount: 6, PassedCount: 6, MatchedCount: 6, SnapshotJSON: snapshotJSON,
		}},
		updateResult: SaaSReleaseEvidenceUpdateResult{Evidence: SaaSReleaseEvidence{
			ID: 2, Key: "real_wecom", Title: "真实企业微信账号联调", Status: SaaSReleaseEvidenceStatusPassed, Version: 2,
		}, OperationID: 81},
		candidateResult: SaaSReleaseCandidateCreateResult{Candidate: SaaSReleaseCandidate{
			ID: 9, CandidateNo: "REL-TEST", ReleaseVersion: "v1.0.0", SourceFingerprint: fingerprint,
			Status: SaaSReleaseCandidateStatusReady, RequiredCount: 6, PassedCount: 6, MatchedCount: 6,
		}, OperationID: 82},
	}
	verifier := &fakeSaaSReleaseArtifactVerifier{failures: map[string]string{}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).
		WithReleaseSourceFingerprint(fingerprint, "build").
		WithReleaseEvidenceVerifier(verifier)

	getRequest := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/releaseReadiness", nil)
	getRequest.Header.Set("X-Mochat-Go-User-ID", "7")
	getRecorder := httptest.NewRecorder()
	handler.ReleaseReadiness(getRecorder, getRequest)
	if getRecorder.Code != http.StatusOK || !strings.Contains(getRecorder.Body.String(), `"ready":true`) ||
		!strings.Contains(getRecorder.Body.String(), `"latestCandidateEvidenceCurrent":true`) ||
		!strings.Contains(getRecorder.Body.String(), `"effectiveStatus":"ready"`) ||
		!strings.Contains(getRecorder.Body.String(), `"sourceFingerprintAuthoritative":true`) ||
		!strings.Contains(getRecorder.Body.String(), `"sourceFingerprintSource":"build"`) ||
		!strings.Contains(getRecorder.Body.String(), `"targetSourceFingerprint":"`+fingerprint+`"`) {
		t.Fatalf("GET status=%d body=%s", getRecorder.Code, getRecorder.Body.String())
	}

	updateBody, _ := json.Marshal(map[string]any{
		"key": "real_wecom", "status": "passed", "evidenceUrl": "https://evidence.company.cn/wecom/run-1",
		"environment": "production", "sourceFingerprint": fingerprint, "artifactSha256": artifactSHA256,
		"artifactSizeBytes": 4096, "note": "验收通过", "expectedVersion": 1,
	})
	updateRequest := httptest.NewRequest(http.MethodPut, "/dashboard/saasAdmin/releaseEvidence", bytes.NewReader(updateBody))
	updateRequest.Header.Set("X-Mochat-Go-User-ID", "7")
	updateRecorder := httptest.NewRecorder()
	handler.ReleaseEvidence(updateRecorder, updateRequest)
	if updateRecorder.Code != http.StatusOK || store.updateCalls != 1 {
		t.Fatalf("PUT status=%d calls=%d body=%s", updateRecorder.Code, store.updateCalls, updateRecorder.Body.String())
	}
	if store.lastUpdate.ActorUserID != 7 || store.lastUpdate.ActorTenantID != 1 || store.lastUpdate.SourceFingerprint != fingerprint || store.lastUpdate.ArtifactSHA256 != artifactSHA256 || store.lastUpdate.ArtifactSizeBytes != 4096 {
		t.Fatalf("update actor/input=%+v", store.lastUpdate)
	}
	if store.lastUpdate.ArtifactVerification == nil || !store.lastUpdate.ArtifactVerification.Verified {
		t.Fatalf("update verification=%+v", store.lastUpdate.ArtifactVerification)
	}

	candidateBody, _ := json.Marshal(map[string]any{"releaseVersion": "v1.0.0"})
	candidateRequest := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/releaseCandidate", bytes.NewReader(candidateBody))
	candidateRequest.Header.Set("X-Mochat-Go-User-ID", "7")
	candidateRecorder := httptest.NewRecorder()
	handler.ReleaseCandidate(candidateRecorder, candidateRequest)
	if candidateRecorder.Code != http.StatusOK || store.candidateCalls != 1 || store.lastCandidate.CandidateNo == "" {
		t.Fatalf("POST status=%d calls=%d input=%+v body=%s", candidateRecorder.Code, store.candidateCalls, store.lastCandidate, candidateRecorder.Body.String())
	}
	if store.lastCandidate.ActorUserID != 7 || store.lastCandidate.ActorTenantID != 1 || store.lastCandidate.SourceFingerprint != fingerprint {
		t.Fatalf("candidate actor=%+v", store.lastCandidate)
	}
	if len(store.lastCandidate.ArtifactVerifications) != SaaSReleaseEvidenceRequiredCount {
		t.Fatalf("candidate verifications=%+v", store.lastCandidate.ArtifactVerifications)
	}
	for key, verification := range store.lastCandidate.ArtifactVerifications {
		if !verification.Verified || verification.EvidenceKey != key || verification.EvidenceVersion != 1 {
			t.Fatalf("candidate verification %s=%+v", key, verification)
		}
	}
}

func TestSaaSReleaseEvidenceActionHandlerPreservesActorAndVersion(t *testing.T) {
	store := &fakeSaaSReleaseReadinessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{7: {ID: 7, TenantID: 1, IsSuperAdmin: 1}}},
		actionResult: SaaSReleaseEvidenceActionUpdateResult{
			Action:      SaaSReleaseEvidenceAction{ID: 2, Key: "real_wecom", Title: "真实企业微信账号联调", OwnerUserID: 9, OwnerActive: true, Version: 2},
			OperationID: 83,
		},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	body := `{"key":"real_wecom","ownerUserId":9,"dueAt":"2026-07-22T09:30","nextAction":"完成授权与回调联调","note":"发布运营跟进","expectedVersion":1}`
	request := httptest.NewRequest(http.MethodPut, "/dashboard/saasAdmin/releaseEvidenceAction", strings.NewReader(body))
	request.Header.Set("X-Mochat-Go-User-ID", "7")
	recorder := httptest.NewRecorder()
	handler.ReleaseEvidenceAction(recorder, request)
	if recorder.Code != http.StatusOK || store.actionCalls != 1 || !strings.Contains(recorder.Body.String(), `"operationId":83`) {
		t.Fatalf("status=%d calls=%d body=%s", recorder.Code, store.actionCalls, recorder.Body.String())
	}
	if store.lastAction.ActorUserID != 7 || store.lastAction.ActorTenantID != 1 || store.lastAction.OwnerUserID != 9 ||
		store.lastAction.DueAt != "2026-07-22 09:30:00" || store.lastAction.ExpectedVersion != 1 {
		t.Fatalf("action=%+v", store.lastAction)
	}
}

func TestSaaSReleaseReadinessInvalidatesReadyCandidateWhenEvidenceDrifts(t *testing.T) {
	fingerprint := strings.Repeat("a", 64)
	artifactSHA256 := strings.Repeat("b", 64)
	evidence := make([]SaaSReleaseEvidence, 0, SaaSReleaseEvidenceRequiredCount)
	for index, key := range []string{"mysql57_amd64", "real_wecom", "real_wechat_open", "real_saas_tenants", "production_frontend", "stability"} {
		evidence = append(evidence, SaaSReleaseEvidence{
			ID: int64(index + 1), Key: key, Required: true, Status: SaaSReleaseEvidenceStatusPassed,
			EvidenceURL: "https://evidence.company.cn/" + key, Environment: "production", SourceFingerprint: fingerprint,
			ArtifactSHA256: artifactSHA256, ArtifactSizeBytes: int64(index + 100), CheckedAt: "2026-07-14 10:00:00",
			CheckedBy: 1, Note: "验收通过", Version: 2,
		})
	}
	candidate := SaaSReleaseCandidate{
		ID: 1, CandidateNo: "REL-READY", ReleaseVersion: "v1.0.0", SourceFingerprint: fingerprint,
		Status: SaaSReleaseCandidateStatusReady, RequiredCount: 6, PassedCount: 6, MatchedCount: 6,
		SnapshotJSON: mustSaaSReleaseCandidateSnapshot(t, evidence),
	}
	verifier := SaaSReleaseEvidenceVerifierStatus{Configured: true}
	ready := saasReleaseReadinessSummaryPayload(evidence, []SaaSReleaseCandidate{candidate}, fingerprint, true, "build", verifier)
	if ready["ready"] != true || ready["latestCandidateEvidenceCurrent"] != true || ready["latestCandidateEffectiveStatus"] != SaaSReleaseCandidateStatusReady {
		t.Fatalf("ready summary=%+v", ready)
	}

	evidence[5].Version++
	evidence[5].Status = SaaSReleaseEvidenceStatusFailed
	evidence[5].Note = "目标环境回归失败"
	drifted := saasReleaseReadinessSummaryPayload(evidence, []SaaSReleaseCandidate{candidate}, fingerprint, true, "build", verifier)
	if drifted["ready"] != false || drifted["latestCandidateSnapshotValid"] != true ||
		drifted["latestCandidateEvidenceCurrent"] != false || drifted["latestCandidateEffectiveStatus"] != SaaSReleaseCandidateStatusStale {
		t.Fatalf("drifted summary=%+v", drifted)
	}
	driftedKeys, ok := drifted["latestCandidateDriftedEvidenceKeys"].([]string)
	if !ok || len(driftedKeys) != 1 || driftedKeys[0] != "stability" {
		t.Fatalf("drifted keys=%#v", drifted["latestCandidateDriftedEvidenceKeys"])
	}

	evidence[5].Status = SaaSReleaseEvidenceStatusPassed
	evidence[5].Note = "重新验收通过"
	restored := saasReleaseReadinessSummaryPayload(evidence, []SaaSReleaseCandidate{candidate}, fingerprint, true, "build", verifier)
	if restored["metadataReady"] != true || restored["ready"] != false || restored["latestCandidateEffectiveStatus"] != SaaSReleaseCandidateStatusStale {
		t.Fatalf("restored summary=%+v", restored)
	}
	payloads := saasReleaseCandidatesPayload([]SaaSReleaseCandidate{candidate}, evidence)
	if len(payloads) != 1 || payloads[0]["effectiveStatus"] != SaaSReleaseCandidateStatusStale || payloads[0]["evidenceCurrent"] != false {
		t.Fatalf("candidate payload=%+v", payloads)
	}

	candidate.SnapshotJSON = `{}`
	invalid := saasReleaseReadinessSummaryPayload(evidence, []SaaSReleaseCandidate{candidate}, fingerprint, true, "build", verifier)
	if invalid["ready"] != false || invalid["latestCandidateSnapshotValid"] != false || invalid["latestCandidateEffectiveStatus"] != SaaSReleaseCandidateStatusStale {
		t.Fatalf("invalid snapshot summary=%+v", invalid)
	}
}

func TestSaaSReleaseEvidenceHandlerRejectsRemoteArtifactFailureBeforeStore(t *testing.T) {
	fingerprint := strings.Repeat("a", 64)
	artifactSHA256 := strings.Repeat("b", 64)
	evidenceURL := "https://evidence.company.cn/releases/real_wecom"
	store := &fakeSaaSReleaseReadinessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}}},
		evidence:           []SaaSReleaseEvidence{{Key: "real_wecom", Required: true, Version: 1}},
	}
	verifier := &fakeSaaSReleaseArtifactVerifier{failures: map[string]string{evidenceURL: "远端工件 SHA-256 与预期不一致"}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).
		WithReleaseSourceFingerprint(fingerprint, "build").WithReleaseEvidenceVerifier(verifier)
	body, _ := json.Marshal(map[string]any{
		"key": "real_wecom", "status": "passed", "evidenceUrl": evidenceURL, "environment": "production",
		"sourceFingerprint": fingerprint, "artifactSha256": artifactSHA256, "artifactSizeBytes": 4096, "expectedVersion": 1,
	})
	request := httptest.NewRequest(http.MethodPut, "/dashboard/saasAdmin/releaseEvidence", bytes.NewReader(body))
	request.Header.Set("X-Mochat-Go-User-ID", "1")
	recorder := httptest.NewRecorder()
	handler.ReleaseEvidence(recorder, request)
	if recorder.Code != http.StatusUnprocessableEntity || store.updateCalls != 0 || !strings.Contains(recorder.Body.String(), "artifactVerification") {
		t.Fatalf("status=%d calls=%d body=%s", recorder.Code, store.updateCalls, recorder.Body.String())
	}
}

func TestSaaSReleaseCandidatePersistsBlockedRemoteVerificationSnapshot(t *testing.T) {
	fingerprint := strings.Repeat("a", 64)
	artifactSHA256 := strings.Repeat("b", 64)
	evidence := make([]SaaSReleaseEvidence, 0, SaaSReleaseEvidenceRequiredCount)
	failureURL := "https://evidence.company.cn/stability"
	for index, key := range []string{"mysql57_amd64", "real_wecom", "real_wechat_open", "real_saas_tenants", "production_frontend", "stability"} {
		evidence = append(evidence, SaaSReleaseEvidence{
			Key: key, Required: true, Status: SaaSReleaseEvidenceStatusPassed, EvidenceURL: "https://evidence.company.cn/" + key,
			Environment: "production", SourceFingerprint: fingerprint, ArtifactSHA256: artifactSHA256,
			ArtifactSizeBytes: int64(index + 10), CheckedAt: "2026-07-14 10:00:00", CheckedBy: 1, Version: 1,
		})
	}
	store := &fakeSaaSReleaseReadinessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}}},
		evidence:           evidence,
		candidateResult: SaaSReleaseCandidateCreateResult{Candidate: SaaSReleaseCandidate{
			CandidateNo: "REL-BLOCKED", Status: SaaSReleaseCandidateStatusBlocked, RequiredCount: 6, PassedCount: 5, MatchedCount: 5,
		}},
	}
	verifier := &fakeSaaSReleaseArtifactVerifier{failures: map[string]string{failureURL: "remote unavailable"}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).
		WithReleaseSourceFingerprint(fingerprint, "build").WithReleaseEvidenceVerifier(verifier)
	request := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/releaseCandidate", strings.NewReader(`{"releaseVersion":"v1.0.0"}`))
	request.Header.Set("X-Mochat-Go-User-ID", "1")
	recorder := httptest.NewRecorder()
	handler.ReleaseCandidate(recorder, request)
	if recorder.Code != http.StatusOK || store.candidateCalls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", recorder.Code, store.candidateCalls, recorder.Body.String())
	}
	verification := store.lastCandidate.ArtifactVerifications["stability"]
	if verification.Verified || verification.Error != "remote unavailable" {
		t.Fatalf("verification=%+v", verification)
	}
}

func TestSaaSReleaseCandidateDirectCallRequiresApproval(t *testing.T) {
	fingerprint := strings.Repeat("a", 64)
	store := &fakeSaaSReleaseReadinessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).
		WithHighRiskApprovalRequired(true).
		WithReleaseSourceFingerprint(fingerprint, "build").
		WithReleaseEvidenceVerifier(&fakeSaaSReleaseArtifactVerifier{failures: map[string]string{}})
	request := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/releaseCandidate", strings.NewReader(`{"releaseVersion":"v1.0.0"}`))
	request.Header.Set("X-Mochat-Go-User-ID", "1")
	recorder := httptest.NewRecorder()
	handler.ReleaseCandidate(recorder, request)
	if recorder.Code != http.StatusPreconditionRequired || store.candidateCalls != 0 ||
		!strings.Contains(recorder.Body.String(), `"actionType":"release.candidate.gate"`) || !strings.Contains(recorder.Body.String(), `"requiredApprovals":2`) {
		t.Fatalf("status=%d calls=%d body=%s", recorder.Code, store.candidateCalls, recorder.Body.String())
	}
}

func TestSaaSReleaseReadinessRequiresAuthoritativeRuntimeFingerprint(t *testing.T) {
	fingerprint := strings.Repeat("a", 64)
	artifactSHA256 := strings.Repeat("d", 64)
	store := &fakeSaaSReleaseReadinessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)

	getRequest := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/releaseReadiness?sourceFingerprint="+fingerprint, nil)
	getRequest.Header.Set("X-Mochat-Go-User-ID", "1")
	getRecorder := httptest.NewRecorder()
	handler.ReleaseReadiness(getRecorder, getRequest)
	if getRecorder.Code != http.StatusOK || !strings.Contains(getRecorder.Body.String(), `"sourceFingerprintAuthoritative":false`) ||
		!strings.Contains(getRecorder.Body.String(), `"candidateGateEnabled":false`) || strings.Contains(getRecorder.Body.String(), `"ready":true`) {
		t.Fatalf("GET status=%d body=%s", getRecorder.Code, getRecorder.Body.String())
	}

	candidateRequest := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/releaseCandidate", strings.NewReader(`{"releaseVersion":"v1.0.0"}`))
	candidateRequest.Header.Set("X-Mochat-Go-User-ID", "1")
	candidateRecorder := httptest.NewRecorder()
	handler.ReleaseCandidate(candidateRecorder, candidateRequest)
	if candidateRecorder.Code != http.StatusServiceUnavailable || store.candidateCalls != 0 {
		t.Fatalf("candidate status=%d calls=%d body=%s", candidateRecorder.Code, store.candidateCalls, candidateRecorder.Body.String())
	}

	evidenceBody, _ := json.Marshal(map[string]any{
		"key": "real_wecom", "status": "passed", "evidenceUrl": "https://evidence.company.cn/wecom/run-1",
		"environment": "production", "sourceFingerprint": fingerprint, "artifactSha256": artifactSHA256,
		"artifactSizeBytes": 4096, "expectedVersion": 1,
	})
	evidenceRequest := httptest.NewRequest(http.MethodPut, "/dashboard/saasAdmin/releaseEvidence", bytes.NewReader(evidenceBody))
	evidenceRequest.Header.Set("X-Mochat-Go-User-ID", "1")
	evidenceRecorder := httptest.NewRecorder()
	handler.ReleaseEvidence(evidenceRecorder, evidenceRequest)
	if evidenceRecorder.Code != http.StatusServiceUnavailable || store.updateCalls != 0 {
		t.Fatalf("evidence status=%d calls=%d body=%s", evidenceRecorder.Code, store.updateCalls, evidenceRecorder.Body.String())
	}
}

func TestSaaSReleaseReadinessRejectsFingerprintMismatchBeforeStore(t *testing.T) {
	runtimeFingerprint := strings.Repeat("b", 64)
	requestFingerprint := strings.Repeat("c", 64)
	artifactSHA256 := strings.Repeat("d", 64)
	store := &fakeSaaSReleaseReadinessStore{
		fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}}},
	}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).WithReleaseSourceFingerprint(runtimeFingerprint, "build")

	getRequest := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/releaseReadiness?sourceFingerprint="+requestFingerprint, nil)
	getRequest.Header.Set("X-Mochat-Go-User-ID", "1")
	getRecorder := httptest.NewRecorder()
	handler.ReleaseReadiness(getRecorder, getRequest)
	if getRecorder.Code != http.StatusConflict {
		t.Fatalf("GET status=%d body=%s", getRecorder.Code, getRecorder.Body.String())
	}

	evidenceBody, _ := json.Marshal(map[string]any{
		"key": "real_wecom", "status": "passed", "evidenceUrl": "https://evidence.company.cn/wecom/run-1",
		"environment": "production", "sourceFingerprint": requestFingerprint, "artifactSha256": artifactSHA256,
		"artifactSizeBytes": 4096, "expectedVersion": 1,
	})
	evidenceRequest := httptest.NewRequest(http.MethodPut, "/dashboard/saasAdmin/releaseEvidence", bytes.NewReader(evidenceBody))
	evidenceRequest.Header.Set("X-Mochat-Go-User-ID", "1")
	evidenceRecorder := httptest.NewRecorder()
	handler.ReleaseEvidence(evidenceRecorder, evidenceRequest)
	if evidenceRecorder.Code != http.StatusConflict || store.updateCalls != 0 {
		t.Fatalf("evidence status=%d calls=%d body=%s", evidenceRecorder.Code, store.updateCalls, evidenceRecorder.Body.String())
	}

	candidateBody, _ := json.Marshal(map[string]any{"releaseVersion": "v1.0.0", "sourceFingerprint": requestFingerprint})
	candidateRequest := httptest.NewRequest(http.MethodPost, "/dashboard/saasAdmin/releaseCandidate", bytes.NewReader(candidateBody))
	candidateRequest.Header.Set("X-Mochat-Go-User-ID", "1")
	candidateRecorder := httptest.NewRecorder()
	handler.ReleaseCandidate(candidateRecorder, candidateRequest)
	if candidateRecorder.Code != http.StatusConflict || store.candidateCalls != 0 {
		t.Fatalf("candidate status=%d calls=%d body=%s", candidateRecorder.Code, store.candidateCalls, candidateRecorder.Body.String())
	}
}

func TestSaaSReleaseEvidenceHandlerRejectsUnknownAndUnsafeInputBeforeStore(t *testing.T) {
	store := &fakeSaaSReleaseReadinessStore{fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{1: {ID: 1, TenantID: 1, IsSuperAdmin: 1}}}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	body := `{"key":"real_wecom","status":"passed","evidenceUrl":"https://localhost/run","environment":"production","sourceFingerprint":"` + strings.Repeat("a", 64) + `","expectedVersion":1}`
	request := httptest.NewRequest(http.MethodPut, "/dashboard/saasAdmin/releaseEvidence", strings.NewReader(body))
	request.Header.Set("X-Mochat-Go-User-ID", "1")
	recorder := httptest.NewRecorder()
	handler.ReleaseEvidence(recorder, request)
	if recorder.Code != http.StatusBadRequest || store.updateCalls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", recorder.Code, store.updateCalls, recorder.Body.String())
	}
}

func TestSaaSAdminReleasePermissionRouting(t *testing.T) {
	if got := SaaSAdminRequiredPermission(httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/releaseReadiness", nil)); got != SaaSAdminPermissionReleaseRead {
		t.Fatalf("read permission=%q", got)
	}
	if got := SaaSAdminRequiredPermission(httptest.NewRequest(http.MethodPut, "/dashboard/saasAdmin/releaseEvidence", nil)); got != SaaSAdminPermissionReleaseManage {
		t.Fatalf("manage permission=%q", got)
	}
	if got := SaaSAdminRequiredPermission(httptest.NewRequest(http.MethodPut, "/dashboard/saasAdmin/releaseEvidenceAction", nil)); got != SaaSAdminPermissionReleaseManage {
		t.Fatalf("action permission=%q", got)
	}
}
