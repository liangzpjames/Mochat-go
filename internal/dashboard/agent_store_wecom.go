package dashboard

import (
	"context"
	"net/url"
)

func (c *RoomWelcomeWeComClient) AgentDetail(ctx context.Context, credential RoomWelcomeCorpCredential, wxSecret string, wxAgentID string) (WorkAgentDetail, error) {
	token, err := c.accessToken(ctx, RoomWelcomeCorpCredential{
		WXCorpID:      credential.WXCorpID,
		ContactSecret: wxSecret,
	})
	if err != nil {
		return WorkAgentDetail{}, err
	}
	var response struct {
		weComBaseResponse
		Name               string `json:"name"`
		SquareLogoURL      string `json:"square_logo_url"`
		Description        string `json:"description"`
		Close              int    `json:"close"`
		RedirectDomain     string `json:"redirect_domain"`
		ReportLocationFlag int    `json:"report_location_flag"`
		IsReportEnter      int    `json:"isreportenter"`
		HomeURL            string `json:"home_url"`
	}
	if err := c.getJSON(ctx, "cgi-bin/agent/get", token, url.Values{"agentid": {wxAgentID}}, &response); err != nil {
		return WorkAgentDetail{}, err
	}
	return WorkAgentDetail{
		Name:               response.Name,
		SquareLogoURL:      response.SquareLogoURL,
		Description:        response.Description,
		Close:              response.Close,
		RedirectDomain:     response.RedirectDomain,
		ReportLocationFlag: response.ReportLocationFlag,
		IsReportEnter:      response.IsReportEnter,
		HomeURL:            response.HomeURL,
	}, nil
}
