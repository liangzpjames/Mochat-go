package dashboard

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/wecomcapability"
)

type ContactBatchDispatchRuntimeStore interface {
	ContactBatchDispatchPayload(context.Context, dashboardprincipal.DashboardPrincipal, wecomcapability.Dispatch) (ContactMessageBatchSendMessagePayload, error)
	ContactBatchDispatchCredential(context.Context, dashboardprincipal.DashboardPrincipal) (RoomWelcomeCorpCredential, bool, error)
}

type ContactBatchDispatchSender struct {
	store  ContactBatchDispatchRuntimeStore
	client ContactMessageBatchSendClient
}

type ContactBatchDispatchPoller struct {
	store  ContactBatchDispatchRuntimeStore
	client interface {
		GroupMessageTasks(context.Context, RoomWelcomeCorpCredential, string, int, string) (BatchSendGroupTaskPage, error)
		GroupMessageSendResults(context.Context, RoomWelcomeCorpCredential, string, string, int, string) (BatchSendGroupResultPage, error)
	}
}

type contactBatchDispatchPollClient interface {
	GroupMessageTasks(context.Context, RoomWelcomeCorpCredential, string, int, string) (BatchSendGroupTaskPage, error)
	GroupMessageSendResults(context.Context, RoomWelcomeCorpCredential, string, string, int, string) (BatchSendGroupResultPage, error)
}

func NewContactBatchDispatchRunner(ledger wecomcapability.DispatchLedger, authorizer wecomcapability.DispatchAuthorizer, store ContactBatchDispatchRuntimeStore, client ContactMessageBatchSendClient) *wecomcapability.DispatchRunner {
	if client == nil {
		client = NewRoomWelcomeWeComClient(defaultWeComAPIBaseURL)
	}
	var pollClient contactBatchDispatchPollClient
	if candidate, ok := client.(contactBatchDispatchPollClient); ok {
		pollClient = candidate
	}
	return wecomcapability.NewDispatchRunner(ledger, authorizer, &ContactBatchDispatchSender{store: store, client: client}, &ContactBatchDispatchPoller{store: store, client: pollClient})
}

func (s *ContactBatchDispatchSender) Submit(ctx context.Context, request wecomcapability.DispatchSubmitRequest) (wecomcapability.DispatchSubmitResult, error) {
	if s == nil || s.store == nil || s.client == nil || request.Dispatch.DispatchKind != string(wecomcapability.DispatchKindContactBatch) {
		return wecomcapability.DispatchSubmitResult{}, wecomcapability.ErrDispatchInvalidState
	}
	payload, err := s.store.ContactBatchDispatchPayload(ctx, dashboardPrincipalFromDispatch(request.Principal), request.Dispatch)
	if err != nil {
		return wecomcapability.DispatchSubmitResult{}, err
	}
	credential, found, err := s.store.ContactBatchDispatchCredential(ctx, dashboardPrincipalFromDispatch(request.Principal))
	if err != nil {
		return wecomcapability.DispatchSubmitResult{}, err
	}
	if !found || strings.TrimSpace(credential.ContactSecret) == "" || strings.TrimSpace(credential.WXCorpID) == "" {
		return wecomcapability.DispatchSubmitResult{}, &wecomcapability.DispatchProviderError{Code: "wecom.contact_credentials_missing", Category: wecomcapability.DispatchErrorContract}
	}
	result, err := s.client.SubmitContactMessageBatchSend(ctx, credential, payload)
	if err != nil {
		return wecomcapability.DispatchSubmitResult{}, classifyContactBatchProviderError(err)
	}
	if result.ErrCode != 0 {
		return wecomcapability.DispatchSubmitResult{}, classifyContactBatchProviderError(fmt.Errorf("WECOM_API_ERROR_%d", result.ErrCode))
	}
	return wecomcapability.DispatchSubmitResult{Submitted: true, ProviderMessageID: strings.TrimSpace(result.MsgID)}, nil
}

func (p *ContactBatchDispatchPoller) Poll(ctx context.Context, request wecomcapability.DispatchPollRequest) (wecomcapability.DispatchPollResult, error) {
	if p == nil || p.store == nil || p.client == nil || request.Dispatch.DispatchKind != string(wecomcapability.DispatchKindContactBatch) {
		return wecomcapability.DispatchPollResult{}, wecomcapability.ErrDispatchInvalidState
	}
	credential, found, err := p.store.ContactBatchDispatchCredential(ctx, dashboardPrincipalFromDispatch(request.Principal))
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
			employeeID, ok := contactBatchDispatchEmployeeID(request.Dispatch.TargetID)
			if !ok {
				return wecomcapability.DispatchPollResult{}, &wecomcapability.DispatchProviderError{Code: "wecom.contact_batch_dispatch_target_invalid", Category: wecomcapability.DispatchErrorContract}
			}
			if err := p.collectContactBatchResults(ctx, credential, messageID, task.UserID, employeeID, "", &results, request); err != nil {
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

func (p *ContactBatchDispatchPoller) collectContactBatchResults(ctx context.Context, credential RoomWelcomeCorpCredential, messageID, userID string, employeeID int, cursor string, results *[]wecomcapability.DispatchResultRequest, request wecomcapability.DispatchPollRequest) error {
	page, err := p.client.GroupMessageSendResults(ctx, credential, messageID, userID, batchSendPageLimit, cursor)
	if err != nil {
		return classifyContactBatchProviderError(err)
	}
	for _, item := range page.SendList {
		if strings.TrimSpace(item.ExternalUserID) == "" {
			continue
		}
		status := wecomcapability.DispatchSucceeded
		errorCode := ""
		if item.Status != batchSendMessageDelivered {
			status = wecomcapability.DispatchFailed
			errorCode = "wecom.contact_batch_target_failed"
		}
		*results = append(*results, wecomcapability.DispatchResultRequest{Principal: request.Principal, TargetKind: "employee_external_userid", TargetID: contactBatchResultTargetID(employeeID, item.ExternalUserID), Status: status, ProviderTargetID: item.UserID, ErrorCode: errorCode})
	}
	if strings.TrimSpace(page.NextCursor) == "" {
		return nil
	}
	return p.collectContactBatchResults(ctx, credential, messageID, userID, employeeID, page.NextCursor, results, request)
}

func contactBatchDispatchEmployeeID(targetID string) (int, bool) {
	parts := strings.Split(strings.TrimSpace(targetID), ":")
	if len(parts) != 6 || parts[0] != "contact_batch" || parts[2] != "employee" || parts[4] != "chunk" {
		return 0, false
	}
	employeeID, err := strconv.Atoi(parts[3])
	return employeeID, err == nil && employeeID > 0 && parts[1] != "" && parts[5] != ""
}

func contactBatchResultTargetID(employeeID int, externalUserID string) string {
	return fmt.Sprintf("%d:%s", employeeID, strings.TrimSpace(externalUserID))
}

func deduplicateContactBatchResults(results []wecomcapability.DispatchResultRequest) []wecomcapability.DispatchResultRequest {
	byTarget := make(map[string]wecomcapability.DispatchResultRequest, len(results))
	order := make([]string, 0, len(results))
	for _, result := range results {
		if strings.TrimSpace(result.TargetID) == "" {
			continue
		}
		current, exists := byTarget[result.TargetID]
		if !exists {
			order = append(order, result.TargetID)
			byTarget[result.TargetID] = result
			continue
		}
		if current.Status == wecomcapability.DispatchSucceeded && result.Status != wecomcapability.DispatchSucceeded {
			byTarget[result.TargetID] = result
		}
	}
	result := make([]wecomcapability.DispatchResultRequest, 0, len(order))
	for _, targetID := range order {
		result = append(result, byTarget[targetID])
	}
	return result
}

func dispatchStatusFromContactResults(results []wecomcapability.DispatchResultRequest) string {
	if len(results) == 0 {
		return wecomcapability.DispatchFailed
	}
	success, failure := 0, 0
	for _, result := range results {
		if result.Status == wecomcapability.DispatchSucceeded {
			success++
		} else {
			failure++
		}
	}
	if failure == 0 {
		return wecomcapability.DispatchSucceeded
	}
	if success > 0 {
		return wecomcapability.DispatchPartialFailed
	}
	return wecomcapability.DispatchFailed
}

func classifyContactBatchProviderError(err error) error {
	if err == nil {
		return nil
	}
	// The RoomWelcomeWeComClient emits stable error strings:
	//   WECOM_HTTP_ERROR_<status> for non-2xx HTTP responses and
	//   WECOM_API_ERROR_<errcode> for WeCom business errcode responses.
	// Matching those exact contracts keeps 429/5xx ambiguous outcomes in the
	// retry/reconcile classes instead of failing them terminally as contract
	// errors.
	message := strings.ToUpper(err.Error())
	category := wecomcapability.DispatchErrorContract
	code := "wecom.contract_error"
	switch {
	case strings.Contains(message, "WECOM_HTTP_ERROR_429"):
		category, code = wecomcapability.DispatchErrorRateLimit, "wecom.http_429"
	case strings.Contains(message, "WECOM_HTTP_ERROR_401"), strings.Contains(message, "WECOM_API_ERROR_40001"), strings.Contains(message, "WECOM_API_ERROR_40014"):
		category, code = wecomcapability.DispatchErrorUnauthorized, "wecom.http_401"
	case strings.Contains(message, "WECOM_HTTP_ERROR_403"):
		category, code = wecomcapability.DispatchErrorForbidden, "wecom.http_403"
	case strings.Contains(message, "WECOM_HTTP_ERROR_5"):
		category, code = wecomcapability.DispatchErrorServer, "wecom.http_500"
	case strings.Contains(message, "TIMEOUT"), strings.Contains(message, "DEADLINE"):
		category, code = wecomcapability.DispatchErrorTimeout, "wecom.timeout"
	}
	return &wecomcapability.DispatchProviderError{Code: code, Category: category, Cause: err}
}

func dashboardPrincipalFromDispatch(principal wecomcapability.DispatchPrincipal) dashboardprincipal.DashboardPrincipal {
	return dashboardprincipal.DashboardPrincipal{UserID: principal.UserID, TenantID: principal.TenantID, CorpID: principal.CorpID, IsSuperAdmin: principal.IsSuperAdmin, AuthVersion: principal.AuthVersion}
}
