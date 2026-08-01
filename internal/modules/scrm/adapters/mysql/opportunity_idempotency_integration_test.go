//go:build integration

package mysql

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"jiyi/mochat-go/internal/modules/scrm/ports"
)

func TestOpportunityAndTagCommandsPersistFingerprintsAndRejectOrphans(t *testing.T) {
	_, db, namespace := integrationRepository(t)
	ensureSCRMIdempotencyTable(t, db)
	ctx := context.Background()
	corpID := insertParityCorp(t, db, namespace.tenantID, namespace.id("command-corp"))
	otherCorpID := insertParityCorp(t, db, namespace.tenantID, namespace.id("command-other-corp"))
	otherCorpOwnerID := insertParityEmployee(t, db, otherCorpID, namespace.id("command-other-owner"), 1, false)
	contactID := namespace.id("command-contact")
	secondContactID := namespace.id("command-second-contact")
	otherContactID := namespace.id("command-other-contact")
	now := time.Now().UTC()
	if _, err := db.Exec(`INSERT INTO mochat_go_scrm_contacts(id,tenant_id,corp_id,name,phone,version,created_at,updated_at) VALUES(?,?,?,?,?,1,?,?),(?,?,?,?,?,1,?,?)`, contactID, namespace.tenantID, corpID, "Primary", "", now, now, otherContactID, namespace.tenantID, otherCorpID, "Other", "", now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mochat_go_scrm_contacts(id,tenant_id,corp_id,name,phone,version,created_at,updated_at) VALUES(?,?,?,?,?,1,?,?)`, secondContactID, namespace.tenantID, corpID, "Second", "", now, now); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, statement := range []string{
			"DELETE FROM mochat_go_scrm_idempotency_keys WHERE tenant_id=?",
			"DELETE FROM mochat_go_scrm_contact_tags WHERE tenant_id=?",
			"DELETE FROM mochat_go_scrm_follow_ups WHERE tenant_id=?",
			"DELETE FROM mochat_go_scrm_opportunities WHERE tenant_id=?",
			"DELETE FROM mochat_go_scrm_tags WHERE tenant_id=?",
			"DELETE FROM mochat_go_scrm_contacts WHERE tenant_id=?",
		} {
			_, _ = db.Exec(statement, namespace.tenantID)
		}
	})

	opportunities, err := NewOpportunityRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	tags, err := NewTagRepository(db)
	if err != nil {
		t.Fatal(err)
	}

	create := ports.CreateOpportunityCommand{TenantID: namespace.tenantID, CorpID: corpID, ContactID: contactID, Stage: "proposal", Amount: 100, StartDate: "2026-08-01", EndDate: "2026-08-31", IdempotencyKey: namespace.key("create-opportunity")}
	created, err := opportunities.CreateOpportunity(ctx, create)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := opportunities.CreateOpportunity(ctx, create)
	if err != nil || replayed.ID != created.ID {
		t.Fatalf("create replay=%#v err=%v", replayed, err)
	}
	changedCreate := create
	changedCreate.Amount = 101
	if _, err := opportunities.CreateOpportunity(ctx, changedCreate); !errors.Is(err, ports.ErrAssignmentConflict) {
		t.Fatalf("changed create request err=%v", err)
	}

	stage := ports.ChangeOpportunityStageCommand{TenantID: namespace.tenantID, CorpID: corpID, OpportunityID: created.ID, ToStage: "won", Version: 1, IdempotencyKey: namespace.key("stage")}
	staged, err := opportunities.ChangeOpportunityStage(ctx, stage)
	if err != nil {
		t.Fatal(err)
	}
	stageReplay, err := opportunities.ChangeOpportunityStage(ctx, stage)
	if err != nil || stageReplay.ID != staged.ID || stageReplay.Version != staged.Version {
		t.Fatalf("stage replay=%#v err=%v", stageReplay, err)
	}
	changedStage := stage
	changedStage.ToStage = "lost"
	changedStage.Reason = "changed"
	if _, err := opportunities.ChangeOpportunityStage(ctx, changedStage); !errors.Is(err, ports.ErrAssignmentConflict) {
		t.Fatalf("changed stage request err=%v", err)
	}
	crossCorpStage := stage
	crossCorpStage.CorpID = otherCorpID
	crossCorpStage.IdempotencyKey = namespace.key("stage-other-corp")
	if _, err := opportunities.ChangeOpportunityStage(ctx, crossCorpStage); !errors.Is(err, ports.ErrOpportunityNotFound) {
		t.Fatalf("cross-corp stage err=%v", err)
	}

	follow := ports.AppendFollowUpCommand{TenantID: namespace.tenantID, CorpID: corpID, ContactID: contactID, Content: "first", CreatedBy: 7, IdempotencyKey: namespace.key("follow")}
	firstFollow, err := opportunities.AppendFollowUp(ctx, follow)
	if err != nil {
		t.Fatal(err)
	}
	followReplay, err := opportunities.AppendFollowUp(ctx, follow)
	if err != nil || followReplay.ID != firstFollow.ID {
		t.Fatalf("follow replay=%#v err=%v", followReplay, err)
	}
	changedFollow := follow
	changedFollow.Content = "different"
	if _, err := opportunities.AppendFollowUp(ctx, changedFollow); !errors.Is(err, ports.ErrAssignmentConflict) {
		t.Fatalf("changed follow request err=%v", err)
	}

	createdTag, err := tags.CreateTag(ctx, namespace.tenantID, corpID, "VIP", namespace.key("tag-create"))
	if err != nil {
		t.Fatal(err)
	}
	tagReplay, err := tags.CreateTag(ctx, namespace.tenantID, corpID, "VIP", namespace.key("tag-create"))
	if err != nil || tagReplay.ID != createdTag.ID {
		t.Fatalf("tag create replay=%#v err=%v", tagReplay, err)
	}
	if _, err := tags.CreateTag(ctx, namespace.tenantID, corpID, "Other", namespace.key("tag-create")); !errors.Is(err, ports.ErrAssignmentConflict) {
		t.Fatalf("changed tag create request err=%v", err)
	}

	renamed, err := tags.RenameTag(ctx, namespace.tenantID, corpID, createdTag.ID, "Key", 1, namespace.key("tag-rename"))
	if err != nil {
		t.Fatal(err)
	}
	renameReplay, err := tags.RenameTag(ctx, namespace.tenantID, corpID, createdTag.ID, "Key", 1, namespace.key("tag-rename"))
	if err != nil || renameReplay.Version != renamed.Version {
		t.Fatalf("rename replay=%#v err=%v", renameReplay, err)
	}
	if _, err := tags.RenameTag(ctx, namespace.tenantID, corpID, createdTag.ID, "Changed", 1, namespace.key("tag-rename")); !errors.Is(err, ports.ErrAssignmentConflict) {
		t.Fatalf("changed rename request err=%v", err)
	}

	if err := tags.BindTags(ctx, namespace.tenantID, corpID, createdTag.ID, []string{contactID}, namespace.key("tag-bind")); err != nil {
		t.Fatal(err)
	}
	if err := tags.BindTags(ctx, namespace.tenantID, corpID, createdTag.ID, []string{contactID}, namespace.key("tag-bind")); err != nil {
		t.Fatalf("bind replay: %v", err)
	}
	if err := tags.BindTags(ctx, namespace.tenantID, corpID, createdTag.ID, []string{contactID, secondContactID}, namespace.key("tag-bind")); !errors.Is(err, ports.ErrAssignmentConflict) {
		t.Fatalf("changed bind request err=%v", err)
	}
	if err := tags.BindTags(ctx, namespace.tenantID, corpID, createdTag.ID, []string{contactID, otherContactID}, namespace.key("tag-bind")); !errors.Is(err, ports.ErrContactNotFound) {
		t.Fatalf("cross-corp contact bind err=%v", err)
	}
	if err := tags.BindTags(ctx, namespace.tenantID, corpID, namespace.id("missing-tag"), []string{contactID}, namespace.key("missing-tag")); !errors.Is(err, ports.ErrTagNotFound) {
		t.Fatalf("missing tag bind err=%v", err)
	}

	missingCreate := create
	missingCreate.ContactID = namespace.id("missing-contact")
	missingCreate.IdempotencyKey = namespace.key("missing-opportunity")
	if _, err := opportunities.CreateOpportunity(ctx, missingCreate); !errors.Is(err, ports.ErrContactNotFound) {
		t.Fatalf("missing contact opportunity err=%v", err)
	}
	outOfScopeOwnerCreate := create
	outOfScopeOwnerCreate.OwnerID = otherCorpOwnerID
	outOfScopeOwnerCreate.IdempotencyKey = namespace.key("other-owner-opportunity")
	if _, err := opportunities.CreateOpportunity(ctx, outOfScopeOwnerCreate); !errors.Is(err, ports.ErrAssignmentForbidden) {
		t.Fatalf("cross-corp owner opportunity err=%v", err)
	}
	missingFollow := follow
	missingFollow.ContactID = otherContactID
	missingFollow.IdempotencyKey = namespace.key("cross-corp-follow")
	if _, err := opportunities.AppendFollowUp(ctx, missingFollow); !errors.Is(err, ports.ErrContactNotFound) {
		t.Fatalf("cross-corp follow err=%v", err)
	}

	assertScopedCount(t, db, `SELECT COUNT(*) FROM mochat_go_scrm_opportunities WHERE tenant_id=? AND corp_id=?`, namespace.tenantID, corpID, 1)
	assertScopedCount(t, db, `SELECT COUNT(*) FROM mochat_go_scrm_follow_ups WHERE tenant_id=? AND corp_id=?`, namespace.tenantID, corpID, 1)
	assertScopedCount(t, db, `SELECT COUNT(*) FROM mochat_go_scrm_contact_tags WHERE tenant_id=? AND corp_id=?`, namespace.tenantID, corpID, 1)
}

func ensureSCRMIdempotencyTable(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS mochat_go_scrm_idempotency_keys (
tenant_id bigint unsigned NOT NULL, corp_id bigint unsigned NOT NULL, action varchar(64) NOT NULL,
idempotency_key varchar(128) NOT NULL, request_fingerprint char(64) NOT NULL, resource_id varchar(64) NOT NULL DEFAULT '',
created_at datetime(6) NOT NULL, PRIMARY KEY (tenant_id,corp_id,action,idempotency_key), KEY idx_scrm_idempotency_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`)
	if err != nil {
		t.Fatal(err)
	}
}

func assertScopedCount(t *testing.T, db *sql.DB, query string, tenant, corp int64, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow(query, tenant, corp).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("count=%d want=%d query=%s", got, want, query)
	}
}
