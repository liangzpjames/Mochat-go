//go:build integration

package mysql

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"jiyi/mochat-go/internal/modules/scrm/domain"
	"jiyi/mochat-go/internal/modules/scrm/ports"
)

func TestPublicPoolMariaDBFiltersHistoryIsolationAndAtomicClaims(t *testing.T) {
	_, db, namespace := integrationRepository(t)
	ensureSCRMIdempotencyTable(t, db)
	repository, err := NewCustomerLifecycleRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	corpID := insertParityCorp(t, db, namespace.tenantID, namespace.id("pool-corp"))
	otherCorpID := insertParityCorp(t, db, namespace.tenantID, namespace.id("pool-other-corp"))
	ownerID := insertParityEmployee(t, db, corpID, namespace.id("pool-owner"), 1, false)
	otherOwnerID := insertParityEmployee(t, db, otherCorpID, namespace.id("pool-other-owner"), 1, false)
	contactID, staleID, ownedID, nonPublicID, hiddenID, replayID := namespace.id("pool-contact"), namespace.id("pool-stale"), namespace.id("pool-owned"), namespace.id("pool-non-public"), namespace.id("pool-hidden"), namespace.id("pool-replay")
	tagID := namespace.id("pool-tag")
	cleanup := func() {
		for _, statement := range []string{
			"DELETE FROM mochat_go_scrm_idempotency_keys WHERE tenant_id=? AND corp_id IN (?,?)",
			"DELETE FROM mochat_go_scrm_assignment_history WHERE tenant_id=? AND corp_id IN (?,?)",
			"DELETE FROM mochat_go_scrm_contact_tags WHERE tenant_id=? AND corp_id IN (?,?)",
			"DELETE FROM mochat_go_scrm_follow_ups WHERE tenant_id=? AND corp_id IN (?,?)",
			"DELETE FROM mochat_go_scrm_tags WHERE tenant_id=? AND corp_id IN (?,?)",
			"DELETE FROM mochat_go_scrm_assignments WHERE tenant_id=? AND corp_id IN (?,?)",
			"DELETE FROM mochat_go_scrm_contacts WHERE tenant_id=? AND corp_id IN (?,?)",
		} {
			_, _ = db.Exec(statement, namespace.tenantID, corpID, otherCorpID)
		}
	}
	cleanup()
	t.Cleanup(cleanup)
	if _, err := db.Exec(`INSERT INTO mochat_go_scrm_contacts(id,tenant_id,corp_id,name,phone,source,business_type,region,version,created_at,updated_at) VALUES
		(?,?,?,?,?,'wecom','retail','Shanghai',1,?,?),(?,?,?,?,?,'manual','service','Beijing',1,?,?),(?,?,?,?,?,'wecom','retail','Shanghai',1,?,?),(?,?,?,?,?,'manual','service','Guangzhou',1,?,?),(?,?,?,?,?,'wecom','retail','Shanghai',1,?,?),(?,?,?,?,?,'manual','service','Shenzhen',1,?,?)`,
		contactID, namespace.tenantID, corpID, "Ada", "13800000000", now, now,
		staleID, namespace.tenantID, corpID, "Grace", "13900000000", now, now,
		ownedID, namespace.tenantID, corpID, "Owned", "13700000000", now, now,
		nonPublicID, namespace.tenantID, corpID, "Non Public", "13400000000", now, now,
		hiddenID, namespace.tenantID, otherCorpID, "Hidden", "13600000000", now, now,
		replayID, namespace.tenantID, corpID, "Replay", "13500000000", now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mochat_go_scrm_assignments(id,tenant_id,corp_id,contact_id,owner_id,status,version,created_at,updated_at) VALUES
		(?,?,?,?,NULL,'public_pool',3,?,?),(?,?,?,?,NULL,'public_pool',5,?,?),(?,?,?,?,?,'owned',2,?,?),(?,?,?,?,?,'owned',6,?,?),(?,?,?,?,NULL,'public_pool',1,?,?),(?,?,?,?,NULL,'public_pool',7,?,?)`,
		namespace.id("pool-assignment"), namespace.tenantID, corpID, contactID, now, now,
		namespace.id("pool-stale-assignment"), namespace.tenantID, corpID, staleID, now, now,
		namespace.id("pool-owned-assignment"), namespace.tenantID, corpID, ownedID, ownerID, now, now,
		namespace.id("pool-non-public-assignment"), namespace.tenantID, corpID, nonPublicID, ownerID, now, now,
		namespace.id("pool-hidden-assignment"), namespace.tenantID, otherCorpID, hiddenID, now, now,
		namespace.id("pool-replay-assignment"), namespace.tenantID, corpID, replayID, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mochat_go_scrm_assignment_history(id,tenant_id,corp_id,contact_id,action,previous_owner_id,new_owner_id,actor_id,reason,assignment_version,created_at) VALUES
		(?,?,?,?, 'return', ?,NULL,?,'expired',3,?),(?,?,?,?, 'reclaim', ?,NULL,?,'other',5,?),(?,?,?,?, 'return', ?,NULL,?,'hidden',1,?)`,
		namespace.id("pool-history"), namespace.tenantID, corpID, contactID, ownerID, ownerID, now,
		namespace.id("pool-stale-history"), namespace.tenantID, corpID, staleID, ownerID, ownerID, now,
		namespace.id("pool-hidden-history"), namespace.tenantID, otherCorpID, hiddenID, otherOwnerID, otherOwnerID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mochat_go_scrm_tags(id,tenant_id,corp_id,name,version,created_at,updated_at) VALUES(?,?,?,?,1,?,?)`, tagID, namespace.tenantID, corpID, "VIP", now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mochat_go_scrm_contact_tags(tenant_id,corp_id,contact_id,tag_id,created_at) VALUES(?,?,?,?,?)`, namespace.tenantID, corpID, contactID, tagID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mochat_go_scrm_follow_ups(id,tenant_id,corp_id,contact_id,content,created_by,created_at) VALUES(?,?,?,?,?,?,?)`, namespace.id("pool-follow"), namespace.tenantID, corpID, contactID, "called", ownerID, now); err != nil {
		t.Fatal(err)
	}

	page, err := repository.ListPublicPool(ctx, ports.ListPublicPoolFilter{TenantID: namespace.tenantID, CorpID: corpID, Keyword: "Ada", Sources: []string{"wecom"}, BusinessTypes: []string{"retail"}, TagIDs: []string{tagID}, Regions: []string{"Shanghai"}, Reasons: []string{"expired"}, PreviousOwnerIDs: []int64{ownerID}, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].ContactID != contactID || page.Items[0].ContactName != "Ada" || page.Items[0].PreviousOwnerID == nil || *page.Items[0].PreviousOwnerID != ownerID || page.Items[0].PoolReason != "expired" || page.Items[0].RecycleCount != 1 || len(page.Items[0].TagNames) != 1 || page.Items[0].LastFollowUpAt == nil {
		t.Fatalf("page=%#v", page)
	}
	if hidden, err := repository.ListPublicPool(ctx, ports.ListPublicPoolFilter{TenantID: namespace.tenantID, CorpID: corpID, Keyword: "Hidden", Limit: 20}); err != nil || len(hidden.Items) != 0 {
		t.Fatalf("cross-corp page=%#v err=%v", hidden, err)
	}

	moved, err := repository.MoveToPublicPool(ctx, ports.MoveToPublicPoolCommand{TenantID: namespace.tenantID, CorpID: corpID, ContactID: ownedID, ActorID: ownerID, Version: 2, Action: domain.PublicPoolActionReclaim, Reason: "inactive", IdempotencyKey: namespace.key("reclaim")})
	if err != nil || moved.Status != domain.AssignmentPublicPool || moved.OwnerID != nil {
		t.Fatalf("moved=%#v err=%v", moved, err)
	}
	var previousOwner sql.NullInt64
	var action, reason string
	if err := db.QueryRow(`SELECT action,previous_owner_id,reason FROM mochat_go_scrm_assignment_history WHERE tenant_id=? AND corp_id=? AND contact_id=? ORDER BY created_at DESC,id DESC LIMIT 1`, namespace.tenantID, corpID, ownedID).Scan(&action, &previousOwner, &reason); err != nil || action != "reclaim" || !previousOwner.Valid || previousOwner.Int64 != ownerID || reason != "inactive" {
		t.Fatalf("history action=%q previous=%#v reason=%q err=%v", action, previousOwner, reason, err)
	}

	if _, err := repository.ClaimFromPublicPool(ctx, ports.ClaimPublicPoolCommand{TenantID: namespace.tenantID, CorpID: corpID, ContactID: staleID, UserID: otherOwnerID, Version: 5, IdempotencyKey: namespace.key("cross-owner")}); !errors.Is(err, ports.ErrAssignmentForbidden) {
		t.Fatalf("cross-corp owner err=%v", err)
	}
	for name, command := range map[string]ports.ClaimPublicPoolCommand{
		"missing contact": {TenantID: namespace.tenantID, CorpID: corpID, ContactID: namespace.id("missing"), UserID: ownerID, Version: 1, IdempotencyKey: namespace.key("missing")},
		"cross corp":      {TenantID: namespace.tenantID, CorpID: corpID, ContactID: hiddenID, UserID: ownerID, Version: 1, IdempotencyKey: namespace.key("cross-corp")},
		"not public pool": {TenantID: namespace.tenantID, CorpID: corpID, ContactID: nonPublicID, UserID: ownerID, Version: 6, IdempotencyKey: namespace.key("not-public")},
	} {
		if _, err := repository.ClaimFromPublicPool(ctx, command); !errors.Is(err, ports.ErrAssignmentNotFound) {
			t.Fatalf("%s err=%v, want assignment not found", name, err)
		}
	}
	if _, err := repository.ClaimFromPublicPool(ctx, ports.ClaimPublicPoolCommand{TenantID: namespace.tenantID, CorpID: corpID, ContactID: staleID, UserID: ownerID, Version: 4, IdempotencyKey: namespace.key("stale-version")}); !errors.Is(err, ports.ErrAssignmentConflict) {
		t.Fatalf("stale version err=%v, want conflict", err)
	}

	replayCommand := ports.ClaimPublicPoolCommand{TenantID: namespace.tenantID, CorpID: corpID, ContactID: replayID, UserID: ownerID, Version: 7, IdempotencyKey: namespace.key("claim-replay")}
	firstReplay, err := repository.ClaimFromPublicPool(ctx, replayCommand)
	if err != nil {
		t.Fatal(err)
	}
	secondReplay, err := repository.ClaimFromPublicPool(ctx, replayCommand)
	if err != nil || secondReplay.ID != firstReplay.ID || secondReplay.Version != firstReplay.Version || secondReplay.OwnerID == nil || *secondReplay.OwnerID != ownerID {
		t.Fatalf("same-key replay first=%#v second=%#v err=%v", firstReplay, secondReplay, err)
	}
	replayConflict := replayCommand
	replayConflict.Version = 8
	if _, err := repository.ClaimFromPublicPool(ctx, replayConflict); !errors.Is(err, ports.ErrAssignmentConflict) {
		t.Fatalf("same-key different-request err=%v, want conflict", err)
	}
	var replayHistoryCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_scrm_assignment_history WHERE tenant_id=? AND corp_id=? AND contact_id=? AND action='claim'`, namespace.tenantID, corpID, replayID).Scan(&replayHistoryCount); err != nil || replayHistoryCount != 1 {
		t.Fatalf("replay history count=%d err=%v", replayHistoryCount, err)
	}

	const contenders = 8
	errs := make(chan error, contenders)
	var wg sync.WaitGroup
	for i := 0; i < contenders; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, claimErr := repository.ClaimFromPublicPool(ctx, ports.ClaimPublicPoolCommand{TenantID: namespace.tenantID, CorpID: corpID, ContactID: contactID, UserID: ownerID, Version: 3, IdempotencyKey: namespace.key("claim-" + string(rune('a'+index)))})
			errs <- claimErr
		}(i)
	}
	wg.Wait()
	close(errs)
	successes, conflicts := 0, 0
	for claimErr := range errs {
		if claimErr == nil {
			successes++
		} else if errors.Is(claimErr, ports.ErrAssignmentConflict) {
			conflicts++
		} else {
			t.Fatalf("unexpected claim error: %v", claimErr)
		}
	}
	if successes != 1 || conflicts != contenders-1 {
		t.Fatalf("successes=%d conflicts=%d", successes, conflicts)
	}
	var persistedOwner sql.NullInt64
	if err := db.QueryRow(`SELECT owner_id FROM mochat_go_scrm_assignments WHERE tenant_id=? AND corp_id=? AND contact_id=?`, namespace.tenantID, corpID, contactID).Scan(&persistedOwner); err != nil || !persistedOwner.Valid || persistedOwner.Int64 != ownerID {
		t.Fatalf("persisted owner=%#v err=%v", persistedOwner, err)
	}
}
