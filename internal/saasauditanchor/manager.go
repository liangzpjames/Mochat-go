package saasauditanchor

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"jiyi/mochat-go/internal/saasbackup"
)

var keyIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

type Manager struct {
	store     Store
	config    Config
	keys      map[string][]byte
	activeKey []byte
	remote    RemoteArtifactStore
	clock     func() time.Time
}

type signedPayload struct {
	Schema             string `json:"schema"`
	CheckpointNo       string `json:"checkpointNo"`
	TenantID           int    `json:"tenantId"`
	ChainVersion       int    `json:"chainVersion"`
	AnchorLogID        int64  `json:"anchorLogId"`
	AnchorHash         string `json:"anchorHash"`
	LegacyLogCount     int64  `json:"legacyLogCount"`
	ChainHeadLogID     int64  `json:"chainHeadLogId"`
	ChainHeadHash      string `json:"chainHeadHash"`
	SignedLogCount     int64  `json:"signedLogCount"`
	SignatureAlgorithm string `json:"signatureAlgorithm"`
	KeyID              string `json:"keyId"`
	SignedAt           string `json:"signedAt"`
}

type artifactEnvelope struct {
	Payload       signedPayload `json:"payload"`
	PayloadSHA256 string        `json:"payloadSha256"`
	Signature     string        `json:"signature"`
}

func NewManager(store Store, config Config) (*Manager, error) {
	if store == nil {
		return nil, fmt.Errorf("audit anchor store is required")
	}
	config.ArtifactRoot = strings.TrimSpace(config.ArtifactRoot)
	config.HMACKeyID = strings.TrimSpace(config.HMACKeyID)
	if config.ArtifactRoot == "" {
		config.ArtifactRoot = "./storage/audit-anchors"
	}
	if config.HMACKeyID == "" {
		config.HMACKeyID = "primary"
	}
	if !keyIDPattern.MatchString(config.HMACKeyID) {
		return nil, fmt.Errorf("audit anchor HMAC key id %q is invalid", config.HMACKeyID)
	}
	keys, err := saasbackup.ParseEncryptionKeyRing(config.HMACKeys)
	if err != nil {
		return nil, rewriteKeyError(err)
	}
	legacy, err := saasbackup.ParseEncryptionKey(config.HMACKey)
	if err != nil {
		return nil, rewriteKeyError(err)
	}
	if len(legacy) == 32 {
		if existing, ok := keys[config.HMACKeyID]; ok && !bytes.Equal(existing, legacy) {
			return nil, fmt.Errorf("active audit anchor HMAC key %q conflicts with the key ring", config.HMACKeyID)
		}
		keys[config.HMACKeyID] = legacy
	}
	active := keys[config.HMACKeyID]
	if len(keys) > 0 && len(active) != 32 {
		return nil, fmt.Errorf("active audit anchor HMAC key %q is missing from the key ring", config.HMACKeyID)
	}
	return &Manager{
		store: store, config: config, keys: keys, activeKey: active, remote: config.Remote, clock: time.Now,
	}, nil
}

func rewriteKeyError(err error) error {
	if err == nil {
		return nil
	}
	message := strings.ReplaceAll(err.Error(), "backup encryption", "audit anchor HMAC")
	return errors.New(message)
}

func (m *Manager) ConfigStatus() ConfigStatus {
	status := ConfigStatus{
		ArtifactRoot: m.config.ArtifactRoot, ArtifactRootConfigured: strings.TrimSpace(m.config.ArtifactRoot) != "",
		HMACConfigured: len(m.activeKey) == 32, HMACKeyID: m.config.HMACKeyID, HMACKeyCount: len(m.keys),
		MissingKeyIDs: []string{}, IndependentStorageRequired: true, RemoteRequired: m.config.RequireRemote,
	}
	if m.remote != nil {
		status.RemoteConfigured = true
		status.RemoteProvider = m.remote.Provider()
		status.RemoteBucket = m.remote.Bucket()
		status.RemotePrefix = m.remote.Prefix()
		status.RemoteRetentionMode = m.remote.RetentionMode()
		status.RemoteRetentionDays = m.remote.RetentionDays()
	}
	return status
}

func (m *Manager) Overview(ctx context.Context, tenantID, limit int) (Overview, error) {
	if tenantID < 0 || limit < 1 || limit > 500 {
		return Overview{}, Invalid("tenantId 必须是非负整数，limit 必须在 1 至 500 之间")
	}
	summary, err := m.store.AuditAnchorCheckpointSummary(ctx, tenantID)
	if err != nil {
		return Overview{}, err
	}
	items, err := m.store.AuditAnchorCheckpoints(ctx, tenantID, limit)
	if err != nil {
		return Overview{}, err
	}
	status := m.ConfigStatus()
	checkpointNos, err := m.store.AuditAnchorCheckpointNos(ctx)
	if err != nil {
		return Overview{}, err
	}
	summary.OrphanArtifactCount, err = m.orphanArtifactCount(checkpointNos)
	if err != nil {
		return Overview{}, err
	}
	summary.OrphanRemoteCount, err = m.orphanRemoteArtifactCount(ctx)
	if err != nil {
		return Overview{}, err
	}
	keyIDs, err := m.store.AuditAnchorCheckpointKeyIDs(ctx)
	if err != nil {
		return Overview{}, err
	}
	for _, keyID := range keyIDs {
		if len(m.keys[keyID]) != 32 {
			status.MissingKeyIDs = append(status.MissingKeyIDs, keyID)
		}
	}
	sort.Strings(status.MissingKeyIDs)
	return Overview{Config: status, Summary: summary, Checkpoints: items}, nil
}

func (m *Manager) Create(ctx context.Context, options CreateOptions) (CreateResult, error) {
	if options.TenantID < 0 || options.Limit < 1 || options.Limit > 500 {
		return CreateResult{}, Invalid("tenantId 必须是非负整数，limit 必须在 1 至 500 之间")
	}
	if len(m.activeKey) != 32 {
		return CreateResult{}, Unavailable("审计锚点 HMAC 主密钥未配置")
	}
	options.Source = normalizeSource(options.Source)
	verification, err := m.store.VerifyAuditAnchorChains(ctx, options)
	if err != nil {
		return CreateResult{}, err
	}
	if verification.FailedChains > 0 {
		return CreateResult{}, Conflict(fmt.Sprintf("审计摘要链存在 %d 条异常，已拒绝创建签名锚点", verification.FailedChains))
	}
	result := CreateResult{ScannedChains: verification.ScannedChains, Checkpoints: []Checkpoint{}}
	for _, chain := range verification.Chains {
		item, created, err := m.createCheckpoint(ctx, chain, options)
		if err != nil {
			result.FailedArtifacts++
			return result, err
		}
		if created {
			result.CreatedCheckpoints++
		} else {
			result.ExistingCheckpoints++
		}
		if item.ArtifactStatus == ArtifactStatusExported {
			result.ExportedArtifacts++
		} else {
			result.FailedArtifacts++
		}
		if item.RemoteStatus == RemoteStatusExported {
			result.RemoteExportedArtifacts++
		} else if item.RemoteStatus == RemoteStatusFailed {
			result.RemoteFailedArtifacts++
		}
		result.Checkpoints = append(result.Checkpoints, item)
	}
	if err := m.backfillRemoteArtifacts(ctx, options, &result); err != nil {
		return result, err
	}
	result.CreatedAt = m.clock().UTC().Truncate(time.Second).Format(time.RFC3339)
	result.OperationID, err = m.store.RecordAuditAnchorOperation(ctx, OperationRecord{
		Action: OperationActionCreate, TenantID: options.TenantID, Source: options.Source,
		Affected: len(result.Checkpoints), Failed: result.FailedArtifacts, Actor: options.Actor,
	})
	if err != nil {
		return result, err
	}
	return result, nil
}

func (m *Manager) backfillRemoteArtifacts(ctx context.Context, options CreateOptions, result *CreateResult) error {
	if m.remote == nil {
		return nil
	}
	items, err := m.store.AuditAnchorCheckpointsPendingRemote(ctx, options.TenantID, options.Limit)
	if err != nil {
		return err
	}
	for _, item := range items {
		updated, ensureErr := m.ensureArtifact(ctx, item)
		if ensureErr != nil {
			result.FailedArtifacts++
			if updated.RemoteStatus == RemoteStatusFailed {
				result.RemoteFailedArtifacts++
			}
			return ensureErr
		}
		result.BackfilledCheckpoints++
		if updated.ArtifactStatus == ArtifactStatusExported {
			result.ExportedArtifacts++
		} else {
			result.FailedArtifacts++
		}
		if updated.RemoteStatus == RemoteStatusExported {
			result.RemoteExportedArtifacts++
		} else if updated.RemoteStatus == RemoteStatusFailed {
			result.RemoteFailedArtifacts++
		}
		result.Checkpoints = append(result.Checkpoints, updated)
	}
	return nil
}

func (m *Manager) createCheckpoint(ctx context.Context, chain Chain, options CreateOptions) (Checkpoint, bool, error) {
	fingerprint := chainFingerprint(chain, m.config.HMACKeyID)
	if existing, ok, err := m.store.AuditAnchorCheckpointByFingerprint(ctx, fingerprint); err != nil {
		return Checkpoint{}, false, err
	} else if ok {
		item, err := m.ensureArtifact(ctx, existing)
		return item, false, err
	}
	signedAt := m.clock().UTC().Truncate(time.Second).Format(time.RFC3339)
	checkpointNo := "AAN-" + strings.ToUpper(fingerprint[:24])
	item := Checkpoint{
		CheckpointNo: checkpointNo, Fingerprint: fingerprint, TenantID: chain.TenantID,
		TenantName: chain.TenantName, ChainVersion: chain.ChainVersion, AnchorLogID: chain.AnchorLogID,
		AnchorHash: chain.AnchorHash, LegacyLogCount: chain.LegacyLogCount, ChainHeadLogID: chain.ChainHeadLogID,
		ChainHeadHash: chain.ChainHeadHash, SignedLogCount: chain.SignedLogCount,
		SignatureAlgorithm: SignatureAlgorithm, KeyID: m.config.HMACKeyID,
		ArtifactName: checkpointNo + ".json", ArtifactStatus: ArtifactStatusPending,
		RemoteStatus:       m.initialRemoteStatus(),
		VerificationStatus: VerifyStatusPending, Source: options.Source,
		ActorUserID: options.Actor.UserID, ActorTenantID: options.Actor.TenantID, SignedAt: signedAt,
	}
	payload, err := checkpointPayload(item)
	if err != nil {
		return Checkpoint{}, false, err
	}
	item.PayloadSHA256 = sha256Hex(payload)
	item.Signature = hmacHex(m.activeKey, payload)
	item, created, err := m.store.CreateAuditAnchorCheckpoint(ctx, CheckpointCreate{Checkpoint: item})
	if err != nil {
		return Checkpoint{}, false, err
	}
	item, err = m.ensureArtifact(ctx, item)
	return item, created, err
}

func (m *Manager) ensureArtifact(ctx context.Context, item Checkpoint) (Checkpoint, error) {
	key := m.keys[item.KeyID]
	if len(key) != 32 {
		return m.markArtifactFailed(ctx, item, fmt.Sprintf("签名密钥 %s 不可用", item.KeyID))
	}
	if err := verifyCheckpointSignature(item, key); err != nil {
		return m.markArtifactFailed(ctx, item, err.Error())
	}
	encoded, err := checkpointArtifact(item)
	if err != nil {
		return item, err
	}
	name := strings.TrimSpace(item.ArtifactName)
	path, err := safeArtifactPath(m.config.ArtifactRoot, name)
	if err != nil {
		return m.markArtifactFailed(ctx, item, err.Error())
	}
	if err := writeImmutableFile(path, encoded); err != nil {
		return m.markArtifactFailed(ctx, item, err.Error())
	}
	updated, err := m.store.UpdateAuditAnchorArtifact(ctx, ArtifactUpdate{
		ID: item.ID, Status: ArtifactStatusExported, ArtifactName: name,
		ArtifactSHA256: sha256Hex(encoded), ExportedAt: m.clock().UTC().Truncate(time.Second),
	})
	if err != nil {
		return updated, err
	}
	return m.ensureRemoteArtifact(ctx, updated, encoded)
}

func (m *Manager) initialRemoteStatus() string {
	if m.remote != nil || m.config.RequireRemote {
		return RemoteStatusPending
	}
	return RemoteStatusDisabled
}

func (m *Manager) ensureRemoteArtifact(ctx context.Context, item Checkpoint, encoded []byte) (Checkpoint, error) {
	if m.remote == nil {
		if m.config.RequireRemote || item.RemoteStatus != "" && item.RemoteStatus != RemoteStatusDisabled {
			return m.markRemoteFailed(ctx, item, "审计锚点远端不可变存储未配置")
		}
		return item, nil
	}
	signedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(item.SignedAt))
	if err != nil {
		return m.markRemoteFailed(ctx, item, "审计锚点签名时间无效")
	}
	remote, err := m.remote.Ensure(
		ctx, item.ArtifactName, encoded, item.ArtifactSHA256, signedAt, remoteArtifactFromCheckpoint(item),
	)
	if err != nil {
		return m.markRemoteFailed(ctx, item, err.Error())
	}
	updated, err := m.store.UpdateAuditAnchorRemoteArtifact(ctx, RemoteArtifactUpdate{
		ID: item.ID, Status: RemoteStatusExported, Artifact: remote,
	})
	return updated, err
}

func (m *Manager) markRemoteFailed(ctx context.Context, item Checkpoint, message string) (Checkpoint, error) {
	updated, err := m.store.UpdateAuditAnchorRemoteArtifact(ctx, RemoteArtifactUpdate{
		ID: item.ID, Status: RemoteStatusFailed, Artifact: remoteArtifactFromCheckpoint(item),
		ErrorMessage: truncate(message, 255),
	})
	if err != nil {
		return item, err
	}
	return updated, Unavailable(message)
}

func (m *Manager) markArtifactFailed(ctx context.Context, item Checkpoint, message string) (Checkpoint, error) {
	updated, err := m.store.UpdateAuditAnchorArtifact(ctx, ArtifactUpdate{
		ID: item.ID, Status: ArtifactStatusFailed, ArtifactName: item.ArtifactName,
		ErrorMessage: truncate(message, 255),
	})
	if err != nil {
		return item, err
	}
	return updated, Unavailable(message)
}

func (m *Manager) Verify(ctx context.Context, options VerifyOptions) (VerifyResult, error) {
	if options.TenantID < 0 || options.Limit < 1 || options.Limit > 500 {
		return VerifyResult{}, Invalid("tenantId 必须是非负整数，limit 必须在 1 至 500 之间")
	}
	options.Source = normalizeSource(options.Source)
	items, err := m.store.AuditAnchorCheckpoints(ctx, options.TenantID, options.Limit)
	if err != nil {
		return VerifyResult{}, err
	}
	now := m.clock().UTC().Truncate(time.Second)
	result := VerifyResult{ScannedCheckpoints: len(items), VerifiedAt: now.Format(time.RFC3339), Checkpoints: []Checkpoint{}}
	for _, item := range items {
		verification := m.verifyCheckpoint(ctx, item)
		status := VerifyStatusPassed
		if verification.message != "" {
			status = VerifyStatusFailed
			result.FailedCheckpoints++
			if verification.missingKey {
				result.MissingKeyCount++
			}
			if verification.missingArtifact {
				result.MissingArtifactCount++
			}
			if verification.missingRemote {
				result.MissingRemoteCount++
			}
		} else {
			result.PassedCheckpoints++
		}
		updated, updateErr := m.store.UpdateAuditAnchorVerification(ctx, VerificationUpdate{
			ID: item.ID, Status: status, ErrorMessage: truncate(verification.message, 255), VerifiedAt: now,
		})
		if updateErr != nil {
			return result, updateErr
		}
		result.Checkpoints = append(result.Checkpoints, updated)
	}
	checkpointNos, err := m.store.AuditAnchorCheckpointNos(ctx)
	if err != nil {
		return result, err
	}
	result.OrphanArtifactCount, err = m.orphanArtifactCount(checkpointNos)
	if err != nil {
		return result, err
	}
	result.FailedCheckpoints += result.OrphanArtifactCount
	result.OrphanRemoteCount, err = m.orphanRemoteArtifactCount(ctx)
	if err != nil {
		return result, err
	}
	result.FailedCheckpoints += result.OrphanRemoteCount
	result.OperationID, err = m.store.RecordAuditAnchorOperation(ctx, OperationRecord{
		Action: OperationActionVerify, TenantID: options.TenantID, Source: options.Source,
		Affected: result.ScannedCheckpoints, Failed: result.FailedCheckpoints, Actor: options.Actor,
	})
	if err != nil {
		return result, err
	}
	return result, nil
}

func (m *Manager) orphanArtifactCount(checkpointNos []string) (int, error) {
	known := make(map[string]struct{}, len(checkpointNos))
	for _, value := range checkpointNos {
		known[strings.TrimSpace(value)+".json"] = struct{}{}
	}
	entries, err := os.ReadDir(m.config.ArtifactRoot)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	orphans := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "AAN-") || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if _, ok := known[entry.Name()]; !ok {
			orphans++
		}
	}
	return orphans, nil
}

func (m *Manager) orphanRemoteArtifactCount(ctx context.Context) (int, error) {
	if m.remote == nil {
		return 0, nil
	}
	knownItems, err := m.store.AuditAnchorRemoteObjectKeys(ctx)
	if err != nil {
		return 0, err
	}
	known := make(map[string]struct{}, len(knownItems))
	for _, item := range knownItems {
		if item = strings.TrimSpace(item); item != "" {
			known[item] = struct{}{}
		}
	}
	actual, err := m.remote.ListObjectKeys(ctx)
	if err != nil {
		return 0, err
	}
	orphans := 0
	for _, item := range actual {
		if _, ok := known[strings.TrimSpace(item)]; !ok {
			orphans++
		}
	}
	return orphans, nil
}

type checkpointVerification struct {
	message         string
	missingKey      bool
	missingArtifact bool
	missingRemote   bool
}

func (m *Manager) verifyCheckpoint(ctx context.Context, item Checkpoint) checkpointVerification {
	key := m.keys[item.KeyID]
	if len(key) != 32 {
		return checkpointVerification{message: fmt.Sprintf("签名密钥 %s 不可用", item.KeyID), missingKey: true}
	}
	if err := verifyCheckpointSignature(item, key); err != nil {
		return checkpointVerification{message: err.Error()}
	}
	encoded, err := checkpointArtifact(item)
	if err != nil {
		return checkpointVerification{message: err.Error()}
	}
	reasons := []string{}
	result := checkpointVerification{}
	path, err := safeArtifactPath(m.config.ArtifactRoot, item.ArtifactName)
	if err != nil {
		return checkpointVerification{message: err.Error()}
	}
	actual, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		result.missingArtifact = true
		reasons = append(reasons, "独立审计锚点证据文件不存在")
	} else if err != nil {
		reasons = append(reasons, fmt.Sprintf("读取审计锚点证据失败: %v", err))
	} else if !hmac.Equal([]byte(sha256Hex(actual)), []byte(item.ArtifactSHA256)) || !bytes.Equal(actual, encoded) {
		reasons = append(reasons, "独立审计锚点证据内容或摘要不匹配")
	}
	if err := m.store.CheckAuditAnchorCheckpointChain(ctx, item); err != nil {
		reasons = append(reasons, err.Error())
	}
	remoteMessage, missingRemote := m.verifyRemoteArtifact(ctx, item, encoded)
	if remoteMessage != "" {
		reasons = append(reasons, remoteMessage)
		result.missingRemote = missingRemote
	}
	result.message = strings.Join(reasons, "; ")
	return result
}

func (m *Manager) verifyRemoteArtifact(ctx context.Context, item Checkpoint, encoded []byte) (string, bool) {
	if m.remote == nil {
		if m.config.RequireRemote || item.RemoteStatus != "" && item.RemoteStatus != RemoteStatusDisabled {
			return "审计锚点远端不可变存储未配置", true
		}
		return "", false
	}
	if strings.TrimSpace(item.RemoteObjectKey) == "" {
		_, _ = m.store.UpdateAuditAnchorRemoteArtifact(ctx, RemoteArtifactUpdate{
			ID: item.ID, Status: RemoteStatusFailed, Artifact: remoteArtifactFromCheckpoint(item),
			ErrorMessage: "审计锚点远端不可变证据不存在",
		})
		return "审计锚点远端不可变证据不存在", true
	}
	remote, err := m.remote.Verify(ctx, remoteArtifactFromCheckpoint(item), encoded, item.ArtifactSHA256)
	if err != nil {
		message := err.Error()
		_, updateErr := m.store.UpdateAuditAnchorRemoteArtifact(ctx, RemoteArtifactUpdate{
			ID: item.ID, Status: RemoteStatusFailed, Artifact: remoteArtifactFromCheckpoint(item),
			ErrorMessage: truncate(message, 255),
		})
		if updateErr != nil {
			message += "; 更新远端校验状态失败: " + updateErr.Error()
		}
		return message, errors.Is(err, ErrRemoteArtifactNotFound)
	}
	if _, err := m.store.UpdateAuditAnchorRemoteArtifact(ctx, RemoteArtifactUpdate{
		ID: item.ID, Status: RemoteStatusExported, Artifact: remote,
	}); err != nil {
		return "更新远端校验状态失败: " + err.Error(), false
	}
	return "", false
}

func remoteArtifactFromCheckpoint(item Checkpoint) RemoteArtifact {
	exportedAt, _ := time.Parse(time.RFC3339, strings.TrimSpace(item.RemoteExportedAt))
	verifiedAt, _ := time.Parse(time.RFC3339, strings.TrimSpace(item.RemoteVerifiedAt))
	return RemoteArtifact{
		Provider: item.RemoteProvider, Bucket: item.RemoteBucket, ObjectKey: item.RemoteObjectKey,
		ETag: item.RemoteETag, VersionID: item.RemoteVersionID, SHA256: item.RemoteSHA256,
		SizeBytes: item.RemoteSizeBytes, RetentionMode: item.RemoteRetentionMode,
		RetainUntil: item.RemoteRetainUntil, ExportedAt: exportedAt, VerifiedAt: verifiedAt,
	}
}

func (m *Manager) CheckConfiguration(ctx context.Context) error {
	if len(m.activeKey) != 32 {
		return fmt.Errorf("active audit anchor HMAC key %q is missing", m.config.HMACKeyID)
	}
	keyIDs, err := m.store.AuditAnchorCheckpointKeyIDs(ctx)
	if err != nil {
		return err
	}
	missing := make([]string, 0)
	for _, keyID := range keyIDs {
		if len(m.keys[keyID]) != 32 {
			missing = append(missing, keyID)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("missing audit anchor HMAC keys: %s", strings.Join(missing, ", "))
	}
	remoteKeys, err := m.store.AuditAnchorRemoteObjectKeys(ctx)
	if err != nil {
		return err
	}
	if m.remote == nil && (m.config.RequireRemote || len(remoteKeys) > 0) {
		return errors.New("audit anchor remote immutable store is required but not configured")
	}
	root, err := filepath.Abs(m.config.ArtifactRoot)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return fmt.Errorf("create audit anchor artifact root: %w", err)
	}
	probe, err := os.CreateTemp(root, ".health-*")
	if err != nil {
		return fmt.Errorf("audit anchor artifact root is not writable: %w", err)
	}
	name := probe.Name()
	if closeErr := probe.Close(); closeErr != nil {
		os.Remove(name)
		return closeErr
	}
	return os.Remove(name)
}

func (m *Manager) ProbeRemote(ctx context.Context) error {
	if m.remote == nil {
		return Unavailable("审计锚点远端不可变存储未配置")
	}
	return m.remote.Probe(ctx)
}

func checkpointPayload(item Checkpoint) ([]byte, error) {
	return json.Marshal(signedPayload{
		Schema: SchemaVersion, CheckpointNo: item.CheckpointNo, TenantID: item.TenantID,
		ChainVersion: item.ChainVersion, AnchorLogID: item.AnchorLogID, AnchorHash: item.AnchorHash,
		LegacyLogCount: item.LegacyLogCount, ChainHeadLogID: item.ChainHeadLogID,
		ChainHeadHash: item.ChainHeadHash, SignedLogCount: item.SignedLogCount,
		SignatureAlgorithm: item.SignatureAlgorithm, KeyID: item.KeyID, SignedAt: item.SignedAt,
	})
}

func checkpointArtifact(item Checkpoint) ([]byte, error) {
	payload := signedPayload{
		Schema: SchemaVersion, CheckpointNo: item.CheckpointNo, TenantID: item.TenantID,
		ChainVersion: item.ChainVersion, AnchorLogID: item.AnchorLogID, AnchorHash: item.AnchorHash,
		LegacyLogCount: item.LegacyLogCount, ChainHeadLogID: item.ChainHeadLogID,
		ChainHeadHash: item.ChainHeadHash, SignedLogCount: item.SignedLogCount,
		SignatureAlgorithm: item.SignatureAlgorithm, KeyID: item.KeyID, SignedAt: item.SignedAt,
	}
	encoded, err := json.MarshalIndent(artifactEnvelope{Payload: payload, PayloadSHA256: item.PayloadSHA256, Signature: item.Signature}, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

func verifyCheckpointSignature(item Checkpoint, key []byte) error {
	if item.SignatureAlgorithm != SignatureAlgorithm {
		return fmt.Errorf("不支持的签名算法 %s", item.SignatureAlgorithm)
	}
	payload, err := checkpointPayload(item)
	if err != nil {
		return err
	}
	if !hmac.Equal([]byte(sha256Hex(payload)), []byte(strings.ToLower(item.PayloadSHA256))) {
		return errors.New("审计锚点载荷摘要不匹配")
	}
	expected, err := hex.DecodeString(hmacHex(key, payload))
	if err != nil {
		return err
	}
	actual, err := hex.DecodeString(strings.TrimSpace(item.Signature))
	if err != nil || !hmac.Equal(expected, actual) {
		return errors.New("审计锚点 HMAC 签名不匹配")
	}
	return nil
}

func chainFingerprint(chain Chain, keyID string) string {
	encoded, _ := json.Marshal(struct {
		Schema         string `json:"schema"`
		TenantID       int    `json:"tenantId"`
		ChainVersion   int    `json:"chainVersion"`
		AnchorLogID    int64  `json:"anchorLogId"`
		AnchorHash     string `json:"anchorHash"`
		LegacyLogCount int64  `json:"legacyLogCount"`
		ChainHeadLogID int64  `json:"chainHeadLogId"`
		ChainHeadHash  string `json:"chainHeadHash"`
		SignedLogCount int64  `json:"signedLogCount"`
		KeyID          string `json:"keyId"`
	}{SchemaVersion, chain.TenantID, chain.ChainVersion, chain.AnchorLogID, chain.AnchorHash,
		chain.LegacyLogCount, chain.ChainHeadLogID, chain.ChainHeadHash, chain.SignedLogCount, keyID})
	return sha256Hex(encoded)
}

func safeArtifactPath(root, name string) (string, error) {
	root, name = strings.TrimSpace(root), strings.TrimSpace(name)
	if root == "" || name == "" || filepath.Base(name) != name || strings.ContainsAny(name, `/\\`) {
		return "", Invalid("审计锚点证据路径无效")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	return filepath.Join(absRoot, name), nil
}

func writeImmutableFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrExist) {
		existing, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if !bytes.Equal(existing, content) {
			return errors.New("同名审计锚点证据已存在但内容不一致")
		}
		return nil
	}
	if err != nil {
		return err
	}
	remove := true
	defer func() {
		file.Close()
		if remove {
			os.Remove(path)
		}
	}()
	if _, err := file.Write(content); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	remove = false
	return nil
}

func hmacHex(key, payload []byte) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func sha256Hex(payload []byte) string {
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

func normalizeSource(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return TriggerManual
	}
	return truncate(value, 24)
}

func truncate(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit])
}
