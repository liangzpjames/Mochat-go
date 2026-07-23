package dashboard

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

type WorkContactTagSyncGroup struct {
	WXGroupID string
	GroupName string
	Order     int
	Tags      []WorkContactTagSyncTag
}

type WorkContactTagSyncTag struct {
	WXContactTagID string
	Name           string
	Order          int
}

type WorkContactTagSyncResult struct {
	CreatedGroups int
	UpdatedGroups int
	DeletedGroups int
	CreatedTags   int
	UpdatedTags   int
	DeletedTags   int
}

type WorkContactTagSyncClient interface {
	CorpTags(ctx context.Context, credential RoomWelcomeCorpCredential) ([]WorkContactTagSyncGroup, error)
}

type WorkContactTagAddRequest struct {
	WXGroupID string
	GroupName string
	TagNames  []string
}

type WorkContactTagWriteClient interface {
	AddCorpTags(ctx context.Context, credential RoomWelcomeCorpCredential, request WorkContactTagAddRequest) (WorkContactTagSyncGroup, error)
	UpdateCorpTag(ctx context.Context, credential RoomWelcomeCorpCredential, wxContactTagID string, name string) error
	DeleteCorpTags(ctx context.Context, credential RoomWelcomeCorpCredential, wxContactTagIDs []string, wxGroupIDs []string) error
}

func (h *WorkReadHandler) WithWorkContactTagSyncClient(client WorkContactTagSyncClient) *WorkReadHandler {
	h.workContactTagSync = client
	return h
}

func (h *WorkReadHandler) WithWorkContactTagWriteClient(client WorkContactTagWriteClient) *WorkReadHandler {
	h.workContactTagWrite = client
	return h
}

func (h *WorkReadHandler) ContactTagSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, loginInfo, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	if err := h.authorize(r.Context(), r, userID, loginInfo); err != nil {
		writeAccessError(w, err)
		return
	}
	if len(loginInfo.CorpIDs) != 1 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "请先选择企业", nil)
		return
	}
	if h.workContactTagSync == nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "企业微信标签同步客户端未配置", nil)
		return
	}
	corpID := loginInfo.CorpIDs[0]
	credential, found, err := h.store.RoomWelcomeCorpCredentialByID(r.Context(), corpID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found || strings.TrimSpace(credential.WXCorpID) == "" || strings.TrimSpace(credential.ContactSecret) == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "企业授权信息错误", nil)
		return
	}
	groups, err := h.workContactTagSync.CorpTags(r.Context(), credential)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if _, err := h.store.SyncWorkContactTags(r.Context(), corpID, groups); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (c *RoomWelcomeWeComClient) CorpTags(ctx context.Context, credential RoomWelcomeCorpCredential) ([]WorkContactTagSyncGroup, error) {
	return c.corpTags(ctx, credential, nil, nil)
}

func (c *RoomWelcomeWeComClient) CorpTagsByIDs(ctx context.Context, credential RoomWelcomeCorpCredential, groupIDs []string, tagIDs []string) ([]WorkContactTagSyncGroup, error) {
	return c.corpTags(ctx, credential, groupIDs, tagIDs)
}

func (c *RoomWelcomeWeComClient) corpTags(ctx context.Context, credential RoomWelcomeCorpCredential, groupIDs []string, tagIDs []string) ([]WorkContactTagSyncGroup, error) {
	token, err := c.accessToken(ctx, credential)
	if err != nil {
		return nil, err
	}
	var response struct {
		weComBaseResponse
		TagGroups []struct {
			GroupID   string `json:"group_id"`
			GroupName string `json:"group_name"`
			Order     int    `json:"order"`
			Tags      []struct {
				ID    string `json:"id"`
				Name  string `json:"name"`
				Order int    `json:"order"`
			} `json:"tag"`
		} `json:"tag_group"`
	}
	payload := map[string]any{}
	if len(groupIDs) > 0 {
		payload["group_id"] = uniqueNonEmptyStrings(groupIDs)
	}
	if len(tagIDs) > 0 {
		payload["tag_id"] = uniqueNonEmptyStrings(tagIDs)
	}
	if err := c.postJSON(ctx, "cgi-bin/externalcontact/get_corp_tag_list", token, payload, &response); err != nil {
		return nil, err
	}
	groups := make([]WorkContactTagSyncGroup, 0, len(response.TagGroups))
	for _, group := range response.TagGroups {
		item := WorkContactTagSyncGroup{
			WXGroupID: strings.TrimSpace(group.GroupID),
			GroupName: group.GroupName,
			Order:     group.Order,
			Tags:      make([]WorkContactTagSyncTag, 0, len(group.Tags)),
		}
		for _, tag := range group.Tags {
			item.Tags = append(item.Tags, WorkContactTagSyncTag{
				WXContactTagID: strings.TrimSpace(tag.ID),
				Name:           tag.Name,
				Order:          tag.Order,
			})
		}
		groups = append(groups, item)
	}
	return groups, nil
}

func (c *RoomWelcomeWeComClient) AddCorpTags(ctx context.Context, credential RoomWelcomeCorpCredential, request WorkContactTagAddRequest) (WorkContactTagSyncGroup, error) {
	token, err := c.accessToken(ctx, credential)
	if err != nil {
		return WorkContactTagSyncGroup{}, err
	}
	tagPayload := make([]map[string]string, 0, len(request.TagNames))
	for _, name := range request.TagNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		tagPayload = append(tagPayload, map[string]string{"name": name})
	}
	if len(tagPayload) == 0 {
		return WorkContactTagSyncGroup{}, errors.New("标签名称必传")
	}
	payload := map[string]any{"tag": tagPayload}
	if wxGroupID := strings.TrimSpace(request.WXGroupID); wxGroupID != "" {
		payload["group_id"] = wxGroupID
	} else if groupName := strings.TrimSpace(request.GroupName); groupName != "" {
		payload["group_name"] = groupName
	} else {
		return WorkContactTagSyncGroup{}, errors.New("标签分组信息缺失")
	}

	var response struct {
		weComBaseResponse
		TagGroup struct {
			GroupID   string `json:"group_id"`
			GroupName string `json:"group_name"`
			Order     int    `json:"order"`
			Tags      []struct {
				ID    string `json:"id"`
				Name  string `json:"name"`
				Order int    `json:"order"`
			} `json:"tag"`
		} `json:"tag_group"`
	}
	if err := c.postJSON(ctx, "cgi-bin/externalcontact/add_corp_tag", token, payload, &response); err != nil {
		return WorkContactTagSyncGroup{}, err
	}
	group := WorkContactTagSyncGroup{
		WXGroupID: strings.TrimSpace(response.TagGroup.GroupID),
		GroupName: response.TagGroup.GroupName,
		Order:     response.TagGroup.Order,
		Tags:      make([]WorkContactTagSyncTag, 0, len(response.TagGroup.Tags)),
	}
	for _, tag := range response.TagGroup.Tags {
		group.Tags = append(group.Tags, WorkContactTagSyncTag{
			WXContactTagID: strings.TrimSpace(tag.ID),
			Name:           tag.Name,
			Order:          tag.Order,
		})
	}
	return group, nil
}

func (c *RoomWelcomeWeComClient) UpdateCorpTag(ctx context.Context, credential RoomWelcomeCorpCredential, wxContactTagID string, name string) error {
	token, err := c.accessToken(ctx, credential)
	if err != nil {
		return err
	}
	payload := map[string]any{
		"id":   strings.TrimSpace(wxContactTagID),
		"name": strings.TrimSpace(name),
	}
	var response weComBaseResponse
	return c.postJSON(ctx, "cgi-bin/externalcontact/edit_corp_tag", token, payload, &response)
}

func (c *RoomWelcomeWeComClient) DeleteCorpTags(ctx context.Context, credential RoomWelcomeCorpCredential, wxContactTagIDs []string, wxGroupIDs []string) error {
	tagIDs := uniqueNonEmptyStrings(wxContactTagIDs)
	groupIDs := uniqueNonEmptyStrings(wxGroupIDs)
	if len(tagIDs) == 0 && len(groupIDs) == 0 {
		return nil
	}
	token, err := c.accessToken(ctx, credential)
	if err != nil {
		return err
	}
	payload := map[string]any{
		"tag_id":   tagIDs,
		"group_id": groupIDs,
	}
	var response weComBaseResponse
	return c.postJSON(ctx, "cgi-bin/externalcontact/del_corp_tag", token, payload, &response)
}
