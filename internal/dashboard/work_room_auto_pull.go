package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type WorkRoomAutoPullFilter struct {
	CorpIDs             []int
	GroupID             int
	QRCodeName          string
	LifecycleState      string
	RestrictBusinessIDs bool
	BusinessIDs         []int
	Page                int
	PerPage             int
}

type WorkRoomAutoPullPage struct {
	Items     []WorkRoomAutoPullItem
	Total     int
	TotalPage int
	PerPage   int
}

type WorkRoomAutoPullItem struct {
	WorkRoomAutoPullID  int
	GroupID             int
	MediumID            int
	QRCodeName          string
	QRCodeURL           string
	LeadingWords        string
	Tags                []string
	Employees           []string
	Rooms               []WorkRoomAutoPullListRoom
	ContactNum          int
	CreatedAt           string
	LifecycleState      string
	DataSource          string
	StatisticsAvailable bool
	ProviderKind        string
	AutoCreateRoom      bool
}

type WorkRoomAutoPullListRoom struct {
	RoomName  string `json:"roomName"`
	StateText string `json:"stateText"`
}

type WorkRoomAutoPullShow struct {
	WorkRoomAutoPullID int
	MediumID           int
	QRCodeName         string
	QRCodeURL          string
	IsVerified         int
	RoomNum            int
	LeadingWords       string
	CreatedAt          string
	Employees          []WorkRoomAutoPullEmployee
	Tags               []ChannelCodeTagGroup
	SelectedTags       []int
	Rooms              []WorkRoomAutoPullShowRoom
	ProviderKind       string
	AutoCreateRoom     bool
	RoomBaseName       string
	RoomBaseID         int
}

type WorkRoomAutoPullEmployee struct {
	ID         int    `json:"id"`
	EmployeeID int    `json:"employeeId"`
	Name       string `json:"name"`
	Avatar     string `json:"avatar"`
	WXUserID   string `json:"wxUserId"`
	Select     bool   `json:"select"`
}

type WorkRoomAutoPullShowRoom struct {
	RoomID            int    `json:"roomId"`
	RoomName          string `json:"roomName"`
	RoomMax           int    `json:"roomMax"`
	Num               int    `json:"num"`
	MaxNum            int    `json:"maxNum"`
	RoomQRCodeURL     string `json:"roomQrcodeUrl"`
	LongRoomQRCodeURL string `json:"longRoomQrcodeUrl"`
	State             int    `json:"state"`
}

type WorkRoomAutoPullWrite struct {
	CorpID         int
	MediumID       int
	QRCodeName     string
	IsVerified     int
	LeadingWords   string
	Employees      string
	Tags           string
	Rooms          string
	EmployeeIDs    []int
	RoomIDs        []int
	ProviderKind   string
	AutoCreateRoom bool
	RoomBaseName   string
	RoomBaseID     int
	QRCodeURL      string
	WXConfigID     string
}

type WorkRoomAutoPullUpdateTarget struct {
	CorpID       int
	WXConfigID   string
	ProviderKind string
}

type WorkRoomAutoPullQRCode struct {
	ConfigID  string
	QRCodeURL string
}

type WorkRoomAutoPullStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	RoomWelcomeCorpCredentialByID(ctx context.Context, corpID int) (RoomWelcomeCorpCredential, bool, error)
	WorkRoomAutoPullPage(ctx context.Context, filter WorkRoomAutoPullFilter) (WorkRoomAutoPullPage, error)
	WorkRoomAutoPullBusinessIDsByOperators(ctx context.Context, operationIDs []int) ([]int, error)
	WorkRoomAutoPullShowByID(ctx context.Context, id int) (WorkRoomAutoPullShow, bool, error)
	WorkRoomAutoPullEmployeeWXUserIDs(ctx context.Context, employeeIDs []int) ([]string, error)
	WorkRoomAutoPullRoomWXChatIDs(ctx context.Context, corpID int, roomIDs []int) ([]string, error)
	CreateWorkRoomAutoPullWithLog(ctx context.Context, values WorkRoomAutoPullWrite, operationID int) (int, error)
	WorkRoomAutoPullUpdateTargetByID(ctx context.Context, id int) (WorkRoomAutoPullUpdateTarget, bool, error)
	UpdateWorkRoomAutoPullWithLog(ctx context.Context, id int, values WorkRoomAutoPullWrite, operationID int) (WorkRoomAutoPullUpdateTarget, bool, error)
	UpdateWorkRoomAutoPullQRCode(ctx context.Context, id int, qrcodeURL string, configID string) error
	DeleteWorkRoomAutoPull(ctx context.Context, id int) error
}

type WorkRoomAutoPullContactWayClient interface {
	CreateContactWay(ctx context.Context, credential RoomWelcomeCorpCredential, userIDs []string, skipVerify bool, state string) (WorkRoomAutoPullQRCode, error)
	UpdateContactWay(ctx context.Context, credential RoomWelcomeCorpCredential, configID string, userIDs []string, skipVerify bool, state string) error
}

type WorkRoomAutoPullHandler struct {
	store            WorkRoomAutoPullStore
	cache            LoginCache
	resolver         UserIDResolver
	authorizer       CorpAdminAuthorizer
	contactWayClient WorkRoomAutoPullContactWayClient
	joinWayClient    WorkRoomAutoPullJoinWayClient
	apiBaseURL       string
}

func NewWorkRoomAutoPullHandler(store WorkRoomAutoPullStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer, apiBaseURL string) *WorkRoomAutoPullHandler {
	return NewWorkRoomAutoPullHandlerWithJoinWayClient(store, cache, resolver, authorizer, apiBaseURL, NewWorkRoomAutoPullJoinWayWeComClient(defaultWeComAPIBaseURL))
}

func NewWorkRoomAutoPullHandlerWithContactWayClient(store WorkRoomAutoPullStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer, apiBaseURL string, contactWayClient WorkRoomAutoPullContactWayClient) *WorkRoomAutoPullHandler {
	return &WorkRoomAutoPullHandler{
		store:            store,
		cache:            cache,
		resolver:         resolver,
		authorizer:       authorizer,
		contactWayClient: contactWayClient,
		apiBaseURL:       strings.TrimRight(apiBaseURL, "/"),
	}
}

func NewWorkRoomAutoPullHandlerWithJoinWayClient(store WorkRoomAutoPullStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer, apiBaseURL string, joinWayClient WorkRoomAutoPullJoinWayClient) *WorkRoomAutoPullHandler {
	return &WorkRoomAutoPullHandler{store: store, cache: cache, resolver: resolver, authorizer: authorizer, joinWayClient: joinWayClient, contactWayClient: NewWorkRoomAutoPullWeComClient(defaultWeComAPIBaseURL), apiBaseURL: strings.TrimRight(apiBaseURL, "/")}
}

func (h *WorkRoomAutoPullHandler) Index(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	access, err := h.authorizeAccess(r.Context(), r, userID, principalScope)
	if err != nil {
		writeAccessError(w, err)
		return
	}
	filter := WorkRoomAutoPullFilter{
		CorpIDs:        append([]int{}, principalScope.CorpIDs...),
		GroupID:        positiveQueryInt(r, "groupId", 0),
		QRCodeName:     strings.TrimSpace(r.URL.Query().Get("qrcodeName")),
		LifecycleState: strings.TrimSpace(r.URL.Query().Get("state")),
		Page:           positiveQueryInt(r, "page", 1),
		PerPage:        positiveQueryInt(r, "perPage", 10),
	}
	if dashboardAccess, hasDashboardAccess := DashboardAccessFromContext(r.Context()); hasDashboardAccess && dashboardAccess.ScopeRequired && dashboardAccess.Scope != DataScopeTenant {
		ids, err := h.store.WorkRoomAutoPullBusinessIDsByOperators(r.Context(), dashboardAccess.AllowedEmployeeIDs)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		filter.RestrictBusinessIDs = true
		filter.BusinessIDs = ids
	} else if access.DataPermission != DataPermissionAll {
		ids, err := h.store.WorkRoomAutoPullBusinessIDsByOperators(r.Context(), access.DeptEmployeeIDs)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		filter.RestrictBusinessIDs = true
		filter.BusinessIDs = ids
	}
	page, err := h.store.WorkRoomAutoPullPage(r.Context(), filter)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, map[string]any{
			"workRoomAutoPullId":  item.WorkRoomAutoPullID,
			"id":                  item.WorkRoomAutoPullID,
			"groupId":             item.GroupID,
			"mediumId":            item.MediumID,
			"qrcodeName":          item.QRCodeName,
			"qrcodeUrl":           h.fileFullURL(item.QRCodeURL),
			"leadingWords":        item.LeadingWords,
			"tags":                item.Tags,
			"employees":           item.Employees,
			"rooms":               item.Rooms,
			"contactNum":          item.ContactNum,
			"createdAt":           item.CreatedAt,
			"state":               item.LifecycleState,
			"dataSource":          item.DataSource,
			"statisticsAvailable": item.StatisticsAvailable,
			"providerKind":        item.ProviderKind,
			"autoCreateRoom":      item.AutoCreateRoom,
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

func (h *WorkRoomAutoPullHandler) Show(w http.ResponseWriter, r *http.Request) {
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
	rawID := strings.TrimSpace(r.URL.Query().Get("workRoomAutoPullId"))
	if rawID == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "自动拉群ID 必填", nil)
		return
	}
	id, err := strconv.Atoi(rawID)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "自动拉群ID 必需为整数", nil)
		return
	}
	if id < 1 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "自动拉群ID 不可小于1", nil)
		return
	}
	info, found, err := h.store.WorkRoomAutoPullShowByID(r.Context(), id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "该自动拉群不存在", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"workRoomAutoPullId": info.WorkRoomAutoPullID,
		"mediumId":           info.MediumID,
		"qrcodeName":         info.QRCodeName,
		"qrcodeUrl":          h.fileFullURL(info.QRCodeURL),
		"isVerified":         info.IsVerified,
		"roomNum":            info.RoomNum,
		"leadingWords":       info.LeadingWords,
		"createdAt":          info.CreatedAt,
		"employees":          info.Employees,
		"tags":               info.Tags,
		"selectedTags":       info.SelectedTags,
		"rooms":              h.fullRoomURLs(info.Rooms),
		"providerKind":       info.ProviderKind,
		"autoCreateRoom":     info.AutoCreateRoom,
		"roomBaseName":       info.RoomBaseName,
		"roomBaseId":         info.RoomBaseID,
	})
}

func (h *WorkRoomAutoPullHandler) Store(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, user, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	access, err := h.authorizeAccess(r.Context(), r, userID, principalScope)
	if err != nil {
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
	if h.joinWayClient != nil {
		h.storeDirectJoinWay(w, r, params, user, access, corpID)
		return
	}
	values, ok := parseWorkRoomAutoPullStoreParams(w, params)
	if !ok {
		return
	}
	if dashboardAccess, scoped := DashboardAccessFromContext(r.Context()); scoped && dashboardAccess.ScopeRequired && dashboardAccess.Scope != DataScopeTenant && !workRoomAutoPullEmployeesAllowed(values.EmployeeIDs, dashboardAccess.AllowedEmployeeIDs) {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, "employee scope denied", nil)
		return
	}
	values.CorpID = corpID
	if values.MediumID > 0 {
		validator, configured := h.store.(mediumAvailabilityValidator)
		if !configured {
			writeEnvelope(w, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "素材 Provider 未配置", nil)
			return
		}
		available, err := validator.MediumAvailableToUser(r.Context(), corpID, userID, values.MediumID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		if !available {
			writeEnvelope(w, http.StatusConflict, http.StatusConflict, "素材不可用于当前企业或权限范围", nil)
			return
		}
	}
	if !enforceSaaSQuota(r.Context(), w, h.store, user.TenantID, SaaSMetricWorkRoomAutoPulls, 1) {
		return
	}
	wxUserIDs, ok := h.resolveWorkRoomAutoPullEmployees(w, r.Context(), values.EmployeeIDs)
	if !ok {
		return
	}
	credential, ok := h.resolveCorpCredential(w, r.Context(), corpID)
	if !ok {
		return
	}
	id, err := h.store.CreateWorkRoomAutoPullWithLog(r.Context(), values, access.WorkEmployeeID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "自动拉群创建失败", nil)
		return
	}
	qrcode, err := h.contactWayClient.CreateContactWay(r.Context(), credential, wxUserIDs, values.IsVerified != 1, "workRoomAutoPullId-"+strconv.Itoa(id))
	if err != nil {
		_ = h.store.DeleteWorkRoomAutoPull(r.Context(), id)
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, fmt.Sprintf("请求微信服务器创建二维码失败，错误信息：%s", err.Error()), nil)
		return
	}
	if err := h.store.UpdateWorkRoomAutoPullQRCode(r.Context(), id, qrcode.QRCodeURL, qrcode.ConfigID); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if err := refreshSaaSUsageCounter(r.Context(), h.store, user.TenantID, SaaSMetricWorkRoomAutoPulls); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *WorkRoomAutoPullHandler) storeDirectJoinWay(w http.ResponseWriter, r *http.Request, params map[string]any, user User, access AccessContext, corpID int) {
	values, ok := parseWorkRoomAutoPullDirectJoinParams(w, params)
	if !ok {
		return
	}
	values.CorpID = corpID
	if !enforceSaaSQuota(r.Context(), w, h.store, user.TenantID, SaaSMetricWorkRoomAutoPulls, 1) {
		return
	}
	chatIDs, err := h.store.WorkRoomAutoPullRoomWXChatIDs(r.Context(), corpID, values.RoomIDs)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	credential, ok := h.resolveCorpCredential(w, r.Context(), corpID)
	if !ok {
		return
	}
	payload := WorkRoomAutoPullJoinWayPayload{QRCodeName: values.QRCodeName, ChatIDs: chatIDs, AutoCreateRoom: values.AutoCreateRoom, RoomBaseName: values.RoomBaseName, RoomBaseID: values.RoomBaseID}
	qrcode, err := h.joinWayClient.CreateJoinWay(r.Context(), credential, payload)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, fmt.Sprintf("请求企业微信生成入群二维码失败：%s", err.Error()), nil)
		return
	}
	values.QRCodeURL = qrcode.QRCodeURL
	values.WXConfigID = qrcode.ConfigID
	id, err := h.store.CreateWorkRoomAutoPullWithLog(r.Context(), values, access.WorkEmployeeID)
	if err != nil {
		cleanupCtx, cancel := workRoomAutoPullCleanupContext(r.Context())
		cleanupErr := h.joinWayClient.DeleteJoinWay(cleanupCtx, credential, qrcode.ConfigID)
		cancel()
		if cleanupErr != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, fmt.Sprintf("群活码保存失败，企业微信配置清理失败，请人工检查：%s", cleanupErr.Error()), nil)
			return
		}
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "群活码保存失败，已清理企业微信配置", nil)
		return
	}
	if err := refreshSaaSUsageCounter(r.Context(), h.store, user.TenantID, SaaSMetricWorkRoomAutoPulls); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{"workRoomAutoPullId": id, "qrcodeUrl": qrcode.QRCodeURL})
}

func (h *WorkRoomAutoPullHandler) Update(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	access, err := h.authorizeAccess(r.Context(), r, userID, principalScope)
	if err != nil {
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
	id, values, ok := parseWorkRoomAutoPullUpdateParams(w, params)
	if !ok {
		return
	}
	preflight, found, err := h.store.WorkRoomAutoPullUpdateTargetByID(r.Context(), id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "自动拉群信息读取失败", nil)
		return
	}
	if !found || preflight.CorpID != corpID {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "自动拉群信息不存在", nil)
		return
	}
	if strings.EqualFold(strings.TrimSpace(preflight.ProviderKind), "join_way") {
		writeEnvelope(w, http.StatusConflict, http.StatusConflict, "新版群活码暂不支持编辑，请重新创建", nil)
		return
	}
	if dashboardAccess, scoped := DashboardAccessFromContext(r.Context()); scoped && dashboardAccess.ScopeRequired && dashboardAccess.Scope != DataScopeTenant && !workRoomAutoPullEmployeesAllowed(values.EmployeeIDs, dashboardAccess.AllowedEmployeeIDs) {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, "employee scope denied", nil)
		return
	}
	if values.MediumID > 0 {
		validator, configured := h.store.(mediumAvailabilityValidator)
		if !configured {
			writeEnvelope(w, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "素材 Provider 未配置", nil)
			return
		}
		available, err := validator.MediumAvailableToUser(r.Context(), corpID, userID, values.MediumID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		if !available {
			writeEnvelope(w, http.StatusConflict, http.StatusConflict, "素材不可用于当前企业或权限范围", nil)
			return
		}
	}
	wxUserIDs, ok := h.resolveWorkRoomAutoPullEmployees(w, r.Context(), values.EmployeeIDs)
	if !ok {
		return
	}
	target, found, err := h.store.UpdateWorkRoomAutoPullWithLog(r.Context(), id, values, access.WorkEmployeeID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "自动拉群更新失败", nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "自动拉群信息不存在", nil)
		return
	}
	credential, ok := h.resolveCorpCredential(w, r.Context(), target.CorpID)
	if !ok {
		return
	}
	if err := h.contactWayClient.UpdateContactWay(r.Context(), credential, target.WXConfigID, wxUserIDs, values.IsVerified != 1, "workRoomAutoPullId-"+strconv.Itoa(id)); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, fmt.Sprintf("请求微信服务器更新二维码失败，错误信息：%s", err.Error()), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *WorkRoomAutoPullHandler) Move(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if _, err := h.authorizeAccess(r.Context(), requestWithPermissionPath(r, "/dashboard/workRoomAutoPull/update"), userID, principalScope); err != nil {
		writeAccessError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *WorkRoomAutoPullHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (int, User, DashboardRequestScope, bool) {
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

func (h *WorkRoomAutoPullHandler) authorizeAccess(ctx context.Context, r *http.Request, userID int, principalScope DashboardRequestScope) (AccessContext, error) {
	if h.authorizer == nil {
		return AccessContext{DataPermission: DataPermissionAll}, nil
	}
	corpID := 0
	if len(principalScope.CorpIDs) > 0 {
		corpID = principalScope.CorpIDs[0]
	}
	return h.authorizer.Resolve(ctx, userID, PermissionKeyFromRequest(r), corpID, principalScope.WorkEmployeeID)
}

func (h *WorkRoomAutoPullHandler) fileFullURL(path string) string {
	if path == "" || strings.Contains(path, "http") || h.apiBaseURL == "" {
		return path
	}
	return h.apiBaseURL + "/static/" + strings.TrimLeft(path, "/")
}

func (h *WorkRoomAutoPullHandler) fullRoomURLs(rooms []WorkRoomAutoPullShowRoom) []WorkRoomAutoPullShowRoom {
	out := make([]WorkRoomAutoPullShowRoom, 0, len(rooms))
	for _, room := range rooms {
		if room.RoomQRCodeURL != "" {
			room.LongRoomQRCodeURL = h.fileFullURL(room.RoomQRCodeURL)
		}
		out = append(out, room)
	}
	return out
}

func (h *WorkRoomAutoPullHandler) resolveCorpCredential(w http.ResponseWriter, ctx context.Context, corpID int) (RoomWelcomeCorpCredential, bool) {
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

func (h *WorkRoomAutoPullHandler) resolveWorkRoomAutoPullEmployees(w http.ResponseWriter, ctx context.Context, employeeIDs []int) ([]string, bool) {
	wxUserIDs, err := h.store.WorkRoomAutoPullEmployeeWXUserIDs(ctx, employeeIDs)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "使用者信息错误", nil)
		return nil, false
	}
	return wxUserIDs, true
}

func parseWorkRoomAutoPullStoreParams(w http.ResponseWriter, params map[string]any) (WorkRoomAutoPullWrite, bool) {
	corpID, ok, err := intParam(params, "corpId")
	if !ok {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "公司授信ID 必填", nil)
		return WorkRoomAutoPullWrite{}, false
	}
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "公司授信ID 必需为整数", nil)
		return WorkRoomAutoPullWrite{}, false
	}
	if corpID < 1 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "公司授信ID 不可小于1", nil)
		return WorkRoomAutoPullWrite{}, false
	}
	qrcodeName := stringParam(params, "qrcodeName")
	if qrcodeName == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "扫码名称 必填", nil)
		return WorkRoomAutoPullWrite{}, false
	}
	if len([]rune(qrcodeName)) > 30 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "扫码名称  字符串最大长度30", nil)
		return WorkRoomAutoPullWrite{}, false
	}
	leadingWords := stringParam(params, "leadingWords")
	if leadingWords == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "入群引导语 必填", nil)
		return WorkRoomAutoPullWrite{}, false
	}
	if len([]rune(leadingWords)) > 1000 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "扫码名称  字符串最大长度1000", nil)
		return WorkRoomAutoPullWrite{}, false
	}
	values, ok := parseWorkRoomAutoPullCommonParams(w, params)
	if !ok {
		return WorkRoomAutoPullWrite{}, false
	}
	values.CorpID = corpID
	values.QRCodeName = qrcodeName
	values.LeadingWords = leadingWords
	return values, true
}

func parseWorkRoomAutoPullDirectJoinParams(w http.ResponseWriter, params map[string]any) (WorkRoomAutoPullWrite, bool) {
	qrcodeName := stringParam(params, "qrcodeName")
	if qrcodeName == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "群活码名称必填", nil)
		return WorkRoomAutoPullWrite{}, false
	}
	if len([]rune(qrcodeName)) > 30 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "群活码名称最多30个字符", nil)
		return WorkRoomAutoPullWrite{}, false
	}
	roomIDs, err := intSliceParam(params, "rooms")
	roomIDs = uniquePositiveInts(roomIDs)
	if err != nil || len(roomIDs) == 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "请选择至少一个可加入群聊", nil)
		return WorkRoomAutoPullWrite{}, false
	}
	if len(roomIDs) > 5 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "可加入群聊最多选择5个", nil)
		return WorkRoomAutoPullWrite{}, false
	}
	autoCreateRoom, found, err := boolParam(params, "autoCreateRoom")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "自动建群开关格式错误", nil)
		return WorkRoomAutoPullWrite{}, false
	}
	if !found {
		autoCreateRoom = false
	}
	roomBaseName := stringParam(params, "roomBaseName")
	roomBaseID := 1
	if value, present, valueErr := intParam(params, "roomBaseId"); present {
		if valueErr != nil {
			roomBaseID = 0
		} else {
			roomBaseID = value
		}
	}
	if autoCreateRoom {
		if roomBaseName == "" || len([]rune(roomBaseName)) > 30 {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "新群名称必填且最多30个字符", nil)
			return WorkRoomAutoPullWrite{}, false
		}
		if roomBaseID < 1 {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "起始序号必须为大于0的整数", nil)
			return WorkRoomAutoPullWrite{}, false
		}
	} else {
		roomBaseName = ""
		roomBaseID = 1
	}
	roomValues := make([]map[string]int, 0, len(roomIDs))
	for _, id := range roomIDs {
		roomValues = append(roomValues, map[string]int{"roomId": id, "maxNum": 0})
	}
	roomsRaw, _ := json.Marshal(roomValues)
	return WorkRoomAutoPullWrite{
		QRCodeName: qrcodeName, IsVerified: 2, LeadingWords: "", Employees: "[]", Tags: "[]", Rooms: string(roomsRaw), RoomIDs: roomIDs,
		ProviderKind: "join_way", AutoCreateRoom: autoCreateRoom, RoomBaseName: roomBaseName, RoomBaseID: roomBaseID,
	}, true
}

func parseWorkRoomAutoPullUpdateParams(w http.ResponseWriter, params map[string]any) (int, WorkRoomAutoPullWrite, bool) {
	id, ok, err := intParam(params, "workRoomAutoPullId")
	if !ok {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "自动拉群ID 必填", nil)
		return 0, WorkRoomAutoPullWrite{}, false
	}
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "自动拉群ID 必需为整数", nil)
		return 0, WorkRoomAutoPullWrite{}, false
	}
	if id < 1 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "自动拉群ID 不可小于1", nil)
		return 0, WorkRoomAutoPullWrite{}, false
	}
	values, ok := parseWorkRoomAutoPullCommonParams(w, params)
	return id, values, ok
}

func parseWorkRoomAutoPullCommonParams(w http.ResponseWriter, params map[string]any) (WorkRoomAutoPullWrite, bool) {
	mediumID, mediumPresent, mediumErr := intParam(params, "mediumId")
	if mediumErr != nil || (mediumPresent && mediumID < 0) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "素材ID 必须为非负整数", nil)
		return WorkRoomAutoPullWrite{}, false
	}
	isVerified, ok, err := intParam(params, "isVerified")
	if !ok {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "添加验证 必填", nil)
		return WorkRoomAutoPullWrite{}, false
	}
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "添加验证 必需为整数", nil)
		return WorkRoomAutoPullWrite{}, false
	}
	if isVerified != 1 && isVerified != 2 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "添加验证 值必须在列表内：[1,2]", nil)
		return WorkRoomAutoPullWrite{}, false
	}
	employeeIDs, employeesJSON, ok := workRoomAutoPullIDsJSON(w, params, "employees", "使用成员")
	if !ok {
		return WorkRoomAutoPullWrite{}, false
	}
	_, tagsJSON, ok := workRoomAutoPullIDsJSON(w, params, "tags", "客户标签")
	if !ok {
		return WorkRoomAutoPullWrite{}, false
	}
	roomsJSON, ok := workRoomAutoPullRoomsJSON(w, params)
	if !ok {
		return WorkRoomAutoPullWrite{}, false
	}
	return WorkRoomAutoPullWrite{
		MediumID:    mediumID,
		IsVerified:  isVerified,
		Employees:   employeesJSON,
		Tags:        tagsJSON,
		Rooms:       roomsJSON,
		EmployeeIDs: employeeIDs,
	}, true
}

func workRoomAutoPullEmployeesAllowed(ids, allowed []int) bool {
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

func workRoomAutoPullIDsJSON(w http.ResponseWriter, params map[string]any, key string, label string) ([]int, string, bool) {
	ids, err := intSliceParam(params, key)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, label+" 必需是字符串类型", nil)
		return nil, "", false
	}
	if len(ids) == 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, label+" 必填", nil)
		return nil, "", false
	}
	encodedIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		encodedIDs = append(encodedIDs, strconv.Itoa(id))
	}
	raw, _ := json.Marshal(encodedIDs)
	return ids, string(raw), true
}

func workRoomAutoPullRoomsJSON(w http.ResponseWriter, params map[string]any) (string, bool) {
	value, exists := params["rooms"]
	if !exists || value == nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "客户群聊 必填", nil)
		return "", false
	}
	raw := stringParam(params, "rooms")
	if raw == "" {
		if encoded, err := json.Marshal(value); err == nil {
			raw = string(encoded)
		}
	}
	if strings.TrimSpace(raw) == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "客户群聊 必填", nil)
		return "", false
	}
	var decoded any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "客户群聊 必需是JSON类型", nil)
		return "", false
	}
	return raw, true
}
