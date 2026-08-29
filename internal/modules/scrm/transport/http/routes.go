package http

import nethttp "net/http"

const LeadsPath = "/api/phase2-2/scrm/leads"
const FormalLeadsPath = "/dashboard/scrm/leads"
const LeadAssignmentsPath = FormalLeadsPath + "/assignments"
const LeadTransitionPath = FormalLeadsPath + "/transition"
const LeadDuplicatesPath = FormalLeadsPath + "/duplicates"
const AssignmentReleasePath = AssignmentsPath + "/release"
const AssignmentClaimPath = AssignmentsPath + "/claim"
const AssignmentBatchClaimPath = AssignmentClaimPath + "/batch"
const TagItemPath = TagsPath + "/{id}"
const TagGroupItemPath = TagGroupsPath + "/{id}"
const TagMovePath = TagItemPath + "/move"
const TagContactsPath = TagItemPath + "/contacts"
const TagDeletePreviewPath = TagItemPath + "/delete-preview"

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
	if err := registrar.Handle(nethttp.MethodPost, LeadAssignmentsPath, nethttp.HandlerFunc(handler.Assign)); err != nil {
		return err
	}
	if err := registrar.Handle(nethttp.MethodPost, LeadTransitionPath, nethttp.HandlerFunc(handler.Transition)); err != nil {
		return err
	}
	if err := registrar.Handle(nethttp.MethodGet, LeadDuplicatesPath, nethttp.HandlerFunc(handler.Duplicates)); err != nil {
		return err
	}
	return nil
}

func RegisterCustomerLifecycleRoutes(registrar RouteRegistrar, handler *CustomerLifecycleHandler) error {
	if err := registrar.Handle(nethttp.MethodGet, ContactsPath, nethttp.HandlerFunc(handler.ListContacts)); err != nil {
		return err
	}
	if err := registrar.Handle(nethttp.MethodPost, ContactsPath, nethttp.HandlerFunc(handler.CreateContact)); err != nil {
		return err
	}
	if err := registrar.Handle(nethttp.MethodGet, ContactsPath+"/{id}", nethttp.HandlerFunc(handler.GetContact)); err != nil {
		return err
	}
	if err := registrar.Handle(nethttp.MethodGet, AssignmentsPath, nethttp.HandlerFunc(handler.ListPublicPool)); err != nil {
		return err
	}
	if err := registrar.Handle(nethttp.MethodPut, AssignmentsPath, nethttp.HandlerFunc(handler.UpdateAssignment)); err != nil {
		return err
	}
	if err := registrar.Handle(nethttp.MethodPost, AssignmentReleasePath, nethttp.HandlerFunc(handler.ReleaseToPublicPool)); err != nil {
		return err
	}
	if err := registrar.Handle(nethttp.MethodPost, AssignmentClaimPath, nethttp.HandlerFunc(handler.ClaimFromPublicPool)); err != nil {
		return err
	}
	return registrar.Handle(nethttp.MethodPost, AssignmentBatchClaimPath, nethttp.HandlerFunc(handler.BatchClaimFromPublicPool))
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
	return nil
}

func RegisterCustomerTagRoutes(registrar RouteRegistrar, handler *CustomerTagHandler) error {
	for _, route := range []struct {
		method, path string
		handler      nethttp.HandlerFunc
	}{
		{nethttp.MethodGet, TagsPath, handler.ListCatalog},
		{nethttp.MethodPost, TagsPath, handler.CreateTag},
		{nethttp.MethodPut, TagItemPath, handler.RenameTag},
		{nethttp.MethodGet, TagGroupsPath, handler.ListCatalog},
		{nethttp.MethodPost, TagGroupsPath, handler.CreateGroup},
		{nethttp.MethodPut, TagGroupItemPath, handler.RenameGroup},
		{nethttp.MethodPost, TagMovePath, handler.MoveTag},
		{nethttp.MethodPut, TagContactsPath, handler.MaintainContacts},
		{nethttp.MethodGet, TagDeletePreviewPath, handler.PreviewDeleteTag},
		{nethttp.MethodDelete, TagItemPath, handler.DeleteTag},
	} {
		if err := registrar.Handle(route.method, route.path, route.handler); err != nil {
			return err
		}
	}
	return nil
}
