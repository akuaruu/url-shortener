package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/akuaruu/url-shortener/gateway/internal/client"
	"github.com/akuaruu/url-shortener/gateway/internal/handler"
)

// config holds all runtime configuration loaded from environment variables.
// Environment variables (all required unless a default is shown):
//
//	HTTP_PORT       — gateway listen port (default: 8080)
//	SHORTENER_ADDR  — Shortener Service gRPC address (e.g. "shortener:50051")
//	REDIRECT_ADDR   — Redirect Service gRPC address  (e.g. "redirect:50052")

// config holds the environment variables required by the API Gateway.
type config struct {
	HTTPPort      string
	ShortenerAddr string
	RedirectAddr  string
}

// loadConfig reads environment variables or applies fallback defaults.
func loadConfig() config {
	return config{
		HTTPPort:      getEnv("HTTP_PORT", "8080"),
		ShortenerAddr: getEnv("SHORTENER_ADDR", "localhost:50051"),
		RedirectAddr:  getEnv("REDIRECT_ADDR", "localhost:50052"),
	}
}

// getEnv is a helper to read an environment variable or return a default value.
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := loadConfig()

	// ─── gRPC clients
	// grpc.NewClient does not block — the actual TCP connection is established
	// lazily on the first RPC call. No ping needed here.

	shortenerClient, shortenerConn, err := client.NewShortenerClient(cfg.ShortenerAddr)
	if err != nil {
		log.Fatalf("failed to create shortener client: %v", err)
	}
	defer shortenerConn.Close()

	redirectClient, redirectConn, err := client.NewRedirectClient(cfg.RedirectAddr)
	if err != nil {
		log.Fatalf("failed to create redirect client: %v", err)
	}
	defer redirectConn.Close()

	// ─── Echo setup
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Middleware stack — order matters.
	//
	// CORS must be first: browser preflight (OPTIONS) must receive CORS headers
	// before any other middleware short-circuits the request.
	// RequestID next so every log line carries the ID.
	// Logger after RequestID so request IDs appear in access logs.
	// Recover nearest to handler so panics don't bypass logging.
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodOptions, // required: browser always sends OPTIONS preflight first
		},
		AllowHeaders: []string{
			echo.HeaderOrigin,
			echo.HeaderContentType, // required: fetch sends Content-Type: application/json
			echo.HeaderAccept,
		},
		MaxAge: 86400, // cache preflight result for 24h — reduces round-trips
	}))
	e.Use(middleware.RequestID())
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: `{"time":"${time_rfc3339}","id":"${id}","method":"${method}","uri":"${uri}","status":${status},"latency_ms":${latency}}` + "\n",
	}))
	e.Use(middleware.Recover())

	// Routes
	h := handler.NewGatewayHandler(shortenerClient, redirectClient)
	h.RegisterRoutes(e)

	// ─── Start + graceful shutdown
	go func() {
		addr := ":" + cfg.HTTPPort
		log.Printf("gateway: HTTP server listening on %s", addr)
		if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
			log.Fatalf("gateway: server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("gateway: received shutdown signal")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.Shutdown(shutdownCtx); err != nil {
		log.Printf("gateway: shutdown error: %v", err)
	}

	log.Println("gateway: stopped cleanly")
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required environment variable %q is not set", key)
	}
	return v
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
