package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/akuaruu/url-shortener/redirect-service/internal/repository"
)

// ErrNotFound is returned when the short code cannot be resolved in cache or DB.
var ErrNotFound = repository.ErrNotFound

// ErrExpired is returned when the URL exists but its TTL has passed.
var ErrExpired = errors.New("service: url expired")

// RedirectService implements the high-performance resolution logic.
type RedirectService struct {
	repo *repository.RedirectRepository
}

// NewRedirectService constructs a RedirectService.
func NewRedirectService(repo *repository.RedirectRepository) *RedirectService {
	return &RedirectService{repo: repo}
}

// ResolveResult holds the resolution outcome.
type ResolveResult struct {
	OriginalURL string
	Expired     bool
}

// ResolveShortCode resolves a short code to its original URL.
// It relies on the repository to implement the cache-aside logic.
func (s *RedirectService) ResolveShortCode(ctx context.Context, shortCode string) (*ResolveResult, error) {
	if shortCode == "" {
		return nil, fmt.Errorf("short code cannot be empty")
	}

	// 1. Resolve URL from Repository (Cache-aside is handled inside repo)
	record, err := s.repo.Resolve(ctx, shortCode)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("resolve error: %w", err)
	}

	// 2. Check Expiration
	if record.IsExpired() {
		return &ResolveResult{
			OriginalURL: record.OriginalURL,
			Expired:     true,
		}, nil
	}

	// 3. Asynchronously increment click count
	// Fire-and-forget: we do not block the critical read path to update analytics.
	go func(code string) {
		// Gunakan context terpisah karena context request utama mungkin segera dibatalkan (canceled) setelah response dikirim.
		bgCtx := context.Background()
		if err := s.repo.IncrementClickCount(bgCtx, code); err != nil {
			// Log error via telemetry/logger in production
			_ = err
		}
	}(shortCode)

	return &ResolveResult{
		OriginalURL: record.OriginalURL,
		Expired:     false,
	}, nil
}
