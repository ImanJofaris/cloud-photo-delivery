# Cloud Photo Delivery

Cloud-based photo delivery SaaS for Malaysian photobooths. See `doc/` for the Product, Technical, and phased Implementation Plan.

## Documentation

- [AGENTS.md](AGENTS.md) — operating guide: commands, layering rules, environment gotchas.
- [Developer Onboarding](doc/Developer%20Onboarding.md) — set up and run the project, project map, troubleshooting.
- [Contributing](doc/Contributing.md) — workflow, layering rules, Definition of Done.
- [Implementation Plan](doc/Implementation%20Plan.md) — all phases and conventions.
- [Test Strategy](doc/Test%20Strategy.md) — unit / integration / E2E approach.
- [Deployment](doc/Deployment.md) — topology, release process, observability, backups, runbook.
- Completion reports: all phases under [doc/phases](doc/phases) — latest: [Phase 10](doc/phases/Phase%2010%20-%20Report.md).

## Status

All phases are **complete**: backend Phases 0–10 and frontend Phases F0–F5. The API covers auth, events, uploads, processing, public galleries, devices, branding, billing, lifecycle/ZIP/analytics/admin, and Phase 10 hardening (rate limits, security headers, Prometheus metrics, audit logs, backups, load scripts). See the phase reports under `doc/phases/` and the [Deployment guide](doc/Deployment.md).

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

Worker (processing, lifecycle, exports, reconciliation):

```text
make worker
```

Operational endpoints (internal only): API `http://localhost:9091/metrics`, worker `http://localhost:9092/metrics`.

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
