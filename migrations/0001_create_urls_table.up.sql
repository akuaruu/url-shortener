-- Migration: 0001_create_urls_table
-- Creates the core table for storing shortened URLs.

CREATE TABLE IF NOT EXISTS urls (
    id           BIGSERIAL PRIMARY KEY,
    short_code   VARCHAR(10) NOT NULL,
    original_url TEXT        NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at   TIMESTAMPTZ NULL,
    click_count  BIGINT      NOT NULL DEFAULT 0,

    CONSTRAINT uq_urls_short_code UNIQUE (short_code)
);

-- Index for fast lookup during redirect (cache-miss fallback path).
CREATE INDEX IF NOT EXISTS idx_urls_short_code ON urls (short_code);

-- Index to support cleanup/maintenance jobs for expired URLs.
CREATE INDEX IF NOT EXISTS idx_urls_expires_at ON urls (expires_at)
    WHERE expires_at IS NOT NULL;
