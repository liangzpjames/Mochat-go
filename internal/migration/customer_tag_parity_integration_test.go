//go:build integration

package migration

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestCustomerTagParityMigrationApplyAndRollbackOnIsolatedMariaDB(t *testing.T) {
	db := newLeadParityMigrationDB(t)
	if _, err := db.Exec(`CREATE TABLE mochat_go_scrm_tags (
		id varchar(36) NOT NULL, tenant_id bigint unsigned NOT NULL, corp_id bigint unsigned NOT NULL,
		name varchar(100) NOT NULL, version bigint unsigned NOT NULL DEFAULT 1, deleted_at datetime(6) NULL,
		created_at datetime(6) NOT NULL, updated_at datetime(6) NOT NULL, PRIMARY KEY(id)
	) ENGINE=InnoDB`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE mochat_go_scrm_contact_tags (
		tenant_id bigint unsigned NOT NULL, corp_id bigint unsigned NOT NULL, contact_id varchar(36) NOT NULL,
		tag_id varchar(36) NOT NULL, created_at datetime(6) NOT NULL, PRIMARY KEY(tenant_id,corp_id,contact_id,tag_id)
	) ENGINE=InnoDB`); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err := db.Exec(`INSERT INTO mochat_go_scrm_tags(id,tenant_id,corp_id,name,version,created_at,updated_at) VALUES('tag-1',1,2,'VIP',1,?,?)`, now, now); err != nil {
		t.Fatal(err)
	}

	root := filepath.Join("..", "..")
	migration := Migration{Version: "0109_scrm_customer_tag_parity", Description: "SCRM customer tag parity", Path: filepath.Join(root, "deploy", "standalone", "migrations", "0109_scrm_customer_tag_parity.up.sql"), DownPath: filepath.Join(root, "deploy", "standalone", "migrations", "0109_scrm_customer_tag_parity.down.sql")}
	runner, err := NewRunner(db, []Migration{migration})
	if err != nil {
		t.Fatal(err)
	}
	items, err := runner.Apply(context.Background())
	if err != nil || len(items) != 1 || items[0].State != "applied_now" {
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
	if rolledBack, err := runner.RollbackLast(context.Background()); err != nil || rolledBack != migration.Version {
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
