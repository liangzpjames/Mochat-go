package dashboard

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	operationModuleLottery         = "lottery"
	operationModuleRoomClockIn     = "roomClockIn"
	operationModuleRoomFission     = "roomFission"
	operationModuleShopCode        = "shopCode"
	operationOfficialTypeRoomClock = 1
	operationOfficialTypeLottery   = 2
	operationOfficialTypeRoomFiss  = 8
	operationDefaultSessionTTL     = defaultOperationSessionTTL
)

type OperationWechatUser struct {
	UnionID  string
	OpenID   string
	Nickname string
	Avatar   string
	City     string
	Source   string
}

type OperationLotteryRecord struct {
	ID            int
	LotteryID     int
	ContactID     int
	PrizeID       int
	PrizeName     string
	ReceiveStatus int
	ReceiveQR     string
	ReceiveType   int
	ReceiveCode   string
	WriteOff      int
	CreatedAt     string
	UpdatedAt     string
}

type OperationH5Store interface {
	LotteryByIDNoCorp(ctx context.Context, id int) (LotteryItem, LotteryPrizeItem, bool, error)
	EnsureLotteryOperationContact(ctx context.Context, lotteryID int, user OperationWechatUser) (LotteryContactItem, error)
	LotteryOperationRecords(ctx context.Context, lotteryID int, contactID int) ([]OperationLotteryRecord, error)
	CreateLotteryOperationRecord(ctx context.Context, lotteryID int, contactID int, prizeID int, prizeName string, receiveType int, receiveQR string, receiveCode string) (OperationLotteryRecord, error)
	MarkLotteryOperationRecordReceived(ctx context.Context, id int) (bool, error)

	RoomClockInByIDNoCorp(ctx context.Context, id int) (RoomClockInItem, bool, error)
	EnsureRoomClockInOperationContact(ctx context.Context, clockInID int, user OperationWechatUser) (RoomClockInContactItem, error)
	RoomClockInOperationRecords(ctx context.Context, clockInID int, contactID int) ([]RoomClockInDayRecord, error)
	RecordRoomClockInOperationDay(ctx context.Context, clockInID int, user OperationWechatUser) (RoomClockInContactItem, bool, error)
	UpdateRoomClockInOperationReceiveLevel(ctx context.Context, clockInID int, unionID string, level int) (bool, error)
	RoomClockInOperationRanking(ctx context.Context, clockInID int, unionID string) (int, []RoomClockInContactItem, error)

	RoomFissionBundleByIDNoCorp(ctx context.Context, id int) (RoomFissionBundle, bool, error)
	EnsureRoomFissionOperationContact(ctx context.Context, fissionID int, user OperationWechatUser, parentUnionID string, roomID int) (RoomFissionContactItem, error)
	RoomFissionOperationChildren(ctx context.Context, fissionID int, parentUnionID string) ([]RoomFissionContactItem, error)
	UpdateRoomFissionOperationReceiveStatus(ctx context.Context, fissionID int, unionID string) (bool, error)

	RoomInfinitePullByIDNoCorp(ctx context.Context, id int) (RoomInfinitePullItem, bool, error)
	ShopCodePage(ctx context.Context, filter ShopCodeFilter) (ShopCodePage, error)
	ShopCodeCities(ctx context.Context, corpID int, keyword string) ([]ShopCodeCity, error)
	ShopCodePageSetting(ctx context.Context, corpID int, codeType int) (ShopCodePageSetting, bool, error)
	CreateShopCodeOperationRecord(ctx context.Context, corpID int, codeType int, shopID int) error

	OperationOfficialAccountInfo(ctx context.Context, module string, id int, params map[string]any) (OfficialAccountOAuthInfo, bool, error)
	OfficialAccountOAuthInfoByAuthorizerAppID(ctx context.Context, authorizerAppID string) (OfficialAccountOAuthInfo, bool, error)
}

type OperationH5Handler struct {
	store            OperationH5Store
	session          OperationSessionStore
	oauthClient      OfficialAccountOAuthClient
	apiBaseURL       string
	operationBaseURL string
	sessionTTL       time.Duration
}

func NewOperationH5Handler(store OperationH5Store, session OperationSessionStore, oauthClient OfficialAccountOAuthClient, apiBaseURL string, operationBaseURL string) *OperationH5Handler {
	return &OperationH5Handler{
		store:            store,
		session:          session,
		oauthClient:      oauthClient,
		apiBaseURL:       strings.TrimRight(strings.TrimSpace(apiBaseURL), "/"),
		operationBaseURL: strings.TrimRight(strings.TrimSpace(operationBaseURL), "/"),
		sessionTTL:       operationDefaultSessionTTL,
	}
}

func (h *OperationH5Handler) LotteryContactData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	params, ok := h.operationParams(w, r)
	if !ok {
		return
	}
	lotteryID, ok := h.requiredID(w, params, "id", "lotteryId", "lottery_id")
	if !ok {
		return
	}
	lottery, prize, found, err := h.store.LotteryByIDNoCorp(r.Context(), lotteryID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, 400, "抽奖活动不存在", nil)
		return
	}
	user := operationWechatUserFromParams(params)
	contact, err := h.store.EnsureLotteryOperationContact(r.Context(), lotteryID, user)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	records, err := h.store.LotteryOperationRecords(r.Context(), lotteryID, contact.ID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	payload := lotteryPayload(lottery, prize, true, "")
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"lottery":  payload,
		"prize":    payload["prize"],
		"corp":     jsonPayload(prize.CorpCardRaw),
		"contact":  lotteryContactPayload(contact),
		"win_list": operationLotteryRecordsPayload(records, h.fullStaticURL),
		"message":  operationLotteryLatestMessage(records, h.fullStaticURL),
	})
}

func (h *OperationH5Handler) LotteryContactLottery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	params, ok := h.operationParams(w, r)
	if !ok {
		return
	}
	lotteryID, ok := h.requiredID(w, params, "id", "lotteryId", "lottery_id")
	if !ok {
		return
	}
	_, prize, found, err := h.store.LotteryByIDNoCorp(r.Context(), lotteryID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, 400, "抽奖活动不存在", nil)
		return
	}
	contact, err := h.store.EnsureLotteryOperationContact(r.Context(), lotteryID, operationWechatUserFromParams(params))
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	prizeID, prizeName := operationLotteryPrize(prize)
	receiveType, receiveQR, receiveCode := operationLotteryExchange(prize.ExchangeRaw)
	record, err := h.store.CreateLotteryOperationRecord(r.Context(), lotteryID, contact.ID, prizeID, prizeName, receiveType, receiveQR, receiveCode)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", operationLotteryRecordPayload(record, h.fullStaticURL))
}

func (h *OperationH5Handler) LotteryReceive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	params, ok := h.operationParams(w, r)
	if !ok {
		return
	}
	id, ok := h.requiredID(w, params, "id", "recordId", "record_id")
	if !ok {
		return
	}
	updated, err := h.store.MarkLotteryOperationRecordReceived(r.Context(), id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !updated {
		writeEnvelope(w, http.StatusBadRequest, 400, "领奖记录不存在", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{id})
}

func (h *OperationH5Handler) RoomClockInContactData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	params := valuesToParams(r.URL.Query())
	clockInID, ok := h.requiredID(w, params, "id", "clockInId", "clock_in_id")
	if !ok {
		return
	}
	item, found, err := h.store.RoomClockInByIDNoCorp(r.Context(), clockInID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, 400, "群打卡活动不存在", nil)
		return
	}
	contact, err := h.store.EnsureRoomClockInOperationContact(r.Context(), clockInID, operationWechatUserFromParams(params))
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	records, err := h.store.RoomClockInOperationRecords(r.Context(), clockInID, contact.ID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	payload := roomClockInPayload(item, "", false)
	payload["employee_qrcode"] = h.fullStaticURL(item.EmployeeQRCode)
	payload["employeeQrcode"] = h.fullStaticURL(item.EmployeeQRCode)
	payload["contact"] = roomClockInContactPayload(contact)
	payload["day_count"] = contact.DayCount
	payload["dayCount"] = contact.DayCount
	payload["clock_in_status"] = operationClockInStatus(records)
	payload["clockInStatus"] = payload["clock_in_status"]
	payload["day_detail"] = operationClockInDayDetail(records)
	payload["dayDetail"] = payload["day_detail"]
	payload["tasks"] = operationRoomClockInTasks(item.TasksRaw, contact.ReceiveLevel, contact.DayCount)
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *OperationH5Handler) RoomClockInContactClockIn(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	params, ok := h.operationParams(w, r)
	if !ok {
		return
	}
	clockInID, ok := h.requiredID(w, params, "id", "clockInId", "clock_in_id")
	if !ok {
		return
	}
	user := operationWechatUserFromParams(params)
	contact, _, err := h.store.RecordRoomClockInOperationDay(r.Context(), clockInID, user)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"id":        contact.ID,
		"day_count": contact.DayCount,
		"dayCount":  contact.DayCount,
		"contact":   roomClockInContactPayload(contact),
	})
}

func (h *OperationH5Handler) RoomClockInRanking(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	params := valuesToParams(r.URL.Query())
	clockInID, ok := h.requiredID(w, params, "id", "clockInId", "clock_in_id")
	if !ok {
		return
	}
	ranking, contacts, err := h.store.RoomClockInOperationRanking(r.Context(), clockInID, operationUnionIDFromParams(params))
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(contacts))
	for index, contact := range contacts {
		item := roomClockInContactPayload(contact)
		item["ranking"] = index + 1
		item["rank"] = index + 1
		item["avatar"] = h.fullStaticURL(contact.Avatar)
		list = append(list, item)
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"total_user":      len(contacts),
		"totalUser":       len(contacts),
		"contact_ranking": ranking,
		"contactRanking":  ranking,
		"contact_list":    list,
		"contactList":     list,
	})
}

func (h *OperationH5Handler) RoomClockInReceive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	params, ok := h.operationParams(w, r)
	if !ok {
		return
	}
	clockInID, ok := h.requiredID(w, params, "id", "clockInId", "clock_in_id")
	if !ok {
		return
	}
	level, exists, err := intParam(params, "level")
	if err != nil || !exists || level <= 0 {
		writeEnvelope(w, http.StatusBadRequest, 400, "level 必须为整数", nil)
		return
	}
	updated, err := h.store.UpdateRoomClockInOperationReceiveLevel(r.Context(), clockInID, operationUnionIDFromParams(params), level)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !updated {
		writeEnvelope(w, http.StatusBadRequest, 400, "客户打卡记录不存在", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{level})
}

func (h *OperationH5Handler) RoomFissionPoster(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	params := valuesToParams(r.URL.Query())
	fissionID, ok := h.requiredID(w, params, "fission_id", "fissionId", "id")
	if !ok {
		return
	}
	bundle, found, err := h.store.RoomFissionBundleByIDNoCorp(r.Context(), fissionID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, 400, "群裂变活动不存在", nil)
		return
	}
	if operationRoomFissionEnded(bundle.Fission) {
		writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{"type": 0})
		return
	}
	parentUnionID := operationFirstString(params, "parentUnionId", "parent_union_id")
	roomID := operationFirstPositiveInt(params, "roomId", "room_id", "wxUserId", "wx_user_id")
	user := operationWechatUserFromParams(params)
	contact, err := h.store.EnsureRoomFissionOperationContact(r.Context(), fissionID, user, parentUnionID, roomID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if strings.TrimSpace(parentUnionID) != "" && parentUnionID != "0" {
		if room, ok := operationFirstRoom(bundle.Rooms, roomID); ok {
			writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
				"type":    1,
				"room":    h.operationRoomFissionRoomPayload(room),
				"contact": roomFissionContactPayload(contact),
			})
			return
		}
	}
	poster := roomFissionPosterPayload(bundle.Poster)
	poster["coverPic"] = h.fullStaticURL(bundle.Poster.CoverPic)
	poster["cover_pic"] = poster["coverPic"]
	poster["qrCodeUrl"] = h.roomFissionShareURL(fissionID, operationContactUnion(contact, user))
	poster["qrcodeUrl"] = poster["qrCodeUrl"]
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"type":    2,
		"poster":  poster,
		"contact": roomFissionContactPayload(contact),
	})
}

func (h *OperationH5Handler) RoomFissionInviteFriends(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	params := valuesToParams(r.URL.Query())
	fissionID, ok := h.requiredID(w, params, "fission_id", "fissionId", "id")
	if !ok {
		return
	}
	bundle, found, err := h.store.RoomFissionBundleByIDNoCorp(r.Context(), fissionID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, 400, "群裂变活动不存在", nil)
		return
	}
	unionID := operationUnionIDFromParams(params)
	children, err := h.store.RoomFissionOperationChildren(r.Context(), fissionID, unionID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(children))
	success := 0
	for _, child := range children {
		if bundle.Fission.DeleteInvalid == 0 || child.Loss == 0 {
			success++
		}
		item := roomFissionContactPayload(child)
		item["avatar"] = h.fullStaticURL(child.Avatar)
		list = append(list, item)
	}
	diff := bundle.Fission.TargetCount - success
	if diff < 0 {
		diff = 0
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"success_num":    success,
		"successNum":     success,
		"diff_num":       diff,
		"diffNum":        diff,
		"invite_friends": list,
		"inviteFriends":  list,
	})
}

func (h *OperationH5Handler) RoomFissionReceive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	params := valuesToParams(r.URL.Query())
	fissionID, ok := h.requiredID(w, params, "fission_id", "fissionId", "id")
	if !ok {
		return
	}
	bundle, found, err := h.store.RoomFissionBundleByIDNoCorp(r.Context(), fissionID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, 400, "群裂变活动不存在", nil)
		return
	}
	_, err = h.store.UpdateRoomFissionOperationReceiveStatus(r.Context(), fissionID, operationUnionIDFromParams(params))
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{"qrCode": h.fullStaticURL(operationRoomFissionReceiveQRCode(bundle))})
}

func (h *OperationH5Handler) RoomInfinitePullQRCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	id := positiveQueryInt(r, "id", 0)
	if id <= 0 {
		writeEnvelope(w, http.StatusBadRequest, 400, "id 必填", nil)
		return
	}
	item, found, err := h.store.RoomInfinitePullByIDNoCorp(r.Context(), id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, 400, "无限拉群不存在", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"logo":      h.fullStaticURL(item.Logo),
		"roomName":  operationNonEmpty(item.Title, item.Name),
		"room_name": operationNonEmpty(item.Title, item.Name),
		"describe":  item.Describe,
		"qrcode":    h.fullStaticURL(operationFirstQRCode(item.QwCodeRaw)),
	})
}

func (h *OperationH5Handler) ShopCodeAreaCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	corpID := positiveQueryInt(r, "corpId", 0)
	if corpID <= 0 {
		corpID = positiveQueryInt(r, "corp_id", 0)
	}
	codeType := positiveQueryInt(r, "type", 1)
	filterCity := strings.TrimSpace(r.URL.Query().Get("city"))
	filterName := shopCodeQueryName(r)
	page, err := h.store.ShopCodePage(r.Context(), ShopCodeFilter{CorpID: corpID, Type: codeType, Status: 1, Name: filterName, City: filterCity, Page: 1, PerPage: 100})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	cities, err := h.store.ShopCodeCities(r.Context(), corpID, "")
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	setting, found, err := h.store.ShopCodePageSetting(r.Context(), corpID, codeType)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		setting = defaultShopCodePageSetting(corpID, codeType)
	}
	var shopInfo any = ""
	if len(page.Items) > 0 {
		shop := page.Items[0]
		payload := shopCodePayload(shop)
		payload["employeeQrcode"] = h.fullStaticURL(operationFirstQRCode(shop.EmployeeQRCodeRaw))
		payload["employee_qrcode"] = payload["employeeQrcode"]
		payload["qrcode"] = h.fullStaticURL(operationFirstQRCode(shop.QWCodeRaw))
		if payload["qrcode"] == "" {
			payload["qrcode"] = payload["employeeQrcode"]
		}
		_ = h.store.CreateShopCodeOperationRecord(r.Context(), corpID, codeType, shop.ID)
		shopInfo = payload
	}
	pagePayload := shopCodePageSettingPayload(setting)
	pagePayload["poster"] = h.fullStaticURL(setting.Poster)
	if defaults, ok := pagePayload["default"].(map[string]any); ok {
		for _, key := range []string{"logo", "image", "poster"} {
			if value := strings.TrimSpace(fmt.Sprint(defaults[key])); value != "" {
				defaults[key] = h.fullStaticURL(value)
			}
		}
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"province":  operationShopProvincePayload(cities),
		"shop_info": shopInfo,
		"shopInfo":  shopInfo,
		"page":      pagePayload,
	})
}

func (h *OperationH5Handler) ShopCodeWeChatSDKConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	params := valuesToParams(r.URL.Query())
	corpID := operationFirstPositiveInt(params, "corpId", "corp_id", "id")
	info, _, err := h.store.OperationOfficialAccountInfo(r.Context(), operationModuleShopCode, corpID, params)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	timestamp := time.Now().Unix()
	nonce := strconv.FormatInt(timestamp, 36)
	targetURL := operationFirstString(params, "url", "uri", "uriPath")
	signature := operationSHA1("jsapi_ticket=&noncestr=" + nonce + "&timestamp=" + strconv.FormatInt(timestamp, 10) + "&url=" + targetURL)
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"appId":     info.AuthorizerAppID,
		"timestamp": timestamp,
		"nonceStr":  nonce,
		"signature": signature,
	})
}

func (h *OperationH5Handler) AuthLottery(w http.ResponseWriter, r *http.Request) {
	h.auth(w, r, operationModuleLottery)
}

func (h *OperationH5Handler) AuthRoomClockIn(w http.ResponseWriter, r *http.Request) {
	h.auth(w, r, operationModuleRoomClockIn)
}

func (h *OperationH5Handler) AuthRoomFission(w http.ResponseWriter, r *http.Request) {
	h.auth(w, r, operationModuleRoomFission)
}

func (h *OperationH5Handler) AuthShopCode(w http.ResponseWriter, r *http.Request) {
	h.auth(w, r, operationModuleShopCode)
}

func (h *OperationH5Handler) OpenUserInfoLottery(w http.ResponseWriter, r *http.Request) {
	h.openUserInfo(w, r, operationModuleLottery)
}

func (h *OperationH5Handler) OpenUserInfoRoomClockIn(w http.ResponseWriter, r *http.Request) {
	h.openUserInfo(w, r, operationModuleRoomClockIn)
}

func (h *OperationH5Handler) OpenUserInfoRoomFission(w http.ResponseWriter, r *http.Request) {
	h.openUserInfo(w, r, operationModuleRoomFission)
}

func (h *OperationH5Handler) OpenUserInfoShopCode(w http.ResponseWriter, r *http.Request) {
	h.openUserInfo(w, r, operationModuleShopCode)
}

func (h *OperationH5Handler) auth(w http.ResponseWriter, r *http.Request, module string) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	params, err := operationRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	target := stringParam(params, "target")
	if target == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "target 必传", nil)
		return
	}
	sessionID, ok := h.ensureSessionID(w, r)
	if !ok {
		return
	}
	code := stringParam(params, "code")
	if code != "" {
		h.authCallback(w, r, module, params, sessionID, code, target)
		return
	}
	info, found := h.officialAccountInfo(w, r, module, params)
	if !found {
		return
	}
	if h.hasAuthorizedUser(w, r, sessionID, info) {
		http.Redirect(w, r, h.normalizeTarget(target), http.StatusFound)
		return
	}
	redirectURI := h.operationBaseURL + "/auth/" + module + "?target=" + url.QueryEscape(h.normalizeTarget(target))
	if id := operationFirstPositiveInt(params, "id", "corpId", "corp_id"); id > 0 {
		redirectURI += "&id=" + strconv.Itoa(id)
	}
	oauthURL, err := h.oauthClient.OAuthURL(info, redirectURI)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	http.Redirect(w, r, oauthURL, http.StatusFound)
}

func (h *OperationH5Handler) openUserInfo(w http.ResponseWriter, r *http.Request, module string) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	params := valuesToParams(r.URL.Query())
	info, found := h.officialAccountInfo(w, r, module, params)
	if !found {
		return
	}
	sessionID := operationSessionID(r)
	if sessionID == "" || h.session == nil {
		writeEnvelope(w, http.StatusOK, 200, "success", []any{})
		return
	}
	for _, key := range operationWechatUserKeys(info) {
		value, found, err := h.session.GetOperationSessionValue(r.Context(), sessionID, key)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		if found {
			writeEnvelope(w, http.StatusOK, 200, "success", operationSessionRawUser(value))
			return
		}
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *OperationH5Handler) authCallback(w http.ResponseWriter, r *http.Request, module string, params map[string]any, sessionID string, code string, target string) {
	info, found := h.officialAccountInfoForCallback(w, r, module, params)
	if !found {
		return
	}
	rawUser, err := h.oauthClient.OAuthUser(r.Context(), info, code)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	sessionValue := map[string]any{"raw": rawUser}
	for _, key := range operationWechatUserKeys(info) {
		if err := h.session.SetOperationSessionValue(r.Context(), sessionID, key, sessionValue, h.sessionTTL); err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
	}
	http.Redirect(w, r, h.normalizeTarget(target), http.StatusFound)
}

func (h *OperationH5Handler) officialAccountInfoForCallback(w http.ResponseWriter, r *http.Request, module string, params map[string]any) (OfficialAccountOAuthInfo, bool) {
	if appID := stringParam(params, "appid"); appID != "" {
		info, found, err := h.store.OfficialAccountOAuthInfoByAuthorizerAppID(r.Context(), appID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return OfficialAccountOAuthInfo{}, false
		}
		if found {
			return info, true
		}
	}
	return h.officialAccountInfo(w, r, module, params)
}

func (h *OperationH5Handler) officialAccountInfo(w http.ResponseWriter, r *http.Request, module string, params map[string]any) (OfficialAccountOAuthInfo, bool) {
	id := operationFirstPositiveInt(params, "id", "corpId", "corp_id", "fission_id", "clockInId", "clock_in_id")
	if id <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "数据不存在", nil)
		return OfficialAccountOAuthInfo{}, false
	}
	info, found, err := h.store.OperationOfficialAccountInfo(r.Context(), module, id, params)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return OfficialAccountOAuthInfo{}, false
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "公众号配置错误或不存在", nil)
		return OfficialAccountOAuthInfo{}, false
	}
	return info, true
}

func (h *OperationH5Handler) hasAuthorizedUser(w http.ResponseWriter, r *http.Request, sessionID string, info OfficialAccountOAuthInfo) bool {
	if h.session == nil {
		return false
	}
	for _, key := range operationWechatUserKeys(info) {
		_, found, err := h.session.GetOperationSessionValue(r.Context(), sessionID, key)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return false
		}
		if found {
			return true
		}
	}
	return false
}

func (h *OperationH5Handler) ensureSessionID(w http.ResponseWriter, r *http.Request) (string, bool) {
	if sessionID := operationSessionID(r); sessionID != "" {
		return sessionID, true
	}
	sessionID, err := newOperationSessionID()
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return "", false
	}
	http.SetCookie(w, &http.Cookie{
		Name:     operationSessionCookieName,
		Value:    sessionID,
		Path:     "/",
		MaxAge:   int(h.sessionTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return sessionID, true
}

func (h *OperationH5Handler) normalizeTarget(target string) string {
	if strings.Contains(target, "http") {
		return target
	}
	if h.operationBaseURL == "" {
		return target
	}
	return h.operationBaseURL + target
}

func (h *OperationH5Handler) operationParams(w http.ResponseWriter, r *http.Request) (map[string]any, bool) {
	params := valuesToParams(r.URL.Query())
	if r.Method == http.MethodGet {
		return params, true
	}
	body, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, 400, "invalid request body", nil)
		return nil, false
	}
	for key, value := range body {
		params[key] = value
	}
	return params, true
}

func (h *OperationH5Handler) requiredID(w http.ResponseWriter, params map[string]any, keys ...string) (int, bool) {
	id := operationFirstPositiveInt(params, keys...)
	if id <= 0 {
		writeEnvelope(w, http.StatusBadRequest, 400, "id 必填", nil)
		return 0, false
	}
	return id, true
}

func (h *OperationH5Handler) fullStaticURL(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") || h.apiBaseURL == "" {
		return path
	}
	return h.apiBaseURL + "/static/" + strings.TrimLeft(strings.TrimPrefix(path, "static/"), "/")
}

func (h *OperationH5Handler) roomFissionShareURL(fissionID int, parentUnionID string) string {
	target := fmt.Sprintf("/roomFission?id=%d&parent_union_id=%s&wx_user_id=0", fissionID, url.QueryEscape(parentUnionID))
	base := h.operationBaseURL
	if base == "" {
		base = "/operation"
	}
	return fmt.Sprintf("%s/auth/roomFission?id=%d&target=%s", base, fissionID, url.QueryEscape(target))
}

func (h *OperationH5Handler) operationRoomFissionRoomPayload(room RoomFissionRoom) map[string]any {
	payload := roomFissionRoomPayload(room)
	payload["roomQrcode"] = h.fullStaticURL(room.RoomQRCode)
	payload["room_qrcode"] = payload["roomQrcode"]
	if raw, ok := jsonPayload(room.RoomRaw).(map[string]any); ok {
		payload["name"] = operationNonEmpty(fmt.Sprint(raw["name"]), fmt.Sprint(raw["roomName"]), fmt.Sprint(raw["room_name"]))
		payload["roomName"] = payload["name"]
	}
	return payload
}

func operationWechatUserFromParams(params map[string]any) OperationWechatUser {
	if nested, ok := operationMapParam(params, "userInfo", "user", "wechatUser", "wxUser"); ok {
		for key, value := range nested {
			if _, exists := params[key]; !exists {
				params[key] = value
			}
		}
	}
	user := OperationWechatUser{
		UnionID:  operationFirstString(params, "union_id", "unionId", "unionid"),
		OpenID:   operationFirstString(params, "openid", "open_id", "openId"),
		Nickname: operationFirstString(params, "nickname", "name"),
		Avatar:   operationFirstString(params, "avatar", "headimgurl", "avatarUrl", "avatar_url"),
		City:     operationFirstString(params, "city"),
		Source:   operationFirstString(params, "source"),
	}
	if user.UnionID == "" {
		user.UnionID = user.OpenID
	}
	return user
}

func operationUnionIDFromParams(params map[string]any) string {
	user := operationWechatUserFromParams(params)
	return user.UnionID
}

func operationMapParam(params map[string]any, keys ...string) (map[string]any, bool) {
	for _, key := range keys {
		value, ok := params[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case map[string]any:
			return typed, true
		case string:
			var decoded map[string]any
			if err := json.Unmarshal([]byte(typed), &decoded); err == nil {
				return decoded, true
			}
		}
	}
	return nil, false
}

func operationFirstString(params map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := stringParam(params, key); value != "" {
			return value
		}
	}
	return ""
}

func operationFirstPositiveInt(params map[string]any, keys ...string) int {
	for _, key := range keys {
		value, ok, err := intParam(params, key)
		if err == nil && ok && value > 0 {
			return value
		}
	}
	return 0
}

func operationContactUnion(contact RoomFissionContactItem, user OperationWechatUser) string {
	if strings.TrimSpace(contact.UnionID) != "" {
		return contact.UnionID
	}
	return user.UnionID
}

func operationLotteryPrize(prize LotteryPrizeItem) (int, string) {
	items, _ := workFissionOperationJSONList(prize.PrizeSetRaw)
	for index, item := range items {
		name := operationNonEmpty(stringParam(item, "name"), stringParam(item, "title"), stringParam(item, "prizeName"))
		if name == "" {
			name = fmt.Sprintf("奖品%d", index+1)
		}
		return prize.ID, name
	}
	return prize.ID, "谢谢参与"
}

func operationLotteryExchange(raw string) (int, string, string) {
	value := jsonPayload(raw)
	item, _ := value.(map[string]any)
	receiveType := 0
	if typed, ok := item["type"]; ok {
		receiveType, _, _ = intParam(map[string]any{"type": typed}, "type")
	}
	receiveQR := operationValueByKeys(item, "receiveQr", "receive_qr", "qrCode", "qr_code", "qrcode", "qrcodeUrl", "url")
	receiveCode := operationValueByKeys(item, "receiveCode", "receive_code", "code", "exchangeCode", "exchange_code")
	if receiveType <= 0 {
		if receiveQR != "" {
			receiveType = 1
		} else if receiveCode != "" {
			receiveType = 2
		}
	}
	return receiveType, receiveQR, receiveCode
}

func operationLotteryRecordsPayload(records []OperationLotteryRecord, fullURL func(string) string) []map[string]any {
	payload := make([]map[string]any, 0, len(records))
	for _, record := range records {
		payload = append(payload, operationLotteryRecordPayload(record, fullURL))
	}
	return payload
}

func operationLotteryRecordPayload(record OperationLotteryRecord, fullURL func(string) string) map[string]any {
	return map[string]any{
		"id":             record.ID,
		"lottery_id":     record.LotteryID,
		"lotteryId":      record.LotteryID,
		"contact_id":     record.ContactID,
		"contactId":      record.ContactID,
		"prize_id":       record.PrizeID,
		"prizeId":        record.PrizeID,
		"prize_name":     record.PrizeName,
		"prizeName":      record.PrizeName,
		"receive_status": record.ReceiveStatus,
		"receiveStatus":  record.ReceiveStatus,
		"receive_qr":     fullURL(record.ReceiveQR),
		"receiveQr":      fullURL(record.ReceiveQR),
		"receive_type":   record.ReceiveType,
		"receiveType":    record.ReceiveType,
		"receive_code":   record.ReceiveCode,
		"receiveCode":    record.ReceiveCode,
		"write_off":      record.WriteOff,
		"writeOff":       record.WriteOff,
		"createdAt":      record.CreatedAt,
		"updatedAt":      record.UpdatedAt,
	}
}

func operationLotteryLatestMessage(records []OperationLotteryRecord, fullURL func(string) string) any {
	if len(records) == 0 {
		return map[string]any{}
	}
	return operationLotteryRecordPayload(records[0], fullURL)
}

func operationClockInStatus(records []RoomClockInDayRecord) int {
	today := time.Now().Format("2006-01-02")
	for _, record := range records {
		if record.Day == today {
			return 1
		}
	}
	return 0
}

func operationClockInDayDetail(records []RoomClockInDayRecord) [][]string {
	result := make([][]string, 12)
	for i := range result {
		result[i] = []string{}
	}
	for _, record := range records {
		parsed, err := time.Parse("2006-01-02", record.Day)
		if err != nil {
			continue
		}
		month := int(parsed.Month()) - 1
		if month >= 0 && month < len(result) {
			result[month] = append(result[month], record.Day)
		}
	}
	return result
}

func operationRoomClockInTasks(raw string, receiveLevel int, dayCount int) []map[string]any {
	items, _ := workFissionOperationJSONList(raw)
	tasks := make([]map[string]any, 0, len(items))
	for index, item := range items {
		count := operationTaskCount(item)
		taskStatus := 0
		if dayCount >= count {
			taskStatus = 1
		}
		receiveStatus := 0
		if receiveLevel >= index+1 {
			receiveStatus = 1
		}
		item["count"] = count
		item["task_status"] = taskStatus
		item["taskStatus"] = taskStatus
		item["receive_status"] = receiveStatus
		item["receiveStatus"] = receiveStatus
		tasks = append(tasks, item)
	}
	return tasks
}

func operationTaskCount(item map[string]any) int {
	for _, key := range []string{"count", "day_count", "dayCount", "num"} {
		value, ok, err := intParam(item, key)
		if err == nil && ok && value > 0 {
			return value
		}
	}
	return 0
}

func operationRoomFissionEnded(item RoomFissionInfo) bool {
	if item.Status != 1 {
		return true
	}
	if strings.TrimSpace(item.EndTime) == "" {
		return false
	}
	parsed, err := time.ParseInLocation("2006-01-02 15:04:05", item.EndTime, time.Local)
	if err != nil {
		return false
	}
	return time.Now().After(parsed)
}

func operationFirstRoom(rooms []RoomFissionRoom, roomID int) (RoomFissionRoom, bool) {
	if roomID > 0 {
		for _, room := range rooms {
			if room.ID == roomID {
				return room, true
			}
		}
	}
	if len(rooms) == 0 {
		return RoomFissionRoom{}, false
	}
	return rooms[0], true
}

func operationRoomFissionReceiveQRCode(bundle RoomFissionBundle) string {
	if qr := operationFirstQRCode(bundle.Fission.ReceiveEmployeesRaw); qr != "" {
		return qr
	}
	if bundle.Invite.LinkPic != "" {
		return bundle.Invite.LinkPic
	}
	if bundle.Welcome.LinkPic != "" {
		return bundle.Welcome.LinkPic
	}
	if len(bundle.Rooms) > 0 {
		return bundle.Rooms[0].RoomQRCode
	}
	return ""
}

func operationFirstQRCode(raw string) string {
	value := jsonPayload(raw)
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			if object, ok := item.(map[string]any); ok {
				if value := operationValueByKeys(object, "qrcode", "qrCode", "qr_code", "qrcodeUrl", "qrcode_url", "roomQrcode", "room_qrcode", "roomQrcodeUrl", "url"); value != "" {
					return value
				}
			}
			if text := strings.TrimSpace(fmt.Sprint(item)); text != "" {
				return text
			}
		}
	case map[string]any:
		return operationValueByKeys(typed, "qrcode", "qrCode", "qr_code", "qrcodeUrl", "qrcode_url", "roomQrcode", "room_qrcode", "roomQrcodeUrl", "url")
	case string:
		return typed
	}
	return ""
}

func operationShopProvincePayload(cities []ShopCodeCity) []map[string]any {
	byProvince := map[string]map[string]struct{}{}
	order := make([]string, 0)
	for _, city := range cities {
		province := strings.TrimSpace(city.Province)
		if province == "" {
			continue
		}
		if _, ok := byProvince[province]; !ok {
			byProvince[province] = map[string]struct{}{}
			order = append(order, province)
		}
		if strings.TrimSpace(city.City) != "" {
			byProvince[province][city.City] = struct{}{}
		}
	}
	result := make([]map[string]any, 0, len(order))
	for _, province := range order {
		cityItems := make([]map[string]any, 0, len(byProvince[province]))
		for city := range byProvince[province] {
			cityItems = append(cityItems, map[string]any{"city": city})
		}
		result = append(result, map[string]any{"province": province, "city": cityItems})
	}
	return result
}

func operationValueByKeys(item map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func operationNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" && strings.TrimSpace(value) != "<nil>" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func operationSHA1(value string) string {
	sum := sha1.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}
