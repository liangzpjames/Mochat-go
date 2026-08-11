package dashboard

import (
	"encoding/json"
	"net/http"
	"strings"
)

type WorkRoomBatchUpdateValues struct {
	CorpID          int
	WorkRoomGroupID int
	WorkRoomIDs     []int
}

func (h *WorkReadHandler) WorkRoomBatchUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
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
	workRoomIDsRaw, present, isString := requiredStringParam(params, "workRoomIds")
	if !isString {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "客户群ID 必需为字符串", nil)
		return
	}
	if !present || strings.TrimSpace(workRoomIDsRaw) == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "客户群ID 必填", nil)
		return
	}
	workRoomGroupID, okInt, err := intParam(params, "workRoomGroupId")
	if !okInt {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "客户群分组ID 必填", nil)
		return
	}
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "客户群分组ID 必需为整数", nil)
		return
	}
	if workRoomGroupID != 0 {
		group, found, err := h.store.WorkRoomGroupByID(r.Context(), workRoomGroupID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		if !found {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "该分组信息不存在，不可操作", nil)
			return
		}
		if group.CorpID != corpID {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "该分组不归属当前登录企业，不可操作", nil)
			return
		}
	}

	if _, err := h.store.UpdateWorkRoomsGroup(r.Context(), WorkRoomBatchUpdateValues{
		CorpID:          corpID,
		WorkRoomGroupID: workRoomGroupID,
		WorkRoomIDs:     parseIDList(workRoomIDsRaw),
	}); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "更新客户群分组失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func requiredStringParam(params map[string]any, key string) (string, bool, bool) {
	value, ok := params[key]
	if !ok || value == nil {
		return "", false, true
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed), true, true
	case []string:
		if len(typed) == 0 {
			return "", true, true
		}
		return strings.TrimSpace(typed[0]), true, true
	case json.Number, float64, int, []any:
		return "", true, false
	default:
		return "", true, false
	}
}
