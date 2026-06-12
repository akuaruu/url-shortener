package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/akuaruu/url-shortener/pkg/shortcode"
)

// ErrNotFound is returned when a URL record does not exist.
var ErrNotFound = errors.New("repository: url not found")

const cacheKeyPrefix = "url:"

// URLRepository persists URL records to PostgreSQL and populates the Redis
// cache used by the Redirect Service for fast lookups (cache-aside pattern,
// write-through on creation).
type URLRepository struct {
	db    *pgxpool.Pool
	cache *redis.Client
}

// NewURLRepository constructs a URLRepository.
func NewURLRepository(db *pgxpool.Pool, cache *redis.Client) *URLRepository {
	return &URLRepository{db: db, cache: cache}
}

// CreateURL inserts a new URL row and returns the generated record,
// including its Base62-encoded short code derived from the new row's id.
//
// It also populates the Redis cache (write-through) so the Redirect Service
// can serve the first request without a cache miss.
func (r *URLRepository) CreateURL(ctx context.Context, originalURL string, expiresAt *time.Time) (*URLRecord, error) {
	var id int64
	var createdAt time.Time

	err := r.db.QueryRow(
		ctx,
		`INSERT INTO urls (short_code, original_url, expires_at)
		 VALUES ($1, $2, $3)
		 RETURNING id, created_at`,
		// short_code is populated in a second step once we know the id (see below),
		// so we insert a temporary placeholder here and update it.
		"", originalURL, expiresAt,
	).Scan(&id, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("insert url: %w", err)
	}

	code := shortcode.Encode(uint64(id))

	if _, err := r.db.Exec(ctx, `UPDATE urls SET short_code = $1 WHERE id = $2`, code, id); err != nil {
		return nil, fmt.Errorf("update short_code: %w", err)
	}

	record := &URLRecord{
		ID:          id,
		ShortCode:   code,
		OriginalURL: originalURL,
		CreatedAt:   createdAt,
		ExpiresAt:   expiresAt,
		ClickCount:  0,
	}

	// Write-through cache population. Best-effort: a cache error here should
	// not fail URL creation, since the Redirect Service falls back to
	// PostgreSQL on a cache miss (cache-aside).
	ttl := cacheTTL(expiresAt)
	if err := r.cache.Set(ctx, cacheKeyPrefix+code, originalURL, ttl).Err(); err != nil {
		// Intentionally not returning an error; caching is best-effort.
		// In production this would be logged via structured logging/metrics.
		_ = err
	}

	return record, nil
}

// GetByShortCode retrieves a URL record by its short code.
// Returns ErrNotFound if no matching row exists.
func (r *URLRepository) GetByShortCode(ctx context.Context, code string) (*URLRecord, error) {
	row := r.db.QueryRow(
		ctx,
		`SELECT id, short_code, original_url, created_at, expires_at, click_count
		 FROM urls WHERE short_code = $1`,
		code,
	)

	var rec URLRecord
	if err := row.Scan(&rec.ID, &rec.ShortCode, &rec.OriginalURL, &rec.CreatedAt, &rec.ExpiresAt, &rec.ClickCount); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("select url: %w", err)
	}

	return &rec, nil
}

// URLRecord mirrors the `urls` table row.
type URLRecord struct {
	ID          int64
	ShortCode   string
	OriginalURL string
	CreatedAt   time.Time
	ExpiresAt   *time.Time
	ClickCount  int64
}

// cacheTTL returns the Redis TTL for a cache entry, capped at 24h, per the
// caching strategy in PLANNING.md (cache TTL must not exceed URL expiry).
func cacheTTL(expiresAt *time.Time) time.Duration {
	const maxTTL = 24 * time.Hour

	if expiresAt == nil {
		return maxTTL
	}

	remaining := time.Until(*expiresAt)
	if remaining <= 0 {
		// Already expired; do not cache (TTL of 0 means "no expiration" in
		// go-redis, so use a tiny positive value to effectively skip caching).
		return time.Second
	}

	if remaining > maxTTL {
		return maxTTL
	}

	return remaining
}
