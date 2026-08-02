//go:build integration

package mysql

import (
	"context"
	"errors"
	"testing"
	"time"

	"jiyi/mochat-go/internal/modules/scrm/domain"
	"jiyi/mochat-go/internal/modules/scrm/ports"
)

func TestContactLifecycleMariaDBIsolationCombinedFilterAndAggregateDetail(t *testing.T) {
	_, db, namespace := integrationRepository(t)
	repository, err := NewCustomerLifecycleRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	now := time.Now().UTC()
	corpID := insertParityCorp(t, db, namespace.tenantID, namespace.id("contact-corp"))
	otherCorpID := insertParityCorp(t, db, namespace.tenantID, namespace.id("contact-other-corp"))
	ownerID := insertParityEmployee(t, db, corpID, namespace.id("contact-owner"), 1, false)
	legacyContactResult, err := db.Exec(`INSERT INTO mc_work_contact(corp_id,wx_external_userid,name,nick_name,unionid,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, corpID, namespace.id("external"), "Ada Contact", "Ada Contact", namespace.id("union"), now, now)
	if err != nil {
		t.Fatal(err)
	}
	legacyContactID, err := legacyContactResult.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	legacyRelationResult, err := db.Exec(`INSERT INTO mc_work_contact_employee(employee_id,contact_id,add_way,corp_id,status,create_time,created_at,updated_at) VALUES(?,?,?,?,1,?,?,?)`, ownerID, legacyContactID, 1, corpID, now, now, now)
	if err != nil {
		t.Fatal(err)
	}
	legacyRelationID, err := legacyRelationResult.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM mc_work_contact_employee WHERE id=?", legacyRelationID)
		_, _ = db.Exec("DELETE FROM mc_work_contact WHERE id=?", legacyContactID)
	})
	contactID := namespace.id("contact")
	hiddenID := namespace.id("hidden")
	tagID := namespace.id("tag")
	cleanup := func() {
		for _, statement := range []string{
			"DELETE FROM mochat_go_scrm_contact_tags WHERE tenant_id=? AND corp_id IN (?,?)",
			"DELETE FROM mochat_go_scrm_follow_ups WHERE tenant_id=? AND corp_id IN (?,?)",
			"DELETE FROM mochat_go_scrm_opportunities WHERE tenant_id=? AND corp_id IN (?,?)",
			"DELETE FROM mochat_go_scrm_tags WHERE tenant_id=? AND corp_id IN (?,?)",
			"DELETE FROM mochat_go_scrm_assignment_collaborators WHERE tenant_id=? AND corp_id IN (?,?)",
			"DELETE FROM mochat_go_scrm_assignments WHERE tenant_id=? AND corp_id IN (?,?)",
			"DELETE FROM mochat_go_scrm_contacts WHERE tenant_id=? AND corp_id IN (?,?)",
		} {
			_, _ = db.Exec(statement, namespace.tenantID, corpID, otherCorpID)
		}
	}
	cleanup()
	t.Cleanup(cleanup)
	if _, err = db.Exec(`INSERT INTO mochat_go_scrm_contacts(id,tenant_id,corp_id,name,phone,version,created_at,updated_at) VALUES(?,?,?,?,?,2,?,?),(?,?,?,?,?,1,?,?)`, contactID, namespace.tenantID, corpID, "Ada Contact", "13800000000", now, now, hiddenID, namespace.tenantID, otherCorpID, "Ada Hidden", "13900000000", now, now); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO mochat_go_scrm_assignments(id,tenant_id,corp_id,contact_id,owner_id,status,version,created_at,updated_at) VALUES(?,?,?,?,?,'owned',4,?,?),(?,?,?,?,NULL,'public_pool',1,?,?)`, namespace.id("assignment"), namespace.tenantID, corpID, contactID, ownerID, now, now, namespace.id("hidden-assignment"), namespace.tenantID, otherCorpID, hiddenID, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO mochat_go_scrm_tags(id,tenant_id,corp_id,name,version,created_at,updated_at) VALUES(?,?,?,?,1,?,?)`, tagID, namespace.tenantID, corpID, "VIP", now, now); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO mochat_go_scrm_contact_tags(tenant_id,corp_id,contact_id,tag_id,created_at) VALUES(?,?,?,?,?)`, namespace.tenantID, corpID, contactID, tagID, now); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO mochat_go_scrm_follow_ups(id,tenant_id,corp_id,contact_id,content,created_by,created_at) VALUES(?,?,?,?,?,?,?)`, namespace.id("follow"), namespace.tenantID, corpID, contactID, "first call", ownerID, now); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO mochat_go_scrm_opportunities(id,tenant_id,corp_id,contact_id,stage_id,status,version,amount,start_date,end_date,created_at,updated_at) VALUES(?,?,?,?,?,'open',1,100,?,?,?,?)`, namespace.id("opportunity"), namespace.tenantID, corpID, contactID, "proposal", now, now.Add(24*time.Hour), now, now); err != nil {
		t.Fatal(err)
	}

	page, err := repository.ListContacts(ctx, ports.ListContactsFilter{TenantID: namespace.tenantID, CorpID: corpID, Keyword: "Ada", OwnerIDs: []int64{ownerID}, TagIDs: []string{tagID}, Statuses: []string{domain.AssignmentOwned}, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != contactID || len(page.Items[0].TagNames) != 1 {
		t.Fatalf("page=%#v", page)
	}
	detail, err := repository.GetContact(ctx, namespace.tenantID, corpID, contactID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Assignment.Version != 4 || len(detail.Tags) != 1 || len(detail.Opportunities) != 1 || len(detail.FollowUps) != 1 {
		t.Fatalf("detail=%#v", detail)
	}
	if detail.WeComFriendsAvailable || len(detail.WeComFriends) != 0 {
		t.Fatalf("same-name legacy contact must not be guessed as linked: %#v", detail.WeComFriends)
	}
	if _, err := repository.GetContact(ctx, namespace.tenantID, corpID, hiddenID); !errors.Is(err, ports.ErrContactNotFound) {
		t.Fatalf("cross-corp err=%v", err)
	}
}
