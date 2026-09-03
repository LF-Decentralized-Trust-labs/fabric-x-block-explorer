/**
 * k6 sustained load test — 50 VU × 5 min.
 * Run: k6 run --env BASE_URL=http://127.0.0.1:18080 scripts/load-test/load.js
 */
import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  vus: 50,
  duration: '5m',
  thresholds: {
    http_req_failed:                                ['rate<0.01'],  // <1% errors
    http_req_duration:                              ['p(99)<500'],  // p99 < 500 ms
    'http_req_duration{path:/blocks/height}':       ['p(99)<50'],   // height must be fast
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://127.0.0.1:18080';

export default function () {
  const tags = { path: '/blocks/height' };
  const h = http.get(`${BASE_URL}/blocks/height`, { tags });
  check(h, { 'height 200': (r) => r.status === 200 });

  const b = http.get(`${BASE_URL}/blocks?limit=10`);
  check(b, { 'blocks 200': (r) => r.status === 200 });

  sleep(0.5);
}
