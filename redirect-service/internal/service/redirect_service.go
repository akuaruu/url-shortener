package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/akuaruu/url-shortener/redirect-service/internal/domain"
	"github.com/akuaruu/url-shortener/redirect-service/internal/repository"
)

var ErrNotFound = repository.ErrNotFound
var ErrExpired = errors.New("service: url expired")

type RedirectService struct {
	repo *repository.RedirectRepository
	rdb  *redis.Client
}

func NewRedirectService(repo *repository.RedirectRepository, rdb *redis.Client) *RedirectService {
	return &RedirectService{repo: repo, rdb: rdb}
}

type ResolveResult struct {
	URLID       int64
	OriginalURL string
	Expired     bool
}

type ClickMeta struct {
	UserAgent string
	IPHash    string
}

func (s *RedirectService) ResolveShortCode(ctx context.Context, shortCode string, meta ClickMeta) (*ResolveResult, error) {
	if shortCode == "" {
		return nil, fmt.Errorf("short code cannot be empty")
	}

	record, err := s.repo.Resolve(ctx, shortCode)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("resolve error: %w", err)
	}

	if record.IsExpired() {
		return &ResolveResult{
			URLID:       record.ID,
			OriginalURL: record.OriginalURL,
			Expired:     true,
		}, nil
	}

	// Enqueue click event ke Redis. Fire-and-forget tetap dipakai untuk tidak
	// memblokir critical path, tapi sekarang menulis ke queue yang benar.
	go s.enqueueClickEvent(shortCode, record.ID, meta)

	return &ResolveResult{
		URLID:       record.ID,
		OriginalURL: record.OriginalURL,
		Expired:     false,
	}, nil
}

func (s *RedirectService) enqueueClickEvent(shortCode string, urlID int64, meta ClickMeta) {
	event := domain.ClickEvent{
		URLID:     urlID,
		ShortCode: shortCode,
		ClickedAt: time.Now().UTC(),
		UserAgent: meta.UserAgent,
		IPHash:    meta.IPHash,
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return
	}

	// RPush agar urutan FIFO terjaga — worker memakai LPopCount dari kiri.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := s.rdb.RPush(ctx, "queue:click_events", payload).Err(); err != nil {
		// Di production, log ini ke structured logger.
		_ = err
	}
}
