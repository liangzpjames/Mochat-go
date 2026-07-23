package dashboard

import (
	"context"
	"net/http"
)

type MediumGroup struct {
	ID   int
	Name string
}

type MediumGroupWrite struct {
	CorpID int
	Name   string
}

type MediumGroupStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	SidebarEmployeeByID(ctx context.Context, employeeID int) (SidebarEmployee, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	MediumGroupsByCorpID(ctx context.Context, corpID int) ([]MediumGroup, error)
	MediumGroupNameExists(ctx context.Context, corpID int, name string, excludeGroupID int) (bool, error)
	CreateMediumGroup(ctx context.Context, values MediumGroupWrite) (int, error)
	UpdateMediumGroup(ctx context.Context, groupID int, values MediumGroupWrite) (bool, error)
	DeleteMediumGroupReassignMedia(ctx context.Context, corpID int, groupID int) (bool, error)
}

type MediumGroupHandler struct {
	store      MediumGroupStore
	cache      LoginCache
	resolver   UserIDResolver
	sidebar    UserIDResolver
	authorizer CorpAdminAuthorizer
}

func NewMediumGroupHandler(store MediumGroupStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer) *MediumGroupHandler {
	return &MediumGroupHandler{store: store, cache: cache, resolver: resolver, authorizer: authorizer}
}

func (h *MediumGroupHandler) WithSidebarEmployeeResolver(resolver UserIDResolver) *MediumGroupHandler {
	h.sidebar = resolver
	return h
}

func (h *MediumGroupHandler) Index(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	if h.authorizer != nil {
		if _, err := h.authorizer.Resolve(r.Context(), userID, "/dashboard/mediumGroup/index#get", corpID, loginInfo.WorkEmployeeID); err != nil {
			writeAccessError(w, err)
			return
		}
	}
	h.writeGroups(w, r, corpID, "全部分组")
}

func (h *MediumGroupHandler) Store(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	if h.authorizer != nil {
		if _, err := h.authorizer.Resolve(r.Context(), userID, "/dashboard/mediumGroup/store#post", corpID, loginInfo.WorkEmployeeID); err != nil {
			writeAccessError(w, err)
			return
		}
	}

	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	name := stringParam(params, "name")
	if name == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "请输入分组名称", nil)
		return
	}
	exists, err := h.store.MediumGroupNameExists(r.Context(), corpID, name, 0)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if exists {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "分组名称已存在", nil)
		return
	}
	id, err := h.store.CreateMediumGroup(r.Context(), MediumGroupWrite{CorpID: corpID, Name: name})
	if err != nil || id <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "添加失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{"id": id})
}

func (h *MediumGroupHandler) Update(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	if h.authorizer != nil {
		if _, err := h.authorizer.Resolve(r.Context(), userID, "/dashboard/mediumGroup/update#put", corpID, loginInfo.WorkEmployeeID); err != nil {
			writeAccessError(w, err)
			return
		}
	}

	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	groupID, okInt, err := intParam(params, "id")
	if err != nil || !okInt || groupID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "唯一标识ID必须", nil)
		return
	}
	name := stringParam(params, "name")
	if name == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "请输入分组名称", nil)
		return
	}
	exists, err := h.store.MediumGroupNameExists(r.Context(), corpID, name, groupID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if exists {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "分组名称已存在", nil)
		return
	}
	updated, err := h.store.UpdateMediumGroup(r.Context(), groupID, MediumGroupWrite{CorpID: corpID, Name: name})
	if err != nil || !updated {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "修改失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *MediumGroupHandler) Destroy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	if h.authorizer != nil {
		if _, err := h.authorizer.Resolve(r.Context(), userID, "/dashboard/mediumGroup/destroy#delete", corpID, loginInfo.WorkEmployeeID); err != nil {
			writeAccessError(w, err)
			return
		}
	}

	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	groupID, okInt, err := intParam(params, "id")
	if err != nil || !okInt || groupID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "唯一标识ID必须", nil)
		return
	}
	deleted, err := h.store.DeleteMediumGroupReassignMedia(r.Context(), corpID, groupID)
	if err != nil || !deleted {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "删除失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *MediumGroupHandler) SidebarIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	corpID, ok := h.sidebarCorpID(w, r)
	if !ok {
		return
	}
	h.writeGroups(w, r, corpID, "未分组")
}

func (h *MediumGroupHandler) writeGroups(w http.ResponseWriter, r *http.Request, corpID int, firstName string) {
	groups, err := h.store.MediumGroupsByCorpID(r.Context(), corpID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	data := make([]map[string]any, 0, len(groups)+1)
	data = append(data, map[string]any{"id": 0, "name": firstName})
	for _, group := range groups {
		data = append(data, map[string]any{"id": group.ID, "name": group.Name})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", data)
}

func (h *MediumGroupHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (int, User, LoginCorpInfo, bool) {
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

func (h *MediumGroupHandler) sidebarCorpID(w http.ResponseWriter, r *http.Request) (int, bool) {
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
