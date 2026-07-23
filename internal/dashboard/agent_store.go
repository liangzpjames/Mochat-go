package dashboard

import (
	"context"
	"net/http"
)

type WorkAgentWriteValues struct {
	CorpID    int
	WXAgentID string
	WXSecret  string
	Type      int
}

type WorkAgentDetail struct {
	Name               string
	SquareLogoURL      string
	Description        string
	Close              int
	RedirectDomain     string
	ReportLocationFlag int
	IsReportEnter      int
	HomeURL            string
}

type DashboardAgentStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	RoomWelcomeCorpCredentialByID(ctx context.Context, corpID int) (RoomWelcomeCorpCredential, bool, error)
	CreateWorkAgent(ctx context.Context, values WorkAgentWriteValues, detail WorkAgentDetail) (int, error)
}

type DashboardAgentWeComClient interface {
	AgentDetail(ctx context.Context, credential RoomWelcomeCorpCredential, wxSecret string, wxAgentID string) (WorkAgentDetail, error)
}

type DashboardAgentHandler struct {
	store    DashboardAgentStore
	cache    LoginCache
	resolver UserIDResolver
	wecom    DashboardAgentWeComClient
}

func NewDashboardAgentHandler(store DashboardAgentStore, cache LoginCache, resolver UserIDResolver, wecom DashboardAgentWeComClient) *DashboardAgentHandler {
	return &DashboardAgentHandler{store: store, cache: cache, resolver: resolver, wecom: wecom}
}

func (h *DashboardAgentHandler) Store(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if h.wecom == nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "企业微信客户端未配置", nil)
		return
	}
	_, user, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if len(loginInfo.CorpIDs) != 1 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "未选择登录企业，不可操作", nil)
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	values, ok := dashboardAgentWriteValues(w, params, loginInfo.CorpIDs[0])
	if !ok {
		return
	}
	if !enforceSaaSQuota(r.Context(), w, h.store, user.TenantID, SaaSMetricAgents, 1) {
		return
	}
	credential, found, err := h.store.RoomWelcomeCorpCredentialByID(r.Context(), values.CorpID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found || credential.WXCorpID == "" {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "应用创建失败,请输入正确的应用id和应用secret", nil)
		return
	}
	detail, err := h.wecom.AgentDetail(r.Context(), credential, values.WXSecret, values.WXAgentID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "应用创建失败,请输入正确的应用id和应用secret", nil)
		return
	}
	if _, err := h.store.CreateWorkAgent(r.Context(), values, detail); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "应用创建失败,请输入正确的应用id和应用secret", nil)
		return
	}
	if err := refreshSaaSUsageCounter(r.Context(), h.store, user.TenantID, SaaSMetricAgents); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "应用用量刷新失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *DashboardAgentHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (int, User, LoginCorpInfo, bool) {
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

func dashboardAgentWriteValues(w http.ResponseWriter, params map[string]any, corpID int) (WorkAgentWriteValues, bool) {
	wxAgentID := stringParam(params, "wxAgentId")
	if wxAgentID == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "企业应用ID 必填", nil)
		return WorkAgentWriteValues{}, false
	}
	wxSecret := stringParam(params, "wxSecret")
	if wxSecret == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "企业应用secret 必填", nil)
		return WorkAgentWriteValues{}, false
	}
	agentType, ok, err := intParam(params, "type")
	if !ok {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "企业应用类型 必填", nil)
		return WorkAgentWriteValues{}, false
	}
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "企业应用类型 必须为整型", nil)
		return WorkAgentWriteValues{}, false
	}
	return WorkAgentWriteValues{
		CorpID:    corpID,
		WXAgentID: wxAgentID,
		WXSecret:  wxSecret,
		Type:      agentType,
	}, true
}
