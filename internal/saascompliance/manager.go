package saascompliance

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"jiyi/mochat-go/internal/saasbackup"
)

type Config struct {
	ArtifactRoot     string
	FileStorageRoot  string
	EncryptionKey    string
	EncryptionKeys   string
	EncryptionKeyID  string
	PlatformTenantID int
	Now              func() time.Time
}

type Manager struct {
	store            Store
	artifactRoot     string
	fileStorageRoot  string
	keys             map[string][]byte
	activeKeyID      string
	platformTenantID int
	now              func() time.Time
}

func NewManager(store Store, config Config) (*Manager, error) {
	if store == nil {
		return nil, errors.New("compliance store is required")
	}
	keys, err := saasbackup.ParseEncryptionKeyRing(config.EncryptionKeys)
	if err != nil {
		return nil, err
	}
	activeKeyID := strings.TrimSpace(config.EncryptionKeyID)
	if activeKeyID == "" {
		activeKeyID = "primary"
	}
	legacyKey, err := saasbackup.ParseEncryptionKey(config.EncryptionKey)
	if err != nil {
		return nil, err
	}
	if len(legacyKey) > 0 {
		if existing, ok := keys[activeKeyID]; ok && !bytes.Equal(existing, legacyKey) {
			return nil, fmt.Errorf("compliance export encryption key %q differs between legacy and key ring configuration", activeKeyID)
		}
		keys[activeKeyID] = legacyKey
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &Manager{
		store: store, artifactRoot: strings.TrimSpace(config.ArtifactRoot), fileStorageRoot: strings.TrimSpace(config.FileStorageRoot),
		keys: keys, activeKeyID: activeKeyID, platformTenantID: config.PlatformTenantID, now: now,
	}, nil
}

func (m *Manager) Overview(ctx context.Context, tenantID, limit int) (Overview, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	policy, err := m.store.CompliancePolicy(ctx)
	if err != nil {
		return Overview{}, err
	}
	holds, err := m.store.ComplianceLegalHolds(ctx, tenantID, limit)
	if err != nil {
		return Overview{}, err
	}
	exports, err := m.store.ComplianceExports(ctx, tenantID, limit)
	if err != nil {
		return Overview{}, err
	}
	for index := range exports {
		_, exports[index].EncryptionKeyReady = m.key(exports[index].EncryptionKeyID)
	}
	erasures, err := m.store.ComplianceErasures(ctx, tenantID, limit)
	if err != nil {
		return Overview{}, err
	}
	coverage, coverageErr := m.store.ComplianceInventoryCoverage(ctx, Inventory())
	status := ConfigStatus{
		ArtifactRootConfigured: m.artifactRoot != "", FileStorageConfigured: m.fileStorageRoot != "",
		EncryptionConfigured: len(m.keys) > 0,
		EncryptionKeyID:      m.activeKeyID, EncryptionKeyCount: len(m.keys), InventoryVersion: InventoryVersion,
		InventoryTableCount: len(Inventory()),
	}
	if coverageErr == nil {
		status.InventoryUnknownTables = coverage.UnknownTables
	} else {
		status.InventoryUnknownTables = []string{"inventory_probe_failed"}
	}
	return Overview{Policy: policy, Holds: holds, Exports: exports, Erasures: erasures, Config: status}, nil
}

func (m *Manager) CheckConfiguration(ctx context.Context) error {
	if m.artifactRoot == "" {
		return Unavailable("租户数据导出工件目录未配置")
	}
	if m.fileStorageRoot == "" {
		return Unavailable("租户文件存储目录未配置")
	}
	if _, ok := m.key(m.activeKeyID); !ok {
		return Unavailable("租户数据导出当前加密密钥不可用")
	}
	coverage, err := m.store.ComplianceInventoryCoverage(ctx, Inventory())
	if err != nil {
		return err
	}
	if len(coverage.UnknownTables) > 0 {
		return Unavailable("租户数据清单存在未登记表: " + strings.Join(coverage.UnknownTables, ", "))
	}
	return nil
}

func (m *Manager) UpdatePolicy(ctx context.Context, input PolicyUpdate) (Policy, error) {
	input, err := m.ValidatePolicyUpdate(input)
	if err != nil {
		return Policy{}, err
	}
	return m.store.UpdateCompliancePolicy(ctx, input)
}

func (m *Manager) ValidatePolicyUpdate(input PolicyUpdate) (PolicyUpdate, error) {
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	if input.Status != PolicyStatusActive && input.Status != PolicyStatusDisabled {
		return PolicyUpdate{}, Invalid("status 必须是 active 或 disabled")
	}
	if input.ExportRetentionDays < 1 || input.ExportRetentionDays > 3650 {
		return PolicyUpdate{}, Invalid("exportRetentionDays 必须在 1 到 3650 之间")
	}
	if input.ErasureGraceDays < 0 || input.ErasureGraceDays > 365 {
		return PolicyUpdate{}, Invalid("erasureGraceDays 必须在 0 到 365 之间")
	}
	if input.RecentExportMaxAgeDays < 1 || input.RecentExportMaxAgeDays > 365 {
		return PolicyUpdate{}, Invalid("recentExportMaxAgeDays 必须在 1 到 365 之间")
	}
	if input.BillingRetentionDays < 0 || input.BillingRetentionDays > 3650 || input.AuditRetentionDays < 0 || input.AuditRetentionDays > 3650 {
		return PolicyUpdate{}, Invalid("账务和审计保留天数必须在 0 到 3650 之间")
	}
	if input.ServiceAccountUsageRetentionDays < 1 || input.ServiceAccountUsageRetentionDays > 3650 {
		return PolicyUpdate{}, Invalid("serviceAccountUsageRetentionDays 必须在 1 到 3650 之间")
	}
	if input.ExpectedVersion <= 0 {
		return PolicyUpdate{}, Invalid("expectedVersion 必须是正整数")
	}
	return input, nil
}

func (m *Manager) PlanPolicyUpdate(ctx context.Context, input PolicyUpdate) (PolicyUpdate, error) {
	input, err := m.ValidatePolicyUpdate(input)
	if err != nil {
		return PolicyUpdate{}, err
	}
	current, err := m.store.CompliancePolicy(ctx)
	if err != nil {
		return PolicyUpdate{}, err
	}
	if current.Version != input.ExpectedVersion {
		return PolicyUpdate{}, Conflict("合规策略版本已变化，请刷新后重试")
	}
	return input, nil
}

func (m *Manager) CreateLegalHold(ctx context.Context, input LegalHoldCreate) (LegalHold, error) {
	input.Reason = strings.TrimSpace(input.Reason)
	if input.TenantID <= 0 || input.TenantID == m.platformTenantID {
		return LegalHold{}, Invalid("法律保留只能应用于非平台业务租户")
	}
	if input.Reason == "" || len([]rune(input.Reason)) > 500 {
		return LegalHold{}, Invalid("reason 必填且不能超过 500 个字符")
	}
	tenant, err := m.store.ComplianceTenant(ctx, input.TenantID)
	if err != nil {
		return LegalHold{}, err
	}
	if tenant.DeletedAt != "" {
		return LegalHold{}, Conflict("已擦除租户不能新增法律保留")
	}
	now := m.now()
	if input.StartsAt.IsZero() {
		input.StartsAt = now
	}
	if !input.ExpiresAt.IsZero() && !input.ExpiresAt.After(input.StartsAt) {
		return LegalHold{}, Invalid("expiresAt 必须晚于 startsAt")
	}
	if input.HoldNo == "" {
		input.HoldNo, err = randomReference("HOLD")
		if err != nil {
			return LegalHold{}, err
		}
	}
	return m.store.CreateComplianceLegalHold(ctx, input)
}

func (m *Manager) ReleaseLegalHold(ctx context.Context, input LegalHoldRelease) (LegalHold, error) {
	input.Reason = strings.TrimSpace(input.Reason)
	if input.HoldID <= 0 || input.ExpectedVersion <= 0 || input.Reason == "" || len([]rune(input.Reason)) > 500 {
		return LegalHold{}, Invalid("holdId、expectedVersion 和不超过 500 字的 reason 必填")
	}
	if (input.ApprovalExecutionID > 0) != (input.ApprovalExecutionVersion > 0) {
		return LegalHold{}, Invalid("审批执行引用必须同时包含审批 ID 和版本")
	}
	if input.ApprovalExecutionID > 0 && input.ApprovalPlan == nil {
		return LegalHold{}, Invalid("审批执行必须包含法律保留冻结快照")
	}
	return m.store.ReleaseComplianceLegalHold(ctx, input)
}

func (m *Manager) PlanLegalHoldRelease(ctx context.Context, holdID int64, expectedVersion int, reason string) (LegalHoldReleasePlan, error) {
	reason = strings.TrimSpace(reason)
	if holdID <= 0 || expectedVersion <= 0 || reason == "" || len([]rune(reason)) > 500 {
		return LegalHoldReleasePlan{}, Invalid("holdId、expectedVersion 和不超过 500 字的 reason 必填")
	}
	hold, err := m.store.ComplianceLegalHold(ctx, holdID)
	if err != nil {
		return LegalHoldReleasePlan{}, err
	}
	if hold.Status != HoldStatusActive || hold.Version != expectedVersion {
		return LegalHoldReleasePlan{}, Conflict("法律保留已变化，请刷新后重试")
	}
	return LegalHoldReleasePlan{
		HoldID: hold.ID, HoldNo: hold.HoldNo, TenantID: hold.TenantID, TenantName: hold.TenantName,
		Status: hold.Status, HoldReason: hold.Reason, StartsAt: hold.StartsAt, ExpiresAt: hold.ExpiresAt,
		ExpectedVersion: hold.Version, ReleaseReason: reason,
	}, nil
}

func (m *Manager) ReleaseLegalHoldPlan(ctx context.Context, plan LegalHoldReleasePlan, actor Actor, approvalID int64, approvalVersion int) (LegalHold, error) {
	plan.ReleaseReason = strings.TrimSpace(plan.ReleaseReason)
	if plan.HoldID <= 0 || plan.HoldNo == "" || plan.TenantID <= 0 || plan.Status != HoldStatusActive ||
		plan.ExpectedVersion <= 0 || plan.ReleaseReason == "" || len([]rune(plan.ReleaseReason)) > 500 || approvalID <= 0 || approvalVersion <= 0 {
		return LegalHold{}, Invalid("法律保留解除审批快照无效")
	}
	return m.ReleaseLegalHold(ctx, LegalHoldRelease{
		HoldID: plan.HoldID, Reason: plan.ReleaseReason, ExpectedVersion: plan.ExpectedVersion, Actor: actor,
		ApprovalExecutionID: approvalID, ApprovalExecutionVersion: approvalVersion, ApprovalPlan: &plan,
	})
}

func (m *Manager) RequestExport(ctx context.Context, tenantID int, reason string, actor Actor) (DataExport, error) {
	reason = strings.TrimSpace(reason)
	if tenantID <= 0 || tenantID == m.platformTenantID {
		return DataExport{}, Invalid("只能导出非平台业务租户")
	}
	if reason == "" || len([]rune(reason)) > 500 {
		return DataExport{}, Invalid("reason 必填且不能超过 500 个字符")
	}
	policy, err := m.store.CompliancePolicy(ctx)
	if err != nil {
		return DataExport{}, err
	}
	if policy.Status != PolicyStatusActive {
		return DataExport{}, Conflict("租户数据合规策略已停用")
	}
	key, ok := m.key(m.activeKeyID)
	if !ok || len(key) != 32 {
		return DataExport{}, Unavailable("租户数据导出加密密钥未配置")
	}
	if m.artifactRoot == "" {
		return DataExport{}, Unavailable("租户数据导出工件目录未配置")
	}
	coverage, err := m.store.ComplianceInventoryCoverage(ctx, Inventory())
	if err != nil {
		return DataExport{}, err
	}
	if len(coverage.UnknownTables) > 0 {
		return DataExport{}, Unavailable("租户数据清单存在未登记表: " + strings.Join(coverage.UnknownTables, ", "))
	}
	tenant, err := m.store.ComplianceTenant(ctx, tenantID)
	if err != nil {
		return DataExport{}, err
	}
	if tenant.DeletedAt != "" {
		return DataExport{}, Conflict("已擦除租户不能创建新导出")
	}
	exportNo, err := randomReference("EXP")
	if err != nil {
		return DataExport{}, err
	}
	artifactName := strings.ToLower(exportNo) + fmt.Sprintf("-tenant-%d.%s", tenantID, ArtifactFormat)
	return m.store.CreateComplianceExport(ctx, DataExportCreate{
		ExportNo: exportNo, TenantID: tenantID, TenantName: tenant.Name, ArtifactName: artifactName,
		EncryptionKeyID: m.activeKeyID, InventoryVersion: InventoryVersion, Reason: reason, Actor: actor,
	})
}

func (m *Manager) ProcessExport(ctx context.Context, exportID int64, actor Actor) (result DataExport, err error) {
	if exportID <= 0 {
		return DataExport{}, Invalid("exportId 必须是正整数")
	}
	run, err := m.store.ClaimComplianceExport(ctx, exportID, m.now())
	if err != nil {
		return DataExport{}, err
	}
	finishedAt := m.now()
	policy, policyErr := m.store.CompliancePolicy(ctx)
	if policyErr != nil {
		_, _ = m.store.CompleteComplianceExport(ctx, DataExportCompletion{ExportID: run.ID, Status: ExportStatusFailed, ErrorMessage: policyErr.Error(), FinishedAt: finishedAt, Actor: actor})
		return DataExport{}, policyErr
	}
	stats, artifactErr := m.writeExportArtifact(ctx, run)
	if artifactErr != nil {
		_, _ = m.store.CompleteComplianceExport(ctx, DataExportCompletion{ExportID: run.ID, Status: ExportStatusFailed, ErrorMessage: truncateError(artifactErr), FinishedAt: finishedAt, Actor: actor})
		return DataExport{}, artifactErr
	}
	completed, err := m.store.CompleteComplianceExport(ctx, DataExportCompletion{
		ExportID: run.ID, Status: ExportStatusSucceeded, SHA256: stats.SHA256, SizeBytes: stats.SizeBytes,
		ManifestSHA256: stats.ManifestSHA256, TableCount: stats.TableCount, RowCount: stats.RowCount,
		FileCount: stats.FileCount, FileSizeBytes: stats.FileSizeBytes, FinishedAt: finishedAt,
		ExpiresAt: finishedAt.Add(time.Duration(policy.ExportRetentionDays) * 24 * time.Hour), Actor: actor,
	})
	if err != nil {
		_ = os.Remove(m.artifactPath(run.ArtifactName))
		return DataExport{}, err
	}
	completed.EncryptionKeyReady = true
	return completed, nil
}

func (m *Manager) OpenExport(ctx context.Context, exportID int64, actor Actor) (ExportDownload, error) {
	run, err := m.store.ComplianceExport(ctx, exportID)
	if err != nil {
		return ExportDownload{}, err
	}
	if run.Status != ExportStatusSucceeded || run.DeletedAt != "" || run.DeletionStatus != "" {
		return ExportDownload{}, Conflict("租户数据导出尚不可下载")
	}
	if !run.ExpiresAtValue.IsZero() && !m.now().Before(run.ExpiresAtValue) {
		return ExportDownload{}, Conflict("租户数据导出已过期")
	}
	key, ok := m.key(run.EncryptionKeyID)
	if !ok {
		return ExportDownload{}, Unavailable("租户数据导出历史密钥不可用")
	}
	path := m.artifactPath(run.ArtifactName)
	sha, size, err := fileDigest(path)
	if err != nil {
		return ExportDownload{}, err
	}
	if sha != run.SHA256 || size != run.SizeBytes {
		return ExportDownload{}, Conflict("租户数据导出工件完整性校验失败")
	}
	file, err := os.Open(path)
	if err != nil {
		return ExportDownload{}, err
	}
	reader, err := newEncryptedReader(file, key)
	if err != nil {
		file.Close()
		return ExportDownload{}, err
	}
	updated, err := m.store.RecordComplianceExportDownload(ctx, run.ID, actor)
	if err != nil {
		file.Close()
		return ExportDownload{}, err
	}
	filename := fmt.Sprintf("mochat-tenant-%d-%s.tar.gz", run.TenantID, strings.ToLower(run.ExportNo))
	return ExportDownload{Export: updated, Filename: filename, Reader: &exportReadCloser{reader: reader, closer: file}}, nil
}

func (m *Manager) DeleteExport(ctx context.Context, exportID int64, actor Actor) (DataExport, error) {
	run, err := m.store.ComplianceExport(ctx, exportID)
	if err != nil {
		return DataExport{}, err
	}
	if run.Status == ExportStatusRunning || run.Status == ExportStatusPending {
		return DataExport{}, Conflict("运行中的租户数据导出不能删除")
	}
	if run.DeletionStatus != "" {
		return DataExport{}, Conflict("租户数据导出已进入审批删除流程")
	}
	if run.ExpiresAtValue.IsZero() || m.now().Before(run.ExpiresAtValue) {
		return DataExport{}, Conflict("租户数据导出仍在保留期内，提前删除必须通过审批中心")
	}
	if err := m.ensureExportDeletionAllowed(ctx, run); err != nil {
		return DataExport{}, err
	}
	if run.ArtifactName != "" {
		path, pathErr := safeRootJoin(m.artifactRoot, run.ArtifactName)
		if pathErr != nil {
			return DataExport{}, pathErr
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return DataExport{}, err
		}
	}
	return m.store.MarkComplianceExportDeleted(ctx, run.ID, actor)
}

func (m *Manager) PlanExportDeletion(ctx context.Context, exportID int64) (DataExportDeletionPlan, error) {
	if exportID <= 0 {
		return DataExportDeletionPlan{}, Invalid("exportId 必须是正整数")
	}
	run, err := m.store.ComplianceExport(ctx, exportID)
	if err != nil {
		return DataExportDeletionPlan{}, err
	}
	if run.Status == ExportStatusPending || run.Status == ExportStatusRunning || run.Status == ExportStatusDeleted || run.DeletedAt != "" {
		return DataExportDeletionPlan{}, Conflict("当前租户数据导出状态不允许删除")
	}
	if run.DeletionStatus != "" {
		return DataExportDeletionPlan{}, Conflict("租户数据导出已有删除任务；失败任务请直接重试")
	}
	if strings.TrimSpace(run.ArtifactName) != "" {
		if filepath.Base(run.ArtifactName) != run.ArtifactName {
			return DataExportDeletionPlan{}, Invalid("租户数据导出工件名称无效")
		}
		if _, err := safeRootJoin(m.artifactRoot, run.ArtifactName); err != nil {
			return DataExportDeletionPlan{}, err
		}
	}
	if err := m.ensureExportDeletionAllowed(ctx, run); err != nil {
		return DataExportDeletionPlan{}, err
	}
	return DataExportDeletionPlan{
		ExportID: run.ID, ExportNo: run.ExportNo, TenantID: run.TenantID, TenantName: run.TenantName,
		Status: run.Status, ArtifactName: run.ArtifactName, SHA256: run.SHA256, SizeBytes: run.SizeBytes,
		FinishedAt: run.FinishedAt, ExpiresAt: run.ExpiresAt,
	}, nil
}

func (m *Manager) ScheduleExportDeletion(ctx context.Context, plan DataExportDeletionPlan, actor Actor, approvalID int64, approvalVersion int) (DataExport, error) {
	scheduled, err := m.store.ScheduleComplianceExportDeletion(ctx, DataExportDeletionSchedule{
		Plan: plan, Actor: actor, ApprovalExecutionID: approvalID, ApprovalExecutionVersion: approvalVersion,
	})
	if err != nil {
		return DataExport{}, err
	}
	claimed, err := m.store.ClaimComplianceExportDeletion(ctx, scheduled.ID, false, actor)
	if err != nil {
		if latest, readErr := m.store.ComplianceExport(context.WithoutCancel(ctx), scheduled.ID); readErr == nil {
			return latest, nil
		}
		return scheduled, nil
	}
	processed, err := m.processExportDeletion(ctx, claimed, actor)
	if err != nil {
		if latest, readErr := m.store.ComplianceExport(context.WithoutCancel(ctx), scheduled.ID); readErr == nil {
			return latest, nil
		}
		return scheduled, nil
	}
	return processed, nil
}

func (m *Manager) RetryExportDeletion(ctx context.Context, exportID int64, actor Actor) (DataExport, error) {
	run, err := m.store.ComplianceExport(ctx, exportID)
	if err != nil {
		return DataExport{}, err
	}
	retry := run.DeletionStatus == ExportDeletionStatusFailed
	if run.DeletionStatus != ExportDeletionStatusPending && run.DeletionStatus != ExportDeletionStatusRunning && !retry {
		return DataExport{}, Conflict("合规导出没有可继续执行的删除任务")
	}
	claimed, err := m.store.ClaimComplianceExportDeletion(ctx, exportID, retry, actor)
	if err != nil {
		return DataExport{}, err
	}
	return m.processExportDeletion(ctx, claimed, actor)
}

func (m *Manager) processExportDeletion(ctx context.Context, run DataExport, actor Actor) (DataExport, error) {
	if run.DeletionArtifactStatus != ExportDeletionStepDeleted && run.DeletionArtifactStatus != ExportDeletionStepMissing {
		status := ExportDeletionStepMissing
		var deleteErr error
		if strings.TrimSpace(run.ArtifactName) != "" {
			status = ExportDeletionStepDeleted
			path, pathErr := safeRootJoin(m.artifactRoot, run.ArtifactName)
			if pathErr != nil {
				deleteErr = pathErr
			} else if err := os.Remove(path); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					status = ExportDeletionStepMissing
				} else {
					deleteErr = err
				}
			}
		}
		if deleteErr != nil {
			updated, checkpointErr := m.store.CheckpointComplianceExportDeletion(context.WithoutCancel(ctx), DataExportDeletionCheckpoint{
				ExportID: run.ID, Attempt: run.DeletionAttempts, Status: ExportDeletionStepFailed,
				ErrorMessage: truncateError(deleteErr), Actor: actor,
			})
			if checkpointErr != nil {
				return run, checkpointErr
			}
			return updated, nil
		}
		updated, err := m.store.CheckpointComplianceExportDeletion(context.WithoutCancel(ctx), DataExportDeletionCheckpoint{
			ExportID: run.ID, Attempt: run.DeletionAttempts, Status: status, Actor: actor,
		})
		if err != nil {
			return run, err
		}
		run = updated
	}
	return m.store.CompleteComplianceExportDeletion(context.WithoutCancel(ctx), DataExportDeletionCompletion{
		ExportID: run.ID, Attempt: run.DeletionAttempts, Actor: actor,
	})
}

func (m *Manager) ensureExportDeletionAllowed(ctx context.Context, run DataExport) error {
	if _, found, err := m.store.ActiveComplianceLegalHold(ctx, run.TenantID, m.now()); err != nil {
		return err
	} else if found {
		return Conflict("租户存在生效中的法律保留，不能删除合规导出工件")
	}
	erasures, err := m.store.ComplianceErasures(ctx, run.TenantID, 200)
	if err != nil {
		return err
	}
	for _, item := range erasures {
		if item.LatestExportID != run.ID {
			continue
		}
		if item.Status != ErasureStatusCanceled && item.Status != ErasureStatusSucceeded {
			return Conflict("合规导出仍被未终结的数据擦除请求引用，不能删除")
		}
	}
	return nil
}

func (m *Manager) RequestErasure(ctx context.Context, tenantID int, confirmation, reason string, actor Actor) (ErasureRequest, error) {
	confirmation = strings.TrimSpace(confirmation)
	reason = strings.TrimSpace(reason)
	if tenantID <= 0 || tenantID == m.platformTenantID {
		return ErasureRequest{}, Invalid("平台管理租户永远不能擦除")
	}
	if reason == "" || len([]rune(reason)) > 500 {
		return ErasureRequest{}, Invalid("reason 必填且不能超过 500 个字符")
	}
	policy, err := m.store.CompliancePolicy(ctx)
	if err != nil {
		return ErasureRequest{}, err
	}
	if policy.Status != PolicyStatusActive {
		return ErasureRequest{}, Conflict("租户数据合规策略已停用")
	}
	tenant, err := m.store.ComplianceTenant(ctx, tenantID)
	if err != nil {
		return ErasureRequest{}, err
	}
	if tenant.DeletedAt != "" {
		return ErasureRequest{}, Conflict("租户已经完成擦除")
	}
	if tenant.Status != 2 {
		return ErasureRequest{}, Conflict("擦除前必须先停用业务租户")
	}
	if confirmation != tenant.Name {
		return ErasureRequest{}, Invalid("确认名称必须与当前租户名称完全一致")
	}
	now := m.now()
	if hold, active, err := m.store.ActiveComplianceLegalHold(ctx, tenantID, now); err != nil {
		return ErasureRequest{}, err
	} else if active {
		return ErasureRequest{}, Conflict("活动法律保留阻止擦除: " + hold.HoldNo)
	}
	latestExportID, err := m.recentExportID(ctx, tenantID, policy, now)
	if err != nil {
		return ErasureRequest{}, err
	}
	requestNo, err := randomReference("ERS")
	if err != nil {
		return ErasureRequest{}, err
	}
	confirmationDigest := sha256.Sum256([]byte(fmt.Sprintf("%d\x00%s", tenantID, confirmation)))
	return m.store.CreateComplianceErasure(ctx, ErasureCreate{
		RequestNo: requestNo, TenantID: tenantID, TenantName: tenant.Name, Reason: reason,
		ConfirmationSHA: hex.EncodeToString(confirmationDigest[:]), EligibleAt: now.Add(time.Duration(policy.ErasureGraceDays) * 24 * time.Hour),
		LatestExportID: latestExportID, InventoryVersion: InventoryVersion, Actor: actor,
	})
}

func (m *Manager) AuthorizeErasure(ctx context.Context, input ErasureAuthorize) (ErasureRequest, error) {
	if input.RequestID <= 0 || input.ApprovalID <= 0 || input.ApprovalUserID <= 0 {
		return ErasureRequest{}, Invalid("requestId、approvalId 和审批执行人必填")
	}
	return m.store.AuthorizeComplianceErasure(ctx, input)
}

func (m *Manager) Erasure(ctx context.Context, requestID int64) (ErasureRequest, error) {
	if requestID <= 0 {
		return ErasureRequest{}, Invalid("requestId 必须是正整数")
	}
	return m.store.ComplianceErasure(ctx, requestID)
}

func (m *Manager) ErasureSteps(ctx context.Context, requestID int64) ([]ErasureStep, error) {
	if requestID <= 0 {
		return nil, Invalid("requestId 必须是正整数")
	}
	if _, err := m.store.ComplianceErasure(ctx, requestID); err != nil {
		return nil, err
	}
	return m.store.ComplianceErasureSteps(ctx, requestID)
}

func (m *Manager) CancelErasure(ctx context.Context, requestID int64, reason string, actor Actor) (ErasureRequest, error) {
	reason = strings.TrimSpace(reason)
	if requestID <= 0 || reason == "" || len([]rune(reason)) > 500 {
		return ErasureRequest{}, Invalid("requestId 和不超过 500 字的 reason 必填")
	}
	return m.store.CancelComplianceErasure(ctx, requestID, reason, actor)
}

func (m *Manager) ProcessErasure(ctx context.Context, requestID int64, actor Actor) (ErasureRequest, error) {
	request, err := m.store.ComplianceErasure(ctx, requestID)
	if err != nil {
		return ErasureRequest{}, err
	}
	if request.Status != ErasureStatusApproved && request.Status != ErasureStatusWaiting && request.Status != ErasureStatusFailed && request.Status != ErasureStatusBlocked && request.Status != ErasureStatusRunning {
		return ErasureRequest{}, Conflict("擦除请求尚未获得审批授权")
	}
	now := m.now()
	if now.Before(request.EligibleAtValue) {
		return ErasureRequest{}, Conflict("擦除请求仍在宽限期内")
	}
	if hold, active, holdErr := m.store.ActiveComplianceLegalHold(ctx, request.TenantID, now); holdErr != nil {
		return ErasureRequest{}, holdErr
	} else if active {
		message := "活动法律保留阻止擦除: " + hold.HoldNo
		_ = m.store.BlockComplianceErasure(ctx, request.ID, message, now)
		return ErasureRequest{}, Conflict(message)
	}
	policy, err := m.store.CompliancePolicy(ctx)
	if err != nil {
		return ErasureRequest{}, err
	}
	if _, err := m.recentExportID(ctx, request.TenantID, policy, now); err != nil {
		_ = m.store.BlockComplianceErasure(ctx, request.ID, err.Error(), now)
		return ErasureRequest{}, err
	}
	blocks, err := m.store.ComplianceRetentionBlocks(ctx, request.TenantID, policy, now)
	if err != nil {
		return ErasureRequest{}, err
	}
	if len(blocks) > 0 {
		message := retentionBlockMessage(blocks)
		_ = m.store.BlockComplianceErasure(ctx, request.ID, message, now)
		return ErasureRequest{}, Conflict(message)
	}
	coverage, err := m.store.ComplianceInventoryCoverage(ctx, Inventory())
	if err != nil {
		return ErasureRequest{}, err
	}
	if len(coverage.UnknownTables) > 0 {
		message := "租户数据清单存在未登记表: " + strings.Join(coverage.UnknownTables, ", ")
		_ = m.store.BlockComplianceErasure(ctx, request.ID, message, now)
		return ErasureRequest{}, Unavailable(message)
	}
	request, err = m.store.PrepareComplianceErasure(ctx, request.ID, Inventory(), now)
	if err != nil {
		return ErasureRequest{}, err
	}
	specByKey := make(map[string]DatasetSpec)
	for _, spec := range Inventory() {
		specByKey[spec.Key] = spec
	}
	steps, err := m.store.ComplianceErasureSteps(ctx, request.ID)
	if err != nil {
		return ErasureRequest{}, err
	}
	for _, step := range steps {
		if step.Status == StepStatusSucceeded || step.Action == DatasetActionVerify || step.Action == DatasetActionTombstone {
			continue
		}
		if step.Action == DatasetActionArtifacts {
			deleted, cleanupErr := m.deleteTenantExportArtifacts(ctx, request.TenantID)
			if cleanupErr != nil {
				_ = m.store.FailComplianceErasure(ctx, request.ID, truncateError(cleanupErr), m.now())
				return ErasureRequest{}, cleanupErr
			}
			if _, err := m.store.ApplyComplianceErasureArtifacts(ctx, request, deleted, m.now()); err != nil {
				_ = m.store.FailComplianceErasure(ctx, request.ID, truncateError(err), m.now())
				return ErasureRequest{}, err
			}
			continue
		}
		if step.Action == DatasetActionFiles {
			deleted, cleanupErr := m.deleteTenantStorageFiles(ctx, request.TenantID)
			if cleanupErr != nil {
				_ = m.store.FailComplianceErasure(ctx, request.ID, truncateError(cleanupErr), m.now())
				return ErasureRequest{}, cleanupErr
			}
			if _, err := m.store.ApplyComplianceErasureStorageFiles(ctx, request, deleted, m.now()); err != nil {
				_ = m.store.FailComplianceErasure(ctx, request.ID, truncateError(err), m.now())
				return ErasureRequest{}, err
			}
			continue
		}
		spec, ok := specByKey[step.StepKey]
		if !ok {
			err := fmt.Errorf("unknown erasure step %s", step.StepKey)
			_ = m.store.FailComplianceErasure(ctx, request.ID, err.Error(), m.now())
			return ErasureRequest{}, err
		}
		if _, err := m.store.ApplyComplianceErasureStep(ctx, request, spec, m.now()); err != nil {
			_ = m.store.FailComplianceErasure(ctx, request.ID, truncateError(err), m.now())
			return ErasureRequest{}, err
		}
	}
	steps, err = m.store.ComplianceErasureSteps(ctx, request.ID)
	if err != nil {
		return ErasureRequest{}, err
	}
	completedAt := m.now()
	for index := range steps {
		if steps[index].Action == DatasetActionVerify || steps[index].Action == DatasetActionTombstone {
			steps[index].Status = StepStatusSucceeded
			steps[index].StartedAt = completedAt.Format("2006-01-02 15:04:05")
			steps[index].FinishedAt = completedAt.Format("2006-01-02 15:04:05")
			if steps[index].Action == DatasetActionTombstone {
				steps[index].AffectedRows = 1
			}
		}
	}
	report := map[string]any{
		"requestNo": request.RequestNo, "tenantId": request.TenantID, "inventoryVersion": InventoryVersion,
		"completedAt": completedAt.UTC().Format(time.RFC3339), "steps": steps,
	}
	reportJSON, err := json.Marshal(report)
	if err != nil {
		return ErasureRequest{}, err
	}
	verification := sha256.Sum256(reportJSON)
	nameDigest := sha256.Sum256([]byte(fmt.Sprintf("%d\x00%s\x00%s", request.TenantID, request.TenantName, request.RequestNo)))
	anonymousRef := "erased-" + hex.EncodeToString(nameDigest[:12])
	completed, err := m.store.FinalizeComplianceErasure(ctx, request, anonymousRef, hex.EncodeToString(nameDigest[:]), hex.EncodeToString(verification[:]), string(reportJSON), actor, m.now())
	if err != nil {
		_ = m.store.FailComplianceErasure(ctx, request.ID, truncateError(err), m.now())
		return ErasureRequest{}, err
	}
	return completed, nil
}

func (m *Manager) ProcessPending(ctx context.Context, limit int, actor Actor) (ProcessResult, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	result := ProcessResult{ExportIDs: []int64{}, ExportDeletionIDs: []int64{}, ErasureIDs: []int64{}}
	for result.ExportsProcessed < limit {
		run, ok, err := m.store.NextPendingComplianceExport(ctx)
		if err != nil {
			return result, err
		}
		if !ok {
			break
		}
		if _, err := m.ProcessExport(ctx, run.ID, actor); err != nil {
			return result, err
		}
		result.ExportsProcessed++
		result.ExportIDs = append(result.ExportIDs, run.ID)
	}
	deletions, err := m.store.RunnableComplianceExportDeletions(ctx, limit)
	if err != nil {
		return result, err
	}
	for _, run := range deletions {
		claimed, claimErr := m.store.ClaimComplianceExportDeletion(ctx, run.ID, false, actor)
		if claimErr != nil {
			var operationErr *OperationError
			if errors.As(claimErr, &operationErr) && operationErr.Code == "conflict" {
				continue
			}
			return result, claimErr
		}
		if _, processErr := m.processExportDeletion(ctx, claimed, actor); processErr != nil {
			return result, processErr
		}
		result.ExportDeletionsProcessed++
		result.ExportDeletionIDs = append(result.ExportDeletionIDs, run.ID)
	}
	for result.ErasuresProcessed < limit {
		request, ok, err := m.store.NextDueComplianceErasure(ctx, m.now())
		if err != nil {
			return result, err
		}
		if !ok {
			break
		}
		if _, err := m.ProcessErasure(ctx, request.ID, actor); err != nil {
			return result, err
		}
		result.ErasuresProcessed++
		result.ErasureIDs = append(result.ErasureIDs, request.ID)
	}
	deleted, err := m.cleanupExpiredExports(ctx, actor)
	if err != nil {
		return result, err
	}
	result.ArtifactsDeleted = deleted
	return result, nil
}

type exportArtifactStats struct {
	SHA256, ManifestSHA256 string
	SizeBytes              int64
	TableCount             int
	RowCount               int64
	FileCount              int
	FileSizeBytes          int64
}

type exportDatasetManifest struct {
	Table   string   `json:"table"`
	Columns []string `json:"columns"`
	Rows    int64    `json:"rows"`
	SHA256  string   `json:"sha256"`
}

type exportFileManifest struct {
	StorageObjectID int64  `json:"storageObjectId"`
	SourcePath      string `json:"sourcePath"`
	ArchivePath     string `json:"archivePath"`
	OriginalName    string `json:"originalName"`
	RecordedSize    int64  `json:"recordedSize"`
	ActualSize      int64  `json:"actualSize"`
	SHA256          string `json:"sha256"`
}

func (m *Manager) writeExportArtifact(ctx context.Context, run DataExport) (stats exportArtifactStats, resultErr error) {
	key, ok := m.key(run.EncryptionKeyID)
	if !ok {
		return stats, Unavailable("租户数据导出加密密钥不可用")
	}
	if err := os.MkdirAll(m.artifactRoot, 0o700); err != nil {
		return stats, err
	}
	workspace, err := os.MkdirTemp(m.artifactRoot, ".compliance-export-work-")
	if err != nil {
		return stats, err
	}
	defer os.RemoveAll(workspace)
	dataRoot := filepath.Join(workspace, "data")
	if err := os.MkdirAll(dataRoot, 0o700); err != nil {
		return stats, err
	}

	datasets := make([]exportDatasetManifest, 0, len(Inventory()))
	for _, spec := range Inventory() {
		if !spec.Export {
			continue
		}
		iterator, err := m.store.OpenComplianceDataset(ctx, spec, run.TenantID)
		if err != nil {
			return stats, fmt.Errorf("open dataset %s: %w", spec.Table, err)
		}
		path := filepath.Join(dataRoot, spec.Key+".jsonl")
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			iterator.Close()
			return stats, err
		}
		encoder := json.NewEncoder(file)
		encoder.SetEscapeHTML(false)
		columns := iterator.Columns()
		var rowCount int64
		for iterator.Next() {
			values, valueErr := iterator.Values()
			if valueErr != nil {
				file.Close()
				iterator.Close()
				return stats, valueErr
			}
			row := make(map[string]any, len(columns))
			for index, column := range columns {
				row[column] = normalizeDatabaseValue(values[index])
			}
			if err := encoder.Encode(row); err != nil {
				file.Close()
				iterator.Close()
				return stats, err
			}
			rowCount++
		}
		iteratorErr := iterator.Err()
		closeRowsErr := iterator.Close()
		closeFileErr := file.Close()
		if iteratorErr != nil {
			return stats, iteratorErr
		}
		if closeRowsErr != nil {
			return stats, closeRowsErr
		}
		if closeFileErr != nil {
			return stats, closeFileErr
		}
		sha, _, err := fileDigest(path)
		if err != nil {
			return stats, err
		}
		datasets = append(datasets, exportDatasetManifest{Table: spec.Table, Columns: columns, Rows: rowCount, SHA256: sha})
		stats.RowCount += rowCount
	}
	stats.TableCount = len(datasets)

	storage, err := m.store.ComplianceStorageFiles(ctx, run.TenantID)
	if err != nil {
		return stats, err
	}
	files := make([]exportFileManifest, 0, len(storage))
	archiveFiles := make(map[string]string, len(storage))
	for _, item := range storage {
		localPath, err := safeRootJoin(m.fileStorageRoot, item.RelativePath)
		if err != nil {
			return stats, fmt.Errorf("storage object %d: %w", item.ID, err)
		}
		sha, size, err := fileDigest(localPath)
		if err != nil {
			return stats, fmt.Errorf("storage object %d: %w", item.ID, err)
		}
		name := safeArchiveName(item.OriginalName)
		archivePath := fmt.Sprintf("files/%d/%s", item.ID, name)
		files = append(files, exportFileManifest{StorageObjectID: item.ID, SourcePath: item.RelativePath, ArchivePath: archivePath, OriginalName: item.OriginalName, RecordedSize: item.SizeBytes, ActualSize: size, SHA256: sha})
		archiveFiles[archivePath] = localPath
		stats.FileCount++
		stats.FileSizeBytes += size
	}

	manifest := map[string]any{
		"formatVersion": 1, "inventoryVersion": InventoryVersion, "exportNo": run.ExportNo,
		"tenant":      map[string]any{"id": run.TenantID, "name": run.TenantName},
		"generatedAt": m.now().UTC().Format(time.RFC3339Nano), "datasets": datasets, "files": files,
		"summary": map[string]any{"tableCount": stats.TableCount, "rowCount": stats.RowCount, "fileCount": stats.FileCount, "fileSizeBytes": stats.FileSizeBytes},
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return stats, err
	}
	manifestBytes = append(manifestBytes, '\n')
	manifestDigest := sha256.Sum256(manifestBytes)
	stats.ManifestSHA256 = hex.EncodeToString(manifestDigest[:])
	manifestPath := filepath.Join(workspace, "manifest.json")
	if err := os.WriteFile(manifestPath, manifestBytes, 0o600); err != nil {
		return stats, err
	}

	finalPath := m.artifactPath(run.ArtifactName)
	tempPath := finalPath + ".partial"
	_ = os.Remove(tempPath)
	output, err := os.OpenFile(tempPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return stats, err
	}
	defer func() {
		if resultErr != nil {
			output.Close()
			_ = os.Remove(tempPath)
		}
	}()
	encrypted, err := newEncryptedWriter(output, key)
	if err != nil {
		return stats, err
	}
	gzipWriter := gzip.NewWriter(encrypted)
	tarWriter := tar.NewWriter(gzipWriter)
	entries := make([]struct{ archive, local string }, 0, len(datasets)+len(archiveFiles)+1)
	entries = append(entries, struct{ archive, local string }{"manifest.json", manifestPath})
	for _, spec := range Inventory() {
		if spec.Export {
			entries = append(entries, struct{ archive, local string }{"data/" + spec.Key + ".jsonl", filepath.Join(dataRoot, spec.Key+".jsonl")})
		}
	}
	fileNames := make([]string, 0, len(archiveFiles))
	for name := range archiveFiles {
		fileNames = append(fileNames, name)
	}
	sort.Strings(fileNames)
	for _, name := range fileNames {
		entries = append(entries, struct{ archive, local string }{name, archiveFiles[name]})
	}
	for _, entry := range entries {
		if err := writeTarFile(tarWriter, entry.archive, entry.local, m.now()); err != nil {
			return stats, err
		}
	}
	if err := tarWriter.Close(); err != nil {
		return stats, err
	}
	if err := gzipWriter.Close(); err != nil {
		return stats, err
	}
	if err := encrypted.Close(); err != nil {
		return stats, err
	}
	if err := output.Sync(); err != nil {
		return stats, err
	}
	if err := output.Close(); err != nil {
		return stats, err
	}
	if err := os.Rename(tempPath, finalPath); err != nil {
		return stats, err
	}
	stats.SHA256, stats.SizeBytes, err = fileDigest(finalPath)
	return stats, err
}

func (m *Manager) recentExportID(ctx context.Context, tenantID int, policy Policy, now time.Time) (int64, error) {
	if !policy.RequireRecentExport {
		return 0, nil
	}
	exports, err := m.store.ComplianceExports(ctx, tenantID, 100)
	if err != nil {
		return 0, err
	}
	cutoff := now.Add(-time.Duration(policy.RecentExportMaxAgeDays) * 24 * time.Hour)
	for _, item := range exports {
		if item.Status == ExportStatusSucceeded && item.DeletedAt == "" && !item.FinishedAtValue.IsZero() && !item.FinishedAtValue.Before(cutoff) &&
			(item.ExpiresAtValue.IsZero() || now.Before(item.ExpiresAtValue)) {
			return item.ID, nil
		}
	}
	return 0, Conflict("擦除前必须完成一份仍在有效期内的近期租户数据导出")
}

func (m *Manager) deleteTenantExportArtifacts(ctx context.Context, tenantID int) (int, error) {
	exports, err := m.store.ComplianceExports(ctx, tenantID, 200)
	if err != nil {
		return 0, err
	}
	deleted := 0
	for _, item := range exports {
		if item.ArtifactName != "" {
			if err := os.Remove(m.artifactPath(item.ArtifactName)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return deleted, err
			}
		}
		deleted++
	}
	return deleted, nil
}

func (m *Manager) deleteTenantStorageFiles(ctx context.Context, tenantID int) (int, error) {
	files, err := m.store.ComplianceStorageFiles(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	deleted := 0
	for _, item := range files {
		path, err := safeRootJoin(m.fileStorageRoot, item.RelativePath)
		if err != nil {
			return deleted, fmt.Errorf("storage object %d: %w", item.ID, err)
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return deleted, fmt.Errorf("storage object %d: %w", item.ID, err)
		}
		deleted++
	}
	return deleted, nil
}

func (m *Manager) cleanupExpiredExports(ctx context.Context, actor Actor) (int, error) {
	exports, err := m.store.ComplianceExports(ctx, 0, 200)
	if err != nil {
		return 0, err
	}
	now := m.now()
	deleted := 0
	for _, item := range exports {
		if item.Status != ExportStatusSucceeded || item.ExpiresAtValue.IsZero() || now.Before(item.ExpiresAtValue) {
			continue
		}
		if _, err := m.DeleteExport(ctx, item.ID, actor); err != nil {
			var operationErr *OperationError
			if errors.As(err, &operationErr) && operationErr.Code == "conflict" {
				continue
			}
			return deleted, err
		}
		deleted++
	}
	return deleted, nil
}

func (m *Manager) key(id string) ([]byte, bool) {
	master, ok := m.keys[strings.TrimSpace(id)]
	if !ok || len(master) != 32 {
		return nil, false
	}
	return deriveComplianceKey(master), true
}

func (m *Manager) artifactPath(name string) string {
	if name == "" || filepath.Base(name) != name || strings.ContainsAny(name, `/\\`) {
		return filepath.Join(m.artifactRoot, "invalid-artifact-name")
	}
	return filepath.Join(m.artifactRoot, name)
}

func randomReference(prefix string) (string, error) {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return prefix + "-" + strings.ToUpper(hex.EncodeToString(value)), nil
}

func fileDigest(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}

func safeRootJoin(root, relative string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", Unavailable("文件存储目录未配置")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	relative = strings.TrimLeft(strings.TrimSpace(relative), `/\\`)
	clean := filepath.Clean(relative)
	if clean == "." || clean == "" || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", Invalid("租户文件相对路径无效")
	}
	result := filepath.Join(absRoot, clean)
	if result != absRoot && !strings.HasPrefix(result, absRoot+string(filepath.Separator)) {
		return "", Invalid("租户文件路径越界")
	}
	return result, nil
}

func safeArchiveName(value string) string {
	value = strings.TrimSpace(filepath.Base(value))
	if value == "" || value == "." || value == string(filepath.Separator) {
		return "file.bin"
	}
	return strings.ReplaceAll(value, "\\", "_")
}

func writeTarFile(writer *tar.Writer, archivePath, localPath string, modTime time.Time) error {
	info, err := os.Stat(localPath)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("export source is not a regular file: %s", localPath)
	}
	header := &tar.Header{Name: filepath.ToSlash(archivePath), Mode: 0o600, Size: info.Size(), ModTime: modTime.UTC()}
	if err := writer.WriteHeader(header); err != nil {
		return err
	}
	file, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(writer, file)
	return err
}

func normalizeDatabaseValue(value any) any {
	switch item := value.(type) {
	case []byte:
		return string(item)
	case time.Time:
		return item.UTC().Format(time.RFC3339Nano)
	default:
		return item
	}
}

func retentionBlockMessage(blocks []RetentionBlock) string {
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		parts = append(parts, fmt.Sprintf("%s(%s) %d 条，保留至 %s", block.Category, block.Table, block.RecordCount, block.RetainUntil))
	}
	return "法定保留期内的数据阻止擦除: " + strings.Join(parts, "; ")
}

func truncateError(err error) string {
	if err == nil {
		return ""
	}
	value := err.Error()
	if len([]rune(value)) <= 1000 {
		return value
	}
	return string([]rune(value)[:1000])
}
