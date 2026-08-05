package http

import (
	"encoding/json"
	"jiyi/mochat-go/internal/modules/scrm/domain"
	nethttp "net/http"
	"strings"
	"sync"
)

type OrderRepository interface {
	Create(domain.Order) (domain.Order, error)
	List(int64, int64) []domain.Order
	Transition(string, int64, domain.OrderStatus, int64) (domain.Order, error)
}
type MemoryOrderRepository struct {
	mu    sync.Mutex
	items map[string]domain.Order
}

func NewMemoryOrderRepository() *MemoryOrderRepository {
	return &MemoryOrderRepository{items: map[string]domain.Order{}}
}
func (r *MemoryOrderRepository) Create(o domain.Order) (domain.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[o.ID]; ok {
		return r.items[o.ID], nil
	}
	r.items[o.ID] = o
	return o, nil
}
func (r *MemoryOrderRepository) List(t, c int64) []domain.Order {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []domain.Order{}
	for _, o := range r.items {
		if o.TenantID == t && o.CorpID == c {
			out = append(out, o)
		}
	}
	return out
}
func (r *MemoryOrderRepository) Transition(id string, t int64, s domain.OrderStatus, v int64) (domain.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	o, ok := r.items[id]
	if !ok {
		return domain.Order{}, nethttp.ErrMissingFile
	}
	if err := o.Transition(s, v); err != nil {
		return domain.Order{}, err
	}
	r.items[id] = o
	return o, nil
}

type OrderHandler struct{ repo OrderRepository }

func NewOrderHandler(repo OrderRepository) *OrderHandler { return &OrderHandler{repo: repo} }
func (h *OrderHandler) ServeHTTP(w nethttp.ResponseWriter, r *nethttp.Request) {
	if r.Method == nethttp.MethodGet {
		json.NewEncoder(w).Encode(map[string]any{"data": h.repo.List(1, 1)})
		return
	}
	var in struct {
		ID          string             `json:"id"`
		TenantID    int64              `json:"tenantId"`
		CorpID      int64              `json:"corpId"`
		ContactID   string             `json:"contactId"`
		AmountCents int64              `json:"amountCents"`
		Currency    string             `json:"currency"`
		Status      domain.OrderStatus `json:"status"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		nethttp.Error(w, "invalid json", 400)
		return
	}
	o, e := domain.NewOrder(domain.NewOrderInput{ID: strings.TrimSpace(in.ID), TenantID: in.TenantID, CorpID: in.CorpID, ContactID: in.ContactID, AmountCents: in.AmountCents, Currency: in.Currency, Status: in.Status})
	if e != nil {
		nethttp.Error(w, e.Error(), 422)
		return
	}
	o, e = h.repo.Create(o)
	if e != nil {
		nethttp.Error(w, e.Error(), 409)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"data": o})
}
