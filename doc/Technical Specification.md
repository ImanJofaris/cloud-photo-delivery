# Cloud Photo Delivery
## Technical Specification — MVP

### 1. Technical Goals

The system must be:

- Fast
- Highly concurrent
- Cheap to operate
- Horizontally scalable
- Reliable for large photo uploads
- Optimized for mobile gallery viewing
- Designed for large event libraries
- Multi-tenant
- Secure
- Cloud-native

### Performance Targets

| Operation | Target |
|---|---:|
| API response | <100 ms for normal CRUD |
| Gallery initial response | <300 ms |
| Photo metadata query | <100 ms |
| Upload initialization | <100 ms |
| Thumbnail availability | <5 sec after upload |
| Gallery image loading | <1 sec under normal conditions |
| Concurrent uploads | 100+ |
| Concurrent gallery users | 1,000+ |
| Event photos | 50,000+ |
| Single photo size | Up to 100 MB initially |

The application server should **never proxy photo data** unless there is a specific reason.

---

# 2. Recommended Architecture

```text
                         ┌─────────────────────┐
                         │      Browser        │
                         │   Photographer     │
                         │       Guest        │
                         └──────────┬──────────┘
                                    │
                                    ▼
                         ┌─────────────────────┐
                         │    Cloudflare       │
                         │ CDN / WAF / TLS     │
                         └──────────┬──────────┘
                                    │
                    ┌───────────────┴───────────────┐
                    │                               │
                    ▼                               ▼
             ┌─────────────┐                ┌─────────────┐
             │ Go API      │                │ Public CDN  │
             │             │                │             │
             │ REST API    │                │ Cached      │
             │ Auth        │                │ thumbnails  │
             │ Events      │                │ images      │
             └──────┬──────┘                └──────┬──────┘
                    │                              │
          ┌─────────┼─────────┐                    │
          │         │         │                    │
          ▼         ▼         ▼                    ▼
      PostgreSQL   Queue     R2 ◄────────────── Cloudflare
                              │
                              │
                       Original Photos
                       Optimized Images
                       Thumbnails
```

---

# 3. Technology Stack

## Backend

### Go

Recommended:

```text
Go 1.24+
```

Use:

```text
net/http
chi
pgx
context
encoding/json
log/slog
```

Avoid a huge framework.

A lightweight Go HTTP stack will keep the service simple and fast.

---

# 4. API Framework

Recommended:

```text
Go
  │
  ├── net/http
  │
  └── chi router
```

Example:

```text
/api/v1/auth
/api/v1/events
/api/v1/photos
/api/v1/uploads
/api/v1/gallery
/api/v1/account
```

Use middleware for:

```text
Request ID
Authentication
Rate limiting
Logging
Recovery
CORS
```

---

# 5. Database

## PostgreSQL

Use PostgreSQL as the primary relational database.

Do NOT use R2 as a database.

R2 stores:

```text
Binary files
```

PostgreSQL stores:

```text
Metadata
Relationships
Users
Events
Photos
Subscriptions
Permissions
```

---

# 6. Database Schema

## users

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY,
    email VARCHAR(320) NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    business_name VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

---

## events

```sql
CREATE TABLE events (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),

    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL,
    client_name VARCHAR(255),

    event_date DATE,
    status VARCHAR(30) NOT NULL,

    storage_bytes BIGINT NOT NULL DEFAULT 0,
    photo_count BIGINT NOT NULL DEFAULT 0,

    expires_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(user_id, slug)
);
```

---

## photos

```sql
CREATE TABLE photos (
    id UUID PRIMARY KEY,

    event_id UUID NOT NULL REFERENCES events(id),

    storage_key TEXT NOT NULL,
    thumbnail_key TEXT,
    optimized_key TEXT,

    original_filename TEXT,

    mime_type VARCHAR(100) NOT NULL,
    file_size BIGINT NOT NULL,

    width INT,
    height INT,

    status VARCHAR(30) NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

Indexes:

```sql
CREATE INDEX idx_photos_event_created
ON photos(event_id, created_at DESC);

CREATE INDEX idx_photos_event_status
ON photos(event_id, status);
```

---

# 7. Multi-Tenancy

Every resource must belong to a tenant/user.

```text
User
 │
 ├── Event A
 │    ├── Photo
 │    ├── Photo
 │    └── Photo
 │
 └── Event B
      ├── Photo
      └── Photo
```

Every query must enforce ownership.

Example:

```sql
SELECT *
FROM events
WHERE id = $1
AND user_id = $2;
```

Never:

```sql
SELECT *
FROM events
WHERE id = $1;
```

without authorization verification.

---

# 8. Cloudflare R2

Use R2 for object storage.

Example:

```text
tenant/{tenantID}/events/{eventID}/
```

Structure:

```text
tenant/
  123/
    events/
      456/
        originals/
          abc.jpg
          def.jpg

        optimized/
          abc.webp
          def.webp

        thumbnails/
          abc.webp
          def.webp
```

The database stores:

```text
storage_key
optimized_key
thumbnail_key
```

---

# 9. Direct Upload Architecture

This is critical for performance.

### DO NOT

```text
Photobooth
    ↓
Go API
    ↓
Go server receives 20 MB
    ↓
Go server uploads to R2
```

### DO

```text
Photobooth
    │
    ▼
Go API
    │
    │ Generate presigned URL
    ▼
Photobooth ───────────────► R2
                             │
                             ▼
                           Object
```

The Go API only handles:

```text
Authentication
Authorization
Upload initialization
Metadata
```

It does not carry the photo bytes.

---

# 10. Upload API

### Initialize upload

```http
POST /api/v1/events/{eventID}/uploads
```

Request:

```json
{
  "filename": "IMG_1234.jpg",
  "contentType": "image/jpeg",
  "size": 18273482
}
```

Response:

```json
{
  "photoId": "uuid",
  "uploadUrl": "...",
  "storageKey": "...",
  "expiresAt": "..."
}
```

Client uploads directly to R2.

---

# 11. Multipart Upload

For large files:

```text
<10 MB
    ↓
Normal upload

>10 MB
    ↓
Multipart upload
```

This allows:

- parallel upload
- retry individual parts
- resumable uploads
- better reliability

Example:

```text
100 MB photo

Part 1 ─┐
Part 2 ─┤
Part 3 ─┤──► R2
Part 4 ─┤
Part 5 ─┘
```

---

# 12. Upload Completion

After successful upload:

```http
POST /api/v1/uploads/{photoID}/complete
```

Backend:

```text
Verify object exists
        ↓
Update photo status
        ↓
Queue image processing
```

Status:

```text
UPLOADING
PROCESSING
READY
FAILED
```

---

# 13. Image Processing

Do not process images synchronously inside the HTTP request.

Bad:

```text
POST upload
 ↓
Resize 20 MB image
 ↓
Generate thumbnail
 ↓
Response
```

Good:

```text
Upload complete
       ↓
Queue job
       ↓
Worker
       ↓
Generate thumbnail
       ↓
Generate optimized image
       ↓
Update DB
```

---

# 14. Image Processing Worker

Worker responsibilities:

```text
Download original
       ↓
Validate image
       ↓
Read dimensions
       ↓
Generate thumbnail
       ↓
Generate optimized image
       ↓
Upload derivatives
       ↓
Update PostgreSQL
```

Recommended outputs:

```text
thumbnail: 400px
medium: 1000px
large: 2000px
original: unchanged
```

For the gallery:

```text
thumbnail → grid
large → full screen
original → download
```

---

# 15. Job Queue

Start simple.

### MVP

Use PostgreSQL-backed jobs or a lightweight queue.

Example:

```text
jobs
────
id
type
payload
status
attempts
run_at
created_at
```

Worker:

```text
Go Worker
   │
   ├── Poll jobs
   │
   ├── Process
   │
   └── Retry
```

Later, if scale requires it:

```text
Redis / Valkey
      or
Cloudflare Queues
```

Do not introduce Redis on day one unless you actually need it.

---

# 16. Gallery API

Public event:

```http
GET /api/v1/public/events/{slug}
```

Return:

```json
{
  "event": {
    "name": "Sarah & John's Wedding",
    "date": "2026-09-12"
  },
  "photos": {
    "total": 1284,
    "items": []
  }
}
```

Pagination:

```text
page
limit
cursor
```

Prefer **cursor pagination** for large galleries.

Example:

```http
GET /photos?cursor=abc123&limit=50
```

Avoid deep offset pagination:

```http
?page=1000
```

---

# 17. Photo Delivery

Guest requests:

```text
GET /photo/abc123
```

Backend generates:

```text
short-lived signed URL
```

Example:

```text
valid for 5 minutes
```

Then:

```text
Browser ─────► CDN ─────► R2
```

The Go API does not stream the image.

---

# 18. CDN Strategy

Use Cloudflare caching for:

```text
Thumbnails
Optimized images
Static assets
```

Example:

```text
Guest 1
   ↓
Cloudflare Edge
   ↓
R2

Guest 2
   ↓
Cloudflare Edge
   ↓
CACHE HIT
```

This is particularly useful when hundreds of guests are viewing the same event.

---

# 19. Gallery Performance

The gallery should never load 1,000 photos at once.

Use:

```text
50 photos
    ↓
lazy loading
    ↓
next 50
```

Frontend:

```text
IntersectionObserver
```

or equivalent virtualized gallery implementation.

Images should use:

```text
loading="lazy"
```

and responsive image sizes.

---

# 20. Thumbnail Strategy

A typical gallery might have:

```text
1,000 photos
```

Do not send:

```text
1,000 × 10 MB
=
10 GB
```

Send:

```text
1,000 × ~100 KB thumbnails
≈ 100 MB
```

When the guest opens a photo:

```text
Thumbnail
    ↓
Large optimized image
```

Only download the original when requested.

---

# 21. Authentication

Use access tokens/session cookies.

For API clients:

```text
Bearer token
```

For photobooth devices:

```text
API Key
```

Store only hashed API keys.

Never store:

```text
plaintext API key
```

---

# 22. Photobooth API Authentication

Each photobooth can have:

```text
Device ID
API Key
Event ID
```

Example:

```text
Photobooth
    │
    ├── Device ID
    ├── API Key
    └── Event ID
```

The device should only be able to upload to its assigned event.

---

# 23. Rate Limiting

Protect:

```text
Authentication
Upload initialization
Gallery API
Download generation
```

Example:

```text
Login:
10 requests/minute/IP

Upload initialization:
100 requests/minute/device

Public gallery:
high limit + CDN caching
```

---

# 24. Reliability

Uploads must survive:

- Network interruptions
- Photobooth restart
- R2 timeout
- API timeout

Implement:

```text
Retry
Exponential backoff
Idempotency keys
Multipart uploads
Upload status
```

Example:

```text
Upload
 ↓
Failed
 ↓
Retry
 ↓
Failed
 ↓
Retry
 ↓
Success
```

---

# 25. Idempotency

Photobooth software may accidentally retry the same request.

Use:

```http
Idempotency-Key: <unique-key>
```

The server must not create duplicate photos when the same request is retried.

---

# 26. Background Tasks

Use workers for:

```text
Image processing
ZIP generation
Event expiry
Storage cleanup
Email
Analytics aggregation
```

Never perform expensive work in the HTTP request.

---

# 27. Large ZIP Downloads

For:

```text
Download All
```

Do:

```text
Request ZIP
      ↓
Create job
      ↓
Worker
      ↓
Stream objects
      ↓
Generate ZIP
      ↓
Store ZIP in R2
      ↓
Signed download URL
```

Add automatic expiration:

```text
ZIP expires after 24 hours
```

---

# 28. Storage Accounting

Do not calculate storage by scanning R2 every time.

Maintain:

```text
events.storage_bytes
users.storage_bytes
```

When upload completes:

```text
storage_bytes += file_size
```

When deleted:

```text
storage_bytes -= file_size
```

Run periodic reconciliation:

```text
Database
    vs
R2
```

to detect inconsistencies.

---

# 29. API Response Format

Use consistent responses.

Success:

```json
{
  "data": {},
  "error": null
}
```

Error:

```json
{
  "data": null,
  "error": {
    "code": "EVENT_NOT_FOUND",
    "message": "Event not found"
  }
}
```

Do not expose internal errors.

Bad:

```text
SQL connection failed: pq: connection refused
```

Good:

```text
An unexpected error occurred.
```

---

# 30. Logging

Use structured logging:

```text
log/slog
```

Example:

```json
{
  "level": "info",
  "request_id": "abc123",
  "user_id": "xyz",
  "event_id": "123",
  "duration_ms": 42,
  "message": "photo upload initialized"
}
```

Never log:

```text
password
API keys
signed URLs
tokens
private photo URLs
```

---

# 31. Observability

Monitor:

```text
API latency
Error rate
Upload failures
R2 failures
Database latency
Queue depth
Image processing time
Storage usage
```

Important metrics:

```text
p50 latency
p95 latency
p99 latency
```

Don't optimize based purely on average latency.

---

# 32. Deployment

Recommended initial architecture:

```text
                Internet
                   │
                   ▼
              Cloudflare
                   │
                   ▼
             Go API servers
                │     │
                │     │
                ▼     ▼
          PostgreSQL   R2
                │
                ▼
             Go Worker
```

Run the Go API as stateless instances.

Therefore:

```text
Go API #1
Go API #2
Go API #3
```

can all handle requests.

No local file storage.

No in-memory session state.

---

# 33. Horizontal Scaling

If traffic increases:

```text
             Load Balancer
                   │
       ┌───────────┼───────────┐
       ▼           ▼           ▼
     Go #1       Go #2       Go #3
       │           │           │
       └───────────┼───────────┘
                   │
              PostgreSQL
```

Adding API servers should require **no application changes**.

---

# 34. Caching

Do not immediately add Redis everywhere.

Use:

### Cloudflare cache

For:

```text
Images
Thumbnails
Static files
```

### Application cache

Only when necessary:

```text
Event metadata
Subscription information
Frequently accessed public event data
```

Possible future:

```text
Valkey/Redis
```

---

# 35. Go vs Rust

## Go

Pros:

- Excellent concurrency
- Very fast compilation
- Simple deployment
- Small binaries
- Excellent HTTP performance
- Excellent networking
- Easy background workers
- Easier hiring
- Easier AI-assisted development
- Excellent for SaaS APIs

Cons:

- Less control over memory
- Not as fast as optimized Rust in some CPU-heavy workloads

## Rust

Pros:

- Extremely high performance
- Memory safety
- Excellent CPU efficiency
- Excellent for image/video processing infrastructure

Cons:

- More development complexity
- Longer development time
- Higher learning curve
- More complicated codebase
- Performance advantage probably won't matter for your MVP

### Recommendation

```text
                    Go
                     ⭐
                     │
          ┌──────────┼──────────┐
          │          │          │
        API        Workers    Uploads
          │          │          │
          └──────────┼──────────┘
                     │
                   R2
```

**Use Go for the entire backend initially.**

If you later identify a CPU-heavy component where Rust gives you a meaningful advantage, extract that component into a Rust service.

Don't start with microservices.

---

# 36. Recommended Project Structure

```text
cloud-photo/
│
├── cmd/
│   ├── api/
│   │   └── main.go
│   │
│   └── worker/
│       └── main.go
│
├── internal/
│   │
│   ├── auth/
│   ├── events/
│   ├── photos/
│   ├── uploads/
│   ├── gallery/
│   ├── storage/
│   ├── billing/
│   ├── jobs/
│   └── users/
│
├── pkg/
│   ├── r2/
│   ├── database/
│   └── httpx/
│
├── migrations/
│
├── api/
│   └── openapi.yaml
│
├── Dockerfile
├── docker-compose.yml
└── go.mod
```

Keep business logic separated from HTTP handlers.

---

# 37. Request Flow — Guest Gallery

```text
Guest
 │
 │ GET /e/abc123
 ▼
Cloudflare
 │
 ▼
Frontend
 │
 │ GET event metadata
 ▼
Go API
 │
 ▼
PostgreSQL
 │
 ▼
Photos metadata
 │
 ▼
Browser
 │
 │ request thumbnail
 ▼
Cloudflare CDN
 │
 ├── CACHE HIT → Browser
 │
 └── CACHE MISS
        ↓
       R2
        ↓
      Browser
```

---

# 38. Request Flow — Photobooth Upload

```text
Photobooth
 │
 │ POST /uploads
 ▼
Go API
 │
 ├── Authenticate device
 ├── Validate event
 ├── Validate file
 ├── Create photo record
 └── Generate signed upload
 │
 ▼
Photobooth
 │
 │ PUT
 ▼
R2
 │
 ▼
Upload Complete
 │
 ▼
Go Worker
 │
 ├── Generate thumbnail
 ├── Generate optimized image
 └── Update DB
 │
 ▼
READY
```

---

# 39. MVP Development Order

### Phase 1

Infrastructure:

```text
Go
PostgreSQL
R2
Cloudflare
Docker
```

### Phase 2

Backend:

```text
Authentication
Users
Events
Photos
```

### Phase 3

Upload:

```text
Presigned uploads
Multipart uploads
Upload status
Retries
```

### Phase 4

Image processing:

```text
Thumbnail
Optimized image
Queue
Worker
```

### Phase 5

Gallery:

```text
Public event
Photo grid
Fullscreen viewer
Download
```

### Phase 6

Photobooth:

```text
API keys
Device registration
Automatic uploads
Event linking
```

### Phase 7

QR:

```text
Generate QR
Download QR
Event URL
```

### Phase 8

Billing:

```text
Plans
Storage limits
Subscriptions
Payments
```

---

# 40. Performance Principles

The architecture should follow these rules:

### Rule 1

**Never proxy large files through Go.**

### Rule 2

**Never process images inside an HTTP request.**

### Rule 3

**Never load an entire event's photos in one API request.**

### Rule 4

**Never use original images for gallery thumbnails.**

### Rule 5

**Use CDN caching aggressively for public photos.**

### Rule 6

**Use cursor pagination.**

### Rule 7

**Make APIs stateless.**

### Rule 8

**Use background workers for expensive operations.**

### Rule 9

**Design for retries and idempotency.**

### Rule 10

**Measure p95/p99 before optimizing.**

---

# 41. Target MVP Scale

The initial system should comfortably support:

```text
1,000 photographers
10,000 events
10 million photos
10 TB storage
1,000 concurrent gallery users
100 concurrent uploads
```

without requiring a major architectural redesign.

The infrastructure can then scale horizontally.

---

# 42. Core Architecture Decision

The most important architectural decision is:

```text
             CONTROL PLANE
                   
             Go API
                │
        ┌───────┴────────┐
        │                │
   PostgreSQL          Queue
        │                │
        └───────┬────────┘
                │
                │
             DATA PLANE
                │
                ▼
           Cloudflare R2
                │
                ▼
         Cloudflare CDN
```

**Go controls the system. R2 carries the data. Cloudflare delivers the data.**

This keeps your application server lightweight even when an event suddenly has hundreds of guests downloading photos simultaneously.

---

# 43. Final Technology Recommendation

### MVP

```text
Language       Go
API            net/http + chi
Database       PostgreSQL
Storage        Cloudflare R2
CDN            Cloudflare
Workers        Go
Queue          PostgreSQL initially
Frontend       Next.js
Deployment     Docker
Authentication JWT/session
API format     REST + OpenAPI
```

### Do not start with

```text
Rust
Kubernetes
Microservices
Redis cluster
Kafka
Complex event architecture
Multiple databases
```

Build a **modular monolith in Go** first.

If you reach significant scale, then split out specific workloads.

The first thing I'd benchmark isn't Go vs Rust. I'd benchmark **the complete upload → R2 → processing → CDN → gallery pipeline**, because that is where the real performance of this SaaS will be determined.