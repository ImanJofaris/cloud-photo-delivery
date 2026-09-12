# Phase 1 — Authentication & Users — Completion Report

**Status:** COMPLETE
**Completed:** 12 September 2026
**Author:** Engineering

---

## 1. Summary

Phase 1 delivered the complete account lifecycle for photobooth operators: signup, login, token refresh with rotation and reuse detection, logout, password reset, and profile management. Every protected route now rejects missing or invalid tokens, and the authenticated user ID is established in request context for all later phases. Passwords are bcrypt-hashed, refresh/reset tokens are opaque and stored only as SHA-256 hashes, and auth endpoints are rate limited.

---

## 2. Exit criteria — verification

| Criteria | Result | Evidence |
|---|---|---|
| Full auth lifecycle works over HTTP | PASS | Live: signup 201, login 200, refresh 200, `/account/me` 200 |
| Protected routes reject missing/invalid tokens | PASS | Live `/account/me` -> 401; unit + E2E tests |
| Tenant ID available in request context | PASS | `auth.UserID(ctx)` set by `RequireAuth`, covered by middleware + E2E tests |
| `make test` passes | PASS | `go test ./...` all packages ok |
| Integration tests pass | PASS | `go test -tags=integration ./...` all packages ok |
| gofmt / build / vet clean | PASS | `gofmt -l .` clean; `go build`, `go vet`, `go vet -tags=integration` clean |
| No secrets logged | PASS | Log scan for JWT prefixes / `refreshToken` / passwords returned nothing |
| OpenAPI documented | PASS | `api/openapi.yaml` v0.2.0 with auth/account paths + `bearerAuth` |

---

## 3. What was built

### 3.1 Auth domain (`internal/auth/`)

| File | Responsibility |
|---|---|
| `password.go` | bcrypt (cost 12) hash/verify. |
| `tokens.go` | HS256 JWT issue/verify (issuer, exp, nbf); opaque token generate + SHA-256 hash; constant-time compare. |
| `mailer.go` | `Mailer` interface + `LogMailer` (dev/no-op; logs reset link only). |
| `repository.go` | Refresh token + password reset persistence (hash-only). |
| `service.go` | Signup, login, refresh (rotation + reuse detection), logout, reset request/confirm, token issuance. |
| `middleware.go` | `RequireAuth` bearer validation -> `auth.UserID(ctx)`. |
| `handler.go` | HTTP handlers with the standard envelope. |

### 3.2 Users domain (`internal/users/`)

| File | Responsibility |
|---|---|
| `model.go` | `User` entity + `IsLocked`. |
| `repository.go` | Tenant-owned/user CRUD, failed-login tracking, password update. |
| `service.go` | Profile read/update with validation. |
| `handler.go` | `GET`/`PATCH /account/me`. |

### 3.3 Shared

| File | Responsibility |
|---|---|
| `pkg/httpx/ratelimit.go` | In-memory token-bucket limiter + middleware + `ClientIP`. |
| `internal/platform/config` | Added `JWT_SECRET`, token TTLs, lockout settings, `PUBLIC_BASE_URL` + validation. |
| `cmd/api/main.go` | Wired auth/users services, middleware, rate-limited auth group, protected account group. |

---

## 4. Database changes

`migrations/0002_auth.sql` (goose up/down):

- `users` — id, email (unique), password_hash, business_name, email_verified_at, failed_login_count, locked_until, timestamps.
- `refresh_tokens` — id, user_id (cascade), family_id, token_hash, expires_at, revoked_at; indexes on user, hash, family.
- `password_resets` — id, user_id (cascade), token_hash, expires_at, used_at; index on hash.

Note: the phase plan referenced `ALTER TABLE users`, but Phase 0 created no users table, so Phase 1 creates it fresh. Rollback drops all three tables.

---

## 5. API surface added

```http
POST /api/v1/auth/signup
POST /api/v1/auth/login
POST /api/v1/auth/refresh
POST /api/v1/auth/logout
POST /api/v1/auth/password/reset-request
POST /api/v1/auth/password/reset-confirm

GET   /api/v1/account/me
PATCH /api/v1/account/me
```

Login response:

```json
{
  "data": {
    "accessToken": "eyJ...",
    "refreshToken": "tdqQyXKz...",
    "expiresIn": 900,
    "user": { "id": "uuid", "email": "a@b.com", "businessName": "Iman Booth" }
  },
  "error": null
}
```

---

## 6. Files created / modified

```text
migrations/0002_auth.sql

internal/auth/password.go
internal/auth/tokens.go
internal/auth/mailer.go
internal/auth/repository.go
internal/auth/service.go
internal/auth/middleware.go
internal/auth/handler.go
internal/auth/service_test.go
internal/auth/tokens_test.go
internal/auth/middleware_test.go
internal/auth/handler_test.go
internal/auth/service_integration_test.go

internal/users/model.go
internal/users/repository.go
internal/users/service.go
internal/users/handler.go
internal/users/service_test.go
internal/users/repository_integration_test.go

pkg/httpx/ratelimit.go
pkg/httpx/ratelimit_test.go

internal/platform/config/config.go   (modified)
internal/platform/config/config_test.go (modified)
cmd/api/main.go                      (modified)
cmd/api/main_test.go                 (modified)

api/openapi.yaml                     (rewritten, v0.2.0)
.env.example                         (modified)

test/e2e/auth_flow_test.go
```

---

## 7. Tests

### Unit
- password: hash/verify round-trip, wrong password rejected.
- tokens: JWT issue/verify, wrong secret, expired, `none` alg rejected; opaque token hash determinism + constant-time compare.
- service: signup success/invalid email/weak password/duplicate; login success/wrong password/unknown email/lockout; refresh rotation + reuse revokes family; logout; reset flow + single-use + session invalidation; unknown-email reset returns no error.
- middleware: missing/malformed/invalid bearer -> 401; valid -> userID in context.
- handlers: status + envelope per branch.
- users service: not found, update, name-too-long.
- httpx rate limiter: under/over limit, key isolation, middleware 429, `ClientIP`.
- config: JWT secret required, new defaults.

### Integration (`//go:build integration`)
- users repo: create + unique email, get by id/email, lockout counter increment/lock/reset, update name/password.
- auth flow: signup -> login -> refresh rotation -> reuse revokes family -> logout.
- lockout persisted to DB.
- password reset single-use + revokes all sessions.
- cascade delete of refresh tokens on user removal.

### E2E (`test/e2e`)
- signup -> login -> `/account/me` -> refresh -> logout -> refresh fails.
- protected route rejects bad/missing token.

### Verification commands and results

```text
gofmt -l .                          -> clean
go build ./...                      -> OK
go vet ./...                        -> OK
go vet -tags=integration ./...      -> OK
go test ./...                       -> all packages PASS
go test -tags=integration ./...     -> all packages PASS (incl. e2e)
```

Live smoke against Postgres + API binary: signup 201, `/account/me` 200, login 200, refresh 200; unauthenticated `/account/me` 401; logs contained no secrets.

---

## 8. Issues found and fixed

| Issue | Fix |
|---|---|
| Phase plan said `ALTER TABLE users` but no users table existed | Created `users` fresh in `0002_auth.sql`. |
| Existing config tests failed after adding required `JWT_SECRET` | Updated tests; added default/TTL assertions. |
| Unused `uuid` import in a test | Removed. |
| PowerShell quoting mangled JSON during live smoke | Used temp JSON files with `curl --data-binary`. |

---

## 9. Known limitations / follow-ups

- `LogMailer` only logs the reset URL; a real provider (Resend/SES) is added later.
- Rate limiter is per-instance (approximate); multi-instance accuracy revisited in Phase 10.
- No email verification flow yet (`email_verified_at` column is ready but unused).
- Google/Microsoft login deferred (product roadmap).
- `-race` not run locally (no gcc); CI runs it on Linux.

---

## 10. How to try it

```text
copy .env.example .env
make up
make migrate-up
make run

# signup
curl -X POST http://localhost:8080/api/v1/auth/signup \
  -H "Content-Type: application/json" \
  -d '{"email":"me@example.com","password":"password123","businessName":"My Booth"}'

# copy accessToken from the response, then
curl http://localhost:8080/api/v1/account/me -H "Authorization: Bearer <accessToken>"
```

If port 8080 is blocked on Windows, set `HTTP_ADDR=:18080` in `.env`.
