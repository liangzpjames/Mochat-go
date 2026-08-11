package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type ChannelCodeGroup struct {
	ID   int
	Name string
}

type ChannelCodeListFilter struct {
	CorpIDs             []int
	Name                string
	Type                *int
	GroupID             *int
	RestrictBusinessIDs bool
	BusinessIDs         []int
	Page                int
	PerPage             int
}

type ChannelCodeListPage struct {
	Items     []ChannelCodeListItem
	Total     int
	TotalPage int
	PerPage   int
}

type ChannelCodeListItem struct {
	ID            int
	GroupID       int
	GroupName     string
	Name          string
	QRCodeURL     string
	AutoAddFriend int
	Tags          []string
	Type          int
	ContactNum    int
}

type ChannelCodeTagGroup struct {
	GroupID   int                  `json:"groupId"`
	GroupName string               `json:"groupName"`
	List      []ChannelCodeTagItem `json:"list"`
}

type ChannelCodeTagItem struct {
	TagID      int    `json:"tagId"`
	TagName    string `json:"tagName"`
	IsSelected int    `json:"isSelected"`
}

type ChannelCodeShow struct {
	GroupID          int
	GroupName        string
	Name             string
	AutoAddFriend    int
	TagGroups        []ChannelCodeTagGroup
	SelectedTags     []int
	DrainageEmployee any
	WelcomeMessage   any
}

type ChannelCodeContactFilter struct {
	ChannelCodeID int
	Page          int
	PerPage       int
}

type ChannelCodeContactPage struct {
	Items     []ChannelCodeContactItem
	Total     int
	TotalPage int
	PerPage   int
}

type ChannelCodeContactItem struct {
	ContactID  int
	EmployeeID int
	CreateTime string
	Name       string
	Employees  string
}

type ChannelCodeStatContact struct {
	Status    int
	CreateAt  string
	DeletedAt string
}

type ChannelCodeWriteValues struct {
	CorpID            int
	GroupID           int
	Name              string
	AutoAddFriend     int
	Tags              []int
	Type              int
	DrainageEmployee  map[string]any
	WelcomeMessage    map[string]any
	OperationEmployee int
}

type ChannelCodeStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	RoomWelcomeCorpCredentialByID(ctx context.Context, corpID int) (RoomWelcomeCorpCredential, bool, error)
	ChannelCodeGroupsByCorpIDs(ctx context.Context, corpIDs []int) ([]ChannelCodeGroup, error)
	ChannelCodeGroupByID(ctx context.Context, groupID int) (ChannelCodeGroup, bool, error)
	ChannelCodeGroupNamesExist(ctx context.Context, names []string) (bool, error)
	CreateChannelCodeGroups(ctx context.Context, corpID int, names []string) error
	UpdateChannelCodeGroupName(ctx context.Context, groupID int, name string) error
	MoveChannelCodeToGroup(ctx context.Context, channelCodeID int, groupID int) error
	CreateChannelCode(ctx context.Context, values ChannelCodeWriteValues) (int, error)
	UpdateChannelCode(ctx context.Context, channelCodeID int, values ChannelCodeWriteValues) (string, error)
	UpdateChannelCodeQRCode(ctx context.Context, channelCodeID int, qrCodeURL string, wxConfigID string) error
	DeleteChannelCode(ctx context.Context, channelCodeID int) error
	ChannelCodeEmployeeWXUserIDs(ctx context.Context, employeeIDs []int) ([]string, error)
	ChannelCodeContactCountsByEmployee(ctx context.Context, employeeIDs []int) (map[int]int, error)
	ChannelCodePage(ctx context.Context, filter ChannelCodeListFilter) (ChannelCodeListPage, error)
	ChannelCodeBusinessIDsByOperators(ctx context.Context, operationIDs []int) ([]int, error)
	ChannelCodeShowByID(ctx context.Context, channelCodeID int, corpID int) (ChannelCodeShow, bool, error)
	ChannelCodeContactPage(ctx context.Context, filter ChannelCodeContactFilter) (ChannelCodeContactPage, error)
	ChannelCodeStatContacts(ctx context.Context, channelCodeID int) ([]ChannelCodeStatContact, error)
}

type ChannelCodeWeComClient interface {
	CreateContactWay(ctx context.Context, credential RoomWelcomeCorpCredential, userIDs []string, skipVerify bool, state string) (qrCodeURL string, configID string, err error)
	UpdateContactWay(ctx context.Context, credential RoomWelcomeCorpCredential, configID string, userIDs []string, skipVerify bool, state string) error
}

type ChannelCodeHandler struct {
	store      ChannelCodeStore
	cache      LoginCache
	resolver   UserIDResolver
	authorizer CorpAdminAuthorizer
	wecom      ChannelCodeWeComClient
	apiBaseURL string
}

func NewChannelCodeHandler(store ChannelCodeStore, cache LoginCache, resolver UserIDResolver) *ChannelCodeHandler {
	return &ChannelCodeHandler{store: store, cache: cache, resolver: resolver}
}

func NewChannelCodeHandlerWithAuthorizer(store ChannelCodeStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer, apiBaseURL string, wecom ...ChannelCodeWeComClient) *ChannelCodeHandler {
	handler := NewChannelCodeHandler(store, cache, resolver)
	handler.authorizer = authorizer
	handler.apiBaseURL = strings.TrimRight(apiBaseURL, "/")
	if len(wecom) > 0 {
		handler.wecom = wecom[0]
	}
	return handler
}

func (h *ChannelCodeHandler) GroupIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}

	groups, err := h.store.ChannelCodeGroupsByCorpIDs(r.Context(), principalScope.CorpIDs)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if len(groups) == 0 {
		writeEnvelope(w, http.StatusOK, 200, "success", []map[string]any{})
		return
	}

	list := make([]map[string]any, 0, len(groups)+1)
	for _, group := range groups {
		list = append(list, map[string]any{
			"groupId": group.ID,
			"name":    group.Name,
		})
	}
	list = append(list, map[string]any{
		"groupId": 0,
		"name":    "未分组",
	})
	writeEnvelope(w, http.StatusOK, 200, "success", list)
}

func (h *ChannelCodeHandler) GroupDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, _, _, ok := h.resolveAccess(w, r); !ok {
		return
	}

	rawGroupID := strings.TrimSpace(r.URL.Query().Get("groupId"))
	if rawGroupID == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "分组id必传", nil)
		return
	}
	groupID, err := strconv.Atoi(rawGroupID)
	if err != nil {
		groupID = 0
	}

	group, found, err := h.store.ChannelCodeGroupByID(r.Context(), groupID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusOK, 200, "success", []map[string]any{})
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"id":      group.ID,
		"name":    group.Name,
		"groupId": group.ID,
	})
}

func (h *ChannelCodeHandler) GroupStore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if len(principalScope.CorpIDs) != 1 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "请先选择企业", nil)
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	names := stringListParam(params, "name")
	if len(names) == 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "分组名称必传", nil)
		return
	}
	exists, err := h.store.ChannelCodeGroupNamesExist(r.Context(), names)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if exists {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "已存在相同分组名", nil)
		return
	}
	if err := h.store.CreateChannelCodeGroups(r.Context(), principalScope.CorpIDs[0], names); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "渠道码分组创建失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *ChannelCodeHandler) GroupUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, _, _, ok := h.resolveAccess(w, r); !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	groupID, ok, err := intParam(params, "groupId")
	if !ok {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "分组id必传", nil)
		return
	}
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "分组id必须为整型", nil)
		return
	}
	name := stringParam(params, "name")
	if name == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "分组名称必传", nil)
		return
	}
	if groupID == 0 {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "【未分组】不能修改", nil)
		return
	}
	exists, err := h.store.ChannelCodeGroupNamesExist(r.Context(), []string{name})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if exists {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "已存在相同分组名", nil)
		return
	}
	if err := h.store.UpdateChannelCodeGroupName(r.Context(), groupID, name); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "分组编辑失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *ChannelCodeHandler) GroupMove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, _, _, ok := h.resolveAccess(w, r); !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	groupID, ok, err := intParam(params, "groupId")
	if !ok {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "分组id必传", nil)
		return
	}
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "分组id必须为整型", nil)
		return
	}
	channelCodeID, _, err := intParam(params, "channelCodeId")
	if err != nil {
		channelCodeID = 0
	}
	if err := h.store.MoveChannelCodeToGroup(r.Context(), channelCodeID, groupID); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "移动分组失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *ChannelCodeHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (int, User, DashboardRequestScope, bool) {
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

func stringListParam(params map[string]any, key string) []string {
	value, ok := params[key]
	if !ok || value == nil {
		return nil
	}
	values := make([]string, 0)
	switch typed := value.(type) {
	case []string:
		for _, item := range typed {
			if trimmed := strings.TrimSpace(item); trimmed != "" {
				values = append(values, trimmed)
			}
		}
	case []any:
		for _, item := range typed {
			if trimmed := strings.TrimSpace(fmt.Sprint(item)); trimmed != "" {
				values = append(values, trimmed)
			}
		}
	case string:
		raw := strings.TrimSpace(typed)
		if raw == "" {
			return nil
		}
		var parsed []string
		if strings.HasPrefix(raw, "[") && json.Unmarshal([]byte(raw), &parsed) == nil {
			for _, item := range parsed {
				if trimmed := strings.TrimSpace(item); trimmed != "" {
					values = append(values, trimmed)
				}
			}
			return values
		}
		values = append(values, raw)
	default:
		if trimmed := strings.TrimSpace(fmt.Sprint(value)); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}
