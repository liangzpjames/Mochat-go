package http

import (
	"context"
	"encoding/json"
	"fmt"
	"jiyi/mochat-go/internal/modules/scrm/domain"
	nethttp "net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type OrderRepository interface {
	Create(domain.Order) (domain.Order, error)
	List(int64, int64) []domain.Order
	Transition(string, int64, domain.OrderStatus, int64) (domain.Order, error)
}
type orderContextRepository interface {
	CreateContext(context.Context, domain.Order, int64) (domain.Order, error)
	ListContext(context.Context, int64, int64, int, int) ([]domain.Order, int, error)
	TransitionContext(context.Context, string, int64, int64, domain.OrderStatus, int64, int64) (domain.Order, error)
}
type orderDetailRepository interface {
	GetContext(context.Context, string, int64, int64) (domain.Order, error)
	AuditContext(context.Context, string, int64, int64) ([]map[string]any, error)
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
func (r *MemoryOrderRepository) ListContext(_ context.Context, t, c int64, page, pageSize int) ([]domain.Order, int, error) {
	all := r.List(t, c)
	sort.Slice(all, func(i, j int) bool { return all[i].ID > all[j].ID })
	total := len(all)
	start := (page - 1) * pageSize
	if start >= total {
		return []domain.Order{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return all[start:end], total, nil
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

type OrderHandler struct {
	repo       OrderRepository
	principal  PrincipalResolver
	authorizer LeadAuthorizer
}

func NewOrderHandler(repo OrderRepository, deps ...any) *OrderHandler {
	h := &OrderHandler{repo: repo}
	for _, d := range deps {
		switch v := d.(type) {
		case PrincipalResolver:
			h.principal = v
		case LeadAuthorizer:
			h.authorizer = v
		}
	}
	return h
}
func (h *OrderHandler) ServeHTTP(w nethttp.ResponseWriter, r *nethttp.Request) {
	var p Principal
	if h.principal != nil {
		var err error
		p, err = h.principal.Resolve(r)
		if err != nil {
			nethttp.Error(w, "principal unauthorized", 401)
			return
		}
		if p.EmployeeScopeRestricted {
			nethttp.Error(w, "order owner scope cannot be resolved", nethttp.StatusForbidden)
			return
		}
	}
	corpID := int64(0)
	if v := r.URL.Query().Get("corpId"); v != "" {
		_, _ = fmt.Sscan(v, &corpID)
	}
	if r.Method == nethttp.MethodGet {
		if corpID <= 0 {
			nethttp.Error(w, "corpId required", 400)
			return
		}
		if h.authorizer != nil {
			if err := h.authorizer.Authorize(r.Context(), p, corpID, "/scrm/orders#get"); err != nil {
				nethttp.Error(w, "forbidden", 403)
				return
			}
		}
		id := orderDetailID(r.URL.Path)
		if id != "" && id != "/" {
			if dr, ok := h.repo.(orderDetailRepository); ok {
				o, err := dr.GetContext(r.Context(), id, p.TenantID, corpID)
				if err != nil {
					nethttp.Error(w, "order not found", 404)
					return
				}
				audit, err := dr.AuditContext(r.Context(), id, p.TenantID, corpID)
				if err != nil {
					nethttp.Error(w, "internal server error", nethttp.StatusInternalServerError)
					return
				}
				writeJSON(w, 200, map[string]any{"data": map[string]any{"order": o, "audit": audit}})
				return
			}
		}
		page, pageSize := 1, 20
		if raw := r.URL.Query().Get("page"); raw != "" {
			if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
				page = parsed
			}
		}
		if raw := r.URL.Query().Get("pageSize"); raw != "" {
			if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 200 {
				pageSize = parsed
			}
		}
		var items []domain.Order
		var total int
		if cr, ok := h.repo.(orderContextRepository); ok {
			var err error
			items, total, err = cr.ListContext(r.Context(), p.TenantID, corpID, page, pageSize)
			if err != nil {
				nethttp.Error(w, err.Error(), 500)
				return
			}
		} else {
			all := h.repo.List(p.TenantID, corpID)
			total = len(all)
			start := (page - 1) * pageSize
			if start < total {
				end := start + pageSize
				if end > total {
					end = total
				}
				items = all[start:end]
			} else {
				items = []domain.Order{}
			}
		}
		writeJSON(w, 200, map[string]any{"data": map[string]any{"items": items, "total": total, "page": page, "pageSize": pageSize}})
		return
	}
	if r.Method == nethttp.MethodPatch || r.Method == nethttp.MethodPut {
		h.transition(w, r, p, corpID)
		return
	}
	if r.Method == nethttp.MethodGet {
		json.NewEncoder(w).Encode(map[string]any{"data": h.repo.List(1, 1)})
		return
	}
	var in struct {
		ID            string             `json:"id"`
		TenantID      int64              `json:"tenantId"`
		CorpID        int64              `json:"corpId"`
		ContactID     string             `json:"contactId"`
		OpportunityID string             `json:"opportunityId"`
		Title         string             `json:"title"`
		Note          string             `json:"note"`
		AmountCents   int64              `json:"amountCents"`
		Currency      string             `json:"currency"`
		Status        domain.OrderStatus `json:"status"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		nethttp.Error(w, "invalid json", 400)
		return
	}
	if p.TenantID > 0 {
		in.TenantID = p.TenantID
	}
	if in.CorpID <= 0 {
		in.CorpID = corpID
	}
	if h.authorizer != nil {
		if err := h.authorizer.Authorize(r.Context(), p, in.CorpID, "/scrm/orders@add#post"); err != nil {
			nethttp.Error(w, "forbidden", 403)
			return
		}
	}
	o, e := domain.NewOrder(domain.NewOrderInput{ID: strings.TrimSpace(in.ID), TenantID: in.TenantID, CorpID: in.CorpID, ContactID: in.ContactID, OpportunityID: in.OpportunityID, Title: in.Title, Note: in.Note, AmountCents: in.AmountCents, Currency: in.Currency, Status: in.Status})
	if e != nil {
		nethttp.Error(w, e.Error(), 422)
		return
	}
	if cr, ok := h.repo.(orderContextRepository); ok {
		o, e = cr.CreateContext(r.Context(), o, p.UserID)
	} else {
		o, e = h.repo.Create(o)
	}
	if e != nil {
		nethttp.Error(w, e.Error(), 409)
		return
	}
	writeJSON(w, nethttp.StatusOK, map[string]any{"data": o})
}

func orderDetailID(path string) string {
	const ordersPath = "/dashboard/scrm/orders"
	if path == ordersPath {
		return ""
	}
	return strings.TrimPrefix(path, ordersPath+"/")
}

func (h *OrderHandler) transition(w nethttp.ResponseWriter, r *nethttp.Request, p Principal, corp int64) {
	id := strings.TrimPrefix(r.URL.Path, "/dashboard/scrm/orders/")
	id = strings.TrimSuffix(id, "/transition")
	var in struct {
		Status  domain.OrderStatus `json:"status"`
		Version int64              `json:"version"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		nethttp.Error(w, "invalid json", 400)
		return
	}
	if corp <= 0 {
		nethttp.Error(w, "corpId required", 400)
		return
	}
	if h.authorizer != nil {
		if err := h.authorizer.Authorize(r.Context(), p, corp, "/scrm/orders@edit#put"); err != nil {
			nethttp.Error(w, "forbidden", 403)
			return
		}
	}
	var o domain.Order
	var err error
	if cr, ok := h.repo.(orderContextRepository); ok {
		o, err = cr.TransitionContext(r.Context(), id, p.TenantID, corp, in.Status, in.Version, p.UserID)
	} else {
		o, err = h.repo.Transition(id, p.TenantID, in.Status, in.Version)
	}
	if err != nil {
		nethttp.Error(w, err.Error(), 409)
		return
	}
	writeJSON(w, 200, map[string]any{"data": o})
}
