package main

import (
	"fmt"
	"net/http"

	appmodules "jiyi/mochat-go/internal/app/modules"
)

func registerDashboardAccessRoutes(registrar appmodules.RouteRegistrar, handler http.Handler) error {
	if err := registrar.Handle(http.MethodGet, "/dashboard/access/profile", handler); err != nil {
		return fmt.Errorf("register dashboard access profile: %w", err)
	}
	if err := registrar.Handle(http.MethodGet, "/dashboard/access/catalog", handler); err != nil {
		return fmt.Errorf("register dashboard access catalog: %w", err)
	}
	if err := registrar.Handle(http.MethodGet, "/dashboard/access/users", handler); err != nil {
		return fmt.Errorf("register dashboard access users: %w", err)
	}
	if err := registrar.Handle(http.MethodGet, "/dashboard/access/users/{id}", handler); err != nil {
		return fmt.Errorf("register dashboard access user: %w", err)
	}
	if err := registrar.Handle(http.MethodPut, "/dashboard/access/users/{id}", handler); err != nil {
		return fmt.Errorf("register dashboard access user update: %w", err)
	}
	if err := registrar.Handle(http.MethodGet, "/dashboard/access/roles", handler); err != nil {
		return fmt.Errorf("register dashboard access roles: %w", err)
	}
	if err := registrar.Handle(http.MethodPost, "/dashboard/access/roles", handler); err != nil {
		return fmt.Errorf("register dashboard access role create: %w", err)
	}
	if err := registrar.Handle(http.MethodPut, "/dashboard/access/roles/{id}", handler); err != nil {
		return fmt.Errorf("register dashboard access role update: %w", err)
	}
	if err := registrar.Handle(http.MethodPut, "/dashboard/access/roles/{id}/status", handler); err != nil {
		return fmt.Errorf("register dashboard access role status: %w", err)
	}
	if err := registrar.Handle(http.MethodDelete, "/dashboard/access/roles/{id}", handler); err != nil {
		return fmt.Errorf("register dashboard access role delete: %w", err)
	}
	if err := registrar.Handle(http.MethodGet, "/dashboard/access/audits", handler); err != nil {
		return fmt.Errorf("register dashboard access audits: %w", err)
	}
	return nil
}
