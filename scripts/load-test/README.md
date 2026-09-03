# Load Test Scripts

Three k6 scripts covering different test scenarios.

## Prerequisites

Install [k6](https://k6.io/docs/getting-started/installation/) and start a live stack first:

```bash
make swagger   # starts stack on http://127.0.0.1:18080, keeps running
```

## Scripts

| Script | VUs | Duration | Purpose |
|---|---|---|---|
| `smoke.js` | 5 | 30 s | Quick sanity check after every deploy |
| `load.js` | 50 | 5 min | Sustained throughput — verifies p99 < 500 ms |
| `soak.js` | 10 | 30 min | Long-running stability — catches memory leaks |

## Running

```bash
# Smoke (default port 18080)
make load-test-smoke

# Sustained load
make load-test-load

# Soak (manual — not in CI)
k6 run --env BASE_URL=http://127.0.0.1:18080 scripts/load-test/soak.js
```

## Thresholds

| Metric | Threshold |
|---|---|
| `http_req_failed` | < 1% errors |
| `http_req_duration` p99 | < 500 ms |
| `/blocks/height` p99 | < 50 ms |

A failed threshold causes k6 to exit with a non-zero code, failing the CI step.
