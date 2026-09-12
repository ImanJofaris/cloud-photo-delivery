# Phase F1 — Authentication and App Shell

> **Status: COMPLETE.** See the [Phase F1 Report](Phase F1 - Report.md) for what was built and how it was verified.

**Goal:** Operators can sign up, log in, reset a password, log out, and use a protected dashboard shell. Session handling is BFF-lite: the refresh token lives in an httpOnly cookie owned by Next; the access token stays in memory.

**Depends on:** Phase F0; Phase 1 (auth API).

**Exit criteria:** A new operator can complete signup → dashboard → logout through the browser; reloading the dashboard silently restores the session; a concurrent refresh across two tabs does not revoke the session family; unauthenticated access to `/dashboard` redirects to `/login`.

---

## 1. Features

- BFF auth route handlers under `apps/web/src/app/api/auth/`:
  - `POST login`, `POST signup` — call the Go API, set the refresh cookie, return `{ accessToken, expiresIn, user }`.
  - `POST refresh` — rotate the cookie via the Go API, return a fresh access token.
  - `POST logout` — call the Go API with the cookie value, clear the cookie.
- Password reset pages: `/forgot-password`, `/reset-password?token=`.
- Client session layer:
  - In-memory access token; `Authorization: Bearer` injected by `packages/api-client`.
  - Single-flight refresh under `navigator.locks` (cross-tab safe) plus per-process server-side coalescing.
  - 401 → refresh → retry once; failed refresh → clear state and redirect to `/login?next=`.
- `proxy.ts` (Next 16's renamed middleware) redirects to `/login` when the refresh cookie is absent. The API's 401 remains the source of truth.
- App shell: sidebar, header, user menu, theme toggle, responsive nav, account/profile page.
- Error UX: `INVALID_CREDENTIALS`, `ACCOUNT_LOCKED` (423), rate limit (429), validation errors (422) mapped to inline form messages.

---

## 2. Locked decisions

| Decision              | Choice                                                                               | Rationale                                                                                   |
| --------------------- | ------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------- |
| Refresh storage       | httpOnly, `SameSite=Lax`, `Secure` in production, `Path=/` cookie on the Next origin | JS never sees the 30-day credential; no CORS/credentials changes to the Go API needed       |
| Access token          | Memory only, 15 min TTL                                                              | Short exposure window; lost on reload and re-fetched by silent refresh                      |
| Refresh serialization | `navigator.locks` client lock + server coalescing keyed by cookie                    | Go revokes the whole family on reuse (`internal/auth/service.go`); two tabs must never race |
| Cookie name           | `cpd_refresh` (`__Host-` prefix considered for production hardening)                 | `__Host-` requires `Secure` and breaks on plain-HTTP localhost                              |
| BFF scope             | Auth routes only                                                                     | Feature calls go browser → Go directly; no general proxy, per `doc/frontend/Conventions.md` |
| Route guarding        | `proxy.ts` cookie-presence check + client session state                              | Edge cheap redirect for UX; real authorization stays in the API                             |

---

## 3. File map

```text
apps/web/src/app/(auth)/
  login/page.tsx
  signup/page.tsx
  forgot-password/page.tsx
  reset-password/page.tsx
apps/web/src/app/(dashboard)/
  layout.tsx                  # app shell + session provider
  dashboard/page.tsx
  account/page.tsx
apps/web/src/app/api/auth/
  login/route.ts
  signup/route.ts
  refresh/route.ts
  logout/route.ts
apps/web/src/lib/auth/
  session.ts                 # in-memory token, user, refresh promise
  provider.tsx               # SessionProvider + useSession
  server.ts                  # cookie read/write helpers, fetch to Go
apps/web/src/proxy.ts
```

---

## 4. Security rules

- The refresh cookie is `httpOnly`, `SameSite=Lax`, `Secure` in production, and never echoed in a response body or log.
- The BFF strips `refreshToken` from the Go API response before returning anything to the browser.
- No tokens in `localStorage`, `sessionStorage`, or URLs.
- `/login?next=` values are validated to be same-origin relative paths before redirect to prevent open redirects.
- Server route handlers validate input with zod; no Go error details beyond `code`/`message` reach the client.

---

## 5. Test Plan

**Unit (Vitest + RTL)**

- [ ] `session.ts`: single-flight refresh returns one promise for concurrent callers; failure clears state.
- [ ] Auth form components: validation, submit, 401/423/429 error mapping.
- [ ] BFF route handlers: cookie is set on login/signup, cleared on logout, rotated on refresh; `refreshToken` is never in a response body.

**E2E (Playwright)**

- [ ] signup → dashboard → reload restores session → logout → `/dashboard` redirects to `/login`.
- [ ] wrong password shows `INVALID_CREDENTIALS`; locked account shows the lockout message.
- [ ] two concurrent refresh calls both succeed and the session survives.

**Gates**

```text
pnpm lint
pnpm typecheck
pnpm test
pnpm build
pnpm test:e2e
```

---

## 6. Definition of Done

- [ ] All master DoD items that apply.
- [ ] No OpenAPI changes required; if auth contract changes are needed, `api/openapi.yaml` and the Go handler land first.
- [ ] Refresh rotation/reuse behavior verified against the real Go API (two-tab scenario included).
- [ ] No token or cookie value appears in browser logs, server logs, or response bodies.
- [ ] AGENTS.md and `doc/frontend/Conventions.md` reflect the final auth flow.
- [ ] Completion report written.
