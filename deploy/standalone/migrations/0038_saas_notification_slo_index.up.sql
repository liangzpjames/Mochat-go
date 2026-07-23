-- Support platform-wide notification SLO trends by channel and creation cohort.

ALTER TABLE `mochat_go_saas_alert_notifications`
  ADD KEY `idx_mochat_go_saas_alert_notifications_slo_window` (`channel`, `created_at`, `tenant_id`, `status`, `delivered_at`);
