package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

const (
	greetingCreateEvent = 300
	greetingUpdateEvent = 301
)

type GreetingFilter struct {
	CorpID               int
	Page                 int
	PerPage              int
	RestrictOperationIDs bool
	OperationIDs         []int
}

type GreetingItem struct {
	ID          int
	CorpID      int
	Type        string
	Words       string
	MediumID    int
	RangeType   int
	EmployeeIDs []int
	CreatedAt   string
}

type GreetingPage struct {
	Items     []GreetingItem
	Total     int
	TotalPage int
	PerPage   int
}

type GreetingEmployee struct {
	ID       int
	Name     string
	Avatar   string
	WXUserID string
}

type GreetingMedium struct {
	ID      int
	Type    int
	Content map[string]any
}

type GreetingWrite struct {
	CorpID      int
	Type        string
	Words       string
	MediumID    int
	RangeType   int
	EmployeeIDs []int
}

type GreetingStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	GreetingPage(ctx context.Context, filter GreetingFilter) (GreetingPage, error)
	GreetingsByCorp(ctx context.Context, corpID int) ([]GreetingItem, error)
	GreetingByID(ctx context.Context, greetingID int) (GreetingItem, bool, error)
	GreetingEmployeesByIDs(ctx context.Context, employeeIDs []int) (map[int]GreetingEmployee, error)
	GreetingMediaByIDs(ctx context.Context, mediumIDs []int) (map[int]GreetingMedium, error)
	CreateGreetingWithLog(ctx context.Context, values GreetingWrite, operationID int) (int, error)
	UpdateGreetingWithLog(ctx context.Context, greetingID int, values GreetingWrite, operationID int) (bool, error)
	DeleteGreeting(ctx context.Context, greetingID int) (bool, error)
}

type GreetingHandler struct {
	store      GreetingStore
	cache      LoginCache
	resolver   UserIDResolver
	authorizer CorpAdminAuthorizer
	apiBaseURL string
}

func NewGreetingHandler(store GreetingStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer, apiBaseURL string) *GreetingHandler {
	return &GreetingHandler{store: store, cache: cache, resolver: resolver, authorizer: authorizer, apiBaseURL: strings.TrimRight(apiBaseURL, "/")}
}

func (h *GreetingHandler) Index(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, access, ok := h.resolveAuthorized(w, r, "/dashboard/greeting/index#get")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	filter := GreetingFilter{
		CorpID:  corpID,
		Page:    positiveQueryInt(r, "page", 1),
		PerPage: positiveQueryInt(r, "perPage", 10),
	}
	if access.DataPermission != DataPermissionAll {
		filter.RestrictOperationIDs = true
		filter.OperationIDs = append([]int{}, access.DeptEmployeeIDs...)
	}
	page, err := h.store.GreetingPage(r.Context(), filter)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	allGreetings, err := h.store.GreetingsByCorp(r.Context(), corpID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	employeeNames, media, err := h.greetingLookups(r.Context(), page.Items)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	hadGeneral, hadEmployees := greetingHadState(allGreetings)
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, h.greetingListPayload(item, employeeNames, media))
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"page": map[string]any{
			"perPage":   page.PerPage,
			"total":     page.Total,
			"totalPage": page.TotalPage,
		},
		"hadGeneral":   hadGeneral,
		"hadEmployees": hadEmployees,
		"list":         list,
	})
}

func (h *GreetingHandler) Show(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/greeting/show#get")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	greetingID, err := positiveQueryIntRequired(r, "greetingId")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "欢迎语ID 必填", nil)
		return
	}
	greeting, found, err := h.store.GreetingByID(r.Context(), greetingID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found || greeting.CorpID != corpID {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "此欢迎语不存在", nil)
		return
	}
	employees, err := h.store.GreetingEmployeesByIDs(r.Context(), greeting.EmployeeIDs)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	media, err := h.store.GreetingMediaByIDs(r.Context(), []int{greeting.MediumID})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"greetingId":    greeting.ID,
		"rangeType":     greeting.RangeType,
		"employees":     greetingEmployeePayloads(greeting.EmployeeIDs, employees, true),
		"words":         greeting.Words,
		"mediumId":      greeting.MediumID,
		"mediumContent": h.greetingMediumContent(greeting.MediumID, media),
	})
}

func (h *GreetingHandler) Store(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, access, ok := h.resolveAuthorized(w, r, "/dashboard/greeting/store#post")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	values, ok := h.parseGreetingWrite(w, r, corpID)
	if !ok {
		return
	}
	if _, err := h.store.CreateGreetingWithLog(r.Context(), values, access.WorkEmployeeID); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "欢迎语创建失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *GreetingHandler) Update(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, access, ok := h.resolveAuthorized(w, r, "/dashboard/greeting/update#put")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	greetingID, okInt, err := intParam(params, "greetingId")
	if err != nil || !okInt || greetingID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "欢迎语ID 必填", nil)
		return
	}
	values, ok := h.greetingWriteFromParams(w, params, corpID)
	if !ok {
		return
	}
	updated, err := h.store.UpdateGreetingWithLog(r.Context(), greetingID, values, access.WorkEmployeeID)
	if err != nil || !updated {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "欢迎语更新失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *GreetingHandler) Destroy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/greeting/destroy#delete")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	greetingID, okInt, err := intParam(params, "greetingId")
	if err != nil || !okInt || greetingID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "欢迎语ID 必填", nil)
		return
	}
	greeting, found, err := h.store.GreetingByID(r.Context(), greetingID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "此欢迎语不存在，不可操作", nil)
		return
	}
	if greeting.CorpID != corpID {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "此欢迎语不归属当前企业，不可操作", nil)
		return
	}
	deleted, err := h.store.DeleteGreeting(r.Context(), greetingID)
	if err != nil || !deleted {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "欢迎语删除失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *GreetingHandler) parseGreetingWrite(w http.ResponseWriter, r *http.Request, corpID int) (GreetingWrite, bool) {
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return GreetingWrite{}, false
	}
	return h.greetingWriteFromParams(w, params, corpID)
}

func (h *GreetingHandler) greetingWriteFromParams(w http.ResponseWriter, params map[string]any, corpID int) (GreetingWrite, bool) {
	rangeType, okInt, err := intParam(params, "rangeType")
	if err != nil || !okInt || (rangeType != 1 && rangeType != 2) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "适用成员类型 必填", nil)
		return GreetingWrite{}, false
	}
	typeRaw := stringParam(params, "type")
	normalizedType := normalizeGreetingType(typeRaw)
	if normalizedType == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "欢迎语类型 必填", nil)
		return GreetingWrite{}, false
	}
	mediumID, okInt, err := intParam(params, "mediumId")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "素材库ID 必需为整数", nil)
		return GreetingWrite{}, false
	}
	if !okInt {
		mediumID = 0
	}
	if mediumID < 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "素材库ID 不可小于1", nil)
		return GreetingWrite{}, false
	}
	return GreetingWrite{
		CorpID:      corpID,
		Type:        normalizedType,
		Words:       stringParam(params, "words"),
		MediumID:    mediumID,
		RangeType:   rangeType,
		EmployeeIDs: intSliceOrCSV(params, "employees"),
	}, true
}

func (h *GreetingHandler) resolveAuthorized(w http.ResponseWriter, r *http.Request, permissionKey string) (int, User, LoginCorpInfo, AccessContext, bool) {
	userID, user, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return 0, User{}, LoginCorpInfo{}, AccessContext{}, false
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return 0, User{}, LoginCorpInfo{}, AccessContext{}, false
	}
	access := AccessContext{User: user, CorpID: corpID, WorkEmployeeID: loginInfo.WorkEmployeeID, DataPermission: DataPermissionAll}
	if h.authorizer != nil {
		var err error
		access, err = h.authorizer.Resolve(r.Context(), userID, permissionKey, corpID, loginInfo.WorkEmployeeID)
		if err != nil {
			writeAccessError(w, err)
			return 0, User{}, LoginCorpInfo{}, AccessContext{}, false
		}
	}
	return userID, user, loginInfo, access, true
}

func (h *GreetingHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (int, User, LoginCorpInfo, bool) {
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

func (h *GreetingHandler) greetingLookups(ctx context.Context, greetings []GreetingItem) (map[int]string, map[int]GreetingMedium, error) {
	employeeIDs := []int{}
	mediumIDs := []int{}
	for _, greeting := range greetings {
		if greeting.RangeType == 2 {
			employeeIDs = append(employeeIDs, greeting.EmployeeIDs...)
		}
		if greeting.MediumID > 0 {
			mediumIDs = append(mediumIDs, greeting.MediumID)
		}
	}
	employees, err := h.store.GreetingEmployeesByIDs(ctx, uniquePositiveIntsLocal(employeeIDs))
	if err != nil {
		return nil, nil, err
	}
	names := make(map[int]string, len(employees))
	for id, employee := range employees {
		names[id] = employee.Name
	}
	media, err := h.store.GreetingMediaByIDs(ctx, uniquePositiveIntsLocal(mediumIDs))
	if err != nil {
		return nil, nil, err
	}
	return names, media, nil
}

func (h *GreetingHandler) greetingListPayload(greeting GreetingItem, employeeNames map[int]string, media map[int]GreetingMedium) map[string]any {
	return map[string]any{
		"greetingId":    greeting.ID,
		"typeText":      greetingTypeText(greeting.Type),
		"rangeType":     greeting.RangeType,
		"rangeTypeText": greetingRangeTypeText(greeting.RangeType),
		"employees":     greetingListEmployees(greeting, employeeNames),
		"words":         greeting.Words,
		"mediumId":      greeting.MediumID,
		"mediumContent": h.greetingMediumContent(greeting.MediumID, media),
		"createdAt":     greeting.CreatedAt,
	}
}

func (h *GreetingHandler) greetingMediumContent(mediumID int, media map[int]GreetingMedium) map[string]any {
	medium, ok := media[mediumID]
	if !ok || mediumID == 0 {
		return map[string]any{}
	}
	content := cloneMap(medium.Content)
	addMediumFullPath(content, medium.Type, h.fileFullURL)
	return content
}

func (h *GreetingHandler) fileFullURL(path string) string {
	if path == "" || strings.Contains(path, "http") || h.apiBaseURL == "" {
		return path
	}
	return h.apiBaseURL + "/static/" + strings.TrimLeft(path, "/")
}

func greetingHadState(greetings []GreetingItem) (int, []int) {
	hadGeneral := 0
	ids := []int{}
	for _, greeting := range greetings {
		if greeting.RangeType == 1 {
			hadGeneral = 1
		}
		ids = append(ids, greeting.EmployeeIDs...)
	}
	return hadGeneral, uniquePositiveIntsLocal(ids)
}

func greetingListEmployees(greeting GreetingItem, employeeNames map[int]string) []any {
	if greeting.RangeType == 1 {
		return []any{greetingRangeTypeText(1)}
	}
	out := make([]any, 0, len(greeting.EmployeeIDs))
	for _, id := range greeting.EmployeeIDs {
		if name := employeeNames[id]; name != "" {
			out = append(out, name)
		}
	}
	return out
}

func greetingEmployeePayloads(ids []int, employees map[int]GreetingEmployee, selected bool) []map[string]any {
	out := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		employee, ok := employees[id]
		if !ok {
			continue
		}
		out = append(out, map[string]any{
			"id":         employee.ID,
			"employeeId": employee.ID,
			"name":       employee.Name,
			"avatar":     employee.Avatar,
			"wxUserId":   employee.WXUserID,
			"select":     selected,
		})
	}
	return out
}

func normalizeGreetingType(raw string) string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		value := strings.Trim(strings.TrimSpace(part), "-")
		if value == "" {
			continue
		}
		if _, err := strconv.Atoi(value); err != nil {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	if len(values) == 0 {
		return ""
	}
	return "-" + strings.Join(values, "-") + "-"
}

func greetingTypeText(raw string) string {
	parts := strings.Split(strings.Trim(raw, "-"), "-")
	text := make([]string, 0, len(parts))
	for _, part := range parts {
		value, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			continue
		}
		if label := mediumTypeText(value); label != "" {
			text = append(text, label)
		}
	}
	return strings.Join(text, "+")
}

func greetingRangeTypeText(value int) string {
	if value == 2 {
		return "指定企业成员"
	}
	return "全体成员"
}

func intSliceOrCSV(params map[string]any, key string) []int {
	values, err := intSliceParam(params, key)
	if err != nil {
		return []int{}
	}
	return values
}

func uniquePositiveIntsLocal(values []int) []int {
	seen := map[int]struct{}{}
	out := make([]int, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Ints(out)
	return out
}

func greetingEmployeesJSON(employeeIDs []int) string {
	employeeIDs = uniquePositiveIntsLocal(employeeIDs)
	raw, _ := json.Marshal(employeeIDs)
	return string(raw)
}
