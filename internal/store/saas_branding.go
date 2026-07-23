package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"jiyi/mochat-go/internal/dashboard"
)

func (s *MySQLStore) SaaSBrandingProfile(ctx context.Context, tenantID int) (dashboard.SaaSBrandingProfile, error) {
	if tenantID <= 0 {
		return dashboard.SaaSBrandingProfile{}, dashboard.NewSaaSAdminBadRequest("tenantId 无效")
	}
	profile, err := scanSaaSBrandingProfile(s.db.QueryRowContext(ctx, saasBrandingProfileSelect+` WHERE t.id = ? AND t.deleted_at IS NULL LIMIT 1`, tenantID))
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSBrandingProfile{}, dashboard.NewSaaSAdminNotFound("租户不存在")
	}
	if err != nil {
		return dashboard.SaaSBrandingProfile{}, err
	}
	return dashboard.NormalizeSaaSBrandingProfile(profile), nil
}

func (s *MySQLStore) SaaSAdminBrandingProfiles(ctx context.Context, options dashboard.SaaSAdminBrandingOptions) ([]dashboard.SaaSBrandingProfile, error) {
	if options.Limit <= 0 || options.Limit > 100 {
		options.Limit = 100
	}
	where := []string{"t.deleted_at IS NULL"}
	args := make([]any, 0, 8)
	if options.TenantID > 0 {
		where = append(where, "t.id = ?")
		args = append(args, options.TenantID)
	}
	switch options.Status {
	case dashboard.SaaSBrandingFilterConfigured:
		where = append(where, "p.tenant_id IS NOT NULL")
	case dashboard.SaaSBrandingFilterUnconfigured:
		where = append(where, "p.tenant_id IS NULL")
	case dashboard.SaaSBrandingStatusActive, dashboard.SaaSBrandingStatusDisabled:
		where = append(where, "p.status = ?")
		args = append(args, options.Status)
	}
	if options.Keyword != "" {
		like := "%" + options.Keyword + "%"
		where = append(where, "(t.name LIKE ? OR COALESCE(p.product_name, '') LIKE ? OR COALESCE(p.product_short_name, '') LIKE ? OR CAST(t.id AS CHAR) = ?)")
		args = append(args, like, like, like, options.Keyword)
	}
	args = append(args, options.Limit)
	rows, err := s.db.QueryContext(ctx, saasBrandingProfileSelect+` WHERE `+strings.Join(where, " AND ")+` ORDER BY (p.tenant_id IS NOT NULL) DESC, t.id ASC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	profiles := make([]dashboard.SaaSBrandingProfile, 0)
	for rows.Next() {
		profile, err := scanSaaSBrandingProfile(rows)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, dashboard.NormalizeSaaSBrandingProfile(profile))
	}
	return profiles, rows.Err()
}

func (s *MySQLStore) UpsertSaaSAdminBrandingProfile(ctx context.Context, input dashboard.SaaSAdminBrandingProfileUpdate) (dashboard.SaaSAdminBrandingProfileUpdateResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return dashboard.SaaSAdminBrandingProfileUpdateResult{}, err
	}
	defer tx.Rollback()

	var tenantName string
	if err := tx.QueryRowContext(ctx, `SELECT name FROM mc_tenant WHERE id = ? AND deleted_at IS NULL FOR UPDATE`, input.TenantID).Scan(&tenantName); errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminBrandingProfileUpdateResult{}, dashboard.NewSaaSAdminNotFound("租户不存在")
	} else if err != nil {
		return dashboard.SaaSAdminBrandingProfileUpdateResult{}, err
	}
	before, found, err := saasBrandingProfileRow(ctx, tx, input.TenantID, true)
	if err != nil {
		return dashboard.SaaSAdminBrandingProfileUpdateResult{}, err
	}
	if !found {
		if input.ExpectedVersion != 0 {
			return dashboard.SaaSAdminBrandingProfileUpdateResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "品牌档案版本已变化，请刷新后重试"}
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO mochat_go_saas_branding_profiles
				(tenant_id, status, product_name, product_short_name, product_subtitle, logo_url, favicon_url,
				 login_background_url, primary_color, accent_color, website_url, support_url, support_qr_url,
				 support_email, docs_url, footer_text, version, updated_by, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, NOW(), NOW())
		`, input.TenantID, input.Status, input.ProductName, input.ProductShortName, input.ProductSubtitle, input.LogoURL, input.FaviconURL,
			input.LoginBackgroundURL, input.PrimaryColor, input.AccentColor, input.WebsiteURL, input.SupportURL, input.SupportQRURL,
			input.SupportEmail, input.DocsURL, input.FooterText, input.ActorUserID)
	} else {
		if before.Version != input.ExpectedVersion {
			return dashboard.SaaSAdminBrandingProfileUpdateResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "品牌档案版本已变化，请刷新后重试"}
		}
		result, updateErr := tx.ExecContext(ctx, `
			UPDATE mochat_go_saas_branding_profiles
			SET status = ?, product_name = ?, product_short_name = ?, product_subtitle = ?, logo_url = ?, favicon_url = ?,
				login_background_url = ?, primary_color = ?, accent_color = ?, website_url = ?, support_url = ?,
				support_qr_url = ?, support_email = ?, docs_url = ?, footer_text = ?, version = version + 1,
				updated_by = ?, updated_at = NOW()
			WHERE tenant_id = ? AND version = ?
		`, input.Status, input.ProductName, input.ProductShortName, input.ProductSubtitle, input.LogoURL, input.FaviconURL,
			input.LoginBackgroundURL, input.PrimaryColor, input.AccentColor, input.WebsiteURL, input.SupportURL, input.SupportQRURL,
			input.SupportEmail, input.DocsURL, input.FooterText, input.ActorUserID, input.TenantID, input.ExpectedVersion)
		if updateErr != nil {
			return dashboard.SaaSAdminBrandingProfileUpdateResult{}, updateErr
		}
		affected, _ := result.RowsAffected()
		if affected != 1 {
			return dashboard.SaaSAdminBrandingProfileUpdateResult{}, &dashboard.SaaSAdminOperationError{Status: http.StatusConflict, Message: "品牌档案版本已变化，请刷新后重试"}
		}
	}
	if err != nil {
		return dashboard.SaaSAdminBrandingProfileUpdateResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mc_tenant SET logo = ?, login_background = ?, url = ?, copyright = ?, updated_at = NOW() WHERE id = ?`,
		input.LogoURL, input.LoginBackgroundURL, input.WebsiteURL, input.FooterText, input.TenantID); err != nil {
		return dashboard.SaaSAdminBrandingProfileUpdateResult{}, err
	}
	after, found, err := saasBrandingProfileRow(ctx, tx, input.TenantID, false)
	if err != nil {
		return dashboard.SaaSAdminBrandingProfileUpdateResult{}, err
	}
	if !found {
		return dashboard.SaaSAdminBrandingProfileUpdateResult{}, fmt.Errorf("branding profile %d was not persisted", input.TenantID)
	}
	after.TenantName = tenantName
	beforeJSON, _ := json.Marshal(saasBrandingAuditPayload(before))
	afterJSON, _ := json.Marshal(saasBrandingAuditPayload(after))
	operationID, err := insertSaaSAdminOperationLogTx(ctx, tx, dashboard.SaaSAdminOperationLog{
		TenantID: input.TenantID, ActorUserID: input.ActorUserID, ActorTenantID: input.ActorTenantID,
		Action: dashboard.SaaSAdminOperationActionBrandingUpdate, TargetType: dashboard.SaaSAdminOperationTargetBranding,
		TargetID: strconv.Itoa(input.TenantID), TargetName: input.ProductName, BeforeJSON: string(beforeJSON), AfterJSON: string(afterJSON),
		Remark: "update SaaS tenant branding profile",
	})
	if err != nil {
		return dashboard.SaaSAdminBrandingProfileUpdateResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return dashboard.SaaSAdminBrandingProfileUpdateResult{}, err
	}
	return dashboard.SaaSAdminBrandingProfileUpdateResult{Profile: dashboard.NormalizeSaaSBrandingProfile(after), OperationID: operationID}, nil
}

const saasBrandingProfileSelect = `
	SELECT t.id, t.name, CASE WHEN p.tenant_id IS NULL THEN 0 ELSE 1 END,
		COALESCE(p.status, 'active'), COALESCE(p.product_name, 'MoChat Go'), COALESCE(p.product_short_name, 'MoChat Go'),
		COALESCE(p.product_subtitle, '企业微信客户运营平台'), COALESCE(NULLIF(p.logo_url, ''), t.logo, ''),
		COALESCE(NULLIF(p.favicon_url, ''), '/favicon.ico'), COALESCE(NULLIF(p.login_background_url, ''), t.login_background, ''),
		COALESCE(p.primary_color, '#1769AA'), COALESCE(p.accent_color, '#0F578F'),
		COALESCE(NULLIF(p.website_url, ''), t.url, ''), COALESCE(p.support_url, ''), COALESCE(p.support_qr_url, ''),
		COALESCE(p.support_email, ''), COALESCE(p.docs_url, ''), COALESCE(NULLIF(p.footer_text, ''), t.copyright, ''),
		COALESCE(p.version, 0), COALESCE(p.updated_by, 0), p.created_at, p.updated_at
	FROM mc_tenant t
	LEFT JOIN mochat_go_saas_branding_profiles p ON p.tenant_id = t.id`

type saasBrandingScanner interface {
	Scan(...any) error
}

func scanSaaSBrandingProfile(scanner saasBrandingScanner) (dashboard.SaaSBrandingProfile, error) {
	var profile dashboard.SaaSBrandingProfile
	var configured int
	var createdAt, updatedAt sql.NullTime
	err := scanner.Scan(&profile.TenantID, &profile.TenantName, &configured, &profile.Status, &profile.ProductName,
		&profile.ProductShortName, &profile.ProductSubtitle, &profile.LogoURL, &profile.FaviconURL, &profile.LoginBackgroundURL,
		&profile.PrimaryColor, &profile.AccentColor, &profile.WebsiteURL, &profile.SupportURL, &profile.SupportQRURL,
		&profile.SupportEmail, &profile.DocsURL, &profile.FooterText, &profile.Version, &profile.UpdatedBy, &createdAt, &updatedAt)
	profile.Configured = configured == 1
	profile.CreatedAt, profile.UpdatedAt = formatTime(createdAt), formatTime(updatedAt)
	return profile, err
}

func saasBrandingProfileRow(ctx context.Context, queryer saasAdminAccessQueryer, tenantID int, forUpdate bool) (dashboard.SaaSBrandingProfile, bool, error) {
	query := `
		SELECT tenant_id, '', 1, status, product_name, product_short_name, product_subtitle, logo_url, favicon_url,
			login_background_url, primary_color, accent_color, website_url, support_url, support_qr_url,
			support_email, docs_url, footer_text, version, updated_by, created_at, updated_at
		FROM mochat_go_saas_branding_profiles WHERE tenant_id = ?`
	if forUpdate {
		query += " FOR UPDATE"
	}
	profile, err := scanSaaSBrandingProfile(queryer.QueryRowContext(ctx, query, tenantID))
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSBrandingProfile{}, false, nil
	}
	return profile, err == nil, err
}

func saasBrandingAuditPayload(profile dashboard.SaaSBrandingProfile) map[string]any {
	if profile.TenantID == 0 {
		return map[string]any{}
	}
	return map[string]any{
		"tenantId": profile.TenantID, "status": profile.Status, "productName": profile.ProductName,
		"productShortName": profile.ProductShortName, "productSubtitle": profile.ProductSubtitle,
		"logoUrl": profile.LogoURL, "faviconUrl": profile.FaviconURL, "loginBackgroundUrl": profile.LoginBackgroundURL,
		"primaryColor": profile.PrimaryColor, "accentColor": profile.AccentColor, "websiteUrl": profile.WebsiteURL,
		"supportUrl": profile.SupportURL, "supportQrUrl": profile.SupportQRURL, "supportEmail": profile.SupportEmail,
		"docsUrl": profile.DocsURL, "footerText": profile.FooterText, "version": profile.Version,
	}
}
