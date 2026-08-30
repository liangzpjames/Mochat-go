package main

import (
	"errors"
	"fmt"

	"jiyi/mochat-go/internal/config"
	"jiyi/mochat-go/internal/saasalertcredentials"
	"jiyi/mochat-go/internal/wechatopencredentials"
	"jiyi/mochat-go/internal/wecomcredentials"
)

type runtimeCredentialManagers struct {
	alerts           *saasalertcredentials.Manager
	weCom            *wecomcredentials.Manager
	weChatOpen       *wechatopencredentials.Manager
	alertStatus      saasalertcredentials.ConfigStatus
	weComStatus      wecomcredentials.ConfigStatus
	weChatOpenStatus wechatopencredentials.ConfigStatus
}

func buildRuntimeCredentialManagers(cfg config.Config) (runtimeCredentialManagers, error) {
	alerts, err := saasalertcredentials.NewManager(saasalertcredentials.Config{EncryptionKey: cfg.SaaSAlertCredentialEncryptionKey, EncryptionKeys: cfg.SaaSAlertCredentialEncryptionKeys, EncryptionKeyID: cfg.SaaSAlertCredentialEncryptionKeyID, RequireEncryption: cfg.SaaSAlertCredentialRequireEncryption, DedicatedConfigured: cfg.SaaSAlertCredentialDedicatedConfigured})
	if err != nil {
		return runtimeCredentialManagers{}, fmt.Errorf("build SaaS alert credential encryption manager: %w", err)
	}
	alertStatus := alerts.ConfigStatus()
	debugf("SaaS alert credential protection: encryption_configured=%t require_encryption=%t dedicated_configured=%t active_key_id=%s key_count=%d", alertStatus.EncryptionConfigured, alertStatus.RequireEncryption, alertStatus.DedicatedConfigured, alertStatus.ActiveKeyID, alertStatus.KeyCount)

	weCom, err := wecomcredentials.NewManager(wecomcredentials.Config{EncryptionKey: cfg.WeComCredentialEncryptionKey, EncryptionKeys: cfg.WeComCredentialEncryptionKeys, EncryptionKeyID: cfg.WeComCredentialEncryptionKeyID, RequireEncryption: cfg.WeComCredentialRequireEncryption, DedicatedConfigured: cfg.WeComCredentialDedicatedConfigured})
	if err != nil {
		return runtimeCredentialManagers{}, fmt.Errorf("build WeCom credential encryption manager: %w", err)
	}
	weComStatus := weCom.ConfigStatus()
	if cfg.EnableDurableWorkMessageArchive && !weComStatus.EncryptionConfigured {
		return runtimeCredentialManagers{}, errors.New("durable work message archive requires configured WeCom credential encryption")
	}
	debugf("WeCom credential protection: encryption_configured=%t require_encryption=%t dedicated_configured=%t active_key_id=%s key_count=%d", weComStatus.EncryptionConfigured, weComStatus.RequireEncryption, weComStatus.DedicatedConfigured, weComStatus.ActiveKeyID, weComStatus.KeyCount)

	weChatOpen, err := wechatopencredentials.NewManager(wechatopencredentials.Config{EncryptionKey: cfg.WeChatOpenCredentialEncryptionKey, EncryptionKeys: cfg.WeChatOpenCredentialEncryptionKeys, EncryptionKeyID: cfg.WeChatOpenCredentialEncryptionKeyID, RequireEncryption: cfg.WeChatOpenCredentialRequireEncryption, DedicatedConfigured: cfg.WeChatOpenCredentialDedicatedConfigured})
	if err != nil {
		return runtimeCredentialManagers{}, fmt.Errorf("build WeChat Open credential encryption manager: %w", err)
	}
	weChatOpenStatus := weChatOpen.ConfigStatus()
	debugf("WeChat Open credential protection: encryption_configured=%t require_encryption=%t dedicated_configured=%t active_key_id=%s key_count=%d", weChatOpenStatus.EncryptionConfigured, weChatOpenStatus.RequireEncryption, weChatOpenStatus.DedicatedConfigured, weChatOpenStatus.ActiveKeyID, weChatOpenStatus.KeyCount)
	return runtimeCredentialManagers{alerts: alerts, weCom: weCom, weChatOpen: weChatOpen, alertStatus: alertStatus, weComStatus: weComStatus, weChatOpenStatus: weChatOpenStatus}, nil
}
