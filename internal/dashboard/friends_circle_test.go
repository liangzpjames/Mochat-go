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

	req := httptest.NewRequest(http.MethodPost, "/dashboard/friendsCircle/taskStore", strings.NewReader(`{"taskName":"夏日活动","sendWay":"manual","content":"欢迎参加","corpId":999}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.TaskStore(rec, req)
	if rec.Code != http.StatusOK || store.createdTask.CorpID != 7 || store.createdTask.UserID != 1 || store.createdTask.Status != "draft" {
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

type fakeFriendsCircleStore struct {
	users           map[int]User
	tasks           FriendsCircleTaskPage
	materials       FriendsCircleMaterialPage
	taskFilter      FriendsCircleTaskFilter
	materialFilter  FriendsCircleMaterialFilter
	createdTask     FriendsCircleTaskWrite
	createdMaterial FriendsCircleMaterialWrite
	taskID          int
	materialID      int
	publishedTaskID int
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
func (s *fakeFriendsCircleStore) FriendsCircleTaskPage(_ context.Context, filter FriendsCircleTaskFilter) (FriendsCircleTaskPage, error) {
	s.taskFilter = filter
	return s.tasks, nil
}
func (s *fakeFriendsCircleStore) FriendsCircleMaterialPage(_ context.Context, filter FriendsCircleMaterialFilter) (FriendsCircleMaterialPage, error) {
	s.materialFilter = filter
	return s.materials, nil
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
func (s *fakeFriendsCircleStore) MarkFriendsCircleTaskPublished(_ context.Context, _ int, taskID int, _ string) (bool, error) {
	s.publishedTaskID = taskID
	return true, nil
}
func (s *fakeFriendsCircleStore) MarkFriendsCircleTaskPublishFailed(context.Context, int, int, string) error {
	return nil
}
