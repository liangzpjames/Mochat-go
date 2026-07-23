package dashboard

import (
	"context"
	"net/http"
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

type CorpDataStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	CorpDataSummary(ctx context.Context, corpID int, now time.Time) (CorpDataSummary, error)
	CorpDataLineChat(ctx context.Context, corpID int, now time.Time) ([]CorpDataPoint, error)
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
	_, _, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if len(loginInfo.CorpIDs) != 1 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "请先选择企业", nil)
		return
	}
	corpID := loginInfo.CorpIDs[0]

	data, err := h.store.CorpDataSummary(r.Context(), corpID, h.currentTime())
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", corpDataSummaryPayload(data))
}

func (h *CorpDataHandler) LineChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if len(loginInfo.CorpIDs) != 1 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "请先选择企业", nil)
		return
	}
	corpID := loginInfo.CorpIDs[0]

	data, err := h.store.CorpDataLineChat(r.Context(), corpID, h.currentTime())
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", data)
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
