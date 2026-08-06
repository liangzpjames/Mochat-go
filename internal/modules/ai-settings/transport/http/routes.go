package http

import (
	"net/http"

	appmodules "jiyi/mochat-go/internal/app/modules"
)

const (
	KnowledgeBasesPath = "/dashboard/ai-settings/knowledge-bases"
	AgentsPath         = "/dashboard/ai-settings/agents"
)

func RegisterRoutes(registrar appmodules.RouteRegistrar, knowledgeBases *KnowledgeBaseHandler, agents *AgentHandler) error {
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		if err := registrar.Handle(method, KnowledgeBasesPath, knowledgeBases); err != nil {
			return err
		}
		if err := registrar.Handle(method, AgentsPath, agents); err != nil {
			return err
		}
	}
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		if err := registrar.Handle(method, KnowledgeBasesPath+"/{id}", knowledgeBases); err != nil {
			return err
		}
		if err := registrar.Handle(method, AgentsPath+"/{id}", agents); err != nil {
			return err
		}
	}
	return nil
}
