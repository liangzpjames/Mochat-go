package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"jiyi/mochat-go/internal/authrealm"
	"jiyi/mochat-go/internal/dashboardauth"
)

var _ dashboardauth.DashboardIdentityStore = (*DashboardIdentityStore)(nil)

var _ dashboardauth.DashboardAuthPersistence = (*DashboardIdentityStore)(nil)

func TestDashboardIdentityStorePasswordQueryUsesOnlyDashboardIdentityTable(t *testing.T) {
	var query string
	store := &DashboardIdentityStore{
		queryRow: func(_ context.Context, statement string, _ ...any) identityRowScanner {
			query = statement
			return identityTestRow{values: []any{7, "13800000000", "bcrypt-hash", 1, 0, uint64(4), 0}}
		},
	}
	identity, err := store.Authenticate(context.Background(), "13800000000")
	if err != nil {
		t.Fatal(err)
	}
	if identity.UserID != 7 || identity.AuthVersion != 4 || identity.PasswordHash != "bcrypt-hash" {
		t.Fatal("Dashboard identity row was not loaded")
	}
	if !strings.Contains(query, "FROM mochat_go_dashboard_identities") || strings.Contains(query, "mc_user") || strings.Contains(query, "deleted_at") {
		t.Fatalf("Dashboard password query crossed an identity boundary: %s", query)
	}
}

func TestDashboardIdentityStoreCheckSessionChecksStatusAndAuthVersion(t *testing.T) {
	cases := []struct {
		name   string
		row    identityTestRow
		wantOK bool
	}{
		{name: "active current version", row: identityTestRow{values: []any{1, uint64(4)}}, wantOK: true},
		{name: "disabled", row: identityTestRow{values: []any{2, uint64(4)}}},
		{name: "stale version", row: identityTestRow{values: []any{1, uint64(5)}}},
		{name: "missing", row: identityTestRow{err: sql.ErrNoRows}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var query string
			store := &DashboardIdentityStore{
				queryRow: func(_ context.Context, statement string, _ ...any) identityRowScanner {
					query = statement
					return tc.row
				},
			}
			err := store.CheckSession(context.Background(), 7, 4)
			if (err == nil) != tc.wantOK {
				t.Fatalf("CheckSession error state = %v, wantOK=%v", err, tc.wantOK)
			}
			if !strings.Contains(query, "status") || !strings.Contains(query, "auth_version") || !strings.Contains(query, "mochat_go_dashboard_identities") || strings.Contains(query, "deleted_at") {
				t.Fatal("CheckSession did not query status and auth_version from the Dashboard identity table")
			}
		})
	}
}

func TestDashboardIdentityStoreActivationConsumesDigestAndUpdatesOnlyIdentityState(t *testing.T) {
	tx := &identityTestTx{}
	tx.query = func(query string, args ...any) identityRowScanner {
		if !strings.Contains(query, "mochat_go_dashboard_identity_activations") || len(args) != 1 {
			t.Fatal("activation lookup did not use the activation digest table")
		}
		return identityTestRow{values: []any{7}}
	}
	execCount := 0
	tx.exec = func(query string, _ ...any) (sql.Result, error) {
		execCount++
		if execCount == 1 && (!strings.Contains(query, "UPDATE mochat_go_dashboard_identities") || strings.Contains(query, "mc_user")) {
			t.Fatal("activation did not update the Dashboard identity table")
		}
		if execCount == 2 && !strings.Contains(query, "mochat_go_dashboard_identity_activations") {
			t.Fatal("activation did not consume the activation digest")
		}
		return identityTestResult{}, nil
	}
	store := &DashboardIdentityStore{begin: func(context.Context) (dashboardIdentityTx, error) { return tx, nil }}
	if err := store.Activate(context.Background(), [32]byte{1, 2, 3}, "bcrypt-hash"); err != nil {
		t.Fatal(err)
	}
	if execCount != 2 || tx.commits != 1 {
		t.Fatal("activation did not atomically update identity and consume the digest")
	}
}

func TestDashboardActivationStatusMapsAllStatesWithoutRawToken(t *testing.T) {
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		activation identityTestRow
		identity   identityTestRow
		tenant     identityTestRow
		want       dashboardauth.ActivationStatusValue
	}{
		{name: "valid", activation: identityTestRow{values: []any{7, "13800138000", now.Add(time.Hour), sql.NullTime{}, sql.NullTime{}, 1}}, identity: identityTestRow{values: []any{7, 9, 0, 1, uint64(2)}}, tenant: identityTestRow{values: []any{"安全租户", 1}}, want: dashboardauth.ActivationStatusValid},
		{name: "expired", activation: identityTestRow{values: []any{7, "13800138000", now.Add(-time.Second), sql.NullTime{}, sql.NullTime{}, 1}}, identity: identityTestRow{values: []any{7, 9, 0, 1, uint64(2)}}, tenant: identityTestRow{values: []any{"安全租户", 1}}, want: dashboardauth.ActivationStatusExpired},
		{name: "activated", activation: identityTestRow{values: []any{7, "13800138000", now.Add(time.Hour), sql.NullTime{Time: now, Valid: true}, sql.NullTime{Time: now, Valid: true}, 1}}, identity: identityTestRow{values: []any{7, 9, 0, 1, uint64(2)}}, tenant: identityTestRow{values: []any{"安全租户", 1}}, want: dashboardauth.ActivationStatusActivated},
		{name: "revoked by resend", activation: identityTestRow{values: []any{7, "13800138000", now.Add(time.Hour), sql.NullTime{Time: now, Valid: true}, sql.NullTime{}, 1}}, identity: identityTestRow{values: []any{7, 9, 0, 1, uint64(2)}}, tenant: identityTestRow{values: []any{"安全租户", 1}}, want: dashboardauth.ActivationStatusRevoked},
		{name: "suspended tenant", activation: identityTestRow{values: []any{7, "13800138000", now.Add(time.Hour), sql.NullTime{}, sql.NullTime{}, 1}}, identity: identityTestRow{values: []any{7, 9, 0, 1, uint64(2)}}, tenant: identityTestRow{values: []any{"安全租户", 2}}, want: dashboardauth.ActivationStatusRevoked},
		{name: "invalid", activation: identityTestRow{err: sql.ErrNoRows}, want: dashboardauth.ActivationStatusInvalid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var gotArg any
			store := &DashboardIdentityStore{queryRow: func(_ context.Context, query string, args ...any) identityRowScanner {
				switch {
				case strings.Contains(query, "mochat_go_dashboard_identity_activations"):
					if strings.Contains(query, "mc_user") || !strings.Contains(query, "token_digest = ?") || strings.Contains(query, "token = ?") || len(args) != 1 {
						t.Fatalf("unsafe activation projection query: %s args=%d", query, len(args))
					}
					gotArg = args[0]
					return test.activation
				case strings.Contains(query, "FROM mochat_go_dashboard_identities d"):
					return test.identity
				case strings.Contains(query, "FROM mc_tenant"):
					return test.tenant
				default:
					t.Fatalf("unexpected activation status query: %s", query)
					return identityTestRow{err: sql.ErrNoRows}
				}
			}}
			token := "raw-activation-token-never-query-or-output"
			status, err := store.DashboardActivationStatus(context.Background(), sha256.Sum256([]byte(token)), now)
			if err != nil {
				t.Fatal(err)
			}
			if status.Status != test.want {
				t.Fatalf("status=%q want=%q", status.Status, test.want)
			}
			if test.want != dashboardauth.ActivationStatusInvalid && string(gotArg.([]byte)) == token {
				t.Fatal("raw activation token reached the store query")
			}
			if test.want != dashboardauth.ActivationStatusInvalid && status.AccountHint != "138****8000" {
				t.Fatalf("unsafe account hint %q", status.AccountHint)
			}
			if strings.Contains(fmt.Sprintf("%+v", status), token) {
				t.Fatal("status contains raw token")
			}
		})
	}
}

func TestDashboardActivationStatusOnlyMapsUnavailableIdentityToRevoked(t *testing.T) {
	now := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)
	databaseErr := errors.New("controlled identity database failure")
	for _, test := range []struct {
		name        string
		identityErr error
		wantRevoked bool
	}{
		{name: "not found is revoked", identityErr: sql.ErrNoRows, wantRevoked: true},
		{name: "database failure propagates", identityErr: databaseErr},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := &DashboardIdentityStore{queryRow: func(_ context.Context, query string, _ ...any) identityRowScanner {
				switch {
				case strings.Contains(query, "mochat_go_dashboard_identity_activations"):
					return identityTestRow{values: []any{7, "13800138000", now.Add(time.Hour), sql.NullTime{}, sql.NullTime{}, 1}}
				case strings.Contains(query, "FROM mochat_go_dashboard_identities d"):
					return identityTestRow{err: test.identityErr}
				default:
					return identityTestRow{err: sql.ErrNoRows}
				}
			}}
			status, err := store.DashboardActivationStatus(context.Background(), [32]byte{1}, now)
			if test.wantRevoked {
				if err != nil || status.Status != dashboardauth.ActivationStatusRevoked {
					t.Fatalf("status=%+v err=%v", status, err)
				}
				return
			}
			if !errors.Is(err, databaseErr) {
				t.Fatalf("error=%v want database failure", err)
			}
		})
	}
}

func TestResolveDashboardIdentityPropagatesDatabaseScanErrors(t *testing.T) {
	databaseErr := errors.New("controlled identity scan failure")
	store := &DashboardIdentityStore{queryRow: func(context.Context, string, ...any) identityRowScanner {
		return identityTestRow{err: databaseErr}
	}}
	if _, err := store.ResolveIdentity(context.Background(), 7); !errors.Is(err, databaseErr) {
		t.Fatalf("ResolveIdentity error=%v want database failure", err)
	}
}

func TestDashboardIdentityStoreCompletesEnrollmentWithPendingToActiveTransition(t *testing.T) {
	now := time.Now().UTC()
	tx := &identityTestTx{}
	tx.query = func(query string, _ ...any) identityRowScanner {
		switch {
		case strings.Contains(query, "mochat_go_dashboard_mfa_challenges"):
			return identityTestRow{values: []any{7, uint64(4), dashboardauth.DashboardMFAChallengeEnrollment, dashboardauth.DashboardMFAStatusPending, 0, 5, now.Add(time.Minute), "ciphertext", "dashboard-key"}}
		case strings.Contains(query, "mochat_go_dashboard_identities"):
			return identityTestRow{values: []any{7, "13800000000", "hash", 1, 0, uint64(4), 1}}
		default:
			return identityTestRow{err: sql.ErrNoRows}
		}
	}
	tx.exec = func(query string, _ ...any) (sql.Result, error) {
		if strings.Contains(query, "mochat_go_dashboard_mfa_credentials") {
			lower := strings.ToLower(query)
			if !strings.Contains(lower, "set status = 1") || !strings.Contains(lower, "where user_id = ? and status = 0") {
				t.Fatalf("enrollment completion did not transition pending credential to active: %s", query)
			}
		}
		if strings.Contains(query, "mochat_go_dashboard_mfa_challenges") && !strings.Contains(strings.ToLower(query), "consumed_at") {
			t.Fatalf("enrollment completion did not consume challenge: %s", query)
		}
		return identityTestResult{}, nil
	}
	store := &DashboardIdentityStore{begin: func(context.Context) (dashboardIdentityTx, error) { return tx, nil }}
	identity, err := store.CompleteMFAChallenge(context.Background(), [32]byte{1}, 7, 4, dashboardauth.DashboardMFAChallengeEnrollment, 123)
	if err != nil {
		t.Fatal(err)
	}
	if identity.UserID != 7 || identity.MFARequired != 1 || tx.commits != 1 {
		t.Fatalf("identity=%+v commits=%d", identity, tx.commits)
	}
}

func TestDashboardIdentityStoreChecksDurableJTIAndAuthVersion(t *testing.T) {
	var query string
	store := &DashboardIdentityStore{
		queryRow: func(_ context.Context, statement string, _ ...any) identityRowScanner {
			query = statement
			return identityTestRow{values: []any{1, uint64(4), 1, uint64(4)}}
		},
	}
	err := store.CheckSessionToken(context.Background(), authrealm.Claims{UserID: 7, AuthVersion: 4, JWTID: "jti"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(query, "mochat_go_dashboard_sessions") || !strings.Contains(query, "jti_digest") || !strings.Contains(query, "auth_version") || strings.Contains(query, "mochat_go_saas_admin_sessions") {
		t.Fatalf("Dashboard session query is not durable and realm-specific: %s", query)
	}
}

func TestDashboardIdentityStoreCreatesFirstLoginPasswordChangeChallenge(t *testing.T) {
	tx := &identityTestTx{}
	tx.query = func(query string, _ ...any) identityRowScanner {
		if !strings.Contains(query, "mochat_go_dashboard_identities") {
			t.Fatalf("password change challenge did not lock the Dashboard identity: %s", query)
		}
		return identityTestRow{values: []any{dashboardauth.DashboardIdentityStatusActive, uint64(4)}}
	}
	var insertedType string
	tx.exec = func(query string, args ...any) (sql.Result, error) {
		if !strings.Contains(query, "mochat_go_dashboard_mfa_challenges") || len(args) < 4 {
			t.Fatalf("password change challenge was not persisted: %s args=%d", query, len(args))
		}
		insertedType, _ = args[3].(string)
		return identityTestResult{}, nil
	}
	store := &DashboardIdentityStore{begin: func(context.Context) (dashboardIdentityTx, error) { return tx, nil }}
	err := store.CreateMFAChallenge(
		context.Background(),
		7,
		4,
		dashboardauth.DashboardMFAChallengePasswordChange,
		[32]byte{1},
		time.Now().UTC().Add(time.Minute),
	)
	if err != nil {
		t.Fatalf("first-login password change challenge failed: %v", err)
	}
	if insertedType != dashboardauth.DashboardMFAChallengePasswordChange || tx.commits != 1 {
		t.Fatalf("insertedType=%q commits=%d", insertedType, tx.commits)
	}
}

type dashboardIdentityAffectedResult struct {
	affected int64
}

func (result dashboardIdentityAffectedResult) LastInsertId() (int64, error) { return 0, nil }
func (result dashboardIdentityAffectedResult) RowsAffected() (int64, error) {
	return result.affected, nil
}

func TestDashboardIdentityStorePasswordChangeRequiresChallengeConsumption(t *testing.T) {
	tx := &identityTestTx{}
	tx.query = func(query string, _ ...any) identityRowScanner {
		switch {
		case strings.Contains(query, "mochat_go_dashboard_mfa_challenges"):
			return identityTestRow{values: []any{7, uint64(4)}}
		case strings.Contains(query, "mochat_go_dashboard_identities"):
			return identityTestRow{values: []any{7, "13800000000", "new-hash", 1, 0, uint64(5), 1}}
		default:
			return identityTestRow{err: sql.ErrNoRows}
		}
	}
	execCount := 0
	tx.exec = func(query string, _ ...any) (sql.Result, error) {
		execCount++
		if execCount == 2 && strings.Contains(query, "mochat_go_dashboard_mfa_challenges") {
			return dashboardIdentityAffectedResult{}, nil
		}
		return identityTestResult{}, nil
	}
	store := &DashboardIdentityStore{begin: func(context.Context) (dashboardIdentityTx, error) { return tx, nil }}
	_, err := store.CompletePasswordChange(context.Background(), [32]byte{1}, "new-hash")
	if !errors.Is(err, dashboardauth.ErrInvalidPassword) {
		t.Fatalf("error=%v, want challenge consumption failure", err)
	}
	if tx.commits != 0 {
		t.Fatal("password change committed after a zero-row challenge consume")
	}
}

func TestDashboardIdentityStorePasswordResetRequiresResetConsumption(t *testing.T) {
	tx := &identityTestTx{}
	tx.query = func(query string, _ ...any) identityRowScanner {
		switch {
		case strings.Contains(query, "mochat_go_dashboard_password_resets"):
			return identityTestRow{values: []any{7, uint64(4)}}
		case strings.Contains(query, "mochat_go_dashboard_identities"):
			return identityTestRow{values: []any{7, "13800000000", "new-hash", 1, 0, uint64(5), 1}}
		default:
			return identityTestRow{err: sql.ErrNoRows}
		}
	}
	execCount := 0
	tx.exec = func(query string, _ ...any) (sql.Result, error) {
		execCount++
		if execCount == 2 && strings.Contains(query, "mochat_go_dashboard_password_resets") {
			return dashboardIdentityAffectedResult{}, nil
		}
		return identityTestResult{}, nil
	}
	store := &DashboardIdentityStore{begin: func(context.Context) (dashboardIdentityTx, error) { return tx, nil }}
	_, err := store.CompletePasswordReset(context.Background(), [32]byte{1}, "new-hash")
	if !errors.Is(err, dashboardauth.ErrInvalidPassword) {
		t.Fatalf("error=%v, want reset consumption failure", err)
	}
	if tx.commits != 0 {
		t.Fatal("password reset committed after a zero-row reset consume")
	}
}

func TestDashboardIdentityStorePasswordResetLocksCurrentIdentityVersion(t *testing.T) {
	tests := []struct {
		name           string
		currentVersion uint64
		wantError      bool
	}{
		{name: "current version", currentVersion: 4},
		{name: "stale version", currentVersion: 5, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tx := &identityTestTx{}
			tx.query = func(query string, _ ...any) identityRowScanner {
				if strings.Contains(query, "mochat_go_dashboard_identities") {
					return identityTestRow{values: []any{1, test.currentVersion}}
				}
				return identityTestRow{err: sql.ErrNoRows}
			}
			tx.exec = func(query string, _ ...any) (sql.Result, error) {
				if !strings.Contains(query, "INSERT INTO mochat_go_dashboard_password_resets") {
					t.Fatalf("unexpected Dashboard reset SQL: %s", query)
				}
				return identityTestResult{}, nil
			}
			store := &DashboardIdentityStore{begin: func(context.Context) (dashboardIdentityTx, error) { return tx, nil }}
			err := store.CreatePasswordReset(context.Background(), 7, 4, [32]byte{1}, time.Now().Add(time.Minute))
			if (err != nil) != test.wantError {
				t.Fatalf("error=%v wantError=%v", err, test.wantError)
			}
			if test.wantError && len(tx.execs) != 0 {
				t.Fatal("stale Dashboard auth version inserted a reset token")
			}
			if !test.wantError && (tx.commits != 1 || len(tx.execs) != 1) {
				t.Fatalf("reset was not committed atomically: commits=%d execs=%d", tx.commits, len(tx.execs))
			}
		})
	}
}
