# Phase F1 — Authentication and App Shell — Completion Report

**Status:** COMPLETE
**Completed:** 2026-09-12
**Author:** opencode

---

## 1. Summary

Phase F1 delivered the BFF-lite session model and the protected operator shell. The refresh token is now stored in an httpOnly cookie owned by Next.js route handlers; the access token lives in memory only and is silently restored on reload. Login, signup, logout, password reset, route protection, and the sidebar shell all work against the existing Go auth API, with no backend changes. Unit tests cover the envelope/cookie behavior and the single-flight refresh; the Playwright auth flow runs when a live API is present.

---

## 2. Exit criteria — verification

| Criteria                                                   | Result | Evidence                                                                                                         |
| ---------------------------------------------------------- | ------ | ---------------------------------------------------------------------------------------------------------------- |
| BFF login/signup/refresh/logout only; no general proxy     | PASS   | `apps/web/src/app/api/auth/*` + `src/lib/auth/bff.ts`                                                            |
| Refresh token httpOnly, SameSite=Lax, Secure in production | PASS   | `refreshCookieOptions` in `src/lib/auth/cookies.ts`; unit test asserts cookie set/rotate/clear                   |
| Access token in memory only                                | PASS   | `session.ts` module state; `getAccessToken` passed to the API client                                             |
| Silent restore on reload                                   | PASS   | `SessionProvider` calls `refreshSession()` on mount; verified in build + E2E (skipped without API)               |
| Single-flight refresh (cross-tab with `navigator.locks`)   | PASS   | `withRefreshLock` + module promise; 2 unit tests; server-side coalescing in `refreshOnce` with a concurrent test |
| Unauthenticated `/dashboard` redirects to `/login`         | PASS   | `src/proxy.ts`; Playwright test passed                                                                           |
| No refresh token in response bodies or logs                | PASS   | `bff.ts` strips it; unit tests assert `refreshToken` is undefined in the response                                |
| `?next=` open-redirect guard                               | PASS   | `login/page.tsx` accepts only same-origin relative paths                                                         |
| Optional refresh-cookie rotation with silent failure       | PASS   | Failed refresh clears cookie and marks the session anonymous                                                     |
| Lint / typecheck / unit tests / build                      | PASS   | see section 7                                                                                                    |
| Completion report                                          | PASS   | this document                                                                                                    |

---

## 3. What was built

### 3.1 BFF (`src/lib/auth/`)

- `bff.ts`: `loginHandler`, `signupHandler`, `refreshHandler`, `logoutHandler`, `makeGoFetch`, `serviceUnavailable`. All are pure functions over a `GoFetch` and a `CookieSink`, so they are unit-testable without Next request context.
- `cookies.ts`: `cookieSink` backed by `next/headers`; cookie name `cpd_refresh`, 30-day max age, `httpOnly`, `SameSite=Lax`, `Secure` in production, `Path=/`.
- Route handlers: `src/app/api/auth/{login,signup,refresh,logout}/route.ts`. They parse nothing themselves and return `503 SERVICE_UNAVAILABLE` if `API_BASE_URL` is missing.

### 3.2 Client session

- `session.ts`: in-memory store read with `useSyncExternalStore`; `refreshSession()` single-flights under `navigator.locks` (fallback to a plain promise); `performRefresh` clears state on failure.
- `session-provider.tsx`: `SessionProvider` + `useSession()`; `login`/`signup` post to the BFF and set the session; `logout` clears it.
- `api.ts`: `apiCall` wraps `openapi-fetch` with envelope unwrapping and a single 401 → refresh → retry.
- `errors.ts`: maps API error codes to operator-facing messages (`INVALID_CREDENTIALS`, `ACCOUNT_LOCKED`, `EMAIL_ALREADY_REGISTERED`, rate limits).

### 3.3 Routes and shell

- `src/proxy.ts` (Next 16 middleware): redirects anonymous users away from `/dashboard/*` and authenticated users away from `/login` and `/signup`.
- `(auth)` group: login, signup, forgot-password, reset-password pages using react-hook-form + zod and shadcn `Card`/`Input`/`Label`/`Alert`.
- `(dashboard)` group: sidebar shell (`SidebarProvider`, `AppSidebar`, `SidebarInset`, `SidebarTrigger`), Dashboard home, Account page (TanStack Query → `GET /account/me`).
- Password reset calls the public Go endpoints directly; not part of the session BFF.
- Providers in the root layout: theme, TanStack Query, session, Sonner toaster.

### 3.4 UI primitives added

`card`, `input`, `label`, `alert`, `separator`, `skeleton`, `dropdown-menu`, `avatar`, `sheet`, `tooltip`, `sidebar`, `sonner`. `use-mobile` was moved from `apps/web/src/hooks` into `packages/ui/src/hooks` so the shared sidebar does not depend on the app (the CLI had routed it to the app; its import was corrected to `@workspace/ui/hooks/use-mobile`).

---

## 4. Database changes

None. `api/openapi.yaml` unchanged.

---

## 5. API surface added

None. Consumes `POST /api/v1/auth/{signup,login,refresh,logout}`, `POST /api/v1/auth/password/reset-*`, `GET /api/v1/account/me`.

---

## 6. Files created / modified

```text
apps/web/.env.example
apps/web/eslint.config.mjs
apps/web/package.json
apps/web/src/app/layout.tsx
apps/web/src/app/(auth)/layout.tsx
apps/web/src/app/(auth)/login/page.tsx
apps/web/src/app/(auth)/signup/page.tsx
apps/web/src/app/(auth)/forgot-password/page.tsx
apps/web/src/app/(auth)/reset-password/page.tsx
apps/web/src/app/(dashboard)/layout.tsx
apps/web/src/app/(dashboard)/dashboard/page.tsx
apps/web/src/app/(dashboard)/account/page.tsx
apps/web/src/app/api/auth/login/route.ts
apps/web/src/app/api/auth/signup/route.ts
apps/web/src/app/api/auth/refresh/route.ts
apps/web/src/app/api/auth/logout/route.ts
apps/web/src/components/app-sidebar.tsx
apps/web/src/features/auth/schemas.ts
apps/web/src/features/auth/login-form.tsx
apps/web/src/features/auth/signup-form.tsx
apps/web/src/features/auth/forgot-password-form.tsx
apps/web/src/features/auth/reset-password-form.tsx
apps/web/src/lib/auth/api.ts
apps/web/src/lib/auth/bff.ts
apps/web/src/lib/auth/bff.test.ts
apps/web/src/lib/auth/cookies.ts
apps/web/src/lib/auth/errors.ts
apps/web/src/lib/auth/session.ts
apps/web/src/lib/auth/session.test.ts
apps/web/src/lib/auth/session-provider.tsx
apps/web/src/lib/env.ts
apps/web/src/lib/query-provider.tsx
apps/web/src/proxy.ts
apps/web/e2e/auth.spec.ts
packages/ui/src/hooks/use-mobile.ts
packages/ui/src/components/* (new primitives listed above)
doc/phases/Phase F1 - Auth and Shell.md (status)
doc/phases/Phase F1 - Report.md
doc/Implementation Plan.md
```

---

## 7. Tests

### Unit

- `bff.test.ts` (11): login sets cookie + strips `refreshToken`; error passthrough; invalid JSON 422; malformed auth response 502; refresh without cookie 401; refresh rotation; refresh failure clears cookie; concurrent refresh coalesced to one Go call; logout revokes and clears; logout clears when Go is unreachable; base URL joining.
- `session.test.ts` (3): concurrent refresh single-flight; failure clears session; set/clear in-memory session.

### E2E

- `auth.spec.ts`: unauthenticated `/dashboard` redirects to `/login` (runs everywhere); signup → dashboard → reload → logout (skipped automatically when `NEXT_PUBLIC_API_BASE_URL` is unreachable).

### Verification commands and results

```text
pnpm lint        -> clean
pnpm typecheck   -> clean (api-client, ui, web)
pnpm test        -> 23 tests passed (6 api-client, 17 web)
pnpm build       -> succeeded; routes /login /signup /dashboard /account + 4 auth API routes
pnpm test:e2e    -> 2 passed, 1 skipped (Go API not running locally)
```

---

## 8. Issues found and fixed

| Issue                                                                                          | Fix                                                                                                    |
| ---------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------ |
| shadcn CLI routed `use-mobile` into `apps/web`, making `packages/ui` import an app path        | Moved the hook into `packages/ui/src/hooks` and rewrote the import to `@workspace/ui/hooks/use-mobile` |
| `sonner` dependency landed in `apps/web` but `sonner.tsx` lives in `packages/ui`               | Added `sonner` to `packages/ui` (and kept it in `apps/web` for `toast`)                                |
| Theme toggle icon caused a hydration mismatch, fixed with `setState` in an effect (lint error) | Replaced the mount guard with CSS (`hidden dark:block` / `block dark:hidden`)                          |
| Missing `API_BASE_URL` produced a 500 with a stack trace                                       | Routes now return `503 SERVICE_UNAVAILABLE` via a shared helper                                        |
| `let release: (() => void) \| null` narrowed to `never` in tests                                | Typed as a no-op function and reassigned                                                               |

### Post-phase fix (2026-09-13)

Browser API calls were built from an origin-only base URL, so `/account/me`
(and later `/events`) hit unversioned paths and returned 404 while the BFF
auth flow kept working. The browser client now uses `apiVersionedBaseUrl()`
(origin + `/api/v1`); both env vars remain origins, and the live Playwright
specs load `.env.local` so they actually exercise these calls.

---

## 9. Known limitations / follow-ups

- The full signup→logout E2E requires a running Go API, Postgres, and migrations; it skips otherwise. Run it locally with the stack up before release.
- Account page is read-only; profile editing (`PATCH /account/me`) is a small follow-up.
- `packages/ui` still has no ESLint script.
- Password reset flow was implemented but not exercised against a real mailer (the Go phase ships the endpoint; email delivery is provider-dependent).

---

## 10. How to try it

```text
# terminal 1: infrastructure + API
docker compose up -d
go run ./cmd/api        # or HTTP_ADDR=:18080

# terminal 2: web
corepack enable         # or: npm install -g pnpm
pnpm install
# apps/web/.env.local:
# API_BASE_URL=http://localhost:8080
# NEXT_PUBLIC_API_BASE_URL=http://localhost:8080
pnpm dev
# http://localhost:3000/signup
```
