package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/akuaruu/url-shortener/shortener-service/internal/repository"
)

// ErrInvalidURL is returned when the provided original URL is not a valid
// absolute http(s) URL.
var ErrInvalidURL = errors.New("service: invalid url")

// ErrNotFound is re-exported from the repository layer for handler use,
// keeping handlers decoupled from repository internals.
var ErrNotFound = repository.ErrNotFound

// ShortenerService implements the business logic for creating and
// retrieving shortened URLs.
type ShortenerService struct {
	repo            *repository.URLRepository
	baseRedirectURL string
}

// NewShortenerService constructs a ShortenerService.
// baseRedirectURL is prefixed to short codes when building the full short URL
// (e.g. "https://short.ly").
func NewShortenerService(repo *repository.URLRepository, baseRedirectURL string) *ShortenerService {
	return &ShortenerService{
		repo:            repo,
		baseRedirectURL: strings.TrimRight(baseRedirectURL, "/"),
	}
}

// CreateShortURLResult is the outcome of creating a short URL.
type CreateShortURLResult struct {
	ShortCode   string
	ShortURL    string
	OriginalURL string
	CreatedAt   time.Time
	ExpiresAt   *time.Time
}

// CreateShortURL validates the input URL, persists a new short URL record,
// and returns the generated short code along with the full short URL.

// ttlSeconds of 0 means the short URL never expires.
func (s *ShortenerService) CreateShortURL(ctx context.Context, originalURL string, ttlSeconds int64) (*CreateShortURLResult, error) {
	if err := validateURL(originalURL); err != nil {
		return nil, err
	}

	if ttlSeconds < 0 {
		return nil, fmt.Errorf("%w: ttl_seconds must be >= 0", ErrInvalidURL)
	}

	var expiresAt *time.Time
	if ttlSeconds > 0 {
		t := time.Now().Add(time.Duration(ttlSeconds) * time.Second)
		expiresAt = &t
	}

	record, err := s.repo.CreateURL(ctx, originalURL, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("create short url: %w", err)
	}

	return &CreateShortURLResult{
		ShortCode:   record.ShortCode,
		ShortURL:    s.baseRedirectURL + "/" + record.ShortCode,
		OriginalURL: record.OriginalURL,
		CreatedAt:   record.CreatedAt,
		ExpiresAt:   record.ExpiresAt,
	}, nil
}

// URLDetails describes metadata for an existing short URL.
type URLDetails struct {
	OriginalURL string
	CreatedAt   time.Time
	ExpiresAt   *time.Time
	ClickCount  int64
}

// GetURLDetails retrieves metadata for the given short code.
// Returns ErrNotFound if the short code does not exist.
func (s *ShortenerService) GetURLDetails(ctx context.Context, shortCode string) (*URLDetails, error) {
	record, err := s.repo.GetByShortCode(ctx, shortCode)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get url details: %w", err)
	}

	return &URLDetails{
		OriginalURL: record.OriginalURL,
		CreatedAt:   record.CreatedAt,
		ExpiresAt:   record.ExpiresAt,
		ClickCount:  record.ClickCount,
	}, nil
}

// validateURL ensures the given string is an absolute http or https URL.
func validateURL(raw string) error {
	if raw == "" {
		return fmt.Errorf("%w: url must not be empty", ErrInvalidURL)
	}

	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%w: scheme must be http or https", ErrInvalidURL)
	}

	if u.Host == "" {
		return fmt.Errorf("%w: missing host", ErrInvalidURL)
	}

	return nil
}
