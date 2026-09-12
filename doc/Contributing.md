# Contributing

This is the working agreement for the Cloud Photo Delivery backend. Read `doc/Developer Onboarding.md` first if you have not.

---

## 1. The phase workflow

We build in phases from `doc/Implementation Plan.md`. Each phase is planned before it is built and documented after it is built.

```text
1. Read the phase plan        doc/phases/Phase N - <name>.md
2. Update OpenAPI first       api/openapi.yaml (if the API changes)
3. Implement                  service -> repository -> handler
4. Test                       unit always, integration where I/O exists, E2E for the critical flow
5. Verify locally             make lint && make test && make test-integration
6. Write the report           doc/phases/Phase N - Report.md (use the template)
7. Update statuses            doc/Implementation Plan.md (if it tracks status), README if needed
8. Commit
```

Do not start Phase N+1 until Phase N's Definition of Done is fully checked.

---

## 2. Definition of Done (every phase)

- [ ] OpenAPI updated and reviewed (if the phase touches the API).
- [ ] Migrations added, reversible, and tested.
- [ ] Service layer unit tested with no DB and no network.
- [ ] Handlers tested with `httptest` against a fake service.
- [ ] Repositories integration tested against real Postgres.
- [ ] Tenant isolation test where the phase touches tenant data.
- [ ] `gofmt -l .` is clean.
- [ ] `go build ./...` and `go vet ./...` pass.
- [ ] `go vet -tags=integration ./...` passes.
- [ ] `go test ./...` and `go test -tags=integration ./...` pass.
- [ ] Phase acceptance criteria satisfied.
- [ ] Completion report written.

---

## 3. Layering rules

```text
handler  -> parses HTTP, calls service, writes envelope
service  -> business logic, owns rules and errors, no SQL, no net/http
repository -> owns all SQL and pgx usage
```

- Services must be unit-testable without a server or database. Depend on interfaces for external I/O (`ObjectStore`, clock, mailer, payment provider).
- Handlers contain no business logic.
- Repositories contain no business rules.
- Never import `net/http` inside a service.

---

## 4. Error handling

- Define domain errors with `apperr.New(code, message, status)` or the predefined helpers.
- Wrap causes with `.WithCause(err)`; the cause is for logs, not the client.
- Handlers call `httpx.Error(w, r, err)` and let it map to the envelope.
- Never return raw driver/SQL errors to clients.
- Use `UPPER_SNAKE_CASE` codes: `EVENT_NOT_FOUND`, `PLAN_LIMIT_REACHED`, `UPLOAD_SIZE_MISMATCH`.

---

## 5. Multi-tenancy

- Every tenant-owned query is scoped: `WHERE id = $1 AND user_id = $2`.
- There is always a test proving user B cannot access user A's data.
- New tenant-scoped tables get a `user_id` (or reach it through an owned parent) and an index.

---

## 6. Uploads & storage

- The API never proxies photo bytes. It issues presigned URLs only.
- All storage access goes through `pkg/r2.ObjectStore`. No direct SDK calls in services.
- Never log signed URLs, tokens, or object contents.
- Expensive work (resize, ZIP, purge) is a worker job, never inline in a request.

---

## 7. Testing rules

- Test files live beside the code: `service_test.go`, `handler_test.go`, `repository_integration_test.go`.
- Integration tests carry `//go:build integration`.
- Use table-driven tests for validation-heavy code.
- No test depends on execution order; no shared mutable global state.
- Unit tests never hit the network. Integration tests use Testcontainers (real Postgres/MinIO).
- Aim for 80%+ coverage on `internal/<domain>` service packages and 90% on `pkg/httpx` and auth.

See `doc/Test Strategy.md`.

---

## 8. Git & commits

Branch names:

```text
phase-0-foundation
phase-1-auth
feat/<short-description>
fix/<short-description>
```

Commit style (imperative, scoped):

```text
phase 0: scaffold API, config, httpx, storage, CI
feat(events): add cursor pagination
fix(uploads): handle idempotency key conflict
test(auth): cover refresh token reuse
docs(phase 0): add completion report
```

- Keep commits focused and self-contained.
- Never commit `.env`, secrets, or build artifacts (`bin/`, `coverage.out`).
- Do not commit to `main` when working with others.

---

## 9. Code style

- `gofmt` is mandatory; `goimports` groups are enforced by golangci-lint.
- Prefer the standard library. Add a dependency only with a clear reason.
- No comments unless they explain *why*, not *what*. Code should be self-explanatory.
- Keep functions small; return early.
- Use `context.Context` as the first argument for anything doing I/O.
- Structured logging only (`log/slog`), with keys: `request_id`, `user_id`, `event_id`, `duration_ms`.

---

## 10. Security checklist for any PR

- [ ] No secrets in code, logs, or tests.
- [ ] Auth required on non-public routes; tenant scoping enforced.
- [ ] Inputs validated (sizes, mime types, slugs, enums).
- [ ] No internal errors leaked in responses.
- [ ] Rate limits considered where the endpoint is costly or public.
- [ ] New dependencies checked (`govulncheck` runs in CI where available).
