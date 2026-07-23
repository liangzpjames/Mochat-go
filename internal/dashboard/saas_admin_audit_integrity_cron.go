package dashboard

import (
	"context"
	"fmt"
	"log"
)

const SaaSAdminAuditIntegrityCronTaskName = "cron-saas-admin-audit-integrity"

type SaaSAdminAuditIntegrityVerifier interface {
	VerifySaaSAdminAuditIntegrity(context.Context, SaaSAdminAuditIntegrityOptions) (SaaSAdminAuditIntegrityVerifyResult, error)
}

type SaaSAdminAuditIntegrityCron struct {
	verifier SaaSAdminAuditIntegrityVerifier
	limit    int
	logger   *log.Logger
}

func NewSaaSAdminAuditIntegrityCron(verifier SaaSAdminAuditIntegrityVerifier, limit int, logger *log.Logger) *SaaSAdminAuditIntegrityCron {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if logger == nil {
		logger = log.Default()
	}
	return &SaaSAdminAuditIntegrityCron{verifier: verifier, limit: limit, logger: logger}
}

func (c *SaaSAdminAuditIntegrityCron) RunOnce(ctx context.Context) error {
	if c == nil || c.verifier == nil {
		return fmt.Errorf("SaaS audit integrity verifier is not configured")
	}
	result, err := c.verifier.VerifySaaSAdminAuditIntegrity(ctx, SaaSAdminAuditIntegrityOptions{
		Limit:  c.limit,
		Source: "cron",
	})
	if err != nil {
		return err
	}
	c.logger.Printf("SaaS audit integrity verification completed: scanned=%d sealed=%d healthy=%d failed=%d legacy=%d signed=%d verified=%d",
		result.ScannedChains, result.SealedChains, result.HealthyChains, result.FailedChains,
		result.LegacyLogs, result.SignedLogs, result.VerifiedLogs)
	if result.FailedChains > 0 {
		return fmt.Errorf("SaaS audit integrity verification detected %d failed chains", result.FailedChains)
	}
	return nil
}
