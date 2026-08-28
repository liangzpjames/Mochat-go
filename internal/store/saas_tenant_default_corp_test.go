package store

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"jiyi/mochat-go/internal/dashboard"
)

func TestSaaSTenantDefaultCorpValues(t *testing.T) {
	name, wxCorpID := saasTenantDefaultCorpValues(42, "  示例客户  ")
	if name != "示例客户" {
		t.Fatalf("name = %q, want 示例客户", name)
	}
	if wxCorpID != "fake_tenant_42" {
		t.Fatalf("wxCorpID = %q, want fake_tenant_42", wxCorpID)
	}
}

func TestSaaSAdminTenantsUsesBoundCompanyNameForKeyword(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(saasAdminTenantsQueryPattern(true)).
		WithArgs(30, dashboard.SaaSAlertStatusOpen, "%蓝鲸%", "%蓝鲸%", "%蓝鲸%", "%蓝鲸%", "%蓝鲸%", 10).
		WillReturnRows(saasAdminTenantRows("MoChat Test Enterprise", "蓝鲸数字科技（上海）有限公司"))

	tenants, err := NewMySQLStore(db).saasAdminTenants(context.Background(), dashboard.SaaSAdminOverviewOptions{
		ExpiringDays: 30,
		Keyword:      "蓝鲸",
		Limit:        10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(tenants) != 1 || tenants[0].TenantName != "MoChat Test Enterprise" || tenants[0].CompanyName != "蓝鲸数字科技（上海）有限公司" {
		t.Fatalf("tenants = %+v", tenants)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSaaSAdminTenantsFallsBackToTenantNameWhenBoundCompanyNameIsBlank(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(saasAdminTenantsQueryPattern(false)).
		WithArgs(30, dashboard.SaaSAlertStatusOpen, 10).
		WillReturnRows(saasAdminTenantRows("MoChat Test Enterprise", "MoChat Test Enterprise"))

	tenants, err := NewMySQLStore(db).saasAdminTenants(context.Background(), dashboard.SaaSAdminOverviewOptions{
		ExpiringDays: 30,
		Limit:        10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(tenants) != 1 || tenants[0].TenantName != "MoChat Test Enterprise" || tenants[0].CompanyName != tenants[0].TenantName {
		t.Fatalf("tenants = %+v", tenants)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func saasAdminTenantsQueryPattern(withKeyword bool) string {
	pattern := `(?s)SELECT.*COALESCE\(NULLIF\(TRIM\(c\.name\), ''\), NULLIF\(TRIM\(t\.name\), ''\), ''\).*FROM mc_tenant t.*LEFT JOIN mochat_go_tenant_corp_bindings tcb ON tcb\.tenant_id = t\.id.*LEFT JOIN mc_corp c ON c\.tenant_id = t\.id AND c\.id = tcb\.corp_id AND c\.deleted_at IS NULL`
	if withKeyword {
		return pattern + `.*WHERE t\.deleted_at IS NULL AND \(t\.name LIKE \? OR c\.name LIKE \? OR CAST\(t\.id AS CHAR\) LIKE \? OR tp\.package_code LIKE \? OR tp\.package_name LIKE \?\).*LIMIT \?`
	}
	return pattern + `.*WHERE t\.deleted_at IS NULL.*LIMIT \?`
}

func saasAdminTenantRows(tenantName, companyName string) *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"tenant_id", "tenant_name", "company_name", "tenant_status", "package_code", "package_name", "package_status", "package_version", "limits_json", "expires_at", "expired", "expiring_soon", "open_alert_count",
	}).AddRow(1, tenantName, companyName, 1, "pro", "专业版", 1, 2, "{}", "", 0, 0, 0)
}
