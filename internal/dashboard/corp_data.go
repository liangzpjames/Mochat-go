package dashboard

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

type CorpDataSummary struct {
	WeChatContactNum          int
	WeChatRoomNum             int
	RoomMemberNum             int
	CorpMemberNum             int
	AddContactNum             int
	LastAddContactNum         int
	AddIntoRoomNum            int
	LastAddIntoRoomNum        int
	LossContactNum            int
	LastLossContactNum        int
	QuitRoomNum               int
	LastQuitRoomNum           int
	AddFriendsNum             int
	LastAddFriendsNum         int
	MonthAddRoomNum           int
	LastMonthAddRoomNum       int
	MonthAddRoomMemberNum     int
	LastMonthAddRoomMemberNum int
	MonthLossContactNum       int
	LastMonthLossContactNum   int
	UpdateTime                string
}

type CorpDataPoint struct {
	ID             int    `json:"id"`
	AddContactNum  int    `json:"addContactNum"`
	AddIntoRoomNum int    `json:"addIntoRoomNum"`
	LossContactNum int    `json:"lossContactNum"`
	QuitRoomNum    int    `json:"quitRoomNum"`
	Date           string `json:"date"`
}

type CorpDataCard struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Value int    `json:"value"`
}

type CorpDataTrendPoint struct {
	Date           string `json:"date"`
	AddContactNum  int    `json:"addContactNum"`
	AddIntoRoomNum int    `json:"addIntoRoomNum"`
	LossContactNum int    `json:"lossContactNum"`
	QuitRoomNum    int    `json:"quitRoomNum"`
}

type CorpDataOverviewQuery struct {
	StartDate     time.Time
	EndDate       time.Time
	EmployeeIDs   []int
	DepartmentIDs []int
	Period        string
	Page          int
	PageSize      int
}

type CorpDataAccessAuthorizer interface {
	Resolve(ctx context.Context, userID int, permissionKey string, corpID int, workEmployeeID int) (AccessContext, error)
}

type CorpDataStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	CorpDataSummary(ctx context.Context, corpID int, now time.Time) (CorpDataSummary, error)
	CorpDataLineChat(ctx context.Context, corpID int, from time.Time, to time.Time) ([]CorpDataPoint, error)
}

type CorpDataHandler struct {
	store      CorpDataStore
	cache      LoginCache
	resolver   UserIDResolver
	authorizer CorpDataAccessAuthorizer
	now        func() time.Time
}

func NewCorpDataHandler(store CorpDataStore, cache LoginCache, resolver UserIDResolver, authorizer ...CorpDataAccessAuthorizer) *CorpDataHandler {
	h := &CorpDataHandler{store: store, cache: cache, resolver: resolver}
	if len(authorizer) > 0 {
		h.authorizer = authorizer[0]
	}
	return h
}

func (h *CorpDataHandler) Index(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	corpID, query, ok := h.resolveOverviewRequest(w, r)
	if !ok {
		return
	}

	summary, err := h.store.CorpDataSummary(r.Context(), corpID, query.EndDate)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	points, err := h.store.CorpDataLineChat(r.Context(), corpID, query.StartDate, query.EndDate)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", corpDataOverviewPayload(summary, corpDataPage(points, query)))
}

func (h *CorpDataHandler) LineChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	corpID, query, ok := h.resolveOverviewRequest(w, r)
	if !ok {
		return
	}

	data, err := h.store.CorpDataLineChat(r.Context(), corpID, query.StartDate, query.EndDate)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", corpDataPage(data, query).Points)
}

func (h *CorpDataHandler) resolveOverviewRequest(w http.ResponseWriter, r *http.Request) (int, CorpDataOverviewQuery, bool) {
	userID, _, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return 0, CorpDataOverviewQuery{}, false
	}
	if len(loginInfo.CorpIDs) != 1 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "请先选择企业", nil)
		return 0, CorpDataOverviewQuery{}, false
	}
	corpID := loginInfo.CorpIDs[0]
	if requested := r.URL.Query().Get("corpId"); requested != "" {
		requestedCorpID, err := strconv.Atoi(requested)
		if err != nil || requestedCorpID <= 0 {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid corpId", nil)
			return 0, CorpDataOverviewQuery{}, false
		}
		if requestedCorpID != corpID {
			writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, "forbidden", nil)
			return 0, CorpDataOverviewQuery{}, false
		}
	}
	query, err := corpDataOverviewQuery(r, h.currentTime())
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return 0, CorpDataOverviewQuery{}, false
	}
	if h.authorizer != nil {
		access, err := h.authorizer.Resolve(r.Context(), userID, PermissionKeyFromRequest(r), corpID, loginInfo.WorkEmployeeID)
		if err != nil {
			writeAccessError(w, err)
			return 0, CorpDataOverviewQuery{}, false
		}
		if access.DataPermission != DataPermissionAll {
			query.EmployeeIDs = corpDataScopedEmployeeIDs(query.EmployeeIDs, access.DeptEmployeeIDs)
		}
	}
	return corpID, query, true
}

func corpDataScopedEmployeeIDs(requested []int, allowed []int) []int {
	allowedSet := make(map[int]struct{}, len(allowed))
	for _, employeeID := range allowed {
		if employeeID > 0 {
			allowedSet[employeeID] = struct{}{}
		}
	}
	if len(requested) == 0 {
		return sortedCorpDataIDs(allowedSet)
	}
	result := make([]int, 0, len(requested))
	for _, employeeID := range requested {
		if _, ok := allowedSet[employeeID]; ok {
			result = append(result, employeeID)
		}
	}
	return result
}

func sortedCorpDataIDs(values map[int]struct{}) []int {
	result := make([]int, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Ints(result)
	return result
}

func corpDataOverviewQuery(r *http.Request, now time.Time) (CorpDataOverviewQuery, error) {
	values := r.URL.Query()
	for _, key := range []string{"startDate", "endDate", "employeeIds", "departmentIds", "period", "page", "pageSize"} {
		if !values.Has(key) {
			return CorpDataOverviewQuery{}, &corpDataInputError{"missing " + key}
		}
	}
	from, to, err := corpDataDateRangeValues(values.Get("startDate"), values.Get("endDate"), now)
	if err != nil {
		return CorpDataOverviewQuery{}, err
	}
	employeeIDs, err := corpDataPositiveIDs(values["employeeIds"], "employeeIds")
	if err != nil {
		return CorpDataOverviewQuery{}, err
	}
	departmentIDs, err := corpDataPositiveIDs(values["departmentIds"], "departmentIds")
	if err != nil {
		return CorpDataOverviewQuery{}, err
	}
	period := values.Get("period")
	if period != "day" && period != "week" && period != "month" {
		return CorpDataOverviewQuery{}, &corpDataInputError{"invalid period"}
	}
	page, err := corpDataPositiveInt(values.Get("page"), "page")
	if err != nil {
		return CorpDataOverviewQuery{}, err
	}
	pageSize, err := corpDataPositiveInt(values.Get("pageSize"), "pageSize")
	if err != nil {
		return CorpDataOverviewQuery{}, err
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return CorpDataOverviewQuery{StartDate: from, EndDate: to, EmployeeIDs: employeeIDs, DepartmentIDs: departmentIDs, Period: period, Page: page, PageSize: pageSize}, nil
}

func corpDataPositiveIDs(raw []string, name string) ([]int, error) {
	set := map[int]struct{}{}
	for _, value := range raw {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			id, err := strconv.Atoi(part)
			if err != nil || id <= 0 {
				return nil, &corpDataInputError{"invalid " + name}
			}
			set[id] = struct{}{}
		}
	}
	return sortedCorpDataIDs(set), nil
}

func corpDataPositiveInt(raw string, name string) (int, error) {
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, &corpDataInputError{"invalid " + name}
	}
	return value, nil
}

func corpDataDateRange(r *http.Request, now time.Time) (time.Time, time.Time, error) {
	fromText := r.URL.Query().Get("from")
	toText := r.URL.Query().Get("to")
	return corpDataDateRangeValues(fromText, toText, now)
}

func corpDataDateRangeValues(fromText string, toText string, now time.Time) (time.Time, time.Time, error) {
	if fromText == "" && toText == "" {
		to := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return to.AddDate(0, 0, -30), to, nil
	}
	if fromText == "" || toText == "" {
		return time.Time{}, time.Time{}, &corpDataInputError{"from and to are required together"}
	}
	from, err := time.ParseInLocation("2006-01-02", fromText, now.Location())
	if err != nil {
		return time.Time{}, time.Time{}, &corpDataInputError{"invalid from date"}
	}
	to, err := time.ParseInLocation("2006-01-02", toText, now.Location())
	if err != nil {
		return time.Time{}, time.Time{}, &corpDataInputError{"invalid to date"}
	}
	if from.After(to) {
		return time.Time{}, time.Time{}, &corpDataInputError{"from must not be after to"}
	}
	if to.Sub(from) > 30*24*time.Hour {
		return time.Time{}, time.Time{}, &corpDataInputError{"date range must not exceed 31 days"}
	}
	return from, to, nil
}

type corpDataPagedPoints struct {
	Points   []CorpDataPoint
	Total    int
	Page     int
	PageSize int
}

func corpDataPage(points []CorpDataPoint, query CorpDataOverviewQuery) corpDataPagedPoints {
	aggregated := corpDataAggregatePeriod(points, query.Period)
	total := len(aggregated)
	start := (query.Page - 1) * query.PageSize
	if start >= total {
		return corpDataPagedPoints{Points: []CorpDataPoint{}, Total: total, Page: query.Page, PageSize: query.PageSize}
	}
	end := start + query.PageSize
	if end > total {
		end = total
	}
	return corpDataPagedPoints{Points: aggregated[start:end], Total: total, Page: query.Page, PageSize: query.PageSize}
}

func corpDataAggregatePeriod(points []CorpDataPoint, period string) []CorpDataPoint {
	if period == "day" {
		return append([]CorpDataPoint{}, points...)
	}
	grouped := map[string]CorpDataPoint{}
	order := make([]string, 0, len(points))
	for _, point := range points {
		date, err := time.Parse("2006-01-02", point.Date[:min(len(point.Date), len("2006-01-02"))])
		if err != nil {
			continue
		}
		key := date.Format("2006-01")
		if period == "week" {
			offset := (int(date.Weekday()) + 6) % 7
			key = date.AddDate(0, 0, -offset).Format("2006-01-02")
		}
		current, exists := grouped[key]
		if !exists {
			current = CorpDataPoint{Date: key}
			order = append(order, key)
		}
		current.AddContactNum += point.AddContactNum
		current.AddIntoRoomNum += point.AddIntoRoomNum
		current.LossContactNum += point.LossContactNum
		current.QuitRoomNum += point.QuitRoomNum
		grouped[key] = current
	}
	sort.Strings(order)
	result := make([]CorpDataPoint, 0, len(order))
	for _, key := range order {
		result = append(result, grouped[key])
	}
	return result
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}

type corpDataInputError struct {
	message string
}

func (e *corpDataInputError) Error() string {
	return e.message
}

func (h *CorpDataHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (int, User, LoginCorpInfo, bool) {
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

func (h *CorpDataHandler) currentTime() time.Time {
	if h.now != nil {
		return h.now()
	}
	return time.Now()
}

func corpDataSummaryPayload(data CorpDataSummary) map[string]any {
	return map[string]any{
		"weChatContactNum":          data.WeChatContactNum,
		"weChatRoomNum":             data.WeChatRoomNum,
		"roomMemberNum":             data.RoomMemberNum,
		"corpMemberNum":             data.CorpMemberNum,
		"addContactNum":             data.AddContactNum,
		"lastAddContactNum":         data.LastAddContactNum,
		"addIntoRoomNum":            data.AddIntoRoomNum,
		"lastAddIntoRoomNum":        data.LastAddIntoRoomNum,
		"lossContactNum":            data.LossContactNum,
		"lastLossContactNum":        data.LastLossContactNum,
		"quitRoomNum":               data.QuitRoomNum,
		"lastQuitRoomNum":           data.LastQuitRoomNum,
		"addFriendsNum":             data.AddFriendsNum,
		"lastAddFriendsNum":         data.LastAddFriendsNum,
		"monthAddRoomNum":           data.MonthAddRoomNum,
		"lastMonthAddRoomNum":       data.LastMonthAddRoomNum,
		"monthAddRoomMemberNum":     data.MonthAddRoomMemberNum,
		"lastMonthAddRoomMemberNum": data.LastMonthAddRoomMemberNum,
		"monthLossContactNum":       data.MonthLossContactNum,
		"lastMonthLossContactNum":   data.LastMonthLossContactNum,
		"updateTime":                data.UpdateTime,
	}
}

func corpDataOverviewPayload(summary CorpDataSummary, page corpDataPagedPoints) map[string]any {
	payload := corpDataSummaryPayload(summary)
	cards := make([]CorpDataCard, 0, 4)
	if summary != (CorpDataSummary{}) {
		cards = append(cards,
			CorpDataCard{Key: "contacts", Label: "客户总数", Value: summary.WeChatContactNum},
			CorpDataCard{Key: "rooms", Label: "客户群总数", Value: summary.WeChatRoomNum},
			CorpDataCard{Key: "roomMembers", Label: "群成员总数", Value: summary.RoomMemberNum},
			CorpDataCard{Key: "employees", Label: "员工总数", Value: summary.CorpMemberNum},
		)
	}
	payload["cards"] = cards
	trend := make([]CorpDataTrendPoint, 0, len(page.Points))
	for _, point := range page.Points {
		date := point.Date
		if len(date) >= len("2006-01-02") {
			date = date[:len("2006-01-02")]
		}
		trend = append(trend, CorpDataTrendPoint{
			Date:           date,
			AddContactNum:  point.AddContactNum,
			AddIntoRoomNum: point.AddIntoRoomNum,
			LossContactNum: point.LossContactNum,
			QuitRoomNum:    point.QuitRoomNum,
		})
	}
	payload["trend"] = trend
	payload["total"] = page.Total
	payload["page"] = page.Page
	payload["pageSize"] = page.PageSize
	payload["updatedAt"] = summary.UpdateTime
	return payload
}
