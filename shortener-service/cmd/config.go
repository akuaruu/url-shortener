package main

import "os"

// config holds runtime configuration loaded from environment variables,
// following 12-factor app conventions.
type config struct {
	// DatabaseURL is the PostgreSQL connection string,
	// e.g. "postgres://user:pass@localhost:5432/urlshortener?sslmode=disable".
	DatabaseURL string

	// RedisAddr is the Redis server address, e.g. "localhost:6379".
	RedisAddr string

	// GRPCPort is the port this service's gRPC server listens on.
	GRPCPort string

	// BaseRedirectURL is prefixed to short codes to build the full short URL,
	// e.g. "https://short.ly" -> "https://short.ly/aZ3xQ1".
	BaseRedirectURL string
}

func loadConfig() config {
	return config{
		DatabaseURL:     getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/urlshortener?sslmode=disable"),
		RedisAddr:       getEnv("REDIS_ADDR", "localhost:6379"),
		GRPCPort:        getEnv("SHORTENER_GRPC_PORT", "50051"),
		BaseRedirectURL: getEnv("BASE_REDIRECT_URL", "http://localhost:8080"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
