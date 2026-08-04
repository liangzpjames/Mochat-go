package dashboard

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
)

var ErrPhase34AcquisitionProviderNotConfigured = errors.New("phase34 acquisition provider not configured")

type Phase34AcquisitionLink struct {
	ID                  int    `json:"id"`
	CorpID              int    `json:"corpId"`
	Name                string `json:"name"`
	TargetURL           string `json:"targetUrl"`
	AuthorizationStatus string `json:"authorizationStatus"`
	Status              string `json:"status"`
	VisitTotal          int    `json:"visitTotal"`
	ConversionTotal     int    `json:"conversionTotal"`
	CreatorName         string `json:"creatorName"`
	CreatedAt           string `json:"createdAt"`
	UpdatedAt           string `json:"updatedAt"`
}

type Phase34AcquisitionLinkFilter struct {
	CorpID  int
	Name    string
	Status  string
	Page    int
	PerPage int
}

type Phase34AcquisitionLinkPage struct {
	Items     []Phase34AcquisitionLink
	Total     int
	Page      int
	PerPage   int
	TotalPage int
}

type Phase34AcquisitionLinkWrite struct {
	CorpID              int
	UserID              int
	CreatorName         string
	Name                string
	TargetURL           string
	AuthorizationStatus string
	Status              string
}

type Phase34CustomerService struct {
	ID          int    `json:"id"`
	CorpID      int    `json:"corpId"`
	Name        string `json:"name"`
	Account     string `json:"account"`
	EmployeeIDs string `json:"employeeIds"`
	ReceiveMode string `json:"receiveMode"`
	Status      string `json:"status"`
	SyncReason  string `json:"syncReason"`
	CreatorName string `json:"creatorName"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type Phase34CustomerServiceFilter struct {
	CorpID  int
	Name    string
	Status  string
	Page    int
	PerPage int
}

type Phase34CustomerServicePage struct {
	Items     []Phase34CustomerService
	Total     int
	Page      int
	PerPage   int
	TotalPage int
}

type Phase34CustomerServiceWrite struct {
	CorpID      int
	UserID      int
	CreatorName string
	Name        string
	Account     string
	EmployeeIDs string
	ReceiveMode string
	Status      string
}

type Phase34ShortLink struct {
	ID          int    `json:"id"`
	CorpID      int    `json:"corpId"`
	Name        string `json:"name"`
	Token       string `json:"token"`
	TargetType  string `json:"targetType"`
	TargetURL   string `json:"targetUrl"`
	Status      string `json:"status"`
	VisitTotal  int    `json:"visitTotal"`
	CreatorName string `json:"creatorName"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
	DisabledAt  string `json:"disabledAt"`
}

type Phase34ShortLinkFilter struct {
	CorpID  int
	Name    string
	Status  string
	Page    int
	PerPage int
}

type Phase34ShortLinkPage struct {
	Items     []Phase34ShortLink
	Total     int
	Page      int
	PerPage   int
	TotalPage int
}

type Phase34ShortLinkWrite struct {
	CorpID      int
	UserID      int
	CreatorName string
	Name        string
	Token       string
	TargetType  string
	TargetURL   string
	Status      string
}

type Phase34AcquisitionStore interface {
	UserByID(context.Context, int) (User, bool, error)
	EmployeeIDByUserCorp(context.Context, int, int) (int, error)
	FirstEmployeeByUser(context.Context, int) (int, int, bool, error)
	Phase34AcquisitionLinkPage(context.Context, Phase34AcquisitionLinkFilter) (Phase34AcquisitionLinkPage, error)
	CreatePhase34AcquisitionLink(context.Context, Phase34AcquisitionLinkWrite) (int, error)
	Phase34AcquisitionLinkByID(context.Context, int, int) (Phase34AcquisitionLink, bool, error)
	DisablePhase34AcquisitionLink(context.Context, int, int) (bool, error)
	UpdatePhase34AcquisitionAuthorizationState(context.Context, int, string, string) error
	Phase34CustomerServicePage(context.Context, Phase34CustomerServiceFilter) (Phase34CustomerServicePage, error)
	CreatePhase34CustomerService(context.Context, Phase34CustomerServiceWrite) (int, error)
	Phase34CustomerServiceByID(context.Context, int, int) (Phase34CustomerService, bool, error)
	UpdatePhase34CustomerServiceSyncState(context.Context, int, string, string) error
	Phase34ShortLinkPage(context.Context, Phase34ShortLinkFilter) (Phase34ShortLinkPage, error)
	CreatePhase34ShortLink(context.Context, Phase34ShortLinkWrite) (Phase34ShortLink, error)
	Phase34ShortLinkByToken(context.Context, string) (Phase34ShortLink, bool, error)
	RecordPhase34ShortLinkVisit(context.Context, int, string, string, string) error
	DisablePhase34ShortLink(context.Context, int, int) (bool, error)
}

type Phase34AcquisitionExternalProvider interface {
	Authorize(context.Context, int) (string, error)
	SyncCustomerService(context.Context, int) error
}

// Phase34UnavailableExternalProvider keeps the production route explicit when
// the external WeCom capability is not configured. It never fabricates a URL
// or an active sync result.
type Phase34UnavailableExternalProvider struct{}

func NewPhase34UnavailableExternalProvider() Phase34AcquisitionExternalProvider {
	return Phase34UnavailableExternalProvider{}
}

func (Phase34UnavailableExternalProvider) Authorize(context.Context, int) (string, error) {
	return "", ErrPhase34AcquisitionProviderNotConfigured
}

func (Phase34UnavailableExternalProvider) SyncCustomerService(context.Context, int) error {
	return ErrPhase34AcquisitionProviderNotConfigured
}

type Phase34AcquisitionHandler struct {
	store      Phase34AcquisitionStore
	cache      LoginCache
	resolver   UserIDResolver
	authorizer CorpAdminAuthorizer
	external   Phase34AcquisitionExternalProvider
}

func NewPhase34AcquisitionHandler(store Phase34AcquisitionStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer, external Phase34AcquisitionExternalProvider) *Phase34AcquisitionHandler {
	return &Phase34AcquisitionHandler{store: store, cache: cache, resolver: resolver, authorizer: authorizer, external: external}
}

func (h *Phase34AcquisitionHandler) AcquisitionLinkIndex(w http.ResponseWriter, r *http.Request) {
	userID, corpID, _, ok := h.authorized(w, r, "/dashboard/acquisitionLink/index#get", http.MethodGet)
	if !ok {
		return
	}
	_ = userID
	page, err := h.store.Phase34AcquisitionLinkPage(r.Context(), Phase34AcquisitionLinkFilter{CorpID: corpID, Name: strings.TrimSpace(r.URL.Query().Get("name")), Status: strings.TrimSpace(r.URL.Query().Get("status")), Page: positiveQueryInt(r, "page", 1), PerPage: min(positiveQueryInt(r, "perPage", 20), 100)})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{"list": page.Items, "page": map[string]any{"total": page.Total, "perPage": page.PerPage, "totalPage": page.TotalPage}})
}

func (h *Phase34AcquisitionHandler) AcquisitionLinkStore(w http.ResponseWriter, r *http.Request) {
	userID, corpID, user, ok := h.authorized(w, r, "/dashboard/acquisitionLink/store#post", http.MethodPost)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	name, target := strings.TrimSpace(stringParam(params, "name")), strings.TrimSpace(stringParam(params, "targetUrl"))
	if name == "" || target == "" || !phase34SafeTargetURL(target) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "name and safe targetUrl are required", nil)
		return
	}
	id, err := h.store.CreatePhase34AcquisitionLink(r.Context(), Phase34AcquisitionLinkWrite{CorpID: corpID, UserID: userID, CreatorName: user.Name, Name: name, TargetURL: target, AuthorizationStatus: "unauthorized", Status: "draft"})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "create acquisition link failed", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{"id": id, "status": "draft", "authorizationStatus": "unauthorized"})
}

func (h *Phase34AcquisitionHandler) AcquisitionLinkAuthorize(w http.ResponseWriter, r *http.Request) {
	_, corpID, _, ok := h.authorized(w, r, "/dashboard/acquisitionLink/authorize#post", http.MethodPost)
	if !ok {
		return
	}
	if h.external == nil {
		writeEnvelope(w, http.StatusServiceUnavailable, http.StatusServiceUnavailable, ErrPhase34AcquisitionProviderNotConfigured.Error(), map[string]any{"status": "unauthorized"})
		return
	}
	url, err := h.external.Authorize(r.Context(), corpID)
	if err != nil {
		if updateErr := h.store.UpdatePhase34AcquisitionAuthorizationState(r.Context(), corpID, "failed", err.Error()); updateErr != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, updateErr.Error(), nil)
			return
		}
		writeEnvelope(w, http.StatusServiceUnavailable, http.StatusServiceUnavailable, err.Error(), map[string]any{"status": "failed"})
		return
	}
	if err := h.store.UpdatePhase34AcquisitionAuthorizationState(r.Context(), corpID, "authorizing", "draft"); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{"status": "authorizing", "authorizeUrl": url})
}

func (h *Phase34AcquisitionHandler) CustomerServiceIndex(w http.ResponseWriter, r *http.Request) {
	userID, corpID, _, ok := h.authorized(w, r, "/dashboard/customerService/index#get", http.MethodGet)
	if !ok {
		return
	}
	_ = userID
	page, err := h.store.Phase34CustomerServicePage(r.Context(), Phase34CustomerServiceFilter{CorpID: corpID, Name: strings.TrimSpace(r.URL.Query().Get("name")), Status: strings.TrimSpace(r.URL.Query().Get("status")), Page: positiveQueryInt(r, "page", 1), PerPage: min(positiveQueryInt(r, "perPage", 20), 100)})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{"list": page.Items, "page": map[string]any{"total": page.Total, "perPage": page.PerPage, "totalPage": page.TotalPage}})
}

func (h *Phase34AcquisitionHandler) CustomerServiceStore(w http.ResponseWriter, r *http.Request) {
	userID, corpID, user, ok := h.authorized(w, r, "/dashboard/customerService/store#post", http.MethodPost)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	name, account := strings.TrimSpace(stringParam(params, "name")), strings.TrimSpace(stringParam(params, "account"))
	if name == "" || account == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "name and account are required", nil)
		return
	}
	employeeIDs := []int{}
	if raw, ok := params["employeeIds"]; ok {
		employeeIDs, err = intSliceParam(map[string]any{"ids": raw}, "ids")
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "employeeIds must be an integer array", nil)
			return
		}
	}
	employeeJSON := mustJSON(employeeIDs)
	receiveMode := strings.TrimSpace(stringParam(params, "receiveMode"))
	if receiveMode == "" {
		receiveMode = "round_robin"
	}
	id, err := h.store.CreatePhase34CustomerService(r.Context(), Phase34CustomerServiceWrite{CorpID: corpID, UserID: userID, CreatorName: user.Name, Name: name, Account: account, EmployeeIDs: employeeJSON, ReceiveMode: receiveMode, Status: "pending_sync"})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "create customer service failed", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{"id": id, "status": "pending_sync"})
}

func (h *Phase34AcquisitionHandler) CustomerServiceSync(w http.ResponseWriter, r *http.Request) {
	_, corpID, _, ok := h.authorized(w, r, "/dashboard/customerService/sync#post", http.MethodPost)
	if !ok {
		return
	}
	if h.external == nil {
		if err := h.store.UpdatePhase34CustomerServiceSyncState(r.Context(), corpID, "failed", ErrPhase34AcquisitionProviderNotConfigured.Error()); err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		writeEnvelope(w, http.StatusServiceUnavailable, http.StatusServiceUnavailable, ErrPhase34AcquisitionProviderNotConfigured.Error(), map[string]any{"status": "failed"})
		return
	}
	if err := h.external.SyncCustomerService(r.Context(), corpID); err != nil {
		if updateErr := h.store.UpdatePhase34CustomerServiceSyncState(r.Context(), corpID, "failed", err.Error()); updateErr != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, updateErr.Error(), nil)
			return
		}
		writeEnvelope(w, http.StatusServiceUnavailable, http.StatusServiceUnavailable, err.Error(), map[string]any{"status": "failed"})
		return
	}
	if err := h.store.UpdatePhase34CustomerServiceSyncState(r.Context(), corpID, "active", ""); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{"status": "active"})
}

func (h *Phase34AcquisitionHandler) ShortLinkIndex(w http.ResponseWriter, r *http.Request) {
	userID, corpID, _, ok := h.authorized(w, r, "/dashboard/liveCodeShortChain/index#get", http.MethodGet)
	if !ok {
		return
	}
	_ = userID
	page, err := h.store.Phase34ShortLinkPage(r.Context(), Phase34ShortLinkFilter{CorpID: corpID, Name: strings.TrimSpace(r.URL.Query().Get("name")), Status: strings.TrimSpace(r.URL.Query().Get("status")), Page: positiveQueryInt(r, "page", 1), PerPage: min(positiveQueryInt(r, "perPage", 20), 100)})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{"list": page.Items, "page": map[string]any{"total": page.Total, "perPage": page.PerPage, "totalPage": page.TotalPage}})
}

func (h *Phase34AcquisitionHandler) ShortLinkStore(w http.ResponseWriter, r *http.Request) {
	userID, corpID, user, ok := h.authorized(w, r, "/dashboard/liveCodeShortChain/store#post", http.MethodPost)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	name, target := strings.TrimSpace(stringParam(params, "name")), strings.TrimSpace(stringParam(params, "targetUrl"))
	if name == "" || target == "" || !phase34SafeTargetURL(target) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "name and safe targetUrl are required", nil)
		return
	}
	token, err := phase34ShortLinkToken()
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "generate short link token failed", nil)
		return
	}
	targetType := strings.TrimSpace(stringParam(params, "targetType"))
	if targetType == "" {
		targetType = "url"
	}
	item, err := h.store.CreatePhase34ShortLink(r.Context(), Phase34ShortLinkWrite{CorpID: corpID, UserID: userID, CreatorName: user.Name, Name: name, Token: token, TargetType: targetType, TargetURL: target, Status: "active"})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "create short link failed", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", item)
}

func (h *Phase34AcquisitionHandler) ShortLinkDisable(w http.ResponseWriter, r *http.Request) {
	_, corpID, _, ok := h.authorized(w, r, "/dashboard/liveCodeShortChain/disable#post", http.MethodPost)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	id, present, err := intParam(params, "id")
	if err != nil || !present || id <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "id is required", nil)
		return
	}
	updated, err := h.store.DisablePhase34ShortLink(r.Context(), corpID, id)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !updated {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "short link not found", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{"id": id, "status": "disabled"})
}

func (h *Phase34AcquisitionHandler) ShortLinkRedirect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	token := strings.TrimPrefix(r.URL.Path, "/r/")
	if token == "" || strings.Contains(token, "/") {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "short link not found", nil)
		return
	}
	item, found, err := h.store.Phase34ShortLinkByToken(r.Context(), token)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "short link not found", nil)
		return
	}
	if item.Status != "active" || !phase34SafeTargetURL(item.TargetURL) {
		writeEnvelope(w, http.StatusGone, http.StatusGone, "short link is unavailable", nil)
		return
	}
	if err := h.store.RecordPhase34ShortLinkVisit(r.Context(), item.ID, token, r.Referer(), r.UserAgent()); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	http.Redirect(w, r, item.TargetURL, http.StatusFound)
}

func (h *Phase34AcquisitionHandler) authorized(w http.ResponseWriter, r *http.Request, permission string, method string) (int, int, User, bool) {
	if r.Method != method {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return 0, 0, User{}, false
	}
	if h.store == nil || h.resolver == nil {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return 0, 0, User{}, false
	}
	userID, err := h.resolver.UserID(r)
	if err != nil || userID <= 0 {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return 0, 0, User{}, false
	}
	user, found, err := h.store.UserByID(r.Context(), userID)
	if err != nil || !found {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "user not found", nil)
		return 0, 0, User{}, false
	}
	cacheValue := ""
	if h.cache != nil {
		cacheValue, err = h.cache.UserCorpCache(r.Context(), userID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return 0, 0, User{}, false
		}
	}
	login, err := ResolveValidatedLoginCorpInfoFromStore(r.Context(), r.Header, user, cacheValue, h.store)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return 0, 0, User{}, false
	}
	corpID, ok := selectedCorpID(w, login)
	if !ok {
		return 0, 0, User{}, false
	}
	if h.authorizer != nil {
		if _, err := h.authorizer.Resolve(r.Context(), userID, permission, corpID, login.WorkEmployeeID); err != nil {
			writeAccessError(w, err)
			return 0, 0, User{}, false
		}
	}
	return userID, corpID, user, true
}

func phase34SafeTargetURL(target string) bool {
	target = strings.TrimSpace(target)
	if target == "" || strings.HasPrefix(target, "//") || strings.ContainsAny(target, "\r\n") {
		return false
	}
	if strings.HasPrefix(strings.ToLower(target), "javascript:") || strings.HasPrefix(strings.ToLower(target), "data:") {
		return false
	}
	return strings.HasPrefix(target, "/")
}

func phase34ShortLinkToken() (string, error) {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
