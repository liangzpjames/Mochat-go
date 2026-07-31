package http

import nethttp "net/http"

const LeadsPath = "/api/phase2-2/scrm/leads"
const FormalLeadsPath = "/dashboard/scrm/leads"

type RouteRegistrar interface {
	Handle(method, pattern string, handler nethttp.Handler) error
}

func RegisterRoutes(registrar RouteRegistrar, handler *LeadHandler) error {
	for _, path := range []string{LeadsPath, FormalLeadsPath} {
		if err := registrar.Handle(nethttp.MethodPost, path, nethttp.HandlerFunc(handler.Create)); err != nil {
			return err
		}
		if err := registrar.Handle(nethttp.MethodGet, path, nethttp.HandlerFunc(handler.List)); err != nil {
			return err
		}
	}
	return nil
}
