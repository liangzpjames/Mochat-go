package identitymigration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"

	"jiyi/mochat-go/internal/migration"
	"jiyi/mochat-go/internal/wecomcredentials"
)

const cutoverMigrationSource = "0131_identity_realms_single_corp_cutover"

type CutoverPreflightReport struct {
	MissingDashboardIdentityIDs []int64
	InvalidBindingTenantIDs     []int64
	DanglingBindingIDs          []int64
	MissingCorpCiphertextIDs    []int64
	UndecryptableCorpIDs        []int64
	MissingAgentCiphertextIDs   []int64
	UndecryptableAgentIDs       []int64
	MissingPrerequisiteFacts    []string
	LegacyPasswordColumnPresent bool
	AlreadyCompleted            bool
}

type CutoverResult struct {
	RequestID  string
	Idempotent bool
}

// ApplyCutover executes the immutable 0131 script on one pinned connection so
// its request, checksum, and target-schema session bindings cannot drift.
func ApplyCutover(ctx context.Context, db *sql.DB, options DatabaseOptions, upPath string) (CutoverResult, error) {
	if db == nil {
		return CutoverResult{}, errors.New("0131 cutover database is required")
	}
	if strings.TrimSpace(options.Schema) == "" || options.PlatformTenantID <= 0 || strings.TrimSpace(options.RequestID) == "" {
		return CutoverResult{}, errors.New("0131 cutover schema, platform tenant, and request are required")
	}
	report, err := PreflightCutover(ctx, db, options)
	if err != nil {
		return CutoverResult{}, err
	}
	if report.AlreadyCompleted {
		return CutoverResult{RequestID: options.RequestID, Idempotent: true}, nil
	}
	body, err := os.ReadFile(upPath)
	if err != nil {
		return CutoverResult{}, phaseFailure("cutover", "script_read")
	}
	checksum := sha256.Sum256(body)
	conn, err := db.Conn(ctx)
	if err != nil {
		return CutoverResult{}, phaseFailure("cutover", "connection")
	}
	defer conn.Close()
	if err := VerifyTargetSchemaOnConn(ctx, conn, options.Schema); err != nil {
		return CutoverResult{}, phaseFailure("preflight", "schema_target")
	}
	if _, err := conn.ExecContext(ctx, "SET @identity_0131_platform_tenant_id = ?, @identity_0131_request_id = CONVERT(? USING utf8mb4) COLLATE utf8mb4_unicode_ci, @identity_0131_script_checksum = ?", options.PlatformTenantID, options.RequestID, hex.EncodeToString(checksum[:])); err != nil {
		return CutoverResult{}, phaseFailure("preflight", "session_bind")
	}
	statements, err := migration.SplitSQLStatements(string(body))
	if err != nil {
		return CutoverResult{}, phaseFailure("cutover", "script_parse")
	}
	for _, statement := range statements {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			return CutoverResult{}, phaseFailureWithCause("cutover", "statement", 0, err)
		}
	}
	if err := finalizeCorpTenantConstraint(ctx, conn); err != nil {
		return CutoverResult{}, err
	}
	return CutoverResult{RequestID: options.RequestID}, nil
}

func finalizeCorpTenantConstraint(ctx context.Context, conn execer) error {
	var invalid int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM mc_corp WHERE tenant_id IS NULL`).Scan(&invalid); err != nil {
		return phaseFailure("cutover", "corp_tenant_validate")
	}
	if invalid != 0 {
		return phaseFailure("cutover", "corp_tenant_incomplete")
	}
	if _, err := conn.ExecContext(ctx, `ALTER TABLE mc_corp MODIFY COLUMN tenant_id int(10) unsigned NOT NULL DEFAULT 0`); err != nil {
		return phaseFailure("cutover", "corp_tenant_constraint")
	}
	var compatible int
	if err := conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema=DATABASE() AND table_name='mc_corp' AND column_name='tenant_id'
		  AND data_type='int' AND numeric_precision=10 AND column_type LIKE '%unsigned%' AND is_nullable='NO'
	`).Scan(&compatible); err != nil || compatible != 1 {
		return phaseFailure("verify", "corp_tenant_constraint")
	}
	return nil
}

func (r CutoverPreflightReport) SafeText() string {
	return fmt.Sprintf(
		"missing_dashboard_identity_ids=%v invalid_binding_tenant_ids=%v dangling_binding_ids=%v missing_corp_credential_copy_ids=%v undecryptable_corp_ids=%v missing_agent_credential_copy_ids=%v undecryptable_agent_ids=%v missing_prerequisite_facts=%v legacy_credential_column_present=%t already_completed=%t",
		r.MissingDashboardIdentityIDs,
		r.InvalidBindingTenantIDs,
		r.DanglingBindingIDs,
		r.MissingCorpCiphertextIDs,
		r.UndecryptableCorpIDs,
		r.MissingAgentCiphertextIDs,
		r.UndecryptableAgentIDs,
		r.MissingPrerequisiteFacts,
		r.LegacyPasswordColumnPresent,
		r.AlreadyCompleted,
	)
}

func (r CutoverPreflightReport) Validate() error {
	if r.AlreadyCompleted {
		return nil
	}
	if len(r.MissingPrerequisiteFacts) > 0 {
		return errors.New("0131 prerequisite migration facts are incomplete")
	}
	if len(r.MissingDashboardIdentityIDs) > 0 {
		return errors.New("0131 active user is missing dashboard identity")
	}
	if len(r.InvalidBindingTenantIDs) > 0 {
		return errors.New("0131 active tenant does not have exactly one binding")
	}
	if len(r.DanglingBindingIDs) > 0 {
		return errors.New("0131 dangling or cross-tenant binding")
	}
	if len(r.MissingCorpCiphertextIDs) > 0 || len(r.MissingAgentCiphertextIDs) > 0 {
		return errors.New("0131 credential ciphertext copy is missing")
	}
	if len(r.UndecryptableCorpIDs) > 0 || len(r.UndecryptableAgentIDs) > 0 {
		return errors.New("0131 credential ciphertext cannot be decrypted")
	}
	if !r.LegacyPasswordColumnPresent {
		return errors.New("0131 legacy password column is unavailable")
	}
	return nil
}

func PreflightCutover(ctx context.Context, db *sql.DB, options DatabaseOptions) (CutoverPreflightReport, error) {
	if db == nil {
		return CutoverPreflightReport{}, errors.New("0131 preflight database is required")
	}
	if options.PlatformTenantID <= 0 || strings.TrimSpace(options.RequestID) == "" {
		return CutoverPreflightReport{}, errors.New("0131 preflight platform tenant and request are required")
	}
	if err := VerifyTargetSchema(ctx, db, options.Schema); err != nil {
		return CutoverPreflightReport{}, phaseFailure("preflight", "schema_target")
	}
	return preflightCutover(ctx, db, options)
}

func preflightCutover(ctx context.Context, db queryer, options DatabaseOptions) (CutoverPreflightReport, error) {
	var report CutoverPreflightReport
	var err error
	report.AlreadyCompleted, err = cutoverAlreadyCompleted(ctx, db, options.RequestID)
	if err != nil {
		return report, phaseWrap("preflight", "cutover_ledger", err)
	}
	if report.AlreadyCompleted {
		return report, nil
	}
	report.MissingPrerequisiteFacts, err = missingCutoverPrerequisites(ctx, db)
	if err != nil {
		return report, phaseWrap("preflight", "prerequisites", err)
	}
	report.MissingDashboardIdentityIDs, err = missingDashboardIdentities(ctx, db, options.PlatformTenantID)
	if err != nil {
		return report, phaseWrap("preflight", "dashboard_identity", err)
	}
	report.InvalidBindingTenantIDs, report.DanglingBindingIDs, err = bindingFindings(ctx, db, options.PlatformTenantID)
	if err != nil {
		return report, phaseWrap("preflight", "tenant_binding", err)
	}
	if options.CredentialManager == nil {
		return report, phaseFailure("preflight", "credential_manager")
	}
	report.MissingCorpCiphertextIDs, report.UndecryptableCorpIDs, err = cutoverCorpCredentialFindings(ctx, db, options.CredentialManager)
	if err != nil {
		return report, phaseWrap("preflight", "corp_credentials", err)
	}
	report.MissingAgentCiphertextIDs, report.UndecryptableAgentIDs, err = cutoverAgentCredentialFindings(ctx, db, options.CredentialManager)
	if err != nil {
		return report, phaseWrap("preflight", "agent_credentials", err)
	}
	report.LegacyPasswordColumnPresent, err = columnExists(ctx, db, "mc_user", "password")
	if err != nil {
		return report, phaseWrap("preflight", "legacy_password_column", err)
	}
	if err := report.Validate(); err != nil {
		return report, &ConsistencyError{Report: PreflightReport{RepairClasses: []string{"CUTOVER_PREFLIGHT_FAILED"}}, Err: err}
	}
	return report, nil
}

func VerifyTargetSchemaOnConn(ctx context.Context, conn queryer, schema string) error {
	if conn == nil || strings.TrimSpace(schema) == "" {
		return errors.New("maintenance target schema is required")
	}
	var actual sql.NullString
	if err := conn.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&actual); err != nil || !actual.Valid || actual.String != strings.TrimSpace(schema) {
		return errors.New("database schema does not match the maintenance target")
	}
	return nil
}

func missingCutoverPrerequisites(ctx context.Context, db queryer) ([]string, error) {
	missing := make([]string, 0, 4)
	if count, err := tableCount(ctx, db, "mochat_go_schema_migrations"); err != nil {
		return nil, err
	} else if count != 1 {
		missing = append(missing, "standard_ledger")
	} else {
		var count int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_schema_migrations WHERE version IN ('0129_identity_realms_single_corp_schema','0130_identity_realms_single_corp_backfill')`).Scan(&count); err != nil {
			return nil, err
		}
		if count != 2 {
			missing = append(missing, "standard_0129_0130")
		}
	}
	if count, err := tableCount(ctx, db, "mochat_go_identity_migration_ledger"); err != nil {
		return nil, err
	} else if count != 1 {
		missing = append(missing, "backfill_ledger")
	} else {
		var count int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_identity_migration_ledger WHERE migration_name=? AND phase='backfill' AND status='success'`, migrationSource).Scan(&count); err != nil {
			return nil, err
		}
		if count != 1 {
			missing = append(missing, "backfill_success")
		}
	}
	if count, err := tableCount(ctx, db, "mochat_go_identity_migration_batches"); err != nil {
		return nil, err
	} else if count != 1 {
		missing = append(missing, "backfill_batch")
	} else {
		var count int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_identity_migration_batches WHERE migration_source=? AND status='completed'`, migrationSource).Scan(&count); err != nil {
			return nil, err
		}
		if count != 1 {
			missing = append(missing, "backfill_completed")
		}
	}
	return missing, nil
}

func missingDashboardIdentities(ctx context.Context, db queryer, platformTenantID int64) ([]int64, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT u.id
		FROM mc_user u
		LEFT JOIN mochat_go_dashboard_identities d ON d.user_id=u.id
		WHERE u.tenant_id<>? AND u.status=1 AND u.deleted_at IS NULL AND d.user_id IS NULL
		ORDER BY u.id`, platformTenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIDs(rows)
}

func bindingFindings(ctx context.Context, db queryer, platformTenantID int64) ([]int64, []int64, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT t.id
		FROM mc_tenant t
		LEFT JOIN mochat_go_tenant_corp_bindings b ON b.tenant_id=t.id
		WHERE t.id<>? AND t.status=1 AND t.deleted_at IS NULL
		GROUP BY t.id
		HAVING COUNT(b.tenant_id)<>1
		ORDER BY t.id`, platformTenantID)
	if err != nil {
		return nil, nil, err
	}
	invalid, scanErr := scanIDs(rows)
	rows.Close()
	if scanErr != nil {
		return nil, nil, scanErr
	}
	rows, err = db.QueryContext(ctx, `
		SELECT b.tenant_id
		FROM mochat_go_tenant_corp_bindings b
		LEFT JOIN mc_corp c ON c.id=b.corp_id
		WHERE c.id IS NULL OR c.tenant_id<>b.tenant_id
		ORDER BY b.tenant_id`)
	if err != nil {
		return nil, nil, err
	}
	dangling, scanErr := scanIDs(rows)
	rows.Close()
	return invalid, dangling, scanErr
}

func cutoverCorpCredentialFindings(ctx context.Context, db queryer, manager *wecomcredentials.Manager) ([]int64, []int64, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, tenant_id, COALESCE(wx_corpid,''), COALESCE(employee_secret,''), COALESCE(contact_secret,''), COALESCE(token,''), COALESCE(encoding_aes_key,''), COALESCE(chat_secret,''), COALESCE(CAST(wecom_credentials_ciphertext AS CHAR),''), COALESCE(wecom_credentials_key_id,'') FROM mc_corp ORDER BY id`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	missing, bad := make([]int64, 0), make([]int64, 0)
	for rows.Next() {
		var id, tenantID int64
		var wxCorpID, employeeSecret, contactSecret, token, aesKey, chatSecret, ciphertext, keyID string
		if err := rows.Scan(&id, &tenantID, &wxCorpID, &employeeSecret, &contactSecret, &token, &aesKey, &chatSecret, &ciphertext, &keyID); err != nil {
			return nil, nil, err
		}
		if !hasCorpCredentialPlaintext(employeeSecret, contactSecret, token, aesKey, chatSecret) && ciphertext == "" {
			continue
		}
		if ciphertext == "" || keyID == "" {
			missing = append(missing, id)
			continue
		}
		if _, err := manager.DecryptCorp(int(tenantID), wxCorpID, keyID, ciphertext); err != nil {
			bad = append(bad, id)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return missing, bad, nil
}

func cutoverAgentCredentialFindings(ctx context.Context, db queryer, manager *wecomcredentials.Manager) ([]int64, []int64, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, corp_id, COALESCE(wx_agent_id,''), COALESCE(wx_secret,''), COALESCE(CAST(wecom_credentials_ciphertext AS CHAR),''), COALESCE(wecom_credentials_key_id,'') FROM mc_work_agent ORDER BY id`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	missing, bad := make([]int64, 0), make([]int64, 0)
	for rows.Next() {
		var id, corpID int64
		var wxAgentID, secret, ciphertext, keyID string
		if err := rows.Scan(&id, &corpID, &wxAgentID, &secret, &ciphertext, &keyID); err != nil {
			return nil, nil, err
		}
		if !hasAgentCredentialPlaintext(secret) && ciphertext == "" {
			continue
		}
		if ciphertext == "" || keyID == "" {
			missing = append(missing, id)
			continue
		}
		if _, err := manager.DecryptAgent(int(corpID), wxAgentID, keyID, ciphertext); err != nil {
			bad = append(bad, id)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return missing, bad, nil
}

func cutoverAlreadyCompleted(ctx context.Context, db queryer, requestID string) (bool, error) {
	if count, err := tableCount(ctx, db, "mochat_go_identity_migration_ledger"); err != nil {
		return false, err
	} else if count == 0 {
		return false, nil
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_identity_migration_ledger WHERE migration_name=? AND request_id=? AND phase='cutover' AND status='success'`, cutoverMigrationSource, requestID).Scan(&count); err != nil {
		return false, err
	}
	return count == 1, nil
}

func tableCount(ctx context.Context, db queryer, table string) (int, error) {
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name=?`, table).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func columnExists(ctx context.Context, db queryer, table, column string) (bool, error) {
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name=?`, table, column).Scan(&count); err != nil {
		return false, err
	}
	return count == 1, nil
}

func scanIDs(rows *sql.Rows) ([]int64, error) {
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}

type LegacyCredentialRestoreResult struct {
	RequestID    string
	RowsRestored int
}

func RestoreLegacyCredentials(ctx context.Context, db *sql.DB, manager *wecomcredentials.Manager, requestID string) (LegacyCredentialRestoreResult, error) {
	if db == nil || manager == nil || strings.TrimSpace(requestID) == "" {
		return LegacyCredentialRestoreResult{}, errors.New("legacy credential restore requires database, manager, and request")
	}
	var rollbackBatchCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_identity_cutover_batches WHERE request_id=? AND status='rolled_back' AND script_checksum<>''`, requestID).Scan(&rollbackBatchCount); err != nil || rollbackBatchCount != 1 {
		return LegacyCredentialRestoreResult{}, errors.New("legacy credential restore requires a rolled back cutover")
	}
	if ok, err := columnExists(ctx, db, "mc_user", "password"); err != nil || !ok {
		return LegacyCredentialRestoreResult{}, errors.New("legacy credential restore requires the restored password column")
	}
	var missingCorpJournalRows, missingAgentJournalRows int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM mochat_go_identity_cutover_journal j
		LEFT JOIN mc_corp c ON c.id=CAST(j.entity_id AS UNSIGNED)
		WHERE j.request_id=? AND j.entity_type='corp_credentials' AND c.id IS NULL`, requestID).Scan(&missingCorpJournalRows); err != nil {
		return LegacyCredentialRestoreResult{}, errors.New("legacy credential restore journal validation failed")
	}
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM mochat_go_identity_cutover_journal j
		LEFT JOIN mc_work_agent a ON a.id=CAST(j.entity_id AS UNSIGNED)
		WHERE j.request_id=? AND j.entity_type='agent_credentials' AND a.id IS NULL`, requestID).Scan(&missingAgentJournalRows); err != nil {
		return LegacyCredentialRestoreResult{}, errors.New("legacy credential restore journal validation failed")
	}
	if missingCorpJournalRows != 0 || missingAgentJournalRows != 0 {
		return LegacyCredentialRestoreResult{}, errors.New("legacy credential restore journal entity is missing")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return LegacyCredentialRestoreResult{}, errors.New("legacy credential restore transaction failed")
	}
	defer rollbackQuietly(tx)
	result := LegacyCredentialRestoreResult{RequestID: requestID}
	corpRows, err := tx.QueryContext(ctx, `
		SELECT c.id, c.tenant_id, COALESCE(c.wx_corpid,''), COALESCE(c.employee_secret,''), COALESCE(c.contact_secret,''), COALESCE(c.token,''), COALESCE(c.encoding_aes_key,''), COALESCE(c.chat_secret,''), COALESCE(CAST(c.wecom_credentials_ciphertext AS CHAR),''), COALESCE(c.wecom_credentials_key_id,'')
		FROM mc_corp c
		INNER JOIN mochat_go_identity_cutover_journal j
			ON j.request_id=? AND j.entity_type='corp_credentials' AND CAST(j.entity_id AS UNSIGNED)=c.id
		ORDER BY c.id
		FOR UPDATE`, requestID)
	if err != nil {
		return LegacyCredentialRestoreResult{}, errors.New("legacy credential restore corp query failed")
	}
	type corpRestoreRow struct {
		ID, TenantID                                                                          int64
		WXCorpID, EmployeeSecret, ContactSecret, Token, AESKey, ChatSecret, Ciphertext, KeyID string
	}
	corps := make([]corpRestoreRow, 0)
	for corpRows.Next() {
		var row corpRestoreRow
		if err := corpRows.Scan(&row.ID, &row.TenantID, &row.WXCorpID, &row.EmployeeSecret, &row.ContactSecret, &row.Token, &row.AESKey, &row.ChatSecret, &row.Ciphertext, &row.KeyID); err != nil {
			corpRows.Close()
			return LegacyCredentialRestoreResult{}, errors.New("legacy credential restore corp scan failed")
		}
		corps = append(corps, row)
	}
	if err := corpRows.Err(); err != nil {
		corpRows.Close()
		return LegacyCredentialRestoreResult{}, errors.New("legacy credential restore corp query failed")
	}
	corpRows.Close()
	for _, row := range corps {
		if row.Ciphertext == "" || row.KeyID == "" {
			continue
		}
		credential, err := manager.DecryptCorp(int(row.TenantID), row.WXCorpID, row.KeyID, row.Ciphertext)
		if err != nil {
			return LegacyCredentialRestoreResult{}, errors.New("legacy credential restore corp decrypt failed")
		}
		if hasCorpCredentialPlaintext(row.EmployeeSecret, row.ContactSecret, row.Token, row.AESKey, row.ChatSecret) {
			if row.EmployeeSecret != credential.EmployeeSecret || row.ContactSecret != credential.ContactSecret || row.Token != credential.CallbackToken || row.AESKey != credential.EncodingAESKey || row.ChatSecret != credential.ChatSecret {
				return LegacyCredentialRestoreResult{}, errors.New("legacy credential restore found conflicting corp plaintext")
			}
			continue
		}
		if _, err := tx.ExecContext(ctx, `UPDATE mc_corp SET employee_secret=?, contact_secret=?, token=?, encoding_aes_key=?, chat_secret=? WHERE id=?`, credential.EmployeeSecret, credential.ContactSecret, credential.CallbackToken, credential.EncodingAESKey, credential.ChatSecret, row.ID); err != nil {
			return LegacyCredentialRestoreResult{}, errors.New("legacy credential restore corp update failed")
		}
		result.RowsRestored++
	}
	agentRows, err := tx.QueryContext(ctx, `
		SELECT a.id, a.corp_id, COALESCE(a.wx_agent_id,''), COALESCE(a.wx_secret,''), COALESCE(CAST(a.wecom_credentials_ciphertext AS CHAR),''), COALESCE(a.wecom_credentials_key_id,'')
		FROM mc_work_agent a
		INNER JOIN mochat_go_identity_cutover_journal j
			ON j.request_id=? AND j.entity_type='agent_credentials' AND CAST(j.entity_id AS UNSIGNED)=a.id
		ORDER BY a.id
		FOR UPDATE`, requestID)
	if err != nil {
		return LegacyCredentialRestoreResult{}, errors.New("legacy credential restore agent query failed")
	}
	type agentRestoreRow struct {
		ID, CorpID                           int64
		WXAgentID, Secret, Ciphertext, KeyID string
	}
	agents := make([]agentRestoreRow, 0)
	for agentRows.Next() {
		var row agentRestoreRow
		if err := agentRows.Scan(&row.ID, &row.CorpID, &row.WXAgentID, &row.Secret, &row.Ciphertext, &row.KeyID); err != nil {
			agentRows.Close()
			return LegacyCredentialRestoreResult{}, errors.New("legacy credential restore agent scan failed")
		}
		agents = append(agents, row)
	}
	if err := agentRows.Err(); err != nil {
		agentRows.Close()
		return LegacyCredentialRestoreResult{}, errors.New("legacy credential restore agent query failed")
	}
	agentRows.Close()
	for _, row := range agents {
		if row.Ciphertext == "" || row.KeyID == "" {
			continue
		}
		credential, err := manager.DecryptAgent(int(row.CorpID), row.WXAgentID, row.KeyID, row.Ciphertext)
		if err != nil {
			return LegacyCredentialRestoreResult{}, errors.New("legacy credential restore agent decrypt failed")
		}
		if strings.TrimSpace(row.Secret) != "" {
			if row.Secret != credential.WXSecret {
				return LegacyCredentialRestoreResult{}, errors.New("legacy credential restore found conflicting agent plaintext")
			}
			continue
		}
		if _, err := tx.ExecContext(ctx, `UPDATE mc_work_agent SET wx_secret=? WHERE id=?`, credential.WXSecret, row.ID); err != nil {
			return LegacyCredentialRestoreResult{}, errors.New("legacy credential restore agent update failed")
		}
		result.RowsRestored++
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mochat_go_identity_cutover_batches SET status='restored' WHERE request_id=? AND status IN ('completed','rolled_back')`, requestID); err != nil {
		return LegacyCredentialRestoreResult{}, errors.New("legacy credential restore batch update failed")
	}
	if err := tx.Commit(); err != nil {
		return LegacyCredentialRestoreResult{}, errors.New("legacy credential restore commit failed")
	}
	return result, nil
}
