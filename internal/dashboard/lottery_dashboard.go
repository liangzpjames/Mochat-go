package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type LotteryFilter struct {
	CorpID  int
	Name    string
	Page    int
	PerPage int
}

type LotteryItem struct {
	ID             int
	Name           string
	Description    string
	Type           string
	TimeType       int
	StartTime      string
	EndTime        string
	ContactTagsRaw string
	TenantID       int
	CorpID         int
	CreateUserID   int
	CreateUserName string
	ContactNum     int
	WinNum         int
	CreatedAt      string
	UpdatedAt      string
}

type LotteryPrizeItem struct {
	ID          int
	LotteryID   int
	PrizeSetRaw string
	IsShow      int
	ExchangeRaw string
	DrawSetRaw  string
	WinSetRaw   string
	CorpCardRaw string
	CreatedAt   string
	UpdatedAt   string
}

type LotteryPage struct {
	Items     []LotteryItem
	Total     int
	TotalPage int
	Page      int
	PerPage   int
}

type LotteryWrite struct {
	CorpID         int
	CreateUserID   int
	Name           string
	HasName        bool
	Description    string
	HasDescription bool
	Type           string
	HasType        bool
	TimeType       int
	HasTimeType    bool
	StartTime      string
	HasStartTime   bool
	EndTime        string
	HasEndTime     bool
	ContactTagsRaw string
	HasContactTags bool
	PrizeSetRaw    string
	HasPrizeSet    bool
	IsShow         int
	HasIsShow      bool
	ExchangeSetRaw string
	HasExchangeSet bool
	DrawSetRaw     string
	HasDrawSet     bool
	WinSetRaw      string
	HasWinSet      bool
	CorpCardRaw    string
	HasCorpCard    bool
}

type LotteryContactFilter struct {
	CorpID    int
	LotteryID int
	Name      string
	Status    int
	WriteOff  int
	Page      int
	PerPage   int
}

type LotteryContactItem struct {
	ID             int
	LotteryID      int
	UnionID        string
	ContactID      int
	Nickname       string
	Avatar         string
	EmployeeIDsRaw string
	City           string
	Source         string
	Grade          int
	ContactTagsRaw string
	DrawNum        int
	WinNum         int
	Status         int
	WriteOff       int
	PrizeName      string
	ReceiveStatus  int
	ReceiveType    int
	ReceiveCode    string
	ReceiveQR      string
	CreatedAt      string
	UpdatedAt      string
}

type LotteryContactPage struct {
	Items     []LotteryContactItem
	Total     int
	TotalPage int
	Page      int
	PerPage   int
}

type LotteryStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	LotteryPage(ctx context.Context, filter LotteryFilter) (LotteryPage, error)
	LotteryByID(ctx context.Context, corpID int, id int) (LotteryItem, LotteryPrizeItem, bool, error)
	CreateLottery(ctx context.Context, values LotteryWrite) (int, error)
	UpdateLottery(ctx context.Context, corpID int, id int, values LotteryWrite) (bool, error)
	DeleteLottery(ctx context.Context, corpID int, id int) (bool, error)
	LotteryContactPage(ctx context.Context, filter LotteryContactFilter) (LotteryContactPage, error)
	WriteOffLotteryContact(ctx context.Context, corpID int, lotteryID int, contactID int) (bool, error)
	BatchTagLotteryContacts(ctx context.Context, corpID int, lotteryID int, contactIDs []int, tagIDs []int) (int, error)
}

type LotteryHandler struct {
	store            LotteryStore
	cache            LoginCache
	resolver         UserIDResolver
	authorizer       CorpAdminAuthorizer
	operationBaseURL string
}

func NewLotteryHandler(store LotteryStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer, operationBaseURL string) *LotteryHandler {
	return &LotteryHandler{store: store, cache: cache, resolver: resolver, authorizer: authorizer, operationBaseURL: strings.TrimRight(strings.TrimSpace(operationBaseURL), "/")}
}

func (h *LotteryHandler) Index(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/lottery/index#get")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	page, err := h.store.LotteryPage(r.Context(), LotteryFilter{
		CorpID:  corpID,
		Name:    lotteryQueryString(r, "name", "keyword"),
		Page:    positiveQueryInt(r, "page", 1),
		PerPage: positiveQueryInt(r, "perPage", 15),
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, lotteryPayload(item, LotteryPrizeItem{}, false, h.shareURL(item.ID)))
	}
	payload := contactBatchAddPagination(r, page.Page, page.PerPage, page.Total, list)
	payload["list"] = list
	payload["page"] = map[string]any{"perPage": page.PerPage, "total": page.Total, "totalPage": page.TotalPage}
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *LotteryHandler) Store(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, user, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/lottery/store#post")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, 400, err.Error(), nil)
		return
	}
	values, err := lotteryWriteFromParams(params, corpID, userID, true)
	if err != nil {
		status := http.StatusInternalServerError
		code := http.StatusInternalServerError
		if isBadRequestError(err) {
			status = http.StatusBadRequest
			code = 400
		}
		writeEnvelope(w, status, code, err.Error(), nil)
		return
	}
	if !enforceSaaSQuota(r.Context(), w, h.store, user.TenantID, SaaSMetricLotteries, 1) {
		return
	}
	id, err := h.store.CreateLottery(r.Context(), values)
	if err != nil {
		if writeSaaSQuotaError(w, err) {
			return
		}
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if err := refreshSaaSUsageCounter(r.Context(), h.store, user.TenantID, SaaSMetricLotteries); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{id})
}

func (h *LotteryHandler) Update(w http.ResponseWriter, r *http.Request) {
	h.writeMutation(w, r, http.MethodPut, "/dashboard/lottery/update#put", func(ctx context.Context, userID int, corpID int, params map[string]any) (any, error) {
		id, err := lotteryIDFromParams(params)
		if err != nil || id <= 0 {
			return nil, badRequestError("lotteryId required")
		}
		values, err := lotteryWriteFromParams(params, corpID, userID, false)
		if err != nil {
			return nil, err
		}
		updated, err := h.store.UpdateLottery(ctx, corpID, id, values)
		if err != nil {
			return nil, err
		}
		if !updated {
			return nil, badRequestError("lottery not found")
		}
		return []any{id}, nil
	})
}

func (h *LotteryHandler) Destroy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, user, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/lottery/destroy#delete")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, 400, err.Error(), nil)
		return
	}
	id, err := lotteryIDFromParams(params)
	if err != nil || id <= 0 {
		writeEnvelope(w, http.StatusBadRequest, 400, "lotteryId required", nil)
		return
	}
	deleted, err := h.store.DeleteLottery(r.Context(), corpID, id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !deleted {
		writeEnvelope(w, http.StatusBadRequest, 400, "lottery not found", nil)
		return
	}
	if err := refreshSaaSUsageCounter(r.Context(), h.store, user.TenantID, SaaSMetricLotteries); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{id})
}

func (h *LotteryHandler) Show(w http.ResponseWriter, r *http.Request) {
	h.showLottery(w, r, "/dashboard/lottery/show#get")
}

func (h *LotteryHandler) Info(w http.ResponseWriter, r *http.Request) {
	h.showLottery(w, r, "/dashboard/lottery/info#get")
}

func (h *LotteryHandler) Share(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/lottery/share#get")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	id := lotteryIDFromQuery(r)
	if id <= 0 {
		writeEnvelope(w, http.StatusBadRequest, 400, "lotteryId required", nil)
		return
	}
	item, prize, exists, err := h.store.LotteryByID(r.Context(), corpID, id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !exists {
		writeEnvelope(w, http.StatusNotFound, 404, "lottery not found", nil)
		return
	}
	link := h.shareURL(id)
	payload := lotteryPayload(item, prize, true, link)
	payload["link"] = link
	payload["url"] = link
	payload["qrcodeUrl"] = link
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *LotteryHandler) ShowContact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/lottery/showContact#get")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	page, err := h.store.LotteryContactPage(r.Context(), LotteryContactFilter{
		CorpID:    corpID,
		LotteryID: lotteryIDFromQuery(r),
		Name:      lotteryQueryString(r, "name", "nickname", "contactName"),
		Status:    lotteryQueryIntAllowZero(r, "status", -1),
		WriteOff:  lotteryQueryIntAllowZero(r, "writeOff", -1),
		Page:      positiveQueryInt(r, "page", 1),
		PerPage:   positiveQueryInt(r, "perPage", 15),
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, lotteryContactPayload(item))
	}
	payload := contactBatchAddPagination(r, page.Page, page.PerPage, page.Total, list)
	payload["list"] = list
	payload["page"] = map[string]any{"perPage": page.PerPage, "total": page.Total, "totalPage": page.TotalPage}
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *LotteryHandler) WriteOff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/lottery/writeOff#get")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	lotteryID := lotteryIDFromQuery(r)
	contactID := positiveQueryInt(r, "contactId", 0)
	if contactID <= 0 {
		contactID = positiveQueryInt(r, "id", 0)
	}
	if lotteryID <= 0 || contactID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, 400, "lotteryId and contactId required", nil)
		return
	}
	updated, err := h.store.WriteOffLotteryContact(r.Context(), corpID, lotteryID, contactID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !updated {
		writeEnvelope(w, http.StatusNotFound, 404, "lottery contact not found", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{contactID})
}

func (h *LotteryHandler) BatchContactTags(w http.ResponseWriter, r *http.Request) {
	h.writeMutation(w, r, http.MethodPut, "/dashboard/lottery/batchContactTags#put", func(ctx context.Context, _ int, corpID int, params map[string]any) (any, error) {
		lotteryID, _, err := firstPositiveIntParam(params, "lotteryId", "lottery_id", "id")
		if err != nil || lotteryID <= 0 {
			return nil, badRequestError("lotteryId required")
		}
		contactIDs, err := firstIntSliceParam(params, "contactIds", "contact_ids", "contactId", "contact_id")
		if err != nil || len(contactIDs) == 0 {
			return nil, badRequestError("contactIds required")
		}
		tagIDs, err := firstIntSliceParam(params, "tagIds", "tag_ids", "tags", "contactTags")
		if err != nil || len(tagIDs) == 0 {
			return nil, badRequestError("tagIds required")
		}
		affected, err := h.store.BatchTagLotteryContacts(ctx, corpID, lotteryID, contactIDs, tagIDs)
		if err != nil {
			return nil, err
		}
		return map[string]any{"affected": affected}, nil
	})
}

func (h *LotteryHandler) showLottery(w http.ResponseWriter, r *http.Request, permission string) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, _, ok := h.resolveAuthorized(w, r, permission)
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	id := lotteryIDFromQuery(r)
	if id <= 0 {
		writeEnvelope(w, http.StatusBadRequest, 400, "lotteryId required", nil)
		return
	}
	item, prize, exists, err := h.store.LotteryByID(r.Context(), corpID, id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !exists {
		writeEnvelope(w, http.StatusNotFound, 404, "lottery not found", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", lotteryPayload(item, prize, true, h.shareURL(id)))
}

func (h *LotteryHandler) writeMutation(w http.ResponseWriter, r *http.Request, method string, permission string, mutate func(context.Context, int, int, map[string]any) (any, error)) {
	if r.Method != method {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, loginInfo, _, ok := h.resolveAuthorized(w, r, permission)
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, 400, err.Error(), nil)
		return
	}
	data, err := mutate(r.Context(), userID, corpID, params)
	if err != nil {
		status := http.StatusInternalServerError
		code := http.StatusInternalServerError
		if isBadRequestError(err) {
			status = http.StatusBadRequest
			code = 400
		}
		writeEnvelope(w, status, code, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", data)
}

func (h *LotteryHandler) resolveAuthorized(w http.ResponseWriter, r *http.Request, permissionKey string) (int, User, LoginCorpInfo, AccessContext, bool) {
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

func (h *LotteryHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (int, User, LoginCorpInfo, bool) {
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
	return userID, user, loginInfo, true
}

func (h *LotteryHandler) shareURL(id int) string {
	base := h.operationBaseURL
	if base == "" {
		base = "/operation"
	}
	return base + "/lottery?id=" + strconv.Itoa(id)
}

func lotteryWriteFromParams(params map[string]any, corpID int, userID int, requireName bool) (LotteryWrite, error) {
	values := LotteryWrite{CorpID: corpID, CreateUserID: userID}
	if name, ok := firstStringParam(params, "name", "activityName"); ok {
		values.Name = name
		values.HasName = true
	}
	if requireName && strings.TrimSpace(values.Name) == "" {
		return values, badRequestError("name required")
	}
	if description, ok := firstStringParam(params, "description", "desc"); ok {
		values.Description = description
		values.HasDescription = true
	}
	if typ, ok := firstStringParam(params, "type", "template"); ok {
		values.Type = typ
		values.HasType = true
	}
	if !values.HasType && requireName {
		values.Type = "roulette"
		values.HasType = true
	}
	if timeType, ok, err := firstIntParam(params, "timeType", "time_type"); err != nil {
		return values, err
	} else if ok {
		values.TimeType = timeType
		values.HasTimeType = true
	}
	if !values.HasTimeType && requireName {
		values.TimeType = 1
		values.HasTimeType = true
	}
	if startTime, ok := firstStringParam(params, "startTime", "start_time"); ok {
		values.StartTime = startTime
		values.HasStartTime = true
	}
	if endTime, ok := firstStringParam(params, "endTime", "end_time"); ok {
		values.EndTime = endTime
		values.HasEndTime = true
	}
	if raw, ok := firstJSONRawParam(params, "contactTags", "contact_tags", "tags"); ok {
		values.ContactTagsRaw = raw
		values.HasContactTags = true
	}
	if raw, ok := firstJSONRawParam(params, "prizeSet", "prize_set", "prizes"); ok {
		values.PrizeSetRaw = raw
		values.HasPrizeSet = true
	}
	if isShow, ok, err := firstIntParam(params, "isShow", "is_show"); err != nil {
		return values, err
	} else if ok {
		values.IsShow = isShow
		values.HasIsShow = true
	}
	if raw, ok := firstJSONRawParam(params, "exchangeSet", "exchange_set"); ok {
		values.ExchangeSetRaw = raw
		values.HasExchangeSet = true
	}
	if raw, ok := firstJSONRawParam(params, "drawSet", "draw_set"); ok {
		values.DrawSetRaw = raw
		values.HasDrawSet = true
	}
	if raw, ok := firstJSONRawParam(params, "winSet", "win_set"); ok {
		values.WinSetRaw = raw
		values.HasWinSet = true
	}
	if raw, ok := firstJSONRawParam(params, "corpCard", "corp_card"); ok {
		values.CorpCardRaw = raw
		values.HasCorpCard = true
	}
	return values, nil
}

func lotteryIDFromParams(params map[string]any) (int, error) {
	id, _, err := firstPositiveIntParam(params, "lotteryId", "lottery_id", "id")
	return id, err
}

func lotteryIDFromQuery(r *http.Request) int {
	for _, key := range []string{"lotteryId", "lottery_id", "id"} {
		if value := positiveQueryInt(r, key, 0); value > 0 {
			return value
		}
	}
	return 0
}

func lotteryQueryString(r *http.Request, keys ...string) string {
	query := r.URL.Query()
	for _, key := range keys {
		if value := strings.TrimSpace(query.Get(key)); value != "" {
			return value
		}
	}
	return ""
}

func lotteryQueryIntAllowZero(r *http.Request, key string, fallback int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return fallback
	}
	return value
}

func lotteryPayload(item LotteryItem, prize LotteryPrizeItem, includePrize bool, shareURL string) map[string]any {
	payload := map[string]any{
		"id":             item.ID,
		"lotteryId":      item.ID,
		"name":           item.Name,
		"description":    item.Description,
		"type":           item.Type,
		"timeType":       item.TimeType,
		"time_type":      item.TimeType,
		"startTime":      item.StartTime,
		"start_time":     item.StartTime,
		"endTime":        item.EndTime,
		"end_time":       item.EndTime,
		"contactTags":    jsonPayload(item.ContactTagsRaw),
		"contact_tags":   jsonPayload(item.ContactTagsRaw),
		"tenantId":       item.TenantID,
		"corpId":         item.CorpID,
		"createUserId":   item.CreateUserID,
		"createUserName": item.CreateUserName,
		"contactNum":     item.ContactNum,
		"winNum":         item.WinNum,
		"shareUrl":       shareURL,
		"createdAt":      item.CreatedAt,
		"updatedAt":      item.UpdatedAt,
	}
	if includePrize {
		prizePayload := map[string]any{
			"id":           prize.ID,
			"lotteryId":    prize.LotteryID,
			"prizeSet":     jsonPayload(prize.PrizeSetRaw),
			"prize_set":    jsonPayload(prize.PrizeSetRaw),
			"isShow":       prize.IsShow,
			"is_show":      prize.IsShow,
			"exchangeSet":  jsonPayload(prize.ExchangeRaw),
			"exchange_set": jsonPayload(prize.ExchangeRaw),
			"drawSet":      jsonPayload(prize.DrawSetRaw),
			"draw_set":     jsonPayload(prize.DrawSetRaw),
			"winSet":       jsonPayload(prize.WinSetRaw),
			"win_set":      jsonPayload(prize.WinSetRaw),
			"corpCard":     jsonPayload(prize.CorpCardRaw),
			"corp_card":    jsonPayload(prize.CorpCardRaw),
			"createdAt":    prize.CreatedAt,
			"updatedAt":    prize.UpdatedAt,
		}
		payload["prize"] = prizePayload
		for key, value := range prizePayload {
			payload[key] = value
		}
	}
	return payload
}

func lotteryContactPayload(item LotteryContactItem) map[string]any {
	return map[string]any{
		"id":              item.ID,
		"contactRecordId": item.ID,
		"lotteryId":       item.LotteryID,
		"unionId":         item.UnionID,
		"contactId":       item.ContactID,
		"nickname":        item.Nickname,
		"name":            item.Nickname,
		"avatar":          item.Avatar,
		"employeeIds":     jsonPayload(item.EmployeeIDsRaw),
		"city":            item.City,
		"source":          item.Source,
		"grade":           item.Grade,
		"contactTags":     jsonPayload(item.ContactTagsRaw),
		"drawNum":         item.DrawNum,
		"winNum":          item.WinNum,
		"status":          item.Status,
		"writeOff":        item.WriteOff,
		"prizeName":       item.PrizeName,
		"receiveStatus":   item.ReceiveStatus,
		"receiveType":     item.ReceiveType,
		"receiveCode":     item.ReceiveCode,
		"receiveQr":       item.ReceiveQR,
		"createdAt":       item.CreatedAt,
		"updatedAt":       item.UpdatedAt,
	}
}

func firstStringParam(params map[string]any, keys ...string) (string, bool) {
	for _, key := range keys {
		if _, exists := params[key]; !exists {
			continue
		}
		return stringParam(params, key), true
	}
	return "", false
}

func firstIntParam(params map[string]any, keys ...string) (int, bool, error) {
	for _, key := range keys {
		value, ok, err := intParam(params, key)
		if err != nil || ok {
			return value, ok, err
		}
	}
	return 0, false, nil
}

func firstIntSliceParam(params map[string]any, keys ...string) ([]int, error) {
	for _, key := range keys {
		if _, exists := params[key]; !exists {
			continue
		}
		return intSliceParam(params, key)
	}
	return []int{}, nil
}

func firstJSONRawParam(params map[string]any, keys ...string) (string, bool) {
	for _, key := range keys {
		value, exists := params[key]
		if !exists {
			continue
		}
		return jsonRawValue(value), true
	}
	return "", false
}

func jsonRawValue(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(raw)
}

func jsonPayload(raw string) any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []any{}
	}
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err == nil {
		return value
	}
	values, err := url.ParseQuery(raw)
	if err == nil && len(values) > 0 {
		return values
	}
	return raw
}
