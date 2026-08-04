package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFriendsCircleTaskIndexUsesSelectedCorpAndFilters(t *testing.T) {
	store := &fakeFriendsCircleStore{users: map[int]User{1: {ID: 1}}, tasks: FriendsCircleTaskPage{Items: []FriendsCircleTask{{ID: 9, TaskName: "夏日活动", Status: "draft"}}, Total: 1, Page: 1, PerPage: 20}}
	handler := NewFriendsCircleHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/friendsCircle/taskIndex?taskName=%E5%A4%8F%E6%97%A5&status=draft&page=1&perPage=20", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.TaskIndex(rec, req)
	if rec.Code != http.StatusOK || store.taskFilter.CorpID != 7 || store.taskFilter.TaskName != "夏日" || store.taskFilter.Status != "draft" {
		t.Fatalf("status=%d filter=%#v body=%s", rec.Code, store.taskFilter, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "夏日活动") {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestFriendsCircleMaterialIndexUsesSelectedCorp(t *testing.T) {
	store := &fakeFriendsCircleStore{users: map[int]User{1: {ID: 1}}, materials: FriendsCircleMaterialPage{Items: []FriendsCircleMaterial{{ID: 3, Name: "新品海报", Type: "image", Status: "available"}}, Total: 1, Page: 1, PerPage: 20}}
	handler := NewFriendsCircleHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/friendsCircle/materialIndex?keyword=%E6%96%B0%E5%93%81&type=image", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.MaterialIndex(rec, req)
	if rec.Code != http.StatusOK || store.materialFilter.CorpID != 7 || store.materialFilter.Keyword != "新品" || store.materialFilter.Type != "image" {
		t.Fatalf("status=%d filter=%#v body=%s", rec.Code, store.materialFilter, rec.Body.String())
	}
}

func TestFriendsCircleStoresRealDraftsWithServerOwnedCorp(t *testing.T) {
	store := &fakeFriendsCircleStore{users: map[int]User{1: {ID: 1}}, taskID: 11, materialID: 12}
	handler := NewFriendsCircleHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, nil)

	req := httptest.NewRequest(http.MethodPost, "/dashboard/friendsCircle/taskStore", strings.NewReader(`{"taskName":"夏日活动","sendWay":"manual","content":"欢迎参加","mediumId":21,"corpId":999}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.TaskStore(rec, req)
	if rec.Code != http.StatusOK || store.createdTask.CorpID != 7 || store.createdTask.UserID != 1 || store.createdTask.Status != "draft" || store.createdTask.MediumID != 21 {
		t.Fatalf("status=%d task=%#v body=%s", rec.Code, store.createdTask, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/dashboard/friendsCircle/materialStore", strings.NewReader(`{"name":"新品海报","type":"image","content":{"url":"/storage/a.png"}}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec = httptest.NewRecorder()
	handler.MaterialStore(rec, req)
	if rec.Code != http.StatusOK || store.createdMaterial.CorpID != 7 || store.createdMaterial.Status != "available" {
		t.Fatalf("status=%d material=%#v body=%s", rec.Code, store.createdMaterial, rec.Body.String())
	}
}

func TestFriendsCircleRejectsUnavailableMaterial(t *testing.T) {
	store := &fakeFriendsCircleStore{
		users:              map[int]User{1: {ID: 1}},
		mediumAvailableSet: true,
		mediumAvailable:    false,
	}
	handler := NewFriendsCircleHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, nil)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/friendsCircle/taskStore", strings.NewReader(`{"taskName":"夏日活动","sendWay":"manual","content":"欢迎参加","mediumId":21}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.TaskStore(rec, req)
	if rec.Code != http.StatusConflict || store.createdTask.TaskName != "" || !strings.Contains(rec.Body.String(), "素材不可用于当前企业或权限范围") {
		t.Fatalf("status=%d task=%#v body=%s", rec.Code, store.createdTask, rec.Body.String())
	}
}

func TestFriendsCirclePublishReturns503WithoutPublisherAndPreservesDraft(t *testing.T) {
	store := &fakeFriendsCircleStore{users: map[int]User{1: {ID: 1}}}
	handler := NewFriendsCircleHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, nil)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/friendsCircle/publish", strings.NewReader(`{"taskId":11}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Publish(rec, req)
	if rec.Code != http.StatusServiceUnavailable || store.publishedTaskID != 0 {
		t.Fatalf("status=%d published=%d body=%s", rec.Code, store.publishedTaskID, rec.Body.String())
	}
}

func TestFriendsCircleUnavailablePublisherPersistsFailedState(t *testing.T) {
	store := &fakeFriendsCircleStore{
		users: map[int]User{1: {ID: 1}},
		tasks: FriendsCircleTaskPage{Items: []FriendsCircleTask{{ID: 11, TaskName: "夏日活动", Status: "draft"}}},
	}
	handler := NewFriendsCircleHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, NewUnavailableFriendsCirclePublisher())
	req := httptest.NewRequest(http.MethodPost, "/dashboard/friendsCircle/publish", strings.NewReader(`{"taskId":11}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Publish(rec, req)
	if rec.Code != http.StatusServiceUnavailable || store.failedTaskID != 11 || store.failedReason == "" {
		t.Fatalf("status=%d failedTask=%d reason=%q body=%s", rec.Code, store.failedTaskID, store.failedReason, rec.Body.String())
	}
}

func TestFriendsCircleProviderCallbackPersistsProgressAndFailureDetails(t *testing.T) {
	store := &fakeFriendsCircleStore{}
	handler := NewFriendsCircleHandlerWithCallbackToken(store, nil, HeaderUserIDResolver{}, nil, nil, "callback-secret")
	req := httptest.NewRequest(http.MethodPost, "/dashboard/friendsCircle/providerCallback", strings.NewReader(`{"corpId":7,"externalTaskId":"external-11","status":"partially_succeeded","completedTotal":1,"targetTotal":2,"failureReason":"部分失败","results":[{"targetEmployeeId":99,"status":"failed","failureCode":"E_TIMEOUT","failureReason":"发送超时"}]}`))
	req.Header.Set("X-Mochat-Friends-Circle-Callback-Token", "callback-secret")
	rec := httptest.NewRecorder()
	handler.ProviderCallback(rec, req)
	if rec.Code != http.StatusOK || store.callback.ExternalTaskID != "external-11" || len(store.callback.Results) != 1 || store.callback.Results[0].FailureCode != "E_TIMEOUT" {
		t.Fatalf("status=%d callback=%#v body=%s", rec.Code, store.callback, rec.Body.String())
	}
}

func TestFriendsCircleTaskResultIndexAndExportUseSelectedCorp(t *testing.T) {
	store := &fakeFriendsCircleStore{
		users:   map[int]User{1: {ID: 1}},
		results: FriendsCircleTaskResultPage{Items: []FriendsCircleTaskResult{{ID: 31, TaskID: 11, TargetEmployeeID: 99, Status: "failed", FailureCode: "E_TIMEOUT", FailureReason: "发送超时"}}, Total: 1, Page: 1, PerPage: 20},
	}
	handler := NewFriendsCircleHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/friendsCircle/taskResultIndex?taskId=11&status=failed", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.TaskResultIndex(rec, req)
	if rec.Code != http.StatusOK || store.resultFilter.CorpID != 7 || store.resultFilter.TaskID != 11 || store.resultFilter.Status != "failed" || !strings.Contains(rec.Body.String(), "E_TIMEOUT") {
		t.Fatalf("status=%d filter=%#v body=%s", rec.Code, store.resultFilter, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/dashboard/friendsCircle/export?taskId=11", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec = httptest.NewRecorder()
	handler.Export(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Type"), "text/csv") || !strings.Contains(rec.Body.String(), "target_employee_id") || !strings.Contains(rec.Body.String(), "E_TIMEOUT") {
		t.Fatalf("status=%d contentType=%q body=%s", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}
}

type fakeFriendsCircleStore struct {
	users              map[int]User
	tasks              FriendsCircleTaskPage
	materials          FriendsCircleMaterialPage
	results            FriendsCircleTaskResultPage
	taskFilter         FriendsCircleTaskFilter
	materialFilter     FriendsCircleMaterialFilter
	resultFilter       FriendsCircleTaskResultFilter
	callback           FriendsCircleCallback
	createdTask        FriendsCircleTaskWrite
	createdMaterial    FriendsCircleMaterialWrite
	taskID             int
	materialID         int
	publishedTaskID    int
	failedTaskID       int
	failedReason       string
	mediumAvailableSet bool
	mediumAvailable    bool
}

func (s *fakeFriendsCircleStore) UserByID(_ context.Context, id int) (User, bool, error) {
	u, ok := s.users[id]
	return u, ok, nil
}
func (s *fakeFriendsCircleStore) EmployeeIDByUserCorp(context.Context, int, int) (int, error) {
	return 0, nil
}
func (s *fakeFriendsCircleStore) FirstEmployeeByUser(context.Context, int) (int, int, bool, error) {
	return 0, 0, false, nil
}
func (s *fakeFriendsCircleStore) MediumAvailableToUser(context.Context, int, int, int) (bool, error) {
	if s.mediumAvailableSet {
		return s.mediumAvailable, nil
	}
	return true, nil
}
func (s *fakeFriendsCircleStore) FriendsCircleTaskPage(_ context.Context, filter FriendsCircleTaskFilter) (FriendsCircleTaskPage, error) {
	s.taskFilter = filter
	return s.tasks, nil
}
func (s *fakeFriendsCircleStore) FriendsCircleMaterialPage(_ context.Context, filter FriendsCircleMaterialFilter) (FriendsCircleMaterialPage, error) {
	s.materialFilter = filter
	return s.materials, nil
}
func (s *fakeFriendsCircleStore) FriendsCircleTaskResultPage(_ context.Context, filter FriendsCircleTaskResultFilter) (FriendsCircleTaskResultPage, error) {
	s.resultFilter = filter
	return s.results, nil
}
func (s *fakeFriendsCircleStore) FriendsCircleTaskByID(_ context.Context, _ int, taskID int) (FriendsCircleTask, bool, error) {
	for _, task := range s.tasks.Items {
		if task.ID == taskID {
			return task, true, nil
		}
	}
	return FriendsCircleTask{}, false, nil
}
func (s *fakeFriendsCircleStore) ClaimFriendsCircleTaskForPublish(ctx context.Context, corpID int, taskID int) (FriendsCircleTask, bool, error) {
	task, found, err := s.FriendsCircleTaskByID(ctx, corpID, taskID)
	return task, found && task.Status == "draft", err
}
func (s *fakeFriendsCircleStore) CreateFriendsCircleTask(_ context.Context, value FriendsCircleTaskWrite) (int, error) {
	s.createdTask = value
	return s.taskID, nil
}
func (s *fakeFriendsCircleStore) CreateFriendsCircleMaterial(_ context.Context, value FriendsCircleMaterialWrite) (int, error) {
	s.createdMaterial = value
	return s.materialID, nil
}
func (s *fakeFriendsCircleStore) ApplyFriendsCircleCallback(_ context.Context, callback FriendsCircleCallback) (FriendsCircleTask, bool, error) {
	s.callback = callback
	return FriendsCircleTask{ID: 11, Status: callback.Status, CompletedTotal: callback.CompletedTotal, TargetTotal: callback.TargetTotal}, true, nil
}
func (s *fakeFriendsCircleStore) MarkFriendsCircleTaskPublished(_ context.Context, _ int, taskID int, _ string) (bool, error) {
	s.publishedTaskID = taskID
	return true, nil
}
func (s *fakeFriendsCircleStore) MarkFriendsCircleTaskPublishFailed(_ context.Context, _ int, taskID int, reason string) error {
	s.failedTaskID = taskID
	s.failedReason = reason
	return nil
}
