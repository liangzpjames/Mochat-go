package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"jiyi/mochat-go/internal/modules/providers"
	"jiyi/mochat-go/internal/wecomcapability"
)

const defaultWeComAPIBaseURL = "https://qyapi.weixin.qq.com"

type RoomWelcomeWeComClient struct {
	baseURL                  string
	httpClient               *http.Client
	mu                       sync.Mutex
	tokens                   map[string]cachedWeComToken
	callbackRouteConfigured  bool
	callbackWorkerConfigured bool
}

// WithCallbackRuntime binds production callback route/worker configuration to
// the runtime adapter. These flags are prerequisites only; a successful
// callback operation/event is required before status can become ready.
func (c *RoomWelcomeWeComClient) WithCallbackRuntime(routeConfigured, workerConfigured bool) *RoomWelcomeWeComClient {
	if c != nil {
		c.callbackRouteConfigured = routeConfigured
		c.callbackWorkerConfigured = workerConfigured
	}
	return c
}

type cachedWeComToken struct {
	token     string
	expiresAt time.Time
}

func NewRoomWelcomeWeComClient(baseURL string) *RoomWelcomeWeComClient {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = defaultWeComAPIBaseURL
	}
	return &RoomWelcomeWeComClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		tokens:     map[string]cachedWeComToken{},
	}
}

// Status reports the runtime adapter boundary only. Tenant credential and
// verification evidence is supplied by companyprofile.ProviderStatusSource.
func (c *RoomWelcomeWeComClient) Status() providers.Status {
	if c == nil || strings.TrimSpace(c.baseURL) == "" {
		return providers.Status{
			Kind:   "wecom_standard",
			State:  providers.StateUnavailable,
			Code:   "wecom.runtime_component_missing",
			Source: providers.SourceExternal,
			Action: "启用企业微信运行时组件",
		}
	}
	return providers.Status{
		Kind:                     "wecom_standard",
		State:                    providers.StateLimited,
		Code:                     "wecom.tenant_credentials_required",
		Source:                   providers.SourceExternal,
		Capabilities:             append([]string(nil), wecomcapability.All...),
		Reason:                   "企业微信 HTTP runtime 已就绪，当前状态需由租户凭据和验证结果决定",
		Action:                   "完成企业微信凭据配置与验证",
		CallbackRouteConfigured:  c.callbackRouteConfigured,
		CallbackWorkerConfigured: c.callbackWorkerConfigured,
	}
}

func (c *RoomWelcomeWeComClient) UploadImage(ctx context.Context, credential RoomWelcomeCorpCredential, filePath string) (string, error) {
	token, err := c.accessToken(ctx, credential)
	if err != nil {
		return "", err
	}
	var response struct {
		weComBaseResponse
		URL string `json:"url"`
	}
	if err := c.postMultipart(ctx, "cgi-bin/media/uploadimg", token, "", filePath, &response); err != nil {
		return "", err
	}
	return response.URL, nil
}

func (c *RoomWelcomeWeComClient) UploadTemporaryImage(ctx context.Context, credential RoomWelcomeCorpCredential, filePath string) (string, error) {
	token, err := c.accessToken(ctx, credential)
	if err != nil {
		return "", err
	}
	var response struct {
		weComBaseResponse
		MediaID string `json:"media_id"`
	}
	if err := c.postMultipart(ctx, "cgi-bin/media/upload", token, "image", filePath, &response); err != nil {
		return "", err
	}
	return response.MediaID, nil
}

func (c *RoomWelcomeWeComClient) UploadTemporaryMedia(ctx context.Context, credential MediumCorpCredential, mediaType string, filePath string) (string, error) {
	token, err := c.accessToken(ctx, RoomWelcomeCorpCredential{
		WXCorpID:      credential.WXCorpID,
		ContactSecret: credential.EmployeeSecret,
	})
	if err != nil {
		return "", err
	}
	var response struct {
		weComBaseResponse
		MediaID string `json:"media_id"`
	}
	if err := c.postMultipart(ctx, "cgi-bin/media/upload", token, mediaType, filePath, &response); err != nil {
		return "", err
	}
	return response.MediaID, nil
}

func (c *RoomWelcomeWeComClient) CreateGroupWelcomeTemplate(ctx context.Context, credential RoomWelcomeCorpCredential, payload RoomWelcomeTemplatePayload) (string, error) {
	token, err := c.accessToken(ctx, credential)
	if err != nil {
		return "", err
	}
	var response struct {
		weComBaseResponse
		TemplateID string `json:"template_id"`
	}
	if err := c.postJSON(ctx, "cgi-bin/externalcontact/group_welcome_template/add", token, roomWelcomeTemplateRequest(payload, ""), &response); err != nil {
		return "", err
	}
	return response.TemplateID, nil
}

func (c *RoomWelcomeWeComClient) UpdateGroupWelcomeTemplate(ctx context.Context, credential RoomWelcomeCorpCredential, templateID string, payload RoomWelcomeTemplatePayload) error {
	token, err := c.accessToken(ctx, credential)
	if err != nil {
		return err
	}
	var response weComBaseResponse
	return c.postJSON(ctx, "cgi-bin/externalcontact/group_welcome_template/edit", token, roomWelcomeTemplateRequest(payload, templateID), &response)
}

func (c *RoomWelcomeWeComClient) DeleteGroupWelcomeTemplate(ctx context.Context, credential RoomWelcomeCorpCredential, templateID string) error {
	token, err := c.accessToken(ctx, credential)
	if err != nil {
		return err
	}
	var response weComBaseResponse
	return c.postJSON(ctx, "cgi-bin/externalcontact/group_welcome_template/del", token, map[string]any{"template_id": templateID}, &response)
}

func (c *RoomWelcomeWeComClient) SendExternalContactWelcome(ctx context.Context, credential RoomWelcomeCorpCredential, welcomeCode string, payload ContactWelcomePayload) error {
	token, err := c.accessToken(ctx, credential)
	if err != nil {
		return err
	}
	request := contactWelcomeRequest(payload, welcomeCode)
	var response weComBaseResponse
	return c.postJSON(ctx, "cgi-bin/externalcontact/send_welcome_msg", token, request, &response)
}

func (c *RoomWelcomeWeComClient) CreateExternalContactMessage(ctx context.Context, credential RoomWelcomeCorpCredential, payload RoomTagPullMessagePayload) (string, error) {
	token, err := c.accessToken(ctx, credential)
	if err != nil {
		return "", err
	}
	request := map[string]any{
		"text":            map[string]any{"content": payload.TextContent},
		"image":           map[string]any{"pic_url": payload.ImagePicURL},
		"external_userid": payload.ExternalUserIDs,
		"sender":          payload.Sender,
	}
	var response struct {
		weComBaseResponse
		MsgID string `json:"msgid"`
	}
	if err := c.postJSON(ctx, "cgi-bin/externalcontact/add_msg_template", token, request, &response); err != nil {
		return "", err
	}
	if response.MsgID == "" {
		return "", fmt.Errorf("企业微信接口未返回 msgid")
	}
	return response.MsgID, nil
}

func (c *RoomWelcomeWeComClient) SubmitContactMessageBatchSend(ctx context.Context, credential RoomWelcomeCorpCredential, payload ContactMessageBatchSendMessagePayload) (ContactMessageBatchSendMessageResult, error) {
	token, err := c.accessToken(ctx, credential)
	if err != nil {
		return ContactMessageBatchSendMessageResult{}, err
	}
	request := contactMessageBatchSendWeComRequest(payload)
	var response struct {
		weComBaseResponse
		MsgID string `json:"msgid"`
	}
	if err := c.postJSON(ctx, "cgi-bin/externalcontact/add_msg_template", token, request, &response); err != nil {
		return ContactMessageBatchSendMessageResult{}, err
	}
	return ContactMessageBatchSendMessageResult{
		ErrCode: response.ErrCode,
		ErrMsg:  response.ErrMsg,
		MsgID:   response.MsgID,
	}, nil
}

func (c *RoomWelcomeWeComClient) SubmitRoomMessageBatchSend(ctx context.Context, credential RoomWelcomeCorpCredential, payload RoomMessageBatchSendMessagePayload) (RoomMessageBatchSendMessageResult, error) {
	token, err := c.accessToken(ctx, credential)
	if err != nil {
		return RoomMessageBatchSendMessageResult{}, err
	}
	request := roomMessageBatchSendWeComRequest(payload)
	var response struct {
		weComBaseResponse
		MsgID string `json:"msgid"`
	}
	if err := c.postJSON(ctx, "cgi-bin/externalcontact/add_msg_template", token, request, &response); err != nil {
		return RoomMessageBatchSendMessageResult{}, err
	}
	return RoomMessageBatchSendMessageResult{
		ErrCode: response.ErrCode,
		ErrMsg:  response.ErrMsg,
		MsgID:   response.MsgID,
	}, nil
}

func (c *RoomWelcomeWeComClient) GroupMessageTasks(ctx context.Context, credential RoomWelcomeCorpCredential, msgID string, limit int, cursor string) (BatchSendGroupTaskPage, error) {
	token, err := c.accessToken(ctx, credential)
	if err != nil {
		return BatchSendGroupTaskPage{}, err
	}
	if limit <= 0 {
		limit = batchSendPageLimit
	}
	request := map[string]any{
		"msgid":  msgID,
		"limit":  limit,
		"cursor": cursor,
	}
	var response struct {
		weComBaseResponse
		TaskList   []BatchSendGroupTask `json:"task_list"`
		NextCursor string               `json:"next_cursor"`
	}
	if err := c.postJSON(ctx, "cgi-bin/externalcontact/get_groupmsg_task", token, request, &response); err != nil {
		return BatchSendGroupTaskPage{}, err
	}
	return BatchSendGroupTaskPage{
		ErrCode:    response.ErrCode,
		ErrMsg:     response.ErrMsg,
		TaskList:   response.TaskList,
		NextCursor: response.NextCursor,
	}, nil
}

func (c *RoomWelcomeWeComClient) GroupMessageSendResults(ctx context.Context, credential RoomWelcomeCorpCredential, msgID string, userID string, limit int, cursor string) (BatchSendGroupResultPage, error) {
	token, err := c.accessToken(ctx, credential)
	if err != nil {
		return BatchSendGroupResultPage{}, err
	}
	if limit <= 0 {
		limit = batchSendPageLimit
	}
	request := map[string]any{
		"msgid":  msgID,
		"userid": userID,
		"limit":  limit,
		"cursor": cursor,
	}
	var response struct {
		weComBaseResponse
		SendList   []BatchSendGroupResult `json:"send_list"`
		NextCursor string                 `json:"next_cursor"`
	}
	if err := c.postJSON(ctx, "cgi-bin/externalcontact/get_groupmsg_send_result", token, request, &response); err != nil {
		return BatchSendGroupResultPage{}, err
	}
	return BatchSendGroupResultPage{
		ErrCode:    response.ErrCode,
		ErrMsg:     response.ErrMsg,
		SendList:   response.SendList,
		NextCursor: response.NextCursor,
	}, nil
}

func (c *RoomWelcomeWeComClient) SendAgentTextMessage(ctx context.Context, credential RoomTagPullAgentCredential, toUser string, content string) error {
	return c.SendAgentMessage(ctx, credential, WorkAgentMessagePayload{
		ToUser:  toUser,
		MsgType: "text",
		Content: content,
		Extra:   nil,
	})
}

func (c *RoomWelcomeWeComClient) SendAgentMessage(ctx context.Context, credential RoomTagPullAgentCredential, payload WorkAgentMessagePayload) error {
	token, err := c.accessToken(ctx, RoomWelcomeCorpCredential{
		WXCorpID:      credential.WXCorpID,
		ContactSecret: credential.WXSecret,
	})
	if err != nil {
		return err
	}
	request, err := workAgentMessageRequest(credential, payload)
	if err != nil {
		return err
	}
	var response weComBaseResponse
	return c.postJSON(ctx, "cgi-bin/message/send", token, request, &response)
}

func workAgentMessageRequest(credential RoomTagPullAgentCredential, payload WorkAgentMessagePayload) (map[string]any, error) {
	msgType := strings.TrimSpace(payload.MsgType)
	if msgType == "" {
		return nil, fmt.Errorf("missing message type")
	}
	request := map[string]any{
		"msgtype":                  msgType,
		"agentid":                  credential.WXAgentID,
		"safe":                     workAgentMessageExtraInt(payload.Extra, "safe", 0),
		"enable_id_trans":          workAgentMessageExtraInt(payload.Extra, "enable_id_trans", 0),
		"enable_duplicate_check":   workAgentMessageExtraInt(payload.Extra, "enable_duplicate_check", 0),
		"duplicate_check_interval": workAgentMessageExtraInt(payload.Extra, "duplicate_check_interval", 1800),
	}
	hasRecipient := false
	if toUser := strings.TrimSpace(payload.ToUser); toUser != "" {
		request["touser"] = toUser
		hasRecipient = true
	}
	if toParty := strings.TrimSpace(payload.ToParty); toParty != "" {
		request["toparty"] = toParty
		hasRecipient = true
	}
	if toTag := strings.TrimSpace(payload.ToTag); toTag != "" {
		request["totag"] = toTag
		hasRecipient = true
	}
	if !hasRecipient {
		return nil, fmt.Errorf("missing recipient")
	}
	if err := addWorkAgentMessageContent(request, msgType, payload.Content); err != nil {
		return nil, err
	}
	return request, nil
}

func addWorkAgentMessageContent(request map[string]any, msgType string, content any) error {
	if content == nil {
		return fmt.Errorf("missing message content")
	}
	if raw, ok := content.(json.RawMessage); ok {
		decoded, err := decodeMessageRemindAny(raw)
		if err != nil {
			return err
		}
		content = decoded
	}
	if text, ok := content.(string); ok {
		request["text"] = map[string]any{"content": text}
		return nil
	}
	request[msgType] = content
	return nil
}

func workAgentMessageExtraInt(extra map[string]any, key string, fallback int) int {
	if extra == nil {
		return fallback
	}
	value, ok := extra[key]
	if !ok {
		return fallback
	}
	raw := strings.TrimSpace(messageRemindStringValue(value))
	if raw == "" {
		return fallback
	}
	integer, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return integer
}

func contactMessageBatchSendWeComRequest(payload ContactMessageBatchSendMessagePayload) map[string]any {
	request := map[string]any{
		"chat_type":       "single",
		"sender":          payload.Sender,
		"external_userid": payload.ExternalUserID,
	}
	attachments := make([]map[string]any, 0)
	for _, item := range payload.Content {
		switch item.MsgType {
		case "text":
			if item.Content != "" {
				request["text"] = map[string]any{"content": item.Content}
			}
		case "image":
			if item.MediaID != "" {
				attachments = append(attachments, map[string]any{
					"msgtype": "image",
					"image":   map[string]any{"media_id": item.MediaID},
				})
			}
		case "link":
			link := map[string]any{
				"title": item.Title,
				"url":   item.URL,
			}
			if item.Desc != "" {
				link["desc"] = item.Desc
			}
			if item.PicURL != "" {
				link["picurl"] = item.PicURL
			}
			attachments = append(attachments, map[string]any{
				"msgtype": "link",
				"link":    link,
			})
		case "miniprogram":
			miniprogram := map[string]any{
				"title":        item.Title,
				"pic_media_id": item.PicMediaID,
				"appid":        item.AppID,
				"page":         item.Page,
			}
			attachments = append(attachments, map[string]any{
				"msgtype":     "miniprogram",
				"miniprogram": miniprogram,
			})
		}
	}
	if len(attachments) > 0 {
		request["attachments"] = attachments
	}
	return request
}

func roomMessageBatchSendWeComRequest(payload RoomMessageBatchSendMessagePayload) map[string]any {
	request := contactMessageBatchSendWeComRequest(ContactMessageBatchSendMessagePayload{
		Content: payload.Content,
		Sender:  payload.Sender,
	})
	request["chat_type"] = "group"
	request["chat_id_list"] = payload.ChatIDs
	delete(request, "external_userid")
	return request
}

func (c *RoomWelcomeWeComClient) accessToken(ctx context.Context, credential RoomWelcomeCorpCredential) (string, error) {
	key := credential.WXCorpID + "\x00" + credential.ContactSecret
	c.mu.Lock()
	if cached, ok := c.tokens[key]; ok && cached.token != "" && time.Now().Before(cached.expiresAt.Add(-60*time.Second)) {
		c.mu.Unlock()
		return cached.token, nil
	}
	c.mu.Unlock()

	values := url.Values{}
	values.Set("corpid", credential.WXCorpID)
	values.Set("corpsecret", credential.ContactSecret)
	requestURL := c.baseURL + "/cgi-bin/gettoken?" + values.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("企业微信接口 HTTP %d", resp.StatusCode)
	}
	var response struct {
		weComBaseResponse
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", err
	}
	if err := response.Err(); err != nil {
		return "", err
	}
	if response.AccessToken == "" {
		return "", fmt.Errorf("企业微信接口未返回 access_token")
	}
	expiresIn := response.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 7200
	}
	c.mu.Lock()
	c.tokens[key] = cachedWeComToken{token: response.AccessToken, expiresAt: time.Now().Add(time.Duration(expiresIn) * time.Second)}
	c.mu.Unlock()
	return response.AccessToken, nil
}

func (c *RoomWelcomeWeComClient) postJSON(ctx context.Context, path string, accessToken string, payload any, out interface{ Err() error }) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	requestURL := c.apiURL(path, accessToken, "")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return weComHTTPError(resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return err
	}
	return out.Err()
}

func (c *RoomWelcomeWeComClient) postMultipart(ctx context.Context, path string, accessToken string, mediaType string, filePath string, out interface{ Err() error }) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("media", filepath.Base(filePath))
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, file); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	requestURL := c.apiURL(path, accessToken, mediaType)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return weComHTTPError(resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return err
	}
	return out.Err()
}

func (c *RoomWelcomeWeComClient) apiURL(path string, accessToken string, mediaType string) string {
	values := url.Values{}
	values.Set("access_token", accessToken)
	if mediaType != "" {
		values.Set("type", mediaType)
	}
	return c.baseURL + "/" + strings.TrimLeft(path, "/") + "?" + values.Encode()
}

type weComBaseResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

func (r weComBaseResponse) Err() error {
	if r.ErrCode == 0 {
		return nil
	}
	return weComAPIError{code: r.ErrCode}
}

type weComAPIError struct {
	code int
}

func (e weComAPIError) Error() string {
	return fmt.Sprintf("WECOM_API_ERROR_%d", e.code)
}

func weComHTTPError(statusCode int) error {
	return fmt.Errorf("WECOM_HTTP_ERROR_%d", statusCode)
}

func roomWelcomeTemplateRequest(payload RoomWelcomeTemplatePayload, templateID string) map[string]any {
	request := map[string]any{"notify": payload.Notify}
	if templateID != "" {
		request["template_id"] = templateID
	}
	if strings.TrimSpace(payload.TextContent) != "" {
		request["text"] = map[string]any{"content": payload.TextContent}
	}
	if strings.TrimSpace(payload.ImagePicURL) != "" {
		request["image"] = map[string]any{"pic_url": payload.ImagePicURL}
	}
	if payload.Link != nil {
		request["link"] = map[string]any{
			"title":  payload.Link.Title,
			"picurl": payload.Link.PicURL,
			"desc":   payload.Link.Desc,
			"url":    payload.Link.URL,
		}
	}
	if payload.MiniProgram != nil {
		request["miniprogram"] = map[string]any{
			"title":        payload.MiniProgram.Title,
			"pic_media_id": payload.MiniProgram.PicMediaID,
			"appid":        payload.MiniProgram.AppID,
			"page":         payload.MiniProgram.Page,
		}
	}
	return request
}
