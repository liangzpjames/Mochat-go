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

func (n LeadName) String() string {
	return n.value
}

type Lead struct {
	ID          string
	TenantID    int64
	BusinessKey string
	Name        LeadName
	Source      LeadSource
	Status      LeadStatus
	Version     int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewLead(id string, tenantID int64, businessKey, rawName string, source LeadSource, now time.Time) (Lead, error) {
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

	now = now.UTC()
	return Lead{
		ID:          id,
		TenantID:    tenantID,
		BusinessKey: businessKey,
		Name:        name,
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
