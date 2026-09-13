# Phase 6 — Photobooth Devices & API Keys — Completion Report

**Status:** COMPLETE
**Completed:** 2026-09-13
**Author:** opencode

---

## 1. Summary

Phase 6 delivers the photobooth authentication path: operators register devices, receive a `cpd_live_<prefix>_<secret>` API key exactly once, and the booth software uploads through the existing Phase 3 upload API with that key. Devices are scoped to a single assigned event, every key is stored as a SHA-256 hash and looked up by prefix, and rotation/revocation take effect immediately. Operators can list, rename, rotate, and revoke devices, with `last_used_at` telemetry and a 100/min device-scoped limit on upload initialization.

---

## 2. Exit criteria — verification

| Criteria | Result | Evidence |
|---|---|---|
| A device registered by an operator can authenticate with its API key | PASS | `internal/devices/service_test.go` (`Authenticate`), `middleware_test.go` (X-Api-Key and `Authorization: Device`), E2E `TestE2E_DeviceUploadScopeAndRotation` |
| Device can upload only to its assigned event | PASS | `TestInitialize_DeviceScopedToAssignedEvent`, `TestStatus_DeviceCannotReachForeignEventPhoto`; E2E device init against a foreign event returns 404 `EVENT_NOT_FOUND` |
| Foreign event access is rejected | PASS | Device without assignment denied; device assigned event A returns `EVENT_NOT_FOUND` for event B |
| Key rotation works | PASS | `TestRotate_OldKeyStopsWorking`, E2E: old key 401 `INVALID_DEVICE_KEY`, new key 201 |
| Key revocation works | PASS | `TestRevoke_TenantScoped`, E2E: revoked key 401 `DEVICE_REVOKED` |
| Operator CRUD: create, list, rename, rotate, revoke | PASS | `internal/devices/handler_test.go` (status + envelope) |
| `last_used_at` updates on success only | PASS | `TestAuthenticate_TouchesLastUsedOnSuccess`, repository integration `TestRepository_RevokeAndTouch`, E2E DB assertion |
| Device-scoped rate limiting | PASS | `TestRateLimit_OnlyThrottlesDevices` (device 429, operator unaffected); 100/min limiter wired in `main.go` |
| OpenAPI documents endpoints + API-key scheme | PASS | `api/openapi.yaml` v0.6.0: `/devices*`, `deviceKey` scheme, uploads accept `bearerAuth` or `deviceKey` |

---

## 3. What was built

### 3.1 Devices package (`internal/devices/`)

| File | Responsibility |
|---|---|
| `model.go` | `Device` struct (+ `Revoked()` helper) |
| `keys.go` | `GenerateKey` (6-byte hex prefix + 32-byte secret, base64url), `HashKey`, `KeyPrefix` parser, constant-time `VerifyKey` |
| `repository.go` | Tenant-scoped CRUD, global prefix lookup, `TouchLastUsed`, `EventOwnedBy` |
| `service.go` | Create/List/Rename/Rotate/Revoke business rules, assigned-event ownership check, `Authenticate` (hash verify + revoked check + best-effort touch) |
| `middleware.go` | `WithDevice`/`FromContext`, `RequireDevice`, `RequireDeviceOrOperator`, per-device `RateLimit` |
| `handler.go` | HTTP DTOs, envelope responses, input validation; raw key returned only on create/rotate |

### 3.2 Upload actor (`internal/uploads/`)

- `Actor{UserID, DeviceID, AssignedEvent}` replaces the bare `userID` in all service methods.
- `actorOwnsEvent` gives operators ownership checks and restricts devices to their assigned event; `photoForActor` applies the same rule to status/parts/complete/abort.
- Idempotency records are attributed to the owning operator for both auth modes.

### 3.3 Wiring (`cmd/api/main.go`)

- Device routes mounted under `RequireAuth` (`/devices*`).
- Upload routes moved to their own group using `RequireDeviceOrOperator(authSvc.RequireAuth)`; `POST /events/{eventID}/uploads` additionally applies the 100/min device limiter.

### 3.4 Contract

- `api/openapi.yaml` bumped to 0.6.0: `Devices` tag, `deviceKey` apiKey scheme (`X-Api-Key`, `Authorization: Device` noted), five device operations, Device schemas, and dual security on the six upload operations.
- `packages/api-client/src/schema.d.ts` regenerated via `pnpm gen:api`.

---

## 4. Database changes

```text
migrations/0007_devices.sql
```

- Creates `devices` (`user_id` → users cascade, `assigned_event_id` → events `ON DELETE SET NULL`, `revoked_at`, `last_used_at`).
- `idx_devices_user`, unique `idx_devices_prefix`.
- Down drops both indexes and the table; covered by `TestMigrations_DevicesUpDownRoundTrip`.

Note: the phase plan names the migration `0006_devices.sql`, but `0006` was taken by Phase 5; the actual migration is `0007_devices.sql`.

---

## 5. API surface added

```http
POST   /api/v1/devices
GET    /api/v1/devices
PATCH  /api/v1/devices/{deviceID}
POST   /api/v1/devices/{deviceID}/rotate
DELETE /api/v1/devices/{deviceID}
```

Create response (the only time the key is returned):

```json
{
  "data": {
    "device": {
      "id": "…", "name": "Booth 1", "keyPrefix": "a1b2c3d4e5f6",
      "assignedEventId": "…", "revokedAt": null, "lastUsedAt": null,
      "createdAt": "2026-09-13T01:00:00Z"
    },
    "key": "cpd_live_a1b2c3d4e5f6_…"
  },
  "error": null
}
```

Device-authenticated uploads (`X-Api-Key` or `Authorization: Device <key>`):

```http
POST /api/v1/events/{eventID}/uploads
GET  /api/v1/uploads/{photoID}
POST /api/v1/uploads/{photoID}/complete
POST /api/v1/uploads/{photoID}/parts
POST /api/v1/uploads/{photoID}/multipart/complete
POST /api/v1/uploads/{photoID}/multipart/abort
```

---

## 6. Files created / modified

```text
migrations/0007_devices.sql                                  (new)
migrations/migrations_integration_test.go                    (modified: devices up/down)

internal/devices/model.go                                    (new)
internal/devices/keys.go                                     (new)
internal/devices/repository.go                               (new)
internal/devices/service.go                                  (new)
internal/devices/middleware.go                               (new)
internal/devices/handler.go                                  (new)
internal/devices/fakes_test.go                               (new)
internal/devices/keys_test.go                                (new)
internal/devices/service_test.go                             (new)
internal/devices/middleware_test.go                          (new)
internal/devices/handler_test.go                             (new)
internal/devices/repository_integration_test.go              (new)

internal/uploads/service.go                                  (modified: Actor + scope rules)
internal/uploads/handler.go                                  (modified: actor resolver)
internal/uploads/service_test.go                             (modified: Actor args + scope tests)
internal/uploads/handler_test.go                             (modified: actor resolver)

cmd/api/main.go                                              (modified: devices wiring, upload auth group, limiter)
api/openapi.yaml                                             (modified: v0.6.0 device endpoints + scheme)
packages/api-client/src/schema.d.ts                          (regenerated)

test/e2e/uploads_flow_test.go                                (modified: devices schema + wiring)
test/e2e/devices_flow_test.go                                (new)

doc/phases/Phase 6 - Photobooth.md                           (status)
doc/phases/Phase 6 - Report.md                               (new)
doc/Implementation Plan.md                                   (completion report list)
```

---

## 7. Tests

### Unit

- **keys:** format/prefix parse round-trip, uniqueness over 50 generations, malformed keys rejected, constant-time verify false for wrong/empty, deterministic hash.
- **service:** name trim/limits, assigned-event ownership, list scoping, tenant-scoped rename/revoke, rotate invalidates old key, rotate rejected for revoked devices, auth with malformed/wrong-secret/unknown-prefix keys, touch only on success, touch failure tolerated, repository error mapping to `INTERNAL_ERROR`.
- **middleware:** X-Api-Key and `Authorization: Device`, invalid/revoked 401 codes, operator fallback, device-only middleware rejects JWT, 50 concurrent verifications, rate limit throttles devices only.
- **handler:** create/list/rename/rotate/revoke status + envelopes, invalid UUID 404, unknown event 404, unauthenticated 401.

### Integration (`//go:build integration`)

- **repository:** create/list round-trip, tenant isolation for get/rename/revoke/list, prefix lookup + rotate (old prefix gone), revoke + touch timestamps, cascade on user delete, `assigned_event_id` set null on event delete, event ownership incl. soft-deleted events.
- **migrations:** `0007` up/down round-trip (table + both indexes).
- **E2E:** signup → two events → create device → device initialize + MinIO PUT + complete → foreign event 404 → operator JWT still works → rotate (old 401, new 201) → revoke (401 `DEVICE_REVOKED`) → `last_used_at` persisted.

### Verification commands and results

```text
gofmt -l .                                              -> no output
go build ./...                                          -> clean
go vet ./...                                            -> clean
go vet -tags=integration ./...                          -> clean
go test ./...                                           -> all packages ok
go test -tags=integration -p 1 -timeout 30m -count=1 ./... -> all packages ok (e2e 66.8s)
go test -tags=integration -cover ./internal/devices/    -> 85.7% coverage
pnpm lint / typecheck / test / build                    -> clean (api-client 6, web 34 tests)
```

---

## 8. Issues found and fixed

| Issue | Fix |
|---|---|
| Phase doc names migration `0006_devices.sql`, already used by gallery | Shipped as `0007_devices.sql`; noted here and in the phase doc |
| `key_prefix` lookup needs to be unambiguous | Made `idx_devices_prefix` unique (plan sketched a non-unique index) |
| Device uploads hit Go's default 10m package timeout when running the full suite with `-p 1` on Windows | Re-ran with `-timeout 30m`; suite completes in ~67s — environment/pull latency, not a code defect |
| `Authenticate` returned a stale `last_used_at` after touching | Service now stamps the returned copy when the touch succeeds |

---

## 9. Known limitations / follow-ups

- `PATCH /devices/{deviceID}` only renames; re-assigning an event requires recreate/rotate. Add an `assignedEventId` patch if operators ask.
- `GET /devices` returns all devices unpaginated (device counts per operator are small; no offset pagination was introduced).
- The 100/min limiter applies to upload initialization; other device calls share the same key but are not throttled.
- Revoked devices remain listed with `revokedAt` for audit; there is no hard delete.
- No frontend device-management UI yet; Phase F-track can consume these endpoints. **Shipped in Phase F5** (`doc/phases/Phase F5 - Devices, Branding and Billing.md`).
- `last_used_at` is written synchronously per authenticated request; a queue/batch write is a Phase 10 candidate.

---

## 10. How to try it

```text
docker compose up -d
go run ./cmd/api        # HTTP_ADDR=:18080 if 8080 is reserved

# signup, then create an event
curl.exe -X POST http://localhost:18080/api/v1/auth/signup -H "Content-Type: application/json" --data-binary "@signup.json"
curl.exe -X POST http://localhost:18080/api/v1/events -H "Authorization: Bearer <access>" -H "Content-Type: application/json" --data-binary "@event.json"

# register a device (key is returned once)
curl.exe -X POST http://localhost:18080/api/v1/devices -H "Authorization: Bearer <access>" ^
  -H "Content-Type: application/json" --data-binary "@device.json"   # {"name":"Booth 1","assignedEventId":"<eventId>"}

# upload as the device
curl.exe -X POST http://localhost:18080/api/v1/events/<eventId>/uploads ^
  -H "X-Api-Key: cpd_live_..." -H "Content-Type: application/json" --data-binary "@init.json"
# PUT the file to the returned uploadUrl, then:
curl.exe -X POST http://localhost:18080/api/v1/uploads/<photoId>/complete -H "X-Api-Key: cpd_live_..."

# rotate: old key 401 INVALID_DEVICE_KEY, new key works
curl.exe -X POST http://localhost:18080/api/v1/devices/<deviceId>/rotate -H "Authorization: Bearer <access>"
# revoke: key returns 401 DEVICE_REVOKED
curl.exe -X DELETE http://localhost:18080/api/v1/devices/<deviceId> -H "Authorization: Bearer <access>"
```
