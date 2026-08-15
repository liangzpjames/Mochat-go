package dashboard

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/wecomcapability"
)

// RoomBatchDispatchRuntimeStore is the runner-side payload/credential read
// boundary. The store resolves the dispatch TargetID to the room batch
// message payload and returns the non-secret credential binding for the
// principal's corp.
type RoomBatchDispatchRuntimeStore interface {
	RoomBatchDispatchPayload(context.Context, dashboardprincipal.DashboardPrincipal, wecomcapability.Dispatch) (RoomMessageBatchSendMessagePayload, error)
	RoomBatchDispatchCredential(context.Context, dashboardprincipal.DashboardPrincipal) (RoomWelcomeCorpCredential, bool, error)
}

type RoomBatchDispatchSender struct {
	store  RoomBatchDispatchRuntimeStore
	client RoomMessageBatchSendClient
}

type RoomBatchDispatchPoller struct {
	store  RoomBatchDispatchRuntimeStore
	client interface {
		GroupMessageTasks(context.Context, RoomWelcomeCorpCredential, string, int, string) (BatchSendGroupTaskPage, error)
		GroupMessageSendResults(context.Context, RoomWelcomeCorpCredential, string, string, int, string) (BatchSendGroupResultPage, error)
	}
}

type roomBatchDispatchPollClient interface {
	GroupMessageTasks(context.Context, RoomWelcomeCorpCredential, string, int, string) (BatchSendGroupTaskPage, error)
	GroupMessageSendResults(context.Context, RoomWelcomeCorpCredential, string, string, int, string) (BatchSendGroupResultPage, error)
}

// NewRoomBatchDispatchRunner assembles the room-only runner. Provider error
// classification is shared with the contact runner (classifyContactBatchProviderError);
// only the payload builder, credential requirement, and result target identity
// differ.
func NewRoomBatchDispatchRunner(ledger wecomcapability.DispatchLedger, authorizer wecomcapability.DispatchAuthorizer, store RoomBatchDispatchRuntimeStore, client RoomMessageBatchSendClient) *wecomcapability.DispatchRunner {
	if client == nil {
		client = NewRoomWelcomeWeComClient(defaultWeComAPIBaseURL)
	}
	var pollClient roomBatchDispatchPollClient
	if candidate, ok := client.(roomBatchDispatchPollClient); ok {
		pollClient = candidate
	}
	return wecomcapability.NewDispatchRunner(ledger, authorizer, &RoomBatchDispatchSender{store: store, client: client}, &RoomBatchDispatchPoller{store: store, client: pollClient})
}

func (s *RoomBatchDispatchSender) Submit(ctx context.Context, request wecomcapability.DispatchSubmitRequest) (wecomcapability.DispatchSubmitResult, error) {
	if s == nil || s.store == nil || s.client == nil || request.Dispatch.DispatchKind != string(wecomcapability.DispatchKindRoomBatch) {
		return wecomcapability.DispatchSubmitResult{}, wecomcapability.ErrDispatchInvalidState
	}
	payload, err := s.store.RoomBatchDispatchPayload(ctx, dashboardPrincipalFromDispatch(request.Principal), request.Dispatch)
	if err != nil {
		return wecomcapability.DispatchSubmitResult{}, err
	}
	credential, found, err := s.store.RoomBatchDispatchCredential(ctx, dashboardPrincipalFromDispatch(request.Principal))
	if err != nil {
		return wecomcapability.DispatchSubmitResult{}, err
	}
	if !found || strings.TrimSpace(credential.ContactSecret) == "" || strings.TrimSpace(credential.WXCorpID) == "" {
		return wecomcapability.DispatchSubmitResult{}, &wecomcapability.DispatchProviderError{Code: "wecom.room_credentials_missing", Category: wecomcapability.DispatchErrorContract}
	}
	result, err := s.client.SubmitRoomMessageBatchSend(ctx, credential, payload)
	if err != nil {
		return wecomcapability.DispatchSubmitResult{}, classifyContactBatchProviderError(err)
	}
	if result.ErrCode != 0 {
		return wecomcapability.DispatchSubmitResult{}, classifyContactBatchProviderError(fmt.Errorf("WECOM_API_ERROR_%d", result.ErrCode))
	}
	return wecomcapability.DispatchSubmitResult{Submitted: true, ProviderMessageID: strings.TrimSpace(result.MsgID)}, nil
}

func (p *RoomBatchDispatchPoller) Poll(ctx context.Context, request wecomcapability.DispatchPollRequest) (wecomcapability.DispatchPollResult, error) {
	if p == nil || p.store == nil || p.client == nil || request.Dispatch.DispatchKind != string(wecomcapability.DispatchKindRoomBatch) {
		return wecomcapability.DispatchPollResult{}, wecomcapability.ErrDispatchInvalidState
	}
	credential, found, err := p.store.RoomBatchDispatchCredential(ctx, dashboardPrincipalFromDispatch(request.Principal))
	if err != nil {
		return wecomcapability.DispatchPollResult{}, err
	}
	if !found || strings.TrimSpace(request.Dispatch.ProviderMessageID) == "" {
		return wecomcapability.DispatchPollResult{}, &wecomcapability.DispatchProviderError{Code: "wecom.dispatch_contract_invalid", Category: wecomcapability.DispatchErrorContract}
	}
	messageID := strings.TrimSpace(request.Dispatch.ProviderMessageID)
	results := make([]wecomcapability.DispatchResultRequest, 0)
	hasPendingTasks := false
	sawTerminalTask := false
	taskCursor := ""
	for {
		page, err := p.client.GroupMessageTasks(ctx, credential, messageID, batchSendPageLimit, taskCursor)
		if err != nil {
			return wecomcapability.DispatchPollResult{}, classifyContactBatchProviderError(err)
		}
		for _, task := range page.TaskList {
			if task.Status == batchSendMessageNotSent || strings.TrimSpace(task.UserID) == "" {
				hasPendingTasks = true
				continue
			}
			sawTerminalTask = true
			employeeID, ok := roomBatchDispatchOwnerEmployeeID(request.Dispatch.TargetID)
			if !ok {
				return wecomcapability.DispatchPollResult{}, &wecomcapability.DispatchProviderError{Code: "wecom.room_batch_dispatch_target_invalid", Category: wecomcapability.DispatchErrorContract}
			}
			if err := p.collectRoomBatchResults(ctx, credential, messageID, task.UserID, employeeID, "", &results, request); err != nil {
				return wecomcapability.DispatchPollResult{}, err
			}
		}
		if strings.TrimSpace(page.NextCursor) == "" {
			break
		}
		taskCursor = page.NextCursor
	}
	results = deduplicateContactBatchResults(results)
	if hasPendingTasks || !sawTerminalTask || len(results) == 0 {
		return wecomcapability.DispatchPollResult{Terminal: false, ProviderMessageID: messageID, Results: results}, nil
	}
	return wecomcapability.DispatchPollResult{Terminal: true, Status: dispatchStatusFromContactResults(results), ProviderMessageID: messageID, Results: results}, nil
}

func (p *RoomBatchDispatchPoller) collectRoomBatchResults(ctx context.Context, credential RoomWelcomeCorpCredential, messageID, userID string, employeeID int, cursor string, results *[]wecomcapability.DispatchResultRequest, request wecomcapability.DispatchPollRequest) error {
	page, err := p.client.GroupMessageSendResults(ctx, credential, messageID, userID, batchSendPageLimit, cursor)
	if err != nil {
		return classifyContactBatchProviderError(err)
	}
	for _, item := range page.SendList {
		if strings.TrimSpace(item.ChatID) == "" {
			continue
		}
		status := wecomcapability.DispatchSucceeded
		errorCode := ""
		if item.Status != batchSendMessageDelivered {
			status = wecomcapability.DispatchFailed
			errorCode = "wecom.room_batch_target_failed"
		}
		*results = append(*results, wecomcapability.DispatchResultRequest{
			Principal: request.Principal, TargetKind: "employee_room_chat_id",
			TargetID: roomBatchResultTargetID(employeeID, item.ChatID), Status: status,
			ProviderTargetID: item.UserID, ErrorCode: errorCode,
		})
	}
	if strings.TrimSpace(page.NextCursor) == "" {
		return nil
	}
	return p.collectRoomBatchResults(ctx, credential, messageID, userID, employeeID, page.NextCursor, results, request)
}

// roomBatchDispatchOwnerEmployeeID parses the owner employee id out of the
// dispatch TargetID "room_batch:{batchID}:owner:{employeeID}:chunk:{n}".
func roomBatchDispatchOwnerEmployeeID(targetID string) (int, bool) {
	parts := strings.Split(strings.TrimSpace(targetID), ":")
	if len(parts) != 6 || parts[0] != "room_batch" || parts[2] != "owner" || parts[4] != "chunk" {
		return 0, false
	}
	employeeID, err := strconv.Atoi(parts[3])
	return employeeID, err == nil && employeeID > 0 && parts[1] != "" && parts[5] != ""
}

func roomBatchResultTargetID(employeeID int, chatID string) string {
	return fmt.Sprintf("%d:%s", employeeID, strings.TrimSpace(chatID))
}
