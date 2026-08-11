package dashboardauth

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeDashboardIdentityStore struct {
	authenticate func(context.Context, string) (DashboardIdentity, error)
	activate     func(context.Context, [32]byte, string) error
	checkSession func(context.Context, int, uint64) error
}

func (store *fakeDashboardIdentityStore) Authenticate(ctx context.Context, loginIdentifier string) (DashboardIdentity, error) {
	return store.authenticate(ctx, loginIdentifier)
}

func (store *fakeDashboardIdentityStore) Activate(ctx context.Context, tokenDigest [32]byte, passwordHash string) error {
	return store.activate(ctx, tokenDigest, passwordHash)
}

func (store *fakeDashboardIdentityStore) CheckSession(ctx context.Context, userID int, authVersion uint64) error {
	return store.checkSession(ctx, userID, authVersion)
}

func TestDashboardAuthUsesUniformCredentialErrorsAndDoesNotReturnPasswordHash(t *testing.T) {
	password := "dashboard-password-not-output"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name     string
		identity DashboardIdentity
		err      error
	}{
		{name: "unknown", err: ErrIdentityNotFound},
		{name: "wrong password", identity: DashboardIdentity{UserID: 7, LoginIdentifier: "13800000000", PasswordHash: hash, Status: DashboardIdentityStatusActive}},
		{name: "disabled", identity: DashboardIdentity{UserID: 7, LoginIdentifier: "13800000000", PasswordHash: hash, Status: DashboardIdentityStatusDisabled}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeDashboardIdentityStore{
				authenticate: func(context.Context, string) (DashboardIdentity, error) {
					return tc.identity, tc.err
				},
			}
			attempt := password
			if tc.name == "wrong password" {
				attempt = "another-dashboard-password"
			}
			_, err := NewService(store).Authenticate(context.Background(), "13800000000", attempt)
			if !errors.Is(err, ErrInvalidCredentials) {
				t.Fatalf("authentication did not fail with the uniform credential error: %v", err)
			}
			if strings.Contains(err.Error(), password) || strings.Contains(err.Error(), attempt) {
				t.Fatal("authentication error exposed password material")
			}
		})
	}

	store := &fakeDashboardIdentityStore{
		authenticate: func(context.Context, string) (DashboardIdentity, error) {
			return DashboardIdentity{UserID: 7, LoginIdentifier: "13800000000", PasswordHash: hash, Status: DashboardIdentityStatusActive, AuthVersion: 5}, nil
		},
	}
	got, err := NewService(store).Authenticate(context.Background(), "13800000000", password)
	if err != nil {
		t.Fatal(err)
	}
	if got.UserID != 7 || got.AuthVersion != 5 || got.PasswordHash != "" {
		t.Fatal("successful authentication returned an unsafe identity projection")
	}
}

func TestDashboardAuthHashesActivationPasswordBeforeStore(t *testing.T) {
	password := "activation-password-not-output"
	var receivedHash string
	store := &fakeDashboardIdentityStore{
		activate: func(_ context.Context, _ [32]byte, passwordHash string) error {
			receivedHash = passwordHash
			return nil
		},
	}
	if err := NewService(store).Activate(context.Background(), [32]byte{1, 2, 3}, password); err != nil {
		t.Fatal(err)
	}
	if receivedHash == "" || receivedHash == password || !VerifyPassword(receivedHash, password) {
		t.Fatal("activation did not pass a verifiable password hash to the store")
	}
}

func TestDashboardAuthCheckSessionDelegatesIdentityVersionValidation(t *testing.T) {
	called := false
	store := &fakeDashboardIdentityStore{
		checkSession: func(_ context.Context, userID int, authVersion uint64) error {
			called = userID == 7 && authVersion == 6
			return nil
		},
	}
	if err := NewService(store).CheckSession(context.Background(), 7, 6); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("session check did not receive the authenticated identity version")
	}
}
