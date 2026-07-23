-- Support cross-tenant notification health windows without scanning the full outbox.

ALTER TABLE `mochat_go_saas_alert_notifications`
  ADD KEY `idx_mochat_go_saas_alert_notifications_health_window` (`tenant_id`, `channel`, `created_at`, `status`);
