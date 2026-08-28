package wecomsuitecallback

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/archivefixture"
	"jiyi/mochat-go/internal/observability"
	"jiyi/mochat-go/internal/wecomarchivedemo"
)

type Config struct {
	SuiteID        string
	SuiteSecret    string
	CallbackToken  string
	EncodingAESKey string
	Logger         *slog.Logger
}

type Store interface {
	ClaimSuiteCallback(context.Context, string, string, string, time.Time) (bool, error)
	CompleteSuiteCallback(context.Context, string, string) error
	FailSuiteCallback(context.Context, string, string) error
	SaveSuiteTicket(context.Context, string, string, time.Time) error
	LoadSuiteTicket(context.Context, string) (string, error)
	SaveSuiteAuthorization(context.Context, string, archivefixture.SuiteAuthorization) error
}

type AuthorizationExchanger interface {
	ExchangeAuthorization(context.Context, string, string, string, string) (archivefixture.SuiteAuthorization, error)
}

type Handler struct {
	config    Config
	store     Store
	exchanger AuthorizationExchanger
	now       func() time.Time
	logger    *slog.Logger
}

func NewHandler(config Config, store Store, exchanger AuthorizationExchanger) (*Handler, error) {
	config.SuiteID = strings.TrimSpace(config.SuiteID)
	config.SuiteSecret = strings.TrimSpace(config.SuiteSecret)
	config.CallbackToken = strings.TrimSpace(config.CallbackToken)
	config.EncodingAESKey = strings.TrimSpace(config.EncodingAESKey)
	if config.SuiteID == "" || config.SuiteSecret == "" || config.CallbackToken == "" || len(config.EncodingAESKey) != 43 || store == nil || exchanger == nil {
		return nil, errors.New("WeCom suite callback configuration is invalid")
	}
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{config: config, store: store, exchanger: exchanger, now: time.Now, logger: logger}, nil
}

func (h *Handler) WithClock(now func() time.Time) *Handler {
	if h != nil && now != nil {
		h.now = now
	}
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, request *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if h == nil || request == nil || (request.Method != http.MethodGet && request.Method != http.MethodPost) {
		http.NotFound(w, request)
		return
	}
	timestamp := strings.TrimSpace(request.URL.Query().Get("timestamp"))
	nonce := strings.TrimSpace(request.URL.Query().Get("nonce"))
	unixTime, err := strconv.ParseInt(timestamp, 10, 64)
	now := h.now().UTC()
	callbackTime := time.Unix(unixTime, 0).UTC()
	if err != nil || nonce == "" || callbackTime.Before(now.Add(-5*time.Minute)) || callbackTime.After(now.Add(5*time.Minute)) {
		h.logRejected(request.Context(), "unknown", "WECOM_CALLBACK_TIMESTAMP_INVALID")
		http.Error(w, "callback verification failed", http.StatusBadRequest)
		return
	}
	if request.Method == http.MethodGet {
		encrypted := strings.TrimSpace(request.URL.Query().Get("echostr"))
		plain, verifyErr := wecomarchivedemo.VerifyAndDecryptCallback(h.config.CallbackToken, h.config.EncodingAESKey, h.config.SuiteID, request.URL.Query(), encrypted)
		if verifyErr != nil {
			h.logRejected(request.Context(), "challenge", "WECOM_CALLBACK_SIGNATURE_INVALID")
			http.Error(w, "callback verification failed", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(plain.Message)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, request.Body, 64<<10))
	if err != nil {
		h.logRejected(request.Context(), "unknown", "WECOM_CALLBACK_BODY_INVALID")
		http.Error(w, "callback verification failed", http.StatusBadRequest)
		return
	}
	var wrapper struct {
		Encrypt string `xml:"Encrypt"`
	}
	if xml.Unmarshal(body, &wrapper) != nil || strings.TrimSpace(wrapper.Encrypt) == "" {
		h.logRejected(request.Context(), "unknown", "WECOM_CALLBACK_ENVELOPE_INVALID")
		http.Error(w, "callback verification failed", http.StatusBadRequest)
		return
	}
	plain, err := wecomarchivedemo.VerifyAndDecryptCallback(h.config.CallbackToken, h.config.EncodingAESKey, h.config.SuiteID, request.URL.Query(), wrapper.Encrypt)
	if err != nil {
		h.logRejected(request.Context(), "unknown", "WECOM_CALLBACK_SIGNATURE_INVALID")
		http.Error(w, "callback verification failed", http.StatusBadRequest)
		return
	}
	var event struct {
		SuiteID     string `xml:"SuiteId"`
		InfoType    string `xml:"InfoType"`
		SuiteTicket string `xml:"SuiteTicket"`
		AuthCode    string `xml:"AuthCode"`
	}
	if xml.Unmarshal(plain.Message, &event) != nil || strings.TrimSpace(event.SuiteID) != h.config.SuiteID {
		h.logRejected(request.Context(), "unknown", "WECOM_CALLBACK_PAYLOAD_INVALID")
		http.Error(w, "callback verification failed", http.StatusBadRequest)
		return
	}
	event.InfoType = strings.TrimSpace(event.InfoType)
	if event.InfoType != "suite_ticket" && event.InfoType != "create_auth" {
		h.logRejected(request.Context(), event.InfoType, "WECOM_CALLBACK_EVENT_UNSUPPORTED")
		http.Error(w, "callback event is unsupported", http.StatusBadRequest)
		return
	}
	digestRaw := sha256.Sum256([]byte(strings.Join([]string{timestamp, nonce, request.URL.Query().Get("msg_signature"), strings.TrimSpace(wrapper.Encrypt)}, "\x00")))
	digest := hex.EncodeToString(digestRaw[:])
	claimed, err := h.store.ClaimSuiteCallback(request.Context(), h.config.SuiteID, digest, event.InfoType, now)
	if err != nil {
		h.logFailure(request.Context(), "wecom_callback_persistence_failed", event.InfoType, "claim", "WECOM_CALLBACK_CLAIM_FAILED")
		http.Error(w, "callback persistence failed", http.StatusServiceUnavailable)
		return
	}
	if !claimed {
		h.logger.DebugContext(request.Context(), "企微第三方回调已处理，本次重复投递跳过",
			"event", "wecom_callback_duplicate", "component", "wecom_callback", "suite_id", h.config.SuiteID,
			"object_type", "wecom_callback_type", "object_id", event.InfoType,
			"request_id", observability.RequestID(request.Context()), "step", "claim", "result", "skipped")
		writeSuccess(w)
		return
	}
	completed := false
	defer func() {
		if !completed {
			_ = h.store.FailSuiteCallback(request.Context(), h.config.SuiteID, digest)
		}
	}()
	switch event.InfoType {
	case "suite_ticket":
		ticket := strings.TrimSpace(event.SuiteTicket)
		if ticket == "" || h.store.SaveSuiteTicket(request.Context(), h.config.SuiteID, ticket, now) != nil {
			h.logFailure(request.Context(), "wecom_callback_persistence_failed", event.InfoType, "save_ticket", "WECOM_SUITE_TICKET_SAVE_FAILED")
			http.Error(w, "callback persistence failed", http.StatusServiceUnavailable)
			return
		}
		h.logger.InfoContext(request.Context(), "企微第三方 suite ticket 已更新；授权失败时请检查 ticket 时效",
			"event", "wecom_suite_ticket_saved", "component", "wecom_callback", "suite_id", h.config.SuiteID,
			"object_type", "wecom_callback_type", "object_id", event.InfoType,
			"request_id", observability.RequestID(request.Context()), "step", "save_ticket", "result", "success")
	case "create_auth":
		authCode := strings.TrimSpace(event.AuthCode)
		ticket, loadErr := h.store.LoadSuiteTicket(request.Context(), h.config.SuiteID)
		if authCode == "" || loadErr != nil || strings.TrimSpace(ticket) == "" {
			h.logFailure(request.Context(), "wecom_authorization_unavailable", event.InfoType, "load_ticket", "WECOM_AUTHORIZATION_INPUT_MISSING")
			http.Error(w, "callback authorization unavailable", http.StatusServiceUnavailable)
			return
		}
		authorization, exchangeErr := h.exchanger.ExchangeAuthorization(request.Context(), h.config.SuiteID, h.config.SuiteSecret, ticket, authCode)
		if exchangeErr != nil || authorization.TenantID <= 0 || strings.TrimSpace(authorization.CorpID) == "" || strings.TrimSpace(authorization.PermanentCode) == "" {
			h.logFailure(request.Context(), "wecom_authorization_exchange_failed", event.InfoType, "exchange_authorization", "WECOM_AUTHORIZATION_EXCHANGE_FAILED")
			http.Error(w, "callback authorization failed", http.StatusBadGateway)
			return
		}
		if saveErr := h.store.SaveSuiteAuthorization(request.Context(), h.config.SuiteID, authorization); saveErr != nil {
			h.logFailure(request.Context(), "wecom_authorization_persistence_failed", event.InfoType, "save_authorization", "WECOM_AUTHORIZATION_SAVE_FAILED")
			http.Error(w, "callback authorization failed", http.StatusBadGateway)
			return
		}
		h.logger.InfoContext(request.Context(), "企微第三方授权已保存；后续请检查企业激活和数据同步",
			"event", "wecom_authorization_saved", "component", "wecom_callback", "suite_id", h.config.SuiteID,
			"tenant_id", authorization.TenantID, "object_type", "wecom_callback_type", "object_id", event.InfoType,
			"request_id", observability.RequestID(request.Context()), "step", "save_authorization", "result", "success")
	}
	if err := h.store.CompleteSuiteCallback(request.Context(), h.config.SuiteID, digest); err != nil {
		h.logFailure(request.Context(), "wecom_callback_persistence_failed", event.InfoType, "complete", "WECOM_CALLBACK_COMPLETE_FAILED")
		http.Error(w, "callback persistence failed", http.StatusServiceUnavailable)
		return
	}
	completed = true
	writeSuccess(w)
}

func (h *Handler) logRejected(ctx context.Context, callbackType string, errorCode string) {
	h.logger.WarnContext(ctx, "企微第三方回调被拒绝；请检查时间窗、验签配置和回调格式",
		"event", "wecom_callback_rejected", "component", "wecom_callback", "suite_id", h.config.SuiteID,
		"object_type", "wecom_callback_type", "object_id", callbackType,
		"request_id", observability.RequestID(ctx), "step", "verify", "result", "rejected", "error_code", errorCode)
}

func (h *Handler) logFailure(ctx context.Context, event string, callbackType string, step string, errorCode string) {
	h.logger.ErrorContext(ctx, "企微第三方回调处理失败；请按步骤检查回调存储和授权接口",
		"event", event, "component", "wecom_callback", "suite_id", h.config.SuiteID,
		"object_type", "wecom_callback_type", "object_id", callbackType,
		"request_id", observability.RequestID(ctx), "step", step, "result", "failed", "error_code", errorCode)
}

func writeSuccess(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("success"))
}
