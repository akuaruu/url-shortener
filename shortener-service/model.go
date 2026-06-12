package model

import "time"

// URL represents a row in the `urls` table.
type URL struct {
	ID          int64
	ShortCode   string
	OriginalURL string
	CreatedAt   time.Time
	ExpiresAt   *time.Time // nil means no expiry
	ClickCount  int64
}

// IsExpired reports whether the URL has passed its expiry time.
func (u *URL) IsExpired(now time.Time) bool {
	return u.ExpiresAt != nil && now.After(*u.ExpiresAt)
}
