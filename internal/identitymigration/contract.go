package identitymigration

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

type CorpMapping struct {
	TenantID int64 `json:"tenantId"`
	CorpID   int64 `json:"corpId"`
}

type MappingDocument struct {
	Version           int           `json:"version"`
	Entries           []CorpMapping `json:"entries"`
	Signature         string        `json:"signature"`
	SignatureVerified bool          `json:"-"`
}

type Options struct {
	PlatformTenantID int64
	Mapping          MappingDocument
}

type ActorColumn struct {
	Table  string
	Column string
	Kind   ActorColumnKind
}

type ActorColumnKind string

const (
	ActorColumnKindActor          ActorColumnKind = "actor"
	ActorColumnKindNonActorTarget ActorColumnKind = "non_actor_target"
)

var allowedActorColumns = map[string]struct{}{
	"mochat_go_saas_admin_user_access.user_id":                         {},
	"mochat_go_saas_admin_user_access.updated_by":                      {},
	"mochat_go_saas_admin_user_roles.user_id":                          {},
	"mochat_go_saas_admin_user_roles.assigned_by":                      {},
	"mochat_go_saas_admin_roles.created_by":                            {},
	"mochat_go_saas_admin_roles.updated_by":                            {},
	"mochat_go_saas_admin_operation_logs.actor_user_id":                {},
	"mochat_go_saas_admin_approvals.requester_user_id":                 {},
	"mochat_go_saas_admin_approvals.reviewer_user_id":                  {},
	"mochat_go_saas_admin_approvals.execution_user_id":                 {},
	"mochat_go_saas_admin_approval_events.actor_user_id":               {},
	"mochat_go_saas_admin_approval_decisions.reviewer_user_id":         {},
	"mochat_go_saas_admin_approval_decisions.delegated_from_user_id":   {},
	"mochat_go_saas_admin_approval_delegations.delegator_user_id":      {},
	"mochat_go_saas_admin_approval_delegations.delegate_user_id":       {},
	"mochat_go_saas_admin_approval_delegations.created_by":             {},
	"mochat_go_saas_admin_approval_delegations.updated_by":             {},
	"mochat_go_saas_admin_approval_policies.updated_by":                {},
	"mochat_go_saas_idempotency_receipts.created_by_saas_user_id":      {},
	"mochat_go_dashboard_identity_activations.created_by_saas_user_id": {},
}

var systemActorZeroAllowed = map[string]struct{}{
	"mochat_go_saas_admin_roles.created_by":                          {},
	"mochat_go_saas_admin_roles.updated_by":                          {},
	"mochat_go_saas_admin_user_access.updated_by":                    {},
	"mochat_go_saas_admin_user_roles.assigned_by":                    {},
	"mochat_go_saas_admin_approvals.reviewer_user_id":                {},
	"mochat_go_saas_admin_approvals.execution_user_id":               {},
	"mochat_go_saas_admin_approval_events.actor_user_id":             {},
	"mochat_go_saas_admin_approval_decisions.delegated_from_user_id": {},
	"mochat_go_saas_admin_approval_delegations.created_by":           {},
	"mochat_go_saas_admin_approval_delegations.updated_by":           {},
	"mochat_go_saas_admin_approval_policies.updated_by":              {},
	"mochat_go_saas_idempotency_receipts.created_by_saas_user_id":    {},
}

var nonActorTargetColumns = map[string]struct{}{
	"mochat_go_saas_admin_mfa_credentials.user_id":     {},
	"mochat_go_saas_admin_mfa_challenges.user_id":      {},
	"mochat_go_saas_admin_sessions.user_id":            {},
	"mochat_go_dashboard_identity_activations.user_id": {},
}

func SystemActorZeroAllowed(column ActorColumn) bool {
	_, ok := systemActorZeroAllowed[strings.TrimSpace(column.Table)+"."+strings.TrimSpace(column.Column)]
	return ok
}

func ValidateActorSchemaInventory(columns []ActorColumn) error {
	seen := make(map[string]struct{}, len(columns))
	unknown := make([]string, 0)
	for _, column := range columns {
		table := strings.TrimSpace(column.Table)
		name := strings.TrimSpace(column.Column)
		if table == "" || name == "" {
			return errors.New("actor inventory contains an empty table or column")
		}
		key := table + "." + name
		if _, exists := seen[key]; exists {
			return fmt.Errorf("actor inventory contains duplicate column %s", key)
		}
		seen[key] = struct{}{}
		expectedKind, known := actorColumnKind(key)
		if !known {
			unknown = append(unknown, key)
			continue
		}
		if column.Kind != "" && column.Kind != expectedKind {
			return fmt.Errorf("actor inventory classification mismatch for %s", key)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return fmt.Errorf("unknown actor column: %s", strings.Join(unknown, ","))
	}
	return nil
}

func actorColumnKind(key string) (ActorColumnKind, bool) {
	if _, ok := allowedActorColumns[key]; ok {
		return ActorColumnKindActor, true
	}
	if _, ok := nonActorTargetColumns[key]; ok {
		return ActorColumnKindNonActorTarget, true
	}
	return "", false
}

type PreflightReport struct {
	ActiveDashboardUsers          int
	DuplicateLoginUserIDs         []int64
	InvalidContactIDs             []int64
	DuplicatePlatformPhoneUserIDs []int64
	SaaSPhoneConflictUserIDs      []int64
	ZeroCorpTenantIDs             []int64
	MultiCorpTenantIDs            []int64
	DanglingTenantIDs             []int64
	DanglingCorpIDs               []int64
	DanglingUserIDs               []int64
	InactiveOrMissingTenantIDs    []int64
	InactiveBusinessSuperAdminIDs []int64
	CrossTenantRelationIDs        []int64
	UnmappedSaaSActorIDs          []int64
	NegativeTenantIDs             []int64
	UndecryptableCorpIDs          []int64
	UndecryptableAgentIDs         []int64
	UnknownActorColumns           []ActorColumn
	RepairClasses                 []string
}

func (r PreflightReport) Validate(options Options) error {
	if options.PlatformTenantID <= 0 {
		return errors.New("platform tenant id must be explicit and positive")
	}
	if len(r.DuplicateLoginUserIDs) > 0 {
		return fmt.Errorf("duplicate dashboard login identifier: user ids %v", r.DuplicateLoginUserIDs)
	}
	if len(r.InvalidContactIDs) > 0 {
		return fmt.Errorf("invalid dashboard login identifier: user ids %v", r.InvalidContactIDs)
	}
	if len(r.DuplicatePlatformPhoneUserIDs) > 0 || len(r.SaaSPhoneConflictUserIDs) > 0 {
		return errors.New("SaaS platform identity phone/login conflict")
	}
	if len(r.DanglingTenantIDs) > 0 || len(r.DanglingCorpIDs) > 0 || len(r.DanglingUserIDs) > 0 {
		return errors.New("dangling tenant, corp, or user reference")
	}
	if len(r.InactiveOrMissingTenantIDs) > 0 {
		return errors.New("business tenant is missing or inactive")
	}
	if len(r.InactiveBusinessSuperAdminIDs) > 0 {
		return errors.New("business superadmin is inactive")
	}
	if len(r.CrossTenantRelationIDs) > 0 {
		return errors.New("cross-tenant historical relation")
	}
	if len(r.UnmappedSaaSActorIDs) > 0 {
		return fmt.Errorf("unmapped SaaS actor: ids %v", r.UnmappedSaaSActorIDs)
	}
	if len(r.NegativeTenantIDs) > 0 {
		return fmt.Errorf("negative tenant id: ids %v", r.NegativeTenantIDs)
	}
	if len(r.UndecryptableCorpIDs) > 0 || len(r.UndecryptableAgentIDs) > 0 {
		return errors.New("unreadable credential ciphertext or plaintext")
	}
	if len(r.UnknownActorColumns) > 0 {
		return errors.New("unknown actor column in identity actor inventory")
	}
	if len(r.MultiCorpTenantIDs) > 0 {
		if !options.Mapping.SignatureVerified {
			return errors.New("multiple valid corp requires a verified mapping")
		}
		entries := make(map[int64]int64, len(options.Mapping.Entries))
		for _, entry := range options.Mapping.Entries {
			if entry.TenantID <= 0 || entry.CorpID <= 0 {
				return errors.New("mapping contains non-positive tenant or corp id")
			}
			if _, exists := entries[entry.TenantID]; exists {
				return fmt.Errorf("mapping contains duplicate tenant %d", entry.TenantID)
			}
			entries[entry.TenantID] = entry.CorpID
		}
		for _, tenantID := range r.MultiCorpTenantIDs {
			if _, exists := entries[tenantID]; !exists {
				return fmt.Errorf("multiple valid corp tenant %d has no mapping", tenantID)
			}
		}
	}
	return nil
}

func (r PreflightReport) SafeText() string {
	classes := append([]string(nil), r.RepairClasses...)
	sort.Strings(classes)
	unknownActors := make([]string, 0, len(r.UnknownActorColumns))
	for _, actor := range r.UnknownActorColumns {
		unknownActors = append(unknownActors, strings.TrimSpace(actor.Table)+"."+strings.TrimSpace(actor.Column))
	}
	sort.Strings(unknownActors)
	return fmt.Sprintf(
		"active_dashboard_users=%d duplicate_login_user_ids=%v invalid_contact_ids=%v duplicate_platform_user_ids=%v saas_identity_conflict_user_ids=%v zero_corp_tenant_ids=%v multi_corp_tenant_ids=%v dangling_tenant_ids=%v dangling_corp_ids=%v dangling_user_ids=%v inactive_or_missing_tenant_ids=%v inactive_business_superadmin_ids=%v cross_tenant_relation_ids=%v unmapped_saas_actor_ids=%v negative_tenant_ids=%v undecryptable_corp_ids=%v undecryptable_agent_ids=%v unknown_actor_columns=%v repair_classes=[%s]",
		r.ActiveDashboardUsers,
		r.DuplicateLoginUserIDs,
		r.InvalidContactIDs,
		r.DuplicatePlatformPhoneUserIDs,
		r.SaaSPhoneConflictUserIDs,
		r.ZeroCorpTenantIDs,
		r.MultiCorpTenantIDs,
		r.DanglingTenantIDs,
		r.DanglingCorpIDs,
		r.DanglingUserIDs,
		r.InactiveOrMissingTenantIDs,
		r.InactiveBusinessSuperAdminIDs,
		r.CrossTenantRelationIDs,
		r.UnmappedSaaSActorIDs,
		r.NegativeTenantIDs,
		r.UndecryptableCorpIDs,
		r.UndecryptableAgentIDs,
		unknownActors,
		strings.Join(classes, " "),
	)
}

type ConsistencyError struct {
	Report PreflightReport
	Err    error
}

func (e *ConsistencyError) Error() string {
	if e == nil || e.Err == nil {
		return "identity preflight consistency check failed"
	}
	return e.Err.Error()
}

func (e *ConsistencyError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (r PreflightReport) HasFindings() bool {
	return len(r.DuplicateLoginUserIDs) > 0 || len(r.InvalidContactIDs) > 0 || len(r.DuplicatePlatformPhoneUserIDs) > 0 || len(r.SaaSPhoneConflictUserIDs) > 0 || len(r.ZeroCorpTenantIDs) > 0 || len(r.MultiCorpTenantIDs) > 0 || len(r.DanglingTenantIDs) > 0 || len(r.DanglingCorpIDs) > 0 || len(r.DanglingUserIDs) > 0 || len(r.InactiveOrMissingTenantIDs) > 0 || len(r.InactiveBusinessSuperAdminIDs) > 0 || len(r.CrossTenantRelationIDs) > 0 || len(r.UnmappedSaaSActorIDs) > 0 || len(r.NegativeTenantIDs) > 0 || len(r.UndecryptableCorpIDs) > 0 || len(r.UndecryptableAgentIDs) > 0 || len(r.UnknownActorColumns) > 0 || len(r.RepairClasses) > 0
}

func NormalizePhone(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("contact identifier is empty")
	}
	if len(value) != 11 || value[0] != '1' || value[1] < '3' || value[1] > '9' {
		return "", errors.New("contact identifier format is invalid")
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return "", errors.New("contact identifier format is invalid")
		}
	}
	return value, nil
}

func ParseMappingDocument(data, signingKey []byte) (MappingDocument, error) {
	var document MappingDocument
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return MappingDocument{}, errors.New("mapping document is invalid")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return MappingDocument{}, errors.New("mapping document has trailing data")
	}
	if document.Version != 1 || len(document.Entries) == 0 || len(signingKey) == 0 {
		return MappingDocument{}, errors.New("mapping document signature is required")
	}
	entries := append([]CorpMapping(nil), document.Entries...)
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].TenantID == entries[j].TenantID {
			return entries[i].CorpID < entries[j].CorpID
		}
		return entries[i].TenantID < entries[j].TenantID
	})
	for index, entry := range entries {
		if entry.TenantID <= 0 || entry.CorpID <= 0 {
			return MappingDocument{}, errors.New("mapping document contains non-positive id")
		}
		if index > 0 && entries[index-1].TenantID == entry.TenantID {
			return MappingDocument{}, fmt.Errorf("mapping document contains duplicate tenant %d", entry.TenantID)
		}
	}
	canonical, err := json.Marshal(struct {
		Version int           `json:"version"`
		Entries []CorpMapping `json:"entries"`
	}{Version: document.Version, Entries: entries})
	if err != nil {
		return MappingDocument{}, errors.New("mapping document cannot be canonicalized")
	}
	provided, err := hex.DecodeString(strings.TrimSpace(document.Signature))
	if err != nil || len(provided) != sha256.Size {
		return MappingDocument{}, errors.New("mapping document signature is invalid")
	}
	mac := hmac.New(sha256.New, signingKey)
	_, _ = mac.Write(canonical)
	if !hmac.Equal(provided, mac.Sum(nil)) {
		return MappingDocument{}, errors.New("mapping document signature verification failed")
	}
	document.Entries = entries
	document.SignatureVerified = true
	return document, nil
}

func ValidateMappingOwnership(document MappingDocument, ownership map[int64][]int64) error {
	if !document.SignatureVerified {
		return errors.New("mapping signature is not verified")
	}
	seenTenants := make(map[int64]struct{}, len(document.Entries))
	for _, entry := range document.Entries {
		if entry.TenantID <= 0 || entry.CorpID <= 0 {
			return errors.New("mapping contains non-positive tenant or corp id")
		}
		if _, exists := seenTenants[entry.TenantID]; exists {
			return fmt.Errorf("mapping contains duplicate tenant %d", entry.TenantID)
		}
		seenTenants[entry.TenantID] = struct{}{}
		corps := ownership[entry.TenantID]
		if len(corps) <= 1 {
			return fmt.Errorf("tenant %d is a one-corp tenant and must not be mapped", entry.TenantID)
		}
		found := false
		for _, corpID := range corps {
			if corpID == entry.CorpID {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("tenant %d mapped corp is not owned by the tenant", entry.TenantID)
		}
	}
	return nil
}

func VerifyMaintenanceConfirmation(data []byte, schema, requestID string) error {
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 || strings.TrimSpace(lines[0]) != "MOCHAT_IDENTITY_MAINTENANCE_V1" {
		return errors.New("maintenance confirmation format is invalid")
	}
	values := make(map[string]string, 2)
	for _, line := range lines[1:] {
		parts := strings.SplitN(strings.TrimSpace(line), "=", 2)
		if len(parts) != 2 || (parts[0] != "schema" && parts[0] != "request_id") || strings.TrimSpace(parts[1]) == "" {
			return errors.New("maintenance confirmation format is invalid")
		}
		if _, exists := values[parts[0]]; exists {
			return errors.New("maintenance confirmation contains duplicate fields")
		}
		values[parts[0]] = strings.TrimSpace(parts[1])
	}
	if values["schema"] != strings.TrimSpace(schema) || values["request_id"] != strings.TrimSpace(requestID) {
		return errors.New("maintenance confirmation does not bind schema and request")
	}
	return nil
}
