package http

import (
	"net/http"

	appmodules "jiyi/mochat-go/internal/app/modules"
)

func RegisterRoutes(registrar appmodules.RouteRegistrar, handler *InsightHandler) error {
	for _, page := range []string{"session-analysis", "smart-analysis", "emotion", "employee-score", "communication-keyword"} {
		if err := registrar.Handle(http.MethodGet, "/dashboard/ai-insight/"+page, handler); err != nil {
			return err
		}
	}
	return nil
}

func RegisterWorkspaceRoutes(registrar appmodules.RouteRegistrar, handler http.Handler) error {
	routes := []struct{ method, path string }{
		{http.MethodGet, "/dashboard/ai-insight/session-analysis/records"},
		{http.MethodGet, "/dashboard/ai-insight/session-analysis/detail"},
		{http.MethodGet, "/dashboard/ai-insight/session-analysis/status"},
		{http.MethodGet, "/dashboard/ai-insight/session-analysis/filter-options"},
		{http.MethodGet, "/dashboard/ai-insight/session-analysis/export"},
		{http.MethodGet, "/dashboard/ai-insight/smart-analysis/records"},
		{http.MethodGet, "/dashboard/ai-insight/smart-analysis/detail"},
		{http.MethodGet, "/dashboard/ai-insight/smart-analysis/status"},
		{http.MethodGet, "/dashboard/ai-insight/smart-analysis/filter-options"},
		{http.MethodGet, "/dashboard/ai-insight/smart-analysis/rules"},
		{http.MethodPost, "/dashboard/ai-insight/smart-analysis/rules"},
		{http.MethodPut, "/dashboard/ai-insight/smart-analysis/rules"},
		{http.MethodDelete, "/dashboard/ai-insight/smart-analysis/rules"},
		{http.MethodPost, "/dashboard/ai-insight/smart-analysis/rules/status"},
		{http.MethodGet, "/dashboard/ai-insight/emotion/records"},
		{http.MethodGet, "/dashboard/ai-insight/emotion/detail"},
		{http.MethodGet, "/dashboard/ai-insight/emotion/status"},
		{http.MethodGet, "/dashboard/ai-insight/emotion/filter-options"},
		{http.MethodGet, "/dashboard/ai-insight/emotion/export"},
		{http.MethodGet, "/dashboard/ai-insight/employee-score/records"},
		{http.MethodGet, "/dashboard/ai-insight/employee-score/detail"},
		{http.MethodGet, "/dashboard/ai-insight/employee-score/status"},
		{http.MethodGet, "/dashboard/ai-insight/employee-score/filter-options"},
		{http.MethodGet, "/dashboard/ai-insight/employee-score/export"},
		{http.MethodGet, "/dashboard/ai-insight/communication-keyword/records"},
		{http.MethodGet, "/dashboard/ai-insight/communication-keyword/detail"},
		{http.MethodGet, "/dashboard/ai-insight/communication-keyword/status"},
		{http.MethodGet, "/dashboard/ai-insight/communication-keyword/filter-options"},
		{http.MethodGet, "/dashboard/ai-insight/communication-keyword/export"},
		{http.MethodPost, "/dashboard/ai-insight/run"},
	}
	for _, route := range routes {
		if err := registrar.Handle(route.method, route.path, handler); err != nil {
			return err
		}
	}
	return nil
}
