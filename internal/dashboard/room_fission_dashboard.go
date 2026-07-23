package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type RoomFissionFilter struct {
	CorpID             int
	CreateUserID       int
	RestrictCreateUser bool
	ActiveName         string
	Page               int
	PerPage            int
}

type RoomFissionListItem struct {
	ID                  int
	OfficialAccountID   int
	ActiveName          string
	EndTime             string
	TargetCount         int
	NewFriend           int
	DeleteInvalid       int
	ReceiveEmployeesRaw string
	AutoPass            int
	Status              int
	TenantID            int
	CorpID              int
	CreateUserID        int
	CreateUserName      string
	ContactNum          int
	CompleteNum         int
	RoomNum             int
	CreatedAt           string
	UpdatedAt           string
}

type RoomFissionBundle struct {
	Fission RoomFissionInfo
	Poster  RoomFissionPoster
	Rooms   []RoomFissionRoom
	Welcome RoomFissionWelcome
	Invite  RoomFissionInvite
	Stats   RoomFissionOverview
}

type RoomFissionInfo struct {
	ID                  int
	OfficialAccountID   int
	ActiveName          string
	EndTime             string
	TargetCount         int
	NewFriend           int
	DeleteInvalid       int
	ReceiveEmployeesRaw string
	AutoPass            int
	Status              int
	TenantID            int
	CorpID              int
	CreateUserID        int
	CreatedAt           string
	UpdatedAt           string
}

type RoomFissionPoster struct {
	ID            int
	FissionID     int
	CoverPic      string
	AvatarShow    int
	NicknameShow  int
	NicknameColor string
	QRCodeW       string
	QRCodeH       string
	QRCodeX       string
	QRCodeY       string
	CreatedAt     string
	UpdatedAt     string
}

type RoomFissionRoom struct {
	ID           int
	FissionID    int
	RoomQRCode   string
	RoomWXQRCode string
	RoomRaw      string
	RoomMax      int
	ContactNum   int
	JoinNum      int
	CreatedAt    string
	UpdatedAt    string
}

type RoomFissionWelcome struct {
	ID         int
	FissionID  int
	Text       string
	LinkTitle  string
	LinkDesc   string
	LinkPic    string
	LinkWXURL  string
	TemplateID string
	CreatedAt  string
	UpdatedAt  string
}

type RoomFissionInvite struct {
	ID               int
	FissionID        int
	Type             int
	EmployeesRaw     string
	ChooseContactRaw string
	Text             string
	LinkTitle        string
	LinkDesc         string
	LinkPic          string
	WXLinkPic        string
	CreatedAt        string
	UpdatedAt        string
}

type RoomFissionWrite struct {
	Fission      RoomFissionBaseWrite
	Poster       RoomFissionPosterWrite
	Rooms        []RoomFissionRoomWrite
	RoomsTouched bool
	Welcome      RoomFissionWelcomeWrite
	Invite       RoomFissionInviteWrite
}

type RoomFissionBaseWrite struct {
	CorpID               int
	CreateUserID         int
	OfficialAccountID    int
	HasOfficialAccountID bool
	ActiveName           string
	HasActiveName        bool
	EndTime              string
	HasEndTime           bool
	TargetCount          int
	HasTargetCount       bool
	NewFriend            int
	HasNewFriend         bool
	DeleteInvalid        int
	HasDeleteInvalid     bool
	ReceiveEmployeesRaw  string
	HasReceiveEmployees  bool
	AutoPass             int
	HasAutoPass          bool
	Status               int
	HasStatus            bool
}

type RoomFissionPosterWrite struct {
	Touched       bool
	CoverPic      string
	AvatarShow    int
	NicknameShow  int
	NicknameColor string
	QRCodeW       string
	QRCodeH       string
	QRCodeX       string
	QRCodeY       string
}

type RoomFissionRoomWrite struct {
	RoomQRCode   string
	RoomWXQRCode string
	RoomRaw      string
	RoomMax      int
}

type RoomFissionWelcomeWrite struct {
	Touched    bool
	Text       string
	LinkTitle  string
	LinkDesc   string
	LinkPic    string
	LinkWXURL  string
	TemplateID string
}

type RoomFissionInviteWrite struct {
	Touched          bool
	FissionID        int
	Type             int
	EmployeesRaw     string
	ChooseContactRaw string
	Text             string
	LinkTitle        string
	LinkDesc         string
	LinkPic          string
	WXLinkPic        string
}

type RoomFissionContactFilter struct {
	CorpID        int
	FissionID     int
	Nickname      string
	Status        int
	WriteOff      int
	JoinStatus    int
	ReceiveStatus int
	Page          int
	PerPage       int
}

type RoomFissionContactItem struct {
	ID             int
	FissionID      int
	UnionID        string
	Nickname       string
	Avatar         string
	ParentUnionID  string
	Level          int
	ContactID      int
	Employee       string
	InviteCount    int
	Loss           int
	Status         int
	ReceiveStatus  int
	IsNew          int
	ExternalUserID string
	RoomID         int
	JoinStatus     int
	WriteOff       int
	CreatedAt      string
	UpdatedAt      string
}

type RoomFissionContactPage struct {
	Items     []RoomFissionContactItem
	Total     int
	TotalPage int
	Page      int
	PerPage   int
}

type RoomFissionRoomFilter struct {
	CorpID    int
	FissionID int
	Page      int
	PerPage   int
}

type RoomFissionRoomPage struct {
	Items     []RoomFissionRoom
	Total     int
	TotalPage int
	Page      int
	PerPage   int
}

type RoomFissionOverview struct {
	ContactNum  int
	CompleteNum int
	WriteOffNum int
	JoinRoomNum int
	InviteCount int
	LossNum     int
	RoomNum     int
}

type RoomFissionStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	RoomFissionPage(ctx context.Context, filter RoomFissionFilter) (RoomFissionPage, error)
	RoomFissionBundleByID(ctx context.Context, corpID int, id int) (RoomFissionBundle, bool, error)
	CreateRoomFission(ctx context.Context, values RoomFissionWrite) (int, error)
	UpdateRoomFission(ctx context.Context, corpID int, id int, values RoomFissionWrite) (bool, error)
	DeleteRoomFission(ctx context.Context, corpID int, id int) (bool, error)
	UpsertRoomFissionInvite(ctx context.Context, corpID int, id int, values RoomFissionInviteWrite) (bool, error)
	RoomFissionContactPage(ctx context.Context, filter RoomFissionContactFilter) (RoomFissionContactPage, error)
	RoomFissionRoomPage(ctx context.Context, filter RoomFissionRoomFilter) (RoomFissionRoomPage, error)
	RoomFissionOverview(ctx context.Context, corpID int, id int) (RoomFissionOverview, error)
	WriteOffRoomFissionContact(ctx context.Context, corpID int, fissionID int, contactID int) (bool, error)
}

type RoomFissionPage struct {
	Items     []RoomFissionListItem
	Total     int
	TotalPage int
	Page      int
	PerPage   int
}

type RoomFissionHandler struct {
	store            RoomFissionStore
	cache            LoginCache
	resolver         UserIDResolver
	authorizer       CorpAdminAuthorizer
	operationBaseURL string
}

func NewRoomFissionHandler(store RoomFissionStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer, operationBaseURL string) *RoomFissionHandler {
	return &RoomFissionHandler{store: store, cache: cache, resolver: resolver, authorizer: authorizer, operationBaseURL: strings.TrimRight(strings.TrimSpace(operationBaseURL), "/")}
}

func (h *RoomFissionHandler) Index(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, user, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/roomFission/index#get")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	page, err := h.store.RoomFissionPage(r.Context(), RoomFissionFilter{
		CorpID:             corpID,
		CreateUserID:       userID,
		RestrictCreateUser: user.IsSuperAdmin == 0,
		ActiveName:         lotteryQueryString(r, "activeName", "active_name", "name", "keyword"),
		Page:               positiveQueryInt(r, "page", 1),
		PerPage:            positiveQueryInt(r, "perPage", 15),
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, roomFissionListPayload(item, h.shareURL(item.ID)))
	}
	payload := contactBatchAddPagination(r, page.Page, page.PerPage, page.Total, list)
	payload["list"] = list
	payload["page"] = map[string]any{"perPage": page.PerPage, "total": page.Total, "totalPage": page.TotalPage}
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *RoomFissionHandler) Store(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, user, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/roomFission/store#post")
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
	values, err := roomFissionWriteFromParams(params, corpID, userID, true)
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
	if !enforceSaaSQuota(r.Context(), w, h.store, user.TenantID, SaaSMetricRoomFissions, 1) {
		return
	}
	id, err := h.store.CreateRoomFission(r.Context(), values)
	if err != nil {
		if writeSaaSQuotaError(w, err) {
			return
		}
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if err := refreshSaaSUsageCounter(r.Context(), h.store, user.TenantID, SaaSMetricRoomFissions); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{id})
}

func (h *RoomFissionHandler) Update(w http.ResponseWriter, r *http.Request) {
	h.writeMutation(w, r, http.MethodPut, "/dashboard/roomFission/update#put", func(ctx context.Context, userID int, corpID int, params map[string]any) (any, error) {
		id, err := roomFissionIDFromParams(params)
		if err != nil || id <= 0 {
			return nil, badRequestError("fissionId required")
		}
		values, err := roomFissionWriteFromParams(params, corpID, userID, false)
		if err != nil {
			return nil, err
		}
		updated, err := h.store.UpdateRoomFission(ctx, corpID, id, values)
		if err != nil {
			return nil, err
		}
		if !updated {
			return nil, badRequestError("room fission not found")
		}
		return []any{id}, nil
	})
}

func (h *RoomFissionHandler) Destroy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, user, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/roomFission/destroy#delete")
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
	id, err := roomFissionIDFromParams(params)
	if err != nil || id <= 0 {
		writeEnvelope(w, http.StatusBadRequest, 400, "fissionId required", nil)
		return
	}
	deleted, err := h.store.DeleteRoomFission(r.Context(), corpID, id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !deleted {
		writeEnvelope(w, http.StatusBadRequest, 400, "room fission not found", nil)
		return
	}
	if err := refreshSaaSUsageCounter(r.Context(), h.store, user.TenantID, SaaSMetricRoomFissions); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{id})
}

func (h *RoomFissionHandler) Info(w http.ResponseWriter, r *http.Request) {
	h.showBundle(w, r, "/dashboard/roomFission/info#get", true)
}

func (h *RoomFissionHandler) Show(w http.ResponseWriter, r *http.Request) {
	h.showBundle(w, r, "/dashboard/roomFission/show#get", false)
}

func (h *RoomFissionHandler) Invite(w http.ResponseWriter, r *http.Request) {
	h.writeMutation(w, r, http.MethodPost, "/dashboard/roomFission/invite#post", func(ctx context.Context, _ int, corpID int, params map[string]any) (any, error) {
		id, err := roomFissionIDFromParams(params)
		if err != nil || id <= 0 {
			return nil, badRequestError("fissionId required")
		}
		inviteParams := params
		if nested, ok := roomFissionMapParam(params, "invite"); ok {
			inviteParams = nested
		}
		invite, err := roomFissionInviteFromParams(inviteParams, true)
		if err != nil {
			return nil, err
		}
		invite.FissionID = id
		updated, err := h.store.UpsertRoomFissionInvite(ctx, corpID, id, invite)
		if err != nil {
			return nil, err
		}
		if !updated {
			return nil, badRequestError("room fission not found")
		}
		return []any{id}, nil
	})
}

func (h *RoomFissionHandler) ShowRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/roomFission/showRoom#get")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	page, err := h.store.RoomFissionRoomPage(r.Context(), RoomFissionRoomFilter{
		CorpID:    corpID,
		FissionID: roomFissionIDFromQuery(r),
		Page:      positiveQueryInt(r, "page", 1),
		PerPage:   positiveQueryInt(r, "perPage", 15),
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, roomFissionRoomPayload(item))
	}
	payload := contactBatchAddPagination(r, page.Page, page.PerPage, page.Total, list)
	payload["list"] = list
	payload["page"] = map[string]any{"perPage": page.PerPage, "total": page.Total, "totalPage": page.TotalPage}
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *RoomFissionHandler) ShowContact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/roomFission/showContact#get")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	page, err := h.store.RoomFissionContactPage(r.Context(), RoomFissionContactFilter{
		CorpID:        corpID,
		FissionID:     roomFissionIDFromQuery(r),
		Nickname:      lotteryQueryString(r, "nickname", "name", "contactName"),
		Status:        lotteryQueryIntAllowZero(r, "status", -1),
		WriteOff:      lotteryQueryIntAllowZero(r, "writeOff", -1),
		JoinStatus:    lotteryQueryIntAllowZero(r, "joinStatus", -1),
		ReceiveStatus: lotteryQueryIntAllowZero(r, "receiveStatus", -1),
		Page:          positiveQueryInt(r, "page", 1),
		PerPage:       positiveQueryInt(r, "perPage", 15),
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, roomFissionContactPayload(item))
	}
	payload := contactBatchAddPagination(r, page.Page, page.PerPage, page.Total, list)
	payload["list"] = list
	payload["page"] = map[string]any{"perPage": page.PerPage, "total": page.Total, "totalPage": page.TotalPage}
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *RoomFissionHandler) WriteOff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, loginInfo, _, ok := h.resolveAuthorized(w, r, "/dashboard/roomFission/writeOff#get")
	if !ok {
		return
	}
	corpID, ok := selectedCorpID(w, loginInfo)
	if !ok {
		return
	}
	fissionID := roomFissionIDFromQuery(r)
	contactID := positiveQueryInt(r, "contactId", 0)
	if contactID <= 0 {
		contactID = positiveQueryInt(r, "contact_id", 0)
	}
	if contactID <= 0 {
		contactID = positiveQueryInt(r, "contactRecordId", 0)
	}
	if contactID <= 0 {
		contactID = positiveQueryInt(r, "id", 0)
	}
	if contactID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, 400, "contactId required", nil)
		return
	}
	updated, err := h.store.WriteOffRoomFissionContact(r.Context(), corpID, fissionID, contactID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !updated {
		writeEnvelope(w, http.StatusNotFound, 404, "room fission contact not found", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{contactID})
}

func (h *RoomFissionHandler) showBundle(w http.ResponseWriter, r *http.Request, permission string, editShape bool) {
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
	id := roomFissionIDFromQuery(r)
	if id <= 0 {
		writeEnvelope(w, http.StatusBadRequest, 400, "fissionId required", nil)
		return
	}
	bundle, found, err := h.store.RoomFissionBundleByID(r.Context(), corpID, id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusNotFound, 404, "room fission not found", nil)
		return
	}
	payload := roomFissionBundlePayload(bundle, h.shareURL(id))
	if !editShape {
		payload["overview"] = roomFissionOverviewPayload(bundle.Stats)
	}
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *RoomFissionHandler) writeMutation(w http.ResponseWriter, r *http.Request, method string, permission string, mutate func(context.Context, int, int, map[string]any) (any, error)) {
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

func (h *RoomFissionHandler) resolveAuthorized(w http.ResponseWriter, r *http.Request, permissionKey string) (int, User, LoginCorpInfo, AccessContext, bool) {
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

func (h *RoomFissionHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (int, User, LoginCorpInfo, bool) {
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

func (h *RoomFissionHandler) shareURL(id int) string {
	base := h.operationBaseURL
	if base == "" {
		base = "/operation"
	}
	target := fmt.Sprintf("/roomFission?id=%d&parent_union_id=0&wx_user_id=0", id)
	return fmt.Sprintf("%s/auth/roomFission?id=%d&target=%s", base, id, url.QueryEscape(target))
}

func roomFissionWriteFromParams(params map[string]any, corpID int, userID int, requireBase bool) (RoomFissionWrite, error) {
	fissionParams := params
	if nested, ok := roomFissionMapParam(params, "fission"); ok {
		fissionParams = nested
	}
	values := RoomFissionWrite{Fission: RoomFissionBaseWrite{CorpID: corpID, CreateUserID: userID, Status: 1, HasStatus: requireBase}}
	if officialAccountID, ok, err := firstIntParam(fissionParams, "official_account_id", "officialAccountId"); err != nil {
		return values, err
	} else if ok {
		values.Fission.OfficialAccountID = officialAccountID
		values.Fission.HasOfficialAccountID = true
	}
	if activeName, ok := firstStringParam(fissionParams, "active_name", "activeName", "name"); ok {
		values.Fission.ActiveName = activeName
		values.Fission.HasActiveName = true
	}
	if requireBase && strings.TrimSpace(values.Fission.ActiveName) == "" {
		return values, badRequestError("activeName required")
	}
	if endTime, ok := firstStringParam(fissionParams, "end_time", "endTime"); ok {
		values.Fission.EndTime = endTime
		values.Fission.HasEndTime = true
	}
	if targetCount, ok, err := firstIntParam(fissionParams, "target_count", "targetCount"); err != nil {
		return values, err
	} else if ok {
		values.Fission.TargetCount = targetCount
		values.Fission.HasTargetCount = true
	}
	if newFriend, ok, err := roomFissionFirstBoolIntParam(fissionParams, "new_friend", "newFriend"); err != nil {
		return values, err
	} else if ok {
		values.Fission.NewFriend = newFriend
		values.Fission.HasNewFriend = true
	}
	if deleteInvalid, ok, err := roomFissionFirstBoolIntParam(fissionParams, "delete_invalid", "deleteInvalid"); err != nil {
		return values, err
	} else if ok {
		values.Fission.DeleteInvalid = deleteInvalid
		values.Fission.HasDeleteInvalid = true
	}
	if raw, ok := firstJSONRawParam(fissionParams, "receive_employees", "receiveEmployees"); ok {
		values.Fission.ReceiveEmployeesRaw = raw
		values.Fission.HasReceiveEmployees = true
	}
	if autoPass, ok, err := roomFissionFirstBoolIntParam(fissionParams, "auto_pass", "autoPass"); err != nil {
		return values, err
	} else if ok {
		values.Fission.AutoPass = autoPass
		values.Fission.HasAutoPass = true
	}
	if status, ok, err := firstIntParam(fissionParams, "status"); err != nil {
		return values, err
	} else if ok {
		values.Fission.Status = status
		values.Fission.HasStatus = true
	}
	if requireBase {
		if !values.Fission.HasOfficialAccountID {
			values.Fission.HasOfficialAccountID = true
		}
		if !values.Fission.HasTargetCount {
			values.Fission.TargetCount = 1
			values.Fission.HasTargetCount = true
		}
		if !values.Fission.HasNewFriend {
			values.Fission.HasNewFriend = true
		}
		if !values.Fission.HasDeleteInvalid {
			values.Fission.DeleteInvalid = 1
			values.Fission.HasDeleteInvalid = true
		}
		if !values.Fission.HasReceiveEmployees {
			values.Fission.ReceiveEmployeesRaw = "[]"
			values.Fission.HasReceiveEmployees = true
		}
		if !values.Fission.HasAutoPass {
			values.Fission.HasAutoPass = true
		}
	}
	if posterParams, ok := roomFissionMapParam(params, "poster"); ok {
		values.Poster = roomFissionPosterFromParams(posterParams)
	}
	rooms, touched := roomFissionRoomsFromParams(params)
	values.Rooms = rooms
	values.RoomsTouched = touched
	if welcomeParams, ok := roomFissionMapParam(params, "welcome"); ok {
		values.Welcome = roomFissionWelcomeFromParams(welcomeParams)
	}
	if inviteParams, ok := roomFissionMapParam(params, "invite"); ok {
		invite, err := roomFissionInviteFromParams(inviteParams, true)
		if err != nil {
			return values, err
		}
		values.Invite = invite
	}
	return values, nil
}

func roomFissionPosterFromParams(params map[string]any) RoomFissionPosterWrite {
	poster := RoomFissionPosterWrite{Touched: true}
	poster.CoverPic, _ = firstStringParam(params, "cover_pic", "coverPic")
	poster.NicknameColor, _ = firstStringParam(params, "nickname_color", "nicknameColor")
	poster.QRCodeW, _ = firstStringParam(params, "qrcode_w", "qrcodeW")
	poster.QRCodeH, _ = firstStringParam(params, "qrcode_h", "qrcodeH")
	poster.QRCodeX, _ = firstStringParam(params, "qrcode_x", "qrcodeX")
	poster.QRCodeY, _ = firstStringParam(params, "qrcode_y", "qrcodeY")
	poster.AvatarShow, _, _ = roomFissionFirstBoolIntParam(params, "avatar_show", "avatarShow")
	poster.NicknameShow, _, _ = roomFissionFirstBoolIntParam(params, "nickname_show", "nicknameShow")
	return poster
}

func roomFissionRoomsFromParams(params map[string]any) ([]RoomFissionRoomWrite, bool) {
	value, exists := params["rooms"]
	if !exists {
		value, exists = params["room"]
	}
	if !exists {
		return nil, false
	}
	rawItems, ok := value.([]any)
	if !ok {
		return []RoomFissionRoomWrite{}, true
	}
	rooms := make([]RoomFissionRoomWrite, 0, len(rawItems))
	for _, raw := range rawItems {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		room := RoomFissionRoomWrite{RoomRaw: "{}"}
		room.RoomQRCode, _ = firstStringParam(item, "room_qrcode", "roomQrcode")
		room.RoomWXQRCode, _ = firstStringParam(item, "room_wx_qrcode", "roomWxQrcode")
		if roomMax, ok, _ := firstIntParam(item, "room_max", "roomMax"); ok {
			room.RoomMax = roomMax
		}
		if rawRoom, ok := firstJSONRawParam(item, "room"); ok {
			room.RoomRaw = rawRoom
		}
		rooms = append(rooms, room)
	}
	return rooms, true
}

func roomFissionWelcomeFromParams(params map[string]any) RoomFissionWelcomeWrite {
	welcome := RoomFissionWelcomeWrite{Touched: true}
	welcome.Text, _ = firstStringParam(params, "text")
	welcome.LinkTitle, _ = firstStringParam(params, "link_title", "linkTitle")
	welcome.LinkDesc, _ = firstStringParam(params, "link_desc", "linkDesc")
	welcome.LinkPic, _ = firstStringParam(params, "link_pic", "linkPic")
	welcome.LinkWXURL, _ = firstStringParam(params, "link_wx_url", "linkWxUrl")
	welcome.TemplateID, _ = firstStringParam(params, "template_id", "templateId")
	return welcome
}

func roomFissionInviteFromParams(params map[string]any, markTouched bool) (RoomFissionInviteWrite, error) {
	invite := RoomFissionInviteWrite{Touched: markTouched, Type: 2, EmployeesRaw: "[]", ChooseContactRaw: "{}"}
	if typ, ok, err := firstIntParam(params, "type"); err != nil {
		return invite, err
	} else if ok {
		invite.Type = typ
	}
	if raw, ok := firstJSONRawParam(params, "employees"); ok {
		invite.EmployeesRaw = raw
	}
	if raw, ok := firstJSONRawParam(params, "choose_contact", "chooseContact"); ok {
		invite.ChooseContactRaw = raw
	}
	invite.Text, _ = firstStringParam(params, "text")
	invite.LinkTitle, _ = firstStringParam(params, "link_title", "linkTitle")
	invite.LinkDesc, _ = firstStringParam(params, "link_desc", "linkDesc")
	invite.LinkPic, _ = firstStringParam(params, "link_pic", "linkPic")
	invite.WXLinkPic, _ = firstStringParam(params, "wx_link_pic", "wxLinkPic")
	return invite, nil
}

func roomFissionIDFromParams(params map[string]any) (int, error) {
	if id, has, err := firstPositiveIntParam(params, "fissionId", "fission_id", "activityId", "activity_id", "id"); err != nil || has {
		return id, err
	}
	if fissionParams, ok := roomFissionMapParam(params, "fission"); ok {
		id, _, err := firstPositiveIntParam(fissionParams, "fissionId", "fission_id", "activityId", "activity_id", "id")
		return id, err
	}
	return 0, nil
}

func roomFissionIDFromQuery(r *http.Request) int {
	for _, key := range []string{"fissionId", "fission_id", "activityId", "activity_id", "id"} {
		if value := positiveQueryInt(r, key, 0); value > 0 {
			return value
		}
	}
	return 0
}

func roomFissionMapParam(params map[string]any, key string) (map[string]any, bool) {
	value, ok := params[key]
	if !ok || value == nil {
		return nil, false
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
	return nil, false
}

func roomFissionFirstBoolIntParam(params map[string]any, keys ...string) (int, bool, error) {
	for _, key := range keys {
		value, exists := params[key]
		if !exists {
			continue
		}
		switch typed := value.(type) {
		case bool:
			if typed {
				return 1, true, nil
			}
			return 0, true, nil
		case string:
			trimmed := strings.TrimSpace(strings.ToLower(typed))
			if trimmed == "" {
				return 0, false, nil
			}
			switch trimmed {
			case "true", "yes", "on":
				return 1, true, nil
			case "false", "no", "off":
				return 0, true, nil
			}
		}
		intValue, ok, err := intParam(params, key)
		if err != nil || ok {
			return intValue, ok, err
		}
	}
	return 0, false, nil
}

func roomFissionListPayload(item RoomFissionListItem, link string) map[string]any {
	payload := roomFissionBasePayload(RoomFissionInfo{
		ID:                  item.ID,
		OfficialAccountID:   item.OfficialAccountID,
		ActiveName:          item.ActiveName,
		EndTime:             item.EndTime,
		TargetCount:         item.TargetCount,
		NewFriend:           item.NewFriend,
		DeleteInvalid:       item.DeleteInvalid,
		ReceiveEmployeesRaw: item.ReceiveEmployeesRaw,
		AutoPass:            item.AutoPass,
		Status:              item.Status,
		TenantID:            item.TenantID,
		CorpID:              item.CorpID,
		CreateUserID:        item.CreateUserID,
		CreatedAt:           item.CreatedAt,
		UpdatedAt:           item.UpdatedAt,
	})
	payload["createUserName"] = item.CreateUserName
	payload["contactNum"] = item.ContactNum
	payload["completeNum"] = item.CompleteNum
	payload["roomNum"] = item.RoomNum
	payload["link"] = link
	payload["shareUrl"] = link
	return payload
}

func roomFissionBundlePayload(bundle RoomFissionBundle, link string) map[string]any {
	rooms := make([]map[string]any, 0, len(bundle.Rooms))
	for _, room := range bundle.Rooms {
		rooms = append(rooms, roomFissionRoomPayload(room))
	}
	fission := roomFissionBasePayload(bundle.Fission)
	return map[string]any{
		"id":        bundle.Fission.ID,
		"fission":   fission,
		"poster":    roomFissionPosterPayload(bundle.Poster),
		"rooms":     rooms,
		"welcome":   roomFissionWelcomePayload(bundle.Welcome),
		"invite":    roomFissionInvitePayload(bundle.Invite),
		"link":      link,
		"url":       link,
		"qrcodeUrl": link,
		"overview":  roomFissionOverviewPayload(bundle.Stats),
	}
}

func roomFissionBasePayload(item RoomFissionInfo) map[string]any {
	return map[string]any{
		"id":                  item.ID,
		"fissionId":           item.ID,
		"fission_id":          item.ID,
		"officialAccountId":   item.OfficialAccountID,
		"official_account_id": item.OfficialAccountID,
		"activeName":          item.ActiveName,
		"active_name":         item.ActiveName,
		"endTime":             item.EndTime,
		"end_time":            item.EndTime,
		"targetCount":         item.TargetCount,
		"target_count":        item.TargetCount,
		"newFriend":           item.NewFriend,
		"new_friend":          item.NewFriend,
		"deleteInvalid":       item.DeleteInvalid,
		"delete_invalid":      item.DeleteInvalid,
		"receiveEmployees":    jsonPayload(item.ReceiveEmployeesRaw),
		"receive_employees":   jsonPayload(item.ReceiveEmployeesRaw),
		"autoPass":            item.AutoPass,
		"auto_pass":           item.AutoPass,
		"status":              item.Status,
		"statusText":          roomFissionStatusText(item.Status),
		"tenantId":            item.TenantID,
		"corpId":              item.CorpID,
		"createUserId":        item.CreateUserID,
		"createdAt":           item.CreatedAt,
		"updatedAt":           item.UpdatedAt,
	}
}

func roomFissionPosterPayload(item RoomFissionPoster) map[string]any {
	return map[string]any{
		"id":             item.ID,
		"fissionId":      item.FissionID,
		"coverPic":       item.CoverPic,
		"cover_pic":      item.CoverPic,
		"avatarShow":     item.AvatarShow,
		"avatar_show":    item.AvatarShow,
		"nicknameShow":   item.NicknameShow,
		"nickname_show":  item.NicknameShow,
		"nicknameColor":  item.NicknameColor,
		"nickname_color": item.NicknameColor,
		"qrcodeW":        item.QRCodeW,
		"qrcode_w":       item.QRCodeW,
		"qrcodeH":        item.QRCodeH,
		"qrcode_h":       item.QRCodeH,
		"qrcodeX":        item.QRCodeX,
		"qrcode_x":       item.QRCodeX,
		"qrcodeY":        item.QRCodeY,
		"qrcode_y":       item.QRCodeY,
		"createdAt":      item.CreatedAt,
		"updatedAt":      item.UpdatedAt,
	}
}

func roomFissionRoomPayload(item RoomFissionRoom) map[string]any {
	return map[string]any{
		"id":             item.ID,
		"roomRecordId":   item.ID,
		"fissionId":      item.FissionID,
		"roomQrcode":     item.RoomQRCode,
		"room_qrcode":    item.RoomQRCode,
		"roomWxQrcode":   item.RoomWXQRCode,
		"room_wx_qrcode": item.RoomWXQRCode,
		"room":           jsonPayload(item.RoomRaw),
		"roomMax":        item.RoomMax,
		"room_max":       item.RoomMax,
		"contactNum":     item.ContactNum,
		"joinNum":        item.JoinNum,
		"createdAt":      item.CreatedAt,
		"updatedAt":      item.UpdatedAt,
	}
}

func roomFissionWelcomePayload(item RoomFissionWelcome) map[string]any {
	return map[string]any{
		"id":          item.ID,
		"fissionId":   item.FissionID,
		"text":        item.Text,
		"linkTitle":   item.LinkTitle,
		"link_title":  item.LinkTitle,
		"linkDesc":    item.LinkDesc,
		"link_desc":   item.LinkDesc,
		"linkPic":     item.LinkPic,
		"link_pic":    item.LinkPic,
		"linkWxUrl":   item.LinkWXURL,
		"link_wx_url": item.LinkWXURL,
		"templateId":  item.TemplateID,
		"template_id": item.TemplateID,
		"createdAt":   item.CreatedAt,
		"updatedAt":   item.UpdatedAt,
	}
}

func roomFissionInvitePayload(item RoomFissionInvite) map[string]any {
	return map[string]any{
		"id":             item.ID,
		"fissionId":      item.FissionID,
		"type":           item.Type,
		"employees":      jsonPayload(item.EmployeesRaw),
		"chooseContact":  jsonPayload(item.ChooseContactRaw),
		"choose_contact": jsonPayload(item.ChooseContactRaw),
		"text":           item.Text,
		"linkTitle":      item.LinkTitle,
		"link_title":     item.LinkTitle,
		"linkDesc":       item.LinkDesc,
		"link_desc":      item.LinkDesc,
		"linkPic":        item.LinkPic,
		"link_pic":       item.LinkPic,
		"wxLinkPic":      item.WXLinkPic,
		"wx_link_pic":    item.WXLinkPic,
		"createdAt":      item.CreatedAt,
		"updatedAt":      item.UpdatedAt,
	}
}

func roomFissionContactPayload(item RoomFissionContactItem) map[string]any {
	return map[string]any{
		"id":               item.ID,
		"contactRecordId":  item.ID,
		"fissionId":        item.FissionID,
		"unionId":          item.UnionID,
		"union_id":         item.UnionID,
		"nickname":         item.Nickname,
		"name":             item.Nickname,
		"avatar":           item.Avatar,
		"parentUnionId":    item.ParentUnionID,
		"parent_union_id":  item.ParentUnionID,
		"level":            item.Level,
		"contactId":        item.ContactID,
		"contact_id":       item.ContactID,
		"employee":         item.Employee,
		"inviteCount":      item.InviteCount,
		"invite_count":     item.InviteCount,
		"loss":             item.Loss,
		"status":           item.Status,
		"statusText":       roomFissionContactStatusText(item.Status),
		"receiveStatus":    item.ReceiveStatus,
		"receive_status":   item.ReceiveStatus,
		"isNew":            item.IsNew,
		"is_new":           item.IsNew,
		"externalUserId":   item.ExternalUserID,
		"external_user_id": item.ExternalUserID,
		"roomId":           item.RoomID,
		"room_id":          item.RoomID,
		"joinStatus":       item.JoinStatus,
		"join_status":      item.JoinStatus,
		"writeOff":         item.WriteOff,
		"write_off":        item.WriteOff,
		"createdAt":        item.CreatedAt,
		"updatedAt":        item.UpdatedAt,
	}
}

func roomFissionOverviewPayload(stats RoomFissionOverview) map[string]any {
	return map[string]any{
		"contactNum":  stats.ContactNum,
		"completeNum": stats.CompleteNum,
		"writeOffNum": stats.WriteOffNum,
		"joinRoomNum": stats.JoinRoomNum,
		"inviteCount": stats.InviteCount,
		"lossNum":     stats.LossNum,
		"roomNum":     stats.RoomNum,
	}
}

func roomFissionStatusText(status int) string {
	switch status {
	case 2:
		return "已完成"
	case 0:
		return "未开始"
	default:
		return "进行中"
	}
}

func roomFissionContactStatusText(status int) string {
	if status == 1 {
		return "已完成"
	}
	return "未完成"
}

func roomFissionJSONIntSlice(values []int) string {
	if len(values) == 0 {
		return "[]"
	}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		if value > 0 {
			parts = append(parts, strconv.Itoa(value))
		}
	}
	return "[" + strings.Join(parts, ",") + "]"
}
