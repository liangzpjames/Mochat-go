package store

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/dashboardauth"
)

var _ dashboardauth.DashboardIdentityStore = (*DashboardIdentityStore)(nil)

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
