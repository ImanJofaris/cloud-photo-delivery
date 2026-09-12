# Cloud Photo Delivery for Malaysian Photobooths

## 1. Product Overview

### Product Name
**Cloud Photo Delivery**

A cloud-based SaaS for Malaysian photobooth operators to automatically upload, store, organize, and deliver event photos to guests through a branded web gallery and QR code.

### Core Problem

Photobooth operators commonly need to:

- Store thousands of photos per event
- Deliver photos immediately to guests
- Share photos through QR codes
- Keep photos available after the event
- Avoid managing local storage manually
- Avoid expensive cloud storage/bandwidth costs
- Provide a professional experience to clients

### Product Goal

Provide a simple workflow:

**Take photo → Upload to cloud → Guest scans QR → View/download photo**

The operator should not need to manually upload photos to Google Photos, Google Drive, Dropbox, etc.

---

# 2. Target Users

## Primary User

### Photobooth Operator

Examples:

- Wedding photobooth businesses
- Birthday/event photobooths
- Corporate event photobooths
- 360° photobooth operators
- Instant-print photobooth businesses

## Secondary User

### Event Guest

Guests do not need an account.

They access photos through:

- QR code
- Event URL
- Shared photo link

## Future User

### Photographer

The platform can later support professional photographers with larger galleries and client delivery.

---

# 3. Core Concept

Everything revolves around an **Event**.

Example:

```text
Event
└── Sarah & John's Wedding
    ├── Photos
    ├── QR Code
    ├── Gallery
    └── Settings
```

Each event gets a unique URL:

```text
photos.example.com/e/sarah-john-wedding
```

And a QR code pointing to the same URL.

---

# 4. MVP Features

## 4.1 Authentication

Photobooth operators can create an account.

### Features

- Email/password login
- Password reset
- Logout
- Basic account profile

Future:

- Google login
- Microsoft login

---

# 5. Event Management

Operator can create and manage events.

### Create Event

Fields:

```text
Event Name
Event Date
Client Name
Client Email
Location
Event Description
```

Example:

```text
Event Name: Sarah & John's Wedding
Date: 12 September 2026
Client: Sarah Tan
Location: Kuala Lumpur
```

### Event Status

```text
Upcoming
Active
Completed
Archived
```

### Event Dashboard

Display:

```text
Sarah & John's Wedding

12 September 2026

Photos
1,284

Storage
8.4 GB

Guests
342

[Open Gallery]
[Upload]
[QR Code]
[Settings]
```

---

# 6. Photo Upload

This is one of the most important features.

The application should support **direct-to-object-storage uploads**.

The backend should NOT proxy the entire photo through the application server.

Architecture:

```text
Photobooth
     │
     │ Request upload URL
     ▼
Cloud Photo API
     │
     │ Presigned URL
     ▼
Photobooth ───────────► Object Storage
                         │
                         ▼
                        Photo
```

## Requirements

- JPG/JPEG
- PNG
- WEBP
- HEIC (future)
- Large files
- Multiple simultaneous uploads
- Upload progress
- Retry failed uploads
- Resumable/multipart uploads for large files

### Upload States

```text
Uploading
Processing
Ready
Failed
```

---

# 7. Cloud Storage

## Recommended Storage

**Cloudflare R2**

Store:

- Original photos
- Thumbnails
- Optimized images
- Future videos

Example structure:

```text
tenant/{tenantId}/
    events/{eventId}/
        photos/
            originals/
                photo-001.jpg
                photo-002.jpg

            large/
                photo-001.webp
                photo-002.webp

            thumbnails/
                photo-001.webp
                photo-002.webp
```

The database stores metadata and the R2 object key.

It should NOT store image binary data.

---

# 8. Image Processing

After upload:

```text
Original
   │
   ├── Thumbnail
   │
   ├── Medium
   │
   └── Large
```

Example:

```text
Original
6000 × 4000
20 MB

        ↓

Large
2000 × 1333
~1 MB

        ↓

Thumbnail
400 × 267
~100 KB
```

The gallery should primarily use optimized images rather than original files.

---

# 9. Photo Gallery

Every event has a public gallery.

Example:

```text
Sarah & John's Wedding

[Cover Photo]

1,284 Photos

┌─────┬─────┬─────┐
│ IMG │ IMG │ IMG │
├─────┼─────┼─────┤
│ IMG │ IMG │ IMG │
├─────┼─────┼─────┤
│ IMG │ IMG │ IMG │
└─────┴─────┴─────┘
```

## Guest Features

- View photos
- Full-screen photo viewer
- Download photo
- Share photo
- Swipe through photos
- Mobile-friendly gallery
- Lazy loading
- Infinite scroll/pagination

No account required.

---

# 10. QR Code

Every event automatically receives a QR code.

Example:

```text
EVENT
Sarah & John's Wedding

Gallery URL:
photos.example.com/e/abc123

QR CODE
```

Operator can:

- Download QR as PNG
- Download QR as SVG
- Copy event URL
- Display QR on photobooth screen

Future:

- Custom QR designs
- Printable QR signage
- QR with event branding

---

# 11. Photobooth Integration

This is a key differentiator.

The platform should expose an API specifically for photobooth software.

### Authentication

Photobooth software receives an API key/token.

### Upload Flow

```text
POST /api/v1/events/{eventId}/uploads
```

Response:

```json
{
  "uploadUrl": "...",
  "photoId": "abc123",
  "expiresAt": "..."
}
```

Photobooth software uploads directly to R2.

After successful upload:

```text
R2
 ↓
Processing
 ↓
Thumbnail generated
 ↓
Gallery updated
```

Target:

**Photo appears in gallery within a few seconds after upload completes.**

---

# 12. Event Gallery Settings

Operator can configure:

```text
Event Name
Cover Photo
Gallery Visibility
Download Enabled
Original Download Enabled
Password Protection
Watermark
Event Expiry
```

### Visibility

```text
Public
Password Protected
Private
```

---

# 13. Download Options

Guest:

```text
[Download Photo]
```

Operator:

```text
[Download All Photos]
```

For large events, generate a ZIP asynchronously.

Example:

```text
1,500 photos
       ↓
Background job
       ↓
ZIP generated
       ↓
Download link
```

Do not generate large ZIP files synchronously through the web request.

---

# 14. Event Storage Management

Dashboard should show:

```text
Storage Used

8.4 GB / 100 GB

██████████████░░░░░
```

Event:

```text
Sarah & John's Wedding
8.4 GB
1,284 photos
```

Operator can delete an event.

Deletion should use a safe process:

```text
Delete requested
       ↓
Soft delete
       ↓
Grace period
       ↓
Permanent deletion
```

This reduces accidental photo loss.

---

# 15. Event Expiry

Photobooth operators often don't need photos stored forever.

Allow:

```text
7 days
30 days
90 days
1 year
Never expire
```

Example:

```text
Event expires:
12 December 2026
```

Before deletion:

```text
Your event will expire in 7 days.

[Extend Event]
```

---

# 16. Subscription System

The SaaS should be subscription-based.

Example initial pricing structure:

### Free

```text
RM0/month

1 active event
500 photos
7-day retention
Basic gallery
QR code
```

### Starter

```text
RM29/month

5 active events
5,000 photos/event
30-day retention
Custom branding
Downloads
```

### Pro

```text
RM59/month

Unlimited events
Higher storage
90-day retention
Custom branding
Analytics
API access
```

### Business

```text
RM99+/month

Large storage
Long retention
White label
Custom domain
Priority support
Advanced API
```

These prices should be validated against actual Malaysian customer willingness-to-pay before launch.

---

# 17. Billing

Support:

- Subscription
- Monthly billing
- Annual billing
- Upgrade
- Downgrade
- Cancellation
- Payment history
- Invoice

Architecture should allow integration with Malaysian payment providers later.

---

# 18. Analytics

Event analytics:

```text
Total Photos
Gallery Views
Unique Visitors
Downloads
QR Scans
```

Example:

```text
Sarah & John's Wedding

QR Scans        342
Gallery Views   1,248
Downloads       684
Photos          1,284
```

This is useful to the photobooth operator because it demonstrates value to their client.

---

# 19. Branding

Operator can configure:

```text
Business Name
Logo
Profile Image
Brand Colors
Contact Information
```

Gallery:

```text
        [BUSINESS LOGO]

     Sarah & John's Wedding

        1,284 Photos
```

Future:

- Custom domain
- White label
- Remove platform branding

---

# 20. Admin Dashboard

SaaS administrator can see:

```text
Users
Events
Photos
Storage Usage
Revenue
Subscriptions
System Health
```

Example:

```text
Total Users       128
Active Events      76
Total Photos      1.2M
Storage            4.8 TB
Monthly Revenue   RM7,420
```

---

# 21. Security

## Authentication

- Secure password hashing
- Session/token expiration
- Rate limiting
- Account lockout protection

## Photo Access

Original photos should not simply be permanently public.

Use:

- Signed URLs
- Expiring download URLs
- Event-level permissions

Example:

```text
Guest
 ↓
Gallery API
 ↓
Temporary signed URL
 ↓
Photo
```

## Tenant Isolation

Every database record should belong to a tenant/user.

Example:

```text
tenant_id
event_id
photo_id
```

Never allow:

```text
User A → User B's photos
```

---

# 22. Suggested Technology Stack

A practical MVP:

```text
Frontend
────────
Next.js / React

Backend
────────
Go / Node.js / .NET

Database
────────
PostgreSQL

Object Storage
────────
Cloudflare R2

Image Processing
────────
Cloudflare Workers / Image processing service

CDN
────────
Cloudflare

Background Jobs
────────
Queue-based worker

Authentication
────────
Your own auth / managed authentication provider

Payments
────────
Malaysian payment gateway
```

You can also keep your existing .NET experience and build:

```text
ASP.NET Core API
        │
        ├── PostgreSQL
        │
        └── Cloudflare R2
```

---

# 23. High-Level Architecture

```text
                         ┌──────────────────┐
                         │   Photobooth PC  │
                         └────────┬─────────┘
                                  │
                             API Request
                                  │
                                  ▼
                         ┌──────────────────┐
                         │    SaaS API      │
                         └────────┬─────────┘
                                  │
                           Presigned URL
                                  │
                                  ▼
                         ┌──────────────────┐
                         │  Cloudflare R2   │
                         │                  │
                         │ Original Photos  │
                         │ Thumbnails       │
                         │ Optimized Photos │
                         └────────┬─────────┘
                                  │
                           Image Processing
                                  │
                                  ▼
                         ┌──────────────────┐
                         │    PostgreSQL    │
                         │                  │
                         │ Users            │
                         │ Events           │
                         │ Photos           │
                         │ Subscriptions    │
                         └────────┬─────────┘
                                  │
                                  ▼
                         ┌──────────────────┐
                         │  Public Gallery  │
                         └────────┬─────────┘
                                  │
                             QR / URL
                                  │
                                  ▼
                              Guest
```

---

# 24. MVP Database

Minimum tables:

```text
users
─────
id
email
password_hash
business_name
created_at

events
──────
id
user_id
name
event_date
slug
status
expires_at
created_at

photos
──────
id
event_id
storage_key
original_filename
mime_type
file_size
width
height
status
created_at

event_settings
───────────────
event_id
visibility
password_hash
allow_download
allow_original_download
watermark_enabled

subscriptions
─────────────
id
user_id
plan
status
storage_limit
expires_at

api_keys
────────
id
user_id
name
key_hash
created_at
last_used_at
```

---

# 25. MVP User Journey

## Step 1

Operator signs up.

```text
Create Account
```

## Step 2

Creates event.

```text
Sarah & John's Wedding
12 September 2026
```

## Step 3

System generates:

```text
Event URL
QR Code
API credentials
```

## Step 4

Photobooth connects to the API.

## Step 5

Guest takes photo.

```text
Camera
 ↓
Photobooth software
 ↓
Cloud Photo Delivery
 ↓
R2
```

## Step 6

Photo becomes available.

```text
Guest scans QR
 ↓
Gallery
 ↓
Photo
 ↓
Download
```

## Step 7

After event:

```text
Event completed
 ↓
30/90-day retention
 ↓
Client can continue accessing photos
```

---

# 26. What NOT to Build in MVP

Avoid feature creep.

Do NOT initially build:

- AI face recognition
- AI photo search
- Mobile app
- Print marketplace
- Photographer CRM
- Contract management
- Invoice management
- Lightroom integration
- Advanced editing
- Video editing
- Social network
- Marketplace

First prove:

**Can Malaysian photobooth operators pay for cloud photo delivery?**

---

# 27. Future Roadmap

### Phase 1 — MVP

```text
Events
Photos
R2
Gallery
QR
Downloads
Basic subscription
```

### Phase 2 — Photobooth Platform

```text
Photobooth API
Automatic upload
Live gallery
Live slideshow
Branding
Analytics
```

### Phase 3 — Photographer Platform

```text
Client galleries
Favorites
Photo selection
Comments
Watermarks
Custom domains
Large galleries
```

### Phase 4 — AI

```text
Face recognition
Guest selfie → find photos
AI photo search
Automatic tagging
Duplicate detection
Quality detection
```

### Phase 5 — Business Platform

```text
CRM
Client management
Bookings
Payments
Invoices
Contracts
Marketing
```

---

# 28. Core Product Differentiator

The product should ultimately communicate one simple promise:

> **Take the photos. We deliver them.**

The customer shouldn't care that the backend uses R2, PostgreSQL, Workers, or anything else.

They should experience:

```text
             PHOTOGRAPHER
                  │
                  ▼
          ┌───────────────┐
          │ Upload photos │
          └───────┬───────┘
                  │
                  ▼
             ☁️ CLOUD
                  │
                  ▼
          ┌───────────────┐
          │ Event Gallery │
          └───────┬───────┘
                  │
                  ▼
             QR CODE
                  │
                  ▼
              GUESTS
                  │
                  ▼
        📸 View & Download
```

## Success Metric

The MVP succeeds if a photobooth operator can go from:

**"I have a new event"**

to:

**"Guests can scan the QR code and receive their photos"**

in **less than 5 minutes**, with no technical knowledge.