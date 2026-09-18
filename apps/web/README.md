# apps/web

Next.js operator dashboard and public gallery for Cloud Photo Delivery. The browser talks directly to the Go API using a Bearer access token; only the auth routes under `src/app/api/auth/*` run as a BFF.

For full local setup (Postgres, MinIO, API, worker), see the root [README](../../README.md#run-locally).

## Prerequisites

- Node 20+ and pnpm (`corepack enable` once)
- Go API running on `http://localhost:18080` (see `apps/web/.env.local`; `API_BASE_URL` is server-only, `NEXT_PUBLIC_API_BASE_URL` is used by the browser)

## Commands

```text
pnpm install              # from the repo root (pnpm workspace)
pnpm dev                  # http://localhost:3000
pnpm lint                 # eslint
pnpm typecheck            # tsc
pnpm test                 # vitest
pnpm build                # production build
pnpm gen:api              # regenerate packages/api-client from api/openapi.yaml
pnpm test:e2e             # Playwright; expects the API on :18081
```

Run these from the repo root or with `pnpm --filter web <script>`.

## Rules

See [doc/frontend/Conventions.md](../../doc/frontend/Conventions.md) for the authoritative frontend conventions: generated API types only, uploads go browser-to-R2 via presigned URLs, no `next/image` optimization for signed URLs.
