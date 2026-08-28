package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"jiyi/mochat-go/internal/archivebridge"
	"jiyi/mochat-go/internal/observability"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	logger, err := observability.ConfigureFromEnv("mochat-archive-simulator")
	if err != nil {
		fmt.Fprintln(os.Stderr, "日志配置无效；请检查 MOCHAT_LOG_LEVEL 和 MOCHAT_LOG_FORMAT")
		os.Exit(1)
	}
	slog.SetDefault(logger)
	if err := run(os.Args[1:]); err != nil {
		logger.Error("会话存档模拟任务失败；请检查动作参数、fixture bridge 和数据库",
			"event", "archive_simulation_failed", "component", "archive", "step", "simulate", "result", "failed",
			"error_code", "ARCHIVE_SIMULATION_FAILED", "error", observability.SanitizeText(err.Error()))
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
	dryRun := flags.Bool("dry-run", false, "preview cleanup without deleting data")
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
	if action == "seed" {
		if err := ensureFixtureSourceOwnership(ctx, db, binding, *dataset); err != nil {
			return err
		}
		if err := claimFixtureDataset(ctx, db, binding, *dataset); err != nil {
			return err
		}
	}
	resolvedBinding = binding
	switch action {
	case "seed":
		path, request = "/v1/fixture/seed", map[string]any{"binding": binding, "dataset": *dataset}
	case "send":
		kind := strings.ToLower(strings.TrimSpace(*messageType))
		if err := validateSendArguments(kind, *textValue, *filePath); err != nil {
			return err
		}
		input := map[string]any{"binding": binding, "dataset": *dataset, "type": kind, "text": *textValue}
		if kind != "text" {
			inputRoot := strings.TrimSpace(getenv("MOCHAT_ARCHIVE_FIXTURE_INPUT_ROOT"))
			if inputRoot == "" {
				inputRoot = "/fixtures/input"
			}
			data, mimeType, fileName, err := readFixtureFile(*filePath, inputRoot, kind)
			if err != nil {
				return err
			}
			input["dataBase64"], input["mimeType"], input["fileName"] = base64.StdEncoding.EncodeToString(data), mimeType, fileName
		}
		path, request = "/v1/fixture/send", input
	case "cleanup":
		if *dryRun {
			path, request = "/v1/fixture/status", map[string]any{"binding": binding, "dataset": *dataset}
			break
		}
		if strings.TrimSpace(*confirmDataset) != strings.TrimSpace(*dataset) {
			return errors.New("cleanup requires --dry-run or an exact --confirm-dataset match")
		}
		path, request = "/v1/fixture/cleanup", map[string]any{"binding": binding, "dataset": *dataset, "confirmDataset": *confirmDataset}
	case "status":
		path, request = "/v1/fixture/status", map[string]any{"binding": binding, "dataset": *dataset}
	}
	if action == "seed" {
		if resolvedBinding.IntegrationMode == archivebridge.ModeThirdPartyDelegated {
			if err := prepareDelegatedFixtureAuthorization(ctx, database, resolvedBinding); err != nil {
				return err
			}
		}
		var bridgeOutput bytes.Buffer
		if err := postFixture(ctx, http.DefaultClient, bridgeURL+path, adminBearer, request, &bridgeOutput); err != nil {
			return err
		}
		var bridgeResult struct {
			DelegatedAuthorizedCount int                             `json:"delegatedAuthorizedCount"`
			Callbacks                []archivebridge.FixtureCallback `json:"callbacks"`
		}
		if json.Unmarshal(bridgeOutput.Bytes(), &bridgeResult) != nil {
			return errors.New("fixture bridge returned invalid seed result")
		}
		if resolvedBinding.IntegrationMode == archivebridge.ModeThirdPartyDelegated {
			if bridgeResult.DelegatedAuthorizedCount == 0 {
				callbackURL := strings.TrimSpace(getenv("MOCHAT_ARCHIVE_FIXTURE_CALLBACK_URL"))
				if callbackURL == "" || len(bridgeResult.Callbacks) != 2 {
					return errors.New("delegated fixture callback boundary is unavailable")
				}
				if err := dispatchFixtureCallbacks(ctx, http.DefaultClient, callbackURL, bridgeResult.Callbacks); err != nil {
					return err
				}
			}
		}
		if err := activateFixtureBinding(ctx, database, resolvedBinding, *dataset); err != nil {
			return err
		}
		if err := updateFixtureDatasetStatus(ctx, database, resolvedBinding, *dataset, "ready"); err != nil {
			return err
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(map[string]any{"status": "seeded", "dataset": strings.TrimSpace(*dataset), "mode": resolvedBinding.IntegrationMode, "bridge": safeFixtureBridgeSummary(bridgeResult.DelegatedAuthorizedCount, bridgeResult.Callbacks)})
	}
	if action == "cleanup" {
		if *dryRun {
			return postFixture(ctx, http.DefaultClient, bridgeURL+path, adminBearer, request, os.Stdout)
		}
		if err := updateFixtureDatasetStatus(ctx, database, resolvedBinding, *dataset, "cleaning"); err != nil {
			return err
		}
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
		if err := deleteFixtureDataset(ctx, database, resolvedBinding, *dataset); err != nil {
			return err
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(map[string]any{"status": "cleaned", "cleanup": cleanup})
	}
	return postFixture(ctx, http.DefaultClient, bridgeURL+path, adminBearer, request, os.Stdout)
}

func safeFixtureBridgeSummary(delegatedAuthorizedCount int, callbacks []archivebridge.FixtureCallback) map[string]any {
	eventTypes := make([]string, 0, len(callbacks))
	for _, callback := range callbacks {
		if eventType := strings.TrimSpace(callback.EventType); eventType != "" {
			eventTypes = append(eventTypes, eventType)
		}
	}
	return map[string]any{
		"delegatedAuthorizedCount": delegatedAuthorizedCount,
		"callbackCount":            len(callbacks),
		"callbackEventTypes":       eventTypes,
	}
}

func dispatchFixtureCallbacks(ctx context.Context, client httpDoer, callbackURL string, callbacks []archivebridge.FixtureCallback) error {
	callbackURL = strings.TrimSpace(callbackURL)
	if client == nil || callbackURL == "" || len(callbacks) == 0 {
		return errors.New("fixture callback delivery is unavailable")
	}
	for _, callback := range callbacks {
		body, err := xml.Marshal(struct {
			XMLName xml.Name `xml:"xml"`
			Encrypt string   `xml:"Encrypt"`
		}{Encrypt: callback.Encrypted})
		if err != nil {
			return errors.New("encode fixture callback failed")
		}
		values := callback.Values()
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, callbackURL+"?"+values.Encode(), bytes.NewReader(body))
		if err != nil {
			return errors.New("build fixture callback failed")
		}
		request.Header.Set("Content-Type", "application/xml")
		response, err := client.Do(request)
		if err != nil {
			return fmt.Errorf("deliver fixture callback: %w", err)
		}
		payload, readErr := io.ReadAll(io.LimitReader(response.Body, 1024))
		response.Body.Close()
		if readErr != nil || response.StatusCode != http.StatusOK || strings.TrimSpace(string(payload)) != "success" {
			return fmt.Errorf("fixture callback %s rejected: HTTP %d", callback.EventType, response.StatusCode)
		}
	}
	return nil
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

func readFixtureFile(path, inputRoot, messageType string) ([]byte, string, string, error) {
	path, inputRoot = strings.TrimSpace(path), strings.TrimSpace(inputRoot)
	if path == "" || inputRoot == "" {
		return nil, "", "", errors.New("--file is required for media messages")
	}
	rootPath, err := filepath.EvalSymlinks(inputRoot)
	if err != nil {
		return nil, "", "", errors.New("fixture input root is unavailable")
	}
	rootPath, err = filepath.Abs(rootPath)
	if err != nil {
		return nil, "", "", errors.New("fixture input root is unavailable")
	}
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, "", "", errors.New("fixture file is unavailable")
	}
	resolvedPath, err = filepath.Abs(resolvedPath)
	if err != nil {
		return nil, "", "", errors.New("fixture file is unavailable")
	}
	relative, err := filepath.Rel(rootPath, resolvedPath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return nil, "", "", errors.New("fixture file must stay inside the read-only input directory")
	}
	info, err := os.Stat(resolvedPath)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 20<<20 {
		return nil, "", "", errors.New("fixture file must be a non-empty regular file no larger than 20 MiB")
	}
	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return nil, "", "", errors.New("read fixture file failed")
	}
	mimeType := strings.ToLower(strings.TrimSpace(strings.Split(http.DetectContentType(data), ";")[0]))
	if !fixtureMIMEAllowed(messageType, mimeType) {
		return nil, "", "", errors.New("fixture file type does not match the requested message type")
	}
	return data, mimeType, filepath.Base(resolvedPath), nil
}

func validateSendArguments(messageType, text, path string) error {
	messageType, text, path = strings.ToLower(strings.TrimSpace(messageType)), strings.TrimSpace(text), strings.TrimSpace(path)
	switch messageType {
	case "text":
		if text == "" || path != "" {
			return errors.New("text messages require --text and must not include --file")
		}
	case "image", "voice", "video", "file":
		if text != "" || path == "" {
			return errors.New("media messages require --file and must not include --text")
		}
	default:
		return errors.New("--type must be text, image, voice, video or file")
	}
	return nil
}

func fixtureMIMEAllowed(messageType, mimeType string) bool {
	switch messageType {
	case "image":
		return strings.HasPrefix(mimeType, "image/")
	case "voice":
		return strings.HasPrefix(mimeType, "audio/") || mimeType == "application/ogg"
	case "video":
		return strings.HasPrefix(mimeType, "video/")
	case "file":
		return mimeType != "" && mimeType != "application/x-dosexec"
	default:
		return false
	}
}

func requireSimulationEnabled(enabled bool) error {
	if !enabled {
		return errors.New("simulation is disabled; pass --enable-simulation explicitly")
	}
	return nil
}
