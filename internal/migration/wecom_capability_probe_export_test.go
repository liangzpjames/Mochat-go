package migration

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ExecuteWeComCapabilityLedger0139UpProbe executes only the production 0139
// up script without touching the standard ledger. It exists solely so the
// external-package residual-schema integration test can re-run the 0139
// preflight against an already-applied, deliberately damaged schema.
func ExecuteWeComCapabilityLedger0139UpProbe(ctx context.Context, db *sql.DB, root string) error {
	var target *Migration
	for _, candidate := range DefaultMigrations(root) {
		if candidate.Version == "0139_wecom_capability_ledger" {
			candidate := candidate
			target = &candidate
			break
		}
	}
	if target == nil {
		return fmt.Errorf("production migration registry does not contain 0139_wecom_capability_ledger")
	}
	body, err := os.ReadFile(filepath.Clean(target.Path))
	if err != nil {
		return err
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if automaticMigrationNeedsServerDetection(string(body)) {
		var serverVersion string
		if err := conn.QueryRowContext(ctx, "SELECT VERSION()").Scan(&serverVersion); err != nil {
			return err
		}
		if strings.HasPrefix(strings.TrimSpace(serverVersion), "5.7.") {
			return execSQLScriptMySQL57(ctx, conn, string(body))
		}
	}
	return execSQLScriptWithExecutor(ctx, conn, string(body))
}
