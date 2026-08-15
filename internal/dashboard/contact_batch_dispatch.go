package dashboard

import (
	"context"
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
	contactBatchDispatchCapability = wecomcapability.ContactBatchSend
	contactBatchDispatchPageCode   = "dashboard.acquisition.precise_group_send"
)

var (
	ErrContactBatchBodyScope                 = errors.New("contact batch request scope is invalid")
	ErrContactBatchConflict                  = errors.New("contact batch operation conflict")
	ErrContactBatchCapabilityLimited         = errors.New("contact batch capability is unavailable")
	ErrContactBatchTargetNotOwned            = errors.New("contact batch target is not owned")
	ErrContactBatchTenantDenied              = errors.New("contact batch tenant access denied")
	ErrContactBatchPermissionDenied          = errors.New("contact batch dashboard permission denied")
	ErrContactBatchNotFound                  = errors.New("contact batch not found")
	ErrContactBatchQuotaExceeded             = errors.New("contact batch quota exceeded")
	ErrContactBatchReminderReconcileRequired = errors.New("contact batch reminder reconcile required")
	ErrContactBatchReminderAlreadyCompleted  = errors.New("contact batch reminder already completed")
)

// ContactBatchDispatchInput is the authenticated, explicit-target create
// contract. Tenant/corp/actor are deliberately absent; they come from the
// DashboardPrincipal and DashboardAccessContext.
type ContactBatchDispatchInput struct {
	Batch            ContactMessageBatchSendWrite
	ContactTargets   []ContactBatchTarget
	SenderEmployeeID int
	IdempotencyKey   string
	RequestID        string
}

// ContactBatchTarget is the only client-visible target identity for durable
// contact sends. The store resolves ContactID to wx_external_userid inside
// the tenant/corp transaction and verifies the employee-contact relation.
type ContactBatchTarget struct {
	EmployeeID int `json:"employeeId"`
	ContactID  int `json:"contactId"`
}

type ContactBatchDispatchResult struct {
	OperationID int64
	BatchID     int64
	Status      string
	Duplicate   bool
}

// ContactBatchDispatchStore is the durable create boundary. Implementations
// must perform business-row, operation, dispatch, and create audit/event
// mutations in one transaction and re-check ownership/generation there.
type ContactBatchDispatchStore interface {
	CreateContactBatchDispatch(context.Context, dashboardprincipal.DashboardPrincipal, DashboardAccessContext, ContactBatchDispatchInput) (ContactBatchDispatchResult, error)
}

type ContactBatchScopedReadStore interface {
	ContactMessageBatchSendByIDForPrincipal(context.Context, dashboardprincipal.DashboardPrincipal, int) (ContactMessageBatchSendItem, bool, error)
}

type ContactBatchDurableView struct {
	Batch      ContactMessageBatchSendItem
	Operation  wecomcapability.Operation
	Dispatches []wecomcapability.Dispatch
	Results    []wecomcapability.OperationResult
}

type ContactBatchDurableReadStore interface {
	ContactBatchDurableView(context.Context, dashboardprincipal.DashboardPrincipal, int) (ContactBatchDurableView, bool, error)
	ContactBatchDurableEmployeePage(context.Context, dashboardprincipal.DashboardPrincipal, int, ContactMessageBatchSendEmployeeFilter) (ContactMessageBatchSendEmployeePage, error)
	ContactBatchDurableReceivePage(context.Context, dashboardprincipal.DashboardPrincipal, int, ContactMessageBatchSendReceiveFilter) (ContactMessageBatchSendReceivePage, error)
}

type ContactBatchDurableMutationStore interface {
	CancelContactBatchDurable(context.Context, dashboardprincipal.DashboardPrincipal, int) error
	PrepareContactBatchDurableReminder(context.Context, dashboardprincipal.DashboardPrincipal, int, int, string) (ContactBatchDurableReminder, error)
	RecordContactBatchDurableReminder(context.Context, dashboardprincipal.DashboardPrincipal, ContactBatchDurableReminder, string, bool, int, int) error
}

type ContactBatchDurableReminder struct {
	OperationID      int64
	LeaseToken       string
	Attempt          int
	IdempotencyKey   string
	Agent            RoomTagPullAgentCredential
	Recipients       []string
	CreatedAt        string
	AlreadyCompleted bool
}

func contactBatchDispatchStoreFrom(store ContactMessageBatchSendStore) ContactBatchDispatchStore {
	if durable, ok := store.(ContactBatchDispatchStore); ok {
		return durable
	}
	return nil
}

// NewContactMessageBatchSendHandlerWithDispatch makes the durable path
// explicit for composition/tests while preserving the existing constructor.
func NewContactMessageBatchSendHandlerWithDispatch(store ContactMessageBatchSendStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer, apiBaseURL string, fileStorageRoot string, client ContactMessageBatchSendClient) *ContactMessageBatchSendHandler {
	handler := NewContactMessageBatchSendHandler(store, cache, resolver, authorizer, apiBaseURL, fileStorageRoot, client)
	handler.durableRequired = true
	return handler
}

func (h *ContactMessageBatchSendHandler) storeDurableContactBatch(w http.ResponseWriter, r *http.Request, user User, principal dashboardprincipal.DashboardPrincipal, access DashboardAccessContext, corpID int) bool {
	body, err := decodeContactBatchDispatchBody(r)
	if err != nil {
		writeMachineEnvelope(w, http.StatusBadRequest, "CONTACT_BATCH_INVALID_REQUEST", "invalid request body", nil)
		return true
	}
	input, err := h.contactBatchDispatchInputFromBody(body, user, corpID, access)
	if err != nil {
		code, status := contactBatchDispatchHTTPError(err)
		writeMachineEnvelope(w, status, code, contactBatchDispatchMessage(code), nil)
		return true
	}
	result, err := h.durableStore.CreateContactBatchDispatch(r.Context(), principal, access, input)
	if err != nil {
		code, status := contactBatchDispatchHTTPError(err)
		writeMachineEnvelope(w, status, code, contactBatchDispatchMessage(code), nil)
		return true
	}
	writeMachineEnvelope(w, http.StatusOK, "CONTACT_BATCH_ACCEPTED", "success", map[string]any{
		"operationId": result.OperationID,
		"batchId":     result.BatchID,
		"status":      result.Status,
		"duplicate":   result.Duplicate,
	})
	return true
}

var ErrContactBatchIdentityField = errors.New("contact batch identity field is forbidden")

type contactBatchDispatchBody struct {
	BatchTitle       string                           `json:"batchTitle"`
	EmployeeIDs      []int                            `json:"employeeIds"`
	ContactTargets   []ContactBatchTarget             `json:"contactTargets"`
	Content          []ContactMessageBatchSendContent `json:"content"`
	SenderEmployeeID *int                             `json:"senderEmployeeId"`
	IdempotencyKey   string                           `json:"idempotencyKey"`
	RequestID        string                           `json:"requestId"`
	MediumID         int                              `json:"mediumId"`
	SendWay          int                              `json:"sendWay"`
	DefiniteTime     string                           `json:"definiteTime"`
	FilterParams     json.RawMessage                  `json:"filterParams"`
}

func decodeContactBatchDispatchBody(r *http.Request) (contactBatchDispatchBody, error) {
	if r == nil || r.Body == nil {
		return contactBatchDispatchBody{}, ErrContactBatchBodyScope
	}
	const maxContactBatchBody = 1 << 20
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxContactBatchBody+1))
	if err != nil || len(raw) > maxContactBatchBody {
		return contactBatchDispatchBody{}, ErrContactBatchBodyScope
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return contactBatchDispatchBody{}, ErrContactBatchBodyScope
	}
	for key := range fields {
		switch key {
		case "tenantId", "tenant_id", "corpId", "corp_id", "actorId", "actor_id", "userId", "user_id":
			return contactBatchDispatchBody{}, ErrContactBatchIdentityField
		}
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	var body contactBatchDispatchBody
	if err := decoder.Decode(&body); err != nil {
		return contactBatchDispatchBody{}, fmt.Errorf("invalid contact batch body: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return contactBatchDispatchBody{}, ErrContactBatchBodyScope
	}
	return body, nil
}

func (h *ContactMessageBatchSendHandler) contactBatchDispatchInputFromBody(body contactBatchDispatchBody, user User, corpID int, access DashboardAccessContext) (ContactBatchDispatchInput, error) {
	if body.SendWay != 1 && body.SendWay != 2 {
		return ContactBatchDispatchInput{}, ErrContactBatchBodyScope
	}
	if body.SendWay == 2 && strings.TrimSpace(body.DefiniteTime) == "" {
		return ContactBatchDispatchInput{}, ErrContactBatchBodyScope
	}
	if body.SendWay == 1 && strings.TrimSpace(body.DefiniteTime) != "" {
		return ContactBatchDispatchInput{}, ErrContactBatchBodyScope
	}
	if strings.TrimSpace(body.DefiniteTime) != "" {
		if _, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(body.DefiniteTime), time.Local); err != nil {
			return ContactBatchDispatchInput{}, ErrContactBatchBodyScope
		}
	}
	if !contactBatchDispatchFilterIsEmpty(body.FilterParams) || !contactBatchDispatchContentIsValid(body.Content) {
		return ContactBatchDispatchInput{}, ErrContactBatchBodyScope
	}
	employeeIDs := uniquePositiveIntsLocal(body.EmployeeIDs)
	if len(employeeIDs) == 0 || !employeeIDsWithinDashboardScope(employeeIDs, access.AllowedEmployeeIDs) && access.ScopeRequired && access.Scope != DataScopeTenant {
		return ContactBatchDispatchInput{}, ErrContactBatchTargetNotOwned
	}
	contactTargets := uniqueContactBatchTargets(body.ContactTargets)
	if len(contactTargets) == 0 || !contactBatchTargetsUseEmployees(contactTargets, employeeIDs) {
		return ContactBatchDispatchInput{}, ErrContactBatchTargetNotOwned
	}
	if len(body.Content) == 0 {
		return ContactBatchDispatchInput{}, ErrContactBatchBodyScope
	}
	senderEmployeeID := employeeIDs[0]
	if body.SenderEmployeeID != nil {
		if *body.SenderEmployeeID <= 0 {
			return ContactBatchDispatchInput{}, ErrContactBatchTargetNotOwned
		}
		senderEmployeeID = *body.SenderEmployeeID
	}
	if !validContactBatchDispatchToken(body.IdempotencyKey, 128) {
		return ContactBatchDispatchInput{}, ErrContactBatchBodyScope
	}
	requestID := strings.TrimSpace(body.RequestID)
	if requestID == "" {
		requestID = body.IdempotencyKey
	}
	filterJSON := "{}"
	if len(body.FilterParams) > 0 && string(body.FilterParams) != "null" {
		var filter map[string]any
		if err := json.Unmarshal(body.FilterParams, &filter); err != nil {
			return ContactBatchDispatchInput{}, ErrContactBatchBodyScope
		}
		filterJSON = mustJSON(filter)
	}
	contentJSON := mustJSON(body.Content)
	return ContactBatchDispatchInput{
		Batch: ContactMessageBatchSendWrite{
			CorpID: corpID, UserID: user.ID, UserName: strings.TrimSpace(user.Name), EmployeeIDs: employeeIDs,
			BatchTitle:       strings.TrimSpace(body.BatchTitle),
			FilterParamsJSON: filterJSON, FilterDetailJSON: "{}", Content: body.Content, ContentJSON: contentJSON,
			MediumID: body.MediumID, SendWay: body.SendWay, DefiniteTime: strings.TrimSpace(body.DefiniteTime), ProcessImmediately: false,
		},
		ContactTargets: contactTargets, SenderEmployeeID: senderEmployeeID, IdempotencyKey: strings.TrimSpace(body.IdempotencyKey), RequestID: requestID,
	}, nil
}

func uniqueContactBatchTargets(values []ContactBatchTarget) []ContactBatchTarget {
	result := make([]ContactBatchTarget, 0, len(values))
	seen := make(map[[2]int]struct{}, len(values))
	for _, value := range values {
		if value.EmployeeID <= 0 || value.ContactID <= 0 {
			return nil
		}
		key := [2]int{value.EmployeeID, value.ContactID}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	return result
}

func contactBatchTargetsUseEmployees(targets []ContactBatchTarget, employeeIDs []int) bool {
	selected := make(map[int]struct{}, len(employeeIDs))
	for _, employeeID := range employeeIDs {
		selected[employeeID] = struct{}{}
	}
	for _, target := range targets {
		if _, ok := selected[target.EmployeeID]; !ok {
			return false
		}
	}
	return true
}

func contactBatchDispatchFilterIsEmpty(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed == "" || trimmed == "null" || trimmed == "{}"
}

func contactBatchDispatchContentIsValid(items []ContactMessageBatchSendContent) bool {
	if len(items) == 0 {
		return false
	}
	for _, item := range items {
		if strings.ContainsAny(item.MsgType+item.Content+item.MediaID+item.PicURL+item.Title+item.Desc+item.URL+item.AppID+item.Page+item.PicMediaID, "\x00\r\n") {
			return false
		}
		switch item.MsgType {
		case "text":
			if strings.TrimSpace(item.Content) == "" || item.MediaID != "" || item.PicURL != "" || item.Title != "" || item.Desc != "" || item.URL != "" || item.AppID != "" || item.Page != "" || item.PicMediaID != "" {
				return false
			}
		case "image":
			if strings.TrimSpace(item.MediaID) == "" || item.PicURL != "" || item.Content != "" || item.Title != "" || item.Desc != "" || item.URL != "" || item.AppID != "" || item.Page != "" || item.PicMediaID != "" {
				return false
			}
		case "link":
			if strings.TrimSpace(item.Title) == "" || strings.TrimSpace(item.URL) == "" || item.Content != "" || item.MediaID != "" || item.AppID != "" || item.Page != "" || item.PicMediaID != "" {
				return false
			}
		case "miniprogram":
			if strings.TrimSpace(item.Title) == "" || strings.TrimSpace(item.PicMediaID) == "" || strings.TrimSpace(item.AppID) == "" || strings.TrimSpace(item.Page) == "" || item.Content != "" || item.MediaID != "" || item.PicURL != "" || item.Desc != "" || item.URL != "" {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func validContactBatchDispatchToken(value string, limit int) bool {
	value = strings.TrimSpace(value)
	return value != "" && len(value) <= limit && !strings.ContainsAny(value, "\x00\r\n")
}

func contactBatchDispatchHTTPError(err error) (string, int) {
	switch {
	case errors.Is(err, ErrContactBatchIdentityField):
		return "CONTACT_BATCH_BODY_SCOPE", http.StatusBadRequest
	case errors.Is(err, ErrContactBatchBodyScope):
		return "CONTACT_BATCH_INVALID_REQUEST", http.StatusBadRequest
	case errors.Is(err, ErrContactBatchTenantDenied):
		return "TENANT_ACCESS_DENIED", http.StatusForbidden
	case errors.Is(err, ErrContactBatchPermissionDenied):
		return "DASHBOARD_PERMISSION_DENIED", http.StatusForbidden
	case errors.Is(err, ErrContactBatchNotFound):
		return "CONTACT_BATCH_NOT_FOUND", http.StatusNotFound
	case errors.Is(err, ErrContactBatchQuotaExceeded):
		return "CONTACT_BATCH_QUOTA_EXCEEDED", http.StatusConflict
	case errors.Is(err, ErrContactBatchTargetNotOwned):
		return "CONTACT_BATCH_SCOPE_DENIED", http.StatusForbidden
	case errors.Is(err, ErrContactBatchCapabilityLimited):
		return "CONTACT_BATCH_CAPABILITY_LIMITED", http.StatusConflict
	case errors.Is(err, ErrContactBatchConflict):
		return "CONTACT_BATCH_CONFLICT", http.StatusConflict
	default:
		return "CONTACT_BATCH_STORE_UNAVAILABLE", http.StatusServiceUnavailable
	}
}

func contactBatchDispatchMessage(code string) string {
	switch code {
	case "TENANT_ACCESS_DENIED":
		return "当前企业无法使用该能力"
	case "DASHBOARD_PERMISSION_DENIED":
		return "当前账号没有该页面权限"
	case "CONTACT_BATCH_NOT_FOUND":
		return "客户群发任务不存在"
	case "CONTACT_BATCH_QUOTA_EXCEEDED":
		return "客户群发额度已用尽"
	case "CONTACT_BATCH_CONFLICT":
		return "群发任务状态或幂等请求发生冲突"
	case "CONTACT_BATCH_BODY_SCOPE":
		return "群发请求身份字段无效"
	case "CONTACT_BATCH_INVALID_REQUEST":
		return "群发请求字段无效"
	case "CONTACT_BATCH_SCOPE_DENIED":
		return "客户群发目标不在当前权限范围"
	case "CONTACT_BATCH_CAPABILITY_LIMITED":
		return "客户群发能力当前不可用"
	default:
		return "客户群发任务暂时不可用"
	}
}
