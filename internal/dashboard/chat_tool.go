package dashboard

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type WorkAgent struct {
	ID            int
	CorpID        int
	Name          string
	SquareLogoURL string
}

type ChatTool struct {
	ID       int
	PageName string
	PageFlag string
}

type ChatToolConfigStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	WorkAgentsByCorpID(ctx context.Context, corpID int) ([]WorkAgent, error)
	EnabledChatTools(ctx context.Context) ([]ChatTool, error)
}

type ChatToolConfigHandler struct {
	store          ChatToolConfigStore
	cache          LoginCache
	resolver       UserIDResolver
	sidebarBaseURL string
	apiBaseURL     string
}

func NewChatToolConfigHandler(store ChatToolConfigStore, cache LoginCache, resolver UserIDResolver, sidebarBaseURL string, apiBaseURL string) *ChatToolConfigHandler {
	return &ChatToolConfigHandler{
		store:          store,
		cache:          cache,
		resolver:       resolver,
		sidebarBaseURL: strings.TrimRight(sidebarBaseURL, "/"),
		apiBaseURL:     strings.TrimRight(apiBaseURL, "/"),
	}
}

func (h *ChatToolConfigHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}

	requestPrincipal, err := DashboardPrincipalFromContext(r.Context())
	userID := requestPrincipal.UserID
	if err != nil || userID <= 0 {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return
	}

	_, found, err := h.store.UserByID(r.Context(), userID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "user not found", nil)
		return
	}
	principalScope, err := DashboardRequestScopeFromContext(r.Context())
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if len(principalScope.CorpIDs) == 0 || principalScope.CorpIDs[0] <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "未选择登录企业，不可操作", nil)
		return
	}

	agents, err := h.store.WorkAgentsByCorpID(r.Context(), principalScope.CorpIDs[0])
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if len(agents) == 0 {
		writeEnvelope(w, http.StatusOK, 200, "success", []any{})
		return
	}

	tools, err := h.store.EnabledChatTools(r.Context())
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	payloadAgents := make([]map[string]any, 0, len(agents))
	for _, agent := range agents {
		payloadAgents = append(payloadAgents, map[string]any{
			"id":            agent.ID,
			"corpId":        agent.CorpID,
			"name":          agent.Name,
			"squareLogoUrl": agent.SquareLogoURL,
			"chatTools":     h.chatToolsPayload(agent.ID, tools),
		})
	}

	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"agents":       payloadAgents,
		"whiteDomains": []string{h.sidebarBaseURL, h.apiBaseURL},
	})
}

func (h *ChatToolConfigHandler) chatToolsPayload(agentID int, tools []ChatTool) []map[string]any {
	payload := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		pageFlag := normalizeChatToolPageFlag(tool.PageFlag)
		payload = append(payload, map[string]any{
			"id":       tool.ID,
			"pageName": tool.PageName,
			"pageFlag": pageFlag,
			"pageUrl":  chatToolPageURL(h.sidebarBaseURL, pageFlag, agentID),
		})
	}
	return payload
}

func normalizeChatToolPageFlag(pageFlag string) string {
	switch pageFlag {
	case "customer":
		return "contact"
	case "mediumGroup":
		return "medium"
	default:
		return pageFlag
	}
}

func chatToolPageURL(baseURL string, pageFlag string, agentID int) string {
	values := url.Values{}
	values.Set("agentId", strconv.Itoa(agentID))
	return strings.TrimRight(baseURL, "/") + "/" + pageFlag + "?" + values.Encode()
}
