package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
)

type OrderStatus string

const (
	OrderPending   OrderStatus = "pending"
	OrderPaid      OrderStatus = "paid"
	OrderFulfilled OrderStatus = "fulfilled"
	OrderCancelled OrderStatus = "cancelled"
)

var (
	ErrOrderVersionConflict     = errors.New("order version conflict")
	ErrOrderIdempotencyConflict = errors.New("order idempotency key reused with different payload")
)

type Order struct {
	ID            string      `json:"id"`
	TenantID      int64       `json:"tenantId"`
	CorpID        int64       `json:"corpId"`
	ContactID     string      `json:"contactId"`
	ContactName   string      `json:"contactName,omitempty"`
	OpportunityID string      `json:"opportunityId,omitempty"`
	Title         string      `json:"title"`
	Note          string      `json:"note"`
	AmountCents   int64       `json:"amountCents"`
	Currency      string      `json:"currency"`
	Status        OrderStatus `json:"status"`
	Version       int64       `json:"version"`
}

type NewOrderInput struct {
	ID                       string
	TenantID, CorpID         int64
	ContactID, OpportunityID string
	Title, Note              string
	AmountCents              int64
	Currency                 string
	Status                   OrderStatus
}

type OrderCreateCommand struct {
	Order          Order
	ActorID        int64
	IdempotencyKey string
	RequestHash    string
	ResponseStatus int
	ResponseBody   []byte
}

type OrderCreateReceipt struct {
	OrderID        string
	RequestHash    string
	ResponseStatus int
	ResponseBody   []byte
	Replayed       bool
}

func NewOrder(input NewOrderInput) (Order, error) {
	title := strings.TrimSpace(input.Title)
	note := strings.TrimSpace(input.Note)
	if input.TenantID <= 0 || input.CorpID <= 0 || strings.TrimSpace(input.ContactID) == "" || title == "" || len([]rune(title)) > 200 || len([]rune(note)) > 2000 || input.AmountCents < 0 || !validOrderStatus(input.Status) {
		return Order{}, errors.New("invalid order")
	}
	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	if currency == "" {
		currency = "CNY"
	}
	id := strings.TrimSpace(input.ID)
	if id == "" {
		id = uuid.NewString()
	}
	return Order{ID: id, TenantID: input.TenantID, CorpID: input.CorpID, ContactID: input.ContactID, OpportunityID: input.OpportunityID, Title: title, Note: note, AmountCents: input.AmountCents, Currency: currency, Status: input.Status, Version: 1}, nil
}

func OrderCreateRequestHash(order Order, requestedIDs ...string) (string, error) {
	requestedID := ""
	if len(requestedIDs) > 0 {
		requestedID = strings.TrimSpace(requestedIDs[0])
	}
	canonical := struct {
		RequestedID   string      `json:"requestedId,omitempty"`
		ContactID     string      `json:"contactId"`
		OpportunityID string      `json:"opportunityId"`
		Title         string      `json:"title"`
		Note          string      `json:"note"`
		AmountCents   int64       `json:"amountCents"`
		Currency      string      `json:"currency"`
		Status        OrderStatus `json:"status"`
	}{
		RequestedID:   requestedID,
		ContactID:     order.ContactID,
		OpportunityID: order.OpportunityID,
		Title:         strings.TrimSpace(order.Title),
		Note:          strings.TrimSpace(order.Note),
		AmountCents:   order.AmountCents,
		Currency:      strings.ToUpper(strings.TrimSpace(order.Currency)),
		Status:        order.Status,
	}
	body, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
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
