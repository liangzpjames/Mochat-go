package dashboard

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type AgentTxtVerifyUploadHandler struct {
	storageRoot string
}

func NewAgentTxtVerifyUploadHandler(storageRoot string) *AgentTxtVerifyUploadHandler {
	return &AgentTxtVerifyUploadHandler{storageRoot: storageRoot}
}

func (h *AgentTxtVerifyUploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}

	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "文件类型错误", nil)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "文件类型错误", nil)
		return
	}
	defer file.Close()

	if !isTextPlainUpload(header.Header.Get("Content-Type")) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "文件类型错误", nil)
		return
	}

	filename := filepath.Base(header.Filename)
	if filename == "." || filename == string(filepath.Separator) || filename == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "文件类型错误", nil)
		return
	}

	targetDir := filepath.Join(h.storageRoot, "wx_txt_verify")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "上传失败:"+err.Error(), nil)
		return
	}

	targetPath := filepath.Join(targetDir, filename)
	out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "上传失败:"+err.Error(), nil)
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "上传失败:"+err.Error(), nil)
		return
	}

	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func isTextPlainUpload(contentType string) bool {
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	return mediaType == "text/plain"
}
