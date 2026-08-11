package dashboard

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const contactMessageBatchSendTextLimit = 4000

type ContactMessageBatchSendFilter struct {
	UserID              int
	BatchTitle          string
	Page                int
	PerPage             int
	AllowedEmployeeIDs  []int
	RestrictEmployeeIDs bool
}

type ContactMessageBatchSendPage struct {
	Items     []ContactMessageBatchSendItem
	Total     int
	TotalPage int
	PerPage   int
}

type ContactMessageBatchSendItem struct {
	ID                 int
	CorpID             int
	UserID             int
	MediumID           int
	UserName           string
	EmployeeIDs        []int
	FilterParams       ContactMessageBatchSendFilterParams
	FilterParamsRaw    string
	FilterParamsDetail ContactMessageBatchSendFilterDetail
	Content            []ContactMessageBatchSendContent
	SendWay            int
	DefiniteTime       string
	SendTime           string
	SendEmployeeTotal  int
	SendContactTotal   int
	SendTotal          int
	NotSendTotal       int
	ReceivedTotal      int
	NotReceivedTotal   int
	ReceiveLimitTotal  int
	NotFriendTotal     int
	SendStatus         int
	CreatedAt          string
}

type ContactMessageBatchSendContent struct {
	MsgType    string `json:"msgType"`
	Content    string `json:"content,omitempty"`
	MediaID    string `json:"media_id,omitempty"`
	PicURL     string `json:"pic_url,omitempty"`
	Title      string `json:"title,omitempty"`
	Desc       string `json:"desc,omitempty"`
	URL        string `json:"url,omitempty"`
	AppID      string `json:"appid,omitempty"`
	Page       string `json:"page,omitempty"`
	PicMediaID string `json:"pic_media_id,omitempty"`
}

type ContactMessageBatchSendFilterParams struct {
	Gender          *int
	AddTimeStart    string
	AddTimeEnd      string
	Rooms           []int
	Tags            []int
	ExcludeContacts []int
}

type ContactMessageBatchSendFilterDetail struct {
	Gender          *int                                 `json:"gender,omitempty"`
	AddTimeStart    string                               `json:"addTimeStart,omitempty"`
	AddTimeEnd      string                               `json:"addTimeEnd,omitempty"`
	Rooms           []ContactMessageBatchSendNameID      `json:"rooms"`
	Tags            []ContactMessageBatchSendNameID      `json:"tags"`
	ExcludeContacts []ContactMessageBatchSendContactBase `json:"excludeContacts"`
}

type ContactMessageBatchSendNameID struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type ContactMessageBatchSendContactBase struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type ContactMessageBatchSendWrite struct {
	CorpID             int
	UserID             int
	MediumID           int
	UserName           string
	EmployeeIDs        []int
	FilterParams       ContactMessageBatchSendFilterParams
	FilterParamsJSON   string
	FilterDetailJSON   string
	Content            []ContactMessageBatchSendContent
	ContentJSON        string
	SendWay            int
	DefiniteTime       string
	ProcessImmediately bool
}

type ContactMessageBatchSendSendTarget struct {
	EmployeeID      int
	WXUserID        string
	ExternalUserIDs []string
	ContactIDs      []int
}

type ContactMessageBatchSendEmployeeItem struct {
	ID                  int
	Status              int
	SendTime            string
	SendContactTotal    int
	EmployeeID          int
	EmployeeName        string
	EmployeeAlias       string
	EmployeeAvatar      string
	EmployeeThumbAvatar string
}

type ContactMessageBatchSendEmployeePage struct {
	Items     []ContactMessageBatchSendEmployeeItem
	Total     int
	TotalPage int
	PerPage   int
}

type ContactMessageBatchSendReceiveItem struct {
	ID              int
	Status          int
	SendTime        int
	ContactID       int
	ContactName     string
	ContactNickName string
	ContactAvatar   string
	EmployeeID      int
	EmployeeName    string
	EmployeeAlias   string
}

type ContactMessageBatchSendReceivePage struct {
	Items     []ContactMessageBatchSendReceiveItem
	Total     int
	TotalPage int
	PerPage   int
}

type ContactMessageBatchSendEmployeeFilter struct {
	BatchID    int
	SendStatus *int
	KeyWords   string
	Page       int
	PerPage    int
}

type ContactMessageBatchSendReceiveFilter struct {
	BatchID    int
	SendStatus *int
	KeyWords   string
	Page       int
	PerPage    int
}

type ContactMessageBatchSendRoomInfo struct {
	Name         string
	OwnerID      int
	OwnerName    string
	OwnerAvatar  string
	Total        int
	TotalContact int
	TodayInsert  int
	TodayLoss    int
}

type ContactMessageBatchSendEmployeeRef struct {
	ID       int
	WXUserID string
}

type ContactMessageBatchSendMessagePayload struct {
	Content        []ContactMessageBatchSendContent
	ExternalUserID []string
	Sender         string
}

type ContactMessageBatchSendMessageResult struct {
	EmployeeID int
	ErrCode    int
	ErrMsg     string
	MsgID      string
}

type ContactMessageBatchSendStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	RoomWelcomeCorpCredentialByID(ctx context.Context, corpID int) (RoomWelcomeCorpCredential, bool, error)
	RoomTagPullRemindAgentByCorpID(ctx context.Context, corpID int) (RoomTagPullAgentCredential, bool, error)
	ContactMessageBatchSendPage(ctx context.Context, filter ContactMessageBatchSendFilter) (ContactMessageBatchSendPage, error)
	ContactMessageBatchSendByID(ctx context.Context, batchID int) (ContactMessageBatchSendItem, bool, error)
	CreateContactMessageBatchSend(ctx context.Context, values ContactMessageBatchSendWrite) (int, error)
	CreateContactMessageBatchSendTasks(ctx context.Context, batchID int) ([]ContactMessageBatchSendSendTarget, error)
	MarkContactMessageBatchSendSubmitted(ctx context.Context, batchID int, results []ContactMessageBatchSendMessageResult) error
	ContactMessageBatchSendEmployeePage(ctx context.Context, filter ContactMessageBatchSendEmployeeFilter) (ContactMessageBatchSendEmployeePage, error)
	ContactMessageBatchSendReceivePage(ctx context.Context, filter ContactMessageBatchSendReceiveFilter) (ContactMessageBatchSendReceivePage, error)
	DeleteContactMessageBatchSend(ctx context.Context, batchID int) (bool, error)
	ContactMessageBatchSendRoomInfo(ctx context.Context, roomID int) (ContactMessageBatchSendRoomInfo, bool, error)
	ContactMessageBatchSendEmployees(ctx context.Context, batchID int, batchEmployeeID int) ([]ContactMessageBatchSendEmployeeRef, error)
	ContactMessageBatchSendEmployeeWXUserID(ctx context.Context, employeeID int) (string, bool, error)
	ContactMessageBatchSendFilterDetail(ctx context.Context, params ContactMessageBatchSendFilterParams) (ContactMessageBatchSendFilterDetail, error)
}

type ContactMessageBatchSendClient interface {
	UploadTemporaryImage(ctx context.Context, credential RoomWelcomeCorpCredential, filePath string) (string, error)
	SubmitContactMessageBatchSend(ctx context.Context, credential RoomWelcomeCorpCredential, payload ContactMessageBatchSendMessagePayload) (ContactMessageBatchSendMessageResult, error)
	SendAgentTextMessage(ctx context.Context, credential RoomTagPullAgentCredential, toUser string, content string) error
}

type ContactMessageBatchSendHandler struct {
	store           ContactMessageBatchSendStore
	cache           LoginCache
	resolver        UserIDResolver
	authorizer      CorpAdminAuthorizer
	apiBaseURL      string
	fileStorageRoot string
	client          ContactMessageBatchSendClient
}

func NewContactMessageBatchSendHandler(store ContactMessageBatchSendStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer, apiBaseURL string, fileStorageRoot string, client ContactMessageBatchSendClient) *ContactMessageBatchSendHandler {
	if strings.TrimSpace(fileStorageRoot) == "" {
		fileStorageRoot = defaultRoomTagPullFileStorageRoot
	}
	if client == nil {
		client = NewRoomWelcomeWeComClient(defaultWeComAPIBaseURL)
	}
	return &ContactMessageBatchSendHandler{
		store:           store,
		cache:           cache,
		resolver:        resolver,
		authorizer:      authorizer,
		apiBaseURL:      strings.TrimRight(apiBaseURL, "/"),
		fileStorageRoot: fileStorageRoot,
		client:          client,
	}
}

func (h *ContactMessageBatchSendHandler) Index(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, _, ok := h.resolveAuthorized(w, r)
	if !ok {
		return
	}
	dashboardAccess, hasDashboardAccess := DashboardAccessFromContext(r.Context())
	page := positiveQueryInt(r, "page", 1)
	perPage := positiveQueryInt(r, "perPage", 10)
	result, err := h.store.ContactMessageBatchSendPage(r.Context(), ContactMessageBatchSendFilter{
		UserID:     userID,
		BatchTitle: strings.TrimSpace(r.URL.Query().Get("batchTitle")),
		Page:       page,
		PerPage:    perPage,
		AllowedEmployeeIDs: func() []int {
			if hasDashboardAccess {
				return append([]int(nil), dashboardAccess.AllowedEmployeeIDs...)
			}
			return nil
		}(),
		RestrictEmployeeIDs: hasDashboardAccess && dashboardAccess.ScopeRequired && dashboardAccess.Scope != DataScopeTenant,
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(result.Items))
	for _, item := range result.Items {
		list = append(list, h.batchListPayload(item))
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"page": map[string]any{
			"perPage":   result.PerPage,
			"total":     result.Total,
			"totalPage": result.TotalPage,
		},
		"list": list,
	})
}

func (h *ContactMessageBatchSendHandler) Show(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, _, ok := h.resolveAuthorized(w, r)
	if !ok {
		return
	}
	batchID, ok := queryPositiveIntParam(w, r, "batchId", "batchId 必填", "batchId 必须为整数")
	if !ok {
		return
	}
	batch, ok := h.loadOwnedBatch(w, r, userID, batchID)
	if !ok {
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", h.batchShowPayload(batch))
}

func (h *ContactMessageBatchSendHandler) MessageShow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, _, ok := h.resolveAuthorized(w, requestWithPermissionPath(r, "/dashboard/contactMessageBatchSend/show"))
	if !ok {
		return
	}
	batchID, ok := queryPositiveIntParam(w, r, "batchId", "batchId 必填", "batchId 必须为整数")
	if !ok {
		return
	}
	batch, ok := h.loadOwnedBatch(w, r, userID, batchID)
	if !ok {
		return
	}
	payload := h.batchShowPayload(batch)
	payload["message"] = payload["content"]
	payload["list"] = payload["content"]
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *ContactMessageBatchSendHandler) Store(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, user, _, ok := h.resolveAuthorized(w, r)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	write, ok := h.batchWriteFromParams(w, r.Context(), params, corpID, userID, user)
	if !ok {
		return
	}
	if access, scoped := DashboardAccessFromContext(r.Context()); scoped && access.ScopeRequired && access.Scope != DataScopeTenant && !employeeIDsWithinDashboardScope(write.EmployeeIDs, access.AllowedEmployeeIDs) {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, "employee scope denied", nil)
		return
	}
	if !enforceSaaSQuota(r.Context(), w, h.store, user.TenantID, SaaSMetricContactMessageBatches, 1) {
		return
	}
	batchID, err := h.store.CreateContactMessageBatchSend(r.Context(), write)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "客户消息创建失败", nil)
		return
	}
	if err := refreshSaaSUsageCounter(r.Context(), h.store, user.TenantID, SaaSMetricContactMessageBatches); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if write.ProcessImmediately {
		if ok := h.sendBatchNow(w, r.Context(), corpID, batchID); !ok {
			return
		}
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func employeeIDsWithinDashboardScope(ids, allowed []int) bool {
	if len(ids) == 0 {
		return false
	}
	set := make(map[int]struct{}, len(allowed))
	for _, id := range allowed {
		if id > 0 {
			set[id] = struct{}{}
		}
	}
	for _, id := range ids {
		if id > 0 {
			if _, ok := set[id]; !ok {
				return false
			}
		}
	}
	return true
}

func (h *ContactMessageBatchSendHandler) EmployeeSendIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, _, ok := h.resolveAuthorized(w, r)
	if !ok {
		return
	}
	batchID, ok := queryPositiveIntParam(w, r, "batchId", "batchId 必填", "batchId 必须为整数")
	if !ok {
		return
	}
	if _, ok = h.loadOwnedBatch(w, r, userID, batchID, true); !ok {
		return
	}
	sendStatus, ok := optionalQueryInt(w, r, "sendStatus", "sendStatus 必须为整数")
	if !ok {
		return
	}
	filter := ContactMessageBatchSendEmployeeFilter{
		BatchID:    batchID,
		SendStatus: sendStatus,
		KeyWords:   strings.TrimSpace(r.URL.Query().Get("keyWords")),
		Page:       positiveQueryInt(r, "page", 1),
		PerPage:    positiveQueryInt(r, "perPage", 15),
	}
	page, err := h.store.ContactMessageBatchSendEmployeePage(r.Context(), filter)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, map[string]any{
			"id":                  item.ID,
			"status":              item.Status,
			"sendTime":            item.SendTime,
			"sendContactTotal":    item.SendContactTotal,
			"employeeId":          item.EmployeeID,
			"employeeName":        item.EmployeeName,
			"employeeAlias":       item.EmployeeAlias,
			"employeeAvatar":      h.fullStaticURL(item.EmployeeAvatar),
			"employeeThumbAvatar": h.fullStaticURL(item.EmployeeThumbAvatar),
		})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"page": map[string]any{
			"perPage":   page.PerPage,
			"total":     page.Total,
			"totalPage": page.TotalPage,
		},
		"list": list,
	})
}

func (h *ContactMessageBatchSendHandler) ContactReceiveIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, _, ok := h.resolveAuthorized(w, r)
	if !ok {
		return
	}
	batchID, ok := queryPositiveIntParam(w, r, "batchId", "batchId 必填", "batchId 必须为整数")
	if !ok {
		return
	}
	if _, ok = h.loadOwnedBatch(w, r, userID, batchID, true); !ok {
		return
	}
	sendStatus, ok := optionalQueryInt(w, r, "sendStatus", "sendStatus 必须为整数")
	if !ok {
		return
	}
	page, err := h.store.ContactMessageBatchSendReceivePage(r.Context(), ContactMessageBatchSendReceiveFilter{
		BatchID:    batchID,
		SendStatus: sendStatus,
		KeyWords:   strings.TrimSpace(r.URL.Query().Get("keyWords")),
		Page:       positiveQueryInt(r, "page", 1),
		PerPage:    positiveQueryInt(r, "perPage", 15),
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, map[string]any{
			"id":              item.ID,
			"status":          item.Status,
			"sendTime":        formatUnixSecondsLocal(item.SendTime),
			"contactId":       item.ContactID,
			"contactName":     item.ContactName,
			"contactNickName": item.ContactNickName,
			"contactAvatar":   h.fullStaticURL(item.ContactAvatar),
			"employeeId":      item.EmployeeID,
			"employeeName":    item.EmployeeName,
			"employeeAlias":   item.EmployeeAlias,
		})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"page": map[string]any{
			"perPage":   page.PerPage,
			"total":     page.Total,
			"totalPage": page.TotalPage,
		},
		"list": list,
	})
}

func (h *ContactMessageBatchSendHandler) ShowRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, _, _, ok := h.resolveAuthorized(w, r); !ok {
		return
	}
	roomID, ok := queryPositiveIntParam(w, r, "workRoomId", "客户群id 必传", "workRoomId 必须为整数")
	if !ok {
		return
	}
	room, found, err := h.store.ContactMessageBatchSendRoomInfo(r.Context(), roomID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "客户群不存在", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"name":         room.Name,
		"ownerId":      room.OwnerID,
		"ownerName":    room.OwnerName,
		"ownerAvatar":  h.fullStaticURL(room.OwnerAvatar),
		"total":        room.Total,
		"totalContact": room.TotalContact,
		"todayInsert":  room.TodayInsert,
		"todayLoss":    room.TodayLoss,
	})
}

func (h *ContactMessageBatchSendHandler) Remind(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, _, ok := h.resolveAuthorized(w, r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	batchID, has, err := intParam(params, "batchId")
	if err != nil || !has || batchID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "batchId 必填", nil)
		return
	}
	batch, ok := h.loadOwnedBatch(w, r, userID, batchID, true)
	if !ok {
		return
	}
	batchEmployeeID, _, err := intParam(params, "batchEmployId")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "batchEmployId 必须为整数", nil)
		return
	}
	agent, found, err := h.store.RoomTagPullRemindAgentByCorpID(r.Context(), batch.CorpID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "无可用的 agent", nil)
		return
	}
	employees, err := h.store.ContactMessageBatchSendEmployees(r.Context(), batchID, batchEmployeeID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if batchEmployeeID > 0 && len(employees) == 0 {
		wxUserID, found, err := h.store.ContactMessageBatchSendEmployeeWXUserID(r.Context(), batchEmployeeID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		if found {
			employees = append(employees, ContactMessageBatchSendEmployeeRef{ID: batchEmployeeID, WXUserID: wxUserID})
		}
	}
	text := contactMessageBatchSendReminderText(batch.CreatedAt)
	for _, employee := range employees {
		if employee.WXUserID == "" {
			continue
		}
		if err := h.client.SendAgentTextMessage(r.Context(), agent, employee.WXUserID, text); err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "发送提醒失败", nil)
			return
		}
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *ContactMessageBatchSendHandler) Destroy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, _, ok := h.resolveAuthorized(w, r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	batchID, has, err := intParam(params, "batchId")
	if err != nil || !has || batchID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "batchId 必填", nil)
		return
	}
	if _, ok = h.loadOwnedBatch(w, r, userID, batchID, true); !ok {
		return
	}
	deleted, err := h.store.DeleteContactMessageBatchSend(r.Context(), batchID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "客户消息删除失败", nil)
		return
	}
	if !deleted {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "未找到记录", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *ContactMessageBatchSendHandler) resolveAuthorized(w http.ResponseWriter, r *http.Request) (int, User, DashboardRequestScope, bool) {
	userID, user, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return 0, User{}, DashboardRequestScope{}, false
	}
	if h.authorizer != nil {
		corpID := 0
		if len(principalScope.CorpIDs) > 0 {
			corpID = principalScope.CorpIDs[0]
		}
		if _, err := h.authorizer.Resolve(r.Context(), userID, PermissionKeyFromRequest(r), corpID, principalScope.WorkEmployeeID); err != nil {
			writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, err.Error(), nil)
			return 0, User{}, DashboardRequestScope{}, false
		}
	}
	return userID, user, principalScope, true
}

func (h *ContactMessageBatchSendHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (int, User, DashboardRequestScope, bool) {
	requestPrincipal, err := DashboardPrincipalFromContext(r.Context())
	userID := requestPrincipal.UserID
	if err != nil || userID <= 0 {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return 0, User{}, DashboardRequestScope{}, false
	}
	user, found, err := h.store.UserByID(r.Context(), userID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return 0, User{}, DashboardRequestScope{}, false
	}
	if !found {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "user not found", nil)
		return 0, User{}, DashboardRequestScope{}, false
	}
	principalScope, err := DashboardRequestScopeFromContext(r.Context())
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return 0, User{}, DashboardRequestScope{}, false
	}
	return userID, user, principalScope, true
}

func (h *ContactMessageBatchSendHandler) loadOwnedBatch(w http.ResponseWriter, r *http.Request, userID int, batchID int, prechecked ...bool) (ContactMessageBatchSendItem, bool) {
	if len(prechecked) == 0 || !prechecked[0] {
		if batchID <= 0 {
			return ContactMessageBatchSendItem{}, false
		}
	}
	batch, found, err := h.store.ContactMessageBatchSendByID(r.Context(), batchID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return ContactMessageBatchSendItem{}, false
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "未找到记录", nil)
		return ContactMessageBatchSendItem{}, false
	}
	if batch.UserID != userID {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, "无操作权限", nil)
		return ContactMessageBatchSendItem{}, false
	}
	return batch, true
}

func (h *ContactMessageBatchSendHandler) batchWriteFromParams(w http.ResponseWriter, ctx context.Context, params map[string]any, corpID int, userID int, user User) (ContactMessageBatchSendWrite, bool) {
	employeeIDs, err := intSliceParam(params, "employeeIds")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "employeeIds 必须为整型数组", nil)
		return ContactMessageBatchSendWrite{}, false
	}
	if len(employeeIDs) == 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "employeeIds 必填", nil)
		return ContactMessageBatchSendWrite{}, false
	}
	sendWay, has, err := intParam(params, "sendWay")
	if err != nil || !has || (sendWay != 1 && sendWay != 2) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "sendWay 值必须在列表内：[1,2]", nil)
		return ContactMessageBatchSendWrite{}, false
	}
	filterParams, ok := contactMessageBatchSendFilterParams(w, params["filterParams"])
	if !ok {
		return ContactMessageBatchSendWrite{}, false
	}
	content, ok := contactMessageBatchSendContents(w, params["content"])
	if !ok {
		return ContactMessageBatchSendWrite{}, false
	}
	mediumID, mediumPresent, mediumErr := intParam(params, "mediumId")
	if mediumErr != nil || (mediumPresent && mediumID < 0) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "mediumId 必须为非负整数", nil)
		return ContactMessageBatchSendWrite{}, false
	}
	if mediumID > 0 {
		validator, configured := h.store.(mediumAvailabilityValidator)
		if !configured {
			writeEnvelope(w, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "素材 Provider 未配置", nil)
			return ContactMessageBatchSendWrite{}, false
		}
		available, err := validator.MediumAvailableToUser(ctx, corpID, userID, mediumID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return ContactMessageBatchSendWrite{}, false
		}
		if !available {
			writeEnvelope(w, http.StatusConflict, http.StatusConflict, "素材不可用于当前企业或权限范围", nil)
			return ContactMessageBatchSendWrite{}, false
		}
	}
	definiteTime := strings.TrimSpace(stringParam(params, "definiteTime"))
	if sendWay == 2 && definiteTime == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "definiteTime 必填", nil)
		return ContactMessageBatchSendWrite{}, false
	}
	if definiteTime != "" {
		if _, err := time.ParseInLocation("2006-01-02 15:04:05", definiteTime, time.Local); err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "definiteTime 日期格式错误", nil)
			return ContactMessageBatchSendWrite{}, false
		}
	}
	detail, err := h.store.ContactMessageBatchSendFilterDetail(ctx, filterParams)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return ContactMessageBatchSendWrite{}, false
	}
	filterJSON := mustJSON(contactMessageBatchSendFilterParamsPayload(filterParams))
	detailJSON := mustJSON(detail)
	contentJSON := mustJSON(content)
	userName := strings.TrimSpace(user.Name)
	if userName == "" {
		userName = strings.TrimSpace(user.Phone)
	}
	return ContactMessageBatchSendWrite{
		CorpID:             corpID,
		UserID:             userID,
		MediumID:           mediumID,
		UserName:           userName,
		EmployeeIDs:        uniquePositiveIntsLocal(employeeIDs),
		FilterParams:       filterParams,
		FilterParamsJSON:   filterJSON,
		FilterDetailJSON:   detailJSON,
		Content:            content,
		ContentJSON:        contentJSON,
		SendWay:            sendWay,
		DefiniteTime:       definiteTime,
		ProcessImmediately: sendWay == 1,
	}, true
}

func (h *ContactMessageBatchSendHandler) sendBatchNow(w http.ResponseWriter, ctx context.Context, corpID int, batchID int) bool {
	_ = corpID
	sent, err := submitContactMessageBatchSend(ctx, h.store, h.client, h.fileStorageRoot, batchID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return false
	}
	if !sent {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "未找到可发送记录", nil)
		return false
	}
	return true
}

func (h *ContactMessageBatchSendHandler) prepareSendContent(w http.ResponseWriter, ctx context.Context, credential RoomWelcomeCorpCredential, content []ContactMessageBatchSendContent) ([]ContactMessageBatchSendContent, bool) {
	prepared := make([]ContactMessageBatchSendContent, 0, len(content))
	for _, item := range content {
		switch item.MsgType {
		case "image":
			if item.PicURL != "" {
				localPath, filePath, err := contactMessageBatchSendMediaFilePath(h.fileStorageRoot, item.PicURL)
				if err != nil {
					writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "图片处理失败", nil)
					return nil, false
				}
				mediaID, err := h.client.UploadTemporaryImage(ctx, credential, filePath)
				if err != nil {
					writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "上传图片失败", nil)
					return nil, false
				}
				item.PicURL = localPath
				item.MediaID = mediaID
			}
		case "miniprogram":
			if item.PicURL != "" {
				localPath, filePath, err := contactMessageBatchSendMediaFilePath(h.fileStorageRoot, item.PicURL)
				if err != nil {
					writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "图片处理失败", nil)
					return nil, false
				}
				mediaID, err := h.client.UploadTemporaryImage(ctx, credential, filePath)
				if err != nil {
					writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "上传图片失败", nil)
					return nil, false
				}
				item.PicURL = localPath
				item.PicMediaID = mediaID
			}
		}
		prepared = append(prepared, item)
	}
	return prepared, true
}

func (h *ContactMessageBatchSendHandler) batchListPayload(item ContactMessageBatchSendItem) map[string]any {
	return map[string]any{
		"id":               item.ID,
		"mediumId":         item.MediumID,
		"sendWay":          item.SendWay,
		"content":          h.contentPayload(item.Content),
		"sendTime":         item.SendTime,
		"sendTotal":        item.SendTotal,
		"notSendTotal":     item.NotSendTotal,
		"receivedTotal":    item.ReceivedTotal,
		"notReceivedTotal": item.NotReceivedTotal,
		"definiteTime":     item.DefiniteTime,
		"sendStatus":       item.SendStatus,
		"createdAt":        item.CreatedAt,
	}
}

func (h *ContactMessageBatchSendHandler) batchShowPayload(item ContactMessageBatchSendItem) map[string]any {
	return map[string]any{
		"id":                 item.ID,
		"mediumId":           item.MediumID,
		"creator":            item.UserName,
		"createdAt":          item.CreatedAt,
		"content":            h.contentPayload(item.Content),
		"sendTime":           item.SendTime,
		"filterParams":       contactMessageBatchSendFilterParamsPayload(item.FilterParams),
		"filterParamsDetail": item.FilterParamsDetail,
		"sendEmployeeTotal":  item.SendEmployeeTotal,
		"sendContactTotal":   item.SendContactTotal,
		"sendTotal":          item.SendTotal,
		"receivedTotal":      item.ReceivedTotal,
		"notSendTotal":       item.NotSendTotal,
		"notReceivedTotal":   item.NotReceivedTotal,
		"receiveLimitTotal":  item.ReceiveLimitTotal,
		"notFriendTotal":     item.NotFriendTotal,
	}
}

func (h *ContactMessageBatchSendHandler) contentPayload(content []ContactMessageBatchSendContent) []map[string]any {
	list := make([]map[string]any, 0, len(content))
	for _, item := range content {
		payload := map[string]any{"msgType": item.MsgType}
		if item.Content != "" {
			payload["content"] = item.Content
		}
		if item.MediaID != "" {
			payload["media_id"] = item.MediaID
		}
		if item.PicURL != "" {
			payload["pic_url"] = h.fullStaticURL(item.PicURL)
		}
		if item.Title != "" {
			payload["title"] = item.Title
		}
		if item.Desc != "" {
			payload["desc"] = item.Desc
		}
		if item.URL != "" {
			payload["url"] = item.URL
		}
		if item.AppID != "" {
			payload["appid"] = item.AppID
		}
		if item.Page != "" {
			payload["page"] = item.Page
		}
		if item.PicMediaID != "" {
			payload["pic_media_id"] = item.PicMediaID
		}
		list = append(list, payload)
	}
	return list
}

func (h *ContactMessageBatchSendHandler) fullStaticURL(path string) string {
	if path == "" || strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") || h.apiBaseURL == "" {
		return path
	}
	return h.apiBaseURL + "/static/" + strings.TrimLeft(path, "/")
}

func contactMessageBatchSendFilterParams(w http.ResponseWriter, raw any) (ContactMessageBatchSendFilterParams, bool) {
	if raw == nil {
		return ContactMessageBatchSendFilterParams{}, true
	}
	params, ok := normalizeMapAny(raw)
	if !ok {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "filterParams 必须为对象", nil)
		return ContactMessageBatchSendFilterParams{}, false
	}
	var out ContactMessageBatchSendFilterParams
	if gender, exists, err := intFromMap(params, "gender"); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "gender 必须为整数", nil)
		return ContactMessageBatchSendFilterParams{}, false
	} else if exists {
		out.Gender = &gender
	}
	out.AddTimeStart = strings.TrimSpace(fmt.Sprint(params["addTimeStart"]))
	if out.AddTimeStart == "<nil>" {
		out.AddTimeStart = ""
	}
	out.AddTimeEnd = strings.TrimSpace(fmt.Sprint(params["addTimeEnd"]))
	if out.AddTimeEnd == "<nil>" {
		out.AddTimeEnd = ""
	}
	var err error
	out.Rooms, err = intSliceFromAny(params["rooms"])
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "rooms 必须为整型数组", nil)
		return ContactMessageBatchSendFilterParams{}, false
	}
	out.Tags, err = intSliceFromAny(params["tags"])
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "tags 必须为整型数组", nil)
		return ContactMessageBatchSendFilterParams{}, false
	}
	out.ExcludeContacts, err = intSliceFromAny(params["excludeContacts"])
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "excludeContacts 必须为整型数组", nil)
		return ContactMessageBatchSendFilterParams{}, false
	}
	return out, true
}

func contactMessageBatchSendContents(w http.ResponseWriter, raw any) ([]ContactMessageBatchSendContent, bool) {
	if raw == nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "消息内容不能为空", nil)
		return nil, false
	}
	var items []map[string]any
	switch typed := raw.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "消息内容不能为空", nil)
			return nil, false
		}
		if err := json.Unmarshal([]byte(typed), &items); err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "content 必须为JSON数组", nil)
			return nil, false
		}
	case []any:
		for _, value := range typed {
			item, ok := normalizeMapAny(value)
			if !ok {
				writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "content 必须为对象数组", nil)
				return nil, false
			}
			items = append(items, item)
		}
	default:
		if encoded, err := json.Marshal(typed); err == nil {
			if err := json.Unmarshal(encoded, &items); err != nil {
				writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "content 必须为JSON数组", nil)
				return nil, false
			}
		}
	}
	if len(items) == 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "消息内容不能为空", nil)
		return nil, false
	}
	out := make([]ContactMessageBatchSendContent, 0, len(items))
	for _, item := range items {
		msgType := strings.TrimSpace(fmt.Sprint(item["msgType"]))
		content := ContactMessageBatchSendContent{
			MsgType:    msgType,
			Content:    strings.TrimSpace(fmt.Sprint(item["content"])),
			MediaID:    strings.TrimSpace(fmt.Sprint(item["media_id"])),
			PicURL:     strings.TrimSpace(fmt.Sprint(item["pic_url"])),
			Title:      strings.TrimSpace(fmt.Sprint(item["title"])),
			Desc:       strings.TrimSpace(fmt.Sprint(item["desc"])),
			URL:        strings.TrimSpace(fmt.Sprint(item["url"])),
			AppID:      strings.TrimSpace(fmt.Sprint(item["appid"])),
			Page:       strings.TrimSpace(fmt.Sprint(item["page"])),
			PicMediaID: strings.TrimSpace(fmt.Sprint(item["pic_media_id"])),
		}
		content = cleanNilStrings(content)
		switch msgType {
		case "text":
			if content.Content == "" {
				writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "文本内容不能为空", nil)
				return nil, false
			}
			if len(content.Content) > contactMessageBatchSendTextLimit {
				writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "文本内容不可超过4000字", nil)
				return nil, false
			}
		case "image":
			if content.MediaID == "" && content.PicURL == "" {
				writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "图片不能为空", nil)
				return nil, false
			}
		case "link":
			if content.Title == "" || content.URL == "" {
				writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "链接标题和地址不能为空", nil)
				return nil, false
			}
			if len(content.Desc) > 512 {
				writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "链接摘要不可超过512字", nil)
				return nil, false
			}
		case "miniprogram":
			if content.Title == "" || content.PicMediaID == "" || content.AppID == "" || content.Page == "" {
				writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "小程序标题、封面、appid和page不能为空", nil)
				return nil, false
			}
			if len(content.Title) > 64 {
				writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "小程序标题不可超过64字", nil)
				return nil, false
			}
			if content.PicURL == "" {
				content.PicURL = content.PicMediaID
			}
		default:
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "暂不支持的消息类型", nil)
			return nil, false
		}
		out = append(out, content)
	}
	return out, true
}

func cleanNilStrings(item ContactMessageBatchSendContent) ContactMessageBatchSendContent {
	clean := func(value string) string {
		if value == "<nil>" {
			return ""
		}
		return value
	}
	item.MsgType = clean(item.MsgType)
	item.Content = clean(item.Content)
	item.MediaID = clean(item.MediaID)
	item.PicURL = clean(item.PicURL)
	item.Title = clean(item.Title)
	item.Desc = clean(item.Desc)
	item.URL = clean(item.URL)
	item.AppID = clean(item.AppID)
	item.Page = clean(item.Page)
	item.PicMediaID = clean(item.PicMediaID)
	return item
}

func contactMessageBatchSendFilterParamsPayload(params ContactMessageBatchSendFilterParams) map[string]any {
	payload := map[string]any{}
	if params.Gender != nil {
		payload["gender"] = *params.Gender
	}
	if params.AddTimeStart != "" {
		payload["addTimeStart"] = params.AddTimeStart
	}
	if params.AddTimeEnd != "" {
		payload["addTimeEnd"] = params.AddTimeEnd
	}
	if len(params.Rooms) > 0 {
		payload["rooms"] = params.Rooms
	}
	if len(params.Tags) > 0 {
		payload["tags"] = params.Tags
	}
	if len(params.ExcludeContacts) > 0 {
		payload["excludeContacts"] = params.ExcludeContacts
	}
	return payload
}

func normalizeMapAny(raw any) (map[string]any, bool) {
	switch typed := raw.(type) {
	case map[string]any:
		return typed, true
	case string:
		var out map[string]any
		if strings.TrimSpace(typed) == "" {
			return map[string]any{}, true
		}
		if err := json.Unmarshal([]byte(typed), &out); err != nil {
			return nil, false
		}
		return out, true
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return nil, false
		}
		var out map[string]any
		if err := json.Unmarshal(encoded, &out); err != nil {
			return nil, false
		}
		return out, true
	}
}

func intFromMap(params map[string]any, key string) (int, bool, error) {
	value, ok := params[key]
	if !ok || value == nil || strings.TrimSpace(fmt.Sprint(value)) == "" || strings.TrimSpace(fmt.Sprint(value)) == "<nil>" {
		return 0, false, nil
	}
	return intParam(map[string]any{key: value}, key)
}

func intSliceFromAny(raw any) ([]int, error) {
	if raw == nil || strings.TrimSpace(fmt.Sprint(raw)) == "<nil>" {
		return []int{}, nil
	}
	return intSliceParam(map[string]any{"value": raw}, "value")
}

func mustJSON(value any) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}

func queryPositiveIntParam(w http.ResponseWriter, r *http.Request, key string, requiredMsg string, integerMsg string) (int, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, requiredMsg, nil)
		return 0, false
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, integerMsg, nil)
		return 0, false
	}
	return value, true
}

func optionalQueryInt(w http.ResponseWriter, r *http.Request, key string, integerMsg string) (*int, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return nil, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, integerMsg, nil)
		return nil, false
	}
	return &value, true
}

func formatUnixSecondsLocal(value int) string {
	if value <= 0 {
		return ""
	}
	return time.Unix(int64(value), 0).Format("2006-01-02 15:04:05")
}

func contactMessageBatchSendReminderText(createdAt string) string {
	return "【任务提醒】有新的任务啦！\n" +
		"任务类型：客户群发任务\n" +
		"创建时间：" + createdAt + "\n" +
		"可前往【客户联系】中确认发送，记得及时完成哦\n"
}

func contactMessageBatchSendMediaFilePath(root string, raw string) (string, string, error) {
	if raw == "" || strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw, raw, nil
	}
	if strings.HasPrefix(raw, "data:") || looksLikeBase64(raw) {
		payload := raw
		if index := strings.Index(payload, ","); strings.HasPrefix(payload, "data:") && index >= 0 {
			payload = payload[index+1:]
		}
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(payload))
		if err != nil {
			decoded, err = base64.RawStdEncoding.DecodeString(strings.TrimSpace(payload))
		}
		if err != nil {
			return "", "", err
		}
		name := time.Now().Format("20060102150405") + "_" + randomHexString(8) + ".jpg"
		relative := filepath.ToSlash(filepath.Join("image", "contactMessageBatchSend", name))
		absolute := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(absolute), 0755); err != nil {
			return "", "", err
		}
		if err := os.WriteFile(absolute, decoded, 0644); err != nil {
			return "", "", err
		}
		return relative, absolute, nil
	}
	if filepath.IsAbs(raw) {
		return raw, raw, nil
	}
	return raw, filepath.Join(root, strings.TrimLeft(raw, "/")), nil
}
