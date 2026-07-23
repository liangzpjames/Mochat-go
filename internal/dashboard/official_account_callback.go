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
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

type OfficialAccountAuthEventStore interface {
	UpsertOfficialAccountAuthEvent(ctx context.Context, values OfficialAccountAuthorization) (int, error)
	UpsertWeChatComponentVerifyTicket(ctx context.Context, componentAppID string, componentVerifyTicket string, createTime int64) error
}

type OfficialAccountMessageClient interface {
	SendCustomerTextFromAuthCode(ctx context.Context, authCode string, toUser string, content string) error
}

type OfficialAccountCallbackHandler struct {
	store           OfficialAccountAuthEventStore
	messageClient   OfficialAccountMessageClient
	componentAppID  string
	componentSecret string
	componentToken  string
	componentAESKey string
}

func NewOfficialAccountCallbackHandler(store OfficialAccountAuthEventStore, messageClient OfficialAccountMessageClient, componentAppID string, componentSecret string, componentToken string, componentAESKey string) *OfficialAccountCallbackHandler {
	return &OfficialAccountCallbackHandler{
		store:           store,
		messageClient:   messageClient,
		componentAppID:  strings.TrimSpace(componentAppID),
		componentSecret: strings.TrimSpace(componentSecret),
		componentToken:  strings.TrimSpace(componentToken),
		componentAESKey: strings.TrimSpace(componentAESKey),
	}
}

func (h *OfficialAccountCallbackHandler) AuthEventCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.Method == http.MethodGet {
		h.writeEcho(w, r)
		return
	}
	message, err := h.parseCallbackMessage(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	infoType := strings.TrimSpace(message["InfoType"])
	if infoType == "component_verify_ticket" {
		componentAppID := firstNonEmpty(message["AppId"], h.componentAppID)
		componentVerifyTicket := strings.TrimSpace(message["ComponentVerifyTicket"])
		if componentAppID != "" && componentVerifyTicket != "" && h.store != nil {
			if err := h.store.UpsertWeChatComponentVerifyTicket(r.Context(), componentAppID, componentVerifyTicket, int64FromString(message["CreateTime"])); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		writeRaw(w, http.StatusOK, "success")
		return
	}
	authorization := OfficialAccountAuthorization{
		ComponentAppID:  firstNonEmpty(message["AppId"], h.componentAppID),
		ComponentSecret: h.componentSecret,
		ComponentToken:  h.componentToken,
		ComponentAESKey: h.componentAESKey,
		AuthorizerAppID: strings.TrimSpace(message["AuthorizerAppid"]),
		PreAuthCode:     strings.TrimSpace(message["PreAuthCode"]),
		CreateTime:      int64FromString(message["CreateTime"]),
	}
	switch infoType {
	case "authorized":
		authorization.AuthorizedStatus = 1
		authorization.AuthorizationCode = strings.TrimSpace(message["AuthorizationCode"])
	case "updateauthorized":
		authorization.AuthorizedStatus = 2
		authorization.AuthorizationCode = strings.TrimSpace(message["AuthorizationCode"])
	case "unauthorized":
		authorization.AuthorizedStatus = 3
	default:
		writeRaw(w, http.StatusOK, "success")
		return
	}
	if authorization.ComponentAppID == "" || authorization.AuthorizerAppID == "" {
		writeRaw(w, http.StatusOK, "success")
		return
	}
	if h.store != nil {
		if _, err := h.store.UpsertOfficialAccountAuthEvent(r.Context(), authorization); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	writeRaw(w, http.StatusOK, "success")
}

func (h *OfficialAccountCallbackHandler) MessageEventCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.Method == http.MethodGet {
		h.writeEcho(w, r)
		return
	}
	message, err := h.parseCallbackMessage(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	response := h.messageResponse(r.Context(), message)
	if strings.TrimSpace(response) == "" {
		writeRaw(w, http.StatusOK, "")
		return
	}
	writeRaw(w, http.StatusOK, response)
}

func (h *OfficialAccountCallbackHandler) writeEcho(w http.ResponseWriter, r *http.Request) {
	echo := r.URL.Query().Get("echostr")
	if echo == "" {
		writeRaw(w, http.StatusOK, "success")
		return
	}
	if h.encryptedRequest(r) {
		if err := h.validateMessageSignature(r, echo); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		decrypted, err := decryptWeChatPayload(echo, h.componentAESKey, h.componentAppID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		echo = decrypted
	}
	writeRaw(w, http.StatusOK, echo)
}

func (h *OfficialAccountCallbackHandler) messageResponse(ctx context.Context, message map[string]string) string {
	if strings.TrimSpace(message["MsgType"]) != "text" {
		return ""
	}
	content := strings.TrimSpace(message["Content"])
	if isOfficialAccountComponentCase(message["ToUserName"]) {
		if content == "TESTCOMPONENT_MSG_TYPE_TEXT" {
			return content + "_callback"
		}
		if strings.HasPrefix(content, "QUERY_AUTH_CODE") {
			queryAuthCode := strings.TrimPrefix(content, "QUERY_AUTH_CODE:")
			if queryAuthCode == content {
				queryAuthCode = strings.TrimPrefix(content, "QUERY_AUTH_CODE")
			}
			queryAuthCode = strings.TrimSpace(queryAuthCode)
			if queryAuthCode != "" && h.messageClient != nil {
				_ = h.messageClient.SendCustomerTextFromAuthCode(ctx, queryAuthCode, strings.TrimSpace(message["FromUserName"]), queryAuthCode+"_from_api")
			}
			return ""
		}
	}
	return "Hello！"
}

func (h *OfficialAccountCallbackHandler) parseCallbackMessage(r *http.Request) (map[string]string, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	message, err := parseWeChatXML(body)
	if err != nil {
		return nil, err
	}
	if encrypt := strings.TrimSpace(message["Encrypt"]); encrypt != "" {
		if err := h.validateMessageSignature(r, encrypt); err != nil {
			return nil, err
		}
		plain, err := decryptWeChatPayload(encrypt, h.componentAESKey, h.componentAppID)
		if err != nil {
			return nil, err
		}
		message, err = parseWeChatXML([]byte(plain))
		if err != nil {
			return nil, err
		}
	}
	return message, nil
}

func (h *OfficialAccountCallbackHandler) encryptedRequest(r *http.Request) bool {
	return strings.EqualFold(r.URL.Query().Get("encrypt_type"), "aes") || r.URL.Query().Get("msg_signature") != ""
}

func (h *OfficialAccountCallbackHandler) validateMessageSignature(r *http.Request, encrypt string) error {
	if strings.TrimSpace(h.componentToken) == "" {
		return nil
	}
	signature := strings.TrimSpace(r.URL.Query().Get("msg_signature"))
	if signature == "" {
		return fmt.Errorf("msg_signature required")
	}
	if !weChatMessageSignatureValid(h.componentToken, r.URL.Query().Get("timestamp"), r.URL.Query().Get("nonce"), encrypt, signature) {
		return fmt.Errorf("invalid msg_signature")
	}
	return nil
}

func parseWeChatXML(raw []byte) (map[string]string, error) {
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	values := map[string]string{}
	var current string
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch item := token.(type) {
		case xml.StartElement:
			current = item.Name.Local
		case xml.CharData:
			if current != "" {
				text := strings.TrimSpace(string(item))
				if text != "" {
					values[current] = text
				}
			}
		case xml.EndElement:
			if current == item.Name.Local {
				current = ""
			}
		}
	}
	return values, nil
}

func decryptWeChatPayload(encrypted string, encodingAESKey string, expectedAppID string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encodingAESKey) + "=")
	if err != nil {
		return "", err
	}
	if len(key) != 32 {
		return "", fmt.Errorf("invalid encoding aes key")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encrypted))
	if err != nil {
		return "", err
	}
	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return "", fmt.Errorf("invalid encrypted payload")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	plain := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, key[:aes.BlockSize]).CryptBlocks(plain, ciphertext)
	plain, err = pkcs7Unpad(plain, aes.BlockSize)
	if err != nil {
		return "", err
	}
	if len(plain) < 20 {
		return "", fmt.Errorf("invalid decrypted payload")
	}
	length := int(binary.BigEndian.Uint32(plain[16:20]))
	if length < 0 || 20+length > len(plain) {
		return "", fmt.Errorf("invalid decrypted message length")
	}
	message := plain[20 : 20+length]
	appID := strings.TrimSpace(string(plain[20+length:]))
	if strings.TrimSpace(expectedAppID) != "" && appID != "" && appID != strings.TrimSpace(expectedAppID) {
		return "", fmt.Errorf("invalid appid in encrypted payload")
	}
	return string(message), nil
}

func weChatMessageSignatureValid(token string, timestamp string, nonce string, encrypt string, signature string) bool {
	parts := []string{token, timestamp, nonce, encrypt}
	sort.Strings(parts)
	sum := sha1.Sum([]byte(strings.Join(parts, "")))
	return hex.EncodeToString(sum[:]) == strings.TrimSpace(signature)
}

func isOfficialAccountComponentCase(toUserName string) bool {
	switch strings.TrimSpace(toUserName) {
	case "gh_3c884a361561", "gh_c0f28a78b318", "gh_3f222ed8d140", "gh_26128078e9ab", "gh_2b3713f184a6", "gh_8dad206e9538", "gh_905ae9d01059", "gh_393666f1fdf4", "gh_39abb5d4e1b7", "gh_7818dcb60240":
		return true
	default:
		return false
	}
}

func writeRaw(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

func int64FromString(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	var result int64
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return 0
		}
		result = result*10 + int64(ch-'0')
	}
	return result
}
