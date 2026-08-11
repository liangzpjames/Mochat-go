package dashboard

import (
	"net/http"
	"strings"
)

type WorkContactBatchLabelingValues struct {
	ContactIDs []int
	TagIDs     []int
}

func (h *WorkReadHandler) WorkContactBatchLabeling(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if principalScope.WorkEmployeeID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "请先选择企业", nil)
		return
	}
	values, err := parseWorkContactBatchLabelingRequest(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	if len(values.ContactIDs) == 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "客户id必传", nil)
		return
	}
	if len(values.TagIDs) == 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "标签id必传", nil)
		return
	}
	if _, err := h.store.BatchLabelWorkContacts(r.Context(), values.ContactIDs, values.TagIDs, principalScope.WorkEmployeeID); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "批量打标签失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func parseWorkContactBatchLabelingRequest(r *http.Request) (WorkContactBatchLabelingValues, error) {
	params, err := parseRequestParams(r)
	if err != nil {
		return WorkContactBatchLabelingValues{}, err
	}
	contactRaw := strings.TrimSpace(stringParam(params, "contactId"))
	tagRaw := strings.TrimSpace(stringParam(params, "tagId"))
	return WorkContactBatchLabelingValues{
		ContactIDs: parseIDList(contactRaw),
		TagIDs:     parseIDList(tagRaw),
	}, nil
}
