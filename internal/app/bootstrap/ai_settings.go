package bootstrap

import (
	"database/sql"
	"errors"

	appmodules "jiyi/mochat-go/internal/app/modules"
	aisettings "jiyi/mochat-go/internal/modules/ai-settings"
	aisettingshttp "jiyi/mochat-go/internal/modules/ai-settings/transport/http"
)

type AISettingsDependencies struct {
	DB                *sql.DB
	FileStorageRoot   string
	PrincipalResolver aisettingshttp.PrincipalResolver
	Authorizer        aisettingshttp.Authorizer
}

func RegisterAISettings(router *appmodules.Router, enabled bool, dependencies AISettingsDependencies) error {
	if !enabled {
		return nil
	}
	if router == nil {
		return errors.New("route registrar is required")
	}
	module, err := aisettings.New(aisettings.Dependencies{
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
