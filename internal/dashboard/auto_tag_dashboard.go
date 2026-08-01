package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type AutoTagFilter struct {
	CorpID  int
	Type    int
	Name    string
	Tags    []int
	Page    int
	PerPage int
}

type AutoTagItem struct {
	ID                   int
	Type                 int
	Name                 string
	EmployeesRaw         string
	FuzzyMatchKeywordRaw string
	ExactMatchKeywordRaw string
	TagRuleRaw           string
	TagsRaw              string
	OnOff                int
	MarkTagCount         int
	TenantID             int
	CorpID               int
	CreateUserID         int
	CreateUserName       string
	CreatedAt            string
	UpdatedAt            string
}

type AutoTagPage struct {
	Items     []AutoTagItem
	Total     int
	TotalPage int
	Page      int
	PerPage   int
}

type AutoTagWrite struct {
	CorpID               int
	CreateUserID         int
	Type                 int
	HasType              bool
	Name                 string
	HasName              bool
	EmployeesRaw         string
	HasEmployees         bool
	FuzzyMatchKeywordRaw string
	HasFuzzyMatchKeyword bool
	ExactMatchKeywordRaw string
	HasExactMatchKeyword bool
	TagRuleRaw           string
	HasTagRule           bool
	TagsRaw              string
	HasTags              bool
	OnOff                int
	HasOnOff             bool
}

type AutoTagRecordFilter struct {
	CorpID        int
	AutoTagID     int
	ContactName   string
	EmployeeName  string
	RoomName      string
	JoinScene     int
	DateTimeStart string
	DateTimeEnd   string
	Page          int
	PerPage       int
}

type AutoTagRecordItem struct {
	ID               int
	AutoTagID        int
	ContactID        int
	ContactName      string
	ContactAvatar    string
	TagRuleID        int
	WXExternalUserID string
	EmployeeID       int
	EmployeeName     string
	Keyword          string
	ContactRoomID    int
	RoomID           int
	RoomName         string
	JoinScene        int
	JoinTime         string
	TagsRaw          string
	CorpID           int
	TriggerCount     int
	Status           int
	ContactCreatedAt string
	CreatedAt        string
	UpdatedAt        string
}

type AutoTagRecordPage struct {
	Items     []AutoTagRecordItem
	Total     int
	TotalPage int
	Page      int
	PerPage   int
}

type AutoTagStatistics struct {
	TotalCount int
	TodayCount int
}

type AutoTagKeywordTaskResult struct {
	RuleCount       int
	PendingMessages int
	MatchedMessages int
	CreatedRecords  int
	QueuedMarkTags  int
	MarkTagsEvents  []MarkTagsEvent
}

type AutoTagRoomJoinTaskResult struct {
	RuleCount      int
	MatchedMembers int
	CreatedRecords int
	QueuedMarkTags int
	MarkTagsEvents []MarkTagsEvent
}

type AutoTagContactTimeTaskResult struct {
	RuleCount      int
	MatchedRules   int
	CreatedRecords int
	QueuedMarkTags int
	MarkTagsEvents []MarkTagsEvent
}

type WorkMessageUserFilter struct {
	CorpID              int
	WorkEmployeeID      int
	ToUserType          int
	ToUserID            int
	Name                string
	Keyword             string
	DateTimeStart       string
	DateTimeEnd         string
	AllowAllEmployees   bool
	RestrictEmployeeIDs bool
	EmployeeIDs         []int
	Page                int
	PerPage             int
}

type WorkMessageToUser struct {
	ID             int
	TableIndex     int
	Seq            int64
	MsgID          string
	WorkEmployeeID int
	EmployeeName   string
	EmployeeAvatar string
	ToUserType     int
	ToUserID       int
	Name           string
	Alias          string
	Avatar         string
	Content        string
	MsgDataTime    string
}

type WorkMessageToUserPage struct {
	Items     []WorkMessageToUser
	Total     int
	TotalPage int
	Page      int
	PerPage   int
}

type WorkMessageFilter struct {
	CorpID         int
	WorkEmployeeID int
	Type           int
	ToUserType     int
	ToUserID       int
	Content        string
	DateTimeStart  string
	DateTimeEnd    string
	Page           int
	PerPage        int
	Latest         bool
}

type WorkMessageArchiveFilter struct {
	CorpID              int
	ArchiveMessageID    string
	RestrictEmployeeIDs bool
	EmployeeIDs         []int
}

type WorkMessageItem struct {
	ID             int
	TableIndex     int
	Seq            int64
	MsgID          string
	WorkEmployeeID int
	EmployeeName   string
	EmployeeAvatar string
	ToUserType     int
	ToUserID       int
	TargetName     string
	TargetAvatar   string
	Action         int
	Name           string
	Avatar         string
	IsCurrentUser  int
	Type           int
	ContentRaw     string
	MsgDataTime    string
}

type WorkMessagePage struct {
	Items     []WorkMessageItem
	Total     int
	TotalPage int
	Page      int
	PerPage   int
}

type WorkMessageFromUser struct {
	ID     int
	Name   string
	Avatar string
}

type WorkMessageConfigItem struct {
	ID                 int
	CorpID             int
	CorpName           string
	WXCorpID           string
	SocialCode         string
	ChatAdmin          string
	ChatAdminPhone     string
	ChatAdminIDCard    string
	ChatApplyStatus    int
	ChatStatus         int
	ChatSecret         string
	ServiceContactURL  string
	ChatWhitelistIPRaw string
	ChatRSAKeyRaw      string
	CreatedAt          string
	UpdatedAt          string
}

type WorkMessageConfigPage struct {
	Items     []WorkMessageConfigItem
	Total     int
	TotalPage int
	Page      int
	PerPage   int
}

type AutoTagStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	AutoTagPage(ctx context.Context, filter AutoTagFilter) (AutoTagPage, error)
	AutoTagByID(ctx context.Context, corpID int, id int) (AutoTagItem, bool, error)
	CreateAutoTag(ctx context.Context, values AutoTagWrite) (int, error)
	UpdateAutoTagOnOff(ctx context.Context, corpID int, id int, onOff int) (bool, error)
	DeleteAutoTag(ctx context.Context, corpID int, id int) (bool, error)
	AutoTagStatistics(ctx context.Context, corpID int, autoTagID int) (AutoTagStatistics, error)
	AutoTagRecordPage(ctx context.Context, filter AutoTagRecordFilter) (AutoTagRecordPage, error)
	AutoTagKeywordTask(ctx context.Context, corpID int) (AutoTagKeywordTaskResult, error)
	WorkMessageFromUsers(ctx context.Context, corpID int, name string, page int, perPage int) ([]WorkMessageFromUser, error)
	WorkMessageToUsers(ctx context.Context, filter WorkMessageUserFilter) (WorkMessageToUserPage, error)
	WorkMessagePage(ctx context.Context, filter WorkMessageFilter) (WorkMessagePage, error)
	WorkMessageArchiveAuthorized(ctx context.Context, tenantID int, corpID int) (bool, error)
	WorkMessageByArchiveID(ctx context.Context, filter WorkMessageArchiveFilter) (WorkMessageItem, bool, error)
	WorkMessageConfigByCorp(ctx context.Context, corpID int) (WorkMessageConfigItem, bool, error)
	WorkMessageConfigPage(ctx context.Context, corpID int, name string, page int, perPage int) (WorkMessageConfigPage, error)
	UpsertWorkMessageCorpConfig(ctx context.Context, corpID int, values WorkMessageConfigItem) (int, error)
	UpdateWorkMessageStepConfig(ctx context.Context, corpID int, values WorkMessageConfigItem) (bool, error)
}

type AutoTagMarkTagsQueue interface {
	EnqueueMarkTags(ctx context.Context, event MarkTagsEvent) error
}

type AutoTagHandler struct {
	store         AutoTagStore
	cache         LoginCache
	resolver      UserIDResolver
	authorizer    CorpAdminAuthorizer
	markTagsQueue AutoTagMarkTagsQueue
}

func NewAutoTagHandler(store AutoTagStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer) *AutoTagHandler {
	return &AutoTagHandler{store: store, cache: cache, resolver: resolver, authorizer: authorizer}
}

func (h *AutoTagHandler) WithMarkTagsQueue(queue AutoTagMarkTagsQueue) *AutoTagHandler {
	h.markTagsQueue = queue
	return h
}

func (h *AutoTagHandler) Index(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/autoTag/index#get")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	page, err := h.store.AutoTagPage(r.Context(), AutoTagFilter{
		CorpID:  corpID,
		Type:    positiveQueryInt(r, "type", 0),
		Name:    sopQueryName(r),
		Tags:    parseQueryIntList(r, "tags"),
		Page:    positiveQueryInt(r, "page", 1),
		PerPage: positiveQueryInt(r, "perPage", 15),
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, autoTagPayload(item))
	}
	payload := contactBatchAddPagination(r, page.Page, page.PerPage, page.Total, list)
	payload["list"] = list
	payload["page"] = map[string]any{"perPage": page.PerPage, "total": page.Total, "totalPage": page.TotalPage}
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *AutoTagHandler) Store(w http.ResponseWriter, r *http.Request) {
	h.writeMutation(w, r, http.MethodPost, "/dashboard/autoTag/store#post", func(ctx context.Context, userID int, corpID int, params map[string]any) (any, error) {
		values, err := autoTagWriteFromParams(params, corpID, userID)
		if err != nil {
			return nil, err
		}
		id, err := h.store.CreateAutoTag(ctx, values)
		if err != nil {
			return nil, err
		}
		return []any{id}, nil
	})
}

func (h *AutoTagHandler) Destroy(w http.ResponseWriter, r *http.Request) {
	h.writeMutation(w, r, http.MethodDelete, "/dashboard/autoTag/destroy#delete", func(ctx context.Context, _ int, corpID int, params map[string]any) (any, error) {
		id, err := autoTagIDFromParams(params)
		if err != nil {
			return nil, err
		}
		ok, err := h.store.DeleteAutoTag(ctx, corpID, id)
		return nil, sopRequireFound(ok, err, "自动打标签规则不存在")
	})
}

func (h *AutoTagHandler) OnOff(w http.ResponseWriter, r *http.Request) {
	h.writeMutation(w, r, http.MethodPut, "/dashboard/autoTag/onOff#put", func(ctx context.Context, _ int, corpID int, params map[string]any) (any, error) {
		id, err := autoTagIDFromParams(params)
		if err != nil {
			return nil, err
		}
		onOff, found, err := firstAutoTagIntParam(params, "on_off", "onOff", "status", "state")
		if err != nil || !found || (onOff != 1 && onOff != 2) {
			return nil, badRequestError("on_off invalid")
		}
		ok, err := h.store.UpdateAutoTagOnOff(ctx, corpID, id, onOff)
		return nil, sopRequireFound(ok, err, "自动打标签规则不存在")
	})
}

func (h *AutoTagHandler) Show(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/autoTag/show#get")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	id := autoTagIDFromQuery(r)
	if id <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "id required", nil)
		return
	}
	item, found, err := h.store.AutoTagByID(r.Context(), corpID, id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "自动打标签规则不存在", nil)
		return
	}
	stats, err := h.store.AutoTagStatistics(r.Context(), corpID, id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"auto_tag":   autoTagPayload(item),
		"autoTag":    autoTagPayload(item),
		"statistics": map[string]any{"total_count": stats.TotalCount, "today_count": stats.TodayCount, "totalCount": stats.TotalCount, "todayCount": stats.TodayCount},
	})
}

func (h *AutoTagHandler) ShowContactKeyWord(w http.ResponseWriter, r *http.Request) {
	h.showRecordPage(w, r, "/dashboard/autoTag/showContactKeyWord#get")
}

func (h *AutoTagHandler) ShowContactRoom(w http.ResponseWriter, r *http.Request) {
	h.showRecordPage(w, r, "/dashboard/autoTag/showContactRoom#get")
}

func (h *AutoTagHandler) ShowContactTime(w http.ResponseWriter, r *http.Request) {
	h.showRecordPage(w, r, "/dashboard/autoTag/showContactTime#get")
}

func (h *AutoTagHandler) KeyWordTag(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, _, ok := h.resolveAuthorized(w, r, "/Task/AutoTag/KeyWordTag#get")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	result, err := h.store.AutoTagKeywordTask(r.Context(), corpID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if h.markTagsQueue != nil {
		for _, event := range result.MarkTagsEvents {
			if err := h.markTagsQueue.EnqueueMarkTags(r.Context(), event); err != nil {
				writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
				return
			}
			result.QueuedMarkTags++
		}
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"ruleCount":        result.RuleCount,
		"rule_count":       result.RuleCount,
		"pendingMessages":  result.PendingMessages,
		"pending_messages": result.PendingMessages,
		"matchedMessages":  result.MatchedMessages,
		"matched_messages": result.MatchedMessages,
		"createdRecords":   result.CreatedRecords,
		"created_records":  result.CreatedRecords,
		"queuedMarkTags":   result.QueuedMarkTags,
		"queued_mark_tags": result.QueuedMarkTags,
	})
}

func (h *AutoTagHandler) WorkMessageFromUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/workMessage/fromUsers#get")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	items, err := h.store.WorkMessageFromUsers(r.Context(), corpID, sopQueryName(r), positiveQueryInt(r, "page", 1), positiveQueryInt(r, "perPage", 100))
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(items))
	for _, item := range items {
		list = append(list, map[string]any{"id": item.ID, "name": item.Name, "avatar": item.Avatar})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", list)
}

func (h *AutoTagHandler) WorkMessageToUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, user, loginInfo, access, ok := h.resolveAuthorized(w, r, "/dashboard/workMessage/toUsers#get")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	global := r.URL.Query().Get("view") == "global"
	if global && !h.workMessageArchiveAllowed(w, r, user.TenantID, corpID) {
		return
	}
	filter := WorkMessageUserFilter{
		CorpID:         corpID,
		WorkEmployeeID: positiveQueryInt(r, "workEmployeeId", 0),
		ToUserType:     autoTagQueryInt(r, 0, "toUsertype", "toUserType", "to_user_type"),
		Name:           sopQueryName(r),
		Page:           positiveQueryInt(r, "page", 1),
		PerPage:        positiveQueryInt(r, "perPage", 15),
	}
	if global {
		var valid bool
		filter, valid = workMessageGlobalFilter(w, r, corpID, access)
		if !valid {
			return
		}
	}
	page, err := h.store.WorkMessageToUsers(r.Context(), filter)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		if global {
			list = append(list, workMessageGlobalConversationPayload(item))
		} else {
			list = append(list, workMessageToUserPayload(item))
		}
	}
	if global {
		writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
			"list":     list,
			"total":    page.Total,
			"page":     page.Page,
			"pageSize": page.PerPage,
		})
		return
	}
	payload := contactBatchAddPagination(r, page.Page, page.PerPage, page.Total, list)
	payload["list"] = list
	payload["page"] = map[string]any{"perPage": page.PerPage, "total": page.Total, "totalPage": page.TotalPage}
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *AutoTagHandler) WorkMessageIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, user, loginInfo, access, ok := h.resolveAuthorized(w, r, "/dashboard/workMessage/index#get")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	if r.URL.Path == "/dashboard/workMessage/detail" {
		h.workMessageGlobalDetail(w, r, user.TenantID, corpID, access)
		return
	}
	page, err := h.store.WorkMessagePage(r.Context(), WorkMessageFilter{
		CorpID:         corpID,
		WorkEmployeeID: positiveQueryInt(r, "workEmployeeId", 0),
		Type:           positiveQueryInt(r, "type", 0),
		ToUserType:     autoTagQueryInt(r, 0, "toUserType", "toUsertype", "to_user_type"),
		ToUserID:       autoTagQueryInt(r, 0, "toUserId", "to_user_id"),
		Content:        strings.TrimSpace(r.URL.Query().Get("content")),
		DateTimeStart:  autoTagQueryString(r, "dateTimeStart", "start_time", "startTime"),
		DateTimeEnd:    autoTagQueryString(r, "dateTimeEnd", "end_time", "endTime"),
		Page:           positiveQueryInt(r, "page", 1),
		PerPage:        positiveQueryInt(r, "perPage", 15),
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, workMessagePayload(item))
	}
	payload := contactBatchAddPagination(r, page.Page, page.PerPage, page.Total, list)
	payload["list"] = list
	payload["page"] = map[string]any{"perPage": page.PerPage, "total": page.Total, "totalPage": page.TotalPage}
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *AutoTagHandler) workMessageGlobalDetail(w http.ResponseWriter, r *http.Request, tenantID int, corpID int, access AccessContext) {
	if !h.workMessageArchiveAllowed(w, r, tenantID, corpID) {
		return
	}
	archiveMessageID := strings.TrimSpace(r.URL.Query().Get("id"))
	if archiveMessageID == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid archive message id", nil)
		return
	}
	archiveFilter := workMessageArchiveFilter(corpID, archiveMessageID, access)
	anchor, found, err := h.store.WorkMessageByArchiveID(r.Context(), archiveFilter)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found || (archiveFilter.RestrictEmployeeIDs && !containsInt(archiveFilter.EmployeeIDs, anchor.WorkEmployeeID)) {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "conversation not found", nil)
		return
	}
	employeeID, toUserType, toUserID := anchor.WorkEmployeeID, anchor.ToUserType, anchor.ToUserID
	page, err := h.store.WorkMessagePage(r.Context(), WorkMessageFilter{
		CorpID:         corpID,
		WorkEmployeeID: employeeID,
		ToUserType:     toUserType,
		ToUserID:       toUserID,
		Page:           1,
		PerPage:        200,
		Latest:         true,
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if page.Total == 0 || len(page.Items) == 0 {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "conversation not found", nil)
		return
	}
	first := page.Items[0]
	messages := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		payload := workMessagePayload(item)
		messages = append(messages, map[string]any{
			"id":           workMessageStableMessageID(item),
			"senderName":   item.Name,
			"senderAvatar": item.Avatar,
			"direction":    workMessageDirection(item.IsCurrentUser),
			"type":         item.Type,
			"content":      payload["content"],
			"sentAt":       item.MsgDataTime,
		})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"id":           archiveMessageID,
		"employeeId":   employeeID,
		"employeeName": first.EmployeeName,
		"targetType":   workMessageTargetType(toUserType),
		"targetId":     toUserID,
		"targetName":   first.TargetName,
		"messageTotal": page.Total,
		"truncated":    page.Total > len(page.Items),
		"window":       "latest",
		"messages":     messages,
	})
}

func workMessageStableMessageID(item WorkMessageItem) string {
	return workMessageArchiveID(item.MsgID, item.Seq, item.TableIndex, item.ID)
}

func workMessageGlobalFilter(w http.ResponseWriter, r *http.Request, corpID int, access AccessContext) (WorkMessageUserFilter, bool) {
	requestedEmployeeIDs, ok := workMessageEmployeeIDs(w, r)
	if !ok {
		return WorkMessageUserFilter{}, false
	}
	customerID, ok := workMessageStrictPositiveQueryInt(w, r, "customerId", 0)
	if !ok {
		return WorkMessageUserFilter{}, false
	}
	roomID, ok := workMessageStrictPositiveQueryInt(w, r, "roomId", 0)
	if !ok {
		return WorkMessageUserFilter{}, false
	}
	page, ok := workMessageStrictPositiveQueryInt(w, r, "page", 1)
	if !ok {
		return WorkMessageUserFilter{}, false
	}
	pageSize, ok := workMessageStrictPositiveQueryInt(w, r, "pageSize", 20)
	if !ok {
		return WorkMessageUserFilter{}, false
	}
	if customerID > 0 && roomID > 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "customerId and roomId cannot be combined", nil)
		return WorkMessageUserFilter{}, false
	}
	start, end, ok := workMessageGlobalDateRange(w, r)
	if !ok {
		return WorkMessageUserFilter{}, false
	}
	if pageSize > 100 {
		pageSize = 100
	}
	filter := WorkMessageUserFilter{
		CorpID:            corpID,
		ToUserType:        -1,
		Keyword:           strings.TrimSpace(r.URL.Query().Get("keyword")),
		DateTimeStart:     start,
		DateTimeEnd:       end,
		AllowAllEmployees: true,
		Page:              page,
		PerPage:           pageSize,
	}
	conversationType := strings.TrimSpace(r.URL.Query().Get("conversationType"))
	switch conversationType {
	case "":
	case "employee":
		filter.ToUserType = 0
	case "customer":
		filter.ToUserType = 1
	case "room":
		filter.ToUserType = 2
	default:
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid conversationType", nil)
		return WorkMessageUserFilter{}, false
	}
	if customerID > 0 {
		filter.ToUserType = 1
		filter.ToUserID = customerID
	}
	if roomID > 0 {
		filter.ToUserType = 2
		filter.ToUserID = roomID
	}
	if access.DataPermission == DataPermissionAll {
		if len(requestedEmployeeIDs) > 0 {
			filter.RestrictEmployeeIDs = true
			filter.EmployeeIDs = requestedEmployeeIDs
		}
	} else {
		filter.RestrictEmployeeIDs = true
		allowed := workMessageUniquePositiveInts(access.DeptEmployeeIDs)
		if len(requestedEmployeeIDs) == 0 {
			filter.EmployeeIDs = allowed
		} else {
			filter.EmployeeIDs = workMessageEmployeeIntersection(requestedEmployeeIDs, allowed)
		}
	}
	return filter, true
}

func (h *AutoTagHandler) workMessageArchiveAllowed(w http.ResponseWriter, r *http.Request, tenantID int, corpID int) bool {
	allowed, err := h.store.WorkMessageArchiveAuthorized(r.Context(), tenantID, corpID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return false
	}
	if !allowed {
		writeEnvelope(w, http.StatusForbidden, 40301, "archive not authorized", nil)
		return false
	}
	return true
}

func workMessageArchiveFilter(corpID int, archiveMessageID string, access AccessContext) WorkMessageArchiveFilter {
	filter := WorkMessageArchiveFilter{CorpID: corpID, ArchiveMessageID: archiveMessageID}
	if access.DataPermission != DataPermissionAll {
		filter.RestrictEmployeeIDs = true
		filter.EmployeeIDs = workMessageUniquePositiveInts(access.DeptEmployeeIDs)
	}
	return filter
}

func workMessageArchiveID(msgID string, seq int64, tableIndex int, id int) string {
	if strings.TrimSpace(msgID) != "" {
		return "msg:" + msgID
	}
	if seq > 0 {
		return fmt.Sprintf("seq:%d", seq)
	}
	if tableIndex > 0 && id > 0 {
		return fmt.Sprintf("table:%d:%d", tableIndex, id)
	}
	return ""
}

func workMessageEmployeeIDs(w http.ResponseWriter, r *http.Request) ([]int, bool) {
	rawValues := append([]string{}, r.URL.Query()["employeeIds"]...)
	if len(rawValues) == 0 && r.URL.Query().Has("employeeId") {
		rawValues = append(rawValues, r.URL.Query().Get("employeeId"))
	}
	ids := make([]int, 0, len(rawValues))
	for _, rawValue := range rawValues {
		for _, raw := range strings.Split(rawValue, ",") {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			value, err := strconv.Atoi(raw)
			if err != nil || value <= 0 {
				writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid employeeIds", nil)
				return nil, false
			}
			ids = append(ids, value)
		}
	}
	return workMessageUniquePositiveInts(ids), true
}

func workMessageUniquePositiveInts(values []int) []int {
	result := make([]int, 0, len(values))
	seen := make(map[int]struct{}, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func workMessageEmployeeIntersection(requested []int, allowed []int) []int {
	allowedSet := make(map[int]struct{}, len(allowed))
	for _, value := range allowed {
		allowedSet[value] = struct{}{}
	}
	result := make([]int, 0, len(requested))
	for _, value := range requested {
		if _, ok := allowedSet[value]; ok {
			result = append(result, value)
		}
	}
	return result
}

func workMessageStrictPositiveQueryInt(w http.ResponseWriter, r *http.Request, name string, fallback int) (int, bool) {
	query := r.URL.Query()
	if !query.Has(name) {
		return fallback, true
	}
	raw := strings.TrimSpace(query.Get(name))
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid "+name, nil)
		return 0, false
	}
	return value, true
}

func workMessageGlobalDateRange(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	from := strings.TrimSpace(r.URL.Query().Get("startAt"))
	to := strings.TrimSpace(r.URL.Query().Get("endAt"))
	if from == "" {
		from = strings.TrimSpace(r.URL.Query().Get("from"))
	}
	if to == "" {
		to = strings.TrimSpace(r.URL.Query().Get("to"))
	}
	if from == "" && to == "" {
		return "", "", true
	}
	if from == "" || to == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "startAt and endAt are required together", nil)
		return "", "", false
	}
	fromDate, fromErr := time.Parse("2006-01-02", from)
	toDate, toErr := time.Parse("2006-01-02", to)
	if fromErr != nil || toErr != nil || fromDate.After(toDate) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid date range", nil)
		return "", "", false
	}
	return fromDate.Format("2006-01-02") + " 00:00:00", toDate.AddDate(0, 0, 1).Format("2006-01-02") + " 00:00:00", true
}

func parseWorkMessageConversationID(raw string) (int, int, int, bool) {
	parts := strings.Split(raw, ":")
	if len(parts) != 3 {
		return 0, 0, 0, false
	}
	employeeID, employeeErr := strconv.Atoi(parts[0])
	toUserType, typeErr := strconv.Atoi(parts[1])
	toUserID, targetErr := strconv.Atoi(parts[2])
	if employeeErr != nil || typeErr != nil || targetErr != nil ||
		employeeID <= 0 || toUserType < 0 || toUserType > 2 || toUserID <= 0 {
		return 0, 0, 0, false
	}
	return employeeID, toUserType, toUserID, true
}

func workMessageConversationID(employeeID int, toUserType int, toUserID int) string {
	return fmt.Sprintf("%d:%d:%d", employeeID, toUserType, toUserID)
}

func workMessageTargetType(value int) string {
	switch value {
	case 0:
		return "employee"
	case 1:
		return "customer"
	case 2:
		return "room"
	default:
		return "unknown"
	}
}

func workMessageDirection(isCurrentUser int) string {
	if isCurrentUser == 1 {
		return "outbound"
	}
	return "inbound"
}

func workMessageGlobalConversationPayload(item WorkMessageToUser) map[string]any {
	return map[string]any{
		"id":             workMessageArchiveID(item.MsgID, item.Seq, item.TableIndex, item.ID),
		"employeeId":     item.WorkEmployeeID,
		"employeeName":   item.EmployeeName,
		"employeeAvatar": item.EmployeeAvatar,
		"targetType":     workMessageTargetType(item.ToUserType),
		"targetId":       item.ToUserID,
		"targetName":     item.Name,
		"targetAvatar":   item.Avatar,
		"lastMessage":    item.Content,
		"sentAt":         item.MsgDataTime,
	}
}

func (h *AutoTagHandler) WorkMessageConfigCorpIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/workMessageConfig/corpIndex#get")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	page, err := h.store.WorkMessageConfigPage(r.Context(), corpID, strings.TrimSpace(r.URL.Query().Get("corpName")), positiveQueryInt(r, "page", 1), positiveQueryInt(r, "perPage", 10))
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, workMessageConfigPayload(item))
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"page": map[string]any{"perPage": page.PerPage, "total": page.Total, "totalPage": page.TotalPage},
		"list": list,
	})
}

func (h *AutoTagHandler) WorkMessageConfigCorpShow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/workMessageConfig/corpShow#get")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	if requested := positiveQueryInt(r, "corpId", 0); requested > 0 {
		corpID = requested
	}
	item, found, err := h.store.WorkMessageConfigByCorp(r.Context(), corpID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "企业不存在", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", workMessageConfigPayload(item))
}

func (h *AutoTagHandler) WorkMessageConfigCorpStore(w http.ResponseWriter, r *http.Request) {
	h.writeMutation(w, r, http.MethodPost, "/dashboard/workMessageConfig/corpStore#post", func(ctx context.Context, _ int, corpID int, params map[string]any) (any, error) {
		values, err := workMessageConfigFromParams(params)
		if err != nil {
			return nil, err
		}
		if requested, found, err := firstAutoTagIntParam(params, "corpId", "corp_id"); err != nil {
			return nil, badRequestError("corpId invalid")
		} else if found && requested > 0 {
			corpID = requested
		}
		id, err := h.store.UpsertWorkMessageCorpConfig(ctx, corpID, values)
		if err != nil {
			return nil, err
		}
		return map[string]any{"id": id}, nil
	})
}

func (h *AutoTagHandler) WorkMessageConfigStepCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/workMessageConfig/stepCreate#get")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	item, found, err := h.store.WorkMessageConfigByCorp(r.Context(), corpID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "企业不存在", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", workMessageStepPayload(item))
}

func (h *AutoTagHandler) WorkMessageConfigStepUpdate(w http.ResponseWriter, r *http.Request) {
	h.writeMutation(w, r, http.MethodPut, "/dashboard/workMessageConfig/stepUpdate#put", func(ctx context.Context, _ int, corpID int, params map[string]any) (any, error) {
		values, err := workMessageConfigFromParams(params)
		if err != nil {
			return nil, err
		}
		ok, err := h.store.UpdateWorkMessageStepConfig(ctx, corpID, values)
		return nil, sopRequireFound(ok, err, "企业不存在")
	})
}

func (h *AutoTagHandler) showRecordPage(w http.ResponseWriter, r *http.Request, permissionKey string) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, _, ok := h.resolveAuthorized(w, r, permissionKey)
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	page, err := h.store.AutoTagRecordPage(r.Context(), AutoTagRecordFilter{
		CorpID:        corpID,
		AutoTagID:     autoTagIDFromQuery(r),
		ContactName:   autoTagQueryString(r, "contact_name", "contactName", "name"),
		EmployeeName:  autoTagQueryString(r, "employee", "employeeName"),
		RoomName:      autoTagQueryString(r, "room_name", "roomName"),
		JoinScene:     positiveQueryInt(r, "join_scene", 0),
		DateTimeStart: autoTagQueryString(r, "start_time", "dateTimeStart", "startTime"),
		DateTimeEnd:   autoTagQueryString(r, "end_time", "dateTimeEnd", "endTime"),
		Page:          positiveQueryInt(r, "page", 1),
		PerPage:       positiveQueryInt(r, "perPage", 15),
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, autoTagRecordPayload(item))
	}
	payload := contactBatchAddPagination(r, page.Page, page.PerPage, page.Total, list)
	payload["list"] = list
	payload["page"] = map[string]any{"perPage": page.PerPage, "total": page.Total, "totalPage": page.TotalPage}
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *AutoTagHandler) writeMutation(w http.ResponseWriter, r *http.Request, method string, permissionKey string, action func(context.Context, int, int, map[string]any) (any, error)) {
	if r.Method != method {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, loginInfo, _, ok := h.resolveAuthorized(w, r, permissionKey)
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "参数错误", nil)
		return
	}
	data, err := action(r.Context(), userID, corpID, params)
	if err != nil {
		if isBadRequestError(err) {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
			return
		}
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if data == nil {
		data = []any{}
	}
	writeEnvelope(w, http.StatusOK, 200, "success", data)
}

func (h *AutoTagHandler) resolveAuthorized(w http.ResponseWriter, r *http.Request, permissionKey string) (int, User, LoginCorpInfo, AccessContext, bool) {
	userID, user, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return 0, User{}, LoginCorpInfo{}, AccessContext{}, false
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return 0, User{}, LoginCorpInfo{}, AccessContext{}, false
	}
	employeeID := loginInfo.WorkEmployeeID
	if employeeID <= 0 {
		resolved, err := h.store.EmployeeIDByUserCorp(r.Context(), userID, corpID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return 0, User{}, LoginCorpInfo{}, AccessContext{}, false
		}
		employeeID = resolved
	}
	access := AccessContext{User: user, PermissionKey: permissionKey, CorpID: corpID, WorkEmployeeID: employeeID, DataPermission: DataPermissionAll}
	if h.authorizer != nil {
		resolved, err := h.authorizer.Resolve(r.Context(), userID, permissionKey, corpID, employeeID)
		if err != nil {
			writeAccessError(w, err)
			return 0, User{}, LoginCorpInfo{}, AccessContext{}, false
		}
		access = resolved
	}
	return userID, user, loginInfo, access, true
}

func (h *AutoTagHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (int, User, LoginCorpInfo, bool) {
	if h.resolver == nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "user resolver not configured", nil)
		return 0, User{}, LoginCorpInfo{}, false
	}
	userID, err := h.resolver.UserID(r)
	if err != nil || userID <= 0 {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return 0, User{}, LoginCorpInfo{}, false
	}
	user, found, err := h.store.UserByID(r.Context(), userID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return 0, User{}, LoginCorpInfo{}, false
	}
	if !found {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "user not found", nil)
		return 0, User{}, LoginCorpInfo{}, false
	}
	cacheValue := ""
	if h.cache != nil {
		cacheValue, err = h.cache.UserCorpCache(r.Context(), userID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return 0, User{}, LoginCorpInfo{}, false
		}
	}
	loginInfo, err := ResolveValidatedLoginCorpInfoFromStore(r.Context(), r.Header, user, cacheValue, h.store)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return 0, User{}, LoginCorpInfo{}, false
	}
	return userID, user, LoginCorpInfo(loginInfo), true
}

func autoTagWriteFromParams(params map[string]any, corpID int, userID int) (AutoTagWrite, error) {
	values := AutoTagWrite{CorpID: corpID, CreateUserID: userID, OnOff: 1, HasOnOff: true}
	if tagType, found, err := firstAutoTagIntParam(params, "type"); err != nil {
		return AutoTagWrite{}, badRequestError("type invalid")
	} else if found {
		values.Type = tagType
		values.HasType = true
	}
	if name, found := sopStringParam(params, "name", "title"); found {
		values.Name = name
		values.HasName = true
	}
	if values.Type <= 0 {
		return AutoTagWrite{}, badRequestError("type required")
	}
	if strings.TrimSpace(values.Name) == "" {
		return AutoTagWrite{}, badRequestError("name required")
	}
	if raw, found, err := sopRawJSONParam(params, "employees", "employeeIds", "employee_ids"); err != nil {
		return AutoTagWrite{}, err
	} else if found {
		values.EmployeesRaw = raw
		values.HasEmployees = true
	}
	if raw, found, err := sopRawJSONParam(params, "fuzzy_match_keyword", "fuzzyMatchKeyword"); err != nil {
		return AutoTagWrite{}, err
	} else if found {
		values.FuzzyMatchKeywordRaw = raw
		values.HasFuzzyMatchKeyword = true
	}
	if raw, found, err := sopRawJSONParam(params, "exact_match_keyword", "exactMatchKeyword"); err != nil {
		return AutoTagWrite{}, err
	} else if found {
		values.ExactMatchKeywordRaw = raw
		values.HasExactMatchKeyword = true
	}
	if raw, found, err := sopRawJSONParam(params, "tag_rule", "tagRule"); err != nil {
		return AutoTagWrite{}, err
	} else if found {
		values.TagRuleRaw = raw
		values.HasTagRule = true
		values.TagsRaw = autoTagTagsFromRule(raw)
		values.HasTags = true
	}
	if raw, found, err := sopRawJSONParam(params, "tags"); err != nil {
		return AutoTagWrite{}, err
	} else if found && !values.HasTags {
		values.TagsRaw = raw
		values.HasTags = true
	}
	if onOff, found, err := firstAutoTagIntParam(params, "on_off", "onOff"); err != nil {
		return AutoTagWrite{}, badRequestError("on_off invalid")
	} else if found {
		values.OnOff = onOff
		values.HasOnOff = true
	}
	return values, nil
}

func autoTagIDFromParams(params map[string]any) (int, error) {
	id, _, err := firstPositiveIntParam(params, "id", "autoTagId", "auto_tag_id")
	if err != nil || id <= 0 {
		return 0, badRequestError("id required")
	}
	return id, nil
}

func autoTagIDFromQuery(r *http.Request) int {
	for _, key := range []string{"id", "autoTagId", "auto_tag_id"} {
		if id := positiveQueryInt(r, key, 0); id > 0 {
			return id
		}
	}
	return 0
}

func firstAutoTagIntParam(params map[string]any, keys ...string) (int, bool, error) {
	for _, key := range keys {
		value, found, err := intParam(params, key)
		if err != nil {
			return 0, found, err
		}
		if found {
			return value, true, nil
		}
	}
	return 0, false, nil
}

func autoTagQueryInt(r *http.Request, fallback int, keys ...string) int {
	for _, key := range keys {
		if strings.TrimSpace(r.URL.Query().Get(key)) == "" {
			continue
		}
		return positiveQueryInt(r, key, fallback)
	}
	return fallback
}

func autoTagQueryString(r *http.Request, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(r.URL.Query().Get(key)); value != "" {
			return value
		}
	}
	return ""
}

func autoTagPayload(item AutoTagItem) map[string]any {
	tags := autoTagTagsPayload(item.TagsRaw, item.TagRuleRaw)
	employees := autoTagEmployeesPayload(item.EmployeesRaw)
	return map[string]any{
		"id":                  item.ID,
		"autoTagId":           item.ID,
		"auto_tag_id":         item.ID,
		"type":                item.Type,
		"name":                item.Name,
		"employees":           employees,
		"employeesRaw":        item.EmployeesRaw,
		"fuzzy_match_keyword": sopJSONPayload(item.FuzzyMatchKeywordRaw),
		"fuzzyMatchKeyword":   sopJSONPayload(item.FuzzyMatchKeywordRaw),
		"exact_match_keyword": sopJSONPayload(item.ExactMatchKeywordRaw),
		"exactMatchKeyword":   sopJSONPayload(item.ExactMatchKeywordRaw),
		"tag_rule":            sopJSONPayload(item.TagRuleRaw),
		"tagRule":             sopJSONPayload(item.TagRuleRaw),
		"tags":                tags,
		"tagsRaw":             item.TagsRaw,
		"on_off":              item.OnOff,
		"onOff":               item.OnOff,
		"mark_tag_count":      item.MarkTagCount,
		"markTagCount":        item.MarkTagCount,
		"tenantId":            item.TenantID,
		"tenant_id":           item.TenantID,
		"corpId":              item.CorpID,
		"corp_id":             item.CorpID,
		"createUserId":        item.CreateUserID,
		"create_user_id":      item.CreateUserID,
		"nickname":            item.CreateUserName,
		"create_user":         item.CreateUserName,
		"createdAt":           item.CreatedAt,
		"created_at":          item.CreatedAt,
		"updatedAt":           item.UpdatedAt,
		"updated_at":          item.UpdatedAt,
	}
}

func autoTagRecordPayload(item AutoTagRecordItem) map[string]any {
	return map[string]any{
		"id":                 item.ID,
		"autoTagId":          item.AutoTagID,
		"auto_tag_id":        item.AutoTagID,
		"contactId":          item.ContactID,
		"contact_id":         item.ContactID,
		"contactName":        item.ContactName,
		"contact_name":       item.ContactName,
		"avatar":             item.ContactAvatar,
		"tagRuleId":          item.TagRuleID,
		"tag_rule_id":        item.TagRuleID,
		"wxExternalUserid":   item.WXExternalUserID,
		"wx_external_userid": item.WXExternalUserID,
		"employeeId":         item.EmployeeID,
		"employee_id":        item.EmployeeID,
		"employeeName":       item.EmployeeName,
		"employee_name":      item.EmployeeName,
		"keyword":            item.Keyword,
		"contactRoomId":      item.ContactRoomID,
		"contact_room_id":    item.ContactRoomID,
		"roomId":             item.RoomID,
		"room_id":            item.RoomID,
		"roomName":           item.RoomName,
		"room_name":          item.RoomName,
		"joinScene":          item.JoinScene,
		"join_scene":         item.JoinScene,
		"joinTime":           item.JoinTime,
		"join_time":          item.JoinTime,
		"tags":               sopJSONPayload(item.TagsRaw),
		"triggerCount":       item.TriggerCount,
		"trigger_count":      item.TriggerCount,
		"status":             item.Status,
		"createTagTime":      item.CreatedAt,
		"create_tag_time":    item.CreatedAt,
		"createTime":         firstNonEmpty(item.ContactCreatedAt, item.CreatedAt),
		"create_time":        firstNonEmpty(item.ContactCreatedAt, item.CreatedAt),
		"createdAt":          item.CreatedAt,
		"created_at":         item.CreatedAt,
		"updatedAt":          item.UpdatedAt,
		"updated_at":         item.UpdatedAt,
	}
}

func workMessageToUserPayload(item WorkMessageToUser) map[string]any {
	return map[string]any{
		"workEmployeeId":   item.WorkEmployeeID,
		"work_employee_id": item.WorkEmployeeID,
		"toUsertype":       item.ToUserType,
		"toUserType":       item.ToUserType,
		"to_user_type":     item.ToUserType,
		"toUserId":         item.ToUserID,
		"to_user_id":       item.ToUserID,
		"name":             item.Name,
		"alias":            item.Alias,
		"avatar":           item.Avatar,
		"content":          item.Content,
		"msgDataTime":      item.MsgDataTime,
		"msg_data_time":    item.MsgDataTime,
	}
}

func workMessagePayload(item WorkMessageItem) map[string]any {
	return map[string]any{
		"id":              item.ID,
		"action":          item.Action,
		"name":            item.Name,
		"avatar":          item.Avatar,
		"isCurrentUser":   item.IsCurrentUser,
		"is_current_user": item.IsCurrentUser,
		"type":            item.Type,
		"content":         workMessageContentPayload(item.ContentRaw),
		"contentRaw":      item.ContentRaw,
		"msgDataTime":     item.MsgDataTime,
		"msg_data_time":   item.MsgDataTime,
	}
}

func workMessageConfigPayload(item WorkMessageConfigItem) map[string]any {
	return map[string]any{
		"id":                  item.ID,
		"corpId":              item.CorpID,
		"corp_id":             item.CorpID,
		"corpName":            item.CorpName,
		"name":                item.CorpName,
		"wxCorpId":            item.WXCorpID,
		"wxCorpid":            item.WXCorpID,
		"wx_corpid":           item.WXCorpID,
		"socialCode":          item.SocialCode,
		"social_code":         item.SocialCode,
		"chatAdmin":           item.ChatAdmin,
		"chat_admin":          item.ChatAdmin,
		"chatAdminPhone":      item.ChatAdminPhone,
		"chat_admin_phone":    item.ChatAdminPhone,
		"chatAdminIdcard":     item.ChatAdminIDCard,
		"chat_admin_idcard":   item.ChatAdminIDCard,
		"chatApplyStatus":     item.ChatApplyStatus,
		"chat_apply_status":   item.ChatApplyStatus,
		"chatStatus":          item.ChatStatus,
		"chat_status":         item.ChatStatus,
		"chatSecret":          item.ChatSecret,
		"chat_secret":         item.ChatSecret,
		"serviceContactUrl":   item.ServiceContactURL,
		"service_contact_url": item.ServiceContactURL,
		"chatWhitelistIp":     sopJSONPayload(item.ChatWhitelistIPRaw),
		"chat_whitelist_ip":   sopJSONPayload(item.ChatWhitelistIPRaw),
		"chatRsaKey":          sopJSONPayload(item.ChatRSAKeyRaw),
		"chat_rsa_key":        sopJSONPayload(item.ChatRSAKeyRaw),
		"messageCreatedAt":    item.CreatedAt,
		"message_created_at":  item.CreatedAt,
		"createdAt":           item.CreatedAt,
		"created_at":          item.CreatedAt,
		"updatedAt":           item.UpdatedAt,
		"updated_at":          item.UpdatedAt,
	}
}

func workMessageStepPayload(item WorkMessageConfigItem) map[string]any {
	payload := workMessageConfigPayload(item)
	rsa := sopJSONPayload(item.ChatRSAKeyRaw)
	if decoded, ok := rsa.(map[string]any); ok {
		payload["rsaPublicKey"] = firstNonEmpty(fmt.Sprint(decoded["publicKey"]), fmt.Sprint(decoded["public_key"]))
		payload["rsaPrivateKey"] = firstNonEmpty(fmt.Sprint(decoded["privateKey"]), fmt.Sprint(decoded["private_key"]))
	} else {
		payload["rsaPublicKey"] = ""
		payload["rsaPrivateKey"] = ""
	}
	return payload
}

func workMessageConfigFromParams(params map[string]any) (WorkMessageConfigItem, error) {
	item := WorkMessageConfigItem{ChatApplyStatus: -1, ChatStatus: -1}
	if value, found := sopStringParam(params, "socialCode", "social_code"); found {
		item.SocialCode = value
	}
	if value, found := sopStringParam(params, "chatAdmin", "chat_admin"); found {
		item.ChatAdmin = value
	}
	if value, found := sopStringParam(params, "chatAdminPhone", "chat_admin_phone"); found {
		item.ChatAdminPhone = value
	}
	if value, found := sopStringParam(params, "chatAdminIdcard", "chatAdminIDCard", "chat_admin_idcard"); found {
		item.ChatAdminIDCard = value
	}
	if value, found := sopStringParam(params, "chatSecret", "chat_secret"); found {
		item.ChatSecret = value
	}
	if value, found := sopStringParam(params, "serviceContactUrl", "service_contact_url"); found {
		item.ServiceContactURL = value
	}
	if value, found, err := firstAutoTagIntParam(params, "chatApplyStatus", "chat_apply_status"); err != nil {
		return WorkMessageConfigItem{}, badRequestError("chatApplyStatus invalid")
	} else if found {
		item.ChatApplyStatus = value
	}
	if value, found, err := firstAutoTagIntParam(params, "chatStatus", "chat_status"); err != nil {
		return WorkMessageConfigItem{}, badRequestError("chatStatus invalid")
	} else if found {
		item.ChatStatus = value
	}
	if raw, found, err := sopRawJSONParam(params, "chatWhitelistIp", "chatWhitelistIP", "chat_whitelist_ip"); err != nil {
		return WorkMessageConfigItem{}, err
	} else if found {
		item.ChatWhitelistIPRaw = raw
	}
	if raw, found, err := sopRawJSONParam(params, "chatRsaKey", "chatRSAKey", "chat_rsa_key"); err != nil {
		return WorkMessageConfigItem{}, err
	} else if found {
		item.ChatRSAKeyRaw = raw
	}
	return item, nil
}

func autoTagTagsFromRule(raw string) string {
	var rules []map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &rules); err != nil {
		return "[]"
	}
	seen := map[string]struct{}{}
	tags := make([]string, 0)
	for _, rule := range rules {
		tagValues, ok := rule["tags"].([]any)
		if !ok {
			continue
		}
		for _, tagValue := range tagValues {
			name := ""
			switch typed := tagValue.(type) {
			case map[string]any:
				name = firstNonEmpty(strings.TrimSpace(fmt.Sprint(typed["tagname"])), strings.TrimSpace(fmt.Sprint(typed["name"])))
			default:
				name = strings.TrimSpace(fmt.Sprint(tagValue))
			}
			if name == "" || name == "<nil>" {
				continue
			}
			if _, exists := seen[name]; exists {
				continue
			}
			seen[name] = struct{}{}
			tags = append(tags, name)
		}
	}
	encoded, err := json.Marshal(tags)
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

func autoTagTagsPayload(tagsRaw string, ruleRaw string) any {
	tagsRaw = strings.TrimSpace(tagsRaw)
	if tagsRaw == "" || tagsRaw == "[]" {
		tagsRaw = autoTagTagsFromRule(ruleRaw)
	}
	payload := sopJSONPayload(tagsRaw)
	if values, ok := payload.([]any); ok {
		result := make([]string, 0, len(values))
		for _, value := range values {
			switch typed := value.(type) {
			case map[string]any:
				name := firstNonEmpty(strings.TrimSpace(fmt.Sprint(typed["tagname"])), strings.TrimSpace(fmt.Sprint(typed["name"])))
				if name != "" && name != "<nil>" {
					result = append(result, name)
				}
			default:
				name := strings.TrimSpace(fmt.Sprint(value))
				if name != "" && name != "<nil>" {
					result = append(result, name)
				}
			}
		}
		return result
	}
	return payload
}

func autoTagEmployeesPayload(raw string) []map[string]any {
	decoded := sopJSONPayload(raw)
	values, ok := decoded.([]any)
	if !ok {
		return []map[string]any{}
	}
	result := make([]map[string]any, 0, len(values))
	for _, value := range values {
		switch typed := value.(type) {
		case map[string]any:
			id := 0
			if rawID, ok := typed["id"]; ok {
				if parsed, err := strconv.Atoi(strings.TrimSpace(fmt.Sprint(rawID))); err == nil {
					id = parsed
				}
			}
			name := firstNonEmpty(strings.TrimSpace(fmt.Sprint(typed["name"])), strings.TrimSpace(fmt.Sprint(typed["wxUserId"])), strings.TrimSpace(fmt.Sprint(typed["wx_user_id"])))
			result = append(result, map[string]any{"id": id, "name": name})
		default:
			name := strings.TrimSpace(fmt.Sprint(value))
			if name != "" && name != "<nil>" {
				result = append(result, map[string]any{"id": 0, "name": name})
			}
		}
	}
	return result
}

func workMessageContentPayload(raw string) any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]any{"content": ""}
	}
	var decoded any
	if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
		return decoded
	}
	return map[string]any{"content": raw}
}
