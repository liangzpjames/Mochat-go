package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type principalContractAutoTagStore struct {
	*fakeAutoTagStore
	userCalls int
}

func (s *principalContractAutoTagStore) UserByID(ctx context.Context, userID int) (User, bool, error) {
	s.userCalls++
	return s.fakeAutoTagStore.UserByID(ctx, userID)
}

type principalContractWorkFissionStore struct {
	*fakeWorkFissionStore
	userCalls int
}

func (s *principalContractWorkFissionStore) UserByID(ctx context.Context, userID int) (User, bool, error) {
	s.userCalls++
	return s.fakeWorkFissionStore.UserByID(ctx, userID)
}

type principalContractRoomMessageBatchSendStore struct {
	*fakeRoomMessageBatchSendStore
	userCalls int
}

func (s *principalContractRoomMessageBatchSendStore) UserByID(ctx context.Context, userID int) (User, bool, error) {
	s.userCalls++
	return s.fakeRoomMessageBatchSendStore.UserByID(ctx, userID)
}

type principalContractAuthorizer struct {
	calls int
}

func (a *principalContractAuthorizer) Resolve(context.Context, int, string, int, int) (AccessContext, error) {
	a.calls++
	return AccessContext{}, nil
}

func TestRepresentativeDashboardHandlersRejectMissingPrincipalBeforeDependencies(t *testing.T) {
	tests := []struct {
		name    string
		request *http.Request
		invoke  func(http.ResponseWriter, *http.Request, *principalContractAuthorizer)
	}{
		{
			name:    "marketing",
			request: httptest.NewRequest(http.MethodGet, "/dashboard/autoTag/index", nil),
			invoke: func(w http.ResponseWriter, r *http.Request, authorizer *principalContractAuthorizer) {
				store := &principalContractAutoTagStore{fakeAutoTagStore: &fakeAutoTagStore{}}
				NewAutoTagHandler(store, nil, nil, authorizer).Index(w, r)
				if store.userCalls != 0 {
					t.Fatalf("marketing user calls = %d, want 0", store.userCalls)
				}
			},
		},
		{
			name:    "employee",
			request: httptest.NewRequest(http.MethodGet, "/dashboard/workFission/index", nil),
			invoke: func(w http.ResponseWriter, r *http.Request, authorizer *principalContractAuthorizer) {
				store := &principalContractWorkFissionStore{fakeWorkFissionStore: &fakeWorkFissionStore{}}
				NewWorkFissionHandler(store, nil, nil, authorizer, "", "").Index(w, r)
				if store.userCalls != 0 {
					t.Fatalf("employee user calls = %d, want 0", store.userCalls)
				}
			},
		},
		{
			name:    "group chat",
			request: httptest.NewRequest(http.MethodPost, "/dashboard/roomMessageBatchSend/store", strings.NewReader(`{}`)),
			invoke: func(w http.ResponseWriter, r *http.Request, authorizer *principalContractAuthorizer) {
				store := &principalContractRoomMessageBatchSendStore{fakeRoomMessageBatchSendStore: &fakeRoomMessageBatchSendStore{}}
				NewRoomMessageBatchSendHandler(store, nil, nil, authorizer, "", t.TempDir(), nil).Store(w, r)
				if store.userCalls != 0 {
					t.Fatalf("group chat user calls = %d, want 0", store.userCalls)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			authorizer := &principalContractAuthorizer{}
			tt.invoke(response, tt.request, authorizer)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, body = %s, want 401", response.Code, response.Body.String())
			}
			if authorizer.calls != 0 {
				t.Fatalf("authorizer calls = %d, want 0", authorizer.calls)
			}
		})
	}
}
