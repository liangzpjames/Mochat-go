package main

import (
	"errors"
	"fmt"
	"log"

	appbootstrap "jiyi/mochat-go/internal/app/bootstrap"
	appmodules "jiyi/mochat-go/internal/app/modules"
	"jiyi/mochat-go/internal/authjwt"
	"jiyi/mochat-go/internal/config"
	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/store"
)

func newUserResolverBuilder(
	cfg config.Config,
	identitySessionChecker authjwt.SessionChecker,
	getRedisStore func() *store.RedisStore,
) func(string) (dashboard.UserIDResolver, dashboard.LoginCache) {
	return func(routeName string) (dashboard.UserIDResolver, dashboard.LoginCache) {
		if cfg.DevAuthHeader {
			log.Printf("%s auth resolver: development header X-Mochat-Go-User-ID", routeName)
			return dashboard.HeaderUserIDResolver{}, nil
		}
		if cfg.SkipJWTBlacklist {
			log.Printf("%s auth resolver: PHP simple-jwt compatible parser without Redis blacklist checks", routeName)
			return authjwt.Parser{
				Secret:        cfg.SimpleJWTSecret,
				Prefix:        cfg.SimpleJWTPrefix,
				SkipBlacklist: true,
				Sessions:      identitySessionChecker,
			}, nil
		}
		redisStore := getRedisStore()
		log.Printf("%s auth resolver: PHP simple-jwt compatible parser", routeName)
		return authjwt.Parser{
			Secret:        cfg.SimpleJWTSecret,
			Prefix:        cfg.SimpleJWTPrefix,
			Blacklist:     redisStore,
			SkipBlacklist: cfg.SkipJWTBlacklist,
			Sessions:      identitySessionChecker,
		}, redisStore
	}
}

func newSCRMModuleRouter(
	cfg config.Config,
	getMySQLStore func() *store.MySQLStore,
	buildUserResolver func(string) (dashboard.UserIDResolver, dashboard.LoginCache),
) (*appmodules.Router, error) {
	router := appmodules.NewRouter()
	dependencies := appbootstrap.SCRMDependencies{}
	if cfg.EnablePhase22SCRMPilot {
		if getMySQLStore == nil || buildUserResolver == nil {
			return nil, errors.New("SCRM runtime dependencies are required")
		}
		mysqlStore := getMySQLStore()
		if mysqlStore == nil {
			return nil, errors.New("SCRM MySQL store is required")
		}
		userIDs, _ := buildUserResolver("SCRM pilot")
		principalResolver, err := appbootstrap.NewSCRMPrincipalResolver(userIDs, mysqlStore)
		if err != nil {
			return nil, fmt.Errorf("build SCRM principal resolver: %w", err)
		}
		dependencies.DB = mysqlStore.DB()
		dependencies.PrincipalResolver = principalResolver
		leadAuthorizer, err := appbootstrap.NewSCRMLeadAuthorizer(mysqlStore, dashboard.NewRBACResolver(mysqlStore))
		if err != nil {
			return nil, fmt.Errorf("build SCRM lead authorizer: %w", err)
		}
		dependencies.LeadAuthorizer = leadAuthorizer
	}
	if err := appbootstrap.RegisterSCRM(router, cfg.EnablePhase22SCRMPilot, dependencies); err != nil {
		return nil, fmt.Errorf("register SCRM pilot module: %w", err)
	}
	return router, nil
}
