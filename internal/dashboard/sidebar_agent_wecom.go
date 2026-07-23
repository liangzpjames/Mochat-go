package dashboard

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func (c *RoomWelcomeWeComClient) OAuthURL(credential SidebarAgentCredential, redirectURI string) string {
	values := url.Values{}
	values.Set("appid", credential.WXCorpID)
	values.Set("redirect_uri", redirectURI)
	values.Set("response_type", "code")
	values.Set("scope", "snsapi_base")
	values.Set("state", "")
	return "https://open.weixin.qq.com/connect/oauth2/authorize?" + values.Encode() + "#wechat_redirect"
}

func (c *RoomWelcomeWeComClient) OAuthUserID(ctx context.Context, credential SidebarAgentCredential, code string) (string, error) {
	token, err := c.accessToken(ctx, RoomWelcomeCorpCredential{
		WXCorpID:      credential.WXCorpID,
		ContactSecret: credential.WXSecret,
	})
	if err != nil {
		return "", err
	}
	var response struct {
		weComBaseResponse
		UserID      string `json:"userid"`
		UserIDCamel string `json:"UserId"`
	}
	if err := c.getJSON(ctx, "cgi-bin/auth/getuserinfo", token, url.Values{"code": {code}}, &response); err != nil {
		return "", err
	}
	userID := response.UserID
	if userID == "" {
		userID = response.UserIDCamel
	}
	if userID == "" {
		return "", fmt.Errorf("企业微信接口未返回 userid")
	}
	return userID, nil
}

func (c *RoomWelcomeWeComClient) JSSDKConfig(ctx context.Context, credential SidebarAgentCredential, agent bool, uri string, jsAPIs []string) (map[string]any, error) {
	secret := credential.ContactSecret
	if agent {
		secret = credential.WXSecret
	}
	token, err := c.accessToken(ctx, RoomWelcomeCorpCredential{
		WXCorpID:      credential.WXCorpID,
		ContactSecret: secret,
	})
	if err != nil {
		return nil, err
	}
	ticketPath := "cgi-bin/get_jsapi_ticket"
	extra := url.Values{}
	if agent {
		ticketPath = "cgi-bin/ticket/get"
		extra.Set("type", "agent_config")
	}
	var ticketResponse struct {
		weComBaseResponse
		Ticket string `json:"ticket"`
	}
	if err := c.getJSON(ctx, ticketPath, token, extra, &ticketResponse); err != nil {
		return nil, err
	}
	if ticketResponse.Ticket == "" {
		return nil, fmt.Errorf("企业微信接口未返回 jsapi ticket")
	}
	nonce, err := randomNonce()
	if err != nil {
		return nil, err
	}
	timestamp := time.Now().Unix()
	signature := jssdkSignature(ticketResponse.Ticket, nonce, timestamp, uri)
	config := map[string]any{
		"beta":      true,
		"debug":     false,
		"timestamp": strconv.FormatInt(timestamp, 10),
		"nonceStr":  nonce,
		"signature": signature,
		"jsApiList": append([]string{}, jsAPIs...),
	}
	if agent {
		config["corpid"] = credential.WXCorpID
		config["corpId"] = credential.WXCorpID
		config["agentid"] = credential.WXAgentID
		config["agentId"] = credential.WXAgentID
	} else {
		config["appId"] = credential.WXCorpID
		config["corpId"] = credential.WXCorpID
	}
	return config, nil
}

func (c *RoomWelcomeWeComClient) getJSON(ctx context.Context, path string, accessToken string, extra url.Values, out interface{ Err() error }) error {
	values := url.Values{}
	values.Set("access_token", accessToken)
	for key, items := range extra {
		for _, item := range items {
			values.Add(key, item)
		}
	}
	requestURL := c.baseURL + "/" + strings.TrimLeft(path, "/") + "?" + values.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("企业微信接口 HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return err
	}
	return out.Err()
}

func jssdkSignature(ticket string, nonce string, timestamp int64, uri string) string {
	raw := "jsapi_ticket=" + ticket + "&noncestr=" + nonce + "&timestamp=" + strconv.FormatInt(timestamp, 10) + "&url=" + uri
	sum := sha1.Sum([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func randomNonce() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}
