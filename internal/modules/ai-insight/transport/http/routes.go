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
