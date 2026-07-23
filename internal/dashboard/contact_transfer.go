package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type ContactTransferContactFilter struct {
	CorpID       int
	ContactName  string
	EmployeeIDs  []int
	AddTimeStart string
	AddTimeEnd   string
}

type ContactTransferContactItem struct {
	ContactID     int
	EmployeeID    int
	ContactWXID   string
	EmployeeWXID  string
	ContactName   string
	NickName      string
	CorpName      string
	EmployeeName  string
	Tags          []string
	TransferState string
	AddTime       string
	AddWay        int
}

type ContactTransferUnassignedFilter struct {
	CorpID       int
	ContactName  string
	EmployeeIDs  []int
	AddTimeStart string
	AddTimeEnd   string
}

type ContactTransferUnassignedPage struct {
	Items    []ContactTransferContactItem
	LastTime string
}

type ContactTransferRoomItem struct {
	RoomID     int
	ChatID     string
	RoomName   string
	Owner      string
	UserNum    int
	AddNum     int
	QuitNum    int
	CreateTime string
}

type ContactTransferLogFilter struct {
	CorpID          int
	Mode            int
	Name            string
	EmployeeWXID    string
	CreateTimeStart string
	CreateTimeEnd   string
}

type ContactTransferLogItem struct {
	Mode       int
	ContactID  int
	RoomID     int
	Name       string
	CorpName   string
	Employee   string
	State      string
	RoomNum    int
	CreateTime string
}

type ContactTransferLogWrite struct {
	CorpID             int
	Status             int
	Type               int
	Name               string
	ContactID          string
	HandoverEmployeeID string
	TakeoverEmployeeID string
}

type ContactTransferUnassignedSeed struct {
	HandoverUserID string
	ExternalUserID string
	DimissionTime  int64
}

type ContactTransferStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	RoomWelcomeCorpCredentialByID(ctx context.Context, corpID int) (RoomWelcomeCorpCredential, bool, error)
	ContactTransferAssignedContacts(ctx context.Context, filter ContactTransferContactFilter) ([]ContactTransferContactItem, error)
	ContactTransferUnassignedContacts(ctx context.Context, filter ContactTransferUnassignedFilter) (ContactTransferUnassignedPage, error)
	ContactTransferRooms(ctx context.Context, corpID int, roomName string) ([]ContactTransferRoomItem, error)
	ContactTransferLogs(ctx context.Context, filter ContactTransferLogFilter) ([]ContactTransferLogItem, error)
	ReplaceContactTransferUnassigned(ctx context.Context, corpID int, items []ContactTransferUnassignedSeed) error
	CreateContactTransferLog(ctx context.Context, values ContactTransferLogWrite) error
	WorkContactNameByExternalUserID(ctx context.Context, corpID int, externalUserID string) (string, bool, error)
	WorkRoomNameByWXChatID(ctx context.Context, corpID int, wxChatID string) (string, bool, error)
}

type ContactTransferWeComClient interface {
	GetUnassigned(ctx context.Context, credential RoomWelcomeCorpCredential) ([]ContactTransferUnassignedSeed, error)
	TransferCustomer(ctx context.Context, credential RoomWelcomeCorpCredential, externalUserIDs []string, handoverUserID string, takeoverUserID string, successMsg string) (map[string]any, error)
	TransferGroupChat(ctx context.Context, credential RoomWelcomeCorpCredential, chatIDs []string, takeoverUserID string) ([]map[string]any, error)
}

type ContactTransferHandler struct {
	store      ContactTransferStore
	cache      LoginCache
	resolver   UserIDResolver
	authorizer CorpAdminAuthorizer
	wecom      ContactTransferWeComClient
}

func NewContactTransferHandler(store ContactTransferStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer, wecom ContactTransferWeComClient) *ContactTransferHandler {
	if wecom == nil {
		wecom = NewRoomWelcomeWeComClient(defaultWeComAPIBaseURL)
	}
	return &ContactTransferHandler{
		store:      store,
		cache:      cache,
		resolver:   resolver,
		authorizer: authorizer,
		wecom:      wecom,
	}
}

func (h *ContactTransferHandler) Info(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, ok := h.authorized(w, r)
	if !ok {
		return
	}
	filter, ok := h.contactFilterFromQuery(w, r, loginInfo)
	if !ok {
		return
	}
	items, err := h.store.ContactTransferAssignedContacts(r.Context(), filter)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", contactTransferContactPayloads(items, true))
}

func (h *ContactTransferHandler) UnassignedList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, ok := h.authorized(w, r)
	if !ok {
		return
	}
	filter, ok := h.unassignedFilterFromQuery(w, r, loginInfo)
	if !ok {
		return
	}
	page, err := h.store.ContactTransferUnassignedContacts(r.Context(), filter)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"list":     contactTransferContactPayloads(page.Items, false),
		"lastTime": page.LastTime,
	})
}

func (h *ContactTransferHandler) Room(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, ok := h.authorized(w, r)
	if !ok {
		return
	}
	corpID, ok := singleCorpID(w, loginInfo)
	if !ok {
		return
	}
	rooms, err := h.store.ContactTransferRooms(r.Context(), corpID, strings.TrimSpace(r.URL.Query().Get("roomName")))
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(rooms))
	for _, room := range rooms {
		list = append(list, map[string]any{
			"roomId":     room.RoomID,
			"chatId":     room.ChatID,
			"roomName":   room.RoomName,
			"owner":      room.Owner,
			"userNum":    room.UserNum,
			"addNum":     room.AddNum,
			"quitNum":    room.QuitNum,
			"createTime": room.CreateTime,
		})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", list)
}

func (h *ContactTransferHandler) Log(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, ok := h.authorized(w, r)
	if !ok {
		return
	}
	corpID, ok := singleCorpID(w, loginInfo)
	if !ok {
		return
	}
	mode, err := queryOptionalInt(r, "mode")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "mode 必须为整数", nil)
		return
	}
	filter := ContactTransferLogFilter{
		CorpID:          corpID,
		Mode:            mode,
		Name:            strings.TrimSpace(r.URL.Query().Get("name")),
		EmployeeWXID:    strings.TrimSpace(r.URL.Query().Get("employeeId")),
		CreateTimeStart: strings.TrimSpace(r.URL.Query().Get("createTimeStart")),
		CreateTimeEnd:   strings.TrimSpace(r.URL.Query().Get("createTimeEnd")),
	}
	items, err := h.store.ContactTransferLogs(r.Context(), filter)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if item.Mode == 2 {
			list = append(list, map[string]any{
				"roomId":     item.RoomID,
				"name":       item.Name,
				"employee":   item.Employee,
				"roomNum":    item.RoomNum,
				"createTime": item.CreateTime,
			})
			continue
		}
		list = append(list, map[string]any{
			"contactId":  item.ContactID,
			"name":       item.Name,
			"corpName":   item.CorpName,
			"employee":   item.Employee,
			"state":      item.State,
			"createTime": item.CreateTime,
		})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", list)
}

func (h *ContactTransferHandler) SaveUnassignedList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, ok := h.authorized(w, r)
	if !ok {
		return
	}
	corpID, ok := singleCorpID(w, loginInfo)
	if !ok {
		return
	}
	credential, ok := h.resolveCredential(w, r.Context(), corpID)
	if !ok {
		return
	}
	items, err := h.wecom.GetUnassigned(r.Context(), credential)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "获取离职待分配群列表失败", nil)
		return
	}
	if err := h.store.ReplaceContactTransferUnassigned(r.Context(), corpID, items); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "同步失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *ContactTransferHandler) TransferCustomer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, ok := h.authorized(w, r)
	if !ok {
		return
	}
	corpID, ok := singleCorpID(w, loginInfo)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	transferType, _, err := intParam(params, "type")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "type 必须为整数", nil)
		return
	}
	takeoverUserID := stringParam(params, "takeoverUserId")
	list, err := contactTransferCustomerList(params["list"])
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "list 格式错误", nil)
		return
	}
	credential, ok := h.resolveCredential(w, r.Context(), corpID)
	if !ok {
		return
	}
	result := make([]map[string]any, 0, len(list))
	for _, item := range list {
		response, err := h.wecom.TransferCustomer(r.Context(), credential, []string{item.ContactWXID}, item.EmployeeWXID, takeoverUserID, "")
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		result = append(result, response)
		if numericMapInt(response, "errcode") != 0 {
			continue
		}
		status := 2
		if transferType == 1 {
			status = 1
		}
		name, _, err := h.store.WorkContactNameByExternalUserID(r.Context(), corpID, item.ContactWXID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		if err := h.store.CreateContactTransferLog(r.Context(), ContactTransferLogWrite{
			CorpID:             corpID,
			Status:             status,
			Type:               1,
			Name:               name,
			ContactID:          item.ContactWXID,
			HandoverEmployeeID: item.EmployeeWXID,
			TakeoverEmployeeID: takeoverUserID,
		}); err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
	}
	writeEnvelope(w, http.StatusOK, 200, "success", result)
}

func (h *ContactTransferHandler) TransferRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, ok := h.authorized(w, r)
	if !ok {
		return
	}
	corpID, ok := singleCorpID(w, loginInfo)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	chatIDs, err := stringListJSONParam(params, "list")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "list 格式错误", nil)
		return
	}
	takeoverUserID := stringParam(params, "takeoverUserId")
	credential, ok := h.resolveCredential(w, r.Context(), corpID)
	if !ok {
		return
	}
	failed, err := h.wecom.TransferGroupChat(r.Context(), credential, chatIDs, takeoverUserID)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "获取离职待分配群列表失败", nil)
		return
	}
	failedSet := map[string]struct{}{}
	for _, item := range failed {
		if chatID := strings.TrimSpace(fmt.Sprint(item["chat_id"])); chatID != "" {
			failedSet[chatID] = struct{}{}
		}
	}
	for _, chatID := range chatIDs {
		if _, exists := failedSet[chatID]; exists {
			continue
		}
		name, _, err := h.store.WorkRoomNameByWXChatID(r.Context(), corpID, chatID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		if err := h.store.CreateContactTransferLog(r.Context(), ContactTransferLogWrite{
			CorpID:             corpID,
			Status:             1,
			Type:               2,
			Name:               name,
			ContactID:          chatID,
			TakeoverEmployeeID: takeoverUserID,
		}); err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
	}
	writeEnvelope(w, http.StatusOK, 200, "success", failed)
}

func (h *ContactTransferHandler) contactFilterFromQuery(w http.ResponseWriter, r *http.Request, loginInfo LoginCorpInfo) (ContactTransferContactFilter, bool) {
	corpID, ok := singleCorpID(w, loginInfo)
	if !ok {
		return ContactTransferContactFilter{}, false
	}
	employeeIDs, err := jsonIntListOrEmpty(r.URL.Query().Get("employeeId"))
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "employeeId 格式错误", nil)
		return ContactTransferContactFilter{}, false
	}
	return ContactTransferContactFilter{
		CorpID:       corpID,
		ContactName:  strings.TrimSpace(r.URL.Query().Get("contactName")),
		EmployeeIDs:  employeeIDs,
		AddTimeStart: strings.TrimSpace(r.URL.Query().Get("addTimeStart")),
		AddTimeEnd:   strings.TrimSpace(r.URL.Query().Get("addTimeEnd")),
	}, true
}

func (h *ContactTransferHandler) unassignedFilterFromQuery(w http.ResponseWriter, r *http.Request, loginInfo LoginCorpInfo) (ContactTransferUnassignedFilter, bool) {
	corpID, ok := singleCorpID(w, loginInfo)
	if !ok {
		return ContactTransferUnassignedFilter{}, false
	}
	employeeIDs, err := jsonIntListOrEmpty(r.URL.Query().Get("employeeId"))
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "employeeId 格式错误", nil)
		return ContactTransferUnassignedFilter{}, false
	}
	return ContactTransferUnassignedFilter{
		CorpID:       corpID,
		ContactName:  strings.TrimSpace(r.URL.Query().Get("contactName")),
		EmployeeIDs:  employeeIDs,
		AddTimeStart: strings.TrimSpace(r.URL.Query().Get("addTimeStart")),
		AddTimeEnd:   strings.TrimSpace(r.URL.Query().Get("addTimeEnd")),
	}, true
}

func (h *ContactTransferHandler) authorized(w http.ResponseWriter, r *http.Request) (int, User, LoginCorpInfo, bool) {
	userID, user, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return 0, User{}, LoginCorpInfo{}, false
	}
	if err := h.authorize(r.Context(), r, userID, loginInfo); err != nil {
		writeAccessError(w, err)
		return 0, User{}, LoginCorpInfo{}, false
	}
	return userID, user, loginInfo, true
}

func (h *ContactTransferHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (int, User, LoginCorpInfo, bool) {
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
	return userID, user, loginInfo, true
}

func (h *ContactTransferHandler) authorize(ctx context.Context, r *http.Request, userID int, loginInfo LoginCorpInfo) error {
	if h.authorizer == nil {
		return nil
	}
	corpID := 0
	if len(loginInfo.CorpIDs) > 0 {
		corpID = loginInfo.CorpIDs[0]
	}
	_, err := h.authorizer.Resolve(ctx, userID, PermissionKeyFromRequest(r), corpID, loginInfo.WorkEmployeeID)
	return err
}

func (h *ContactTransferHandler) resolveCredential(w http.ResponseWriter, ctx context.Context, corpID int) (RoomWelcomeCorpCredential, bool) {
	credential, found, err := h.store.RoomWelcomeCorpCredentialByID(ctx, corpID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return RoomWelcomeCorpCredential{}, false
	}
	if !found || strings.TrimSpace(credential.WXCorpID) == "" || strings.TrimSpace(credential.ContactSecret) == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "企业微信客户联系配置不存在", nil)
		return RoomWelcomeCorpCredential{}, false
	}
	return credential, true
}

func singleCorpID(w http.ResponseWriter, loginInfo LoginCorpInfo) (int, bool) {
	if len(loginInfo.CorpIDs) != 1 || loginInfo.CorpIDs[0] <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "请先选择企业", nil)
		return 0, false
	}
	return loginInfo.CorpIDs[0], true
}

func contactTransferContactPayloads(items []ContactTransferContactItem, includeTransferState bool) []map[string]any {
	list := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payload := map[string]any{
			"contactId":    item.ContactID,
			"employeeId":   item.EmployeeID,
			"contactWxId":  item.ContactWXID,
			"employeeWxId": item.EmployeeWXID,
			"contactName":  item.ContactName,
			"nickName":     item.NickName,
			"corpName":     item.CorpName,
			"employeeName": item.EmployeeName,
			"tags":         item.Tags,
			"addTime":      item.AddTime,
			"lastMsgTime":  "",
			"addWay":       contactTransferAddWayText(item.AddWay),
		}
		if includeTransferState {
			payload["transferState"] = item.TransferState
		}
		list = append(list, payload)
	}
	return list
}

func jsonIntListOrEmpty(raw string) ([]int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []int{}, nil
	}
	var ints []int
	if err := json.Unmarshal([]byte(raw), &ints); err == nil {
		return uniquePositiveIntList(ints), nil
	}
	var stringsValue []string
	if err := json.Unmarshal([]byte(raw), &stringsValue); err == nil {
		ints = make([]int, 0, len(stringsValue))
		for _, item := range stringsValue {
			parsed, err := strconv.Atoi(strings.TrimSpace(item))
			if err != nil {
				return nil, err
			}
			ints = append(ints, parsed)
		}
		return uniquePositiveIntList(ints), nil
	}
	return parseIDList(raw), nil
}

func stringListJSONParam(params map[string]any, key string) ([]string, error) {
	value, ok := params[key]
	if !ok || value == nil {
		return []string{}, nil
	}
	switch typed := value.(type) {
	case string:
		raw := strings.TrimSpace(typed)
		if raw == "" {
			return []string{}, nil
		}
		var list []string
		if err := json.Unmarshal([]byte(raw), &list); err == nil {
			return uniqueNonEmptyStrings(list), nil
		}
		return uniqueNonEmptyStrings(strings.Split(raw, ",")), nil
	case []string:
		return uniqueNonEmptyStrings(typed), nil
	case []any:
		list := make([]string, 0, len(typed))
		for _, item := range typed {
			list = append(list, fmt.Sprint(item))
		}
		return uniqueNonEmptyStrings(list), nil
	default:
		return nil, fmt.Errorf("unsupported list type")
	}
}

type contactTransferCustomerTransferItem struct {
	ContactWXID  string
	EmployeeWXID string
}

func contactTransferCustomerList(value any) ([]contactTransferCustomerTransferItem, error) {
	if value == nil {
		return []contactTransferCustomerTransferItem{}, nil
	}
	var rawItems []map[string]any
	switch typed := value.(type) {
	case string:
		raw := strings.TrimSpace(typed)
		if raw == "" {
			return []contactTransferCustomerTransferItem{}, nil
		}
		if err := json.Unmarshal([]byte(raw), &rawItems); err != nil {
			return nil, err
		}
	case []any:
		rawItems = make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			itemMap, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("invalid item")
			}
			rawItems = append(rawItems, itemMap)
		}
	default:
		return nil, fmt.Errorf("unsupported list type")
	}
	list := make([]contactTransferCustomerTransferItem, 0, len(rawItems))
	for _, raw := range rawItems {
		item := contactTransferCustomerTransferItem{
			ContactWXID:  strings.TrimSpace(fmt.Sprint(raw["contactWxId"])),
			EmployeeWXID: strings.TrimSpace(fmt.Sprint(raw["employeeWxId"])),
		}
		if item.ContactWXID == "" || item.EmployeeWXID == "" {
			continue
		}
		list = append(list, item)
	}
	return list, nil
}

func queryOptionalInt(r *http.Request, key string) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return 0, nil
	}
	return strconv.Atoi(raw)
}

func numericMapInt(values map[string]any, key string) int {
	raw, ok := values[key]
	if !ok || raw == nil {
		return 0
	}
	switch typed := raw.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		integer, _ := typed.Int64()
		return int(integer)
	case string:
		integer, _ := strconv.Atoi(strings.TrimSpace(typed))
		return integer
	default:
		return 0
	}
}

func uniquePositiveIntList(values []int) []int {
	seen := map[int]struct{}{}
	result := make([]int, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func uniqueNonEmptyStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func contactTransferStateText(state int) string {
	switch state {
	case 1:
		return "接替完毕"
	case 2:
		return "等待接替"
	case 3:
		return "客户拒绝"
	case 4:
		return "接替成员客户达到上限"
	case 5:
		return "无接替记录"
	default:
		return ""
	}
}

func contactTransferAddWayText(addWay int) string {
	switch addWay {
	case 0:
		return "未知来源"
	case 1:
		return "扫描二维码"
	case 2:
		return "搜索手机号"
	case 3:
		return "名片分享"
	case 4:
		return "群聊"
	case 5:
		return "手机通讯录"
	case 6:
		return "微信联系人"
	case 7:
		return "来自微信的添加好友申请"
	case 8:
		return "安装第三方应用时自动添加的客服人员"
	case 9:
		return "搜索邮箱"
	case 201:
		return "内部成员共享"
	case 202:
		return "管理员/负责人分配"
	case 1001:
		return "渠道活码"
	case 1002:
		return "自动拉群"
	case 1003:
		return "裂变引流"
	default:
		return ""
	}
}
