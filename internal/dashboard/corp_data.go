package dashboard

import (
	"context"
	"net/http"
	"strconv"
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

type CorpDataStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	CorpDataSummary(ctx context.Context, corpID int, now time.Time) (CorpDataSummary, error)
	CorpDataLineChat(ctx context.Context, corpID int, from time.Time, to time.Time) ([]CorpDataPoint, error)
}

type CorpDataHandler struct {
	store    CorpDataStore
	cache    LoginCache
	resolver UserIDResolver
	now      func() time.Time
}

func NewCorpDataHandler(store CorpDataStore, cache LoginCache, resolver UserIDResolver) *CorpDataHandler {
	return &CorpDataHandler{store: store, cache: cache, resolver: resolver}
}

func (h *CorpDataHandler) Index(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	corpID, from, to, ok := h.resolveOverviewRequest(w, r)
	if !ok {
		return
	}

	summary, err := h.store.CorpDataSummary(r.Context(), corpID, to)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	points, err := h.store.CorpDataLineChat(r.Context(), corpID, from, to)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", corpDataOverviewPayload(summary, points))
}

func (h *CorpDataHandler) LineChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	corpID, from, to, ok := h.resolveOverviewRequest(w, r)
	if !ok {
		return
	}

	data, err := h.store.CorpDataLineChat(r.Context(), corpID, from, to)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", data)
}

func (h *CorpDataHandler) resolveOverviewRequest(w http.ResponseWriter, r *http.Request) (int, time.Time, time.Time, bool) {
	_, _, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return 0, time.Time{}, time.Time{}, false
	}
	if len(loginInfo.CorpIDs) != 1 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "请先选择企业", nil)
		return 0, time.Time{}, time.Time{}, false
	}
	corpID := loginInfo.CorpIDs[0]
	if requested := r.URL.Query().Get("corpId"); requested != "" {
		requestedCorpID, err := strconv.Atoi(requested)
		if err != nil || requestedCorpID <= 0 {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid corpId", nil)
			return 0, time.Time{}, time.Time{}, false
		}
		if requestedCorpID != corpID {
			writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, "forbidden", nil)
			return 0, time.Time{}, time.Time{}, false
		}
	}
	from, to, err := corpDataDateRange(r, h.currentTime())
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return 0, time.Time{}, time.Time{}, false
	}
	return corpID, from, to, true
}

func corpDataDateRange(r *http.Request, now time.Time) (time.Time, time.Time, error) {
	fromText := r.URL.Query().Get("from")
	toText := r.URL.Query().Get("to")
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

func corpDataOverviewPayload(summary CorpDataSummary, points []CorpDataPoint) map[string]any {
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
	trend := make([]CorpDataTrendPoint, 0, len(points))
	for _, point := range points {
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
	payload["updatedAt"] = summary.UpdateTime
	return payload
}
