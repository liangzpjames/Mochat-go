package dashboard

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	workMessageExportPageSize    = 20
	workMessageExportMaxObjects  = 100
	workMessageExportMaxMessages = 100000
	workMessageExportMaxYears    = 2
	workMessageExportRetention   = 7 * 24 * time.Hour
)

type WorkMessageExportType string

const (
	WorkMessageExportTypeEmployee WorkMessageExportType = "employee"
	WorkMessageExportTypeCustomer WorkMessageExportType = "customer"
	WorkMessageExportTypeRoom     WorkMessageExportType = "room"
)

type WorkMessageExportCandidateFilter struct {
	TenantID            int
	CorpID              int
	UserID              int
	ExportType          WorkMessageExportType
	Keyword             string
	DepartmentID        int
	Page                int
	PageSize            int
	RestrictEmployeeIDs bool
	EmployeeIDs         []int
}

type WorkMessageExportCandidate struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	Avatar            string `json:"avatar"`
	ExternalID        string `json:"externalId,omitempty"`
	Subtitle          string `json:"subtitle,omitempty"`
	ConversationCount int    `json:"conversationCount"`
	MessageCount      int    `json:"messageCount"`
	LastMessageAt     string `json:"lastMessageAt,omitempty"`
	Selectable        bool   `json:"selectable"`
	Limitation        string `json:"limitation,omitempty"`
}

type WorkMessageExportCandidatesPage struct {
	Items        []WorkMessageExportCandidate  `json:"items"`
	Total        int                           `json:"total"`
	Page         int                           `json:"page"`
	PageSize     int                           `json:"pageSize"`
	Limitations  []WorkMessageExportLimitation `json:"limitations"`
	Capabilities []WorkMessageExportCapability `json:"capabilities"`
}

type WorkMessageExportLimitation struct {
	Key    string `json:"key"`
	Reason string `json:"reason"`
}

type WorkMessageExportCapability struct {
	Key       string `json:"key"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

type WorkMessageExportTaskQuery struct {
	TenantID int
	CorpID   int
	UserID   int
	Page     int
	PageSize int
}

type WorkMessageExportTask struct {
	ID                    int64     `json:"id"`
	ExportType            string    `json:"exportType"`
	ObjectCount           int       `json:"objectCount"`
	StartAt               time.Time `json:"startAt"`
	EndAt                 time.Time `json:"endAt"`
	FileMode              string    `json:"fileMode"`
	Format                string    `json:"format"`
	Status                string    `json:"status"`
	EstimatedMessageCount int       `json:"estimatedMessageCount"`
	MessageCount          int       `json:"messageCount"`
	FileCount             int       `json:"fileCount"`
	ArtifactName          string    `json:"artifactName"`
	ArtifactSize          int64     `json:"artifactSize"`
	ErrorCode             string    `json:"errorCode,omitempty"`
	ErrorMessage          string    `json:"errorMessage,omitempty"`
	ExpiresAt             time.Time `json:"expiresAt"`
	CreatedAt             time.Time `json:"createdAt"`
	FinishedAt            time.Time `json:"finishedAt,omitempty"`
}

type WorkMessageExportTaskPage struct {
	Items    []WorkMessageExportTask `json:"items"`
	Total    int                     `json:"total"`
	Page     int                     `json:"page"`
	PageSize int                     `json:"pageSize"`
}

type WorkMessageExportCreateRequest struct {
	ExportType         string    `json:"exportType"`
	ObjectIDs          []int     `json:"objectIds"`
	ConversationScopes []string  `json:"conversationScopes"`
	EmployeeIDs        []int     `json:"employeeIds"`
	StartAt            time.Time `json:"startAt"`
	EndAt              time.Time `json:"endAt"`
	FileMode           string    `json:"fileMode"`
	Format             string    `json:"format"`
	IdempotencyKey     string    `json:"idempotencyKey"`
}

type WorkMessageExportCreateResult struct {
	Task        WorkMessageExportTask         `json:"task"`
	Reused      bool                          `json:"reused"`
	Limitations []WorkMessageExportLimitation `json:"limitations"`
}

type WorkMessageExportArtifact struct {
	Path        string
	Filename    string
	ContentType string
}

type WorkMessageExportStore interface {
	WorkMessageExportCandidates(context.Context, WorkMessageExportCandidateFilter) (WorkMessageExportCandidatesPage, error)
	WorkMessageExportTasks(context.Context, WorkMessageExportTaskQuery) (WorkMessageExportTaskPage, error)
	CreateWorkMessageExportTask(context.Context, WorkMessageExportTaskInput) (WorkMessageExportCreateResult, error)
	WorkMessageExportArtifact(context.Context, int, int, int, int64) (WorkMessageExportArtifact, error)
}

type WorkMessageExportTaskInput struct {
	TenantID            int
	CorpID              int
	UserID              int
	RestrictEmployeeIDs bool
	EmployeeIDs         []int
	Request             WorkMessageExportCreateRequest
}

var (
	ErrWorkMessageExportTaskNotFound = errors.New("导出任务不存在")
	ErrWorkMessageExportTaskExpired  = errors.New("导出文件已过期")
	ErrWorkMessageExportMessageLimit = errors.New("预计导出消息数超过 100,000 条")
)

func (h *AutoTagHandler) WorkMessageExportCandidates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, access, ok := h.resolveAuthorized(w, r, workMessageConversationPermissionKey)
	if !ok || !h.workMessageArchiveAllowed(w, r, principalScope.Principal.TenantID, principalScope.Principal.CorpID) {
		return
	}
	exportType := WorkMessageExportType(strings.TrimSpace(r.URL.Query().Get("type")))
	if !validWorkMessageExportType(exportType) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid export type", nil)
		return
	}
	pageSize, ok := workMessageExportPageSizeFromQuery(w, r)
	if !ok {
		return
	}
	store, ok := h.store.(WorkMessageExportStore)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "会话导出数据能力未接入", nil)
		return
	}
	page, err := store.WorkMessageExportCandidates(r.Context(), WorkMessageExportCandidateFilter{
		TenantID: principalScope.Principal.TenantID, CorpID: principalScope.Principal.CorpID, UserID: userID,
		ExportType: exportType, Keyword: strings.TrimSpace(r.URL.Query().Get("keyword")),
		DepartmentID: positiveQueryInt(r, "departmentId", 0), Page: positiveQueryInt(r, "page", 1), PageSize: pageSize,
		RestrictEmployeeIDs: access.DataPermission != DataPermissionAll, EmployeeIDs: workMessageUniquePositiveInts(access.DeptEmployeeIDs),
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	normalizeWorkMessageExportCandidatesPage(&page)
	page = applyWorkMessageExportEmployeeScope(page, access)
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", page)
}

func (h *AutoTagHandler) WorkMessageExportTasks(w http.ResponseWriter, r *http.Request) {
	userID, _, principalScope, access, ok := h.resolveAuthorized(w, r, workMessageConversationPermissionKey)
	if !ok || !h.workMessageArchiveAllowed(w, r, principalScope.Principal.TenantID, principalScope.Principal.CorpID) {
		return
	}
	store, ok := h.store.(WorkMessageExportStore)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "会话导出任务能力未接入", nil)
		return
	}
	switch r.Method {
	case http.MethodGet:
		pageSize, valid := workMessageExportPageSizeFromQuery(w, r)
		if !valid {
			return
		}
		page, err := store.WorkMessageExportTasks(r.Context(), WorkMessageExportTaskQuery{
			TenantID: principalScope.Principal.TenantID, CorpID: principalScope.Principal.CorpID, UserID: userID,
			Page: positiveQueryInt(r, "page", 1), PageSize: pageSize,
		})
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		normalizeWorkMessageExportTaskPage(&page)
		writeEnvelope(w, http.StatusOK, http.StatusOK, "success", page)
	case http.MethodPost:
		var request WorkMessageExportCreateRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&request); err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "导出参数格式错误", nil)
			return
		}
		now := time.Now()
		request = normalizeWorkMessageExportRequestAt(request, now)
		if err := validateWorkMessageExportRequest(request, now); err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
			return
		}
		if request.IdempotencyKey == "" {
			request.IdempotencyKey = workMessageExportIdempotencyKey(request)
		}
		result, err := store.CreateWorkMessageExportTask(r.Context(), WorkMessageExportTaskInput{
			TenantID: principalScope.Principal.TenantID, CorpID: principalScope.Principal.CorpID, UserID: userID,
			RestrictEmployeeIDs: access.DataPermission != DataPermissionAll, EmployeeIDs: workMessageUniquePositiveInts(access.DeptEmployeeIDs), Request: request,
		})
		if errors.Is(err, ErrWorkMessageExportTaskExpired) {
			writeEnvelope(w, http.StatusGone, http.StatusGone, err.Error(), nil)
			return
		}
		if errors.Is(err, ErrWorkMessageExportMessageLimit) {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
			return
		}
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		writeEnvelope(w, http.StatusAccepted, http.StatusAccepted, "export task accepted", result)
	default:
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
	}
}

func (h *AutoTagHandler) WorkMessageExportDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, _, ok := h.resolveAuthorized(w, r, workMessageConversationPermissionKey)
	if !ok || !h.workMessageArchiveAllowed(w, r, principalScope.Principal.TenantID, principalScope.Principal.CorpID) {
		return
	}
	taskID, err := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("taskId")), 10, 64)
	if err != nil || taskID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid taskId", nil)
		return
	}
	store, ok := h.store.(WorkMessageExportStore)
	if !ok {
		writeEnvelope(w, http.StatusNotImplemented, http.StatusNotImplemented, "会话导出下载能力未接入", nil)
		return
	}
	artifact, err := store.WorkMessageExportArtifact(r.Context(), principalScope.Principal.TenantID, principalScope.Principal.CorpID, userID, taskID)
	if errors.Is(err, ErrWorkMessageExportTaskNotFound) {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, err.Error(), nil)
		return
	}
	if errors.Is(err, ErrWorkMessageExportTaskExpired) {
		writeEnvelope(w, http.StatusGone, http.StatusGone, err.Error(), nil)
		return
	}
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	filename := filepath.Base(artifact.Filename)
	if filename == "." || filename == "" || filename == string(filepath.Separator) {
		filename = "conversation-export.zip"
	}
	w.Header().Set("Content-Type", artifact.ContentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+strings.ReplaceAll(filename, `"`, "_")+`"`)
	http.ServeFile(w, r, artifact.Path)
}

func validateWorkMessageExportRequest(request WorkMessageExportCreateRequest, now time.Time) error {
	if !validWorkMessageExportType(WorkMessageExportType(strings.TrimSpace(request.ExportType))) {
		return errors.New("导出类型不受支持")
	}
	if len(request.ObjectIDs) == 0 {
		return errors.New("至少选择一个导出对象")
	}
	if len(request.ObjectIDs) > workMessageExportMaxObjects {
		return fmt.Errorf("最多选择 %d 个对象", workMessageExportMaxObjects)
	}
	if request.StartAt.IsZero() || request.EndAt.IsZero() {
		return errors.New("请填写完整的日期范围")
	}
	if !request.EndAt.After(request.StartAt) {
		return errors.New("结束时间必须晚于开始时间")
	}
	if request.EndAt.After(now.Add(time.Minute)) {
		return errors.New("结束时间不能晚于当前时间")
	}
	if request.EndAt.After(request.StartAt.AddDate(workMessageExportMaxYears, 0, 0)) {
		return fmt.Errorf("日期跨度不能超过 %d 年", workMessageExportMaxYears)
	}
	for _, scope := range request.ConversationScopes {
		switch strings.TrimSpace(scope) {
		case "", "customer_direct", "external_group":
		default:
			return errors.New("当前仅支持客户单聊和外部群聊")
		}
	}
	if request.FileMode != "" && request.FileMode != "split" && request.FileMode != "merge" {
		return errors.New("文件组织方式不受支持")
	}
	if request.Format != "" && request.Format != "zip" {
		return errors.New("导出格式不受支持")
	}
	for _, id := range request.ObjectIDs {
		if id <= 0 {
			return errors.New("导出对象 ID 不合法")
		}
	}
	return nil
}

func validateWorkMessageExportPageSize(pageSize int) error {
	if pageSize != workMessageExportPageSize {
		return fmt.Errorf("会话导出分页固定为 %d 条", workMessageExportPageSize)
	}
	return nil
}

func workMessageExportPageSizeFromQuery(w http.ResponseWriter, r *http.Request) (int, bool) {
	pageSize := positiveQueryInt(r, "pageSize", workMessageExportPageSize)
	if raw := strings.TrimSpace(r.URL.Query().Get("pageSize")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid pageSize", nil)
			return 0, false
		}
		pageSize = parsed
	}
	if err := validateWorkMessageExportPageSize(pageSize); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return 0, false
	}
	return pageSize, true
}

func normalizeWorkMessageExportRequest(request WorkMessageExportCreateRequest) WorkMessageExportCreateRequest {
	request.ExportType = string(WorkMessageExportType(strings.TrimSpace(request.ExportType)))
	request.FileMode = strings.TrimSpace(request.FileMode)
	if request.FileMode == "" {
		request.FileMode = "split"
	}
	request.Format = strings.TrimSpace(request.Format)
	if request.Format == "" {
		request.Format = "zip"
	}
	request.ConversationScopes = uniqueWorkMessageExportStrings(request.ConversationScopes)
	request.ObjectIDs = uniqueWorkMessageExportInts(request.ObjectIDs)
	request.EmployeeIDs = uniqueWorkMessageExportInts(request.EmployeeIDs)
	return request
}

func normalizeWorkMessageExportRequestAt(request WorkMessageExportCreateRequest, now time.Time) WorkMessageExportCreateRequest {
	request = normalizeWorkMessageExportRequest(request)
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	endDate := request.EndAt.In(shanghai).Format("2006-01-02")
	nowDate := now.In(shanghai).Format("2006-01-02")
	if request.EndAt.After(now) && endDate == nowDate {
		request.EndAt = now
	}
	return request
}

func normalizeWorkMessageExportCandidatesPage(page *WorkMessageExportCandidatesPage) {
	if page.Items == nil {
		page.Items = []WorkMessageExportCandidate{}
	}
	if page.Limitations == nil {
		page.Limitations = []WorkMessageExportLimitation{}
	}
	if page.Capabilities == nil {
		page.Capabilities = []WorkMessageExportCapability{}
	}
	page.PageSize = workMessageExportPageSize
}

func normalizeWorkMessageExportTaskPage(page *WorkMessageExportTaskPage) {
	if page.Items == nil {
		page.Items = []WorkMessageExportTask{}
	}
	page.PageSize = workMessageExportPageSize
}

func applyWorkMessageExportEmployeeScope(page WorkMessageExportCandidatesPage, access AccessContext) WorkMessageExportCandidatesPage {
	if page.Items == nil || access.DataPermission == DataPermissionAll || len(access.DeptEmployeeIDs) == 0 {
		return page
	}
	allowed := make(map[int]struct{}, len(access.DeptEmployeeIDs))
	for _, id := range access.DeptEmployeeIDs {
		allowed[id] = struct{}{}
	}
	for index := range page.Items {
		if page.Items[index].Selectable {
			if _, ok := allowed[page.Items[index].ID]; !ok && page.Items[index].Subtitle == "employee" {
				page.Items[index].Selectable = false
				page.Items[index].Limitation = "当前账号无权导出该员工范围"
			}
		}
	}
	return page
}

func validWorkMessageExportType(exportType WorkMessageExportType) bool {
	return exportType == WorkMessageExportTypeEmployee || exportType == WorkMessageExportTypeCustomer || exportType == WorkMessageExportTypeRoom
}

func uniqueWorkMessageExportInts(values []int) []int {
	seen := make(map[int]struct{}, len(values))
	result := make([]int, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func uniqueWorkMessageExportStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func workMessageExportIdempotencyKey(request WorkMessageExportCreateRequest) string {
	request.IdempotencyKey = ""
	encoded, _ := json.Marshal(request)
	digest := sha256.Sum256(encoded)
	return "auto-" + hex.EncodeToString(digest[:])
}

func safeWorkMessageExportCSVCell(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	if strings.ContainsRune("=+-@", []rune(value)[0]) {
		return "'" + value
	}
	return value
}

// SafeWorkMessageExportCSVCell is used by the storage worker when it writes
// archive content into a spreadsheet-compatible CSV artifact.
func SafeWorkMessageExportCSVCell(value string) string { return safeWorkMessageExportCSVCell(value) }
