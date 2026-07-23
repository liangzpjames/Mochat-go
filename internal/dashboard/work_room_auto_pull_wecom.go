package dashboard

import "context"

type WorkRoomAutoPullWeComClient struct {
	client *RoomWelcomeWeComClient
}

func NewWorkRoomAutoPullWeComClient(baseURL string) *WorkRoomAutoPullWeComClient {
	return &WorkRoomAutoPullWeComClient{client: NewRoomWelcomeWeComClient(baseURL)}
}

func (c *WorkRoomAutoPullWeComClient) CreateContactWay(ctx context.Context, credential RoomWelcomeCorpCredential, userIDs []string, skipVerify bool, state string) (WorkRoomAutoPullQRCode, error) {
	token, err := c.client.accessToken(ctx, credential)
	if err != nil {
		return WorkRoomAutoPullQRCode{}, err
	}
	var response struct {
		weComBaseResponse
		ConfigID  string `json:"config_id"`
		QRCodeURL string `json:"qr_code"`
	}
	payload := map[string]any{
		"type":  2,
		"scene": 2,
		"config": map[string]any{
			"skip_verify": skipVerify,
			"state":       state,
			"user":        userIDs,
		},
	}
	if err := c.client.postJSON(ctx, "cgi-bin/externalcontact/contact_way/create", token, payload, &response); err != nil {
		return WorkRoomAutoPullQRCode{}, err
	}
	return WorkRoomAutoPullQRCode{ConfigID: response.ConfigID, QRCodeURL: response.QRCodeURL}, nil
}

func (c *WorkRoomAutoPullWeComClient) UpdateContactWay(ctx context.Context, credential RoomWelcomeCorpCredential, configID string, userIDs []string, skipVerify bool, state string) error {
	token, err := c.client.accessToken(ctx, credential)
	if err != nil {
		return err
	}
	var response weComBaseResponse
	payload := map[string]any{
		"config_id":   configID,
		"skip_verify": skipVerify,
		"state":       state,
		"user":        userIDs,
	}
	return c.client.postJSON(ctx, "cgi-bin/externalcontact/contact_way/update", token, payload, &response)
}
