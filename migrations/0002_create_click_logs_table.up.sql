-- Migration: 0002_create_click_logs_table
-- Optional analytics table: one row per redirect/click event.

CREATE TABLE IF NOT EXISTS click_logs (
    id         BIGSERIAL PRIMARY KEY,
    url_id     BIGINT      NOT NULL,
    clicked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    user_agent TEXT        NULL,
    ip_hash    VARCHAR(64) NULL,

    CONSTRAINT fk_click_logs_url
        FOREIGN KEY (url_id) REFERENCES urls (id)
        ON DELETE CASCADE
);

-- Index to support per-URL analytics queries.
CREATE INDEX IF NOT EXISTS idx_click_logs_url_id ON click_logs (url_id);

-- Index to support time-range analytics queries.
CREATE INDEX IF NOT EXISTS idx_click_logs_clicked_at ON click_logs (clicked_at);
