package dashboard

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (h *ChannelCodeHandler) WorkspaceStatistics(w http.ResponseWriter, r *http.Request) {
	page, ok := h.loadWorkspaceStatistics(w, r)
	if !ok {
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"summary": channelCodeStatisticsSummaryPayload(page.Summary),
		"page": map[string]any{
			"perPage":   page.PerPage,
			"total":     page.Total,
			"totalPage": page.TotalPage,
		},
		"list": channelCodeStatisticsRowsPayload(page.Rows),
	})
}

func (h *ChannelCodeHandler) WorkspaceStatisticsIndex(w http.ResponseWriter, r *http.Request) {
	h.WorkspaceStatistics(w, r)
}

func (h *ChannelCodeHandler) ExportWorkspaceStatistics(w http.ResponseWriter, r *http.Request) {
	page, ok := h.loadWorkspaceStatistics(w, r)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="channel-codes-statistics.csv"`)
	writer := csv.NewWriter(w)
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})
	_ = writer.Write([]string{"活码名称", "新增好友次数", "流失好友次数", "新增好友数", "留存好友数", "状态"})
	for _, row := range page.Rows {
		status := "已接入"
		if !row.Available {
			status = "暂无可验证数据"
		}
		_ = writer.Write([]string{
			csvSafeCell(row.Name), strconv.Itoa(row.AddedAttempts), strconv.Itoa(row.LostAttempts),
			strconv.Itoa(row.AddedCustomers), strconv.Itoa(row.RetainedCustomers), csvSafeCell(status),
		})
	}
	writer.Flush()
}

func (h *ChannelCodeHandler) loadWorkspaceStatistics(w http.ResponseWriter, r *http.Request) (ChannelCodeStatisticsPage, bool) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return ChannelCodeStatisticsPage{}, false
	}
	userID, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return ChannelCodeStatisticsPage{}, false
	}
	if len(principalScope.CorpIDs) != 1 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "请先选择企业", nil)
		return ChannelCodeStatisticsPage{}, false
	}
	if _, err := h.authorizeAccess(r.Context(), r, userID, principalScope); err != nil {
		writeAccessError(w, err)
		return ChannelCodeStatisticsPage{}, false
	}
	store, ok := h.store.(ChannelCodeStatisticsStore)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "渠道码统计能力未接入", nil)
		return ChannelCodeStatisticsPage{}, false
	}
	filter, err := channelCodeStatisticsFilterFromRequest(r, principalScope.CorpIDs[0])
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return ChannelCodeStatisticsPage{}, false
	}
	page, err := store.ChannelCodeWorkspaceStatistics(r.Context(), filter)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "渠道码统计查询失败", nil)
		return ChannelCodeStatisticsPage{}, false
	}
	return page, true
}

func channelCodeStatisticsFilterFromRequest(r *http.Request, corpID int) (ChannelCodeStatisticsFilter, error) {
	startDate := strings.TrimSpace(r.URL.Query().Get("startDate"))
	endDate := strings.TrimSpace(r.URL.Query().Get("endDate"))
	now := time.Now()
	if startDate == "" {
		startDate = now.AddDate(0, 0, -29).Format("2006-01-02")
	}
	if endDate == "" {
		endDate = now.Format("2006-01-02")
	}
	start, startErr := time.ParseInLocation("2006-01-02", startDate, time.Local)
	end, endErr := time.ParseInLocation("2006-01-02", endDate, time.Local)
	if startErr != nil || endErr != nil || start.After(end) {
		return ChannelCodeStatisticsFilter{}, fmt.Errorf("日期范围无效")
	}
	return ChannelCodeStatisticsFilter{
		CorpIDs:   []int{corpID},
		GroupID:   positiveQueryInt(r, "groupId", 0),
		Name:      strings.TrimSpace(r.URL.Query().Get("name")),
		StartDate: startDate,
		EndDate:   endDate,
		Page:      positiveQueryInt(r, "page", 1),
		PerPage:   positiveQueryInt(r, "perPage", 20),
	}, nil
}

func channelCodeStatisticsSummaryPayload(summary ChannelCodeStatisticsSummary) map[string]any {
	return map[string]any{
		"addedAttempts":     summary.AddedAttempts,
		"lostAttempts":      summary.LostAttempts,
		"addedCustomers":    summary.AddedCustomers,
		"retainedCustomers": summary.RetainedCustomers,
		"codeCount":         summary.CodeCount,
		"available":         summary.Available,
		"asOf":              summary.AsOf,
		"timezone":          summary.Timezone,
		"definition":        summary.Definition,
	}
}

func channelCodeStatisticsRowsPayload(rows []ChannelCodeStatisticsRow) []map[string]any {
	result := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		result = append(result, map[string]any{
			"id":                row.ID,
			"name":              row.Name,
			"addedAttempts":     row.AddedAttempts,
			"lostAttempts":      row.LostAttempts,
			"addedCustomers":    row.AddedCustomers,
			"retainedCustomers": row.RetainedCustomers,
			"available":         row.Available,
		})
	}
	return result
}

func csvSafeCell(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	if strings.ContainsRune("=+-@", []rune(value)[0]) {
		return "'" + value
	}
	return value
}
