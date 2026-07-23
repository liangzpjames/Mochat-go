package dashboard

import (
	"context"
	"net/http"
)

type WorkRoomGroupItem struct {
	ID        int
	CorpID    int
	Name      string
	CreatedAt string
}

type WorkRoomGroupPage struct {
	Items     []WorkRoomGroupItem
	Total     int
	TotalPage int
	PerPage   int
}

type WorkRoomGroupWrite struct {
	CorpID int
	Name   string
}

type WorkRoomGroupStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	WorkRoomGroupPage(ctx context.Context, corpID int, page int, perPage int) (WorkRoomGroupPage, error)
	WorkRoomGroupByID(ctx context.Context, groupID int) (WorkRoomGroupItem, bool, error)
	WorkRoomGroupNameExists(ctx context.Context, corpID int, name string, excludeGroupID int) (bool, error)
	CreateWorkRoomGroup(ctx context.Context, values WorkRoomGroupWrite) (int, error)
	UpdateWorkRoomGroup(ctx context.Context, groupID int, name string) (bool, error)
	DeleteWorkRoomGroupReassignRooms(ctx context.Context, groupID int) (bool, error)
}

type WorkRoomGroupHandler struct {
	store    WorkRoomGroupStore
	cache    LoginCache
	resolver UserIDResolver
}

func NewWorkRoomGroupHandler(store WorkRoomGroupStore, cache LoginCache, resolver UserIDResolver) *WorkRoomGroupHandler {
	return &WorkRoomGroupHandler{store: store, cache: cache, resolver: resolver}
}

func (h *WorkRoomGroupHandler) Index(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	page, err := h.store.WorkRoomGroupPage(r.Context(), corpID, positiveQueryInt(r, "page", 1), positiveQueryInt(r, "perPage", 10))
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, workRoomGroupPayload(item))
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

func (h *WorkRoomGroupHandler) Store(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
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
	corpID, okInt, err := intParam(params, "corpId")
	if err != nil || !okInt || corpID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "企业授信ID 必填", nil)
		return
	}
	name := stringParam(params, "workRoomGroupName")
	if name == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "分组名称 必填", nil)
		return
	}
	exists, err := h.store.WorkRoomGroupNameExists(r.Context(), corpID, name, 0)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if exists {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "该客户群分组名称已存在，不可重复添加", nil)
		return
	}
	if _, err := h.store.CreateWorkRoomGroup(r.Context(), WorkRoomGroupWrite{CorpID: corpID, Name: name}); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "客户群分组创建失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *WorkRoomGroupHandler) Update(w http.ResponseWriter, r *http.Request) {
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
	groupID, okInt, err := intParam(params, "workRoomGroupId")
	if err != nil || !okInt || groupID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "客户群分组ID 必填", nil)
		return
	}
	name := stringParam(params, "workRoomGroupName")
	if name == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "分组名称 必填", nil)
		return
	}
	group, found, err := h.store.WorkRoomGroupByID(r.Context(), groupID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "该客户群分组不存在，不可操作", nil)
		return
	}
	exists, err := h.store.WorkRoomGroupNameExists(r.Context(), group.CorpID, name, groupID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if exists {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "该客户群分组名称已存在，不可更新", nil)
		return
	}
	updated, err := h.store.UpdateWorkRoomGroup(r.Context(), groupID, name)
	if err != nil || !updated {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "客户群分组更新失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *WorkRoomGroupHandler) Destroy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
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
	groupID, okInt, err := intParam(params, "workRoomGroupId")
	if err != nil || !okInt || groupID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "客户群分组ID 必填", nil)
		return
	}
	if _, found, err := h.store.WorkRoomGroupByID(r.Context(), groupID); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	} else if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "该客户群分组不存在，不可操作", nil)
		return
	}
	deleted, err := h.store.DeleteWorkRoomGroupReassignRooms(r.Context(), groupID)
	if err != nil || !deleted {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "客户群分组删除失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *WorkRoomGroupHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (int, User, LoginCorpInfo, bool) {
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

func workRoomGroupPayload(item WorkRoomGroupItem) map[string]any {
	return map[string]any{
		"corpId":            item.CorpID,
		"createdAt":         item.CreatedAt,
		"workRoomGroupId":   item.ID,
		"workRoomGroupName": item.Name,
	}
}
