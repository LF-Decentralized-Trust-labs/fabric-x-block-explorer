/**
 * k6 smoke test — 5 VU × 30 s quick sanity check.
 * Run: k6 run --env BASE_URL=http://127.0.0.1:18080 scripts/load-test/smoke.js
 */
import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  vus: 5,
  duration: '30s',
  thresholds: {
    http_req_failed:   ['rate<0.01'],  // <1 % errors
    http_req_duration: ['p(99)<1000'], // p99 < 1 s
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://127.0.0.1:18080';

export default function () {
  const r = http.get(`${BASE_URL}/blocks/height`);
  check(r, { 'height 200': (res) => res.status === 200 });

  const rz = http.get(`${BASE_URL}/readyz`);
  check(rz, { 'readyz 200': (res) => res.status === 200 });

  sleep(1);
}
