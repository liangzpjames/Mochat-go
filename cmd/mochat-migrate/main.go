package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"jiyi/mochat-go/internal/migration"
	"jiyi/mochat-go/internal/mysqlconn"
	"jiyi/mochat-go/internal/observability"
)

type migrationRunner interface {
	Apply(context.Context) ([]migration.StatusItem, error)
	BaselineComposeInit(context.Context) ([]migration.StatusItem, error)
	Status(context.Context) ([]migration.StatusItem, error)
	Baseline(context.Context) ([]migration.StatusItem, error)
	RollbackLast(context.Context) (string, error)
}

type migrationCommandOptions struct {
	Action string
}

func main() {
	logger, err := observability.ConfigureFromEnv("mochat-migrate")
	if err != nil {
		fmt.Fprintln(os.Stderr, "日志配置无效；请检查 MOCHAT_LOG_LEVEL 和 MOCHAT_LOG_FORMAT")
		os.Exit(1)
	}
	action := flag.String("action", "apply", "migration action: apply, status, baseline, baseline-compose-init, rollback, inventory")
	dsn := flag.String("dsn", os.Getenv("MOCHAT_MYSQL_DSN"), "MySQL DSN; defaults to MOCHAT_MYSQL_DSN")
	projectRoot := flag.String("project-root", ".", "project root used to resolve default migrations")
	timeout := flag.Duration("timeout", 5*time.Minute, "migration timeout")
	flag.Parse()

	root, err := filepath.Abs(*projectRoot)
	if err != nil {
		logMigrationSetupFailure(logger, "MIGRATION_PROJECT_ROOT_INVALID")
		os.Exit(1)
	}
	if normalized := strings.ToLower(strings.TrimSpace(*action)); normalized == "inventory" || normalized == "manifest" {
		items, inventoryErr := migration.DefaultInventory(root)
		if inventoryErr != nil {
			logMigrationSetupFailure(logger, "MIGRATION_INVENTORY_INVALID")
			os.Exit(1)
		}
		writeMigrationInventory(os.Stdout, items)
		return
	}
	if *dsn == "" {
		logMigrationSetupFailure(logger, "MIGRATION_DSN_MISSING")
		os.Exit(1)
	}
	db, err := mysqlconn.Open(*dsn)
	if err != nil {
		logMigrationSetupFailure(logger, "MIGRATION_DATABASE_OPEN_FAILED")
		os.Exit(1)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		logMigrationSetupFailure(logger, "MIGRATION_DATABASE_UNAVAILABLE")
		os.Exit(1)
	}

	runner, err := migration.NewRunner(db, migration.DefaultMigrations(root))
	if err != nil {
		logMigrationSetupFailure(logger, "MIGRATION_RUNNER_INVALID")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	if err := runMigrationCommand(ctx, migrationCommandOptions{Action: *action}, runner, logger, os.Stdout); err != nil {
		os.Exit(1)
	}
}

func writeMigrationInventory(output io.Writer, items []migration.InventoryItem) {
	for _, item := range items {
		fmt.Fprintf(output, "%s\t%s\t%s\t%s\n", item.Version, item.Checksum, item.Kind, item.Description)
	}
}

func runMigrationCommand(ctx context.Context, options migrationCommandOptions, runner migrationRunner, logger *slog.Logger, output io.Writer) error {
	if logger == nil {
		logger = slog.Default()
	}
	if output == nil {
		output = io.Discard
	}
	action := strings.ToLower(strings.TrimSpace(options.Action))
	startedAt := time.Now()
	logger.Info("数据库迁移开始；失败时请检查数据库连通性和迁移版本",
		"event", "migration_started",
		"component", "migration",
		"step", action,
		"result", "started",
	)
	var status []migration.StatusItem
	var rolledBack string
	var err error
	switch action {
	case "apply", "up":
		status, err = runner.Apply(ctx)
	case "baseline-compose-init":
		status, err = runner.BaselineComposeInit(ctx)
	case "status":
		status, err = runner.Status(ctx)
	case "baseline":
		status, err = runner.Baseline(ctx)
	case "rollback", "down":
		rolledBack, err = runner.RollbackLast(ctx)
	default:
		err = fmt.Errorf("unknown migration action")
	}
	if err != nil {
		var pending *migration.ControlledMigrationPendingError
		if errors.As(err, &pending) {
			fmt.Fprintf(output, "MIGRATION_CONTROLLED_PENDING\t%s\n", pending.Version)
		}
		logger.Error("数据库迁移失败；请检查数据库状态、迁移顺序和冲突记录",
			"event", "migration_failed",
			"component", "migration",
			"step", action,
			"result", "failed",
			"error_code", migrationFailureCode(action, err),
			"duration_ms", time.Since(startedAt).Milliseconds(),
		)
		return err
	}
	if rolledBack != "" {
		fmt.Fprintf(output, "%s\trolled_back\n", rolledBack)
		logger.Info("数据库迁移已回滚；请核对目标版本和业务兼容性",
			"event", "migration_completed", "component", "migration", "step", action, "result", "success",
			"rolled_back", rolledBack, "applied", 0, "total", 0, "duration_ms", time.Since(startedAt).Milliseconds())
		return nil
	}
	applied := 0
	for _, item := range status {
		if item.State == "applied_now" {
			applied++
		}
		appliedAt := ""
		if item.Applied != nil && !item.Applied.AppliedAt.IsZero() {
			appliedAt = item.Applied.AppliedAt.Format(time.RFC3339)
		}
		fmt.Fprintf(output, "%s\t%s\t%s\t%s\t%s\n", item.Migration.Version, item.State, item.Checksum, appliedAt, item.Migration.Description)
	}
	if action == "status" {
		if integrityErr := migrationStatusIntegrityError(status); integrityErr != nil {
			logger.Error("数据库迁移状态校验失败；请检查校验和或未知版本",
				"event", "migration_failed", "component", "migration", "step", action, "result", "failed",
				"error_code", migrationFailureCode(action, integrityErr), "duration_ms", time.Since(startedAt).Milliseconds())
			return integrityErr
		}
	}
	logger.Info("数据库迁移完成；请核对本次应用数量和迁移状态",
		"event", "migration_completed", "component", "migration", "step", action, "result", "success",
		"applied", applied, "total", len(status), "duration_ms", time.Since(startedAt).Milliseconds())
	return nil
}

func migrationStatusIntegrityError(status []migration.StatusItem) error {
	for _, item := range status {
		switch item.State {
		case "checksum_mismatch", "database_ahead":
			return fmt.Errorf("migration %s has integrity state %s", item.Migration.Version, item.State)
		}
	}
	return nil
}

func logMigrationSetupFailure(logger *slog.Logger, errorCode string) {
	logger.Error("数据库迁移命令无法启动；请检查必需配置和数据库连通性",
		"event", "migration_setup_failed", "component", "migration", "step", "setup", "result", "failed", "error_code", errorCode)
}

func migrationErrorCode(action string) string {
	normalized := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(action), "-", "_"))
	if normalized == "" {
		normalized = "UNKNOWN"
	}
	return "MIGRATION_" + normalized + "_FAILED"
}

func migrationFailureCode(action string, err error) string {
	var pending *migration.ControlledMigrationPendingError
	if errors.As(err, &pending) {
		return "MIGRATION_CONTROLLED_PENDING"
	}
	return migrationErrorCode(action)
}
