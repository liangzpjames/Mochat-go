-- Phase 3 Final acceptance cleanup: soft-remove seeded archive messages and
-- hard-remove audio/AI-analysis acceptance rows (data created by P36-ACCEPT-*).
DELETE FROM mochat_go_ai_analysis
WHERE corp_id IN (SELECT id FROM mc_corp WHERE tenant_id = 1)
  AND page IN ('session-analysis','smart-analysis','emotion','employee-score','communication-keyword');

DELETE FROM mochat_go_audio_objects
WHERE original_name LIKE 'P36-ACCEPT-%' OR original_name LIKE 'p36-accept%';

DELETE FROM mc_work_message_1 WHERE msgid LIKE 'P36-ARCH-%';
