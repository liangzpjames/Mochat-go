package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"jiyi/mochat-go/internal/archivesim"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "archive simulator:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mochat-archive-simulator apply|status|cleanup --corp-id N --batch NAME")
	}
	action := strings.ToLower(strings.TrimSpace(args[0]))
	if action != "apply" && action != "status" && action != "cleanup" {
		return fmt.Errorf("unsupported action %q", action)
	}
	flags := flag.NewFlagSet("mochat-archive-simulator "+action, flag.ContinueOnError)
	corpID := flags.Int("corp-id", 0, "target corp id")
	batch := flags.String("batch", "acceptance", "isolated simulation batch key")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	dsn := strings.TrimSpace(os.Getenv("MOCHAT_MYSQL_DSN"))
	if dsn == "" {
		return fmt.Errorf("MOCHAT_MYSQL_DSN is required")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	simulator := archivesim.New(db)
	var result archivesim.Result
	switch action {
	case "apply":
		result, err = simulator.Apply(ctx, *corpID, *batch)
	case "status":
		result, err = simulator.Status(ctx, *corpID, *batch)
	case "cleanup":
		result, err = simulator.Cleanup(ctx, *corpID, *batch)
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}
