package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

const (
	contactEmployeeTrackEventTag  = 2
	contactEmployeeTrackEventInfo = 3
)

type WorkContactUpdateValues struct {
	CorpID      int
	ContactID   int
	EmployeeID  int
	Remark      *string
	Description *string
	BusinessNo  *string
	HasTag      bool
	TagIDs      []int
}

type WorkContactUpdateResult struct {
	WXUserID         string
	WXExternalUserID string
	AddedWXTagIDs    []string
	AddedTagNames    []string
}

type WorkContactUpdateWeComClient interface {
	UpdateExternalContactRemark(ctx context.Context, credential RoomWelcomeCorpCredential, payload WorkContactRemarkPayload) error
	MarkExternalContactTags(ctx context.Context, credential RoomWelcomeCorpCredential, payload WorkContactMarkTagsPayload) error
}

type WorkContactRemarkPayload struct {
	UserID         string
	ExternalUserID string
	Remark         *string
	Description    *string
}

type WorkContactMarkTagsPayload struct {
	UserID         string
	ExternalUserID string
	AddTag         []string
}

func (h *WorkReadHandler) WithWorkContactUpdateClient(client WorkContactUpdateWeComClient) *WorkReadHandler {
	h.workContactUpdateClient = client
	return h
}

func (h *WorkReadHandler) WorkContactUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if _, err := h.authorizeAccess(r.Context(), r, userID, loginInfo); err != nil {
		writeAccessError(w, err)
		return
	}
	if len(loginInfo.CorpIDs) != 1 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "请先选择企业", nil)
		return
	}
	values, err := parseWorkContactUpdateRequest(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	if values.ContactID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "客户id必传", nil)
		return
	}
	if values.EmployeeID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "员工id必传", nil)
		return
	}
	values.CorpID = loginInfo.CorpIDs[0]
	h.writeWorkContactUpdate(w, r, values)
}

func (h *WorkReadHandler) SidebarWorkContactUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	employee, ok := h.resolveSidebarAccess(w, r)
	if !ok {
		return
	}
	values, err := parseWorkContactUpdateRequest(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	if values.ContactID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "客户id必传", nil)
		return
	}
	values.CorpID = employee.CorpID
	values.EmployeeID = employee.ID
	h.writeWorkContactUpdate(w, r, values)
}

func (h *WorkReadHandler) writeWorkContactUpdate(w http.ResponseWriter, r *http.Request, values WorkContactUpdateValues) {
	result, found, err := h.store.UpdateWorkContactProfile(r.Context(), values)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusOK, 200, "success", []any{})
		return
	}
	if h.workContactUpdateClient != nil && result.WXUserID != "" && result.WXExternalUserID != "" {
		credential, found, err := h.store.RoomWelcomeCorpCredentialByID(r.Context(), values.CorpID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		if !found || strings.TrimSpace(credential.WXCorpID) == "" || strings.TrimSpace(credential.ContactSecret) == "" {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "企业微信配置不存在", nil)
			return
		}
		if values.Remark != nil || values.Description != nil {
			if err := h.workContactUpdateClient.UpdateExternalContactRemark(r.Context(), credential, WorkContactRemarkPayload{
				UserID:         result.WXUserID,
				ExternalUserID: result.WXExternalUserID,
				Remark:         values.Remark,
				Description:    values.Description,
			}); err != nil {
				writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
				return
			}
		}
		if len(result.AddedWXTagIDs) > 0 {
			if err := h.workContactUpdateClient.MarkExternalContactTags(r.Context(), credential, WorkContactMarkTagsPayload{
				UserID:         result.WXUserID,
				ExternalUserID: result.WXExternalUserID,
				AddTag:         result.AddedWXTagIDs,
			}); err != nil {
				writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
				return
			}
		}
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func parseWorkContactUpdateRequest(r *http.Request) (WorkContactUpdateValues, error) {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return WorkContactUpdateValues{}, err
		}
		values := WorkContactUpdateValues{}
		values.ContactID, _ = strconv.Atoi(r.FormValue("contactId"))
		values.EmployeeID, _ = strconv.Atoi(r.FormValue("employeeId"))
		if _, ok := r.Form["remark"]; ok {
			value := r.FormValue("remark")
			values.Remark = &value
		}
		if _, ok := r.Form["description"]; ok {
			value := r.FormValue("description")
			values.Description = &value
		}
		if _, ok := r.Form["businessNo"]; ok {
			value := r.FormValue("businessNo")
			values.BusinessNo = &value
		}
		if _, ok := r.Form["tag"]; ok {
			values.HasTag = true
			values.TagIDs = parseIDList(r.FormValue("tag"))
		}
		return values, nil
	}

	var raw map[string]json.RawMessage
	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil {
		return WorkContactUpdateValues{}, err
	}
	values := WorkContactUpdateValues{}
	_ = json.Unmarshal(raw["contactId"], &values.ContactID)
	_ = json.Unmarshal(raw["employeeId"], &values.EmployeeID)
	setString := func(key string) *string {
		rawValue, ok := raw[key]
		if !ok || string(rawValue) == "null" {
			return nil
		}
		var value string
		if err := json.Unmarshal(rawValue, &value); err == nil {
			return &value
		}
		var number json.Number
		if err := json.Unmarshal(rawValue, &number); err == nil {
			value = number.String()
			return &value
		}
		return nil
	}
	values.Remark = setString("remark")
	values.Description = setString("description")
	values.BusinessNo = setString("businessNo")
	if tagRaw, ok := raw["tag"]; ok && string(tagRaw) != "null" {
		values.HasTag = true
		values.TagIDs = parseWorkContactUpdateTagIDs(tagRaw)
	}
	return values, nil
}

func parseWorkContactUpdateTagIDs(raw json.RawMessage) []int {
	var ids []int
	if err := json.Unmarshal(raw, &ids); err == nil {
		return uniquePositiveIntList(ids)
	}
	var stringsValue []string
	if err := json.Unmarshal(raw, &stringsValue); err == nil {
		ids = make([]int, 0, len(stringsValue))
		for _, item := range stringsValue {
			if id, err := strconv.Atoi(strings.TrimSpace(item)); err == nil && id > 0 {
				ids = append(ids, id)
			}
		}
		return uniquePositiveIntList(ids)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err == nil {
		return parseIDList(value)
	}
	return []int{}
}

func workContactUpdateTagContent(names []string) string {
	var builder strings.Builder
	builder.WriteString("系统对该客户打标签")
	for i, name := range names {
		builder.WriteString("【")
		builder.WriteString(name)
		builder.WriteString("】")
		if i != len(names)-1 {
			builder.WriteString("、")
		}
	}
	return builder.String()
}

func workContactUpdateError(label string) error {
	return fmt.Errorf("%s修改失败", label)
}
