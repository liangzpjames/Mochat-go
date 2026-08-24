package dashboardprincipal

import (
	"context"
	"sort"

	"jiyi/mochat-go/internal/wecomcapability"
)

type capabilityAccess struct {
	SuperAdmin      bool
	PermissionCodes []string
}

type capabilityAccessKey struct{}

func WithCapabilityAccess(ctx context.Context, superadmin bool, permissionCodes []string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, capabilityAccessKey{}, capabilityAccess{
		SuperAdmin: superadmin, PermissionCodes: append([]string(nil), permissionCodes...),
	})
}

// HasPermissionCode keeps service-layer authorization bound to the exact
// permission codes attached by the Dashboard access guard. Missing or
// inconsistent context fails closed for ordinary users.
func HasPermissionCode(ctx context.Context, principal DashboardPrincipal, code string) bool {
	if principal.IsSuperAdmin {
		return true
	}
	if ctx == nil || code == "" {
		return false
	}
	access, ok := ctx.Value(capabilityAccessKey{}).(capabilityAccess)
	if !ok || access.SuperAdmin {
		return false
	}
	for _, allowed := range access.PermissionCodes {
		if allowed == code {
			return true
		}
	}
	return false
}

// VisibleProviderCapabilities is an allowlist derived from exact dashboard
// page permission codes. Unknown permissions expose nothing.
func VisibleProviderCapabilities(ctx context.Context, principal DashboardPrincipal) []string {
	if principal.IsSuperAdmin {
		return append([]string(nil), wecomcapability.All...)
	}
	access, ok := ctx.Value(capabilityAccessKey{}).(capabilityAccess)
	if !ok || access.SuperAdmin {
		return nil
	}
	seen := make(map[string]struct{})
	for _, code := range access.PermissionCodes {
		for _, capability := range ProviderPageCapabilityMapping[code] {
			if capability != "" {
				seen[capability] = struct{}{}
			}
		}
	}
	visible := make([]string, 0, len(seen))
	for capability := range seen {
		visible = append(visible, capability)
	}
	sort.Strings(visible)
	return visible
}

// ProviderPageCapabilityMapping is intentionally exact and is cross-checked
// against internal/dashboard/dashboard_page_catalog.json in tests.
var ProviderPageCapabilityMapping = map[string][]string{
	"dashboard.company_setting.website":             append(append([]string(nil), wecomcapability.All...), "archive_sync"),
	"dashboard.index":                               {wecomcapability.EmployeeSync, wecomcapability.DepartmentSync},
	"dashboard.chat.v2_all":                         {wecomcapability.EmployeeSync, wecomcapability.ExternalContactSync, wecomcapability.RoomSync},
	"dashboard.chat.v2_staff":                       {wecomcapability.EmployeeSync, wecomcapability.DepartmentSync},
	"dashboard.chat.v2_customer":                    {wecomcapability.ExternalContactSync},
	"dashboard.chat.v2_group":                       {wecomcapability.RoomSync},
	"dashboard.customer.inheritance":                {wecomcapability.ContactTransfer},
	"dashboard.acquisition.v2_channel_code":         {wecomcapability.ContactWay},
	"dashboard.acquisition.group_code":              {wecomcapability.ContactWay},
	"dashboard.acquisition.wechat_customer_service": {wecomcapability.AgentMessage},
	"dashboard.acquisition.group_template":          {wecomcapability.ContactWay},
	"dashboard.acquisition.precise_group_send":      {wecomcapability.ContactBatchSend, wecomcapability.RoomBatchSend},
	"dashboard.customer.contact":                    {wecomcapability.ExternalContactSync},
	"dashboard.customer.friends":                    {wecomcapability.ExternalContactSync},
	"dashboard.customer.group":                      {wecomcapability.RoomSync},
	"dashboard.customer.tags":                       {wecomcapability.ContactTagSync},
}
