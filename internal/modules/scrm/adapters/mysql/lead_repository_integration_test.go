//go:build integration

package mysql

import (
	"context"
	"database/sql"
	"errors"
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

var integrationNamespaceSequence atomic.Int64

type integrationNamespace struct {
	tenantID      int64
	otherTenantID int64
	prefix        string
}

func newIntegrationNamespace() integrationNamespace {
	sequence := integrationNamespaceSequence.Add(1)
	tenantID := int64(1_000_000_000) + int64(os.Getpid()%100_000)*1_000 + sequence*2
	return integrationNamespace{
		tenantID:      tenantID,
		otherTenantID: tenantID + 1,
		prefix:        fmt.Sprintf("%d-%d", os.Getpid(), sequence),
	}
}

func (n integrationNamespace) id(suffix string) string {
	return n.prefix + "-" + suffix
}

func (n integrationNamespace) key(suffix string) string {
	return n.prefix + "-" + suffix
}

func resolveMySQLIntegrationDSN(dsn string, required bool) (string, error) {
	if dsn != "" {
		return dsn, nil
	}
	if required {
		return "", errors.New("MOCHAT_MYSQL_DSN is required when MOCHAT_REQUIRE_MYSQL_INTEGRATION=1")
	}
	return "", nil
}

func mysqlIntegrationDSN(t *testing.T) string {
	t.Helper()
	dsn, err := resolveMySQLIntegrationDSN(
		os.Getenv("MOCHAT_MYSQL_DSN"),
		os.Getenv("MOCHAT_REQUIRE_MYSQL_INTEGRATION") == "1",
	)
	if err != nil {
		t.Fatal(err)
	}
	if dsn == "" {
		t.Skip("MOCHAT_MYSQL_DSN is required for MySQL integration tests")
	}
	return dsn
}

func TestLeadRepositoryTenantIsolation(t *testing.T) {
	repository, db, namespace := integrationRepository(t)
	ctx := context.Background()
	createdAt := time.Date(2026, time.July, 30, 8, 0, 0, 0, time.UTC)
	businessKey := namespace.key("shared")
	firstID := namespace.id("tenant-1")
	secondID := namespace.id("tenant-2")

	mustCreateLead(t, repository, newTestLead(t, firstID, namespace.tenantID, businessKey, "Tenant One", createdAt))
	mustCreateLead(t, repository, newTestLead(t, secondID, namespace.otherTenantID, businessKey, "Tenant Two", createdAt))

	page, err := repository.List(ctx, ports.ListLeadsFilter{TenantID: namespace.tenantID, CorpID: 1, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].TenantID != namespace.tenantID || page.Items[0].ID != firstID {
		t.Fatalf("tenant-scoped page = %#v", page.Items)
	}

	assertTenantBusinessKeyCount(t, db, namespace.tenantID, businessKey, 1)
	assertTenantBusinessKeyCount(t, db, namespace.otherTenantID, businessKey, 1)
}

func TestLeadRepositoryCreateOrGetIsIdempotent(t *testing.T) {
	repository, db, namespace := integrationRepository(t)
	createdAt := time.Date(2026, time.July, 30, 8, 0, 0, 0, time.UTC)
	businessKey := namespace.key("request")
	original := newTestLead(t, namespace.id("original"), namespace.tenantID, businessKey, "Original", createdAt)

	first, firstCreated, err := repository.CreateOrGet(context.Background(), original)
	if err != nil {
		t.Fatal(err)
	}
	retry := newTestLead(t, namespace.id("retry"), namespace.tenantID, businessKey, "Retry", createdAt.Add(time.Minute))
	second, secondCreated, err := repository.CreateOrGet(context.Background(), retry)
	if err != nil {
		t.Fatal(err)
	}

	if !firstCreated || secondCreated {
		t.Fatalf("created flags = %v, %v; want true, false", firstCreated, secondCreated)
	}
	if first.ID != original.ID || second.ID != first.ID || second.Name.String() != "Original" {
		t.Fatalf("persisted leads = %#v, %#v", first, second)
	}
	assertTenantBusinessKeyCount(t, db, namespace.tenantID, businessKey, 1)
}

func TestLeadRepositoryAllowsSameBusinessKeyAcrossTenants(t *testing.T) {
	repository, db, namespace := integrationRepository(t)
	createdAt := time.Date(2026, time.July, 30, 8, 0, 0, 0, time.UTC)
	businessKey := namespace.key("shared")

	first, firstCreated, err := repository.CreateOrGet(
		context.Background(),
		newTestLead(t, namespace.id("tenant-1"), namespace.tenantID, businessKey, "Tenant One", createdAt),
	)
	if err != nil {
		t.Fatal(err)
	}
	second, secondCreated, err := repository.CreateOrGet(
		context.Background(),
		newTestLead(t, namespace.id("tenant-2"), namespace.otherTenantID, businessKey, "Tenant Two", createdAt),
	)
	if err != nil {
		t.Fatal(err)
	}

	if !firstCreated || !secondCreated || first.ID == second.ID {
		t.Fatalf("results = (%#v, %v), (%#v, %v)", first, firstCreated, second, secondCreated)
	}
	assertTenantBusinessKeyCount(t, db, namespace.tenantID, businessKey, 1)
	assertTenantBusinessKeyCount(t, db, namespace.otherTenantID, businessKey, 1)
}

func TestLeadRepositoryListUsesStableCreatedAtAndIDOrder(t *testing.T) {
	repository, _, namespace := integrationRepository(t)
	base := time.Date(2026, time.July, 30, 8, 0, 0, 123456000, time.UTC)
	idA := namespace.id("lead-a")
	idB := namespace.id("lead-b")
	idC := namespace.id("lead-c")
	for _, lead := range []domain.Lead{
		newTestLead(t, idA, namespace.tenantID, namespace.key("key-a"), "A", base),
		newTestLead(t, idC, namespace.tenantID, namespace.key("key-c"), "C", base.Add(time.Second)),
		newTestLead(t, idB, namespace.tenantID, namespace.key("key-b"), "B", base),
	} {
		mustCreateLead(t, repository, lead)
	}

	first, err := repository.List(context.Background(), ports.ListLeadsFilter{TenantID: namespace.tenantID, CorpID: 1, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	assertLeadIDs(t, first.Items, idC, idB)
	if first.NextCursor == "" {
		t.Fatal("first page cursor is empty")
	}

	second, err := repository.List(context.Background(), ports.ListLeadsFilter{
		TenantID: namespace.tenantID,
		CorpID:   1,
		Cursor:   first.NextCursor,
		Limit:    2,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertLeadIDs(t, second.Items, idA)
	if second.NextCursor != "" {
		t.Fatalf("second page cursor = %q, want empty", second.NextCursor)
	}
}

func TestLeadRepositoryListIncludesMaximumMySQLTimestampOnFirstPage(t *testing.T) {
	repository, _, namespace := integrationRepository(t)
	// Keep the value within DATETIME after the test DSN's Asia/Shanghai conversion.
	maximumMySQLTimestamp := time.Date(9999, time.December, 31, 15, 59, 59, 999999000, time.UTC)
	id := namespace.id("maximum-time")
	mustCreateLead(
		t,
		repository,
		newTestLead(t, id, namespace.tenantID, namespace.key("maximum-time"), "Maximum Time", maximumMySQLTimestamp),
	)

	page, err := repository.List(context.Background(), ports.ListLeadsFilter{TenantID: namespace.tenantID, CorpID: 1, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	assertLeadIDs(t, page.Items, id)
}

func TestLeadRepositoryConcurrentCreateProducesOneRow(t *testing.T) {
	repository, db, namespace := integrationRepository(t)
	const calls = 10
	createdAt := time.Date(2026, time.July, 30, 8, 0, 0, 0, time.UTC)
	businessKey := namespace.key("concurrent")

	start := make(chan struct{})
	ids := make(chan string, calls)
	errs := make(chan error, calls)
	var createdCount atomic.Int32
	var wait sync.WaitGroup
	leads := make([]domain.Lead, calls)
	for i := 0; i < calls; i++ {
		leads[i] = newTestLead(
			t,
			namespace.id(fmt.Sprintf("concurrent-%02d", i)),
			namespace.tenantID,
			businessKey,
			"Concurrent",
			createdAt,
		)
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
	assertTenantBusinessKeyCount(t, db, namespace.tenantID, businessKey, 1)
}

func TestIntegrationRepositoryPreservesOtherRowsAndCleansOwnTenant(t *testing.T) {
	dsn := mysqlIntegrationDSN(t)
	db, err := mysqlconn.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	sentinelNamespace := newIntegrationNamespace()
	t.Cleanup(func() {
		_, _ = db.ExecContext(
			context.Background(),
			"DELETE FROM mochat_go_scrm_leads WHERE tenant_id IN (?, ?)",
			sentinelNamespace.tenantID,
			sentinelNamespace.otherTenantID,
		)
		_ = db.Close()
	})
	repository, err := NewLeadRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	sentinelID := sentinelNamespace.id("sentinel")
	sentinelKey := sentinelNamespace.key("sentinel")
	mustCreateLead(
		t,
		repository,
		newTestLead(t, sentinelID, sentinelNamespace.tenantID, sentinelKey, "Sentinel", time.Now().UTC()),
	)

	var ownedTenant int64
	var ownedKey string
	t.Run("scoped fixture", func(t *testing.T) {
		scopedRepository, _, namespace := integrationRepository(t)
		ownedTenant = namespace.tenantID
		ownedKey = namespace.key("owned")
		mustCreateLead(
			t,
			scopedRepository,
			newTestLead(t, namespace.id("owned"), ownedTenant, ownedKey, "Owned", time.Now().UTC()),
		)
	})

	sentinelCount := tenantBusinessKeyCount(t, db, sentinelNamespace.tenantID, sentinelKey)
	ownedCount := tenantBusinessKeyCount(t, db, ownedTenant, ownedKey)
	if sentinelCount != 1 || ownedCount != 0 {
		t.Fatalf("post-fixture counts = sentinel:%d owned:%d, want sentinel:1 owned:0", sentinelCount, ownedCount)
	}
}

func integrationRepository(t *testing.T) (*LeadRepository, *sql.DB, integrationNamespace) {
	t.Helper()
	dsn := mysqlIntegrationDSN(t)
	db, err := mysqlconn.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	namespace := newIntegrationNamespace()
	t.Cleanup(func() {
		if _, err := db.ExecContext(
			context.Background(),
			"DELETE FROM mochat_go_scrm_leads WHERE tenant_id IN (?, ?)",
			namespace.tenantID,
			namespace.otherTenantID,
		); err != nil {
			t.Errorf("clean integration tenant namespace: %v", err)
		}
		_ = db.Close()
	})
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	repository, err := NewLeadRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	return repository, db, namespace
}

func newTestLead(t *testing.T, id string, tenantID int64, businessKey, name string, createdAt time.Time) domain.Lead {
	t.Helper()
	lead, err := domain.NewLead(id, tenantID, businessKey, name, domain.LeadSourceManual, createdAt)
	if err != nil {
		t.Fatal(err)
	}
	lead.CorpID = 1
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
	got := tenantBusinessKeyCount(t, db, tenantID, businessKey)
	if got != want {
		t.Fatalf("row count for tenant=%d business_key=%q = %d, want %d", tenantID, businessKey, got, want)
	}
}

func tenantBusinessKeyCount(t *testing.T, db *sql.DB, tenantID int64, businessKey string) int {
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
	return got
}

func assertLeadIDs(t *testing.T, leads []domain.Lead, want ...string) {
	t.Helper()
	got := leadIDs(leads)
	if len(got) != len(want) {
		t.Fatalf("lead IDs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("lead IDs = %v, want %v", got, want)
		}
	}
}

func leadIDs(leads []domain.Lead) []string {
	ids := make([]string, len(leads))
	for i, lead := range leads {
		ids[i] = lead.ID
	}
	return ids
}
