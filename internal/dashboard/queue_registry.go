package dashboard

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	QueueNameWeWorkCallback                = "wework-callback"
	QueueNameEmployeeApply                 = "employee-apply"
	QueueNameContactWelcome                = "contact-welcome"
	QueueNameAsyncFileUpload               = "async-file-upload"
	QueueNameMarkTags                      = "mark-tags"
	QueueNameMessageRemind                 = "message-remind"
	QueueNameWorkRoomSync                  = "work-room-sync"
	QueueNameWorkContactSync               = "work-contact-sync"
	QueueNameWorkDepartmentList            = "work-department-list"
	QueueNameMediumMediaIDUpdate           = "medium-media-id-update"
	QueueNameEmployeeStatisticApply        = "employee-statistic-apply"
	QueuePayloadTypeWeWorkCallback         = "dashboard.WeWorkCallbackEvent.v1"
	QueuePayloadTypeEmployeeApply          = "dashboard.EmployeeApplyEvent.v1"
	QueuePayloadTypeContactWelcome         = "dashboard.ContactWelcomeEvent.v1"
	QueuePayloadTypeAsyncFileUpload        = "dashboard.AsyncFileUploadEvent.v1"
	QueuePayloadTypeMarkTags               = "dashboard.MarkTagsEvent.v1"
	QueuePayloadTypeMessageRemind          = "dashboard.MessageRemindEvent.v1"
	QueuePayloadTypeWorkRoomSync           = "dashboard.WorkRoomSyncEvent.v1"
	QueuePayloadTypeWorkContactSync        = "dashboard.WorkContactSyncEvent.v1"
	QueuePayloadTypeWorkDepartmentList     = "dashboard.WorkDepartmentListEvent.v1"
	QueuePayloadTypeMediumMediaIDUpdate    = "dashboard.MediumMediaIDUpdateEvent.v1"
	QueuePayloadTypeEmployeeStatisticApply = "dashboard.EmployeeStatisticApplyEvent.v1"
)

type QueuePayloadDescriptor struct {
	Name           string
	PayloadType    string
	SourceKey      string
	ProcessingKey  string
	DeadLetterKey  string
	IdempotencyTTL time.Duration
}

func WeWorkCallbackQueueDescriptor() QueuePayloadDescriptor {
	return QueuePayloadDescriptor{
		Name:           QueueNameWeWorkCallback,
		PayloadType:    QueuePayloadTypeWeWorkCallback,
		SourceKey:      "mochat-go:wework-callback",
		ProcessingKey:  "mochat-go:wework-callback:processing",
		DeadLetterKey:  "mochat-go:wework-callback:dead",
		IdempotencyTTL: 10 * time.Minute,
	}
}

func EmployeeApplyQueueDescriptor() QueuePayloadDescriptor {
	return QueuePayloadDescriptor{
		Name:           QueueNameEmployeeApply,
		PayloadType:    QueuePayloadTypeEmployeeApply,
		SourceKey:      "mochat-go:employee-apply",
		ProcessingKey:  "mochat-go:employee-apply:processing",
		DeadLetterKey:  "mochat-go:employee-apply:dead",
		IdempotencyTTL: 10 * time.Minute,
	}
}

func ContactWelcomeQueueDescriptor() QueuePayloadDescriptor {
	return QueuePayloadDescriptor{
		Name:           QueueNameContactWelcome,
		PayloadType:    QueuePayloadTypeContactWelcome,
		SourceKey:      "mochat-go:contact-welcome",
		ProcessingKey:  "mochat-go:contact-welcome:processing",
		DeadLetterKey:  "mochat-go:contact-welcome:dead",
		IdempotencyTTL: 10 * time.Minute,
	}
}

func AsyncFileUploadQueueDescriptor() QueuePayloadDescriptor {
	return QueuePayloadDescriptor{
		Name:           QueueNameAsyncFileUpload,
		PayloadType:    QueuePayloadTypeAsyncFileUpload,
		SourceKey:      "mochat-go:async-file-upload",
		ProcessingKey:  "mochat-go:async-file-upload:processing",
		DeadLetterKey:  "mochat-go:async-file-upload:dead",
		IdempotencyTTL: 10 * time.Minute,
	}
}

func MarkTagsQueueDescriptor() QueuePayloadDescriptor {
	return QueuePayloadDescriptor{
		Name:           QueueNameMarkTags,
		PayloadType:    QueuePayloadTypeMarkTags,
		SourceKey:      "mochat-go:mark-tags",
		ProcessingKey:  "mochat-go:mark-tags:processing",
		DeadLetterKey:  "mochat-go:mark-tags:dead",
		IdempotencyTTL: 10 * time.Minute,
	}
}

func MessageRemindQueueDescriptor() QueuePayloadDescriptor {
	return QueuePayloadDescriptor{
		Name:           QueueNameMessageRemind,
		PayloadType:    QueuePayloadTypeMessageRemind,
		SourceKey:      "mochat-go:message-remind",
		ProcessingKey:  "mochat-go:message-remind:processing",
		DeadLetterKey:  "mochat-go:message-remind:dead",
		IdempotencyTTL: 10 * time.Minute,
	}
}

func WorkRoomSyncQueueDescriptor() QueuePayloadDescriptor {
	return QueuePayloadDescriptor{
		Name:           QueueNameWorkRoomSync,
		PayloadType:    QueuePayloadTypeWorkRoomSync,
		SourceKey:      "mochat-go:work-room-sync",
		ProcessingKey:  "mochat-go:work-room-sync:processing",
		DeadLetterKey:  "mochat-go:work-room-sync:dead",
		IdempotencyTTL: 10 * time.Minute,
	}
}

func WorkContactSyncQueueDescriptor() QueuePayloadDescriptor {
	return QueuePayloadDescriptor{
		Name:           QueueNameWorkContactSync,
		PayloadType:    QueuePayloadTypeWorkContactSync,
		SourceKey:      "mochat-go:work-contact-sync",
		ProcessingKey:  "mochat-go:work-contact-sync:processing",
		DeadLetterKey:  "mochat-go:work-contact-sync:dead",
		IdempotencyTTL: 10 * time.Minute,
	}
}

func WorkDepartmentListQueueDescriptor() QueuePayloadDescriptor {
	return QueuePayloadDescriptor{
		Name:           QueueNameWorkDepartmentList,
		PayloadType:    QueuePayloadTypeWorkDepartmentList,
		SourceKey:      "mochat-go:work-department-list",
		ProcessingKey:  "mochat-go:work-department-list:processing",
		DeadLetterKey:  "mochat-go:work-department-list:dead",
		IdempotencyTTL: 10 * time.Minute,
	}
}

func MediumMediaIDUpdateQueueDescriptor() QueuePayloadDescriptor {
	return QueuePayloadDescriptor{
		Name:           QueueNameMediumMediaIDUpdate,
		PayloadType:    QueuePayloadTypeMediumMediaIDUpdate,
		SourceKey:      "mochat-go:medium-media-id-update",
		ProcessingKey:  "mochat-go:medium-media-id-update:processing",
		DeadLetterKey:  "mochat-go:medium-media-id-update:dead",
		IdempotencyTTL: 10 * time.Minute,
	}
}

func EmployeeStatisticApplyQueueDescriptor() QueuePayloadDescriptor {
	return QueuePayloadDescriptor{
		Name:           QueueNameEmployeeStatisticApply,
		PayloadType:    QueuePayloadTypeEmployeeStatisticApply,
		SourceKey:      "mochat-go:employee-statistic-apply",
		ProcessingKey:  "mochat-go:employee-statistic-apply:processing",
		DeadLetterKey:  "mochat-go:employee-statistic-apply:dead",
		IdempotencyTTL: 10 * time.Minute,
	}
}

func QueuePayloadRegistry() []QueuePayloadDescriptor {
	return []QueuePayloadDescriptor{
		WeWorkCallbackQueueDescriptor(),
		EmployeeApplyQueueDescriptor(),
		ContactWelcomeQueueDescriptor(),
		AsyncFileUploadQueueDescriptor(),
		MarkTagsQueueDescriptor(),
		MessageRemindQueueDescriptor(),
		WorkRoomSyncQueueDescriptor(),
		WorkContactSyncQueueDescriptor(),
		WorkDepartmentListQueueDescriptor(),
		MediumMediaIDUpdateQueueDescriptor(),
		EmployeeStatisticApplyQueueDescriptor(),
	}
}

func QueuePayloadDescriptorByName(name string) (QueuePayloadDescriptor, bool) {
	name = strings.TrimSpace(name)
	for _, descriptor := range QueuePayloadRegistry() {
		if descriptor.Name == name {
			return descriptor, true
		}
	}
	return QueuePayloadDescriptor{}, false
}

func QueueIdempotencyRedisKey(queueName string, digest string) string {
	queueName = strings.TrimSpace(queueName)
	digest = strings.TrimSpace(digest)
	if queueName == "" || digest == "" {
		return ""
	}
	return fmt.Sprintf("mochat-go:queue-idempotency:%s:%s", queueName, digest)
}

func WeWorkCallbackIdempotencyKey(event WeWorkCallbackEvent) string {
	digest := stableQueuePayloadDigest(struct {
		CorpID    int               `json:"corpId"`
		WxCorpID  string            `json:"wxCorpId"`
		EventPath string            `json:"eventPath"`
		Business  map[string]string `json:"business"`
	}{
		CorpID:    event.CorpID,
		WxCorpID:  strings.TrimSpace(event.WxCorpID),
		EventPath: strings.TrimSpace(event.EventPath),
		Business:  weWorkCallbackBusinessIdentity(event),
	})
	return QueueIdempotencyRedisKey(QueueNameWeWorkCallback, digest)
}

func weWorkCallbackBusinessIdentity(event WeWorkCallbackEvent) map[string]string {
	message := event.Message
	identity := map[string]string{}
	add := func(name string, keys ...string) {
		if value := queueMessageString(message, keys...); value != "" {
			identity[name] = value
		}
	}
	add("toUserName", "ToUserName", "tousername", "to_user_name")
	add("fromUserName", "FromUserName", "fromusername", "from_user_name")
	add("createTime", "CreateTime", "createtime", "create_time")

	switch strings.TrimSpace(event.EventPath) {
	case "event.change_contact.create_user", "event.change_contact.update_user", "event.change_contact.delete_user":
		add("userID", "UserID", "UserId", "userid", "user_id")
	case "event.change_contact.create_party", "event.change_contact.update_party", "event.change_contact.delete_party":
		add("id", "Id", "ID", "id")
	case "event.change_external_tag.create", "event.change_external_tag.update", "event.change_external_tag.delete":
		add("tagType", "TagType", "tag_type")
		add("id", "Id", "ID", "id")
	case "event.change_external_contact.add_external_contact",
		"event.change_external_contact.edit_external_contact",
		"event.change_external_contact.add_half_external_contact",
		"event.change_external_contact.del_external_contact",
		"event.change_external_contact.del_follow_user",
		"event.change_external_contact.transfer_fail":
		add("userID", "UserID", "UserId", "userid", "user_id")
		add("externalUserID", "ExternalUserID", "ExternalUserid", "external_userid", "externalUserID")
	case "event.change_external_chat.create", "event.change_external_chat.update", "event.change_external_chat.dismiss":
		add("chatID", "ChatId", "ChatID", "chat_id", "chatid")
	default:
		add("userID", "UserID", "UserId", "userid", "user_id")
		add("externalUserID", "ExternalUserID", "ExternalUserid", "external_userid", "externalUserID")
		add("chatID", "ChatId", "ChatID", "chat_id", "chatid")
		add("id", "Id", "ID", "id")
		add("eventKey", "EventKey", "event_key")
	}
	if len(identity) == 0 {
		identity["rawXml"] = strings.TrimSpace(event.RawXML)
	}
	return identity
}

func queueMessageString(message map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(message[key]); value != "" {
			return value
		}
	}
	return ""
}

func EmployeeApplyIdempotencyKey(event EmployeeApplyEvent) string {
	digest := stableQueuePayloadDigest(struct {
		CorpIDs []int  `json:"corpIds"`
		UserID  int    `json:"userId,omitempty"`
		Source  string `json:"source,omitempty"`
	}{
		CorpIDs: uniqueQueueCorpIDs(event.CorpIDs),
		UserID:  event.UserID,
		Source:  strings.TrimSpace(event.Source),
	})
	return QueueIdempotencyRedisKey(QueueNameEmployeeApply, digest)
}

func ContactWelcomeIdempotencyKey(event ContactWelcomeEvent) string {
	digest := stableQueuePayloadDigest(struct {
		CorpID      int    `json:"corpId"`
		ContactID   int    `json:"contactId"`
		EmployeeID  int    `json:"employeeId"`
		WelcomeCode string `json:"welcomeCode"`
	}{
		CorpID:      event.CorpID,
		ContactID:   event.ContactID,
		EmployeeID:  event.EmployeeID,
		WelcomeCode: strings.TrimSpace(event.WelcomeCode),
	})
	return QueueIdempotencyRedisKey(QueueNameContactWelcome, digest)
}

func AsyncFileUploadIdempotencyKey(event AsyncFileUploadEvent) string {
	files := make([]AsyncFileUploadFile, 0, len(event.Files))
	for _, file := range event.Files {
		files = append(files, AsyncFileUploadFile{
			SourcePath:   strings.TrimSpace(file.SourcePath),
			TargetPath:   strings.TrimSpace(file.TargetPath),
			DeleteSource: file.DeleteSource,
		})
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].SourcePath != files[j].SourcePath {
			return files[i].SourcePath < files[j].SourcePath
		}
		if files[i].TargetPath != files[j].TargetPath {
			return files[i].TargetPath < files[j].TargetPath
		}
		return !files[i].DeleteSource && files[j].DeleteSource
	})
	digest := stableQueuePayloadDigest(struct {
		Files    []AsyncFileUploadFile `json:"files"`
		Source   string                `json:"source,omitempty"`
		TenantID int                   `json:"tenantId,omitempty"`
		CorpID   int                   `json:"corpId,omitempty"`
	}{
		Files:    files,
		Source:   strings.TrimSpace(event.Source),
		TenantID: event.TenantID,
		CorpID:   event.CorpID,
	})
	return QueueIdempotencyRedisKey(QueueNameAsyncFileUpload, digest)
}

func MarkTagsIdempotencyKey(event MarkTagsEvent) string {
	digest := stableQueuePayloadDigest(struct {
		CorpID          int    `json:"corpId"`
		ContactID       int    `json:"contactId"`
		EmployeeID      int    `json:"employeeId"`
		TagIDs          []int  `json:"tagIds"`
		Source          string `json:"source,omitempty"`
		AutoTagID       int    `json:"autoTagId,omitempty"`
		AutoTagRecordID int    `json:"autoTagRecordId,omitempty"`
	}{
		CorpID:          event.CorpID,
		ContactID:       event.ContactID,
		EmployeeID:      event.EmployeeID,
		TagIDs:          uniqueQueueCorpIDs(event.TagIDs),
		Source:          strings.TrimSpace(event.Source),
		AutoTagID:       event.AutoTagID,
		AutoTagRecordID: event.AutoTagRecordID,
	})
	return QueueIdempotencyRedisKey(QueueNameMarkTags, digest)
}

func MessageRemindIdempotencyKey(event MessageRemindEvent) string {
	toType, recipients, _ := event.recipientTarget()
	digest := stableQueuePayloadDigest(struct {
		CorpID  int            `json:"corpId"`
		ToType  string         `json:"toType"`
		To      []string       `json:"to"`
		MsgType string         `json:"msgType"`
		Content any            `json:"content,omitempty"`
		Extra   map[string]any `json:"extra,omitempty"`
		Source  string         `json:"source,omitempty"`
	}{
		CorpID:  event.CorpID,
		ToType:  strings.TrimSpace(toType),
		To:      uniqueQueueStrings(recipients),
		MsgType: strings.TrimSpace(event.MsgType),
		Content: event.Content,
		Extra:   event.Extra,
		Source:  strings.TrimSpace(event.Source),
	})
	return QueueIdempotencyRedisKey(QueueNameMessageRemind, digest)
}

func WorkRoomSyncIdempotencyKey(event WorkRoomSyncEvent) string {
	digest := stableQueuePayloadDigest(struct {
		CorpID   int    `json:"corpId,omitempty"`
		WXCorpID string `json:"wxCorpId,omitempty"`
		ChatID   string `json:"chatId,omitempty"`
		Source   string `json:"source,omitempty"`
	}{
		CorpID:   event.CorpID,
		WXCorpID: strings.TrimSpace(event.WXCorpID),
		ChatID:   strings.TrimSpace(event.ChatID),
		Source:   strings.TrimSpace(event.Source),
	})
	return QueueIdempotencyRedisKey(QueueNameWorkRoomSync, digest)
}

func WorkContactSyncIdempotencyKey(event WorkContactSyncEvent) string {
	employees := normalizeWorkContactSyncEmployees(append([]WorkContactSyncEmployee{event.Employee}, event.Employees...))
	digest := stableQueuePayloadDigest(struct {
		CorpID          int                       `json:"corpId,omitempty"`
		WXCorpID        string                    `json:"wxCorpId,omitempty"`
		Employees       []WorkContactSyncEmployee `json:"employees,omitempty"`
		ExternalUserIDs []string                  `json:"externalUserIds,omitempty"`
		Source          string                    `json:"source,omitempty"`
	}{
		CorpID:          event.CorpID,
		WXCorpID:        strings.TrimSpace(event.WXCorpID),
		Employees:       employees,
		ExternalUserIDs: uniqueQueueStrings(event.ExternalUserIDs),
		Source:          strings.TrimSpace(event.Source),
	})
	return QueueIdempotencyRedisKey(QueueNameWorkContactSync, digest)
}

func WorkDepartmentListIdempotencyKey(event WorkDepartmentListEvent) string {
	digest := stableQueuePayloadDigest(struct {
		CorpIDs  []int  `json:"corpIds,omitempty"`
		UserID   int    `json:"userId,omitempty"`
		TenantID int    `json:"tenantId,omitempty"`
		Source   string `json:"source,omitempty"`
	}{
		CorpIDs:  uniqueQueueCorpIDs(event.CorpIDs),
		UserID:   event.UserID,
		TenantID: event.TenantID,
		Source:   strings.TrimSpace(event.Source),
	})
	return QueueIdempotencyRedisKey(QueueNameWorkDepartmentList, digest)
}

func MediumMediaIDUpdateIdempotencyKey(event MediumMediaIDUpdateEvent) string {
	digest := stableQueuePayloadDigest(struct {
		CorpID    int    `json:"corpId,omitempty"`
		MediumIDs []int  `json:"mediumIds,omitempty"`
		Source    string `json:"source,omitempty"`
	}{
		CorpID:    event.CorpID,
		MediumIDs: uniqueQueueCorpIDs(event.MediumIDs),
		Source:    strings.TrimSpace(event.Source),
	})
	return QueueIdempotencyRedisKey(QueueNameMediumMediaIDUpdate, digest)
}

func EmployeeStatisticApplyIdempotencyKey(event EmployeeStatisticApplyEvent) string {
	digest := stableQueuePayloadDigest(struct {
		CorpID   int    `json:"corpId,omitempty"`
		TenantID int    `json:"tenantId,omitempty"`
		Source   string `json:"source,omitempty"`
	}{
		CorpID:   event.CorpID,
		TenantID: event.TenantID,
		Source:   strings.TrimSpace(event.Source),
	})
	return QueueIdempotencyRedisKey(QueueNameEmployeeStatisticApply, digest)
}

func stableQueuePayloadDigest(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		sum := sha256.Sum256([]byte(fmt.Sprintf("%#v", value)))
		return hex.EncodeToString(sum[:])
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func uniqueQueueCorpIDs(values []int) []int {
	seen := map[int]struct{}{}
	out := make([]int, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Ints(out)
	return out
}

func uniqueQueueStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
