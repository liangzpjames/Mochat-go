// Package dashboardadmin owns the SaaS-controlled Dashboard administrator
// lifecycle. It deliberately does not import the Dashboard HTTP package: the
// HTTP layer supplies an authenticated SaaS actor, while the Store adapter is
// the only layer allowed to open and commit SQL transactions.
package dashboardadmin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
)

const PermissionTenantsManage = "platform.tenants.manage"

var (
	ErrInvalidRequest                = errors.New("dashboard admin request is invalid")
	ErrPermissionDenied              = errors.New("dashboard admin permission denied")
	ErrStoreUnavailable              = errors.New("dashboard admin store unavailable")
	ErrLoginIdentifierConflict       = errors.New("dashboard login identifier conflict")
	ErrIdempotencyConflict           = errors.New("dashboard tenant provisioning idempotency conflict")
	ErrTargetNotFound                = errors.New("dashboard administration target not found")
	ErrVersionConflict               = errors.New("dashboard administration version conflict")
	ErrReplacementRequiresActivation = errors.New("new dashboard administrator must be activated before replacement")
	ErrActivationAlreadyComplete     = errors.New("dashboard administrator is already activated")
	ErrNotSuperAdmin                 = errors.New("dashboard target is not an SaaS-governed super administrator")
	ErrLastSuperAdmin                = errors.New("cannot disable the last active dashboard super administrator")
	ErrSuperAdminGovernanceOnly      = errors.New("dashboard super administrator is governed by SaaS only")
)

// Actor is built from the authenticated SaaS principal. It is never decoded
// from a provisioning or governance request body.
type Actor struct {
	UserID      int
	Active      bool
	Permissions []string
}

func (actor Actor) HasPermission(permission string) bool {
	for _, candidate := range actor.Permissions {
		candidate = strings.TrimSpace(candidate)
		if candidate == permission || candidate == "*" {
			return true
		}
	}
	return false
}

// SaaSAdminPackageLimits is the immutable 26-field quota snapshot written into
// the tenant package row at provisioning time.
type SaaSAdminPackageLimits struct {
	MaxCorps              int64 `json:"maxCorps"`
	MaxUsers              int64 `json:"maxUsers"`
	MaxContacts           int64 `json:"maxContacts"`
	MaxRooms              int64 `json:"maxRooms"`
	MaxAgents             int64 `json:"maxAgents"`
	ChannelCodes          int64 `json:"channelCodes"`
	ShopCodes             int64 `json:"shopCodes"`
	Radars                int64 `json:"radars"`
	Lotteries             int64 `json:"lotteries"`
	RoomInfinitePulls     int64 `json:"roomInfinitePulls"`
	RoomFissions          int64 `json:"roomFissions"`
	RoomClockIns          int64 `json:"roomClockIns"`
	RoomQualities         int64 `json:"roomQualities"`
	RoomCalendars         int64 `json:"roomCalendars"`
	RoomReminds           int64 `json:"roomReminds"`
	ContactSOPs           int64 `json:"contactSops"`
	RoomSOPs              int64 `json:"roomSops"`
	SensitiveWords        int64 `json:"sensitiveWords"`
	StorageMB             int64 `json:"storageMb"`
	ContactMessageBatches int64 `json:"contactMessageBatches"`
	RoomMessageBatches    int64 `json:"roomMessageBatches"`
	RoomTagPulls          int64 `json:"roomTagPulls"`
	WorkRoomAutoPulls     int64 `json:"workRoomAutoPulls"`
	WorkFissions          int64 `json:"workFissions"`
	OfficialAccounts      int64 `json:"officialAccounts"`
	AsyncExecutions       int64 `json:"asyncExecutions"`
	present               map[string]struct{}
}

var packageLimitJSONKeys = [...]string{
	"maxCorps", "maxUsers", "maxContacts", "maxRooms", "maxAgents", "channelCodes", "shopCodes", "radars", "lotteries", "roomInfinitePulls", "roomFissions", "roomClockIns", "roomQualities", "roomCalendars", "roomReminds", "contactSops", "roomSops", "sensitiveWords", "storageMb", "contactMessageBatches", "roomMessageBatches", "roomTagPulls", "workRoomAutoPulls", "workFissions", "officialAccounts", "asyncExecutions",
}

// UnmarshalJSON records field presence so an HTTP request cannot turn a
// missing quota into a legitimate zero quota. The outer request decoder still
// rejects unknown fields; this nested decoder keeps that guarantee for the
// quota object as well.
func (limits *SaaSAdminPackageLimits) UnmarshalJSON(data []byte) error {
	type packageLimitsAlias SaaSAdminPackageLimits
	var decoded packageLimitsAlias
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("quota snapshot has trailing JSON")
		}
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	present := make(map[string]struct{}, len(fields))
	for key := range fields {
		present[key] = struct{}{}
	}
	for _, key := range packageLimitJSONKeys {
		if _, ok := present[key]; !ok {
			return errors.New("quota snapshot is incomplete")
		}
	}
	*limits = SaaSAdminPackageLimits(decoded)
	limits.present = present
	return nil
}

func (limits SaaSAdminPackageLimits) hasAllJSONFields() bool {
	if limits.present == nil {
		// Values assembled from the locked package row or typed internal tests do
		// not pass through JSON and are already structurally complete.
		return true
	}
	for _, key := range packageLimitJSONKeys {
		if _, ok := limits.present[key]; !ok {
			return false
		}
	}
	return true
}

type SubscriptionInput struct {
	PackageCode  string `json:"packageCode"`
	Status       string `json:"status"`
	BillingCycle string `json:"billingCycle"`
	StartsAt     string `json:"startsAt"`
	ExpiresAt    string `json:"expiresAt"`
}

type ProvisionDashboardTenant struct {
	TenantName           string                 `json:"tenantName"`
	PackageID            int                    `json:"packageId"`
	Limits               SaaSAdminPackageLimits `json:"limits"`
	Subscription         SubscriptionInput      `json:"subscription"`
	AdminLoginIdentifier string                 `json:"adminLoginIdentifier"`
	AdminName            string                 `json:"adminName"`
	IdempotencyKey       string                 `json:"idempotencyKey"`
	ExpectedVersion      uint64                 `json:"expectedVersion"`
	RequestID            string                 `json:"-"`

	// These fields are populated only by a decoder contract test to ensure
	// caller-supplied scope cannot be smuggled into the service. They are not
	// JSON fields and must remain zero in production requests.
	BodyTenantID int `json:"-"`
	BodyActorID  int `json:"-"`
}

type ProvisionResult struct {
	TenantID        int
	DashboardUserID int
	BindingCorpID   int
	ActivationToken string
	Idempotent      bool
}

type ResendActivationInput struct {
	TenantID        int    `json:"-"`
	TargetUserID    int    `json:"targetUserId"`
	ExpectedVersion uint64 `json:"expectedVersion"`
	RequestID       string `json:"-"`
	BodyTenantID    int    `json:"-"`
	BodyActorID     int    `json:"-"`
}

type ResendActivationResult struct {
	TenantID        int
	DashboardUserID int
	Version         uint64
	ActivationToken string
	Idempotent      bool
}

type ReplaceSuperAdminInput struct {
	TenantID        int    `json:"-"`
	CurrentAdminID  int    `json:"currentAdminId"`
	NewAdminID      int    `json:"newAdminId"`
	ExpectedVersion uint64 `json:"expectedVersion"`
	RequestID       string `json:"-"`
	BodyTenantID    int    `json:"-"`
	BodyActorID     int    `json:"-"`
}

type SuperAdminStatusInput struct {
	TenantID        int    `json:"-"`
	TargetUserID    int    `json:"targetUserId"`
	Enabled         bool   `json:"enabled"`
	ExpectedVersion uint64 `json:"expectedVersion"`
	RequestID       string `json:"-"`
	BodyTenantID    int    `json:"-"`
	BodyActorID     int    `json:"-"`
}

type GovernanceResult struct {
	TenantID        int
	DashboardUserID int
	Version         uint64
	Idempotent      bool
}

type DashboardIdentityRecord struct {
	ID              int
	Name            string
	LoginIdentifier string
	UserStatus      int
	IdentityStatus  int
	ActivatedAt     string
	IsSuperAdmin    bool
}

type DashboardAdminGovernanceView struct {
	TenantID       int
	BindingVersion uint64
	Identities     []DashboardIdentityRecord
}

type DashboardUserMutation struct {
	UserID          int    `json:"userId"`
	Name            string `json:"name"`
	LoginIdentifier string `json:"loginIdentifier"`
	IsSuperAdmin    bool   `json:"isSuperAdmin"`
}

// Store is implemented by the MySQL adapter. Each method is one complete
// database transaction; callers cannot observe partially-created tenant
// artifacts.
type Store interface {
	ProvisionDashboardTenant(context.Context, Actor, ProvisionDashboardTenant) (ProvisionResult, error)
	DashboardAdminGovernance(context.Context, Actor, int) (DashboardAdminGovernanceView, error)
	ResendDashboardActivation(context.Context, Actor, ResendActivationInput) (ResendActivationResult, error)
	ReplaceDashboardSuperAdmin(context.Context, Actor, ReplaceSuperAdminInput) (GovernanceResult, error)
	SetDashboardSuperAdminStatus(context.Context, Actor, SuperAdminStatusInput) (GovernanceResult, error)
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (service *Service) ProvisionDashboardTenant(ctx context.Context, actor Actor, input ProvisionDashboardTenant) (ProvisionResult, error) {
	if err := validateActor(actor); err != nil {
		return ProvisionResult{}, err
	}
	if err := validateProvision(input); err != nil {
		return ProvisionResult{}, err
	}
	if service == nil || service.store == nil {
		return ProvisionResult{}, ErrStoreUnavailable
	}
	result, err := service.store.ProvisionDashboardTenant(ctx, actor, input)
	if err != nil {
		return ProvisionResult{}, err
	}
	if result.Idempotent {
		// A durable idempotency receipt contains only the business result and
		// token digest. The original opaque token is never replayable.
		result.ActivationToken = ""
	}
	return result, nil
}

func (service *Service) DashboardAdminGovernance(ctx context.Context, actor Actor, tenantID int) (DashboardAdminGovernanceView, error) {
	if err := validateActor(actor); err != nil {
		return DashboardAdminGovernanceView{}, err
	}
	if tenantID <= 0 {
		return DashboardAdminGovernanceView{}, ErrInvalidRequest
	}
	if service == nil || service.store == nil {
		return DashboardAdminGovernanceView{}, ErrStoreUnavailable
	}
	return service.store.DashboardAdminGovernance(ctx, actor, tenantID)
}

func (service *Service) ResendActivation(ctx context.Context, actor Actor, input ResendActivationInput) (ResendActivationResult, error) {
	if err := validateActor(actor); err != nil {
		return ResendActivationResult{}, err
	}
	if input.BodyTenantID != 0 || input.BodyActorID != 0 || input.TenantID <= 0 || input.TargetUserID <= 0 || input.ExpectedVersion == 0 {
		return ResendActivationResult{}, ErrInvalidRequest
	}
	if requestID := strings.TrimSpace(input.RequestID); requestID == "" || len([]rune(requestID)) > 80 {
		return ResendActivationResult{}, ErrInvalidRequest
	}
	if service == nil || service.store == nil {
		return ResendActivationResult{}, ErrStoreUnavailable
	}
	return service.store.ResendDashboardActivation(ctx, actor, input)
}

func (service *Service) ReplaceDashboardSuperAdmin(ctx context.Context, actor Actor, input ReplaceSuperAdminInput) (GovernanceResult, error) {
	if err := validateActor(actor); err != nil {
		return GovernanceResult{}, err
	}
	if input.BodyTenantID != 0 || input.BodyActorID != 0 || input.TenantID <= 0 || input.CurrentAdminID <= 0 || input.NewAdminID <= 0 || input.CurrentAdminID == input.NewAdminID || input.ExpectedVersion == 0 {
		return GovernanceResult{}, ErrInvalidRequest
	}
	if !validGovernanceRequestKey(input.RequestID) {
		return GovernanceResult{}, ErrInvalidRequest
	}
	if service == nil || service.store == nil {
		return GovernanceResult{}, ErrStoreUnavailable
	}
	return service.store.ReplaceDashboardSuperAdmin(ctx, actor, input)
}

func (service *Service) SetDashboardSuperAdminStatus(ctx context.Context, actor Actor, input SuperAdminStatusInput) (GovernanceResult, error) {
	if err := validateActor(actor); err != nil {
		return GovernanceResult{}, err
	}
	if input.BodyTenantID != 0 || input.BodyActorID != 0 || input.TenantID <= 0 || input.TargetUserID <= 0 || input.ExpectedVersion == 0 {
		return GovernanceResult{}, ErrInvalidRequest
	}
	if !validGovernanceRequestKey(input.RequestID) {
		return GovernanceResult{}, ErrInvalidRequest
	}
	if service == nil || service.store == nil {
		return GovernanceResult{}, ErrStoreUnavailable
	}
	return service.store.SetDashboardSuperAdminStatus(ctx, actor, input)
}

func validGovernanceRequestKey(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && len([]rune(value)) <= 80
}

func ValidateDashboardUserMutation(input DashboardUserMutation) error {
	if input.IsSuperAdmin {
		return ErrSuperAdminGovernanceOnly
	}
	return nil
}

func validateActor(actor Actor) error {
	if actor.UserID <= 0 || !actor.Active {
		return ErrPermissionDenied
	}
	// An HTTP principal carries identity facts only. The SQL Store must resolve
	// and recheck the current SaaS role/permission inside the same transaction.
	// A non-empty permission projection is an optional earlier rejection, never
	// an authorization grant.
	if len(actor.Permissions) > 0 && !actor.HasPermission(PermissionTenantsManage) {
		return ErrPermissionDenied
	}
	return nil
}

func validateProvision(input ProvisionDashboardTenant) error {
	if input.BodyTenantID != 0 || input.BodyActorID != 0 {
		return ErrInvalidRequest
	}
	if strings.TrimSpace(input.TenantName) == "" || len([]rune(strings.TrimSpace(input.TenantName))) > 255 {
		return ErrInvalidRequest
	}
	if strings.TrimSpace(input.AdminName) == "" || len([]rune(strings.TrimSpace(input.AdminName))) > 255 {
		return ErrInvalidRequest
	}
	if input.PackageID <= 0 || !validPhone(input.AdminLoginIdentifier) {
		return ErrInvalidRequest
	}
	if strings.TrimSpace(input.IdempotencyKey) == "" || len([]rune(strings.TrimSpace(input.IdempotencyKey))) > 128 {
		return ErrInvalidRequest
	}
	if input.ExpectedVersion == 0 || !validPackageLimits(input.Limits) {
		return ErrInvalidRequest
	}
	if input.Subscription.PackageCode != "" && len([]rune(strings.TrimSpace(input.Subscription.PackageCode))) > 64 {
		return ErrInvalidRequest
	}
	if input.Subscription.Status != "trialing" && input.Subscription.Status != "active" {
		return ErrInvalidRequest
	}
	if input.Subscription.BillingCycle != "monthly" && input.Subscription.BillingCycle != "yearly" && input.Subscription.BillingCycle != "custom" && input.Subscription.BillingCycle != "lifetime" {
		return ErrInvalidRequest
	}
	startsAt, startsOK := parseProvisionTime(input.Subscription.StartsAt)
	expiresAt, expiresOK := parseProvisionTime(input.Subscription.ExpiresAt)
	if !startsOK || !expiresOK {
		return ErrInvalidRequest
	}
	if startsOK && expiresOK && !expiresAt.After(startsAt) {
		return ErrInvalidRequest
	}
	return nil
}

func validPhone(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 11 {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func parseProvisionTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func validPackageLimits(limits SaaSAdminPackageLimits) bool {
	return limits.hasAllJSONFields() && limits.MaxCorps >= 0 && limits.MaxUsers >= 0 && limits.MaxContacts >= 0 &&
		limits.MaxRooms >= 0 && limits.MaxAgents >= 0 && limits.ChannelCodes >= 0 &&
		limits.ShopCodes >= 0 && limits.Radars >= 0 && limits.Lotteries >= 0 &&
		limits.RoomInfinitePulls >= 0 && limits.RoomFissions >= 0 && limits.RoomClockIns >= 0 &&
		limits.RoomQualities >= 0 && limits.RoomCalendars >= 0 && limits.RoomReminds >= 0 &&
		limits.ContactSOPs >= 0 && limits.RoomSOPs >= 0 && limits.SensitiveWords >= 0 &&
		limits.StorageMB >= 0 && limits.ContactMessageBatches >= 0 && limits.RoomMessageBatches >= 0 &&
		limits.RoomTagPulls >= 0 && limits.WorkRoomAutoPulls >= 0 && limits.WorkFissions >= 0 &&
		limits.OfficialAccounts >= 0 && limits.AsyncExecutions >= 0
}
