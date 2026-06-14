package domain

import "time"

// ClickEvent merepresentasikan satu log klik di dalam antrean Redis
type ClickEvent struct {
	URLID     int64     `json:"url_id"`
	ShortCode string    `json:"short_code"` // Untuk mengupdate total_clicks
	UserAgent string    `json:"user_agent"`
	IPHash    string    `json:"ip_hash"`
	ClickedAt time.Time `json:"clicked_at"`
}

// service/redirect_service.go
type ResolveResult struct {
	URLID       int64
	OriginalURL string
	Expired     bool
}
