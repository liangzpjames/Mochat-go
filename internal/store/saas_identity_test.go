package store

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/saasauth"
)

type identityTestRow struct {
	values []any
	err    error
}

func (row identityTestRow) Scan(dest ...any) error {
	if row.err != nil {
		return row.err
	}
	if len(dest) != len(row.values) {
		return errors.New("identity test row column count mismatch")
	}
	for index, value := range row.values {
		target := reflect.ValueOf(dest[index])
		if target.Kind() != reflect.Pointer || target.IsNil() {
			return errors.New("identity test row destination is not a pointer")
		}
		target = target.Elem()
		source := reflect.ValueOf(value)
		if !source.IsValid() {
			target.SetZero()
			continue
		}
		if source.Type().AssignableTo(target.Type()) {
			target.Set(source)
			continue
		}
		if source.Type().ConvertibleTo(target.Type()) {
			target.Set(source.Convert(target.Type()))
			continue
		}
		return errors.New("identity test row value type mismatch")
	}
	return nil
}

type identityTestResult struct{ id int64 }

func (result identityTestResult) LastInsertId() (int64, error) { return result.id, nil }
func (result identityTestResult) RowsAffected() (int64, error) { return 1, nil }

type identityTestTx struct {
	query     func(string, ...any) identityRowScanner
	exec      func(string, ...any) (sql.Result, error)
	queries   []string
	queryArgs [][]any
	execs     []string
	execArgs  [][]any
	commits   int
	rollbacks int
}

func (tx *identityTestTx) QueryRowContext(_ context.Context, query string, args ...any) identityRowScanner {
	tx.queries = append(tx.queries, query)
	tx.queryArgs = append(tx.queryArgs, append([]any(nil), args...))
	return tx.query(query, args...)
}

func (tx *identityTestTx) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	tx.execs = append(tx.execs, query)
	tx.execArgs = append(tx.execArgs, append([]any(nil), args...))
	return tx.exec(query, args...)
}

func (tx *identityTestTx) Commit() error   { tx.commits++; return nil }
func (tx *identityTestTx) Rollback() error { tx.rollbacks++; return nil }

func TestSaaSIdentityStoreQueriesOnlySaaSIdentityTable(t *testing.T) {
	var query string
	store := &SaaSIdentityStore{
		queryRow: func(_ context.Context, statement string, _ ...any) identityRowScanner {
			query = statement
			return identityTestRow{values: []any{7, "admin", "13800000000", "bcrypt-hash", "Platform Admin", 1, 1, uint64(3), 1}}
		},
	}
	identity, err := store.Authenticate(context.Background(), "admin")
	if err != nil {
		t.Fatal(err)
	}
	if identity.ID != 7 || identity.AuthVersion != 3 || identity.PasswordHash != "bcrypt-hash" {
		t.Fatal("SaaS identity row was not loaded")
	}
	if !strings.Contains(query, "FROM mochat_go_saas_admin_users") || strings.Contains(query, "mc_user") || strings.Contains(query, "dashboard") || strings.Contains(query, "deleted_at") {
		t.Fatalf("SaaS identity query crossed an identity boundary: %s", query)
	}
}

func TestSaaSIdentityStoreCheckSessionChecksStatusAndAuthVersion(t *testing.T) {
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
			store := &SaaSIdentityStore{
				queryRow: func(_ context.Context, statement string, _ ...any) identityRowScanner {
					query = statement
					return tc.row
				},
			}
			err := store.CheckSession(context.Background(), 7, 4)
			if (err == nil) != tc.wantOK {
				t.Fatalf("CheckSession error state = %v, wantOK=%v", err, tc.wantOK)
			}
			if !strings.Contains(query, "status") || !strings.Contains(query, "auth_version") || !strings.Contains(query, "mochat_go_saas_admin_users") || strings.Contains(query, "deleted_at") {
				t.Fatal("CheckSession did not query status and auth_version from the SaaS identity table")
			}
		})
	}
}

func TestSaaSIdentityStoreBootstrapIsIdempotentAndDoesNotResetPassword(t *testing.T) {
	lookupCount := 0
	tx := &identityTestTx{}
	tx.query = func(_ string, _ ...any) identityRowScanner {
		lookupCount++
		if lookupCount == 1 {
			return identityTestRow{err: sql.ErrNoRows}
		}
		return identityTestRow{values: []any{42, "platform-admin", "", "first-hash", "Platform Admin", 1, 1, uint64(1), 1}}
	}
	insertCount := 0
	tx.exec = func(statement string, _ ...any) (sql.Result, error) {
		insertCount++
		if !strings.Contains(statement, "INSERT INTO mochat_go_saas_admin_users") || strings.Contains(statement, "ON DUPLICATE KEY UPDATE") || strings.Contains(statement, "UPDATE mochat_go_saas_admin_users") {
			t.Fatal("bootstrap must insert once and never update an existing password")
		}
		return identityTestResult{id: 42}, nil
	}
	store := &SaaSIdentityStore{
		begin: func(context.Context) (saasIdentityTx, error) { return tx, nil },
	}
	input := saasauth.BootstrapSaaSAdmin{RequestKey: "bootstrap-request-1", LoginName: "platform-admin", Name: "Platform Admin", PasswordHash: "first-hash"}
	first, err := store.Bootstrap(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	input.PasswordHash = "second-hash"
	second, err := store.Bootstrap(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if insertCount != 1 || first.ID != 42 || second.PasswordHash != "first-hash" || tx.commits != 2 {
		t.Fatal("repeated bootstrap was not idempotent or changed the stored password")
	}
	for _, query := range tx.queries {
		if !strings.Contains(query, "mochat_go_saas_admin_users") || strings.Contains(query, "mc_user") || strings.Contains(query, "mc_tenant") || strings.Contains(query, "mc_corp") {
			t.Fatal("SaaS bootstrap crossed into a business identity table")
		}
	}
}
