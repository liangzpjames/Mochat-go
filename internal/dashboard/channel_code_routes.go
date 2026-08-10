package dashboard

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (h *ChannelCodeHandler) Index(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, user, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	access, err := h.authorizeAccess(r.Context(), r, userID, loginInfo)
	if err != nil {
		writeAccessError(w, err)
		return
	}

	filter := ChannelCodeListFilter{
		CorpIDs: append([]int{}, loginInfo.CorpIDs...),
		Name:    strings.TrimSpace(r.URL.Query().Get("name")),
		Page:    positiveQueryInt(r, "page", 1),
		PerPage: positiveQueryInt(r, "perPage", 20),
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("type")); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil {
			filter.Type = &value
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("groupId")); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil {
			filter.GroupID = &value
		}
	}
	if dashboardAccess, hasDashboardAccess := DashboardAccessFromContext(r.Context()); hasDashboardAccess && dashboardAccess.ScopeRequired && dashboardAccess.Scope != DataScopeTenant {
		ids, err := h.store.ChannelCodeBusinessIDsByOperators(r.Context(), dashboardAccess.AllowedEmployeeIDs)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		filter.RestrictBusinessIDs = true
		filter.BusinessIDs = ids
	} else if access.DataPermission != DataPermissionAll {
		ids, err := h.store.ChannelCodeBusinessIDsByOperators(r.Context(), access.DeptEmployeeIDs)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		filter.RestrictBusinessIDs = true
		filter.BusinessIDs = ids
	}

	page, err := h.store.ChannelCodePage(r.Context(), filter)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if len(page.Items) == 0 {
		writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
			"page": map[string]any{"perPage": 20, "total": 0, "totalPage": 0},
			"list": []any{},
		})
		return
	}

	userPayload := workContactIndexUserPayload(user, loginInfo, access)
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, map[string]any{
			"id":            item.ID,
			"channelCodeId": item.ID,
			"groupId":       item.GroupID,
			"groupName":     item.GroupName,
			"name":          item.Name,
			"qrcodeUrl":     h.fileFullURL(item.QRCodeURL),
			"autoAddFriend": channelCodeAutoAddFriendValue(item.AutoAddFriend),
			"tags":          item.Tags,
			"type":          channelCodeTypeText(item.Type),
			"contactNum":    item.ContactNum,
			"user":          userPayload,
		})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"page": map[string]any{
			"perPage":   page.PerPage,
			"total":     page.Total,
			"totalPage": page.TotalPage,
		},
		"list": list,
	})
}

func (h *ChannelCodeHandler) Show(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if _, err := h.authorizeAccess(r.Context(), r, userID, loginInfo); err != nil {
		writeAccessError(w, err)
		return
	}
	rawID := strings.TrimSpace(r.URL.Query().Get("channelCodeId"))
	if rawID == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "渠道码id必传", nil)
		return
	}
	channelCodeID, err := strconv.Atoi(rawID)
	if err != nil {
		channelCodeID = 0
	}
	if len(loginInfo.CorpIDs) == 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "请先选择企业", nil)
		return
	}
	info, found, err := h.store.ChannelCodeShowByID(r.Context(), channelCodeID, loginInfo.CorpIDs[0])
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusOK, 200, "success", []any{})
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"baseInfo": map[string]any{
			"groupId":       info.GroupID,
			"groupName":     info.GroupName,
			"name":          info.Name,
			"autoAddFriend": info.AutoAddFriend,
			"tags":          info.TagGroups,
			"selectedTags":  info.SelectedTags,
		},
		"drainageEmployee": info.DrainageEmployee,
		"welcomeMessage":   info.WelcomeMessage,
	})
}

func (h *ChannelCodeHandler) Contact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if _, err := h.authorizeAccess(r.Context(), r, userID, loginInfo); err != nil {
		writeAccessError(w, err)
		return
	}
	rawID := strings.TrimSpace(r.URL.Query().Get("channelCodeId"))
	if rawID == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "渠道码id必传", nil)
		return
	}
	channelCodeID, err := strconv.Atoi(rawID)
	if err != nil {
		channelCodeID = 0
	}
	page, err := h.store.ChannelCodeContactPage(r.Context(), ChannelCodeContactFilter{
		ChannelCodeID: channelCodeID,
		Page:          positiveQueryInt(r, "page", 1),
		PerPage:       positiveQueryInt(r, "perPage", 15),
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if len(page.Items) == 0 {
		writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
			"page": map[string]any{"perPage": 15, "total": 0, "totalPage": 0},
			"list": []any{},
		})
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, map[string]any{
			"contactId":  item.ContactID,
			"employeeId": item.EmployeeID,
			"createTime": item.CreateTime,
			"name":       item.Name,
			"employees":  item.Employees,
		})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"page": map[string]any{
			"perPage":   page.PerPage,
			"total":     page.Total,
			"totalPage": page.TotalPage,
		},
		"list": list,
	})
}

func (h *ChannelCodeHandler) Statistics(w http.ResponseWriter, r *http.Request) {
	params, ok := h.validateStatisticsParams(w, r)
	if !ok {
		return
	}
	contacts, err := h.store.ChannelCodeStatContacts(r.Context(), params.ChannelCodeID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", channelCodeStatisticsData(contacts, params))
}

func (h *ChannelCodeHandler) StatisticsIndex(w http.ResponseWriter, r *http.Request) {
	params, ok := h.validateStatisticsParams(w, r)
	if !ok {
		return
	}
	params.Page = positiveQueryInt(r, "page", 1)
	params.PerPage = positiveQueryInt(r, "perPage", 10)
	contacts, err := h.store.ChannelCodeStatContacts(r.Context(), params.ChannelCodeID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := channelCodeStatisticsRows(contacts, params)
	totalPage := 0
	if len(list) > 0 {
		totalPage = (len(list) + params.PerPage - 1) / params.PerPage
	}
	start := (params.Page - 1) * params.PerPage
	if start > len(list) {
		start = len(list)
	}
	end := start + params.PerPage
	if end > len(list) {
		end = len(list)
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"page": map[string]any{
			"perPage":   strconv.Itoa(params.PerPage),
			"total":     len(list),
			"totalPage": totalPage,
		},
		"list": list[start:end],
	})
}

func (h *ChannelCodeHandler) validateStatisticsParams(w http.ResponseWriter, r *http.Request) (channelCodeStatisticsParams, bool) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return channelCodeStatisticsParams{}, false
	}
	userID, _, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return channelCodeStatisticsParams{}, false
	}
	if _, err := h.authorizeAccess(r.Context(), r, userID, loginInfo); err != nil {
		writeAccessError(w, err)
		return channelCodeStatisticsParams{}, false
	}
	rawID := strings.TrimSpace(r.URL.Query().Get("channelCodeId"))
	if rawID == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "渠道码ID 必填", nil)
		return channelCodeStatisticsParams{}, false
	}
	channelCodeID, err := strconv.Atoi(rawID)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "渠道码ID 必需为整数", nil)
		return channelCodeStatisticsParams{}, false
	}
	rawType := strings.TrimSpace(r.URL.Query().Get("type"))
	if rawType == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "统计类型 必填", nil)
		return channelCodeStatisticsParams{}, false
	}
	statType, err := strconv.Atoi(rawType)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "统计类型 必需为整数", nil)
		return channelCodeStatisticsParams{}, false
	}
	if statType < 1 || statType > 3 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "统计类型 值必须在列表内：[1,2,3]", nil)
		return channelCodeStatisticsParams{}, false
	}
	params := channelCodeStatisticsParams{
		ChannelCodeID: channelCodeID,
		Type:          statType,
		StartTime:     strings.TrimSpace(r.URL.Query().Get("startTime")),
		EndTime:       strings.TrimSpace(r.URL.Query().Get("endTime")),
	}
	if params.Type == 1 && (params.StartTime == "" || params.EndTime == "") {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "按天统计开始和结束时间必传", nil)
		return channelCodeStatisticsParams{}, false
	}
	return params, true
}

type channelCodeStatisticsParams struct {
	ChannelCodeID int
	Type          int
	StartTime     string
	EndTime       string
	Page          int
	PerPage       int
}

type channelCodeStatisticRow struct {
	Time             string `json:"time"`
	AddNumRange      int    `json:"addNumRange"`
	DefriendNumRange int    `json:"defriendNumRange"`
	DeleteNumRange   int    `json:"deleteNumRange"`
	NetNumRange      int    `json:"netNumRange"`
}

func channelCodeStatisticsData(contacts []ChannelCodeStatContact, params channelCodeStatisticsParams) map[string]any {
	rows := channelCodeEmptyStatisticRows(params)
	data := map[string]any{
		"addNum":          0,
		"defriendNum":     0,
		"deleteNum":       0,
		"netNum":          0,
		"addNumLong":      0,
		"defriendNumLong": 0,
		"deleteNumLong":   0,
		"netNumLong":      0,
		"list":            rows,
	}
	if len(contacts) == 0 {
		return data
	}
	index := map[string]*channelCodeStatisticRow{}
	for i := range rows {
		index[rows[i].Time] = &rows[i]
	}
	today := time.Now().Format("2006-01-02")
	addNum := 0
	defriendNum := 0
	deleteNum := 0
	addNumLong := 0
	defriendNumLong := 0
	deleteNumLong := 0
	for _, contact := range contacts {
		createDay := channelCodeDateKey(contact.CreateAt, params.Type)
		deleteDay := channelCodeDateKey(contact.DeletedAt, params.Type)
		if channelCodeDateKey(contact.CreateAt, 1) == today {
			addNum++
		}
		if row, ok := index[createDay]; ok {
			addNumLong++
			row.AddNumRange++
		}
		switch contact.Status {
		case 2:
			if channelCodeDateKey(contact.DeletedAt, 1) == today {
				deleteNum++
			}
			if row, ok := index[deleteDay]; ok {
				deleteNumLong++
				row.DeleteNumRange++
			}
		case 3:
			if channelCodeDateKey(contact.DeletedAt, 1) == today {
				defriendNum++
			}
			if row, ok := index[deleteDay]; ok {
				defriendNumLong++
				row.DefriendNumRange++
			}
		}
	}
	for i := range rows {
		rows[i].NetNumRange = rows[i].AddNumRange - rows[i].DefriendNumRange
	}
	data["addNum"] = addNum
	data["defriendNum"] = defriendNum
	data["deleteNum"] = deleteNum
	data["netNum"] = addNum - defriendNum
	data["addNumLong"] = addNumLong
	data["defriendNumLong"] = defriendNumLong
	data["deleteNumLong"] = deleteNumLong
	data["netNumLong"] = addNumLong - defriendNumLong
	data["list"] = rows
	return data
}

func channelCodeStatisticsRows(contacts []ChannelCodeStatContact, params channelCodeStatisticsParams) []channelCodeStatisticRow {
	rows := channelCodeEmptyStatisticRows(params)
	index := map[string]*channelCodeStatisticRow{}
	for i := range rows {
		index[rows[i].Time] = &rows[i]
	}
	for _, contact := range contacts {
		createKey := channelCodeDateKey(contact.CreateAt, params.Type)
		deleteKey := channelCodeDateKey(contact.DeletedAt, params.Type)
		if row, ok := index[createKey]; ok {
			row.AddNumRange++
		}
		switch contact.Status {
		case 2:
			if row, ok := index[deleteKey]; ok {
				row.DeleteNumRange++
			}
		case 3:
			if row, ok := index[deleteKey]; ok {
				row.DefriendNumRange++
			}
		}
	}
	for i := range rows {
		rows[i].NetNumRange = rows[i].AddNumRange - rows[i].DefriendNumRange
	}
	return rows
}

func channelCodeEmptyStatisticRows(params channelCodeStatisticsParams) []channelCodeStatisticRow {
	rows := make([]channelCodeStatisticRow, 0)
	switch params.Type {
	case 1:
		start, startErr := time.ParseInLocation("2006-01-02", params.StartTime, time.Local)
		end, endErr := time.ParseInLocation("2006-01-02", params.EndTime, time.Local)
		if startErr != nil || endErr != nil || end.Before(start) {
			return rows
		}
		for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
			rows = append(rows, channelCodeStatisticRow{Time: day.Format("2006-01-02")})
		}
	case 2:
		beforeWeekDay := time.Now().AddDate(0, 0, -7)
		for i := 1; i <= 7; i++ {
			rows = append(rows, channelCodeStatisticRow{Time: beforeWeekDay.AddDate(0, 0, i).Format("2006-01-02")})
		}
	default:
		beforeYearMonth := time.Now().AddDate(-1, 0, 0)
		for i := 1; i <= 12; i++ {
			rows = append(rows, channelCodeStatisticRow{Time: beforeYearMonth.AddDate(0, i, 0).Format("2006-01")})
		}
	}
	return rows
}

func channelCodeDateKey(raw string, statType int) string {
	if raw == "" {
		return "outTime"
	}
	layouts := []string{"2006-01-02 15:04:05", "2006-01-02"}
	for _, layout := range layouts {
		if value, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			if statType == 3 {
				return value.Format("2006-01")
			}
			return value.Format("2006-01-02")
		}
	}
	if statType == 3 && len(raw) >= 7 {
		return raw[:7]
	}
	if len(raw) >= 10 {
		return raw[:10]
	}
	return raw
}

func (h *ChannelCodeHandler) authorizeAccess(ctx context.Context, r *http.Request, userID int, loginInfo LoginCorpInfo) (AccessContext, error) {
	if h.authorizer == nil {
		return AccessContext{DataPermission: DataPermissionAll}, nil
	}
	corpID := 0
	if len(loginInfo.CorpIDs) > 0 {
		corpID = loginInfo.CorpIDs[0]
	}
	return h.authorizer.Resolve(ctx, userID, PermissionKeyFromRequest(r), corpID, loginInfo.WorkEmployeeID)
}

func (h *ChannelCodeHandler) fileFullURL(path string) string {
	if path == "" || strings.Contains(path, "http") || h.apiBaseURL == "" {
		return path
	}
	return h.apiBaseURL + "/static/" + strings.TrimLeft(path, "/")
}

func channelCodeAutoAddFriendValue(value int) any {
	switch value {
	case 1:
		return "开启"
	case 2:
		return "关闭"
	default:
		return value
	}
}

func channelCodeTypeText(value int) string {
	switch value {
	case 1:
		return "单人"
	case 2:
		return "多人"
	default:
		return ""
	}
}
