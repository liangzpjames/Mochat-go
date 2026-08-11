package dashboard

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ContactBatchAddFilter struct {
	CorpID    int
	Status    int
	HasStatus bool
	SearchKey string
	RecordID  int
	Page      int
	PerPage   int
}

type ContactBatchAddItem struct {
	ID         int
	RecordID   int
	Phone      string
	UploadAt   string
	Status     int
	AddAt      string
	EmployeeID int
	AllotNum   int
	Remark     string
	TagIDs     []int
}

type ContactBatchAddPage struct {
	Items     []ContactBatchAddItem
	Total     int
	TotalPage int
	PerPage   int
	Page      int
}

type ContactBatchAddImportRecord struct {
	ID          int
	CorpID      int
	Title       string
	UploadAt    string
	EmployeeIDs []int
	TagIDs      []int
	ImportNum   int
	AddNum      int
	FileName    string
	FileURL     string
}

type ContactBatchAddImportRecordPage struct {
	Items     []ContactBatchAddImportRecord
	Total     int
	TotalPage int
	PerPage   int
	Page      int
}

type ContactBatchAddEmployee struct {
	ID   int
	Name string
}

type ContactBatchAddTag struct {
	ID   int
	Name string
}

type ContactBatchAddConfig struct {
	PendingStatus       int
	PendingTimeOut      int
	PendingReminderTime string
	PendingLeaderID     int
	UndoneStatus        int
	UndoneTimeOut       int
	UndoneReminderTime  string
	RecycleStatus       int
	RecycleTimeOut      int
}

type ContactBatchAddImportRow struct {
	Phone  string
	Remark string
}

type ContactBatchAddImportWrite struct {
	CorpID      int
	Title       string
	EmployeeIDs []int
	TagIDs      []int
	FileName    string
	FileURL     string
	Rows        []ContactBatchAddImportRow
	OperateID   int
}

type ContactBatchAddEmployeeStat struct {
	ID         int
	Name       string
	AllotNum   int
	ToAddNum   int
	PendingNum int
	PassedNum  int
	RecycleNum int
	Completion float64
}

type ContactBatchAddEmployeeStatPage struct {
	Items     []ContactBatchAddEmployeeStat
	Total     int
	TotalPage int
	PerPage   int
	Page      int
}

type ContactBatchAddDashboardSummary struct {
	ContactNum int
	PendingNum int
	ToAddNum   int
	PassedNum  int
	Completion float64
}

type ContactBatchAddDashboardData struct {
	Employees ContactBatchAddEmployeeStatPage
	Dashboard ContactBatchAddDashboardSummary
}

type ContactBatchAddDashboardStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	ContactBatchAddPage(ctx context.Context, filter ContactBatchAddFilter) (ContactBatchAddPage, error)
	ContactBatchAddImportRecordPage(ctx context.Context, corpID int, page int, perPage int) (ContactBatchAddImportRecordPage, error)
	ContactBatchAddEmployeesByIDs(ctx context.Context, employeeIDs []int) (map[int]ContactBatchAddEmployee, error)
	ContactBatchAddTagsByIDs(ctx context.Context, corpID int, tagIDs []int) (map[int]ContactBatchAddTag, error)
	ContactBatchAddConfig(ctx context.Context, corpID int) (ContactBatchAddConfig, bool, error)
	UpsertContactBatchAddConfig(ctx context.Context, corpID int, values ContactBatchAddConfig) error
	CreateContactBatchAddImport(ctx context.Context, values ContactBatchAddImportWrite) (int, error)
	AllotContactBatchAddImports(ctx context.Context, corpID int, importIDs []int, employeeIDs []int, operateID int) (int, error)
	DeleteContactBatchAddImports(ctx context.Context, corpID int, importIDs []int) (int, error)
	DeleteContactBatchAddImportRecords(ctx context.Context, corpID int, recordIDs []int) (int, int, error)
	ContactBatchAddDashboard(ctx context.Context, corpID int, page int, perPage int) (ContactBatchAddDashboardData, error)
}

type ContactBatchAddDashboardHandler struct {
	store       ContactBatchAddDashboardStore
	cache       LoginCache
	resolver    UserIDResolver
	authorizer  CorpAdminAuthorizer
	storageRoot string
	apiBaseURL  string
	now         func() time.Time
}

func NewContactBatchAddDashboardHandler(store ContactBatchAddDashboardStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer) *ContactBatchAddDashboardHandler {
	return &ContactBatchAddDashboardHandler{store: store, cache: cache, resolver: resolver, authorizer: authorizer}
}

func (h *ContactBatchAddDashboardHandler) WithFileStorage(storageRoot string, apiBaseURL string) *ContactBatchAddDashboardHandler {
	h.storageRoot = strings.TrimSpace(storageRoot)
	h.apiBaseURL = strings.TrimRight(strings.TrimSpace(apiBaseURL), "/")
	return h
}

func (h *ContactBatchAddDashboardHandler) Index(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/contactBatchAdd/index#get")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	status, hasStatus, ok := contactBatchAddDashboardStatus(w, r)
	if !ok {
		return
	}
	recordID := positiveQueryInt(r, "recordId", 0)
	if recordID == 0 {
		recordID = positiveQueryInt(r, "record_id", 0)
	}
	page, err := h.store.ContactBatchAddPage(r.Context(), ContactBatchAddFilter{
		CorpID:    corpID,
		Status:    status,
		HasStatus: hasStatus,
		SearchKey: strings.TrimSpace(r.URL.Query().Get("searchKey")),
		RecordID:  recordID,
		Page:      positiveQueryInt(r, "page", 1),
		PerPage:   positiveQueryInt(r, "perPage", 15),
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	employees, tags, err := h.contactBatchAddLookups(r.Context(), corpID, page.Items, nil)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, contactBatchAddItemPayload(item, employees, tags))
	}
	writeEnvelope(w, http.StatusOK, 200, "success", contactBatchAddPagination(r, page.Page, page.PerPage, page.Total, list))
}

func (h *ContactBatchAddDashboardHandler) ImportIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/contactBatchAdd/importIndex#get")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	page, err := h.store.ContactBatchAddImportRecordPage(r.Context(), corpID, positiveQueryInt(r, "page", 1), positiveQueryInt(r, "perPage", 15))
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	employees, tags, err := h.contactBatchAddRecordLookups(r.Context(), corpID, page.Items)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, contactBatchAddRecordPayload(item, employees, tags, h.fileFullURL))
	}
	writeEnvelope(w, http.StatusOK, 200, "success", contactBatchAddPagination(r, page.Page, page.PerPage, page.Total, list))
}

func (h *ContactBatchAddDashboardHandler) SettingEdit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/contactBatchAdd/settingEdit#get")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	config, _, err := h.store.ContactBatchAddConfig(r.Context(), corpID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	var leader map[string]any
	if config.PendingLeaderID > 0 {
		employees, err := h.store.ContactBatchAddEmployeesByIDs(r.Context(), []int{config.PendingLeaderID})
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		if employee, ok := employees[config.PendingLeaderID]; ok {
			leader = map[string]any{"id": employee.ID, "name": employee.Name}
		}
	}
	payload := contactBatchAddConfigPayload(config)
	payload["pendingLeader"] = leader
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *ContactBatchAddDashboardHandler) SettingUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/contactBatchAdd/settingUpdate#post")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	config, ok := contactBatchAddConfigFromParams(w, params)
	if !ok {
		return
	}
	if err := h.store.UpsertContactBatchAddConfig(r.Context(), corpID, config); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{"status": 1})
}

func (h *ContactBatchAddDashboardHandler) ImportStore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, user, _, access, ok := h.resolveAuthorized(w, r, "/dashboard/contactBatchAdd/importStore#post")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	write, ok := h.contactBatchAddImportWrite(w, r, corpID, user.TenantID, userID, access.WorkEmployeeID)
	if !ok {
		return
	}
	successNum, err := h.store.CreateContactBatchAddImport(r.Context(), write)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{"successNum": successNum})
}

func (h *ContactBatchAddDashboardHandler) Allot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, access, ok := h.resolveAuthorized(w, r, "/dashboard/contactBatchAdd/allot#post")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	ids, err := intSliceParamAny(params, "ids", "id", "importIds", "importId", "contactIds", "contactId")
	if err != nil || len(ids) == 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "导入客户ID 必填", nil)
		return
	}
	employeeIDs, err := intSliceParamAny(params, "allotEmployee", "employeeIds", "employeeId", "employees")
	if err != nil || len(employeeIDs) == 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "分配员工 必填", nil)
		return
	}
	updateNum, err := h.store.AllotContactBatchAddImports(r.Context(), corpID, ids, employeeIDs, access.WorkEmployeeID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{"updateNum": updateNum})
}

func (h *ContactBatchAddDashboardHandler) Destroy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/contactBatchAdd/destroy#delete")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	ids, err := intSliceParamAny(params, "ids", "id", "importIds", "importId", "contactIds", "contactId")
	if err != nil || len(ids) == 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "导入客户ID 必填", nil)
		return
	}
	deleted, err := h.store.DeleteContactBatchAddImports(r.Context(), corpID, ids)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{"delNum": deleted})
}

func (h *ContactBatchAddDashboardHandler) ImportDestroy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/contactBatchAdd/importDestroy#delete")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	ids, err := intSliceParamAny(params, "ids", "id", "recordIds", "recordId")
	if err != nil || len(ids) == 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "导入批次ID 必填", nil)
		return
	}
	recordNum, contactNum, err := h.store.DeleteContactBatchAddImportRecords(r.Context(), corpID, ids)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{"delRecordNum": recordNum, "delContactNum": contactNum})
}

func (h *ContactBatchAddDashboardHandler) DataStatistic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/contactBatchAdd/dataStatistic#get")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	data, err := h.store.ContactBatchAddDashboard(r.Context(), corpID, positiveQueryInt(r, "page", 1), positiveQueryInt(r, "perPage", 15))
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	employees := make([]map[string]any, 0, len(data.Employees.Items))
	for _, item := range data.Employees.Items {
		employees = append(employees, map[string]any{
			"id":         item.ID,
			"name":       item.Name,
			"allotNum":   item.AllotNum,
			"toAddNum":   item.ToAddNum,
			"pendingNum": item.PendingNum,
			"passedNum":  item.PassedNum,
			"recycleNum": item.RecycleNum,
			"completion": item.Completion,
		})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"employees": contactBatchAddPagination(r, data.Employees.Page, data.Employees.PerPage, data.Employees.Total, employees),
		"dashboard": map[string]any{
			"contactNum": data.Dashboard.ContactNum,
			"pendingNum": data.Dashboard.PendingNum,
			"toAddNum":   data.Dashboard.ToAddNum,
			"passedNum":  data.Dashboard.PassedNum,
			"completion": data.Dashboard.Completion,
		},
	})
}

func (h *ContactBatchAddDashboardHandler) Remind(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if _, _, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/contactBatchAdd/remind#get"); !ok {
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *ContactBatchAddDashboardHandler) contactBatchAddImportWrite(w http.ResponseWriter, r *http.Request, corpID int, tenantID int, userID int, operateID int) (ContactBatchAddImportWrite, bool) {
	var (
		title       string
		employeeIDs []int
		tagIDs      []int
		fileName    string
		fileURL     string
		rows        []ContactBatchAddImportRow
	)
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "导入文件解析失败", nil)
			return ContactBatchAddImportWrite{}, false
		}
		title = strings.TrimSpace(r.FormValue("title"))
		employeeIDs = intSliceFromValues(r.MultipartForm.Value, "allotEmployee", "allotEmployee[]", "employeeIds", "employeeIds[]", "employeeId")
		tagIDs = intSliceFromValues(r.MultipartForm.Value, "tags", "tags[]", "tagIds", "tagIds[]", "tagId")
		if title == "" {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "标题 必填", nil)
			return ContactBatchAddImportWrite{}, false
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "导入文件 必填", nil)
			return ContactBatchAddImportWrite{}, false
		}
		defer file.Close()
		raw, err := io.ReadAll(io.LimitReader(file, 20<<20))
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "导入文件解析失败:"+err.Error(), nil)
			return ContactBatchAddImportWrite{}, false
		}
		parsedRows, err := contactBatchAddRowsFromFileBytes(raw, header.Filename)
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "导入文件解析失败:"+err.Error(), nil)
			return ContactBatchAddImportWrite{}, false
		}
		if len(parsedRows) == 0 {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "导入客户不能为空", nil)
			return ContactBatchAddImportWrite{}, false
		}
		rows = parsedRows
		fileName = header.Filename
		relativePath, err := h.storeContactBatchAddImportFile(r.Context(), tenantID, userID, corpID, header.Filename, header.Header.Get("Content-Type"), raw)
		if err != nil {
			if writeSaaSQuotaError(w, err) {
				return ContactBatchAddImportWrite{}, false
			}
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "导入文件保存失败:"+err.Error(), nil)
			return ContactBatchAddImportWrite{}, false
		}
		fileURL = relativePath
	} else {
		params, err := parseRequestParams(r)
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
			return ContactBatchAddImportWrite{}, false
		}
		title = stringParam(params, "title")
		var idsErr error
		employeeIDs, idsErr = intSliceParamAny(params, "allotEmployee", "employeeIds", "employeeId", "employees")
		if idsErr != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "分配员工参数错误", nil)
			return ContactBatchAddImportWrite{}, false
		}
		tagIDs, idsErr = intSliceParamAny(params, "tags", "tagIds", "tagId")
		if idsErr != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "标签参数错误", nil)
			return ContactBatchAddImportWrite{}, false
		}
		rows = contactBatchAddRowsFromParams(params)
		fileName = stringParam(params, "fileName")
		fileURL = stringParam(params, "fileUrl")
		if fileURL == "" {
			fileURL = stringParam(params, "file_url")
		}
	}
	if title == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "标题 必填", nil)
		return ContactBatchAddImportWrite{}, false
	}
	if len(rows) == 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "导入客户不能为空", nil)
		return ContactBatchAddImportWrite{}, false
	}
	return ContactBatchAddImportWrite{
		CorpID:      corpID,
		Title:       title,
		EmployeeIDs: employeeIDs,
		TagIDs:      tagIDs,
		FileName:    fileName,
		FileURL:     fileURL,
		Rows:        rows,
		OperateID:   operateID,
	}, true
}

func (h *ContactBatchAddDashboardHandler) storeContactBatchAddImportFile(ctx context.Context, tenantID int, userID int, corpID int, originalName string, contentType string, raw []byte) (string, error) {
	if strings.TrimSpace(h.storageRoot) == "" || len(raw) == 0 {
		return "", nil
	}
	relativePath, err := h.contactBatchAddImportRelativePath(originalName)
	if err != nil {
		return "", err
	}
	targetPath, err := contactBatchAddImportTargetPath(h.storageRoot, relativePath)
	if err != nil {
		return "", err
	}
	if tenantID > 0 {
		if quotaStore, ok := h.store.(commonUploadStorageQuotaStore); ok {
			status, err := quotaStore.SaaSStorageQuotaStatus(ctx, tenantID, int64(len(raw)))
			if err != nil {
				return "", err
			}
			if status.Exceeded() {
				return "", NewSaaSQuotaExceededError(status)
			}
		}
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(targetPath), ".contact-batch-add-*")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if err := tmp.Chmod(0644); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		return "", err
	}
	if tenantID > 0 {
		if recorder, ok := h.store.(commonUploadStorageRecorder); ok {
			if err := recorder.RecordCommonUploadStorageObject(ctx, CommonUploadStorageObject{
				TenantID:     tenantID,
				UserID:       userID,
				CorpID:       corpID,
				Source:       "dashboard.contactBatchAdd.importStore",
				OriginalName: originalName,
				RelativePath: relativePath,
				ContentType:  strings.TrimSpace(contentType),
				SizeBytes:    int64(len(raw)),
			}); err != nil {
				_ = os.Remove(targetPath)
				return "", err
			}
		}
		if quotaStore, ok := h.store.(commonUploadStorageQuotaStore); ok {
			if err := quotaStore.RefreshSaaSUsageCounter(ctx, tenantID, SaaSMetricStorage); err != nil {
				_ = os.Remove(targetPath)
				return "", err
			}
		}
	}
	return relativePath, nil
}

func (h *ContactBatchAddDashboardHandler) contactBatchAddImportRelativePath(originalName string) (string, error) {
	extension := strings.ToLower(filepath.Ext(originalName))
	if extension == "" {
		extension = ".csv"
	}
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	now := h.currentTime()
	name := fmt.Sprintf("%d%s%s", now.UnixNano()/1e5, hex.EncodeToString(random), extension)
	return filepath.ToSlash(filepath.Join("contactBatchAdd", "import", now.Format("2006/0102"), name)), nil
}

func (h *ContactBatchAddDashboardHandler) currentTime() time.Time {
	if h.now != nil {
		return h.now()
	}
	return time.Now()
}

func (h *ContactBatchAddDashboardHandler) fileFullURL(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") || h.apiBaseURL == "" {
		return path
	}
	return h.apiBaseURL + "/static/" + strings.TrimLeft(path, "/")
}

func contactBatchAddImportTargetPath(root string, relativePath string) (string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if !commonUploadSafeRelativePath(relativePath) {
		return "", fmt.Errorf("导入文件路径非法")
	}
	target := filepath.Join(root, filepath.FromSlash(relativePath))
	if target != root && !strings.HasPrefix(target, root+string(filepath.Separator)) {
		return "", fmt.Errorf("导入文件路径越界")
	}
	return target, nil
}

func (h *ContactBatchAddDashboardHandler) contactBatchAddLookups(ctx context.Context, corpID int, items []ContactBatchAddItem, records []ContactBatchAddImportRecord) (map[int]ContactBatchAddEmployee, map[int]ContactBatchAddTag, error) {
	employeeIDs := make([]int, 0)
	tagIDs := make([]int, 0)
	for _, item := range items {
		employeeIDs = append(employeeIDs, item.EmployeeID)
		tagIDs = append(tagIDs, item.TagIDs...)
	}
	for _, record := range records {
		employeeIDs = append(employeeIDs, record.EmployeeIDs...)
		tagIDs = append(tagIDs, record.TagIDs...)
	}
	employees, err := h.store.ContactBatchAddEmployeesByIDs(ctx, uniquePositiveIntsLocal(employeeIDs))
	if err != nil {
		return nil, nil, err
	}
	tags, err := h.store.ContactBatchAddTagsByIDs(ctx, corpID, uniquePositiveIntsLocal(tagIDs))
	if err != nil {
		return nil, nil, err
	}
	return employees, tags, nil
}

func (h *ContactBatchAddDashboardHandler) contactBatchAddRecordLookups(ctx context.Context, corpID int, records []ContactBatchAddImportRecord) (map[int]ContactBatchAddEmployee, map[int]ContactBatchAddTag, error) {
	return h.contactBatchAddLookups(ctx, corpID, nil, records)
}

func (h *ContactBatchAddDashboardHandler) resolveAuthorized(w http.ResponseWriter, r *http.Request, permissionKey string) (int, User, DashboardRequestScope, AccessContext, bool) {
	userID, user, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return 0, User{}, DashboardRequestScope{}, AccessContext{}, false
	}
	corpID, ok := principalCorpID(r)
	if !ok {
		return 0, User{}, DashboardRequestScope{}, AccessContext{}, false
	}
	employeeID := principalScope.WorkEmployeeID
	if employeeID <= 0 {
		resolved, err := h.store.EmployeeIDByUserCorp(r.Context(), userID, corpID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return 0, User{}, DashboardRequestScope{}, AccessContext{}, false
		}
		employeeID = resolved
	}
	access := AccessContext{User: user, PermissionKey: permissionKey, CorpID: corpID, WorkEmployeeID: employeeID, DataPermission: DataPermissionAll}
	if h.authorizer != nil {
		resolved, err := h.authorizer.Resolve(r.Context(), userID, permissionKey, corpID, employeeID)
		if err != nil {
			writeAccessError(w, err)
			return 0, User{}, DashboardRequestScope{}, AccessContext{}, false
		}
		access = resolved
	}
	return userID, user, principalScope, access, true
}

func (h *ContactBatchAddDashboardHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (int, User, DashboardRequestScope, bool) {
	if h.resolver == nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "user resolver not configured", nil)
		return 0, User{}, DashboardRequestScope{}, false
	}
	requestPrincipal, err := DashboardPrincipalFromContext(r.Context())
	userID := requestPrincipal.UserID
	if err != nil || userID <= 0 {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return 0, User{}, DashboardRequestScope{}, false
	}
	user, found, err := h.store.UserByID(r.Context(), userID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return 0, User{}, DashboardRequestScope{}, false
	}
	if !found {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "user not found", nil)
		return 0, User{}, DashboardRequestScope{}, false
	}
	principalScope, err := DashboardRequestScopeFromContext(r.Context())
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return 0, User{}, DashboardRequestScope{}, false
	}
	return userID, user, DashboardRequestScope(principalScope), true
}

func contactBatchAddItemPayload(item ContactBatchAddItem, employees map[int]ContactBatchAddEmployee, tags map[int]ContactBatchAddTag) map[string]any {
	tagPayload := make([]map[string]any, 0, len(item.TagIDs))
	for _, id := range item.TagIDs {
		if tag, ok := tags[id]; ok {
			tagPayload = append(tagPayload, map[string]any{"id": tag.ID, "name": tag.Name})
		}
	}
	var employeePayload map[string]any
	if employee, ok := employees[item.EmployeeID]; ok {
		employeePayload = map[string]any{"id": employee.ID, "name": employee.Name}
	}
	return map[string]any{
		"id":            item.ID,
		"recordId":      item.RecordID,
		"phone":         item.Phone,
		"uploadAt":      item.UploadAt,
		"status":        item.Status,
		"statusText":    contactBatchAddStatusText(item.Status),
		"addAt":         item.AddAt,
		"employeeId":    item.EmployeeID,
		"allotNum":      item.AllotNum,
		"remark":        item.Remark,
		"tags":          tagPayload,
		"allotEmployee": employeePayload,
	}
}

func contactBatchAddRecordPayload(item ContactBatchAddImportRecord, employees map[int]ContactBatchAddEmployee, tags map[int]ContactBatchAddTag, fileFullURL func(string) string) map[string]any {
	employeePayload := make([]map[string]any, 0, len(item.EmployeeIDs))
	for _, id := range item.EmployeeIDs {
		if employee, ok := employees[id]; ok {
			employeePayload = append(employeePayload, map[string]any{"id": employee.ID, "name": employee.Name})
		}
	}
	tagPayload := make([]map[string]any, 0, len(item.TagIDs))
	for _, id := range item.TagIDs {
		if tag, ok := tags[id]; ok {
			tagPayload = append(tagPayload, map[string]any{"id": tag.ID, "name": tag.Name})
		}
	}
	return map[string]any{
		"id":            item.ID,
		"corpId":        item.CorpID,
		"title":         item.Title,
		"uploadAt":      item.UploadAt,
		"allotEmployee": employeePayload,
		"tags":          tagPayload,
		"importNum":     item.ImportNum,
		"addNum":        item.AddNum,
		"fileName":      item.FileName,
		"fileUrl":       fileFullURL(item.FileURL),
	}
}

func contactBatchAddConfigPayload(config ContactBatchAddConfig) map[string]any {
	return map[string]any{
		"pendingStatus":       config.PendingStatus,
		"pendingTimeOut":      config.PendingTimeOut,
		"pendingReminderTime": config.PendingReminderTime,
		"pendingLeaderId":     config.PendingLeaderID,
		"undoneStatus":        config.UndoneStatus,
		"undoneTimeOut":       config.UndoneTimeOut,
		"undoneReminderTime":  config.UndoneReminderTime,
		"recycleStatus":       config.RecycleStatus,
		"recycleTimeOut":      config.RecycleTimeOut,
	}
}

func contactBatchAddConfigFromParams(w http.ResponseWriter, params map[string]any) (ContactBatchAddConfig, bool) {
	intValue := func(key string) (int, bool) {
		value, ok, err := intParam(params, key)
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, key+" 参数错误", nil)
			return 0, false
		}
		if !ok {
			return 0, true
		}
		return value, true
	}
	pendingStatus, ok := intValue("pendingStatus")
	if !ok {
		return ContactBatchAddConfig{}, false
	}
	pendingTimeOut, ok := intValue("pendingTimeOut")
	if !ok {
		return ContactBatchAddConfig{}, false
	}
	pendingLeaderID, ok := intValue("pendingLeaderId")
	if !ok {
		return ContactBatchAddConfig{}, false
	}
	undoneStatus, ok := intValue("undoneStatus")
	if !ok {
		return ContactBatchAddConfig{}, false
	}
	undoneTimeOut, ok := intValue("undoneTimeOut")
	if !ok {
		return ContactBatchAddConfig{}, false
	}
	recycleStatus, ok := intValue("recycleStatus")
	if !ok {
		return ContactBatchAddConfig{}, false
	}
	recycleTimeOut, ok := intValue("recycleTimeOut")
	if !ok {
		return ContactBatchAddConfig{}, false
	}
	return ContactBatchAddConfig{
		PendingStatus:       pendingStatus,
		PendingTimeOut:      pendingTimeOut,
		PendingReminderTime: normalizeClockTime(stringParam(params, "pendingReminderTime")),
		PendingLeaderID:     pendingLeaderID,
		UndoneStatus:        undoneStatus,
		UndoneTimeOut:       undoneTimeOut,
		UndoneReminderTime:  normalizeClockTime(stringParam(params, "undoneReminderTime")),
		RecycleStatus:       recycleStatus,
		RecycleTimeOut:      recycleTimeOut,
	}, true
}

func contactBatchAddDashboardStatus(w http.ResponseWriter, r *http.Request) (int, bool, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get("status"))
	if raw == "" {
		return 0, false, true
	}
	status, err := strconv.Atoi(raw)
	if err != nil || status < 0 || status > 3 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "状态参数错误", nil)
		return 0, false, false
	}
	return status, true, true
}

func contactBatchAddPagination(r *http.Request, currentPage int, perPage int, total int, data any) map[string]any {
	if currentPage <= 0 {
		currentPage = 1
	}
	if perPage <= 0 {
		perPage = 15
	}
	lastPage := 0
	if total > 0 {
		lastPage = (total + perPage - 1) / perPage
	}
	basePath := "http://" + r.Host + r.URL.Path
	pageURL := func(page int) any {
		if page <= 0 || (lastPage > 0 && page > lastPage) {
			return nil
		}
		return fmt.Sprintf("%s?page=%d", basePath, page)
	}
	from := 0
	to := 0
	if total > 0 && currentPage <= lastPage {
		from = (currentPage-1)*perPage + 1
		to = currentPage * perPage
		if to > total {
			to = total
		}
	}
	return map[string]any{
		"current_page":   currentPage,
		"data":           data,
		"first_page_url": pageURL(1),
		"from":           from,
		"last_page":      lastPage,
		"last_page_url":  pageURL(lastPage),
		"next_page_url":  pageURL(currentPage + 1),
		"path":           basePath,
		"per_page":       perPage,
		"prev_page_url":  pageURL(currentPage - 1),
		"to":             to,
		"total":          total,
	}
}

func contactBatchAddRowsFromParams(params map[string]any) []ContactBatchAddImportRow {
	rows := make([]ContactBatchAddImportRow, 0)
	if contacts, ok := params["contacts"]; ok {
		rows = append(rows, contactBatchAddRowsFromAny(contacts)...)
	}
	if phones := stringSliceParam(params, "phones"); len(phones) > 0 {
		for _, phone := range phones {
			rows = append(rows, ContactBatchAddImportRow{Phone: phone})
		}
	}
	if phone := stringParam(params, "phone"); phone != "" {
		rows = append(rows, ContactBatchAddImportRow{Phone: phone, Remark: stringParam(params, "remark")})
	}
	return normalizeContactBatchAddRows(rows)
}

func contactBatchAddRowsFromAny(value any) []ContactBatchAddImportRow {
	switch typed := value.(type) {
	case []any:
		rows := make([]ContactBatchAddImportRow, 0, len(typed))
		for _, item := range typed {
			rows = append(rows, contactBatchAddRowsFromAny(item)...)
		}
		return rows
	case []map[string]any:
		rows := make([]ContactBatchAddImportRow, 0, len(typed))
		for _, item := range typed {
			rows = append(rows, ContactBatchAddImportRow{Phone: stringParam(item, "phone"), Remark: stringParam(item, "remark")})
		}
		return rows
	case map[string]any:
		return []ContactBatchAddImportRow{{Phone: stringParam(typed, "phone"), Remark: stringParam(typed, "remark")}}
	case string:
		lines := strings.Split(typed, "\n")
		rows := make([]ContactBatchAddImportRow, 0, len(lines))
		for _, line := range lines {
			parts := strings.FieldsFunc(line, func(r rune) bool { return r == ',' || r == '\t' })
			if len(parts) > 0 {
				row := ContactBatchAddImportRow{Phone: parts[0]}
				if len(parts) > 1 {
					row.Remark = parts[1]
				}
				rows = append(rows, row)
			}
		}
		return rows
	default:
		raw := strings.TrimSpace(fmt.Sprint(value))
		if raw == "" {
			return nil
		}
		return []ContactBatchAddImportRow{{Phone: raw}}
	}
}

func contactBatchAddRowsFromFile(file multipart.File, header *multipart.FileHeader) ([]ContactBatchAddImportRow, error) {
	raw, err := io.ReadAll(io.LimitReader(file, 20<<20))
	if err != nil {
		return nil, err
	}
	return contactBatchAddRowsFromFileBytes(raw, header.Filename)
}

func contactBatchAddRowsFromFileBytes(raw []byte, filename string) ([]ContactBatchAddImportRow, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".xlsx":
		return contactBatchAddRowsFromXLSX(raw)
	default:
		reader := csv.NewReader(bytes.NewReader(raw))
		reader.FieldsPerRecord = -1
		reader.TrimLeadingSpace = true
		records, err := reader.ReadAll()
		if err != nil {
			return contactBatchAddRowsFromDelimitedText(string(raw)), nil
		}
		return normalizeContactBatchAddRows(contactBatchAddRowsFromRecords(records)), nil
	}
}

func contactBatchAddRowsFromDelimitedText(raw string) []ContactBatchAddImportRow {
	lines := strings.Split(raw, "\n")
	rows := make([]ContactBatchAddImportRow, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.FieldsFunc(line, func(r rune) bool { return r == ',' || r == '\t' || r == ';' })
		if len(parts) == 0 {
			continue
		}
		row := ContactBatchAddImportRow{Phone: parts[0]}
		if len(parts) > 1 {
			row.Remark = parts[1]
		}
		rows = append(rows, row)
	}
	return normalizeContactBatchAddRows(rows)
}

func contactBatchAddRowsFromRecords(records [][]string) []ContactBatchAddImportRow {
	rows := make([]ContactBatchAddImportRow, 0, len(records))
	for _, record := range records {
		if len(record) == 0 {
			continue
		}
		row := ContactBatchAddImportRow{Phone: record[0]}
		if len(record) > 1 {
			row.Remark = record[1]
		}
		rows = append(rows, row)
	}
	return rows
}

func contactBatchAddRowsFromXLSX(raw []byte) ([]ContactBatchAddImportRow, error) {
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, err
	}
	files := map[string]*zip.File{}
	for _, file := range zr.File {
		files[file.Name] = file
	}
	sharedStrings, err := xlsxSharedStrings(files["xl/sharedStrings.xml"])
	if err != nil {
		return nil, err
	}
	sheet := files["xl/worksheets/sheet1.xml"]
	if sheet == nil {
		return nil, fmt.Errorf("sheet1.xml not found")
	}
	rc, err := sheet.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	var worksheet xlsxWorksheet
	if err := xml.NewDecoder(rc).Decode(&worksheet); err != nil {
		return nil, err
	}
	records := make([][]string, 0, len(worksheet.SheetData.Rows))
	for _, row := range worksheet.SheetData.Rows {
		values := make([]string, 0, len(row.Cells))
		for _, cell := range row.Cells {
			values = append(values, xlsxCellValue(cell, sharedStrings))
		}
		records = append(records, values)
	}
	return normalizeContactBatchAddRows(contactBatchAddRowsFromRecords(records)), nil
}

func xlsxSharedStrings(file *zip.File) ([]string, error) {
	if file == nil {
		return []string{}, nil
	}
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	var table xlsxSharedStringTable
	if err := xml.NewDecoder(rc).Decode(&table); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(table.Items))
	for _, item := range table.Items {
		if item.Text != "" {
			out = append(out, item.Text)
			continue
		}
		parts := make([]string, 0, len(item.Runs))
		for _, run := range item.Runs {
			parts = append(parts, run.Text)
		}
		out = append(out, strings.Join(parts, ""))
	}
	return out, nil
}

func xlsxCellValue(cell xlsxCell, sharedStrings []string) string {
	if cell.Type == "s" {
		index, _ := strconv.Atoi(strings.TrimSpace(cell.Value))
		if index >= 0 && index < len(sharedStrings) {
			return sharedStrings[index]
		}
	}
	if cell.InlineString.Text != "" {
		return cell.InlineString.Text
	}
	return strings.TrimSpace(cell.Value)
}

type xlsxWorksheet struct {
	SheetData xlsxSheetData `xml:"sheetData"`
}

type xlsxSheetData struct {
	Rows []xlsxRow `xml:"row"`
}

type xlsxRow struct {
	Cells []xlsxCell `xml:"c"`
}

type xlsxCell struct {
	Type         string           `xml:"t,attr"`
	Value        string           `xml:"v"`
	InlineString xlsxInlineString `xml:"is"`
}

type xlsxInlineString struct {
	Text string `xml:"t"`
}

type xlsxSharedStringTable struct {
	Items []xlsxSharedStringItem `xml:"si"`
}

type xlsxSharedStringItem struct {
	Text string          `xml:"t"`
	Runs []xlsxStringRun `xml:"r"`
}

type xlsxStringRun struct {
	Text string `xml:"t"`
}

var contactBatchAddPhoneRE = regexp.MustCompile(`\d{4,}`)

func normalizeContactBatchAddRows(rows []ContactBatchAddImportRow) []ContactBatchAddImportRow {
	seen := map[string]struct{}{}
	out := make([]ContactBatchAddImportRow, 0, len(rows))
	for _, row := range rows {
		phone := strings.TrimSpace(row.Phone)
		remark := strings.TrimSpace(row.Remark)
		if phone == "" || !contactBatchAddPhoneRE.MatchString(phone) {
			continue
		}
		lower := strings.ToLower(phone)
		if _, ok := seen[lower]; ok {
			continue
		}
		seen[lower] = struct{}{}
		out = append(out, ContactBatchAddImportRow{Phone: phone, Remark: remark})
	}
	return out
}

func intSliceParamAny(params map[string]any, keys ...string) ([]int, error) {
	for _, key := range keys {
		if _, ok := params[key]; !ok {
			continue
		}
		values, err := intSliceParam(params, key)
		return values, err
	}
	return []int{}, nil
}

func intSliceFromValues(values map[string][]string, keys ...string) []int {
	params := map[string]any{}
	for _, key := range keys {
		if value, ok := values[key]; ok {
			params[key] = value
			ints, _ := intSliceParam(params, key)
			return ints
		}
	}
	return []int{}
}

func normalizeClockTime(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "00:00:00"
	}
	if len(raw) == 5 {
		return raw + ":00"
	}
	return raw
}
