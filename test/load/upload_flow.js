// Upload flow load test (Phase 10, §6): presign -> PUT to object storage ->
// complete. The photo bytes go directly to R2/MinIO; the API only issues URLs.
//
//   k6 run -e BASE_URL=http://localhost:18080 test/load/upload_flow.js
//
// Rate limits (see doc/phases/Phase 10 - Hardening and Deploy.md §2) cap
// upload init at 100/min/user. Raise the limits for a full 100+ VU run, or
// expect RATE_LIMITED responses beyond the burst.
import http from 'k6/http';
import { check, fail, sleep } from 'k6';
import { randomString } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';
import encoding from 'k6/encoding';

export const options = {
  scenarios: {
    uploads: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: 25 },
        { duration: '1m', target: 100 },
        { duration: '30s', target: 0 },
      ],
      gracefulRampDown: '10s',
    },
  },
  thresholds: {
    checks: ['rate>0.99'],
    'http_req_failed{step:complete}': ['rate<0.01'],
    'http_req_duration{step:init}': ['p(95)<100'],
    'http_req_duration{step:complete}': ['p(95)<100'],
  },
};

const BASE = (__ENV.BASE_URL || 'http://localhost:18080').replace(/\/+$/, '');
const PASSWORD = __ENV.PASSWORD || 'load-test-password-123';

// 1x1 JPEG.
const JPEG = encoding.b64decode(
  '/9j/4AAQSkZJRgABAQEAYABgAAD/2wBDAP//////////////////////////////////////////////////////////////////////////////////////2wBDAf//////////////////////////////////////////////////////////////////////////////////////wAARCAABAAEDASIAAhEBAxEB/8QAFQABAQAAAAAAAAAAAAAAAAAAAAX/xAAUEAEAAAAAAAAAAAAAAAAAAAAA/9oADAMBAAIQAxAAAAH/xAAUEAEAAAAAAAAAAAAAAAAAAAAA/9oACAEBAAEFAqf/xAAUEQEAAAAAAAAAAAAAAAAAAAAA/9oACAEDAQE/Aaf/xAAUEQEAAAAAAAAAAAAAAAAAAAAA/9oACAECAQE/Aaf/xAAUEAEAAAAAAAAAAAAAAAAAAAAA/9oACAEBAAY/Aqf/xAAUEAEAAAAAAAAAAAAAAAAAAAAA/9oACAEBAAE/IV//2gAMAwEAAgADAAAAEP/EABQRAQAAAAAAAAAAAAAAAAAAABD/2gAIAQMBAT8QH//EABQRAQAAAAAAAAAAAAAAAAAAABD/2gAIAQECAT8QH//EABQQAQAAAAAAAAAAAAAAAAAAABD/2gAIAQEAAT8QH//Z',
  'std',
  'b'
);

export function setup() {
  const email = __ENV.EMAIL || `load-${Date.now()}-${randomString(6)}@example.com`;
  const json = { headers: { 'Content-Type': 'application/json' } };

  http.post(
    `${BASE}/api/v1/auth/signup`,
    JSON.stringify({ email: email, password: PASSWORD, businessName: 'Load Test' }),
    json
  );
  const login = http.post(
    `${BASE}/api/v1/auth/login`,
    JSON.stringify({ email: email, password: PASSWORD }),
    json
  );
  const token = login.json('data.accessToken');
  if (!token) {
    fail(`login failed: ${login.status} ${login.body}`);
  }

  const auth = { headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` } };
  const event = http.post(`${BASE}/api/v1/events`, JSON.stringify({ name: 'Load Test Event' }), auth);
  const eventId = event.json('data.id');
  if (!eventId) {
    fail(`event create failed: ${event.status} ${event.body}`);
  }
  return { token: token, eventId: eventId };
}

export default function (data) {
  const auth = { Authorization: `Bearer ${data.token}` };
  const suffix = `${__VU}-${__ITER}-${randomString(6)}`;
  const idem = `load-${suffix}`;

  const initRes = http.post(
    `${BASE}/api/v1/events/${data.eventId}/uploads`,
    JSON.stringify({ filename: `${suffix}.jpg`, contentType: 'image/jpeg', size: JPEG.byteLength }),
    { headers: Object.assign({ 'Content-Type': 'application/json', 'Idempotency-Key': idem }, auth), tags: { step: 'init' } }
  );
  check(initRes, { 'init 201': (r) => r.status === 201 });
  if (initRes.status !== 201) {
    sleep(1);
    return;
  }

  const uploadUrl = initRes.json('data.uploadUrl');
  const photoId = initRes.json('data.photoId');

  const putRes = http.put(uploadUrl, JPEG, {
    headers: { 'Content-Type': 'image/jpeg' },
    tags: { step: 'put' },
  });
  check(putRes, { 'put 2xx': (r) => r.status >= 200 && r.status < 300 });

  const completeRes = http.post(`${BASE}/api/v1/uploads/${photoId}/complete`, null, {
    headers: Object.assign({ 'Idempotency-Key': `${idem}-complete` }, auth),
    tags: { step: 'complete' },
  });
  check(completeRes, { 'complete 200': (r) => r.status === 200 });

  sleep(0.2);
}
