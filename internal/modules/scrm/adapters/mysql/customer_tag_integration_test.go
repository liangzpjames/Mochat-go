//go:build integration

package mysql

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"jiyi/mochat-go/internal/modules/scrm/ports"
)

func TestCustomerTagMariaDBCatalogIsolationVersionsAndIdempotency(t *testing.T) {
	_, db, namespace := integrationRepository(t)
	ensureSCRMIdempotencyTable(t, db)
	repository, err := NewTagRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	corpID := insertParityCorp(t, db, namespace.tenantID, namespace.id("tag-corp"))
	otherCorpID := insertParityCorp(t, db, namespace.tenantID, namespace.id("tag-other-corp"))
	contact1, contact2, otherContact := namespace.id("tag-contact-1"), namespace.id("tag-contact-2"), namespace.id("tag-other-contact")
	now := time.Now().UTC()
	if _, err := db.Exec(`INSERT INTO mochat_go_scrm_contacts(id,tenant_id,corp_id,name,phone,version,created_at,updated_at) VALUES(?,?,?,?,?,1,?,?),(?,?,?,?,?,1,?,?),(?,?,?,?,?,1,?,?)`,
		contact1, namespace.tenantID, corpID, "Ada", "", now, now,
		contact2, namespace.tenantID, corpID, "Grace", "", now, now,
		otherContact, namespace.tenantID, otherCorpID, "Hidden", "", now, now); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, statement := range []string{
			"DELETE FROM mochat_go_scrm_idempotency_keys WHERE tenant_id=? AND corp_id IN (?,?)",
			"DELETE FROM mochat_go_scrm_contact_tags WHERE tenant_id=? AND corp_id IN (?,?)",
			"DELETE FROM mochat_go_scrm_tags WHERE tenant_id=? AND corp_id IN (?,?)",
			"DELETE FROM mochat_go_scrm_tag_groups WHERE tenant_id=? AND corp_id IN (?,?)",
			"DELETE FROM mochat_go_scrm_contacts WHERE tenant_id=? AND corp_id IN (?,?)",
		} {
			_, _ = db.Exec(statement, namespace.tenantID, corpID, otherCorpID)
		}
	})

	groupCommand := ports.CreateTagGroupCommand{TenantID: namespace.tenantID, CorpID: corpID, Name: namespace.id("客户等级"), IdempotencyKey: namespace.key("group-create")}
	group, err := repository.CreateGroup(ctx, groupCommand)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := repository.CreateGroup(ctx, groupCommand)
	if err != nil || replay.ID != group.ID {
		t.Fatalf("group replay=%#v err=%v", replay, err)
	}
	changedGroup := groupCommand
	changedGroup.Name = namespace.id("changed")
	if _, err := repository.CreateGroup(ctx, changedGroup); !errors.Is(err, ports.ErrAssignmentConflict) {
		t.Fatalf("changed group replay err=%v", err)
	}
	duplicateGroup := groupCommand
	duplicateGroup.IdempotencyKey = namespace.key("group-duplicate")
	if _, err := repository.CreateGroup(ctx, duplicateGroup); !errors.Is(err, ports.ErrDuplicateTagName) {
		t.Fatalf("duplicate group err=%v", err)
	}
	secondGroup, err := repository.CreateGroup(ctx, ports.CreateTagGroupCommand{TenantID: namespace.tenantID, CorpID: corpID, Name: namespace.id("地域"), IdempotencyKey: namespace.key("group-second")})
	if err != nil {
		t.Fatal(err)
	}
	concurrentGroupName := namespace.id("并发组")
	groupErrors := make(chan error, 2)
	var groupWait sync.WaitGroup
	for index := 0; index < 2; index++ {
		groupWait.Add(1)
		go func(index int) {
			defer groupWait.Done()
			_, createErr := repository.CreateGroup(ctx, ports.CreateTagGroupCommand{TenantID: namespace.tenantID, CorpID: corpID, Name: concurrentGroupName, IdempotencyKey: namespace.key(fmt.Sprintf("group-concurrent-%d", index))})
			groupErrors <- createErr
		}(index)
	}
	groupWait.Wait()
	close(groupErrors)
	assertOneConcurrentDuplicate(t, groupErrors)

	tagCommand := ports.CreateCustomerTagCommand{TenantID: namespace.tenantID, CorpID: corpID, GroupID: group.ID, Name: namespace.id("VIP"), IdempotencyKey: namespace.key("tag-create")}
	tag, err := repository.CreateCustomerTag(ctx, tagCommand)
	if err != nil {
		t.Fatal(err)
	}
	concurrentTagName := namespace.id("并发标签")
	tagErrors := make(chan error, 2)
	var tagWait sync.WaitGroup
	for index := 0; index < 2; index++ {
		tagWait.Add(1)
		go func(index int) {
			defer tagWait.Done()
			_, createErr := repository.CreateCustomerTag(ctx, ports.CreateCustomerTagCommand{TenantID: namespace.tenantID, CorpID: corpID, GroupID: group.ID, Name: concurrentTagName, IdempotencyKey: namespace.key(fmt.Sprintf("tag-concurrent-%d", index))})
			tagErrors <- createErr
		}(index)
	}
	tagWait.Wait()
	close(tagErrors)
	assertOneConcurrentDuplicate(t, tagErrors)
	duplicateTag := tagCommand
	duplicateTag.IdempotencyKey = namespace.key("tag-duplicate")
	if _, err := repository.CreateCustomerTag(ctx, duplicateTag); !errors.Is(err, ports.ErrDuplicateTagName) {
		t.Fatalf("duplicate tag err=%v", err)
	}
	sameNameOtherGroup := tagCommand
	sameNameOtherGroup.GroupID = secondGroup.ID
	sameNameOtherGroup.IdempotencyKey = namespace.key("tag-other-group")
	if _, err := repository.CreateCustomerTag(ctx, sameNameOtherGroup); err != nil {
		t.Fatalf("same name in another group: %v", err)
	}

	maintain := ports.MaintainTagContactsCommand{TenantID: namespace.tenantID, CorpID: corpID, TagID: tag.ID, AddContactIDs: []string{contact1, contact2}, Version: 1, IdempotencyKey: namespace.key("tag-maintain")}
	maintained, err := repository.MaintainTagContacts(ctx, maintain)
	if err != nil || maintained.Version != 2 || maintained.UsageCount != 2 {
		t.Fatalf("maintained=%#v err=%v", maintained, err)
	}
	replayedTag, err := repository.MaintainTagContacts(ctx, maintain)
	if err != nil || replayedTag.UsageCount != 2 {
		t.Fatalf("maintain replay=%#v err=%v", replayedTag, err)
	}
	changedMaintain := maintain
	changedMaintain.RemoveContactIDs = []string{contact1}
	if _, err := repository.MaintainTagContacts(ctx, changedMaintain); !errors.Is(err, ports.ErrAssignmentConflict) {
		t.Fatalf("changed maintain err=%v", err)
	}
	crossCorp := ports.MaintainTagContactsCommand{TenantID: namespace.tenantID, CorpID: corpID, TagID: tag.ID, AddContactIDs: []string{otherContact}, Version: 2, IdempotencyKey: namespace.key("cross-corp")}
	if _, err := repository.MaintainTagContacts(ctx, crossCorp); !errors.Is(err, ports.ErrContactNotFound) {
		t.Fatalf("cross-corp contact err=%v", err)
	}

	catalog, err := repository.ListTagCatalog(ctx, ports.ListTagCatalogFilter{TenantID: namespace.tenantID, CorpID: corpID, GroupID: group.ID, Keyword: "VIP"})
	if err != nil || len(catalog.Groups) != 2 || len(catalog.Tags) != 1 || catalog.Tags[0].UsageCount != 2 {
		t.Fatalf("catalog=%#v err=%v", catalog, err)
	}
	hidden, err := repository.ListTagCatalog(ctx, ports.ListTagCatalogFilter{TenantID: namespace.tenantID, CorpID: otherCorpID, Keyword: "VIP"})
	if err != nil || len(hidden.Tags) != 0 {
		t.Fatalf("cross-corp catalog=%#v err=%v", hidden, err)
	}

	if _, err := repository.MoveCustomerTag(ctx, ports.MoveCustomerTagCommand{TenantID: namespace.tenantID, CorpID: corpID, TagID: tag.ID, GroupID: secondGroup.ID, Version: 2, IdempotencyKey: namespace.key("move-duplicate")}); !errors.Is(err, ports.ErrDuplicateTagName) {
		t.Fatalf("duplicate move err=%v", err)
	}
	deleted, err := repository.DeleteCustomerTag(ctx, ports.DeleteCustomerTagCommand{TenantID: namespace.tenantID, CorpID: corpID, TagID: tag.ID, Version: 2, IdempotencyKey: namespace.key("delete")})
	if err != nil || deleted.AffectedResourceCount != 2 {
		t.Fatalf("deleted=%#v err=%v", deleted, err)
	}
	deleteReplay, err := repository.DeleteCustomerTag(ctx, ports.DeleteCustomerTagCommand{TenantID: namespace.tenantID, CorpID: corpID, TagID: tag.ID, Version: 2, IdempotencyKey: namespace.key("delete")})
	if err != nil || deleteReplay.AffectedResourceCount != 2 {
		t.Fatalf("delete replay=%#v err=%v", deleteReplay, err)
	}
	if _, err := repository.DeleteCustomerTag(ctx, ports.DeleteCustomerTagCommand{TenantID: namespace.tenantID, CorpID: otherCorpID, TagID: tag.ID, Version: 2, IdempotencyKey: namespace.key("hidden-delete")}); !errors.Is(err, ports.ErrTagNotFound) {
		t.Fatalf("cross-corp delete err=%v", err)
	}
}

func assertOneConcurrentDuplicate(t *testing.T, errorsChannel <-chan error) {
	t.Helper()
	succeeded, duplicated := 0, 0
	for err := range errorsChannel {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, ports.ErrDuplicateTagName):
			duplicated++
		default:
			t.Fatalf("unexpected concurrent create error: %v", err)
		}
	}
	if succeeded != 1 || duplicated != 1 {
		t.Fatalf("concurrent create succeeded=%d duplicated=%d", succeeded, duplicated)
	}
}
