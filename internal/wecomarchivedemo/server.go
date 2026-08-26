package wecomarchivedemo

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func NewPublicHandler(config Config, store *EvidenceStore, sdkLoaded bool) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		state, err := store.LoadState()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok", "sdk_loaded": sdkLoaded,
			"callback_configured":    config.CallbackToken != "" && config.EncodingAESKey != "",
			"active_pull_configured": config.CorpID != "" && config.ArchiveSecret != "" && config.RSAPrivateKey != "",
			"callback_count":         state.CallbackCount, "pull_count": state.PullCount,
		})
	})
	mux.Handle("/wecom/callback", callbackHandler(config, store))
	return mux
}

func callbackHandler(config Config, store *EvidenceStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state, err := store.LoadState()
		if err != nil {
			http.Error(w, "state unavailable", http.StatusInternalServerError)
			return
		}
		expectedReceiveID := strings.TrimSpace(config.CorpID)
		if expectedReceiveID == "" {
			expectedReceiveID = state.BoundReceiveID
		}
		var encrypted string
		switch r.Method {
		case http.MethodGet:
			encrypted = strings.TrimSpace(r.URL.Query().Get("echostr"))
		case http.MethodPost:
			body, readErr := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
			if readErr != nil {
				http.Error(w, "invalid callback body", http.StatusBadRequest)
				return
			}
			var wrapper struct {
				Encrypt string `xml:"Encrypt"`
			}
			if xml.Unmarshal(body, &wrapper) != nil {
				http.Error(w, "invalid callback XML", http.StatusBadRequest)
				return
			}
			encrypted = strings.TrimSpace(wrapper.Encrypt)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		plain, err := VerifyAndDecryptCallback(config.CallbackToken, config.EncodingAESKey, expectedReceiveID, r.URL.Query(), encrypted)
		if err != nil {
			http.Error(w, "callback verification failed", http.StatusBadRequest)
			return
		}
		if r.Method == http.MethodGet {
			if err := store.BindReceiveID(plain.ReceiveID); err != nil {
				http.Error(w, "callback receive ID conflict", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write(plain.Message)
			return
		}
		eventPath := callbackEventPath(plain.Message)
		sum := sha256.Sum256(plain.Message)
		now := time.Now().UTC()
		if err := store.RecordCallback(CallbackEvidence{ReceivedAt: now, ReceiveID: plain.ReceiveID, EventPath: eventPath, ContentSHA256: hex.EncodeToString(sum[:])}); err != nil {
			http.Error(w, "evidence unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("success"))
	})
}

func NewAdminHandler(config Config, store *EvidenceStore, archive *ArchiveService) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/status", func(w http.ResponseWriter, _ *http.Request) {
		state, err := store.LoadState()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "state unavailable"})
			return
		}
		fingerprint := sha256.Sum256([]byte(config.RSAPublicKey))
		writeJSON(w, http.StatusOK, map[string]any{
			"state": state, "callback_url": strings.TrimRight(config.PublicURL, "/") + "/wecom/callback",
			"rsa_public_key_sha256":  hex.EncodeToString(fingerprint[:]),
			"active_pull_configured": archive != nil,
		})
	})
	mux.HandleFunc("POST /admin/pull", func(w http.ResponseWriter, r *http.Request) {
		if archive == nil {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "active pull credentials are not configured"})
			return
		}
		result, err := archive.Pull(r.Context())
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, result)
	})
	mux.HandleFunc("POST /work-message/archive/messages", func(w http.ResponseWriter, r *http.Request) {
		if archive == nil {
			writeJSON(w, http.StatusConflict, map[string]any{"errcode": http.StatusConflict, "errmsg": "archive pull is not configured"})
			return
		}
		var input struct {
			CorpID   int    `json:"corp_id"`
			WXCorpID string `json:"wx_corpid"`
			Seq      uint64 `json:"seq"`
			Limit    uint32 `json:"limit"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		if err := decoder.Decode(&input); err != nil || input.CorpID <= 0 || strings.TrimSpace(input.WXCorpID) != strings.TrimSpace(config.CorpID) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"errcode": http.StatusBadRequest, "errmsg": "archive corp binding mismatch"})
			return
		}
		if input.Limit == 0 {
			input.Limit = config.PullLimit
		}
		page, err := archive.FetchPage(r.Context(), input.Seq, input.Limit)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{"errcode": http.StatusBadGateway, "errmsg": sanitizeArchiveError(err)})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"errcode": 0, "errmsg": "ok", "messages": page.Messages})
	})
	mux.HandleFunc("POST /work-message/archive/media", func(w http.ResponseWriter, r *http.Request) {
		if archive == nil {
			writeJSON(w, http.StatusConflict, map[string]any{"errcode": "ARCHIVE_PULL_NOT_CONFIGURED", "errmsg": "archive pull is not configured"})
			return
		}
		var input struct {
			CorpID         int    `json:"corp_id"`
			WXCorpID       string `json:"wx_corpid"`
			SDKFileID      string `json:"sdkFileId"`
			IndexBuf       string `json:"indexBuf"`
			TimeoutSeconds int    `json:"timeoutSeconds"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		if err := decoder.Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"errcode": "ARCHIVE_MEDIA_INVALID_REQUEST", "errmsg": "invalid archive media request"})
			return
		}
		var trailing any
		if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"errcode": "ARCHIVE_MEDIA_INVALID_REQUEST", "errmsg": "invalid archive media request"})
			return
		}
		if input.CorpID <= 0 || strings.TrimSpace(input.WXCorpID) != strings.TrimSpace(config.CorpID) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"errcode": "ARCHIVE_CORP_BINDING_MISMATCH", "errmsg": "archive corp binding mismatch"})
			return
		}
		if strings.TrimSpace(input.SDKFileID) == "" || len(input.SDKFileID) > 4096 || len(input.IndexBuf) > maxMediaIndexLength {
			writeJSON(w, http.StatusBadRequest, map[string]any{"errcode": "ARCHIVE_MEDIA_INVALID_REQUEST", "errmsg": "invalid archive media request"})
			return
		}
		if input.TimeoutSeconds == 0 {
			input.TimeoutSeconds = config.TimeoutSeconds
			if input.TimeoutSeconds == 0 {
				input.TimeoutSeconds = 5
			}
		}
		if input.TimeoutSeconds < 1 || input.TimeoutSeconds > 30 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"errcode": "ARCHIVE_MEDIA_INVALID_REQUEST", "errmsg": "invalid archive media request"})
			return
		}
		chunk, err := archive.FetchMediaWithTimeout(r.Context(), input.SDKFileID, input.IndexBuf, input.TimeoutSeconds)
		if err != nil {
			code, status := sanitizeArchiveMediaError(err)
			writeJSON(w, status, map[string]any{"errcode": code, "errmsg": "archive media request failed"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"errcode": 0, "errmsg": "ok",
			"dataBase64":   base64.StdEncoding.EncodeToString(chunk.Data),
			"nextIndexBuf": chunk.NextIndexBuf, "finished": chunk.Finished,
		})
	})
	return requireBearer(config.AdminToken, mux)
}

func sanitizeArchiveError(err error) string {
	var sdkErr SDKError
	if errors.As(err, &sdkErr) {
		return fmt.Sprintf("%s failed with code %d", sdkErr.Operation, sdkErr.Code)
	}
	return "archive bridge request failed"
}

func sanitizeArchiveMediaError(err error) (string, int) {
	var sdkErr SDKError
	if errors.As(err, &sdkErr) {
		return "ARCHIVE_MEDIA_SDK_ERROR", http.StatusBadGateway
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "ARCHIVE_MEDIA_TIMEOUT", http.StatusGatewayTimeout
	}
	if errors.Is(err, ErrFinanceSDKCapabilityUnavailable) {
		return "ARCHIVE_MEDIA_CAPABILITY_UNAVAILABLE", http.StatusServiceUnavailable
	}
	return "ARCHIVE_MEDIA_REQUEST_FAILED", http.StatusBadGateway
}

func requireBearer(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(token) < 40 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if len(got) != len(token) || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func callbackEventPath(value []byte) string {
	decoder := xml.NewDecoder(strings.NewReader(string(value)))
	fields := map[string]string{}
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "invalid_xml"
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local == "xml" {
			continue
		}
		var field string
		if decoder.DecodeElement(&field, &start) == nil {
			fields[start.Name.Local] = strings.TrimSpace(field)
		}
	}
	parts := []string{fields["MsgType"], fields["Event"], fields["ChangeType"]}
	if parts[2] == "" {
		parts[2] = fields["EventKey"]
	}
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			result = append(result, part)
		}
	}
	if len(result) == 0 {
		return "unknown"
	}
	return strings.Join(result, ".")
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func RunServers(ctx context.Context, config Config, publicHandler, adminHandler http.Handler) error {
	publicServer := newHTTPServer(config.PublicAddr, publicHandler)
	adminServer := newHTTPServer(config.AdminAddr, adminHandler)
	errorsChannel := make(chan error, 2)
	go func() { errorsChannel <- publicServer.ListenAndServe() }()
	go func() { errorsChannel <- adminServer.ListenAndServe() }()
	select {
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = publicServer.Shutdown(shutdown)
		_ = adminServer.Shutdown(shutdown)
		return nil
	case err := <-errorsChannel:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr: addr, Handler: handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
}
