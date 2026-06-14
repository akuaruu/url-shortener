# url-shortener

High-performance URL shortener built as a microservices monorepo: Go, gRPC, Redis (cache-aside), and PostgreSQL. Built as a portfolio project to demonstrate backend architecture, inter-service communication, and caching strategy with measurable benchmarks.

See [`/docs` planning document] for full PRD, architecture diagrams, sequence diagrams, caching strategy, and benchmark plan referenced during design.

## Repository structure

```
url-shortener/
├── proto/                    # gRPC service definitions (source of truth for contracts)
│   ├── shortener.proto
│   ├── redirect.proto
│   └── gen/                   # generated Go stubs (via `make proto`)
├── shortener-service/         # write-path: create short URLs, fetch details
│   ├── cmd/                   # main.go entrypoint
│   └── internal/
│       ├── handler/           # gRPC handlers
│       ├── service/           # business logic (short code generation, validation)
│       └── repository/        # PostgreSQL + Redis access
├── redirect-service/           # read-path: resolve short code -> original URL
│   ├── cmd/
│   └── internal/
│       ├── handler/            # gRPC handlers
│       ├── service/             # cache-aside resolution logic
│       └── repository/          # PostgreSQL + Redis access
├── gateway/                     # HTTP entrypoint, translates REST <-> gRPC
│   ├── cmd/
│   └── internal/
│       ├── handler/             # HTTP route handlers
│       └── client/              # gRPC clients for shortener/redirect services
├── pkg/
│   └── shortcode/                # shared Base62 encoding logic
├── migrations/                   # SQL migrations (golang-migrate format)
├── Makefile                       # proto generation & migration commands
└── go.mod
```

## Why this structure

- **proto/ as contract source of truth** — both services and the gateway depend on generated stubs from `proto/gen`, ensuring type-safe communication and a single point of change for API contracts.
- **Service separation (Shortener vs Redirect)** — write path and read path scale independently. Redirect Service is the hot path and can be horizontally scaled without affecting Shortener Service.
- **Gateway as the only public-facing component** — internal services communicate over gRPC and are not directly exposed; the gateway translates public REST requests into internal gRPC calls.
- **pkg/shortcode shared module** — Base62 encoding logic used by Shortener Service is isolated as a reusable package, avoiding duplication and making it independently testable.

## Generating gRPC code from proto

Requires `protoc`, `protoc-gen-go`, and `protoc-gen-go-grpc` on `$PATH`:

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

make proto
```

This generates `*.pb.go` and `*_grpc.pb.go` files into `proto/gen/`.

> Note: code generation could not be run in this environment because the Go module proxy (`proxy.golang.org`) is not reachable from the sandbox network. Run `make proto` locally after cloning.

## Database setup

Requires [`golang-migrate`](https://github.com/golang-migrate/migrate):

```bash
export DATABASE_URL="postgres://user:password@localhost:5432/urlshortener?sslmode=disable"
make migrate-up
```

Migrations:
- `0001_create_urls_table` — core `urls` table (short_code, original_url, expiry, click_count)
- `0002_create_click_logs_table` — optional analytics table for per-click events

## Short code generation strategy

Base62 encoding (`[0-9a-zA-Z]`) applied to the auto-incrementing `urls.id` from PostgreSQL. This avoids collision-detection overhead while keeping short codes compact and unambiguous. Implemented in `pkg/shortcode` as a shared package used by Shortener Service.

## Status

This repository currently contains the planning artifacts and project skeleton:
- [x] PRD, architecture diagram, ERD, API contracts, sequence diagrams, caching strategy (see planning doc)
- [x] Monorepo structure
- [x] `.proto` definitions for ShortenerService and RedirectService
- [x] Database migrations (ERD implementation)
- [x] gRPC stub generation (run `make proto` locally)
- [x] Service implementations (Shortener, Redirect, Gateway)
- [x] Docker Compose setup
- [x] Load testing & benchmark results
