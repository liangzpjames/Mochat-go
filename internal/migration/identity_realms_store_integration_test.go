package migration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"testing"

	"jiyi/mochat-go/internal/dashboardauth"
	"jiyi/mochat-go/internal/saasauth"
	"jiyi/mochat-go/internal/store"
)

func TestIdentityRealmsStoresAgainstIsolatedMariaDB(t *testing.T) {
	db := newIdentitySingleCorpMigrationDB(t)
	createIdentitySingleCorpBaseFixture(t, db)
	execIdentitySingleCorpMigration(t, db, "0129_identity_realms_single_corp_schema.up.sql", false)

	ctx := context.Background()
	saasStore := store.NewSaaSIdentityStore(db)
	dashboardStore := store.NewDashboardIdentityStore(db)

	beforeBootstrap := identityRealmsCounts(t, db)
	hashA, err := saasauth.HashPassword("real-bootstrap-password-a")
	if err != nil {
		t.Fatal(err)
	}
	hashB, err := saasauth.HashPassword("real-bootstrap-password-b")
	if err != nil {
		t.Fatal(err)
	}
	bootstrapInput := saasauth.BootstrapSaaSAdmin{
		RequestKey: "real-bootstrap-request-1",
		LoginName:  "platform-admin",
		Phone:      "13800000009",
		Name:       "Platform Admin",
	}

	results := make(chan struct {
		identity saasauth.SaaSIdentity
		err      error
	}, 2)
	start := make(chan struct{})
	var wait sync.WaitGroup
	for _, passwordHash := range []string{hashA, hashB} {
		passwordHash := passwordHash
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			input := bootstrapInput
			input.PasswordHash = passwordHash
			identity, err := saasStore.Bootstrap(ctx, input)
			results <- struct {
				identity saasauth.SaaSIdentity
				err      error
			}{identity: identity, err: err}
		}()
	}
	close(start)
	wait.Wait()
	close(results)

	var concurrentID int
	for result := range results {
		if result.err != nil {
			t.Fatalf("concurrent bootstrap failed: %v", result.err)
		}
		if concurrentID == 0 {
			concurrentID = result.identity.ID
		} else if result.identity.ID != concurrentID {
			t.Fatalf("concurrent bootstrap returned different identities: %d and %d", concurrentID, result.identity.ID)
		}
	}
	if concurrentID <= 0 || identityRealmsCount(t, db, "mochat_go_saas_admin_users") != 1 {
		t.Fatal("concurrent bootstrap did not create exactly one SaaS identity")
	}
	assertSaaSPlatformRootAssignment(t, db, concurrentID)
	assertIdentityRealmCountsEqual(t, db, beforeBootstrap, "bootstrap must not create Dashboard, tenant, corp, or mc_user rows")

	var storedHash, storedRequestKey string
	if err := db.QueryRow(`SELECT password_hash, bootstrap_request_key FROM mochat_go_saas_admin_users WHERE id = ?`, concurrentID).Scan(&storedHash, &storedRequestKey); err != nil {
		t.Fatal(err)
	}
	if storedRequestKey != bootstrapInput.RequestKey || (storedHash != hashA && storedHash != hashB) {
		t.Fatal("bootstrap request key or initial password hash was not persisted")
	}
	if err := saasStore.CheckSession(ctx, concurrentID, 1); err != nil {
		t.Fatalf("SaaS session check rejected current status/version: %v", err)
	}
	if _, err := db.Exec(`UPDATE mochat_go_saas_admin_users SET status = 2 WHERE id = ?`, concurrentID); err != nil {
		t.Fatal(err)
	}
	if err := saasStore.CheckSession(ctx, concurrentID, 1); !errors.Is(err, saasauth.ErrSessionInvalid) {
		t.Fatalf("disabled SaaS identity session error=%v", err)
	}
	if _, err := db.Exec(`UPDATE mochat_go_saas_admin_users SET status = 1, auth_version = 2 WHERE id = ?`, concurrentID); err != nil {
		t.Fatal(err)
	}
	if err := saasStore.CheckSession(ctx, concurrentID, 1); !errors.Is(err, saasauth.ErrSessionInvalid) {
		t.Fatalf("stale SaaS identity session error=%v", err)
	}
	if err := saasStore.CheckSession(ctx, concurrentID, 2); err != nil {
		t.Fatalf("current SaaS auth_version was not accepted: %v", err)
	}
	if _, err := db.Exec(`UPDATE mochat_go_saas_admin_users SET auth_version = 1 WHERE id = ?`, concurrentID); err != nil {
		t.Fatal(err)
	}

	hashC, err := saasauth.HashPassword("real-bootstrap-password-c")
	if err != nil {
		t.Fatal(err)
	}
	repeated := bootstrapInput
	repeated.PasswordHash = hashC
	repeatedIdentity, err := saasStore.Bootstrap(ctx, repeated)
	if err != nil {
		t.Fatal(err)
	}
	if repeatedIdentity.ID != concurrentID || repeatedIdentity.PasswordHash != storedHash || identityRealmsCount(t, db, "mochat_go_saas_admin_users") != 1 {
		t.Fatal("repeated request key reset the password or created another SaaS identity")
	}
	assertSaaSPlatformRootAssignment(t, db, concurrentID)

	conflictInput := bootstrapInput
	conflictInput.LoginName = "different-login"
	conflictInput.Phone = "13800000008"
	conflictInput.Name = "Different Admin"
	conflictInput.PasswordHash = hashC
	if _, err := saasStore.Bootstrap(ctx, conflictInput); !errors.Is(err, saasauth.ErrBootstrapConflict) {
		t.Fatalf("same request key with different business input error=%v", err)
	}
	otherRequest := conflictInput
	otherRequest.RequestKey = "real-bootstrap-request-2"
	otherRequest.Phone = bootstrapInput.Phone
	if _, err := saasStore.Bootstrap(ctx, otherRequest); !errors.Is(err, saasauth.ErrBootstrapConflict) {
		t.Fatalf("different request key with a unique phone conflict error=%v", err)
	}
	if identityRealmsCount(t, db, "mochat_go_saas_admin_users") != 1 {
		t.Fatal("bootstrap conflict wrote a second SaaS identity")
	}
	if _, err := saasauth.NewService(saasStore).Bootstrap(ctx, otherRequest); !errors.Is(err, saasauth.ErrBootstrapConflict) || strings.Contains(err.Error(), hashC) {
		t.Fatal("bootstrap conflict was not safely sanitized")
	}

	dashboardHash, err := dashboardauth.HashPassword("real-dashboard-password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO mochat_go_dashboard_identities
			(user_id, login_identifier, password_hash, status, must_rotate_password, auth_version, mfa_required)
		VALUES (10, 'dashboard-real-user', ?, 1, 1, 7, 0)
	`, dashboardHash); err != nil {
		t.Fatal(err)
	}
	dashboardIdentity, err := dashboardStore.Authenticate(ctx, "dashboard-real-user")
	if err != nil || dashboardIdentity.PasswordHash != dashboardHash || dashboardIdentity.UserID != 10 {
		t.Fatalf("Dashboard store did not read its identity row safely: %v", err)
	}
	if err := dashboardStore.CheckSession(ctx, 10, 7); err != nil {
		t.Fatalf("Dashboard session check rejected current status/version: %v", err)
	}
	if _, err := db.Exec(`UPDATE mochat_go_dashboard_identities SET status = 2 WHERE user_id = 10`); err != nil {
		t.Fatal(err)
	}
	if err := dashboardStore.CheckSession(ctx, 10, 7); !errors.Is(err, dashboardauth.ErrSessionInvalid) {
		t.Fatalf("disabled Dashboard identity session error=%v", err)
	}
	if _, err := db.Exec(`UPDATE mochat_go_dashboard_identities SET status = 1, auth_version = 8 WHERE user_id = 10`); err != nil {
		t.Fatal(err)
	}
	if err := dashboardStore.CheckSession(ctx, 10, 7); !errors.Is(err, dashboardauth.ErrSessionInvalid) {
		t.Fatalf("stale Dashboard identity session error=%v", err)
	}
	if err := dashboardStore.CheckSession(ctx, 10, 8); err != nil {
		t.Fatalf("current Dashboard auth_version was not accepted: %v", err)
	}

	activationService := dashboardauth.NewService(dashboardStore)
	activationDigest := sha256.Sum256([]byte("real-dashboard-activation-one"))
	if _, err := db.Exec(`
		INSERT INTO mochat_go_dashboard_identity_activations
			(user_id, token_digest, expires_at, created_by_saas_user_id, request_id)
		VALUES (10, ?, DATE_ADD(NOW(), INTERVAL 10 MINUTE), ?, 'real-activation-1')
	`, activationDigest[:], concurrentID); err != nil {
		t.Fatal(err)
	}
	if err := activationService.Activate(ctx, activationDigest, "real-activation-password"); err != nil {
		t.Fatalf("Dashboard activation failed: %v", err)
	}
	var activatedHash string
	var activatedVersion uint64
	var consumedAt sql.NullTime
	if err := db.QueryRow(`
		SELECT d.password_hash, d.auth_version, a.consumed_at
		FROM mochat_go_dashboard_identities d
		JOIN mochat_go_dashboard_identity_activations a ON a.user_id = d.user_id
		WHERE d.user_id = 10 AND a.token_digest = ?
	`, activationDigest[:]).Scan(&activatedHash, &activatedVersion, &consumedAt); err != nil {
		t.Fatal(err)
	}
	if activatedVersion != 9 || !consumedAt.Valid || !dashboardauth.VerifyPassword(activatedHash, "real-activation-password") {
		t.Fatal("Dashboard activation did not consume its digest and increment the identity version")
	}
	if err := activationService.Activate(ctx, activationDigest, "real-activation-password-again"); !errors.Is(err, dashboardauth.ErrActivationInvalid) {
		t.Fatalf("reused activation error=%v", err)
	}
	var repeatedVersion uint64
	var repeatedHash string
	if err := db.QueryRow(`SELECT password_hash, auth_version FROM mochat_go_dashboard_identities WHERE user_id = 10`).Scan(&repeatedHash, &repeatedVersion); err != nil {
		t.Fatal(err)
	}
	if repeatedVersion != activatedVersion || repeatedHash != activatedHash {
		t.Fatal("reused activation changed Dashboard identity state")
	}

	rollbackDigest := sha256.Sum256([]byte("real-dashboard-activation-rollback"))
	if _, err := db.Exec(`
		INSERT INTO mochat_go_dashboard_identity_activations
			(user_id, token_digest, expires_at, created_by_saas_user_id, request_id)
		VALUES (10, ?, DATE_ADD(NOW(), INTERVAL 10 MINUTE), ?, 'real-activation-rollback')
	`, rollbackDigest[:], concurrentID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE mochat_go_dashboard_identities SET status = 2 WHERE user_id = 10`); err != nil {
		t.Fatal(err)
	}
	if err := activationService.Activate(ctx, rollbackDigest, "real-rollback-password"); !errors.Is(err, dashboardauth.ErrActivationInvalid) {
		t.Fatalf("activation rollback error=%v", err)
	}
	var rollbackStatus int
	var rollbackVersion uint64
	var rollbackConsumed sql.NullTime
	if err := db.QueryRow(`
		SELECT d.status, d.auth_version, a.consumed_at
		FROM mochat_go_dashboard_identities d
		JOIN mochat_go_dashboard_identity_activations a ON a.user_id = d.user_id
		WHERE d.user_id = 10 AND a.token_digest = ?
	`, rollbackDigest[:]).Scan(&rollbackStatus, &rollbackVersion, &rollbackConsumed); err != nil {
		t.Fatal(err)
	}
	if rollbackStatus != 2 || rollbackVersion != activatedVersion || rollbackConsumed.Valid {
		t.Fatal("failed Dashboard activation did not roll back its partial state")
	}
}

func assertSaaSPlatformRootAssignment(t *testing.T, db *sql.DB, userID int) {
	t.Helper()
	var roleCount, permissionCount, assignmentCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_saas_admin_roles WHERE code='platform_root' AND status=1 AND is_system=1`).Scan(&roleCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_saas_admin_role_permissions rp INNER JOIN mochat_go_saas_admin_roles r ON r.id=rp.role_id WHERE r.code='platform_root' AND rp.permission_code='*'`).Scan(&permissionCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_saas_admin_user_roles ur INNER JOIN mochat_go_saas_admin_roles r ON r.id=ur.role_id WHERE ur.user_id=? AND r.code='platform_root'`, userID).Scan(&assignmentCount); err != nil {
		t.Fatal(err)
	}
	if roleCount != 1 || permissionCount != 1 || assignmentCount != 1 {
		t.Fatalf("SaaS root authority counts role=%d permission=%d assignment=%d", roleCount, permissionCount, assignmentCount)
	}
}

func identityRealmsCounts(t *testing.T, db *sql.DB, tables ...string) map[string]int {
	t.Helper()
	counts := make(map[string]int, len(tables))
	if len(tables) == 0 {
		tables = []string{
			"mochat_go_dashboard_identities",
			"mochat_go_tenant_corp_bindings",
			"mc_tenant",
			"mc_corp",
			"mc_user",
		}
	}
	for _, table := range tables {
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM `" + table + "`").Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		counts[table] = count
	}
	return counts
}

func identityRealmsCount(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	return identityRealmsCounts(t, db, table)[table]
}

func assertIdentityRealmCountsEqual(t *testing.T, db *sql.DB, before map[string]int, message string) {
	t.Helper()
	for table, want := range before {
		got := identityRealmsCounts(t, db, table)[table]
		if got != want {
			t.Fatalf("%s: %s before=%d after=%d", message, table, want, got)
		}
	}
}
