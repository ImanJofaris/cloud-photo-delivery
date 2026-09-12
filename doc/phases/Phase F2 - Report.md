# Phase F2 — Events Dashboard — Completion Report

**Status:** COMPLETE
**Completed:** 2026-09-13
**Author:** opencode

---

## 1. Summary

Phase F2 delivered the operator event lifecycle in the browser: cursor-paginated search, create, detail with dashboard aggregates and gallery link sharing, gallery settings, and the archive/delete danger zone. Every call goes through the generated typed client with transparent 401 refresh. The phase also tightened the OpenAPI contract with `required` fields on response schemas so generated types stop treating always-present fields as optional.

---

## 2. Exit criteria — verification

| Criteria                                     | Result | Evidence                                                                                               |
| -------------------------------------------- | ------ | ------------------------------------------------------------------------------------------------------ |
| Create an event through the UI               | PASS   | `/events/new` form → `POST /events`; Playwright lifecycle spec (skips without API)                     |
| Search + cursor pagination                   | PASS   | `useEvents` `useInfiniteQuery` with `q`, `status`, `cursor`; "Load more" only when `nextCursor` exists |
| Edit event fields                            | PASS   | `useUpdateEvent` (PATCH) with cache update + list invalidation                                         |
| Configure gallery settings                   | PASS   | `settings-form.tsx` PATCHes visibility/password/downloads/watermark with cross-field validation        |
| View aggregates                              | PASS   | `/events/{id}` cards from `GET /events/{id}/dashboard`                                                 |
| Copy the gallery link                        | PASS   | `share-panel.tsx` copies `${origin}/e/{slug}` (never a signed URL)                                     |
| Archive and delete                           | PASS   | `danger-zone.tsx`; delete requires typing the event name                                               |
| Tenant isolation surfaces as not-found       | PASS   | `isNotFound()` renders an "Event not found" card for 404s, no error toast                              |
| No hand-written API types                    | PASS   | All types from `components["schemas"][...]`; `pnpm gen:api` drift-checked in CI                        |
| No offset pagination / full-collection fetch | PASS   | Only cursor-based `useInfiniteQuery`                                                                   |
| Lint / typecheck / unit tests / build        | PASS   | see section 7                                                                                          |
| Completion report                            | PASS   | this document                                                                                          |

---

## 3. What was built

### 3.1 Feature module (`apps/web/src/features/events/`)

- `api.ts`: `useEvents`, `useEvent`, `useEventSettings`, `useEventDashboard`, `useCreateEvent`, `useUpdateEvent`, `useUpdateEventSettings`, `useArchiveEvent`, `useDeleteEvent`, plus `toCreateEventRequest` / `toUpdateEventSettingsRequest` builders and `isNotFound`.
- `keys.ts`: query-key factory including filters.
- `schema.ts`: zod schemas for create and settings with cross-field validation (original downloads require downloads).
- `errors.ts`: maps `EVENT_NOT_FOUND`, `INVALID_STATUS_TRANSITION`, `PLAN_LIMIT_REACHED`, `VALIDATION_ERROR`.
- `format.ts`: `formatDate`, `formatDateTime` (Asia/Kuala_Lumpur), `formatBytes`.
- Components: `events-list.tsx` (deferred search input, status select, skeletons, empty state, Load more), `event-form.tsx`, `event-detail.tsx`, `settings-form.tsx` (RHF + Controller over Select/Switch), `share-panel.tsx`, `danger-zone.tsx`, `status-badge.tsx`.

### 3.2 Routes

- `(dashboard)/events/page.tsx` — list.
- `(dashboard)/events/new/page.tsx` — create.
- `(dashboard)/events/[eventID]/page.tsx` — async `params`, renders detail.
- Sidebar navigation now includes Events.

### 3.3 UI primitives added

`select`, `textarea`, `dialog`, `switch`, `badge` in `packages/ui`.

### 3.4 OpenAPI contract tightening

Response schemas the API always populates now declare `required` arrays: `Event`, `EventList`, `EventSettings`, `EventDashboard`, `EventWithSettings`, `AuthResponse`, `Profile`, `Error`. This is a documentation-only change — no handler behavior changed — and it removes optional-chaining noise from generated types. `packages/api-client/src/schema.d.ts` regenerated and committed.

---

## 4. Database changes

None. No migrations.

---

## 5. API surface added

No endpoints added or changed. Consumed: `GET/POST /api/v1/events`, `GET/PATCH/DELETE /api/v1/events/{eventID}`, `POST /api/v1/events/{eventID}/archive`, `GET/PATCH /api/v1/events/{eventID}/settings`, `GET /api/v1/events/{eventID}/dashboard`.

---

## 6. Files created / modified

```text
api/openapi.yaml                                      # required arrays
packages/api-client/src/schema.d.ts                   # regenerated
apps/web/src/app/(dashboard)/events/page.tsx
apps/web/src/app/(dashboard)/events/new/page.tsx
apps/web/src/app/(dashboard)/events/[eventID]/page.tsx
apps/web/src/components/app-sidebar.tsx
apps/web/src/features/events/api.ts
apps/web/src/features/events/danger-zone.tsx
apps/web/src/features/events/errors.ts
apps/web/src/features/events/event-detail.tsx
apps/web/src/features/events/event-form.tsx
apps/web/src/features/events/events-list.tsx
apps/web/src/features/events/format.ts
apps/web/src/features/events/format.test.ts
apps/web/src/features/events/keys.ts
apps/web/src/features/events/schema.ts
apps/web/src/features/events/schema.test.ts
apps/web/src/features/events/settings-form.tsx
apps/web/src/features/events/share-panel.tsx
apps/web/src/features/events/status-badge.tsx
apps/web/e2e/events.spec.ts
packages/ui/src/components/{select,textarea,dialog,switch,badge}.tsx
doc/phases/Phase F2 - Events Dashboard.md (status)
doc/phases/Phase F2 - Report.md
doc/Implementation Plan.md
```

---

## 7. Tests

### Unit

- `schema.test.ts` (8): name required/trimmed, client email validation, original-download cross-field rule, request builders omit empty fields and empty passwords.
- `format.test.ts` (4): bytes across units, date formatting and placeholders.

### E2E

- `events.spec.ts`: signup → create → search → open → save settings → archive → delete. Skips automatically when the Go API is unreachable.

### Verification commands and results

```text
pnpm lint        -> clean
pnpm typecheck   -> clean (api-client, ui, web)
pnpm test        -> 29 tests passed (6 api-client, 23 web)
pnpm build       -> succeeded; /events, /events/new, /events/[eventID] present
pnpm test:e2e    -> 2 passed, 2 skipped (Go API not running locally)
gofmt -l .       -> no output
go build ./...   -> clean
go vet ./...     -> clean
go test ./...    -> all packages ok
```

---

## 8. Issues found and fixed

| Issue                                                           | Fix                                                                                                           |
| --------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| Generated event types were all-optional, forcing defensive code | Added `required` arrays to always-populated response schemas in `api/openapi.yaml` and regenerated the client |
| `CalendarDaysPlus` does not exist in lucide 1.x                 | Used `CalendarDays`                                                                                           |
| React Compiler ESLint warning on `form.watch`                   | Switched to `useWatch({ control, name })`                                                                     |
| `setState` in effect for a mount guard (F1)                     | Removed in favor of CSS-driven icon switching                                                                 |

### Post-phase fix (2026-09-13)

Manual testing against the live API showed every direct browser call
(`/events`, `/events/{id}`, `/account/me`) returning 404: the client base URL
lacked the OpenAPI server path `/api/v1` (openapi-fetch does not read
`servers`). The browser client now uses `apiVersionedBaseUrl()`; the env vars
stay origin-only, and the live E2E specs load `.env.local` so a run against
the real API covers these calls.

---

## 9. Known limitations / follow-ups

- Event detail packs overview, settings, and danger zone on one page; tabs/virtualized photo grid arrive when photo management does.
- No photo counts update until Phases 3/4 populate `photo_count`; aggregates show real values that may be zero.
- The full lifecycle E2E needs a live Go API + Postgres; it skips otherwise.
- QR codes (Phase 7) will replace the plain share link with a QR asset.

---

## 10. How to try it

```text
# infrastructure + API
docker compose up -d
go run ./cmd/api

# frontend
pnpm install
# apps/web/.env.local:
# API_BASE_URL=http://localhost:8080
# NEXT_PUBLIC_API_BASE_URL=http://localhost:8080
pnpm dev
# sign up at http://localhost:3000/signup, then open /events
```
