package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type RadarFilter struct {
	CorpID  int
	Type    int
	Title   string
	Page    int
	PerPage int
}

type RadarItem struct {
	ID              int
	Type            int
	Title           string
	Link            string
	LinkTitle       string
	LinkDescription string
	LinkCover       string
	PDFName         string
	PDF             string
	ArticleType     int
	ArticleRaw      string
	EmployeeCard    int
	ActionNotice    int
	DynamicNotice   int
	ContactTagsRaw  string
	TagStatus       int
	ContactGradeRaw string
	TenantID        int
	CorpID          int
	CreateUserID    int
	CreateUserName  string
	ClickNum        int
	ClickPersonNum  int
	ChannelNum      int
	CreatedAt       string
	UpdatedAt       string
}

type RadarPage struct {
	Items     []RadarItem
	Total     int
	TotalPage int
	Page      int
	PerPage   int
}

type RadarWrite struct {
	CorpID             int
	CreateUserID       int
	Type               int
	HasType            bool
	Title              string
	HasTitle           bool
	Link               string
	HasLink            bool
	LinkTitle          string
	HasLinkTitle       bool
	LinkDescription    string
	HasLinkDescription bool
	LinkCover          string
	HasLinkCover       bool
	PDFName            string
	HasPDFName         bool
	PDF                string
	HasPDF             bool
	ArticleType        int
	HasArticleType     bool
	ArticleRaw         string
	HasArticle         bool
	EmployeeCard       int
	HasEmployeeCard    bool
	ActionNotice       int
	HasActionNotice    bool
	DynamicNotice      int
	HasDynamicNotice   bool
	ContactTagsRaw     string
	HasContactTags     bool
	TagStatus          int
	HasTagStatus       bool
	ContactGradeRaw    string
	HasContactGrade    bool
}

type RadarChannelItem struct {
	ID             int
	Name           string
	TenantID       int
	CorpID         int
	CreateUserID   int
	CreateUserName string
	CreatedAt      string
	UpdatedAt      string
}

type RadarChannelPage struct {
	Items     []RadarChannelItem
	Total     int
	TotalPage int
	Page      int
	PerPage   int
}

type RadarChannelLinkWrite struct {
	RadarID      int
	ChannelID    int
	EmployeeID   int
	Type         int
	CorpID       int
	CreateUserID int
}

type RadarChannelLinkItem struct {
	ID             int
	RadarID        int
	RadarTitle     string
	ChannelID      int
	ChannelName    string
	Link           string
	EmployeeID     int
	EmployeeName   string
	ClickNum       int
	ClickPersonNum int
	TenantID       int
	CorpID         int
	CreateUserID   int
	CreateUserName string
	CreatedAt      string
	UpdatedAt      string
}

type RadarChannelLinkPage struct {
	Items     []RadarChannelLinkItem
	Total     int
	TotalPage int
	Page      int
	PerPage   int
}

type RadarRecordItem struct {
	ID           int
	RadarID      int
	RadarTitle   string
	ChannelID    int
	ChannelName  string
	Type         int
	UnionID      string
	Nickname     string
	Avatar       string
	ContactID    int
	EmployeeID   int
	EmployeeName string
	Content      string
	CorpID       int
	ClickNum     int
	ClickInfo    []RadarClickInfo
	CreatedAt    string
	UpdatedAt    string
}

type RadarClickInfo struct {
	CreatedAt string
	Content   string
}

type RadarRecordPage struct {
	Items     []RadarRecordItem
	Total     int
	TotalPage int
	Page      int
	PerPage   int
}

type RadarOverview struct {
	RadarID        int
	ClickNum       int
	ClickPersonNum int
	ChannelNum     int
}

type RadarStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	RadarPage(ctx context.Context, filter RadarFilter) (RadarPage, error)
	RadarByID(ctx context.Context, corpID int, id int) (RadarItem, bool, error)
	CreateRadar(ctx context.Context, values RadarWrite) (int, error)
	UpdateRadar(ctx context.Context, corpID int, id int, values RadarWrite) (bool, error)
	DeleteRadar(ctx context.Context, corpID int, id int) (bool, error)
	CreateRadarChannel(ctx context.Context, corpID int, userID int, name string) (int, error)
	RadarChannelPage(ctx context.Context, corpID int, name string, page int, perPage int) (RadarChannelPage, error)
	UpsertRadarChannelLink(ctx context.Context, values RadarChannelLinkWrite) (RadarChannelLinkItem, error)
	UpdateRadarChannelLinkURL(ctx context.Context, corpID int, id int, link string) (bool, error)
	RadarChannelLinkPage(ctx context.Context, corpID int, radarID int, page int, perPage int) (RadarChannelLinkPage, error)
	RadarOverview(ctx context.Context, corpID int, radarID int, codeType int) (RadarOverview, error)
	RadarRecordPage(ctx context.Context, corpID int, radarID int, channelID int, page int, perPage int) (RadarRecordPage, error)
	RadarChannelStats(ctx context.Context, corpID int, radarID int, page int, perPage int) (RadarChannelLinkPage, error)
}

type RadarHandler struct {
	store            RadarStore
	cache            LoginCache
	resolver         UserIDResolver
	authorizer       CorpAdminAuthorizer
	operationBaseURL string
}

func NewRadarHandler(store RadarStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer, operationBaseURL string) *RadarHandler {
	return &RadarHandler{store: store, cache: cache, resolver: resolver, authorizer: authorizer, operationBaseURL: strings.TrimRight(strings.TrimSpace(operationBaseURL), "/")}
}

func (h *RadarHandler) Index(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/radar/index#get")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	page, err := h.store.RadarPage(r.Context(), RadarFilter{
		CorpID:  corpID,
		Type:    positiveQueryInt(r, "type", 0),
		Title:   radarQueryTitle(r),
		Page:    positiveQueryInt(r, "page", 1),
		PerPage: positiveQueryInt(r, "perPage", 15),
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, radarPayload(item))
	}
	payload := contactBatchAddPagination(r, page.Page, page.PerPage, page.Total, list)
	payload["list"] = list
	payload["page"] = map[string]any{"perPage": page.PerPage, "total": page.Total, "totalPage": page.TotalPage}
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *RadarHandler) Store(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, user, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/radar/store#post")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "参数错误", nil)
		return
	}
	values, err := radarWriteFromParams(params, corpID, userID, true)
	if err != nil {
		if isBadRequestError(err) {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
			return
		}
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !enforceSaaSQuota(r.Context(), w, h.store, user.TenantID, SaaSMetricRadars, 1) {
		return
	}
	id, err := h.store.CreateRadar(r.Context(), values)
	if err != nil {
		if writeSaaSQuotaError(w, err) {
			return
		}
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if err := refreshSaaSUsageCounter(r.Context(), h.store, user.TenantID, SaaSMetricRadars); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{id})
}

func (h *RadarHandler) Update(w http.ResponseWriter, r *http.Request) {
	h.writeMutation(w, r, http.MethodPut, "/dashboard/radar/update#put", func(ctx context.Context, userID int, corpID int, params map[string]any) (any, error) {
		id, err := radarIDFromParams(params)
		if err != nil {
			return nil, err
		}
		values, err := radarWriteFromParams(params, corpID, userID, false)
		if err != nil {
			return nil, err
		}
		ok, err := h.store.UpdateRadar(ctx, corpID, id, values)
		return nil, sopRequireFound(ok, err, "互动雷达不存在")
	})
}

func (h *RadarHandler) Destroy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, user, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/radar/destroy#delete")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "参数错误", nil)
		return
	}
	id, err := radarIDFromParams(params)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	deleted, err := h.store.DeleteRadar(r.Context(), corpID, id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !deleted {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "互动雷达不存在", nil)
		return
	}
	if err := refreshSaaSUsageCounter(r.Context(), h.store, user.TenantID, SaaSMetricRadars); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *RadarHandler) Info(w http.ResponseWriter, r *http.Request) {
	h.showRadar(w, r, "/dashboard/radar/info#get")
}

func (h *RadarHandler) Show(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/radar/show#get")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	radarID := radarIDFromQuery(r)
	if radarID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "radarId required", nil)
		return
	}
	item, found, err := h.store.RadarByID(r.Context(), corpID, radarID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "互动雷达不存在", nil)
		return
	}
	overview, err := h.store.RadarOverview(r.Context(), corpID, radarID, positiveQueryInt(r, "type", 0))
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	payload := radarPayload(item)
	payload["clickNum"] = overview.ClickNum
	payload["click_num"] = overview.ClickNum
	payload["clickPersonNum"] = overview.ClickPersonNum
	payload["click_person_num"] = overview.ClickPersonNum
	payload["channelNum"] = overview.ChannelNum
	payload["channel_num"] = overview.ChannelNum
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *RadarHandler) showRadar(w http.ResponseWriter, r *http.Request, permissionKey string) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, _, ok := h.resolveAuthorized(w, r, permissionKey)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	id := radarIDFromQuery(r)
	if id <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "radarId required", nil)
		return
	}
	item, found, err := h.store.RadarByID(r.Context(), corpID, id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "互动雷达不存在", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", radarPayload(item))
}

func (h *RadarHandler) StoreChannel(w http.ResponseWriter, r *http.Request) {
	h.writeMutation(w, r, http.MethodPost, "/dashboard/radar/storeChannel#post", func(ctx context.Context, userID int, corpID int, params map[string]any) (any, error) {
		name := stringParam(params, "name")
		if strings.TrimSpace(name) == "" {
			return nil, badRequestError("name required")
		}
		id, err := h.store.CreateRadarChannel(ctx, corpID, userID, name)
		if err != nil {
			return nil, err
		}
		return []any{id}, nil
	})
}

func (h *RadarHandler) IndexChannel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/radar/indexChannel#get")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	page, err := h.store.RadarChannelPage(r.Context(), corpID, radarQueryTitle(r), positiveQueryInt(r, "page", 1), positiveQueryInt(r, "perPage", 100))
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, radarChannelPayload(item))
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"page": map[string]any{"perPage": page.PerPage, "total": page.Total, "totalPage": page.TotalPage},
		"list": list,
	})
}

func (h *RadarHandler) StoreChannelLink(w http.ResponseWriter, r *http.Request) {
	h.writeMutation(w, r, http.MethodPost, "/dashboard/radar/storeChannelLink#post", func(ctx context.Context, userID int, corpID int, params map[string]any) (any, error) {
		radarID, _, err := firstPositiveIntParam(params, "radar_id", "radarId", "id")
		if err != nil || radarID <= 0 {
			return nil, badRequestError("radarId required")
		}
		channelID, _, err := firstPositiveIntParam(params, "channel_id", "channelId")
		if err != nil || channelID <= 0 {
			return nil, badRequestError("channelId required")
		}
		employeeID, _, err := firstPositiveIntParam(params, "employeeId", "employee_id")
		if err != nil || employeeID <= 0 {
			return nil, badRequestError("employeeId required")
		}
		codeType, _, _ := intParam(params, "type")
		item, err := h.store.UpsertRadarChannelLink(ctx, RadarChannelLinkWrite{
			RadarID:      radarID,
			ChannelID:    channelID,
			EmployeeID:   employeeID,
			Type:         codeType,
			CorpID:       corpID,
			CreateUserID: userID,
		})
		if err != nil {
			return nil, err
		}
		link := h.radarOperationURL(r, radarID, codeType, employeeID, item.ID)
		if ok, err := h.store.UpdateRadarChannelLinkURL(ctx, corpID, item.ID, link); err != nil {
			return nil, err
		} else if ok {
			item.Link = link
		}
		return radarChannelLinkPayload(item), nil
	})
}

func (h *RadarHandler) IndexChannelLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/radar/indexChannelLink#get")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	page, err := h.store.RadarChannelLinkPage(r.Context(), corpID, radarIDFromQuery(r), positiveQueryInt(r, "page", 1), positiveQueryInt(r, "perPage", 100))
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, radarChannelLinkPayload(item))
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"page": map[string]any{"perPage": page.PerPage, "total": page.Total, "totalPage": page.TotalPage},
		"list": list,
	})
}

func (h *RadarHandler) ShowContact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/radar/showContact#get")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	page, err := h.store.RadarRecordPage(r.Context(), corpID, radarIDFromQuery(r), radarQueryInt(r, 0, "channelId", "channel_id"), positiveQueryInt(r, "page", 1), positiveQueryInt(r, "perPage", 15))
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, radarRecordPayload(item))
	}
	payload := contactBatchAddPagination(r, page.Page, page.PerPage, page.Total, list)
	payload["list"] = list
	payload["page"] = map[string]any{"perPage": page.PerPage, "total": page.Total, "totalPage": page.TotalPage}
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *RadarHandler) ShowChannel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/radar/showChannel#get")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	page, err := h.store.RadarChannelStats(r.Context(), corpID, radarIDFromQuery(r), positiveQueryInt(r, "page", 1), positiveQueryInt(r, "perPage", 15))
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, radarChannelLinkPayload(item))
	}
	payload := contactBatchAddPagination(r, page.Page, page.PerPage, page.Total, list)
	payload["list"] = list
	payload["page"] = map[string]any{"perPage": page.PerPage, "total": page.Total, "totalPage": page.TotalPage}
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *RadarHandler) RadarArticle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/radar/radarArticle#get")
	if !ok {
		return
	}
	link := strings.TrimSpace(r.URL.Query().Get("link"))
	if link == "" {
		link = strings.TrimSpace(r.URL.Query().Get("url"))
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"title":       strings.TrimSpace(r.URL.Query().Get("title")),
		"link":        link,
		"url":         link,
		"description": strings.TrimSpace(r.URL.Query().Get("description")),
		"cover":       strings.TrimSpace(r.URL.Query().Get("cover")),
		"content":     "",
	})
}

func (h *RadarHandler) writeMutation(w http.ResponseWriter, r *http.Request, method string, permissionKey string, action func(context.Context, int, int, map[string]any) (any, error)) {
	if r.Method != method {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, _, _, ok := h.resolveAuthorized(w, r, permissionKey)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
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

func (h *RadarHandler) resolveAuthorized(w http.ResponseWriter, r *http.Request, permissionKey string) (int, User, DashboardRequestScope, AccessContext, bool) {
	userID, user, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return 0, User{}, DashboardRequestScope{}, AccessContext{}, false
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return 0, User{}, DashboardRequestScope{}, AccessContext{}, false
	}
	employeeID := principalScope.WorkEmployeeID
	if employeeID <= 0 {
		resolved, err := h.store.EmployeeIDByUserCorp(r.Context(), userID, corpID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return 0, User{}, DashboardRequestScope{}, AccessContext{}, false
		}
		employeeID = resolved
	}
	access := AccessContext{User: user, PermissionKey: permissionKey, CorpID: corpID, WorkEmployeeID: employeeID, DataPermission: DataPermissionAll}
	if h.authorizer != nil {
		resolved, err := h.authorizer.Resolve(r.Context(), userID, permissionKey, corpID, employeeID)
		if err != nil {
			writeAccessError(w, err)
			return 0, User{}, DashboardRequestScope{}, AccessContext{}, false
		}
		access = resolved
	}
	return userID, user, principalScope, access, true
}

func (h *RadarHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (int, User, DashboardRequestScope, bool) {
	if h.resolver == nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "user resolver not configured", nil)
		return 0, User{}, DashboardRequestScope{}, false
	}
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
	return userID, user, DashboardRequestScope(principalScope), true
}

func (h *RadarHandler) radarOperationURL(r *http.Request, radarID int, codeType int, employeeID int, targetID int) string {
	target := "/radar?id=" + strconv.Itoa(radarID) + "&type=" + strconv.Itoa(codeType) + "&employee_id=" + strconv.Itoa(employeeID) + "&target_id=" + strconv.Itoa(targetID)
	base := h.operationBaseURL
	if base == "" {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		base = scheme + "://" + r.Host
	}
	return base + "/auth/radar?id=" + strconv.Itoa(radarID) + "&target=" + url.QueryEscape(target)
}

func radarWriteFromParams(params map[string]any, corpID int, userID int, requireCreateFields bool) (RadarWrite, error) {
	values := RadarWrite{CorpID: corpID, CreateUserID: userID}
	if codeType, found, err := intParam(params, "type"); err != nil {
		return RadarWrite{}, badRequestError("type invalid")
	} else if found {
		values.Type = codeType
		values.HasType = true
	}
	if title, found := sopStringParam(params, "title", "name"); found {
		values.Title = title
		values.HasTitle = true
	}
	if requireCreateFields {
		if values.Type <= 0 {
			return RadarWrite{}, badRequestError("type required")
		}
		if strings.TrimSpace(values.Title) == "" {
			return RadarWrite{}, badRequestError("title required")
		}
	}
	radarStringField(params, &values.Link, &values.HasLink, "link", "url")
	radarStringField(params, &values.LinkTitle, &values.HasLinkTitle, "linkTitle", "link_title")
	radarStringField(params, &values.LinkDescription, &values.HasLinkDescription, "linkDescription", "link_description", "description")
	radarStringField(params, &values.LinkCover, &values.HasLinkCover, "linkCover", "link_cover", "cover")
	radarStringField(params, &values.PDFName, &values.HasPDFName, "pdfName", "pdf_name")
	radarStringField(params, &values.PDF, &values.HasPDF, "pdf", "pdfUrl", "pdf_url")
	if articleType, found, err := radarFirstIntParam(params, "articleType", "article_type"); err != nil {
		return RadarWrite{}, badRequestError("articleType invalid")
	} else if found {
		values.ArticleType = articleType
		values.HasArticleType = true
	}
	if raw, found, err := sopRawJSONParam(params, "article"); err != nil {
		return RadarWrite{}, err
	} else if found {
		values.ArticleRaw = raw
		values.HasArticle = true
	}
	if employeeCard, found, err := radarFirstIntParam(params, "employeeCard", "employee_card"); err != nil {
		return RadarWrite{}, badRequestError("employeeCard invalid")
	} else if found {
		values.EmployeeCard = employeeCard
		values.HasEmployeeCard = true
	}
	if actionNotice, found, err := radarFirstIntParam(params, "actionNotice", "action_notice"); err != nil {
		return RadarWrite{}, badRequestError("actionNotice invalid")
	} else if found {
		values.ActionNotice = actionNotice
		values.HasActionNotice = true
	}
	if dynamicNotice, found, err := radarFirstIntParam(params, "dynamicNotice", "dynamic_notice"); err != nil {
		return RadarWrite{}, badRequestError("dynamicNotice invalid")
	} else if found {
		values.DynamicNotice = dynamicNotice
		values.HasDynamicNotice = true
	}
	if raw, found, err := sopRawJSONParam(params, "contactTags", "contact_tags", "tags"); err != nil {
		return RadarWrite{}, err
	} else if found {
		values.ContactTagsRaw = raw
		values.HasContactTags = true
	}
	if tagStatus, found, err := radarFirstIntParam(params, "tagStatus", "tag_status"); err != nil {
		return RadarWrite{}, badRequestError("tagStatus invalid")
	} else if found {
		values.TagStatus = tagStatus
		values.HasTagStatus = true
	}
	if raw, found, err := sopRawJSONParam(params, "contactGrade", "contact_grade", "grade"); err != nil {
		return RadarWrite{}, err
	} else if found {
		values.ContactGradeRaw = raw
		values.HasContactGrade = true
	}
	return values, nil
}

func radarStringField(params map[string]any, target *string, present *bool, keys ...string) {
	for _, key := range keys {
		if _, ok := params[key]; !ok {
			continue
		}
		*target = stringParam(params, key)
		*present = true
		return
	}
}

func radarFirstIntParam(params map[string]any, keys ...string) (int, bool, error) {
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

func radarIDFromParams(params map[string]any) (int, error) {
	id, _, err := firstPositiveIntParam(params, "radarId", "radar_id", "id")
	if err != nil || id <= 0 {
		return 0, badRequestError("radarId required")
	}
	return id, nil
}

func radarIDFromQuery(r *http.Request) int {
	for _, key := range []string{"radarId", "radar_id", "id"} {
		if id := positiveQueryInt(r, key, 0); id > 0 {
			return id
		}
	}
	return 0
}

func radarQueryInt(r *http.Request, fallback int, keys ...string) int {
	for _, key := range keys {
		if strings.TrimSpace(r.URL.Query().Get(key)) == "" {
			continue
		}
		return positiveQueryInt(r, key, fallback)
	}
	return fallback
}

func radarQueryTitle(r *http.Request) string {
	for _, key := range []string{"title", "name", "keyword", "keyWords"} {
		if value := strings.TrimSpace(r.URL.Query().Get(key)); value != "" {
			return value
		}
	}
	return ""
}

func radarPayload(item RadarItem) map[string]any {
	return map[string]any{
		"id":               item.ID,
		"radarId":          item.ID,
		"radar_id":         item.ID,
		"type":             item.Type,
		"title":            item.Title,
		"name":             item.Title,
		"link":             item.Link,
		"linkTitle":        item.LinkTitle,
		"link_title":       item.LinkTitle,
		"linkDescription":  item.LinkDescription,
		"link_description": item.LinkDescription,
		"linkCover":        item.LinkCover,
		"link_cover":       item.LinkCover,
		"pdfName":          item.PDFName,
		"pdf_name":         item.PDFName,
		"pdf":              item.PDF,
		"articleType":      item.ArticleType,
		"article_type":     item.ArticleType,
		"article":          sopJSONPayload(item.ArticleRaw),
		"articleRaw":       item.ArticleRaw,
		"employeeCard":     item.EmployeeCard,
		"employee_card":    item.EmployeeCard,
		"actionNotice":     item.ActionNotice,
		"action_notice":    item.ActionNotice,
		"dynamicNotice":    item.DynamicNotice,
		"dynamic_notice":   item.DynamicNotice,
		"contactTags":      sopJSONPayload(item.ContactTagsRaw),
		"contact_tags":     sopJSONPayload(item.ContactTagsRaw),
		"tagStatus":        item.TagStatus,
		"tag_status":       item.TagStatus,
		"contactGrade":     sopJSONPayload(item.ContactGradeRaw),
		"contact_grade":    sopJSONPayload(item.ContactGradeRaw),
		"clickNum":         item.ClickNum,
		"click_num":        item.ClickNum,
		"clickPersonNum":   item.ClickPersonNum,
		"click_person_num": item.ClickPersonNum,
		"channelNum":       item.ChannelNum,
		"channel_num":      item.ChannelNum,
		"tenantId":         item.TenantID,
		"tenant_id":        item.TenantID,
		"corpId":           item.CorpID,
		"corp_id":          item.CorpID,
		"createUserId":     item.CreateUserID,
		"create_user_id":   item.CreateUserID,
		"create_user":      item.CreateUserName,
		"createdAt":        item.CreatedAt,
		"created_at":       item.CreatedAt,
		"updatedAt":        item.UpdatedAt,
		"updated_at":       item.UpdatedAt,
	}
}

func radarChannelPayload(item RadarChannelItem) map[string]any {
	return map[string]any{
		"id":             item.ID,
		"channelId":      item.ID,
		"channel_id":     item.ID,
		"name":           item.Name,
		"tenantId":       item.TenantID,
		"tenant_id":      item.TenantID,
		"corpId":         item.CorpID,
		"corp_id":        item.CorpID,
		"createUserId":   item.CreateUserID,
		"create_user_id": item.CreateUserID,
		"create_user":    item.CreateUserName,
		"createdAt":      item.CreatedAt,
		"created_at":     item.CreatedAt,
		"updatedAt":      item.UpdatedAt,
		"updated_at":     item.UpdatedAt,
	}
}

func radarChannelLinkPayload(item RadarChannelLinkItem) map[string]any {
	return map[string]any{
		"id":               item.ID,
		"linkId":           item.ID,
		"link_id":          item.ID,
		"radarId":          item.RadarID,
		"radar_id":         item.RadarID,
		"radarTitle":       item.RadarTitle,
		"radar_title":      item.RadarTitle,
		"channelId":        item.ChannelID,
		"channel_id":       item.ChannelID,
		"channel":          item.ChannelName,
		"name":             item.ChannelName,
		"link":             item.Link,
		"employeeId":       item.EmployeeID,
		"employee_id":      item.EmployeeID,
		"employee":         item.EmployeeName,
		"create_user":      item.CreateUserName,
		"clickNum":         item.ClickNum,
		"click_num":        item.ClickNum,
		"clickPersonNum":   item.ClickPersonNum,
		"click_person_num": item.ClickPersonNum,
		"tenantId":         item.TenantID,
		"tenant_id":        item.TenantID,
		"corpId":           item.CorpID,
		"corp_id":          item.CorpID,
		"createUserId":     item.CreateUserID,
		"create_user_id":   item.CreateUserID,
		"createdAt":        item.CreatedAt,
		"created_at":       item.CreatedAt,
		"updatedAt":        item.UpdatedAt,
		"updated_at":       item.UpdatedAt,
	}
}

func radarRecordPayload(item RadarRecordItem) map[string]any {
	clickInfo := make([]map[string]any, 0, len(item.ClickInfo))
	for _, info := range item.ClickInfo {
		clickInfo = append(clickInfo, map[string]any{"createdAt": info.CreatedAt, "created_at": info.CreatedAt, "content": info.Content})
	}
	if len(clickInfo) == 0 {
		clickInfo = append(clickInfo, map[string]any{"createdAt": item.CreatedAt, "created_at": item.CreatedAt, "content": item.Content})
	}
	return map[string]any{
		"id":           item.ID,
		"radarId":      item.RadarID,
		"radar_id":     item.RadarID,
		"channelId":    item.ChannelID,
		"channel_id":   item.ChannelID,
		"channel":      item.ChannelName,
		"type":         item.Type,
		"unionId":      item.UnionID,
		"union_id":     item.UnionID,
		"nickname":     item.Nickname,
		"contactName":  firstNonEmpty(item.Nickname, item.UnionID),
		"contact_name": firstNonEmpty(item.Nickname, item.UnionID),
		"avatar":       item.Avatar,
		"contactId":    item.ContactID,
		"contact_id":   item.ContactID,
		"employeeId":   item.EmployeeID,
		"employee_id":  item.EmployeeID,
		"employee":     item.EmployeeName,
		"content":      item.Content,
		"click_num":    item.ClickNum,
		"clickNum":     item.ClickNum,
		"click_info":   clickInfo,
		"clickInfo":    clickInfo,
		"createdAt":    item.CreatedAt,
		"created_at":   item.CreatedAt,
		"updatedAt":    item.UpdatedAt,
		"updated_at":   item.UpdatedAt,
	}
}

func radarMustJSON(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return "[]"
	}
	return string(raw)
}
