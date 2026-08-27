package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"jiyi/mochat-go/internal/dashboardadmin"
	"jiyi/mochat-go/internal/dashboardauth"
	archiveprovider "jiyi/mochat-go/internal/modules/providers/archive"
	"jiyi/mochat-go/internal/mysqlconn"
	"jiyi/mochat-go/internal/store"
	"jiyi/mochat-go/internal/testfixtures/archivesource"
	"jiyi/mochat-go/internal/wecomarchivedemo"
	"jiyi/mochat-go/internal/wecomcredentials"
)

const (
	datasetID               = archivesource.DatasetMarker
	acceptanceTenant        = 820827
	acceptanceCorp          = 820827
	acceptanceUser          = 820827
	acceptanceStaff         = 820827
	acceptanceContact       = 820827
	acceptanceWXCorp        = "ww-MOCHAT-LOCAL-ACCEPTANCE-20260827"
	acceptanceSource        = "wecom:" + acceptanceWXCorp
	acceptanceRunKey        = "archive:MOCHAT-LOCAL-ACCEPTANCE-20260827:cursor:0"
	acceptanceWorkerRunLike = "archive:820827:820827:wecom:ww-MOCHAT-LOCAL-ACCEPTANCE-20260827:%"
	acceptanceIntegrationID = "a2080827-0000-4000-8000-000000000001"
	acceptancePackageID     = 820827
	acceptancePackageCode   = datasetID + "-ACTIVATION"
	acceptanceLimitsJSON    = `{"maxCorps":1,"maxUsers":10,"maxContacts":100,"maxRooms":10,"maxAgents":10,"channelCodes":10,"shopCodes":10,"radars":10,"lotteries":10,"roomInfinitePulls":10,"roomFissions":10,"roomClockIns":10,"roomQualities":10,"roomCalendars":10,"roomReminds":10,"contactSops":10,"roomSops":10,"sensitiveWords":10,"storageMb":100,"contactMessageBatches":10,"roomMessageBatches":10,"roomTagPulls":10,"workRoomAutoPulls":10,"workFissions":10,"officialAccounts":10,"asyncExecutions":10}`
)

type options struct {
	Action                string
	DSN                   string
	BridgeURL             string
	BridgeToken           string
	EncryptionKey         string
	StorageRoot           string
	StateDir              string
	Addr                  string
	APIBaseURL            string
	DashboardPasswordFile string
	ActivationFixtureDir  string
	DryRun                bool
	Timeout               time.Duration
}

type summary struct {
	Dataset    string `json:"dataset"`
	Action     string `json:"action"`
	BridgeURL  string `json:"bridgeUrl,omitempty"`
	APIBaseURL string `json:"apiBaseUrl,omitempty"`
	DryRun     bool   `json:"dryRun,omitempty"`
}

func (o options) publicSummary() summary {
	return summary{Dataset: datasetID, Action: o.Action, BridgeURL: o.BridgeURL, APIBaseURL: o.APIBaseURL, DryRun: o.DryRun}
}

func main() {
	if err := run(context.Background(), os.Stdout, os.Args[1:], os.Getenv); err != nil {
		log.Fatal(redactError(err))
	}
}

func run(ctx context.Context, output io.Writer, args []string, getenv func(string) string) error {
	values, err := parseOptions(args, getenv)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, values.Timeout)
	defer cancel()
	switch values.Action {
	case "bridge":
		cancel()
		return serveBridge(context.Background(), output, values)
	case "seed":
		return seed(ctx, output, values)
	case "verify":
		return verify(ctx, output, values)
	case "cleanup":
		return cleanup(ctx, output, values)
	default:
		return fmt.Errorf("unsupported action %q", values.Action)
	}
}

func parseOptions(args []string, getenv func(string) string) (options, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	action := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		action, args = strings.ToLower(strings.TrimSpace(args[0])), args[1:]
	}
	flags := flag.NewFlagSet("mochat-archive-acceptance", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	values := options{Action: action}
	flags.StringVar(&values.Action, "action", values.Action, "bridge, seed, verify, or cleanup")
	flags.StringVar(&values.DSN, "dsn", getenv("MOCHAT_MYSQL_DSN"), "MySQL DSN")
	flags.StringVar(&values.BridgeURL, "bridge-url", envOr(getenv, "MOCHAT_ACCEPTANCE_BRIDGE_URL", "http://127.0.0.1:19091"), "fixture bridge URL")
	flags.StringVar(&values.BridgeToken, "bridge-token", getenv("MOCHAT_ACCEPTANCE_BRIDGE_TOKEN"), "fixture bridge bearer")
	flags.StringVar(&values.EncryptionKey, "encryption-key", getenv("MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY"), "WeCom credential encryption key")
	flags.StringVar(&values.StorageRoot, "storage-root", envOr(getenv, "MOCHAT_FILE_STORAGE_ROOT", ".runtime/wecom-acceptance/storage"), "media storage root")
	flags.StringVar(&values.StateDir, "state-dir", envOr(getenv, "MOCHAT_ACCEPTANCE_STATE_DIR", ".runtime/wecom-acceptance/bridge"), "bridge evidence directory")
	flags.StringVar(&values.Addr, "addr", envOr(getenv, "MOCHAT_ACCEPTANCE_BRIDGE_ADDR", "127.0.0.1:19091"), "bridge listen address")
	flags.StringVar(&values.APIBaseURL, "api-base-url", envOr(getenv, "MOCHAT_ACCEPTANCE_API_BASE_URL", "http://127.0.0.1:19080"), "application base URL")
	flags.StringVar(&values.DashboardPasswordFile, "dashboard-password-file", getenv("MOCHAT_ACCEPTANCE_DASHBOARD_PASSWORD_FILE"), "local Dashboard acceptance password file")
	flags.StringVar(&values.ActivationFixtureDir, "activation-fixture-dir", envOr(getenv, "MOCHAT_ACCEPTANCE_ACTIVATION_FIXTURE_DIR", ".runtime/wecom-acceptance/activation"), "directory for local activation fixture tokens")
	flags.BoolVar(&values.DryRun, "dry-run", false, "report cleanup counts without deleting")
	flags.DurationVar(&values.Timeout, "timeout", 2*time.Minute, "command timeout")
	if err := flags.Parse(args); err != nil {
		return options{}, err
	}
	values.Action = strings.ToLower(strings.TrimSpace(values.Action))
	if values.Action == "" {
		return options{}, errors.New("action is required")
	}
	if values.Timeout <= 0 {
		return options{}, errors.New("timeout must be positive")
	}
	if values.Action == "bridge" {
		if len(strings.TrimSpace(values.BridgeToken)) < 40 {
			return options{}, errors.New("fixture bridge bearer is missing or too short")
		}
		return values, nil
	}
	if strings.TrimSpace(values.DSN) == "" {
		return options{}, errors.New("MySQL DSN is required")
	}
	if values.Action == "seed" && (len(strings.TrimSpace(values.BridgeToken)) < 40 || strings.TrimSpace(values.EncryptionKey) == "" || strings.TrimSpace(values.DashboardPasswordFile) == "") {
		return options{}, errors.New("seed requires fixture bridge bearer, WeCom credential encryption key, and Dashboard password file")
	}
	if values.Action == "verify" && strings.TrimSpace(values.DashboardPasswordFile) == "" {
		return options{}, errors.New("verify requires Dashboard password file")
	}
	return values, nil
}

func envOr(getenv func(string) string, key, fallback string) string {
	if value := strings.TrimSpace(getenv(key)); value != "" {
		return value
	}
	return fallback
}

func newFixtureBridge(stateDir, token string) (http.Handler, func(), error) {
	fixture, err := archivesource.NewArchiveFixture()
	if err != nil {
		return nil, nil, err
	}
	evidence, err := wecomarchivedemo.NewEvidenceStore(stateDir)
	if err != nil {
		fixture.Close()
		return nil, nil, err
	}
	service, err := wecomarchivedemo.NewArchiveService(fixture, fixture.PrivateKeyPEM(), evidence, 100, 5)
	if err != nil {
		fixture.Close()
		return nil, nil, err
	}
	admin := wecomarchivedemo.NewAdminHandler(wecomarchivedemo.Config{
		AdminToken: token, CorpID: acceptanceWXCorp, PullLimit: 100, TimeoutSeconds: 5,
	}, evidence, service)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "dataset": datasetID, "production": false})
	})
	mux.Handle("/", admin)
	return mux, func() { _ = fixture.Close() }, nil
}

func serveBridge(ctx context.Context, output io.Writer, values options) error {
	handler, closeFixture, err := newFixtureBridge(values.StateDir, values.BridgeToken)
	if err != nil {
		return err
	}
	defer closeFixture()
	server := &http.Server{Addr: values.Addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second}
	shutdownContext, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-shutdownContext.Done()
		deadline, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(deadline)
	}()
	_ = json.NewEncoder(output).Encode(map[string]any{"dataset": datasetID, "action": "bridge", "addr": values.Addr, "production": false})
	err = server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func openAcceptanceStore(ctx context.Context, values options) (*sql.DB, *store.MySQLStore, error) {
	db, err := mysqlconn.Open(values.DSN)
	if err != nil {
		return nil, nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, nil, err
	}
	manager, err := newAcceptanceCredentialManager(values)
	if err != nil {
		db.Close()
		return nil, nil, err
	}
	return db, store.NewMySQLStore(db).WithWeComCredentialCipher(manager), nil
}

func newAcceptanceCredentialManager(values options) (*wecomcredentials.Manager, error) {
	return wecomcredentials.NewManager(wecomcredentials.Config{
		EncryptionKey: values.EncryptionKey, EncryptionKeyID: "acceptance-v1", RequireEncryption: values.EncryptionKey != "", DedicatedConfigured: values.EncryptionKey != "",
	})
}

func seed(ctx context.Context, output io.Writer, values options) error {
	db, archiveStore, err := openAcceptanceStore(ctx, values)
	if err != nil {
		return err
	}
	defer db.Close()
	password, err := readAcceptancePassword(values.DashboardPasswordFile)
	if err != nil {
		return err
	}
	passwordHash, err := dashboardauth.HashPassword(password)
	if err != nil {
		return errors.New("hash local Dashboard acceptance password")
	}
	if err := prepareInfrastructure(ctx, db, passwordHash); err != nil {
		return err
	}
	credentialManager, err := newAcceptanceCredentialManager(values)
	if err != nil {
		return err
	}
	if err := prepareWeComIntegrationFixtures(ctx, db, archiveStore, credentialManager); err != nil {
		return err
	}
	activationStates, err := prepareActivationFixtures(ctx, db, archiveStore, values.ActivationFixtureDir)
	if err != nil {
		return err
	}
	client, err := archiveprovider.NewBridgeArchiveClient(values.BridgeURL, values.BridgeToken, nil)
	if err != nil {
		return err
	}
	source, err := archiveprovider.NewBridgeSource(client, archiveprovider.Scope{TenantID: acceptanceTenant, CorpID: acceptanceCorp}, acceptanceWXCorp)
	if err != nil {
		return err
	}
	run, err := archiveprovider.NewSyncService(archiveStore).Sync(ctx, source, archiveprovider.SyncRequest{
		Scope: archiveprovider.Scope{TenantID: acceptanceTenant, CorpID: acceptanceCorp}, StartCursor: archiveprovider.Cursor{}, Limit: 3,
		RetryFailed: true, IdempotencyKey: acceptanceRunKey,
	})
	if err != nil {
		return err
	}
	media := archiveprovider.NewMediaSyncService(archiveStore, client, values.StorageRoot)
	processed := 0
	for processed < 32 {
		didWork, runErr := media.RunOne(ctx)
		if runErr != nil && !terminalFixtureMediaError(runErr) {
			return runErr
		}
		if !didWork {
			break
		}
		processed++
	}
	counts, err := collectCounts(ctx, db)
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(map[string]any{
		"dataset": datasetID, "action": "seed", "tenantId": acceptanceTenant, "corpId": acceptanceCorp,
		"runId": run.ID, "cursor": run.Cursor.Sequence, "syncIdempotent": run.Idempotent, "mediaProcessed": processed, "counts": counts, "activationFixtures": activationStates, "production": false,
	})
}

func terminalFixtureMediaError(err error) bool {
	if err == nil {
		return false
	}
	value := err.Error()
	return value == "archive.media_missing" || strings.HasPrefix(value, "archive.media_corrupt:") || strings.HasPrefix(value, "archive.media_integrity_mismatch:")
}

func prepareInfrastructure(ctx context.Context, db *sql.DB, dashboardPasswordHash string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, ownership := range []struct {
		query string
		id    any
		want  string
	}{
		{`SELECT name FROM mc_tenant WHERE id=?`, acceptanceTenant, datasetID + " 本地验收租户（非生产）"},
		{`SELECT name FROM mc_corp WHERE id=?`, acceptanceCorp, datasetID + " 本地验收企业（非生产）"},
	} {
		var existing string
		err := tx.QueryRowContext(ctx, ownership.query, ownership.id).Scan(&existing)
		if err == nil && existing != ownership.want {
			return fmt.Errorf("acceptance fixture id is occupied by non-dataset row")
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
	}
	statements := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO mc_tenant (id,name,status,copyright) VALUES (?,?,1,?) ON DUPLICATE KEY UPDATE status=1,deleted_at=NULL`, []any{acceptanceTenant, datasetID + " 本地验收租户（非生产）", datasetID}},
		{`INSERT INTO mochat_go_saas_tenant_packages (tenant_id,package_code,package_name,starts_at,expires_at,status,version,limits_json) VALUES (?,?,'本地验收套餐（非生产）',DATE_SUB(NOW(),INTERVAL 1 DAY),DATE_ADD(NOW(),INTERVAL 30 DAY),1,1,?) ON DUPLICATE KEY UPDATE package_code=VALUES(package_code),package_name=VALUES(package_name),starts_at=VALUES(starts_at),expires_at=VALUES(expires_at),status=1,limits_json=VALUES(limits_json),deleted_at=NULL`, []any{acceptanceTenant, datasetID, acceptanceLimitsJSON}},
		{`INSERT INTO mochat_go_saas_packages (id,code,name,status,version,max_corps,max_users,max_contacts,max_rooms,max_agents,channel_codes,shop_codes,radars,lotteries,room_infinite_pulls,room_fissions,room_clock_ins,room_qualities,room_calendars,room_reminds,contact_sops,room_sops,sensitive_words,storage_mb,contact_message_batches,room_message_batches,room_tag_pulls,work_room_auto_pulls,work_fissions,official_accounts,async_executions) VALUES (?,?,?,1,1,10,10,100,10,10,10,10,10,10,10,10,10,10,10,10,10,10,10,100,10,10,10,10,10,10,10) ON DUPLICATE KEY UPDATE name=VALUES(name),status=1,version=1,deleted_at=NULL`, []any{acceptancePackageID, acceptancePackageCode, datasetID + " 激活验收套餐（非生产）"}},
		{`INSERT INTO mochat_go_saas_subscriptions (tenant_id,package_code,package_name,status,billing_cycle,current_period_starts_at,current_period_ends_at,version,state_reason,metadata_json) VALUES (?,?,?,'active','custom',DATE_SUB(NOW(),INTERVAL 1 DAY),DATE_ADD(NOW(),INTERVAL 30 DAY),1,?,JSON_OBJECT('dataset',?)) ON DUPLICATE KEY UPDATE package_code=VALUES(package_code),package_name=VALUES(package_name),status='active',current_period_starts_at=VALUES(current_period_starts_at),current_period_ends_at=VALUES(current_period_ends_at),state_reason=VALUES(state_reason),metadata_json=VALUES(metadata_json),deleted_at=NULL`, []any{acceptanceTenant, datasetID, "本地验收订阅（非生产）", datasetID, datasetID}},
		{`INSERT INTO mc_corp (id,name,wx_corpid,tenant_id,chat_status,created_at) VALUES (?,?,?,?,1,NOW()) ON DUPLICATE KEY UPDATE chat_status=1,deleted_at=NULL`, []any{acceptanceCorp, datasetID + " 本地验收企业（非生产）", acceptanceWXCorp, acceptanceTenant}},
		{`INSERT INTO mc_user (id,phone,name,status,tenant_id,isSuperAdmin) VALUES (?,?,?,1,?,1) ON DUPLICATE KEY UPDATE deleted_at=NULL,status=1`, []any{acceptanceUser, "19008208270", datasetID + " 本地验收管理员", acceptanceTenant}},
		{`INSERT INTO mochat_go_dashboard_identities (user_id,login_identifier,password_hash,status,must_rotate_password,auth_version,mfa_required,activated_at) VALUES (?,?,?,1,0,1,0,NOW()) ON DUPLICATE KEY UPDATE password_hash=VALUES(password_hash),status=1,must_rotate_password=0,mfa_required=0,activated_at=COALESCE(activated_at,NOW())`, []any{acceptanceUser, "19008208270", dashboardPasswordHash}},
		{`INSERT INTO mc_work_employee (id,wx_user_id,corp_id,name,status,audit_status,created_at) VALUES (?,?,?,?,1,1,NOW()) ON DUPLICATE KEY UPDATE deleted_at=NULL,status=1,audit_status=1`, []any{acceptanceStaff, datasetID + "-STAFF-01", acceptanceCorp, datasetID + " 本地验收员工"}},
		{`INSERT INTO mc_work_contact (id,corp_id,wx_external_userid,name,created_at) VALUES (?,?,?,?,NOW()) ON DUPLICATE KEY UPDATE deleted_at=NULL`, []any{acceptanceContact, acceptanceCorp, datasetID + "-EXTERNAL-01", datasetID + " 本地验收客户"}},
		{`INSERT INTO mochat_go_tenant_corp_bindings (tenant_id,corp_id,status,version,verified_wx_corpid,verified_corp_name,verified_at) VALUES (?,?,2,1,?,?,NOW()) ON DUPLICATE KEY UPDATE status=2,verified_wx_corpid=VALUES(verified_wx_corpid),verified_corp_name=VALUES(verified_corp_name),verified_at=COALESCE(verified_at,NOW())`, []any{acceptanceTenant, acceptanceCorp, acceptanceWXCorp, datasetID + " 本地验收企业（非生产）"}},
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement.query, statement.args...); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func prepareWeComIntegrationFixtures(ctx context.Context, db *sql.DB, archiveStore *store.MySQLStore, credentialManager *wecomcredentials.Manager) error {
	var actorID int
	if err := db.QueryRowContext(ctx, `SELECT id FROM mochat_go_saas_admin_users WHERE login_name=? AND bootstrap_request_key=? AND status=1 LIMIT 1`, "mochat-local-acceptance-admin", datasetID+"-SAAS-ADMIN").Scan(&actorID); err != nil {
		return fmt.Errorf("resolve acceptance SaaS actor: %w", err)
	}
	actor := dashboardadmin.Actor{UserID: actorID, Active: true, Permissions: []string{"*"}}
	if err := bootstrapEncryptedCurrentIntegration(ctx, db, credentialManager, actorID); err != nil {
		return err
	}
	verifier := dashboardadmin.WeComIntegrationVerifierFunc(func(_ context.Context, request dashboardadmin.WeComVerificationRequest) (dashboardadmin.WeComVerificationResult, error) {
		if request.TenantID != acceptanceTenant || request.CorpID != acceptanceCorp || request.AuthoritativeWXCorpID != acceptanceWXCorp || request.Mode != "third_party_delegated" || request.ProviderAppID != datasetID+"-PROVIDER-APP" || request.Credentials.PermanentCode == "" || request.Credentials.EmployeeSecret != "" || request.Credentials.ChatSecret != "" {
			return dashboardadmin.WeComVerificationResult{}, errors.New("acceptance delegated integration verification contract mismatch")
		}
		return dashboardadmin.WeComVerificationResult{VerifiedWXCorpID: acceptanceWXCorp, Scope: []string{"archive.read"}, VerificationLevel: dashboardadmin.WeComVerificationLocalContract}, nil
	})
	service := dashboardadmin.NewWeComIntegrationService(archiveStore, verifier)
	view, err := service.Get(ctx, actor, acceptanceTenant)
	if err != nil {
		return err
	}
	if view.Current == nil || !view.Current.CredentialConfigured {
		return errors.New("acceptance current WeCom integration credential is not configured")
	}
	if view.Candidate == nil {
		candidate, err := service.SaveCandidate(ctx, actor, acceptanceTenant, dashboardadmin.WeComIntegrationCandidateInput{
			Mode: "third_party_delegated", ProviderAppID: datasetID + "-PROVIDER-APP", PermanentCode: datasetID + "-PERMANENT-CODE-LOCAL-ONLY", Scope: []string{"archive.read"}, Version: view.Current.Version,
		})
		if err != nil {
			return err
		}
		candidate, err = service.VerifyCandidate(ctx, actor, acceptanceTenant, candidate.Version)
		if err != nil {
			return err
		}
		switched, err := service.Switch(ctx, actor, acceptanceTenant, candidate.Version)
		if err != nil {
			return err
		}
		if switched.Candidate == nil {
			return errors.New("acceptance integration switch did not preserve rollback candidate")
		}
		view, err = service.Rollback(ctx, actor, acceptanceTenant, switched.Candidate.Version)
		if err != nil {
			return err
		}
	}
	if view.Current == nil || view.Candidate == nil || view.Current.Mode != "self_built" || view.Candidate.Mode != "third_party_delegated" || !view.Current.CredentialConfigured || !view.Candidate.CredentialConfigured {
		return errors.New("acceptance integration current/candidate rollback contract failed")
	}
	if _, err := archiveStore.WeComIntegrationVerificationCandidate(ctx, actor, acceptanceTenant, view.Candidate.Version); err != nil {
		return fmt.Errorf("decrypt acceptance candidate integration: %w", err)
	}
	return nil
}

func bootstrapEncryptedCurrentIntegration(ctx context.Context, db *sql.DB, manager *wecomcredentials.Manager, actorID int) error {
	var integrationID, ciphertext, keyID string
	currentExists := true
	err := db.QueryRowContext(ctx, `SELECT id,COALESCE(credential_ciphertext,''),COALESCE(credential_key_id,'') FROM mochat_go_wecom_integrations WHERE tenant_id=? AND corp_id=? AND slot='current' LIMIT 1`, acceptanceTenant, acceptanceCorp).Scan(&integrationID, &ciphertext, &keyID)
	if errors.Is(err, sql.ErrNoRows) {
		currentExists = false
		integrationID = acceptanceIntegrationID
	} else if err != nil {
		return err
	}
	if ciphertext != "" && keyID != "" {
		if _, err := manager.DecryptAuthorization(acceptanceTenant, integrationID, keyID, ciphertext); err != nil {
			return fmt.Errorf("decrypt acceptance current integration: %w", err)
		}
		return nil
	}
	credential := wecomcredentials.AuthorizationCredential{
		Mode: "self_built", EmployeeSecret: datasetID + "-EMPLOYEE-SECRET", ContactSecret: datasetID + "-CONTACT-SECRET", AgentSecret: datasetID + "-AGENT-SECRET", ChatSecret: datasetID + "-CHAT-SECRET",
	}
	ciphertext, keyID, err = manager.EncryptAuthorization(acceptanceTenant, integrationID, credential)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if !currentExists {
		_, err = tx.ExecContext(ctx, `INSERT INTO mochat_go_wecom_integrations (id,tenant_id,corp_id,mode,slot,status,verified_wx_corpid,agent_id,credential_ciphertext,credential_key_id,credential_hint,scope_json,scope_digest,missing_capabilities_json,generation,version,verification_level,verified_at,activated_at,last_audit_at) VALUES (?,?,?,'self_built','current','active',?,1000002,?,?,?,JSON_ARRAY('archive.read'),?,JSON_ARRAY(),1,1,?,NOW(6),NOW(6),NOW(6))`, integrationID, acceptanceTenant, acceptanceCorp, acceptanceWXCorp, ciphertext, keyID, "agent,chat,contact,employee", dashboardadmin.WeComScopeDigest([]string{"archive.read"}), dashboardadmin.WeComVerificationLocalContract)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE mochat_go_wecom_integrations SET mode='self_built',status='active',verified_wx_corpid=?,agent_id=1000002,provider_app_id='',credential_ciphertext=?,credential_key_id=?,credential_hint=?,scope_json=JSON_ARRAY('archive.read'),scope_digest=?,missing_capabilities_json=JSON_ARRAY(),verification_level=?,verified_at=NOW(6),last_error_code='',last_error_at=NULL,updated_at=NOW(6) WHERE id=? AND tenant_id=? AND corp_id=? AND slot='current'`, acceptanceWXCorp, ciphertext, keyID, "agent,chat,contact,employee", dashboardadmin.WeComScopeDigest([]string{"archive.read"}), dashboardadmin.WeComVerificationLocalContract, integrationID, acceptanceTenant, acceptanceCorp)
	}
	if err != nil {
		return err
	}
	afterJSON := fmt.Sprintf(`{"id":%q,"mode":"self_built","slot":"current","credentialConfigured":true}`, integrationID)
	if _, err := tx.ExecContext(ctx, `INSERT INTO mochat_go_saas_admin_operation_logs (tenant_id,actor_user_id,actor_tenant_id,action,target_type,target_id,target_name,after_json,remark) VALUES (?,?,0,'wecom.integration.acceptance.bootstrap','wecom_integration',?,?,?,'local acceptance bootstrap; credential omitted')`, acceptanceTenant, actorID, integrationID, datasetID+" encrypted current", afterJSON); err != nil {
		return err
	}
	return tx.Commit()
}

func prepareActivationFixtures(ctx context.Context, db *sql.DB, archiveStore *store.MySQLStore, fixtureDir string) (map[string]string, error) {
	var actorID int
	if err := db.QueryRowContext(ctx, `SELECT id FROM mochat_go_saas_admin_users WHERE login_name=? AND bootstrap_request_key=? AND status=1 LIMIT 1`, "mochat-local-acceptance-admin", datasetID+"-SAAS-ADMIN").Scan(&actorID); err != nil {
		return nil, fmt.Errorf("resolve activation fixture SaaS actor: %w", err)
	}
	actor := dashboardadmin.Actor{UserID: actorID, Active: true, Permissions: []string{"*"}}
	provisioning := dashboardadmin.NewService(archiveStore)
	identityStore := store.NewDashboardIdentityStore(db)
	authService := dashboardauth.NewService(identityStore)
	states := map[string]string{}
	now := time.Now().UTC()
	const fixtureStartsAt = "2026-08-27T00:00:00Z"
	const fixtureExpiresAt = "2027-08-27T00:00:00Z"
	for index, state := range []string{"valid", "expired", "activated", "revoked"} {
		key := datasetID + "-ACTIVATION-" + strings.ToUpper(state)
		result, err := provisioning.ProvisionDashboardTenant(ctx, actor, dashboardadmin.ProvisionDashboardTenant{
			TenantName: datasetID + " 激活状态 " + state + "（非生产）", PackageID: acceptancePackageID, Limits: acceptanceActivationLimits(),
			Subscription:         dashboardadmin.SubscriptionInput{PackageCode: acceptancePackageCode, Status: "trialing", BillingCycle: "custom", StartsAt: fixtureStartsAt, ExpiresAt: fixtureExpiresAt},
			AdminLoginIdentifier: fmt.Sprintf("1900820827%d", index+1), AdminName: datasetID + " 激活管理员 " + state,
			IdempotencyKey: key, ExpectedVersion: 1, RequestID: key,
		})
		if err != nil {
			return nil, fmt.Errorf("provision %s activation fixture: %w", state, err)
		}
		tokenPath := filepath.Join(fixtureDir, state+".token")
		token := result.ActivationToken
		if token != "" {
			if err := writeAcceptanceSecretFile(tokenPath, token); err != nil {
				return nil, err
			}
		} else {
			token, err = readAcceptanceSecretFile(tokenPath)
			if err != nil {
				return nil, fmt.Errorf("activation fixture %s was replayed but its local token file is unavailable; run cleanup before reseeding: %w", state, err)
			}
		}
		digest := sha256.Sum256([]byte(token))
		status, err := identityStore.DashboardActivationStatus(ctx, digest, now)
		if err != nil {
			return nil, err
		}
		switch state {
		case "expired":
			if status.Status == dashboardauth.ActivationStatusValid {
				if _, err := db.ExecContext(ctx, `UPDATE mochat_go_dashboard_identity_activations SET expires_at=DATE_SUB(NOW(),INTERVAL 1 HOUR) WHERE user_id=? AND token_digest=? AND consumed_at IS NULL`, result.DashboardUserID, digest[:]); err != nil {
					return nil, err
				}
			}
		case "activated":
			if status.Status == dashboardauth.ActivationStatusValid {
				if err := authService.Activate(ctx, digest, datasetID+"-Activated-Password-Only-Local!42"); err != nil {
					return nil, err
				}
			}
		case "revoked":
			if status.Status == dashboardauth.ActivationStatusValid {
				var bindingVersion uint64
				if err := db.QueryRowContext(ctx, `SELECT version FROM mochat_go_tenant_corp_bindings WHERE tenant_id=? AND corp_id=?`, result.TenantID, result.BindingCorpID).Scan(&bindingVersion); err != nil {
					return nil, err
				}
				resent, err := provisioning.ResendActivation(ctx, actor, dashboardadmin.ResendActivationInput{TenantID: result.TenantID, TargetUserID: result.DashboardUserID, ExpectedVersion: bindingVersion, RequestID: key + "-RESEND"})
				if err != nil {
					return nil, err
				}
				if resent.ActivationToken != "" {
					if err := writeAcceptanceSecretFile(filepath.Join(fixtureDir, "revoked-replacement.token"), resent.ActivationToken); err != nil {
						return nil, err
					}
				}
			}
		}
		status, err = identityStore.DashboardActivationStatus(ctx, digest, time.Now().UTC())
		if err != nil {
			return nil, err
		}
		if string(status.Status) != state {
			return nil, fmt.Errorf("activation fixture %s has status %s", state, status.Status)
		}
		states[state] = string(status.Status)
	}
	invalidToken := datasetID + "-INVALID-ACTIVATION-TOKEN"
	if err := writeAcceptanceSecretFile(filepath.Join(fixtureDir, "invalid.token"), invalidToken); err != nil {
		return nil, err
	}
	invalidStatus, err := identityStore.DashboardActivationStatus(ctx, sha256.Sum256([]byte(invalidToken)), time.Now().UTC())
	if err != nil {
		return nil, err
	}
	if invalidStatus.Status != dashboardauth.ActivationStatusInvalid {
		return nil, errors.New("invalid activation fixture unexpectedly resolved")
	}
	states["invalid"] = string(invalidStatus.Status)
	return states, nil
}

func acceptanceActivationLimits() dashboardadmin.SaaSAdminPackageLimits {
	return dashboardadmin.SaaSAdminPackageLimits{
		MaxCorps: 10, MaxUsers: 10, MaxContacts: 100, MaxRooms: 10, MaxAgents: 10,
		ChannelCodes: 10, ShopCodes: 10, Radars: 10, Lotteries: 10, RoomInfinitePulls: 10,
		RoomFissions: 10, RoomClockIns: 10, RoomQualities: 10, RoomCalendars: 10, RoomReminds: 10,
		ContactSOPs: 10, RoomSOPs: 10, SensitiveWords: 10, StorageMB: 100, ContactMessageBatches: 10,
		RoomMessageBatches: 10, RoomTagPulls: 10, WorkRoomAutoPulls: 10, WorkFissions: 10, OfficialAccounts: 10, AsyncExecutions: 10,
	}
}

func writeAcceptanceSecretFile(path, value string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		return fmt.Errorf("write local acceptance secret file: %w", err)
	}
	return nil
}

func readAcceptanceSecretFile(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(raw))
	if value == "" {
		return "", errors.New("local acceptance secret file is empty")
	}
	return value, nil
}

type acceptanceCounts struct {
	Runs     int `json:"runs"`
	Messages int `json:"messages"`
	Media    int `json:"media"`
	Ready    int `json:"ready"`
	Missing  int `json:"missing"`
	Corrupt  int `json:"corrupt"`
}

func collectCounts(ctx context.Context, db *sql.DB) (acceptanceCounts, error) {
	var result acceptanceCounts
	queries := []struct {
		target *int
		query  string
		args   []any
	}{
		{&result.Runs, `SELECT COUNT(*) FROM mochat_go_archive_sync_runs WHERE tenant_id=? AND corp_id=? AND source_id=? AND idempotency_key=?`, []any{acceptanceTenant, acceptanceCorp, acceptanceSource, acceptanceRunKey}},
		{&result.Messages, `SELECT COUNT(*) FROM mochat_go_archive_message_sources WHERE tenant_id=? AND corp_id=? AND msgid LIKE ?`, []any{acceptanceTenant, acceptanceCorp, datasetID + "-MSG-%"}},
		{&result.Media, `SELECT COUNT(*) FROM mochat_go_archive_media_objects WHERE tenant_id=? AND corp_id=? AND msgid LIKE ?`, []any{acceptanceTenant, acceptanceCorp, datasetID + "-MSG-%"}},
		{&result.Ready, `SELECT COUNT(*) FROM mochat_go_archive_media_objects WHERE tenant_id=? AND corp_id=? AND msgid LIKE ? AND status='ready'`, []any{acceptanceTenant, acceptanceCorp, datasetID + "-MSG-%"}},
		{&result.Missing, `SELECT COUNT(*) FROM mochat_go_archive_media_objects WHERE tenant_id=? AND corp_id=? AND msgid LIKE ? AND status='missing'`, []any{acceptanceTenant, acceptanceCorp, datasetID + "-MSG-%"}},
		{&result.Corrupt, `SELECT COUNT(*) FROM mochat_go_archive_media_objects WHERE tenant_id=? AND corp_id=? AND msgid LIKE ? AND status='corrupt'`, []any{acceptanceTenant, acceptanceCorp, datasetID + "-MSG-%"}},
	}
	for _, item := range queries {
		if err := db.QueryRowContext(ctx, item.query, item.args...).Scan(item.target); err != nil {
			return result, err
		}
	}
	return result, nil
}

func verify(ctx context.Context, output io.Writer, values options) error {
	db, _, err := openAcceptanceStore(ctx, values)
	if err != nil {
		return err
	}
	defer db.Close()
	counts, err := collectCounts(ctx, db)
	if err != nil {
		return err
	}
	if counts != (acceptanceCounts{Runs: 1, Messages: 10, Media: 7, Ready: 5, Missing: 1, Corrupt: 1}) {
		return fmt.Errorf("acceptance counts do not match contract: %+v", counts)
	}
	var cursor int64
	var status string
	if err := db.QueryRowContext(ctx, `SELECT cursor_sequence,status FROM mochat_go_archive_sync_runs WHERE tenant_id=? AND corp_id=? AND source_id=? AND idempotency_key=?`, acceptanceTenant, acceptanceCorp, acceptanceSource, acceptanceRunKey).Scan(&cursor, &status); err != nil {
		return err
	}
	if cursor != 10 || status != "succeeded" {
		return fmt.Errorf("archive run did not finish at cursor 10")
	}
	expectedTypes := []int{1, 2, 3, 4, 5, 6, 7, 2, 9, 100}
	for index, expectedType := range expectedTypes {
		seq := index + 1
		table := `mc_work_message_` + strconv.Itoa((seq-1)%10+1)
		var messageType int
		if err := db.QueryRowContext(ctx, `SELECT msg_type FROM `+table+` WHERE corp_id=? AND msgid=?`, acceptanceCorp, fmt.Sprintf("%s-MSG-%02d", datasetID, seq)).Scan(&messageType); err != nil {
			return err
		}
		if messageType != expectedType {
			return fmt.Errorf("archive message %d type=%d want=%d", seq, messageType, expectedType)
		}
	}
	var syncAuditCount, integrationCount, integrationAuditCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_archive_sync_audits WHERE tenant_id=? AND corp_id=? AND source_id=?`, acceptanceTenant, acceptanceCorp, acceptanceSource).Scan(&syncAuditCount); err != nil {
		return err
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_wecom_integrations WHERE tenant_id=? AND corp_id=? AND status='active' AND credential_ciphertext<>'' AND credential_key_id='acceptance-v1' AND verification_level=? AND JSON_CONTAINS(scope_json, JSON_QUOTE('archive.read')) AND ((mode='self_built' AND slot='current') OR (mode='third_party_delegated' AND slot='candidate'))`, acceptanceTenant, acceptanceCorp, dashboardadmin.WeComVerificationLocalContract).Scan(&integrationCount); err != nil {
		return err
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT action) FROM mochat_go_saas_admin_operation_logs WHERE tenant_id=? AND target_type='wecom_integration' AND action IN ('wecom.integration.acceptance.bootstrap','wecom.integration.candidate.save','wecom.integration.candidate.verify','wecom.integration.switch','wecom.integration.rollback')`, acceptanceTenant).Scan(&integrationAuditCount); err != nil {
		return err
	}
	if syncAuditCount == 0 || integrationCount != 2 || integrationAuditCount != 5 {
		return errors.New("archive audit or active integration contract failed")
	}
	activationStates, err := verifyActivationFixtures(ctx, db, values.ActivationFixtureDir)
	if err != nil {
		return err
	}
	password, err := readAcceptancePassword(values.DashboardPasswordFile)
	if err != nil {
		return err
	}
	var mediaID, mediaSHA string
	if err := db.QueryRowContext(ctx, `SELECT id,sha256 FROM mochat_go_archive_media_objects WHERE tenant_id=? AND corp_id=? AND status='ready' ORDER BY media_type,id LIMIT 1`, acceptanceTenant, acceptanceCorp).Scan(&mediaID, &mediaSHA); err != nil {
		return err
	}
	terminalMediaIDs := make(map[string]string, 2)
	rows, err := db.QueryContext(ctx, `SELECT id,status FROM mochat_go_archive_media_objects WHERE tenant_id=? AND corp_id=? AND msgid LIKE ? AND status IN ('missing','corrupt')`, acceptanceTenant, acceptanceCorp, datasetID+"-MSG-%")
	if err != nil {
		return err
	}
	for rows.Next() {
		var id, mediaStatus string
		if err := rows.Scan(&id, &mediaStatus); err != nil {
			_ = rows.Close()
			return err
		}
		terminalMediaIDs[mediaStatus] = id
	}
	if err := rows.Close(); err != nil {
		return err
	}
	dashboardEvidence, err := verifyDashboardMediaHTTP(ctx, http.DefaultClient, values.APIBaseURL, "19008208270", password, mediaID, mediaSHA, terminalMediaIDs)
	if err != nil {
		return err
	}
	var unsafeLocatorCount, invalidReady int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_archive_media_objects WHERE tenant_id=? AND corp_id=? AND (sdk_file_id_ciphertext LIKE ? OR storage_path LIKE ?)`, acceptanceTenant, acceptanceCorp, "%SDKFILE%", "%SDKFILE%").Scan(&unsafeLocatorCount); err != nil {
		return err
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_archive_media_objects WHERE tenant_id=? AND corp_id=? AND status='ready' AND (bytes_received<=0 OR CHAR_LENGTH(sha256)<>64 OR storage_path='')`, acceptanceTenant, acceptanceCorp).Scan(&invalidReady); err != nil {
		return err
	}
	if unsafeLocatorCount != 0 || invalidReady != 0 {
		return errors.New("archive media confidentiality or integrity contract failed")
	}
	rows, err = db.QueryContext(ctx, `SELECT storage_path,bytes_received,sha256 FROM mochat_go_archive_media_objects WHERE tenant_id=? AND corp_id=? AND status='ready'`, acceptanceTenant, acceptanceCorp)
	if err != nil {
		return err
	}
	readyFiles := 0
	for rows.Next() {
		var storedPath, expectedSHA string
		var expectedSize int64
		if err := rows.Scan(&storedPath, &expectedSize, &expectedSHA); err != nil {
			_ = rows.Close()
			return err
		}
		target, ok := datasetObjectPath(values.StorageRoot, storedPath)
		if !ok {
			_ = rows.Close()
			return errors.New("archive media storage path escaped acceptance root")
		}
		body, err := os.ReadFile(target)
		if err != nil {
			_ = rows.Close()
			return err
		}
		actualSHA := fmt.Sprintf("%x", sha256.Sum256(body))
		if int64(len(body)) != expectedSize || !strings.EqualFold(actualSHA, expectedSHA) {
			_ = rows.Close()
			return errors.New("archive media object file integrity mismatch")
		}
		readyFiles++
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if readyFiles != 5 {
		return fmt.Errorf("ready media object files=%d want=5", readyFiles)
	}
	for shard := 1; shard <= 10; shard++ {
		var leaked int
		query := `SELECT COUNT(*) FROM mc_work_message_` + strconv.Itoa(shard) + ` WHERE corp_id=? AND msgid LIKE ? AND (content LIKE ? OR content_text LIKE ?)`
		if err := db.QueryRowContext(ctx, query, acceptanceCorp, datasetID+"-MSG-%", "%SDKFILE%", "%SDKFILE%").Scan(&leaked); err != nil {
			return err
		}
		if leaked != 0 {
			return errors.New("SDK media locator leaked into message projection")
		}
	}
	return json.NewEncoder(output).Encode(map[string]any{"dataset": datasetID, "action": "verify", "status": "PASS", "cursor": cursor, "counts": counts, "messageTypes": expectedTypes, "syncAudits": syncAuditCount, "integrationAudits": integrationAuditCount, "activationFixtures": activationStates, "authorizedMediaReads": 1, "terminalMediaReads": map[string]int{"missing": http.StatusNotFound, "corrupt": http.StatusNotFound}, "dashboardMessages": dashboardEvidence.MessageCount, "dashboardMediaTypes": dashboardEvidence.MediaTypes, "dashboardMessageTypes": dashboardEvidence.MessageTypes, "dashboardTerminalMediaStatuses": dashboardEvidence.TerminalMediaStatuses, "production": false})
}

func verifyActivationFixtures(ctx context.Context, db *sql.DB, fixtureDir string) (map[string]string, error) {
	identityStore := store.NewDashboardIdentityStore(db)
	result := map[string]string{}
	for _, expected := range []string{"valid", "expired", "activated", "revoked", "invalid"} {
		token, err := readAcceptanceSecretFile(filepath.Join(fixtureDir, expected+".token"))
		if err != nil {
			return nil, err
		}
		status, err := identityStore.DashboardActivationStatus(ctx, sha256.Sum256([]byte(token)), time.Now().UTC())
		if err != nil {
			return nil, err
		}
		if string(status.Status) != expected {
			return nil, fmt.Errorf("activation fixture %s has status %s", expected, status.Status)
		}
		result[expected] = string(status.Status)
	}
	return result, nil
}

type dashboardProjectionEvidence struct {
	MessageCount          int
	MediaTypes            []string
	MessageTypes          []int
	TerminalMediaStatuses []string
}

func verifyDashboardMediaHTTP(ctx context.Context, client *http.Client, baseURL, loginIdentifier, password, mediaID, expectedSHA256 string, terminalMediaIDs map[string]string) (dashboardProjectionEvidence, error) {
	if client == nil {
		client = http.DefaultClient
	}
	mediaURL := strings.TrimRight(baseURL, "/") + "/dashboard/archive/media/" + mediaID + "/content"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaURL, nil)
	if err != nil {
		return dashboardProjectionEvidence{}, err
	}
	response, err := client.Do(request)
	if err != nil {
		return dashboardProjectionEvidence{}, err
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	_ = response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		return dashboardProjectionEvidence{}, fmt.Errorf("unauthenticated archive media returned status %d", response.StatusCode)
	}

	loginBody, err := json.Marshal(map[string]string{"phone": loginIdentifier, "password": password})
	if err != nil {
		return dashboardProjectionEvidence{}, err
	}
	request, err = http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/dashboard/user/auth", bytes.NewReader(loginBody))
	if err != nil {
		return dashboardProjectionEvidence{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err = client.Do(request)
	if err != nil {
		return dashboardProjectionEvidence{}, err
	}
	defer response.Body.Close()
	var envelope struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&envelope) != nil || strings.TrimSpace(envelope.Data.Token) == "" {
		return dashboardProjectionEvidence{}, fmt.Errorf("dashboard acceptance login returned status %d", response.StatusCode)
	}

	request, err = http.NewRequestWithContext(ctx, http.MethodGet, mediaURL, nil)
	if err != nil {
		return dashboardProjectionEvidence{}, err
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(envelope.Data.Token))
	response, err = client.Do(request)
	if err != nil {
		return dashboardProjectionEvidence{}, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	if err != nil {
		return dashboardProjectionEvidence{}, err
	}
	actual := fmt.Sprintf("%x", sha256.Sum256(body))
	if response.StatusCode != http.StatusOK || !strings.EqualFold(actual, strings.TrimSpace(expectedSHA256)) {
		return dashboardProjectionEvidence{}, fmt.Errorf("authenticated archive media integrity check failed with status %d", response.StatusCode)
	}
	for _, terminalStatus := range []string{"missing", "corrupt"} {
		terminalID := strings.TrimSpace(terminalMediaIDs[terminalStatus])
		if terminalID == "" {
			return dashboardProjectionEvidence{}, fmt.Errorf("dashboard acceptance fixture missing %s media id", terminalStatus)
		}
		request, err = http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/dashboard/archive/media/"+url.PathEscape(terminalID)+"/content", nil)
		if err != nil {
			return dashboardProjectionEvidence{}, err
		}
		request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(envelope.Data.Token))
		response, err = client.Do(request)
		if err != nil {
			return dashboardProjectionEvidence{}, err
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		_ = response.Body.Close()
		if response.StatusCode != http.StatusNotFound {
			return dashboardProjectionEvidence{}, fmt.Errorf("dashboard %s archive media returned status %d", terminalStatus, response.StatusCode)
		}
	}
	return verifyDashboardGlobalMessagesHTTP(ctx, client, baseURL, strings.TrimSpace(envelope.Data.Token))
}

func verifyDashboardGlobalMessagesHTTP(ctx context.Context, client *http.Client, baseURL, token string) (dashboardProjectionEvidence, error) {
	listURL := fmt.Sprintf("%s/dashboard/workMessage/toUsers?view=global&corpId=%d&page=1&pageSize=100", strings.TrimRight(baseURL, "/"), acceptanceCorp)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, listURL, nil)
	if err != nil {
		return dashboardProjectionEvidence{}, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := client.Do(request)
	if err != nil {
		return dashboardProjectionEvidence{}, err
	}
	defer response.Body.Close()
	var listEnvelope struct {
		Code int `json:"code"`
		Data struct {
			List []struct {
				ID              string `json:"id"`
				ArchiveSource   string `json:"archiveSource"`
				ArchiveSourceID string `json:"archiveSourceId"`
			} `json:"list"`
			Total int `json:"total"`
		} `json:"data"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&listEnvelope) != nil || listEnvelope.Code != http.StatusOK {
		return dashboardProjectionEvidence{}, fmt.Errorf("dashboard global message list returned status %d", response.StatusCode)
	}
	var anchorID string
	for _, item := range listEnvelope.Data.List {
		if strings.HasPrefix(item.ID, "msg:"+datasetID+"-MSG-") && item.ArchiveSource == "external" && strings.HasPrefix(item.ArchiveSourceID, "wecom:") {
			anchorID = item.ID
			break
		}
	}
	if listEnvelope.Data.Total <= 0 || anchorID == "" {
		return dashboardProjectionEvidence{}, errors.New("dashboard global message list does not expose the acceptance archive")
	}
	detailURL := strings.TrimRight(baseURL, "/") + "/dashboard/workMessage/detail?id=" + url.QueryEscape(anchorID)
	request, err = http.NewRequestWithContext(ctx, http.MethodGet, detailURL, nil)
	if err != nil {
		return dashboardProjectionEvidence{}, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err = client.Do(request)
	if err != nil {
		return dashboardProjectionEvidence{}, err
	}
	defer response.Body.Close()
	var detailEnvelope struct {
		Code int `json:"code"`
		Data struct {
			MessageTotal int `json:"messageTotal"`
			Messages     []struct {
				ID            string         `json:"id"`
				ArchiveSource string         `json:"archiveSource"`
				Type          int            `json:"type"`
				Content       map[string]any `json:"content"`
			} `json:"messages"`
		} `json:"data"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(&detailEnvelope) != nil || detailEnvelope.Code != http.StatusOK {
		return dashboardProjectionEvidence{}, fmt.Errorf("dashboard global message detail returned status %d", response.StatusCode)
	}
	foundTypes := map[string]bool{}
	foundMessageTypes := map[int]bool{}
	foundTerminalStatuses := map[string]bool{}
	for _, message := range detailEnvelope.Data.Messages {
		if !strings.HasPrefix(message.ID, "msg:"+datasetID+"-MSG-") || message.ArchiveSource != "external" {
			continue
		}
		foundMessageTypes[message.Type] = true
		media, _ := message.Content["media"].(map[string]any)
		mediaType, _ := media["type"].(string)
		status, _ := media["status"].(string)
		mediaURL, _ := media["url"].(string)
		if status == "ready" && strings.HasPrefix(mediaURL, "/dashboard/archive/media/") && strings.HasSuffix(mediaURL, "/content") {
			foundTypes[mediaType] = true
		}
		if status == "missing" || status == "corrupt" {
			foundTerminalStatuses[status] = true
		}
	}
	requiredTypes := []string{"image", "voice", "video", "file"}
	for _, mediaType := range requiredTypes {
		if !foundTypes[mediaType] {
			return dashboardProjectionEvidence{}, fmt.Errorf("dashboard global message detail missing ready %s projection", mediaType)
		}
	}
	requiredMessageTypes := []int{1, 2, 3, 4, 5, 9}
	for _, messageType := range requiredMessageTypes {
		if !foundMessageTypes[messageType] {
			return dashboardProjectionEvidence{}, fmt.Errorf("dashboard global message detail missing message type %d", messageType)
		}
	}
	requiredTerminalStatuses := []string{"missing", "corrupt"}
	for _, terminalStatus := range requiredTerminalStatuses {
		if !foundTerminalStatuses[terminalStatus] {
			return dashboardProjectionEvidence{}, fmt.Errorf("dashboard global message detail missing %s media projection", terminalStatus)
		}
	}
	if detailEnvelope.Data.MessageTotal < 10 || len(detailEnvelope.Data.Messages) < 10 {
		return dashboardProjectionEvidence{}, fmt.Errorf("dashboard global message detail count=%d messages=%d", detailEnvelope.Data.MessageTotal, len(detailEnvelope.Data.Messages))
	}
	return dashboardProjectionEvidence{MessageCount: detailEnvelope.Data.MessageTotal, MediaTypes: requiredTypes, MessageTypes: requiredMessageTypes, TerminalMediaStatuses: requiredTerminalStatuses}, nil
}

func cleanupStatements() []string {
	return []string{
		`DELETE FROM mochat_go_archive_media_objects WHERE tenant_id=? AND corp_id=? AND msgid LIKE ?`,
		`DELETE FROM mochat_go_archive_message_sources WHERE tenant_id=? AND corp_id=? AND msgid LIKE ?`,
		`DELETE FROM mochat_go_archive_sync_audits WHERE tenant_id=? AND corp_id=? AND source_id=? AND run_id IN (SELECT owned.id FROM mochat_go_archive_sync_runs owned WHERE owned.tenant_id=? AND owned.corp_id=? AND owned.source_id=? AND (owned.idempotency_key=? OR owned.idempotency_key LIKE ?))`,
		`DELETE FROM mochat_go_archive_sync_runs WHERE tenant_id=? AND corp_id=? AND source_id=? AND (idempotency_key=? OR idempotency_key LIKE ?)`,
		`DELETE FROM mochat_go_wecom_integrations WHERE tenant_id=? AND corp_id=? AND id=?`,
		`DELETE FROM mochat_go_tenant_corp_bindings WHERE tenant_id=? AND corp_id=?`,
		`DELETE FROM mochat_go_dashboard_sessions WHERE user_id=?`,
		`DELETE FROM mochat_go_dashboard_identity_activations WHERE user_id=?`,
		`DELETE FROM mochat_go_dashboard_identities WHERE user_id=? AND login_identifier=?`,
		`DELETE FROM mochat_go_saas_subscriptions WHERE tenant_id=? AND package_code=?`,
		`DELETE FROM mochat_go_saas_tenant_packages WHERE tenant_id=? AND package_code=?`,
		`DELETE FROM mochat_go_work_message_participant_identity WHERE corp_id=? AND msgid LIKE ?`,
		`DELETE FROM mc_work_contact WHERE id=? AND corp_id=? AND wx_external_userid=?`,
		`DELETE FROM mc_work_employee WHERE id=? AND corp_id=? AND wx_user_id=?`,
		`DELETE FROM mc_user WHERE id=? AND tenant_id=? AND name=?`,
		`DELETE FROM mc_corp WHERE id=? AND tenant_id=? AND name=?`,
		`DELETE FROM mc_tenant WHERE id=? AND name=?`,
		`DELETE FROM mochat_go_saas_admin_sessions WHERE user_id IN (SELECT id FROM mochat_go_saas_admin_users WHERE login_name=? AND bootstrap_request_key=?)`,
		`DELETE FROM mochat_go_saas_admin_mfa_challenges WHERE user_id IN (SELECT id FROM mochat_go_saas_admin_users WHERE login_name=? AND bootstrap_request_key=?)`,
		`DELETE FROM mochat_go_saas_admin_mfa_credentials WHERE user_id IN (SELECT id FROM mochat_go_saas_admin_users WHERE login_name=? AND bootstrap_request_key=?)`,
		`DELETE FROM mochat_go_saas_admin_users WHERE login_name=? AND bootstrap_request_key=?`,
	}
}

func cleanup(ctx context.Context, output io.Writer, values options) error {
	db, _, err := openAcceptanceStore(ctx, values)
	if err != nil {
		return err
	}
	defer db.Close()
	counts, err := collectCounts(ctx, db)
	if err != nil {
		return err
	}
	if values.DryRun {
		return json.NewEncoder(output).Encode(map[string]any{"dataset": datasetID, "action": "cleanup", "dryRun": true, "counts": counts})
	}
	paths, err := datasetStoragePaths(ctx, db)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for shard := 1; shard <= 10; shard++ {
		if _, err := tx.ExecContext(ctx, `DELETE FROM mc_work_message_`+strconv.Itoa(shard)+` WHERE corp_id=? AND msgid LIKE ?`, acceptanceCorp, datasetID+"-MSG-%"); err != nil {
			return err
		}
	}
	statements := cleanupStatements()
	arguments := [][]any{
		{acceptanceTenant, acceptanceCorp, datasetID + "-MSG-%"}, {acceptanceTenant, acceptanceCorp, datasetID + "-MSG-%"},
		{acceptanceTenant, acceptanceCorp, acceptanceSource, acceptanceTenant, acceptanceCorp, acceptanceSource, acceptanceRunKey, acceptanceWorkerRunLike},
		{acceptanceTenant, acceptanceCorp, acceptanceSource, acceptanceRunKey, acceptanceWorkerRunLike},
		{acceptanceTenant, acceptanceCorp, acceptanceIntegrationID}, {acceptanceTenant, acceptanceCorp},
		{acceptanceUser}, {acceptanceUser}, {acceptanceUser, "19008208270"},
		{acceptanceTenant, datasetID}, {acceptanceTenant, datasetID},
		{acceptanceCorp, datasetID + "-MSG-%"}, {acceptanceContact, acceptanceCorp, datasetID + "-EXTERNAL-01"},
		{acceptanceStaff, acceptanceCorp, datasetID + "-STAFF-01"}, {acceptanceUser, acceptanceTenant, datasetID + " 本地验收管理员"},
		{acceptanceCorp, acceptanceTenant, datasetID + " 本地验收企业（非生产）"}, {acceptanceTenant, datasetID + " 本地验收租户（非生产）"},
		{"mochat-local-acceptance-admin", datasetID + "-SAAS-ADMIN"},
		{"mochat-local-acceptance-admin", datasetID + "-SAAS-ADMIN"},
		{"mochat-local-acceptance-admin", datasetID + "-SAAS-ADMIN"},
		{"mochat-local-acceptance-admin", datasetID + "-SAAS-ADMIN"},
	}
	for index, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement, arguments[index]...); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	removed := 0
	for _, path := range paths {
		if target, ok := datasetObjectPath(values.StorageRoot, path); ok {
			if err := os.Remove(target); err == nil {
				removed++
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	if err := verifyCleanupOrphans(ctx, db, paths, values.StorageRoot); err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(map[string]any{"dataset": datasetID, "action": "cleanup", "status": "PASS", "objectsRemoved": removed, "countsBefore": counts})
}

func verifyCleanupOrphans(ctx context.Context, db *sql.DB, storagePaths []string, storageRoot string) error {
	checks := []struct {
		name  string
		query string
		args  []any
	}{
		{"media", `SELECT COUNT(*) FROM mochat_go_archive_media_objects WHERE tenant_id=? AND corp_id=? AND msgid LIKE ?`, []any{acceptanceTenant, acceptanceCorp, datasetID + "-MSG-%"}},
		{"sources", `SELECT COUNT(*) FROM mochat_go_archive_message_sources WHERE tenant_id=? AND corp_id=? AND msgid LIKE ?`, []any{acceptanceTenant, acceptanceCorp, datasetID + "-MSG-%"}},
		{"runs", `SELECT COUNT(*) FROM mochat_go_archive_sync_runs WHERE tenant_id=? AND corp_id=? AND source_id=? AND (idempotency_key=? OR idempotency_key LIKE ?)`, []any{acceptanceTenant, acceptanceCorp, acceptanceSource, acceptanceRunKey, acceptanceWorkerRunLike}},
		{"integration", `SELECT COUNT(*) FROM mochat_go_wecom_integrations WHERE tenant_id=? AND corp_id=?`, []any{acceptanceTenant, acceptanceCorp}},
		{"participant identities", `SELECT COUNT(*) FROM mochat_go_work_message_participant_identity WHERE corp_id=? AND msgid LIKE ?`, []any{acceptanceCorp, datasetID + "-MSG-%"}},
		{"tenant", `SELECT COUNT(*) FROM mc_tenant WHERE id=? AND name=?`, []any{acceptanceTenant, datasetID + " 本地验收租户（非生产）"}},
	}
	for _, check := range checks {
		var count int
		if err := db.QueryRowContext(ctx, check.query, check.args...).Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			return fmt.Errorf("cleanup left %d acceptance %s orphan(s)", count, check.name)
		}
	}
	for shard := 1; shard <= 10; shard++ {
		var count int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mc_work_message_`+strconv.Itoa(shard)+` WHERE corp_id=? AND msgid LIKE ?`, acceptanceCorp, datasetID+"-MSG-%").Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			return fmt.Errorf("cleanup left %d acceptance message shard orphan(s)", count)
		}
	}
	for _, storedPath := range storagePaths {
		target, ok := datasetObjectPath(storageRoot, storedPath)
		if !ok {
			return errors.New("cleanup storage path escaped acceptance root")
		}
		if _, err := os.Stat(target); err == nil {
			return fmt.Errorf("cleanup left acceptance media file %s", filepath.Base(target))
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func datasetStoragePaths(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT storage_path FROM mochat_go_archive_media_objects WHERE tenant_id=? AND corp_id=? AND msgid LIKE ? AND storage_path<>''`, acceptanceTenant, acceptanceCorp, datasetID+"-MSG-%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		result = append(result, path)
	}
	return result, rows.Err()
}

func readAcceptancePassword(path string) (string, error) {
	raw, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		return "", fmt.Errorf("read local Dashboard acceptance password file: %w", err)
	}
	password := strings.TrimRight(string(raw), "\r\n")
	if strings.TrimSpace(password) == "" {
		return "", errors.New("local Dashboard acceptance password file is empty")
	}
	return password, nil
}

func safeDatasetObjectPath(root, value string) bool {
	_, ok := datasetObjectPath(root, value)
	return ok
}

func datasetObjectPath(root, value string) (string, bool) {
	root = filepath.Clean(root)
	target := filepath.Clean(filepath.FromSlash(value))
	if !filepath.IsAbs(target) {
		target = filepath.Join(root, target)
	}
	relative, err := filepath.Rel(root, target)
	ok := err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && strings.HasPrefix(filepath.ToSlash(relative), "archive-media/")
	return target, ok
}

func redactError(err error) string {
	if err == nil {
		return ""
	}
	text := err.Error()
	upper := strings.ToUpper(text)
	for _, marker := range []string{"@TCP(", "SDKFILE", "BEARER"} {
		if strings.Contains(upper, marker) {
			return "acceptance command failed (sensitive detail redacted)"
		}
	}
	return text
}
