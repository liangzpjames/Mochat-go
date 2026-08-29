package main

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"jiyi/mochat-go/internal/aiproviderconfig"
	"jiyi/mochat-go/internal/authjwt"
	"jiyi/mochat-go/internal/authrealm"
	"jiyi/mochat-go/internal/clientip"
	"jiyi/mochat-go/internal/companyprofile"
	"jiyi/mochat-go/internal/config"
	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/dashboardadmin"
	"jiyi/mochat-go/internal/dashboardauth"
	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/frontend"
	"jiyi/mochat-go/internal/identitysecurity"
	archiveprovider "jiyi/mochat-go/internal/modules/providers/archive"
	wecomarchiveprovider "jiyi/mochat-go/internal/modules/providers/archive/wecom"
	audioprovider "jiyi/mochat-go/internal/modules/providers/audio/local"
	providercatalog "jiyi/mochat-go/internal/modules/providers/catalog"
	"jiyi/mochat-go/internal/mysqlconn"
	"jiyi/mochat-go/internal/outboundhttp"
	"jiyi/mochat-go/internal/providerstatus"
	"jiyi/mochat-go/internal/saasalertcredentials"
	"jiyi/mochat-go/internal/saasauditanchor"
	"jiyi/mochat-go/internal/saasauth"
	"jiyi/mochat-go/internal/saasbackup"
	"jiyi/mochat-go/internal/saascompliance"
	compatserver "jiyi/mochat-go/internal/server"
	"jiyi/mochat-go/internal/serviceaccountkey"
	"jiyi/mochat-go/internal/store"
	"jiyi/mochat-go/internal/taskrunner"
	"jiyi/mochat-go/internal/wechatopencredentials"
	"jiyi/mochat-go/internal/wecomcapability"
	"jiyi/mochat-go/internal/wecomcredentials"
	"jiyi/mochat-go/internal/wecomsuitecallback"
)

type companyProfileWeComVerifier struct {
	client *dashboard.RoomWelcomeWeComClient
}

type dashboardArchiveComponentBridge struct {
	client *archiveprovider.BridgeArchiveClient
}

func (bridge dashboardArchiveComponentBridge) FetchArchiveComponent(ctx context.Context, object dashboard.ArchiveComponentObject) (dashboard.ArchiveComponentContent, error) {
	if bridge.client == nil {
		return dashboard.ArchiveComponentContent{}, fmt.Errorf("archive component bridge is unavailable")
	}
	content, err := bridge.client.FetchComponent(ctx, archiveprovider.ComponentRequest{
		Scope: archiveprovider.Scope{TenantID: int64(object.TenantID), CorpID: int64(object.CorpID)}, WXCorpID: object.WXCorpID,
		MessageID: object.MessageID, PublicKeyVersion: object.PublicKeyVersion, EncryptedSecretKey: object.EncryptedSecretKey,
	})
	if err != nil {
		return dashboard.ArchiveComponentContent{}, err
	}
	return dashboard.ArchiveComponentContent{Type: content.Type, MIMEType: content.MIMEType, FileName: content.FileName, Body: content.Data}, nil
}

func (v companyProfileWeComVerifier) Verify(ctx context.Context, request companyprofile.VerificationRequest) (companyprofile.VerificationResult, error) {
	if v.client == nil || request.TenantID <= 0 || request.CorpID <= 0 || strings.TrimSpace(request.WXCorpID) == "" {
		return companyprofile.VerificationResult{}, fmt.Errorf("company verification provider is not configured")
	}
	result, err := v.client.VerifyCompany(ctx, request.WXCorpID, request.Credentials.EmployeeSecret, request.Credentials.ContactSecret)
	if err != nil {
		return companyprofile.VerificationResult{}, err
	}
	return companyprofile.VerificationResult{WXCorpID: result.WXCorpID, CorpName: result.CorpName}, nil
}

func main() {
	configureLogging()
	cfg, err := config.Load()
	if err != nil {
		fatalf("load config: %v", err)
	}
	archivePlan := archiveRuntimePlanFor(cfg)
	applicationLocation, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		fatalf("load application timezone: %v", err)
	}
	debugf("runtime mode: role=%s standalone=%t all_migrated_routes_default=%t php_fallback_enabled=%t", cfg.RuntimeRole, cfg.Standalone, cfg.EnableAllMigratedRoutes, strings.TrimSpace(cfg.PHPUpstream) != "")
	debugf("identity MFA requirements: saas_admin_required=%t dashboard_required=%t", cfg.SaaSAdminMFARequired, cfg.DashboardMFARequired)
	alertCredentialManager, err := saasalertcredentials.NewManager(saasalertcredentials.Config{
		EncryptionKey:       cfg.SaaSAlertCredentialEncryptionKey,
		EncryptionKeys:      cfg.SaaSAlertCredentialEncryptionKeys,
		EncryptionKeyID:     cfg.SaaSAlertCredentialEncryptionKeyID,
		RequireEncryption:   cfg.SaaSAlertCredentialRequireEncryption,
		DedicatedConfigured: cfg.SaaSAlertCredentialDedicatedConfigured,
	})
	if err != nil {
		fatalf("build SaaS alert credential encryption manager: %v", err)
	}
	alertCredentialStatus := alertCredentialManager.ConfigStatus()
	debugf("SaaS alert credential protection: encryption_configured=%t require_encryption=%t dedicated_configured=%t active_key_id=%s key_count=%d",
		alertCredentialStatus.EncryptionConfigured, alertCredentialStatus.RequireEncryption,
		alertCredentialStatus.DedicatedConfigured, alertCredentialStatus.ActiveKeyID, alertCredentialStatus.KeyCount)
	weComCredentialManager, err := wecomcredentials.NewManager(wecomcredentials.Config{
		EncryptionKey:       cfg.WeComCredentialEncryptionKey,
		EncryptionKeys:      cfg.WeComCredentialEncryptionKeys,
		EncryptionKeyID:     cfg.WeComCredentialEncryptionKeyID,
		RequireEncryption:   cfg.WeComCredentialRequireEncryption,
		DedicatedConfigured: cfg.WeComCredentialDedicatedConfigured,
	})
	if err != nil {
		fatalf("build WeCom credential encryption manager: %v", err)
	}
	weComCredentialStatus := weComCredentialManager.ConfigStatus()
	if cfg.EnableDurableWorkMessageArchive && !weComCredentialStatus.EncryptionConfigured {
		fatal("durable work message archive requires configured WeCom credential encryption")
	}
	debugf("WeCom credential protection: encryption_configured=%t require_encryption=%t dedicated_configured=%t active_key_id=%s key_count=%d",
		weComCredentialStatus.EncryptionConfigured, weComCredentialStatus.RequireEncryption,
		weComCredentialStatus.DedicatedConfigured, weComCredentialStatus.ActiveKeyID, weComCredentialStatus.KeyCount)
	weChatOpenCredentialManager, err := wechatopencredentials.NewManager(wechatopencredentials.Config{
		EncryptionKey:       cfg.WeChatOpenCredentialEncryptionKey,
		EncryptionKeys:      cfg.WeChatOpenCredentialEncryptionKeys,
		EncryptionKeyID:     cfg.WeChatOpenCredentialEncryptionKeyID,
		RequireEncryption:   cfg.WeChatOpenCredentialRequireEncryption,
		DedicatedConfigured: cfg.WeChatOpenCredentialDedicatedConfigured,
	})
	if err != nil {
		fatalf("build WeChat Open credential encryption manager: %v", err)
	}
	weChatOpenCredentialStatus := weChatOpenCredentialManager.ConfigStatus()
	debugf("WeChat Open credential protection: encryption_configured=%t require_encryption=%t dedicated_configured=%t active_key_id=%s key_count=%d",
		weChatOpenCredentialStatus.EncryptionConfigured, weChatOpenCredentialStatus.RequireEncryption,
		weChatOpenCredentialStatus.DedicatedConfigured, weChatOpenCredentialStatus.ActiveKeyID, weChatOpenCredentialStatus.KeyCount)
	aiProviderCredentialManager, err := aiproviderconfig.NewManager(aiproviderconfig.Config{
		EncryptionKey: cfg.AIProviderCredentialEncryptionKey, EncryptionKeys: cfg.AIProviderCredentialEncryptionKeys,
		EncryptionKeyID: cfg.AIProviderCredentialEncryptionKeyID, RequireEncryption: cfg.AIProviderCredentialRequireEncryption,
	})
	if err != nil {
		fatalf("build AI provider credential encryption manager: %v", err)
	}

	var options []compatserver.Option
	var mysqlStore *store.MySQLStore
	getMySQLStore := func() *store.MySQLStore {
		if mysqlStore != nil {
			return mysqlStore
		}
		db, err := mysqlconn.Open(cfg.MySQLDSN)
		if err != nil {
			fatalf("open mysql: %v", err)
		}
		if err := db.Ping(); err != nil {
			fatalf("ping mysql: %v", err)
		}
		mysqlStore = store.NewMySQLStore(db).
			WithSaaSAlertCredentialCipher(alertCredentialManager).
			WithWeComCredentialCipher(weComCredentialManager).
			WithWeChatOpenCredentialCipher(weChatOpenCredentialManager).
			WithAIProviderCredentialCipher(aiProviderCredentialManager).
			WithAIProviderOutboundGuard(outboundhttp.MustDefaultGuard())
		return mysqlStore
	}
	if cfg.EnableWeComSuiteCallback {
		exchanger, exchangeErr := wecomsuitecallback.NewBridgeAuthorizationExchanger(cfg.WorkMessageArchiveBridgeBaseURL, cfg.WorkMessageArchiveBridgeToken, nil)
		if exchangeErr != nil {
			fatalf("build WeCom suite authorization exchanger: %v", exchangeErr)
		}
		callbackHandler, callbackErr := wecomsuitecallback.NewHandler(wecomsuitecallback.Config{
			SuiteID: cfg.WeComSuiteID, SuiteSecret: cfg.WeComSuiteSecret,
			CallbackToken: cfg.WeComSuiteCallbackToken, EncodingAESKey: cfg.WeComSuiteEncodingAESKey,
			Logger: structuredLogger(),
		}, getMySQLStore(), exchanger)
		if callbackErr != nil {
			fatalf("build WeCom suite callback handler: %v", callbackErr)
		}
		options = append(options, compatserver.WithWeComSuiteCallbackHandler(callbackHandler))
		debugf("go WeCom suite callback enabled: POST /wecom/suite/callback")
	}

	var redisStore *store.RedisStore
	getRedisStore := func() *store.RedisStore {
		if redisStore != nil {
			return redisStore
		}
		redisStore = store.NewRedisStore(store.RedisConfig{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		})
		if err := redisStore.Ping(context.Background()); err != nil {
			fatalf("ping redis: %v", err)
		}
		return redisStore
	}
	var weWorkCallbackRedisStore *store.RedisStore
	getOptionalWeWorkCallbackRedisStore := func() *store.RedisStore {
		if weWorkCallbackRedisStore == nil {
			weWorkCallbackRedisStore = store.NewRedisStore(store.RedisConfig{
				Addr: cfg.RedisAddr, Password: cfg.RedisPassword, DB: cfg.RedisDB,
			})
		}
		return weWorkCallbackRedisStore
	}
	var identityManager *identitysecurity.Manager
	var identitySessionChecker authjwt.SessionChecker
	var tenantDomainVerifier *dashboard.SaaSTenantDomainDNSVerifier
	var dashboardIdentityGuard *dashboardauth.RequestGuard
	var serviceAccountKeyManager *serviceaccountkey.Manager
	var serviceAccountClientIPResolver *clientip.Resolver
	var dashboardAdminService *dashboardadmin.Service
	if cfg.EnableSaaSAdminDashboard {
		dashboardAdminService = dashboardadmin.NewService(getMySQLStore())
		saasMFAKey, parseErr := saasbackup.ParseEncryptionKey(cfg.SaaSAdminMFAEncryptionKey)
		if parseErr != nil {
			fatalf("build SaaS MFA encryption key: %v", parseErr)
		}
		saasTokenConfig := authrealm.TokenConfig{
			Secret: []byte(cfg.SaaSAdminJWTSecret), Issuer: cfg.SaaSAdminJWTIssuer,
			Audience: cfg.SaaSAdminJWTAudience, TTL: cfg.SaaSAdminJWTTTL,
			Realm: authrealm.RealmSaaSAdmin, Prefix: cfg.SaaSAdminJWTPrefix,
		}
		saasIdentityStore := store.NewSaaSIdentityStore(getMySQLStore().DB())
		saasIdentityService := saasauth.NewService(saasIdentityStore)
		saasParser := authrealm.Parser{
			Config: saasTokenConfig,
			ValidateSession: func(ctx context.Context, claims authrealm.Claims) error {
				return saasIdentityService.CheckTokenSession(ctx, claims)
			},
		}
		saasAuthHandler, authErr := saasauth.NewHTTPHandler(saasauth.HTTPConfig{
			Service:     saasIdentityService,
			Persistence: saasIdentityStore,
			Signer:      saasTokenConfig,
			Parser:      saasParser,
			MFAKey:      saasMFAKey,
			MFAKeyID:    cfg.SaaSAdminMFAEncryptionKeyID,
			MFARequired: cfg.SaaSAdminMFARequired,
		})
		if authErr != nil {
			fatalf("build SaaS authentication handler: %v", authErr)
		}
		saasRequestGuard, authErr := saasauth.NewRequestGuard(saasParser)
		if authErr != nil {
			fatalf("build SaaS request guard: %v", authErr)
		}
		options = append(options,
			compatserver.WithSaaSAuthHandler(saasAuthHandler),
			compatserver.WithSaaSRequestGuard(saasRequestGuard),
			compatserver.WithSaaSLoginPageHandler(saasauth.NewLoginPageHandler()),
		)
		tenantDomainVerifier, err = dashboard.NewSaaSTenantDomainDNSVerifier(cfg.SaaSTenantDomainDNSServer, cfg.SaaSTenantDomainDNSTimeout)
		if err != nil {
			fatalf("build SaaS tenant domain DNS verifier: %v", err)
		}
		serviceAccountKeyManager, err = serviceaccountkey.NewManager(serviceaccountkey.Config{
			ActiveKeyID: cfg.SaaSServiceAccountKeyPepperID, ActiveKey: cfg.SaaSServiceAccountKeyPepper,
			Keys: cfg.SaaSServiceAccountKeyPeppers, LegacyJWTSecret: cfg.SimpleJWTSecret,
			AllowLegacyJWT:   cfg.SaaSServiceAccountAllowLegacyJWTPepper,
			RequireDedicated: cfg.SaaSServiceAccountRequireDedicatedPepper,
		})
		if err != nil {
			fatalf("build SaaS service account pepper manager: %v", err)
		}
		pepperStatus := serviceAccountKeyManager.ConfigStatus()
		debugf("SaaS service account pepper manager enabled: active_key_id=%s key_count=%d dedicated_configured=%v legacy_jwt_enabled=%v require_dedicated=%v",
			pepperStatus.ActiveKeyID, pepperStatus.KeyCount, pepperStatus.DedicatedConfigured,
			pepperStatus.LegacyJWTEnabled, pepperStatus.RequireDedicated)
		serviceAccountClientIPResolver, err = clientip.NewResolver(clientip.Config{
			TrustProxyHeaders: cfg.SaaSServiceAccountTrustProxyHeaders,
			TrustedProxyCIDRs: cfg.SaaSTrustedProxyCIDRs,
		})
		if err != nil {
			fatalf("build SaaS service account client IP resolver: %v", err)
		}
		clientIPStatus := serviceAccountClientIPResolver.Status()
		debugf("SaaS service account client IP resolver enabled: trusted_proxy_headers=%t trusted_proxy_cidrs=%d",
			clientIPStatus.TrustProxyHeaders, clientIPStatus.TrustedProxyCIDRCount)
	}

	if cfg.EnableSaaSIdentitySecurity {
		identityManager, err = identitysecurity.NewManager(getMySQLStore(), identitysecurity.Config{
			EncryptionKey: cfg.SaaSIdentityEncryptionKey, EncryptionKeys: cfg.SaaSIdentityEncryptionKeys,
			EncryptionKeyID: cfg.SaaSIdentityEncryptionKeyID, Issuer: cfg.SaaSIdentityIssuer,
			EnforceSessions: cfg.SaaSIdentityEnforceSessions, TrustProxyHeaders: cfg.SaaSIdentityTrustProxyHeaders,
			TrustedProxyCIDRs: cfg.SaaSTrustedProxyCIDRs,
		})
		if err != nil {
			fatalf("build SaaS identity security manager: %v", err)
		}
		identitySessionChecker = identityManager
		status := identityManager.ConfigStatus()
		debugf("SaaS identity security enabled: session_enforced=%t mfa_key_id=%s mfa_key_count=%d trusted_proxy_headers=%t trusted_proxy_cidrs=%d",
			status.SessionEnforced, status.MFAEncryptionKeyID, status.MFAEncryptionKeys, status.TrustedProxyHeaders, status.TrustedProxyCIDRCount)
	}
	buildUserResolver := newUserResolverBuilder(cfg, identitySessionChecker, getRedisStore)

	buildSidebarEmployeeResolver := func(routeName string) dashboard.UserIDResolver {
		if cfg.DevAuthHeader {
			debugf("%s auth resolver: development header X-Mochat-Go-Employee-ID", routeName)
			return dashboard.HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"}
		}
		if cfg.SkipJWTBlacklist {
			debugf("%s auth resolver: PHP sidebar simple-jwt compatible parser without Redis blacklist checks", routeName)
			return authjwt.Parser{
				Secret:        cfg.SidebarJWTSecret,
				Prefix:        cfg.SidebarJWTPrefix,
				SkipBlacklist: true,
			}
		}
		redisStore := getRedisStore()
		debugf("%s auth resolver: PHP sidebar simple-jwt compatible parser", routeName)
		return authjwt.Parser{
			Secret:        cfg.SidebarJWTSecret,
			Prefix:        cfg.SidebarJWTPrefix,
			Blacklist:     redisStore,
			SkipBlacklist: cfg.SkipJWTBlacklist,
		}
	}

	var saasAlertNotifier dashboard.SaaSAlertNotifier
	var saasAlertWebhookNotifier dashboard.SaaSAlertNotifier
	var saasAlertEnvWebhookNotifier dashboard.SaaSAlertNotifier
	saasAlertWebhookGuard, err := outboundhttp.NewGuard(outboundhttp.Config{
		RequireHTTPS: cfg.SaaSAlertWebhookRequireHTTPS,
		AllowedCIDRs: cfg.SaaSAlertWebhookAllowedCIDRs,
	})
	if err != nil {
		fatalf("build SaaS alert webhook egress guard: %v", err)
	}
	webhookSecurity := saasAlertWebhookGuard.Status()
	debugf("go SaaS alert webhook egress security: require_https=%t private_networks_blocked=%t metadata_blocked=%t allowed_cidrs=%d dns_pinning=%t same_origin_redirects=%t environment_proxy_disabled=%t",
		webhookSecurity.RequireHTTPS, webhookSecurity.PrivateNetworksBlocked, webhookSecurity.MetadataAddressesBlocked,
		webhookSecurity.AllowedCIDRCount, webhookSecurity.DNSPinningEnabled, webhookSecurity.SameOriginRedirectsOnly,
		webhookSecurity.EnvironmentProxyDisabled)
	var releaseEvidenceRootCAs *x509.CertPool
	if cfg.SaaSReleaseEvidenceCAFile != "" {
		releaseEvidenceRootCAs, err = x509.SystemCertPool()
		if err != nil || releaseEvidenceRootCAs == nil {
			releaseEvidenceRootCAs = x509.NewCertPool()
		}
		certificatePEM, readErr := os.ReadFile(cfg.SaaSReleaseEvidenceCAFile)
		if readErr != nil {
			fatalf("read SaaS release evidence CA file: %v", readErr)
		}
		if !releaseEvidenceRootCAs.AppendCertsFromPEM(certificatePEM) {
			fatalf("parse SaaS release evidence CA file: no valid PEM certificate")
		}
	}
	releaseEvidenceGuard, err := outboundhttp.NewGuard(outboundhttp.Config{
		RequireHTTPS: true,
		AllowedCIDRs: cfg.SaaSReleaseEvidenceAllowedCIDRs,
		TLSRootCAs:   releaseEvidenceRootCAs,
	})
	if err != nil {
		fatalf("build SaaS release evidence egress guard: %v", err)
	}
	releaseEvidenceVerifier, err := dashboard.NewSaaSReleaseArtifactVerifier(
		releaseEvidenceGuard,
		cfg.SaaSReleaseEvidenceVerifyTimeout,
		cfg.SaaSReleaseEvidenceMaxBytes,
	)
	if err != nil {
		fatalf("build SaaS release evidence verifier: %v", err)
	}
	releaseEvidenceSecurity := releaseEvidenceVerifier.Status()
	debugf("go SaaS release evidence verifier: timeout=%s max_bytes=%d require_https=%t private_networks_blocked=%t metadata_blocked=%t allowed_cidrs=%d custom_root_cas=%t dns_pinning=%t same_origin_redirects=%t environment_proxy_disabled=%t",
		cfg.SaaSReleaseEvidenceVerifyTimeout, cfg.SaaSReleaseEvidenceMaxBytes,
		releaseEvidenceSecurity.RequireHTTPS, releaseEvidenceSecurity.PrivateNetworksBlocked,
		releaseEvidenceSecurity.MetadataAddressesBlocked, releaseEvidenceSecurity.ExplicitAllowedCIDRCount,
		releaseEvidenceSecurity.CustomRootCAsConfigured,
		releaseEvidenceSecurity.DNSPinningEnabled, releaseEvidenceSecurity.SameOriginRedirectsOnly,
		releaseEvidenceSecurity.EnvironmentProxyDisabled)
	persistentOutbox := false
	if strings.TrimSpace(cfg.SaaSAlertWebhookURL) != "" {
		webhookNotifier, err := dashboard.NewSaaSAlertWebhookNotifierWithTemplates(
			cfg.SaaSAlertWebhookURL,
			cfg.SaaSAlertWebhookTimeout,
			cfg.SaaSAlertWebhookSecret,
			cfg.SaaSAlertWebhookTitleTemplate,
			cfg.SaaSAlertWebhookBodyTemplate,
			saasAlertWebhookGuard,
		)
		if err != nil {
			fatalf("build SaaS alert webhook notifier: %v", err)
		}
		retryingNotifier := dashboard.NewRetryingSaaSAlertNotifier(webhookNotifier, cfg.SaaSAlertWebhookRetryAttempts, cfg.SaaSAlertWebhookRetryDelay)
		saasAlertEnvWebhookNotifier = retryingNotifier
	}
	if strings.TrimSpace(cfg.MySQLDSN) != "" {
		saasAlertWebhookNotifier = dashboard.NewStoreBackedSaaSAlertNotifier(
			getMySQLStore(),
			saasAlertEnvWebhookNotifier,
			dashboard.SaaSAlertNotificationChannelWebhook,
			saasAlertWebhookGuard,
		)
		saasAlertNotifier = dashboard.NewPersistentSaaSAlertNotifier(
			getMySQLStore(),
			saasAlertWebhookNotifier,
			dashboard.SaaSAlertNotificationChannelWebhook,
			cfg.SaaSAlertNotificationMaxAttempts,
			cfg.SaaSAlertNotificationRetryDelay,
		)
		persistentOutbox = true
		debugf("go SaaS alert tenant settings enabled: channel=webhook global_fallback=%t outbox=true outbox_max_attempts=%d outbox_retry_delay=%s", saasAlertEnvWebhookNotifier != nil, cfg.SaaSAlertNotificationMaxAttempts, cfg.SaaSAlertNotificationRetryDelay)
	} else if saasAlertEnvWebhookNotifier != nil {
		saasAlertWebhookNotifier = saasAlertEnvWebhookNotifier
		saasAlertNotifier = saasAlertEnvWebhookNotifier
	}
	if strings.TrimSpace(cfg.SaaSAlertWebhookURL) != "" {
		debugf("go SaaS alert webhook enabled: endpoint_configured=true timeout=%s signed=%t retry_attempts=%d retry_delay=%s templated=true outbox=%t outbox_max_attempts=%d outbox_retry_delay=%s", cfg.SaaSAlertWebhookTimeout, strings.TrimSpace(cfg.SaaSAlertWebhookSecret) != "", cfg.SaaSAlertWebhookRetryAttempts, cfg.SaaSAlertWebhookRetryDelay, persistentOutbox, cfg.SaaSAlertNotificationMaxAttempts, cfg.SaaSAlertNotificationRetryDelay)
	}

	var dashboardPrincipalResolver dashboardprincipal.PrincipalResolver
	if cfg.MigrateAuth {
		mysqlStore := getMySQLStore()
		dashboardMFAKey, parseErr := saasbackup.ParseEncryptionKey(cfg.DashboardMFAEncryptionKey)
		if parseErr != nil {
			fatalf("build Dashboard MFA encryption key: %v", parseErr)
		}
		dashboardTokenConfig := authrealm.TokenConfig{
			Secret: []byte(cfg.DashboardJWTSecret), Issuer: cfg.DashboardJWTIssuer,
			Audience: cfg.DashboardJWTAudience, TTL: cfg.DashboardJWTTTL,
			Realm: authrealm.RealmDashboard, Prefix: cfg.DashboardJWTPrefix,
		}
		dashboardIdentityStore := store.NewDashboardIdentityStore(mysqlStore.DB())
		dashboardIdentityService := dashboardauth.NewService(dashboardIdentityStore)
		dashboardTenantGate := dashboardauth.DashboardTenantGate(func(ctx context.Context, tenantID int, now time.Time) (dashboardauth.TenantAccess, error) {
			access, err := mysqlStore.DashboardTenantAccess(ctx, tenantID, now)
			return dashboardauth.TenantAccess{TenantID: access.TenantID, Allowed: access.Allowed, Reason: access.Reason}, err
		})
		dashboardBindingStore := store.NewTenantCorpBindingStore(mysqlStore.DB())
		var principalErr error
		dashboardPrincipalResolver, principalErr = dashboardprincipal.NewResolverWithIdentity(
			dashboardIdentityStore,
			dashboardprincipal.TenantGateFunc(func(ctx context.Context, tenantID int, now time.Time) (bool, error) {
				access, err := dashboardTenantGate(ctx, tenantID, now)
				if err != nil {
					return false, err
				}
				return access.Allowed && access.TenantID == tenantID, nil
			}),
			dashboardBindingStore,
		)
		if principalErr != nil {
			fatalf("build Dashboard principal resolver: %v", principalErr)
		}
		dashboardParser := authrealm.Parser{
			Config: dashboardTokenConfig,
			ValidateSession: func(ctx context.Context, claims authrealm.Claims) error {
				return dashboardIdentityService.CheckTokenSession(ctx, claims)
			},
		}
		dashboardAuthHandler, authErr := dashboardauth.NewHTTPHandler(dashboardauth.HTTPConfig{
			Service: dashboardIdentityService, Persistence: dashboardIdentityStore,
			Signer: dashboardTokenConfig, Parser: dashboardParser,
			MFAKey: dashboardMFAKey, MFAKeyID: cfg.DashboardMFAEncryptionKeyID,
			MFARequired: cfg.DashboardMFARequired,
			TenantGate:  dashboardTenantGate, PrincipalResolver: dashboardPrincipalResolver,
		})
		if authErr != nil {
			fatalf("build Dashboard authentication handler: %v", authErr)
		}
		dashboardIdentityGuard, authErr = dashboardauth.NewRequestGuard(dashboardParser, dashboardIdentityStore, dashboardTenantGate)
		if authErr != nil {
			fatalf("build Dashboard request guard: %v", authErr)
		}
		dashboardIdentityGuard.WithPrincipalResolver(dashboardPrincipalResolver)
		dashboardIdentityGuard.WithPublicRouteContracts(dashboard.PublicDashboardRouteContracts())
		companyProfileWeComClient := dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL).WithCallbackRuntime(
			cfg.MigrateWeWorkCallback,
			cfg.EnableWeWorkCallbackWorker,
		)
		companyProfileService := companyprofile.NewService(mysqlStore, companyProfileWeComVerifier{client: companyProfileWeComClient}).WithEmployeeSyncScheduler(
			dashboard.NewCompanyEmployeeSyncScheduler(getRedisStore()),
		)
		if archivePlan.durableAPI {
			companyArchiveBridgeClient, bridgeErr := archiveprovider.NewBridgeArchiveClient(
				cfg.WorkMessageArchiveBridgeBaseURL,
				cfg.WorkMessageArchiveBridgeToken,
				nil,
			)
			if bridgeErr != nil {
				fatalf("build Dashboard manual archive synchronization: %v", bridgeErr)
			}
			companyArchiveRunner := archiveprovider.NewDurableBridgeRunner(mysqlStore, companyArchiveBridgeClient, cfg.WorkMessageArchiveSyncLimit)
			companyProfileService.WithArchiveSyncScheduler(dashboard.NewCompanyArchiveSyncScheduler(companyArchiveRunner))
		}
		aiRuntime, err := buildDashboardAIStatusProvider(cfg)
		if err != nil {
			fatalf("build AI Provider runtime: %v", err)
		}
		audioRuntime, err := audioprovider.New(audioprovider.Config{Root: cfg.FileStorageRoot})
		if err != nil {
			fatalf("build audio Provider runtime: %v", err)
		}
		archiveRuntime, err := wecomarchiveprovider.New(wecomarchiveprovider.Config{})
		if err != nil {
			fatalf("build archive Provider runtime: %v", err)
		}
		providerRegistry, err := providercatalog.NewRegistry(providercatalog.Dependencies{
			AI:            aiRuntime,
			AIEnabled:     cfg.EnableAIDebtClearance && cfg.EnableAIInsight,
			Archive:       archiveRuntime,
			AudioStorage:  audioRuntime,
			WeComStandard: companyProfileWeComClient,
		})
		if err != nil {
			fatalf("build Provider registry: %v", err)
		}
		providerStatusService := providerstatus.NewService(companyprofile.NewProviderStatusSource(mysqlStore, providerRegistry))
		options = append(options,
			compatserver.WithDashboardAuthHandler(dashboardAuthHandler),
			compatserver.WithCompanyProfileHandler(companyprofile.NewHTTPHandler(companyProfileService)),
			compatserver.WithProviderStatusHandler(providerstatus.NewHTTPHandler(providerStatusService)),
		)
		routeDebugf("go Dashboard identity routes enabled: POST /dashboard/user/auth POST /dashboard/user/authMFA POST /dashboard/auth/activate POST /dashboard/auth/password/reset-request POST /dashboard/auth/password/reset GET /dashboard/auth/session")
		routeDebugf("go Dashboard company profile routes enabled: GET/PUT /dashboard/company/profile PUT /dashboard/company/wecom-credentials PUT /dashboard/company/agent-credentials PUT /dashboard/company/archive-credentials POST /dashboard/company/verify POST /dashboard/company/employee-sync GET /dashboard/company/sync-status POST /dashboard/company/archive-sync GET /dashboard/company/archive-sync-status GET /dashboard/company/audits")
	}

	if cfg.MigrateUserIndex || cfg.MigrateUserShow || cfg.MigrateUserStore || cfg.MigrateUserUpdate || cfg.MigrateUserStatusUpdate || cfg.MigrateUserPasswordReset || cfg.MigrateUserPasswordUpdate {
		mysqlStore := getMySQLStore()
		resolver, loginCache := buildUserResolver("userAdmin")
		var logoutStore dashboard.LogoutStore
		parser := authjwt.Parser{
			Secret:   cfg.SimpleJWTSecret,
			Prefix:   cfg.SimpleJWTPrefix,
			Sessions: identitySessionChecker,
		}
		if cfg.MigrateUserPasswordUpdate {
			redisStore := getRedisStore()
			logoutStore = redisStore
			parser.Blacklist = redisStore
		}
		userAdmin := dashboard.NewUserAdminHandler(mysqlStore, loginCache, resolver, dashboard.NewRBACResolver(mysqlStore), cfg.SimpleJWTSecret, logoutStore, parser, cfg.SimpleJWTRefreshTTL)
		if cfg.MigrateUserIndex {
			options = append(options, compatserver.WithUserIndexHandler(http.HandlerFunc(userAdmin.Index)))
			routeDebugf("go migrated route enabled: GET /dashboard/user/index")
		}
		if cfg.MigrateUserShow {
			options = append(options, compatserver.WithUserShowHandler(http.HandlerFunc(userAdmin.Show)))
			routeDebugf("go migrated route enabled: GET /dashboard/user/show")
		}
		if cfg.MigrateUserStore {
			options = append(options, compatserver.WithUserStoreHandler(http.HandlerFunc(userAdmin.Store)))
			routeDebugf("go migrated route enabled: POST /dashboard/user/store")
		}
		if cfg.MigrateUserUpdate {
			options = append(options, compatserver.WithUserUpdateHandler(http.HandlerFunc(userAdmin.Update)))
			routeDebugf("go migrated route enabled: PUT /dashboard/user/update")
		}
		if cfg.MigrateUserStatusUpdate {
			options = append(options, compatserver.WithUserStatusUpdateHandler(http.HandlerFunc(userAdmin.StatusUpdate)))
			routeDebugf("go migrated route enabled: PUT /dashboard/user/statusUpdate")
		}
		if cfg.MigrateUserPasswordReset {
			options = append(options, compatserver.WithUserPasswordResetHandler(http.HandlerFunc(userAdmin.PasswordReset)))
			routeDebugf("go migrated route enabled: PUT /dashboard/user/passwordReset")
		}
		if cfg.MigrateUserPasswordUpdate {
			options = append(options, compatserver.WithUserPasswordUpdateHandler(http.HandlerFunc(userAdmin.PasswordUpdate)))
			routeDebugf("go migrated route enabled: PUT /dashboard/user/passwordUpdate")
		}
	}

	if cfg.MigratePermissionByUser {
		mysqlStore := getMySQLStore()
		resolver, _ := buildUserResolver("permissionByUser")
		options = append(options, compatserver.WithPermissionByUserHandler(
			dashboard.NewPermissionByUserHandler(mysqlStore, resolver),
		))
		routeDebugf("go migrated route enabled: GET /dashboard/role/permissionByUser")
	}

	if cfg.MigrateWeWorkCallback {
		mysqlStore := getMySQLStore()
		// The worker polls the durable MySQL inbox. Do not publish an unconsumed
		// Redis wakeup or make callback ACK latency depend on Redis at all.
		weWorkCallback := dashboard.NewWeWorkCallbackHandler(mysqlStore, nil)
		options = append(options, compatserver.WithWeWorkCallbackHandler(weWorkCallback))
		routeDebugf("go migrated route enabled: GET/POST /weWork/callback")
		routeDebugf("go migrated route enabled: GET/POST /dashboard/corp/weWorkCallback")
	}

	if cfg.MigrateCorpDataIndex || cfg.MigrateCorpDataLineChat {
		mysqlStore := getMySQLStore()
		resolver, loginCache := buildUserResolver("corpData")
		corpData := dashboard.NewCorpDataHandler(mysqlStore, loginCache, resolver, dashboard.NewRBACResolver(mysqlStore)).WithLocation(applicationLocation)
		externalTenant := dashboard.NewExternalTenantHandler(mysqlStore, resolver, cfg.SaaSPlatformAdminTenantID)
		if cfg.MigrateCorpDataIndex {
			options = append(options, compatserver.WithCorpDataIndexHandler(http.HandlerFunc(corpData.Index)))
			options = append(options, compatserver.WithExternalTenantIndexHandler(http.HandlerFunc(externalTenant.TenantIndex)))
			routeDebugf("go migrated route enabled: GET /dashboard/corpData/index")
			routeDebugf("go migrated route enabled: GET /dashboard/external/tenantIndex")
		}
		if cfg.MigrateCorpDataLineChat {
			options = append(options, compatserver.WithCorpDataLineChatHandler(http.HandlerFunc(corpData.LineChat)))
			routeDebugf("go migrated route enabled: GET /dashboard/corpData/lineChat")
		}
	}

	if cfg.MigrateStatisticIndex || cfg.MigrateStatisticTopList || cfg.MigrateStatisticEmployeeCounts || cfg.MigrateStatisticEmployees || cfg.MigrateStatisticEmployeesTrend {
		mysqlStore := getMySQLStore()
		resolver, loginCache := buildUserResolver("statistic")
		statistic := dashboard.NewStatisticHandler(mysqlStore, loginCache, resolver, dashboard.NewRBACResolver(mysqlStore), cfg.APIBaseURL, dashboard.NewStatisticWeComClient(cfg.WeComAPIBaseURL))
		if cfg.MigrateStatisticIndex {
			options = append(options, compatserver.WithStatisticIndexHandler(http.HandlerFunc(statistic.Index)))
			routeDebugf("go migrated route enabled: GET /dashboard/statistic/index")
		}
		if cfg.MigrateStatisticTopList {
			options = append(options, compatserver.WithStatisticTopListHandler(http.HandlerFunc(statistic.TopList)))
			routeDebugf("go migrated route enabled: GET /dashboard/statistic/topList")
		}
		if cfg.MigrateStatisticEmployeeCounts {
			options = append(options, compatserver.WithStatisticEmployeeCountsHandler(http.HandlerFunc(statistic.EmployeeCounts)))
			routeDebugf("go migrated route enabled: GET /dashboard/statistic/employeeCounts")
		}
		if cfg.MigrateStatisticEmployees {
			options = append(options, compatserver.WithStatisticEmployeesHandler(http.HandlerFunc(statistic.Employees)))
			routeDebugf("go migrated route enabled: GET /dashboard/statistic/employees")
		}
		if cfg.MigrateStatisticEmployeesTrend {
			options = append(options, compatserver.WithStatisticEmployeesTrendHandler(http.HandlerFunc(statistic.EmployeesTrend)))
			routeDebugf("go migrated route enabled: GET /dashboard/statistic/employeesTrend")
		}
	}

	if cfg.MigrateWorkEmployeeIndex || cfg.MigrateWorkEmployeeCond || cfg.MigrateWorkEmployeeSync || cfg.MigrateWorkDeptIndex || cfg.MigrateWorkDeptMember || cfg.MigrateWorkDeptPhone || cfg.MigrateWorkDeptPage || cfg.MigrateWorkDeptEmployee || cfg.MigrateWorkTagGroupIndex || cfg.MigrateWorkTagGroupDetail || cfg.MigrateWorkTagGroupStore || cfg.MigrateWorkTagGroupUpdate || cfg.MigrateWorkTagGroupDestroy || cfg.MigrateSidebarTagGroupIndex || cfg.MigrateWorkContactTagIndex || cfg.MigrateWorkContactTagDetail || cfg.MigrateWorkContactTagList || cfg.MigrateWorkContactTagAll || cfg.MigrateWorkContactTagStore || cfg.MigrateWorkContactTagUpdate || cfg.MigrateWorkContactTagDestroy || cfg.MigrateWorkContactTagMove || cfg.MigrateWorkContactTagSync || cfg.MigrateWorkContactSync || cfg.MigrateWorkContactIndex || cfg.MigrateWorkContactLoss || cfg.MigrateWorkContactSource || cfg.MigrateWorkContactShow || cfg.MigrateWorkContactTrack || cfg.MigrateWorkContactUpdate || cfg.MigrateWorkContactBatchLabeling || cfg.MigrateWorkContactRoomIndex || cfg.MigrateWorkRoomIndex || cfg.MigrateWorkRoomRoomIndex || cfg.MigrateWorkRoomStatistics || cfg.MigrateWorkRoomStatisticsIndex || cfg.MigrateWorkRoomSync || cfg.MigrateWorkRoomBatchUpdate || cfg.MigrateSidebarWorkRoomManage || cfg.MigrateSidebarContactTagAll || cfg.MigrateSidebarContactDetail || cfg.MigrateSidebarContactShow || cfg.MigrateSidebarContactTrack || cfg.MigrateSidebarContactUpdate || cfg.MigrateSidebarProcessStatus || cfg.MigrateSidebarProcessUpdate {
		mysqlStore := getMySQLStore()
		resolver, loginCache := buildUserResolver("workRead")
		workRead := dashboard.NewWorkReadHandler(mysqlStore, loginCache, resolver, cfg.APIBaseURL)
		if cfg.MigrateWorkEmployeeIndex || cfg.MigrateWorkDeptPage || cfg.MigrateWorkDeptEmployee || cfg.MigrateWorkContactTagSync || cfg.MigrateWorkContactSync || cfg.MigrateWorkContactIndex || cfg.MigrateWorkContactLoss || cfg.MigrateWorkContactShow || cfg.MigrateWorkContactTrack || cfg.MigrateWorkContactUpdate || cfg.MigrateWorkContactRoomIndex || cfg.MigrateWorkRoomIndex || cfg.MigrateWorkRoomStatistics || cfg.MigrateWorkRoomStatisticsIndex || cfg.MigrateWorkRoomSync || cfg.MigrateWorkRoomBatchUpdate {
			workRead = dashboard.NewWorkReadHandlerWithAuthorizer(mysqlStore, loginCache, resolver, cfg.APIBaseURL, dashboard.NewRBACResolver(mysqlStore))
		}
		if cfg.MigrateWorkContactUpdate || cfg.MigrateSidebarContactUpdate {
			workRead.WithWorkContactUpdateClient(dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL))
		}
		if cfg.MigrateWorkContactTagSync {
			workRead.WithWorkContactTagSyncClient(dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL))
		}
		if cfg.MigrateWorkTagGroupDestroy || cfg.MigrateWorkContactTagStore || cfg.MigrateWorkContactTagUpdate || cfg.MigrateWorkContactTagDestroy || cfg.MigrateWorkContactTagMove {
			workRead.WithWorkContactTagWriteClient(dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL))
		}
		if cfg.MigrateWorkContactSync {
			workRead.WithWorkContactSyncClient(dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL))
		}
		if cfg.MigrateWorkRoomSync {
			workRead.WithWorkRoomSyncClient(dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL))
		}
		if cfg.MigrateWorkEmployeeSync {
			workRead.WithWorkEmployeeSyncClient(dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL), cfg.SimpleJWTSecret)
		}
		if cfg.MigrateWorkEmployeeIndex {
			options = append(options, compatserver.WithWorkEmployeeIndexHandler(http.HandlerFunc(workRead.WorkEmployeeIndex)))
			routeDebugf("go migrated route enabled: GET /dashboard/workEmployee/index")
		}
		if cfg.MigrateWorkEmployeeCond {
			options = append(options, compatserver.WithWorkEmployeeSearchConditionHandler(http.HandlerFunc(workRead.SearchCondition)))
			routeDebugf("go migrated route enabled: GET /dashboard/workEmployee/searchCondition")
		}
		if cfg.MigrateWorkEmployeeSync {
			options = append(options, compatserver.WithWorkEmployeeSyncHandler(http.HandlerFunc(workRead.WorkEmployeeSync)))
			routeDebugf("go migrated route enabled: PUT /dashboard/workEmployee/synEmployee")
		}
		if cfg.MigrateWorkDeptIndex {
			options = append(options, compatserver.WithWorkDepartmentIndexHandler(http.HandlerFunc(workRead.DepartmentIndex)))
			routeDebugf("go migrated route enabled: GET /dashboard/workDepartment/index")
		}
		if cfg.MigrateWorkDeptMember {
			options = append(options, compatserver.WithWorkEmployeeDepartmentMemberIndexHandler(http.HandlerFunc(workRead.MemberIndex)))
			routeDebugf("go migrated route enabled: GET /dashboard/workEmployeeDepartment/memberIndex")
			routeDebugf("go migrated route enabled: GET /dashboard/workDepartment/memberIndex")
		}
		if cfg.MigrateWorkDeptPhone {
			options = append(options, compatserver.WithWorkDepartmentSelectByPhoneHandler(http.HandlerFunc(workRead.SelectByPhone)))
			routeDebugf("go migrated route enabled: GET /dashboard/workDepartment/selectByPhone")
		}
		if cfg.MigrateWorkDeptPage {
			options = append(options, compatserver.WithWorkDepartmentPageIndexHandler(http.HandlerFunc(workRead.DepartmentPageIndex)))
			routeDebugf("go migrated route enabled: GET /dashboard/workDepartment/pageIndex")
		}
		if cfg.MigrateWorkDeptEmployee {
			options = append(options, compatserver.WithWorkDepartmentShowEmployeeHandler(http.HandlerFunc(workRead.DepartmentShowEmployee)))
			routeDebugf("go migrated route enabled: GET /dashboard/workDepartment/showEmployee")
		}
		if cfg.MigrateWorkTagGroupIndex {
			options = append(options, compatserver.WithWorkContactTagGroupIndexHandler(http.HandlerFunc(workRead.ContactTagGroupIndex)))
			routeDebugf("go migrated route enabled: GET /dashboard/workContactTagGroup/index")
		}
		if cfg.MigrateWorkTagGroupDetail {
			options = append(options, compatserver.WithWorkContactTagGroupDetailHandler(http.HandlerFunc(workRead.ContactTagGroupDetail)))
			routeDebugf("go migrated route enabled: GET /dashboard/workContactTagGroup/detail")
		}
		if cfg.MigrateWorkTagGroupStore {
			options = append(options, compatserver.WithWorkContactTagGroupStoreHandler(http.HandlerFunc(workRead.ContactTagGroupStore)))
			routeDebugf("go migrated route enabled: POST /dashboard/workContactTagGroup/store")
		}
		if cfg.MigrateWorkTagGroupUpdate {
			options = append(options, compatserver.WithWorkContactTagGroupUpdateHandler(http.HandlerFunc(workRead.ContactTagGroupUpdate)))
			routeDebugf("go migrated route enabled: PUT /dashboard/workContactTagGroup/update")
		}
		if cfg.MigrateWorkTagGroupDestroy {
			options = append(options, compatserver.WithWorkContactTagGroupDestroyHandler(http.HandlerFunc(workRead.ContactTagGroupDestroy)))
			routeDebugf("go migrated route enabled: DELETE /dashboard/workContactTagGroup/destroy")
		}
		if cfg.MigrateSidebarTagGroupIndex {
			workRead.WithSidebarEmployeeResolver(buildSidebarEmployeeResolver("sidebarWorkContactTagGroupIndex"))
			options = append(options, compatserver.WithSidebarWorkContactTagGroupIndexHandler(http.HandlerFunc(workRead.SidebarContactTagGroupIndex)))
			routeDebugf("go migrated route enabled: GET /sidebar/workContactTagGroup/index")
		}
		if cfg.MigrateWorkContactTagIndex {
			options = append(options, compatserver.WithWorkContactTagIndexHandler(http.HandlerFunc(workRead.ContactTagIndex)))
			routeDebugf("go migrated route enabled: GET /dashboard/workContactTag/index")
		}
		if cfg.MigrateWorkContactTagDetail {
			options = append(options, compatserver.WithWorkContactTagDetailHandler(http.HandlerFunc(workRead.ContactTagDetail)))
			routeDebugf("go migrated route enabled: GET /dashboard/workContactTag/detail")
		}
		if cfg.MigrateWorkContactTagList {
			options = append(options, compatserver.WithWorkContactTagListHandler(http.HandlerFunc(workRead.ContactTagList)))
			routeDebugf("go migrated route enabled: GET /dashboard/workContactTag/contactTagList")
		}
		if cfg.MigrateWorkContactTagAll {
			options = append(options, compatserver.WithWorkContactTagAllHandler(http.HandlerFunc(workRead.ContactTagAll)))
			routeDebugf("go migrated route enabled: GET /dashboard/workContactTag/allTag")
		}
		if cfg.MigrateWorkContactTagStore {
			options = append(options, compatserver.WithWorkContactTagStoreHandler(http.HandlerFunc(workRead.ContactTagStore)))
			routeDebugf("go migrated route enabled: POST /dashboard/workContactTag/store")
		}
		if cfg.MigrateWorkContactTagUpdate {
			options = append(options, compatserver.WithWorkContactTagUpdateHandler(http.HandlerFunc(workRead.ContactTagUpdate)))
			routeDebugf("go migrated route enabled: PUT /dashboard/workContactTag/update")
		}
		if cfg.MigrateWorkContactTagDestroy {
			options = append(options, compatserver.WithWorkContactTagDestroyHandler(http.HandlerFunc(workRead.ContactTagDestroy)))
			routeDebugf("go migrated route enabled: DELETE /dashboard/workContactTag/destroy")
		}
		if cfg.MigrateWorkContactTagMove {
			options = append(options, compatserver.WithWorkContactTagMoveHandler(http.HandlerFunc(workRead.ContactTagMove)))
			routeDebugf("go migrated route enabled: PUT /dashboard/workContactTag/move")
		}
		if cfg.MigrateWorkContactTagSync {
			options = append(options, compatserver.WithWorkContactTagSyncHandler(http.HandlerFunc(workRead.ContactTagSync)))
			routeDebugf("go migrated route enabled: PUT /dashboard/workContactTag/synContactTag")
		}
		if cfg.MigrateWorkContactSync {
			options = append(options, compatserver.WithWorkContactSyncHandler(http.HandlerFunc(workRead.WorkContactSync)))
			routeDebugf("go migrated route enabled: PUT /dashboard/workContact/synContact")
		}
		if cfg.MigrateWorkContactIndex {
			options = append(options, compatserver.WithWorkContactIndexHandler(http.HandlerFunc(workRead.WorkContactIndex)))
			routeDebugf("go migrated route enabled: GET /dashboard/workContact/index")
		}
		if cfg.MigrateWorkContactLoss {
			options = append(options, compatserver.WithWorkContactLossHandler(http.HandlerFunc(workRead.WorkContactLoss)))
			routeDebugf("go migrated route enabled: GET /dashboard/workContact/lossContact")
		}
		if cfg.MigrateWorkContactSource {
			options = append(options, compatserver.WithWorkContactSourceHandler(http.HandlerFunc(workRead.WorkContactSource)))
			routeDebugf("go migrated route enabled: GET /dashboard/workContact/source")
		}
		if cfg.MigrateWorkContactShow {
			options = append(options, compatserver.WithWorkContactShowHandler(http.HandlerFunc(workRead.WorkContactShow)))
			routeDebugf("go migrated route enabled: GET /dashboard/workContact/show")
		}
		if cfg.MigrateWorkContactTrack {
			options = append(options, compatserver.WithWorkContactTrackHandler(http.HandlerFunc(workRead.WorkContactTrack)))
			routeDebugf("go migrated route enabled: GET /dashboard/workContact/track")
		}
		if cfg.MigrateWorkContactUpdate {
			options = append(options, compatserver.WithWorkContactUpdateHandler(http.HandlerFunc(workRead.WorkContactUpdate)))
			routeDebugf("go migrated route enabled: PUT /dashboard/workContact/update")
		}
		if cfg.MigrateWorkContactBatchLabeling {
			options = append(options, compatserver.WithWorkContactBatchLabelingHandler(http.HandlerFunc(workRead.WorkContactBatchLabeling)))
			routeDebugf("go migrated route enabled: POST /dashboard/workContact/batchLabeling")
		}
		if cfg.MigrateWorkContactRoomIndex {
			options = append(options, compatserver.WithWorkContactRoomIndexHandler(http.HandlerFunc(workRead.WorkContactRoomIndex)))
			routeDebugf("go migrated route enabled: GET /dashboard/workContactRoom/index")
		}
		if cfg.MigrateWorkRoomIndex {
			options = append(options, compatserver.WithWorkRoomIndexHandler(http.HandlerFunc(workRead.WorkRoomIndex)))
			routeDebugf("go migrated route enabled: GET /dashboard/workRoom/index")
		}
		if cfg.MigrateWorkRoomRoomIndex {
			options = append(options, compatserver.WithWorkRoomRoomIndexHandler(http.HandlerFunc(workRead.WorkRoomRoomIndex)))
			routeDebugf("go migrated route enabled: GET /dashboard/workRoom/roomIndex")
		}
		if cfg.MigrateWorkRoomStatistics {
			options = append(options, compatserver.WithWorkRoomStatisticsHandler(http.HandlerFunc(workRead.WorkRoomStatistics)))
			routeDebugf("go migrated route enabled: GET /dashboard/workRoom/statistics")
		}
		if cfg.MigrateWorkRoomStatisticsIndex {
			options = append(options, compatserver.WithWorkRoomStatisticsIndexHandler(http.HandlerFunc(workRead.WorkRoomStatisticsIndex)))
			routeDebugf("go migrated route enabled: GET /dashboard/workRoom/statisticsIndex")
		}
		if cfg.MigrateWorkRoomSync {
			options = append(options, compatserver.WithWorkRoomSyncHandler(http.HandlerFunc(workRead.WorkRoomSync)))
			routeDebugf("go migrated route enabled: PUT /dashboard/workRoom/syn")
		}
		if cfg.MigrateWorkRoomBatchUpdate {
			options = append(options, compatserver.WithWorkRoomBatchUpdateHandler(http.HandlerFunc(workRead.WorkRoomBatchUpdate)))
			routeDebugf("go migrated route enabled: PUT /dashboard/workRoom/batchUpdate")
		}
		if cfg.MigrateSidebarWorkRoomManage {
			workRead.WithSidebarEmployeeResolver(buildSidebarEmployeeResolver("sidebarWorkRoomManage"))
			options = append(options, compatserver.WithSidebarWorkRoomManageHandler(http.HandlerFunc(workRead.SidebarWorkRoomManage)))
			routeDebugf("go migrated route enabled: GET /sidebar/workRoom/roomManage")
		}
		if cfg.MigrateSidebarContactTagAll {
			workRead.WithSidebarEmployeeResolver(buildSidebarEmployeeResolver("sidebarWorkContactTagAll"))
			options = append(options, compatserver.WithSidebarWorkContactTagAllHandler(http.HandlerFunc(workRead.SidebarContactTagAll)))
			routeDebugf("go migrated route enabled: GET /sidebar/workContactTag/allTag")
		}
		if cfg.MigrateSidebarContactDetail {
			workRead.WithSidebarEmployeeResolver(buildSidebarEmployeeResolver("sidebarWorkContactDetail"))
			options = append(options, compatserver.WithSidebarWorkContactDetailHandler(http.HandlerFunc(workRead.SidebarWorkContactDetail)))
			routeDebugf("go migrated route enabled: GET /sidebar/workContact/detail")
		}
		if cfg.MigrateSidebarContactShow {
			workRead.WithSidebarEmployeeResolver(buildSidebarEmployeeResolver("sidebarWorkContactShow"))
			options = append(options, compatserver.WithSidebarWorkContactShowHandler(http.HandlerFunc(workRead.SidebarWorkContactShow)))
			routeDebugf("go migrated route enabled: GET /sidebar/workContact/show")
		}
		if cfg.MigrateSidebarContactTrack {
			workRead.WithSidebarEmployeeResolver(buildSidebarEmployeeResolver("sidebarWorkContactTrack"))
			options = append(options, compatserver.WithSidebarWorkContactTrackHandler(http.HandlerFunc(workRead.SidebarWorkContactTrack)))
			routeDebugf("go migrated route enabled: GET /sidebar/workContact/track")
		}
		if cfg.MigrateSidebarContactUpdate {
			workRead.WithSidebarEmployeeResolver(buildSidebarEmployeeResolver("sidebarWorkContactUpdate"))
			options = append(options, compatserver.WithSidebarWorkContactUpdateHandler(http.HandlerFunc(workRead.SidebarWorkContactUpdate)))
			routeDebugf("go migrated route enabled: PUT /sidebar/workContact/update")
		}
		if cfg.MigrateSidebarProcessStatus {
			workRead.WithSidebarEmployeeResolver(buildSidebarEmployeeResolver("sidebarContactProcessStatusIndex"))
			options = append(options, compatserver.WithSidebarContactProcessStatusIndexHandler(http.HandlerFunc(workRead.SidebarContactProcessStatusIndex)))
			routeDebugf("go migrated route enabled: GET /sidebar/contactProcessStatus/index")
		}
		if cfg.MigrateSidebarProcessUpdate {
			workRead.WithSidebarEmployeeResolver(buildSidebarEmployeeResolver("sidebarContactProcessStatusUpdate"))
			options = append(options, compatserver.WithSidebarContactProcessStatusUpdateHandler(http.HandlerFunc(workRead.SidebarContactProcessStatusUpdate)))
			routeDebugf("go migrated route enabled: PUT /sidebar/contactProcessStatus/update")
		}
	}

	if cfg.MigrateWorkRoomGroupIndex || cfg.MigrateWorkRoomGroupStore || cfg.MigrateWorkRoomGroupUpdate || cfg.MigrateWorkRoomGroupDestroy {
		mysqlStore := getMySQLStore()
		resolver, loginCache := buildUserResolver("workRoomGroup")
		workRoomGroup := dashboard.NewWorkRoomGroupHandler(mysqlStore, loginCache, resolver)
		if cfg.MigrateWorkRoomGroupIndex {
			options = append(options, compatserver.WithWorkRoomGroupIndexHandler(http.HandlerFunc(workRoomGroup.Index)))
			routeDebugf("go migrated route enabled: GET /dashboard/workRoomGroup/index")
		}
		if cfg.MigrateWorkRoomGroupStore {
			options = append(options, compatserver.WithWorkRoomGroupStoreHandler(http.HandlerFunc(workRoomGroup.Store)))
			routeDebugf("go migrated route enabled: POST /dashboard/workRoomGroup/store")
		}
		if cfg.MigrateWorkRoomGroupUpdate {
			options = append(options, compatserver.WithWorkRoomGroupUpdateHandler(http.HandlerFunc(workRoomGroup.Update)))
			routeDebugf("go migrated route enabled: PUT /dashboard/workRoomGroup/update")
		}
		if cfg.MigrateWorkRoomGroupDestroy {
			options = append(options, compatserver.WithWorkRoomGroupDestroyHandler(http.HandlerFunc(workRoomGroup.Destroy)))
			routeDebugf("go migrated route enabled: DELETE /dashboard/workRoomGroup/destroy")
		}
	}

	if cfg.MigrateContactTransferInfo || cfg.MigrateContactTransferUnassigned || cfg.MigrateContactTransferRoom || cfg.MigrateContactTransferLog || cfg.MigrateContactTransferSync || cfg.MigrateContactTransferCustomer || cfg.MigrateContactTransferRoomStore {
		mysqlStore := getMySQLStore()
		resolver, loginCache := buildUserResolver("contactTransfer")
		contactTransfer := dashboard.NewContactTransferHandler(
			mysqlStore,
			loginCache,
			resolver,
			dashboard.NewRBACResolver(mysqlStore),
			dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL),
		)
		if cfg.MigrateContactTransferInfo {
			options = append(options, compatserver.WithContactTransferInfoHandler(http.HandlerFunc(contactTransfer.Info)))
			routeDebugf("go migrated route enabled: GET /dashboard/contactTransfer/info")
		}
		if cfg.MigrateContactTransferUnassigned {
			options = append(options, compatserver.WithContactTransferUnassignedListHandler(http.HandlerFunc(contactTransfer.UnassignedList)))
			routeDebugf("go migrated route enabled: GET /dashboard/contactTransfer/unassignedList")
		}
		if cfg.MigrateContactTransferRoom {
			options = append(options, compatserver.WithContactTransferRoomHandler(http.HandlerFunc(contactTransfer.Room)))
			routeDebugf("go migrated route enabled: GET /dashboard/contactTransfer/room")
		}
		if cfg.MigrateContactTransferLog {
			options = append(options, compatserver.WithContactTransferLogHandler(http.HandlerFunc(contactTransfer.Log)))
			routeDebugf("go migrated route enabled: GET /dashboard/contactTransfer/log")
		}
		if cfg.MigrateContactTransferSync {
			options = append(options, compatserver.WithContactTransferSaveUnassignedListHandler(http.HandlerFunc(contactTransfer.SaveUnassignedList)))
			routeDebugf("go migrated route enabled: GET /dashboard/contactTransfer/saveUnassignedList")
		}
		if cfg.MigrateContactTransferCustomer {
			options = append(options, compatserver.WithContactTransferCustomerHandler(http.HandlerFunc(contactTransfer.TransferCustomer)))
			routeDebugf("go migrated route enabled: POST /dashboard/contactTransfer/index")
		}
		if cfg.MigrateContactTransferRoomStore {
			options = append(options, compatserver.WithContactTransferRoomStoreHandler(http.HandlerFunc(contactTransfer.TransferRoom)))
			routeDebugf("go migrated route enabled: POST /dashboard/contactTransfer/room")
		}
	}

	if cfg.MigrateWorkRoomAutoPullIndex || cfg.MigrateWorkRoomAutoPullShow || cfg.MigrateWorkRoomAutoPullStore || cfg.MigrateWorkRoomAutoPullUpdate {
		mysqlStore := getMySQLStore()
		resolver, loginCache := buildUserResolver("workRoomAutoPull")
		workRoomAutoPull := dashboard.NewWorkRoomAutoPullHandlerWithContactWayClient(mysqlStore, loginCache, resolver, dashboard.NewRBACResolver(mysqlStore), cfg.APIBaseURL, dashboard.NewWorkRoomAutoPullWeComClient(cfg.WeComAPIBaseURL))
		if cfg.MigrateWorkRoomAutoPullIndex {
			options = append(options, compatserver.WithWorkRoomAutoPullIndexHandler(http.HandlerFunc(workRoomAutoPull.Index)))
			routeDebugf("go migrated route enabled: GET /dashboard/workRoomAutoPull/index")
		}
		if cfg.MigrateWorkRoomAutoPullShow {
			options = append(options, compatserver.WithWorkRoomAutoPullShowHandler(http.HandlerFunc(workRoomAutoPull.Show)))
			routeDebugf("go migrated route enabled: GET /dashboard/workRoomAutoPull/show")
		}
		if cfg.MigrateWorkRoomAutoPullStore {
			options = append(options, compatserver.WithWorkRoomAutoPullStoreHandler(http.HandlerFunc(workRoomAutoPull.Store)))
			routeDebugf("go migrated route enabled: POST /dashboard/workRoomAutoPull/store")
		}
		if cfg.MigrateWorkRoomAutoPullUpdate {
			options = append(options, compatserver.WithWorkRoomAutoPullUpdateHandler(http.HandlerFunc(workRoomAutoPull.Update)))
			options = append(options, compatserver.WithWorkRoomAutoPullMoveHandler(http.HandlerFunc(workRoomAutoPull.Move)))
			routeDebugf("go migrated route enabled: PUT /dashboard/workRoomAutoPull/update")
			routeDebugf("go migrated route enabled: PUT /dashboard/workRoomAutoPull/move")
		}
	}

	if cfg.MigrateRoomTagPullIndex || cfg.MigrateRoomTagPullShow || cfg.MigrateRoomTagPullShowContact || cfg.MigrateRoomTagPullRoomList || cfg.MigrateRoomTagPullChooseContact || cfg.MigrateRoomTagPullStore || cfg.MigrateRoomTagPullFilterContact || cfg.MigrateRoomTagPullRemindSend || cfg.MigrateRoomTagPullDestroy {
		mysqlStore := getMySQLStore()
		resolver, loginCache := buildUserResolver("roomTagPull")
		roomTagPull := dashboard.NewRoomTagPullHandlerWithMessageClient(mysqlStore, loginCache, resolver, dashboard.NewRBACResolver(mysqlStore), cfg.APIBaseURL, cfg.FileStorageRoot, dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL))
		if cfg.MigrateRoomTagPullIndex {
			options = append(options, compatserver.WithRoomTagPullIndexHandler(http.HandlerFunc(roomTagPull.Index)))
			routeDebugf("go migrated route enabled: GET /dashboard/roomTagPull/index")
		}
		if cfg.MigrateRoomTagPullShow {
			options = append(options, compatserver.WithRoomTagPullShowHandler(http.HandlerFunc(roomTagPull.Show)))
			routeDebugf("go migrated route enabled: GET /dashboard/roomTagPull/show")
		}
		if cfg.MigrateRoomTagPullShowContact {
			options = append(options,
				compatserver.WithRoomTagPullShowContactHandler(http.HandlerFunc(roomTagPull.ShowContact)),
				compatserver.WithRoomTagPullContactDetailPageHandler(dashboard.NewRoomTagPullContactDetailPageHandler()),
			)
			routeDebugf("go migrated route enabled: GET /dashboard/roomTagPull/showContact")
			routeDebugf("go migrated route enabled: GET /dashboard/roomTagPull/contactDetail")
			routeDebugf("go migrated route enabled: GET /roomTagPull/contactDetail")
			routeDebugf("go migrated route enabled: GET /roomTagPull/clientDetails")
		}
		if cfg.MigrateRoomTagPullRoomList {
			options = append(options, compatserver.WithRoomTagPullRoomListHandler(http.HandlerFunc(roomTagPull.RoomList)))
			routeDebugf("go migrated route enabled: GET /dashboard/roomTagPull/roomList")
		}
		if cfg.MigrateRoomTagPullChooseContact {
			options = append(options, compatserver.WithRoomTagPullChooseContactHandler(http.HandlerFunc(roomTagPull.ChooseContact)))
			routeDebugf("go migrated route enabled: GET /dashboard/roomTagPull/chooseContact")
		}
		if cfg.MigrateRoomTagPullStore {
			options = append(options, compatserver.WithRoomTagPullStoreHandler(http.HandlerFunc(roomTagPull.Store)))
			routeDebugf("go migrated route enabled: POST /dashboard/roomTagPull/store")
		}
		if cfg.MigrateRoomTagPullFilterContact {
			options = append(options, compatserver.WithRoomTagPullFilterContactHandler(http.HandlerFunc(roomTagPull.FilterContact)))
			routeDebugf("go migrated route enabled: POST /dashboard/roomTagPull/filterContact")
		}
		if cfg.MigrateRoomTagPullRemindSend {
			options = append(options, compatserver.WithRoomTagPullRemindSendHandler(http.HandlerFunc(roomTagPull.RemindSend)))
			routeDebugf("go migrated route enabled: GET /dashboard/roomTagPull/remindSend")
		}
		if cfg.MigrateRoomTagPullDestroy {
			options = append(options, compatserver.WithRoomTagPullDestroyHandler(http.HandlerFunc(roomTagPull.Destroy)))
			routeDebugf("go migrated route enabled: DELETE /dashboard/roomTagPull/destroy")
		}
	}

	if cfg.MigrateContactMessageBatchSendIndex || cfg.MigrateContactMessageBatchSendShow || cfg.MigrateContactMessageBatchSendShowRoom || cfg.MigrateContactMessageBatchSendEmployee || cfg.MigrateContactMessageBatchSendContactReceive || cfg.MigrateContactMessageBatchSendStore || cfg.MigrateContactMessageBatchSendRemind || cfg.MigrateContactMessageBatchSendDestroy {
		mysqlStore := getMySQLStore()
		resolver, loginCache := buildUserResolver("contactMessageBatchSend")
		contactMessageBatchSend := dashboard.NewContactMessageBatchSendHandlerWithDispatch(mysqlStore, loginCache, resolver, dashboard.NewRBACResolver(mysqlStore), cfg.APIBaseURL, cfg.FileStorageRoot, dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL))
		if cfg.MigrateContactMessageBatchSendIndex {
			options = append(options, compatserver.WithContactMessageBatchSendIndexHandler(http.HandlerFunc(contactMessageBatchSend.Index)))
			routeDebugf("go migrated route enabled: GET /dashboard/contactMessageBatchSend/index")
		}
		if cfg.MigrateContactMessageBatchSendShow {
			options = append(options, compatserver.WithContactMessageBatchSendShowHandler(http.HandlerFunc(contactMessageBatchSend.Show)))
			options = append(options, compatserver.WithContactMessageBatchSendMessageShowHandler(http.HandlerFunc(contactMessageBatchSend.MessageShow)))
			routeDebugf("go migrated route enabled: GET /dashboard/contactMessageBatchSend/show")
			routeDebugf("go migrated route enabled: GET /dashboard/contactMessageBatchSend/messageShow")
		}
		if cfg.MigrateContactMessageBatchSendShowRoom {
			options = append(options, compatserver.WithContactMessageBatchSendShowRoomHandler(http.HandlerFunc(contactMessageBatchSend.ShowRoom)))
			routeDebugf("go migrated route enabled: GET /dashboard/contactMessageBatchSend/showRoom")
		}
		if cfg.MigrateContactMessageBatchSendEmployee {
			options = append(options, compatserver.WithContactMessageBatchSendEmployeeSendIndexHandler(http.HandlerFunc(contactMessageBatchSend.EmployeeSendIndex)))
			routeDebugf("go migrated route enabled: GET /dashboard/contactMessageBatchSend/employeeSendIndex")
		}
		if cfg.MigrateContactMessageBatchSendContactReceive {
			options = append(options, compatserver.WithContactMessageBatchSendContactReceiveIndexHandler(http.HandlerFunc(contactMessageBatchSend.ContactReceiveIndex)))
			routeDebugf("go migrated route enabled: GET /dashboard/contactMessageBatchSend/contactReceiveIndex")
		}
		if cfg.MigrateContactMessageBatchSendStore {
			options = append(options, compatserver.WithContactMessageBatchSendStoreHandler(http.HandlerFunc(contactMessageBatchSend.Store)))
			routeDebugf("go migrated route enabled: POST /dashboard/contactMessageBatchSend/store")
		}
		if cfg.MigrateContactMessageBatchSendRemind {
			options = append(options, compatserver.WithContactMessageBatchSendRemindHandler(http.HandlerFunc(contactMessageBatchSend.Remind)))
			routeDebugf("go migrated route enabled: POST /dashboard/contactMessageBatchSend/remind")
		}
		if cfg.MigrateContactMessageBatchSendDestroy {
			options = append(options, compatserver.WithContactMessageBatchSendDestroyHandler(http.HandlerFunc(contactMessageBatchSend.Destroy)))
			routeDebugf("go migrated route enabled: DELETE /dashboard/contactMessageBatchSend/destroy")
		}
	}

	if cfg.MigrateRoomMessageBatchSendIndex || cfg.MigrateRoomMessageBatchSendShow || cfg.MigrateRoomMessageBatchSendOwner || cfg.MigrateRoomMessageBatchSendRoomReceive || cfg.MigrateRoomMessageBatchSendStore || cfg.MigrateRoomMessageBatchSendRemind || cfg.MigrateRoomMessageBatchSendDestroy {
		mysqlStore := getMySQLStore()
		resolver, loginCache := buildUserResolver("roomMessageBatchSend")
		roomMessageBatchSend := dashboard.NewRoomMessageBatchSendHandlerWithDispatch(mysqlStore, loginCache, resolver, dashboard.NewRBACResolver(mysqlStore), cfg.APIBaseURL, cfg.FileStorageRoot, dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL))
		if cfg.MigrateRoomMessageBatchSendIndex {
			options = append(options, compatserver.WithRoomMessageBatchSendIndexHandler(http.HandlerFunc(roomMessageBatchSend.Index)))
			routeDebugf("go migrated route enabled: GET /dashboard/roomMessageBatchSend/index")
		}
		if cfg.MigrateRoomMessageBatchSendShow {
			options = append(options, compatserver.WithRoomMessageBatchSendShowHandler(http.HandlerFunc(roomMessageBatchSend.Show)))
			routeDebugf("go migrated route enabled: GET /dashboard/roomMessageBatchSend/show")
		}
		if cfg.MigrateRoomMessageBatchSendOwner {
			options = append(options, compatserver.WithRoomMessageBatchSendRoomOwnerSendIndexHandler(http.HandlerFunc(roomMessageBatchSend.RoomOwnerSendIndex)))
			routeDebugf("go migrated route enabled: GET /dashboard/roomMessageBatchSend/roomOwnerSendIndex")
		}
		if cfg.MigrateRoomMessageBatchSendRoomReceive {
			options = append(options, compatserver.WithRoomMessageBatchSendRoomReceiveIndexHandler(http.HandlerFunc(roomMessageBatchSend.RoomReceiveIndex)))
			routeDebugf("go migrated route enabled: GET /dashboard/roomMessageBatchSend/roomReceiveIndex")
		}
		if cfg.MigrateRoomMessageBatchSendStore {
			options = append(options, compatserver.WithRoomMessageBatchSendStoreHandler(http.HandlerFunc(roomMessageBatchSend.Store)))
			routeDebugf("go migrated route enabled: POST /dashboard/roomMessageBatchSend/store")
		}
		if cfg.MigrateRoomMessageBatchSendRemind {
			options = append(options, compatserver.WithRoomMessageBatchSendRemindHandler(http.HandlerFunc(roomMessageBatchSend.Remind)))
			routeDebugf("go migrated route enabled: GET /dashboard/roomMessageBatchSend/remind")
		}
		if cfg.MigrateRoomMessageBatchSendDestroy {
			options = append(options, compatserver.WithRoomMessageBatchSendDestroyHandler(http.HandlerFunc(roomMessageBatchSend.Destroy)))
			routeDebugf("go migrated route enabled: DELETE /dashboard/roomMessageBatchSend/destroy")
		}
	}

	if cfg.MigrateOfficialAccountIndex || cfg.MigrateOfficialAccountSet {
		mysqlStore := getMySQLStore()
		resolver, loginCache := buildUserResolver("officialAccount")
		officialAccount := dashboard.NewOfficialAccountHandler(mysqlStore, loginCache, resolver, dashboard.NewRBACResolver(mysqlStore), cfg.APIBaseURL)
		if cfg.MigrateOfficialAccountIndex {
			options = append(options, compatserver.WithOfficialAccountIndexHandler(http.HandlerFunc(officialAccount.Index)))
			routeDebugf("go migrated route enabled: GET /dashboard/officialAccount/index")
		}
		if cfg.MigrateOfficialAccountSet {
			options = append(options, compatserver.WithOfficialAccountSetHandler(http.HandlerFunc(officialAccount.Set)))
			routeDebugf("go migrated route enabled: GET /dashboard/officialAccount/set")
		}
	}

	if cfg.MigrateOfficialAccountGetPreAuthURL || cfg.MigrateOfficialAccountAuthRedirect {
		mysqlStore := getMySQLStore()
		resolver, loginCache := buildUserResolver("officialAccountAuthorization")
		officialAccountAuthorization := dashboard.NewOfficialAccountAuthorizationHandler(
			mysqlStore,
			loginCache,
			resolver,
			dashboard.NewRBACResolver(mysqlStore),
			dashboard.NewOfficialAccountOAuthClient(
				cfg.WeChatAPIBaseURL,
				cfg.WeChatOpenPlatformAppID,
				cfg.WeChatOpenPlatformSecret,
				cfg.WeChatComponentVerifyTicket,
			).WithComponentVerifyTicketProvider(mysqlStore),
			cfg.DashboardBaseURL,
			cfg.WeChatOpenPlatformAppID,
			cfg.WeChatOpenPlatformSecret,
			cfg.WeChatOpenPlatformToken,
			cfg.WeChatOpenPlatformAESKey,
		)
		if cfg.MigrateOfficialAccountGetPreAuthURL {
			options = append(options, compatserver.WithOfficialAccountGetPreAuthURLHandler(http.HandlerFunc(officialAccountAuthorization.GetPreAuthURL)))
			routeDebugf("go migrated route enabled: GET /dashboard/officialAccount/getPreAuthUrl")
		}
		if cfg.MigrateOfficialAccountAuthRedirect {
			options = append(options, compatserver.WithOfficialAccountAuthRedirectHandler(http.HandlerFunc(officialAccountAuthorization.AuthRedirect)))
			routeDebugf("go migrated route enabled: GET/POST /dashboard/officialAccount/authRedirect/")
		}
	}

	if cfg.MigrateOfficialAccountAuthEventCallback || cfg.MigrateOfficialAccountMessageEventCallback {
		mysqlStore := getMySQLStore()
		var authEventStore dashboard.OfficialAccountAuthEventStore
		if cfg.MigrateOfficialAccountAuthEventCallback {
			authEventStore = mysqlStore
		}
		officialAccountCallbacks := dashboard.NewOfficialAccountCallbackHandler(
			authEventStore,
			dashboard.NewOfficialAccountOAuthClient(
				cfg.WeChatAPIBaseURL,
				cfg.WeChatOpenPlatformAppID,
				cfg.WeChatOpenPlatformSecret,
				cfg.WeChatComponentVerifyTicket,
			).WithComponentVerifyTicketProvider(mysqlStore),
			cfg.WeChatOpenPlatformAppID,
			cfg.WeChatOpenPlatformSecret,
			cfg.WeChatOpenPlatformToken,
			cfg.WeChatOpenPlatformAESKey,
		)
		if cfg.MigrateOfficialAccountAuthEventCallback {
			options = append(options, compatserver.WithOfficialAccountAuthEventCallbackHandler(http.HandlerFunc(officialAccountCallbacks.AuthEventCallback)))
			routeDebugf("go migrated route enabled: GET/POST /dashboard/officialAccount/authEventCallback")
		}
		if cfg.MigrateOfficialAccountMessageEventCallback {
			options = append(options, compatserver.WithOfficialAccountMessageEventCallbackHandler(http.HandlerFunc(officialAccountCallbacks.MessageEventCallback)))
			routeDebugf("go migrated route enabled: GET/POST /dashboard/{appId}/officialAccount/messageEventCallback")
		}
	}

	if cfg.MigrateWorkFissionIndex || cfg.MigrateWorkFissionShow || cfg.MigrateWorkFissionInfo || cfg.MigrateWorkFissionStatistics || cfg.MigrateWorkFissionChooseContact || cfg.MigrateWorkFissionStore || cfg.MigrateWorkFissionUpdate || cfg.MigrateWorkFissionInvite || cfg.MigrateWorkFissionInviteData || cfg.MigrateWorkFissionInviteDetail || cfg.MigrateWorkFissionDestroy {
		mysqlStore := getMySQLStore()
		resolver, loginCache := buildUserResolver("workFission")
		workFission := dashboard.NewWorkFissionHandlerWithMessageClient(mysqlStore, loginCache, resolver, dashboard.NewRBACResolver(mysqlStore), cfg.APIBaseURL, cfg.OperationBaseURL, cfg.FileStorageRoot, dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL))
		if cfg.MigrateWorkFissionIndex {
			options = append(options, compatserver.WithWorkFissionIndexHandler(http.HandlerFunc(workFission.Index)))
			routeDebugf("go migrated route enabled: GET /dashboard/workFission/index")
		}
		if cfg.MigrateWorkFissionShow {
			options = append(options, compatserver.WithWorkFissionShowHandler(http.HandlerFunc(workFission.Show)))
			routeDebugf("go migrated route enabled: GET /dashboard/workFission/show")
		}
		if cfg.MigrateWorkFissionInfo {
			options = append(options, compatserver.WithWorkFissionInfoHandler(http.HandlerFunc(workFission.Info)))
			routeDebugf("go migrated route enabled: GET /dashboard/workFission/info")
		}
		if cfg.MigrateWorkFissionStatistics {
			options = append(options, compatserver.WithWorkFissionStatisticsHandler(http.HandlerFunc(workFission.Statistics)))
			routeDebugf("go migrated route enabled: GET /dashboard/workFission/statistics")
		}
		if cfg.MigrateWorkFissionChooseContact {
			options = append(options, compatserver.WithWorkFissionChooseContactHandler(http.HandlerFunc(workFission.ChooseContact)))
			routeDebugf("go migrated route enabled: GET /dashboard/workFission/chooseContact")
		}
		if cfg.MigrateWorkFissionStore {
			options = append(options, compatserver.WithWorkFissionStoreHandler(http.HandlerFunc(workFission.Store)))
			routeDebugf("go migrated route enabled: POST /dashboard/workFission/store")
		}
		if cfg.MigrateWorkFissionUpdate {
			options = append(options, compatserver.WithWorkFissionUpdateHandler(http.HandlerFunc(workFission.Update)))
			routeDebugf("go migrated route enabled: PUT /dashboard/workFission/update")
		}
		if cfg.MigrateWorkFissionInvite {
			options = append(options, compatserver.WithWorkFissionInviteHandler(http.HandlerFunc(workFission.Invite)))
			routeDebugf("go migrated route enabled: POST /dashboard/workFission/invite")
		}
		if cfg.MigrateWorkFissionInviteData {
			options = append(options, compatserver.WithWorkFissionInviteDataHandler(http.HandlerFunc(workFission.InviteData)))
			routeDebugf("go migrated route enabled: GET /dashboard/workFission/inviteData")
		}
		if cfg.MigrateWorkFissionInviteDetail {
			options = append(options, compatserver.WithWorkFissionInviteDetailHandler(http.HandlerFunc(workFission.InviteDetail)))
			routeDebugf("go migrated route enabled: GET /dashboard/workFission/inviteDetail")
		}
		if cfg.MigrateWorkFissionDestroy {
			options = append(options, compatserver.WithWorkFissionDestroyHandler(http.HandlerFunc(workFission.Destroy)))
			routeDebugf("go migrated route enabled: DELETE /dashboard/workFission/destroy")
		}
	}

	if cfg.MigrateOperationWorkFissionInviteFriends || cfg.MigrateOperationWorkFissionPoster || cfg.MigrateOperationWorkFissionTaskData || cfg.MigrateOperationWorkFissionReceive {
		mysqlStore := getMySQLStore()
		operationWorkFission := dashboard.NewWorkFissionOperationHandler(mysqlStore, cfg.APIBaseURL, dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL))
		if cfg.MigrateOperationWorkFissionInviteFriends {
			options = append(options, compatserver.WithOperationWorkFissionInviteFriendsHandler(http.HandlerFunc(operationWorkFission.InviteFriends)))
			routeDebugf("go migrated route enabled: GET /operation/workFission/inviteFriends")
		}
		if cfg.MigrateOperationWorkFissionPoster {
			options = append(options, compatserver.WithOperationWorkFissionPosterHandler(http.HandlerFunc(operationWorkFission.Poster)))
			routeDebugf("go migrated route enabled: GET /operation/workFission/poster")
		}
		if cfg.MigrateOperationWorkFissionTaskData {
			options = append(options, compatserver.WithOperationWorkFissionTaskDataHandler(http.HandlerFunc(operationWorkFission.TaskData)))
			routeDebugf("go migrated route enabled: GET /operation/workFission/taskData")
		}
		if cfg.MigrateOperationWorkFissionReceive {
			options = append(options, compatserver.WithOperationWorkFissionReceiveHandler(http.HandlerFunc(operationWorkFission.Receive)))
			routeDebugf("go migrated route enabled: PUT /operation/workFission/receive")
		}
	}

	if cfg.MigrateOperationWorkFissionAuth || cfg.MigrateOperationWorkFissionOpenUserInfo {
		mysqlStore := getMySQLStore()
		redisStore := getRedisStore()
		operationWorkFissionOAuth := dashboard.NewWorkFissionOAuthHandler(
			mysqlStore,
			redisStore,
			dashboard.NewOfficialAccountOAuthClient(
				cfg.WeChatAPIBaseURL,
				cfg.WeChatOpenPlatformAppID,
				cfg.WeChatOpenPlatformSecret,
				cfg.WeChatComponentVerifyTicket,
			).WithComponentVerifyTicketProvider(mysqlStore),
			cfg.OperationBaseURL,
		)
		if cfg.MigrateOperationWorkFissionAuth {
			options = append(options, compatserver.WithOperationWorkFissionAuthHandler(http.HandlerFunc(operationWorkFissionOAuth.Auth)))
			routeDebugf("go migrated route enabled: GET/POST /operation/auth/workFission")
		}
		if cfg.MigrateOperationWorkFissionOpenUserInfo {
			options = append(options, compatserver.WithOperationWorkFissionOpenUserInfoHandler(http.HandlerFunc(operationWorkFissionOAuth.OpenUserInfo)))
			routeDebugf("go migrated route enabled: GET /operation/openUserInfo/workFission")
		}
	}

	if cfg.MigrateLotteryDashboard || cfg.MigrateRoomClockInDashboard || cfg.MigrateRoomFissionDashboard || cfg.MigrateRoomInfinitePullDashboard || cfg.MigrateShopCodeDashboard {
		mysqlStore := getMySQLStore()
		var redisStore dashboard.OperationSessionStore
		if cfg.MigrateLotteryDashboard || cfg.MigrateRoomClockInDashboard || cfg.MigrateRoomFissionDashboard || cfg.MigrateShopCodeDashboard {
			redisStore = getRedisStore()
		}
		operationH5 := dashboard.NewOperationH5Handler(
			mysqlStore,
			redisStore,
			dashboard.NewOfficialAccountOAuthClient(
				cfg.WeChatAPIBaseURL,
				cfg.WeChatOpenPlatformAppID,
				cfg.WeChatOpenPlatformSecret,
				cfg.WeChatComponentVerifyTicket,
			).WithComponentVerifyTicketProvider(mysqlStore),
			cfg.APIBaseURL,
			cfg.OperationBaseURL,
		)
		if cfg.MigrateLotteryDashboard {
			options = append(options,
				compatserver.WithOperationLotteryContactDataHandler(http.HandlerFunc(operationH5.LotteryContactData)),
				compatserver.WithOperationLotteryContactLotteryHandler(http.HandlerFunc(operationH5.LotteryContactLottery)),
				compatserver.WithOperationLotteryReceiveHandler(http.HandlerFunc(operationH5.LotteryReceive)),
				compatserver.WithOperationLotteryAuthHandler(http.HandlerFunc(operationH5.AuthLottery)),
				compatserver.WithOperationLotteryOpenUserInfoHandler(http.HandlerFunc(operationH5.OpenUserInfoLottery)),
			)
			routeDebugf("go migrated route enabled: POST /operation/lottery/contactData")
			routeDebugf("go migrated route enabled: PUT /operation/lottery/contactLottery")
			routeDebugf("go migrated route enabled: PUT /operation/lottery/receive")
			routeDebugf("go migrated route enabled: GET/POST /operation/auth/lottery")
			routeDebugf("go migrated route enabled: GET /operation/openUserInfo/lottery")
		}
		if cfg.MigrateRoomClockInDashboard {
			options = append(options,
				compatserver.WithOperationRoomClockInContactDataHandler(http.HandlerFunc(operationH5.RoomClockInContactData)),
				compatserver.WithOperationRoomClockInClockInRankingHandler(http.HandlerFunc(operationH5.RoomClockInRanking)),
				compatserver.WithOperationRoomClockInContactClockInHandler(http.HandlerFunc(operationH5.RoomClockInContactClockIn)),
				compatserver.WithOperationRoomClockInReceiveHandler(http.HandlerFunc(operationH5.RoomClockInReceive)),
				compatserver.WithOperationRoomClockInAuthHandler(http.HandlerFunc(operationH5.AuthRoomClockIn)),
				compatserver.WithOperationRoomClockInOpenUserInfoHandler(http.HandlerFunc(operationH5.OpenUserInfoRoomClockIn)),
			)
			routeDebugf("go migrated route enabled: GET /operation/roomClockIn/contactData")
			routeDebugf("go migrated route enabled: GET /operation/roomClockIn/clockInRanking")
			routeDebugf("go migrated route enabled: PUT /operation/roomClockIn/contactClockIn")
			routeDebugf("go migrated route enabled: PUT /operation/roomClockIn/receive")
			routeDebugf("go migrated route enabled: GET/POST /operation/auth/roomClockIn")
			routeDebugf("go migrated route enabled: GET /operation/openUserInfo/roomClockIn")
		}
		if cfg.MigrateRoomFissionDashboard {
			options = append(options,
				compatserver.WithOperationRoomFissionPosterHandler(http.HandlerFunc(operationH5.RoomFissionPoster)),
				compatserver.WithOperationRoomFissionInviteFriendsHandler(http.HandlerFunc(operationH5.RoomFissionInviteFriends)),
				compatserver.WithOperationRoomFissionReceiveHandler(http.HandlerFunc(operationH5.RoomFissionReceive)),
				compatserver.WithOperationRoomFissionAuthHandler(http.HandlerFunc(operationH5.AuthRoomFission)),
				compatserver.WithOperationRoomFissionOpenUserInfoHandler(http.HandlerFunc(operationH5.OpenUserInfoRoomFission)),
			)
			routeDebugf("go migrated route enabled: GET /operation/roomFission/poster")
			routeDebugf("go migrated route enabled: GET /operation/roomFission/inviteFriends")
			routeDebugf("go migrated route enabled: GET /operation/roomFission/receive")
			routeDebugf("go migrated route enabled: GET/POST /operation/auth/roomFission")
			routeDebugf("go migrated route enabled: GET /operation/openUserInfo/roomFission")
		}
		if cfg.MigrateRoomInfinitePullDashboard {
			options = append(options, compatserver.WithOperationRoomInfinitePullQRCodeHandler(http.HandlerFunc(operationH5.RoomInfinitePullQRCode)))
			routeDebugf("go migrated route enabled: GET /operation/roomInfinitePull/qrCode")
		}
		if cfg.MigrateShopCodeDashboard {
			options = append(options,
				compatserver.WithOperationShopCodeAreaCodeHandler(http.HandlerFunc(operationH5.ShopCodeAreaCode)),
				compatserver.WithOperationShopCodeWeChatSDKConfigHandler(http.HandlerFunc(operationH5.ShopCodeWeChatSDKConfig)),
				compatserver.WithOperationShopCodeAuthHandler(http.HandlerFunc(operationH5.AuthShopCode)),
				compatserver.WithOperationShopCodeOpenUserInfoHandler(http.HandlerFunc(operationH5.OpenUserInfoShopCode)),
			)
			routeDebugf("go migrated route enabled: GET /operation/shopCode/areaCode")
			routeDebugf("go migrated route enabled: GET /operation/shopCode/weChatSdkConfig")
			routeDebugf("go migrated route enabled: GET/POST /operation/auth/shopCode")
			routeDebugf("go migrated route enabled: GET /operation/openUserInfo/shopCode")
		}
	}

	if cfg.MigrateLoad {
		mysqlStore := getMySQLStore()
		redisStore := getRedisStore()
		loadOAuth := dashboard.NewOperationLoadOAuthHandler(
			mysqlStore,
			redisStore,
			dashboard.NewOfficialAccountOAuthClient(
				cfg.WeChatAPIBaseURL,
				cfg.WeChatOpenPlatformAppID,
				cfg.WeChatOpenPlatformSecret,
				cfg.WeChatComponentVerifyTicket,
			).WithComponentVerifyTicketProvider(mysqlStore),
			cfg.OperationBaseURL,
		)
		options = append(options, compatserver.WithLoadHandler(http.HandlerFunc(loadOAuth.Load)))
		routeDebugf("go migrated route enabled: GET/POST /load/{params?}")
	}

	if cfg.MigrateMediumIndex || cfg.MigrateMediumShow || cfg.MigrateMediumStore || cfg.MigrateMediumUpdate || cfg.MigrateMediumDestroy || cfg.MigrateMediumItemGroupUpdate || cfg.MigrateSidebarMediumIndex || cfg.MigrateSidebarMediumMediaIDUpdate {
		mysqlStore := getMySQLStore()
		resolver, loginCache := buildUserResolver("medium")
		medium := dashboard.NewMediumHandlerWithMediaClient(mysqlStore, loginCache, resolver, dashboard.NewRBACResolver(mysqlStore), cfg.APIBaseURL, cfg.FileStorageRoot, dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL))
		if cfg.MigrateSidebarMediumIndex || cfg.MigrateSidebarMediumMediaIDUpdate {
			medium.WithSidebarEmployeeResolver(buildSidebarEmployeeResolver("sidebarMedium"))
		}
		if cfg.MigrateMediumIndex {
			options = append(options, compatserver.WithMediumIndexHandler(http.HandlerFunc(medium.Index)))
			routeDebugf("go migrated route enabled: GET /dashboard/medium/index")
		}
		if cfg.MigrateMediumShow {
			options = append(options, compatserver.WithMediumShowHandler(http.HandlerFunc(medium.Show)))
			routeDebugf("go migrated route enabled: GET /dashboard/medium/show")
		}
		if cfg.MigrateMediumStore {
			options = append(options, compatserver.WithMediumStoreHandler(http.HandlerFunc(medium.Store)))
			routeDebugf("go migrated route enabled: POST /dashboard/medium/store")
		}
		if cfg.MigrateMediumUpdate {
			options = append(options, compatserver.WithMediumUpdateHandler(http.HandlerFunc(medium.Update)))
			routeDebugf("go migrated route enabled: PUT /dashboard/medium/update")
		}
		if cfg.MigrateMediumDestroy {
			options = append(options, compatserver.WithMediumDestroyHandler(http.HandlerFunc(medium.Destroy)))
			routeDebugf("go migrated route enabled: DELETE /dashboard/medium/destroy")
		}
		if cfg.MigrateMediumItemGroupUpdate {
			options = append(options, compatserver.WithMediumItemGroupUpdateHandler(http.HandlerFunc(medium.GroupUpdate)))
			routeDebugf("go migrated route enabled: PUT /dashboard/medium/groupUpdate")
		}
		options = append(options,
			compatserver.WithMediumBatchGroupUpdateHandler(http.HandlerFunc(medium.BatchGroupUpdate)),
			compatserver.WithMediumReferenceCheckHandler(http.HandlerFunc(medium.ReferenceCheck)),
			compatserver.WithMediumBatchDestroyHandler(http.HandlerFunc(medium.BatchDestroy)),
			compatserver.WithMaterialSelectorIndexHandler(http.HandlerFunc(medium.Selector)),
		)
		debugf("go material foundation routes enabled")
		if cfg.MigrateSidebarMediumIndex {
			options = append(options, compatserver.WithSidebarMediumIndexHandler(http.HandlerFunc(medium.SidebarIndex)))
			routeDebugf("go migrated route enabled: GET /sidebar/medium/index")
		}
		if cfg.MigrateSidebarMediumMediaIDUpdate {
			options = append(options, compatserver.WithSidebarMediumMediaIDUpdateHandler(http.HandlerFunc(medium.MediaIDUpdate)))
			routeDebugf("go migrated route enabled: GET /sidebar/medium/mediaIdUpdate")
		}
	}

	if cfg.MigrateMediumGroupIndex || cfg.MigrateMediumGroupStore || cfg.MigrateMediumGroupUpdate || cfg.MigrateMediumGroupDestroy || cfg.MigrateSidebarMediumGroupIndex {
		mysqlStore := getMySQLStore()
		resolver, loginCache := buildUserResolver("mediumGroup")
		mediumGroup := dashboard.NewMediumGroupHandler(mysqlStore, loginCache, resolver, dashboard.NewRBACResolver(mysqlStore))
		if cfg.MigrateMediumGroupIndex {
			options = append(options, compatserver.WithMediumGroupIndexHandler(http.HandlerFunc(mediumGroup.Index)))
			routeDebugf("go migrated route enabled: GET /dashboard/mediumGroup/index")
		}
		if cfg.MigrateMediumGroupStore {
			options = append(options, compatserver.WithMediumGroupStoreHandler(http.HandlerFunc(mediumGroup.Store)))
			routeDebugf("go migrated route enabled: POST /dashboard/mediumGroup/store")
		}
		if cfg.MigrateMediumGroupUpdate {
			options = append(options, compatserver.WithMediumGroupUpdateHandler(http.HandlerFunc(mediumGroup.Update)))
			routeDebugf("go migrated route enabled: PUT /dashboard/mediumGroup/update")
		}
		if cfg.MigrateMediumGroupDestroy {
			options = append(options, compatserver.WithMediumGroupDestroyHandler(http.HandlerFunc(mediumGroup.Destroy)))
			routeDebugf("go migrated route enabled: DELETE /dashboard/mediumGroup/destroy")
		}
		if cfg.MigrateSidebarMediumGroupIndex {
			mediumGroup.WithSidebarEmployeeResolver(buildSidebarEmployeeResolver("sidebarMediumGroupIndex"))
			options = append(options, compatserver.WithSidebarMediumGroupIndexHandler(http.HandlerFunc(mediumGroup.SidebarIndex)))
			routeDebugf("go migrated route enabled: GET /sidebar/mediumGroup/index")
		}
	}

	if cfg.MigrateFriendsCircleProvider {
		mysqlStore := getMySQLStore()
		resolver, loginCache := buildUserResolver("friendsCircle")
		friendsCircle := dashboard.NewFriendsCircleHandlerWithCallbackToken(mysqlStore, loginCache, resolver, dashboard.NewRBACResolver(mysqlStore), dashboard.NewUnavailableFriendsCirclePublisher(), cfg.FriendsCircleCallbackToken)
		options = append(options,
			compatserver.WithFriendsCircleTaskIndexHandler(http.HandlerFunc(friendsCircle.TaskIndex)),
			compatserver.WithFriendsCircleMaterialIndexHandler(http.HandlerFunc(friendsCircle.MaterialIndex)),
			compatserver.WithFriendsCircleTaskStoreHandler(http.HandlerFunc(friendsCircle.TaskStore)),
			compatserver.WithFriendsCircleMaterialStoreHandler(http.HandlerFunc(friendsCircle.MaterialStore)),
			compatserver.WithFriendsCirclePublishHandler(http.HandlerFunc(friendsCircle.Publish)),
			compatserver.WithFriendsCircleTaskResultIndexHandler(http.HandlerFunc(friendsCircle.TaskResultIndex)),
			compatserver.WithFriendsCircleExportHandler(http.HandlerFunc(friendsCircle.Export)),
			compatserver.WithFriendsCircleExportDataHandler(http.HandlerFunc(friendsCircle.ExportData)),
			compatserver.WithFriendsCircleProviderCallbackHandler(http.HandlerFunc(friendsCircle.ProviderCallback)),
		)
		routeDebugf("go migrated routes enabled: friends circle provider (external publisher fail-closed; callback token configured=%t)", cfg.FriendsCircleCallbackToken != "")

		phase34Acquisition := dashboard.NewPhase34AcquisitionHandler(
			mysqlStore,
			loginCache,
			resolver,
			dashboard.NewRBACResolver(mysqlStore),
			dashboard.NewPhase34UnavailableExternalProvider(),
		)
		options = append(options,
			compatserver.WithPhase34AcquisitionLinkIndexHandler(http.HandlerFunc(phase34Acquisition.AcquisitionLinkIndex)),
			compatserver.WithPhase34AcquisitionLinkStoreHandler(http.HandlerFunc(phase34Acquisition.AcquisitionLinkStore)),
			compatserver.WithPhase34AcquisitionLinkAuthorizeHandler(http.HandlerFunc(phase34Acquisition.AcquisitionLinkAuthorize)),
			compatserver.WithPhase34CustomerServiceIndexHandler(http.HandlerFunc(phase34Acquisition.CustomerServiceIndex)),
			compatserver.WithPhase34CustomerServiceStoreHandler(http.HandlerFunc(phase34Acquisition.CustomerServiceStore)),
			compatserver.WithPhase34CustomerServiceSyncHandler(http.HandlerFunc(phase34Acquisition.CustomerServiceSync)),
			compatserver.WithPhase34ShortLinkIndexHandler(http.HandlerFunc(phase34Acquisition.ShortLinkIndex)),
			compatserver.WithPhase34ShortLinkStoreHandler(http.HandlerFunc(phase34Acquisition.ShortLinkStore)),
			compatserver.WithPhase34ShortLinkDisableHandler(http.HandlerFunc(phase34Acquisition.ShortLinkDisable)),
			compatserver.WithPhase34ShortLinkRedirectHandler(http.HandlerFunc(phase34Acquisition.ShortLinkRedirect)),
		)
		routeDebugf("go migrated routes enabled: phase34 acquisition provider (external adapter fail-closed)")
	}

	if cfg.MigrateChannelCodeIndex || cfg.MigrateChannelCodeShow || cfg.MigrateChannelCodeContact || cfg.MigrateChannelCodeStatistics || cfg.MigrateChannelCodeStatsIndex || cfg.MigrateChannelCodeStore || cfg.MigrateChannelCodeUpdate ||
		cfg.MigrateChannelCodeGroupIndex || cfg.MigrateChannelCodeGroupDetail || cfg.MigrateChannelCodeGroupStore || cfg.MigrateChannelCodeGroupUpdate || cfg.MigrateChannelCodeGroupMove {
		mysqlStore := getMySQLStore()
		resolver, loginCache := buildUserResolver("channelCode")
		channelCode := dashboard.NewChannelCodeHandlerWithAuthorizer(mysqlStore, loginCache, resolver, dashboard.NewRBACResolver(mysqlStore), cfg.APIBaseURL, dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL))
		if cfg.MigrateChannelCodeIndex {
			options = append(options, compatserver.WithChannelCodeIndexHandler(http.HandlerFunc(channelCode.Index)))
			routeDebugf("go migrated route enabled: GET /dashboard/channelCode/index")
		}
		if cfg.MigrateChannelCodeShow {
			options = append(options, compatserver.WithChannelCodeShowHandler(http.HandlerFunc(channelCode.Show)))
			routeDebugf("go migrated route enabled: GET /dashboard/channelCode/show")
		}
		if cfg.MigrateChannelCodeContact {
			options = append(options, compatserver.WithChannelCodeContactHandler(http.HandlerFunc(channelCode.Contact)))
			routeDebugf("go migrated route enabled: GET /dashboard/channelCode/contact")
		}
		if cfg.MigrateChannelCodeStatistics {
			options = append(options, compatserver.WithChannelCodeStatisticsHandler(http.HandlerFunc(channelCode.Statistics)))
			routeDebugf("go migrated route enabled: GET /dashboard/channelCode/statistics")
		}
		if cfg.MigrateChannelCodeStatsIndex {
			options = append(options, compatserver.WithChannelCodeStatisticsIndexHandler(http.HandlerFunc(channelCode.StatisticsIndex)))
			routeDebugf("go migrated route enabled: GET /dashboard/channelCode/statisticsIndex")
		}
		if cfg.MigrateChannelCodeStore {
			options = append(options, compatserver.WithChannelCodeStoreHandler(http.HandlerFunc(channelCode.Store)))
			routeDebugf("go migrated route enabled: POST /dashboard/channelCode/store")
		}
		if cfg.MigrateChannelCodeUpdate {
			options = append(options, compatserver.WithChannelCodeUpdateHandler(http.HandlerFunc(channelCode.Update)))
			routeDebugf("go migrated route enabled: PUT /dashboard/channelCode/update")
		}
		options = append(options, compatserver.WithChannelCodeBatchInvalidateHandler(http.HandlerFunc(channelCode.BatchInvalidate)))
		routeDebugf("go migrated route enabled: POST /dashboard/channelCode/batchInvalidate")
		options = append(options,
			compatserver.WithChannelCodeWorkspaceStatisticsHandler(http.HandlerFunc(channelCode.WorkspaceStatistics)),
			compatserver.WithChannelCodeWorkspaceStatisticsIndexHandler(http.HandlerFunc(channelCode.WorkspaceStatisticsIndex)),
			compatserver.WithChannelCodeExportHandler(http.HandlerFunc(channelCode.ExportWorkspaceStatistics)),
		)
		routeDebugf("go migrated route enabled: GET /dashboard/channelCode/workspaceStatistics")
		routeDebugf("go migrated route enabled: GET /dashboard/channelCode/workspaceStatisticsIndex")
		routeDebugf("go migrated route enabled: GET /dashboard/channelCode/export")
		if cfg.MigrateChannelCodeGroupIndex {
			options = append(options, compatserver.WithChannelCodeGroupIndexHandler(http.HandlerFunc(channelCode.GroupIndex)))
			routeDebugf("go migrated route enabled: GET /dashboard/channelCodeGroup/index")
		}
		if cfg.MigrateChannelCodeGroupDetail {
			options = append(options, compatserver.WithChannelCodeGroupDetailHandler(http.HandlerFunc(channelCode.GroupDetail)))
			routeDebugf("go migrated route enabled: GET /dashboard/channelCodeGroup/detail")
		}
		if cfg.MigrateChannelCodeGroupStore {
			options = append(options, compatserver.WithChannelCodeGroupStoreHandler(http.HandlerFunc(channelCode.GroupStore)))
			routeDebugf("go migrated route enabled: POST /dashboard/channelCodeGroup/store")
		}
		if cfg.MigrateChannelCodeGroupUpdate {
			options = append(options, compatserver.WithChannelCodeGroupUpdateHandler(http.HandlerFunc(channelCode.GroupUpdate)))
			routeDebugf("go migrated route enabled: PUT /dashboard/channelCodeGroup/update")
		}
		if cfg.MigrateChannelCodeGroupMove {
			options = append(options, compatserver.WithChannelCodeGroupMoveHandler(http.HandlerFunc(channelCode.GroupMove)))
			routeDebugf("go migrated route enabled: PUT /dashboard/channelCodeGroup/move")
		}
	}

	if cfg.MigrateGreetingIndex || cfg.MigrateGreetingShow || cfg.MigrateGreetingStore || cfg.MigrateGreetingUpdate || cfg.MigrateGreetingDestroy {
		mysqlStore := getMySQLStore()
		resolver, loginCache := buildUserResolver("greeting")
		greeting := dashboard.NewGreetingHandler(mysqlStore, loginCache, resolver, dashboard.NewRBACResolver(mysqlStore), cfg.APIBaseURL)
		if cfg.MigrateGreetingIndex {
			options = append(options, compatserver.WithGreetingIndexHandler(http.HandlerFunc(greeting.Index)))
			routeDebugf("go migrated route enabled: GET /dashboard/greeting/index")
		}
		if cfg.MigrateGreetingShow {
			options = append(options, compatserver.WithGreetingShowHandler(http.HandlerFunc(greeting.Show)))
			routeDebugf("go migrated route enabled: GET /dashboard/greeting/show")
		}
		if cfg.MigrateGreetingStore {
			options = append(options, compatserver.WithGreetingStoreHandler(http.HandlerFunc(greeting.Store)))
			routeDebugf("go migrated route enabled: POST /dashboard/greeting/store")
		}
		if cfg.MigrateGreetingUpdate {
			options = append(options, compatserver.WithGreetingUpdateHandler(http.HandlerFunc(greeting.Update)))
			routeDebugf("go migrated route enabled: PUT /dashboard/greeting/update")
		}
		if cfg.MigrateGreetingDestroy {
			options = append(options, compatserver.WithGreetingDestroyHandler(http.HandlerFunc(greeting.Destroy)))
			routeDebugf("go migrated route enabled: DELETE /dashboard/greeting/destroy")
		}
	}

	if cfg.MigrateRoomWelcomeIndex || cfg.MigrateRoomWelcomeSelect || cfg.MigrateRoomWelcomeShow || cfg.MigrateRoomWelcomeStore || cfg.MigrateRoomWelcomeUpdate || cfg.MigrateRoomWelcomeDestroy {
		mysqlStore := getMySQLStore()
		resolver, loginCache := buildUserResolver("roomWelcome")
		roomWelcome := dashboard.NewRoomWelcomeHandler(
			mysqlStore,
			loginCache,
			resolver,
			dashboard.NewRBACResolver(mysqlStore),
			cfg.APIBaseURL,
			cfg.FileStorageRoot,
			dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL),
		)
		if cfg.MigrateRoomWelcomeIndex {
			options = append(options, compatserver.WithRoomWelcomeIndexHandler(http.HandlerFunc(roomWelcome.Index)))
			routeDebugf("go migrated route enabled: GET /dashboard/roomWelcome/index")
			routeDebugf("go migrated route enabled: GET /dashboard/clockIn/index")
		}
		if cfg.MigrateRoomWelcomeSelect {
			options = append(options, compatserver.WithRoomWelcomeSelectHandler(http.HandlerFunc(roomWelcome.Select)))
			routeDebugf("go migrated route enabled: GET /dashboard/roomWelcome/select")
		}
		if cfg.MigrateRoomWelcomeShow {
			options = append(options, compatserver.WithRoomWelcomeShowHandler(http.HandlerFunc(roomWelcome.Show)))
			routeDebugf("go migrated route enabled: GET /dashboard/roomWelcome/show")
		}
		if cfg.MigrateRoomWelcomeStore {
			options = append(options, compatserver.WithRoomWelcomeStoreHandler(http.HandlerFunc(roomWelcome.Store)))
			routeDebugf("go migrated route enabled: POST /dashboard/roomWelcome/store")
		}
		if cfg.MigrateRoomWelcomeUpdate {
			options = append(options, compatserver.WithRoomWelcomeUpdateHandler(http.HandlerFunc(roomWelcome.Update)))
			routeDebugf("go migrated route enabled: PUT /dashboard/roomWelcome/update")
		}
		if cfg.MigrateRoomWelcomeDestroy {
			options = append(options, compatserver.WithRoomWelcomeDestroyHandler(http.HandlerFunc(roomWelcome.Destroy)))
			routeDebugf("go migrated route enabled: DELETE /dashboard/roomWelcome/destroy")
		}
	}

	if cfg.MigrateContactFieldIndex || cfg.MigrateContactFieldShow || cfg.MigrateContactFieldPortrait || cfg.MigrateContactFieldStore || cfg.MigrateContactFieldUpdate || cfg.MigrateContactFieldStatus || cfg.MigrateContactFieldDestroy || cfg.MigrateContactFieldBatch || cfg.MigrateContactFieldPivot || cfg.MigrateContactFieldPivotUpdate || cfg.MigrateSidebarFieldPivot || cfg.MigrateSidebarFieldPivotUpdate {
		mysqlStore := getMySQLStore()
		resolver, loginCache := buildUserResolver("contactField")
		contactField := dashboard.NewContactFieldHandler(mysqlStore, loginCache, resolver, dashboard.NewRBACResolver(mysqlStore), cfg.APIBaseURL)
		if cfg.MigrateSidebarFieldPivot || cfg.MigrateSidebarFieldPivotUpdate {
			contactField.WithSidebarEmployeeResolver(buildSidebarEmployeeResolver("sidebarContactFieldPivot"))
		}
		if cfg.MigrateContactFieldIndex {
			options = append(options, compatserver.WithContactFieldIndexHandler(http.HandlerFunc(contactField.Index)))
			routeDebugf("go migrated route enabled: GET /dashboard/contactField/index")
		}
		if cfg.MigrateContactFieldShow {
			options = append(options, compatserver.WithContactFieldShowHandler(http.HandlerFunc(contactField.Show)))
			routeDebugf("go migrated route enabled: GET /dashboard/contactField/show")
		}
		if cfg.MigrateContactFieldPortrait {
			options = append(options, compatserver.WithContactFieldPortraitHandler(http.HandlerFunc(contactField.Portrait)))
			routeDebugf("go migrated route enabled: GET /dashboard/contactField/portrait")
		}
		if cfg.MigrateContactFieldStore {
			options = append(options, compatserver.WithContactFieldStoreHandler(http.HandlerFunc(contactField.Store)))
			routeDebugf("go migrated route enabled: POST /dashboard/contactField/store")
		}
		if cfg.MigrateContactFieldUpdate {
			options = append(options, compatserver.WithContactFieldUpdateHandler(http.HandlerFunc(contactField.Update)))
			routeDebugf("go migrated route enabled: PUT /dashboard/contactField/update")
		}
		if cfg.MigrateContactFieldStatus {
			options = append(options, compatserver.WithContactFieldStatusUpdateHandler(http.HandlerFunc(contactField.StatusUpdate)))
			routeDebugf("go migrated route enabled: PUT /dashboard/contactField/statusUpdate")
		}
		if cfg.MigrateContactFieldDestroy {
			options = append(options, compatserver.WithContactFieldDestroyHandler(http.HandlerFunc(contactField.Destroy)))
			routeDebugf("go migrated route enabled: DELETE /dashboard/contactField/destroy")
		}
		if cfg.MigrateContactFieldBatch {
			options = append(options, compatserver.WithContactFieldBatchUpdateHandler(http.HandlerFunc(contactField.BatchUpdate)))
			routeDebugf("go migrated route enabled: PUT /dashboard/contactField/batchUpdate")
		}
		if cfg.MigrateContactFieldPivot {
			options = append(options, compatserver.WithContactFieldPivotIndexHandler(http.HandlerFunc(contactField.FieldPivotIndex)))
			routeDebugf("go migrated route enabled: GET /dashboard/contactFieldPivot/index")
		}
		if cfg.MigrateContactFieldPivotUpdate {
			options = append(options, compatserver.WithContactFieldPivotUpdateHandler(http.HandlerFunc(contactField.FieldPivotUpdate)))
			routeDebugf("go migrated route enabled: PUT /dashboard/contactFieldPivot/update")
		}
		if cfg.MigrateSidebarFieldPivot {
			options = append(options, compatserver.WithSidebarContactFieldPivotIndexHandler(http.HandlerFunc(contactField.SidebarFieldPivotIndex)))
			routeDebugf("go migrated route enabled: GET /sidebar/contactFieldPivot/index")
		}
		if cfg.MigrateSidebarFieldPivotUpdate {
			options = append(options, compatserver.WithSidebarContactFieldPivotUpdateHandler(http.HandlerFunc(contactField.SidebarFieldPivotUpdate)))
			routeDebugf("go migrated route enabled: PUT /sidebar/contactFieldPivot/update")
		}
	}

	if cfg.MigrateContactBatchAddDashboard || cfg.MigrateSidebarContactBatchAddDetail {
		mysqlStore := getMySQLStore()
		if cfg.MigrateContactBatchAddDashboard {
			resolver, loginCache := buildUserResolver("contactBatchAdd")
			contactBatchAddDashboard := dashboard.NewContactBatchAddDashboardHandler(mysqlStore, loginCache, resolver, dashboard.NewRBACResolver(mysqlStore)).
				WithFileStorage(cfg.FileStorageRoot, cfg.APIBaseURL)
			options = append(options,
				compatserver.WithContactBatchAddIndexHandler(http.HandlerFunc(contactBatchAddDashboard.Index)),
				compatserver.WithContactBatchAddImportIndexHandler(http.HandlerFunc(contactBatchAddDashboard.ImportIndex)),
				compatserver.WithContactBatchAddImportStoreHandler(http.HandlerFunc(contactBatchAddDashboard.ImportStore)),
				compatserver.WithContactBatchAddAllotHandler(http.HandlerFunc(contactBatchAddDashboard.Allot)),
				compatserver.WithContactBatchAddDataStatisticHandler(http.HandlerFunc(contactBatchAddDashboard.DataStatistic)),
				compatserver.WithContactBatchAddDestroyHandler(http.HandlerFunc(contactBatchAddDashboard.Destroy)),
				compatserver.WithContactBatchAddImportDestroyHandler(http.HandlerFunc(contactBatchAddDashboard.ImportDestroy)),
				compatserver.WithContactBatchAddSettingEditHandler(http.HandlerFunc(contactBatchAddDashboard.SettingEdit)),
				compatserver.WithContactBatchAddSettingUpdateHandler(http.HandlerFunc(contactBatchAddDashboard.SettingUpdate)),
				compatserver.WithContactBatchAddRemindHandler(http.HandlerFunc(contactBatchAddDashboard.Remind)),
			)
			routeDebugf("go migrated route enabled: GET /dashboard/contactBatchAdd/index")
			routeDebugf("go migrated route enabled: GET /dashboard/contactBatchAdd/importIndex")
			routeDebugf("go migrated route enabled: POST /dashboard/contactBatchAdd/importStore")
			routeDebugf("go migrated route enabled: POST /dashboard/contactBatchAdd/allot")
			routeDebugf("go migrated route enabled: GET /dashboard/contactBatchAdd/dataStatistic")
			routeDebugf("go migrated route enabled: DELETE /dashboard/contactBatchAdd/destroy")
			routeDebugf("go migrated route enabled: DELETE /dashboard/contactBatchAdd/importDestroy")
			routeDebugf("go migrated route enabled: GET /dashboard/contactBatchAdd/settingEdit")
			routeDebugf("go migrated route enabled: POST /dashboard/contactBatchAdd/settingUpdate")
			routeDebugf("go migrated route enabled: GET/POST /dashboard/contactBatchAdd/remind")
		}
		if cfg.MigrateSidebarContactBatchAddDetail {
			contactBatchAdd := dashboard.NewContactBatchAddHandler(mysqlStore, buildSidebarEmployeeResolver("sidebarContactBatchAdd"))
			options = append(options, compatserver.WithSidebarContactBatchAddDetailHandler(http.HandlerFunc(contactBatchAdd.Detail)))
			routeDebugf("go migrated route enabled: GET /sidebar/contactBatchAdd/detail")
		}
	}

	if cfg.MigrateSensitiveWordsDashboard {
		mysqlStore := getMySQLStore()
		resolver, loginCache := buildUserResolver("sensitiveWords")
		sensitiveWords := dashboard.NewSensitiveWordHandler(mysqlStore, loginCache, resolver, dashboard.NewRBACResolver(mysqlStore)).WithMonitorEnabled(cfg.EnableSensitiveWordMonitorCron)
		options = append(options,
			compatserver.WithSensitiveWordsPageHandler(dashboard.NewSensitiveWordPageHandler()),
			compatserver.WithSensitiveWordIndexHandler(http.HandlerFunc(sensitiveWords.Index)),
			compatserver.WithSensitiveWordStoreHandler(http.HandlerFunc(sensitiveWords.Store)),
			compatserver.WithSensitiveWordDestroyHandler(http.HandlerFunc(sensitiveWords.Destroy)),
			compatserver.WithSensitiveWordStatusUpdateHandler(http.HandlerFunc(sensitiveWords.StatusUpdate)),
			compatserver.WithSensitiveWordMoveHandler(http.HandlerFunc(sensitiveWords.Move)),
			compatserver.WithSensitiveWordGroupSelectHandler(http.HandlerFunc(sensitiveWords.GroupSelect)),
			compatserver.WithSensitiveWordGroupStoreHandler(http.HandlerFunc(sensitiveWords.GroupStore)),
			compatserver.WithSensitiveWordGroupUpdateHandler(http.HandlerFunc(sensitiveWords.GroupUpdate)),
			compatserver.WithSensitiveWordsMonitorIndexHandler(http.HandlerFunc(sensitiveWords.MonitorIndex)),
			compatserver.WithSensitiveWordsMonitorStatusHandler(http.HandlerFunc(sensitiveWords.MonitorStatus)),
			compatserver.WithSensitiveWordsMonitorShowHandler(http.HandlerFunc(sensitiveWords.MonitorShow)),
		)
		routeDebugf("go migrated route enabled: GET /dashboard/sensitiveWords/page")
		routeDebugf("go migrated route enabled: GET /dashboard/sensitiveWord/index")
		routeDebugf("go migrated route enabled: POST /dashboard/sensitiveWord/store")
		routeDebugf("go migrated route enabled: DELETE /dashboard/sensitiveWord/destroy")
		routeDebugf("go migrated route enabled: PUT /dashboard/sensitiveWord/statusUpdate")
		routeDebugf("go migrated route enabled: PUT /dashboard/sensitiveWord/move")
		routeDebugf("go migrated route enabled: GET /dashboard/sensitiveWordGroup/select")
		routeDebugf("go migrated route enabled: POST /dashboard/sensitiveWordGroup/store")
		routeDebugf("go migrated route enabled: PUT /dashboard/sensitiveWordGroup/update")
		routeDebugf("go migrated route enabled: GET /dashboard/sensitiveWordsMonitor/index")
		routeDebugf("go migrated route enabled: GET /dashboard/sensitiveWordsMonitor/status")
		routeDebugf("go migrated route enabled: GET /dashboard/sensitiveWordsMonitor/show")
	}

	if cfg.MigrateContactSOPDashboard || cfg.MigrateRoomSOPDashboard {
		mysqlStore := getMySQLStore()
		resolver, loginCache := buildUserResolver("sopDashboard")
		sopDashboard := dashboard.NewSOPDashboardHandler(mysqlStore, loginCache, resolver, dashboard.NewRBACResolver(mysqlStore))
		if cfg.MigrateContactSOPDashboard {
			options = append(options,
				compatserver.WithContactSOPPageHandler(dashboard.NewContactSOPPageHandler()),
				compatserver.WithContactSOPIndexHandler(http.HandlerFunc(sopDashboard.ContactIndex)),
				compatserver.WithContactSOPStoreHandler(http.HandlerFunc(sopDashboard.ContactStore)),
				compatserver.WithContactSOPSetEmployeeHandler(http.HandlerFunc(sopDashboard.ContactSetEmployee)),
				compatserver.WithContactSOPStateHandler(http.HandlerFunc(sopDashboard.ContactState)),
				compatserver.WithContactSOPInfoHandler(http.HandlerFunc(sopDashboard.ContactInfo)),
				compatserver.WithContactSOPDestroyHandler(http.HandlerFunc(sopDashboard.ContactDestroy)),
				compatserver.WithContactSOPUpdateHandler(http.HandlerFunc(sopDashboard.ContactUpdate)),
			)
			routeDebugf("go migrated route enabled: GET /dashboard/contactSop/page")
			routeDebugf("go migrated route enabled: GET /dashboard/contactSop/index")
			routeDebugf("go migrated route enabled: POST /dashboard/contactSop/store")
			routeDebugf("go migrated route enabled: PUT /dashboard/contactSop/setEmployee")
			routeDebugf("go migrated route enabled: PUT /dashboard/contactSop/state")
			routeDebugf("go migrated route enabled: GET /dashboard/contactSop/info")
			routeDebugf("go migrated route enabled: DELETE /dashboard/contactSop/destroy")
			routeDebugf("go migrated route enabled: PUT /dashboard/contactSop/update")
		}
		if cfg.MigrateRoomSOPDashboard {
			options = append(options,
				compatserver.WithRoomSOPPageHandler(dashboard.NewRoomSOPPageHandler()),
				compatserver.WithRoomSOPIndexHandler(http.HandlerFunc(sopDashboard.RoomIndex)),
				compatserver.WithRoomSOPStoreHandler(http.HandlerFunc(sopDashboard.RoomStore)),
				compatserver.WithRoomSOPSetRoomHandler(http.HandlerFunc(sopDashboard.RoomSetRoom)),
				compatserver.WithRoomSOPStateHandler(http.HandlerFunc(sopDashboard.RoomState)),
				compatserver.WithRoomSOPInfoHandler(http.HandlerFunc(sopDashboard.RoomInfo)),
				compatserver.WithRoomSOPDestroyHandler(http.HandlerFunc(sopDashboard.RoomDestroy)),
				compatserver.WithRoomSOPUpdateHandler(http.HandlerFunc(sopDashboard.RoomUpdate)),
			)
			routeDebugf("go migrated route enabled: GET /dashboard/roomSop/page")
			routeDebugf("go migrated route enabled: GET /dashboard/roomSop/index")
			routeDebugf("go migrated route enabled: POST /dashboard/roomSop/store")
			routeDebugf("go migrated route enabled: PUT /dashboard/roomSop/setRoom")
			routeDebugf("go migrated route enabled: PUT /dashboard/roomSop/state")
			routeDebugf("go migrated route enabled: GET /dashboard/roomSop/info")
			routeDebugf("go migrated route enabled: DELETE /dashboard/roomSop/destroy")
			routeDebugf("go migrated route enabled: PUT /dashboard/roomSop/update")
		}
	}

	if cfg.MigrateShopCodeDashboard {
		mysqlStore := getMySQLStore()
		resolver, loginCache := buildUserResolver("shopCodeDashboard")
		shopCode := dashboard.NewShopCodeHandler(mysqlStore, loginCache, resolver, dashboard.NewRBACResolver(mysqlStore), cfg.OperationBaseURL)
		options = append(options,
			compatserver.WithShopCodePageHandler(dashboard.NewShopCodePageHandler()),
			compatserver.WithShopCodeLocationHandler(http.HandlerFunc(shopCode.Location)),
			compatserver.WithShopCodeAddressKeyWordListHandler(http.HandlerFunc(shopCode.AddressKeyWordList)),
			compatserver.WithShopCodeStoreHandler(http.HandlerFunc(shopCode.Store)),
			compatserver.WithShopCodeUpdateHandler(http.HandlerFunc(shopCode.Update)),
			compatserver.WithShopCodeDestroyHandler(http.HandlerFunc(shopCode.Destroy)),
			compatserver.WithShopCodeInfoHandler(http.HandlerFunc(shopCode.Info)),
			compatserver.WithShopCodeStatusHandler(http.HandlerFunc(shopCode.Status)),
			compatserver.WithShopCodeIndexHandler(http.HandlerFunc(shopCode.Index)),
			compatserver.WithShopCodeSearchCityHandler(http.HandlerFunc(shopCode.SearchCity)),
			compatserver.WithShopCodeShareHandler(http.HandlerFunc(shopCode.Share)),
			compatserver.WithShopCodePageInfoHandler(http.HandlerFunc(shopCode.PageInfo)),
			compatserver.WithShopCodePageSetHandler(http.HandlerFunc(shopCode.PageSet)),
			compatserver.WithShopCodeShowHandler(http.HandlerFunc(shopCode.Show)),
			compatserver.WithShopCodeShowContactHandler(http.HandlerFunc(shopCode.ShowContact)),
			compatserver.WithShopCodeShowShopHandler(http.HandlerFunc(shopCode.ShowShop)),
			compatserver.WithShopCodeUpdateEmployeeHandler(http.HandlerFunc(shopCode.UpdateEmployee)),
			compatserver.WithShopCodeUpdateQRCodeHandler(http.HandlerFunc(shopCode.UpdateQRCode)),
			compatserver.WithShopCodeBatchContactTagsHandler(http.HandlerFunc(shopCode.BatchContactTags)),
		)
		routeDebugf("go migrated route enabled: GET /dashboard/shopCode/page")
		routeDebugf("go migrated route enabled: GET /dashboard/shopCode/location")
		routeDebugf("go migrated route enabled: GET /dashboard/shopCode/addressKeyWordList")
		routeDebugf("go migrated route enabled: POST /dashboard/shopCode/store")
		routeDebugf("go migrated route enabled: PUT /dashboard/shopCode/update")
		routeDebugf("go migrated route enabled: DELETE /dashboard/shopCode/destroy")
		routeDebugf("go migrated route enabled: GET /dashboard/shopCode/info")
		routeDebugf("go migrated route enabled: PUT /dashboard/shopCode/status")
		routeDebugf("go migrated route enabled: GET /dashboard/shopCode/index")
		routeDebugf("go migrated route enabled: GET /dashboard/shopCode/searchCity")
		routeDebugf("go migrated route enabled: GET /dashboard/shopCode/share")
		routeDebugf("go migrated route enabled: GET /dashboard/shopCode/pageInfo")
		routeDebugf("go migrated route enabled: POST /dashboard/shopCode/pageSet")
		routeDebugf("go migrated route enabled: GET /dashboard/shopCode/show")
		routeDebugf("go migrated route enabled: GET /dashboard/shopCode/showContact")
		routeDebugf("go migrated route enabled: GET /dashboard/shopCode/showShop")
		routeDebugf("go migrated route enabled: POST /dashboard/shopCode/updateEmployee")
		routeDebugf("go migrated route enabled: POST /dashboard/shopCode/updateQrcode")
		routeDebugf("go migrated route enabled: PUT /dashboard/shopCode/batchContactTags")
	}

	if cfg.MigrateRadarDashboard {
		mysqlStore := getMySQLStore()
		resolver, loginCache := buildUserResolver("radarDashboard")
		radar := dashboard.NewRadarHandler(mysqlStore, loginCache, resolver, dashboard.NewRBACResolver(mysqlStore), cfg.OperationBaseURL)
		options = append(options,
			compatserver.WithRadarPageHandler(dashboard.NewRadarPageHandler()),
			compatserver.WithRadarStoreHandler(http.HandlerFunc(radar.Store)),
			compatserver.WithRadarUpdateHandler(http.HandlerFunc(radar.Update)),
			compatserver.WithRadarIndexHandler(http.HandlerFunc(radar.Index)),
			compatserver.WithRadarDestroyHandler(http.HandlerFunc(radar.Destroy)),
			compatserver.WithRadarInfoHandler(http.HandlerFunc(radar.Info)),
			compatserver.WithRadarStoreChannelHandler(http.HandlerFunc(radar.StoreChannel)),
			compatserver.WithRadarStoreChannelLinkHandler(http.HandlerFunc(radar.StoreChannelLink)),
			compatserver.WithRadarIndexChannelHandler(http.HandlerFunc(radar.IndexChannel)),
			compatserver.WithRadarIndexChannelLinkHandler(http.HandlerFunc(radar.IndexChannelLink)),
			compatserver.WithRadarShowHandler(http.HandlerFunc(radar.Show)),
			compatserver.WithRadarShowContactHandler(http.HandlerFunc(radar.ShowContact)),
			compatserver.WithRadarShowChannelHandler(http.HandlerFunc(radar.ShowChannel)),
			compatserver.WithRadarArticleHandler(http.HandlerFunc(radar.RadarArticle)),
		)
		routeDebugf("go migrated route enabled: GET /dashboard/radar/page")
		routeDebugf("go migrated route enabled: POST /dashboard/radar/store")
		routeDebugf("go migrated route enabled: PUT /dashboard/radar/update")
		routeDebugf("go migrated route enabled: GET /dashboard/radar/index")
		routeDebugf("go migrated route enabled: DELETE /dashboard/radar/destroy")
		routeDebugf("go migrated route enabled: GET /dashboard/radar/info")
		routeDebugf("go migrated route enabled: POST /dashboard/radar/storeChannel")
		routeDebugf("go migrated route enabled: POST /dashboard/radar/storeChannelLink")
		routeDebugf("go migrated route enabled: GET /dashboard/radar/indexChannel")
		routeDebugf("go migrated route enabled: GET /dashboard/radar/indexChannelLink")
		routeDebugf("go migrated route enabled: GET /dashboard/radar/show")
		routeDebugf("go migrated route enabled: GET /dashboard/radar/showContact")
		routeDebugf("go migrated route enabled: GET /dashboard/radar/showChannel")
		routeDebugf("go migrated route enabled: GET /dashboard/radar/radarArticle")
	}

	if cfg.MigrateAutoTagDashboard {
		mysqlStore := getMySQLStore()
		resolver, loginCache := buildUserResolver("autoTagDashboard")
		autoTag := dashboard.NewAutoTagHandler(mysqlStore, loginCache, resolver, dashboard.NewRBACResolver(mysqlStore))
		var archiveComponentHandler http.Handler
		if archivePlan.durableAPI {
			componentBridgeClient, componentBridgeErr := archiveprovider.NewBridgeArchiveClient(cfg.WorkMessageArchiveBridgeBaseURL, cfg.WorkMessageArchiveBridgeToken, nil)
			if componentBridgeErr != nil {
				fatalf("build Dashboard archive component bridge: %v", componentBridgeErr)
			}
			archiveComponentHandler = dashboard.NewArchiveComponentHandler(mysqlStore, dashboardArchiveComponentBridge{client: componentBridgeClient})
		}
		riskBehavior := dashboard.NewRiskBehaviorHandler(mysqlStore, loginCache, resolver, dashboard.NewRBACResolver(mysqlStore)).WithScannerEnabled(false)
		timeoutWarning := dashboard.NewTimeoutWarningHandler(mysqlStore, loginCache, resolver, dashboard.NewRBACResolver(mysqlStore))
		messageIntercept := dashboard.NewMessageInterceptHandler(mysqlStore, loginCache, resolver, dashboard.NewRBACResolver(mysqlStore))
		phase33Closure := dashboard.NewPhase33ClosureHandler(mysqlStore, loginCache, resolver, dashboard.NewRBACResolver(mysqlStore))
		if cfg.EnableMarkTagsWorker {
			autoTag.WithMarkTagsQueue(getRedisStore())
		}
		options = append(options,
			compatserver.WithAutoTagStoreHandler(http.HandlerFunc(autoTag.Store)),
			compatserver.WithAutoTagIndexHandler(http.HandlerFunc(autoTag.Index)),
			compatserver.WithAutoTagDestroyHandler(http.HandlerFunc(autoTag.Destroy)),
			compatserver.WithAutoTagOnOffHandler(http.HandlerFunc(autoTag.OnOff)),
			compatserver.WithAutoTagShowHandler(http.HandlerFunc(autoTag.Show)),
			compatserver.WithAutoTagShowContactKeyWordHandler(http.HandlerFunc(autoTag.ShowContactKeyWord)),
			compatserver.WithAutoTagKeyWordTagHandler(http.HandlerFunc(autoTag.KeyWordTag)),
			compatserver.WithAutoTagShowContactRoomHandler(http.HandlerFunc(autoTag.ShowContactRoom)),
			compatserver.WithAutoTagShowContactTimeHandler(http.HandlerFunc(autoTag.ShowContactTime)),
			compatserver.WithWorkMessageFromUsersHandler(http.HandlerFunc(autoTag.WorkMessageFromUsers)),
			compatserver.WithWorkMessageToUsersHandler(http.HandlerFunc(autoTag.WorkMessageToUsers)),
			compatserver.WithWorkMessageGlobalOverviewHandler(http.HandlerFunc(autoTag.WorkMessageGlobalOverview)),
			compatserver.WithWorkMessageFocusHandler(http.HandlerFunc(autoTag.WorkMessageFocus)),
			compatserver.WithWorkMessageStaffDirectoryHandler(http.HandlerFunc(autoTag.WorkMessageStaffDirectory)),
			compatserver.WithWorkMessageStaffDetailHandler(http.HandlerFunc(autoTag.WorkMessageStaffDetail)),
			compatserver.WithWorkMessageTrajectoryDayHandler(http.HandlerFunc(autoTag.WorkMessageTrajectoryDay)),
			compatserver.WithWorkMessageCustomerDirectoryHandler(http.HandlerFunc(autoTag.WorkMessageCustomerDirectory)),
			compatserver.WithWorkMessageCustomerConversationsHandler(http.HandlerFunc(autoTag.WorkMessageCustomerConversations)),
			compatserver.WithWorkMessageCustomerDetailHandler(http.HandlerFunc(autoTag.WorkMessageCustomerDetail)),
			compatserver.WithWorkMessageRoomDirectoryHandler(http.HandlerFunc(autoTag.WorkMessageRoomDirectory)),
			compatserver.WithWorkMessageRoomProfileHandler(http.HandlerFunc(autoTag.WorkMessageRoomProfile)),
			compatserver.WithWorkMessageRoomMessagesHandler(http.HandlerFunc(autoTag.WorkMessageRoomMessages)),
			compatserver.WithWorkMessageRoomMembersHandler(http.HandlerFunc(autoTag.WorkMessageRoomMembers)),
			compatserver.WithWorkMessageRoomFilterOptionsHandler(http.HandlerFunc(autoTag.WorkMessageRoomFilterOptions)),
			compatserver.WithWorkMessageExportCandidatesHandler(http.HandlerFunc(autoTag.WorkMessageExportCandidates)),
			compatserver.WithWorkMessageExportTasksHandler(http.HandlerFunc(autoTag.WorkMessageExportTasks)),
			compatserver.WithWorkMessageExportDownloadHandler(http.HandlerFunc(autoTag.WorkMessageExportDownload)),
			compatserver.WithArchiveMediaContentHandler(dashboard.NewArchiveMediaContentHandler(mysqlStore, cfg.FileStorageRoot)),
			compatserver.WithRiskBehaviorRulesHandler(http.HandlerFunc(riskBehavior.Rules)),
			compatserver.WithRiskBehaviorRecordsHandler(http.HandlerFunc(riskBehavior.Records)),
			compatserver.WithRiskBehaviorRecordDetailHandler(http.HandlerFunc(riskBehavior.RecordDetail)),
			compatserver.WithRiskBehaviorScannerStatusHandler(http.HandlerFunc(riskBehavior.ScannerStatus)),
			compatserver.WithRiskBehaviorRuleCreateHandler(http.HandlerFunc(riskBehavior.CreateRule)),
			compatserver.WithRiskBehaviorRuleUpdateHandler(http.HandlerFunc(riskBehavior.UpdateRule)),
			compatserver.WithRiskBehaviorRuleStatusHandler(http.HandlerFunc(riskBehavior.RuleStatus)),
			compatserver.WithRiskBehaviorRuleDeleteHandler(http.HandlerFunc(riskBehavior.DeleteRule)),
			compatserver.WithRiskBehaviorRecordsAuditHandler(http.HandlerFunc(riskBehavior.AuditRecords)),
			compatserver.WithRiskBehaviorEvaluateHandler(http.HandlerFunc(riskBehavior.Evaluate)),
			compatserver.WithTimeoutWarningRulesHandler(http.HandlerFunc(timeoutWarning.Rules)),
			compatserver.WithTimeoutWarningRecordsHandler(http.HandlerFunc(timeoutWarning.Records)),
			compatserver.WithTimeoutWarningRuleCreateHandler(http.HandlerFunc(timeoutWarning.CreateRule)),
			compatserver.WithTimeoutWarningRuleUpdateHandler(http.HandlerFunc(timeoutWarning.UpdateRule)),
			compatserver.WithTimeoutWarningRuleStatusHandler(http.HandlerFunc(timeoutWarning.RuleStatus)),
			compatserver.WithTimeoutWarningRuleDeleteHandler(http.HandlerFunc(timeoutWarning.DeleteRule)),
			compatserver.WithTimeoutWarningRecordsAuditHandler(http.HandlerFunc(timeoutWarning.AuditRecords)),
			compatserver.WithTimeoutWarningRecordsAssignHandler(http.HandlerFunc(timeoutWarning.AssignRecords)),
			compatserver.WithTimeoutWarningSettingsHandler(http.HandlerFunc(timeoutWarning.Settings)),
			compatserver.WithTimeoutWarningSettingsUpdateHandler(http.HandlerFunc(timeoutWarning.SaveSettings)),
			compatserver.WithTimeoutWarningEvaluateHandler(http.HandlerFunc(timeoutWarning.Evaluate)),
			compatserver.WithKeywordLibrariesHandler(http.HandlerFunc(messageIntercept.Libraries)),
			compatserver.WithKeywordLibrarySaveHandler(http.HandlerFunc(messageIntercept.SaveLibrary)),
			compatserver.WithKeywordLibraryStatusHandler(http.HandlerFunc(messageIntercept.LibraryStatus)),
			compatserver.WithKeywordLibraryDeleteHandler(http.HandlerFunc(messageIntercept.DeleteLibrary)),
			compatserver.WithKeywordLibraryPublishHandler(http.HandlerFunc(messageIntercept.PublishLibrary)),
			compatserver.WithKeywordEntriesHandler(http.HandlerFunc(messageIntercept.Entries)),
			compatserver.WithKeywordEntrySaveHandler(http.HandlerFunc(messageIntercept.SaveEntry)),
			compatserver.WithKeywordEntryStatusHandler(http.HandlerFunc(messageIntercept.EntryStatus)),
			compatserver.WithKeywordEntryDeleteHandler(http.HandlerFunc(messageIntercept.DeleteEntry)),
			compatserver.WithMessageInterceptRulesHandler(http.HandlerFunc(messageIntercept.Rules)),
			compatserver.WithMessageInterceptRuleSaveHandler(http.HandlerFunc(messageIntercept.SaveRule)),
			compatserver.WithMessageInterceptRuleStatusHandler(http.HandlerFunc(messageIntercept.RuleStatus)),
			compatserver.WithMessageInterceptRuleDeleteHandler(http.HandlerFunc(messageIntercept.DeleteRule)),
			compatserver.WithMessageInterceptRecordsHandler(http.HandlerFunc(messageIntercept.Records)),
			compatserver.WithMessageInterceptEvaluateHandler(http.HandlerFunc(messageIntercept.Evaluate)),
			compatserver.WithMessageInterceptAuditHandler(http.HandlerFunc(messageIntercept.Audit)),
			compatserver.WithSilentCustomerRulesHandler(http.HandlerFunc(phase33Closure.SilentRules)),
			compatserver.WithSilentCustomerRuleSaveHandler(http.HandlerFunc(phase33Closure.SaveSilentRule)),
			compatserver.WithSilentCustomerRuleStatusHandler(http.HandlerFunc(phase33Closure.SilentRuleStatus)),
			compatserver.WithSilentCustomerRuleDeleteHandler(http.HandlerFunc(phase33Closure.DeleteSilentRule)),
			compatserver.WithSilentCustomerRecordsHandler(http.HandlerFunc(phase33Closure.SilentRecords)),
			compatserver.WithSilentCustomerEvaluateHandler(http.HandlerFunc(phase33Closure.EvaluateSilent)),
			compatserver.WithSilentCustomerActionHandler(http.HandlerFunc(phase33Closure.ActSilent)),
			compatserver.WithRefuseArchiveRecordsHandler(http.HandlerFunc(phase33Closure.RefuseRecords)),
			compatserver.WithRefuseArchiveSyncHandler(http.HandlerFunc(phase33Closure.SyncRefuse)),
			compatserver.WithRefuseArchiveFollowUpHandler(http.HandlerFunc(phase33Closure.FollowRefuse)),
			compatserver.WithWorkMessageIndexHandler(http.HandlerFunc(autoTag.WorkMessageIndex)),
			compatserver.WithWorkMessageConfigCorpStoreHandler(http.HandlerFunc(autoTag.WorkMessageConfigCorpStore)),
			compatserver.WithWorkMessageConfigCorpShowHandler(http.HandlerFunc(autoTag.WorkMessageConfigCorpShow)),
			compatserver.WithWorkMessageConfigCorpIndexHandler(http.HandlerFunc(autoTag.WorkMessageConfigCorpIndex)),
			compatserver.WithWorkMessageConfigStepCreateHandler(http.HandlerFunc(autoTag.WorkMessageConfigStepCreate)),
			compatserver.WithWorkMessageConfigStepUpdateHandler(http.HandlerFunc(autoTag.WorkMessageConfigStepUpdate)),
		)
		if archiveComponentHandler != nil {
			options = append(options, compatserver.WithArchiveComponentHandler(archiveComponentHandler))
		}
		routeDebugf("go migrated route enabled: POST /dashboard/autoTag/store")
		routeDebugf("go migrated route enabled: GET /dashboard/autoTag/index")
		routeDebugf("go migrated route enabled: DELETE /dashboard/autoTag/destroy")
		routeDebugf("go migrated route enabled: PUT /dashboard/autoTag/onOff")
		routeDebugf("go migrated route enabled: GET /dashboard/autoTag/show")
		routeDebugf("go migrated route enabled: GET /dashboard/autoTag/showContactKeyWord")
		routeDebugf("go migrated route enabled: GET /Task/AutoTag/KeyWordTag")
		routeDebugf("go migrated route enabled: GET /dashboard/Task/AutoTag/KeyWordTag")
		routeDebugf("go migrated route enabled: GET /dashboard/autoTag/showContactRoom")
		routeDebugf("go migrated route enabled: GET /dashboard/autoTag/showContactTime")
		routeDebugf("go migrated route enabled: GET /dashboard/workMessage/fromUsers")
		routeDebugf("go migrated route enabled: GET /dashboard/workMessage/toUsers")
		routeDebugf("go migrated route enabled: GET /dashboard/workMessage/customerDirectory")
		routeDebugf("go migrated route enabled: GET /dashboard/workMessage/customerConversations")
		routeDebugf("go migrated route enabled: GET /dashboard/workMessage/customerDetail")
		routeDebugf("go migrated route enabled: GET /dashboard/workMessage/trajectoryDay")
		routeDebugf("go migrated route enabled: GET /dashboard/workMessage/roomDirectory")
		routeDebugf("go migrated route enabled: GET /dashboard/workMessage/roomProfile")
		routeDebugf("go migrated route enabled: GET /dashboard/workMessage/roomMessages")
		routeDebugf("go migrated route enabled: GET /dashboard/workMessage/roomMembers")
		routeDebugf("go migrated route enabled: GET /dashboard/workMessage/roomFilterOptions")
		routeDebugf("go migrated route enabled: GET /dashboard/workMessage/index")
		routeDebugf("go migrated route enabled: POST /dashboard/workMessageConfig/corpStore")
		routeDebugf("go migrated route enabled: GET /dashboard/workMessageConfig/corpShow")
		routeDebugf("go migrated route enabled: GET /dashboard/workMessageConfig/corpIndex")
		routeDebugf("go migrated route enabled: GET /dashboard/workMessageConfig/stepCreate")
		routeDebugf("go migrated route enabled: PUT /dashboard/workMessageConfig/stepUpdate")
	}

	if cfg.MigrateLotteryDashboard {
		mysqlStore := getMySQLStore()
		resolver, loginCache := buildUserResolver("lotteryDashboard")
		lottery := dashboard.NewLotteryHandler(mysqlStore, loginCache, resolver, dashboard.NewRBACResolver(mysqlStore), cfg.OperationBaseURL)
		options = append(options,
			compatserver.WithLotteryPageHandler(dashboard.NewLotteryPageHandler()),
			compatserver.WithLotteryIndexHandler(http.HandlerFunc(lottery.Index)),
			compatserver.WithLotteryStoreHandler(http.HandlerFunc(lottery.Store)),
			compatserver.WithLotteryShowContactHandler(http.HandlerFunc(lottery.ShowContact)),
			compatserver.WithLotteryShowHandler(http.HandlerFunc(lottery.Show)),
			compatserver.WithLotteryDestroyHandler(http.HandlerFunc(lottery.Destroy)),
			compatserver.WithLotteryShareHandler(http.HandlerFunc(lottery.Share)),
			compatserver.WithLotteryUpdateHandler(http.HandlerFunc(lottery.Update)),
			compatserver.WithLotteryInfoHandler(http.HandlerFunc(lottery.Info)),
			compatserver.WithLotteryWriteOffHandler(http.HandlerFunc(lottery.WriteOff)),
			compatserver.WithLotteryBatchContactTagsHandler(http.HandlerFunc(lottery.BatchContactTags)),
		)
		routeDebugf("go migrated route enabled: GET /dashboard/lottery/page")
		routeDebugf("go migrated route enabled: GET /dashboard/lottery/index")
		routeDebugf("go migrated route enabled: POST /dashboard/lottery/store")
		routeDebugf("go migrated route enabled: GET /dashboard/lottery/showContact")
		routeDebugf("go migrated route enabled: GET /dashboard/lottery/show")
		routeDebugf("go migrated route enabled: DELETE /dashboard/lottery/destroy")
		routeDebugf("go migrated route enabled: GET /dashboard/lottery/share")
		routeDebugf("go migrated route enabled: PUT /dashboard/lottery/update")
		routeDebugf("go migrated route enabled: GET /dashboard/lottery/info")
		routeDebugf("go migrated route enabled: GET /dashboard/lottery/writeOff")
		routeDebugf("go migrated route enabled: PUT /dashboard/lottery/batchContactTags")
	}

	if cfg.MigrateRoomFissionDashboard {
		mysqlStore := getMySQLStore()
		resolver, loginCache := buildUserResolver("roomFissionDashboard")
		roomFission := dashboard.NewRoomFissionHandler(mysqlStore, loginCache, resolver, dashboard.NewRBACResolver(mysqlStore), cfg.OperationBaseURL)
		options = append(options,
			compatserver.WithRoomFissionPageHandler(dashboard.NewRoomFissionPageHandler()),
			compatserver.WithRoomFissionIndexHandler(http.HandlerFunc(roomFission.Index)),
			compatserver.WithRoomFissionStoreHandler(http.HandlerFunc(roomFission.Store)),
			compatserver.WithRoomFissionInfoHandler(http.HandlerFunc(roomFission.Info)),
			compatserver.WithRoomFissionUpdateHandler(http.HandlerFunc(roomFission.Update)),
			compatserver.WithRoomFissionDestroyHandler(http.HandlerFunc(roomFission.Destroy)),
			compatserver.WithRoomFissionInviteHandler(http.HandlerFunc(roomFission.Invite)),
			compatserver.WithRoomFissionShowHandler(http.HandlerFunc(roomFission.Show)),
			compatserver.WithRoomFissionShowRoomHandler(http.HandlerFunc(roomFission.ShowRoom)),
			compatserver.WithRoomFissionShowContactHandler(http.HandlerFunc(roomFission.ShowContact)),
			compatserver.WithRoomFissionWriteOffHandler(http.HandlerFunc(roomFission.WriteOff)),
		)
		routeDebugf("go migrated route enabled: GET /dashboard/roomFission/page")
		routeDebugf("go migrated route enabled: GET /dashboard/roomFission/index")
		routeDebugf("go migrated route enabled: POST /dashboard/roomFission/store")
		routeDebugf("go migrated route enabled: GET /dashboard/roomFission/info")
		routeDebugf("go migrated route enabled: PUT /dashboard/roomFission/update")
		routeDebugf("go migrated route enabled: DELETE /dashboard/roomFission/destroy")
		routeDebugf("go migrated route enabled: POST /dashboard/roomFission/invite")
		routeDebugf("go migrated route enabled: GET /dashboard/roomFission/show")
		routeDebugf("go migrated route enabled: GET /dashboard/roomFission/showRoom")
		routeDebugf("go migrated route enabled: GET /dashboard/roomFission/showContact")
		routeDebugf("go migrated route enabled: GET /dashboard/roomFission/writeOff")
	}

	if cfg.MigrateRoomClockInDashboard {
		mysqlStore := getMySQLStore()
		resolver, loginCache := buildUserResolver("roomClockInDashboard")
		roomClockIn := dashboard.NewRoomClockInHandler(mysqlStore, loginCache, resolver, dashboard.NewRBACResolver(mysqlStore), cfg.OperationBaseURL)
		options = append(options,
			compatserver.WithRoomClockInPageHandler(dashboard.NewRoomClockInPageHandler()),
			compatserver.WithRoomClockInIndexHandler(http.HandlerFunc(roomClockIn.Index)),
			compatserver.WithRoomClockInStoreHandler(http.HandlerFunc(roomClockIn.Store)),
			compatserver.WithRoomClockInUpdateHandler(http.HandlerFunc(roomClockIn.Update)),
			compatserver.WithRoomClockInDestroyHandler(http.HandlerFunc(roomClockIn.Destroy)),
			compatserver.WithRoomClockInShowHandler(http.HandlerFunc(roomClockIn.Show)),
			compatserver.WithRoomClockInShowContactHandler(http.HandlerFunc(roomClockIn.ShowContact)),
			compatserver.WithRoomClockInBatchContactTagsHandler(http.HandlerFunc(roomClockIn.BatchContactTags)),
			compatserver.WithRoomClockInInfoHandler(http.HandlerFunc(roomClockIn.Info)),
			compatserver.WithRoomClockInDayDetailHandler(http.HandlerFunc(roomClockIn.DayDetail)),
		)
		routeDebugf("go migrated route enabled: GET /dashboard/roomClockIn/page")
		routeDebugf("go migrated route enabled: GET /dashboard/roomClockIn/index")
		routeDebugf("go migrated route enabled: POST /dashboard/roomClockIn/store")
		routeDebugf("go migrated route enabled: PUT /dashboard/roomClockIn/update")
		routeDebugf("go migrated route enabled: DELETE /dashboard/roomClockIn/destroy")
		routeDebugf("go migrated route enabled: GET /dashboard/roomClockIn/show")
		routeDebugf("go migrated route enabled: GET /dashboard/roomClockIn/showContact")
		routeDebugf("go migrated route enabled: PUT /dashboard/roomClockIn/batchContactTags")
		routeDebugf("go migrated route enabled: GET /dashboard/roomClockIn/info")
		routeDebugf("go migrated route enabled: GET /dashboard/roomClockIn/dayDetail")
	}

	if cfg.MigrateRoomQualityDashboard {
		mysqlStore := getMySQLStore()
		resolver, loginCache := buildUserResolver("roomQualityDashboard")
		roomQuality := dashboard.NewRoomQualityHandler(mysqlStore, loginCache, resolver, dashboard.NewRBACResolver(mysqlStore))
		options = append(options,
			compatserver.WithRoomQualityPageHandler(dashboard.NewRoomQualityPageHandler()),
			compatserver.WithRoomQualityIndexHandler(http.HandlerFunc(roomQuality.Index)),
			compatserver.WithRoomQualityStoreHandler(http.HandlerFunc(roomQuality.Store)),
			compatserver.WithRoomQualityStatusHandler(http.HandlerFunc(roomQuality.Status)),
			compatserver.WithRoomQualityInfoHandler(http.HandlerFunc(roomQuality.Info)),
			compatserver.WithRoomQualityUpdateHandler(http.HandlerFunc(roomQuality.Update)),
			compatserver.WithRoomQualityShowContactHandler(http.HandlerFunc(roomQuality.ShowContact)),
			compatserver.WithRoomQualityDestroyHandler(http.HandlerFunc(roomQuality.Destroy)),
			compatserver.WithRoomQualityContactDetailHandler(http.HandlerFunc(roomQuality.ContactDetail)),
		)
		routeDebugf("go migrated route enabled: GET /dashboard/roomQuality/page")
		routeDebugf("go migrated route enabled: GET /dashboard/roomQuality/index")
		routeDebugf("go migrated route enabled: POST /dashboard/roomQuality/store")
		routeDebugf("go migrated route enabled: PUT /dashboard/roomQuality/status")
		routeDebugf("go migrated route enabled: GET /dashboard/roomQuality/info")
		routeDebugf("go migrated route enabled: PUT /dashboard/roomQuality/update")
		routeDebugf("go migrated route enabled: GET /dashboard/roomQuality/showContact")
		routeDebugf("go migrated route enabled: DELETE /dashboard/roomQuality/destroy")
		routeDebugf("go migrated route enabled: GET /dashboard/roomQuality/contactDetail")
	}

	if cfg.MigrateRoomCalendarDashboard {
		mysqlStore := getMySQLStore()
		resolver, loginCache := buildUserResolver("roomCalendarDashboard")
		roomCalendar := dashboard.NewRoomCalendarHandler(mysqlStore, loginCache, resolver, dashboard.NewRBACResolver(mysqlStore))
		options = append(options,
			compatserver.WithRoomCalendarPageHandler(dashboard.NewRoomCalendarPageHandler()),
			compatserver.WithRoomCalendarIndexHandler(http.HandlerFunc(roomCalendar.Index)),
			compatserver.WithRoomCalendarAddRoomHandler(http.HandlerFunc(roomCalendar.AddRoom)),
			compatserver.WithRoomCalendarDestroyRoomHandler(http.HandlerFunc(roomCalendar.DestroyRoom)),
			compatserver.WithRoomCalendarStoreHandler(http.HandlerFunc(roomCalendar.Store)),
			compatserver.WithRoomCalendarDestroyHandler(http.HandlerFunc(roomCalendar.Destroy)),
			compatserver.WithRoomCalendarShowHandler(http.HandlerFunc(roomCalendar.Show)),
			compatserver.WithRoomCalendarUpdateHandler(http.HandlerFunc(roomCalendar.Update)),
		)
		routeDebugf("go migrated route enabled: GET /dashboard/roomCalendar/page")
		routeDebugf("go migrated route enabled: GET /dashboard/roomCalendar/index")
		routeDebugf("go migrated route enabled: POST /dashboard/roomCalendar/addRoom")
		routeDebugf("go migrated route enabled: DELETE /dashboard/roomCalendar/destroyRoom")
		routeDebugf("go migrated route enabled: POST /dashboard/roomCalendar/store")
		routeDebugf("go migrated route enabled: DELETE /dashboard/roomCalendar/destroy")
		routeDebugf("go migrated route enabled: GET /dashboard/roomCalendar/show")
		routeDebugf("go migrated route enabled: PUT /dashboard/roomCalendar/update")
	}

	if cfg.MigrateRoomRemindDashboard {
		mysqlStore := getMySQLStore()
		resolver, loginCache := buildUserResolver("roomRemindDashboard")
		roomRemind := dashboard.NewRoomRemindHandler(mysqlStore, loginCache, resolver, dashboard.NewRBACResolver(mysqlStore))
		options = append(options,
			compatserver.WithRoomRemindPageHandler(dashboard.NewRoomRemindPageHandler()),
			compatserver.WithRoomRemindIndexHandler(http.HandlerFunc(roomRemind.Index)),
			compatserver.WithRoomRemindDestroyHandler(http.HandlerFunc(roomRemind.Destroy)),
			compatserver.WithRoomRemindInfoHandler(http.HandlerFunc(roomRemind.Info)),
			compatserver.WithRoomRemindStatusHandler(http.HandlerFunc(roomRemind.Status)),
			compatserver.WithRoomRemindStoreHandler(http.HandlerFunc(roomRemind.Store)),
			compatserver.WithRoomRemindUpdateHandler(http.HandlerFunc(roomRemind.Update)),
			compatserver.WithRoomRemindTaskHandler(http.HandlerFunc(roomRemind.Task)),
		)
		routeDebugf("go migrated route enabled: GET /dashboard/roomRemind/page")
		routeDebugf("go migrated route enabled: GET /dashboard/roomRemind/index")
		routeDebugf("go migrated route enabled: DELETE /dashboard/roomRemind/destroy")
		routeDebugf("go migrated route enabled: GET /dashboard/roomRemind/info")
		routeDebugf("go migrated route enabled: GET /dashboard/roomRemind/status")
		routeDebugf("go migrated route enabled: POST /dashboard/roomRemind/store")
		routeDebugf("go migrated route enabled: PUT /dashboard/roomRemind/update")
		routeDebugf("go migrated route enabled: GET /dashboard/task/roomRemind")
	}

	if cfg.MigrateRoomInfinitePullDashboard {
		mysqlStore := getMySQLStore()
		resolver, loginCache := buildUserResolver("roomInfinitePullDashboard")
		roomInfinitePull := dashboard.NewRoomInfinitePullHandler(mysqlStore, loginCache, resolver, dashboard.NewRBACResolver(mysqlStore))
		options = append(options,
			compatserver.WithRoomInfinitePullPageHandler(dashboard.NewRoomInfinitePullPageHandler()),
			compatserver.WithRoomInfinitePullIndexHandler(http.HandlerFunc(roomInfinitePull.Index)),
			compatserver.WithRoomInfinitePullInfoHandler(http.HandlerFunc(roomInfinitePull.Info)),
			compatserver.WithRoomInfinitePullUpdateHandler(http.HandlerFunc(roomInfinitePull.Update)),
			compatserver.WithRoomInfinitePullDestroyHandler(http.HandlerFunc(roomInfinitePull.Destroy)),
			compatserver.WithRoomInfinitePullStoreHandler(http.HandlerFunc(roomInfinitePull.Store)),
		)
		routeDebugf("go migrated route enabled: GET /dashboard/roomInfinitePull/page")
		routeDebugf("go migrated route enabled: GET /dashboard/roomInfinitePull/index")
		routeDebugf("go migrated route enabled: GET /dashboard/roomInfinitePull/info")
		routeDebugf("go migrated route enabled: PUT /dashboard/roomInfinitePull/update")
		routeDebugf("go migrated route enabled: DELETE /dashboard/roomInfinitePull/destroy")
		routeDebugf("go migrated route enabled: POST /dashboard/roomInfinitePull/store")
	}

	if cfg.MigrateSidebarContactSOPInfo || cfg.MigrateSidebarContactSOPTipInfo {
		mysqlStore := getMySQLStore()
		contactSOP := dashboard.NewContactSOPHandler(mysqlStore, buildSidebarEmployeeResolver("sidebarContactSOP"), cfg.APIBaseURL)
		if cfg.MigrateSidebarContactSOPInfo {
			options = append(options, compatserver.WithSidebarContactSOPGetInfoHandler(http.HandlerFunc(contactSOP.GetSOPInfo)))
			routeDebugf("go migrated route enabled: GET /sidebar/contactSop/getSopInfo")
		}
		if cfg.MigrateSidebarContactSOPTipInfo {
			options = append(options, compatserver.WithSidebarContactSOPGetTipInfoHandler(http.HandlerFunc(contactSOP.GetSOPTipInfo)))
			routeDebugf("go migrated route enabled: GET /sidebar/contactSop/getSopTipInfo")
		}
	}

	if cfg.MigrateSidebarRoomSOPInfo || cfg.MigrateSidebarRoomSOPLogState {
		mysqlStore := getMySQLStore()
		roomSOP := dashboard.NewRoomSOPHandler(mysqlStore, buildSidebarEmployeeResolver("sidebarRoomSOP"), cfg.APIBaseURL)
		if cfg.MigrateSidebarRoomSOPInfo {
			options = append(options, compatserver.WithSidebarRoomSOPGetInfoHandler(http.HandlerFunc(roomSOP.GetSOPInfo)))
			routeDebugf("go migrated route enabled: GET /sidebar/roomSop/getSopInfo")
		}
		if cfg.MigrateSidebarRoomSOPLogState {
			options = append(options, compatserver.WithSidebarRoomSOPLogStateHandler(http.HandlerFunc(roomSOP.LogState)))
			routeDebugf("go migrated route enabled: PUT /sidebar/roomSop/logState")
		}
	}

	if cfg.MigrateSidebarWorkbench {
		mysqlStore := getMySQLStore()
		workbench := dashboard.NewSidebarWorkbenchHandler(mysqlStore, buildSidebarEmployeeResolver("sidebarWorkbench"))
		options = append(options,
			compatserver.WithSidebarWorkbenchSummaryHandler(http.HandlerFunc(workbench.Summary)),
			compatserver.WithSidebarWorkContactIndexHandler(http.HandlerFunc(workbench.Contacts)),
			compatserver.WithSidebarWorkbenchTasksHandler(http.HandlerFunc(workbench.Tasks)),
		)
		routeDebugf("go migrated route enabled: GET /sidebar/workbench/summary")
		routeDebugf("go migrated route enabled: GET /sidebar/workContact/index")
		routeDebugf("go migrated route enabled: GET /sidebar/workbench/tasks")
	}

	if cfg.MigrateChatToolConfig {
		mysqlStore := getMySQLStore()
		resolver, loginCache := buildUserResolver("chatToolConfig")
		options = append(options, compatserver.WithChatToolConfigHandler(
			dashboard.NewChatToolConfigHandler(mysqlStore, loginCache, resolver, cfg.SidebarBaseURL, cfg.APIBaseURL),
		))
		routeDebugf("go migrated route enabled: GET /dashboard/chatTool/config")
	}

	if cfg.MigrateCommonUpload || cfg.MigrateCommonUploadFile {
		mysqlStore := getMySQLStore()
		resolver, _ := buildUserResolver("commonUpload")
		commonUpload := dashboard.NewCommonUploadHandlerWithStore(cfg.FileStorageRoot, cfg.APIBaseURL, resolver, mysqlStore)
		if cfg.MigrateCommonUpload {
			options = append(options, compatserver.WithCommonUploadHandler(http.HandlerFunc(commonUpload.DashboardUpload)))
			routeDebugf("go migrated route enabled: POST /dashboard/common/upload")
		}
		if cfg.MigrateCommonUploadFile {
			options = append(options, compatserver.WithCommonUploadFileHandler(http.HandlerFunc(commonUpload.DashboardUploadFile)))
			routeDebugf("go migrated route enabled: POST /dashboard/common/uploadFile")
		}
	}

	if cfg.MigrateSidebarCommonUpload {
		mysqlStore := getMySQLStore()
		resolver := buildSidebarEmployeeResolver("sidebarCommonUpload")
		sidebarUpload := dashboard.NewCommonUploadHandlerWithStore(cfg.FileStorageRoot, cfg.APIBaseURL, resolver, mysqlStore)
		options = append(options, compatserver.WithSidebarCommonUploadHandler(http.HandlerFunc(sidebarUpload.SidebarUpload)))
		routeDebugf("go migrated route enabled: POST /sidebar/common/upload")
	}

	if cfg.MigrateAgentTxtVerify {
		options = append(options, compatserver.WithAgentTxtVerify())
		routeDebugf("go migrated route enabled: GET /{wxVerifyTxt:WW_verify_[0-9a-zA-Z]{16}.txt}")
	}

	if cfg.MigrateAgentTxtUpload {
		options = append(options, compatserver.WithAgentTxtUploadHandler(
			dashboard.NewAgentTxtVerifyUploadHandler(cfg.FileStorageRoot),
		))
		routeDebugf("go migrated route enabled: POST /dashboard/agent/txtVerifyUpload")
	}

	if cfg.MigrateAgentStore {
		mysqlStore := getMySQLStore()
		resolver, loginCache := buildUserResolver("agentStore")
		agent := dashboard.NewDashboardAgentHandler(mysqlStore, loginCache, resolver, dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL))
		options = append(options, compatserver.WithDashboardAgentStoreHandler(http.HandlerFunc(agent.Store)))
		routeDebugf("go migrated route enabled: POST /dashboard/agent/store")
	}

	if cfg.MigrateSidebarAgentAuth || cfg.MigrateSidebarAgentOAuth || cfg.MigrateSidebarAgentJSSDK || cfg.MigrateSidebarWxJSSDK {
		mysqlStore := getMySQLStore()
		sidebarResolver := buildSidebarEmployeeResolver("sidebarAgent")
		sidebarAgent := dashboard.NewSidebarAgentHandler(
			mysqlStore,
			dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL),
			sidebarResolver,
			cfg.APIBaseURL,
			cfg.SidebarBaseURL,
			cfg.SimpleJWTSecret,
			cfg.SidebarJWTSecret,
			cfg.SimpleJWTTTL,
		)
		if cfg.MigrateSidebarAgentAuth {
			options = append(options, compatserver.WithSidebarAgentAuthHandler(http.HandlerFunc(sidebarAgent.Auth)))
			routeDebugf("go migrated route enabled: GET|POST /sidebar/agent/auth")
		}
		if cfg.MigrateSidebarAgentOAuth {
			options = append(options, compatserver.WithSidebarAgentOAuthHandler(http.HandlerFunc(sidebarAgent.OAuth)))
			routeDebugf("go migrated route enabled: GET /sidebar/agent/oauth")
		}
		if cfg.MigrateSidebarAgentJSSDK {
			options = append(options, compatserver.WithSidebarAgentJSSDKHandler(http.HandlerFunc(sidebarAgent.AgentJSSDKConfig)))
			routeDebugf("go migrated route enabled: GET /sidebar/agent/jssdkConfig")
		}
		if cfg.MigrateSidebarWxJSSDK {
			options = append(options, compatserver.WithSidebarWxJSSDKHandler(http.HandlerFunc(sidebarAgent.WxJSSDKConfig)))
			routeDebugf("go migrated route enabled: GET /sidebar/wxJsSdk/config")
		}
	}

	if cfg.MigrateRoleSelect {
		mysqlStore := getMySQLStore()
		resolver, _ := buildUserResolver("roleSelect")
		options = append(options, compatserver.WithRoleSelectHandler(
			dashboard.NewRoleSelectHandler(mysqlStore, resolver),
		))
		routeDebugf("go migrated route enabled: GET /dashboard/role/select")
	}

	if cfg.MigrateRoleIndex || cfg.MigrateRoleShow || cfg.MigrateRolePermission || cfg.MigrateRoleShowEmployee || cfg.MigrateRoleStore || cfg.MigrateRoleUpdate || cfg.MigrateRoleStatusUpdate || cfg.MigrateRoleDestroy || cfg.MigrateRolePermissionStore {
		mysqlStore := getMySQLStore()
		resolver, loginCache := buildUserResolver("roleAdmin")
		roleAdmin := dashboard.NewRoleAdminHandler(mysqlStore, loginCache, resolver, dashboard.NewRBACResolver(mysqlStore))
		if cfg.MigrateRoleIndex {
			options = append(options, compatserver.WithRoleIndexHandler(http.HandlerFunc(roleAdmin.Index)))
			routeDebugf("go migrated route enabled: GET /dashboard/role/index")
		}
		if cfg.MigrateRoleShow {
			options = append(options, compatserver.WithRoleShowHandler(http.HandlerFunc(roleAdmin.Show)))
			routeDebugf("go migrated route enabled: GET /dashboard/role/show")
		}
		if cfg.MigrateRolePermission {
			options = append(options, compatserver.WithRolePermissionShowHandler(http.HandlerFunc(roleAdmin.PermissionShow)))
			routeDebugf("go migrated route enabled: GET /dashboard/role/permissionShow")
		}
		if cfg.MigrateRoleShowEmployee {
			options = append(options, compatserver.WithRoleShowEmployeeHandler(http.HandlerFunc(roleAdmin.ShowEmployee)))
			routeDebugf("go migrated route enabled: GET /dashboard/role/showEmployee")
		}
		if cfg.MigrateRoleStore {
			options = append(options, compatserver.WithRoleStoreHandler(http.HandlerFunc(roleAdmin.Store)))
			routeDebugf("go migrated route enabled: POST /dashboard/role/store")
		}
		if cfg.MigrateRoleUpdate {
			options = append(options, compatserver.WithRoleUpdateHandler(http.HandlerFunc(roleAdmin.Update)))
			routeDebugf("go migrated route enabled: PUT /dashboard/role/update")
		}
		if cfg.MigrateRoleStatusUpdate {
			options = append(options, compatserver.WithRoleStatusUpdateHandler(http.HandlerFunc(roleAdmin.StatusUpdate)))
			routeDebugf("go migrated route enabled: PUT /dashboard/role/statusUpdate")
		}
		if cfg.MigrateRoleDestroy {
			options = append(options, compatserver.WithRoleDestroyHandler(http.HandlerFunc(roleAdmin.Destroy)))
			routeDebugf("go migrated route enabled: DELETE /dashboard/role/destroy")
		}
		if cfg.MigrateRolePermissionStore {
			options = append(options, compatserver.WithRolePermissionStoreHandler(http.HandlerFunc(roleAdmin.PermissionStore)))
			routeDebugf("go migrated route enabled: POST /dashboard/role/permissionStore")
		}
	}

	if cfg.MigrateMenuIconIndex || cfg.MigrateMenuSelect {
		mysqlStore := getMySQLStore()
		resolver, _ := buildUserResolver("menuRead")
		menuRead := dashboard.NewMenuReadHandler(mysqlStore, resolver)
		if cfg.MigrateMenuIconIndex {
			options = append(options, compatserver.WithMenuIconIndexHandler(http.HandlerFunc(menuRead.IconIndex)))
			routeDebugf("go migrated route enabled: GET /dashboard/menu/iconIndex")
		}
		if cfg.MigrateMenuSelect {
			options = append(options, compatserver.WithMenuSelectHandler(http.HandlerFunc(menuRead.Select)))
			routeDebugf("go migrated route enabled: GET /dashboard/menu/select")
		}
	}

	if cfg.MigrateMenuIndex || cfg.MigrateMenuShow || cfg.MigrateMenuStore || cfg.MigrateMenuUpdate || cfg.MigrateMenuStatusUpdate || cfg.MigrateMenuDestroy {
		mysqlStore := getMySQLStore()
		resolver, loginCache := buildUserResolver("menuAdmin")
		menuAdmin := dashboard.NewMenuAdminHandler(mysqlStore, loginCache, resolver, dashboard.NewRBACResolver(mysqlStore))
		if cfg.MigrateMenuIndex {
			options = append(options, compatserver.WithMenuIndexHandler(http.HandlerFunc(menuAdmin.Index)))
			routeDebugf("go migrated route enabled: GET /dashboard/menu/index")
		}
		if cfg.MigrateMenuShow {
			options = append(options, compatserver.WithMenuShowHandler(http.HandlerFunc(menuAdmin.Show)))
			routeDebugf("go migrated route enabled: GET /dashboard/menu/show")
		}
		if cfg.MigrateMenuStore {
			options = append(options, compatserver.WithMenuStoreHandler(http.HandlerFunc(menuAdmin.Store)))
			routeDebugf("go migrated route enabled: POST /dashboard/menu/store")
		}
		if cfg.MigrateMenuUpdate {
			options = append(options, compatserver.WithMenuUpdateHandler(http.HandlerFunc(menuAdmin.Update)))
			routeDebugf("go migrated route enabled: PUT /dashboard/menu/update")
		}
		if cfg.MigrateMenuStatusUpdate {
			options = append(options, compatserver.WithMenuStatusUpdateHandler(http.HandlerFunc(menuAdmin.StatusUpdate)))
			routeDebugf("go migrated route enabled: PUT /dashboard/menu/statusUpdate")
		}
		if cfg.MigrateMenuDestroy {
			options = append(options, compatserver.WithMenuDestroyHandler(http.HandlerFunc(menuAdmin.Destroy)))
			routeDebugf("go migrated route enabled: DELETE /dashboard/menu/destroy")
		}
	}

	if cfg.EnableSaaSAlertDashboard {
		mysqlStore := getMySQLStore()
		saasAlert := dashboard.NewSaaSAlertHandler(mysqlStore, nil, cfg.SaaSPlatformAdminTenantID).WithWebhookGuard(saasAlertWebhookGuard)
		options = append(options,
			compatserver.WithSaaSAlertPageHandler(dashboard.NewSaaSAlertPageHandler()),
			compatserver.WithSaaSAlertIndexHandler(http.HandlerFunc(saasAlert.Index)),
			compatserver.WithSaaSAlertResolveHandler(http.HandlerFunc(saasAlert.Resolve)),
			compatserver.WithSaaSAlertSettingHandler(http.HandlerFunc(saasAlert.Setting)),
		)
		routeDebugf("go SaaS alert route enabled: GET /dashboard/saasAlert/page")
		routeDebugf("go SaaS alert route enabled: GET /dashboard/saasAlert/index")
		routeDebugf("go SaaS alert route enabled: PUT/POST /dashboard/saasAlert/resolve")
		routeDebugf("go SaaS alert route enabled: GET/PUT/POST /dashboard/saasAlert/setting")
	}

	if cfg.EnableSaaSBillingPortal {
		mysqlStore := getMySQLStore()
		saasBilling := dashboard.NewSaaSBillingHandler(mysqlStore, nil, cfg.SaaSPlatformAdminTenantID)
		options = append(options,
			compatserver.WithSaaSBillingPageHandler(dashboard.NewSaaSBillingPageHandler()),
			compatserver.WithSaaSBillingSummaryHandler(http.HandlerFunc(saasBilling.Summary)),
			compatserver.WithSaaSBillingPaymentOrdersHandler(http.HandlerFunc(saasBilling.PaymentOrders)),
			compatserver.WithSaaSBillingPaymentRefundsHandler(http.HandlerFunc(saasBilling.PaymentRefunds)),
			compatserver.WithSaaSBillingInvoiceProfileHandler(http.HandlerFunc(saasBilling.InvoiceProfile)),
			compatserver.WithSaaSBillingInvoicesHandler(http.HandlerFunc(saasBilling.Invoices)),
			compatserver.WithSaaSBillingInvoiceHandler(http.HandlerFunc(saasBilling.CreateInvoice)),
			compatserver.WithSaaSBillingInvoiceCancelHandler(http.HandlerFunc(saasBilling.CancelInvoice)),
		)
		routeDebugf("go SaaS billing route enabled: GET /dashboard/saasBilling/page")
		routeDebugf("go SaaS billing route enabled: GET /dashboard/saasBilling/summary")
		routeDebugf("go SaaS billing route enabled: GET /dashboard/saasBilling/paymentOrders")
		routeDebugf("go SaaS billing route enabled: GET /dashboard/saasBilling/paymentRefunds")
		routeDebugf("go SaaS billing route enabled: GET/POST/PUT /dashboard/saasBilling/invoiceProfile")
		routeDebugf("go SaaS billing route enabled: GET /dashboard/saasBilling/invoices")
		routeDebugf("go SaaS billing route enabled: POST/PUT /dashboard/saasBilling/invoice")
		routeDebugf("go SaaS billing route enabled: POST/PUT /dashboard/saasBilling/invoiceCancel")
	}

	var paymentSettlementSyncService *dashboard.SaaSPaymentSettlementSyncService
	if (cfg.EnableSaaSAdminDashboard || cfg.EnableSaaSPaymentSettlementSyncCron) && strings.TrimSpace(cfg.SaaSPaymentSettlementBridgeBaseURL) != "" {
		bridgeClient, err := dashboard.NewSaaSPaymentSettlementHTTPBridgeClient(
			cfg.SaaSPaymentSettlementBridgeBaseURL,
			cfg.SaaSPaymentSettlementBridgeToken,
			cfg.SaaSPaymentSettlementBridgeTimeout,
		)
		if err != nil {
			fatalf("build SaaS payment settlement bridge client: %v", err)
		}
		paymentSettlementSyncService, err = dashboard.NewSaaSPaymentSettlementSyncService(
			getMySQLStore(), bridgeClient, cfg.SaaSPaymentSettlementProviders, cfg.SaaSPaymentSettlementSyncLimit,
			cfg.SaaSPlatformAdminTenantID, cfg.SaaSAlertNotificationMaxAttempts, log.Default(),
		)
		if err != nil {
			fatalf("build SaaS payment settlement sync service: %v", err)
		}
		debugf("SaaS payment settlement bridge enabled: providers=%s limit=%d timeout=%s", strings.Join(cfg.SaaSPaymentSettlementProviders, ","), cfg.SaaSPaymentSettlementSyncLimit, cfg.SaaSPaymentSettlementBridgeTimeout)
	}

	var tenantDomainDeliveryProcessor *dashboard.SaaSTenantDomainDeliveryProcessor
	if (cfg.EnableSaaSAdminDashboard || cfg.EnableSaaSTenantDomainDeliveryCron) && strings.TrimSpace(cfg.SaaSTenantDomainDeliveryBridgeBaseURL) != "" {
		bridgeClient, err := dashboard.NewSaaSTenantDomainDeliveryHTTPBridgeClient(
			cfg.SaaSTenantDomainDeliveryBridgeBaseURL,
			cfg.SaaSTenantDomainDeliveryBridgeToken,
			cfg.SaaSTenantDomainDeliveryBridgeTimeout,
		)
		if err != nil {
			fatalf("build SaaS tenant domain delivery bridge client: %v", err)
		}
		tenantDomainDeliveryProcessor = dashboard.NewSaaSTenantDomainDeliveryProcessor(
			getMySQLStore(), bridgeClient, cfg.SaaSTenantDomainDeliveryCallbackURL,
			cfg.SaaSTenantDomainDeliveryLimit, cfg.SaaSTenantDomainDeliveryLease,
			cfg.SaaSTenantDomainDeliveryRetryDelay, cfg.SaaSTenantDomainDeliveryCallbackWait, log.Default(),
		)
		debugf("SaaS tenant domain delivery bridge enabled: limit=%d timeout=%s callback=%t", cfg.SaaSTenantDomainDeliveryLimit, cfg.SaaSTenantDomainDeliveryBridgeTimeout, cfg.SaaSTenantDomainDeliveryCallbackURL != "")
	}

	var backupManager *saasbackup.Manager
	if cfg.EnableSaaSAdminDashboard || cfg.EnableSaaSBackupCron {
		var backupReplica saasbackup.ReplicaStore
		if strings.TrimSpace(cfg.SaaSBackupS3Endpoint) != "" {
			backupReplica, err = saasbackup.NewS3ReplicaStore(saasbackup.S3ReplicaConfig{
				Endpoint: cfg.SaaSBackupS3Endpoint, Bucket: cfg.SaaSBackupS3Bucket, Region: cfg.SaaSBackupS3Region,
				AccessKeyID: cfg.SaaSBackupS3AccessKeyID, SecretAccessKey: cfg.SaaSBackupS3SecretAccessKey,
				SessionToken: cfg.SaaSBackupS3SessionToken, UseSSL: cfg.SaaSBackupS3UseSSL, Prefix: cfg.SaaSBackupS3Prefix,
			})
			if err != nil {
				fatalf("build SaaS backup replica store: %v", err)
			}
		}
		backupManager, err = saasbackup.NewManager(getMySQLStore(), saasbackup.Config{
			SourceDSN: cfg.MySQLDSN, BackupRoot: cfg.SaaSBackupRoot, EncryptionKey: cfg.SaaSBackupEncryptionKey,
			EncryptionKeys:  cfg.SaaSBackupEncryptionKeys,
			EncryptionKeyID: cfg.SaaSBackupEncryptionKeyID, CronEnabled: cfg.EnableSaaSBackupCron,
			CronInterval: cfg.SaaSBackupCronInterval, CronRunOnStart: cfg.SaaSBackupCronRunOnStart,
			DumpBinary:    cfg.SaaSBackupDumpBinary,
			RestoreBinary: cfg.SaaSBackupRestoreBinary, RestoreDSN: cfg.SaaSBackupRestoreDSN,
			RestoreAdminDSN: cfg.SaaSBackupRestoreAdminDSN, RestoreAutoProvision: cfg.SaaSBackupRestoreAutoProvision,
			RestoreKeepOnFailure:  cfg.SaaSBackupRestoreKeepOnFailure,
			RestoreDatabasePrefix: cfg.SaaSBackupRestoreDatabasePrefix, Replica: backupReplica,
		})
		if err != nil {
			fatalf("build SaaS backup manager: %v", err)
		}
		backupStatus := backupManager.ConfigStatus()
		debugf("SaaS backup manager enabled: encryption_configured=%v key_count=%d cron_enabled=%v cron_interval=%s cron_run_on_start=%v restore_configured=%v restore_auto=%v replica_configured=%v replica_provider=%s dump_tool_ready=%v restore_tool_ready=%v",
			backupStatus.EncryptionConfigured, backupStatus.EncryptionKeyCount,
			backupStatus.CronEnabled, cfg.SaaSBackupCronInterval, backupStatus.CronRunOnStart,
			backupStatus.RestoreConfigured, backupStatus.RestoreAutoProvision, backupStatus.ReplicaConfigured, backupStatus.ReplicaProvider,
			backupStatus.DumpToolReady, backupStatus.RestoreToolReady)
	}

	var complianceManager *saascompliance.Manager
	if cfg.EnableSaaSAdminDashboard || cfg.EnableSaaSComplianceCron {
		complianceManager, err = saascompliance.NewManager(getMySQLStore(), saascompliance.Config{
			ArtifactRoot: cfg.SaaSComplianceArtifactRoot, FileStorageRoot: cfg.FileStorageRoot,
			EncryptionKey: cfg.SaaSComplianceEncryptionKey, EncryptionKeys: cfg.SaaSComplianceEncryptionKeys,
			EncryptionKeyID: cfg.SaaSComplianceEncryptionKeyID, PlatformTenantID: cfg.SaaSPlatformAdminTenantID,
		})
		if err != nil {
			fatalf("build SaaS compliance manager: %v", err)
		}
		debugf("SaaS compliance manager enabled: artifact_root=%s key_id=%s", cfg.SaaSComplianceArtifactRoot, cfg.SaaSComplianceEncryptionKeyID)
	}

	var auditAnchorManager *saasauditanchor.Manager
	if cfg.EnableSaaSAdminDashboard || cfg.EnableSaaSAuditAnchorCron {
		var auditAnchorRemote saasauditanchor.RemoteArtifactStore
		if strings.TrimSpace(cfg.SaaSAuditAnchorS3Endpoint) != "" {
			auditAnchorRemote, err = saasauditanchor.NewS3RemoteArtifactStore(saasauditanchor.S3RemoteArtifactConfig{
				Endpoint: cfg.SaaSAuditAnchorS3Endpoint, Bucket: cfg.SaaSAuditAnchorS3Bucket,
				Region: cfg.SaaSAuditAnchorS3Region, AccessKeyID: cfg.SaaSAuditAnchorS3AccessKeyID,
				SecretAccessKey: cfg.SaaSAuditAnchorS3SecretAccessKey, SessionToken: cfg.SaaSAuditAnchorS3SessionToken,
				UseSSL: cfg.SaaSAuditAnchorS3UseSSL, Prefix: cfg.SaaSAuditAnchorS3Prefix,
				RetentionMode: cfg.SaaSAuditAnchorS3RetentionMode, RetentionDays: cfg.SaaSAuditAnchorS3RetentionDays,
			})
			if err != nil {
				fatalf("build SaaS audit anchor remote store: %v", err)
			}
		}
		auditAnchorManager, err = saasauditanchor.NewManager(getMySQLStore(), saasauditanchor.Config{
			ArtifactRoot: cfg.SaaSAuditAnchorArtifactRoot, HMACKey: cfg.SaaSAuditAnchorHMACKey,
			HMACKeys: cfg.SaaSAuditAnchorHMACKeys, HMACKeyID: cfg.SaaSAuditAnchorHMACKeyID,
			Remote: auditAnchorRemote, RequireRemote: cfg.SaaSAuditAnchorRequireRemote,
		})
		if err != nil {
			fatalf("build SaaS audit anchor manager: %v", err)
		}
		anchorStatus := auditAnchorManager.ConfigStatus()
		debugf("SaaS audit anchor manager enabled: artifact_root=%s hmac_configured=%v key_id=%s key_count=%d independent_storage_required=%v remote_configured=%v remote_required=%v remote_provider=%s remote_bucket=%s retention=%s/%dd",
			anchorStatus.ArtifactRoot, anchorStatus.HMACConfigured, anchorStatus.HMACKeyID,
			anchorStatus.HMACKeyCount, anchorStatus.IndependentStorageRequired,
			anchorStatus.RemoteConfigured, anchorStatus.RemoteRequired, anchorStatus.RemoteProvider,
			anchorStatus.RemoteBucket, anchorStatus.RemoteRetentionMode, anchorStatus.RemoteRetentionDays)
	}

	healthRedisStore := store.NewRedisStore(store.RedisConfig{Addr: cfg.RedisAddr, Password: cfg.RedisPassword, DB: cfg.RedisDB})
	newSaaSAdminHandler := func() *dashboard.SaaSAdminHandler {
		handler := dashboard.NewSaaSAdminHandler(getMySQLStore(), nil, cfg.SaaSPlatformAdminTenantID, cfg.SimpleJWTSecret).
			WithPaymentSettlementSyncService(paymentSettlementSyncService).
			WithBackupManager(backupManager).
			WithAuditAnchorManager(auditAnchorManager).
			WithComplianceManager(complianceManager).
			WithIdentitySecurityManager(identityManager).
			WithTenantDomainVerifier(tenantDomainVerifier).
			WithReleaseSourceFingerprint(cfg.SaaSReleaseSourceFingerprint, cfg.SaaSReleaseSourceFingerprintSource).
			WithReleaseEvidenceVerifier(releaseEvidenceVerifier).
			WithWebhookGuard(saasAlertWebhookGuard).
			WithHighRiskApprovalRequired(cfg.SaaSAdminApprovalRequired).
			WithServiceAccountKeyManager(serviceAccountKeyManager).
			WithServiceAccountClientIPResolver(serviceAccountClientIPResolver).
			WithSystemHealthNotificationMaxAttempts(cfg.SaaSAlertNotificationMaxAttempts).
			WithSystemHealthProbe(dashboard.SaaSAdminSystemHealthProbe{
				Code: "redis_connection", Name: "Redis 连接", Category: "runtime", Severity: dashboard.SaaSAdminSystemHealthStateCritical,
				Check: healthRedisStore.Ping,
			})
		if serviceAccountKeyManager != nil {
			handler.WithSystemHealthProbe(dashboard.SaaSAdminSystemHealthProbe{
				Code: "service_account_key_protection", Name: "API Key pepper 密钥环", Category: "integrations", Severity: dashboard.SaaSAdminSystemHealthStateCritical,
				Check: handler.CheckServiceAccountKeyProtection,
			})
		}
		if alertCredentialStatus.EncryptionConfigured || alertCredentialStatus.RequireEncryption {
			handler.WithSystemHealthProbe(dashboard.SaaSAdminSystemHealthProbe{
				Code: "saas_alert_credential_protection", Name: "通知凭据加密", Category: "notifications", Severity: dashboard.SaaSAdminSystemHealthStateCritical,
				Check: getMySQLStore().CheckSaaSAlertCredentialProtection,
			})
		}
		if weComCredentialStatus.EncryptionConfigured || weComCredentialStatus.RequireEncryption {
			handler.WithSystemHealthProbe(dashboard.SaaSAdminSystemHealthProbe{
				Code: "wecom_credential_protection", Name: "企业微信凭据加密", Category: "integrations", Severity: dashboard.SaaSAdminSystemHealthStateCritical,
				Check: getMySQLStore().CheckSaaSWeComCredentialProtection,
			})
		}
		if weChatOpenCredentialStatus.EncryptionConfigured || weChatOpenCredentialStatus.RequireEncryption {
			handler.WithSystemHealthProbe(dashboard.SaaSAdminSystemHealthProbe{
				Code: "wechat_open_credential_protection", Name: "微信开放平台凭据加密", Category: "integrations", Severity: dashboard.SaaSAdminSystemHealthStateCritical,
				Check: getMySQLStore().CheckSaaSWeChatOpenCredentialProtection,
			})
		}
		if backupManager != nil {
			handler.WithSystemHealthProbe(dashboard.SaaSAdminSystemHealthProbe{
				Code: "backup_automation", Name: "自动备份调度", Category: "disaster_recovery", Severity: dashboard.SaaSAdminSystemHealthStateCritical,
				Check: backupManager.CheckAutomation,
			})
			handler.WithSystemHealthProbe(dashboard.SaaSAdminSystemHealthProbe{
				Code: "backup_keyring", Name: "备份密钥环", Category: "disaster_recovery", Severity: dashboard.SaaSAdminSystemHealthStateCritical,
				Check: backupManager.CheckEncryptionKeyRing,
			})
			handler.WithSystemHealthProbe(dashboard.SaaSAdminSystemHealthProbe{
				Code: "backup_retention_cleanup", Name: "备份保留清理队列", Category: "disaster_recovery", Severity: dashboard.SaaSAdminSystemHealthStateCritical,
				Check: backupManager.CheckCleanupQueue,
			})
			if backupManager.ConfigStatus().ReplicaConfigured {
				handler.WithSystemHealthProbe(dashboard.SaaSAdminSystemHealthProbe{
					Code: "backup_replica_store", Name: "异地副本存储", Category: "disaster_recovery", Severity: dashboard.SaaSAdminSystemHealthStateCritical,
					Check: backupManager.ProbeReplica,
				})
			}
		}
		if complianceManager != nil {
			handler.WithSystemHealthProbe(dashboard.SaaSAdminSystemHealthProbe{
				Code: "compliance_configuration", Name: "租户数据合规配置", Category: "compliance", Severity: dashboard.SaaSAdminSystemHealthStateCritical,
				Check: complianceManager.CheckConfiguration,
			})
		}
		if auditAnchorManager != nil {
			handler.WithSystemHealthProbe(dashboard.SaaSAdminSystemHealthProbe{
				Code: "audit_anchor_configuration", Name: "审计签名锚点", Category: "audit", Severity: dashboard.SaaSAdminSystemHealthStateCritical,
				Check: auditAnchorManager.CheckConfiguration,
			})
			anchorStatus := auditAnchorManager.ConfigStatus()
			if anchorStatus.RemoteConfigured || anchorStatus.RemoteRequired {
				handler.WithSystemHealthProbe(dashboard.SaaSAdminSystemHealthProbe{
					Code: "audit_anchor_remote_store", Name: "审计锚点异地 Object Lock", Category: "audit", Severity: dashboard.SaaSAdminSystemHealthStateCritical,
					Check: auditAnchorManager.ProbeRemote,
				})
			}
		}
		if identityManager != nil {
			handler.WithSystemHealthProbe(dashboard.SaaSAdminSystemHealthProbe{
				Code: "identity_security_configuration", Name: "身份安全配置", Category: "identity", Severity: dashboard.SaaSAdminSystemHealthStateCritical,
				Check: identityManager.CheckConfiguration,
			})
		}
		if tenantDomainDeliveryProcessor != nil {
			handler.WithSystemHealthProbe(dashboard.SaaSAdminSystemHealthProbe{
				Code: "domain_delivery_configuration", Name: "域名交付 Bridge", Category: "domains", Severity: dashboard.SaaSAdminSystemHealthStateCritical,
				Check: tenantDomainDeliveryProcessor.CheckConfiguration,
			})
		}
		return handler
	}

	var saasAdminHandler *dashboard.SaaSAdminHandler
	if cfg.EnableSaaSAdminDashboard {
		saasAdminHandler = newSaaSAdminHandler()
		saasAdmin := saasAdminHandler
		if dashboardAdminService == nil {
			dashboardAdminService = dashboardadmin.NewService(getMySQLStore())
		}
		saasAdmin.WithDashboardAdminApprovalExecutor(func(ctx context.Context, actorUserID int, approvalID int64, approvalVersion int, actionType string, payload json.RawMessage) (map[string]any, error) {
			return dashboardAdminService.ExecuteApproval(ctx, dashboardadmin.NewSaaSApprovalExecutionActor(actorUserID), actionType, payload, approvalID, approvalVersion)
		})
		dashboardAdminHTTP := dashboardadmin.NewHTTPHandler(dashboardAdminService).WithWeComIntegration(dashboardadmin.NewWeComIntegrationService(getMySQLStore(), nil)).WithApprovalGate(func(ctx context.Context, actionType string) (dashboardadmin.ApprovalGateResult, error) {
			policy, required, err := saasAdmin.DirectSaaSAdminApprovalRequired(ctx, actionType)
			if err != nil {
				return dashboardadmin.ApprovalGateResult{}, err
			}
			return dashboardadmin.ApprovalGateResult{Required: required, ActionType: policy.ActionType, RequiredApprovals: policy.RequiredApprovals, ExpiryHours: policy.ExpiryHours}, nil
		})
		options = append(options,
			compatserver.WithSaaSAdminDashboardProvisioningHandler(dashboardAdminHTTP),
			compatserver.WithSaaSAdminPageHandler(dashboard.NewSaaSAdminPageHandler()),
			compatserver.WithSaaSAdminOverviewHandler(http.HandlerFunc(saasAdmin.Overview)),
			compatserver.WithSaaSAdminTenantReadinessHandler(http.HandlerFunc(saasAdmin.TenantReadiness)),
			compatserver.WithSaaSAdminTenantHandler(http.HandlerFunc(saasAdmin.TenantDetail)),
			compatserver.WithSaaSAdminTenantLifecycleHandler(http.HandlerFunc(saasAdmin.TenantLifecycle)),
			compatserver.WithSaaSAdminUsageHandler(http.HandlerFunc(saasAdmin.Usage)),
			compatserver.WithSaaSAdminRiskHandler(http.HandlerFunc(saasAdmin.Risk)),
			compatserver.WithSaaSAdminBusinessMetricsHandler(http.HandlerFunc(saasAdmin.BusinessMetrics)),
			compatserver.WithSaaSAdminBusinessTrendsHandler(http.HandlerFunc(saasAdmin.BusinessTrends)),
			compatserver.WithSaaSAdminOperationQueueHandler(http.HandlerFunc(saasAdmin.OperationQueue)),
			compatserver.WithSaaSAdminOperationQueueOwnersHandler(http.HandlerFunc(saasAdmin.OperationQueueOwners)),
			compatserver.WithSaaSAdminOperationQueueAssignmentsHandler(http.HandlerFunc(saasAdmin.OperationQueueAssignments)),
			compatserver.WithSaaSAdminOperationQueueAssignmentCloseHandler(http.HandlerFunc(saasAdmin.OperationQueueAssignmentClose)),
			compatserver.WithSaaSAdminOperationQueueAssignmentNotificationsHandler(http.HandlerFunc(saasAdmin.OperationQueueAssignmentNotifications)),
			compatserver.WithSaaSAdminOperationQueueAssignHandler(http.HandlerFunc(saasAdmin.OperationQueueAssign)),
			compatserver.WithSaaSAdminRenewalForecastHandler(http.HandlerFunc(saasAdmin.RenewalForecast)),
			compatserver.WithSaaSAdminRenewalForecastTasksHandler(http.HandlerFunc(saasAdmin.RenewalForecastTasks)),
			compatserver.WithSaaSAdminRenewalForecastAssignHandler(http.HandlerFunc(saasAdmin.RenewalForecastAssign)),
			compatserver.WithSaaSAdminRenewalForecastNotificationsHandler(http.HandlerFunc(saasAdmin.RenewalForecastNotifications)),
			compatserver.WithSaaSAdminCustomerSuccessHandler(http.HandlerFunc(saasAdmin.CustomerSuccess)),
			compatserver.WithSaaSAdminCustomerSuccessOwnersHandler(http.HandlerFunc(saasAdmin.CustomerSuccessOwners)),
			compatserver.WithSaaSAdminCustomerSuccessAssignHandler(http.HandlerFunc(saasAdmin.CustomerSuccessAssign)),
			compatserver.WithSaaSAdminCustomerSuccessRenewalTasksHandler(http.HandlerFunc(saasAdmin.CustomerSuccessRenewalTasks)),
			compatserver.WithSaaSAdminCustomerSuccessRenewalNotificationsHandler(http.HandlerFunc(saasAdmin.CustomerSuccessRenewalNotifications)),
			compatserver.WithSaaSAdminRiskFollowUpHandler(http.HandlerFunc(saasAdmin.RiskFollowUp)),
			compatserver.WithSaaSAdminRiskFollowUpsHandler(http.HandlerFunc(saasAdmin.RiskFollowUps)),
			compatserver.WithSaaSAdminRiskFollowUpOwnersHandler(http.HandlerFunc(saasAdmin.RiskFollowUpOwners)),
			compatserver.WithSaaSAdminRiskFollowUpBulkCloseHandler(http.HandlerFunc(saasAdmin.RiskFollowUpBulkClose)),
			compatserver.WithSaaSAdminAlertsHandler(http.HandlerFunc(saasAdmin.Alerts)),
			compatserver.WithSaaSAdminAlertResolveHandler(http.HandlerFunc(saasAdmin.ResolveAlert)),
			compatserver.WithSaaSAdminAlertBulkResolveHandler(http.HandlerFunc(saasAdmin.BulkResolveAlerts)),
			compatserver.WithSaaSAdminNotificationsHandler(http.HandlerFunc(saasAdmin.Notifications)),
			compatserver.WithSaaSAdminNotificationHealthHandler(http.HandlerFunc(saasAdmin.NotificationHealth)),
			compatserver.WithSaaSAdminNotificationSLOHandler(http.HandlerFunc(saasAdmin.NotificationSLO)),
			compatserver.WithSaaSAdminNotificationHealthRecoveryHandler(http.HandlerFunc(saasAdmin.NotificationHealthRecovery)),
			compatserver.WithSaaSAdminNotificationPoliciesHandler(http.HandlerFunc(saasAdmin.NotificationPolicies)),
			compatserver.WithSaaSAdminNotificationPolicyHandler(http.HandlerFunc(saasAdmin.NotificationPolicy)),
			compatserver.WithSaaSAdminNotificationPolicyTestHandler(http.HandlerFunc(saasAdmin.NotificationPolicyTest)),
			compatserver.WithSaaSAdminNotificationCredentialRotationHandler(http.HandlerFunc(saasAdmin.NotificationCredentialRotation)),
			compatserver.WithSaaSAdminWeComCredentialProtectionHandler(http.HandlerFunc(saasAdmin.WeComCredentialProtection)),
			compatserver.WithSaaSAdminWeComCredentialRotationHandler(http.HandlerFunc(saasAdmin.WeComCredentialRotation)),
			compatserver.WithSaaSAdminWeChatOpenCredentialProtectionHandler(http.HandlerFunc(saasAdmin.WeChatOpenCredentialProtection)),
			compatserver.WithSaaSAdminWeChatOpenCredentialRotationHandler(http.HandlerFunc(saasAdmin.WeChatOpenCredentialRotation)),
			compatserver.WithSaaSAdminNotificationRetryHandler(http.HandlerFunc(saasAdmin.RetryNotification)),
			compatserver.WithSaaSAdminNotificationBulkRetryHandler(http.HandlerFunc(saasAdmin.BulkRetryNotifications)),
			compatserver.WithSaaSAdminNotificationCloseHandler(http.HandlerFunc(saasAdmin.CloseNotification)),
			compatserver.WithSaaSAdminNotificationBulkCloseHandler(http.HandlerFunc(saasAdmin.BulkCloseNotifications)),
			compatserver.WithSaaSAdminPackagesHandler(http.HandlerFunc(saasAdmin.Packages)),
			compatserver.WithSaaSAdminSubscriptionsHandler(http.HandlerFunc(saasAdmin.Subscriptions)),
			compatserver.WithSaaSAdminSubscriptionEventsHandler(http.HandlerFunc(saasAdmin.SubscriptionEvents)),
			compatserver.WithSaaSAdminSubscriptionTransitionHandler(http.HandlerFunc(saasAdmin.TransitionSubscription)),
			compatserver.WithSaaSAdminSubscriptionReconcileHandler(http.HandlerFunc(saasAdmin.ReconcileSubscriptions)),
			compatserver.WithSaaSAdminPaymentOrdersHandler(http.HandlerFunc(saasAdmin.PaymentOrders)),
			compatserver.WithSaaSAdminPaymentWebhookEventsHandler(http.HandlerFunc(saasAdmin.PaymentWebhookEvents)),
			compatserver.WithSaaSAdminPaymentOrderHandler(http.HandlerFunc(saasAdmin.CreatePaymentOrder)),
			compatserver.WithSaaSAdminPaymentOrderCancelHandler(http.HandlerFunc(saasAdmin.CancelPaymentOrder)),
			compatserver.WithSaaSAdminPaymentDunningHandler(http.HandlerFunc(saasAdmin.PaymentDunning)),
			compatserver.WithSaaSAdminPaymentRefundsHandler(http.HandlerFunc(saasAdmin.PaymentRefunds)),
			compatserver.WithSaaSAdminPaymentRefundHandler(http.HandlerFunc(saasAdmin.CreatePaymentRefund)),
			compatserver.WithSaaSAdminPaymentRefundCancelHandler(http.HandlerFunc(saasAdmin.CancelPaymentRefund)),
			compatserver.WithSaaSAdminPaymentSettlementBatchesHandler(http.HandlerFunc(saasAdmin.PaymentSettlementBatches)),
			compatserver.WithSaaSAdminPaymentSettlementEntriesHandler(http.HandlerFunc(saasAdmin.PaymentSettlementEntries)),
			compatserver.WithSaaSAdminPaymentSettlementImportHandler(http.HandlerFunc(saasAdmin.ImportPaymentSettlement)),
			compatserver.WithSaaSAdminPaymentSettlementReconcileHandler(http.HandlerFunc(saasAdmin.ReconcilePaymentSettlement)),
			compatserver.WithSaaSAdminPaymentSettlementResolveHandler(http.HandlerFunc(saasAdmin.ResolvePaymentSettlementEntry)),
			compatserver.WithSaaSAdminPaymentSettlementTransitionHandler(http.HandlerFunc(saasAdmin.TransitionPaymentSettlement)),
			compatserver.WithSaaSAdminPaymentSettlementSyncRunsHandler(http.HandlerFunc(saasAdmin.PaymentSettlementSyncRuns)),
			compatserver.WithSaaSAdminPaymentSettlementSyncHandler(http.HandlerFunc(saasAdmin.PaymentSettlementSync)),
			compatserver.WithSaaSAdminAccessProfileHandler(http.HandlerFunc(saasAdmin.AccessProfile)),
			compatserver.WithSaaSAdminAccessRolesHandler(http.HandlerFunc(saasAdmin.AccessRoles)),
			compatserver.WithSaaSAdminAccessRoleHandler(http.HandlerFunc(saasAdmin.AccessRole)),
			compatserver.WithSaaSAdminAccessAssignmentsHandler(http.HandlerFunc(saasAdmin.AccessAssignments)),
			compatserver.WithSaaSAdminAccessAssignmentHandler(http.HandlerFunc(saasAdmin.AccessAssignment)),
			compatserver.WithSaaSAdminBrandingProfilesHandler(http.HandlerFunc(saasAdmin.BrandingProfiles)),
			compatserver.WithSaaSAdminBrandingProfileHandler(http.HandlerFunc(saasAdmin.BrandingProfile)),
			compatserver.WithSaaSAdminTenantDomainsHandler(http.HandlerFunc(saasAdmin.TenantDomains)),
			compatserver.WithSaaSAdminTenantDomainHandler(http.HandlerFunc(saasAdmin.TenantDomain)),
			compatserver.WithSaaSAdminTenantDomainDeliveryJobsHandler(http.HandlerFunc(saasAdmin.TenantDomainDeliveryJobs)),
			compatserver.WithSaaSAdminTenantDomainDeliveryHandler(http.HandlerFunc(saasAdmin.TenantDomainDelivery)),
			compatserver.WithSaaSAdminTenantAIProviderHandler(http.HandlerFunc(saasAdmin.TenantAIProvider)),
			compatserver.WithSaaSAdminReleaseReadinessHandler(http.HandlerFunc(saasAdmin.ReleaseReadiness)),
			compatserver.WithSaaSAdminReleaseEvidenceHandler(http.HandlerFunc(saasAdmin.ReleaseEvidence)),
			compatserver.WithSaaSAdminReleaseEvidenceActionHandler(http.HandlerFunc(saasAdmin.ReleaseEvidenceAction)),
			compatserver.WithSaaSAdminReleaseCandidateHandler(http.HandlerFunc(saasAdmin.ReleaseCandidate)),
			compatserver.WithSaaSAdminSystemHealthHandler(http.HandlerFunc(saasAdmin.SystemHealth)),
			compatserver.WithSaaSAdminSystemHealthScansHandler(http.HandlerFunc(saasAdmin.SystemHealthScans)),
			compatserver.WithSaaSAdminSystemIncidentsHandler(http.HandlerFunc(saasAdmin.SystemIncidents)),
			compatserver.WithSaaSAdminSystemHealthScanHandler(http.HandlerFunc(saasAdmin.SystemHealthScan)),
			compatserver.WithSaaSAdminSystemIncidentHandler(http.HandlerFunc(saasAdmin.SystemIncident)),
			compatserver.WithSaaSAdminServiceAccountsHandler(http.HandlerFunc(saasAdmin.ServiceAccounts)),
			compatserver.WithSaaSAdminServiceAccountUsageHandler(http.HandlerFunc(saasAdmin.ServiceAccountUsage)),
			compatserver.WithSaaSAdminServiceAccountUsageAlertEvaluateHandler(http.HandlerFunc(saasAdmin.ServiceAccountUsageAlertEvaluate)),
			compatserver.WithSaaSAdminServiceAccountHandler(http.HandlerFunc(saasAdmin.ServiceAccount)),
			compatserver.WithSaaSAdminServiceAccountKeyRotateHandler(http.HandlerFunc(saasAdmin.ServiceAccountKeyRotate)),
			compatserver.WithSaaSAdminServiceAccountKeyRevokeHandler(http.HandlerFunc(saasAdmin.ServiceAccountKeyRevoke)),
			compatserver.WithSaaSAdminAuditIntegrityHandler(http.HandlerFunc(saasAdmin.AuditIntegrity)),
			compatserver.WithSaaSAdminAuditIntegrityVerifyHandler(http.HandlerFunc(saasAdmin.AuditIntegrityVerify)),
			compatserver.WithSaaSAdminAuditAnchorsHandler(http.HandlerFunc(saasAdmin.AuditAnchors)),
			compatserver.WithSaaSAdminAuditAnchorHandler(http.HandlerFunc(saasAdmin.AuditAnchor)),
			compatserver.WithSaaSAdminBackupOverviewHandler(http.HandlerFunc(saasAdmin.BackupOverview)),
			compatserver.WithSaaSAdminBackupPolicyHandler(http.HandlerFunc(saasAdmin.BackupPolicy)),
			compatserver.WithSaaSAdminBackupRunHandler(http.HandlerFunc(saasAdmin.BackupRun)),
			compatserver.WithSaaSAdminRestoreDrillHandler(http.HandlerFunc(saasAdmin.RestoreDrill)),
			compatserver.WithSaaSAdminComplianceOverviewHandler(http.HandlerFunc(saasAdmin.ComplianceOverview)),
			compatserver.WithSaaSAdminCompliancePolicyHandler(http.HandlerFunc(saasAdmin.CompliancePolicy)),
			compatserver.WithSaaSAdminComplianceLegalHoldHandler(http.HandlerFunc(saasAdmin.ComplianceLegalHold)),
			compatserver.WithSaaSAdminComplianceExportHandler(http.HandlerFunc(saasAdmin.ComplianceExport)),
			compatserver.WithSaaSAdminComplianceExportDownloadHandler(http.HandlerFunc(saasAdmin.ComplianceExportDownload)),
			compatserver.WithSaaSAdminComplianceErasureHandler(http.HandlerFunc(saasAdmin.ComplianceErasure)),
			compatserver.WithSaaSAdminComplianceErasureStepsHandler(http.HandlerFunc(saasAdmin.ComplianceErasureSteps)),
			compatserver.WithSaaSServiceAccountWhoAmIHandler(http.HandlerFunc(saasAdmin.SaaSServiceAccountWhoAmI)),
			compatserver.WithSaaSServiceAccountUsageHandler(http.HandlerFunc(saasAdmin.SaaSServiceAccountUsage)),
			compatserver.WithSaaSServiceAccountAlertsHandler(http.HandlerFunc(saasAdmin.SaaSServiceAccountAlerts)),
			compatserver.WithSaaSAdminApprovalPoliciesHandler(http.HandlerFunc(saasAdmin.ApprovalPolicies)),
			compatserver.WithSaaSAdminApprovalPolicyHandler(http.HandlerFunc(saasAdmin.ApprovalPolicy)),
			compatserver.WithSaaSAdminApprovalsHandler(http.HandlerFunc(saasAdmin.Approvals)),
			compatserver.WithSaaSAdminApprovalEventsHandler(http.HandlerFunc(saasAdmin.ApprovalEvents)),
			compatserver.WithSaaSAdminApprovalDecisionsHandler(http.HandlerFunc(saasAdmin.ApprovalDecisions)),
			compatserver.WithSaaSAdminApprovalDelegationsHandler(http.HandlerFunc(saasAdmin.ApprovalDelegations)),
			compatserver.WithSaaSAdminApprovalDelegationHandler(http.HandlerFunc(saasAdmin.ApprovalDelegation)),
			compatserver.WithSaaSAdminApprovalRemindersHandler(http.HandlerFunc(saasAdmin.ApprovalReminders)),
			compatserver.WithSaaSAdminApprovalRequestHandler(http.HandlerFunc(saasAdmin.ApprovalRequest)),
			compatserver.WithSaaSAdminApprovalDecisionHandler(http.HandlerFunc(saasAdmin.ApprovalDecision)),
			compatserver.WithSaaSAdminApprovalCancelHandler(http.HandlerFunc(saasAdmin.ApprovalCancel)),
			compatserver.WithSaaSAdminApprovalExecuteHandler(http.HandlerFunc(saasAdmin.ApprovalExecute)),
			compatserver.WithSaaSAdminInvoiceProfileHandler(http.HandlerFunc(saasAdmin.InvoiceProfile)),
			compatserver.WithSaaSAdminInvoiceDocumentsHandler(http.HandlerFunc(saasAdmin.InvoiceDocuments)),
			compatserver.WithSaaSAdminInvoiceHandler(http.HandlerFunc(saasAdmin.CreateInvoiceDocument)),
			compatserver.WithSaaSAdminCreditNoteHandler(http.HandlerFunc(saasAdmin.CreateCreditNote)),
			compatserver.WithSaaSAdminInvoiceTransitionHandler(http.HandlerFunc(saasAdmin.TransitionInvoiceDocument)),
			compatserver.WithSaaSAdminOperationsHandler(http.HandlerFunc(saasAdmin.OperationLogs)),
			compatserver.WithSaaSAdminBillingEventsHandler(http.HandlerFunc(saasAdmin.BillingEvents)),
			compatserver.WithSaaSAdminBillingReconciliationHandler(http.HandlerFunc(saasAdmin.BillingReconciliation)),
			compatserver.WithSaaSAdminBillingReconciliationFollowUpHandler(http.HandlerFunc(saasAdmin.BillingReconciliationFollowUp)),
			compatserver.WithSaaSAdminBillingReconciliationFollowUpsHandler(http.HandlerFunc(saasAdmin.BillingReconciliationFollowUps)),
			compatserver.WithSaaSAdminBillingReconciliationFollowUpOwnersHandler(http.HandlerFunc(saasAdmin.BillingReconciliationFollowUpOwners)),
			compatserver.WithSaaSAdminBillingReconciliationFollowUpBulkCloseHandler(http.HandlerFunc(saasAdmin.BillingReconciliationFollowUpBulkClose)),
			compatserver.WithSaaSAdminTasksHandler(http.HandlerFunc(saasAdmin.Tasks)),
			compatserver.WithSaaSAdminTaskOwnersHandler(http.HandlerFunc(saasAdmin.TaskOwners)),
			compatserver.WithSaaSAdminTaskSLAHandler(http.HandlerFunc(saasAdmin.TaskSLA)),
			compatserver.WithSaaSAdminTaskSLANotificationsHandler(http.HandlerFunc(saasAdmin.TaskSLANotifications)),
			compatserver.WithSaaSAdminTaskCancelHandler(http.HandlerFunc(saasAdmin.TaskCancel)),
			compatserver.WithSaaSAdminTaskBulkCancelHandler(http.HandlerFunc(saasAdmin.TaskBulkCancel)),
			compatserver.WithSaaSAdminTaskBulkResetHandler(http.HandlerFunc(saasAdmin.TaskBulkReset)),
			compatserver.WithSaaSAdminTaskResetHandler(http.HandlerFunc(saasAdmin.TaskReset)),
			compatserver.WithSaaSAdminDailyReportHandler(http.HandlerFunc(saasAdmin.DailyReport)),
			compatserver.WithSaaSAdminExportHandler(http.HandlerFunc(saasAdmin.ExportCSV)),
			compatserver.WithSaaSAdminPackageHandler(http.HandlerFunc(saasAdmin.UpsertPackage)),
			compatserver.WithSaaSAdminPackageSyncHandler(http.HandlerFunc(saasAdmin.PackageSync)),
			compatserver.WithSaaSAdminPackageSyncTaskHandler(http.HandlerFunc(saasAdmin.PackageSyncTask)),
			compatserver.WithSaaSAdminPackageSyncTaskApplyHandler(http.HandlerFunc(saasAdmin.PackageSyncTaskApply)),
			compatserver.WithSaaSAdminPackageSyncTaskBulkApplyHandler(http.HandlerFunc(saasAdmin.PackageSyncTaskBulkApply)),
			compatserver.WithSaaSAdminTenantStatusHandler(http.HandlerFunc(saasAdmin.UpdateTenantStatus)),
			compatserver.WithSaaSAdminTenantRenewalHandler(http.HandlerFunc(saasAdmin.RenewTenant)),
			compatserver.WithSaaSAdminTenantRenewalTaskHandler(http.HandlerFunc(saasAdmin.TenantRenewalTask)),
			compatserver.WithSaaSAdminTenantRenewalTaskApplyHandler(http.HandlerFunc(saasAdmin.TenantRenewalTaskApply)),
			compatserver.WithSaaSAdminTenantRenewalTaskBulkApplyHandler(http.HandlerFunc(saasAdmin.TenantRenewalTaskBulkApply)),
			compatserver.WithSaaSAdminTenantProvisionHandler(http.HandlerFunc(saasAdmin.ProvisionTenant)),
			compatserver.WithSaaSAdminTenantProvisionTaskHandler(http.HandlerFunc(saasAdmin.TenantProvisionTask)),
			compatserver.WithSaaSAdminTenantProvisionTaskApplyHandler(http.HandlerFunc(saasAdmin.TenantProvisionTaskApply)),
			compatserver.WithSaaSAdminTenantProvisionTaskBulkApplyHandler(http.HandlerFunc(saasAdmin.TenantProvisionTaskBulkApply)),
			compatserver.WithSaaSAdminTenantPackageHandler(http.HandlerFunc(saasAdmin.UpdateTenantPackage)),
		)
		if identityManager != nil {
			options = append(options,
				compatserver.WithSaaSAdminIdentityOverviewHandler(http.HandlerFunc(saasAdmin.IdentityOverview)),
				compatserver.WithSaaSAdminIdentityPolicyHandler(http.HandlerFunc(saasAdmin.IdentityPolicy)),
				compatserver.WithSaaSAdminIdentitySessionsHandler(http.HandlerFunc(saasAdmin.IdentitySessions)),
				compatserver.WithSaaSAdminIdentitySessionHandler(http.HandlerFunc(saasAdmin.IdentitySession)),
				compatserver.WithSaaSAdminIdentityLoginEventsHandler(http.HandlerFunc(saasAdmin.IdentityLoginEvents)),
				compatserver.WithSaaSAdminIdentityIncidentsHandler(http.HandlerFunc(saasAdmin.IdentityIncidents)),
				compatserver.WithSaaSAdminIdentityIncidentHandler(http.HandlerFunc(saasAdmin.IdentityIncident)),
				compatserver.WithSaaSAdminIdentityUserHandler(http.HandlerFunc(saasAdmin.IdentityUser)),
				compatserver.WithSaaSAdminIdentityMFAHandler(http.HandlerFunc(saasAdmin.IdentityMFA)),
			)
			routeDebugf("go SaaS admin identity routes enabled: overview policy sessions login-events incidents user-unlock mfa-reset")
		}
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/page")
		routeDebugf("go SaaS admin branding routes enabled: GET brandingProfiles GET/POST/PUT brandingProfile")
		routeDebugf("go SaaS admin tenant domain routes enabled: GET tenantDomains POST/PUT tenantDomain dns_server_configured=%t", cfg.SaaSTenantDomainDNSServer != "")
		routeDebugf("go SaaS admin release readiness routes enabled: GET releaseReadiness POST/PUT releaseEvidence POST/PUT releaseCandidate source_fingerprint_configured=%t source=%s fingerprint=%s artifact_verify_on_pass=true artifact_reverify_on_candidate=true", cfg.SaaSReleaseSourceFingerprint != "", cfg.SaaSReleaseSourceFingerprintSource, cfg.SaaSReleaseSourceFingerprint)
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/overview platform_admin_tenant_id=%d", cfg.SaaSPlatformAdminTenantID)
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/tenantReadiness")
		routeDebugf("go SaaS admin notification credential route enabled: POST/PUT /dashboard/saasAdmin/notificationCredentialRotation")
		routeDebugf("go SaaS admin WeCom credential routes enabled: GET /dashboard/saasAdmin/wecomCredentialProtection POST/PUT /dashboard/saasAdmin/wecomCredentialRotation")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/tenant")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/tenantLifecycle")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/usage")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/risk")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/businessMetrics")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/businessTrends")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/operationQueue")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/operationQueueOwners")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/operationQueueAssignments")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/operationQueueAssignmentClose")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/operationQueueAssignmentNotifications")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/operationQueueAssign")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/renewalForecast")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/renewalForecastTasks")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/renewalForecastAssign")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/renewalForecastNotifications")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/customerSuccess")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/customerSuccessOwners")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/customerSuccessAssign")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/customerSuccessRenewalTasks")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/riskFollowUp")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/riskFollowUps")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/riskFollowUpOwners")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/riskFollowUpBulkClose")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/alerts")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/alertResolve")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/alertBulkResolve")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/notifications")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/notificationHealth")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/notificationSlo")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/notificationHealthRecovery")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/notificationPolicies")
		routeDebugf("go SaaS admin route enabled: GET/POST/PUT /dashboard/saasAdmin/notificationPolicy")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/notificationPolicyTest")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/notificationRetry")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/notificationBulkRetry")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/notificationClose")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/notificationBulkClose")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/packages")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/paymentOrders")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/paymentWebhookEvents")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/paymentOrder")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/paymentOrderCancel")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/paymentDunning")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/paymentRefunds")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/paymentRefund")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/paymentRefundCancel")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/paymentSettlementBatches")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/paymentSettlementEntries")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/paymentSettlementImport")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/paymentSettlementReconcile")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/paymentSettlementResolve")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/paymentSettlementTransition")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/paymentSettlementSyncRuns")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/paymentSettlementSync")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/accessProfile")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/accessRoles")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/accessRole")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/accessAssignments")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/accessAssignment")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/systemHealth")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/systemHealthScans")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/systemIncidents")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/systemHealthScan")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/systemIncident")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/serviceAccounts")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/serviceAccountUsage")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/serviceAccountUsageAlertEvaluate")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/serviceAccount")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/serviceAccountKeyRotate")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/serviceAccountKeyRevoke")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/backupOverview")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/backupPolicy")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/backupRun")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/restoreDrill")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/complianceOverview")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/compliancePolicy")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/complianceLegalHold")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/complianceExport")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/complianceExportDownload")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/complianceErasure")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/complianceErasureSteps")
		routeDebugf("go SaaS service account route enabled: GET /api/saas/v1/whoami")
		routeDebugf("go SaaS service account route enabled: GET /api/saas/v1/usage")
		routeDebugf("go SaaS service account route enabled: GET /api/saas/v1/alerts")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/approvalPolicies approval_required=%t", cfg.SaaSAdminApprovalRequired)
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/approvalPolicy")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/approvals")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/approvalEvents")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/approvalDecisions")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/approvalDelegations")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/approvalDelegation")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/approvalReminders")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/approvalRequest")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/approvalDecision")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/approvalCancel")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/approvalExecute")
		routeDebugf("go SaaS admin route enabled: GET/POST/PUT /dashboard/saasAdmin/invoiceProfile")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/invoiceDocuments")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/invoice")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/creditNote")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/invoiceTransition")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/operations")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/billingEvents")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/billingReconciliation")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/billingReconciliationFollowUp")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/billingReconciliationFollowUps")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/billingReconciliationFollowUpOwners")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/billingReconciliationFollowUpBulkClose")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/dailyReport")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/export")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/package")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/packageSync")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/tasks")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/taskOwners")
		routeDebugf("go SaaS admin route enabled: GET /dashboard/saasAdmin/taskSla")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/taskSlaNotifications")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/customerSuccessRenewalNotifications")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/taskCancel")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/taskBulkCancel")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/taskBulkReset")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/taskReset")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/packageSyncTask")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/packageSyncTaskApply")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/packageSyncTaskBulkApply")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/tenantStatus")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/tenantRenewal")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/tenantRenewalTask")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/tenantRenewalTaskApply")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/tenantRenewalTaskBulkApply")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/tenantProvision")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/tenantProvisionTask")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/tenantProvisionTaskApply")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/tenantProvisionTaskBulkApply")
		routeDebugf("go SaaS admin route enabled: POST/PUT /dashboard/saasAdmin/tenantPackage")
	}
	if cfg.EnableSaaSPaymentWebhook {
		paymentWebhook := dashboard.NewSaaSPaymentWebhookHandler(getMySQLStore(), cfg.SaaSPaymentWebhookSecret, cfg.SaaSPaymentWebhookTolerance)
		options = append(options, compatserver.WithSaaSPaymentWebhookHandler(paymentWebhook))
		debugf("go SaaS payment webhook enabled: POST /webhooks/saas/payment tolerance=%s signed=true", cfg.SaaSPaymentWebhookTolerance)
	}
	if cfg.SaaSTenantDomainDeliveryCallbackSecret != "" {
		domainDeliveryWebhook := dashboard.NewSaaSTenantDomainDeliveryWebhookHandler(
			getMySQLStore(), cfg.SaaSTenantDomainDeliveryCallbackSecret, cfg.SaaSTenantDomainDeliveryCallbackTolerance,
		)
		options = append(options, compatserver.WithSaaSTenantDomainDeliveryWebhookHandler(domainDeliveryWebhook))
		debugf("go SaaS tenant domain delivery webhook enabled: POST /webhooks/saas/domain-delivery tolerance=%s signed=true", cfg.SaaSTenantDomainDeliveryCallbackTolerance)
	}

	backgroundTasksEnabled := cfg.EnableWeWorkCallbackWorker || cfg.EnableEmployeeApplyWorker || cfg.EnableAsyncFileUploadWorker || cfg.EnableMarkTagsWorker || cfg.EnableMessageRemindWorker || cfg.EnableWorkRoomSyncWorker || cfg.EnableWorkContactSyncWorker || cfg.EnableWorkDepartmentListWorker || cfg.EnableMediaIDUpdateWorker || cfg.EnableEmployeeStatisticWorker || cfg.EnablePullAgentCron || cfg.EnableEmployeeStatisticCron || cfg.EnableChannelCodeCron || cfg.EnableContactBatchSendCron || cfg.EnableRoomBatchSendCron || cfg.EnableContactSyncSendResultCron || cfg.EnableRoomSyncSendResultCron || cfg.EnableRoomTagPullCron || cfg.EnableCorpDataCron || cfg.EnableMediaIDUpdateCron || cfg.EnableTransferStateRefreshCron || cfg.EnableSOPLogCron || cfg.EnableSensitiveWordMonitorCron || archivePlan.durableWorker || archivePlan.durableScheduler || archivePlan.legacyScheduler || cfg.EnableSaaSStorageReconcileCron || cfg.EnableSaaSAlertNotificationDispatchCron || cfg.EnableSaaSOperationQueueAssignmentReminderCron || cfg.EnableSaaSApprovalReminderCron || cfg.EnableSaaSSystemHealthCron || cfg.EnableSaaSBackupCron || cfg.EnableSaaSComplianceCron || cfg.EnableSaaSIdentityCleanupCron || cfg.EnableSaaSServiceAccountUsageAlertCron || cfg.EnableSaaSAuditIntegrityCron || cfg.EnableSaaSAuditAnchorCron || cfg.EnableSaaSServiceAccountUsageCleanupCron || cfg.EnableSaaSTenantDomainDeliveryCron || cfg.EnableSaaSNotificationHealthRecoveryCron || cfg.EnableSaaSSubscriptionReconcileCron || cfg.EnableSaaSPaymentDunningCron || cfg.EnableSaaSPaymentSettlementSyncCron
	persistentBackgroundRecorderEnabled := cfg.EnableWeWorkCallbackWorker || cfg.EnableEmployeeApplyWorker || (cfg.EnableAsyncFileUploadWorker && strings.TrimSpace(cfg.MySQLDSN) != "") || cfg.EnableMarkTagsWorker || cfg.EnableMessageRemindWorker || cfg.EnableWorkRoomSyncWorker || cfg.EnableWorkContactSyncWorker || cfg.EnableWorkDepartmentListWorker || cfg.EnableMediaIDUpdateWorker || cfg.EnableEmployeeStatisticWorker || cfg.EnablePullAgentCron || cfg.EnableEmployeeStatisticCron || cfg.EnableChannelCodeCron || cfg.EnableContactBatchSendCron || cfg.EnableRoomBatchSendCron || cfg.EnableContactSyncSendResultCron || cfg.EnableRoomSyncSendResultCron || cfg.EnableRoomTagPullCron || cfg.EnableCorpDataCron || cfg.EnableMediaIDUpdateCron || cfg.EnableTransferStateRefreshCron || cfg.EnableSOPLogCron || cfg.EnableSensitiveWordMonitorCron || archivePlan.durableWorker || archivePlan.durableScheduler || archivePlan.legacyScheduler || cfg.EnableSaaSStorageReconcileCron || cfg.EnableSaaSAlertNotificationDispatchCron || cfg.EnableSaaSOperationQueueAssignmentReminderCron || cfg.EnableSaaSApprovalReminderCron || cfg.EnableSaaSSystemHealthCron || cfg.EnableSaaSBackupCron || cfg.EnableSaaSComplianceCron || cfg.EnableSaaSIdentityCleanupCron || cfg.EnableSaaSServiceAccountUsageAlertCron || cfg.EnableSaaSAuditIntegrityCron || cfg.EnableSaaSAuditAnchorCron || cfg.EnableSaaSServiceAccountUsageCleanupCron || cfg.EnableSaaSTenantDomainDeliveryCron || cfg.EnableSaaSNotificationHealthRecoveryCron || cfg.EnableSaaSSubscriptionReconcileCron || cfg.EnableSaaSPaymentDunningCron || cfg.EnableSaaSPaymentSettlementSyncCron
	if cfg.EnableConversationExportWorker {
		backgroundTasksEnabled = true
		persistentBackgroundRecorderEnabled = true
	}
	workerGroup := taskrunner.New(structuredLogger())
	if cfg.EnableConversationExportWorker {
		owner := fmt.Sprintf("conversation-export-%d", os.Getpid())
		workerGroup.Add("conversation-export-worker", taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name: "conversation-export-worker", Interval: cfg.ConversationExportWorkerInterval, RunOnStart: true, Logger: structuredLogger(),
		}, func(ctx context.Context) error {
			_, err := getMySQLStore().RunWorkMessageExportWorker(ctx, cfg.ConversationExportRoot, owner)
			return err
		}))
		debugf("go worker enabled: conversation export interval=%s root=%s", cfg.ConversationExportWorkerInterval, cfg.ConversationExportRoot)
	}
	if cfg.EnablePullAgentCron {
		cron := dashboard.NewWorkAgentSyncCron(getMySQLStore(), dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL), log.Default())
		workerGroup.Add("cron-pull-agent", taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name:       "cron-pull-agent",
			Interval:   cfg.PullAgentCronInterval,
			RunOnStart: cfg.PullAgentCronRunOnStart,
			Logger:     structuredLogger(),
		}, cron.RunOnce))
		debugf("go cron enabled: pullAgent 企业微信应用同步 interval=%s run_on_start=%v", cfg.PullAgentCronInterval, cfg.PullAgentCronRunOnStart)
	}
	if cfg.EnableEmployeeStatisticCron {
		cron := dashboard.NewEmployeeStatisticCron(getMySQLStore(), getRedisStore(), dashboard.NewStatisticWeComClient(cfg.WeComAPIBaseURL), log.Default())
		workerGroup.Add("cron-employee-statistic", taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name:       "cron-employee-statistic",
			Interval:   cfg.EmployeeStatisticCronInterval,
			RunOnStart: cfg.EmployeeStatisticCronRunOnStart,
			Logger:     structuredLogger(),
		}, cron.RunOnce))
		debugf("go cron enabled: employeeStatistic 成员统计拉取 interval=%s run_on_start=%v", cfg.EmployeeStatisticCronInterval, cfg.EmployeeStatisticCronRunOnStart)
	}
	if cfg.EnableChannelCodeCron {
		cron := dashboard.NewChannelCodeCron(getMySQLStore(), dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL), log.Default())
		workerGroup.Add("cron-channel-code", taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name:       "cron-channel-code",
			Interval:   cfg.ChannelCodeCronInterval,
			RunOnStart: cfg.ChannelCodeCronRunOnStart,
			Logger:     structuredLogger(),
		}, cron.RunOnce))
		debugf("go cron enabled: channelCode 渠道码联系我方式更新 interval=%s run_on_start=%v", cfg.ChannelCodeCronInterval, cfg.ChannelCodeCronRunOnStart)
	}
	if cfg.EnableContactBatchSendCron {
		contactClient := dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL)
		legacyContactCron := dashboard.NewContactBatchSendScheduleCron(getMySQLStore(), contactClient, cfg.FileStorageRoot, log.Default())
		workerGroup.Add("cron-contact-batch-send-legacy", taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name:       "cron-contact-batch-send-legacy",
			Interval:   cfg.ContactBatchSendCronInterval,
			RunOnStart: cfg.ContactBatchSendCronRunOnStart,
			Logger:     structuredLogger(),
		}, legacyContactCron.RunOnce))
		contactDispatchRunner := dashboard.NewContactBatchDispatchRunner(getMySQLStore(), getMySQLStore(), getMySQLStore(), contactClient)
		contactDispatchCron := dashboard.NewContactBatchDispatchCron(getMySQLStore(), contactDispatchRunner, cfg.WorkerProcessingTimeout, 50, log.Default())
		workerGroup.Add("cron-contact-batch-dispatch", taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name:       "cron-contact-batch-dispatch",
			Interval:   cfg.ContactBatchSendCronInterval,
			RunOnStart: cfg.ContactBatchSendCronRunOnStart,
			Logger:     structuredLogger(),
		}, contactDispatchCron.RunOnce))
		debugf("go cron enabled: durable contact batch dispatch interval=%s run_on_start=%v kind=%s", cfg.ContactBatchSendCronInterval, cfg.ContactBatchSendCronRunOnStart, wecomcapability.DispatchKindContactBatch)
	}
	if cfg.EnableRoomBatchSendCron {
		roomClient := dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL)
		legacyRoomCron := dashboard.NewRoomBatchSendScheduleCron(getMySQLStore(), roomClient, cfg.FileStorageRoot, log.Default())
		workerGroup.Add("cron-room-batch-send-legacy", taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name:       "cron-room-batch-send-legacy",
			Interval:   cfg.RoomBatchSendCronInterval,
			RunOnStart: cfg.RoomBatchSendCronRunOnStart,
			Logger:     structuredLogger(),
		}, legacyRoomCron.RunOnce))
		roomDispatchRunner := dashboard.NewRoomBatchDispatchRunner(getMySQLStore(), getMySQLStore(), getMySQLStore(), roomClient)
		roomDispatchCron := dashboard.NewRoomBatchDispatchCron(getMySQLStore(), roomDispatchRunner, cfg.WorkerProcessingTimeout, 50, log.Default())
		workerGroup.Add("cron-room-batch-dispatch", taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name:       "cron-room-batch-dispatch",
			Interval:   cfg.RoomBatchSendCronInterval,
			RunOnStart: cfg.RoomBatchSendCronRunOnStart,
			Logger:     structuredLogger(),
		}, roomDispatchCron.RunOnce))
		debugf("go cron enabled: durable room batch dispatch interval=%s run_on_start=%v kind=%s", cfg.RoomBatchSendCronInterval, cfg.RoomBatchSendCronRunOnStart, wecomcapability.DispatchKindRoomBatch)
	}
	if cfg.EnableContactSyncSendResultCron {
		cron := dashboard.NewContactBatchSendResultCron(getMySQLStore(), dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL), log.Default())
		workerGroup.Add("cron-contact-sync-send-result", taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name:       "cron-contact-sync-send-result",
			Interval:   cfg.ContactSyncSendResultCronInterval,
			RunOnStart: cfg.ContactSyncSendResultCronRunOnStart,
			Logger:     structuredLogger(),
		}, cron.RunOnce))
		debugf("go cron enabled: ContactSyncSendResultTask 客户群发结果同步 interval=%s run_on_start=%v", cfg.ContactSyncSendResultCronInterval, cfg.ContactSyncSendResultCronRunOnStart)
	}
	if cfg.EnableRoomSyncSendResultCron {
		cron := dashboard.NewRoomBatchSendResultCron(getMySQLStore(), dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL), log.Default())
		workerGroup.Add("cron-room-sync-send-result", taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name:       "cron-room-sync-send-result",
			Interval:   cfg.RoomSyncSendResultCronInterval,
			RunOnStart: cfg.RoomSyncSendResultCronRunOnStart,
			Logger:     structuredLogger(),
		}, cron.RunOnce))
		debugf("go cron enabled: RoomSyncSendResultTask 客户群群发结果同步 interval=%s run_on_start=%v", cfg.RoomSyncSendResultCronInterval, cfg.RoomSyncSendResultCronRunOnStart)
	}
	if cfg.EnableRoomTagPullCron {
		cron := dashboard.NewRoomTagPullCron(getMySQLStore(), dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL), log.Default())
		workerGroup.Add("cron-room-tag-pull", taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name:       "cron-room-tag-pull",
			Interval:   cfg.RoomTagPullCronInterval,
			RunOnStart: cfg.RoomTagPullCronRunOnStart,
			Logger:     structuredLogger(),
		}, cron.RunOnce))
		debugf("go cron enabled: RoomTagPull 标签建群结果同步 interval=%s run_on_start=%v", cfg.RoomTagPullCronInterval, cfg.RoomTagPullCronRunOnStart)
	}
	var durableBridgeClient *archiveprovider.BridgeArchiveClient
	var durableRunner *archiveprovider.DurableBridgeRunner
	if archivePlan.durableWorker || archivePlan.durableScheduler {
		bridgeClient, err := archiveprovider.NewBridgeArchiveClient(
			cfg.WorkMessageArchiveBridgeBaseURL,
			cfg.WorkMessageArchiveBridgeToken,
			nil,
		)
		if err != nil {
			fatalf("build durable work message archive bridge: %v", err)
		}
		durableBridgeClient = bridgeClient
		durableRunner = archiveprovider.NewDurableBridgeRunner(getMySQLStore(), bridgeClient, cfg.WorkMessageArchiveSyncLimit)
	}
	if archivePlan.durableWorker {
		mediaRunner := archiveprovider.NewMediaSyncService(getMySQLStore(), durableBridgeClient, cfg.FileStorageRoot)
		workerGroup.Add("worker-durable-work-message-archive-sync", taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name:       "worker-durable-work-message-archive-sync",
			Interval:   cfg.WorkMessageArchiveSyncCronInterval,
			RunOnStart: true,
			Logger:     structuredLogger(),
		}, durableRunner.RunPendingOnce))
		workerGroup.Add("worker-durable-work-message-archive-media", taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name:       "worker-durable-work-message-archive-media",
			Interval:   cfg.WorkMessageArchiveSyncCronInterval,
			RunOnStart: true,
			Logger:     structuredLogger(),
		}, func(ctx context.Context) error {
			return runDurableArchiveMediaBatch(ctx, mediaRunner, cfg.WorkMessageArchiveSyncLimit, log.Default())
		}))
		debugf("durable archive workers enabled: automatic_schedule=%t interval=%s schedule_run_on_start=%v limit=%d storage_root=%s",
			archivePlan.durableScheduler, cfg.WorkMessageArchiveSyncCronInterval, cfg.WorkMessageArchiveSyncCronRunOnStart, cfg.WorkMessageArchiveSyncLimit, cfg.FileStorageRoot)
	}
	if archivePlan.durableScheduler {
		workerGroup.Add("cron-durable-work-message-archive-enqueue", taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name:       "cron-durable-work-message-archive-enqueue",
			Interval:   cfg.WorkMessageArchiveSyncCronInterval,
			RunOnStart: cfg.WorkMessageArchiveSyncCronRunOnStart,
			Logger:     structuredLogger(),
		}, durableRunner.EnqueueScheduledOnce))
	}
	var workMessageArchiveCron *dashboard.WorkMessageArchiveSyncCron
	if archivePlan.legacyScheduler {
		workMessageArchiveCron = dashboard.NewWorkMessageArchiveSyncCron(
			getMySQLStore(),
			dashboard.NewWorkMessageArchiveBridgeClient(cfg.WorkMessageArchiveBridgeBaseURL, cfg.WorkMessageArchiveBridgeToken),
			structuredLogger(),
		).WithLimit(cfg.WorkMessageArchiveSyncLimit)
	}
	if cfg.EnableWeWorkCallbackWorker {
		worker := dashboard.NewWeWorkCallbackWorker(
			getOptionalWeWorkCallbackRedisStore(),
			getMySQLStore(),
			dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL),
			"",
			log.Default(),
		).WithProcessingTimeout(cfg.WorkerProcessingTimeout).
			WithWorkFissionBaseURLs(cfg.APIBaseURL, cfg.OperationBaseURL).
			WithSidebarBaseURL(cfg.SidebarBaseURL).
			WithFileStorageRoot(cfg.FileStorageRoot).
			WithSaaSAlertNotifier(saasAlertNotifier)
		if workMessageArchiveCron != nil {
			worker.WithArchiveSyncTrigger(workMessageArchiveCron)
		}
		workerGroup.Add("wework-callback", worker.Run)
		debugf("go worker enabled: durable MySQL WeWork callback inbox consumer (Redis only used by optional downstream queues)")

		contactWelcomeWorker := dashboard.NewContactWelcomeWorker(
			getRedisStore(),
			getMySQLStore(),
			dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL),
			cfg.FileStorageRoot,
			cfg.APIBaseURL,
			log.Default(),
		).WithProcessingTimeout(cfg.WorkerProcessingTimeout).
			WithSaaSAlertNotifier(saasAlertNotifier)
		workerGroup.Add("contact-welcome", contactWelcomeWorker.Run)
		debugf("go worker enabled: ContactWelcome welcome message Redis consumer")
	}

	if cfg.EnableEmployeeApplyWorker {
		worker := dashboard.NewEmployeeApplyWorker(
			getRedisStore(),
			getMySQLStore(),
			dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL),
			log.Default(),
		).WithProcessingTimeout(cfg.WorkerProcessingTimeout).
			WithSaaSAlertNotifier(saasAlertNotifier)
		workerGroup.Add("employee-apply", worker.Run)
		debugf("go worker enabled: EmployeeApply Redis consumer")
	}
	if cfg.EnableAsyncFileUploadWorker {
		worker := dashboard.NewAsyncFileUploadWorker(
			getRedisStore(),
			cfg.FileStorageRoot,
			log.Default(),
		).WithProcessingTimeout(cfg.WorkerProcessingTimeout)
		if strings.TrimSpace(cfg.MySQLDSN) != "" {
			worker.WithSaaSExecutionStore(getMySQLStore()).
				WithSaaSAlertNotifier(saasAlertNotifier)
		}
		workerGroup.Add(dashboard.QueueNameAsyncFileUpload, worker.Run)
		debugf("go worker enabled: AsyncFileUpload Redis consumer storage_root=%s", cfg.FileStorageRoot)
	}
	if cfg.EnableMarkTagsWorker {
		worker := dashboard.NewMarkTagsWorker(
			getRedisStore(),
			getMySQLStore(),
			dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL),
			log.Default(),
		).WithProcessingTimeout(cfg.WorkerProcessingTimeout).
			WithSaaSAlertNotifier(saasAlertNotifier)
		workerGroup.Add(dashboard.QueueNameMarkTags, worker.Run)
		debugf("go worker enabled: MarkTags Redis consumer")
	}
	if cfg.EnableMessageRemindWorker {
		worker := dashboard.NewMessageRemindWorker(
			getRedisStore(),
			getMySQLStore(),
			dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL),
			cfg.FileStorageRoot,
			log.Default(),
		).WithProcessingTimeout(cfg.WorkerProcessingTimeout).
			WithSaaSAlertNotifier(saasAlertNotifier)
		workerGroup.Add(dashboard.QueueNameMessageRemind, worker.Run)
		debugf("go worker enabled: MessageRemind Redis consumer storage_root=%s", cfg.FileStorageRoot)
	}
	if cfg.EnableWorkRoomSyncWorker {
		worker := dashboard.NewWorkRoomSyncWorker(
			getRedisStore(),
			getMySQLStore(),
			dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL),
			log.Default(),
		).WithProcessingTimeout(cfg.WorkerProcessingTimeout).
			WithSaaSAlertNotifier(saasAlertNotifier)
		workerGroup.Add(dashboard.QueueNameWorkRoomSync, worker.Run)
		debugf("go worker enabled: WorkRoomSync Redis consumer")
	}
	if cfg.EnableWorkContactSyncWorker {
		worker := dashboard.NewWorkContactSyncWorker(
			getRedisStore(),
			getMySQLStore(),
			dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL),
			log.Default(),
		).WithProcessingTimeout(cfg.WorkerProcessingTimeout).
			WithSaaSAlertNotifier(saasAlertNotifier)
		workerGroup.Add(dashboard.QueueNameWorkContactSync, worker.Run)
		debugf("go worker enabled: WorkContactSync Redis consumer")
	}
	if cfg.EnableWorkDepartmentListWorker {
		worker := dashboard.NewWorkDepartmentListWorker(
			getRedisStore(),
			getMySQLStore(),
			dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL),
			"",
			log.Default(),
		).WithProcessingTimeout(cfg.WorkerProcessingTimeout).
			WithSaaSAlertNotifier(saasAlertNotifier)
		workerGroup.Add(dashboard.QueueNameWorkDepartmentList, worker.Run)
		debugf("go worker enabled: WorkDepartmentList Redis consumer")
	}
	if cfg.EnableMediaIDUpdateWorker {
		worker := dashboard.NewMediumMediaIDUpdateWorker(
			getRedisStore(),
			getMySQLStore(),
			dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL),
			cfg.FileStorageRoot,
			log.Default(),
		).WithProcessingTimeout(cfg.WorkerProcessingTimeout).
			WithSaaSAlertNotifier(saasAlertNotifier)
		workerGroup.Add(dashboard.QueueNameMediumMediaIDUpdate, worker.Run)
		debugf("go worker enabled: MediaIDUpdate Redis consumer storage_root=%s", cfg.FileStorageRoot)
	}
	if cfg.EnableEmployeeStatisticWorker {
		worker := dashboard.NewEmployeeStatisticApplyWorker(
			getRedisStore(),
			getMySQLStore(),
			getRedisStore(),
			dashboard.NewStatisticWeComClient(cfg.WeComAPIBaseURL),
			log.Default(),
		).WithProcessingTimeout(cfg.WorkerProcessingTimeout).
			WithSaaSAlertNotifier(saasAlertNotifier)
		workerGroup.Add(dashboard.QueueNameEmployeeStatisticApply, worker.Run)
		debugf("go worker enabled: EmployeeStatisticApply Redis consumer")
	}
	if cfg.EnableCorpDataCron {
		cron := dashboard.NewCorpDataCron(getMySQLStore(), log.Default())
		workerGroup.Add("cron-corp-data", taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name:       "cron-corp-data",
			Interval:   cfg.CorpDataCronInterval,
			RunOnStart: cfg.CorpDataCronRunOnStart,
			Logger:     structuredLogger(),
		}, cron.RunOnce))
		debugf("go cron enabled: corpData 首页数据统计 interval=%s run_on_start=%v", cfg.CorpDataCronInterval, cfg.CorpDataCronRunOnStart)
	}
	if cfg.EnableMediaIDUpdateCron {
		cron := dashboard.NewMediumMediaCron(getMySQLStore(), dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL), cfg.FileStorageRoot, log.Default())
		workerGroup.Add("cron-media-id-update", taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name:       "cron-media-id-update",
			Interval:   cfg.MediaIDUpdateCronInterval,
			RunOnStart: cfg.MediaIDUpdateCronRunOnStart,
			Logger:     structuredLogger(),
		}, cron.RunOnce))
		debugf("go cron enabled: mediaIdUpdate 素材库media_id更新 interval=%s run_on_start=%v", cfg.MediaIDUpdateCronInterval, cfg.MediaIDUpdateCronRunOnStart)
	}
	if cfg.EnableTransferStateRefreshCron {
		cron := dashboard.NewContactTransferStateCron(getMySQLStore(), getRedisStore(), dashboard.NewRoomWelcomeWeComClient(cfg.WeComAPIBaseURL), log.Default())
		workerGroup.Add("cron-transfer-state-refresh", taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name:       "cron-transfer-state-refresh",
			Interval:   cfg.TransferStateRefreshCronInterval,
			RunOnStart: cfg.TransferStateRefreshCronRunOnStart,
			Logger:     structuredLogger(),
		}, cron.RunOnce))
		debugf("go cron enabled: TransferStateRefresh 分配状态更新 interval=%s run_on_start=%v", cfg.TransferStateRefreshCronInterval, cfg.TransferStateRefreshCronRunOnStart)
	}
	if cfg.EnableSOPLogCron {
		cron := dashboard.NewSOPLogCron(getMySQLStore(), log.Default())
		workerGroup.Add("cron-sop-log", taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name:       "cron-sop-log",
			Interval:   cfg.SOPLogCronInterval,
			RunOnStart: cfg.SOPLogCronRunOnStart,
			Logger:     structuredLogger(),
		}, cron.RunOnce))
		debugf("go cron enabled: SOP log 个人/群 SOP 提醒生成 interval=%s run_on_start=%v", cfg.SOPLogCronInterval, cfg.SOPLogCronRunOnStart)
	}
	if workMessageArchiveCron != nil {
		workerGroup.Add("cron-work-message-archive-sync", taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name:                "cron-work-message-archive-sync",
			Interval:            cfg.WorkMessageArchiveSyncCronInterval,
			RunOnStart:          cfg.WorkMessageArchiveSyncCronRunOnStart,
			Logger:              structuredLogger(),
			SuppressOutcomeLogs: true,
		}, workMessageArchiveCron.RunOnce))
		debugf("go cron enabled: workMessageArchive 会话存档同步 interval=%s run_on_start=%v limit=%d bridge_configured=%t", cfg.WorkMessageArchiveSyncCronInterval, cfg.WorkMessageArchiveSyncCronRunOnStart, cfg.WorkMessageArchiveSyncLimit, strings.TrimSpace(cfg.WorkMessageArchiveBridgeBaseURL) != "")
	}
	if cfg.EnableSensitiveWordMonitorCron {
		cron := dashboard.NewSensitiveWordMonitorCron(getMySQLStore(), log.Default())
		workerGroup.Add("cron-sensitive-word-monitor", taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name:       "cron-sensitive-word-monitor",
			Interval:   cfg.SensitiveWordMonitorCronInterval,
			RunOnStart: cfg.SensitiveWordMonitorCronRunOnStart,
			Logger:     structuredLogger(),
		}, cron.RunOnce))
		debugf("go cron enabled: sensitiveWordsMonitor 会话存档敏感词监控 interval=%s run_on_start=%v", cfg.SensitiveWordMonitorCronInterval, cfg.SensitiveWordMonitorCronRunOnStart)
	}
	if cfg.EnableSaaSStorageReconcileCron {
		runOnce := func(ctx context.Context) error {
			result, err := getMySQLStore().ReconcileSaaSStorageObjects(ctx, cfg.FileStorageRoot, 0)
			if err != nil {
				return err
			}
			debugf("go cron completed: SaaS storage reconcile scanned=%d missing_marked=%d unsafe_marked=%d size_updated=%d counters_refreshed=%d refreshed_tenants=%v", result.Scanned, result.MissingMarked, result.UnsafeMarked, result.SizeUpdated, result.CountersRefreshed, result.RefreshedTenants)
			return nil
		}
		workerGroup.Add("cron-saas-storage-reconcile", taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name:       "cron-saas-storage-reconcile",
			Interval:   cfg.SaaSStorageReconcileCronInterval,
			RunOnStart: cfg.SaaSStorageReconcileCronRunOnStart,
			Logger:     structuredLogger(),
		}, runOnce))
		debugf("go cron enabled: SaaS storage reconcile interval=%s run_on_start=%v storage_root=%s", cfg.SaaSStorageReconcileCronInterval, cfg.SaaSStorageReconcileCronRunOnStart, cfg.FileStorageRoot)
	}
	if cfg.EnableSaaSTenantDomainDeliveryCron {
		if tenantDomainDeliveryProcessor == nil {
			fatal("SaaS tenant domain delivery cron enabled without a configured bridge")
		}
		workerGroup.Add(dashboard.SaaSTenantDomainDeliveryCronTaskName, taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name:       dashboard.SaaSTenantDomainDeliveryCronTaskName,
			Interval:   cfg.SaaSTenantDomainDeliveryCronInterval,
			RunOnStart: cfg.SaaSTenantDomainDeliveryCronRunOnStart,
			Logger:     structuredLogger(),
		}, tenantDomainDeliveryProcessor.RunOnce))
		debugf("go cron enabled: SaaS tenant domain delivery interval=%s run_on_start=%v limit=%d", cfg.SaaSTenantDomainDeliveryCronInterval, cfg.SaaSTenantDomainDeliveryCronRunOnStart, cfg.SaaSTenantDomainDeliveryLimit)
	}
	if cfg.EnableSaaSAlertNotificationDispatchCron {
		cron := dashboard.NewSaaSAlertNotificationDispatchCron(
			getMySQLStore(),
			saasAlertWebhookNotifier,
			cfg.SaaSAlertNotificationDispatchLimit,
			cfg.SaaSAlertNotificationRetryDelay,
			log.Default(),
		)
		workerGroup.Add("cron-saas-alert-notification-dispatch", taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name:       "cron-saas-alert-notification-dispatch",
			Interval:   cfg.SaaSAlertNotificationDispatchCronInterval,
			RunOnStart: cfg.SaaSAlertNotificationDispatchCronRunOnStart,
			Logger:     structuredLogger(),
		}, cron.RunOnce))
		debugf("go cron enabled: SaaS alert notification dispatch interval=%s run_on_start=%v limit=%d", cfg.SaaSAlertNotificationDispatchCronInterval, cfg.SaaSAlertNotificationDispatchCronRunOnStart, cfg.SaaSAlertNotificationDispatchLimit)
	}
	if cfg.EnableSaaSOperationQueueAssignmentReminderCron {
		if saasAdminHandler == nil {
			saasAdminHandler = newSaaSAdminHandler()
		}
		cron := dashboard.NewSaaSAdminOperationQueueAssignmentReminderCron(
			saasAdminHandler,
			cfg.SaaSOperationQueueAssignmentReminderLimit,
			cfg.SaaSAlertNotificationMaxAttempts,
			log.Default(),
		)
		workerGroup.Add(dashboard.SaaSAdminOperationQueueAssignmentReminderCronTaskName, taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name:       dashboard.SaaSAdminOperationQueueAssignmentReminderCronTaskName,
			Interval:   cfg.SaaSOperationQueueAssignmentReminderCronInterval,
			RunOnStart: cfg.SaaSOperationQueueAssignmentReminderCronRunOnStart,
			Logger:     structuredLogger(),
		}, cron.RunOnce))
		debugf("go cron enabled: SaaS operation queue assignment reminder interval=%s run_on_start=%v limit=%d due_states=overdue,due_soon", cfg.SaaSOperationQueueAssignmentReminderCronInterval, cfg.SaaSOperationQueueAssignmentReminderCronRunOnStart, cfg.SaaSOperationQueueAssignmentReminderLimit)
	}
	if cfg.EnableSaaSApprovalReminderCron {
		if saasAdminHandler == nil {
			saasAdminHandler = newSaaSAdminHandler()
		}
		cron := dashboard.NewSaaSAdminApprovalReminderCron(saasAdminHandler, cfg.SaaSApprovalReminderLimit, cfg.SaaSAlertNotificationMaxAttempts, log.Default())
		workerGroup.Add(dashboard.SaaSAdminApprovalReminderCronTaskName, taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name: dashboard.SaaSAdminApprovalReminderCronTaskName, Interval: cfg.SaaSApprovalReminderCronInterval,
			RunOnStart: cfg.SaaSApprovalReminderCronRunOnStart, Logger: structuredLogger(),
		}, cron.RunOnce))
		debugf("go cron enabled: SaaS approval reminder interval=%s run_on_start=%v limit=%d", cfg.SaaSApprovalReminderCronInterval, cfg.SaaSApprovalReminderCronRunOnStart, cfg.SaaSApprovalReminderLimit)
	}
	if cfg.EnableSaaSBackupCron {
		if backupManager == nil {
			fatal("SaaS backup cron enabled without backup manager")
		}
		cron := saasbackup.NewCron(backupManager, log.Default())
		workerGroup.Add(saasbackup.CronTaskName, taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name: saasbackup.CronTaskName, Interval: cfg.SaaSBackupCronInterval,
			RunOnStart: cfg.SaaSBackupCronRunOnStart, Logger: structuredLogger(),
		}, cron.RunOnce))
		debugf("go cron enabled: SaaS encrypted database backup interval=%s run_on_start=%v key_id=%s",
			cfg.SaaSBackupCronInterval, cfg.SaaSBackupCronRunOnStart, cfg.SaaSBackupEncryptionKeyID)
	}
	if cfg.EnableSaaSComplianceCron {
		if complianceManager == nil {
			fatal("SaaS compliance cron enabled without compliance manager")
		}
		cron := saascompliance.NewCron(complianceManager, saascompliance.Actor{TenantID: cfg.SaaSPlatformAdminTenantID}, 10, log.Default())
		workerGroup.Add(saascompliance.CronTaskName, taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name: saascompliance.CronTaskName, Interval: cfg.SaaSComplianceCronInterval,
			RunOnStart: cfg.SaaSComplianceCronRunOnStart, Logger: structuredLogger(),
		}, cron.RunOnce))
		debugf("go cron enabled: SaaS tenant data compliance interval=%s run_on_start=%v key_id=%s",
			cfg.SaaSComplianceCronInterval, cfg.SaaSComplianceCronRunOnStart, cfg.SaaSComplianceEncryptionKeyID)
	}
	if cfg.EnableSaaSIdentityCleanupCron {
		if identityManager == nil {
			fatal("SaaS identity cleanup cron enabled without identity security manager")
		}
		cron := identitysecurity.NewCron(identityManager, cfg.SaaSIdentityCleanupLimit, log.Default())
		workerGroup.Add(identitysecurity.CronTaskName, taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name: identitysecurity.CronTaskName, Interval: cfg.SaaSIdentityCleanupCronInterval,
			RunOnStart: cfg.SaaSIdentityCleanupCronRunOnStart, Logger: structuredLogger(),
		}, cron.RunOnce))
		debugf("go cron enabled: SaaS identity cleanup interval=%s run_on_start=%v limit=%d",
			cfg.SaaSIdentityCleanupCronInterval, cfg.SaaSIdentityCleanupCronRunOnStart, cfg.SaaSIdentityCleanupLimit)
	}
	if cfg.EnableSaaSServiceAccountUsageAlertCron {
		cron := dashboard.NewSaaSServiceAccountUsageAlertCron(getMySQLStore(), cfg.SaaSServiceAccountUsageAlertLimit, log.Default())
		workerGroup.Add(dashboard.SaaSServiceAccountUsageAlertCronTaskName, taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name: dashboard.SaaSServiceAccountUsageAlertCronTaskName, Interval: cfg.SaaSServiceAccountUsageAlertCronInterval,
			RunOnStart: cfg.SaaSServiceAccountUsageAlertCronRunOnStart, Logger: structuredLogger(),
		}, cron.RunOnce))
		debugf("go cron enabled: SaaS service account usage alert interval=%s run_on_start=%v limit=%d",
			cfg.SaaSServiceAccountUsageAlertCronInterval, cfg.SaaSServiceAccountUsageAlertCronRunOnStart, cfg.SaaSServiceAccountUsageAlertLimit)
	}
	if cfg.EnableSaaSAuditIntegrityCron {
		cron := dashboard.NewSaaSAdminAuditIntegrityCron(getMySQLStore(), cfg.SaaSAuditIntegrityLimit, log.Default())
		workerGroup.Add(dashboard.SaaSAdminAuditIntegrityCronTaskName, taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name: dashboard.SaaSAdminAuditIntegrityCronTaskName, Interval: cfg.SaaSAuditIntegrityCronInterval,
			RunOnStart: cfg.SaaSAuditIntegrityCronRunOnStart, Logger: structuredLogger(),
		}, cron.RunOnce))
		debugf("go cron enabled: SaaS audit integrity interval=%s run_on_start=%v limit=%d",
			cfg.SaaSAuditIntegrityCronInterval, cfg.SaaSAuditIntegrityCronRunOnStart, cfg.SaaSAuditIntegrityLimit)
	}
	if cfg.EnableSaaSAuditAnchorCron {
		if auditAnchorManager == nil {
			fatal("SaaS audit anchor cron enabled without audit anchor manager")
		}
		cron := saasauditanchor.NewCron(auditAnchorManager, cfg.SaaSAuditAnchorLimit, log.Default())
		workerGroup.Add(saasauditanchor.CronTaskName, taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name: saasauditanchor.CronTaskName, Interval: cfg.SaaSAuditAnchorCronInterval,
			RunOnStart: cfg.SaaSAuditAnchorCronRunOnStart, Logger: structuredLogger(),
		}, cron.RunOnce))
		debugf("go cron enabled: SaaS audit anchor interval=%s run_on_start=%v limit=%d key_id=%s",
			cfg.SaaSAuditAnchorCronInterval, cfg.SaaSAuditAnchorCronRunOnStart, cfg.SaaSAuditAnchorLimit, cfg.SaaSAuditAnchorHMACKeyID)
	}
	if cfg.EnableSaaSServiceAccountUsageCleanupCron {
		cron := dashboard.NewSaaSServiceAccountUsageCleanupCron(getMySQLStore(), cfg.SaaSServiceAccountUsageCleanupLimit, log.Default())
		workerGroup.Add(dashboard.SaaSServiceAccountUsageCleanupCronTaskName, taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name: dashboard.SaaSServiceAccountUsageCleanupCronTaskName, Interval: cfg.SaaSServiceAccountUsageCleanupCronInterval,
			RunOnStart: cfg.SaaSServiceAccountUsageCleanupCronRunOnStart, Logger: structuredLogger(),
		}, cron.RunOnce))
		debugf("go cron enabled: SaaS service account usage cleanup interval=%s run_on_start=%v limit=%d",
			cfg.SaaSServiceAccountUsageCleanupCronInterval, cfg.SaaSServiceAccountUsageCleanupCronRunOnStart, cfg.SaaSServiceAccountUsageCleanupLimit)
	}
	if cfg.EnableSaaSSystemHealthCron {
		if saasAdminHandler == nil {
			saasAdminHandler = newSaaSAdminHandler()
		}
		cron := dashboard.NewSaaSAdminSystemHealthCron(
			saasAdminHandler, cfg.SaaSSystemHealthFailureWindowHours, cfg.SaaSSystemHealthNotificationStaleMinutes,
			cfg.SaaSAlertNotificationMaxAttempts, log.Default(),
		)
		workerGroup.Add(dashboard.SaaSAdminSystemHealthCronTaskName, taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name: dashboard.SaaSAdminSystemHealthCronTaskName, Interval: cfg.SaaSSystemHealthCronInterval,
			RunOnStart: cfg.SaaSSystemHealthCronRunOnStart, Logger: structuredLogger(),
		}, cron.RunOnce))
		debugf("go cron enabled: SaaS system health interval=%s run_on_start=%v failure_window_hours=%d notification_stale_minutes=%d",
			cfg.SaaSSystemHealthCronInterval, cfg.SaaSSystemHealthCronRunOnStart,
			cfg.SaaSSystemHealthFailureWindowHours, cfg.SaaSSystemHealthNotificationStaleMinutes)
	}
	if cfg.EnableSaaSNotificationHealthRecoveryCron {
		if saasAdminHandler == nil {
			saasAdminHandler = newSaaSAdminHandler()
		}
		cron := dashboard.NewSaaSAdminNotificationHealthRecoveryCron(
			saasAdminHandler,
			cfg.SaaSNotificationHealthRecoveryWindowHours,
			cfg.SaaSNotificationHealthRecoveryStaleMinutes,
			log.Default(),
		)
		workerGroup.Add(dashboard.SaaSAdminNotificationHealthRecoveryCronTaskName, taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name:       dashboard.SaaSAdminNotificationHealthRecoveryCronTaskName,
			Interval:   cfg.SaaSNotificationHealthRecoveryCronInterval,
			RunOnStart: cfg.SaaSNotificationHealthRecoveryCronRunOnStart,
			Logger:     structuredLogger(),
		}, cron.RunOnce))
		debugf("go cron enabled: SaaS notification health recovery interval=%s run_on_start=%v window_hours=%d stale_minutes=%d", cfg.SaaSNotificationHealthRecoveryCronInterval, cfg.SaaSNotificationHealthRecoveryCronRunOnStart, cfg.SaaSNotificationHealthRecoveryWindowHours, cfg.SaaSNotificationHealthRecoveryStaleMinutes)
	}
	if cfg.EnableSaaSSubscriptionReconcileCron {
		if saasAdminHandler == nil {
			saasAdminHandler = newSaaSAdminHandler()
		}
		cron := dashboard.NewSaaSAdminSubscriptionReconcileCron(saasAdminHandler, cfg.SaaSSubscriptionReconcileLimit, log.Default())
		workerGroup.Add(dashboard.SaaSAdminSubscriptionReconcileCronTaskName, taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name:       dashboard.SaaSAdminSubscriptionReconcileCronTaskName,
			Interval:   cfg.SaaSSubscriptionReconcileCronInterval,
			RunOnStart: cfg.SaaSSubscriptionReconcileCronRunOnStart,
			Logger:     structuredLogger(),
		}, cron.RunOnce))
		debugf("go cron enabled: SaaS subscription reconcile interval=%s run_on_start=%v limit=%d", cfg.SaaSSubscriptionReconcileCronInterval, cfg.SaaSSubscriptionReconcileCronRunOnStart, cfg.SaaSSubscriptionReconcileLimit)
	}
	if cfg.EnableSaaSPaymentDunningCron {
		cron := dashboard.NewSaaSPaymentDunningCron(
			getMySQLStore(), cfg.SaaSPaymentDunningLimit, int(cfg.SaaSPaymentDunningRetryDelay/time.Second),
			cfg.SaaSAlertNotificationMaxAttempts, cfg.SaaSPlatformAdminTenantID, log.Default(),
		)
		workerGroup.Add(dashboard.SaaSPaymentDunningCronTaskName, taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name: dashboard.SaaSPaymentDunningCronTaskName, Interval: cfg.SaaSPaymentDunningCronInterval,
			RunOnStart: cfg.SaaSPaymentDunningCronRunOnStart, Logger: structuredLogger(),
		}, cron.RunOnce))
		debugf("go cron enabled: SaaS payment dunning interval=%s run_on_start=%v limit=%d retry_delay=%s", cfg.SaaSPaymentDunningCronInterval, cfg.SaaSPaymentDunningCronRunOnStart, cfg.SaaSPaymentDunningLimit, cfg.SaaSPaymentDunningRetryDelay)
	}
	if cfg.EnableSaaSPaymentSettlementSyncCron {
		if paymentSettlementSyncService == nil {
			fatal("SaaS payment settlement sync cron enabled without a configured bridge service")
		}
		cron := dashboard.NewSaaSPaymentSettlementSyncCron(paymentSettlementSyncService)
		workerGroup.Add(dashboard.SaaSPaymentSettlementSyncCronTaskName, taskrunner.Periodic(taskrunner.PeriodicConfig{
			Name: dashboard.SaaSPaymentSettlementSyncCronTaskName, Interval: cfg.SaaSPaymentSettlementSyncCronInterval,
			RunOnStart: cfg.SaaSPaymentSettlementSyncCronRunOnStart, Logger: structuredLogger(),
		}, cron.RunOnce))
		debugf("go cron enabled: SaaS payment settlement sync interval=%s run_on_start=%v providers=%s limit=%d", cfg.SaaSPaymentSettlementSyncCronInterval, cfg.SaaSPaymentSettlementSyncCronRunOnStart, strings.Join(cfg.SaaSPaymentSettlementProviders, ","), cfg.SaaSPaymentSettlementSyncLimit)
	}
	if persistentBackgroundRecorderEnabled {
		recorder := taskrunner.NewSQLRecorder(getMySQLStore().DB(), structuredLogger(), taskrunner.WithHistoryRetention(14*24*time.Hour, time.Hour, 10000))
		if err := recorder.Ensure(context.Background()); err != nil {
			fatalf("ensure background task recorder: %v", err)
		}
		workerGroup.WithRecorder(recorder)
		debugf("background task recorder enabled: mysql tables=mochat_go_background_tasks,mochat_go_background_task_runs,mochat_go_background_task_executions")
	}
	if err := workerGroup.Start(context.Background()); err != nil {
		fatalf("start background tasks: %v", err)
	}
	if backgroundTasksEnabled {
		options = append(options, compatserver.WithBackgroundTasks(workerGroup.Snapshots))
	}
	if !cfg.RuntimeRole.RunsAPI() {
		debugf("runtime role %s started without HTTP listeners", cfg.RuntimeRole)
		shutdown := make(chan os.Signal, 1)
		signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)
		<-shutdown
		debugf("runtime role %s stopping after shutdown signal", cfg.RuntimeRole)
		return
	}

	modulePrincipalResolver := dashboardModulePrincipalResolver{}
	moduleRouter, err := newSCRMModuleRouter(cfg, getMySQLStore, modulePrincipalResolver)
	if err != nil {
		fatal(err)
	}
	if err := registerAIDebtClearanceModules(moduleRouter, cfg, getMySQLStore, modulePrincipalResolver); err != nil {
		fatal(err)
	}
	if err := registerChatMediaModule(moduleRouter, cfg, getMySQLStore, modulePrincipalResolver); err != nil {
		fatal(err)
	}
	dashboardAccessStore := getMySQLStore()
	dashboardAccessService := dashboard.NewDashboardAccessService(dashboardAccessStore)
	dashboardAccessGuard := dashboard.NewDashboardAccessGuard(dashboardAccessStore, dashboardAccessService)
	dashboardAccessAdminService := dashboard.NewDashboardAccessAdminService(dashboardAccessStore, dashboardAccessService)
	dashboardAccessHTTP := dashboard.NewDashboardAccessHTTP(dashboardAccessAdminService)
	if err := registerDashboardAccessRoutes(moduleRouter, dashboardAccessHTTP); err != nil {
		fatalf("register Dashboard access administration routes: %v", err)
	}
	if dashboardIdentityGuard == nil {
		fatal("Dashboard identity request guard is required")
	}
	dashboardIdentityGuard.WithNext(dashboardAccessGuard)
	options = append(options, compatserver.WithDashboardRequestGuard(dashboardIdentityGuard))
	options = append(options,
		compatserver.WithDashboardAccessHandler(dashboardAccessHTTP),
		compatserver.WithModuleRouter(moduleRouter),
	)
	handler, err := compatserver.New(cfg, options...)
	if err != nil {
		fatalf("build server: %v", err)
	}
	loggedAPIHandler := withHTTPLogging(handler)
	serveHandler := frontend.WrapDashboard(loggedAPIHandler, frontend.DashboardConfig{DistDir: cfg.DashboardDist})
	serveHandler = frontend.WrapApp(serveHandler, frontend.AppConfig{DistDir: cfg.OperationDist, MountPath: "/operation-app/"})
	serveHandler = frontend.WrapApp(serveHandler, frontend.AppConfig{DistDir: cfg.SidebarDist, MountPath: "/sidebar-app/"})
	if cfg.EnableSaaSAdminDashboard {
		serveHandler = frontend.WrapApp(serveHandler, frontend.AppConfig{DistDir: cfg.SaaSAdminDist, MountPath: "/saas-admin/"})
	}
	startFrontend := func(name, addr, dist string) {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			return
		}
		if !frontend.DistAvailable(dist) {
			debugf("%s frontend skipped: dist not found at %s", name, dist)
			return
		}
		appHandler := frontend.WrapApp(loggedAPIHandler, frontend.AppConfig{DistDir: dist})
		frontendListener, err := net.Listen("tcp", addr)
		if err != nil {
			fatalf("%s frontend listen on %s: %v", name, addr, err)
		}
		logRuntimeListening(name, frontendListener.Addr().String(), false)
		go func(listener net.Listener) {
			debugf("%s frontend listening on %s from %s", name, listener.Addr().String(), dist)
			if err := http.Serve(listener, appHandler); err != nil {
				fatalf("%s frontend serve: %v", name, err)
			}
		}(frontendListener)
	}
	startFrontend("sidebar", cfg.SidebarFrontendAddr, cfg.SidebarDist)
	startFrontend("operation", cfg.OperationFrontendAddr, cfg.OperationDist)

	listener, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		fatalf("listen on %s: %v", cfg.ListenAddr, err)
	}
	logRuntimeListening("main", listener.Addr().String(), strings.TrimSpace(cfg.PHPUpstream) != "")
	if err := http.Serve(listener, serveHandler); err != nil {
		fatalf("serve: %v", err)
	}
}

type durableArchiveMediaBatchRunner interface {
	CleanupStaleAttempts(context.Context) (int, error)
	RunOne(context.Context) (bool, error)
}

func runDurableArchiveMediaBatch(ctx context.Context, runner durableArchiveMediaBatchRunner, limit int, logger *log.Logger) error {
	if logger == nil {
		logger = log.Default()
	}
	if _, err := runner.CleanupStaleAttempts(ctx); err != nil {
		logger.Print("go durable archive media attempt cleanup failed")
	}
	failed := false
	for index := 0; index < limit; index++ {
		worked, err := runner.RunOne(ctx)
		if err != nil {
			failed = true
			if !worked {
				break
			}
			continue
		}
		if !worked {
			break
		}
	}
	if failed {
		return errors.New("archive media batch completed with failed items")
	}
	return nil
}
