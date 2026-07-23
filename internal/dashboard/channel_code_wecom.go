package dashboard

import (
	"context"
)

func (c *RoomWelcomeWeComClient) CreateContactWay(ctx context.Context, credential RoomWelcomeCorpCredential, userIDs []string, skipVerify bool, state string) (string, string, error) {
	token, err := c.accessToken(ctx, credential)
	if err != nil {
		return "", "", err
	}
	request := map[string]any{
		"type":        2,
		"scene":       2,
		"skip_verify": skipVerify,
		"state":       state,
		"user":        userIDs,
	}
	var response struct {
		weComBaseResponse
		QRCode   string `json:"qr_code"`
		ConfigID string `json:"config_id"`
	}
	if err := c.postJSON(ctx, "cgi-bin/externalcontact/add_contact_way", token, request, &response); err != nil {
		return "", "", err
	}
	return response.QRCode, response.ConfigID, nil
}

func (c *RoomWelcomeWeComClient) UpdateContactWay(ctx context.Context, credential RoomWelcomeCorpCredential, configID string, userIDs []string, skipVerify bool, state string) error {
	token, err := c.accessToken(ctx, credential)
	if err != nil {
		return err
	}
	request := map[string]any{
		"config_id":   configID,
		"skip_verify": skipVerify,
		"state":       state,
		"user":        userIDs,
	}
	var response weComBaseResponse
	return c.postJSON(ctx, "cgi-bin/externalcontact/update_contact_way", token, request, &response)
}
