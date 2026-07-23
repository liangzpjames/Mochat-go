package dashboard

import (
	"encoding/csv"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	SaaSAdminNotificationHealthStateAll      = "all"
	SaaSAdminNotificationHealthStateHealthy  = "healthy"
	SaaSAdminNotificationHealthStateWarning  = "warning"
	SaaSAdminNotificationHealthStateCritical = "critical"
	SaaSAdminNotificationHealthStateNoData   = "no_data"

	saasAdminNotificationHealthDefaultWindowHours  = 24
	saasAdminNotificationHealthMaxWindowHours      = 720
	saasAdminNotificationHealthDefaultStaleMinutes = 15
	saasAdminNotificationHealthMaxStaleMinutes     = 10080
	saasAdminNotificationHealthRateSampleSize      = 5
	saasAdminNotificationHealthWarningRate         = 0.95
	saasAdminNotificationHealthCriticalRate        = 0.80
	saasAdminNotificationHealthFailureReasonLimit  = 10
)

type SaaSAdminNotificationHealthOptions struct {
	TenantID         int
	ExcludedTenantID int
	Channel          string
	Keyword          string
	State            string
	WindowHours      int
	StaleMinutes     int
	Limit            int
}

type SaaSAdminNotificationHealthSource struct {
	Tenants        []SaaSAdminNotificationHealthSnapshot
	FailureReasons []SaaSAdminNotificationHealthFailureSnapshot
}

type SaaSAdminNotificationHealthSnapshot struct {
	TenantID               int
	TenantName             string
	TenantStatus           int
	PackageCode            string
	PackageName            string
	PolicyConfigured       bool
	PolicyEnabled          bool
	NotificationCount      int
	PendingCount           int
	ReadyPendingCount      int
	DeferredCount          int
	StalePendingCount      int
	FailedCount            int
	DeliveredCount         int
	DeadCount              int
	ClosedCount            int
	SuppressedCount        int
	TotalAttempts          int64
	AverageDeliverySeconds float64
	MaxDeliverySeconds     int64
	OldestPendingAt        string
	LastDeliveredAt        string
	LastFailureAt          string
	LatestNotificationAt   string
}

type SaaSAdminNotificationHealthFailureSnapshot struct {
	TenantID       int
	Reason         string
	Count          int
	LastOccurredAt string
}

type SaaSAdminNotificationHealthReport struct {
	Options        SaaSAdminNotificationHealthOptions
	WindowStartAt  string
	WindowEndAt    string
	Summary        SaaSAdminNotificationHealthSummary
	Tenants        []SaaSAdminNotificationHealthTenant
	FailureReasons []SaaSAdminNotificationHealthFailureReason
}

type SaaSAdminNotificationHealthSummary struct {
	TenantCount            int
	MatchedTenantCount     int
	ReturnedTenantCount    int
	HealthyTenantCount     int
	WarningTenantCount     int
	CriticalTenantCount    int
	NoDataTenantCount      int
	PolicyConfiguredCount  int
	PolicyEnabledCount     int
	NotificationCount      int
	AttemptedCount         int
	DeliveredCount         int
	PendingCount           int
	ReadyPendingCount      int
	DeferredCount          int
	StalePendingCount      int
	FailedCount            int
	DeadCount              int
	ClosedCount            int
	SuppressedCount        int
	TotalAttempts          int64
	DeliverySuccessRate    float64
	AverageDeliverySeconds float64
	MaxDeliverySeconds     int64
	FailureReasonCount     int
}

type SaaSAdminNotificationHealthTenant struct {
	SaaSAdminNotificationHealthSnapshot
	HealthState         string
	AttemptedCount      int
	DeliverySuccessRate float64
	Reasons             []string
	SuggestedAction     string
}

type SaaSAdminNotificationHealthFailureReason struct {
	Reason         string
	Count          int
	TenantCount    int
	LastOccurredAt string
}

func (h *SaaSAdminHandler) NotificationHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	options, ok := h.notificationHealthOptions(w, r, positiveQueryInt(r, "limit", 50), saasAdminListMaxLimit)
	if !ok {
		return
	}
	source, err := h.store.SaaSAdminNotificationHealth(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	report := saasAdminBuildNotificationHealthReport(source, options, time.Now())
	writeEnvelope(w, http.StatusOK, 200, "success", saasAdminNotificationHealthPayload(report, h.platformAdminTenantID))
}

func (h *SaaSAdminHandler) notificationHealthOptions(w http.ResponseWriter, r *http.Request, defaultLimit, maxLimit int) (SaaSAdminNotificationHealthOptions, bool) {
	options := SaaSAdminNotificationHealthOptions{
		TenantID:     saasAdminQueryInt(r, "tenantId", 0),
		Channel:      strings.ToLower(strings.TrimSpace(r.URL.Query().Get("channel"))),
		Keyword:      strings.TrimSpace(r.URL.Query().Get("keyword")),
		State:        strings.ToLower(strings.TrimSpace(r.URL.Query().Get("state"))),
		WindowHours:  positiveQueryInt(r, "windowHours", saasAdminNotificationHealthDefaultWindowHours),
		StaleMinutes: positiveQueryInt(r, "staleMinutes", saasAdminNotificationHealthDefaultStaleMinutes),
		Limit:        positiveQueryInt(r, "limit", defaultLimit),
	}
	if options.Channel == "" {
		options.Channel = SaaSAlertNotificationChannelWebhook
	}
	if options.Channel != SaaSAlertNotificationChannelWebhook {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "channel 目前仅支持 webhook", nil)
		return SaaSAdminNotificationHealthOptions{}, false
	}
	if options.State == "" {
		options.State = SaaSAdminNotificationHealthStateAll
	}
	switch options.State {
	case SaaSAdminNotificationHealthStateAll, SaaSAdminNotificationHealthStateHealthy, SaaSAdminNotificationHealthStateWarning, SaaSAdminNotificationHealthStateCritical, SaaSAdminNotificationHealthStateNoData:
	default:
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "state 必须是 all、healthy、warning、critical 或 no_data", nil)
		return SaaSAdminNotificationHealthOptions{}, false
	}
	if options.WindowHours > saasAdminNotificationHealthMaxWindowHours {
		options.WindowHours = saasAdminNotificationHealthMaxWindowHours
	}
	if options.StaleMinutes > saasAdminNotificationHealthMaxStaleMinutes {
		options.StaleMinutes = saasAdminNotificationHealthMaxStaleMinutes
	}
	if options.Limit > maxLimit {
		options.Limit = maxLimit
	}
	return options, true
}

func saasAdminBuildNotificationHealthReport(source SaaSAdminNotificationHealthSource, options SaaSAdminNotificationHealthOptions, now time.Time) SaaSAdminNotificationHealthReport {
	if options.Channel == "" {
		options.Channel = SaaSAlertNotificationChannelWebhook
	}
	if options.State == "" {
		options.State = SaaSAdminNotificationHealthStateAll
	}
	if options.WindowHours <= 0 {
		options.WindowHours = saasAdminNotificationHealthDefaultWindowHours
	}
	if options.StaleMinutes <= 0 {
		options.StaleMinutes = saasAdminNotificationHealthDefaultStaleMinutes
	}
	if options.Limit <= 0 {
		options.Limit = 50
	}
	report := SaaSAdminNotificationHealthReport{
		Options:        options,
		WindowStartAt:  now.Add(-time.Duration(options.WindowHours) * time.Hour).Format("2006-01-02 15:04:05"),
		WindowEndAt:    now.Format("2006-01-02 15:04:05"),
		Tenants:        make([]SaaSAdminNotificationHealthTenant, 0, len(source.Tenants)),
		FailureReasons: []SaaSAdminNotificationHealthFailureReason{},
	}
	matchedTenantIDs := map[int]struct{}{}
	weightedDeliverySeconds := 0.0
	for _, snapshot := range source.Tenants {
		item := saasAdminNotificationHealthTenant(snapshot, options)
		report.Summary.TenantCount++
		if snapshot.PolicyConfigured {
			report.Summary.PolicyConfiguredCount++
		}
		if snapshot.PolicyEnabled {
			report.Summary.PolicyEnabledCount++
		}
		switch item.HealthState {
		case SaaSAdminNotificationHealthStateCritical:
			report.Summary.CriticalTenantCount++
		case SaaSAdminNotificationHealthStateWarning:
			report.Summary.WarningTenantCount++
		case SaaSAdminNotificationHealthStateHealthy:
			report.Summary.HealthyTenantCount++
		case SaaSAdminNotificationHealthStateNoData:
			report.Summary.NoDataTenantCount++
		}
		if options.State != SaaSAdminNotificationHealthStateAll && item.HealthState != options.State {
			continue
		}
		report.Tenants = append(report.Tenants, item)
		matchedTenantIDs[item.TenantID] = struct{}{}
		report.Summary.MatchedTenantCount++
		report.Summary.NotificationCount += item.NotificationCount
		report.Summary.AttemptedCount += item.AttemptedCount
		report.Summary.DeliveredCount += item.DeliveredCount
		report.Summary.PendingCount += item.PendingCount
		report.Summary.ReadyPendingCount += item.ReadyPendingCount
		report.Summary.DeferredCount += item.DeferredCount
		report.Summary.StalePendingCount += item.StalePendingCount
		report.Summary.FailedCount += item.FailedCount
		report.Summary.DeadCount += item.DeadCount
		report.Summary.ClosedCount += item.ClosedCount
		report.Summary.SuppressedCount += item.SuppressedCount
		report.Summary.TotalAttempts += item.TotalAttempts
		weightedDeliverySeconds += item.AverageDeliverySeconds * float64(item.DeliveredCount)
		if item.MaxDeliverySeconds > report.Summary.MaxDeliverySeconds {
			report.Summary.MaxDeliverySeconds = item.MaxDeliverySeconds
		}
	}
	report.Summary.DeliverySuccessRate = saasAdminNotificationHealthRate(report.Summary.DeliveredCount, report.Summary.AttemptedCount)
	if report.Summary.DeliveredCount > 0 {
		report.Summary.AverageDeliverySeconds = roundFloat(weightedDeliverySeconds/float64(report.Summary.DeliveredCount), 2)
	}
	sort.SliceStable(report.Tenants, func(i, j int) bool {
		left, right := report.Tenants[i], report.Tenants[j]
		if saasAdminNotificationHealthStateRank(left.HealthState) != saasAdminNotificationHealthStateRank(right.HealthState) {
			return saasAdminNotificationHealthStateRank(left.HealthState) < saasAdminNotificationHealthStateRank(right.HealthState)
		}
		if left.DeadCount != right.DeadCount {
			return left.DeadCount > right.DeadCount
		}
		if left.StalePendingCount != right.StalePendingCount {
			return left.StalePendingCount > right.StalePendingCount
		}
		if left.FailedCount != right.FailedCount {
			return left.FailedCount > right.FailedCount
		}
		if left.DeliverySuccessRate != right.DeliverySuccessRate {
			return left.DeliverySuccessRate < right.DeliverySuccessRate
		}
		if left.NotificationCount != right.NotificationCount {
			return left.NotificationCount > right.NotificationCount
		}
		return left.TenantID < right.TenantID
	})
	report.FailureReasons = saasAdminNotificationHealthFailureReasons(source.FailureReasons, matchedTenantIDs)
	report.Summary.FailureReasonCount = len(report.FailureReasons)
	if len(report.Tenants) > options.Limit {
		report.Tenants = report.Tenants[:options.Limit]
	}
	report.Summary.ReturnedTenantCount = len(report.Tenants)
	return report
}

func saasAdminNotificationHealthTenant(snapshot SaaSAdminNotificationHealthSnapshot, options SaaSAdminNotificationHealthOptions) SaaSAdminNotificationHealthTenant {
	item := SaaSAdminNotificationHealthTenant{SaaSAdminNotificationHealthSnapshot: snapshot}
	item.AttemptedCount = snapshot.DeliveredCount + snapshot.FailedCount + snapshot.DeadCount
	item.DeliverySuccessRate = saasAdminNotificationHealthRate(snapshot.DeliveredCount, item.AttemptedCount)
	if snapshot.NotificationCount == 0 {
		item.HealthState = SaaSAdminNotificationHealthStateNoData
		item.Reasons = []string{"窗口内无通知"}
		item.SuggestedAction = "确认租户是否需要通知或尚未产生业务事件"
		return item
	}
	rateCritical := item.AttemptedCount >= saasAdminNotificationHealthRateSampleSize && item.DeliverySuccessRate < saasAdminNotificationHealthCriticalRate
	rateWarning := item.AttemptedCount >= saasAdminNotificationHealthRateSampleSize && item.DeliverySuccessRate < saasAdminNotificationHealthWarningRate
	if snapshot.DeadCount > 0 {
		item.Reasons = append(item.Reasons, strconv.Itoa(snapshot.DeadCount)+" 条通知已耗尽")
	}
	if snapshot.StalePendingCount > 0 {
		item.Reasons = append(item.Reasons, strconv.Itoa(snapshot.StalePendingCount)+" 条通知积压超过 "+strconv.Itoa(options.StaleMinutes)+" 分钟")
	}
	if rateCritical || rateWarning {
		item.Reasons = append(item.Reasons, "送达成功率 "+strconv.FormatFloat(item.DeliverySuccessRate*100, 'f', 1, 64)+"%")
	}
	if snapshot.FailedCount > 0 {
		item.Reasons = append(item.Reasons, strconv.Itoa(snapshot.FailedCount)+" 条通知等待重试")
	}
	if snapshot.ReadyPendingCount > 0 && snapshot.StalePendingCount == 0 {
		item.Reasons = append(item.Reasons, strconv.Itoa(snapshot.ReadyPendingCount)+" 条通知已到投递时间")
	}
	switch {
	case snapshot.DeadCount > 0 || snapshot.StalePendingCount > 0 || rateCritical:
		item.HealthState = SaaSAdminNotificationHealthStateCritical
		item.SuggestedAction = "检查 Webhook、重试耗尽和 dispatcher 积压"
	case snapshot.FailedCount > 0 || snapshot.ReadyPendingCount > 0 || rateWarning:
		item.HealthState = SaaSAdminNotificationHealthStateWarning
		item.SuggestedAction = "处理待重试通知并确认 dispatcher 正常运行"
	default:
		item.HealthState = SaaSAdminNotificationHealthStateHealthy
		item.SuggestedAction = "保持观察"
	}
	return item
}

func saasAdminNotificationHealthFailureReasons(source []SaaSAdminNotificationHealthFailureSnapshot, matchedTenantIDs map[int]struct{}) []SaaSAdminNotificationHealthFailureReason {
	type aggregate struct {
		SaaSAdminNotificationHealthFailureReason
		tenantIDs map[int]struct{}
	}
	byReason := map[string]*aggregate{}
	for _, item := range source {
		if _, ok := matchedTenantIDs[item.TenantID]; !ok {
			continue
		}
		reason := strings.TrimSpace(item.Reason)
		if reason == "" || item.Count <= 0 {
			continue
		}
		entry := byReason[reason]
		if entry == nil {
			entry = &aggregate{SaaSAdminNotificationHealthFailureReason: SaaSAdminNotificationHealthFailureReason{Reason: reason}, tenantIDs: map[int]struct{}{}}
			byReason[reason] = entry
		}
		entry.Count += item.Count
		entry.tenantIDs[item.TenantID] = struct{}{}
		if item.LastOccurredAt > entry.LastOccurredAt {
			entry.LastOccurredAt = item.LastOccurredAt
		}
	}
	result := make([]SaaSAdminNotificationHealthFailureReason, 0, len(byReason))
	for _, item := range byReason {
		item.TenantCount = len(item.tenantIDs)
		result = append(result, item.SaaSAdminNotificationHealthFailureReason)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Count != result[j].Count {
			return result[i].Count > result[j].Count
		}
		if result[i].TenantCount != result[j].TenantCount {
			return result[i].TenantCount > result[j].TenantCount
		}
		if result[i].LastOccurredAt != result[j].LastOccurredAt {
			return result[i].LastOccurredAt > result[j].LastOccurredAt
		}
		return result[i].Reason < result[j].Reason
	})
	if len(result) > saasAdminNotificationHealthFailureReasonLimit {
		result = result[:saasAdminNotificationHealthFailureReasonLimit]
	}
	return result
}

func saasAdminNotificationHealthRate(delivered, attempted int) float64 {
	if attempted <= 0 {
		return 0
	}
	return roundFloat(float64(delivered)/float64(attempted), 4)
}

func saasAdminNotificationHealthStateRank(state string) int {
	switch state {
	case SaaSAdminNotificationHealthStateCritical:
		return 0
	case SaaSAdminNotificationHealthStateWarning:
		return 1
	case SaaSAdminNotificationHealthStateHealthy:
		return 2
	default:
		return 3
	}
}

func saasAdminNotificationHealthPayload(report SaaSAdminNotificationHealthReport, platformAdminTenantID int) map[string]any {
	return map[string]any{
		"platformAdminTenantId": platformAdminTenantID,
		"generatedAt":           report.WindowEndAt,
		"windowStartAt":         report.WindowStartAt,
		"windowEndAt":           report.WindowEndAt,
		"filters": map[string]any{
			"tenantId":     report.Options.TenantID,
			"channel":      report.Options.Channel,
			"keyword":      report.Options.Keyword,
			"state":        report.Options.State,
			"windowHours":  report.Options.WindowHours,
			"staleMinutes": report.Options.StaleMinutes,
			"limit":        report.Options.Limit,
		},
		"thresholds": map[string]any{
			"rateSampleSize":      saasAdminNotificationHealthRateSampleSize,
			"warningSuccessRate":  saasAdminNotificationHealthWarningRate,
			"criticalSuccessRate": saasAdminNotificationHealthCriticalRate,
		},
		"summary":        saasAdminNotificationHealthSummaryPayload(report.Summary),
		"returnedCount":  len(report.Tenants),
		"tenants":        saasAdminNotificationHealthTenantPayloads(report.Tenants),
		"failureReasons": saasAdminNotificationHealthFailureReasonPayloads(report.FailureReasons),
	}
}

func saasAdminNotificationHealthSummaryPayload(summary SaaSAdminNotificationHealthSummary) map[string]any {
	return map[string]any{
		"tenantCount":            summary.TenantCount,
		"matchedTenantCount":     summary.MatchedTenantCount,
		"returnedTenantCount":    summary.ReturnedTenantCount,
		"healthyTenantCount":     summary.HealthyTenantCount,
		"warningTenantCount":     summary.WarningTenantCount,
		"criticalTenantCount":    summary.CriticalTenantCount,
		"noDataTenantCount":      summary.NoDataTenantCount,
		"policyConfiguredCount":  summary.PolicyConfiguredCount,
		"policyEnabledCount":     summary.PolicyEnabledCount,
		"notificationCount":      summary.NotificationCount,
		"attemptedCount":         summary.AttemptedCount,
		"deliveredCount":         summary.DeliveredCount,
		"pendingCount":           summary.PendingCount,
		"readyPendingCount":      summary.ReadyPendingCount,
		"deferredCount":          summary.DeferredCount,
		"stalePendingCount":      summary.StalePendingCount,
		"failedCount":            summary.FailedCount,
		"deadCount":              summary.DeadCount,
		"closedCount":            summary.ClosedCount,
		"suppressedCount":        summary.SuppressedCount,
		"totalAttempts":          summary.TotalAttempts,
		"deliverySuccessRate":    summary.DeliverySuccessRate,
		"averageDeliverySeconds": summary.AverageDeliverySeconds,
		"maxDeliverySeconds":     summary.MaxDeliverySeconds,
		"failureReasonCount":     summary.FailureReasonCount,
	}
}

func saasAdminNotificationHealthTenantPayloads(items []SaaSAdminNotificationHealthTenant) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, saasAdminNotificationHealthTenantPayload(item))
	}
	return result
}

func saasAdminNotificationHealthTenantPayload(item SaaSAdminNotificationHealthTenant) map[string]any {
	return map[string]any{
		"tenantId":               item.TenantID,
		"tenantName":             item.TenantName,
		"tenantStatus":           item.TenantStatus,
		"packageCode":            item.PackageCode,
		"packageName":            item.PackageName,
		"policyConfigured":       item.PolicyConfigured,
		"policyEnabled":          item.PolicyEnabled,
		"healthState":            item.HealthState,
		"notificationCount":      item.NotificationCount,
		"attemptedCount":         item.AttemptedCount,
		"deliverySuccessRate":    item.DeliverySuccessRate,
		"pendingCount":           item.PendingCount,
		"readyPendingCount":      item.ReadyPendingCount,
		"deferredCount":          item.DeferredCount,
		"stalePendingCount":      item.StalePendingCount,
		"failedCount":            item.FailedCount,
		"deliveredCount":         item.DeliveredCount,
		"deadCount":              item.DeadCount,
		"closedCount":            item.ClosedCount,
		"suppressedCount":        item.SuppressedCount,
		"totalAttempts":          item.TotalAttempts,
		"averageDeliverySeconds": item.AverageDeliverySeconds,
		"maxDeliverySeconds":     item.MaxDeliverySeconds,
		"oldestPendingAt":        item.OldestPendingAt,
		"lastDeliveredAt":        item.LastDeliveredAt,
		"lastFailureAt":          item.LastFailureAt,
		"latestNotificationAt":   item.LatestNotificationAt,
		"reasons":                append([]string{}, item.Reasons...),
		"suggestedAction":        item.SuggestedAction,
	}
}

func saasAdminNotificationHealthFailureReasonPayloads(items []SaaSAdminNotificationHealthFailureReason) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"reason":         item.Reason,
			"count":          item.Count,
			"tenantCount":    item.TenantCount,
			"lastOccurredAt": item.LastOccurredAt,
		})
	}
	return result
}

func writeSaaSAdminNotificationHealthCSV(writer *csv.Writer, report SaaSAdminNotificationHealthReport) {
	_ = writer.Write([]string{
		"windowStartAt", "windowEndAt", "windowHours", "staleMinutes", "tenantId", "tenantName", "tenantStatus", "packageCode", "packageName", "healthState",
		"notificationCount", "attemptedCount", "deliverySuccessRate", "deliveredCount", "pendingCount", "readyPendingCount", "deferredCount", "stalePendingCount", "failedCount", "deadCount", "closedCount", "suppressedCount",
		"totalAttempts", "averageDeliverySeconds", "maxDeliverySeconds", "policyConfigured", "policyEnabled", "oldestPendingAt", "lastDeliveredAt", "lastFailureAt", "latestNotificationAt", "reasons", "suggestedAction",
	})
	for _, item := range report.Tenants {
		_ = writer.Write([]string{
			report.WindowStartAt,
			report.WindowEndAt,
			strconv.Itoa(report.Options.WindowHours),
			strconv.Itoa(report.Options.StaleMinutes),
			strconv.Itoa(item.TenantID),
			item.TenantName,
			strconv.Itoa(item.TenantStatus),
			item.PackageCode,
			item.PackageName,
			item.HealthState,
			strconv.Itoa(item.NotificationCount),
			strconv.Itoa(item.AttemptedCount),
			strconv.FormatFloat(item.DeliverySuccessRate, 'f', 4, 64),
			strconv.Itoa(item.DeliveredCount),
			strconv.Itoa(item.PendingCount),
			strconv.Itoa(item.ReadyPendingCount),
			strconv.Itoa(item.DeferredCount),
			strconv.Itoa(item.StalePendingCount),
			strconv.Itoa(item.FailedCount),
			strconv.Itoa(item.DeadCount),
			strconv.Itoa(item.ClosedCount),
			strconv.Itoa(item.SuppressedCount),
			strconv.FormatInt(item.TotalAttempts, 10),
			strconv.FormatFloat(item.AverageDeliverySeconds, 'f', 2, 64),
			strconv.FormatInt(item.MaxDeliverySeconds, 10),
			strconv.FormatBool(item.PolicyConfigured),
			strconv.FormatBool(item.PolicyEnabled),
			item.OldestPendingAt,
			item.LastDeliveredAt,
			item.LastFailureAt,
			item.LatestNotificationAt,
			strings.Join(item.Reasons, "；"),
			item.SuggestedAction,
		})
	}
}
