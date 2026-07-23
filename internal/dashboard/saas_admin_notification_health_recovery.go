package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

type SaaSAdminNotificationHealthRecovery struct {
	Options       SaaSAdminNotificationHealthOptions
	Remark        string
	ActorUserID   int
	ActorTenantID int
}

type SaaSAdminNotificationHealthRecoveryResult struct {
	Options                 SaaSAdminNotificationHealthOptions
	MatchedAssignmentCount  int
	ActiveAssignmentCount   int
	RecoveredTenantCount    int
	ClosedCount             int
	AlreadyClosedCount      int
	UnhealthyCount          int
	NoDataCount             int
	NoDeliveryEvidenceCount int
	MissingTenantCount      int
	Remark                  string
	Closed                  []SaaSAdminOperationQueueAssignmentCloseResult
	Skipped                 []SaaSAdminNotificationHealthRecoverySkipped
}

type SaaSAdminNotificationHealthRecoverySkipped struct {
	OperationID int64
	TenantID    int
	HealthState string
	Reason      string
}

func (h *SaaSAdminHandler) NotificationHealthRecovery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	user, ok := h.resolvePlatformSuperAdmin(w, r)
	if !ok {
		return
	}
	options, ok := h.notificationHealthOptions(w, r, saasAdminExportMaxLimit, saasAdminExportMaxLimit)
	if !ok {
		return
	}
	options.State = SaaSAdminNotificationHealthStateAll
	options.Limit = saasAdminExportMaxLimit
	recovery, err := parseSaaSAdminNotificationHealthRecovery(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	recovery.Options = options
	recovery.ActorUserID = user.ID
	recovery.ActorTenantID = user.TenantID
	result, err := h.recoverNotificationHealthAssignments(r.Context(), recovery)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminNotificationHealthRecoveryPayload(result, h.platformAdminTenantID))
}

func (h *SaaSAdminHandler) recoverNotificationHealthAssignments(ctx context.Context, recovery SaaSAdminNotificationHealthRecovery) (SaaSAdminNotificationHealthRecoveryResult, error) {
	source, err := h.store.SaaSAdminNotificationHealth(ctx, recovery.Options)
	if err != nil {
		return SaaSAdminNotificationHealthRecoveryResult{}, err
	}
	healthByTenant := make(map[int]SaaSAdminNotificationHealthTenant, len(source.Tenants))
	for _, snapshot := range source.Tenants {
		healthByTenant[snapshot.TenantID] = saasAdminNotificationHealthTenant(snapshot, recovery.Options)
	}
	latest, err := h.latestOperationQueueAssignments(ctx)
	if err != nil {
		return SaaSAdminNotificationHealthRecoveryResult{}, err
	}
	assignments := make([]SaaSAdminOperationQueueAssignment, 0, len(latest))
	for _, assignment := range latest {
		if assignment.Source != SaaSAdminOperationQueueSourceNotificationHealth {
			continue
		}
		if recovery.Options.TenantID > 0 && assignment.TenantID != recovery.Options.TenantID {
			continue
		}
		if recovery.Options.Keyword != "" {
			if _, ok := healthByTenant[assignment.TenantID]; !ok {
				continue
			}
		}
		assignments = append(assignments, assignment)
	}
	sort.SliceStable(assignments, func(i, j int) bool {
		return assignments[i].OperationID > assignments[j].OperationID
	})

	result := SaaSAdminNotificationHealthRecoveryResult{
		Options: recovery.Options,
		Remark:  recovery.Remark,
		Closed:  make([]SaaSAdminOperationQueueAssignmentCloseResult, 0),
		Skipped: make([]SaaSAdminNotificationHealthRecoverySkipped, 0),
	}
	for _, assignment := range assignments {
		result.MatchedAssignmentCount++
		assignment.DueState = saasAdminOperationQueueAssignmentDueState(assignment, time.Now())
		if assignment.DueState == SaaSAdminRiskFollowUpDueStateClosed {
			result.AlreadyClosedCount++
			result.Skipped = append(result.Skipped, SaaSAdminNotificationHealthRecoverySkipped{
				OperationID: assignment.OperationID,
				TenantID:    assignment.TenantID,
				Reason:      "already_closed",
			})
			continue
		}
		result.ActiveAssignmentCount++
		health, found := healthByTenant[assignment.TenantID]
		if !found {
			result.MissingTenantCount++
			result.Skipped = append(result.Skipped, SaaSAdminNotificationHealthRecoverySkipped{
				OperationID: assignment.OperationID,
				TenantID:    assignment.TenantID,
				Reason:      "tenant_not_found",
			})
			continue
		}
		if health.HealthState == SaaSAdminNotificationHealthStateNoData {
			result.NoDataCount++
			result.Skipped = append(result.Skipped, SaaSAdminNotificationHealthRecoverySkipped{
				OperationID: assignment.OperationID,
				TenantID:    assignment.TenantID,
				HealthState: health.HealthState,
				Reason:      "no_delivery_evidence",
			})
			continue
		}
		if health.HealthState == SaaSAdminNotificationHealthStateHealthy && (health.AttemptedCount <= 0 || health.DeliveredCount <= 0) {
			result.NoDeliveryEvidenceCount++
			result.Skipped = append(result.Skipped, SaaSAdminNotificationHealthRecoverySkipped{
				OperationID: assignment.OperationID,
				TenantID:    assignment.TenantID,
				HealthState: health.HealthState,
				Reason:      "no_successful_delivery",
			})
			continue
		}
		if health.HealthState != SaaSAdminNotificationHealthStateHealthy {
			result.UnhealthyCount++
			result.Skipped = append(result.Skipped, SaaSAdminNotificationHealthRecoverySkipped{
				OperationID: assignment.OperationID,
				TenantID:    assignment.TenantID,
				HealthState: health.HealthState,
				Reason:      "still_unhealthy",
			})
			continue
		}

		result.RecoveredTenantCount++
		closed, err := h.closeOperationQueueAssignment(ctx, assignment, SaaSAdminOperationQueueAssignmentClose{
			OperationID:   assignment.OperationID,
			Status:        SaaSAdminRiskFollowUpStatusResolved,
			Remark:        recovery.Remark,
			ActorUserID:   recovery.ActorUserID,
			ActorTenantID: recovery.ActorTenantID,
			Context: map[string]any{
				"reason": "notification_health_recovered",
				"health": saasAdminNotificationHealthTenantPayload(health),
				"filters": map[string]any{
					"channel":      recovery.Options.Channel,
					"windowHours":  recovery.Options.WindowHours,
					"staleMinutes": recovery.Options.StaleMinutes,
				},
			},
		})
		if err != nil {
			return SaaSAdminNotificationHealthRecoveryResult{}, err
		}
		if closed.Closed {
			result.ClosedCount++
		}
		if closed.AlreadyClosed {
			result.AlreadyClosedCount++
		}
		result.Closed = append(result.Closed, closed)
	}
	return result, nil
}

type saasAdminNotificationHealthRecoveryRequest struct {
	Remark string `json:"remark"`
}

func parseSaaSAdminNotificationHealthRecovery(r *http.Request) (SaaSAdminNotificationHealthRecovery, error) {
	var req saasAdminNotificationHealthRecoveryRequest
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return SaaSAdminNotificationHealthRecovery{}, err
		}
		req.Remark = r.FormValue("remark")
	} else {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return SaaSAdminNotificationHealthRecovery{}, err
		}
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				return SaaSAdminNotificationHealthRecovery{}, errors.New("JSON 格式错误")
			}
		}
	}
	remark := strings.TrimSpace(req.Remark)
	if remark == "" {
		remark = "通知健康恢复自动结案"
	}
	if len([]rune(remark)) > 255 {
		return SaaSAdminNotificationHealthRecovery{}, errors.New("remark too long")
	}
	return SaaSAdminNotificationHealthRecovery{Remark: remark}, nil
}

func saasAdminNotificationHealthRecoveryPayload(result SaaSAdminNotificationHealthRecoveryResult, platformAdminTenantID int) map[string]any {
	closed := make([]map[string]any, 0, len(result.Closed))
	for _, item := range result.Closed {
		closed = append(closed, saasAdminOperationQueueAssignmentClosePayload(item, platformAdminTenantID))
	}
	skipped := make([]map[string]any, 0, len(result.Skipped))
	for _, item := range result.Skipped {
		skipped = append(skipped, map[string]any{
			"operationId": item.OperationID,
			"tenantId":    item.TenantID,
			"healthState": item.HealthState,
			"reason":      item.Reason,
		})
	}
	return map[string]any{
		"canPlatformScope":      true,
		"platformAdminTenantId": platformAdminTenantID,
		"generatedAt":           time.Now().Format("2006-01-02 15:04:05"),
		"filters": map[string]any{
			"tenantId":     result.Options.TenantID,
			"channel":      result.Options.Channel,
			"keyword":      result.Options.Keyword,
			"windowHours":  result.Options.WindowHours,
			"staleMinutes": result.Options.StaleMinutes,
		},
		"matchedAssignmentCount":  result.MatchedAssignmentCount,
		"activeAssignmentCount":   result.ActiveAssignmentCount,
		"recoveredTenantCount":    result.RecoveredTenantCount,
		"closedCount":             result.ClosedCount,
		"alreadyClosedCount":      result.AlreadyClosedCount,
		"unhealthyCount":          result.UnhealthyCount,
		"noDataCount":             result.NoDataCount,
		"noDeliveryEvidenceCount": result.NoDeliveryEvidenceCount,
		"missingTenantCount":      result.MissingTenantCount,
		"remark":                  result.Remark,
		"closed":                  closed,
		"skipped":                 skipped,
	}
}
