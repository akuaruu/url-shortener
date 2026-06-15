# Documentation

This folder contains supporting artifacts for the url-shortener project: benchmark evidence, database snapshots, and planning documents. All data here was generated from real test runs against the production system.

---

## Structure

```
docs/
├── k6_testing/
│   ├── benchmarks/       # k6 terminal screenshots from all three test runs
│   └── database/         # SQL query results from Supabase post-test
└── PLANNING.md           # Full project planning document
```

---

## k6_testing/benchmarks

Screenshots of k6 terminal output. Each file corresponds to one test scenario described in the root [README benchmark section](../README.md#benchmark-results).

| File | Scenario | VUs | Key Result |
|---|---|---|---|
| `test-1-baseline.png` | Baseline, local | 50 | p95 = 5.21ms, 0% error |
| `test-2-stress.png` | Stress, local | 500 | 3,862 req/s, p95 = 15.07ms, 0% error |
| `test-3-production.png` | Production, via Cloudflare | 100 | p95 = 54.20ms, 0% error |

All three tests hit the redirect endpoint (`GET /:short_code`) with `redirects: 0` to measure the 302 response time directly, not the destination page load time.

---

## k6_testing/database

SQL query results exported from Supabase after all tests completed and the Redis background sync had fully flushed to PostgreSQL.

**File:** [`database/database-overview.md`](k6_testing/database/database-overview.md)

| Query | What it shows |
|---|---|
| Overview keseluruhan sistem | Total URLs, click counts, and data consistency check between `urls.click_count` and actual `click_logs` rows |
| Top 10 URL paling banyak diklik | Distribution of clicks across short codes — confirms randomized test setup worked |
| Bukti load test — lonjakan klik per jam | Per-hour click spikes that map directly to each k6 test run |
| Distribusi klik per URL | Full counter vs log entry comparison confirming 0 data loss across 280,672 events |
| Snapshot post load test | Final state snapshot showing 280,664 clicks across 14 unique URLs |

### Final numbers

| Metric | Value |
|---|---|
| Total URLs created | 40 |
| Total click events recorded | 280,672 |
| Data loss (counter vs log) | 0 |
| Unique URLs hit during tests | 14 |

---

## PLANNING.md

Full planning document written before implementation. Contains:

- Product Requirements Document (PRD)
- Architecture diagram
- Entity Relationship Diagram (ERD)
- API contracts (REST and gRPC)
- Sequence diagrams for shorten and redirect flows
- Caching strategy and TTL decisions
- Benchmark plan and acceptance criteria

The implementation follows this document closely. Any deviations are noted inline in the code.