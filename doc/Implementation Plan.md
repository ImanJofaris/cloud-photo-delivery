# Cloud Photo Delivery — Phased Implementation Plan

This is the master plan. Each phase has its own detailed document under `doc/phases/`.

**Start here:** [Developer Onboarding](Developer Onboarding.md) | [Contributing](Contributing.md) | [Test Strategy](Test Strategy.md)

## Phase Plans

- [Phase 0 — Foundation & Scaffold](phases/Phase 0 - Foundation.md)
- [Phase 1 — Authentication & Users](phases/Phase 1 - Authentication.md)
- [Phase 2 — Events & Settings](phases/Phase 2 - Events.md)
- [Phase 3 — Uploads & Object Storage](phases/Phase 3 - Uploads.md)
- [Phase 4 — Image Processing Worker & Queue](phases/Phase 4 - Processing.md)
- [Phase 5 — Public Gallery & Delivery](phases/Phase 5 - Gallery.md)
- [Phase 6 — Photobooth Devices & API Keys](phases/Phase 6 - Photobooth.md)
- [Phase 7 — QR Codes & Branding](phases/Phase 7 - QR and Branding.md)
- [Phase 8 — Billing & Subscriptions](phases/Phase 8 - Billing.md)
- [Phase 9 — Lifecycle, ZIP, Analytics & Admin](phases/Phase 9 - Lifecycle and Admin.md)
- [Phase 10 — Hardening, Observability & Deployment](phases/Phase 10 - Hardening and Deploy.md)

### Frontend track (`apps/web`)

- [Phase F0 — Web Foundation](phases/Phase F0 - Web Foundation.md)
- [Phase F1 — Auth and Shell](phases/Phase F1 - Auth and Shell.md)
- [Phase F2 — Events Dashboard](phases/Phase F2 - Events Dashboard.md)
- [Phase F3 — Photo Uploads](phases/Phase F3 - Photo Uploads.md)
- [Phase F4 — Public Gallery](phases/Phase F4 - Public Gallery.md)

Frontend conventions: [`doc/frontend/Conventions.md`](frontend/Conventions.md).

Ordering: F3 shipped its own owner-facing photo API prerequisites (list, owner signed URLs, re-presign, delete) and is independent of Phase 7. Phase 7 (branding) landed before F4, whose §2.1 backend prerequisite fixed gallery view/download semantics (`doc/phases/Phase F4 - Public Gallery.md` §2). Remaining frontend surfaces (devices, QR/branding, billing) get phase docs when their backend phases land.

## Completion Reports

- [Phase 0 — Report](phases/Phase 0 - Report.md) — COMPLETE
- [Phase 1 — Report](phases/Phase 1 - Report.md) — COMPLETE
- [Phase 2 — Report](phases/Phase 2 - Report.md) — COMPLETE
- [Phase 3 — Report](phases/Phase 3 - Report.md) — COMPLETE
- [Phase 4 — Report](phases/Phase 4 - Report.md) — COMPLETE
- [Phase 4.1 — Report](phases/Phase 4.1 - Report.md) — COMPLETE (lossy WebP + bilinear resize)
- [Phase 5 — Report](phases/Phase 5 - Report.md) — COMPLETE (public gallery API; UI deferred)
- [Phase 6 — Report](phases/Phase 6 - Report.md) — COMPLETE (devices & API keys)
- [Phase 7 — Report](phases/Phase 7 - Report.md) — COMPLETE (QR codes & branding)
- [Phase F0 — Report](phases/Phase F0 - Report.md) — COMPLETE (web foundation)
- [Phase F1 — Report](phases/Phase F1 - Report.md) — COMPLETE (BFF-lite auth + shell)
- [Phase F2 — Report](phases/Phase F2 - Report.md) — COMPLETE (events dashboard)
- [Phase F3 — Report](phases/Phase F3 - Report.md) — COMPLETE (photo uploads & library)
- [Phase F4 — Report](phases/Phase F4 - Report.md) — COMPLETE (public gallery)

Report template: [`phases/_TEMPLATE - Phase Report.md`](phases/_TEMPLATE - Phase Report.md)

---

## 1. Guiding Principles

1. **API-first.** Every feature is designed, specified, and tested at the HTTP/API layer before any UI is built. OpenAPI is the contract.
2. **Modular monolith in Go.** One deployable API binary and one worker binary. No microservices, no Redis/Kafka on day one.
3. **Never proxy bytes.** The Go API only issues presigned URLs. Photo data flows client ⇄ R2 directly.
4. **Never process in the request path.** Image processing, ZIP generation, expiry, and emails are worker jobs.
5. **Multi-tenant by default.** Every query is scoped by `user_id` / tenant. Enforced in the repository layer and tested.
6. **Test each phase before moving on.** A phase is "done" only when its unit + integration tests pass and its acceptance criteria are met.
7. **Build for scale targets** from the Technical Spec (10M photos, 10TB, 1,000 concurrent viewers, 100 concurrent uploads) without redesign.

---

## 2. Confirmed Stack

| Layer | Choice | Notes |
|---|---|---|
| Language | Go 1.26+ | Installed and verified |
| HTTP router | `chi` | `net/http` stdlib elsewhere |
| DB driver | `pgx/v5` | Connection pool via `pgxpool` |
| Database | PostgreSQL 16 | Via Docker locally |
| Migrations | `golang-migrate` or `goose` | Plain SQL files in `migrations/` |
| Object storage | Cloudflare R2 | S3-compatible API |
| Local object store | MinIO | For dev + integration tests |
| Queue | PostgreSQL-backed jobs table | No Redis on day one |
| Image processing | `gen2brain/vpx` (pure-Go libwebp port) | Lossy VP8 + lossless VP8L, CGO-free; swap for `libvips` if needed |
| Auth | JWT access + refresh, bcrypt/argon2 | Bearer for API, API keys for devices |
| QR | `skip2/go-qrcode` | PNG + SVG output |
| API contract | OpenAPI 3.1 | `api/openapi.yaml`, code-reviewed first |
| Frontend | Next.js 16 App Router + pnpm workspace | `apps/web` + `packages/ui` + generated `packages/api-client`; consumes the API only |
| Deployment | Docker + docker-compose | Stateless API instances |
| CI | GitHub Actions | lint, vet, test, build |

### Tooling to install locally

```text
Go 1.26+            (installed)
Docker Desktop      (installed)
Node 24+            (installed; frontend in apps/web)
pnpm                corepack enable
golangci-lint       go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
goose OR migrate    go install github.com/pressly/goose/v3/cmd/goose@latest
mockery (optional)  go install github.com/vektra/mockery/v2@latest
```

---

## 3. Phase Map

| Phase | Focus | Primary Deliverable | Test Gate |
|---|---|---|---|
| 0 | Foundation & Scaffold | Runnable API skeleton, Docker, CI, migrations runner | `go test ./...` green; health endpoint tested |
| 1 | Authentication & Users | Signup/login/refresh/reset, tenant identity | Unit + integration auth tests |
| 2 | Events & Settings | Event CRUD, slug, settings, ownership | Unit + integration + tenant-isolation tests |
| 3 | Uploads & Object Storage | Presign, multipart, complete, idempotency | Unit + integration (MinIO) upload tests |
| 4 | Processing Worker & Queue | Jobs table, thumbnail/optimized generation | Unit + integration worker tests |
| 5 | Public Gallery & Delivery | Public event API, cursor pagination, signed URLs | Unit + integration + E2E gallery test |
| 6 | Photobooth Devices & API Keys | Device registration, scoped API keys | Unit + integration device-auth tests |
| 7 | QR Codes & Branding | QR PNG/SVG, event URL, brand settings | Unit + golden-file tests |
| 8 | Billing & Subscriptions | Plans, limits enforcement, subscription lifecycle | Unit + integration billing tests |
| 9 | Lifecycle, ZIP, Analytics & Admin | Expiry, soft delete, ZIP jobs, analytics, admin API | Unit + integration + job tests |
| 10 | Hardening, Observability & Deploy | Rate limit, logging, metrics, production deploy | Load smoke test + full suite green |

Phases 0–5 form the **MVP critical path**. Phases 6–10 make it a sellable SaaS.

---

## 4. Target Repository Layout (before, then after Phase 0)

```text
cloud-photo-delivery/
├── cmd/
│   ├── api/main.go
│   └── worker/main.go
├── internal/
│   ├── auth/
│   ├── users/
│   ├── events/
│   ├── photos/
│   ├── uploads/
│   ├── gallery/
│   ├── storage/
│   ├── billing/
│   ├── jobs/
│   ├── qr/
│   └── platform/        # config, logging, errors
├── pkg/
│   ├── r2/              # object storage client
│   ├── database/        # pgx pool, tx helper
│   └── httpx/           # JSON envelope, middleware, errors
├── migrations/
├── api/openapi.yaml
├── apps/
│   └── web/             # Next.js 16 frontend (Phase F0+)
├── packages/
│   ├── ui/              # shared shadcn/ui components
│   └── api-client/      # generated OpenAPI types + typed fetch wrapper
├── test/
│   ├── integration/
│   └── e2e/
├── testdata/
├── Dockerfile
├── docker-compose.yml
├── Makefile
└── go.mod
```

Business logic lives in `internal/<domain>` service structs, separate from HTTP handlers, so it is unit-testable without a server.

---

## 5. Standard API Conventions (locked in Phase 0)

**Base path:** `/api/v1`

**Response envelope**

```json
{ "data": {}, "error": null }
```

```json
{ "data": null, "error": { "code": "EVENT_NOT_FOUND", "message": "Event not found" } }
```

**Route groups**

```text
/api/v1/auth/*
/api/v1/account/*
/api/v1/events/*
/api/v1/events/{eventID}/uploads
/api/v1/uploads/{photoID}/complete
/api/v1/devices/*
/api/v1/public/*          # no auth, public gallery
/api/v1/admin/*           # admin only
```

**Middleware order:** Request ID → Recovery → Logging → CORS → Rate Limit → Auth.

**Pagination:** cursor-based (`?cursor=&limit=`). Offset pagination is banned for photos.

**Idempotency:** all mutating upload endpoints honor `Idempotency-Key`.

**Errors:** typed domain errors mapped to codes + HTTP status in `httpx`. Never leak SQL/driver errors.

---

## 6. Definition of Done (every phase)

- [ ] OpenAPI updated and reviewed before implementation.
- [ ] Migrations added and reversible.
- [ ] Service layer unit tested (no DB, no network).
- [ ] Handler tested with `httptest` + mocked service.
- [ ] Repository integration tested against real Postgres.
- [ ] Ownership/isolation test where the phase touches tenant data.
- [ ] `golangci-lint run` and `go vet ./...` clean.
- [ ] `go test ./... -race` green.
- [ ] Phase acceptance criteria (in the phase doc) satisfied.

---

## 7. Risks & Decision Log

| Risk | Mitigation |
|---|---|
| R2 signing quirks vs S3 | Integration tests run against MinIO; a thin `pkg/r2` abstraction allows swapping. |
| Large-file multipart complexity | Phase 3 isolates it; ship simple upload first, add multipart behind the same contract. |
| Image pipeline CPU cost | Choose `imaging` for MVP, containerize so `libvips` can replace it later. |
| WebP derivative size/egress | Phase 4.1 switched to lossy VP8 via `gen2brain/vpx` (CGO-free) with per-derivative quality 80/82/85; ~4.3× smaller than lossless on worst-case data. |
| Payment provider for Malaysia | Billing (Phase 8) starts with a provider-agnostic `PaymentProvider` interface (Billplz/Stripe adapters). |
| Scope creep | Section 26 of the Product Spec is a hard "do not build" list for MVP. |
