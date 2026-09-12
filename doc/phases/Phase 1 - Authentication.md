# Phase 1 — Authentication & Users

**Goal:** Operators can register, log in, refresh, log out, reset password, and read/update their profile. Tenant identity is established for all later phases.

**Depends on:** Phase 0.

**Exit criteria:** Full auth lifecycle works over HTTP; every protected route rejects missing/invalid tokens; tenant ID is available in request context.

---

## 1. Features

- Email + password signup with strong password hashing.
- Login issuing access token (short-lived JWT) + refresh token (rotating, stored hashed).
- Refresh endpoint with rotation and reuse detection.
- Logout (revoke refresh token/session).
- Password reset request + confirm (token emailed; email sending stubbed behind an interface in Phase 1, real provider later).
- `GET /account/me`, `PATCH /account/me` (business name, profile fields).
- Auth middleware populating `userID` in context.
- Basic account lockout after repeated failed logins.
- Rate limiting on auth endpoints.

---

## 2. Database (migration `0002_auth.sql`)

Reuse/extend the spec's `users` table; add:

```sql
-- users: as per Technical Spec section 6
ALTER TABLE users ADD COLUMN email_verified_at TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN failed_login_count INT NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN locked_until TIMESTAMPTZ;

CREATE TABLE refresh_tokens (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_refresh_user ON refresh_tokens(user_id);

CREATE TABLE password_resets (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ
);
```

Store **only hashes** of refresh tokens and reset tokens.

---

## 3. API Surface

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

Example login response:

```json
{
  "data": {
    "accessToken": "...",
    "refreshToken": "...",
    "expiresIn": 900,
    "user": { "id": "uuid", "email": "a@b.com", "businessName": "Iman Booth" }
  },
  "error": null
}
```

---

## 4. Domain Layout

```text
internal/auth/
  service.go        # signup, login, refresh, logout, reset
  tokens.go         # JWT issue/verify, refresh hashing
  middleware.go     # bearer auth -> context userID
  handler.go
internal/users/
  service.go        # profile read/update
  repository.go
  handler.go
```

---

## 5. Security Rules

- Hash passwords with bcrypt (cost ≥ 12) or argon2id.
- Access token TTL ~15 min; refresh TTL ~30 days with rotation.
- Refresh reuse (already-rotated token) revokes the whole family.
- Constant-time comparisons for token hashes.
- Generic error on login failure (`INVALID_CREDENTIALS`) — never reveal whether email exists.
- Lockout: e.g. 5 failures → 15 min lock.
- Never log passwords, tokens, or hashes.

---

## 6. Test Plan

**Unit**
- [ ] signup: valid input creates user; weak password rejected; duplicate email rejected.
- [ ] login: correct password ok; wrong password `INVALID_CREDENTIALS`; locked account rejected.
- [ ] password hashing/verify round-trip; tokens not stored in plaintext.
- [ ] JWT: issue/verify, expiry, tampered signature rejected.
- [ ] refresh: valid rotates; reused/revoked token rejected.
- [ ] reset: expired/used token rejected; valid resets and invalidates sessions.
- [ ] handler: envelope + status codes for each branch.
- [ ] middleware: missing/malformed/expired bearer → 401; valid → context userID set.

**Integration**
- [ ] repository: unique email constraint; refresh token lookup by hash; CASCADE delete.
- [ ] lockout counter increments and resets correctly against real DB.
- [ ] reset token single-use.

**E2E**
- [ ] signup → login → call `/account/me` → refresh → logout → refresh now fails.

---

## 7. Definition of Done

- [ ] All master DoD items.
- [ ] Auth endpoints documented in OpenAPI.
- [ ] Auth middleware applied to a sample protected route and tested.
