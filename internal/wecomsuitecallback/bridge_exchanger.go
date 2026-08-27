package wecomsuitecallback

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"jiyi/mochat-go/internal/archivefixture"
)

type BridgeAuthorizationExchanger struct {
	endpoint string
	bearer   string
	client   *http.Client
}

func NewBridgeAuthorizationExchanger(baseURL, bearer string, client *http.Client) (*BridgeAuthorizationExchanger, error) {
	baseURL, bearer = strings.TrimRight(strings.TrimSpace(baseURL), "/"), strings.TrimSpace(bearer)
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || len(bearer) < 40 {
		return nil, errors.New("WeCom suite authorization bridge configuration is invalid")
	}
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &BridgeAuthorizationExchanger{endpoint: baseURL + "/v1/suite/authorization/exchange", bearer: bearer, client: client}, nil
}

func (exchanger *BridgeAuthorizationExchanger) ExchangeAuthorization(ctx context.Context, suiteID, suiteSecret, ticket, authCode string) (archivefixture.SuiteAuthorization, error) {
	raw, err := json.Marshal(map[string]string{"suiteId": suiteID, "suiteSecret": suiteSecret, "suiteTicket": ticket, "authCode": authCode})
	if err != nil {
		return archivefixture.SuiteAuthorization{}, errors.New("WeCom suite authorization request failed")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, exchanger.endpoint, bytes.NewReader(raw))
	if err != nil {
		return archivefixture.SuiteAuthorization{}, errors.New("WeCom suite authorization request failed")
	}
	request.Header.Set("Authorization", "Bearer "+exchanger.bearer)
	request.Header.Set("Content-Type", "application/json")
	response, err := exchanger.client.Do(request)
	if err != nil {
		return archivefixture.SuiteAuthorization{}, errors.New("WeCom suite authorization bridge unavailable")
	}
	defer response.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(response.Body, 64<<10))
	decoder.DisallowUnknownFields()
	var output struct {
		TenantID      int    `json:"tenantId"`
		CorpID        string `json:"corpId"`
		PermanentCode string `json:"permanentCode"`
	}
	if response.StatusCode != http.StatusOK || decoder.Decode(&output) != nil || output.TenantID <= 0 || strings.TrimSpace(output.CorpID) == "" || strings.TrimSpace(output.PermanentCode) == "" {
		return archivefixture.SuiteAuthorization{}, errors.New("WeCom suite authorization bridge rejected request")
	}
	return archivefixture.SuiteAuthorization{TenantID: output.TenantID, CorpID: strings.TrimSpace(output.CorpID), PermanentCode: strings.TrimSpace(output.PermanentCode)}, nil
}
