package saasbackup

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"

	"jiyi/mochat-go/internal/mysqlconn"
)

type Config struct {
	SourceDSN             string
	BackupRoot            string
	EncryptionKey         string
	EncryptionKeys        string
	EncryptionKeyID       string
	CronEnabled           bool
	CronInterval          time.Duration
	CronRunOnStart        bool
	DumpBinary            string
	RestoreBinary         string
	RestoreDSN            string
	RestoreAdminDSN       string
	RestoreAutoProvision  bool
	RestoreKeepOnFailure  bool
	RestoreDatabasePrefix string
	Replica               ReplicaStore
}

type Manager struct {
	store               Store
	config              Config
	encryptionKeys      map[string][]byte
	activeEncryptionKey []byte
	sourceConfig        *mysqldriver.Config
	restoreConfig       *mysqldriver.Config
	restoreAdminConfig  *mysqldriver.Config
	replica             ReplicaStore
}

func NewManager(store Store, config Config) (*Manager, error) {
	if store == nil {
		return nil, fmt.Errorf("backup store is required")
	}
	config.SourceDSN = strings.TrimSpace(config.SourceDSN)
	config.BackupRoot = strings.TrimSpace(config.BackupRoot)
	config.EncryptionKeyID = strings.TrimSpace(config.EncryptionKeyID)
	config.DumpBinary = strings.TrimSpace(config.DumpBinary)
	config.RestoreBinary = strings.TrimSpace(config.RestoreBinary)
	config.RestoreDSN = strings.TrimSpace(config.RestoreDSN)
	config.RestoreAdminDSN = strings.TrimSpace(config.RestoreAdminDSN)
	config.RestoreDatabasePrefix = strings.TrimSpace(config.RestoreDatabasePrefix)
	if config.BackupRoot == "" {
		config.BackupRoot = "./storage/backups"
	}
	if config.EncryptionKeyID == "" {
		config.EncryptionKeyID = "primary"
	}
	if config.DumpBinary == "" {
		config.DumpBinary = "mysqldump"
	}
	if config.RestoreBinary == "" {
		config.RestoreBinary = "mysql"
	}
	if config.RestoreDatabasePrefix == "" {
		config.RestoreDatabasePrefix = "mochat_restore_"
	}
	if config.CronEnabled && config.CronInterval <= 0 {
		return nil, fmt.Errorf("backup cron interval must be positive when cron is enabled")
	}
	if !encryptionKeyIDPattern.MatchString(config.EncryptionKeyID) {
		return nil, fmt.Errorf("backup encryption key id %q is invalid", config.EncryptionKeyID)
	}
	keys, err := ParseEncryptionKeyRing(config.EncryptionKeys)
	if err != nil {
		return nil, err
	}
	legacyKey, err := ParseEncryptionKey(config.EncryptionKey)
	if err != nil {
		return nil, err
	}
	if len(legacyKey) == 32 {
		if existing, ok := keys[config.EncryptionKeyID]; ok && !bytes.Equal(existing, legacyKey) {
			return nil, fmt.Errorf("active backup encryption key %q conflicts with the key ring", config.EncryptionKeyID)
		}
		keys[config.EncryptionKeyID] = legacyKey
	}
	activeKey := keys[config.EncryptionKeyID]
	if len(keys) > 0 && len(activeKey) != 32 {
		return nil, fmt.Errorf("active backup encryption key %q is missing from the key ring", config.EncryptionKeyID)
	}
	if config.RestoreAutoProvision && config.RestoreDSN != "" {
		return nil, fmt.Errorf("preconfigured restore DSN and automatic restore provisioning cannot be enabled together")
	}
	manager := &Manager{
		store: store, config: config, encryptionKeys: keys, activeEncryptionKey: activeKey, replica: config.Replica,
	}
	if config.SourceDSN != "" {
		manager.sourceConfig, err = mysqldriver.ParseDSN(config.SourceDSN)
		if err != nil {
			return nil, fmt.Errorf("parse backup source DSN: %w", err)
		}
	}
	if config.RestoreDSN != "" {
		manager.restoreConfig, err = mysqldriver.ParseDSN(config.RestoreDSN)
		if err != nil {
			return nil, fmt.Errorf("parse backup restore DSN: %w", err)
		}
	}
	if config.RestoreAdminDSN != "" {
		manager.restoreAdminConfig, err = mysqldriver.ParseDSN(config.RestoreAdminDSN)
		if err != nil {
			return nil, fmt.Errorf("parse backup restore admin DSN: %w", err)
		}
	}
	if config.RestoreAutoProvision && manager.restoreAdminConfig == nil {
		return nil, fmt.Errorf("automatic restore provisioning requires a restore admin DSN")
	}
	return manager, nil
}

func (m *Manager) ConfigStatus() ConfigStatus {
	_, dumpErr := exec.LookPath(m.config.DumpBinary)
	_, restoreErr := exec.LookPath(m.config.RestoreBinary)
	status := ConfigStatus{
		SourceConfigured:       m.sourceConfig != nil && strings.TrimSpace(m.sourceConfig.DBName) != "",
		BackupRootConfigured:   strings.TrimSpace(m.config.BackupRoot) != "",
		EncryptionConfigured:   len(m.activeEncryptionKey) == 32,
		EncryptionKeyCount:     len(m.encryptionKeys),
		CronEnabled:            m.config.CronEnabled,
		CronIntervalSeconds:    int64(m.config.CronInterval / time.Second),
		CronRunOnStart:         m.config.CronRunOnStart,
		RestoreConfigured:      (m.restoreConfig != nil && strings.TrimSpace(m.restoreConfig.DBName) != "") || (m.config.RestoreAutoProvision && m.restoreAdminConfig != nil),
		RestoreAutoProvision:   m.config.RestoreAutoProvision,
		RestoreAdminConfigured: m.restoreAdminConfig != nil,
		DumpToolReady:          dumpErr == nil,
		RestoreToolReady:       restoreErr == nil,
		ReplicaConfigured:      m.replica != nil,
		EncryptionKeyID:        m.config.EncryptionKeyID,
		RestoreDatabasePrefix:  m.config.RestoreDatabasePrefix,
	}
	if m.replica != nil {
		status.ReplicaProvider = m.replica.Provider()
		status.ReplicaBucket = m.replica.Bucket()
	}
	return status
}

func (m *Manager) CheckAutomation(ctx context.Context) error {
	policy, err := m.store.BackupPolicy(ctx)
	if err != nil {
		return fmt.Errorf("读取备份策略: %w", err)
	}
	if policy.Status != PolicyStatusActive {
		return fmt.Errorf("自动备份策略已停用")
	}
	status := m.ConfigStatus()
	if !status.CronEnabled {
		return fmt.Errorf("自动备份调度器未启用")
	}
	if status.CronIntervalSeconds <= 0 {
		return fmt.Errorf("自动备份调度扫描间隔无效")
	}
	return nil
}

func (m *Manager) ProbeReplica(ctx context.Context) error {
	if m.replica == nil {
		return Unavailable("S3 兼容异地副本未配置")
	}
	return m.replica.Probe(ctx)
}

func (m *Manager) CheckEncryptionKeyRing(ctx context.Context) error {
	if len(m.activeEncryptionKey) != 32 {
		return fmt.Errorf("active backup encryption key %q is missing", m.config.EncryptionKeyID)
	}
	runs, err := m.store.BackupRuns(ctx, 1000)
	if err != nil {
		return err
	}
	missingSet := make(map[string]struct{})
	for _, run := range runs {
		if run.Status != RunStatusSucceeded || !run.Encrypted || len(m.encryptionKeyForID(run.EncryptionKeyID)) == 32 {
			continue
		}
		id := strings.TrimSpace(run.EncryptionKeyID)
		if id == "" {
			id = "<legacy-active>"
		}
		missingSet[id] = struct{}{}
	}
	if len(missingSet) == 0 {
		return nil
	}
	missing := make([]string, 0, len(missingSet))
	for id := range missingSet {
		missing = append(missing, id)
	}
	sort.Strings(missing)
	return fmt.Errorf("missing backup encryption keys: %s", strings.Join(missing, ", "))
}

func (m *Manager) Overview(ctx context.Context, limit int) (Overview, error) {
	policy, err := m.store.BackupPolicy(ctx)
	if err != nil {
		return Overview{}, err
	}
	runs, err := m.store.BackupRuns(ctx, limit)
	if err != nil {
		return Overview{}, err
	}
	for i := range runs {
		runs[i] = m.withEncryptionKeyStatus(runs[i])
	}
	drills, err := m.store.RestoreDrills(ctx, limit)
	if err != nil {
		return Overview{}, err
	}
	cleanupRuns, err := m.store.BackupCleanupRuns(ctx, limit)
	if err != nil {
		return Overview{}, err
	}
	return Overview{Policy: policy, Runs: runs, Drills: drills, CleanupRuns: cleanupRuns, Config: m.ConfigStatus()}, nil
}

func (m *Manager) UpdatePolicy(ctx context.Context, input PolicyUpdate) (Policy, error) {
	input, err := m.ValidatePolicyUpdate(input)
	if err != nil {
		return Policy{}, err
	}
	return m.store.UpdateBackupPolicy(ctx, input)
}

func (m *Manager) ValidatePolicyUpdate(input PolicyUpdate) (PolicyUpdate, error) {
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	if input.Status != PolicyStatusActive && input.Status != PolicyStatusDisabled {
		return PolicyUpdate{}, Invalid("备份策略状态必须是 active 或 disabled")
	}
	if input.IntervalMinutes < 5 || input.IntervalMinutes > 10080 {
		return PolicyUpdate{}, Invalid("备份间隔必须在 5 至 10080 分钟之间")
	}
	if input.RetentionDays < 1 || input.RetentionDays > 3650 {
		return PolicyUpdate{}, Invalid("保留天数必须在 1 至 3650 天之间")
	}
	if input.MinSuccessfulBackups < 1 || input.MinSuccessfulBackups > 365 {
		return PolicyUpdate{}, Invalid("最少成功备份数必须在 1 至 365 之间")
	}
	if input.MaxBackupAgeMinutes < input.IntervalMinutes || input.MaxBackupAgeMinutes > 43200 {
		return PolicyUpdate{}, Invalid("最大备份年龄必须不小于备份间隔且不超过 43200 分钟")
	}
	if input.RestoreDrillIntervalDays < 1 || input.RestoreDrillIntervalDays > 365 {
		return PolicyUpdate{}, Invalid("恢复演练间隔必须在 1 至 365 天之间")
	}
	if input.RequireEncryption && len(m.activeEncryptionKey) != 32 {
		return PolicyUpdate{}, Unavailable("策略要求加密，但 MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY 未配置")
	}
	if input.RequireOffsiteReplica && m.replica == nil {
		return PolicyUpdate{}, Unavailable("策略要求异地副本，但 S3 兼容对象存储未配置")
	}
	return input, nil
}

func (m *Manager) Create(ctx context.Context, trigger string, actor Actor) (BackupRun, error) {
	trigger = normalizeTrigger(trigger)
	policy, err := m.store.BackupPolicy(ctx)
	if err != nil {
		return BackupRun{}, err
	}
	if policy.Status != PolicyStatusActive {
		return BackupRun{}, Conflict("备份策略已停用")
	}
	status := m.ConfigStatus()
	if !status.SourceConfigured || !status.BackupRootConfigured || !status.DumpToolReady {
		return BackupRun{}, Unavailable("备份源、工件目录或 mysqldump 工具未就绪")
	}
	encrypted := len(m.activeEncryptionKey) == 32
	if policy.RequireEncryption && !encrypted {
		return BackupRun{}, Unavailable("备份策略要求加密，但加密密钥未配置")
	}
	if policy.RequireOffsiteReplica && m.replica == nil {
		return BackupRun{}, Unavailable("备份策略要求异地副本，但 S3 兼容对象存储未配置")
	}
	metadata, err := m.store.BackupSourceMetadata(ctx)
	if err != nil {
		return BackupRun{}, fmt.Errorf("read backup source metadata: %w", err)
	}
	backupNo, err := newRunNo("bkp")
	if err != nil {
		return BackupRun{}, err
	}
	extension := ".sql.gz"
	format := ArtifactFormatPlain
	if encrypted {
		extension += ".mgbk"
		format = ArtifactFormatEncrypted
	}
	artifactName := backupNo + extension
	replicaStatus, replicaProvider := ReplicaStatusDisabled, ""
	if m.replica != nil {
		replicaStatus, replicaProvider = ReplicaStatusPending, m.replica.Provider()
	}
	run, err := m.store.StartBackupRun(ctx, BackupStart{
		BackupNo: backupNo, TriggerType: trigger, ArtifactName: artifactName, ArtifactFormat: format,
		Encrypted: encrypted, EncryptionKeyID: m.config.EncryptionKeyID, ReplicaStatus: replicaStatus,
		ReplicaProvider: replicaProvider, Metadata: metadata, StartedAt: time.Now(), Actor: actor,
	})
	if err != nil {
		return BackupRun{}, err
	}
	sha, size, writeErr := m.writeBackupArtifact(ctx, run)
	if writeErr != nil {
		completed, recordErr := m.store.CompleteBackupRun(context.WithoutCancel(ctx), BackupCompletion{
			RunID: run.ID, Status: RunStatusFailed, ErrorMessage: truncateError(writeErr), FinishedAt: time.Now(), Actor: actor,
		})
		if recordErr != nil {
			return completed, fmt.Errorf("create backup: %v; record failure: %w", writeErr, recordErr)
		}
		return completed, writeErr
	}
	run, err = m.store.CompleteBackupRun(context.WithoutCancel(ctx), BackupCompletion{
		RunID: run.ID, Status: RunStatusSucceeded, SHA256: sha, SizeBytes: size, FinishedAt: time.Now(), Actor: actor,
	})
	if err != nil {
		return run, err
	}
	verified, verifyErr := m.Verify(ctx, run.ID, actor)
	if verifyErr != nil {
		return verified, fmt.Errorf("backup created but verification failed: %w", verifyErr)
	}
	if m.replica != nil {
		replicated, replicaErr := m.Replicate(ctx, run.ID, actor)
		if replicaErr != nil {
			return replicated, fmt.Errorf("backup created but offsite replication failed: %w", replicaErr)
		}
		verified = replicated
	}
	return m.withEncryptionKeyStatus(verified), nil
}

func (m *Manager) CreateIfDue(ctx context.Context) (BackupRun, bool, error) {
	policy, err := m.store.BackupPolicy(ctx)
	if err != nil {
		return BackupRun{}, false, err
	}
	if policy.Status != PolicyStatusActive {
		return BackupRun{}, false, nil
	}
	runs, err := m.store.BackupRuns(ctx, 100)
	if err != nil {
		return BackupRun{}, false, err
	}
	for _, run := range runs {
		if run.Status != RunStatusSucceeded || run.VerificationStatus != VerificationPassed {
			continue
		}
		if policy.RequireOffsiteReplica && run.ReplicaStatus != ReplicaStatusSucceeded {
			continue
		}
		finished, parseErr := parseStoreTime(run.FinishedAt)
		if parseErr == nil && time.Since(finished) < time.Duration(policy.IntervalMinutes)*time.Minute {
			return BackupRun{}, false, nil
		}
		break
	}
	run, err := m.Create(ctx, TriggerCron, Actor{})
	return run, err == nil, err
}

func (m *Manager) Verify(ctx context.Context, runID int64, actor Actor) (BackupRun, error) {
	run, err := m.store.BackupRun(ctx, runID)
	if err != nil {
		return BackupRun{}, err
	}
	if run.Status != RunStatusSucceeded {
		return run, Conflict("只有成功备份可以校验")
	}
	if run.CleanupRunID > 0 {
		return run, Conflict("备份已进入保留清理任务，不能再执行校验")
	}
	verifyErr := m.ensureLocalArtifact(ctx, run)
	if verifyErr == nil && run.ReplicaStatus == ReplicaStatusSucceeded {
		if m.replica == nil {
			verifyErr = Unavailable("备份存在异地副本，但当前对象存储未配置")
		} else {
			_, verifyErr = m.replica.Verify(ctx, run.ReplicaObjectKey, run.ReplicaVersionID, run.SHA256, run.SizeBytes)
		}
	}
	verification := BackupVerification{RunID: run.ID, Status: VerificationPassed, VerifiedAt: time.Now(), Actor: actor}
	if verifyErr != nil {
		verification.Status = VerificationFailed
		verification.ErrorMessage = truncateError(verifyErr)
	}
	updated, recordErr := m.store.RecordBackupVerification(context.WithoutCancel(ctx), verification)
	if recordErr != nil {
		return updated, recordErr
	}
	if verifyErr != nil {
		return updated, verifyErr
	}
	return m.withEncryptionKeyStatus(updated), nil
}

func (m *Manager) Replicate(ctx context.Context, runID int64, actor Actor) (BackupRun, error) {
	if m.replica == nil {
		return BackupRun{}, Unavailable("S3 兼容异地副本未配置")
	}
	run, err := m.store.BackupRun(ctx, runID)
	if err != nil {
		return BackupRun{}, err
	}
	if run.Status != RunStatusSucceeded {
		return run, Conflict("只有成功备份可以复制到异地对象存储")
	}
	if run.CleanupRunID > 0 {
		return run, Conflict("备份已进入保留清理任务，不能再复制异地副本")
	}
	replicaInvalid := false
	if run.ReplicaStatus == ReplicaStatusSucceeded {
		if _, err := m.replica.Verify(ctx, run.ReplicaObjectKey, run.ReplicaVersionID, run.SHA256, run.SizeBytes); err == nil {
			return m.withEncryptionKeyStatus(run), nil
		}
		replicaInvalid = true
	}
	if err := m.ensureLocalArtifact(ctx, run); err != nil {
		return run, err
	}
	replicaRemoved := false
	if replicaInvalid {
		if err := m.replica.Delete(ctx, run.ReplicaObjectKey, run.ReplicaVersionID); err != nil {
			return run, fmt.Errorf("remove invalid backup replica before repair: %w", err)
		}
		replicaRemoved = true
	}
	if !replicaRemoved && run.ReplicaStatus == ReplicaStatusFailed && strings.TrimSpace(run.ReplicaObjectKey) != "" {
		if err := m.replica.Delete(ctx, run.ReplicaObjectKey, run.ReplicaVersionID); err != nil {
			return run, fmt.Errorf("remove failed backup replica before retry: %w", err)
		}
	}
	createdAt, parseErr := parseStoreTime(run.CreatedAt)
	if parseErr != nil {
		createdAt = time.Now()
	}
	objectKey := strings.TrimSpace(run.ReplicaObjectKey)
	if objectKey == "" {
		objectKey, err = m.replica.ObjectKey(run.ArtifactName, createdAt)
		if err != nil {
			return run, err
		}
	}
	run, err = m.store.StartBackupReplica(ctx, BackupReplicaStart{
		RunID: run.ID, Provider: m.replica.Provider(), Bucket: m.replica.Bucket(), ObjectKey: objectKey,
		StartedAt: time.Now(), Actor: actor,
	})
	if err != nil {
		return run, err
	}
	fail := func(cause error, uploaded ReplicaObject) (BackupRun, error) {
		replicatedAt := time.Time{}
		if strings.TrimSpace(uploaded.ObjectKey) != "" {
			replicatedAt = time.Now()
		}
		completed, recordErr := m.store.CompleteBackupReplica(context.WithoutCancel(ctx), BackupReplicaCompletion{
			RunID: run.ID, Status: ReplicaStatusFailed, ETag: uploaded.ETag, VersionID: uploaded.VersionID,
			SHA256: uploaded.SHA256, SizeBytes: uploaded.SizeBytes, ReplicatedAt: replicatedAt,
			ErrorMessage: truncateError(cause), Actor: actor,
		})
		if recordErr != nil {
			return completed, fmt.Errorf("replicate backup: %v; record failure: %w", cause, recordErr)
		}
		return completed, cause
	}
	localPath, err := safeArtifactPath(m.config.BackupRoot, run.ArtifactName)
	if err != nil {
		return fail(err, ReplicaObject{})
	}
	uploaded, err := m.replica.Put(ctx, objectKey, localPath, run.SHA256, run.SizeBytes)
	if err != nil {
		return fail(err, uploaded)
	}
	verified, err := m.replica.Verify(ctx, objectKey, uploaded.VersionID, run.SHA256, run.SizeBytes)
	if err != nil {
		return fail(err, uploaded)
	}
	now := time.Now()
	completed, err := m.store.CompleteBackupReplica(context.WithoutCancel(ctx), BackupReplicaCompletion{
		RunID: run.ID, Status: ReplicaStatusSucceeded, ETag: verified.ETag, VersionID: verified.VersionID,
		SHA256: verified.SHA256, SizeBytes: verified.SizeBytes, ReplicatedAt: now, VerifiedAt: now, Actor: actor,
	})
	if err != nil {
		return completed, err
	}
	if completed.VerificationStatus != VerificationPassed {
		completed, err = m.store.RecordBackupVerification(context.WithoutCancel(ctx), BackupVerification{
			RunID: completed.ID, Status: VerificationPassed, VerifiedAt: now, Actor: actor,
		})
		if err != nil {
			return completed, err
		}
	}
	return m.withEncryptionKeyStatus(completed), nil
}

func (m *Manager) RestoreDrill(ctx context.Context, runID int64, trigger string, actor Actor) (RestoreDrill, error) {
	trigger = normalizeTrigger(trigger)
	run, err := m.store.BackupRun(ctx, runID)
	if err != nil {
		return RestoreDrill{}, err
	}
	if run.Status != RunStatusSucceeded {
		return RestoreDrill{}, Conflict("只有成功备份可以执行恢复演练")
	}
	if run.CleanupRunID > 0 {
		return RestoreDrill{}, Conflict("备份已进入保留清理任务，不能再执行恢复演练")
	}
	target, err := m.prepareRestoreTarget(ctx)
	if err != nil {
		return RestoreDrill{}, err
	}
	drillNo, err := newRunNo("rdr")
	if err != nil {
		_ = target.cleanup(context.WithoutCancel(ctx), false, m.config.RestoreKeepOnFailure)
		return RestoreDrill{}, err
	}
	started := time.Now()
	drill, err := m.store.StartRestoreDrill(ctx, RestoreDrillStart{
		DrillNo: drillNo, BackupRun: run, TriggerType: trigger, TargetFingerprint: target.fingerprint,
		TargetDatabase: target.config.DBName, TargetLifecycle: target.lifecycle,
		TargetCleanupStatus: target.initialCleanupStatus(), StartedAt: started, Actor: actor,
	})
	if err != nil {
		cleanupErr := target.cleanup(context.WithoutCancel(ctx), false, m.config.RestoreKeepOnFailure)
		if cleanupErr != nil {
			return RestoreDrill{}, fmt.Errorf("start restore drill: %v; cleanup target: %w", err, cleanupErr)
		}
		return RestoreDrill{}, err
	}
	var targetDB *sql.DB
	fail := func(cause error) (RestoreDrill, error) {
		if targetDB != nil {
			_ = targetDB.Close()
			targetDB = nil
		}
		cleanupStatus, cleanedAt, cleanupErr := target.finishCleanup(context.WithoutCancel(ctx), false, m.config.RestoreKeepOnFailure)
		cleanupMessage := ""
		if cleanupErr != nil {
			cleanupMessage = truncateError(cleanupErr)
			cause = fmt.Errorf("%v; cleanup restore target: %w", cause, cleanupErr)
		}
		completed, recordErr := m.store.CompleteRestoreDrill(context.WithoutCancel(ctx), RestoreDrillCompletion{
			DrillID: drill.ID, Status: DrillStatusFailed, ErrorMessage: truncateError(cause), FinishedAt: time.Now(),
			DurationMS: time.Since(started).Milliseconds(), TargetCleanupStatus: cleanupStatus,
			TargetCleanedAt: cleanedAt, TargetCleanupError: cleanupMessage, Actor: actor,
		})
		if recordErr != nil {
			return completed, fmt.Errorf("restore drill: %v; record failure: %w", cause, recordErr)
		}
		return completed, cause
	}
	if _, verifyErr := m.Verify(ctx, run.ID, actor); verifyErr != nil {
		return fail(fmt.Errorf("verify backup before restore: %w", verifyErr))
	}
	targetDB, err = mysqlconn.OpenConfig(target.config)
	if err != nil {
		return fail(fmt.Errorf("open restore target: %w", err))
	}
	if err := targetDB.PingContext(ctx); err != nil {
		return fail(fmt.Errorf("ping restore target: %w", err))
	}
	tableCount, err := databaseTableCount(ctx, targetDB, target.config.DBName)
	if err != nil {
		return fail(fmt.Errorf("inspect restore target: %w", err))
	}
	if tableCount != 0 {
		return fail(Conflict("恢复演练目标库必须为空，请清理后重试"))
	}
	reader, err := m.openRunSQL(run)
	if err != nil {
		return fail(err)
	}
	commandErr := m.runRestoreCommand(ctx, target.config, reader)
	closeErr := reader.Close()
	if commandErr != nil {
		return fail(commandErr)
	}
	if closeErr != nil {
		return fail(closeErr)
	}
	actual, checks, err := inspectRestoredDatabase(ctx, targetDB, target.config.DBName, run)
	if err != nil {
		return fail(err)
	}
	checksPassed, _ := checks["passed"].(bool)
	statusValue := DrillStatusSucceeded
	errorMessage := ""
	if !checksPassed {
		statusValue = DrillStatusFailed
		errorMessage = "恢复后的迁移版本、表数量或关键表校验未通过"
	}
	if targetDB != nil {
		_ = targetDB.Close()
		targetDB = nil
	}
	cleanupStatus, cleanedAt, cleanupErr := target.finishCleanup(context.WithoutCancel(ctx), statusValue == DrillStatusSucceeded, m.config.RestoreKeepOnFailure)
	cleanupMessage := ""
	if cleanupErr != nil {
		cleanupMessage = truncateError(cleanupErr)
		statusValue = DrillStatusFailed
		errorMessage = "恢复校验完成，但临时目标库销毁失败"
	}
	checks["targetLifecycle"] = target.lifecycle
	checks["targetCleanupStatus"] = cleanupStatus
	checks["targetCleanupPassed"] = cleanupStatus == RestoreCleanupSucceeded || cleanupStatus == RestoreCleanupNotRequired
	completed, err := m.store.CompleteRestoreDrill(context.WithoutCancel(ctx), RestoreDrillCompletion{
		DrillID: drill.ID, Status: statusValue, ActualMigrationVersion: actual.MigrationVersion,
		ActualMigrationCount: actual.MigrationCount, ActualTableCount: actual.TableCount, Checks: checks,
		ErrorMessage: errorMessage, FinishedAt: time.Now(), DurationMS: time.Since(started).Milliseconds(),
		TargetCleanupStatus: cleanupStatus, TargetCleanedAt: cleanedAt, TargetCleanupError: cleanupMessage, Actor: actor,
	})
	if err != nil {
		return completed, err
	}
	if statusValue != DrillStatusSucceeded {
		return completed, Conflict(errorMessage)
	}
	return completed, nil
}

func (m *Manager) PlanCleanup(ctx context.Context) (CleanupPlan, error) {
	policy, err := m.store.BackupPolicy(ctx)
	if err != nil {
		return CleanupPlan{}, err
	}
	runs, err := m.store.BackupRuns(ctx, 1000)
	if err != nil {
		return CleanupPlan{}, err
	}
	sort.SliceStable(runs, func(i, j int) bool {
		left, leftErr := parseStoreTime(runs[i].FinishedAt)
		right, rightErr := parseStoreTime(runs[j].FinishedAt)
		if leftErr == nil && rightErr == nil && !left.Equal(right) {
			return left.After(right)
		}
		return runs[i].ID > runs[j].ID
	})
	plan := CleanupPlan{
		PolicyVersion: policy.Version,
		CutoffAt:      time.Now().Add(-time.Duration(policy.RetentionDays) * 24 * time.Hour),
		Items:         []CleanupPlanItem{},
	}
	successfulPreserved := 0
	for _, run := range runs {
		if run.Status == RunStatusRunning || run.Status == RunStatusDeleted || run.CleanupRunID > 0 || run.ReplicaStatus == ReplicaStatusUploading {
			continue
		}
		plan.Scanned++
		if run.Status == RunStatusSucceeded && successfulPreserved < policy.MinSuccessfulBackups {
			successfulPreserved++
			plan.Preserved++
			continue
		}
		finished, parseErr := parseStoreTime(run.FinishedAt)
		if parseErr != nil || !finished.Before(plan.CutoffAt) {
			continue
		}
		if strings.TrimSpace(run.ArtifactName) != "" {
			if _, pathErr := safeArtifactPath(m.config.BackupRoot, run.ArtifactName); pathErr != nil {
				return CleanupPlan{}, pathErr
			}
		}
		if strings.TrimSpace(run.ReplicaObjectKey) != "" && run.ReplicaStatus != ReplicaStatusDeleted && m.replica == nil {
			return CleanupPlan{}, Unavailable("存在待清理的异地备份副本，但当前对象存储未配置")
		}
		plan.Items = append(plan.Items, CleanupPlanItem{
			BackupRunID: run.ID, BackupRunVersion: run.Version, BackupNo: run.BackupNo, Status: run.Status,
			FinishedAt: run.FinishedAt, ArtifactName: run.ArtifactName, SHA256: run.SHA256, SizeBytes: run.SizeBytes,
			ReplicaStatus: run.ReplicaStatus, ReplicaProvider: run.ReplicaProvider, ReplicaBucket: run.ReplicaBucket,
			ReplicaObjectKey: run.ReplicaObjectKey, ReplicaVersionID: run.ReplicaVersionID,
		})
	}
	if len(plan.Items) == 0 {
		return CleanupPlan{}, Conflict("当前没有满足保留策略的待清理备份")
	}
	return plan, nil
}

func (m *Manager) ScheduleCleanup(ctx context.Context, plan CleanupPlan, actor Actor, approvalID int64, approvalVersion int) (CleanupRun, error) {
	cleanupNo, err := newRunNo("cln")
	if err != nil {
		return CleanupRun{}, err
	}
	scheduled, err := m.store.ScheduleBackupCleanup(ctx, CleanupSchedule{
		CleanupNo: cleanupNo, Plan: plan, Actor: actor,
		ApprovalExecutionID: approvalID, ApprovalExecutionVersion: approvalVersion,
	})
	if err != nil {
		return CleanupRun{}, err
	}
	claimed, items, err := m.store.ClaimBackupCleanup(ctx, scheduled.ID, false, actor)
	if err != nil {
		latest, readErr := m.store.BackupCleanupRun(context.WithoutCancel(ctx), scheduled.ID)
		if readErr == nil {
			return latest, nil
		}
		return scheduled, nil
	}
	processed, err := m.processClaimedCleanup(ctx, claimed, items, actor, 50)
	if err != nil {
		latest, readErr := m.store.BackupCleanupRun(context.WithoutCancel(ctx), scheduled.ID)
		if readErr == nil {
			return latest, nil
		}
		return scheduled, nil
	}
	return processed, nil
}

func (m *Manager) RetryCleanup(ctx context.Context, cleanupRunID int64, actor Actor) (CleanupRun, error) {
	claimed, items, err := m.store.ClaimBackupCleanup(ctx, cleanupRunID, true, actor)
	if err != nil {
		return CleanupRun{}, err
	}
	return m.processClaimedCleanup(ctx, claimed, items, actor, 50)
}

func (m *Manager) ResumeCleanup(ctx context.Context, actor Actor) (CleanupRun, bool, error) {
	claimed, items, found, err := m.store.ClaimNextBackupCleanup(ctx, actor)
	if err != nil || !found {
		return CleanupRun{}, found, err
	}
	processed, err := m.processClaimedCleanup(ctx, claimed, items, actor, 50)
	return processed, true, err
}

func (m *Manager) CheckCleanupQueue(ctx context.Context) error {
	runs, err := m.store.BackupCleanupRuns(ctx, 100)
	if err != nil {
		return err
	}
	for _, run := range runs {
		if run.Status == CleanupStatusFailed || run.Status == CleanupStatusPartial {
			return fmt.Errorf("备份保留清理任务 %s 状态为 %s: %s", run.CleanupNo, run.Status, run.LastError)
		}
		if run.Status == CleanupStatusRunning {
			lease, parseErr := parseStoreTime(run.LeaseExpiresAt)
			if parseErr != nil || !lease.After(time.Now()) {
				return fmt.Errorf("备份保留清理任务 %s 的执行租约已过期", run.CleanupNo)
			}
		}
	}
	return nil
}

func (m *Manager) processClaimedCleanup(ctx context.Context, run CleanupRun, items []CleanupItem, actor Actor, limit int) (CleanupRun, error) {
	processed := 0
	for _, item := range items {
		if item.Status == CleanupItemStatusSucceeded {
			continue
		}
		if processed >= limit {
			break
		}
		if err := ctx.Err(); err != nil {
			return run, err
		}
		if err := m.processCleanupItem(ctx, run, item, actor); err != nil {
			return run, err
		}
		processed++
	}
	return m.store.FinishBackupCleanup(context.WithoutCancel(ctx), CleanupRunCompletion{
		CleanupRunID: run.ID, CleanupAttempt: run.Attempts, Actor: actor,
	})
}

func (m *Manager) processCleanupItem(ctx context.Context, run CleanupRun, item CleanupItem, actor Actor) error {
	checkpoint := func(step, status string, stepErr error) (CleanupItem, error) {
		message := ""
		if stepErr != nil {
			message = truncateError(stepErr)
		}
		return m.store.CheckpointBackupCleanupItem(context.WithoutCancel(ctx), CleanupCheckpoint{
			CleanupRunID: run.ID, CleanupAttempt: run.Attempts, ItemID: item.ID,
			Step: step, Status: status, ErrorMessage: message, Actor: actor,
		})
	}
	if !cleanupStepComplete(item.ReplicaStatus) {
		if m.replica == nil {
			_, err := checkpoint(CleanupStepReplica, CleanupStepFailed, Unavailable("S3 兼容异地副本未配置"))
			return err
		}
		if err := m.replica.Delete(ctx, item.ReplicaObjectKey, item.ReplicaVersionID); err != nil {
			_, checkpointErr := checkpoint(CleanupStepReplica, CleanupStepFailed, err)
			return checkpointErr
		}
		updated, err := checkpoint(CleanupStepReplica, CleanupStepDeleted, nil)
		if err != nil {
			return err
		}
		item = updated
	}
	if item.ReplicaStatus == CleanupStepFailed {
		return nil
	}
	if !cleanupStepComplete(item.LocalStatus) {
		path, err := safeArtifactPath(m.config.BackupRoot, item.ArtifactName)
		if err != nil {
			_, checkpointErr := checkpoint(CleanupStepLocal, CleanupStepFailed, err)
			return checkpointErr
		}
		removeErr := os.Remove(path)
		status := CleanupStepDeleted
		if errors.Is(removeErr, os.ErrNotExist) {
			status, removeErr = CleanupStepMissing, nil
		}
		if removeErr != nil {
			_, checkpointErr := checkpoint(CleanupStepLocal, CleanupStepFailed, fmt.Errorf("delete backup artifact %s: %w", item.BackupNo, removeErr))
			return checkpointErr
		}
		updated, err := checkpoint(CleanupStepLocal, status, nil)
		if err != nil {
			return err
		}
		item = updated
	}
	if item.LocalStatus == CleanupStepFailed {
		return nil
	}
	_, err := m.store.CompleteBackupCleanupItem(context.WithoutCancel(ctx), CleanupItemCompletion{
		CleanupRunID: run.ID, CleanupAttempt: run.Attempts, ItemID: item.ID, Actor: actor,
	})
	return err
}

func cleanupStepComplete(status string) bool {
	return status == CleanupStepDeleted || status == CleanupStepMissing || status == CleanupStepNotRequired
}

func (m *Manager) writeBackupArtifact(ctx context.Context, run BackupRun) (string, int64, error) {
	if err := os.MkdirAll(m.config.BackupRoot, 0o700); err != nil {
		return "", 0, fmt.Errorf("create backup root: %w", err)
	}
	path, err := safeArtifactPath(m.config.BackupRoot, run.ArtifactName)
	if err != nil {
		return "", 0, err
	}
	partial := path + ".partial"
	_ = os.Remove(partial)
	file, err := os.OpenFile(partial, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", 0, fmt.Errorf("create backup artifact: %w", err)
	}
	success := false
	defer func() {
		_ = file.Close()
		if !success {
			_ = os.Remove(partial)
		}
	}()
	hashed := newHashCountingWriter(file)
	var compressedDestination io.Writer = hashed
	var encryptionWriter *chunkEncryptWriter
	if run.Encrypted {
		if len(m.activeEncryptionKey) != 32 {
			return "", 0, Unavailable("当前备份加密密钥未配置或长度无效")
		}
		encryptionWriter, err = newChunkEncryptWriter(hashed, m.activeEncryptionKey)
		if err != nil {
			return "", 0, err
		}
		compressedDestination = encryptionWriter
	}
	gzipWriter, err := gzip.NewWriterLevel(compressedDestination, gzip.BestSpeed)
	if err != nil {
		return "", 0, err
	}
	commandErr := m.runDumpCommand(ctx, m.sourceConfig, gzipWriter)
	gzipErr := gzipWriter.Close()
	var encryptionErr error
	if encryptionWriter != nil {
		encryptionErr = encryptionWriter.Close()
	}
	if commandErr != nil {
		return "", 0, commandErr
	}
	if gzipErr != nil {
		return "", 0, fmt.Errorf("finish backup compression: %w", gzipErr)
	}
	if encryptionErr != nil {
		return "", 0, fmt.Errorf("finish backup encryption: %w", encryptionErr)
	}
	if err := file.Sync(); err != nil {
		return "", 0, fmt.Errorf("sync backup artifact: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", 0, fmt.Errorf("close backup artifact: %w", err)
	}
	if err := os.Rename(partial, path); err != nil {
		return "", 0, fmt.Errorf("publish backup artifact: %w", err)
	}
	success = true
	return hashed.Sum(), hashed.count, nil
}

func (m *Manager) verifyArtifact(run BackupRun) error {
	path, err := safeArtifactPath(m.config.BackupRoot, run.ArtifactName)
	if err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("stat backup artifact: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("backup artifact is not a regular file")
	}
	sha, size, err := hashArtifact(path)
	if err != nil {
		return fmt.Errorf("hash backup artifact: %w", err)
	}
	if !strings.EqualFold(sha, run.SHA256) || size != run.SizeBytes {
		return fmt.Errorf("backup artifact checksum mismatch")
	}
	reader, err := m.openRunSQL(run)
	if err != nil {
		return err
	}
	defer reader.Close()
	decompressed, err := io.Copy(io.Discard, reader)
	if err != nil {
		return fmt.Errorf("read backup SQL stream: %w", err)
	}
	if decompressed < 128 {
		return fmt.Errorf("backup SQL stream is unexpectedly small")
	}
	return nil
}

func (m *Manager) ensureLocalArtifact(ctx context.Context, run BackupRun) error {
	localErr := m.verifyArtifact(run)
	if localErr == nil {
		return nil
	}
	if run.ReplicaStatus != ReplicaStatusSucceeded || strings.TrimSpace(run.ReplicaObjectKey) == "" || m.replica == nil {
		return localErr
	}
	if err := os.MkdirAll(m.config.BackupRoot, 0o700); err != nil {
		return fmt.Errorf("create backup root before replica hydration: %w", err)
	}
	path, err := safeArtifactPath(m.config.BackupRoot, run.ArtifactName)
	if err != nil {
		return err
	}
	partial := path + ".hydrate.partial"
	_ = os.Remove(partial)
	file, err := os.OpenFile(partial, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create hydrated backup artifact: %w", err)
	}
	success := false
	defer func() {
		_ = file.Close()
		if !success {
			_ = os.Remove(partial)
		}
	}()
	metadata, reader, err := m.replica.Open(ctx, run.ReplicaObjectKey, run.ReplicaVersionID)
	if err != nil {
		return err
	}
	hashed := newHashCountingWriter(file)
	_, copyErr := io.Copy(hashed, reader)
	closeErr := reader.Close()
	if copyErr != nil {
		return fmt.Errorf("hydrate backup from replica: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close backup replica stream: %w", closeErr)
	}
	if hashed.count != run.SizeBytes || metadata.SizeBytes != run.SizeBytes || !strings.EqualFold(hashed.Sum(), run.SHA256) {
		return fmt.Errorf("hydrated backup replica checksum mismatch")
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync hydrated backup artifact: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close hydrated backup artifact: %w", err)
	}
	if err := os.Rename(partial, path); err != nil {
		return fmt.Errorf("publish hydrated backup artifact: %w", err)
	}
	success = true
	if err := m.verifyArtifact(run); err != nil {
		return fmt.Errorf("verify hydrated backup artifact: %w", err)
	}
	return nil
}

func (m *Manager) openRunSQL(run BackupRun) (io.ReadCloser, error) {
	path, err := safeArtifactPath(m.config.BackupRoot, run.ArtifactName)
	if err != nil {
		return nil, err
	}
	key := m.encryptionKeyForID(run.EncryptionKeyID)
	if run.Encrypted && len(key) != 32 {
		return nil, Unavailable("备份加密密钥 " + strings.TrimSpace(run.EncryptionKeyID) + " 未配置")
	}
	return openArtifactSQL(path, run.Encrypted, key)
}

func (m *Manager) encryptionKeyForID(id string) []byte {
	id = strings.TrimSpace(id)
	if id == "" {
		return m.activeEncryptionKey
	}
	return m.encryptionKeys[id]
}

func (m *Manager) withEncryptionKeyStatus(run BackupRun) BackupRun {
	run.EncryptionKeyReady = !run.Encrypted || len(m.encryptionKeyForID(run.EncryptionKeyID)) == 32
	return run
}

type restoreTarget struct {
	config      *mysqldriver.Config
	fingerprint string
	lifecycle   string
	adminDB     *sql.DB
}

func (m *Manager) prepareRestoreTarget(ctx context.Context) (restoreTarget, error) {
	status := m.ConfigStatus()
	if !status.RestoreConfigured || !status.RestoreToolReady {
		return restoreTarget{}, Unavailable("隔离恢复目标或 mysql 客户端未配置")
	}
	if m.config.RestoreAutoProvision {
		adminConfig := *m.restoreAdminConfig
		adminConnectionConfig := adminConfig
		adminConnectionConfig.DBName = ""
		adminDB, err := mysqlconn.OpenConfig(&adminConnectionConfig)
		if err != nil {
			return restoreTarget{}, fmt.Errorf("open restore admin connection: %w", err)
		}
		if err := adminDB.PingContext(ctx); err != nil {
			adminDB.Close()
			return restoreTarget{}, fmt.Errorf("ping restore admin connection: %w", err)
		}
		runNo, err := newRunNo("db")
		if err != nil {
			adminDB.Close()
			return restoreTarget{}, err
		}
		targetDB := m.config.RestoreDatabasePrefix + strings.TrimPrefix(runNo, "db_")
		if len(targetDB) > 128 || !regexp.MustCompile(`^[A-Za-z0-9_]+$`).MatchString(targetDB) {
			adminDB.Close()
			return restoreTarget{}, Invalid("自动恢复目标数据库名无效")
		}
		if _, err := adminDB.ExecContext(ctx, "CREATE DATABASE `"+targetDB+"` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
			adminDB.Close()
			return restoreTarget{}, fmt.Errorf("create ephemeral restore database: %w", err)
		}
		adminConfig.DBName = targetDB
		identity := strings.ToLower(adminConfig.Net + "|" + adminConfig.Addr + "|" + targetDB)
		fingerprint := sha256.Sum256([]byte(identity))
		return restoreTarget{
			config: &adminConfig, fingerprint: hex.EncodeToString(fingerprint[:]),
			lifecycle: RestoreTargetLifecycleEphemeral, adminDB: adminDB,
		}, nil
	}
	targetDB := strings.TrimSpace(m.restoreConfig.DBName)
	if !strings.HasPrefix(targetDB, m.config.RestoreDatabasePrefix) {
		return restoreTarget{}, Invalid("恢复目标数据库必须使用前缀 " + m.config.RestoreDatabasePrefix)
	}
	if m.sourceConfig != nil && sameDatabaseTarget(m.sourceConfig, m.restoreConfig) {
		return restoreTarget{}, Invalid("恢复目标不能与源数据库相同")
	}
	identity := strings.ToLower(m.restoreConfig.Net + "|" + m.restoreConfig.Addr + "|" + targetDB)
	fingerprint := sha256.Sum256([]byte(identity))
	return restoreTarget{
		config: m.restoreConfig, fingerprint: hex.EncodeToString(fingerprint[:]),
		lifecycle: RestoreTargetLifecyclePreconfigured,
	}, nil
}

func (t *restoreTarget) initialCleanupStatus() string {
	if t.lifecycle == RestoreTargetLifecycleEphemeral {
		return RestoreCleanupPending
	}
	return RestoreCleanupNotRequired
}

func (t *restoreTarget) cleanup(ctx context.Context, succeeded, keepOnFailure bool) error {
	_, _, err := t.finishCleanup(ctx, succeeded, keepOnFailure)
	return err
}

func (t *restoreTarget) finishCleanup(ctx context.Context, succeeded, keepOnFailure bool) (string, time.Time, error) {
	if t.lifecycle != RestoreTargetLifecycleEphemeral {
		return RestoreCleanupNotRequired, time.Time{}, nil
	}
	if !succeeded && keepOnFailure {
		if t.adminDB != nil {
			_ = t.adminDB.Close()
			t.adminDB = nil
		}
		return RestoreCleanupRetained, time.Time{}, nil
	}
	if t.adminDB == nil {
		return RestoreCleanupFailed, time.Time{}, fmt.Errorf("restore admin connection is unavailable for cleanup")
	}
	cleanupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	_, err := t.adminDB.ExecContext(cleanupCtx, "DROP DATABASE IF EXISTS `"+t.config.DBName+"`")
	closeErr := t.adminDB.Close()
	t.adminDB = nil
	if err != nil {
		return RestoreCleanupFailed, time.Time{}, fmt.Errorf("drop ephemeral restore database: %w", err)
	}
	if closeErr != nil {
		return RestoreCleanupFailed, time.Time{}, fmt.Errorf("close restore admin connection: %w", closeErr)
	}
	return RestoreCleanupSucceeded, time.Now(), nil
}

func (m *Manager) runDumpCommand(ctx context.Context, config *mysqldriver.Config, output io.Writer) error {
	defaultsFile, err := mysqlDefaultsFile(config)
	if err != nil {
		return err
	}
	defer os.Remove(defaultsFile)
	args := []string{
		"--defaults-extra-file=" + defaultsFile,
		"--single-transaction", "--quick", "--routines", "--events", "--triggers", "--hex-blob",
		"--default-character-set=utf8mb4", "--skip-lock-tables", "--no-tablespaces", "--add-drop-table", config.DBName,
	}
	command := exec.CommandContext(ctx, m.config.DumpBinary, args...)
	command.Stdout = output
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("mysqldump failed: %w: %s", err, singleLine(stderr.String()))
	}
	return nil
}

func (m *Manager) runRestoreCommand(ctx context.Context, config *mysqldriver.Config, input io.Reader) error {
	defaultsFile, err := mysqlDefaultsFile(config)
	if err != nil {
		return err
	}
	defer os.Remove(defaultsFile)
	command := exec.CommandContext(ctx, m.config.RestoreBinary,
		"--defaults-extra-file="+defaultsFile, "--default-character-set=utf8mb4", config.DBName)
	command.Stdin = input
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("mysql restore failed: %w: %s", err, singleLine(stderr.String()))
	}
	return nil
}

func mysqlDefaultsFile(config *mysqldriver.Config) (string, error) {
	if config == nil || strings.TrimSpace(config.DBName) == "" {
		return "", Invalid("MySQL DSN 未配置数据库名")
	}
	file, err := os.CreateTemp("", "mochat-mysql-client-*.cnf")
	if err != nil {
		return "", err
	}
	name := file.Name()
	cleanup := true
	defer func() {
		_ = file.Close()
		if cleanup {
			_ = os.Remove(name)
		}
	}()
	if err := file.Chmod(0o600); err != nil {
		return "", err
	}
	lines := []string{"[client]", "user=" + quoteOptionValue(config.User), "password=" + quoteOptionValue(config.Passwd)}
	if strings.TrimSpace(config.TLSConfig) == "" || strings.EqualFold(strings.TrimSpace(config.TLSConfig), "false") {
		// Match go-sql-driver's non-TLS default. MariaDB 11 clients otherwise may require SSL automatically.
		lines = append(lines, "ssl=0")
	} else {
		lines = append(lines, "ssl=1")
	}
	switch config.Net {
	case "", "tcp", "tcp4", "tcp6":
		host, port, err := net.SplitHostPort(config.Addr)
		if err != nil {
			return "", fmt.Errorf("parse MySQL TCP address: %w", err)
		}
		lines = append(lines, "protocol=tcp", "host="+quoteOptionValue(host), "port="+port)
	case "unix":
		lines = append(lines, "protocol=socket", "socket="+quoteOptionValue(config.Addr))
	default:
		return "", Invalid("备份只支持 tcp 或 unix MySQL DSN")
	}
	lines = append(lines, "default-character-set=utf8mb4", "")
	if _, err := file.WriteString(strings.Join(lines, "\n")); err != nil {
		return "", err
	}
	if err := file.Sync(); err != nil {
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	cleanup = false
	return name, nil
}

func quoteOptionValue(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	value = strings.ReplaceAll(value, "\n", `\n`)
	value = strings.ReplaceAll(value, "\r", `\r`)
	return `"` + value + `"`
}

func inspectRestoredDatabase(ctx context.Context, db *sql.DB, databaseName string, expected BackupRun) (SourceMetadata, map[string]any, error) {
	actual, err := queryDatabaseMetadata(ctx, db, databaseName)
	if err != nil {
		return SourceMetadata{}, nil, err
	}
	criticalTables := []string{"mc_tenant", "mc_user", "mochat_go_schema_migrations", "mochat_go_saas_backup_runs"}
	missing := make([]string, 0)
	for _, table := range criticalTables {
		var count int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?`, databaseName, table).Scan(&count); err != nil {
			return SourceMetadata{}, nil, err
		}
		if count != 1 {
			missing = append(missing, table)
		}
	}
	passed := actual.MigrationVersion == expected.MigrationVersion && actual.MigrationCount == expected.MigrationCount &&
		actual.TableCount == expected.TableCount && len(missing) == 0
	checks := map[string]any{
		"passed": passed, "migrationVersionMatched": actual.MigrationVersion == expected.MigrationVersion,
		"migrationCountMatched": actual.MigrationCount == expected.MigrationCount,
		"tableCountMatched":     actual.TableCount == expected.TableCount, "missingCriticalTables": missing,
	}
	return actual, checks, nil
}

func queryDatabaseMetadata(ctx context.Context, db *sql.DB, databaseName string) (SourceMetadata, error) {
	metadata := SourceMetadata{DatabaseName: databaseName}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = ?`, databaseName).Scan(&metadata.TableCount); err != nil {
		return SourceMetadata{}, err
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_schema_migrations`).Scan(&metadata.MigrationCount); err != nil {
		return SourceMetadata{}, err
	}
	err := db.QueryRowContext(ctx, `SELECT version FROM mochat_go_schema_migrations ORDER BY applied_at DESC, version DESC LIMIT 1`).Scan(&metadata.MigrationVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return SourceMetadata{}, fmt.Errorf("schema migration ledger is empty")
	}
	return metadata, err
}

func databaseTableCount(ctx context.Context, db *sql.DB, databaseName string) (int, error) {
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = ?`, databaseName).Scan(&count)
	return count, err
}

func sameDatabaseTarget(left, right *mysqldriver.Config) bool {
	if left == nil || right == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(left.Net), strings.TrimSpace(right.Net)) &&
		strings.EqualFold(strings.TrimSpace(left.Addr), strings.TrimSpace(right.Addr)) &&
		strings.EqualFold(strings.TrimSpace(left.DBName), strings.TrimSpace(right.DBName))
}

func newRunNo(prefix string) (string, error) {
	random := make([]byte, 6)
	if _, err := io.ReadFull(rand.Reader, random); err != nil {
		return "", err
	}
	return prefix + "_" + time.Now().UTC().Format("20060102T150405Z") + "_" + hex.EncodeToString(random), nil
}

func normalizeTrigger(trigger string) string {
	switch strings.ToLower(strings.TrimSpace(trigger)) {
	case TriggerCron:
		return TriggerCron
	case TriggerMaintenance:
		return TriggerMaintenance
	default:
		return TriggerManual
	}
}

func parseStoreTime(value string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(value), time.Local)
}

func truncateError(err error) string {
	if err == nil {
		return ""
	}
	value := singleLine(err.Error())
	if len([]rune(value)) > 1000 {
		value = string([]rune(value)[:1000])
	}
	return value
}

func singleLine(value string) string {
	value = strings.NewReplacer("\r", " ", "\n", " ", "\t", " ").Replace(value)
	return strings.Join(strings.Fields(value), " ")
}

func MarshalChecks(value map[string]any) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}

func ArtifactPath(root string, run BackupRun) (string, error) {
	return safeArtifactPath(root, run.ArtifactName)
}

func ArtifactBaseName(path string) string { return filepath.Base(path) }

func FormatBytes(value int64) string {
	if value < 1024 {
		return strconv.FormatInt(value, 10) + " B"
	}
	return fmt.Sprintf("%.1f MiB", float64(value)/(1024*1024))
}
