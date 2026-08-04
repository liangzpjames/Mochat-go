package dashboard

import (
	"context"
	"net/http"
	"strings"
)

type MaterialReference struct {
	SourceType string `json:"sourceType"`
	SourceID   int    `json:"sourceId"`
	SourceName string `json:"sourceName"`
}

type mediumAvailabilityValidator interface {
	MediumAvailableToUser(context.Context, int, int, int) (bool, error)
}

type MaterialFoundationStore interface {
	MediumStore
	MaterialReferences(ctx context.Context, corpID int, ids []int) ([]MaterialReference, error)
	BatchDeleteMedium(ctx context.Context, corpID int, ids []int) (bool, error)
	BatchUpdateMediumGroupID(ctx context.Context, corpID int, ids []int, groupID int) (bool, error)
}

func (h *MediumHandler) BatchGroupUpdate(w http.ResponseWriter, r *http.Request) {
	store, userID, corpID, _, ok := h.materialWriteAccess(w, r, "/dashboard/medium/batchGroupUpdate#post")
	if !ok {
		return
	}
	_ = userID
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	ids := positiveIDs(params["ids"])
	groupID, hasGroup, err := intParam(params, "mediumGroupId")
	if len(ids) == 0 || err != nil || !hasGroup || groupID < 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "素材与分组必需", nil)
		return
	}
	updated, err := store.BatchUpdateMediumGroupID(r.Context(), corpID, ids, groupID)
	if err != nil || !updated {
		writeEnvelope(w, http.StatusConflict, http.StatusConflict, "批量移动失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{"updated": len(ids)})
}

func (h *MediumHandler) ReferenceCheck(w http.ResponseWriter, r *http.Request) {
	store, _, corpID, _, ok := h.materialWriteAccess(w, r, "/dashboard/medium/referenceCheck#post")
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	ids := positiveIDs(params["ids"])
	if len(ids) == 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "素材ID必需", nil)
		return
	}
	references, err := store.MaterialReferences(r.Context(), corpID, ids)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{"deletable": len(references) == 0, "references": references})
}

func (h *MediumHandler) BatchDestroy(w http.ResponseWriter, r *http.Request) {
	store, _, corpID, _, ok := h.materialWriteAccess(w, r, "/dashboard/medium/batchDestroy#post")
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	ids := positiveIDs(params["ids"])
	if len(ids) == 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "素材ID必需", nil)
		return
	}
	references, err := store.MaterialReferences(r.Context(), corpID, ids)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if len(references) > 0 {
		writeEnvelope(w, http.StatusConflict, http.StatusConflict, "素材正在被引用", map[string]any{"deletable": false, "references": references})
		return
	}
	deleted, err := store.BatchDeleteMedium(r.Context(), corpID, ids)
	if err != nil || !deleted {
		writeEnvelope(w, http.StatusConflict, http.StatusConflict, "批量删除失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{"deleted": len(ids)})
}

func (h *MediumHandler) Selector(w http.ResponseWriter, r *http.Request) {
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
		if _, err := h.authorizer.Resolve(r.Context(), userID, "/dashboard/materialSelector/index#get", corpID, loginInfo.WorkEmployeeID); err != nil {
			writeAccessError(w, err)
			return
		}
	}
	filter := mediumFilterFromQuery(r, corpID, userID, false)
	employeeID, err := h.store.EmployeeIDByUserCorp(r.Context(), userID, corpID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	filter.ScopeType = ""
	filter.ScopeID = 0
	filter.SelectorVisible = true
	filter.UserID = userID
	filter.EmployeeID = employeeID
	filter.Status = "available"
	filter.PerPage = positiveQueryInt(r, "perPage", 50)
	if strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("scene")), "chat") {
		visible := true
		filter.SidebarVisible = &visible
	}
	page, err := h.store.MediumPage(r.Context(), filter)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	items := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, map[string]any{"id": item.ID, "name": materialName(item), "type": mediumTypeText(item.Type), "preview": materialPreview(item), "groupId": item.MediumGroupID, "groupName": item.MediumGroupName, "scopeType": mediumScopeType(item.ScopeType)})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{"list": items, "total": page.Total})
}

func (h *MediumHandler) materialWriteAccess(w http.ResponseWriter, r *http.Request, permission string) (MaterialFoundationStore, int, int, LoginCorpInfo, bool) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return nil, 0, 0, LoginCorpInfo{}, false
	}
	store, ok := h.store.(MaterialFoundationStore)
	if !ok {
		writeEnvelope(w, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "素材 Provider 未配置", nil)
		return nil, 0, 0, LoginCorpInfo{}, false
	}
	userID, _, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return nil, 0, 0, LoginCorpInfo{}, false
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return nil, 0, 0, LoginCorpInfo{}, false
	}
	if h.authorizer != nil {
		if _, err := h.authorizer.Resolve(r.Context(), userID, permission, corpID, loginInfo.WorkEmployeeID); err != nil {
			writeAccessError(w, err)
			return nil, 0, 0, LoginCorpInfo{}, false
		}
	}
	return store, userID, corpID, loginInfo, true
}

func positiveIDs(value any) []int {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	seen := map[int]struct{}{}
	ids := make([]int, 0, len(raw))
	for _, item := range raw {
		id, ok, err := intParam(map[string]any{"id": item}, "id")
		if err != nil || !ok || id <= 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

func materialName(item MediumItem) string {
	for _, key := range []string{"title", "name", "fileName"} {
		if value := strings.TrimSpace(toString(item.Content[key])); value != "" {
			return value
		}
	}
	return "素材"
}

func materialPreview(item MediumItem) string {
	for _, key := range []string{"content", "description", "imagePath", "filePath", "videoPath"} {
		if value := strings.TrimSpace(toString(item.Content[key])); value != "" {
			return value
		}
	}
	return materialName(item)
}
