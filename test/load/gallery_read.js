// Public gallery read load test (Phase 10, §6): event -> cursor list -> signed
// URL, optionally fetching the image itself (which goes to the CDN/R2, never
// through the API).
//
//   k6 run -e BASE_URL=http://localhost:18080 -e EVENT_SLUG=my-event test/load/gallery_read.js
//
// The gallery is designed to sit behind Cloudflare, so run this against the
// CDN hostname for the real 1,000-viewer target. Signed URL generation is
// limited to 60/min/IP by the API; use a proxy/CDN or raise the limit for the
// URL step when driving it from a single host.
import http from 'k6/http';
import { check, fail, sleep } from 'k6';

export const options = {
  scenarios: {
    viewers: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: 200 },
        { duration: '2m', target: 1000 },
        { duration: '30s', target: 0 },
      ],
      gracefulRampDown: '10s',
    },
  },
  thresholds: {
    checks: ['rate>0.99'],
    http_req_failed: ['rate<0.01'],
    'http_req_duration{step:event}': ['p(95)<300'],
    'http_req_duration{step:list}': ['p(95)<300'],
  },
};

const BASE = (__ENV.BASE_URL || 'http://localhost:18080').replace(/\/+$/, '');
const SLUG = __ENV.EVENT_SLUG;
const FETCH_IMAGES = __ENV.FETCH_IMAGES === 'true';

export function setup() {
  if (!SLUG) {
    fail('EVENT_SLUG is required (the public event slug to read)');
  }
  return { slug: SLUG };
}

export default function (data) {
  const event = http.get(`${BASE}/api/v1/public/events/${data.slug}`, { tags: { step: 'event' } });
  check(event, { 'event 200': (r) => r.status === 200 });

  const list = http.get(`${BASE}/api/v1/public/events/${data.slug}/photos?limit=30`, {
    tags: { step: 'list' },
  });
  check(list, { 'list 200': (r) => r.status === 200 });

  const photos = list.json('data.items') || [];
  if (photos.length === 0) {
    sleep(1);
    return;
  }

  if (Math.random() < 0.2) {
    const photo = photos[Math.floor(Math.random() * photos.length)];
    const urlRes = http.get(
      `${BASE}/api/v1/public/events/${data.slug}/photos/${photo.id}/url`,
      { tags: { step: 'url' } }
    );
    check(urlRes, { 'url 200': (r) => r.status === 200 });

    if (FETCH_IMAGES) {
      const signedUrl = urlRes.json('data.url');
      if (signedUrl) {
        const image = http.get(signedUrl, { tags: { step: 'image' } });
        check(image, { 'image 2xx': (r) => r.status >= 200 && r.status < 300 });
      }
    }
  }

  sleep(Math.random() * 0.5 + 0.2);
}
