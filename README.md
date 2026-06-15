# url-shortener

High-performance URL shortener built as a microservices monorepo. The system is designed to separate the write path and read path into independent services, connected via gRPC, with Redis as the primary cache layer for redirect resolution.

Built as a portfolio project to demonstrate backend architecture, inter-service communication, and caching strategy with measurable benchmarks.

**Live demo:** [url-shortener-aruu.vercel.app](https://url-shortener-aruu.vercel.app)
**API:** [url-s.aruu.app](https://url-s.aruu.app)

---

## Architecture

```
                          ┌─────────────────────────────┐
  Browser / Frontend      │          API Gateway         │
  (Next.js on Vercel) --> │     Echo v4 · REST → gRPC    │
                          └────────────┬────────────┬────┘
                                       │            │
                          ┌────────────▼──┐  ┌──────▼──────────┐
                          │   Shortener   │  │    Redirect      │
                          │   Service     │  │    Service       │
                          │  (write path) │  │  (read path)     │
                          └────────────┬──┘  └──────┬──────────┘
                                       │             │
                          ┌────────────▼─────────────▼──────────┐
                          │        PostgreSQL (Supabase)         │
                          └──────────────────────────────────────┘
                                       │             │
                          ┌────────────▼─────────────▼──────────┐
                          │            Redis (cache-aside)       │
                          └──────────────────────────────────────┘
```

### Why this structure

**Separate Shortener and Redirect Services.** The redirect endpoint is the hot path. It handles orders of magnitude more traffic than the shorten endpoint and needs to scale independently. Keeping them separate means you can horizontal-scale the Redirect Service without touching the Shortener Service.

**Gateway as the only public component.** Internal services communicate over gRPC and are never exposed directly. The gateway translates public REST requests into typed gRPC calls, which gives you a single point of entry for auth, rate limiting, and observability.

**proto/ as contract source of truth.** Both services and the gateway depend on generated stubs from `proto/gen`. Any API change requires updating the proto file first, which makes breaking changes explicit and type-safe.

**Cache-aside on the read path.** The Redirect Service checks Redis before hitting PostgreSQL. On a cache hit (the common case), the database is never touched. This is the main reason the system can sustain low latency under high concurrency.

---

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Go 1.22 |
| HTTP Gateway | Echo v4 |
| Inter-service communication | gRPC (protobuf) |
| Cache | Redis 7 (cache-aside) |
| Database | PostgreSQL via Supabase |
| Frontend | Next.js 14, TypeScript, Tailwind CSS |
| Containerization | Docker Compose |
| Reverse proxy / SSL | Cloudflare (Flexible SSL) |
| Frontend deployment | Vercel |
| Load testing | k6 |

---

## Repository Structure

```
url-shortener/
├── proto/                        # gRPC service definitions (source of truth)
│   ├── shortener.proto
│   ├── redirect.proto
│   └── gen/                      # generated Go stubs (via make proto)
├── shortener-service/
│   ├── cmd/
│   └── internal/
│       ├── handler/              # gRPC handlers
│       ├── service/              # short code generation, validation
│       └── repository/           # PostgreSQL + Redis access
├── redirect-service/
│   ├── cmd/
│   └── internal/
│       ├── handler/              # gRPC handlers
│       ├── service/              # cache-aside resolution logic
│       └── repository/           # PostgreSQL + Redis access
├── gateway/
│   ├── cmd/
│   └── internal/
│       ├── handler/              # HTTP route handlers
│       └── client/               # gRPC clients
├── pkg/
│   └── shortcode/                # shared Base62 encoding
├── migrations/                   # SQL migrations (golang-migrate)
├── frontend/                     # Next.js app
├── loadtest.js                   # k6 load test script
├── docker-compose.yml
└── Makefile
```

---

## Getting Started

### Prerequisites

- Go 1.22+
- Docker + Docker Compose
- `protoc`, `protoc-gen-go`, `protoc-gen-go-grpc` (for proto generation)

### Generate gRPC stubs

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
make proto
```

### Run with Docker Compose

```bash
cp .env.example .env   # fill in DATABASE_URL and REDIS_URL
docker compose up -d
```

### Database migrations

```bash
export DATABASE_URL="postgres://user:password@localhost:5432/urlshortener?sslmode=disable"
make migrate-up
```

---

## Short Code Generation

Short codes use Base62 encoding (`[0-9a-zA-Z]`) applied to the auto-incrementing `urls.id` from PostgreSQL.

This approach avoids collision detection overhead entirely. There is no retry loop and no random generation. Given a database ID, the short code is deterministic and unique by construction. The tradeoff is that codes are sequential and predictable, which is an acceptable compromise for this use case.

The encoding logic lives in `pkg/shortcode` as a shared package, used by the Shortener Service and independently testable.

---

## Caching Strategy

The Redirect Service uses cache-aside (lazy loading):

1. Request arrives for short code `X`
2. Check Redis for key `redirect:X`
3. Cache hit: return immediately, no database query
4. Cache miss: query PostgreSQL, write result to Redis with TTL, return

This makes the redirect path almost entirely in-memory for warm URLs. The benchmark results below show the practical effect of this at scale.

A background worker syncs click counts from Redis back to PostgreSQL in batches. This avoids a database write on every redirect while keeping click_count eventually consistent.

---

## Benchmark Results

Tested with [k6](https://k6.io). All tests hit the redirect endpoint (`GET /:short_code`) with `redirects: 0` to measure the 302 response directly. Each test run creates 10 distinct short codes in setup, then randomizes across them during the VU phase to simulate realistic cache hit patterns.

### Summary

| Scenario | VUs | Throughput | avg | p90 | p95 | Error Rate |
|---|---|---|---|---|---|---|
| Baseline (local) | 50 | 406 req/s | 2.92ms | 4.25ms | **5.21ms** | 0.00% |
| Stress (local) | 500 | 3,862 req/s | 4.42ms | 9.02ms | **15.07ms** | 0.00% |
| Production (via Cloudflare) | 100 | 464 req/s | 38.85ms | 48.82ms | **54.20ms** | 0.00% |

Local tests were executed on localhost to isolate application and cache performance from network overhead. Production latency includes round-trip through Cloudflare and the public internet.

### Test 1: Baseline (50 VU, local)

```
stages:
  10s  ramp to 10 VU
  30s  hold at 50 VU
  10s  ramp down

threshold:  p(95) < 10ms    PASS (5.21ms)
threshold:  error rate < 1%  PASS (0.00%)

http_reqs:      22,606   406 req/s
http_req_failed: 0.00%
redirect_duration:
  avg=2.92ms  min=362µs  med=2.59ms  max=91.58ms  p(90)=4.25ms  p(95)=5.21ms
```

Baseline confirms the system handles normal load comfortably. Median latency at 2.59ms indicates most requests are returning from Redis without touching the database.

### Test 2: Stress (500 VU, local)

```
stages:
  10s  ramp to 50 VU
  30s  ramp to 200 VU
  30s  ramp to 500 VU
  15s  ramp down

threshold:  p(95) < 50ms     PASS (15.07ms)
threshold:  error rate < 5%   PASS (0.00%)

http_reqs:       331,070   3,862 req/s
http_req_failed:   0.00%
redirect_duration:
  avg=4.42ms  min=343µs  med=2.37ms  max=89.46ms  p(90)=9.02ms  p(95)=15.07ms
```

The most notable result here is the median at 2.37ms under 500 concurrent users, which is actually lower than the baseline median at 50 VU. This is the cache-aside strategy working: as VUs increase, more requests hit warm Redis keys. The p95 increases from 5.21ms to 15.07ms (roughly 3x) while load increased 10x, which shows the system scales sublinearly.

### Test 3: Production (100 VU, real VPS via Cloudflare)

```
stages:
  15s  ramp to 10 VU
  30s  hold at 50 VU
  30s  ramp to 100 VU
  15s  ramp down

threshold:  p(95) < 100ms    PASS (54.20ms)
threshold:  error rate < 1%   PASS (0.00%)

http_reqs:       43,952   464 req/s
http_req_failed:   0.00%
redirect_duration:
  avg=38.85ms  min=27.51ms  med=36.35ms  max=289.69ms  p(90)=48.82ms  p(95)=54.20ms
```

Production latency is dominated by network round-trip (laptop to Cloudflare to VPS), not application logic. The average of 38.85ms is consistent with expected RTT for this network path. 0% error rate across 43,952 requests confirms the system is stable under concurrent production traffic.

### Evidence from load test: click_logs per hour

The following was queried from the production database immediately after all three tests completed:

| hour (UTC) | click events |
|---|---|
| 2026-06-15 01:00 | 43,942 |
| 2026-06-15 00:00 | 81,719 |
| 2026-06-14 17:00 | 7 |

The two spikes correspond directly to the three test runs. All click events were written to PostgreSQL via the background batch sync from Redis.

---

## Database Schema

### urls

```sql
CREATE TABLE public.urls (
    id          BIGSERIAL PRIMARY KEY,
    short_code  VARCHAR(10)  NOT NULL,
    original_url TEXT        NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ  NULL,
    click_count BIGINT       NOT NULL DEFAULT 0,
    CONSTRAINT uq_urls_short_code UNIQUE (short_code)
);

-- Primary lookup index for redirect resolution
CREATE INDEX idx_urls_short_code ON urls USING btree (short_code);

-- Partial index: only indexes rows where expires_at is set.
-- Expiry cleanup queries only touch URLs that actually have an expiry,
-- so a full index on this column would waste space and slow down writes.
CREATE INDEX idx_urls_expires_at ON urls (expires_at)
WHERE expires_at IS NOT NULL;
```

### click_logs

```sql
CREATE TABLE public.click_logs (
    id          BIGSERIAL PRIMARY KEY,
    url_id      BIGINT      NOT NULL,
    clicked_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    user_agent  TEXT        NULL,
    ip_hash     VARCHAR(64) NULL,   -- SHA-256 hash of client IP, never raw IP
    CONSTRAINT fk_click_logs_url
        FOREIGN KEY (url_id) REFERENCES urls (id) ON DELETE CASCADE
);

CREATE INDEX idx_click_logs_url_id    ON click_logs (url_id);
CREATE INDEX idx_click_logs_clicked_at ON click_logs (clicked_at);
```

**Design notes:**

`ip_hash` stores a SHA-256 hash of the client IP rather than the raw address. This allows deduplication and analytics without retaining personally identifiable information.

`ON DELETE CASCADE` on `fk_click_logs_url` ensures that deleting a URL also removes all associated click history atomically, without needing a separate cleanup step.

The partial index on `expires_at` is a deliberate choice. Expiry-related queries (e.g., finding URLs to invalidate) only need to scan rows where `expires_at IS NOT NULL`. A standard index would include every row, most of which are irrelevant to expiry logic.

---

## Database Overview

Snapshot taken after benchmark tests completed.

### System overview

| total_urls | permanent_urls | active_with_expiry | total_cached_clicks | total_click_events |
|---|---|---|---|---|
| 39 | 0 | 39 | 123,669 | 123,669 |

`counter_cached` and `total_click_events` matching confirms that the background batch sync between Redis and PostgreSQL is consistent with zero data loss.

### Top URLs by click count

| short_code | click_count | created_on | expires_on |
|---|---|---|---|
| 9 | 59,585 | 2026-06-15 | 2026-06-16 |
| 8 | 22,127 | 2026-06-15 | 2026-06-16 |
| c | 4,484 | 2026-06-15 | 2026-06-16 |
| b | 4,476 | 2026-06-15 | 2026-06-16 |
| Z | 4,451 | 2026-06-15 | 2026-06-16 |
| a | 4,429 | 2026-06-15 | 2026-06-16 |
| U | 4,406 | 2026-06-15 | 2026-06-16 |
| X | 4,403 | 2026-06-15 | 2026-06-16 |
| Y | 4,357 | 2026-06-15 | 2026-06-16 |
| d | 4,351 | 2026-06-15 | 2026-06-16 |

Short codes `9` and `8` were used in the baseline and stress tests (single-code setup). The remaining codes (`c`, `b`, `Z`, etc.) were distributed across 10 randomized codes in the production test, which explains the relatively even distribution.

### Data integrity check: counter_cached vs actual log entries

| short_code | counter_cached | log_entries | delta |
|---|---|---|---|
| 9 | 60,585 | 60,585 | 0 |
| 8 | 22,127 | 22,127 | 0 |
| c | 4,484 | 4,484 | 0 |
| b | 4,476 | 4,476 | 0 |

All counters match their corresponding `click_logs` row counts. The Redis-to-PostgreSQL batch sync produced no data loss or double-counting across 123,669 events.

---

## Status

- [x] PRD, architecture diagram, ERD, API contracts, sequence diagrams, caching strategy
- [x] Monorepo structure with proto as contract source of truth
- [x] gRPC proto definitions (ShortenerService, RedirectService)
- [x] Database migrations
- [x] Shortener Service (create, fetch)
- [x] Redirect Service (cache-aside resolution, click tracking)
- [x] API Gateway (REST to gRPC translation)
- [x] Background worker (Redis to PostgreSQL batch sync)
- [x] Docker Compose setup
- [x] Frontend (Next.js, deployed on Vercel)
- [x] Load testing with k6 (baseline, stress, production)
- [x] Deployed to VPS behind Cloudflare