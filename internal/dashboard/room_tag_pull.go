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

const defaultRoomTagPullFileStorageRoot = "./storage/upload/static"

type RoomTagPullFilter struct {
	CorpID    int
	CreatorID *int
	Name      string
	Page      int
	PerPage   int
}

type RoomTagPullPage struct {
	Items     []RoomTagPullItem
	Total     int
	TotalPage int
	PerPage   int
}

type RoomTagPullItem struct {
	ID          int
	Name        string
	Employees   []string
	Rooms       []string
	InviteNum   int
	JoinRoomNum int
	NoSendNum   int
	NoInviteNum int
	CreatedAt   string
}

type RoomTagPullEmployee struct {
	Name     string `json:"name"`
	Avatar   string `json:"avatar"`
	WXUserID string `json:"wxUserId"`
}

type RoomTagPullShowRoom struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	RoomMax    int    `json:"room_max"`
	ContactNum int    `json:"contact_num"`
}

type RoomTagPullShow struct {
	Employees     []RoomTagPullEmployee
	Rooms         []RoomTagPullShowRoom
	JoinRoomNum   int
	NoJoinRoomNum int
	InviteNum     int
	NoInviteNum   int
	SendNum       int
	NoSendNum     int
}

type RoomTagPullContactFilter struct {
	RoomTagPullID int
	ContactName   string
	WXUserID      string
	SendStatus    *int
	IsJoinRoom    *int
	RoomID        *int
	Page          int
	PerPage       int
}

type RoomTagPullContactPage struct {
	Items      []RoomTagPullContactItem
	Total      int
	TotalPage  int
	PerPage    int
	ContactNum int
}

type RoomTagPullContactItem struct {
	Avatar       string `json:"avatar"`
	ContactName  string `json:"contact_name"`
	EmployeeName string `json:"employee_name"`
	SendStatus   int    `json:"send_status"`
	RoomName     string `json:"room_name"`
	IsJoinRoom   int    `json:"is_join_room"`
}

type RoomTagPullEmployeeTaskFilter struct {
	RoomTagPullID int
	WXUserID      string
	IsSend        *int
	TaskType      *int
}

type RoomTagPullEmployeeTask struct {
	WXUserID   string `json:"wxUserId"`
	Status     int    `json:"status"`
	TaskNum    int    `json:"task_num"`
	Name       string `json:"name"`
	Avatar     string `json:"avatar"`
	ContactNum int    `json:"contact_num"`
	InviteNum  int    `json:"invite_num"`
}

type RoomTagPullRoomFilter struct {
	CorpID      int
	EmployeeIDs []int
	Name        string
	Type        int
}

type RoomTagPullRoom struct {
	ID          int    `json:"id"`
	WXChatID    string `json:"wxChatId"`
	Name        string `json:"name"`
	OwnerID     int    `json:"ownerId"`
	RoomMax     int    `json:"roomMax"`
	ContactNum  int    `json:"contact_num"`
	AuditStatus int    `json:"audit_status,omitempty"`
}

type RoomTagPullContactSearch struct {
	Employees []int
	IsAll     int
	Gender    *int
	StartTime string
	EndTime   string
	TagIDs    []int
}

type RoomTagPullWriteRoom struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Num     int    `json:"num"`
	Image   string `json:"image,omitempty"`
	WXImage string `json:"wx_image,omitempty"`
}

type RoomTagPullWrite struct {
	Name              string
	EmployeeIDs       []int
	Employees         string
	ChooseContact     RoomTagPullContactSearch
	ChooseContactJSON string
	Guide             string
	Rooms             []RoomTagPullWriteRoom
	RoomsJSON         string
	FilterContact     int
	ContactNum        int
	WXTIDJSON         string
	TenantID          int
	CorpID            int
	CreateUserID      int
}

type RoomTagPullSendTarget struct {
	EmployeeID int
	WXUserID   string
	Contacts   []RoomTagPullSendContact
}

type RoomTagPullSendContact struct {
	ContactID        int
	WXExternalUserID string
	ContactName      string
	EmployeeID       int
	WXUserID         string
}

type RoomTagPullMessagePayload struct {
	TextContent     string
	ImagePicURL     string
	ExternalUserIDs []string
	Sender          string
}

type RoomTagPullWXTID struct {
	WXUserID string `json:"wxUserId"`
	TID      string `json:"tid"`
	Status   int    `json:"status"`
}

type RoomTagPullRemindActivity struct {
	EmployeeIDs   []int
	ChooseContact RoomTagPullContactSearch
	ContactNum    int
	WXTIDs        []RoomTagPullWXTID
	CreatedAt     string
}

type RoomTagPullAgentCredential struct {
	CorpID    int
	WXCorpID  string
	WXAgentID string
	WXSecret  string
}

type RoomTagPullStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	RoomWelcomeCorpCredentialByID(ctx context.Context, corpID int) (RoomWelcomeCorpCredential, bool, error)
	RoomTagPullRemindAgentByCorpID(ctx context.Context, corpID int) (RoomTagPullAgentCredential, bool, error)
	RoomTagPullPage(ctx context.Context, filter RoomTagPullFilter) (RoomTagPullPage, error)
	RoomTagPullByID(ctx context.Context, id int) (RoomTagPullShow, bool, error)
	RoomTagPullRemindByID(ctx context.Context, id int) (RoomTagPullRemindActivity, bool, error)
	RoomTagPullContactPage(ctx context.Context, filter RoomTagPullContactFilter) (RoomTagPullContactPage, error)
	RoomTagPullEmployeeTasks(ctx context.Context, filter RoomTagPullEmployeeTaskFilter) ([]RoomTagPullEmployeeTask, int, error)
	RoomTagPullRooms(ctx context.Context, filter RoomTagPullRoomFilter) ([]RoomTagPullRoom, error)
	CountRoomTagPullSearchContacts(ctx context.Context, corpID int, search RoomTagPullContactSearch) (int, error)
	CountRoomTagPullFilteredContacts(ctx context.Context, corpID int, search RoomTagPullContactSearch, rooms []RoomTagPullWriteRoom, filterContact int) (int, error)
	RoomTagPullSendTargets(ctx context.Context, corpID int, employeeIDs []int, search RoomTagPullContactSearch) ([]RoomTagPullSendTarget, error)
	CreateRoomTagPull(ctx context.Context, values RoomTagPullWrite) (int, error)
	DeleteRoomTagPull(ctx context.Context, id int) (bool, error)
}

type RoomTagPullMessageClient interface {
	UploadImage(ctx context.Context, credential RoomWelcomeCorpCredential, filePath string) (string, error)
	CreateExternalContactMessage(ctx context.Context, credential RoomWelcomeCorpCredential, payload RoomTagPullMessagePayload) (string, error)
	SendAgentTextMessage(ctx context.Context, credential RoomTagPullAgentCredential, toUser string, content string) error
}

type RoomTagPullHandler struct {
	store           RoomTagPullStore
	cache           LoginCache
	resolver        UserIDResolver
	authorizer      CorpAdminAuthorizer
	apiBaseURL      string
	fileStorageRoot string
	messageClient   RoomTagPullMessageClient
}

func NewRoomTagPullHandler(store RoomTagPullStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer, apiBaseURL string) *RoomTagPullHandler {
	return NewRoomTagPullHandlerWithMessageClient(store, cache, resolver, authorizer, apiBaseURL, defaultRoomTagPullFileStorageRoot, NewRoomWelcomeWeComClient(defaultWeComAPIBaseURL))
}

func NewRoomTagPullHandlerWithMessageClient(
	store RoomTagPullStore,
	cache LoginCache,
	resolver UserIDResolver,
	authorizer CorpAdminAuthorizer,
	apiBaseURL string,
	fileStorageRoot string,
	messageClient RoomTagPullMessageClient,
) *RoomTagPullHandler {
	if strings.TrimSpace(fileStorageRoot) == "" {
		fileStorageRoot = defaultRoomTagPullFileStorageRoot
	}
	return &RoomTagPullHandler{
		store:           store,
		cache:           cache,
		resolver:        resolver,
		authorizer:      authorizer,
		apiBaseURL:      strings.TrimRight(apiBaseURL, "/"),
		fileStorageRoot: fileStorageRoot,
		messageClient:   messageClient,
	}
}

func (h *RoomTagPullHandler) Index(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, user, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if _, err := h.authorizeAccess(r.Context(), r, userID, principalScope); err != nil {
		writeAccessError(w, err)
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	var creatorID *int
	if user.IsSuperAdmin == 0 {
		creator := user.ID
		creatorID = &creator
	}
	page, err := h.store.RoomTagPullPage(r.Context(), RoomTagPullFilter{
		CorpID:    corpID,
		CreatorID: creatorID,
		Name:      strings.TrimSpace(r.URL.Query().Get("name")),
		Page:      positiveQueryInt(r, "page", 1),
		PerPage:   positiveQueryInt(r, "perPage", 10000),
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, map[string]any{
			"id":            item.ID,
			"name":          item.Name,
			"employees":     item.Employees,
			"rooms":         item.Rooms,
			"invite_num":    item.InviteNum,
			"join_room_num": item.JoinRoomNum,
			"no_send_num":   item.NoSendNum,
			"no_invite_num": item.NoInviteNum,
			"created_at":    item.CreatedAt,
		})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"page": map[string]any{
			"perPage":   strconv.Itoa(page.PerPage),
			"total":     page.Total,
			"totalPage": page.TotalPage,
		},
		"list": list,
	})
}

func (h *RoomTagPullHandler) Show(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if _, err := h.authorizeAccess(r.Context(), r, userID, principalScope); err != nil {
		writeAccessError(w, err)
		return
	}
	if _, ok := principalCorpID(w, r); !ok {
		return
	}
	id, ok := requiredPositiveQueryInt(w, r, "id", "活动id 必填", "活动id 必须为整型")
	if !ok {
		return
	}
	info, found, err := h.store.RoomTagPullByID(r.Context(), id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "标签建群活动不存在", nil)
		return
	}
	for index := range info.Employees {
		info.Employees[index].Avatar = h.fileFullURL(info.Employees[index].Avatar)
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"employees":        info.Employees,
		"rooms":            info.Rooms,
		"join_room_num":    info.JoinRoomNum,
		"no_join_room_num": info.NoJoinRoomNum,
		"invite_num":       info.InviteNum,
		"no_invite_num":    info.NoInviteNum,
		"send_num":         info.SendNum,
		"no_send_num":      info.NoSendNum,
	})
}

func (h *RoomTagPullHandler) ShowContact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if _, err := h.authorizeAccess(r.Context(), r, userID, principalScope); err != nil {
		writeAccessError(w, err)
		return
	}
	if _, ok := principalCorpID(w, r); !ok {
		return
	}
	id, ok := requiredPositiveQueryInt(w, r, "id", "活动id 必填", "活动id 必须为整型")
	if !ok {
		return
	}
	viewType, ok := requiredPositiveQueryInt(w, r, "type", "type 必填", "type 必须为整型")
	if !ok {
		return
	}
	switch viewType {
	case 1:
		filter := RoomTagPullContactFilter{
			RoomTagPullID: id,
			ContactName:   strings.TrimSpace(r.URL.Query().Get("contact_name")),
			WXUserID:      strings.TrimSpace(r.URL.Query().Get("wx_user_id")),
			Page:          positiveQueryInt(r, "page", 1),
			PerPage:       positiveQueryInt(r, "perPage", 10000),
		}
		filter.SendStatus = optionalQueryIntPtr(r, "send_status")
		filter.IsJoinRoom = optionalQueryIntPtr(r, "is_join_room")
		filter.RoomID = optionalQueryIntPtr(r, "room_id")
		page, err := h.store.RoomTagPullContactPage(r.Context(), filter)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		for index := range page.Items {
			page.Items[index].Avatar = h.fileFullURL(page.Items[index].Avatar)
		}
		writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
			"page": map[string]any{
				"perPage":   strconv.Itoa(page.PerPage),
				"total":     page.Total,
				"totalPage": page.TotalPage,
			},
			"list":        page.Items,
			"contact_num": page.ContactNum,
		})
	case 2:
		filter := RoomTagPullEmployeeTaskFilter{
			RoomTagPullID: id,
			WXUserID:      strings.TrimSpace(r.URL.Query().Get("wx_user_id")),
			IsSend:        optionalQueryIntPtr(r, "is_send"),
			TaskType:      optionalQueryIntPtr(r, "task_type"),
		}
		list, contactNum, err := h.store.RoomTagPullEmployeeTasks(r.Context(), filter)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		for index := range list {
			list[index].Avatar = h.fileFullURL(list[index].Avatar)
		}
		writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
			"list":        list,
			"contact_num": contactNum,
		})
	default:
		writeEnvelope(w, http.StatusOK, 200, "success", []any{})
	}
}

func (h *RoomTagPullHandler) RoomList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if _, err := h.authorizeAccess(r.Context(), r, userID, principalScope); err != nil {
		writeAccessError(w, err)
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	params := valuesToParams(r.URL.Query())
	employeeIDs, err := intSliceParam(params, "employees")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "employees 必须为整型数组", nil)
		return
	}
	roomType, _, err := intParam(params, "type")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "type 必须为整型", nil)
		return
	}
	rooms, err := h.store.RoomTagPullRooms(r.Context(), RoomTagPullRoomFilter{
		CorpID:      corpID,
		EmployeeIDs: employeeIDs,
		Name:        strings.TrimSpace(r.URL.Query().Get("name")),
		Type:        roomType,
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", rooms)
}

func (h *RoomTagPullHandler) ChooseContact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if _, err := h.authorizeAccess(r.Context(), r, userID, principalScope); err != nil {
		writeAccessError(w, err)
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	search, ok := parseRoomTagPullContactSearch(w, valuesToParams(r.URL.Query()))
	if !ok {
		return
	}
	count, err := h.store.CountRoomTagPullSearchContacts(r.Context(), corpID, search)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []int{count})
}

func (h *RoomTagPullHandler) FilterContact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if _, err := h.authorizeAccess(r.Context(), r, userID, principalScope); err != nil {
		writeAccessError(w, err)
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	search, rooms, ok := parseRoomTagPullFilterContactParams(w, params)
	if !ok {
		return
	}
	count, err := h.store.CountRoomTagPullFilteredContacts(r.Context(), corpID, search, rooms, 1)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []int{count})
}

func (h *RoomTagPullHandler) Store(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, user, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if _, err := h.authorizeAccess(r.Context(), r, userID, principalScope); err != nil {
		writeAccessError(w, err)
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	if h.messageClient == nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "企业微信群发客户端未配置", nil)
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	values, ok := parseRoomTagPullStoreParams(w, params, corpID, user.ID)
	if !ok {
		return
	}
	if !enforceSaaSQuota(r.Context(), w, h.store, user.TenantID, SaaSMetricRoomTagPulls, 1) {
		return
	}
	credential, ok := h.resolveCorpCredential(w, r.Context(), corpID)
	if !ok {
		return
	}
	rooms, ok := h.prepareRoomTagPullRooms(w, r.Context(), credential, values.Rooms)
	if !ok {
		return
	}
	values.Rooms = rooms
	values.RoomsJSON = mustJSONText(rooms)
	targets, err := h.store.RoomTagPullSendTargets(r.Context(), corpID, values.EmployeeIDs, values.ChooseContact)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	tids, ok := h.sendRoomTagPullMessages(w, r.Context(), credential, values.Guide, rooms, values.FilterContact, targets)
	if !ok {
		return
	}
	values.WXTIDJSON = mustJSONText(tids)
	contactNum, err := h.store.CountRoomTagPullFilteredContacts(r.Context(), corpID, values.ChooseContact, rooms, values.FilterContact)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	values.ContactNum = contactNum
	id, err := h.store.CreateRoomTagPull(r.Context(), values)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if err := refreshSaaSUsageCounter(r.Context(), h.store, user.TenantID, SaaSMetricRoomTagPulls); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []int{id})
}

func (h *RoomTagPullHandler) RemindSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if _, err := h.authorizeAccess(r.Context(), r, userID, principalScope); err != nil {
		writeAccessError(w, err)
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	if h.messageClient == nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "企业微信提醒客户端未配置", nil)
		return
	}
	id, ok := requiredPositiveQueryInt(w, r, "id", "标签建群活动id 必传", "标签建群活动id 必须为整型")
	if !ok {
		return
	}
	activity, found, err := h.store.RoomTagPullRemindByID(r.Context(), id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "标签建群活动不存在", nil)
		return
	}
	agent, found, err := h.store.RoomTagPullRemindAgentByCorpID(r.Context(), corpID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "无可用的 agent", nil)
		return
	}
	targets, err := h.store.RoomTagPullSendTargets(r.Context(), corpID, activity.EmployeeIDs, activity.ChooseContact)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	contactNameByWXUserID := map[string]string{}
	for _, target := range targets {
		if target.WXUserID == "" || len(target.Contacts) == 0 {
			continue
		}
		contactNameByWXUserID[target.WXUserID] = target.Contacts[0].ContactName
	}
	onlyWXUserID := strings.TrimSpace(r.URL.Query().Get("wxUserId"))
	for _, tid := range activity.WXTIDs {
		if onlyWXUserID != "" && tid.WXUserID != onlyWXUserID {
			continue
		}
		if tid.Status != 0 || tid.WXUserID == "" {
			continue
		}
		contactName := strings.TrimSpace(contactNameByWXUserID[tid.WXUserID])
		if contactName == "" {
			contactName = "客户"
		}
		content := fmt.Sprintf("管理员提醒你发送群发任务\n任务创建于%s,将群发给%s等%d个客户，可前往【客户联系】中确认发送", activity.CreatedAt, contactName, activity.ContactNum)
		if err := h.messageClient.SendAgentTextMessage(r.Context(), agent, tid.WXUserID, content); err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "发送提醒失败", nil)
			return
		}
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *RoomTagPullHandler) Destroy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if _, err := h.authorizeAccess(r.Context(), r, userID, principalScope); err != nil {
		writeAccessError(w, err)
		return
	}
	if _, ok := principalCorpID(w, r); !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	if len(params) == 0 {
		params = valuesToParams(r.URL.Query())
	}
	id, ok, err := intParam(params, "id")
	if !ok {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "活动id 必填", nil)
		return
	}
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "活动id 必须为整型", nil)
		return
	}
	deleted, err := h.store.DeleteRoomTagPull(r.Context(), id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "标签建群删除失败"+err.Error(), nil)
		return
	}
	if !deleted {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "标签建群活动不存在", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *RoomTagPullHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (int, User, DashboardRequestScope, bool) {
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

func (h *RoomTagPullHandler) authorizeAccess(ctx context.Context, r *http.Request, userID int, principalScope DashboardRequestScope) (AccessContext, error) {
	if h.authorizer == nil {
		return AccessContext{DataPermission: DataPermissionAll}, nil
	}
	corpID := 0
	if len(principalScope.CorpIDs) > 0 {
		corpID = principalScope.CorpIDs[0]
	}
	return h.authorizer.Resolve(ctx, userID, PermissionKeyFromRequest(r), corpID, principalScope.WorkEmployeeID)
}

func (h *RoomTagPullHandler) fileFullURL(path string) string {
	if path == "" || strings.Contains(path, "http") || h.apiBaseURL == "" {
		return path
	}
	return h.apiBaseURL + "/static/" + strings.TrimLeft(path, "/")
}

func (h *RoomTagPullHandler) resolveCorpCredential(w http.ResponseWriter, ctx context.Context, corpID int) (RoomWelcomeCorpCredential, bool) {
	credential, found, err := h.store.RoomWelcomeCorpCredentialByID(ctx, corpID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return RoomWelcomeCorpCredential{}, false
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "企业授信信息不存在", nil)
		return RoomWelcomeCorpCredential{}, false
	}
	return credential, true
}

func (h *RoomTagPullHandler) prepareRoomTagPullRooms(w http.ResponseWriter, ctx context.Context, credential RoomWelcomeCorpCredential, rooms []RoomTagPullWriteRoom) ([]RoomTagPullWriteRoom, bool) {
	out := make([]RoomTagPullWriteRoom, 0, len(rooms))
	for _, room := range rooms {
		if room.Image != "" {
			localPath, filePath, ok := h.prepareRoomTagPullImage(w, room.Image)
			if !ok {
				return nil, false
			}
			wxImage, err := h.messageClient.UploadImage(ctx, credential, filePath)
			if err != nil {
				writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "上传图片失败", nil)
				return nil, false
			}
			room.Image = localPath
			room.WXImage = wxImage
		}
		out = append(out, room)
	}
	return out, true
}

func (h *RoomTagPullHandler) prepareRoomTagPullImage(w http.ResponseWriter, raw string) (string, string, bool) {
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw, raw, true
	}
	localPath := raw
	filePath := raw
	if strings.HasPrefix(raw, "data:") || looksLikeBase64(raw) {
		relative, absolute, err := h.saveRoomTagPullBase64Image(raw)
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "图片处理失败", nil)
			return "", "", false
		}
		localPath = relative
		filePath = absolute
	} else if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(h.fileStorageRoot, strings.TrimLeft(raw, "/"))
	}
	return localPath, filePath, true
}

func (h *RoomTagPullHandler) saveRoomTagPullBase64Image(raw string) (string, string, error) {
	payload := raw
	if index := strings.Index(payload, ","); strings.HasPrefix(payload, "data:") && index >= 0 {
		payload = payload[index+1:]
	}
	payload = strings.TrimSpace(payload)
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(payload)
	}
	if err != nil {
		return "", "", err
	}
	if len(decoded) <= 5 {
		return "", "", fmt.Errorf("image too small")
	}
	name := time.Now().Format("20060102150405") + "_" + randomHexString(8) + ".jpg"
	relative := filepath.ToSlash(filepath.Join("image", "roomTagPull", name))
	absolute := filepath.Join(h.fileStorageRoot, relative)
	if err := os.MkdirAll(filepath.Dir(absolute), 0755); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(absolute, decoded, 0644); err != nil {
		return "", "", err
	}
	return relative, absolute, nil
}

func (h *RoomTagPullHandler) sendRoomTagPullMessages(w http.ResponseWriter, ctx context.Context, credential RoomWelcomeCorpCredential, guide string, rooms []RoomTagPullWriteRoom, filterContact int, targets []RoomTagPullSendTarget) ([]RoomTagPullWXTID, bool) {
	tids := make([]RoomTagPullWXTID, 0)
	for _, target := range targets {
		if len(target.Contacts) == 0 || target.WXUserID == "" {
			continue
		}
		externalUserIDs := uniqueRoomTagPullExternalUserIDs(target.Contacts)
		start := 0
		for _, room := range rooms {
			if room.Num <= 0 {
				continue
			}
			sendContacts := sliceRoomTagPullContacts(target.Contacts, start, room.Num)
			if len(sendContacts) == 0 {
				break
			}
			start += room.Num
			if room.WXImage == "" {
				continue
			}
			msgID, err := h.messageClient.CreateExternalContactMessage(ctx, credential, RoomTagPullMessagePayload{
				TextContent:     guide,
				ImagePicURL:     room.WXImage,
				ExternalUserIDs: externalUserIDs,
				Sender:          target.WXUserID,
			})
			if err != nil {
				writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "发送失败", nil)
				return nil, false
			}
			tids = append(tids, RoomTagPullWXTID{WXUserID: target.WXUserID, TID: msgID, Status: 0})
			_ = filterContact
		}
	}
	return tids, true
}

func requiredPositiveQueryInt(w http.ResponseWriter, r *http.Request, key string, requiredMsg string, integerMsg string) (int, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, requiredMsg, nil)
		return 0, false
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, integerMsg, nil)
		return 0, false
	}
	return value, true
}

func optionalQueryIntPtr(r *http.Request, key string) *int {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return nil
	}
	return &value
}

func parseRoomTagPullContactSearch(w http.ResponseWriter, params map[string]any) (RoomTagPullContactSearch, bool) {
	employees, err := intSliceParam(params, "employees")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "employees 必须为整型数组", nil)
		return RoomTagPullContactSearch{}, false
	}
	isAll, ok, err := intParam(params, "is_all")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "is_all 必须为整型", nil)
		return RoomTagPullContactSearch{}, false
	}
	if !ok {
		isAll = 0
	}
	search := RoomTagPullContactSearch{
		Employees: employees,
		IsAll:     isAll,
		StartTime: stringParam(params, "start_time"),
		EndTime:   stringParam(params, "end_time"),
	}
	if raw := strings.TrimSpace(stringParam(params, "gender")); raw != "" {
		gender, err := strconv.Atoi(raw)
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "gender 必须为整型", nil)
			return RoomTagPullContactSearch{}, false
		}
		search.Gender = &gender
	}
	search.TagIDs, err = intSliceParam(params, "tag_ids")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "tag_ids 必须为整型数组", nil)
		return RoomTagPullContactSearch{}, false
	}
	return search, true
}

func parseRoomTagPullFilterContactParams(w http.ResponseWriter, params map[string]any) (RoomTagPullContactSearch, []RoomTagPullWriteRoom, bool) {
	employeeIDs, err := intSliceParam(params, "employees")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "employees 必须为整型数组", nil)
		return RoomTagPullContactSearch{}, nil, false
	}
	chooseContact, ok := roomTagPullMapParam(params, "choose_contact")
	if !ok {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "choose_contact 必须为对象", nil)
		return RoomTagPullContactSearch{}, nil, false
	}
	search, ok := roomTagPullSearchFromMap(w, chooseContact, employeeIDs)
	if !ok {
		return RoomTagPullContactSearch{}, nil, false
	}
	rooms, ok := roomTagPullRoomsParam(w, params)
	if !ok {
		return RoomTagPullContactSearch{}, nil, false
	}
	return search, rooms, true
}

func parseRoomTagPullStoreParams(w http.ResponseWriter, params map[string]any, corpID int, userID int) (RoomTagPullWrite, bool) {
	name := stringParam(params, "name")
	if name == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "规则名称 必传", nil)
		return RoomTagPullWrite{}, false
	}
	employeeIDs, err := intSliceParam(params, "employees")
	if err != nil || len(employeeIDs) == 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "选择群发账号 必传", nil)
		return RoomTagPullWrite{}, false
	}
	chooseContact, found := roomTagPullMapParam(params, "choose_contact")
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "筛选客户 必传", nil)
		return RoomTagPullWrite{}, false
	}
	search, ok := roomTagPullSearchFromMap(w, chooseContact, employeeIDs)
	if !ok {
		return RoomTagPullWrite{}, false
	}
	guide := stringParam(params, "guide")
	if guide == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "入群引导语 必传", nil)
		return RoomTagPullWrite{}, false
	}
	rooms, ok := roomTagPullRoomsParam(w, params)
	if !ok || len(rooms) == 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "群聊 必传", nil)
		return RoomTagPullWrite{}, false
	}
	filterContact, okInt, err := intParam(params, "filter_contact")
	if err != nil || !okInt {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "过滤客户 必传", nil)
		return RoomTagPullWrite{}, false
	}
	tenantID, _, _ := intParam(params, "tenant_id")
	return RoomTagPullWrite{
		Name:              name,
		EmployeeIDs:       employeeIDs,
		Employees:         joinInts(employeeIDs, ","),
		ChooseContact:     search,
		ChooseContactJSON: mustJSONText(chooseContact),
		Guide:             guide,
		Rooms:             rooms,
		RoomsJSON:         mustJSONText(rooms),
		FilterContact:     filterContact,
		TenantID:          tenantID,
		CorpID:            corpID,
		CreateUserID:      userID,
	}, true
}

func roomTagPullMapParam(params map[string]any, key string) (map[string]any, bool) {
	value, ok := params[key]
	if !ok || value == nil {
		return nil, false
	}
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case string:
		var decoded map[string]any
		decoder := json.NewDecoder(strings.NewReader(typed))
		decoder.UseNumber()
		if err := decoder.Decode(&decoded); err == nil {
			return decoded, true
		}
	}
	return nil, false
}

func roomTagPullSearchFromMap(w http.ResponseWriter, params map[string]any, employeeIDs []int) (RoomTagPullContactSearch, bool) {
	isAll, ok, err := intParam(params, "is_all")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "is_all 必须为整型", nil)
		return RoomTagPullContactSearch{}, false
	}
	if !ok {
		isAll = 0
	}
	search := RoomTagPullContactSearch{
		Employees: employeeIDs,
		IsAll:     isAll,
		StartTime: stringParam(params, "start_time"),
		EndTime:   stringParam(params, "end_time"),
	}
	if raw := strings.TrimSpace(stringParam(params, "gender")); raw != "" {
		gender, err := strconv.Atoi(raw)
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "gender 必须为整型", nil)
			return RoomTagPullContactSearch{}, false
		}
		search.Gender = &gender
	}
	var errTags error
	search.TagIDs, errTags = intSliceParam(params, "tag_ids")
	if errTags != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "tag_ids 必须为整型数组", nil)
		return RoomTagPullContactSearch{}, false
	}
	return search, true
}

func roomTagPullRoomsParam(w http.ResponseWriter, params map[string]any) ([]RoomTagPullWriteRoom, bool) {
	value, ok := params["rooms"]
	if !ok || value == nil {
		return nil, false
	}
	var values []any
	switch typed := value.(type) {
	case []any:
		values = typed
	case []map[string]any:
		for _, item := range typed {
			values = append(values, item)
		}
	case string:
		decoder := json.NewDecoder(strings.NewReader(typed))
		decoder.UseNumber()
		if err := decoder.Decode(&values); err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "rooms 必须为数组", nil)
			return nil, false
		}
	default:
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "rooms 必须为数组", nil)
		return nil, false
	}
	rooms := make([]RoomTagPullWriteRoom, 0, len(values))
	for _, value := range values {
		object, ok := value.(map[string]any)
		if !ok {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "rooms 必须为对象数组", nil)
			return nil, false
		}
		id, _, _ := intParam(object, "id")
		if id <= 0 {
			id, _, _ = intParam(object, "roomId")
		}
		num, _, _ := intParam(object, "num")
		if num <= 0 {
			num, _, _ = intParam(object, "maxNum")
		}
		rooms = append(rooms, RoomTagPullWriteRoom{
			ID:      id,
			Name:    firstNonEmptyString(object["name"], object["roomName"]),
			Num:     num,
			Image:   firstNonEmptyString(object["image"], object["roomQrcodeUrl"]),
			WXImage: firstNonEmptyString(object["wx_image"], object["wxImage"]),
		})
	}
	return rooms, true
}

func mustJSONText(value any) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}

func joinInts(values []int, sep string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.Itoa(value))
	}
	return strings.Join(parts, sep)
}

func uniqueRoomTagPullExternalUserIDs(contacts []RoomTagPullSendContact) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(contacts))
	for _, contact := range contacts {
		id := strings.TrimSpace(contact.WXExternalUserID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func sliceRoomTagPullContacts(contacts []RoomTagPullSendContact, start int, length int) []RoomTagPullSendContact {
	if start >= len(contacts) || length <= 0 {
		return []RoomTagPullSendContact{}
	}
	end := start + length
	if end > len(contacts) {
		end = len(contacts)
	}
	return contacts[start:end]
}
