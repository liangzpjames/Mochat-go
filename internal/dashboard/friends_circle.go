package dashboard

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

var ErrFriendsCirclePublisherNotConfigured = errors.New("friends circle publisher not configured")

type FriendsCircleTask struct {
	ID              int    `json:"id"`
	TaskName        string `json:"taskName"`
	SendWay         string `json:"sendWay"`
	Content         string `json:"content"`
	TargetEmployees string `json:"targetEmployees"`
	Status          string `json:"status"`
	CompletedTotal  int    `json:"completedTotal"`
	TargetTotal     int    `json:"targetTotal"`
	CreatorName     string `json:"creatorName"`
	CreatedAt       string `json:"createdAt"`
	StartAt         string `json:"startAt"`
	EndAt           string `json:"endAt"`
	ExternalTaskID  string `json:"externalTaskId"`
	FailureReason   string `json:"failureReason"`
}

type FriendsCircleMaterial struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Content     string `json:"content"`
	Status      string `json:"status"`
	CreatorName string `json:"creatorName"`
	CreatedAt   string `json:"createdAt"`
}

type FriendsCircleTaskFilter struct {
	CorpID   int
	TaskName string
	Status   string
	Page     int
	PerPage  int
}
type FriendsCircleMaterialFilter struct {
	CorpID  int
	Keyword string
	Type    string
	Page    int
	PerPage int
}
type FriendsCircleTaskPage struct {
	Items     []FriendsCircleTask
	Total     int
	Page      int
	PerPage   int
	TotalPage int
}
type FriendsCircleMaterialPage struct {
	Items     []FriendsCircleMaterial
	Total     int
	Page      int
	PerPage   int
	TotalPage int
}
type FriendsCircleTaskWrite struct {
	CorpID          int
	UserID          int
	CreatorName     string
	TaskName        string
	SendWay         string
	Content         string
	TargetEmployees string
	Status          string
}
type FriendsCircleMaterialWrite struct {
	CorpID      int
	UserID      int
	CreatorName string
	Name        string
	Type        string
	Content     string
	Status      string
}

type FriendsCircleStore interface {
	UserByID(context.Context, int) (User, bool, error)
	EmployeeIDByUserCorp(context.Context, int, int) (int, error)
	FirstEmployeeByUser(context.Context, int) (int, int, bool, error)
	FriendsCircleTaskPage(context.Context, FriendsCircleTaskFilter) (FriendsCircleTaskPage, error)
	FriendsCircleMaterialPage(context.Context, FriendsCircleMaterialFilter) (FriendsCircleMaterialPage, error)
	FriendsCircleTaskByID(context.Context, int, int) (FriendsCircleTask, bool, error)
	ClaimFriendsCircleTaskForPublish(context.Context, int, int) (FriendsCircleTask, bool, error)
	CreateFriendsCircleTask(context.Context, FriendsCircleTaskWrite) (int, error)
	CreateFriendsCircleMaterial(context.Context, FriendsCircleMaterialWrite) (int, error)
	MarkFriendsCircleTaskPublished(context.Context, int, int, string) (bool, error)
	MarkFriendsCircleTaskPublishFailed(context.Context, int, int, string) error
}

type FriendsCirclePublisher interface {
	Publish(context.Context, int, int) (string, error)
}

type FriendsCircleHandler struct {
	store      FriendsCircleStore
	cache      LoginCache
	resolver   UserIDResolver
	authorizer CorpAdminAuthorizer
	publisher  FriendsCirclePublisher
}

func NewFriendsCircleHandler(store FriendsCircleStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer, publisher FriendsCirclePublisher) *FriendsCircleHandler {
	return &FriendsCircleHandler{store: store, cache: cache, resolver: resolver, authorizer: authorizer, publisher: publisher}
}

func (h *FriendsCircleHandler) TaskIndex(w http.ResponseWriter, r *http.Request) {
	userID, corpID, _, ok := h.authorized(w, r, "/dashboard/friendsCircle/taskIndex#get", http.MethodGet)
	if !ok {
		return
	}
	_ = userID
	page := positiveQueryInt(r, "page", 1)
	perPage := min(positiveQueryInt(r, "perPage", 20), 100)
	result, err := h.store.FriendsCircleTaskPage(r.Context(), FriendsCircleTaskFilter{CorpID: corpID, TaskName: strings.TrimSpace(r.URL.Query().Get("taskName")), Status: strings.TrimSpace(r.URL.Query().Get("status")), Page: page, PerPage: perPage})
	if err != nil {
		writeEnvelope(w, 500, 500, err.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 200, "success", map[string]any{"list": result.Items, "page": map[string]any{"total": result.Total, "perPage": result.PerPage, "totalPage": result.TotalPage}})
}

func (h *FriendsCircleHandler) MaterialIndex(w http.ResponseWriter, r *http.Request) {
	_, corpID, _, ok := h.authorized(w, r, "/dashboard/friendsCircle/materialIndex#get", http.MethodGet)
	if !ok {
		return
	}
	page := positiveQueryInt(r, "page", 1)
	perPage := min(positiveQueryInt(r, "perPage", 20), 100)
	result, err := h.store.FriendsCircleMaterialPage(r.Context(), FriendsCircleMaterialFilter{CorpID: corpID, Keyword: strings.TrimSpace(r.URL.Query().Get("keyword")), Type: strings.TrimSpace(r.URL.Query().Get("type")), Page: page, PerPage: perPage})
	if err != nil {
		writeEnvelope(w, 500, 500, err.Error(), nil)
		return
	}
	writeEnvelope(w, 200, 200, "success", map[string]any{"list": result.Items, "page": map[string]any{"total": result.Total, "perPage": result.PerPage, "totalPage": result.TotalPage}})
}

func (h *FriendsCircleHandler) TaskStore(w http.ResponseWriter, r *http.Request) {
	userID, corpID, user, ok := h.authorized(w, r, "/dashboard/friendsCircle/taskStore#post", http.MethodPost)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, 400, 400, "invalid request body", nil)
		return
	}
	name, sendWay, content := stringParam(params, "taskName"), stringParam(params, "sendWay"), stringParam(params, "content")
	if name == "" || content == "" || (sendWay != "manual" && sendWay != "scheduled") {
		writeEnvelope(w, 400, 400, "taskName, content and valid sendWay are required", nil)
		return
	}
	target := mustJSON(params["targetEmployees"])
	id, err := h.store.CreateFriendsCircleTask(r.Context(), FriendsCircleTaskWrite{CorpID: corpID, UserID: userID, CreatorName: user.Name, TaskName: name, SendWay: sendWay, Content: content, TargetEmployees: target, Status: "draft"})
	if err != nil || id <= 0 {
		writeEnvelope(w, 500, 500, "create task failed", nil)
		return
	}
	writeEnvelope(w, 200, 200, "success", map[string]any{"id": id, "status": "draft"})
}

func (h *FriendsCircleHandler) MaterialStore(w http.ResponseWriter, r *http.Request) {
	userID, corpID, user, ok := h.authorized(w, r, "/dashboard/friendsCircle/materialStore#post", http.MethodPost)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, 400, 400, "invalid request body", nil)
		return
	}
	name, kind := stringParam(params, "name"), stringParam(params, "type")
	if name == "" || !map[string]bool{"text": true, "image": true, "video": true, "link": true}[kind] || params["content"] == nil {
		writeEnvelope(w, 400, 400, "name, valid type and content are required", nil)
		return
	}
	id, err := h.store.CreateFriendsCircleMaterial(r.Context(), FriendsCircleMaterialWrite{CorpID: corpID, UserID: userID, CreatorName: user.Name, Name: name, Type: kind, Content: mustJSON(params["content"]), Status: "available"})
	if err != nil || id <= 0 {
		writeEnvelope(w, 500, 500, "create material failed", nil)
		return
	}
	writeEnvelope(w, 200, 200, "success", map[string]any{"id": id, "status": "available"})
}

func (h *FriendsCircleHandler) Publish(w http.ResponseWriter, r *http.Request) {
	_, corpID, _, ok := h.authorized(w, r, "/dashboard/friendsCircle/publish#post", http.MethodPost)
	if !ok {
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, 400, 400, "invalid request body", nil)
		return
	}
	taskID, present, err := intParam(params, "taskId")
	if err != nil || !present || taskID <= 0 {
		writeEnvelope(w, 400, 400, "taskId is required", nil)
		return
	}
	if h.publisher == nil {
		writeEnvelope(w, 503, 503, ErrFriendsCirclePublisherNotConfigured.Error(), nil)
		return
	}
	task, claimed, err := h.store.ClaimFriendsCircleTaskForPublish(r.Context(), corpID, taskID)
	if err != nil {
		writeEnvelope(w, 500, 500, err.Error(), nil)
		return
	}
	if !claimed {
		_, found, lookupErr := h.store.FriendsCircleTaskByID(r.Context(), corpID, taskID)
		if lookupErr != nil {
			writeEnvelope(w, 500, 500, lookupErr.Error(), nil)
		} else if !found {
			writeEnvelope(w, 404, 404, "task not found", nil)
		} else {
			writeEnvelope(w, 422, 422, "task is not publishable", nil)
		}
		return
	}
	_ = task
	externalID, err := h.publisher.Publish(r.Context(), corpID, taskID)
	if err != nil {
		if markErr := h.store.MarkFriendsCircleTaskPublishFailed(r.Context(), corpID, taskID, err.Error()); markErr != nil {
			writeEnvelope(w, 500, 500, markErr.Error(), nil)
			return
		}
		writeEnvelope(w, 503, 503, err.Error(), nil)
		return
	}
	updated, err := h.store.MarkFriendsCircleTaskPublished(r.Context(), corpID, taskID, externalID)
	if err != nil || !updated {
		writeEnvelope(w, 500, 500, "publish state reconciliation required", nil)
		return
	}
	writeEnvelope(w, 200, 200, "success", map[string]any{"externalTaskId": externalID, "status": "queued"})
}

func (h *FriendsCircleHandler) authorized(w http.ResponseWriter, r *http.Request, permission string, method string) (int, int, User, bool) {
	if r.Method != method {
		writeEnvelope(w, 405, 405, "method not allowed", nil)
		return 0, 0, User{}, false
	}
	userID, err := h.resolver.UserID(r)
	if err != nil || userID <= 0 {
		writeEnvelope(w, 401, 401, "unauthorized", nil)
		return 0, 0, User{}, false
	}
	user, found, err := h.store.UserByID(r.Context(), userID)
	if err != nil || !found {
		writeEnvelope(w, 401, 401, "user not found", nil)
		return 0, 0, User{}, false
	}
	cacheValue := ""
	if h.cache != nil {
		cacheValue, err = h.cache.UserCorpCache(r.Context(), userID)
		if err != nil {
			writeEnvelope(w, 500, 500, err.Error(), nil)
			return 0, 0, User{}, false
		}
	}
	login, err := ResolveValidatedLoginCorpInfoFromStore(r.Context(), r.Header, user, cacheValue, h.store)
	if err != nil {
		writeEnvelope(w, 500, 500, err.Error(), nil)
		return 0, 0, User{}, false
	}
	info := LoginCorpInfo(login)
	corpID, ok := selectedCorpID(w, info)
	if !ok {
		return 0, 0, User{}, false
	}
	if h.authorizer != nil {
		if _, err := h.authorizer.Resolve(r.Context(), userID, permission, corpID, info.WorkEmployeeID); err != nil {
			writeAccessError(w, err)
			return 0, 0, User{}, false
		}
	}
	return userID, corpID, user, true
}
