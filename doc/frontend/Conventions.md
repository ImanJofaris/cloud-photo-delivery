# Frontend Conventions

This is the working agreement for `apps/web`, `packages/ui`, and `packages/api-client`. It complements `doc/Contributing.md`; the API remains the source of truth.

**Stack:** Next.js 16 (App Router, React 19, TypeScript strict), Tailwind CSS v4, shadcn/ui components in `packages/ui`, TanStack Query for server state, react-hook-form + zod for forms, Vitest + Testing Library + Playwright for tests.

---

## 1. Workspace layout

```text
apps/web/                # Next.js app (operator dashboard now, public gallery later)
packages/ui/             # shadcn components, hooks, globals.css, cn()
packages/api-client/     # generated OpenAPI types + typed fetch wrapper
```

- Package manager is **pnpm** workspaces (`corepack enable`). Do not mix npm/yarn lockfiles.
- Conventional workspace names: `web`, `@workspace/ui`, `@workspace/api-client`.
- The Go service stays at the repo root. The frontend consumes it over HTTP only.
- Run shadcn CLI commands from `apps/web`; the CLI routes components into `packages/ui` via the monorepo config.

```text
corepack enable
pnpm install
pnpm dev            # apps/web on :3000; Go API expected on API_BASE_URL
pnpm gen:api        # regenerate packages/api-client from api/openapi.yaml
pnpm lint
pnpm typecheck
pnpm test
pnpm test:e2e
```

---

## 2. The API is the contract

- All request/response types come from `api/openapi.yaml` via `openapi-typescript`. Generated output is committed; hand-written duplicate types are forbidden.
- `api/openapi.yaml` changes land **before** the frontend uses them. If a screen needs a field the API does not return, change the API first.
- Envelope handling is centralized in `packages/api-client`: unwrap `{ data, error }`, throw a typed `ApiError` carrying `error.code`, `error.message`, and HTTP status.
- UI maps error codes (`EVENT_NOT_FOUND`, `INVALID_CREDENTIALS`, `PLAN_LIMIT_REACHED`, ...) to user-facing messages. Never render raw error messages from the server.
- Cursor pagination only (`?cursor=&limit=`). Offset pagination is banned, matching the API.

---

## 3. Session and auth (BFF-lite)

The browser never stores the refresh token.

```text
Browser                     Next route handlers              Go API
  │  POST /api/auth/login         │                             │
  ├───────────────────────────────►│  POST /api/v1/auth/login    │
  │                                ├────────────────────────────►│
  │                                │◄────────────────────────────┤
  │  { accessToken, user }         │
  │◄───────────────────────────────┤  Set-Cookie: refresh (httpOnly)
  │                                │
  │  GET /api/v1/events  (Bearer access token, held in memory)
  ├──────────────────────────────────────────────────────────────►│
```

- The refresh token lives in an `httpOnly`, `SameSite=Lax`, `Secure` (production) cookie scoped to the Next origin. The access token (15 min) is held in memory only.
- The BFF route handlers under `apps/web/src/app/api/auth/` are the **only** proxy. Never add a general-purpose API proxy; feature calls go from the browser directly to the Go API with the Bearer token (public gallery endpoints need no auth at all).
- The Go API rotates the refresh token on every refresh and revokes the whole family on reuse (`internal/auth/service.go`). Serialize refreshes:
  - Client: a single in-flight refresh promise, taken under `navigator.locks` so multiple tabs cannot race.
  - Server: coalesce concurrent refreshes for the same cookie per process.
  - A failed refresh clears the cookie and returns the user to `/login?next=`.
- 401 handling: transparent single-flight refresh, then retry the original request once. 423 (`ACCOUNT_LOCKED`) and 429 are surfaced explicitly.
- Public gallery unlock tokens are event-scoped, short-lived, and kept in memory/session storage only; they are never treated as account sessions.

---

## 4. Data fetching

- Server Components for first render where it helps (public gallery metadata, dashboard shells). Client Components + TanStack Query for interactive/authenticated data.
- Query keys are colocated with the feature (`events.keys.ts`) and include every input (filters, cursor, event id).
- Mutations invalidate the narrowest key that changed. Optimistic updates only where rollback is trivial (archive, delete).
- No global client-state library until there is real shared client state. Server state belongs to TanStack Query; forms belong to react-hook-form.

---

## 5. Images and signed URLs

- The API returns **no image URLs** in metadata; each variant URL is fetched per photo and expires in `SIGNED_URL_TTL` (default 5 min).
- Fetch signed URLs only for photos that are visible or about to be visible (`IntersectionObserver`), never for a whole page at once.
- Do **not** use `next/image` optimization for signed R2 URLs. The URL changes on every fetch, so the optimizer caches nothing and adds latency. Use plain `<img loading="lazy" decoding="async">` with explicit dimensions.
- Never log, persist, or embed signed URLs in HTML that outlives their TTL. Cache them only in memory with an expiry shorter than the TTL.

---

## 6. Uploads

- The browser uploads directly to R2 with presigned URLs. The frontend must never send photo bytes to the Go API or the Next server.
- Flow: `POST /events/{id}/uploads` (with `Idempotency-Key`) → `PUT` to the presigned URL(s) with `XMLHttpRequest` for progress → `POST /uploads/{id}/complete`.
- Use simple PUT for small files and multipart for large files behind the same uploader interface. Resume/recover via `GET /uploads/{id}`; abort multipart uploads on cancel.
- Uploads are queued and retried with backoff; failures never block the rest of the batch.

---

## 7. UI and shadcn

- All shared primitives live in `packages/ui`; app-specific compositions live in `apps/web/src/components`. Do not fork shadcn primitives into the app.
- Both `components.json` files keep the same `style`, `baseColor`, and `iconLibrary`.
- The shadcn MCP server is configured in `opencode.json` and prefers registry components over hand-rolled ones.
- Design mobile-first for the gallery (guests are on phones); the operator dashboard is desktop-first with usable small-screen fallbacks.
- Accessibility is part of done: keyboard navigation, focus states, labelled inputs, `aria-*` where the primitive does not already provide it.

---

## 8. Environment and configuration

- Server-only values use plain env vars (`API_BASE_URL`). Only values that must reach the browser use `NEXT_PUBLIC_*`.
- Both API env vars are **origins only** (`http://localhost:8080`); the `/api/v1` prefix is applied in code (`apiVersionedBaseUrl()` for the browser client, hardcoded `/api/v1/...` paths in the auth BFF). Never set the versioned path in the env vars.
- No secrets in the client bundle; the refresh cookie is the only credential the browser holds.
- `.env.local` is gitignored; `apps/web/.env.example` documents every variable.
- Local ports: Go API on `:8080` (use `HTTP_ADDR=:18080` if WinNAT reserves it), Next on `:3000`.

---

## 9. Testing

- Unit/component: Vitest + Testing Library beside the code (`*.test.tsx`).
- E2E: Playwright in `apps/web/e2e/`, covering the phase's critical flow against a running Go API.
- No test may depend on execution order or shared mutable state. Mock at the network boundary (`msw` or a fake fetch), not internal modules.
- Every phase's critical flow gets one E2E test. Business rules are tested in Go; the frontend tests rendering, interaction, and error mapping.

---

## 10. Hard rules

```text
Never proxy photo bytes through Next or Go request paths.
Never hand-write API types that codegen can produce.
Never store the refresh token in JS-accessible storage.
Never fetch signed URLs for photos that are not visible.
Never add business rules to the frontend; the API enforces them.
```
