package repository

import (
	"context"
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

// URLRecord holds the resolved data for a given short code.
type URLRecord struct {
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

	// Step 1: Redis lookup.
	cached, err := r.rdb.Get(ctx, key).Result()
	if err == nil {
		// Cache hit. TTL on this key was set to match the URL's remaining lifetime,
		// so a hit here guarantees the URL is still valid — no expiry check needed.
		return &URLRecord{OriginalURL: cached}, nil
	}
	if !errors.Is(err, redis.Nil) {
		// Unexpected Redis error (connection refused, timeout, etc.).
		// Fall through to the DB; this upholds the resilience guarantee:
		// the system keeps working without Redis.
		// Production note: emit a warning metric/log here.
		_ = err
	}

	// Step 2: PostgreSQL fallback.
	const q = `
		SELECT original_url, expires_at
		FROM urls
		WHERE short_code = $1
		LIMIT 1
	`
	var record URLRecord
	err = r.db.QueryRow(ctx, q, shortCode).Scan(&record.OriginalURL, &record.ExpiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repository: db query failed: %w", err)
	}

	// Step 3: Populate cache — only for URLs that are still valid.
	// An expired URL will never be cached; the next request will always hit the DB
	// and the service layer will handle returning the 410 Gone response.
	if !record.IsExpired() {
		ttl := maxCacheTTL
		if record.ExpiresAt != nil {
			if remaining := time.Until(*record.ExpiresAt); remaining < maxCacheTTL {
				ttl = remaining
			}
		}
		// Ignore Redis write errors — cache is best-effort; DB is the source of truth.
		_ = r.rdb.Set(ctx, key, record.OriginalURL, ttl).Err()
	}

	return &record, nil
}

// IncrementClickCount atomically increments the Redis click counter for a short code.
// These counters are periodically synced to the click_count column in PostgreSQL
// by a background job (not implemented in this package), avoiding write amplification
// on the critical redirect path.
func (r *RedirectRepository) IncrementClickCount(ctx context.Context, shortCode string) error {
	key := clickKeyPrefix + shortCode
	if err := r.rdb.Incr(ctx, key).Err(); err != nil {
		return fmt.Errorf("repository: incr click count: %w", err)
	}
	return nil
}
