package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"jiyi/mochat-go/internal/modules/scrm/domain"
	"jiyi/mochat-go/internal/modules/scrm/ports"
	nethttp "net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type OrderRepository interface {
	CreateIdempotentContext(context.Context, domain.OrderCreateCommand) (domain.OrderCreateReceipt, error)
	ListContext(context.Context, int64, int64, int, int) ([]domain.Order, int, error)
	TransitionContext(context.Context, string, int64, int64, domain.OrderStatus, int64, int64) (domain.Order, error)
}
type orderDetailRepository interface {
	GetContext(context.Context, string, int64, int64) (domain.Order, error)
	AuditContext(context.Context, string, int64, int64) ([]map[string]any, error)
}
type MemoryOrderRepository struct {
	mu       sync.Mutex
	items    map[string]domain.Order
	receipts map[string]domain.OrderCreateReceipt
}

func NewMemoryOrderRepository() *MemoryOrderRepository {
	return &MemoryOrderRepository{items: map[string]domain.Order{}, receipts: map[string]domain.OrderCreateReceipt{}}
}
func (r *MemoryOrderRepository) CreateIdempotentContext(_ context.Context, command domain.OrderCreateCommand) (domain.OrderCreateReceipt, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	scope := strconv.FormatInt(command.Order.TenantID, 10) + "\x00" + strconv.FormatInt(command.Order.CorpID, 10) + "\x00" + command.IdempotencyKey
	if receipt, ok := r.receipts[scope]; ok {
		if receipt.OrderID == "" || command.RequestHash == "" || receipt.ResponseBody == nil || receipt.ResponseStatus == 0 {
			return domain.OrderCreateReceipt{}, errors.New("incomplete order idempotency receipt")
		}
		if receipt.RequestHash != command.RequestHash {
			return domain.OrderCreateReceipt{}, domain.ErrOrderIdempotencyConflict
		}
		receipt.Replayed = true
		receipt.ResponseBody = append([]byte(nil), receipt.ResponseBody...)
		return receipt, nil
	}
	if _, ok := r.items[command.Order.ID]; ok {
		return domain.OrderCreateReceipt{}, errors.New("order already exists")
	}
	receipt := domain.OrderCreateReceipt{
		OrderID:        command.Order.ID,
		RequestHash:    command.RequestHash,
		ResponseStatus: command.ResponseStatus,
		ResponseBody:   append([]byte(nil), command.ResponseBody...),
	}
	r.items[command.Order.ID] = command.Order
	r.receipts[scope] = receipt
	return receipt, nil
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
func (r *MemoryOrderRepository) TransitionContext(_ context.Context, id string, tenantID, corpID int64, status domain.OrderStatus, version, _ int64) (domain.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	o, ok := r.items[id]
	if !ok || o.TenantID != tenantID || o.CorpID != corpID {
		return domain.Order{}, nethttp.ErrMissingFile
	}
	if err := o.Transition(status, version); err != nil {
		return domain.Order{}, err
	}
	r.items[id] = o
	return o, nil
}

type OrderHandler struct {
	repo        OrderRepository
	principal   PrincipalResolver
	authorizer  LeadAuthorizer
	idGenerator ports.IDGenerator
}

func NewOrderHandler(repo OrderRepository, deps ...any) *OrderHandler {
	h := &OrderHandler{repo: repo}
	for _, d := range deps {
		switch v := d.(type) {
		case PrincipalResolver:
			h.principal = v
		case LeadAuthorizer:
			h.authorizer = v
		case ports.IDGenerator:
			h.idGenerator = v
		}
	}
	return h
}
func (h *OrderHandler) ServeHTTP(w nethttp.ResponseWriter, r *nethttp.Request) {
	if h == nil || h.principal == nil {
		nethttp.Error(w, "principal unauthorized", nethttp.StatusUnauthorized)
		return
	}
	p, err := h.principal.Resolve(r)
	if err != nil || p.UserID <= 0 || p.TenantID <= 0 || p.CorpID <= 0 {
		nethttp.Error(w, "principal unauthorized", nethttp.StatusUnauthorized)
		return
	}
	if p.EmployeeScopeRestricted {
		nethttp.Error(w, "order owner scope cannot be resolved", nethttp.StatusForbidden)
		return
	}
	corpID := p.CorpID
	if r.Method == nethttp.MethodGet {
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
		items, total, err := h.repo.ListContext(r.Context(), p.TenantID, corpID, page, pageSize)
		if err != nil {
			nethttp.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, 200, map[string]any{"data": map[string]any{"items": items, "total": total, "page": page, "pageSize": pageSize}})
		return
	}
	if r.Method == nethttp.MethodPatch || r.Method == nethttp.MethodPut {
		h.transition(w, r, p, corpID)
		return
	}
	var in struct {
		ID            string             `json:"id"`
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
	inTenantID, inCorpID := p.TenantID, p.CorpID
	if h.authorizer != nil {
		if err := h.authorizer.Authorize(r.Context(), p, inCorpID, "/scrm/orders@add#post"); err != nil {
			nethttp.Error(w, "forbidden", 403)
			return
		}
	}
	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey == "" || len(idempotencyKey) > 128 {
		writeError(w, nethttp.StatusUnprocessableEntity, "Idempotency-Key is required and must not exceed 128 bytes")
		return
	}
	requestedID := strings.TrimSpace(in.ID)
	if strings.TrimSpace(in.ID) == "" {
		if h.idGenerator == nil {
			writeError(w, nethttp.StatusInternalServerError, "internal server error")
			return
		}
		generatedID, err := h.idGenerator.NewID()
		if err != nil || strings.TrimSpace(generatedID) == "" {
			writeError(w, nethttp.StatusInternalServerError, "internal server error")
			return
		}
		in.ID = generatedID
	}
	o, e := domain.NewOrder(domain.NewOrderInput{ID: strings.TrimSpace(in.ID), TenantID: inTenantID, CorpID: inCorpID, ContactID: in.ContactID, OpportunityID: in.OpportunityID, Title: in.Title, Note: in.Note, AmountCents: in.AmountCents, Currency: in.Currency, Status: in.Status})
	if e != nil {
		nethttp.Error(w, e.Error(), 422)
		return
	}
	requestHash, e := domain.OrderCreateRequestHash(o, requestedID)
	if e != nil {
		writeError(w, nethttp.StatusInternalServerError, "internal server error")
		return
	}
	responseBody, e := encodeOrderCreateResponse(o)
	if e != nil {
		writeError(w, nethttp.StatusInternalServerError, "internal server error")
		return
	}
	receipt, e := h.repo.CreateIdempotentContext(r.Context(), domain.OrderCreateCommand{Order: o, ActorID: p.UserID, IdempotencyKey: idempotencyKey, RequestHash: requestHash, ResponseStatus: nethttp.StatusOK, ResponseBody: responseBody})
	if e != nil {
		if errors.Is(e, domain.ErrOrderIdempotencyConflict) {
			writeError(w, nethttp.StatusConflict, "Idempotency-Key already used with a different order payload")
		} else {
			writeError(w, nethttp.StatusInternalServerError, "internal server error")
		}
		return
	}
	writeOrderCreateReceipt(w, receipt)
}

func encodeOrderCreateResponse(order domain.Order) ([]byte, error) {
	var buffer bytes.Buffer
	err := json.NewEncoder(&buffer).Encode(map[string]any{"code": nethttp.StatusOK, "msg": "success", "data": order})
	return buffer.Bytes(), err
}

func writeOrderCreateReceipt(w nethttp.ResponseWriter, receipt domain.OrderCreateReceipt) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(receipt.ResponseStatus)
	_, _ = w.Write(receipt.ResponseBody)
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
	var err error
	corp = p.CorpID
	if h.authorizer != nil {
		if err := h.authorizer.Authorize(r.Context(), p, corp, "/scrm/orders@edit#put"); err != nil {
			nethttp.Error(w, "forbidden", 403)
			return
		}
	}
	o, err := h.repo.TransitionContext(r.Context(), id, p.TenantID, corp, in.Status, in.Version, p.UserID)
	if err != nil {
		nethttp.Error(w, err.Error(), 409)
		return
	}
	writeJSON(w, 200, map[string]any{"data": o})
}
