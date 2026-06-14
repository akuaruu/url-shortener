package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	redirectpb "github.com/akuaruu/url-shortener/proto/gen/redirect"
	"github.com/akuaruu/url-shortener/redirect-service/internal/handler"
	"github.com/akuaruu/url-shortener/redirect-service/internal/repository"
	"github.com/akuaruu/url-shortener/redirect-service/internal/service"
	"github.com/akuaruu/url-shortener/redirect-service/internal/worker"
)

func main() {
	// Config loaded from environment (12-factor app).
	// DATABASE_URL: "postgres://user:pass@host:5432/dbname?sslmode=disable"
	// REDIS_ADDR:   "localhost:6379"
	// GRPC_PORT:    ":50052"
	dbURL := mustEnv("DATABASE_URL")
	redisAddr := mustEnv("REDIS_ADDR")
	grpcPort := envOr("GRPC_PORT", "50052")

	ctx := context.Background()

	// ─── PostgreSQL (supabase)
	db, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("failed to create postgres pool: %v", err)
	}
	defer db.Close()

	if err := db.Ping(ctx); err != nil {
		log.Fatalf("postgres ping failed: %v", err)
	}
	log.Println("connected to postgres")

	// ─── Redis
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	defer rdb.Close()

	if err := rdb.Ping(ctx).Err(); err != nil {
		// Non-fatal: Redis is cache-only. The system falls back to Postgres on miss.
		// Log a warning so the operator knows the cache is unavailable.
		log.Printf("warning: redis ping failed (%v) — continuing without cache", err)
	} else {
		log.Println("connected to redis")
	}

	// ─── Wire dependencies
	repo := repository.NewRedirectRepository(db, rdb)
	svc := service.NewRedirectService(repo, rdb)
	h := handler.NewRedirectHandler(svc)

	// ─── gRPC server
	grpcServer := grpc.NewServer()
	redirectpb.RegisterRedirectServiceServer(grpcServer, h)

	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()
	go worker.StartClickSyncWorker(workerCtx, db, rdb, 30*time.Second)

	// reflection enables grpcurl-based introspection in dev/staging.
	// Remove in production if not needed.
	reflection.Register(grpcServer)

	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", grpcPort, err)
	}
	log.Printf("redirect-service listening on %s", grpcPort)

	// Run server in a goroutine so the main goroutine can block on signals.
	errCh := make(chan error, 1)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			errCh <- err
		}
	}()

	// ─── Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-quit:
		log.Printf("received signal %s — shutting down", sig)
	case err := <-errCh:
		log.Printf("server error: %v", err)
	}

	// GracefulStop stops accepting new RPCs and waits for in-flight ones to finish.
	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()

	const shutdownTimeout = 10 * time.Second
	select {
	case <-stopped:
		log.Println("server stopped cleanly")
	case <-time.After(shutdownTimeout):
		log.Println("shutdown timeout exceeded — forcing stop")
		grpcServer.Stop()
	}
}

// mustEnv returns the value of the named environment variable or exits.
func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required environment variable %q is not set", key)
	}
	return v
}

// envOr returns the value of the named environment variable, or fallback if unset.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
