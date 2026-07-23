package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAutoTagIndexReturnsListAndPagination(t *testing.T) {
	store := &fakeAutoTagStore{
		user: User{ID: 1, IsSuperAdmin: 1},
		page: AutoTagPage{
			Page:    1,
			PerPage: 15,
			Total:   1,
			Items: []AutoTagItem{{
				ID:                   31,
				Type:                 1,
				Name:                 "关键词规则",
				FuzzyMatchKeywordRaw: `["报价"]`,
				ExactMatchKeywordRaw: `["下单"]`,
				TagRuleRaw:           `[{"tags":[{"tagid":7,"tagname":"意向"}]}]`,
				TagsRaw:              `["意向"]`,
				OnOff:                1,
				MarkTagCount:         3,
				CreatedAt:            "2026-07-05 10:00:00",
			}},
		},
	}
	handler := NewAutoTagHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/autoTag/index?type=1&name=关键&tags=7&page=1&perPage=15", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Index(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Data struct {
			Total int              `json:"total"`
			List  []map[string]any `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Total != 1 || len(envelope.Data.List) != 1 {
		t.Fatalf("data = %#v", envelope.Data)
	}
	item := envelope.Data.List[0]
	if item["id"].(float64) != 31 || item["name"] != "关键词规则" || item["mark_tag_count"].(float64) != 3 {
		t.Fatalf("item = %#v", item)
	}
	if store.lastFilter.CorpID != 7 || store.lastFilter.Type != 1 || store.lastFilter.Name != "关键" || len(store.lastFilter.Tags) != 1 || store.lastFilter.Tags[0] != 7 {
		t.Fatalf("filter = %#v", store.lastFilter)
	}
}

func TestAutoTagStorePreservesRuleJSONAndFlattensTags(t *testing.T) {
	store := &fakeAutoTagStore{user: User{ID: 1, IsSuperAdmin: 1}, createID: 88}
	handler := NewAutoTagHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/autoTag/store", strings.NewReader(`{"type":1,"name":"关键词规则","employees":["zhangsan"],"fuzzy_match_keyword":["报价"],"exact_match_keyword":["下单"],"tag_rule":[{"time_type":1,"trigger_count":2,"tags":[{"tagid":7,"tagname":"意向"}]}]}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Store(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.created.CorpID != 7 || store.created.CreateUserID != 1 || store.created.Type != 1 || store.created.Name != "关键词规则" {
		t.Fatalf("created = %#v", store.created)
	}
	if !strings.Contains(store.created.TagRuleRaw, "trigger_count") || !strings.Contains(store.created.EmployeesRaw, "zhangsan") || !strings.Contains(store.created.FuzzyMatchKeywordRaw, "报价") {
		t.Fatalf("json raw = %#v", store.created)
	}
	if !strings.Contains(store.created.TagsRaw, "意向") {
		t.Fatalf("flattened tags = %s", store.created.TagsRaw)
	}
}

func TestAutoTagShowReturnsDetailAndStatistics(t *testing.T) {
	store := &fakeAutoTagStore{
		user: User{ID: 1, IsSuperAdmin: 1},
		item: AutoTagItem{
			ID:           31,
			Type:         3,
			Name:         "分时段规则",
			EmployeesRaw: `["zhangsan"]`,
			TagRuleRaw:   `[{"time_type":1,"tags":[{"tagid":8,"tagname":"新客"}]}]`,
			TagsRaw:      `["新客"]`,
			OnOff:        1,
			CreatedAt:    "2026-07-05 10:00:00",
		},
		stats: AutoTagStatistics{TotalCount: 12, TodayCount: 2},
	}
	handler := NewAutoTagHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/autoTag/show?id=31", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.Show(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Data struct {
			AutoTag    map[string]any `json:"auto_tag"`
			Statistics map[string]any `json:"statistics"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.AutoTag["name"] != "分时段规则" || envelope.Data.Statistics["total_count"].(float64) != 12 || envelope.Data.Statistics["today_count"].(float64) != 2 {
		t.Fatalf("data = %#v", envelope.Data)
	}
}

func TestWorkMessageIndexReturnsMessageList(t *testing.T) {
	store := &fakeAutoTagStore{
		user: User{ID: 1, IsSuperAdmin: 1},
		messagePage: WorkMessagePage{
			Page:    1,
			PerPage: 100,
			Total:   1,
			Items: []WorkMessageItem{{
				ID:            9,
				Action:        0,
				Name:          "张三",
				IsCurrentUser: 1,
				Type:          1,
				ContentRaw:    `{"content":"你好"}`,
				MsgDataTime:   "2026-07-05 11:00:00",
			}},
		},
	}
	handler := NewAutoTagHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/workMessage/index?workEmployeeId=99&toUserType=1&toUserId=31&page=1&perPage=100", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.WorkMessageIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Data struct {
			List []map[string]any `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data.List) != 1 || envelope.Data.List[0]["name"] != "张三" {
		t.Fatalf("list = %#v", envelope.Data.List)
	}
	content := envelope.Data.List[0]["content"].(map[string]any)
	if content["content"] != "你好" {
		t.Fatalf("content = %#v", content)
	}
	if store.lastMessageFilter.WorkEmployeeID != 99 || store.lastMessageFilter.ToUserType != 1 || store.lastMessageFilter.ToUserID != 31 {
		t.Fatalf("message filter = %#v", store.lastMessageFilter)
	}
}

func TestWorkMessageConfigStepCreateReturnsCorpConfig(t *testing.T) {
	store := &fakeAutoTagStore{
		user: User{ID: 1, IsSuperAdmin: 1},
		config: WorkMessageConfigItem{
			ID:                 7,
			CorpID:             7,
			CorpName:           "测试企业",
			WXCorpID:           "wx123",
			ChatApplyStatus:    3,
			ChatWhitelistIPRaw: `["127.0.0.1"]`,
			ChatRSAKeyRaw:      `{"publicKey":"pub","privateKey":"pri"}`,
		},
		configFound: true,
	}
	handler := NewAutoTagHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{HeaderName: "X-Mochat-Go-User-ID"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/workMessageConfig/stepCreate", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.WorkMessageConfigStepCreate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data["corpName"] != "测试企业" || envelope.Data["rsaPublicKey"] != "pub" {
		t.Fatalf("data = %#v", envelope.Data)
	}
}

type fakeAutoTagStore struct {
	user              User
	page              AutoTagPage
	item              AutoTagItem
	stats             AutoTagStatistics
	recordPage        AutoTagRecordPage
	created           AutoTagWrite
	createID          int
	lastFilter        AutoTagFilter
	messagePage       WorkMessagePage
	lastMessageFilter WorkMessageFilter
	toUserPage        WorkMessageToUserPage
	config            WorkMessageConfigItem
	configFound       bool
}

func (s *fakeAutoTagStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	if userID != s.user.ID {
		return User{}, false, nil
	}
	return s.user, true, nil
}

func (s *fakeAutoTagStore) EmployeeIDByUserCorp(context.Context, int, int) (int, error) {
	return 99, nil
}

func (s *fakeAutoTagStore) FirstEmployeeByUser(context.Context, int) (int, int, bool, error) {
	return 7, 99, true, nil
}

func (s *fakeAutoTagStore) AutoTagPage(_ context.Context, filter AutoTagFilter) (AutoTagPage, error) {
	s.lastFilter = filter
	return s.page, nil
}

func (s *fakeAutoTagStore) AutoTagByID(context.Context, int, int) (AutoTagItem, bool, error) {
	if s.item.ID <= 0 {
		return AutoTagItem{}, false, nil
	}
	return s.item, true, nil
}

func (s *fakeAutoTagStore) CreateAutoTag(_ context.Context, values AutoTagWrite) (int, error) {
	s.created = values
	if s.createID > 0 {
		return s.createID, nil
	}
	return 1, nil
}

func (s *fakeAutoTagStore) UpdateAutoTagOnOff(context.Context, int, int, int) (bool, error) {
	return true, nil
}

func (s *fakeAutoTagStore) DeleteAutoTag(context.Context, int, int) (bool, error) {
	return true, nil
}

func (s *fakeAutoTagStore) AutoTagStatistics(context.Context, int, int) (AutoTagStatistics, error) {
	return s.stats, nil
}

func (s *fakeAutoTagStore) AutoTagRecordPage(context.Context, AutoTagRecordFilter) (AutoTagRecordPage, error) {
	return s.recordPage, nil
}

func (s *fakeAutoTagStore) AutoTagKeywordTask(context.Context, int) (AutoTagKeywordTaskResult, error) {
	return AutoTagKeywordTaskResult{RuleCount: 1}, nil
}

func (s *fakeAutoTagStore) WorkMessageFromUsers(context.Context, int, string, int, int) ([]WorkMessageFromUser, error) {
	return []WorkMessageFromUser{{ID: 99, Name: "张三"}}, nil
}

func (s *fakeAutoTagStore) WorkMessageToUsers(context.Context, WorkMessageUserFilter) (WorkMessageToUserPage, error) {
	return s.toUserPage, nil
}

func (s *fakeAutoTagStore) WorkMessagePage(_ context.Context, filter WorkMessageFilter) (WorkMessagePage, error) {
	s.lastMessageFilter = filter
	return s.messagePage, nil
}

func (s *fakeAutoTagStore) WorkMessageConfigByCorp(context.Context, int) (WorkMessageConfigItem, bool, error) {
	return s.config, s.configFound, nil
}

func (s *fakeAutoTagStore) WorkMessageConfigPage(context.Context, int, string, int, int) (WorkMessageConfigPage, error) {
	return WorkMessageConfigPage{Items: []WorkMessageConfigItem{s.config}, Total: 1, Page: 1, PerPage: 10}, nil
}

func (s *fakeAutoTagStore) UpsertWorkMessageCorpConfig(context.Context, int, WorkMessageConfigItem) (int, error) {
	return 7, nil
}

func (s *fakeAutoTagStore) UpdateWorkMessageStepConfig(context.Context, int, WorkMessageConfigItem) (bool, error) {
	return true, nil
}
