package dashboard

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

type WeWorkCallbackCorp struct {
	TenantID       int
	ID             int
	WxCorpID       string
	Token          string
	EncodingAESKey string
}

type WeWorkCallbackStore interface {
	WeWorkCallbackCorpByID(ctx context.Context, corpID int) (WeWorkCallbackCorp, bool, error)
	WeWorkCallbackCorpByWXID(ctx context.Context, wxCorpID string) (WeWorkCallbackCorp, bool, error)
	WeWorkCallbackInboxStore
}

type WeWorkCallbackEvent struct {
	TenantID   int               `json:"tenantId"`
	CorpID     int               `json:"corpId"`
	WxCorpID   string            `json:"wxCorpId"`
	EventPath  string            `json:"eventPath"`
	Message    map[string]string `json:"message"`
	RawXML     string            `json:"rawXml,omitempty"`
	ReceivedAt string            `json:"receivedAt"`
	EventKey   string            `json:"-"`
	LeaseFence uint64            `json:"-"`
}

type WeWorkCallbackHandler struct {
	store             WeWorkCallbackStore
	wakeup            WeWorkCallbackWakeup
	now               func() time.Time
	acceptanceTimeout time.Duration
}

func NewWeWorkCallbackHandler(store WeWorkCallbackStore, wakeup WeWorkCallbackWakeup) *WeWorkCallbackHandler {
	return &WeWorkCallbackHandler{store: store, wakeup: wakeup, now: time.Now, acceptanceTimeout: 3 * time.Second}
}

func (h *WeWorkCallbackHandler) WithAcceptanceTimeout(timeout time.Duration) *WeWorkCallbackHandler {
	if timeout > 0 {
		h.acceptanceTimeout = timeout
	}
	return h
}

func (h *WeWorkCallbackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.acceptanceTimeout > 0 {
		ctx, cancel := context.WithTimeout(r.Context(), h.acceptanceTimeout)
		defer cancel()
		r = r.WithContext(ctx)
	}
	switch r.Method {
	case http.MethodGet:
		h.verifyURL(w, r)
	case http.MethodPost:
		h.receiveEvent(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *WeWorkCallbackHandler) verifyURL(w http.ResponseWriter, r *http.Request) {
	encrypted := strings.TrimSpace(r.URL.Query().Get("echostr"))
	if encrypted == "" {
		http.Error(w, "missing echostr", http.StatusBadRequest)
		return
	}
	corp, ok, err := h.callbackCorp(r, "")
	if err != nil {
		http.Error(w, "callback store unavailable", http.StatusServiceUnavailable)
		return
	}
	if !ok {
		http.Error(w, "corp not found", http.StatusBadRequest)
		return
	}
	if !weWorkCallbackSignatureOK(corp.Token, r.URL.Query().Get("timestamp"), r.URL.Query().Get("nonce"), encrypted, r.URL.Query().Get("msg_signature")) {
		http.Error(w, "invalid signature", http.StatusBadRequest)
		return
	}
	plain, err := decryptWeWorkCallback(corp.EncodingAESKey, encrypted, corp.WxCorpID)
	if err != nil {
		http.Error(w, "decrypt failed", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(plain)
}

func (h *WeWorkCallbackHandler) receiveEvent(w http.ResponseWriter, r *http.Request) {
	if !weWorkCallbackTimestampFresh(r.URL.Query().Get("timestamp"), h.now(), 10*time.Minute) {
		http.Error(w, "stale timestamp", http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}
	wrapper, err := parseWeWorkEncryptedXML(body)
	if err != nil || wrapper.Encrypt == "" {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	corp, ok, err := h.callbackCorp(r, wrapper.ToUserName)
	if err != nil {
		http.Error(w, "callback store unavailable", http.StatusServiceUnavailable)
		return
	}
	if !ok {
		http.Error(w, "corp not found", http.StatusBadRequest)
		return
	}
	if !weWorkCallbackSignatureOK(corp.Token, r.URL.Query().Get("timestamp"), r.URL.Query().Get("nonce"), wrapper.Encrypt, r.URL.Query().Get("msg_signature")) {
		http.Error(w, "invalid signature", http.StatusBadRequest)
		return
	}
	plain, err := decryptWeWorkCallback(corp.EncodingAESKey, wrapper.Encrypt, corp.WxCorpID)
	if err != nil {
		http.Error(w, "decrypt failed", http.StatusBadRequest)
		return
	}
	message, err := parseWeWorkMessageXML(plain)
	if err != nil {
		http.Error(w, "invalid message", http.StatusBadRequest)
		return
	}
	event := WeWorkCallbackEvent{
		TenantID:   corp.TenantID,
		CorpID:     corp.ID,
		WxCorpID:   corp.WxCorpID,
		EventPath:  weWorkEventPath(message),
		Message:    normalizedWeWorkCallbackMessage(message),
		ReceivedAt: h.now().Format("2006-01-02 15:04:05"),
	}
	_, err = h.store.AcceptWeWorkCallback(r.Context(), event, WeWorkCallbackEventKey(event), WeWorkCallbackPayloadFingerprint(event))
	if errors.Is(err, ErrWeWorkCallbackConflict) {
		http.Error(w, "callback payload conflicts with accepted event", http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(w, "callback store unavailable", http.StatusServiceUnavailable)
		return
	}
	if h.wakeup != nil {
		wakeupCtx, cancel := context.WithTimeout(r.Context(), 50*time.Millisecond)
		_ = h.wakeup.WakeWeWorkCallback(wakeupCtx)
		cancel()
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("success"))
}

func (h *WeWorkCallbackHandler) callbackCorp(r *http.Request, wxCorpID string) (WeWorkCallbackCorp, bool, error) {
	if h.store == nil {
		return WeWorkCallbackCorp{}, false, errors.New("callback store is not configured")
	}
	ctx := r.Context()
	if corpID := positiveStringInt(r.URL.Query().Get("cid")); corpID > 0 {
		corp, found, err := h.store.WeWorkCallbackCorpByID(ctx, corpID)
		return corp, found, err
	}
	if wxCorpID == "" {
		wxCorpID = r.URL.Query().Get("ToUserName")
	}
	if strings.TrimSpace(wxCorpID) == "" {
		return WeWorkCallbackCorp{}, false, nil
	}
	corp, found, err := h.store.WeWorkCallbackCorpByWXID(ctx, strings.TrimSpace(wxCorpID))
	return corp, found, err
}

func weWorkCallbackTimestampFresh(raw string, now time.Time, tolerance time.Duration) bool {
	seconds, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || seconds <= 0 || tolerance <= 0 {
		return false
	}
	delta := now.Sub(time.Unix(seconds, 0))
	return delta >= -tolerance && delta <= tolerance
}

type weWorkEncryptedXML struct {
	XMLName    xml.Name `xml:"xml"`
	ToUserName string   `xml:"ToUserName"`
	Encrypt    string   `xml:"Encrypt"`
}

func parseWeWorkEncryptedXML(body []byte) (weWorkEncryptedXML, error) {
	var wrapper weWorkEncryptedXML
	if err := xml.Unmarshal(body, &wrapper); err != nil {
		return weWorkEncryptedXML{}, err
	}
	return wrapper, nil
}

func parseWeWorkMessageXML(body []byte) (map[string]string, error) {
	decoder := xml.NewDecoder(bytes.NewReader(body))
	message := map[string]string{}
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local == "xml" {
			continue
		}
		var value string
		if err := decoder.DecodeElement(&value, &start); err != nil {
			return nil, err
		}
		message[start.Name.Local] = value
	}
	if len(message) == 0 {
		return nil, fmt.Errorf("empty message")
	}
	return message, nil
}

func weWorkEventPath(message map[string]string) string {
	parts := make([]string, 0, 3)
	if value := strings.TrimSpace(message["MsgType"]); value != "" {
		parts = append(parts, value)
	}
	if value := strings.TrimSpace(message["Event"]); value != "" {
		parts = append(parts, value)
	}
	if value := strings.TrimSpace(message["ChangeType"]); value != "" {
		parts = append(parts, value)
	} else if value := strings.TrimSpace(message["EventKey"]); value != "" {
		parts = append(parts, value)
	}
	return strings.Join(parts, ".")
}

func decryptWeWorkCallback(encodingAESKey string, encrypted string, expectedReceiveID string) ([]byte, error) {
	aesKey, err := decodeWeWorkAESKey(encodingAESKey)
	if err != nil {
		return nil, err
	}
	cipherText, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return nil, err
	}
	if len(cipherText) == 0 || len(cipherText)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("invalid ciphertext length")
	}
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, err
	}
	plain := make([]byte, len(cipherText))
	cipher.NewCBCDecrypter(block, aesKey[:aes.BlockSize]).CryptBlocks(plain, cipherText)
	plain, err = pkcs7Unpad(plain, aes.BlockSize)
	if err != nil {
		return nil, err
	}
	if len(plain) < 20 {
		return nil, fmt.Errorf("invalid plaintext length")
	}
	msgLen := int(binary.BigEndian.Uint32(plain[16:20]))
	if msgLen < 0 || 20+msgLen > len(plain) {
		return nil, fmt.Errorf("invalid message length")
	}
	message := plain[20 : 20+msgLen]
	receiveID := string(plain[20+msgLen:])
	if expectedReceiveID != "" && receiveID != "" && receiveID != expectedReceiveID {
		return nil, fmt.Errorf("receive id mismatch")
	}
	return message, nil
}

func decodeWeWorkAESKey(encodingAESKey string) ([]byte, error) {
	key := strings.TrimSpace(encodingAESKey)
	if len(key) != 43 {
		return nil, fmt.Errorf("invalid encoding aes key")
	}
	decoded, err := base64.StdEncoding.DecodeString(key + "=")
	if err != nil {
		return nil, err
	}
	if len(decoded) != 32 {
		return nil, fmt.Errorf("invalid aes key length")
	}
	return decoded, nil
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, fmt.Errorf("invalid padding size")
	}
	padding := int(data[len(data)-1])
	if padding == 0 || padding > blockSize || padding > len(data) {
		return nil, fmt.Errorf("invalid padding")
	}
	for _, value := range data[len(data)-padding:] {
		if int(value) != padding {
			return nil, fmt.Errorf("invalid padding")
		}
	}
	return data[:len(data)-padding], nil
}

func weWorkCallbackSignatureOK(token string, timestamp string, nonce string, encrypted string, got string) bool {
	got = strings.TrimSpace(got)
	if got == "" {
		return false
	}
	items := []string{token, timestamp, nonce, encrypted}
	sort.Strings(items)
	sum := sha1.Sum([]byte(strings.Join(items, "")))
	return hex.EncodeToString(sum[:]) == got
}

func positiveStringInt(value string) int {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n <= 0 {
		return 0
	}
	return n
}
