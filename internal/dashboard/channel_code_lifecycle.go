package dashboard

import "net/http"

func (h *ChannelCodeHandler) BatchInvalidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if len(principalScope.CorpIDs) != 1 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "请先选择企业", nil)
		return
	}
	if _, err := h.authorizeAccess(r.Context(), r, userID, principalScope); err != nil {
		writeAccessError(w, err)
		return
	}
	lifecycleStore, ok := h.store.(ChannelCodeLifecycleStore)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "渠道码生命周期能力未接入", nil)
		return
	}
	deleter, ok := h.wecom.(ChannelCodeContactWayDeleter)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "企微渠道码失效能力未接入", nil)
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	ids, err := intSliceParam(params, "ids")
	if err != nil || len(ids) == 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "ids 必须为非空整数数组", nil)
		return
	}
	if len(ids) > 100 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "单次最多作废100个渠道码", nil)
		return
	}
	items := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		item := map[string]any{"id": id, "success": false}
		credential, configID, found, lookupErr := lifecycleStore.ChannelCodeProviderConfig(r.Context(), id, principalScope.CorpIDs[0])
		if lookupErr != nil {
			item["error"] = "查询渠道码失败"
			items = append(items, item)
			continue
		}
		if !found || configID == "" {
			item["error"] = "渠道码不存在或缺少企微凭证"
			items = append(items, item)
			continue
		}
		if invalidateErr := deleter.DeleteContactWay(r.Context(), credential, configID); invalidateErr != nil {
			item["error"] = "企微同步失败"
			items = append(items, item)
			continue
		}
		if stateErr := lifecycleStore.SetChannelCodeLifecycle(r.Context(), id, principalScope.CorpIDs[0], "invalidated", "synced", ""); stateErr != nil {
			item["error"] = "保存渠道码状态失败"
			items = append(items, item)
			continue
		}
		item["success"] = true
		items = append(items, item)
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{"items": items})
}
