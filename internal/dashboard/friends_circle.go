package dashboard

import (
	"context"
	"crypto/subtle"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

var ErrFriendsCirclePublisherNotConfigured = errors.New("friends circle publisher not configured")
var ErrFriendsCircleInvalidTransition = errors.New("invalid friends circle task state transition")

func filterFriendsCircleTasksByEmployees(items []FriendsCircleTask, allowed []int) []FriendsCircleTask {
	set := make(map[int]struct{}, len(allowed))
	for _, id := range allowed {
		set[id] = struct{}{}
	}
	out := make([]FriendsCircleTask, 0, len(items))
	for _, item := range items {
		var ids []int
		if json.Unmarshal([]byte(item.TargetEmployees), &ids) != nil {
			continue
		}
		for _, id := range ids {
			if _, ok := set[id]; ok {
				out = append(out, item)
				break
			}
		}
	}
	return out
}
func filterFriendsCircleResultsByEmployees(items []FriendsCircleTaskResult, allowed []int) []FriendsCircleTaskResult {
	set := make(map[int]struct{}, len(allowed))
	for _, id := range allowed {
		set[id] = struct{}{}
	}
	out := make([]FriendsCircleTaskResult, 0, len(items))
	for _, item := range items {
		if _, ok := set[item.TargetEmployeeID]; ok {
			out = append(out, item)
		}
	}
	return out
}

type FriendsCircleTask struct {
	ID              int    `json:"id"`
	TaskName        string `json:"taskName"`
	SendWay         string `json:"sendWay"`
	Content         string `json:"content"`
	MediumID        int    `json:"mediumId"`
	TargetEmployees string `json:"targetEmployees"`
	Status          string `json:"status"`
	CompletedTotal  int    `json:"completedTotal"`
	TargetTotal     int    `json:"targetTotal"`
	CreatorName     string `json:"creatorName"`
	CreatedAt       string `json:"createdAt"`
	StartAt         string `json:"startAt"`
	EndAt           string `json:"endAt"`
	ExternalTaskID  string `json:"externalTaskId"`
	PublishAttempts int    `json:"publishAttempts"`
	FailureReason   string `json:"failureReason"`
	LastCallbackAt  string `json:"lastCallbackAt"`
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
	CorpID              int
	TaskName            string
	Status              string
	Page                int
	PerPage             int
	AllowedEmployeeIDs  []int
	RestrictEmployeeIDs bool
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

type FriendsCircleTaskResult struct {
	ID               int    `json:"id"`
	TaskID           int    `json:"taskId"`
	TargetEmployeeID int    `json:"targetEmployeeId"`
	Status           string `json:"status"`
	FailureCode      string `json:"failureCode"`
	FailureReason    string `json:"failureReason"`
	OccurredAt       string `json:"occurredAt"`
}

type FriendsCircleTaskResultFilter struct {
	CorpID              int
	TaskID              int
	Status              string
	Page                int
	PerPage             int
	AllowedEmployeeIDs  []int
	RestrictEmployeeIDs bool
}

type FriendsCircleTaskResultPage struct {
	Items     []FriendsCircleTaskResult
	Total     int
	Page      int
	PerPage   int
	TotalPage int
}

type FriendsCircleTaskResultWrite struct {
	TargetEmployeeID int
	Status           string
	FailureCode      string
	FailureReason    string
}

type FriendsCircleCallback struct {
	CorpID         int
	ExternalTaskID string
	Status         string
	CompletedTotal int
	TargetTotal    int
	FailureReason  string
	Results        []FriendsCircleTaskResultWrite
}
type FriendsCircleTaskWrite struct {
	CorpID          int
	UserID          int
	CreatorName     string
	TaskName        string
	SendWay         string
	Content         string
	MediumID        int
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
	FriendsCircleTaskResultPage(context.Context, FriendsCircleTaskResultFilter) (FriendsCircleTaskResultPage, error)
	FriendsCircleTaskByID(context.Context, int, int) (FriendsCircleTask, bool, error)
	ClaimFriendsCircleTaskForPublish(context.Context, int, int) (FriendsCircleTask, bool, error)
	ApplyFriendsCircleCallback(context.Context, FriendsCircleCallback) (FriendsCircleTask, bool, error)
	CreateFriendsCircleTask(context.Context, FriendsCircleTaskWrite) (int, error)
	CreateFriendsCircleMaterial(context.Context, FriendsCircleMaterialWrite) (int, error)
	MarkFriendsCircleTaskPublished(context.Context, int, int, string) (bool, error)
	MarkFriendsCircleTaskPublishFailed(context.Context, int, int, string) error
}

type FriendsCirclePublisher interface {
	Publish(context.Context, int, int) (string, error)
}

type unavailableFriendsCirclePublisher struct{}

func NewUnavailableFriendsCirclePublisher() FriendsCirclePublisher {
	return unavailableFriendsCirclePublisher{}
}

func (unavailableFriendsCirclePublisher) Publish(context.Context, int, int) (string, error) {
	return "", ErrFriendsCirclePublisherNotConfigured
}

type FriendsCircleHandler struct {
	store         FriendsCircleStore
	cache         LoginCache
	resolver      UserIDResolver
	authorizer    CorpAdminAuthorizer
	publisher     FriendsCirclePublisher
	callbackToken string
}

func NewFriendsCircleHandler(store FriendsCircleStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer, publisher FriendsCirclePublisher) *FriendsCircleHandler {
	return NewFriendsCircleHandlerWithCallbackToken(store, cache, resolver, authorizer, publisher, "")
}

func NewFriendsCircleHandlerWithCallbackToken(store FriendsCircleStore, cache LoginCache, resolver UserIDResolver, authorizer CorpAdminAuthorizer, publisher FriendsCirclePublisher, callbackToken string) *FriendsCircleHandler {
	return &FriendsCircleHandler{store: store, cache: cache, resolver: resolver, authorizer: authorizer, publisher: publisher, callbackToken: strings.TrimSpace(callbackToken)}
}

func (h *FriendsCircleHandler) TaskIndex(w http.ResponseWriter, r *http.Request) {
	userID, corpID, _, ok := h.authorized(w, r, "/dashboard/friendsCircle/taskIndex#get", http.MethodGet)
	if !ok {
		return
	}
	_ = userID
	page := positiveQueryInt(r, "page", 1)
	perPage := min(positiveQueryInt(r, "perPage", 20), 100)
	access, _ := DashboardAccessFromContext(r.Context())
	result, err := h.store.FriendsCircleTaskPage(r.Context(), FriendsCircleTaskFilter{CorpID: corpID, TaskName: strings.TrimSpace(r.URL.Query().Get("taskName")), Status: strings.TrimSpace(r.URL.Query().Get("status")), Page: page, PerPage: perPage, AllowedEmployeeIDs: access.AllowedEmployeeIDs, RestrictEmployeeIDs: access.ScopeRequired && access.Scope != DataScopeTenant})
	if err != nil {
		writeEnvelope(w, 500, 500, err.Error(), nil)
		return
	}
	if access, scoped := DashboardAccessFromContext(r.Context()); scoped && access.ScopeRequired && access.Scope != DataScopeTenant {
		result.Items = filterFriendsCircleTasksByEmployees(result.Items, access.AllowedEmployeeIDs)
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

func (h *FriendsCircleHandler) TaskResultIndex(w http.ResponseWriter, r *http.Request) {
	_, corpID, _, ok := h.authorized(w, r, "/dashboard/friendsCircle/taskResultIndex#get", http.MethodGet)
	if !ok {
		return
	}
	taskID, present, err := intParam(map[string]any{"taskId": r.URL.Query().Get("taskId")}, "taskId")
	if err != nil || !present || taskID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "taskId is required", nil)
		return
	}
	page := positiveQueryInt(r, "page", 1)
	perPage := min(positiveQueryInt(r, "perPage", 20), 100)
	access, _ := DashboardAccessFromContext(r.Context())
	result, err := h.store.FriendsCircleTaskResultPage(r.Context(), FriendsCircleTaskResultFilter{CorpID: corpID, TaskID: taskID, Status: strings.TrimSpace(r.URL.Query().Get("status")), Page: page, PerPage: perPage, AllowedEmployeeIDs: access.AllowedEmployeeIDs, RestrictEmployeeIDs: access.ScopeRequired && access.Scope != DataScopeTenant})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if access, scoped := DashboardAccessFromContext(r.Context()); scoped && access.ScopeRequired && access.Scope != DataScopeTenant {
		result.Items = filterFriendsCircleResultsByEmployees(result.Items, access.AllowedEmployeeIDs)
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"list": result.Items, "page": map[string]any{"total": result.Total, "perPage": result.PerPage, "totalPage": result.TotalPage}})
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
	mediumID, mediumPresent, mediumErr := intParam(params, "mediumId")
	if mediumErr != nil || (mediumPresent && mediumID < 0) {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "mediumId 无效", nil)
		return
	}
	if mediumID > 0 {
		validator, configured := h.store.(mediumAvailabilityValidator)
		if !configured {
			writeEnvelope(w, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "素材引用校验 Provider 未配置", nil)
			return
		}
		available, err := validator.MediumAvailableToUser(r.Context(), corpID, userID, mediumID)
		if err != nil {
			writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
			return
		}
		if !available {
			writeEnvelope(w, http.StatusConflict, http.StatusConflict, "素材不可用于当前企业或权限范围", nil)
			return
		}
	}
	target := mustJSON(params["targetEmployees"])
	if access, scoped := DashboardAccessFromContext(r.Context()); scoped && access.ScopeRequired && access.Scope != DataScopeTenant {
		var targetIDs []int
		if json.Unmarshal([]byte(target), &targetIDs) != nil || !employeeIDsWithinDashboardScope(targetIDs, access.AllowedEmployeeIDs) {
			writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, "employee scope denied", nil)
			return
		}
	}
	id, err := h.store.CreateFriendsCircleTask(r.Context(), FriendsCircleTaskWrite{CorpID: corpID, UserID: userID, CreatorName: user.Name, TaskName: name, SendWay: sendWay, Content: content, MediumID: mediumID, TargetEmployees: target, Status: "draft"})
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
	if access, ok := DashboardAccessFromContext(r.Context()); ok && access.ScopeRequired && access.Scope != DataScopeTenant {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, "employee scope denied", nil)
		return
	}
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

func (h *FriendsCircleHandler) ProviderCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeEnvelope(w, http.StatusMethodNotAllowed, http.StatusMethodNotAllowed, "method not allowed", nil)
		return
	}
	if h.callbackToken == "" {
		writeEnvelope(w, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "friends circle callback verifier not configured", nil)
		return
	}
	provided := strings.TrimSpace(r.Header.Get("X-Mochat-Friends-Circle-Callback-Token"))
	if subtle.ConstantTimeCompare([]byte(provided), []byte(h.callbackToken)) != 1 {
		writeEnvelope(w, http.StatusUnauthorized, http.StatusUnauthorized, "invalid friends circle callback token", nil)
		return
	}
	params, err := parseRequestParams(r)
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid request body", nil)
		return
	}
	corpID, corpPresent, corpErr := intParam(params, "corpId")
	if corpErr != nil || !corpPresent || corpID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "corpId is required", nil)
		return
	}
	status := strings.TrimSpace(stringParam(params, "status"))
	if !friendsCircleCallbackStatuses[status] {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid callback status", nil)
		return
	}
	completedTotal, _, completedErr := intParam(params, "completedTotal")
	targetTotal, _, targetErr := intParam(params, "targetTotal")
	if completedErr != nil || targetErr != nil || completedTotal < 0 || targetTotal < 0 || completedTotal > targetTotal && targetTotal > 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid callback progress", nil)
		return
	}
	results, err := parseFriendsCircleCallbackResults(params["results"])
	if err != nil {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "invalid callback results", nil)
		return
	}
	externalTaskID := stringParam(params, "externalTaskId")
	if externalTaskID == "" {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "externalTaskId is required", nil)
		return
	}
	updated, found, err := h.store.ApplyFriendsCircleCallback(r.Context(), FriendsCircleCallback{CorpID: corpID, ExternalTaskID: externalTaskID, Status: status, CompletedTotal: completedTotal, TargetTotal: targetTotal, FailureReason: stringParam(params, "failureReason"), Results: results})
	if err != nil {
		if errors.Is(err, ErrFriendsCircleInvalidTransition) {
			writeEnvelope(w, http.StatusUnprocessableEntity, http.StatusUnprocessableEntity, err.Error(), nil)
			return
		}
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	if !found {
		writeEnvelope(w, http.StatusNotFound, http.StatusNotFound, "task not found or callback already closed", nil)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"taskId": updated.ID, "status": updated.Status, "completedTotal": updated.CompletedTotal, "targetTotal": updated.TargetTotal})
}

func (h *FriendsCircleHandler) Export(w http.ResponseWriter, r *http.Request) {
	_, corpID, _, ok := h.authorized(w, r, "/dashboard/friendsCircle/export#get", http.MethodGet)
	if !ok {
		return
	}
	taskID, present, err := intParam(map[string]any{"taskId": r.URL.Query().Get("taskId")}, "taskId")
	if err != nil || !present || taskID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "taskId is required", nil)
		return
	}
	result, err := h.store.FriendsCircleTaskResultPage(r.Context(), FriendsCircleTaskResultFilter{CorpID: corpID, TaskID: taskID, Status: strings.TrimSpace(r.URL.Query().Get("status")), Page: 1, PerPage: 10000})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"friends-circle-task-%d-results.csv\"", taskID))
	w.WriteHeader(http.StatusOK)
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"task_id", "target_employee_id", "status", "failure_code", "failure_reason", "occurred_at"})
	for _, item := range result.Items {
		_ = writer.Write([]string{strconv.Itoa(item.TaskID), strconv.Itoa(item.TargetEmployeeID), item.Status, item.FailureCode, item.FailureReason, item.OccurredAt})
	}
	writer.Flush()
}

func (h *FriendsCircleHandler) ExportData(w http.ResponseWriter, r *http.Request) {
	if access, ok := DashboardAccessFromContext(r.Context()); ok && access.ScopeRequired && access.Scope != DataScopeTenant {
		writeEnvelope(w, http.StatusForbidden, http.StatusForbidden, "employee scope denied", nil)
		return
	}
	_, corpID, _, ok := h.authorized(w, r, "/dashboard/friendsCircle/export#get", http.MethodGet)
	if !ok {
		return
	}
	taskID, present, err := intParam(map[string]any{"taskId": r.URL.Query().Get("taskId")}, "taskId")
	if err != nil || !present || taskID <= 0 {
		writeEnvelope(w, http.StatusBadRequest, http.StatusBadRequest, "taskId is required", nil)
		return
	}
	result, err := h.store.FriendsCircleTaskResultPage(r.Context(), FriendsCircleTaskResultFilter{CorpID: corpID, TaskID: taskID, Status: strings.TrimSpace(r.URL.Query().Get("status")), Page: 1, PerPage: 10000})
	if err != nil {
		writeEnvelope(w, http.StatusInternalServerError, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	writeEnvelope(w, http.StatusOK, http.StatusOK, "success", map[string]any{"list": result.Items, "taskId": taskID})
}

var friendsCircleCallbackStatuses = map[string]bool{
	"queued":              true,
	"running":             true,
	"partially_succeeded": true,
	"succeeded":           true,
	"failed":              true,
	"cancelled":           true,
}

func parseFriendsCircleCallbackResults(value any) ([]FriendsCircleTaskResultWrite, error) {
	if value == nil {
		return []FriendsCircleTaskResultWrite{}, nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var results []FriendsCircleTaskResultWrite
	if err := json.Unmarshal(raw, &results); err != nil {
		return nil, err
	}
	for _, result := range results {
		if result.TargetEmployeeID <= 0 || !map[string]bool{"pending": true, "succeeded": true, "failed": true}[result.Status] {
			return nil, errors.New("invalid callback result")
		}
	}
	return results, nil
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
