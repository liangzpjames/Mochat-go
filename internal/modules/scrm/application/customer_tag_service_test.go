package application

import (
	"context"
	"errors"
	"testing"

	"jiyi/mochat-go/internal/modules/scrm/ports"
)

func TestCustomerTagServiceValidatesAndForwardsCatalogCommands(t *testing.T) {
	repository := &customerTagRepositoryFake{}
	service, err := NewCustomerTagService(repository)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	group, err := service.CreateGroup(ctx, ports.CreateTagGroupCommand{TenantID: 11, CorpID: 22, Name: "客户等级", IdempotencyKey: "group-create-1"})
	if err != nil || group.Name != "客户等级" {
		t.Fatalf("create group = %#v, %v", group, err)
	}
	tag, err := service.CreateTag(ctx, ports.CreateCustomerTagCommand{TenantID: 11, CorpID: 22, GroupID: "g1", Name: "VIP", IdempotencyKey: "tag-create-1"})
	if err != nil || tag.GroupID != "g1" {
		t.Fatalf("create tag = %#v, %v", tag, err)
	}
	if _, err := service.MoveTag(ctx, ports.MoveCustomerTagCommand{TenantID: 11, CorpID: 22, TagID: "t1", GroupID: "g2", Version: 2, IdempotencyKey: "tag-move-1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.MaintainContacts(ctx, ports.MaintainTagContactsCommand{TenantID: 11, CorpID: 22, TagID: "t1", AddContactIDs: []string{"c1", "c2"}, RemoveContactIDs: []string{"c3"}, Version: 3, IdempotencyKey: "tag-contacts-1"}); err != nil {
		t.Fatal(err)
	}
	deleted, err := service.DeleteTag(ctx, ports.DeleteCustomerTagCommand{TenantID: 11, CorpID: 22, TagID: "t1", Version: 4, IdempotencyKey: "tag-delete-1"})
	if err != nil || deleted.AffectedResourceCount != 2 {
		t.Fatalf("delete result = %#v, %v", deleted, err)
	}

	for _, run := range []func() error{
		func() error {
			_, err := service.CreateGroup(ctx, ports.CreateTagGroupCommand{TenantID: 11, CorpID: 22, Name: " ", IdempotencyKey: "k"})
			return err
		},
		func() error {
			_, err := service.CreateTag(ctx, ports.CreateCustomerTagCommand{TenantID: 11, CorpID: 22, GroupID: "", Name: "VIP", IdempotencyKey: "k"})
			return err
		},
		func() error {
			_, err := service.MoveTag(ctx, ports.MoveCustomerTagCommand{TenantID: 11, CorpID: 22, TagID: "t1", GroupID: "g2", Version: 0, IdempotencyKey: "k"})
			return err
		},
		func() error {
			_, err := service.MaintainContacts(ctx, ports.MaintainTagContactsCommand{TenantID: 11, CorpID: 22, TagID: "t1", Version: 1, IdempotencyKey: "k"})
			return err
		},
	} {
		if err := run(); !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("invalid command error = %v", err)
		}
	}
}

type customerTagRepositoryFake struct{}

func (*customerTagRepositoryFake) ListTagCatalog(context.Context, ports.ListTagCatalogFilter) (ports.TagCatalog, error) {
	return ports.TagCatalog{}, nil
}
func (*customerTagRepositoryFake) CreateGroup(_ context.Context, command ports.CreateTagGroupCommand) (ports.TagGroup, error) {
	return ports.TagGroup{Name: command.Name}, nil
}
func (*customerTagRepositoryFake) RenameGroup(context.Context, ports.RenameTagGroupCommand) (ports.TagGroup, error) {
	return ports.TagGroup{}, nil
}
func (*customerTagRepositoryFake) CreateCustomerTag(_ context.Context, command ports.CreateCustomerTagCommand) (ports.CustomerTag, error) {
	return ports.CustomerTag{GroupID: command.GroupID, Name: command.Name}, nil
}
func (*customerTagRepositoryFake) RenameCustomerTag(context.Context, ports.RenameCustomerTagCommand) (ports.CustomerTag, error) {
	return ports.CustomerTag{}, nil
}
func (*customerTagRepositoryFake) MoveCustomerTag(context.Context, ports.MoveCustomerTagCommand) (ports.CustomerTag, error) {
	return ports.CustomerTag{}, nil
}
func (*customerTagRepositoryFake) DeleteCustomerTag(context.Context, ports.DeleteCustomerTagCommand) (ports.DeleteCustomerTagResult, error) {
	return ports.DeleteCustomerTagResult{AffectedResourceCount: 2}, nil
}
func (*customerTagRepositoryFake) MaintainTagContacts(context.Context, ports.MaintainTagContactsCommand) (ports.CustomerTag, error) {
	return ports.CustomerTag{}, nil
}
