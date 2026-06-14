package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// ErrNotFound is returned when the short code cannot be found in cache or DB.
var ErrNotFound = errors.New("repository: url not found")

const (
	cacheKeyPrefix = "url:"
	clickKeyPrefix = "clicks:"
	maxCacheTTL    = 24 * time.Hour
)

// cachedRecord adalah struktur yang disimpan sebagai value di Redis.
// Sengaja dibuat minimal untuk menjaga memory footprint cache tetap kecil.
type cachedRecord struct {
	ID          int64  `json:"i"`
	OriginalURL string `json:"u"`
}

// URLRecord holds the resolved data for a given short code.
type URLRecord struct {
	ID          int64
	OriginalURL string
	ExpiresAt   *time.Time
}

// IsExpired reports whether the URL has passed its expiry time.
// Returns false when ExpiresAt is nil (URL has no expiry).
func (r *URLRecord) IsExpired() bool {
	if r.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*r.ExpiresAt)
}

// RedirectRepository handles all data access for the redirect service.
// It implements the cache-aside pattern: Redis first, PostgreSQL as fallback.
type RedirectRepository struct {
	db  *pgxpool.Pool
	rdb *redis.Client
}

// NewRedirectRepository constructs a RedirectRepository.
func NewRedirectRepository(db *pgxpool.Pool, rdb *redis.Client) *RedirectRepository {
	return &RedirectRepository{db: db, rdb: rdb}
}

// Resolve resolves a short code to a URLRecord using cache-aside:
//
//  1. Check Redis — return immediately on cache hit.
//     Redis TTL guarantees the cached URL has not expired, so ExpiresAt is nil on hit.
//  2. On miss (or Redis unavailable), query PostgreSQL.
//  3. If the DB record is not yet expired, populate Redis for subsequent requests.
//     TTL = min(remaining URL lifetime, 24h).
func (r *RedirectRepository) Resolve(ctx context.Context, shortCode string) (*URLRecord, error) {
	key := cacheKeyPrefix + shortCode

	// Step 1: Redis lookup
	cached, err := r.rdb.Get(ctx, key).Result()
	if err == nil {
		var cr cachedRecord
		if jsonErr := json.Unmarshal([]byte(cached), &cr); jsonErr == nil {
			return &URLRecord{
				ID:          cr.ID,
				OriginalURL: cr.OriginalURL,
			}, nil
		}
		// JSON parse gagal (mungkin data lama format plain string) — fall through ke DB
	}

	if !errors.Is(err, redis.Nil) {
		_ = err // Redis error non-fatal, fall through
	}

	// Step 2: PostgreSQL fallback
	const q = `
        SELECT id, original_url, expires_at
        FROM urls
        WHERE short_code = $1
        LIMIT 1
    `
	var record URLRecord
	err = r.db.QueryRow(ctx, q, shortCode).Scan(
		&record.ID,
		&record.OriginalURL,
		&record.ExpiresAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repository: db query failed: %w", err)
	}

	// Step 3: Populate cache dengan format JSON baru
	if !record.IsExpired() {
		ttl := maxCacheTTL
		if record.ExpiresAt != nil {
			if remaining := time.Until(*record.ExpiresAt); remaining < maxCacheTTL {
				ttl = remaining
			}
		}

		if payload, jsonErr := json.Marshal(cachedRecord{
			ID:          record.ID,
			OriginalURL: record.OriginalURL,
		}); jsonErr == nil {
			_ = r.rdb.Set(ctx, key, payload, ttl).Err()
		}
	}

	return &record, nil
}
