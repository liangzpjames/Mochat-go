package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"jiyi/mochat-go/internal/modules/scrm/domain"
	"jiyi/mochat-go/internal/modules/scrm/ports"
)

type CustomerTagService struct{ repository ports.CustomerTagRepository }

func NewCustomerTagService(repository ports.CustomerTagRepository) (CustomerTagService, error) {
	if isNilDependency(repository) {
		return CustomerTagService{}, fmt.Errorf("customer tag repository is required")
	}
	return CustomerTagService{repository: repository}, nil
}

func (s CustomerTagService) ListCatalog(ctx context.Context, filter ports.ListTagCatalogFilter) (ports.TagCatalog, error) {
	if filter.TenantID <= 0 || filter.CorpID <= 0 {
		return ports.TagCatalog{}, fmt.Errorf("%w: invalid tag scope", ErrInvalidArgument)
	}
	filter.GroupID, filter.Keyword = strings.TrimSpace(filter.GroupID), strings.TrimSpace(filter.Keyword)
	return s.repository.ListTagCatalog(ctx, filter)
}

func (s CustomerTagService) CreateGroup(ctx context.Context, command ports.CreateTagGroupCommand) (ports.TagGroup, error) {
	command.Name, command.IdempotencyKey = strings.TrimSpace(command.Name), strings.TrimSpace(command.IdempotencyKey)
	if !validTagScope(command.TenantID, command.CorpID, command.IdempotencyKey) || domain.ValidateTagName(command.Name) != nil {
		return ports.TagGroup{}, fmt.Errorf("%w: invalid tag group", ErrInvalidArgument)
	}
	item, err := s.repository.CreateGroup(ctx, command)
	return item, mapCustomerTagError(err)
}

func (s CustomerTagService) RenameGroup(ctx context.Context, command ports.RenameTagGroupCommand) (ports.TagGroup, error) {
	command.GroupID, command.Name, command.IdempotencyKey = strings.TrimSpace(command.GroupID), strings.TrimSpace(command.Name), strings.TrimSpace(command.IdempotencyKey)
	if !validTagMutation(command.TenantID, command.CorpID, command.Version, command.GroupID, command.IdempotencyKey) || domain.ValidateTagName(command.Name) != nil {
		return ports.TagGroup{}, fmt.Errorf("%w: invalid tag group", ErrInvalidArgument)
	}
	item, err := s.repository.RenameGroup(ctx, command)
	return item, mapCustomerTagError(err)
}

func (s CustomerTagService) CreateTag(ctx context.Context, command ports.CreateCustomerTagCommand) (ports.CustomerTag, error) {
	command.GroupID, command.Name, command.IdempotencyKey = strings.TrimSpace(command.GroupID), strings.TrimSpace(command.Name), strings.TrimSpace(command.IdempotencyKey)
	if !validTagScope(command.TenantID, command.CorpID, command.IdempotencyKey) || command.GroupID == "" || domain.ValidateTagName(command.Name) != nil {
		return ports.CustomerTag{}, fmt.Errorf("%w: invalid tag", ErrInvalidArgument)
	}
	item, err := s.repository.CreateCustomerTag(ctx, command)
	return item, mapCustomerTagError(err)
}

func (s CustomerTagService) RenameTag(ctx context.Context, command ports.RenameCustomerTagCommand) (ports.CustomerTag, error) {
	command.TagID, command.Name, command.IdempotencyKey = strings.TrimSpace(command.TagID), strings.TrimSpace(command.Name), strings.TrimSpace(command.IdempotencyKey)
	if !validTagMutation(command.TenantID, command.CorpID, command.Version, command.TagID, command.IdempotencyKey) || domain.ValidateTagName(command.Name) != nil {
		return ports.CustomerTag{}, fmt.Errorf("%w: invalid tag", ErrInvalidArgument)
	}
	item, err := s.repository.RenameCustomerTag(ctx, command)
	return item, mapCustomerTagError(err)
}

func (s CustomerTagService) MoveTag(ctx context.Context, command ports.MoveCustomerTagCommand) (ports.CustomerTag, error) {
	command.TagID, command.GroupID, command.IdempotencyKey = strings.TrimSpace(command.TagID), strings.TrimSpace(command.GroupID), strings.TrimSpace(command.IdempotencyKey)
	if !validTagMutation(command.TenantID, command.CorpID, command.Version, command.TagID, command.IdempotencyKey) || command.GroupID == "" {
		return ports.CustomerTag{}, fmt.Errorf("%w: invalid tag move", ErrInvalidArgument)
	}
	item, err := s.repository.MoveCustomerTag(ctx, command)
	return item, mapCustomerTagError(err)
}

func (s CustomerTagService) DeleteTag(ctx context.Context, command ports.DeleteCustomerTagCommand) (ports.DeleteCustomerTagResult, error) {
	command.TagID, command.IdempotencyKey = strings.TrimSpace(command.TagID), strings.TrimSpace(command.IdempotencyKey)
	if !validTagMutation(command.TenantID, command.CorpID, command.Version, command.TagID, command.IdempotencyKey) {
		return ports.DeleteCustomerTagResult{}, fmt.Errorf("%w: invalid tag delete", ErrInvalidArgument)
	}
	item, err := s.repository.DeleteCustomerTag(ctx, command)
	return item, mapCustomerTagError(err)
}

func (s CustomerTagService) PreviewDeleteTag(ctx context.Context, query ports.PreviewCustomerTagDeleteQuery) (ports.DeleteCustomerTagPreview, error) {
	query.TagID = strings.TrimSpace(query.TagID)
	if query.TenantID <= 0 || query.CorpID <= 0 || query.TagID == "" {
		return ports.DeleteCustomerTagPreview{}, fmt.Errorf("%w: invalid tag delete preview", ErrInvalidArgument)
	}
	item, err := s.repository.PreviewDeleteCustomerTag(ctx, query)
	return item, mapCustomerTagError(err)
}

func (s CustomerTagService) MaintainContacts(ctx context.Context, command ports.MaintainTagContactsCommand) (ports.CustomerTag, error) {
	command.TagID, command.IdempotencyKey = strings.TrimSpace(command.TagID), strings.TrimSpace(command.IdempotencyKey)
	command.AddContactIDs, command.RemoveContactIDs = normalizedStrings(command.AddContactIDs), normalizedStrings(command.RemoveContactIDs)
	if !validTagMutation(command.TenantID, command.CorpID, command.Version, command.TagID, command.IdempotencyKey) || len(command.AddContactIDs)+len(command.RemoveContactIDs) == 0 {
		return ports.CustomerTag{}, fmt.Errorf("%w: invalid tag contacts", ErrInvalidArgument)
	}
	item, err := s.repository.MaintainTagContacts(ctx, command)
	return item, mapCustomerTagError(err)
}

func validTagScope(tenantID, corpID int64, key string) bool {
	return tenantID > 0 && corpID > 0 && key != ""
}
func validTagMutation(tenantID, corpID, version int64, id, key string) bool {
	return validTagScope(tenantID, corpID, key) && version > 0 && id != ""
}

func normalizedStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func mapCustomerTagError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, ports.ErrDuplicateTagName):
		return fmt.Errorf("%w: %v", ErrInvalidArgument, err)
	case errors.Is(err, ports.ErrAssignmentConflict):
		return fmt.Errorf("%w: %v", ports.ErrAssignmentConflict, err)
	case errors.Is(err, ports.ErrTagNotFound), errors.Is(err, ports.ErrTagGroupNotFound), errors.Is(err, ports.ErrContactNotFound):
		return fmt.Errorf("%w: %v", ErrNotFound, err)
	default:
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
}
