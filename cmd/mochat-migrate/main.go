package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"jiyi/mochat-go/internal/migration"
	"jiyi/mochat-go/internal/mysqlconn"
)

func main() {
	action := flag.String("action", "apply", "migration action: apply, status, baseline, rollback")
	dsn := flag.String("dsn", os.Getenv("MOCHAT_MYSQL_DSN"), "MySQL DSN; defaults to MOCHAT_MYSQL_DSN")
	projectRoot := flag.String("project-root", ".", "project root used to resolve default migrations")
	timeout := flag.Duration("timeout", 5*time.Minute, "migration timeout")
	flag.Parse()

	if *dsn == "" {
		log.Fatal("MOCHAT_MYSQL_DSN or -dsn is required")
	}
	root, err := filepath.Abs(*projectRoot)
	if err != nil {
		log.Fatalf("resolve project root: %v", err)
	}
	db, err := mysqlconn.Open(*dsn)
	if err != nil {
		log.Fatalf("open mysql: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("ping mysql: %v", err)
	}

	runner, err := migration.NewRunner(db, migration.DefaultMigrations(root))
	if err != nil {
		log.Fatalf("build runner: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	var status []migration.StatusItem
	var rolledBack string
	switch *action {
	case "apply", "up":
		status, err = runner.Apply(ctx)
	case "status":
		status, err = runner.Status(ctx)
	case "baseline":
		status, err = runner.Baseline(ctx)
	case "rollback", "down":
		rolledBack, err = runner.RollbackLast(ctx)
	default:
		err = fmt.Errorf("unknown action %q", *action)
	}
	if err != nil {
		log.Fatalf("migration %s failed: %v", *action, err)
	}
	if rolledBack != "" {
		fmt.Printf("%s\trolled_back\n", rolledBack)
		return
	}
	for _, item := range status {
		appliedAt := ""
		if item.Applied != nil && !item.Applied.AppliedAt.IsZero() {
			appliedAt = item.Applied.AppliedAt.Format(time.RFC3339)
		}
		fmt.Printf("%s\t%s\t%s\t%s\t%s\n", item.Migration.Version, item.State, item.Checksum, appliedAt, item.Migration.Description)
	}
}
