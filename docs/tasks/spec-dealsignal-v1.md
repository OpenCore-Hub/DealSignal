# SPEC: DealSignal v1

> Technical specification derived from: `tasks/prd-dealsignal-v1.md`
> Generated: 2026-06-17 | Target stack: TypeScript full-stack, PostgreSQL, R2, Resend, pg-boss
> Tile strategy: Server-side encrypted tile stream, front-end assembles like a map

## 1. Summary

### 1.1 What This SPEC Covers

This SPEC defines how to implement the DealSignal v1 web application:
- Multi-tenant workspace model with role-based access.
- Document upload pipeline that converts PDFs/PPTs into encrypted image tiles stored in Cloudflare R2.
- Smart Link creation with access modes (public, email verification, allowlist, password, approval).
- A viewer that fetches encrypted tiles, decrypts them in the browser, and assembles pages on a canvas.
- Page-level analytics collection, intent scoring, and alerting.
- Basic Deal Rooms with folder-level permissions.
- CSV export and email notifications.

### 1.2 PRD Reference

- Source: `tasks/prd-dealsignal-v1.md` (and its Chinese counterpart `tasks/prd-dealsignal-v1-zh.md`)
- User Stories covered: US-001 through US-010
- Functional Requirements covered: FR-1 through FR-30
- Hard constraints: HC-1 through HC-5

### 1.3 Design Decisions Summary

| Decision | Choice | Rationale |
|---|---|---|
| Backend runtime | Node.js + TypeScript (Express/Fastify) | Full-stack TS, strong ecosystem, rapid iteration. |
| Frontend framework | React 18 + Vite | Modern, fast HMR, large ecosystem. |
| ORM / DB client | Drizzle ORM | Type-safe SQL, migration tooling, good Postgres support. |
| Database | PostgreSQL 15+ | PRD schema already designed; ACID for audit events. |
| Object storage | Cloudflare R2 | S3-compatible, cost-effective, no egress fees for tiles. |
| Email provider | Resend | Reliable transactional email, simple API. |
| Job queue | pg-boss | Postgres-backed; no additional infrastructure. |
| Document rendering | Server-side tile pipeline for all file types | PDF / PPT / DOC / XLS are converted to images first, then sliced into encrypted WebP tiles. No source file is ever exposed to the client. |
| Tile format | WebP | Smaller than PNG, widely supported, good quality at low bandwidth. |
| Office file conversion | Self-hosted OnlyOffice → PNG → WebP tiles | Keeps files inside our infrastructure; handles PPT/DOC/XLS uniformly. |
| PDF rendering | Poppler (pdftoppm / pdftocairo) | Fast, battle-tested, server-side PDF → PNG conversion. |
| Watermark | Server-side burned into tiles + client-side dynamic overlay | Server watermark deters screenshots; client overlay shows real-time email/time without re-tiling. |
| Download bundle | Watermarked PDF | Generated on demand when downloadPolicy allows; never returns original file. |
| Viewer rendering | Canvas 2D + OffscreenCanvas + Web Worker | Tile decryption and partial rendering happen off the main thread; scrolling stays at 60 fps. |
| Tile encryption | AES-256-GCM per tile | Protects tile content at rest and in transit; keys are short-lived. |
| Auth | Email/password + JWT sessions | Simple, fits founder wedge; SSO deferred to P2. |

## 2. Architecture

### 2.1 System Context

```
┌─────────────────────────────────────────────────────────────────────┐
│                           Web Browser                               │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────┐  │
│  │   Admin App     │  │  Viewer App     │  │  Mobile Lite        │  │
│  │   (React)       │  │  (React/Canvas) │  │  (React)            │  │
│  └────────┬────────┘  └────────┬────────┘  └────────┬────────────┘  │
└───────────┼────────────────────┼────────────────────┼───────────────┘
            │                    │                    │
            └────────────────────┼────────────────────┘
                                 │ HTTPS / JSON
                                 ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         API Gateway / Server                        │
│                    Node.js + TypeScript + Fastify                   │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌────────────┐  │
│  │ Auth Module  │ │ Doc Module   │ │ Link Module  │ │ Room Module│  │
│  └──────────────┘ └──────────────┘ └──────────────┘ └────────────┘  │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌────────────┐  │
│  │ Viewer Module│ │Analytics Mod │ │ Score Module │ │ Alert Mod  │  │
│  └──────────────┘ └──────────────┘ └──────────────┘ └────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
            │                    │                    │
            ▼                    ▼                    ▼
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────────┐
│   PostgreSQL    │  │   Cloudflare R2 │  │        Resend           │
│  (App data +    │  │  (Source files  │  │   (Email delivery)      │
│   job queue)    │  │   + tile cache) │  │                         │
└─────────────────┘  └─────────────────┘  └─────────────────────────┘
```

### 2.2 Component Design

| Component | Responsibility |
|---|---|
| `AuthService` | Registration, login, JWT issuance, workspace membership, role checks. |
| `DocumentService` | Upload orchestration, version management, processing job enqueueing. |
| `ProcessingPipeline` | Converts source files to page images, slices tiles, encrypts, uploads to R2. |
| `SmartLinkService` | Creates links, validates access modes, manages expiration/revocation. |
| `ViewerService` | Authenticates viewer requests, issues short-lived tile URLs/tokens. |
| `AnalyticsService` | Records view sessions, page views, downloads, access denials. |
| `ScoringService` | Computes intent scores from events via pg-boss jobs. |
| `AlertService` | Queues and sends email alerts for first-open and hot-score events. |
| `RoomService` | Deal room creation, folder/file management, member access rules. |
| `ExportService` | Generates CSV exports from analytics data. |

### 2.3 Module Interactions

Upload and share flow:

```
1. Client POST /documents (multipart)
2. DocumentService streams file to R2 (source bucket)
3. DocumentService inserts document + version rows (status = processing)
4. DocumentService publishes job to pg-boss: process-document
5. Worker downloads source, renders pages, tiles, encrypts, uploads tiles to R2
6. Worker updates document_version.status = ready, document.current_version_id
7. Client creates SmartLink via POST /smart-links
8. Recipient opens /v/{slug}
9. ViewerService validates access, creates view_session
10. Client requests tile manifest + encrypted tiles
11. Client decrypts tiles and assembles pages
12. AnalyticsService records page_view_events
13. ScoringService recomputes intent score
14. AlertService sends first-open / hot-score emails
```

### 2.4 File Structure

```
/apps
  /web                 # Vite React admin + viewer apps
    /src
      /admin           # Dashboard, documents, links, rooms
      /viewer          # Tile-based document viewer
        /canvas        # Main canvas controller
        /worker        # OffscreenCanvas Web Worker (decrypt + decode)
      /shared          # API client, types, utilities
  /api                 # Fastify API server
    /src
      /modules
        /auth
        /documents
        /processing
        /smart-links
        /viewer
        /analytics
        /scoring
        /alerts
        /rooms
        /exports
      /lib
        /db            # Drizzle schema + migrations
        /storage       # R2 client
        /queue         # pg-boss wrapper
        /crypto        # Tile encryption helpers
      /jobs            # pg-boss job handlers
/packages
  /shared-types        # Shared TypeScript types
/supabase|drizzle
  /migrations          # SQL migrations
```

## 3. Data Model

### 3.1 Schema Changes

Use the existing `sql/schema.sql` as the v1 baseline. Drizzle schema files will mirror it.

Key additions for the tile pipeline:

```sql
-- Tile metadata for each document page
CREATE TABLE document_page_tiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    document_version_id UUID NOT NULL REFERENCES document_versions(id) ON DELETE CASCADE,
    page_number INTEGER NOT NULL CHECK (page_number > 0),
    zoom_level INTEGER NOT NULL DEFAULT 1, -- 1 = screen, 2 = high-dpi
    tile_size_px INTEGER NOT NULL DEFAULT 512,
    cols INTEGER NOT NULL,
    rows INTEGER NOT NULL,
    tile_manifest JSONB NOT NULL, -- array of {key, iv, tag, x, y}
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (document_version_id, page_number, zoom_level)
);

CREATE INDEX idx_document_page_tiles_version_page ON document_page_tiles(document_version_id, page_number);
```

### 3.2 Entity Definitions

```typescript
// Tile manifest entry
interface TileManifestEntry {
  key: string;        // R2 object key
  iv: string;         // base64 nonce
  tag: string;        // base64 auth tag
  x: number;          // column index
  y: number;          // row index
}

interface DocumentPageTile {
  id: string;
  workspaceId: string;
  documentVersionId: string;
  pageNumber: number;
  zoomLevel: number;
  tileSizePx: number;
  cols: number;
  rows: number;
  tileManifest: TileManifestEntry[];
}

interface TileTokenPayload {
  tileKey: string;
  sessionId: string;
  exp: number;        // short expiry, e.g. 5 minutes
}
```

### 3.3 Relationships

- `document_versions` 1:N `document_pages` (page metadata)
- `document_versions` 1:N `document_page_tiles` (tile metadata)
- `smart_links` N:1 `documents` / `document_versions`
- `view_sessions` 1:N `page_view_events`
- `contacts` 1:N `intent_scores`
- `workspaces` 1:N `integrations`

### 3.4 Migration Plan

1. Apply baseline `sql/schema.sql` as migration 0001.
2. Add `document_page_tiles` table as migration 0002.
3. Add `pgboss` schema as migration 0003 (if using pg-boss tables).
4. Backward compatibility: v1 has no previous production data.
5. Rollback: drop tables in reverse order.

## 4. API Design

### 4.1 Endpoints

#### Auth

| Method | Path | Description | Auth |
|---|---|---|---|
| POST | /auth/register | Create user + workspace | Public |
| POST | /auth/login | Email/password login | Public |
| POST | /auth/logout | Invalidate session | User |
| GET | /auth/me | Current user + workspaces | User |
| POST | /workspaces | Create workspace | User |
| GET | /workspaces/:id | Get workspace | Member |

#### Documents

| Method | Path | Description | Auth |
|---|---|---|---|
| POST | /documents | Upload file | Member |
| GET | /documents | List workspace documents | Member |
| GET | /documents/:id | Document detail | Member |
| POST | /documents/:id/versions | Upload new version | Member |
| DELETE | /documents/:id | Soft delete | Member/Admin |

#### Smart Links

| Method | Path | Description | Auth |
|---|---|---|---|
| POST | /smart-links | Create link | Member |
| GET | /smart-links | List links | Member |
| GET | /smart-links/:id | Link detail | Member |
| POST | /smart-links/:id/revoke | Revoke link | Member/Admin |

#### Viewer (Public/Recipient)

| Method | Path | Description | Auth |
|---|---|---|---|
| GET | /v/:slug | Resolve link + access check | None / token |
| POST | /v/:slug/verify | Submit email verification code | None |
| POST | /v/:slug/password | Submit password | None |
| POST | /v/:slug/request-access | Request access | None |
| GET | /v/:slug/manifest | Get tile manifest | Viewer session |
| GET | /v/:slug/tiles/:token | Fetch encrypted tile | Viewer session |
| POST | /v/:slug/events | Beacon analytics events | Viewer session |

#### Analytics

| Method | Path | Description | Auth |
|---|---|---|---|
| GET | /analytics/links/:id | Link analytics | Member |
| GET | /analytics/documents/:id | Document analytics | Member |
| GET | /analytics/rooms/:id | Room analytics | Member |
| GET | /analytics/dashboard | Dashboard summary | Member |

#### Deal Rooms

| Method | Path | Description | Auth |
|---|---|---|---|
| POST | /rooms | Create room | Member |
| GET | /rooms | List rooms | Member |
| GET | /rooms/:id | Room detail | Member |
| POST | /rooms/:id/folders | Create folder | Member |
| POST | /rooms/:id/files | Add file to folder | Member |
| POST | /rooms/:id/members | Invite member | Member |

#### Exports

| Method | Path | Description | Auth |
|---|---|---|---|
| GET | /exports/links/:id.csv | Export link analytics | Member |
| GET | /exports/documents/:id.csv | Export document analytics | Member |
| GET | /exports/rooms/:id.csv | Export room analytics | Member |

### 4.2 Request/Response Schemas

#### POST /smart-links

Request:
```json
{
  "documentId": "uuid",
  "documentVersionId": "uuid",
  "name": "Sequoia - Sarah Chen",
  "recipientEmail": "sarah@sequoiacap.com",
  "accessMode": "email_verification",
  "downloadPolicy": "allowed",
  "watermarkEnabled": true,
  "expiresAt": "2026-02-01T00:00:00Z"
}
```

Response:
```json
{
  "id": "uuid",
  "slug": "abc123xyz",
  "url": "https://dealsignal.app/v/abc123xyz",
  "name": "Sequoia - Sarah Chen",
  "accessMode": "email_verification",
  "status": "active",
  "recipientFrictionLevel": "medium"
}
```

#### GET /v/:slug/manifest

Response:
```json
{
  "document": {
    "id": "uuid",
    "name": "Series A Deck.pdf",
    "pageCount": 12,
    "watermarkEnabled": true,
    "watermarkText": "sarah@sequoiacap.com 2026-01-01 12:00 UTC",
    "downloadPolicy": "allowed"
  },
  "pages": [
    {
      "pageNumber": 1,
      "width": 1920,
      "height": 1080,
      "zoomLevels": [
        {
          "zoomLevel": 1,
          "tileSizePx": 512,
          "cols": 4,
          "rows": 3,
          "tiles": [
            {"token": "eyJ...", "x": 0, "y": 0},
            {"token": "eyJ...", "x": 1, "y": 0}
          ]
        }
      ]
    }
  ]
}
```

Note: Each tile token is a short-lived signed JWT containing the encrypted R2 key and decryption metadata. The token itself does not expose the key in plaintext. The browser never receives the original document bytes.

### 4.3 Error Responses

```json
{
  "error": {
    "code": "LINK_EXPIRED",
    "message": "This link has expired. Please contact the sender.",
    "statusCode": 410
  }
}
```

| Code | HTTP | Condition |
|---|---|---|
| LINK_EXPIRED | 410 | `expires_at` passed |
| LINK_REVOKED | 403 | `revoked_at` set |
| ACCESS_DENIED | 403 | Email not allowed or approval pending |
| INVALID_PASSWORD | 401 | Wrong password |
| VERIFICATION_REQUIRED | 401 | Email not verified |
| NOT_FOUND | 404 | Slug does not exist |
| WORKSPACE_ISOLATION | 403 | User accessing cross-workspace resource |
| VALIDATION_ERROR | 400 | Invalid input |

### 4.4 Breaking Changes

v1 is the first release. No backward compatibility concerns.

## 5. Business Logic

### 5.1 Core Algorithms

#### Tile Pipeline

```
Input: source file (PDF/PPT/DOC/XLS) in R2
1. Download source to temporary filesystem
2. Convert source to page images:
   - PDF: render to PNG using Poppler (pdftoppm or pdftocairo)
   - PPT/DOC/XLS: send to self-hosted OnlyOffice Document Server, convert to PDF or PNG, then to WebP
3. For each page:
   a. Determine page dimensions at target DPI (e.g., 150 DPI for screen, 300 DPI for zoom)
   b. Slice page into 512x512px regions
   c. Encode each region as WebP
   d. For each WebP tile:
      i. Generate AES-256-GCM key (or derive from page secret)
      ii. Encrypt tile bytes
      iii. Upload encrypted blob to R2 with key: tiles/{workspaceId}/{versionId}/{page}/{zoom}/{x}-{y}.webp.enc
      iv. Store {key, iv, tag, x, y} in tile_manifest
   e. Insert document_page_tiles row
4. Delete temporary source copy and intermediate PNGs
5. Update document_version.status = ready
```

**Unified download protection:** Because every document is rendered to image tiles on the server and only encrypted WebP tiles are sent to the browser, there is no PDF/PPT/DOC/XLS source file to download. The viewer's "download" action (when enabled by `downloadPolicy`) will generate a watermarked, lower-resolution PDF or image bundle on demand rather than returning the original file.

#### Access Resolution

```
Input: slug, viewer context
1. Lookup smart_links by slug
2. If not found → 404
3. If revoked_at set → LINK_REVOKED
4. If expires_at passed → LINK_EXPIRED
5. Resolve recipient identity:
   - public: anonymous session
   - email_verification: require verified email in session
   - allowlist: require email in allowed list
   - password: require correct password in session
   - approval_required: require approved access_grant
6. If blocked → ACCESS_DENIED
7. Create view_session
8. Return document metadata + access token
```

#### Viewer Rendering (Canvas 2D + OffscreenCanvas + Web Worker)

```
Input: tile manifest from GET /v/:slug/manifest
1. Main thread creates a visible <canvas> and an OffscreenCanvas
2. OffscreenCanvas is transferred to a dedicated Web Worker
3. Worker maintains a tile cache (LRU, max ~50 tiles)
4. On scroll/zoom:
   a. Main thread computes visible tile coordinates
   b. Main thread requests tile tokens for missing tiles
   c. Worker fetches encrypted WebP tiles via /v/:slug/tiles/:token
   d. Worker decrypts tile using Web Crypto API (SubtleCrypto)
   e. Worker decodes WebP to ImageBitmap
   f. Worker draws ImageBitmap onto OffscreenCanvas at correct (x, y)
5. Main thread renders the OffscreenCanvas content to the visible canvas each frame
6. Main thread overlays dynamic watermark text (recipient email + current UTC time) on top of the document canvas

**Watermark layering:**
- **Server-side watermark:** Baked into each tile during processing with semi-transparent text. This survives screenshots and screen recordings of individual tiles.
- **Client-side watermark:** Rendered dynamically on the visible canvas with the current recipient email and timestamp. Updates in real time without re-fetching tiles and remains sharp during zoom/pan.
- Combined, the two layers make it harder to remove or crop out the watermark.
```

**Why this matters:**
- Decryption and decoding happen off the main thread → scrolling stays smooth.
- The visible canvas can composite multiple layers (document tiles + watermark + loading placeholders) without re-decoding.
- `requestAnimationFrame` throttles main-thread rendering to 60 fps.

#### Intent Scoring

```
Input: contact_id, smart_link_id, event stream
1. Aggregate last 30 days of events for this contact-link pair
2. Compute signals:
   - open_count: count link_opened events
   - repeat_opens: open_count > 1
   - total_view_time_ms: sum page_view durations
   - key_pages_viewed: count of distinct important pages (e.g., team, financials, pricing)
   - re_read_count: pages viewed more than once
   - forwarded: distinct emails/sessions from same link
3. Base score = weighted sum of signals
4. Normalize to 0-100
5. Label: cold/warm/hot
6. Generate explanation from top 2-3 signals
7. Insert intent_scores row
```

### 5.2 Validation Rules

- `expiresAt` must be in the future.
- `accessMode = password` requires `password` of at least 8 characters.
- `accessMode = allowlist` requires at least one allowed email or domain.
- File uploads limited to 100 MB per file in v1.
- Supported MIME types: application/pdf, application/vnd.openxmlformats-officedocument.*, image/*, video/*.
- Video files bypass tile pipeline; served as streaming source with access token.
- `downloadPolicy = allowed` triggers on-demand generation of a watermarked PDF bundle (never the original source file).

### 5.3 State Machine

**Document version status:**

```
uploaded → processing → ready
              ↓
            failed
```

**Smart link status:**

```
active → expired (time-based)
     ↘ revoked (manual)
```

**Access grant status:**

```
pending → approved → revoked
     ↘ denied
```

### 5.4 Edge Cases

- **Large PDF (>100 pages or >100 MB):** Cap processing at 200 pages in v1; beyond that, process first 200 and flag for manual review.
- **No text layer:** Tile pipeline still works; search deferred to P2.
- **Email verification code expired:** Allow resend with rate limit (max 3 per 10 minutes).
- **Concurrent score jobs for same contact:** Use unique job key in pg-boss to deduplicate.
- **Tile token expired while viewing:** Client refreshes manifest; viewer remains seamless.
- **Viewer takes screenshot:** Accepted risk; watermark provides traceability.

## 6. Error Handling

### 6.1 Error Taxonomy

| Error Code | HTTP Status | Condition | User Message |
|---|---|---|---|
| VALIDATION_ERROR | 400 | Invalid input | "Please check your input and try again." |
| UNAUTHORIZED | 401 | Missing/invalid auth | "Please sign in." |
| FORBIDDEN | 403 | Cross-workspace or role | "You don't have permission." |
| NOT_FOUND | 404 | Resource missing | "Not found." |
| LINK_EXPIRED | 410 | Link expired | "This link has expired. Contact the sender." |
| LINK_REVOKED | 410 | Link revoked | "This link has been revoked. Contact the sender." |
| RATE_LIMITED | 429 | Too many requests | "Too many attempts. Please wait." |
| INTERNAL_ERROR | 500 | Unexpected failure | "Something went wrong. We're looking into it." |

### 6.2 Retry Strategy

| Operation | Retryable | Backoff | Max attempts |
|---|---|---|---|
| Upload to R2 | Yes | Exponential, 1s base | 5 |
| Email send | Yes | Exponential, 5s base | 5 |
| Processing job | Yes | Fixed, 30s delay | 3 |
| Score job | Yes | Exponential, 1s base | 5 |
| CRM sync | Yes | Exponential, 5s base, respect 429 | 10 |

### 6.3 Failure Modes

- **R2 unavailable:** Uploads queue; viewer falls back to "document temporarily unavailable" message.
- **pg-boss down:** Processing/scoring jobs fail; alerts logged; manual retry endpoint available for admins.
- **Resend down:** Emails queued in `notifications` table with `failed` status; retry via cron.
- **Processing worker crash:** Job requeued; if 3 failures, mark version as `failed` and notify uploader.

## 7. Security

### 7.1 Authentication & Authorization

- Users authenticate via email/password with bcrypt hashing.
- Sessions use HTTP-only cookies + CSRF protection for admin app.
- Viewer sessions use signed tokens stored in memory/localStorage (no long-lived viewer accounts).
- All API routes enforce workspace membership and role checks.
- Cross-workspace access returns 403.

### 7.2 Input Validation

- Zod schemas for all API inputs.
- File type and size validation before streaming.
- Sanitize all text fields; no HTML rendering without sanitization.
- Rate limiting: 100 req/min per IP for public viewer endpoints; 1000 req/min for authenticated users.

### 7.3 Data Protection

- TLS 1.3 for all traffic.
- Tile encryption keys are per-version, rotated on new uploads.
- Tile tokens are short-lived JWTs signed with server secret.
- Source files in R2 are not publicly readable; presigned URLs are never exposed to clients.
- Integration credentials encrypted with AES-256-GCM using app-level KMS key.
- IP addresses retained for 90 days by default.

## 8. Performance

### 8.1 Expected Load

- 1,000 workspaces in beta.
- 10,000 documents processed in first 3 months.
- Peak: 100 concurrent viewers.
- Average document: 12 pages, 5 MB source, ~50 tiles at 512px.

### 8.2 Optimization Strategy

- **Tile CDN:** R2 supports custom domains + caching headers; tiles immutable once uploaded.
- **Manifest caching:** Cache tile manifest for 60 seconds; tokens refreshed on each fetch.
- **Lazy tile loading:** Only fetch tiles in viewport; prefetch adjacent pages.
- **Connection pooling:** PostgreSQL pool size 20 per worker.
- **Score caching:** Cache latest intent_score per contact-link for 5 minutes.
- **Tile format:** Serve WebP tiles. Worker decodes WebP to ImageBitmap; if decoding fails, fall back to a low-resolution placeholder and log.

### 8.3 Database Considerations

- All tenant-scoped queries filter by `workspace_id`.
- Indexes from `sql/schema.sql` are mandatory.
- Avoid N+1 by joining document_versions + document_pages + tiles in manifest query.
- Analytics events partitioned by `occurred_at` monthly when volume > 1M rows.

## 9. Testing Strategy

### 9.1 Unit Tests

- Access resolution logic (all modes, expired, revoked, allowlist).
- Tile encryption/decryption roundtrip.
- Intent score calculation with synthetic events.
- Workspace isolation middleware.

### 9.2 Integration Tests

- Full upload → process → create link → open viewer → record event → score update flow.
- Email verification flow end-to-end.
- Revoked link returns block page and no tiles.
- Cross-workspace access returns 403.

### 9.3 Edge Case Tests

- 200-page PDF processing cap.
- Concurrent score jobs deduplication.
- Tile token expiry mid-session.
- Resend failure retry.

### 9.4 Acceptance Criteria Mapping

| US/FR | Test | Type | Description |
|---|---|---|---|
| US-001 | upload-supported-types | integration | Upload PDF, verify DB + R2 records. |
| US-002 | create-smart-link | integration | Create link, verify slug + settings. |
| US-003 | viewer-access-public | integration | Open public link without account. |
| US-004 | page-view-event | integration | View page, verify event row. |
| US-005 | intent-score-update | integration | Simulate activity, verify score change. |
| US-006 | create-deal-room | integration | Create room from template, invite member. |
| US-007 | watermark-visible | e2e | Screenshot viewer with watermark. |
| US-008 | first-open-alert | integration | Trigger event, verify email queued. |
| FR-29 | revoked-link-blocked | e2e | Revoke link, verify no tiles loaded. |
| HC-1 | workspace-isolation | integration | Cross-workspace request returns 403. |

## 10. Implementation Plan

### 10.1 Phases

**Phase 0: Foundation (Issues 1-2)**
- Project scaffold, DB migrations, auth, workspace model.

**Phase 1: Document pipeline (Issues 3-5)**
- Upload, R2 source storage, tile processing pipeline, documents UI.

**Phase 2: Smart Links (Issues 6-8)**
- Link creation, permissions, link detail UI.

**Phase 3: Viewer (Issues 9-10)**
- Access resolution, encrypted tile manifest, canvas viewer.

**Phase 4: Analytics & Scoring (Issues 11-15)**
- Event collection, timeline, dashboard, intent scoring.

**Phase 5: Security & Notifications (Issues 16-17)**
- Watermark, email alerts.

**Phase 6: Deal Rooms (Issues 18-19)**
- Room backend, room management UI.

**Phase 7: Exports (Issue 20)**
- CSV export.

### 10.2 Issue Mapping

| Issue | PRD Source | SPEC Sections | Priority | Depends On |
|---|---|---|---|---|
| 1 | HC-1, schema | 2.4, 3.1, 3.4 | high | — |
| 2 | HC-1 | 7.1, 9.1 | high | #1 |
| 3 | US-001, FR-1 | 4.1 Documents, 5.1 Tile pipeline | high | #1, #2 |
| 4 | US-001, FR-12 | 3.1 document_page_tiles, 5.1 | high | #3 |
| 5 | US-001 | 4.1, 9.4 | high | #3, #4 |
| 6 | US-002, FR-2~10 | 4.1 Smart Links, 5.2 | high | #3 |
| 7 | US-002 | 4.2, 9.4 | high | #6 |
| 8 | US-002 | 4.1, 9.4 | high | #6 |
| 9 | US-003, FR-6, FR-29 | 4.1 Viewer, 5.1 Access resolution, 7.3 | high | #6 |
| 10 | US-003 | 2.1, 4.2 manifest, 5.1 | high | #4, #9 |
| 11 | US-004, FR-11~12 | 4.1 events, 5.1 | high | #9, #10 |
| 12 | US-004, FR-13 | 4.1, 5.4 | high | #9 |
| 13 | US-004, FR-14~15 | 4.1 Analytics, 9.4 | high | #11, #12 |
| 14 | US-005, FR-16~17 | 5.1 Scoring, 8.2 | high | #11, #12 |
| 15 | US-005, US-008 | 4.1 Dashboard, 9.4 | high | #13, #14 |
| 16 | US-007, FR-10 | 5.1 Watermark, 7.3 | medium | #10 |
| 17 | US-008, FR-22 | 4.1 Alerts, 6.2, 9.4 | medium | #14 |
| 18 | US-006, FR-18~21 | 3.1, 4.1 Rooms, 5.1 | medium | #3, #2 |
| 19 | US-006 | 4.1, 9.4 | medium | #18 |
| 20 | FR-30 | 4.1 Exports, 8.3 | medium | #13 |

### 10.3 Incremental Delivery

- Use feature flags for tile pipeline vs legacy source-file viewer during transition (not needed in v1 but good practice).
- Ship P0 end-to-end before adding P1 CRM/Slack.
- Beta users get all P0 features; P1 features gated by workspace plan.

## 11. Open Questions & Risks

### 11.1 Resolved Questions

The following questions from the initial SPEC draft have been confirmed by the product owner:

| # | Question | Decision |
|---|---|---|
| 1 | PDF rendering strategy | Convert PDF to PNG images, then slice into encrypted WebP tiles. Unified download protection: no source file is exposed. |
| 2 | Tile format | WebP |
| 3 | PPT/DOC/XLS rendering | OnlyOffice Document Server → PNG → WebP tiles |
| 4 | Viewer canvas technology | Canvas 2D with OffscreenCanvas + Web Worker for decryption/decoding; main thread stays responsive |

### 11.2 Resolved Questions (Batch 2)

The following remaining questions have now been confirmed by the product owner:

| # | Question | Decision |
|---|---|---|
| 5 | PDF-to-PNG library | Poppler (`pdftoppm` / `pdftocairo`) |
| 6 | OnlyOffice deployment | Self-hosted |
| 7 | Watermark strategy | Server-side burned into tiles + client-side dynamic overlay |
| 8 | Download bundle format | Watermarked PDF generated on demand |

### 11.3 Technical Risks

| Risk | Impact | Mitigation |
|---|---|---|
| Tile processing is too slow/costly | High | Benchmark libraries; cap pages; cache aggressively; use R2 CDN. |
| Encrypted tile delivery increases viewer latency | Medium | Use small tiles, lazy loading, HTTP/2, CDN caching of immutable tiles. |
| Browser canvas performance poor on mobile | Medium | Optimize tile size; use 2D canvas; test on low-end devices. |
| pg-boss queue backlog under load | Medium | Scale workers horizontally; set job concurrency limits. |
| Source-file extraction for non-PDF types | Medium | Use self-hosted OnlyOffice for PPT/DOC/XLS conversion; isolate in worker. |

### 11.4 Assumptions

- Cloudflare R2 bucket is configured with no public access; all tile reads go through API-signed tokens.
- Poppler binaries (`pdftoppm`, `pdftocairo`) are available on processing workers.
- A self-hosted OnlyOffice Document Server is deployed and reachable from processing workers.
- Processing workers have sufficient CPU/memory to render PDF pages to images.
- Resend account is configured and domain verified before launch.
- PostgreSQL has `pgcrypto` and `citext` extensions enabled.
- No existing user data; migrations can be destructive in v1.
