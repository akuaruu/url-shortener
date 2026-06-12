# High-Performance URL Shortener — Project Planning

> Microservices-based URL shortener built with Go, gRPC, Redis, and PostgreSQL.
> Goal: demonstrate backend architecture, caching strategy, and inter-service communication using gRPC — designed as a portfolio project with measurable performance benchmarks.

---

## 1. PRD (Product Requirements Document)

### 1.1 Problem Statement
URL shortener adalah sistem klasik untuk mendemonstrasikan trade-off antara write-heavy operation (create short URL) dan read-heavy operation (redirect). Redirect harus secepat mungkin karena ini operasi yang paling sering dipanggil dan langsung berdampak pada user experience.

### 1.2 Target User
- Primary: portfolio reviewer / recruiter / technical interviewer
- Secondary: pengguna demo yang mencoba shorten & redirect URL

### 1.3 Core Features (In Scope)
- **Create Short URL** — generate short code dari original URL
- **Redirect** — resolve short code ke original URL dan redirect (HTTP 301/302)
- **Expiry** — short URL punya masa berlaku (TTL), otomatis tidak valid setelah expired
- **Click Tracking (basic)** — hitung jumlah klik per short URL
- **Get URL Details** — melihat metadata short URL (original URL, created_at, click_count, expires_at)

### 1.4 Non-Functional Requirements
| Requirement | Target |
|---|---|
| Redirect latency (cache hit) | < 50ms (p95) |
| Redirect latency (cache miss) | < 150ms (p95) |
| Throughput | Mampu handle minimal 500 req/s pada redirect endpoint (load test) |
| Availability pattern | Cache-aside, sistem tetap berjalan jika Redis down (fallback ke DB) |

### 1.5 Out of Scope
- User authentication / multi-tenant ownership
- Custom domain / custom alias pricing tier
- Rate limiting & abuse prevention (disebutkan sebagai future work)
- Frontend UI lengkap (cukup minimal client/Postman collection untuk demo)
- Geo-distributed deployment / multi-region caching

---

## 2. System Architecture (High-Level)

### 2.1 Components
- **API Gateway** — entry point HTTP, menerima request dari client, meneruskan ke service via gRPC
- **Shortener Service (gRPC)** — menangani pembuatan short URL, generate short code, simpan ke PostgreSQL, populate cache di Redis
- **Redirect Service (gRPC)** — menangani resolusi short code → original URL, cache-aside dengan Redis, fallback ke PostgreSQL jika cache miss
- **PostgreSQL** — source of truth untuk seluruh data URL
- **Redis** — cache layer untuk mapping `short_code → original_url`, juga digunakan untuk increment click counter secara async

### 2.2 Component Diagram (Text-based)

```
                          ┌─────────────┐
                          │   Client    │
                          └──────┬──────┘
                                 │ HTTP
                                 ▼
                       ┌───────────────────┐
                       │    API Gateway     │
                       │   (HTTP → gRPC)    │
                       └─────────┬──────────┘
                                 │
                 ┌───────────────┴────────────────┐
                 │ gRPC                            │ gRPC
                 ▼                                 ▼
       ┌─────────────────────┐         ┌─────────────────────┐
       │  Shortener Service   │         │  Redirect Service    │
       │  (Create / GetDetail)│         │  (Resolve short code)│
       └──────────┬───────────┘         └───────────┬──────────┘
                  │                                  │
        ┌─────────┴─────────┐              ┌────────┴─────────┐
        ▼                   ▼              ▼                  ▼
 ┌─────────────┐     ┌─────────────┐  ┌──────────┐    ┌──────────────┐
 │ PostgreSQL  │◄────┤   Redis     │  │  Redis   │───►│  PostgreSQL  │
 │ (source of  │     │ (cache:     │  │ (cache   │    │ (fallback on │
 │   truth)    │     │ url mapping)│  │  lookup) │    │  cache miss) │
 └─────────────┘     └─────────────┘  └──────────┘    └──────────────┘
```

### 2.3 Why Two Separate Services?
- **Shortener Service**: write-path, lebih jarang dipanggil, bisa toleran terhadap latency lebih tinggi (perlu transaksi DB)
- **Redirect Service**: read-path, paling sering dipanggil, harus seoptimal mungkin — dipisah agar bisa di-scale secara independen (misal: replika Redirect Service lebih banyak dibanding Shortener Service)
- Komunikasi antar service via **gRPC** karena overhead lebih kecil dibanding REST/JSON untuk komunikasi internal antar service (binary protobuf, HTTP/2, strongly typed contract)

---

## 3. ERD (Entity Relationship Diagram)

### 3.1 Tables

**`urls`**
| Column | Type | Constraint |
|---|---|---|
| id | BIGSERIAL | PRIMARY KEY |
| short_code | VARCHAR(10) | UNIQUE, NOT NULL, INDEXED |
| original_url | TEXT | NOT NULL |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT now() |
| expires_at | TIMESTAMPTZ | NULLABLE |
| click_count | BIGINT | NOT NULL, DEFAULT 0 |

**`click_logs`** (optional — basic analytics)
| Column | Type | Constraint |
|---|---|---|
| id | BIGSERIAL | PRIMARY KEY |
| url_id | BIGINT | FOREIGN KEY → urls.id |
| clicked_at | TIMESTAMPTZ | NOT NULL, DEFAULT now() |
| user_agent | TEXT | NULLABLE |
| ip_hash | VARCHAR(64) | NULLABLE (hashed for privacy) |

### 3.2 Relationship
```
urls (1) ──────< (N) click_logs
```
Satu `url` bisa punya banyak `click_logs`. Relasi ini opsional — bisa diimplementasi belakangan setelah core flow (create + redirect) selesai.

---

## 4. API Contract

### 4.1 gRPC Service Definitions (Conceptual — `.proto` outline)

**ShortenerService**
```protobuf
service ShortenerService {
  rpc CreateShortURL(CreateShortURLRequest) returns (CreateShortURLResponse);
  rpc GetURLDetails(GetURLDetailsRequest) returns (GetURLDetailsResponse);
}

message CreateShortURLRequest {
  string original_url = 1;
  int64  ttl_seconds  = 2; // optional, 0 = no expiry
}

message CreateShortURLResponse {
  string short_code   = 1;
  string short_url    = 2;
  string original_url = 3;
  string expires_at   = 4; // ISO8601, empty if no expiry
}

message GetURLDetailsRequest {
  string short_code = 1;
}

message GetURLDetailsResponse {
  string original_url = 1;
  string created_at   = 2;
  string expires_at   = 3;
  int64  click_count  = 4;
}
```

**RedirectService**
```protobuf
service RedirectService {
  rpc ResolveShortCode(ResolveRequest) returns (ResolveResponse);
}

message ResolveRequest {
  string short_code = 1;
}

message ResolveResponse {
  string original_url = 1;
  bool   found        = 2;
  bool   expired      = 3;
}
```

### 4.2 REST API (Client-Facing, via API Gateway)

| Method | Endpoint | Description |
|---|---|---|
| POST | `/api/v1/shorten` | Buat short URL baru |
| GET | `/api/v1/urls/{short_code}` | Ambil detail short URL |
| GET | `/{short_code}` | Redirect ke original URL |

**POST `/api/v1/shorten`**
```json
// Request
{
  "original_url": "https://example.com/very/long/path",
  "ttl_seconds": 86400
}

// Response 201
{
  "short_code": "aZ3xQ1",
  "short_url": "https://short.ly/aZ3xQ1",
  "original_url": "https://example.com/very/long/path",
  "expires_at": "2026-06-13T10:00:00Z"
}
```

**GET `/api/v1/urls/{short_code}`**
```json
// Response 200
{
  "original_url": "https://example.com/very/long/path",
  "created_at": "2026-06-12T10:00:00Z",
  "expires_at": "2026-06-13T10:00:00Z",
  "click_count": 42
}
```

**GET `/{short_code}`**
- Response 302 Found, header `Location: <original_url>`
- Response 404 jika short code tidak ditemukan
- Response 410 Gone jika expired

---

## 5. Sequence Diagrams (Core Flows)

### 5.1 Create Short URL Flow
```
Client          API Gateway        Shortener Svc        PostgreSQL        Redis
  │ POST /shorten    │                   │                   │              │
  │─────────────────►│                   │                   │              │
  │                  │  CreateShortURL   │                   │              │
  │                  │──────────────────►│                   │              │
  │                  │                   │  INSERT url       │              │
  │                  │                   │──────────────────►│              │
  │                  │                   │◄──────────────────│              │
  │                  │                   │  SET short_code → original_url   │
  │                  │                   │──────────────────────────────────►│
  │                  │◄──────────────────│                   │              │
  │◄─────────────────│                   │                   │              │
  │   201 Created    │                   │                   │              │
```

### 5.2 Redirect Flow (Cache-Aside Pattern)

**Cache Hit:**
```
Client          API Gateway        Redirect Svc          Redis
  │ GET /{code}      │                   │                  │
  │─────────────────►│                   │                  │
  │                  │ ResolveShortCode  │                  │
  │                  │──────────────────►│                  │
  │                  │                   │  GET short_code  │
  │                  │                   │─────────────────►│
  │                  │                   │◄─────────────────│
  │                  │                   │   (HIT: original_url)
  │                  │◄──────────────────│                  │
  │◄─────────────────│                   │                  │
  │  302 Redirect    │                   │                  │
```

**Cache Miss:**
```
Client      API Gateway     Redirect Svc        Redis          PostgreSQL
  │ GET /{code}  │                │                 │                │
  │─────────────►│                │                 │                │
  │              │ResolveShortCode│                 │                │
  │              │───────────────►│                 │                │
  │              │                │  GET short_code │                │
  │              │                │────────────────►│                │
  │              │                │◄────────────────│                │
  │              │                │   (MISS)        │                │
  │              │                │  SELECT original_url             │
  │              │                │──────────────────────────────────►│
  │              │                │◄──────────────────────────────────│
  │              │                │  SET short_code → original_url   │
  │              │                │────────────────►│                │
  │              │◄───────────────│                 │                │
  │◄─────────────│                │                 │                │
  │ 302 Redirect │                │                 │                │
```

> Click count increment dilakukan secara **asynchronous** (fire-and-forget ke goroutine / message queue sederhana) agar tidak menambah latency pada critical path redirect.

---

## 6. Caching Strategy

### 6.1 Pattern: Cache-Aside (Lazy Loading)
- Redirect Service selalu cek Redis dulu sebelum query PostgreSQL
- Jika cache miss, query PostgreSQL, lalu populate Redis untuk request berikutnya
- Dipilih karena: sederhana, resilient (sistem tetap berjalan jika Redis down — fallback ke DB), dan cocok untuk access pattern URL shortener (sebagian kecil short URL menerima sebagian besar traffic — hot keys)

### 6.2 Cache Key Design
```
Key pattern : url:{short_code}
Value       : original_url (string)
TTL         : min(remaining_url_ttl, 24h)  // cache TTL tidak boleh melebihi expiry URL asli
```

### 6.3 Cache Invalidation
- Saat short URL expired secara alami → cache entry juga expire otomatis (TTL Redis disinkronkan dengan `expires_at`)
- Tidak ada update/delete short URL di MVP, sehingga invalidation manual tidak diperlukan (future work jika fitur edit ditambahkan)

### 6.4 Click Counter Strategy
- `INCR` di Redis untuk counter real-time (`clicks:{short_code}`)
- Periodic background job (misal setiap 1 menit) melakukan sync counter dari Redis ke kolom `click_count` di PostgreSQL — menghindari write amplification ke DB pada setiap klik

---

## 7. Tech Stack & Infrastructure

| Layer | Choice | Notes |
|---|---|---|
| Language | Go 1.22+ | |
| API Gateway | Go (Fiber/Echo) | HTTP → gRPC translation |
| Inter-service communication | gRPC + Protocol Buffers | |
| Cache | Redis 7 | cache-aside + counter |
| Database | PostgreSQL 16 | source of truth |
| ORM/Query | sqlc atau pgx (raw, performant) | |
| Containerization | Docker + Docker Compose | local orchestration |
| Load Testing | k6 | benchmark redirect endpoint |
| Short Code Generation | Base62 encoding dari auto-increment ID atau random string + collision check | |

---

## 8. Benchmark / Testing Plan

### 8.1 Scenarios
| Scenario | Description |
|---|---|
| Baseline (no cache) | Redirect langsung query PostgreSQL setiap request |
| With Redis cache | Redirect via cache-aside |
| Concurrent load | k6 dengan ramping virtual users (misal 10 → 100 → 500 VUs) ke endpoint redirect |

### 8.2 Metrics to Capture
- Latency p50 / p95 / p99
- Requests per second (throughput)
- Error rate
- Cache hit ratio

### 8.3 Expected Output
- Grafik/tabel perbandingan latency with-cache vs without-cache
- Ringkasan hasil dimasukkan ke README sebagai bukti dampak caching strategy (pengganti metrik "production traffic" yang belum tersedia)

---

## 9. Documentation Outline (README Structure)

1. Project overview & motivation
2. Architecture diagram
3. Tech stack
4. Setup instructions (`docker-compose up`)
5. API documentation (REST + gRPC contract)
6. Database schema (ERD)
7. Caching strategy explanation
8. Benchmark results (with graphs)
9. Future work (rate limiting, auth, multi-region)

---

## 10. Build Order (Recommended Sequence)

1. Setup repo structure (monorepo: `gateway/`, `shortener-service/`, `redirect-service/`, `proto/`)
2. Define `.proto` files & generate Go stubs
3. Setup PostgreSQL schema + migration
4. Implement Shortener Service (Create + GetDetails)
5. Implement Redirect Service (cache-aside logic)
6. Implement API Gateway (HTTP routing → gRPC calls)
7. Docker Compose untuk semua service + Redis + PostgreSQL
8. Load testing dengan k6, capture hasil benchmark
9. Tulis README + dokumentasi final
