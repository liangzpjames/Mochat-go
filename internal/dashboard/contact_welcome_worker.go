package dashboard

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"jiyi/mochat-go/internal/taskrunner"
)

const contactWelcomeStatusTTL = time.Minute

type ContactWelcomeEvent struct {
	CorpID      int                   `json:"corpId"`
	ContactID   int                   `json:"contactId"`
	EmployeeID  int                   `json:"employeeId"`
	ContactName string                `json:"contactName"`
	WelcomeCode string                `json:"welcomeCode"`
	Content     ContactWelcomeContent `json:"content"`
}

type ContactWelcomeContent struct {
	Text   string                `json:"text,omitempty"`
	Medium *ContactWelcomeMedium `json:"medium,omitempty"`
}

type ContactWelcomeMedium struct {
	MediumType    int            `json:"mediumType"`
	MediumContent map[string]any `json:"mediumContent,omitempty"`
}

type ContactWelcomePayload struct {
	Text        *ContactWelcomeText        `json:"text,omitempty"`
	Attachments []ContactWelcomeAttachment `json:"attachments,omitempty"`
}

type ContactWelcomeText struct {
	Content string `json:"content"`
}

type ContactWelcomeAttachment struct {
	MsgType     string                     `json:"msgtype"`
	Image       *ContactWelcomeImage       `json:"image,omitempty"`
	Link        *ContactWelcomeLink        `json:"link,omitempty"`
	MiniProgram *ContactWelcomeMiniProgram `json:"miniprogram,omitempty"`
}

type ContactWelcomeImage struct {
	MediaID string `json:"media_id"`
}

type ContactWelcomeLink struct {
	Title  string `json:"title"`
	PicURL string `json:"picurl,omitempty"`
	Desc   string `json:"desc,omitempty"`
	URL    string `json:"url"`
}

type ContactWelcomeMiniProgram struct {
	Title      string `json:"title"`
	PicMediaID string `json:"pic_media_id"`
	AppID      string `json:"appid"`
	Page       string `json:"page"`
}

type ContactWelcomeWorkerQueue interface {
	DequeueContactWelcome(ctx context.Context, timeout time.Duration) (ContactWelcomeDelivery, bool, error)
	AckContactWelcome(ctx context.Context, delivery ContactWelcomeDelivery) error
	RetryContactWelcome(ctx context.Context, delivery ContactWelcomeDelivery, reason string, maxAttempts int) (bool, error)
	RecoverContactWelcomeProcessing(ctx context.Context, staleAfter time.Duration, maxAttempts int) (int, error)
}

type ContactWelcomeEnqueuer interface {
	EnqueueContactWelcome(ctx context.Context, event ContactWelcomeEvent) error
}

type ContactWelcomeDelivery struct {
	Event    ContactWelcomeEvent
	Raw      string
	Attempts int
}

type ContactWelcomeStore interface {
	RoomWelcomeCorpCredentialByID(ctx context.Context, corpID int) (RoomWelcomeCorpCredential, bool, error)
}

type GenericContactWelcomeStore interface {
	GreetingsByCorp(ctx context.Context, corpID int) ([]GreetingItem, error)
	GreetingMediaByIDs(ctx context.Context, mediumIDs []int) (map[int]GreetingMedium, error)
}

type ChannelCodeWelcome struct {
	ID             int
	WelcomeMessage map[string]any
	TagIDs         []int
}

type ChannelCodeWelcomeStore interface {
	ChannelCodeWelcomeByID(ctx context.Context, channelCodeID int) (ChannelCodeWelcome, bool, error)
}

type ChannelCodeContactWelcomeStore interface {
	ChannelCodeWelcomeStore
	GreetingMediaByIDs(ctx context.Context, mediumIDs []int) (map[int]GreetingMedium, error)
}

type WorkRoomAutoPullWelcome struct {
	ID           int
	LeadingWords string
	Rooms        []WorkRoomAutoPullWelcomeRoom
	TagIDs       []int
}

type WorkRoomAutoPullWelcomeRoom struct {
	RoomID        int
	MaxNum        int
	RoomMax       int
	MemberNum     int
	RoomQRCodeURL string
}

type WorkRoomAutoPullWelcomeStore interface {
	WorkRoomAutoPullWelcomeByID(ctx context.Context, id int) (WorkRoomAutoPullWelcome, bool, error)
}

type WorkFissionContactWelcome struct {
	ParentContactID int
	FissionID       int
	MsgText         string
	LinkTitle       string
	LinkDesc        string
	LinkCoverURL    string
}

type WorkFissionContactWelcomeStore interface {
	WorkFissionContactWelcomeByParentID(ctx context.Context, parentContactID int) (WorkFissionContactWelcome, bool, error)
}

type ContactWelcomeStatusCache interface {
	WorkContactWelcomeStatus(ctx context.Context, contactID int) (int, error)
	SetWorkContactWelcomeStatus(ctx context.Context, contactID int, status int, ttl time.Duration) error
}

type ContactWelcomeClient interface {
	UploadTemporaryImage(ctx context.Context, credential RoomWelcomeCorpCredential, filePath string) (string, error)
	SendExternalContactWelcome(ctx context.Context, credential RoomWelcomeCorpCredential, welcomeCode string, payload ContactWelcomePayload) error
}

type ContactWelcomeWorker struct {
	queue               ContactWelcomeWorkerQueue
	store               ContactWelcomeStore
	client              ContactWelcomeClient
	fileStorageRoot     string
	apiBaseURL          string
	pollTimeout         time.Duration
	maxAttempts         int
	processingTimeout   time.Duration
	recoveryInterval    time.Duration
	dependencyRetryBase time.Duration
	dependencyRetryMax  time.Duration
	alertNotifier       SaaSAlertNotifier
	logger              *log.Logger
}

func NewContactWelcomeWorker(queue ContactWelcomeWorkerQueue, store ContactWelcomeStore, client ContactWelcomeClient, fileStorageRoot string, apiBaseURL string, logger *log.Logger) *ContactWelcomeWorker {
	if logger == nil {
		logger = log.Default()
	}
	if strings.TrimSpace(fileStorageRoot) == "" {
		fileStorageRoot = defaultMediumFileStorageRoot
	}
	return &ContactWelcomeWorker{
		queue:               queue,
		store:               store,
		client:              client,
		fileStorageRoot:     fileStorageRoot,
		apiBaseURL:          strings.TrimRight(strings.TrimSpace(apiBaseURL), "/"),
		pollTimeout:         5 * time.Second,
		maxAttempts:         3,
		processingTimeout:   5 * time.Minute,
		recoveryInterval:    time.Minute,
		dependencyRetryBase: time.Second,
		dependencyRetryMax:  30 * time.Second,
		logger:              logger,
	}
}

func (w *ContactWelcomeWorker) WithProcessingTimeout(timeout time.Duration) *ContactWelcomeWorker {
	if timeout > 0 {
		w.processingTimeout = timeout
		w.recoveryInterval = timeout
		if w.recoveryInterval > time.Minute {
			w.recoveryInterval = time.Minute
		}
	}
	return w
}

func (w *ContactWelcomeWorker) WithSaaSAlertNotifier(notifier SaaSAlertNotifier) *ContactWelcomeWorker {
	w.alertNotifier = notifier
	return w
}

func (w *ContactWelcomeWorker) Run(ctx context.Context) error {
	if w.queue == nil || w.store == nil || w.client == nil {
		return fmt.Errorf("contact welcome worker dependencies are not configured")
	}
	nextRecovery := time.Now()
	consecutiveDependencyFailures := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if !time.Now().Before(nextRecovery) {
			if err := w.recoverProcessing(ctx); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				consecutiveDependencyFailures++
				delay := contactWelcomeDependencyRetryDelay(w.dependencyRetryBase, w.dependencyRetryMax, consecutiveDependencyFailures-1)
				w.logger.Printf("event=contact_welcome_queue_dependency_degraded component=contact_welcome_worker operation=recover dependency=redis result=retrying failure_count=%d retry_in=%s", consecutiveDependencyFailures, delay)
				if err := waitContactWelcomeDependency(ctx, delay); err != nil {
					return err
				}
				continue
			}
			nextRecovery = time.Now().Add(w.recoveryInterval)
			if consecutiveDependencyFailures > 0 {
				w.logger.Printf("event=contact_welcome_queue_dependency_recovered component=contact_welcome_worker dependency=redis result=recovered failure_count=%d", consecutiveDependencyFailures)
				consecutiveDependencyFailures = 0
			}
		}
		delivery, ok, err := w.queue.DequeueContactWelcome(ctx, w.pollTimeout)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			consecutiveDependencyFailures++
			delay := contactWelcomeDependencyRetryDelay(w.dependencyRetryBase, w.dependencyRetryMax, consecutiveDependencyFailures-1)
			w.logger.Printf("event=contact_welcome_queue_dependency_degraded component=contact_welcome_worker operation=dequeue dependency=redis result=retrying failure_count=%d retry_in=%s", consecutiveDependencyFailures, delay)
			if err := waitContactWelcomeDependency(ctx, delay); err != nil {
				return err
			}
			continue
		}
		if consecutiveDependencyFailures > 0 {
			w.logger.Printf("event=contact_welcome_queue_dependency_recovered component=contact_welcome_worker dependency=redis result=recovered failure_count=%d", consecutiveDependencyFailures)
			consecutiveDependencyFailures = 0
		}
		if !ok {
			continue
		}
		w.handleDelivery(ctx, delivery)
	}
}

func (w *ContactWelcomeWorker) recoverProcessing(ctx context.Context) error {
	recovered, err := w.queue.RecoverContactWelcomeProcessing(ctx, w.processingTimeout, w.maxAttempts)
	if err != nil {
		return err
	}
	if recovered > 0 {
		w.logger.Printf("contact welcome recovered processing jobs: %d", recovered)
	}
	return nil
}

func contactWelcomeDependencyRetryDelay(base time.Duration, maximum time.Duration, failureIndex int) time.Duration {
	if base <= 0 {
		base = time.Second
	}
	if maximum < base {
		maximum = base
	}
	delay := base
	for index := 0; index < failureIndex; index++ {
		if delay >= maximum/2 {
			return maximum
		}
		delay *= 2
	}
	if delay > maximum {
		return maximum
	}
	return delay
}

func waitContactWelcomeDependency(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (w *ContactWelcomeWorker) handleDelivery(ctx context.Context, delivery ContactWelcomeDelivery) {
	ctx = WithSaaSAlertNotifier(ctx, w.alertNotifier)
	tenantID := tenantIDForQueueExecution(ctx, w.logger, w.store, delivery.Event.CorpID)
	finishExecution := startQueueItemExecution(ctx, w.logger, QueueNameContactWelcome, w.store, tenantID)
	if err := w.Process(ctx, delivery.Event); err != nil {
		deadLettered, retryErr := w.queue.RetryContactWelcome(ctx, delivery, err.Error(), w.maxAttempts)
		if retryErr != nil {
			finishExecution(taskrunner.StatusFailed, fmt.Errorf("%w; retry failed: %v", err, retryErr))
			w.logger.Printf("contact welcome retry failed: corp=%d contact=%d attempts=%d err=%v retry_err=%v", delivery.Event.CorpID, delivery.Event.ContactID, delivery.Attempts+1, err, retryErr)
			return
		}
		finishExecution(taskrunner.StatusFailed, err)
		if deadLettered {
			w.logger.Printf("contact welcome moved to dead letter: corp=%d contact=%d attempts=%d err=%v", delivery.Event.CorpID, delivery.Event.ContactID, delivery.Attempts+1, err)
			return
		}
		w.logger.Printf("contact welcome requeued: corp=%d contact=%d attempts=%d err=%v", delivery.Event.CorpID, delivery.Event.ContactID, delivery.Attempts+1, err)
		return
	}
	if err := w.queue.AckContactWelcome(ctx, delivery); err != nil {
		finishExecution(taskrunner.StatusFailed, fmt.Errorf("ack failed: %w", err))
		w.logger.Printf("contact welcome ack failed: corp=%d contact=%d err=%v", delivery.Event.CorpID, delivery.Event.ContactID, err)
		return
	}
	finishExecution(taskrunner.StatusSucceeded, nil)
}

func (w *ContactWelcomeWorker) Process(ctx context.Context, event ContactWelcomeEvent) error {
	if event.CorpID <= 0 || event.ContactID <= 0 || strings.TrimSpace(event.WelcomeCode) == "" {
		return nil
	}
	credential, found, err := w.store.RoomWelcomeCorpCredentialByID(ctx, event.CorpID)
	if err != nil {
		return err
	}
	if !found || strings.TrimSpace(credential.WXCorpID) == "" || strings.TrimSpace(credential.ContactSecret) == "" {
		return fmt.Errorf("corp contact credential is incomplete")
	}
	payload, err := contactWelcomePayload(ctx, w.client, credential, event, w.fileStorageRoot, w.apiBaseURL)
	if err != nil {
		return err
	}
	if contactWelcomePayloadEmpty(payload) {
		return nil
	}
	return w.client.SendExternalContactWelcome(ctx, credential, strings.TrimSpace(event.WelcomeCode), payload)
}

func selectGenericContactWelcomeContent(ctx context.Context, store GenericContactWelcomeStore, corpID int, employeeID int) (ContactWelcomeContent, bool, error) {
	if store == nil || corpID <= 0 || employeeID <= 0 {
		return ContactWelcomeContent{}, false, nil
	}
	greetings, err := store.GreetingsByCorp(ctx, corpID)
	if err != nil {
		return ContactWelcomeContent{}, false, err
	}
	var common GreetingItem
	var assigned GreetingItem
	for _, greeting := range greetings {
		switch greeting.RangeType {
		case 1:
			common = greeting
		case 2:
			if intSliceContains(greeting.EmployeeIDs, employeeID) {
				assigned = greeting
			}
		}
	}
	selected := assigned
	if selected.ID == 0 {
		selected = common
	}
	if selected.ID == 0 {
		return ContactWelcomeContent{}, false, nil
	}
	content := ContactWelcomeContent{Text: selected.Words}
	if selected.MediumID > 0 {
		media, err := store.GreetingMediaByIDs(ctx, []int{selected.MediumID})
		if err != nil {
			return ContactWelcomeContent{}, false, err
		}
		if medium, ok := media[selected.MediumID]; ok {
			content.Medium = &ContactWelcomeMedium{
				MediumType:    medium.Type,
				MediumContent: cloneMap(medium.Content),
			}
		}
	}
	if strings.TrimSpace(content.Text) == "" && (content.Medium == nil || content.Medium.MediumType == 0) {
		return ContactWelcomeContent{}, false, nil
	}
	return content, true, nil
}

func selectWorkRoomAutoPullContactWelcomeContent(ctx context.Context, store WorkRoomAutoPullWelcomeStore, id int) (ContactWelcomeContent, bool, error) {
	if store == nil || id <= 0 {
		return ContactWelcomeContent{}, false, nil
	}
	welcome, found, err := store.WorkRoomAutoPullWelcomeByID(ctx, id)
	if err != nil {
		return ContactWelcomeContent{}, false, err
	}
	if !found {
		return ContactWelcomeContent{}, false, nil
	}
	content := ContactWelcomeContent{Text: welcome.LeadingWords}
	for _, room := range welcome.Rooms {
		if strings.TrimSpace(room.RoomQRCodeURL) == "" || room.MaxNum <= 0 || room.RoomMax <= 0 {
			continue
		}
		if room.MemberNum >= room.MaxNum || room.MemberNum >= room.RoomMax {
			continue
		}
		content.Medium = &ContactWelcomeMedium{
			MediumType: 2,
			MediumContent: map[string]any{
				"imagePath": room.RoomQRCodeURL,
			},
		}
		break
	}
	if strings.TrimSpace(content.Text) == "" && (content.Medium == nil || content.Medium.MediumType == 0) {
		return ContactWelcomeContent{}, false, nil
	}
	return content, true, nil
}

func selectWorkFissionContactWelcomeContent(ctx context.Context, store WorkFissionContactWelcomeStore, parentContactID int, authRedirectURL func(int) string) (ContactWelcomeContent, bool, error) {
	if store == nil || parentContactID <= 0 {
		return ContactWelcomeContent{}, false, nil
	}
	welcome, found, err := store.WorkFissionContactWelcomeByParentID(ctx, parentContactID)
	if err != nil {
		return ContactWelcomeContent{}, false, err
	}
	if !found || welcome.FissionID <= 0 {
		return ContactWelcomeContent{}, false, nil
	}
	content := ContactWelcomeContent{Text: welcome.MsgText}
	if authRedirectURL != nil {
		linkURL := strings.TrimSpace(authRedirectURL(welcome.FissionID))
		if strings.TrimSpace(welcome.LinkTitle) != "" && linkURL != "" {
			content.Medium = &ContactWelcomeMedium{
				MediumType: 3,
				MediumContent: map[string]any{
					"title":       welcome.LinkTitle,
					"description": welcome.LinkDesc,
					"imagePath":   welcome.LinkCoverURL,
					"imageLink":   linkURL,
				},
			}
		}
	}
	if strings.TrimSpace(content.Text) == "" && (content.Medium == nil || content.Medium.MediumType == 0) {
		return ContactWelcomeContent{}, false, nil
	}
	return content, true, nil
}

func selectChannelCodeContactWelcomeContent(ctx context.Context, store ChannelCodeContactWelcomeStore, channelCodeID int, now time.Time) (ContactWelcomeContent, bool, error) {
	if store == nil || channelCodeID <= 0 {
		return ContactWelcomeContent{}, false, nil
	}
	welcome, found, err := store.ChannelCodeWelcomeByID(ctx, channelCodeID)
	if err != nil {
		return ContactWelcomeContent{}, false, err
	}
	if !found || len(welcome.WelcomeMessage) == 0 {
		return ContactWelcomeContent{}, false, nil
	}
	if channelCodeInt(welcome.WelcomeMessage["scanCodePush"]) == 2 {
		return ContactWelcomeContent{}, false, nil
	}
	details := channelCodeMaps(welcome.WelcomeMessage["messageDetail"])
	if len(details) == 0 {
		return ContactWelcomeContent{}, false, nil
	}
	text, mediumID, found := channelCodeWelcomeMessageDetailContent(details, now)
	if !found {
		return ContactWelcomeContent{}, false, nil
	}
	content := ContactWelcomeContent{Text: text}
	if mediumID > 0 {
		media, err := store.GreetingMediaByIDs(ctx, []int{mediumID})
		if err != nil {
			return ContactWelcomeContent{}, false, err
		}
		if medium, ok := media[mediumID]; ok {
			content.Medium = &ContactWelcomeMedium{
				MediumType:    medium.Type,
				MediumContent: cloneMap(medium.Content),
			}
		}
	}
	if strings.TrimSpace(content.Text) == "" && (content.Medium == nil || content.Medium.MediumType == 0) {
		return ContactWelcomeContent{}, false, nil
	}
	return content, true, nil
}

func channelCodeWelcomeMessageDetailContent(details []map[string]any, now time.Time) (string, int, bool) {
	byType := make(map[int]map[string]any, len(details))
	for _, detail := range details {
		byType[channelCodeInt(detail["type"])] = detail
	}
	if detail, ok := byType[3]; ok && channelCodeInt(detail["status"]) == 1 {
		for _, period := range channelCodeMaps(detail["detail"]) {
			if !channelCodeDateContains(channelCodeString(period["startDate"]), channelCodeString(period["endDate"]), now) {
				continue
			}
			if text, mediumID, found := channelCodeWelcomeSlotContent(channelCodeMaps(period["timeSlot"]), now); found {
				return text, mediumID, true
			}
		}
	}
	if detail, ok := byType[2]; ok && channelCodeInt(detail["status"]) == 1 {
		currentWeek := int(now.Weekday())
		for _, period := range channelCodeMaps(detail["detail"]) {
			if !intSliceContains(channelCodeInts(period["chooseCycle"]), currentWeek) {
				continue
			}
			if text, mediumID, found := channelCodeWelcomeSlotContent(channelCodeMaps(period["timeSlot"]), now); found {
				return text, mediumID, true
			}
		}
	}
	if detail, ok := byType[1]; ok {
		return channelCodeWelcomeEntryContent(detail)
	}
	return "", 0, false
}

func channelCodeWelcomeSlotContent(slots []map[string]any, now time.Time) (string, int, bool) {
	var common map[string]any
	for _, slot := range slots {
		startTime := channelCodeString(slot["startTime"])
		endTime := channelCodeString(slot["endTime"])
		if channelCodeTimeSlotContains(startTime, endTime, now) {
			return channelCodeWelcomeEntryContent(slot)
		}
		if startTime == "00:00" && endTime == "00:00" {
			common = slot
		}
	}
	if common != nil {
		return channelCodeWelcomeEntryContent(common)
	}
	return "", 0, false
}

func channelCodeWelcomeEntryContent(entry map[string]any) (string, int, bool) {
	text := channelCodeString(entry["welcomeContent"])
	mediumID := channelCodeInt(entry["mediumId"])
	return text, mediumID, strings.TrimSpace(text) != "" || mediumID > 0
}

func channelCodeDateContains(startDate string, endDate string, now time.Time) bool {
	start, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(startDate), now.Location())
	if err != nil {
		return false
	}
	end, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(endDate), now.Location())
	if err != nil {
		return false
	}
	current, err := time.ParseInLocation("2006-01-02", now.Format("2006-01-02"), now.Location())
	if err != nil {
		return false
	}
	return !current.Before(start) && !current.After(end)
}

func channelCodeTimeSlotContains(startTime string, endTime string, now time.Time) bool {
	start, ok := channelCodeMinuteOfDay(startTime)
	if !ok {
		return false
	}
	end, ok := channelCodeMinuteOfDay(endTime)
	if !ok {
		return false
	}
	current := now.Hour()*60 + now.Minute()
	return current >= start && current <= end
}

func channelCodeMinuteOfDay(value string) (int, bool) {
	parsed, err := time.Parse("15:04", strings.TrimSpace(value))
	if err != nil {
		return 0, false
	}
	return parsed.Hour()*60 + parsed.Minute(), true
}

func contactWelcomePayload(ctx context.Context, client ContactWelcomeClient, credential RoomWelcomeCorpCredential, event ContactWelcomeEvent, fileStorageRoot string, apiBaseURL string) (ContactWelcomePayload, error) {
	var payload ContactWelcomePayload
	if text := strings.TrimSpace(event.Content.Text); text != "" {
		payload.Text = &ContactWelcomeText{Content: strings.ReplaceAll(text, "##客户名称##", event.ContactName)}
	}
	if event.Content.Medium == nil {
		return payload, nil
	}
	content := event.Content.Medium.MediumContent
	switch event.Content.Medium.MediumType {
	case 2:
		imagePath := strings.TrimSpace(toString(content["imagePath"]))
		if imagePath == "" {
			return payload, fmt.Errorf("欢迎语图片素材缺少 imagePath")
		}
		mediaID, err := client.UploadTemporaryImage(ctx, credential, mediumLocalFilePath(fileStorageRoot, imagePath))
		if err != nil {
			return payload, err
		}
		payload.Attachments = append(payload.Attachments, ContactWelcomeAttachment{
			MsgType: "image",
			Image:   &ContactWelcomeImage{MediaID: mediaID},
		})
	case 3:
		title := strings.TrimSpace(toString(content["title"]))
		linkURL := strings.TrimSpace(toString(content["imageLink"]))
		if title == "" || linkURL == "" {
			return payload, fmt.Errorf("欢迎语图文素材缺少 title 或 imageLink")
		}
		link := &ContactWelcomeLink{
			Title: title,
			URL:   linkURL,
			Desc:  strings.TrimSpace(toString(content["description"])),
		}
		if imagePath := strings.TrimSpace(toString(content["imagePath"])); imagePath != "" {
			link.PicURL = contactWelcomeFullStaticURL(apiBaseURL, imagePath)
		}
		payload.Attachments = append(payload.Attachments, ContactWelcomeAttachment{
			MsgType: "link",
			Link:    link,
		})
	case 6:
		imagePath := strings.TrimSpace(toString(content["imagePath"]))
		title := strings.TrimSpace(toString(content["title"]))
		appID := strings.TrimSpace(toString(content["appid"]))
		page := strings.TrimSpace(toString(content["page"]))
		if imagePath == "" || title == "" || appID == "" || page == "" {
			return payload, fmt.Errorf("欢迎语小程序素材缺少必填字段")
		}
		mediaID, err := client.UploadTemporaryImage(ctx, credential, mediumLocalFilePath(fileStorageRoot, imagePath))
		if err != nil {
			return payload, err
		}
		payload.Attachments = append(payload.Attachments, ContactWelcomeAttachment{
			MsgType: "miniprogram",
			MiniProgram: &ContactWelcomeMiniProgram{
				Title:      title,
				PicMediaID: mediaID,
				AppID:      appID,
				Page:       page,
			},
		})
	}
	return payload, nil
}

func contactWelcomePayloadEmpty(payload ContactWelcomePayload) bool {
	return payload.Text == nil && len(payload.Attachments) == 0
}

func contactWelcomeRequest(payload ContactWelcomePayload, welcomeCode string) map[string]any {
	request := map[string]any{"welcome_code": strings.TrimSpace(welcomeCode)}
	if payload.Text != nil {
		request["text"] = payload.Text
	}
	if len(payload.Attachments) > 0 {
		request["attachments"] = payload.Attachments
	}
	return request
}

func contactWelcomeFullStaticURL(apiBaseURL string, path string) string {
	path = strings.TrimSpace(path)
	if path == "" || strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") || strings.TrimSpace(apiBaseURL) == "" {
		return path
	}
	return strings.TrimRight(strings.TrimSpace(apiBaseURL), "/") + "/static/" + strings.TrimLeft(filepath.ToSlash(path), "/")
}

func intSliceContains(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
