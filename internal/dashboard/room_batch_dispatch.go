package dashboard

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/wecomcapability"
)

const (
	roomBatchDispatchCapability = wecomcapability.RoomBatchSend
	// The store enforces the acquisition precise-group-send page permission for
	// the durable room create (see MySQLStore.CreateRoomBatchDispatch), so the
	// room durable path rides the same page code as the contact durable path.
	roomBatchDispatchPageCode = contactBatchDispatchPageCode
)

// Room batch reuses the contact batch error taxonomy: the store layer returns
// the contact error values (ErrContactBatchTenantDenied and friends), so the
// room variables are aliases of the same sentinel values. Only the machine
// codes and user-facing messages are room-specific.
var (
	ErrRoomBatchBodyScope         = ErrContactBatchBodyScope
	ErrRoomBatchConflict          = ErrContactBatchConflict
	ErrRoomBatchCapabilityLimited = ErrContactBatchCapabilityLimited
	ErrRoomBatchTargetNotOwned    = ErrContactBatchTargetNotOwned
	ErrRoomBatchTenantDenied      = ErrContactBatchTenantDenied
	ErrRoomBatchPermissionDenied  = ErrContactBatchPermissionDenied
	ErrRoomBatchNotFound          = ErrContactBatchNotFound
	ErrRoomBatchQuotaExceeded     = ErrContactBatchQuotaExceeded
	ErrRoomBatchIdentityField     = ErrContactBatchIdentityField
)

// RoomBatchDispatchInput is the authenticated, explicit-target room create
// contract. Tenant/corp/actor are deliberately absent; they come from the
// DashboardPrincipal and DashboardAccessContext.
type RoomBatchDispatchInput struct {
	Batch                 RoomMessageBatchSendWrite
	RoomTargets           []RoomBatchTarget
	SenderOwnerEmployeeID int
	IdempotencyKey        string
	RequestID             string
}

// RoomBatchTarget is the only client-visible target identity for durable room
// sends. The store resolves RoomID to a WeCom chat_id inside the tenant/corp
// transaction and verifies the room-owner relation.
type RoomBatchTarget struct {
	OwnerEmployeeID int `json:"ownerEmployeeId"`
	RoomID          int `json:"roomId"`
}

type RoomBatchDispatchResult struct {
	OperationID int64
	BatchID     int64
	Status      string
	Duplicate   bool
}

// RoomBatchDispatchStore is the durable create boundary. Implementations must
// perform business-row, operation, dispatch, and create audit/event mutations
// in one transaction and re-check ownership/generation there.
type RoomBatchDispatchStore interface {
	CreateRoomBatchDispatch(context.Context, dashboardprincipal.DashboardPrincipal, DashboardAccessContext, RoomBatchDispatchInput) (RoomBatchDispatchResult, error)
}

// RoomBatchDispatchScopedReadStore is the principal-scoped single-batch read
// used by loadOwnedBatch when the durable store is bound. A store that does
// not implement it falls back to the legacy UserID-ownership read.
type RoomBatchDispatchScopedReadStore interface {
	RoomMessageBatchSendByIDForPrincipal(context.Context, dashboardprincipal.DashboardPrincipal, int) (RoomMessageBatchSendItem, bool, error)
}

type RoomBatchDurableView struct {
	Batch      RoomMessageBatchSendItem
	Operation  wecomcapability.Operation
	Dispatches []wecomcapability.Dispatch
	Results    []wecomcapability.OperationResult
}

// RoomBatchDispatchDurableReadStore is the minimal read projection the
// dashboard pages need: a per-batch durable view (Index/Show) plus the owner
// and room-receive pages. The store agent implements these exact names.
type RoomBatchDispatchDurableReadStore interface {
	RoomBatchDurableView(context.Context, dashboardprincipal.DashboardPrincipal, int) (RoomBatchDurableView, bool, error)
	RoomBatchDurableOwnerPage(context.Context, dashboardprincipal.DashboardPrincipal, int, RoomMessageBatchSendOwnerFilter) (RoomMessageBatchSendOwnerPage, error)
	RoomBatchDurableReceivePage(context.Context, dashboardprincipal.DashboardPrincipal, int, RoomMessageBatchSendRoomFilter) (RoomMessageBatchSendRoomPage, error)
}

func roomBatchDispatchStoreFrom(store RoomMessageBatchSendStore) RoomBatchDispatchStore {
	if durable, ok := store.(RoomBatchDispatchStore); ok {
		return durable
	}
	return nil
}

// NewRoomMessageBatchSendHandlerWithDispatch makes the durable room path
// explicit for composition/tests while preserving the existing constructor.
func NewRoomMessageBatchSendHandlerWithDispatch(store RoomMessageBatchSendStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer, apiBaseURL string, fileStorageRoot string, client RoomMessageBatchSendClient) *RoomMessageBatchSendHandler {
	handler := NewRoomMessageBatchSendHandler(store, cache, resolver, authorizer, apiBaseURL, fileStorageRoot, client)
	handler.durableRequired = true
	return handler
}

func writeRoomBatchDurableError(w http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		writeMachineEnvelope(w, http.StatusNotFound, "ROOM_BATCH_NOT_FOUND", "room batch not found", nil)
		return
	}
	code, status := roomBatchDispatchHTTPError(err)
	writeMachineEnvelope(w, status, code, roomBatchDispatchMessage(code), nil)
}

func (h *RoomMessageBatchSendHandler) storeDurableRoomBatch(w http.ResponseWriter, r *http.Request, user User, principal dashboardprincipal.DashboardPrincipal, access DashboardAccessContext, corpID int) bool {
	body, err := decodeRoomBatchDispatchBody(r)
	if err != nil {
		writeMachineEnvelope(w, http.StatusBadRequest, "ROOM_BATCH_INVALID_REQUEST", "invalid request body", nil)
		return true
	}
	input, err := h.roomBatchDispatchInputFromBody(body, user, corpID, access)
	if err != nil {
		code, status := roomBatchDispatchHTTPError(err)
		writeMachineEnvelope(w, status, code, roomBatchDispatchMessage(code), nil)
		return true
	}
	result, err := h.durableStore.CreateRoomBatchDispatch(r.Context(), principal, access, input)
	if err != nil {
		code, status := roomBatchDispatchHTTPError(err)
		writeMachineEnvelope(w, status, code, roomBatchDispatchMessage(code), nil)
		return true
	}
	writeMachineEnvelope(w, http.StatusOK, "ROOM_BATCH_ACCEPTED", "success", map[string]any{
		"operationId": result.OperationID,
		"batchId":     result.BatchID,
		"status":      result.Status,
		"duplicate":   result.Duplicate,
	})
	return true
}

type roomBatchDispatchBody struct {
	BatchTitle            string                           `json:"batchTitle"`
	EmployeeIDs           []int                            `json:"employeeIds"`
	RoomTargets           []RoomBatchTarget                `json:"roomTargets"`
	Content               []ContactMessageBatchSendContent `json:"content"`
	SenderOwnerEmployeeID *int                             `json:"senderOwnerEmployeeId"`
	IdempotencyKey        string                           `json:"idempotencyKey"`
	RequestID             string                           `json:"requestId"`
	MediumID              int                              `json:"mediumId"`
	SendWay               int                              `json:"sendWay"`
	DefiniteTime          string                           `json:"definiteTime"`
	FilterParams          json.RawMessage                  `json:"filterParams"`
}

func decodeRoomBatchDispatchBody(r *http.Request) (roomBatchDispatchBody, error) {
	if r == nil || r.Body == nil {
		return roomBatchDispatchBody{}, ErrRoomBatchBodyScope
	}
	const maxRoomBatchBody = 1 << 20
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxRoomBatchBody+1))
	if err != nil || len(raw) > maxRoomBatchBody {
		return roomBatchDispatchBody{}, ErrRoomBatchBodyScope
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return roomBatchDispatchBody{}, ErrRoomBatchBodyScope
	}
	for key := range fields {
		switch key {
		case "tenantId", "tenant_id", "corpId", "corp_id", "actorId", "actor_id", "userId", "user_id":
			return roomBatchDispatchBody{}, ErrRoomBatchIdentityField
		}
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	var body roomBatchDispatchBody
	if err := decoder.Decode(&body); err != nil {
		return roomBatchDispatchBody{}, fmt.Errorf("invalid room batch body: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return roomBatchDispatchBody{}, ErrRoomBatchBodyScope
	}
	return body, nil
}

func (h *RoomMessageBatchSendHandler) roomBatchDispatchInputFromBody(body roomBatchDispatchBody, user User, corpID int, access DashboardAccessContext) (RoomBatchDispatchInput, error) {
	if body.SendWay != 1 && body.SendWay != 2 {
		return RoomBatchDispatchInput{}, ErrRoomBatchBodyScope
	}
	if body.SendWay == 2 && strings.TrimSpace(body.DefiniteTime) == "" {
		return RoomBatchDispatchInput{}, ErrRoomBatchBodyScope
	}
	if body.SendWay == 1 && strings.TrimSpace(body.DefiniteTime) != "" {
		return RoomBatchDispatchInput{}, ErrRoomBatchBodyScope
	}
	if strings.TrimSpace(body.DefiniteTime) != "" {
		if _, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(body.DefiniteTime), time.Local); err != nil {
			return RoomBatchDispatchInput{}, ErrRoomBatchBodyScope
		}
	}
	if !contactBatchDispatchFilterIsEmpty(body.FilterParams) || !contactBatchDispatchContentIsValid(body.Content) {
		return RoomBatchDispatchInput{}, ErrRoomBatchBodyScope
	}
	employeeIDs := uniquePositiveIntsLocal(body.EmployeeIDs)
	if len(employeeIDs) == 0 || !employeeIDsWithinDashboardScope(employeeIDs, access.AllowedEmployeeIDs) && access.ScopeRequired && access.Scope != DataScopeTenant {
		return RoomBatchDispatchInput{}, ErrRoomBatchTargetNotOwned
	}
	roomTargets := uniqueRoomBatchTargets(body.RoomTargets)
	if len(roomTargets) == 0 || !roomBatchTargetsUseEmployees(roomTargets, employeeIDs) {
		return RoomBatchDispatchInput{}, ErrRoomBatchTargetNotOwned
	}
	senderOwnerEmployeeID := employeeIDs[0]
	if body.SenderOwnerEmployeeID != nil {
		if *body.SenderOwnerEmployeeID <= 0 {
			return RoomBatchDispatchInput{}, ErrRoomBatchTargetNotOwned
		}
		senderOwnerEmployeeID = *body.SenderOwnerEmployeeID
	}
	if !validContactBatchDispatchToken(body.IdempotencyKey, 128) {
		return RoomBatchDispatchInput{}, ErrRoomBatchBodyScope
	}
	requestID := strings.TrimSpace(body.RequestID)
	if requestID == "" {
		requestID = body.IdempotencyKey
	}
	contentJSON := mustJSON(body.Content)
	return RoomBatchDispatchInput{
		Batch: RoomMessageBatchSendWrite{
			CorpID: corpID, UserID: user.ID, UserName: strings.TrimSpace(user.Name), EmployeeIDs: employeeIDs,
			BatchTitle: strings.TrimSpace(body.BatchTitle), Content: body.Content, ContentJSON: contentJSON,
			MediumID: body.MediumID, SendWay: body.SendWay, DefiniteTime: strings.TrimSpace(body.DefiniteTime), ProcessImmediately: false,
		},
		RoomTargets: roomTargets, SenderOwnerEmployeeID: senderOwnerEmployeeID, IdempotencyKey: strings.TrimSpace(body.IdempotencyKey), RequestID: requestID,
	}, nil
}

func uniqueRoomBatchTargets(values []RoomBatchTarget) []RoomBatchTarget {
	result := make([]RoomBatchTarget, 0, len(values))
	seen := make(map[[2]int]struct{}, len(values))
	for _, value := range values {
		if value.OwnerEmployeeID <= 0 || value.RoomID <= 0 {
			return nil
		}
		key := [2]int{value.OwnerEmployeeID, value.RoomID}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	return result
}

func roomBatchTargetsUseEmployees(targets []RoomBatchTarget, employeeIDs []int) bool {
	selected := make(map[int]struct{}, len(employeeIDs))
	for _, employeeID := range employeeIDs {
		selected[employeeID] = struct{}{}
	}
	for _, target := range targets {
		if _, ok := selected[target.OwnerEmployeeID]; !ok {
			return false
		}
	}
	return true
}

func roomBatchDispatchHTTPError(err error) (string, int) {
	switch {
	case errors.Is(err, ErrRoomBatchIdentityField):
		return "ROOM_BATCH_BODY_SCOPE", http.StatusBadRequest
	case errors.Is(err, ErrRoomBatchBodyScope):
		return "ROOM_BATCH_INVALID_REQUEST", http.StatusBadRequest
	case errors.Is(err, ErrRoomBatchTenantDenied):
		return "TENANT_ACCESS_DENIED", http.StatusForbidden
	case errors.Is(err, ErrRoomBatchPermissionDenied):
		return "DASHBOARD_PERMISSION_DENIED", http.StatusForbidden
	case errors.Is(err, ErrRoomBatchNotFound):
		return "ROOM_BATCH_NOT_FOUND", http.StatusNotFound
	case errors.Is(err, ErrRoomBatchQuotaExceeded):
		return "ROOM_BATCH_QUOTA_EXCEEDED", http.StatusConflict
	case errors.Is(err, ErrRoomBatchTargetNotOwned):
		return "ROOM_BATCH_SCOPE_DENIED", http.StatusForbidden
	case errors.Is(err, ErrRoomBatchCapabilityLimited):
		return "ROOM_BATCH_CAPABILITY_LIMITED", http.StatusConflict
	case errors.Is(err, ErrRoomBatchConflict):
		return "ROOM_BATCH_CONFLICT", http.StatusConflict
	default:
		return "ROOM_BATCH_STORE_UNAVAILABLE", http.StatusServiceUnavailable
	}
}

func roomBatchDispatchMessage(code string) string {
	switch code {
	case "TENANT_ACCESS_DENIED":
		return "当前企业无法使用该能力"
	case "DASHBOARD_PERMISSION_DENIED":
		return "当前账号没有该页面权限"
	case "ROOM_BATCH_NOT_FOUND":
		return "客户群群发任务不存在"
	case "ROOM_BATCH_QUOTA_EXCEEDED":
		return "客户群群发额度已用尽"
	case "ROOM_BATCH_CONFLICT":
		return "群发任务状态或幂等请求发生冲突"
	case "ROOM_BATCH_BODY_SCOPE":
		return "群发请求身份字段无效"
	case "ROOM_BATCH_INVALID_REQUEST":
		return "群发请求字段无效"
	case "ROOM_BATCH_SCOPE_DENIED":
		return "客户群群发目标不在当前权限范围"
	case "ROOM_BATCH_CAPABILITY_LIMITED":
		return "客户群群发能力当前不可用"
	default:
		return "客户群群发任务暂时不可用"
	}
}

func roomBatchOperationPayload(operation wecomcapability.Operation) map[string]any {
	return map[string]any{
		"id":                operation.ID,
		"status":            operation.Status,
		"targetTotal":       operation.TargetTotal,
		"successTotal":      operation.SuccessTotal,
		"failureTotal":      operation.FailureTotal,
		"errorCode":         operation.ErrorCode,
		"providerRequestId": operation.ProviderRequestID,
	}
}
