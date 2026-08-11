package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type WorkContactSyncEmployee struct {
	ID       int
	WXUserID string
}

type WorkContactSyncTag struct {
	WXContactTagID string
	GroupName      string
	TagName        string
	Type           int
}

type WorkContactSyncFollowUser struct {
	UserID         string
	Remark         string
	Description    string
	RemarkCorpName string
	RemarkMobiles  []string
	AddWay         int
	OperUserID     string
	State          string
	CreateTime     int64
	Tags           []WorkContactSyncTag
}

type WorkContactSyncContact struct {
	WXExternalUserID string
	Name             string
	Avatar           string
	Type             int
	Gender           int
	UnionID          string
	Position         string
	CorpName         string
	CorpFullName     string
	ExternalProfile  json.RawMessage
	BusinessNo       string
	FollowUsers      []WorkContactSyncFollowUser
}

type WorkContactSyncEmployeeContacts struct {
	Employee        WorkContactSyncEmployee
	ExternalUserIDs []string
	Contacts        []WorkContactSyncContact
	NoContact       bool
}

type WorkContactSyncResult struct {
	ContactID        int
	ContactWasNew    bool
	ContactsCreated  int
	ContactsUpdated  int
	RelationsCreated int
	RelationsUpdated int
	RelationsRemoved int
	TagsCreated      int
	TagsUpdated      int
	PivotsCreated    int
	PivotsDeleted    int
}

type WorkContactSyncClient interface {
	ExternalContactList(ctx context.Context, credential RoomWelcomeCorpCredential, wxUserID string) ([]string, bool, error)
	ExternalContactDetail(ctx context.Context, credential RoomWelcomeCorpCredential, wxExternalUserID string) (WorkContactSyncContact, error)
}

func (h *WorkReadHandler) WithWorkContactSyncClient(client WorkContactSyncClient) *WorkReadHandler {
	h.workContactSync = client
	return h
}

func (h *WorkReadHandler) WorkContactSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if err := h.authorize(r.Context(), r, userID, principalScope); err != nil {
		writeAccessError(w, err)
		return
	}
	if len(principalScope.CorpIDs) != 1 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "请先选择企业", nil)
		return
	}
	if h.workContactSync == nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "企业微信客户同步客户端未配置", nil)
		return
	}
	corpID := principalScope.CorpIDs[0]
	credential, found, err := h.store.RoomWelcomeCorpCredentialByID(r.Context(), corpID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found || strings.TrimSpace(credential.WXCorpID) == "" || strings.TrimSpace(credential.ContactSecret) == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "企业授权信息错误", nil)
		return
	}
	employees, err := h.store.WorkContactSyncEmployees(r.Context(), corpID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if len(employees) == 0 {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "查询不到有效的企业微信成员信息", nil)
		return
	}

	bundles := make([]WorkContactSyncEmployeeContacts, 0, len(employees))
	for _, employee := range employees {
		wxUserID := strings.TrimSpace(employee.WXUserID)
		if employee.ID <= 0 || wxUserID == "" {
			continue
		}
		employee.WXUserID = wxUserID
		externalUserIDs, noContact, err := h.workContactSync.ExternalContactList(r.Context(), credential, wxUserID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		externalUserIDs = uniqueNonEmptyStrings(externalUserIDs)
		bundle := WorkContactSyncEmployeeContacts{
			Employee:        employee,
			ExternalUserIDs: externalUserIDs,
			NoContact:       noContact,
		}
		if !noContact && len(externalUserIDs) > 0 {
			for _, wxExternalUserID := range externalUserIDs {
				contact, err := h.workContactSync.ExternalContactDetail(r.Context(), credential, wxExternalUserID)
				if err != nil {
					writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
					return
				}
				if strings.TrimSpace(contact.WXExternalUserID) != "" {
					bundle.Contacts = append(bundle.Contacts, contact)
				}
			}
		}
		bundles = append(bundles, bundle)
	}
	if _, err := h.store.SyncWorkContacts(r.Context(), corpID, bundles); err != nil {
		if writeSaaSQuotaError(w, err) {
			return
		}
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (c *RoomWelcomeWeComClient) ExternalContactList(ctx context.Context, credential RoomWelcomeCorpCredential, wxUserID string) ([]string, bool, error) {
	token, err := c.accessToken(ctx, credential)
	if err != nil {
		return nil, false, err
	}
	var response struct {
		weComBaseResponse
		ExternalUserIDs []string `json:"external_userid"`
	}
	err = c.getJSONRaw(ctx, "cgi-bin/externalcontact/list", token, url.Values{"userid": {wxUserID}}, &response)
	if err != nil {
		return nil, false, err
	}
	if response.ErrCode == 84061 {
		return []string{}, true, nil
	}
	if err := response.Err(); err != nil {
		return nil, false, err
	}
	return response.ExternalUserIDs, false, nil
}

func (c *RoomWelcomeWeComClient) ExternalContactDetail(ctx context.Context, credential RoomWelcomeCorpCredential, wxExternalUserID string) (WorkContactSyncContact, error) {
	token, err := c.accessToken(ctx, credential)
	if err != nil {
		return WorkContactSyncContact{}, err
	}
	var response struct {
		weComBaseResponse
		ExternalContact struct {
			ExternalUserID  string          `json:"external_userid"`
			Name            string          `json:"name"`
			Avatar          string          `json:"avatar"`
			Type            int             `json:"type"`
			Gender          int             `json:"gender"`
			UnionID         string          `json:"unionid"`
			Position        string          `json:"position"`
			CorpName        string          `json:"corp_name"`
			CorpFullName    string          `json:"corp_full_name"`
			ExternalProfile json.RawMessage `json:"external_profile"`
			BusinessNo      string          `json:"business_no"`
		} `json:"external_contact"`
		FollowUsers []struct {
			UserID         string   `json:"userid"`
			Remark         string   `json:"remark"`
			Description    string   `json:"description"`
			RemarkCorpName string   `json:"remark_corp_name"`
			RemarkMobiles  []string `json:"remark_mobiles"`
			AddWay         int      `json:"add_way"`
			OperUserID     string   `json:"oper_userid"`
			State          string   `json:"state"`
			CreateTime     int64    `json:"createtime"`
			Tags           []struct {
				TagID     string `json:"tag_id"`
				GroupName string `json:"group_name"`
				TagName   string `json:"tag_name"`
				Type      int    `json:"type"`
			} `json:"tags"`
		} `json:"follow_user"`
	}
	if err := c.getJSON(ctx, "cgi-bin/externalcontact/get", token, url.Values{"external_userid": {wxExternalUserID}}, &response); err != nil {
		return WorkContactSyncContact{}, err
	}
	contact := WorkContactSyncContact{
		WXExternalUserID: strings.TrimSpace(response.ExternalContact.ExternalUserID),
		Name:             response.ExternalContact.Name,
		Avatar:           response.ExternalContact.Avatar,
		Type:             response.ExternalContact.Type,
		Gender:           response.ExternalContact.Gender,
		UnionID:          response.ExternalContact.UnionID,
		Position:         response.ExternalContact.Position,
		CorpName:         response.ExternalContact.CorpName,
		CorpFullName:     response.ExternalContact.CorpFullName,
		ExternalProfile:  response.ExternalContact.ExternalProfile,
		BusinessNo:       response.ExternalContact.BusinessNo,
		FollowUsers:      make([]WorkContactSyncFollowUser, 0, len(response.FollowUsers)),
	}
	for _, follow := range response.FollowUsers {
		item := WorkContactSyncFollowUser{
			UserID:         strings.TrimSpace(follow.UserID),
			Remark:         follow.Remark,
			Description:    follow.Description,
			RemarkCorpName: follow.RemarkCorpName,
			RemarkMobiles:  follow.RemarkMobiles,
			AddWay:         follow.AddWay,
			OperUserID:     follow.OperUserID,
			State:          follow.State,
			CreateTime:     follow.CreateTime,
			Tags:           make([]WorkContactSyncTag, 0, len(follow.Tags)),
		}
		for _, tag := range follow.Tags {
			item.Tags = append(item.Tags, WorkContactSyncTag{
				WXContactTagID: strings.TrimSpace(tag.TagID),
				GroupName:      tag.GroupName,
				TagName:        tag.TagName,
				Type:           tag.Type,
			})
		}
		contact.FollowUsers = append(contact.FollowUsers, item)
	}
	return contact, nil
}

func (c *RoomWelcomeWeComClient) getJSONRaw(ctx context.Context, path string, accessToken string, extra url.Values, out any) error {
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
	return json.NewDecoder(resp.Body).Decode(out)
}
