package server

import (
	"context"
	_ "embed"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/config"
	"jiyi/mochat-go/internal/nilcheck"
	"jiyi/mochat-go/internal/taskrunner"
)

//go:embed compat_manifest_embedded.json
var embeddedCompatManifest []byte

type Server struct {
	cfg                                             config.Config
	startedAt                                       time.Time
	proxy                                           http.Handler
	moduleRouter                                    ModuleRouter
	dashboardRequestGuard                           DashboardRequestGuard
	saasRequestGuard                                SaaSRequestGuard
	dashboardAccess                                 http.Handler
	saasAuth                                        http.Handler
	saasLoginPage                                   http.Handler
	dashboardAuth                                   http.Handler
	companyProfile                                  http.Handler
	providerStatus                                  http.Handler
	identitySelf                                    http.Handler
	logout                                          http.Handler
	userIndex                                       http.Handler
	userShow                                        http.Handler
	userStore                                       http.Handler
	userUpdate                                      http.Handler
	userStatusUpdate                                http.Handler
	userPasswordReset                               http.Handler
	userPasswordUpdate                              http.Handler
	permissionByUser                                http.Handler
	weWorkCallback                                  http.Handler
	corpDataIndex                                   http.Handler
	corpDataLineChat                                http.Handler
	externalTenantIndex                             http.Handler
	statisticIndex                                  http.Handler
	statisticTopList                                http.Handler
	statisticEmployeeCounts                         http.Handler
	statisticEmployees                              http.Handler
	statisticEmployeesTrend                         http.Handler
	workEmployeeIndex                               http.Handler
	workEmployeeCond                                http.Handler
	workEmployeeSync                                http.Handler
	workDeptIndex                                   http.Handler
	workDeptMember                                  http.Handler
	workDeptPhone                                   http.Handler
	workDeptPage                                    http.Handler
	workDeptEmployee                                http.Handler
	workTagGroupIndex                               http.Handler
	workTagGroupDetail                              http.Handler
	workTagGroupStore                               http.Handler
	workTagGroupUpdate                              http.Handler
	workTagGroupDestroy                             http.Handler
	sidebarTagGroupIndex                            http.Handler
	workContactTagIndex                             http.Handler
	workContactTagDetail                            http.Handler
	workContactTagList                              http.Handler
	workContactTagAll                               http.Handler
	workContactTagStore                             http.Handler
	workContactTagUpdate                            http.Handler
	workContactTagDestroy                           http.Handler
	workContactTagMove                              http.Handler
	workContactTagSync                              http.Handler
	workContactSync                                 http.Handler
	workContactIndex                                http.Handler
	workContactLoss                                 http.Handler
	workContactSource                               http.Handler
	workContactShow                                 http.Handler
	workContactTrack                                http.Handler
	workContactUpdate                               http.Handler
	workContactBatchLabeling                        http.Handler
	workContactRoomIndex                            http.Handler
	workRoomIndex                                   http.Handler
	workRoomRoomIndex                               http.Handler
	workRoomStatistics                              http.Handler
	workRoomStatisticsIndex                         http.Handler
	workRoomSync                                    http.Handler
	workRoomBatchUpdate                             http.Handler
	sidebarWorkRoomManage                           http.Handler
	contactTransferInfo                             http.Handler
	contactTransferUnassigned                       http.Handler
	contactTransferRoom                             http.Handler
	contactTransferLog                              http.Handler
	contactTransferSync                             http.Handler
	contactTransferCustomer                         http.Handler
	contactTransferRoomStore                        http.Handler
	workRoomAutoPullIndex                           http.Handler
	workRoomAutoPullShow                            http.Handler
	workRoomAutoPullStore                           http.Handler
	workRoomAutoPullUpdate                          http.Handler
	workRoomAutoPullMove                            http.Handler
	roomTagPullIndex                                http.Handler
	roomTagPullShow                                 http.Handler
	roomTagPullShowContact                          http.Handler
	roomTagPullRoomList                             http.Handler
	roomTagPullChooseContact                        http.Handler
	roomTagPullStore                                http.Handler
	roomTagPullFilterContact                        http.Handler
	roomTagPullRemindSend                           http.Handler
	roomTagPullDestroy                              http.Handler
	roomTagPullContactDetailPage                    http.Handler
	contactMessageBatchSendIndex                    http.Handler
	contactMessageBatchSendShow                     http.Handler
	contactMessageBatchSendMessageShow              http.Handler
	contactMessageBatchSendShowRoom                 http.Handler
	contactMessageBatchSendEmployee                 http.Handler
	contactMessageBatchSendContactReceive           http.Handler
	contactMessageBatchSendStore                    http.Handler
	contactMessageBatchSendRemind                   http.Handler
	contactMessageBatchSendDestroy                  http.Handler
	roomMessageBatchSendIndex                       http.Handler
	roomMessageBatchSendShow                        http.Handler
	roomMessageBatchSendOwner                       http.Handler
	roomMessageBatchSendRoomReceive                 http.Handler
	roomMessageBatchSendStore                       http.Handler
	roomMessageBatchSendRemind                      http.Handler
	roomMessageBatchSendDestroy                     http.Handler
	officialAccountIndex                            http.Handler
	officialAccountSet                              http.Handler
	officialAccountGetPreAuthURL                    http.Handler
	officialAccountAuthRedirect                     http.Handler
	officialAccountAuthEventCallback                http.Handler
	officialAccountMessageEventCallback             http.Handler
	load                                            http.Handler
	workFissionIndex                                http.Handler
	workFissionShow                                 http.Handler
	workFissionInfo                                 http.Handler
	workFissionStatistics                           http.Handler
	workFissionChooseContact                        http.Handler
	workFissionStore                                http.Handler
	workFissionUpdate                               http.Handler
	workFissionInvite                               http.Handler
	workFissionInviteData                           http.Handler
	workFissionInviteDetail                         http.Handler
	workFissionDestroy                              http.Handler
	operationWorkFissionInviteFriends               http.Handler
	operationWorkFissionPoster                      http.Handler
	operationWorkFissionTaskData                    http.Handler
	operationWorkFissionReceive                     http.Handler
	operationWorkFissionAuth                        http.Handler
	operationWorkFissionOpenUserInfo                http.Handler
	operationLotteryContactData                     http.Handler
	operationLotteryContactLottery                  http.Handler
	operationLotteryReceive                         http.Handler
	operationLotteryAuth                            http.Handler
	operationLotteryOpenUserInfo                    http.Handler
	operationRoomClockInContactData                 http.Handler
	operationRoomClockInClockInRanking              http.Handler
	operationRoomClockInContactClockIn              http.Handler
	operationRoomClockInReceive                     http.Handler
	operationRoomClockInAuth                        http.Handler
	operationRoomClockInOpenUserInfo                http.Handler
	operationRoomFissionPoster                      http.Handler
	operationRoomFissionInviteFriends               http.Handler
	operationRoomFissionReceive                     http.Handler
	operationRoomFissionAuth                        http.Handler
	operationRoomFissionOpenUserInfo                http.Handler
	operationRoomInfinitePullQRCode                 http.Handler
	operationShopCodeAreaCode                       http.Handler
	operationShopCodeWeChatSDKConfig                http.Handler
	operationShopCodeAuth                           http.Handler
	operationShopCodeOpenUserInfo                   http.Handler
	workRoomGroupIndex                              http.Handler
	workRoomGroupStore                              http.Handler
	workRoomGroupUpdate                             http.Handler
	workRoomGroupDestroy                            http.Handler
	sidebarContactTagAll                            http.Handler
	sidebarContactDetail                            http.Handler
	sidebarContactShow                              http.Handler
	sidebarContactTrack                             http.Handler
	sidebarContactUpdate                            http.Handler
	sidebarProcessStatus                            http.Handler
	sidebarProcessUpdate                            http.Handler
	mediumIndex                                     http.Handler
	mediumShow                                      http.Handler
	mediumStore                                     http.Handler
	mediumUpdate                                    http.Handler
	mediumDestroy                                   http.Handler
	mediumItemGroupUpdate                           http.Handler
	mediumBatchGroupUpdate                          http.Handler
	mediumReferenceCheck                            http.Handler
	mediumBatchDestroy                              http.Handler
	materialSelectorIndex                           http.Handler
	sidebarMediumIndex                              http.Handler
	sidebarMediumMediaIDUpdate                      http.Handler
	mediumGroupIndex                                http.Handler
	mediumGroupStore                                http.Handler
	mediumGroupUpdate                               http.Handler
	mediumGroupDestroy                              http.Handler
	friendsCircleTaskIndex                          http.Handler
	friendsCircleMaterialIndex                      http.Handler
	friendsCircleTaskStore                          http.Handler
	friendsCircleMaterialStore                      http.Handler
	friendsCirclePublish                            http.Handler
	friendsCircleTaskResultIndex                    http.Handler
	friendsCircleExport                             http.Handler
	friendsCircleExportData                         http.Handler
	friendsCircleProviderCallback                   http.Handler
	phase34AcquisitionLinkIndex                     http.Handler
	phase34AcquisitionLinkStore                     http.Handler
	phase34AcquisitionLinkAuthorize                 http.Handler
	phase34CustomerServiceIndex                     http.Handler
	phase34CustomerServiceStore                     http.Handler
	phase34CustomerServiceSync                      http.Handler
	phase34ShortLinkIndex                           http.Handler
	phase34ShortLinkStore                           http.Handler
	phase34ShortLinkDisable                         http.Handler
	phase34ShortLinkRedirect                        http.Handler
	sidebarMediumGroupIndex                         http.Handler
	channelCodeIndex                                http.Handler
	channelCodeShow                                 http.Handler
	channelCodeContact                              http.Handler
	channelCodeStatistics                           http.Handler
	channelCodeStatsIndex                           http.Handler
	channelCodeStore                                http.Handler
	channelCodeUpdate                               http.Handler
	channelCodeGroupIndex                           http.Handler
	channelCodeGroupDetail                          http.Handler
	channelCodeGroupStore                           http.Handler
	channelCodeGroupUpdate                          http.Handler
	channelCodeGroupMove                            http.Handler
	greetingIndex                                   http.Handler
	greetingShow                                    http.Handler
	greetingStore                                   http.Handler
	greetingUpdate                                  http.Handler
	greetingDestroy                                 http.Handler
	roomWelcomeIndex                                http.Handler
	roomWelcomeSelect                               http.Handler
	roomWelcomeShow                                 http.Handler
	roomWelcomeStore                                http.Handler
	roomWelcomeUpdate                               http.Handler
	roomWelcomeDestroy                              http.Handler
	contactFieldIndex                               http.Handler
	contactFieldShow                                http.Handler
	contactFieldPortrait                            http.Handler
	contactFieldStore                               http.Handler
	contactFieldUpdate                              http.Handler
	contactFieldStatus                              http.Handler
	contactFieldDestroy                             http.Handler
	contactFieldBatch                               http.Handler
	contactFieldPivot                               http.Handler
	contactFieldPivotUpdate                         http.Handler
	contactBatchAddIndex                            http.Handler
	contactBatchAddImportIndex                      http.Handler
	contactBatchAddImportStore                      http.Handler
	contactBatchAddAllot                            http.Handler
	contactBatchAddDataStatistic                    http.Handler
	contactBatchAddDestroy                          http.Handler
	contactBatchAddImportDestroy                    http.Handler
	contactBatchAddSettingEdit                      http.Handler
	contactBatchAddSettingUpdate                    http.Handler
	contactBatchAddRemind                           http.Handler
	sensitiveWordsPage                              http.Handler
	sensitiveWordIndex                              http.Handler
	sensitiveWordStore                              http.Handler
	sensitiveWordDestroy                            http.Handler
	sensitiveWordStatusUpdate                       http.Handler
	sensitiveWordMove                               http.Handler
	sensitiveWordGroupSelect                        http.Handler
	sensitiveWordGroupStore                         http.Handler
	sensitiveWordGroupUpdate                        http.Handler
	sensitiveWordsMonitorIndex                      http.Handler
	sensitiveWordsMonitorShow                       http.Handler
	contactSOPIndex                                 http.Handler
	contactSOPStore                                 http.Handler
	contactSOPSetEmployee                           http.Handler
	contactSOPState                                 http.Handler
	contactSOPInfo                                  http.Handler
	contactSOPDestroy                               http.Handler
	contactSOPUpdate                                http.Handler
	contactSOPPage                                  http.Handler
	roomSOPPage                                     http.Handler
	roomSOPIndex                                    http.Handler
	roomSOPStore                                    http.Handler
	roomSOPSetRoom                                  http.Handler
	roomSOPState                                    http.Handler
	roomSOPInfo                                     http.Handler
	roomSOPDestroy                                  http.Handler
	roomSOPUpdate                                   http.Handler
	shopCodePage                                    http.Handler
	shopCodeLocation                                http.Handler
	shopCodeAddressKeyWordList                      http.Handler
	shopCodeStore                                   http.Handler
	shopCodeUpdate                                  http.Handler
	shopCodeDestroy                                 http.Handler
	shopCodeInfo                                    http.Handler
	shopCodeStatus                                  http.Handler
	shopCodeIndex                                   http.Handler
	shopCodeSearchCity                              http.Handler
	shopCodeShare                                   http.Handler
	shopCodePageInfo                                http.Handler
	shopCodePageSet                                 http.Handler
	shopCodeShow                                    http.Handler
	shopCodeShowContact                             http.Handler
	shopCodeShowShop                                http.Handler
	shopCodeUpdateEmployee                          http.Handler
	shopCodeUpdateQRCode                            http.Handler
	shopCodeBatchContactTags                        http.Handler
	radarPage                                       http.Handler
	radarStore                                      http.Handler
	radarUpdate                                     http.Handler
	radarIndex                                      http.Handler
	radarDestroy                                    http.Handler
	radarInfo                                       http.Handler
	radarStoreChannel                               http.Handler
	radarStoreChannelLink                           http.Handler
	radarIndexChannel                               http.Handler
	radarIndexChannelLink                           http.Handler
	radarShow                                       http.Handler
	radarShowContact                                http.Handler
	radarShowChannel                                http.Handler
	radarArticle                                    http.Handler
	autoTagStore                                    http.Handler
	autoTagIndex                                    http.Handler
	autoTagDestroy                                  http.Handler
	autoTagOnOff                                    http.Handler
	autoTagShow                                     http.Handler
	autoTagShowContactKeyWord                       http.Handler
	autoTagKeyWordTag                               http.Handler
	autoTagShowContactRoom                          http.Handler
	autoTagShowContactTime                          http.Handler
	workMessageFromUsers                            http.Handler
	workMessageToUsers                              http.Handler
	workMessageGlobalOverview                       http.Handler
	workMessageFocus                                http.Handler
	workMessageStaffDirectory                       http.Handler
	workMessageStaffDetail                          http.Handler
	workMessageTrajectoryDay                        http.Handler
	workMessageCustomerDirectory                    http.Handler
	workMessageCustomerConversations                http.Handler
	workMessageCustomerDetail                       http.Handler
	workMessageRoomDirectory                        http.Handler
	workMessageRoomProfile                          http.Handler
	workMessageRoomMessages                         http.Handler
	workMessageRoomMembers                          http.Handler
	workMessageRoomFilterOptions                    http.Handler
	riskBehaviorRules                               http.Handler
	riskBehaviorRecords                             http.Handler
	riskBehaviorRuleCreate                          http.Handler
	riskBehaviorRuleUpdate                          http.Handler
	riskBehaviorRuleStatus                          http.Handler
	riskBehaviorRuleDelete                          http.Handler
	riskBehaviorRecordsAudit                        http.Handler
	riskBehaviorEvaluate                            http.Handler
	timeoutWarningRules                             http.Handler
	timeoutWarningRecords                           http.Handler
	timeoutWarningRuleCreate                        http.Handler
	timeoutWarningRuleUpdate                        http.Handler
	timeoutWarningRuleStatus                        http.Handler
	timeoutWarningRuleDelete                        http.Handler
	timeoutWarningRecordsAudit                      http.Handler
	timeoutWarningRecordsAssign                     http.Handler
	timeoutWarningSettings                          http.Handler
	timeoutWarningSettingsUpdate                    http.Handler
	timeoutWarningEvaluate                          http.Handler
	keywordLibraries                                http.Handler
	keywordLibrarySave                              http.Handler
	keywordLibraryStatus                            http.Handler
	keywordLibraryDelete                            http.Handler
	keywordLibraryPublish                           http.Handler
	keywordEntries                                  http.Handler
	keywordEntrySave                                http.Handler
	keywordEntryStatus                              http.Handler
	keywordEntryDelete                              http.Handler
	messageInterceptRules                           http.Handler
	messageInterceptRuleSave                        http.Handler
	messageInterceptRuleStatus                      http.Handler
	messageInterceptRuleDelete                      http.Handler
	messageInterceptRecords                         http.Handler
	messageInterceptEvaluate                        http.Handler
	messageInterceptAudit                           http.Handler
	silentCustomerRules                             http.Handler
	silentCustomerRuleSave                          http.Handler
	silentCustomerRuleStatus                        http.Handler
	silentCustomerRuleDelete                        http.Handler
	silentCustomerRecords                           http.Handler
	silentCustomerEvaluate                          http.Handler
	silentCustomerAction                            http.Handler
	refuseArchiveRecords                            http.Handler
	refuseArchiveSync                               http.Handler
	refuseArchiveFollowUp                           http.Handler
	workMessageIndex                                http.Handler
	workMessageConfigCorpStore                      http.Handler
	workMessageConfigCorpShow                       http.Handler
	workMessageConfigCorpIndex                      http.Handler
	workMessageConfigStepCreate                     http.Handler
	workMessageConfigStepUpdate                     http.Handler
	lotteryPage                                     http.Handler
	lotteryIndex                                    http.Handler
	lotteryStore                                    http.Handler
	lotteryShowContact                              http.Handler
	lotteryShow                                     http.Handler
	lotteryDestroy                                  http.Handler
	lotteryShare                                    http.Handler
	lotteryUpdate                                   http.Handler
	lotteryInfo                                     http.Handler
	lotteryWriteOff                                 http.Handler
	lotteryBatchContactTags                         http.Handler
	roomFissionPage                                 http.Handler
	roomFissionIndex                                http.Handler
	roomFissionStore                                http.Handler
	roomFissionInfo                                 http.Handler
	roomFissionUpdate                               http.Handler
	roomFissionDestroy                              http.Handler
	roomFissionInvite                               http.Handler
	roomFissionShow                                 http.Handler
	roomFissionShowRoom                             http.Handler
	roomFissionShowContact                          http.Handler
	roomFissionWriteOff                             http.Handler
	roomClockInPage                                 http.Handler
	roomClockInIndex                                http.Handler
	roomClockInStore                                http.Handler
	roomClockInUpdate                               http.Handler
	roomClockInDestroy                              http.Handler
	roomClockInShow                                 http.Handler
	roomClockInShowContact                          http.Handler
	roomClockInBatchContactTags                     http.Handler
	roomClockInInfo                                 http.Handler
	roomClockInDayDetail                            http.Handler
	roomQualityPage                                 http.Handler
	roomQualityIndex                                http.Handler
	roomQualityStore                                http.Handler
	roomQualityStatus                               http.Handler
	roomQualityInfo                                 http.Handler
	roomQualityUpdate                               http.Handler
	roomQualityShowContact                          http.Handler
	roomQualityDestroy                              http.Handler
	roomQualityContactDetail                        http.Handler
	roomCalendarPage                                http.Handler
	roomCalendarIndex                               http.Handler
	roomCalendarAddRoom                             http.Handler
	roomCalendarDestroyRoom                         http.Handler
	roomCalendarStore                               http.Handler
	roomCalendarDestroy                             http.Handler
	roomCalendarShow                                http.Handler
	roomCalendarUpdate                              http.Handler
	roomRemindPage                                  http.Handler
	roomRemindIndex                                 http.Handler
	roomRemindDestroy                               http.Handler
	roomRemindInfo                                  http.Handler
	roomRemindStatus                                http.Handler
	roomRemindStore                                 http.Handler
	roomRemindUpdate                                http.Handler
	roomRemindTask                                  http.Handler
	roomInfinitePullPage                            http.Handler
	roomInfinitePullIndex                           http.Handler
	roomInfinitePullInfo                            http.Handler
	roomInfinitePullUpdate                          http.Handler
	roomInfinitePullDestroy                         http.Handler
	roomInfinitePullStore                           http.Handler
	sidebarFieldPivot                               http.Handler
	sidebarFieldPivotUpdate                         http.Handler
	sidebarContactBatchAddDetail                    http.Handler
	sidebarContactSOPInfo                           http.Handler
	sidebarContactSOPTipInfo                        http.Handler
	sidebarRoomSOPInfo                              http.Handler
	sidebarRoomSOPLogState                          http.Handler
	saasAlertPage                                   http.Handler
	saasAlertIndex                                  http.Handler
	saasAlertResolve                                http.Handler
	saasAlertSetting                                http.Handler
	saasBillingPage                                 http.Handler
	saasBillingSummary                              http.Handler
	saasBillingPaymentOrders                        http.Handler
	saasBillingPaymentRefunds                       http.Handler
	saasBillingInvoiceProfile                       http.Handler
	saasBillingInvoices                             http.Handler
	saasBillingInvoice                              http.Handler
	saasBillingInvoiceCancel                        http.Handler
	saasAdminPage                                   http.Handler
	saasAdminOverview                               http.Handler
	saasAdminTenantReadiness                        http.Handler
	saasAdminTenant                                 http.Handler
	saasAdminTenantLifecycle                        http.Handler
	saasAdminUsage                                  http.Handler
	saasAdminRisk                                   http.Handler
	saasAdminBusinessMetrics                        http.Handler
	saasAdminBusinessTrends                         http.Handler
	saasAdminOperationQueue                         http.Handler
	saasAdminOperationQueueOwners                   http.Handler
	saasAdminOperationQueueAssignments              http.Handler
	saasAdminOperationQueueAssignmentClose          http.Handler
	saasAdminOperationQueueAssignmentNotifications  http.Handler
	saasAdminOperationQueueAssign                   http.Handler
	saasAdminRenewalForecast                        http.Handler
	saasAdminRenewalForecastTasks                   http.Handler
	saasAdminRenewalForecastAssign                  http.Handler
	saasAdminRenewalForecastNotifications           http.Handler
	saasAdminCustomerSuccess                        http.Handler
	saasAdminCustomerSuccessOwners                  http.Handler
	saasAdminCustomerSuccessAssign                  http.Handler
	saasAdminCustomerSuccessRenewalTasks            http.Handler
	saasAdminCustomerSuccessRenewalNotifications    http.Handler
	saasAdminRiskFollowUp                           http.Handler
	saasAdminRiskFollowUps                          http.Handler
	saasAdminRiskFollowUpOwners                     http.Handler
	saasAdminRiskFollowUpBulkClose                  http.Handler
	saasAdminAlerts                                 http.Handler
	saasAdminAlertResolve                           http.Handler
	saasAdminAlertBulkResolve                       http.Handler
	saasAdminNotifications                          http.Handler
	saasAdminNotificationHealth                     http.Handler
	saasAdminNotificationSLO                        http.Handler
	saasAdminNotificationHealthRecovery             http.Handler
	saasAdminNotificationPolicies                   http.Handler
	saasAdminNotificationPolicy                     http.Handler
	saasAdminNotificationPolicyTest                 http.Handler
	saasAdminNotificationCredentialRotation         http.Handler
	saasAdminWeComCredentialProtection              http.Handler
	saasAdminWeComCredentialRotation                http.Handler
	saasAdminWeChatOpenCredentialProtection         http.Handler
	saasAdminWeChatOpenCredentialRotation           http.Handler
	saasAdminNotificationRetry                      http.Handler
	saasAdminNotificationBulkRetry                  http.Handler
	saasAdminNotificationClose                      http.Handler
	saasAdminNotificationBulkClose                  http.Handler
	saasAdminPackages                               http.Handler
	saasAdminSubscriptions                          http.Handler
	saasAdminSubscriptionEvents                     http.Handler
	saasAdminSubscriptionTransition                 http.Handler
	saasAdminSubscriptionReconcile                  http.Handler
	saasAdminPaymentOrders                          http.Handler
	saasAdminPaymentWebhookEvents                   http.Handler
	saasAdminPaymentOrder                           http.Handler
	saasAdminPaymentOrderCancel                     http.Handler
	saasAdminPaymentDunning                         http.Handler
	saasAdminPaymentRefunds                         http.Handler
	saasAdminPaymentRefund                          http.Handler
	saasAdminPaymentRefundCancel                    http.Handler
	saasAdminPaymentSettlementBatches               http.Handler
	saasAdminPaymentSettlementEntries               http.Handler
	saasAdminPaymentSettlementImport                http.Handler
	saasAdminPaymentSettlementReconcile             http.Handler
	saasAdminPaymentSettlementResolve               http.Handler
	saasAdminPaymentSettlementTransition            http.Handler
	saasAdminPaymentSettlementSyncRuns              http.Handler
	saasAdminPaymentSettlementSync                  http.Handler
	saasAdminAccessProfile                          http.Handler
	saasAdminAccessRoles                            http.Handler
	saasAdminAccessRole                             http.Handler
	saasAdminAccessAssignments                      http.Handler
	saasAdminAccessAssignment                       http.Handler
	saasAdminSystemHealth                           http.Handler
	saasAdminSystemHealthScans                      http.Handler
	saasAdminSystemIncidents                        http.Handler
	saasAdminSystemHealthScan                       http.Handler
	saasAdminSystemIncident                         http.Handler
	saasAdminServiceAccounts                        http.Handler
	saasAdminServiceAccountUsage                    http.Handler
	saasAdminServiceAccountUsageAlertEvaluate       http.Handler
	saasAdminServiceAccount                         http.Handler
	saasAdminServiceAccountKeyRotate                http.Handler
	saasAdminServiceAccountKeyRevoke                http.Handler
	saasAdminAuditIntegrity                         http.Handler
	saasAdminAuditIntegrityVerify                   http.Handler
	saasAdminAuditAnchors                           http.Handler
	saasAdminAuditAnchor                            http.Handler
	saasAdminBackupOverview                         http.Handler
	saasAdminBackupPolicy                           http.Handler
	saasAdminBackupRun                              http.Handler
	saasAdminRestoreDrill                           http.Handler
	saasAdminComplianceOverview                     http.Handler
	saasAdminCompliancePolicy                       http.Handler
	saasAdminComplianceLegalHold                    http.Handler
	saasAdminComplianceExport                       http.Handler
	saasAdminComplianceExportDownload               http.Handler
	saasAdminComplianceErasure                      http.Handler
	saasAdminComplianceErasureSteps                 http.Handler
	saasAdminIdentityOverview                       http.Handler
	saasAdminIdentityPolicy                         http.Handler
	saasAdminIdentitySessions                       http.Handler
	saasAdminIdentitySession                        http.Handler
	saasAdminIdentityLoginEvents                    http.Handler
	saasAdminIdentityIncidents                      http.Handler
	saasAdminIdentityIncident                       http.Handler
	saasAdminIdentityUser                           http.Handler
	saasAdminIdentityMFA                            http.Handler
	saasAdminBrandingProfiles                       http.Handler
	saasAdminBrandingProfile                        http.Handler
	saasAdminTenantDomains                          http.Handler
	saasAdminTenantDomain                           http.Handler
	saasAdminTenantDomainDeliveryJobs               http.Handler
	saasAdminTenantDomainDelivery                   http.Handler
	saasAdminReleaseReadiness                       http.Handler
	saasAdminReleaseEvidence                        http.Handler
	saasAdminReleaseEvidenceAction                  http.Handler
	saasAdminReleaseCandidate                       http.Handler
	saasServiceAccountWhoAmI                        http.Handler
	saasServiceAccountUsage                         http.Handler
	saasServiceAccountAlerts                        http.Handler
	saasAdminApprovalPolicies                       http.Handler
	saasAdminApprovalPolicy                         http.Handler
	saasAdminApprovals                              http.Handler
	saasAdminApprovalEvents                         http.Handler
	saasAdminApprovalDecisions                      http.Handler
	saasAdminApprovalDelegations                    http.Handler
	saasAdminApprovalDelegation                     http.Handler
	saasAdminApprovalReminders                      http.Handler
	saasAdminApprovalRequest                        http.Handler
	saasAdminApprovalDecision                       http.Handler
	saasAdminApprovalCancel                         http.Handler
	saasAdminApprovalExecute                        http.Handler
	saasAdminInvoiceProfile                         http.Handler
	saasAdminInvoiceDocuments                       http.Handler
	saasAdminInvoice                                http.Handler
	saasAdminCreditNote                             http.Handler
	saasAdminInvoiceTransition                      http.Handler
	saasPaymentWebhook                              http.Handler
	saasTenantDomainDeliveryWebhook                 http.Handler
	saasAdminOperations                             http.Handler
	saasAdminBillingEvents                          http.Handler
	saasAdminBillingReconciliation                  http.Handler
	saasAdminBillingReconciliationFollowUp          http.Handler
	saasAdminBillingReconciliationFollowUps         http.Handler
	saasAdminBillingReconciliationFollowUpOwners    http.Handler
	saasAdminBillingReconciliationFollowUpBulkClose http.Handler
	saasAdminTasks                                  http.Handler
	saasAdminTaskOwners                             http.Handler
	saasAdminTaskSLA                                http.Handler
	saasAdminTaskSLANotifications                   http.Handler
	saasAdminTaskCancel                             http.Handler
	saasAdminTaskBulkCancel                         http.Handler
	saasAdminTaskBulkReset                          http.Handler
	saasAdminTaskReset                              http.Handler
	saasAdminDailyReport                            http.Handler
	saasAdminExport                                 http.Handler
	saasAdminPackage                                http.Handler
	saasAdminPackageSync                            http.Handler
	saasAdminPackageSyncTask                        http.Handler
	saasAdminPackageSyncTaskApply                   http.Handler
	saasAdminPackageSyncTaskBulkApply               http.Handler
	saasAdminTenantStatus                           http.Handler
	saasAdminTenantRenewal                          http.Handler
	saasAdminTenantRenewalTask                      http.Handler
	saasAdminTenantRenewalTaskApply                 http.Handler
	saasAdminTenantRenewalTaskBulkApply             http.Handler
	saasAdminTenantProvision                        http.Handler
	saasAdminTenantProvisionTask                    http.Handler
	saasAdminTenantProvisionTaskApply               http.Handler
	saasAdminTenantProvisionTaskBulkApply           http.Handler
	saasAdminTenantPackage                          http.Handler
	saasAdminDashboardProvisioning                  http.Handler
	chatToolConfig                                  http.Handler
	commonUpload                                    http.Handler
	commonUploadFile                                http.Handler
	sidebarCommonUpload                             http.Handler
	agentTxtUpload                                  http.Handler
	agentTxtVerify                                  bool
	agentStore                                      http.Handler
	sidebarAgentAuth                                http.Handler
	sidebarAgentOAuth                               http.Handler
	sidebarAgentJSSDK                               http.Handler
	sidebarWxJSSDK                                  http.Handler
	roleSelect                                      http.Handler
	roleIndex                                       http.Handler
	roleShow                                        http.Handler
	rolePermission                                  http.Handler
	roleShowEmployee                                http.Handler
	roleStore                                       http.Handler
	roleUpdate                                      http.Handler
	roleStatusUpdate                                http.Handler
	roleDestroy                                     http.Handler
	rolePermissionStore                             http.Handler
	menuIconIndex                                   http.Handler
	menuSelect                                      http.Handler
	menuIndex                                       http.Handler
	menuShow                                        http.Handler
	menuStore                                       http.Handler
	menuUpdate                                      http.Handler
	menuStatusUpdate                                http.Handler
	menuDestroy                                     http.Handler
	backgroundTasks                                 func() []taskrunner.Snapshot
}

type statusPayload struct {
	Name                  string                `json:"name"`
	Mode                  string                `json:"mode"`
	StartedAt             string                `json:"started_at"`
	Standalone            bool                  `json:"standalone"`
	SourceRoot            string                `json:"source_root"`
	SourceRootExists      bool                  `json:"source_root_exists"`
	ManifestPath          string                `json:"manifest_path"`
	ManifestExists        bool                  `json:"manifest_exists"`
	PHPUpstream           string                `json:"php_upstream"`
	PHPUpstreamReady      bool                  `json:"php_upstream_ready"`
	PHPUpstreamProbe      string                `json:"php_upstream_probe"`
	ProxyFallbackEnabled  bool                  `json:"proxy_fallback_enabled"`
	MigratedRouteCount    int                   `json:"migrated_route_count"`
	MigratedRoutes        []string              `json:"migrated_routes"`
	BackgroundTasks       []taskrunner.Snapshot `json:"background_tasks,omitempty"`
	NextMigrationBoundary string                `json:"next_migration_boundary"`
}

var migratedRoutes = []string{
	"GET /",
	"POST /",
	"HEAD /",
	"GET /favicon.ico",
	"GET /healthz",
	"GET /readyz",
	"GET /compat/status",
	"GET /compat/routes",
}

type ModuleRouter interface {
	Match(*http.Request) (http.Handler, bool)
}

type DashboardRequestGuard interface {
	Authorize(http.ResponseWriter, *http.Request) bool
}

type SaaSRequestGuard interface {
	Authorize(http.ResponseWriter, *http.Request) bool
}

type Option func(*Server)

func WithModuleRouter(router ModuleRouter) Option {
	return func(server *Server) {
		if nilcheck.IsNil(router) {
			server.moduleRouter = nil
			return
		}
		server.moduleRouter = router
	}
}

func WithDashboardRequestGuard(guard DashboardRequestGuard) Option {
	return func(server *Server) {
		if nilcheck.IsNil(guard) {
			server.dashboardRequestGuard = nil
			return
		}
		server.dashboardRequestGuard = guard
	}
}

func WithSaaSRequestGuard(guard SaaSRequestGuard) Option {
	return func(server *Server) {
		if nilcheck.IsNil(guard) {
			server.saasRequestGuard = nil
			return
		}
		server.saasRequestGuard = guard
	}
}

func WithSaaSAuthHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAuth = handler }
}

func WithSaaSLoginPageHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasLoginPage = handler }
}

func WithDashboardAccessHandler(handler http.Handler) Option {
	return func(server *Server) {
		if nilcheck.IsNil(handler) {
			server.dashboardAccess = nil
			return
		}
		server.dashboardAccess = handler
	}
}

func WithBackgroundTasks(snapshot func() []taskrunner.Snapshot) Option {
	return func(server *Server) {
		server.backgroundTasks = snapshot
	}
}

func WithDashboardAuthHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.dashboardAuth = handler
	}
}

func WithCompanyProfileHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.companyProfile = handler
	}
}

func WithProviderStatusHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.providerStatus = handler
	}
}

func WithIdentitySelfHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.identitySelf = handler
	}
}

func WithLogoutHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.logout = handler
	}
}

func WithUserIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.userIndex = handler
	}
}

func WithUserShowHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.userShow = handler
	}
}

func WithUserStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.userStore = handler
	}
}

func WithUserUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.userUpdate = handler
	}
}

func WithUserStatusUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.userStatusUpdate = handler
	}
}

func WithUserPasswordResetHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.userPasswordReset = handler
	}
}

func WithUserPasswordUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.userPasswordUpdate = handler
	}
}

func WithPermissionByUserHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.permissionByUser = handler
	}
}

func WithWeWorkCallbackHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.weWorkCallback = handler
	}
}

func WithCorpDataIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.corpDataIndex = handler
	}
}

func WithCorpDataLineChatHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.corpDataLineChat = handler
	}
}

func WithExternalTenantIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.externalTenantIndex = handler
	}
}

func WithStatisticIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.statisticIndex = handler
	}
}

func WithStatisticTopListHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.statisticTopList = handler
	}
}

func WithStatisticEmployeeCountsHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.statisticEmployeeCounts = handler
	}
}

func WithStatisticEmployeesHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.statisticEmployees = handler
	}
}

func WithStatisticEmployeesTrendHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.statisticEmployeesTrend = handler
	}
}

func WithWorkEmployeeSearchConditionHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workEmployeeCond = handler
	}
}

func WithWorkEmployeeIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workEmployeeIndex = handler
	}
}

func WithWorkEmployeeSyncHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workEmployeeSync = handler
	}
}

func WithWorkDepartmentIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workDeptIndex = handler
	}
}

func WithWorkEmployeeDepartmentMemberIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workDeptMember = handler
	}
}

func WithWorkDepartmentSelectByPhoneHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workDeptPhone = handler
	}
}

func WithWorkDepartmentPageIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workDeptPage = handler
	}
}

func WithWorkDepartmentShowEmployeeHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workDeptEmployee = handler
	}
}

func WithWorkContactTagGroupIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workTagGroupIndex = handler
	}
}

func WithWorkContactTagGroupDetailHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workTagGroupDetail = handler
	}
}

func WithWorkContactTagGroupStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workTagGroupStore = handler
	}
}

func WithWorkContactTagGroupUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workTagGroupUpdate = handler
	}
}

func WithWorkContactTagGroupDestroyHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workTagGroupDestroy = handler
	}
}

func WithSidebarWorkContactTagGroupIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.sidebarTagGroupIndex = handler
	}
}

func WithWorkContactTagIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workContactTagIndex = handler
	}
}

func WithWorkContactTagDetailHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workContactTagDetail = handler
	}
}

func WithWorkContactTagListHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workContactTagList = handler
	}
}

func WithWorkContactTagAllHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workContactTagAll = handler
	}
}

func WithWorkContactTagStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workContactTagStore = handler
	}
}

func WithWorkContactTagUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workContactTagUpdate = handler
	}
}

func WithWorkContactTagDestroyHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workContactTagDestroy = handler
	}
}

func WithWorkContactTagMoveHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workContactTagMove = handler
	}
}

func WithWorkContactTagSyncHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workContactTagSync = handler
	}
}

func WithWorkContactSyncHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workContactSync = handler
	}
}

func WithWorkContactIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workContactIndex = handler
	}
}

func WithWorkContactLossHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workContactLoss = handler
	}
}

func WithWorkContactSourceHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workContactSource = handler
	}
}

func WithWorkContactShowHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workContactShow = handler
	}
}

func WithWorkContactTrackHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workContactTrack = handler
	}
}

func WithWorkContactUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workContactUpdate = handler
	}
}

func WithWorkContactBatchLabelingHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workContactBatchLabeling = handler
	}
}

func WithWorkContactRoomIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workContactRoomIndex = handler
	}
}

func WithWorkRoomIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workRoomIndex = handler
	}
}

func WithWorkRoomRoomIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workRoomRoomIndex = handler
	}
}

func WithWorkRoomStatisticsHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workRoomStatistics = handler
	}
}

func WithWorkRoomStatisticsIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workRoomStatisticsIndex = handler
	}
}

func WithWorkRoomSyncHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workRoomSync = handler
	}
}

func WithWorkRoomBatchUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workRoomBatchUpdate = handler
	}
}

func WithSidebarWorkRoomManageHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.sidebarWorkRoomManage = handler
	}
}

func WithContactTransferInfoHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactTransferInfo = handler
	}
}

func WithContactTransferUnassignedListHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactTransferUnassigned = handler
	}
}

func WithContactTransferRoomHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactTransferRoom = handler
	}
}

func WithContactTransferLogHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactTransferLog = handler
	}
}

func WithContactTransferSaveUnassignedListHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactTransferSync = handler
	}
}

func WithContactTransferCustomerHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactTransferCustomer = handler
	}
}

func WithContactTransferRoomStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactTransferRoomStore = handler
	}
}

func WithWorkRoomAutoPullIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workRoomAutoPullIndex = handler
	}
}

func WithWorkRoomAutoPullShowHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workRoomAutoPullShow = handler
	}
}

func WithWorkRoomAutoPullStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workRoomAutoPullStore = handler
	}
}

func WithWorkRoomAutoPullUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workRoomAutoPullUpdate = handler
	}
}

func WithWorkRoomAutoPullMoveHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workRoomAutoPullMove = handler
	}
}

func WithRoomTagPullIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomTagPullIndex = handler
	}
}

func WithRoomTagPullShowHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomTagPullShow = handler
	}
}

func WithRoomTagPullShowContactHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomTagPullShowContact = handler
	}
}

func WithRoomTagPullRoomListHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomTagPullRoomList = handler
	}
}

func WithRoomTagPullChooseContactHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomTagPullChooseContact = handler
	}
}

func WithRoomTagPullStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomTagPullStore = handler
	}
}

func WithRoomTagPullFilterContactHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomTagPullFilterContact = handler
	}
}

func WithRoomTagPullRemindSendHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomTagPullRemindSend = handler
	}
}

func WithRoomTagPullDestroyHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomTagPullDestroy = handler
	}
}

func WithRoomTagPullContactDetailPageHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomTagPullContactDetailPage = handler
	}
}

func WithContactMessageBatchSendIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactMessageBatchSendIndex = handler
	}
}

func WithContactMessageBatchSendShowHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactMessageBatchSendShow = handler
	}
}

func WithContactMessageBatchSendMessageShowHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactMessageBatchSendMessageShow = handler
	}
}

func WithContactMessageBatchSendShowRoomHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactMessageBatchSendShowRoom = handler
	}
}

func WithContactMessageBatchSendEmployeeSendIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactMessageBatchSendEmployee = handler
	}
}

func WithContactMessageBatchSendContactReceiveIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactMessageBatchSendContactReceive = handler
	}
}

func WithContactMessageBatchSendStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactMessageBatchSendStore = handler
	}
}

func WithContactMessageBatchSendRemindHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactMessageBatchSendRemind = handler
	}
}

func WithContactMessageBatchSendDestroyHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactMessageBatchSendDestroy = handler
	}
}

func WithRoomMessageBatchSendIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomMessageBatchSendIndex = handler
	}
}

func WithRoomMessageBatchSendShowHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomMessageBatchSendShow = handler
	}
}

func WithRoomMessageBatchSendRoomOwnerSendIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomMessageBatchSendOwner = handler
	}
}

func WithRoomMessageBatchSendRoomReceiveIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomMessageBatchSendRoomReceive = handler
	}
}

func WithRoomMessageBatchSendStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomMessageBatchSendStore = handler
	}
}

func WithRoomMessageBatchSendRemindHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomMessageBatchSendRemind = handler
	}
}

func WithRoomMessageBatchSendDestroyHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomMessageBatchSendDestroy = handler
	}
}

func WithOfficialAccountIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.officialAccountIndex = handler
	}
}

func WithOfficialAccountSetHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.officialAccountSet = handler
	}
}

func WithOfficialAccountGetPreAuthURLHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.officialAccountGetPreAuthURL = handler
	}
}

func WithOfficialAccountAuthRedirectHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.officialAccountAuthRedirect = handler
	}
}

func WithOfficialAccountAuthEventCallbackHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.officialAccountAuthEventCallback = handler
	}
}

func WithOfficialAccountMessageEventCallbackHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.officialAccountMessageEventCallback = handler
	}
}

func WithLoadHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.load = handler
	}
}

func WithWorkFissionIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workFissionIndex = handler
	}
}

func WithWorkFissionShowHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workFissionShow = handler
	}
}

func WithWorkFissionInfoHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workFissionInfo = handler
	}
}

func WithWorkFissionStatisticsHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workFissionStatistics = handler
	}
}

func WithWorkFissionChooseContactHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workFissionChooseContact = handler
	}
}

func WithWorkFissionStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workFissionStore = handler
	}
}

func WithWorkFissionUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workFissionUpdate = handler
	}
}

func WithWorkFissionInviteHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workFissionInvite = handler
	}
}

func WithWorkFissionInviteDataHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workFissionInviteData = handler
	}
}

func WithWorkFissionInviteDetailHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workFissionInviteDetail = handler
	}
}

func WithWorkFissionDestroyHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workFissionDestroy = handler
	}
}

func WithOperationWorkFissionInviteFriendsHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.operationWorkFissionInviteFriends = handler
	}
}

func WithOperationWorkFissionPosterHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.operationWorkFissionPoster = handler
	}
}

func WithOperationWorkFissionTaskDataHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.operationWorkFissionTaskData = handler
	}
}

func WithOperationWorkFissionReceiveHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.operationWorkFissionReceive = handler
	}
}

func WithOperationWorkFissionAuthHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.operationWorkFissionAuth = handler
	}
}

func WithOperationWorkFissionOpenUserInfoHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.operationWorkFissionOpenUserInfo = handler
	}
}

func WithOperationLotteryContactDataHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.operationLotteryContactData = handler
	}
}

func WithOperationLotteryContactLotteryHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.operationLotteryContactLottery = handler
	}
}

func WithOperationLotteryReceiveHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.operationLotteryReceive = handler
	}
}

func WithOperationLotteryAuthHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.operationLotteryAuth = handler
	}
}

func WithOperationLotteryOpenUserInfoHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.operationLotteryOpenUserInfo = handler
	}
}

func WithOperationRoomClockInContactDataHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.operationRoomClockInContactData = handler
	}
}

func WithOperationRoomClockInClockInRankingHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.operationRoomClockInClockInRanking = handler
	}
}

func WithOperationRoomClockInContactClockInHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.operationRoomClockInContactClockIn = handler
	}
}

func WithOperationRoomClockInReceiveHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.operationRoomClockInReceive = handler
	}
}

func WithOperationRoomClockInAuthHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.operationRoomClockInAuth = handler
	}
}

func WithOperationRoomClockInOpenUserInfoHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.operationRoomClockInOpenUserInfo = handler
	}
}

func WithOperationRoomFissionPosterHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.operationRoomFissionPoster = handler
	}
}

func WithOperationRoomFissionInviteFriendsHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.operationRoomFissionInviteFriends = handler
	}
}

func WithOperationRoomFissionReceiveHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.operationRoomFissionReceive = handler
	}
}

func WithOperationRoomFissionAuthHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.operationRoomFissionAuth = handler
	}
}

func WithOperationRoomFissionOpenUserInfoHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.operationRoomFissionOpenUserInfo = handler
	}
}

func WithOperationRoomInfinitePullQRCodeHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.operationRoomInfinitePullQRCode = handler
	}
}

func WithOperationShopCodeAreaCodeHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.operationShopCodeAreaCode = handler
	}
}

func WithOperationShopCodeWeChatSDKConfigHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.operationShopCodeWeChatSDKConfig = handler
	}
}

func WithOperationShopCodeAuthHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.operationShopCodeAuth = handler
	}
}

func WithOperationShopCodeOpenUserInfoHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.operationShopCodeOpenUserInfo = handler
	}
}

func WithWorkRoomGroupIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workRoomGroupIndex = handler
	}
}

func WithWorkRoomGroupStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workRoomGroupStore = handler
	}
}

func WithWorkRoomGroupUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workRoomGroupUpdate = handler
	}
}

func WithWorkRoomGroupDestroyHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workRoomGroupDestroy = handler
	}
}

func WithSidebarWorkContactTagAllHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.sidebarContactTagAll = handler
	}
}

func WithSidebarWorkContactDetailHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.sidebarContactDetail = handler
	}
}

func WithSidebarWorkContactShowHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.sidebarContactShow = handler
	}
}

func WithSidebarWorkContactTrackHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.sidebarContactTrack = handler
	}
}

func WithSidebarWorkContactUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.sidebarContactUpdate = handler
	}
}

func WithSidebarContactProcessStatusIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.sidebarProcessStatus = handler
	}
}

func WithSidebarContactProcessStatusUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.sidebarProcessUpdate = handler
	}
}

func WithMediumIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.mediumIndex = handler
	}
}

func WithMediumShowHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.mediumShow = handler
	}
}

func WithMediumStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.mediumStore = handler
	}
}

func WithMediumUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.mediumUpdate = handler
	}
}

func WithMediumDestroyHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.mediumDestroy = handler
	}
}

func WithMediumItemGroupUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.mediumItemGroupUpdate = handler
	}
}

func WithMediumBatchGroupUpdateHandler(handler http.Handler) Option {
	return func(server *Server) { server.mediumBatchGroupUpdate = handler }
}

func WithMediumReferenceCheckHandler(handler http.Handler) Option {
	return func(server *Server) { server.mediumReferenceCheck = handler }
}

func WithMediumBatchDestroyHandler(handler http.Handler) Option {
	return func(server *Server) { server.mediumBatchDestroy = handler }
}

func WithMaterialSelectorIndexHandler(handler http.Handler) Option {
	return func(server *Server) { server.materialSelectorIndex = handler }
}

func WithSidebarMediumIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.sidebarMediumIndex = handler
	}
}

func WithSidebarMediumMediaIDUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.sidebarMediumMediaIDUpdate = handler
	}
}

func WithMediumGroupIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.mediumGroupIndex = handler
	}
}

func WithMediumGroupStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.mediumGroupStore = handler
	}
}

func WithMediumGroupUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.mediumGroupUpdate = handler
	}
}

func WithMediumGroupDestroyHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.mediumGroupDestroy = handler
	}
}

func WithFriendsCircleTaskIndexHandler(handler http.Handler) Option {
	return func(server *Server) { server.friendsCircleTaskIndex = handler }
}
func WithFriendsCircleMaterialIndexHandler(handler http.Handler) Option {
	return func(server *Server) { server.friendsCircleMaterialIndex = handler }
}
func WithFriendsCircleTaskStoreHandler(handler http.Handler) Option {
	return func(server *Server) { server.friendsCircleTaskStore = handler }
}
func WithFriendsCircleMaterialStoreHandler(handler http.Handler) Option {
	return func(server *Server) { server.friendsCircleMaterialStore = handler }
}
func WithFriendsCirclePublishHandler(handler http.Handler) Option {
	return func(server *Server) { server.friendsCirclePublish = handler }
}
func WithFriendsCircleTaskResultIndexHandler(handler http.Handler) Option {
	return func(server *Server) { server.friendsCircleTaskResultIndex = handler }
}
func WithFriendsCircleExportHandler(handler http.Handler) Option {
	return func(server *Server) { server.friendsCircleExport = handler }
}
func WithFriendsCircleExportDataHandler(handler http.Handler) Option {
	return func(server *Server) { server.friendsCircleExportData = handler }
}
func WithFriendsCircleProviderCallbackHandler(handler http.Handler) Option {
	return func(server *Server) { server.friendsCircleProviderCallback = handler }
}

func WithPhase34AcquisitionLinkIndexHandler(handler http.Handler) Option {
	return func(server *Server) { server.phase34AcquisitionLinkIndex = handler }
}

func WithPhase34AcquisitionLinkStoreHandler(handler http.Handler) Option {
	return func(server *Server) { server.phase34AcquisitionLinkStore = handler }
}

func WithPhase34AcquisitionLinkAuthorizeHandler(handler http.Handler) Option {
	return func(server *Server) { server.phase34AcquisitionLinkAuthorize = handler }
}

func WithPhase34CustomerServiceIndexHandler(handler http.Handler) Option {
	return func(server *Server) { server.phase34CustomerServiceIndex = handler }
}

func WithPhase34CustomerServiceStoreHandler(handler http.Handler) Option {
	return func(server *Server) { server.phase34CustomerServiceStore = handler }
}

func WithPhase34CustomerServiceSyncHandler(handler http.Handler) Option {
	return func(server *Server) { server.phase34CustomerServiceSync = handler }
}

func WithPhase34ShortLinkIndexHandler(handler http.Handler) Option {
	return func(server *Server) { server.phase34ShortLinkIndex = handler }
}

func WithPhase34ShortLinkStoreHandler(handler http.Handler) Option {
	return func(server *Server) { server.phase34ShortLinkStore = handler }
}

func WithPhase34ShortLinkDisableHandler(handler http.Handler) Option {
	return func(server *Server) { server.phase34ShortLinkDisable = handler }
}

func WithPhase34ShortLinkRedirectHandler(handler http.Handler) Option {
	return func(server *Server) { server.phase34ShortLinkRedirect = handler }
}

func WithSidebarMediumGroupIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.sidebarMediumGroupIndex = handler
	}
}

func WithChannelCodeIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.channelCodeIndex = handler
	}
}

func WithChannelCodeShowHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.channelCodeShow = handler
	}
}

func WithChannelCodeContactHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.channelCodeContact = handler
	}
}

func WithChannelCodeStatisticsHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.channelCodeStatistics = handler
	}
}

func WithChannelCodeStatisticsIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.channelCodeStatsIndex = handler
	}
}

func WithChannelCodeStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.channelCodeStore = handler
	}
}

func WithChannelCodeUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.channelCodeUpdate = handler
	}
}

func WithChannelCodeGroupIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.channelCodeGroupIndex = handler
	}
}

func WithChannelCodeGroupDetailHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.channelCodeGroupDetail = handler
	}
}

func WithChannelCodeGroupStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.channelCodeGroupStore = handler
	}
}

func WithChannelCodeGroupUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.channelCodeGroupUpdate = handler
	}
}

func WithChannelCodeGroupMoveHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.channelCodeGroupMove = handler
	}
}

func WithGreetingIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.greetingIndex = handler
	}
}

func WithGreetingShowHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.greetingShow = handler
	}
}

func WithGreetingStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.greetingStore = handler
	}
}

func WithGreetingUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.greetingUpdate = handler
	}
}

func WithGreetingDestroyHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.greetingDestroy = handler
	}
}

func WithRoomWelcomeIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomWelcomeIndex = handler
	}
}

func WithRoomWelcomeSelectHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomWelcomeSelect = handler
	}
}

func WithRoomWelcomeShowHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomWelcomeShow = handler
	}
}

func WithRoomWelcomeStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomWelcomeStore = handler
	}
}

func WithRoomWelcomeUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomWelcomeUpdate = handler
	}
}

func WithRoomWelcomeDestroyHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomWelcomeDestroy = handler
	}
}

func WithContactFieldIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactFieldIndex = handler
	}
}

func WithContactFieldShowHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactFieldShow = handler
	}
}

func WithContactFieldPortraitHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactFieldPortrait = handler
	}
}

func WithContactFieldStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactFieldStore = handler
	}
}

func WithContactFieldUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactFieldUpdate = handler
	}
}

func WithContactFieldStatusUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactFieldStatus = handler
	}
}

func WithContactFieldDestroyHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactFieldDestroy = handler
	}
}

func WithContactFieldBatchUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactFieldBatch = handler
	}
}

func WithContactFieldPivotIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactFieldPivot = handler
	}
}

func WithContactFieldPivotUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactFieldPivotUpdate = handler
	}
}

func WithSidebarContactFieldPivotIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.sidebarFieldPivot = handler
	}
}

func WithSidebarContactFieldPivotUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.sidebarFieldPivotUpdate = handler
	}
}

func WithContactBatchAddIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactBatchAddIndex = handler
	}
}

func WithContactBatchAddImportIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactBatchAddImportIndex = handler
	}
}

func WithContactBatchAddImportStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactBatchAddImportStore = handler
	}
}

func WithContactBatchAddAllotHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactBatchAddAllot = handler
	}
}

func WithContactBatchAddDataStatisticHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactBatchAddDataStatistic = handler
	}
}

func WithContactBatchAddDestroyHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactBatchAddDestroy = handler
	}
}

func WithContactBatchAddImportDestroyHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactBatchAddImportDestroy = handler
	}
}

func WithContactBatchAddSettingEditHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactBatchAddSettingEdit = handler
	}
}

func WithContactBatchAddSettingUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactBatchAddSettingUpdate = handler
	}
}

func WithContactBatchAddRemindHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactBatchAddRemind = handler
	}
}

func WithSensitiveWordsPageHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.sensitiveWordsPage = handler
	}
}

func WithSensitiveWordIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.sensitiveWordIndex = handler
	}
}

func WithSensitiveWordStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.sensitiveWordStore = handler
	}
}

func WithSensitiveWordDestroyHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.sensitiveWordDestroy = handler
	}
}

func WithSensitiveWordStatusUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.sensitiveWordStatusUpdate = handler
	}
}

func WithSensitiveWordMoveHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.sensitiveWordMove = handler
	}
}

func WithSensitiveWordGroupSelectHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.sensitiveWordGroupSelect = handler
	}
}

func WithSensitiveWordGroupStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.sensitiveWordGroupStore = handler
	}
}

func WithSensitiveWordGroupUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.sensitiveWordGroupUpdate = handler
	}
}

func WithSensitiveWordsMonitorIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.sensitiveWordsMonitorIndex = handler
	}
}

func WithSensitiveWordsMonitorShowHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.sensitiveWordsMonitorShow = handler
	}
}

func WithContactSOPIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactSOPIndex = handler
	}
}

func WithContactSOPStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactSOPStore = handler
	}
}

func WithContactSOPSetEmployeeHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactSOPSetEmployee = handler
	}
}

func WithContactSOPStateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactSOPState = handler
	}
}

func WithContactSOPInfoHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactSOPInfo = handler
	}
}

func WithContactSOPDestroyHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactSOPDestroy = handler
	}
}

func WithContactSOPUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactSOPUpdate = handler
	}
}

func WithRoomSOPIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomSOPIndex = handler
	}
}

func WithRoomSOPStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomSOPStore = handler
	}
}

func WithRoomSOPSetRoomHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomSOPSetRoom = handler
	}
}

func WithRoomSOPStateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomSOPState = handler
	}
}

func WithRoomSOPInfoHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomSOPInfo = handler
	}
}

func WithRoomSOPDestroyHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomSOPDestroy = handler
	}
}

func WithRoomSOPUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomSOPUpdate = handler
	}
}

func WithContactSOPPageHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.contactSOPPage = handler
	}
}

func WithRoomSOPPageHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomSOPPage = handler
	}
}

func WithShopCodePageHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.shopCodePage = handler
	}
}

func WithShopCodeLocationHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.shopCodeLocation = handler
	}
}

func WithShopCodeAddressKeyWordListHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.shopCodeAddressKeyWordList = handler
	}
}

func WithShopCodeStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.shopCodeStore = handler
	}
}

func WithShopCodeUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.shopCodeUpdate = handler
	}
}

func WithShopCodeDestroyHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.shopCodeDestroy = handler
	}
}

func WithShopCodeInfoHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.shopCodeInfo = handler
	}
}

func WithShopCodeStatusHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.shopCodeStatus = handler
	}
}

func WithShopCodeIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.shopCodeIndex = handler
	}
}

func WithShopCodeSearchCityHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.shopCodeSearchCity = handler
	}
}

func WithShopCodeShareHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.shopCodeShare = handler
	}
}

func WithShopCodePageInfoHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.shopCodePageInfo = handler
	}
}

func WithShopCodePageSetHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.shopCodePageSet = handler
	}
}

func WithShopCodeShowHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.shopCodeShow = handler
	}
}

func WithShopCodeShowContactHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.shopCodeShowContact = handler
	}
}

func WithShopCodeShowShopHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.shopCodeShowShop = handler
	}
}

func WithShopCodeUpdateEmployeeHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.shopCodeUpdateEmployee = handler
	}
}

func WithShopCodeUpdateQRCodeHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.shopCodeUpdateQRCode = handler
	}
}

func WithShopCodeBatchContactTagsHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.shopCodeBatchContactTags = handler
	}
}

func WithRadarPageHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.radarPage = handler
	}
}

func WithRadarStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.radarStore = handler
	}
}

func WithRadarUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.radarUpdate = handler
	}
}

func WithRadarIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.radarIndex = handler
	}
}

func WithRadarDestroyHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.radarDestroy = handler
	}
}

func WithRadarInfoHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.radarInfo = handler
	}
}

func WithRadarStoreChannelHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.radarStoreChannel = handler
	}
}

func WithRadarStoreChannelLinkHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.radarStoreChannelLink = handler
	}
}

func WithRadarIndexChannelHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.radarIndexChannel = handler
	}
}

func WithRadarIndexChannelLinkHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.radarIndexChannelLink = handler
	}
}

func WithRadarShowHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.radarShow = handler
	}
}

func WithRadarShowContactHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.radarShowContact = handler
	}
}

func WithRadarShowChannelHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.radarShowChannel = handler
	}
}

func WithRadarArticleHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.radarArticle = handler
	}
}

func WithAutoTagStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.autoTagStore = handler
	}
}

func WithAutoTagIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.autoTagIndex = handler
	}
}

func WithAutoTagDestroyHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.autoTagDestroy = handler
	}
}

func WithAutoTagOnOffHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.autoTagOnOff = handler
	}
}

func WithAutoTagShowHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.autoTagShow = handler
	}
}

func WithAutoTagShowContactKeyWordHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.autoTagShowContactKeyWord = handler
	}
}

func WithAutoTagKeyWordTagHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.autoTagKeyWordTag = handler
	}
}

func WithAutoTagShowContactRoomHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.autoTagShowContactRoom = handler
	}
}

func WithAutoTagShowContactTimeHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.autoTagShowContactTime = handler
	}
}

func WithWorkMessageFromUsersHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workMessageFromUsers = handler
	}
}

func WithWorkMessageToUsersHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workMessageToUsers = handler
	}
}

func WithWorkMessageGlobalOverviewHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workMessageGlobalOverview = handler
	}
}

func WithWorkMessageFocusHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workMessageFocus = handler
	}
}

func WithWorkMessageStaffDirectoryHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workMessageStaffDirectory = handler
	}
}

func WithWorkMessageStaffDetailHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workMessageStaffDetail = handler
	}
}

func WithWorkMessageTrajectoryDayHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workMessageTrajectoryDay = handler
	}
}
func WithWorkMessageCustomerDirectoryHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workMessageCustomerDirectory = handler
	}
}

func WithWorkMessageCustomerConversationsHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workMessageCustomerConversations = handler
	}
}

func WithWorkMessageCustomerDetailHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workMessageCustomerDetail = handler
	}
}

func WithWorkMessageRoomDirectoryHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workMessageRoomDirectory = handler
	}
}

func WithWorkMessageRoomProfileHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workMessageRoomProfile = handler
	}
}

func WithWorkMessageRoomMessagesHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workMessageRoomMessages = handler
	}
}

func WithWorkMessageRoomMembersHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workMessageRoomMembers = handler
	}
}

func WithWorkMessageRoomFilterOptionsHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workMessageRoomFilterOptions = handler
	}
}
func WithRiskBehaviorRulesHandler(handler http.Handler) Option {
	return func(server *Server) { server.riskBehaviorRules = handler }
}
func WithRiskBehaviorRecordsHandler(handler http.Handler) Option {
	return func(server *Server) { server.riskBehaviorRecords = handler }
}
func WithRiskBehaviorRuleCreateHandler(handler http.Handler) Option {
	return func(server *Server) { server.riskBehaviorRuleCreate = handler }
}
func WithRiskBehaviorRuleUpdateHandler(handler http.Handler) Option {
	return func(server *Server) { server.riskBehaviorRuleUpdate = handler }
}
func WithRiskBehaviorRuleStatusHandler(handler http.Handler) Option {
	return func(server *Server) { server.riskBehaviorRuleStatus = handler }
}
func WithRiskBehaviorRuleDeleteHandler(handler http.Handler) Option {
	return func(server *Server) { server.riskBehaviorRuleDelete = handler }
}
func WithRiskBehaviorRecordsAuditHandler(handler http.Handler) Option {
	return func(server *Server) { server.riskBehaviorRecordsAudit = handler }
}
func WithRiskBehaviorEvaluateHandler(handler http.Handler) Option {
	return func(server *Server) { server.riskBehaviorEvaluate = handler }
}
func WithTimeoutWarningRulesHandler(handler http.Handler) Option {
	return func(s *Server) { s.timeoutWarningRules = handler }
}
func WithTimeoutWarningRecordsHandler(handler http.Handler) Option {
	return func(s *Server) { s.timeoutWarningRecords = handler }
}
func WithTimeoutWarningRuleCreateHandler(handler http.Handler) Option {
	return func(s *Server) { s.timeoutWarningRuleCreate = handler }
}
func WithTimeoutWarningRuleUpdateHandler(handler http.Handler) Option {
	return func(s *Server) { s.timeoutWarningRuleUpdate = handler }
}
func WithTimeoutWarningRuleStatusHandler(handler http.Handler) Option {
	return func(s *Server) { s.timeoutWarningRuleStatus = handler }
}
func WithTimeoutWarningRuleDeleteHandler(handler http.Handler) Option {
	return func(s *Server) { s.timeoutWarningRuleDelete = handler }
}
func WithTimeoutWarningRecordsAuditHandler(handler http.Handler) Option {
	return func(s *Server) { s.timeoutWarningRecordsAudit = handler }
}
func WithTimeoutWarningRecordsAssignHandler(handler http.Handler) Option {
	return func(s *Server) { s.timeoutWarningRecordsAssign = handler }
}
func WithTimeoutWarningSettingsHandler(handler http.Handler) Option {
	return func(s *Server) { s.timeoutWarningSettings = handler }
}
func WithTimeoutWarningSettingsUpdateHandler(handler http.Handler) Option {
	return func(s *Server) { s.timeoutWarningSettingsUpdate = handler }
}
func WithTimeoutWarningEvaluateHandler(handler http.Handler) Option {
	return func(s *Server) { s.timeoutWarningEvaluate = handler }
}
func WithKeywordLibrariesHandler(h http.Handler) Option {
	return func(s *Server) { s.keywordLibraries = h }
}
func WithKeywordLibrarySaveHandler(h http.Handler) Option {
	return func(s *Server) { s.keywordLibrarySave = h }
}
func WithKeywordLibraryStatusHandler(h http.Handler) Option {
	return func(s *Server) { s.keywordLibraryStatus = h }
}
func WithKeywordLibraryDeleteHandler(h http.Handler) Option {
	return func(s *Server) { s.keywordLibraryDelete = h }
}
func WithKeywordLibraryPublishHandler(h http.Handler) Option {
	return func(s *Server) { s.keywordLibraryPublish = h }
}
func WithKeywordEntriesHandler(h http.Handler) Option {
	return func(s *Server) { s.keywordEntries = h }
}
func WithKeywordEntrySaveHandler(h http.Handler) Option {
	return func(s *Server) { s.keywordEntrySave = h }
}
func WithKeywordEntryStatusHandler(h http.Handler) Option {
	return func(s *Server) { s.keywordEntryStatus = h }
}
func WithKeywordEntryDeleteHandler(h http.Handler) Option {
	return func(s *Server) { s.keywordEntryDelete = h }
}
func WithMessageInterceptRulesHandler(h http.Handler) Option {
	return func(s *Server) { s.messageInterceptRules = h }
}
func WithMessageInterceptRuleSaveHandler(h http.Handler) Option {
	return func(s *Server) { s.messageInterceptRuleSave = h }
}
func WithMessageInterceptRuleStatusHandler(h http.Handler) Option {
	return func(s *Server) { s.messageInterceptRuleStatus = h }
}
func WithMessageInterceptRuleDeleteHandler(h http.Handler) Option {
	return func(s *Server) { s.messageInterceptRuleDelete = h }
}
func WithMessageInterceptRecordsHandler(h http.Handler) Option {
	return func(s *Server) { s.messageInterceptRecords = h }
}
func WithMessageInterceptEvaluateHandler(h http.Handler) Option {
	return func(s *Server) { s.messageInterceptEvaluate = h }
}
func WithMessageInterceptAuditHandler(h http.Handler) Option {
	return func(s *Server) { s.messageInterceptAudit = h }
}
func WithSilentCustomerRulesHandler(h http.Handler) Option {
	return func(s *Server) { s.silentCustomerRules = h }
}
func WithSilentCustomerRuleSaveHandler(h http.Handler) Option {
	return func(s *Server) { s.silentCustomerRuleSave = h }
}
func WithSilentCustomerRuleStatusHandler(h http.Handler) Option {
	return func(s *Server) { s.silentCustomerRuleStatus = h }
}
func WithSilentCustomerRuleDeleteHandler(h http.Handler) Option {
	return func(s *Server) { s.silentCustomerRuleDelete = h }
}
func WithSilentCustomerRecordsHandler(h http.Handler) Option {
	return func(s *Server) { s.silentCustomerRecords = h }
}
func WithSilentCustomerEvaluateHandler(h http.Handler) Option {
	return func(s *Server) { s.silentCustomerEvaluate = h }
}
func WithSilentCustomerActionHandler(h http.Handler) Option {
	return func(s *Server) { s.silentCustomerAction = h }
}
func WithRefuseArchiveRecordsHandler(h http.Handler) Option {
	return func(s *Server) { s.refuseArchiveRecords = h }
}
func WithRefuseArchiveSyncHandler(h http.Handler) Option {
	return func(s *Server) { s.refuseArchiveSync = h }
}
func WithRefuseArchiveFollowUpHandler(h http.Handler) Option {
	return func(s *Server) { s.refuseArchiveFollowUp = h }
}

func WithWorkMessageIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workMessageIndex = handler
	}
}

func WithWorkMessageConfigCorpStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workMessageConfigCorpStore = handler
	}
}

func WithWorkMessageConfigCorpShowHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workMessageConfigCorpShow = handler
	}
}

func WithWorkMessageConfigCorpIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workMessageConfigCorpIndex = handler
	}
}

func WithWorkMessageConfigStepCreateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workMessageConfigStepCreate = handler
	}
}

func WithWorkMessageConfigStepUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.workMessageConfigStepUpdate = handler
	}
}

func WithLotteryPageHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.lotteryPage = handler
	}
}

func WithLotteryIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.lotteryIndex = handler
	}
}

func WithLotteryStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.lotteryStore = handler
	}
}

func WithLotteryShowContactHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.lotteryShowContact = handler
	}
}

func WithLotteryShowHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.lotteryShow = handler
	}
}

func WithLotteryDestroyHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.lotteryDestroy = handler
	}
}

func WithLotteryShareHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.lotteryShare = handler
	}
}

func WithLotteryUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.lotteryUpdate = handler
	}
}

func WithLotteryInfoHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.lotteryInfo = handler
	}
}

func WithLotteryWriteOffHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.lotteryWriteOff = handler
	}
}

func WithLotteryBatchContactTagsHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.lotteryBatchContactTags = handler
	}
}

func WithRoomFissionPageHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomFissionPage = handler
	}
}

func WithRoomFissionIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomFissionIndex = handler
	}
}

func WithRoomFissionStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomFissionStore = handler
	}
}

func WithRoomFissionInfoHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomFissionInfo = handler
	}
}

func WithRoomFissionUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomFissionUpdate = handler
	}
}

func WithRoomFissionDestroyHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomFissionDestroy = handler
	}
}

func WithRoomFissionInviteHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomFissionInvite = handler
	}
}

func WithRoomFissionShowHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomFissionShow = handler
	}
}

func WithRoomFissionShowRoomHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomFissionShowRoom = handler
	}
}

func WithRoomFissionShowContactHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomFissionShowContact = handler
	}
}

func WithRoomFissionWriteOffHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomFissionWriteOff = handler
	}
}

func WithRoomClockInPageHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomClockInPage = handler
	}
}

func WithRoomClockInIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomClockInIndex = handler
	}
}

func WithRoomClockInStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomClockInStore = handler
	}
}

func WithRoomClockInUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomClockInUpdate = handler
	}
}

func WithRoomClockInDestroyHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomClockInDestroy = handler
	}
}

func WithRoomClockInShowHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomClockInShow = handler
	}
}

func WithRoomClockInShowContactHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomClockInShowContact = handler
	}
}

func WithRoomClockInBatchContactTagsHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomClockInBatchContactTags = handler
	}
}

func WithRoomClockInInfoHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomClockInInfo = handler
	}
}

func WithRoomClockInDayDetailHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomClockInDayDetail = handler
	}
}

func WithRoomQualityPageHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomQualityPage = handler
	}
}

func WithRoomQualityIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomQualityIndex = handler
	}
}

func WithRoomQualityStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomQualityStore = handler
	}
}

func WithRoomQualityStatusHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomQualityStatus = handler
	}
}

func WithRoomQualityInfoHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomQualityInfo = handler
	}
}

func WithRoomQualityUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomQualityUpdate = handler
	}
}

func WithRoomQualityShowContactHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomQualityShowContact = handler
	}
}

func WithRoomQualityDestroyHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomQualityDestroy = handler
	}
}

func WithRoomQualityContactDetailHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomQualityContactDetail = handler
	}
}

func WithRoomCalendarPageHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomCalendarPage = handler
	}
}

func WithRoomCalendarIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomCalendarIndex = handler
	}
}

func WithRoomCalendarAddRoomHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomCalendarAddRoom = handler
	}
}

func WithRoomCalendarDestroyRoomHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomCalendarDestroyRoom = handler
	}
}

func WithRoomCalendarStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomCalendarStore = handler
	}
}

func WithRoomCalendarDestroyHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomCalendarDestroy = handler
	}
}

func WithRoomCalendarShowHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomCalendarShow = handler
	}
}

func WithRoomCalendarUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomCalendarUpdate = handler
	}
}

func WithRoomRemindPageHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomRemindPage = handler
	}
}

func WithRoomRemindIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomRemindIndex = handler
	}
}

func WithRoomRemindDestroyHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomRemindDestroy = handler
	}
}

func WithRoomRemindInfoHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomRemindInfo = handler
	}
}

func WithRoomRemindStatusHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomRemindStatus = handler
	}
}

func WithRoomRemindStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomRemindStore = handler
	}
}

func WithRoomRemindUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomRemindUpdate = handler
	}
}

func WithRoomRemindTaskHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomRemindTask = handler
	}
}

func WithRoomInfinitePullPageHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomInfinitePullPage = handler
	}
}

func WithRoomInfinitePullIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomInfinitePullIndex = handler
	}
}

func WithRoomInfinitePullInfoHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomInfinitePullInfo = handler
	}
}

func WithRoomInfinitePullUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomInfinitePullUpdate = handler
	}
}

func WithRoomInfinitePullDestroyHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomInfinitePullDestroy = handler
	}
}

func WithRoomInfinitePullStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roomInfinitePullStore = handler
	}
}

func WithSidebarContactBatchAddDetailHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.sidebarContactBatchAddDetail = handler
	}
}

func WithSidebarContactSOPGetInfoHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.sidebarContactSOPInfo = handler
	}
}

func WithSidebarContactSOPGetTipInfoHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.sidebarContactSOPTipInfo = handler
	}
}

func WithSidebarRoomSOPGetInfoHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.sidebarRoomSOPInfo = handler
	}
}

func WithSidebarRoomSOPLogStateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.sidebarRoomSOPLogState = handler
	}
}

func WithSaaSAlertIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAlertIndex = handler
	}
}

func WithSaaSAlertPageHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAlertPage = handler
	}
}

func WithSaaSAlertResolveHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAlertResolve = handler
	}
}

func WithSaaSAlertSettingHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAlertSetting = handler
	}
}

func WithSaaSBillingPageHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasBillingPage = handler }
}

func WithSaaSBillingSummaryHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasBillingSummary = handler }
}

func WithSaaSBillingPaymentOrdersHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasBillingPaymentOrders = handler }
}

func WithSaaSBillingPaymentRefundsHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasBillingPaymentRefunds = handler }
}

func WithSaaSBillingInvoiceProfileHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasBillingInvoiceProfile = handler }
}

func WithSaaSBillingInvoicesHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasBillingInvoices = handler }
}

func WithSaaSBillingInvoiceHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasBillingInvoice = handler }
}

func WithSaaSBillingInvoiceCancelHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasBillingInvoiceCancel = handler }
}

func WithSaaSAdminPageHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminPage = handler
	}
}

func WithSaaSAdminOverviewHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminOverview = handler
	}
}

func WithSaaSAdminTenantReadinessHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminTenantReadiness = handler
	}
}

func WithSaaSAdminTenantHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminTenant = handler
	}
}

func WithSaaSAdminTenantLifecycleHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminTenantLifecycle = handler
	}
}

func WithSaaSAdminUsageHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminUsage = handler
	}
}

func WithSaaSAdminRiskHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminRisk = handler
	}
}

func WithSaaSAdminBusinessMetricsHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminBusinessMetrics = handler
	}
}

func WithSaaSAdminBusinessTrendsHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminBusinessTrends = handler
	}
}

func WithSaaSAdminOperationQueueHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminOperationQueue = handler
	}
}

func WithSaaSAdminOperationQueueOwnersHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminOperationQueueOwners = handler
	}
}

func WithSaaSAdminOperationQueueAssignmentsHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminOperationQueueAssignments = handler
	}
}

func WithSaaSAdminOperationQueueAssignmentCloseHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminOperationQueueAssignmentClose = handler
	}
}

func WithSaaSAdminOperationQueueAssignmentNotificationsHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminOperationQueueAssignmentNotifications = handler
	}
}

func WithSaaSAdminOperationQueueAssignHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminOperationQueueAssign = handler
	}
}

func WithSaaSAdminRenewalForecastHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminRenewalForecast = handler
	}
}

func WithSaaSAdminRenewalForecastTasksHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminRenewalForecastTasks = handler
	}
}

func WithSaaSAdminRenewalForecastAssignHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminRenewalForecastAssign = handler
	}
}

func WithSaaSAdminCustomerSuccessHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminCustomerSuccess = handler
	}
}

func WithSaaSAdminCustomerSuccessOwnersHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminCustomerSuccessOwners = handler
	}
}

func WithSaaSAdminCustomerSuccessAssignHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminCustomerSuccessAssign = handler
	}
}

func WithSaaSAdminCustomerSuccessRenewalTasksHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminCustomerSuccessRenewalTasks = handler
	}
}

func WithSaaSAdminCustomerSuccessRenewalNotificationsHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminCustomerSuccessRenewalNotifications = handler
	}
}

func WithSaaSAdminRenewalForecastNotificationsHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminRenewalForecastNotifications = handler
	}
}

func WithSaaSAdminRiskFollowUpHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminRiskFollowUp = handler
	}
}

func WithSaaSAdminRiskFollowUpsHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminRiskFollowUps = handler
	}
}

func WithSaaSAdminRiskFollowUpOwnersHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminRiskFollowUpOwners = handler
	}
}

func WithSaaSAdminRiskFollowUpBulkCloseHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminRiskFollowUpBulkClose = handler
	}
}

func WithSaaSAdminAlertsHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminAlerts = handler
	}
}

func WithSaaSAdminAlertResolveHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminAlertResolve = handler
	}
}

func WithSaaSAdminAlertBulkResolveHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminAlertBulkResolve = handler
	}
}

func WithSaaSAdminNotificationsHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminNotifications = handler
	}
}

func WithSaaSAdminNotificationHealthHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminNotificationHealth = handler
	}
}

func WithSaaSAdminNotificationSLOHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminNotificationSLO = handler
	}
}

func WithSaaSAdminNotificationHealthRecoveryHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminNotificationHealthRecovery = handler
	}
}

func WithSaaSAdminNotificationPoliciesHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminNotificationPolicies = handler
	}
}

func WithSaaSAdminNotificationPolicyHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminNotificationPolicy = handler
	}
}

func WithSaaSAdminNotificationPolicyTestHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminNotificationPolicyTest = handler
	}
}

func WithSaaSAdminNotificationCredentialRotationHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminNotificationCredentialRotation = handler
	}
}

func WithSaaSAdminWeComCredentialProtectionHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminWeComCredentialProtection = handler
	}
}

func WithSaaSAdminWeComCredentialRotationHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminWeComCredentialRotation = handler
	}
}

func WithSaaSAdminWeChatOpenCredentialProtectionHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminWeChatOpenCredentialProtection = handler
	}
}

func WithSaaSAdminWeChatOpenCredentialRotationHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminWeChatOpenCredentialRotation = handler
	}
}

func WithSaaSAdminNotificationRetryHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminNotificationRetry = handler
	}
}

func WithSaaSAdminNotificationBulkRetryHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminNotificationBulkRetry = handler
	}
}

func WithSaaSAdminNotificationCloseHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminNotificationClose = handler
	}
}

func WithSaaSAdminNotificationBulkCloseHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminNotificationBulkClose = handler
	}
}

func WithSaaSAdminPackagesHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminPackages = handler
	}
}

func WithSaaSAdminSubscriptionsHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminSubscriptions = handler
	}
}

func WithSaaSAdminSubscriptionEventsHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminSubscriptionEvents = handler
	}
}

func WithSaaSAdminSubscriptionTransitionHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminSubscriptionTransition = handler
	}
}

func WithSaaSAdminSubscriptionReconcileHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminSubscriptionReconcile = handler
	}
}

func WithSaaSAdminPaymentOrdersHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminPaymentOrders = handler
	}
}

func WithSaaSAdminPaymentWebhookEventsHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminPaymentWebhookEvents = handler
	}
}

func WithSaaSAdminPaymentOrderHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminPaymentOrder = handler
	}
}

func WithSaaSAdminPaymentOrderCancelHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminPaymentOrderCancel = handler
	}
}

func WithSaaSAdminPaymentDunningHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminPaymentDunning = handler
	}
}

func WithSaaSAdminPaymentRefundsHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminPaymentRefunds = handler
	}
}

func WithSaaSAdminPaymentRefundHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminPaymentRefund = handler
	}
}

func WithSaaSAdminPaymentRefundCancelHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminPaymentRefundCancel = handler
	}
}

func WithSaaSAdminPaymentSettlementBatchesHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminPaymentSettlementBatches = handler }
}

func WithSaaSAdminPaymentSettlementEntriesHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminPaymentSettlementEntries = handler }
}

func WithSaaSAdminPaymentSettlementImportHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminPaymentSettlementImport = handler }
}

func WithSaaSAdminPaymentSettlementReconcileHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminPaymentSettlementReconcile = handler }
}

func WithSaaSAdminPaymentSettlementResolveHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminPaymentSettlementResolve = handler }
}

func WithSaaSAdminPaymentSettlementTransitionHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminPaymentSettlementTransition = handler }
}

func WithSaaSAdminPaymentSettlementSyncRunsHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminPaymentSettlementSyncRuns = handler }
}

func WithSaaSAdminPaymentSettlementSyncHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminPaymentSettlementSync = handler }
}

func WithSaaSAdminAccessProfileHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminAccessProfile = handler }
}

func WithSaaSAdminAccessRolesHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminAccessRoles = handler }
}

func WithSaaSAdminAccessRoleHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminAccessRole = handler }
}

func WithSaaSAdminAccessAssignmentsHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminAccessAssignments = handler }
}

func WithSaaSAdminAccessAssignmentHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminAccessAssignment = handler }
}

func WithSaaSAdminSystemHealthHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminSystemHealth = handler }
}

func WithSaaSAdminSystemHealthScansHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminSystemHealthScans = handler }
}

func WithSaaSAdminSystemIncidentsHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminSystemIncidents = handler }
}

func WithSaaSAdminSystemHealthScanHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminSystemHealthScan = handler }
}

func WithSaaSAdminSystemIncidentHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminSystemIncident = handler }
}

func WithSaaSAdminServiceAccountsHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminServiceAccounts = handler }
}

func WithSaaSAdminServiceAccountUsageHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminServiceAccountUsage = handler }
}

func WithSaaSAdminServiceAccountUsageAlertEvaluateHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminServiceAccountUsageAlertEvaluate = handler }
}

func WithSaaSAdminServiceAccountHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminServiceAccount = handler }
}

func WithSaaSAdminServiceAccountKeyRotateHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminServiceAccountKeyRotate = handler }
}

func WithSaaSAdminServiceAccountKeyRevokeHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminServiceAccountKeyRevoke = handler }
}

func WithSaaSAdminAuditIntegrityHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminAuditIntegrity = handler }
}

func WithSaaSAdminAuditIntegrityVerifyHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminAuditIntegrityVerify = handler }
}

func WithSaaSAdminAuditAnchorsHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminAuditAnchors = handler }
}

func WithSaaSAdminAuditAnchorHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminAuditAnchor = handler }
}

func WithSaaSAdminBackupOverviewHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminBackupOverview = handler }
}

func WithSaaSAdminBackupPolicyHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminBackupPolicy = handler }
}

func WithSaaSAdminBackupRunHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminBackupRun = handler }
}

func WithSaaSAdminRestoreDrillHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminRestoreDrill = handler }
}

func WithSaaSAdminComplianceOverviewHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminComplianceOverview = handler }
}

func WithSaaSAdminCompliancePolicyHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminCompliancePolicy = handler }
}

func WithSaaSAdminComplianceLegalHoldHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminComplianceLegalHold = handler }
}

func WithSaaSAdminComplianceExportHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminComplianceExport = handler }
}

func WithSaaSAdminComplianceExportDownloadHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminComplianceExportDownload = handler }
}

func WithSaaSAdminComplianceErasureHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminComplianceErasure = handler }
}

func WithSaaSAdminComplianceErasureStepsHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminComplianceErasureSteps = handler }
}

func WithSaaSAdminIdentityOverviewHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminIdentityOverview = handler }
}

func WithSaaSAdminIdentityPolicyHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminIdentityPolicy = handler }
}

func WithSaaSAdminIdentitySessionsHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminIdentitySessions = handler }
}

func WithSaaSAdminIdentitySessionHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminIdentitySession = handler }
}

func WithSaaSAdminIdentityLoginEventsHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminIdentityLoginEvents = handler }
}

func WithSaaSAdminIdentityIncidentsHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminIdentityIncidents = handler }
}

func WithSaaSAdminIdentityIncidentHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminIdentityIncident = handler }
}

func WithSaaSAdminIdentityUserHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminIdentityUser = handler }
}

func WithSaaSAdminIdentityMFAHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminIdentityMFA = handler }
}

func WithSaaSAdminBrandingProfilesHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminBrandingProfiles = handler }
}

func WithSaaSAdminBrandingProfileHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminBrandingProfile = handler }
}

func WithSaaSAdminTenantDomainsHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminTenantDomains = handler }
}

func WithSaaSAdminTenantDomainHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminTenantDomain = handler }
}

func WithSaaSAdminTenantDomainDeliveryJobsHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminTenantDomainDeliveryJobs = handler }
}

func WithSaaSAdminTenantDomainDeliveryHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminTenantDomainDelivery = handler }
}

func WithSaaSAdminReleaseReadinessHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminReleaseReadiness = handler }
}

func WithSaaSAdminReleaseEvidenceHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminReleaseEvidence = handler }
}

func WithSaaSAdminReleaseEvidenceActionHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminReleaseEvidenceAction = handler }
}

func WithSaaSAdminReleaseCandidateHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminReleaseCandidate = handler }
}

func WithSaaSServiceAccountWhoAmIHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasServiceAccountWhoAmI = handler }
}

func WithSaaSServiceAccountUsageHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasServiceAccountUsage = handler }
}

func WithSaaSServiceAccountAlertsHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasServiceAccountAlerts = handler }
}

func WithSaaSAdminApprovalPoliciesHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminApprovalPolicies = handler }
}

func WithSaaSAdminApprovalPolicyHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminApprovalPolicy = handler }
}

func WithSaaSAdminApprovalsHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminApprovals = handler }
}

func WithSaaSAdminApprovalEventsHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminApprovalEvents = handler }
}

func WithSaaSAdminApprovalDecisionsHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminApprovalDecisions = handler }
}

func WithSaaSAdminApprovalDelegationsHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminApprovalDelegations = handler }
}

func WithSaaSAdminApprovalDelegationHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminApprovalDelegation = handler }
}

func WithSaaSAdminApprovalRemindersHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminApprovalReminders = handler }
}

func WithSaaSAdminApprovalRequestHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminApprovalRequest = handler }
}

func WithSaaSAdminApprovalDecisionHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminApprovalDecision = handler }
}

func WithSaaSAdminApprovalCancelHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminApprovalCancel = handler }
}

func WithSaaSAdminApprovalExecuteHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminApprovalExecute = handler }
}

func WithSaaSAdminInvoiceProfileHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminInvoiceProfile = handler }
}

func WithSaaSAdminInvoiceDocumentsHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminInvoiceDocuments = handler }
}

func WithSaaSAdminInvoiceHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminInvoice = handler }
}

func WithSaaSAdminCreditNoteHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminCreditNote = handler }
}

func WithSaaSAdminInvoiceTransitionHandler(handler http.Handler) Option {
	return func(server *Server) { server.saasAdminInvoiceTransition = handler }
}

func WithSaaSPaymentWebhookHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasPaymentWebhook = handler
	}
}

func WithSaaSTenantDomainDeliveryWebhookHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasTenantDomainDeliveryWebhook = handler
	}
}

func WithSaaSAdminOperationsHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminOperations = handler
	}
}

func WithSaaSAdminBillingEventsHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminBillingEvents = handler
	}
}

func WithSaaSAdminTasksHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminTasks = handler
	}
}

func WithSaaSAdminTaskOwnersHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminTaskOwners = handler
	}
}

func WithSaaSAdminTaskSLAHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminTaskSLA = handler
	}
}

func WithSaaSAdminTaskSLANotificationsHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminTaskSLANotifications = handler
	}
}

func WithSaaSAdminTaskCancelHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminTaskCancel = handler
	}
}

func WithSaaSAdminTaskBulkCancelHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminTaskBulkCancel = handler
	}
}

func WithSaaSAdminTaskBulkResetHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminTaskBulkReset = handler
	}
}

func WithSaaSAdminTaskResetHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminTaskReset = handler
	}
}

func WithSaaSAdminDailyReportHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminDailyReport = handler
	}
}

func WithSaaSAdminExportHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminExport = handler
	}
}

func WithSaaSAdminPackageHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminPackage = handler
	}
}

func WithSaaSAdminPackageSyncHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminPackageSync = handler
	}
}

func WithSaaSAdminPackageSyncTaskHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminPackageSyncTask = handler
	}
}

func WithSaaSAdminPackageSyncTaskApplyHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminPackageSyncTaskApply = handler
	}
}

func WithSaaSAdminPackageSyncTaskBulkApplyHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminPackageSyncTaskBulkApply = handler
	}
}

func WithSaaSAdminTenantStatusHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminTenantStatus = handler
	}
}

func WithSaaSAdminTenantRenewalHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminTenantRenewal = handler
	}
}

func WithSaaSAdminBillingReconciliationHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminBillingReconciliation = handler
	}
}

func WithSaaSAdminBillingReconciliationFollowUpHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminBillingReconciliationFollowUp = handler
	}
}

func WithSaaSAdminBillingReconciliationFollowUpsHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminBillingReconciliationFollowUps = handler
	}
}

func WithSaaSAdminBillingReconciliationFollowUpOwnersHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminBillingReconciliationFollowUpOwners = handler
	}
}

func WithSaaSAdminBillingReconciliationFollowUpBulkCloseHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminBillingReconciliationFollowUpBulkClose = handler
	}
}

func WithSaaSAdminTenantRenewalTaskHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminTenantRenewalTask = handler
	}
}

func WithSaaSAdminTenantRenewalTaskApplyHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminTenantRenewalTaskApply = handler
	}
}

func WithSaaSAdminTenantRenewalTaskBulkApplyHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminTenantRenewalTaskBulkApply = handler
	}
}

func WithSaaSAdminTenantProvisionHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminTenantProvision = handler
	}
}

func WithSaaSAdminTenantProvisionTaskHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminTenantProvisionTask = handler
	}
}

func WithSaaSAdminTenantProvisionTaskApplyHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminTenantProvisionTaskApply = handler
	}
}

func WithSaaSAdminTenantProvisionTaskBulkApplyHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminTenantProvisionTaskBulkApply = handler
	}
}

func WithSaaSAdminTenantPackageHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminTenantPackage = handler
	}
}

func WithSaaSAdminDashboardProvisioningHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.saasAdminDashboardProvisioning = handler
	}
}

func WithChatToolConfigHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.chatToolConfig = handler
	}
}

func WithCommonUploadHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.commonUpload = handler
	}
}

func WithCommonUploadFileHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.commonUploadFile = handler
	}
}

func WithSidebarCommonUploadHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.sidebarCommonUpload = handler
	}
}

func WithAgentTxtUploadHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.agentTxtUpload = handler
	}
}

func WithAgentTxtVerify() Option {
	return func(server *Server) {
		server.agentTxtVerify = true
	}
}

func WithDashboardAgentStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.agentStore = handler
	}
}

func WithSidebarAgentAuthHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.sidebarAgentAuth = handler
	}
}

func WithSidebarAgentOAuthHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.sidebarAgentOAuth = handler
	}
}

func WithSidebarAgentJSSDKHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.sidebarAgentJSSDK = handler
	}
}

func WithSidebarWxJSSDKHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.sidebarWxJSSDK = handler
	}
}

func WithRoleSelectHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roleSelect = handler
	}
}

func WithRoleIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roleIndex = handler
	}
}

func WithRoleShowHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roleShow = handler
	}
}

func WithRolePermissionShowHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.rolePermission = handler
	}
}

func WithRoleShowEmployeeHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roleShowEmployee = handler
	}
}

func WithRoleStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roleStore = handler
	}
}

func WithRoleUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roleUpdate = handler
	}
}

func WithRoleStatusUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roleStatusUpdate = handler
	}
}

func WithRoleDestroyHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.roleDestroy = handler
	}
}

func WithRolePermissionStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.rolePermissionStore = handler
	}
}

func WithMenuIconIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.menuIconIndex = handler
	}
}

func WithMenuSelectHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.menuSelect = handler
	}
}

func WithMenuIndexHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.menuIndex = handler
	}
}

func WithMenuShowHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.menuShow = handler
	}
}

func WithMenuStoreHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.menuStore = handler
	}
}

func WithMenuUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.menuUpdate = handler
	}
}

func WithMenuStatusUpdateHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.menuStatusUpdate = handler
	}
}

func WithMenuDestroyHandler(handler http.Handler) Option {
	return func(server *Server) {
		server.menuDestroy = handler
	}
}

func New(cfg config.Config, options ...Option) (*Server, error) {
	var proxy http.Handler
	if cfg.PHPUpstream != "" {
		target, err := url.Parse(cfg.PHPUpstream)
		if err != nil {
			return nil, err
		}
		reverseProxy := httputil.NewSingleHostReverseProxy(target)
		originalDirector := reverseProxy.Director
		reverseProxy.Director = func(req *http.Request) {
			originalDirector(req)
			req.Header.Set("X-Mochat-Go-Compat", "fallback-proxy")
			req.Header.Set("X-Mochat-Go-Compat-Upstream", target.Host)
		}
		reverseProxy.ModifyResponse = func(res *http.Response) error {
			res.Header.Set("X-Mochat-Go-Compat", "fallback-proxy")
			return nil
		}
		reverseProxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			writeJSON(w, http.StatusBadGateway, map[string]any{
				"code":    http.StatusBadGateway,
				"message": "PHP upstream fallback failed",
				"error":   err.Error(),
			})
		}
		proxy = timeoutHandler(reverseProxy, cfg.ProxyTimeout)
	}

	server := &Server{
		cfg:       cfg,
		startedAt: time.Now().UTC(),
		proxy:     proxy,
	}
	for _, option := range options {
		option(server)
	}
	return server, nil
}

func isSaaSAdminDashboardProvisioningRoute(method, path string) bool {
	const prefix = "/dashboard/saasAdmin/tenants/"
	if method == http.MethodGet {
		if !strings.HasPrefix(path, prefix) {
			return false
		}
		rest := strings.TrimPrefix(path, prefix)
		idText, suffix, ok := strings.Cut(rest, "/")
		if !ok || suffix != "dashboard-admins" {
			return false
		}
		tenantID, err := strconv.Atoi(idText)
		return err == nil && tenantID > 0 && strconv.Itoa(tenantID) == idText
	}
	if method != http.MethodPost {
		return false
	}
	if path == "/dashboard/saasAdmin/tenants/provision" {
		return true
	}
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	rest := strings.TrimPrefix(path, prefix)
	idText, suffix, ok := strings.Cut(rest, "/")
	if !ok || (suffix != "activation/resend" && suffix != "super-admin/replace" && suffix != "super-admin/status") {
		return false
	}
	tenantID, err := strconv.Atoi(idText)
	return err == nil && tenantID > 0 && strconv.Itoa(tenantID) == idText
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if normalizedPath, ok := normalizeBundledFrontendAPIPath(r.URL.Path); ok {
		normalized := r.Clone(r.Context())
		normalized.URL = cloneURL(r.URL)
		normalized.URL.Path = normalizedPath
		normalized.URL.RawPath = ""
		s.ServeHTTP(w, normalized)
		return
	}
	if isRetiredDashboardEndpoint(r.URL.Path) {
		http.NotFound(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/dashboard/") && !isDashboardSaaSRequestPath(r.URL.Path) && !nilcheck.IsNil(s.dashboardRequestGuard) {
		if !s.dashboardRequestGuard.Authorize(w, r) {
			return
		}
	}
	if isDashboardSaaSRequestPath(r.URL.Path) && !nilcheck.IsNil(s.saasRequestGuard) {
		if !s.saasRequestGuard.Authorize(w, r) {
			return
		}
	}
	if strings.HasPrefix(r.URL.Path, "/dashboard/access/") && !nilcheck.IsNil(s.dashboardAccess) {
		if !nilcheck.IsNil(s.moduleRouter) {
			if handler, ok := s.moduleRouter.Match(r); ok && !nilcheck.IsNil(handler) {
				handler.ServeHTTP(w, r)
				return
			}
		}
		s.dashboardAccess.ServeHTTP(w, r)
		return
	}
	if !nilcheck.IsNil(s.moduleRouter) {
		if handler, ok := s.moduleRouter.Match(r); ok {
			if !nilcheck.IsNil(handler) {
				handler.ServeHTTP(w, r)
				return
			}
		}
	}

	switch {
	case strings.HasPrefix(r.URL.Path, "/saas/auth/") && s.saasAuth != nil:
		s.saasAuth.ServeHTTP(w, r)
	case r.URL.Path == "/saas/login" && (r.Method == http.MethodGet || r.Method == http.MethodHead) && s.saasLoginPage != nil:
		s.saasLoginPage.ServeHTTP(w, r)
	case r.URL.Path == "/" && (r.Method == http.MethodGet || r.Method == http.MethodPost || r.Method == http.MethodHead):
		s.handleRoot(w, r)
	case r.URL.Path == "/favicon.ico" && r.Method == http.MethodGet:
		w.WriteHeader(http.StatusOK)
	case r.URL.Path == "/healthz" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": "mochat-go"})
	case r.URL.Path == "/readyz" && r.Method == http.MethodGet:
		status := http.StatusOK
		payload := s.status()
		if s.cfg.Standalone {
			payload.PHPUpstreamReady = false
			payload.PHPUpstreamProbe = "standalone mode: PHP upstream disabled"
			if len(embeddedCompatManifest) == 0 {
				status = http.StatusServiceUnavailable
			}
		} else {
			ready, probe := s.probePHPUpstream(r.Context())
			payload.PHPUpstreamReady = ready
			payload.PHPUpstreamProbe = probe
			if !payload.SourceRootExists || !payload.ManifestExists || !payload.ProxyFallbackEnabled || !payload.PHPUpstreamReady {
				status = http.StatusServiceUnavailable
			}
		}
		writeJSON(w, status, payload)
	case strings.HasPrefix(r.URL.Path, "/static/") && (r.Method == http.MethodGet || r.Method == http.MethodHead):
		s.serveStaticUpload(w, r)
	case r.URL.Path == "/compat/status" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, s.status())
	case r.URL.Path == "/compat/routes" && r.Method == http.MethodGet:
		s.handleRoutes(w)
	case s.dashboardAuth != nil && r.URL.Path == "/dashboard/user/auth" && r.Method == http.MethodPost:
		s.dashboardAuth.ServeHTTP(w, r)
	case s.dashboardAuth != nil && r.URL.Path == "/dashboard/user/authMFA" && r.Method == http.MethodPost:
		s.dashboardAuth.ServeHTTP(w, r)
	case s.dashboardAuth != nil && r.URL.Path == "/dashboard/auth/activate" && r.Method == http.MethodPost:
		s.dashboardAuth.ServeHTTP(w, r)
	case s.dashboardAuth != nil && r.URL.Path == "/dashboard/auth/password/reset-request" && r.Method == http.MethodPost:
		s.dashboardAuth.ServeHTTP(w, r)
	case s.dashboardAuth != nil && r.URL.Path == "/dashboard/auth/password/reset" && r.Method == http.MethodPost:
		s.dashboardAuth.ServeHTTP(w, r)
	case s.dashboardAuth != nil && r.URL.Path == "/dashboard/auth/session" && r.Method == http.MethodGet:
		s.dashboardAuth.ServeHTTP(w, r)
	case s.dashboardAuth != nil && r.URL.Path == "/dashboard/auth/logout" && r.Method == http.MethodPost:
		s.dashboardAuth.ServeHTTP(w, r)
	case s.dashboardAuth != nil && r.URL.Path == "/dashboard/user/logout" && r.Method == http.MethodPut:
		s.dashboardAuth.ServeHTTP(w, r)
	case s.companyProfile != nil && r.URL.Path == "/dashboard/company/profile" && (r.Method == http.MethodGet || r.Method == http.MethodPut):
		s.companyProfile.ServeHTTP(w, r)
	case s.companyProfile != nil && r.URL.Path == "/dashboard/company/wecom-credentials" && r.Method == http.MethodPut:
		s.companyProfile.ServeHTTP(w, r)
	case s.companyProfile != nil && r.URL.Path == "/dashboard/company/agent-credentials" && r.Method == http.MethodPut:
		s.companyProfile.ServeHTTP(w, r)
	case s.companyProfile != nil && r.URL.Path == "/dashboard/company/application-credentials" && r.Method == http.MethodPut:
		s.companyProfile.ServeHTTP(w, r)
	case s.companyProfile != nil && r.URL.Path == "/dashboard/company/archive-credentials" && r.Method == http.MethodPut:
		s.companyProfile.ServeHTTP(w, r)
	case s.companyProfile != nil && r.URL.Path == "/dashboard/company/callback-configuration" && r.Method == http.MethodGet:
		s.companyProfile.ServeHTTP(w, r)
	case s.companyProfile != nil && r.URL.Path == "/dashboard/company/callback-configuration/regenerate" && r.Method == http.MethodPost:
		s.companyProfile.ServeHTTP(w, r)
	case s.companyProfile != nil && r.URL.Path == "/dashboard/company/verify" && r.Method == http.MethodPost:
		s.companyProfile.ServeHTTP(w, r)
	case s.companyProfile != nil && r.URL.Path == "/dashboard/company/employee-sync" && r.Method == http.MethodPost:
		s.companyProfile.ServeHTTP(w, r)
	case s.companyProfile != nil && r.URL.Path == "/dashboard/company/sync-status" && r.Method == http.MethodGet:
		s.companyProfile.ServeHTTP(w, r)
	case s.companyProfile != nil && r.URL.Path == "/dashboard/company/audits" && r.Method == http.MethodGet:
		s.companyProfile.ServeHTTP(w, r)
	case s.providerStatus != nil && r.URL.Path == "/dashboard/providers/status" && r.Method == http.MethodGet:
		s.providerStatus.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/user/securityMFA" && (r.Method == http.MethodGet || r.Method == http.MethodPost || r.Method == http.MethodPut) && s.identitySelf != nil:
		s.identitySelf.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/user/logout" && r.Method == http.MethodPut && s.logout != nil:
		s.logout.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/user/index" && r.Method == http.MethodGet && s.userIndex != nil:
		s.userIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/user/show" && r.Method == http.MethodGet && s.userShow != nil:
		s.userShow.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/user/store" && r.Method == http.MethodPost && s.userStore != nil:
		s.userStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/user/update" && r.Method == http.MethodPut && s.userUpdate != nil:
		s.userUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/user/statusUpdate" && r.Method == http.MethodPut && s.userStatusUpdate != nil:
		s.userStatusUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/user/passwordReset" && r.Method == http.MethodPut && s.userPasswordReset != nil:
		s.userPasswordReset.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/user/passwordUpdate" && r.Method == http.MethodPut && s.userPasswordUpdate != nil:
		s.userPasswordUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/role/permissionByUser" && r.Method == http.MethodGet && s.permissionByUser != nil:
		s.permissionByUser.ServeHTTP(w, r)
	case (r.URL.Path == "/weWork/callback" || r.URL.Path == "/dashboard/corp/weWorkCallback") && (r.Method == http.MethodGet || r.Method == http.MethodPost) && s.weWorkCallback != nil:
		s.weWorkCallback.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/corpData/index" && r.Method == http.MethodGet && s.corpDataIndex != nil:
		s.corpDataIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/corpData/lineChat" && r.Method == http.MethodGet && s.corpDataLineChat != nil:
		s.corpDataLineChat.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/external/tenantIndex" && r.Method == http.MethodGet && s.externalTenantIndex != nil:
		s.externalTenantIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/statistic/index" && r.Method == http.MethodGet && s.statisticIndex != nil:
		s.statisticIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/statistic/topList" && r.Method == http.MethodGet && s.statisticTopList != nil:
		s.statisticTopList.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/statistic/employeeCounts" && r.Method == http.MethodGet && s.statisticEmployeeCounts != nil:
		s.statisticEmployeeCounts.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/statistic/employees" && r.Method == http.MethodGet && s.statisticEmployees != nil:
		s.statisticEmployees.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/statistic/employeesTrend" && r.Method == http.MethodGet && s.statisticEmployeesTrend != nil:
		s.statisticEmployeesTrend.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workEmployee/searchCondition" && r.Method == http.MethodGet && s.workEmployeeCond != nil:
		s.workEmployeeCond.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workEmployee/index" && r.Method == http.MethodGet && s.workEmployeeIndex != nil:
		s.workEmployeeIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workEmployee/synEmployee" && r.Method == http.MethodPut && s.workEmployeeSync != nil:
		s.workEmployeeSync.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workDepartment/index" && r.Method == http.MethodGet && s.workDeptIndex != nil:
		s.workDeptIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workEmployeeDepartment/memberIndex" && r.Method == http.MethodGet && s.workDeptMember != nil:
		s.workDeptMember.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workDepartment/memberIndex" && r.Method == http.MethodGet && s.workDeptMember != nil:
		serveHTTPWithPath(s.workDeptMember, w, r, "/dashboard/workEmployeeDepartment/memberIndex")
	case r.URL.Path == "/dashboard/workDepartment/selectByPhone" && r.Method == http.MethodGet && s.workDeptPhone != nil:
		s.workDeptPhone.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workDepartment/pageIndex" && r.Method == http.MethodGet && s.workDeptPage != nil:
		s.workDeptPage.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workDepartment/showEmployee" && r.Method == http.MethodGet && s.workDeptEmployee != nil:
		s.workDeptEmployee.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workContactTagGroup/index" && r.Method == http.MethodGet && s.workTagGroupIndex != nil:
		s.workTagGroupIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workContactTagGroup/detail" && r.Method == http.MethodGet && s.workTagGroupDetail != nil:
		s.workTagGroupDetail.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workContactTagGroup/store" && r.Method == http.MethodPost && s.workTagGroupStore != nil:
		s.workTagGroupStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workContactTagGroup/update" && r.Method == http.MethodPut && s.workTagGroupUpdate != nil:
		s.workTagGroupUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workContactTagGroup/destroy" && r.Method == http.MethodDelete && s.workTagGroupDestroy != nil:
		s.workTagGroupDestroy.ServeHTTP(w, r)
	case r.URL.Path == "/sidebar/workContactTagGroup/index" && r.Method == http.MethodGet && s.sidebarTagGroupIndex != nil:
		s.sidebarTagGroupIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workContactTag/index" && r.Method == http.MethodGet && s.workContactTagIndex != nil:
		s.workContactTagIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workContactTag/detail" && r.Method == http.MethodGet && s.workContactTagDetail != nil:
		s.workContactTagDetail.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workContactTag/contactTagList" && r.Method == http.MethodGet && s.workContactTagList != nil:
		s.workContactTagList.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workContactTag/allTag" && r.Method == http.MethodGet && s.workContactTagAll != nil:
		s.workContactTagAll.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workContactTag/store" && r.Method == http.MethodPost && s.workContactTagStore != nil:
		s.workContactTagStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workContactTag/update" && r.Method == http.MethodPut && s.workContactTagUpdate != nil:
		s.workContactTagUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workContactTag/destroy" && r.Method == http.MethodDelete && s.workContactTagDestroy != nil:
		s.workContactTagDestroy.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workContactTag/move" && r.Method == http.MethodPut && s.workContactTagMove != nil:
		s.workContactTagMove.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workContactTag/synContactTag" && r.Method == http.MethodPut && s.workContactTagSync != nil:
		s.workContactTagSync.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workContact/synContact" && r.Method == http.MethodPut && s.workContactSync != nil:
		s.workContactSync.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workContact/index" && r.Method == http.MethodGet && s.workContactIndex != nil:
		s.workContactIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workContact/lossContact" && r.Method == http.MethodGet && s.workContactLoss != nil:
		s.workContactLoss.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workContact/source" && r.Method == http.MethodGet && s.workContactSource != nil:
		s.workContactSource.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workContact/show" && r.Method == http.MethodGet && s.workContactShow != nil:
		s.workContactShow.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workContact/track" && r.Method == http.MethodGet && s.workContactTrack != nil:
		s.workContactTrack.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workContact/update" && r.Method == http.MethodPut && s.workContactUpdate != nil:
		s.workContactUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workContact/batchLabeling" && r.Method == http.MethodPost && s.workContactBatchLabeling != nil:
		s.workContactBatchLabeling.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workContactRoom/index" && r.Method == http.MethodGet && s.workContactRoomIndex != nil:
		s.workContactRoomIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workRoom/index" && r.Method == http.MethodGet && s.workRoomIndex != nil:
		s.workRoomIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workRoom/roomIndex" && r.Method == http.MethodGet && s.workRoomRoomIndex != nil:
		s.workRoomRoomIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workRoom/statistics" && r.Method == http.MethodGet && s.workRoomStatistics != nil:
		s.workRoomStatistics.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workRoom/statisticsIndex" && r.Method == http.MethodGet && s.workRoomStatisticsIndex != nil:
		s.workRoomStatisticsIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workRoom/syn" && r.Method == http.MethodPut && s.workRoomSync != nil:
		s.workRoomSync.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workRoom/batchUpdate" && r.Method == http.MethodPut && s.workRoomBatchUpdate != nil:
		s.workRoomBatchUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/sidebar/workRoom/roomManage" && r.Method == http.MethodGet && s.sidebarWorkRoomManage != nil:
		s.sidebarWorkRoomManage.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactTransfer/info" && r.Method == http.MethodGet && s.contactTransferInfo != nil:
		s.contactTransferInfo.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactTransfer/unassignedList" && r.Method == http.MethodGet && s.contactTransferUnassigned != nil:
		s.contactTransferUnassigned.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactTransfer/room" && r.Method == http.MethodGet && s.contactTransferRoom != nil:
		s.contactTransferRoom.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactTransfer/log" && r.Method == http.MethodGet && s.contactTransferLog != nil:
		s.contactTransferLog.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactTransfer/saveUnassignedList" && r.Method == http.MethodGet && s.contactTransferSync != nil:
		s.contactTransferSync.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactTransfer/index" && r.Method == http.MethodPost && s.contactTransferCustomer != nil:
		s.contactTransferCustomer.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactTransfer/room" && r.Method == http.MethodPost && s.contactTransferRoomStore != nil:
		s.contactTransferRoomStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workRoomAutoPull/index" && r.Method == http.MethodGet && s.workRoomAutoPullIndex != nil:
		s.workRoomAutoPullIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workRoomAutoPull/show" && r.Method == http.MethodGet && s.workRoomAutoPullShow != nil:
		s.workRoomAutoPullShow.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workRoomAutoPull/store" && r.Method == http.MethodPost && s.workRoomAutoPullStore != nil:
		s.workRoomAutoPullStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workRoomAutoPull/update" && r.Method == http.MethodPut && s.workRoomAutoPullUpdate != nil:
		s.workRoomAutoPullUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workRoomAutoPull/move" && r.Method == http.MethodPut && s.workRoomAutoPullMove != nil:
		s.workRoomAutoPullMove.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomTagPull/index" && r.Method == http.MethodGet && s.roomTagPullIndex != nil:
		s.roomTagPullIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomTagPull/show" && r.Method == http.MethodGet && s.roomTagPullShow != nil:
		s.roomTagPullShow.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomTagPull/showContact" && r.Method == http.MethodGet && s.roomTagPullShowContact != nil:
		s.roomTagPullShowContact.ServeHTTP(w, r)
	case (r.URL.Path == "/dashboard/roomTagPull/contactDetail" || r.URL.Path == "/roomTagPull/contactDetail" || r.URL.Path == "/roomTagPull/clientDetails") && (r.Method == http.MethodGet || r.Method == http.MethodHead) && s.roomTagPullContactDetailPage != nil:
		s.roomTagPullContactDetailPage.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomTagPull/roomList" && r.Method == http.MethodGet && s.roomTagPullRoomList != nil:
		s.roomTagPullRoomList.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomTagPull/chooseContact" && r.Method == http.MethodGet && s.roomTagPullChooseContact != nil:
		s.roomTagPullChooseContact.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomTagPull/store" && r.Method == http.MethodPost && s.roomTagPullStore != nil:
		s.roomTagPullStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomTagPull/filterContact" && r.Method == http.MethodPost && s.roomTagPullFilterContact != nil:
		s.roomTagPullFilterContact.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomTagPull/remindSend" && r.Method == http.MethodGet && s.roomTagPullRemindSend != nil:
		s.roomTagPullRemindSend.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomTagPull/destroy" && r.Method == http.MethodDelete && s.roomTagPullDestroy != nil:
		s.roomTagPullDestroy.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactMessageBatchSend/index" && r.Method == http.MethodGet && s.contactMessageBatchSendIndex != nil:
		s.contactMessageBatchSendIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactMessageBatchSend/show" && r.Method == http.MethodGet && s.contactMessageBatchSendShow != nil:
		s.contactMessageBatchSendShow.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactMessageBatchSend/messageShow" && r.Method == http.MethodGet && s.contactMessageBatchSendMessageShow != nil:
		s.contactMessageBatchSendMessageShow.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactMessageBatchSend/showRoom" && r.Method == http.MethodGet && s.contactMessageBatchSendShowRoom != nil:
		s.contactMessageBatchSendShowRoom.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactMessageBatchSend/employeeSendIndex" && r.Method == http.MethodGet && s.contactMessageBatchSendEmployee != nil:
		s.contactMessageBatchSendEmployee.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactMessageBatchSend/contactReceiveIndex" && r.Method == http.MethodGet && s.contactMessageBatchSendContactReceive != nil:
		s.contactMessageBatchSendContactReceive.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactMessageBatchSend/store" && r.Method == http.MethodPost && s.contactMessageBatchSendStore != nil:
		s.contactMessageBatchSendStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactMessageBatchSend/remind" && r.Method == http.MethodPost && s.contactMessageBatchSendRemind != nil:
		s.contactMessageBatchSendRemind.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactMessageBatchSend/destroy" && r.Method == http.MethodDelete && s.contactMessageBatchSendDestroy != nil:
		s.contactMessageBatchSendDestroy.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomMessageBatchSend/index" && r.Method == http.MethodGet && s.roomMessageBatchSendIndex != nil:
		s.roomMessageBatchSendIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomMessageBatchSend/show" && r.Method == http.MethodGet && s.roomMessageBatchSendShow != nil:
		s.roomMessageBatchSendShow.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomMessageBatchSend/roomOwnerSendIndex" && r.Method == http.MethodGet && s.roomMessageBatchSendOwner != nil:
		s.roomMessageBatchSendOwner.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomMessageBatchSend/roomReceiveIndex" && r.Method == http.MethodGet && s.roomMessageBatchSendRoomReceive != nil:
		s.roomMessageBatchSendRoomReceive.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomMessageBatchSend/store" && r.Method == http.MethodPost && s.roomMessageBatchSendStore != nil:
		s.roomMessageBatchSendStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomMessageBatchSend/remind" && r.Method == http.MethodGet && s.roomMessageBatchSendRemind != nil:
		s.roomMessageBatchSendRemind.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomMessageBatchSend/destroy" && r.Method == http.MethodDelete && s.roomMessageBatchSendDestroy != nil:
		s.roomMessageBatchSendDestroy.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/officialAccount/index" && r.Method == http.MethodGet && s.officialAccountIndex != nil:
		s.officialAccountIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/officialAccount/set" && r.Method == http.MethodGet && s.officialAccountSet != nil:
		s.officialAccountSet.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/officialAccount/getPreAuthUrl" && r.Method == http.MethodGet && s.officialAccountGetPreAuthURL != nil:
		s.officialAccountGetPreAuthURL.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/officialAccount/authRedirect/" && (r.Method == http.MethodGet || r.Method == http.MethodPost) && s.officialAccountAuthRedirect != nil:
		s.officialAccountAuthRedirect.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/officialAccount/authEventCallback" && (r.Method == http.MethodGet || r.Method == http.MethodPost) && s.officialAccountAuthEventCallback != nil:
		s.officialAccountAuthEventCallback.ServeHTTP(w, r)
	case officialAccountMessageEventCallbackPath(r.URL.Path) && (r.Method == http.MethodGet || r.Method == http.MethodPost) && s.officialAccountMessageEventCallback != nil:
		s.officialAccountMessageEventCallback.ServeHTTP(w, r)
	case loadPath(r.URL.Path) && (r.Method == http.MethodGet || r.Method == http.MethodPost) && s.load != nil:
		s.load.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workFission/index" && r.Method == http.MethodGet && s.workFissionIndex != nil:
		s.workFissionIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workFission/show" && r.Method == http.MethodGet && s.workFissionShow != nil:
		s.workFissionShow.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workFission/info" && r.Method == http.MethodGet && s.workFissionInfo != nil:
		s.workFissionInfo.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workFission/statistics" && r.Method == http.MethodGet && s.workFissionStatistics != nil:
		s.workFissionStatistics.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workFission/chooseContact" && r.Method == http.MethodGet && s.workFissionChooseContact != nil:
		s.workFissionChooseContact.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workFission/store" && r.Method == http.MethodPost && s.workFissionStore != nil:
		s.workFissionStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workFission/update" && r.Method == http.MethodPut && s.workFissionUpdate != nil:
		s.workFissionUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workFission/invite" && r.Method == http.MethodPost && s.workFissionInvite != nil:
		s.workFissionInvite.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workFission/inviteData" && r.Method == http.MethodGet && s.workFissionInviteData != nil:
		s.workFissionInviteData.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workFission/inviteDetail" && r.Method == http.MethodGet && s.workFissionInviteDetail != nil:
		s.workFissionInviteDetail.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workFission/destroy" && r.Method == http.MethodDelete && s.workFissionDestroy != nil:
		s.workFissionDestroy.ServeHTTP(w, r)
	case r.URL.Path == "/operation/workFission/inviteFriends" && r.Method == http.MethodGet && s.operationWorkFissionInviteFriends != nil:
		s.operationWorkFissionInviteFriends.ServeHTTP(w, r)
	case r.URL.Path == "/operation/workFission/poster" && r.Method == http.MethodGet && s.operationWorkFissionPoster != nil:
		s.operationWorkFissionPoster.ServeHTTP(w, r)
	case r.URL.Path == "/operation/workFission/taskData" && r.Method == http.MethodGet && s.operationWorkFissionTaskData != nil:
		s.operationWorkFissionTaskData.ServeHTTP(w, r)
	case r.URL.Path == "/operation/workFission/receive" && (r.Method == http.MethodPut || r.Method == http.MethodGet) && s.operationWorkFissionReceive != nil:
		s.operationWorkFissionReceive.ServeHTTP(w, r)
	case (r.URL.Path == "/operation/auth/workFission" || r.URL.Path == "/auth/workFission") && (r.Method == http.MethodGet || r.Method == http.MethodPost) && s.operationWorkFissionAuth != nil:
		s.operationWorkFissionAuth.ServeHTTP(w, r)
	case (r.URL.Path == "/operation/openUserInfo/workFission" || r.URL.Path == "/openUserInfo/workFission") && r.Method == http.MethodGet && s.operationWorkFissionOpenUserInfo != nil:
		s.operationWorkFissionOpenUserInfo.ServeHTTP(w, r)
	case r.URL.Path == "/operation/lottery/contactData" && r.Method == http.MethodPost && s.operationLotteryContactData != nil:
		s.operationLotteryContactData.ServeHTTP(w, r)
	case r.URL.Path == "/operation/lottery/contactLottery" && r.Method == http.MethodPut && s.operationLotteryContactLottery != nil:
		s.operationLotteryContactLottery.ServeHTTP(w, r)
	case r.URL.Path == "/operation/lottery/receive" && r.Method == http.MethodPut && s.operationLotteryReceive != nil:
		s.operationLotteryReceive.ServeHTTP(w, r)
	case (r.URL.Path == "/operation/auth/lottery" || r.URL.Path == "/auth/lottery") && (r.Method == http.MethodGet || r.Method == http.MethodPost) && s.operationLotteryAuth != nil:
		s.operationLotteryAuth.ServeHTTP(w, r)
	case (r.URL.Path == "/operation/openUserInfo/lottery" || r.URL.Path == "/openUserInfo/lottery") && r.Method == http.MethodGet && s.operationLotteryOpenUserInfo != nil:
		s.operationLotteryOpenUserInfo.ServeHTTP(w, r)
	case r.URL.Path == "/operation/roomClockIn/contactData" && r.Method == http.MethodGet && s.operationRoomClockInContactData != nil:
		s.operationRoomClockInContactData.ServeHTTP(w, r)
	case r.URL.Path == "/operation/roomClockIn/clockInRanking" && r.Method == http.MethodGet && s.operationRoomClockInClockInRanking != nil:
		s.operationRoomClockInClockInRanking.ServeHTTP(w, r)
	case r.URL.Path == "/operation/roomClockIn/contactClockIn" && r.Method == http.MethodPut && s.operationRoomClockInContactClockIn != nil:
		s.operationRoomClockInContactClockIn.ServeHTTP(w, r)
	case r.URL.Path == "/operation/roomClockIn/receive" && r.Method == http.MethodPut && s.operationRoomClockInReceive != nil:
		s.operationRoomClockInReceive.ServeHTTP(w, r)
	case (r.URL.Path == "/operation/auth/roomClockIn" || r.URL.Path == "/auth/roomClockIn") && (r.Method == http.MethodGet || r.Method == http.MethodPost) && s.operationRoomClockInAuth != nil:
		s.operationRoomClockInAuth.ServeHTTP(w, r)
	case (r.URL.Path == "/operation/openUserInfo/roomClockIn" || r.URL.Path == "/openUserInfo/roomClockIn") && r.Method == http.MethodGet && s.operationRoomClockInOpenUserInfo != nil:
		s.operationRoomClockInOpenUserInfo.ServeHTTP(w, r)
	case r.URL.Path == "/operation/roomFission/poster" && r.Method == http.MethodGet && s.operationRoomFissionPoster != nil:
		s.operationRoomFissionPoster.ServeHTTP(w, r)
	case r.URL.Path == "/operation/roomFission/inviteFriends" && r.Method == http.MethodGet && s.operationRoomFissionInviteFriends != nil:
		s.operationRoomFissionInviteFriends.ServeHTTP(w, r)
	case r.URL.Path == "/operation/roomFission/receive" && r.Method == http.MethodGet && s.operationRoomFissionReceive != nil:
		s.operationRoomFissionReceive.ServeHTTP(w, r)
	case (r.URL.Path == "/operation/auth/roomFission" || r.URL.Path == "/auth/roomFission") && (r.Method == http.MethodGet || r.Method == http.MethodPost) && s.operationRoomFissionAuth != nil:
		s.operationRoomFissionAuth.ServeHTTP(w, r)
	case (r.URL.Path == "/operation/openUserInfo/roomFission" || r.URL.Path == "/openUserInfo/roomFission") && r.Method == http.MethodGet && s.operationRoomFissionOpenUserInfo != nil:
		s.operationRoomFissionOpenUserInfo.ServeHTTP(w, r)
	case r.URL.Path == "/operation/roomInfinitePull/qrCode" && r.Method == http.MethodGet && s.operationRoomInfinitePullQRCode != nil:
		s.operationRoomInfinitePullQRCode.ServeHTTP(w, r)
	case r.URL.Path == "/operation/shopCode/areaCode" && r.Method == http.MethodGet && s.operationShopCodeAreaCode != nil:
		s.operationShopCodeAreaCode.ServeHTTP(w, r)
	case r.URL.Path == "/operation/shopCode/weChatSdkConfig" && r.Method == http.MethodGet && s.operationShopCodeWeChatSDKConfig != nil:
		s.operationShopCodeWeChatSDKConfig.ServeHTTP(w, r)
	case (r.URL.Path == "/operation/auth/shopCode" || r.URL.Path == "/auth/shopCode") && (r.Method == http.MethodGet || r.Method == http.MethodPost) && s.operationShopCodeAuth != nil:
		s.operationShopCodeAuth.ServeHTTP(w, r)
	case (r.URL.Path == "/operation/openUserInfo/shopCode" || r.URL.Path == "/openUserInfo/shopCode") && r.Method == http.MethodGet && s.operationShopCodeOpenUserInfo != nil:
		s.operationShopCodeOpenUserInfo.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workRoomGroup/index" && r.Method == http.MethodGet && s.workRoomGroupIndex != nil:
		s.workRoomGroupIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workRoomGroup/store" && r.Method == http.MethodPost && s.workRoomGroupStore != nil:
		s.workRoomGroupStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workRoomGroup/update" && r.Method == http.MethodPut && s.workRoomGroupUpdate != nil:
		s.workRoomGroupUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workRoomGroup/destroy" && r.Method == http.MethodDelete && s.workRoomGroupDestroy != nil:
		s.workRoomGroupDestroy.ServeHTTP(w, r)
	case r.URL.Path == "/sidebar/workContactTag/allTag" && r.Method == http.MethodGet && s.sidebarContactTagAll != nil:
		s.sidebarContactTagAll.ServeHTTP(w, r)
	case r.URL.Path == "/sidebar/workContact/detail" && r.Method == http.MethodGet && s.sidebarContactDetail != nil:
		s.sidebarContactDetail.ServeHTTP(w, r)
	case r.URL.Path == "/sidebar/workContact/show" && r.Method == http.MethodGet && s.sidebarContactShow != nil:
		s.sidebarContactShow.ServeHTTP(w, r)
	case r.URL.Path == "/sidebar/workContact/track" && r.Method == http.MethodGet && s.sidebarContactTrack != nil:
		s.sidebarContactTrack.ServeHTTP(w, r)
	case r.URL.Path == "/sidebar/workContact/update" && r.Method == http.MethodPut && s.sidebarContactUpdate != nil:
		s.sidebarContactUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/sidebar/contactProcessStatus/index" && r.Method == http.MethodGet && s.sidebarProcessStatus != nil:
		s.sidebarProcessStatus.ServeHTTP(w, r)
	case r.URL.Path == "/sidebar/contactProcessStatus/update" && r.Method == http.MethodPut && s.sidebarProcessUpdate != nil:
		s.sidebarProcessUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/medium/index" && r.Method == http.MethodGet && s.mediumIndex != nil:
		s.mediumIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/medium/show" && r.Method == http.MethodGet && s.mediumShow != nil:
		s.mediumShow.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/medium/store" && r.Method == http.MethodPost && s.mediumStore != nil:
		s.mediumStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/medium/update" && r.Method == http.MethodPut && s.mediumUpdate != nil:
		s.mediumUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/medium/destroy" && r.Method == http.MethodDelete && s.mediumDestroy != nil:
		s.mediumDestroy.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/medium/groupUpdate" && r.Method == http.MethodPut && s.mediumItemGroupUpdate != nil:
		s.mediumItemGroupUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/medium/batchGroupUpdate" && r.Method == http.MethodPost && s.mediumBatchGroupUpdate != nil:
		s.mediumBatchGroupUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/medium/referenceCheck" && r.Method == http.MethodPost && s.mediumReferenceCheck != nil:
		s.mediumReferenceCheck.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/medium/batchDestroy" && r.Method == http.MethodPost && s.mediumBatchDestroy != nil:
		s.mediumBatchDestroy.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/materialSelector/index" && r.Method == http.MethodGet && s.materialSelectorIndex != nil:
		s.materialSelectorIndex.ServeHTTP(w, r)
	case r.URL.Path == "/sidebar/medium/index" && r.Method == http.MethodGet && s.sidebarMediumIndex != nil:
		s.sidebarMediumIndex.ServeHTTP(w, r)
	case r.URL.Path == "/sidebar/medium/mediaIdUpdate" && r.Method == http.MethodGet && s.sidebarMediumMediaIDUpdate != nil:
		s.sidebarMediumMediaIDUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/mediumGroup/index" && r.Method == http.MethodGet && s.mediumGroupIndex != nil:
		s.mediumGroupIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/friendsCircle/taskIndex" && r.Method == http.MethodGet && s.friendsCircleTaskIndex != nil:
		s.friendsCircleTaskIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/friendsCircle/materialIndex" && r.Method == http.MethodGet && s.friendsCircleMaterialIndex != nil:
		s.friendsCircleMaterialIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/friendsCircle/taskStore" && r.Method == http.MethodPost && s.friendsCircleTaskStore != nil:
		s.friendsCircleTaskStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/friendsCircle/materialStore" && r.Method == http.MethodPost && s.friendsCircleMaterialStore != nil:
		s.friendsCircleMaterialStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/friendsCircle/publish" && r.Method == http.MethodPost && s.friendsCirclePublish != nil:
		s.friendsCirclePublish.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/friendsCircle/taskResultIndex" && r.Method == http.MethodGet && s.friendsCircleTaskResultIndex != nil:
		s.friendsCircleTaskResultIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/friendsCircle/export" && r.Method == http.MethodGet && s.friendsCircleExport != nil:
		s.friendsCircleExport.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/friendsCircle/exportData" && r.Method == http.MethodGet && s.friendsCircleExportData != nil:
		s.friendsCircleExportData.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/friendsCircle/providerCallback" && r.Method == http.MethodPost && s.friendsCircleProviderCallback != nil:
		s.friendsCircleProviderCallback.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/acquisitionLink/index" && r.Method == http.MethodGet && s.phase34AcquisitionLinkIndex != nil:
		s.phase34AcquisitionLinkIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/acquisitionLink/store" && r.Method == http.MethodPost && s.phase34AcquisitionLinkStore != nil:
		s.phase34AcquisitionLinkStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/acquisitionLink/authorize" && r.Method == http.MethodPost && s.phase34AcquisitionLinkAuthorize != nil:
		s.phase34AcquisitionLinkAuthorize.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/customerService/index" && r.Method == http.MethodGet && s.phase34CustomerServiceIndex != nil:
		s.phase34CustomerServiceIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/customerService/store" && r.Method == http.MethodPost && s.phase34CustomerServiceStore != nil:
		s.phase34CustomerServiceStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/customerService/sync" && r.Method == http.MethodPost && s.phase34CustomerServiceSync != nil:
		s.phase34CustomerServiceSync.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/liveCodeShortChain/index" && r.Method == http.MethodGet && s.phase34ShortLinkIndex != nil:
		s.phase34ShortLinkIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/liveCodeShortChain/store" && r.Method == http.MethodPost && s.phase34ShortLinkStore != nil:
		s.phase34ShortLinkStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/liveCodeShortChain/disable" && r.Method == http.MethodPost && s.phase34ShortLinkDisable != nil:
		s.phase34ShortLinkDisable.ServeHTTP(w, r)
	case strings.HasPrefix(r.URL.Path, "/r/") && r.Method == http.MethodGet && s.phase34ShortLinkRedirect != nil:
		s.phase34ShortLinkRedirect.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/mediumGroup/store" && r.Method == http.MethodPost && s.mediumGroupStore != nil:
		s.mediumGroupStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/mediumGroup/update" && r.Method == http.MethodPut && s.mediumGroupUpdate != nil:
		s.mediumGroupUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/mediumGroup/destroy" && r.Method == http.MethodDelete && s.mediumGroupDestroy != nil:
		s.mediumGroupDestroy.ServeHTTP(w, r)
	case r.URL.Path == "/sidebar/mediumGroup/index" && r.Method == http.MethodGet && s.sidebarMediumGroupIndex != nil:
		s.sidebarMediumGroupIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/channelCode/index" && r.Method == http.MethodGet && s.channelCodeIndex != nil:
		s.channelCodeIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/channelCode/show" && r.Method == http.MethodGet && s.channelCodeShow != nil:
		s.channelCodeShow.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/channelCode/contact" && r.Method == http.MethodGet && s.channelCodeContact != nil:
		s.channelCodeContact.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/channelCode/statistics" && r.Method == http.MethodGet && s.channelCodeStatistics != nil:
		s.channelCodeStatistics.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/channelCode/statisticsIndex" && r.Method == http.MethodGet && s.channelCodeStatsIndex != nil:
		s.channelCodeStatsIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/channelCode/store" && r.Method == http.MethodPost && s.channelCodeStore != nil:
		s.channelCodeStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/channelCode/update" && r.Method == http.MethodPut && s.channelCodeUpdate != nil:
		s.channelCodeUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/channelCodeGroup/index" && r.Method == http.MethodGet && s.channelCodeGroupIndex != nil:
		s.channelCodeGroupIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/channelCodeGroup/detail" && r.Method == http.MethodGet && s.channelCodeGroupDetail != nil:
		s.channelCodeGroupDetail.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/channelCodeGroup/store" && r.Method == http.MethodPost && s.channelCodeGroupStore != nil:
		s.channelCodeGroupStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/channelCodeGroup/update" && r.Method == http.MethodPut && s.channelCodeGroupUpdate != nil:
		s.channelCodeGroupUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/channelCodeGroup/move" && r.Method == http.MethodPut && s.channelCodeGroupMove != nil:
		s.channelCodeGroupMove.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/greeting/index" && r.Method == http.MethodGet && s.greetingIndex != nil:
		s.greetingIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/greeting/show" && r.Method == http.MethodGet && s.greetingShow != nil:
		s.greetingShow.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/greeting/store" && r.Method == http.MethodPost && s.greetingStore != nil:
		s.greetingStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/greeting/update" && r.Method == http.MethodPut && s.greetingUpdate != nil:
		s.greetingUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/greeting/destroy" && r.Method == http.MethodDelete && s.greetingDestroy != nil:
		s.greetingDestroy.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomWelcome/index" && r.Method == http.MethodGet && s.roomWelcomeIndex != nil:
		s.roomWelcomeIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/clockIn/index" && r.Method == http.MethodGet && s.roomWelcomeIndex != nil:
		serveHTTPWithPath(s.roomWelcomeIndex, w, r, "/dashboard/roomWelcome/index")
	case r.URL.Path == "/dashboard/roomWelcome/select" && r.Method == http.MethodGet && s.roomWelcomeSelect != nil:
		s.roomWelcomeSelect.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomWelcome/show" && r.Method == http.MethodGet && s.roomWelcomeShow != nil:
		s.roomWelcomeShow.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomWelcome/store" && r.Method == http.MethodPost && s.roomWelcomeStore != nil:
		s.roomWelcomeStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomWelcome/update" && r.Method == http.MethodPut && s.roomWelcomeUpdate != nil:
		s.roomWelcomeUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomWelcome/destroy" && r.Method == http.MethodDelete && s.roomWelcomeDestroy != nil:
		s.roomWelcomeDestroy.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactField/index" && r.Method == http.MethodGet && s.contactFieldIndex != nil:
		s.contactFieldIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactField/show" && r.Method == http.MethodGet && s.contactFieldShow != nil:
		s.contactFieldShow.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactField/portrait" && r.Method == http.MethodGet && s.contactFieldPortrait != nil:
		s.contactFieldPortrait.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactField/store" && r.Method == http.MethodPost && s.contactFieldStore != nil:
		s.contactFieldStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactField/update" && r.Method == http.MethodPut && s.contactFieldUpdate != nil:
		s.contactFieldUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactField/statusUpdate" && r.Method == http.MethodPut && s.contactFieldStatus != nil:
		s.contactFieldStatus.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactField/destroy" && r.Method == http.MethodDelete && s.contactFieldDestroy != nil:
		s.contactFieldDestroy.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactField/batchUpdate" && r.Method == http.MethodPut && s.contactFieldBatch != nil:
		s.contactFieldBatch.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactFieldPivot/index" && r.Method == http.MethodGet && s.contactFieldPivot != nil:
		s.contactFieldPivot.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactFieldPivot/update" && r.Method == http.MethodPut && s.contactFieldPivotUpdate != nil:
		s.contactFieldPivotUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactBatchAdd/index" && r.Method == http.MethodGet && s.contactBatchAddIndex != nil:
		s.contactBatchAddIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactBatchAdd/importIndex" && r.Method == http.MethodGet && s.contactBatchAddImportIndex != nil:
		s.contactBatchAddImportIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactBatchAdd/importStore" && r.Method == http.MethodPost && s.contactBatchAddImportStore != nil:
		s.contactBatchAddImportStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactBatchAdd/allot" && r.Method == http.MethodPost && s.contactBatchAddAllot != nil:
		s.contactBatchAddAllot.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactBatchAdd/dataStatistic" && r.Method == http.MethodGet && s.contactBatchAddDataStatistic != nil:
		s.contactBatchAddDataStatistic.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactBatchAdd/destroy" && r.Method == http.MethodDelete && s.contactBatchAddDestroy != nil:
		s.contactBatchAddDestroy.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactBatchAdd/importDestroy" && r.Method == http.MethodDelete && s.contactBatchAddImportDestroy != nil:
		s.contactBatchAddImportDestroy.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactBatchAdd/settingEdit" && r.Method == http.MethodGet && s.contactBatchAddSettingEdit != nil:
		s.contactBatchAddSettingEdit.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactBatchAdd/settingUpdate" && r.Method == http.MethodPost && s.contactBatchAddSettingUpdate != nil:
		s.contactBatchAddSettingUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactBatchAdd/remind" && (r.Method == http.MethodGet || r.Method == http.MethodPost) && s.contactBatchAddRemind != nil:
		s.contactBatchAddRemind.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/sensitiveWords/page" && (r.Method == http.MethodGet || r.Method == http.MethodHead) && s.sensitiveWordsPage != nil:
		s.sensitiveWordsPage.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/sensitiveWord/index" && r.Method == http.MethodGet && s.sensitiveWordIndex != nil:
		s.sensitiveWordIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/sensitiveWord/store" && r.Method == http.MethodPost && s.sensitiveWordStore != nil:
		s.sensitiveWordStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/sensitiveWord/destroy" && r.Method == http.MethodDelete && s.sensitiveWordDestroy != nil:
		s.sensitiveWordDestroy.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/sensitiveWord/statusUpdate" && r.Method == http.MethodPut && s.sensitiveWordStatusUpdate != nil:
		s.sensitiveWordStatusUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/sensitiveWord/move" && r.Method == http.MethodPut && s.sensitiveWordMove != nil:
		s.sensitiveWordMove.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/sensitiveWordGroup/select" && r.Method == http.MethodGet && s.sensitiveWordGroupSelect != nil:
		s.sensitiveWordGroupSelect.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/sensitiveWordGroup/store" && r.Method == http.MethodPost && s.sensitiveWordGroupStore != nil:
		s.sensitiveWordGroupStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/sensitiveWordGroup/update" && r.Method == http.MethodPut && s.sensitiveWordGroupUpdate != nil:
		s.sensitiveWordGroupUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/sensitiveWordsMonitor/index" && r.Method == http.MethodGet && s.sensitiveWordsMonitorIndex != nil:
		s.sensitiveWordsMonitorIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/sensitiveWordsMonitor/show" && r.Method == http.MethodGet && s.sensitiveWordsMonitorShow != nil:
		s.sensitiveWordsMonitorShow.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactSop/index" && r.Method == http.MethodGet && s.contactSOPIndex != nil:
		s.contactSOPIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactSop/store" && r.Method == http.MethodPost && s.contactSOPStore != nil:
		s.contactSOPStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactSop/setEmployee" && r.Method == http.MethodPut && s.contactSOPSetEmployee != nil:
		s.contactSOPSetEmployee.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactSop/state" && r.Method == http.MethodPut && s.contactSOPState != nil:
		s.contactSOPState.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactSop/info" && r.Method == http.MethodGet && s.contactSOPInfo != nil:
		s.contactSOPInfo.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactSop/destroy" && r.Method == http.MethodDelete && s.contactSOPDestroy != nil:
		s.contactSOPDestroy.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactSop/update" && r.Method == http.MethodPut && s.contactSOPUpdate != nil:
		s.contactSOPUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/contactSop/page" && (r.Method == http.MethodGet || r.Method == http.MethodHead) && s.contactSOPPage != nil:
		s.contactSOPPage.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomSop/page" && (r.Method == http.MethodGet || r.Method == http.MethodHead) && s.roomSOPPage != nil:
		s.roomSOPPage.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomSop/index" && r.Method == http.MethodGet && s.roomSOPIndex != nil:
		s.roomSOPIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomSop/store" && r.Method == http.MethodPost && s.roomSOPStore != nil:
		s.roomSOPStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomSop/setRoom" && r.Method == http.MethodPut && s.roomSOPSetRoom != nil:
		s.roomSOPSetRoom.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomSop/state" && r.Method == http.MethodPut && s.roomSOPState != nil:
		s.roomSOPState.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomSop/info" && r.Method == http.MethodGet && s.roomSOPInfo != nil:
		s.roomSOPInfo.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomSop/destroy" && r.Method == http.MethodDelete && s.roomSOPDestroy != nil:
		s.roomSOPDestroy.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomSop/update" && r.Method == http.MethodPut && s.roomSOPUpdate != nil:
		s.roomSOPUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/shopCode/page" && (r.Method == http.MethodGet || r.Method == http.MethodHead) && s.shopCodePage != nil:
		s.shopCodePage.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/shopCode/location" && r.Method == http.MethodGet && s.shopCodeLocation != nil:
		s.shopCodeLocation.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/shopCode/addressKeyWordList" && r.Method == http.MethodGet && s.shopCodeAddressKeyWordList != nil:
		s.shopCodeAddressKeyWordList.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/shopCode/store" && r.Method == http.MethodPost && s.shopCodeStore != nil:
		s.shopCodeStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/shopCode/update" && r.Method == http.MethodPut && s.shopCodeUpdate != nil:
		s.shopCodeUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/shopCode/destroy" && r.Method == http.MethodDelete && s.shopCodeDestroy != nil:
		s.shopCodeDestroy.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/shopCode/info" && r.Method == http.MethodGet && s.shopCodeInfo != nil:
		s.shopCodeInfo.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/shopCode/status" && r.Method == http.MethodPut && s.shopCodeStatus != nil:
		s.shopCodeStatus.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/shopCode/index" && r.Method == http.MethodGet && s.shopCodeIndex != nil:
		s.shopCodeIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/shopCode/searchCity" && r.Method == http.MethodGet && s.shopCodeSearchCity != nil:
		s.shopCodeSearchCity.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/shopCode/share" && r.Method == http.MethodGet && s.shopCodeShare != nil:
		s.shopCodeShare.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/shopCode/pageInfo" && r.Method == http.MethodGet && s.shopCodePageInfo != nil:
		s.shopCodePageInfo.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/shopCode/pageSet" && r.Method == http.MethodPost && s.shopCodePageSet != nil:
		s.shopCodePageSet.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/shopCode/show" && r.Method == http.MethodGet && s.shopCodeShow != nil:
		s.shopCodeShow.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/shopCode/showContact" && r.Method == http.MethodGet && s.shopCodeShowContact != nil:
		s.shopCodeShowContact.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/shopCode/showShop" && r.Method == http.MethodGet && s.shopCodeShowShop != nil:
		s.shopCodeShowShop.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/shopCode/updateEmployee" && r.Method == http.MethodPost && s.shopCodeUpdateEmployee != nil:
		s.shopCodeUpdateEmployee.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/shopCode/updateQrcode" && r.Method == http.MethodPost && s.shopCodeUpdateQRCode != nil:
		s.shopCodeUpdateQRCode.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/shopCode/batchContactTags" && r.Method == http.MethodPut && s.shopCodeBatchContactTags != nil:
		s.shopCodeBatchContactTags.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/radar/page" && (r.Method == http.MethodGet || r.Method == http.MethodHead) && s.radarPage != nil:
		s.radarPage.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/radar/store" && r.Method == http.MethodPost && s.radarStore != nil:
		s.radarStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/radar/update" && r.Method == http.MethodPut && s.radarUpdate != nil:
		s.radarUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/radar/index" && r.Method == http.MethodGet && s.radarIndex != nil:
		s.radarIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/radar/destroy" && r.Method == http.MethodDelete && s.radarDestroy != nil:
		s.radarDestroy.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/radar/info" && r.Method == http.MethodGet && s.radarInfo != nil:
		s.radarInfo.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/radar/storeChannel" && r.Method == http.MethodPost && s.radarStoreChannel != nil:
		s.radarStoreChannel.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/radar/storeChannelLink" && r.Method == http.MethodPost && s.radarStoreChannelLink != nil:
		s.radarStoreChannelLink.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/radar/indexChannel" && r.Method == http.MethodGet && s.radarIndexChannel != nil:
		s.radarIndexChannel.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/radar/indexChannelLink" && r.Method == http.MethodGet && s.radarIndexChannelLink != nil:
		s.radarIndexChannelLink.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/radar/show" && r.Method == http.MethodGet && s.radarShow != nil:
		s.radarShow.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/radar/showContact" && r.Method == http.MethodGet && s.radarShowContact != nil:
		s.radarShowContact.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/radar/showChannel" && r.Method == http.MethodGet && s.radarShowChannel != nil:
		s.radarShowChannel.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/radar/radarArticle" && r.Method == http.MethodGet && s.radarArticle != nil:
		s.radarArticle.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/autoTag/store" && r.Method == http.MethodPost && s.autoTagStore != nil:
		s.autoTagStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/autoTag/index" && r.Method == http.MethodGet && s.autoTagIndex != nil:
		s.autoTagIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/autoTag/destroy" && r.Method == http.MethodDelete && s.autoTagDestroy != nil:
		s.autoTagDestroy.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/autoTag/onOff" && r.Method == http.MethodPut && s.autoTagOnOff != nil:
		s.autoTagOnOff.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/autoTag/show" && r.Method == http.MethodGet && s.autoTagShow != nil:
		s.autoTagShow.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/autoTag/showContactKeyWord" && r.Method == http.MethodGet && s.autoTagShowContactKeyWord != nil:
		s.autoTagShowContactKeyWord.ServeHTTP(w, r)
	case (r.URL.Path == "/Task/AutoTag/KeyWordTag" || r.URL.Path == "/dashboard/Task/AutoTag/KeyWordTag") && r.Method == http.MethodGet && s.autoTagKeyWordTag != nil:
		s.autoTagKeyWordTag.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/autoTag/showContactRoom" && r.Method == http.MethodGet && s.autoTagShowContactRoom != nil:
		s.autoTagShowContactRoom.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/autoTag/showContactTime" && r.Method == http.MethodGet && s.autoTagShowContactTime != nil:
		s.autoTagShowContactTime.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workMessage/fromUsers" && r.Method == http.MethodGet && s.workMessageFromUsers != nil:
		s.workMessageFromUsers.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workMessage/toUsers" && r.Method == http.MethodGet && s.workMessageToUsers != nil:
		s.workMessageToUsers.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workMessage/globalOverview" && r.Method == http.MethodGet && s.workMessageGlobalOverview != nil:
		s.workMessageGlobalOverview.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workMessage/focus" && (r.Method == http.MethodPut || r.Method == http.MethodDelete) && s.workMessageFocus != nil:
		s.workMessageFocus.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workMessage/staffDirectory" && r.Method == http.MethodGet && s.workMessageStaffDirectory != nil:
		s.workMessageStaffDirectory.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workMessage/staffDetail" && r.Method == http.MethodGet && s.workMessageStaffDetail != nil:
		s.workMessageStaffDetail.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workMessage/trajectoryDay" && r.Method == http.MethodGet && s.workMessageTrajectoryDay != nil:
		s.workMessageTrajectoryDay.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workMessage/customerDirectory" && r.Method == http.MethodGet && s.workMessageCustomerDirectory != nil:
		s.workMessageCustomerDirectory.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workMessage/customerConversations" && r.Method == http.MethodGet && s.workMessageCustomerConversations != nil:
		s.workMessageCustomerConversations.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workMessage/customerDetail" && r.Method == http.MethodGet && s.workMessageCustomerDetail != nil:
		s.workMessageCustomerDetail.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workMessage/roomDirectory" && r.Method == http.MethodGet && s.workMessageRoomDirectory != nil:
		s.workMessageRoomDirectory.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workMessage/roomProfile" && r.Method == http.MethodGet && s.workMessageRoomProfile != nil:
		s.workMessageRoomProfile.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workMessage/roomMessages" && r.Method == http.MethodGet && s.workMessageRoomMessages != nil:
		s.workMessageRoomMessages.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workMessage/roomMembers" && r.Method == http.MethodGet && s.workMessageRoomMembers != nil:
		s.workMessageRoomMembers.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workMessage/roomFilterOptions" && r.Method == http.MethodGet && s.workMessageRoomFilterOptions != nil:
		s.workMessageRoomFilterOptions.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/risk/rules" && r.Method == http.MethodGet && s.riskBehaviorRules != nil:
		s.riskBehaviorRules.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/risk/records" && r.Method == http.MethodGet && s.riskBehaviorRecords != nil:
		s.riskBehaviorRecords.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/risk/rules" && r.Method == http.MethodPost && s.riskBehaviorRuleCreate != nil:
		s.riskBehaviorRuleCreate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/risk/rules" && r.Method == http.MethodPut && s.riskBehaviorRuleUpdate != nil:
		s.riskBehaviorRuleUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/risk/rules/status" && r.Method == http.MethodPut && s.riskBehaviorRuleStatus != nil:
		s.riskBehaviorRuleStatus.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/risk/rules" && r.Method == http.MethodDelete && s.riskBehaviorRuleDelete != nil:
		s.riskBehaviorRuleDelete.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/risk/records/audit" && r.Method == http.MethodPost && s.riskBehaviorRecordsAudit != nil:
		s.riskBehaviorRecordsAudit.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/risk/evaluate" && r.Method == http.MethodPost && s.riskBehaviorEvaluate != nil:
		s.riskBehaviorEvaluate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/timeout-warning/rules" && r.Method == http.MethodGet && s.timeoutWarningRules != nil:
		s.timeoutWarningRules.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/timeout-warning/records" && r.Method == http.MethodGet && s.timeoutWarningRecords != nil:
		s.timeoutWarningRecords.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/timeout-warning/rules" && r.Method == http.MethodPost && s.timeoutWarningRuleCreate != nil:
		s.timeoutWarningRuleCreate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/timeout-warning/rules" && r.Method == http.MethodPut && s.timeoutWarningRuleUpdate != nil:
		s.timeoutWarningRuleUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/timeout-warning/rules/status" && r.Method == http.MethodPut && s.timeoutWarningRuleStatus != nil:
		s.timeoutWarningRuleStatus.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/timeout-warning/rules" && r.Method == http.MethodDelete && s.timeoutWarningRuleDelete != nil:
		s.timeoutWarningRuleDelete.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/timeout-warning/records/audit" && r.Method == http.MethodPost && s.timeoutWarningRecordsAudit != nil:
		s.timeoutWarningRecordsAudit.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/timeout-warning/records/assign" && r.Method == http.MethodPut && s.timeoutWarningRecordsAssign != nil:
		s.timeoutWarningRecordsAssign.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/timeout-warning/settings" && r.Method == http.MethodGet && s.timeoutWarningSettings != nil:
		s.timeoutWarningSettings.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/timeout-warning/settings" && r.Method == http.MethodPut && s.timeoutWarningSettingsUpdate != nil:
		s.timeoutWarningSettingsUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/timeout-warning/evaluate" && r.Method == http.MethodPost && s.timeoutWarningEvaluate != nil:
		s.timeoutWarningEvaluate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/keyword-library/libraries" && r.Method == http.MethodGet && s.keywordLibraries != nil:
		s.keywordLibraries.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/keyword-library/libraries" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.keywordLibrarySave != nil:
		s.keywordLibrarySave.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/keyword-library/libraries/status" && r.Method == http.MethodPut && s.keywordLibraryStatus != nil:
		s.keywordLibraryStatus.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/keyword-library/libraries" && r.Method == http.MethodDelete && s.keywordLibraryDelete != nil:
		s.keywordLibraryDelete.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/keyword-library/libraries/publish" && r.Method == http.MethodPost && s.keywordLibraryPublish != nil:
		s.keywordLibraryPublish.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/keyword-library/entries" && r.Method == http.MethodGet && s.keywordEntries != nil:
		s.keywordEntries.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/keyword-library/entries" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.keywordEntrySave != nil:
		s.keywordEntrySave.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/keyword-library/entries/status" && r.Method == http.MethodPut && s.keywordEntryStatus != nil:
		s.keywordEntryStatus.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/keyword-library/entries" && r.Method == http.MethodDelete && s.keywordEntryDelete != nil:
		s.keywordEntryDelete.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/message-intercept/rules" && r.Method == http.MethodGet && s.messageInterceptRules != nil:
		s.messageInterceptRules.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/message-intercept/rules" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.messageInterceptRuleSave != nil:
		s.messageInterceptRuleSave.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/message-intercept/rules/status" && r.Method == http.MethodPut && s.messageInterceptRuleStatus != nil:
		s.messageInterceptRuleStatus.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/message-intercept/rules" && r.Method == http.MethodDelete && s.messageInterceptRuleDelete != nil:
		s.messageInterceptRuleDelete.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/message-intercept/records" && r.Method == http.MethodGet && s.messageInterceptRecords != nil:
		s.messageInterceptRecords.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/message-intercept/evaluate" && r.Method == http.MethodPost && s.messageInterceptEvaluate != nil:
		s.messageInterceptEvaluate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/message-intercept/records/audit" && r.Method == http.MethodPost && s.messageInterceptAudit != nil:
		s.messageInterceptAudit.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/silent-customer/rules" && r.Method == http.MethodGet && s.silentCustomerRules != nil:
		s.silentCustomerRules.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/silent-customer/rules" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.silentCustomerRuleSave != nil:
		s.silentCustomerRuleSave.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/silent-customer/rules/status" && r.Method == http.MethodPut && s.silentCustomerRuleStatus != nil:
		s.silentCustomerRuleStatus.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/silent-customer/rules" && r.Method == http.MethodDelete && s.silentCustomerRuleDelete != nil:
		s.silentCustomerRuleDelete.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/silent-customer/records" && r.Method == http.MethodGet && s.silentCustomerRecords != nil:
		s.silentCustomerRecords.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/silent-customer/evaluate" && r.Method == http.MethodPost && s.silentCustomerEvaluate != nil:
		s.silentCustomerEvaluate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/silent-customer/records/action" && r.Method == http.MethodPost && s.silentCustomerAction != nil:
		s.silentCustomerAction.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/refuse-archive/records" && r.Method == http.MethodGet && s.refuseArchiveRecords != nil:
		s.refuseArchiveRecords.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/refuse-archive/sync" && r.Method == http.MethodPost && s.refuseArchiveSync != nil:
		s.refuseArchiveSync.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/refuse-archive/follow-up" && r.Method == http.MethodPost && s.refuseArchiveFollowUp != nil:
		s.refuseArchiveFollowUp.ServeHTTP(w, r)
	case (r.URL.Path == "/dashboard/workMessage/index" || r.URL.Path == "/dashboard/workMessage/detail") && r.Method == http.MethodGet && s.workMessageIndex != nil:
		s.workMessageIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workMessageConfig/corpStore" && r.Method == http.MethodPost && s.workMessageConfigCorpStore != nil:
		s.workMessageConfigCorpStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workMessageConfig/corpShow" && r.Method == http.MethodGet && s.workMessageConfigCorpShow != nil:
		s.workMessageConfigCorpShow.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workMessageConfig/corpIndex" && r.Method == http.MethodGet && s.workMessageConfigCorpIndex != nil:
		s.workMessageConfigCorpIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workMessageConfig/stepCreate" && r.Method == http.MethodGet && s.workMessageConfigStepCreate != nil:
		s.workMessageConfigStepCreate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/workMessageConfig/stepUpdate" && r.Method == http.MethodPut && s.workMessageConfigStepUpdate != nil:
		s.workMessageConfigStepUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/lottery/page" && (r.Method == http.MethodGet || r.Method == http.MethodHead) && s.lotteryPage != nil:
		s.lotteryPage.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/lottery/index" && r.Method == http.MethodGet && s.lotteryIndex != nil:
		s.lotteryIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/lottery/store" && r.Method == http.MethodPost && s.lotteryStore != nil:
		s.lotteryStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/lottery/showContact" && r.Method == http.MethodGet && s.lotteryShowContact != nil:
		s.lotteryShowContact.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/lottery/show" && r.Method == http.MethodGet && s.lotteryShow != nil:
		s.lotteryShow.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/lottery/destroy" && r.Method == http.MethodDelete && s.lotteryDestroy != nil:
		s.lotteryDestroy.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/lottery/share" && r.Method == http.MethodGet && s.lotteryShare != nil:
		s.lotteryShare.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/lottery/update" && r.Method == http.MethodPut && s.lotteryUpdate != nil:
		s.lotteryUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/lottery/info" && r.Method == http.MethodGet && s.lotteryInfo != nil:
		s.lotteryInfo.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/lottery/writeOff" && r.Method == http.MethodGet && s.lotteryWriteOff != nil:
		s.lotteryWriteOff.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/lottery/batchContactTags" && r.Method == http.MethodPut && s.lotteryBatchContactTags != nil:
		s.lotteryBatchContactTags.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomFission/page" && (r.Method == http.MethodGet || r.Method == http.MethodHead) && s.roomFissionPage != nil:
		s.roomFissionPage.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomFission/index" && r.Method == http.MethodGet && s.roomFissionIndex != nil:
		s.roomFissionIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomFission/store" && r.Method == http.MethodPost && s.roomFissionStore != nil:
		s.roomFissionStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomFission/info" && r.Method == http.MethodGet && s.roomFissionInfo != nil:
		s.roomFissionInfo.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomFission/update" && r.Method == http.MethodPut && s.roomFissionUpdate != nil:
		s.roomFissionUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomFission/destroy" && r.Method == http.MethodDelete && s.roomFissionDestroy != nil:
		s.roomFissionDestroy.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomFission/invite" && r.Method == http.MethodPost && s.roomFissionInvite != nil:
		s.roomFissionInvite.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomFission/show" && r.Method == http.MethodGet && s.roomFissionShow != nil:
		s.roomFissionShow.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomFission/showRoom" && r.Method == http.MethodGet && s.roomFissionShowRoom != nil:
		s.roomFissionShowRoom.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomFission/showContact" && r.Method == http.MethodGet && s.roomFissionShowContact != nil:
		s.roomFissionShowContact.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomFission/writeOff" && r.Method == http.MethodGet && s.roomFissionWriteOff != nil:
		s.roomFissionWriteOff.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomClockIn/page" && (r.Method == http.MethodGet || r.Method == http.MethodHead) && s.roomClockInPage != nil:
		s.roomClockInPage.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomClockIn/index" && r.Method == http.MethodGet && s.roomClockInIndex != nil:
		s.roomClockInIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomClockIn/store" && r.Method == http.MethodPost && s.roomClockInStore != nil:
		s.roomClockInStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomClockIn/update" && r.Method == http.MethodPut && s.roomClockInUpdate != nil:
		s.roomClockInUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomClockIn/destroy" && r.Method == http.MethodDelete && s.roomClockInDestroy != nil:
		s.roomClockInDestroy.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomClockIn/show" && r.Method == http.MethodGet && s.roomClockInShow != nil:
		s.roomClockInShow.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomClockIn/showContact" && r.Method == http.MethodGet && s.roomClockInShowContact != nil:
		s.roomClockInShowContact.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomClockIn/batchContactTags" && r.Method == http.MethodPut && s.roomClockInBatchContactTags != nil:
		s.roomClockInBatchContactTags.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomClockIn/info" && r.Method == http.MethodGet && s.roomClockInInfo != nil:
		s.roomClockInInfo.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomClockIn/dayDetail" && r.Method == http.MethodGet && s.roomClockInDayDetail != nil:
		s.roomClockInDayDetail.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomQuality/page" && (r.Method == http.MethodGet || r.Method == http.MethodHead) && s.roomQualityPage != nil:
		s.roomQualityPage.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomQuality/index" && r.Method == http.MethodGet && s.roomQualityIndex != nil:
		s.roomQualityIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomQuality/store" && r.Method == http.MethodPost && s.roomQualityStore != nil:
		s.roomQualityStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomQuality/status" && r.Method == http.MethodPut && s.roomQualityStatus != nil:
		s.roomQualityStatus.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomQuality/info" && r.Method == http.MethodGet && s.roomQualityInfo != nil:
		s.roomQualityInfo.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomQuality/update" && r.Method == http.MethodPut && s.roomQualityUpdate != nil:
		s.roomQualityUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomQuality/showContact" && r.Method == http.MethodGet && s.roomQualityShowContact != nil:
		s.roomQualityShowContact.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomQuality/destroy" && r.Method == http.MethodDelete && s.roomQualityDestroy != nil:
		s.roomQualityDestroy.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomQuality/contactDetail" && r.Method == http.MethodGet && s.roomQualityContactDetail != nil:
		s.roomQualityContactDetail.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomCalendar/page" && (r.Method == http.MethodGet || r.Method == http.MethodHead) && s.roomCalendarPage != nil:
		s.roomCalendarPage.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomCalendar/index" && r.Method == http.MethodGet && s.roomCalendarIndex != nil:
		s.roomCalendarIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomCalendar/addRoom" && r.Method == http.MethodPost && s.roomCalendarAddRoom != nil:
		s.roomCalendarAddRoom.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomCalendar/destroyRoom" && r.Method == http.MethodDelete && s.roomCalendarDestroyRoom != nil:
		s.roomCalendarDestroyRoom.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomCalendar/store" && r.Method == http.MethodPost && s.roomCalendarStore != nil:
		s.roomCalendarStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomCalendar/destroy" && r.Method == http.MethodDelete && s.roomCalendarDestroy != nil:
		s.roomCalendarDestroy.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomCalendar/show" && r.Method == http.MethodGet && s.roomCalendarShow != nil:
		s.roomCalendarShow.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomCalendar/update" && r.Method == http.MethodPut && s.roomCalendarUpdate != nil:
		s.roomCalendarUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomRemind/page" && (r.Method == http.MethodGet || r.Method == http.MethodHead) && s.roomRemindPage != nil:
		s.roomRemindPage.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomRemind/index" && r.Method == http.MethodGet && s.roomRemindIndex != nil:
		s.roomRemindIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomRemind/destroy" && r.Method == http.MethodDelete && s.roomRemindDestroy != nil:
		s.roomRemindDestroy.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomRemind/info" && r.Method == http.MethodGet && s.roomRemindInfo != nil:
		s.roomRemindInfo.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomRemind/status" && r.Method == http.MethodGet && s.roomRemindStatus != nil:
		s.roomRemindStatus.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomRemind/store" && r.Method == http.MethodPost && s.roomRemindStore != nil:
		s.roomRemindStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomRemind/update" && r.Method == http.MethodPut && s.roomRemindUpdate != nil:
		s.roomRemindUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/task/roomRemind" && r.Method == http.MethodGet && s.roomRemindTask != nil:
		s.roomRemindTask.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomInfinitePull/page" && (r.Method == http.MethodGet || r.Method == http.MethodHead) && s.roomInfinitePullPage != nil:
		s.roomInfinitePullPage.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomInfinitePull/index" && r.Method == http.MethodGet && s.roomInfinitePullIndex != nil:
		s.roomInfinitePullIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomInfinitePull/info" && r.Method == http.MethodGet && s.roomInfinitePullInfo != nil:
		s.roomInfinitePullInfo.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomInfinitePull/update" && r.Method == http.MethodPut && s.roomInfinitePullUpdate != nil:
		s.roomInfinitePullUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomInfinitePull/destroy" && r.Method == http.MethodDelete && s.roomInfinitePullDestroy != nil:
		s.roomInfinitePullDestroy.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/roomInfinitePull/store" && r.Method == http.MethodPost && s.roomInfinitePullStore != nil:
		s.roomInfinitePullStore.ServeHTTP(w, r)
	case r.URL.Path == "/sidebar/contactFieldPivot/index" && r.Method == http.MethodGet && s.sidebarFieldPivot != nil:
		s.sidebarFieldPivot.ServeHTTP(w, r)
	case r.URL.Path == "/sidebar/contactFieldPivot/update" && r.Method == http.MethodPut && s.sidebarFieldPivotUpdate != nil:
		s.sidebarFieldPivotUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/sidebar/contactBatchAdd/detail" && r.Method == http.MethodGet && s.sidebarContactBatchAddDetail != nil:
		s.sidebarContactBatchAddDetail.ServeHTTP(w, r)
	case r.URL.Path == "/sidebar/contactSop/getSopInfo" && r.Method == http.MethodGet && s.sidebarContactSOPInfo != nil:
		s.sidebarContactSOPInfo.ServeHTTP(w, r)
	case r.URL.Path == "/sidebar/contactSop/getSopTipInfo" && r.Method == http.MethodGet && s.sidebarContactSOPTipInfo != nil:
		s.sidebarContactSOPTipInfo.ServeHTTP(w, r)
	case r.URL.Path == "/sidebar/roomSop/getSopInfo" && r.Method == http.MethodGet && s.sidebarRoomSOPInfo != nil:
		s.sidebarRoomSOPInfo.ServeHTTP(w, r)
	case r.URL.Path == "/sidebar/roomSop/logState" && r.Method == http.MethodPut && s.sidebarRoomSOPLogState != nil:
		s.sidebarRoomSOPLogState.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAlert/page" && (r.Method == http.MethodGet || r.Method == http.MethodHead) && s.saasAlertPage != nil:
		s.saasAlertPage.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAlert/index" && r.Method == http.MethodGet && s.saasAlertIndex != nil:
		s.saasAlertIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAlert/resolve" && (r.Method == http.MethodPut || r.Method == http.MethodPost) && s.saasAlertResolve != nil:
		s.saasAlertResolve.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAlert/setting" && (r.Method == http.MethodGet || r.Method == http.MethodPut || r.Method == http.MethodPost) && s.saasAlertSetting != nil:
		s.saasAlertSetting.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasBilling/page" && (r.Method == http.MethodGet || r.Method == http.MethodHead) && s.saasBillingPage != nil:
		s.saasBillingPage.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasBilling/summary" && r.Method == http.MethodGet && s.saasBillingSummary != nil:
		s.saasBillingSummary.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasBilling/paymentOrders" && r.Method == http.MethodGet && s.saasBillingPaymentOrders != nil:
		s.saasBillingPaymentOrders.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasBilling/paymentRefunds" && r.Method == http.MethodGet && s.saasBillingPaymentRefunds != nil:
		s.saasBillingPaymentRefunds.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasBilling/invoiceProfile" && (r.Method == http.MethodGet || r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasBillingInvoiceProfile != nil:
		s.saasBillingInvoiceProfile.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasBilling/invoices" && r.Method == http.MethodGet && s.saasBillingInvoices != nil:
		s.saasBillingInvoices.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasBilling/invoice" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasBillingInvoice != nil:
		s.saasBillingInvoice.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasBilling/invoiceCancel" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasBillingInvoiceCancel != nil:
		s.saasBillingInvoiceCancel.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/page" && (r.Method == http.MethodGet || r.Method == http.MethodHead) && s.saasAdminPage != nil:
		s.saasAdminPage.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/overview" && r.Method == http.MethodGet && s.saasAdminOverview != nil:
		s.saasAdminOverview.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/tenantReadiness" && r.Method == http.MethodGet && s.saasAdminTenantReadiness != nil:
		s.saasAdminTenantReadiness.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/tenant" && r.Method == http.MethodGet && s.saasAdminTenant != nil:
		s.saasAdminTenant.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/tenantLifecycle" && r.Method == http.MethodGet && s.saasAdminTenantLifecycle != nil:
		s.saasAdminTenantLifecycle.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/usage" && r.Method == http.MethodGet && s.saasAdminUsage != nil:
		s.saasAdminUsage.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/risk" && r.Method == http.MethodGet && s.saasAdminRisk != nil:
		s.saasAdminRisk.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/businessMetrics" && r.Method == http.MethodGet && s.saasAdminBusinessMetrics != nil:
		s.saasAdminBusinessMetrics.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/businessTrends" && r.Method == http.MethodGet && s.saasAdminBusinessTrends != nil:
		s.saasAdminBusinessTrends.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/operationQueue" && r.Method == http.MethodGet && s.saasAdminOperationQueue != nil:
		s.saasAdminOperationQueue.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/operationQueueOwners" && r.Method == http.MethodGet && s.saasAdminOperationQueueOwners != nil:
		s.saasAdminOperationQueueOwners.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/operationQueueAssignments" && r.Method == http.MethodGet && s.saasAdminOperationQueueAssignments != nil:
		s.saasAdminOperationQueueAssignments.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/operationQueueAssignmentClose" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminOperationQueueAssignmentClose != nil:
		s.saasAdminOperationQueueAssignmentClose.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/operationQueueAssignmentNotifications" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminOperationQueueAssignmentNotifications != nil:
		s.saasAdminOperationQueueAssignmentNotifications.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/operationQueueAssign" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminOperationQueueAssign != nil:
		s.saasAdminOperationQueueAssign.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/renewalForecast" && r.Method == http.MethodGet && s.saasAdminRenewalForecast != nil:
		s.saasAdminRenewalForecast.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/renewalForecastTasks" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminRenewalForecastTasks != nil:
		s.saasAdminRenewalForecastTasks.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/renewalForecastAssign" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminRenewalForecastAssign != nil:
		s.saasAdminRenewalForecastAssign.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/renewalForecastNotifications" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminRenewalForecastNotifications != nil:
		s.saasAdminRenewalForecastNotifications.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/customerSuccess" && r.Method == http.MethodGet && s.saasAdminCustomerSuccess != nil:
		s.saasAdminCustomerSuccess.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/customerSuccessOwners" && r.Method == http.MethodGet && s.saasAdminCustomerSuccessOwners != nil:
		s.saasAdminCustomerSuccessOwners.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/customerSuccessAssign" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminCustomerSuccessAssign != nil:
		s.saasAdminCustomerSuccessAssign.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/customerSuccessRenewalTasks" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminCustomerSuccessRenewalTasks != nil:
		s.saasAdminCustomerSuccessRenewalTasks.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/customerSuccessRenewalNotifications" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminCustomerSuccessRenewalNotifications != nil:
		s.saasAdminCustomerSuccessRenewalNotifications.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/riskFollowUp" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminRiskFollowUp != nil:
		s.saasAdminRiskFollowUp.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/riskFollowUps" && r.Method == http.MethodGet && s.saasAdminRiskFollowUps != nil:
		s.saasAdminRiskFollowUps.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/riskFollowUpOwners" && r.Method == http.MethodGet && s.saasAdminRiskFollowUpOwners != nil:
		s.saasAdminRiskFollowUpOwners.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/riskFollowUpBulkClose" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminRiskFollowUpBulkClose != nil:
		s.saasAdminRiskFollowUpBulkClose.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/alerts" && r.Method == http.MethodGet && s.saasAdminAlerts != nil:
		s.saasAdminAlerts.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/alertResolve" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminAlertResolve != nil:
		s.saasAdminAlertResolve.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/alertBulkResolve" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminAlertBulkResolve != nil:
		s.saasAdminAlertBulkResolve.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/notifications" && r.Method == http.MethodGet && s.saasAdminNotifications != nil:
		s.saasAdminNotifications.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/notificationHealth" && r.Method == http.MethodGet && s.saasAdminNotificationHealth != nil:
		s.saasAdminNotificationHealth.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/notificationSlo" && r.Method == http.MethodGet && s.saasAdminNotificationSLO != nil:
		s.saasAdminNotificationSLO.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/notificationHealthRecovery" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminNotificationHealthRecovery != nil:
		s.saasAdminNotificationHealthRecovery.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/notificationPolicies" && r.Method == http.MethodGet && s.saasAdminNotificationPolicies != nil:
		s.saasAdminNotificationPolicies.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/notificationPolicy" && (r.Method == http.MethodGet || r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminNotificationPolicy != nil:
		s.saasAdminNotificationPolicy.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/notificationPolicyTest" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminNotificationPolicyTest != nil:
		s.saasAdminNotificationPolicyTest.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/notificationCredentialRotation" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminNotificationCredentialRotation != nil:
		s.saasAdminNotificationCredentialRotation.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/wecomCredentialProtection" && r.Method == http.MethodGet && s.saasAdminWeComCredentialProtection != nil:
		s.saasAdminWeComCredentialProtection.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/wecomCredentialRotation" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminWeComCredentialRotation != nil:
		s.saasAdminWeComCredentialRotation.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/wechatOpenCredentialProtection" && r.Method == http.MethodGet && s.saasAdminWeChatOpenCredentialProtection != nil:
		s.saasAdminWeChatOpenCredentialProtection.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/wechatOpenCredentialRotation" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminWeChatOpenCredentialRotation != nil:
		s.saasAdminWeChatOpenCredentialRotation.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/notificationRetry" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminNotificationRetry != nil:
		s.saasAdminNotificationRetry.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/notificationBulkRetry" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminNotificationBulkRetry != nil:
		s.saasAdminNotificationBulkRetry.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/notificationClose" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminNotificationClose != nil:
		s.saasAdminNotificationClose.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/notificationBulkClose" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminNotificationBulkClose != nil:
		s.saasAdminNotificationBulkClose.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/packages" && r.Method == http.MethodGet && s.saasAdminPackages != nil:
		s.saasAdminPackages.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/subscriptions" && r.Method == http.MethodGet && s.saasAdminSubscriptions != nil:
		s.saasAdminSubscriptions.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/subscriptionEvents" && r.Method == http.MethodGet && s.saasAdminSubscriptionEvents != nil:
		s.saasAdminSubscriptionEvents.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/subscriptionTransition" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminSubscriptionTransition != nil:
		s.saasAdminSubscriptionTransition.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/subscriptionReconcile" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminSubscriptionReconcile != nil:
		s.saasAdminSubscriptionReconcile.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/paymentOrders" && r.Method == http.MethodGet && s.saasAdminPaymentOrders != nil:
		s.saasAdminPaymentOrders.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/paymentWebhookEvents" && r.Method == http.MethodGet && s.saasAdminPaymentWebhookEvents != nil:
		s.saasAdminPaymentWebhookEvents.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/paymentOrder" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminPaymentOrder != nil:
		s.saasAdminPaymentOrder.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/paymentOrderCancel" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminPaymentOrderCancel != nil:
		s.saasAdminPaymentOrderCancel.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/paymentDunning" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminPaymentDunning != nil:
		s.saasAdminPaymentDunning.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/paymentRefunds" && r.Method == http.MethodGet && s.saasAdminPaymentRefunds != nil:
		s.saasAdminPaymentRefunds.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/paymentRefund" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminPaymentRefund != nil:
		s.saasAdminPaymentRefund.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/paymentRefundCancel" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminPaymentRefundCancel != nil:
		s.saasAdminPaymentRefundCancel.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/paymentSettlementBatches" && r.Method == http.MethodGet && s.saasAdminPaymentSettlementBatches != nil:
		s.saasAdminPaymentSettlementBatches.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/paymentSettlementEntries" && r.Method == http.MethodGet && s.saasAdminPaymentSettlementEntries != nil:
		s.saasAdminPaymentSettlementEntries.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/paymentSettlementImport" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminPaymentSettlementImport != nil:
		s.saasAdminPaymentSettlementImport.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/paymentSettlementReconcile" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminPaymentSettlementReconcile != nil:
		s.saasAdminPaymentSettlementReconcile.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/paymentSettlementResolve" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminPaymentSettlementResolve != nil:
		s.saasAdminPaymentSettlementResolve.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/paymentSettlementTransition" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminPaymentSettlementTransition != nil:
		s.saasAdminPaymentSettlementTransition.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/paymentSettlementSyncRuns" && r.Method == http.MethodGet && s.saasAdminPaymentSettlementSyncRuns != nil:
		s.saasAdminPaymentSettlementSyncRuns.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/paymentSettlementSync" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminPaymentSettlementSync != nil:
		s.saasAdminPaymentSettlementSync.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/accessProfile" && r.Method == http.MethodGet && s.saasAdminAccessProfile != nil:
		s.saasAdminAccessProfile.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/accessRoles" && r.Method == http.MethodGet && s.saasAdminAccessRoles != nil:
		s.saasAdminAccessRoles.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/accessRole" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminAccessRole != nil:
		s.saasAdminAccessRole.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/accessAssignments" && r.Method == http.MethodGet && s.saasAdminAccessAssignments != nil:
		s.saasAdminAccessAssignments.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/accessAssignment" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminAccessAssignment != nil:
		s.saasAdminAccessAssignment.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/systemHealth" && r.Method == http.MethodGet && s.saasAdminSystemHealth != nil:
		s.saasAdminSystemHealth.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/systemHealthScans" && r.Method == http.MethodGet && s.saasAdminSystemHealthScans != nil:
		s.saasAdminSystemHealthScans.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/systemIncidents" && r.Method == http.MethodGet && s.saasAdminSystemIncidents != nil:
		s.saasAdminSystemIncidents.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/systemHealthScan" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminSystemHealthScan != nil:
		s.saasAdminSystemHealthScan.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/systemIncident" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminSystemIncident != nil:
		s.saasAdminSystemIncident.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/serviceAccounts" && r.Method == http.MethodGet && s.saasAdminServiceAccounts != nil:
		s.saasAdminServiceAccounts.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/serviceAccountUsage" && r.Method == http.MethodGet && s.saasAdminServiceAccountUsage != nil:
		s.saasAdminServiceAccountUsage.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/serviceAccountUsageAlertEvaluate" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminServiceAccountUsageAlertEvaluate != nil:
		s.saasAdminServiceAccountUsageAlertEvaluate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/serviceAccount" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminServiceAccount != nil:
		s.saasAdminServiceAccount.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/serviceAccountKeyRotate" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminServiceAccountKeyRotate != nil:
		s.saasAdminServiceAccountKeyRotate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/serviceAccountKeyRevoke" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminServiceAccountKeyRevoke != nil:
		s.saasAdminServiceAccountKeyRevoke.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/auditIntegrity" && r.Method == http.MethodGet && s.saasAdminAuditIntegrity != nil:
		s.saasAdminAuditIntegrity.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/auditIntegrityVerify" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminAuditIntegrityVerify != nil:
		s.saasAdminAuditIntegrityVerify.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/auditAnchors" && r.Method == http.MethodGet && s.saasAdminAuditAnchors != nil:
		s.saasAdminAuditAnchors.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/auditAnchor" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminAuditAnchor != nil:
		s.saasAdminAuditAnchor.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/backupOverview" && r.Method == http.MethodGet && s.saasAdminBackupOverview != nil:
		s.saasAdminBackupOverview.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/backupPolicy" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminBackupPolicy != nil:
		s.saasAdminBackupPolicy.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/backupRun" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminBackupRun != nil:
		s.saasAdminBackupRun.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/restoreDrill" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminRestoreDrill != nil:
		s.saasAdminRestoreDrill.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/complianceOverview" && r.Method == http.MethodGet && s.saasAdminComplianceOverview != nil:
		s.saasAdminComplianceOverview.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/compliancePolicy" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminCompliancePolicy != nil:
		s.saasAdminCompliancePolicy.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/complianceLegalHold" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminComplianceLegalHold != nil:
		s.saasAdminComplianceLegalHold.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/complianceExport" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminComplianceExport != nil:
		s.saasAdminComplianceExport.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/complianceExportDownload" && r.Method == http.MethodGet && s.saasAdminComplianceExportDownload != nil:
		s.saasAdminComplianceExportDownload.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/complianceErasure" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminComplianceErasure != nil:
		s.saasAdminComplianceErasure.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/complianceErasureSteps" && r.Method == http.MethodGet && s.saasAdminComplianceErasureSteps != nil:
		s.saasAdminComplianceErasureSteps.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/identityOverview" && r.Method == http.MethodGet && s.saasAdminIdentityOverview != nil:
		s.saasAdminIdentityOverview.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/identityPolicy" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminIdentityPolicy != nil:
		s.saasAdminIdentityPolicy.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/identitySessions" && r.Method == http.MethodGet && s.saasAdminIdentitySessions != nil:
		s.saasAdminIdentitySessions.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/identitySession" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminIdentitySession != nil:
		s.saasAdminIdentitySession.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/identityLoginEvents" && r.Method == http.MethodGet && s.saasAdminIdentityLoginEvents != nil:
		s.saasAdminIdentityLoginEvents.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/identityIncidents" && r.Method == http.MethodGet && s.saasAdminIdentityIncidents != nil:
		s.saasAdminIdentityIncidents.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/identityIncident" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminIdentityIncident != nil:
		s.saasAdminIdentityIncident.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/identityUser" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminIdentityUser != nil:
		s.saasAdminIdentityUser.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/identityMFA" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminIdentityMFA != nil:
		s.saasAdminIdentityMFA.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/brandingProfiles" && r.Method == http.MethodGet && s.saasAdminBrandingProfiles != nil:
		s.saasAdminBrandingProfiles.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/brandingProfile" && (r.Method == http.MethodGet || r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminBrandingProfile != nil:
		s.saasAdminBrandingProfile.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/tenantDomains" && r.Method == http.MethodGet && s.saasAdminTenantDomains != nil:
		s.saasAdminTenantDomains.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/tenantDomain" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminTenantDomain != nil:
		s.saasAdminTenantDomain.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/tenantDomainDeliveryJobs" && r.Method == http.MethodGet && s.saasAdminTenantDomainDeliveryJobs != nil:
		s.saasAdminTenantDomainDeliveryJobs.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/tenantDomainDelivery" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminTenantDomainDelivery != nil:
		s.saasAdminTenantDomainDelivery.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/releaseReadiness" && r.Method == http.MethodGet && s.saasAdminReleaseReadiness != nil:
		s.saasAdminReleaseReadiness.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/releaseEvidence" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminReleaseEvidence != nil:
		s.saasAdminReleaseEvidence.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/releaseEvidenceAction" && r.Method == http.MethodPut && s.saasAdminReleaseEvidenceAction != nil:
		s.saasAdminReleaseEvidenceAction.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/releaseCandidate" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminReleaseCandidate != nil:
		s.saasAdminReleaseCandidate.ServeHTTP(w, r)
	case r.URL.Path == "/api/saas/v1/whoami" && r.Method == http.MethodGet && s.saasServiceAccountWhoAmI != nil:
		s.saasServiceAccountWhoAmI.ServeHTTP(w, r)
	case r.URL.Path == "/api/saas/v1/usage" && r.Method == http.MethodGet && s.saasServiceAccountUsage != nil:
		s.saasServiceAccountUsage.ServeHTTP(w, r)
	case r.URL.Path == "/api/saas/v1/alerts" && r.Method == http.MethodGet && s.saasServiceAccountAlerts != nil:
		s.saasServiceAccountAlerts.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/approvalPolicies" && r.Method == http.MethodGet && s.saasAdminApprovalPolicies != nil:
		s.saasAdminApprovalPolicies.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/approvalPolicy" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminApprovalPolicy != nil:
		s.saasAdminApprovalPolicy.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/approvals" && r.Method == http.MethodGet && s.saasAdminApprovals != nil:
		s.saasAdminApprovals.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/approvalEvents" && r.Method == http.MethodGet && s.saasAdminApprovalEvents != nil:
		s.saasAdminApprovalEvents.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/approvalDecisions" && r.Method == http.MethodGet && s.saasAdminApprovalDecisions != nil:
		s.saasAdminApprovalDecisions.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/approvalDelegations" && r.Method == http.MethodGet && s.saasAdminApprovalDelegations != nil:
		s.saasAdminApprovalDelegations.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/approvalDelegation" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminApprovalDelegation != nil:
		s.saasAdminApprovalDelegation.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/approvalReminders" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminApprovalReminders != nil:
		s.saasAdminApprovalReminders.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/approvalRequest" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminApprovalRequest != nil:
		s.saasAdminApprovalRequest.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/approvalDecision" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminApprovalDecision != nil:
		s.saasAdminApprovalDecision.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/approvalCancel" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminApprovalCancel != nil:
		s.saasAdminApprovalCancel.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/approvalExecute" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminApprovalExecute != nil:
		s.saasAdminApprovalExecute.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/invoiceProfile" && (r.Method == http.MethodGet || r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminInvoiceProfile != nil:
		s.saasAdminInvoiceProfile.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/invoiceDocuments" && r.Method == http.MethodGet && s.saasAdminInvoiceDocuments != nil:
		s.saasAdminInvoiceDocuments.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/invoice" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminInvoice != nil:
		s.saasAdminInvoice.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/creditNote" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminCreditNote != nil:
		s.saasAdminCreditNote.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/invoiceTransition" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminInvoiceTransition != nil:
		s.saasAdminInvoiceTransition.ServeHTTP(w, r)
	case r.URL.Path == "/webhooks/saas/payment" && r.Method == http.MethodPost && s.saasPaymentWebhook != nil:
		s.saasPaymentWebhook.ServeHTTP(w, r)
	case r.URL.Path == "/webhooks/saas/domain-delivery" && r.Method == http.MethodPost && s.saasTenantDomainDeliveryWebhook != nil:
		s.saasTenantDomainDeliveryWebhook.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/operations" && r.Method == http.MethodGet && s.saasAdminOperations != nil:
		s.saasAdminOperations.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/billingEvents" && r.Method == http.MethodGet && s.saasAdminBillingEvents != nil:
		s.saasAdminBillingEvents.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/billingReconciliation" && r.Method == http.MethodGet && s.saasAdminBillingReconciliation != nil:
		s.saasAdminBillingReconciliation.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/billingReconciliationFollowUp" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminBillingReconciliationFollowUp != nil:
		s.saasAdminBillingReconciliationFollowUp.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/billingReconciliationFollowUps" && r.Method == http.MethodGet && s.saasAdminBillingReconciliationFollowUps != nil:
		s.saasAdminBillingReconciliationFollowUps.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/billingReconciliationFollowUpOwners" && r.Method == http.MethodGet && s.saasAdminBillingReconciliationFollowUpOwners != nil:
		s.saasAdminBillingReconciliationFollowUpOwners.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/billingReconciliationFollowUpBulkClose" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminBillingReconciliationFollowUpBulkClose != nil:
		s.saasAdminBillingReconciliationFollowUpBulkClose.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/tasks" && r.Method == http.MethodGet && s.saasAdminTasks != nil:
		s.saasAdminTasks.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/taskOwners" && r.Method == http.MethodGet && s.saasAdminTaskOwners != nil:
		s.saasAdminTaskOwners.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/taskSla" && r.Method == http.MethodGet && s.saasAdminTaskSLA != nil:
		s.saasAdminTaskSLA.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/taskSlaNotifications" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminTaskSLANotifications != nil:
		s.saasAdminTaskSLANotifications.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/taskCancel" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminTaskCancel != nil:
		s.saasAdminTaskCancel.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/taskBulkCancel" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminTaskBulkCancel != nil:
		s.saasAdminTaskBulkCancel.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/taskBulkReset" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminTaskBulkReset != nil:
		s.saasAdminTaskBulkReset.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/taskReset" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminTaskReset != nil:
		s.saasAdminTaskReset.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/dailyReport" && r.Method == http.MethodGet && s.saasAdminDailyReport != nil:
		s.saasAdminDailyReport.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/export" && r.Method == http.MethodGet && s.saasAdminExport != nil:
		s.saasAdminExport.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/package" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminPackage != nil:
		s.saasAdminPackage.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/packageSync" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminPackageSync != nil:
		s.saasAdminPackageSync.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/packageSyncTask" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminPackageSyncTask != nil:
		s.saasAdminPackageSyncTask.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/packageSyncTaskApply" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminPackageSyncTaskApply != nil:
		s.saasAdminPackageSyncTaskApply.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/packageSyncTaskBulkApply" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminPackageSyncTaskBulkApply != nil:
		s.saasAdminPackageSyncTaskBulkApply.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/tenantStatus" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminTenantStatus != nil:
		s.saasAdminTenantStatus.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/tenantRenewal" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminTenantRenewal != nil:
		s.saasAdminTenantRenewal.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/tenantRenewalTask" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminTenantRenewalTask != nil:
		s.saasAdminTenantRenewalTask.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/tenantRenewalTaskApply" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminTenantRenewalTaskApply != nil:
		s.saasAdminTenantRenewalTaskApply.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/tenantRenewalTaskBulkApply" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminTenantRenewalTaskBulkApply != nil:
		s.saasAdminTenantRenewalTaskBulkApply.ServeHTTP(w, r)
	case isSaaSAdminDashboardProvisioningRoute(r.Method, r.URL.Path) && s.saasAdminDashboardProvisioning != nil:
		s.saasAdminDashboardProvisioning.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/tenantProvision" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminTenantProvision != nil:
		s.saasAdminTenantProvision.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/tenantProvisionTask" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminTenantProvisionTask != nil:
		s.saasAdminTenantProvisionTask.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/tenantProvisionTaskApply" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminTenantProvisionTaskApply != nil:
		s.saasAdminTenantProvisionTaskApply.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/tenantProvisionTaskBulkApply" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminTenantProvisionTaskBulkApply != nil:
		s.saasAdminTenantProvisionTaskBulkApply.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/saasAdmin/tenantPackage" && (r.Method == http.MethodPost || r.Method == http.MethodPut) && s.saasAdminTenantPackage != nil:
		s.saasAdminTenantPackage.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/chatTool/config" && r.Method == http.MethodGet && s.chatToolConfig != nil:
		s.chatToolConfig.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/common/upload" && r.Method == http.MethodPost && s.commonUpload != nil:
		s.commonUpload.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/common/uploadFile" && r.Method == http.MethodPost && s.commonUploadFile != nil:
		s.commonUploadFile.ServeHTTP(w, r)
	case r.URL.Path == "/sidebar/common/upload" && r.Method == http.MethodPost && s.sidebarCommonUpload != nil:
		s.sidebarCommonUpload.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/agent/txtVerifyUpload" && r.Method == http.MethodPost && s.agentTxtUpload != nil:
		s.agentTxtUpload.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/agent/store" && r.Method == http.MethodPost && s.agentStore != nil:
		s.agentStore.ServeHTTP(w, r)
	case r.URL.Path == "/sidebar/agent/auth" && (r.Method == http.MethodGet || r.Method == http.MethodPost) && s.sidebarAgentAuth != nil:
		s.sidebarAgentAuth.ServeHTTP(w, r)
	case r.URL.Path == "/sidebar/agent/oauth" && r.Method == http.MethodGet && s.sidebarAgentOAuth != nil:
		s.sidebarAgentOAuth.ServeHTTP(w, r)
	case r.URL.Path == "/sidebar/agent/jssdkConfig" && r.Method == http.MethodGet && s.sidebarAgentJSSDK != nil:
		s.sidebarAgentJSSDK.ServeHTTP(w, r)
	case r.URL.Path == "/sidebar/wxJsSdk/config" && r.Method == http.MethodGet && s.sidebarWxJSSDK != nil:
		s.sidebarWxJSSDK.ServeHTTP(w, r)
	case s.agentTxtVerify && r.Method == http.MethodGet && isAgentTxtVerifyPath(r.URL.Path):
		s.handleAgentTxtVerify(w, r)
	case r.URL.Path == "/dashboard/role/select" && r.Method == http.MethodGet && s.roleSelect != nil:
		s.roleSelect.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/role/index" && r.Method == http.MethodGet && s.roleIndex != nil:
		s.roleIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/role/show" && r.Method == http.MethodGet && s.roleShow != nil:
		s.roleShow.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/role/permissionShow" && r.Method == http.MethodGet && s.rolePermission != nil:
		s.rolePermission.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/role/showEmployee" && r.Method == http.MethodGet && s.roleShowEmployee != nil:
		s.roleShowEmployee.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/role/store" && r.Method == http.MethodPost && s.roleStore != nil:
		s.roleStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/role/update" && r.Method == http.MethodPut && s.roleUpdate != nil:
		s.roleUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/role/statusUpdate" && r.Method == http.MethodPut && s.roleStatusUpdate != nil:
		s.roleStatusUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/role/destroy" && r.Method == http.MethodDelete && s.roleDestroy != nil:
		s.roleDestroy.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/role/permissionStore" && r.Method == http.MethodPost && s.rolePermissionStore != nil:
		s.rolePermissionStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/menu/iconIndex" && r.Method == http.MethodGet && s.menuIconIndex != nil:
		s.menuIconIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/menu/select" && r.Method == http.MethodGet && s.menuSelect != nil:
		s.menuSelect.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/menu/index" && r.Method == http.MethodGet && s.menuIndex != nil:
		s.menuIndex.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/menu/show" && r.Method == http.MethodGet && s.menuShow != nil:
		s.menuShow.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/menu/store" && r.Method == http.MethodPost && s.menuStore != nil:
		s.menuStore.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/menu/update" && r.Method == http.MethodPut && s.menuUpdate != nil:
		s.menuUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/menu/statusUpdate" && r.Method == http.MethodPut && s.menuStatusUpdate != nil:
		s.menuStatusUpdate.ServeHTTP(w, r)
	case r.URL.Path == "/dashboard/menu/destroy" && r.Method == http.MethodDelete && s.menuDestroy != nil:
		s.menuDestroy.ServeHTTP(w, r)
	default:
		s.forwardToPHP(w, r)
	}
}

func isRetiredDashboardEndpoint(path string) bool {
	securityParts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(securityParts) == 2 && securityParts[0] == "security" && securityParts[1] == "login" {
		return true
	}
	parts := strings.Split(strings.TrimPrefix(path, "/dashboard/"), "/")
	if len(parts) != 2 || parts[0] != "corp" {
		return false
	}
	switch parts[1] {
	case "select", "bind", "index", "show", "store", "update":
		return true
	default:
		return false
	}
}

func isDashboardSaaSRequestPath(path string) bool {
	return strings.HasPrefix(path, "/dashboard/saasAdmin/") ||
		strings.HasPrefix(path, "/dashboard/saasAlert/") ||
		strings.HasPrefix(path, "/dashboard/saasBilling/")
}

func normalizeBundledFrontendAPIPath(path string) (string, bool) {
	for _, prefix := range []string{"/undefined/dashboard/", "/undefined/sidebar/", "/undefined/operation/"} {
		if strings.HasPrefix(path, prefix) {
			return strings.TrimPrefix(path, "/undefined"), true
		}
	}
	return "", false
}

func cloneURL(src *url.URL) *url.URL {
	if src == nil {
		return &url.URL{}
	}
	clone := *src
	return &clone
}

func serveHTTPWithPath(handler http.Handler, w http.ResponseWriter, r *http.Request, path string) {
	if r.URL.Path == path {
		handler.ServeHTTP(w, r)
		return
	}
	cloned := r.Clone(r.Context())
	cloned.URL = cloneURL(r.URL)
	cloned.URL.Path = path
	cloned.URL.RawPath = ""
	handler.ServeHTTP(w, cloned)
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = w.Write([]byte("Hello MoChat "))
}

func (s *Server) serveStaticUpload(w http.ResponseWriter, r *http.Request) {
	root := strings.TrimSpace(s.cfg.FileStorageRoot)
	if root == "" {
		http.NotFound(w, r)
		return
	}
	rel := strings.TrimPrefix(r.URL.Path, "/static/")
	rel = strings.TrimLeft(rel, "/")
	if rel == "" {
		http.NotFound(w, r)
		return
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || clean == ".." || filepath.IsAbs(clean) || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		http.NotFound(w, r)
		return
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	targetAbs, err := filepath.Abs(filepath.Join(rootAbs, clean))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if targetAbs != rootAbs && !strings.HasPrefix(targetAbs, rootAbs+string(os.PathSeparator)) {
		http.NotFound(w, r)
		return
	}
	info, err := os.Stat(targetAbs)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, targetAbs)
}

func (s *Server) forwardToPHP(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Standalone {
		writeJSON(w, http.StatusNotImplemented, map[string]any{
			"code":                 http.StatusNotImplemented,
			"message":              "route not yet migrated in standalone Go runtime",
			"path":                 r.URL.Path,
			"method":               r.Method,
			"migrated_route_count": len(s.migratedRoutes()),
		})
		return
	}
	if s.proxy == nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"code":    http.StatusBadGateway,
			"message": "no PHP upstream configured for unmigrated route",
		})
		return
	}
	s.proxy.ServeHTTP(w, r)
}

func (s *Server) handleRoutes(w http.ResponseWriter) {
	content := embeddedCompatManifest
	if s.cfg.ManifestPath != "" {
		var err error
		content, err = os.ReadFile(s.cfg.ManifestPath)
		if err != nil {
			if !s.cfg.Standalone {
				writeJSON(w, http.StatusNotFound, map[string]any{
					"code":    http.StatusNotFound,
					"message": "compat manifest not found",
					"path":    s.cfg.ManifestPath,
				})
				return
			}
			content = embeddedCompatManifest
		}
	}

	var manifest map[string]any
	if err := json.Unmarshal(content, &manifest); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"code":    http.StatusInternalServerError,
			"message": "compat manifest is invalid JSON",
			"error":   err.Error(),
		})
		return
	}

	manifest["migrated_routes"] = s.migratedRoutes()
	manifest["migrated_route_count"] = len(s.migratedRoutes())
	writeJSON(w, http.StatusOK, manifest)
}

func (s *Server) status() statusPayload {
	sourceExists := false
	manifestExists := len(embeddedCompatManifest) > 0
	if s.cfg.SourceRoot != "" {
		_, statErr := os.Stat(s.cfg.SourceRoot)
		sourceExists = statErr == nil
	}
	if s.cfg.ManifestPath != "" {
		_, manifestErr := os.Stat(s.cfg.ManifestPath)
		manifestExists = manifestErr == nil
	}
	mode := "php-first-strangler"
	nextBoundary := "auth/tenant/rbac"
	if s.cfg.Standalone {
		mode = "standalone-go"
		nextBoundary = "complete remaining PHP fallback routes"
	}
	var backgroundTasks []taskrunner.Snapshot
	if s.backgroundTasks != nil {
		backgroundTasks = s.backgroundTasks()
	}
	return statusPayload{
		Name:                  "mochat-go",
		Mode:                  mode,
		StartedAt:             s.startedAt.Format(time.RFC3339),
		Standalone:            s.cfg.Standalone,
		SourceRoot:            s.cfg.SourceRoot,
		SourceRootExists:      sourceExists,
		ManifestPath:          s.cfg.ManifestPath,
		ManifestExists:        manifestExists,
		PHPUpstream:           s.cfg.PHPUpstream,
		PHPUpstreamReady:      false,
		PHPUpstreamProbe:      "not probed",
		ProxyFallbackEnabled:  s.proxy != nil,
		MigratedRouteCount:    len(s.migratedRoutes()),
		MigratedRoutes:        s.migratedRoutes(),
		BackgroundTasks:       backgroundTasks,
		NextMigrationBoundary: nextBoundary,
	}
}

func (s *Server) migratedRoutes() []string {
	routes := append([]string{}, migratedRoutes...)
	if s.dashboardAuth != nil {
		routes = append(routes,
			"POST /dashboard/user/auth",
			"POST /dashboard/user/authMFA",
			"POST /dashboard/auth/activate",
			"POST /dashboard/auth/password/reset-request",
			"POST /dashboard/auth/password/reset",
			"GET /dashboard/auth/session",
			"POST /dashboard/auth/logout",
			"PUT /dashboard/user/logout",
		)
	}
	if s.companyProfile != nil {
		routes = append(routes,
			"GET /dashboard/company/profile",
			"PUT /dashboard/company/profile",
			"PUT /dashboard/company/wecom-credentials",
			"PUT /dashboard/company/agent-credentials",
			"PUT /dashboard/company/application-credentials",
			"PUT /dashboard/company/archive-credentials",
			"GET /dashboard/company/callback-configuration",
			"POST /dashboard/company/callback-configuration/regenerate",
			"POST /dashboard/company/verify",
			"POST /dashboard/company/employee-sync",
			"GET /dashboard/company/sync-status",
			"GET /dashboard/company/audits",
		)
	}
	if s.providerStatus != nil {
		routes = append(routes, "GET /dashboard/providers/status")
	}
	if s.identitySelf != nil {
		routes = append(routes, "GET /dashboard/user/securityMFA", "POST /dashboard/user/securityMFA", "PUT /dashboard/user/securityMFA")
	}
	if s.logout != nil {
		routes = append(routes, "PUT /dashboard/user/logout")
	}
	if s.userIndex != nil {
		routes = append(routes, "GET /dashboard/user/index")
	}
	if s.userShow != nil {
		routes = append(routes, "GET /dashboard/user/show")
	}
	if s.userStore != nil {
		routes = append(routes, "POST /dashboard/user/store")
	}
	if s.userUpdate != nil {
		routes = append(routes, "PUT /dashboard/user/update")
	}
	if s.userStatusUpdate != nil {
		routes = append(routes, "PUT /dashboard/user/statusUpdate")
	}
	if s.userPasswordReset != nil {
		routes = append(routes, "PUT /dashboard/user/passwordReset")
	}
	if s.userPasswordUpdate != nil {
		routes = append(routes, "PUT /dashboard/user/passwordUpdate")
	}
	if s.permissionByUser != nil {
		routes = append(routes, "GET /dashboard/role/permissionByUser")
	}
	if s.weWorkCallback != nil {
		routes = append(routes,
			"GET /dashboard/corp/weWorkCallback",
			"POST /dashboard/corp/weWorkCallback",
			"GET /weWork/callback",
			"POST /weWork/callback",
		)
	}
	if s.corpDataIndex != nil {
		routes = append(routes, "GET /dashboard/corpData/index")
	}
	if s.corpDataLineChat != nil {
		routes = append(routes, "GET /dashboard/corpData/lineChat")
	}
	if s.externalTenantIndex != nil {
		routes = append(routes, "GET /dashboard/external/tenantIndex")
	}
	if s.statisticIndex != nil {
		routes = append(routes, "GET /dashboard/statistic/index")
	}
	if s.statisticTopList != nil {
		routes = append(routes, "GET /dashboard/statistic/topList")
	}
	if s.statisticEmployeeCounts != nil {
		routes = append(routes, "GET /dashboard/statistic/employeeCounts")
	}
	if s.statisticEmployees != nil {
		routes = append(routes, "GET /dashboard/statistic/employees")
	}
	if s.statisticEmployeesTrend != nil {
		routes = append(routes, "GET /dashboard/statistic/employeesTrend")
	}
	if s.workEmployeeCond != nil {
		routes = append(routes, "GET /dashboard/workEmployee/searchCondition")
	}
	if s.workEmployeeIndex != nil {
		routes = append(routes, "GET /dashboard/workEmployee/index")
	}
	if s.workEmployeeSync != nil {
		routes = append(routes, "PUT /dashboard/workEmployee/synEmployee")
	}
	if s.workDeptIndex != nil {
		routes = append(routes, "GET /dashboard/workDepartment/index")
	}
	if s.workDeptMember != nil {
		routes = append(routes, "GET /dashboard/workEmployeeDepartment/memberIndex")
		routes = append(routes, "GET /dashboard/workDepartment/memberIndex")
	}
	if s.workDeptPhone != nil {
		routes = append(routes, "GET /dashboard/workDepartment/selectByPhone")
	}
	if s.workDeptPage != nil {
		routes = append(routes, "GET /dashboard/workDepartment/pageIndex")
	}
	if s.workDeptEmployee != nil {
		routes = append(routes, "GET /dashboard/workDepartment/showEmployee")
	}
	if s.workTagGroupIndex != nil {
		routes = append(routes, "GET /dashboard/workContactTagGroup/index")
	}
	if s.workTagGroupDetail != nil {
		routes = append(routes, "GET /dashboard/workContactTagGroup/detail")
	}
	if s.workTagGroupStore != nil {
		routes = append(routes, "POST /dashboard/workContactTagGroup/store")
	}
	if s.workTagGroupUpdate != nil {
		routes = append(routes, "PUT /dashboard/workContactTagGroup/update")
	}
	if s.workTagGroupDestroy != nil {
		routes = append(routes, "DELETE /dashboard/workContactTagGroup/destroy")
	}
	if s.sidebarTagGroupIndex != nil {
		routes = append(routes, "GET /sidebar/workContactTagGroup/index")
	}
	if s.workContactTagIndex != nil {
		routes = append(routes, "GET /dashboard/workContactTag/index")
	}
	if s.workContactTagDetail != nil {
		routes = append(routes, "GET /dashboard/workContactTag/detail")
	}
	if s.workContactTagList != nil {
		routes = append(routes, "GET /dashboard/workContactTag/contactTagList")
	}
	if s.workContactTagAll != nil {
		routes = append(routes, "GET /dashboard/workContactTag/allTag")
	}
	if s.workContactTagStore != nil {
		routes = append(routes, "POST /dashboard/workContactTag/store")
	}
	if s.workContactTagUpdate != nil {
		routes = append(routes, "PUT /dashboard/workContactTag/update")
	}
	if s.workContactTagDestroy != nil {
		routes = append(routes, "DELETE /dashboard/workContactTag/destroy")
	}
	if s.workContactTagMove != nil {
		routes = append(routes, "PUT /dashboard/workContactTag/move")
	}
	if s.workContactTagSync != nil {
		routes = append(routes, "PUT /dashboard/workContactTag/synContactTag")
	}
	if s.workContactSync != nil {
		routes = append(routes, "PUT /dashboard/workContact/synContact")
	}
	if s.workContactIndex != nil {
		routes = append(routes, "GET /dashboard/workContact/index")
	}
	if s.workContactLoss != nil {
		routes = append(routes, "GET /dashboard/workContact/lossContact")
	}
	if s.workContactSource != nil {
		routes = append(routes, "GET /dashboard/workContact/source")
	}
	if s.workContactShow != nil {
		routes = append(routes, "GET /dashboard/workContact/show")
	}
	if s.workContactTrack != nil {
		routes = append(routes, "GET /dashboard/workContact/track")
	}
	if s.workContactUpdate != nil {
		routes = append(routes, "PUT /dashboard/workContact/update")
	}
	if s.workContactBatchLabeling != nil {
		routes = append(routes, "POST /dashboard/workContact/batchLabeling")
	}
	if s.workContactRoomIndex != nil {
		routes = append(routes, "GET /dashboard/workContactRoom/index")
	}
	if s.workRoomIndex != nil {
		routes = append(routes, "GET /dashboard/workRoom/index")
	}
	if s.workRoomRoomIndex != nil {
		routes = append(routes, "GET /dashboard/workRoom/roomIndex")
	}
	if s.workRoomStatistics != nil {
		routes = append(routes, "GET /dashboard/workRoom/statistics")
	}
	if s.workRoomStatisticsIndex != nil {
		routes = append(routes, "GET /dashboard/workRoom/statisticsIndex")
	}
	if s.workRoomSync != nil {
		routes = append(routes, "PUT /dashboard/workRoom/syn")
	}
	if s.workRoomBatchUpdate != nil {
		routes = append(routes, "PUT /dashboard/workRoom/batchUpdate")
	}
	if s.sidebarWorkRoomManage != nil {
		routes = append(routes, "GET /sidebar/workRoom/roomManage")
	}
	if s.contactTransferInfo != nil {
		routes = append(routes, "GET /dashboard/contactTransfer/info")
	}
	if s.contactTransferUnassigned != nil {
		routes = append(routes, "GET /dashboard/contactTransfer/unassignedList")
	}
	if s.contactTransferRoom != nil {
		routes = append(routes, "GET /dashboard/contactTransfer/room")
	}
	if s.contactTransferLog != nil {
		routes = append(routes, "GET /dashboard/contactTransfer/log")
	}
	if s.contactTransferSync != nil {
		routes = append(routes, "GET /dashboard/contactTransfer/saveUnassignedList")
	}
	if s.contactTransferCustomer != nil {
		routes = append(routes, "POST /dashboard/contactTransfer/index")
	}
	if s.contactTransferRoomStore != nil {
		routes = append(routes, "POST /dashboard/contactTransfer/room")
	}
	if s.workRoomAutoPullIndex != nil {
		routes = append(routes, "GET /dashboard/workRoomAutoPull/index")
	}
	if s.workRoomAutoPullShow != nil {
		routes = append(routes, "GET /dashboard/workRoomAutoPull/show")
	}
	if s.workRoomAutoPullStore != nil {
		routes = append(routes, "POST /dashboard/workRoomAutoPull/store")
	}
	if s.workRoomAutoPullUpdate != nil {
		routes = append(routes, "PUT /dashboard/workRoomAutoPull/update")
	}
	if s.workRoomAutoPullMove != nil {
		routes = append(routes, "PUT /dashboard/workRoomAutoPull/move")
	}
	if s.roomTagPullIndex != nil {
		routes = append(routes, "GET /dashboard/roomTagPull/index")
	}
	if s.roomTagPullShow != nil {
		routes = append(routes, "GET /dashboard/roomTagPull/show")
	}
	if s.roomTagPullShowContact != nil {
		routes = append(routes, "GET /dashboard/roomTagPull/showContact")
	}
	if s.roomTagPullContactDetailPage != nil {
		routes = append(routes,
			"GET /dashboard/roomTagPull/contactDetail",
			"HEAD /dashboard/roomTagPull/contactDetail",
			"GET /roomTagPull/contactDetail",
			"HEAD /roomTagPull/contactDetail",
			"GET /roomTagPull/clientDetails",
			"HEAD /roomTagPull/clientDetails",
		)
	}
	if s.roomTagPullRoomList != nil {
		routes = append(routes, "GET /dashboard/roomTagPull/roomList")
	}
	if s.roomTagPullChooseContact != nil {
		routes = append(routes, "GET /dashboard/roomTagPull/chooseContact")
	}
	if s.roomTagPullStore != nil {
		routes = append(routes, "POST /dashboard/roomTagPull/store")
	}
	if s.roomTagPullFilterContact != nil {
		routes = append(routes, "POST /dashboard/roomTagPull/filterContact")
	}
	if s.roomTagPullRemindSend != nil {
		routes = append(routes, "GET /dashboard/roomTagPull/remindSend")
	}
	if s.roomTagPullDestroy != nil {
		routes = append(routes, "DELETE /dashboard/roomTagPull/destroy")
	}
	if s.contactMessageBatchSendIndex != nil {
		routes = append(routes, "GET /dashboard/contactMessageBatchSend/index")
	}
	if s.contactMessageBatchSendShow != nil {
		routes = append(routes, "GET /dashboard/contactMessageBatchSend/show")
	}
	if s.contactMessageBatchSendMessageShow != nil {
		routes = append(routes, "GET /dashboard/contactMessageBatchSend/messageShow")
	}
	if s.contactMessageBatchSendShowRoom != nil {
		routes = append(routes, "GET /dashboard/contactMessageBatchSend/showRoom")
	}
	if s.contactMessageBatchSendEmployee != nil {
		routes = append(routes, "GET /dashboard/contactMessageBatchSend/employeeSendIndex")
	}
	if s.contactMessageBatchSendContactReceive != nil {
		routes = append(routes, "GET /dashboard/contactMessageBatchSend/contactReceiveIndex")
	}
	if s.contactMessageBatchSendStore != nil {
		routes = append(routes, "POST /dashboard/contactMessageBatchSend/store")
	}
	if s.contactMessageBatchSendRemind != nil {
		routes = append(routes, "POST /dashboard/contactMessageBatchSend/remind")
	}
	if s.contactMessageBatchSendDestroy != nil {
		routes = append(routes, "DELETE /dashboard/contactMessageBatchSend/destroy")
	}
	if s.roomMessageBatchSendIndex != nil {
		routes = append(routes, "GET /dashboard/roomMessageBatchSend/index")
	}
	if s.roomMessageBatchSendShow != nil {
		routes = append(routes, "GET /dashboard/roomMessageBatchSend/show")
	}
	if s.roomMessageBatchSendOwner != nil {
		routes = append(routes, "GET /dashboard/roomMessageBatchSend/roomOwnerSendIndex")
	}
	if s.roomMessageBatchSendRoomReceive != nil {
		routes = append(routes, "GET /dashboard/roomMessageBatchSend/roomReceiveIndex")
	}
	if s.roomMessageBatchSendStore != nil {
		routes = append(routes, "POST /dashboard/roomMessageBatchSend/store")
	}
	if s.roomMessageBatchSendRemind != nil {
		routes = append(routes, "GET /dashboard/roomMessageBatchSend/remind")
	}
	if s.roomMessageBatchSendDestroy != nil {
		routes = append(routes, "DELETE /dashboard/roomMessageBatchSend/destroy")
	}
	if s.officialAccountIndex != nil {
		routes = append(routes, "GET /dashboard/officialAccount/index")
	}
	if s.officialAccountSet != nil {
		routes = append(routes, "GET /dashboard/officialAccount/set")
	}
	if s.officialAccountGetPreAuthURL != nil {
		routes = append(routes, "GET /dashboard/officialAccount/getPreAuthUrl")
	}
	if s.officialAccountAuthRedirect != nil {
		routes = append(routes, "GET /dashboard/officialAccount/authRedirect/", "POST /dashboard/officialAccount/authRedirect/")
	}
	if s.officialAccountAuthEventCallback != nil {
		routes = append(routes, "GET /dashboard/officialAccount/authEventCallback", "POST /dashboard/officialAccount/authEventCallback")
	}
	if s.officialAccountMessageEventCallback != nil {
		routes = append(routes, "GET /dashboard/{appId}/officialAccount/messageEventCallback", "POST /dashboard/{appId}/officialAccount/messageEventCallback")
	}
	if s.load != nil {
		routes = append(routes, "GET /load/{params?}", "POST /load/{params?}")
	}
	if s.workFissionIndex != nil {
		routes = append(routes, "GET /dashboard/workFission/index")
	}
	if s.workFissionShow != nil {
		routes = append(routes, "GET /dashboard/workFission/show")
	}
	if s.workFissionInfo != nil {
		routes = append(routes, "GET /dashboard/workFission/info")
	}
	if s.workFissionStatistics != nil {
		routes = append(routes, "GET /dashboard/workFission/statistics")
	}
	if s.workFissionChooseContact != nil {
		routes = append(routes, "GET /dashboard/workFission/chooseContact")
	}
	if s.workFissionStore != nil {
		routes = append(routes, "POST /dashboard/workFission/store")
	}
	if s.workFissionUpdate != nil {
		routes = append(routes, "PUT /dashboard/workFission/update")
	}
	if s.workFissionInvite != nil {
		routes = append(routes, "POST /dashboard/workFission/invite")
	}
	if s.workFissionInviteData != nil {
		routes = append(routes, "GET /dashboard/workFission/inviteData")
	}
	if s.workFissionInviteDetail != nil {
		routes = append(routes, "GET /dashboard/workFission/inviteDetail")
	}
	if s.workFissionDestroy != nil {
		routes = append(routes, "DELETE /dashboard/workFission/destroy")
	}
	if s.operationWorkFissionInviteFriends != nil {
		routes = append(routes, "GET /operation/workFission/inviteFriends")
	}
	if s.operationWorkFissionPoster != nil {
		routes = append(routes, "GET /operation/workFission/poster")
	}
	if s.operationWorkFissionTaskData != nil {
		routes = append(routes, "GET /operation/workFission/taskData")
	}
	if s.operationWorkFissionReceive != nil {
		routes = append(routes, "PUT /operation/workFission/receive")
	}
	if s.operationWorkFissionAuth != nil {
		routes = append(routes, "GET /operation/auth/workFission", "POST /operation/auth/workFission")
	}
	if s.operationWorkFissionOpenUserInfo != nil {
		routes = append(routes, "GET /operation/openUserInfo/workFission")
	}
	if s.operationLotteryContactData != nil {
		routes = append(routes, "POST /operation/lottery/contactData")
	}
	if s.operationLotteryContactLottery != nil {
		routes = append(routes, "PUT /operation/lottery/contactLottery")
	}
	if s.operationLotteryReceive != nil {
		routes = append(routes, "PUT /operation/lottery/receive")
	}
	if s.operationLotteryAuth != nil {
		routes = append(routes, "GET /operation/auth/lottery", "POST /operation/auth/lottery")
	}
	if s.operationLotteryOpenUserInfo != nil {
		routes = append(routes, "GET /operation/openUserInfo/lottery")
	}
	if s.operationRoomClockInContactData != nil {
		routes = append(routes, "GET /operation/roomClockIn/contactData")
	}
	if s.operationRoomClockInClockInRanking != nil {
		routes = append(routes, "GET /operation/roomClockIn/clockInRanking")
	}
	if s.operationRoomClockInContactClockIn != nil {
		routes = append(routes, "PUT /operation/roomClockIn/contactClockIn")
	}
	if s.operationRoomClockInReceive != nil {
		routes = append(routes, "PUT /operation/roomClockIn/receive")
	}
	if s.operationRoomClockInAuth != nil {
		routes = append(routes, "GET /operation/auth/roomClockIn", "POST /operation/auth/roomClockIn")
	}
	if s.operationRoomClockInOpenUserInfo != nil {
		routes = append(routes, "GET /operation/openUserInfo/roomClockIn")
	}
	if s.operationRoomFissionPoster != nil {
		routes = append(routes, "GET /operation/roomFission/poster")
	}
	if s.operationRoomFissionInviteFriends != nil {
		routes = append(routes, "GET /operation/roomFission/inviteFriends")
	}
	if s.operationRoomFissionReceive != nil {
		routes = append(routes, "GET /operation/roomFission/receive")
	}
	if s.operationRoomFissionAuth != nil {
		routes = append(routes, "GET /operation/auth/roomFission", "POST /operation/auth/roomFission")
	}
	if s.operationRoomFissionOpenUserInfo != nil {
		routes = append(routes, "GET /operation/openUserInfo/roomFission")
	}
	if s.operationRoomInfinitePullQRCode != nil {
		routes = append(routes, "GET /operation/roomInfinitePull/qrCode")
	}
	if s.operationShopCodeAreaCode != nil {
		routes = append(routes, "GET /operation/shopCode/areaCode")
	}
	if s.operationShopCodeWeChatSDKConfig != nil {
		routes = append(routes, "GET /operation/shopCode/weChatSdkConfig")
	}
	if s.operationShopCodeAuth != nil {
		routes = append(routes, "GET /operation/auth/shopCode", "POST /operation/auth/shopCode")
	}
	if s.operationShopCodeOpenUserInfo != nil {
		routes = append(routes, "GET /operation/openUserInfo/shopCode")
	}
	if s.workRoomGroupIndex != nil {
		routes = append(routes, "GET /dashboard/workRoomGroup/index")
	}
	if s.workRoomGroupStore != nil {
		routes = append(routes, "POST /dashboard/workRoomGroup/store")
	}
	if s.workRoomGroupUpdate != nil {
		routes = append(routes, "PUT /dashboard/workRoomGroup/update")
	}
	if s.workRoomGroupDestroy != nil {
		routes = append(routes, "DELETE /dashboard/workRoomGroup/destroy")
	}
	if s.sidebarContactTagAll != nil {
		routes = append(routes, "GET /sidebar/workContactTag/allTag")
	}
	if s.sidebarContactDetail != nil {
		routes = append(routes, "GET /sidebar/workContact/detail")
	}
	if s.sidebarContactShow != nil {
		routes = append(routes, "GET /sidebar/workContact/show")
	}
	if s.sidebarContactTrack != nil {
		routes = append(routes, "GET /sidebar/workContact/track")
	}
	if s.sidebarContactUpdate != nil {
		routes = append(routes, "PUT /sidebar/workContact/update")
	}
	if s.sidebarProcessStatus != nil {
		routes = append(routes, "GET /sidebar/contactProcessStatus/index")
	}
	if s.sidebarProcessUpdate != nil {
		routes = append(routes, "PUT /sidebar/contactProcessStatus/update")
	}
	if s.mediumIndex != nil {
		routes = append(routes, "GET /dashboard/medium/index")
	}
	if s.mediumShow != nil {
		routes = append(routes, "GET /dashboard/medium/show")
	}
	if s.mediumStore != nil {
		routes = append(routes, "POST /dashboard/medium/store")
	}
	if s.mediumUpdate != nil {
		routes = append(routes, "PUT /dashboard/medium/update")
	}
	if s.mediumDestroy != nil {
		routes = append(routes, "DELETE /dashboard/medium/destroy")
	}
	if s.mediumItemGroupUpdate != nil {
		routes = append(routes, "PUT /dashboard/medium/groupUpdate")
	}
	if s.mediumBatchGroupUpdate != nil {
		routes = append(routes, "POST /dashboard/medium/batchGroupUpdate")
	}
	if s.mediumReferenceCheck != nil {
		routes = append(routes, "POST /dashboard/medium/referenceCheck")
	}
	if s.mediumBatchDestroy != nil {
		routes = append(routes, "POST /dashboard/medium/batchDestroy")
	}
	if s.materialSelectorIndex != nil {
		routes = append(routes, "GET /dashboard/materialSelector/index")
	}
	if s.sidebarMediumIndex != nil {
		routes = append(routes, "GET /sidebar/medium/index")
	}
	if s.sidebarMediumMediaIDUpdate != nil {
		routes = append(routes, "GET /sidebar/medium/mediaIdUpdate")
	}
	if s.mediumGroupIndex != nil {
		routes = append(routes, "GET /dashboard/mediumGroup/index")
	}
	if s.friendsCircleTaskIndex != nil {
		routes = append(routes, "GET /dashboard/friendsCircle/taskIndex")
	}
	if s.friendsCircleMaterialIndex != nil {
		routes = append(routes, "GET /dashboard/friendsCircle/materialIndex")
	}
	if s.friendsCircleTaskStore != nil {
		routes = append(routes, "POST /dashboard/friendsCircle/taskStore")
	}
	if s.friendsCircleMaterialStore != nil {
		routes = append(routes, "POST /dashboard/friendsCircle/materialStore")
	}
	if s.friendsCirclePublish != nil {
		routes = append(routes, "POST /dashboard/friendsCircle/publish")
	}
	if s.friendsCircleTaskResultIndex != nil {
		routes = append(routes, "GET /dashboard/friendsCircle/taskResultIndex")
	}
	if s.friendsCircleExport != nil {
		routes = append(routes, "GET /dashboard/friendsCircle/export")
	}
	if s.friendsCircleExportData != nil {
		routes = append(routes, "GET /dashboard/friendsCircle/exportData")
	}
	if s.friendsCircleProviderCallback != nil {
		routes = append(routes, "POST /dashboard/friendsCircle/providerCallback")
	}
	if s.phase34AcquisitionLinkIndex != nil {
		routes = append(routes, "GET /dashboard/acquisitionLink/index")
	}
	if s.phase34AcquisitionLinkStore != nil {
		routes = append(routes, "POST /dashboard/acquisitionLink/store")
	}
	if s.phase34AcquisitionLinkAuthorize != nil {
		routes = append(routes, "POST /dashboard/acquisitionLink/authorize")
	}
	if s.phase34CustomerServiceIndex != nil {
		routes = append(routes, "GET /dashboard/customerService/index")
	}
	if s.phase34CustomerServiceStore != nil {
		routes = append(routes, "POST /dashboard/customerService/store")
	}
	if s.phase34CustomerServiceSync != nil {
		routes = append(routes, "POST /dashboard/customerService/sync")
	}
	if s.phase34ShortLinkIndex != nil {
		routes = append(routes, "GET /dashboard/liveCodeShortChain/index")
	}
	if s.phase34ShortLinkStore != nil {
		routes = append(routes, "POST /dashboard/liveCodeShortChain/store")
	}
	if s.phase34ShortLinkDisable != nil {
		routes = append(routes, "POST /dashboard/liveCodeShortChain/disable")
	}
	if s.phase34ShortLinkRedirect != nil {
		routes = append(routes, "GET /r/{token}")
	}
	if s.mediumGroupStore != nil {
		routes = append(routes, "POST /dashboard/mediumGroup/store")
	}
	if s.mediumGroupUpdate != nil {
		routes = append(routes, "PUT /dashboard/mediumGroup/update")
	}
	if s.mediumGroupDestroy != nil {
		routes = append(routes, "DELETE /dashboard/mediumGroup/destroy")
	}
	if s.sidebarMediumGroupIndex != nil {
		routes = append(routes, "GET /sidebar/mediumGroup/index")
	}
	if s.channelCodeIndex != nil {
		routes = append(routes, "GET /dashboard/channelCode/index")
	}
	if s.channelCodeShow != nil {
		routes = append(routes, "GET /dashboard/channelCode/show")
	}
	if s.channelCodeContact != nil {
		routes = append(routes, "GET /dashboard/channelCode/contact")
	}
	if s.channelCodeStatistics != nil {
		routes = append(routes, "GET /dashboard/channelCode/statistics")
	}
	if s.channelCodeStatsIndex != nil {
		routes = append(routes, "GET /dashboard/channelCode/statisticsIndex")
	}
	if s.channelCodeStore != nil {
		routes = append(routes, "POST /dashboard/channelCode/store")
	}
	if s.channelCodeUpdate != nil {
		routes = append(routes, "PUT /dashboard/channelCode/update")
	}
	if s.channelCodeGroupIndex != nil {
		routes = append(routes, "GET /dashboard/channelCodeGroup/index")
	}
	if s.channelCodeGroupDetail != nil {
		routes = append(routes, "GET /dashboard/channelCodeGroup/detail")
	}
	if s.channelCodeGroupStore != nil {
		routes = append(routes, "POST /dashboard/channelCodeGroup/store")
	}
	if s.channelCodeGroupUpdate != nil {
		routes = append(routes, "PUT /dashboard/channelCodeGroup/update")
	}
	if s.channelCodeGroupMove != nil {
		routes = append(routes, "PUT /dashboard/channelCodeGroup/move")
	}
	if s.greetingIndex != nil {
		routes = append(routes, "GET /dashboard/greeting/index")
	}
	if s.greetingShow != nil {
		routes = append(routes, "GET /dashboard/greeting/show")
	}
	if s.greetingStore != nil {
		routes = append(routes, "POST /dashboard/greeting/store")
	}
	if s.greetingUpdate != nil {
		routes = append(routes, "PUT /dashboard/greeting/update")
	}
	if s.greetingDestroy != nil {
		routes = append(routes, "DELETE /dashboard/greeting/destroy")
	}
	if s.roomWelcomeIndex != nil {
		routes = append(routes, "GET /dashboard/roomWelcome/index")
		routes = append(routes, "GET /dashboard/clockIn/index")
	}
	if s.roomWelcomeSelect != nil {
		routes = append(routes, "GET /dashboard/roomWelcome/select")
	}
	if s.roomWelcomeShow != nil {
		routes = append(routes, "GET /dashboard/roomWelcome/show")
	}
	if s.roomWelcomeStore != nil {
		routes = append(routes, "POST /dashboard/roomWelcome/store")
	}
	if s.roomWelcomeUpdate != nil {
		routes = append(routes, "PUT /dashboard/roomWelcome/update")
	}
	if s.roomWelcomeDestroy != nil {
		routes = append(routes, "DELETE /dashboard/roomWelcome/destroy")
	}
	if s.contactFieldIndex != nil {
		routes = append(routes, "GET /dashboard/contactField/index")
	}
	if s.contactFieldShow != nil {
		routes = append(routes, "GET /dashboard/contactField/show")
	}
	if s.contactFieldPortrait != nil {
		routes = append(routes, "GET /dashboard/contactField/portrait")
	}
	if s.contactFieldStore != nil {
		routes = append(routes, "POST /dashboard/contactField/store")
	}
	if s.contactFieldUpdate != nil {
		routes = append(routes, "PUT /dashboard/contactField/update")
	}
	if s.contactFieldStatus != nil {
		routes = append(routes, "PUT /dashboard/contactField/statusUpdate")
	}
	if s.contactFieldDestroy != nil {
		routes = append(routes, "DELETE /dashboard/contactField/destroy")
	}
	if s.contactFieldBatch != nil {
		routes = append(routes, "PUT /dashboard/contactField/batchUpdate")
	}
	if s.contactFieldPivot != nil {
		routes = append(routes, "GET /dashboard/contactFieldPivot/index")
	}
	if s.contactFieldPivotUpdate != nil {
		routes = append(routes, "PUT /dashboard/contactFieldPivot/update")
	}
	if s.contactBatchAddIndex != nil {
		routes = append(routes, "GET /dashboard/contactBatchAdd/index")
	}
	if s.contactBatchAddImportIndex != nil {
		routes = append(routes, "GET /dashboard/contactBatchAdd/importIndex")
	}
	if s.contactBatchAddImportStore != nil {
		routes = append(routes, "POST /dashboard/contactBatchAdd/importStore")
	}
	if s.contactBatchAddAllot != nil {
		routes = append(routes, "POST /dashboard/contactBatchAdd/allot")
	}
	if s.contactBatchAddDataStatistic != nil {
		routes = append(routes, "GET /dashboard/contactBatchAdd/dataStatistic")
	}
	if s.contactBatchAddDestroy != nil {
		routes = append(routes, "DELETE /dashboard/contactBatchAdd/destroy")
	}
	if s.contactBatchAddImportDestroy != nil {
		routes = append(routes, "DELETE /dashboard/contactBatchAdd/importDestroy")
	}
	if s.contactBatchAddSettingEdit != nil {
		routes = append(routes, "GET /dashboard/contactBatchAdd/settingEdit")
	}
	if s.contactBatchAddSettingUpdate != nil {
		routes = append(routes, "POST /dashboard/contactBatchAdd/settingUpdate")
	}
	if s.contactBatchAddRemind != nil {
		routes = append(routes, "GET /dashboard/contactBatchAdd/remind")
		routes = append(routes, "POST /dashboard/contactBatchAdd/remind")
	}
	if s.sensitiveWordsPage != nil {
		routes = append(routes, "GET /dashboard/sensitiveWords/page", "HEAD /dashboard/sensitiveWords/page")
	}
	if s.sensitiveWordIndex != nil {
		routes = append(routes, "GET /dashboard/sensitiveWord/index")
	}
	if s.sensitiveWordStore != nil {
		routes = append(routes, "POST /dashboard/sensitiveWord/store")
	}
	if s.sensitiveWordDestroy != nil {
		routes = append(routes, "DELETE /dashboard/sensitiveWord/destroy")
	}
	if s.sensitiveWordStatusUpdate != nil {
		routes = append(routes, "PUT /dashboard/sensitiveWord/statusUpdate")
	}
	if s.sensitiveWordMove != nil {
		routes = append(routes, "PUT /dashboard/sensitiveWord/move")
	}
	if s.sensitiveWordGroupSelect != nil {
		routes = append(routes, "GET /dashboard/sensitiveWordGroup/select")
	}
	if s.sensitiveWordGroupStore != nil {
		routes = append(routes, "POST /dashboard/sensitiveWordGroup/store")
	}
	if s.sensitiveWordGroupUpdate != nil {
		routes = append(routes, "PUT /dashboard/sensitiveWordGroup/update")
	}
	if s.sensitiveWordsMonitorIndex != nil {
		routes = append(routes, "GET /dashboard/sensitiveWordsMonitor/index")
	}
	if s.sensitiveWordsMonitorShow != nil {
		routes = append(routes, "GET /dashboard/sensitiveWordsMonitor/show")
	}
	if s.contactSOPIndex != nil {
		routes = append(routes, "GET /dashboard/contactSop/index")
	}
	if s.contactSOPStore != nil {
		routes = append(routes, "POST /dashboard/contactSop/store")
	}
	if s.contactSOPSetEmployee != nil {
		routes = append(routes, "PUT /dashboard/contactSop/setEmployee")
	}
	if s.contactSOPState != nil {
		routes = append(routes, "PUT /dashboard/contactSop/state")
	}
	if s.contactSOPInfo != nil {
		routes = append(routes, "GET /dashboard/contactSop/info")
	}
	if s.contactSOPDestroy != nil {
		routes = append(routes, "DELETE /dashboard/contactSop/destroy")
	}
	if s.contactSOPUpdate != nil {
		routes = append(routes, "PUT /dashboard/contactSop/update")
	}
	if s.contactSOPPage != nil {
		routes = append(routes, "GET /dashboard/contactSop/page", "HEAD /dashboard/contactSop/page")
	}
	if s.roomSOPPage != nil {
		routes = append(routes, "GET /dashboard/roomSop/page", "HEAD /dashboard/roomSop/page")
	}
	if s.roomSOPIndex != nil {
		routes = append(routes, "GET /dashboard/roomSop/index")
	}
	if s.roomSOPStore != nil {
		routes = append(routes, "POST /dashboard/roomSop/store")
	}
	if s.roomSOPSetRoom != nil {
		routes = append(routes, "PUT /dashboard/roomSop/setRoom")
	}
	if s.roomSOPState != nil {
		routes = append(routes, "PUT /dashboard/roomSop/state")
	}
	if s.roomSOPInfo != nil {
		routes = append(routes, "GET /dashboard/roomSop/info")
	}
	if s.roomSOPDestroy != nil {
		routes = append(routes, "DELETE /dashboard/roomSop/destroy")
	}
	if s.roomSOPUpdate != nil {
		routes = append(routes, "PUT /dashboard/roomSop/update")
	}
	if s.shopCodePage != nil {
		routes = append(routes, "GET /dashboard/shopCode/page", "HEAD /dashboard/shopCode/page")
	}
	if s.shopCodeLocation != nil {
		routes = append(routes, "GET /dashboard/shopCode/location")
	}
	if s.shopCodeAddressKeyWordList != nil {
		routes = append(routes, "GET /dashboard/shopCode/addressKeyWordList")
	}
	if s.shopCodeStore != nil {
		routes = append(routes, "POST /dashboard/shopCode/store")
	}
	if s.shopCodeUpdate != nil {
		routes = append(routes, "PUT /dashboard/shopCode/update")
	}
	if s.shopCodeDestroy != nil {
		routes = append(routes, "DELETE /dashboard/shopCode/destroy")
	}
	if s.shopCodeInfo != nil {
		routes = append(routes, "GET /dashboard/shopCode/info")
	}
	if s.shopCodeStatus != nil {
		routes = append(routes, "PUT /dashboard/shopCode/status")
	}
	if s.shopCodeIndex != nil {
		routes = append(routes, "GET /dashboard/shopCode/index")
	}
	if s.shopCodeSearchCity != nil {
		routes = append(routes, "GET /dashboard/shopCode/searchCity")
	}
	if s.shopCodeShare != nil {
		routes = append(routes, "GET /dashboard/shopCode/share")
	}
	if s.shopCodePageInfo != nil {
		routes = append(routes, "GET /dashboard/shopCode/pageInfo")
	}
	if s.shopCodePageSet != nil {
		routes = append(routes, "POST /dashboard/shopCode/pageSet")
	}
	if s.shopCodeShow != nil {
		routes = append(routes, "GET /dashboard/shopCode/show")
	}
	if s.shopCodeShowContact != nil {
		routes = append(routes, "GET /dashboard/shopCode/showContact")
	}
	if s.shopCodeShowShop != nil {
		routes = append(routes, "GET /dashboard/shopCode/showShop")
	}
	if s.shopCodeUpdateEmployee != nil {
		routes = append(routes, "POST /dashboard/shopCode/updateEmployee")
	}
	if s.shopCodeUpdateQRCode != nil {
		routes = append(routes, "POST /dashboard/shopCode/updateQrcode")
	}
	if s.shopCodeBatchContactTags != nil {
		routes = append(routes, "PUT /dashboard/shopCode/batchContactTags")
	}
	if s.radarPage != nil {
		routes = append(routes, "GET /dashboard/radar/page", "HEAD /dashboard/radar/page")
	}
	if s.radarStore != nil {
		routes = append(routes, "POST /dashboard/radar/store")
	}
	if s.radarUpdate != nil {
		routes = append(routes, "PUT /dashboard/radar/update")
	}
	if s.radarIndex != nil {
		routes = append(routes, "GET /dashboard/radar/index")
	}
	if s.radarDestroy != nil {
		routes = append(routes, "DELETE /dashboard/radar/destroy")
	}
	if s.radarInfo != nil {
		routes = append(routes, "GET /dashboard/radar/info")
	}
	if s.radarStoreChannel != nil {
		routes = append(routes, "POST /dashboard/radar/storeChannel")
	}
	if s.radarStoreChannelLink != nil {
		routes = append(routes, "POST /dashboard/radar/storeChannelLink")
	}
	if s.radarIndexChannel != nil {
		routes = append(routes, "GET /dashboard/radar/indexChannel")
	}
	if s.radarIndexChannelLink != nil {
		routes = append(routes, "GET /dashboard/radar/indexChannelLink")
	}
	if s.radarShow != nil {
		routes = append(routes, "GET /dashboard/radar/show")
	}
	if s.radarShowContact != nil {
		routes = append(routes, "GET /dashboard/radar/showContact")
	}
	if s.radarShowChannel != nil {
		routes = append(routes, "GET /dashboard/radar/showChannel")
	}
	if s.radarArticle != nil {
		routes = append(routes, "GET /dashboard/radar/radarArticle")
	}
	if s.autoTagStore != nil {
		routes = append(routes, "POST /dashboard/autoTag/store")
	}
	if s.autoTagIndex != nil {
		routes = append(routes, "GET /dashboard/autoTag/index")
	}
	if s.autoTagDestroy != nil {
		routes = append(routes, "DELETE /dashboard/autoTag/destroy")
	}
	if s.autoTagOnOff != nil {
		routes = append(routes, "PUT /dashboard/autoTag/onOff")
	}
	if s.autoTagShow != nil {
		routes = append(routes, "GET /dashboard/autoTag/show")
	}
	if s.autoTagShowContactKeyWord != nil {
		routes = append(routes, "GET /dashboard/autoTag/showContactKeyWord")
	}
	if s.autoTagKeyWordTag != nil {
		routes = append(routes, "GET /Task/AutoTag/KeyWordTag")
		routes = append(routes, "GET /dashboard/Task/AutoTag/KeyWordTag")
	}
	if s.autoTagShowContactRoom != nil {
		routes = append(routes, "GET /dashboard/autoTag/showContactRoom")
	}
	if s.autoTagShowContactTime != nil {
		routes = append(routes, "GET /dashboard/autoTag/showContactTime")
	}
	if s.workMessageFromUsers != nil {
		routes = append(routes, "GET /dashboard/workMessage/fromUsers")
	}
	if s.workMessageToUsers != nil {
		routes = append(routes, "GET /dashboard/workMessage/toUsers")
	}
	if s.workMessageGlobalOverview != nil {
		routes = append(routes, "GET /dashboard/workMessage/globalOverview")
	}
	if s.workMessageFocus != nil {
		routes = append(routes, "PUT|DELETE /dashboard/workMessage/focus")
	}
	if s.workMessageStaffDirectory != nil {
		routes = append(routes, "GET /dashboard/workMessage/staffDirectory")
	}
	if s.workMessageStaffDetail != nil {
		routes = append(routes, "GET /dashboard/workMessage/staffDetail")
	}
	if s.workMessageTrajectoryDay != nil {
		routes = append(routes, "GET /dashboard/workMessage/trajectoryDay")
	}
	if s.workMessageCustomerDirectory != nil {
		routes = append(routes, "GET /dashboard/workMessage/customerDirectory")
	}
	if s.workMessageCustomerConversations != nil {
		routes = append(routes, "GET /dashboard/workMessage/customerConversations")
	}
	if s.workMessageCustomerDetail != nil {
		routes = append(routes, "GET /dashboard/workMessage/customerDetail")
	}
	if s.workMessageRoomDirectory != nil {
		routes = append(routes, "GET /dashboard/workMessage/roomDirectory")
	}
	if s.workMessageRoomProfile != nil {
		routes = append(routes, "GET /dashboard/workMessage/roomProfile")
	}
	if s.workMessageRoomMessages != nil {
		routes = append(routes, "GET /dashboard/workMessage/roomMessages")
	}
	if s.workMessageRoomMembers != nil {
		routes = append(routes, "GET /dashboard/workMessage/roomMembers")
	}
	if s.workMessageRoomFilterOptions != nil {
		routes = append(routes, "GET /dashboard/workMessage/roomFilterOptions")
	}
	if s.workMessageIndex != nil {
		routes = append(routes, "GET /dashboard/workMessage/index", "GET /dashboard/workMessage/detail")
	}
	if s.workMessageConfigCorpStore != nil {
		routes = append(routes, "POST /dashboard/workMessageConfig/corpStore")
	}
	if s.workMessageConfigCorpShow != nil {
		routes = append(routes, "GET /dashboard/workMessageConfig/corpShow")
	}
	if s.workMessageConfigCorpIndex != nil {
		routes = append(routes, "GET /dashboard/workMessageConfig/corpIndex")
	}
	if s.workMessageConfigStepCreate != nil {
		routes = append(routes, "GET /dashboard/workMessageConfig/stepCreate")
	}
	if s.workMessageConfigStepUpdate != nil {
		routes = append(routes, "PUT /dashboard/workMessageConfig/stepUpdate")
	}
	if s.lotteryPage != nil {
		routes = append(routes, "GET /dashboard/lottery/page", "HEAD /dashboard/lottery/page")
	}
	if s.lotteryIndex != nil {
		routes = append(routes, "GET /dashboard/lottery/index")
	}
	if s.lotteryStore != nil {
		routes = append(routes, "POST /dashboard/lottery/store")
	}
	if s.lotteryShowContact != nil {
		routes = append(routes, "GET /dashboard/lottery/showContact")
	}
	if s.lotteryShow != nil {
		routes = append(routes, "GET /dashboard/lottery/show")
	}
	if s.lotteryDestroy != nil {
		routes = append(routes, "DELETE /dashboard/lottery/destroy")
	}
	if s.lotteryShare != nil {
		routes = append(routes, "GET /dashboard/lottery/share")
	}
	if s.lotteryUpdate != nil {
		routes = append(routes, "PUT /dashboard/lottery/update")
	}
	if s.lotteryInfo != nil {
		routes = append(routes, "GET /dashboard/lottery/info")
	}
	if s.lotteryWriteOff != nil {
		routes = append(routes, "GET /dashboard/lottery/writeOff")
	}
	if s.lotteryBatchContactTags != nil {
		routes = append(routes, "PUT /dashboard/lottery/batchContactTags")
	}
	if s.roomFissionPage != nil {
		routes = append(routes, "GET /dashboard/roomFission/page", "HEAD /dashboard/roomFission/page")
	}
	if s.roomFissionIndex != nil {
		routes = append(routes, "GET /dashboard/roomFission/index")
	}
	if s.roomFissionStore != nil {
		routes = append(routes, "POST /dashboard/roomFission/store")
	}
	if s.roomFissionInfo != nil {
		routes = append(routes, "GET /dashboard/roomFission/info")
	}
	if s.roomFissionUpdate != nil {
		routes = append(routes, "PUT /dashboard/roomFission/update")
	}
	if s.roomFissionDestroy != nil {
		routes = append(routes, "DELETE /dashboard/roomFission/destroy")
	}
	if s.roomFissionInvite != nil {
		routes = append(routes, "POST /dashboard/roomFission/invite")
	}
	if s.roomFissionShow != nil {
		routes = append(routes, "GET /dashboard/roomFission/show")
	}
	if s.roomFissionShowRoom != nil {
		routes = append(routes, "GET /dashboard/roomFission/showRoom")
	}
	if s.roomFissionShowContact != nil {
		routes = append(routes, "GET /dashboard/roomFission/showContact")
	}
	if s.roomFissionWriteOff != nil {
		routes = append(routes, "GET /dashboard/roomFission/writeOff")
	}
	if s.roomClockInPage != nil {
		routes = append(routes, "GET /dashboard/roomClockIn/page", "HEAD /dashboard/roomClockIn/page")
	}
	if s.roomClockInIndex != nil {
		routes = append(routes, "GET /dashboard/roomClockIn/index")
	}
	if s.roomClockInStore != nil {
		routes = append(routes, "POST /dashboard/roomClockIn/store")
	}
	if s.roomClockInUpdate != nil {
		routes = append(routes, "PUT /dashboard/roomClockIn/update")
	}
	if s.roomClockInDestroy != nil {
		routes = append(routes, "DELETE /dashboard/roomClockIn/destroy")
	}
	if s.roomClockInShow != nil {
		routes = append(routes, "GET /dashboard/roomClockIn/show")
	}
	if s.roomClockInShowContact != nil {
		routes = append(routes, "GET /dashboard/roomClockIn/showContact")
	}
	if s.roomClockInBatchContactTags != nil {
		routes = append(routes, "PUT /dashboard/roomClockIn/batchContactTags")
	}
	if s.roomClockInInfo != nil {
		routes = append(routes, "GET /dashboard/roomClockIn/info")
	}
	if s.roomClockInDayDetail != nil {
		routes = append(routes, "GET /dashboard/roomClockIn/dayDetail")
	}
	if s.roomQualityPage != nil {
		routes = append(routes, "GET /dashboard/roomQuality/page", "HEAD /dashboard/roomQuality/page")
	}
	if s.roomQualityIndex != nil {
		routes = append(routes, "GET /dashboard/roomQuality/index")
	}
	if s.roomQualityStore != nil {
		routes = append(routes, "POST /dashboard/roomQuality/store")
	}
	if s.roomQualityStatus != nil {
		routes = append(routes, "PUT /dashboard/roomQuality/status")
	}
	if s.roomQualityInfo != nil {
		routes = append(routes, "GET /dashboard/roomQuality/info")
	}
	if s.roomQualityUpdate != nil {
		routes = append(routes, "PUT /dashboard/roomQuality/update")
	}
	if s.roomQualityShowContact != nil {
		routes = append(routes, "GET /dashboard/roomQuality/showContact")
	}
	if s.roomQualityDestroy != nil {
		routes = append(routes, "DELETE /dashboard/roomQuality/destroy")
	}
	if s.roomQualityContactDetail != nil {
		routes = append(routes, "GET /dashboard/roomQuality/contactDetail")
	}
	if s.roomCalendarPage != nil {
		routes = append(routes, "GET /dashboard/roomCalendar/page", "HEAD /dashboard/roomCalendar/page")
	}
	if s.roomCalendarIndex != nil {
		routes = append(routes, "GET /dashboard/roomCalendar/index")
	}
	if s.roomCalendarAddRoom != nil {
		routes = append(routes, "POST /dashboard/roomCalendar/addRoom")
	}
	if s.roomCalendarDestroyRoom != nil {
		routes = append(routes, "DELETE /dashboard/roomCalendar/destroyRoom")
	}
	if s.roomCalendarStore != nil {
		routes = append(routes, "POST /dashboard/roomCalendar/store")
	}
	if s.roomCalendarDestroy != nil {
		routes = append(routes, "DELETE /dashboard/roomCalendar/destroy")
	}
	if s.roomCalendarShow != nil {
		routes = append(routes, "GET /dashboard/roomCalendar/show")
	}
	if s.roomCalendarUpdate != nil {
		routes = append(routes, "PUT /dashboard/roomCalendar/update")
	}
	if s.roomRemindPage != nil {
		routes = append(routes, "GET /dashboard/roomRemind/page", "HEAD /dashboard/roomRemind/page")
	}
	if s.roomRemindIndex != nil {
		routes = append(routes, "GET /dashboard/roomRemind/index")
	}
	if s.roomRemindDestroy != nil {
		routes = append(routes, "DELETE /dashboard/roomRemind/destroy")
	}
	if s.roomRemindInfo != nil {
		routes = append(routes, "GET /dashboard/roomRemind/info")
	}
	if s.roomRemindStatus != nil {
		routes = append(routes, "GET /dashboard/roomRemind/status")
	}
	if s.roomRemindStore != nil {
		routes = append(routes, "POST /dashboard/roomRemind/store")
	}
	if s.roomRemindUpdate != nil {
		routes = append(routes, "PUT /dashboard/roomRemind/update")
	}
	if s.roomRemindTask != nil {
		routes = append(routes, "GET /dashboard/task/roomRemind")
	}
	if s.roomInfinitePullPage != nil {
		routes = append(routes, "GET /dashboard/roomInfinitePull/page", "HEAD /dashboard/roomInfinitePull/page")
	}
	if s.roomInfinitePullIndex != nil {
		routes = append(routes, "GET /dashboard/roomInfinitePull/index")
	}
	if s.roomInfinitePullInfo != nil {
		routes = append(routes, "GET /dashboard/roomInfinitePull/info")
	}
	if s.roomInfinitePullUpdate != nil {
		routes = append(routes, "PUT /dashboard/roomInfinitePull/update")
	}
	if s.roomInfinitePullDestroy != nil {
		routes = append(routes, "DELETE /dashboard/roomInfinitePull/destroy")
	}
	if s.roomInfinitePullStore != nil {
		routes = append(routes, "POST /dashboard/roomInfinitePull/store")
	}
	if s.sidebarFieldPivot != nil {
		routes = append(routes, "GET /sidebar/contactFieldPivot/index")
	}
	if s.sidebarFieldPivotUpdate != nil {
		routes = append(routes, "PUT /sidebar/contactFieldPivot/update")
	}
	if s.sidebarContactBatchAddDetail != nil {
		routes = append(routes, "GET /sidebar/contactBatchAdd/detail")
	}
	if s.sidebarContactSOPInfo != nil {
		routes = append(routes, "GET /sidebar/contactSop/getSopInfo")
	}
	if s.sidebarContactSOPTipInfo != nil {
		routes = append(routes, "GET /sidebar/contactSop/getSopTipInfo")
	}
	if s.sidebarRoomSOPInfo != nil {
		routes = append(routes, "GET /sidebar/roomSop/getSopInfo")
	}
	if s.sidebarRoomSOPLogState != nil {
		routes = append(routes, "PUT /sidebar/roomSop/logState")
	}
	if s.saasAlertPage != nil {
		routes = append(routes, "GET /dashboard/saasAlert/page", "HEAD /dashboard/saasAlert/page")
	}
	if s.saasAlertIndex != nil {
		routes = append(routes, "GET /dashboard/saasAlert/index")
	}
	if s.saasAlertResolve != nil {
		routes = append(routes, "PUT /dashboard/saasAlert/resolve", "POST /dashboard/saasAlert/resolve")
	}
	if s.saasAlertSetting != nil {
		routes = append(routes, "GET /dashboard/saasAlert/setting", "PUT /dashboard/saasAlert/setting", "POST /dashboard/saasAlert/setting")
	}
	if s.saasBillingPage != nil {
		routes = append(routes, "GET /dashboard/saasBilling/page", "HEAD /dashboard/saasBilling/page")
	}
	if s.saasBillingSummary != nil {
		routes = append(routes, "GET /dashboard/saasBilling/summary")
	}
	if s.saasBillingPaymentOrders != nil {
		routes = append(routes, "GET /dashboard/saasBilling/paymentOrders")
	}
	if s.saasBillingPaymentRefunds != nil {
		routes = append(routes, "GET /dashboard/saasBilling/paymentRefunds")
	}
	if s.saasBillingInvoiceProfile != nil {
		routes = append(routes, "GET /dashboard/saasBilling/invoiceProfile", "POST /dashboard/saasBilling/invoiceProfile", "PUT /dashboard/saasBilling/invoiceProfile")
	}
	if s.saasBillingInvoices != nil {
		routes = append(routes, "GET /dashboard/saasBilling/invoices")
	}
	if s.saasBillingInvoice != nil {
		routes = append(routes, "POST /dashboard/saasBilling/invoice", "PUT /dashboard/saasBilling/invoice")
	}
	if s.saasBillingInvoiceCancel != nil {
		routes = append(routes, "POST /dashboard/saasBilling/invoiceCancel", "PUT /dashboard/saasBilling/invoiceCancel")
	}
	if s.saasAdminPage != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/page", "HEAD /dashboard/saasAdmin/page")
	}
	if s.saasAdminOverview != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/overview")
	}
	if s.saasAdminTenantReadiness != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/tenantReadiness")
	}
	if s.saasAdminTenant != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/tenant")
	}
	if s.saasAdminTenantLifecycle != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/tenantLifecycle")
	}
	if s.saasAdminUsage != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/usage")
	}
	if s.saasAdminRisk != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/risk")
	}
	if s.saasAdminBusinessMetrics != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/businessMetrics")
	}
	if s.saasAdminBusinessTrends != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/businessTrends")
	}
	if s.saasAdminOperationQueue != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/operationQueue")
	}
	if s.saasAdminOperationQueueOwners != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/operationQueueOwners")
	}
	if s.saasAdminOperationQueueAssignments != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/operationQueueAssignments")
	}
	if s.saasAdminOperationQueueAssignmentClose != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/operationQueueAssignmentClose", "PUT /dashboard/saasAdmin/operationQueueAssignmentClose")
	}
	if s.saasAdminOperationQueueAssignmentNotifications != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/operationQueueAssignmentNotifications", "PUT /dashboard/saasAdmin/operationQueueAssignmentNotifications")
	}
	if s.saasAdminOperationQueueAssign != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/operationQueueAssign", "PUT /dashboard/saasAdmin/operationQueueAssign")
	}
	if s.saasAdminRenewalForecast != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/renewalForecast")
	}
	if s.saasAdminRenewalForecastTasks != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/renewalForecastTasks", "PUT /dashboard/saasAdmin/renewalForecastTasks")
	}
	if s.saasAdminRenewalForecastAssign != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/renewalForecastAssign", "PUT /dashboard/saasAdmin/renewalForecastAssign")
	}
	if s.saasAdminRenewalForecastNotifications != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/renewalForecastNotifications", "PUT /dashboard/saasAdmin/renewalForecastNotifications")
	}
	if s.saasAdminCustomerSuccess != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/customerSuccess")
	}
	if s.saasAdminCustomerSuccessOwners != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/customerSuccessOwners")
	}
	if s.saasAdminCustomerSuccessAssign != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/customerSuccessAssign", "PUT /dashboard/saasAdmin/customerSuccessAssign")
	}
	if s.saasAdminCustomerSuccessRenewalTasks != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/customerSuccessRenewalTasks", "PUT /dashboard/saasAdmin/customerSuccessRenewalTasks")
	}
	if s.saasAdminCustomerSuccessRenewalNotifications != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/customerSuccessRenewalNotifications", "PUT /dashboard/saasAdmin/customerSuccessRenewalNotifications")
	}
	if s.saasAdminRiskFollowUp != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/riskFollowUp", "PUT /dashboard/saasAdmin/riskFollowUp")
	}
	if s.saasAdminRiskFollowUps != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/riskFollowUps")
	}
	if s.saasAdminRiskFollowUpOwners != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/riskFollowUpOwners")
	}
	if s.saasAdminRiskFollowUpBulkClose != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/riskFollowUpBulkClose", "PUT /dashboard/saasAdmin/riskFollowUpBulkClose")
	}
	if s.saasAdminAlerts != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/alerts")
	}
	if s.saasAdminAlertResolve != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/alertResolve", "PUT /dashboard/saasAdmin/alertResolve")
	}
	if s.saasAdminAlertBulkResolve != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/alertBulkResolve", "PUT /dashboard/saasAdmin/alertBulkResolve")
	}
	if s.saasAdminNotifications != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/notifications")
	}
	if s.saasAdminNotificationHealth != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/notificationHealth")
	}
	if s.saasAdminNotificationSLO != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/notificationSlo")
	}
	if s.saasAdminNotificationHealthRecovery != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/notificationHealthRecovery", "PUT /dashboard/saasAdmin/notificationHealthRecovery")
	}
	if s.saasAdminNotificationPolicies != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/notificationPolicies")
	}
	if s.saasAdminNotificationPolicy != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/notificationPolicy", "POST /dashboard/saasAdmin/notificationPolicy", "PUT /dashboard/saasAdmin/notificationPolicy")
	}
	if s.saasAdminNotificationPolicyTest != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/notificationPolicyTest", "PUT /dashboard/saasAdmin/notificationPolicyTest")
	}
	if s.saasAdminNotificationCredentialRotation != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/notificationCredentialRotation", "PUT /dashboard/saasAdmin/notificationCredentialRotation")
	}
	if s.saasAdminWeComCredentialProtection != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/wecomCredentialProtection")
	}
	if s.saasAdminWeComCredentialRotation != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/wecomCredentialRotation", "PUT /dashboard/saasAdmin/wecomCredentialRotation")
	}
	if s.saasAdminWeChatOpenCredentialProtection != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/wechatOpenCredentialProtection")
	}
	if s.saasAdminWeChatOpenCredentialRotation != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/wechatOpenCredentialRotation", "PUT /dashboard/saasAdmin/wechatOpenCredentialRotation")
	}
	if s.saasAdminNotificationRetry != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/notificationRetry", "PUT /dashboard/saasAdmin/notificationRetry")
	}
	if s.saasAdminNotificationBulkRetry != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/notificationBulkRetry", "PUT /dashboard/saasAdmin/notificationBulkRetry")
	}
	if s.saasAdminNotificationClose != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/notificationClose", "PUT /dashboard/saasAdmin/notificationClose")
	}
	if s.saasAdminNotificationBulkClose != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/notificationBulkClose", "PUT /dashboard/saasAdmin/notificationBulkClose")
	}
	if s.saasAdminPackages != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/packages")
	}
	if s.saasAdminSubscriptions != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/subscriptions")
	}
	if s.saasAdminSubscriptionEvents != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/subscriptionEvents")
	}
	if s.saasAdminSubscriptionTransition != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/subscriptionTransition", "PUT /dashboard/saasAdmin/subscriptionTransition")
	}
	if s.saasAdminSubscriptionReconcile != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/subscriptionReconcile", "PUT /dashboard/saasAdmin/subscriptionReconcile")
	}
	if s.saasAdminPaymentOrders != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/paymentOrders")
	}
	if s.saasAdminPaymentWebhookEvents != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/paymentWebhookEvents")
	}
	if s.saasAdminPaymentOrder != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/paymentOrder", "PUT /dashboard/saasAdmin/paymentOrder")
	}
	if s.saasAdminPaymentOrderCancel != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/paymentOrderCancel", "PUT /dashboard/saasAdmin/paymentOrderCancel")
	}
	if s.saasAdminPaymentDunning != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/paymentDunning", "PUT /dashboard/saasAdmin/paymentDunning")
	}
	if s.saasAdminPaymentRefunds != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/paymentRefunds")
	}
	if s.saasAdminPaymentRefund != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/paymentRefund", "PUT /dashboard/saasAdmin/paymentRefund")
	}
	if s.saasAdminPaymentRefundCancel != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/paymentRefundCancel", "PUT /dashboard/saasAdmin/paymentRefundCancel")
	}
	if s.saasAdminPaymentSettlementBatches != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/paymentSettlementBatches")
	}
	if s.saasAdminPaymentSettlementEntries != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/paymentSettlementEntries")
	}
	if s.saasAdminPaymentSettlementImport != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/paymentSettlementImport", "PUT /dashboard/saasAdmin/paymentSettlementImport")
	}
	if s.saasAdminPaymentSettlementReconcile != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/paymentSettlementReconcile", "PUT /dashboard/saasAdmin/paymentSettlementReconcile")
	}
	if s.saasAdminPaymentSettlementResolve != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/paymentSettlementResolve", "PUT /dashboard/saasAdmin/paymentSettlementResolve")
	}
	if s.saasAdminPaymentSettlementTransition != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/paymentSettlementTransition", "PUT /dashboard/saasAdmin/paymentSettlementTransition")
	}
	if s.saasAdminPaymentSettlementSyncRuns != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/paymentSettlementSyncRuns")
	}
	if s.saasAdminPaymentSettlementSync != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/paymentSettlementSync", "PUT /dashboard/saasAdmin/paymentSettlementSync")
	}
	if s.saasAdminAccessProfile != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/accessProfile")
	}
	if s.saasAdminAccessRoles != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/accessRoles")
	}
	if s.saasAdminAccessRole != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/accessRole", "PUT /dashboard/saasAdmin/accessRole")
	}
	if s.saasAdminAccessAssignments != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/accessAssignments")
	}
	if s.saasAdminAccessAssignment != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/accessAssignment", "PUT /dashboard/saasAdmin/accessAssignment")
	}
	if s.saasAdminSystemHealth != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/systemHealth")
	}
	if s.saasAdminSystemHealthScans != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/systemHealthScans")
	}
	if s.saasAdminSystemIncidents != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/systemIncidents")
	}
	if s.saasAdminSystemHealthScan != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/systemHealthScan", "PUT /dashboard/saasAdmin/systemHealthScan")
	}
	if s.saasAdminSystemIncident != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/systemIncident", "PUT /dashboard/saasAdmin/systemIncident")
	}
	if s.saasAdminServiceAccounts != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/serviceAccounts")
	}
	if s.saasAdminServiceAccountUsage != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/serviceAccountUsage")
	}
	if s.saasAdminServiceAccountUsageAlertEvaluate != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/serviceAccountUsageAlertEvaluate", "PUT /dashboard/saasAdmin/serviceAccountUsageAlertEvaluate")
	}
	if s.saasAdminServiceAccount != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/serviceAccount", "PUT /dashboard/saasAdmin/serviceAccount")
	}
	if s.saasAdminServiceAccountKeyRotate != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/serviceAccountKeyRotate", "PUT /dashboard/saasAdmin/serviceAccountKeyRotate")
	}
	if s.saasAdminServiceAccountKeyRevoke != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/serviceAccountKeyRevoke", "PUT /dashboard/saasAdmin/serviceAccountKeyRevoke")
	}
	if s.saasAdminAuditIntegrity != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/auditIntegrity")
	}
	if s.saasAdminAuditIntegrityVerify != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/auditIntegrityVerify", "PUT /dashboard/saasAdmin/auditIntegrityVerify")
	}
	if s.saasAdminAuditAnchors != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/auditAnchors")
	}
	if s.saasAdminAuditAnchor != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/auditAnchor", "PUT /dashboard/saasAdmin/auditAnchor")
	}
	if s.saasAdminBackupOverview != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/backupOverview")
	}
	if s.saasAdminBackupPolicy != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/backupPolicy", "PUT /dashboard/saasAdmin/backupPolicy")
	}
	if s.saasAdminBackupRun != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/backupRun", "PUT /dashboard/saasAdmin/backupRun")
	}
	if s.saasAdminRestoreDrill != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/restoreDrill", "PUT /dashboard/saasAdmin/restoreDrill")
	}
	if s.saasAdminComplianceOverview != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/complianceOverview")
	}
	if s.saasAdminCompliancePolicy != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/compliancePolicy", "PUT /dashboard/saasAdmin/compliancePolicy")
	}
	if s.saasAdminComplianceLegalHold != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/complianceLegalHold", "PUT /dashboard/saasAdmin/complianceLegalHold")
	}
	if s.saasAdminComplianceExport != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/complianceExport", "PUT /dashboard/saasAdmin/complianceExport")
	}
	if s.saasAdminComplianceExportDownload != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/complianceExportDownload")
	}
	if s.saasAdminComplianceErasure != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/complianceErasure", "PUT /dashboard/saasAdmin/complianceErasure")
	}
	if s.saasAdminComplianceErasureSteps != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/complianceErasureSteps")
	}
	if s.saasAdminIdentityOverview != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/identityOverview")
	}
	if s.saasAdminIdentityPolicy != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/identityPolicy", "PUT /dashboard/saasAdmin/identityPolicy")
	}
	if s.saasAdminIdentitySessions != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/identitySessions")
	}
	if s.saasAdminIdentitySession != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/identitySession", "PUT /dashboard/saasAdmin/identitySession")
	}
	if s.saasAdminIdentityLoginEvents != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/identityLoginEvents")
	}
	if s.saasAdminIdentityIncidents != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/identityIncidents")
	}
	if s.saasAdminIdentityIncident != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/identityIncident", "PUT /dashboard/saasAdmin/identityIncident")
	}
	if s.saasAdminIdentityUser != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/identityUser", "PUT /dashboard/saasAdmin/identityUser")
	}
	if s.saasAdminIdentityMFA != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/identityMFA", "PUT /dashboard/saasAdmin/identityMFA")
	}
	if s.saasAdminBrandingProfiles != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/brandingProfiles")
	}
	if s.saasAdminBrandingProfile != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/brandingProfile", "POST /dashboard/saasAdmin/brandingProfile", "PUT /dashboard/saasAdmin/brandingProfile")
	}
	if s.saasAdminTenantDomains != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/tenantDomains")
	}
	if s.saasAdminTenantDomain != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/tenantDomain", "PUT /dashboard/saasAdmin/tenantDomain")
	}
	if s.saasAdminTenantDomainDeliveryJobs != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/tenantDomainDeliveryJobs")
	}
	if s.saasAdminTenantDomainDelivery != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/tenantDomainDelivery", "PUT /dashboard/saasAdmin/tenantDomainDelivery")
	}
	if s.saasAdminReleaseReadiness != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/releaseReadiness")
	}
	if s.saasAdminReleaseEvidence != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/releaseEvidence", "PUT /dashboard/saasAdmin/releaseEvidence")
	}
	if s.saasAdminReleaseEvidenceAction != nil {
		routes = append(routes, "PUT /dashboard/saasAdmin/releaseEvidenceAction")
	}
	if s.saasAdminReleaseCandidate != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/releaseCandidate", "PUT /dashboard/saasAdmin/releaseCandidate")
	}
	if s.saasServiceAccountWhoAmI != nil {
		routes = append(routes, "GET /api/saas/v1/whoami")
	}
	if s.saasServiceAccountUsage != nil {
		routes = append(routes, "GET /api/saas/v1/usage")
	}
	if s.saasServiceAccountAlerts != nil {
		routes = append(routes, "GET /api/saas/v1/alerts")
	}
	if s.saasAdminApprovalPolicies != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/approvalPolicies")
	}
	if s.saasAdminApprovalPolicy != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/approvalPolicy", "PUT /dashboard/saasAdmin/approvalPolicy")
	}
	if s.saasAdminApprovals != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/approvals")
	}
	if s.saasAdminApprovalEvents != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/approvalEvents")
	}
	if s.saasAdminApprovalDecisions != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/approvalDecisions")
	}
	if s.saasAdminApprovalDelegations != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/approvalDelegations")
	}
	if s.saasAdminApprovalDelegation != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/approvalDelegation", "PUT /dashboard/saasAdmin/approvalDelegation")
	}
	if s.saasAdminApprovalReminders != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/approvalReminders", "PUT /dashboard/saasAdmin/approvalReminders")
	}
	if s.saasAdminApprovalRequest != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/approvalRequest", "PUT /dashboard/saasAdmin/approvalRequest")
	}
	if s.saasAdminApprovalDecision != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/approvalDecision", "PUT /dashboard/saasAdmin/approvalDecision")
	}
	if s.saasAdminApprovalCancel != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/approvalCancel", "PUT /dashboard/saasAdmin/approvalCancel")
	}
	if s.saasAdminApprovalExecute != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/approvalExecute", "PUT /dashboard/saasAdmin/approvalExecute")
	}
	if s.saasAdminInvoiceProfile != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/invoiceProfile", "POST /dashboard/saasAdmin/invoiceProfile", "PUT /dashboard/saasAdmin/invoiceProfile")
	}
	if s.saasAdminInvoiceDocuments != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/invoiceDocuments")
	}
	if s.saasAdminInvoice != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/invoice", "PUT /dashboard/saasAdmin/invoice")
	}
	if s.saasAdminCreditNote != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/creditNote", "PUT /dashboard/saasAdmin/creditNote")
	}
	if s.saasAdminInvoiceTransition != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/invoiceTransition", "PUT /dashboard/saasAdmin/invoiceTransition")
	}
	if s.saasPaymentWebhook != nil {
		routes = append(routes, "POST /webhooks/saas/payment")
	}
	if s.saasTenantDomainDeliveryWebhook != nil {
		routes = append(routes, "POST /webhooks/saas/domain-delivery")
	}
	if s.saasAdminOperations != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/operations")
	}
	if s.saasAdminBillingEvents != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/billingEvents")
	}
	if s.saasAdminBillingReconciliation != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/billingReconciliation")
	}
	if s.saasAdminBillingReconciliationFollowUp != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/billingReconciliationFollowUp", "PUT /dashboard/saasAdmin/billingReconciliationFollowUp")
	}
	if s.saasAdminBillingReconciliationFollowUps != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/billingReconciliationFollowUps")
	}
	if s.saasAdminBillingReconciliationFollowUpOwners != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/billingReconciliationFollowUpOwners")
	}
	if s.saasAdminBillingReconciliationFollowUpBulkClose != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/billingReconciliationFollowUpBulkClose", "PUT /dashboard/saasAdmin/billingReconciliationFollowUpBulkClose")
	}
	if s.saasAdminTasks != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/tasks")
	}
	if s.saasAdminTaskOwners != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/taskOwners")
	}
	if s.saasAdminTaskSLA != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/taskSla")
	}
	if s.saasAdminTaskSLANotifications != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/taskSlaNotifications", "PUT /dashboard/saasAdmin/taskSlaNotifications")
	}
	if s.saasAdminTaskCancel != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/taskCancel", "PUT /dashboard/saasAdmin/taskCancel")
	}
	if s.saasAdminTaskBulkCancel != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/taskBulkCancel", "PUT /dashboard/saasAdmin/taskBulkCancel")
	}
	if s.saasAdminTaskBulkReset != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/taskBulkReset", "PUT /dashboard/saasAdmin/taskBulkReset")
	}
	if s.saasAdminTaskReset != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/taskReset", "PUT /dashboard/saasAdmin/taskReset")
	}
	if s.saasAdminDailyReport != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/dailyReport")
	}
	if s.saasAdminExport != nil {
		routes = append(routes, "GET /dashboard/saasAdmin/export")
	}
	if s.saasAdminPackage != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/package", "PUT /dashboard/saasAdmin/package")
	}
	if s.saasAdminPackageSync != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/packageSync", "PUT /dashboard/saasAdmin/packageSync")
	}
	if s.saasAdminPackageSyncTask != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/packageSyncTask", "PUT /dashboard/saasAdmin/packageSyncTask")
	}
	if s.saasAdminPackageSyncTaskApply != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/packageSyncTaskApply", "PUT /dashboard/saasAdmin/packageSyncTaskApply")
	}
	if s.saasAdminPackageSyncTaskBulkApply != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/packageSyncTaskBulkApply", "PUT /dashboard/saasAdmin/packageSyncTaskBulkApply")
	}
	if s.saasAdminTenantStatus != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/tenantStatus", "PUT /dashboard/saasAdmin/tenantStatus")
	}
	if s.saasAdminTenantRenewal != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/tenantRenewal", "PUT /dashboard/saasAdmin/tenantRenewal")
	}
	if s.saasAdminTenantRenewalTask != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/tenantRenewalTask", "PUT /dashboard/saasAdmin/tenantRenewalTask")
	}
	if s.saasAdminTenantRenewalTaskApply != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/tenantRenewalTaskApply", "PUT /dashboard/saasAdmin/tenantRenewalTaskApply")
	}
	if s.saasAdminTenantRenewalTaskBulkApply != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/tenantRenewalTaskBulkApply", "PUT /dashboard/saasAdmin/tenantRenewalTaskBulkApply")
	}
	if s.saasAdminDashboardProvisioning != nil {
		routes = append(routes,
			"POST /dashboard/saasAdmin/tenants/provision",
			"POST /dashboard/saasAdmin/tenants/{tenantId}/activation/resend",
			"POST /dashboard/saasAdmin/tenants/{tenantId}/super-admin/replace",
			"POST /dashboard/saasAdmin/tenants/{tenantId}/super-admin/status",
			"GET /dashboard/saasAdmin/tenants/{tenantId}/dashboard-admins",
		)
	}
	if s.saasAdminTenantProvision != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/tenantProvision", "PUT /dashboard/saasAdmin/tenantProvision")
	}
	if s.saasAdminTenantProvisionTask != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/tenantProvisionTask", "PUT /dashboard/saasAdmin/tenantProvisionTask")
	}
	if s.saasAdminTenantProvisionTaskApply != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/tenantProvisionTaskApply", "PUT /dashboard/saasAdmin/tenantProvisionTaskApply")
	}
	if s.saasAdminTenantProvisionTaskBulkApply != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/tenantProvisionTaskBulkApply", "PUT /dashboard/saasAdmin/tenantProvisionTaskBulkApply")
	}
	if s.saasAdminTenantPackage != nil {
		routes = append(routes, "POST /dashboard/saasAdmin/tenantPackage", "PUT /dashboard/saasAdmin/tenantPackage")
	}
	if s.chatToolConfig != nil {
		routes = append(routes, "GET /dashboard/chatTool/config")
	}
	if s.commonUpload != nil {
		routes = append(routes, "POST /dashboard/common/upload")
	}
	if s.commonUploadFile != nil {
		routes = append(routes, "POST /dashboard/common/uploadFile")
	}
	if s.sidebarCommonUpload != nil {
		routes = append(routes, "POST /sidebar/common/upload")
	}
	if s.agentTxtUpload != nil {
		routes = append(routes, "POST /dashboard/agent/txtVerifyUpload")
	}
	if s.agentStore != nil {
		routes = append(routes, "POST /dashboard/agent/store")
	}
	if s.sidebarAgentAuth != nil {
		routes = append(routes, "GET /sidebar/agent/auth", "POST /sidebar/agent/auth")
	}
	if s.sidebarAgentOAuth != nil {
		routes = append(routes, "GET /sidebar/agent/oauth")
	}
	if s.sidebarAgentJSSDK != nil {
		routes = append(routes, "GET /sidebar/agent/jssdkConfig")
	}
	if s.sidebarWxJSSDK != nil {
		routes = append(routes, "GET /sidebar/wxJsSdk/config")
	}
	if s.agentTxtVerify {
		routes = append(routes, "GET /{wxVerifyTxt:WW_verify_[0-9a-zA-Z]{16}\\.txt$}")
	}
	if s.roleSelect != nil {
		routes = append(routes, "GET /dashboard/role/select")
	}
	if s.roleIndex != nil {
		routes = append(routes, "GET /dashboard/role/index")
	}
	if s.roleShow != nil {
		routes = append(routes, "GET /dashboard/role/show")
	}
	if s.rolePermission != nil {
		routes = append(routes, "GET /dashboard/role/permissionShow")
	}
	if s.roleShowEmployee != nil {
		routes = append(routes, "GET /dashboard/role/showEmployee")
	}
	if s.roleStore != nil {
		routes = append(routes, "POST /dashboard/role/store")
	}
	if s.roleUpdate != nil {
		routes = append(routes, "PUT /dashboard/role/update")
	}
	if s.roleStatusUpdate != nil {
		routes = append(routes, "PUT /dashboard/role/statusUpdate")
	}
	if s.roleDestroy != nil {
		routes = append(routes, "DELETE /dashboard/role/destroy")
	}
	if s.rolePermissionStore != nil {
		routes = append(routes, "POST /dashboard/role/permissionStore")
	}
	if s.menuIconIndex != nil {
		routes = append(routes, "GET /dashboard/menu/iconIndex")
	}
	if s.menuSelect != nil {
		routes = append(routes, "GET /dashboard/menu/select")
	}
	if s.menuIndex != nil {
		routes = append(routes, "GET /dashboard/menu/index")
	}
	if s.menuShow != nil {
		routes = append(routes, "GET /dashboard/menu/show")
	}
	if s.menuStore != nil {
		routes = append(routes, "POST /dashboard/menu/store")
	}
	if s.menuUpdate != nil {
		routes = append(routes, "PUT /dashboard/menu/update")
	}
	if s.menuStatusUpdate != nil {
		routes = append(routes, "PUT /dashboard/menu/statusUpdate")
	}
	if s.menuDestroy != nil {
		routes = append(routes, "DELETE /dashboard/menu/destroy")
	}
	return routes
}

func (s *Server) handleAgentTxtVerify(w http.ResponseWriter, r *http.Request) {
	code, ok := agentTxtVerifyCode(r.URL.Path)
	if !ok {
		s.forwardToPHP(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(code))
}

func isAgentTxtVerifyPath(path string) bool {
	_, ok := agentTxtVerifyCode(path)
	return ok
}

func officialAccountMessageEventCallbackPath(path string) bool {
	const prefix = "/dashboard/"
	const suffix = "/officialAccount/messageEventCallback"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return false
	}
	appID := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	return appID != "" && !strings.Contains(appID, "/")
}

func loadPath(path string) bool {
	return path == "/load" || strings.HasPrefix(path, "/load/")
}

func agentTxtVerifyCode(path string) (string, bool) {
	const prefix = "/WW_verify_"
	const suffix = ".txt"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return "", false
	}
	code := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	if len(code) != 16 {
		return "", false
	}
	for _, ch := range code {
		switch {
		case ch >= '0' && ch <= '9':
		case ch >= 'a' && ch <= 'z':
		case ch >= 'A' && ch <= 'Z':
		default:
			return "", false
		}
	}
	return code, true
}

func (s *Server) probePHPUpstream(ctx context.Context) (bool, string) {
	if s.cfg.PHPUpstream == "" {
		return false, "MOCHAT_PHP_UPSTREAM is empty"
	}

	probeURL, err := url.JoinPath(s.cfg.PHPUpstream, "/")
	if err != nil {
		return false, err.Error()
	}

	timeout := s.cfg.ProxyTimeout
	if timeout <= 0 || timeout > 3*time.Second {
		timeout = 3 * time.Second
	}
	client := http.Client{Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
	if err != nil {
		return false, err.Error()
	}
	res, err := client.Do(req)
	if err != nil {
		return false, err.Error()
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
	if res.StatusCode != http.StatusOK {
		return false, "unexpected status " + res.Status
	}
	if !strings.Contains(string(body), "MoChat") {
		return false, "upstream root does not look like MoChat"
	}
	return true, "ok"
}

func timeoutHandler(next http.Handler, timeout time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
