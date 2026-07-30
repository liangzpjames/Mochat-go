package http

import nethttp "net/http"

const LeadsPath = "/api/phase2-2/scrm/leads"

type RouteRegistrar interface {
	Handle(method, pattern string, handler nethttp.Handler) error
}

func RegisterRoutes(registrar RouteRegistrar, handler *LeadHandler) error {
	if err := registrar.Handle(nethttp.MethodPost, LeadsPath, nethttp.HandlerFunc(handler.Create)); err != nil {
		return err
	}
	return registrar.Handle(nethttp.MethodGet, LeadsPath, nethttp.HandlerFunc(handler.List))
}
