package domain

import (
	"fmt"
	"strings"
	"time"
)

const (
	LeadSourceManual LeadSource = "manual"
	LeadSourceImport LeadSource = "import"
	LeadSourceWeCom  LeadSource = "wecom"

	LeadStatusNew LeadStatus = "new"
)

type LeadSource string

type LeadStatus string

type LeadName struct {
	value string
}

func (lead *Lead) TransitionTo(status LeadStatus, now time.Time) error {
	allowed := (lead.Status == LeadStatusNew && (status == LeadStatusQualified || status == LeadStatusDiscarded)) ||
		(lead.Status == LeadStatusQualified && (status == LeadStatusConverted || status == LeadStatusDiscarded))
	if !allowed {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidLeadTransition, lead.Status, status)
	}
	lead.Status = status
	lead.Version++
	lead.UpdatedAt = now.UTC()
	return nil
}

func (n LeadName) String() string {
	return n.value
}

type Lead struct {
	ID                 string
	TenantID           int64
	CorpID             int64
	BusinessKey        string
	Name               LeadName
	Phone              string
	Source             LeadSource
	Status             LeadStatus
	OwnerID            *int64
	ConvertedContactID string
	DiscardReason      string
	Version            int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func NewLead(id string, tenantID int64, businessKey, rawName string, source LeadSource, now time.Time) (Lead, error) {
	return newLead(id, tenantID, 0, businessKey, rawName, "", source, now)
}

func NewCorpLead(id string, tenantID, corpID int64, businessKey, rawName, rawPhone string, source LeadSource, now time.Time) (Lead, error) {
	if corpID <= 0 {
		return Lead{}, fmt.Errorf("%w: %d", ErrInvalidCorpID, corpID)
	}
	return newLead(id, tenantID, corpID, businessKey, rawName, rawPhone, source, now)
}

func newLead(id string, tenantID, corpID int64, businessKey, rawName, rawPhone string, source LeadSource, now time.Time) (Lead, error) {
	if tenantID <= 0 {
		return Lead{}, fmt.Errorf("%w: %d", ErrInvalidTenantID, tenantID)
	}
	if strings.TrimSpace(businessKey) == "" || len(businessKey) > 128 {
		return Lead{}, fmt.Errorf("%w", ErrInvalidBusinessKey)
	}

	name, err := newLeadName(rawName)
	if err != nil {
		return Lead{}, err
	}
	if !source.valid() {
		return Lead{}, fmt.Errorf("%w: %q", ErrUnsupportedLeadSource, source)
	}
	phone := strings.TrimSpace(rawPhone)
	if len(phone) > 64 {
		return Lead{}, fmt.Errorf("%w", ErrInvalidLeadPhone)
	}

	now = now.UTC()
	return Lead{
		ID:          id,
		TenantID:    tenantID,
		CorpID:      corpID,
		BusinessKey: businessKey,
		Name:        name,
		Phone:       phone,
		Source:      source,
		Status:      LeadStatusNew,
		Version:     1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func newLeadName(raw string) (LeadName, error) {
	name := strings.TrimSpace(raw)
	if name == "" || len(name) > 200 {
		return LeadName{}, fmt.Errorf("%w", ErrInvalidLeadName)
	}
	return LeadName{value: name}, nil
}

func (s LeadSource) valid() bool {
	switch s {
	case LeadSourceManual, LeadSourceImport, LeadSourceWeCom:
		return true
	default:
		return false
	}
}
