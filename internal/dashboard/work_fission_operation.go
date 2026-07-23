package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type WorkFissionOperationFission struct {
	ID               int
	CorpID           int
	ServiceEmployees string
	AutoPass         int
	Tasks            string
	EndTime          string
	DeleteInvalid    int
	ReceivePrize     int
	ReceiveLinks     string
	ReceiveQRCode    string
}

type WorkFissionOperationContact struct {
	ID                        int
	FissionID                 int
	UnionID                   string
	Nickname                  string
	Avatar                    string
	ContactSuperiorUserParent int
	Level                     int
	Employee                  string
	InviteCount               int
	Loss                      int
	Status                    int
	ReceiveLevel              int
	IsNew                     int
	ExternalUserID            string
	QRCodeID                  string
	QRCodeURL                 string
	CreatedAt                 string
}

type WorkFissionOperationWorkContact struct {
	ID               int
	CorpID           int
	WXExternalUserID string
	Avatar           string
}

type WorkFissionOperationContactCreate struct {
	FissionID      int
	UnionID        string
	Nickname       string
	Avatar         string
	Level          int
	ExternalUserID string
}

type WorkFissionOperationStore interface {
	WorkFissionOperationFissionByID(ctx context.Context, id int) (WorkFissionOperationFission, bool, error)
	WorkFissionOperationPosterByFissionID(ctx context.Context, fissionID int) (WorkFissionPoster, bool, error)
	WorkFissionOperationContactByUnionID(ctx context.Context, unionID string) (WorkFissionOperationContact, bool, error)
	WorkFissionOperationContactByFissionUnionID(ctx context.Context, fissionID int, unionID string) (WorkFissionOperationContact, bool, error)
	WorkFissionOperationChildren(ctx context.Context, parentID int, fissionID int) ([]WorkFissionOperationContact, error)
	WorkFissionOperationWorkContactByUnionID(ctx context.Context, unionID string) (WorkFissionOperationWorkContact, bool, error)
	WorkFissionOperationInviteCount(ctx context.Context, parentID int, fissionID int, onlyNotLoss bool) (int, error)
	CreateWorkFissionOperationContact(ctx context.Context, values WorkFissionOperationContactCreate) (int, error)
	UpdateWorkFissionOperationContactQRCode(ctx context.Context, id int, qrcodeID string, qrcodeURL string) error
	UpdateWorkFissionOperationReceiveLevel(ctx context.Context, id int, level int) (bool, error)
	RoomWelcomeCorpCredentialByID(ctx context.Context, corpID int) (RoomWelcomeCorpCredential, bool, error)
}

type WorkFissionOperationContactWayClient interface {
	CreateContactWay(ctx context.Context, credential RoomWelcomeCorpCredential, userIDs []string, skipVerify bool, state string) (string, string, error)
}

type WorkFissionOperationHandler struct {
	store      WorkFissionOperationStore
	apiBaseURL string
	client     WorkFissionOperationContactWayClient
}

func NewWorkFissionOperationHandler(store WorkFissionOperationStore, apiBaseURL string, client WorkFissionOperationContactWayClient) *WorkFissionOperationHandler {
	return &WorkFissionOperationHandler{
		store:      store,
		apiBaseURL: strings.TrimRight(apiBaseURL, "/"),
		client:     client,
	}
}

func (h *WorkFissionOperationHandler) InviteFriends(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	unionID := strings.TrimSpace(r.URL.Query().Get("union_id"))
	if unionID == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "客户微信id 必填", nil)
		return
	}
	fissionID, ok := queryPositiveIntParam(w, r, "fission_id", "活动id 必填", "活动id 必须为整数")
	if !ok {
		return
	}
	parent, found, err := h.store.WorkFissionOperationContactByUnionID(r.Context(), unionID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusOK, 200, "success", []any{})
		return
	}
	fission, found, err := h.store.WorkFissionOperationFissionByID(r.Context(), fissionID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "活动不存在", nil)
		return
	}
	children, err := h.store.WorkFissionOperationChildren(r.Context(), parent.ID, fissionID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(children))
	for _, child := range children {
		avatar := child.Avatar
		if contact, found, err := h.store.WorkFissionOperationWorkContactByUnionID(r.Context(), child.UnionID); err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		} else if found {
			avatar = contact.Avatar
		}
		fail := 0
		if fission.DeleteInvalid == 1 && child.Loss == 1 {
			fail = 1
		}
		list = append(list, map[string]any{
			"id":         child.ID,
			"nickname":   child.Nickname,
			"unionId":    child.UnionID,
			"union_id":   child.UnionID,
			"avatar":     h.fullStaticURL(avatar),
			"createdAt":  child.CreatedAt,
			"created_at": child.CreatedAt,
			"loss":       child.Loss,
			"fail":       fail,
		})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", list)
}

func (h *WorkFissionOperationHandler) TaskData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	unionID := strings.TrimSpace(r.URL.Query().Get("union_id"))
	if unionID == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "客户微信id 必填", nil)
		return
	}
	fissionID, ok := queryPositiveIntParam(w, r, "fission_id", "活动id 必填", "活动id 必须为整数")
	if !ok {
		return
	}
	user, found, err := h.store.WorkFissionOperationContactByUnionID(r.Context(), unionID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "客户不存在", nil)
		return
	}
	fission, found, err := h.store.WorkFissionOperationFissionByID(r.Context(), fissionID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "活动不存在", nil)
		return
	}
	inviteCount, err := h.store.WorkFissionOperationInviteCount(r.Context(), user.ID, fissionID, fission.DeleteInvalid == 1)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	tasks, err := h.operationTasks(fission, user.ReceiveLevel, inviteCount)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"invite_count": inviteCount,
		"differ_count": workFissionOperationDifferCount(tasks, inviteCount),
		"end_time":     workFissionOperationUnix(fission.EndTime),
		"task":         tasks,
	})
}

func (h *WorkFissionOperationHandler) Receive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	params := valuesToParams(r.URL.Query())
	if r.Method == http.MethodPut {
		bodyParams, err := parseRequestParams(r)
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
			return
		}
		for key, value := range bodyParams {
			params[key] = value
		}
	}
	unionID := strings.TrimSpace(stringParam(params, "union_id"))
	if unionID == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "union_id必填", nil)
		return
	}
	level, has, err := intParam(params, "level")
	if err != nil || !has {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "level 必须为整数", nil)
		return
	}
	user, found, err := h.store.WorkFissionOperationContactByUnionID(r.Context(), unionID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "客户不存在", nil)
		return
	}
	if _, err := h.store.UpdateWorkFissionOperationReceiveLevel(r.Context(), user.ID, level); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *WorkFissionOperationHandler) Poster(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	fissionID, ok := queryPositiveIntParam(w, r, "fission_id", "活动id 必填", "活动id 必须为整数")
	if !ok {
		return
	}
	unionID := strings.TrimSpace(r.URL.Query().Get("union_id"))
	if unionID == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "union_id必填", nil)
		return
	}
	nickname := strings.TrimSpace(r.URL.Query().Get("nickname"))
	if nickname == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "nickname必填", nil)
		return
	}
	avatar := strings.TrimSpace(r.URL.Query().Get("avatar"))
	if avatar == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "avatar必填", nil)
		return
	}
	fission, found, err := h.store.WorkFissionOperationFissionByID(r.Context(), fissionID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "活动不存在", nil)
		return
	}
	poster, found, err := h.store.WorkFissionOperationPosterByFissionID(r.Context(), fissionID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "海报不存在", nil)
		return
	}
	qrcodeURL, err := h.ensurePosterQRCode(r.Context(), fission, unionID, nickname, avatar)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"posterType":        poster.PosterType,
		"coverPic":          h.fullStaticURL(poster.CoverPic),
		"fowardText":        poster.FowardText,
		"avatarShow":        poster.AvatarShow,
		"nicknameShow":      poster.NicknameShow,
		"nicknameColor":     poster.NicknameColor,
		"cardCorpImageName": poster.CardCorpImageName,
		"cardCorpName":      poster.CardCorpName,
		"cardCorpLogo":      h.fullStaticURL(poster.CardCorpLogo),
		"qrcodeW":           poster.QRCodeW,
		"qrcodeH":           poster.QRCodeH,
		"qrcodeX":           poster.QRCodeX,
		"qrcodeY":           poster.QRCodeY,
		"qrcodeId":          poster.QRCodeID,
		"qrcodeUrl":         h.fullStaticURL(qrcodeURL),
	})
}

func (h *WorkFissionOperationHandler) ensurePosterQRCode(ctx context.Context, fission WorkFissionOperationFission, unionID string, nickname string, avatar string) (string, error) {
	contact, found, err := h.store.WorkFissionOperationContactByFissionUnionID(ctx, fission.ID, unionID)
	if err != nil {
		return "", err
	}
	if !found {
		level := 0
		externalUserID := ""
		if workContact, found, err := h.store.WorkFissionOperationWorkContactByUnionID(ctx, unionID); err != nil {
			return "", err
		} else if found {
			level = 1
			externalUserID = workContact.WXExternalUserID
		}
		id, err := h.store.CreateWorkFissionOperationContact(ctx, WorkFissionOperationContactCreate{
			FissionID:      fission.ID,
			UnionID:        unionID,
			Nickname:       nickname,
			Avatar:         avatar,
			Level:          level,
			ExternalUserID: externalUserID,
		})
		if err != nil {
			return "", err
		}
		contact = WorkFissionOperationContact{ID: id, FissionID: fission.ID, UnionID: unionID}
	}
	if strings.TrimSpace(contact.QRCodeURL) != "" {
		return contact.QRCodeURL, nil
	}
	qrcodeURL, err := h.createPosterQRCode(ctx, fission, contact.ID)
	if err != nil {
		return "", err
	}
	if err := h.store.UpdateWorkFissionOperationContactQRCode(ctx, contact.ID, qrcodeURL, qrcodeURL); err != nil {
		return "", err
	}
	return qrcodeURL, nil
}

func (h *WorkFissionOperationHandler) createPosterQRCode(ctx context.Context, fission WorkFissionOperationFission, contactID int) (string, error) {
	if h.client == nil {
		return "", fmt.Errorf("企业微信客户端未配置")
	}
	credential, found, err := h.store.RoomWelcomeCorpCredentialByID(ctx, fission.CorpID)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("企业微信配置不存在")
	}
	userIDs, err := workFissionWXUserIDs(map[string]any{"service_employees": fission.ServiceEmployees}, "service_employees")
	if err != nil {
		return "", err
	}
	qrcodeURL, _, err := h.client.CreateContactWay(ctx, credential, userIDs, fission.AutoPass == 1, fmt.Sprintf("fission-%d", contactID))
	if err != nil {
		return "", fmt.Errorf("请求失败，部分员工未进行实名认证，请实名后重试")
	}
	if strings.TrimSpace(qrcodeURL) == "" {
		return "", fmt.Errorf("生成二维码失败")
	}
	return qrcodeURL, nil
}

func (h *WorkFissionOperationHandler) operationTasks(fission WorkFissionOperationFission, receiveLevel int, inviteCount int) ([]map[string]any, error) {
	sourceTasks, err := workFissionOperationJSONList(fission.Tasks)
	if err != nil {
		return nil, err
	}
	receiveLinks, _ := workFissionOperationJSONList(fission.ReceiveLinks)
	receiveQRCodeURL := workFissionOperationQRCodeURL(fission.ReceiveQRCode)
	tasks := make([]map[string]any, 0, len(sourceTasks))
	for index, rawTask := range sourceTasks {
		count := workFissionOperationCount(rawTask)
		item := map[string]any{
			"count":          count,
			"status":         0,
			"receive_status": 0,
			"gift_type":      fission.ReceivePrize,
			"gift_url":       "",
		}
		if inviteCount >= count {
			item["status"] = 1
		}
		if receiveLevel > 0 && receiveLevel >= index+1 {
			item["receive_status"] = 1
		}
		if fission.ReceivePrize == 0 {
			item["gift_url"] = h.fullStaticURL(receiveQRCodeURL)
		}
		if fission.ReceivePrize == 1 && index < len(receiveLinks) {
			item["gift_url"] = workFissionOperationGiftURL(receiveLinks[index])
		}
		tasks = append(tasks, item)
	}
	return tasks, nil
}

func (h *WorkFissionOperationHandler) fullStaticURL(path string) string {
	if path == "" || strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") || h.apiBaseURL == "" {
		return path
	}
	return h.apiBaseURL + "/static/" + strings.TrimLeft(path, "/")
}

func workFissionOperationJSONList(raw string) ([]map[string]any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return []map[string]any{}, nil
	}
	var items []map[string]any
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&items); err != nil {
		return nil, err
	}
	return items, nil
}

func workFissionOperationCount(task map[string]any) int {
	if count, ok, err := intParam(task, "count"); err == nil && ok {
		return count
	}
	if count, ok, err := intParam(task, "num"); err == nil && ok {
		return count
	}
	return 0
}

func workFissionOperationGiftURL(item map[string]any) string {
	if url := stringParam(item, "url"); url != "" {
		return url
	}
	if link := stringParam(item, "link"); link != "" {
		return link
	}
	if value := stringParam(item, "value"); value != "" {
		return value
	}
	raw, _ := json.Marshal(item)
	return string(raw)
}

func workFissionOperationQRCodeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" || raw == "[]" {
		return ""
	}
	var item map[string]any
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&item); err != nil {
		return ""
	}
	return stringParam(item, "url")
}

func workFissionOperationDifferCount(tasks []map[string]any, inviteCount int) int {
	for _, task := range tasks {
		count, _, _ := intParam(task, "count")
		if count > inviteCount {
			return count - inviteCount
		}
	}
	return 0
}

func workFissionOperationUnix(raw string) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	parsed, err := time.ParseInLocation("2006-01-02 15:04:05", raw, time.Local)
	if err != nil {
		return 0
	}
	return parsed.Unix()
}
