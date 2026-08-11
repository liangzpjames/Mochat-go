package saasauth

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeSaaSIdentityStore struct {
	authenticate func(context.Context, string) (SaaSIdentity, error)
	bootstrap    func(context.Context, BootstrapSaaSAdmin) (SaaSIdentity, error)
	checkSession func(context.Context, int, uint64) error
}

func (store *fakeSaaSIdentityStore) Authenticate(ctx context.Context, login string) (SaaSIdentity, error) {
	return store.authenticate(ctx, login)
}

func (store *fakeSaaSIdentityStore) Bootstrap(ctx context.Context, input BootstrapSaaSAdmin) (SaaSIdentity, error) {
	return store.bootstrap(ctx, input)
}

func (store *fakeSaaSIdentityStore) CheckSession(ctx context.Context, userID int, authVersion uint64) error {
	return store.checkSession(ctx, userID, authVersion)
}

func TestSaaSAuthUsesUniformCredentialErrorsAndDoesNotReturnPasswordHash(t *testing.T) {
	password := "unit-password-not-output"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name     string
		identity SaaSIdentity
		err      error
	}{
		{name: "unknown", err: ErrIdentityNotFound},
		{name: "wrong password", identity: SaaSIdentity{ID: 7, LoginName: "admin", PasswordHash: hash, Status: SaaSIdentityStatusActive}},
		{name: "disabled", identity: SaaSIdentity{ID: 7, LoginName: "admin", PasswordHash: hash, Status: SaaSIdentityStatusDisabled}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeSaaSIdentityStore{
				authenticate: func(context.Context, string) (SaaSIdentity, error) {
					return tc.identity, tc.err
				},
			}
			attempt := password
			if tc.name == "wrong password" {
				attempt = "another-unit-password"
			}
			_, err := NewService(store).Authenticate(context.Background(), "admin", attempt)
			if !errors.Is(err, ErrInvalidCredentials) {
				t.Fatalf("authentication did not fail with the uniform credential error: %v", err)
			}
			if strings.Contains(err.Error(), password) || strings.Contains(err.Error(), attempt) {
				t.Fatal("authentication error exposed password material")
			}
		})
	}

	store := &fakeSaaSIdentityStore{
		authenticate: func(context.Context, string) (SaaSIdentity, error) {
			return SaaSIdentity{ID: 7, LoginName: "admin", PasswordHash: hash, Status: SaaSIdentityStatusActive, AuthVersion: 3}, nil
		},
	}
	got, err := NewService(store).Authenticate(context.Background(), "admin", password)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 7 || got.AuthVersion != 3 || got.PasswordHash != "" {
		t.Fatal("successful authentication returned an unsafe identity projection")
	}
}

func TestSaaSAuthBootstrapRequiresHashedPasswordAndSanitizesResult(t *testing.T) {
	password := "bootstrap-password-not-output"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	var received BootstrapSaaSAdmin
	store := &fakeSaaSIdentityStore{
		bootstrap: func(_ context.Context, input BootstrapSaaSAdmin) (SaaSIdentity, error) {
			received = input
			return SaaSIdentity{ID: 9, LoginName: input.LoginName, PasswordHash: input.PasswordHash, AuthVersion: 1}, nil
		},
	}

	got, err := NewService(store).Bootstrap(context.Background(), BootstrapSaaSAdmin{
		RequestKey:   "bootstrap-request-1",
		LoginName:    "platform-admin",
		Phone:        "13800000000",
		Name:         "Platform Admin",
		PasswordHash: hash,
	})
	if err != nil {
		t.Fatal(err)
	}
	if received.PasswordHash != hash || received.LoginName != "platform-admin" || received.RequestKey != "bootstrap-request-1" {
		t.Fatal("bootstrap input contract was not preserved")
	}
	if got.ID != 9 || got.PasswordHash != "" {
		t.Fatal("bootstrap returned password material")
	}

	if _, err := NewService(store).Bootstrap(context.Background(), BootstrapSaaSAdmin{LoginName: "platform-admin"}); !errors.Is(err, ErrInvalidBootstrap) {
		t.Fatal("bootstrap accepted missing hashed password or request key")
	}
}

func TestSaaSAuthBootstrapSanitizesStoreErrors(t *testing.T) {
	hash, err := HashPassword("bootstrap-error-password-not-output")
	if err != nil {
		t.Fatal(err)
	}
	store := &fakeSaaSIdentityStore{
		bootstrap: func(context.Context, BootstrapSaaSAdmin) (SaaSIdentity, error) {
			return SaaSIdentity{}, errors.New("store failure: " + hash)
		},
	}
	_, err = NewService(store).Bootstrap(context.Background(), BootstrapSaaSAdmin{
		RequestKey:   "bootstrap-error-request",
		LoginName:    "platform-admin",
		Name:         "Platform Admin",
		PasswordHash: hash,
	})
	if !errors.Is(err, ErrIdentityUnavailable) || strings.Contains(err.Error(), hash) {
		t.Fatal("bootstrap returned a store error containing password material")
	}
}

func TestSaaSAuthBootstrapPreservesConflictWithoutPasswordMaterial(t *testing.T) {
	hash, err := HashPassword("bootstrap-conflict-password-not-output")
	if err != nil {
		t.Fatal(err)
	}
	store := &fakeSaaSIdentityStore{
		bootstrap: func(context.Context, BootstrapSaaSAdmin) (SaaSIdentity, error) {
			return SaaSIdentity{}, ErrBootstrapConflict
		},
	}
	_, err = NewService(store).Bootstrap(context.Background(), BootstrapSaaSAdmin{
		RequestKey:   "bootstrap-conflict-request",
		LoginName:    "platform-admin",
		Name:         "Platform Admin",
		PasswordHash: hash,
	})
	if !errors.Is(err, ErrBootstrapConflict) || strings.Contains(err.Error(), hash) {
		t.Fatalf("bootstrap conflict was not preserved safely: %v", err)
	}
}

func TestSaaSAuthCheckSessionDelegatesIdentityVersionValidation(t *testing.T) {
	called := false
	store := &fakeSaaSIdentityStore{
		checkSession: func(_ context.Context, userID int, authVersion uint64) error {
			called = userID == 7 && authVersion == 4
			return nil
		},
	}
	if err := NewService(store).CheckSession(context.Background(), 7, 4); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("session check did not receive the authenticated identity version")
	}
}
