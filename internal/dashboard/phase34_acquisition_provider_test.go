package dashboard

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPhase34UnavailableExternalProviderIsExplicitlyFailClosed(t *testing.T) {
	provider := NewPhase34UnavailableExternalProvider()
	if _, err := provider.Authorize(context.Background(), 7); !errors.Is(err, ErrPhase34AcquisitionProviderNotConfigured) {
		t.Fatalf("authorize error=%v", err)
	}
	if err := provider.SyncCustomerService(context.Background(), 7); !errors.Is(err, ErrPhase34AcquisitionProviderNotConfigured) {
		t.Fatalf("sync error=%v", err)
	}
}

func TestPhase34AcquisitionLinkStoreUsesSelectedCorpAndCreatesDraft(t *testing.T) {
	store := &fakePhase34AcquisitionStore{users: map[int]User{1: {ID: 1, Name: "管理员"}}, linkID: 17}
	handler := NewPhase34AcquisitionHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, nil)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/acquisitionLink/store", strings.NewReader(`{"name":"官网落地页","targetUrl":"/customer/contact","corpId":999}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.AcquisitionLinkStore(rec, req)

	if rec.Code != http.StatusOK || store.createdLink.CorpID != 7 || store.createdLink.Status != "draft" || store.createdLink.TargetURL != "/customer/contact" {
		t.Fatalf("status=%d write=%#v body=%s", rec.Code, store.createdLink, rec.Body.String())
	}
}

func TestPhase34AcquisitionLinkAuthorizeFailsClosedWithoutExternalProvider(t *testing.T) {
	store := &fakePhase34AcquisitionStore{users: map[int]User{1: {ID: 1}}}
	handler := NewPhase34AcquisitionHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, nil)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/acquisitionLink/authorize", strings.NewReader(`{}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.AcquisitionLinkAuthorize(rec, req)

	if rec.Code != http.StatusServiceUnavailable || store.authorizationCalls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.authorizationCalls, rec.Body.String())
	}
}

func TestPhase34AcquisitionAuthorizePersistsAuthorizedState(t *testing.T) {
	store := &fakePhase34AcquisitionStore{users: map[int]User{1: {ID: 1}}, linkID: 17}
	external := &fakePhase34AcquisitionExternal{authorizeURL: "https://wecom.example/authorize"}
	handler := NewPhase34AcquisitionHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, external)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/acquisitionLink/authorize", strings.NewReader(`{}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.AcquisitionLinkAuthorize(rec, req)

	if rec.Code != http.StatusOK || store.authorizationStatus != "authorizing" || store.authorizationState != "draft" {
		t.Fatalf("status=%d authorization=%q state=%q body=%s", rec.Code, store.authorizationStatus, store.authorizationState, rec.Body.String())
	}
}

func TestPhase34CustomerServiceSyncPersistsFailureReason(t *testing.T) {
	store := &fakePhase34AcquisitionStore{users: map[int]User{1: {ID: 1}}}
	external := &fakePhase34AcquisitionExternal{syncErr: errors.New("customer-service credentials rejected")}
	handler := NewPhase34AcquisitionHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, external)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/customerService/sync", strings.NewReader(`{}`))
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.CustomerServiceSync(rec, req)

	if rec.Code != http.StatusServiceUnavailable || store.customerServiceStatus != "failed" || store.customerServiceReason != "customer-service credentials rejected" {
		t.Fatalf("status=%d status=%q reason=%q body=%s", rec.Code, store.customerServiceStatus, store.customerServiceReason, rec.Body.String())
	}
}

func TestPhase34CustomerServiceIndexUsesSelectedCorp(t *testing.T) {
	store := &fakePhase34AcquisitionStore{
		users:            map[int]User{1: {ID: 1}},
		customerServices: Phase34CustomerServicePage{Items: []Phase34CustomerService{{ID: 3, Name: "售前客服", Status: "draft"}}, Total: 1, Page: 1, PerPage: 20},
	}
	handler := NewPhase34AcquisitionHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, &recordingAuthorizer{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/customerService/index?name=%E5%94%AE%E5%89%8D", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()

	handler.CustomerServiceIndex(rec, req)

	if rec.Code != http.StatusOK || store.customerServiceFilter.CorpID != 7 || store.customerServiceFilter.Name != "售前" || !strings.Contains(rec.Body.String(), "售前客服") {
		t.Fatalf("status=%d filter=%#v body=%s", rec.Code, store.customerServiceFilter, rec.Body.String())
	}
}

func TestPhase34ShortLinkRedirectRecordsVisitAndRejectsDisabledLink(t *testing.T) {
	store := &fakePhase34AcquisitionStore{shortLink: Phase34ShortLink{ID: 8, CorpID: 7, Token: "abc123", TargetURL: "/acquisition/v2-channel-code", Status: "active"}}
	handler := NewPhase34AcquisitionHandler(store, nil, nil, nil, nil)
	activeReq := httptest.NewRequest(http.MethodGet, "/r/abc123", nil)
	activeRec := httptest.NewRecorder()
	handler.ShortLinkRedirect(activeRec, activeReq)
	if activeRec.Code != http.StatusFound || activeRec.Header().Get("Location") != "/acquisition/v2-channel-code" || store.visitToken != "abc123" {
		t.Fatalf("active status=%d location=%q visit=%q body=%s", activeRec.Code, activeRec.Header().Get("Location"), store.visitToken, activeRec.Body.String())
	}

	store.shortLink.Status = "disabled"
	disabledReq := httptest.NewRequest(http.MethodGet, "/r/abc123", nil)
	disabledRec := httptest.NewRecorder()
	handler.ShortLinkRedirect(disabledRec, disabledReq)
	if disabledRec.Code != http.StatusGone {
		t.Fatalf("disabled status=%d body=%s", disabledRec.Code, disabledRec.Body.String())
	}
}

type fakePhase34AcquisitionStore struct {
	users                 map[int]User
	linkID                int
	createdLink           Phase34AcquisitionLinkWrite
	authorizationCalls    int
	authorizationStatus   string
	authorizationState    string
	customerServices      Phase34CustomerServicePage
	customerServiceFilter Phase34CustomerServiceFilter
	customerServiceStatus string
	customerServiceReason string
	shortLink             Phase34ShortLink
	visitToken            string
}

type fakePhase34AcquisitionExternal struct {
	authorizeURL string
	authorizeErr error
	syncErr      error
}

func (s *fakePhase34AcquisitionExternal) Authorize(context.Context, int) (string, error) {
	return s.authorizeURL, s.authorizeErr
}

func (s *fakePhase34AcquisitionExternal) SyncCustomerService(context.Context, int) error {
	return s.syncErr
}

func (s *fakePhase34AcquisitionStore) UserByID(_ context.Context, id int) (User, bool, error) {
	user, ok := s.users[id]
	return user, ok, nil
}

func (s *fakePhase34AcquisitionStore) EmployeeIDByUserCorp(context.Context, int, int) (int, error) {
	return 99, nil
}

func (s *fakePhase34AcquisitionStore) FirstEmployeeByUser(context.Context, int) (int, int, bool, error) {
	return 7, 99, true, nil
}

func (s *fakePhase34AcquisitionStore) Phase34AcquisitionLinkPage(context.Context, Phase34AcquisitionLinkFilter) (Phase34AcquisitionLinkPage, error) {
	return Phase34AcquisitionLinkPage{}, nil
}

func (s *fakePhase34AcquisitionStore) CreatePhase34AcquisitionLink(_ context.Context, value Phase34AcquisitionLinkWrite) (int, error) {
	s.createdLink = value
	return s.linkID, nil
}

func (s *fakePhase34AcquisitionStore) Phase34AcquisitionLinkByID(context.Context, int, int) (Phase34AcquisitionLink, bool, error) {
	return Phase34AcquisitionLink{}, false, nil
}

func (s *fakePhase34AcquisitionStore) DisablePhase34AcquisitionLink(context.Context, int, int) (bool, error) {
	return true, nil
}

func (s *fakePhase34AcquisitionStore) UpdatePhase34AcquisitionAuthorizationState(_ context.Context, _ int, authorizationStatus string, state string) error {
	s.authorizationStatus = authorizationStatus
	s.authorizationState = state
	return nil
}

func (s *fakePhase34AcquisitionStore) Phase34CustomerServicePage(_ context.Context, filter Phase34CustomerServiceFilter) (Phase34CustomerServicePage, error) {
	s.customerServiceFilter = filter
	return s.customerServices, nil
}

func (s *fakePhase34AcquisitionStore) CreatePhase34CustomerService(context.Context, Phase34CustomerServiceWrite) (int, error) {
	return 1, nil
}

func (s *fakePhase34AcquisitionStore) Phase34CustomerServiceByID(context.Context, int, int) (Phase34CustomerService, bool, error) {
	return Phase34CustomerService{}, false, nil
}

func (s *fakePhase34AcquisitionStore) UpdatePhase34CustomerServiceSyncState(_ context.Context, _ int, state string, reason string) error {
	s.customerServiceStatus = state
	s.customerServiceReason = reason
	return nil
}

func (s *fakePhase34AcquisitionStore) Phase34ShortLinkPage(context.Context, Phase34ShortLinkFilter) (Phase34ShortLinkPage, error) {
	return Phase34ShortLinkPage{}, nil
}

func (s *fakePhase34AcquisitionStore) CreatePhase34ShortLink(context.Context, Phase34ShortLinkWrite) (Phase34ShortLink, error) {
	return s.shortLink, nil
}

func (s *fakePhase34AcquisitionStore) Phase34ShortLinkByToken(context.Context, string) (Phase34ShortLink, bool, error) {
	return s.shortLink, s.shortLink.Token != "", nil
}

func (s *fakePhase34AcquisitionStore) RecordPhase34ShortLinkVisit(_ context.Context, _ int, token string, _ string, _ string) error {
	s.visitToken = token
	return nil
}

func (s *fakePhase34AcquisitionStore) DisablePhase34ShortLink(context.Context, int, int) (bool, error) {
	return true, nil
}
