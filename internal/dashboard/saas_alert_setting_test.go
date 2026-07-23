package dashboard

import (
	"reflect"
	"testing"
)

func TestDefaultSaaSAlertSettingPreservesLegacyDeliveryPolicy(t *testing.T) {
	setting := DefaultSaaSAlertSetting(7, SaaSAlertNotificationChannelWebhook)
	if setting.MinimumSeverity != SaaSAlertSeverityWarning {
		t.Fatalf("minimum severity = %q", setting.MinimumSeverity)
	}
	if len(setting.AllowedAlertTypes) != 0 || setting.QuietHoursEnabled || setting.HourlyLimit != 0 {
		t.Fatalf("policy defaults = %+v", setting)
	}
	if setting.QuietHoursStart != "22:00" || setting.QuietHoursEnd != "08:00" || setting.Timezone != "Asia/Shanghai" {
		t.Fatalf("quiet defaults = %+v", setting)
	}
	if err := ValidateSaaSAlertSetting(setting); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeSaaSAlertSettingNormalizesAlertTypes(t *testing.T) {
	setting := NormalizeSaaSAlertSetting(SaaSAlertSetting{
		AllowedAlertTypes: []string{" Tenant_Renewal_Reminder ", "quota_exceeded", "quota_exceeded", ""},
	})
	want := []string{"quota_exceeded", "tenant_renewal_reminder"}
	if !reflect.DeepEqual(setting.AllowedAlertTypes, want) {
		t.Fatalf("alert types = %#v want %#v", setting.AllowedAlertTypes, want)
	}
}

func TestValidateSaaSAlertSettingRejectsInvalidPolicyControls(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*SaaSAlertSetting)
	}{
		{"minimum severity", func(setting *SaaSAlertSetting) { setting.MinimumSeverity = "info" }},
		{"alert type", func(setting *SaaSAlertSetting) { setting.AllowedAlertTypes = []string{"bad alert"} }},
		{"quiet start", func(setting *SaaSAlertSetting) { setting.QuietHoursStart = "24:00" }},
		{"equal quiet range", func(setting *SaaSAlertSetting) {
			setting.QuietHoursEnabled = true
			setting.QuietHoursStart = "08:00"
			setting.QuietHoursEnd = "08:00"
		}},
		{"timezone", func(setting *SaaSAlertSetting) { setting.Timezone = "Mars/Olympus" }},
		{"hourly limit", func(setting *SaaSAlertSetting) { setting.HourlyLimit = 10001 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setting := DefaultSaaSAlertSetting(7, SaaSAlertNotificationChannelWebhook)
			test.mutate(&setting)
			if err := ValidateSaaSAlertSetting(setting); err == nil {
				t.Fatalf("expected validation error for %+v", setting)
			}
		})
	}
}

func TestApplySaaSAlertSettingParamsReadsPolicyControls(t *testing.T) {
	setting, err := applySaaSAlertSettingParams(DefaultSaaSAlertSetting(7, SaaSAlertNotificationChannelWebhook), map[string]any{
		"minimumSeverity":   "critical",
		"alertTypes":        []any{"quota_exceeded", "tenant_renewal_reminder"},
		"quietHoursEnabled": true,
		"quietHoursStart":   "21:30",
		"quietHoursEnd":     "07:15",
		"timezone":          "Asia/Shanghai",
		"hourlyLimit":       float64(12),
	})
	if err != nil {
		t.Fatal(err)
	}
	if setting.MinimumSeverity != SaaSAlertSeverityCritical || !setting.QuietHoursEnabled || setting.HourlyLimit != 12 {
		t.Fatalf("setting = %+v", setting)
	}
	if setting.QuietHoursStart != "21:30" || setting.QuietHoursEnd != "07:15" || len(setting.AllowedAlertTypes) != 2 {
		t.Fatalf("setting = %+v", setting)
	}
}
