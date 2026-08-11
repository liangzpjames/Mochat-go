package config

import (
	"strings"
	"testing"
	"time"

	appruntime "jiyi/mochat-go/internal/app/runtime"
)

func TestFromEnvDefaults(t *testing.T) {
	clearEnv(t)

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddr != ":8080" {
		t.Fatalf("ListenAddr = %q", cfg.ListenAddr)
	}
	if cfg.Timezone != "Asia/Shanghai" {
		t.Fatalf("Timezone = %q", cfg.Timezone)
	}
	if cfg.RuntimeRole != appruntime.RoleAll {
		t.Fatalf("RuntimeRole = %q, want %q", cfg.RuntimeRole, appruntime.RoleAll)
	}
	if cfg.Standalone {
		t.Fatalf("Standalone = true")
	}
	if cfg.EnableAllMigratedRoutes {
		t.Fatalf("EnableAllMigratedRoutes = true")
	}
	if cfg.RedisAddr != "localhost:6379" {
		t.Fatalf("RedisAddr = %q", cfg.RedisAddr)
	}
	if cfg.SimpleJWTPrefix != "default" {
		t.Fatalf("SimpleJWTPrefix = %q", cfg.SimpleJWTPrefix)
	}
	if cfg.SimpleJWTTTL != 7*24*time.Hour {
		t.Fatalf("SimpleJWTTTL = %s", cfg.SimpleJWTTTL)
	}
	if cfg.SimpleJWTRefreshTTL != 7*24*time.Hour {
		t.Fatalf("SimpleJWTRefreshTTL = %s", cfg.SimpleJWTRefreshTTL)
	}
	if cfg.EnablePullAgentCron || cfg.PullAgentCronInterval != time.Hour || cfg.PullAgentCronRunOnStart {
		t.Fatalf("pullAgent cron config = enabled %v interval %s runOnStart %v", cfg.EnablePullAgentCron, cfg.PullAgentCronInterval, cfg.PullAgentCronRunOnStart)
	}
	if cfg.EnableEmployeeStatisticCron || cfg.EmployeeStatisticCronInterval != 24*time.Hour || cfg.EmployeeStatisticCronRunOnStart {
		t.Fatalf("employeeStatistic cron config = enabled %v interval %s runOnStart %v", cfg.EnableEmployeeStatisticCron, cfg.EmployeeStatisticCronInterval, cfg.EmployeeStatisticCronRunOnStart)
	}
	if cfg.EnableChannelCodeCron || cfg.ChannelCodeCronInterval != 30*time.Second || cfg.ChannelCodeCronRunOnStart {
		t.Fatalf("channelCode cron config = enabled %v interval %s runOnStart %v", cfg.EnableChannelCodeCron, cfg.ChannelCodeCronInterval, cfg.ChannelCodeCronRunOnStart)
	}
	if cfg.EnableContactBatchSendCron || cfg.ContactBatchSendCronInterval != time.Minute || cfg.ContactBatchSendCronRunOnStart {
		t.Fatalf("ContactMessageBatchSend cron config = enabled %v interval %s runOnStart %v", cfg.EnableContactBatchSendCron, cfg.ContactBatchSendCronInterval, cfg.ContactBatchSendCronRunOnStart)
	}
	if cfg.EnableRoomBatchSendCron || cfg.RoomBatchSendCronInterval != time.Minute || cfg.RoomBatchSendCronRunOnStart {
		t.Fatalf("RoomMessageBatchSend cron config = enabled %v interval %s runOnStart %v", cfg.EnableRoomBatchSendCron, cfg.RoomBatchSendCronInterval, cfg.RoomBatchSendCronRunOnStart)
	}
	if cfg.EnableContactSyncSendResultCron || cfg.ContactSyncSendResultCronInterval != time.Hour || cfg.ContactSyncSendResultCronRunOnStart {
		t.Fatalf("ContactSyncSendResultTask cron config = enabled %v interval %s runOnStart %v", cfg.EnableContactSyncSendResultCron, cfg.ContactSyncSendResultCronInterval, cfg.ContactSyncSendResultCronRunOnStart)
	}
	if cfg.EnableRoomSyncSendResultCron || cfg.RoomSyncSendResultCronInterval != time.Hour || cfg.RoomSyncSendResultCronRunOnStart {
		t.Fatalf("RoomSyncSendResultTask cron config = enabled %v interval %s runOnStart %v", cfg.EnableRoomSyncSendResultCron, cfg.RoomSyncSendResultCronInterval, cfg.RoomSyncSendResultCronRunOnStart)
	}
	if cfg.EnableRoomTagPullCron || cfg.RoomTagPullCronInterval != 5*time.Minute || cfg.RoomTagPullCronRunOnStart {
		t.Fatalf("RoomTagPull cron config = enabled %v interval %s runOnStart %v", cfg.EnableRoomTagPullCron, cfg.RoomTagPullCronInterval, cfg.RoomTagPullCronRunOnStart)
	}
	if cfg.EnableCorpDataCron || cfg.CorpDataCronInterval != 10*time.Minute || cfg.CorpDataCronRunOnStart {
		t.Fatalf("corpData cron config = enabled %v interval %s runOnStart %v", cfg.EnableCorpDataCron, cfg.CorpDataCronInterval, cfg.CorpDataCronRunOnStart)
	}
	if cfg.EnableMediaIDUpdateCron || cfg.MediaIDUpdateCronInterval != 5*time.Minute || cfg.MediaIDUpdateCronRunOnStart {
		t.Fatalf("mediaIdUpdate cron config = enabled %v interval %s runOnStart %v", cfg.EnableMediaIDUpdateCron, cfg.MediaIDUpdateCronInterval, cfg.MediaIDUpdateCronRunOnStart)
	}
	if cfg.EnableTransferStateRefreshCron || cfg.TransferStateRefreshCronInterval != 5*time.Minute || cfg.TransferStateRefreshCronRunOnStart {
		t.Fatalf("TransferStateRefresh cron config = enabled %v interval %s runOnStart %v", cfg.EnableTransferStateRefreshCron, cfg.TransferStateRefreshCronInterval, cfg.TransferStateRefreshCronRunOnStart)
	}
	if cfg.EnableSOPLogCron || cfg.SOPLogCronInterval != 5*time.Minute || cfg.SOPLogCronRunOnStart {
		t.Fatalf("SOP log cron config = enabled %v interval %s runOnStart %v", cfg.EnableSOPLogCron, cfg.SOPLogCronInterval, cfg.SOPLogCronRunOnStart)
	}
	if cfg.EnableSensitiveWordMonitorCron || cfg.SensitiveWordMonitorCronInterval != time.Minute || cfg.SensitiveWordMonitorCronRunOnStart {
		t.Fatalf("sensitiveWordsMonitor cron config = enabled %v interval %s runOnStart %v", cfg.EnableSensitiveWordMonitorCron, cfg.SensitiveWordMonitorCronInterval, cfg.SensitiveWordMonitorCronRunOnStart)
	}
	if cfg.EnableWorkMessageArchiveSyncCron || cfg.WorkMessageArchiveSyncCronInterval != time.Minute || cfg.WorkMessageArchiveSyncCronRunOnStart || cfg.WorkMessageArchiveSyncLimit != 100 || cfg.WorkMessageArchiveBridgeBaseURL != "" || cfg.WorkMessageArchiveBridgeToken != "" {
		t.Fatalf("workMessageArchive sync cron config = enabled %v interval %s runOnStart %v limit %d bridge %q token %q", cfg.EnableWorkMessageArchiveSyncCron, cfg.WorkMessageArchiveSyncCronInterval, cfg.WorkMessageArchiveSyncCronRunOnStart, cfg.WorkMessageArchiveSyncLimit, cfg.WorkMessageArchiveBridgeBaseURL, cfg.WorkMessageArchiveBridgeToken)
	}
	if cfg.EnableSaaSStorageReconcileCron || cfg.SaaSStorageReconcileCronInterval != 24*time.Hour || cfg.SaaSStorageReconcileCronRunOnStart {
		t.Fatalf("SaaS storage reconcile cron config = enabled %v interval %s runOnStart %v", cfg.EnableSaaSStorageReconcileCron, cfg.SaaSStorageReconcileCronInterval, cfg.SaaSStorageReconcileCronRunOnStart)
	}
	if cfg.EnableSaaSAlertNotificationDispatchCron || cfg.SaaSAlertNotificationDispatchCronInterval != 5*time.Minute || cfg.SaaSAlertNotificationDispatchCronRunOnStart || cfg.SaaSAlertNotificationDispatchLimit != 100 {
		t.Fatalf("SaaS alert notification dispatch cron config = enabled %v interval %s runOnStart %v limit %d", cfg.EnableSaaSAlertNotificationDispatchCron, cfg.SaaSAlertNotificationDispatchCronInterval, cfg.SaaSAlertNotificationDispatchCronRunOnStart, cfg.SaaSAlertNotificationDispatchLimit)
	}
	if cfg.EnableSaaSPaymentSettlementSyncCron || cfg.SaaSPaymentSettlementSyncCronInterval != 15*time.Minute || cfg.SaaSPaymentSettlementSyncCronRunOnStart ||
		cfg.SaaSPaymentSettlementBridgeBaseURL != "" || cfg.SaaSPaymentSettlementBridgeToken != "" || len(cfg.SaaSPaymentSettlementProviders) != 0 ||
		cfg.SaaSPaymentSettlementSyncLimit != 100 || cfg.SaaSPaymentSettlementBridgeTimeout != 10*time.Second {
		t.Fatalf("payment settlement sync config = %+v", cfg)
	}
	if cfg.EnableSaaSTenantDomainDeliveryCron || cfg.SaaSTenantDomainDeliveryCronInterval != time.Minute || cfg.SaaSTenantDomainDeliveryCronRunOnStart ||
		cfg.SaaSTenantDomainDeliveryBridgeBaseURL != "" || cfg.SaaSTenantDomainDeliveryBridgeToken != "" || cfg.SaaSTenantDomainDeliveryBridgeTimeout != 10*time.Second ||
		cfg.SaaSTenantDomainDeliveryCallbackURL != "" || cfg.SaaSTenantDomainDeliveryCallbackSecret != "" || cfg.SaaSTenantDomainDeliveryCallbackTolerance != 5*time.Minute ||
		cfg.SaaSTenantDomainDeliveryLimit != 20 || cfg.SaaSTenantDomainDeliveryLease != 2*time.Minute || cfg.SaaSTenantDomainDeliveryRetryDelay != time.Minute || cfg.SaaSTenantDomainDeliveryCallbackWait != 10*time.Minute {
		t.Fatalf("tenant domain delivery config = %+v", cfg)
	}
	if cfg.EnableSaaSAlertDashboard {
		t.Fatalf("EnableSaaSAlertDashboard = true")
	}
	if cfg.EnableSaaSAdminDashboard || cfg.EnableSaaSBillingPortal || cfg.SaaSPlatformAdminTenantID != 1 || cfg.SaaSReleaseSourceFingerprint != "" || cfg.SaaSReleaseSourceFingerprintSource != "" {
		t.Fatalf("SaaS admin dashboard config = enabled %v platformTenantID %d releaseFingerprint %q source %q", cfg.EnableSaaSAdminDashboard, cfg.SaaSPlatformAdminTenantID, cfg.SaaSReleaseSourceFingerprint, cfg.SaaSReleaseSourceFingerprintSource)
	}
	if cfg.EnableAsyncFileUploadWorker {
		t.Fatalf("EnableAsyncFileUploadWorker = true")
	}
	if cfg.EnableMarkTagsWorker {
		t.Fatalf("EnableMarkTagsWorker = true")
	}
	if cfg.EnableMessageRemindWorker {
		t.Fatalf("EnableMessageRemindWorker = true")
	}
	if cfg.EnableWorkRoomSyncWorker {
		t.Fatalf("EnableWorkRoomSyncWorker = true")
	}
	if cfg.EnableWorkContactSyncWorker {
		t.Fatalf("EnableWorkContactSyncWorker = true")
	}
	if cfg.EnableWorkDepartmentListWorker {
		t.Fatalf("EnableWorkDepartmentListWorker = true")
	}
	if cfg.EnableMediaIDUpdateWorker {
		t.Fatalf("EnableMediaIDUpdateWorker = true")
	}
	if cfg.EnableEmployeeStatisticWorker {
		t.Fatalf("EnableEmployeeStatisticWorker = true")
	}
	if cfg.SaaSAlertWebhookURL != "" || cfg.SaaSAlertWebhookSecret != "" || cfg.SaaSAlertWebhookTimeout != 5*time.Second || cfg.SaaSAlertWebhookRetryAttempts != 1 || cfg.SaaSAlertWebhookRetryDelay != 250*time.Millisecond || cfg.SaaSAlertWebhookTitleTemplate != DefaultSaaSAlertWebhookTitleTemplate || cfg.SaaSAlertWebhookBodyTemplate != DefaultSaaSAlertWebhookBodyTemplate || !cfg.SaaSAlertWebhookRequireHTTPS || len(cfg.SaaSAlertWebhookAllowedCIDRs) != 0 || cfg.SaaSAlertNotificationMaxAttempts != 3 || cfg.SaaSAlertNotificationRetryDelay != 5*time.Minute {
		t.Fatalf("SaaS alert webhook config = url %q secret %q timeout %s attempts %d delay %s title %q body %q outbox_attempts %d outbox_delay %s", cfg.SaaSAlertWebhookURL, cfg.SaaSAlertWebhookSecret, cfg.SaaSAlertWebhookTimeout, cfg.SaaSAlertWebhookRetryAttempts, cfg.SaaSAlertWebhookRetryDelay, cfg.SaaSAlertWebhookTitleTemplate, cfg.SaaSAlertWebhookBodyTemplate, cfg.SaaSAlertNotificationMaxAttempts, cfg.SaaSAlertNotificationRetryDelay)
	}
	if cfg.SidebarJWTSecret != "Br3LXhp&Ysha1zRDh" || cfg.SidebarJWTPrefix != "default" {
		t.Fatalf("sidebar jwt config = secret %q prefix %q", cfg.SidebarJWTSecret, cfg.SidebarJWTPrefix)
	}
	if cfg.PHPUpstream != "" {
		t.Fatalf("PHPUpstream = %q", cfg.PHPUpstream)
	}
	if cfg.APIBaseURL != "http://127.0.0.1:8080" || cfg.SidebarBaseURL != "http://127.0.0.1:8080" || cfg.OperationBaseURL != "http://127.0.0.1:8080" {
		t.Fatalf("base urls = api %q sidebar %q operation %q", cfg.APIBaseURL, cfg.SidebarBaseURL, cfg.OperationBaseURL)
	}
	if cfg.FileStorageRoot != "../mochat/api-server/storage/upload/static" {
		t.Fatalf("FileStorageRoot = %q", cfg.FileStorageRoot)
	}
	if cfg.DashboardDist != "./web/apps/dashboard/dist" {
		t.Fatalf("DashboardDist = %q", cfg.DashboardDist)
	}
	if cfg.SaaSAdminDist != "./web/apps/saas-admin/dist" {
		t.Fatalf("SaaSAdminDist = %q", cfg.SaaSAdminDist)
	}
	if cfg.SidebarDist != "./web/apps/sidebar/dist" || cfg.OperationDist != "./web/apps/operation/dist" {
		t.Fatalf("frontend dists = sidebar %q operation %q", cfg.SidebarDist, cfg.OperationDist)
	}
	if cfg.SidebarFrontendAddr != "" || cfg.OperationFrontendAddr != "" {
		t.Fatalf("frontend addrs = sidebar %q operation %q", cfg.SidebarFrontendAddr, cfg.OperationFrontendAddr)
	}
}

func TestFromEnvRejectsInvalidTimezone(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_TIMEZONE", "Mars/Olympus")

	_, err := FromEnv()

	if err == nil || !strings.Contains(err.Error(), "MOCHAT_TIMEZONE") {
		t.Fatalf("err = %v", err)
	}
}

func TestFromEnvRejectsValidTimezoneOutsideSupportedDashboardContract(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_TIMEZONE", "America/New_York")

	_, err := FromEnv()

	if err == nil || !strings.Contains(err.Error(), "Asia/Shanghai") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadRequiresDistinctSaaSAndDashboardRealmConfiguration(t *testing.T) {
	valid := func(t *testing.T) {
		t.Helper()
		clearEnv(t)
		t.Setenv("MOCHAT_SAAS_ADMIN_JWT_SECRET", "saas-admin-secret-012345678901234567890")
		t.Setenv("MOCHAT_SAAS_ADMIN_JWT_ISSUER", "mochat-go/saas-auth")
		t.Setenv("MOCHAT_SAAS_ADMIN_JWT_AUDIENCE", "mochat-saas-admin")
		t.Setenv("MOCHAT_DASHBOARD_JWT_SECRET", "dashboard-secret-012345678901234567890")
		t.Setenv("MOCHAT_DASHBOARD_JWT_ISSUER", "mochat-go/dashboard-auth")
		t.Setenv("MOCHAT_DASHBOARD_JWT_AUDIENCE", "mochat-dashboard")
	}

	cases := []struct {
		name   string
		mutate func(*testing.T)
	}{
		{name: "missing SaaS secret", mutate: func(t *testing.T) { t.Setenv("MOCHAT_SAAS_ADMIN_JWT_SECRET", "") }},
		{name: "missing Dashboard secret", mutate: func(t *testing.T) { t.Setenv("MOCHAT_DASHBOARD_JWT_SECRET", "") }},
		{name: "same secrets", mutate: func(t *testing.T) {
			t.Setenv("MOCHAT_DASHBOARD_JWT_SECRET", "saas-admin-secret-012345678901234567890")
		}},
		{name: "missing SaaS issuer", mutate: func(t *testing.T) { t.Setenv("MOCHAT_SAAS_ADMIN_JWT_ISSUER", "") }},
		{name: "missing Dashboard audience", mutate: func(t *testing.T) { t.Setenv("MOCHAT_DASHBOARD_JWT_AUDIENCE", "") }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			valid(t)
			tc.mutate(t)
			if _, err := Load(); err == nil {
				t.Fatal("Load unexpectedly accepted incomplete realm configuration")
			}
		})
	}
}

func TestLoadProvidesIndependentRealmTokenConfiguration(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_SAAS_ADMIN_JWT_SECRET", "saas-admin-secret-012345678901234567890")
	t.Setenv("MOCHAT_SAAS_ADMIN_JWT_ISSUER", "mochat-go/saas-auth")
	t.Setenv("MOCHAT_SAAS_ADMIN_JWT_AUDIENCE", "mochat-saas-admin")
	t.Setenv("MOCHAT_SAAS_ADMIN_JWT_PREFIX", "mochat_saas_admin_")
	t.Setenv("MOCHAT_SAAS_ADMIN_JWT_TTL", "3600")
	t.Setenv("MOCHAT_DASHBOARD_JWT_SECRET", "dashboard-secret-012345678901234567890")
	t.Setenv("MOCHAT_DASHBOARD_JWT_ISSUER", "mochat-go/dashboard-auth")
	t.Setenv("MOCHAT_DASHBOARD_JWT_AUDIENCE", "mochat-dashboard")
	t.Setenv("MOCHAT_DASHBOARD_JWT_PREFIX", "mochat_dashboard_")
	t.Setenv("MOCHAT_DASHBOARD_JWT_TTL", "7200")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SaaSAdminJWTSecret == cfg.DashboardJWTSecret || cfg.SaaSAdminJWTPrefix == cfg.DashboardJWTPrefix {
		t.Fatal("SaaS and Dashboard realm configuration is not independent")
	}
	if cfg.SaaSAdminJWTTTL != time.Hour || cfg.DashboardJWTTTL != 2*time.Hour {
		t.Fatalf("realm TTLs = %s/%s", cfg.SaaSAdminJWTTTL, cfg.DashboardJWTTTL)
	}
}

func TestPhase22SCRMPilotDisabledByDefault(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_PHASE2_2_SCRM_PILOT", "")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.EnablePhase22SCRMPilot {
		t.Fatal("EnablePhase22SCRMPilot = true")
	}
}

func TestPhase22SCRMPilotCanBeEnabled(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_PHASE2_2_SCRM_PILOT", "true")
	setPhase22SCRMProductionDependencies(t)

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnablePhase22SCRMPilot {
		t.Fatal("EnablePhase22SCRMPilot = false")
	}
}

func TestPhase22SCRMPilotRequiresProductionStateChangingDependencies(t *testing.T) {
	t.Run("MySQL", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("MOCHAT_GO_ENABLE_PHASE2_2_SCRM_PILOT", "true")

		_, err := FromEnv()
		requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")
	})

	t.Run("JWT", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("MOCHAT_GO_ENABLE_PHASE2_2_SCRM_PILOT", "true")
		t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

		_, err := FromEnv()
		requireErrorContains(t, err, "SIMPLE_JWT_SECRET")
	})

	t.Run("production dependencies", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("MOCHAT_GO_ENABLE_PHASE2_2_SCRM_PILOT", "true")
		setPhase22SCRMProductionDependencies(t)

		cfg, err := FromEnv()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.RedisAddr != "127.0.0.1:6379" {
			t.Fatalf("RedisAddr = %q", cfg.RedisAddr)
		}
	})
}

func TestPhase22SCRMPilotRejectsDevAuthHeader(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_PHASE2_2_SCRM_PILOT", "true")
	setPhase22SCRMProductionDependencies(t)
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "true")

	_, err := FromEnv()
	requireErrorContains(t, err, "state-changing migrated routes require PHP JWT auth")
}

func TestPhase22SCRMPilotRejectsSkippedJWTBlacklist(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_PHASE2_2_SCRM_PILOT", "true")
	setPhase22SCRMProductionDependencies(t)
	t.Setenv("MOCHAT_GO_SKIP_JWT_BLACKLIST", "true")

	_, err := FromEnv()
	requireErrorContains(t, err, "state-changing migrated routes cannot skip JWT blacklist checks")
}

func setPhase22SCRMProductionDependencies(t *testing.T) {
	t.Helper()
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_REDIS_ADDR", "127.0.0.1:6379")
}

func TestFromEnvRuntimeRole(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_RUNTIME_ROLE", "api")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RuntimeRole != appruntime.RoleAPI {
		t.Fatalf("RuntimeRole = %q, want %q", cfg.RuntimeRole, appruntime.RoleAPI)
	}
}

func TestFromEnvDefaultSaaSAdminDist(t *testing.T) {
	clearEnv(t)

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SaaSAdminDist != "./web/apps/saas-admin/dist" {
		t.Fatalf("SaaSAdminDist = %q", cfg.SaaSAdminDist)
	}
}

func TestFromEnvRejectsInvalidRuntimeRole(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_RUNTIME_ROLE", "background")

	if _, err := FromEnv(); err == nil || !strings.Contains(err.Error(), "invalid runtime role") {
		t.Fatalf("FromEnv() error = %v, want invalid runtime role", err)
	}
}

func TestFromEnvRuntimeRoleFiltersBackgroundResponsibilities(t *testing.T) {
	tests := []struct {
		role                      string
		wantWorker, wantScheduler bool
	}{
		{role: "all", wantWorker: true, wantScheduler: true},
		{role: "api"},
		{role: "worker", wantWorker: true},
		{role: "scheduler", wantScheduler: true},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			clearEnv(t)
			t.Setenv("MOCHAT_GO_RUNTIME_ROLE", tt.role)
			t.Setenv("MOCHAT_GO_ENABLE_MARK_TAGS_WORKER", "1")
			t.Setenv("MOCHAT_GO_ENABLE_PULL_AGENT_CRON", "1")
			t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

			cfg, err := FromEnv()
			if err != nil {
				t.Fatal(err)
			}
			if cfg.EnableMarkTagsWorker != tt.wantWorker {
				t.Fatalf("EnableMarkTagsWorker = %v, want %v", cfg.EnableMarkTagsWorker, tt.wantWorker)
			}
			if cfg.EnablePullAgentCron != tt.wantScheduler {
				t.Fatalf("EnablePullAgentCron = %v, want %v", cfg.EnablePullAgentCron, tt.wantScheduler)
			}
		})
	}
}

func TestPHPFallbackIsExplicitAndDrivesDefaultBaseURLs(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_PHP_UPSTREAM", "http://127.0.0.1:9501")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PHPUpstream != "http://127.0.0.1:9501" {
		t.Fatalf("PHPUpstream = %q", cfg.PHPUpstream)
	}
	if cfg.APIBaseURL != "http://127.0.0.1:9501" || cfg.SidebarBaseURL != "http://127.0.0.1:9501" || cfg.OperationBaseURL != "http://127.0.0.1:9501" {
		t.Fatalf("base urls = api %q sidebar %q operation %q", cfg.APIBaseURL, cfg.SidebarBaseURL, cfg.OperationBaseURL)
	}
}

func TestStandaloneDefaultsDoNotDependOnMoChatSourceOrPHP(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_STANDALONE", "1")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Standalone {
		t.Fatalf("Standalone = false")
	}
	if cfg.EnableAllMigratedRoutes {
		t.Fatalf("EnableAllMigratedRoutes = true")
	}
	if cfg.PHPUpstream != "" {
		t.Fatalf("PHPUpstream = %q", cfg.PHPUpstream)
	}
	if cfg.SourceRoot != "" {
		t.Fatalf("SourceRoot = %q", cfg.SourceRoot)
	}
	if cfg.ManifestPath != "" {
		t.Fatalf("ManifestPath = %q", cfg.ManifestPath)
	}
	if cfg.FileStorageRoot != "./storage/upload/static" {
		t.Fatalf("FileStorageRoot = %q", cfg.FileStorageRoot)
	}
	if cfg.DashboardDist != "./web/apps/dashboard/dist" {
		t.Fatalf("DashboardDist = %q", cfg.DashboardDist)
	}
	if cfg.SaaSAdminDist != "./web/apps/saas-admin/dist" {
		t.Fatalf("SaaSAdminDist = %q", cfg.SaaSAdminDist)
	}
	if cfg.SidebarDist != "./web/apps/sidebar/dist" || cfg.OperationDist != "./web/apps/operation/dist" {
		t.Fatalf("frontend dists = sidebar %q operation %q", cfg.SidebarDist, cfg.OperationDist)
	}
	if cfg.SidebarFrontendAddr != "" || cfg.OperationFrontendAddr != "" {
		t.Fatalf("frontend addrs = sidebar %q operation %q", cfg.SidebarFrontendAddr, cfg.OperationFrontendAddr)
	}
	if cfg.APIBaseURL != "http://127.0.0.1:8080" || cfg.SidebarBaseURL != "http://127.0.0.1:8080" || cfg.OperationBaseURL != "http://127.0.0.1:8080" {
		t.Fatalf("base urls = api %q sidebar %q operation %q", cfg.APIBaseURL, cfg.SidebarBaseURL, cfg.OperationBaseURL)
	}
}

func TestStandaloneWithMySQLDefaultsToAllMigratedRoutes(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_STANDALONE", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "standalone-secret")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableAllMigratedRoutes {
		t.Fatalf("EnableAllMigratedRoutes = false")
	}
	if !cfg.MigrateAuth || !cfg.MigrateCorpIndex || !cfg.MigrateChannelCodeStore || !cfg.MigrateOperationWorkFissionAuth || !cfg.MigrateSidebarContactUpdate || !cfg.MigrateSidebarContactSOPTipInfo || !cfg.MigrateSidebarRoomSOPLogState || !cfg.MigrateContactBatchAddDashboard || !cfg.MigrateSensitiveWordsDashboard || !cfg.MigrateRadarDashboard || !cfg.MigrateAutoTagDashboard || !cfg.MigrateLotteryDashboard || !cfg.MigrateRoomFissionDashboard || !cfg.MigrateRoomClockInDashboard || !cfg.MigrateRoomQualityDashboard || !cfg.MigrateRoomCalendarDashboard || !cfg.MigrateRoomRemindDashboard || !cfg.MigrateRoomInfinitePullDashboard || !cfg.MigrateSidebarContactBatchAddDetail {
		t.Fatalf("representative migrated routes not enabled: auth=%v corpIndex=%v channelCodeStore=%v operationAuth=%v sidebarContactUpdate=%v sidebarContactSOPTipInfo=%v sidebarRoomSOPLogState=%v contactBatchAddDashboard=%v sensitiveWordsDashboard=%v radarDashboard=%v autoTagDashboard=%v lotteryDashboard=%v roomFissionDashboard=%v roomClockInDashboard=%v roomQualityDashboard=%v roomCalendarDashboard=%v roomRemindDashboard=%v roomInfinitePullDashboard=%v sidebarContactBatchAddDetail=%v", cfg.MigrateAuth, cfg.MigrateCorpIndex, cfg.MigrateChannelCodeStore, cfg.MigrateOperationWorkFissionAuth, cfg.MigrateSidebarContactUpdate, cfg.MigrateSidebarContactSOPTipInfo, cfg.MigrateSidebarRoomSOPLogState, cfg.MigrateContactBatchAddDashboard, cfg.MigrateSensitiveWordsDashboard, cfg.MigrateRadarDashboard, cfg.MigrateAutoTagDashboard, cfg.MigrateLotteryDashboard, cfg.MigrateRoomFissionDashboard, cfg.MigrateRoomClockInDashboard, cfg.MigrateRoomQualityDashboard, cfg.MigrateRoomCalendarDashboard, cfg.MigrateRoomRemindDashboard, cfg.MigrateRoomInfinitePullDashboard, cfg.MigrateSidebarContactBatchAddDetail)
	}
}

func TestStandaloneAllMigratedRoutesCanBeDisabled(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_STANDALONE", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "standalone-secret")
	t.Setenv("MOCHAT_GO_ENABLE_ALL_MIGRATED_ROUTES", "0")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.EnableAllMigratedRoutes || cfg.MigrateAuth || cfg.MigrateCorpIndex {
		t.Fatalf("routes should be disabled: all=%v auth=%v corpIndex=%v", cfg.EnableAllMigratedRoutes, cfg.MigrateAuth, cfg.MigrateCorpIndex)
	}
}

func TestStandaloneAllMigratedRoutesAllowsPerRouteOverride(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_STANDALONE", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "standalone-secret")
	t.Setenv("MOCHAT_GO_MIGRATE_AUTH", "0")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableAllMigratedRoutes {
		t.Fatalf("EnableAllMigratedRoutes = false")
	}
	if cfg.MigrateAuth || !cfg.MigrateCorpIndex {
		t.Fatalf("per-route override failed: auth=%v corpIndex=%v", cfg.MigrateAuth, cfg.MigrateCorpIndex)
	}
}

func TestStandaloneFrontendPortsFollowListenAddr(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_STANDALONE", "1")
	t.Setenv("MOCHAT_GO_ENABLE_FRONTEND_SERVERS", "1")
	t.Setenv("MOCHAT_GO_ADDR", "127.0.0.1:18082")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SidebarFrontendAddr != "127.0.0.1:18083" || cfg.OperationFrontendAddr != "127.0.0.1:18084" {
		t.Fatalf("frontend addrs = sidebar %q operation %q", cfg.SidebarFrontendAddr, cfg.OperationFrontendAddr)
	}
	if cfg.APIBaseURL != "http://127.0.0.1:18082" || cfg.DashboardBaseURL != "http://127.0.0.1:18082" {
		t.Fatalf("base urls = api %q dashboard %q", cfg.APIBaseURL, cfg.DashboardBaseURL)
	}
	if cfg.SidebarBaseURL != "http://127.0.0.1:18083" || cfg.OperationBaseURL != "http://127.0.0.1:18084" {
		t.Fatalf("base urls = sidebar %q operation %q", cfg.SidebarBaseURL, cfg.OperationBaseURL)
	}
}

func TestStandaloneIgnoresPHPUpstreamAndUsesListenAddrBaseURL(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_STANDALONE", "1")
	t.Setenv("MOCHAT_GO_ADDR", "127.0.0.1:18090")
	t.Setenv("MOCHAT_PHP_UPSTREAM", "http://127.0.0.1:9501")
	t.Setenv("MOCHAT_SOURCE_ROOT", "/tmp/mochat-php")
	t.Setenv("MOCHAT_COMPAT_MANIFEST", "/tmp/mochat-compat-manifest.json")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PHPUpstream != "" {
		t.Fatalf("PHPUpstream = %q", cfg.PHPUpstream)
	}
	if cfg.SourceRoot != "" || cfg.ManifestPath != "" {
		t.Fatalf("standalone should ignore external source and manifest env: source=%q manifest=%q", cfg.SourceRoot, cfg.ManifestPath)
	}
	if cfg.APIBaseURL != "http://127.0.0.1:18090" || cfg.DashboardBaseURL != "http://127.0.0.1:18090" || cfg.SidebarBaseURL != "http://127.0.0.1:18090" || cfg.OperationBaseURL != "http://127.0.0.1:18090" {
		t.Fatalf("base urls = api %q dashboard %q sidebar %q operation %q", cfg.APIBaseURL, cfg.DashboardBaseURL, cfg.SidebarBaseURL, cfg.OperationBaseURL)
	}
}

func TestStandaloneExplicitFrontendAddrUpdatesBaseURL(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_STANDALONE", "1")
	t.Setenv("MOCHAT_SIDEBAR_FRONTEND_ADDR", "127.0.0.1:28081")
	t.Setenv("MOCHAT_OPERATION_FRONTEND_ADDR", "127.0.0.1:28082")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SidebarBaseURL != "http://127.0.0.1:28081" || cfg.OperationBaseURL != "http://127.0.0.1:28082" {
		t.Fatalf("base urls = sidebar %q operation %q", cfg.SidebarBaseURL, cfg.OperationBaseURL)
	}
}

func TestAuthRequiresMySQLAndJWTSecret(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_AUTH", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_AUTH", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	_, err = FromEnv()
	requireErrorContains(t, err, "SIMPLE_JWT_SECRET")
}

func TestLoginShowDevHeaderDoesNotRequireJWTSecret(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_LOGIN_SHOW", "1")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MigrateLoginShow || !cfg.DevAuthHeader {
		t.Fatalf("unexpected flags: %+v", cfg)
	}
}

func TestLoginShowJWTRequiresSecret(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_LOGIN_SHOW", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	_, err := FromEnv()
	requireErrorContains(t, err, "SIMPLE_JWT_SECRET")
}

func TestUserAdminReadRoutesAllowDevHeader(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_USER_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_USER_SHOW", "1")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MigrateUserIndex || !cfg.MigrateUserShow || !cfg.DevAuthHeader {
		t.Fatalf("unexpected flags: %+v", cfg)
	}
}

func TestUserAdminWriteRoutesRequireRealJWT(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_USER_STORE", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "state-changing migrated routes require PHP JWT auth")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_USER_STORE", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_USER_UPDATE", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_USER_STATUS_UPDATE", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_USER_PASSWORD_RESET", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_USER_PASSWORD_UPDATE", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MigrateUserStore || !cfg.MigrateUserUpdate || !cfg.MigrateUserStatusUpdate || !cfg.MigrateUserPasswordReset || !cfg.MigrateUserPasswordUpdate {
		t.Fatalf("unexpected flags: %+v", cfg)
	}
}

func TestPermissionByUserRequiresMySQL(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_PERMISSION_BY_USER", "1")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")
}

func TestCorpSelectRequiresMySQLAndJWTSecret(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_CORP_SELECT", "1")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_CORP_SELECT", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	_, err = FromEnv()
	requireErrorContains(t, err, "SIMPLE_JWT_SECRET")
}

func TestCorpBindRequiresRedisAndRealJWT(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_CORP_BIND", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "state-changing migrated routes require PHP JWT auth")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_CORP_BIND", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_GO_SKIP_JWT_BLACKLIST", "1")

	_, err = FromEnv()
	requireErrorContains(t, err, "state-changing migrated routes cannot skip JWT blacklist checks")
}

func TestCorpAdminReadRoutesAllowDevHeader(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_CORP_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_CORP_SHOW", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_CORP_DATA_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_CORP_DATA_LINE_CHAT", "1")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MigrateCorpIndex || !cfg.MigrateCorpShow || !cfg.MigrateCorpDataIndex || !cfg.MigrateCorpDataLineChat || !cfg.DevAuthHeader {
		t.Fatalf("unexpected flags: %+v", cfg)
	}
}

func TestChatToolConfigAllowsDevHeaderAndReadsBaseURLs(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_CHAT_TOOL_CONFIG", "1")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_API_BASE_URL", "http://api.example.com")
	t.Setenv("MOCHAT_SIDEBAR_BASE_URL", "http://sidebar.example.com")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MigrateChatToolConfig || !cfg.DevAuthHeader {
		t.Fatalf("unexpected flags: %+v", cfg)
	}
	if cfg.APIBaseURL != "http://api.example.com" || cfg.SidebarBaseURL != "http://sidebar.example.com" {
		t.Fatalf("base urls = api %q sidebar %q", cfg.APIBaseURL, cfg.SidebarBaseURL)
	}
}

func TestChatToolConfigJWTRequiresSecret(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_CHAT_TOOL_CONFIG", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	_, err := FromEnv()
	requireErrorContains(t, err, "SIMPLE_JWT_SECRET")
}

func TestSaaSAlertDashboardRequiresMySQLAndRealJWT(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_ALERT_DASHBOARD", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_ALERT_DASHBOARD", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	_, err = FromEnv()
	requireErrorContains(t, err, "SIMPLE_JWT_SECRET")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_ALERT_DASHBOARD", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")

	_, err = FromEnv()
	requireErrorContains(t, err, "state-changing migrated routes require PHP JWT auth")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_ALERT_DASHBOARD", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableSaaSAlertDashboard {
		t.Fatalf("EnableSaaSAlertDashboard = false")
	}
}

func TestSaaSAdminDashboardRequiresMySQLAndReadsPlatformTenant(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	_, err = FromEnv()
	requireErrorContains(t, err, "SIMPLE_JWT_SECRET")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")
	t.Setenv("MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID", "9")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableSaaSAdminDashboard || cfg.SaaSPlatformAdminTenantID != 9 {
		t.Fatalf("SaaS admin dashboard config = enabled %v platformTenantID %d", cfg.EnableSaaSAdminDashboard, cfg.SaaSPlatformAdminTenantID)
	}
}

func TestSaaSReleaseSourceFingerprintEnvironmentFallback(t *testing.T) {
	clearEnv(t)
	fingerprint := strings.Repeat("A", 64)
	t.Setenv("MOCHAT_GO_RELEASE_SOURCE_FINGERPRINT", fingerprint)

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SaaSReleaseSourceFingerprint != strings.ToLower(fingerprint) || cfg.SaaSReleaseSourceFingerprintSource != "environment" {
		t.Fatalf("release fingerprint=%q source=%q", cfg.SaaSReleaseSourceFingerprint, cfg.SaaSReleaseSourceFingerprintSource)
	}

	clearEnv(t)
	t.Setenv("MOCHAT_GO_RELEASE_SOURCE_FINGERPRINT", "not-a-sha256")
	_, err = FromEnv()
	requireErrorContains(t, err, "release source fingerprint")
}

func TestSaaSBillingPortalRequiresMySQLAndJWT(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_BILLING_PORTAL", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_BILLING_PORTAL", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	_, err = FromEnv()
	requireErrorContains(t, err, "SIMPLE_JWT_SECRET")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_BILLING_PORTAL", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_REDIS_ADDR", "127.0.0.1:6379")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableSaaSBillingPortal {
		t.Fatalf("EnableSaaSBillingPortal = false")
	}
}

func TestSaaSAlertWebhookConfig(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_ALERT_WEBHOOK_URL", "https://alerts.example.com/hook")
	t.Setenv("MOCHAT_GO_SAAS_ALERT_WEBHOOK_SECRET", "alert-secret")
	t.Setenv("MOCHAT_GO_SAAS_ALERT_WEBHOOK_TIMEOUT_SECONDS", "9")
	t.Setenv("MOCHAT_GO_SAAS_ALERT_WEBHOOK_RETRY_ATTEMPTS", "3")
	t.Setenv("MOCHAT_GO_SAAS_ALERT_WEBHOOK_RETRY_DELAY_MS", "125")
	t.Setenv("MOCHAT_GO_SAAS_ALERT_WEBHOOK_TITLE_TEMPLATE", "租户 {{.TenantID}} {{.Metric}} 告警")
	t.Setenv("MOCHAT_GO_SAAS_ALERT_WEBHOOK_BODY_TEMPLATE", "当前 {{.CurrentValue}}/{{.LimitValue}} 来源 {{.Source}}")
	t.Setenv("MOCHAT_GO_SAAS_ALERT_WEBHOOK_ALLOWED_CIDRS", "127.0.0.1,10.0.0.7/8")
	t.Setenv("MOCHAT_GO_SAAS_ALERT_NOTIFICATION_MAX_ATTEMPTS", "5")
	t.Setenv("MOCHAT_GO_SAAS_ALERT_NOTIFICATION_RETRY_DELAY_SECONDS", "42")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SaaSAlertWebhookURL != "https://alerts.example.com/hook" || cfg.SaaSAlertWebhookSecret != "alert-secret" || cfg.SaaSAlertWebhookTimeout != 9*time.Second || cfg.SaaSAlertWebhookRetryAttempts != 3 || cfg.SaaSAlertWebhookRetryDelay != 125*time.Millisecond || cfg.SaaSAlertWebhookTitleTemplate != "租户 {{.TenantID}} {{.Metric}} 告警" || cfg.SaaSAlertWebhookBodyTemplate != "当前 {{.CurrentValue}}/{{.LimitValue}} 来源 {{.Source}}" || !cfg.SaaSAlertWebhookRequireHTTPS || strings.Join(cfg.SaaSAlertWebhookAllowedCIDRs, ",") != "10.0.0.0/8,127.0.0.1/32" || cfg.SaaSAlertNotificationMaxAttempts != 5 || cfg.SaaSAlertNotificationRetryDelay != 42*time.Second {
		t.Fatalf("webhook config = url %q secret %q timeout %s attempts %d delay %s title %q body %q outbox_attempts %d outbox_delay %s", cfg.SaaSAlertWebhookURL, cfg.SaaSAlertWebhookSecret, cfg.SaaSAlertWebhookTimeout, cfg.SaaSAlertWebhookRetryAttempts, cfg.SaaSAlertWebhookRetryDelay, cfg.SaaSAlertWebhookTitleTemplate, cfg.SaaSAlertWebhookBodyTemplate, cfg.SaaSAlertNotificationMaxAttempts, cfg.SaaSAlertNotificationRetryDelay)
	}

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_ALERT_WEBHOOK_URL", "alerts.example.com/hook")

	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_SAAS_ALERT_WEBHOOK_URL")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_ALERT_WEBHOOK_URL", "http://127.0.0.1/hook")
	t.Setenv("MOCHAT_GO_SAAS_ALERT_WEBHOOK_REQUIRE_HTTPS", "0")
	t.Setenv("MOCHAT_GO_SAAS_ALERT_WEBHOOK_ALLOWED_CIDRS", "127.0.0.0/8")
	if _, err = FromEnv(); err != nil {
		t.Fatalf("explicit local webhook exception: %v", err)
	}

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_ALERT_WEBHOOK_URL", "http://alerts.example.com/hook")
	_, err = FromEnv()
	requireErrorContains(t, err, "URL must use HTTPS")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_ALERT_WEBHOOK_URL", "https://10.0.0.8/hook")
	_, err = FromEnv()
	requireErrorContains(t, err, "blocked network")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_ALERT_WEBHOOK_ALLOWED_CIDRS", "bad-cidr")
	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_SAAS_ALERT_WEBHOOK_ALLOWED_CIDRS")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_ALERT_WEBHOOK_TIMEOUT_SECONDS", "0")

	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_SAAS_ALERT_WEBHOOK_TIMEOUT_SECONDS must be a positive integer")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_ALERT_WEBHOOK_RETRY_ATTEMPTS", "0")

	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_SAAS_ALERT_WEBHOOK_RETRY_ATTEMPTS must be a positive integer")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_ALERT_WEBHOOK_RETRY_DELAY_MS", "-1")

	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_SAAS_ALERT_WEBHOOK_RETRY_DELAY_MS")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_ALERT_WEBHOOK_TITLE_TEMPLATE", "租户 {{.UnknownField}}")

	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_SAAS_ALERT_WEBHOOK_TITLE_TEMPLATE contains unsupported SaaS alert template field")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_ALERT_WEBHOOK_BODY_TEMPLATE", "{{")

	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_SAAS_ALERT_WEBHOOK_BODY_TEMPLATE must be a valid Go text/template")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_ALERT_NOTIFICATION_MAX_ATTEMPTS", "0")

	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_SAAS_ALERT_NOTIFICATION_MAX_ATTEMPTS must be a positive integer")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_ALERT_NOTIFICATION_RETRY_DELAY_SECONDS", "-1")

	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_SAAS_ALERT_NOTIFICATION_RETRY_DELAY_SECONDS")
}

func TestSaaSAlertNotificationDispatchCronRequiresMySQLAndReadsOptions(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_ALERT_NOTIFICATION_DISPATCH_CRON", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_ALERT_NOTIFICATION_DISPATCH_CRON", "1")
	t.Setenv("MOCHAT_GO_SAAS_ALERT_NOTIFICATION_DISPATCH_CRON_INTERVAL_SECONDS", "45")
	t.Setenv("MOCHAT_GO_SAAS_ALERT_NOTIFICATION_DISPATCH_CRON_RUN_ON_START", "1")
	t.Setenv("MOCHAT_GO_SAAS_ALERT_NOTIFICATION_DISPATCH_LIMIT", "7")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableSaaSAlertNotificationDispatchCron || cfg.SaaSAlertNotificationDispatchCronInterval != 45*time.Second || !cfg.SaaSAlertNotificationDispatchCronRunOnStart || cfg.SaaSAlertNotificationDispatchLimit != 7 {
		t.Fatalf("dispatch cron config = enabled %v interval %s runOnStart %v limit %d", cfg.EnableSaaSAlertNotificationDispatchCron, cfg.SaaSAlertNotificationDispatchCronInterval, cfg.SaaSAlertNotificationDispatchCronRunOnStart, cfg.SaaSAlertNotificationDispatchLimit)
	}
}

func TestSaaSOperationQueueAssignmentReminderCronRequiresMySQLAndReadsOptions(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_OPERATION_QUEUE_ASSIGNMENT_REMINDER_CRON", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_OPERATION_QUEUE_ASSIGNMENT_REMINDER_CRON", "1")
	t.Setenv("MOCHAT_GO_SAAS_OPERATION_QUEUE_ASSIGNMENT_REMINDER_CRON_INTERVAL_SECONDS", "90")
	t.Setenv("MOCHAT_GO_SAAS_OPERATION_QUEUE_ASSIGNMENT_REMINDER_CRON_RUN_ON_START", "1")
	t.Setenv("MOCHAT_GO_SAAS_OPERATION_QUEUE_ASSIGNMENT_REMINDER_LIMIT", "250")
	t.Setenv("MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID", "9")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableSaaSOperationQueueAssignmentReminderCron ||
		cfg.SaaSOperationQueueAssignmentReminderCronInterval != 90*time.Second ||
		!cfg.SaaSOperationQueueAssignmentReminderCronRunOnStart ||
		cfg.SaaSOperationQueueAssignmentReminderLimit != 250 ||
		cfg.SaaSPlatformAdminTenantID != 9 {
		t.Fatalf("assignment reminder cron config = enabled %v interval %s runOnStart %v limit %d platformTenantID %d", cfg.EnableSaaSOperationQueueAssignmentReminderCron, cfg.SaaSOperationQueueAssignmentReminderCronInterval, cfg.SaaSOperationQueueAssignmentReminderCronRunOnStart, cfg.SaaSOperationQueueAssignmentReminderLimit, cfg.SaaSPlatformAdminTenantID)
	}
}

func TestSaaSApprovalReminderCronRequiresMySQLAndReadsOptions(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_APPROVAL_REMINDER_CRON", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_APPROVAL_REMINDER_CRON", "1")
	t.Setenv("MOCHAT_GO_SAAS_APPROVAL_REMINDER_CRON_INTERVAL_SECONDS", "75")
	t.Setenv("MOCHAT_GO_SAAS_APPROVAL_REMINDER_CRON_RUN_ON_START", "1")
	t.Setenv("MOCHAT_GO_SAAS_APPROVAL_REMINDER_LIMIT", "80")
	t.Setenv("MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID", "9")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableSaaSApprovalReminderCron || cfg.SaaSApprovalReminderCronInterval != 75*time.Second ||
		!cfg.SaaSApprovalReminderCronRunOnStart || cfg.SaaSApprovalReminderLimit != 80 || cfg.SaaSPlatformAdminTenantID != 9 {
		t.Fatalf("approval reminder cron config = enabled %v interval %s runOnStart %v limit %d platformTenantID %d",
			cfg.EnableSaaSApprovalReminderCron, cfg.SaaSApprovalReminderCronInterval, cfg.SaaSApprovalReminderCronRunOnStart,
			cfg.SaaSApprovalReminderLimit, cfg.SaaSPlatformAdminTenantID)
	}
}

func TestSaaSSystemHealthCronRequiresMySQLAndReadsOptions(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_SYSTEM_HEALTH_CRON", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_SYSTEM_HEALTH_CRON", "1")
	t.Setenv("MOCHAT_GO_SAAS_SYSTEM_HEALTH_CRON_INTERVAL_SECONDS", "90")
	t.Setenv("MOCHAT_GO_SAAS_SYSTEM_HEALTH_CRON_RUN_ON_START", "1")
	t.Setenv("MOCHAT_GO_SAAS_SYSTEM_HEALTH_FAILURE_WINDOW_HOURS", "48")
	t.Setenv("MOCHAT_GO_SAAS_SYSTEM_HEALTH_NOTIFICATION_STALE_MINUTES", "30")
	t.Setenv("MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID", "9")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableSaaSSystemHealthCron || cfg.SaaSSystemHealthCronInterval != 90*time.Second ||
		!cfg.SaaSSystemHealthCronRunOnStart || cfg.SaaSSystemHealthFailureWindowHours != 48 ||
		cfg.SaaSSystemHealthNotificationStaleMinutes != 30 || cfg.SaaSPlatformAdminTenantID != 9 {
		t.Fatalf("system health cron config = enabled %v interval %s runOnStart %v windowHours %d staleMinutes %d platformTenantID %d",
			cfg.EnableSaaSSystemHealthCron, cfg.SaaSSystemHealthCronInterval, cfg.SaaSSystemHealthCronRunOnStart,
			cfg.SaaSSystemHealthFailureWindowHours, cfg.SaaSSystemHealthNotificationStaleMinutes, cfg.SaaSPlatformAdminTenantID)
	}

	for key, value := range map[string]string{
		"MOCHAT_GO_SAAS_SYSTEM_HEALTH_CRON_INTERVAL_SECONDS":      "0",
		"MOCHAT_GO_SAAS_SYSTEM_HEALTH_FAILURE_WINDOW_HOURS":       "721",
		"MOCHAT_GO_SAAS_SYSTEM_HEALTH_NOTIFICATION_STALE_MINUTES": "10081",
	} {
		clearEnv(t)
		t.Setenv("MOCHAT_GO_ENABLE_SAAS_SYSTEM_HEALTH_CRON", "1")
		t.Setenv("MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID", "9")
		t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
		t.Setenv(key, value)
		if _, err := FromEnv(); err == nil {
			t.Fatalf("expected invalid %s=%s", key, value)
		}
	}
}

func TestSaaSBackupCronRequiresEncryptionAndReadsOptions(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_BACKUP_CRON", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_BACKUP_CRON", "1")
	t.Setenv("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY", "0707070707070707070707070707070707070707070707070707070707070707")
	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_BACKUP_CRON", "1")
	t.Setenv("MOCHAT_GO_SAAS_BACKUP_CRON_INTERVAL_SECONDS", "120")
	t.Setenv("MOCHAT_GO_SAAS_BACKUP_CRON_RUN_ON_START", "1")
	t.Setenv("MOCHAT_GO_SAAS_BACKUP_ROOT", "/tmp/mochat-backups")
	t.Setenv("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY", "0707070707070707070707070707070707070707070707070707070707070707")
	t.Setenv("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY_ID", "test-key")
	t.Setenv("MOCHAT_GO_SAAS_BACKUP_DUMP_BINARY", "mariadb-dump")
	t.Setenv("MOCHAT_GO_SAAS_BACKUP_RESTORE_BINARY", "mariadb")
	t.Setenv("MOCHAT_GO_SAAS_BACKUP_RESTORE_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat_restore_test")
	t.Setenv("MOCHAT_GO_SAAS_BACKUP_RESTORE_DATABASE_PREFIX", "mochat_restore_")
	t.Setenv("MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID", "9")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableSaaSBackupCron || cfg.SaaSBackupCronInterval != 120*time.Second || !cfg.SaaSBackupCronRunOnStart ||
		cfg.SaaSBackupRoot != "/tmp/mochat-backups" || cfg.SaaSBackupEncryptionKeyID != "test-key" ||
		cfg.SaaSBackupDumpBinary != "mariadb-dump" || cfg.SaaSBackupRestoreBinary != "mariadb" ||
		cfg.SaaSBackupRestoreDatabasePrefix != "mochat_restore_" || cfg.SaaSPlatformAdminTenantID != 9 {
		t.Fatalf("backup cron config = %+v", cfg)
	}

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_BACKUP_CRON", "1")
	t.Setenv("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEYS", `{"rotated":"0707070707070707070707070707070707070707070707070707070707070707"}`)
	t.Setenv("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY_ID", "rotated")
	t.Setenv("MOCHAT_GO_SAAS_BACKUP_RESTORE_AUTO_PROVISION", "1")
	t.Setenv("MOCHAT_GO_SAAS_BACKUP_RESTORE_ADMIN_DSN", "root:pass@tcp(127.0.0.1:3306)/")
	t.Setenv("MOCHAT_GO_SAAS_BACKUP_S3_ENDPOINT", "http://127.0.0.1:9000")
	t.Setenv("MOCHAT_GO_SAAS_BACKUP_S3_BUCKET", "mochat-backups")
	t.Setenv("MOCHAT_GO_SAAS_BACKUP_S3_ACCESS_KEY_ID", "minio")
	t.Setenv("MOCHAT_GO_SAAS_BACKUP_S3_SECRET_ACCESS_KEY", "minio-secret")
	t.Setenv("MOCHAT_GO_SAAS_BACKUP_S3_PREFIX", "production/database")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	cfg, err = FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SaaSBackupEncryptionKeys == "" || cfg.SaaSBackupEncryptionKeyID != "rotated" ||
		!cfg.SaaSBackupRestoreAutoProvision || cfg.SaaSBackupRestoreAdminDSN == "" ||
		cfg.SaaSBackupS3Bucket != "mochat-backups" || cfg.SaaSBackupS3Prefix != "production/database" {
		t.Fatalf("resilient backup config = %+v", cfg)
	}

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_BACKUP_RESTORE_AUTO_PROVISION", "1")
	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_SAAS_BACKUP_RESTORE_ADMIN_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_BACKUP_S3_ENDPOINT", "127.0.0.1:9000")
	_, err = FromEnv()
	requireErrorContains(t, err, "must be configured together")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_BACKUP_RESTORE_DATABASE_PREFIX", "../unsafe")
	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_SAAS_BACKUP_RESTORE_DATABASE_PREFIX")
}

func TestSaaSAlertCredentialEncryptionConfig(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_ALERT_CREDENTIAL_REQUIRE_ENCRYPTION", "1")
	_, err := FromEnv()
	requireErrorContains(t, err, "encryption is required")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_ALERT_CREDENTIAL_ENCRYPTION_KEYS", `{"alert-q3":"1111111111111111111111111111111111111111111111111111111111111111"}`)
	t.Setenv("MOCHAT_GO_SAAS_ALERT_CREDENTIAL_ENCRYPTION_KEY_ID", "alert-q3")
	t.Setenv("MOCHAT_GO_SAAS_ALERT_CREDENTIAL_REQUIRE_ENCRYPTION", "1")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SaaSAlertCredentialEncryptionKeys == "" || cfg.SaaSAlertCredentialEncryptionKeyID != "alert-q3" ||
		!cfg.SaaSAlertCredentialRequireEncryption || !cfg.SaaSAlertCredentialDedicatedConfigured {
		t.Fatalf("dedicated alert credential config = %+v", cfg)
	}

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEYS", `{"identity-q3":"1212121212121212121212121212121212121212121212121212121212121212"}`)
	t.Setenv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY_ID", "identity-q3")
	cfg, err = FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SaaSAlertCredentialEncryptionKeys == "" || cfg.SaaSAlertCredentialEncryptionKeyID != "identity-q3" || cfg.SaaSAlertCredentialDedicatedConfigured {
		t.Fatalf("fallback alert credential config = %+v", cfg)
	}

	clearEnv(t)
	alertSingleKey := "1414141414141414141414141414141414141414141414141414141414141414"
	t.Setenv("MOCHAT_GO_SAAS_ALERT_CREDENTIAL_ENCRYPTION_KEY", alertSingleKey)
	t.Setenv("MOCHAT_GO_SAAS_ALERT_CREDENTIAL_ENCRYPTION_KEY_ID", "alert-single")
	t.Setenv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEYS", `{"identity-old":"1515151515151515151515151515151515151515151515151515151515151515"}`)
	t.Setenv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY_ID", "identity-old")
	cfg, err = FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SaaSAlertCredentialEncryptionKey != alertSingleKey || cfg.SaaSAlertCredentialEncryptionKeys != "" ||
		cfg.SaaSAlertCredentialEncryptionKeyID != "alert-single" || !cfg.SaaSAlertCredentialDedicatedConfigured {
		t.Fatalf("dedicated single-key alert credential config mixed fallback domain = %+v", cfg)
	}

	clearEnv(t)
	identitySingleKey := "1616161616161616161616161616161616161616161616161616161616161616"
	t.Setenv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY", identitySingleKey)
	t.Setenv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY_ID", "identity-single")
	t.Setenv("MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEYS", `{"compliance-old":"1717171717171717171717171717171717171717171717171717171717171717"}`)
	t.Setenv("MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY_ID", "compliance-old")
	cfg, err = FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SaaSAlertCredentialEncryptionKey != identitySingleKey || cfg.SaaSAlertCredentialEncryptionKeys != "" ||
		cfg.SaaSAlertCredentialEncryptionKeyID != "identity-single" || cfg.SaaSAlertCredentialDedicatedConfigured {
		t.Fatalf("fallback alert credential config mixed encryption domains = %+v", cfg)
	}

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_ALERT_CREDENTIAL_ENCRYPTION_KEYS", `{"old":"1313131313131313131313131313131313131313131313131313131313131313"}`)
	t.Setenv("MOCHAT_GO_SAAS_ALERT_CREDENTIAL_ENCRYPTION_KEY_ID", "missing")
	_, err = FromEnv()
	requireErrorContains(t, err, "active encryption key")
}

func TestWeComCredentialEncryptionConfig(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_WECOM_CREDENTIAL_REQUIRE_ENCRYPTION", "1")
	_, err := FromEnv()
	requireErrorContains(t, err, "encryption is required")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEYS", `{"wecom-q3":"1818181818181818181818181818181818181818181818181818181818181818"}`)
	t.Setenv("MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY_ID", "wecom-q3")
	t.Setenv("MOCHAT_GO_WECOM_CREDENTIAL_REQUIRE_ENCRYPTION", "1")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WeComCredentialEncryptionKeys == "" || cfg.WeComCredentialEncryptionKeyID != "wecom-q3" ||
		!cfg.WeComCredentialRequireEncryption || !cfg.WeComCredentialDedicatedConfigured {
		t.Fatalf("dedicated WeCom credential config = %+v", cfg)
	}

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEYS", `{"identity-q3":"1919191919191919191919191919191919191919191919191919191919191919"}`)
	t.Setenv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY_ID", "identity-q3")
	cfg, err = FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WeComCredentialEncryptionKeys == "" || cfg.WeComCredentialEncryptionKeyID != "identity-q3" || cfg.WeComCredentialDedicatedConfigured {
		t.Fatalf("fallback WeCom credential config = %+v", cfg)
	}

	clearEnv(t)
	weComSingleKey := "2020202020202020202020202020202020202020202020202020202020202020"
	t.Setenv("MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY", weComSingleKey)
	t.Setenv("MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY_ID", "wecom-single")
	t.Setenv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEYS", `{"identity-old":"2121212121212121212121212121212121212121212121212121212121212121"}`)
	t.Setenv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY_ID", "identity-old")
	cfg, err = FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WeComCredentialEncryptionKey != weComSingleKey || cfg.WeComCredentialEncryptionKeys != "" ||
		cfg.WeComCredentialEncryptionKeyID != "wecom-single" || !cfg.WeComCredentialDedicatedConfigured {
		t.Fatalf("dedicated single-key WeCom credential config mixed fallback domain = %+v", cfg)
	}

	clearEnv(t)
	t.Setenv("MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEYS", `{"old":"2222222222222222222222222222222222222222222222222222222222222222"}`)
	t.Setenv("MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY_ID", "missing")
	_, err = FromEnv()
	requireErrorContains(t, err, "active encryption key")
}

func TestSaaSComplianceCronRequiresEncryptionAndReadsOptions(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_COMPLIANCE_CRON", "1")
	t.Setenv("MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID", "9")

	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_COMPLIANCE_CRON", "1")
	t.Setenv("MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID", "9")
	t.Setenv("MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY", "0808080808080808080808080808080808080808080808080808080808080808")
	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_COMPLIANCE_CRON", "1")
	t.Setenv("MOCHAT_GO_SAAS_COMPLIANCE_CRON_INTERVAL_SECONDS", "180")
	t.Setenv("MOCHAT_GO_SAAS_COMPLIANCE_CRON_RUN_ON_START", "1")
	t.Setenv("MOCHAT_GO_SAAS_COMPLIANCE_ARTIFACT_ROOT", "/tmp/mochat-compliance")
	t.Setenv("MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEYS", `{"compliance-q3":"0808080808080808080808080808080808080808080808080808080808080808"}`)
	t.Setenv("MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY_ID", "compliance-q3")
	t.Setenv("MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID", "9")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableSaaSComplianceCron || cfg.SaaSComplianceCronInterval != 180*time.Second || !cfg.SaaSComplianceCronRunOnStart ||
		cfg.SaaSComplianceArtifactRoot != "/tmp/mochat-compliance" || cfg.SaaSComplianceEncryptionKeyID != "compliance-q3" ||
		cfg.SaaSComplianceEncryptionKeys == "" || cfg.SaaSPlatformAdminTenantID != 9 {
		t.Fatalf("compliance cron config = %+v", cfg)
	}

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY", "0909090909090909090909090909090909090909090909090909090909090909")
	t.Setenv("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY_ID", "shared-q3")
	cfg, err = FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SaaSComplianceEncryptionKey == "" || cfg.SaaSComplianceEncryptionKeyID != "shared-q3" {
		t.Fatalf("compliance fallback key config = %+v", cfg)
	}

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_COMPLIANCE_CRON_INTERVAL_SECONDS", "0")
	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_SAAS_COMPLIANCE_CRON_INTERVAL_SECONDS")
}

func TestSaaSAuditAnchorCronRequiresKeyAndReadsOptions(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_AUDIT_ANCHOR_CRON", "1")
	t.Setenv("MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID", "9")
	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_SAAS_AUDIT_ANCHOR_HMAC_KEY")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_AUDIT_ANCHOR_CRON", "1")
	t.Setenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_CRON_INTERVAL_SECONDS", "7200")
	t.Setenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_CRON_RUN_ON_START", "1")
	t.Setenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_LIMIT", "250")
	t.Setenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_ARTIFACT_ROOT", "/tmp/mochat-audit-anchors")
	t.Setenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_HMAC_KEYS", `{"audit-q3":"0808080808080808080808080808080808080808080808080808080808080808"}`)
	t.Setenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_HMAC_KEY_ID", "audit-q3")
	t.Setenv("MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID", "9")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableSaaSAuditAnchorCron || cfg.SaaSAuditAnchorCronInterval != 2*time.Hour ||
		!cfg.SaaSAuditAnchorCronRunOnStart || cfg.SaaSAuditAnchorLimit != 250 ||
		cfg.SaaSAuditAnchorArtifactRoot != "/tmp/mochat-audit-anchors" ||
		cfg.SaaSAuditAnchorHMACKeyID != "audit-q3" || cfg.SaaSAuditAnchorHMACKeys == "" {
		t.Fatalf("audit anchor cron config = %+v", cfg)
	}

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_LIMIT", "501")
	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_SAAS_AUDIT_ANCHOR_LIMIT")
}

func TestSaaSAuditAnchorRemoteObjectLockConfig(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_REQUIRE_REMOTE", "1")
	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_SAAS_AUDIT_ANCHOR_REQUIRE_REMOTE")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_ENDPOINT", "http://127.0.0.1:9000")
	_, err = FromEnv()
	requireErrorContains(t, err, "must be configured together")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_REQUIRE_REMOTE", "1")
	t.Setenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_ENDPOINT", "http://127.0.0.1:9000")
	t.Setenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_BUCKET", "mochat-audit-anchors")
	t.Setenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_REGION", "cn-test-1")
	t.Setenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_ACCESS_KEY_ID", "minio")
	t.Setenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_SECRET_ACCESS_KEY", "minio-secret")
	t.Setenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_SESSION_TOKEN", "session-token")
	t.Setenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_USE_SSL", "0")
	t.Setenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_PREFIX", "production/audit-anchors")
	t.Setenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_RETENTION_MODE", "governance")
	t.Setenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_RETENTION_DAYS", "365")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.SaaSAuditAnchorRequireRemote || cfg.SaaSAuditAnchorS3Endpoint != "http://127.0.0.1:9000" ||
		cfg.SaaSAuditAnchorS3Bucket != "mochat-audit-anchors" || cfg.SaaSAuditAnchorS3Region != "cn-test-1" ||
		cfg.SaaSAuditAnchorS3AccessKeyID != "minio" || cfg.SaaSAuditAnchorS3SecretAccessKey != "minio-secret" ||
		cfg.SaaSAuditAnchorS3SessionToken != "session-token" || cfg.SaaSAuditAnchorS3UseSSL ||
		cfg.SaaSAuditAnchorS3Prefix != "production/audit-anchors" || cfg.SaaSAuditAnchorS3RetentionMode != "governance" ||
		cfg.SaaSAuditAnchorS3RetentionDays != 365 {
		t.Fatalf("audit anchor remote config = %+v", cfg)
	}

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_RETENTION_MODE", "invalid")
	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_RETENTION_MODE")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_RETENTION_DAYS", "0")
	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_RETENTION_DAYS")
}

func TestSaaSIdentitySecurityRequiresKeyAndReadsOptions(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_IDENTITY_SECURITY", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_IDENTITY_ENFORCE_SESSIONS", "1")
	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_ENABLE_SAAS_IDENTITY_SECURITY")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_IDENTITY_CLEANUP_CRON", "1")
	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_ENABLE_SAAS_IDENTITY_SECURITY")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_IDENTITY_SECURITY", "1")
	t.Setenv("MOCHAT_GO_SAAS_IDENTITY_ENFORCE_SESSIONS", "1")
	t.Setenv("MOCHAT_GO_SAAS_IDENTITY_TRUST_PROXY_HEADERS", "1")
	t.Setenv("MOCHAT_GO_SAAS_SERVICE_ACCOUNT_TRUST_PROXY_HEADERS", "1")
	t.Setenv("MOCHAT_GO_SAAS_TRUSTED_PROXY_CIDRS", "127.0.0.1, 10.0.0.7/8")
	t.Setenv("MOCHAT_GO_SAAS_IDENTITY_ISSUER", "MoChat Test")
	t.Setenv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEYS", `{"identity-q3":"1010101010101010101010101010101010101010101010101010101010101010"}`)
	t.Setenv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY_ID", "identity-q3")
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_IDENTITY_CLEANUP_CRON", "1")
	t.Setenv("MOCHAT_GO_SAAS_IDENTITY_CLEANUP_CRON_INTERVAL_SECONDS", "600")
	t.Setenv("MOCHAT_GO_SAAS_IDENTITY_CLEANUP_CRON_RUN_ON_START", "1")
	t.Setenv("MOCHAT_GO_SAAS_IDENTITY_CLEANUP_LIMIT", "321")
	t.Setenv("MOCHAT_GO_SAAS_TENANT_DOMAIN_DNS_SERVER", "127.0.0.1:15353")
	t.Setenv("MOCHAT_GO_SAAS_TENANT_DOMAIN_DNS_TIMEOUT_SECONDS", "7")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableSaaSIdentitySecurity || !cfg.SaaSIdentityEnforceSessions || !cfg.SaaSIdentityTrustProxyHeaders || !cfg.SaaSServiceAccountTrustProxyHeaders ||
		strings.Join(cfg.SaaSTrustedProxyCIDRs, ",") != "10.0.0.0/8,127.0.0.1/32" ||
		cfg.SaaSIdentityIssuer != "MoChat Test" || cfg.SaaSIdentityEncryptionKeyID != "identity-q3" ||
		!cfg.EnableSaaSIdentityCleanupCron || cfg.SaaSIdentityCleanupCronInterval != 10*time.Minute ||
		!cfg.SaaSIdentityCleanupCronRunOnStart || cfg.SaaSIdentityCleanupLimit != 321 ||
		cfg.SaaSTenantDomainDNSServer != "127.0.0.1:15353" || cfg.SaaSTenantDomainDNSTimeout != 7*time.Second {
		t.Fatalf("identity security config = %+v", cfg)
	}

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_IDENTITY_TRUST_PROXY_HEADERS", "1")
	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_SAAS_TRUSTED_PROXY_CIDRS")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_SERVICE_ACCOUNT_TRUST_PROXY_HEADERS", "1")
	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_SAAS_TRUSTED_PROXY_CIDRS")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_TRUSTED_PROXY_CIDRS", "bad-cidr")
	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_SAAS_TRUSTED_PROXY_CIDRS")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_IDENTITY_CLEANUP_LIMIT", "5001")
	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_SAAS_IDENTITY_CLEANUP_LIMIT")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_TENANT_DOMAIN_DNS_SERVER", "127.0.0.1")
	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_SAAS_TENANT_DOMAIN_DNS_SERVER")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_TENANT_DOMAIN_DNS_TIMEOUT_SECONDS", "31")
	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_SAAS_TENANT_DOMAIN_DNS_TIMEOUT_SECONDS")
}

func TestSaaSServiceAccountUsageCleanupCronRequiresMySQLAndReadsOptions(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_SERVICE_ACCOUNT_USAGE_CLEANUP_CRON", "1")
	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_SERVICE_ACCOUNT_USAGE_CLEANUP_CRON", "1")
	t.Setenv("MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_CLEANUP_CRON_INTERVAL_SECONDS", "7200")
	t.Setenv("MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_CLEANUP_CRON_RUN_ON_START", "1")
	t.Setenv("MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_CLEANUP_LIMIT", "4567")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableSaaSServiceAccountUsageCleanupCron || cfg.SaaSServiceAccountUsageCleanupCronInterval != 2*time.Hour ||
		!cfg.SaaSServiceAccountUsageCleanupCronRunOnStart || cfg.SaaSServiceAccountUsageCleanupLimit != 4567 {
		t.Fatalf("service account usage cleanup config = %+v", cfg)
	}

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_CLEANUP_LIMIT", "1000001")
	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_CLEANUP_LIMIT")
}

func TestSaaSServiceAccountUsageAlertCronRequiresMySQLAndReadsOptions(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_SERVICE_ACCOUNT_USAGE_ALERT_CRON", "1")
	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_SERVICE_ACCOUNT_USAGE_ALERT_CRON", "1")
	t.Setenv("MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_ALERT_CRON_INTERVAL_SECONDS", "600")
	t.Setenv("MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_ALERT_CRON_RUN_ON_START", "1")
	t.Setenv("MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_ALERT_LIMIT", "123")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableSaaSServiceAccountUsageAlertCron || cfg.SaaSServiceAccountUsageAlertCronInterval != 10*time.Minute ||
		!cfg.SaaSServiceAccountUsageAlertCronRunOnStart || cfg.SaaSServiceAccountUsageAlertLimit != 123 {
		t.Fatalf("service account usage alert config = %+v", cfg)
	}

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_ALERT_LIMIT", "501")
	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_ALERT_LIMIT")
}

func TestSaaSServiceAccountPepperConfiguration(t *testing.T) {
	clearEnv(t)
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SaaSServiceAccountKeyPepper != "" || cfg.SaaSServiceAccountKeyPeppers != "" ||
		cfg.SaaSServiceAccountKeyPepperID != "primary" || !cfg.SaaSServiceAccountAllowLegacyJWTPepper ||
		cfg.SaaSServiceAccountRequireDedicatedPepper {
		t.Fatalf("default service account pepper config = %+v", cfg)
	}

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_SERVICE_ACCOUNT_KEY_PEPPER", strings.Repeat("07", 32))
	t.Setenv("MOCHAT_GO_SAAS_SERVICE_ACCOUNT_KEY_PEPPERS", `{"2026-q2":"`+strings.Repeat("08", 32)+`"}`)
	t.Setenv("MOCHAT_GO_SAAS_SERVICE_ACCOUNT_KEY_PEPPER_ID", "2026-q3")
	t.Setenv("MOCHAT_GO_SAAS_SERVICE_ACCOUNT_ALLOW_LEGACY_JWT_PEPPER", "0")
	t.Setenv("MOCHAT_GO_SAAS_SERVICE_ACCOUNT_REQUIRE_DEDICATED_PEPPER", "1")
	cfg, err = FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SaaSServiceAccountKeyPepperID != "2026-q3" || cfg.SaaSServiceAccountAllowLegacyJWTPepper ||
		!cfg.SaaSServiceAccountRequireDedicatedPepper || cfg.SaaSServiceAccountKeyPeppers == "" {
		t.Fatalf("service account pepper config = %+v", cfg)
	}

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_SERVICE_ACCOUNT_REQUIRE_DEDICATED_PEPPER", "1")
	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_SAAS_SERVICE_ACCOUNT_KEY_PEPPER")
}

func TestSaaSNotificationHealthRecoveryCronRequiresMySQLAndReadsOptions(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_NOTIFICATION_HEALTH_RECOVERY_CRON", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_NOTIFICATION_HEALTH_RECOVERY_CRON", "1")
	t.Setenv("MOCHAT_GO_SAAS_NOTIFICATION_HEALTH_RECOVERY_CRON_INTERVAL_SECONDS", "120")
	t.Setenv("MOCHAT_GO_SAAS_NOTIFICATION_HEALTH_RECOVERY_CRON_RUN_ON_START", "1")
	t.Setenv("MOCHAT_GO_SAAS_NOTIFICATION_HEALTH_RECOVERY_WINDOW_HOURS", "48")
	t.Setenv("MOCHAT_GO_SAAS_NOTIFICATION_HEALTH_RECOVERY_STALE_MINUTES", "30")
	t.Setenv("MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID", "9")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableSaaSNotificationHealthRecoveryCron ||
		cfg.SaaSNotificationHealthRecoveryCronInterval != 120*time.Second ||
		!cfg.SaaSNotificationHealthRecoveryCronRunOnStart ||
		cfg.SaaSNotificationHealthRecoveryWindowHours != 48 ||
		cfg.SaaSNotificationHealthRecoveryStaleMinutes != 30 ||
		cfg.SaaSPlatformAdminTenantID != 9 {
		t.Fatalf("notification health recovery cron config = enabled %v interval %s runOnStart %v windowHours %d staleMinutes %d platformTenantID %d", cfg.EnableSaaSNotificationHealthRecoveryCron, cfg.SaaSNotificationHealthRecoveryCronInterval, cfg.SaaSNotificationHealthRecoveryCronRunOnStart, cfg.SaaSNotificationHealthRecoveryWindowHours, cfg.SaaSNotificationHealthRecoveryStaleMinutes, cfg.SaaSPlatformAdminTenantID)
	}

	for key, value := range map[string]string{
		"MOCHAT_GO_SAAS_NOTIFICATION_HEALTH_RECOVERY_CRON_INTERVAL_SECONDS": "0",
		"MOCHAT_GO_SAAS_NOTIFICATION_HEALTH_RECOVERY_WINDOW_HOURS":          "721",
		"MOCHAT_GO_SAAS_NOTIFICATION_HEALTH_RECOVERY_STALE_MINUTES":         "10081",
	} {
		clearEnv(t)
		t.Setenv("MOCHAT_GO_ENABLE_SAAS_NOTIFICATION_HEALTH_RECOVERY_CRON", "1")
		t.Setenv("MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID", "9")
		t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
		t.Setenv(key, value)
		if _, err := FromEnv(); err == nil {
			t.Fatalf("expected invalid %s=%s", key, value)
		}
	}
}

func TestSaaSSubscriptionReconcileCronRequiresMySQLAndReadsOptions(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_SUBSCRIPTION_RECONCILE_CRON", "1")
	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_SUBSCRIPTION_RECONCILE_CRON", "1")
	t.Setenv("MOCHAT_GO_SAAS_SUBSCRIPTION_RECONCILE_CRON_INTERVAL_SECONDS", "45")
	t.Setenv("MOCHAT_GO_SAAS_SUBSCRIPTION_RECONCILE_CRON_RUN_ON_START", "1")
	t.Setenv("MOCHAT_GO_SAAS_SUBSCRIPTION_RECONCILE_LIMIT", "321")
	t.Setenv("MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID", "9")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableSaaSSubscriptionReconcileCron || cfg.SaaSSubscriptionReconcileCronInterval != 45*time.Second ||
		!cfg.SaaSSubscriptionReconcileCronRunOnStart || cfg.SaaSSubscriptionReconcileLimit != 321 || cfg.SaaSPlatformAdminTenantID != 9 {
		t.Fatalf("subscription cron config = enabled %v interval %s runOnStart %v limit %d platformTenantID %d",
			cfg.EnableSaaSSubscriptionReconcileCron, cfg.SaaSSubscriptionReconcileCronInterval,
			cfg.SaaSSubscriptionReconcileCronRunOnStart, cfg.SaaSSubscriptionReconcileLimit, cfg.SaaSPlatformAdminTenantID)
	}

	for key, value := range map[string]string{
		"MOCHAT_GO_SAAS_SUBSCRIPTION_RECONCILE_CRON_INTERVAL_SECONDS": "0",
		"MOCHAT_GO_SAAS_SUBSCRIPTION_RECONCILE_LIMIT":                 "5001",
	} {
		clearEnv(t)
		t.Setenv("MOCHAT_GO_ENABLE_SAAS_SUBSCRIPTION_RECONCILE_CRON", "1")
		t.Setenv("MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID", "9")
		t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
		t.Setenv(key, value)
		if _, err := FromEnv(); err == nil {
			t.Fatalf("expected invalid %s=%s", key, value)
		}
	}
}

func TestSaaSPaymentWebhookAndDunningConfig(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_PAYMENT_WEBHOOK", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_SAAS_PAYMENT_WEBHOOK_SECRET")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_PAYMENT_WEBHOOK", "1")
	t.Setenv("MOCHAT_GO_SAAS_PAYMENT_WEBHOOK_SECRET", "payment-webhook-secret-at-least-32-characters")
	t.Setenv("MOCHAT_GO_SAAS_PAYMENT_WEBHOOK_TOLERANCE_SECONDS", "180")
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_PAYMENT_DUNNING_CRON", "1")
	t.Setenv("MOCHAT_GO_SAAS_PAYMENT_DUNNING_CRON_INTERVAL_SECONDS", "45")
	t.Setenv("MOCHAT_GO_SAAS_PAYMENT_DUNNING_CRON_RUN_ON_START", "1")
	t.Setenv("MOCHAT_GO_SAAS_PAYMENT_DUNNING_LIMIT", "321")
	t.Setenv("MOCHAT_GO_SAAS_PAYMENT_DUNNING_RETRY_DELAY_SECONDS", "7200")
	t.Setenv("MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID", "9")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableSaaSPaymentWebhook || cfg.SaaSPaymentWebhookTolerance != 180*time.Second ||
		!cfg.EnableSaaSPaymentDunningCron || cfg.SaaSPaymentDunningCronInterval != 45*time.Second ||
		!cfg.SaaSPaymentDunningCronRunOnStart || cfg.SaaSPaymentDunningLimit != 321 ||
		cfg.SaaSPaymentDunningRetryDelay != 7200*time.Second || cfg.SaaSPlatformAdminTenantID != 9 {
		t.Fatalf("payment config = %+v", cfg)
	}

	for key, value := range map[string]string{
		"MOCHAT_GO_SAAS_PAYMENT_WEBHOOK_TOLERANCE_SECONDS":     "29",
		"MOCHAT_GO_SAAS_PAYMENT_DUNNING_CRON_INTERVAL_SECONDS": "0",
		"MOCHAT_GO_SAAS_PAYMENT_DUNNING_LIMIT":                 "5001",
		"MOCHAT_GO_SAAS_PAYMENT_DUNNING_RETRY_DELAY_SECONDS":   "59",
	} {
		clearEnv(t)
		t.Setenv("MOCHAT_GO_ENABLE_SAAS_PAYMENT_WEBHOOK", "1")
		t.Setenv("MOCHAT_GO_SAAS_PAYMENT_WEBHOOK_SECRET", "payment-webhook-secret-at-least-32-characters")
		t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
		t.Setenv(key, value)
		if _, err := FromEnv(); err == nil {
			t.Fatalf("expected invalid %s=%s", key, value)
		}
	}
}

func TestSaaSPaymentSettlementSyncConfig(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_PAYMENT_SETTLEMENT_SYNC_CRON", "1")
	_, err := FromEnv()
	requireErrorContains(t, err, "payment settlement bridge URL and providers")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_BRIDGE_BASE_URL", "https://settlement-bridge.example")
	_, err = FromEnv()
	requireErrorContains(t, err, "must be configured together")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_PAYMENT_SETTLEMENT_SYNC_CRON", "1")
	t.Setenv("MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_BRIDGE_BASE_URL", "https://settlement-bridge.example")
	t.Setenv("MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_PROVIDERS", "Gateway, alipay, gateway")
	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	t.Setenv("MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_SYNC_CRON_INTERVAL_SECONDS", "45")
	t.Setenv("MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_SYNC_CRON_RUN_ON_START", "1")
	t.Setenv("MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_BRIDGE_TOKEN", "bridge-token")
	t.Setenv("MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_SYNC_LIMIT", "321")
	t.Setenv("MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_BRIDGE_TIMEOUT_SECONDS", "12")
	t.Setenv("MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID", "9")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableSaaSPaymentSettlementSyncCron || cfg.SaaSPaymentSettlementSyncCronInterval != 45*time.Second ||
		!cfg.SaaSPaymentSettlementSyncCronRunOnStart || cfg.SaaSPaymentSettlementBridgeBaseURL != "https://settlement-bridge.example" ||
		cfg.SaaSPaymentSettlementBridgeToken != "bridge-token" || strings.Join(cfg.SaaSPaymentSettlementProviders, ",") != "gateway,alipay" ||
		cfg.SaaSPaymentSettlementSyncLimit != 321 || cfg.SaaSPaymentSettlementBridgeTimeout != 12*time.Second || cfg.SaaSPlatformAdminTenantID != 9 {
		t.Fatalf("payment settlement sync config = %+v", cfg)
	}

	for key, value := range map[string]string{
		"MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_SYNC_CRON_INTERVAL_SECONDS": "0",
		"MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_SYNC_LIMIT":                 "5001",
		"MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_BRIDGE_TIMEOUT_SECONDS":     "121",
		"MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_PROVIDERS":                  "bad provider",
	} {
		clearEnv(t)
		t.Setenv("MOCHAT_GO_ENABLE_SAAS_PAYMENT_SETTLEMENT_SYNC_CRON", "1")
		t.Setenv("MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_BRIDGE_BASE_URL", "https://settlement-bridge.example")
		t.Setenv("MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_PROVIDERS", "gateway")
		t.Setenv("MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID", "9")
		t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
		t.Setenv(key, value)
		if _, err := FromEnv(); err == nil {
			t.Fatalf("expected invalid %s=%s", key, value)
		}
	}
}

func TestSaaSTenantDomainDeliveryConfig(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_TENANT_DOMAIN_DELIVERY_CRON", "1")
	_, err := FromEnv()
	requireErrorContains(t, err, "bridge URL is required")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_BRIDGE_BASE_URL", "https://domain-bridge.example")
	t.Setenv("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_BRIDGE_TOKEN", "short")
	_, err = FromEnv()
	requireErrorContains(t, err, "BRIDGE_TOKEN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CALLBACK_URL", "https://api.example/webhooks/saas/domain-delivery")
	_, err = FromEnv()
	requireErrorContains(t, err, "must be configured together")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_TENANT_DOMAIN_DELIVERY_CRON", "1")
	t.Setenv("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CRON_INTERVAL_SECONDS", "45")
	t.Setenv("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CRON_RUN_ON_START", "1")
	t.Setenv("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_BRIDGE_BASE_URL", "https://domain-bridge.example")
	t.Setenv("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_BRIDGE_TOKEN", "domain-bridge-token-123456")
	t.Setenv("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_BRIDGE_TIMEOUT_SECONDS", "12")
	t.Setenv("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CALLBACK_URL", "https://api.example/webhooks/saas/domain-delivery")
	t.Setenv("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CALLBACK_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CALLBACK_TOLERANCE_SECONDS", "180")
	t.Setenv("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_LIMIT", "33")
	t.Setenv("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_LEASE_SECONDS", "90")
	t.Setenv("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_RETRY_DELAY_SECONDS", "15")
	t.Setenv("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CALLBACK_WAIT_SECONDS", "240")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableSaaSTenantDomainDeliveryCron || cfg.SaaSTenantDomainDeliveryCronInterval != 45*time.Second || !cfg.SaaSTenantDomainDeliveryCronRunOnStart ||
		cfg.SaaSTenantDomainDeliveryBridgeBaseURL != "https://domain-bridge.example" || cfg.SaaSTenantDomainDeliveryBridgeTimeout != 12*time.Second ||
		cfg.SaaSTenantDomainDeliveryCallbackTolerance != 3*time.Minute || cfg.SaaSTenantDomainDeliveryLimit != 33 ||
		cfg.SaaSTenantDomainDeliveryLease != 90*time.Second || cfg.SaaSTenantDomainDeliveryRetryDelay != 15*time.Second || cfg.SaaSTenantDomainDeliveryCallbackWait != 4*time.Minute {
		t.Fatalf("tenant domain delivery config = %+v", cfg)
	}

	for key, value := range map[string]string{
		"MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CRON_INTERVAL_SECONDS":      "0",
		"MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_BRIDGE_TIMEOUT_SECONDS":     "121",
		"MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CALLBACK_TOLERANCE_SECONDS": "29",
		"MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_LIMIT":                      "101",
		"MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_LEASE_SECONDS":              "9",
		"MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_RETRY_DELAY_SECONDS":        "0",
		"MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CALLBACK_WAIT_SECONDS":      "9",
	} {
		clearEnv(t)
		t.Setenv(key, value)
		if _, err := FromEnv(); err == nil {
			t.Fatalf("expected invalid %s=%s", key, value)
		}
	}
}

func TestChannelCodeGroupReadRoutesAllowDevHeader(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_CHANNEL_CODE_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_CHANNEL_CODE_SHOW", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_CHANNEL_CODE_CONTACT", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_CHANNEL_CODE_STATISTICS", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_CHANNEL_CODE_STATISTICS_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_CHANNEL_CODE_GROUP_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_CHANNEL_CODE_GROUP_DETAIL", "1")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MigrateChannelCodeIndex || !cfg.MigrateChannelCodeShow || !cfg.MigrateChannelCodeContact || !cfg.MigrateChannelCodeStatistics || !cfg.MigrateChannelCodeStatsIndex ||
		!cfg.MigrateChannelCodeGroupIndex || !cfg.MigrateChannelCodeGroupDetail || !cfg.DevAuthHeader {
		t.Fatalf("unexpected flags: %+v", cfg)
	}
}

func TestChannelCodeGroupWriteRoutesRequireRealJWT(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_CHANNEL_CODE_STORE", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_CHANNEL_CODE_GROUP_STORE", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "state-changing migrated routes require PHP JWT auth")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_CHANNEL_CODE_STORE", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_CHANNEL_CODE_UPDATE", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_CHANNEL_CODE_GROUP_STORE", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_CHANNEL_CODE_GROUP_UPDATE", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_CHANNEL_CODE_GROUP_MOVE", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MigrateChannelCodeStore || !cfg.MigrateChannelCodeUpdate || !cfg.MigrateChannelCodeGroupStore || !cfg.MigrateChannelCodeGroupUpdate || !cfg.MigrateChannelCodeGroupMove {
		t.Fatalf("unexpected flags: %+v", cfg)
	}
}

func TestCommonUploadRoutesRequireRealJWT(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_COMMON_UPLOAD", "1")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "state-changing migrated routes require PHP JWT auth")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_COMMON_UPLOAD", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_COMMON_UPLOAD_FILE", "1")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_GO_SKIP_JWT_BLACKLIST", "1")

	_, err = FromEnv()
	requireErrorContains(t, err, "state-changing migrated routes cannot skip JWT blacklist checks")
}

func TestSidebarAgentRoutesRequireExpectedAuthConfig(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_SIDEBAR_AGENT_OAUTH", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	_, err := FromEnv()
	requireErrorContains(t, err, "SIMPLE_JWT_SECRET")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_SIDEBAR_AGENT_AUTH", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_SIDEBAR_AGENT_OAUTH", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_SIDEBAR_AGENT_JSSDK_CONFIG", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_SIDEBAR_WX_JS_SDK_CONFIG", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_SIDEBAR_JWT_SECRET", "sidebar-secret")
	t.Setenv("MOCHAT_REDIS_ADDR", "127.0.0.1:6379")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MigrateSidebarAgentAuth || !cfg.MigrateSidebarAgentOAuth || !cfg.MigrateSidebarAgentJSSDK || !cfg.MigrateSidebarWxJSSDK {
		t.Fatalf("unexpected flags: %+v", cfg)
	}
}

func TestAgentStoreRequiresRealJWT(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_AGENT_STORE", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "state-changing migrated routes require PHP JWT auth")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_AGENT_STORE", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_REDIS_ADDR", "127.0.0.1:6379")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MigrateAgentStore {
		t.Fatalf("MigrateAgentStore = false")
	}
}

func TestSidebarCommonUploadRequiresSidebarJWT(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_SIDEBAR_COMMON_UPLOAD", "1")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "state-changing migrated routes require PHP JWT auth")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_SIDEBAR_COMMON_UPLOAD", "1")
	t.Setenv("MOCHAT_SIDEBAR_JWT_SECRET", "")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MigrateSidebarCommonUpload || cfg.SidebarJWTSecret != "Br3LXhp&Ysha1zRDh" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestSidebarMediumMediaIDUpdateRequiresRealSidebarJWT(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_SIDEBAR_MEDIUM_MEDIA_ID_UPDATE", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "state-changing migrated routes require PHP JWT auth")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_SIDEBAR_MEDIUM_MEDIA_ID_UPDATE", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_GO_SKIP_JWT_BLACKLIST", "1")

	_, err = FromEnv()
	requireErrorContains(t, err, "state-changing migrated routes cannot skip JWT blacklist checks")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_SIDEBAR_MEDIUM_MEDIA_ID_UPDATE", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIDEBAR_JWT_SECRET", "sidebar-secret")
	t.Setenv("MOCHAT_REDIS_ADDR", "127.0.0.1:6379")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MigrateSidebarMediumMediaIDUpdate {
		t.Fatalf("unexpected flags: %+v", cfg)
	}
}

func TestStatisticRoutesAllowDevHeader(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_STATISTIC_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_STATISTIC_TOP_LIST", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_STATISTIC_EMPLOYEE_COUNTS", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_STATISTIC_EMPLOYEES", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_STATISTIC_EMPLOYEES_TREND", "1")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MigrateStatisticIndex || !cfg.MigrateStatisticTopList || !cfg.MigrateStatisticEmployeeCounts || !cfg.MigrateStatisticEmployees || !cfg.MigrateStatisticEmployeesTrend || !cfg.DevAuthHeader {
		t.Fatalf("unexpected flags: %+v", cfg)
	}
}

func TestWorkReadRoutesAllowDevHeader(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_EMPLOYEE_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_EMPLOYEE_SEARCH_CONDITION", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_DEPARTMENT_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_EMPLOYEE_DEPARTMENT_MEMBER_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_DEPARTMENT_SELECT_BY_PHONE", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_DEPARTMENT_PAGE_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_DEPARTMENT_SHOW_EMPLOYEE", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_GROUP_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_GROUP_DETAIL", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_TAG_GROUP_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_DETAIL", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_LIST", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_ALL", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_CONTACT_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_CONTACT_LOSS", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_CONTACT_SOURCE", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_CONTACT_SHOW", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_CONTACT_TRACK", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_CONTACT_ROOM_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_ROOM_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_ROOM_ROOM_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_ROOM_STATISTICS", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_ROOM_STATISTICS_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_ROOM_AUTO_PULL_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_ROOM_AUTO_PULL_SHOW", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_SIDEBAR_WORK_ROOM_MANAGE", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_SHOW", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_SHOW_CONTACT", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_ROOM_LIST", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_CHOOSE_CONTACT", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_SHOW", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_SHOW_ROOM", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_EMPLOYEE_SEND_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_CONTACT_RECEIVE_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_SHOW", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_ROOM_OWNER_SEND_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_ROOM_RECEIVE_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_FISSION_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_FISSION_SHOW", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_FISSION_INFO", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_FISSION_STATISTICS", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_FISSION_CHOOSE_CONTACT", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_FISSION_INVITE_DATA", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_FISSION_INVITE_DETAIL", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_TAG_ALL", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_DETAIL", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_SHOW", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_TRACK", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_SIDEBAR_CONTACT_PROCESS_STATUS_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_SIDEBAR_CONTACT_BATCH_ADD_DETAIL", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_CONTACT_FIELD_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_CONTACT_FIELD_SHOW", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_CONTACT_FIELD_PORTRAIT", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_CONTACT_FIELD_PIVOT_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_SIDEBAR_CONTACT_FIELD_PIVOT_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_SIDEBAR_CONTACT_SOP_GET_SOP_INFO", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_SIDEBAR_CONTACT_SOP_GET_SOP_TIP_INFO", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_SIDEBAR_ROOM_SOP_GET_SOP_INFO", "1")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MigrateWorkEmployeeIndex || !cfg.MigrateWorkEmployeeCond || !cfg.MigrateWorkDeptIndex || !cfg.MigrateWorkDeptMember || !cfg.MigrateWorkDeptPhone || !cfg.MigrateWorkDeptPage || !cfg.MigrateWorkDeptEmployee || !cfg.MigrateWorkTagGroupIndex || !cfg.MigrateWorkTagGroupDetail || !cfg.MigrateSidebarTagGroupIndex || !cfg.MigrateWorkContactTagIndex || !cfg.MigrateWorkContactTagDetail || !cfg.MigrateWorkContactTagList || !cfg.MigrateWorkContactTagAll || !cfg.MigrateWorkContactIndex || !cfg.MigrateWorkContactLoss || !cfg.MigrateWorkContactSource || !cfg.MigrateWorkContactShow || !cfg.MigrateWorkContactTrack || !cfg.MigrateWorkContactRoomIndex || !cfg.MigrateWorkRoomIndex || !cfg.MigrateWorkRoomRoomIndex || !cfg.MigrateWorkRoomStatistics || !cfg.MigrateWorkRoomStatisticsIndex || !cfg.MigrateSidebarWorkRoomManage || !cfg.MigrateWorkRoomAutoPullIndex || !cfg.MigrateWorkRoomAutoPullShow || !cfg.MigrateRoomTagPullIndex || !cfg.MigrateRoomTagPullShow || !cfg.MigrateRoomTagPullShowContact || !cfg.MigrateRoomTagPullRoomList || !cfg.MigrateRoomTagPullChooseContact || !cfg.MigrateContactMessageBatchSendIndex || !cfg.MigrateContactMessageBatchSendShow || !cfg.MigrateContactMessageBatchSendShowRoom || !cfg.MigrateContactMessageBatchSendEmployee || !cfg.MigrateContactMessageBatchSendContactReceive || !cfg.MigrateRoomMessageBatchSendIndex || !cfg.MigrateRoomMessageBatchSendShow || !cfg.MigrateRoomMessageBatchSendOwner || !cfg.MigrateRoomMessageBatchSendRoomReceive || !cfg.MigrateOfficialAccountIndex || !cfg.MigrateWorkFissionIndex || !cfg.MigrateWorkFissionShow || !cfg.MigrateWorkFissionInfo || !cfg.MigrateWorkFissionStatistics || !cfg.MigrateWorkFissionChooseContact || !cfg.MigrateWorkFissionInviteData || !cfg.MigrateWorkFissionInviteDetail || !cfg.MigrateSidebarContactTagAll || !cfg.MigrateSidebarContactDetail || !cfg.MigrateSidebarContactShow || !cfg.MigrateSidebarContactTrack || !cfg.MigrateSidebarProcessStatus || !cfg.MigrateSidebarContactBatchAddDetail || !cfg.MigrateContactFieldIndex || !cfg.MigrateContactFieldShow || !cfg.MigrateContactFieldPortrait || !cfg.MigrateContactFieldPivot || !cfg.MigrateSidebarFieldPivot || !cfg.MigrateSidebarContactSOPInfo || !cfg.MigrateSidebarContactSOPTipInfo || !cfg.MigrateSidebarRoomSOPInfo || !cfg.DevAuthHeader {
		t.Fatalf("unexpected flags: %+v", cfg)
	}
}

func TestContactTransferReadRoutesAllowDevHeader(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_CONTACT_TRANSFER_INFO", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_CONTACT_TRANSFER_UNASSIGNED_LIST", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_CONTACT_TRANSFER_ROOM", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_CONTACT_TRANSFER_LOG", "1")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MigrateContactTransferInfo || !cfg.MigrateContactTransferUnassigned || !cfg.MigrateContactTransferRoom || !cfg.MigrateContactTransferLog || !cfg.DevAuthHeader {
		t.Fatalf("unexpected flags: %+v", cfg)
	}
}

func TestContactTransferWriteRoutesRequireRealJWT(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_CONTACT_TRANSFER_INDEX", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "state-changing migrated routes require PHP JWT auth")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_CONTACT_TRANSFER_SAVE_UNASSIGNED_LIST", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_CONTACT_TRANSFER_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_CONTACT_TRANSFER_ROOM_STORE", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_REDIS_ADDR", "127.0.0.1:6379")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MigrateContactTransferSync || !cfg.MigrateContactTransferCustomer || !cfg.MigrateContactTransferRoomStore {
		t.Fatalf("unexpected flags: %+v", cfg)
	}
}

func TestSidebarProcessStatusUpdateRequiresRealSidebarJWT(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_SIDEBAR_CONTACT_PROCESS_STATUS_UPDATE", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "state-changing migrated routes require PHP JWT auth")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_SIDEBAR_CONTACT_PROCESS_STATUS_UPDATE", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_GO_SKIP_JWT_BLACKLIST", "1")

	_, err = FromEnv()
	requireErrorContains(t, err, "state-changing migrated routes cannot skip JWT blacklist checks")
}

func TestContactFieldPivotUpdateRequiresRealJWT(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_CONTACT_FIELD_PIVOT_UPDATE", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "state-changing migrated routes require PHP JWT auth")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_CONTACT_FIELD_PIVOT_UPDATE", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_REDIS_ADDR", "127.0.0.1:6379")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MigrateContactFieldPivotUpdate {
		t.Fatalf("unexpected flags: %+v", cfg)
	}

	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_SIDEBAR_CONTACT_FIELD_PIVOT_UPDATE", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIDEBAR_JWT_SECRET", "sidebar-secret")
	t.Setenv("MOCHAT_REDIS_ADDR", "127.0.0.1:6379")

	cfg, err = FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MigrateSidebarFieldPivotUpdate {
		t.Fatalf("unexpected flags: %+v", cfg)
	}
}

func TestWorkContactTagWriteFlagsRequireStateChangingAuth(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_STORE", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_SYNC", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "state-changing migrated routes require PHP JWT auth")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_GROUP_STORE", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_GROUP_UPDATE", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_GROUP_DESTROY", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_STORE", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_UPDATE", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_DESTROY", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_MOVE", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_SYNC", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MigrateWorkTagGroupStore || !cfg.MigrateWorkTagGroupUpdate || !cfg.MigrateWorkTagGroupDestroy || !cfg.MigrateWorkContactTagStore || !cfg.MigrateWorkContactTagUpdate || !cfg.MigrateWorkContactTagDestroy || !cfg.MigrateWorkContactTagMove || !cfg.MigrateWorkContactTagSync {
		t.Fatalf("unexpected flags: %+v", cfg)
	}
}

func TestWorkContactUpdateRequiresRealJWT(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_CONTACT_UPDATE", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "state-changing migrated routes require PHP JWT auth")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_CONTACT_UPDATE", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_REDIS_ADDR", "127.0.0.1:6379")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MigrateWorkContactUpdate {
		t.Fatalf("MigrateWorkContactUpdate = false")
	}
}

func TestWorkContactSyncRequiresRealJWT(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_CONTACT_SYNC", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "state-changing migrated routes require PHP JWT auth")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_CONTACT_SYNC", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_REDIS_ADDR", "127.0.0.1:6379")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MigrateWorkContactSync {
		t.Fatalf("MigrateWorkContactSync = false")
	}
}

func TestWorkContactBatchLabelingRequiresRealJWT(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_CONTACT_BATCH_LABELING", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "state-changing migrated routes require PHP JWT auth")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_CONTACT_BATCH_LABELING", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MigrateWorkContactBatchLabeling {
		t.Fatalf("MigrateWorkContactBatchLabeling = false")
	}
}

func TestWorkRoomBatchUpdateRequiresRealJWT(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_ROOM_BATCH_UPDATE", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "state-changing migrated routes require PHP JWT auth")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_ROOM_BATCH_UPDATE", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MigrateWorkRoomBatchUpdate {
		t.Fatalf("MigrateWorkRoomBatchUpdate = false")
	}
}

func TestWorkRoomSyncRequiresRealJWT(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_ROOM_SYNC", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "state-changing migrated routes require PHP JWT auth")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_ROOM_SYNC", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_REDIS_ADDR", "127.0.0.1:6379")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MigrateWorkRoomSync {
		t.Fatalf("MigrateWorkRoomSync = false")
	}
}

func TestSidebarWorkContactUpdateRequiresRealJWT(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_UPDATE", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIDEBAR_JWT_SECRET", "sidebar-secret")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "state-changing migrated routes require PHP JWT auth")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_UPDATE", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIDEBAR_JWT_SECRET", "sidebar-secret")
	t.Setenv("MOCHAT_REDIS_ADDR", "127.0.0.1:6379")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MigrateSidebarContactUpdate {
		t.Fatalf("MigrateSidebarContactUpdate = false")
	}
}

func TestWorkRoomAutoPullWriteFlagsRequireStateChangingAuth(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_ROOM_AUTO_PULL_STORE", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "state-changing migrated routes require PHP JWT auth")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_ROOM_AUTO_PULL_STORE", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_ROOM_AUTO_PULL_UPDATE", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_REDIS_ADDR", "127.0.0.1:6379")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MigrateWorkRoomAutoPullStore || !cfg.MigrateWorkRoomAutoPullUpdate {
		t.Fatalf("unexpected flags: %+v", cfg)
	}
}

func TestRoomTagPullWriteFlagsRequireStateChangingAuth(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_STORE", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "state-changing migrated routes require PHP JWT auth")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_STORE", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_FILTER_CONTACT", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_REMIND_SEND", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_DESTROY", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_REDIS_ADDR", "127.0.0.1:6379")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MigrateRoomTagPullStore || !cfg.MigrateRoomTagPullFilterContact || !cfg.MigrateRoomTagPullRemindSend || !cfg.MigrateRoomTagPullDestroy {
		t.Fatalf("unexpected flags: %+v", cfg)
	}
}

func TestContactMessageBatchSendWriteFlagsRequireStateChangingAuth(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_STORE", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "state-changing migrated routes require PHP JWT auth")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_STORE", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_REMIND", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_DESTROY", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_REDIS_ADDR", "127.0.0.1:6379")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MigrateContactMessageBatchSendStore || !cfg.MigrateContactMessageBatchSendRemind || !cfg.MigrateContactMessageBatchSendDestroy {
		t.Fatalf("unexpected flags: %+v", cfg)
	}
}

func TestRoomMessageBatchSendWriteFlagsRequireStateChangingAuth(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_STORE", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "state-changing migrated routes require PHP JWT auth")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_STORE", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_REMIND", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_DESTROY", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_REDIS_ADDR", "127.0.0.1:6379")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MigrateRoomMessageBatchSendStore || !cfg.MigrateRoomMessageBatchSendRemind || !cfg.MigrateRoomMessageBatchSendDestroy {
		t.Fatalf("unexpected flags: %+v", cfg)
	}
}

func TestOfficialAccountSetRequiresStateChangingAuth(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_SET", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "state-changing migrated routes require PHP JWT auth")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_SET", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_REDIS_ADDR", "127.0.0.1:6379")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MigrateOfficialAccountSet {
		t.Fatalf("unexpected flags: %+v", cfg)
	}
}

func TestWorkFissionDestroyRequiresStateChangingAuth(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_FISSION_STORE", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_FISSION_UPDATE", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_FISSION_DESTROY", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "state-changing migrated routes require PHP JWT auth")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_FISSION_STORE", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_FISSION_UPDATE", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_FISSION_DESTROY", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_REDIS_ADDR", "127.0.0.1:6379")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MigrateWorkFissionStore || !cfg.MigrateWorkFissionUpdate || !cfg.MigrateWorkFissionDestroy {
		t.Fatalf("unexpected flags: %+v", cfg)
	}
}

func TestRoleSelectAllowsDevHeader(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_ROLE_SELECT", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_ROLE_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_ROLE_SHOW", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_ROLE_PERMISSION_SHOW", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_ROLE_SHOW_EMPLOYEE", "1")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MigrateRoleSelect || !cfg.MigrateRoleIndex || !cfg.MigrateRoleShow || !cfg.MigrateRolePermission || !cfg.MigrateRoleShowEmployee || !cfg.DevAuthHeader {
		t.Fatalf("unexpected flags: %+v", cfg)
	}
}

func TestMenuReadRoutesAllowDevHeader(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_MENU_ICON_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_MENU_SELECT", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_MENU_INDEX", "1")
	t.Setenv("MOCHAT_GO_MIGRATE_MENU_SHOW", "1")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MigrateMenuIconIndex || !cfg.MigrateMenuSelect || !cfg.MigrateMenuIndex || !cfg.MigrateMenuShow || !cfg.DevAuthHeader {
		t.Fatalf("unexpected flags: %+v", cfg)
	}
}

func TestAgentTxtVerifyDoesNotRequireBackends(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_AGENT_TXT_VERIFY", "1")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MigrateAgentTxtVerify {
		t.Fatalf("MigrateAgentTxtVerify = false")
	}
}

func TestAgentTxtVerifyUploadDoesNotRequireBackends(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_AGENT_TXT_VERIFY_UPLOAD", "1")
	t.Setenv("MOCHAT_FILE_STORAGE_ROOT", "/tmp/mochat-upload")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MigrateAgentTxtUpload || cfg.FileStorageRoot != "/tmp/mochat-upload" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestCorpUpdateRequiresRealJWT(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_CORP_UPDATE", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "state-changing migrated routes require PHP JWT auth")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_CORP_UPDATE", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_GO_SKIP_JWT_BLACKLIST", "1")

	_, err = FromEnv()
	requireErrorContains(t, err, "state-changing migrated routes cannot skip JWT blacklist checks")
}

func TestWorkEmployeeSyncRequiresRealJWT(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_EMPLOYEE_SYNC", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "state-changing migrated routes require PHP JWT auth")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_EMPLOYEE_SYNC", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_GO_SKIP_JWT_BLACKLIST", "1")

	_, err = FromEnv()
	requireErrorContains(t, err, "state-changing migrated routes cannot skip JWT blacklist checks")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_WORK_EMPLOYEE_SYNC", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_REDIS_ADDR", "127.0.0.1:6379")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MigrateWorkEmployeeSync {
		t.Fatalf("MigrateWorkEmployeeSync = false")
	}
}

func TestCorpStoreRequiresRealJWT(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_CORP_STORE", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "state-changing migrated routes require PHP JWT auth")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_CORP_STORE", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_REDIS_ADDR", "127.0.0.1:6379")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MigrateCorpStore {
		t.Fatalf("MigrateCorpStore = false")
	}
}

func TestLogoutRequiresRealJWTAndBlacklist(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_LOGOUT", "1")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_GO_DEV_AUTH_HEADER", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "require PHP JWT auth")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_LOGOUT", "1")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_GO_SKIP_JWT_BLACKLIST", "1")

	_, err = FromEnv()
	requireErrorContains(t, err, "cannot skip JWT blacklist")
}

func TestJWTAndRedisOverrides(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_MIGRATE_PERMISSION_BY_USER", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "secret")
	t.Setenv("MOCHAT_SIMPLE_JWT_PREFIX", "tenant-a")
	t.Setenv("MOCHAT_SIMPLE_JWT_TTL", "30")
	t.Setenv("MOCHAT_SIMPLE_JWT_REFRESH_TTL", "60")
	t.Setenv("MOCHAT_REDIS_ADDR", "127.0.0.1:6380")
	t.Setenv("MOCHAT_REDIS_PASSWORD", "redis-pass")
	t.Setenv("MOCHAT_REDIS_DB", "2")
	t.Setenv("MOCHAT_GO_WORKER_PROCESSING_TIMEOUT_SECONDS", "45")
	t.Setenv("MOCHAT_DASHBOARD_DIST", "./tmp-dashboard-dist")
	t.Setenv("MOCHAT_LEGACY_DASHBOARD_DIST", "./tmp-legacy-dashboard-dist")
	t.Setenv("MOCHAT_SAAS_ADMIN_DIST", "./tmp-saas-admin-dist")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SimpleJWTSecret != "secret" || cfg.SimpleJWTPrefix != "tenant-a" {
		t.Fatalf("jwt config = secret %q prefix %q", cfg.SimpleJWTSecret, cfg.SimpleJWTPrefix)
	}
	t.Setenv("MOCHAT_SIDEBAR_JWT_SECRET", "sidebar-secret")
	t.Setenv("MOCHAT_SIDEBAR_JWT_PREFIX", "sidebar-prefix")
	cfg, err = FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SidebarJWTSecret != "sidebar-secret" || cfg.SidebarJWTPrefix != "sidebar-prefix" {
		t.Fatalf("sidebar jwt config = secret %q prefix %q", cfg.SidebarJWTSecret, cfg.SidebarJWTPrefix)
	}
	if cfg.SimpleJWTTTL != 30*time.Second || cfg.SimpleJWTRefreshTTL != time.Minute {
		t.Fatalf("ttl = %s refresh ttl = %s", cfg.SimpleJWTTTL, cfg.SimpleJWTRefreshTTL)
	}
	if cfg.RedisAddr != "127.0.0.1:6380" || cfg.RedisPassword != "redis-pass" || cfg.RedisDB != 2 {
		t.Fatalf("redis config = %+v", cfg)
	}
	if cfg.WorkerProcessingTimeout != 45*time.Second {
		t.Fatalf("worker processing timeout = %s", cfg.WorkerProcessingTimeout)
	}
	if cfg.DashboardDist != "./tmp-dashboard-dist" {
		t.Fatalf("dashboard dist = %q", cfg.DashboardDist)
	}
	if cfg.SaaSAdminDist != "./tmp-saas-admin-dist" {
		t.Fatalf("saas admin dist = %q", cfg.SaaSAdminDist)
	}
}

func TestCorpDataCronRequiresOnlyMySQL(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_CORP_DATA_CRON", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_CORP_DATA_CRON", "1")
	t.Setenv("MOCHAT_GO_CORP_DATA_CRON_INTERVAL_SECONDS", "30")
	t.Setenv("MOCHAT_GO_CORP_DATA_CRON_RUN_ON_START", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableCorpDataCron || cfg.CorpDataCronInterval != 30*time.Second || !cfg.CorpDataCronRunOnStart {
		t.Fatalf("corpData cron config = enabled %v interval %s runOnStart %v", cfg.EnableCorpDataCron, cfg.CorpDataCronInterval, cfg.CorpDataCronRunOnStart)
	}
}

func TestAsyncFileUploadWorkerDoesNotRequireMySQLOrJWT(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_ASYNC_FILE_UPLOAD_WORKER", "1")
	t.Setenv("MOCHAT_FILE_STORAGE_ROOT", t.TempDir())

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableAsyncFileUploadWorker {
		t.Fatalf("EnableAsyncFileUploadWorker = false")
	}
	if cfg.MySQLDSN != "" || cfg.SimpleJWTSecret != "" {
		t.Fatalf("unexpected mysql/jwt requirement: mysql=%q jwt=%q", cfg.MySQLDSN, cfg.SimpleJWTSecret)
	}
}

func TestMarkTagsWorkerRequiresMySQLButNotJWT(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_MARK_TAGS_WORKER", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_MARK_TAGS_WORKER", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableMarkTagsWorker {
		t.Fatalf("EnableMarkTagsWorker = false")
	}
	if cfg.SimpleJWTSecret != "" {
		t.Fatalf("unexpected jwt requirement: %q", cfg.SimpleJWTSecret)
	}
}

func TestMessageRemindWorkerRequiresMySQLButNotJWT(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_MESSAGE_REMIND_WORKER", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_MESSAGE_REMIND_WORKER", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableMessageRemindWorker {
		t.Fatalf("EnableMessageRemindWorker = false")
	}
	if cfg.SimpleJWTSecret != "" {
		t.Fatalf("unexpected jwt requirement: %q", cfg.SimpleJWTSecret)
	}
}

func TestWorkRoomSyncWorkerRequiresMySQLButNotJWT(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_WORK_ROOM_SYNC_WORKER", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_WORK_ROOM_SYNC_WORKER", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableWorkRoomSyncWorker {
		t.Fatalf("EnableWorkRoomSyncWorker = false")
	}
	if cfg.SimpleJWTSecret != "" {
		t.Fatalf("unexpected jwt requirement: %q", cfg.SimpleJWTSecret)
	}
}

func TestWorkContactSyncWorkerRequiresMySQLButNotJWT(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_WORK_CONTACT_SYNC_WORKER", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_WORK_CONTACT_SYNC_WORKER", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableWorkContactSyncWorker {
		t.Fatalf("EnableWorkContactSyncWorker = false")
	}
	if cfg.SimpleJWTSecret != "" {
		t.Fatalf("unexpected jwt requirement: %q", cfg.SimpleJWTSecret)
	}
}

func TestWorkDepartmentListWorkerRequiresMySQLAndJWT(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_WORK_DEPARTMENT_LIST_WORKER", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_WORK_DEPARTMENT_LIST_WORKER", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_SIMPLE_JWT_SECRET")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_WORK_DEPARTMENT_LIST_WORKER", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")
	t.Setenv("MOCHAT_SIMPLE_JWT_SECRET", "worker-secret")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableWorkDepartmentListWorker {
		t.Fatalf("EnableWorkDepartmentListWorker = false")
	}
}

func TestMediaIDUpdateWorkerRequiresMySQLButNotJWT(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_MEDIA_ID_UPDATE_WORKER", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_MEDIA_ID_UPDATE_WORKER", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableMediaIDUpdateWorker {
		t.Fatalf("EnableMediaIDUpdateWorker = false")
	}
	if cfg.SimpleJWTSecret != "" {
		t.Fatalf("unexpected jwt requirement: %q", cfg.SimpleJWTSecret)
	}
}

func TestEmployeeStatisticWorkerRequiresMySQLAndRedisButNotJWT(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_EMPLOYEE_STATISTIC_WORKER", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_EMPLOYEE_STATISTIC_WORKER", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableEmployeeStatisticWorker {
		t.Fatalf("EnableEmployeeStatisticWorker = false")
	}
	if cfg.RedisAddr == "" {
		t.Fatalf("RedisAddr should use default when worker is enabled")
	}
	if cfg.SimpleJWTSecret != "" {
		t.Fatalf("unexpected jwt requirement: %q", cfg.SimpleJWTSecret)
	}
}

func TestMediaIDUpdateCronRequiresOnlyMySQL(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_MEDIA_ID_UPDATE_CRON", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_MEDIA_ID_UPDATE_CRON", "1")
	t.Setenv("MOCHAT_GO_MEDIA_ID_UPDATE_CRON_INTERVAL_SECONDS", "45")
	t.Setenv("MOCHAT_GO_MEDIA_ID_UPDATE_CRON_RUN_ON_START", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableMediaIDUpdateCron || cfg.MediaIDUpdateCronInterval != 45*time.Second || !cfg.MediaIDUpdateCronRunOnStart {
		t.Fatalf("mediaIdUpdate cron config = enabled %v interval %s runOnStart %v", cfg.EnableMediaIDUpdateCron, cfg.MediaIDUpdateCronInterval, cfg.MediaIDUpdateCronRunOnStart)
	}
}

func TestPullAgentCronRequiresMySQLAndReadsInterval(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_PULL_AGENT_CRON", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_PULL_AGENT_CRON", "1")
	t.Setenv("MOCHAT_GO_PULL_AGENT_CRON_INTERVAL_SECONDS", "45")
	t.Setenv("MOCHAT_GO_PULL_AGENT_CRON_RUN_ON_START", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnablePullAgentCron || cfg.PullAgentCronInterval != 45*time.Second || !cfg.PullAgentCronRunOnStart {
		t.Fatalf("pullAgent cron config = enabled %v interval %s runOnStart %v", cfg.EnablePullAgentCron, cfg.PullAgentCronInterval, cfg.PullAgentCronRunOnStart)
	}
}

func TestEmployeeStatisticCronRequiresMySQLAndReadsInterval(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_EMPLOYEE_STATISTIC_CRON", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_EMPLOYEE_STATISTIC_CRON", "1")
	t.Setenv("MOCHAT_GO_EMPLOYEE_STATISTIC_CRON_INTERVAL_SECONDS", "45")
	t.Setenv("MOCHAT_GO_EMPLOYEE_STATISTIC_CRON_RUN_ON_START", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableEmployeeStatisticCron || cfg.EmployeeStatisticCronInterval != 45*time.Second || !cfg.EmployeeStatisticCronRunOnStart {
		t.Fatalf("employeeStatistic cron config = enabled %v interval %s runOnStart %v", cfg.EnableEmployeeStatisticCron, cfg.EmployeeStatisticCronInterval, cfg.EmployeeStatisticCronRunOnStart)
	}
}

func TestChannelCodeCronRequiresMySQLAndReadsInterval(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_CHANNEL_CODE_CRON", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_CHANNEL_CODE_CRON", "1")
	t.Setenv("MOCHAT_GO_CHANNEL_CODE_CRON_INTERVAL_SECONDS", "45")
	t.Setenv("MOCHAT_GO_CHANNEL_CODE_CRON_RUN_ON_START", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableChannelCodeCron || cfg.ChannelCodeCronInterval != 45*time.Second || !cfg.ChannelCodeCronRunOnStart {
		t.Fatalf("channelCode cron config = enabled %v interval %s runOnStart %v", cfg.EnableChannelCodeCron, cfg.ChannelCodeCronInterval, cfg.ChannelCodeCronRunOnStart)
	}
}

func TestContactSyncSendResultCronRequiresMySQLAndReadsInterval(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_CONTACT_SYNC_SEND_RESULT_CRON", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_CONTACT_SYNC_SEND_RESULT_CRON", "1")
	t.Setenv("MOCHAT_GO_CONTACT_SYNC_SEND_RESULT_CRON_INTERVAL_SECONDS", "45")
	t.Setenv("MOCHAT_GO_CONTACT_SYNC_SEND_RESULT_CRON_RUN_ON_START", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableContactSyncSendResultCron || cfg.ContactSyncSendResultCronInterval != 45*time.Second || !cfg.ContactSyncSendResultCronRunOnStart {
		t.Fatalf("ContactSyncSendResultTask cron config = enabled %v interval %s runOnStart %v", cfg.EnableContactSyncSendResultCron, cfg.ContactSyncSendResultCronInterval, cfg.ContactSyncSendResultCronRunOnStart)
	}
}

func TestContactBatchSendCronRequiresMySQLAndReadsInterval(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_CONTACT_BATCH_SEND_CRON", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_CONTACT_BATCH_SEND_CRON", "1")
	t.Setenv("MOCHAT_GO_CONTACT_BATCH_SEND_CRON_INTERVAL_SECONDS", "45")
	t.Setenv("MOCHAT_GO_CONTACT_BATCH_SEND_CRON_RUN_ON_START", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableContactBatchSendCron || cfg.ContactBatchSendCronInterval != 45*time.Second || !cfg.ContactBatchSendCronRunOnStart {
		t.Fatalf("ContactMessageBatchSend cron config = enabled %v interval %s runOnStart %v", cfg.EnableContactBatchSendCron, cfg.ContactBatchSendCronInterval, cfg.ContactBatchSendCronRunOnStart)
	}
}

func TestRoomBatchSendCronRequiresMySQLAndReadsInterval(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_ROOM_BATCH_SEND_CRON", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_ROOM_BATCH_SEND_CRON", "1")
	t.Setenv("MOCHAT_GO_ROOM_BATCH_SEND_CRON_INTERVAL_SECONDS", "45")
	t.Setenv("MOCHAT_GO_ROOM_BATCH_SEND_CRON_RUN_ON_START", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableRoomBatchSendCron || cfg.RoomBatchSendCronInterval != 45*time.Second || !cfg.RoomBatchSendCronRunOnStart {
		t.Fatalf("RoomMessageBatchSend cron config = enabled %v interval %s runOnStart %v", cfg.EnableRoomBatchSendCron, cfg.RoomBatchSendCronInterval, cfg.RoomBatchSendCronRunOnStart)
	}
}

func TestRoomSyncSendResultCronRequiresMySQLAndReadsInterval(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_ROOM_SYNC_SEND_RESULT_CRON", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_ROOM_SYNC_SEND_RESULT_CRON", "1")
	t.Setenv("MOCHAT_GO_ROOM_SYNC_SEND_RESULT_CRON_INTERVAL_SECONDS", "45")
	t.Setenv("MOCHAT_GO_ROOM_SYNC_SEND_RESULT_CRON_RUN_ON_START", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableRoomSyncSendResultCron || cfg.RoomSyncSendResultCronInterval != 45*time.Second || !cfg.RoomSyncSendResultCronRunOnStart {
		t.Fatalf("RoomSyncSendResultTask cron config = enabled %v interval %s runOnStart %v", cfg.EnableRoomSyncSendResultCron, cfg.RoomSyncSendResultCronInterval, cfg.RoomSyncSendResultCronRunOnStart)
	}
}

func TestRoomTagPullCronRequiresMySQLAndReadsInterval(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_ROOM_TAG_PULL_CRON", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_ROOM_TAG_PULL_CRON", "1")
	t.Setenv("MOCHAT_GO_ROOM_TAG_PULL_CRON_INTERVAL_SECONDS", "45")
	t.Setenv("MOCHAT_GO_ROOM_TAG_PULL_CRON_RUN_ON_START", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableRoomTagPullCron || cfg.RoomTagPullCronInterval != 45*time.Second || !cfg.RoomTagPullCronRunOnStart {
		t.Fatalf("RoomTagPull cron config = enabled %v interval %s runOnStart %v", cfg.EnableRoomTagPullCron, cfg.RoomTagPullCronInterval, cfg.RoomTagPullCronRunOnStart)
	}
}

func TestTransferStateRefreshCronRequiresMySQLAndReadsInterval(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_TRANSFER_STATE_REFRESH_CRON", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_TRANSFER_STATE_REFRESH_CRON", "1")
	t.Setenv("MOCHAT_GO_TRANSFER_STATE_REFRESH_CRON_INTERVAL_SECONDS", "45")
	t.Setenv("MOCHAT_GO_TRANSFER_STATE_REFRESH_CRON_RUN_ON_START", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableTransferStateRefreshCron || cfg.TransferStateRefreshCronInterval != 45*time.Second || !cfg.TransferStateRefreshCronRunOnStart {
		t.Fatalf("TransferStateRefresh cron config = enabled %v interval %s runOnStart %v", cfg.EnableTransferStateRefreshCron, cfg.TransferStateRefreshCronInterval, cfg.TransferStateRefreshCronRunOnStart)
	}
}

func TestSOPLogCronRequiresMySQLAndReadsInterval(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SOP_LOG_CRON", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SOP_LOG_CRON", "1")
	t.Setenv("MOCHAT_GO_SOP_LOG_CRON_INTERVAL_SECONDS", "45")
	t.Setenv("MOCHAT_GO_SOP_LOG_CRON_RUN_ON_START", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableSOPLogCron || cfg.SOPLogCronInterval != 45*time.Second || !cfg.SOPLogCronRunOnStart {
		t.Fatalf("SOP log cron config = enabled %v interval %s runOnStart %v", cfg.EnableSOPLogCron, cfg.SOPLogCronInterval, cfg.SOPLogCronRunOnStart)
	}
}

func TestSensitiveWordMonitorCronRequiresMySQLAndReadsInterval(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SENSITIVE_WORD_MONITOR_CRON", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SENSITIVE_WORD_MONITOR_CRON", "1")
	t.Setenv("MOCHAT_GO_SENSITIVE_WORD_MONITOR_CRON_INTERVAL_SECONDS", "45")
	t.Setenv("MOCHAT_GO_SENSITIVE_WORD_MONITOR_CRON_RUN_ON_START", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableSensitiveWordMonitorCron || cfg.SensitiveWordMonitorCronInterval != 45*time.Second || !cfg.SensitiveWordMonitorCronRunOnStart {
		t.Fatalf("sensitiveWordsMonitor cron config = enabled %v interval %s runOnStart %v", cfg.EnableSensitiveWordMonitorCron, cfg.SensitiveWordMonitorCronInterval, cfg.SensitiveWordMonitorCronRunOnStart)
	}
}

func TestWorkMessageArchiveSyncCronRequiresMySQLBridgeAndReadsConfig(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_WORK_MESSAGE_ARCHIVE_SYNC_CRON", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_WORK_MESSAGE_ARCHIVE_BRIDGE_BASE_URL")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_WORK_MESSAGE_ARCHIVE_SYNC_CRON", "1")
	t.Setenv("MOCHAT_GO_WORK_MESSAGE_ARCHIVE_BRIDGE_BASE_URL", "https://archive-bridge.example")

	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_WORK_MESSAGE_ARCHIVE_SYNC_CRON", "1")
	t.Setenv("MOCHAT_GO_WORK_MESSAGE_ARCHIVE_SYNC_CRON_INTERVAL_SECONDS", "45")
	t.Setenv("MOCHAT_GO_WORK_MESSAGE_ARCHIVE_SYNC_CRON_RUN_ON_START", "1")
	t.Setenv("MOCHAT_GO_WORK_MESSAGE_ARCHIVE_SYNC_LIMIT", "50")
	t.Setenv("MOCHAT_GO_WORK_MESSAGE_ARCHIVE_BRIDGE_BASE_URL", "https://archive-bridge.example")
	t.Setenv("MOCHAT_GO_WORK_MESSAGE_ARCHIVE_BRIDGE_TOKEN", "bridge-token")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableWorkMessageArchiveSyncCron || cfg.WorkMessageArchiveSyncCronInterval != 45*time.Second || !cfg.WorkMessageArchiveSyncCronRunOnStart || cfg.WorkMessageArchiveSyncLimit != 50 || cfg.WorkMessageArchiveBridgeBaseURL != "https://archive-bridge.example" || cfg.WorkMessageArchiveBridgeToken != "bridge-token" {
		t.Fatalf("workMessageArchive sync cron config = enabled %v interval %s runOnStart %v limit %d bridge %q token %q", cfg.EnableWorkMessageArchiveSyncCron, cfg.WorkMessageArchiveSyncCronInterval, cfg.WorkMessageArchiveSyncCronRunOnStart, cfg.WorkMessageArchiveSyncLimit, cfg.WorkMessageArchiveBridgeBaseURL, cfg.WorkMessageArchiveBridgeToken)
	}
}

func TestSaaSStorageReconcileCronRequiresMySQLAndReadsInterval(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_STORAGE_RECONCILE_CRON", "1")

	_, err := FromEnv()
	requireErrorContains(t, err, "MOCHAT_MYSQL_DSN")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ENABLE_SAAS_STORAGE_RECONCILE_CRON", "1")
	t.Setenv("MOCHAT_GO_SAAS_STORAGE_RECONCILE_CRON_INTERVAL_SECONDS", "45")
	t.Setenv("MOCHAT_GO_SAAS_STORAGE_RECONCILE_CRON_RUN_ON_START", "1")
	t.Setenv("MOCHAT_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/mochat")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EnableSaaSStorageReconcileCron || cfg.SaaSStorageReconcileCronInterval != 45*time.Second || !cfg.SaaSStorageReconcileCronRunOnStart {
		t.Fatalf("SaaS storage reconcile cron config = enabled %v interval %s runOnStart %v", cfg.EnableSaaSStorageReconcileCron, cfg.SaaSStorageReconcileCronInterval, cfg.SaaSStorageReconcileCronRunOnStart)
	}
}

func TestInvalidNumericEnv(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOCHAT_SIMPLE_JWT_REFRESH_TTL", "0")

	_, err := FromEnv()
	requireErrorContains(t, err, "must be a positive integer")

	clearEnv(t)
	t.Setenv("MOCHAT_REDIS_DB", "-1")

	_, err = FromEnv()
	requireErrorContains(t, err, "must be a non-negative integer")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_PULL_AGENT_CRON_INTERVAL_SECONDS", "0")

	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_PULL_AGENT_CRON_INTERVAL_SECONDS must be a positive integer")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_EMPLOYEE_STATISTIC_CRON_INTERVAL_SECONDS", "0")

	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_EMPLOYEE_STATISTIC_CRON_INTERVAL_SECONDS must be a positive integer")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_CHANNEL_CODE_CRON_INTERVAL_SECONDS", "0")

	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_CHANNEL_CODE_CRON_INTERVAL_SECONDS must be a positive integer")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_CONTACT_BATCH_SEND_CRON_INTERVAL_SECONDS", "0")

	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_CONTACT_BATCH_SEND_CRON_INTERVAL_SECONDS must be a positive integer")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ROOM_BATCH_SEND_CRON_INTERVAL_SECONDS", "0")

	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_ROOM_BATCH_SEND_CRON_INTERVAL_SECONDS must be a positive integer")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_CONTACT_SYNC_SEND_RESULT_CRON_INTERVAL_SECONDS", "0")

	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_CONTACT_SYNC_SEND_RESULT_CRON_INTERVAL_SECONDS must be a positive integer")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ROOM_SYNC_SEND_RESULT_CRON_INTERVAL_SECONDS", "0")

	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_ROOM_SYNC_SEND_RESULT_CRON_INTERVAL_SECONDS must be a positive integer")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_ROOM_TAG_PULL_CRON_INTERVAL_SECONDS", "0")

	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_ROOM_TAG_PULL_CRON_INTERVAL_SECONDS must be a positive integer")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_CORP_DATA_CRON_INTERVAL_SECONDS", "0")

	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_CORP_DATA_CRON_INTERVAL_SECONDS must be a positive integer")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_MEDIA_ID_UPDATE_CRON_INTERVAL_SECONDS", "0")

	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_MEDIA_ID_UPDATE_CRON_INTERVAL_SECONDS must be a positive integer")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_TRANSFER_STATE_REFRESH_CRON_INTERVAL_SECONDS", "0")

	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_TRANSFER_STATE_REFRESH_CRON_INTERVAL_SECONDS must be a positive integer")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_STORAGE_RECONCILE_CRON_INTERVAL_SECONDS", "0")

	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_SAAS_STORAGE_RECONCILE_CRON_INTERVAL_SECONDS must be a positive integer")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_ALERT_NOTIFICATION_DISPATCH_CRON_INTERVAL_SECONDS", "0")

	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_SAAS_ALERT_NOTIFICATION_DISPATCH_CRON_INTERVAL_SECONDS must be a positive integer")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_ALERT_NOTIFICATION_DISPATCH_LIMIT", "0")

	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_SAAS_ALERT_NOTIFICATION_DISPATCH_LIMIT must be a positive integer")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_OPERATION_QUEUE_ASSIGNMENT_REMINDER_CRON_INTERVAL_SECONDS", "0")

	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_SAAS_OPERATION_QUEUE_ASSIGNMENT_REMINDER_CRON_INTERVAL_SECONDS must be a positive integer")

	clearEnv(t)
	t.Setenv("MOCHAT_GO_SAAS_OPERATION_QUEUE_ASSIGNMENT_REMINDER_LIMIT", "0")

	_, err = FromEnv()
	requireErrorContains(t, err, "MOCHAT_GO_SAAS_OPERATION_QUEUE_ASSIGNMENT_REMINDER_LIMIT must be a positive integer")
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"MOCHAT_TIMEZONE",
		"MOCHAT_GO_ADDR",
		"MOCHAT_GO_STANDALONE",
		"MOCHAT_GO_ENABLE_ALL_MIGRATED_ROUTES",
		"MOCHAT_GO_ENABLE_PHASE2_2_SCRM_PILOT",
		"MOCHAT_GO_ENABLE_FRONTEND_SERVERS",
		"MOCHAT_PHP_UPSTREAM",
		"MOCHAT_API_BASE_URL",
		"API_BASE_URL",
		"MOCHAT_DASHBOARD_BASE_URL",
		"DASHBOARD_BASE_URL",
		"MOCHAT_SIDEBAR_BASE_URL",
		"SIDEBAR_BASE_URL",
		"MOCHAT_OPERATION_BASE_URL",
		"OPERATION_BASE_URL",
		"MOCHAT_FILE_STORAGE_ROOT",
		"FILE_STORAGE_ROOT",
		"MOCHAT_DASHBOARD_DIST",
		"MOCHAT_LEGACY_DASHBOARD_DIST",
		"MOCHAT_SAAS_ADMIN_DIST",
		"MOCHAT_SIDEBAR_DIST",
		"MOCHAT_OPERATION_DIST",
		"MOCHAT_SIDEBAR_FRONTEND_ADDR",
		"MOCHAT_OPERATION_FRONTEND_ADDR",
		"MOCHAT_WECOM_API_BASE_URL",
		"MOCHAT_SOURCE_ROOT",
		"MOCHAT_COMPAT_MANIFEST",
		"MOCHAT_MYSQL_DSN",
		"MOCHAT_SIMPLE_JWT_SECRET",
		"SIMPLE_JWT_SECRET",
		"MOCHAT_SIMPLE_JWT_PREFIX",
		"SIMPLE_JWT_PREFIX",
		"MOCHAT_SIMPLE_JWT_TTL",
		"SIMPLE_JWT_TTL",
		"MOCHAT_SIMPLE_JWT_REFRESH_TTL",
		"SIMPLE_JWT_REFRESH_TTL",
		"MOCHAT_SAAS_ADMIN_JWT_SECRET",
		"MOCHAT_SAAS_ADMIN_JWT_ISSUER",
		"MOCHAT_SAAS_ADMIN_JWT_AUDIENCE",
		"MOCHAT_SAAS_ADMIN_JWT_PREFIX",
		"MOCHAT_SAAS_ADMIN_JWT_TTL",
		"MOCHAT_DASHBOARD_JWT_SECRET",
		"MOCHAT_DASHBOARD_JWT_ISSUER",
		"MOCHAT_DASHBOARD_JWT_AUDIENCE",
		"MOCHAT_DASHBOARD_JWT_PREFIX",
		"MOCHAT_DASHBOARD_JWT_TTL",
		"MOCHAT_SIDEBAR_JWT_SECRET",
		"SIDEBAR_JWT_SECRET",
		"MOCHAT_SIDEBAR_JWT_PREFIX",
		"SIDEBAR_JWT_PREFIX",
		"MOCHAT_REDIS_ADDR",
		"REDIS_HOST",
		"REDIS_PORT",
		"MOCHAT_REDIS_PASSWORD",
		"REDIS_AUTH",
		"MOCHAT_REDIS_DB",
		"REDIS_DB",
		"MOCHAT_GO_WORKER_PROCESSING_TIMEOUT_SECONDS",
		"MOCHAT_GO_ENABLE_PULL_AGENT_CRON",
		"MOCHAT_GO_PULL_AGENT_CRON_INTERVAL_SECONDS",
		"MOCHAT_GO_PULL_AGENT_CRON_RUN_ON_START",
		"MOCHAT_GO_ENABLE_EMPLOYEE_STATISTIC_CRON",
		"MOCHAT_GO_EMPLOYEE_STATISTIC_CRON_INTERVAL_SECONDS",
		"MOCHAT_GO_EMPLOYEE_STATISTIC_CRON_RUN_ON_START",
		"MOCHAT_GO_ENABLE_CHANNEL_CODE_CRON",
		"MOCHAT_GO_CHANNEL_CODE_CRON_INTERVAL_SECONDS",
		"MOCHAT_GO_CHANNEL_CODE_CRON_RUN_ON_START",
		"MOCHAT_GO_ENABLE_CONTACT_BATCH_SEND_CRON",
		"MOCHAT_GO_CONTACT_BATCH_SEND_CRON_INTERVAL_SECONDS",
		"MOCHAT_GO_CONTACT_BATCH_SEND_CRON_RUN_ON_START",
		"MOCHAT_GO_ENABLE_ROOM_BATCH_SEND_CRON",
		"MOCHAT_GO_ROOM_BATCH_SEND_CRON_INTERVAL_SECONDS",
		"MOCHAT_GO_ROOM_BATCH_SEND_CRON_RUN_ON_START",
		"MOCHAT_GO_ENABLE_CONTACT_SYNC_SEND_RESULT_CRON",
		"MOCHAT_GO_CONTACT_SYNC_SEND_RESULT_CRON_INTERVAL_SECONDS",
		"MOCHAT_GO_CONTACT_SYNC_SEND_RESULT_CRON_RUN_ON_START",
		"MOCHAT_GO_ENABLE_ROOM_SYNC_SEND_RESULT_CRON",
		"MOCHAT_GO_ROOM_SYNC_SEND_RESULT_CRON_INTERVAL_SECONDS",
		"MOCHAT_GO_ROOM_SYNC_SEND_RESULT_CRON_RUN_ON_START",
		"MOCHAT_GO_ENABLE_ROOM_TAG_PULL_CRON",
		"MOCHAT_GO_ROOM_TAG_PULL_CRON_INTERVAL_SECONDS",
		"MOCHAT_GO_ROOM_TAG_PULL_CRON_RUN_ON_START",
		"MOCHAT_GO_ENABLE_CORP_DATA_CRON",
		"MOCHAT_GO_CORP_DATA_CRON_INTERVAL_SECONDS",
		"MOCHAT_GO_CORP_DATA_CRON_RUN_ON_START",
		"MOCHAT_GO_ENABLE_MEDIA_ID_UPDATE_CRON",
		"MOCHAT_GO_MEDIA_ID_UPDATE_CRON_INTERVAL_SECONDS",
		"MOCHAT_GO_MEDIA_ID_UPDATE_CRON_RUN_ON_START",
		"MOCHAT_GO_ENABLE_TRANSFER_STATE_REFRESH_CRON",
		"MOCHAT_GO_TRANSFER_STATE_REFRESH_CRON_INTERVAL_SECONDS",
		"MOCHAT_GO_TRANSFER_STATE_REFRESH_CRON_RUN_ON_START",
		"MOCHAT_GO_ENABLE_SAAS_STORAGE_RECONCILE_CRON",
		"MOCHAT_GO_SAAS_STORAGE_RECONCILE_CRON_INTERVAL_SECONDS",
		"MOCHAT_GO_SAAS_STORAGE_RECONCILE_CRON_RUN_ON_START",
		"MOCHAT_GO_ENABLE_SAAS_ALERT_NOTIFICATION_DISPATCH_CRON",
		"MOCHAT_GO_SAAS_ALERT_NOTIFICATION_DISPATCH_CRON_INTERVAL_SECONDS",
		"MOCHAT_GO_SAAS_ALERT_NOTIFICATION_DISPATCH_CRON_RUN_ON_START",
		"MOCHAT_GO_SAAS_ALERT_NOTIFICATION_DISPATCH_LIMIT",
		"MOCHAT_GO_SAAS_ALERT_CREDENTIAL_ENCRYPTION_KEY",
		"MOCHAT_GO_SAAS_ALERT_CREDENTIAL_ENCRYPTION_KEYS",
		"MOCHAT_GO_SAAS_ALERT_CREDENTIAL_ENCRYPTION_KEY_ID",
		"MOCHAT_GO_SAAS_ALERT_CREDENTIAL_REQUIRE_ENCRYPTION",
		"MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY",
		"MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEYS",
		"MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY_ID",
		"MOCHAT_GO_WECOM_CREDENTIAL_REQUIRE_ENCRYPTION",
		"MOCHAT_GO_ENABLE_SAAS_OPERATION_QUEUE_ASSIGNMENT_REMINDER_CRON",
		"MOCHAT_GO_SAAS_OPERATION_QUEUE_ASSIGNMENT_REMINDER_CRON_INTERVAL_SECONDS",
		"MOCHAT_GO_SAAS_OPERATION_QUEUE_ASSIGNMENT_REMINDER_CRON_RUN_ON_START",
		"MOCHAT_GO_SAAS_OPERATION_QUEUE_ASSIGNMENT_REMINDER_LIMIT",
		"MOCHAT_GO_ENABLE_SAAS_APPROVAL_REMINDER_CRON",
		"MOCHAT_GO_SAAS_APPROVAL_REMINDER_CRON_INTERVAL_SECONDS",
		"MOCHAT_GO_SAAS_APPROVAL_REMINDER_CRON_RUN_ON_START",
		"MOCHAT_GO_SAAS_APPROVAL_REMINDER_LIMIT",
		"MOCHAT_GO_ENABLE_SAAS_SYSTEM_HEALTH_CRON",
		"MOCHAT_GO_SAAS_SYSTEM_HEALTH_CRON_INTERVAL_SECONDS",
		"MOCHAT_GO_SAAS_SYSTEM_HEALTH_CRON_RUN_ON_START",
		"MOCHAT_GO_SAAS_SYSTEM_HEALTH_FAILURE_WINDOW_HOURS",
		"MOCHAT_GO_SAAS_SYSTEM_HEALTH_NOTIFICATION_STALE_MINUTES",
		"MOCHAT_GO_ENABLE_SAAS_BACKUP_CRON",
		"MOCHAT_GO_SAAS_BACKUP_CRON_INTERVAL_SECONDS",
		"MOCHAT_GO_SAAS_BACKUP_CRON_RUN_ON_START",
		"MOCHAT_GO_SAAS_BACKUP_ROOT",
		"MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY",
		"MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEYS",
		"MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY_ID",
		"MOCHAT_GO_SAAS_BACKUP_DUMP_BINARY",
		"MOCHAT_GO_SAAS_BACKUP_RESTORE_BINARY",
		"MOCHAT_GO_SAAS_BACKUP_RESTORE_DSN",
		"MOCHAT_GO_SAAS_BACKUP_RESTORE_ADMIN_DSN",
		"MOCHAT_GO_SAAS_BACKUP_RESTORE_AUTO_PROVISION",
		"MOCHAT_GO_SAAS_BACKUP_RESTORE_KEEP_ON_FAILURE",
		"MOCHAT_GO_SAAS_BACKUP_RESTORE_DATABASE_PREFIX",
		"MOCHAT_GO_SAAS_BACKUP_S3_ENDPOINT",
		"MOCHAT_GO_SAAS_BACKUP_S3_BUCKET",
		"MOCHAT_GO_SAAS_BACKUP_S3_REGION",
		"MOCHAT_GO_SAAS_BACKUP_S3_ACCESS_KEY_ID",
		"MOCHAT_GO_SAAS_BACKUP_S3_SECRET_ACCESS_KEY",
		"MOCHAT_GO_SAAS_BACKUP_S3_SESSION_TOKEN",
		"MOCHAT_GO_SAAS_BACKUP_S3_USE_SSL",
		"MOCHAT_GO_SAAS_BACKUP_S3_PREFIX",
		"MOCHAT_GO_ENABLE_SAAS_COMPLIANCE_CRON",
		"MOCHAT_GO_SAAS_COMPLIANCE_CRON_INTERVAL_SECONDS",
		"MOCHAT_GO_SAAS_COMPLIANCE_CRON_RUN_ON_START",
		"MOCHAT_GO_SAAS_COMPLIANCE_ARTIFACT_ROOT",
		"MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY",
		"MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEYS",
		"MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY_ID",
		"MOCHAT_GO_ENABLE_SAAS_AUDIT_INTEGRITY_CRON",
		"MOCHAT_GO_SAAS_AUDIT_INTEGRITY_CRON_INTERVAL_SECONDS",
		"MOCHAT_GO_SAAS_AUDIT_INTEGRITY_CRON_RUN_ON_START",
		"MOCHAT_GO_SAAS_AUDIT_INTEGRITY_LIMIT",
		"MOCHAT_GO_ENABLE_SAAS_AUDIT_ANCHOR_CRON",
		"MOCHAT_GO_SAAS_AUDIT_ANCHOR_CRON_INTERVAL_SECONDS",
		"MOCHAT_GO_SAAS_AUDIT_ANCHOR_CRON_RUN_ON_START",
		"MOCHAT_GO_SAAS_AUDIT_ANCHOR_LIMIT",
		"MOCHAT_GO_SAAS_AUDIT_ANCHOR_ARTIFACT_ROOT",
		"MOCHAT_GO_SAAS_AUDIT_ANCHOR_HMAC_KEY",
		"MOCHAT_GO_SAAS_AUDIT_ANCHOR_HMAC_KEYS",
		"MOCHAT_GO_SAAS_AUDIT_ANCHOR_HMAC_KEY_ID",
		"MOCHAT_GO_SAAS_AUDIT_ANCHOR_REQUIRE_REMOTE",
		"MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_ENDPOINT",
		"MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_BUCKET",
		"MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_REGION",
		"MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_ACCESS_KEY_ID",
		"MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_SECRET_ACCESS_KEY",
		"MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_SESSION_TOKEN",
		"MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_USE_SSL",
		"MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_PREFIX",
		"MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_RETENTION_MODE",
		"MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_RETENTION_DAYS",
		"MOCHAT_GO_ENABLE_SAAS_IDENTITY_SECURITY",
		"MOCHAT_GO_SAAS_IDENTITY_ENFORCE_SESSIONS",
		"MOCHAT_GO_SAAS_IDENTITY_TRUST_PROXY_HEADERS",
		"MOCHAT_GO_SAAS_SERVICE_ACCOUNT_TRUST_PROXY_HEADERS",
		"MOCHAT_GO_SAAS_TRUSTED_PROXY_CIDRS",
		"MOCHAT_GO_SAAS_IDENTITY_ISSUER",
		"MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY",
		"MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEYS",
		"MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY_ID",
		"MOCHAT_GO_ENABLE_SAAS_IDENTITY_CLEANUP_CRON",
		"MOCHAT_GO_SAAS_IDENTITY_CLEANUP_CRON_INTERVAL_SECONDS",
		"MOCHAT_GO_SAAS_IDENTITY_CLEANUP_CRON_RUN_ON_START",
		"MOCHAT_GO_SAAS_IDENTITY_CLEANUP_LIMIT",
		"MOCHAT_GO_ENABLE_SAAS_SERVICE_ACCOUNT_USAGE_ALERT_CRON",
		"MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_ALERT_CRON_INTERVAL_SECONDS",
		"MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_ALERT_CRON_RUN_ON_START",
		"MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_ALERT_LIMIT",
		"MOCHAT_GO_SAAS_SERVICE_ACCOUNT_KEY_PEPPER",
		"MOCHAT_GO_SAAS_SERVICE_ACCOUNT_KEY_PEPPERS",
		"MOCHAT_GO_SAAS_SERVICE_ACCOUNT_KEY_PEPPER_ID",
		"MOCHAT_GO_SAAS_SERVICE_ACCOUNT_ALLOW_LEGACY_JWT_PEPPER",
		"MOCHAT_GO_SAAS_SERVICE_ACCOUNT_REQUIRE_DEDICATED_PEPPER",
		"MOCHAT_GO_ENABLE_SAAS_SERVICE_ACCOUNT_USAGE_CLEANUP_CRON",
		"MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_CLEANUP_CRON_INTERVAL_SECONDS",
		"MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_CLEANUP_CRON_RUN_ON_START",
		"MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_CLEANUP_LIMIT",
		"MOCHAT_GO_SAAS_TENANT_DOMAIN_DNS_SERVER",
		"MOCHAT_GO_SAAS_TENANT_DOMAIN_DNS_TIMEOUT_SECONDS",
		"MOCHAT_GO_ENABLE_SAAS_TENANT_DOMAIN_DELIVERY_CRON",
		"MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CRON_INTERVAL_SECONDS",
		"MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CRON_RUN_ON_START",
		"MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_BRIDGE_BASE_URL",
		"MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_BRIDGE_TOKEN",
		"MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_BRIDGE_TIMEOUT_SECONDS",
		"MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CALLBACK_URL",
		"MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CALLBACK_SECRET",
		"MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CALLBACK_TOLERANCE_SECONDS",
		"MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_LIMIT",
		"MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_LEASE_SECONDS",
		"MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_RETRY_DELAY_SECONDS",
		"MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CALLBACK_WAIT_SECONDS",
		"MOCHAT_GO_ENABLE_SAAS_NOTIFICATION_HEALTH_RECOVERY_CRON",
		"MOCHAT_GO_SAAS_NOTIFICATION_HEALTH_RECOVERY_CRON_INTERVAL_SECONDS",
		"MOCHAT_GO_SAAS_NOTIFICATION_HEALTH_RECOVERY_CRON_RUN_ON_START",
		"MOCHAT_GO_SAAS_NOTIFICATION_HEALTH_RECOVERY_WINDOW_HOURS",
		"MOCHAT_GO_SAAS_NOTIFICATION_HEALTH_RECOVERY_STALE_MINUTES",
		"MOCHAT_GO_ENABLE_SAAS_PAYMENT_SETTLEMENT_SYNC_CRON",
		"MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_SYNC_CRON_INTERVAL_SECONDS",
		"MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_SYNC_CRON_RUN_ON_START",
		"MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_BRIDGE_BASE_URL",
		"MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_BRIDGE_TOKEN",
		"MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_PROVIDERS",
		"MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_SYNC_LIMIT",
		"MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_BRIDGE_TIMEOUT_SECONDS",
		"MOCHAT_GO_ENABLE_SAAS_ALERT_DASHBOARD",
		"MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD",
		"MOCHAT_GO_RELEASE_SOURCE_FINGERPRINT",
		"MOCHAT_GO_ENABLE_SAAS_BILLING_PORTAL",
		"MOCHAT_GO_SAAS_ALERT_WEBHOOK_URL",
		"MOCHAT_GO_SAAS_ALERT_WEBHOOK_SECRET",
		"MOCHAT_GO_SAAS_ALERT_WEBHOOK_TIMEOUT_SECONDS",
		"MOCHAT_GO_SAAS_ALERT_WEBHOOK_RETRY_ATTEMPTS",
		"MOCHAT_GO_SAAS_ALERT_WEBHOOK_RETRY_DELAY_MS",
		"MOCHAT_GO_SAAS_ALERT_WEBHOOK_TITLE_TEMPLATE",
		"MOCHAT_GO_SAAS_ALERT_WEBHOOK_BODY_TEMPLATE",
		"MOCHAT_GO_SAAS_ALERT_WEBHOOK_REQUIRE_HTTPS",
		"MOCHAT_GO_SAAS_ALERT_WEBHOOK_ALLOWED_CIDRS",
		"MOCHAT_GO_SAAS_ALERT_NOTIFICATION_MAX_ATTEMPTS",
		"MOCHAT_GO_SAAS_ALERT_NOTIFICATION_RETRY_DELAY_SECONDS",
		"MOCHAT_GO_ENABLE_WEWORK_CALLBACK_WORKER",
		"MOCHAT_GO_ENABLE_EMPLOYEE_APPLY_WORKER",
		"MOCHAT_GO_ENABLE_ASYNC_FILE_UPLOAD_WORKER",
		"MOCHAT_GO_ENABLE_MARK_TAGS_WORKER",
		"MOCHAT_GO_ENABLE_MESSAGE_REMIND_WORKER",
		"MOCHAT_GO_ENABLE_WORK_ROOM_SYNC_WORKER",
		"MOCHAT_GO_ENABLE_WORK_CONTACT_SYNC_WORKER",
		"MOCHAT_GO_ENABLE_WORK_DEPARTMENT_LIST_WORKER",
		"MOCHAT_GO_ENABLE_MEDIA_ID_UPDATE_WORKER",
		"MOCHAT_GO_ENABLE_EMPLOYEE_STATISTIC_WORKER",
		"MOCHAT_GO_MIGRATE_AUTH",
		"MOCHAT_GO_MIGRATE_LOGIN_SHOW",
		"MOCHAT_GO_MIGRATE_LOGOUT",
		"MOCHAT_GO_MIGRATE_USER_INDEX",
		"MOCHAT_GO_MIGRATE_USER_SHOW",
		"MOCHAT_GO_MIGRATE_USER_STORE",
		"MOCHAT_GO_MIGRATE_USER_UPDATE",
		"MOCHAT_GO_MIGRATE_USER_STATUS_UPDATE",
		"MOCHAT_GO_MIGRATE_USER_PASSWORD_RESET",
		"MOCHAT_GO_MIGRATE_USER_PASSWORD_UPDATE",
		"MOCHAT_GO_MIGRATE_PERMISSION_BY_USER",
		"MOCHAT_GO_MIGRATE_CORP_SELECT",
		"MOCHAT_GO_MIGRATE_CORP_BIND",
		"MOCHAT_GO_MIGRATE_CORP_INDEX",
		"MOCHAT_GO_MIGRATE_CORP_SHOW",
		"MOCHAT_GO_MIGRATE_CORP_STORE",
		"MOCHAT_GO_MIGRATE_CORP_UPDATE",
		"MOCHAT_GO_MIGRATE_WEWORK_CALLBACK",
		"MOCHAT_GO_MIGRATE_CORP_DATA_INDEX",
		"MOCHAT_GO_MIGRATE_CORP_DATA_LINE_CHAT",
		"MOCHAT_GO_MIGRATE_STATISTIC_INDEX",
		"MOCHAT_GO_MIGRATE_STATISTIC_TOP_LIST",
		"MOCHAT_GO_MIGRATE_STATISTIC_EMPLOYEE_COUNTS",
		"MOCHAT_GO_MIGRATE_STATISTIC_EMPLOYEES",
		"MOCHAT_GO_MIGRATE_STATISTIC_EMPLOYEES_TREND",
		"MOCHAT_GO_MIGRATE_WORK_EMPLOYEE_SYNC",
		"MOCHAT_GO_MIGRATE_WORK_EMPLOYEE_SEARCH_CONDITION",
		"MOCHAT_GO_MIGRATE_WORK_DEPARTMENT_INDEX",
		"MOCHAT_GO_MIGRATE_CHAT_TOOL_CONFIG",
		"MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_TAG_GROUP_INDEX",
		"MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_SYNC",
		"MOCHAT_GO_MIGRATE_WORK_CONTACT_SYNC",
		"MOCHAT_GO_MIGRATE_WORK_CONTACT_SOURCE",
		"MOCHAT_GO_MIGRATE_WORK_CONTACT_SHOW",
		"MOCHAT_GO_MIGRATE_WORK_CONTACT_TRACK",
		"MOCHAT_GO_MIGRATE_WORK_CONTACT_UPDATE",
		"MOCHAT_GO_MIGRATE_WORK_CONTACT_BATCH_LABELING",
		"MOCHAT_GO_MIGRATE_WORK_CONTACT_ROOM_INDEX",
		"MOCHAT_GO_MIGRATE_WORK_ROOM_ROOM_INDEX",
		"MOCHAT_GO_MIGRATE_WORK_ROOM_STATISTICS",
		"MOCHAT_GO_MIGRATE_WORK_ROOM_STATISTICS_INDEX",
		"MOCHAT_GO_MIGRATE_WORK_ROOM_SYNC",
		"MOCHAT_GO_MIGRATE_WORK_ROOM_BATCH_UPDATE",
		"MOCHAT_GO_MIGRATE_SIDEBAR_WORK_ROOM_MANAGE",
		"MOCHAT_GO_MIGRATE_CONTACT_TRANSFER_INFO",
		"MOCHAT_GO_MIGRATE_CONTACT_TRANSFER_UNASSIGNED_LIST",
		"MOCHAT_GO_MIGRATE_CONTACT_TRANSFER_ROOM",
		"MOCHAT_GO_MIGRATE_CONTACT_TRANSFER_LOG",
		"MOCHAT_GO_MIGRATE_CONTACT_TRANSFER_SAVE_UNASSIGNED_LIST",
		"MOCHAT_GO_MIGRATE_CONTACT_TRANSFER_INDEX",
		"MOCHAT_GO_MIGRATE_CONTACT_TRANSFER_ROOM_STORE",
		"MOCHAT_GO_MIGRATE_WORK_ROOM_AUTO_PULL_INDEX",
		"MOCHAT_GO_MIGRATE_WORK_ROOM_AUTO_PULL_SHOW",
		"MOCHAT_GO_MIGRATE_WORK_ROOM_AUTO_PULL_STORE",
		"MOCHAT_GO_MIGRATE_WORK_ROOM_AUTO_PULL_UPDATE",
		"MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_INDEX",
		"MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_SHOW",
		"MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_SHOW_CONTACT",
		"MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_ROOM_LIST",
		"MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_CHOOSE_CONTACT",
		"MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_STORE",
		"MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_FILTER_CONTACT",
		"MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_REMIND_SEND",
		"MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_DESTROY",
		"MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_INDEX",
		"MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_SHOW",
		"MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_SHOW_ROOM",
		"MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_EMPLOYEE_SEND_INDEX",
		"MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_CONTACT_RECEIVE_INDEX",
		"MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_STORE",
		"MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_REMIND",
		"MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_DESTROY",
		"MOCHAT_GO_MIGRATE_ROOM_WELCOME_INDEX",
		"MOCHAT_GO_MIGRATE_ROOM_WELCOME_SELECT",
		"MOCHAT_GO_MIGRATE_ROOM_WELCOME_SHOW",
		"MOCHAT_GO_MIGRATE_ROOM_WELCOME_STORE",
		"MOCHAT_GO_MIGRATE_ROOM_WELCOME_UPDATE",
		"MOCHAT_GO_MIGRATE_ROOM_WELCOME_DESTROY",
		"MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_TAG_ALL",
		"MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_DETAIL",
		"MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_SHOW",
		"MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_TRACK",
		"MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_UPDATE",
		"MOCHAT_GO_MIGRATE_SIDEBAR_CONTACT_PROCESS_STATUS_INDEX",
		"MOCHAT_GO_MIGRATE_SIDEBAR_CONTACT_PROCESS_STATUS_UPDATE",
		"MOCHAT_GO_MIGRATE_CONTACT_FIELD_PIVOT_UPDATE",
		"MOCHAT_GO_MIGRATE_SIDEBAR_MEDIUM_MEDIA_ID_UPDATE",
		"MOCHAT_GO_MIGRATE_SIDEBAR_CONTACT_FIELD_PIVOT_INDEX",
		"MOCHAT_GO_MIGRATE_SIDEBAR_CONTACT_FIELD_PIVOT_UPDATE",
		"MOCHAT_GO_MIGRATE_CHANNEL_CODE_INDEX",
		"MOCHAT_GO_MIGRATE_CHANNEL_CODE_SHOW",
		"MOCHAT_GO_MIGRATE_CHANNEL_CODE_CONTACT",
		"MOCHAT_GO_MIGRATE_CHANNEL_CODE_STATISTICS",
		"MOCHAT_GO_MIGRATE_CHANNEL_CODE_STATISTICS_INDEX",
		"MOCHAT_GO_MIGRATE_CHANNEL_CODE_STORE",
		"MOCHAT_GO_MIGRATE_CHANNEL_CODE_UPDATE",
		"MOCHAT_GO_MIGRATE_CHANNEL_CODE_GROUP_INDEX",
		"MOCHAT_GO_MIGRATE_CHANNEL_CODE_GROUP_DETAIL",
		"MOCHAT_GO_MIGRATE_CHANNEL_CODE_GROUP_STORE",
		"MOCHAT_GO_MIGRATE_CHANNEL_CODE_GROUP_UPDATE",
		"MOCHAT_GO_MIGRATE_CHANNEL_CODE_GROUP_MOVE",
		"MOCHAT_GO_MIGRATE_COMMON_UPLOAD",
		"MOCHAT_GO_MIGRATE_COMMON_UPLOAD_FILE",
		"MOCHAT_GO_MIGRATE_SIDEBAR_COMMON_UPLOAD",
		"MOCHAT_GO_MIGRATE_AGENT_TXT_VERIFY",
		"MOCHAT_GO_MIGRATE_AGENT_TXT_VERIFY_UPLOAD",
		"MOCHAT_GO_MIGRATE_AGENT_STORE",
		"MOCHAT_GO_MIGRATE_SIDEBAR_AGENT_AUTH",
		"MOCHAT_GO_MIGRATE_SIDEBAR_AGENT_OAUTH",
		"MOCHAT_GO_MIGRATE_SIDEBAR_AGENT_JSSDK_CONFIG",
		"MOCHAT_GO_MIGRATE_SIDEBAR_WX_JS_SDK_CONFIG",
		"MOCHAT_GO_MIGRATE_ROLE_SELECT",
		"MOCHAT_GO_MIGRATE_ROLE_INDEX",
		"MOCHAT_GO_MIGRATE_ROLE_SHOW",
		"MOCHAT_GO_MIGRATE_ROLE_PERMISSION_SHOW",
		"MOCHAT_GO_MIGRATE_ROLE_SHOW_EMPLOYEE",
		"MOCHAT_GO_MIGRATE_MENU_ICON_INDEX",
		"MOCHAT_GO_MIGRATE_MENU_SELECT",
		"MOCHAT_GO_MIGRATE_MENU_INDEX",
		"MOCHAT_GO_MIGRATE_MENU_SHOW",
		"MOCHAT_GO_DEV_AUTH_HEADER",
		"MOCHAT_GO_SKIP_JWT_BLACKLIST",
		"MOCHAT_PROXY_TIMEOUT_SECONDS",
	} {
		t.Setenv(key, "")
	}
}

func requireErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("err = nil, want containing %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("err = %q, want containing %q", err.Error(), want)
	}
}
