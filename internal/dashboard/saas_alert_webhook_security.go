package dashboard

import (
	"fmt"
	"strings"

	"jiyi/mochat-go/internal/outboundhttp"
)

var defaultSaaSAlertWebhookGuard = outboundhttp.MustDefaultGuard()

func normalizeSaaSAlertWebhookGuard(guard *outboundhttp.Guard) *outboundhttp.Guard {
	if guard == nil {
		return defaultSaaSAlertWebhookGuard
	}
	return guard
}

func validateSaaSAlertWebhookURLSecurity(guard *outboundhttp.Guard, raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if err := normalizeSaaSAlertWebhookGuard(guard).ValidateURL(raw); err != nil {
		return fieldError(fmt.Sprintf("webhookUrl 不符合出站安全策略: %v", err))
	}
	return nil
}

func saasAlertWebhookSecurityPayload(guard *outboundhttp.Guard) outboundhttp.Status {
	return normalizeSaaSAlertWebhookGuard(guard).Status()
}

func saasAlertWebhookURLSecurityPayload(guard *outboundhttp.Guard, raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	payload := map[string]any{
		"configured": raw != "",
		"allowed":    true,
		"error":      "",
	}
	if raw == "" {
		return payload
	}
	if err := normalizeSaaSAlertWebhookGuard(guard).ValidateURL(raw); err != nil {
		payload["allowed"] = false
		payload["error"] = err.Error()
	}
	return payload
}
