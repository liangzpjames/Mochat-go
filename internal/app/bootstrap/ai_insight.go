package bootstrap

import (
	"errors"

	appmodules "jiyi/mochat-go/internal/app/modules"
	aiinsight "jiyi/mochat-go/internal/modules/ai-insight"
	aiinsighthttp "jiyi/mochat-go/internal/modules/ai-insight/transport/http"
)

type AIInsightDependencies struct {
	PrincipalResolver aiinsighthttp.PrincipalResolver
	Authorizer        aiinsighthttp.Authorizer
}

func RegisterAIInsight(router *appmodules.Router, enabled bool, dependencies AIInsightDependencies) error {
	if !enabled {
		return nil
	}
	if router == nil {
		return errors.New("route registrar is required")
	}
	module, err := aiinsight.New(aiinsight.Dependencies{
		PrincipalResolver: dependencies.PrincipalResolver,
		Authorizer:        dependencies.Authorizer,
	})
	if err != nil {
		return err
	}
	return module.RegisterRoutes(router)
}
