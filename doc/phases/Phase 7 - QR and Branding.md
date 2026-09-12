# Phase 7 — QR Codes & Branding

**Goal:** Every event gets a QR code (PNG + SVG) pointing to its gallery URL. Operators can brand their account and galleries.

**Depends on:** Phase 2 (events), Phase 5 (public slug).

**Exit criteria:** Operator can download a scannable PNG and SVG for an event; branded gallery shows business name/logo/colors.

---

## 1. Features

- QR code generation from the event's public URL.
- Download QR as PNG (configurable size) and SVG.
- Stable event URL + copy.
- Embeddable QR image endpoint (for photobooth screens: `<img src=...>`).
- Branding settings: business name, logo, profile image, brand colors, contact info.
- Logo upload (reuses presigned upload flow).
- Apply tenant branding to public gallery responses/DTO.

---

## 2. Database (migration `0008_branding.sql`)

```sql
CREATE TABLE tenant_branding (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    business_name VARCHAR(255),
    logo_key TEXT,
    profile_image_key TEXT,
    primary_color VARCHAR(9),
    secondary_color VARCHAR(9),
    contact_email VARCHAR(320),
    contact_phone VARCHAR(40),
    website_url VARCHAR(320),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

QRs are generated on demand (cheap, cacheable) — no table needed.

---

## 3. API Surface

```http
GET  /api/v1/events/{eventID}/qr.png?size=512
GET  /api/v1/events/{eventID}/qr.svg
GET  /api/v1/events/{eventID}/url

GET   /api/v1/account/branding
PATCH /api/v1/account/branding
```

Public gallery DTO gains:

```json
{ "branding": { "businessName": "...", "logoUrl": "...", "primaryColor": "#..." } }
```

---

## 4. Domain Layout

```text
internal/qr/
  service.go        # build target URL, render PNG/SVG
  handler.go
internal/users/
  branding.go       # branding service + repository
```

---

## 5. Rules

- QR must encode the canonical public URL (`https://photos.example.com/e/{slug}`), never a signed URL.
- PNG `Content-Type: image/png`; SVG `image/svg+xml`; set cache headers.
- Only the owning tenant can fetch QR for an event (private events still get QRs, but scanning hits password gate).
- Colors validated as hex; ignore invalid values.
- Logo/profile images served via signed/CDN URLs.

---

## 6. Test Plan

**Unit**
- [ ] QR target URL built from slug + configured base URL.
- [ ] PNG/SVG rendering returns non-empty correct content types.
- [ ] color validation accepts valid hex, rejects invalid.
- [ ] event QRs require ownership.

**Integration**
- [ ] generated QR decodes back to the expected URL (use a QR decoder in the test).
- [ ] branding CRUD persists and is returned in public gallery DTO.

**E2E**
- [ ] create event → download QR PNG/SVG → decode and assert gallery URL → guest loads gallery and sees branding.

---

## 7. Definition of Done

- [ ] All master DoD items.
- [ ] OpenAPI documents QR + branding endpoints.
- [ ] Golden-file tests for SVG output (stable rendering).
