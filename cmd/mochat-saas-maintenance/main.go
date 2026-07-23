package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/config"
	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/identitysecurity"
	"jiyi/mochat-go/internal/mysqlconn"
	"jiyi/mochat-go/internal/outboundhttp"
	"jiyi/mochat-go/internal/saasalertcredentials"
	"jiyi/mochat-go/internal/saasauditanchor"
	"jiyi/mochat-go/internal/saasbackup"
	"jiyi/mochat-go/internal/saascompliance"
	"jiyi/mochat-go/internal/store"
	"jiyi/mochat-go/internal/wechatopencredentials"
	"jiyi/mochat-go/internal/wecomcredentials"
)

type maintenanceOptions struct {
	Action                                  string
	DSN                                     string
	StorageRoot                             string
	TenantID                                int
	Timeout                                 time.Duration
	AlertStatus                             string
	AlertMetric                             string
	AlertType                               string
	AlertLimit                              int
	WebhookURL                              string
	WebhookSecret                           string
	WebhookTimeout                          time.Duration
	WebhookRetryAttempts                    int
	WebhookRetryDelay                       time.Duration
	WebhookTitleTemplate                    string
	WebhookBodyTemplate                     string
	WebhookRequireHTTPS                     bool
	WebhookAllowedCIDRs                     string
	AlertNotificationRetryDelay             time.Duration
	AlertCredentialEncryptionKey            string
	AlertCredentialEncryptionKeys           string
	AlertCredentialEncryptionKeyID          string
	AlertCredentialRequireEncryption        bool
	AlertCredentialDedicatedConfigured      bool
	AlertCredentialRotationLimit            int
	WeComCredentialEncryptionKey            string
	WeComCredentialEncryptionKeys           string
	WeComCredentialEncryptionKeyID          string
	WeComCredentialRequireEncryption        bool
	WeComCredentialDedicatedConfigured      bool
	WeComCredentialRotationLimit            int
	WeChatOpenCredentialEncryptionKey       string
	WeChatOpenCredentialEncryptionKeys      string
	WeChatOpenCredentialEncryptionKeyID     string
	WeChatOpenCredentialRequireEncryption   bool
	WeChatOpenCredentialDedicatedConfigured bool
	WeChatOpenCredentialRotationLimit       int
	BackupRoot                              string
	BackupEncryptionKey                     string
	BackupEncryptionKeys                    string
	BackupEncryptionKeyID                   string
	BackupDumpBinary                        string
	BackupRestoreBinary                     string
	BackupRestoreDSN                        string
	BackupRestoreAdminDSN                   string
	BackupRestoreAutoProvision              bool
	BackupRestoreKeepOnFailure              bool
	BackupRestoreDatabasePrefix             string
	BackupS3Endpoint                        string
	BackupS3Bucket                          string
	BackupS3Region                          string
	BackupS3AccessKeyID                     string
	BackupS3SecretAccessKey                 string
	BackupS3SessionToken                    string
	BackupS3UseSSL                          bool
	BackupS3Prefix                          string
	BackupRunID                             int64
	ComplianceArtifactRoot                  string
	ComplianceEncryptionKey                 string
	ComplianceEncryptionKeys                string
	ComplianceEncryptionKeyID               string
	ComplianceID                            int64
	ComplianceLimit                         int
	IdentityEncryptionKey                   string
	IdentityEncryptionKeys                  string
	IdentityEncryptionKeyID                 string
	IdentityCleanupLimit                    int
	ServiceAccountID                        int64
	ServiceAccountUsageAlertLimit           int
	ServiceAccountUsageCleanupLimit         int
	AuditIntegrityLimit                     int
	AuditAnchorArtifactRoot                 string
	AuditAnchorHMACKey                      string
	AuditAnchorHMACKeys                     string
	AuditAnchorHMACKeyID                    string
	AuditAnchorLimit                        int
	AuditAnchorRequireRemote                bool
	AuditAnchorS3Endpoint                   string
	AuditAnchorS3Bucket                     string
	AuditAnchorS3Region                     string
	AuditAnchorS3AccessKeyID                string
	AuditAnchorS3SecretAccessKey            string
	AuditAnchorS3SessionToken               string
	AuditAnchorS3UseSSL                     bool
	AuditAnchorS3Prefix                     string
	AuditAnchorS3RetentionMode              string
	AuditAnchorS3RetentionDays              int
	PlatformTenantID                        int
}

func main() {
	alertCredentialDefaults, alertCredentialDedicatedConfigured := alertCredentialEncryptionDefaultsFromEnv()
	weComCredentialDefaults, weComCredentialDedicatedConfigured := weComCredentialEncryptionDefaultsFromEnv()
	weChatOpenCredentialDefaults, weChatOpenCredentialDedicatedConfigured := weChatOpenCredentialEncryptionDefaultsFromEnv()
	options := maintenanceOptions{
		AlertCredentialDedicatedConfigured:      alertCredentialDedicatedConfigured,
		WeComCredentialDedicatedConfigured:      weComCredentialDedicatedConfigured,
		WeChatOpenCredentialDedicatedConfigured: weChatOpenCredentialDedicatedConfigured,
	}
	flag.StringVar(&options.Action, "action", envDefault("MOCHAT_SAAS_MAINTENANCE_ACTION", "reconcile-storage"), "maintenance action: reconcile-storage, refresh-usage, list-alerts, resolve-alert, dispatch-alert-notifications, rotate-alert-credentials, rotate-wecom-credentials, rotate-wechat-open-credentials, backup-status, backup-create, backup-verify, backup-replicate, backup-restore-drill, backup-cleanup, compliance-status, compliance-process, compliance-export-process, compliance-erasure-process, identity-cleanup, evaluate-service-account-usage-alerts, cleanup-service-account-usage, verify-audit-integrity, create-audit-anchor, verify-audit-anchor")
	flag.StringVar(&options.DSN, "dsn", os.Getenv("MOCHAT_MYSQL_DSN"), "MySQL DSN; defaults to MOCHAT_MYSQL_DSN")
	flag.StringVar(&options.StorageRoot, "storage-root", firstEnv("MOCHAT_FILE_STORAGE_ROOT", "FILE_STORAGE_ROOT", "MOCHAT_SAAS_STORAGE_ROOT"), "upload storage root; defaults to MOCHAT_FILE_STORAGE_ROOT, FILE_STORAGE_ROOT, or ./storage/upload/static")
	flag.IntVar(&options.TenantID, "tenant-id", intEnv("MOCHAT_SAAS_TENANT_ID", 0), "optional tenant id filter; 0 reconciles all tenants")
	flag.DurationVar(&options.Timeout, "timeout", 30*time.Minute, "maintenance timeout")
	flag.StringVar(&options.AlertStatus, "alert-status", envDefault("MOCHAT_SAAS_ALERT_STATUS", "open"), "alert status filter for list-alerts; use all for every status")
	flag.StringVar(&options.AlertMetric, "metric", os.Getenv("MOCHAT_SAAS_ALERT_METRIC"), "SaaS metric filter for list-alerts or resolve-alert")
	flag.StringVar(&options.AlertType, "alert-type", envDefault("MOCHAT_SAAS_ALERT_TYPE", "quota_exceeded"), "alert type filter for list-alerts or resolve-alert")
	flag.IntVar(&options.AlertLimit, "alert-limit", intEnv("MOCHAT_SAAS_ALERT_LIMIT", 50), "maximum alerts returned by list-alerts")
	flag.StringVar(&options.WebhookURL, "alert-webhook-url", os.Getenv("MOCHAT_GO_SAAS_ALERT_WEBHOOK_URL"), "SaaS alert webhook URL for dispatch-alert-notifications")
	flag.StringVar(&options.WebhookSecret, "alert-webhook-secret", os.Getenv("MOCHAT_GO_SAAS_ALERT_WEBHOOK_SECRET"), "SaaS alert webhook signing secret")
	flag.DurationVar(&options.WebhookTimeout, "alert-webhook-timeout", time.Duration(intEnv("MOCHAT_GO_SAAS_ALERT_WEBHOOK_TIMEOUT_SECONDS", 5))*time.Second, "SaaS alert webhook timeout")
	flag.IntVar(&options.WebhookRetryAttempts, "alert-webhook-retry-attempts", intEnv("MOCHAT_GO_SAAS_ALERT_WEBHOOK_RETRY_ATTEMPTS", 1), "SaaS alert webhook HTTP retry attempts")
	flag.DurationVar(&options.WebhookRetryDelay, "alert-webhook-retry-delay", time.Duration(intEnv("MOCHAT_GO_SAAS_ALERT_WEBHOOK_RETRY_DELAY_MS", 250))*time.Millisecond, "SaaS alert webhook HTTP retry delay")
	flag.StringVar(&options.WebhookTitleTemplate, "alert-webhook-title-template", envDefault("MOCHAT_GO_SAAS_ALERT_WEBHOOK_TITLE_TEMPLATE", config.DefaultSaaSAlertWebhookTitleTemplate), "SaaS alert webhook title template")
	flag.StringVar(&options.WebhookBodyTemplate, "alert-webhook-body-template", envDefault("MOCHAT_GO_SAAS_ALERT_WEBHOOK_BODY_TEMPLATE", config.DefaultSaaSAlertWebhookBodyTemplate), "SaaS alert webhook body template")
	flag.BoolVar(&options.WebhookRequireHTTPS, "alert-webhook-require-https", boolEnv("MOCHAT_GO_SAAS_ALERT_WEBHOOK_REQUIRE_HTTPS", true), "require HTTPS for SaaS alert webhook destinations")
	flag.StringVar(&options.WebhookAllowedCIDRs, "alert-webhook-allowed-cidrs", os.Getenv("MOCHAT_GO_SAAS_ALERT_WEBHOOK_ALLOWED_CIDRS"), "explicit CIDR exceptions for SaaS alert webhook destinations")
	flag.DurationVar(&options.AlertNotificationRetryDelay, "alert-notification-retry-delay", time.Duration(intEnv("MOCHAT_GO_SAAS_ALERT_NOTIFICATION_RETRY_DELAY_SECONDS", 300))*time.Second, "SaaS alert notification outbox retry delay after dispatch failure")
	flag.StringVar(&options.AlertCredentialEncryptionKey, "alert-credential-encryption-key", alertCredentialDefaults.Key, "32-byte SaaS alert credential encryption master key")
	flag.StringVar(&options.AlertCredentialEncryptionKeys, "alert-credential-encryption-keys", alertCredentialDefaults.Keys, "JSON object containing active and historical SaaS alert credential master keys")
	flag.StringVar(&options.AlertCredentialEncryptionKeyID, "alert-credential-encryption-key-id", alertCredentialDefaults.KeyID, "non-secret SaaS alert credential encryption key identifier")
	flag.BoolVar(&options.AlertCredentialRequireEncryption, "alert-credential-require-encryption", boolEnv("MOCHAT_GO_SAAS_ALERT_CREDENTIAL_REQUIRE_ENCRYPTION", false), "reject plaintext SaaS alert credential writes when no encryption key is configured")
	flag.IntVar(&options.AlertCredentialRotationLimit, "alert-credential-rotation-limit", intEnv("MOCHAT_GO_SAAS_ALERT_CREDENTIAL_ROTATION_LIMIT", 100), "maximum SaaS alert credentials rotated in one batch")
	flag.StringVar(&options.WeComCredentialEncryptionKey, "wecom-credential-encryption-key", weComCredentialDefaults.Key, "32-byte WeCom credential encryption master key")
	flag.StringVar(&options.WeComCredentialEncryptionKeys, "wecom-credential-encryption-keys", weComCredentialDefaults.Keys, "JSON object containing active and historical WeCom credential master keys")
	flag.StringVar(&options.WeComCredentialEncryptionKeyID, "wecom-credential-encryption-key-id", weComCredentialDefaults.KeyID, "non-secret WeCom credential encryption key identifier")
	flag.BoolVar(&options.WeComCredentialRequireEncryption, "wecom-credential-require-encryption", boolEnv("MOCHAT_GO_WECOM_CREDENTIAL_REQUIRE_ENCRYPTION", false), "reject plaintext WeCom credential writes when no encryption key is configured")
	flag.IntVar(&options.WeComCredentialRotationLimit, "wecom-credential-rotation-limit", intEnv("MOCHAT_GO_WECOM_CREDENTIAL_ROTATION_LIMIT", 100), "maximum WeCom credentials rotated in one batch")
	flag.StringVar(&options.WeChatOpenCredentialEncryptionKey, "wechat-open-credential-encryption-key", weChatOpenCredentialDefaults.Key, "32-byte WeChat Open credential encryption master key")
	flag.StringVar(&options.WeChatOpenCredentialEncryptionKeys, "wechat-open-credential-encryption-keys", weChatOpenCredentialDefaults.Keys, "JSON object containing active and historical WeChat Open credential master keys")
	flag.StringVar(&options.WeChatOpenCredentialEncryptionKeyID, "wechat-open-credential-encryption-key-id", weChatOpenCredentialDefaults.KeyID, "non-secret WeChat Open credential encryption key identifier")
	flag.BoolVar(&options.WeChatOpenCredentialRequireEncryption, "wechat-open-credential-require-encryption", boolEnv("MOCHAT_GO_WECHAT_OPEN_CREDENTIAL_REQUIRE_ENCRYPTION", false), "reject plaintext WeChat Open credential writes when no encryption key is configured")
	flag.IntVar(&options.WeChatOpenCredentialRotationLimit, "wechat-open-credential-rotation-limit", intEnv("MOCHAT_GO_WECHAT_OPEN_CREDENTIAL_ROTATION_LIMIT", 100), "maximum WeChat Open credentials rotated in one batch")
	flag.StringVar(&options.BackupRoot, "backup-root", envDefault("MOCHAT_GO_SAAS_BACKUP_ROOT", "./storage/backups"), "backup artifact root")
	flag.StringVar(&options.BackupEncryptionKey, "backup-encryption-key", os.Getenv("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY"), "32-byte backup encryption key encoded as base64 or hex")
	flag.StringVar(&options.BackupEncryptionKeys, "backup-encryption-keys", os.Getenv("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEYS"), "JSON object containing active and historical backup encryption keys")
	flag.StringVar(&options.BackupEncryptionKeyID, "backup-encryption-key-id", envDefault("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY_ID", "primary"), "non-secret backup encryption key identifier")
	flag.StringVar(&options.BackupDumpBinary, "backup-dump-binary", envDefault("MOCHAT_GO_SAAS_BACKUP_DUMP_BINARY", "mysqldump"), "mysqldump-compatible binary")
	flag.StringVar(&options.BackupRestoreBinary, "backup-restore-binary", envDefault("MOCHAT_GO_SAAS_BACKUP_RESTORE_BINARY", "mysql"), "mysql-compatible restore binary")
	flag.StringVar(&options.BackupRestoreDSN, "backup-restore-dsn", os.Getenv("MOCHAT_GO_SAAS_BACKUP_RESTORE_DSN"), "isolated restore drill MySQL DSN")
	flag.StringVar(&options.BackupRestoreAdminDSN, "backup-restore-admin-dsn", os.Getenv("MOCHAT_GO_SAAS_BACKUP_RESTORE_ADMIN_DSN"), "server-level MySQL DSN for ephemeral restore databases")
	flag.BoolVar(&options.BackupRestoreAutoProvision, "backup-restore-auto-provision", boolEnv("MOCHAT_GO_SAAS_BACKUP_RESTORE_AUTO_PROVISION", false), "create and destroy an ephemeral restore database")
	flag.BoolVar(&options.BackupRestoreKeepOnFailure, "backup-restore-keep-on-failure", boolEnv("MOCHAT_GO_SAAS_BACKUP_RESTORE_KEEP_ON_FAILURE", false), "retain an ephemeral restore database after a failed drill")
	flag.StringVar(&options.BackupRestoreDatabasePrefix, "backup-restore-database-prefix", envDefault("MOCHAT_GO_SAAS_BACKUP_RESTORE_DATABASE_PREFIX", "mochat_restore_"), "required restore drill database prefix")
	flag.StringVar(&options.BackupS3Endpoint, "backup-s3-endpoint", os.Getenv("MOCHAT_GO_SAAS_BACKUP_S3_ENDPOINT"), "S3-compatible backup replica endpoint")
	flag.StringVar(&options.BackupS3Bucket, "backup-s3-bucket", os.Getenv("MOCHAT_GO_SAAS_BACKUP_S3_BUCKET"), "S3-compatible backup replica bucket")
	flag.StringVar(&options.BackupS3Region, "backup-s3-region", os.Getenv("MOCHAT_GO_SAAS_BACKUP_S3_REGION"), "S3-compatible backup replica region")
	flag.StringVar(&options.BackupS3AccessKeyID, "backup-s3-access-key-id", os.Getenv("MOCHAT_GO_SAAS_BACKUP_S3_ACCESS_KEY_ID"), "S3-compatible backup replica access key id")
	flag.StringVar(&options.BackupS3SecretAccessKey, "backup-s3-secret-access-key", os.Getenv("MOCHAT_GO_SAAS_BACKUP_S3_SECRET_ACCESS_KEY"), "S3-compatible backup replica secret access key")
	flag.StringVar(&options.BackupS3SessionToken, "backup-s3-session-token", os.Getenv("MOCHAT_GO_SAAS_BACKUP_S3_SESSION_TOKEN"), "S3-compatible backup replica session token")
	flag.BoolVar(&options.BackupS3UseSSL, "backup-s3-use-ssl", boolEnv("MOCHAT_GO_SAAS_BACKUP_S3_USE_SSL", true), "use TLS for an S3 endpoint without a URL scheme")
	flag.StringVar(&options.BackupS3Prefix, "backup-s3-prefix", envDefault("MOCHAT_GO_SAAS_BACKUP_S3_PREFIX", "mochat-go/backups"), "S3-compatible backup replica object prefix")
	flag.Int64Var(&options.BackupRunID, "backup-run-id", int64Env("MOCHAT_GO_SAAS_BACKUP_RUN_ID", 0), "backup run id for verify or restore drill")
	flag.StringVar(&options.ComplianceArtifactRoot, "compliance-artifact-root", envDefault("MOCHAT_GO_SAAS_COMPLIANCE_ARTIFACT_ROOT", "./storage/compliance"), "tenant data compliance artifact root")
	flag.StringVar(&options.ComplianceEncryptionKey, "compliance-encryption-key", firstEnv("MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY", "MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY"), "32-byte compliance export encryption master key")
	flag.StringVar(&options.ComplianceEncryptionKeys, "compliance-encryption-keys", firstEnv("MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEYS", "MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEYS"), "JSON object containing active and historical compliance export master keys")
	flag.StringVar(&options.ComplianceEncryptionKeyID, "compliance-encryption-key-id", envDefault("MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY_ID", envDefault("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY_ID", "primary")), "non-secret compliance export key identifier")
	flag.Int64Var(&options.ComplianceID, "compliance-id", int64Env("MOCHAT_GO_SAAS_COMPLIANCE_ID", 0), "export or erasure request id for an explicit compliance action")
	flag.IntVar(&options.ComplianceLimit, "compliance-limit", intEnv("MOCHAT_GO_SAAS_COMPLIANCE_LIMIT", 10), "maximum pending compliance jobs processed")
	flag.StringVar(&options.IdentityEncryptionKey, "identity-encryption-key", firstEnv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY", "MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY", "MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY"), "32-byte identity MFA encryption master key")
	flag.StringVar(&options.IdentityEncryptionKeys, "identity-encryption-keys", firstEnv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEYS", "MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEYS", "MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEYS"), "JSON object containing active and historical identity MFA master keys")
	flag.StringVar(&options.IdentityEncryptionKeyID, "identity-encryption-key-id", envDefault("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY_ID", envDefault("MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY_ID", envDefault("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY_ID", "primary"))), "non-secret identity MFA key identifier")
	flag.IntVar(&options.IdentityCleanupLimit, "identity-cleanup-limit", intEnv("MOCHAT_GO_SAAS_IDENTITY_CLEANUP_LIMIT", 1000), "maximum identity rows processed per cleanup stage")
	flag.Int64Var(&options.ServiceAccountID, "service-account-id", int64Env("MOCHAT_GO_SAAS_SERVICE_ACCOUNT_ID", 0), "optional service account id filter")
	flag.IntVar(&options.ServiceAccountUsageAlertLimit, "service-account-usage-alert-limit", intEnv("MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_ALERT_LIMIT", 100), "maximum service accounts evaluated for usage alerts")
	flag.IntVar(&options.ServiceAccountUsageCleanupLimit, "service-account-usage-cleanup-limit", intEnv("MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_CLEANUP_LIMIT", 10000), "maximum service account usage rows deleted per cleanup run")
	flag.IntVar(&options.AuditIntegrityLimit, "audit-integrity-limit", intEnv("MOCHAT_GO_SAAS_AUDIT_INTEGRITY_LIMIT", 100), "maximum tenant audit chains verified")
	flag.StringVar(&options.AuditAnchorArtifactRoot, "audit-anchor-artifact-root", envDefault("MOCHAT_GO_SAAS_AUDIT_ANCHOR_ARTIFACT_ROOT", "./storage/audit-anchors"), "independent audit anchor artifact root")
	flag.StringVar(&options.AuditAnchorHMACKey, "audit-anchor-hmac-key", os.Getenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_HMAC_KEY"), "32-byte audit anchor HMAC key encoded as base64 or hex")
	flag.StringVar(&options.AuditAnchorHMACKeys, "audit-anchor-hmac-keys", os.Getenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_HMAC_KEYS"), "JSON object containing active and historical audit anchor HMAC keys")
	flag.StringVar(&options.AuditAnchorHMACKeyID, "audit-anchor-hmac-key-id", envDefault("MOCHAT_GO_SAAS_AUDIT_ANCHOR_HMAC_KEY_ID", "primary"), "non-secret audit anchor HMAC key identifier")
	flag.IntVar(&options.AuditAnchorLimit, "audit-anchor-limit", intEnv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_LIMIT", 100), "maximum audit chains or checkpoints processed")
	flag.BoolVar(&options.AuditAnchorRequireRemote, "audit-anchor-require-remote", boolEnv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_REQUIRE_REMOTE", false), "require immutable remote audit anchor evidence")
	flag.StringVar(&options.AuditAnchorS3Endpoint, "audit-anchor-s3-endpoint", os.Getenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_ENDPOINT"), "S3-compatible audit anchor endpoint")
	flag.StringVar(&options.AuditAnchorS3Bucket, "audit-anchor-s3-bucket", os.Getenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_BUCKET"), "S3-compatible audit anchor bucket with Object Lock")
	flag.StringVar(&options.AuditAnchorS3Region, "audit-anchor-s3-region", os.Getenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_REGION"), "S3-compatible audit anchor region")
	flag.StringVar(&options.AuditAnchorS3AccessKeyID, "audit-anchor-s3-access-key-id", os.Getenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_ACCESS_KEY_ID"), "S3-compatible audit anchor access key id")
	flag.StringVar(&options.AuditAnchorS3SecretAccessKey, "audit-anchor-s3-secret-access-key", os.Getenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_SECRET_ACCESS_KEY"), "S3-compatible audit anchor secret access key")
	flag.StringVar(&options.AuditAnchorS3SessionToken, "audit-anchor-s3-session-token", os.Getenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_SESSION_TOKEN"), "optional S3-compatible audit anchor session token")
	flag.BoolVar(&options.AuditAnchorS3UseSSL, "audit-anchor-s3-use-ssl", boolEnv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_USE_SSL", true), "use TLS for S3-compatible audit anchor storage")
	flag.StringVar(&options.AuditAnchorS3Prefix, "audit-anchor-s3-prefix", envDefault("MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_PREFIX", "mochat-go/audit-anchors"), "S3-compatible audit anchor object prefix")
	flag.StringVar(&options.AuditAnchorS3RetentionMode, "audit-anchor-s3-retention-mode", envDefault("MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_RETENTION_MODE", "compliance"), "S3 Object Lock retention mode: compliance or governance")
	flag.IntVar(&options.AuditAnchorS3RetentionDays, "audit-anchor-s3-retention-days", intEnv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_RETENTION_DAYS", 3650), "S3 Object Lock retention days")
	flag.IntVar(&options.PlatformTenantID, "platform-tenant-id", intEnv("MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID", 1), "platform admin tenant id")
	flag.Parse()
	visitedFlags := make(map[string]bool)
	flag.Visit(func(value *flag.Flag) {
		visitedFlags[value.Name] = true
	})
	applyAlertCredentialFlagSelection(&options, alertCredentialDedicatedConfigured, visitedFlags)
	applyWeComCredentialFlagSelection(&options, weComCredentialDedicatedConfigured, visitedFlags)
	applyWeChatOpenCredentialFlagSelection(&options, weChatOpenCredentialDedicatedConfigured, visitedFlags)

	if err := run(options); err != nil {
		log.Fatal(err)
	}
}

func run(options maintenanceOptions) error {
	if strings.TrimSpace(options.DSN) == "" {
		return fmt.Errorf("MOCHAT_MYSQL_DSN or -dsn is required")
	}
	action := strings.TrimSpace(options.Action)
	switch action {
	case "reconcile-storage":
		if strings.TrimSpace(options.StorageRoot) == "" {
			options.StorageRoot = "./storage/upload/static"
		}
	case "refresh-usage":
	case "list-alerts":
		if strings.EqualFold(strings.TrimSpace(options.AlertStatus), "all") {
			options.AlertStatus = ""
		}
		if strings.EqualFold(strings.TrimSpace(options.AlertType), "all") {
			options.AlertType = ""
		}
	case "resolve-alert":
		if options.TenantID <= 0 {
			return fmt.Errorf("-tenant-id is required for resolve-alert")
		}
		if strings.TrimSpace(options.AlertMetric) == "" {
			return fmt.Errorf("-metric is required for resolve-alert")
		}
	case "dispatch-alert-notifications":
		if options.WebhookTimeout <= 0 {
			return fmt.Errorf("-alert-webhook-timeout must be positive")
		}
		if options.WebhookRetryAttempts <= 0 {
			return fmt.Errorf("-alert-webhook-retry-attempts must be positive")
		}
		if options.WebhookRetryDelay < 0 || options.AlertNotificationRetryDelay < 0 {
			return fmt.Errorf("retry delays must be non-negative")
		}
	case "rotate-alert-credentials":
		if options.TenantID < 0 || options.AlertCredentialRotationLimit <= 0 || options.AlertCredentialRotationLimit > 1000 {
			return fmt.Errorf("-tenant-id must be non-negative and -alert-credential-rotation-limit must be between 1 and 1000")
		}
	case "rotate-wecom-credentials":
		if options.TenantID < 0 || options.WeComCredentialRotationLimit <= 0 || options.WeComCredentialRotationLimit > 1000 {
			return fmt.Errorf("-tenant-id must be non-negative and -wecom-credential-rotation-limit must be between 1 and 1000")
		}
	case "rotate-wechat-open-credentials":
		if options.TenantID < 0 || options.WeChatOpenCredentialRotationLimit <= 0 || options.WeChatOpenCredentialRotationLimit > 1000 {
			return fmt.Errorf("-tenant-id must be non-negative and -wechat-open-credential-rotation-limit must be between 1 and 1000")
		}
	case "backup-status", "backup-create", "backup-cleanup":
	case "backup-verify", "backup-replicate", "backup-restore-drill":
		if options.BackupRunID <= 0 {
			return fmt.Errorf("-backup-run-id is required for %s", action)
		}
	case "compliance-status", "compliance-process":
		if strings.TrimSpace(options.StorageRoot) == "" {
			options.StorageRoot = "./storage/upload/static"
		}
		if options.ComplianceLimit <= 0 || options.ComplianceLimit > 100 {
			return fmt.Errorf("-compliance-limit must be between 1 and 100")
		}
	case "compliance-export-process", "compliance-erasure-process":
		if strings.TrimSpace(options.StorageRoot) == "" {
			options.StorageRoot = "./storage/upload/static"
		}
		if options.ComplianceID <= 0 {
			return fmt.Errorf("-compliance-id is required for %s", action)
		}
	case "identity-cleanup":
		if options.IdentityCleanupLimit <= 0 || options.IdentityCleanupLimit > 5000 {
			return fmt.Errorf("-identity-cleanup-limit must be between 1 and 5000")
		}
	case "evaluate-service-account-usage-alerts":
		if options.TenantID < 0 || options.ServiceAccountID < 0 {
			return fmt.Errorf("-tenant-id and -service-account-id cannot be negative")
		}
		if options.ServiceAccountUsageAlertLimit <= 0 || options.ServiceAccountUsageAlertLimit > 500 {
			return fmt.Errorf("-service-account-usage-alert-limit must be between 1 and 500")
		}
	case "cleanup-service-account-usage":
		if options.ServiceAccountUsageCleanupLimit <= 0 || options.ServiceAccountUsageCleanupLimit > 1000000 {
			return fmt.Errorf("-service-account-usage-cleanup-limit must be between 1 and 1000000")
		}
	case "verify-audit-integrity":
		if options.TenantID < 0 || options.AuditIntegrityLimit <= 0 || options.AuditIntegrityLimit > 500 {
			return fmt.Errorf("-tenant-id must be non-negative and -audit-integrity-limit must be between 1 and 500")
		}
	case "create-audit-anchor", "verify-audit-anchor":
		if options.TenantID < 0 || options.AuditAnchorLimit <= 0 || options.AuditAnchorLimit > 500 {
			return fmt.Errorf("-tenant-id must be non-negative and -audit-anchor-limit must be between 1 and 500")
		}
		if options.AuditAnchorS3RetentionDays < 1 || options.AuditAnchorS3RetentionDays > 36500 {
			return fmt.Errorf("-audit-anchor-s3-retention-days must be between 1 and 36500")
		}
	default:
		return fmt.Errorf("unknown maintenance action %q", options.Action)
	}

	db, err := mysqlconn.Open(options.DSN)
	if err != nil {
		return fmt.Errorf("open mysql: %w", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		return fmt.Errorf("ping mysql: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), options.Timeout)
	defer cancel()

	alertCredentialManager, err := saasalertcredentials.NewManager(saasalertcredentials.Config{
		EncryptionKey:       options.AlertCredentialEncryptionKey,
		EncryptionKeys:      options.AlertCredentialEncryptionKeys,
		EncryptionKeyID:     options.AlertCredentialEncryptionKeyID,
		RequireEncryption:   options.AlertCredentialRequireEncryption,
		DedicatedConfigured: options.AlertCredentialDedicatedConfigured,
	})
	if err != nil {
		return fmt.Errorf("build SaaS alert credential encryption manager: %w", err)
	}
	weComCredentialManager, err := wecomcredentials.NewManager(wecomcredentials.Config{
		EncryptionKey:       options.WeComCredentialEncryptionKey,
		EncryptionKeys:      options.WeComCredentialEncryptionKeys,
		EncryptionKeyID:     options.WeComCredentialEncryptionKeyID,
		RequireEncryption:   options.WeComCredentialRequireEncryption,
		DedicatedConfigured: options.WeComCredentialDedicatedConfigured,
	})
	if err != nil {
		return fmt.Errorf("build WeCom credential encryption manager: %w", err)
	}
	weChatOpenCredentialManager, err := wechatopencredentials.NewManager(wechatopencredentials.Config{
		EncryptionKey:       options.WeChatOpenCredentialEncryptionKey,
		EncryptionKeys:      options.WeChatOpenCredentialEncryptionKeys,
		EncryptionKeyID:     options.WeChatOpenCredentialEncryptionKeyID,
		RequireEncryption:   options.WeChatOpenCredentialRequireEncryption,
		DedicatedConfigured: options.WeChatOpenCredentialDedicatedConfigured,
	})
	if err != nil {
		return fmt.Errorf("build WeChat Open credential encryption manager: %w", err)
	}
	mysqlStore := store.NewMySQLStore(db).
		WithSaaSAlertCredentialCipher(alertCredentialManager).
		WithWeComCredentialCipher(weComCredentialManager).
		WithWeChatOpenCredentialCipher(weChatOpenCredentialManager)
	var backupManager *saasbackup.Manager
	if strings.HasPrefix(action, "backup-") {
		var backupReplica saasbackup.ReplicaStore
		if strings.TrimSpace(options.BackupS3Endpoint) != "" {
			backupReplica, err = saasbackup.NewS3ReplicaStore(saasbackup.S3ReplicaConfig{
				Endpoint: options.BackupS3Endpoint, Bucket: options.BackupS3Bucket, Region: options.BackupS3Region,
				AccessKeyID: options.BackupS3AccessKeyID, SecretAccessKey: options.BackupS3SecretAccessKey,
				SessionToken: options.BackupS3SessionToken, UseSSL: options.BackupS3UseSSL, Prefix: options.BackupS3Prefix,
			})
			if err != nil {
				return fmt.Errorf("build backup replica store: %w", err)
			}
		}
		backupManager, err = saasbackup.NewManager(mysqlStore, saasbackup.Config{
			SourceDSN: options.DSN, BackupRoot: options.BackupRoot, EncryptionKey: options.BackupEncryptionKey,
			EncryptionKeys:  options.BackupEncryptionKeys,
			EncryptionKeyID: options.BackupEncryptionKeyID, DumpBinary: options.BackupDumpBinary,
			RestoreBinary: options.BackupRestoreBinary, RestoreDSN: options.BackupRestoreDSN,
			RestoreAdminDSN: options.BackupRestoreAdminDSN, RestoreAutoProvision: options.BackupRestoreAutoProvision,
			RestoreKeepOnFailure:  options.BackupRestoreKeepOnFailure,
			RestoreDatabasePrefix: options.BackupRestoreDatabasePrefix, Replica: backupReplica,
		})
		if err != nil {
			return fmt.Errorf("build backup manager: %w", err)
		}
	}
	var complianceManager *saascompliance.Manager
	if strings.HasPrefix(action, "compliance-") {
		complianceManager, err = saascompliance.NewManager(mysqlStore, saascompliance.Config{
			ArtifactRoot: options.ComplianceArtifactRoot, FileStorageRoot: options.StorageRoot,
			EncryptionKey: options.ComplianceEncryptionKey, EncryptionKeys: options.ComplianceEncryptionKeys,
			EncryptionKeyID: options.ComplianceEncryptionKeyID, PlatformTenantID: options.PlatformTenantID,
		})
		if err != nil {
			return fmt.Errorf("build compliance manager: %w", err)
		}
	}
	var identityManager *identitysecurity.Manager
	if action == "identity-cleanup" {
		identityManager, err = identitysecurity.NewManager(mysqlStore, identitysecurity.Config{
			EncryptionKey: options.IdentityEncryptionKey, EncryptionKeys: options.IdentityEncryptionKeys,
			EncryptionKeyID: options.IdentityEncryptionKeyID,
		})
		if err != nil {
			return fmt.Errorf("build identity security manager: %w", err)
		}
	}
	var auditAnchorManager *saasauditanchor.Manager
	if action == "create-audit-anchor" || action == "verify-audit-anchor" {
		var remote saasauditanchor.RemoteArtifactStore
		if strings.TrimSpace(options.AuditAnchorS3Endpoint) != "" {
			remote, err = saasauditanchor.NewS3RemoteArtifactStore(saasauditanchor.S3RemoteArtifactConfig{
				Endpoint: options.AuditAnchorS3Endpoint, Bucket: options.AuditAnchorS3Bucket,
				Region: options.AuditAnchorS3Region, AccessKeyID: options.AuditAnchorS3AccessKeyID,
				SecretAccessKey: options.AuditAnchorS3SecretAccessKey, SessionToken: options.AuditAnchorS3SessionToken,
				UseSSL: options.AuditAnchorS3UseSSL, Prefix: options.AuditAnchorS3Prefix,
				RetentionMode: options.AuditAnchorS3RetentionMode, RetentionDays: options.AuditAnchorS3RetentionDays,
			})
			if err != nil {
				return fmt.Errorf("build audit anchor remote store: %w", err)
			}
		}
		auditAnchorManager, err = saasauditanchor.NewManager(mysqlStore, saasauditanchor.Config{
			ArtifactRoot: options.AuditAnchorArtifactRoot, HMACKey: options.AuditAnchorHMACKey,
			HMACKeys: options.AuditAnchorHMACKeys, HMACKeyID: options.AuditAnchorHMACKeyID,
			Remote: remote, RequireRemote: options.AuditAnchorRequireRemote,
		})
		if err != nil {
			return fmt.Errorf("build audit anchor manager: %w", err)
		}
	}
	switch action {
	case "reconcile-storage":
		result, err := mysqlStore.ReconcileSaaSStorageObjects(ctx, options.StorageRoot, options.TenantID)
		if err != nil {
			return fmt.Errorf("reconcile storage: %w", err)
		}
		printStorageReconcileResult(result)
	case "refresh-usage":
		result, err := mysqlStore.RefreshSaaSUsageCounters(ctx, options.TenantID)
		if err != nil {
			return fmt.Errorf("refresh usage: %w", err)
		}
		printUsageRefreshResult(result)
	case "list-alerts":
		page, err := mysqlStore.ListSaaSAlerts(ctx, store.SaaSAlertListOptions{
			TenantID:  options.TenantID,
			Status:    options.AlertStatus,
			Metric:    options.AlertMetric,
			AlertType: options.AlertType,
			Page:      1,
			PerPage:   options.AlertLimit,
		})
		if err != nil {
			return fmt.Errorf("list alerts: %w", err)
		}
		printAlertListResult(options, page)
	case "resolve-alert":
		resolved, err := mysqlStore.ResolveSaaSAlert(ctx, options.TenantID, options.AlertMetric, options.AlertType)
		if err != nil {
			return fmt.Errorf("resolve alert: %w", err)
		}
		printAlertResolveResult(options, resolved)
	case "dispatch-alert-notifications":
		webhookGuard, err := outboundhttp.NewGuard(outboundhttp.Config{
			RequireHTTPS: options.WebhookRequireHTTPS,
			AllowedCIDRs: []string{options.WebhookAllowedCIDRs},
		})
		if err != nil {
			return fmt.Errorf("build alert webhook egress guard: %w", err)
		}
		var fallback dashboard.SaaSAlertNotifier
		if strings.TrimSpace(options.WebhookURL) != "" {
			webhookNotifier, err := dashboard.NewSaaSAlertWebhookNotifierWithTemplates(
				options.WebhookURL,
				options.WebhookTimeout,
				options.WebhookSecret,
				options.WebhookTitleTemplate,
				options.WebhookBodyTemplate,
				webhookGuard,
			)
			if err != nil {
				return fmt.Errorf("build alert webhook notifier: %w", err)
			}
			fallback = dashboard.NewRetryingSaaSAlertNotifier(webhookNotifier, options.WebhookRetryAttempts, options.WebhookRetryDelay)
		}
		notifier := dashboard.NewStoreBackedSaaSAlertNotifier(mysqlStore, fallback, dashboard.SaaSAlertNotificationChannelWebhook, webhookGuard)
		result, err := dashboard.DispatchDueSaaSAlertNotifications(ctx, mysqlStore, notifier, options.AlertLimit, options.AlertNotificationRetryDelay)
		if err != nil {
			return fmt.Errorf("dispatch alert notifications: %w", err)
		}
		printAlertNotificationDispatchResult(result)
	case "rotate-alert-credentials":
		result, err := mysqlStore.RotateSaaSAlertCredentials(ctx, options.TenantID, options.AlertCredentialRotationLimit)
		if err != nil {
			return fmt.Errorf("rotate SaaS alert credentials: %w", err)
		}
		printAlertCredentialRotation(result)
	case "rotate-wecom-credentials":
		result, err := mysqlStore.RotateSaaSWeComCredentials(ctx, options.TenantID, options.WeComCredentialRotationLimit)
		if err != nil {
			return fmt.Errorf("rotate WeCom credentials: %w", err)
		}
		printWeComCredentialRotation(result)
	case "rotate-wechat-open-credentials":
		result, err := mysqlStore.RotateSaaSWeChatOpenCredentials(ctx, options.TenantID, options.WeChatOpenCredentialRotationLimit)
		if err != nil {
			return fmt.Errorf("rotate WeChat Open credentials: %w", err)
		}
		printWeChatOpenCredentialRotation(result)
	case "backup-status":
		overview, err := backupManager.Overview(ctx, options.AlertLimit)
		if err != nil {
			return fmt.Errorf("read backup status: %w", err)
		}
		printBackupOverview(overview)
	case "backup-create":
		run, err := backupManager.Create(ctx, saasbackup.TriggerMaintenance, saasbackup.Actor{})
		if err != nil {
			return fmt.Errorf("create backup: %w", err)
		}
		printBackupRun("backup-create", run)
	case "backup-verify":
		run, err := backupManager.Verify(ctx, options.BackupRunID, saasbackup.Actor{})
		if err != nil {
			return fmt.Errorf("verify backup: %w", err)
		}
		printBackupRun("backup-verify", run)
	case "backup-replicate":
		run, err := backupManager.Replicate(ctx, options.BackupRunID, saasbackup.Actor{})
		if err != nil {
			return fmt.Errorf("replicate backup: %w", err)
		}
		printBackupRun("backup-replicate", run)
	case "backup-restore-drill":
		drill, err := backupManager.RestoreDrill(ctx, options.BackupRunID, saasbackup.TriggerMaintenance, saasbackup.Actor{})
		if err != nil {
			return fmt.Errorf("restore drill: %w", err)
		}
		printRestoreDrill(drill)
	case "backup-cleanup":
		result, resumed, err := backupManager.ResumeCleanup(ctx, saasbackup.Actor{})
		if err != nil {
			return fmt.Errorf("resume approved backup cleanup: %w", err)
		}
		printBackupCleanup(result, resumed)
	case "compliance-status":
		overview, err := complianceManager.Overview(ctx, options.TenantID, options.ComplianceLimit)
		if err != nil {
			return fmt.Errorf("read compliance status: %w", err)
		}
		printComplianceOverview(overview)
	case "compliance-process":
		result, err := complianceManager.ProcessPending(ctx, options.ComplianceLimit, saascompliance.Actor{TenantID: options.PlatformTenantID})
		if err != nil {
			return fmt.Errorf("process compliance queue: %w", err)
		}
		printComplianceProcess(result)
	case "compliance-export-process":
		item, err := complianceManager.ProcessExport(ctx, options.ComplianceID, saascompliance.Actor{TenantID: options.PlatformTenantID})
		if err != nil {
			return fmt.Errorf("process compliance export: %w", err)
		}
		fmt.Printf("action\tcompliance-export-process\nexport_id\t%d\nstatus\t%s\nsha256\t%s\n", item.ID, item.Status, item.SHA256)
	case "compliance-erasure-process":
		item, err := complianceManager.ProcessErasure(ctx, options.ComplianceID, saascompliance.Actor{TenantID: options.PlatformTenantID})
		if err != nil {
			return fmt.Errorf("process compliance erasure: %w", err)
		}
		fmt.Printf("action\tcompliance-erasure-process\nrequest_id\t%d\nstatus\t%s\nverification_sha256\t%s\n", item.ID, item.Status, item.VerificationSHA256)
	case "identity-cleanup":
		result, err := identityManager.Cleanup(ctx, options.IdentityCleanupLimit)
		if err != nil {
			return fmt.Errorf("cleanup identity security data: %w", err)
		}
		fmt.Printf("action\tidentity-cleanup\nexpired_challenges\t%d\nexpired_sessions\t%d\ndeleted_challenges\t%d\ndeleted_sessions\t%d\ndeleted_events\t%d\n",
			result.ExpiredChallenges, result.ExpiredSessions, result.DeletedChallenges, result.DeletedSessions, result.DeletedEvents)
	case "evaluate-service-account-usage-alerts":
		result, err := mysqlStore.EvaluateSaaSServiceAccountUsageAlerts(ctx, dashboard.SaaSServiceAccountUsageAlertEvaluateOptions{
			TenantID: options.TenantID, ServiceAccountID: options.ServiceAccountID, Limit: options.ServiceAccountUsageAlertLimit, Source: "maintenance",
		})
		if err != nil {
			return fmt.Errorf("evaluate service account usage alerts: %w", err)
		}
		printServiceAccountUsageAlertEvaluation(result)
	case "cleanup-service-account-usage":
		result, err := mysqlStore.CleanupSaaSServiceAccountUsage(ctx, options.ServiceAccountUsageCleanupLimit)
		if err != nil {
			return fmt.Errorf("cleanup service account usage: %w", err)
		}
		printServiceAccountUsageCleanup(result)
	case "verify-audit-integrity":
		result, err := mysqlStore.VerifySaaSAdminAuditIntegrity(ctx, dashboard.SaaSAdminAuditIntegrityOptions{
			TenantID: options.TenantID, Limit: options.AuditIntegrityLimit, Source: "maintenance",
		})
		if err != nil {
			return fmt.Errorf("verify audit integrity: %w", err)
		}
		printAuditIntegrityVerification(result)
		if result.FailedChains > 0 {
			return fmt.Errorf("audit integrity verification detected %d failed chains", result.FailedChains)
		}
	case "create-audit-anchor":
		result, err := auditAnchorManager.Create(ctx, saasauditanchor.CreateOptions{
			TenantID: options.TenantID, Limit: options.AuditAnchorLimit, Source: saasauditanchor.TriggerMaintenance,
			Actor: saasauditanchor.Actor{TenantID: options.PlatformTenantID},
		})
		if err != nil {
			return fmt.Errorf("create audit anchor: %w", err)
		}
		printAuditAnchorCreate(result)
	case "verify-audit-anchor":
		result, err := auditAnchorManager.Verify(ctx, saasauditanchor.VerifyOptions{
			TenantID: options.TenantID, Limit: options.AuditAnchorLimit, Source: saasauditanchor.TriggerMaintenance,
			Actor: saasauditanchor.Actor{TenantID: options.PlatformTenantID},
		})
		if err != nil {
			return fmt.Errorf("verify audit anchor: %w", err)
		}
		printAuditAnchorVerification(result)
		if result.FailedCheckpoints > 0 {
			return fmt.Errorf("audit anchor verification detected %d failed checkpoints", result.FailedCheckpoints)
		}
	}
	return nil
}

func printServiceAccountUsageAlertEvaluation(result dashboard.SaaSServiceAccountUsageAlertEvaluateResult) {
	fmt.Printf("action\tevaluate-service-account-usage-alerts\n")
	fmt.Printf("scanned_accounts\t%d\n", result.ScannedAccounts)
	fmt.Printf("eligible_accounts\t%d\n", result.EligibleAccounts)
	fmt.Printf("disabled_policies\t%d\n", result.DisabledPolicies)
	fmt.Printf("usage_warning_accounts\t%d\n", result.UsageWarningAccounts)
	fmt.Printf("rejection_warning_accounts\t%d\n", result.RejectionWarningAccounts)
	fmt.Printf("resolved_alerts\t%d\n", result.ResolvedAlerts)
	fmt.Printf("notifications_queued\t%d\n", result.NotificationsQueued)
	fmt.Printf("notifications_closed\t%d\n", result.NotificationsClosed)
	fmt.Printf("evaluated_at\t%s\n", result.EvaluatedAt)
}

func printServiceAccountUsageCleanup(result dashboard.SaaSServiceAccountUsageCleanupResult) {
	fmt.Printf("action\tcleanup-service-account-usage\n")
	fmt.Printf("retention_days\t%d\n", result.RetentionDays)
	fmt.Printf("cutoff_date\t%s\n", result.CutoffDate)
	fmt.Printf("eligible_rows\t%d\n", result.EligibleRows)
	fmt.Printf("protected_rows\t%d\n", result.ProtectedRows)
	fmt.Printf("deleted_rows\t%d\n", result.DeletedRows)
	fmt.Printf("remaining_rows\t%d\n", result.RemainingRows)
}

func printAuditIntegrityVerification(result dashboard.SaaSAdminAuditIntegrityVerifyResult) {
	fmt.Printf("action\tverify-audit-integrity\n")
	fmt.Printf("scanned_chains\t%d\n", result.ScannedChains)
	fmt.Printf("sealed_chains\t%d\n", result.SealedChains)
	fmt.Printf("healthy_chains\t%d\n", result.HealthyChains)
	fmt.Printf("failed_chains\t%d\n", result.FailedChains)
	fmt.Printf("legacy_logs\t%d\n", result.LegacyLogs)
	fmt.Printf("signed_logs\t%d\n", result.SignedLogs)
	fmt.Printf("verified_logs\t%d\n", result.VerifiedLogs)
	fmt.Printf("verified_at\t%s\n", result.VerifiedAt)
}

func printAuditAnchorCreate(result saasauditanchor.CreateResult) {
	fmt.Printf("action\tcreate-audit-anchor\n")
	fmt.Printf("scanned_chains\t%d\n", result.ScannedChains)
	fmt.Printf("created_checkpoints\t%d\n", result.CreatedCheckpoints)
	fmt.Printf("existing_checkpoints\t%d\n", result.ExistingCheckpoints)
	fmt.Printf("backfilled_checkpoints\t%d\n", result.BackfilledCheckpoints)
	fmt.Printf("exported_artifacts\t%d\n", result.ExportedArtifacts)
	fmt.Printf("failed_artifacts\t%d\n", result.FailedArtifacts)
	fmt.Printf("remote_exported_artifacts\t%d\n", result.RemoteExportedArtifacts)
	fmt.Printf("remote_failed_artifacts\t%d\n", result.RemoteFailedArtifacts)
	fmt.Printf("operation_id\t%d\n", result.OperationID)
	fmt.Printf("created_at\t%s\n", result.CreatedAt)
}

func printAuditAnchorVerification(result saasauditanchor.VerifyResult) {
	fmt.Printf("action\tverify-audit-anchor\n")
	fmt.Printf("scanned_checkpoints\t%d\n", result.ScannedCheckpoints)
	fmt.Printf("passed_checkpoints\t%d\n", result.PassedCheckpoints)
	fmt.Printf("failed_checkpoints\t%d\n", result.FailedCheckpoints)
	fmt.Printf("missing_keys\t%d\n", result.MissingKeyCount)
	fmt.Printf("missing_artifacts\t%d\n", result.MissingArtifactCount)
	fmt.Printf("orphan_artifacts\t%d\n", result.OrphanArtifactCount)
	fmt.Printf("missing_remote_artifacts\t%d\n", result.MissingRemoteCount)
	fmt.Printf("orphan_remote_artifacts\t%d\n", result.OrphanRemoteCount)
	fmt.Printf("operation_id\t%d\n", result.OperationID)
	fmt.Printf("verified_at\t%s\n", result.VerifiedAt)
}

func printStorageReconcileResult(result store.SaaSStorageReconcileResult) {
	fmt.Printf("action\treconcile-storage\n")
	fmt.Printf("tenant_id\t%d\n", result.TenantID)
	fmt.Printf("scanned\t%d\n", result.Scanned)
	fmt.Printf("missing_marked\t%d\n", result.MissingMarked)
	fmt.Printf("unsafe_marked\t%d\n", result.UnsafeMarked)
	fmt.Printf("size_updated\t%d\n", result.SizeUpdated)
	fmt.Printf("counters_refreshed\t%d\n", result.CountersRefreshed)
	fmt.Printf("refreshed_tenants\t%s\n", joinInts(result.RefreshedTenants))
}

func printUsageRefreshResult(result store.SaaSUsageRefreshResult) {
	fmt.Printf("action\trefresh-usage\n")
	fmt.Printf("tenant_id\t%d\n", result.TenantID)
	fmt.Printf("tenants_scanned\t%d\n", result.TenantsScanned)
	fmt.Printf("metrics_refreshed\t%d\n", result.MetricsRefreshed)
	fmt.Printf("refreshed_tenants\t%s\n", joinInts(result.RefreshedTenants))
}

func printAlertListResult(options maintenanceOptions, page store.SaaSAlertListPage) {
	status := strings.TrimSpace(options.AlertStatus)
	if status == "" {
		status = "all"
	}
	fmt.Printf("action\tlist-alerts\n")
	fmt.Printf("tenant_id\t%d\n", options.TenantID)
	fmt.Printf("status\t%s\n", status)
	fmt.Printf("alerts\t%d\n", len(page.Items))
	fmt.Printf("total\t%d\n", page.Total)
	fmt.Printf("id\ttenant_id\tstatus\tseverity\tmetric\tcurrent\tlimit\toccurrences\tlast_seen_at\tsource\tmessage\n")
	for _, alert := range page.Items {
		fmt.Printf("%d\t%d\t%s\t%s\t%s\t%d\t%d\t%d\t%s\t%s\t%s\n",
			alert.ID,
			alert.TenantID,
			alert.Status,
			alert.Severity,
			alert.Metric,
			alert.CurrentValue,
			alert.LimitValue,
			alert.OccurrenceCount,
			alert.LastSeenAt,
			alert.Source,
			singleLine(alert.Message),
		)
	}
}

func printAlertResolveResult(options maintenanceOptions, resolved bool) {
	fmt.Printf("action\tresolve-alert\n")
	fmt.Printf("tenant_id\t%d\n", options.TenantID)
	fmt.Printf("metric\t%s\n", strings.TrimSpace(options.AlertMetric))
	fmt.Printf("alert_type\t%s\n", strings.TrimSpace(options.AlertType))
	fmt.Printf("resolved\t%t\n", resolved)
}

func printAlertNotificationDispatchResult(result dashboard.SaaSAlertNotificationDispatchResult) {
	fmt.Printf("action\tdispatch-alert-notifications\n")
	fmt.Printf("scanned\t%d\n", result.Scanned)
	fmt.Printf("delivered\t%d\n", result.Delivered)
	fmt.Printf("deferred\t%d\n", result.Deferred)
	fmt.Printf("suppressed\t%d\n", result.Suppressed)
	fmt.Printf("failed\t%d\n", result.Failed)
	fmt.Printf("dead\t%d\n", result.Dead)
}

func printAlertCredentialRotation(result dashboard.SaaSAlertCredentialRotationResult) {
	fmt.Printf("action\trotate-alert-credentials\n")
	fmt.Printf("tenant_id\t%d\n", result.TenantID)
	fmt.Printf("limit\t%d\n", result.Limit)
	fmt.Printf("scanned\t%d\n", result.ScannedCount)
	fmt.Printf("rotated\t%d\n", result.RotatedCount)
	fmt.Printf("legacy_plaintext\t%d\n", result.LegacyCount)
	fmt.Printf("reencrypted\t%d\n", result.ReencryptedCount)
	fmt.Printf("active_key_id\t%s\n", result.ActiveKeyID)
}

func printWeComCredentialRotation(result dashboard.SaaSWeComCredentialRotationResult) {
	fmt.Printf("action\trotate-wecom-credentials\n")
	fmt.Printf("tenant_id\t%d\n", result.TenantID)
	fmt.Printf("limit\t%d\n", result.Limit)
	fmt.Printf("scanned\t%d\n", result.ScannedCount)
	fmt.Printf("rotated\t%d\n", result.RotatedCount)
	fmt.Printf("corp_rotated\t%d\n", result.CorpRotatedCount)
	fmt.Printf("agent_rotated\t%d\n", result.AgentRotatedCount)
	fmt.Printf("legacy_plaintext\t%d\n", result.LegacyCount)
	fmt.Printf("reencrypted\t%d\n", result.ReencryptedCount)
	fmt.Printf("active_key_id\t%s\n", result.ActiveKeyID)
}

func printWeChatOpenCredentialRotation(result dashboard.SaaSWeChatOpenCredentialRotationResult) {
	fmt.Printf("action\trotate-wechat-open-credentials\n")
	fmt.Printf("tenant_id\t%d\n", result.TenantID)
	fmt.Printf("limit\t%d\n", result.Limit)
	fmt.Printf("scanned\t%d\n", result.ScannedCount)
	fmt.Printf("rotated\t%d\n", result.RotatedCount)
	fmt.Printf("component_ticket_rotated\t%d\n", result.ComponentTicketRotatedCount)
	fmt.Printf("official_account_rotated\t%d\n", result.OfficialAccountRotatedCount)
	fmt.Printf("legacy_plaintext\t%d\n", result.LegacyCount)
	fmt.Printf("reencrypted\t%d\n", result.ReencryptedCount)
	fmt.Printf("active_key_id\t%s\n", result.ActiveKeyID)
}

func printBackupOverview(overview saasbackup.Overview) {
	fmt.Printf("action\tbackup-status\n")
	fmt.Printf("policy_status\t%s\n", overview.Policy.Status)
	fmt.Printf("policy_version\t%d\n", overview.Policy.Version)
	fmt.Printf("encryption_configured\t%t\n", overview.Config.EncryptionConfigured)
	fmt.Printf("encryption_key_count\t%d\n", overview.Config.EncryptionKeyCount)
	fmt.Printf("restore_configured\t%t\n", overview.Config.RestoreConfigured)
	fmt.Printf("restore_auto_provision\t%t\n", overview.Config.RestoreAutoProvision)
	fmt.Printf("replica_configured\t%t\n", overview.Config.ReplicaConfigured)
	fmt.Printf("replica_provider\t%s\n", overview.Config.ReplicaProvider)
	fmt.Printf("replica_bucket\t%s\n", overview.Config.ReplicaBucket)
	fmt.Printf("backup_runs\t%d\n", len(overview.Runs))
	fmt.Printf("restore_drills\t%d\n", len(overview.Drills))
}

func printBackupRun(action string, run saasbackup.BackupRun) {
	fmt.Printf("action\t%s\n", action)
	fmt.Printf("backup_run_id\t%d\n", run.ID)
	fmt.Printf("backup_no\t%s\n", run.BackupNo)
	fmt.Printf("status\t%s\n", run.Status)
	fmt.Printf("encrypted\t%t\n", run.Encrypted)
	fmt.Printf("size_bytes\t%d\n", run.SizeBytes)
	fmt.Printf("sha256\t%s\n", run.SHA256)
	fmt.Printf("migration_version\t%s\n", run.MigrationVersion)
	fmt.Printf("migration_count\t%d\n", run.MigrationCount)
	fmt.Printf("table_count\t%d\n", run.TableCount)
	fmt.Printf("verification_status\t%s\n", run.VerificationStatus)
	fmt.Printf("replica_status\t%s\n", run.ReplicaStatus)
	fmt.Printf("replica_provider\t%s\n", run.ReplicaProvider)
	fmt.Printf("replica_bucket\t%s\n", run.ReplicaBucket)
	fmt.Printf("replica_object_key\t%s\n", run.ReplicaObjectKey)
}

func printRestoreDrill(drill saasbackup.RestoreDrill) {
	fmt.Printf("action\tbackup-restore-drill\n")
	fmt.Printf("restore_drill_id\t%d\n", drill.ID)
	fmt.Printf("drill_no\t%s\n", drill.DrillNo)
	fmt.Printf("backup_run_id\t%d\n", drill.BackupRunID)
	fmt.Printf("status\t%s\n", drill.Status)
	fmt.Printf("target_database\t%s\n", drill.TargetDatabase)
	fmt.Printf("target_lifecycle\t%s\n", drill.TargetLifecycle)
	fmt.Printf("target_cleanup_status\t%s\n", drill.TargetCleanupStatus)
	fmt.Printf("actual_migration_version\t%s\n", drill.ActualMigrationVersion)
	fmt.Printf("actual_migration_count\t%d\n", drill.ActualMigrationCount)
	fmt.Printf("actual_table_count\t%d\n", drill.ActualTableCount)
	fmt.Printf("duration_ms\t%d\n", drill.DurationMS)
}

func printBackupCleanup(result saasbackup.CleanupRun, resumed bool) {
	fmt.Printf("action\tbackup-cleanup\n")
	fmt.Printf("resumed\t%t\n", resumed)
	fmt.Printf("cleanup_run_id\t%d\n", result.ID)
	fmt.Printf("cleanup_no\t%s\n", result.CleanupNo)
	fmt.Printf("status\t%s\n", result.Status)
	fmt.Printf("candidates\t%d\n", result.CandidateCount)
	fmt.Printf("deleted\t%d\n", result.DeletedCount)
	fmt.Printf("failed\t%d\n", result.FailedCount)
	fmt.Printf("replicas_deleted\t%d\n", result.ReplicasDeletedCount)
	fmt.Printf("missing_files\t%d\n", result.MissingFilesCount)
}

func printComplianceOverview(overview saascompliance.Overview) {
	fmt.Printf("action\tcompliance-status\n")
	fmt.Printf("policy_status\t%s\n", overview.Policy.Status)
	fmt.Printf("policy_version\t%d\n", overview.Policy.Version)
	fmt.Printf("inventory_version\t%s\n", overview.Config.InventoryVersion)
	fmt.Printf("inventory_tables\t%d\n", overview.Config.InventoryTableCount)
	fmt.Printf("inventory_unknown_tables\t%d\n", len(overview.Config.InventoryUnknownTables))
	fmt.Printf("encryption_configured\t%t\n", overview.Config.EncryptionConfigured)
	fmt.Printf("legal_holds\t%d\n", len(overview.Holds))
	fmt.Printf("exports\t%d\n", len(overview.Exports))
	fmt.Printf("erasures\t%d\n", len(overview.Erasures))
}

func printComplianceProcess(result saascompliance.ProcessResult) {
	fmt.Printf("action\tcompliance-process\n")
	fmt.Printf("exports_processed\t%d\n", result.ExportsProcessed)
	fmt.Printf("export_deletions_processed\t%d\n", result.ExportDeletionsProcessed)
	fmt.Printf("erasures_processed\t%d\n", result.ErasuresProcessed)
	fmt.Printf("artifacts_deleted\t%d\n", result.ArtifactsDeleted)
}

func firstEnv(names ...string) string {
	for _, name := range names {
		if value := os.Getenv(name); value != "" {
			return value
		}
	}
	return ""
}

func alertCredentialEncryptionDefaultsFromEnv() (saasalertcredentials.EncryptionSource, bool) {
	return saasalertcredentials.SelectEncryptionSource(
		saasalertcredentials.EncryptionSource{
			Key:   os.Getenv("MOCHAT_GO_SAAS_ALERT_CREDENTIAL_ENCRYPTION_KEY"),
			Keys:  os.Getenv("MOCHAT_GO_SAAS_ALERT_CREDENTIAL_ENCRYPTION_KEYS"),
			KeyID: os.Getenv("MOCHAT_GO_SAAS_ALERT_CREDENTIAL_ENCRYPTION_KEY_ID"),
		},
		saasalertcredentials.EncryptionSource{
			Key:   os.Getenv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY"),
			Keys:  os.Getenv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEYS"),
			KeyID: os.Getenv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY_ID"),
		},
		saasalertcredentials.EncryptionSource{
			Key:   os.Getenv("MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY"),
			Keys:  os.Getenv("MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEYS"),
			KeyID: os.Getenv("MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY_ID"),
		},
		saasalertcredentials.EncryptionSource{
			Key:   os.Getenv("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY"),
			Keys:  os.Getenv("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEYS"),
			KeyID: os.Getenv("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY_ID"),
		},
	)
}

func applyAlertCredentialFlagSelection(options *maintenanceOptions, defaultsDedicated bool, visited map[string]bool) {
	keySet := visited["alert-credential-encryption-key"]
	keysSet := visited["alert-credential-encryption-keys"]
	if !keySet && !keysSet {
		return
	}
	options.AlertCredentialDedicatedConfigured = true
	if defaultsDedicated {
		return
	}
	if !keySet {
		options.AlertCredentialEncryptionKey = ""
	}
	if !keysSet {
		options.AlertCredentialEncryptionKeys = ""
	}
	if !visited["alert-credential-encryption-key-id"] {
		options.AlertCredentialEncryptionKeyID = "primary"
	}
}

func weComCredentialEncryptionDefaultsFromEnv() (wecomcredentials.EncryptionSource, bool) {
	return wecomcredentials.SelectEncryptionSource(
		wecomcredentials.EncryptionSource{
			Key:   os.Getenv("MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY"),
			Keys:  os.Getenv("MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEYS"),
			KeyID: os.Getenv("MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY_ID"),
		},
		wecomcredentials.EncryptionSource{
			Key:   os.Getenv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY"),
			Keys:  os.Getenv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEYS"),
			KeyID: os.Getenv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY_ID"),
		},
		wecomcredentials.EncryptionSource{
			Key:   os.Getenv("MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY"),
			Keys:  os.Getenv("MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEYS"),
			KeyID: os.Getenv("MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY_ID"),
		},
		wecomcredentials.EncryptionSource{
			Key:   os.Getenv("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY"),
			Keys:  os.Getenv("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEYS"),
			KeyID: os.Getenv("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY_ID"),
		},
	)
}

func applyWeComCredentialFlagSelection(options *maintenanceOptions, defaultsDedicated bool, visited map[string]bool) {
	keySet := visited["wecom-credential-encryption-key"]
	keysSet := visited["wecom-credential-encryption-keys"]
	if !keySet && !keysSet {
		return
	}
	options.WeComCredentialDedicatedConfigured = true
	if defaultsDedicated {
		return
	}
	if !keySet {
		options.WeComCredentialEncryptionKey = ""
	}
	if !keysSet {
		options.WeComCredentialEncryptionKeys = ""
	}
	if !visited["wecom-credential-encryption-key-id"] {
		options.WeComCredentialEncryptionKeyID = "primary"
	}
}

func weChatOpenCredentialEncryptionDefaultsFromEnv() (wechatopencredentials.EncryptionSource, bool) {
	return wechatopencredentials.SelectEncryptionSource(
		wechatopencredentials.EncryptionSource{
			Key: os.Getenv("MOCHAT_GO_WECHAT_OPEN_CREDENTIAL_ENCRYPTION_KEY"), Keys: os.Getenv("MOCHAT_GO_WECHAT_OPEN_CREDENTIAL_ENCRYPTION_KEYS"), KeyID: os.Getenv("MOCHAT_GO_WECHAT_OPEN_CREDENTIAL_ENCRYPTION_KEY_ID"),
		},
		wechatopencredentials.EncryptionSource{
			Key: os.Getenv("MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY"), Keys: os.Getenv("MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEYS"), KeyID: os.Getenv("MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY_ID"),
		},
		wechatopencredentials.EncryptionSource{
			Key: os.Getenv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY"), Keys: os.Getenv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEYS"), KeyID: os.Getenv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY_ID"),
		},
		wechatopencredentials.EncryptionSource{
			Key: os.Getenv("MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY"), Keys: os.Getenv("MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEYS"), KeyID: os.Getenv("MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY_ID"),
		},
		wechatopencredentials.EncryptionSource{
			Key: os.Getenv("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY"), Keys: os.Getenv("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEYS"), KeyID: os.Getenv("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY_ID"),
		},
	)
}

func applyWeChatOpenCredentialFlagSelection(options *maintenanceOptions, defaultsDedicated bool, visited map[string]bool) {
	keySet := visited["wechat-open-credential-encryption-key"]
	keysSet := visited["wechat-open-credential-encryption-keys"]
	if !keySet && !keysSet {
		return
	}
	options.WeChatOpenCredentialDedicatedConfigured = true
	if defaultsDedicated {
		return
	}
	if !keySet {
		options.WeChatOpenCredentialEncryptionKey = ""
	}
	if !keysSet {
		options.WeChatOpenCredentialEncryptionKeys = ""
	}
	if !visited["wechat-open-credential-encryption-key-id"] {
		options.WeChatOpenCredentialEncryptionKeyID = "primary"
	}
}

func envDefault(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func intEnv(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func boolEnv(name string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	switch strings.ToLower(value) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func int64Env(name string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func joinInts(values []int) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.Itoa(value))
	}
	return strings.Join(parts, ",")
}

func singleLine(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\t", " ")
	return strings.TrimSpace(value)
}
