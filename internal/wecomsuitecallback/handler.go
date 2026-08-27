package wecomsuitecallback

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/archivefixture"
	"jiyi/mochat-go/internal/wecomarchivedemo"
)

type Config struct {
	SuiteID        string
	SuiteSecret    string
	CallbackToken  string
	EncodingAESKey string
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
}

func NewHandler(config Config, store Store, exchanger AuthorizationExchanger) (*Handler, error) {
	config.SuiteID = strings.TrimSpace(config.SuiteID)
	config.SuiteSecret = strings.TrimSpace(config.SuiteSecret)
	config.CallbackToken = strings.TrimSpace(config.CallbackToken)
	config.EncodingAESKey = strings.TrimSpace(config.EncodingAESKey)
	if config.SuiteID == "" || config.SuiteSecret == "" || config.CallbackToken == "" || len(config.EncodingAESKey) != 43 || store == nil || exchanger == nil {
		return nil, errors.New("WeCom suite callback configuration is invalid")
	}
	return &Handler{config: config, store: store, exchanger: exchanger, now: time.Now}, nil
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
		http.Error(w, "callback verification failed", http.StatusBadRequest)
		return
	}
	if request.Method == http.MethodGet {
		encrypted := strings.TrimSpace(request.URL.Query().Get("echostr"))
		plain, verifyErr := wecomarchivedemo.VerifyAndDecryptCallback(h.config.CallbackToken, h.config.EncodingAESKey, h.config.SuiteID, request.URL.Query(), encrypted)
		if verifyErr != nil {
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
		http.Error(w, "callback verification failed", http.StatusBadRequest)
		return
	}
	var wrapper struct {
		Encrypt string `xml:"Encrypt"`
	}
	if xml.Unmarshal(body, &wrapper) != nil || strings.TrimSpace(wrapper.Encrypt) == "" {
		http.Error(w, "callback verification failed", http.StatusBadRequest)
		return
	}
	plain, err := wecomarchivedemo.VerifyAndDecryptCallback(h.config.CallbackToken, h.config.EncodingAESKey, h.config.SuiteID, request.URL.Query(), wrapper.Encrypt)
	if err != nil {
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
		http.Error(w, "callback verification failed", http.StatusBadRequest)
		return
	}
	event.InfoType = strings.TrimSpace(event.InfoType)
	if event.InfoType != "suite_ticket" && event.InfoType != "create_auth" {
		http.Error(w, "callback event is unsupported", http.StatusBadRequest)
		return
	}
	digestRaw := sha256.Sum256([]byte(strings.Join([]string{timestamp, nonce, request.URL.Query().Get("msg_signature"), strings.TrimSpace(wrapper.Encrypt)}, "\x00")))
	digest := hex.EncodeToString(digestRaw[:])
	claimed, err := h.store.ClaimSuiteCallback(request.Context(), h.config.SuiteID, digest, event.InfoType, now)
	if err != nil {
		http.Error(w, "callback persistence failed", http.StatusServiceUnavailable)
		return
	}
	if !claimed {
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
			http.Error(w, "callback persistence failed", http.StatusServiceUnavailable)
			return
		}
	case "create_auth":
		authCode := strings.TrimSpace(event.AuthCode)
		ticket, loadErr := h.store.LoadSuiteTicket(request.Context(), h.config.SuiteID)
		if authCode == "" || loadErr != nil || strings.TrimSpace(ticket) == "" {
			http.Error(w, "callback authorization unavailable", http.StatusServiceUnavailable)
			return
		}
		authorization, exchangeErr := h.exchanger.ExchangeAuthorization(request.Context(), h.config.SuiteID, h.config.SuiteSecret, ticket, authCode)
		if exchangeErr != nil || authorization.TenantID <= 0 || strings.TrimSpace(authorization.CorpID) == "" || strings.TrimSpace(authorization.PermanentCode) == "" {
			log.Printf("WeCom suite callback authorization exchange failed")
			http.Error(w, "callback authorization failed", http.StatusBadGateway)
			return
		}
		if saveErr := h.store.SaveSuiteAuthorization(request.Context(), h.config.SuiteID, authorization); saveErr != nil {
			log.Printf("WeCom suite callback authorization persistence failed: %v", saveErr)
			http.Error(w, "callback authorization failed", http.StatusBadGateway)
			return
		}
	}
	if err := h.store.CompleteSuiteCallback(request.Context(), h.config.SuiteID, digest); err != nil {
		http.Error(w, "callback persistence failed", http.StatusServiceUnavailable)
		return
	}
	completed = true
	writeSuccess(w)
}

func writeSuccess(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("success"))
}
