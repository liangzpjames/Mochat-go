ALTER TABLE mc_corp_day_data
  ADD INDEX idx_mc_corp_day_data_corp_date (corp_id, date);
