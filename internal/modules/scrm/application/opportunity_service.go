package application

import (
	"context"
	"fmt"
	"strings"

	"jiyi/mochat-go/internal/modules/scrm/domain"
	"jiyi/mochat-go/internal/modules/scrm/ports"
)

type OpportunityService struct {
	repository ports.OpportunityRepository
	tags       ports.TagRepository
}

func NewOpportunityService(repository ports.OpportunityRepository, tags ports.TagRepository) (OpportunityService, error) {
	if isNilDependency(repository) || isNilDependency(tags) {
		return OpportunityService{}, fmt.Errorf("opportunity repositories are required")
	}
	return OpportunityService{repository: repository, tags: tags}, nil
}

func (s OpportunityService) ListOpportunities(ctx context.Context, filter ports.OpportunityFilter) ([]ports.Opportunity, error) {
	if filter.TenantID <= 0 || filter.CorpID <= 0 {
		return nil, fmt.Errorf("%w: invalid opportunity scope", ErrInvalidArgument)
	}
	return s.repository.ListOpportunities(ctx, filter)
}

func (s OpportunityService) CreateOpportunity(ctx context.Context, command ports.CreateOpportunityCommand) (ports.Opportunity, error) {
	if command.TenantID <= 0 || command.CorpID <= 0 || strings.TrimSpace(command.ContactID) == "" || strings.TrimSpace(command.IdempotencyKey) == "" {
		return ports.Opportunity{}, fmt.Errorf("%w: invalid opportunity", ErrInvalidArgument)
	}
	if err := domain.ValidateOpportunityInput(command.Amount, command.StartDate, command.EndDate); err != nil {
		return ports.Opportunity{}, fmt.Errorf("%w: %v", ErrInvalidArgument, err)
	}
	return s.repository.CreateOpportunity(ctx, command)
}

func (s OpportunityService) ChangeOpportunityStage(ctx context.Context, command ports.ChangeOpportunityStageCommand) (ports.Opportunity, error) {
	if command.TenantID <= 0 || command.CorpID <= 0 || strings.TrimSpace(command.OpportunityID) == "" || command.Version <= 0 || strings.TrimSpace(command.IdempotencyKey) == "" {
		return ports.Opportunity{}, fmt.Errorf("%w: invalid opportunity transition", ErrInvalidArgument)
	}
	if command.ToStage != domain.OpportunityStageProposal && command.ToStage != domain.OpportunityStatusWon && command.ToStage != domain.OpportunityStatusLost {
		return ports.Opportunity{}, fmt.Errorf("%w: invalid opportunity stage", ErrInvalidArgument)
	}
	if command.ToStage == domain.OpportunityStatusLost && strings.TrimSpace(command.Reason) == "" {
		return ports.Opportunity{}, fmt.Errorf("%w: lost opportunity requires a reason", ErrInvalidArgument)
	}
	return s.repository.ChangeOpportunityStage(ctx, command)
}

func (s OpportunityService) AppendFollowUp(ctx context.Context, command ports.AppendFollowUpCommand) (ports.FollowUpRecord, error) {
	if command.TenantID <= 0 || command.CorpID <= 0 || command.CreatedBy <= 0 || strings.TrimSpace(command.ContactID) == "" || strings.TrimSpace(command.IdempotencyKey) == "" {
		return ports.FollowUpRecord{}, fmt.Errorf("%w: invalid follow-up", ErrInvalidArgument)
	}
	if err := domain.ValidateFollowUp(command.Content); err != nil {
		return ports.FollowUpRecord{}, fmt.Errorf("%w: %v", ErrInvalidArgument, err)
	}
	return s.repository.AppendFollowUp(ctx, command)
}

func (s OpportunityService) ListFollowUps(ctx context.Context, tenantID, corpID int64, contactID string) ([]ports.FollowUpRecord, error) {
	if tenantID <= 0 || corpID <= 0 || strings.TrimSpace(contactID) == "" {
		return nil, fmt.Errorf("%w: invalid follow-up scope", ErrInvalidArgument)
	}
	return s.repository.ListFollowUps(ctx, tenantID, corpID, contactID)
}

func (s OpportunityService) ListTags(ctx context.Context, tenantID, corpID int64) ([]ports.Tag, error) {
	if tenantID <= 0 || corpID <= 0 {
		return nil, fmt.Errorf("%w: invalid tag scope", ErrInvalidArgument)
	}
	return s.tags.ListTags(ctx, tenantID, corpID)
}
func (s OpportunityService) CreateTag(ctx context.Context, tenantID, corpID int64, name, key string) (ports.Tag, error) {
	if tenantID <= 0 || corpID <= 0 || strings.TrimSpace(key) == "" {
		return ports.Tag{}, fmt.Errorf("%w: invalid tag", ErrInvalidArgument)
	}
	if err := domain.ValidateTagName(name); err != nil {
		return ports.Tag{}, fmt.Errorf("%w: %v", ErrInvalidArgument, err)
	}
	return s.tags.CreateTag(ctx, tenantID, corpID, name, key)
}
func (s OpportunityService) RenameTag(ctx context.Context, tenantID, corpID int64, id, name string, version int64, key string) (ports.Tag, error) {
	if strings.TrimSpace(id) == "" || version <= 0 || strings.TrimSpace(key) == "" {
		return ports.Tag{}, fmt.Errorf("%w: invalid tag", ErrInvalidArgument)
	}
	if err := domain.ValidateTagName(name); err != nil {
		return ports.Tag{}, fmt.Errorf("%w: %v", ErrInvalidArgument, err)
	}
	return s.tags.RenameTag(ctx, tenantID, corpID, id, name, version, key)
}
func (s OpportunityService) BindTags(ctx context.Context, tenantID, corpID int64, id string, contacts []string, key string) error {
	if strings.TrimSpace(id) == "" || len(contacts) == 0 || strings.TrimSpace(key) == "" {
		return fmt.Errorf("%w: invalid tag binding", ErrInvalidArgument)
	}
	return s.tags.BindTags(ctx, tenantID, corpID, id, contacts, key)
}
