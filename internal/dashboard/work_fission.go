package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type WorkFissionListFilter struct {
	CorpID             int
	CreateUserID       int
	RestrictCreateUser bool
	ActiveName         string
	Page               int
	PerPage            int
}

type WorkFissionListPage struct {
	Items     []WorkFissionListItem
	Total     int
	TotalPage int
	PerPage   int
}

type WorkFissionListItem struct {
	ID               int
	ActiveName       string
	ServiceEmployees string
	ContactTags      string
	Tasks            string
	EndTime          string
	CreatedAt        string
	EmployeeNum      int
}

type WorkFissionBundle struct {
	Fission WorkFissionInfo
	Poster  WorkFissionPoster
	Welcome WorkFissionWelcome
	Push    WorkFissionPush
	Invite  WorkFissionInvite
}

type WorkFissionInfo struct {
	ID                    int
	CorpID                int
	ActiveName            string
	ServiceEmployees      string
	AutoPass              int
	AutoAddTag            int
	ContactTags           string
	EndTime               string
	QRCodeInvalid         int
	Tasks                 string
	NewFriend             int
	DeleteInvalid         int
	ReceivePrize          int
	ReceivePrizeEmployees string
	ReceiveLinks          string
	CreatedAt             string
	CreateUserID          int
}

type WorkFissionPoster struct {
	PosterType        int
	CoverPic          string
	WXCoverPic        string
	FowardText        string
	AvatarShow        int
	NicknameShow      int
	NicknameColor     string
	CardCorpImageName string
	CardCorpName      string
	CardCorpLogo      string
	QRCodeW           string
	QRCodeH           string
	QRCodeX           string
	QRCodeY           string
	QRCodeID          string
	QRCodeURL         string
}

type WorkFissionWelcome struct {
	MsgText      string
	LinkTitle    string
	LinkDesc     string
	LinkCoverURL string
}

type WorkFissionPush struct {
	PushEmployee   int
	PushContact    int
	MsgText        string
	MsgComplex     string
	MsgComplexType string
}

type WorkFissionInvite struct {
	Text      string
	LinkTitle string
	LinkDesc  string
	LinkPic   string
}

type WorkFissionInviteWrite struct {
	FissionID    int
	Text         string
	LinkTitle    string
	LinkDesc     string
	LinkPic      string
	CreateUserID int
}

type WorkFissionWrite struct {
	Fission WorkFissionBaseWrite
	Poster  WorkFissionPosterWrite
	Welcome WorkFissionWelcomeWrite
	Push    WorkFissionPushWrite
	Invite  WorkFissionInviteCreateWrite
}

type WorkFissionBaseWrite struct {
	CorpID                int
	ActiveName            string
	ServiceEmployees      string
	AutoPass              int
	AutoAddTag            int
	ContactTags           string
	EndTime               string
	QRCodeInvalid         int
	Tasks                 string
	NewFriend             int
	DeleteInvalid         int
	ReceivePrize          int
	ReceivePrizeEmployees string
	ReceiveLinks          string
	ReceiveQRCode         string
	CreateUserID          int
}

type WorkFissionPosterWrite struct {
	PosterType        int
	CoverPic          string
	WXCoverPic        string
	FowardText        string
	AvatarShow        int
	NicknameShow      int
	NicknameColor     string
	CardCorpImageName string
	CardCorpName      string
	CardCorpLogo      string
	QRCodeW           string
	QRCodeH           string
	QRCodeX           string
	QRCodeY           string
	QRCodeID          string
	QRCodeURL         string
}

type WorkFissionWelcomeWrite struct {
	MsgText      string
	LinkTitle    string
	LinkDesc     string
	LinkCoverURL string
	LinkWXURL    string
}

type WorkFissionPushWrite struct {
	PushEmployee   int
	PushContact    int
	MsgText        string
	MsgComplex     string
	MsgComplexType string
}

type WorkFissionInviteCreateWrite struct {
	Text      string
	LinkTitle string
	LinkDesc  string
	LinkPic   string
	WXLinkPic string
}

type WorkFissionName struct {
	ID         int    `json:"id"`
	ActiveName string `json:"active_name"`
}

type WorkFissionStatistics struct {
	FirstUserCount     int
	UserCount          int
	LossCount          int
	TodayIncreaseCount int
	NewIncreaseCount   int
	NewLossCount       int
	InviteCount        int
	FirstLevelCount    int
	SecondLevelCount   int
	ThirdLevelCount    int
	Active             []WorkFissionName
	ServiceEmployees   []string
}

type WorkFissionChooseContactFilter struct {
	CorpID      int
	EmployeeIDs []int
	IsAll       int
	StartTime   string
	EndTime     string
	Gender      *int
}

type WorkFissionInviteDataFilter struct {
	CorpID     int
	FissionIDs []int
	Nickname   string
	Employee   string
	StartTime  string
	EndTime    string
	Status     *int
	Loss       *int
	Page       int
	PerPage    int
}

type WorkFissionInviteDataPage struct {
	Items     []WorkFissionInviteDataItem
	Total     int
	TotalPage int
	PerPage   int
}

type WorkFissionInviteDataItem struct {
	ID           int
	Nickname     string
	Avatar       string
	ActiveName   string
	Employee     string
	EmployeeName string
	CreatedAt    string
	Loss         int
	Level        int
	Status       int
	InviteCount  int
	ContactID    int
	EmployeeID   int
}

type WorkFissionInviteDetail struct {
	TotalCount int
	NewCount   int
	LossCount  int
	Children   []WorkFissionInviteDetailChild
}

type WorkFissionInviteDetailChild struct {
	ID        int
	Nickname  string
	Avatar    string
	Loss      int
	CreatedAt string
}

type WorkFissionStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	WorkFissionPage(ctx context.Context, filter WorkFissionListFilter) (WorkFissionListPage, error)
	WorkFissionBundleByID(ctx context.Context, corpID int, id int) (WorkFissionBundle, bool, error)
	WorkFissionStatistics(ctx context.Context, corpID int, fissionIDs []int) (WorkFissionStatistics, error)
	WorkFissionChooseContactCount(ctx context.Context, filter WorkFissionChooseContactFilter) (int, error)
	WorkFissionInviteDataPage(ctx context.Context, filter WorkFissionInviteDataFilter) (WorkFissionInviteDataPage, error)
	WorkFissionInviteDetail(ctx context.Context, corpID int, id int) (WorkFissionInviteDetail, bool, error)
	WorkFissionInviteTargets(ctx context.Context, filter WorkFissionChooseContactFilter) ([]string, error)
	CreateWorkFission(ctx context.Context, values WorkFissionWrite) (int, error)
	UpdateWorkFission(ctx context.Context, corpID int, id int, values WorkFissionWrite) (bool, error)
	UpsertWorkFissionInvite(ctx context.Context, values WorkFissionInviteWrite) error
	DeleteWorkFissionCascade(ctx context.Context, corpID int, id int) (bool, error)
	RoomWelcomeCorpCredentialByID(ctx context.Context, corpID int) (RoomWelcomeCorpCredential, bool, error)
}

type WorkFissionInviteMessageClient interface {
	UploadImage(ctx context.Context, credential RoomWelcomeCorpCredential, filePath string) (string, error)
	CreateContactWay(ctx context.Context, credential RoomWelcomeCorpCredential, userIDs []string, skipVerify bool, state string) (string, string, error)
	SubmitContactMessageBatchSend(ctx context.Context, credential RoomWelcomeCorpCredential, payload ContactMessageBatchSendMessagePayload) (ContactMessageBatchSendMessageResult, error)
}

type WorkFissionHandler struct {
	store            WorkFissionStore
	cache            LoginCache
	resolver         UserIDResolver
	authorizer       CorpAdminAuthorizer
	messageClient    WorkFissionInviteMessageClient
	apiBaseURL       string
	operationBaseURL string
	fileStorageRoot  string
}

func NewWorkFissionHandler(store WorkFissionStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer, apiBaseURL string, operationBaseURL string) *WorkFissionHandler {
	return NewWorkFissionHandlerWithMessageClient(store, cache, resolver, authorizer, apiBaseURL, operationBaseURL, defaultRoomTagPullFileStorageRoot, NewRoomWelcomeWeComClient(defaultWeComAPIBaseURL))
}

func NewWorkFissionHandlerWithMessageClient(store WorkFissionStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer, apiBaseURL string, operationBaseURL string, fileStorageRoot string, messageClient WorkFissionInviteMessageClient) *WorkFissionHandler {
	if fileStorageRoot == "" {
		fileStorageRoot = defaultRoomTagPullFileStorageRoot
	}
	return &WorkFissionHandler{
		store:            store,
		cache:            cache,
		resolver:         resolver,
		authorizer:       authorizer,
		messageClient:    messageClient,
		apiBaseURL:       strings.TrimRight(apiBaseURL, "/"),
		operationBaseURL: strings.TrimRight(operationBaseURL, "/"),
		fileStorageRoot:  fileStorageRoot,
	}
}

func (h *WorkFissionHandler) Index(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, user, _, ok := h.resolveAuthorized(w, r)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	page, err := h.store.WorkFissionPage(r.Context(), WorkFissionListFilter{
		CorpID:             corpID,
		CreateUserID:       userID,
		RestrictCreateUser: user.IsSuperAdmin == 0,
		ActiveName:         strings.TrimSpace(r.URL.Query().Get("active_name")),
		Page:               positiveQueryInt(r, "page", 1),
		PerPage:            positiveQueryInt(r, "perPage", 10000),
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, map[string]any{
			"id":                item.ID,
			"tasks":             item.Tasks,
			"active_name":       item.ActiveName,
			"service_employees": item.ServiceEmployees,
			"contact_tags":      item.ContactTags,
			"created_at":        item.CreatedAt,
			"status":            workFissionStatus(item.EndTime),
			"finance_tag":       "1/2",
			"employeeNum":       item.EmployeeNum,
		})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"page": map[string]any{"perPage": page.PerPage, "total": page.Total, "totalPage": page.TotalPage},
		"list": list,
	})
}

func (h *WorkFissionHandler) Show(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, ok := h.resolveAuthorized(w, r)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	id, ok := queryPositiveIntParam(w, r, "id", "任务宝ID 必填", "任务宝ID 必需为整数")
	if !ok {
		return
	}
	bundle, found, err := h.store.WorkFissionBundleByID(r.Context(), corpID, id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "任务宝不存在", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"id":                bundle.Fission.ID,
		"active_name":       bundle.Fission.ActiveName,
		"qrcode_url":        bundle.Poster.QRCodeURL,
		"link":              h.authRedirectURL(id),
		"active_time":       bundle.Fission.CreatedAt + "-" + bundle.Fission.EndTime,
		"service_employees": bundle.Fission.ServiceEmployees,
		"contact_tags":      bundle.Fission.ContactTags,
		"welcome_text":      bundle.Welcome.MsgText,
		"welcome_title":     bundle.Welcome.LinkTitle,
		"welcome_desc":      bundle.Welcome.LinkDesc,
		"welcome_url":       bundle.Welcome.LinkCoverURL,
	})
}

func (h *WorkFissionHandler) Info(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, ok := h.resolveAuthorized(w, r)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	id, ok := queryPositiveIntParam(w, r, "id", "活动ID 必填", "活动ID 必需为整数")
	if !ok {
		return
	}
	bundle, found, err := h.store.WorkFissionBundleByID(r.Context(), corpID, id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "活动不存在", nil)
		return
	}
	complex, err := h.pushComplexPayload(bundle.Push.MsgComplex)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"fission": map[string]any{
			"id":                      bundle.Fission.ID,
			"active_name":             bundle.Fission.ActiveName,
			"service_employees":       bundle.Fission.ServiceEmployees,
			"auto_pass":               boolString(bundle.Fission.AutoPass == 1),
			"auto_add_tag":            boolString(bundle.Fission.AutoAddTag == 1),
			"contact_tags":            bundle.Fission.ContactTags,
			"end_time":                bundle.Fission.EndTime,
			"qr_code_invalid":         bundle.Fission.QRCodeInvalid,
			"tasks":                   bundle.Fission.Tasks,
			"new_friend":              boolString(bundle.Fission.NewFriend == 1),
			"delete_invalid":          boolString(bundle.Fission.DeleteInvalid == 1),
			"receive_prize":           bundle.Fission.ReceivePrize,
			"receive_prize_employees": bundle.Fission.ReceivePrizeEmployees,
			"receive_links":           bundle.Fission.ReceiveLinks,
		},
		"welcome": map[string]any{
			"msg_text":       bundle.Welcome.MsgText,
			"link_title":     bundle.Welcome.LinkTitle,
			"link_desc":      bundle.Welcome.LinkDesc,
			"link_cover_url": h.fullStaticURL(bundle.Welcome.LinkCoverURL),
		},
		"poster": map[string]any{
			"poster_type":          bundle.Poster.PosterType,
			"cover_pic":            h.fullStaticURL(bundle.Poster.CoverPic),
			"wx_cover_pic":         bundle.Poster.WXCoverPic,
			"foward_text":          bundle.Poster.FowardText,
			"avatar_show":          boolString(bundle.Poster.AvatarShow == 1),
			"nickname_show":        boolString(bundle.Poster.NicknameShow == 1),
			"nickname_color":       bundle.Poster.NicknameColor,
			"card_corp_image_name": bundle.Poster.CardCorpImageName,
			"card_corp_name":       bundle.Poster.CardCorpName,
			"card_corp_logo":       bundle.Poster.CardCorpLogo,
			"qrcode_w":             bundle.Poster.QRCodeW,
			"qrcode_h":             bundle.Poster.QRCodeH,
			"qrcode_x":             bundle.Poster.QRCodeX,
			"qrcode_y":             bundle.Poster.QRCodeY,
			"qrcode_id":            bundle.Poster.QRCodeID,
			"qrcode_url":           bundle.Poster.QRCodeURL,
		},
		"push": map[string]any{
			"push_employee":    boolString(bundle.Push.PushEmployee == 1),
			"push_contact":     boolString(bundle.Push.PushContact == 1),
			"msg_text":         bundle.Push.MsgText,
			"msg_complex":      complex,
			"msg_complex_type": bundle.Push.MsgComplexType,
		},
		"invite": map[string]any{
			"text":       bundle.Invite.Text,
			"link_title": bundle.Invite.LinkTitle,
			"link_desc":  bundle.Invite.LinkDesc,
			"link_pic":   h.fullStaticURL(bundle.Invite.LinkPic),
		},
	})
}

func (h *WorkFissionHandler) Statistics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, ok := h.resolveAuthorized(w, r)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	fissionIDs, ok := intJSONQuery(w, r, "fission_ids", "fission_ids 必须为整数数组")
	if !ok {
		return
	}
	stats, err := h.store.WorkFissionStatistics(r.Context(), corpID, fissionIDs)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	userCount := stats.UserCount
	lossCount := stats.LossCount
	insertCount := userCount - lossCount
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"user": map[string]any{
			"user_count":           userCount,
			"loss_count":           lossCount,
			"insert_count":         insertCount,
			"today_increase_count": stats.TodayIncreaseCount,
			"new_increase_count":   stats.NewIncreaseCount,
			"new_loss_count":       stats.NewLossCount,
			"net_increase":         stats.NewIncreaseCount - stats.NewLossCount,
			"invite_count":         stats.InviteCount,
			"fission_rate":         workFissionRate(userCount-stats.FirstUserCount, userCount),
			"insert_rate":          workFissionRate(insertCount, userCount),
			"share_rate":           workFissionRate(stats.InviteCount, userCount),
		},
		"active": stats.Active,
		"level": map[string]any{
			"first_level":  stats.FirstLevelCount,
			"second_level": stats.SecondLevelCount,
			"third_level":  stats.ThirdLevelCount,
		},
		"employee": workFissionUniqueEmployees(stats.ServiceEmployees),
	})
}

func (h *WorkFissionHandler) ChooseContact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, ok := h.resolveAuthorized(w, r)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	employeeIDs, ok := intJSONQuery(w, r, "employee_ids", "employee_ids 必须为整数数组")
	if !ok {
		return
	}
	isAll, ok := optionalQueryInt(w, r, "is_all", "is_all 必须为整数")
	if !ok {
		return
	}
	gender, ok := optionalQueryInt(w, r, "gender", "gender 必须为整数")
	if !ok {
		return
	}
	isAllValue := 0
	if isAll != nil {
		isAllValue = *isAll
	}
	if isAllValue == 0 || gender == nil {
		gender = nil
	}
	count, err := h.store.WorkFissionChooseContactCount(r.Context(), WorkFissionChooseContactFilter{
		CorpID:      corpID,
		EmployeeIDs: employeeIDs,
		IsAll:       isAllValue,
		StartTime:   strings.TrimSpace(r.URL.Query().Get("start_time")),
		EndTime:     strings.TrimSpace(r.URL.Query().Get("end_time")),
		Gender:      gender,
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{count})
}

func (h *WorkFissionHandler) Store(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, user, _, ok := h.resolveAuthorized(w, r)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	if !enforceSaaSQuota(r.Context(), w, h.store, user.TenantID, SaaSMetricWorkFissions, 1) {
		return
	}
	values, ok := h.workFissionWriteValues(w, r, userID, corpID, params, true)
	if !ok {
		return
	}
	id, err := h.store.CreateWorkFission(r.Context(), values)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if err := refreshSaaSUsageCounter(r.Context(), h.store, user.TenantID, SaaSMetricWorkFissions); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{id})
}

func (h *WorkFissionHandler) Update(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, _, ok := h.resolveAuthorized(w, r)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	fissionParams, ok := workFissionMapParam(params, "fission")
	if !ok {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "fission 必填", nil)
		return
	}
	id, has, err := intParam(fissionParams, "id")
	if err != nil || !has || id <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "活动ID 必须为整数", nil)
		return
	}
	values, ok := h.workFissionWriteValues(w, r, userID, corpID, params, false)
	if !ok {
		return
	}
	updated, err := h.store.UpdateWorkFission(r.Context(), corpID, id, values)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !updated {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "活动不存在", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *WorkFissionHandler) Invite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, _, ok := h.resolveAuthorized(w, r)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	values, filter, ok := h.workFissionInviteValues(w, userID, corpID, params)
	if !ok {
		return
	}
	targets, err := h.store.WorkFissionInviteTargets(r.Context(), filter)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if len(targets) == 0 {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "没有客户", nil)
		return
	}
	if h.messageClient == nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "企业微信客户端未配置", nil)
		return
	}
	credential, found, err := h.store.RoomWelcomeCorpCredentialByID(r.Context(), corpID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "企业微信配置不存在", nil)
		return
	}
	linkPicURL := values.LinkPic
	if linkPicURL != "" && !strings.HasPrefix(linkPicURL, "http://") && !strings.HasPrefix(linkPicURL, "https://") {
		linkPicURL = h.fullStaticURL(linkPicURL)
	}
	if _, err := h.messageClient.SubmitContactMessageBatchSend(r.Context(), credential, ContactMessageBatchSendMessagePayload{
		Content: []ContactMessageBatchSendContent{
			{MsgType: "text", Content: values.Text},
			{MsgType: "link", Title: values.LinkTitle, Desc: values.LinkDesc, PicURL: linkPicURL, URL: h.authRedirectURL(values.FissionID)},
		},
		ExternalUserID: targets,
	}); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "发送失败", nil)
		return
	}
	if err := h.store.UpsertWorkFissionInvite(r.Context(), values); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *WorkFissionHandler) workFissionWriteValues(w http.ResponseWriter, r *http.Request, userID int, corpID int, params map[string]any, includeInvite bool) (WorkFissionWrite, bool) {
	fissionParams, ok := workFissionMapParam(params, "fission")
	if !ok {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "fission 必填", nil)
		return WorkFissionWrite{}, false
	}
	welcomeParams, ok := workFissionMapParam(params, "welcome")
	if !ok {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "welcome 必填", nil)
		return WorkFissionWrite{}, false
	}
	posterParams, ok := workFissionMapParam(params, "poster")
	if !ok {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "poster 必填", nil)
		return WorkFissionWrite{}, false
	}
	pushParams, ok := workFissionMapParam(params, "push")
	if !ok {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "push 必填", nil)
		return WorkFissionWrite{}, false
	}

	credential, found, err := h.store.RoomWelcomeCorpCredentialByID(r.Context(), corpID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return WorkFissionWrite{}, false
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "企业微信配置不存在", nil)
		return WorkFissionWrite{}, false
	}

	serviceEmployees, err := workFissionJSONParam(fissionParams, "service_employees")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "service_employees 必须为 JSON", nil)
		return WorkFissionWrite{}, false
	}
	contactTags, err := workFissionJSONParam(fissionParams, "contact_tags")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "contact_tags 必须为 JSON", nil)
		return WorkFissionWrite{}, false
	}
	tasks, err := workFissionJSONParam(fissionParams, "tasks")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "tasks 必须为 JSON", nil)
		return WorkFissionWrite{}, false
	}
	receivePrizeEmployees, err := workFissionJSONParam(fissionParams, "receive_prize_employees")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "receive_prize_employees 必须为 JSON", nil)
		return WorkFissionWrite{}, false
	}
	receiveLinks, err := workFissionJSONParam(fissionParams, "receive_links")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "receive_links 必须为 JSON", nil)
		return WorkFissionWrite{}, false
	}
	autoPass := workFissionBoolInt(fissionParams, "auto_pass")
	receivePrize := workFissionIntValue(fissionParams, "receive_prize")
	receiveQRCode := "[]"
	if receivePrize == 0 {
		if h.messageClient == nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "企业微信客户端未配置", nil)
			return WorkFissionWrite{}, false
		}
		wxUserIDs, err := workFissionWXUserIDs(fissionParams, "receive_prize_employees")
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "receive_prize_employees 必须包含 wxUserId", nil)
			return WorkFissionWrite{}, false
		}
		if len(wxUserIDs) > 0 {
			qrCodeURL, configID, err := h.messageClient.CreateContactWay(r.Context(), credential, wxUserIDs, autoPass == 1, "")
			if err != nil {
				writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "生成二维码失败", nil)
				return WorkFissionWrite{}, false
			}
			receiveQRCode = workFissionMustJSON(map[string]any{"qrId": configID, "url": qrCodeURL})
		}
	}

	welcomeCover, welcomeWXURL, ok := h.workFissionStoredImage(w, r, credential, stringParam(welcomeParams, "link_cover_url"), true)
	if !ok {
		return WorkFissionWrite{}, false
	}
	posterCover, _, ok := h.workFissionStoredImage(w, r, credential, stringParam(posterParams, "cover_pic"), false)
	if !ok {
		return WorkFissionWrite{}, false
	}
	cardLogo, _, ok := h.workFissionStoredImage(w, r, credential, stringParam(posterParams, "card_corp_logo"), false)
	if !ok {
		return WorkFissionWrite{}, false
	}
	msgComplex, msgComplexType, ok := h.workFissionPushComplex(w, r, credential, pushParams, includeInvite)
	if !ok {
		return WorkFissionWrite{}, false
	}

	values := WorkFissionWrite{
		Fission: WorkFissionBaseWrite{
			CorpID:                corpID,
			ActiveName:            stringParam(fissionParams, "active_name"),
			ServiceEmployees:      serviceEmployees,
			AutoPass:              autoPass,
			AutoAddTag:            workFissionBoolInt(fissionParams, "auto_add_tag"),
			ContactTags:           contactTags,
			EndTime:               stringParam(fissionParams, "end_time"),
			QRCodeInvalid:         workFissionIntValue(fissionParams, "qr_code_invalid"),
			Tasks:                 tasks,
			NewFriend:             workFissionBoolInt(fissionParams, "new_friend"),
			DeleteInvalid:         workFissionBoolInt(fissionParams, "delete_invalid"),
			ReceivePrize:          receivePrize,
			ReceivePrizeEmployees: receivePrizeEmployees,
			ReceiveLinks:          receiveLinks,
			ReceiveQRCode:         receiveQRCode,
			CreateUserID:          userID,
		},
		Welcome: WorkFissionWelcomeWrite{
			MsgText:      stringParam(welcomeParams, "msg_text"),
			LinkTitle:    stringParam(welcomeParams, "link_title"),
			LinkDesc:     stringParam(welcomeParams, "link_desc"),
			LinkCoverURL: welcomeCover,
			LinkWXURL:    welcomeWXURL,
		},
		Poster: WorkFissionPosterWrite{
			PosterType:        workFissionIntValue(posterParams, "poster_type"),
			CoverPic:          posterCover,
			FowardText:        stringParam(posterParams, "foward_text"),
			AvatarShow:        workFissionBoolInt(posterParams, "avatar_show"),
			NicknameShow:      workFissionBoolInt(posterParams, "nickname_show"),
			NicknameColor:     stringParam(posterParams, "nickname_color"),
			CardCorpImageName: stringParam(posterParams, "card_corp_image_name"),
			CardCorpName:      stringParam(posterParams, "card_corp_name"),
			CardCorpLogo:      cardLogo,
			QRCodeW:           stringParam(posterParams, "qrcode_w"),
			QRCodeH:           stringParam(posterParams, "qrcode_h"),
			QRCodeX:           stringParam(posterParams, "qrcode_x"),
			QRCodeY:           stringParam(posterParams, "qrcode_y"),
		},
		Push: WorkFissionPushWrite{
			PushEmployee:   workFissionBoolInt(pushParams, "push_employee"),
			PushContact:    workFissionBoolInt(pushParams, "push_contact"),
			MsgText:        stringParam(pushParams, "msg_text"),
			MsgComplex:     msgComplex,
			MsgComplexType: msgComplexType,
		},
	}
	if includeInvite {
		inviteParams, ok := workFissionMapParam(params, "invite")
		if !ok {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invite 必填", nil)
			return WorkFissionWrite{}, false
		}
		linkPic, wxLinkPic, ok := h.workFissionStoredImage(w, r, credential, stringParam(inviteParams, "link_pic"), true)
		if !ok {
			return WorkFissionWrite{}, false
		}
		values.Invite = WorkFissionInviteCreateWrite{
			Text:      stringParam(inviteParams, "text"),
			LinkTitle: stringParam(inviteParams, "link_title"),
			LinkDesc:  stringParam(inviteParams, "link_desc"),
			LinkPic:   linkPic,
			WXLinkPic: wxLinkPic,
		}
	}
	return values, true
}

func (h *WorkFissionHandler) workFissionStoredImage(w http.ResponseWriter, r *http.Request, credential RoomWelcomeCorpCredential, raw string, uploadToWeCom bool) (string, string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", true
	}
	relative, localPath, err := contactMessageBatchSendMediaFilePath(h.fileStorageRoot, raw)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "图片处理失败", nil)
		return "", "", false
	}
	if !uploadToWeCom {
		return relative, "", true
	}
	if h.messageClient == nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "企业微信客户端未配置", nil)
		return "", "", false
	}
	if strings.HasPrefix(localPath, "http://") || strings.HasPrefix(localPath, "https://") {
		return relative, "", true
	}
	wxURL, err := h.messageClient.UploadImage(r.Context(), credential, localPath)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "图片上传企业微信失败", nil)
		return "", "", false
	}
	return relative, wxURL, true
}

func (h *WorkFissionHandler) workFissionPushComplex(w http.ResponseWriter, r *http.Request, credential RoomWelcomeCorpCredential, pushParams map[string]any, storeStyle bool) (string, string, bool) {
	complexParams, ok := workFissionMapParam(pushParams, "msg_complex")
	if !ok {
		return "{}", "", true
	}
	msgComplexType := stringParam(complexParams, "msg_complex_type")
	if !storeStyle {
		msgComplexType = stringParam(pushParams, "msg_complex_type")
	}
	payload := map[string]any{}
	if image := stringParam(complexParams, "image"); image != "" {
		relative, wxURL, ok := h.workFissionStoredImage(w, r, credential, image, true)
		if !ok {
			return "", "", false
		}
		payload["image"] = relative
		payload["pic_url"] = wxURL
	}
	if linkParams, ok := workFissionMapParam(complexParams, "link"); ok && stringParam(linkParams, "image") != "" {
		relative, wxURL, ok := h.workFissionStoredImage(w, r, credential, stringParam(linkParams, "image"), true)
		if !ok {
			return "", "", false
		}
		payload["image"] = relative
		payload["pic_url"] = wxURL
		payload["title"] = stringParam(linkParams, "title")
		payload["url"] = stringParam(linkParams, "url")
		payload["desc"] = stringParam(linkParams, "desc")
	}
	if appletsParams, ok := workFissionMapParam(complexParams, "applets"); ok && stringParam(appletsParams, "image") != "" {
		relative, wxURL, ok := h.workFissionStoredImage(w, r, credential, stringParam(appletsParams, "image"), true)
		if !ok {
			return "", "", false
		}
		payload["image"] = relative
		payload["pic_url"] = wxURL
		payload["title"] = stringParam(appletsParams, "title")
		payload["appid"] = stringParam(appletsParams, "appid")
		payload["path"] = stringParam(appletsParams, "path")
	}
	if len(payload) == 0 {
		msgComplexType = ""
	}
	return workFissionMustJSON(payload), msgComplexType, true
}

func (h *WorkFissionHandler) InviteData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, ok := h.resolveAuthorized(w, r)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	fissionIDs, ok := intJSONQuery(w, r, "fission_ids", "fission_ids 必须为整数数组")
	if !ok {
		return
	}
	status, ok := optionalQueryInt(w, r, "status", "status 必须为整数")
	if !ok {
		return
	}
	loss, ok := optionalQueryInt(w, r, "loss", "loss 必须为整数")
	if !ok {
		return
	}
	page, err := h.store.WorkFissionInviteDataPage(r.Context(), WorkFissionInviteDataFilter{
		CorpID:     corpID,
		FissionIDs: fissionIDs,
		Nickname:   strings.TrimSpace(r.URL.Query().Get("nickname")),
		Employee:   strings.TrimSpace(r.URL.Query().Get("employee")),
		StartTime:  strings.TrimSpace(r.URL.Query().Get("start_time")),
		EndTime:    strings.TrimSpace(r.URL.Query().Get("end_time")),
		Status:     status,
		Loss:       loss,
		Page:       positiveQueryInt(r, "page", 1),
		PerPage:    positiveQueryInt(r, "perPage", 10000),
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		employeeName := "未知"
		if item.Employee != "" && item.EmployeeName != "" {
			employeeName = item.EmployeeName
		}
		list = append(list, map[string]any{
			"id":           item.ID,
			"nickname":     item.Nickname,
			"avatar":       item.Avatar,
			"active_name":  item.ActiveName,
			"employees":    employeeName,
			"created_at":   item.CreatedAt,
			"loss":         workFissionLossStatus(item.Loss),
			"level":        item.Level,
			"status":       workFissionDoneStatus(item.Status),
			"invite_count": item.InviteCount,
			"contact_id":   item.ContactID,
			"employee_id":  item.EmployeeID,
		})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"page": map[string]any{"perPage": page.PerPage, "total": page.Total, "totalPage": page.TotalPage},
		"list": list,
	})
}

func (h *WorkFissionHandler) InviteDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, ok := h.resolveAuthorized(w, r)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	id, ok := queryPositiveIntParam(w, r, "id", "客户id 必填", "客户id 必须为整数")
	if !ok {
		return
	}
	detail, found, err := h.store.WorkFissionInviteDetail(r.Context(), corpID, id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "客户不存在", nil)
		return
	}
	users := make([]map[string]any, 0, len(detail.Children))
	for _, child := range detail.Children {
		users = append(users, map[string]any{
			"id":         child.ID,
			"nickname":   child.Nickname,
			"loss":       child.Loss,
			"created_at": child.CreatedAt,
			"avatar":     h.fullStaticURL(child.Avatar),
		})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"total_count": detail.TotalCount,
		"new_count":   detail.NewCount,
		"loss":        detail.LossCount,
		"insert":      detail.TotalCount - detail.LossCount,
		"user_list":   users,
	})
}

func (h *WorkFissionHandler) Destroy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, ok := h.resolveAuthorized(w, r)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("id")); raw != "" {
		params["id"] = raw
	}
	id, has, err := intParam(params, "id")
	if err != nil || !has || id <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "任务宝id 必填", nil)
		return
	}
	deleted, err := h.store.DeleteWorkFissionCascade(r.Context(), corpID, id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !deleted {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "任务宝不存在", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *WorkFissionHandler) workFissionInviteValues(w http.ResponseWriter, userID int, corpID int, params map[string]any) (WorkFissionInviteWrite, WorkFissionChooseContactFilter, bool) {
	fissionID, ok, err := intParam(params, "fission_id")
	if err != nil || !ok || fissionID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "fission_id 必须为整数", nil)
		return WorkFissionInviteWrite{}, WorkFissionChooseContactFilter{}, false
	}
	filterParams, ok := workFissionMapParam(params, "filter")
	if !ok {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "filter 必填", nil)
		return WorkFissionInviteWrite{}, WorkFissionChooseContactFilter{}, false
	}
	employeeIDs, err := workFissionIntSlice(filterParams, "employee_ids")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "employee_ids 必须为整数数组", nil)
		return WorkFissionInviteWrite{}, WorkFissionChooseContactFilter{}, false
	}
	isAll, _, err := intParam(filterParams, "is_all")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "is_all 必须为整数", nil)
		return WorkFissionInviteWrite{}, WorkFissionChooseContactFilter{}, false
	}
	var gender *int
	if raw := strings.TrimSpace(stringParam(filterParams, "gender")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "gender 必须为整数", nil)
			return WorkFissionInviteWrite{}, WorkFissionChooseContactFilter{}, false
		}
		gender = &value
	}
	if isAll == 0 || gender == nil {
		gender = nil
	}
	linkPic := stringParam(params, "link_pic")
	if linkPic != "" {
		relative, _, err := contactMessageBatchSendMediaFilePath(h.fileStorageRoot, linkPic)
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "图片处理失败", nil)
			return WorkFissionInviteWrite{}, WorkFissionChooseContactFilter{}, false
		}
		linkPic = relative
	}
	values := WorkFissionInviteWrite{
		FissionID:    fissionID,
		Text:         stringParam(params, "text"),
		LinkTitle:    stringParam(params, "link_title"),
		LinkDesc:     stringParam(params, "link_desc"),
		LinkPic:      linkPic,
		CreateUserID: userID,
	}
	filter := WorkFissionChooseContactFilter{
		CorpID:      corpID,
		EmployeeIDs: employeeIDs,
		IsAll:       isAll,
		StartTime:   stringParam(filterParams, "start_time"),
		EndTime:     stringParam(filterParams, "end_time"),
		Gender:      gender,
	}
	return values, filter, true
}

func (h *WorkFissionHandler) resolveAuthorized(w http.ResponseWriter, r *http.Request) (int, User, DashboardRequestScope, bool) {
	userID, user, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return 0, User{}, DashboardRequestScope{}, false
	}
	if h.authorizer != nil {
		corpID := 0
		if len(principalScope.CorpIDs) > 0 {
			corpID = principalScope.CorpIDs[0]
		}
		if _, err := h.authorizer.Resolve(r.Context(), userID, PermissionKeyFromRequest(r), corpID, principalScope.WorkEmployeeID); err != nil {
			writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, err.Error(), nil)
			return 0, User{}, DashboardRequestScope{}, false
		}
	}
	return userID, user, principalScope, true
}

func (h *WorkFissionHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (int, User, DashboardRequestScope, bool) {
	requestPrincipal, err := DashboardPrincipalFromContext(r.Context())
	userID := requestPrincipal.UserID
	if err != nil || userID <= 0 {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return 0, User{}, DashboardRequestScope{}, false
	}
	user, found, err := h.store.UserByID(r.Context(), userID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return 0, User{}, DashboardRequestScope{}, false
	}
	if !found {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "user not found", nil)
		return 0, User{}, DashboardRequestScope{}, false
	}
	principalScope, err := DashboardRequestScopeFromContext(r.Context())
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return 0, User{}, DashboardRequestScope{}, false
	}
	return userID, user, principalScope, true
}

func (h *WorkFissionHandler) fullStaticURL(path string) string {
	if path == "" || strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") || h.apiBaseURL == "" {
		return path
	}
	return h.apiBaseURL + "/static/" + strings.TrimLeft(path, "/")
}

func (h *WorkFissionHandler) authRedirectURL(id int) string {
	baseURL := h.operationBaseURL
	if baseURL == "" {
		baseURL = h.apiBaseURL
	}
	target := fmt.Sprintf("/workFission?id=%d", id)
	return fmt.Sprintf("%s/auth/workFission?id=%d&target=%s", baseURL, id, url.QueryEscape(target))
}

func (h *WorkFissionHandler) pushComplexPayload(raw string) (any, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	var payload any
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return nil, err
	}
	if item, ok := payload.(map[string]any); ok {
		if image, ok := item["image"].(string); ok {
			item["image"] = h.fullStaticURL(image)
		}
	}
	return payload, nil
}

func workFissionStatus(endTime string) string {
	if strings.TrimSpace(endTime) == "" {
		return "已结束"
	}
	parsed, err := time.ParseInLocation("2006-01-02 15:04:05", endTime, time.Local)
	if err != nil || time.Now().After(parsed) {
		return "已结束"
	}
	return "进行中"
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func intJSONQuery(w http.ResponseWriter, r *http.Request, key string, message string) ([]int, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return []int{}, true
	}
	var values []any
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&values); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, message, nil)
		return nil, false
	}
	ids := make([]int, 0, len(values))
	seen := map[int]struct{}{}
	for _, value := range values {
		id, ok, err := intParam(map[string]any{key: value}, key)
		if err != nil || !ok || id <= 0 {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, message, nil)
			return nil, false
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, true
}

func workFissionMapParam(params map[string]any, key string) (map[string]any, bool) {
	value, ok := params[key]
	if !ok || value == nil {
		return nil, false
	}
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case string:
		var decoded map[string]any
		decoder := json.NewDecoder(strings.NewReader(typed))
		decoder.UseNumber()
		if err := decoder.Decode(&decoded); err == nil {
			return decoded, true
		}
	}
	return nil, false
}

func workFissionIntSlice(params map[string]any, key string) ([]int, error) {
	raw := params[key]
	if text, ok := raw.(string); ok {
		trimmed := strings.TrimSpace(text)
		if strings.HasPrefix(trimmed, "[") {
			var values []any
			decoder := json.NewDecoder(strings.NewReader(trimmed))
			decoder.UseNumber()
			if err := decoder.Decode(&values); err != nil {
				return nil, err
			}
			return intSliceParam(map[string]any{key: values}, key)
		}
	}
	return intSliceParam(params, key)
}

func workFissionIntValue(params map[string]any, key string) int {
	value, ok, err := intParam(params, key)
	if err != nil || !ok {
		return 0
	}
	return value
}

func workFissionBoolInt(params map[string]any, key string) int {
	value, ok := params[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case bool:
		if typed {
			return 1
		}
		return 0
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true", "1", "yes", "on":
			return 1
		default:
			return 0
		}
	default:
		if integer, ok, err := intParam(params, key); err == nil && ok && integer != 0 {
			return 1
		}
	}
	return 0
}

func workFissionJSONParam(params map[string]any, key string) (string, error) {
	value, ok := params[key]
	if !ok || value == nil {
		return "[]", nil
	}
	if text, ok := value.(string); ok {
		trimmed := strings.TrimSpace(text)
		if trimmed == "" {
			return "[]", nil
		}
		if strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "{") {
			var decoded any
			decoder := json.NewDecoder(strings.NewReader(trimmed))
			decoder.UseNumber()
			if err := decoder.Decode(&decoded); err != nil {
				return "", err
			}
			return workFissionMustJSON(decoded), nil
		}
	}
	return workFissionMustJSON(value), nil
}

func workFissionMustJSON(value any) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}

func workFissionWXUserIDs(params map[string]any, key string) ([]string, error) {
	value, ok := params[key]
	if !ok || value == nil {
		return []string{}, nil
	}
	if text, ok := value.(string); ok {
		trimmed := strings.TrimSpace(text)
		if trimmed == "" {
			return []string{}, nil
		}
		var decoded any
		decoder := json.NewDecoder(strings.NewReader(trimmed))
		decoder.UseNumber()
		if err := decoder.Decode(&decoded); err != nil {
			return nil, err
		}
		value = decoded
	}
	items, ok := value.([]any)
	if !ok {
		return []string{}, nil
	}
	result := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		object, ok := item.(map[string]any)
		if !ok {
			continue
		}
		wxUserID := strings.TrimSpace(stringParam(object, "wxUserId"))
		if wxUserID == "" {
			wxUserID = strings.TrimSpace(stringParam(object, "wx_user_id"))
		}
		if wxUserID == "" {
			continue
		}
		if _, exists := seen[wxUserID]; exists {
			continue
		}
		seen[wxUserID] = struct{}{}
		result = append(result, wxUserID)
	}
	return result, nil
}

func workFissionRate(numerator int, denominator int) any {
	if denominator == 0 {
		return 0
	}
	return strconv.FormatFloat(float64(numerator)/float64(denominator)*100, 'f', 2, 64) + "%"
}

func workFissionUniqueEmployees(rawItems []string) []map[string]any {
	type employeeItem struct {
		ID   string
		Data map[string]any
	}
	order := []string{}
	items := map[string]employeeItem{}
	for _, raw := range rawItems {
		var values []map[string]any
		decoder := json.NewDecoder(strings.NewReader(raw))
		decoder.UseNumber()
		if err := decoder.Decode(&values); err != nil {
			continue
		}
		for _, value := range values {
			id := strings.TrimSpace(fmt.Sprint(value["id"]))
			if id == "" || id == "<nil>" {
				continue
			}
			if _, exists := items[id]; !exists {
				order = append(order, id)
			}
			items[id] = employeeItem{ID: id, Data: value}
		}
	}
	result := make([]map[string]any, 0, len(order))
	for _, id := range order {
		result = append(result, items[id].Data)
	}
	return result
}

func workFissionLossStatus(loss int) string {
	if loss == 1 {
		return "已流失"
	}
	return "未流失"
}

func workFissionDoneStatus(status int) string {
	if status == 1 {
		return "已完成"
	}
	return "未完成"
}
