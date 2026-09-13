# Phase 7 — QR Codes & Branding — Completion Report

**Status:** COMPLETE
**Completed:** 2026-09-13
**Author:** opencode

---

## 1. Summary

Phase 7 adds the guest entry point and tenant identity: every event now has a canonical public URL (`<PUBLIC_BASE_URL>/e/{slug}`) and an owner-scoped QR code served as PNG (configurable size) or SVG. Operators configure branding — business name, logo, profile image, colors, and contact details — through `GET/PATCH /account/branding`, upload logo/image bytes straight to R2 through presigned PUT URLs, and the public gallery DTO now carries a nullable `branding` object matching the Phase F4 contract. Branding is fetched per tenant, so one operator's branding never leaks into another's gallery.

---

## 2. Exit criteria — verification

| Criteria | Result | Evidence |
|---|---|---|
| Operator can download a scannable PNG for an event | PASS | `TestService_PNG_DecodesBackToURL`, `TestHandler_PNG`, E2E `TestE2E_BrandingAndQRFlow` decodes the PNG back to the gallery URL with gozxing |
| Operator can download an SVG for an event | PASS | `TestService_SVG_MatchesGolden` (golden `internal/qr/testdata/event_qr.svg`), `TestHandler_SVG`, E2E asserts `image/svg+xml` |
| Branded gallery shows business name/logo/colors | PASS | `TestHandler_GetEvent_Branding`, E2E `GET /public/events/{slug}` returns `branding.businessName`/`primaryColor`/`logoUrl` |
| QR encodes the canonical public URL, never a signed URL | PASS | `TestService_EventURL_Canonical`; E2E `GET /events/{id}/url` equals the decoded PNG text |
| Only the owning tenant can fetch an event's QR | PASS | `TestHandler_OtherTenantGets404`; E2E: second tenant gets 404 for both `/qr.png` and `/url` |
| Colors validated as hex | PASS | `TestBrandingService_Update_Validation` (13 invalid cases), normalization to lowercase in `TestBrandingService_Update_NormalizesColor` |
| OpenAPI documents QR + branding endpoints | PASS | `api/openapi.yaml` v0.7.0: `/events/{eventID}/url`, `/qr.png`, `/qr.svg`, `/account/branding`, `/account/branding/assets`, `PublicBranding` schema |
| Public gallery DTO carries the F4 branding contract | PASS | `PublicEvent.branding` in OpenAPI; `TestHandler_ListPhotos_IncludesBranding`; E2E guest response |
| Golden-file test for SVG | PASS | `go test ./internal/qr/` compares byte-for-byte against `testdata/event_qr.svg` (deterministic `renderSVG`) |

---

## 3. What was built

### 3.1 QR package (`internal/qr/`)

| File | Responsibility |
|---|---|
| `service.go` | `EventLookup` interface, canonical URL building (`PUBLIC_BASE_URL` + `/e/{slug}`), PNG via `skip2/go-qrcode`, SVG via a single crisp-edges `<path>` per module row, size clamping 64–2048 (default 512) |
| `handler.go` | Owner-scoped `URL`/`PNG`/`SVG` handlers, `EVENT_NOT_FOUND` for foreign events, cache headers (`public, max-age=3600` for images, `private, max-age=60` for the URL) |

Ownership is enforced by the existing `events.Service.Get(ctx, userID, id)`, so invalid IDs, deleted events, and other tenants' events all return 404.

### 3.2 Branding (`internal/users/`)

| File | Responsibility |
|---|---|
| `branding.go` | `Branding` model, partial `BrandingInput`, `BrandingView` projection, `BrandingService` (Get/Update/CreateAssetUpload), validation for hex colors, email, website URL, key prefix |
| `branding_repository.go` | Tenant-scoped `GetBranding`/`UpsertBranding` (`INSERT ... ON CONFLICT (user_id) DO UPDATE`) |
| `branding_handler.go` | `GET/PATCH /account/branding` and `POST /account/branding/assets` DTOs and envelope responses |

Asset uploads reuse the existing `r2.ObjectStore.PresignPut`; keys are generated server-side as `tenant/{userID}/branding/{kind}/{uuid}.{ext}` and the PATCH rejects any key outside the caller's prefix. GET/PATCH responses replace stored keys with short-lived signed URLs so the operator UI never sees raw keys.

### 3.3 Gallery integration (`internal/gallery/`)

- `VisibleEvent` gained `Branding *users.BrandingView`; `BrandingProvider` is a one-method interface implemented by `BrandingService`.
- `GetEvent` and `ListPhotos` fetch branding for the event owner; missing branding returns an empty view (not an error).
- `publicEventDTO` renders `branding` as `null` when no field is set, otherwise all seven F4 fields (`businessName`, `logoUrl`, `primaryColor`, `secondaryColor`, `contactEmail`, `contactPhone`, `websiteUrl`).
- `logoUrl` is a presigned GET URL with `SIGNED_URL_TTL` (5 min default), which outlives the 60 s metadata cache — safe to embed directly; F4 needs no separate fetch.

### 3.4 Wiring and contract

- `cmd/api/main.go`: `BrandingService` built from `users.PostgresRepository` + `r2.ObjectStore`; QR service built from `events.Service`; routes mounted under `RequireAuth`.
- `api/openapi.yaml` bumped to 0.7.0; `packages/api-client/src/schema.d.ts` regenerated (`pnpm gen:api`, deterministic).
- `pkg/r2/keys.go`: `BrandingAssetKey` follows the existing tenant key layout.

---

## 4. Database changes

```text
migrations/0008_branding.sql
```

- Creates `tenant_branding` keyed by `user_id` (`ON DELETE CASCADE`), with `business_name`, `logo_key`, `profile_image_key`, `primary_color`, `secondary_color`, `contact_email`, `contact_phone`, `website_url`, `updated_at`.
- One row per tenant; upsert is the only write path.
- Down drops the table; covered by `TestMigrations_BrandingUpDownRoundTrip`.

---

## 5. API surface added

```http
GET   /api/v1/events/{eventID}/url
GET   /api/v1/events/{eventID}/qr.png?size=512
GET   /api/v1/events/{eventID}/qr.svg

GET   /api/v1/account/branding
PATCH /api/v1/account/branding
POST  /api/v1/account/branding/assets
```

Example — `PATCH /api/v1/account/branding` (partial; empty string clears):

```json
{
  "businessName": "Iman Booth",
  "primaryColor": "#AABBCC",
  "contactEmail": "hello@example.com",
  "logoKey": "tenant/<uid>/branding/logo/<uuid>.png"
}
```

Response:

```json
{
  "data": {
    "businessName": "Iman Booth",
    "logoUrl": "https://<r2>/tenant/<uid>/branding/logo/<uuid>.png?...",
    "profileImageUrl": null,
    "primaryColor": "#aabbcc",
    "secondaryColor": null,
    "contactEmail": "hello@example.com",
    "contactPhone": null,
    "websiteUrl": null,
    "updatedAt": "2026-09-13T05:40:00Z"
  },
  "error": null
}
```

Guest gallery (`GET /api/v1/public/events/{slug}`) now includes:

```json
"branding": {
  "businessName": "Iman Booth",
  "logoUrl": "https://<r2>/...",
  "primaryColor": "#aabbcc",
  "secondaryColor": null,
  "contactEmail": "hello@example.com",
  "contactPhone": null,
  "websiteUrl": null
}
```

---

## 6. Files created / modified

```text
migrations/0008_branding.sql
migrations/migrations_integration_test.go
pkg/r2/keys.go
internal/qr/service.go
internal/qr/handler.go
internal/qr/service_test.go
internal/qr/handler_test.go
internal/qr/testdata/event_qr.svg
internal/users/branding.go
internal/users/branding_repository.go
internal/users/branding_handler.go
internal/users/branding_test.go
internal/users/branding_handler_test.go
internal/users/branding_repository_integration_test.go
internal/gallery/model.go
internal/gallery/service.go
internal/gallery/handler.go
internal/gallery/service_test.go
internal/gallery/handler_test.go
internal/gallery/fakes_test.go
cmd/api/main.go
api/openapi.yaml
packages/api-client/src/schema.d.ts
test/e2e/gallery_flow_test.go
test/e2e/branding_flow_test.go
go.mod / go.sum
```

---

## 7. Tests

### Unit

- `internal/qr` (14 tests): canonical URL + trailing-slash trimming, ownership, PNG decodes to the URL, size clamping, SVG golden file, handler content types/cache headers/JSON/401/404, `parseSize`, `NormalizeSize`. Coverage 85.8%.
- `internal/users` branding: defaults when no row, signed URLs, partial merge, clearing, color normalization, 13 validation cases, repo/store error mapping, asset upload kinds/content types/key layout, handler envelopes. Branding functions 83–100% covered.
- `internal/gallery`: branding attached in `GetEvent`/`ListPhotos`, provider error → `INTERNAL_ERROR`, DTO null when unset, list response includes branding.

### Integration (`//go:build integration`)

- `internal/users/branding_repository_integration_test.go`: upsert/get round trip, partial update overwrite, missing row → `ErrNotFound`, tenant isolation across two users.
- `migrations`: `TestMigrations_BrandingUpDownRoundTrip`.
- E2E `TestE2E_BrandingAndQRFlow`: signup → create event → `/url` → decode PNG → SVG → unauthenticated 401 → foreign tenant 404 → brand PATCH → GET branding → presign logo → PATCH `logoKey` → guest gallery shows branding (cache header intact) → other tenant's gallery does not.

### Verification commands and results

```text
gofmt -l .                      -> no output
go build ./...                  -> ok
go vet ./...                    -> ok
go vet -tags=integration ./...  -> ok
go test ./...                   -> all packages ok (qr 14, gallery 49, users 24 branded tests)
go test -tags=integration ./... -> all packages ok (auth 69.8s, photos 125.5s, e2e 117.2s)
pnpm gen:api                    -> deterministic (sha256 A7F99113...8464)
pnpm lint                       -> ok
pnpm typecheck                  -> ok
pnpm test                       -> 6 + 53 tests passed
pnpm build                      -> ok
```

---

## 8. Issues found and fixed

| Issue | Fix |
|---|---|
| `skip2/go-qrcode` has no SVG renderer | Render SVG from `QRCode.Bitmap()` (quiet zone included) as one `<path>` of per-row runs; golden test locks the output |
| SVG had to be deterministic across runs | No map iteration or timestamps in `renderSVG`; golden test compares bytes |
| Gallery had no safe way to presign a logo inside the 60 s-cached metadata | `logoUrl` is signed with `SIGNED_URL_TTL` (default 5 min), longer than the cache window, so it can be embedded safely |
| `PATCH /account/branding` could reference another tenant's object | Service validates every key against `tenant/{userID}/branding/` and rejects `..` |
| Presigned PUTs cannot enforce size or bytes | Documented as a known limitation; keys are server-issued and scoped per tenant |

---

## 9. Known limitations / follow-ups

- Uploaded logo/profile bytes are not inspected (no magic-byte/content-length check on PUT); the key prefix and content-type at presign time are the only guarantees. Revisit if public abuse appears.
- `primaryColor`/`secondaryColor` accept only `#rrggbb` (alpha not supported) even though the column allows 9 chars.
- No cleanup job for orphaned previous logo objects when a tenant replaces their logo; Phase 9 lifecycle work is the natural home.
- `allowDownload=false` still blocks all photo variants in the public gallery — fixed by the Phase F4 §2.1 backend prerequisite, not this phase.
- No operator UI yet; QR/branding surfaces are API-only until the frontend phase for branding. **Shipped in Phase F5** (`doc/phases/Phase F5 - Devices, Branding and Billing.md`).
- QR images are bearer-authenticated, so an `<img src>` on a photobooth screen cannot embed the URL directly; the operator app must fetch the image with the access token and use a blob object URL.

---

## 10. How to try it

```text
docker compose up -d
goose -dir migrations postgres "$DATABASE_URL" up
go run ./cmd/api

# create an account and event, then:
curl.exe -H "Authorization: Bearer <access>" http://localhost:8080/api/v1/events/<eventID>/url
curl.exe -H "Authorization: Bearer <access>" -o qr.png "http://localhost:8080/api/v1/events/<eventID>/qr.png?size=512"
curl.exe -H "Authorization: Bearer <access>" -o qr.svg http://localhost:8080/api/v1/events/<eventID>/qr.svg

curl.exe -X PATCH -H "Authorization: Bearer <access>" -H "Content-Type: application/json" \
  --data-binary "@branding.json" http://localhost:8080/api/v1/account/branding

curl.exe http://localhost:8080/api/v1/public/events/<slug>
```
