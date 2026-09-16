# Phase F7 — Admin — Completion Report

**Status:** COMPLETE
**Completed:** 2026-09-16
**Author:** opencode

---

## 1. Summary

Phase F7 adds the read-only admin surface: an `isAdmin` flag on the session profile (the phase's only backend change) and an `/admin` page that renders platform stats, queue health, and cursor-paginated operator and subscription tables against the existing Phase 9 admin API. Promotion stays SQL-only; the UI contains no mutations. Operators see no Admin entry and are redirected away from `/admin`, while the API's per-request `403 FORBIDDEN` remains the authority.

---

## 2. Exit criteria — verification

| Criteria | Result | Evidence |
|---|---|---|
| After `UPDATE users SET is_admin = TRUE`, an admin signs in and sees an Admin entry in the sidebar | PASS | `visibleNavItems(true)` includes `/admin` after `/billing` (`app-sidebar.test.ts`); sidebar renders `visibleNavItems(user?.isAdmin)` |
| `/admin` shows six stats cards | PASS | `AdminStatsGrid` renders Operators, Events, Photos, Storage, Revenue, Subscriptions with `—` while loading (`admin-panel.test.tsx`) |
| `/admin` shows queue health | PASS | `AdminHealthCard` renders `status` badge, `queueDepth`, pending/running/failed, relative oldest pending, `checkedAt` |
| `/admin` shows cursor-paginated operator and subscription tables | PASS | `useAdminUsers`/`useAdminSubscriptions` (`limit: 20`, `pageParam` cursor, `getNextPageParam` from `nextCursor`); tables flatten pages and offer "Load more" |
| A non-admin sees no Admin entry | PASS | `visibleNavItems(false)` excludes Admin; test asserts exact titles |
| A non-admin is redirected away from `/admin` | PASS | `AdminPanel` calls `router.replace("/dashboard")` when authenticated and `!user.isAdmin` (`admin-panel.test.tsx`) |
| The API's 403 remains the authority | PASS | Queries carry the bearer token; a mid-session `FORBIDDEN` renders "Admin access required." (`admin-panel.test.tsx`); Go E2E asserts `403 FORBIDDEN` for non-admins |
| No mutation exists anywhere in the UI | PASS | No `useMutation` in `features/admin/`; page is strictly read-only |
| `GET /account/me` and auth responses return `isAdmin` | PASS | Live smoke: signup/login `"isAdmin":false`, after SQL promotion login and `/account/me` `"isAdmin":true`; `TestE2E_AdminFlow` asserts the same |

---

## 3. What was built

### 3.1 `isAdmin` on the profile (§2.1)

- `api/openapi.yaml` — optional `isAdmin: { type: boolean }` on `Profile`, shared by `AuthResponse` (login / signup / refresh) and `ProfileEnvelope` (`GET/PATCH /account/me`); not in `required`, so existing fixtures compile and the UI treats `undefined` as `false`.
- Go — `User.IsAdmin` (`internal/users/model.go`), `is_admin` added to `userColumns`/`scanUser` (`internal/users/repository.go`), `IsAdmin` on `profileDTO` (`internal/users/handler.go`) and `userDTO` (`internal/auth/handler.go`).
- Codegen — `pnpm gen:api` adds one line to `packages/api-client/src/schema.d.ts`.

### 3.2 Session & shell

- `apps/web/src/lib/auth/session.ts` — `SessionUser.isAdmin?: boolean`.
- `apps/web/src/components/app-sidebar.tsx` — nav descriptors typed with `adminOnly?: boolean`; `visibleNavItems(isAdmin)` filters; Admin (`ShieldCheck`) sits between Billing and Account.
- `apps/web/src/proxy.ts` — `/admin` in `PROTECTED_PREFIXES` and `/admin/:path*` in the matcher (cookie-presence check only).

### 3.3 Admin feature (`apps/web/src/features/admin/`)

- `keys.ts` — `adminKeys.{all,stats,users,subscriptions,health}`.
- `api.ts` — `useAdminStats` and `useAdminHealth` plain queries with a 60 s `staleTime`; `useAdminUsers`/`useAdminSubscriptions` `useInfiniteQuery` (`limit: 20`, `getNextPageParam` from `nextCursor`, page fields `users`/`subscriptions`); every hook takes `enabled` so non-admins never fire a doomed request.
- `errors.ts` — `FORBIDDEN` → "Admin access required."; `VALIDATION_ERROR` / `RATE_LIMITED` mapped; fallback never leaks server text.
- `format.ts` — re-exports `formatBytes` (events) and `formatPendingAge` (relative "N minutes/hours/days ago", "just now", `—` for null/invalid).
- `admin-stats.tsx` — six-card grid; `—` while loading, never a flash of `0` (Reused `formatCount`, `formatMYR`).
- `admin-health-card.tsx` — `ok`/`degraded` text badge (`destructive` when degraded), queue depth, pending/running/failed, oldest-pending age, checked timestamp; skeleton while loading.
- `admin-users-table.tsx` — business, email, admin badge, event count, storage, joined; empty state; `Load more`.
- `admin-subscriptions-table.tsx` — operator email, plan, status badge, interval, current period end, cancel flag; empty state; `Load more`.
- `admin-panel.tsx` — h1 "Admin" + "Read-only overview."; skeletons while `status === "loading"`; redirect for authenticated non-admins; per-section skeleton/empty/error composition.
- `app/(dashboard)/admin/page.tsx` — server wrapper rendering `AdminPanel`.

---

## 4. Database changes

None. `users.is_admin` already exists (`migrations/0013_admin.sql`); no migration was added, no change to the down path.

---

## 5. API surface added

No new endpoints. The only contract change is the `isAdmin` field on `Profile`, which surfaces through:

```http
POST /api/v1/auth/signup   # data.user.isAdmin
POST /api/v1/auth/login    # data.user.isAdmin
POST /api/v1/auth/refresh  # data.user.isAdmin
GET  /api/v1/account/me    # data.isAdmin
```

Example (live smoke, after `UPDATE users SET is_admin = TRUE`):

```json
{
  "data": {
    "id": "4aff8db2-fc01-4171-a8db-1a6de04104c3",
    "email": "f7smoke20260916@example.com",
    "businessName": "F7 Smoke",
    "isAdmin": true
  },
  "error": null
}
```

Consumed (unchanged, Phase 9): `GET /api/v1/admin/{stats,users,subscriptions,health}`.

---

## 6. Files created / modified

```text
api/openapi.yaml
internal/users/model.go
internal/users/repository.go
internal/users/handler.go
internal/users/handler_test.go                          (new)
internal/users/repository_integration_test.go
internal/auth/handler.go
internal/auth/handler_test.go
internal/auth/service_integration_test.go
test/e2e/admin_flow_test.go
test/e2e/auth_flow_test.go
test/e2e/analytics_flow_test.go
test/e2e/billing_flow_test.go
test/e2e/events_flow_test.go
test/e2e/exports_flow_test.go
test/e2e/gallery_flow_test.go
test/e2e/lifecycle_flow_test.go
test/e2e/uploads_flow_test.go
packages/api-client/src/schema.d.ts
apps/web/src/lib/auth/session.ts
apps/web/src/components/app-sidebar.tsx
apps/web/src/components/app-sidebar.test.ts
apps/web/src/proxy.ts
apps/web/src/proxy.test.ts
apps/web/src/app/(dashboard)/admin/page.tsx             (new)
apps/web/src/features/admin/keys.ts                     (new)
apps/web/src/features/admin/api.ts                      (new)
apps/web/src/features/admin/errors.ts                   (new)
apps/web/src/features/admin/format.ts                   (new)
apps/web/src/features/admin/format.test.ts              (new)
apps/web/src/features/admin/admin-stats.tsx             (new)
apps/web/src/features/admin/admin-health-card.tsx       (new)
apps/web/src/features/admin/admin-users-table.tsx       (new)
apps/web/src/features/admin/admin-users-table.test.tsx  (new)
apps/web/src/features/admin/admin-subscriptions-table.tsx      (new)
apps/web/src/features/admin/admin-subscriptions-table.test.tsx (new)
apps/web/src/features/admin/admin-panel.tsx             (new)
apps/web/src/features/admin/admin-panel.test.tsx        (new)
doc/phases/Phase F7 - Admin.md
doc/Implementation Plan.md
```

---

## 7. Tests

### Unit (Vitest + RTL, 6 files / 31 tests, 22 new)

- `visibleNavItems`: Admin present for admins, absent otherwise, order preserved (Admin immediately after Billing).
- `proxy.test.ts`: `/admin` in the exact `PROTECTED_PREFIXES` array; `/admin/:path*` in the matcher; unauthenticated `/admin` → `/login?next=/admin`; authenticated passes through.
- `AdminPanel`: stats/health/both tables for an admin; pages flattened across cursors; skeletons (4) while the session loads and hooks called with `enabled: false`; empty states; `FORBIDDEN` copy with no server-text leakage; authenticated non-admin redirected to `/dashboard`.
- `AdminUsersTable` / `AdminSubscriptionsTable`: columns and badges, empty states, "Load more" hidden at `nextCursor === null`, click-through, disabled while fetching.
- `format`: bytes, MYR, relative oldest-pending age (sub-minute / minutes / hours / days / invalid).

### Go

- `internal/users/handler_test.go` (new): `/account/me` emits `"isAdmin":false` for ordinary operators and `true` for admins.
- `internal/auth/handler_test.go`: signup and login responses include `isAdmin:false`.
- `internal/users` repository integration: `scanUser` selects `is_admin` for both `GetByID` and `GetByEmail` (fixture table updated).
- `TestE2E_AdminFlow`: after the SQL promotion, login and `GET /account/me` return `isAdmin:true`; the non-promoted operator stays `false`; existing stats/users/subscriptions/health/reconcile coverage unchanged.

### E2E (Playwright)

- Not added by decision. A Playwright run cannot promote a user (no promotion endpoint; the local `api-proxy.mjs` has no DB access). The API flow is covered by the Go E2E and the UI by Vitest — recorded under known limitations.

### Verification commands and results

```text
gofmt -l .                                        -> (nothing)
go build ./...                                    -> exit 0
go vet ./...                                      -> exit 0
go vet -tags=integration ./...                    -> exit 0
go test ./...                                     -> all packages ok
go test -tags=integration ./...                   -> all packages ok (test/e2e 159.2s)
pnpm gen:api (rerun)                              -> schema.d.ts hash stable; diff vs HEAD is the single isAdmin line
pnpm lint                                         -> exit 0
pnpm typecheck                                    -> exit 0 (ui, api-client, web)
pnpm test                                         -> 43 files / 235 tests (web) + 1 file / 7 tests (api-client) passed
pnpm build                                        -> compiled successfully; /admin listed as a static route
```

---

## 8. Issues found and fixed

| Issue | Fix |
|---|---|
| Adding `is_admin` to `userColumns` broke every integration fixture that builds its own `users` table (10 files) | Added `is_admin BOOLEAN NOT NULL DEFAULT FALSE` to each inline DDL |
| Unconditional admin queries would fire a doomed 403 for non-admins before the redirect | Each `useAdmin*` hook accepts `enabled`, gated on `status === "authenticated" && isAdmin`; the API remains the authority |
| In tests, "1.5 KB" and "booth@example.com" legitimately appear in two sections, and `en-MY` renders "1 Sept 2026" | Used `getAllByText`/`getAllByText(/Sept 2026/)` in assertions |

---

## 9. Known limitations / follow-ups

- No Playwright admin spec: promotion is SQL-only and the E2E proxy has no DB access. Go E2E covers the API; Vitest covers the UI.
- Promotion has no endpoint or UI (by design, Phase 9); run SQL as below. A promoted session is picked up on the next page load (refresh-on-mount); no re-login needed.
- There is no `GET /admin/users/{id}`, so the tables are flat lists with no detail view.
- `stats`/`health` are plain queries with a 60 s `staleTime` and no polling; refresh the page to re-read.
- The dev database has `test@gmail.com` promoted to admin from the manual smoke; it can be demoted with the inverse `UPDATE`.

---

## 10. How to try it

```text
# Infra + migrations + apps
docker compose up -d
goose -dir migrations postgres "$env:DATABASE_URL" up
$env:HTTP_ADDR=":18080"; go run ./cmd/api     # separate terminal
pnpm install
pnpm dev

# Promote yourself, then reload the web app
docker exec cpd_postgres psql -U cpd -d cpd -c "UPDATE users SET is_admin = TRUE WHERE email = '<you>';"

# Signed in:
#   Sidebar            -> Admin appears after Billing (ShieldCheck)
#   /admin             -> stats grid, queue health, operator + subscription tables
#   /admin (non-admin) -> redirect to /dashboard; no Admin entry in the sidebar
#   GET /account/me    -> {"data":{...,"isAdmin":true},"error":null}

# Gates
pnpm lint; pnpm typecheck; pnpm test; pnpm build
pnpm gen:api; git diff --exit-code packages/api-client/src/schema.d.ts   # stable after commit
gofmt -l .; go build ./...; go vet ./...; go vet -tags=integration ./...; go test ./...; go test -tags=integration ./...
```
