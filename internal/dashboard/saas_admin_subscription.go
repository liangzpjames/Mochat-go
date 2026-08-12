package dashboard

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	SaaSAdminSubscriptionStatusAll       = "all"
	SaaSAdminSubscriptionStatusTrialing  = "trialing"
	SaaSAdminSubscriptionStatusActive    = "active"
	SaaSAdminSubscriptionStatusGrace     = "grace"
	SaaSAdminSubscriptionStatusPastDue   = "past_due"
	SaaSAdminSubscriptionStatusSuspended = "suspended"
	SaaSAdminSubscriptionStatusCanceled  = "canceled"

	SaaSAdminSubscriptionAccessAll     = "all"
	SaaSAdminSubscriptionAccessAllowed = "allowed"
	SaaSAdminSubscriptionAccessBlocked = "blocked"

	SaaSAdminSubscriptionDefaultGraceDays = 7
	SaaSAdminExportKindSubscriptions      = "subscriptions"

	SaaSAdminOperationActionSubscriptionTransition = "tenant.subscription.transition"
	SaaSAdminOperationActionSubscriptionReconcile  = "tenant.subscription.reconcile"
	SaaSAdminOperationTargetSubscription           = "subscription"
)

type SaaSAdminSubscriptionOptions struct {
	TenantID    int
	Status      string
	Access      string
	PackageCode string
	Keyword     string
	Limit       int
}

type SaaSAdminSubscription struct {
	ID                    int64
	TenantID              int
	TenantName            string
	TenantStatus          int
	PackageCode           string
	PackageName           string
	Status                string
	EffectiveStatus       string
	BillingCycle          string
	TrialStartsAt         string
	TrialEndsAt           string
	CurrentPeriodStartsAt string
	CurrentPeriodEndsAt   string
	GraceEndsAt           string
	CancelAtPeriodEnd     bool
	CanceledAt            string
	SuspendedAt           string
	LatestBillingEventID  int64
	Version               int
	StateReason           string
	MetadataJSON          string
	CreatedAt             string
	UpdatedAt             string
	AccessAllowed         bool
	AccessReason          string
	NeedsReconciliation   bool
	NextActionAt          string
}

type SaaSAdminSubscriptionSummary struct {
	SubscriptionCount      int
	TrialingCount          int
	ActiveCount            int
	GraceCount             int
	PastDueCount           int
	SuspendedCount         int
	CanceledCount          int
	AccessAllowedCount     int
	AccessBlockedCount     int
	ReconciliationDueCount int
	PackageCount           int
	TenantCount            int
}

type SaaSAdminSubscriptionReport struct {
	Options       SaaSAdminSubscriptionOptions
	Summary       SaaSAdminSubscriptionSummary
	Subscriptions []SaaSAdminSubscription
}

type SaaSAdminSubscriptionEventOptions struct {
	TenantID int
	Status   string
	Source   string
	Keyword  string
	Limit    int
}

type SaaSAdminSubscriptionEvent struct {
	ID             int64
	SubscriptionID int64
	TenantID       int
	TenantName     string
	EventType      string
	FromStatus     string
	ToStatus       string
	EffectiveAt    string
	ActorUserID    int
	ActorTenantID  int
	Source         string
	IdempotencyKey string
	Reason         string
	PayloadJSON    string
	CreatedAt      string
}

type SaaSAdminSubscriptionTransition struct {
	TenantID                 int    `json:"tenantId"`
	Status                   string `json:"status"`
	PackageCode              string `json:"packageCode,omitempty"`
	BillingCycle             string `json:"billingCycle,omitempty"`
	TrialStartsAt            string `json:"trialStartsAt,omitempty"`
	TrialEndsAt              string `json:"trialEndsAt,omitempty"`
	CurrentPeriodStartsAt    string `json:"currentPeriodStartsAt,omitempty"`
	CurrentPeriodEndsAt      string `json:"currentPeriodEndsAt,omitempty"`
	GraceEndsAt              string `json:"graceEndsAt,omitempty"`
	CancelAtPeriodEnd        *bool  `json:"cancelAtPeriodEnd,omitempty"`
	ExpectedVersion          int    `json:"expectedVersion"`
	IdempotencyKey           string `json:"idempotencyKey"`
	Reason                   string `json:"reason"`
	Source                   string `json:"source"`
	ExpectedTenantStatus     int    `json:"-"`
	ExpectedSubscriptionID   int64  `json:"-"`
	ActorUserID              int    `json:"-"`
	ActorTenantID            int    `json:"-"`
	ApprovalExecutionID      int64  `json:"-"`
	ApprovalExecutionVersion int    `json:"-"`
}

type SaaSAdminSubscriptionTransitionSnapshot struct {
	ID                    int64  `json:"id"`
	TenantID              int    `json:"tenantId"`
	TenantName            string `json:"tenantName"`
	TenantStatus          int    `json:"tenantStatus"`
	PackageCode           string `json:"packageCode"`
	PackageName           string `json:"packageName"`
	Status                string `json:"status"`
	BillingCycle          string `json:"billingCycle"`
	TrialStartsAt         string `json:"trialStartsAt"`
	TrialEndsAt           string `json:"trialEndsAt"`
	CurrentPeriodStartsAt string `json:"currentPeriodStartsAt"`
	CurrentPeriodEndsAt   string `json:"currentPeriodEndsAt"`
	GraceEndsAt           string `json:"graceEndsAt"`
	CancelAtPeriodEnd     bool   `json:"cancelAtPeriodEnd"`
	CanceledAt            string `json:"canceledAt"`
	SuspendedAt           string `json:"suspendedAt"`
	LatestBillingEventID  int64  `json:"latestBillingEventId"`
	Version               int    `json:"version"`
	StateReason           string `json:"stateReason"`
}

type SaaSAdminSubscriptionTransitionApprovalPlan struct {
	Transition   SaaSAdminSubscriptionTransition         `json:"transition"`
	Subscription SaaSAdminSubscriptionTransitionSnapshot `json:"subscription"`
}

type SaaSAdminSubscriptionTransitionResult struct {
	Subscription   SaaSAdminSubscription
	PreviousStatus string
	Changed        bool
	Idempotent     bool
	EventID        int64
	OperationID    int64
}

type SaaSAdminSubscriptionReconcile struct {
	TenantID         int
	Limit            int
	DryRun           bool
	ExcludedTenantID int
	ActorUserID      int
	ActorTenantID    int
}

type SaaSAdminSubscriptionReconcileResult struct {
	ScannedCount      int
	ReconciliationDue int
	ChangedCount      int
	SkippedCount      int
	FailedCount       int
	DryRun            bool
	Transitions       []SaaSAdminSubscriptionTransitionResult
	Errors            []SaaSAdminSubscriptionReconcileError
}

type SaaSAdminSubscriptionReconcileError struct {
	TenantID int
	Error    string
}

func SaaSAdminSubscriptionStatusValid(status string) bool {
	switch strings.TrimSpace(status) {
	case SaaSAdminSubscriptionStatusTrialing,
		SaaSAdminSubscriptionStatusActive,
		SaaSAdminSubscriptionStatusGrace,
		SaaSAdminSubscriptionStatusPastDue,
		SaaSAdminSubscriptionStatusSuspended,
		SaaSAdminSubscriptionStatusCanceled:
		return true
	default:
		return false
	}
}

func SaaSAdminSubscriptionAllowsAccess(status string) bool {
	switch strings.TrimSpace(status) {
	case SaaSAdminSubscriptionStatusTrialing, SaaSAdminSubscriptionStatusActive, SaaSAdminSubscriptionStatusGrace:
		return true
	default:
		return false
	}
}

func SaaSAdminSubscriptionAccessReason(status string) string {
	switch strings.TrimSpace(status) {
	case SaaSAdminSubscriptionStatusPastDue:
		return "租户订阅已欠费"
	case SaaSAdminSubscriptionStatusSuspended:
		return "租户订阅已暂停"
	case SaaSAdminSubscriptionStatusCanceled:
		return "租户订阅已取消"
	case SaaSAdminSubscriptionStatusTrialing:
		return "租户试用有效"
	case SaaSAdminSubscriptionStatusGrace:
		return "租户处于续费宽限期"
	case SaaSAdminSubscriptionStatusActive:
		return "租户订阅有效"
	default:
		return "租户订阅状态无效"
	}
}

func SaaSAdminEffectiveSubscriptionStatus(item SaaSAdminSubscription, now time.Time) string {
	status := strings.TrimSpace(item.Status)
	if item.TenantStatus == 2 {
		return SaaSAdminSubscriptionStatusSuspended
	}
	periodEnd, hasPeriodEnd := parseSaaSAdminNormalizedDateTime(item.CurrentPeriodEndsAt)
	trialEnd, hasTrialEnd := parseSaaSAdminNormalizedDateTime(item.TrialEndsAt)
	graceEnd, hasGraceEnd := parseSaaSAdminNormalizedDateTime(item.GraceEndsAt)
	if item.CancelAtPeriodEnd && hasPeriodEnd && !periodEnd.After(now) {
		return SaaSAdminSubscriptionStatusCanceled
	}
	switch status {
	case SaaSAdminSubscriptionStatusTrialing:
		if hasTrialEnd && !trialEnd.After(now) {
			if hasGraceEnd && graceEnd.After(now) {
				return SaaSAdminSubscriptionStatusGrace
			}
			return SaaSAdminSubscriptionStatusPastDue
		}
	case SaaSAdminSubscriptionStatusActive:
		if hasPeriodEnd && !periodEnd.After(now) {
			if hasGraceEnd && graceEnd.After(now) {
				return SaaSAdminSubscriptionStatusGrace
			}
			return SaaSAdminSubscriptionStatusPastDue
		}
	case SaaSAdminSubscriptionStatusGrace:
		if !hasGraceEnd || !graceEnd.After(now) {
			return SaaSAdminSubscriptionStatusPastDue
		}
	case SaaSAdminSubscriptionStatusPastDue,
		SaaSAdminSubscriptionStatusSuspended,
		SaaSAdminSubscriptionStatusCanceled:
		return status
	default:
		return SaaSAdminSubscriptionStatusPastDue
	}
	return status
}

func SaaSAdminSubscriptionTransitionAllowed(from, to string) bool {
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)
	if from == to {
		return true
	}
	allowed := map[string]map[string]bool{
		SaaSAdminSubscriptionStatusTrialing: {
			SaaSAdminSubscriptionStatusActive: true, SaaSAdminSubscriptionStatusGrace: true,
			SaaSAdminSubscriptionStatusPastDue: true, SaaSAdminSubscriptionStatusSuspended: true,
			SaaSAdminSubscriptionStatusCanceled: true,
		},
		SaaSAdminSubscriptionStatusActive: {
			SaaSAdminSubscriptionStatusTrialing: true,
			SaaSAdminSubscriptionStatusGrace:    true, SaaSAdminSubscriptionStatusPastDue: true,
			SaaSAdminSubscriptionStatusSuspended: true, SaaSAdminSubscriptionStatusCanceled: true,
		},
		SaaSAdminSubscriptionStatusGrace: {
			SaaSAdminSubscriptionStatusActive: true, SaaSAdminSubscriptionStatusPastDue: true,
			SaaSAdminSubscriptionStatusSuspended: true, SaaSAdminSubscriptionStatusCanceled: true,
		},
		SaaSAdminSubscriptionStatusPastDue: {
			SaaSAdminSubscriptionStatusActive: true, SaaSAdminSubscriptionStatusGrace: true,
			SaaSAdminSubscriptionStatusSuspended: true, SaaSAdminSubscriptionStatusCanceled: true,
		},
		SaaSAdminSubscriptionStatusSuspended: {
			SaaSAdminSubscriptionStatusTrialing: true, SaaSAdminSubscriptionStatusActive: true,
			SaaSAdminSubscriptionStatusGrace: true, SaaSAdminSubscriptionStatusPastDue: true,
			SaaSAdminSubscriptionStatusCanceled: true,
		},
		SaaSAdminSubscriptionStatusCanceled: {
			SaaSAdminSubscriptionStatusTrialing: true, SaaSAdminSubscriptionStatusActive: true,
			SaaSAdminSubscriptionStatusSuspended: true,
		},
	}
	return allowed[from][to]
}

func ApplySaaSAdminSubscriptionTransitionState(item *SaaSAdminSubscription, transition SaaSAdminSubscriptionTransition, now time.Time) error {
	if item == nil {
		return NewSaaSAdminBadRequest("subscription missing")
	}
	if transition.BillingCycle != "" {
		item.BillingCycle = transition.BillingCycle
	}
	if transition.TrialStartsAt != "" {
		item.TrialStartsAt = transition.TrialStartsAt
	}
	if transition.TrialEndsAt != "" {
		item.TrialEndsAt = transition.TrialEndsAt
	}
	if transition.CurrentPeriodStartsAt != "" {
		item.CurrentPeriodStartsAt = transition.CurrentPeriodStartsAt
	}
	if transition.CurrentPeriodEndsAt != "" {
		item.CurrentPeriodEndsAt = transition.CurrentPeriodEndsAt
	}
	if transition.GraceEndsAt != "" {
		item.GraceEndsAt = transition.GraceEndsAt
	}
	if transition.CancelAtPeriodEnd != nil {
		item.CancelAtPeriodEnd = *transition.CancelAtPeriodEnd
	}
	if item.BillingCycle == "lifetime" {
		item.CurrentPeriodEndsAt = ""
		item.GraceEndsAt = ""
		item.CancelAtPeriodEnd = false
	}
	item.Status = transition.Status
	item.StateReason = strings.TrimSpace(transition.Reason)
	if item.StateReason == "" {
		item.StateReason = "订阅状态调整"
	}
	switch item.Status {
	case SaaSAdminSubscriptionStatusTrialing:
		trialEnd, ok := parseSaaSAdminNormalizedDateTime(item.TrialEndsAt)
		if !ok || !trialEnd.After(now) {
			return NewSaaSAdminBadRequest("trialing requires future trialEndsAt")
		}
		item.CanceledAt = ""
		item.SuspendedAt = ""
	case SaaSAdminSubscriptionStatusActive:
		if item.BillingCycle != "lifetime" && item.CurrentPeriodEndsAt != "" {
			periodEnd, ok := parseSaaSAdminNormalizedDateTime(item.CurrentPeriodEndsAt)
			if !ok || !periodEnd.After(now) {
				return NewSaaSAdminBadRequest("active requires future currentPeriodEndsAt or lifetime billingCycle")
			}
		}
		item.CanceledAt = ""
		item.SuspendedAt = ""
		if transition.CancelAtPeriodEnd == nil {
			item.CancelAtPeriodEnd = false
		}
	case SaaSAdminSubscriptionStatusGrace:
		graceEnd, ok := parseSaaSAdminNormalizedDateTime(item.GraceEndsAt)
		if !ok || !graceEnd.After(now) {
			return NewSaaSAdminBadRequest("grace requires future graceEndsAt")
		}
		item.CanceledAt = ""
		item.SuspendedAt = ""
	case SaaSAdminSubscriptionStatusPastDue:
		item.CanceledAt = ""
		item.SuspendedAt = ""
	case SaaSAdminSubscriptionStatusSuspended:
		item.SuspendedAt = now.Format("2006-01-02 15:04:05")
		item.CanceledAt = ""
	case SaaSAdminSubscriptionStatusCanceled:
		item.CanceledAt = now.Format("2006-01-02 15:04:05")
		item.SuspendedAt = ""
		item.CancelAtPeriodEnd = false
	}
	return nil
}

func SaaSAdminSubscriptionStateEqual(left, right SaaSAdminSubscription) bool {
	return left.PackageCode == right.PackageCode && left.PackageName == right.PackageName &&
		left.Status == right.Status && left.BillingCycle == right.BillingCycle &&
		left.TrialStartsAt == right.TrialStartsAt && left.TrialEndsAt == right.TrialEndsAt &&
		left.CurrentPeriodStartsAt == right.CurrentPeriodStartsAt && left.CurrentPeriodEndsAt == right.CurrentPeriodEndsAt &&
		left.GraceEndsAt == right.GraceEndsAt && left.CancelAtPeriodEnd == right.CancelAtPeriodEnd &&
		left.CanceledAt == right.CanceledAt && left.SuspendedAt == right.SuspendedAt &&
		left.StateReason == right.StateReason
}

func (h *SaaSAdminHandler) Subscriptions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	options, err := parseSaaSAdminSubscriptionOptions(r, saasAdminListMaxLimit)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	report, err := h.store.SaaSAdminSubscriptions(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminSubscriptionReportPayload(report, h.platformAdminTenantID))
}

func (h *SaaSAdminHandler) SubscriptionEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	options, err := parseSaaSAdminSubscriptionEventOptions(r, saasAdminListMaxLimit)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	events, err := h.store.SaaSAdminSubscriptionEvents(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"platformAdminTenantId": h.platformAdminTenantID,
		"generatedAt":           time.Now().Format("2006-01-02 15:04:05"),
		"filters": map[string]any{
			"tenantId": options.TenantID, "status": options.Status, "source": options.Source,
			"keyword": options.Keyword, "limit": options.Limit,
		},
		"count":  len(events),
		"events": saasAdminSubscriptionEventPayloads(events),
	})
}

func (h *SaaSAdminHandler) TransitionSubscription(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	transition, err := parseSaaSAdminSubscriptionTransition(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if h.rejectDirectHighRiskAction(r.Context(), w, SaaSAdminApprovalActionSubscriptionTransition, 0) {
		return
	}
	transition.Source = "admin"
	transition.ActorUserID = user.ID
	transition.ActorTenantID = user.TenantID
	result, err := h.store.TransitionSaaSAdminSubscription(r.Context(), transition)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminSubscriptionTransitionResultPayload(result))
}

func (h *SaaSAdminHandler) planSaaSAdminSubscriptionTransition(ctx context.Context, transition SaaSAdminSubscriptionTransition) (SaaSAdminSubscriptionTransitionApprovalPlan, error) {
	report, err := h.store.SaaSAdminSubscriptions(ctx, SaaSAdminSubscriptionOptions{
		TenantID: transition.TenantID,
		Status:   SaaSAdminSubscriptionStatusAll,
		Access:   SaaSAdminSubscriptionAccessAll,
		Limit:    1,
	})
	if err != nil {
		return SaaSAdminSubscriptionTransitionApprovalPlan{}, err
	}
	if len(report.Subscriptions) != 1 {
		return SaaSAdminSubscriptionTransitionApprovalPlan{}, NewSaaSAdminNotFound("tenant subscription not found")
	}
	current := report.Subscriptions[0]
	if current.ID <= 0 || current.TenantID != transition.TenantID || current.TenantStatus <= 0 ||
		current.Version <= 0 || !SaaSAdminSubscriptionStatusValid(current.Status) {
		return SaaSAdminSubscriptionTransitionApprovalPlan{}, &SaaSAdminOperationError{
			Status: http.StatusConflict, Message: "租户订阅快照不完整，请刷新后重试",
		}
	}
	if transition.ExpectedVersion > 0 && transition.ExpectedVersion != current.Version {
		return SaaSAdminSubscriptionTransitionApprovalPlan{}, &SaaSAdminOperationError{
			Status: http.StatusConflict, Message: "订阅版本已变化，请刷新后重试",
		}
	}
	if transition.PackageCode != "" && transition.PackageCode != current.PackageCode {
		return SaaSAdminSubscriptionTransitionApprovalPlan{}, NewSaaSAdminBadRequest("packageCode must match active tenant package")
	}
	if !SaaSAdminSubscriptionTransitionAllowed(current.Status, transition.Status) {
		return SaaSAdminSubscriptionTransitionApprovalPlan{}, NewSaaSAdminBadRequest("subscription transition not allowed: " + current.Status + " -> " + transition.Status)
	}
	transition.ExpectedVersion = current.Version
	next := current
	if err := ApplySaaSAdminSubscriptionTransitionState(&next, transition, time.Now()); err != nil {
		return SaaSAdminSubscriptionTransitionApprovalPlan{}, err
	}
	if SaaSAdminSubscriptionStateEqual(current, next) {
		return SaaSAdminSubscriptionTransitionApprovalPlan{}, &SaaSAdminOperationError{
			Status: http.StatusConflict, Message: "目标订阅状态与当前状态一致，无需重复申请",
		}
	}
	return SaaSAdminSubscriptionTransitionApprovalPlan{
		Transition: transition,
		Subscription: SaaSAdminSubscriptionTransitionSnapshot{
			ID: current.ID, TenantID: current.TenantID, TenantName: current.TenantName,
			TenantStatus: current.TenantStatus, PackageCode: current.PackageCode, PackageName: current.PackageName,
			Status: current.Status, BillingCycle: current.BillingCycle,
			TrialStartsAt: current.TrialStartsAt, TrialEndsAt: current.TrialEndsAt,
			CurrentPeriodStartsAt: current.CurrentPeriodStartsAt, CurrentPeriodEndsAt: current.CurrentPeriodEndsAt,
			GraceEndsAt: current.GraceEndsAt, CancelAtPeriodEnd: current.CancelAtPeriodEnd,
			CanceledAt: current.CanceledAt, SuspendedAt: current.SuspendedAt,
			LatestBillingEventID: current.LatestBillingEventID, Version: current.Version,
			StateReason: current.StateReason,
		},
	}, nil
}

func (h *SaaSAdminHandler) ReconcileSubscriptions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	reconcile, err := parseSaaSAdminSubscriptionReconcile(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	reconcile.ActorUserID = user.ID
	reconcile.ActorTenantID = user.TenantID
	result, err := h.store.ReconcileSaaSAdminSubscriptions(r.Context(), reconcile)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminSubscriptionReconcileResultPayload(result))
}

type saasAdminSubscriptionTransitionRequest struct {
	TenantID              int    `json:"tenantId"`
	TenantIDSnake         int    `json:"tenant_id"`
	Status                string `json:"status"`
	PackageCode           string `json:"packageCode"`
	PackageCodeSnake      string `json:"package_code"`
	BillingCycle          string `json:"billingCycle"`
	BillingCycleSnake     string `json:"billing_cycle"`
	TrialStartsAt         string `json:"trialStartsAt"`
	TrialEndsAt           string `json:"trialEndsAt"`
	CurrentPeriodStartsAt string `json:"currentPeriodStartsAt"`
	CurrentPeriodEndsAt   string `json:"currentPeriodEndsAt"`
	GraceEndsAt           string `json:"graceEndsAt"`
	CancelAtPeriodEnd     *bool  `json:"cancelAtPeriodEnd"`
	ExpectedVersion       int    `json:"expectedVersion"`
	IdempotencyKey        string `json:"idempotencyKey"`
	Reason                string `json:"reason"`
}

func parseSaaSAdminSubscriptionTransition(r *http.Request) (SaaSAdminSubscriptionTransition, error) {
	var req saasAdminSubscriptionTransitionRequest
	if strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "json") {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return SaaSAdminSubscriptionTransition{}, err
		}
		if err := json.Unmarshal(body, &req); err != nil {
			return SaaSAdminSubscriptionTransition{}, errors.New("JSON 格式错误")
		}
	} else {
		if err := r.ParseForm(); err != nil {
			return SaaSAdminSubscriptionTransition{}, err
		}
		req.TenantID, _ = strconv.Atoi(strings.TrimSpace(saasAdminFirstNonEmpty(r.FormValue("tenantId"), r.FormValue("tenant_id"))))
		req.Status = r.FormValue("status")
		req.PackageCode = saasAdminFirstNonEmpty(r.FormValue("packageCode"), r.FormValue("package_code"))
		req.BillingCycle = saasAdminFirstNonEmpty(r.FormValue("billingCycle"), r.FormValue("billing_cycle"))
		req.TrialStartsAt = r.FormValue("trialStartsAt")
		req.TrialEndsAt = r.FormValue("trialEndsAt")
		req.CurrentPeriodStartsAt = r.FormValue("currentPeriodStartsAt")
		req.CurrentPeriodEndsAt = r.FormValue("currentPeriodEndsAt")
		req.GraceEndsAt = r.FormValue("graceEndsAt")
		if raw := strings.TrimSpace(r.FormValue("cancelAtPeriodEnd")); raw != "" {
			value, err := strconv.ParseBool(raw)
			if err != nil {
				return SaaSAdminSubscriptionTransition{}, errors.New("cancelAtPeriodEnd 必须是布尔值")
			}
			req.CancelAtPeriodEnd = &value
		}
		req.ExpectedVersion, _ = strconv.Atoi(strings.TrimSpace(r.FormValue("expectedVersion")))
		req.IdempotencyKey = r.FormValue("idempotencyKey")
		req.Reason = r.FormValue("reason")
	}
	if req.TenantID <= 0 {
		req.TenantID = req.TenantIDSnake
	}
	status := strings.ToLower(strings.TrimSpace(req.Status))
	if req.TenantID <= 0 {
		return SaaSAdminSubscriptionTransition{}, errors.New("tenantId required")
	}
	if !SaaSAdminSubscriptionStatusValid(status) {
		return SaaSAdminSubscriptionTransition{}, errors.New("status 必须是 trialing、active、grace、past_due、suspended 或 canceled")
	}
	packageCode := strings.TrimSpace(saasAdminFirstNonEmpty(req.PackageCode, req.PackageCodeSnake))
	if len(packageCode) > 64 {
		return SaaSAdminSubscriptionTransition{}, errors.New("packageCode too long")
	}
	billingCycle := strings.ToLower(strings.TrimSpace(saasAdminFirstNonEmpty(req.BillingCycle, req.BillingCycleSnake)))
	if billingCycle != "" && billingCycle != "monthly" && billingCycle != "yearly" && billingCycle != "custom" && billingCycle != "lifetime" {
		return SaaSAdminSubscriptionTransition{}, errors.New("billingCycle 必须是 monthly、yearly、custom 或 lifetime")
	}
	dates := []*string{&req.TrialStartsAt, &req.TrialEndsAt, &req.CurrentPeriodStartsAt, &req.CurrentPeriodEndsAt, &req.GraceEndsAt}
	for _, raw := range dates {
		value, err := normalizeSaaSAdminOptionalDateTime(*raw)
		if err != nil {
			return SaaSAdminSubscriptionTransition{}, err
		}
		*raw = value
	}
	if req.ExpectedVersion < 0 {
		return SaaSAdminSubscriptionTransition{}, errors.New("expectedVersion must not be negative")
	}
	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	if len(idempotencyKey) > 128 {
		return SaaSAdminSubscriptionTransition{}, errors.New("idempotencyKey too long")
	}
	reason := strings.TrimSpace(req.Reason)
	if len([]rune(reason)) > 255 {
		return SaaSAdminSubscriptionTransition{}, errors.New("reason too long")
	}
	return SaaSAdminSubscriptionTransition{
		TenantID: req.TenantID, Status: status, PackageCode: packageCode, BillingCycle: billingCycle,
		TrialStartsAt: req.TrialStartsAt, TrialEndsAt: req.TrialEndsAt,
		CurrentPeriodStartsAt: req.CurrentPeriodStartsAt, CurrentPeriodEndsAt: req.CurrentPeriodEndsAt,
		GraceEndsAt: req.GraceEndsAt, CancelAtPeriodEnd: req.CancelAtPeriodEnd,
		ExpectedVersion: req.ExpectedVersion, IdempotencyKey: idempotencyKey, Reason: reason,
	}, nil
}

func parseSaaSAdminSubscriptionOptions(r *http.Request, maxLimit int) (SaaSAdminSubscriptionOptions, error) {
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	if status == "" {
		status = SaaSAdminSubscriptionStatusAll
	}
	if status != SaaSAdminSubscriptionStatusAll && !SaaSAdminSubscriptionStatusValid(status) {
		return SaaSAdminSubscriptionOptions{}, errors.New("status invalid")
	}
	access := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("access")))
	if access == "" {
		access = SaaSAdminSubscriptionAccessAll
	}
	if access != SaaSAdminSubscriptionAccessAll && access != SaaSAdminSubscriptionAccessAllowed && access != SaaSAdminSubscriptionAccessBlocked {
		return SaaSAdminSubscriptionOptions{}, errors.New("access 必须是 all、allowed 或 blocked")
	}
	limit := positiveQueryInt(r, "limit", 100)
	if limit > maxLimit {
		limit = maxLimit
	}
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	if len([]rune(keyword)) > 80 {
		return SaaSAdminSubscriptionOptions{}, errors.New("keyword too long")
	}
	packageCode := strings.TrimSpace(r.URL.Query().Get("packageCode"))
	if len(packageCode) > 64 {
		return SaaSAdminSubscriptionOptions{}, errors.New("packageCode too long")
	}
	return SaaSAdminSubscriptionOptions{
		TenantID: saasAdminQueryInt(r, "tenantId", 0), Status: status, Access: access,
		PackageCode: packageCode, Keyword: keyword, Limit: limit,
	}, nil
}

func parseSaaSAdminSubscriptionEventOptions(r *http.Request, maxLimit int) (SaaSAdminSubscriptionEventOptions, error) {
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	if status != "" && status != SaaSAdminSubscriptionStatusAll && !SaaSAdminSubscriptionStatusValid(status) {
		return SaaSAdminSubscriptionEventOptions{}, errors.New("status invalid")
	}
	if status == SaaSAdminSubscriptionStatusAll {
		status = ""
	}
	limit := positiveQueryInt(r, "limit", 50)
	if limit > maxLimit {
		limit = maxLimit
	}
	source := strings.TrimSpace(r.URL.Query().Get("source"))
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	if len(source) > 32 || len([]rune(keyword)) > 80 {
		return SaaSAdminSubscriptionEventOptions{}, errors.New("filter too long")
	}
	return SaaSAdminSubscriptionEventOptions{TenantID: saasAdminQueryInt(r, "tenantId", 0), Status: status, Source: source, Keyword: keyword, Limit: limit}, nil
}

type saasAdminSubscriptionReconcileRequest struct {
	TenantID int  `json:"tenantId"`
	Limit    int  `json:"limit"`
	DryRun   bool `json:"dryRun"`
}

func parseSaaSAdminSubscriptionReconcile(r *http.Request) (SaaSAdminSubscriptionReconcile, error) {
	req := saasAdminSubscriptionReconcileRequest{Limit: 500}
	if strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "json") {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return SaaSAdminSubscriptionReconcile{}, err
		}
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				return SaaSAdminSubscriptionReconcile{}, errors.New("JSON 格式错误")
			}
		}
	} else {
		if err := r.ParseForm(); err != nil {
			return SaaSAdminSubscriptionReconcile{}, err
		}
		req.TenantID, _ = strconv.Atoi(strings.TrimSpace(r.FormValue("tenantId")))
		if raw := strings.TrimSpace(r.FormValue("limit")); raw != "" {
			req.Limit, _ = strconv.Atoi(raw)
		}
		if raw := strings.TrimSpace(r.FormValue("dryRun")); raw != "" {
			value, err := strconv.ParseBool(raw)
			if err != nil {
				return SaaSAdminSubscriptionReconcile{}, errors.New("dryRun 必须是布尔值")
			}
			req.DryRun = value
		}
	}
	if req.TenantID < 0 || req.Limit <= 0 || req.Limit > 5000 {
		return SaaSAdminSubscriptionReconcile{}, errors.New("tenantId 或 limit invalid")
	}
	return SaaSAdminSubscriptionReconcile{TenantID: req.TenantID, Limit: req.Limit, DryRun: req.DryRun}, nil
}

func saasAdminSubscriptionReportPayload(report SaaSAdminSubscriptionReport, platformTenantID int) map[string]any {
	return map[string]any{
		"platformAdminTenantId": platformTenantID,
		"generatedAt":           time.Now().Format("2006-01-02 15:04:05"),
		"filters": map[string]any{
			"tenantId": report.Options.TenantID, "status": report.Options.Status,
			"access": report.Options.Access, "packageCode": report.Options.PackageCode,
			"keyword": report.Options.Keyword, "limit": report.Options.Limit,
		},
		"summary": map[string]any{
			"subscriptionCount": report.Summary.SubscriptionCount, "trialingCount": report.Summary.TrialingCount,
			"activeCount": report.Summary.ActiveCount, "graceCount": report.Summary.GraceCount,
			"pastDueCount": report.Summary.PastDueCount, "suspendedCount": report.Summary.SuspendedCount,
			"canceledCount": report.Summary.CanceledCount, "accessAllowedCount": report.Summary.AccessAllowedCount,
			"accessBlockedCount": report.Summary.AccessBlockedCount, "reconciliationDueCount": report.Summary.ReconciliationDueCount,
			"packageCount": report.Summary.PackageCount, "tenantCount": report.Summary.TenantCount,
		},
		"count":         len(report.Subscriptions),
		"subscriptions": saasAdminSubscriptionPayloads(report.Subscriptions),
	}
}

func saasAdminSubscriptionPayloads(items []SaaSAdminSubscription) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, saasAdminSubscriptionPayload(item))
	}
	return result
}

func saasAdminSubscriptionPayload(item SaaSAdminSubscription) map[string]any {
	return map[string]any{
		"id": item.ID, "tenantId": item.TenantID, "tenantName": item.TenantName, "tenantStatus": item.TenantStatus,
		"packageCode": item.PackageCode, "packageName": item.PackageName,
		"status": item.Status, "effectiveStatus": item.EffectiveStatus, "billingCycle": item.BillingCycle,
		"trialStartsAt": item.TrialStartsAt, "trialEndsAt": item.TrialEndsAt,
		"currentPeriodStartsAt": item.CurrentPeriodStartsAt, "currentPeriodEndsAt": item.CurrentPeriodEndsAt,
		"graceEndsAt": item.GraceEndsAt, "cancelAtPeriodEnd": item.CancelAtPeriodEnd,
		"canceledAt": item.CanceledAt, "suspendedAt": item.SuspendedAt,
		"latestBillingEventId": item.LatestBillingEventID, "version": item.Version,
		"stateReason": item.StateReason, "metadataJson": item.MetadataJSON,
		"createdAt": item.CreatedAt, "updatedAt": item.UpdatedAt,
		"accessAllowed": item.AccessAllowed, "accessReason": item.AccessReason,
		"needsReconciliation": item.NeedsReconciliation, "nextActionAt": item.NextActionAt,
	}
}

func saasAdminSubscriptionEventPayloads(items []SaaSAdminSubscriptionEvent) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"id": item.ID, "subscriptionId": item.SubscriptionID, "tenantId": item.TenantID,
			"tenantName": item.TenantName, "eventType": item.EventType,
			"fromStatus": item.FromStatus, "toStatus": item.ToStatus, "effectiveAt": item.EffectiveAt,
			"actorUserId": item.ActorUserID, "actorTenantId": item.ActorTenantID,
			"source": item.Source, "idempotencyKey": item.IdempotencyKey,
			"reason": item.Reason, "payloadJson": item.PayloadJSON, "createdAt": item.CreatedAt,
		})
	}
	return result
}

func saasAdminSubscriptionTransitionResultPayload(result SaaSAdminSubscriptionTransitionResult) map[string]any {
	return map[string]any{
		"changed": result.Changed, "idempotent": result.Idempotent,
		"previousStatus": result.PreviousStatus, "eventId": result.EventID,
		"operationId": result.OperationID, "subscription": saasAdminSubscriptionPayload(result.Subscription),
	}
}

func saasAdminSubscriptionReconcileResultPayload(result SaaSAdminSubscriptionReconcileResult) map[string]any {
	transitions := make([]map[string]any, 0, len(result.Transitions))
	for _, item := range result.Transitions {
		transitions = append(transitions, saasAdminSubscriptionTransitionResultPayload(item))
	}
	errorsPayload := make([]map[string]any, 0, len(result.Errors))
	for _, item := range result.Errors {
		errorsPayload = append(errorsPayload, map[string]any{"tenantId": item.TenantID, "error": item.Error})
	}
	return map[string]any{
		"scannedCount": result.ScannedCount, "reconciliationDue": result.ReconciliationDue,
		"changedCount": result.ChangedCount, "skippedCount": result.SkippedCount,
		"failedCount": result.FailedCount, "dryRun": result.DryRun,
		"transitions": transitions, "errors": errorsPayload,
	}
}

func writeSaaSAdminSubscriptionCSV(writer *csv.Writer, report SaaSAdminSubscriptionReport) {
	_ = writer.Write([]string{"租户ID", "租户名称", "套餐编码", "套餐名称", "存储状态", "有效状态", "访问", "计费周期", "试用结束", "周期结束", "宽限结束", "期末取消", "版本", "待对账", "状态原因", "更新时间"})
	for _, item := range report.Subscriptions {
		access := "阻断"
		if item.AccessAllowed {
			access = "允许"
		}
		_ = writer.Write([]string{
			strconv.Itoa(item.TenantID), item.TenantName, item.PackageCode, item.PackageName,
			item.Status, item.EffectiveStatus, access, item.BillingCycle, item.TrialEndsAt,
			item.CurrentPeriodEndsAt, item.GraceEndsAt, strconv.FormatBool(item.CancelAtPeriodEnd),
			strconv.Itoa(item.Version), strconv.FormatBool(item.NeedsReconciliation), item.StateReason, item.UpdatedAt,
		})
	}
}
