package archive

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/modules/providers"
)

const (
	bridgeMessagePath       = "/v1/archive/messages"
	bridgeMediaPath         = "/v1/archive/media/chunks"
	bridgeComponentPath     = "/v1/archive/component/session"
	legacyBridgeMessagePath = "/work-message/archive/messages"
	legacyBridgeMediaPath   = "/work-message/archive/media"
	maxBridgeBody           = 16 << 20

	IntegrationModeSelfBuilt           = "self_built"
	IntegrationModeThirdPartyDelegated = "third_party_delegated"
)

var bridgeErrorCodePattern = regexp.MustCompile(`^[A-Z0-9_.-]{1,96}$`)

type MediaChunk struct {
	Data         []byte
	NextIndexBuf string
	Finished     bool
}

type ComponentRequest struct {
	Scope              Scope
	WXCorpID           string
	MessageID          string
	PublicKeyVersion   uint32
	EncryptedSecretKey string
}

type ComponentContent struct {
	Type     string
	FileName string
	MIMEType string
	Data     []byte
}

type MediaFetchError struct{ Code string }

func (e *MediaFetchError) Error() string {
	if e == nil || strings.TrimSpace(e.Code) == "" {
		return "archive media fetch failed"
	}
	return strings.TrimSpace(e.Code)
}

type BridgeArchiveClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

type bridgeHTTPStatusError struct{ status int }

func (e *bridgeHTTPStatusError) Error() string {
	return fmt.Sprintf("archive bridge HTTP status %d", e.status)
}

func NewBridgeArchiveClient(baseURL, token string, client *http.Client) (*BridgeArchiveClient, error) {
	baseURL, token = strings.TrimRight(strings.TrimSpace(baseURL), "/"), strings.TrimSpace(token)
	parsed, err := url.Parse(baseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, errors.New("archive bridge URL is invalid")
	}
	if len(token) < 40 {
		return nil, errors.New("archive bridge bearer is missing or too short")
	}
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &BridgeArchiveClient{baseURL: baseURL, token: token, httpClient: client}, nil
}

type bridgeMessagePage struct {
	Messages []json.RawMessage
}

func (c *BridgeArchiveClient) fetchMessages(ctx context.Context, scope Scope, wxCorpID, mode string, cursor Cursor, limit int) (bridgeMessagePage, error) {
	request := map[string]any{"tenant_id": scope.TenantID, "corp_id": scope.CorpID, "wx_corpid": wxCorpID, "integration_mode": mode, "seq": cursor.Sequence, "limit": limit}
	var response struct {
		ErrCode  int               `json:"errcode"`
		Messages []json.RawMessage `json:"messages"`
		ChatData []json.RawMessage `json:"chatdata"`
	}
	if err := c.postJSON(ctx, bridgeMessagePath, request, &response); err != nil {
		var statusErr *bridgeHTTPStatusError
		if !errors.As(err, &statusErr) || statusErr.status != http.StatusNotFound {
			return bridgeMessagePage{}, err
		}
		response = struct {
			ErrCode  int               `json:"errcode"`
			Messages []json.RawMessage `json:"messages"`
			ChatData []json.RawMessage `json:"chatdata"`
		}{}
		if err := c.postJSON(ctx, legacyBridgeMessagePath, request, &response); err != nil {
			return bridgeMessagePage{}, err
		}
	}
	if response.ErrCode != 0 {
		return bridgeMessagePage{}, errors.New("archive bridge rejected message request")
	}
	if len(response.Messages) == 0 {
		response.Messages = response.ChatData
	}
	return bridgeMessagePage{Messages: response.Messages}, nil
}

func (c *BridgeArchiveClient) FetchMedia(ctx context.Context, scope Scope, wxCorpID, sdkFileID, indexBuf string) (MediaChunk, error) {
	request := map[string]any{
		"tenant_id": scope.TenantID, "corp_id": scope.CorpID, "wx_corpid": wxCorpID, "integration_mode": IntegrationModeSelfBuilt, "sdkFileId": sdkFileID,
		"indexBuf": indexBuf, "timeoutSeconds": 5,
	}
	var response struct {
		ErrCode      json.RawMessage `json:"errcode"`
		DataBase64   string          `json:"dataBase64"`
		NextIndexBuf string          `json:"nextIndexBuf"`
		Finished     bool            `json:"finished"`
	}
	if err := c.postJSON(ctx, bridgeMediaPath, request, &response); err != nil {
		var statusErr *bridgeHTTPStatusError
		if errors.As(err, &statusErr) && statusErr.status == http.StatusNotFound {
			response = struct {
				ErrCode      json.RawMessage `json:"errcode"`
				DataBase64   string          `json:"dataBase64"`
				NextIndexBuf string          `json:"nextIndexBuf"`
				Finished     bool            `json:"finished"`
			}{}
			err = c.postJSON(ctx, legacyBridgeMediaPath, request, &response)
		}
		if err == nil {
			code := rawErrorCode(response.ErrCode)
			if code != "" && code != "0" {
				return MediaChunk{}, &MediaFetchError{Code: code}
			}
			data, decodeErr := base64.StdEncoding.DecodeString(response.DataBase64)
			if decodeErr != nil {
				return MediaChunk{}, &MediaFetchError{Code: "ARCHIVE_MEDIA_INVALID_RESPONSE"}
			}
			return MediaChunk{Data: data, NextIndexBuf: response.NextIndexBuf, Finished: response.Finished}, nil
		}
		code := rawErrorCode(response.ErrCode)
		if errors.As(err, &statusErr) && code != "" && code != "0" {
			return MediaChunk{}, &MediaFetchError{Code: code}
		}
		return MediaChunk{}, err
	}
	code := rawErrorCode(response.ErrCode)
	if code != "" && code != "0" {
		return MediaChunk{}, &MediaFetchError{Code: code}
	}
	data, err := base64.StdEncoding.DecodeString(response.DataBase64)
	if err != nil {
		return MediaChunk{}, &MediaFetchError{Code: "ARCHIVE_MEDIA_INVALID_RESPONSE"}
	}
	return MediaChunk{Data: data, NextIndexBuf: response.NextIndexBuf, Finished: response.Finished}, nil
}

func (c *BridgeArchiveClient) FetchComponent(ctx context.Context, input ComponentRequest) (ComponentContent, error) {
	if !input.Scope.valid() || strings.TrimSpace(input.WXCorpID) == "" || strings.TrimSpace(input.MessageID) == "" || input.PublicKeyVersion == 0 || strings.TrimSpace(input.EncryptedSecretKey) == "" {
		return ComponentContent{}, errors.New("archive component request is invalid")
	}
	request := map[string]any{
		"tenant_id": input.Scope.TenantID, "corp_id": input.Scope.CorpID, "wx_corpid": strings.TrimSpace(input.WXCorpID),
		"integration_mode": IntegrationModeThirdPartyDelegated, "msgid": strings.TrimSpace(input.MessageID),
		"public_key_ver": input.PublicKeyVersion, "encrypted_secret_key": strings.TrimSpace(input.EncryptedSecretKey),
	}
	var response struct {
		ErrCode    json.RawMessage `json:"errcode"`
		Type       string          `json:"msgtype"`
		FileName   string          `json:"fileName"`
		MIMEType   string          `json:"mimeType"`
		DataBase64 string          `json:"dataBase64"`
	}
	if err := c.postJSON(ctx, bridgeComponentPath, request, &response); err != nil {
		return ComponentContent{}, errors.New("archive component request failed")
	}
	if code := rawErrorCode(response.ErrCode); code != "" && code != "0" {
		return ComponentContent{}, errors.New("archive component request rejected")
	}
	data, err := base64.StdEncoding.DecodeString(response.DataBase64)
	if err != nil || len(data) == 0 {
		return ComponentContent{}, errors.New("archive component response is invalid")
	}
	return ComponentContent{Type: strings.ToLower(strings.TrimSpace(response.Type)), FileName: safeMediaName(response.FileName), MIMEType: strings.TrimSpace(response.MIMEType), Data: data}, nil
}

func (c *BridgeArchiveClient) postJSON(ctx context.Context, path string, input, output any) error {
	if c == nil || c.httpClient == nil {
		return errors.New("archive bridge client unavailable")
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return errors.New("archive bridge request creation failed")
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return errors.New("archive bridge request failed")
	}
	defer response.Body.Close()
	limited := &io.LimitedReader{R: response.Body, N: maxBridgeBody + 1}
	body, err := io.ReadAll(limited)
	if err != nil || len(body) > maxBridgeBody {
		return errors.New("archive bridge response is invalid")
	}
	if err := json.Unmarshal(body, output); err != nil {
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return &bridgeHTTPStatusError{status: response.StatusCode}
		}
		return errors.New("archive bridge response is invalid")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &bridgeHTTPStatusError{status: response.StatusCode}
	}
	return nil
}

func rawErrorCode(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		text = strings.TrimSpace(text)
		if bridgeErrorCodePattern.MatchString(text) {
			return text
		}
		return "ARCHIVE_MEDIA_REQUEST_FAILED"
	}
	var number int
	if json.Unmarshal(raw, &number) == nil {
		return strconv.Itoa(number)
	}
	return "ARCHIVE_MEDIA_INVALID_RESPONSE"
}

type BridgeSource struct {
	client          *BridgeArchiveClient
	scope           Scope
	wxCorpID        string
	integrationMode string
	sourceID        string
}

var _ ArchiveSource = (*BridgeSource)(nil)

func NewBridgeSource(client *BridgeArchiveClient, scope Scope, wxCorpID, integrationMode string) (*BridgeSource, error) {
	wxCorpID, integrationMode = strings.TrimSpace(wxCorpID), strings.TrimSpace(integrationMode)
	if client == nil || !scope.valid() || wxCorpID == "" || (integrationMode != IntegrationModeSelfBuilt && integrationMode != IntegrationModeThirdPartyDelegated) {
		return nil, errors.New("archive bridge source binding is invalid")
	}
	return &BridgeSource{client: client, scope: scope, wxCorpID: wxCorpID, integrationMode: integrationMode, sourceID: "wecom:" + integrationMode + ":" + wxCorpID}, nil
}

func (s *BridgeSource) Kind() providers.Source { return providers.SourceExternal }
func (s *BridgeSource) SourceID() string {
	if s == nil {
		return ""
	}
	return s.sourceID
}
func (s *BridgeSource) Namespace() string {
	if s == nil {
		return ""
	}
	return s.sourceID
}
func (s *BridgeSource) Status() providers.Status {
	if s == nil || s.client == nil || !s.scope.valid() || s.wxCorpID == "" || s.integrationMode == "" {
		return providers.Status{Kind: "wecom_archive", Source: providers.SourceExternal, State: providers.StateUnavailable, Code: "archive.source_unavailable"}
	}
	capabilities := []string{"archive_sync"}
	if s.integrationMode == IntegrationModeSelfBuilt {
		capabilities = append(capabilities, "archive_media")
	} else {
		capabilities = append(capabilities, "archive_component")
	}
	return providers.Status{Kind: "wecom_archive", Source: providers.SourceExternal, State: providers.StateReady, Code: "archive.bridge_ready", Capabilities: capabilities}
}

func (s *BridgeSource) Fetch(ctx context.Context, scope Scope, cursor Cursor, limit int) (Page, error) {
	if ctx == nil {
		return Page{}, errors.New("context is required")
	}
	if s == nil || s.client == nil || scope != s.scope {
		return Page{}, ErrInvalidScope
	}
	if limit <= 0 {
		limit = DefaultFetchLimit
	}
	page, err := s.client.fetchMessages(ctx, scope, s.wxCorpID, s.integrationMode, cursor, limit)
	if err != nil {
		return Page{}, err
	}
	result := Page{NextCursor: cursor}
	for _, raw := range page.Messages {
		message, err := parseBridgeMessage(raw, s.sourceID)
		if err != nil {
			return Page{}, err
		}
		if message.Seq <= cursor.Sequence {
			continue
		}
		result.Messages = append(result.Messages, message)
		if message.Seq > result.NextCursor.Sequence {
			result.NextCursor.Sequence = message.Seq
		}
	}
	result.HasMore = len(page.Messages) >= limit && result.NextCursor.Sequence > cursor.Sequence
	return result, nil
}

func parseBridgeMessage(raw json.RawMessage, sourceID string) (Message, error) {
	var object map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&object); err != nil {
		return Message{}, errors.New("archive bridge message is invalid")
	}
	seq := anyInt64(object["seq"])
	msgID := anyString(object["msgid"])
	msgType := strings.ToLower(anyString(object["msgtype"]))
	if seq <= 0 || msgID == "" || msgType == "" {
		return Message{}, errors.New("archive bridge message identity is invalid")
	}
	policy := ContentPolicy(anyString(object["content_policy"]))
	if policy == "" {
		policy = ContentPolicyPlaintext
	}
	if policy != ContentPolicyPlaintext && policy != ContentPolicyComponent {
		return Message{}, errors.New("archive bridge content policy is invalid")
	}
	var component *ComponentDescriptor
	if policy == ContentPolicyComponent {
		locator, _ := object["component_locator"].(map[string]any)
		component = &ComponentDescriptor{
			MessageID: anyString(locator["msgid"]), PublicKeyVersion: uint32(anyInt64(locator["public_key_ver"])),
			EncryptedSecretKey: anyString(locator["encrypted_secret_key"]),
		}
		if component.MessageID != msgID || component.PublicKeyVersion == 0 || component.EncryptedSecretKey == "" {
			return Message{}, errors.New("archive bridge component locator is invalid")
		}
		delete(object, "component_locator")
	}
	var media []MediaDescriptor
	if policy == ContentPolicyPlaintext {
		media = mediaDescriptors(msgType, object[msgType])
	}
	sanitized := sanitizeSDKIdentifiers(object).(map[string]any)
	sanitizedRaw, err := json.Marshal(sanitized)
	if err != nil {
		return Message{}, errors.New("archive bridge message sanitization failed")
	}
	content := sanitized[msgType]
	if content == nil {
		content = map[string]any{}
	}
	if policy == ContentPolicyComponent {
		content = map[string]any{}
	}
	contentRaw, err := json.Marshal(content)
	if err != nil {
		return Message{}, errors.New("archive bridge message content is invalid")
	}
	return Message{
		Source: providers.SourceExternal, SourceID: sourceID, Namespace: sourceID,
		MsgID: msgID, Seq: seq, Action: anyString(object["action"]), From: anyString(object["from"]),
		ToList: anyStringSlice(object["tolist"]), RoomID: anyString(object["roomid"]), MsgType: msgType,
		MsgTime: unixBridgeTime(anyInt64(object["msgtime"])), ContentRaw: string(contentRaw),
		ContentText: bridgeContentText(msgType, content), RawJSON: string(sanitizedRaw), Media: media,
		ContentPolicy: policy, Component: component,
	}, nil
}

func mediaDescriptors(msgType string, payload any) []MediaDescriptor {
	if msgType == "mixed" {
		return mixedMediaDescriptors(payload)
	}
	if msgType != "image" && msgType != "voice" && msgType != "video" && msgType != "file" {
		return nil
	}
	object, ok := payload.(map[string]any)
	if !ok {
		return nil
	}
	sdkFileID := anyString(firstAny(object, "sdkfileid", "sdk_file_id"))
	if sdkFileID == "" {
		return nil
	}
	name := safeMediaName(anyString(firstAny(object, "filename", "file_name")))
	mimeType := anyString(firstAny(object, "mime_type", "mimetype"))
	if mimeType == "" && name != "" {
		mimeType = mime.TypeByExtension(filepath.Ext(name))
	}
	return []MediaDescriptor{{Type: msgType, SDKFileID: sdkFileID, FileName: name, MIMEType: mimeType,
		ExpectedSize: anyInt64(firstAny(object, "filesize", "file_size", "voice_size")),
		ExpectedMD5:  strings.ToLower(anyString(firstAny(object, "md5sum", "md5"))),
	}}
}

func mixedMediaDescriptors(payload any) []MediaDescriptor {
	root, ok := payload.(map[string]any)
	if !ok {
		return nil
	}
	items, _ := root["item"].([]any)
	result := make([]MediaDescriptor, 0)
	for _, item := range items {
		object, ok := item.(map[string]any)
		if !ok {
			continue
		}
		kind := strings.ToLower(anyString(object["type"]))
		content := object[kind]
		if content == nil {
			content = object["content"]
		}
		result = append(result, mediaDescriptors(kind, content)...)
	}
	return result
}

func sanitizeSDKIdentifiers(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			normalized := strings.ToLower(strings.ReplaceAll(key, "_", ""))
			if normalized == "sdkfileid" {
				continue
			}
			result[key] = sanitizeSDKIdentifiers(item)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, item := range typed {
			result[index] = sanitizeSDKIdentifiers(item)
		}
		return result
	default:
		return value
	}
}

func bridgeContentText(kind string, payload any) string {
	object, _ := payload.(map[string]any)
	for _, key := range []string{"content", "title", "address", "filename"} {
		if text := anyString(object[key]); text != "" {
			return text
		}
	}
	if kind == "mixed" {
		var values []string
		items, _ := object["item"].([]any)
		for _, item := range items {
			entry, _ := item.(map[string]any)
			if text := bridgeContentText(anyString(entry["type"]), entry["content"]); text != "" {
				values = append(values, text)
			}
		}
		return strings.Join(values, " ")
	}
	return ""
}

func firstAny(object map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := object[key]; ok {
			return value
		}
	}
	return nil
}
func anyString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return ""
	}
}
func anyInt64(value any) int64 {
	switch typed := value.(type) {
	case json.Number:
		result, _ := typed.Int64()
		return result
	case float64:
		return int64(typed)
	case int64:
		return typed
	case string:
		result, _ := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return result
	default:
		return 0
	}
}
func anyStringSlice(value any) []string {
	items, _ := value.([]any)
	result := make([]string, 0, len(items))
	for _, item := range items {
		if text := anyString(item); text != "" {
			result = append(result, text)
		}
	}
	return result
}
func unixBridgeTime(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	if value > 1_000_000_000_000 {
		return time.UnixMilli(value).UTC()
	}
	return time.Unix(value, 0).UTC()
}
func safeMediaName(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	value = filepath.Base(value)
	if value == "." || value == "/" {
		return ""
	}
	runes := []rune(value)
	if len(runes) > 255 {
		value = string(runes[:255])
	}
	return value
}
