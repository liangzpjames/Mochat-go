package domain

import (
	"errors"
	"strings"
)

type OrderStatus string

const (
	OrderPending   OrderStatus = "pending"
	OrderPaid      OrderStatus = "paid"
	OrderFulfilled OrderStatus = "fulfilled"
	OrderCancelled OrderStatus = "cancelled"
)

var ErrOrderVersionConflict = errors.New("order version conflict")

type Order struct {
	ID            string      `json:"id"`
	TenantID      int64       `json:"tenantId"`
	CorpID        int64       `json:"corpId"`
	ContactID     string      `json:"contactId"`
	OpportunityID string      `json:"opportunityId,omitempty"`
	AmountCents   int64       `json:"amountCents"`
	Currency      string      `json:"currency"`
	Status        OrderStatus `json:"status"`
	Version       int64       `json:"version"`
}

type NewOrderInput struct {
	ID                       string
	TenantID, CorpID         int64
	ContactID, OpportunityID string
	AmountCents              int64
	Currency                 string
	Status                   OrderStatus
}

func NewOrder(input NewOrderInput) (Order, error) {
	if strings.TrimSpace(input.ID) == "" || input.TenantID <= 0 || input.CorpID <= 0 || strings.TrimSpace(input.ContactID) == "" || input.AmountCents < 0 || !validOrderStatus(input.Status) {
		return Order{}, errors.New("invalid order")
	}
	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	if currency == "" {
		currency = "CNY"
	}
	return Order{ID: input.ID, TenantID: input.TenantID, CorpID: input.CorpID, ContactID: input.ContactID, OpportunityID: input.OpportunityID, AmountCents: input.AmountCents, Currency: currency, Status: input.Status, Version: 1}, nil
}

func (order *Order) Transition(status OrderStatus, version int64) error {
	if order == nil || order.Version != version {
		return ErrOrderVersionConflict
	}
	allowed := (order.Status == OrderPending && (status == OrderPaid || status == OrderCancelled)) || (order.Status == OrderPaid && (status == OrderFulfilled || status == OrderCancelled))
	if !allowed {
		return errors.New("invalid order transition")
	}
	order.Status = status
	order.Version++
	return nil
}

func validOrderStatus(status OrderStatus) bool {
	return status == OrderPending || status == OrderPaid || status == OrderFulfilled || status == OrderCancelled
}
