package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	redirectpb "github.com/akuaruu/url-shortener/proto/gen/redirect"
	shortenerpb "github.com/akuaruu/url-shortener/proto/gen/shortener"
)

// GatewayHandler holds gRPC clients and exposes HTTP handler methods.
// The gateway is the only public-facing component: it translates REST ↔ gRPC.
// No business logic lives here — only transport translation.
type GatewayHandler struct {
	shortener shortenerpb.ShortenerServiceClient
	redirect  redirectpb.RedirectServiceClient
}

// NewGatewayHandler constructs a GatewayHandler.
func NewGatewayHandler(
	shortener shortenerpb.ShortenerServiceClient,
	redirect redirectpb.RedirectServiceClient,
) *GatewayHandler {
	return &GatewayHandler{shortener: shortener, redirect: redirect}
}

// RegisterRoutes wires all HTTP routes to handler methods.
//
// Per PLANNING.md API contract:
//
//	POST /api/v1/shorten           → CreateShortURL
//	GET  /api/v1/urls/:short_code  → GetURLDetails
//	GET  /:short_code              → Redirect (hot path — registered at root)
func (h *GatewayHandler) RegisterRoutes(e *echo.Echo) {
	api := e.Group("/api/v1")
	api.POST("/shorten", h.CreateShortURL)
	api.GET("/urls/:short_code", h.GetURLDetails)

	// Redirect is the highest-traffic endpoint.
	// Registered at root to minimise path-resolution work per request.
	e.GET("/:short_code", h.Redirect)
}

// ── DTOs ─────────────────────────────────────────────────────────────────────
//
// Explicit request/response structs decouple the HTTP contract from proto field
// names and let us control JSON serialisation (snake_case, omitempty) without
// depending on protojson behaviour.

type shortenRequest struct {
	OriginalURL string `json:"original_url"`
	TTLSeconds  int64  `json:"ttl_seconds"`
}

type shortenResponse struct {
	ShortCode   string `json:"short_code"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
	ExpiresAt   string `json:"expires_at,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

type detailsResponse struct {
	OriginalURL string `json:"original_url"`
	CreatedAt   string `json:"created_at"`
	ExpiresAt   string `json:"expires_at,omitempty"`
	ClickCount  int64  `json:"click_count"`
}

type errBody struct {
	Error string `json:"error"`
}

// ── Handlers

// CreateShortURL handles POST /api/v1/shorten.
// Returns 201 Created with the generated short code and full short URL.
func (h *GatewayHandler) CreateShortURL(c echo.Context) error {
	var req shortenRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errBody{Error: "invalid request body"})
	}
	if req.OriginalURL == "" {
		return c.JSON(http.StatusBadRequest, errBody{Error: "original_url is required"})
	}

	res, err := h.shortener.CreateShortURL(c.Request().Context(), &shortenerpb.CreateShortURLRequest{
		OriginalUrl: req.OriginalURL,
		TtlSeconds:  req.TTLSeconds,
	})
	if err != nil {
		return grpcErrToHTTP(c, err)
	}

	return c.JSON(http.StatusCreated, shortenResponse{
		ShortCode:   res.ShortCode,
		ShortURL:    res.ShortUrl,
		OriginalURL: res.OriginalUrl,
		ExpiresAt:   res.ExpiresAt,
		CreatedAt:   res.CreatedAt,
	})
}

// GetURLDetails handles GET /api/v1/urls/:short_code.
// Returns metadata for the given short code: original URL, timestamps, click count.
func (h *GatewayHandler) GetURLDetails(c echo.Context) error {
	shortCode := c.Param("short_code")
	if shortCode == "" {
		return c.JSON(http.StatusBadRequest, errBody{Error: "short_code is required"})
	}

	res, err := h.shortener.GetURLDetails(c.Request().Context(), &shortenerpb.GetURLDetailsRequest{
		ShortCode: shortCode,
	})
	if err != nil {
		return grpcErrToHTTP(c, err)
	}

	return c.JSON(http.StatusOK, detailsResponse{
		OriginalURL: res.OriginalUrl,
		CreatedAt:   res.CreatedAt,
		ExpiresAt:   res.ExpiresAt,
		ClickCount:  res.ClickCount,
	})
}

// Redirect handles GET /:short_code — the hot path.
//
// Resolves the short code via the Redirect Service (cache-aside) and issues a
// 302 Found redirect. Returns 410 Gone for expired URLs, 404 for unknown codes.
//
// 302 (temporary) is used instead of 301 (permanent) intentionally: permanent
// redirects are cached aggressively by browsers, which would break expiry
// semantics and make click analytics unreliable.
func (h *GatewayHandler) Redirect(c echo.Context) error {
	shortCode := c.Param("short_code")
	if shortCode == "" {
		return c.JSON(http.StatusBadRequest, errBody{Error: "short_code is required"})
	}

	res, err := h.redirect.ResolveShortCode(c.Request().Context(), &redirectpb.ResolveRequest{
		ShortCode: shortCode,
		UserAgent: c.Request().UserAgent(), // tambah
		Ip:        c.RealIP(),              // tambah
	})
	if err != nil {
		return grpcErrToHTTP(c, err)
	}

	if res.Expired {
		return c.JSON(http.StatusGone, errBody{Error: "this short URL has expired"})
	}

	return c.Redirect(http.StatusFound, res.OriginalUrl)
}

// ── gRPC → HTTP error translation

// grpcErrToHTTP maps a gRPC status error to the appropriate HTTP response.
// Internal error details are never exposed to the client.
func grpcErrToHTTP(c echo.Context, err error) error {
	st, ok := status.FromError(err)
	if !ok {
		return c.JSON(http.StatusInternalServerError, errBody{Error: "internal server error"})
	}

	switch st.Code() {
	case codes.NotFound:
		return c.JSON(http.StatusNotFound, errBody{Error: st.Message()})
	case codes.InvalidArgument:
		return c.JSON(http.StatusBadRequest, errBody{Error: st.Message()})
	case codes.AlreadyExists:
		return c.JSON(http.StatusConflict, errBody{Error: st.Message()})
	case codes.Unavailable:
		// Downstream gRPC service is unreachable.
		return c.JSON(http.StatusServiceUnavailable, errBody{Error: "service temporarily unavailable"})
	default:
		return c.JSON(http.StatusInternalServerError, errBody{Error: "internal server error"})
	}
}
