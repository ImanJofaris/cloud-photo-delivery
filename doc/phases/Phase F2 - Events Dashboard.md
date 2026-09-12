# Phase F2 — Events Dashboard

> **Status: COMPLETE.** See the [Phase F2 Report](Phase F2 - Report.md) for what was built and how it was verified.

**Goal:** Operators manage the full event lifecycle in the browser: browse/search their events, create and edit them, configure gallery settings, read dashboard aggregates, archive and delete, and copy the guest gallery link.

**Depends on:** Phase F1; Phase 2 (events API); Phase 5 (public gallery API) for the share link target.

**Exit criteria:** An operator can create an event, find it with search and cursor pagination, edit it and its settings, view aggregates, copy its gallery URL, archive it, and delete it — all through typed API calls, with tenant isolation surfacing as a correct not-found state.

---

## 1. Features

- `/events` list: search by name, filter by status, cursor pagination via `useInfiniteQuery`, empty and loading states.
- `/events/new`: create form (name, client name, event date, expiry, visibility, client-facing fields supported by the API).
- `/events/[eventID]` detail with tabs:
  - **Overview** — dashboard aggregates (`photo_count`, `storage_bytes`, `guest_count`, status), share panel with copy-link for `/e/{slug}`.
  - **Settings** — visibility (`public`/`password`/`private`), password, downloads, original download, watermark (`PATCH /events/{id}/settings`).
  - **Danger** — archive (`POST /events/{id}/archive`) and soft delete (`DELETE /events/{id}`) with confirmation dialogs.
- Status lifecycle actions respecting `INVALID_STATUS_TRANSITION` responses.
- Formatting helpers: bytes (GB), dates in `Asia/Kuala_Lumpur`, status badges.

---

## 2. API surface consumed

```http
GET    /api/v1/events?cursor=&limit=&status=&q=
POST   /api/v1/events
GET    /api/v1/events/{eventID}
PATCH  /api/v1/events/{eventID}
DELETE /api/v1/events/{eventID}
POST   /api/v1/events/{eventID}/archive
GET    /api/v1/events/{eventID}/settings
PATCH  /api/v1/events/{eventID}/settings
GET    /api/v1/events/{eventID}/dashboard
GET    /api/v1/account/me
```

No API or migration changes are expected in this phase. If a field is missing, the OpenAPI contract and Go implementation change first.

---

## 3. File map

```text
apps/web/src/app/(dashboard)/events/
  page.tsx                    # list
  new/page.tsx                # create
  [eventID]/page.tsx          # detail + tabs
  [eventID]/settings/page.tsx
  [eventID]/danger/page.tsx
apps/web/src/features/events/
  api.ts                      # query/mutation hooks
  keys.ts
  schema.ts                   # zod mirrors of API validation
  components/
    event-list.tsx
    event-form.tsx
    event-status-badge.tsx
    settings-form.tsx
    share-panel.tsx
    dashboard-stats.tsx
    delete-event-dialog.tsx
```

---

## 4. UX rules

- Cursor pagination via a "Load more" control or intersection sentinel; never render page numbers and never request all photos/photos-like collections at once.
- Optimistic archive; optimistic delete only after confirmation and with rollback on failure.
- 404 (`EVENT_NOT_FOUND`) renders a not-found state, not an error toast — it is also the tenant-isolation response for foreign ids.
- Destructive actions require typed confirmation of the event name for delete.
- The share panel copies the public URL built from the configured public base URL, never a signed URL.

---

## 5. Test Plan

**Unit (Vitest + RTL)**

- [ ] `schema.ts` matches API validation (name length, slug rules, password requirement for `password` visibility).
- [ ] event list renders items, empty state, and pagination trigger.
- [ ] settings form blocks invalid combinations (password visibility without a password; original download without downloads).
- [ ] error mapping: `INVALID_STATUS_TRANSITION`, `EVENT_NOT_FOUND`, `VALIDATION_ERROR`.

**E2E (Playwright)**

- [ ] create event → appears in list after search → open detail → update settings → copy link → archive → delete.
- [ ] second operator cannot open the first operator's event id (404 state).

**Gates**

```text
pnpm lint
pnpm typecheck
pnpm test
pnpm build
pnpm test:e2e        # against live Go API + Postgres
```

---

## 6. Definition of Done

- [ ] All master DoD items that apply.
- [ ] No hand-written types: every request/response comes from `packages/api-client`.
- [ ] Tenant isolation verified from the frontend (foreign event id renders not-found).
- [ ] No offset pagination, no full-collection fetches.
- [ ] AGENTS.md updated with the feature's commands if any were added.
- [ ] Completion report written.
