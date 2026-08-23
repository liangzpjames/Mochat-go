package aisettings

import (
	"database/sql"
	"errors"

	appmodules "jiyi/mochat-go/internal/app/modules"
	"jiyi/mochat-go/internal/modules/ai-settings/adapters/mysql"
	transporthttp "jiyi/mochat-go/internal/modules/ai-settings/transport/http"
)

type Dependencies struct {
	DB                *sql.DB
	FileStorageRoot   string
	PrincipalResolver transporthttp.PrincipalResolver
	Authorizer        transporthttp.Authorizer
}

type Module struct {
	knowledgeBases *transporthttp.KnowledgeBaseHandler
	agents         *transporthttp.AgentHandler
	documents      *transporthttp.DocumentHandler
}

func New(dependencies Dependencies) (*Module, error) {
	if dependencies.DB == nil {
		return nil, errors.New("AI settings database is required")
	}
	if dependencies.PrincipalResolver == nil {
		return nil, errors.New("AI settings principal resolver is required")
	}
	kbRepo, err := mysql.NewKnowledgeBaseRepository(dependencies.DB)
	if err != nil {
		return nil, err
	}
	agentRepo, err := mysql.NewAgentRepository(dependencies.DB)
	if err != nil {
		return nil, err
	}
	documentRepo, err := mysql.NewDocumentRepository(dependencies.DB)
	if err != nil {
		return nil, err
	}
	privateStorage, err := NewPrivateStorage(dependencies.FileStorageRoot)
	if err != nil {
		return nil, err
	}
	generate := mysql.NewIDGenerator()
	return &Module{
		knowledgeBases: transporthttp.NewKnowledgeBaseHandler(kbRepo, agentRepo, dependencies.PrincipalResolver, dependencies.Authorizer, generate, documentRepo),
		agents:         transporthttp.NewAgentHandler(agentRepo, kbRepo, dependencies.PrincipalResolver, dependencies.Authorizer, generate),
		documents:      transporthttp.NewDocumentHandler(documentRepo, kbRepo, privateStorage, ParseDocument, dependencies.PrincipalResolver, dependencies.Authorizer, generate),
	}, nil
}

func (m *Module) RegisterRoutes(registrar appmodules.RouteRegistrar) error {
	if m == nil || m.knowledgeBases == nil {
		return errors.New("AI settings module is not initialized")
	}
	return transporthttp.RegisterRoutes(registrar, m.knowledgeBases, m.agents, m.documents)
}
