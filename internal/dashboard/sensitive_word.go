package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type SensitiveWordFilter struct {
	CorpID   int
	GroupID  int
	KeyWords string
	Page     int
	PerPage  int
}

type SensitiveWordItem struct {
	ID          int
	CorpID      int
	GroupID     int
	GroupName   string
	Name        string
	Status      int
	EmployeeNum int
	ContactNum  int
	CreatedAt   string
	Version     string
}

type SensitiveWordPage struct {
	Items     []SensitiveWordItem
	Total     int
	TotalPage int
	PerPage   int
}

type SensitiveWordGroup struct {
	ID      int
	Name    string
	Version string
}

const (
	SensitiveWordMutationCreateWords = "create_words"
	SensitiveWordMutationSetStatus   = "set_status"
	SensitiveWordMutationMoveWord    = "move_word"
	SensitiveWordMutationDeleteWord  = "delete_word"
	SensitiveWordMutationCreateGroup = "create_group"
	SensitiveWordMutationRenameGroup = "rename_group"
)

type SensitiveWordMutation struct {
	Action         string
	TenantID       int
	CorpID         int
	ActorUserID    int
	WordID         int
	GroupID        int
	Status         int
	Name           string
	Names          []string
	Version        string
	IdempotencyKey string
}

type SensitiveWordMutationResult struct {
	Version    string
	Idempotent bool
}

type SensitiveWordOperationError struct {
	Status  int
	Message string
}

func (e *SensitiveWordOperationError) Error() string { return e.Message }

func NewSensitiveWordConflict(message string) error {
	return &SensitiveWordOperationError{Status: http.StatusConflict, Message: message}
}

type SensitiveWordsMonitorFilter struct {
	CorpID             int
	EmployeeIDs        []int
	WorkRoomID         int
	IntelligentGroupID int
	TriggerStart       string
	TriggerEnd         string
	Page               int
	PerPage            int
}

type SensitiveWordsMonitorItem struct {
	ID                int
	SensitiveWordID   int
	SensitiveWordName string
	Source            int
	TriggerName       string
	TriggerScenario   string
	TriggerTime       string
}

type SensitiveWordsMonitorPage struct {
	Items     []SensitiveWordsMonitorItem
	Total     int
	TotalPage int
	PerPage   int
}

type SensitiveWordsMonitorMessage struct {
	Sender     string
	MsgType    int
	SendTime   string
	IsTrigger  int
	MsgContent any
}

type SensitiveWordStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	SensitiveWordPage(ctx context.Context, filter SensitiveWordFilter) (SensitiveWordPage, error)
	SensitiveWordMutationReplay(ctx context.Context, mutation SensitiveWordMutation) (SensitiveWordMutationResult, bool, error)
	MutateSensitiveWords(ctx context.Context, mutation SensitiveWordMutation) (SensitiveWordMutationResult, error)
	SensitiveWordGroups(ctx context.Context, corpID int) ([]SensitiveWordGroup, error)
	SensitiveWordsMonitorPage(ctx context.Context, filter SensitiveWordsMonitorFilter) (SensitiveWordsMonitorPage, error)
	SensitiveWordsMonitorMessages(ctx context.Context, corpID int, monitorID int) ([]SensitiveWordsMonitorMessage, bool, error)
}

type SensitiveWordHandler struct {
	store      SensitiveWordStore
	cache      LoginCache
	resolver   UserIDResolver
	authorizer CorpAdminAuthorizer
}

type sensitiveWordBadRequest string

func (e sensitiveWordBadRequest) Error() string {
	return string(e)
}

func badRequestError(message string) error {
	return sensitiveWordBadRequest(message)
}

func isBadRequestError(err error) bool {
	var target sensitiveWordBadRequest
	return errors.As(err, &target)
}

func NewSensitiveWordHandler(store SensitiveWordStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer) *SensitiveWordHandler {
	return &SensitiveWordHandler{store: store, cache: cache, resolver: resolver, authorizer: authorizer}
}

func (h *SensitiveWordHandler) Index(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/sensitiveWord/index#get")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	page, err := h.store.SensitiveWordPage(r.Context(), SensitiveWordFilter{
		CorpID:   corpID,
		GroupID:  positiveQueryInt(r, "groupId", 0),
		KeyWords: strings.TrimSpace(r.URL.Query().Get("keyWords")),
		Page:     positiveQueryInt(r, "page", 1),
		PerPage:  positiveQueryInt(r, "perPage", 10),
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, sensitiveWordPayload(item))
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"page": map[string]any{"perPage": page.PerPage, "total": page.Total, "totalPage": page.TotalPage},
		"list": list,
	})
}

func (h *SensitiveWordHandler) Store(w http.ResponseWriter, r *http.Request) {
	h.writeWordMutation(w, r, http.MethodPost, "/dashboard/sensitiveWord/store#post", func(ctx context.Context, user User, corpID int, params map[string]any, version string, idempotencyKey string) (SensitiveWordMutationResult, error) {
		groupID, _, err := intParam(params, "groupId")
		if err != nil || groupID <= 0 {
			return SensitiveWordMutationResult{}, badRequestError("groupId required")
		}
		names := splitSensitiveNames(stringParam(params, "name"))
		if len(names) == 0 {
			return SensitiveWordMutationResult{}, badRequestError("name required")
		}
		mutation := SensitiveWordMutation{
			Action: SensitiveWordMutationCreateWords, TenantID: user.TenantID, CorpID: corpID, ActorUserID: user.ID,
			GroupID: groupID, Names: names, Version: version, IdempotencyKey: idempotencyKey,
		}
		if replay, found, err := h.store.SensitiveWordMutationReplay(ctx, mutation); err != nil {
			return SensitiveWordMutationResult{}, err
		} else if found {
			return replay, nil
		}
		if err := requireSaaSQuota(ctx, h.store, user.TenantID, SaaSMetricSensitiveWords, int64(len(names))); err != nil {
			return SensitiveWordMutationResult{}, err
		}
		result, err := h.store.MutateSensitiveWords(ctx, mutation)
		if err != nil {
			return SensitiveWordMutationResult{}, err
		}
		if err := refreshSaaSUsageCounter(ctx, h.store, user.TenantID, SaaSMetricSensitiveWords); err != nil {
			return SensitiveWordMutationResult{}, err
		}
		return result, nil
	})
}

func (h *SensitiveWordHandler) Destroy(w http.ResponseWriter, r *http.Request) {
	h.writeWordMutation(w, r, http.MethodDelete, "/dashboard/sensitiveWord/destroy#delete", func(ctx context.Context, user User, corpID int, params map[string]any, version string, idempotencyKey string) (SensitiveWordMutationResult, error) {
		wordID, _, err := firstPositiveIntParam(params, "sensitiveWordId", "id")
		if err != nil || wordID <= 0 {
			return SensitiveWordMutationResult{}, badRequestError("sensitiveWordId required")
		}
		confirmed, found, err := boolParam(params, "confirmed")
		if err != nil || !found || !confirmed {
			return SensitiveWordMutationResult{}, badRequestError("delete confirmation required")
		}
		result, err := h.store.MutateSensitiveWords(ctx, SensitiveWordMutation{
			Action: SensitiveWordMutationDeleteWord, TenantID: user.TenantID, CorpID: corpID, ActorUserID: user.ID,
			WordID: wordID, Version: version, IdempotencyKey: idempotencyKey,
		})
		if err != nil {
			return SensitiveWordMutationResult{}, err
		}
		if err := refreshSaaSUsageCounter(ctx, h.store, user.TenantID, SaaSMetricSensitiveWords); err != nil {
			return SensitiveWordMutationResult{}, err
		}
		return result, nil
	})
}

func (h *SensitiveWordHandler) StatusUpdate(w http.ResponseWriter, r *http.Request) {
	h.writeWordMutation(w, r, http.MethodPut, "/dashboard/sensitiveWord/statusUpdate#put", func(ctx context.Context, user User, corpID int, params map[string]any, version string, idempotencyKey string) (SensitiveWordMutationResult, error) {
		wordID, _, err := firstPositiveIntParam(params, "sensitiveWordId", "id")
		if err != nil || wordID <= 0 {
			return SensitiveWordMutationResult{}, badRequestError("sensitiveWordId required")
		}
		status, _, err := intParam(params, "status")
		if err != nil || (status != 1 && status != 2) {
			return SensitiveWordMutationResult{}, badRequestError("status must be 1 or 2")
		}
		return h.store.MutateSensitiveWords(ctx, SensitiveWordMutation{
			Action: SensitiveWordMutationSetStatus, TenantID: user.TenantID, CorpID: corpID, ActorUserID: user.ID,
			WordID: wordID, Status: status, Version: version, IdempotencyKey: idempotencyKey,
		})
	})
}

func (h *SensitiveWordHandler) Move(w http.ResponseWriter, r *http.Request) {
	h.writeWordMutation(w, r, http.MethodPut, "/dashboard/sensitiveWord/move#put", func(ctx context.Context, user User, corpID int, params map[string]any, version string, idempotencyKey string) (SensitiveWordMutationResult, error) {
		wordID, _, err := firstPositiveIntParam(params, "sensitiveWordId", "id")
		if err != nil || wordID <= 0 {
			return SensitiveWordMutationResult{}, badRequestError("sensitiveWordId required")
		}
		groupID, _, err := intParam(params, "groupId")
		if err != nil || groupID <= 0 {
			return SensitiveWordMutationResult{}, badRequestError("groupId required")
		}
		return h.store.MutateSensitiveWords(ctx, SensitiveWordMutation{
			Action: SensitiveWordMutationMoveWord, TenantID: user.TenantID, CorpID: corpID, ActorUserID: user.ID,
			WordID: wordID, GroupID: groupID, Version: version, IdempotencyKey: idempotencyKey,
		})
	})
}

func (h *SensitiveWordHandler) GroupSelect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/sensitiveWordGroup/select#get")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	groups, err := h.store.SensitiveWordGroups(r.Context(), corpID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	payload := make([]map[string]any, 0, len(groups))
	for _, group := range groups {
		payload = append(payload, map[string]any{"groupId": group.ID, "name": group.Name, "version": group.Version})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *SensitiveWordHandler) GroupStore(w http.ResponseWriter, r *http.Request) {
	h.writeWordMutation(w, r, http.MethodPost, "/dashboard/sensitiveWordGroup/store#post", func(ctx context.Context, user User, corpID int, params map[string]any, version string, idempotencyKey string) (SensitiveWordMutationResult, error) {
		names := splitSensitiveNames(stringParam(params, "name"))
		if len(names) == 0 {
			return SensitiveWordMutationResult{}, badRequestError("name required")
		}
		return h.store.MutateSensitiveWords(ctx, SensitiveWordMutation{
			Action: SensitiveWordMutationCreateGroup, TenantID: user.TenantID, CorpID: corpID, ActorUserID: user.ID,
			Names: names, Version: version, IdempotencyKey: idempotencyKey,
		})
	})
}

func (h *SensitiveWordHandler) GroupUpdate(w http.ResponseWriter, r *http.Request) {
	h.writeWordMutation(w, r, http.MethodPut, "/dashboard/sensitiveWordGroup/update#put", func(ctx context.Context, user User, corpID int, params map[string]any, version string, idempotencyKey string) (SensitiveWordMutationResult, error) {
		groupID, _, err := firstPositiveIntParam(params, "groupId", "id")
		if err != nil || groupID <= 0 {
			return SensitiveWordMutationResult{}, badRequestError("groupId required")
		}
		name := strings.TrimSpace(stringParam(params, "name"))
		if name == "" {
			return SensitiveWordMutationResult{}, badRequestError("name required")
		}
		return h.store.MutateSensitiveWords(ctx, SensitiveWordMutation{
			Action: SensitiveWordMutationRenameGroup, TenantID: user.TenantID, CorpID: corpID, ActorUserID: user.ID,
			GroupID: groupID, Name: name, Version: version, IdempotencyKey: idempotencyKey,
		})
	})
}

func (h *SensitiveWordHandler) MonitorIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/sensitiveWordsMonitor/index#get")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	employeeIDs := parseQueryIntList(r, "employeeId")
	page, err := h.store.SensitiveWordsMonitorPage(r.Context(), SensitiveWordsMonitorFilter{
		CorpID:             corpID,
		EmployeeIDs:        employeeIDs,
		WorkRoomID:         positiveQueryInt(r, "workRoomId", 0),
		IntelligentGroupID: positiveQueryInt(r, "intelligentGroupId", 0),
		TriggerStart:       strings.TrimSpace(r.URL.Query().Get("triggerStart")),
		TriggerEnd:         strings.TrimSpace(r.URL.Query().Get("triggerEnd")),
		Page:               positiveQueryInt(r, "page", 1),
		PerPage:            positiveQueryInt(r, "perPage", 10),
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, sensitiveWordsMonitorPayload(item))
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"page": map[string]any{"perPage": page.PerPage, "total": page.Total, "totalPage": page.TotalPage},
		"list": list,
	})
}

func (h *SensitiveWordHandler) MonitorShow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/sensitiveWordsMonitor/show#get")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	monitorID := positiveQueryInt(r, "sensitiveWordsMonitorId", 0)
	if monitorID == 0 {
		monitorID = positiveQueryInt(r, "id", 0)
	}
	if monitorID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "sensitiveWordsMonitorId required", nil)
		return
	}
	messages, found, err := h.store.SensitiveWordsMonitorMessages(r.Context(), corpID, monitorID)
	if err != nil {
		writeEnvelope(w, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "扫描结果暂时无法连接，请稍后重试", nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "not found", nil)
		return
	}
	payload := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		payload = append(payload, map[string]any{
			"sender":     message.Sender,
			"msgType":    message.MsgType,
			"sendTime":   message.SendTime,
			"isTrigger":  message.IsTrigger,
			"msgContent": message.MsgContent,
		})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *SensitiveWordHandler) writeWordMutation(w http.ResponseWriter, r *http.Request, method string, permissionKey string, action func(context.Context, User, int, map[string]any, string, string) (SensitiveWordMutationResult, error)) {
	if r.Method != method {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, user, loginInfo, _, ok := h.resolveAuthorized(w, r, permissionKey)
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	version := strings.TrimSpace(stringParam(params, "version"))
	idempotencyKey := strings.TrimSpace(stringParam(params, "idempotencyKey"))
	if version == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "version required", nil)
		return
	}
	if idempotencyKey == "" || len(idempotencyKey) > 128 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "idempotencyKey required", nil)
		return
	}
	result, err := action(r.Context(), user, corpID, params, version, idempotencyKey)
	if err != nil {
		if isBadRequestError(err) {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
			return
		}
		if writeSaaSQuotaError(w, err) {
			return
		}
		var operationErr *SensitiveWordOperationError
		if errors.As(err, &operationErr) {
			writeEnvelope(w, operationErr.Status, operationErr.Status, operationErr.Message, nil)
			return
		}
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{"version": result.Version, "idempotent": result.Idempotent})
}

func (h *SensitiveWordHandler) resolveAuthorized(w http.ResponseWriter, r *http.Request, permissionKey string) (int, User, LoginCorpInfo, AccessContext, bool) {
	userID, user, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return 0, User{}, LoginCorpInfo{}, AccessContext{}, false
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return 0, User{}, LoginCorpInfo{}, AccessContext{}, false
	}
	employeeID := loginInfo.WorkEmployeeID
	if employeeID <= 0 {
		resolved, err := h.store.EmployeeIDByUserCorp(r.Context(), userID, corpID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return 0, User{}, LoginCorpInfo{}, AccessContext{}, false
		}
		employeeID = resolved
	}
	access := AccessContext{User: user, PermissionKey: permissionKey, CorpID: corpID, WorkEmployeeID: employeeID, DataPermission: DataPermissionAll}
	if h.authorizer != nil {
		resolved, err := h.authorizer.Resolve(r.Context(), userID, permissionKey, corpID, employeeID)
		if err != nil {
			writeAccessError(w, err)
			return 0, User{}, LoginCorpInfo{}, AccessContext{}, false
		}
		access = resolved
	}
	return userID, user, loginInfo, access, true
}

func (h *SensitiveWordHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (int, User, LoginCorpInfo, bool) {
	if h.resolver == nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "user resolver not configured", nil)
		return 0, User{}, LoginCorpInfo{}, false
	}
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

func sensitiveWordPayload(item SensitiveWordItem) map[string]any {
	return map[string]any{
		"sensitiveWordId": item.ID,
		"groupId":         item.GroupID,
		"groupName":       item.GroupName,
		"name":            item.Name,
		"employeeNum":     item.EmployeeNum,
		"contactNum":      item.ContactNum,
		"createdAt":       item.CreatedAt,
		"status":          item.Status,
		"version":         item.Version,
	}
}

func sensitiveWordsMonitorPayload(item SensitiveWordsMonitorItem) map[string]any {
	sourceText := "员工"
	if item.Source == 1 {
		sourceText = "客户"
	}
	return map[string]any{
		"sensitiveWordMonitorId":  item.ID,
		"sensitiveWordsMonitorId": item.ID,
		"sensitiveWordName":       item.SensitiveWordName,
		"source":                  item.Source,
		"sourceText":              sourceText,
		"triggerName":             item.TriggerName,
		"triggerScenario":         item.TriggerScenario,
		"triggerTime":             item.TriggerTime,
	}
}

func splitSensitiveNames(raw string) []string {
	replacer := strings.NewReplacer("，", ",", "\n", ",", "\r", ",", "、", ",", ";", ",", "；", ",")
	parts := strings.Split(replacer.Replace(raw), ",")
	seen := map[string]struct{}{}
	names := make([]string, 0, len(parts))
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}
		if len([]rune(name)) > 64 {
			name = string([]rune(name)[:64])
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return names
}

func firstPositiveIntParam(params map[string]any, keys ...string) (int, bool, error) {
	for _, key := range keys {
		value, ok, err := intParam(params, key)
		if err != nil {
			return 0, ok, err
		}
		if ok && value > 0 {
			return value, true, nil
		}
	}
	return 0, false, nil
}

func parseQueryIntList(r *http.Request, key string) []int {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return nil
	}
	params := map[string]any{key: value}
	result, err := intSliceParam(params, key)
	if err != nil {
		return nil
	}
	return result
}

func monitorContentFromJSON(raw string) any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]any{"content": ""}
	}
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err == nil {
		return value
	}
	return map[string]any{"content": raw}
}
