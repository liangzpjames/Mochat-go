//go:build integration

package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"jiyi/mochat-go/internal/modules/scrm/domain"
	"jiyi/mochat-go/internal/modules/scrm/ports"
	"jiyi/mochat-go/internal/mysqlconn"
)

func TestLeadRepositoryTenantIsolation(t *testing.T) {
	repository, db := integrationRepository(t)
	ctx := context.Background()
	createdAt := time.Date(2026, time.July, 30, 8, 0, 0, 0, time.UTC)

	mustCreateLead(t, repository, newTestLead(t, "tenant-1-lead", 101, "shared-key", "Tenant One", createdAt))
	mustCreateLead(t, repository, newTestLead(t, "tenant-2-lead", 202, "shared-key", "Tenant Two", createdAt))

	page, err := repository.List(ctx, ports.ListLeadsFilter{TenantID: 101, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].TenantID != 101 || page.Items[0].ID != "tenant-1-lead" {
		t.Fatalf("tenant-scoped page = %#v", page.Items)
	}

	assertTenantBusinessKeyCount(t, db, 101, "shared-key", 1)
	assertTenantBusinessKeyCount(t, db, 202, "shared-key", 1)
}

func TestLeadRepositoryCreateOrGetIsIdempotent(t *testing.T) {
	repository, db := integrationRepository(t)
	createdAt := time.Date(2026, time.July, 30, 8, 0, 0, 0, time.UTC)
	original := newTestLead(t, "lead-original", 101, "request-1", "Original", createdAt)

	first, firstCreated, err := repository.CreateOrGet(context.Background(), original)
	if err != nil {
		t.Fatal(err)
	}
	retry := newTestLead(t, "lead-retry", 101, "request-1", "Retry", createdAt.Add(time.Minute))
	second, secondCreated, err := repository.CreateOrGet(context.Background(), retry)
	if err != nil {
		t.Fatal(err)
	}

	if !firstCreated || secondCreated {
		t.Fatalf("created flags = %v, %v; want true, false", firstCreated, secondCreated)
	}
	if first.ID != "lead-original" || second.ID != first.ID || second.Name.String() != "Original" {
		t.Fatalf("persisted leads = %#v, %#v", first, second)
	}
	assertTenantBusinessKeyCount(t, db, 101, "request-1", 1)
}

func TestLeadRepositoryAllowsSameBusinessKeyAcrossTenants(t *testing.T) {
	repository, db := integrationRepository(t)
	createdAt := time.Date(2026, time.July, 30, 8, 0, 0, 0, time.UTC)

	first, firstCreated, err := repository.CreateOrGet(
		context.Background(),
		newTestLead(t, "lead-tenant-1", 101, "shared-key", "Tenant One", createdAt),
	)
	if err != nil {
		t.Fatal(err)
	}
	second, secondCreated, err := repository.CreateOrGet(
		context.Background(),
		newTestLead(t, "lead-tenant-2", 202, "shared-key", "Tenant Two", createdAt),
	)
	if err != nil {
		t.Fatal(err)
	}

	if !firstCreated || !secondCreated || first.ID == second.ID {
		t.Fatalf("results = (%#v, %v), (%#v, %v)", first, firstCreated, second, secondCreated)
	}
	assertTenantBusinessKeyCount(t, db, 101, "shared-key", 1)
	assertTenantBusinessKeyCount(t, db, 202, "shared-key", 1)
}

func TestLeadRepositoryListUsesStableCreatedAtAndIDOrder(t *testing.T) {
	repository, _ := integrationRepository(t)
	base := time.Date(2026, time.July, 30, 8, 0, 0, 123456000, time.UTC)
	for _, lead := range []domain.Lead{
		newTestLead(t, "lead-a", 101, "key-a", "A", base),
		newTestLead(t, "lead-c", 101, "key-c", "C", base.Add(time.Second)),
		newTestLead(t, "lead-b", 101, "key-b", "B", base),
	} {
		mustCreateLead(t, repository, lead)
	}

	first, err := repository.List(context.Background(), ports.ListLeadsFilter{TenantID: 101, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if got := leadIDs(first.Items); fmt.Sprint(got) != "[lead-c lead-b]" {
		t.Fatalf("first page IDs = %v", got)
	}
	if first.NextCursor == "" {
		t.Fatal("first page cursor is empty")
	}

	second, err := repository.List(context.Background(), ports.ListLeadsFilter{
		TenantID: 101,
		Cursor:   first.NextCursor,
		Limit:    2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := leadIDs(second.Items); fmt.Sprint(got) != "[lead-a]" {
		t.Fatalf("second page IDs = %v", got)
	}
	if second.NextCursor != "" {
		t.Fatalf("second page cursor = %q, want empty", second.NextCursor)
	}
}

func TestLeadRepositoryListIncludesMaximumMySQLTimestampOnFirstPage(t *testing.T) {
	repository, _ := integrationRepository(t)
	maximumMySQLTimestamp := time.Date(9999, time.December, 31, 23, 59, 59, 999999000, time.UTC)
	mustCreateLead(
		t,
		repository,
		newTestLead(t, "maximum-time", 101, "maximum-time", "Maximum Time", maximumMySQLTimestamp),
	)

	page, err := repository.List(context.Background(), ports.ListLeadsFilter{TenantID: 101, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got := leadIDs(page.Items); fmt.Sprint(got) != "[maximum-time]" {
		t.Fatalf("first page IDs = %v", got)
	}
}

func TestLeadRepositoryConcurrentCreateProducesOneRow(t *testing.T) {
	repository, db := integrationRepository(t)
	const calls = 10
	createdAt := time.Date(2026, time.July, 30, 8, 0, 0, 0, time.UTC)

	start := make(chan struct{})
	ids := make(chan string, calls)
	errs := make(chan error, calls)
	var createdCount atomic.Int32
	var wait sync.WaitGroup
	leads := make([]domain.Lead, calls)
	for i := 0; i < calls; i++ {
		leads[i] = newTestLead(t, fmt.Sprintf("concurrent-%02d", i), 101, "concurrent-key", "Concurrent", createdAt)
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			persisted, created, err := repository.CreateOrGet(context.Background(), leads[index])
			if err != nil {
				errs <- err
				return
			}
			if created {
				createdCount.Add(1)
			}
			ids <- persisted.ID
		}(i)
	}
	close(start)
	wait.Wait()
	close(ids)
	close(errs)

	for err := range errs {
		t.Errorf("CreateOrGet: %v", err)
	}
	if got := createdCount.Load(); got != 1 {
		t.Fatalf("created count = %d, want 1", got)
	}
	var persistedID string
	for id := range ids {
		if persistedID == "" {
			persistedID = id
		}
		if id != persistedID {
			t.Fatalf("returned ID = %q, want %q", id, persistedID)
		}
	}
	assertTenantBusinessKeyCount(t, db, 101, "concurrent-key", 1)
}

func integrationRepository(t *testing.T) (*LeadRepository, *sql.DB) {
	t.Helper()
	dsn := os.Getenv("MOCHAT_MYSQL_DSN")
	if dsn == "" {
		t.Skip("MOCHAT_MYSQL_DSN is required for MySQL integration tests")
	}
	db, err := mysqlconn.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(context.Background(), "DELETE FROM mochat_go_scrm_leads"); err != nil {
		t.Fatalf("clean leads table: %v", err)
	}
	repository, err := NewLeadRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	return repository, db
}

func newTestLead(t *testing.T, id string, tenantID int64, businessKey, name string, createdAt time.Time) domain.Lead {
	t.Helper()
	lead, err := domain.NewLead(id, tenantID, businessKey, name, domain.LeadSourceManual, createdAt)
	if err != nil {
		t.Fatal(err)
	}
	return lead
}

func mustCreateLead(t *testing.T, repository *LeadRepository, lead domain.Lead) {
	t.Helper()
	if _, created, err := repository.CreateOrGet(context.Background(), lead); err != nil {
		t.Fatal(err)
	} else if !created {
		t.Fatalf("lead %q already existed", lead.ID)
	}
}

func assertTenantBusinessKeyCount(t *testing.T, db *sql.DB, tenantID int64, businessKey string, want int) {
	t.Helper()
	var got int
	err := db.QueryRowContext(
		context.Background(),
		"SELECT COUNT(*) FROM mochat_go_scrm_leads WHERE tenant_id = ? AND business_key = ?",
		tenantID,
		businessKey,
	).Scan(&got)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("row count for tenant=%d business_key=%q = %d, want %d", tenantID, businessKey, got, want)
	}
}

func leadIDs(leads []domain.Lead) []string {
	ids := make([]string, len(leads))
	for i, lead := range leads {
		ids[i] = lead.ID
	}
	return ids
}
