package dashboard

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type RoomWelcomeFilter struct {
	CorpID               int
	Text                 string
	Page                 int
	PerPage              int
	RestrictCreateUserID bool
	CreateUserID         int
}

type RoomWelcomeItem struct {
	ID                int
	CorpID            int
	MsgText           string
	ComplexType       string
	MsgComplex        string
	ComplexTemplateID string
	CreateUserID      int
	CreatedAt         string
	UpdatedAt         string
}

type RoomWelcomePage struct {
	Items     []RoomWelcomeItem
	Total     int
	TotalPage int
	PerPage   int
}

type RoomWelcomeWrite struct {
	CorpID            int
	MsgText           string
	ComplexType       string
	MsgComplex        string
	ComplexTemplateID string
	CreateUserID      int
}

type RoomWelcomeCorpCredential struct {
	CorpID        int
	WXCorpID      string
	ContactSecret string
}

type RoomWelcomeTemplatePayload struct {
	TextContent string
	Notify      int
	ImagePicURL string
	Link        *RoomWelcomeTemplateLink
	MiniProgram *RoomWelcomeTemplateMiniProgram
}

type RoomWelcomeTemplateLink struct {
	Title  string
	PicURL string
	Desc   string
	URL    string
}

type RoomWelcomeTemplateMiniProgram struct {
	Title      string
	PicMediaID string
	AppID      string
	Page       string
}

type RoomWelcomeTemplateClient interface {
	UploadImage(ctx context.Context, credential RoomWelcomeCorpCredential, filePath string) (string, error)
	UploadTemporaryImage(ctx context.Context, credential RoomWelcomeCorpCredential, filePath string) (string, error)
	CreateGroupWelcomeTemplate(ctx context.Context, credential RoomWelcomeCorpCredential, payload RoomWelcomeTemplatePayload) (string, error)
	UpdateGroupWelcomeTemplate(ctx context.Context, credential RoomWelcomeCorpCredential, templateID string, payload RoomWelcomeTemplatePayload) error
	DeleteGroupWelcomeTemplate(ctx context.Context, credential RoomWelcomeCorpCredential, templateID string) error
}

type RoomWelcomeStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	RoomWelcomePage(ctx context.Context, filter RoomWelcomeFilter) (RoomWelcomePage, error)
	RoomWelcomeByID(ctx context.Context, roomWelcomeID int) (RoomWelcomeItem, bool, error)
	RoomWelcomeCreatorNamesByIDs(ctx context.Context, userIDs []int) (map[int]string, error)
	RoomWelcomeCorpCredentialByID(ctx context.Context, corpID int) (RoomWelcomeCorpCredential, bool, error)
	CreateRoomWelcome(ctx context.Context, values RoomWelcomeWrite) (int, error)
	UpdateRoomWelcome(ctx context.Context, roomWelcomeID int, values RoomWelcomeWrite) (bool, error)
	DeleteRoomWelcome(ctx context.Context, roomWelcomeID int) (bool, error)
}

type RoomWelcomeHandler struct {
	store           RoomWelcomeStore
	cache           LoginCache
	resolver        UserIDResolver
	authorizer      CorpAdminAuthorizer
	apiBaseURL      string
	fileStorageRoot string
	templateClient  RoomWelcomeTemplateClient
}

func NewRoomWelcomeHandler(
	store RoomWelcomeStore,
	cache LoginCache,
	resolver UserIDResolver,
	authorizer CorpAdminAuthorizer,
	apiBaseURL string,
	fileStorageRoot string,
	templateClient RoomWelcomeTemplateClient,
) *RoomWelcomeHandler {
	return &RoomWelcomeHandler{
		store:           store,
		cache:           cache,
		resolver:        resolver,
		authorizer:      authorizer,
		apiBaseURL:      strings.TrimRight(apiBaseURL, "/"),
		fileStorageRoot: fileStorageRoot,
		templateClient:  templateClient,
	}
}

func (h *RoomWelcomeHandler) Index(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, user, _, ok := h.resolveAccess(w, r)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	filter := RoomWelcomeFilter{
		CorpID:  corpID,
		Text:    strings.TrimSpace(r.URL.Query().Get("text")),
		Page:    positiveQueryInt(r, "page", 1),
		PerPage: positiveQueryInt(r, "perPage", 10),
	}
	if user.IsSuperAdmin == 0 {
		filter.RestrictCreateUserID = true
		filter.CreateUserID = user.ID
	}
	page, err := h.store.RoomWelcomePage(r.Context(), filter)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	creatorIDs := make([]int, 0, len(page.Items))
	for _, item := range page.Items {
		creatorIDs = append(creatorIDs, item.CreateUserID)
	}
	creators, err := h.store.RoomWelcomeCreatorNamesByIDs(r.Context(), uniquePositiveIntsLocal(creatorIDs))
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, h.roomWelcomeListPayload(item, creators))
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"page": map[string]any{
			"perPage":   page.PerPage,
			"total":     page.Total,
			"totalPage": page.TotalPage,
		},
		"list": list,
	})
}

func (h *RoomWelcomeHandler) Show(w http.ResponseWriter, r *http.Request) {
	h.showOrSelect(w, r, "/dashboard/roomWelcome/show#get")
}

func (h *RoomWelcomeHandler) Select(w http.ResponseWriter, r *http.Request) {
	h.showOrSelect(w, r, "/dashboard/roomWelcome/select#get")
}

func (h *RoomWelcomeHandler) Store(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, user, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/roomWelcome/store#post")
	if !ok {
		return
	}
	_ = userID
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	if h.templateClient == nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "企业微信模板客户端未配置", nil)
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	credential, ok := h.resolveCorpCredential(w, r.Context(), corpID)
	if !ok {
		return
	}
	write, payload, ok := h.roomWelcomeWriteFromParams(w, r.Context(), params, credential, corpID, user.ID)
	if !ok {
		return
	}
	templateID, err := h.templateClient.CreateGroupWelcomeTemplate(r.Context(), credential, payload)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "添加入群欢迎语素材失败"+err.Error(), nil)
		return
	}
	write.ComplexTemplateID = templateID
	if _, err := h.store.CreateRoomWelcome(r.Context(), write); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "欢迎语创建失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *RoomWelcomeHandler) Update(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, user, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/roomWelcome/update#put")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	if h.templateClient == nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "企业微信模板客户端未配置", nil)
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	roomWelcomeID, okInt, err := intParam(params, "id")
	if err != nil || !okInt || roomWelcomeID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "入群欢迎语ID 必填", nil)
		return
	}
	item, found, err := h.store.RoomWelcomeByID(r.Context(), roomWelcomeID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found || item.CorpID != corpID {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "当前入群欢迎语信息不存在", nil)
		return
	}
	credential, ok := h.resolveCorpCredential(w, r.Context(), corpID)
	if !ok {
		return
	}
	write, payload, ok := h.roomWelcomeWriteFromParams(w, r.Context(), params, credential, corpID, user.ID)
	if !ok {
		return
	}
	if err := h.templateClient.UpdateGroupWelcomeTemplate(r.Context(), credential, item.ComplexTemplateID, payload); err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "修改入群欢迎语素材失败"+err.Error(), nil)
		return
	}
	updated, err := h.store.UpdateRoomWelcome(r.Context(), roomWelcomeID, write)
	if err != nil || !updated {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "欢迎语创建失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *RoomWelcomeHandler) Destroy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, _, ok := h.resolveAuthorized(w, r, "/dashboard/roomWelcome/destroy#delete")
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	if h.templateClient == nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "企业微信模板客户端未配置", nil)
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	roomWelcomeID, okInt, err := intParam(params, "id")
	if err != nil || !okInt || roomWelcomeID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "欢迎语id 必填", nil)
		return
	}
	item, found, err := h.store.RoomWelcomeByID(r.Context(), roomWelcomeID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found || item.CorpID != corpID {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "当前入群欢迎语信息不存在", nil)
		return
	}
	credential, ok := h.resolveCorpCredential(w, r.Context(), corpID)
	if !ok {
		return
	}
	if item.ComplexTemplateID != "" {
		if err := h.templateClient.DeleteGroupWelcomeTemplate(r.Context(), credential, item.ComplexTemplateID); err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "删除入群欢迎语素材失败"+err.Error(), nil)
			return
		}
	}
	deleted, err := h.store.DeleteRoomWelcome(r.Context(), roomWelcomeID)
	if err != nil || !deleted {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "欢迎语创建失败", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *RoomWelcomeHandler) showOrSelect(w http.ResponseWriter, r *http.Request, permissionKey string) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	_, _, _, _, ok := h.resolveAuthorized(w, r, permissionKey)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	roomWelcomeID, err := positiveQueryIntRequired(r, "id")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "入群欢迎语ID 必填", nil)
		return
	}
	item, found, err := h.store.RoomWelcomeByID(r.Context(), roomWelcomeID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found || item.CorpID != corpID {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "当前入群欢迎语信息不存在", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", h.roomWelcomeDetailPayload(item))
}

func (h *RoomWelcomeHandler) roomWelcomeWriteFromParams(
	w http.ResponseWriter,
	ctx context.Context,
	params map[string]any,
	credential RoomWelcomeCorpCredential,
	corpID int,
	userID int,
) (RoomWelcomeWrite, RoomWelcomeTemplatePayload, bool) {
	notice, okInt, err := intParam(params, "notice")
	if err != nil || !okInt {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "是否消息提醒 必填", nil)
		return RoomWelcomeWrite{}, RoomWelcomeTemplatePayload{}, false
	}
	if notice != 0 && notice != 1 {
		notice = 1
	}
	msgText := stringParam(params, "msg_text")
	complex, ok := roomWelcomeComplexParam(params)
	if !ok {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "欢迎语2 数据错误", nil)
		return RoomWelcomeWrite{}, RoomWelcomeTemplatePayload{}, false
	}
	complexType, msgComplex, payload, ok := h.prepareRoomWelcomeTemplate(w, ctx, credential, msgText, notice, complex)
	if !ok {
		return RoomWelcomeWrite{}, RoomWelcomeTemplatePayload{}, false
	}
	if strings.TrimSpace(msgText) == "" && payload.ImagePicURL == "" && payload.Link == nil && payload.MiniProgram == nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "欢迎语内容必填", nil)
		return RoomWelcomeWrite{}, RoomWelcomeTemplatePayload{}, false
	}
	rawMsgComplex, err := json.Marshal(msgComplex)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "欢迎语2 数据错误", nil)
		return RoomWelcomeWrite{}, RoomWelcomeTemplatePayload{}, false
	}
	return RoomWelcomeWrite{
		CorpID:       corpID,
		MsgText:      msgText,
		ComplexType:  complexType,
		MsgComplex:   string(rawMsgComplex),
		CreateUserID: userID,
	}, payload, true
}

func (h *RoomWelcomeHandler) prepareRoomWelcomeTemplate(
	w http.ResponseWriter,
	ctx context.Context,
	credential RoomWelcomeCorpCredential,
	msgText string,
	notice int,
	complex map[string]any,
) (string, map[string]any, RoomWelcomeTemplatePayload, bool) {
	templateText := strings.ReplaceAll(msgText, "[用户昵称]", "%NICKNAME%")
	payload := RoomWelcomeTemplatePayload{Notify: notice}
	if strings.TrimSpace(msgText) != "" {
		payload.TextContent = templateText
	}

	complexType := strings.TrimSpace(toString(complex["type"]))
	if complexType == "" {
		return "", map[string]any{}, payload, true
	}

	switch complexType {
	case "image":
		image := cloneMap(roomWelcomeNestedMap(complex, "image"))
		pic := strings.TrimSpace(toString(image["pic"]))
		if pic != "" {
			localPic, remotePic, ok := h.prepareRoomWelcomeImage(w, ctx, credential, pic, false)
			if !ok {
				return "", nil, RoomWelcomeTemplatePayload{}, false
			}
			image["pic"] = localPic
			image["pic_url"] = remotePic
			payload.ImagePicURL = remotePic
		}
		return complexType, image, payload, true
	case "link":
		link := cloneMap(roomWelcomeNestedMap(complex, "link"))
		if strings.TrimSpace(toString(link["url"])) != "" {
			picURL := ""
			pic := strings.TrimSpace(toString(link["pic"]))
			if pic != "" {
				localPic, remotePic, ok := h.prepareRoomWelcomeImage(w, ctx, credential, pic, false)
				if !ok {
					return "", nil, RoomWelcomeTemplatePayload{}, false
				}
				link["pic"] = localPic
				link["pic_url"] = remotePic
				picURL = remotePic
			}
			payload.Link = &RoomWelcomeTemplateLink{
				Title:  toString(link["title"]),
				PicURL: picURL,
				Desc:   toString(link["desc"]),
				URL:    toString(link["url"]),
			}
		}
		return complexType, link, payload, true
	case "miniprogram":
		miniProgram := cloneMap(roomWelcomeNestedMap(complex, "miniprogram"))
		if strings.TrimSpace(toString(miniProgram["title"])) != "" {
			mediaID := ""
			pic := strings.TrimSpace(toString(miniProgram["pic"]))
			if pic != "" {
				localPic, remoteMediaID, ok := h.prepareRoomWelcomeImage(w, ctx, credential, pic, true)
				if !ok {
					return "", nil, RoomWelcomeTemplatePayload{}, false
				}
				miniProgram["pic"] = localPic
				miniProgram["pic_url"] = remoteMediaID
				miniProgram["pic_media_id"] = remoteMediaID
				mediaID = remoteMediaID
			}
			payload.MiniProgram = &RoomWelcomeTemplateMiniProgram{
				Title:      toString(miniProgram["title"]),
				PicMediaID: mediaID,
				AppID:      firstNonEmptyString(miniProgram["appid"], miniProgram["appId"]),
				Page:       toString(miniProgram["page"]),
			}
		}
		return complexType, miniProgram, payload, true
	default:
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "欢迎语2 类型错误", nil)
		return "", nil, RoomWelcomeTemplatePayload{}, false
	}
}

func (h *RoomWelcomeHandler) prepareRoomWelcomeImage(
	w http.ResponseWriter,
	ctx context.Context,
	credential RoomWelcomeCorpCredential,
	raw string,
	temporary bool,
) (string, string, bool) {
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw, raw, true
	}
	localPath := raw
	filePath := raw
	if strings.HasPrefix(raw, "data:") || looksLikeBase64(raw) {
		relative, absolute, err := h.saveRoomWelcomeBase64Image(raw)
		if err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "图片处理失败", nil)
			return "", "", false
		}
		localPath = relative
		filePath = absolute
	} else if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(h.fileStorageRoot, strings.TrimLeft(raw, "/"))
	}

	var (
		remote string
		err    error
	)
	if temporary {
		remote, err = h.templateClient.UploadTemporaryImage(ctx, credential, filePath)
	} else {
		remote, err = h.templateClient.UploadImage(ctx, credential, filePath)
	}
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "上传图片失败"+err.Error(), nil)
		return "", "", false
	}
	return localPath, remote, true
}

func (h *RoomWelcomeHandler) saveRoomWelcomeBase64Image(raw string) (string, string, error) {
	payload := raw
	if index := strings.Index(payload, ","); strings.HasPrefix(payload, "data:") && index >= 0 {
		payload = payload[index+1:]
	}
	payload = strings.TrimSpace(payload)
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(payload)
	}
	if err != nil {
		return "", "", err
	}
	if len(decoded) <= 5 {
		return "", "", fmt.Errorf("image too small")
	}
	name := time.Now().Format("20060102150405") + "_" + randomHexString(8) + ".jpg"
	relative := filepath.ToSlash(filepath.Join("image", "roomWelcome", name))
	absolute := filepath.Join(h.fileStorageRoot, relative)
	if err := os.MkdirAll(filepath.Dir(absolute), 0755); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(absolute, decoded, 0644); err != nil {
		return "", "", err
	}
	return relative, absolute, nil
}

func (h *RoomWelcomeHandler) roomWelcomeListPayload(item RoomWelcomeItem, creators map[int]string) map[string]any {
	return map[string]any{
		"id":           item.ID,
		"msg_text":     item.MsgText,
		"complex_type": item.ComplexType,
		"msg_complex":  h.roomWelcomeMsgComplex(item.MsgComplex),
		"create_user":  creators[item.CreateUserID],
		"create_time":  item.CreatedAt,
	}
}

func (h *RoomWelcomeHandler) roomWelcomeDetailPayload(item RoomWelcomeItem) map[string]any {
	return map[string]any{
		"id":                item.ID,
		"corpId":            item.CorpID,
		"msgText":           item.MsgText,
		"complexType":       item.ComplexType,
		"msgComplex":        h.roomWelcomeMsgComplex(item.MsgComplex),
		"complexTemplateId": item.ComplexTemplateID,
		"createUserId":      item.CreateUserID,
		"createdAt":         item.CreatedAt,
		"updatedAt":         item.UpdatedAt,
	}
}

func (h *RoomWelcomeHandler) roomWelcomeMsgComplex(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "null"
	}
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return raw
	}
	if object, ok := value.(map[string]any); ok {
		if pic := strings.TrimSpace(toString(object["pic"])); pic != "" {
			object["pic"] = h.fileFullURL(pic)
		}
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return raw
	}
	return string(encoded)
}

func (h *RoomWelcomeHandler) resolveAuthorized(w http.ResponseWriter, r *http.Request, permissionKey string) (int, User, DashboardRequestScope, AccessContext, bool) {
	userID, user, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return 0, User{}, DashboardRequestScope{}, AccessContext{}, false
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return 0, User{}, DashboardRequestScope{}, AccessContext{}, false
	}
	access := AccessContext{User: user, CorpID: corpID, WorkEmployeeID: principalScope.WorkEmployeeID, DataPermission: DataPermissionAll}
	if h.authorizer != nil {
		var err error
		access, err = h.authorizer.Resolve(r.Context(), userID, permissionKey, corpID, principalScope.WorkEmployeeID)
		if err != nil {
			writeAccessError(w, err)
			return 0, User{}, DashboardRequestScope{}, AccessContext{}, false
		}
	}
	return userID, user, principalScope, access, true
}

func (h *RoomWelcomeHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (int, User, DashboardRequestScope, bool) {
	requestPrincipal, err := DashboardPrincipalFromContext(r.Context())
	userID := requestPrincipal.UserID
	if err != nil || userID <= 0 {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "unauthorized", nil)
		return 0, User{}, DashboardRequestScope{}, false
	}
	user, found, err := h.store.UserByID(r.Context(), userID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return 0, User{}, DashboardRequestScope{}, false
	}
	if !found {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "user not found", nil)
		return 0, User{}, DashboardRequestScope{}, false
	}
	principalScope, err := DashboardRequestScopeFromContext(r.Context())
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return 0, User{}, DashboardRequestScope{}, false
	}
	return userID, user, DashboardRequestScope(principalScope), true
}

func (h *RoomWelcomeHandler) resolveCorpCredential(w http.ResponseWriter, ctx context.Context, corpID int) (RoomWelcomeCorpCredential, bool) {
	credential, found, err := h.store.RoomWelcomeCorpCredentialByID(ctx, corpID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return RoomWelcomeCorpCredential{}, false
	}
	if !found || strings.TrimSpace(credential.WXCorpID) == "" || strings.TrimSpace(credential.ContactSecret) == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "企业微信配置不存在", nil)
		return RoomWelcomeCorpCredential{}, false
	}
	return credential, true
}

func (h *RoomWelcomeHandler) fileFullURL(path string) string {
	if path == "" || strings.Contains(path, "http") || h.apiBaseURL == "" {
		return path
	}
	return h.apiBaseURL + "/static/" + strings.TrimLeft(path, "/")
}

func roomWelcomeComplexParam(params map[string]any) (map[string]any, bool) {
	value, exists := params["msg_complex"]
	if !exists || value == nil || strings.TrimSpace(fmt.Sprint(value)) == "" {
		return map[string]any{}, true
	}
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case map[string]string:
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			out[key] = value
		}
		return out, true
	case string:
		var parsed map[string]any
		if err := json.Unmarshal([]byte(typed), &parsed); err != nil {
			return nil, false
		}
		return parsed, true
	default:
		return nil, false
	}
}

func roomWelcomeNestedMap(values map[string]any, key string) map[string]any {
	value, ok := values[key]
	if !ok || value == nil {
		return map[string]any{}
	}
	switch typed := value.(type) {
	case map[string]any:
		return typed
	case map[string]string:
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			out[key] = value
		}
		return out
	default:
		return map[string]any{}
	}
}

func firstNonEmptyString(values ...any) string {
	for _, value := range values {
		if text := strings.TrimSpace(toString(value)); text != "" {
			return text
		}
	}
	return ""
}

func looksLikeBase64(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) < 16 || strings.Contains(value, "/") || strings.Contains(value, "\\") || strings.Contains(value, ".") {
		return false
	}
	for _, char := range value {
		if (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '+' || char == '/' || char == '=' {
			continue
		}
		return false
	}
	return true
}

func randomHexString(length int) string {
	buffer := make([]byte, length)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buffer)
}
