package dashboard

import (
	"context"
	"fmt"
	"net/url"
)

type WeComCompanyVerificationResult struct {
	WXCorpID string
	CorpName string
}

// VerifyCompany validates both company credentials and the provider-returned
// company name. The caller must persist only the returned name after the
// binding/version transaction succeeds.
func (c *RoomWelcomeWeComClient) VerifyCompany(ctx context.Context, wxCorpID string, employeeSecret string, contactSecret string) (WeComCompanyVerificationResult, error) {
	employeeToken, err := c.accessToken(ctx, RoomWelcomeCorpCredential{WXCorpID: wxCorpID, ContactSecret: employeeSecret})
	if err != nil {
		return WeComCompanyVerificationResult{}, fmt.Errorf("通讯录管理secret或企业ID无效")
	}
	var employeeResp struct {
		weComBaseResponse
		CorpName     string `json:"corp_name"`
		CorpFullName string `json:"corp_full_name"`
	}
	if err := c.getJSON(ctx, "cgi-bin/user/get", employeeToken, url.Values{"userid": {"1"}}, &employeeResp); err != nil {
		return WeComCompanyVerificationResult{}, fmt.Errorf("通讯录管理secret或企业ID无效")
	}
	contactToken, err := c.accessToken(ctx, RoomWelcomeCorpCredential{WXCorpID: wxCorpID, ContactSecret: contactSecret})
	if err != nil {
		return WeComCompanyVerificationResult{}, fmt.Errorf("外部联系人管理secret无效")
	}
	var contactResp weComBaseResponse
	if err := c.getJSON(ctx, "cgi-bin/externalcontact/get", contactToken, url.Values{"external_userid": {"1"}}, &contactResp); err != nil {
		return WeComCompanyVerificationResult{}, fmt.Errorf("外部联系人管理secret无效")
	}
	name := employeeResp.CorpName
	if name == "" {
		name = employeeResp.CorpFullName
	}
	if name == "" {
		return WeComCompanyVerificationResult{}, fmt.Errorf("企业微信未返回企业名称")
	}
	return WeComCompanyVerificationResult{WXCorpID: wxCorpID, CorpName: name}, nil
}

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
