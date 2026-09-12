# Test Strategy

Testing is part of every phase, not a later phase. This document defines the three layers, the tooling, and the rules.

---

## 1. Test Pyramid

```text
        ┌───────────────┐
        │   E2E (~5%)   │  Full HTTP flow against api + worker + Postgres + MinIO
        ├───────────────┤
        │ Integration   │  Real Postgres + MinIO (Testcontainers/docker-compose)
        │    (~25%)     │  Repositories, storage client, worker jobs
        ├───────────────┤
        │    Unit       │  Services, validators, helpers, handlers (mocked deps)
        │    (~70%)     │  Fast, no I/O, run on every save
        └───────────────┘
```

---

## 2. Tooling

| Purpose | Tool |
|---|---|
| Test framework | Go stdlib `testing` |
| Assertions | `stretchr/testify` (`require`, `assert`) |
| Mocks | Hand-written fakes first; `mockery` if interfaces grow |
| HTTP tests | `net/http/httptest` |
| DB integration | `testcontainers-go` (Postgres module) or docker-compose test DB |
| Object store integration | MinIO container; `pkg/r2` pointed at MinIO |
| Fixtures | `testdata/` files + `tt` table-driven cases |
| Coverage | `go test -coverprofile`; enforce per-package threshold in CI |
| Race detection | `go test -race ./...` |
| Load smoke | `k6` scripts (Phase 10) |
| Frontend unit | Vitest + Testing Library (jsdom) |
| Frontend E2E | Playwright against a live Go API |

Integration tests are guarded by build tags so unit runs stay fast:

```go
//go:build integration

package events_test
```

Run with:

```text
go test ./...                       # unit only
go test -tags=integration ./...     # unit + integration
go test -tags=e2e ./test/e2e/...    # full flows
```

---

## 3. What to Test at Each Layer

### Unit (per phase, mandatory)

- **Services:** all business rules, error branches, limit checks, ownership checks.
- **Validators:** event fields, file type/size, slug normalization, plan limits.
- **Handlers:** request parsing, JSON envelope shape, status codes, auth middleware — with a fake service.
- **Pure helpers:** cursor encode/decode, storage-key builder, error mapper, JWT claims.

Example (event service):

```go
func TestCreateEvent_RejectsWhenActiveEventLimitReached(t *testing.T) { ... }
func TestCreateEvent_GeneratesUniqueSlugWithinTenant(t *testing.T) { ... }
func TestGetEvent_NotFoundForOtherTenant(t *testing.T) { ... }
```

### Integration (per phase, mandatory where I/O exists)

- **Repositories:** real SQL against Postgres — CRUD, constraints, cascade, indexes used.
- **Tenant isolation:** create data as user A, attempt access as user B, assert 0 rows.
- **Storage client:** presign/put/get/delete round-trip against MinIO.
- **Worker jobs:** enqueue a job, run worker once, assert DB + storage side effects.
- **Migrations:** fresh DB migrates up and down cleanly.

Example (uploads repository):

```go
//go:build integration
func TestCompleteUpload_IsIdempotentOnRetry(t *testing.T) { ... }
```

### E2E (key flows only)

- **Signup → create event → presign → upload to MinIO → complete → worker processes → public gallery lists photo.**
- **Guest flow:** fetch public event by slug, paginate with cursor, get signed image URL, download succeeds.
- **Photobooth flow:** API key auth → scoped upload to assigned event → rejected for foreign event.
- **Billing flow (Phase 8):** subscribe → limits enforced → upgrade lifts limit.

E2E spins up the real `cmd/api` and `cmd/worker` against containerized Postgres + MinIO.

### Frontend (per FE phase)

- Vitest + Testing Library beside the code: rendering, interaction, form validation, and error-code mapping. Mock at the network boundary, never internal modules.
- Playwright covers the phase's critical browser flow (auth, event lifecycle, gallery, upload) against a running Go API.
- Business rules stay in Go tests; the frontend proves it renders and sends the right requests. Details: `doc/frontend/Conventions.md`.

---

## 4. Test Data & Isolation

- Each integration test gets a transaction rolled back after the test, or a unique schema.
- No test depends on execution order; no shared mutable global state.
- Fixtures use deterministic UUIDs and fixed clocks (`clock` interface) so cursor/time assertions are stable.
- No real network calls in unit tests. No real Cloudflare in any test — MinIO always.

---

## 5. Naming & Structure

- Test files live beside the code: `events/service_test.go`, `events/handler_test.go`.
- Integration tests: `events/repository_integration_test.go` with the build tag.
- E2E: `test/e2e/*_test.go`.
- Use table-driven tests for validation-heavy code.

---

## 6. CI Gates (GitHub Actions)

```text
1. go vet ./...
2. golangci-lint run
3. go test ./... -race -cover
4. go test -tags=integration ./... -race
5. go build ./cmd/api ./cmd/worker
6. Frontend (Phase F0+): pnpm lint, pnpm typecheck, pnpm test, pnpm build
```

A phase PR cannot merge unless gates pass and the phase's acceptance criteria are checked off.

---

## 7. Coverage Philosophy

- Aim for **high service-layer coverage** (business risk lives there).
- Do not chase 100% on generated code or `main.go`.
- Suggested floor: **80% on `internal/<domain>` service packages**, **90% on `pkg/httpx` and auth**.

---

## 8. Per-Phase Test Checklist Template

Each phase doc contains a "Test Plan" section. Use this shape:

```text
Unit
  [ ] service business rules
  [ ] validation + errors
  [ ] handler envelope/status
Integration
  [ ] repository CRUD + constraints
  [ ] tenant isolation
  [ ] external I/O (if any) vs MinIO/Postgres
E2E
  [ ] <the one critical user flow for this phase>
```
