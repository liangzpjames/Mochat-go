package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"jiyi/mochat-go/internal/taskrunner"
)

type WorkContactSyncEvent struct {
	CorpID          int                       `json:"corpId,omitempty"`
	WXCorpID        string                    `json:"wxCorpId,omitempty"`
	Employee        WorkContactSyncEmployee   `json:"employee,omitempty"`
	Employees       []WorkContactSyncEmployee `json:"employees,omitempty"`
	ExternalUserIDs []string                  `json:"externalUserIds,omitempty"`
	Source          string                    `json:"source,omitempty"`
}

func (e *WorkContactSyncEvent) UnmarshalJSON(raw []byte) error {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	if strings.HasPrefix(trimmed, "{") {
		return e.unmarshalWorkContactSyncObject(raw)
	}
	return e.unmarshalWorkContactSyncLegacyArray(raw)
}

func (e *WorkContactSyncEvent) unmarshalWorkContactSyncObject(raw []byte) error {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}
	if rawValue, ok := firstJSONField(payload, "corpId", "corp_id"); ok {
		corpID, err := decodeMessageRemindInt(rawValue)
		if err != nil {
			return fmt.Errorf("corpId: %w", err)
		}
		e.CorpID = corpID
	}
	if err := decodeOptionalJSONField(payload, &e.WXCorpID, "wxCorpId", "wxCorpid", "wx_corpid", "wx_corp_id"); err != nil {
		return fmt.Errorf("wxCorpId: %w", err)
	}
	if err := decodeOptionalJSONField(payload, &e.Source, "source"); err != nil {
		return fmt.Errorf("source: %w", err)
	}
	if rawValue, ok := firstJSONField(payload, "employee"); ok {
		employee, err := decodeWorkContactSyncQueueEmployee(rawValue)
		if err != nil {
			return fmt.Errorf("employee: %w", err)
		}
		e.Employee = employee
	}
	if rawValue, ok := firstJSONField(payload, "employees"); ok {
		employees, err := decodeWorkContactSyncQueueEmployees(rawValue)
		if err != nil {
			return fmt.Errorf("employees: %w", err)
		}
		e.Employees = employees
	}
	if rawValue, ok := firstJSONField(payload, "externalUserIds", "external_userids", "external_user_ids", "wxContactIds", "wx_contact_ids"); ok {
		externalUserIDs, err := decodeWorkContactSyncExternalUserIDs(rawValue)
		if err != nil {
			return fmt.Errorf("externalUserIds: %w", err)
		}
		e.ExternalUserIDs = externalUserIDs
	}
	return nil
}

func (e *WorkContactSyncEvent) unmarshalWorkContactSyncLegacyArray(raw []byte) error {
	var legacy []json.RawMessage
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return err
	}
	if len(legacy) < 2 {
		return fmt.Errorf("work contact sync legacy payload requires at least 2 fields")
	}
	employees, err := decodeWorkContactSyncQueueEmployees(legacy[0])
	if err != nil {
		return fmt.Errorf("employee: %w", err)
	}
	if len(employees) == 1 {
		e.Employee = employees[0]
	} else {
		e.Employees = employees
	}
	corpID, err := decodeMessageRemindInt(legacy[1])
	if err != nil {
		return fmt.Errorf("corpId: %w", err)
	}
	e.CorpID = corpID
	if len(legacy) > 2 && strings.TrimSpace(string(legacy[2])) != "null" {
		if err := json.Unmarshal(legacy[2], &e.WXCorpID); err != nil {
			return fmt.Errorf("wxCorpId: %w", err)
		}
	}
	if len(legacy) > 3 && strings.TrimSpace(string(legacy[3])) != "null" {
		externalUserIDs, err := decodeWorkContactSyncExternalUserIDs(legacy[3])
		if err != nil {
			return fmt.Errorf("externalUserIds: %w", err)
		}
		e.ExternalUserIDs = externalUserIDs
	}
	return nil
}

func decodeWorkContactSyncQueueEmployees(raw json.RawMessage) ([]WorkContactSyncEmployee, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil, nil
	}
	if strings.HasPrefix(trimmed, "[") {
		var values []json.RawMessage
		if err := json.Unmarshal(raw, &values); err != nil {
			return nil, err
		}
		employees := make([]WorkContactSyncEmployee, 0, len(values))
		for _, value := range values {
			employee, err := decodeWorkContactSyncQueueEmployee(value)
			if err != nil {
				return nil, err
			}
			employees = append(employees, employee)
		}
		return employees, nil
	}
	employee, err := decodeWorkContactSyncQueueEmployee(raw)
	if err != nil {
		return nil, err
	}
	return []WorkContactSyncEmployee{employee}, nil
}

func decodeWorkContactSyncQueueEmployee(raw json.RawMessage) (WorkContactSyncEmployee, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		return WorkContactSyncEmployee{}, err
	}
	var employee WorkContactSyncEmployee
	if rawValue, ok := firstJSONField(payload, "id", "ID", "employeeId", "employee_id"); ok {
		id, err := decodeMessageRemindInt(rawValue)
		if err != nil {
			return WorkContactSyncEmployee{}, fmt.Errorf("id: %w", err)
		}
		employee.ID = id
	}
	if err := decodeOptionalJSONField(payload, &employee.WXUserID, "wxUserId", "wx_user_id", "wxUserid", "wx_userid", "WXUserID", "userid", "userId", "user_id"); err != nil {
		return WorkContactSyncEmployee{}, fmt.Errorf("wxUserId: %w", err)
	}
	return employee, nil
}

func decodeWorkContactSyncExternalUserIDs(raw json.RawMessage) ([]string, error) {
	recipients, err := decodeMessageRemindRecipients(raw)
	if err != nil {
		return nil, err
	}
	return []string(recipients), nil
}

type WorkContactSyncDelivery struct {
	Event    WorkContactSyncEvent
	Raw      string
	Attempts int
}

type WorkContactSyncWorkerQueue interface {
	DequeueWorkContactSync(ctx context.Context, timeout time.Duration) (WorkContactSyncDelivery, bool, error)
	AckWorkContactSync(ctx context.Context, delivery WorkContactSyncDelivery) error
	RetryWorkContactSync(ctx context.Context, delivery WorkContactSyncDelivery, reason string, maxAttempts int) (bool, error)
	RecoverWorkContactSyncProcessing(ctx context.Context, staleAfter time.Duration, maxAttempts int) (int, error)
}

type WorkContactSyncWorkerStore interface {
	RoomWelcomeCorpCredentialByID(ctx context.Context, corpID int) (RoomWelcomeCorpCredential, bool, error)
	RoomWelcomeCorpCredentialByWXCorpID(ctx context.Context, wxCorpID string) (RoomWelcomeCorpCredential, bool, error)
	WorkContactSyncEmployees(ctx context.Context, corpID int) ([]WorkContactSyncEmployee, error)
	SyncWorkContacts(ctx context.Context, corpID int, bundles []WorkContactSyncEmployeeContacts) (WorkContactSyncResult, error)
}

type WorkContactSyncWorker struct {
	queue             WorkContactSyncWorkerQueue
	store             WorkContactSyncWorkerStore
	client            WorkContactSyncClient
	pollTimeout       time.Duration
	maxAttempts       int
	processingTimeout time.Duration
	recoveryInterval  time.Duration
	alertNotifier     SaaSAlertNotifier
	logger            *log.Logger
}

func NewWorkContactSyncWorker(queue WorkContactSyncWorkerQueue, store WorkContactSyncWorkerStore, client WorkContactSyncClient, logger *log.Logger) *WorkContactSyncWorker {
	if logger == nil {
		logger = log.Default()
	}
	if client == nil {
		client = NewRoomWelcomeWeComClient(defaultWeComAPIBaseURL)
	}
	return &WorkContactSyncWorker{
		queue:             queue,
		store:             store,
		client:            client,
		pollTimeout:       5 * time.Second,
		maxAttempts:       3,
		processingTimeout: 5 * time.Minute,
		recoveryInterval:  time.Minute,
		logger:            logger,
	}
}

func (w *WorkContactSyncWorker) WithProcessingTimeout(timeout time.Duration) *WorkContactSyncWorker {
	if timeout > 0 {
		w.processingTimeout = timeout
		w.recoveryInterval = timeout
		if w.recoveryInterval > time.Minute {
			w.recoveryInterval = time.Minute
		}
	}
	return w
}

func (w *WorkContactSyncWorker) WithSaaSAlertNotifier(notifier SaaSAlertNotifier) *WorkContactSyncWorker {
	w.alertNotifier = notifier
	return w
}

func (w *WorkContactSyncWorker) Run(ctx context.Context) error {
	if w.queue == nil || w.store == nil || w.client == nil {
		return fmt.Errorf("work contact sync worker dependencies are not configured")
	}
	nextRecovery := time.Now()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if !time.Now().Before(nextRecovery) {
			w.recoverProcessing(ctx)
			nextRecovery = time.Now().Add(w.recoveryInterval)
		}
		delivery, ok, err := w.queue.DequeueWorkContactSync(ctx, w.pollTimeout)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			w.logger.Printf("work contact sync dequeue failed: %v", err)
			continue
		}
		if !ok {
			continue
		}
		w.handleDelivery(ctx, delivery)
	}
}

func (w *WorkContactSyncWorker) recoverProcessing(ctx context.Context) {
	recovered, err := w.queue.RecoverWorkContactSyncProcessing(ctx, w.processingTimeout, w.maxAttempts)
	if err != nil {
		w.logger.Printf("work contact sync processing recovery failed: %v", err)
		return
	}
	if recovered > 0 {
		w.logger.Printf("work contact sync recovered processing jobs: %d", recovered)
	}
}

func (w *WorkContactSyncWorker) handleDelivery(ctx context.Context, delivery WorkContactSyncDelivery) {
	ctx = WithSaaSAlertNotifier(ctx, w.alertNotifier)
	corpIDForExecution := delivery.Event.CorpID
	if corpIDForExecution <= 0 {
		if credential, err := w.credential(ctx, delivery.Event); err == nil {
			corpIDForExecution = credential.CorpID
		}
	}
	tenantID := tenantIDForQueueExecution(ctx, w.logger, w.store, corpIDForExecution)
	finishExecution := startQueueItemExecution(ctx, w.logger, QueueNameWorkContactSync, w.store, tenantID)
	if err := w.Process(ctx, delivery.Event); err != nil {
		deadLettered, retryErr := w.queue.RetryWorkContactSync(ctx, delivery, err.Error(), w.maxAttempts)
		if retryErr != nil {
			finishExecution(taskrunner.StatusFailed, fmt.Errorf("%w; retry failed: %v", err, retryErr))
			w.logger.Printf("work contact sync retry failed: corp_id=%d wx_corpid=%s source=%s attempts=%d err=%v retry_err=%v", delivery.Event.CorpID, delivery.Event.WXCorpID, delivery.Event.Source, delivery.Attempts+1, err, retryErr)
			return
		}
		finishExecution(taskrunner.StatusFailed, err)
		if deadLettered {
			w.logger.Printf("work contact sync moved to dead letter: corp_id=%d wx_corpid=%s source=%s attempts=%d err=%v", delivery.Event.CorpID, delivery.Event.WXCorpID, delivery.Event.Source, delivery.Attempts+1, err)
			return
		}
		w.logger.Printf("work contact sync requeued: corp_id=%d wx_corpid=%s source=%s attempts=%d err=%v", delivery.Event.CorpID, delivery.Event.WXCorpID, delivery.Event.Source, delivery.Attempts+1, err)
		return
	}
	if err := w.queue.AckWorkContactSync(ctx, delivery); err != nil {
		finishExecution(taskrunner.StatusFailed, fmt.Errorf("ack failed: %w", err))
		w.logger.Printf("work contact sync ack failed: corp_id=%d wx_corpid=%s source=%s err=%v", delivery.Event.CorpID, delivery.Event.WXCorpID, delivery.Event.Source, err)
		return
	}
	finishExecution(taskrunner.StatusSucceeded, nil)
}

func (w *WorkContactSyncWorker) Process(ctx context.Context, event WorkContactSyncEvent) error {
	credential, err := w.credential(ctx, event)
	if err != nil {
		return err
	}
	employees, err := w.employees(ctx, credential.CorpID, event)
	if err != nil {
		return err
	}
	if len(employees) == 0 {
		return fmt.Errorf("查询不到有效的企业微信成员信息")
	}
	bundles := make([]WorkContactSyncEmployeeContacts, 0, len(employees))
	for _, employee := range employees {
		employee.WXUserID = strings.TrimSpace(employee.WXUserID)
		if employee.ID <= 0 || employee.WXUserID == "" {
			continue
		}
		externalUserIDs := uniqueNonEmptyStrings(event.ExternalUserIDs)
		noContact := false
		if len(externalUserIDs) == 0 {
			var err error
			externalUserIDs, noContact, err = w.client.ExternalContactList(ctx, credential, employee.WXUserID)
			if err != nil {
				return err
			}
			externalUserIDs = uniqueNonEmptyStrings(externalUserIDs)
		}
		bundle := WorkContactSyncEmployeeContacts{Employee: employee, ExternalUserIDs: externalUserIDs, NoContact: noContact}
		if !noContact {
			for _, wxExternalUserID := range externalUserIDs {
				contact, err := w.client.ExternalContactDetail(ctx, credential, wxExternalUserID)
				if err != nil {
					return err
				}
				if strings.TrimSpace(contact.WXExternalUserID) != "" {
					bundle.Contacts = append(bundle.Contacts, contact)
				}
			}
		}
		bundles = append(bundles, bundle)
	}
	if len(bundles) == 0 {
		return fmt.Errorf("查询不到有效的企业微信成员信息")
	}
	_, err = w.store.SyncWorkContacts(ctx, credential.CorpID, bundles)
	return err
}

func (w *WorkContactSyncWorker) credential(ctx context.Context, event WorkContactSyncEvent) (RoomWelcomeCorpCredential, error) {
	if event.CorpID > 0 {
		credential, found, err := w.store.RoomWelcomeCorpCredentialByID(ctx, event.CorpID)
		if err != nil {
			return RoomWelcomeCorpCredential{}, err
		}
		if !found || strings.TrimSpace(credential.WXCorpID) == "" || strings.TrimSpace(credential.ContactSecret) == "" {
			return RoomWelcomeCorpCredential{}, fmt.Errorf("corp %d contact credential is not configured", event.CorpID)
		}
		return credential, nil
	}
	wxCorpID := strings.TrimSpace(event.WXCorpID)
	if wxCorpID == "" {
		return RoomWelcomeCorpCredential{}, fmt.Errorf("missing corp id or wx corpid")
	}
	credential, found, err := w.store.RoomWelcomeCorpCredentialByWXCorpID(ctx, wxCorpID)
	if err != nil {
		return RoomWelcomeCorpCredential{}, err
	}
	if !found || credential.CorpID <= 0 || strings.TrimSpace(credential.WXCorpID) == "" || strings.TrimSpace(credential.ContactSecret) == "" {
		return RoomWelcomeCorpCredential{}, fmt.Errorf("wx corpid %s contact credential is not configured", wxCorpID)
	}
	return credential, nil
}

func (w *WorkContactSyncWorker) employees(ctx context.Context, corpID int, event WorkContactSyncEvent) ([]WorkContactSyncEmployee, error) {
	employees := make([]WorkContactSyncEmployee, 0, len(event.Employees)+1)
	if event.Employee.ID > 0 || strings.TrimSpace(event.Employee.WXUserID) != "" {
		employees = append(employees, event.Employee)
	}
	employees = append(employees, event.Employees...)
	employees = normalizeWorkContactSyncEmployees(employees)
	if len(employees) > 0 {
		return employees, nil
	}
	return w.store.WorkContactSyncEmployees(ctx, corpID)
}

func normalizeWorkContactSyncEmployees(values []WorkContactSyncEmployee) []WorkContactSyncEmployee {
	seen := map[string]struct{}{}
	out := make([]WorkContactSyncEmployee, 0, len(values))
	for _, value := range values {
		value.WXUserID = strings.TrimSpace(value.WXUserID)
		if value.ID <= 0 || value.WXUserID == "" {
			continue
		}
		key := fmt.Sprintf("%d:%s", value.ID, value.WXUserID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		return out[i].WXUserID < out[j].WXUserID
	})
	return out
}
