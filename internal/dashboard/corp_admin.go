package dashboard

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"strconv"
	"strings"
)

type CorpDetail struct {
	ID             int
	Name           string
	WxCorpID       string
	SocialCode     string
	EmployeeSecret string
	EventCallback  string
	ContactSecret  string
	Token          string
	EncodingAESKey string
	TenantID       int
	CreatedAt      string
	UpdatedAt      string
}

type CorpListPage struct {
	Items     []CorpDetail
	Total     int
	TotalPage int
}

type CorpAdminStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	CorpList(ctx context.Context, filter CorpListFilter) (CorpListPage, error)
	CorpDetailByID(ctx context.Context, corpID int) (CorpDetail, bool, error)
	CountCorps(ctx context.Context) (int, error)
	CreateCorp(ctx context.Context, values CorpCreateValues) (int, error)
	UpdateCorp(ctx context.Context, corpID int, values CorpUpdateValues) error
}

type CorpAdminAuthorizer interface {
	Resolve(ctx context.Context, userID int, permissionKey string, corpID int, workEmployeeID int) (AccessContext, error)
}

type CorpWeComValidator interface {
	ValidateCorpSecrets(ctx context.Context, wxCorpID string, employeeSecret string, contactSecret string) error
}

type EmployeeApplyEnqueuer interface {
	EnqueueEmployeeApply(ctx context.Context, event EmployeeApplyEvent) error
}

type CorpListFilter struct {
	TenantID   int
	CorpIDs    []int
	CorpName   string
	Page       int
	PerPage    int
	SuperAdmin bool
}

type CorpUpdateValues struct {
	Name           string
	WxCorpID       string
	EmployeeSecret string
	ContactSecret  string
}

type CorpCreateValues struct {
	Name           string
	WxCorpID       string
	EmployeeSecret string
	ContactSecret  string
	EventCallback  string
	Token          string
	EncodingAESKey string
	TenantID       int
}

type CorpAdminHandler struct {
	store       CorpAdminStore
	cache       LoginCache
	cacheWriter UserCorpCacheWriter
	resolver    UserIDResolver
	authorizer  CorpAdminAuthorizer
	wecom       CorpWeComValidator
	employeeJob EmployeeApplyEnqueuer
	apiBaseURL  string
}

func NewCorpAdminHandler(store CorpAdminStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer) *CorpAdminHandler {
	return &CorpAdminHandler{store: store, cache: cache, resolver: resolver, authorizer: authorizer}
}

func (h *CorpAdminHandler) WithCacheWriter(cacheWriter UserCorpCacheWriter) *CorpAdminHandler {
	h.cacheWriter = cacheWriter
	return h
}

func (h *CorpAdminHandler) WithWeComValidator(wecom CorpWeComValidator) *CorpAdminHandler {
	h.wecom = wecom
	return h
}

func (h *CorpAdminHandler) WithEmployeeApplyQueue(queue EmployeeApplyEnqueuer) *CorpAdminHandler {
	h.employeeJob = queue
	return h
}

func (h *CorpAdminHandler) WithAPIBaseURL(apiBaseURL string) *CorpAdminHandler {
	h.apiBaseURL = strings.TrimRight(apiBaseURL, "/")
	return h
}

func (h *CorpAdminHandler) Index(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}

	userID, user, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if _, err := h.authorize(r.Context(), r, userID, loginInfo); err != nil {
		writeAccessError(w, err)
		return
	}

	filter := CorpListFilter{
		TenantID:   user.TenantID,
		CorpIDs:    loginInfo.CorpIDs,
		CorpName:   strings.TrimSpace(r.URL.Query().Get("corpName")),
		Page:       positiveQueryInt(r, "page", 1),
		PerPage:    positiveQueryInt(r, "perPage", 10),
		SuperAdmin: user.IsSuperAdmin == 1,
	}
	page, err := h.store.CorpList(r.Context(), filter)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	list := make([]map[string]any, 0, len(page.Items))
	for _, corp := range page.Items {
		list = append(list, map[string]any{
			"corpId":           corp.ID,
			"corpName":         corp.Name,
			"wxCorpId":         corp.WxCorpID,
			"createdAt":        corp.CreatedAt,
			"chatStatus":       0,
			"chatApplyStatus":  0,
			"messageCreatedAt": "",
		})
	}

	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"page": map[string]any{
			"perPage":   filter.PerPage,
			"total":     page.Total,
			"totalPage": page.TotalPage,
		},
		"list": list,
	})
}

func (h *CorpAdminHandler) Show(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}

	userID, _, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if _, err := h.authorize(r.Context(), r, userID, loginInfo); err != nil {
		writeAccessError(w, err)
		return
	}

	corpID, err := positiveQueryIntRequired(r, "corpId")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "企业授信ID 必填", nil)
		return
	}

	corp, found, err := h.store.CorpDetailByID(r.Context(), corpID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "当前企业信息不存在", nil)
		return
	}

	writeEnvelope(w, http.StatusOK, 200, "success", corpDetailPayload(corp))
}

func (h *CorpAdminHandler) Update(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}

	userID, _, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if _, err := h.authorize(r.Context(), r, userID, loginInfo); err != nil {
		writeAccessError(w, err)
		return
	}

	values, err := parseCorpUpdateRequest(r)
	if err != nil || values.CorpID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "企业授信ID 必填", nil)
		return
	}
	if err := validateCorpUpdate(values); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}

	if _, found, err := h.store.CorpDetailByID(r.Context(), values.CorpID); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	} else if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "当前企业信息不存在，不可操作", nil)
		return
	}

	if err := h.store.UpdateCorp(r.Context(), values.CorpID, CorpUpdateValues{
		Name:           strings.TrimSpace(values.CorpName),
		WxCorpID:       strings.TrimSpace(values.WxCorpID),
		EmployeeSecret: strings.TrimSpace(values.EmployeeSecret),
		ContactSecret:  strings.TrimSpace(values.ContactSecret),
	}); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *CorpAdminHandler) Store(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, user, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if _, err := h.authorize(r.Context(), r, userID, loginInfo); err != nil {
		writeAccessError(w, err)
		return
	}
	if _, ok := h.store.(SaaSQuotaStore); ok {
		if !enforceSaaSQuota(r.Context(), w, h.store, user.TenantID, SaaSMetricCorps, 1) {
			return
		}
	} else {
		total, err := h.store.CountCorps(r.Context())
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		if total >= 1 {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "只能添加一个企业", nil)
			return
		}
	}
	req, err := parseCorpStoreRequest(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	if err := validateCorpStore(req); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if h.wecom != nil {
		if err := h.wecom.ValidateCorpSecrets(r.Context(), strings.TrimSpace(req.WxCorpID), strings.TrimSpace(req.EmployeeSecret), strings.TrimSpace(req.ContactSecret)); err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
			return
		}
	}
	token, aesKey, err := corpCallbackSecrets()
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	corpID, err := h.store.CreateCorp(r.Context(), CorpCreateValues{
		Name:           strings.TrimSpace(req.CorpName),
		WxCorpID:       strings.TrimSpace(req.WxCorpID),
		EmployeeSecret: strings.TrimSpace(req.EmployeeSecret),
		ContactSecret:  strings.TrimSpace(req.ContactSecret),
		EventCallback:  h.callbackURL(),
		Token:          token,
		EncodingAESKey: aesKey,
		TenantID:       user.TenantID,
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "企业微信授信创建失败", nil)
		return
	}
	if h.cacheWriter != nil {
		if err := h.cacheWriter.SetUserCorpCache(r.Context(), userID, strconv.Itoa(corpID)+"-0"); err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "企业微信授信创建失败", nil)
			return
		}
	}
	if h.employeeJob != nil {
		if err := h.employeeJob.EnqueueEmployeeApply(r.Context(), EmployeeApplyEvent{CorpIDs: []int{corpID}, UserID: userID, Source: "dashboard.corp.store"}); err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "企业通讯录同步任务创建失败", nil)
			return
		}
	}
	if err := refreshSaaSUsageCounter(r.Context(), h.store, user.TenantID, SaaSMetricCorps); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "企业用量刷新失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *CorpAdminHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (int, User, LoginCorpInfo, bool) {
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

func (h *CorpAdminHandler) authorize(ctx context.Context, r *http.Request, userID int, loginInfo LoginCorpInfo) (AccessContext, error) {
	if h.authorizer == nil {
		return AccessContext{}, nil
	}
	corpID := 0
	if len(loginInfo.CorpIDs) > 0 {
		corpID = loginInfo.CorpIDs[0]
	}
	return h.authorizer.Resolve(ctx, userID, PermissionKeyFromRequest(r), corpID, loginInfo.WorkEmployeeID)
}

type corpUpdateRequest struct {
	CorpID         int    `json:"corpId"`
	CorpName       string `json:"corpName"`
	WxCorpID       string `json:"wxCorpId"`
	EmployeeSecret string `json:"employeeSecret"`
	ContactSecret  string `json:"contactSecret"`
}

type corpStoreRequest struct {
	CorpName       string `json:"corpName"`
	WxCorpID       string `json:"wxCorpId"`
	EmployeeSecret string `json:"employeeSecret"`
	ContactSecret  string `json:"contactSecret"`
}

func parseCorpStoreRequest(r *http.Request) (corpStoreRequest, error) {
	var req corpStoreRequest
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return req, err
		}
		req.CorpName = r.FormValue("corpName")
		req.WxCorpID = r.FormValue("wxCorpId")
		req.EmployeeSecret = r.FormValue("employeeSecret")
		req.ContactSecret = r.FormValue("contactSecret")
		return req, nil
	}
	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	if err := decoder.Decode(&req); err != nil {
		return req, err
	}
	return req, nil
}

func parseCorpUpdateRequest(r *http.Request) (corpUpdateRequest, error) {
	var req corpUpdateRequest
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return req, err
		}
		req.CorpID, _ = strconv.Atoi(r.FormValue("corpId"))
		req.CorpName = r.FormValue("corpName")
		req.WxCorpID = r.FormValue("wxCorpId")
		req.EmployeeSecret = r.FormValue("employeeSecret")
		req.ContactSecret = r.FormValue("contactSecret")
		return req, nil
	}

	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	if err := decoder.Decode(&req); err != nil {
		return req, err
	}
	return req, nil
}

func validateCorpStore(req corpStoreRequest) error {
	return validateCorpFields(req.CorpName, req.WxCorpID, req.EmployeeSecret, req.ContactSecret)
}

func validateCorpUpdate(req corpUpdateRequest) error {
	return validateCorpFields(req.CorpName, req.WxCorpID, req.EmployeeSecret, req.ContactSecret)
}

func validateCorpFields(corpName string, wxCorpID string, employeeSecret string, contactSecret string) error {
	switch {
	case strings.TrimSpace(corpName) == "":
		return fieldError("企业名称 必填")
	case strings.TrimSpace(wxCorpID) == "":
		return fieldError("企业ID 必填")
	case len(strings.TrimSpace(wxCorpID)) > 18:
		return fieldError("企业ID 字符串最大长度18")
	case strings.TrimSpace(employeeSecret) == "":
		return fieldError("通讯录管理secret 必填")
	case len(strings.TrimSpace(employeeSecret)) > 43:
		return fieldError("通讯录管理secret  字符串最大长度43")
	case strings.TrimSpace(contactSecret) == "":
		return fieldError("外部联系人管理secret 必填")
	case len(strings.TrimSpace(contactSecret)) > 43:
		return fieldError("外部联系人管理secret 字符串最大长度43")
	default:
		return nil
	}
}

type fieldError string

func (e fieldError) Error() string { return string(e) }

func positiveQueryInt(r *http.Request, key string, fallback int) int {
	value, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func positiveQueryIntRequired(r *http.Request, key string) (int, error) {
	value, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil || value <= 0 {
		return 0, err
	}
	return value, nil
}

func corpDetailPayload(corp CorpDetail) map[string]any {
	eventCallback := corp.EventCallback
	if eventCallback != "" {
		eventCallback += "?cid=" + strconv.Itoa(corp.ID)
	}
	return map[string]any{
		"corpId":         corp.ID,
		"corpName":       corp.Name,
		"wxCorpId":       corp.WxCorpID,
		"socialCode":     corp.SocialCode,
		"employeeSecret": corp.EmployeeSecret,
		"eventCallback":  eventCallback,
		"contactSecret":  corp.ContactSecret,
		"token":          corp.Token,
		"encodingAesKey": corp.EncodingAESKey,
		"tenantId":       corp.TenantID,
	}
}

func (h *CorpAdminHandler) callbackURL() string {
	if h.apiBaseURL == "" {
		return "/weWork/callback"
	}
	return h.apiBaseURL + "/weWork/callback"
}

func corpCallbackSecrets() (string, string, error) {
	tokenBytes := make([]byte, 24)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", "", err
	}
	token := strings.NewReplacer("/", "", "+", "", "=", "").Replace(base64.StdEncoding.EncodeToString(tokenBytes))
	aesKey, err := randomAlphaNum(43)
	if err != nil {
		return "", "", err
	}
	return token, aesKey, nil
}

func randomAlphaNum(length int) (string, error) {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	var builder strings.Builder
	builder.Grow(length)
	max := big.NewInt(int64(len(chars)))
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		builder.WriteByte(chars[n.Int64()])
	}
	return builder.String(), nil
}

func writeAccessError(w http.ResponseWriter, err error) {
	switch err {
	case ErrUnauthorized:
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
	case ErrPermissionDenied:
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, "permission denied", nil)
	default:
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
	}
}
