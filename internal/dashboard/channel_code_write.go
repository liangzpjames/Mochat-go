package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (h *ChannelCodeHandler) Store(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, user, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if len(loginInfo.CorpIDs) != 1 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "请先选择企业", nil)
		return
	}
	if _, err := h.authorizeAccess(r.Context(), r, userID, loginInfo); err != nil {
		writeAccessError(w, err)
		return
	}
	values, ok := h.channelCodeWriteValues(w, r, loginInfo.CorpIDs[0], loginInfo.WorkEmployeeID)
	if !ok {
		return
	}
	if err := h.checkChannelCodeEmployees(values.DrainageEmployee); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !enforceSaaSQuota(r.Context(), w, h.store, user.TenantID, SaaSMetricChannelCodes, 1) {
		return
	}
	channelCodeID, err := h.store.CreateChannelCode(r.Context(), values)
	if err != nil {
		if writeSaaSQuotaError(w, err) {
			return
		}
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "新建渠道码失败", nil)
		return
	}
	if err := h.refreshChannelCodeQRCode(r.Context(), values, channelCodeID, ""); err != nil {
		_ = h.store.DeleteChannelCode(r.Context(), channelCodeID)
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := refreshSaaSUsageCounter(r.Context(), h.store, user.TenantID, SaaSMetricChannelCodes); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *ChannelCodeHandler) Update(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if len(loginInfo.CorpIDs) != 1 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "请先选择企业", nil)
		return
	}
	if _, err := h.authorizeAccess(r.Context(), r, userID, loginInfo); err != nil {
		writeAccessError(w, err)
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	channelCodeID, ok, err := intParam(params, "channelCodeId")
	if !ok {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "渠道码id必传", nil)
		return
	}
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "渠道码id必须为整型", nil)
		return
	}
	values, ok := channelCodeWriteValuesFromParams(w, params, loginInfo.CorpIDs[0], loginInfo.WorkEmployeeID)
	if !ok {
		return
	}
	if err := h.checkChannelCodeEmployees(values.DrainageEmployee); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	wxConfigID, err := h.store.UpdateChannelCode(r.Context(), channelCodeID, values)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "编辑渠道码失败", nil)
		return
	}
	if wxConfigID == "" {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "查询不到渠道码信息", nil)
		return
	}
	if err := h.refreshChannelCodeQRCode(r.Context(), values, channelCodeID, wxConfigID); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *ChannelCodeHandler) channelCodeWriteValues(w http.ResponseWriter, r *http.Request, corpID int, operationEmployeeID int) (ChannelCodeWriteValues, bool) {
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return ChannelCodeWriteValues{}, false
	}
	return channelCodeWriteValuesFromParams(w, params, corpID, operationEmployeeID)
}

func channelCodeWriteValuesFromParams(w http.ResponseWriter, params map[string]any, corpID int, operationEmployeeID int) (ChannelCodeWriteValues, bool) {
	baseInfo, ok := channelCodeMapParam(params, "baseInfo")
	if !ok {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "baseInfo 必传", nil)
		return ChannelCodeWriteValues{}, false
	}
	drainageEmployee, ok := channelCodeMapParam(params, "drainageEmployee")
	if !ok {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "drainageEmployee 必传", nil)
		return ChannelCodeWriteValues{}, false
	}
	welcomeMessage, ok := channelCodeMapParam(params, "welcomeMessage")
	if !ok {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "welcomeMessage 必传", nil)
		return ChannelCodeWriteValues{}, false
	}
	groupID := channelCodeInt(baseInfo["groupId"])
	name := strings.TrimSpace(channelCodeString(baseInfo["name"]))
	if name == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "活码名称必传", nil)
		return ChannelCodeWriteValues{}, false
	}
	autoAddFriend := channelCodeInt(baseInfo["autoAddFriend"])
	if autoAddFriend == 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "自动添加好友必传", nil)
		return ChannelCodeWriteValues{}, false
	}
	channelType := channelCodeInt(drainageEmployee["type"])
	if channelType == 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "渠道码类型必传", nil)
		return ChannelCodeWriteValues{}, false
	}
	return ChannelCodeWriteValues{
		CorpID:            corpID,
		GroupID:           groupID,
		Name:              name,
		AutoAddFriend:     autoAddFriend,
		Tags:              channelCodeInts(baseInfo["tags"]),
		Type:              channelType,
		DrainageEmployee:  drainageEmployee,
		WelcomeMessage:    welcomeMessage,
		OperationEmployee: operationEmployeeID,
	}, true
}

func (h *ChannelCodeHandler) checkChannelCodeEmployees(drainage map[string]any) error {
	if channelCodeInt(drainage["type"]) == 1 {
		return nil
	}
	if special, ok := channelCodeMap(drainage["specialPeriod"]); ok && channelCodeInt(special["status"]) == 1 {
		for _, detail := range channelCodeMaps(special["detail"]) {
			for _, slot := range channelCodeMaps(detail["timeSlot"]) {
				if len(channelCodeInts(slot["employeeId"])) > 100 {
					return fmt.Errorf("特殊时期内每个时间段最多配置100个使用成员")
				}
			}
		}
	}
	for _, employee := range channelCodeMaps(drainage["employees"]) {
		for _, slot := range channelCodeMaps(employee["timeSlot"]) {
			if len(channelCodeInts(slot["employeeId"])) > 100 {
				return fmt.Errorf("特殊时期内每个时间段最多配置100个使用成员")
			}
		}
	}
	if addMax, ok := channelCodeMap(drainage["addMax"]); ok && channelCodeInt(addMax["status"]) == 1 {
		if len(channelCodeInts(addMax["spareEmployeeIds"])) > 100 {
			return fmt.Errorf("备用员工最多配置100个使用成员")
		}
	}
	return nil
}

func (h *ChannelCodeHandler) refreshChannelCodeQRCode(ctx context.Context, values ChannelCodeWriteValues, channelCodeID int, wxConfigID string) error {
	if h.wecom == nil {
		return fmt.Errorf("企业微信客户端未配置")
	}
	employeeIDs := channelCodeActiveEmployeeIDs(values.DrainageEmployee, time.Now())
	if len(employeeIDs) == 0 {
		return nil
	}
	if addMax, ok := channelCodeMap(values.DrainageEmployee["addMax"]); ok && channelCodeInt(addMax["status"]) == 1 {
		counts, err := h.store.ChannelCodeContactCountsByEmployee(ctx, employeeIDs)
		if err != nil {
			return err
		}
		employeeIDs = channelCodeApplyAddMax(employeeIDs, addMax, counts)
	}
	if len(employeeIDs) == 0 {
		return nil
	}
	wxUserIDs, err := h.store.ChannelCodeEmployeeWXUserIDs(ctx, employeeIDs)
	if err != nil {
		return err
	}
	if len(wxUserIDs) == 0 {
		return nil
	}
	credential, found, err := h.store.RoomWelcomeCorpCredentialByID(ctx, values.CorpID)
	if err != nil {
		return err
	}
	if !found || credential.WXCorpID == "" || credential.ContactSecret == "" {
		return fmt.Errorf("企业微信企业配置不存在")
	}
	skipVerify := values.AutoAddFriend == 1
	state := "channelCode-" + strconv.Itoa(channelCodeID)
	if wxConfigID == "" {
		qrCodeURL, configID, err := h.wecom.CreateContactWay(ctx, credential, wxUserIDs, skipVerify, state)
		if err != nil {
			return err
		}
		return h.store.UpdateChannelCodeQRCode(ctx, channelCodeID, qrCodeURL, configID)
	}
	return h.wecom.UpdateContactWay(ctx, credential, wxConfigID, wxUserIDs, skipVerify, state)
}

func channelCodeActiveEmployeeIDs(drainage map[string]any, now time.Time) []int {
	today := now.Format("2006-01-02")
	current := now.Format("15:04")
	ids := []int{}
	if special, ok := channelCodeMap(drainage["specialPeriod"]); ok && channelCodeInt(special["status"]) == 1 {
		for _, detail := range channelCodeMaps(special["detail"]) {
			startDate := channelCodeString(detail["startDate"])
			endDate := channelCodeString(detail["endDate"])
			if startDate <= today && today <= endDate {
				for _, slot := range channelCodeMaps(detail["timeSlot"]) {
					start := channelCodeString(slot["startTime"])
					end := channelCodeString(slot["endTime"])
					if (start <= current && current <= end) || (start == "00:00" && end == "00:00") {
						ids = channelCodeInts(slot["employeeId"])
					}
				}
			}
		}
	}
	if len(ids) == 0 {
		week := int(now.Weekday())
		for _, employee := range channelCodeMaps(drainage["employees"]) {
			if channelCodeInt(employee["week"]) != week {
				continue
			}
			for _, slot := range channelCodeMaps(employee["timeSlot"]) {
				start := channelCodeString(slot["startTime"])
				end := channelCodeString(slot["endTime"])
				if (start <= current && current <= end) || (start == "00:00" && end == "00:00") {
					ids = channelCodeInts(slot["employeeId"])
				}
			}
		}
	}
	return channelCodeUniquePositiveInts(ids)
}

func channelCodeApplyAddMax(employeeIDs []int, addMax map[string]any, counts map[int]int) []int {
	allowed := map[int]struct{}{}
	for _, item := range channelCodeMaps(addMax["employees"]) {
		employeeID := channelCodeInt(item["employeeId"])
		if employeeID <= 0 {
			continue
		}
		max := channelCodeInt(item["max"])
		if max <= 0 || counts[employeeID] < max {
			allowed[employeeID] = struct{}{}
		}
	}
	result := make([]int, 0, len(employeeIDs))
	for _, id := range channelCodeUniquePositiveInts(employeeIDs) {
		if _, ok := allowed[id]; ok {
			result = append(result, id)
		}
	}
	if len(result) > 0 {
		return result
	}
	return channelCodeUniquePositiveInts(channelCodeInts(addMax["spareEmployeeIds"]))
}

func channelCodeMapParam(params map[string]any, key string) (map[string]any, bool) {
	value, ok := params[key]
	if !ok {
		return nil, false
	}
	return channelCodeMap(value)
}

func channelCodeMap(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil, false
		}
		var decoded map[string]any
		decoder := json.NewDecoder(strings.NewReader(typed))
		decoder.UseNumber()
		if err := decoder.Decode(&decoded); err == nil {
			return decoded, true
		}
	}
	return nil, false
}

func channelCodeMaps(value any) []map[string]any {
	values := []any{}
	switch typed := value.(type) {
	case []any:
		values = typed
	case []map[string]any:
		result := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, item)
		}
		return result
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		decoder := json.NewDecoder(strings.NewReader(typed))
		decoder.UseNumber()
		_ = decoder.Decode(&values)
	default:
		if value != nil {
			values = []any{value}
		}
	}
	result := make([]map[string]any, 0, len(values))
	for _, value := range values {
		if item, ok := channelCodeMap(value); ok {
			result = append(result, item)
		}
	}
	return result
}

func channelCodeInts(value any) []int {
	values := []any{}
	switch typed := value.(type) {
	case nil:
		return []int{}
	case []any:
		values = typed
	case []int:
		result := append([]int{}, typed...)
		return channelCodeUniquePositiveInts(result)
	case []string:
		for _, item := range typed {
			values = append(values, item)
		}
	case string:
		raw := strings.TrimSpace(typed)
		if raw == "" {
			return []int{}
		}
		if strings.HasPrefix(raw, "[") {
			decoder := json.NewDecoder(strings.NewReader(raw))
			decoder.UseNumber()
			if err := decoder.Decode(&values); err == nil {
				break
			}
		}
		for _, part := range strings.Split(raw, ",") {
			values = append(values, strings.TrimSpace(part))
		}
	default:
		values = []any{typed}
	}
	result := make([]int, 0, len(values))
	for _, value := range values {
		if integer := channelCodeInt(value); integer > 0 {
			result = append(result, integer)
		}
	}
	return channelCodeUniquePositiveInts(result)
}

func channelCodeUniquePositiveInts(values []int) []int {
	seen := map[int]struct{}{}
	result := make([]int, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func channelCodeInt(value any) int {
	switch typed := value.(type) {
	case nil:
		return 0
	case json.Number:
		integer, _ := typed.Int64()
		return int(integer)
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		integer, _ := strconv.Atoi(strings.TrimSpace(typed))
		return integer
	default:
		integer, _ := strconv.Atoi(strings.TrimSpace(fmt.Sprint(value)))
		return integer
	}
}

func channelCodeString(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}
