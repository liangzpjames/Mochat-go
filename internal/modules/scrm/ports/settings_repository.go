package ports

import (
	"context"
	"time"
)

type SCRMSetting struct {
	ID        string    `json:"id"`
	TenantID  int64     `json:"tenantId"`
	CorpID    int64     `json:"corpId"`
	Type      string    `json:"type"`
	Key       string    `json:"key"`
	Label     string    `json:"label"`
	Value     any       `json:"value"`
	Enabled   bool      `json:"enabled"`
	Version   int64     `json:"version"`
	UpdatedBy int64     `json:"updatedBy"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type SettingsRepository interface {
	List(context.Context, int64, int64, string) ([]SCRMSetting, error)
	Upsert(context.Context, SCRMSetting, int64) (SCRMSetting, error)
}
