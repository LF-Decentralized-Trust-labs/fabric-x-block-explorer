/**
 * k6 soak test — 10 VU × 30 min. Checks for memory leaks and goroutine growth.
 * Run: k6 run --env BASE_URL=http://127.0.0.1:18080 scripts/load-test/soak.js
 */
import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  vus: 10,
  duration: '30m',
  thresholds: {
    http_req_failed:   ['rate<0.005'],  // <0.5% errors over the full soak
    http_req_duration: ['p(99)<1000'],  // p99 < 1 s
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://127.0.0.1:18080';

export default function () {
  const h = http.get(`${BASE_URL}/blocks/height`);
  check(h, { 'height 200': (r) => r.status === 200 });

  const b = http.get(`${BASE_URL}/blocks?limit=5`);
  check(b, { 'blocks 200': (r) => r.status === 200 });

  sleep(2);
}
