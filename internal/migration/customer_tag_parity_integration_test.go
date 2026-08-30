//go:build integration

package migration

import (
	"context"
	"testing"
	"time"
)

func TestCustomerTagParityMigrationApplyAndRollbackOnIsolatedMariaDB(t *testing.T) {
	db := newMigrationIntegrationDBThrough(t, "0108_scrm_public_pool_parity")
	now := time.Now().UTC()
	if _, err := db.Exec(`INSERT INTO mochat_go_scrm_tags(id,tenant_id,corp_id,name,version,created_at,updated_at) VALUES('tag-1',1,2,'VIP',1,?,?)`, now, now); err != nil {
		t.Fatal(err)
	}

	runner := newMigrationRunnerThrough(t, db, "0109_scrm_customer_tag_parity")
	items, err := runner.Apply(context.Background())
	if err != nil || len(items) == 0 || items[len(items)-1].Migration.Version != "0109_scrm_customer_tag_parity" || items[len(items)-1].State != "applied_now" {
		t.Fatalf("apply items=%#v err=%v", items, err)
	}
	if !tableExists(t, db, "mochat_go_scrm_tag_groups") || !columnExists(t, db, "mochat_go_scrm_tags", "group_id") {
		t.Fatal("0109 catalog schema missing")
	}
	if !indexExists(t, db, "mochat_go_scrm_tags", "idx_scrm_tag_catalog") || !indexExists(t, db, "mochat_go_scrm_contact_tags", "idx_scrm_contact_tag_usage") {
		t.Fatal("0109 indexes missing")
	}
	var groupID string
	if err := db.QueryRow(`SELECT group_id FROM mochat_go_scrm_tags WHERE id='tag-1'`).Scan(&groupID); err != nil || groupID == "" {
		t.Fatalf("group backfill=%q err=%v", groupID, err)
	}
	if rolledBack, err := runner.RollbackLast(context.Background()); err != nil || rolledBack != "0109_scrm_customer_tag_parity" {
		t.Fatalf("rollback=%q err=%v", rolledBack, err)
	}
	if tableExists(t, db, "mochat_go_scrm_tag_groups") || columnExists(t, db, "mochat_go_scrm_tags", "group_id") {
		t.Fatal("0109 down left catalog schema")
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_scrm_tags WHERE id='tag-1'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("tag preserved count=%d err=%v", count, err)
	}
}
