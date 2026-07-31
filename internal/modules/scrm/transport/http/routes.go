package http

import nethttp "net/http"

const LeadsPath = "/api/phase2-2/scrm/leads"
const FormalLeadsPath = "/dashboard/scrm/leads"
const AssignmentReleasePath = AssignmentsPath + "/release"
const AssignmentClaimPath = AssignmentsPath + "/claim"

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

func RegisterCustomerLifecycleRoutes(registrar RouteRegistrar, handler *CustomerLifecycleHandler) error {
	if err := registrar.Handle(nethttp.MethodGet, AssignmentsPath, nethttp.HandlerFunc(handler.ListPublicPool)); err != nil {
		return err
	}
	if err := registrar.Handle(nethttp.MethodPut, AssignmentsPath, nethttp.HandlerFunc(handler.UpdateAssignment)); err != nil {
		return err
	}
	if err := registrar.Handle(nethttp.MethodPost, AssignmentReleasePath, nethttp.HandlerFunc(handler.ReleaseToPublicPool)); err != nil {
		return err
	}
	return registrar.Handle(nethttp.MethodPost, AssignmentClaimPath, nethttp.HandlerFunc(handler.ClaimFromPublicPool))
}

func RegisterOpportunityRoutes(registrar RouteRegistrar, handler *OpportunityHandler) error {
	if err := registrar.Handle(nethttp.MethodGet, OpportunitiesPath, nethttp.HandlerFunc(handler.List)); err != nil {
		return err
	}
	if err := registrar.Handle(nethttp.MethodPost, OpportunitiesPath, nethttp.HandlerFunc(handler.Create)); err != nil {
		return err
	}
	if err := registrar.Handle(nethttp.MethodPost, OpportunitiesPath+"/{id}/stage", nethttp.HandlerFunc(handler.Stage)); err != nil {
		return err
	}
	if err := registrar.Handle(nethttp.MethodGet, FollowUpsPath, nethttp.HandlerFunc(handler.ListFollowUps)); err != nil {
		return err
	}
	if err := registrar.Handle(nethttp.MethodPost, FollowUpsPath, nethttp.HandlerFunc(handler.AppendFollowUp)); err != nil {
		return err
	}
	if err := registrar.Handle(nethttp.MethodGet, TagsPath, nethttp.HandlerFunc(handler.ListTags)); err != nil {
		return err
	}
	if err := registrar.Handle(nethttp.MethodPost, TagsPath, nethttp.HandlerFunc(handler.CreateTag)); err != nil {
		return err
	}
	if err := registrar.Handle(nethttp.MethodPut, TagsPath+"/{id}", nethttp.HandlerFunc(handler.RenameTag)); err != nil {
		return err
	}
	return registrar.Handle(nethttp.MethodPost, TagsPath+"/{id}/contacts", nethttp.HandlerFunc(handler.BindTags))
}
