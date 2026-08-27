package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"jiyi/mochat-go/internal/archivebridge"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "archive simulator:", err)
		os.Exit(1)
	}
}

func run(args []string) error { return runWith(args, os.Getenv, sql.Open) }

type environmentReader func(string) string
type mysqlOpener func(string, string) (*sql.DB, error)

func runWith(args []string, getenv environmentReader, open mysqlOpener) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mochat-archive-simulator seed|send|status|cleanup --enable-simulation")
	}
	action := strings.ToLower(strings.TrimSpace(args[0]))
	if action != "seed" && action != "send" && action != "status" && action != "cleanup" {
		return fmt.Errorf("unsupported action %q", action)
	}
	flags := flag.NewFlagSet("mochat-archive-simulator "+action, flag.ContinueOnError)
	tenantID := flags.Int64("tenant-id", 0, "target tenant id")
	_ = flags.Int64("corp-id", 0, "deprecated; tenant binding is resolved from the database")
	mode := flags.String("mode", "", "self_built or third_party_delegated")
	dataset := flags.String("dataset", "MOCHAT-LOCAL-SIM-default", "explicit local fixture dataset")
	messageType := flags.String("type", "text", "text, image, voice, video or file")
	textValue := flags.String("text", "", "text message content")
	filePath := flags.String("file", "", "media fixture path")
	confirmDataset := flags.String("confirm-dataset", "", "exact dataset confirmation for cleanup")
	enableSimulation := flags.Bool("enable-simulation", false, "explicitly enable local fixture requests")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if err := requireSimulationEnabled(*enableSimulation); err != nil {
		return err
	}
	bridgeURL := strings.TrimRight(strings.TrimSpace(getenv("MOCHAT_ARCHIVE_FIXTURE_URL")), "/")
	adminBearer := strings.TrimSpace(getenv("MOCHAT_ARCHIVE_FIXTURE_ADMIN_BEARER"))
	if bridgeURL == "" || len(adminBearer) < 40 {
		return errors.New("fixture bridge URL and independent admin bearer are required")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	var path string
	var request any
	var database *sql.DB
	var resolvedBinding archivebridge.Binding
	if action == "status" {
		path, request = "/v1/fixture/status", struct{}{}
	} else {
		if *tenantID <= 0 {
			return errors.New("--tenant-id is required")
		}
		requestedMode := strings.TrimSpace(*mode)
		if requestedMode != archivebridge.ModeSelfBuilt && requestedMode != archivebridge.ModeThirdPartyDelegated {
			return errors.New("--mode must match self_built or third_party_delegated")
		}
		dsn := strings.TrimSpace(getenv("MOCHAT_MYSQL_DSN"))
		if dsn == "" {
			return errors.New("MOCHAT_MYSQL_DSN is required")
		}
		db, err := open("mysql", dsn)
		if err != nil {
			return err
		}
		defer db.Close()
		database = db
		if err := db.PingContext(ctx); err != nil {
			return fmt.Errorf("connect database: %w", err)
		}
		var binding archivebridge.Binding
		if action == "seed" {
			binding, err = resolveSeedBinding(ctx, db, *tenantID, requestedMode)
		} else {
			binding, err = resolveBinding(ctx, db, *tenantID, requestedMode)
		}
		if err != nil {
			return err
		}
		resolvedBinding = binding
		switch action {
		case "seed":
			path, request = "/v1/fixture/seed", map[string]any{"binding": binding, "dataset": *dataset}
		case "send":
			input := map[string]any{"binding": binding, "dataset": *dataset, "type": strings.ToLower(strings.TrimSpace(*messageType)), "text": *textValue}
			if input["type"] != "text" {
				data, mimeType, fileName, err := readFixtureFile(*filePath)
				if err != nil {
					return err
				}
				input["dataBase64"], input["mimeType"], input["fileName"] = base64.StdEncoding.EncodeToString(data), mimeType, fileName
			}
			path, request = "/v1/fixture/send", input
		case "cleanup":
			if strings.TrimSpace(*confirmDataset) != strings.TrimSpace(*dataset) {
				return errors.New("cleanup is dry-run only until --confirm-dataset exactly matches --dataset")
			}
			path, request = "/v1/fixture/cleanup", map[string]any{"binding": binding, "dataset": *dataset, "confirmDataset": *confirmDataset}
		}
	}
	if action == "seed" {
		var bridgeOutput bytes.Buffer
		if err := postFixture(ctx, http.DefaultClient, bridgeURL+path, adminBearer, request, &bridgeOutput); err != nil {
			return err
		}
		if err := activateFixtureBinding(ctx, database, resolvedBinding, *dataset); err != nil {
			return err
		}
		var bridgeResult any
		if json.Unmarshal(bridgeOutput.Bytes(), &bridgeResult) != nil {
			return errors.New("fixture bridge returned invalid seed result")
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(map[string]any{"status": "seeded", "dataset": strings.TrimSpace(*dataset), "mode": resolvedBinding.IntegrationMode, "bridge": bridgeResult})
	}
	if action == "cleanup" {
		cleanup, err := cleanupIngestedFixture(ctx, database, resolvedBinding, *dataset, getenv("MOCHAT_FILE_STORAGE_ROOT"))
		if err != nil {
			return err
		}
		// Delete the durable downstream state first. If the bridge request then
		// fails, its retained upstream fixture can be replayed or cleaned by a
		// retry; deleting upstream first would make a downstream refusal harder
		// to recover from.
		if err := postFixture(ctx, http.DefaultClient, bridgeURL+path, adminBearer, request, io.Discard); err != nil {
			return err
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(map[string]any{"status": "cleaned", "cleanup": cleanup})
	}
	return postFixture(ctx, http.DefaultClient, bridgeURL+path, adminBearer, request, os.Stdout)
}

type bindingQuery interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func resolveBinding(ctx context.Context, db bindingQuery, tenantID int64, requestedMode string) (archivebridge.Binding, error) {
	var binding archivebridge.Binding
	err := db.QueryRowContext(ctx, `
		SELECT integration.tenant_id,integration.corp_id,integration.verified_wx_corpid,binding.wecom_integration_mode
		FROM mochat_go_wecom_integrations integration
		INNER JOIN mochat_go_tenant_corp_bindings binding ON binding.tenant_id=integration.tenant_id AND binding.corp_id=integration.corp_id
		INNER JOIN mc_tenant tenant ON tenant.id=integration.tenant_id AND tenant.deleted_at IS NULL
		INNER JOIN mc_corp corp ON corp.id=integration.corp_id AND corp.tenant_id=integration.tenant_id AND corp.deleted_at IS NULL
		WHERE integration.tenant_id=?
		  AND integration.slot='current' AND integration.status='active'
		  AND integration.mode=binding.wecom_integration_mode
		  AND binding.status=2 AND integration.verified_wx_corpid<>''
		LIMIT 1
	`, tenantID).Scan(&binding.TenantID, &binding.CorpID, &binding.WXCorpID, &binding.IntegrationMode)
	if errors.Is(err, sql.ErrNoRows) {
		return archivebridge.Binding{}, errors.New("tenant has no available enterprise binding")
	}
	if err != nil {
		return archivebridge.Binding{}, fmt.Errorf("resolve tenant enterprise binding: %w", err)
	}
	if binding.IntegrationMode != requestedMode {
		return archivebridge.Binding{}, fmt.Errorf("tenant authoritative mode is %s, not %s", binding.IntegrationMode, requestedMode)
	}
	return binding, nil
}

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

func postFixture(ctx context.Context, client httpDoer, target, bearer string, input any, output io.Writer) error {
	raw, err := json.Marshal(input)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+bearer)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("request fixture bridge: %w", err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("fixture bridge rejected request: HTTP %d", response.StatusCode)
	}
	var result any
	if json.Unmarshal(payload, &result) != nil {
		return errors.New("fixture bridge returned invalid JSON")
	}
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func readFixtureFile(path string) ([]byte, string, string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, "", "", errors.New("--file is required for media messages")
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() <= 0 || info.Size() > 20<<20 {
		return nil, "", "", errors.New("fixture file must be a non-empty regular file no larger than 20 MiB")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", "", errors.New("read fixture file failed")
	}
	return data, http.DetectContentType(data), filepath.Base(path), nil
}

func requireSimulationEnabled(enabled bool) error {
	if !enabled {
		return errors.New("simulation is disabled; pass --enable-simulation explicitly")
	}
	return nil
}
