package config

import (
	"bytes"
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	appruntime "jiyi/mochat-go/internal/app/runtime"
	"jiyi/mochat-go/internal/buildinfo"
	"jiyi/mochat-go/internal/clientip"
	"jiyi/mochat-go/internal/outboundhttp"
	"jiyi/mochat-go/internal/saasalertcredentials"
	"jiyi/mochat-go/internal/wechatopencredentials"
	"jiyi/mochat-go/internal/wecomcredentials"
)

const (
	DefaultSaaSAlertWebhookTitleTemplate = "SaaS额度告警：租户 {{.TenantID}} {{.Metric}}"
	DefaultSaaSAlertWebhookBodyTemplate  = "{{.Message}}（当前 {{.CurrentValue}} / 上限 {{.LimitValue}}，来源 {{.Source}}）"
)

type Config struct {
	RuntimeRole                                        appruntime.Role
	ListenAddr                                         string
	Timezone                                           string
	Standalone                                         bool
	EnableAllMigratedRoutes                            bool
	EnablePhase22SCRMPilot                             bool
	PHPUpstream                                        string
	APIBaseURL                                         string
	DashboardBaseURL                                   string
	SidebarBaseURL                                     string
	OperationBaseURL                                   string
	FileStorageRoot                                    string
	DashboardDist                                      string
	SaaSAdminDist                                      string
	SidebarDist                                        string
	OperationDist                                      string
	SidebarFrontendAddr                                string
	OperationFrontendAddr                              string
	WeComAPIBaseURL                                    string
	WeChatAPIBaseURL                                   string
	WeChatOpenPlatformAppID                            string
	WeChatOpenPlatformSecret                           string
	WeChatOpenPlatformToken                            string
	WeChatOpenPlatformAESKey                           string
	WeChatComponentVerifyTicket                        string
	SourceRoot                                         string
	ManifestPath                                       string
	MySQLDSN                                           string
	SimpleJWTSecret                                    string
	SimpleJWTPrefix                                    string
	SimpleJWTTTL                                       time.Duration
	SimpleJWTRefreshTTL                                time.Duration
	SidebarJWTSecret                                   string
	SidebarJWTPrefix                                   string
	RedisAddr                                          string
	RedisPassword                                      string
	RedisDB                                            int
	WorkerProcessingTimeout                            time.Duration
	EnablePullAgentCron                                bool
	PullAgentCronInterval                              time.Duration
	PullAgentCronRunOnStart                            bool
	EnableEmployeeStatisticCron                        bool
	EmployeeStatisticCronInterval                      time.Duration
	EmployeeStatisticCronRunOnStart                    bool
	EnableChannelCodeCron                              bool
	ChannelCodeCronInterval                            time.Duration
	ChannelCodeCronRunOnStart                          bool
	EnableContactBatchSendCron                         bool
	ContactBatchSendCronInterval                       time.Duration
	ContactBatchSendCronRunOnStart                     bool
	EnableRoomBatchSendCron                            bool
	RoomBatchSendCronInterval                          time.Duration
	RoomBatchSendCronRunOnStart                        bool
	EnableContactSyncSendResultCron                    bool
	ContactSyncSendResultCronInterval                  time.Duration
	ContactSyncSendResultCronRunOnStart                bool
	EnableRoomSyncSendResultCron                       bool
	RoomSyncSendResultCronInterval                     time.Duration
	RoomSyncSendResultCronRunOnStart                   bool
	EnableRoomTagPullCron                              bool
	RoomTagPullCronInterval                            time.Duration
	RoomTagPullCronRunOnStart                          bool
	EnableCorpDataCron                                 bool
	CorpDataCronInterval                               time.Duration
	CorpDataCronRunOnStart                             bool
	EnableMediaIDUpdateCron                            bool
	MediaIDUpdateCronInterval                          time.Duration
	MediaIDUpdateCronRunOnStart                        bool
	EnableTransferStateRefreshCron                     bool
	TransferStateRefreshCronInterval                   time.Duration
	TransferStateRefreshCronRunOnStart                 bool
	EnableSOPLogCron                                   bool
	SOPLogCronInterval                                 time.Duration
	SOPLogCronRunOnStart                               bool
	EnableSensitiveWordMonitorCron                     bool
	SensitiveWordMonitorCronInterval                   time.Duration
	SensitiveWordMonitorCronRunOnStart                 bool
	EnableWorkMessageArchiveSyncCron                   bool
	WorkMessageArchiveSyncCronInterval                 time.Duration
	WorkMessageArchiveSyncCronRunOnStart               bool
	WorkMessageArchiveSyncLimit                        int
	WorkMessageArchiveBridgeBaseURL                    string
	WorkMessageArchiveBridgeToken                      string
	EnableSaaSStorageReconcileCron                     bool
	SaaSStorageReconcileCronInterval                   time.Duration
	SaaSStorageReconcileCronRunOnStart                 bool
	EnableSaaSAlertNotificationDispatchCron            bool
	SaaSAlertNotificationDispatchCronInterval          time.Duration
	SaaSAlertNotificationDispatchCronRunOnStart        bool
	SaaSAlertNotificationDispatchLimit                 int
	SaaSAlertCredentialEncryptionKey                   string
	SaaSAlertCredentialEncryptionKeys                  string
	SaaSAlertCredentialEncryptionKeyID                 string
	SaaSAlertCredentialRequireEncryption               bool
	SaaSAlertCredentialDedicatedConfigured             bool
	WeComCredentialEncryptionKey                       string
	WeComCredentialEncryptionKeys                      string
	WeComCredentialEncryptionKeyID                     string
	WeComCredentialRequireEncryption                   bool
	WeComCredentialDedicatedConfigured                 bool
	WeChatOpenCredentialEncryptionKey                  string
	WeChatOpenCredentialEncryptionKeys                 string
	WeChatOpenCredentialEncryptionKeyID                string
	WeChatOpenCredentialRequireEncryption              bool
	WeChatOpenCredentialDedicatedConfigured            bool
	EnableSaaSOperationQueueAssignmentReminderCron     bool
	SaaSOperationQueueAssignmentReminderCronInterval   time.Duration
	SaaSOperationQueueAssignmentReminderCronRunOnStart bool
	SaaSOperationQueueAssignmentReminderLimit          int
	EnableSaaSApprovalReminderCron                     bool
	SaaSApprovalReminderCronInterval                   time.Duration
	SaaSApprovalReminderCronRunOnStart                 bool
	SaaSApprovalReminderLimit                          int
	EnableSaaSSystemHealthCron                         bool
	SaaSSystemHealthCronInterval                       time.Duration
	SaaSSystemHealthCronRunOnStart                     bool
	SaaSSystemHealthFailureWindowHours                 int
	SaaSSystemHealthNotificationStaleMinutes           int
	EnableSaaSBackupCron                               bool
	SaaSBackupCronInterval                             time.Duration
	SaaSBackupCronRunOnStart                           bool
	SaaSBackupRoot                                     string
	SaaSBackupEncryptionKey                            string
	SaaSBackupEncryptionKeys                           string
	SaaSBackupEncryptionKeyID                          string
	SaaSBackupDumpBinary                               string
	SaaSBackupRestoreBinary                            string
	SaaSBackupRestoreDSN                               string
	SaaSBackupRestoreAdminDSN                          string
	SaaSBackupRestoreAutoProvision                     bool
	SaaSBackupRestoreKeepOnFailure                     bool
	SaaSBackupRestoreDatabasePrefix                    string
	SaaSBackupS3Endpoint                               string
	SaaSBackupS3Bucket                                 string
	SaaSBackupS3Region                                 string
	SaaSBackupS3AccessKeyID                            string
	SaaSBackupS3SecretAccessKey                        string
	SaaSBackupS3SessionToken                           string
	SaaSBackupS3UseSSL                                 bool
	SaaSBackupS3Prefix                                 string
	EnableSaaSComplianceCron                           bool
	SaaSComplianceCronInterval                         time.Duration
	SaaSComplianceCronRunOnStart                       bool
	SaaSComplianceArtifactRoot                         string
	SaaSComplianceEncryptionKey                        string
	SaaSComplianceEncryptionKeys                       string
	SaaSComplianceEncryptionKeyID                      string
	EnableSaaSIdentitySecurity                         bool
	SaaSIdentityEnforceSessions                        bool
	SaaSIdentityTrustProxyHeaders                      bool
	SaaSServiceAccountTrustProxyHeaders                bool
	SaaSTrustedProxyCIDRs                              []string
	SaaSIdentityIssuer                                 string
	SaaSIdentityEncryptionKey                          string
	SaaSIdentityEncryptionKeys                         string
	SaaSIdentityEncryptionKeyID                        string
	EnableSaaSIdentityCleanupCron                      bool
	SaaSIdentityCleanupCronInterval                    time.Duration
	SaaSIdentityCleanupCronRunOnStart                  bool
	SaaSIdentityCleanupLimit                           int
	EnableSaaSServiceAccountUsageAlertCron             bool
	SaaSServiceAccountUsageAlertCronInterval           time.Duration
	SaaSServiceAccountUsageAlertCronRunOnStart         bool
	SaaSServiceAccountUsageAlertLimit                  int
	SaaSServiceAccountKeyPepper                        string
	SaaSServiceAccountKeyPeppers                       string
	SaaSServiceAccountKeyPepperID                      string
	SaaSServiceAccountAllowLegacyJWTPepper             bool
	SaaSServiceAccountRequireDedicatedPepper           bool
	EnableSaaSAuditIntegrityCron                       bool
	SaaSAuditIntegrityCronInterval                     time.Duration
	SaaSAuditIntegrityCronRunOnStart                   bool
	SaaSAuditIntegrityLimit                            int
	EnableSaaSAuditAnchorCron                          bool
	SaaSAuditAnchorCronInterval                        time.Duration
	SaaSAuditAnchorCronRunOnStart                      bool
	SaaSAuditAnchorLimit                               int
	SaaSAuditAnchorArtifactRoot                        string
	SaaSAuditAnchorHMACKey                             string
	SaaSAuditAnchorHMACKeys                            string
	SaaSAuditAnchorHMACKeyID                           string
	SaaSAuditAnchorRequireRemote                       bool
	SaaSAuditAnchorS3Endpoint                          string
	SaaSAuditAnchorS3Bucket                            string
	SaaSAuditAnchorS3Region                            string
	SaaSAuditAnchorS3AccessKeyID                       string
	SaaSAuditAnchorS3SecretAccessKey                   string
	SaaSAuditAnchorS3SessionToken                      string
	SaaSAuditAnchorS3UseSSL                            bool
	SaaSAuditAnchorS3Prefix                            string
	SaaSAuditAnchorS3RetentionMode                     string
	SaaSAuditAnchorS3RetentionDays                     int
	EnableSaaSServiceAccountUsageCleanupCron           bool
	SaaSServiceAccountUsageCleanupCronInterval         time.Duration
	SaaSServiceAccountUsageCleanupCronRunOnStart       bool
	SaaSServiceAccountUsageCleanupLimit                int
	SaaSTenantDomainDNSServer                          string
	SaaSTenantDomainDNSTimeout                         time.Duration
	EnableSaaSTenantDomainDeliveryCron                 bool
	SaaSTenantDomainDeliveryCronInterval               time.Duration
	SaaSTenantDomainDeliveryCronRunOnStart             bool
	SaaSTenantDomainDeliveryBridgeBaseURL              string
	SaaSTenantDomainDeliveryBridgeToken                string
	SaaSTenantDomainDeliveryBridgeTimeout              time.Duration
	SaaSTenantDomainDeliveryCallbackURL                string
	SaaSTenantDomainDeliveryCallbackSecret             string
	SaaSTenantDomainDeliveryCallbackTolerance          time.Duration
	SaaSTenantDomainDeliveryLimit                      int
	SaaSTenantDomainDeliveryLease                      time.Duration
	SaaSTenantDomainDeliveryRetryDelay                 time.Duration
	SaaSTenantDomainDeliveryCallbackWait               time.Duration
	SaaSTenantDomainCertificateExpiryWarningDays       int
	EnableSaaSNotificationHealthRecoveryCron           bool
	SaaSNotificationHealthRecoveryCronInterval         time.Duration
	SaaSNotificationHealthRecoveryCronRunOnStart       bool
	SaaSNotificationHealthRecoveryWindowHours          int
	SaaSNotificationHealthRecoveryStaleMinutes         int
	EnableSaaSSubscriptionReconcileCron                bool
	SaaSSubscriptionReconcileCronInterval              time.Duration
	SaaSSubscriptionReconcileCronRunOnStart            bool
	SaaSSubscriptionReconcileLimit                     int
	EnableSaaSPaymentWebhook                           bool
	SaaSPaymentWebhookSecret                           string
	SaaSPaymentWebhookTolerance                        time.Duration
	EnableSaaSPaymentDunningCron                       bool
	SaaSPaymentDunningCronInterval                     time.Duration
	SaaSPaymentDunningCronRunOnStart                   bool
	SaaSPaymentDunningLimit                            int
	SaaSPaymentDunningRetryDelay                       time.Duration
	EnableSaaSPaymentSettlementSyncCron                bool
	SaaSPaymentSettlementSyncCronInterval              time.Duration
	SaaSPaymentSettlementSyncCronRunOnStart            bool
	SaaSPaymentSettlementBridgeBaseURL                 string
	SaaSPaymentSettlementBridgeToken                   string
	SaaSPaymentSettlementProviders                     []string
	SaaSPaymentSettlementSyncLimit                     int
	SaaSPaymentSettlementBridgeTimeout                 time.Duration
	EnableSaaSAlertDashboard                           bool
	EnableSaaSAdminDashboard                           bool
	SaaSAdminApprovalRequired                          bool
	EnableSaaSBillingPortal                            bool
	SaaSPlatformAdminTenantID                          int
	SaaSReleaseSourceFingerprint                       string
	SaaSReleaseSourceFingerprintSource                 string
	SaaSReleaseEvidenceVerifyTimeout                   time.Duration
	SaaSReleaseEvidenceMaxBytes                        int64
	SaaSReleaseEvidenceAllowedCIDRs                    []string
	SaaSReleaseEvidenceCAFile                          string
	SaaSAlertWebhookURL                                string
	SaaSAlertWebhookSecret                             string
	SaaSAlertWebhookTimeout                            time.Duration
	SaaSAlertWebhookRetryAttempts                      int
	SaaSAlertWebhookRetryDelay                         time.Duration
	SaaSAlertWebhookTitleTemplate                      string
	SaaSAlertWebhookBodyTemplate                       string
	SaaSAlertWebhookRequireHTTPS                       bool
	SaaSAlertWebhookAllowedCIDRs                       []string
	SaaSAlertNotificationMaxAttempts                   int
	SaaSAlertNotificationRetryDelay                    time.Duration
	MigrateAuth                                        bool
	MigrateLoginShow                                   bool
	MigrateLogout                                      bool
	MigrateUserIndex                                   bool
	MigrateUserShow                                    bool
	MigrateUserStore                                   bool
	MigrateUserUpdate                                  bool
	MigrateUserStatusUpdate                            bool
	MigrateUserPasswordReset                           bool
	MigrateUserPasswordUpdate                          bool
	MigratePermissionByUser                            bool
	MigrateCorpSelect                                  bool
	MigrateCorpBind                                    bool
	MigrateCorpIndex                                   bool
	MigrateCorpShow                                    bool
	MigrateCorpStore                                   bool
	MigrateCorpUpdate                                  bool
	MigrateWeWorkCallback                              bool
	MigrateCorpDataIndex                               bool
	MigrateCorpDataLineChat                            bool
	MigrateStatisticIndex                              bool
	MigrateStatisticTopList                            bool
	MigrateStatisticEmployeeCounts                     bool
	MigrateStatisticEmployees                          bool
	MigrateStatisticEmployeesTrend                     bool
	MigrateWorkEmployeeIndex                           bool
	MigrateWorkEmployeeCond                            bool
	MigrateWorkEmployeeSync                            bool
	MigrateWorkDeptIndex                               bool
	MigrateWorkDeptMember                              bool
	MigrateWorkDeptPhone                               bool
	MigrateWorkDeptPage                                bool
	MigrateWorkDeptEmployee                            bool
	MigrateWorkTagGroupIndex                           bool
	MigrateWorkTagGroupDetail                          bool
	MigrateWorkTagGroupStore                           bool
	MigrateWorkTagGroupUpdate                          bool
	MigrateWorkTagGroupDestroy                         bool
	MigrateSidebarTagGroupIndex                        bool
	MigrateWorkContactTagIndex                         bool
	MigrateWorkContactTagDetail                        bool
	MigrateWorkContactTagList                          bool
	MigrateWorkContactTagAll                           bool
	MigrateWorkContactTagStore                         bool
	MigrateWorkContactTagUpdate                        bool
	MigrateWorkContactTagDestroy                       bool
	MigrateWorkContactTagMove                          bool
	MigrateWorkContactTagSync                          bool
	MigrateWorkContactSync                             bool
	MigrateWorkContactIndex                            bool
	MigrateWorkContactLoss                             bool
	MigrateWorkContactSource                           bool
	MigrateWorkContactShow                             bool
	MigrateWorkContactTrack                            bool
	MigrateWorkContactUpdate                           bool
	MigrateWorkContactBatchLabeling                    bool
	MigrateWorkContactRoomIndex                        bool
	MigrateWorkRoomIndex                               bool
	MigrateWorkRoomRoomIndex                           bool
	MigrateWorkRoomStatistics                          bool
	MigrateWorkRoomStatisticsIndex                     bool
	MigrateWorkRoomSync                                bool
	MigrateWorkRoomBatchUpdate                         bool
	MigrateSidebarWorkRoomManage                       bool
	MigrateContactTransferInfo                         bool
	MigrateContactTransferUnassigned                   bool
	MigrateContactTransferRoom                         bool
	MigrateContactTransferLog                          bool
	MigrateContactTransferSync                         bool
	MigrateContactTransferCustomer                     bool
	MigrateContactTransferRoomStore                    bool
	MigrateWorkRoomAutoPullIndex                       bool
	MigrateWorkRoomAutoPullShow                        bool
	MigrateWorkRoomAutoPullStore                       bool
	MigrateWorkRoomAutoPullUpdate                      bool
	MigrateRoomTagPullIndex                            bool
	MigrateRoomTagPullShow                             bool
	MigrateRoomTagPullShowContact                      bool
	MigrateRoomTagPullRoomList                         bool
	MigrateRoomTagPullChooseContact                    bool
	MigrateRoomTagPullStore                            bool
	MigrateRoomTagPullFilterContact                    bool
	MigrateRoomTagPullRemindSend                       bool
	MigrateRoomTagPullDestroy                          bool
	MigrateContactMessageBatchSendIndex                bool
	MigrateContactMessageBatchSendShow                 bool
	MigrateContactMessageBatchSendShowRoom             bool
	MigrateContactMessageBatchSendEmployee             bool
	MigrateContactMessageBatchSendContactReceive       bool
	MigrateContactMessageBatchSendStore                bool
	MigrateContactMessageBatchSendRemind               bool
	MigrateContactMessageBatchSendDestroy              bool
	MigrateRoomMessageBatchSendIndex                   bool
	MigrateRoomMessageBatchSendShow                    bool
	MigrateRoomMessageBatchSendOwner                   bool
	MigrateRoomMessageBatchSendRoomReceive             bool
	MigrateRoomMessageBatchSendStore                   bool
	MigrateRoomMessageBatchSendRemind                  bool
	MigrateRoomMessageBatchSendDestroy                 bool
	MigrateOfficialAccountIndex                        bool
	MigrateOfficialAccountSet                          bool
	MigrateOfficialAccountGetPreAuthURL                bool
	MigrateOfficialAccountAuthRedirect                 bool
	MigrateOfficialAccountAuthEventCallback            bool
	MigrateOfficialAccountMessageEventCallback         bool
	MigrateLoad                                        bool
	MigrateWorkFissionIndex                            bool
	MigrateWorkFissionShow                             bool
	MigrateWorkFissionInfo                             bool
	MigrateWorkFissionStatistics                       bool
	MigrateWorkFissionChooseContact                    bool
	MigrateWorkFissionStore                            bool
	MigrateWorkFissionUpdate                           bool
	MigrateWorkFissionInvite                           bool
	MigrateWorkFissionInviteData                       bool
	MigrateWorkFissionInviteDetail                     bool
	MigrateWorkFissionDestroy                          bool
	MigrateOperationWorkFissionInviteFriends           bool
	MigrateOperationWorkFissionPoster                  bool
	MigrateOperationWorkFissionTaskData                bool
	MigrateOperationWorkFissionReceive                 bool
	MigrateOperationWorkFissionAuth                    bool
	MigrateOperationWorkFissionOpenUserInfo            bool
	MigrateWorkRoomGroupIndex                          bool
	MigrateWorkRoomGroupStore                          bool
	MigrateWorkRoomGroupUpdate                         bool
	MigrateWorkRoomGroupDestroy                        bool
	MigrateSidebarContactTagAll                        bool
	MigrateSidebarContactDetail                        bool
	MigrateSidebarContactShow                          bool
	MigrateSidebarContactTrack                         bool
	MigrateSidebarContactUpdate                        bool
	MigrateSidebarProcessStatus                        bool
	MigrateSidebarProcessUpdate                        bool
	MigrateContactBatchAddDashboard                    bool
	MigrateSensitiveWordsDashboard                     bool
	MigrateContactSOPDashboard                         bool
	MigrateRoomSOPDashboard                            bool
	MigrateShopCodeDashboard                           bool
	MigrateRadarDashboard                              bool
	MigrateAutoTagDashboard                            bool
	MigrateLotteryDashboard                            bool
	MigrateRoomFissionDashboard                        bool
	MigrateRoomClockInDashboard                        bool
	MigrateRoomQualityDashboard                        bool
	MigrateRoomCalendarDashboard                       bool
	MigrateRoomRemindDashboard                         bool
	MigrateRoomInfinitePullDashboard                   bool
	MigrateSidebarContactBatchAddDetail                bool
	MigrateMediumIndex                                 bool
	MigrateMediumShow                                  bool
	MigrateMediumStore                                 bool
	MigrateMediumUpdate                                bool
	MigrateMediumDestroy                               bool
	MigrateMediumItemGroupUpdate                       bool
	MigrateSidebarMediumIndex                          bool
	MigrateSidebarMediumMediaIDUpdate                  bool
	MigrateMediumGroupIndex                            bool
	MigrateMediumGroupStore                            bool
	MigrateMediumGroupUpdate                           bool
	MigrateMediumGroupDestroy                          bool
	MigrateSidebarMediumGroupIndex                     bool
	MigrateFriendsCircleProvider                       bool
	MigrateChannelCodeIndex                            bool
	MigrateChannelCodeShow                             bool
	MigrateChannelCodeContact                          bool
	MigrateChannelCodeStatistics                       bool
	MigrateChannelCodeStatsIndex                       bool
	MigrateChannelCodeStore                            bool
	MigrateChannelCodeUpdate                           bool
	MigrateChannelCodeGroupIndex                       bool
	MigrateChannelCodeGroupDetail                      bool
	MigrateChannelCodeGroupStore                       bool
	MigrateChannelCodeGroupUpdate                      bool
	MigrateChannelCodeGroupMove                        bool
	MigrateGreetingIndex                               bool
	MigrateGreetingShow                                bool
	MigrateGreetingStore                               bool
	MigrateGreetingUpdate                              bool
	MigrateGreetingDestroy                             bool
	MigrateRoomWelcomeIndex                            bool
	MigrateRoomWelcomeSelect                           bool
	MigrateRoomWelcomeShow                             bool
	MigrateRoomWelcomeStore                            bool
	MigrateRoomWelcomeUpdate                           bool
	MigrateRoomWelcomeDestroy                          bool
	MigrateContactFieldIndex                           bool
	MigrateContactFieldShow                            bool
	MigrateContactFieldPortrait                        bool
	MigrateContactFieldStore                           bool
	MigrateContactFieldUpdate                          bool
	MigrateContactFieldStatus                          bool
	MigrateContactFieldDestroy                         bool
	MigrateContactFieldBatch                           bool
	MigrateContactFieldPivot                           bool
	MigrateContactFieldPivotUpdate                     bool
	MigrateSidebarFieldPivot                           bool
	MigrateSidebarFieldPivotUpdate                     bool
	MigrateSidebarContactSOPInfo                       bool
	MigrateSidebarContactSOPTipInfo                    bool
	MigrateSidebarRoomSOPInfo                          bool
	MigrateSidebarRoomSOPLogState                      bool
	MigrateChatToolConfig                              bool
	MigrateCommonUpload                                bool
	MigrateCommonUploadFile                            bool
	MigrateSidebarCommonUpload                         bool
	MigrateAgentTxtVerify                              bool
	MigrateAgentTxtUpload                              bool
	MigrateAgentStore                                  bool
	MigrateSidebarAgentAuth                            bool
	MigrateSidebarAgentOAuth                           bool
	MigrateSidebarAgentJSSDK                           bool
	MigrateSidebarWxJSSDK                              bool
	MigrateRoleSelect                                  bool
	MigrateRoleIndex                                   bool
	MigrateRoleShow                                    bool
	MigrateRolePermission                              bool
	MigrateRoleShowEmployee                            bool
	MigrateRoleStore                                   bool
	MigrateRoleUpdate                                  bool
	MigrateRoleStatusUpdate                            bool
	MigrateRoleDestroy                                 bool
	MigrateRolePermissionStore                         bool
	MigrateMenuIconIndex                               bool
	MigrateMenuSelect                                  bool
	MigrateMenuIndex                                   bool
	MigrateMenuShow                                    bool
	MigrateMenuStore                                   bool
	MigrateMenuUpdate                                  bool
	MigrateMenuStatusUpdate                            bool
	MigrateMenuDestroy                                 bool
	EnableWeWorkCallbackWorker                         bool
	EnableEmployeeApplyWorker                          bool
	EnableAsyncFileUploadWorker                        bool
	EnableMarkTagsWorker                               bool
	EnableMessageRemindWorker                          bool
	EnableWorkRoomSyncWorker                           bool
	EnableWorkContactSyncWorker                        bool
	EnableWorkDepartmentListWorker                     bool
	EnableMediaIDUpdateWorker                          bool
	EnableEmployeeStatisticWorker                      bool
	DevAuthHeader                                      bool
	SkipJWTBlacklist                                   bool
	ProxyTimeout                                       time.Duration
}

func FromEnv() (Config, error) {
	runtimeRole, err := appruntime.ParseRole(os.Getenv("MOCHAT_GO_RUNTIME_ROLE"))
	if err != nil {
		return Config{}, err
	}
	standalone := envBool("MOCHAT_GO_STANDALONE")
	timezone := envOrDefault("MOCHAT_TIMEZONE", "Asia/Shanghai")
	if timezone != "Asia/Shanghai" {
		return Config{}, fmt.Errorf("MOCHAT_TIMEZONE currently supports only Asia/Shanghai")
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return Config{}, fmt.Errorf("MOCHAT_TIMEZONE: %w", err)
	}
	redisDB, err := envInt("MOCHAT_REDIS_DB", "REDIS_DB", 0)
	if err != nil {
		return Config{}, err
	}
	ttl, err := envInt("MOCHAT_SIMPLE_JWT_TTL", "SIMPLE_JWT_TTL", 60*60*24*7)
	if err != nil {
		return Config{}, err
	}
	if ttl <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_SIMPLE_JWT_TTL or SIMPLE_JWT_TTL must be a positive integer")
	}
	refreshTTL, err := envInt("MOCHAT_SIMPLE_JWT_REFRESH_TTL", "SIMPLE_JWT_REFRESH_TTL", 60*60*24*7)
	if err != nil {
		return Config{}, err
	}
	if refreshTTL <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_SIMPLE_JWT_REFRESH_TTL or SIMPLE_JWT_REFRESH_TTL must be a positive integer")
	}
	workerProcessingTimeout, err := envInt("MOCHAT_GO_WORKER_PROCESSING_TIMEOUT_SECONDS", "", 300)
	if err != nil {
		return Config{}, err
	}
	if workerProcessingTimeout <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_WORKER_PROCESSING_TIMEOUT_SECONDS must be a positive integer")
	}
	pullAgentCronInterval, err := envInt("MOCHAT_GO_PULL_AGENT_CRON_INTERVAL_SECONDS", "", 3600)
	if err != nil {
		return Config{}, err
	}
	if pullAgentCronInterval <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_PULL_AGENT_CRON_INTERVAL_SECONDS must be a positive integer")
	}
	employeeStatisticCronInterval, err := envInt("MOCHAT_GO_EMPLOYEE_STATISTIC_CRON_INTERVAL_SECONDS", "", 86400)
	if err != nil {
		return Config{}, err
	}
	if employeeStatisticCronInterval <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_EMPLOYEE_STATISTIC_CRON_INTERVAL_SECONDS must be a positive integer")
	}
	channelCodeCronInterval, err := envInt("MOCHAT_GO_CHANNEL_CODE_CRON_INTERVAL_SECONDS", "", 30)
	if err != nil {
		return Config{}, err
	}
	if channelCodeCronInterval <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_CHANNEL_CODE_CRON_INTERVAL_SECONDS must be a positive integer")
	}
	contactBatchSendCronInterval, err := envInt("MOCHAT_GO_CONTACT_BATCH_SEND_CRON_INTERVAL_SECONDS", "", 60)
	if err != nil {
		return Config{}, err
	}
	if contactBatchSendCronInterval <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_CONTACT_BATCH_SEND_CRON_INTERVAL_SECONDS must be a positive integer")
	}
	roomBatchSendCronInterval, err := envInt("MOCHAT_GO_ROOM_BATCH_SEND_CRON_INTERVAL_SECONDS", "", 60)
	if err != nil {
		return Config{}, err
	}
	if roomBatchSendCronInterval <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_ROOM_BATCH_SEND_CRON_INTERVAL_SECONDS must be a positive integer")
	}
	contactSyncSendResultCronInterval, err := envInt("MOCHAT_GO_CONTACT_SYNC_SEND_RESULT_CRON_INTERVAL_SECONDS", "", 3600)
	if err != nil {
		return Config{}, err
	}
	if contactSyncSendResultCronInterval <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_CONTACT_SYNC_SEND_RESULT_CRON_INTERVAL_SECONDS must be a positive integer")
	}
	roomSyncSendResultCronInterval, err := envInt("MOCHAT_GO_ROOM_SYNC_SEND_RESULT_CRON_INTERVAL_SECONDS", "", 3600)
	if err != nil {
		return Config{}, err
	}
	if roomSyncSendResultCronInterval <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_ROOM_SYNC_SEND_RESULT_CRON_INTERVAL_SECONDS must be a positive integer")
	}
	roomTagPullCronInterval, err := envInt("MOCHAT_GO_ROOM_TAG_PULL_CRON_INTERVAL_SECONDS", "", 300)
	if err != nil {
		return Config{}, err
	}
	if roomTagPullCronInterval <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_ROOM_TAG_PULL_CRON_INTERVAL_SECONDS must be a positive integer")
	}
	corpDataCronInterval, err := envInt("MOCHAT_GO_CORP_DATA_CRON_INTERVAL_SECONDS", "", 600)
	if err != nil {
		return Config{}, err
	}
	if corpDataCronInterval <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_CORP_DATA_CRON_INTERVAL_SECONDS must be a positive integer")
	}
	mediaIDUpdateCronInterval, err := envInt("MOCHAT_GO_MEDIA_ID_UPDATE_CRON_INTERVAL_SECONDS", "", 300)
	if err != nil {
		return Config{}, err
	}
	if mediaIDUpdateCronInterval <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_MEDIA_ID_UPDATE_CRON_INTERVAL_SECONDS must be a positive integer")
	}
	transferStateRefreshCronInterval, err := envInt("MOCHAT_GO_TRANSFER_STATE_REFRESH_CRON_INTERVAL_SECONDS", "", 300)
	if err != nil {
		return Config{}, err
	}
	if transferStateRefreshCronInterval <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_TRANSFER_STATE_REFRESH_CRON_INTERVAL_SECONDS must be a positive integer")
	}
	sopLogCronInterval, err := envInt("MOCHAT_GO_SOP_LOG_CRON_INTERVAL_SECONDS", "", 300)
	if err != nil {
		return Config{}, err
	}
	if sopLogCronInterval <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SOP_LOG_CRON_INTERVAL_SECONDS must be a positive integer")
	}
	sensitiveWordMonitorCronInterval, err := envInt("MOCHAT_GO_SENSITIVE_WORD_MONITOR_CRON_INTERVAL_SECONDS", "", 60)
	if err != nil {
		return Config{}, err
	}
	if sensitiveWordMonitorCronInterval <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SENSITIVE_WORD_MONITOR_CRON_INTERVAL_SECONDS must be a positive integer")
	}
	workMessageArchiveSyncCronInterval, err := envInt("MOCHAT_GO_WORK_MESSAGE_ARCHIVE_SYNC_CRON_INTERVAL_SECONDS", "", 60)
	if err != nil {
		return Config{}, err
	}
	if workMessageArchiveSyncCronInterval <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_WORK_MESSAGE_ARCHIVE_SYNC_CRON_INTERVAL_SECONDS must be a positive integer")
	}
	workMessageArchiveSyncLimit, err := envInt("MOCHAT_GO_WORK_MESSAGE_ARCHIVE_SYNC_LIMIT", "", 100)
	if err != nil {
		return Config{}, err
	}
	if workMessageArchiveSyncLimit <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_WORK_MESSAGE_ARCHIVE_SYNC_LIMIT must be a positive integer")
	}
	saasStorageReconcileCronInterval, err := envInt("MOCHAT_GO_SAAS_STORAGE_RECONCILE_CRON_INTERVAL_SECONDS", "", 86400)
	if err != nil {
		return Config{}, err
	}
	if saasStorageReconcileCronInterval <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_STORAGE_RECONCILE_CRON_INTERVAL_SECONDS must be a positive integer")
	}
	saasAlertNotificationDispatchCronInterval, err := envInt("MOCHAT_GO_SAAS_ALERT_NOTIFICATION_DISPATCH_CRON_INTERVAL_SECONDS", "", 300)
	if err != nil {
		return Config{}, err
	}
	if saasAlertNotificationDispatchCronInterval <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_ALERT_NOTIFICATION_DISPATCH_CRON_INTERVAL_SECONDS must be a positive integer")
	}
	saasAlertNotificationDispatchLimit, err := envInt("MOCHAT_GO_SAAS_ALERT_NOTIFICATION_DISPATCH_LIMIT", "", 100)
	if err != nil {
		return Config{}, err
	}
	if saasAlertNotificationDispatchLimit <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_ALERT_NOTIFICATION_DISPATCH_LIMIT must be a positive integer")
	}
	saasOperationQueueAssignmentReminderCronInterval, err := envInt("MOCHAT_GO_SAAS_OPERATION_QUEUE_ASSIGNMENT_REMINDER_CRON_INTERVAL_SECONDS", "", 3600)
	if err != nil {
		return Config{}, err
	}
	if saasOperationQueueAssignmentReminderCronInterval <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_OPERATION_QUEUE_ASSIGNMENT_REMINDER_CRON_INTERVAL_SECONDS must be a positive integer")
	}
	saasOperationQueueAssignmentReminderLimit, err := envInt("MOCHAT_GO_SAAS_OPERATION_QUEUE_ASSIGNMENT_REMINDER_LIMIT", "", 500)
	if err != nil {
		return Config{}, err
	}
	if saasOperationQueueAssignmentReminderLimit <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_OPERATION_QUEUE_ASSIGNMENT_REMINDER_LIMIT must be a positive integer")
	}
	saasApprovalReminderCronInterval, err := envInt("MOCHAT_GO_SAAS_APPROVAL_REMINDER_CRON_INTERVAL_SECONDS", "", 300)
	if err != nil {
		return Config{}, err
	}
	if saasApprovalReminderCronInterval <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_APPROVAL_REMINDER_CRON_INTERVAL_SECONDS must be a positive integer")
	}
	saasApprovalReminderLimit, err := envInt("MOCHAT_GO_SAAS_APPROVAL_REMINDER_LIMIT", "", 100)
	if err != nil {
		return Config{}, err
	}
	if saasApprovalReminderLimit <= 0 || saasApprovalReminderLimit > 500 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_APPROVAL_REMINDER_LIMIT must be between 1 and 500")
	}
	saasSystemHealthCronInterval, err := envInt("MOCHAT_GO_SAAS_SYSTEM_HEALTH_CRON_INTERVAL_SECONDS", "", 300)
	if err != nil {
		return Config{}, err
	}
	if saasSystemHealthCronInterval <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_SYSTEM_HEALTH_CRON_INTERVAL_SECONDS must be a positive integer")
	}
	saasSystemHealthFailureWindowHours, err := envInt("MOCHAT_GO_SAAS_SYSTEM_HEALTH_FAILURE_WINDOW_HOURS", "", 24)
	if err != nil {
		return Config{}, err
	}
	if saasSystemHealthFailureWindowHours <= 0 || saasSystemHealthFailureWindowHours > 720 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_SYSTEM_HEALTH_FAILURE_WINDOW_HOURS must be between 1 and 720")
	}
	saasSystemHealthNotificationStaleMinutes, err := envInt("MOCHAT_GO_SAAS_SYSTEM_HEALTH_NOTIFICATION_STALE_MINUTES", "", 15)
	if err != nil {
		return Config{}, err
	}
	if saasSystemHealthNotificationStaleMinutes <= 0 || saasSystemHealthNotificationStaleMinutes > 10080 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_SYSTEM_HEALTH_NOTIFICATION_STALE_MINUTES must be between 1 and 10080")
	}
	saasBackupCronInterval, err := envInt("MOCHAT_GO_SAAS_BACKUP_CRON_INTERVAL_SECONDS", "", 300)
	if err != nil {
		return Config{}, err
	}
	if saasBackupCronInterval <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_BACKUP_CRON_INTERVAL_SECONDS must be a positive integer")
	}
	saasComplianceCronInterval, err := envInt("MOCHAT_GO_SAAS_COMPLIANCE_CRON_INTERVAL_SECONDS", "", 300)
	if err != nil {
		return Config{}, err
	}
	if saasComplianceCronInterval <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_COMPLIANCE_CRON_INTERVAL_SECONDS must be a positive integer")
	}
	saasIdentityCleanupCronInterval, err := envInt("MOCHAT_GO_SAAS_IDENTITY_CLEANUP_CRON_INTERVAL_SECONDS", "", 3600)
	if err != nil {
		return Config{}, err
	}
	if saasIdentityCleanupCronInterval <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_IDENTITY_CLEANUP_CRON_INTERVAL_SECONDS must be a positive integer")
	}
	saasIdentityCleanupLimit, err := envInt("MOCHAT_GO_SAAS_IDENTITY_CLEANUP_LIMIT", "", 1000)
	if err != nil {
		return Config{}, err
	}
	if saasIdentityCleanupLimit <= 0 || saasIdentityCleanupLimit > 5000 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_IDENTITY_CLEANUP_LIMIT must be between 1 and 5000")
	}
	saasServiceAccountUsageAlertCronInterval, err := envInt("MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_ALERT_CRON_INTERVAL_SECONDS", "", 300)
	if err != nil {
		return Config{}, err
	}
	if saasServiceAccountUsageAlertCronInterval <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_ALERT_CRON_INTERVAL_SECONDS must be a positive integer")
	}
	saasServiceAccountUsageAlertLimit, err := envInt("MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_ALERT_LIMIT", "", 100)
	if err != nil {
		return Config{}, err
	}
	if saasServiceAccountUsageAlertLimit <= 0 || saasServiceAccountUsageAlertLimit > 500 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_ALERT_LIMIT must be between 1 and 500")
	}
	saasAuditIntegrityCronInterval, err := envInt("MOCHAT_GO_SAAS_AUDIT_INTEGRITY_CRON_INTERVAL_SECONDS", "", 3600)
	if err != nil {
		return Config{}, err
	}
	if saasAuditIntegrityCronInterval <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_AUDIT_INTEGRITY_CRON_INTERVAL_SECONDS must be a positive integer")
	}
	saasAuditIntegrityLimit, err := envInt("MOCHAT_GO_SAAS_AUDIT_INTEGRITY_LIMIT", "", 100)
	if err != nil {
		return Config{}, err
	}
	if saasAuditIntegrityLimit <= 0 || saasAuditIntegrityLimit > 500 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_AUDIT_INTEGRITY_LIMIT must be between 1 and 500")
	}
	saasAuditAnchorCronInterval, err := envInt("MOCHAT_GO_SAAS_AUDIT_ANCHOR_CRON_INTERVAL_SECONDS", "", 86400)
	if err != nil {
		return Config{}, err
	}
	if saasAuditAnchorCronInterval <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_AUDIT_ANCHOR_CRON_INTERVAL_SECONDS must be a positive integer")
	}
	saasAuditAnchorLimit, err := envInt("MOCHAT_GO_SAAS_AUDIT_ANCHOR_LIMIT", "", 100)
	if err != nil {
		return Config{}, err
	}
	if saasAuditAnchorLimit <= 0 || saasAuditAnchorLimit > 500 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_AUDIT_ANCHOR_LIMIT must be between 1 and 500")
	}
	saasAuditAnchorS3RetentionDays, err := envInt("MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_RETENTION_DAYS", "", 3650)
	if err != nil {
		return Config{}, err
	}
	if saasAuditAnchorS3RetentionDays < 1 || saasAuditAnchorS3RetentionDays > 36500 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_RETENTION_DAYS must be between 1 and 36500")
	}
	saasServiceAccountUsageCleanupCronInterval, err := envInt("MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_CLEANUP_CRON_INTERVAL_SECONDS", "", 86400)
	if err != nil {
		return Config{}, err
	}
	if saasServiceAccountUsageCleanupCronInterval <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_CLEANUP_CRON_INTERVAL_SECONDS must be a positive integer")
	}
	saasServiceAccountUsageCleanupLimit, err := envInt("MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_CLEANUP_LIMIT", "", 10000)
	if err != nil {
		return Config{}, err
	}
	if saasServiceAccountUsageCleanupLimit <= 0 || saasServiceAccountUsageCleanupLimit > 1000000 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_CLEANUP_LIMIT must be between 1 and 1000000")
	}
	saasTenantDomainDNSTimeout, err := envInt("MOCHAT_GO_SAAS_TENANT_DOMAIN_DNS_TIMEOUT_SECONDS", "", 5)
	if err != nil {
		return Config{}, err
	}
	if saasTenantDomainDNSTimeout <= 0 || saasTenantDomainDNSTimeout > 30 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_TENANT_DOMAIN_DNS_TIMEOUT_SECONDS must be between 1 and 30")
	}
	saasTenantDomainDeliveryCronInterval, err := envInt("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CRON_INTERVAL_SECONDS", "", 60)
	if err != nil {
		return Config{}, err
	}
	if saasTenantDomainDeliveryCronInterval <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CRON_INTERVAL_SECONDS must be a positive integer")
	}
	saasTenantDomainDeliveryBridgeTimeout, err := envInt("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_BRIDGE_TIMEOUT_SECONDS", "", 10)
	if err != nil {
		return Config{}, err
	}
	if saasTenantDomainDeliveryBridgeTimeout <= 0 || saasTenantDomainDeliveryBridgeTimeout > 120 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_BRIDGE_TIMEOUT_SECONDS must be between 1 and 120")
	}
	saasTenantDomainDeliveryCallbackTolerance, err := envInt("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CALLBACK_TOLERANCE_SECONDS", "", 300)
	if err != nil {
		return Config{}, err
	}
	if saasTenantDomainDeliveryCallbackTolerance < 30 || saasTenantDomainDeliveryCallbackTolerance > 3600 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CALLBACK_TOLERANCE_SECONDS must be between 30 and 3600")
	}
	saasTenantDomainDeliveryLimit, err := envInt("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_LIMIT", "", 20)
	if err != nil {
		return Config{}, err
	}
	if saasTenantDomainDeliveryLimit <= 0 || saasTenantDomainDeliveryLimit > 100 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_LIMIT must be between 1 and 100")
	}
	saasTenantDomainDeliveryLease, err := envInt("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_LEASE_SECONDS", "", 120)
	if err != nil {
		return Config{}, err
	}
	if saasTenantDomainDeliveryLease < 10 || saasTenantDomainDeliveryLease > 3600 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_LEASE_SECONDS must be between 10 and 3600")
	}
	saasTenantDomainDeliveryRetryDelay, err := envInt("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_RETRY_DELAY_SECONDS", "", 60)
	if err != nil {
		return Config{}, err
	}
	if saasTenantDomainDeliveryRetryDelay < 1 || saasTenantDomainDeliveryRetryDelay > 86400 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_RETRY_DELAY_SECONDS must be between 1 and 86400")
	}
	saasTenantDomainDeliveryCallbackWait, err := envInt("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CALLBACK_WAIT_SECONDS", "", 600)
	if err != nil {
		return Config{}, err
	}
	if saasTenantDomainDeliveryCallbackWait < 10 || saasTenantDomainDeliveryCallbackWait > 86400 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CALLBACK_WAIT_SECONDS must be between 10 and 86400")
	}
	saasTenantDomainCertificateExpiryWarningDays, err := envInt("MOCHAT_GO_SAAS_TENANT_DOMAIN_CERTIFICATE_EXPIRY_WARNING_DAYS", "", 30)
	if err != nil {
		return Config{}, err
	}
	if saasTenantDomainCertificateExpiryWarningDays < 1 || saasTenantDomainCertificateExpiryWarningDays > 180 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_TENANT_DOMAIN_CERTIFICATE_EXPIRY_WARNING_DAYS must be between 1 and 180")
	}
	saasNotificationHealthRecoveryCronInterval, err := envInt("MOCHAT_GO_SAAS_NOTIFICATION_HEALTH_RECOVERY_CRON_INTERVAL_SECONDS", "", 900)
	if err != nil {
		return Config{}, err
	}
	if saasNotificationHealthRecoveryCronInterval <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_NOTIFICATION_HEALTH_RECOVERY_CRON_INTERVAL_SECONDS must be a positive integer")
	}
	saasNotificationHealthRecoveryWindowHours, err := envInt("MOCHAT_GO_SAAS_NOTIFICATION_HEALTH_RECOVERY_WINDOW_HOURS", "", 24)
	if err != nil {
		return Config{}, err
	}
	if saasNotificationHealthRecoveryWindowHours <= 0 || saasNotificationHealthRecoveryWindowHours > 720 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_NOTIFICATION_HEALTH_RECOVERY_WINDOW_HOURS must be between 1 and 720")
	}
	saasNotificationHealthRecoveryStaleMinutes, err := envInt("MOCHAT_GO_SAAS_NOTIFICATION_HEALTH_RECOVERY_STALE_MINUTES", "", 15)
	if err != nil {
		return Config{}, err
	}
	if saasNotificationHealthRecoveryStaleMinutes <= 0 || saasNotificationHealthRecoveryStaleMinutes > 10080 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_NOTIFICATION_HEALTH_RECOVERY_STALE_MINUTES must be between 1 and 10080")
	}
	saasSubscriptionReconcileCronInterval, err := envInt("MOCHAT_GO_SAAS_SUBSCRIPTION_RECONCILE_CRON_INTERVAL_SECONDS", "", 300)
	if err != nil {
		return Config{}, err
	}
	if saasSubscriptionReconcileCronInterval <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_SUBSCRIPTION_RECONCILE_CRON_INTERVAL_SECONDS must be a positive integer")
	}
	saasSubscriptionReconcileLimit, err := envInt("MOCHAT_GO_SAAS_SUBSCRIPTION_RECONCILE_LIMIT", "", 500)
	if err != nil {
		return Config{}, err
	}
	if saasSubscriptionReconcileLimit <= 0 || saasSubscriptionReconcileLimit > 5000 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_SUBSCRIPTION_RECONCILE_LIMIT must be between 1 and 5000")
	}
	saasPaymentWebhookToleranceSeconds, err := envInt("MOCHAT_GO_SAAS_PAYMENT_WEBHOOK_TOLERANCE_SECONDS", "", 300)
	if err != nil {
		return Config{}, err
	}
	if saasPaymentWebhookToleranceSeconds < 30 || saasPaymentWebhookToleranceSeconds > 3600 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_PAYMENT_WEBHOOK_TOLERANCE_SECONDS must be between 30 and 3600")
	}
	saasPaymentDunningCronInterval, err := envInt("MOCHAT_GO_SAAS_PAYMENT_DUNNING_CRON_INTERVAL_SECONDS", "", 300)
	if err != nil {
		return Config{}, err
	}
	if saasPaymentDunningCronInterval <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_PAYMENT_DUNNING_CRON_INTERVAL_SECONDS must be a positive integer")
	}
	saasPaymentDunningLimit, err := envInt("MOCHAT_GO_SAAS_PAYMENT_DUNNING_LIMIT", "", 100)
	if err != nil {
		return Config{}, err
	}
	if saasPaymentDunningLimit <= 0 || saasPaymentDunningLimit > 5000 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_PAYMENT_DUNNING_LIMIT must be between 1 and 5000")
	}
	saasPaymentDunningRetryDelaySeconds, err := envInt("MOCHAT_GO_SAAS_PAYMENT_DUNNING_RETRY_DELAY_SECONDS", "", 86400)
	if err != nil {
		return Config{}, err
	}
	if saasPaymentDunningRetryDelaySeconds < 60 || saasPaymentDunningRetryDelaySeconds > 2592000 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_PAYMENT_DUNNING_RETRY_DELAY_SECONDS must be between 60 and 2592000")
	}
	saasPaymentSettlementSyncCronInterval, err := envInt("MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_SYNC_CRON_INTERVAL_SECONDS", "", 900)
	if err != nil {
		return Config{}, err
	}
	if saasPaymentSettlementSyncCronInterval <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_SYNC_CRON_INTERVAL_SECONDS must be a positive integer")
	}
	saasPaymentSettlementSyncLimit, err := envInt("MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_SYNC_LIMIT", "", 100)
	if err != nil {
		return Config{}, err
	}
	if saasPaymentSettlementSyncLimit <= 0 || saasPaymentSettlementSyncLimit > 5000 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_SYNC_LIMIT must be between 1 and 5000")
	}
	saasPaymentSettlementBridgeTimeoutSeconds, err := envInt("MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_BRIDGE_TIMEOUT_SECONDS", "", 10)
	if err != nil {
		return Config{}, err
	}
	if saasPaymentSettlementBridgeTimeoutSeconds <= 0 || saasPaymentSettlementBridgeTimeoutSeconds > 120 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_BRIDGE_TIMEOUT_SECONDS must be between 1 and 120")
	}
	saasPaymentSettlementProviders, err := paymentSettlementProvidersFromEnv(os.Getenv("MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_PROVIDERS"))
	if err != nil {
		return Config{}, err
	}
	saasAlertWebhookTimeout, err := envInt("MOCHAT_GO_SAAS_ALERT_WEBHOOK_TIMEOUT_SECONDS", "", 5)
	if err != nil {
		return Config{}, err
	}
	if saasAlertWebhookTimeout <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_ALERT_WEBHOOK_TIMEOUT_SECONDS must be a positive integer")
	}
	saasAlertWebhookRetryAttempts, err := envInt("MOCHAT_GO_SAAS_ALERT_WEBHOOK_RETRY_ATTEMPTS", "", 1)
	if err != nil {
		return Config{}, err
	}
	if saasAlertWebhookRetryAttempts <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_ALERT_WEBHOOK_RETRY_ATTEMPTS must be a positive integer")
	}
	saasAlertWebhookRetryDelayMS, err := envInt("MOCHAT_GO_SAAS_ALERT_WEBHOOK_RETRY_DELAY_MS", "", 250)
	if err != nil {
		return Config{}, err
	}
	saasAlertWebhookTitleTemplate := envOrDefault("MOCHAT_GO_SAAS_ALERT_WEBHOOK_TITLE_TEMPLATE", DefaultSaaSAlertWebhookTitleTemplate)
	if err := validateSaaSAlertWebhookTemplate("MOCHAT_GO_SAAS_ALERT_WEBHOOK_TITLE_TEMPLATE", saasAlertWebhookTitleTemplate); err != nil {
		return Config{}, err
	}
	saasAlertWebhookBodyTemplate := envOrDefault("MOCHAT_GO_SAAS_ALERT_WEBHOOK_BODY_TEMPLATE", DefaultSaaSAlertWebhookBodyTemplate)
	if err := validateSaaSAlertWebhookTemplate("MOCHAT_GO_SAAS_ALERT_WEBHOOK_BODY_TEMPLATE", saasAlertWebhookBodyTemplate); err != nil {
		return Config{}, err
	}
	saasAlertWebhookAllowedCIDRs, err := outboundhttp.NormalizeCIDRs([]string{os.Getenv("MOCHAT_GO_SAAS_ALERT_WEBHOOK_ALLOWED_CIDRS")})
	if err != nil {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_ALERT_WEBHOOK_ALLOWED_CIDRS: %w", err)
	}
	saasAlertWebhookRequireHTTPS := envBoolDefault("MOCHAT_GO_SAAS_ALERT_WEBHOOK_REQUIRE_HTTPS", true)
	saasAlertNotificationMaxAttempts, err := envInt("MOCHAT_GO_SAAS_ALERT_NOTIFICATION_MAX_ATTEMPTS", "", 3)
	if err != nil {
		return Config{}, err
	}
	if saasAlertNotificationMaxAttempts <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_ALERT_NOTIFICATION_MAX_ATTEMPTS must be a positive integer")
	}
	saasAlertNotificationRetryDelaySeconds, err := envInt("MOCHAT_GO_SAAS_ALERT_NOTIFICATION_RETRY_DELAY_SECONDS", "", 300)
	if err != nil {
		return Config{}, err
	}
	saasPlatformAdminTenantID, err := envInt("MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID", "", 1)
	if err != nil {
		return Config{}, err
	}
	saasReleaseEvidenceVerifyTimeoutSeconds, err := envInt("MOCHAT_GO_SAAS_RELEASE_EVIDENCE_VERIFY_TIMEOUT_SECONDS", "", 30)
	if err != nil {
		return Config{}, err
	}
	if saasReleaseEvidenceVerifyTimeoutSeconds <= 0 || saasReleaseEvidenceVerifyTimeoutSeconds > 300 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_RELEASE_EVIDENCE_VERIFY_TIMEOUT_SECONDS must be between 1 and 300")
	}
	saasReleaseEvidenceMaxBytes, err := envInt("MOCHAT_GO_SAAS_RELEASE_EVIDENCE_MAX_BYTES", "", 64<<20)
	if err != nil {
		return Config{}, err
	}
	if saasReleaseEvidenceMaxBytes <= 0 || int64(saasReleaseEvidenceMaxBytes) > int64(1<<30) {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_RELEASE_EVIDENCE_MAX_BYTES must be between 1 and 1073741824")
	}
	saasReleaseEvidenceAllowedCIDRs, err := outboundhttp.NormalizeCIDRs([]string{os.Getenv("MOCHAT_GO_SAAS_RELEASE_EVIDENCE_ALLOWED_CIDRS")})
	if err != nil {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_RELEASE_EVIDENCE_ALLOWED_CIDRS: %w", err)
	}

	listenAddr := envOrDefault("MOCHAT_GO_ADDR", ":8080")
	defaultPHPUpstream := ""
	defaultBaseURL := baseURLFromListenAddr(listenAddr)
	if defaultBaseURL == "" {
		defaultBaseURL = "http://127.0.0.1:8080"
	}
	if upstream := strings.TrimSpace(os.Getenv("MOCHAT_PHP_UPSTREAM")); upstream != "" && !standalone {
		defaultPHPUpstream = upstream
		defaultBaseURL = upstream
	}
	defaultAPIBaseURL := defaultBaseURL
	defaultDashboardBaseURL := defaultBaseURL
	defaultSidebarBaseURL := defaultBaseURL
	defaultOperationBaseURL := defaultBaseURL
	defaultFileStorageRoot := "../mochat/api-server/storage/upload/static"
	defaultDashboardDist := "./web/apps/dashboard/dist"
	defaultSaaSAdminDist := "./web/apps/saas-admin/dist"
	defaultSidebarDist := "./web/apps/sidebar/dist"
	defaultOperationDist := "./web/apps/operation/dist"
	defaultSidebarFrontendAddr := ""
	defaultOperationFrontendAddr := ""
	defaultSourceRoot := "../mochat"
	// The compatibility manifest is embedded in the Go server binary. Keep the
	// default independent from the source-tree docs directory so production
	// images do not need to ship docs just to serve route metadata.
	defaultManifestPath := ""
	enableFrontendServers := envBool("MOCHAT_GO_ENABLE_FRONTEND_SERVERS")
	if standalone {
		defaultPHPUpstream = ""
		defaultAPIBaseURL = defaultBaseURL
		defaultDashboardBaseURL = defaultBaseURL
		defaultSidebarBaseURL = defaultDashboardBaseURL
		defaultOperationBaseURL = defaultDashboardBaseURL
		if enableFrontendServers {
			defaultSidebarFrontendAddr = offsetListenAddr(listenAddr, 1)
			defaultOperationFrontendAddr = offsetListenAddr(listenAddr, 2)
		}
		defaultFileStorageRoot = "./storage/upload/static"
		defaultSourceRoot = ""
		defaultManifestPath = ""
	}
	sidebarFrontendAddr := envOrDefault("MOCHAT_SIDEBAR_FRONTEND_ADDR", defaultSidebarFrontendAddr)
	operationFrontendAddr := envOrDefault("MOCHAT_OPERATION_FRONTEND_ADDR", defaultOperationFrontendAddr)
	if standalone {
		if baseURL := baseURLFromListenAddr(sidebarFrontendAddr); baseURL != "" {
			defaultSidebarBaseURL = baseURL
		}
		if baseURL := baseURLFromListenAddr(operationFrontendAddr); baseURL != "" {
			defaultOperationBaseURL = baseURL
		}
	}
	enableAllMigratedRoutes := envBoolDefault("MOCHAT_GO_ENABLE_ALL_MIGRATED_ROUTES", standalone && strings.TrimSpace(os.Getenv("MOCHAT_MYSQL_DSN")) != "" && strings.TrimSpace(envFirst("MOCHAT_SIMPLE_JWT_SECRET", "SIMPLE_JWT_SECRET")) != "")
	phpUpstream := envOrDefault("MOCHAT_PHP_UPSTREAM", defaultPHPUpstream)
	sourceRoot := envOrDefault("MOCHAT_SOURCE_ROOT", defaultSourceRoot)
	manifestPath := envOrDefault("MOCHAT_COMPAT_MANIFEST", defaultManifestPath)
	if standalone {
		phpUpstream = ""
		sourceRoot = ""
		manifestPath = ""
	}
	releaseSourceFingerprint := strings.ToLower(strings.TrimSpace(buildinfo.SourceFingerprint))
	releaseSourceFingerprintSource := "build"
	if releaseSourceFingerprint == "" {
		releaseSourceFingerprint = strings.ToLower(strings.TrimSpace(os.Getenv("MOCHAT_GO_RELEASE_SOURCE_FINGERPRINT")))
		releaseSourceFingerprintSource = "environment"
	}
	if releaseSourceFingerprint == "" {
		releaseSourceFingerprintSource = ""
	}
	identityTrustProxyHeaders := envBool("MOCHAT_GO_SAAS_IDENTITY_TRUST_PROXY_HEADERS")
	serviceAccountTrustProxyHeaders := envBool("MOCHAT_GO_SAAS_SERVICE_ACCOUNT_TRUST_PROXY_HEADERS")
	trustedProxyCIDRs, err := clientip.NormalizeCIDRs([]string{os.Getenv("MOCHAT_GO_SAAS_TRUSTED_PROXY_CIDRS")})
	if err != nil {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_TRUSTED_PROXY_CIDRS: %w", err)
	}
	alertCredentialEncryption, alertCredentialDedicatedConfigured := saasalertcredentials.SelectEncryptionSource(
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
	weComCredentialEncryption, weComCredentialDedicatedConfigured := wecomcredentials.SelectEncryptionSource(
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
	weChatOpenCredentialEncryption, weChatOpenCredentialDedicatedConfigured := wechatopencredentials.SelectEncryptionSource(
		wechatopencredentials.EncryptionSource{
			Key:   os.Getenv("MOCHAT_GO_WECHAT_OPEN_CREDENTIAL_ENCRYPTION_KEY"),
			Keys:  os.Getenv("MOCHAT_GO_WECHAT_OPEN_CREDENTIAL_ENCRYPTION_KEYS"),
			KeyID: os.Getenv("MOCHAT_GO_WECHAT_OPEN_CREDENTIAL_ENCRYPTION_KEY_ID"),
		},
		wechatopencredentials.EncryptionSource{
			Key:   os.Getenv("MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY"),
			Keys:  os.Getenv("MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEYS"),
			KeyID: os.Getenv("MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY_ID"),
		},
		wechatopencredentials.EncryptionSource{
			Key:   os.Getenv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY"),
			Keys:  os.Getenv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEYS"),
			KeyID: os.Getenv("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY_ID"),
		},
		wechatopencredentials.EncryptionSource{
			Key:   os.Getenv("MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY"),
			Keys:  os.Getenv("MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEYS"),
			KeyID: os.Getenv("MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY_ID"),
		},
		wechatopencredentials.EncryptionSource{
			Key:   os.Getenv("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY"),
			Keys:  os.Getenv("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEYS"),
			KeyID: os.Getenv("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY_ID"),
		},
	)

	cfg := Config{
		RuntimeRole:                                        runtimeRole,
		ListenAddr:                                         listenAddr,
		Timezone:                                           timezone,
		Standalone:                                         standalone,
		EnableAllMigratedRoutes:                            enableAllMigratedRoutes,
		EnablePhase22SCRMPilot:                             envBool("MOCHAT_GO_ENABLE_PHASE2_2_SCRM_PILOT"),
		PHPUpstream:                                        phpUpstream,
		APIBaseURL:                                         envOrDefault("MOCHAT_API_BASE_URL", envOrDefault("API_BASE_URL", defaultAPIBaseURL)),
		DashboardBaseURL:                                   envOrDefault("MOCHAT_DASHBOARD_BASE_URL", envOrDefault("DASHBOARD_BASE_URL", defaultDashboardBaseURL)),
		SidebarBaseURL:                                     envOrDefault("MOCHAT_SIDEBAR_BASE_URL", envOrDefault("SIDEBAR_BASE_URL", defaultSidebarBaseURL)),
		OperationBaseURL:                                   envOrDefault("MOCHAT_OPERATION_BASE_URL", envOrDefault("OPERATION_BASE_URL", defaultOperationBaseURL)),
		FileStorageRoot:                                    envOrDefault("MOCHAT_FILE_STORAGE_ROOT", envOrDefault("FILE_STORAGE_ROOT", defaultFileStorageRoot)),
		DashboardDist:                                      envOrDefault("MOCHAT_DASHBOARD_DIST", defaultDashboardDist),
		SaaSAdminDist:                                      envOrDefault("MOCHAT_SAAS_ADMIN_DIST", defaultSaaSAdminDist),
		SidebarDist:                                        envOrDefault("MOCHAT_SIDEBAR_DIST", defaultSidebarDist),
		OperationDist:                                      envOrDefault("MOCHAT_OPERATION_DIST", defaultOperationDist),
		SidebarFrontendAddr:                                sidebarFrontendAddr,
		OperationFrontendAddr:                              operationFrontendAddr,
		WeComAPIBaseURL:                                    envOrDefault("MOCHAT_WECOM_API_BASE_URL", "https://qyapi.weixin.qq.com"),
		WeChatAPIBaseURL:                                   envOrDefault("MOCHAT_WECHAT_API_BASE_URL", "https://api.weixin.qq.com"),
		WeChatOpenPlatformAppID:                            envFirst("MOCHAT_WECHAT_OPEN_PLATFORM_APP_ID", "WECHAT_OPEN_PLATFORM_APP_ID"),
		WeChatOpenPlatformSecret:                           envFirst("MOCHAT_WECHAT_OPEN_PLATFORM_SECRET", "WECHAT_OPEN_PLATFORM_SECRET"),
		WeChatOpenPlatformToken:                            envFirst("MOCHAT_WECHAT_OPEN_PLATFORM_TOKEN", "WECHAT_OPEN_PLATFORM_TOKEN"),
		WeChatOpenPlatformAESKey:                           envFirst("MOCHAT_WECHAT_OPEN_PLATFORM_AES_KEY", "WECHAT_OPEN_PLATFORM_AES_KEY"),
		WeChatComponentVerifyTicket:                        envFirst("MOCHAT_WECHAT_COMPONENT_VERIFY_TICKET", "WECHAT_COMPONENT_VERIFY_TICKET"),
		SourceRoot:                                         sourceRoot,
		ManifestPath:                                       manifestPath,
		MySQLDSN:                                           os.Getenv("MOCHAT_MYSQL_DSN"),
		SimpleJWTSecret:                                    envFirst("MOCHAT_SIMPLE_JWT_SECRET", "SIMPLE_JWT_SECRET"),
		SimpleJWTPrefix:                                    envOrDefault("MOCHAT_SIMPLE_JWT_PREFIX", envOrDefault("SIMPLE_JWT_PREFIX", "default")),
		SimpleJWTTTL:                                       time.Duration(ttl) * time.Second,
		SimpleJWTRefreshTTL:                                time.Duration(refreshTTL) * time.Second,
		SidebarJWTSecret:                                   envOrDefault("MOCHAT_SIDEBAR_JWT_SECRET", envOrDefault("SIDEBAR_JWT_SECRET", "Br3LXhp&Ysha1zRDh")),
		SidebarJWTPrefix:                                   envOrDefault("MOCHAT_SIDEBAR_JWT_PREFIX", envOrDefault("SIDEBAR_JWT_PREFIX", "default")),
		RedisAddr:                                          redisAddrFromEnv(),
		RedisPassword:                                      envFirst("MOCHAT_REDIS_PASSWORD", "REDIS_AUTH"),
		RedisDB:                                            redisDB,
		WorkerProcessingTimeout:                            time.Duration(workerProcessingTimeout) * time.Second,
		EnablePullAgentCron:                                envBool("MOCHAT_GO_ENABLE_PULL_AGENT_CRON"),
		PullAgentCronInterval:                              time.Duration(pullAgentCronInterval) * time.Second,
		PullAgentCronRunOnStart:                            envBool("MOCHAT_GO_PULL_AGENT_CRON_RUN_ON_START"),
		EnableEmployeeStatisticCron:                        envBool("MOCHAT_GO_ENABLE_EMPLOYEE_STATISTIC_CRON"),
		EmployeeStatisticCronInterval:                      time.Duration(employeeStatisticCronInterval) * time.Second,
		EmployeeStatisticCronRunOnStart:                    envBool("MOCHAT_GO_EMPLOYEE_STATISTIC_CRON_RUN_ON_START"),
		EnableChannelCodeCron:                              envBool("MOCHAT_GO_ENABLE_CHANNEL_CODE_CRON"),
		ChannelCodeCronInterval:                            time.Duration(channelCodeCronInterval) * time.Second,
		ChannelCodeCronRunOnStart:                          envBool("MOCHAT_GO_CHANNEL_CODE_CRON_RUN_ON_START"),
		EnableContactBatchSendCron:                         envBool("MOCHAT_GO_ENABLE_CONTACT_BATCH_SEND_CRON"),
		ContactBatchSendCronInterval:                       time.Duration(contactBatchSendCronInterval) * time.Second,
		ContactBatchSendCronRunOnStart:                     envBool("MOCHAT_GO_CONTACT_BATCH_SEND_CRON_RUN_ON_START"),
		EnableRoomBatchSendCron:                            envBool("MOCHAT_GO_ENABLE_ROOM_BATCH_SEND_CRON"),
		RoomBatchSendCronInterval:                          time.Duration(roomBatchSendCronInterval) * time.Second,
		RoomBatchSendCronRunOnStart:                        envBool("MOCHAT_GO_ROOM_BATCH_SEND_CRON_RUN_ON_START"),
		EnableContactSyncSendResultCron:                    envBool("MOCHAT_GO_ENABLE_CONTACT_SYNC_SEND_RESULT_CRON"),
		ContactSyncSendResultCronInterval:                  time.Duration(contactSyncSendResultCronInterval) * time.Second,
		ContactSyncSendResultCronRunOnStart:                envBool("MOCHAT_GO_CONTACT_SYNC_SEND_RESULT_CRON_RUN_ON_START"),
		EnableRoomSyncSendResultCron:                       envBool("MOCHAT_GO_ENABLE_ROOM_SYNC_SEND_RESULT_CRON"),
		RoomSyncSendResultCronInterval:                     time.Duration(roomSyncSendResultCronInterval) * time.Second,
		RoomSyncSendResultCronRunOnStart:                   envBool("MOCHAT_GO_ROOM_SYNC_SEND_RESULT_CRON_RUN_ON_START"),
		EnableRoomTagPullCron:                              envBool("MOCHAT_GO_ENABLE_ROOM_TAG_PULL_CRON"),
		RoomTagPullCronInterval:                            time.Duration(roomTagPullCronInterval) * time.Second,
		RoomTagPullCronRunOnStart:                          envBool("MOCHAT_GO_ROOM_TAG_PULL_CRON_RUN_ON_START"),
		EnableCorpDataCron:                                 envBool("MOCHAT_GO_ENABLE_CORP_DATA_CRON"),
		CorpDataCronInterval:                               time.Duration(corpDataCronInterval) * time.Second,
		CorpDataCronRunOnStart:                             envBool("MOCHAT_GO_CORP_DATA_CRON_RUN_ON_START"),
		EnableMediaIDUpdateCron:                            envBool("MOCHAT_GO_ENABLE_MEDIA_ID_UPDATE_CRON"),
		MediaIDUpdateCronInterval:                          time.Duration(mediaIDUpdateCronInterval) * time.Second,
		MediaIDUpdateCronRunOnStart:                        envBool("MOCHAT_GO_MEDIA_ID_UPDATE_CRON_RUN_ON_START"),
		EnableTransferStateRefreshCron:                     envBool("MOCHAT_GO_ENABLE_TRANSFER_STATE_REFRESH_CRON"),
		TransferStateRefreshCronInterval:                   time.Duration(transferStateRefreshCronInterval) * time.Second,
		TransferStateRefreshCronRunOnStart:                 envBool("MOCHAT_GO_TRANSFER_STATE_REFRESH_CRON_RUN_ON_START"),
		EnableSOPLogCron:                                   envBool("MOCHAT_GO_ENABLE_SOP_LOG_CRON"),
		SOPLogCronInterval:                                 time.Duration(sopLogCronInterval) * time.Second,
		SOPLogCronRunOnStart:                               envBool("MOCHAT_GO_SOP_LOG_CRON_RUN_ON_START"),
		EnableSensitiveWordMonitorCron:                     envBool("MOCHAT_GO_ENABLE_SENSITIVE_WORD_MONITOR_CRON"),
		SensitiveWordMonitorCronInterval:                   time.Duration(sensitiveWordMonitorCronInterval) * time.Second,
		SensitiveWordMonitorCronRunOnStart:                 envBool("MOCHAT_GO_SENSITIVE_WORD_MONITOR_CRON_RUN_ON_START"),
		EnableWorkMessageArchiveSyncCron:                   envBool("MOCHAT_GO_ENABLE_WORK_MESSAGE_ARCHIVE_SYNC_CRON"),
		WorkMessageArchiveSyncCronInterval:                 time.Duration(workMessageArchiveSyncCronInterval) * time.Second,
		WorkMessageArchiveSyncCronRunOnStart:               envBool("MOCHAT_GO_WORK_MESSAGE_ARCHIVE_SYNC_CRON_RUN_ON_START"),
		WorkMessageArchiveSyncLimit:                        workMessageArchiveSyncLimit,
		WorkMessageArchiveBridgeBaseURL:                    os.Getenv("MOCHAT_GO_WORK_MESSAGE_ARCHIVE_BRIDGE_BASE_URL"),
		WorkMessageArchiveBridgeToken:                      os.Getenv("MOCHAT_GO_WORK_MESSAGE_ARCHIVE_BRIDGE_TOKEN"),
		EnableSaaSStorageReconcileCron:                     envBool("MOCHAT_GO_ENABLE_SAAS_STORAGE_RECONCILE_CRON"),
		SaaSStorageReconcileCronInterval:                   time.Duration(saasStorageReconcileCronInterval) * time.Second,
		SaaSStorageReconcileCronRunOnStart:                 envBool("MOCHAT_GO_SAAS_STORAGE_RECONCILE_CRON_RUN_ON_START"),
		EnableSaaSAlertNotificationDispatchCron:            envBool("MOCHAT_GO_ENABLE_SAAS_ALERT_NOTIFICATION_DISPATCH_CRON"),
		SaaSAlertNotificationDispatchCronInterval:          time.Duration(saasAlertNotificationDispatchCronInterval) * time.Second,
		SaaSAlertNotificationDispatchCronRunOnStart:        envBool("MOCHAT_GO_SAAS_ALERT_NOTIFICATION_DISPATCH_CRON_RUN_ON_START"),
		SaaSAlertNotificationDispatchLimit:                 saasAlertNotificationDispatchLimit,
		SaaSAlertCredentialEncryptionKey:                   alertCredentialEncryption.Key,
		SaaSAlertCredentialEncryptionKeys:                  alertCredentialEncryption.Keys,
		SaaSAlertCredentialEncryptionKeyID:                 alertCredentialEncryption.KeyID,
		SaaSAlertCredentialRequireEncryption:               envBool("MOCHAT_GO_SAAS_ALERT_CREDENTIAL_REQUIRE_ENCRYPTION"),
		SaaSAlertCredentialDedicatedConfigured:             alertCredentialDedicatedConfigured,
		WeComCredentialEncryptionKey:                       weComCredentialEncryption.Key,
		WeComCredentialEncryptionKeys:                      weComCredentialEncryption.Keys,
		WeComCredentialEncryptionKeyID:                     weComCredentialEncryption.KeyID,
		WeComCredentialRequireEncryption:                   envBool("MOCHAT_GO_WECOM_CREDENTIAL_REQUIRE_ENCRYPTION"),
		WeComCredentialDedicatedConfigured:                 weComCredentialDedicatedConfigured,
		WeChatOpenCredentialEncryptionKey:                  weChatOpenCredentialEncryption.Key,
		WeChatOpenCredentialEncryptionKeys:                 weChatOpenCredentialEncryption.Keys,
		WeChatOpenCredentialEncryptionKeyID:                weChatOpenCredentialEncryption.KeyID,
		WeChatOpenCredentialRequireEncryption:              envBool("MOCHAT_GO_WECHAT_OPEN_CREDENTIAL_REQUIRE_ENCRYPTION"),
		WeChatOpenCredentialDedicatedConfigured:            weChatOpenCredentialDedicatedConfigured,
		EnableSaaSOperationQueueAssignmentReminderCron:     envBool("MOCHAT_GO_ENABLE_SAAS_OPERATION_QUEUE_ASSIGNMENT_REMINDER_CRON"),
		SaaSOperationQueueAssignmentReminderCronInterval:   time.Duration(saasOperationQueueAssignmentReminderCronInterval) * time.Second,
		SaaSOperationQueueAssignmentReminderCronRunOnStart: envBool("MOCHAT_GO_SAAS_OPERATION_QUEUE_ASSIGNMENT_REMINDER_CRON_RUN_ON_START"),
		SaaSOperationQueueAssignmentReminderLimit:          saasOperationQueueAssignmentReminderLimit,
		EnableSaaSApprovalReminderCron:                     envBool("MOCHAT_GO_ENABLE_SAAS_APPROVAL_REMINDER_CRON"),
		SaaSApprovalReminderCronInterval:                   time.Duration(saasApprovalReminderCronInterval) * time.Second,
		SaaSApprovalReminderCronRunOnStart:                 envBool("MOCHAT_GO_SAAS_APPROVAL_REMINDER_CRON_RUN_ON_START"),
		SaaSApprovalReminderLimit:                          saasApprovalReminderLimit,
		EnableSaaSSystemHealthCron:                         envBool("MOCHAT_GO_ENABLE_SAAS_SYSTEM_HEALTH_CRON"),
		SaaSSystemHealthCronInterval:                       time.Duration(saasSystemHealthCronInterval) * time.Second,
		SaaSSystemHealthCronRunOnStart:                     envBool("MOCHAT_GO_SAAS_SYSTEM_HEALTH_CRON_RUN_ON_START"),
		SaaSSystemHealthFailureWindowHours:                 saasSystemHealthFailureWindowHours,
		SaaSSystemHealthNotificationStaleMinutes:           saasSystemHealthNotificationStaleMinutes,
		EnableSaaSBackupCron:                               envBool("MOCHAT_GO_ENABLE_SAAS_BACKUP_CRON"),
		SaaSBackupCronInterval:                             time.Duration(saasBackupCronInterval) * time.Second,
		SaaSBackupCronRunOnStart:                           envBool("MOCHAT_GO_SAAS_BACKUP_CRON_RUN_ON_START"),
		SaaSBackupRoot:                                     envOrDefault("MOCHAT_GO_SAAS_BACKUP_ROOT", "./storage/backups"),
		SaaSBackupEncryptionKey:                            os.Getenv("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY"),
		SaaSBackupEncryptionKeys:                           os.Getenv("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEYS"),
		SaaSBackupEncryptionKeyID:                          envOrDefault("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY_ID", "primary"),
		SaaSBackupDumpBinary:                               envOrDefault("MOCHAT_GO_SAAS_BACKUP_DUMP_BINARY", "mysqldump"),
		SaaSBackupRestoreBinary:                            envOrDefault("MOCHAT_GO_SAAS_BACKUP_RESTORE_BINARY", "mysql"),
		SaaSBackupRestoreDSN:                               os.Getenv("MOCHAT_GO_SAAS_BACKUP_RESTORE_DSN"),
		SaaSBackupRestoreAdminDSN:                          os.Getenv("MOCHAT_GO_SAAS_BACKUP_RESTORE_ADMIN_DSN"),
		SaaSBackupRestoreAutoProvision:                     envBool("MOCHAT_GO_SAAS_BACKUP_RESTORE_AUTO_PROVISION"),
		SaaSBackupRestoreKeepOnFailure:                     envBool("MOCHAT_GO_SAAS_BACKUP_RESTORE_KEEP_ON_FAILURE"),
		SaaSBackupRestoreDatabasePrefix:                    envOrDefault("MOCHAT_GO_SAAS_BACKUP_RESTORE_DATABASE_PREFIX", "mochat_restore_"),
		SaaSBackupS3Endpoint:                               os.Getenv("MOCHAT_GO_SAAS_BACKUP_S3_ENDPOINT"),
		SaaSBackupS3Bucket:                                 os.Getenv("MOCHAT_GO_SAAS_BACKUP_S3_BUCKET"),
		SaaSBackupS3Region:                                 os.Getenv("MOCHAT_GO_SAAS_BACKUP_S3_REGION"),
		SaaSBackupS3AccessKeyID:                            os.Getenv("MOCHAT_GO_SAAS_BACKUP_S3_ACCESS_KEY_ID"),
		SaaSBackupS3SecretAccessKey:                        os.Getenv("MOCHAT_GO_SAAS_BACKUP_S3_SECRET_ACCESS_KEY"),
		SaaSBackupS3SessionToken:                           os.Getenv("MOCHAT_GO_SAAS_BACKUP_S3_SESSION_TOKEN"),
		SaaSBackupS3UseSSL:                                 envBoolDefault("MOCHAT_GO_SAAS_BACKUP_S3_USE_SSL", true),
		SaaSBackupS3Prefix:                                 envOrDefault("MOCHAT_GO_SAAS_BACKUP_S3_PREFIX", "mochat-go/backups"),
		EnableSaaSComplianceCron:                           envBool("MOCHAT_GO_ENABLE_SAAS_COMPLIANCE_CRON"),
		SaaSComplianceCronInterval:                         time.Duration(saasComplianceCronInterval) * time.Second,
		SaaSComplianceCronRunOnStart:                       envBool("MOCHAT_GO_SAAS_COMPLIANCE_CRON_RUN_ON_START"),
		SaaSComplianceArtifactRoot:                         envOrDefault("MOCHAT_GO_SAAS_COMPLIANCE_ARTIFACT_ROOT", "./storage/compliance"),
		SaaSComplianceEncryptionKey:                        envOrDefault("MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY", os.Getenv("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY")),
		SaaSComplianceEncryptionKeys:                       envOrDefault("MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEYS", os.Getenv("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEYS")),
		SaaSComplianceEncryptionKeyID:                      envOrDefault("MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY_ID", envOrDefault("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY_ID", "primary")),
		EnableSaaSIdentitySecurity:                         envBool("MOCHAT_GO_ENABLE_SAAS_IDENTITY_SECURITY"),
		SaaSIdentityEnforceSessions:                        envBool("MOCHAT_GO_SAAS_IDENTITY_ENFORCE_SESSIONS"),
		SaaSIdentityTrustProxyHeaders:                      identityTrustProxyHeaders,
		SaaSServiceAccountTrustProxyHeaders:                serviceAccountTrustProxyHeaders,
		SaaSTrustedProxyCIDRs:                              trustedProxyCIDRs,
		SaaSIdentityIssuer:                                 envOrDefault("MOCHAT_GO_SAAS_IDENTITY_ISSUER", "MoChat Go"),
		SaaSIdentityEncryptionKey:                          envOrDefault("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY", envOrDefault("MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY", os.Getenv("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY"))),
		SaaSIdentityEncryptionKeys:                         envOrDefault("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEYS", envOrDefault("MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEYS", os.Getenv("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEYS"))),
		SaaSIdentityEncryptionKeyID:                        envOrDefault("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY_ID", envOrDefault("MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY_ID", envOrDefault("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY_ID", "primary"))),
		EnableSaaSIdentityCleanupCron:                      envBool("MOCHAT_GO_ENABLE_SAAS_IDENTITY_CLEANUP_CRON"),
		SaaSIdentityCleanupCronInterval:                    time.Duration(saasIdentityCleanupCronInterval) * time.Second,
		SaaSIdentityCleanupCronRunOnStart:                  envBool("MOCHAT_GO_SAAS_IDENTITY_CLEANUP_CRON_RUN_ON_START"),
		SaaSIdentityCleanupLimit:                           saasIdentityCleanupLimit,
		EnableSaaSServiceAccountUsageAlertCron:             envBool("MOCHAT_GO_ENABLE_SAAS_SERVICE_ACCOUNT_USAGE_ALERT_CRON"),
		SaaSServiceAccountUsageAlertCronInterval:           time.Duration(saasServiceAccountUsageAlertCronInterval) * time.Second,
		SaaSServiceAccountUsageAlertCronRunOnStart:         envBool("MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_ALERT_CRON_RUN_ON_START"),
		SaaSServiceAccountUsageAlertLimit:                  saasServiceAccountUsageAlertLimit,
		SaaSServiceAccountKeyPepper:                        os.Getenv("MOCHAT_GO_SAAS_SERVICE_ACCOUNT_KEY_PEPPER"),
		SaaSServiceAccountKeyPeppers:                       os.Getenv("MOCHAT_GO_SAAS_SERVICE_ACCOUNT_KEY_PEPPERS"),
		SaaSServiceAccountKeyPepperID:                      envOrDefault("MOCHAT_GO_SAAS_SERVICE_ACCOUNT_KEY_PEPPER_ID", "primary"),
		SaaSServiceAccountAllowLegacyJWTPepper:             envBoolDefault("MOCHAT_GO_SAAS_SERVICE_ACCOUNT_ALLOW_LEGACY_JWT_PEPPER", true),
		SaaSServiceAccountRequireDedicatedPepper:           envBool("MOCHAT_GO_SAAS_SERVICE_ACCOUNT_REQUIRE_DEDICATED_PEPPER"),
		EnableSaaSAuditIntegrityCron:                       envBool("MOCHAT_GO_ENABLE_SAAS_AUDIT_INTEGRITY_CRON"),
		SaaSAuditIntegrityCronInterval:                     time.Duration(saasAuditIntegrityCronInterval) * time.Second,
		SaaSAuditIntegrityCronRunOnStart:                   envBool("MOCHAT_GO_SAAS_AUDIT_INTEGRITY_CRON_RUN_ON_START"),
		SaaSAuditIntegrityLimit:                            saasAuditIntegrityLimit,
		EnableSaaSAuditAnchorCron:                          envBool("MOCHAT_GO_ENABLE_SAAS_AUDIT_ANCHOR_CRON"),
		SaaSAuditAnchorCronInterval:                        time.Duration(saasAuditAnchorCronInterval) * time.Second,
		SaaSAuditAnchorCronRunOnStart:                      envBool("MOCHAT_GO_SAAS_AUDIT_ANCHOR_CRON_RUN_ON_START"),
		SaaSAuditAnchorLimit:                               saasAuditAnchorLimit,
		SaaSAuditAnchorArtifactRoot:                        envOrDefault("MOCHAT_GO_SAAS_AUDIT_ANCHOR_ARTIFACT_ROOT", "./storage/audit-anchors"),
		SaaSAuditAnchorHMACKey:                             os.Getenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_HMAC_KEY"),
		SaaSAuditAnchorHMACKeys:                            os.Getenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_HMAC_KEYS"),
		SaaSAuditAnchorHMACKeyID:                           envOrDefault("MOCHAT_GO_SAAS_AUDIT_ANCHOR_HMAC_KEY_ID", "primary"),
		SaaSAuditAnchorRequireRemote:                       envBool("MOCHAT_GO_SAAS_AUDIT_ANCHOR_REQUIRE_REMOTE"),
		SaaSAuditAnchorS3Endpoint:                          strings.TrimSpace(os.Getenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_ENDPOINT")),
		SaaSAuditAnchorS3Bucket:                            strings.TrimSpace(os.Getenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_BUCKET")),
		SaaSAuditAnchorS3Region:                            strings.TrimSpace(os.Getenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_REGION")),
		SaaSAuditAnchorS3AccessKeyID:                       strings.TrimSpace(os.Getenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_ACCESS_KEY_ID")),
		SaaSAuditAnchorS3SecretAccessKey:                   os.Getenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_SECRET_ACCESS_KEY"),
		SaaSAuditAnchorS3SessionToken:                      os.Getenv("MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_SESSION_TOKEN"),
		SaaSAuditAnchorS3UseSSL:                            envBoolDefault("MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_USE_SSL", true),
		SaaSAuditAnchorS3Prefix:                            envOrDefault("MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_PREFIX", "mochat-go/audit-anchors"),
		SaaSAuditAnchorS3RetentionMode:                     strings.ToLower(envOrDefault("MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_RETENTION_MODE", "compliance")),
		SaaSAuditAnchorS3RetentionDays:                     saasAuditAnchorS3RetentionDays,
		EnableSaaSServiceAccountUsageCleanupCron:           envBool("MOCHAT_GO_ENABLE_SAAS_SERVICE_ACCOUNT_USAGE_CLEANUP_CRON"),
		SaaSServiceAccountUsageCleanupCronInterval:         time.Duration(saasServiceAccountUsageCleanupCronInterval) * time.Second,
		SaaSServiceAccountUsageCleanupCronRunOnStart:       envBool("MOCHAT_GO_SAAS_SERVICE_ACCOUNT_USAGE_CLEANUP_CRON_RUN_ON_START"),
		SaaSServiceAccountUsageCleanupLimit:                saasServiceAccountUsageCleanupLimit,
		SaaSTenantDomainDNSServer:                          strings.TrimSpace(os.Getenv("MOCHAT_GO_SAAS_TENANT_DOMAIN_DNS_SERVER")),
		SaaSTenantDomainDNSTimeout:                         time.Duration(saasTenantDomainDNSTimeout) * time.Second,
		EnableSaaSTenantDomainDeliveryCron:                 envBool("MOCHAT_GO_ENABLE_SAAS_TENANT_DOMAIN_DELIVERY_CRON"),
		SaaSTenantDomainDeliveryCronInterval:               time.Duration(saasTenantDomainDeliveryCronInterval) * time.Second,
		SaaSTenantDomainDeliveryCronRunOnStart:             envBool("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CRON_RUN_ON_START"),
		SaaSTenantDomainDeliveryBridgeBaseURL:              strings.TrimSpace(os.Getenv("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_BRIDGE_BASE_URL")),
		SaaSTenantDomainDeliveryBridgeToken:                strings.TrimSpace(os.Getenv("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_BRIDGE_TOKEN")),
		SaaSTenantDomainDeliveryBridgeTimeout:              time.Duration(saasTenantDomainDeliveryBridgeTimeout) * time.Second,
		SaaSTenantDomainDeliveryCallbackURL:                strings.TrimSpace(os.Getenv("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CALLBACK_URL")),
		SaaSTenantDomainDeliveryCallbackSecret:             strings.TrimSpace(os.Getenv("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CALLBACK_SECRET")),
		SaaSTenantDomainDeliveryCallbackTolerance:          time.Duration(saasTenantDomainDeliveryCallbackTolerance) * time.Second,
		SaaSTenantDomainDeliveryLimit:                      saasTenantDomainDeliveryLimit,
		SaaSTenantDomainDeliveryLease:                      time.Duration(saasTenantDomainDeliveryLease) * time.Second,
		SaaSTenantDomainDeliveryRetryDelay:                 time.Duration(saasTenantDomainDeliveryRetryDelay) * time.Second,
		SaaSTenantDomainDeliveryCallbackWait:               time.Duration(saasTenantDomainDeliveryCallbackWait) * time.Second,
		SaaSTenantDomainCertificateExpiryWarningDays:       saasTenantDomainCertificateExpiryWarningDays,
		EnableSaaSNotificationHealthRecoveryCron:           envBool("MOCHAT_GO_ENABLE_SAAS_NOTIFICATION_HEALTH_RECOVERY_CRON"),
		SaaSNotificationHealthRecoveryCronInterval:         time.Duration(saasNotificationHealthRecoveryCronInterval) * time.Second,
		SaaSNotificationHealthRecoveryCronRunOnStart:       envBool("MOCHAT_GO_SAAS_NOTIFICATION_HEALTH_RECOVERY_CRON_RUN_ON_START"),
		SaaSNotificationHealthRecoveryWindowHours:          saasNotificationHealthRecoveryWindowHours,
		SaaSNotificationHealthRecoveryStaleMinutes:         saasNotificationHealthRecoveryStaleMinutes,
		EnableSaaSSubscriptionReconcileCron:                envBool("MOCHAT_GO_ENABLE_SAAS_SUBSCRIPTION_RECONCILE_CRON"),
		SaaSSubscriptionReconcileCronInterval:              time.Duration(saasSubscriptionReconcileCronInterval) * time.Second,
		SaaSSubscriptionReconcileCronRunOnStart:            envBool("MOCHAT_GO_SAAS_SUBSCRIPTION_RECONCILE_CRON_RUN_ON_START"),
		SaaSSubscriptionReconcileLimit:                     saasSubscriptionReconcileLimit,
		EnableSaaSPaymentWebhook:                           envBool("MOCHAT_GO_ENABLE_SAAS_PAYMENT_WEBHOOK"),
		SaaSPaymentWebhookSecret:                           os.Getenv("MOCHAT_GO_SAAS_PAYMENT_WEBHOOK_SECRET"),
		SaaSPaymentWebhookTolerance:                        time.Duration(saasPaymentWebhookToleranceSeconds) * time.Second,
		EnableSaaSPaymentDunningCron:                       envBool("MOCHAT_GO_ENABLE_SAAS_PAYMENT_DUNNING_CRON"),
		SaaSPaymentDunningCronInterval:                     time.Duration(saasPaymentDunningCronInterval) * time.Second,
		SaaSPaymentDunningCronRunOnStart:                   envBool("MOCHAT_GO_SAAS_PAYMENT_DUNNING_CRON_RUN_ON_START"),
		SaaSPaymentDunningLimit:                            saasPaymentDunningLimit,
		SaaSPaymentDunningRetryDelay:                       time.Duration(saasPaymentDunningRetryDelaySeconds) * time.Second,
		EnableSaaSPaymentSettlementSyncCron:                envBool("MOCHAT_GO_ENABLE_SAAS_PAYMENT_SETTLEMENT_SYNC_CRON"),
		SaaSPaymentSettlementSyncCronInterval:              time.Duration(saasPaymentSettlementSyncCronInterval) * time.Second,
		SaaSPaymentSettlementSyncCronRunOnStart:            envBool("MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_SYNC_CRON_RUN_ON_START"),
		SaaSPaymentSettlementBridgeBaseURL:                 os.Getenv("MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_BRIDGE_BASE_URL"),
		SaaSPaymentSettlementBridgeToken:                   os.Getenv("MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_BRIDGE_TOKEN"),
		SaaSPaymentSettlementProviders:                     saasPaymentSettlementProviders,
		SaaSPaymentSettlementSyncLimit:                     saasPaymentSettlementSyncLimit,
		SaaSPaymentSettlementBridgeTimeout:                 time.Duration(saasPaymentSettlementBridgeTimeoutSeconds) * time.Second,
		EnableSaaSAlertDashboard:                           envBool("MOCHAT_GO_ENABLE_SAAS_ALERT_DASHBOARD"),
		EnableSaaSAdminDashboard:                           envBool("MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD"),
		SaaSAdminApprovalRequired:                          envBoolDefault("MOCHAT_GO_SAAS_ADMIN_APPROVAL_REQUIRED", true),
		EnableSaaSBillingPortal:                            envBool("MOCHAT_GO_ENABLE_SAAS_BILLING_PORTAL"),
		SaaSPlatformAdminTenantID:                          saasPlatformAdminTenantID,
		SaaSReleaseSourceFingerprint:                       releaseSourceFingerprint,
		SaaSReleaseSourceFingerprintSource:                 releaseSourceFingerprintSource,
		SaaSReleaseEvidenceVerifyTimeout:                   time.Duration(saasReleaseEvidenceVerifyTimeoutSeconds) * time.Second,
		SaaSReleaseEvidenceMaxBytes:                        int64(saasReleaseEvidenceMaxBytes),
		SaaSReleaseEvidenceAllowedCIDRs:                    saasReleaseEvidenceAllowedCIDRs,
		SaaSReleaseEvidenceCAFile:                          strings.TrimSpace(os.Getenv("MOCHAT_GO_SAAS_RELEASE_EVIDENCE_CA_FILE")),
		SaaSAlertWebhookURL:                                os.Getenv("MOCHAT_GO_SAAS_ALERT_WEBHOOK_URL"),
		SaaSAlertWebhookSecret:                             os.Getenv("MOCHAT_GO_SAAS_ALERT_WEBHOOK_SECRET"),
		SaaSAlertWebhookTimeout:                            time.Duration(saasAlertWebhookTimeout) * time.Second,
		SaaSAlertWebhookRetryAttempts:                      saasAlertWebhookRetryAttempts,
		SaaSAlertWebhookRetryDelay:                         time.Duration(saasAlertWebhookRetryDelayMS) * time.Millisecond,
		SaaSAlertWebhookTitleTemplate:                      saasAlertWebhookTitleTemplate,
		SaaSAlertWebhookBodyTemplate:                       saasAlertWebhookBodyTemplate,
		SaaSAlertWebhookRequireHTTPS:                       saasAlertWebhookRequireHTTPS,
		SaaSAlertWebhookAllowedCIDRs:                       saasAlertWebhookAllowedCIDRs,
		SaaSAlertNotificationMaxAttempts:                   saasAlertNotificationMaxAttempts,
		SaaSAlertNotificationRetryDelay:                    time.Duration(saasAlertNotificationRetryDelaySeconds) * time.Second,
		MigrateAuth:                                        envBoolDefault("MOCHAT_GO_MIGRATE_AUTH", enableAllMigratedRoutes),
		MigrateLoginShow:                                   envBoolDefault("MOCHAT_GO_MIGRATE_LOGIN_SHOW", enableAllMigratedRoutes),
		MigrateLogout:                                      envBoolDefault("MOCHAT_GO_MIGRATE_LOGOUT", enableAllMigratedRoutes),
		MigrateUserIndex:                                   envBoolDefault("MOCHAT_GO_MIGRATE_USER_INDEX", enableAllMigratedRoutes),
		MigrateUserShow:                                    envBoolDefault("MOCHAT_GO_MIGRATE_USER_SHOW", enableAllMigratedRoutes),
		MigrateUserStore:                                   envBoolDefault("MOCHAT_GO_MIGRATE_USER_STORE", enableAllMigratedRoutes),
		MigrateUserUpdate:                                  envBoolDefault("MOCHAT_GO_MIGRATE_USER_UPDATE", enableAllMigratedRoutes),
		MigrateUserStatusUpdate:                            envBoolDefault("MOCHAT_GO_MIGRATE_USER_STATUS_UPDATE", enableAllMigratedRoutes),
		MigrateUserPasswordReset:                           envBoolDefault("MOCHAT_GO_MIGRATE_USER_PASSWORD_RESET", enableAllMigratedRoutes),
		MigrateUserPasswordUpdate:                          envBoolDefault("MOCHAT_GO_MIGRATE_USER_PASSWORD_UPDATE", enableAllMigratedRoutes),
		MigratePermissionByUser:                            envBoolDefault("MOCHAT_GO_MIGRATE_PERMISSION_BY_USER", enableAllMigratedRoutes),
		MigrateCorpSelect:                                  envBoolDefault("MOCHAT_GO_MIGRATE_CORP_SELECT", enableAllMigratedRoutes),
		MigrateCorpBind:                                    envBoolDefault("MOCHAT_GO_MIGRATE_CORP_BIND", enableAllMigratedRoutes),
		MigrateCorpIndex:                                   envBoolDefault("MOCHAT_GO_MIGRATE_CORP_INDEX", enableAllMigratedRoutes),
		MigrateCorpShow:                                    envBoolDefault("MOCHAT_GO_MIGRATE_CORP_SHOW", enableAllMigratedRoutes),
		MigrateCorpStore:                                   envBoolDefault("MOCHAT_GO_MIGRATE_CORP_STORE", enableAllMigratedRoutes),
		MigrateCorpUpdate:                                  envBoolDefault("MOCHAT_GO_MIGRATE_CORP_UPDATE", enableAllMigratedRoutes),
		MigrateWeWorkCallback:                              envBoolDefault("MOCHAT_GO_MIGRATE_WEWORK_CALLBACK", enableAllMigratedRoutes),
		MigrateCorpDataIndex:                               envBoolDefault("MOCHAT_GO_MIGRATE_CORP_DATA_INDEX", enableAllMigratedRoutes),
		MigrateCorpDataLineChat:                            envBoolDefault("MOCHAT_GO_MIGRATE_CORP_DATA_LINE_CHAT", enableAllMigratedRoutes),
		MigrateStatisticIndex:                              envBoolDefault("MOCHAT_GO_MIGRATE_STATISTIC_INDEX", enableAllMigratedRoutes),
		MigrateStatisticTopList:                            envBoolDefault("MOCHAT_GO_MIGRATE_STATISTIC_TOP_LIST", enableAllMigratedRoutes),
		MigrateStatisticEmployeeCounts:                     envBoolDefault("MOCHAT_GO_MIGRATE_STATISTIC_EMPLOYEE_COUNTS", enableAllMigratedRoutes),
		MigrateStatisticEmployees:                          envBoolDefault("MOCHAT_GO_MIGRATE_STATISTIC_EMPLOYEES", enableAllMigratedRoutes),
		MigrateStatisticEmployeesTrend:                     envBoolDefault("MOCHAT_GO_MIGRATE_STATISTIC_EMPLOYEES_TREND", enableAllMigratedRoutes),
		MigrateWorkEmployeeIndex:                           envBoolDefault("MOCHAT_GO_MIGRATE_WORK_EMPLOYEE_INDEX", enableAllMigratedRoutes),
		MigrateWorkEmployeeCond:                            envBoolDefault("MOCHAT_GO_MIGRATE_WORK_EMPLOYEE_SEARCH_CONDITION", enableAllMigratedRoutes),
		MigrateWorkEmployeeSync:                            envBoolDefault("MOCHAT_GO_MIGRATE_WORK_EMPLOYEE_SYNC", enableAllMigratedRoutes),
		MigrateWorkDeptIndex:                               envBoolDefault("MOCHAT_GO_MIGRATE_WORK_DEPARTMENT_INDEX", enableAllMigratedRoutes),
		MigrateWorkDeptMember:                              envBoolDefault("MOCHAT_GO_MIGRATE_WORK_EMPLOYEE_DEPARTMENT_MEMBER_INDEX", enableAllMigratedRoutes),
		MigrateWorkDeptPhone:                               envBoolDefault("MOCHAT_GO_MIGRATE_WORK_DEPARTMENT_SELECT_BY_PHONE", enableAllMigratedRoutes),
		MigrateWorkDeptPage:                                envBoolDefault("MOCHAT_GO_MIGRATE_WORK_DEPARTMENT_PAGE_INDEX", enableAllMigratedRoutes),
		MigrateWorkDeptEmployee:                            envBoolDefault("MOCHAT_GO_MIGRATE_WORK_DEPARTMENT_SHOW_EMPLOYEE", enableAllMigratedRoutes),
		MigrateWorkTagGroupIndex:                           envBoolDefault("MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_GROUP_INDEX", enableAllMigratedRoutes),
		MigrateWorkTagGroupDetail:                          envBoolDefault("MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_GROUP_DETAIL", enableAllMigratedRoutes),
		MigrateWorkTagGroupStore:                           envBoolDefault("MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_GROUP_STORE", enableAllMigratedRoutes),
		MigrateWorkTagGroupUpdate:                          envBoolDefault("MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_GROUP_UPDATE", enableAllMigratedRoutes),
		MigrateWorkTagGroupDestroy:                         envBoolDefault("MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_GROUP_DESTROY", enableAllMigratedRoutes),
		MigrateSidebarTagGroupIndex:                        envBoolDefault("MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_TAG_GROUP_INDEX", enableAllMigratedRoutes),
		MigrateWorkContactTagIndex:                         envBoolDefault("MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_INDEX", enableAllMigratedRoutes),
		MigrateWorkContactTagDetail:                        envBoolDefault("MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_DETAIL", enableAllMigratedRoutes),
		MigrateWorkContactTagList:                          envBoolDefault("MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_LIST", enableAllMigratedRoutes),
		MigrateWorkContactTagAll:                           envBoolDefault("MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_ALL", enableAllMigratedRoutes),
		MigrateWorkContactTagStore:                         envBoolDefault("MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_STORE", enableAllMigratedRoutes),
		MigrateWorkContactTagUpdate:                        envBoolDefault("MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_UPDATE", enableAllMigratedRoutes),
		MigrateWorkContactTagDestroy:                       envBoolDefault("MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_DESTROY", enableAllMigratedRoutes),
		MigrateWorkContactTagMove:                          envBoolDefault("MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_MOVE", enableAllMigratedRoutes),
		MigrateWorkContactTagSync:                          envBoolDefault("MOCHAT_GO_MIGRATE_WORK_CONTACT_TAG_SYNC", enableAllMigratedRoutes),
		MigrateWorkContactSync:                             envBoolDefault("MOCHAT_GO_MIGRATE_WORK_CONTACT_SYNC", enableAllMigratedRoutes),
		MigrateWorkContactIndex:                            envBoolDefault("MOCHAT_GO_MIGRATE_WORK_CONTACT_INDEX", enableAllMigratedRoutes),
		MigrateWorkContactLoss:                             envBoolDefault("MOCHAT_GO_MIGRATE_WORK_CONTACT_LOSS", enableAllMigratedRoutes),
		MigrateWorkContactSource:                           envBoolDefault("MOCHAT_GO_MIGRATE_WORK_CONTACT_SOURCE", enableAllMigratedRoutes),
		MigrateWorkContactShow:                             envBoolDefault("MOCHAT_GO_MIGRATE_WORK_CONTACT_SHOW", enableAllMigratedRoutes),
		MigrateWorkContactTrack:                            envBoolDefault("MOCHAT_GO_MIGRATE_WORK_CONTACT_TRACK", enableAllMigratedRoutes),
		MigrateWorkContactUpdate:                           envBoolDefault("MOCHAT_GO_MIGRATE_WORK_CONTACT_UPDATE", enableAllMigratedRoutes),
		MigrateWorkContactBatchLabeling:                    envBoolDefault("MOCHAT_GO_MIGRATE_WORK_CONTACT_BATCH_LABELING", enableAllMigratedRoutes),
		MigrateWorkContactRoomIndex:                        envBoolDefault("MOCHAT_GO_MIGRATE_WORK_CONTACT_ROOM_INDEX", enableAllMigratedRoutes),
		MigrateWorkRoomIndex:                               envBoolDefault("MOCHAT_GO_MIGRATE_WORK_ROOM_INDEX", enableAllMigratedRoutes),
		MigrateWorkRoomRoomIndex:                           envBoolDefault("MOCHAT_GO_MIGRATE_WORK_ROOM_ROOM_INDEX", enableAllMigratedRoutes),
		MigrateWorkRoomStatistics:                          envBoolDefault("MOCHAT_GO_MIGRATE_WORK_ROOM_STATISTICS", enableAllMigratedRoutes),
		MigrateWorkRoomStatisticsIndex:                     envBoolDefault("MOCHAT_GO_MIGRATE_WORK_ROOM_STATISTICS_INDEX", enableAllMigratedRoutes),
		MigrateWorkRoomSync:                                envBoolDefault("MOCHAT_GO_MIGRATE_WORK_ROOM_SYNC", enableAllMigratedRoutes),
		MigrateWorkRoomBatchUpdate:                         envBoolDefault("MOCHAT_GO_MIGRATE_WORK_ROOM_BATCH_UPDATE", enableAllMigratedRoutes),
		MigrateSidebarWorkRoomManage:                       envBoolDefault("MOCHAT_GO_MIGRATE_SIDEBAR_WORK_ROOM_MANAGE", enableAllMigratedRoutes),
		MigrateContactTransferInfo:                         envBoolDefault("MOCHAT_GO_MIGRATE_CONTACT_TRANSFER_INFO", enableAllMigratedRoutes),
		MigrateContactTransferUnassigned:                   envBoolDefault("MOCHAT_GO_MIGRATE_CONTACT_TRANSFER_UNASSIGNED_LIST", enableAllMigratedRoutes),
		MigrateContactTransferRoom:                         envBoolDefault("MOCHAT_GO_MIGRATE_CONTACT_TRANSFER_ROOM", enableAllMigratedRoutes),
		MigrateContactTransferLog:                          envBoolDefault("MOCHAT_GO_MIGRATE_CONTACT_TRANSFER_LOG", enableAllMigratedRoutes),
		MigrateContactTransferSync:                         envBoolDefault("MOCHAT_GO_MIGRATE_CONTACT_TRANSFER_SAVE_UNASSIGNED_LIST", enableAllMigratedRoutes),
		MigrateContactTransferCustomer:                     envBoolDefault("MOCHAT_GO_MIGRATE_CONTACT_TRANSFER_INDEX", enableAllMigratedRoutes),
		MigrateContactTransferRoomStore:                    envBoolDefault("MOCHAT_GO_MIGRATE_CONTACT_TRANSFER_ROOM_STORE", enableAllMigratedRoutes),
		MigrateWorkRoomAutoPullIndex:                       envBoolDefault("MOCHAT_GO_MIGRATE_WORK_ROOM_AUTO_PULL_INDEX", enableAllMigratedRoutes),
		MigrateWorkRoomAutoPullShow:                        envBoolDefault("MOCHAT_GO_MIGRATE_WORK_ROOM_AUTO_PULL_SHOW", enableAllMigratedRoutes),
		MigrateWorkRoomAutoPullStore:                       envBoolDefault("MOCHAT_GO_MIGRATE_WORK_ROOM_AUTO_PULL_STORE", enableAllMigratedRoutes),
		MigrateWorkRoomAutoPullUpdate:                      envBoolDefault("MOCHAT_GO_MIGRATE_WORK_ROOM_AUTO_PULL_UPDATE", enableAllMigratedRoutes),
		MigrateRoomTagPullIndex:                            envBoolDefault("MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_INDEX", enableAllMigratedRoutes),
		MigrateRoomTagPullShow:                             envBoolDefault("MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_SHOW", enableAllMigratedRoutes),
		MigrateRoomTagPullShowContact:                      envBoolDefault("MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_SHOW_CONTACT", enableAllMigratedRoutes),
		MigrateRoomTagPullRoomList:                         envBoolDefault("MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_ROOM_LIST", enableAllMigratedRoutes),
		MigrateRoomTagPullChooseContact:                    envBoolDefault("MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_CHOOSE_CONTACT", enableAllMigratedRoutes),
		MigrateRoomTagPullStore:                            envBoolDefault("MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_STORE", enableAllMigratedRoutes),
		MigrateRoomTagPullFilterContact:                    envBoolDefault("MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_FILTER_CONTACT", enableAllMigratedRoutes),
		MigrateRoomTagPullRemindSend:                       envBoolDefault("MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_REMIND_SEND", enableAllMigratedRoutes),
		MigrateRoomTagPullDestroy:                          envBoolDefault("MOCHAT_GO_MIGRATE_ROOM_TAG_PULL_DESTROY", enableAllMigratedRoutes),
		MigrateContactMessageBatchSendIndex:                envBoolDefault("MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_INDEX", enableAllMigratedRoutes),
		MigrateContactMessageBatchSendShow:                 envBoolDefault("MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_SHOW", enableAllMigratedRoutes),
		MigrateContactMessageBatchSendShowRoom:             envBoolDefault("MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_SHOW_ROOM", enableAllMigratedRoutes),
		MigrateContactMessageBatchSendEmployee:             envBoolDefault("MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_EMPLOYEE_SEND_INDEX", enableAllMigratedRoutes),
		MigrateContactMessageBatchSendContactReceive:       envBoolDefault("MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_CONTACT_RECEIVE_INDEX", enableAllMigratedRoutes),
		MigrateContactMessageBatchSendStore:                envBoolDefault("MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_STORE", enableAllMigratedRoutes),
		MigrateContactMessageBatchSendRemind:               envBoolDefault("MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_REMIND", enableAllMigratedRoutes),
		MigrateContactMessageBatchSendDestroy:              envBoolDefault("MOCHAT_GO_MIGRATE_CONTACT_MESSAGE_BATCH_SEND_DESTROY", enableAllMigratedRoutes),
		MigrateRoomMessageBatchSendIndex:                   envBoolDefault("MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_INDEX", enableAllMigratedRoutes),
		MigrateRoomMessageBatchSendShow:                    envBoolDefault("MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_SHOW", enableAllMigratedRoutes),
		MigrateRoomMessageBatchSendOwner:                   envBoolDefault("MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_ROOM_OWNER_SEND_INDEX", enableAllMigratedRoutes),
		MigrateRoomMessageBatchSendRoomReceive:             envBoolDefault("MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_ROOM_RECEIVE_INDEX", enableAllMigratedRoutes),
		MigrateRoomMessageBatchSendStore:                   envBoolDefault("MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_STORE", enableAllMigratedRoutes),
		MigrateRoomMessageBatchSendRemind:                  envBoolDefault("MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_REMIND", enableAllMigratedRoutes),
		MigrateRoomMessageBatchSendDestroy:                 envBoolDefault("MOCHAT_GO_MIGRATE_ROOM_MESSAGE_BATCH_SEND_DESTROY", enableAllMigratedRoutes),
		MigrateOfficialAccountIndex:                        envBoolDefault("MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_INDEX", enableAllMigratedRoutes),
		MigrateOfficialAccountSet:                          envBoolDefault("MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_SET", enableAllMigratedRoutes),
		MigrateOfficialAccountGetPreAuthURL:                envBoolDefault("MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_GET_PRE_AUTH_URL", enableAllMigratedRoutes),
		MigrateOfficialAccountAuthRedirect:                 envBoolDefault("MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_AUTH_REDIRECT", enableAllMigratedRoutes),
		MigrateOfficialAccountAuthEventCallback:            envBoolDefault("MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_AUTH_EVENT_CALLBACK", enableAllMigratedRoutes),
		MigrateOfficialAccountMessageEventCallback:         envBoolDefault("MOCHAT_GO_MIGRATE_OFFICIAL_ACCOUNT_MESSAGE_EVENT_CALLBACK", enableAllMigratedRoutes),
		MigrateLoad:                                        envBoolDefault("MOCHAT_GO_MIGRATE_LOAD", enableAllMigratedRoutes),
		MigrateWorkFissionIndex:                            envBoolDefault("MOCHAT_GO_MIGRATE_WORK_FISSION_INDEX", enableAllMigratedRoutes),
		MigrateWorkFissionShow:                             envBoolDefault("MOCHAT_GO_MIGRATE_WORK_FISSION_SHOW", enableAllMigratedRoutes),
		MigrateWorkFissionInfo:                             envBoolDefault("MOCHAT_GO_MIGRATE_WORK_FISSION_INFO", enableAllMigratedRoutes),
		MigrateWorkFissionStatistics:                       envBoolDefault("MOCHAT_GO_MIGRATE_WORK_FISSION_STATISTICS", enableAllMigratedRoutes),
		MigrateWorkFissionChooseContact:                    envBoolDefault("MOCHAT_GO_MIGRATE_WORK_FISSION_CHOOSE_CONTACT", enableAllMigratedRoutes),
		MigrateWorkFissionStore:                            envBoolDefault("MOCHAT_GO_MIGRATE_WORK_FISSION_STORE", enableAllMigratedRoutes),
		MigrateWorkFissionUpdate:                           envBoolDefault("MOCHAT_GO_MIGRATE_WORK_FISSION_UPDATE", enableAllMigratedRoutes),
		MigrateWorkFissionInvite:                           envBoolDefault("MOCHAT_GO_MIGRATE_WORK_FISSION_INVITE", enableAllMigratedRoutes),
		MigrateWorkFissionInviteData:                       envBoolDefault("MOCHAT_GO_MIGRATE_WORK_FISSION_INVITE_DATA", enableAllMigratedRoutes),
		MigrateWorkFissionInviteDetail:                     envBoolDefault("MOCHAT_GO_MIGRATE_WORK_FISSION_INVITE_DETAIL", enableAllMigratedRoutes),
		MigrateWorkFissionDestroy:                          envBoolDefault("MOCHAT_GO_MIGRATE_WORK_FISSION_DESTROY", enableAllMigratedRoutes),
		MigrateOperationWorkFissionInviteFriends:           envBoolDefault("MOCHAT_GO_MIGRATE_OPERATION_WORK_FISSION_INVITE_FRIENDS", enableAllMigratedRoutes),
		MigrateOperationWorkFissionPoster:                  envBoolDefault("MOCHAT_GO_MIGRATE_OPERATION_WORK_FISSION_POSTER", enableAllMigratedRoutes),
		MigrateOperationWorkFissionTaskData:                envBoolDefault("MOCHAT_GO_MIGRATE_OPERATION_WORK_FISSION_TASK_DATA", enableAllMigratedRoutes),
		MigrateOperationWorkFissionReceive:                 envBoolDefault("MOCHAT_GO_MIGRATE_OPERATION_WORK_FISSION_RECEIVE", enableAllMigratedRoutes),
		MigrateOperationWorkFissionAuth:                    envBoolDefault("MOCHAT_GO_MIGRATE_OPERATION_WORK_FISSION_AUTH", enableAllMigratedRoutes),
		MigrateOperationWorkFissionOpenUserInfo:            envBoolDefault("MOCHAT_GO_MIGRATE_OPERATION_WORK_FISSION_OPEN_USER_INFO", enableAllMigratedRoutes),
		MigrateWorkRoomGroupIndex:                          envBoolDefault("MOCHAT_GO_MIGRATE_WORK_ROOM_GROUP_INDEX", enableAllMigratedRoutes),
		MigrateWorkRoomGroupStore:                          envBoolDefault("MOCHAT_GO_MIGRATE_WORK_ROOM_GROUP_STORE", enableAllMigratedRoutes),
		MigrateWorkRoomGroupUpdate:                         envBoolDefault("MOCHAT_GO_MIGRATE_WORK_ROOM_GROUP_UPDATE", enableAllMigratedRoutes),
		MigrateWorkRoomGroupDestroy:                        envBoolDefault("MOCHAT_GO_MIGRATE_WORK_ROOM_GROUP_DESTROY", enableAllMigratedRoutes),
		MigrateSidebarContactTagAll:                        envBoolDefault("MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_TAG_ALL", enableAllMigratedRoutes),
		MigrateSidebarContactDetail:                        envBoolDefault("MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_DETAIL", enableAllMigratedRoutes),
		MigrateSidebarContactShow:                          envBoolDefault("MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_SHOW", enableAllMigratedRoutes),
		MigrateSidebarContactTrack:                         envBoolDefault("MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_TRACK", enableAllMigratedRoutes),
		MigrateSidebarContactUpdate:                        envBoolDefault("MOCHAT_GO_MIGRATE_SIDEBAR_WORK_CONTACT_UPDATE", enableAllMigratedRoutes),
		MigrateSidebarProcessStatus:                        envBoolDefault("MOCHAT_GO_MIGRATE_SIDEBAR_CONTACT_PROCESS_STATUS_INDEX", enableAllMigratedRoutes),
		MigrateSidebarProcessUpdate:                        envBoolDefault("MOCHAT_GO_MIGRATE_SIDEBAR_CONTACT_PROCESS_STATUS_UPDATE", enableAllMigratedRoutes),
		MigrateContactBatchAddDashboard:                    envBoolDefault("MOCHAT_GO_MIGRATE_CONTACT_BATCH_ADD_DASHBOARD", enableAllMigratedRoutes),
		MigrateSensitiveWordsDashboard:                     envBoolDefault("MOCHAT_GO_MIGRATE_SENSITIVE_WORDS_DASHBOARD", enableAllMigratedRoutes),
		MigrateContactSOPDashboard:                         envBoolDefault("MOCHAT_GO_MIGRATE_CONTACT_SOP_DASHBOARD", enableAllMigratedRoutes),
		MigrateRoomSOPDashboard:                            envBoolDefault("MOCHAT_GO_MIGRATE_ROOM_SOP_DASHBOARD", enableAllMigratedRoutes),
		MigrateShopCodeDashboard:                           envBoolDefault("MOCHAT_GO_MIGRATE_SHOP_CODE_DASHBOARD", enableAllMigratedRoutes),
		MigrateRadarDashboard:                              envBoolDefault("MOCHAT_GO_MIGRATE_RADAR_DASHBOARD", enableAllMigratedRoutes),
		MigrateAutoTagDashboard:                            envBoolDefault("MOCHAT_GO_MIGRATE_AUTO_TAG_DASHBOARD", enableAllMigratedRoutes),
		MigrateLotteryDashboard:                            envBoolDefault("MOCHAT_GO_MIGRATE_LOTTERY_DASHBOARD", enableAllMigratedRoutes),
		MigrateRoomFissionDashboard:                        envBoolDefault("MOCHAT_GO_MIGRATE_ROOM_FISSION_DASHBOARD", enableAllMigratedRoutes),
		MigrateRoomClockInDashboard:                        envBoolDefault("MOCHAT_GO_MIGRATE_ROOM_CLOCK_IN_DASHBOARD", enableAllMigratedRoutes),
		MigrateRoomQualityDashboard:                        envBoolDefault("MOCHAT_GO_MIGRATE_ROOM_QUALITY_DASHBOARD", enableAllMigratedRoutes),
		MigrateRoomCalendarDashboard:                       envBoolDefault("MOCHAT_GO_MIGRATE_ROOM_CALENDAR_DASHBOARD", enableAllMigratedRoutes),
		MigrateRoomRemindDashboard:                         envBoolDefault("MOCHAT_GO_MIGRATE_ROOM_REMIND_DASHBOARD", enableAllMigratedRoutes),
		MigrateRoomInfinitePullDashboard:                   envBoolDefault("MOCHAT_GO_MIGRATE_ROOM_INFINITE_PULL_DASHBOARD", enableAllMigratedRoutes),
		MigrateSidebarContactBatchAddDetail:                envBoolDefault("MOCHAT_GO_MIGRATE_SIDEBAR_CONTACT_BATCH_ADD_DETAIL", enableAllMigratedRoutes),
		MigrateMediumIndex:                                 envBoolDefault("MOCHAT_GO_MIGRATE_MEDIUM_INDEX", enableAllMigratedRoutes),
		MigrateMediumShow:                                  envBoolDefault("MOCHAT_GO_MIGRATE_MEDIUM_SHOW", enableAllMigratedRoutes),
		MigrateMediumStore:                                 envBoolDefault("MOCHAT_GO_MIGRATE_MEDIUM_STORE", enableAllMigratedRoutes),
		MigrateMediumUpdate:                                envBoolDefault("MOCHAT_GO_MIGRATE_MEDIUM_UPDATE", enableAllMigratedRoutes),
		MigrateMediumDestroy:                               envBoolDefault("MOCHAT_GO_MIGRATE_MEDIUM_DESTROY", enableAllMigratedRoutes),
		MigrateMediumItemGroupUpdate:                       envBoolDefault("MOCHAT_GO_MIGRATE_MEDIUM_ITEM_GROUP_UPDATE", enableAllMigratedRoutes),
		MigrateSidebarMediumIndex:                          envBoolDefault("MOCHAT_GO_MIGRATE_SIDEBAR_MEDIUM_INDEX", enableAllMigratedRoutes),
		MigrateSidebarMediumMediaIDUpdate:                  envBoolDefault("MOCHAT_GO_MIGRATE_SIDEBAR_MEDIUM_MEDIA_ID_UPDATE", enableAllMigratedRoutes),
		MigrateMediumGroupIndex:                            envBoolDefault("MOCHAT_GO_MIGRATE_MEDIUM_GROUP_INDEX", enableAllMigratedRoutes),
		MigrateMediumGroupStore:                            envBoolDefault("MOCHAT_GO_MIGRATE_MEDIUM_GROUP_STORE", enableAllMigratedRoutes),
		MigrateMediumGroupUpdate:                           envBoolDefault("MOCHAT_GO_MIGRATE_MEDIUM_GROUP_UPDATE", enableAllMigratedRoutes),
		MigrateMediumGroupDestroy:                          envBoolDefault("MOCHAT_GO_MIGRATE_MEDIUM_GROUP_DESTROY", enableAllMigratedRoutes),
		MigrateSidebarMediumGroupIndex:                     envBoolDefault("MOCHAT_GO_MIGRATE_SIDEBAR_MEDIUM_GROUP_INDEX", enableAllMigratedRoutes),
		MigrateFriendsCircleProvider:                       envBoolDefault("MOCHAT_GO_MIGRATE_FRIENDS_CIRCLE_PROVIDER", enableAllMigratedRoutes),
		MigrateChannelCodeIndex:                            envBoolDefault("MOCHAT_GO_MIGRATE_CHANNEL_CODE_INDEX", enableAllMigratedRoutes),
		MigrateChannelCodeShow:                             envBoolDefault("MOCHAT_GO_MIGRATE_CHANNEL_CODE_SHOW", enableAllMigratedRoutes),
		MigrateChannelCodeContact:                          envBoolDefault("MOCHAT_GO_MIGRATE_CHANNEL_CODE_CONTACT", enableAllMigratedRoutes),
		MigrateChannelCodeStatistics:                       envBoolDefault("MOCHAT_GO_MIGRATE_CHANNEL_CODE_STATISTICS", enableAllMigratedRoutes),
		MigrateChannelCodeStatsIndex:                       envBoolDefault("MOCHAT_GO_MIGRATE_CHANNEL_CODE_STATISTICS_INDEX", enableAllMigratedRoutes),
		MigrateChannelCodeStore:                            envBoolDefault("MOCHAT_GO_MIGRATE_CHANNEL_CODE_STORE", enableAllMigratedRoutes),
		MigrateChannelCodeUpdate:                           envBoolDefault("MOCHAT_GO_MIGRATE_CHANNEL_CODE_UPDATE", enableAllMigratedRoutes),
		MigrateChannelCodeGroupIndex:                       envBoolDefault("MOCHAT_GO_MIGRATE_CHANNEL_CODE_GROUP_INDEX", enableAllMigratedRoutes),
		MigrateChannelCodeGroupDetail:                      envBoolDefault("MOCHAT_GO_MIGRATE_CHANNEL_CODE_GROUP_DETAIL", enableAllMigratedRoutes),
		MigrateChannelCodeGroupStore:                       envBoolDefault("MOCHAT_GO_MIGRATE_CHANNEL_CODE_GROUP_STORE", enableAllMigratedRoutes),
		MigrateChannelCodeGroupUpdate:                      envBoolDefault("MOCHAT_GO_MIGRATE_CHANNEL_CODE_GROUP_UPDATE", enableAllMigratedRoutes),
		MigrateChannelCodeGroupMove:                        envBoolDefault("MOCHAT_GO_MIGRATE_CHANNEL_CODE_GROUP_MOVE", enableAllMigratedRoutes),
		MigrateGreetingIndex:                               envBoolDefault("MOCHAT_GO_MIGRATE_GREETING_INDEX", enableAllMigratedRoutes),
		MigrateGreetingShow:                                envBoolDefault("MOCHAT_GO_MIGRATE_GREETING_SHOW", enableAllMigratedRoutes),
		MigrateGreetingStore:                               envBoolDefault("MOCHAT_GO_MIGRATE_GREETING_STORE", enableAllMigratedRoutes),
		MigrateGreetingUpdate:                              envBoolDefault("MOCHAT_GO_MIGRATE_GREETING_UPDATE", enableAllMigratedRoutes),
		MigrateGreetingDestroy:                             envBoolDefault("MOCHAT_GO_MIGRATE_GREETING_DESTROY", enableAllMigratedRoutes),
		MigrateRoomWelcomeIndex:                            envBoolDefault("MOCHAT_GO_MIGRATE_ROOM_WELCOME_INDEX", enableAllMigratedRoutes),
		MigrateRoomWelcomeSelect:                           envBoolDefault("MOCHAT_GO_MIGRATE_ROOM_WELCOME_SELECT", enableAllMigratedRoutes),
		MigrateRoomWelcomeShow:                             envBoolDefault("MOCHAT_GO_MIGRATE_ROOM_WELCOME_SHOW", enableAllMigratedRoutes),
		MigrateRoomWelcomeStore:                            envBoolDefault("MOCHAT_GO_MIGRATE_ROOM_WELCOME_STORE", enableAllMigratedRoutes),
		MigrateRoomWelcomeUpdate:                           envBoolDefault("MOCHAT_GO_MIGRATE_ROOM_WELCOME_UPDATE", enableAllMigratedRoutes),
		MigrateRoomWelcomeDestroy:                          envBoolDefault("MOCHAT_GO_MIGRATE_ROOM_WELCOME_DESTROY", enableAllMigratedRoutes),
		MigrateContactFieldIndex:                           envBoolDefault("MOCHAT_GO_MIGRATE_CONTACT_FIELD_INDEX", enableAllMigratedRoutes),
		MigrateContactFieldShow:                            envBoolDefault("MOCHAT_GO_MIGRATE_CONTACT_FIELD_SHOW", enableAllMigratedRoutes),
		MigrateContactFieldPortrait:                        envBoolDefault("MOCHAT_GO_MIGRATE_CONTACT_FIELD_PORTRAIT", enableAllMigratedRoutes),
		MigrateContactFieldStore:                           envBoolDefault("MOCHAT_GO_MIGRATE_CONTACT_FIELD_STORE", enableAllMigratedRoutes),
		MigrateContactFieldUpdate:                          envBoolDefault("MOCHAT_GO_MIGRATE_CONTACT_FIELD_UPDATE", enableAllMigratedRoutes),
		MigrateContactFieldStatus:                          envBoolDefault("MOCHAT_GO_MIGRATE_CONTACT_FIELD_STATUS_UPDATE", enableAllMigratedRoutes),
		MigrateContactFieldDestroy:                         envBoolDefault("MOCHAT_GO_MIGRATE_CONTACT_FIELD_DESTROY", enableAllMigratedRoutes),
		MigrateContactFieldBatch:                           envBoolDefault("MOCHAT_GO_MIGRATE_CONTACT_FIELD_BATCH_UPDATE", enableAllMigratedRoutes),
		MigrateContactFieldPivot:                           envBoolDefault("MOCHAT_GO_MIGRATE_CONTACT_FIELD_PIVOT_INDEX", enableAllMigratedRoutes),
		MigrateContactFieldPivotUpdate:                     envBoolDefault("MOCHAT_GO_MIGRATE_CONTACT_FIELD_PIVOT_UPDATE", enableAllMigratedRoutes),
		MigrateSidebarFieldPivot:                           envBoolDefault("MOCHAT_GO_MIGRATE_SIDEBAR_CONTACT_FIELD_PIVOT_INDEX", enableAllMigratedRoutes),
		MigrateSidebarFieldPivotUpdate:                     envBoolDefault("MOCHAT_GO_MIGRATE_SIDEBAR_CONTACT_FIELD_PIVOT_UPDATE", enableAllMigratedRoutes),
		MigrateSidebarContactSOPInfo:                       envBoolDefault("MOCHAT_GO_MIGRATE_SIDEBAR_CONTACT_SOP_GET_SOP_INFO", enableAllMigratedRoutes),
		MigrateSidebarContactSOPTipInfo:                    envBoolDefault("MOCHAT_GO_MIGRATE_SIDEBAR_CONTACT_SOP_GET_SOP_TIP_INFO", enableAllMigratedRoutes),
		MigrateSidebarRoomSOPInfo:                          envBoolDefault("MOCHAT_GO_MIGRATE_SIDEBAR_ROOM_SOP_GET_SOP_INFO", enableAllMigratedRoutes),
		MigrateSidebarRoomSOPLogState:                      envBoolDefault("MOCHAT_GO_MIGRATE_SIDEBAR_ROOM_SOP_LOG_STATE", enableAllMigratedRoutes),
		MigrateChatToolConfig:                              envBoolDefault("MOCHAT_GO_MIGRATE_CHAT_TOOL_CONFIG", enableAllMigratedRoutes),
		MigrateCommonUpload:                                envBoolDefault("MOCHAT_GO_MIGRATE_COMMON_UPLOAD", enableAllMigratedRoutes),
		MigrateCommonUploadFile:                            envBoolDefault("MOCHAT_GO_MIGRATE_COMMON_UPLOAD_FILE", enableAllMigratedRoutes),
		MigrateSidebarCommonUpload:                         envBoolDefault("MOCHAT_GO_MIGRATE_SIDEBAR_COMMON_UPLOAD", enableAllMigratedRoutes),
		MigrateAgentTxtVerify:                              envBoolDefault("MOCHAT_GO_MIGRATE_AGENT_TXT_VERIFY", enableAllMigratedRoutes),
		MigrateAgentTxtUpload:                              envBoolDefault("MOCHAT_GO_MIGRATE_AGENT_TXT_VERIFY_UPLOAD", enableAllMigratedRoutes),
		MigrateAgentStore:                                  envBoolDefault("MOCHAT_GO_MIGRATE_AGENT_STORE", enableAllMigratedRoutes),
		MigrateSidebarAgentAuth:                            envBoolDefault("MOCHAT_GO_MIGRATE_SIDEBAR_AGENT_AUTH", enableAllMigratedRoutes),
		MigrateSidebarAgentOAuth:                           envBoolDefault("MOCHAT_GO_MIGRATE_SIDEBAR_AGENT_OAUTH", enableAllMigratedRoutes),
		MigrateSidebarAgentJSSDK:                           envBoolDefault("MOCHAT_GO_MIGRATE_SIDEBAR_AGENT_JSSDK_CONFIG", enableAllMigratedRoutes),
		MigrateSidebarWxJSSDK:                              envBoolDefault("MOCHAT_GO_MIGRATE_SIDEBAR_WX_JS_SDK_CONFIG", enableAllMigratedRoutes),
		MigrateRoleSelect:                                  envBoolDefault("MOCHAT_GO_MIGRATE_ROLE_SELECT", enableAllMigratedRoutes),
		MigrateRoleIndex:                                   envBoolDefault("MOCHAT_GO_MIGRATE_ROLE_INDEX", enableAllMigratedRoutes),
		MigrateRoleShow:                                    envBoolDefault("MOCHAT_GO_MIGRATE_ROLE_SHOW", enableAllMigratedRoutes),
		MigrateRolePermission:                              envBoolDefault("MOCHAT_GO_MIGRATE_ROLE_PERMISSION_SHOW", enableAllMigratedRoutes),
		MigrateRoleShowEmployee:                            envBoolDefault("MOCHAT_GO_MIGRATE_ROLE_SHOW_EMPLOYEE", enableAllMigratedRoutes),
		MigrateRoleStore:                                   envBoolDefault("MOCHAT_GO_MIGRATE_ROLE_STORE", enableAllMigratedRoutes),
		MigrateRoleUpdate:                                  envBoolDefault("MOCHAT_GO_MIGRATE_ROLE_UPDATE", enableAllMigratedRoutes),
		MigrateRoleStatusUpdate:                            envBoolDefault("MOCHAT_GO_MIGRATE_ROLE_STATUS_UPDATE", enableAllMigratedRoutes),
		MigrateRoleDestroy:                                 envBoolDefault("MOCHAT_GO_MIGRATE_ROLE_DESTROY", enableAllMigratedRoutes),
		MigrateRolePermissionStore:                         envBoolDefault("MOCHAT_GO_MIGRATE_ROLE_PERMISSION_STORE", enableAllMigratedRoutes),
		MigrateMenuIconIndex:                               envBoolDefault("MOCHAT_GO_MIGRATE_MENU_ICON_INDEX", enableAllMigratedRoutes),
		MigrateMenuSelect:                                  envBoolDefault("MOCHAT_GO_MIGRATE_MENU_SELECT", enableAllMigratedRoutes),
		MigrateMenuIndex:                                   envBoolDefault("MOCHAT_GO_MIGRATE_MENU_INDEX", enableAllMigratedRoutes),
		MigrateMenuShow:                                    envBoolDefault("MOCHAT_GO_MIGRATE_MENU_SHOW", enableAllMigratedRoutes),
		MigrateMenuStore:                                   envBoolDefault("MOCHAT_GO_MIGRATE_MENU_STORE", enableAllMigratedRoutes),
		MigrateMenuUpdate:                                  envBoolDefault("MOCHAT_GO_MIGRATE_MENU_UPDATE", enableAllMigratedRoutes),
		MigrateMenuStatusUpdate:                            envBoolDefault("MOCHAT_GO_MIGRATE_MENU_STATUS_UPDATE", enableAllMigratedRoutes),
		MigrateMenuDestroy:                                 envBoolDefault("MOCHAT_GO_MIGRATE_MENU_DESTROY", enableAllMigratedRoutes),
		EnableWeWorkCallbackWorker:                         envBool("MOCHAT_GO_ENABLE_WEWORK_CALLBACK_WORKER"),
		EnableEmployeeApplyWorker:                          envBool("MOCHAT_GO_ENABLE_EMPLOYEE_APPLY_WORKER"),
		EnableAsyncFileUploadWorker:                        envBool("MOCHAT_GO_ENABLE_ASYNC_FILE_UPLOAD_WORKER"),
		EnableMarkTagsWorker:                               envBool("MOCHAT_GO_ENABLE_MARK_TAGS_WORKER"),
		EnableMessageRemindWorker:                          envBool("MOCHAT_GO_ENABLE_MESSAGE_REMIND_WORKER"),
		EnableWorkRoomSyncWorker:                           envBool("MOCHAT_GO_ENABLE_WORK_ROOM_SYNC_WORKER"),
		EnableWorkContactSyncWorker:                        envBool("MOCHAT_GO_ENABLE_WORK_CONTACT_SYNC_WORKER"),
		EnableWorkDepartmentListWorker:                     envBool("MOCHAT_GO_ENABLE_WORK_DEPARTMENT_LIST_WORKER"),
		EnableMediaIDUpdateWorker:                          envBool("MOCHAT_GO_ENABLE_MEDIA_ID_UPDATE_WORKER"),
		EnableEmployeeStatisticWorker:                      envBool("MOCHAT_GO_ENABLE_EMPLOYEE_STATISTIC_WORKER"),
		DevAuthHeader:                                      envBool("MOCHAT_GO_DEV_AUTH_HEADER"),
		SkipJWTBlacklist:                                   envBool("MOCHAT_GO_SKIP_JWT_BLACKLIST"),
		ProxyTimeout:                                       30 * time.Second,
	}
	cfg.applyRuntimeRole()

	if raw := os.Getenv("MOCHAT_PROXY_TIMEOUT_SECONDS"); raw != "" {
		seconds, err := strconv.Atoi(raw)
		if err != nil || seconds <= 0 {
			return Config{}, fmt.Errorf("MOCHAT_PROXY_TIMEOUT_SECONDS must be a positive integer")
		}
		cfg.ProxyTimeout = time.Duration(seconds) * time.Second
	}

	if cfg.PHPUpstream != "" {
		parsed, err := url.Parse(cfg.PHPUpstream)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return Config{}, fmt.Errorf("MOCHAT_PHP_UPSTREAM must be an absolute URL")
		}
	}
	if err := requireAbsoluteURL("MOCHAT_API_BASE_URL or API_BASE_URL", cfg.APIBaseURL); err != nil {
		return Config{}, err
	}
	if err := requireAbsoluteURL("MOCHAT_DASHBOARD_BASE_URL or DASHBOARD_BASE_URL", cfg.DashboardBaseURL); err != nil {
		return Config{}, err
	}
	if err := requireAbsoluteURL("MOCHAT_SIDEBAR_BASE_URL or SIDEBAR_BASE_URL", cfg.SidebarBaseURL); err != nil {
		return Config{}, err
	}
	if err := requireAbsoluteURL("MOCHAT_OPERATION_BASE_URL or OPERATION_BASE_URL", cfg.OperationBaseURL); err != nil {
		return Config{}, err
	}
	if err := requireAbsoluteURL("MOCHAT_WECOM_API_BASE_URL", cfg.WeComAPIBaseURL); err != nil {
		return Config{}, err
	}
	if err := requireAbsoluteURL("MOCHAT_WECHAT_API_BASE_URL", cfg.WeChatAPIBaseURL); err != nil {
		return Config{}, err
	}
	if strings.TrimSpace(cfg.SaaSAlertWebhookURL) != "" {
		guard, err := outboundhttp.NewGuard(outboundhttp.Config{
			RequireHTTPS: cfg.SaaSAlertWebhookRequireHTTPS,
			AllowedCIDRs: cfg.SaaSAlertWebhookAllowedCIDRs,
		})
		if err != nil {
			return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_ALERT_WEBHOOK_ALLOWED_CIDRS: %w", err)
		}
		if err := guard.ValidateURL(cfg.SaaSAlertWebhookURL); err != nil {
			return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_ALERT_WEBHOOK_URL: %w", err)
		}
	}
	if _, err := saasalertcredentials.NewManager(saasalertcredentials.Config{
		EncryptionKey:       cfg.SaaSAlertCredentialEncryptionKey,
		EncryptionKeys:      cfg.SaaSAlertCredentialEncryptionKeys,
		EncryptionKeyID:     cfg.SaaSAlertCredentialEncryptionKeyID,
		RequireEncryption:   cfg.SaaSAlertCredentialRequireEncryption,
		DedicatedConfigured: cfg.SaaSAlertCredentialDedicatedConfigured,
	}); err != nil {
		return Config{}, fmt.Errorf("SaaS alert credential encryption configuration: %w", err)
	}
	if _, err := wecomcredentials.NewManager(wecomcredentials.Config{
		EncryptionKey:       cfg.WeComCredentialEncryptionKey,
		EncryptionKeys:      cfg.WeComCredentialEncryptionKeys,
		EncryptionKeyID:     cfg.WeComCredentialEncryptionKeyID,
		RequireEncryption:   cfg.WeComCredentialRequireEncryption,
		DedicatedConfigured: cfg.WeComCredentialDedicatedConfigured,
	}); err != nil {
		return Config{}, fmt.Errorf("WeCom credential encryption configuration: %w", err)
	}
	if _, err := wechatopencredentials.NewManager(wechatopencredentials.Config{
		EncryptionKey:       cfg.WeChatOpenCredentialEncryptionKey,
		EncryptionKeys:      cfg.WeChatOpenCredentialEncryptionKeys,
		EncryptionKeyID:     cfg.WeChatOpenCredentialEncryptionKeyID,
		RequireEncryption:   cfg.WeChatOpenCredentialRequireEncryption,
		DedicatedConfigured: cfg.WeChatOpenCredentialDedicatedConfigured,
	}); err != nil {
		return Config{}, fmt.Errorf("WeChat Open credential encryption configuration: %w", err)
	}
	if (cfg.EnableSaaSAdminDashboard || cfg.EnableSaaSOperationQueueAssignmentReminderCron || cfg.EnableSaaSApprovalReminderCron || cfg.EnableSaaSSystemHealthCron || cfg.EnableSaaSBackupCron || cfg.EnableSaaSComplianceCron || cfg.EnableSaaSAuditAnchorCron || cfg.EnableSaaSNotificationHealthRecoveryCron || cfg.EnableSaaSSubscriptionReconcileCron || cfg.EnableSaaSPaymentDunningCron || cfg.EnableSaaSPaymentSettlementSyncCron) && cfg.SaaSPlatformAdminTenantID <= 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_PLATFORM_ADMIN_TENANT_ID must be a positive integer when SaaS admin dashboard or a SaaS platform cron is enabled")
	}
	if cfg.SaaSReleaseSourceFingerprint != "" && !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(cfg.SaaSReleaseSourceFingerprint) {
		return Config{}, fmt.Errorf("release source fingerprint must be a 64-character SHA-256")
	}
	if strings.TrimSpace(cfg.SaaSBackupRoot) == "" {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_BACKUP_ROOT must not be empty")
	}
	if strings.TrimSpace(cfg.SaaSBackupDumpBinary) == "" || strings.TrimSpace(cfg.SaaSBackupRestoreBinary) == "" {
		return Config{}, fmt.Errorf("backup dump and restore binaries must not be empty")
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_]{3,64}$`).MatchString(cfg.SaaSBackupRestoreDatabasePrefix) {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_BACKUP_RESTORE_DATABASE_PREFIX must contain only letters, numbers, and underscores")
	}
	if cfg.EnableSaaSBackupCron && strings.TrimSpace(cfg.SaaSBackupEncryptionKey) == "" && strings.TrimSpace(cfg.SaaSBackupEncryptionKeys) == "" {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY or MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEYS is required when SaaS backup cron is enabled")
	}
	if strings.TrimSpace(cfg.SaaSComplianceArtifactRoot) == "" {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_COMPLIANCE_ARTIFACT_ROOT must not be empty")
	}
	serviceAccountDedicatedPepperConfigured := strings.TrimSpace(cfg.SaaSServiceAccountKeyPepper) != "" || strings.TrimSpace(cfg.SaaSServiceAccountKeyPeppers) != ""
	if cfg.SaaSServiceAccountRequireDedicatedPepper && !serviceAccountDedicatedPepperConfigured {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_SERVICE_ACCOUNT_KEY_PEPPER or MOCHAT_GO_SAAS_SERVICE_ACCOUNT_KEY_PEPPERS is required when dedicated service account pepper is enforced")
	}
	if !cfg.SaaSServiceAccountAllowLegacyJWTPepper && !serviceAccountDedicatedPepperConfigured && cfg.EnableSaaSAdminDashboard {
		return Config{}, fmt.Errorf("a dedicated service account pepper is required when legacy JWT pepper compatibility is disabled")
	}
	if cfg.EnableSaaSComplianceCron && strings.TrimSpace(cfg.SaaSComplianceEncryptionKey) == "" && strings.TrimSpace(cfg.SaaSComplianceEncryptionKeys) == "" {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY or MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEYS is required when SaaS compliance cron is enabled")
	}
	if strings.TrimSpace(cfg.SaaSAuditAnchorArtifactRoot) == "" {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_AUDIT_ANCHOR_ARTIFACT_ROOT must not be empty")
	}
	if cfg.EnableSaaSAuditAnchorCron && strings.TrimSpace(cfg.SaaSAuditAnchorHMACKey) == "" && strings.TrimSpace(cfg.SaaSAuditAnchorHMACKeys) == "" {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_AUDIT_ANCHOR_HMAC_KEY or MOCHAT_GO_SAAS_AUDIT_ANCHOR_HMAC_KEYS is required when SaaS audit anchor cron is enabled")
	}
	auditAnchorS3Required := []struct {
		name  string
		value string
	}{
		{"MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_ENDPOINT", cfg.SaaSAuditAnchorS3Endpoint},
		{"MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_BUCKET", cfg.SaaSAuditAnchorS3Bucket},
		{"MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_ACCESS_KEY_ID", cfg.SaaSAuditAnchorS3AccessKeyID},
		{"MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_SECRET_ACCESS_KEY", cfg.SaaSAuditAnchorS3SecretAccessKey},
	}
	auditAnchorS3Configured := 0
	for _, item := range auditAnchorS3Required {
		if strings.TrimSpace(item.value) != "" {
			auditAnchorS3Configured++
		}
	}
	if auditAnchorS3Configured != 0 && auditAnchorS3Configured != len(auditAnchorS3Required) {
		return Config{}, fmt.Errorf("audit anchor S3 endpoint, bucket, access key, and secret key must be configured together")
	}
	if cfg.SaaSAuditAnchorRequireRemote && auditAnchorS3Configured != len(auditAnchorS3Required) {
		return Config{}, fmt.Errorf("audit anchor S3 Object Lock storage is required when MOCHAT_GO_SAAS_AUDIT_ANCHOR_REQUIRE_REMOTE=1")
	}
	if cfg.SaaSAuditAnchorS3RetentionMode != "compliance" && cfg.SaaSAuditAnchorS3RetentionMode != "governance" {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_RETENTION_MODE must be compliance or governance")
	}
	if cfg.SaaSIdentityEnforceSessions && !cfg.EnableSaaSIdentitySecurity {
		return Config{}, fmt.Errorf("MOCHAT_GO_ENABLE_SAAS_IDENTITY_SECURITY is required when persistent identity sessions are enforced")
	}
	if cfg.EnableSaaSIdentityCleanupCron && !cfg.EnableSaaSIdentitySecurity {
		return Config{}, fmt.Errorf("MOCHAT_GO_ENABLE_SAAS_IDENTITY_SECURITY is required when identity cleanup cron is enabled")
	}
	if cfg.EnableSaaSIdentitySecurity && strings.TrimSpace(cfg.SaaSIdentityEncryptionKey) == "" && strings.TrimSpace(cfg.SaaSIdentityEncryptionKeys) == "" {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY or MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEYS is required when identity security is enabled")
	}
	if (cfg.SaaSIdentityTrustProxyHeaders || cfg.SaaSServiceAccountTrustProxyHeaders) && len(cfg.SaaSTrustedProxyCIDRs) == 0 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_TRUSTED_PROXY_CIDRS is required when trusted proxy headers are enabled")
	}
	if cfg.SaaSTenantDomainDNSServer != "" {
		if _, _, err := net.SplitHostPort(cfg.SaaSTenantDomainDNSServer); err != nil {
			return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_TENANT_DOMAIN_DNS_SERVER must use host:port: %w", err)
		}
	}
	domainDeliveryBridgeConfigured := strings.TrimSpace(cfg.SaaSTenantDomainDeliveryBridgeBaseURL) != ""
	if domainDeliveryBridgeConfigured {
		if err := requireAbsoluteURL("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_BRIDGE_BASE_URL", cfg.SaaSTenantDomainDeliveryBridgeBaseURL); err != nil {
			return Config{}, err
		}
		if len(cfg.SaaSTenantDomainDeliveryBridgeToken) < 16 {
			return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_BRIDGE_TOKEN must contain at least 16 characters when the bridge is configured")
		}
	}
	domainDeliveryCallbackConfigured := cfg.SaaSTenantDomainDeliveryCallbackURL != "" || cfg.SaaSTenantDomainDeliveryCallbackSecret != ""
	if domainDeliveryCallbackConfigured {
		if cfg.SaaSTenantDomainDeliveryCallbackURL == "" || cfg.SaaSTenantDomainDeliveryCallbackSecret == "" {
			return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CALLBACK_URL and MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CALLBACK_SECRET must be configured together")
		}
		if err := requireAbsoluteURL("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CALLBACK_URL", cfg.SaaSTenantDomainDeliveryCallbackURL); err != nil {
			return Config{}, err
		}
		if len(cfg.SaaSTenantDomainDeliveryCallbackSecret) < 32 {
			return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_TENANT_DOMAIN_DELIVERY_CALLBACK_SECRET must contain at least 32 characters")
		}
	}
	if cfg.EnableSaaSTenantDomainDeliveryCron && !domainDeliveryBridgeConfigured {
		return Config{}, fmt.Errorf("tenant domain delivery bridge URL is required when the delivery cron is enabled")
	}
	if cfg.SaaSBackupRestoreAutoProvision {
		if strings.TrimSpace(cfg.SaaSBackupRestoreAdminDSN) == "" {
			return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_BACKUP_RESTORE_ADMIN_DSN is required when automatic restore provisioning is enabled")
		}
		if strings.TrimSpace(cfg.SaaSBackupRestoreDSN) != "" {
			return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_BACKUP_RESTORE_DSN must be empty when automatic restore provisioning is enabled")
		}
	}
	s3Required := []struct {
		name  string
		value string
	}{
		{"MOCHAT_GO_SAAS_BACKUP_S3_ENDPOINT", cfg.SaaSBackupS3Endpoint},
		{"MOCHAT_GO_SAAS_BACKUP_S3_BUCKET", cfg.SaaSBackupS3Bucket},
		{"MOCHAT_GO_SAAS_BACKUP_S3_ACCESS_KEY_ID", cfg.SaaSBackupS3AccessKeyID},
		{"MOCHAT_GO_SAAS_BACKUP_S3_SECRET_ACCESS_KEY", cfg.SaaSBackupS3SecretAccessKey},
	}
	s3Configured := 0
	for _, item := range s3Required {
		if strings.TrimSpace(item.value) != "" {
			s3Configured++
		}
	}
	if s3Configured != 0 && s3Configured != len(s3Required) {
		return Config{}, fmt.Errorf("S3 backup replica endpoint, bucket, access key, and secret key must be configured together")
	}
	if cfg.EnableSaaSPaymentWebhook && len(strings.TrimSpace(cfg.SaaSPaymentWebhookSecret)) < 32 {
		return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_PAYMENT_WEBHOOK_SECRET must contain at least 32 characters when payment webhook is enabled")
	}
	if strings.TrimSpace(cfg.WorkMessageArchiveBridgeBaseURL) != "" {
		if err := requireAbsoluteURL("MOCHAT_GO_WORK_MESSAGE_ARCHIVE_BRIDGE_BASE_URL", cfg.WorkMessageArchiveBridgeBaseURL); err != nil {
			return Config{}, err
		}
	}
	if cfg.EnableWorkMessageArchiveSyncCron && strings.TrimSpace(cfg.WorkMessageArchiveBridgeBaseURL) == "" {
		return Config{}, fmt.Errorf("MOCHAT_GO_WORK_MESSAGE_ARCHIVE_BRIDGE_BASE_URL is required when work message archive sync cron is enabled")
	}
	settlementBridgeConfigured := strings.TrimSpace(cfg.SaaSPaymentSettlementBridgeBaseURL) != "" || len(cfg.SaaSPaymentSettlementProviders) > 0
	if settlementBridgeConfigured {
		if strings.TrimSpace(cfg.SaaSPaymentSettlementBridgeBaseURL) == "" || len(cfg.SaaSPaymentSettlementProviders) == 0 {
			return Config{}, fmt.Errorf("MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_BRIDGE_BASE_URL and MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_PROVIDERS must be configured together")
		}
		if err := requireAbsoluteURL("MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_BRIDGE_BASE_URL", cfg.SaaSPaymentSettlementBridgeBaseURL); err != nil {
			return Config{}, err
		}
	}
	if cfg.EnableSaaSPaymentSettlementSyncCron && !settlementBridgeConfigured {
		return Config{}, fmt.Errorf("payment settlement bridge URL and providers are required when payment settlement sync cron is enabled")
	}
	mysqlBacked := cfg.EnablePhase22SCRMPilot || cfg.EnableSaaSAlertDashboard || cfg.EnableSaaSAdminDashboard || cfg.EnableSaaSBillingPortal || cfg.EnableSaaSIdentitySecurity || cfg.EnableSaaSAlertNotificationDispatchCron || cfg.EnableSaaSOperationQueueAssignmentReminderCron || cfg.EnableSaaSApprovalReminderCron || cfg.EnableSaaSSystemHealthCron || cfg.EnableSaaSBackupCron || cfg.EnableSaaSComplianceCron || cfg.EnableSaaSIdentityCleanupCron || cfg.EnableSaaSServiceAccountUsageAlertCron || cfg.EnableSaaSAuditIntegrityCron || cfg.EnableSaaSAuditAnchorCron || cfg.EnableSaaSServiceAccountUsageCleanupCron || cfg.EnableSaaSTenantDomainDeliveryCron || cfg.EnableSaaSNotificationHealthRecoveryCron || cfg.EnableSaaSSubscriptionReconcileCron || cfg.EnableSaaSPaymentWebhook || cfg.EnableSaaSPaymentDunningCron || cfg.EnableSaaSPaymentSettlementSyncCron || cfg.MigrateAuth || cfg.MigrateLoginShow ||
		cfg.MigrateUserIndex || cfg.MigrateUserShow || cfg.MigrateUserStore || cfg.MigrateUserUpdate || cfg.MigrateUserStatusUpdate || cfg.MigrateUserPasswordReset || cfg.MigrateUserPasswordUpdate ||
		cfg.MigratePermissionByUser ||
		cfg.MigrateCorpSelect || cfg.MigrateCorpBind ||
		cfg.MigrateCorpIndex || cfg.MigrateCorpShow || cfg.MigrateCorpStore || cfg.MigrateCorpUpdate ||
		cfg.MigrateWeWorkCallback ||
		cfg.MigrateCorpDataIndex || cfg.MigrateCorpDataLineChat ||
		cfg.MigrateStatisticIndex || cfg.MigrateStatisticTopList || cfg.MigrateStatisticEmployeeCounts || cfg.MigrateStatisticEmployees || cfg.MigrateStatisticEmployeesTrend ||
		cfg.MigrateWorkEmployeeIndex || cfg.MigrateWorkEmployeeCond || cfg.MigrateWorkEmployeeSync || cfg.MigrateWorkDeptIndex || cfg.MigrateWorkDeptMember ||
		cfg.MigrateWorkDeptPhone || cfg.MigrateWorkDeptPage || cfg.MigrateWorkDeptEmployee ||
		cfg.MigrateWorkTagGroupIndex || cfg.MigrateWorkTagGroupDetail || cfg.MigrateWorkTagGroupStore || cfg.MigrateWorkTagGroupUpdate || cfg.MigrateWorkTagGroupDestroy || cfg.MigrateSidebarTagGroupIndex ||
		cfg.MigrateWorkContactTagIndex || cfg.MigrateWorkContactTagDetail || cfg.MigrateWorkContactTagList || cfg.MigrateWorkContactTagAll || cfg.MigrateWorkContactTagStore || cfg.MigrateWorkContactTagUpdate || cfg.MigrateWorkContactTagDestroy || cfg.MigrateWorkContactTagMove || cfg.MigrateWorkContactTagSync || cfg.MigrateWorkContactSync || cfg.MigrateWorkContactIndex || cfg.MigrateWorkContactLoss || cfg.MigrateWorkContactSource || cfg.MigrateWorkContactShow || cfg.MigrateWorkContactTrack || cfg.MigrateWorkContactUpdate || cfg.MigrateWorkContactBatchLabeling || cfg.MigrateWorkContactRoomIndex || cfg.MigrateWorkRoomIndex || cfg.MigrateWorkRoomRoomIndex || cfg.MigrateWorkRoomStatistics || cfg.MigrateWorkRoomStatisticsIndex || cfg.MigrateWorkRoomSync || cfg.MigrateWorkRoomBatchUpdate || cfg.MigrateSidebarWorkRoomManage ||
		cfg.MigrateContactTransferInfo || cfg.MigrateContactTransferUnassigned || cfg.MigrateContactTransferRoom || cfg.MigrateContactTransferLog || cfg.MigrateContactTransferSync || cfg.MigrateContactTransferCustomer || cfg.MigrateContactTransferRoomStore ||
		cfg.MigrateWorkRoomAutoPullIndex || cfg.MigrateWorkRoomAutoPullShow || cfg.MigrateWorkRoomAutoPullStore || cfg.MigrateWorkRoomAutoPullUpdate ||
		cfg.MigrateRoomTagPullIndex || cfg.MigrateRoomTagPullShow || cfg.MigrateRoomTagPullShowContact || cfg.MigrateRoomTagPullRoomList || cfg.MigrateRoomTagPullChooseContact || cfg.MigrateRoomTagPullStore || cfg.MigrateRoomTagPullFilterContact || cfg.MigrateRoomTagPullRemindSend || cfg.MigrateRoomTagPullDestroy ||
		cfg.MigrateContactMessageBatchSendIndex || cfg.MigrateContactMessageBatchSendShow || cfg.MigrateContactMessageBatchSendShowRoom || cfg.MigrateContactMessageBatchSendEmployee || cfg.MigrateContactMessageBatchSendContactReceive || cfg.MigrateContactMessageBatchSendStore || cfg.MigrateContactMessageBatchSendRemind || cfg.MigrateContactMessageBatchSendDestroy ||
		cfg.MigrateRoomMessageBatchSendIndex || cfg.MigrateRoomMessageBatchSendShow || cfg.MigrateRoomMessageBatchSendOwner || cfg.MigrateRoomMessageBatchSendRoomReceive || cfg.MigrateRoomMessageBatchSendStore || cfg.MigrateRoomMessageBatchSendRemind || cfg.MigrateRoomMessageBatchSendDestroy ||
		cfg.MigrateOfficialAccountIndex || cfg.MigrateOfficialAccountSet || cfg.MigrateOfficialAccountGetPreAuthURL || cfg.MigrateOfficialAccountAuthRedirect || cfg.MigrateOfficialAccountAuthEventCallback || cfg.MigrateLoad ||
		cfg.MigrateWorkFissionIndex || cfg.MigrateWorkFissionShow || cfg.MigrateWorkFissionInfo || cfg.MigrateWorkFissionStatistics || cfg.MigrateWorkFissionChooseContact || cfg.MigrateWorkFissionStore || cfg.MigrateWorkFissionUpdate || cfg.MigrateWorkFissionInvite || cfg.MigrateWorkFissionInviteData || cfg.MigrateWorkFissionInviteDetail || cfg.MigrateWorkFissionDestroy ||
		cfg.MigrateOperationWorkFissionInviteFriends || cfg.MigrateOperationWorkFissionPoster || cfg.MigrateOperationWorkFissionTaskData || cfg.MigrateOperationWorkFissionReceive || cfg.MigrateOperationWorkFissionAuth || cfg.MigrateOperationWorkFissionOpenUserInfo ||
		cfg.MigrateWorkRoomGroupIndex || cfg.MigrateWorkRoomGroupStore || cfg.MigrateWorkRoomGroupUpdate || cfg.MigrateWorkRoomGroupDestroy || cfg.MigrateSidebarContactTagAll || cfg.MigrateSidebarContactDetail || cfg.MigrateSidebarContactShow || cfg.MigrateSidebarContactTrack || cfg.MigrateSidebarContactUpdate || cfg.MigrateSidebarProcessStatus || cfg.MigrateSidebarProcessUpdate || cfg.MigrateContactBatchAddDashboard || cfg.MigrateSensitiveWordsDashboard || cfg.MigrateContactSOPDashboard || cfg.MigrateRoomSOPDashboard || cfg.MigrateShopCodeDashboard || cfg.MigrateRadarDashboard || cfg.MigrateAutoTagDashboard || cfg.MigrateLotteryDashboard || cfg.MigrateRoomFissionDashboard || cfg.MigrateRoomClockInDashboard || cfg.MigrateRoomQualityDashboard || cfg.MigrateRoomCalendarDashboard || cfg.MigrateRoomRemindDashboard || cfg.MigrateRoomInfinitePullDashboard || cfg.MigrateSidebarContactBatchAddDetail ||
		cfg.MigrateMediumIndex || cfg.MigrateMediumShow || cfg.MigrateMediumStore || cfg.MigrateMediumUpdate || cfg.MigrateMediumDestroy || cfg.MigrateMediumItemGroupUpdate || cfg.MigrateSidebarMediumIndex || cfg.MigrateSidebarMediumMediaIDUpdate ||
		cfg.MigrateMediumGroupIndex || cfg.MigrateMediumGroupStore || cfg.MigrateMediumGroupUpdate || cfg.MigrateMediumGroupDestroy || cfg.MigrateSidebarMediumGroupIndex || cfg.MigrateFriendsCircleProvider ||
		cfg.MigrateChannelCodeIndex || cfg.MigrateChannelCodeShow || cfg.MigrateChannelCodeContact || cfg.MigrateChannelCodeStatistics || cfg.MigrateChannelCodeStatsIndex || cfg.MigrateChannelCodeStore || cfg.MigrateChannelCodeUpdate ||
		cfg.MigrateChannelCodeGroupIndex || cfg.MigrateChannelCodeGroupDetail || cfg.MigrateChannelCodeGroupStore || cfg.MigrateChannelCodeGroupUpdate || cfg.MigrateChannelCodeGroupMove ||
		cfg.MigrateGreetingIndex || cfg.MigrateGreetingShow || cfg.MigrateGreetingStore || cfg.MigrateGreetingUpdate || cfg.MigrateGreetingDestroy ||
		cfg.MigrateRoomWelcomeIndex || cfg.MigrateRoomWelcomeSelect || cfg.MigrateRoomWelcomeShow || cfg.MigrateRoomWelcomeStore || cfg.MigrateRoomWelcomeUpdate || cfg.MigrateRoomWelcomeDestroy ||
		cfg.MigrateContactFieldIndex || cfg.MigrateContactFieldShow || cfg.MigrateContactFieldPortrait ||
		cfg.MigrateContactFieldStore || cfg.MigrateContactFieldUpdate || cfg.MigrateContactFieldStatus || cfg.MigrateContactFieldDestroy || cfg.MigrateContactFieldBatch ||
		cfg.MigrateContactFieldPivot || cfg.MigrateContactFieldPivotUpdate || cfg.MigrateSidebarFieldPivot || cfg.MigrateSidebarFieldPivotUpdate || cfg.MigrateSidebarContactSOPInfo || cfg.MigrateSidebarContactSOPTipInfo || cfg.MigrateSidebarRoomSOPInfo || cfg.MigrateSidebarRoomSOPLogState ||
		cfg.MigrateChatToolConfig || cfg.MigrateAgentStore || cfg.MigrateSidebarAgentAuth || cfg.MigrateSidebarAgentOAuth || cfg.MigrateSidebarAgentJSSDK || cfg.MigrateSidebarWxJSSDK || cfg.MigrateRoleSelect ||
		cfg.MigrateRoleIndex || cfg.MigrateRoleShow ||
		cfg.MigrateRolePermission || cfg.MigrateRoleShowEmployee ||
		cfg.MigrateRoleStore || cfg.MigrateRoleUpdate ||
		cfg.MigrateRoleStatusUpdate || cfg.MigrateRoleDestroy ||
		cfg.MigrateRolePermissionStore ||
		cfg.MigrateMenuIconIndex || cfg.MigrateMenuSelect ||
		cfg.MigrateMenuIndex || cfg.MigrateMenuShow ||
		cfg.MigrateMenuStore || cfg.MigrateMenuUpdate ||
		cfg.MigrateMenuStatusUpdate || cfg.MigrateMenuDestroy
	jwtBackedRead := cfg.EnableSaaSAlertDashboard || cfg.EnableSaaSAdminDashboard || cfg.EnableSaaSBillingPortal || cfg.MigrateLoginShow || cfg.MigrateUserIndex || cfg.MigrateUserShow || cfg.MigratePermissionByUser ||
		cfg.MigrateCorpSelect || cfg.MigrateCorpIndex || cfg.MigrateCorpShow ||
		cfg.MigrateCorpDataIndex || cfg.MigrateCorpDataLineChat ||
		cfg.MigrateStatisticIndex || cfg.MigrateStatisticTopList || cfg.MigrateStatisticEmployeeCounts || cfg.MigrateStatisticEmployees || cfg.MigrateStatisticEmployeesTrend ||
		cfg.MigrateWorkEmployeeIndex || cfg.MigrateWorkEmployeeCond || cfg.MigrateWorkEmployeeSync || cfg.MigrateWorkDeptIndex || cfg.MigrateWorkDeptMember ||
		cfg.MigrateWorkDeptPhone || cfg.MigrateWorkDeptPage || cfg.MigrateWorkDeptEmployee ||
		cfg.MigrateWorkTagGroupIndex || cfg.MigrateWorkTagGroupDetail ||
		cfg.MigrateWorkContactTagIndex || cfg.MigrateWorkContactTagDetail || cfg.MigrateWorkContactTagList || cfg.MigrateWorkContactTagAll || cfg.MigrateWorkContactIndex || cfg.MigrateWorkContactLoss || cfg.MigrateWorkContactSource || cfg.MigrateWorkContactShow || cfg.MigrateWorkContactTrack || cfg.MigrateWorkContactRoomIndex || cfg.MigrateWorkRoomIndex || cfg.MigrateWorkRoomRoomIndex || cfg.MigrateWorkRoomStatistics || cfg.MigrateWorkRoomStatisticsIndex ||
		cfg.MigrateContactTransferInfo || cfg.MigrateContactTransferUnassigned || cfg.MigrateContactTransferRoom || cfg.MigrateContactTransferLog ||
		cfg.MigrateWorkRoomAutoPullIndex || cfg.MigrateWorkRoomAutoPullShow || cfg.MigrateRoomTagPullIndex || cfg.MigrateRoomTagPullShow || cfg.MigrateRoomTagPullShowContact || cfg.MigrateRoomTagPullRoomList || cfg.MigrateRoomTagPullChooseContact ||
		cfg.MigrateContactMessageBatchSendIndex || cfg.MigrateContactMessageBatchSendShow || cfg.MigrateContactMessageBatchSendShowRoom || cfg.MigrateContactMessageBatchSendEmployee || cfg.MigrateContactMessageBatchSendContactReceive ||
		cfg.MigrateRoomMessageBatchSendIndex || cfg.MigrateRoomMessageBatchSendShow || cfg.MigrateRoomMessageBatchSendOwner || cfg.MigrateRoomMessageBatchSendRoomReceive ||
		cfg.MigrateOfficialAccountIndex || cfg.MigrateOfficialAccountGetPreAuthURL ||
		cfg.MigrateWorkFissionIndex || cfg.MigrateWorkFissionShow || cfg.MigrateWorkFissionInfo || cfg.MigrateWorkFissionStatistics || cfg.MigrateWorkFissionChooseContact || cfg.MigrateWorkFissionInviteData || cfg.MigrateWorkFissionInviteDetail ||
		cfg.MigrateWorkRoomGroupIndex ||
		cfg.MigrateContactBatchAddDashboard || cfg.MigrateSensitiveWordsDashboard || cfg.MigrateContactSOPDashboard || cfg.MigrateRoomSOPDashboard || cfg.MigrateShopCodeDashboard || cfg.MigrateRadarDashboard || cfg.MigrateAutoTagDashboard || cfg.MigrateLotteryDashboard || cfg.MigrateRoomFissionDashboard || cfg.MigrateRoomClockInDashboard || cfg.MigrateRoomQualityDashboard || cfg.MigrateRoomCalendarDashboard || cfg.MigrateRoomRemindDashboard || cfg.MigrateRoomInfinitePullDashboard ||
		cfg.MigrateMediumIndex || cfg.MigrateMediumShow || cfg.MigrateMediumGroupIndex || cfg.MigrateFriendsCircleProvider ||
		cfg.MigrateChannelCodeIndex || cfg.MigrateChannelCodeShow || cfg.MigrateChannelCodeContact || cfg.MigrateChannelCodeStatistics || cfg.MigrateChannelCodeStatsIndex ||
		cfg.MigrateChannelCodeGroupIndex || cfg.MigrateChannelCodeGroupDetail ||
		cfg.MigrateGreetingIndex || cfg.MigrateGreetingShow ||
		cfg.MigrateRoomWelcomeIndex || cfg.MigrateRoomWelcomeSelect || cfg.MigrateRoomWelcomeShow ||
		cfg.MigrateContactFieldIndex || cfg.MigrateContactFieldShow || cfg.MigrateContactFieldPortrait || cfg.MigrateContactFieldPivot ||
		cfg.MigrateChatToolConfig || cfg.MigrateRoleSelect ||
		cfg.MigrateRoleIndex || cfg.MigrateRoleShow ||
		cfg.MigrateRolePermission || cfg.MigrateRoleShowEmployee ||
		cfg.MigrateMenuIconIndex || cfg.MigrateMenuSelect ||
		cfg.MigrateMenuIndex || cfg.MigrateMenuShow
	dashboardStateChanging := cfg.EnablePhase22SCRMPilot || cfg.EnableSaaSAlertDashboard || cfg.EnableSaaSBillingPortal || cfg.MigrateLogout || cfg.MigrateUserStore || cfg.MigrateUserUpdate || cfg.MigrateUserStatusUpdate || cfg.MigrateUserPasswordReset || cfg.MigrateUserPasswordUpdate ||
		cfg.MigrateCommonUpload || cfg.MigrateCommonUploadFile ||
		cfg.MigrateCorpBind || cfg.MigrateCorpStore || cfg.MigrateCorpUpdate ||
		cfg.MigrateWorkTagGroupStore || cfg.MigrateWorkTagGroupUpdate || cfg.MigrateWorkTagGroupDestroy ||
		cfg.MigrateWorkEmployeeSync ||
		cfg.MigrateWorkContactTagStore || cfg.MigrateWorkContactTagUpdate || cfg.MigrateWorkContactTagDestroy || cfg.MigrateWorkContactTagMove || cfg.MigrateWorkContactTagSync || cfg.MigrateWorkContactSync ||
		cfg.MigrateWorkContactUpdate || cfg.MigrateWorkContactBatchLabeling || cfg.MigrateWorkRoomSync || cfg.MigrateWorkRoomBatchUpdate ||
		cfg.MigrateContactTransferSync || cfg.MigrateContactTransferCustomer || cfg.MigrateContactTransferRoomStore ||
		cfg.MigrateWorkRoomAutoPullStore || cfg.MigrateWorkRoomAutoPullUpdate ||
		cfg.MigrateRoomTagPullStore || cfg.MigrateRoomTagPullFilterContact || cfg.MigrateRoomTagPullRemindSend || cfg.MigrateRoomTagPullDestroy ||
		cfg.MigrateContactMessageBatchSendStore || cfg.MigrateContactMessageBatchSendRemind || cfg.MigrateContactMessageBatchSendDestroy ||
		cfg.MigrateRoomMessageBatchSendStore || cfg.MigrateRoomMessageBatchSendRemind || cfg.MigrateRoomMessageBatchSendDestroy ||
		cfg.MigrateOfficialAccountSet ||
		cfg.MigrateWorkFissionStore || cfg.MigrateWorkFissionUpdate || cfg.MigrateWorkFissionInvite || cfg.MigrateWorkFissionDestroy ||
		cfg.MigrateWorkRoomGroupStore || cfg.MigrateWorkRoomGroupUpdate || cfg.MigrateWorkRoomGroupDestroy ||
		cfg.MigrateContactBatchAddDashboard || cfg.MigrateSensitiveWordsDashboard || cfg.MigrateContactSOPDashboard || cfg.MigrateRoomSOPDashboard || cfg.MigrateShopCodeDashboard || cfg.MigrateRadarDashboard || cfg.MigrateAutoTagDashboard || cfg.MigrateLotteryDashboard || cfg.MigrateRoomFissionDashboard || cfg.MigrateRoomClockInDashboard || cfg.MigrateRoomQualityDashboard || cfg.MigrateRoomCalendarDashboard || cfg.MigrateRoomRemindDashboard || cfg.MigrateRoomInfinitePullDashboard ||
		cfg.MigrateMediumStore || cfg.MigrateMediumUpdate || cfg.MigrateMediumDestroy || cfg.MigrateMediumItemGroupUpdate ||
		cfg.MigrateMediumGroupStore || cfg.MigrateMediumGroupUpdate || cfg.MigrateMediumGroupDestroy ||
		cfg.MigrateChannelCodeStore || cfg.MigrateChannelCodeUpdate || cfg.MigrateChannelCodeGroupStore || cfg.MigrateChannelCodeGroupUpdate || cfg.MigrateChannelCodeGroupMove ||
		cfg.MigrateGreetingStore || cfg.MigrateGreetingUpdate || cfg.MigrateGreetingDestroy ||
		cfg.MigrateRoomWelcomeStore || cfg.MigrateRoomWelcomeUpdate || cfg.MigrateRoomWelcomeDestroy ||
		cfg.MigrateContactFieldStore || cfg.MigrateContactFieldUpdate || cfg.MigrateContactFieldStatus || cfg.MigrateContactFieldDestroy || cfg.MigrateContactFieldBatch || cfg.MigrateContactFieldPivotUpdate ||
		cfg.MigrateAgentStore ||
		cfg.MigrateRoleStore || cfg.MigrateRoleUpdate || cfg.MigrateRoleStatusUpdate || cfg.MigrateRoleDestroy || cfg.MigrateRolePermissionStore ||
		cfg.MigrateMenuStore || cfg.MigrateMenuUpdate || cfg.MigrateMenuStatusUpdate || cfg.MigrateMenuDestroy
	stateChanging := dashboardStateChanging || cfg.MigrateSidebarContactUpdate || cfg.MigrateSidebarProcessUpdate || cfg.MigrateSidebarCommonUpload || cfg.MigrateSidebarFieldPivotUpdate || cfg.MigrateSidebarMediumMediaIDUpdate || cfg.MigrateSidebarRoomSOPLogState
	operationSessionBacked := cfg.MigrateOperationWorkFissionAuth || cfg.MigrateOperationWorkFissionOpenUserInfo || cfg.MigrateLoad || cfg.MigrateLotteryDashboard || cfg.MigrateRoomClockInDashboard || cfg.MigrateRoomFissionDashboard || cfg.MigrateShopCodeDashboard

	jwtWorkerEnabled := cfg.EnableWeWorkCallbackWorker || cfg.EnableEmployeeApplyWorker || cfg.EnableWorkDepartmentListWorker
	mysqlWorkerEnabled := jwtWorkerEnabled || cfg.EnableMarkTagsWorker || cfg.EnableMessageRemindWorker || cfg.EnableWorkRoomSyncWorker || cfg.EnableWorkContactSyncWorker || cfg.EnableMediaIDUpdateWorker || cfg.EnableEmployeeStatisticWorker
	redisWorkerEnabled := mysqlWorkerEnabled || cfg.EnableAsyncFileUploadWorker
	cronEnabled := cfg.EnablePullAgentCron || cfg.EnableEmployeeStatisticCron || cfg.EnableChannelCodeCron || cfg.EnableContactBatchSendCron || cfg.EnableRoomBatchSendCron || cfg.EnableContactSyncSendResultCron || cfg.EnableRoomSyncSendResultCron || cfg.EnableRoomTagPullCron || cfg.EnableCorpDataCron || cfg.EnableMediaIDUpdateCron || cfg.EnableTransferStateRefreshCron || cfg.EnableSOPLogCron || cfg.EnableSensitiveWordMonitorCron || cfg.EnableWorkMessageArchiveSyncCron || cfg.EnableSaaSStorageReconcileCron || cfg.EnableSaaSPaymentSettlementSyncCron
	if (mysqlBacked || mysqlWorkerEnabled || cronEnabled) && cfg.MySQLDSN == "" {
		return Config{}, fmt.Errorf("MOCHAT_MYSQL_DSN is required when migrated MySQL-backed routes, MySQL-backed Go workers, or Go cron tasks are enabled")
	}
	if jwtWorkerEnabled && cfg.SimpleJWTSecret == "" {
		return Config{}, fmt.Errorf("MOCHAT_SIMPLE_JWT_SECRET or SIMPLE_JWT_SECRET is required when JWT-backed Go workers are enabled")
	}
	if (cfg.MigrateAuth || ((jwtBackedRead || dashboardStateChanging) && !cfg.DevAuthHeader)) && cfg.SimpleJWTSecret == "" {
		return Config{}, fmt.Errorf("MOCHAT_SIMPLE_JWT_SECRET or SIMPLE_JWT_SECRET is required when migrated auth routes use PHP JWT auth")
	}
	if cfg.MigrateSidebarAgentOAuth && cfg.SimpleJWTSecret == "" {
		return Config{}, fmt.Errorf("MOCHAT_SIMPLE_JWT_SECRET or SIMPLE_JWT_SECRET is required when sidebar agent oauth signs dashboard JWT")
	}
	if cfg.MigrateSidebarAgentAuth && cfg.SidebarJWTSecret == "" {
		return Config{}, fmt.Errorf("MOCHAT_SIDEBAR_JWT_SECRET or SIDEBAR_JWT_SECRET is required when sidebar agent auth signs sidebar JWT")
	}
	sidebarBackedRead := cfg.MigrateSidebarTagGroupIndex || cfg.MigrateSidebarContactTagAll || cfg.MigrateSidebarContactDetail || cfg.MigrateSidebarContactShow || cfg.MigrateSidebarContactTrack || cfg.MigrateSidebarContactUpdate || cfg.MigrateSidebarWorkRoomManage || cfg.MigrateSidebarProcessStatus || cfg.MigrateSidebarProcessUpdate || cfg.MigrateSidebarContactBatchAddDetail || cfg.MigrateSidebarMediumIndex || cfg.MigrateSidebarMediumMediaIDUpdate || cfg.MigrateSidebarMediumGroupIndex || cfg.MigrateSidebarFieldPivot || cfg.MigrateSidebarFieldPivotUpdate || cfg.MigrateSidebarContactSOPInfo || cfg.MigrateSidebarContactSOPTipInfo || cfg.MigrateSidebarRoomSOPInfo || cfg.MigrateSidebarRoomSOPLogState || cfg.MigrateSidebarCommonUpload || cfg.MigrateSidebarAgentJSSDK
	if sidebarBackedRead && !cfg.DevAuthHeader && cfg.SidebarJWTSecret == "" {
		return Config{}, fmt.Errorf("MOCHAT_SIDEBAR_JWT_SECRET or SIDEBAR_JWT_SECRET is required when migrated sidebar routes use PHP sidebar JWT auth")
	}
	if (jwtBackedRead || dashboardStateChanging) && !cfg.DevAuthHeader && cfg.RedisAddr == "" && !cfg.SkipJWTBlacklist {
		return Config{}, fmt.Errorf("MOCHAT_REDIS_ADDR or REDIS_HOST/REDIS_PORT is required for JWT blacklist checks")
	}
	if sidebarBackedRead && !cfg.DevAuthHeader && cfg.RedisAddr == "" && !cfg.SkipJWTBlacklist {
		return Config{}, fmt.Errorf("MOCHAT_REDIS_ADDR or REDIS_HOST/REDIS_PORT is required for sidebar JWT blacklist checks")
	}
	if stateChanging && cfg.DevAuthHeader {
		return Config{}, fmt.Errorf("state-changing migrated routes require PHP JWT auth; unset MOCHAT_GO_DEV_AUTH_HEADER")
	}
	if stateChanging && cfg.SkipJWTBlacklist {
		return Config{}, fmt.Errorf("state-changing migrated routes cannot skip JWT blacklist checks")
	}
	if stateChanging && cfg.RedisAddr == "" {
		return Config{}, fmt.Errorf("MOCHAT_REDIS_ADDR or REDIS_HOST/REDIS_PORT is required when state-changing migrated routes are enabled")
	}
	if operationSessionBacked && cfg.RedisAddr == "" {
		return Config{}, fmt.Errorf("MOCHAT_REDIS_ADDR or REDIS_HOST/REDIS_PORT is required when operation session routes are enabled")
	}
	if redisWorkerEnabled && cfg.RedisAddr == "" {
		return Config{}, fmt.Errorf("MOCHAT_REDIS_ADDR or REDIS_HOST/REDIS_PORT is required when Go workers are enabled")
	}
	if cfg.EnableTransferStateRefreshCron && cfg.RedisAddr == "" {
		return Config{}, fmt.Errorf("MOCHAT_REDIS_ADDR or REDIS_HOST/REDIS_PORT is required when TransferStateRefresh cron is enabled")
	}
	if cfg.EnableEmployeeStatisticCron && cfg.RedisAddr == "" {
		return Config{}, fmt.Errorf("MOCHAT_REDIS_ADDR or REDIS_HOST/REDIS_PORT is required when employeeStatistic cron is enabled")
	}

	return cfg, nil
}

func (cfg *Config) applyRuntimeRole() {
	if !cfg.RuntimeRole.RunsWorkers() {
		cfg.EnableWeWorkCallbackWorker = false
		cfg.EnableEmployeeApplyWorker = false
		cfg.EnableAsyncFileUploadWorker = false
		cfg.EnableMarkTagsWorker = false
		cfg.EnableMessageRemindWorker = false
		cfg.EnableWorkRoomSyncWorker = false
		cfg.EnableWorkContactSyncWorker = false
		cfg.EnableWorkDepartmentListWorker = false
		cfg.EnableMediaIDUpdateWorker = false
		cfg.EnableEmployeeStatisticWorker = false
	}
	if !cfg.RuntimeRole.RunsScheduler() {
		cfg.EnablePullAgentCron = false
		cfg.EnableEmployeeStatisticCron = false
		cfg.EnableChannelCodeCron = false
		cfg.EnableContactBatchSendCron = false
		cfg.EnableRoomBatchSendCron = false
		cfg.EnableContactSyncSendResultCron = false
		cfg.EnableRoomSyncSendResultCron = false
		cfg.EnableRoomTagPullCron = false
		cfg.EnableCorpDataCron = false
		cfg.EnableMediaIDUpdateCron = false
		cfg.EnableTransferStateRefreshCron = false
		cfg.EnableSOPLogCron = false
		cfg.EnableSensitiveWordMonitorCron = false
		cfg.EnableWorkMessageArchiveSyncCron = false
		cfg.EnableSaaSStorageReconcileCron = false
		cfg.EnableSaaSAlertNotificationDispatchCron = false
		cfg.EnableSaaSOperationQueueAssignmentReminderCron = false
		cfg.EnableSaaSApprovalReminderCron = false
		cfg.EnableSaaSSystemHealthCron = false
		cfg.EnableSaaSBackupCron = false
		cfg.EnableSaaSComplianceCron = false
		cfg.EnableSaaSIdentityCleanupCron = false
		cfg.EnableSaaSServiceAccountUsageAlertCron = false
		cfg.EnableSaaSAuditIntegrityCron = false
		cfg.EnableSaaSAuditAnchorCron = false
		cfg.EnableSaaSServiceAccountUsageCleanupCron = false
		cfg.EnableSaaSTenantDomainDeliveryCron = false
		cfg.EnableSaaSNotificationHealthRecoveryCron = false
		cfg.EnableSaaSSubscriptionReconcileCron = false
		cfg.EnableSaaSPaymentDunningCron = false
		cfg.EnableSaaSPaymentSettlementSyncCron = false
	}
}

func offsetListenAddr(listenAddr string, offset int) string {
	host, portText, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return ""
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return ""
	}
	port += offset
	if port <= 0 || port > 65535 {
		return ""
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

func baseURLFromListenAddr(listenAddr string) string {
	host, portText, err := net.SplitHostPort(listenAddr)
	if err != nil || portText == "" {
		return ""
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, portText)
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func envBool(key string) bool {
	value := os.Getenv(key)
	return value == "1" || value == "true" || value == "TRUE" || value == "yes" || value == "YES"
}

func envBoolDefault(key string, defaultValue bool) bool {
	if os.Getenv(key) == "" {
		return defaultValue
	}
	return envBool(key)
}

func requireAbsoluteURL(name string, value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%s must be an absolute URL", name)
	}
	return nil
}

func validateSaaSAlertWebhookTemplate(name string, raw string) error {
	tmpl, err := template.New(name).Option("missingkey=error").Parse(raw)
	if err != nil {
		return fmt.Errorf("%s must be a valid Go text/template: %w", name, err)
	}
	sample := map[string]any{
		"Event":           "saas.quota_alert",
		"AlertType":       "quota_exceeded",
		"Severity":        "warning",
		"PeriodKey":       "lifetime",
		"Source":          "worker.queue_item",
		"Message":         "套餐额度已达上限：异步执行量 2/1",
		"TenantID":        1,
		"Metric":          "async_executions",
		"CurrentValue":    int64(2),
		"LimitValue":      int64(1),
		"AdditionalValue": int64(0),
		"OccurredAt":      time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
		"Context":         map[string]any{"nonBlocking": true},
	}
	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, sample); err != nil {
		return fmt.Errorf("%s contains unsupported SaaS alert template field: %w", name, err)
	}
	return nil
}

func envFirst(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}

func envInt(primary string, fallback string, defaultValue int) (int, error) {
	raw := envFirst(primary, fallback)
	if raw == "" {
		return defaultValue, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0, fmt.Errorf("%s or %s must be a non-negative integer", primary, fallback)
	}
	return value, nil
}

var paymentSettlementProviderPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]*$`)

func paymentSettlementProvidersFromEnv(raw string) ([]string, error) {
	seen := map[string]struct{}{}
	providers := make([]string, 0)
	for _, value := range strings.Split(raw, ",") {
		provider := strings.ToLower(strings.TrimSpace(value))
		if provider == "" {
			continue
		}
		if len(provider) > 32 || !paymentSettlementProviderPattern.MatchString(provider) {
			return nil, fmt.Errorf("MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_PROVIDERS contains invalid provider %q", value)
		}
		if _, exists := seen[provider]; exists {
			continue
		}
		seen[provider] = struct{}{}
		providers = append(providers, provider)
	}
	return providers, nil
}

func redisAddrFromEnv() string {
	if value := os.Getenv("MOCHAT_REDIS_ADDR"); value != "" {
		return value
	}
	host := envOrDefault("REDIS_HOST", "localhost")
	port := envOrDefault("REDIS_PORT", "6379")
	if host == "" || port == "" {
		return ""
	}
	return net.JoinHostPort(host, port)
}
