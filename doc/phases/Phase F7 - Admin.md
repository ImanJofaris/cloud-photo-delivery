# Phase F7 — Admin

> **Status: NOT STARTED.** Backend Phase 9 (admin API) is complete. This phase adds one small backend prerequisite (`isAdmin` on the session profile, §2.1) and a read-only admin UI. A completion report will be linked here when done.

**Goal:** A promoted operator (admin) can open an Admin page and see platform-wide totals, the operator list, subscriptions, and job-queue health — read-only, against the existing Phase 9 admin API plus an `isAdmin` flag exposed on the profile.

**Depends on:** Phase F0 (env, api-client, shell); Phase F1 (session, BFF, `apiCall`); Phase 9 (admin API — complete). §2.1 must land before the UI. Phase F6 is complete; no other phase is in progress.

**Exit criteria:** After a SQL promotion (`UPDATE users SET is_admin = TRUE`), an admin signs in and sees an Admin entry in the sidebar; `/admin` shows six stats cards, queue health, and cursor-paginated operator and subscription tables; a non-admin sees no Admin entry, is redirected away from `/admin`, and the API's 403 remains the authority; no mutation exists anywhere in the UI.

---

## 1. Features

### 1.1 `isAdmin` in the session (§2.1 prerequisite)

- `GET /account/me` and the auth responses (login / signup / refresh) return `isAdmin` (boolean, always present; `false` for ordinary operators).
- `SessionUser` carries it; navigation and the route guard use it. Promotion stays SQL-only by design (Phase 9): there is no grant/revoke endpoint and this phase adds none.

### 1.2 Admin page

- `/admin` (sidebar: Admin, `ShieldCheck`, after Billing, admins only):
  - **Stats grid** (`GET /admin/stats`): six cards — Operators, Events, Photos, Storage (`formatBytes`), Revenue (`formatMYR`), Subscriptions. Values "—" while loading, never a flash of `0`.
  - **Queue health** (`GET /admin/health`): status badge (`ok`/`degraded`), `queueDepth`, pending / running / failed, oldest pending `run_at` (relative), `checkedAt`; degraded styling when `status === "degraded"`.
  - **Operators table** (`GET /admin/users`, cursor, limit 20): business name, email, admin badge, event count, storage, joined date; "Load more" while `nextCursor` is non-null.
  - **Subscriptions table** (`GET /admin/subscriptions`, cursor, limit 20): operator email, plan, status badge, interval, current period end, cancel-at-period-end flag; "Load more".
  - Per-section empty states; feature-local error copy (including 403 `FORBIDDEN`).
- Strictly read-only: no actions column, no promotion UI, no operator detail (the API has no `GET /admin/users/{id}`).

### 1.3 Shell & guard

- `components/app-sidebar.tsx`: nav descriptors gain `adminOnly`; `visibleNavItems(isAdmin)` filters; the Admin entry renders only for admins.
- `proxy.ts`: `/admin` added to `PROTECTED_PREFIXES` and the matcher (cookie-presence check only).
- `AdminPanel`: while the session is `loading`, show skeletons; when authenticated and `!user.isAdmin`, `router.replace("/dashboard")` — no 403 page. The API is the authority regardless.

---

## 2. Backend contract

### 2.1 Prerequisite: `isAdmin` on `Profile` (must land first)

- `api/openapi.yaml`: add optional `isAdmin: { type: boolean }` to `Profile` (~line 2281). `Profile` is shared by `AuthResponse` (login / signup / refresh) and `ProfileEnvelope` (`GET/PATCH /account/me`), so the single change covers every consumer. Optional (not in `required`) so existing fixtures keep compiling; the UI treats `undefined` as `false`.
- Go:
  - `internal/users/model.go` — `IsAdmin bool` on `User`.
  - `internal/users/repository.go` — add `is_admin` to `userColumns` and `scanUser`.
  - `internal/users/handler.go` — `IsAdmin` in `profileDTO` / `toProfileDTO`.
  - `internal/auth/handler.go` — `IsAdmin` in `userDTO` / `toUserDTO`.
- Tests: repository integration scans `is_admin`; users/auth handler tests assert `isAdmin: false`; `test/e2e/admin_flow_test.go` promotes with SQL and asserts `isAdmin: true` on login and `/account/me` (ordinary user stays `false`).
- `pnpm gen:api`; generated diff must be stable. **No migration** — `users.is_admin` exists (`migrations/0013_admin.sql`).

### 2.2 Admin API (existing, read-only)

- Contracts: `AdminStats` (users, events, photos, storageBytes, revenueCents, subscriptions); `AdminUser` (`isAdmin`, `storageBytes`, `eventCount`, `createdAt`); `AdminSubscription` (`userEmail`, `planId`, `status`, `interval`, `currentPeriodEnd`, `cancelAtPeriodEnd`); `AdminHealth` (`status`, `queueDepth`, `queue.{pending,running,failed,oldestPendingAt}`, `checkedAt`).
- Quirks the UI must respect: list envelopes use `users` / `subscriptions` keys with `nextCursor` (not `items`); `limit` 1–100, default 20; stats semantics — events excludes deleted, photos excludes failed uploads, revenue counts paid invoices, subscriptions counts trialing / active / past_due; `health.status` is `degraded` when any job has failed.
- Auth: bearer; 401 unauthenticated, 403 `FORBIDDEN` non-admin (checked per request against the DB flag), 422 `VALIDATION_ERROR` for bad cursor/limit, 429 `RATE_LIMITED` (admin limiter 30 / 10 min / user).
- Out of scope, by decision: mutations, promotion endpoints, `GET /admin/users/{id}`, storage-reconcile report endpoint, audit-log browsing.

---

## 3. API surface consumed

```http
GET /api/v1/account/me                    # isAdmin (prerequisite); also login/signup/refresh
GET /api/v1/admin/stats
GET /api/v1/admin/users?cursor=&limit=
GET /api/v1/admin/subscriptions?cursor=&limit=
GET /api/v1/admin/health
```

---

## 4. File map

```text
api/openapi.yaml                              # Profile.isAdmin (§2.1)
internal/users/model.go                       # User.IsAdmin
internal/users/repository.go                  # userColumns + scanUser
internal/users/handler.go                     # profileDTO
internal/auth/handler.go                      # userDTO
apps/web/src/lib/auth/session.ts              # SessionUser.isAdmin
apps/web/src/components/app-sidebar.tsx       # admin-only nav + visibleNavItems
apps/web/src/proxy.ts                         # /admin protected + matcher
apps/web/src/app/(dashboard)/admin/page.tsx   # server wrapper -> AdminPanel
apps/web/src/features/admin/
  api.ts                                      # useAdmin{Stats,Users,Subscriptions,Health}
  keys.ts                                     # adminKeys
  errors.ts format.ts
  admin-panel.tsx                             # guard + layout + sections
  admin-stats.tsx                             # 6-card grid
  admin-health-card.tsx                       # status + queue counters
  admin-users-table.tsx                       # table + Load more
  admin-subscriptions-table.tsx               # table + Load more
test/e2e/admin_flow_test.go                   # isAdmin assertions added
```

---

## 5. Data and state rules

- All calls go through `apiCall` from `lib/auth/api.ts`; use generated types only (Conventions §2).
- Query keys are colocated: `adminKeys.stats()`, `adminKeys.users()`, `adminKeys.subscriptions()`, `adminKeys.health()`. Users/subscriptions use `useInfiniteQuery` (limit 20, `pageParam` cursor, `getNextPageParam` from `nextCursor`, page fields `users` / `subscriptions`).
- `stats` / `health` are plain queries with a 60 s `staleTime`; the page is read-only, so there are no mutations and nothing to invalidate. No polling.
- Session: `SessionUser.isAdmin` is optional and defaults to `false`; a SQL promotion is picked up on the next page load (refresh-on-mount), no re-login required.
- Formatting: reuse `formatBytes` (`features/events/format`) and `formatMYR` (`features/billing/format`); dates via the existing format helpers, local timezone.
- No client-side business rules (Conventions §10): the API stays authoritative for access, pagination, and totals.

---

## 6. UX rules

- Read-only affordance: a short line on the page — "Read-only overview." No disabled action buttons implying future actions.
- Non-admins never see the Admin entry; authenticated non-admins landing on `/admin` are redirected to `/dashboard`; a 403 returned mid-session maps to a clear error card, not raw server text.
- Numbers via `Intl.NumberFormat` (`en-MY`); storage via `formatBytes`; "—" while loading; distinct empty states per table.
- Status is never color-only: health and subscription statuses render as text badges.
- Accessibility: tables use header cells with scopes, "Load more" is keyboard reachable and disabled while fetching, badges carry readable text.

---

## 7. Test Plan

**Unit (Vitest + RTL)**

- [ ] `visibleNavItems(isAdmin)`: Admin included for admins, excluded otherwise; order preserved.
- [ ] `proxy.test.ts`: `/admin` in the exact `PROTECTED_PREFIXES` array and matcher; unauthenticated `/admin` → `/login?next=/admin`; cookie present passes through.
- [ ] Admin panel (mocked hooks): stats cards, health card, both tables render; loading placeholders; empty states; `FORBIDDEN` error copy.
- [ ] Tables: "Load more" disabled while fetching; hidden when `nextCursor` is null; rows flatten across pages.
- [ ] Format helpers: bytes, MYR, relative "oldest pending" time.

**Go**

- [ ] Handler tests: login/signup/refresh and `/account/me` include `isAdmin`; default `false`.
- [ ] Repository integration: `scanUser` selects `is_admin` for both `GetByID` and `GetByEmail`.
- [ ] `TestE2E_AdminFlow`: after SQL promotion, login and `/account/me` return `isAdmin: true`; ordinary user returns `false`; existing stats/users/subscriptions/health coverage unchanged.

**E2E (Playwright)**

- [ ] No new spec by decision: a Playwright run cannot promote a user to admin (no promotion endpoint; the local `api-proxy.mjs` has no DB access). The API flow is covered by the Go E2E and the UI by Vitest. Recorded as a known limitation in the report. (Alternative, not chosen: a local-only promote route in `e2e/api-proxy.mjs` plus a `pg` devDependency.)

**Gates**

```text
pnpm gen:api && git diff --exit-code packages/api-client/src/schema.d.ts
pnpm lint
pnpm typecheck
pnpm test
pnpm build
gofmt -l .
go build ./...
go vet ./...
go vet -tags=integration ./...
go test ./...
go test -tags=integration ./...
```

Manual smoke: `docker exec cpd_postgres psql -U cpd -d cpd -c "UPDATE users SET is_admin = TRUE WHERE email = '<you>';"` → reload → Admin appears → `/admin` renders stats, health, and both tables.

---

## 8. Definition of Done

- [ ] OpenAPI + Go land before the frontend; `pnpm gen:api` produces a stable diff.
- [ ] `isAdmin` is present on login/signup/refresh and `/account/me`; `false` for ordinary operators; no migration added.
- [ ] Admin nav entry only for admins; `/admin` protected in `proxy.ts` and client-guarded; the API's 403 remains authoritative.
- [ ] Stats, health, operators, and subscriptions render with loading / empty / error states and cursor pagination; strictly read-only.
- [ ] No mutations, promotion UI, or admin actions added; no new dependencies; no new env vars.
- [ ] Unit tests cover nav, proxy, panel, tables, and formatting; Go tests updated; all gates green.
- [ ] Phase 9 report's "no admin UI" limitation points at this phase; `doc/Implementation Plan.md` frontend track lists F7 and its ordering note.
- [ ] Completion report written from the template.
