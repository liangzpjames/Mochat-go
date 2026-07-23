package dashboard

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

type CommonUploadHandler struct {
	storageRoot string
	apiBaseURL  string
	resolver    UserIDResolver
	store       any
	now         func() time.Time
}

type CommonUploadStorageObject struct {
	TenantID     int
	UserID       int
	EmployeeID   int
	CorpID       int
	Source       string
	OriginalName string
	RelativePath string
	ContentType  string
	SizeBytes    int64
}

type commonUploadUserStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
}

type commonUploadEmployeeStore interface {
	SidebarEmployeeByID(ctx context.Context, employeeID int) (SidebarEmployee, bool, error)
}

type commonUploadTenantStore interface {
	TenantIDByCorpID(ctx context.Context, corpID int) (int, error)
}

type commonUploadStorageQuotaStore interface {
	SaaSStorageQuotaStatus(ctx context.Context, tenantID int, additionalBytes int64) (SaaSQuotaStatus, error)
	RefreshSaaSUsageCounter(ctx context.Context, tenantID int, metric string) error
}

type commonUploadStorageRecorder interface {
	RecordCommonUploadStorageObject(ctx context.Context, object CommonUploadStorageObject) error
}

type commonUploadOwner struct {
	TenantID   int
	UserID     int
	EmployeeID int
	CorpID     int
}

func NewCommonUploadHandler(storageRoot string, apiBaseURL string, resolver UserIDResolver) *CommonUploadHandler {
	return &CommonUploadHandler{
		storageRoot: storageRoot,
		apiBaseURL:  strings.TrimRight(strings.TrimSpace(apiBaseURL), "/"),
		resolver:    resolver,
	}
}

func NewCommonUploadHandlerWithStore(storageRoot string, apiBaseURL string, resolver UserIDResolver, store any) *CommonUploadHandler {
	handler := NewCommonUploadHandler(storageRoot, apiBaseURL, resolver)
	handler.store = store
	return handler
}

func (h *CommonUploadHandler) DashboardUpload(w http.ResponseWriter, r *http.Request) {
	h.handleGeneratedPathUpload(w, r, "dashboard.common.upload")
}

func (h *CommonUploadHandler) DashboardUploadFile(w http.ResponseWriter, r *http.Request) {
	h.handleGeneratedPathUpload(w, r, "dashboard.common.uploadFile")
}

func (h *CommonUploadHandler) SidebarUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	owner, ok := h.sidebarOwner(w, r)
	if !ok {
		return
	}
	file, header, ok := h.uploadedFile(w, r)
	if !ok {
		return
	}
	defer file.Close()
	if !commonUploadAllowedExtension(header.Filename) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "文件类型不合法", nil)
		return
	}

	uploadPath := strings.TrimSpace(r.FormValue("path"))
	if uploadPath == "" {
		uploadPath = h.currentTime().Format("2006/01/02/150405")
	}
	if !commonUploadSafeRelativePath(uploadPath) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "文件路径非法", nil)
		return
	}

	fileName := strings.TrimSpace(r.FormValue("name"))
	if fileName == "" {
		fileName = filepath.Base(header.Filename)
	}
	fileName = filepath.Base(fileName)
	if fileName == "." || fileName == string(filepath.Separator) || fileName == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "文件类型不合法", nil)
		return
	}
	if containsHan(fileName) || len(fileName) > 32 {
		sum := md5.Sum([]byte(fileName))
		fileName = hex.EncodeToString(sum[:])
	}

	relativePath := filepath.ToSlash(filepath.Join(uploadPath, fileName))
	targetPath, ok := h.targetPath(w, relativePath)
	if !ok {
		return
	}
	if !h.enforceStorageQuota(w, r, owner, header.Size) {
		return
	}
	if _, err := os.Stat(targetPath); err == nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "上传失败:已经存在此文件名的文件", nil)
		return
	} else if err != nil && !os.IsNotExist(err) {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "上传失败:"+err.Error(), nil)
		return
	}
	if err := h.writeUpload(file, targetPath); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "上传失败:"+err.Error(), nil)
		return
	}
	if !h.recordStorageObject(w, r, owner, CommonUploadStorageObject{
		Source:       "sidebar.common.upload",
		OriginalName: header.Filename,
		RelativePath: relativePath,
		ContentType:  header.Header.Get("Content-Type"),
		SizeBytes:    header.Size,
	}) {
		_ = os.Remove(targetPath)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", h.uploadPayload(fileName, header.Header.Get("Content-Type"), relativePath))
}

func (h *CommonUploadHandler) handleGeneratedPathUpload(w http.ResponseWriter, r *http.Request, source string) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	owner, ok := h.dashboardOwner(w, r)
	if !ok {
		return
	}
	file, header, ok := h.uploadedFile(w, r)
	if !ok {
		return
	}
	defer file.Close()
	if !commonUploadAllowedExtension(header.Filename) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "文件类型不合法", nil)
		return
	}
	extension := strings.TrimPrefix(strings.ToLower(filepath.Ext(header.Filename)), ".")
	relativePath, err := h.generatedRelativePath(extension)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "上传失败:"+err.Error(), nil)
		return
	}
	targetPath, ok := h.targetPath(w, relativePath)
	if !ok {
		return
	}
	if !h.enforceStorageQuota(w, r, owner, header.Size) {
		return
	}
	if err := h.writeUpload(file, targetPath); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "上传失败:"+err.Error(), nil)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		name = filepath.Base(header.Filename)
	}
	if !h.recordStorageObject(w, r, owner, CommonUploadStorageObject{
		Source:       source,
		OriginalName: header.Filename,
		RelativePath: relativePath,
		ContentType:  header.Header.Get("Content-Type"),
		SizeBytes:    header.Size,
	}) {
		_ = os.Remove(targetPath)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", h.uploadPayload(name, header.Header.Get("Content-Type"), relativePath))
}

func (h *CommonUploadHandler) dashboardOwner(w http.ResponseWriter, r *http.Request) (commonUploadOwner, bool) {
	if h.resolver == nil {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return commonUploadOwner{}, false
	}
	userID, err := h.resolver.UserID(r)
	if err != nil || userID <= 0 {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return commonUploadOwner{}, false
	}
	owner := commonUploadOwner{UserID: userID}
	userStore, ok := h.store.(commonUploadUserStore)
	if h.store == nil || !ok {
		return owner, true
	}
	user, found, err := userStore.UserByID(r.Context(), userID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return commonUploadOwner{}, false
	}
	if !found {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "user not found", nil)
		return commonUploadOwner{}, false
	}
	owner.TenantID = user.TenantID
	return owner, true
}

func (h *CommonUploadHandler) sidebarOwner(w http.ResponseWriter, r *http.Request) (commonUploadOwner, bool) {
	if h.resolver == nil {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return commonUploadOwner{}, false
	}
	employeeID, err := h.resolver.UserID(r)
	if err != nil || employeeID <= 0 {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return commonUploadOwner{}, false
	}
	owner := commonUploadOwner{EmployeeID: employeeID}
	employeeStore, ok := h.store.(commonUploadEmployeeStore)
	if h.store == nil || !ok {
		return owner, true
	}
	employee, found, err := employeeStore.SidebarEmployeeByID(r.Context(), employeeID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return commonUploadOwner{}, false
	}
	if !found {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "employee not found", nil)
		return commonUploadOwner{}, false
	}
	owner.CorpID = employee.CorpID
	owner.UserID = employee.LogUserID
	if employee.LogUserID > 0 {
		userStore, ok := h.store.(commonUploadUserStore)
		if ok {
			user, found, err := userStore.UserByID(r.Context(), employee.LogUserID)
			if err != nil {
				writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
				return commonUploadOwner{}, false
			}
			if found {
				owner.TenantID = user.TenantID
			}
		}
	}
	if owner.TenantID == 0 && employee.CorpID > 0 {
		tenantStore, ok := h.store.(commonUploadTenantStore)
		if ok {
			tenantID, err := tenantStore.TenantIDByCorpID(r.Context(), employee.CorpID)
			if err != nil {
				writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
				return commonUploadOwner{}, false
			}
			owner.TenantID = tenantID
		}
	}
	return owner, true
}

func (h *CommonUploadHandler) enforceStorageQuota(w http.ResponseWriter, r *http.Request, owner commonUploadOwner, sizeBytes int64) bool {
	if owner.TenantID <= 0 || sizeBytes <= 0 {
		return true
	}
	quotaStore, ok := h.store.(commonUploadStorageQuotaStore)
	if !ok {
		return true
	}
	status, err := quotaStore.SaaSStorageQuotaStatus(r.Context(), owner.TenantID, sizeBytes)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return false
	}
	if status.Exceeded() {
		writeSaaSQuotaExceeded(w, status)
		return false
	}
	return true
}

func (h *CommonUploadHandler) recordStorageObject(w http.ResponseWriter, r *http.Request, owner commonUploadOwner, object CommonUploadStorageObject) bool {
	recorder, ok := h.store.(commonUploadStorageRecorder)
	if !ok || owner.TenantID <= 0 || object.SizeBytes <= 0 {
		return true
	}
	object.TenantID = owner.TenantID
	object.UserID = owner.UserID
	object.EmployeeID = owner.EmployeeID
	object.CorpID = owner.CorpID
	if err := recorder.RecordCommonUploadStorageObject(r.Context(), object); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "上传用量记录失败", nil)
		return false
	}
	if quotaStore, ok := h.store.(commonUploadStorageQuotaStore); ok {
		if err := quotaStore.RefreshSaaSUsageCounter(r.Context(), owner.TenantID, SaaSMetricStorage); err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "上传用量刷新失败", nil)
			return false
		}
	}
	return true
}

func (h *CommonUploadHandler) uploadedFile(w http.ResponseWriter, r *http.Request) (multipart.File, *multipart.FileHeader, bool) {
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "文件类型不合法", nil)
		return nil, nil, false
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "文件 必填", nil)
		return nil, nil, false
	}
	return file, header, true
}

func (h *CommonUploadHandler) generatedRelativePath(extension string) (string, error) {
	if extension == "" {
		return "", fmt.Errorf("文件类型不合法")
	}
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	now := h.currentTime()
	filename := fmt.Sprintf("%d%s.%s", now.UnixNano()/1e5, hex.EncodeToString(random), extension)
	return filepath.ToSlash(filepath.Join(now.Format("2006/0102/1504"), filename)), nil
}

func (h *CommonUploadHandler) targetPath(w http.ResponseWriter, relativePath string) (string, bool) {
	if !commonUploadSafeRelativePath(relativePath) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "文件路径非法", nil)
		return "", false
	}
	root, err := filepath.Abs(h.storageRoot)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "上传失败:"+err.Error(), nil)
		return "", false
	}
	target := filepath.Join(root, filepath.FromSlash(relativePath))
	if !strings.HasPrefix(target, root+string(filepath.Separator)) && target != root {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "文件路径非法", nil)
		return "", false
	}
	return target, true
}

func (h *CommonUploadHandler) writeUpload(file multipart.File, targetPath string) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return err
	}
	out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, file)
	return err
}

func (h *CommonUploadHandler) uploadPayload(name string, contentType string, relativePath string) map[string]any {
	return map[string]any{
		"name":     name,
		"type":     strings.TrimSpace(contentType),
		"path":     relativePath,
		"fullPath": h.fileFullURL(relativePath),
	}
}

func (h *CommonUploadHandler) fileFullURL(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") || h.apiBaseURL == "" {
		return path
	}
	return h.apiBaseURL + "/static/" + strings.TrimLeft(path, "/")
}

func (h *CommonUploadHandler) currentTime() time.Time {
	if h.now != nil {
		return h.now()
	}
	return time.Now()
}

func commonUploadAllowedExtension(filename string) bool {
	extension := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	if extension == "" {
		return false
	}
	_, ok := commonUploadAllowedExtensions[extension]
	return ok
}

func commonUploadSafeRelativePath(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" || strings.Contains(path, "../") || strings.Contains(path, `..\`) || filepath.IsAbs(path) {
		return false
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	return clean != "." && clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator))
}

func containsHan(value string) bool {
	for _, r := range value {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

var commonUploadAllowedExtensions = map[string]struct{}{
	"jpg": {}, "png": {}, "jpeg": {}, "svg": {}, "gif": {}, "psd": {}, "bmp": {},
	"mp4": {}, "avi": {}, "3gp": {}, "flv": {}, "mp3": {}, "amr": {}, "wav": {},
	"ogg": {}, "mov": {}, "rmvb": {}, "wma": {}, "mkv": {},
	"doc": {}, "docx": {}, "xls": {}, "xlsx": {}, "csv": {}, "ppt": {}, "pptx": {},
	"txt": {}, "pdf": {}, "xmind": {},
}
