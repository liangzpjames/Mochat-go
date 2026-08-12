package store

import (
	"context"
	"strings"

	"jiyi/mochat-go/internal/dashboard"
)

const saasTenantReadinessQueryMaxLimit = 5000

func (s *MySQLStore) SaaSAdminTenantReadinessFacts(ctx context.Context, options dashboard.SaaSAdminTenantReadinessOptions) (dashboard.SaaSAdminTenantReadinessFactPage, error) {
	where := []string{"t.deleted_at IS NULL"}
	args := make([]any, 0, 5)
	if options.PlatformAdminTenantID > 0 {
		where = append(where, "t.id <> ?")
		args = append(args, options.PlatformAdminTenantID)
	}
	if options.TenantID > 0 {
		where = append(where, "t.id = ?")
		args = append(args, options.TenantID)
	}
	if strings.TrimSpace(options.Keyword) != "" {
		keyword := "%" + strings.TrimSpace(options.Keyword) + "%"
		where = append(where, "(CAST(t.id AS CHAR) LIKE ? OR COALESCE(t.name, '') LIKE ?)")
		args = append(args, keyword, keyword)
	}
	args = append(args, saasTenantReadinessQueryMaxLimit+1)
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			t.id, COALESCE(t.name, ''), COALESCE(t.status, 0),
			COALESCE(admins.active_super_admin_count, 0),
			COALESCE(package.package_code, ''), COALESCE(package.package_name, ''), COALESCE(package.status, 0),
			COALESCE(DATE_FORMAT(package.expires_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(subscription.id, 0), COALESCE(subscription.status, ''),
			COALESCE(DATE_FORMAT(subscription.trial_ends_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(subscription.current_period_ends_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(DATE_FORMAT(subscription.grace_ends_at, '%Y-%m-%d %H:%i:%s'), ''),
			COALESCE(subscription.cancel_at_period_end, 0),
			COALESCE(seeds.core_seed_count, 0),
			COALESCE(corps.bound_corp_count, 0), COALESCE(corps.configured_credential_count, 0), COALESCE(corps.protected_credential_count, 0),
			COALESCE(identity_policy.status, ''),
			COALESCE(notification.enabled_policy_count, 0), COALESCE(notification.secure_policy_count, 0),
			COALESCE(branding.status, ''),
			COALESCE(domains.primary_verified_count, 0), COALESCE(domains.ready_primary_count, 0)
		FROM mc_tenant t
		LEFT JOIN (
			SELECT tenant_id, COUNT(*) AS active_super_admin_count
			FROM mc_user
			WHERE deleted_at IS NULL AND status = 1 AND isSuperAdmin = 1
			GROUP BY tenant_id
		) admins ON admins.tenant_id = t.id
		LEFT JOIN mochat_go_saas_tenant_packages package
			ON package.tenant_id = t.id AND package.deleted_at IS NULL
		LEFT JOIN mochat_go_saas_subscriptions subscription
			ON subscription.tenant_id = t.id AND subscription.deleted_at IS NULL
		LEFT JOIN (
			SELECT target_id AS tenant_id, COUNT(DISTINCT seed_name) AS core_seed_count
			FROM mochat_go_seed_versions
			WHERE scope = 'tenant' AND seed_name IN ('rbac_menu', 'contact_field', 'chat_tool')
			GROUP BY target_id
		) seeds ON seeds.tenant_id = t.id
		LEFT JOIN (
			SELECT tenant_id,
				SUM(CASE WHEN TRIM(COALESCE(wx_corpid, '')) <> '' THEN 1 ELSE 0 END) AS bound_corp_count,
				SUM(CASE WHEN TRIM(COALESCE(wx_corpid, '')) <> '' AND COALESCE(wecom_credentials_ciphertext, '') <> ''
					THEN 1 ELSE 0 END) AS configured_credential_count,
				SUM(CASE WHEN TRIM(COALESCE(wx_corpid, '')) <> '' AND
					COALESCE(wecom_credentials_ciphertext, '') <> '' AND TRIM(COALESCE(wecom_credentials_key_id, '')) <> ''
				THEN 1 ELSE 0 END) AS protected_credential_count
			FROM mc_corp
			WHERE deleted_at IS NULL
			GROUP BY tenant_id
		) corps ON corps.tenant_id = t.id
		LEFT JOIN mochat_go_saas_identity_policies identity_policy ON identity_policy.tenant_id = t.id
		LEFT JOIN (
			SELECT tenant_id,
				SUM(CASE WHEN enabled = 1 THEN 1 ELSE 0 END) AS enabled_policy_count,
				SUM(CASE WHEN enabled = 1 AND COALESCE(webhook_credentials_ciphertext, '') <> '' AND
					TRIM(COALESCE(webhook_credentials_key_id, '')) <> '' AND TRIM(COALESCE(webhook_url, '')) = '' AND
					TRIM(COALESCE(webhook_secret, '')) = '' THEN 1 ELSE 0 END) AS secure_policy_count
			FROM mochat_go_saas_alert_settings
			WHERE deleted_at IS NULL
			GROUP BY tenant_id
		) notification ON notification.tenant_id = t.id
		LEFT JOIN mochat_go_saas_branding_profiles branding ON branding.tenant_id = t.id
		LEFT JOIN (
			SELECT domain.tenant_id,
				SUM(CASE WHEN domain.is_primary = 1 AND domain.status = 'active' AND domain.verified_at IS NOT NULL THEN 1 ELSE 0 END) AS primary_verified_count,
				SUM(CASE WHEN domain.is_primary = 1 AND domain.status = 'active' AND domain.verified_at IS NOT NULL AND
					delivery.delivery_status = 'ready' AND delivery.routing_status = 'ready' AND delivery.certificate_status = 'active'
				THEN 1 ELSE 0 END) AS ready_primary_count
			FROM mochat_go_saas_tenant_domains domain
			LEFT JOIN mochat_go_saas_tenant_domain_deliveries delivery ON delivery.domain_id = domain.id
			WHERE domain.deleted_at IS NULL
			GROUP BY domain.tenant_id
		) domains ON domains.tenant_id = t.id
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY t.id ASC
		LIMIT ?
	`, args...)
	if err != nil {
		return dashboard.SaaSAdminTenantReadinessFactPage{}, err
	}
	defer rows.Close()

	page := dashboard.SaaSAdminTenantReadinessFactPage{Facts: make([]dashboard.SaaSAdminTenantReadinessFacts, 0)}
	for rows.Next() {
		var item dashboard.SaaSAdminTenantReadinessFacts
		var cancelAtPeriodEnd int
		if err := rows.Scan(
			&item.TenantID, &item.TenantName, &item.TenantStatus,
			&item.ActiveSuperAdminCount,
			&item.PackageCode, &item.PackageName, &item.PackageStatus, &item.PackageExpiresAt,
			&item.SubscriptionID, &item.SubscriptionStatus, &item.SubscriptionTrialEndsAt,
			&item.SubscriptionCurrentPeriodEndsAt, &item.SubscriptionGraceEndsAt, &cancelAtPeriodEnd,
			&item.CoreSeedCount,
			&item.BoundCorpCount, &item.ConfiguredCorpCredentialCount, &item.ProtectedCorpCredentialCount,
			&item.IdentityPolicyStatus,
			&item.EnabledNotificationPolicyCount, &item.SecureNotificationPolicyCount,
			&item.BrandingStatus,
			&item.PrimaryVerifiedDomainCount, &item.ReadyPrimaryDomainCount,
		); err != nil {
			return dashboard.SaaSAdminTenantReadinessFactPage{}, err
		}
		item.SubscriptionCancelAtPeriodEnd = cancelAtPeriodEnd == 1
		page.Facts = append(page.Facts, item)
	}
	if err := rows.Err(); err != nil {
		return dashboard.SaaSAdminTenantReadinessFactPage{}, err
	}
	if len(page.Facts) > saasTenantReadinessQueryMaxLimit {
		page.Facts = page.Facts[:saasTenantReadinessQueryMaxLimit]
		page.Truncated = true
	}
	return page, nil
}
