package application

import (
	"context"
	"errors"
	"testing"

	"jiyi/mochat-go/internal/modules/scrm/ports"
)

func TestOpportunityServiceForwardsCompleteStageCommand(t *testing.T) {
	repository := &opportunityRepositoryFake{}
	service, err := NewOpportunityService(repository, tagRepositoryFake{})
	if err != nil {
		t.Fatal(err)
	}
	command := ports.ChangeOpportunityStageCommand{
		TenantID: 11, CorpID: 22, OpportunityID: "opportunity-1", StageID: "lost",
		Version: 3, LostReason: "预算取消", IdempotencyKey: "stage-1",
	}
	if _, err := service.ChangeOpportunityStage(context.Background(), command); err != nil {
		t.Fatal(err)
	}
	if repository.stageCommand != command {
		t.Fatalf("stage command = %#v, want %#v", repository.stageCommand, command)
	}
}

func TestOpportunityServiceRejectsInvalidCreateAndStageInputs(t *testing.T) {
	repository := &opportunityRepositoryFake{}
	service, err := NewOpportunityService(repository, tagRepositoryFake{})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		run  func() error
	}{
		{name: "negative amount", run: func() error {
			_, err := service.CreateOpportunity(context.Background(), ports.CreateOpportunityCommand{TenantID: 1, CorpID: 2, ContactID: "c1", Stage: "proposal", Amount: -1, StartDate: "2026-08-02", EndDate: "2026-08-03", IdempotencyKey: "create-1"})
			return err
		}},
		{name: "reversed dates", run: func() error {
			_, err := service.CreateOpportunity(context.Background(), ports.CreateOpportunityCommand{TenantID: 1, CorpID: 2, ContactID: "c1", Stage: "proposal", Amount: 1, StartDate: "2026-08-03", EndDate: "2026-08-02", IdempotencyKey: "create-2"})
			return err
		}},
		{name: "blank stage", run: func() error {
			_, err := service.CreateOpportunity(context.Background(), ports.CreateOpportunityCommand{TenantID: 1, CorpID: 2, ContactID: "c1", Amount: 1, StartDate: "2026-08-02", EndDate: "2026-08-03", IdempotencyKey: "create-3"})
			return err
		}},
		{name: "lost without reason", run: func() error {
			_, err := service.ChangeOpportunityStage(context.Background(), ports.ChangeOpportunityStageCommand{TenantID: 1, CorpID: 2, OpportunityID: "o1", StageID: "lost", Version: 1, IdempotencyKey: "stage-1"})
			return err
		}},
		{name: "blank target stage", run: func() error {
			_, err := service.ChangeOpportunityStage(context.Background(), ports.ChangeOpportunityStageCommand{TenantID: 1, CorpID: 2, OpportunityID: "o1", Version: 1, IdempotencyKey: "stage-2"})
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.run(); !errors.Is(err, ErrInvalidArgument) {
				t.Fatalf("error = %v, want invalid argument", err)
			}
		})
	}
	if repository.calls != 0 {
		t.Fatalf("repository called %d times for invalid commands", repository.calls)
	}
}

type opportunityRepositoryFake struct {
	calls        int
	stageCommand ports.ChangeOpportunityStageCommand
}

func (r *opportunityRepositoryFake) ListOpportunities(context.Context, ports.OpportunityFilter) (ports.OpportunityPage, error) {
	r.calls++
	return ports.OpportunityPage{}, nil
}
func (r *opportunityRepositoryFake) CreateOpportunity(context.Context, ports.CreateOpportunityCommand) (ports.Opportunity, error) {
	r.calls++
	return ports.Opportunity{}, nil
}
func (r *opportunityRepositoryFake) ChangeOpportunityStage(_ context.Context, command ports.ChangeOpportunityStageCommand) (ports.Opportunity, error) {
	r.calls++
	r.stageCommand = command
	return ports.Opportunity{ID: command.OpportunityID}, nil
}
func (r *opportunityRepositoryFake) ListFollowUps(context.Context, int64, int64, string) ([]ports.FollowUpRecord, error) {
	r.calls++
	return nil, nil
}
func (r *opportunityRepositoryFake) AppendFollowUp(context.Context, ports.AppendFollowUpCommand) (ports.FollowUpRecord, error) {
	r.calls++
	return ports.FollowUpRecord{}, nil
}

type tagRepositoryFake struct{}

func (tagRepositoryFake) ListTags(context.Context, int64, int64) ([]ports.Tag, error) {
	return nil, nil
}
func (tagRepositoryFake) CreateTag(context.Context, int64, int64, string, string) (ports.Tag, error) {
	return ports.Tag{}, nil
}
func (tagRepositoryFake) RenameTag(context.Context, int64, int64, string, string, int64, string) (ports.Tag, error) {
	return ports.Tag{}, nil
}
func (tagRepositoryFake) BindTags(context.Context, int64, int64, string, []string, string) error {
	return nil
}
