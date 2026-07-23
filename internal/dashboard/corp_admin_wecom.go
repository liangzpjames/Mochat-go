package dashboard

import (
	"context"
	"fmt"
	"net/url"
)

func (c *RoomWelcomeWeComClient) ValidateCorpSecrets(ctx context.Context, wxCorpID string, employeeSecret string, contactSecret string) error {
	employeeToken, err := c.accessToken(ctx, RoomWelcomeCorpCredential{WXCorpID: wxCorpID, ContactSecret: employeeSecret})
	if err != nil {
		return fmt.Errorf("通讯录管理secret或企业ID无效")
	}
	var employeeResp weComBaseResponse
	if err := c.getJSON(ctx, "cgi-bin/user/get", employeeToken, url.Values{"userid": {"1"}}, &employeeResp); err != nil {
		return fmt.Errorf("通讯录管理secret或企业ID无效")
	}
	contactToken, err := c.accessToken(ctx, RoomWelcomeCorpCredential{WXCorpID: wxCorpID, ContactSecret: contactSecret})
	if err != nil {
		return fmt.Errorf("外部联系人管理secret无效")
	}
	var contactResp weComBaseResponse
	if err := c.getJSON(ctx, "cgi-bin/externalcontact/get", contactToken, url.Values{"external_userid": {"1"}}, &contactResp); err != nil {
		return fmt.Errorf("外部联系人管理secret无效")
	}
	return nil
}
