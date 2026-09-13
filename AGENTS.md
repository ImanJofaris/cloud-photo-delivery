# AGENTS.md

Operating guide for this repository, for both AI agents and new engineers. Keep it short: it is an index into the authoritative docs, not a replacement for them.

**Cloud Photo Delivery** is a cloud photo delivery SaaS for Malaysian photobooth operators. The API is the product; every feature is designed and tested at the HTTP layer first. Stack: Go + `chi` + `pgx` + PostgreSQL, with Cloudflare R2 (MinIO locally).

## Authoritative docs (read on demand)

Do not preemptively load all of these. Read the ones relevant to the task.

- `doc/Developer Onboarding.md` — deep project map, setup, troubleshooting. **Read this first for any non-trivial task.**
- `doc/Contributing.md` — workflow, layering rules, Definition of Done. **Authoritative for how we work.**
- `doc/Test Strategy.md` — unit / integration / E2E rules. **Authoritative for testing.**
- `doc/Implementation Plan.md` — phase map and conventions.
- `doc/frontend/Conventions.md` — frontend stack, auth/session, API client, image/upload rules. **Authoritative for `apps/web` and `packages/*`.**
- `doc/phases/Phase N - <name>.md` — the plan for the phase being built. Frontend phases use the `F` prefix (`Phase F0`, `Phase F1`, ...).
- `doc/phases/Phase N - Report.md` — what was actually built (newest = most reliable).
- `doc/phases/_TEMPLATE - Phase Report.md` — report template.
- `api/openapi.yaml` — the API contract. Update it before implementing API changes.

## Commands

Prefer the exact gates below; they match CI and the phase reports.

```text
# build / run
go build ./...
go run ./cmd/api
go run ./cmd/worker

# verification gates (run in this order)
gofmt -l .                        # must print nothing
go build ./...
go vet ./...
go vet -tags=integration ./...
go test ./...
go test -tags=integration ./...   # needs Docker Desktop running

# migrations (goose is NOT installed by default)
go install github.com/pressly/goose/v3/cmd/goose@latest
goose -dir migrations postgres "$DATABASE_URL" up
goose -dir migrations postgres "$DATABASE_URL" down

# infrastructure
docker compose up -d
docker compose down

# web (apps/web) — pnpm via `corepack enable`
pnpm install
pnpm dev                          # http://localhost:3000, expects Go API on API_BASE_URL
pnpm gen:api                      # regenerate packages/api-client from api/openapi.yaml
pnpm lint
pnpm typecheck
pnpm test
pnpm build
pnpm test:e2e                     # Playwright, needs `pnpm --filter web exec playwright install`
pnpm shadcn add <component> -c apps/web   # shadcn CLI with Windows path fix (see gotchas)
```

`make` targets exist (`make test`, `make test-integration`, `make migrate-up`, ...) but `make` may not be installed. The `go` commands above are canonical.

## Architecture & layering (hard rules)

```text
handler   -> parse HTTP, call service, write envelope. No business logic.
service   -> business rules, validation, errors. No SQL, no net/http.
repository -> owns all SQL and pgx usage. No business rules.
```

- Services must be unit-testable with fakes: no DB, no network, no `net/http`.
- Depend on interfaces for external I/O (storage, clock, mailer, `PlanLimits`).
- Never import `net/http` inside a service.
- Every tenant-owned query is scoped: `WHERE id = $1 AND user_id = $2`. Add a test proving user B cannot access user A's data.
- Responses use the standard envelope `{"data": ..., "error": null}`. Do not change the middleware chain or envelope shape.
- Errors use `internal/platform/apperr` codes (`EVENT_NOT_FOUND`, `VALIDATION_ERROR`, `INVALID_STATUS_TRANSITION`, `PLAN_LIMIT_REACHED`, ...). Never leak SQL/driver errors or log secrets/tokens/signed URLs.
- Pagination for collections is cursor-based (`?cursor=&limit=`). Offset pagination is banned.
- No code comments unless they explain *why*.

Frontend hard rules (details in `doc/frontend/Conventions.md`):

```text
api-client -> generated types only; never hand-write API types.
browser    -> talks directly to the Go API with a Bearer access token (public routes unauth).
BFF        -> apps/web/src/app/api/auth/* only; refresh token in httpOnly cookie.
uploads    -> browser <-> R2 via presigned URLs; never proxy bytes.
images     -> no next/image optimization for signed URLs; fetch per visible photo.
```

## Repository map

```text
cmd/api            HTTP API entrypoint; NewRouter wires config, DB, routes
cmd/worker         background worker entrypoint
internal/auth      JWT + opaque refresh tokens; RequireAuth -> auth.UserID(ctx)
internal/users     operator accounts / profile
internal/events    event CRUD, slugs, status lifecycle, per-event settings
internal/platform  cross-cutting: config, apperr, logging, limits
pkg/httpx          response envelope, middleware, rate limiter
pkg/database       pgx pool helper
pkg/r2             object storage client (R2 + MinIO)
migrations         goose SQL migrations (-- +goose Up / -- +goose Down)
api/openapi.yaml   OpenAPI 3.1 contract
test/e2e           full HTTP flow tests (//go:build integration)
apps/web           Next.js 16 operator dashboard / public gallery (Phase F0+)
packages/ui        shared shadcn/ui primitives, hooks, globals.css
packages/api-client generated OpenAPI types + envelope-unwrapping fetch wrapper
doc/               specs, plan, phase docs + completion reports
```

## Phase workflow

1. Read the phase doc under `doc/phases/`.
2. Update `api/openapi.yaml` first if the API changes.
3. Implement service -> repository -> handler.
4. Test: unit always; integration where I/O exists; E2E for the phase's critical flow.
5. Run all verification gates above.
6. Write `doc/phases/Phase N - Report.md` from the template.
7. Update the phase doc status note and the Completion Reports list in `doc/Implementation Plan.md`.
8. Commit with a focused, scoped message (e.g. `feat(events): add cursor pagination`).

Do not start Phase N+1 until Phase N's Definition of Done is fully checked. Do not commit unless explicitly asked.

Frontend phases (F-prefixed) follow the same workflow; step 3 becomes UI/data work under `apps/web`, and their gates are `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build` (plus `pnpm test:e2e` for the phase's critical flow).

## Testing conventions

- Test files live beside the code: `service_test.go`, `handler_test.go`, `repository_integration_test.go`.
- Integration tests carry `//go:build integration` and use real Postgres/MinIO via Testcontainers.
- E2E tests live in `test/e2e/` (also guarded by `//go:build integration`).
- Hand-written fakes over mocks. Table-driven tests for validation-heavy code.
- No test order dependence; no shared mutable global state.
- Coverage floors: 80% on `internal/<domain>` services, 90% on `pkg/httpx` and auth.

## Environment gotchas (win32 / PowerShell 5.1)

- Port 8080 may be reserved by WinNAT/Hyper-V. Use `HTTP_ADDR=:18080` for live smoke tests.
- `pnpm` is not installed by default; run `corepack enable` once. If that fails with `EPERM` (Node in `C:\Program Files`), use `npm install -g pnpm`. Never add npm/yarn lockfiles.
- Next dev serves `apps/web` on `:3000` and reads `API_BASE_URL` (server-only) from `apps/web/.env.local`.
- shadcn CLI 4.20/4.21 truncates resolved paths when the project path contains a dot (e.g. `UF-Iman.Jofaris`) due to a Windows regex bug. Use `pnpm shadcn add <component> -c apps/web` (wrapper in `scripts/shadcn.cjs` applies `scripts/shadcn-path-fix.cjs`). The shadcn MCP server gets the same fix via `NODE_OPTIONS` in `opencode.json`.
- `-race` cannot run locally without gcc (install `mingw` or rely on CI, which runs `-race` on Linux).
- Docker Desktop must be running for integration tests. MinIO images come from `quay.io`.
- Use `curl.exe` (not `Invoke-WebRequest`) and pass JSON via temp files with `--data-binary "@file"` to avoid PowerShell quoting problems.
- PowerShell 5.1 does not support `&&`; chain with `;` or `cmd1; if ($?) { cmd2 }`.
- `$pid` is a read-only automatic variable — use a different name (`$apipid`).
- `cmd/api` reads configuration from environment variables via `os.Getenv`. The `-config` flag is currently ignored and `.env` is **not** auto-loaded; export the vars (or use a tool) before running.
- Long-running servers started with `Start-Process` from a captured shell can make that command hang until the servers exit (the shell waits on the process tree). Start them detached (e.g. WMI `Win32_Process.Create` around a `.cmd` wrapper that sets env vars and redirects logs), or keep them running across commands and only kill them explicitly.
- Integration tests each spin up their own Postgres container and create tables inline; they do not run goose. The migration down-path is covered by `migrations/migrations_integration_test.go`.

## Doc drift to be aware of

- `.github/workflows/ci.yml` and `go.mod` should track the same Go version. If they disagree, `go.mod` is the source of truth and CI should be bumped to match.
