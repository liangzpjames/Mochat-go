package dashboard

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// WorkRoomAutoPullJoinWayPayload is the business contract for a WeCom
// customer-group join QR code. ChatIDs must already be resolved inside one corp.
type WorkRoomAutoPullJoinWayPayload struct {
	QRCodeName     string
	ChatIDs        []string
	AutoCreateRoom bool
	RoomBaseName   string
	RoomBaseID     int
}

type WorkRoomAutoPullJoinWayClient interface {
	CreateJoinWay(context.Context, RoomWelcomeCorpCredential, WorkRoomAutoPullJoinWayPayload) (WorkRoomAutoPullQRCode, error)
	UpdateJoinWay(context.Context, RoomWelcomeCorpCredential, string, WorkRoomAutoPullJoinWayPayload) error
	DeleteJoinWay(context.Context, RoomWelcomeCorpCredential, string) error
}

type WorkRoomAutoPullJoinWayWeComClient struct{ client *RoomWelcomeWeComClient }

func NewWorkRoomAutoPullJoinWayWeComClient(baseURL string) *WorkRoomAutoPullJoinWayWeComClient {
	return &WorkRoomAutoPullJoinWayWeComClient{client: NewRoomWelcomeWeComClient(baseURL)}
}

func (c *WorkRoomAutoPullJoinWayWeComClient) CreateJoinWay(ctx context.Context, credential RoomWelcomeCorpCredential, payload WorkRoomAutoPullJoinWayPayload) (WorkRoomAutoPullQRCode, error) {
	token, err := c.client.accessToken(ctx, credential)
	if err != nil {
		return WorkRoomAutoPullQRCode{}, err
	}
	var created struct {
		weComBaseResponse
		ConfigID string `json:"config_id"`
	}
	if err := c.client.postJSON(ctx, "cgi-bin/externalcontact/groupchat/add_join_way", token, workRoomAutoPullJoinWayRequest(payload, ""), &created); err != nil {
		return WorkRoomAutoPullQRCode{}, err
	}
	created.ConfigID = strings.TrimSpace(created.ConfigID)
	if created.ConfigID == "" {
		return WorkRoomAutoPullQRCode{}, fmt.Errorf("企业微信未返回群活码配置ID")
	}
	var detail struct {
		weComBaseResponse
		JoinWay struct {
			ConfigID  string `json:"config_id"`
			QRCodeURL string `json:"qr_code"`
		} `json:"join_way"`
	}
	if err := c.client.postJSON(ctx, "cgi-bin/externalcontact/groupchat/get_join_way", token, map[string]any{"config_id": created.ConfigID}, &detail); err != nil {
		cleanupCtx, cancel := workRoomAutoPullCleanupContext(ctx)
		cleanupErr := c.deleteWithToken(cleanupCtx, token, created.ConfigID)
		cancel()
		if cleanupErr != nil {
			return WorkRoomAutoPullQRCode{}, fmt.Errorf("%w；企业微信配置清理失败：%v", err, cleanupErr)
		}
		return WorkRoomAutoPullQRCode{}, err
	}
	detail.JoinWay.QRCodeURL = strings.TrimSpace(detail.JoinWay.QRCodeURL)
	if detail.JoinWay.QRCodeURL == "" {
		cleanupCtx, cancel := workRoomAutoPullCleanupContext(ctx)
		cleanupErr := c.deleteWithToken(cleanupCtx, token, created.ConfigID)
		cancel()
		if cleanupErr != nil {
			return WorkRoomAutoPullQRCode{}, fmt.Errorf("企业微信未返回群活码二维码；配置清理失败：%v", cleanupErr)
		}
		return WorkRoomAutoPullQRCode{}, fmt.Errorf("企业微信未返回群活码二维码")
	}
	return WorkRoomAutoPullQRCode{ConfigID: created.ConfigID, QRCodeURL: detail.JoinWay.QRCodeURL}, nil
}

func workRoomAutoPullCleanupContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(parent), 10*time.Second)
}

func (c *WorkRoomAutoPullJoinWayWeComClient) UpdateJoinWay(ctx context.Context, credential RoomWelcomeCorpCredential, configID string, payload WorkRoomAutoPullJoinWayPayload) error {
	token, err := c.client.accessToken(ctx, credential)
	if err != nil {
		return err
	}
	var response weComBaseResponse
	return c.client.postJSON(ctx, "cgi-bin/externalcontact/groupchat/update_join_way", token, workRoomAutoPullJoinWayRequest(payload, configID), &response)
}

func (c *WorkRoomAutoPullJoinWayWeComClient) DeleteJoinWay(ctx context.Context, credential RoomWelcomeCorpCredential, configID string) error {
	token, err := c.client.accessToken(ctx, credential)
	if err != nil {
		return err
	}
	return c.deleteWithToken(ctx, token, configID)
}

func (c *WorkRoomAutoPullJoinWayWeComClient) deleteWithToken(ctx context.Context, token, configID string) error {
	var response weComBaseResponse
	return c.client.postJSON(ctx, "cgi-bin/externalcontact/groupchat/del_join_way", token, map[string]any{"config_id": strings.TrimSpace(configID)}, &response)
}

func workRoomAutoPullJoinWayRequest(payload WorkRoomAutoPullJoinWayPayload, configID string) map[string]any {
	request := map[string]any{
		"scene":            2,
		"remark":           strings.TrimSpace(payload.QRCodeName),
		"auto_create_room": 0,
		"chat_id_list":     append([]string{}, payload.ChatIDs...),
	}
	if strings.TrimSpace(configID) != "" {
		request["config_id"] = strings.TrimSpace(configID)
	}
	if payload.AutoCreateRoom {
		request["auto_create_room"] = 1
		request["room_base_name"] = strings.TrimSpace(payload.RoomBaseName)
		request["room_base_id"] = payload.RoomBaseID
	}
	return request
}
