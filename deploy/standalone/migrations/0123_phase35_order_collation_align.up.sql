-- Phase 3.5 orders/settings tables were created with the server default
-- collation (utf8mb4_general_ci) while the rest of the SCRM schema uses
-- utf8mb4_unicode_ci. Queries joining mochat_go_scrm_orders with
-- mochat_go_scrm_contacts fail with "Illegal mix of collations".
-- Align the new tables to the schema-wide collation.
ALTER TABLE mochat_go_scrm_orders CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
ALTER TABLE mochat_go_scrm_order_audit CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
ALTER TABLE mochat_go_scrm_settings CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
