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
	ID             int
	WxCorpID       string
	Token          string
	EncodingAESKey string
}

type WeWorkCallbackStore interface {
	WeWorkCallbackCorpByID(ctx context.Context, corpID int) (WeWorkCallbackCorp, bool, error)
	WeWorkCallbackCorpByWXID(ctx context.Context, wxCorpID string) (WeWorkCallbackCorp, bool, error)
}

type WeWorkCallbackQueue interface {
	EnqueueWeWorkCallback(ctx context.Context, event WeWorkCallbackEvent) error
}

type WeWorkCallbackEvent struct {
	CorpID     int               `json:"corpId"`
	WxCorpID   string            `json:"wxCorpId"`
	EventPath  string            `json:"eventPath"`
	Message    map[string]string `json:"message"`
	RawXML     string            `json:"rawXml"`
	ReceivedAt string            `json:"receivedAt"`
}

type WeWorkCallbackDelivery struct {
	Event    WeWorkCallbackEvent
	Raw      string
	Attempts int
}

type WeWorkCallbackHandler struct {
	store WeWorkCallbackStore
	queue WeWorkCallbackQueue
	now   func() time.Time
}

func NewWeWorkCallbackHandler(store WeWorkCallbackStore, queue WeWorkCallbackQueue) *WeWorkCallbackHandler {
	return &WeWorkCallbackHandler{store: store, queue: queue, now: time.Now}
}

func (h *WeWorkCallbackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	corp, ok := h.callbackCorp(r, "")
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
	corp, ok := h.callbackCorp(r, wrapper.ToUserName)
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
	if h.queue != nil {
		_ = h.queue.EnqueueWeWorkCallback(r.Context(), WeWorkCallbackEvent{
			CorpID:     corp.ID,
			WxCorpID:   corp.WxCorpID,
			EventPath:  weWorkEventPath(message),
			Message:    message,
			RawXML:     string(plain),
			ReceivedAt: h.now().Format("2006-01-02 15:04:05"),
		})
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("success"))
}

func (h *WeWorkCallbackHandler) callbackCorp(r *http.Request, wxCorpID string) (WeWorkCallbackCorp, bool) {
	if h.store == nil {
		return WeWorkCallbackCorp{}, false
	}
	ctx := r.Context()
	if corpID := positiveStringInt(r.URL.Query().Get("cid")); corpID > 0 {
		corp, found, err := h.store.WeWorkCallbackCorpByID(ctx, corpID)
		return corp, found && err == nil
	}
	if wxCorpID == "" {
		wxCorpID = r.URL.Query().Get("ToUserName")
	}
	if strings.TrimSpace(wxCorpID) == "" {
		return WeWorkCallbackCorp{}, false
	}
	corp, found, err := h.store.WeWorkCallbackCorpByWXID(ctx, strings.TrimSpace(wxCorpID))
	return corp, found && err == nil
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
