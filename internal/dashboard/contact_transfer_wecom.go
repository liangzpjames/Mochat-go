package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func (c *RoomWelcomeWeComClient) GetUnassigned(ctx context.Context, credential RoomWelcomeCorpCredential) ([]ContactTransferUnassignedSeed, error) {
	token, err := c.accessToken(ctx, credential)
	if err != nil {
		return nil, err
	}
	var response struct {
		weComBaseResponse
		Info []struct {
			HandoverUserID string `json:"handover_userid"`
			ExternalUserID string `json:"external_userid"`
			DimissionTime  int64  `json:"dimission_time"`
		} `json:"info"`
	}
	if err := c.postJSON(ctx, "cgi-bin/externalcontact/get_unassigned_list", token, map[string]any{}, &response); err != nil {
		return nil, err
	}
	items := make([]ContactTransferUnassignedSeed, 0, len(response.Info))
	for _, item := range response.Info {
		items = append(items, ContactTransferUnassignedSeed{
			HandoverUserID: item.HandoverUserID,
			ExternalUserID: item.ExternalUserID,
			DimissionTime:  item.DimissionTime,
		})
	}
	return items, nil
}

func (c *RoomWelcomeWeComClient) TransferCustomer(ctx context.Context, credential RoomWelcomeCorpCredential, externalUserIDs []string, handoverUserID string, takeoverUserID string, successMsg string) (map[string]any, error) {
	if isLocalContactTransferSimulation(credential) {
		return map[string]any{"errcode": 0, "errmsg": "simulated success"}, nil
	}
	token, err := c.accessToken(ctx, credential)
	if err != nil {
		return nil, err
	}
	request := map[string]any{
		"external_userid":      externalUserIDs,
		"handover_userid":      handoverUserID,
		"takeover_userid":      takeoverUserID,
		"transfer_success_msg": successMsg,
	}
	response := map[string]any{}
	if err := c.postJSONRaw(ctx, "cgi-bin/externalcontact/transfer_customer", token, request, &response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *RoomWelcomeWeComClient) TransferGroupChat(ctx context.Context, credential RoomWelcomeCorpCredential, chatIDs []string, takeoverUserID string) ([]map[string]any, error) {
	if isLocalContactTransferSimulation(credential) {
		return []map[string]any{}, nil
	}
	token, err := c.accessToken(ctx, credential)
	if err != nil {
		return nil, err
	}
	var response struct {
		weComBaseResponse
		FailedChatList []map[string]any `json:"failed_chat_list"`
	}
	request := map[string]any{
		"chat_id_list": chatIDs,
		"new_owner":    takeoverUserID,
	}
	if err := c.postJSON(ctx, "cgi-bin/externalcontact/groupchat/transfer", token, request, &response); err != nil {
		return nil, err
	}
	if response.FailedChatList == nil {
		response.FailedChatList = []map[string]any{}
	}
	return response.FailedChatList, nil
}

// isLocalContactTransferSimulation is intentionally limited to the explicit
// development fixture credentials. Real enterprise credentials always use the
// HTTP provider path above; this keeps local seeded data testable without ever
// treating a production credential as a simulated transfer.
func isLocalContactTransferSimulation(credential RoomWelcomeCorpCredential) bool {
	return strings.HasPrefix(strings.TrimSpace(credential.WXCorpID), "wwSIM") &&
		strings.HasPrefix(strings.TrimSpace(credential.ContactSecret), "SIM-")
}

func (c *RoomWelcomeWeComClient) TransferResult(ctx context.Context, credential RoomWelcomeCorpCredential, handoverUserID string, takeoverUserID string) ([]ContactTransferStateResult, error) {
	token, err := c.accessToken(ctx, credential)
	if err != nil {
		return nil, err
	}
	var response struct {
		weComBaseResponse
		Customer []struct {
			ExternalUserID string `json:"external_userid"`
			Status         int    `json:"status"`
		} `json:"customer"`
	}
	request := map[string]any{
		"handover_userid": handoverUserID,
		"takeover_userid": takeoverUserID,
	}
	if err := c.postJSON(ctx, "cgi-bin/externalcontact/transfer_result", token, request, &response); err != nil {
		return nil, err
	}
	results := make([]ContactTransferStateResult, 0, len(response.Customer))
	for _, item := range response.Customer {
		results = append(results, ContactTransferStateResult{
			ExternalUserID: item.ExternalUserID,
			Status:         item.Status,
		})
	}
	return results, nil
}

func (c *RoomWelcomeWeComClient) UpdateExternalContactRemark(ctx context.Context, credential RoomWelcomeCorpCredential, payload WorkContactRemarkPayload) error {
	token, err := c.accessToken(ctx, credential)
	if err != nil {
		return err
	}
	request := map[string]any{
		"userid":          payload.UserID,
		"external_userid": payload.ExternalUserID,
	}
	if payload.Remark != nil {
		request["remark"] = *payload.Remark
	}
	if payload.Description != nil {
		request["description"] = *payload.Description
	}
	var response weComBaseResponse
	return c.postJSON(ctx, "cgi-bin/externalcontact/remark", token, request, &response)
}

func (c *RoomWelcomeWeComClient) MarkExternalContactTags(ctx context.Context, credential RoomWelcomeCorpCredential, payload WorkContactMarkTagsPayload) error {
	token, err := c.accessToken(ctx, credential)
	if err != nil {
		return err
	}
	request := map[string]any{
		"userid":          payload.UserID,
		"external_userid": payload.ExternalUserID,
		"add_tag":         payload.AddTag,
	}
	var response weComBaseResponse
	return c.postJSON(ctx, "cgi-bin/externalcontact/mark_tag", token, request, &response)
}

func (c *RoomWelcomeWeComClient) postJSONRaw(ctx context.Context, path string, accessToken string, payload any, out any) error {
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
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("企业微信接口 HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
