-- Migration: 0002_create_click_logs_table (rollback)

DROP INDEX IF EXISTS idx_click_logs_clicked_at;
DROP INDEX IF EXISTS idx_click_logs_url_id;
DROP TABLE IF EXISTS click_logs;
