package dashboard

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"
)

type fakeSaaSAdminAuditIntegrityVerifier struct {
	options SaaSAdminAuditIntegrityOptions
	result  SaaSAdminAuditIntegrityVerifyResult
}

func (f *fakeSaaSAdminAuditIntegrityVerifier) VerifySaaSAdminAuditIntegrity(_ context.Context, options SaaSAdminAuditIntegrityOptions) (SaaSAdminAuditIntegrityVerifyResult, error) {
	f.options = options
	return f.result, nil
}

func TestSaaSAdminAuditIntegrityCron(t *testing.T) {
	var output bytes.Buffer
	verifier := &fakeSaaSAdminAuditIntegrityVerifier{result: SaaSAdminAuditIntegrityVerifyResult{ScannedChains: 2, HealthyChains: 2, VerifiedLogs: 4}}
	cron := NewSaaSAdminAuditIntegrityCron(verifier, 25, log.New(&output, "", 0))
	if err := cron.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if verifier.options.Source != "cron" || verifier.options.Limit != 25 || !strings.Contains(output.String(), "scanned=2") {
		t.Fatalf("options=%+v log=%q", verifier.options, output.String())
	}

	verifier.result = SaaSAdminAuditIntegrityVerifyResult{ScannedChains: 2, HealthyChains: 1, FailedChains: 1}
	if err := cron.RunOnce(context.Background()); err == nil || !strings.Contains(err.Error(), "1 failed chains") {
		t.Fatalf("err=%v", err)
	}
}
