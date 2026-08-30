package dashboard

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/taskrunner"
)

const (
	workContactEmployeeStatusRemoved        = 2
	workContactEmployeeStatusPassiveRemoved = 3
	weWorkCallbackLeaseCompletionGrace      = 30 * time.Second
	weWorkCallbackDependencyDeferMaxDelay   = 5 * time.Minute
)

var (
	ErrWeWorkCallbackDependencyUnavailable       = errors.New("wework callback dependency is unavailable")
	ErrWeWorkCallbackSideEffectReconcileRequired = errors.New("wework callback side effect requires reconciliation")
)

type WeWorkCallbackWorkerStore interface {
	SOPLogCronStore
	GenericContactWelcomeStore
	ChannelCodeWelcomeStore
	WorkRoomAutoPullWelcomeStore
	WorkFissionContactRuleStore
	WorkFissionContactWelcomeStore
	WorkFissionAddContactStore
	RoomTagPullRemindAgentByCorpID(ctx context.Context, corpID int) (RoomTagPullAgentCredential, bool, error)
	WorkEmployeeSyncCredentials(ctx context.Context, corpIDs []int) ([]WorkEmployeeSyncCredential, error)
	SyncWorkEmployees(ctx context.Context, credential WorkEmployeeSyncCredential, departments []WorkEmployeeSyncDepartment, employees []WorkEmployeeSyncEmployee, followUserIDs []string, defaultPasswordHash string) (WorkEmployeeSyncResult, error)
	RoomWelcomeCorpCredentialByID(ctx context.Context, corpID int) (RoomWelcomeCorpCredential, bool, error)
	WorkContactSyncEmployees(ctx context.Context, corpID int) ([]WorkContactSyncEmployee, error)
	WorkContactSyncEmployeeByWXUserID(ctx context.Context, corpID int, wxUserID string) (WorkContactSyncEmployee, bool, error)
	SyncWorkContacts(ctx context.Context, corpID int, bundles []WorkContactSyncEmployeeContacts) (WorkContactSyncResult, error)
	SyncWorkContactForEmployee(ctx context.Context, corpID int, employee WorkContactSyncEmployee, contact WorkContactSyncContact, createMissing bool) (WorkContactSyncResult, error)
	UpdateWorkContactProfile(ctx context.Context, values WorkContactUpdateValues) (WorkContactUpdateResult, bool, error)
	RemoveWorkContactEmployeeRelation(ctx context.Context, corpID int, wxUserID string, wxExternalUserID string, status int) (WorkContactRemovalResult, bool, error)
	SyncWorkContactTags(ctx context.Context, corpID int, groups []WorkContactTagSyncGroup) (WorkContactTagSyncResult, error)
	SyncWorkContactTagGroup(ctx context.Context, corpID int, group WorkContactTagSyncGroup) (WorkContactTagSyncResult, error)
	DeleteWorkContactTagByWXContactTagID(ctx context.Context, corpID int, wxContactTagID string) (bool, error)
	DeleteWorkContactTagGroupByWXGroupID(ctx context.Context, corpID int, wxGroupID string) (bool, error)
	SyncWorkRooms(ctx context.Context, corpID int, rooms []WorkRoomSyncRoom) (WorkRoomSyncResult, error)
	SyncWorkRoom(ctx context.Context, corpID int, room WorkRoomSyncRoom) (WorkRoomSyncResult, error)
	DeleteWorkRoomByWXChatID(ctx context.Context, corpID int, wxChatID string) (bool, error)
	AutoTagRoomJoinTask(ctx context.Context, corpID int, wxChatID string) (AutoTagRoomJoinTaskResult, error)
	AutoTagContactTimeTask(ctx context.Context, corpID int, employeeID int, contactID int) (AutoTagContactTimeTaskResult, error)
	DeleteWorkEmployeeByWXUserID(ctx context.Context, corpID int, wxUserID string) (bool, error)
	SyncWorkDepartment(ctx context.Context, corpID int, department WorkDepartmentEventDepartment) (WorkEmployeeSyncResult, error)
	DeleteWorkDepartmentByWXDepartmentID(ctx context.Context, corpID int, wxDepartmentID int) (bool, error)
	TenantIDByCorpID(ctx context.Context, corpID int) (int, error)
}

type WeWorkCallbackWorkerCapabilities struct {
	ContactWelcomeQueue ContactWelcomeEnqueuer
	ContactWelcomeCache ContactWelcomeStatusCache
	MarkTagsQueue       AutoTagMarkTagsQueue
}

type WeWorkCallbackCapabilityResolver interface {
	ResolveWeWorkCallbackCapabilities(context.Context) (WeWorkCallbackWorkerCapabilities, error)
}

type WorkFissionContactRule struct {
	ID     int
	TagIDs []int
}

type WorkContactRemovalResult struct {
	ContactID   int
	ContactName string
	WXUserID    string
	Status      int
}

type WorkFissionContactRuleStore interface {
	WorkFissionContactRuleByID(ctx context.Context, id int) (WorkFissionContactRule, bool, error)
}

type WorkFissionAddContactEvent struct {
	CorpID           int
	ParentContactID  int
	EmployeeID       int
	EmployeeWXUserID string
	WXExternalUserID string
	UnionID          string
	Name             string
	Avatar           string
	IsNew            bool
}

type WorkFissionAddContactResult struct {
	ParentContactID  int
	ChildContactID   int
	FissionID        int
	InviteCount      int
	TotalCount       int
	Completed        bool
	EmployeeReminder *WorkFissionEmployeeReminder
	CustomerPush     *WorkFissionCustomerPush
}

type WorkFissionEmployeeReminder struct {
	ToUser  string
	Content string
}

type WorkFissionCustomerPush struct {
	Sender         string
	ExternalUserID string
	Content        []ContactMessageBatchSendContent
}

type WorkFissionAddContactStore interface {
	HandleWorkFissionAddContact(ctx context.Context, event WorkFissionAddContactEvent) (WorkFissionAddContactResult, bool, error)
}

type WeWorkCallbackWorkerClient interface {
	Departments(ctx context.Context, credential WorkEmployeeSyncCredential) ([]WorkEmployeeSyncDepartment, error)
	DepartmentUsers(ctx context.Context, credential WorkEmployeeSyncCredential, wxDepartmentID int) ([]WorkEmployeeSyncEmployee, error)
	User(ctx context.Context, credential WorkEmployeeSyncCredential, wxUserID string) (WorkEmployeeSyncEmployee, error)
	FollowUsers(ctx context.Context, credential WorkEmployeeSyncCredential) ([]string, error)
	CorpTags(ctx context.Context, credential RoomWelcomeCorpCredential) ([]WorkContactTagSyncGroup, error)
	CorpTagsByIDs(ctx context.Context, credential RoomWelcomeCorpCredential, groupIDs []string, tagIDs []string) ([]WorkContactTagSyncGroup, error)
	ExternalContactList(ctx context.Context, credential RoomWelcomeCorpCredential, wxUserID string) ([]string, bool, error)
	ExternalContactDetail(ctx context.Context, credential RoomWelcomeCorpCredential, wxExternalUserID string) (WorkContactSyncContact, error)
	MarkExternalContactTags(ctx context.Context, credential RoomWelcomeCorpCredential, payload WorkContactMarkTagsPayload) error
	UploadTemporaryImage(ctx context.Context, credential RoomWelcomeCorpCredential, filePath string) (string, error)
	SubmitContactMessageBatchSend(ctx context.Context, credential RoomWelcomeCorpCredential, payload ContactMessageBatchSendMessagePayload) (ContactMessageBatchSendMessageResult, error)
	SendAgentTextMessageWithDuplicateCheck(ctx context.Context, credential RoomTagPullAgentCredential, toUser string, content string) error
	GroupChats(ctx context.Context, credential RoomWelcomeCorpCredential) ([]WorkRoomSyncGroupChat, error)
	GroupChatDetail(ctx context.Context, credential RoomWelcomeCorpCredential, wxChatID string) (WorkRoomSyncRoom, error)
}

type WorkMessageArchiveSyncTrigger interface {
	RunCorp(context.Context, int) error
}

type WeWorkCallbackWakeupWaiter interface {
	WaitWeWorkCallbackWakeup(context.Context, time.Duration) error
}

type WeWorkCallbackWorker struct {
	capabilities       WeWorkCallbackWorkerCapabilities
	capabilityResolver WeWorkCallbackCapabilityResolver
	inbox              WeWorkCallbackInbox
	store              WeWorkCallbackWorkerStore
	client             WeWorkCallbackWorkerClient
	passwordKey        string
	pollTimeout        time.Duration
	maxAttempts        int
	processingTimeout  time.Duration
	retryDelay         time.Duration
	apiBaseURL         string
	operationBaseURL   string
	sidebarBaseURL     string
	fileStorageRoot    string
	alertNotifier      SaaSAlertNotifier
	archiveSyncTrigger WorkMessageArchiveSyncTrigger
	wakeupWaiter       WeWorkCallbackWakeupWaiter
	logger             *log.Logger
	now                func() time.Time
}

func NewWeWorkCallbackWorker(capabilities WeWorkCallbackWorkerCapabilities, store WeWorkCallbackWorkerStore, client WeWorkCallbackWorkerClient, passwordKey string, logger *log.Logger) *WeWorkCallbackWorker {
	if logger == nil {
		logger = log.Default()
	}
	worker := &WeWorkCallbackWorker{
		capabilities:      capabilities,
		store:             store,
		client:            client,
		passwordKey:       passwordKey,
		pollTimeout:       5 * time.Second,
		maxAttempts:       3,
		processingTimeout: 5 * time.Minute,
		retryDelay:        time.Second,
		fileStorageRoot:   defaultRoomTagPullFileStorageRoot,
		logger:            logger,
		now:               time.Now,
	}
	worker.inbox, _ = store.(WeWorkCallbackInbox)
	return worker
}

func (w *WeWorkCallbackWorker) WithCapabilityResolver(resolver WeWorkCallbackCapabilityResolver) *WeWorkCallbackWorker {
	w.capabilityResolver = resolver
	return w
}

func (w *WeWorkCallbackWorker) resolveCapabilities(ctx context.Context) (WeWorkCallbackWorkerCapabilities, error) {
	if w.capabilityResolver == nil {
		return w.capabilities, nil
	}
	return w.capabilityResolver.ResolveWeWorkCallbackCapabilities(ctx)
}

func (w *WeWorkCallbackWorker) WithProcessingTimeout(timeout time.Duration) *WeWorkCallbackWorker {
	if timeout > 0 {
		w.processingTimeout = timeout
	}
	return w
}

func (w *WeWorkCallbackWorker) WithWorkFissionBaseURLs(apiBaseURL string, operationBaseURL string) *WeWorkCallbackWorker {
	w.apiBaseURL = strings.TrimRight(strings.TrimSpace(apiBaseURL), "/")
	w.operationBaseURL = strings.TrimRight(strings.TrimSpace(operationBaseURL), "/")
	return w
}

func (w *WeWorkCallbackWorker) WithSidebarBaseURL(sidebarBaseURL string) *WeWorkCallbackWorker {
	w.sidebarBaseURL = strings.TrimRight(strings.TrimSpace(sidebarBaseURL), "/")
	return w
}

func (w *WeWorkCallbackWorker) WithFileStorageRoot(fileStorageRoot string) *WeWorkCallbackWorker {
	if strings.TrimSpace(fileStorageRoot) != "" {
		w.fileStorageRoot = fileStorageRoot
	}
	return w
}

func (w *WeWorkCallbackWorker) WithSaaSAlertNotifier(notifier SaaSAlertNotifier) *WeWorkCallbackWorker {
	w.alertNotifier = notifier
	return w
}

func (w *WeWorkCallbackWorker) WithArchiveSyncTrigger(trigger WorkMessageArchiveSyncTrigger) *WeWorkCallbackWorker {
	w.archiveSyncTrigger = trigger
	return w
}

func (w *WeWorkCallbackWorker) WithWakeupWaiter(waiter WeWorkCallbackWakeupWaiter) *WeWorkCallbackWorker {
	w.wakeupWaiter = waiter
	return w
}

func (w *WeWorkCallbackWorker) WithNow(now func() time.Time) *WeWorkCallbackWorker {
	if now != nil {
		w.now = now
	}
	return w
}

func (w *WeWorkCallbackWorker) Run(ctx context.Context) error {
	if w.inbox == nil || w.store == nil || w.client == nil {
		return fmt.Errorf("wework callback worker dependencies are not configured")
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		claim, ok, err := w.inbox.ClaimWeWorkCallback(ctx, w.processingTimeout+weWorkCallbackLeaseCompletionGrace, w.maxAttempts)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			w.logger.Printf("wework callback durable claim failed: %v", errors.New(SanitizeWeWorkCallbackFailure(err.Error())))
			if err := waitWeWorkCallbackPoll(ctx, w.pollTimeout); err != nil {
				return err
			}
			continue
		}
		if !ok {
			if err := w.waitForCallbackWork(ctx); err != nil {
				return err
			}
			continue
		}
		w.handleClaim(ctx, claim)
	}
}

func (w *WeWorkCallbackWorker) waitForCallbackWork(ctx context.Context) error {
	if w.wakeupWaiter == nil {
		return waitWeWorkCallbackPoll(ctx, w.pollTimeout)
	}
	if err := w.wakeupWaiter.WaitWeWorkCallbackWakeup(ctx, w.pollTimeout); err == nil {
		return nil
	} else if ctx.Err() != nil {
		return ctx.Err()
	} else {
		w.logger.Printf("wework callback wakeup wait failed: %v", errors.New(SanitizeWeWorkCallbackFailure(err.Error())))
		return waitWeWorkCallbackPoll(ctx, w.pollTimeout)
	}
}

// ImportLegacyWeWorkCallbackBacklog is used only by the explicit maintenance
// cutover command after every legacy producer, consumer, and retry writer has
// been stopped. Ordinary inbox workers never call it and therefore do not
// depend on Redis availability.
func ImportLegacyWeWorkCallbackBacklog(ctx context.Context, legacy LegacyWeWorkCallbackBacklog, store LegacyWeWorkCallbackImportStore, logger *log.Logger) (int, error) {
	if legacy == nil || store == nil {
		return 0, errors.New("legacy wework callback cutover dependencies are not configured")
	}
	if logger == nil {
		logger = log.Default()
	}
	stats, err := legacy.PreflightLegacyWeWorkCallbackBacklog(ctx)
	if err != nil {
		return 0, fmt.Errorf("legacy wework callback backlog preflight: %w", err)
	}
	logger.Printf("legacy wework callback backlog preflight: pending=%d processing=%d dead=%d", stats.Pending, stats.Processing, stats.Dead)
	if stats.Dead > 0 {
		return 0, fmt.Errorf("%w: dead=%d pending=%d processing=%d", ErrLegacyWeWorkCallbackDeadBacklog, stats.Dead, stats.Pending, stats.Processing)
	}
	imported := 0
	for {
		delivery, ok, err := legacy.NextLegacyWeWorkCallback(ctx)
		if err != nil {
			return imported, fmt.Errorf("read legacy wework callback backlog: %w", err)
		}
		if !ok {
			break
		}
		event := delivery.Event
		if event.TenantID <= 0 {
			event.TenantID, err = store.TenantIDByCorpID(ctx, event.CorpID)
			if err != nil {
				return imported, fmt.Errorf("resolve legacy wework callback tenant: %w", err)
			}
		}
		if event.TenantID <= 0 || event.CorpID <= 0 {
			return imported, fmt.Errorf("legacy wework callback scope is incomplete")
		}
		event.RawXML = ""
		event.Message = normalizedWeWorkCallbackMessage(event.Message)
		if _, err := store.AcceptWeWorkCallback(ctx, event, WeWorkCallbackEventKey(event), WeWorkCallbackPayloadFingerprint(event)); err != nil {
			return imported, fmt.Errorf("accept legacy wework callback durably: %w", err)
		}
		if err := legacy.AckLegacyWeWorkCallback(ctx, delivery); err != nil {
			return imported, fmt.Errorf("ack imported legacy wework callback: %w", err)
		}
		imported++
	}
	if imported > 0 {
		logger.Printf("legacy wework callback backlog imported: count=%d", imported)
	}
	finalStats, err := legacy.PreflightLegacyWeWorkCallbackBacklog(ctx)
	if err != nil {
		return imported, fmt.Errorf("legacy wework callback final preflight: %w", err)
	}
	if finalStats.Pending != 0 || finalStats.Processing != 0 || finalStats.Dead != 0 {
		return imported, fmt.Errorf("legacy wework callback backlog changed during cutover: pending=%d processing=%d dead=%d", finalStats.Pending, finalStats.Processing, finalStats.Dead)
	}
	return imported, nil
}

func waitWeWorkCallbackPoll(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		delay = time.Second
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (w *WeWorkCallbackWorker) handleClaim(ctx context.Context, claim WeWorkCallbackClaim) {
	if err := w.inbox.ValidateWeWorkCallbackClaim(ctx, claim); err != nil {
		w.logger.Printf("wework callback claim rejected before side effects: event_key=%s fence=%d err=%v", claim.EventKey, claim.LeaseFence, errors.New(SanitizeWeWorkCallbackFailure(err.Error())))
		return
	}
	leaseCtx := ctx
	processCtx, cancelProcess := context.WithTimeout(ctx, w.processingTimeout)
	defer cancelProcess()
	claim.Event.EventKey = claim.EventKey
	claim.Event.LeaseFence = claim.LeaseFence
	processCtx = withWeWorkCallbackExecution(processCtx, claim)
	processCtx = WithSaaSAlertNotifier(processCtx, w.alertNotifier)
	tenantID := claim.Event.TenantID
	if tenantID <= 0 {
		tenantID = tenantIDForQueueExecution(processCtx, w.logger, w.store, claim.Event.CorpID)
	}
	finishExecution := startQueueItemExecution(processCtx, w.logger, "wework-callback", w.store, tenantID)
	if err := w.Process(processCtx, claim.Event); err != nil {
		safeErr := errors.New(SanitizeWeWorkCallbackFailure(err.Error()))
		if errors.Is(err, ErrWeWorkCallbackDependencyUnavailable) {
			retryDelay := weWorkCallbackDependencyDeferDelay(w.retryDelay, claim.DependencyDeferCount)
			deferErr := w.inbox.DeferWeWorkCallbackDependency(leaseCtx, claim, safeErr.Error(), retryDelay)
			if deferErr != nil {
				safeDeferErr := errors.New(SanitizeWeWorkCallbackFailure(deferErr.Error()))
				finishExecution(taskrunner.StatusFailed, fmt.Errorf("%w; durable dependency defer failed: %v", safeErr, safeDeferErr))
				w.logger.Printf("wework callback durable dependency defer failed: event_key=%s fence=%d err=%v transition_err=%v", claim.EventKey, claim.LeaseFence, safeErr, safeDeferErr)
				return
			}
			finishExecution(taskrunner.StatusFailed, safeErr)
			w.logger.Printf("wework callback durable dependency deferred: event_key=%s fence=%d dependency_defers=%d retry_in=%s err=%v", claim.EventKey, claim.LeaseFence, claim.DependencyDeferCount+1, retryDelay, safeErr)
			return
		}
		deadLettered, failErr := w.inbox.FailWeWorkCallback(leaseCtx, claim, safeErr.Error(), w.maxAttempts, w.retryDelay)
		if failErr != nil {
			safeFailErr := errors.New(SanitizeWeWorkCallbackFailure(failErr.Error()))
			finishExecution(taskrunner.StatusFailed, fmt.Errorf("%w; durable fail transition failed: %v", safeErr, safeFailErr))
			w.logger.Printf("wework callback durable fail transition failed: event_key=%s fence=%d err=%v transition_err=%v", claim.EventKey, claim.LeaseFence, safeErr, safeFailErr)
			return
		}
		finishExecution(taskrunner.StatusFailed, safeErr)
		if deadLettered {
			w.logger.Printf("wework callback durable event dead lettered: event_key=%s fence=%d attempts=%d err=%v", claim.EventKey, claim.LeaseFence, claim.Attempt, safeErr)
			return
		}
		w.logger.Printf("wework callback durable event scheduled for replay: event_key=%s fence=%d attempts=%d err=%v", claim.EventKey, claim.LeaseFence, claim.Attempt, safeErr)
		return
	}
	if err := w.inbox.CompleteWeWorkCallback(leaseCtx, claim); err != nil {
		safeErr := errors.New(SanitizeWeWorkCallbackFailure(err.Error()))
		finishExecution(taskrunner.StatusFailed, fmt.Errorf("durable completion failed: %w", safeErr))
		w.logger.Printf("wework callback durable completion failed: event_key=%s fence=%d err=%v", claim.EventKey, claim.LeaseFence, safeErr)
		return
	}
	finishExecution(taskrunner.StatusSucceeded, nil)
}

func weWorkCallbackDependencyDeferDelay(base time.Duration, deferCount int) time.Duration {
	if base <= 0 {
		base = time.Second
	}
	if base >= weWorkCallbackDependencyDeferMaxDelay {
		return weWorkCallbackDependencyDeferMaxDelay
	}
	delay := base
	for count := 0; count < deferCount; count++ {
		if delay >= weWorkCallbackDependencyDeferMaxDelay/2 {
			return weWorkCallbackDependencyDeferMaxDelay
		}
		delay *= 2
	}
	return delay
}

func (w *WeWorkCallbackWorker) Process(ctx context.Context, event WeWorkCallbackEvent) error {
	corpID := event.CorpID
	if corpID <= 0 {
		return fmt.Errorf("missing corp id")
	}
	switch strings.TrimSpace(event.EventPath) {
	case "event.msgaudit_notify":
		if w.archiveSyncTrigger == nil {
			return nil
		}
		return w.archiveSyncTrigger.RunCorp(ctx, corpID)
	case "event.change_contact.create_user", "event.change_contact.update_user":
		return w.syncEmployeeFromEvent(ctx, corpID, event)
	case "event.change_contact.create_party":
		return w.syncDepartmentFromEvent(ctx, corpID, event, true)
	case "event.change_contact.update_party":
		return w.syncDepartmentFromEvent(ctx, corpID, event, false)
	case "event.change_contact.delete_party":
		return w.deleteDepartmentFromEvent(ctx, corpID, event)
	case "event.change_contact.delete_user":
		return w.deleteEmployeeFromEvent(ctx, corpID, event)
	case "event.change_external_tag.create", "event.change_external_tag.update":
		return w.syncContactTagFromEvent(ctx, corpID, event)
	case "event.change_external_tag.delete":
		return w.deleteContactTagFromEvent(ctx, corpID, event)
	case "event.change_contact.update_tag", "event.change_external_tag.shuffle":
		return nil
	case "event.change_external_contact.add_external_contact":
		return w.syncContactFromEvent(ctx, corpID, event, true)
	case "event.change_external_contact.edit_external_contact":
		return w.syncContactFromEvent(ctx, corpID, event, false)
	case "event.change_external_contact.del_external_contact":
		return w.removeContactRelationFromEvent(ctx, corpID, event, workContactEmployeeStatusRemoved)
	case "event.change_external_contact.del_follow_user":
		return w.removeContactRelationFromEvent(ctx, corpID, event, workContactEmployeeStatusPassiveRemoved)
	case "event.change_external_contact.add_half_external_contact", "event.change_external_contact.transfer_fail":
		return nil
	case "event.change_external_chat.create", "event.change_external_chat.update":
		return w.syncRoomFromEvent(ctx, corpID, event)
	case "event.change_external_chat.dismiss":
		wxChatID := strings.TrimSpace(event.Message["ChatId"])
		if wxChatID == "" {
			wxChatID = strings.TrimSpace(event.Message["ChatID"])
		}
		if wxChatID == "" {
			return nil
		}
		_, err := w.store.DeleteWorkRoomByWXChatID(ctx, corpID, wxChatID)
		return err
	default:
		return nil
	}
}

func (w *WeWorkCallbackWorker) syncEmployees(ctx context.Context, corpID int) error {
	return syncWorkEmployeesForCorp(ctx, w.store, w.client, w.passwordKey, corpID)
}

func (w *WeWorkCallbackWorker) syncEmployeeFromEvent(ctx context.Context, corpID int, event WeWorkCallbackEvent) error {
	wxUserID := weWorkCallbackEmployeeUserID(event)
	if wxUserID == "" {
		return nil
	}
	credentials, err := w.store.WorkEmployeeSyncCredentials(ctx, []int{corpID})
	if err != nil {
		return err
	}
	if len(credentials) == 0 {
		return fmt.Errorf("corp credential not found")
	}
	credential := credentials[0]
	if strings.TrimSpace(credential.WXCorpID) == "" || strings.TrimSpace(credential.EmployeeSecret) == "" {
		return fmt.Errorf("corp employee credential is incomplete")
	}
	departments, err := w.client.Departments(ctx, credential)
	if err != nil {
		return err
	}
	employee, err := w.client.User(ctx, credential, wxUserID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(employee.WXUserID) == "" {
		employee.WXUserID = wxUserID
	}
	if strings.TrimSpace(employee.WXUserID) == "" {
		return nil
	}
	followUserIDs, _ := w.client.FollowUsers(ctx, credential)
	_, err = w.store.SyncWorkEmployees(ctx, credential, departments, []WorkEmployeeSyncEmployee{employee}, followUserIDs, "")
	return err
}

func (w *WeWorkCallbackWorker) deleteEmployeeFromEvent(ctx context.Context, corpID int, event WeWorkCallbackEvent) error {
	wxUserID := weWorkCallbackEmployeeUserID(event)
	if wxUserID == "" {
		return nil
	}
	_, err := w.store.DeleteWorkEmployeeByWXUserID(ctx, corpID, wxUserID)
	return err
}

func (w *WeWorkCallbackWorker) syncDepartmentFromEvent(ctx context.Context, corpID int, event WeWorkCallbackEvent, create bool) error {
	wxDepartmentID, ok := weWorkCallbackInt(event, "Id", "ID", "id")
	if !ok || wxDepartmentID <= 0 {
		return nil
	}
	name, hasName := weWorkCallbackString(event, "Name", "name")
	parentID, hasParent := weWorkCallbackInt(event, "ParentId", "ParentID", "parentid", "parent_id")
	order, hasOrder := weWorkCallbackInt(event, "Order", "order")
	department := WorkDepartmentEventDepartment{
		WXDepartmentID: wxDepartmentID,
		Name:           name,
		HasName:        hasName && name != "",
		WXParentID:     parentID,
		HasParent:      hasParent || create,
		Order:          order,
		HasOrder:       hasOrder || create,
	}
	_, err := w.store.SyncWorkDepartment(ctx, corpID, department)
	return err
}

func (w *WeWorkCallbackWorker) deleteDepartmentFromEvent(ctx context.Context, corpID int, event WeWorkCallbackEvent) error {
	wxDepartmentID, ok := weWorkCallbackInt(event, "Id", "ID", "id")
	if !ok || wxDepartmentID <= 0 {
		return nil
	}
	_, err := w.store.DeleteWorkDepartmentByWXDepartmentID(ctx, corpID, wxDepartmentID)
	return err
}

func weWorkCallbackEmployeeUserID(event WeWorkCallbackEvent) string {
	wxUserID := strings.TrimSpace(event.Message["UserID"])
	if wxUserID == "" {
		wxUserID = strings.TrimSpace(event.Message["UserId"])
	}
	if wxUserID == "" {
		wxUserID = strings.TrimSpace(event.Message["userid"])
	}
	return wxUserID
}

func weWorkCallbackExternalUserID(event WeWorkCallbackEvent) string {
	wxExternalUserID, _ := weWorkCallbackString(event, "ExternalUserID", "ExternalUserid", "external_userid", "externalUserID")
	return wxExternalUserID
}

func weWorkCallbackString(event WeWorkCallbackEvent, keys ...string) (string, bool) {
	for _, key := range keys {
		value, ok := event.Message[key]
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return "", false
		}
		return value, true
	}
	return "", false
}

func weWorkCallbackInt(event WeWorkCallbackEvent, keys ...string) (int, bool) {
	value, ok := weWorkCallbackString(event, keys...)
	if !ok {
		return 0, false
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func (w *WeWorkCallbackWorker) syncContactTags(ctx context.Context, corpID int) error {
	credential, found, err := w.store.RoomWelcomeCorpCredentialByID(ctx, corpID)
	if err != nil {
		return err
	}
	if !found || strings.TrimSpace(credential.WXCorpID) == "" || strings.TrimSpace(credential.ContactSecret) == "" {
		return fmt.Errorf("corp contact credential is incomplete")
	}
	groups, err := w.client.CorpTags(ctx, credential)
	if err != nil {
		return err
	}
	_, err = w.store.SyncWorkContactTags(ctx, corpID, groups)
	return err
}

func (w *WeWorkCallbackWorker) syncContactTagFromEvent(ctx context.Context, corpID int, event WeWorkCallbackEvent) error {
	wxID, ok := weWorkCallbackString(event, "Id", "ID", "id")
	if !ok || wxID == "" {
		return nil
	}
	tagType, _ := weWorkCallbackString(event, "TagType", "tag_type")
	tagType = strings.TrimSpace(tagType)
	credential, found, err := w.store.RoomWelcomeCorpCredentialByID(ctx, corpID)
	if err != nil {
		return err
	}
	if !found || strings.TrimSpace(credential.WXCorpID) == "" || strings.TrimSpace(credential.ContactSecret) == "" {
		return fmt.Errorf("corp contact credential is incomplete")
	}
	var groups []WorkContactTagSyncGroup
	switch tagType {
	case "tag":
		groups, err = w.client.CorpTagsByIDs(ctx, credential, nil, []string{wxID})
	case "tag_group":
		groups, err = w.client.CorpTagsByIDs(ctx, credential, []string{wxID}, nil)
	default:
		groups, err = w.client.CorpTags(ctx, credential)
	}
	if err != nil {
		return err
	}
	for _, group := range groups {
		if tagType == "tag_group" && strings.TrimSpace(group.WXGroupID) != wxID {
			continue
		}
		if tagType == "tag" && !workContactTagGroupContainsTag(group, wxID) {
			continue
		}
		if _, err := w.store.SyncWorkContactTagGroup(ctx, corpID, group); err != nil {
			return err
		}
	}
	return nil
}

func (w *WeWorkCallbackWorker) deleteContactTagFromEvent(ctx context.Context, corpID int, event WeWorkCallbackEvent) error {
	wxID, ok := weWorkCallbackString(event, "Id", "ID", "id")
	if !ok || wxID == "" {
		return nil
	}
	tagType, _ := weWorkCallbackString(event, "TagType", "tag_type")
	switch strings.TrimSpace(tagType) {
	case "tag_group":
		_, err := w.store.DeleteWorkContactTagGroupByWXGroupID(ctx, corpID, wxID)
		return err
	default:
		_, err := w.store.DeleteWorkContactTagByWXContactTagID(ctx, corpID, wxID)
		return err
	}
}

func workContactTagGroupContainsTag(group WorkContactTagSyncGroup, wxContactTagID string) bool {
	for _, tag := range group.Tags {
		if strings.TrimSpace(tag.WXContactTagID) == wxContactTagID {
			return true
		}
	}
	return false
}

func (w *WeWorkCallbackWorker) syncContacts(ctx context.Context, corpID int) error {
	credential, found, err := w.store.RoomWelcomeCorpCredentialByID(ctx, corpID)
	if err != nil {
		return err
	}
	if !found || strings.TrimSpace(credential.WXCorpID) == "" || strings.TrimSpace(credential.ContactSecret) == "" {
		return fmt.Errorf("corp contact credential is incomplete")
	}
	employees, err := w.store.WorkContactSyncEmployees(ctx, corpID)
	if err != nil {
		return err
	}
	bundles := make([]WorkContactSyncEmployeeContacts, 0, len(employees))
	for _, employee := range employees {
		wxUserID := strings.TrimSpace(employee.WXUserID)
		if employee.ID <= 0 || wxUserID == "" {
			continue
		}
		externalUserIDs, noContact, err := w.client.ExternalContactList(ctx, credential, wxUserID)
		if err != nil {
			return err
		}
		bundle := WorkContactSyncEmployeeContacts{Employee: employee, ExternalUserIDs: uniqueNonEmptyStrings(externalUserIDs), NoContact: noContact}
		if !noContact {
			for _, wxExternalUserID := range bundle.ExternalUserIDs {
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
	_, err = w.store.SyncWorkContacts(ctx, corpID, bundles)
	return err
}

func (w *WeWorkCallbackWorker) syncContactFromEvent(ctx context.Context, corpID int, event WeWorkCallbackEvent, createMissing bool) error {
	wxUserID := weWorkCallbackEmployeeUserID(event)
	wxExternalUserID := weWorkCallbackExternalUserID(event)
	if wxUserID == "" || wxExternalUserID == "" {
		return nil
	}
	credential, found, err := w.store.RoomWelcomeCorpCredentialByID(ctx, corpID)
	if err != nil {
		return err
	}
	if !found || strings.TrimSpace(credential.WXCorpID) == "" || strings.TrimSpace(credential.ContactSecret) == "" {
		return fmt.Errorf("corp contact credential is incomplete")
	}
	employee, found, err := w.store.WorkContactSyncEmployeeByWXUserID(ctx, corpID, wxUserID)
	if err != nil {
		return err
	}
	if !found || employee.ID <= 0 {
		return nil
	}
	if strings.TrimSpace(employee.WXUserID) == "" {
		employee.WXUserID = wxUserID
	}
	contact, err := w.client.ExternalContactDetail(ctx, credential, wxExternalUserID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(contact.WXExternalUserID) == "" {
		contact.WXExternalUserID = wxExternalUserID
	}
	if strings.TrimSpace(contact.WXExternalUserID) == "" {
		return nil
	}
	result, err := w.store.SyncWorkContactForEmployee(ctx, corpID, employee, contact, createMissing)
	if err != nil {
		return err
	}
	if createMissing {
		if err := w.handleAutoTagContactTime(ctx, corpID, employee.ID, result.ContactID); err != nil {
			return err
		}
		if err := w.handleContactSOPLog(ctx, corpID, employee.ID, result.ContactID); err != nil {
			return err
		}
		if err := w.markContactTagsFromState(ctx, corpID, credential, employee.ID, result.ContactID, event); err != nil {
			return fmt.Errorf("wework callback mark contact tags failed: corp=%d employee=%d contact=%d state=%q: %w", corpID, employee.ID, result.ContactID, contactWelcomeState(event), err)
		}
		if err := w.handleFissionAddContactFromState(ctx, corpID, credential, employee, contact, result, event); err != nil {
			w.logger.Printf("wework callback work fission add contact skipped: corp=%d employee=%d contact=%d state=%q err=%v", corpID, employee.ID, result.ContactID, contactWelcomeState(event), errors.New(SanitizeWeWorkCallbackFailure(err.Error())))
			if _, durable := WeWorkCallbackExecutionFromContext(ctx); durable {
				return err
			}
		}
		return w.enqueueGenericContactWelcome(ctx, corpID, employee.ID, contact.Name, result.ContactID, event)
	}
	return nil
}

func (w *WeWorkCallbackWorker) handleAutoTagContactTime(ctx context.Context, corpID int, employeeID int, contactID int) error {
	result, err := w.store.AutoTagContactTimeTask(ctx, corpID, employeeID, contactID)
	if err != nil {
		return err
	}
	if len(result.MarkTagsEvents) == 0 {
		return nil
	}
	capabilities, err := w.resolveCapabilities(ctx)
	if err != nil {
		return err
	}
	enqueuer := capabilities.MarkTagsQueue
	if enqueuer == nil {
		return nil
	}
	for _, event := range result.MarkTagsEvents {
		if err := enqueuer.EnqueueMarkTags(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

func (w *WeWorkCallbackWorker) handleContactSOPLog(ctx context.Context, corpID int, employeeID int, contactID int) error {
	cron := NewSOPLogCron(w.store, w.logger)
	cron.now = w.now
	result, err := cron.RunContactTarget(ctx, corpID, employeeID, contactID)
	if err != nil {
		return err
	}
	if result.ContactLogsInserted > 0 {
		w.logger.Printf("wework callback generated contact SOP logs: corp=%d employee=%d contact=%d inserted=%d", corpID, employeeID, contactID, result.ContactLogsInserted)
	}
	return nil
}

func (w *WeWorkCallbackWorker) handleFissionAddContactFromState(ctx context.Context, corpID int, credential RoomWelcomeCorpCredential, employee WorkContactSyncEmployee, contact WorkContactSyncContact, result WorkContactSyncResult, event WeWorkCallbackEvent) error {
	parentContactID, ok := fissionIDFromContactWelcomeState(event)
	if !ok || parentContactID <= 0 || strings.TrimSpace(contact.WXExternalUserID) == "" {
		return nil
	}
	fissionResult, _, err := w.store.HandleWorkFissionAddContact(ctx, WorkFissionAddContactEvent{
		CorpID:           corpID,
		ParentContactID:  parentContactID,
		EmployeeID:       employee.ID,
		EmployeeWXUserID: employee.WXUserID,
		WXExternalUserID: contact.WXExternalUserID,
		UnionID:          contact.UnionID,
		Name:             contact.Name,
		Avatar:           contact.Avatar,
		IsNew:            result.ContactWasNew,
	})
	if err != nil {
		return err
	}
	reminderErr := w.sendWorkFissionEmployeeReminder(ctx, corpID, fissionResult)
	customerErr := w.sendWorkFissionCustomerPush(ctx, credential, fissionResult)
	return errors.Join(reminderErr, customerErr)
}

func (w *WeWorkCallbackWorker) sendWorkFissionEmployeeReminder(ctx context.Context, corpID int, result WorkFissionAddContactResult) error {
	if !result.Completed || result.EmployeeReminder == nil || strings.TrimSpace(result.EmployeeReminder.ToUser) == "" || strings.TrimSpace(result.EmployeeReminder.Content) == "" {
		return nil
	}
	agent, found, err := w.store.RoomTagPullRemindAgentByCorpID(ctx, corpID)
	if err != nil {
		return err
	}
	if !found || strings.TrimSpace(agent.WXCorpID) == "" || strings.TrimSpace(agent.WXAgentID) == "" || strings.TrimSpace(agent.WXSecret) == "" {
		return fmt.Errorf("work fission employee reminder agent is incomplete")
	}
	execute, complete, err := w.beginDurableSideEffect(ctx, WeWorkCallbackActionFissionEmployeeReminder, result.EmployeeReminder)
	if err != nil || !execute {
		return err
	}
	if err := w.client.SendAgentTextMessageWithDuplicateCheck(ctx, agent, result.EmployeeReminder.ToUser, result.EmployeeReminder.Content); err != nil {
		return weWorkCallbackSideEffectFailure(ctx, err)
	}
	return complete(ctx)
}

func (w *WeWorkCallbackWorker) sendWorkFissionCustomerPush(ctx context.Context, credential RoomWelcomeCorpCredential, result WorkFissionAddContactResult) error {
	if !result.Completed || result.CustomerPush == nil || strings.TrimSpace(result.CustomerPush.Sender) == "" || strings.TrimSpace(result.CustomerPush.ExternalUserID) == "" || len(result.CustomerPush.Content) == 0 {
		return nil
	}
	execute, complete, err := w.beginDurableSideEffect(ctx, WeWorkCallbackActionFissionCustomerPush, result.CustomerPush)
	if err != nil || !execute {
		return err
	}
	content, err := prepareContactMessageBatchSendContent(ctx, w.client, credential, w.fileStorageRoot, result.CustomerPush.Content)
	if err != nil {
		return weWorkCallbackSideEffectFailure(ctx, err)
	}
	if len(content) == 0 {
		return weWorkCallbackSideEffectFailure(ctx, errors.New("work fission customer push content is empty"))
	}
	_, err = w.client.SubmitContactMessageBatchSend(ctx, credential, ContactMessageBatchSendMessagePayload{
		Content:        content,
		Sender:         result.CustomerPush.Sender,
		ExternalUserID: []string{result.CustomerPush.ExternalUserID},
	})
	if err != nil {
		return weWorkCallbackSideEffectFailure(ctx, err)
	}
	return complete(ctx)
}

func weWorkCallbackSideEffectFailure(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if _, durable := WeWorkCallbackExecutionFromContext(ctx); !durable {
		return err
	}
	return errors.Join(ErrWeWorkCallbackSideEffectReconcileRequired, err)
}

func (w *WeWorkCallbackWorker) beginDurableSideEffect(ctx context.Context, actionKey string, payload any) (bool, func(context.Context) error, error) {
	execution, durable := WeWorkCallbackExecutionFromContext(ctx)
	if !durable {
		return true, func(context.Context) error { return nil }, nil
	}
	if execution.TenantID <= 0 || execution.CorpID <= 0 {
		return false, nil, errors.Join(ErrWeWorkCallbackSideEffectReconcileRequired, errors.New("callback side effect scope is incomplete"))
	}
	store, ok := w.store.(WeWorkCallbackSideEffectStore)
	if !ok {
		return false, nil, errors.Join(ErrWeWorkCallbackSideEffectReconcileRequired, errors.New("callback side effect store is unavailable"))
	}
	payloadHash, err := WeWorkCallbackSideEffectPayloadHash(actionKey, payload)
	if err != nil {
		return false, nil, err
	}
	execute, status, err := store.BeginWeWorkCallbackSideEffect(ctx, execution, actionKey, payloadHash)
	if err != nil {
		return false, nil, err
	}
	if !execute {
		switch status {
		case WeWorkCallbackSideEffectSent:
			return false, nil, nil
		case WeWorkCallbackSideEffectUnknown:
			return false, nil, ErrWeWorkCallbackSideEffectReconcileRequired
		default:
			return false, nil, errors.Join(ErrWeWorkCallbackSideEffectReconcileRequired, fmt.Errorf("invalid callback side effect status %q", status))
		}
	}
	if status != WeWorkCallbackSideEffectUnknown {
		return false, nil, errors.Join(ErrWeWorkCallbackSideEffectReconcileRequired, fmt.Errorf("callback side effect did not enter unknown state: %q", status))
	}
	complete := func(completeCtx context.Context) error {
		if err := store.CompleteWeWorkCallbackSideEffect(completeCtx, execution, actionKey, payloadHash); err != nil {
			return errors.Join(ErrWeWorkCallbackSideEffectReconcileRequired, err)
		}
		return nil
	}
	return true, complete, nil
}

func (w *WeWorkCallbackWorker) markContactTagsFromState(ctx context.Context, corpID int, credential RoomWelcomeCorpCredential, employeeID int, contactID int, event WeWorkCallbackEvent) error {
	if contactID <= 0 || employeeID <= 0 {
		return nil
	}
	tagIDs, found, err := w.contactTagIDsFromState(ctx, event)
	if err != nil || !found || len(tagIDs) == 0 {
		return err
	}
	result, found, err := w.store.UpdateWorkContactProfile(ctx, WorkContactUpdateValues{
		CorpID:     corpID,
		ContactID:  contactID,
		EmployeeID: employeeID,
		HasTag:     true,
		TagIDs:     tagIDs,
	})
	if err != nil || !found {
		return err
	}
	if len(result.AddedWXTagIDs) > 0 {
		if strings.TrimSpace(result.WXUserID) == "" || strings.TrimSpace(result.WXExternalUserID) == "" {
			return fmt.Errorf("work contact tag sync identity is incomplete")
		}
		if err := w.client.MarkExternalContactTags(ctx, credential, WorkContactMarkTagsPayload{
			UserID:         result.WXUserID,
			ExternalUserID: result.WXExternalUserID,
			AddTag:         result.AddedWXTagIDs,
		}); err != nil {
			return err
		}
	}
	if len(result.UnsyncableTagIDs) > 0 {
		return fmt.Errorf("work contact tags are missing WeCom mappings: %v", result.UnsyncableTagIDs)
	}
	return nil
}

func (w *WeWorkCallbackWorker) contactTagIDsFromState(ctx context.Context, event WeWorkCallbackEvent) ([]int, bool, error) {
	if channelCodeID, ok := channelCodeIDFromContactWelcomeState(event); ok {
		welcome, found, err := w.store.ChannelCodeWelcomeByID(ctx, channelCodeID)
		return append([]int{}, welcome.TagIDs...), found, err
	}
	if workRoomAutoPullID, ok := workRoomAutoPullIDFromContactWelcomeState(event); ok {
		welcome, found, err := w.store.WorkRoomAutoPullWelcomeByID(ctx, workRoomAutoPullID)
		return append([]int{}, welcome.TagIDs...), found, err
	}
	if fissionID, ok := fissionIDFromContactWelcomeState(event); ok {
		rule, found, err := w.store.WorkFissionContactRuleByID(ctx, fissionID)
		return append([]int{}, rule.TagIDs...), found, err
	}
	return nil, false, nil
}

func (w *WeWorkCallbackWorker) enqueueGenericContactWelcome(ctx context.Context, corpID int, employeeID int, contactName string, contactID int, event WeWorkCallbackEvent) error {
	welcomeCode, _ := weWorkCallbackString(event, "WelcomeCode", "welcomeCode", "welcome_code")
	welcomeCode = strings.TrimSpace(welcomeCode)
	if contactID <= 0 || employeeID <= 0 || welcomeCode == "" {
		return nil
	}
	content, found, err := w.selectContactWelcomeContent(ctx, corpID, employeeID, event)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	capabilities, err := w.resolveCapabilities(ctx)
	if err != nil {
		return err
	}
	cache := capabilities.ContactWelcomeCache
	if cache != nil {
		status, err := cache.WorkContactWelcomeStatus(ctx, contactID)
		if err != nil {
			return err
		}
		if status == 1 {
			return nil
		}
	}
	enqueuer := capabilities.ContactWelcomeQueue
	if enqueuer == nil {
		return nil
	}
	welcome := ContactWelcomeEvent{
		CorpID:      corpID,
		ContactID:   contactID,
		EmployeeID:  employeeID,
		ContactName: contactName,
		WelcomeCode: welcomeCode,
		Content:     content,
	}
	if err := enqueuer.EnqueueContactWelcome(ctx, welcome); err != nil {
		return err
	}
	if cache != nil {
		return cache.SetWorkContactWelcomeStatus(ctx, contactID, 1, contactWelcomeStatusTTL)
	}
	return nil
}

func (w *WeWorkCallbackWorker) selectContactWelcomeContent(ctx context.Context, corpID int, employeeID int, event WeWorkCallbackEvent) (ContactWelcomeContent, bool, error) {
	if channelCodeID, ok := channelCodeIDFromContactWelcomeState(event); ok {
		now := time.Now()
		if w.now != nil {
			now = w.now()
		}
		content, found, err := selectChannelCodeContactWelcomeContent(ctx, w.store, channelCodeID, now)
		if err != nil || found {
			return content, found, err
		}
	}
	if workRoomAutoPullID, ok := workRoomAutoPullIDFromContactWelcomeState(event); ok {
		content, found, err := selectWorkRoomAutoPullContactWelcomeContent(ctx, w.store, workRoomAutoPullID)
		if err != nil || found {
			return content, found, err
		}
	}
	if fissionParentID, ok := fissionIDFromContactWelcomeState(event); ok {
		content, found, err := selectWorkFissionContactWelcomeContent(ctx, w.store, fissionParentID, w.workFissionAuthRedirectURL)
		if err != nil || found {
			return content, found, err
		}
	}
	return selectGenericContactWelcomeContent(ctx, w.store, corpID, employeeID)
}

func (w *WeWorkCallbackWorker) workFissionAuthRedirectURL(id int) string {
	if id <= 0 {
		return ""
	}
	baseURL := w.operationBaseURL
	if baseURL == "" {
		baseURL = w.apiBaseURL
	}
	if baseURL == "" {
		return ""
	}
	target := fmt.Sprintf("/workFission?id=%d", id)
	return fmt.Sprintf("%s/auth/workFission?id=%d&target=%s", baseURL, id, url.QueryEscape(target))
}

func contactWelcomeState(event WeWorkCallbackEvent) string {
	state, _ := weWorkCallbackString(event, "State", "state")
	return strings.TrimSpace(state)
}

func channelCodeIDFromContactWelcomeState(event WeWorkCallbackEvent) (int, bool) {
	state := contactWelcomeState(event)
	if state == "" {
		return 0, false
	}
	parts := strings.SplitN(state, "-", 2)
	if len(parts) != 2 || parts[0] != "channelCode" {
		return 0, false
	}
	channelCodeID, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || channelCodeID <= 0 {
		return 0, false
	}
	return channelCodeID, true
}

func workRoomAutoPullIDFromContactWelcomeState(event WeWorkCallbackEvent) (int, bool) {
	state := contactWelcomeState(event)
	if state == "" {
		return 0, false
	}
	parts := strings.SplitN(state, "-", 2)
	if len(parts) != 2 || parts[0] != "workRoomAutoPullId" {
		return 0, false
	}
	id, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func fissionIDFromContactWelcomeState(event WeWorkCallbackEvent) (int, bool) {
	state := contactWelcomeState(event)
	if state == "" {
		return 0, false
	}
	parts := strings.SplitN(state, "-", 2)
	if len(parts) != 2 || parts[0] != "fission" {
		return 0, false
	}
	id, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func (w *WeWorkCallbackWorker) removeContactRelationFromEvent(ctx context.Context, corpID int, event WeWorkCallbackEvent, status int) error {
	wxUserID := weWorkCallbackEmployeeUserID(event)
	wxExternalUserID := weWorkCallbackExternalUserID(event)
	if wxUserID == "" || wxExternalUserID == "" {
		return nil
	}
	result, removed, err := w.store.RemoveWorkContactEmployeeRelation(ctx, corpID, wxUserID, wxExternalUserID, status)
	if err != nil || !removed {
		return err
	}
	return w.sendWorkContactRemovalReminder(ctx, corpID, result)
}

func (w *WeWorkCallbackWorker) sendWorkContactRemovalReminder(ctx context.Context, corpID int, result WorkContactRemovalResult) error {
	if result.ContactID <= 0 || strings.TrimSpace(result.WXUserID) == "" {
		return nil
	}
	agent, found, err := w.store.RoomTagPullRemindAgentByCorpID(ctx, corpID)
	if err != nil {
		return err
	}
	if !found || strings.TrimSpace(agent.WXCorpID) == "" || strings.TrimSpace(agent.WXAgentID) == "" || strings.TrimSpace(agent.WXSecret) == "" {
		return fmt.Errorf("work contact removal reminder agent is incomplete")
	}
	return w.client.SendAgentTextMessageWithDuplicateCheck(ctx, agent, result.WXUserID, w.workContactRemovalReminderContent(result))
}

func (w *WeWorkCallbackWorker) workContactRemovalReminderContent(result WorkContactRemovalResult) string {
	title := "【删除提醒】您已将客户删除哦！"
	if result.Status == workContactEmployeeStatusPassiveRemoved {
		title = "【流失提醒】有客户将您删除哦！"
	}
	link := w.workContactRemovalSidebarURL(result.ContactID)
	return fmt.Sprintf("%s\n客户昵称：%s\n建议重新添加，或者分享给同事哦\n<a href='%s'>点击查看客户详情</a>", title, result.ContactName, link)
}

func (w *WeWorkCallbackWorker) workContactRemovalSidebarURL(contactID int) string {
	baseURL := w.sidebarBaseURL
	if baseURL == "" {
		baseURL = w.apiBaseURL
	}
	if baseURL == "" || contactID <= 0 {
		return ""
	}
	return fmt.Sprintf("%s/contact?agentId={{agentId}}&contactId=%d", baseURL, contactID)
}

func (w *WeWorkCallbackWorker) syncRoomFromEvent(ctx context.Context, corpID int, event WeWorkCallbackEvent) error {
	wxChatID := strings.TrimSpace(event.Message["ChatId"])
	if wxChatID == "" {
		wxChatID = strings.TrimSpace(event.Message["ChatID"])
	}
	if wxChatID == "" {
		return nil
	}
	credential, found, err := w.store.RoomWelcomeCorpCredentialByID(ctx, corpID)
	if err != nil {
		return err
	}
	if !found || strings.TrimSpace(credential.WXCorpID) == "" || strings.TrimSpace(credential.ContactSecret) == "" {
		return fmt.Errorf("corp contact credential is incomplete")
	}
	room, err := w.client.GroupChatDetail(ctx, credential, wxChatID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(room.WXChatID) == "" {
		room.WXChatID = wxChatID
	}
	groupChats, err := w.client.GroupChats(ctx, credential)
	if err != nil {
		return err
	}
	for _, groupChat := range groupChats {
		if strings.TrimSpace(groupChat.WXChatID) == wxChatID {
			room.Status = groupChat.Status
			break
		}
	}
	if strings.TrimSpace(room.WXChatID) == "" {
		return nil
	}
	if _, err := w.store.SyncWorkRoom(ctx, corpID, room); err != nil {
		return err
	}
	if err := w.handleAutoTagRoomJoin(ctx, corpID, wxChatID); err != nil {
		return err
	}
	return w.handleRoomJoinSOPLog(ctx, corpID, wxChatID)
}

func (w *WeWorkCallbackWorker) handleAutoTagRoomJoin(ctx context.Context, corpID int, wxChatID string) error {
	result, err := w.store.AutoTagRoomJoinTask(ctx, corpID, wxChatID)
	if err != nil {
		return err
	}
	if len(result.MarkTagsEvents) == 0 {
		return nil
	}
	capabilities, err := w.resolveCapabilities(ctx)
	if err != nil {
		return err
	}
	enqueuer := capabilities.MarkTagsQueue
	if enqueuer == nil {
		return nil
	}
	for _, event := range result.MarkTagsEvents {
		if err := enqueuer.EnqueueMarkTags(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

func (w *WeWorkCallbackWorker) handleRoomJoinSOPLog(ctx context.Context, corpID int, wxChatID string) error {
	cron := NewSOPLogCron(w.store, w.logger)
	cron.now = w.now
	result, err := cron.RunRoomJoinTargetByWXChatID(ctx, corpID, wxChatID)
	if err != nil {
		return err
	}
	if result.RoomLogsInserted > 0 {
		w.logger.Printf("wework callback generated room SOP logs: corp=%d wx_chat_id=%s inserted=%d", corpID, wxChatID, result.RoomLogsInserted)
	}
	return nil
}

func (w *WeWorkCallbackWorker) syncRooms(ctx context.Context, corpID int) error {
	credential, found, err := w.store.RoomWelcomeCorpCredentialByID(ctx, corpID)
	if err != nil {
		return err
	}
	if !found || strings.TrimSpace(credential.WXCorpID) == "" || strings.TrimSpace(credential.ContactSecret) == "" {
		return fmt.Errorf("corp contact credential is incomplete")
	}
	groupChats, err := w.client.GroupChats(ctx, credential)
	if err != nil {
		return err
	}
	groupChats = uniqueWorkRoomSyncGroupChats(groupChats)
	rooms := make([]WorkRoomSyncRoom, 0, len(groupChats))
	for _, groupChat := range groupChats {
		room, err := w.client.GroupChatDetail(ctx, credential, groupChat.WXChatID)
		if err != nil {
			return err
		}
		if strings.TrimSpace(room.WXChatID) == "" {
			room.WXChatID = groupChat.WXChatID
		}
		room.Status = groupChat.Status
		if strings.TrimSpace(room.WXChatID) != "" {
			rooms = append(rooms, room)
		}
	}
	_, err = w.store.SyncWorkRooms(ctx, corpID, rooms)
	return err
}
