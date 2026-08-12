package identitymigration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"jiyi/mochat-go/internal/migration"
	"jiyi/mochat-go/internal/wecomcredentials"
)

const migrationSource = "0130_identity_realms_single_corp_backfill"

const saasAdminUsersTable = "mochat_go_saas_admin_users"

var errPlatformTenantPreflight = errors.New("identity preflight platform tenant is invalid")

var requiredSaaSAdminUsersColumns = []string{
	"id",
	"login_name",
	"phone",
	"password_hash",
	"name",
	"status",
	"must_rotate_password",
	"auth_version",
	"mfa_required",
	"bootstrap_request_key",
	"created_at",
	"updated_at",
}

type DatabaseOptions struct {
	Schema            string
	PlatformTenantID  int64
	Mapping           MappingDocument
	RequestID         string
	ScriptChecksum    string
	CredentialManager *wecomcredentials.Manager
}

type BackfillResult struct {
	RequestID  string
	Idempotent bool
}

type CredentialEncryptionResult struct {
	RequestID        string
	CorpRowsWritten  int
	AgentRowsWritten int
	Idempotent       bool
}

type validatedBatchContract struct {
	PlatformTenantID int64
	Status           string
	MappingDigest    string
	ScriptChecksum   string
	PreflightStatus  string
	CredentialStatus string
	ActorStatus      string
	MigrationSource  string
}

func platformTenantGuardAllowed(platformTenantID, platformTenantCount, legacyUserCount, legacyTenantFactCount, danglingActorCount, activeBootstrapRootCount int64) bool {
	if platformTenantID <= 0 {
		return false
	}
	if platformTenantCount == 1 {
		return true
	}
	return platformTenantCount == 0 &&
		legacyUserCount == 0 &&
		legacyTenantFactCount == 0 &&
		danglingActorCount == 0 &&
		activeBootstrapRootCount == 1
}

func validatedBatchCompatible(existing, expected validatedBatchContract) bool {
	return existing == expected
}

// PhaseError is the only migration execution error that the maintenance CLI
// may expose beyond its generic failure line. Its fields are constrained to
// stable, non-sensitive phase labels.
type PhaseError struct {
	Phase string
	Label string
}

func (e *PhaseError) Error() string {
	if e == nil {
		return "identity migration phase=unknown label=unknown failed"
	}
	return fmt.Sprintf("identity migration phase=%s label=%s failed", safePhaseName(e.Phase), safePhaseName(e.Label))
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type execer interface {
	queryer
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func Preflight(ctx context.Context, db *sql.DB, options DatabaseOptions) (PreflightReport, error) {
	if db == nil {
		return PreflightReport{}, errors.New("identity preflight database is required")
	}
	return preflight(ctx, db, options)
}

func preflight(ctx context.Context, db queryer, options DatabaseOptions) (PreflightReport, error) {
	if options.PlatformTenantID <= 0 {
		return PreflightReport{}, errors.New("platform tenant id must be explicit and positive")
	}
	if strings.TrimSpace(options.Schema) != "" {
		var actualSchema sql.NullString
		if err := db.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&actualSchema); err != nil || !actualSchema.Valid || actualSchema.String != strings.TrimSpace(options.Schema) {
			return PreflightReport{}, errors.New("database schema does not match the maintenance target")
		}
	}
	saasIdentityTablePresent, err := inspectSaaSAdminUsersTable(ctx, db)
	if err != nil {
		return PreflightReport{}, err
	}
	if err := validatePlatformTenantContext(ctx, db, options.PlatformTenantID, saasIdentityTablePresent); err != nil {
		return PreflightReport{}, err
	}

	var report PreflightReport
	if report.ActiveDashboardUsers, report.DuplicateLoginUserIDs, report.InvalidContactIDs, err = businessContactFindings(ctx, db, options.PlatformTenantID); err != nil {
		return PreflightReport{}, err
	}
	var platformPhones map[string][]int64
	var invalidPlatformContacts []int64
	if report.DuplicatePlatformPhoneUserIDs, platformPhones, invalidPlatformContacts, err = platformContactFindings(ctx, db, options.PlatformTenantID); err != nil {
		return PreflightReport{}, err
	}
	report.InvalidContactIDs = appendUniqueIDs(report.InvalidContactIDs, invalidPlatformContacts...)
	if report.SaaSPhoneConflictUserIDs, err = saasContactConflicts(ctx, db, platformPhones, saasIdentityTablePresent); err != nil {
		return PreflightReport{}, err
	}
	if report.ZeroCorpTenantIDs, err = queryIDs(ctx, db, `
		SELECT t.id FROM mc_tenant t
		LEFT JOIN mc_corp c ON c.tenant_id=t.id AND c.deleted_at IS NULL
		WHERE t.status=1 AND t.deleted_at IS NULL
		GROUP BY t.id HAVING COUNT(c.id)=0 ORDER BY t.id`); err != nil {
		return PreflightReport{}, err
	}
	if report.MultiCorpTenantIDs, err = queryIDs(ctx, db, `
		SELECT c.tenant_id FROM mc_corp c
		INNER JOIN mc_tenant t ON t.id=c.tenant_id AND t.status=1 AND t.deleted_at IS NULL
		WHERE c.deleted_at IS NULL GROUP BY c.tenant_id HAVING COUNT(*)>1 ORDER BY c.tenant_id`); err != nil {
		return PreflightReport{}, err
	}
	if report.InactiveOrMissingTenantIDs, err = queryIDs(ctx, db, `
		SELECT DISTINCT u.tenant_id
		FROM mc_user u
		LEFT JOIN mc_tenant t ON t.id=u.tenant_id
		WHERE u.deleted_at IS NULL AND u.tenant_id<>? AND (t.id IS NULL OR t.status<>1 OR t.deleted_at IS NOT NULL)
		ORDER BY u.tenant_id`, options.PlatformTenantID); err != nil {
		return PreflightReport{}, err
	}
	if report.InactiveBusinessSuperAdminIDs, err = queryIDs(ctx, db, `
		SELECT u.id
		FROM mc_user u
		LEFT JOIN mc_tenant t ON t.id=u.tenant_id
		WHERE u.tenant_id<>? AND u.isSuperAdmin=1 AND (u.status<>1 OR u.deleted_at IS NOT NULL OR t.id IS NULL OR t.status<>1 OR t.deleted_at IS NOT NULL)
		ORDER BY u.id`, options.PlatformTenantID); err != nil {
		return PreflightReport{}, err
	}
	if report.CrossTenantRelationIDs, err = queryIDs(ctx, db, `
		SELECT ur.id
		FROM mc_rbac_user_role ur
		INNER JOIN mc_user u ON u.id=CAST(ur.user_id AS UNSIGNED)
		INNER JOIN mc_rbac_role r ON r.id=ur.role_id
		WHERE ur.deleted_at IS NULL AND u.tenant_id<>r.tenant_id
		UNION
		SELECT dr.user_id
		FROM mochat_go_dashboard_user_roles dr
		INNER JOIN mc_user u ON u.id=dr.user_id
		WHERE u.tenant_id<>dr.tenant_id
		UNION
		SELECT dp.user_id
		FROM mochat_go_dashboard_user_permissions dp
		INNER JOIN mc_user u ON u.id=dp.user_id
		WHERE u.tenant_id<>dp.tenant_id
		ORDER BY id`); err != nil {
		return PreflightReport{}, err
	}
	if report.DanglingTenantIDs, err = queryIDs(ctx, db, `
		SELECT DISTINCT u.tenant_id FROM mc_user u LEFT JOIN mc_tenant t ON t.id=u.tenant_id WHERE t.id IS NULL
		UNION SELECT DISTINCT c.tenant_id FROM mc_corp c LEFT JOIN mc_tenant t ON t.id=c.tenant_id WHERE t.id IS NULL
		UNION SELECT DISTINCT r.tenant_id FROM mc_rbac_role r LEFT JOIN mc_tenant t ON t.id=r.tenant_id WHERE t.id IS NULL
		ORDER BY tenant_id`); err != nil {
		return PreflightReport{}, err
	}
	if report.DanglingCorpIDs, err = queryIDs(ctx, db, `SELECT c.id FROM mc_corp c LEFT JOIN mc_tenant t ON t.id=c.tenant_id WHERE t.id IS NULL ORDER BY c.id`); err != nil {
		return PreflightReport{}, err
	}
	if report.DanglingUserIDs, err = queryIDs(ctx, db, `
		SELECT DISTINCT CAST(ur.user_id AS UNSIGNED) FROM mc_rbac_user_role ur LEFT JOIN mc_user u ON u.id=CAST(ur.user_id AS UNSIGNED)
		WHERE ur.deleted_at IS NULL AND u.id IS NULL ORDER BY CAST(ur.user_id AS UNSIGNED)`); err != nil {
		return PreflightReport{}, err
	}
	if report.NegativeTenantIDs, err = queryIDs(ctx, db, `
		SELECT tenant_id FROM mc_user WHERE CAST(tenant_id AS DECIMAL(20,0))<0 OR CAST(tenant_id AS DECIMAL(20,0))>4294967295
		UNION SELECT tenant_id FROM mc_corp WHERE CAST(tenant_id AS DECIMAL(20,0))<0 OR CAST(tenant_id AS DECIMAL(20,0))>4294967295
		UNION SELECT tenant_id FROM mc_rbac_role WHERE CAST(tenant_id AS DECIMAL(20,0))<0 OR CAST(tenant_id AS DECIMAL(20,0))>4294967295
		ORDER BY tenant_id`); err != nil {
		return PreflightReport{}, err
	}
	actorColumns, inventoryErr := actorInventory(ctx, db)
	if inventoryErr != nil {
		for _, column := range actorColumns {
			if _, known := actorColumnKind(column.Table + "." + column.Column); !known {
				report.UnknownActorColumns = append(report.UnknownActorColumns, column)
			}
		}
		return report, inventoryErr
	}
	actorQuery, actorArgs, err := buildActorReferenceQuery(actorColumns, options.PlatformTenantID, saasIdentityTablePresent)
	if err != nil {
		return PreflightReport{}, err
	}
	if report.UnmappedSaaSActorIDs, err = queryIDs(ctx, db, actorQuery, actorArgs...); err != nil {
		return PreflightReport{}, err
	}
	if options.CredentialManager != nil {
		if report.UndecryptableCorpIDs, report.UndecryptableAgentIDs, err = credentialFailures(ctx, db, options.CredentialManager); err != nil {
			return PreflightReport{}, err
		}
	} else {
		report.UndecryptableCorpIDs = []int64{-1}
	}

	ownership, err := corpOwnership(ctx, db)
	if err != nil {
		return PreflightReport{}, err
	}
	report.RepairClasses = repairClasses(report)
	if err := report.Validate(Options{PlatformTenantID: options.PlatformTenantID, Mapping: options.Mapping}); err != nil {
		return report, &ConsistencyError{Report: report, Err: err}
	}
	if len(options.Mapping.Entries) > 0 {
		if err := ValidateMappingOwnership(options.Mapping, ownership); err != nil {
			return report, &ConsistencyError{Report: report, Err: err}
		}
	}
	return report, nil
}

func validatePlatformTenantContext(ctx context.Context, db queryer, platformTenantID int64, saasIdentityTablePresent bool) error {
	platformTenantCount, err := queryCount(ctx, db, `
		SELECT COUNT(*) FROM mc_tenant
		WHERE id = ? AND status = 1 AND deleted_at IS NULL`, platformTenantID)
	if err != nil {
		return fmt.Errorf("%w: platform tenant inspection failed", errPlatformTenantPreflight)
	}
	legacyUserCount, err := queryCount(ctx, db, `SELECT COUNT(*) FROM mc_user`)
	if err != nil {
		return fmt.Errorf("%w: legacy user inspection failed", errPlatformTenantPreflight)
	}
	legacyTenantCount, err := queryCount(ctx, db, `SELECT COUNT(*) FROM mc_tenant`)
	if err != nil {
		return fmt.Errorf("%w: legacy tenant inspection failed", errPlatformTenantPreflight)
	}
	legacyCorpCount, err := queryCount(ctx, db, `SELECT COUNT(*) FROM mc_corp`)
	if err != nil {
		return fmt.Errorf("%w: legacy corp inspection failed", errPlatformTenantPreflight)
	}
	legacyRoleCount, err := queryCount(ctx, db, `SELECT COUNT(*) FROM mc_rbac_role`)
	if err != nil {
		return fmt.Errorf("%w: legacy role inspection failed", errPlatformTenantPreflight)
	}
	legacyTenantFactCount := int64(legacyTenantCount + legacyCorpCount + legacyRoleCount)
	danglingActorCount := int64(0)
	activeBootstrapRootCount := int64(0)
	if saasIdentityTablePresent {
		accessDangling, queryErr := queryCount(ctx, db, `
			SELECT COUNT(*)
			FROM mochat_go_saas_admin_user_access a
			LEFT JOIN mochat_go_saas_admin_users u ON u.id = a.user_id
			WHERE u.id IS NULL OR u.status <> 1`)
		if queryErr != nil {
			return fmt.Errorf("%w: SaaS access actor inspection failed", errPlatformTenantPreflight)
		}
		roleDangling, queryErr := queryCount(ctx, db, `
			SELECT COUNT(*)
			FROM mochat_go_saas_admin_user_roles ur
			LEFT JOIN mochat_go_saas_admin_users u ON u.id = ur.user_id
			WHERE u.id IS NULL OR u.status <> 1`)
		if queryErr != nil {
			return fmt.Errorf("%w: SaaS role actor inspection failed", errPlatformTenantPreflight)
		}
		danglingActorCount = int64(accessDangling + roleDangling)
		bootstrapRootCount, queryErr := queryCount(ctx, db, `
			SELECT COUNT(*)
			FROM mochat_go_saas_admin_users u
			WHERE u.status = 1
			  AND u.must_rotate_password = 0
			  AND u.mfa_required = 1
			  AND NULLIF(TRIM(COALESCE(u.bootstrap_request_key, '')), '') IS NOT NULL
			  AND EXISTS (
				SELECT 1
				FROM mochat_go_saas_admin_user_roles ur
				INNER JOIN mochat_go_saas_admin_roles r ON r.id = ur.role_id
				INNER JOIN mochat_go_saas_admin_role_permissions rp ON rp.role_id = r.id
				WHERE ur.user_id = u.id
				  AND r.code = 'platform_root'
				  AND r.status = 1
				  AND r.is_system = 1
				  AND rp.permission_code = '*'
			)`)
		if queryErr != nil {
			return fmt.Errorf("%w: SaaS bootstrap actor inspection failed", errPlatformTenantPreflight)
		}
		activeBootstrapRootCount = int64(bootstrapRootCount)
	}
	if !platformTenantGuardAllowed(platformTenantID, int64(platformTenantCount), int64(legacyUserCount), legacyTenantFactCount, danglingActorCount, activeBootstrapRootCount) {
		return errPlatformTenantPreflight
	}
	return nil
}

func inspectSaaSAdminUsersTable(ctx context.Context, db queryer) (bool, error) {
	var tableType sql.NullString
	err := db.QueryRowContext(ctx, `
		SELECT table_type
		FROM information_schema.tables
		WHERE table_schema=DATABASE() AND table_name=?`, saasAdminUsersTable).Scan(&tableType)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, errors.New("SaaS identity table preflight inspection failed")
	}
	if !tableType.Valid || tableType.String != "BASE TABLE" {
		return false, errors.New("SaaS identity table preflight schema failed")
	}
	rows, err := db.QueryContext(ctx, `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema=DATABASE() AND table_name=?
		ORDER BY ordinal_position`, saasAdminUsersTable)
	if err != nil {
		return false, errors.New("SaaS identity table preflight inspection failed")
	}
	defer rows.Close()
	columns := make([]string, 0, len(requiredSaaSAdminUsersColumns))
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			return false, errors.New("SaaS identity table preflight schema scan failed")
		}
		columns = append(columns, column)
	}
	if err := rows.Err(); err != nil {
		return false, errors.New("SaaS identity table preflight inspection failed")
	}
	if err := validateSaaSAdminUsersColumns(columns); err != nil {
		return false, fmt.Errorf("SaaS identity table preflight schema failed: %w", err)
	}
	return true, nil
}

func validateSaaSAdminUsersColumns(columns []string) error {
	available := make(map[string]struct{}, len(columns))
	for _, column := range columns {
		column = strings.TrimSpace(column)
		if column == "" {
			return errors.New("SaaS identity table contains an empty column")
		}
		if _, exists := available[column]; exists {
			return fmt.Errorf("SaaS identity table contains duplicate column %s", column)
		}
		available[column] = struct{}{}
	}
	missing := make([]string, 0)
	for _, required := range requiredSaaSAdminUsersColumns {
		if _, exists := available[required]; !exists {
			missing = append(missing, required)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("SaaS identity table is missing required columns: %s", strings.Join(missing, ","))
	}
	return nil
}

type contactRow struct {
	ID     int64
	Phone  string
	Status int64
}

func businessContactFindings(ctx context.Context, db queryer, platformTenantID int64) (int, []int64, []int64, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, COALESCE(phone,''), status FROM mc_user WHERE tenant_id<>? AND deleted_at IS NULL ORDER BY id`, platformTenantID)
	if err != nil {
		return 0, nil, nil, errors.New("dashboard contact preflight query failed")
	}
	defer rows.Close()
	contacts := make([]contactRow, 0)
	active := 0
	invalid := make([]int64, 0)
	for rows.Next() {
		var item contactRow
		if err := rows.Scan(&item.ID, &item.Phone, &item.Status); err != nil {
			return 0, nil, nil, errors.New("dashboard contact preflight scan failed")
		}
		if item.Status == 1 {
			active++
		}
		canonical, normalizeErr := NormalizePhone(item.Phone)
		if normalizeErr != nil || canonical != strings.TrimSpace(item.Phone) {
			if item.Status == 1 {
				invalid = append(invalid, item.ID)
			}
			continue
		}
		contacts = append(contacts, item)
	}
	if err := rows.Err(); err != nil {
		return 0, nil, nil, errors.New("dashboard contact preflight query failed")
	}
	return active, duplicateContactIDs(contacts), invalid, nil
}

func platformContactFindings(ctx context.Context, db queryer, platformTenantID int64) ([]int64, map[string][]int64, []int64, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT u.id, COALESCE(u.phone,''), u.status
		FROM mc_user u
		WHERE u.tenant_id=? AND u.deleted_at IS NULL AND u.status=1
		  AND (u.isSuperAdmin=1
		       OR EXISTS (SELECT 1 FROM mochat_go_saas_admin_user_access a WHERE a.user_id=u.id)
		       OR EXISTS (SELECT 1 FROM mochat_go_saas_admin_user_roles r WHERE r.user_id=u.id))
		ORDER BY u.id`, platformTenantID)
	if err != nil {
		return nil, nil, nil, errors.New("platform contact preflight query failed")
	}
	defer rows.Close()
	contacts := make([]contactRow, 0)
	invalid := make([]int64, 0)
	for rows.Next() {
		var item contactRow
		if err := rows.Scan(&item.ID, &item.Phone, &item.Status); err != nil {
			return nil, nil, nil, errors.New("platform contact preflight scan failed")
		}
		canonical, normalizeErr := NormalizePhone(item.Phone)
		if normalizeErr != nil || canonical != strings.TrimSpace(item.Phone) {
			invalid = append(invalid, item.ID)
			continue
		}
		item.Phone = canonical
		contacts = append(contacts, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, nil, errors.New("platform contact preflight query failed")
	}
	phones := make(map[string][]int64)
	for _, item := range contacts {
		phones[item.Phone] = append(phones[item.Phone], item.ID)
	}
	return duplicateContactIDs(contacts), phones, invalid, nil
}

func saasContactConflicts(ctx context.Context, db queryer, platformPhones map[string][]int64, saasIdentityTablePresent bool) ([]int64, error) {
	if !saasIdentityTablePresent || len(platformPhones) == 0 {
		return nil, nil
	}
	rows, err := db.QueryContext(ctx, `SELECT id, COALESCE(phone,'') FROM mochat_go_saas_admin_users WHERE TRIM(COALESCE(phone,''))<>'' ORDER BY id`)
	if err != nil {
		return nil, errors.New("SaaS contact preflight query failed")
	}
	defer rows.Close()
	conflicts := make([]int64, 0)
	for rows.Next() {
		var id int64
		var phone string
		if err := rows.Scan(&id, &phone); err != nil {
			return nil, errors.New("SaaS contact preflight scan failed")
		}
		canonical, normalizeErr := NormalizePhone(phone)
		if normalizeErr != nil {
			continue
		}
		for _, platformID := range platformPhones[canonical] {
			if platformID != id {
				conflicts = appendUniqueIDs(conflicts, platformID)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, errors.New("SaaS contact preflight query failed")
	}
	return conflicts, nil
}

func duplicateContactIDs(contacts []contactRow) []int64 {
	byPhone := make(map[string][]int64)
	for _, item := range contacts {
		canonical, err := NormalizePhone(item.Phone)
		if err != nil {
			continue
		}
		byPhone[canonical] = append(byPhone[canonical], item.ID)
	}
	duplicates := make([]int64, 0)
	for _, ids := range byPhone {
		if len(ids) > 1 {
			duplicates = append(duplicates, ids...)
		}
	}
	return appendUniqueIDs(nil, duplicates...)
}

func appendUniqueIDs(existing []int64, values ...int64) []int64 {
	seen := make(map[int64]struct{}, len(existing)+len(values))
	result := make([]int64, 0, len(existing)+len(values))
	for _, value := range append(append([]int64(nil), existing...), values...) {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func repairClasses(report PreflightReport) []string {
	classes := make([]string, 0, 8)
	if len(report.DuplicateLoginUserIDs) > 0 {
		classes = append(classes, "DASHBOARD_LOGIN_DUPLICATE")
	}
	if len(report.InvalidContactIDs) > 0 {
		classes = append(classes, "DASHBOARD_LOGIN_INVALID")
	}
	if len(report.DuplicatePlatformPhoneUserIDs) > 0 {
		classes = append(classes, "SAAS_PLATFORM_CONTACT_DUPLICATE")
	}
	if len(report.SaaSPhoneConflictUserIDs) > 0 {
		classes = append(classes, "SAAS_IDENTITY_CONTACT_CONFLICT")
	}
	if len(report.ZeroCorpTenantIDs) > 0 {
		classes = append(classes, "TENANT_CORP_MISSING")
	}
	if len(report.MultiCorpTenantIDs) > 0 {
		classes = append(classes, "TENANT_CORP_MAPPING_REQUIRED")
	}
	if len(report.DanglingTenantIDs) > 0 || len(report.DanglingCorpIDs) > 0 || len(report.DanglingUserIDs) > 0 {
		classes = append(classes, "HISTORICAL_REFERENCE_DANGLING")
	}
	if len(report.InactiveOrMissingTenantIDs) > 0 {
		classes = append(classes, "BUSINESS_TENANT_INVALID")
	}
	if len(report.InactiveBusinessSuperAdminIDs) > 0 {
		classes = append(classes, "BUSINESS_SUPERADMIN_INVALID")
	}
	if len(report.CrossTenantRelationIDs) > 0 {
		classes = append(classes, "CROSS_TENANT_RELATION")
	}
	if len(report.UnmappedSaaSActorIDs) > 0 {
		classes = append(classes, "SAAS_ACTOR_UNMAPPED")
	}
	if len(report.NegativeTenantIDs) > 0 {
		classes = append(classes, "TENANT_ID_OUT_OF_RANGE")
	}
	if len(report.UndecryptableCorpIDs) > 0 || len(report.UndecryptableAgentIDs) > 0 {
		classes = append(classes, "CREDENTIAL_DECRYPTION_FAILED")
	}
	if len(report.UnknownActorColumns) > 0 {
		classes = append(classes, "ACTOR_INVENTORY_UNKNOWN")
	}
	return classes
}

func actorInventory(ctx context.Context, db queryer) ([]ActorColumn, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT table_name, column_name
		FROM information_schema.columns
		WHERE table_schema=DATABASE()
		  AND (table_name LIKE 'mochat_go_saas_admin_%' OR table_name IN ('mochat_go_saas_idempotency_receipts', 'mochat_go_dashboard_identity_activations'))
		  AND (column_name LIKE '%user_id' OR column_name IN ('created_by','updated_by','assigned_by'))
		ORDER BY table_name, column_name`)
	if err != nil {
		return nil, errors.New("identity actor inventory query failed")
	}
	defer rows.Close()
	items := make([]ActorColumn, 0)
	for rows.Next() {
		var item ActorColumn
		if err := rows.Scan(&item.Table, &item.Column); err != nil {
			return nil, errors.New("identity actor inventory scan failed")
		}
		item.Kind, _ = actorColumnKind(item.Table + "." + item.Column)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.New("identity actor inventory query failed")
	}
	if err := ValidateActorSchemaInventory(items); err != nil {
		return items, err
	}
	return items, nil
}

func buildActorReferenceQuery(columns []ActorColumn, platformTenantID int64, saasIdentityTablePresent bool) (string, []any, error) {
	if platformTenantID <= 0 {
		return "", nil, errors.New("platform tenant id must be explicit and positive")
	}
	actors := make([]string, 0, len(columns))
	for _, column := range columns {
		if column.Kind != "" && column.Kind != ActorColumnKindActor {
			continue
		}
		key := column.Table + "." + column.Column
		kind, known := actorColumnKind(key)
		if !known || kind != ActorColumnKindActor {
			continue
		}
		actors = append(actors, "SELECT `"+column.Column+"` AS actor_id FROM `"+column.Table+"` WHERE `"+column.Column+"` > 0")
	}
	sort.Strings(actors)
	if len(actors) == 0 {
		return `SELECT CAST(NULL AS UNSIGNED) AS actor_id WHERE 1=0`, nil, nil
	}
	identityJoin := ""
	identityFilter := "(u.id IS NULL OR u.tenant_id<>? OR u.status<>1 OR u.deleted_at IS NOT NULL)"
	if saasIdentityTablePresent {
		identityJoin = "LEFT JOIN mochat_go_saas_admin_users s ON s.id=actors.actor_id\n"
		identityFilter = "s.id IS NULL AND " + identityFilter
	}
	return `SELECT DISTINCT actors.actor_id
FROM (` + strings.Join(actors, " UNION ") + `) actors
` + identityJoin + `LEFT JOIN mc_user u ON u.id=actors.actor_id
WHERE ` + identityFilter + `
ORDER BY actors.actor_id`, []any{platformTenantID}, nil
}

func corpOwnership(ctx context.Context, db queryer) (map[int64][]int64, error) {
	rows, err := db.QueryContext(ctx, `SELECT tenant_id, id FROM mc_corp WHERE deleted_at IS NULL ORDER BY tenant_id, id`)
	if err != nil {
		return nil, errors.New("corp ownership query failed")
	}
	defer rows.Close()
	ownership := make(map[int64][]int64)
	for rows.Next() {
		var tenantID, corpID int64
		if err := rows.Scan(&tenantID, &corpID); err != nil {
			return nil, errors.New("corp ownership scan failed")
		}
		ownership[tenantID] = append(ownership[tenantID], corpID)
	}
	return ownership, rows.Err()
}

func credentialFailures(ctx context.Context, db queryer, manager *wecomcredentials.Manager) ([]int64, []int64, error) {
	corpRows, err := db.QueryContext(ctx, `SELECT id, tenant_id, wx_corpid, COALESCE(wecom_credentials_key_id,''), COALESCE(CAST(wecom_credentials_ciphertext AS CHAR), '') FROM mc_corp WHERE wecom_credentials_ciphertext IS NOT NULL AND COALESCE(CAST(wecom_credentials_ciphertext AS CHAR), '')<>''`)
	if err != nil {
		return nil, nil, errors.New("credential preflight query failed")
	}
	defer corpRows.Close()
	badCorps := make([]int64, 0)
	for corpRows.Next() {
		var id, tenantID int64
		var wxCorpID, keyID, ciphertext string
		if err := corpRows.Scan(&id, &tenantID, &wxCorpID, &keyID, &ciphertext); err != nil {
			return nil, nil, errors.New("credential preflight scan failed")
		}
		if _, err := manager.DecryptCorp(int(tenantID), wxCorpID, keyID, ciphertext); err != nil {
			badCorps = append(badCorps, id)
		}
	}
	if err := corpRows.Err(); err != nil {
		return nil, nil, errors.New("credential preflight query failed")
	}
	agentRows, err := db.QueryContext(ctx, `SELECT id, corp_id, wx_agent_id, COALESCE(wecom_credentials_key_id,''), COALESCE(CAST(wecom_credentials_ciphertext AS CHAR), '') FROM mc_work_agent WHERE wecom_credentials_ciphertext IS NOT NULL AND COALESCE(CAST(wecom_credentials_ciphertext AS CHAR), '')<>''`)
	if err != nil {
		return nil, nil, errors.New("credential preflight query failed")
	}
	defer agentRows.Close()
	badAgents := make([]int64, 0)
	for agentRows.Next() {
		var id, corpID int64
		var wxAgentID, keyID, ciphertext string
		if err := agentRows.Scan(&id, &corpID, &wxAgentID, &keyID, &ciphertext); err != nil {
			return nil, nil, errors.New("credential preflight scan failed")
		}
		if _, err := manager.DecryptAgent(int(corpID), wxAgentID, keyID, ciphertext); err != nil {
			badAgents = append(badAgents, id)
		}
	}
	if err := agentRows.Err(); err != nil {
		return nil, nil, errors.New("credential preflight query failed")
	}
	return badCorps, badAgents, nil
}

func StageValidatedBatch(ctx context.Context, db *sql.DB, options DatabaseOptions, report PreflightReport) error {
	if db == nil {
		return errors.New("identity staging database is required")
	}
	if strings.TrimSpace(options.RequestID) == "" {
		return errors.New("identity staging request id is required")
	}
	if err := report.Validate(Options{PlatformTenantID: options.PlatformTenantID, Mapping: options.Mapping}); err != nil {
		return err
	}
	if !validScriptChecksum(options.ScriptChecksum) {
		return errors.New("identity staging script checksum is required")
	}
	if len(report.UnknownActorColumns) > 0 || ValidateActorSchemaInventory(report.UnknownActorColumns) != nil {
		return errors.New("identity actor inventory must be supplied by the verified schema scan")
	}
	ownership, err := corpOwnership(ctx, db)
	if err != nil {
		return err
	}
	if len(options.Mapping.Entries) > 0 {
		if err := ValidateMappingOwnership(options.Mapping, ownership); err != nil {
			return err
		}
	}
	digest, err := mappingDigest(options.Mapping)
	if err != nil {
		return err
	}
	for _, statement := range []string{
		`CREATE TABLE IF NOT EXISTS mochat_go_identity_migration_batches (request_id varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL, platform_tenant_id int(10) unsigned NOT NULL, status varchar(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL, mapping_digest char(64) NOT NULL DEFAULT '', script_checksum char(64) NOT NULL, preflight_status varchar(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'passed', credential_status varchar(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'verified', actor_inventory_status varchar(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'verified', migration_source varchar(96) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '0130_identity_realms_single_corp_backfill', PRIMARY KEY (request_id)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS mochat_go_identity_migration_corp_map (request_id varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL, tenant_id int(10) unsigned NOT NULL, corp_id int(10) unsigned NOT NULL, status varchar(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL, migration_source varchar(96) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '0130_identity_realms_single_corp_backfill', PRIMARY KEY (request_id, tenant_id), UNIQUE KEY uni_task8_stage_corp (request_id, corp_id)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return errors.New("identity staging schema creation failed")
		}
	}
	if err := validateStagingSchema(ctx, db); err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return errors.New("identity staging transaction failed")
	}
	defer rollbackQuietly(tx)
	if err := stageValidatedBatchRows(ctx, tx, options, digest); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return errors.New("identity staging commit failed")
	}
	return nil
}

func ApplyBackfill(ctx context.Context, db *sql.DB, options DatabaseOptions, upPath string) (BackfillResult, error) {
	if db == nil {
		return BackfillResult{}, errors.New("identity backfill database is required")
	}
	if strings.TrimSpace(options.RequestID) == "" {
		return BackfillResult{}, errors.New("identity backfill request id is required")
	}
	if strings.TrimSpace(options.Schema) != "" {
		if err := VerifyTargetSchema(ctx, db, options.Schema); err != nil {
			return BackfillResult{}, phaseFailure("preflight", "schema_target")
		}
	}
	body, err := os.ReadFile(upPath)
	if err != nil {
		return BackfillResult{}, errors.New("identity backfill migration file is unavailable")
	}
	scriptChecksum := sha256.Sum256(body)
	options.ScriptChecksum = hex.EncodeToString(scriptChecksum[:])
	if idempotent, err := completedBackfillLedger(ctx, db, options.RequestID, options.ScriptChecksum); err != nil {
		return BackfillResult{}, err
	} else if idempotent {
		return BackfillResult{RequestID: options.RequestID, Idempotent: true}, nil
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return BackfillResult{}, errors.New("identity backfill connection is unavailable")
	}
	defer conn.Close()
	if _, err := preflight(ctx, conn, options); err != nil {
		if errors.Is(err, errPlatformTenantPreflight) {
			return BackfillResult{}, phaseFailure("preflight", "platform_tenant")
		}
		return BackfillResult{}, phaseWrap("preflight", "consistency", err)
	}
	if err := StageValidatedBatchOnConn(ctx, conn, options); err != nil {
		return BackfillResult{}, phaseWrap("stage", "contract", err)
	}
	statements, err := migration.SplitSQLStatements(string(body))
	if err != nil {
		return BackfillResult{}, phaseFailure("backfill", "script_parse")
	}
	if _, err := conn.ExecContext(ctx, `SET @identity_0130_requested_request_id = ?`, options.RequestID); err != nil {
		return BackfillResult{}, phaseFailure("preflight", "request_binding")
	}
	for _, statement := range statements {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			return BackfillResult{}, phaseFailure(statementPhase(statement), statementLabel(statement))
		}
	}
	return BackfillResult{RequestID: options.RequestID}, nil
}

func StageValidatedBatchOnConn(ctx context.Context, conn execer, options DatabaseOptions) error {
	if conn == nil {
		return errors.New("identity staging connection is required")
	}
	if strings.TrimSpace(options.RequestID) == "" {
		return errors.New("identity staging request id is required")
	}
	if !validScriptChecksum(options.ScriptChecksum) {
		return errors.New("identity staging script checksum is required")
	}
	digest, err := mappingDigest(options.Mapping)
	if err != nil {
		return err
	}
	ownership, err := corpOwnership(ctx, conn)
	if err != nil {
		return err
	}
	if len(options.Mapping.Entries) > 0 {
		if err := ValidateMappingOwnership(options.Mapping, ownership); err != nil {
			return err
		}
	}
	for _, statement := range []string{
		`CREATE TABLE IF NOT EXISTS mochat_go_identity_migration_batches (request_id varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL, platform_tenant_id int(10) unsigned NOT NULL, status varchar(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL, mapping_digest char(64) NOT NULL DEFAULT '', script_checksum char(64) NOT NULL, preflight_status varchar(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'passed', credential_status varchar(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'verified', actor_inventory_status varchar(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'verified', migration_source varchar(96) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '0130_identity_realms_single_corp_backfill', PRIMARY KEY (request_id)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS mochat_go_identity_migration_corp_map (request_id varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL, tenant_id int(10) unsigned NOT NULL, corp_id int(10) unsigned NOT NULL, status varchar(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL, migration_source varchar(96) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '0130_identity_realms_single_corp_backfill', PRIMARY KEY (request_id, tenant_id), UNIQUE KEY uni_task8_stage_corp (request_id, corp_id)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	} {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			return errors.New("identity staging schema creation failed")
		}
	}
	if err := validateStagingSchema(ctx, conn); err != nil {
		return err
	}
	return stageValidatedBatchRows(ctx, conn, options, digest)
}

func stageValidatedBatchRows(ctx context.Context, conn execer, options DatabaseOptions, digest string) error {
	expected := validatedBatchContract{
		PlatformTenantID: options.PlatformTenantID,
		Status:           "validated",
		MappingDigest:    digest,
		ScriptChecksum:   options.ScriptChecksum,
		PreflightStatus:  "passed",
		CredentialStatus: "verified",
		ActorStatus:      "verified",
		MigrationSource:  migrationSource,
	}
	var existing validatedBatchContract
	err := conn.QueryRowContext(ctx, `
		SELECT platform_tenant_id, status, mapping_digest, script_checksum,
			preflight_status, credential_status, actor_inventory_status, migration_source
		FROM mochat_go_identity_migration_batches
		WHERE request_id = ?
		LIMIT 1
		FOR UPDATE
	`, options.RequestID).Scan(
		&existing.PlatformTenantID, &existing.Status, &existing.MappingDigest, &existing.ScriptChecksum,
		&existing.PreflightStatus, &existing.CredentialStatus, &existing.ActorStatus, &existing.MigrationSource,
	)
	if err == nil {
		if !validatedBatchCompatible(existing, expected) {
			return errors.New("identity staging batch conflict")
		}
		if err := validateStagedMappingRows(ctx, conn, options); err != nil {
			return err
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return errors.New("identity staging batch lookup failed")
	}
	var existingMappingCount int
	if err := conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM mochat_go_identity_migration_corp_map WHERE request_id = ?
	`, options.RequestID).Scan(&existingMappingCount); err != nil {
		return errors.New("identity staging mapping lookup failed")
	}
	if existingMappingCount != 0 {
		return errors.New("identity staging mapping conflict")
	}
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO mochat_go_identity_migration_batches
			(request_id, platform_tenant_id, status, mapping_digest, script_checksum, preflight_status, credential_status, actor_inventory_status)
		VALUES (?, ?, 'validated', ?, ?, 'passed', 'verified', 'verified')
	`, options.RequestID, options.PlatformTenantID, digest, options.ScriptChecksum); err != nil {
		return errors.New("identity staging batch conflict")
	}
	for _, entry := range options.Mapping.Entries {
		if _, err := conn.ExecContext(ctx, `
			INSERT INTO mochat_go_identity_migration_corp_map (request_id, tenant_id, corp_id, status)
			VALUES (?, ?, ?, 'validated')
		`, options.RequestID, entry.TenantID, entry.CorpID); err != nil {
			return errors.New("identity staging mapping conflict")
		}
	}
	return nil
}

func validateStagedMappingRows(ctx context.Context, conn queryer, options DatabaseOptions) error {
	expected := append([]CorpMapping(nil), options.Mapping.Entries...)
	sort.Slice(expected, func(i, j int) bool {
		if expected[i].TenantID == expected[j].TenantID {
			return expected[i].CorpID < expected[j].CorpID
		}
		return expected[i].TenantID < expected[j].TenantID
	})
	rows, err := conn.QueryContext(ctx, `
		SELECT tenant_id, corp_id, status, migration_source
		FROM mochat_go_identity_migration_corp_map
		WHERE request_id = ?
		ORDER BY tenant_id, corp_id
	`, options.RequestID)
	if err != nil {
		return errors.New("identity staging mapping lookup failed")
	}
	defer rows.Close()
	index := 0
	for rows.Next() {
		var tenantID, corpID int64
		var status, source string
		if err := rows.Scan(&tenantID, &corpID, &status, &source); err != nil {
			return errors.New("identity staging mapping lookup failed")
		}
		if index >= len(expected) || tenantID != expected[index].TenantID || corpID != expected[index].CorpID || status != "validated" || source != migrationSource {
			return errors.New("identity staging mapping conflict")
		}
		index++
	}
	if err := rows.Err(); err != nil {
		return errors.New("identity staging mapping lookup failed")
	}
	if index != len(expected) {
		return errors.New("identity staging mapping conflict")
	}
	return nil
}

func validateStagingSchema(ctx context.Context, db queryer) error {
	expectedTypes := map[string]map[string]string{
		"mochat_go_identity_migration_batches": {
			"request_id": "varchar(128)", "platform_tenant_id": "int(10) unsigned", "status": "varchar(16)",
			"mapping_digest": "char(64)", "script_checksum": "char(64)", "preflight_status": "varchar(16)",
			"credential_status": "varchar(16)", "actor_inventory_status": "varchar(16)", "migration_source": "varchar(96)",
		},
		"mochat_go_identity_migration_corp_map": {
			"request_id": "varchar(128)", "tenant_id": "int(10) unsigned", "corp_id": "int(10) unsigned",
			"status": "varchar(16)", "migration_source": "varchar(96)",
		},
	}
	rows, err := db.QueryContext(ctx, `
		SELECT table_name, column_name, column_type
		FROM information_schema.columns
		WHERE table_schema=DATABASE() AND table_name IN ('mochat_go_identity_migration_batches','mochat_go_identity_migration_corp_map')
		ORDER BY table_name, ordinal_position`)
	if err != nil {
		return errors.New("identity staging schema inspection failed")
	}
	defer rows.Close()
	actual := make(map[string]map[string]string)
	for rows.Next() {
		var table, column, columnType string
		if err := rows.Scan(&table, &column, &columnType); err != nil {
			return errors.New("identity staging schema inspection failed")
		}
		if actual[table] == nil {
			actual[table] = make(map[string]string)
		}
		actual[table][column] = columnType
	}
	if err := rows.Err(); err != nil {
		return errors.New("identity staging schema inspection failed")
	}
	for table, columns := range expectedTypes {
		if len(actual[table]) != len(columns) {
			return errors.New("identity staging schema column contract mismatch")
		}
		for column, expectedType := range columns {
			if actual[table][column] != expectedType {
				return errors.New("identity staging schema column type contract mismatch")
			}
		}
	}
	indexRows, err := db.QueryContext(ctx, `
		SELECT table_name, index_name, GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',')
		FROM information_schema.statistics
		WHERE table_schema=DATABASE() AND table_name IN ('mochat_go_identity_migration_batches','mochat_go_identity_migration_corp_map')
		  AND index_name IN ('PRIMARY','uni_task8_stage_corp')
		GROUP BY table_name, index_name`)
	if err != nil {
		return errors.New("identity staging schema index inspection failed")
	}
	defer indexRows.Close()
	indexes := make(map[string]string)
	for indexRows.Next() {
		var table, name, signature string
		if err := indexRows.Scan(&table, &name, &signature); err != nil {
			return errors.New("identity staging schema index inspection failed")
		}
		indexes[table+"."+name] = signature
	}
	if err := indexRows.Err(); err != nil {
		return errors.New("identity staging schema index inspection failed")
	}
	for key, expected := range map[string]string{
		"mochat_go_identity_migration_batches.PRIMARY":               "request_id",
		"mochat_go_identity_migration_corp_map.PRIMARY":              "request_id,tenant_id",
		"mochat_go_identity_migration_corp_map.uni_task8_stage_corp": "request_id,corp_id",
	} {
		if indexes[key] != expected {
			return errors.New("identity staging schema unique key contract mismatch")
		}
	}
	return nil
}

func completedBackfillLedger(ctx context.Context, db *sql.DB, requestID, scriptChecksum string) (bool, error) {
	var tableCount int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_name = 'mochat_go_identity_migration_ledger'
	`).Scan(&tableCount); err != nil {
		return false, errors.New("identity backfill ledger check failed")
	}
	if tableCount == 0 {
		return false, nil
	}
	var resultJSON []byte
	err := db.QueryRowContext(ctx, `
		SELECT result_json
		FROM mochat_go_identity_migration_ledger
		WHERE migration_name = ? AND request_id = ? AND phase = 'backfill' AND status = 'success'
		ORDER BY id DESC LIMIT 1
	`, migrationSource, requestID).Scan(&resultJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, errors.New("identity backfill ledger check failed")
	}
	var result struct {
		ScriptChecksum string `json:"scriptChecksum"`
	}
	if err := json.Unmarshal(resultJSON, &result); err != nil || !validScriptChecksum(result.ScriptChecksum) || result.ScriptChecksum != scriptChecksum {
		return false, errors.New("identity backfill ledger checksum does not match the migration script")
	}
	var completed int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM mochat_go_identity_migration_batches
		WHERE request_id = ? AND status = 'completed'
	`, requestID).Scan(&completed); err != nil || completed != 1 {
		return false, errors.New("identity backfill completed batch is missing")
	}
	return true, nil
}

func validScriptChecksum(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func hasCorpCredentialPlaintext(employeeSecret, contactSecret, callbackToken, encodingAESKey, chatSecret string) bool {
	return strings.TrimSpace(employeeSecret) != "" ||
		strings.TrimSpace(contactSecret) != "" ||
		strings.TrimSpace(callbackToken) != "" ||
		strings.TrimSpace(encodingAESKey) != "" ||
		strings.TrimSpace(chatSecret) != ""
}

func hasAgentCredentialPlaintext(secret string) bool {
	return strings.TrimSpace(secret) != ""
}

func credentialJournalBeforeJSON(ciphertext, keyID string) ([]byte, error) {
	digest := sha256.Sum256([]byte(ciphertext))
	return json.Marshal(struct {
		AfterCiphertextSHA256 string `json:"afterCiphertextSha256"`
		WrittenKeyID          string `json:"writtenKeyId"`
	}{AfterCiphertextSHA256: hex.EncodeToString(digest[:]), WrittenKeyID: keyID})
}

func phaseFailure(phase, label string) error {
	return &PhaseError{Phase: safePhaseName(phase), Label: safePhaseName(label)}
}

func phaseWrap(phase, label string, _ error) error {
	return phaseFailure(phase, label)
}

func safePhaseName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '_' && character != '-' {
			return "unknown"
		}
	}
	return value
}

func statementPhase(statement string) string {
	upper := strings.ToUpper(strings.TrimSpace(statement))
	switch {
	case strings.Contains(upper, "PLATFORM_TENANT"):
		return "preflight"
	case strings.Contains(upper, "FOREIGN KEY") && strings.Contains(upper, "SAAS_ADMIN_USER_ACCESS"):
		return "fk_access"
	case strings.Contains(upper, "FOREIGN KEY") && (strings.Contains(upper, "SAAS_ADMIN_USER_ROLES") || strings.Contains(upper, "RBAC_ROLE")):
		return "fk_role"
	case strings.Contains(upper, "IDENTITY_MIGRATION_LEDGER"):
		return "ledger"
	default:
		return "backfill"
	}
}

func statementLabel(statement string) string {
	upper := strings.ToUpper(strings.TrimSpace(statement))
	switch {
	case strings.Contains(upper, "PLATFORM_TENANT"):
		return "platform_tenant"
	case strings.Contains(upper, "FOREIGN KEY") && strings.Contains(upper, "SAAS_ADMIN_USER_ACCESS"):
		return "access_fk"
	case strings.Contains(upper, "FOREIGN KEY") && (strings.Contains(upper, "SAAS_ADMIN_USER_ROLES") || strings.Contains(upper, "RBAC_ROLE")):
		return "role_fk"
	case strings.Contains(upper, "IDENTITY_MIGRATION_LEDGER"):
		return "ledger"
	case strings.HasPrefix(upper, "CREATE TABLE"):
		return "schema_ddl"
	default:
		return "statement"
	}
}

func EncryptCredentials(ctx context.Context, db *sql.DB, manager *wecomcredentials.Manager, requestID string) (CredentialEncryptionResult, error) {
	if db == nil || manager == nil {
		return CredentialEncryptionResult{}, errors.New("credential encryption database and manager are required")
	}
	if strings.TrimSpace(requestID) == "" {
		return CredentialEncryptionResult{}, errors.New("credential encryption request id is required")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return CredentialEncryptionResult{}, errors.New("credential encryption transaction failed")
	}
	defer rollbackQuietly(tx)
	var batchStatus, batchChecksum string
	if err := tx.QueryRowContext(ctx, `SELECT status, script_checksum FROM mochat_go_identity_migration_batches WHERE request_id=? AND migration_source=? FOR UPDATE`, requestID, migrationSource).Scan(&batchStatus, &batchChecksum); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CredentialEncryptionResult{}, errors.New("credential encryption requires a completed identity backfill batch")
		}
		return CredentialEncryptionResult{}, errors.New("credential encryption batch query failed")
	}
	if batchStatus != "completed" {
		return CredentialEncryptionResult{}, errors.New("credential encryption requires a completed identity backfill batch")
	}
	if !validScriptChecksum(batchChecksum) {
		return CredentialEncryptionResult{}, errors.New("credential encryption batch script checksum is invalid")
	}
	var baseLedgerCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_identity_migration_ledger WHERE migration_name=? AND request_id=? AND phase='backfill' AND status='success'`, migrationSource, requestID).Scan(&baseLedgerCount); err != nil {
		return CredentialEncryptionResult{}, errors.New("credential encryption base ledger query failed")
	}
	if baseLedgerCount != 1 {
		return CredentialEncryptionResult{}, errors.New("credential encryption requires a successful identity backfill ledger")
	}
	var baseResultJSON []byte
	if err := tx.QueryRowContext(ctx, `SELECT result_json FROM mochat_go_identity_migration_ledger WHERE migration_name=? AND request_id=? AND phase='backfill' AND status='success' LIMIT 1`, migrationSource, requestID).Scan(&baseResultJSON); err != nil {
		return CredentialEncryptionResult{}, errors.New("credential encryption base ledger query failed")
	}
	var baseResult struct {
		ScriptChecksum string `json:"scriptChecksum"`
	}
	if err := json.Unmarshal(baseResultJSON, &baseResult); err != nil || baseResult.ScriptChecksum != batchChecksum || !validScriptChecksum(baseResult.ScriptChecksum) {
		return CredentialEncryptionResult{}, errors.New("credential encryption base ledger checksum does not match the completed batch")
	}
	var existingStatus string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM mochat_go_identity_migration_ledger WHERE migration_name=? AND request_id=? LIMIT 1`, migrationSource+"/encrypt-credentials", requestID).Scan(&existingStatus); err == nil {
		if existingStatus == "success" {
			if err := tx.Commit(); err != nil {
				return CredentialEncryptionResult{}, errors.New("credential encryption commit failed")
			}
			return CredentialEncryptionResult{RequestID: requestID, Idempotent: true}, nil
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return CredentialEncryptionResult{}, errors.New("credential encryption ledger query failed")
	}
	result := CredentialEncryptionResult{RequestID: requestID}
	corpRows, err := tx.QueryContext(ctx, `SELECT id, tenant_id, wx_corpid, COALESCE(employee_secret,''), COALESCE(contact_secret,''), COALESCE(token,''), COALESCE(encoding_aes_key,''), COALESCE(chat_secret,''), COALESCE(CAST(wecom_credentials_ciphertext AS CHAR),''), COALESCE(wecom_credentials_key_id,'') FROM mc_corp FOR UPDATE`)
	if err != nil {
		return CredentialEncryptionResult{}, errors.New("credential encryption corp query failed")
	}
	type corpCredentialRow struct {
		ID, TenantID                                                                                          int64
		WXCorpID, EmployeeSecret, ContactSecret, CallbackToken, EncodingAESKey, ChatSecret, Ciphertext, KeyID string
	}
	corpItems := make([]corpCredentialRow, 0)
	for corpRows.Next() {
		var id, tenantID int64
		var wxCorpID, employeeSecret, contactSecret, callbackToken, encodingAESKey, chatSecret, ciphertext, keyID string
		if err := corpRows.Scan(&id, &tenantID, &wxCorpID, &employeeSecret, &contactSecret, &callbackToken, &encodingAESKey, &chatSecret, &ciphertext, &keyID); err != nil {
			corpRows.Close()
			return CredentialEncryptionResult{}, errors.New("credential encryption corp scan failed")
		}
		corpItems = append(corpItems, corpCredentialRow{ID: id, TenantID: tenantID, WXCorpID: wxCorpID, EmployeeSecret: employeeSecret, ContactSecret: contactSecret, CallbackToken: callbackToken, EncodingAESKey: encodingAESKey, ChatSecret: chatSecret, Ciphertext: ciphertext, KeyID: keyID})
	}
	if err := corpRows.Err(); err != nil {
		corpRows.Close()
		return CredentialEncryptionResult{}, errors.New("credential encryption corp query failed")
	}
	corpRows.Close()
	for _, item := range corpItems {
		id, tenantID, wxCorpID := item.ID, item.TenantID, item.WXCorpID
		employeeSecret, contactSecret, callbackToken, encodingAESKey, chatSecret := item.EmployeeSecret, item.ContactSecret, item.CallbackToken, item.EncodingAESKey, item.ChatSecret
		ciphertext, keyID := item.Ciphertext, item.KeyID
		if ciphertext != "" {
			if _, err := manager.DecryptCorp(int(tenantID), wxCorpID, keyID, ciphertext); err != nil {
				return CredentialEncryptionResult{}, errors.New("credential encryption found invalid corp ciphertext")
			}
			continue
		}
		if !hasCorpCredentialPlaintext(employeeSecret, contactSecret, callbackToken, encodingAESKey, chatSecret) {
			continue
		}
		encoded, encryptedKeyID, err := manager.EncryptCorp(int(tenantID), wxCorpID, wecomcredentials.CorpCredential{EmployeeSecret: employeeSecret, ContactSecret: contactSecret, CallbackToken: callbackToken, EncodingAESKey: encodingAESKey, ChatSecret: chatSecret})
		if err != nil {
			return CredentialEncryptionResult{}, errors.New("credential encryption corp failed")
		}
		journalBefore, err := credentialJournalBeforeJSON(encoded, encryptedKeyID)
		if err != nil {
			return CredentialEncryptionResult{}, errors.New("credential encryption corp journal failed")
		}
		journalResult, err := tx.ExecContext(ctx, `INSERT INTO mochat_go_identity_migration_journal (request_id, entity_type, entity_id, before_json) VALUES (?, 'corp_credentials', ?, ?)`, requestID, fmt.Sprintf("%d", id), journalBefore)
		if err != nil {
			return CredentialEncryptionResult{}, errors.New("credential encryption corp journal write failed")
		}
		journalAffected, _ := journalResult.RowsAffected()
		if journalAffected != 1 {
			return CredentialEncryptionResult{}, errors.New("credential encryption corp journal conflict")
		}
		updated, err := tx.ExecContext(ctx, `UPDATE mc_corp SET wecom_credentials_ciphertext=?, wecom_credentials_key_id=? WHERE id=? AND (wecom_credentials_ciphertext IS NULL OR wecom_credentials_ciphertext='')`, encoded, encryptedKeyID, id)
		if err != nil {
			return CredentialEncryptionResult{}, errors.New("credential encryption corp write failed")
		}
		affected, _ := updated.RowsAffected()
		if affected != 1 {
			return CredentialEncryptionResult{}, errors.New("credential encryption corp write conflict")
		}
		result.CorpRowsWritten++
	}
	agentRows, err := tx.QueryContext(ctx, `SELECT id, corp_id, wx_agent_id, COALESCE(wx_secret,''), COALESCE(CAST(wecom_credentials_ciphertext AS CHAR),''), COALESCE(wecom_credentials_key_id,'') FROM mc_work_agent FOR UPDATE`)
	if err != nil {
		return CredentialEncryptionResult{}, errors.New("credential encryption agent query failed")
	}
	type agentCredentialRow struct {
		ID, CorpID                         int64
		AgentID, Secret, Ciphertext, KeyID string
	}
	agentItems := make([]agentCredentialRow, 0)
	for agentRows.Next() {
		var id, corpID int64
		var agentID, secret, ciphertext, keyID string
		if err := agentRows.Scan(&id, &corpID, &agentID, &secret, &ciphertext, &keyID); err != nil {
			agentRows.Close()
			return CredentialEncryptionResult{}, errors.New("credential encryption agent scan failed")
		}
		agentItems = append(agentItems, agentCredentialRow{ID: id, CorpID: corpID, AgentID: agentID, Secret: secret, Ciphertext: ciphertext, KeyID: keyID})
	}
	if err := agentRows.Err(); err != nil {
		agentRows.Close()
		return CredentialEncryptionResult{}, errors.New("credential encryption agent query failed")
	}
	agentRows.Close()
	for _, item := range agentItems {
		id, corpID, agentID := item.ID, item.CorpID, item.AgentID
		secret, ciphertext, keyID := item.Secret, item.Ciphertext, item.KeyID
		if ciphertext != "" {
			if _, err := manager.DecryptAgent(int(corpID), agentID, keyID, ciphertext); err != nil {
				return CredentialEncryptionResult{}, errors.New("credential encryption found invalid agent ciphertext")
			}
			continue
		}
		if !hasAgentCredentialPlaintext(secret) {
			continue
		}
		encoded, encryptedKeyID, err := manager.EncryptAgent(int(corpID), agentID, wecomcredentials.AgentCredential{WXSecret: secret})
		if err != nil {
			return CredentialEncryptionResult{}, errors.New("credential encryption agent failed")
		}
		journalBefore, err := credentialJournalBeforeJSON(encoded, encryptedKeyID)
		if err != nil {
			return CredentialEncryptionResult{}, errors.New("credential encryption agent journal failed")
		}
		journalResult, err := tx.ExecContext(ctx, `INSERT INTO mochat_go_identity_migration_journal (request_id, entity_type, entity_id, before_json) VALUES (?, 'agent_credentials', ?, ?)`, requestID, fmt.Sprintf("%d", id), journalBefore)
		if err != nil {
			return CredentialEncryptionResult{}, errors.New("credential encryption agent journal write failed")
		}
		journalAffected, _ := journalResult.RowsAffected()
		if journalAffected != 1 {
			return CredentialEncryptionResult{}, errors.New("credential encryption agent journal conflict")
		}
		updated, err := tx.ExecContext(ctx, `UPDATE mc_work_agent SET wecom_credentials_ciphertext=?, wecom_credentials_key_id=? WHERE id=? AND (wecom_credentials_ciphertext IS NULL OR wecom_credentials_ciphertext='')`, encoded, encryptedKeyID, id)
		if err != nil {
			return CredentialEncryptionResult{}, errors.New("credential encryption agent write failed")
		}
		affected, _ := updated.RowsAffected()
		if affected != 1 {
			return CredentialEncryptionResult{}, errors.New("credential encryption agent write conflict")
		}
		result.AgentRowsWritten++
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO mochat_go_identity_migration_ledger (migration_name, request_id, phase, status, result_json) VALUES (?, ?, 'encrypt-credentials', 'success', ?)`, migrationSource+"/encrypt-credentials", requestID, fmt.Sprintf(`{"corpRowsWritten":%d,"agentRowsWritten":%d}`, result.CorpRowsWritten, result.AgentRowsWritten)); err != nil {
		return CredentialEncryptionResult{}, errors.New("credential encryption ledger write failed")
	}
	if err := tx.Commit(); err != nil {
		return CredentialEncryptionResult{}, errors.New("credential encryption commit failed")
	}
	return result, nil
}

func queryIDs(ctx context.Context, db queryer, query string, args ...any) ([]int64, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.New("identity preflight query failed")
	}
	defer rows.Close()
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, errors.New("identity preflight scan failed")
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.New("identity preflight query failed")
	}
	return ids, nil
}

func queryCount(ctx context.Context, db queryer, query string, args ...any) (int, error) {
	var count int
	if err := db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, errors.New("identity preflight count query failed")
	}
	return count, nil
}

func mappingDigest(document MappingDocument) (string, error) {
	entries := append([]CorpMapping(nil), document.Entries...)
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].TenantID == entries[j].TenantID {
			return entries[i].CorpID < entries[j].CorpID
		}
		return entries[i].TenantID < entries[j].TenantID
	})
	body, err := json.Marshal(struct {
		Version int           `json:"version"`
		Entries []CorpMapping `json:"entries"`
	}{document.Version, entries})
	if err != nil {
		return "", errors.New("mapping digest cannot be created")
	}
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:]), nil
}

func rollbackQuietly(tx *sql.Tx) {
	if tx != nil {
		_ = tx.Rollback()
	}
}
