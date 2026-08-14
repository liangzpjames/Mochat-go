package wecomcapability

import "time"

type CredentialGroup string

const (
	CredentialGroupEmployee CredentialGroup = "employee"
	CredentialGroupContact  CredentialGroup = "contact"
	CredentialGroupAgent    CredentialGroup = "agent"
	CredentialGroupCallback CredentialGroup = "callback"
)

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
	ID                int64
	TenantID          int
	CorpID            int
	Capability        string
	Action            string
	CredentialGroup   CredentialGroup
	Status            string
	IdempotencyKey    string
	ProviderRequestID string
	ProviderObjectID  string
	ExternalSuccess   bool
	CallbackEvidence  bool
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

func CredentialGroupForCapability(capability string) CredentialGroup {
	switch capability {
	case EmployeeSync, DepartmentSync:
		return CredentialGroupEmployee
	case ExternalContactSync, ContactTagSync, RoomSync, ContactWay, WelcomeMessage, ContactTransfer, ContactBatchSend, RoomBatchSend:
		return CredentialGroupContact
	case AgentMessage:
		return CredentialGroupAgent
	case Callback:
		return CredentialGroupCallback
	default:
		return ""
	}
}

func IsValidOperationAction(action string) bool {
	switch action {
	case "sync", "pull", "send", "create", "update", "transfer", "receive", "verify", "poll", "retry":
		return true
	default:
		return false
	}
}

func IsValidOperationActionForCapability(capability, action string) bool {
	if !IsValidOperationAction(action) {
		return false
	}
	if capability == Callback {
		return action == "receive" || action == "verify"
	}
	if capability == EmployeeSync || capability == DepartmentSync {
		return action == "sync" || action == "pull" || action == "retry"
	}
	return true
}

// IsCurrentOperationEvidence applies the second, application-side evidence
// check after a tenant-scoped store lookup. It rejects malformed success rows
// instead of allowing a status row to self-certify readiness.
func IsCurrentOperationEvidence(operation Operation, tenantID, corpID int, capability string, expectedVersion uint64) bool {
	if operation.ID <= 0 || operation.TenantID != tenantID || operation.CorpID != corpID || operation.Capability != capability || CredentialGroupForCapability(capability) == "" || operation.CredentialGroup != CredentialGroupForCapability(capability) || expectedVersion == 0 || operation.CredentialVersion != expectedVersion || !IsValidOperationActionForCapability(capability, operation.Action) || !IsValidOperationStatus(operation.Status) {
		return false
	}
	if operation.TargetTotal < 0 || operation.SuccessTotal < 0 || operation.FailureTotal < 0 || operation.SuccessTotal+operation.FailureTotal > operation.TargetTotal {
		return false
	}
	if operation.Status == OperationSucceeded {
		if operation.FinishedAt == nil {
			return false
		}
		switch capability {
		case ContactBatchSend, RoomBatchSend:
			return operation.ProviderRequestID != "" && operation.TargetTotal > 0 && operation.SuccessTotal == operation.TargetTotal && operation.FailureTotal == 0
		case AgentMessage, ContactWay, WelcomeMessage, ContactTransfer:
			return operation.ProviderObjectID != "" || operation.ProviderRequestID != ""
		case Callback:
			return operation.CallbackEvidence && operation.SuccessTotal == 1 && operation.TargetTotal == 1
		default:
			return operation.ExternalSuccess && operation.SuccessTotal+operation.FailureTotal == operation.TargetTotal
		}
	}
	if operation.Status == OperationPartialFailed || operation.Status == OperationFailed {
		if operation.Status == OperationPartialFailed && operation.SuccessTotal+operation.FailureTotal != operation.TargetTotal {
			return false
		}
		return operation.FinishedAt != nil && operation.ErrorCode != ""
	}
	return true
}

func IsValidOperationStatus(status string) bool {
	switch status {
	case OperationPending, OperationClaimed, OperationSubmitting, OperationSubmitted,
		OperationPolling, OperationSucceeded, OperationPartialFailed, OperationFailed,
		OperationCancelled:
		return true
	default:
		return false
	}
}

func IsValidDispatchStatus(status string) bool {
	switch status {
	case DispatchQueued, DispatchClaimed, DispatchSubmitting, DispatchSubmitted,
		DispatchPolling, DispatchSucceeded, DispatchPartialFailed, DispatchFailed:
		return true
	default:
		return false
	}
}

func CanTransitionOperation(from, to string) bool {
	if !IsValidOperationStatus(from) || !IsValidOperationStatus(to) || from == to {
		return false
	}
	switch from {
	case OperationPending:
		return to == OperationClaimed || to == OperationCancelled
	case OperationClaimed:
		return to == OperationSubmitting || to == OperationCancelled || to == OperationFailed
	case OperationSubmitting:
		return to == OperationSubmitted || to == OperationFailed
	case OperationSubmitted:
		return to == OperationPolling || to == OperationFailed
	case OperationPolling:
		return to == OperationPolling || to == OperationSucceeded || to == OperationPartialFailed || to == OperationFailed || to == OperationCancelled
	default:
		return false
	}
}
