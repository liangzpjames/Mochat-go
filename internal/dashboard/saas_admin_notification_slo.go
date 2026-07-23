package dashboard

import (
	"encoding/csv"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	SaaSAdminNotificationSLOStateMet      = "met"
	SaaSAdminNotificationSLOStateBreached = "breached"
	SaaSAdminNotificationSLOStateNoData   = "no_data"

	saasAdminNotificationSLODefaultDays                 = 7
	saasAdminNotificationSLOMaxDays                     = 90
	saasAdminNotificationSLODefaultSuccessRateTarget    = 0.95
	saasAdminNotificationSLODefaultLatencySecondsTarget = 300
	saasAdminNotificationSLOMaxLatencySecondsTarget     = 86400
	saasAdminNotificationSLODefaultLatencyRateTarget    = 0.95
	saasAdminNotificationSLOMinRateTarget               = 0.5
)

type SaaSAdminNotificationSLOOptions struct {
	TenantID             int
	Channel              string
	Keyword              string
	Days                 int
	SuccessRateTarget    float64
	LatencySecondsTarget int
	LatencyRateTarget    float64
	Limit                int
}

type SaaSAdminNotificationSLOSource struct {
	WindowStartDate string
	WindowEndDate   string
	Days            []SaaSAdminNotificationSLODaySnapshot
	Tenants         []SaaSAdminNotificationSLOTenantSnapshot
}

type SaaSAdminNotificationSLOMetricsSnapshot struct {
	NotificationCount      int
	PendingCount           int
	DeliveredCount         int
	FailedCount            int
	DeadCount              int
	ClosedCount            int
	SuppressedCount        int
	DeliveredWithinTarget  int
	TotalAttempts          int64
	AverageDeliverySeconds float64
	MaxDeliverySeconds     int64
}

type SaaSAdminNotificationSLODaySnapshot struct {
	Day         string
	TenantCount int
	SaaSAdminNotificationSLOMetricsSnapshot
}

type SaaSAdminNotificationSLOTenantSnapshot struct {
	TenantID     int
	TenantName   string
	TenantStatus int
	PackageCode  string
	PackageName  string
	SaaSAdminNotificationSLOMetricsSnapshot
}

type SaaSAdminNotificationSLOReport struct {
	Options         SaaSAdminNotificationSLOOptions
	GeneratedAt     string
	WindowStartDate string
	WindowEndDate   string
	Summary         SaaSAdminNotificationSLOSummary
	Days            []SaaSAdminNotificationSLODay
	Tenants         []SaaSAdminNotificationSLOTenant
}

type SaaSAdminNotificationSLOSummary struct {
	SaaSAdminNotificationSLOMeasurement
	DayCount            int
	MetDayCount         int
	BreachedDayCount    int
	NoDataDayCount      int
	TenantCount         int
	ReturnedTenantCount int
	MetTenantCount      int
	BreachedTenantCount int
	NoDataTenantCount   int
}

type SaaSAdminNotificationSLOMeasurement struct {
	NotificationCount      int
	AttemptedCount         int
	PendingCount           int
	DeliveredCount         int
	FailedCount            int
	DeadCount              int
	ClosedCount            int
	SuppressedCount        int
	DeliveredWithinTarget  int
	TotalAttempts          int64
	DeliverySuccessRate    float64
	LatencyAttainmentRate  float64
	AverageDeliverySeconds float64
	MaxDeliverySeconds     int64
	SuccessObjectiveMet    bool
	LatencyObjectiveMet    bool
	SLOState               string
}

type SaaSAdminNotificationSLODay struct {
	Day         string
	TenantCount int
	SaaSAdminNotificationSLOMeasurement
}

type SaaSAdminNotificationSLOTenant struct {
	TenantID     int
	TenantName   string
	TenantStatus int
	PackageCode  string
	PackageName  string
	SaaSAdminNotificationSLOMeasurement
}

func (h *SaaSAdminHandler) NotificationSLO(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, ok := h.resolvePlatformSuperAdmin(w, r); !ok {
		return
	}
	options, ok := h.notificationSLOOptions(w, r, 50, saasAdminListMaxLimit)
	if !ok {
		return
	}
	source, err := h.store.SaaSAdminNotificationSLO(r.Context(), options)
	if err != nil {
		writeSaaSAdminError(w, err)
		return
	}
	report := saasAdminBuildNotificationSLOReport(source, options, time.Now())
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", saasAdminNotificationSLOPayload(report, h.platformAdminTenantID))
}

func (h *SaaSAdminHandler) notificationSLOOptions(w http.ResponseWriter, r *http.Request, defaultLimit, maxLimit int) (SaaSAdminNotificationSLOOptions, bool) {
	options := SaaSAdminNotificationSLOOptions{
		TenantID: saasAdminQueryInt(r, "tenantId", 0),
		Channel:  strings.ToLower(strings.TrimSpace(r.URL.Query().Get("channel"))),
		Days:     saasAdminNotificationSLODefaultDays,
		Limit:    defaultLimit,
	}
	var ok bool
	if options.Keyword, ok = saasAdminQueryString(w, "keyword", 100, r.URL.Query().Get("keyword")); !ok {
		return SaaSAdminNotificationSLOOptions{}, false
	}
	if options.Channel == "" {
		options.Channel = SaaSAlertNotificationChannelWebhook
	}
	if options.Channel != SaaSAlertNotificationChannelWebhook {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "channel 目前仅支持 webhook", nil)
		return SaaSAdminNotificationSLOOptions{}, false
	}
	if options.Days, ok = saasAdminNotificationSLOQueryInt(w, r, "days", saasAdminNotificationSLODefaultDays, 1, saasAdminNotificationSLOMaxDays); !ok {
		return SaaSAdminNotificationSLOOptions{}, false
	}
	if options.SuccessRateTarget, ok = saasAdminNotificationSLOQueryRate(w, r, "successRateTarget", saasAdminNotificationSLODefaultSuccessRateTarget); !ok {
		return SaaSAdminNotificationSLOOptions{}, false
	}
	if options.LatencySecondsTarget, ok = saasAdminNotificationSLOQueryInt(w, r, "latencySecondsTarget", saasAdminNotificationSLODefaultLatencySecondsTarget, 1, saasAdminNotificationSLOMaxLatencySecondsTarget); !ok {
		return SaaSAdminNotificationSLOOptions{}, false
	}
	if options.LatencyRateTarget, ok = saasAdminNotificationSLOQueryRate(w, r, "latencyRateTarget", saasAdminNotificationSLODefaultLatencyRateTarget); !ok {
		return SaaSAdminNotificationSLOOptions{}, false
	}
	if options.Limit, ok = saasAdminNotificationSLOQueryInt(w, r, "limit", defaultLimit, 1, maxLimit); !ok {
		return SaaSAdminNotificationSLOOptions{}, false
	}
	return options, true
}

func saasAdminNotificationSLOQueryInt(w http.ResponseWriter, r *http.Request, field string, fallback, minimum, maximum int) (int, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get(field))
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < minimum || value > maximum {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, field+" 必须在 "+strconv.Itoa(minimum)+" 到 "+strconv.Itoa(maximum)+" 之间", nil)
		return 0, false
	}
	return value, true
}

func saasAdminNotificationSLOQueryRate(w http.ResponseWriter, r *http.Request, field string, fallback float64) (float64, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get(field))
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < saasAdminNotificationSLOMinRateTarget || value > 1 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, field+" 必须在 0.5 到 1 之间", nil)
		return 0, false
	}
	return value, true
}

func saasAdminBuildNotificationSLOReport(source SaaSAdminNotificationSLOSource, options SaaSAdminNotificationSLOOptions, now time.Time) SaaSAdminNotificationSLOReport {
	options = saasAdminNormalizeNotificationSLOOptions(options)
	location := now.Location()
	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	start := end.AddDate(0, 0, -options.Days+1)
	if sourceStart, startErr := time.ParseInLocation("2006-01-02", strings.TrimSpace(source.WindowStartDate), location); startErr == nil {
		if sourceEnd, endErr := time.ParseInLocation("2006-01-02", strings.TrimSpace(source.WindowEndDate), location); endErr == nil && sourceStart.AddDate(0, 0, options.Days-1).Equal(sourceEnd) {
			start = sourceStart
			end = sourceEnd
		}
	}
	report := SaaSAdminNotificationSLOReport{
		Options:         options,
		GeneratedAt:     now.Format("2006-01-02 15:04:05"),
		WindowStartDate: start.Format("2006-01-02"),
		WindowEndDate:   end.Format("2006-01-02"),
		Days:            make([]SaaSAdminNotificationSLODay, 0, options.Days),
		Tenants:         make([]SaaSAdminNotificationSLOTenant, 0, len(source.Tenants)),
	}

	daySnapshots := make(map[string]SaaSAdminNotificationSLODaySnapshot, len(source.Days))
	for _, snapshot := range source.Days {
		day, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(snapshot.Day), location)
		if err != nil || day.Before(start) || day.After(end) {
			continue
		}
		key := day.Format("2006-01-02")
		current := daySnapshots[key]
		current.Day = key
		current.TenantCount += snapshot.TenantCount
		current.SaaSAdminNotificationSLOMetricsSnapshot = saasAdminAddNotificationSLOSnapshot(current.SaaSAdminNotificationSLOMetricsSnapshot, snapshot.SaaSAdminNotificationSLOMetricsSnapshot)
		daySnapshots[key] = current
	}

	weightedDeliverySeconds := 0.0
	for offset := 0; offset < options.Days; offset++ {
		day := start.AddDate(0, 0, offset).Format("2006-01-02")
		snapshot := daySnapshots[day]
		item := SaaSAdminNotificationSLODay{
			Day:                                 day,
			TenantCount:                         snapshot.TenantCount,
			SaaSAdminNotificationSLOMeasurement: saasAdminNotificationSLOMeasurement(snapshot.SaaSAdminNotificationSLOMetricsSnapshot, options),
		}
		report.Days = append(report.Days, item)
		report.Summary.DayCount++
		saasAdminCountNotificationSLOState(item.SLOState, &report.Summary.MetDayCount, &report.Summary.BreachedDayCount, &report.Summary.NoDataDayCount)
		saasAdminAddNotificationSLOMeasurement(&report.Summary.SaaSAdminNotificationSLOMeasurement, item.SaaSAdminNotificationSLOMeasurement)
		weightedDeliverySeconds += snapshot.AverageDeliverySeconds * float64(snapshot.DeliveredCount)
	}
	report.Summary.DeliverySuccessRate = saasAdminNotificationSLORate(report.Summary.DeliveredCount, report.Summary.AttemptedCount)
	report.Summary.LatencyAttainmentRate = saasAdminNotificationSLORate(report.Summary.DeliveredWithinTarget, report.Summary.DeliveredCount)
	if report.Summary.DeliveredCount > 0 {
		report.Summary.AverageDeliverySeconds = roundFloat(weightedDeliverySeconds/float64(report.Summary.DeliveredCount), 2)
	}
	saasAdminApplyNotificationSLOObjectives(&report.Summary.SaaSAdminNotificationSLOMeasurement, options)

	for _, snapshot := range source.Tenants {
		item := SaaSAdminNotificationSLOTenant{
			TenantID:                            snapshot.TenantID,
			TenantName:                          snapshot.TenantName,
			TenantStatus:                        snapshot.TenantStatus,
			PackageCode:                         snapshot.PackageCode,
			PackageName:                         snapshot.PackageName,
			SaaSAdminNotificationSLOMeasurement: saasAdminNotificationSLOMeasurement(snapshot.SaaSAdminNotificationSLOMetricsSnapshot, options),
		}
		report.Tenants = append(report.Tenants, item)
		report.Summary.TenantCount++
		saasAdminCountNotificationSLOState(item.SLOState, &report.Summary.MetTenantCount, &report.Summary.BreachedTenantCount, &report.Summary.NoDataTenantCount)
	}
	sort.SliceStable(report.Tenants, func(i, j int) bool {
		left, right := report.Tenants[i], report.Tenants[j]
		if saasAdminNotificationSLOStateRank(left.SLOState) != saasAdminNotificationSLOStateRank(right.SLOState) {
			return saasAdminNotificationSLOStateRank(left.SLOState) < saasAdminNotificationSLOStateRank(right.SLOState)
		}
		if left.DeliverySuccessRate != right.DeliverySuccessRate {
			return left.DeliverySuccessRate < right.DeliverySuccessRate
		}
		if left.LatencyAttainmentRate != right.LatencyAttainmentRate {
			return left.LatencyAttainmentRate < right.LatencyAttainmentRate
		}
		if left.AttemptedCount != right.AttemptedCount {
			return left.AttemptedCount > right.AttemptedCount
		}
		return left.TenantID < right.TenantID
	})
	if len(report.Tenants) > options.Limit {
		report.Tenants = report.Tenants[:options.Limit]
	}
	report.Summary.ReturnedTenantCount = len(report.Tenants)
	return report
}

func saasAdminNormalizeNotificationSLOOptions(options SaaSAdminNotificationSLOOptions) SaaSAdminNotificationSLOOptions {
	if strings.TrimSpace(options.Channel) == "" {
		options.Channel = SaaSAlertNotificationChannelWebhook
	}
	if options.Days <= 0 || options.Days > saasAdminNotificationSLOMaxDays {
		options.Days = saasAdminNotificationSLODefaultDays
	}
	if options.SuccessRateTarget < saasAdminNotificationSLOMinRateTarget || options.SuccessRateTarget > 1 {
		options.SuccessRateTarget = saasAdminNotificationSLODefaultSuccessRateTarget
	}
	if options.LatencySecondsTarget <= 0 || options.LatencySecondsTarget > saasAdminNotificationSLOMaxLatencySecondsTarget {
		options.LatencySecondsTarget = saasAdminNotificationSLODefaultLatencySecondsTarget
	}
	if options.LatencyRateTarget < saasAdminNotificationSLOMinRateTarget || options.LatencyRateTarget > 1 {
		options.LatencyRateTarget = saasAdminNotificationSLODefaultLatencyRateTarget
	}
	if options.Limit <= 0 {
		options.Limit = 50
	}
	return options
}

func saasAdminNotificationSLOMeasurement(snapshot SaaSAdminNotificationSLOMetricsSnapshot, options SaaSAdminNotificationSLOOptions) SaaSAdminNotificationSLOMeasurement {
	measurement := SaaSAdminNotificationSLOMeasurement{
		NotificationCount:      snapshot.NotificationCount,
		AttemptedCount:         snapshot.DeliveredCount + snapshot.FailedCount + snapshot.DeadCount + snapshot.ClosedCount,
		PendingCount:           snapshot.PendingCount,
		DeliveredCount:         snapshot.DeliveredCount,
		FailedCount:            snapshot.FailedCount,
		DeadCount:              snapshot.DeadCount,
		ClosedCount:            snapshot.ClosedCount,
		SuppressedCount:        snapshot.SuppressedCount,
		DeliveredWithinTarget:  snapshot.DeliveredWithinTarget,
		TotalAttempts:          snapshot.TotalAttempts,
		DeliverySuccessRate:    saasAdminNotificationSLORate(snapshot.DeliveredCount, snapshot.DeliveredCount+snapshot.FailedCount+snapshot.DeadCount+snapshot.ClosedCount),
		LatencyAttainmentRate:  saasAdminNotificationSLORate(snapshot.DeliveredWithinTarget, snapshot.DeliveredCount),
		AverageDeliverySeconds: roundFloat(snapshot.AverageDeliverySeconds, 2),
		MaxDeliverySeconds:     snapshot.MaxDeliverySeconds,
	}
	saasAdminApplyNotificationSLOObjectives(&measurement, options)
	return measurement
}

func saasAdminApplyNotificationSLOObjectives(measurement *SaaSAdminNotificationSLOMeasurement, options SaaSAdminNotificationSLOOptions) {
	measurement.SuccessObjectiveMet = measurement.AttemptedCount > 0 && measurement.DeliverySuccessRate >= options.SuccessRateTarget
	measurement.LatencyObjectiveMet = measurement.DeliveredCount > 0 && measurement.LatencyAttainmentRate >= options.LatencyRateTarget
	switch {
	case measurement.AttemptedCount == 0:
		measurement.SLOState = SaaSAdminNotificationSLOStateNoData
	case measurement.SuccessObjectiveMet && measurement.LatencyObjectiveMet:
		measurement.SLOState = SaaSAdminNotificationSLOStateMet
	default:
		measurement.SLOState = SaaSAdminNotificationSLOStateBreached
	}
}

func saasAdminAddNotificationSLOSnapshot(left, right SaaSAdminNotificationSLOMetricsSnapshot) SaaSAdminNotificationSLOMetricsSnapshot {
	leftDelivered := left.DeliveredCount
	left.NotificationCount += right.NotificationCount
	left.PendingCount += right.PendingCount
	left.DeliveredCount += right.DeliveredCount
	left.FailedCount += right.FailedCount
	left.DeadCount += right.DeadCount
	left.ClosedCount += right.ClosedCount
	left.SuppressedCount += right.SuppressedCount
	left.DeliveredWithinTarget += right.DeliveredWithinTarget
	left.TotalAttempts += right.TotalAttempts
	if left.DeliveredCount > 0 {
		left.AverageDeliverySeconds = (left.AverageDeliverySeconds*float64(leftDelivered) + right.AverageDeliverySeconds*float64(right.DeliveredCount)) / float64(left.DeliveredCount)
	}
	if right.MaxDeliverySeconds > left.MaxDeliverySeconds {
		left.MaxDeliverySeconds = right.MaxDeliverySeconds
	}
	return left
}

func saasAdminAddNotificationSLOMeasurement(target *SaaSAdminNotificationSLOMeasurement, item SaaSAdminNotificationSLOMeasurement) {
	target.NotificationCount += item.NotificationCount
	target.AttemptedCount += item.AttemptedCount
	target.PendingCount += item.PendingCount
	target.DeliveredCount += item.DeliveredCount
	target.FailedCount += item.FailedCount
	target.DeadCount += item.DeadCount
	target.ClosedCount += item.ClosedCount
	target.SuppressedCount += item.SuppressedCount
	target.DeliveredWithinTarget += item.DeliveredWithinTarget
	target.TotalAttempts += item.TotalAttempts
	if item.MaxDeliverySeconds > target.MaxDeliverySeconds {
		target.MaxDeliverySeconds = item.MaxDeliverySeconds
	}
}

func saasAdminCountNotificationSLOState(state string, met, breached, noData *int) {
	switch state {
	case SaaSAdminNotificationSLOStateMet:
		*met++
	case SaaSAdminNotificationSLOStateBreached:
		*breached++
	default:
		*noData++
	}
}

func saasAdminNotificationSLORate(numerator, denominator int) float64 {
	if denominator <= 0 {
		return 0
	}
	return roundFloat(float64(numerator)/float64(denominator), 4)
}

func saasAdminNotificationSLOStateRank(state string) int {
	switch state {
	case SaaSAdminNotificationSLOStateBreached:
		return 0
	case SaaSAdminNotificationSLOStateMet:
		return 1
	default:
		return 2
	}
}

func saasAdminNotificationSLOPayload(report SaaSAdminNotificationSLOReport, platformAdminTenantID int) map[string]any {
	return map[string]any{
		"platformAdminTenantId": platformAdminTenantID,
		"generatedAt":           report.GeneratedAt,
		"window": map[string]any{
			"startDate": report.WindowStartDate,
			"endDate":   report.WindowEndDate,
			"days":      report.Options.Days,
		},
		"filters": map[string]any{
			"tenantId": report.Options.TenantID,
			"channel":  report.Options.Channel,
			"keyword":  report.Options.Keyword,
			"limit":    report.Options.Limit,
		},
		"objectives": map[string]any{
			"successRateTarget":    report.Options.SuccessRateTarget,
			"latencySecondsTarget": report.Options.LatencySecondsTarget,
			"latencyRateTarget":    report.Options.LatencyRateTarget,
		},
		"measurementBasis": map[string]any{
			"cohort":            "created_at",
			"windowClock":       "mysql_session",
			"attemptedStatuses": []string{SaaSAlertNotificationStatusDelivered, SaaSAlertNotificationStatusFailed, SaaSAlertNotificationStatusDead, SaaSAlertNotificationStatusClosed},
			"excludedStatuses":  []string{SaaSAlertNotificationStatusPending, SaaSAlertNotificationStatusSuppressed},
		},
		"summary":       saasAdminNotificationSLOSummaryPayload(report.Summary),
		"returnedCount": len(report.Tenants),
		"days":          saasAdminNotificationSLODayPayloads(report.Days),
		"tenants":       saasAdminNotificationSLOTenantPayloads(report.Tenants),
	}
}

func saasAdminNotificationSLOSummaryPayload(summary SaaSAdminNotificationSLOSummary) map[string]any {
	payload := saasAdminNotificationSLOMeasurementPayload(summary.SaaSAdminNotificationSLOMeasurement)
	payload["dayCount"] = summary.DayCount
	payload["metDayCount"] = summary.MetDayCount
	payload["breachedDayCount"] = summary.BreachedDayCount
	payload["noDataDayCount"] = summary.NoDataDayCount
	payload["tenantCount"] = summary.TenantCount
	payload["returnedTenantCount"] = summary.ReturnedTenantCount
	payload["metTenantCount"] = summary.MetTenantCount
	payload["breachedTenantCount"] = summary.BreachedTenantCount
	payload["noDataTenantCount"] = summary.NoDataTenantCount
	return payload
}

func saasAdminNotificationSLODayPayloads(items []SaaSAdminNotificationSLODay) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payload := saasAdminNotificationSLOMeasurementPayload(item.SaaSAdminNotificationSLOMeasurement)
		payload["day"] = item.Day
		payload["tenantCount"] = item.TenantCount
		result = append(result, payload)
	}
	return result
}

func saasAdminNotificationSLOTenantPayloads(items []SaaSAdminNotificationSLOTenant) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payload := saasAdminNotificationSLOMeasurementPayload(item.SaaSAdminNotificationSLOMeasurement)
		payload["tenantId"] = item.TenantID
		payload["tenantName"] = item.TenantName
		payload["tenantStatus"] = item.TenantStatus
		payload["packageCode"] = item.PackageCode
		payload["packageName"] = item.PackageName
		result = append(result, payload)
	}
	return result
}

func saasAdminNotificationSLOMeasurementPayload(item SaaSAdminNotificationSLOMeasurement) map[string]any {
	return map[string]any{
		"notificationCount":      item.NotificationCount,
		"attemptedCount":         item.AttemptedCount,
		"pendingCount":           item.PendingCount,
		"deliveredCount":         item.DeliveredCount,
		"failedCount":            item.FailedCount,
		"deadCount":              item.DeadCount,
		"closedCount":            item.ClosedCount,
		"suppressedCount":        item.SuppressedCount,
		"deliveredWithinTarget":  item.DeliveredWithinTarget,
		"totalAttempts":          item.TotalAttempts,
		"deliverySuccessRate":    item.DeliverySuccessRate,
		"latencyAttainmentRate":  item.LatencyAttainmentRate,
		"averageDeliverySeconds": item.AverageDeliverySeconds,
		"maxDeliverySeconds":     item.MaxDeliverySeconds,
		"successObjectiveMet":    item.SuccessObjectiveMet,
		"latencyObjectiveMet":    item.LatencyObjectiveMet,
		"sloState":               item.SLOState,
	}
}

func writeSaaSAdminNotificationSLOCSV(writer *csv.Writer, report SaaSAdminNotificationSLOReport) {
	_ = writer.Write([]string{
		"section", "windowStartDate", "windowEndDate", "days", "successRateTarget", "latencySecondsTarget", "latencyRateTarget",
		"day", "tenantId", "tenantName", "tenantStatus", "packageCode", "packageName", "tenantCount", "sloState",
		"notificationCount", "attemptedCount", "deliverySuccessRate", "deliveredCount", "deliveredWithinTarget", "latencyAttainmentRate",
		"pendingCount", "failedCount", "deadCount", "closedCount", "suppressedCount", "totalAttempts", "averageDeliverySeconds", "maxDeliverySeconds",
	})
	writeSaaSAdminNotificationSLOCSVRow(writer, report, "summary", "", 0, "", 0, "", "", report.Summary.TenantCount, report.Summary.SaaSAdminNotificationSLOMeasurement)
	for _, item := range report.Days {
		writeSaaSAdminNotificationSLOCSVRow(writer, report, "day", item.Day, 0, "", 0, "", "", item.TenantCount, item.SaaSAdminNotificationSLOMeasurement)
	}
	for _, item := range report.Tenants {
		writeSaaSAdminNotificationSLOCSVRow(writer, report, "tenant", "", item.TenantID, item.TenantName, item.TenantStatus, item.PackageCode, item.PackageName, 0, item.SaaSAdminNotificationSLOMeasurement)
	}
}

func writeSaaSAdminNotificationSLOCSVRow(writer *csv.Writer, report SaaSAdminNotificationSLOReport, section, day string, tenantID int, tenantName string, tenantStatus int, packageCode, packageName string, tenantCount int, item SaaSAdminNotificationSLOMeasurement) {
	_ = writer.Write([]string{
		section,
		report.WindowStartDate,
		report.WindowEndDate,
		strconv.Itoa(report.Options.Days),
		strconv.FormatFloat(report.Options.SuccessRateTarget, 'f', 4, 64),
		strconv.Itoa(report.Options.LatencySecondsTarget),
		strconv.FormatFloat(report.Options.LatencyRateTarget, 'f', 4, 64),
		day,
		saasAdminOptionalCSVInt(tenantID),
		tenantName,
		saasAdminOptionalCSVInt(tenantStatus),
		packageCode,
		packageName,
		saasAdminOptionalCSVInt(tenantCount),
		item.SLOState,
		strconv.Itoa(item.NotificationCount),
		strconv.Itoa(item.AttemptedCount),
		strconv.FormatFloat(item.DeliverySuccessRate, 'f', 4, 64),
		strconv.Itoa(item.DeliveredCount),
		strconv.Itoa(item.DeliveredWithinTarget),
		strconv.FormatFloat(item.LatencyAttainmentRate, 'f', 4, 64),
		strconv.Itoa(item.PendingCount),
		strconv.Itoa(item.FailedCount),
		strconv.Itoa(item.DeadCount),
		strconv.Itoa(item.ClosedCount),
		strconv.Itoa(item.SuppressedCount),
		strconv.FormatInt(item.TotalAttempts, 10),
		strconv.FormatFloat(item.AverageDeliverySeconds, 'f', 2, 64),
		strconv.FormatInt(item.MaxDeliverySeconds, 10),
	})
}

func saasAdminOptionalCSVInt(value int) string {
	if value == 0 {
		return ""
	}
	return strconv.Itoa(value)
}
