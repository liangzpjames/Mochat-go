package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/authjwt"
	"jiyi/mochat-go/internal/mysqlconn"
)

type bootstrapOptions struct {
	DSN                string
	Secret             string
	BatchFile          string
	TenantID           int
	TenantName         string
	Phone              string
	Password           string
	UserName           string
	RoleName           string
	PackageCode        string
	PackageName        string
	PackageDescription string
	PackageExpiresAt   string
	MaxCorps           int
	MaxUsers           int
	MaxContacts        int
	MaxRooms           int
	MaxAgents          int
	ChannelCodes       int
	ShopCodes          int
	Radars             int
	Lotteries          int
	RoomInfinitePulls  int
	RoomFissions       int
	RoomClockIns       int
	RoomQualities      int
	RoomCalendars      int
	RoomReminds        int
	ContactSOPs        int
	RoomSOPs           int
	SensitiveWords     int
	StorageMB          int
	ContactBatches     int
	RoomBatches        int
	RoomTagPulls       int
	WorkRoomAutoPulls  int
	WorkFissions       int
	OfficialAccounts   int
	AsyncExecutions    int
	ConfigCopyMode     string
	Timeout            time.Duration
}

type bootstrapResult struct {
	TenantID         int
	UserID           int
	RoleID           int
	MenuCount        int
	PackageCode      string
	UsageMetricCount int
	SeedVersionCount int
	ConfigCopyCount  int
}

type defaultCorpEmployeeProvision struct {
	TenantID          int
	UserID            int
	CorpName          string
	WXCorpID          string
	EmployeeMobile    string
	CorpLookupSQL     string
	EmployeeLookupSQL string
	EmployeeInsertSQL string
}

func defaultCorpEmployeeContract(options bootstrapOptions, userID int) defaultCorpEmployeeProvision {
	return defaultCorpEmployeeProvision{
		TenantID:          options.TenantID,
		UserID:            userID,
		CorpName:          strings.TrimSpace(options.TenantName) + "演示企业",
		WXCorpID:          fmt.Sprintf("fake_tenant_%d", options.TenantID),
		EmployeeMobile:    options.Phone,
		CorpLookupSQL:     `SELECT id FROM mc_corp WHERE tenant_id = ? AND deleted_at IS NULL ORDER BY id ASC LIMIT 1`,
		EmployeeLookupSQL: `SELECT id FROM mc_work_employee WHERE corp_id = ? AND log_user_id = ? AND deleted_at IS NULL ORDER BY id ASC LIMIT 1`,
		EmployeeInsertSQL: `INSERT INTO mc_work_employee (wx_user_id, corp_id, name, mobile, status, log_user_id, audit_status, created_at, updated_at, deleted_at) VALUES (?, ?, ?, ?, 1, ?, 1, NOW(), NOW(), NULL)`,
	}
}

type saasLimits struct {
	MaxCorps          int `json:"maxCorps"`
	MaxUsers          int `json:"maxUsers"`
	MaxContacts       int `json:"maxContacts"`
	MaxRooms          int `json:"maxRooms"`
	MaxAgents         int `json:"maxAgents"`
	ChannelCodes      int `json:"channelCodes"`
	ShopCodes         int `json:"shopCodes"`
	Radars            int `json:"radars"`
	Lotteries         int `json:"lotteries"`
	RoomInfinitePulls int `json:"roomInfinitePulls"`
	RoomFissions      int `json:"roomFissions"`
	RoomClockIns      int `json:"roomClockIns"`
	RoomQualities     int `json:"roomQualities"`
	RoomCalendars     int `json:"roomCalendars"`
	RoomReminds       int `json:"roomReminds"`
	ContactSOPs       int `json:"contactSops"`
	RoomSOPs          int `json:"roomSops"`
	SensitiveWords    int `json:"sensitiveWords"`
	StorageMB         int `json:"storageMb"`
	ContactBatches    int `json:"contactMessageBatches"`
	RoomBatches       int `json:"roomMessageBatches"`
	RoomTagPulls      int `json:"roomTagPulls"`
	AutoPulls         int `json:"workRoomAutoPulls"`
	WorkFissions      int `json:"workFissions"`
	OfficialAccounts  int `json:"officialAccounts"`
	AsyncExecutions   int `json:"asyncExecutions"`
}

func main() {
	options := bootstrapOptions{}
	flag.StringVar(&options.DSN, "dsn", os.Getenv("MOCHAT_MYSQL_DSN"), "MySQL DSN; defaults to MOCHAT_MYSQL_DSN")
	flag.StringVar(&options.Secret, "secret", firstEnv("MOCHAT_SIMPLE_JWT_SECRET", "SIMPLE_JWT_SECRET"), "password hash secret; defaults to MOCHAT_SIMPLE_JWT_SECRET or SIMPLE_JWT_SECRET")
	flag.StringVar(&options.BatchFile, "batch-file", os.Getenv("MOCHAT_BOOTSTRAP_BATCH_FILE"), "CSV batch file for provisioning multiple tenants")
	flag.IntVar(&options.TenantID, "tenant-id", intEnv("MOCHAT_BOOTSTRAP_TENANT_ID", 1), "tenant id to create or update")
	flag.StringVar(&options.TenantName, "tenant-name", envDefault("MOCHAT_BOOTSTRAP_TENANT_NAME", "默认租户"), "tenant name")
	flag.StringVar(&options.Phone, "phone", os.Getenv("MOCHAT_BOOTSTRAP_PHONE"), "admin phone; defaults to MOCHAT_BOOTSTRAP_PHONE")
	flag.StringVar(&options.Password, "password", os.Getenv("MOCHAT_BOOTSTRAP_PASSWORD"), "admin password; defaults to MOCHAT_BOOTSTRAP_PASSWORD")
	flag.StringVar(&options.UserName, "user-name", envDefault("MOCHAT_BOOTSTRAP_USER_NAME", "超级管理员"), "admin display name")
	flag.StringVar(&options.RoleName, "role-name", envDefault("MOCHAT_BOOTSTRAP_ROLE_NAME", "超级管理员"), "admin role name")
	flag.StringVar(&options.PackageCode, "package-code", envDefault("MOCHAT_BOOTSTRAP_PACKAGE_CODE", "standard"), "SaaS package code")
	flag.StringVar(&options.PackageName, "package-name", envDefault("MOCHAT_BOOTSTRAP_PACKAGE_NAME", "标准版"), "SaaS package name")
	flag.StringVar(&options.PackageDescription, "package-description", os.Getenv("MOCHAT_BOOTSTRAP_PACKAGE_DESCRIPTION"), "SaaS package description")
	flag.StringVar(&options.PackageExpiresAt, "package-expires-at", os.Getenv("MOCHAT_BOOTSTRAP_PACKAGE_EXPIRES_AT"), "package expiration time; accepts RFC3339, YYYY-MM-DD, or YYYY-MM-DD HH:MM:SS")
	flag.IntVar(&options.MaxCorps, "max-corps", intEnvAllowZero("MOCHAT_BOOTSTRAP_MAX_CORPS", 0), "package corp limit; 0 means unlimited")
	flag.IntVar(&options.MaxUsers, "max-users", intEnvAllowZero("MOCHAT_BOOTSTRAP_MAX_USERS", 0), "package user limit; 0 means unlimited")
	flag.IntVar(&options.MaxContacts, "max-contacts", intEnvAllowZero("MOCHAT_BOOTSTRAP_MAX_CONTACTS", 0), "package contact limit; 0 means unlimited")
	flag.IntVar(&options.MaxRooms, "max-rooms", intEnvAllowZero("MOCHAT_BOOTSTRAP_MAX_ROOMS", 0), "package room limit; 0 means unlimited")
	flag.IntVar(&options.MaxAgents, "max-agents", intEnvAllowZero("MOCHAT_BOOTSTRAP_MAX_AGENTS", 0), "package agent limit; 0 means unlimited")
	flag.IntVar(&options.ChannelCodes, "channel-codes", intEnvAllowZero("MOCHAT_BOOTSTRAP_CHANNEL_CODES", 0), "package channel code limit; 0 means unlimited")
	flag.IntVar(&options.ShopCodes, "shop-codes", intEnvAllowZero("MOCHAT_BOOTSTRAP_SHOP_CODES", 0), "package shop code limit; 0 means unlimited")
	flag.IntVar(&options.Radars, "radars", intEnvAllowZero("MOCHAT_BOOTSTRAP_RADARS", 0), "package radar limit; 0 means unlimited")
	flag.IntVar(&options.Lotteries, "lotteries", intEnvAllowZero("MOCHAT_BOOTSTRAP_LOTTERIES", 0), "package lottery activity limit; 0 means unlimited")
	flag.IntVar(&options.RoomInfinitePulls, "room-infinite-pulls", intEnvAllowZero("MOCHAT_BOOTSTRAP_ROOM_INFINITE_PULLS", 0), "package room infinite pull limit; 0 means unlimited")
	flag.IntVar(&options.RoomFissions, "room-fissions", intEnvAllowZero("MOCHAT_BOOTSTRAP_ROOM_FISSIONS", 0), "package room fission activity limit; 0 means unlimited")
	flag.IntVar(&options.RoomClockIns, "room-clock-ins", intEnvAllowZero("MOCHAT_BOOTSTRAP_ROOM_CLOCK_INS", 0), "package room clock-in activity limit; 0 means unlimited")
	flag.IntVar(&options.RoomQualities, "room-qualities", intEnvAllowZero("MOCHAT_BOOTSTRAP_ROOM_QUALITIES", 0), "package room quality rule limit; 0 means unlimited")
	flag.IntVar(&options.RoomCalendars, "room-calendars", intEnvAllowZero("MOCHAT_BOOTSTRAP_ROOM_CALENDARS", 0), "package room calendar limit; 0 means unlimited")
	flag.IntVar(&options.RoomReminds, "room-reminds", intEnvAllowZero("MOCHAT_BOOTSTRAP_ROOM_REMINDS", 0), "package room remind limit; 0 means unlimited")
	flag.IntVar(&options.ContactSOPs, "contact-sops", intEnvAllowZero("MOCHAT_BOOTSTRAP_CONTACT_SOPS", 0), "package contact SOP rule limit; 0 means unlimited")
	flag.IntVar(&options.RoomSOPs, "room-sops", intEnvAllowZero("MOCHAT_BOOTSTRAP_ROOM_SOPS", 0), "package room SOP rule limit; 0 means unlimited")
	flag.IntVar(&options.SensitiveWords, "sensitive-words", intEnvAllowZero("MOCHAT_BOOTSTRAP_SENSITIVE_WORDS", 0), "package sensitive word limit; 0 means unlimited")
	flag.IntVar(&options.StorageMB, "storage-mb", intEnvAllowZero("MOCHAT_BOOTSTRAP_STORAGE_MB", 0), "package storage limit in MB; 0 means unlimited")
	flag.IntVar(&options.ContactBatches, "contact-message-batches", intEnvAllowZero("MOCHAT_BOOTSTRAP_CONTACT_MESSAGE_BATCHES", 0), "package contact message batch task limit; 0 means unlimited")
	flag.IntVar(&options.RoomBatches, "room-message-batches", intEnvAllowZero("MOCHAT_BOOTSTRAP_ROOM_MESSAGE_BATCHES", 0), "package room message batch task limit; 0 means unlimited")
	flag.IntVar(&options.RoomTagPulls, "room-tag-pulls", intEnvAllowZero("MOCHAT_BOOTSTRAP_ROOM_TAG_PULLS", 0), "package room tag pull task limit; 0 means unlimited")
	flag.IntVar(&options.WorkRoomAutoPulls, "work-room-auto-pulls", intEnvAllowZero("MOCHAT_BOOTSTRAP_WORK_ROOM_AUTO_PULLS", 0), "package work room auto pull limit; 0 means unlimited")
	flag.IntVar(&options.WorkFissions, "work-fissions", intEnvAllowZero("MOCHAT_BOOTSTRAP_WORK_FISSIONS", 0), "package work fission activity limit; 0 means unlimited")
	flag.IntVar(&options.OfficialAccounts, "official-accounts", intEnvAllowZero("MOCHAT_BOOTSTRAP_OFFICIAL_ACCOUNTS", 0), "package official account authorization limit; 0 means unlimited")
	flag.IntVar(&options.AsyncExecutions, "async-executions", intEnvAllowZero("MOCHAT_BOOTSTRAP_ASYNC_EXECUTIONS", 0), "package async queue execution limit; 0 means unlimited")
	flag.StringVar(&options.ConfigCopyMode, "config-copy-mode", envDefault("MOCHAT_BOOTSTRAP_CONFIG_COPY_MODE", "missing"), "tenant default config copy mode: missing, overwrite, or skip")
	flag.DurationVar(&options.Timeout, "timeout", 2*time.Minute, "bootstrap timeout")
	flag.Parse()

	provisions, err := buildProvisionOptions(options)
	if err != nil {
		log.Fatal(err)
	}
	db, err := mysqlconn.Open(options.DSN)
	if err != nil {
		log.Fatalf("open mysql: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("ping mysql: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), options.Timeout)
	defer cancel()

	for _, provision := range provisions {
		result, err := bootstrap(ctx, db, provision)
		if err != nil {
			log.Fatalf("bootstrap tenant %d failed: %v", provision.TenantID, err)
		}
		printResult(result)
	}
}

func printResult(result bootstrapResult) {
	fmt.Printf("tenant_id\t%d\n", result.TenantID)
	fmt.Printf("user_id\t%d\n", result.UserID)
	fmt.Printf("role_id\t%d\n", result.RoleID)
	fmt.Printf("role_menu_count\t%d\n", result.MenuCount)
	fmt.Printf("package_code\t%s\n", result.PackageCode)
	fmt.Printf("usage_metric_count\t%d\n", result.UsageMetricCount)
	fmt.Printf("seed_version_count\t%d\n", result.SeedVersionCount)
	fmt.Printf("config_copy_count\t%d\n", result.ConfigCopyCount)
}

func buildProvisionOptions(options bootstrapOptions) ([]bootstrapOptions, error) {
	if strings.TrimSpace(options.DSN) == "" {
		return nil, fmt.Errorf("MOCHAT_MYSQL_DSN or -dsn is required")
	}
	if strings.TrimSpace(options.Secret) == "" {
		return nil, fmt.Errorf("MOCHAT_SIMPLE_JWT_SECRET/SIMPLE_JWT_SECRET or -secret is required")
	}
	if strings.TrimSpace(options.BatchFile) != "" {
		return loadBatchOptions(options)
	}
	if err := validateProvisionOptions(options); err != nil {
		return nil, err
	}
	return []bootstrapOptions{options}, nil
}

func validateProvisionOptions(options bootstrapOptions) error {
	if strings.TrimSpace(options.DSN) == "" {
		return fmt.Errorf("MOCHAT_MYSQL_DSN or -dsn is required")
	}
	if strings.TrimSpace(options.Secret) == "" {
		return fmt.Errorf("MOCHAT_SIMPLE_JWT_SECRET/SIMPLE_JWT_SECRET or -secret is required")
	}
	if options.TenantID <= 0 {
		return fmt.Errorf("tenant id must be positive")
	}
	if strings.TrimSpace(options.TenantName) == "" {
		return fmt.Errorf("tenant name is required")
	}
	if strings.TrimSpace(options.Phone) == "" {
		return fmt.Errorf("MOCHAT_BOOTSTRAP_PHONE or -phone is required")
	}
	if strings.TrimSpace(options.Password) == "" {
		return fmt.Errorf("MOCHAT_BOOTSTRAP_PASSWORD or -password is required")
	}
	if strings.TrimSpace(options.UserName) == "" {
		return fmt.Errorf("user name is required")
	}
	if strings.TrimSpace(options.RoleName) == "" {
		return fmt.Errorf("role name is required")
	}
	if strings.TrimSpace(options.PackageCode) == "" {
		return fmt.Errorf("package code is required")
	}
	if strings.TrimSpace(options.PackageName) == "" {
		return fmt.Errorf("package name is required")
	}
	if options.MaxCorps < 0 || options.MaxUsers < 0 || options.MaxContacts < 0 || options.MaxRooms < 0 || options.MaxAgents < 0 || options.ChannelCodes < 0 || options.ShopCodes < 0 || options.Radars < 0 || options.Lotteries < 0 || options.RoomInfinitePulls < 0 || options.RoomFissions < 0 || options.RoomClockIns < 0 || options.RoomQualities < 0 || options.RoomCalendars < 0 || options.RoomReminds < 0 || options.ContactSOPs < 0 || options.RoomSOPs < 0 || options.SensitiveWords < 0 || options.StorageMB < 0 || options.ContactBatches < 0 || options.RoomBatches < 0 || options.RoomTagPulls < 0 || options.WorkRoomAutoPulls < 0 || options.WorkFissions < 0 || options.OfficialAccounts < 0 || options.AsyncExecutions < 0 {
		return fmt.Errorf("package limits must not be negative")
	}
	if _, err := parseOptionalTime(options.PackageExpiresAt); err != nil {
		return err
	}
	switch normalizeConfigCopyMode(options.ConfigCopyMode) {
	case "missing", "overwrite", "skip":
	default:
		return fmt.Errorf("config copy mode must be missing, overwrite, or skip")
	}
	return nil
}

func loadBatchOptions(defaults bootstrapOptions) ([]bootstrapOptions, error) {
	file, err := os.Open(defaults.BatchFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read batch header: %w", err)
	}
	columns := map[string]int{}
	for index, name := range header {
		columns[strings.TrimSpace(strings.ToLower(name))] = index
	}
	for _, required := range []string{"tenant_id", "tenant_name", "phone", "password"} {
		if _, ok := columns[required]; !ok {
			return nil, fmt.Errorf("batch file is missing required column %s", required)
		}
	}
	result := []bootstrapOptions{}
	for line := 2; ; line++ {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read batch line %d: %w", line, err)
		}
		if csvRecordBlank(record) {
			continue
		}
		options := defaults
		options.BatchFile = ""
		tenantIDValue, ok := csvRequiredString(record, columns, "tenant_id")
		if !ok {
			return nil, fmt.Errorf("line %d tenant_id is required", line)
		}
		tenantID, parseErr := strconv.Atoi(tenantIDValue)
		if parseErr != nil {
			return nil, fmt.Errorf("line %d tenant_id: %w", line, parseErr)
		}
		options.TenantID = tenantID
		if options.TenantName, ok = csvRequiredString(record, columns, "tenant_name"); !ok {
			return nil, fmt.Errorf("line %d tenant_name is required", line)
		}
		if options.Phone, ok = csvRequiredString(record, columns, "phone"); !ok {
			return nil, fmt.Errorf("line %d phone is required", line)
		}
		if options.Password, ok = csvRequiredString(record, columns, "password"); !ok {
			return nil, fmt.Errorf("line %d password is required", line)
		}
		options.UserName = csvString(record, columns, "user_name", options.UserName)
		options.RoleName = csvString(record, columns, "role_name", options.RoleName)
		options.PackageCode = csvString(record, columns, "package_code", options.PackageCode)
		options.PackageName = csvString(record, columns, "package_name", options.PackageName)
		options.PackageDescription = csvString(record, columns, "package_description", options.PackageDescription)
		options.PackageExpiresAt = csvString(record, columns, "package_expires_at", options.PackageExpiresAt)
		options.ConfigCopyMode = csvString(record, columns, "config_copy_mode", options.ConfigCopyMode)
		for _, field := range []struct {
			name  string
			value *int
		}{
			{name: "max_corps", value: &options.MaxCorps},
			{name: "max_users", value: &options.MaxUsers},
			{name: "max_contacts", value: &options.MaxContacts},
			{name: "max_rooms", value: &options.MaxRooms},
			{name: "max_agents", value: &options.MaxAgents},
			{name: "channel_codes", value: &options.ChannelCodes},
			{name: "shop_codes", value: &options.ShopCodes},
			{name: "radars", value: &options.Radars},
			{name: "lotteries", value: &options.Lotteries},
			{name: "room_infinite_pulls", value: &options.RoomInfinitePulls},
			{name: "room_fissions", value: &options.RoomFissions},
			{name: "room_clock_ins", value: &options.RoomClockIns},
			{name: "room_qualities", value: &options.RoomQualities},
			{name: "room_calendars", value: &options.RoomCalendars},
			{name: "room_reminds", value: &options.RoomReminds},
			{name: "contact_sops", value: &options.ContactSOPs},
			{name: "room_sops", value: &options.RoomSOPs},
			{name: "sensitive_words", value: &options.SensitiveWords},
			{name: "storage_mb", value: &options.StorageMB},
			{name: "contact_message_batches", value: &options.ContactBatches},
			{name: "room_message_batches", value: &options.RoomBatches},
			{name: "room_tag_pulls", value: &options.RoomTagPulls},
			{name: "work_room_auto_pulls", value: &options.WorkRoomAutoPulls},
			{name: "work_fissions", value: &options.WorkFissions},
			{name: "official_accounts", value: &options.OfficialAccounts},
			{name: "async_executions", value: &options.AsyncExecutions},
		} {
			*field.value, parseErr = csvInt(record, columns, field.name, *field.value)
			if parseErr != nil {
				return nil, fmt.Errorf("line %d %s: %w", line, field.name, parseErr)
			}
		}
		if err := validateProvisionOptions(options); err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		result = append(result, options)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("batch file %s does not contain any tenant rows", defaults.BatchFile)
	}
	return result, nil
}

func csvRecordBlank(record []string) bool {
	for _, value := range record {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

func csvString(record []string, columns map[string]int, name string, fallback string) string {
	index, ok := columns[name]
	if !ok || index >= len(record) {
		return fallback
	}
	value := strings.TrimSpace(record[index])
	if value == "" {
		return fallback
	}
	return value
}

func csvRequiredString(record []string, columns map[string]int, name string) (string, bool) {
	index, ok := columns[name]
	if !ok || index >= len(record) {
		return "", false
	}
	value := strings.TrimSpace(record[index])
	return value, value != ""
}

func csvInt(record []string, columns map[string]int, name string, fallback int) (int, error) {
	value := csvString(record, columns, name, "")
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}

func bootstrap(ctx context.Context, db *sql.DB, options bootstrapOptions) (bootstrapResult, error) {
	passwordHash, err := authjwt.GeneratePasswordHash(options.Secret, options.Password)
	if err != nil {
		return bootstrapResult{}, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return bootstrapResult{}, err
	}
	defer rollbackQuietly(tx)

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO mc_tenant (id, name, status, logo, login_background, url, copyright, created_at, updated_at, deleted_at)
		VALUES (?, ?, 1, '', '', '', '', NOW(), NOW(), NULL)
		ON DUPLICATE KEY UPDATE
			name = VALUES(name),
			status = 1,
			updated_at = NOW(),
			deleted_at = NULL
	`, options.TenantID, options.TenantName); err != nil {
		return bootstrapResult{}, err
	}

	userID, err := upsertAdminUser(ctx, tx, options, passwordHash)
	if err != nil {
		return bootstrapResult{}, err
	}
	if err := ensureBootstrapCorpEmployee(ctx, tx, options, userID); err != nil {
		return bootstrapResult{}, err
	}
	roleID, err := upsertAdminRole(ctx, tx, options, userID)
	if err != nil {
		return bootstrapResult{}, err
	}
	menuCount, err := replaceRoleMenusWithActiveMenus(ctx, tx, roleID)
	if err != nil {
		return bootstrapResult{}, err
	}
	if err := upsertUserRole(ctx, tx, userID, roleID); err != nil {
		return bootstrapResult{}, err
	}
	configCopyCount, err := copyDefaultTenantConfigs(ctx, tx, options)
	if err != nil {
		return bootstrapResult{}, err
	}
	usageMetricCount, seedVersionCount, err := syncSaaSProvisioning(ctx, tx, options, roleID, menuCount)
	if err != nil {
		return bootstrapResult{}, err
	}
	if err := recordProvisionRun(ctx, tx, options); err != nil {
		return bootstrapResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return bootstrapResult{}, err
	}
	return bootstrapResult{
		TenantID:         options.TenantID,
		UserID:           userID,
		RoleID:           roleID,
		MenuCount:        menuCount,
		PackageCode:      options.PackageCode,
		UsageMetricCount: usageMetricCount,
		SeedVersionCount: seedVersionCount,
		ConfigCopyCount:  configCopyCount,
	}, nil
}

func ensureBootstrapCorpEmployee(ctx context.Context, tx *sql.Tx, options bootstrapOptions, userID int) error {
	contract := defaultCorpEmployeeContract(options, userID)
	var corpID int
	err := tx.QueryRowContext(ctx, contract.CorpLookupSQL, contract.TenantID).Scan(&corpID)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if err == sql.ErrNoRows {
		result, insertErr := tx.ExecContext(ctx, `
			INSERT INTO mc_corp
				(name, wx_corpid, social_code, employee_secret, event_callback, contact_secret, token, encoding_aes_key, tenant_id, created_at, updated_at, deleted_at)
			VALUES (?, ?, '', '', '', '', '', '', ?, NOW(), NOW(), NULL)
		`, contract.CorpName, contract.WXCorpID, contract.TenantID)
		if insertErr != nil {
			return insertErr
		}
		id, idErr := result.LastInsertId()
		if idErr != nil {
			return idErr
		}
		corpID = int(id)
	}

	var employeeID int
	err = tx.QueryRowContext(ctx, contract.EmployeeLookupSQL, corpID, contract.UserID).Scan(&employeeID)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if err == sql.ErrNoRows {
		_, err = tx.ExecContext(ctx, contract.EmployeeInsertSQL,
			fmt.Sprintf("bootstrap_%d", contract.UserID), corpID, options.UserName, contract.EmployeeMobile, contract.UserID)
	}
	return err
}

func upsertAdminUser(ctx context.Context, tx *sql.Tx, options bootstrapOptions, passwordHash string) (int, error) {
	var existingID int
	err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM mc_user
		WHERE phone = ? AND tenant_id = ?
		ORDER BY deleted_at IS NULL DESC, id ASC
		LIMIT 1
	`, options.Phone, options.TenantID).Scan(&existingID)
	if err != nil && err != sql.ErrNoRows {
		return 0, err
	}
	if existingID > 0 {
		if _, err := tx.ExecContext(ctx, `
			UPDATE mc_user
			SET password = ?, name = ?, status = 1, tenant_id = ?, isSuperAdmin = 1, updated_at = NOW(), deleted_at = NULL
			WHERE id = ?
		`, passwordHash, options.UserName, options.TenantID, existingID); err != nil {
			return 0, err
		}
		return existingID, nil
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mc_user
			(phone, password, name, gender, department, position, login_time, status, tenant_id, created_at, updated_at, deleted_at, isSuperAdmin)
		VALUES (?, ?, ?, 0, '', '', NULL, 1, ?, NOW(), NOW(), NULL, 1)
	`, options.Phone, passwordHash, options.UserName, options.TenantID)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

func upsertAdminRole(ctx context.Context, tx *sql.Tx, options bootstrapOptions, userID int) (int, error) {
	var roleID int
	err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM mc_rbac_role
		WHERE tenant_id = ? AND name = ?
		ORDER BY deleted_at IS NULL DESC, id ASC
		LIMIT 1
	`, options.TenantID, options.RoleName).Scan(&roleID)
	if err != nil && err != sql.ErrNoRows {
		return 0, err
	}
	if roleID > 0 {
		if _, err := tx.ExecContext(ctx, `
			UPDATE mc_rbac_role
			SET status = 1, operate_id = ?, operate_name = ?, data_permission = '[]', updated_at = NOW(), deleted_at = NULL
			WHERE id = ?
		`, userID, options.UserName, roleID); err != nil {
			return 0, err
		}
		return roleID, nil
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mc_rbac_role
			(tenant_id, name, remarks, status, operate_id, operate_name, data_permission, created_at, updated_at, deleted_at)
		VALUES (?, ?, '系统预置全权限角色', 1, ?, ?, '[]', NOW(), NOW(), NULL)
	`, options.TenantID, options.RoleName, userID, options.UserName)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

func replaceRoleMenusWithActiveMenus(ctx context.Context, tx *sql.Tx, roleID int) (int, error) {
	if _, err := tx.ExecContext(ctx, `DELETE FROM mc_rbac_role_menu WHERE role_id = ?`, roleID); err != nil {
		return 0, err
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO mc_rbac_role_menu (role_id, menu_id, created_at, updated_at)
		SELECT ?, id, NOW(), NOW()
		FROM mc_rbac_menu
		WHERE deleted_at IS NULL
		ORDER BY id ASC
	`, roleID)
	if err != nil {
		return 0, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if count == 0 {
		return 0, fmt.Errorf("no active RBAC menus found; run mochat-migrate apply before bootstrap")
	}
	return int(count), nil
}

func upsertUserRole(ctx context.Context, tx *sql.Tx, userID int, roleID int) error {
	if _, err := tx.ExecContext(ctx, `
		UPDATE mc_rbac_user_role
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE user_id = ? AND role_id <> ? AND deleted_at IS NULL
	`, userID, roleID); err != nil {
		return err
	}
	var userRoleID int
	err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM mc_rbac_user_role
		WHERE user_id = ? AND role_id = ?
		ORDER BY id ASC
		LIMIT 1
	`, userID, roleID).Scan(&userRoleID)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if userRoleID > 0 {
		_, err = tx.ExecContext(ctx, `
			UPDATE mc_rbac_user_role
			SET updated_at = NOW(), deleted_at = NULL
			WHERE id = ?
		`, userRoleID)
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO mc_rbac_user_role (user_id, role_id, created_at, updated_at, deleted_at)
		VALUES (?, ?, NOW(), NOW(), NULL)
	`, userID, roleID)
	return err
}

func copyDefaultTenantConfigs(ctx context.Context, tx *sql.Tx, options bootstrapOptions) (int, error) {
	mode := normalizeConfigCopyMode(options.ConfigCopyMode)
	if mode == "skip" {
		return 0, nil
	}
	if mode == "overwrite" {
		if _, err := tx.ExecContext(ctx, `
			UPDATE mc_system_config_value tenant_value
			JOIN mc_system_config_value src
				ON src.config_id = tenant_value.config_id
				AND src.type = 1
				AND src.target_id = 0
				AND src.deleted_at IS NULL
			SET tenant_value.deleted_at = NOW(), tenant_value.updated_at = NOW()
			WHERE tenant_value.type = 3
				AND tenant_value.target_id = ?
				AND tenant_value.deleted_at IS NULL
		`, options.TenantID); err != nil {
			return 0, err
		}
	}
	query := `
		INSERT INTO mc_system_config_value
			(type, target_id, config_id, value, description, created_at, updated_at, deleted_at)
		SELECT 3, ?, src.config_id, src.value, src.description, NOW(), NOW(), NULL
		FROM mc_system_config_value src
		WHERE src.type = 1
			AND src.target_id = 0
			AND src.deleted_at IS NULL
	`
	if mode == "missing" {
		query += `
			AND NOT EXISTS (
				SELECT 1
				FROM mc_system_config_value tenant_value
				WHERE tenant_value.type = 3
					AND tenant_value.target_id = ?
					AND tenant_value.config_id = src.config_id
					AND tenant_value.deleted_at IS NULL
			)
		`
	}
	var result sql.Result
	var err error
	if mode == "missing" {
		result, err = tx.ExecContext(ctx, query, options.TenantID, options.TenantID)
	} else {
		result, err = tx.ExecContext(ctx, query, options.TenantID)
	}
	if err != nil {
		return 0, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func syncSaaSProvisioning(ctx context.Context, tx *sql.Tx, options bootstrapOptions, roleID int, menuCount int) (int, int, error) {
	limits := saasLimits{
		MaxCorps:          options.MaxCorps,
		MaxUsers:          options.MaxUsers,
		MaxContacts:       options.MaxContacts,
		MaxRooms:          options.MaxRooms,
		MaxAgents:         options.MaxAgents,
		ChannelCodes:      options.ChannelCodes,
		ShopCodes:         options.ShopCodes,
		Radars:            options.Radars,
		Lotteries:         options.Lotteries,
		RoomInfinitePulls: options.RoomInfinitePulls,
		RoomFissions:      options.RoomFissions,
		RoomClockIns:      options.RoomClockIns,
		RoomQualities:     options.RoomQualities,
		RoomCalendars:     options.RoomCalendars,
		RoomReminds:       options.RoomReminds,
		ContactSOPs:       options.ContactSOPs,
		RoomSOPs:          options.RoomSOPs,
		SensitiveWords:    options.SensitiveWords,
		StorageMB:         options.StorageMB,
		ContactBatches:    options.ContactBatches,
		RoomBatches:       options.RoomBatches,
		RoomTagPulls:      options.RoomTagPulls,
		AutoPulls:         options.WorkRoomAutoPulls,
		WorkFissions:      options.WorkFissions,
		OfficialAccounts:  options.OfficialAccounts,
		AsyncExecutions:   options.AsyncExecutions,
	}
	limitsJSON, err := json.Marshal(limits)
	if err != nil {
		return 0, 0, err
	}
	expiresAt, err := parseOptionalTime(options.PackageExpiresAt)
	if err != nil {
		return 0, 0, err
	}
	var expiresValue any
	if expiresAt != nil {
		expiresValue = *expiresAt
	}
	if _, err := tx.ExecContext(ctx, `
			INSERT INTO mochat_go_saas_packages
				(code, name, description, max_corps, max_users, max_contacts, max_rooms, max_agents, channel_codes, shop_codes, radars, lotteries, room_infinite_pulls, room_fissions, room_clock_ins, room_qualities, room_calendars, room_reminds, contact_sops, room_sops, sensitive_words, storage_mb, contact_message_batches, room_message_batches, room_tag_pulls, work_room_auto_pulls, work_fissions, official_accounts, async_executions, status, created_at, updated_at, deleted_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, NOW(), NOW(), NULL)
		ON DUPLICATE KEY UPDATE
			name = VALUES(name),
			description = VALUES(description),
			max_corps = VALUES(max_corps),
			max_users = VALUES(max_users),
			max_contacts = VALUES(max_contacts),
			max_rooms = VALUES(max_rooms),
			max_agents = VALUES(max_agents),
				channel_codes = VALUES(channel_codes),
				shop_codes = VALUES(shop_codes),
				radars = VALUES(radars),
				lotteries = VALUES(lotteries),
					room_infinite_pulls = VALUES(room_infinite_pulls),
					room_fissions = VALUES(room_fissions),
					room_clock_ins = VALUES(room_clock_ins),
					room_qualities = VALUES(room_qualities),
					room_calendars = VALUES(room_calendars),
					room_reminds = VALUES(room_reminds),
					contact_sops = VALUES(contact_sops),
					room_sops = VALUES(room_sops),
					sensitive_words = VALUES(sensitive_words),
					storage_mb = VALUES(storage_mb),
			contact_message_batches = VALUES(contact_message_batches),
			room_message_batches = VALUES(room_message_batches),
			room_tag_pulls = VALUES(room_tag_pulls),
			work_room_auto_pulls = VALUES(work_room_auto_pulls),
			work_fissions = VALUES(work_fissions),
			official_accounts = VALUES(official_accounts),
			async_executions = VALUES(async_executions),
			status = 1,
			updated_at = NOW(),
			deleted_at = NULL
			`, options.PackageCode, options.PackageName, options.PackageDescription, options.MaxCorps, options.MaxUsers, options.MaxContacts, options.MaxRooms, options.MaxAgents, options.ChannelCodes, options.ShopCodes, options.Radars, options.Lotteries, options.RoomInfinitePulls, options.RoomFissions, options.RoomClockIns, options.RoomQualities, options.RoomCalendars, options.RoomReminds, options.ContactSOPs, options.RoomSOPs, options.SensitiveWords, options.StorageMB, options.ContactBatches, options.RoomBatches, options.RoomTagPulls, options.WorkRoomAutoPulls, options.WorkFissions, options.OfficialAccounts, options.AsyncExecutions); err != nil {
		return 0, 0, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_saas_tenant_packages
			(tenant_id, package_code, package_name, starts_at, expires_at, status, limits_json, created_at, updated_at, deleted_at)
		VALUES (?, ?, ?, NOW(), ?, 1, ?, NOW(), NOW(), NULL)
		ON DUPLICATE KEY UPDATE
			package_code = VALUES(package_code),
			package_name = VALUES(package_name),
			expires_at = VALUES(expires_at),
			status = 1,
			limits_json = VALUES(limits_json),
			updated_at = NOW(),
			deleted_at = NULL
	`, options.TenantID, options.PackageCode, options.PackageName, expiresValue, string(limitsJSON)); err != nil {
		return 0, 0, err
	}
	usageMetricCount, err := upsertUsageCounters(ctx, tx, options.TenantID, limits)
	if err != nil {
		return 0, 0, err
	}
	seedVersionCount, err := upsertSeedVersions(ctx, tx, options.TenantID, roleID, menuCount)
	if err != nil {
		return 0, 0, err
	}
	return usageMetricCount, seedVersionCount, nil
}

func upsertUsageCounters(ctx context.Context, tx *sql.Tx, tenantID int, limits saasLimits) (int, error) {
	current, err := currentUsage(ctx, tx, tenantID)
	if err != nil {
		return 0, err
	}
	metrics := []struct {
		name  string
		limit int
	}{
		{name: "corps", limit: limits.MaxCorps},
		{name: "users", limit: limits.MaxUsers},
		{name: "contacts", limit: limits.MaxContacts},
		{name: "rooms", limit: limits.MaxRooms},
		{name: "agents", limit: limits.MaxAgents},
		{name: "channel_codes", limit: limits.ChannelCodes},
		{name: "shop_codes", limit: limits.ShopCodes},
		{name: "radars", limit: limits.Radars},
		{name: "lotteries", limit: limits.Lotteries},
		{name: "room_infinite_pulls", limit: limits.RoomInfinitePulls},
		{name: "room_fissions", limit: limits.RoomFissions},
		{name: "room_clock_ins", limit: limits.RoomClockIns},
		{name: "room_qualities", limit: limits.RoomQualities},
		{name: "room_calendars", limit: limits.RoomCalendars},
		{name: "room_reminds", limit: limits.RoomReminds},
		{name: "contact_sops", limit: limits.ContactSOPs},
		{name: "room_sops", limit: limits.RoomSOPs},
		{name: "sensitive_words", limit: limits.SensitiveWords},
		{name: "storage_mb", limit: limits.StorageMB},
		{name: "contact_message_batches", limit: limits.ContactBatches},
		{name: "room_message_batches", limit: limits.RoomBatches},
		{name: "room_tag_pulls", limit: limits.RoomTagPulls},
		{name: "work_room_auto_pulls", limit: limits.AutoPulls},
		{name: "work_fissions", limit: limits.WorkFissions},
		{name: "official_accounts", limit: limits.OfficialAccounts},
		{name: "async_executions", limit: limits.AsyncExecutions},
	}
	for _, metric := range metrics {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO mochat_go_saas_usage_counters
				(tenant_id, metric, period_key, used_value, limit_value, updated_by, created_at, updated_at, deleted_at)
			VALUES (?, ?, 'lifetime', ?, ?, 'bootstrap', NOW(), NOW(), NULL)
			ON DUPLICATE KEY UPDATE
				used_value = VALUES(used_value),
				limit_value = VALUES(limit_value),
				updated_by = 'bootstrap',
				updated_at = NOW(),
				deleted_at = NULL
		`, tenantID, metric.name, current[metric.name], metric.limit); err != nil {
			return 0, err
		}
	}
	return len(metrics), nil
}

func currentUsage(ctx context.Context, tx *sql.Tx, tenantID int) (map[string]int, error) {
	result := map[string]int{"storage_mb": 0, "async_executions": 0}
	queries := map[string]string{
		"corps": `
			SELECT COUNT(*)
			FROM mc_corp
			WHERE tenant_id = ? AND deleted_at IS NULL
		`,
		"users": `
			SELECT COUNT(*)
			FROM mc_user
			WHERE tenant_id = ? AND deleted_at IS NULL
		`,
		"contacts": `
			SELECT COUNT(*)
			FROM mc_work_contact contact
			JOIN mc_corp corp ON corp.id = contact.corp_id
			WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND contact.deleted_at IS NULL
		`,
		"rooms": `
			SELECT COUNT(*)
			FROM mc_work_room room
			JOIN mc_corp corp ON corp.id = room.corp_id
			WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND room.deleted_at IS NULL
		`,
		"agents": `
				SELECT COUNT(*)
				FROM mc_work_agent agent
				JOIN mc_corp corp ON corp.id = agent.corp_id
				WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND agent.deleted_at IS NULL
			`,
		"channel_codes": `
				SELECT COUNT(*)
				FROM mc_channel_code code
				JOIN mc_corp corp ON corp.id = code.corp_id
				WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND code.deleted_at IS NULL
			`,
		"shop_codes": `
			SELECT COUNT(*)
			FROM mc_shop_code code
			JOIN mc_corp corp ON corp.id = code.corp_id
			WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND code.deleted_at IS NULL
		`,
		"radars": `
				SELECT COUNT(*)
				FROM mc_radar radar
				JOIN mc_corp corp ON corp.id = radar.corp_id
				WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND radar.deleted_at IS NULL
			`,
		"lotteries": `
				SELECT COUNT(*)
				FROM mc_lottery lottery
				JOIN mc_corp corp ON corp.id = lottery.corp_id
				WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND lottery.deleted_at IS NULL
			`,
		"room_infinite_pulls": `
				SELECT COUNT(*)
				FROM mc_room_infinite activity
				JOIN mc_corp corp ON corp.id = activity.corp_id
				WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND activity.deleted_at IS NULL
			`,
		"room_fissions": `
				SELECT COUNT(*)
				FROM mc_room_fission activity
				JOIN mc_corp corp ON corp.id = activity.corp_id
				WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND activity.deleted_at IS NULL
			`,
		"room_clock_ins": `
					SELECT COUNT(*)
					FROM mc_room_clock_in activity
					JOIN mc_corp corp ON corp.id = activity.corp_id
					WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND activity.deleted_at IS NULL
				`,
		"room_qualities": `
					SELECT COUNT(*)
					FROM mc_room_quality activity
					JOIN mc_corp corp ON corp.id = activity.corp_id
					WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND activity.deleted_at IS NULL
				`,
		"room_calendars": `
					SELECT COUNT(*)
					FROM mc_room_calendar activity
					JOIN mc_corp corp ON corp.id = activity.corp_id
					WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND activity.deleted_at IS NULL
				`,
		"room_reminds": `
					SELECT COUNT(*)
					FROM mc_room_remind activity
					JOIN mc_corp corp ON corp.id = activity.corp_id
					WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND activity.deleted_at IS NULL
				`,
		"contact_sops": `
			SELECT COUNT(*)
			FROM mc_contact_sop sop
			JOIN mc_corp corp ON corp.id = sop.corp_id
			WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL
		`,
		"room_sops": `
			SELECT COUNT(*)
			FROM mc_room_sop sop
			JOIN mc_corp corp ON corp.id = sop.corp_id
			WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL
		`,
		"sensitive_words": `
			SELECT COUNT(*)
			FROM mc_sensitive_word word
			JOIN mc_corp corp ON corp.id = word.corp_id
			WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND word.deleted_at IS NULL
		`,
		"contact_message_batches": `
			SELECT COUNT(*)
			FROM mc_contact_message_batch_send batch
			JOIN mc_corp corp ON corp.id = batch.corp_id
			WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND batch.deleted_at IS NULL
		`,
		"room_message_batches": `
			SELECT COUNT(*)
			FROM mc_room_message_batch_send batch
			JOIN mc_corp corp ON corp.id = batch.corp_id
			WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND batch.deleted_at IS NULL
		`,
		"room_tag_pulls": `
			SELECT COUNT(*)
			FROM mc_room_tag_pull activity
			JOIN mc_corp corp ON corp.id = activity.corp_id
			WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND activity.deleted_at IS NULL
		`,
		"work_room_auto_pulls": `
			SELECT COUNT(*)
			FROM mc_work_room_auto_pull activity
			JOIN mc_corp corp ON corp.id = activity.corp_id
			WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND activity.deleted_at IS NULL
		`,
		"work_fissions": `
			SELECT COUNT(*)
			FROM mc_work_fission activity
			JOIN mc_corp corp ON corp.id = activity.corp_id
			WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND activity.deleted_at IS NULL
		`,
		"official_accounts": `
			SELECT COUNT(*)
			FROM mc_official_account account
			JOIN mc_corp corp ON corp.id = account.corp_id
			WHERE corp.tenant_id = ? AND corp.deleted_at IS NULL AND account.deleted_at IS NULL
		`,
	}
	for metric, query := range queries {
		value, err := queryInt(ctx, tx, query, tenantID)
		if err != nil {
			return nil, err
		}
		result[metric] = value
	}
	asyncTableReady, err := txTableColumnExists(ctx, tx, "mochat_go_background_task_executions", "tenant_id")
	if err != nil {
		return nil, err
	}
	if asyncTableReady {
		value, err := queryInt(ctx, tx, `
			SELECT COUNT(*)
			FROM mochat_go_background_task_executions
			WHERE tenant_id = ?
				AND kind = 'queue_item'
				AND status IN ('succeeded', 'failed', 'stopped')
		`, tenantID)
		if err != nil {
			return nil, err
		}
		result["async_executions"] = value
	}
	return result, nil
}

func upsertSeedVersions(ctx context.Context, tx *sql.Tx, tenantID int, roleID int, menuCount int) (int, error) {
	checksum, err := migrationChecksum(ctx, tx, "0002_seed_core_data")
	if err != nil {
		return 0, err
	}
	contactFieldCount, err := queryInt(ctx, tx, `SELECT COUNT(*) FROM mc_contact_field WHERE deleted_at IS NULL`)
	if err != nil {
		return 0, err
	}
	chatToolCount, err := queryInt(ctx, tx, `SELECT COUNT(*) FROM mc_chat_tool WHERE deleted_at IS NULL`)
	if err != nil {
		return 0, err
	}
	seeds := []struct {
		name     string
		metadata map[string]any
	}{
		{name: "rbac_menu", metadata: map[string]any{"activeMenuCount": menuCount, "roleId": roleID}},
		{name: "contact_field", metadata: map[string]any{"activeFieldCount": contactFieldCount}},
		{name: "chat_tool", metadata: map[string]any{"activeToolCount": chatToolCount}},
	}
	for _, seed := range seeds {
		metadata, err := json.Marshal(seed.metadata)
		if err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO mochat_go_seed_versions
				(scope, target_id, seed_name, seed_version, checksum, metadata, applied_at, created_at, updated_at)
			VALUES ('tenant', ?, ?, '0002_seed_core_data', ?, ?, NOW(), NOW(), NOW())
			ON DUPLICATE KEY UPDATE
				seed_version = VALUES(seed_version),
				checksum = VALUES(checksum),
				metadata = VALUES(metadata),
				applied_at = NOW(),
				updated_at = NOW()
		`, tenantID, seed.name, checksum, string(metadata)); err != nil {
			return 0, err
		}
	}
	return len(seeds), nil
}

func migrationChecksum(ctx context.Context, tx *sql.Tx, version string) (string, error) {
	var checksum string
	err := tx.QueryRowContext(ctx, `
		SELECT checksum
		FROM mochat_go_schema_migrations
		WHERE version = ?
		LIMIT 1
	`, version).Scan(&checksum)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return checksum, err
}

func recordProvisionRun(ctx context.Context, tx *sql.Tx, options bootstrapOptions) error {
	runKey := fmt.Sprintf("tenant:%d:admin:%s", options.TenantID, options.Phone)
	_, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_tenant_provision_runs
			(run_key, tenant_id, package_code, admin_phone, status, message, started_at, finished_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, 1, 'provisioned', NOW(), NOW(), NOW(), NOW())
		ON DUPLICATE KEY UPDATE
			tenant_id = VALUES(tenant_id),
			package_code = VALUES(package_code),
			admin_phone = VALUES(admin_phone),
			status = 1,
			message = 'provisioned',
			started_at = NOW(),
			finished_at = NOW(),
			updated_at = NOW()
	`, runKey, options.TenantID, options.PackageCode, options.Phone)
	return err
}

func queryInt(ctx context.Context, tx *sql.Tx, query string, args ...any) (int, error) {
	var value int
	if err := tx.QueryRowContext(ctx, query, args...).Scan(&value); err != nil {
		return 0, err
	}
	return value, nil
}

func txTableColumnExists(ctx context.Context, tx *sql.Tx, table string, column string) (bool, error) {
	var count int
	err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?
	`, table, column).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func rollbackQuietly(tx *sql.Tx) {
	if tx != nil {
		_ = tx.Rollback()
	}
}

func firstEnv(names ...string) string {
	for _, name := range names {
		if value := os.Getenv(name); value != "" {
			return value
		}
	}
	return ""
}

func envDefault(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func intEnv(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	var parsed int
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func intEnvAllowZero(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}

func normalizeConfigCopyMode(mode string) string {
	mode = strings.TrimSpace(strings.ToLower(mode))
	if mode == "" {
		return "missing"
	}
	return mode
}

func parseOptionalTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	var lastErr error
	for _, layout := range layouts {
		parsed, err := time.ParseInLocation(layout, value, time.Local)
		if err == nil {
			return &parsed, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("package expires at must be RFC3339, YYYY-MM-DD, or YYYY-MM-DD HH:MM:SS: %w", lastErr)
}
