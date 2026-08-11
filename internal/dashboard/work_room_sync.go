package dashboard

import (
	"context"
	"net/http"
	"strings"
)

type WorkRoomSyncRoom struct {
	WXChatID   string
	Name       string
	Owner      string
	Notice     string
	Status     int
	CreateTime int64
	Members    []WorkRoomSyncMember
}

type WorkRoomSyncGroupChat struct {
	WXChatID string
	Status   int
}

type WorkRoomSyncMember struct {
	WXUserID  string
	Type      int
	JoinTime  int64
	JoinScene int
	UnionID   string
}

type WorkRoomSyncResult struct {
	RoomsCreated   int
	RoomsUpdated   int
	RoomsDeleted   int
	MembersCreated int
	MembersUpdated int
	MembersQuit    int
}

type WorkRoomSyncClient interface {
	GroupChats(ctx context.Context, credential RoomWelcomeCorpCredential) ([]WorkRoomSyncGroupChat, error)
	GroupChatDetail(ctx context.Context, credential RoomWelcomeCorpCredential, wxChatID string) (WorkRoomSyncRoom, error)
}

func (h *WorkReadHandler) WithWorkRoomSyncClient(client WorkRoomSyncClient) *WorkReadHandler {
	h.workRoomSync = client
	return h
}

func (h *WorkReadHandler) WorkRoomSync(w http.ResponseWriter, r *http.Request) {
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
	corpID, ok := principalCorpID(r)
	if !ok {
		return
	}
	if h.workRoomSync == nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "企业微信客户群同步客户端未配置", nil)
		return
	}
	credential, found, err := h.store.RoomWelcomeCorpCredentialByID(r.Context(), corpID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found || strings.TrimSpace(credential.WXCorpID) == "" || strings.TrimSpace(credential.ContactSecret) == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "企业授权信息错误", nil)
		return
	}

	groupChats, err := h.workRoomSync.GroupChats(r.Context(), credential)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	groupChats = uniqueWorkRoomSyncGroupChats(groupChats)
	if len(groupChats) == 0 {
		writeEnvelope(w, http.StatusOK, 200, "success", []any{})
		return
	}

	rooms := make([]WorkRoomSyncRoom, 0, len(groupChats))
	for _, groupChat := range groupChats {
		room, err := h.workRoomSync.GroupChatDetail(r.Context(), credential, groupChat.WXChatID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		if strings.TrimSpace(room.WXChatID) == "" {
			room.WXChatID = groupChat.WXChatID
		}
		room.Status = groupChat.Status
		if strings.TrimSpace(room.WXChatID) != "" {
			rooms = append(rooms, room)
		}
	}
	if _, err := h.store.SyncWorkRooms(r.Context(), corpID, rooms); err != nil {
		if writeSaaSQuotaError(w, err) {
			return
		}
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (c *RoomWelcomeWeComClient) GroupChats(ctx context.Context, credential RoomWelcomeCorpCredential) ([]WorkRoomSyncGroupChat, error) {
	token, err := c.accessToken(ctx, credential)
	if err != nil {
		return nil, err
	}
	chats := make([]WorkRoomSyncGroupChat, 0)
	cursor := ""
	for {
		payload := map[string]any{
			"status_filter": 0,
			"owner_filter": map[string]any{
				"userid_list":  []string{},
				"partyid_list": []int{},
			},
			"limit": 100,
		}
		if cursor != "" {
			payload["cursor"] = cursor
		}
		var response struct {
			weComBaseResponse
			GroupChats []struct {
				ChatID string `json:"chat_id"`
				Status int    `json:"status"`
			} `json:"group_chat_list"`
			NextCursor string `json:"next_cursor"`
		}
		if err := c.postJSON(ctx, "cgi-bin/externalcontact/groupchat/list", token, payload, &response); err != nil {
			return nil, err
		}
		for _, chat := range response.GroupChats {
			chats = append(chats, WorkRoomSyncGroupChat{WXChatID: strings.TrimSpace(chat.ChatID), Status: chat.Status})
		}
		cursor = strings.TrimSpace(response.NextCursor)
		if cursor == "" {
			break
		}
	}
	return chats, nil
}

func (c *RoomWelcomeWeComClient) GroupChatDetail(ctx context.Context, credential RoomWelcomeCorpCredential, wxChatID string) (WorkRoomSyncRoom, error) {
	token, err := c.accessToken(ctx, credential)
	if err != nil {
		return WorkRoomSyncRoom{}, err
	}
	var response struct {
		weComBaseResponse
		GroupChat struct {
			ChatID     string `json:"chat_id"`
			Name       string `json:"name"`
			Owner      string `json:"owner"`
			Notice     string `json:"notice"`
			CreateTime int64  `json:"create_time"`
			MemberList []struct {
				UserID    string `json:"userid"`
				Type      int    `json:"type"`
				JoinTime  int64  `json:"join_time"`
				JoinScene int    `json:"join_scene"`
				UnionID   string `json:"unionid"`
			} `json:"member_list"`
		} `json:"group_chat"`
	}
	if err := c.postJSON(ctx, "cgi-bin/externalcontact/groupchat/get", token, map[string]any{
		"chat_id":   wxChatID,
		"need_name": 1,
	}, &response); err != nil {
		return WorkRoomSyncRoom{}, err
	}
	room := WorkRoomSyncRoom{
		WXChatID:   strings.TrimSpace(response.GroupChat.ChatID),
		Name:       response.GroupChat.Name,
		Owner:      strings.TrimSpace(response.GroupChat.Owner),
		Notice:     response.GroupChat.Notice,
		CreateTime: response.GroupChat.CreateTime,
		Members:    make([]WorkRoomSyncMember, 0, len(response.GroupChat.MemberList)),
	}
	for _, member := range response.GroupChat.MemberList {
		room.Members = append(room.Members, WorkRoomSyncMember{
			WXUserID:  strings.TrimSpace(member.UserID),
			Type:      member.Type,
			JoinTime:  member.JoinTime,
			JoinScene: member.JoinScene,
			UnionID:   member.UnionID,
		})
	}
	return room, nil
}

func uniqueWorkRoomSyncGroupChats(chats []WorkRoomSyncGroupChat) []WorkRoomSyncGroupChat {
	seen := map[string]struct{}{}
	result := make([]WorkRoomSyncGroupChat, 0, len(chats))
	for _, chat := range chats {
		chat.WXChatID = strings.TrimSpace(chat.WXChatID)
		if chat.WXChatID == "" {
			continue
		}
		if _, exists := seen[chat.WXChatID]; exists {
			continue
		}
		seen[chat.WXChatID] = struct{}{}
		result = append(result, chat)
	}
	return result
}
