package archivebridge

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"jiyi/mochat-go/internal/archivefixture"
)

const maxRequestBody = 64 << 10

type Config struct {
	BearerToken       string
	FixtureEnabled    bool
	FixtureAdminToken string
	FixtureManager    *FixtureManager
}

type Handler struct {
	config Config
	store  *Store
}

func NewHandler(config Config, store *Store) (http.Handler, error) {
	config.BearerToken = strings.TrimSpace(config.BearerToken)
	config.FixtureAdminToken = strings.TrimSpace(config.FixtureAdminToken)
	if len(config.BearerToken) < 40 || store == nil {
		return nil, errors.New("archive bridge configuration is invalid")
	}
	if config.FixtureEnabled && (len(config.FixtureAdminToken) < 40 || config.FixtureManager == nil) {
		return nil, errors.New("archive bridge fixture configuration is invalid")
	}
	handler := &Handler{config: config, store: store}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handler.health)
	mux.HandleFunc("POST /v1/archive/messages", handler.messages)
	mux.HandleFunc("POST /v1/archive/media/chunks", handler.media)
	mux.HandleFunc("POST /v1/archive/component/session", handler.component)
	if config.FixtureEnabled {
		mux.HandleFunc("POST /v1/fixture/seed", handler.fixtureSeed)
		mux.HandleFunc("POST /v1/fixture/send", handler.fixtureSend)
		mux.HandleFunc("POST /v1/fixture/status", handler.fixtureStatus)
		mux.HandleFunc("POST /v1/fixture/cleanup", handler.fixtureCleanup)
	}
	// Keep the established app contract during the migration window. These
	// aliases still require the explicit integration_mode field.
	mux.HandleFunc("POST /work-message/archive/messages", handler.messages)
	mux.HandleFunc("POST /work-message/archive/media", handler.media)
	return handler.requireBearer(mux), nil
}

type fixtureSeedRequest struct {
	Binding Binding `json:"binding"`
	Dataset string  `json:"dataset"`
}

func (h *Handler) fixtureSeed(w http.ResponseWriter, r *http.Request) {
	var input fixtureSeedRequest
	if decodeStrict(w, r, &input) != nil {
		writeBridgeError(w, http.StatusBadRequest, "FIXTURE_REQUEST_INVALID")
		return
	}
	status, err := h.config.FixtureManager.Seed(input.Binding, input.Dataset)
	if err != nil {
		writeBridgeError(w, fixtureErrorStatus(err), ErrorCode(err))
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *Handler) fixtureSend(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Binding    Binding `json:"binding"`
		Dataset    string  `json:"dataset"`
		Type       string  `json:"type"`
		Text       string  `json:"text,omitempty"`
		DataBase64 string  `json:"dataBase64,omitempty"`
		FileName   string  `json:"fileName,omitempty"`
		MIMEType   string  `json:"mimeType,omitempty"`
	}
	if decodeStrict(w, r, &input) != nil {
		writeBridgeError(w, http.StatusBadRequest, "FIXTURE_REQUEST_INVALID")
		return
	}
	result, err := h.config.FixtureManager.Send(FixtureSendInput{Binding: input.Binding, Dataset: input.Dataset, Type: input.Type, Text: input.Text, DataBase64: input.DataBase64, FileName: input.FileName, MIMEType: input.MIMEType})
	if err != nil {
		writeBridgeError(w, fixtureErrorStatus(err), ErrorCode(err))
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) fixtureStatus(w http.ResponseWriter, r *http.Request) {
	var input struct{}
	if decodeStrict(w, r, &input) != nil {
		writeBridgeError(w, http.StatusBadRequest, "FIXTURE_REQUEST_INVALID")
		return
	}
	writeJSON(w, http.StatusOK, h.config.FixtureManager.Status())
}

func (h *Handler) fixtureCleanup(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Binding        Binding `json:"binding"`
		Dataset        string  `json:"dataset"`
		ConfirmDataset string  `json:"confirmDataset"`
	}
	if decodeStrict(w, r, &input) != nil {
		writeBridgeError(w, http.StatusBadRequest, "FIXTURE_REQUEST_INVALID")
		return
	}
	status, err := h.config.FixtureManager.Cleanup(input.Binding, input.Dataset, input.ConfirmDataset)
	if err != nil {
		writeBridgeError(w, fixtureErrorStatus(err), ErrorCode(err))
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func fixtureErrorStatus(err error) int {
	if ErrorCode(err) == "FIXTURE_DATASET_NOT_FOUND" {
		return http.StatusNotFound
	}
	if ErrorCode(err) == "FIXTURE_BINDING_CONFLICT" {
		return http.StatusConflict
	}
	return http.StatusBadRequest
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.store.Status())
}

type messageRequest struct {
	Binding
	Seq   int64 `json:"seq"`
	Limit int   `json:"limit"`
}

func (h *Handler) messages(w http.ResponseWriter, r *http.Request) {
	var input messageRequest
	if err := decodeStrict(w, r, &input); err != nil || input.Seq < 0 {
		writeBridgeError(w, http.StatusBadRequest, "ARCHIVE_REQUEST_INVALID")
		return
	}
	driver, err := h.store.Resolve(input.Binding)
	if err != nil {
		writeBridgeError(w, http.StatusConflict, ErrorCode(err))
		return
	}
	if input.Limit <= 0 || input.Limit > 1000 {
		input.Limit = 100
	}
	if driver.Finance != nil {
		page, err := driver.Finance.FetchPage(r.Context(), uint64(input.Seq), uint32(input.Limit))
		if err != nil {
			writeBridgeError(w, http.StatusBadGateway, "ARCHIVE_UPSTREAM_FAILED")
			return
		}
		messages := make([]json.RawMessage, 0, len(page.Messages))
		for _, raw := range page.Messages {
			var object map[string]any
			if json.Unmarshal(raw, &object) != nil {
				writeBridgeError(w, http.StatusBadGateway, "ARCHIVE_UPSTREAM_INVALID")
				return
			}
			object["content_policy"] = "plaintext"
			object["source_mode"] = ModeSelfBuilt
			normalized, _ := json.Marshal(object)
			messages = append(messages, normalized)
		}
		writeJSON(w, http.StatusOK, map[string]any{"errcode": 0, "messages": messages})
		return
	}
	items, err := driver.DataZone.Fetch(input.WXCorpID, input.Seq, input.Limit)
	if err != nil {
		writeBridgeError(w, http.StatusBadGateway, archivefixture.ErrorCode(err))
		return
	}
	messages := make([]map[string]any, 0, len(items))
	for _, item := range items {
		messages = append(messages, map[string]any{
			"seq": item.Sequence, "msgid": item.MessageID, "msgtype": item.Type, "from": item.Sender,
			"tolist": item.Receivers, "roomid": item.RoomID, "msgtime": item.SentAt.UnixMilli(),
			"content_policy": "component", "source_mode": ModeThirdPartyDelegated,
			"component_locator": map[string]any{"msgid": item.MessageID, "public_key_ver": item.PublicKeyVersion, "encrypted_secret_key": item.EncryptedSecretKey},
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"errcode": 0, "messages": messages})
}

type mediaRequest struct {
	Binding
	SDKFileID      string `json:"sdkFileId"`
	IndexBuf       string `json:"indexBuf"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
}

func (h *Handler) media(w http.ResponseWriter, r *http.Request) {
	var input mediaRequest
	if err := decodeStrict(w, r, &input); err != nil || strings.TrimSpace(input.SDKFileID) == "" || len(input.IndexBuf) > 4096 {
		writeBridgeError(w, http.StatusBadRequest, "ARCHIVE_MEDIA_INVALID_REQUEST")
		return
	}
	driver, err := h.store.Resolve(input.Binding)
	if err != nil {
		writeBridgeError(w, http.StatusConflict, ErrorCode(err))
		return
	}
	if driver.Finance == nil {
		writeBridgeError(w, http.StatusConflict, "ARCHIVE_MEDIA_MODE_UNAVAILABLE")
		return
	}
	if input.TimeoutSeconds <= 0 || input.TimeoutSeconds > 30 {
		input.TimeoutSeconds = 5
	}
	chunk, err := driver.Finance.FetchMediaWithTimeout(r.Context(), input.SDKFileID, input.IndexBuf, input.TimeoutSeconds)
	if err != nil {
		writeBridgeError(w, http.StatusBadGateway, "ARCHIVE_MEDIA_UPSTREAM_FAILED")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"errcode": 0, "dataBase64": base64.StdEncoding.EncodeToString(chunk.Data), "nextIndexBuf": chunk.NextIndexBuf, "finished": chunk.Finished})
}

type componentRequest struct {
	Binding
	MessageID          string `json:"msgid"`
	PublicKeyVersion   uint32 `json:"public_key_ver"`
	EncryptedSecretKey string `json:"encrypted_secret_key"`
}

func (h *Handler) component(w http.ResponseWriter, r *http.Request) {
	var input componentRequest
	if err := decodeStrict(w, r, &input); err != nil || strings.TrimSpace(input.MessageID) == "" || input.PublicKeyVersion == 0 || strings.TrimSpace(input.EncryptedSecretKey) == "" {
		writeBridgeError(w, http.StatusBadRequest, "ARCHIVE_COMPONENT_INVALID_REQUEST")
		return
	}
	driver, err := h.store.Resolve(input.Binding)
	if err != nil {
		writeBridgeError(w, http.StatusConflict, ErrorCode(err))
		return
	}
	if driver.DataZone == nil {
		writeBridgeError(w, http.StatusConflict, "ARCHIVE_COMPONENT_MODE_UNAVAILABLE")
		return
	}
	metadata := archivefixture.DataZoneMessage{MessageID: input.MessageID, PublicKeyVersion: input.PublicKeyVersion, EncryptedSecretKey: input.EncryptedSecretKey}
	secret, err := driver.DataZone.DecryptSecretKey(metadata)
	if err != nil {
		writeBridgeError(w, http.StatusNotFound, "ARCHIVE_COMPONENT_UNAVAILABLE")
		return
	}
	content, err := driver.DataZone.Render(input.WXCorpID, input.MessageID, secret)
	if err != nil {
		writeBridgeError(w, http.StatusNotFound, "ARCHIVE_COMPONENT_UNAVAILABLE")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"errcode": 0, "msgtype": content.Type, "fileName": content.FileName, "mimeType": content.MIMEType, "dataBase64": base64.StdEncoding.EncodeToString(content.Body)})
}

func (h *Handler) requireBearer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		expected := h.config.BearerToken
		if strings.HasPrefix(r.URL.Path, "/v1/fixture/") {
			expected = h.config.FixtureAdminToken
		}
		provided := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		if len(provided) != len(expected) || subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
			writeBridgeError(w, http.StatusUnauthorized, "ARCHIVE_UNAUTHORIZED")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func decodeStrict(w http.ResponseWriter, r *http.Request, output any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBody))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(output); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("invalid trailing input")
	}
	return nil
}

func writeBridgeError(w http.ResponseWriter, status int, code string) {
	if strings.TrimSpace(code) == "" {
		code = "ARCHIVE_REQUEST_FAILED"
	}
	writeJSON(w, status, map[string]any{"errcode": code, "errmsg": "archive bridge request failed"})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
