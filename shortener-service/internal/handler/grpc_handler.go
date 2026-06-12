package handler

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	shortenerpb "github.com/akuaruu/url-shortener/proto/gen/shortener"
	"github.com/akuaruu/url-shortener/shortener-service/internal/service"
)

type ShortenerHandler struct {
	shortenerpb.UnimplementedShortenerServiceServer
	svc *service.ShortenerService
}

func NewShortenerHandler(svc *service.ShortenerService) *ShortenerHandler {
	return &ShortenerHandler{svc: svc}
}

// CreateShortURL handles the RPC request to generate a new short code.
func (h *ShortenerHandler) CreateShortURL(ctx context.Context, req *shortenerpb.CreateShortURLRequest) (*shortenerpb.CreateShortURLResponse, error) {
	// Panggil layer service yang sudah memiliki validasi internal
	res, err := h.svc.CreateShortURL(ctx, req.OriginalUrl, req.TtlSeconds)
	if err != nil {
		if errors.Is(err, service.ErrInvalidURL) {
			return nil, status.Errorf(codes.InvalidArgument, "invalid input: %v", err)
		}
		// Log internal error
		return nil, status.Errorf(codes.Internal, "failed to create short url")
	}

	var expiresAt string
	if res.ExpiresAt != nil {
		expiresAt = res.ExpiresAt.Format(time.RFC3339)
	}

	return &shortenerpb.CreateShortURLResponse{
		ShortCode:   res.ShortCode,
		ShortUrl:    res.ShortURL,
		OriginalUrl: res.OriginalURL,
		ExpiresAt:   expiresAt,
		CreatedAt:   res.CreatedAt.Format(time.RFC3339),
	}, nil
}

// GetURLDetails handles the RPC request to retrieve metadata for a short code.
func (h *ShortenerHandler) GetURLDetails(ctx context.Context, req *shortenerpb.GetURLDetailsRequest) (*shortenerpb.GetURLDetailsResponse, error) {
	if req.ShortCode == "" {
		return nil, status.Error(codes.InvalidArgument, "short code is required")
	}

	res, err := h.svc.GetURLDetails(ctx, req.ShortCode)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "short code '%s' not found", req.ShortCode)
		}
		return nil, status.Errorf(codes.Internal, "failed to retrieve url details")
	}

	var expiresAt string
	if res.ExpiresAt != nil {
		expiresAt = res.ExpiresAt.Format(time.RFC3339)
	}

	return &shortenerpb.GetURLDetailsResponse{
		OriginalUrl: res.OriginalURL,
		CreatedAt:   res.CreatedAt.Format(time.RFC3339),
		ExpiresAt:   expiresAt,
		ClickCount:  res.ClickCount,
	}, nil
}
