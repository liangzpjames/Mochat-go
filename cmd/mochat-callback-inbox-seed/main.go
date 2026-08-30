package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/store"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	if os.Getenv("MOCHAT_GO_ALLOW_CALLBACK_INBOX_SEED") != "1" {
		fatalf("callback inbox seed is disabled; set MOCHAT_GO_ALLOW_CALLBACK_INBOX_SEED=1 only in an isolated smoke environment")
	}
	dsn := flag.String("dsn", "", "isolated smoke MySQL DSN")
	eventPath := flag.String("event", "", "callback event JSON file")
	tenantID := flag.Int("tenant-id", 0, "tenant id assigned to the controlled event")
	mode := flag.String("mode", "accept", "accept or exercise-lifecycle")
	flag.Parse()
	if strings.TrimSpace(*dsn) == "" || *tenantID <= 0 {
		fatalf("dsn and positive tenant-id are required")
	}
	db, err := sql.Open("mysql", *dsn)
	if err != nil {
		fatalf("open mysql: %v", err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		fatalf("ping mysql: %v", err)
	}
	inbox := store.NewMySQLStore(db)
	switch strings.TrimSpace(*mode) {
	case "accept":
		if strings.TrimSpace(*eventPath) == "" {
			fatalf("event path is required in accept mode")
		}
		raw, err := os.ReadFile(*eventPath)
		if err != nil {
			fatalf("read event: %v", err)
		}
		var event dashboard.WeWorkCallbackEvent
		if err := json.Unmarshal(raw, &event); err != nil {
			fatalf("decode event: %v", err)
		}
		event.TenantID = *tenantID
		event.RawXML = ""
		replayed, err := accept(ctx, inbox, event)
		if err != nil {
			fatalf("accept callback event: %v", err)
		}
		fmt.Printf("callback inbox seed accepted replayed=%t event_key=%s\n", replayed, dashboard.WeWorkCallbackEventKey(event))
	case "exercise-lifecycle":
		if err := exerciseLifecycle(ctx, inbox, *tenantID); err != nil {
			fatalf("exercise callback inbox lifecycle: %v", err)
		}
		fmt.Println("callback inbox lifecycle exercised")
	default:
		fatalf("unknown mode %q", *mode)
	}
}

type callbackInbox interface {
	dashboard.WeWorkCallbackInbox
}

func accept(ctx context.Context, inbox callbackInbox, event dashboard.WeWorkCallbackEvent) (bool, error) {
	return inbox.AcceptWeWorkCallback(ctx, event, dashboard.WeWorkCallbackEventKey(event), dashboard.WeWorkCallbackPayloadFingerprint(event))
}

func exerciseLifecycle(ctx context.Context, inbox callbackInbox, tenantID int) error {
	completed := lifecycleEvent(tenantID, "smoke-completed")
	if _, err := accept(ctx, inbox, completed); err != nil {
		return err
	}
	completedClaim, found, err := inbox.ClaimWeWorkCallback(ctx, time.Minute, 3)
	if err != nil {
		return fmt.Errorf("claim completed fixture: %w", err)
	}
	if !found {
		return fmt.Errorf("claim completed fixture: no event found")
	}
	if err := inbox.CompleteWeWorkCallback(ctx, completedClaim); err != nil {
		return err
	}

	dead := lifecycleEvent(tenantID, "smoke-dead")
	if _, err := accept(ctx, inbox, dead); err != nil {
		return err
	}
	deadClaim, found, err := inbox.ClaimWeWorkCallback(ctx, time.Minute, 1)
	if err != nil {
		return fmt.Errorf("claim dead fixture: %w", err)
	}
	if !found {
		return fmt.Errorf("claim dead fixture: no event found")
	}
	if _, err := inbox.FailWeWorkCallback(ctx, deadClaim, "controlled smoke failure", 1, 0); err != nil {
		return err
	}

	recovered := lifecycleEvent(tenantID, "smoke-recovered")
	if _, err := accept(ctx, inbox, recovered); err != nil {
		return err
	}
	first, found, err := inbox.ClaimWeWorkCallback(ctx, 5*time.Millisecond, 2)
	if err != nil {
		return fmt.Errorf("claim recovery fixture: %w", err)
	}
	if !found {
		return fmt.Errorf("claim recovery fixture: no event found")
	}
	time.Sleep(20 * time.Millisecond)
	second, found, err := inbox.ClaimWeWorkCallback(ctx, time.Minute, 2)
	if err != nil {
		return fmt.Errorf("reclaim recovery fixture: %w", err)
	}
	if !found {
		return fmt.Errorf("reclaim recovery fixture: no event found")
	}
	if second.EventKey != first.EventKey || second.LeaseFence <= first.LeaseFence {
		return fmt.Errorf("reclaim recovery fixture: first_key=%q second_key=%q first_fence=%d second_fence=%d", first.EventKey, second.EventKey, first.LeaseFence, second.LeaseFence)
	}
	return inbox.CompleteWeWorkCallback(ctx, second)
}

func lifecycleEvent(tenantID int, msgID string) dashboard.WeWorkCallbackEvent {
	return dashboard.WeWorkCallbackEvent{
		TenantID: tenantID, CorpID: 1, WxCorpID: "ww-worker", EventPath: "event.ignored",
		Message: map[string]string{"MsgId": msgID}, ReceivedAt: time.Now().Format("2006-01-02 15:04:05"),
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
