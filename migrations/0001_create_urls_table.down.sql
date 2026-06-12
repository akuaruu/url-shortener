-- Migration: 0001_create_urls_table (rollback)

DROP INDEX IF EXISTS idx_urls_expires_at;
DROP INDEX IF EXISTS idx_urls_short_code;
DROP TABLE IF EXISTS urls;
