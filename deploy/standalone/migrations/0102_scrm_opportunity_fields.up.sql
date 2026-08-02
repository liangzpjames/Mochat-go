ALTER TABLE mochat_go_scrm_opportunities
  ADD COLUMN amount decimal(18,2) NOT NULL DEFAULT 0,
  ADD COLUMN start_date date NULL,
  ADD COLUMN end_date date NULL;
