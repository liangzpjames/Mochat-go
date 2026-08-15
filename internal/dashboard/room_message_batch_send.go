package dashboard

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboardprincipal"
)

type RoomMessageBatchSendFilter struct {
	UserID              int
	BatchTitle          string
	Page                int
	PerPage             int
	AllowedEmployeeIDs  []int
	RestrictEmployeeIDs bool
}

type RoomMessageBatchSendPage struct {
	Items     []RoomMessageBatchSendItem
	Total     int
	TotalPage int
	PerPage   int
}

type RoomMessageBatchSendItem struct {
	ID                int
	CorpID            int
	UserID            int
	MediumID          int
	UserName          string
	EmployeeIDs       []int
	BatchTitle        string
	Content           []ContactMessageBatchSendContent
	SendWay           int
	DefiniteTime      string
	SendTime          string
	SendRoomTotal     int
	SendEmployeeTotal int
	SendTotal         int
	NotSendTotal      int
	ReceivedTotal     int
	NotReceivedTotal  int
	SendStatus        int
	CreatedAt         string
}

type RoomMessageBatchSendWrite struct {
	CorpID             int
	UserID             int
	MediumID           int
	UserName           string
	EmployeeIDs        []int
	BatchTitle         string
	Content            []ContactMessageBatchSendContent
	ContentJSON        string
	SendWay            int
	DefiniteTime       string
	ProcessImmediately bool
}

type RoomMessageBatchSendTarget struct {
	EmployeeID int
	WXUserID   string
	ChatIDs    []string
}

type RoomMessageBatchSendMessagePayload struct {
	Content []ContactMessageBatchSendContent
	ChatIDs []string
	Sender  string
}

type RoomMessageBatchSendMessageResult struct {
	EmployeeID int
	ErrCode    int
	ErrMsg     string
	MsgID      string
}

type RoomMessageBatchSendOwnerItem struct {
	ID                  int
	Status              int
	SendTime            string
	SendRoomTotal       int
	SendSuccessTotal    int
	EmployeeID          int
	EmployeeName        string
	EmployeeAlias       string
	EmployeeAvatar      string
	EmployeeThumbAvatar string
}

type RoomMessageBatchSendOwnerPage struct {
	Items     []RoomMessageBatchSendOwnerItem
	Total     int
	TotalPage int
	PerPage   int
}

type RoomMessageBatchSendRoomItem struct {
	ID              int
	Status          int
	SendTime        int
	RoomID          int
	RoomName        string
	RoomCreateTime  string
	RoomEmployeeNum int
	EmployeeID      int
	EmployeeName    string
	EmployeeAlias   string
}

type RoomMessageBatchSendRoomPage struct {
	Items     []RoomMessageBatchSendRoomItem
	Total     int
	TotalPage int
	PerPage   int
}

type RoomMessageBatchSendOwnerFilter struct {
	BatchID    int
	SendStatus *int
	Page       int
	PerPage    int
}

type RoomMessageBatchSendRoomFilter struct {
	BatchID    int
	SendStatus *int
	KeyWords   string
	Page       int
	PerPage    int
}

type RoomMessageBatchSendEmployeeRef struct {
	ID       int
	WXUserID string
}

type RoomMessageBatchSendStore interface {
	UserByID(ctx context.Context, userID int) (User, bool, error)
	EmployeeIDByUserCorp(ctx context.Context, userID int, corpID int) (int, error)
	FirstEmployeeByUser(ctx context.Context, userID int) (corpID int, employeeID int, ok bool, err error)
	RoomWelcomeCorpCredentialByID(ctx context.Context, corpID int) (RoomWelcomeCorpCredential, bool, error)
	RoomTagPullRemindAgentByCorpID(ctx context.Context, corpID int) (RoomTagPullAgentCredential, bool, error)
	RoomMessageBatchSendPage(ctx context.Context, filter RoomMessageBatchSendFilter) (RoomMessageBatchSendPage, error)
	RoomMessageBatchSendByID(ctx context.Context, batchID int) (RoomMessageBatchSendItem, bool, error)
	RoomMessageBatchSendSeedRooms(ctx context.Context, batchID int, limit int) ([]ContactMessageBatchSendNameID, error)
	RoomMessageBatchSendValidateEmployees(ctx context.Context, corpID int, employeeIDs []int) (bool, error)
	CreateRoomMessageBatchSend(ctx context.Context, values RoomMessageBatchSendWrite) (int, error)
	CreateRoomMessageBatchSendTasks(ctx context.Context, batchID int) ([]RoomMessageBatchSendTarget, error)
	MarkRoomMessageBatchSendSubmitted(ctx context.Context, batchID int, results []RoomMessageBatchSendMessageResult) error
	RoomMessageBatchSendOwnerPage(ctx context.Context, filter RoomMessageBatchSendOwnerFilter) (RoomMessageBatchSendOwnerPage, error)
	RoomMessageBatchSendRoomPage(ctx context.Context, filter RoomMessageBatchSendRoomFilter) (RoomMessageBatchSendRoomPage, error)
	DeleteRoomMessageBatchSend(ctx context.Context, batchID int) (bool, error)
	RoomMessageBatchSendEmployees(ctx context.Context, batchID int) ([]RoomMessageBatchSendEmployeeRef, error)
	RoomMessageBatchSendEmployeeWXUserID(ctx context.Context, employeeID int) (string, bool, error)
}

type RoomMessageBatchSendClient interface {
	UploadTemporaryImage(ctx context.Context, credential RoomWelcomeCorpCredential, filePath string) (string, error)
	SubmitRoomMessageBatchSend(ctx context.Context, credential RoomWelcomeCorpCredential, payload RoomMessageBatchSendMessagePayload) (RoomMessageBatchSendMessageResult, error)
	SendAgentTextMessage(ctx context.Context, credential RoomTagPullAgentCredential, toUser string, content string) error
}

type RoomMessageBatchSendHandler struct {
	store           RoomMessageBatchSendStore
	durableStore    RoomBatchDispatchStore
	cache           LoginCache
	resolver        UserIDResolver
	authorizer      CorpAdminAuthorizer
	apiBaseURL      string
	fileStorageRoot string
	client          RoomMessageBatchSendClient
	durableRequired bool
}

func NewRoomMessageBatchSendHandler(store RoomMessageBatchSendStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer, apiBaseURL string, fileStorageRoot string, client RoomMessageBatchSendClient) *RoomMessageBatchSendHandler {
	if strings.TrimSpace(fileStorageRoot) == "" {
		fileStorageRoot = defaultRoomTagPullFileStorageRoot
	}
	if client == nil {
		client = NewRoomWelcomeWeComClient(defaultWeComAPIBaseURL)
	}
	return &RoomMessageBatchSendHandler{
		store:           store,
		durableStore:    roomBatchDispatchStoreFrom(store),
		cache:           cache,
		resolver:        resolver,
		authorizer:      authorizer,
		apiBaseURL:      strings.TrimRight(apiBaseURL, "/"),
		fileStorageRoot: fileStorageRoot,
		client:          client,
	}
}

func (h *RoomMessageBatchSendHandler) Index(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, _, ok := h.resolveAuthorized(w, r)
	if !ok {
		return
	}
	dashboardAccess, hasDashboardAccess := DashboardAccessFromContext(r.Context())
	page, err := h.store.RoomMessageBatchSendPage(r.Context(), RoomMessageBatchSendFilter{
		UserID:     userID,
		BatchTitle: strings.TrimSpace(r.URL.Query().Get("batchTitle")),
		Page:       positiveQueryInt(r, "page", 1),
		PerPage:    positiveQueryInt(r, "perPage", 10),
		AllowedEmployeeIDs: func() []int {
			if hasDashboardAccess {
				return append([]int(nil), dashboardAccess.AllowedEmployeeIDs...)
			}
			return nil
		}(),
		RestrictEmployeeIDs: hasDashboardAccess && dashboardAccess.ScopeRequired && dashboardAccess.Scope != DataScopeTenant,
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	var durable RoomBatchDispatchDurableReadStore
	var durablePrincipal dashboardprincipal.DashboardPrincipal
	if candidate, ok := h.durableStore.(RoomBatchDispatchDurableReadStore); ok {
		principal, err := DashboardPrincipalFromContext(r.Context())
		if err != nil {
			writeMachineEnvelope(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized", nil)
			return
		}
		durable = candidate
		durablePrincipal = principal
	}
	for _, item := range page.Items {
		payload := h.batchListPayload(item)
		if durable != nil {
			view, found, viewErr := durable.RoomBatchDurableView(r.Context(), durablePrincipal, item.ID)
			if viewErr != nil {
				writeRoomBatchDurableError(w, viewErr)
				return
			}
			if found {
				payload = h.batchListPayload(view.Batch)
				payload["operation"] = roomBatchOperationPayload(view.Operation)
			}
		}
		list = append(list, payload)
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

func (h *RoomMessageBatchSendHandler) Show(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, _, ok := h.resolveAuthorized(w, r)
	if !ok {
		return
	}
	batchID, ok := queryPositiveIntParam(w, r, "batchId", "batchId 必填", "batchId 必须为整数")
	if !ok {
		return
	}
	if durable, available := h.durableStore.(RoomBatchDispatchDurableReadStore); available {
		principal, err := DashboardPrincipalFromContext(r.Context())
		if err != nil {
			writeMachineEnvelope(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized", nil)
			return
		}
		view, found, err := durable.RoomBatchDurableView(r.Context(), principal, batchID)
		if err != nil {
			writeRoomBatchDurableError(w, err)
			return
		}
		if found {
			payload := h.batchShowPayload(view.Batch)
			payload["operation"] = roomBatchOperationPayload(view.Operation)
			if seedRooms, seedErr := h.store.RoomMessageBatchSendSeedRooms(r.Context(), batchID, 10); seedErr == nil {
				payload["seedRooms"] = seedRooms
			}
			writeEnvelope(w, http.StatusOK, 200, "success", payload)
			return
		}
	}
	batch, ok := h.loadOwnedBatch(w, r, userID, batchID)
	if !ok {
		return
	}
	seedRooms, err := h.store.RoomMessageBatchSendSeedRooms(r.Context(), batchID, 10)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	payload := h.batchShowPayload(batch)
	payload["seedRooms"] = seedRooms
	writeEnvelope(w, http.StatusOK, 200, "success", payload)
}

func (h *RoomMessageBatchSendHandler) Store(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, user, _, ok := h.resolveAuthorized(w, r)
	if !ok {
		return
	}
	corpID, ok := principalCorpID(w, r)
	if !ok {
		return
	}
	if h.durableRequired && h.durableStore == nil {
		writeMachineEnvelope(w, http.StatusServiceUnavailable, "ROOM_BATCH_DURABLE_UNAVAILABLE", "durable room batch provider unavailable", nil)
		return
	}
	if h.durableStore != nil {
		principal, err := DashboardPrincipalFromContext(r.Context())
		if err != nil {
			writeMachineEnvelope(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized", nil)
			return
		}
		access, _ := DashboardAccessFromContext(r.Context())
		if h.storeDurableRoomBatch(w, r, user, principal, access, corpID) {
			return
		}
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	write, ok := h.batchWriteFromParams(w, r.Context(), params, corpID, userID, user)
	if !ok {
		return
	}
	if access, scoped := DashboardAccessFromContext(r.Context()); scoped && access.ScopeRequired && access.Scope != DataScopeTenant && !employeeIDsWithinDashboardScope(write.EmployeeIDs, access.AllowedEmployeeIDs) {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, "employee scope denied", nil)
		return
	}
	if !enforceSaaSQuota(r.Context(), w, h.store, user.TenantID, SaaSMetricRoomMessageBatches, 1) {
		return
	}
	batchID, err := h.store.CreateRoomMessageBatchSend(r.Context(), write)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "客户群消息群发创建失败", nil)
		return
	}
	if err := refreshSaaSUsageCounter(r.Context(), h.store, user.TenantID, SaaSMetricRoomMessageBatches); err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if write.ProcessImmediately {
		if ok := h.sendBatchNow(w, r.Context(), corpID, batchID); !ok {
			return
		}
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *RoomMessageBatchSendHandler) RoomOwnerSendIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, _, ok := h.resolveAuthorized(w, r)
	if !ok {
		return
	}
	batchID, ok := queryPositiveIntParam(w, r, "batchId", "batchId 必填", "batchId 必须为整数")
	if !ok {
		return
	}
	if durable, available := h.durableStore.(RoomBatchDispatchDurableReadStore); available {
		sendStatus, valid := optionalQueryInt(w, r, "sendStatus", "sendStatus 必须为整数")
		if !valid {
			return
		}
		principal, err := DashboardPrincipalFromContext(r.Context())
		if err != nil {
			writeMachineEnvelope(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized", nil)
			return
		}
		filter := RoomMessageBatchSendOwnerFilter{
			BatchID: batchID, SendStatus: sendStatus,
			Page: positiveQueryInt(r, "page", 1), PerPage: positiveQueryInt(r, "perPage", 15),
		}
		page, err := durable.RoomBatchDurableOwnerPage(r.Context(), principal, batchID, filter)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			writeRoomBatchDurableError(w, err)
			return
		}
		if err == nil {
			list := make([]map[string]any, 0, len(page.Items))
			for _, item := range page.Items {
				list = append(list, map[string]any{
					"id":                  item.ID,
					"status":              item.Status,
					"sendTime":            item.SendTime,
					"sendRoomTotal":       item.SendRoomTotal,
					"sendSuccessTotal":    item.SendSuccessTotal,
					"employeeId":          item.EmployeeID,
					"employeeName":        item.EmployeeName,
					"employeeAlias":       item.EmployeeAlias,
					"employeeAvatar":      h.fullStaticURL(item.EmployeeAvatar),
					"employeeThumbAvatar": h.fullStaticURL(item.EmployeeThumbAvatar),
				})
			}
			writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
				"page": map[string]any{"perPage": page.PerPage, "total": page.Total, "totalPage": page.TotalPage},
				"list": list,
			})
			return
		}
	}
	if _, ok = h.loadOwnedBatch(w, r, userID, batchID, true); !ok {
		return
	}
	sendStatus, ok := optionalQueryInt(w, r, "sendStatus", "sendStatus 必须为整数")
	if !ok {
		return
	}
	page, err := h.store.RoomMessageBatchSendOwnerPage(r.Context(), RoomMessageBatchSendOwnerFilter{
		BatchID:    batchID,
		SendStatus: sendStatus,
		Page:       positiveQueryInt(r, "page", 1),
		PerPage:    positiveQueryInt(r, "perPage", 15),
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, map[string]any{
			"id":                  item.ID,
			"status":              item.Status,
			"sendTime":            item.SendTime,
			"sendRoomTotal":       item.SendRoomTotal,
			"sendSuccessTotal":    item.SendSuccessTotal,
			"employeeId":          item.EmployeeID,
			"employeeName":        item.EmployeeName,
			"employeeAlias":       item.EmployeeAlias,
			"employeeAvatar":      h.fullStaticURL(item.EmployeeAvatar),
			"employeeThumbAvatar": h.fullStaticURL(item.EmployeeThumbAvatar),
		})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"page": map[string]any{"perPage": page.PerPage, "total": page.Total, "totalPage": page.TotalPage},
		"list": list,
	})
}

func (h *RoomMessageBatchSendHandler) RoomReceiveIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, _, ok := h.resolveAuthorized(w, r)
	if !ok {
		return
	}
	batchID, ok := queryPositiveIntParam(w, r, "batchId", "batchId 必填", "batchId 必须为整数")
	if !ok {
		return
	}
	if durable, available := h.durableStore.(RoomBatchDispatchDurableReadStore); available {
		sendStatus, valid := optionalQueryInt(w, r, "sendStatus", "sendStatus 必须为整数")
		if !valid {
			return
		}
		principal, err := DashboardPrincipalFromContext(r.Context())
		if err != nil {
			writeMachineEnvelope(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized", nil)
			return
		}
		filter := RoomMessageBatchSendRoomFilter{
			BatchID: batchID, SendStatus: sendStatus, KeyWords: strings.TrimSpace(r.URL.Query().Get("keyWords")),
			Page: positiveQueryInt(r, "page", 1), PerPage: positiveQueryInt(r, "perPage", 15),
		}
		page, err := durable.RoomBatchDurableReceivePage(r.Context(), principal, batchID, filter)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			writeRoomBatchDurableError(w, err)
			return
		}
		if err == nil {
			list := make([]map[string]any, 0, len(page.Items))
			for _, item := range page.Items {
				list = append(list, map[string]any{
					"id":              item.ID,
					"status":          item.Status,
					"sendTime":        formatUnixSecondsLocal(item.SendTime),
					"roomId":          item.RoomID,
					"roomName":        item.RoomName,
					"roomCreateTime":  item.RoomCreateTime,
					"roomEmployeeNum": item.RoomEmployeeNum,
					"employeeId":      item.EmployeeID,
					"employeeName":    item.EmployeeName,
					"employeeAlias":   item.EmployeeAlias,
				})
			}
			writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
				"page": map[string]any{"perPage": page.PerPage, "total": page.Total, "totalPage": page.TotalPage},
				"list": list,
			})
			return
		}
	}
	if _, ok = h.loadOwnedBatch(w, r, userID, batchID, true); !ok {
		return
	}
	sendStatus, ok := optionalQueryInt(w, r, "sendStatus", "sendStatus 必须为整数")
	if !ok {
		return
	}
	page, err := h.store.RoomMessageBatchSendRoomPage(r.Context(), RoomMessageBatchSendRoomFilter{
		BatchID:    batchID,
		SendStatus: sendStatus,
		KeyWords:   strings.TrimSpace(r.URL.Query().Get("keyWords")),
		Page:       positiveQueryInt(r, "page", 1),
		PerPage:    positiveQueryInt(r, "perPage", 15),
	})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	list := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		list = append(list, map[string]any{
			"id":              item.ID,
			"status":          item.Status,
			"sendTime":        formatUnixSecondsLocal(item.SendTime),
			"roomId":          item.RoomID,
			"roomName":        item.RoomName,
			"roomCreateTime":  item.RoomCreateTime,
			"roomEmployeeNum": item.RoomEmployeeNum,
			"employeeId":      item.EmployeeID,
			"employeeName":    item.EmployeeName,
			"employeeAlias":   item.EmployeeAlias,
		})
	}
	writeEnvelope(w, http.StatusOK, 200, "success", map[string]any{
		"page": map[string]any{"perPage": page.PerPage, "total": page.Total, "totalPage": page.TotalPage},
		"list": list,
	})
}

func (h *RoomMessageBatchSendHandler) Remind(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, _, ok := h.resolveAuthorized(w, r)
	if !ok {
		return
	}
	batchID, ok := queryPositiveIntParam(w, r, "batchId", "batchId 必填", "batchId 必须为整数")
	if !ok {
		return
	}
	batch, ok := h.loadOwnedBatch(w, r, userID, batchID, true)
	if !ok {
		return
	}
	agent, found, err := h.store.RoomTagPullRemindAgentByCorpID(r.Context(), batch.CorpID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "无可用的 agent", nil)
		return
	}
	employees := []RoomMessageBatchSendEmployeeRef{}
	batchEmployeeID, ok := optionalQueryInt(w, r, "batchEmployId", "batchEmployId 必须为整数")
	if !ok {
		return
	}
	if batchEmployeeID != nil && *batchEmployeeID > 0 {
		wxUserID, found, err := h.store.RoomMessageBatchSendEmployeeWXUserID(r.Context(), *batchEmployeeID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		if found {
			employees = append(employees, RoomMessageBatchSendEmployeeRef{ID: *batchEmployeeID, WXUserID: wxUserID})
		}
	} else {
		employees, err = h.store.RoomMessageBatchSendEmployees(r.Context(), batchID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
	}
	text := roomMessageBatchSendReminderText(batch.CreatedAt)
	for _, employee := range employees {
		if employee.WXUserID == "" {
			continue
		}
		if err := h.client.SendAgentTextMessage(r.Context(), agent, employee.WXUserID, text); err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "发送提醒失败", nil)
			return
		}
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *RoomMessageBatchSendHandler) Destroy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	userID, _, _, ok := h.resolveAuthorized(w, r)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	batchID, has, err := intParam(params, "batchId")
	if err != nil || !has || batchID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "batchId 必填", nil)
		return
	}
	if _, ok = h.loadOwnedBatch(w, r, userID, batchID, true); !ok {
		return
	}
	deleted, err := h.store.DeleteRoomMessageBatchSend(r.Context(), batchID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, "客户群消息删除失败", nil)
		return
	}
	if !deleted {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "未找到记录", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, 200, "success", []any{})
}

func (h *RoomMessageBatchSendHandler) resolveAuthorized(w http.ResponseWriter, r *http.Request) (int, User, DashboardRequestScope, bool) {
	userID, user, principalScope, ok := h.resolveAccess(w, r)
	if !ok {
		return 0, User{}, DashboardRequestScope{}, false
	}
	if h.authorizer != nil {
		corpID := 0
		if len(principalScope.CorpIDs) > 0 {
			corpID = principalScope.CorpIDs[0]
		}
		if _, err := h.authorizer.Resolve(r.Context(), userID, PermissionKeyFromRequest(r), corpID, principalScope.WorkEmployeeID); err != nil {
			writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, err.Error(), nil)
			return 0, User{}, DashboardRequestScope{}, false
		}
	}
	return userID, user, principalScope, true
}

func (h *RoomMessageBatchSendHandler) resolveAccess(w http.ResponseWriter, r *http.Request) (int, User, DashboardRequestScope, bool) {
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
	return userID, user, principalScope, true
}

func (h *RoomMessageBatchSendHandler) loadOwnedBatch(w http.ResponseWriter, r *http.Request, userID int, batchID int, prechecked ...bool) (RoomMessageBatchSendItem, bool) {
	if len(prechecked) == 0 || !prechecked[0] {
		if batchID <= 0 {
			return RoomMessageBatchSendItem{}, false
		}
	}
	var batch RoomMessageBatchSendItem
	var found bool
	var err error
	if principal, principalErr := DashboardPrincipalFromContext(r.Context()); principalErr == nil {
		if scoped, scopedOK := h.store.(RoomBatchDispatchScopedReadStore); scopedOK {
			batch, found, err = scoped.RoomMessageBatchSendByIDForPrincipal(r.Context(), principal, batchID)
		} else if h.durableRequired {
			writeMachineEnvelope(w, http.StatusServiceUnavailable, "ROOM_BATCH_DURABLE_UNAVAILABLE", "durable room batch provider unavailable", nil)
			return RoomMessageBatchSendItem{}, false
		} else {
			batch, found, err = h.store.RoomMessageBatchSendByID(r.Context(), batchID)
		}
	} else {
		batch, found, err = h.store.RoomMessageBatchSendByID(r.Context(), batchID)
	}
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return RoomMessageBatchSendItem{}, false
	}
	if !found {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "未找到记录", nil)
		return RoomMessageBatchSendItem{}, false
	}
	if batch.UserID != userID {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, "无操作权限", nil)
		return RoomMessageBatchSendItem{}, false
	}
	return batch, true
}

func (h *RoomMessageBatchSendHandler) batchWriteFromParams(w http.ResponseWriter, ctx context.Context, params map[string]any, corpID int, userID int, user User) (RoomMessageBatchSendWrite, bool) {
	batchTitle := strings.TrimSpace(stringParam(params, "batchTitle"))
	if batchTitle == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "群发名称 必填", nil)
		return RoomMessageBatchSendWrite{}, false
	}
	if len(batchTitle) > 100 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "群发名称不可超过100字", nil)
		return RoomMessageBatchSendWrite{}, false
	}
	employeeIDs, err := intSliceParam(params, "employeeIds")
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "employeeIds 必须为整型数组", nil)
		return RoomMessageBatchSendWrite{}, false
	}
	employeeIDs = uniquePositiveIntsLocal(employeeIDs)
	if len(employeeIDs) == 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "群主 必填", nil)
		return RoomMessageBatchSendWrite{}, false
	}
	valid, err := h.store.RoomMessageBatchSendValidateEmployees(ctx, corpID, employeeIDs)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return RoomMessageBatchSendWrite{}, false
	}
	if !valid {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "群主参数有误", nil)
		return RoomMessageBatchSendWrite{}, false
	}
	sendWay, has, err := intParam(params, "sendWay")
	if err != nil || !has || (sendWay != 1 && sendWay != 2) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "sendWay 值必须在列表内：[1,2]", nil)
		return RoomMessageBatchSendWrite{}, false
	}
	content, ok := contactMessageBatchSendContents(w, params["content"])
	if !ok {
		return RoomMessageBatchSendWrite{}, false
	}
	mediumID, mediumPresent, mediumErr := intParam(params, "mediumId")
	if mediumErr != nil || (mediumPresent && mediumID < 0) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "mediumId 必须为非负整数", nil)
		return RoomMessageBatchSendWrite{}, false
	}
	if mediumID > 0 {
		validator, configured := h.store.(mediumAvailabilityValidator)
		if !configured {
			writeEnvelope(w, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "素材 Provider 未配置", nil)
			return RoomMessageBatchSendWrite{}, false
		}
		available, err := validator.MediumAvailableToUser(ctx, corpID, userID, mediumID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return RoomMessageBatchSendWrite{}, false
		}
		if !available {
			writeEnvelope(w, http.StatusConflict, http.StatusConflict, "素材不可用于当前企业或权限范围", nil)
			return RoomMessageBatchSendWrite{}, false
		}
	}
	definiteTime := strings.TrimSpace(stringParam(params, "definiteTime"))
	if sendWay == 2 && definiteTime == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "definiteTime 必填", nil)
		return RoomMessageBatchSendWrite{}, false
	}
	if definiteTime != "" {
		if _, err := time.ParseInLocation("2006-01-02 15:04:05", definiteTime, time.Local); err != nil {
			writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "definiteTime 日期格式错误", nil)
			return RoomMessageBatchSendWrite{}, false
		}
	}
	userName := strings.TrimSpace(user.Name)
	if userName == "" {
		userName = strings.TrimSpace(user.Phone)
	}
	return RoomMessageBatchSendWrite{
		CorpID:             corpID,
		UserID:             userID,
		MediumID:           mediumID,
		UserName:           userName,
		EmployeeIDs:        employeeIDs,
		BatchTitle:         batchTitle,
		Content:            content,
		ContentJSON:        mustJSON(content),
		SendWay:            sendWay,
		DefiniteTime:       definiteTime,
		ProcessImmediately: sendWay == 1,
	}, true
}

func (h *RoomMessageBatchSendHandler) sendBatchNow(w http.ResponseWriter, ctx context.Context, corpID int, batchID int) bool {
	_ = corpID
	sent, err := submitRoomMessageBatchSend(ctx, h.store, h.client, h.fileStorageRoot, batchID)
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return false
	}
	if !sent {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "未找到可发送记录", nil)
		return false
	}
	return true
}

func (h *RoomMessageBatchSendHandler) prepareSendContent(w http.ResponseWriter, ctx context.Context, credential RoomWelcomeCorpCredential, content []ContactMessageBatchSendContent) ([]ContactMessageBatchSendContent, bool) {
	prepared := make([]ContactMessageBatchSendContent, 0, len(content))
	for _, item := range content {
		switch item.MsgType {
		case "image":
			if item.PicURL != "" {
				localPath, filePath, err := contactMessageBatchSendMediaFilePath(h.fileStorageRoot, item.PicURL)
				if err != nil {
					writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "图片处理失败", nil)
					return nil, false
				}
				mediaID, err := h.client.UploadTemporaryImage(ctx, credential, filePath)
				if err != nil {
					writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "上传图片失败", nil)
					return nil, false
				}
				item.PicURL = localPath
				item.MediaID = mediaID
			}
		case "miniprogram":
			if item.PicURL != "" {
				localPath, filePath, err := contactMessageBatchSendMediaFilePath(h.fileStorageRoot, item.PicURL)
				if err != nil {
					writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "图片处理失败", nil)
					return nil, false
				}
				mediaID, err := h.client.UploadTemporaryImage(ctx, credential, filePath)
				if err != nil {
					writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "上传图片失败", nil)
					return nil, false
				}
				item.PicURL = localPath
				item.PicMediaID = mediaID
			}
		}
		prepared = append(prepared, item)
	}
	return prepared, true
}

func (h *RoomMessageBatchSendHandler) batchListPayload(item RoomMessageBatchSendItem) map[string]any {
	return map[string]any{
		"id":               item.ID,
		"mediumId":         item.MediumID,
		"batchTitle":       item.BatchTitle,
		"sendWay":          item.SendWay,
		"content":          h.contentPayload(item.Content),
		"sendTime":         item.SendTime,
		"sendTotal":        item.SendTotal,
		"notSendTotal":     item.NotSendTotal,
		"receivedTotal":    item.ReceivedTotal,
		"notReceivedTotal": item.NotReceivedTotal,
		"definiteTime":     item.DefiniteTime,
		"sendStatus":       item.SendStatus,
		"createdAt":        item.CreatedAt,
	}
}

func (h *RoomMessageBatchSendHandler) batchShowPayload(item RoomMessageBatchSendItem) map[string]any {
	return map[string]any{
		"id":                item.ID,
		"mediumId":          item.MediumID,
		"batchTitle":        item.BatchTitle,
		"creator":           item.UserName,
		"createdAt":         item.CreatedAt,
		"content":           h.contentPayload(item.Content),
		"sendTime":          item.SendTime,
		"sendEmployeeTotal": item.SendEmployeeTotal,
		"sendRoomTotal":     item.SendRoomTotal,
		"sendTotal":         item.SendTotal,
		"receivedTotal":     item.ReceivedTotal,
		"notSendTotal":      item.NotSendTotal,
		"notReceivedTotal":  item.NotReceivedTotal,
	}
}

func (h *RoomMessageBatchSendHandler) contentPayload(content []ContactMessageBatchSendContent) []map[string]any {
	list := make([]map[string]any, 0, len(content))
	for _, item := range content {
		payload := map[string]any{"msgType": item.MsgType}
		if item.Content != "" {
			payload["content"] = item.Content
		}
		if item.MediaID != "" {
			payload["media_id"] = item.MediaID
		}
		if item.PicURL != "" {
			payload["pic_url"] = h.fullStaticURL(item.PicURL)
		}
		if item.Title != "" {
			payload["title"] = item.Title
		}
		if item.Desc != "" {
			payload["desc"] = item.Desc
		}
		if item.URL != "" {
			payload["url"] = item.URL
		}
		if item.AppID != "" {
			payload["appid"] = item.AppID
		}
		if item.Page != "" {
			payload["page"] = item.Page
		}
		if item.PicMediaID != "" {
			payload["pic_media_id"] = item.PicMediaID
		}
		list = append(list, payload)
	}
	return list
}

func (h *RoomMessageBatchSendHandler) fullStaticURL(path string) string {
	if path == "" || strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") || h.apiBaseURL == "" {
		return path
	}
	return h.apiBaseURL + "/static/" + strings.TrimLeft(path, "/")
}

func roomMessageBatchSendReminderText(createdAt string) string {
	return "【任务提醒】有新的任务啦！\n" +
		"任务类型：客户群群发任务\n" +
		"创建时间：" + createdAt + "\n" +
		"可前往【客户群】中确认发送，记得及时完成哦\n"
}
