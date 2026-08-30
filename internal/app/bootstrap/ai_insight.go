package bootstrap

import (
	"database/sql"
	"errors"

	appmodules "jiyi/mochat-go/internal/app/modules"
	aiinsight "jiyi/mochat-go/internal/modules/ai-insight"
	aiinsighthttp "jiyi/mochat-go/internal/modules/ai-insight/transport/http"
	aisettingsmysql "jiyi/mochat-go/internal/modules/ai-settings/adapters/mysql"
	"jiyi/mochat-go/internal/modules/providers"
)

type AIInsightDependencies struct {
	PrincipalResolver  aiinsighthttp.PrincipalResolver
	Authorizer         aiinsighthttp.Authorizer
	DB                 *sql.DB
	AIProviderResolver providers.AIProviderResolver
	AssistantContext   any
}

func RegisterAIInsight(router *appmodules.Router, enabled bool, dependencies AIInsightDependencies) error {
	if !enabled {
		return nil
	}
	if router == nil {
		return errors.New("route registrar is required")
	}
	module, err := aiinsight.New(aiinsight.Dependencies{
		PrincipalResolver:  dependencies.PrincipalResolver,
		Authorizer:         dependencies.Authorizer,
		DB:                 dependencies.DB,
		AIProviderResolver: dependencies.AIProviderResolver,
		AssistantContext:   dependencies.AssistantContext,
	})
	if err != nil {
		return err
	}
	return module.RegisterRoutes(router)
}

func NewAIInsightAssistantContext(db *sql.DB) (any, error) {
	if db == nil {
		return nil, errors.New("AI insight assistant database is required")
	}
	return aisettingsmysql.NewAgentRepository(db)
}
