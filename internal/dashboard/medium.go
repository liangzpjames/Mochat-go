package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultMediumFileStorageRoot   = "./storage/upload/static"
	mediumTemporaryMediaFreshLimit = 60*60*24*3 - 60*60*2
)

type MediumFilter struct {
	CorpID          int
	Search          string
	MediumGroupID   *int
	Type            int
	ScopeType       string
	ScopeID         int
	SelectorVisible bool
	UserID          int
	EmployeeID      int
	Status          string
	SidebarVisible  *bool
	Page            int
	PerPage         int
}

type MediumItem struct {
	ID              int
	Type            int
	MediaID         string
	Content         map[string]any
	CorpID          int
	MediumGroupID   int
	MediumGroupName string
	UserID          int
	UserName        string
	ScopeType       string
	ScopeID         int
	SidebarVisible  bool
	Status          string
	CreatedAt       string
}

type MediumPage struct {
	Items     []MediumItem
	Total     int
	TotalPage int
	PerPage   int
}

type MediumWrite struct {
	CorpID         int
	Type           int
	IsSync         int
	Content        string
	MediumGroupID  int
	UserID         int
	UserName       string
	ScopeType      string
	ScopeID        int
	SidebarVisible bool
	Status         string
}

type MediumMediaUpdateItem struct {
	ID             int
	MediaID        string
	LastUploadTime int64
	Type           int
	Content        map[string]any
}

type MediumCorpCredential struct {
	CorpID         int
	WXCorpID       string
	EmployeeSecret string
}

type MediumStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	SidebarEmployeeByID(ctx context.Context, employeeID int) (SidebarEmployee, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	MediumPage(ctx context.Context, filter MediumFilter) (MediumPage, error)
	MediumByID(ctx context.Context, corpID int, mediumID int) (MediumItem, bool, error)
	CreateMedium(ctx context.Context, values MediumWrite) (int, error)
	UpdateMedium(ctx context.Context, mediumID int, values MediumWrite) (bool, error)
	DeleteMedium(ctx context.Context, corpID int, mediumID int) (bool, error)
	UpdateMediumGroupID(ctx context.Context, corpID int, mediumID int, groupID int) (bool, error)
	MediumMediaForUpdateByID(ctx context.Context, mediumID int) (MediumMediaUpdateItem, bool, error)
	UpdateMediumMediaID(ctx context.Context, mediumID int, mediaID string, lastUploadTime int64) (bool, error)
	MediumCorpCredentialByID(ctx context.Context, corpID int) (MediumCorpCredential, bool, error)
}

type MediumMediaClient interface {
	UploadTemporaryMedia(ctx context.Context, credential MediumCorpCredential, mediaType string, filePath string) (string, error)
}

type MediumHandler struct {
	store           MediumStore
	cache           LoginCache
	resolver        UserIDResolver
	sidebar         UserIDResolver
	authorizer      CorpAdminAuthorizer
	apiBaseURL      string
	fileStorageRoot string
	mediaClient     MediumMediaClient
}

func NewMediumHandler(store MediumStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer, apiBaseURL string) *MediumHandler {
	return NewMediumHandlerWithMediaClient(store, cache, resolver, authorizer, apiBaseURL, defaultMediumFileStorageRoot, NewRoomWelcomeWeComClient(defaultWeComAPIBaseURL))
}

func NewMediumHandlerWithMediaClient(store MediumStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer, apiBaseURL string, fileStorageRoot string, mediaClient MediumMediaClient) *MediumHandler {
	fileStorageRoot = strings.TrimSpace(fileStorageRoot)
	if fileStorageRoot == "" {
		fileStorageRoot = defaultMediumFileStorageRoot
	}
	return &MediumHandler{
		store:           store,
		cache:           cache,
		resolver:        resolver,
		authorizer:      authorizer,
		apiBaseURL:      strings.TrimRight(apiBaseURL, "/"),
		fileStorageRoot: fileStorageRoot,
		mediaClient:     mediaClient,
	}
}

func (h *MediumHandler) WithSidebarEmployeeResolver(resolver UserIDResolver) *MediumHandler {
	h.sidebar = resolver
	return h
}

func (h *MediumHandler) Index(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	if h.authorizer != nil {
		if _, err := h.authorizer.Resolve(r.Context(), userID, "/dashboard/medium/index#get", corpID, principalScope.WorkEmployeeID); err != nil {
			writeAccessError(w, err)
			return
		}
	}
	h.writeIndex(w, r, mediumFilterFromQuery(r, corpID, userID, false))
}

func (h *MediumHandler) SidebarIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	corpID, ok := h.sidebarCorpID(w, r)
	if !ok {
		return
	}
	h.writeIndex(w, r, mediumFilterFromQuery(r, corpID, 0, true))
}

func (h *MediumHandler) MediaIDUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	corpID, ok := h.sidebarCorpID(w, r)
	if !ok {
		return
	}
	mediumID, err := positiveQueryIntRequired(r, "mediumId")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "素材id必须", nil)
		return
	}
	item, found, err := h.store.MediumMediaForUpdateByID(r.Context(), mediumID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		h.writeMediaID(w, "")
		return
	}
	if time.Now().Unix()-item.LastUploadTime <= mediumTemporaryMediaFreshLimit {
		h.writeMediaID(w, item.MediaID)
		return
	}
	mediaType := mediumWXMediaType(item.Type)
	if mediaType == "" {
		h.writeMediaID(w, item.MediaID)
		return
	}
	filePath := strings.TrimSpace(toString(item.Content[mediaType+"Path"]))
	if filePath == "" {
		h.writeMediaID(w, item.MediaID)
		return
	}
	localPath := h.mediumLocalFilePath(filePath)
	info, err := os.Stat(localPath)
	if err != nil || info.IsDir() {
		h.writeMediaID(w, item.MediaID)
		return
	}
	credential, found, err := h.store.MediumCorpCredentialByID(r.Context(), corpID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found || strings.TrimSpace(credential.WXCorpID) == "" || strings.TrimSpace(credential.EmployeeSecret) == "" || h.mediaClient == nil {
		h.writeMediaID(w, item.MediaID)
		return
	}
	now := time.Now().Unix()
	mediaID, err := h.mediaClient.UploadTemporaryMedia(r.Context(), credential, mediaType, localPath)
	if err != nil || strings.TrimSpace(mediaID) == "" {
		h.writeMediaID(w, item.MediaID)
		return
	}
	updated, err := h.store.UpdateMediumMediaID(r.Context(), item.ID, mediaID, now)
	if err != nil || !updated {
		h.writeMediaID(w, item.MediaID)
		return
	}
	h.writeMediaID(w, mediaID)
}

func (h *MediumHandler) Show(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	if h.authorizer != nil {
		if _, err := h.authorizer.Resolve(r.Context(), userID, "/dashboard/medium/show#get", corpID, principalScope.WorkEmployeeID); err != nil {
			writeAccessError(w, err)
			return
		}
	}
	mediumID, err := positiveQueryIntRequired(r, "id")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "唯一标识ID必须", nil)
		return
	}
	item, found, err := h.store.MediumByID(r.Context(), corpID, mediumID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusOK, 200, "success", []any{})
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", h.mediumPayload(item, false))
}

func (h *MediumHandler) Store(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, user, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	if h.authorizer != nil {
		if _, err := h.authorizer.Resolve(r.Context(), userID, "/dashboard/medium/store#post", corpID, principalScope.WorkEmployeeID); err != nil {
			writeAccessError(w, err)
			return
		}
	}
	values, ok := h.parseMediumWrite(w, r, corpID, user)
	if !ok {
		return
	}
	id, err := h.store.CreateMedium(r.Context(), values)
	if err != nil || id <= 0 {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "素材添加失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{"id": id})
}

func (h *MediumHandler) Update(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, user, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	if h.authorizer != nil {
		if _, err := h.authorizer.Resolve(r.Context(), userID, "/dashboard/medium/update#put", corpID, principalScope.WorkEmployeeID); err != nil {
			writeAccessError(w, err)
			return
		}
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	mediumID, okInt, err := intParam(params, "id")
	if err != nil || !okInt || mediumID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "唯一标识ID必须", nil)
		return
	}
	values, ok := h.mediumWriteFromParams(w, params, corpID, user)
	if !ok {
		return
	}
	updated, err := h.store.UpdateMedium(r.Context(), mediumID, values)
	if err != nil || !updated {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "修改失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *MediumHandler) Destroy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	if h.authorizer != nil {
		if _, err := h.authorizer.Resolve(r.Context(), userID, "/dashboard/medium/destroy#delete", corpID, principalScope.WorkEmployeeID); err != nil {
			writeAccessError(w, err)
			return
		}
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	mediumID, okInt, err := intParam(params, "id")
	if err != nil || !okInt || mediumID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "唯一标识ID必须", nil)
		return
	}
	deleted, err := h.store.DeleteMedium(r.Context(), corpID, mediumID)
	if err != nil || !deleted {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "删除失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *MediumHandler) GroupUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	if h.authorizer != nil {
		if _, err := h.authorizer.Resolve(r.Context(), userID, "/dashboard/medium/groupUpdate#put", corpID, principalScope.WorkEmployeeID); err != nil {
			writeAccessError(w, err)
			return
		}
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	mediumID, okInt, err := intParam(params, "id")
	if err != nil || !okInt || mediumID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "唯一标识ID必须", nil)
		return
	}
	groupID, okInt, err := intParam(params, "mediumGroupId")
	if err != nil || !okInt || groupID < 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "分组ID必须", nil)
		return
	}
	updated, err := h.store.UpdateMediumGroupID(r.Context(), corpID, mediumID, groupID)
	if err != nil || !updated {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "移动失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *MediumHandler) writeIndex(w http.ResponseWriter, r *http.Request, filter MediumFilter) {
	page, err := h.store.MediumPage(r.Context(), filter)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, h.mediumPayload(item, true))
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

func (h *MediumHandler) parseMediumWrite(w http.ResponseWriter, r *http.Request, corpID int, user User) (MediumWrite, bool) {
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return MediumWrite{}, false
	}
	return h.mediumWriteFromParams(w, params, corpID, user)
}

func (h *MediumHandler) mediumWriteFromParams(w http.ResponseWriter, params map[string]any, corpID int, user User) (MediumWrite, bool) {
	mediumType, okInt, err := intParam(params, "type")
	if err != nil || !okInt || mediumType < 1 || mediumType > 7 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "素材类型必须", nil)
		return MediumWrite{}, false
	}
	groupID, okInt, err := intParam(params, "mediumGroupId")
	if err != nil || !okInt || groupID < 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "分组ID必须", nil)
		return MediumWrite{}, false
	}
	isSync, okInt, err := intParam(params, "isSync")
	if err != nil || !okInt || isSync <= 0 {
		isSync = 1
	}
	content, ok := mapParam(params, "content")
	if !ok {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "内容数据必须", nil)
		return MediumWrite{}, false
	}
	if msg := validateMediumContent(mediumType, content); msg != "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, msg, nil)
		return MediumWrite{}, false
	}
	raw, err := json.Marshal(content)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "内容数据错误", nil)
		return MediumWrite{}, false
	}
	return MediumWrite{
		CorpID:         corpID,
		Type:           mediumType,
		IsSync:         isSync,
		Content:        string(raw),
		MediumGroupID:  groupID,
		UserID:         user.ID,
		UserName:       user.Name,
		ScopeType:      mediumScopeType(stringParam(params, "scopeType")),
		ScopeID:        mediumScopeID(mediumScopeType(stringParam(params, "scopeType")), params, user.ID),
		SidebarVisible: boolParamDefault(params, "sidebarVisible", true),
		Status:         "available",
	}, true
}

func (h *MediumHandler) mediumPayload(item MediumItem, list bool) map[string]any {
	content := cloneMap(item.Content)
	addMediumFullPath(content, item.Type, h.fileFullURL)
	payload := map[string]any{
		"id":              item.ID,
		"type":            item.Type,
		"mediaId":         item.MediaID,
		"content":         content,
		"corpId":          item.CorpID,
		"mediumGroupId":   item.MediumGroupID,
		"mediumGroupName": item.MediumGroupName,
		"userId":          item.UserID,
		"userName":        item.UserName,
		"scopeType":       mediumScopeType(item.ScopeType),
		"scopeId":         item.ScopeID,
		"sidebarVisible":  item.SidebarVisible,
		"status":          item.Status,
	}
	if item.CreatedAt != "" {
		payload["createdAt"] = item.CreatedAt
	}
	if list {
		payload["type"] = mediumTypeText(item.Type)
	}
	return payload
}

func (h *MediumHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (int, User, DashboardRequestScope, bool) {
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

func (h *MediumHandler) sidebarCorpID(w http.ResponseWriter, r *http.Request) (int, bool) {
	if h.sidebar == nil {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return 0, false
	}
	employeeID, err := h.sidebar.UserID(r)
	if err != nil || employeeID <= 0 {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return 0, false
	}
	employee, found, err := h.store.SidebarEmployeeByID(r.Context(), employeeID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return 0, false
	}
	if !found {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "employee not found", nil)
		return 0, false
	}
	return employee.CorpID, true
}

func (h *MediumHandler) fileFullURL(path string) string {
	if path == "" || strings.Contains(path, "http") || h.apiBaseURL == "" {
		return path
	}
	return h.apiBaseURL + "/static/" + strings.TrimLeft(path, "/")
}

func (h *MediumHandler) mediumLocalFilePath(path string) string {
	return mediumLocalFilePath(h.fileStorageRoot, path)
}

func (h *MediumHandler) writeMediaID(w http.ResponseWriter, mediaID string) {
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{"mediaId": mediaID})
}

func mediumFilterFromQuery(r *http.Request, corpID int, userID int, sidebar bool) MediumFilter {
	scopeType := mediumScopeType(r.URL.Query().Get("scopeType"))
	filter := MediumFilter{
		CorpID:    corpID,
		Search:    strings.TrimSpace(r.URL.Query().Get("searchStr")),
		Type:      positiveQueryInt(r, "type", 0),
		Page:      positiveQueryInt(r, "page", 1),
		PerPage:   positiveQueryInt(r, "perPage", 10),
		ScopeType: scopeType,
		Status:    strings.TrimSpace(r.URL.Query().Get("status")),
	}
	if scopeType == "personal" {
		filter.ScopeID = userID
	} else if scopeType == "department" {
		filter.ScopeID = positiveQueryInt(r, "scopeId", 0)
	}
	if rawVisible, ok := r.URL.Query()["sidebarVisible"]; ok && len(rawVisible) > 0 {
		visible := rawVisible[0] == "1" || strings.EqualFold(strings.TrimSpace(rawVisible[0]), "true")
		filter.SidebarVisible = &visible
	}
	if sidebar {
		visible := true
		filter.SidebarVisible = &visible
	}
	rawValues, hasGroup := r.URL.Query()["mediumGroupId"]
	if hasGroup && len(rawValues) > 0 {
		groupID := 0
		if rawValues[0] != "" {
			parsed, err := parsePositiveOrZeroInt(rawValues[0])
			if err == nil {
				groupID = parsed
			}
		}
		if sidebar || groupID > 0 {
			filter.MediumGroupID = &groupID
		}
	}
	return filter
}

func mediumScopeType(value string) string {
	switch strings.TrimSpace(value) {
	case "department", "personal":
		return strings.TrimSpace(value)
	default:
		return "public"
	}
}

func mediumScopeID(scopeType string, params map[string]any, userID int) int {
	if scopeType == "personal" {
		return userID
	}
	if scopeType != "department" {
		return 0
	}
	value, ok, err := intParam(params, "scopeId")
	if err != nil || !ok || value < 0 {
		return 0
	}
	return value
}

func boolParamDefault(params map[string]any, key string, fallback bool) bool {
	value, ok := params[key]
	if !ok {
		return fallback
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return typed == "1" || strings.EqualFold(typed, "true")
	case float64:
		return typed != 0
	default:
		return fallback
	}
}

func parsePositiveOrZeroInt(raw string) (int, error) {
	params := map[string]any{"value": raw}
	value, _, err := intParam(params, "value")
	if err != nil {
		return 0, err
	}
	if value < 0 {
		return 0, fmt.Errorf("must be non-negative")
	}
	return value, nil
}

func mapParam(params map[string]any, key string) (map[string]any, bool) {
	value, ok := params[key]
	if !ok || value == nil {
		return nil, false
	}
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case map[string]string:
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			out[key] = value
		}
		return out, true
	default:
		return nil, false
	}
}

func validateMediumContent(mediumType int, content map[string]any) string {
	switch mediumType {
	case 1:
		if strings.TrimSpace(toString(content["title"])) == "" || strings.TrimSpace(toString(content["content"])) == "" {
			return "文本素材内容必须"
		}
	case 2:
		if strings.TrimSpace(toString(content["imagePath"])) == "" {
			return "图片必须"
		}
	case 3:
		if strings.TrimSpace(toString(content["title"])) == "" || strings.TrimSpace(toString(content["imagePath"])) == "" || strings.TrimSpace(toString(content["imageLink"])) == "" {
			return "图文素材内容必须"
		}
	case 4:
		if strings.TrimSpace(toString(content["title"])) == "" || strings.TrimSpace(toString(content["voicePath"])) == "" {
			return "音频素材内容必须"
		}
	case 5:
		if strings.TrimSpace(toString(content["videoPath"])) == "" {
			return "视频必须"
		}
	case 6:
		if strings.TrimSpace(toString(content["title"])) == "" || strings.TrimSpace(toString(content["imagePath"])) == "" || strings.TrimSpace(toString(content["appid"])) == "" || strings.TrimSpace(toString(content["page"])) == "" {
			return "小程序素材内容必须"
		}
	case 7:
		if strings.TrimSpace(toString(content["title"])) == "" || strings.TrimSpace(toString(content["filePath"])) == "" {
			return "文件素材内容必须"
		}
	}
	return ""
}

func addMediumFullPath(content map[string]any, mediumType int, fullURL func(string) string) {
	switch mediumType {
	case 2, 3, 6:
		if path := strings.TrimSpace(toString(content["imagePath"])); path != "" {
			content["imageFullPath"] = fullURL(path)
		}
	case 4:
		if path := strings.TrimSpace(toString(content["voicePath"])); path != "" {
			content["voiceFullPath"] = fullURL(path)
		}
	case 5:
		if path := strings.TrimSpace(toString(content["videoPath"])); path != "" {
			content["videoFullPath"] = fullURL(path)
		}
	case 7:
		if path := strings.TrimSpace(toString(content["filePath"])); path != "" {
			content["fileFullPath"] = fullURL(path)
		}
	}
}

func mediumTypeText(value int) string {
	switch value {
	case 1:
		return "文本"
	case 2:
		return "图片"
	case 3:
		return "图文"
	case 4:
		return "音频"
	case 5:
		return "视频"
	case 6:
		return "小程序"
	case 7:
		return "文件"
	default:
		return ""
	}
}

func mediumWXMediaType(value int) string {
	switch value {
	case 2:
		return "image"
	case 4:
		return "voice"
	case 5:
		return "video"
	case 7:
		return "file"
	default:
		return ""
	}
}

func cloneMap(input map[string]any) map[string]any {
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func toString(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}
