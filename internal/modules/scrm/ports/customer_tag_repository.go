package ports

import (
	"context"
	"errors"
)

var (
	ErrTagGroupNotFound = errors.New("tag group not found")
	ErrDuplicateTagName = errors.New("duplicate tag name")
)

type TagGroup struct {
	ID, Name          string
	TenantID, CorpID  int64
	Version, TagCount int64
}

type CustomerTag struct {
	ID, GroupID, Name   string
	TenantID, CorpID    int64
	Version, UsageCount int64
}

type TagCatalog struct {
	Groups []TagGroup
	Tags   []CustomerTag
}

type ListTagCatalogFilter struct {
	TenantID, CorpID int64
	GroupID, Keyword string
}

type CreateTagGroupCommand struct {
	TenantID, CorpID     int64
	Name, IdempotencyKey string
}

type RenameTagGroupCommand struct {
	TenantID, CorpID, Version     int64
	GroupID, Name, IdempotencyKey string
}

type CreateCustomerTagCommand struct {
	TenantID, CorpID              int64
	GroupID, Name, IdempotencyKey string
}

type RenameCustomerTagCommand struct {
	TenantID, CorpID, Version   int64
	TagID, Name, IdempotencyKey string
}

type MoveCustomerTagCommand struct {
	TenantID, CorpID, Version      int64
	TagID, GroupID, IdempotencyKey string
}

type DeleteCustomerTagCommand struct {
	TenantID, CorpID, Version int64
	TagID, IdempotencyKey     string
}

type DeleteCustomerTagResult struct {
	AffectedResourceCount int64
}

type MaintainTagContactsCommand struct {
	TenantID, CorpID, Version       int64
	TagID, IdempotencyKey           string
	AddContactIDs, RemoveContactIDs []string
}

type CustomerTagRepository interface {
	ListTagCatalog(context.Context, ListTagCatalogFilter) (TagCatalog, error)
	CreateGroup(context.Context, CreateTagGroupCommand) (TagGroup, error)
	RenameGroup(context.Context, RenameTagGroupCommand) (TagGroup, error)
	CreateCustomerTag(context.Context, CreateCustomerTagCommand) (CustomerTag, error)
	RenameCustomerTag(context.Context, RenameCustomerTagCommand) (CustomerTag, error)
	MoveCustomerTag(context.Context, MoveCustomerTagCommand) (CustomerTag, error)
	DeleteCustomerTag(context.Context, DeleteCustomerTagCommand) (DeleteCustomerTagResult, error)
	MaintainTagContacts(context.Context, MaintainTagContactsCommand) (CustomerTag, error)
}
