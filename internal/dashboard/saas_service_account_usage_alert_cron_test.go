package dashboard

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"
)

type fakeSaaSServiceAccountUsageAlertEvaluator struct {
	options SaaSServiceAccountUsageAlertEvaluateOptions
	result  SaaSServiceAccountUsageAlertEvaluateResult
	err     error
}

func (f *fakeSaaSServiceAccountUsageAlertEvaluator) EvaluateSaaSServiceAccountUsageAlerts(_ context.Context, options SaaSServiceAccountUsageAlertEvaluateOptions) (SaaSServiceAccountUsageAlertEvaluateResult, error) {
	f.options = options
	return f.result, f.err
}

func TestSaaSServiceAccountUsageAlertCron(t *testing.T) {
	var output bytes.Buffer
	evaluator := &fakeSaaSServiceAccountUsageAlertEvaluator{result: SaaSServiceAccountUsageAlertEvaluateResult{
		ScannedAccounts: 3, EligibleAccounts: 2, DisabledPolicies: 1, UsageWarningAccounts: 1,
		RejectionWarningAccounts: 1, ResolvedAlerts: 2, NotificationsQueued: 2, NotificationsClosed: 1,
	}}
	cron := NewSaaSServiceAccountUsageAlertCron(evaluator, 25, log.New(&output, "", 0))
	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if evaluator.options.Limit != 25 || evaluator.options.Source != "cron" || !strings.Contains(output.String(), "usage_warnings=1 rejection_warnings=1 resolved=2 notifications=2 notifications_closed=1") {
		t.Fatalf("options=%+v log=%q", evaluator.options, output.String())
	}
	failing := &fakeSaaSServiceAccountUsageAlertEvaluator{err: errors.New("evaluate failed")}
	if err := NewSaaSServiceAccountUsageAlertCron(failing, 0, nil).RunOnce(context.Background()); err == nil || failing.options.Limit != 100 {
		t.Fatalf("err=%v options=%+v", err, failing.options)
	}
}
