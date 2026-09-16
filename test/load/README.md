# Load tests (Phase 10)

`k6` scenarios for the Phase 10 load targets (`doc/phases/Phase 10 - Hardening and Deploy.md` §6).

| Script | Flow | Target |
|---|---|---|
| `upload_flow.js` | signup → login → create event → init → PUT → complete | 100+ concurrent uploads; init/complete p95 < 100 ms |
| `gallery_read.js` | public event → cursor list → signed URL (optional image fetch) | 1,000+ viewers; p95 < 300 ms |
| `mixed.js` | gallery readers + uploaders together | both paths at once |

## Install and run

```text
# k6 is not installed by default
winget install k6 --source winget      # Windows
brew install k6                        # macOS

k6 run -e BASE_URL=http://localhost:18080 test/load/upload_flow.js
k6 run -e BASE_URL=http://localhost:18080 -e EVENT_SLUG=my-event test/load/gallery_read.js
k6 run -e BASE_URL=http://localhost:18080 -e EVENT_SLUG=my-event -e FETCH_IMAGES=true test/load/mixed.js
```

`EVENT_SLUG` is the public slug of a seeded event with photos (created through
the API or the `F2`/`F3` UI). For a realistic gallery result, seed thousands of
photos into the event first.

## Notes

- The API's rate limits are deliberately low for auth and URL endpoints
  (login 10/min/IP, signed URL generation 60/min/IP, upload init 100/min/user).
  For full-scale runs, point the scripts at an environment whose limits are
  raised, or route reads through the CDN. Upload scripts sign up and log in
  once (`setup()`) and reuse the token, so auth limits are not the bottleneck.
- The upload PUT goes directly from k6 to R2/MinIO over the presigned URL; the
  API never proxies bytes. Thumbnail availability (< 5 s) is produced by the
  worker and is verified separately (see the E2E tests).
- Run gallery reads against the CDN hostname to measure the cached path.
- A k6 run is a smoke test, not a CI gate: run it manually against staging and
  record results in the phase report or an incident review.
