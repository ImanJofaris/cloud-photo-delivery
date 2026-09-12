# Cloud Photo Delivery

Cloud-based photo delivery SaaS for Malaysian photobooths. See `doc/` for the Product, Technical, and phased Implementation Plan.

## Documentation

- [AGENTS.md](AGENTS.md) — operating guide: commands, layering rules, environment gotchas.
- [Developer Onboarding](doc/Developer%20Onboarding.md) — set up and run the project, project map, troubleshooting.
- [Contributing](doc/Contributing.md) — workflow, layering rules, Definition of Done.
- [Implementation Plan](doc/Implementation%20Plan.md) — all phases and conventions.
- [Test Strategy](doc/Test%20Strategy.md) — unit / integration / E2E approach.
- Completion reports: [Phase 0](doc/phases/Phase%200%20-%20Report.md), [Phase 1](doc/phases/Phase%201%20-%20Report.md), [Phase 2](doc/phases/Phase%202%20-%20Report.md).

## Status

Phases 0–2 are **complete**: foundation, authentication & users, and events & settings. See the phase reports under `doc/phases/` for details. Next up: Phase 3 — Uploads & Object Storage.

## Stack

- Go 1.26+ (`net/http` + `chi` + `pgx`)
- PostgreSQL 16
- Cloudflare R2 (S3-compatible); MinIO locally
- Docker / docker-compose

## Quick start

```text
copy .env.example .env
make up            # start Postgres + MinIO + create bucket
make run           # run the API on :8080
```

Verify:

```text
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

Worker (placeholder until Phase 4):

```text
make worker
```

## Common commands

```text
make test                 # unit tests
make test-integration     # integration tests (requires Docker)
make cover                # coverage summary
make lint                 # go vet + golangci-lint (if installed)
make build                # build bin/api and bin/worker
make migrate-up           # apply migrations (requires goose)
make migrate-down         # roll back one migration
```

## Layout

```text
cmd/api        HTTP API entrypoint
cmd/worker     background worker entrypoint
internal/      domain + platform packages
pkg/           reusable libraries (httpx, database, r2)
migrations/    SQL migrations
api/           OpenAPI contract
```

## API conventions

All responses use the envelope:

```json
{ "data": {}, "error": null }
```

```json
{ "data": null, "error": { "code": "EVENT_NOT_FOUND", "message": "Event not found" } }
```
