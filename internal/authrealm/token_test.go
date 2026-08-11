package authrealm

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRealmTokensAreRejectedByTheOtherParser(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	saasConfig := testTokenConfig(RealmSaaSAdmin, "saas")
	dashboardConfig := testTokenConfig(RealmDashboard, "dashboard")

	saasToken, err := Sign(saasConfig, Claims{UserID: 7, AuthVersion: 3}, now)
	if err != nil {
		t.Fatal(err)
	}
	dashboardToken, err := Sign(dashboardConfig, Claims{UserID: 9, AuthVersion: 4}, now)
	if err != nil {
		t.Fatal(err)
	}

	_, err = (Parser{Config: dashboardConfig, Now: func() time.Time { return now }, ValidateSession: allowSession}).Parse(context.Background(), saasToken)
	assertUnauthorized(t, err)
	_, err = (Parser{Config: saasConfig, Now: func() time.Time { return now }, ValidateSession: allowSession}).Parse(context.Background(), dashboardToken)
	assertUnauthorized(t, err)
}

func TestParserRejectsInvalidClaimsWithStableUnauthorizedErrors(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	config := testTokenConfig(RealmDashboard, "dashboard")
	parser := Parser{Config: config, Now: func() time.Time { return now }, ValidateSession: allowSession}

	cases := []struct {
		name   string
		config TokenConfig
		claims Claims
	}{
		{
			name:   "wrong secret",
			config: testTokenConfig(RealmDashboard, "other-secret"),
			claims: Claims{UserID: 7, AuthVersion: 1},
		},
		{
			name:   "wrong issuer",
			config: TokenConfig{Secret: config.Secret, Issuer: "wrong-issuer", Audience: config.Audience, TTL: config.TTL, Realm: config.Realm},
			claims: Claims{UserID: 7, AuthVersion: 1},
		},
		{
			name:   "wrong audience",
			config: TokenConfig{Secret: config.Secret, Issuer: config.Issuer, Audience: "wrong-audience", TTL: config.TTL, Realm: config.Realm},
			claims: Claims{UserID: 7, AuthVersion: 1},
		},
		{
			name:   "wrong realm",
			config: testTokenConfig(RealmSaaSAdmin, "dashboard"),
			claims: Claims{UserID: 7, AuthVersion: 1},
		},
		{
			name:   "expired",
			config: config,
			claims: Claims{UserID: 7, AuthVersion: 1, ExpiresAt: now.Add(-time.Second).Unix(), NotBefore: now.Add(-time.Hour).Unix()},
		},
		{
			name:   "future nbf",
			config: config,
			claims: Claims{UserID: 7, AuthVersion: 1, NotBefore: now.Add(time.Minute).Unix()},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			token, err := Sign(tc.config, tc.claims, now)
			if err != nil {
				t.Fatal(err)
			}
			_, err = parser.Parse(context.Background(), token)
			assertUnauthorized(t, err)
		})
	}
}

func TestParserRejectsOldAuthVersion(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	config := testTokenConfig(RealmDashboard, "dashboard")
	token, err := Sign(config, Claims{UserID: 7, AuthVersion: 3}, now)
	if err != nil {
		t.Fatal(err)
	}

	parser := Parser{Config: config, Now: func() time.Time { return now }, ValidateSession: allowSession}
	_, err = parser.ParseWithAuthVersion(context.Background(), token, 4)
	assertUnauthorized(t, err)
	if !errors.Is(err, ErrAuthVersionMismatch) {
		t.Fatalf("err = %v, want auth version mismatch", err)
	}
}

func TestParserRejectsTokensWhenSessionValidatorIsMissing(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	config := testTokenConfig(RealmDashboard, "dashboard")
	token, err := Sign(config, Claims{UserID: 7, AuthVersion: 3}, now)
	if err != nil {
		t.Fatal(err)
	}

	_, err = (Parser{Config: config, Now: func() time.Time { return now }}).Parse(context.Background(), token)
	assertUnauthorized(t, err)
	if !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("err = %v, want missing validator to fail as session invalid", err)
	}
}

func TestParserPassesClaimsToSessionValidatorForAuthVersionValidation(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	config := testTokenConfig(RealmDashboard, "dashboard")
	token, err := Sign(config, Claims{UserID: 7, AuthVersion: 3}, now)
	if err != nil {
		t.Fatal(err)
	}

	called := false
	parser := Parser{
		Config: config,
		Now:    func() time.Time { return now },
		ValidateSession: func(_ context.Context, claims Claims) error {
			called = true
			if claims.UserID != 7 || claims.AuthVersion != 3 {
				return ErrAuthVersionMismatch
			}
			return nil
		},
	}
	if _, err := parser.Parse(context.Background(), token); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("session validator was not called")
	}
}

func TestParserValidatesSessionWithoutExposingSessionDetails(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	config := testTokenConfig(RealmSaaSAdmin, "saas")
	token, err := Sign(config, Claims{UserID: 7, AuthVersion: 3}, now)
	if err != nil {
		t.Fatal(err)
	}

	secret := string(config.Secret)
	parser := Parser{
		Config: config,
		Now:    func() time.Time { return now },
		ValidateSession: func(context.Context, Claims) error {
			return errors.New("backend detail " + secret)
		},
	}
	_, err = parser.Parse(context.Background(), token)
	assertUnauthorized(t, err)
	if !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("err = %v, want session invalid", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatal("authentication error exposed secret")
	}
}

func TestSignerAndParserRoundTripClaims(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	config := testTokenConfig(RealmDashboard, "dashboard")
	want := Claims{UserID: 8, AuthVersion: 11}

	token, err := Sign(config, want, now)
	if err != nil {
		t.Fatal(err)
	}
	got, err := (Parser{Config: config, Now: func() time.Time { return now }, ValidateSession: allowSession}).Parse(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	if got.UserID != want.UserID || got.AuthVersion != want.AuthVersion {
		t.Fatalf("claims = %+v, want user/auth_version from %+v", got, want)
	}
	if got.Subject != "dashboard-user:8" || got.Realm != RealmDashboard || got.Issuer != config.Issuer || got.Audience != config.Audience || got.JWTID == "" {
		t.Fatalf("derived claims = %+v", got)
	}
}

func TestDashboardClaimsDoNotExposeBusinessBindingFields(t *testing.T) {
	typeOfClaims := reflect.TypeOf(Claims{})
	for _, field := range []string{"TenantID", "CorpID", "IsSuperAdmin"} {
		if _, ok := typeOfClaims.FieldByName(field); ok {
			t.Fatalf("Claims unexpectedly contains business field %s", field)
		}
	}
}

func TestTokenConfigRejectsMissingIdentityBoundary(t *testing.T) {
	cases := []TokenConfig{
		{Issuer: "issuer", Audience: "audience", TTL: time.Hour, Realm: RealmSaaSAdmin},
		{Secret: []byte("secret"), Audience: "audience", TTL: time.Hour, Realm: RealmSaaSAdmin},
		{Secret: []byte("secret"), Issuer: "issuer", TTL: time.Hour, Realm: RealmSaaSAdmin},
		{Secret: []byte("secret"), Issuer: "issuer", Audience: "audience", TTL: time.Hour},
	}
	for index, config := range cases {
		if err := config.Validate(); err == nil {
			t.Fatalf("config case %d unexpectedly validated", index)
		}
	}
}

func assertUnauthorized(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected authentication failure")
	}
	var status interface{ StatusCode() int }
	if !errors.As(err, &status) || status.StatusCode() != 401 {
		t.Fatalf("err = %v, want stable HTTP 401", err)
	}
}

func allowSession(context.Context, Claims) error { return nil }

func testTokenConfig(realm Realm, secret string) TokenConfig {
	issuer := "mochat-go/saas-auth"
	audience := "mochat-saas-admin"
	if realm == RealmDashboard {
		issuer = "mochat-go/dashboard-auth"
		audience = "mochat-dashboard"
	}
	return TokenConfig{
		Secret:   []byte(strings.Repeat(secret, 4)),
		Issuer:   issuer,
		Audience: audience,
		TTL:      24 * time.Hour,
		Realm:    realm,
	}
}
