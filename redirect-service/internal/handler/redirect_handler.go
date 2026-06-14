package handler

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	redirectpb "github.com/akuaruu/url-shortener/proto/gen/redirect"
	"github.com/akuaruu/url-shortener/redirect-service/internal/service"
)

// RedirectHandler implements the gRPC RedirectServiceServer interface.
// It is responsible solely for transport translation: proto ↔ domain types.
// All business logic lives in service.RedirectService.
type RedirectHandler struct {
	redirectpb.UnimplementedRedirectServiceServer
	svc *service.RedirectService
}

// NewRedirectHandler constructs a RedirectHandler.
func NewRedirectHandler(svc *service.RedirectService) *RedirectHandler {
	return &RedirectHandler{svc: svc}
}

// ResolveShortCode implements RedirectServiceServer.
func (h *RedirectHandler) ResolveShortCode(ctx context.Context, req *redirectpb.ResolveRequest) (*redirectpb.ResolveResponse, error) {
	if req.GetShortCode() == "" {
		return nil, status.Error(codes.InvalidArgument, "short_code is required")
	}

	meta := service.ClickMeta{
		UserAgent: req.GetUserAgent(),
		IPHash:    hashIP(req.GetIp()),
	}

	result, err := h.svc.ResolveShortCode(ctx, req.GetShortCode(), meta)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "short code not found")
		}
		// Any other error is an internal failure — don't leak internal details.
		return nil, status.Errorf(codes.Internal, "failed to resolve short code")
	}

	// Expired URLs: the service returns a result (not an error) with Expired = true.
	// The gateway translates this into HTTP 410 Gone. Here we propagate the flag.
	return &redirectpb.ResolveResponse{
		OriginalUrl: result.OriginalURL,
		Found:       true,
		Expired:     result.Expired,
	}, nil
}

func hashIP(ip string) string {
	if ip == "" {
		return ""
	}
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(ip)))[:16]
}
