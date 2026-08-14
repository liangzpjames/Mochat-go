package wecomcapability

import "time"

const (
	EmployeeSync        = "employee_sync"
	DepartmentSync      = "department_sync"
	ExternalContactSync = "external_contact_sync"
	ContactTagSync      = "contact_tag_sync"
	RoomSync            = "room_sync"
	ContactWay          = "contact_way"
	WelcomeMessage      = "welcome_message"
	ContactTransfer     = "contact_transfer"
	AgentMessage        = "agent_message"
	ContactBatchSend    = "contact_batch_send"
	RoomBatchSend       = "room_batch_send"
	Callback            = "callback"
)

var All = []string{
	EmployeeSync, DepartmentSync, ExternalContactSync, ContactTagSync,
	RoomSync, ContactWay, WelcomeMessage, ContactTransfer, AgentMessage,
	ContactBatchSend, RoomBatchSend, Callback,
}

const (
	OperationPending       = "pending"
	OperationClaimed       = "claimed"
	OperationSubmitting    = "submitting"
	OperationSubmitted     = "submitted"
	OperationPolling       = "polling"
	OperationSucceeded     = "succeeded"
	OperationPartialFailed = "partial_failed"
	OperationFailed        = "failed"
	OperationCancelled     = "cancelled"

	DispatchQueued        = "queued"
	DispatchClaimed       = "claimed"
	DispatchSubmitting    = "submitting"
	DispatchSubmitted     = "submitted"
	DispatchPolling       = "polling"
	DispatchSucceeded     = "succeeded"
	DispatchPartialFailed = "partial_failed"
	DispatchFailed        = "failed"
)

// Operation is the non-secret latest evidence used by provider status and
// readiness checks. It intentionally carries no request payload or credential.
type Operation struct {
	Capability        string
	Action            string
	Status            string
	IdempotencyKey    string
	ProviderRequestID string
	TargetTotal       int
	SuccessTotal      int
	FailureTotal      int
	ErrorCode         string
	ActorUserID       int
	RequestedAt       *time.Time
	StartedAt         *time.Time
	FinishedAt        *time.Time
	UpdatedAt         *time.Time
	Attempt           int
	LeaseExpiresAt    *time.Time
	// CredentialVersion is the non-secret company binding version captured when
	// the operation was created. A zero value is never current evidence.
	CredentialVersion uint64
}
