package bootstrap

import (
	"database/sql"
	"errors"

	appmodules "jiyi/mochat-go/internal/app/modules"
	chatmedia "jiyi/mochat-go/internal/modules/chat-media"
	scrmhttp "jiyi/mochat-go/internal/modules/scrm/transport/http"
)

type ChatMediaDependencies struct {
	DB              *sql.DB
	FileStorageRoot string
	PrincipalResolver scrmhttp.PrincipalResolver
	Authorizer        scrmhttp.LeadAuthorizer
}

func RegisterChatMedia(router *appmodules.Router, enabled bool, dependencies ChatMediaDependencies) error {
	if !enabled {
		return nil
	}
	if router == nil {
		return errors.New("route registrar is required")
	}
	module, err := chatmedia.New(chatmedia.Dependencies{
		DB:                dependencies.DB,
		FileStorageRoot:   dependencies.FileStorageRoot,
		PrincipalResolver: dependencies.PrincipalResolver,
		Authorizer:        dependencies.Authorizer,
	})
	if err != nil {
		return err
	}
	return module.RegisterRoutes(router)
}
