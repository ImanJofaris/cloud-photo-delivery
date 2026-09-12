# Phase 6 — Photobooth Devices & API Keys

**Goal:** Photobooth software authenticates with API keys, is scoped to an assigned event, and uploads automatically through the Phase 3 upload API. This is the core differentiator.

**Depends on:** Phase 3 (uploads). Reuses Phase 1 auth concepts.

**Exit criteria:** A device registered by an operator can authenticate with its API key and upload only to its assigned event; foreign event access is rejected; key rotation/revocation works.

---

## 1. Features

- Operator creates a device (name, optional assigned event).
- API key generated and shown **once**; only its hash stored.
- Device list, rename, rotate key, revoke.
- Device authentication middleware for API clients (`Authorization: Device <key>` or `X-Api-Key`).
- Scoped uploads: device can only initialize uploads for its assigned event(s).
- `last_used_at` tracking.
- Device-scoped rate limiting (100 upload inits/min).

---

## 2. Database (migration `0006_devices.sql`)

```sql
CREATE TABLE devices (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(120) NOT NULL,
    key_prefix VARCHAR(12) NOT NULL,
    key_hash TEXT NOT NULL,
    assigned_event_id UUID REFERENCES events(id) ON DELETE SET NULL,
    revoked_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_devices_user ON devices(user_id);
CREATE INDEX idx_devices_prefix ON devices(key_prefix);
```

Key format (prefix allows lookup without scanning):

```text
cpd_live_<prefix>_<secret>
```

---

## 3. API Surface

```http
POST   /api/v1/devices
GET    /api/v1/devices
PATCH  /api/v1/devices/{deviceID}
POST   /api/v1/devices/{deviceID}/rotate
DELETE /api/v1/devices/{deviceID}          # revoke
```

Device-authenticated (API-key, not JWT):

```http
POST /api/v1/events/{eventID}/uploads
```

The upload handler accepts either an operator JWT (any owned event) or a device key (assigned event only).

---

## 4. Domain Layout

```text
internal/devices/
  service.go        # create, rotate, revoke, verify key
  repository.go
  middleware.go     # device auth
  handler.go
```

---

## 5. Security Rules

- Generate keys with a CSPRNG (≥ 32 bytes entropy).
- Store hash only (e.g. SHA-256 of secret, or argon2 if slow-hash desired).
- Lookup by `key_prefix`, then constant-time verify hash.
- Revoked device → `401 DEVICE_REVOKED`.
- Device cannot access resources outside its assigned event.
- Never log the raw key; return it only in the create/rotate response.
- Rotation invalidates the previous key immediately.

---

## 6. Test Plan

**Unit**
- [ ] key generation format + entropy; only hash persisted.
- [ ] verify: correct key ok; wrong key fails; revoked device fails.
- [ ] rotate: old key stops working, new key works.
- [ ] scope: device assigned event A rejected for event B.
- [ ] handler envelope/status.

**Integration**
- [ ] device repository CRUD; cascade when user deleted; `assigned_event_id` set null on event delete.
- [ ] `last_used_at` updates on success, not on failure.
- [ ] middleware under concurrent requests verifies correctly.

**E2E**
- [ ] operator creates device → device initializes + completes upload to assigned event → foreign event rejected.
- [ ] rotate key → old key 401, new key 200.

---

## 7. Definition of Done

- [ ] All master DoD items.
- [ ] OpenAPI documents device endpoints + the API-key auth scheme.
- [ ] Upload handler supports both auth modes with tests for each.
