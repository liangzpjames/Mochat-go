package aiinsight

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	appmodules "jiyi/mochat-go/internal/app/modules"
	transporthttp "jiyi/mochat-go/internal/modules/ai-insight/transport/http"
	aisettingsmysql "jiyi/mochat-go/internal/modules/ai-settings/adapters/mysql"
	"jiyi/mochat-go/internal/modules/providers"
)

type Dependencies struct {
	PrincipalResolver  transporthttp.PrincipalResolver
	Authorizer         transporthttp.Authorizer
	DB                 *sql.DB
	AIProviderResolver providers.AIProviderResolver
}

type Module struct {
	handler   *transporthttp.InsightHandler
	workspace *WorkspaceHandler
}

func New(dependencies Dependencies) (*Module, error) {
	if dependencies.PrincipalResolver == nil {
		return nil, errors.New("AI insight principal resolver is required")
	}
	handler := transporthttp.NewInsightHandlerWithStore(dependencies.PrincipalResolver, dependencies.Authorizer, dependencies.DB)
	var workspace *WorkspaceHandler
	if dependencies.DB != nil {
		var workspaceAuthorizer WorkspaceAuthorizer
		if dependencies.Authorizer != nil {
			workspaceAuthorizer = workspaceAuthorizerAdapter{authorizer: dependencies.Authorizer}
		}
		assistantRepo, _ := aisettingsmysql.NewAgentRepository(dependencies.DB)
		if dependencies.AIProviderResolver != nil {
			workspace = NewWorkspaceHandlerWithResolver(workspacePrincipalAdapter{resolver: dependencies.PrincipalResolver}, workspaceAuthorizer, NewSQLRepository(dependencies.DB), dependencies.AIProviderResolver, assistantRepo)
		}
	}
	return &Module{handler: handler, workspace: workspace}, nil
}

type workspacePrincipalAdapter struct {
	resolver transporthttp.PrincipalResolver
}

func (a workspacePrincipalAdapter) Resolve(r *http.Request) (WorkspacePrincipal, error) {
	p, err := a.resolver.Resolve(r)
	if err != nil {
		return WorkspacePrincipal{}, err
	}
	return WorkspacePrincipal{UserID: p.UserID, TenantID: p.TenantID, CorpID: p.CorpID, AllowedEmployeeIDs: p.AllowedEmployeeIDs, EmployeeScopeRestricted: p.EmployeeScopeRestricted, CanRunAnalysis: p.IsSuperAdmin}, nil
}

type workspaceAuthorizerAdapter struct{ authorizer transporthttp.Authorizer }

func (a workspaceAuthorizerAdapter) Authorize(ctx context.Context, p WorkspacePrincipal, corpID int64, permission string) error {
	return a.authorizer.Authorize(ctx, transporthttp.Principal{UserID: p.UserID, TenantID: p.TenantID, CorpID: p.CorpID, AllowedEmployeeIDs: p.AllowedEmployeeIDs, EmployeeScopeRestricted: p.EmployeeScopeRestricted}, corpID, permission)
}

func (m *Module) RegisterRoutes(registrar appmodules.RouteRegistrar) error {
	if m == nil || m.handler == nil {
		return errors.New("AI insight module is not initialized")
	}
	if err := transporthttp.RegisterRoutes(registrar, m.handler); err != nil {
		return err
	}
	if m.workspace != nil {
		return transporthttp.RegisterWorkspaceRoutes(registrar, m.workspace)
	}
	return nil
}
