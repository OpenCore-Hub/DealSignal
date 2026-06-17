---
workflow_contract_version: 1
feature_slug: dealsignal-v1
target_surface: web
product_depth: deep
recommended_next_step: to-issues
p0_stories: [US-001, US-002, US-003, US-004, US-005, US-006, US-007, US-008]
issue_mapping_count: 46
hard_constraints_count: 5
known_unknowns_count: 6
acceptance_scripts_count: 5
generated_at: 2026-06-17
---

# DealSignal v1 — Unified Product Requirements Document

> Merged from `PRD.md` and `PRD + 产品设计的完整文档草案.md`. Version 1. Old files are intentionally untouched.

## 0. Flow Readiness Card

- **Product:** DealSignal — secure document sharing, deal rooms, and intent analytics for fundraising founders, investment firms, and B2B sales teams.
- **Core user loop:** Upload a document → create a controlled Smart Link → recipient opens in a branded viewer → sender sees page-level engagement and intent score → sender follows up at the right time.
- **Target surface:** Web-first (desktop admin + mobile viewer + mobile management lite). No native apps in v1.
- **P0 outcome:** A founder can upload a pitch deck, create an investor-specific Smart Link, see who opened it and which pages they re-read, and receive a Hot-intent alert within minutes.
- **Hard constraints:** Workspace-level tenant isolation; access control enforced before content loads; analytics events are append-only; recipients only create accounts when sender policy requires it; link revocation takes effect immediately.
- **Recommended defaults:** PostgreSQL 15+; object storage for files; email verification as the default access mode; founder segment as the first wedge; 10 active links on the Founder Plan.
- **Creative space:** Exact UI copy, empty states, animation details, brand color defaults, notification frequency, and scoring algorithm weights within the documented bands.
- **Must not build:** Native mobile apps, full e-signature workflows, legal-grade DRM against screenshots, enterprise DLP, data residency, or AI-generated content rewriting in v1.
- **Sharpest product decision:** Reduce fundraising uncertainty before adding control — the first wedge is founders who need to know which investors are truly interested, not funds who need audit-grade VDRs.
- **P0 acceptance script:** A founder uploads a 12-page PDF, creates a Smart Link with email verification + watermark, opens it from a second email, navigates to page 8 for 45 seconds, and sees the event + intent score update in the dashboard within 60 seconds.
- **Best next step:** `/to-issues` because the P0 loop is local to one web surface and the existing issue mapping can be consumed directly.

## 1. Product Decision Core

### 1.1 Positioning

DealSignal lets founders, investors, and sales teams send sensitive business documents through controlled, trackable links and convert recipient behavior into actionable deal signals — without the friction of legacy secure document tools or the blind follow-up of ordinary file sharing.

Chinese positioning:

> 把每一份关键文档变成可控、可追踪、可推进成交的交易信号系统。

### 1.2 Differentiation and Switching Trigger

| Current alternative / workaround | Structural reason it fails | Product difference | Switching trigger | Fact status |
|---|---|---|---|---|
| DocSend / legacy secure docs | Viewer forces registration or heavy gating, creating recipient friction in investor/customer workflows; analytics stop at "who opened" without explaining what to do next. | Lower-friction viewer with segment-specific intent scores and recommended next actions. | Founder sends a deck to 20 investors, gets opens but no signal, misses the one investor who re-read the financials three times. | Unverified — based on product positioning, not audited pricing. |
| Google Drive / Dropbox / email attachments | No page-level analytics, no access revocation after sending, no intent signal, version control is manual. | Controlled links with expiration, revocation, version updates, and per-page engagement tracking. | Sender forwards a proposal and later learns the buyer shared an outdated version internally. | Verified — generic cloud drives lack engagement analytics by design. |
| Traditional VDRs (Intralinks, Merrill) | Heavy procurement, slow setup, built for M&A not for lightweight fundraising or sales proposals. | Lightweight deal rooms with templates and minutes-long setup. | Emerging fund or sales team needs a room today but faces a 2-week VDR onboarding process. | Unverified — based on category positioning. |

### 1.3 User Segments

| Segment | Core goal | Current frustration | Switch-worthy moment | P0 relevance |
|---|---|---|---|---|
| Fundraising founders | Identify real investor interest before following up. | Don't know which investors are serious; decks leak or get outdated. | After sending a deck, sees an investor re-read the team and financial pages three times. | Primary P0 target. |
| Investment firms (VC/PE/IR/M&A) | Maintain control and auditability of sensitive capital materials; identify LP/buyer engagement. | LP updates disappear after sending; no engagement visibility; VDRs are overkill for small funds. | LP committee asks for proof of engagement before the next close. | Secondary; enters through recipient exposure and founder referrals. |
| B2B sales / BD teams | Identify buying intent and time follow-ups. | Proposals go silent; champion forwards materials but seller has no visibility. | Pricing page is viewed by three new stakeholders in one day. | Secondary; natural expansion after founders start selling. |

### 1.4 User Problem

- **Current pain:** After sending important materials, users lose visibility, control, and timing. They don't know who is truly interested, when to follow up, or whether sensitive content is being shared inappropriately.
- **Why now:** Fundraising is increasingly remote and asynchronous; buyers involve larger committees; funds need lighter LP communication tools than traditional VDRs.
- **Existing workaround:** DocSend, Google Drive, Dropbox, email attachments, or traditional VDRs.
- **Why the workaround fails:** See Section 1.2.

### 1.5 Success Definition

- **User-visible success:** Sender knows which recipients are hot within minutes of their activity and can take the right next action without guessing.
- **Business/project success:** Founders activate by uploading a deck and creating at least one investor link; 50%+ of active users return after the first recipient open; 30%+ of beta users create a deal room.
- **Engineering success:** P0 features are shippable as a single web app with one database and one object storage backend; end-to-end loop can be verified in a local environment.

### 1.6 Assumptions and Fact Status

| Item | Status | Why it matters |
|---|---|---|
| Founder wedge is the fastest path to initial users. | Assumption | Drives P0 segment focus and first landing page copy. |
| Investors will open founder decks without registering. | Assumption | Drives default access-mode decision and viewer UX. |
| Page-level time-on-page can be accurately measured in a browser PDF viewer. | Assumption | Drives analytics event design and scoring inputs. |
| HubSpot/Salesforce API rate limits won't constrain MVP usage. | Unknown | Affects CRM sync batching design in P1. |
| Dynamic watermarking on PDF downloads is feasible without expensive rendering infrastructure. | Unknown | Affects watermark scope in P0 vs P1. |
| 10 active links is the right Founder Plan limit. | Assumption | Affects pricing and free-to-paid conversion flow. |

## 2. Scope Contract

### 2.1 Hard Constraints

| Constraint | Why it is hard | Downstream impact |
|---|---|---|
| Every tenant-scoped table must be filtered by `workspace_id`. | Multi-tenant SaaS; data leakage between workspaces is catastrophic. | All queries, indexes, and API handlers must include workspace scoping; tests must verify cross-workspace isolation. |
| Access rules must be enforced before document bytes are returned. | Security promise; a leaked document cannot be un-leaked. | Viewer middleware must resolve and validate the link before streaming content; no direct object-storage URLs. |
| Analytics event records are append-only. | Auditability and scoring reproducibility depend on immutable raw events. | Events are inserted, never updated; derived values live in separate tables. |
| Recipients create accounts only when sender policy explicitly requires it. | Core growth loop; friction kills deal velocity. | Public and email-verification modes must work without registration. |
| Link revocation must take effect immediately. | Sender trust; a revoked link must never return content. | Status check is authoritative and cached revocation must invalidate quickly. |

### 2.2 Recommended Defaults

| Default | Why this default | Acceptable substitute |
|---|---|---|
| PostgreSQL 15+ with the provided schema. | Schema already designed; relational model fits tenant + documents + events. | Cloud-managed Postgres (RDS, Supabase, Neon) with the same schema. |
| Object storage for file blobs. | Decouples file serving from app servers; scales independently. | S3, R2, MinIO, or GCS with compatible API. |
| Email verification as default access mode. | Balances sender confidence with recipient friction. | Public mode for very low-friction campaigns; allowlist/password for high-security materials. |
| Founder segment as first wedge. | Fastest buying cycle; urgent pain; natural viral loop to investors. | Sales segment if beta data shows stronger activation. |
| 10 active links on Founder Plan. | Creates conversion pressure without blocking a real fundraise. | Adjust based on free-to-paid conversion data. |

### 2.3 Creative Space

| Area | What may improve | Guardrail |
|---|---|---|
| Dashboard card layout and copy | Empty states, alert badges, recommended-action wording | Must still surface hot signals, recent opens, and risks. |
| Scoring algorithm weights | Exact point assignments for page re-reads, forwards, time | Score bands (0-39/40-69/70-100) and segment labels must remain. |
| Viewer UI chrome | Top bar, bottom bar, outline drawer, loading skeletons | Must not block document content or hide download-policy state. |
| Email alert design | Subject lines, send timing, digest vs instant | Must deliver first-open and hot-score events reliably. |
| Onboarding flow | Step order, tooltips, template suggestions | Must get user to first Smart Link within minutes. |

### 2.4 Non-Goals

| Non-goal | Why excluded | Revisit trigger |
|---|---|---|
| General-purpose cloud drive | Out of category; would dilute transaction-intelligence positioning. | User research shows persistent demand for general storage. |
| Legal-grade DRM against screenshots | Technically impossible to guarantee; watermark + audit is the MVP position. | Enterprise customers demand it and are willing to pay. |
| Native email campaign automation | Would compete with Mailchimp/Apollo; not core to document signals. | Sales users repeatedly ask for sequences. |
| Full e-signature workflows | HelloSign/Docusign exist; signature is a different job. | Fundraising closing workflows demand it. |
| AI rewriting of decks/proposals | High risk of generic output; action recommendations are safer. | User research shows strong demand and trust. |
| Enterprise DLP / data residency / SSO / SCIM in v1 | Heavy enterprise procurement; founder wedge doesn't need it. | Secure Room plan gains traction. |

## 3. Users, Jobs, and Scenarios

### 3.1 Primary User

- **Role:** Fundraising founder (Seed/Series A CEO/CFO/operator).
- **Job-to-be-done:** Know which investors are truly interested before sending the next follow-up.
- **Current trigger:** Starting a fundraise, sending the first deck, investors ask for a data room.
- **Success evidence:** Creates investor-specific links, sees Hot scores, and schedules more meetings with serious investors.

### 3.2 Secondary Users or Operators

- **Investment firm operators / IR / partners:** Need controlled LP/deal materials with audit trails.
- **B2B sales reps / managers:** Need proposal tracking and buying-committee visibility.
- **Workspace admins:** Configure defaults, approve content, manage members.

### 3.3 Critical Scenarios

| Scenario | Entry point | Desired outcome | Failure to avoid |
|---|---|---|---|
| Founder sends deck to investor | Documents page → Create Smart Link | Investor opens with low friction; founder sees first-open alert and page analytics | Investor hits a registration wall or gets an expired link by mistake. |
| Investor forwards deck to partner | Recipient viewer → new email opens link | Founder detects new viewer/forward and scores increase | New viewer is blocked because allowlist is too narrow. |
| Founder opens fundraising data room | Deal Rooms → Create room → invite investors | Investors browse materials; founder tracks room engagement | Room setup is too slow or permissions are confusing. |
| Sales rep sends proposal to champion | Content Library or Documents → Smart Link | Champion and committee view pricing; rep gets Slack/email alert | Rep never learns the proposal was forwarded internally. |
| Fund IR sends quarterly LP update | Deal Rooms → LP Update Room template | LPs access branded portal; IR identifies high-engagement LPs | LP update looks unprofessional or lacks access control. |

## 4. User Stories

### US-001: Upload a Document

**Description:** As a sender, I want to upload a document so that I can create a secure, trackable link.
**Priority:** P0
**Source:** Critical scenario — founder sends deck to investor.

**Acceptance Criteria:**
- [ ] User can upload PDF, PPT, DOC, XLS, image, and video files.
- [ ] Upload progress is visible.
- [ ] Failed uploads show a specific error message.
- [ ] Uploaded document appears in the Documents list with status.
- [ ] Typecheck, lint, and build pass.

### US-002: Create a Smart Link

**Description:** As a sender, I want to generate a link with access settings so that I can share a document safely.
**Priority:** P0
**Source:** Critical scenario — founder sends deck to investor.

**Acceptance Criteria:**
- [ ] User can create a named link from a document.
- [ ] User can choose access mode and security preset.
- [ ] User can enable or disable downloads.
- [ ] User can set expiration.
- [ ] User can copy the created link.
- [ ] Recipient friction level is shown before creation.
- [ ] Verify in browser.

### US-003: View a Shared Document

**Description:** As a recipient, I want to open a shared document with minimal friction so that I can review it quickly.
**Priority:** P0
**Source:** Critical scenario — investor opens deck.

**Acceptance Criteria:**
- [ ] Recipient can open a valid link without creating an account when policy allows.
- [ ] Recipient sees a readable document viewer on desktop and mobile.
- [ ] Recipient can move between pages and see an outline.
- [ ] Recipient sees a clear message if access is expired or denied.
- [ ] Download is only shown when enabled.
- [ ] Verify desktop and mobile browser views.

### US-004: Track Recipient Activity

**Description:** As a sender, I want to see recipient activity so that I can understand interest.
**Priority:** P0
**Source:** Critical scenario — investor forwards deck to partner.

**Acceptance Criteria:**
- [ ] System records first open and repeat open.
- [ ] System records page-level viewing with duration.
- [ ] System records download event when download is enabled.
- [ ] Activity appears in link analytics within 60 seconds.
- [ ] Event delay is under 10 seconds in normal conditions.

### US-005: Generate Intent Score

**Description:** As a sender, I want DealSignal to score engagement so that I can prioritize follow-up.
**Priority:** P0
**Source:** Primary user job-to-be-done.

**Acceptance Criteria:**
- [ ] System generates a 0-100 score per recipient.
- [ ] Score maps to Cold (0-39), Warm (40-69), or Hot (70-100).
- [ ] Score includes a natural-language explanation.
- [ ] Score updates when new activity occurs.
- [ ] Segment-specific scoring types are supported.

### US-006: Create a Deal Room

**Description:** As a sender, I want to create a room from a template so that I can share multiple diligence materials quickly.
**Priority:** P0
**Source:** Critical scenario — founder opens fundraising data room.

**Acceptance Criteria:**
- [ ] User can choose a room template (Seed Fundraising, Series A, LP Update, M&A Diligence, Enterprise Sales, Partner Enablement).
- [ ] Room contains default folders from the template.
- [ ] User can upload files into folders.
- [ ] User can invite recipients.
- [ ] User can view room activity.
- [ ] Verify in browser.

### US-007: Apply Dynamic Watermark

**Description:** As a sender, I want to watermark documents with recipient information so that leaks are discouraged and traceable.
**Priority:** P0
**Source:** Hard constraint — deterrence and traceability.

**Acceptance Criteria:**
- [ ] User can enable watermark for a link or room.
- [ ] Viewer displays watermark with recipient email and timestamp.
- [ ] Downloaded files include watermark when download is enabled and policy requires it.
- [ ] Watermark setting is visible in link/room settings.

### US-008: Receive High-Intent Alerts

**Description:** As a sender, I want to be alerted when a recipient shows strong interest so that I can follow up at the right time.
**Priority:** P0
**Source:** Primary user job-to-be-done.

**Acceptance Criteria:**
- [ ] User can configure email alerts.
- [ ] First-open alert is sent.
- [ ] Hot-score alert is sent.
- [ ] Alert links to the relevant analytics page.
- [ ] Alerts are queued and retried on failure.

### US-009: Sync Activity to CRM

**Description:** As a sales user, I want document activity synced to CRM so that the deal record stays current.
**Priority:** P1
**Source:** Secondary user segment — B2B sales.

**Acceptance Criteria:**
- [ ] User can connect HubSpot or Salesforce.
- [ ] User can associate a Smart Link with a CRM deal/contact.
- [ ] System writes open and high-intent events to CRM timeline.
- [ ] System creates follow-up task for Hot score events when enabled.

### US-010: Manage Approved Sales Content

**Description:** As a sales manager, I want approved content in a shared library so that reps send the correct materials.
**Priority:** P1
**Source:** Secondary user segment — B2B sales.

**Acceptance Criteria:**
- [ ] Admin can mark a document as Approved, In Review, Draft, or Archived.
- [ ] Team members can filter by status.
- [ ] Admin can restrict Smart Link creation to approved content when enabled.
- [ ] Content performance is trackable.

## 5. Functional Requirements

- **FR-1:** The system must allow users to upload supported document files.
- **FR-2:** The system must generate unique Smart Links for documents.
- **FR-3:** The system must allow multiple Smart Links per document.
- **FR-4:** The system must allow users to set link expiration.
- **FR-5:** The system must allow users to revoke a link.
- **FR-6:** The system must allow users to require recipient email verification.
- **FR-7:** The system must allow users to restrict access by email allowlist.
- **FR-8:** The system must allow users to enable password protection.
- **FR-9:** The system must allow users to enable or disable downloads.
- **FR-10:** The system must allow users to enable dynamic watermarking.
- **FR-11:** The system must record document open events.
- **FR-12:** The system must record page-level viewing events.
- **FR-13:** The system must record download events.
- **FR-14:** The system must display recipient-level analytics.
- **FR-15:** The system must display document-level analytics.
- **FR-16:** The system must generate a segment-specific intent score.
- **FR-17:** The system must explain why an intent score changed.
- **FR-18:** The system must allow users to create Deal Rooms.
- **FR-19:** The system must allow users to apply folder-level room permissions.
- **FR-20:** The system must allow users to invite recipients to a room.
- **FR-21:** The system must provide room activity logs.
- **FR-22:** The system must provide high-intent notifications.
- **FR-23:** The system must allow users to connect Slack.
- **FR-24:** The system must allow users to connect HubSpot or Salesforce.
- **FR-25:** The system must sync selected activity events to CRM.
- **FR-26:** The system must provide a content library.
- **FR-27:** The system must support document version history.
- **FR-28:** The system must allow admins to archive documents.
- **FR-29:** The system must show blocked, expired, and denied access pages.
- **FR-30:** The system must provide CSV export for analytics.

## 6. Experience and State Contract

### 6.1 Primary Flow

```
[Sender]          [System]              [Recipient]           [Sender]
   │                 │                       │                    │
   │ Upload doc      │                       │                    │
   │────────────────>│                       │                    │
   │                 │ Process pages         │                    │
   │                 │──────┐                │                    │
   │                 │<─────┘                │                    │
   │                 │                       │                    │
   │ Create Smart Link                      │                    │
   │────────────────>│                       │                    │
   │                 │ Generate slug + settings                  │
   │                 │<──────┐               │                    │
   │ Copy link       │       │               │                    │
   │<────────────────│       │               │                    │
   │                 │       │               │                    │
   │ Send link via email/Slack/etc.         │                    │
   │───────────────────────────────────────>│                    │
   │                 │                       │                    │
   │                 │ Resolve link          │                    │
   │                 │<──────────────────────│                    │
   │                 │ Enforce access rules  │                    │
   │                 │──────────────────────>│ (pass / block)     │
   │                 │                       │                    │
   │                 │ Stream document       │                    │
   │                 │──────────────────────>│                    │
   │                 │                       │                    │
   │                 │ Record events         │                    │
   │                 │──────┐                │                    │
   │                 │      │ Update score   │                    │
   │                 │<─────┘                │                    │
   │                 │                       │                    │
   │                 │ Send alert            │                    │
   │                 │──────────────────────>│                    │
   │                 │                       │                    │
   │ View analytics  │                       │                    │
   │<────────────────│                       │                    │
   │                 │                       │                    │
   │ Take follow-up action                  │                    │
   │─────────────────────────────────────────────────────────────>│
```

### 6.2 Layout or Interface Model

Desktop Web Admin:

```text
┌────────────────────────────────────────────────────────────────────┐
│ DealSignal    Search    + Create    Alerts    Workspace  Profile   │
├──────────────┬─────────────────────────────────────────────────────┤
│ Dashboard    │ Today                                                    │
│ Documents    │ ┌────────────┐ ┌────────────┐ ┌────────────┐             │
│ Links        │ │ Hot Signals│ │ Opens      │ │ Risks      │             │
│ Deal Rooms   │ │ 8          │ │ 34         │ │ 2          │             │
│ Contacts     │ └────────────┘ └────────────┘ └────────────┘             │
│ Insights     │                                                         │
│ Library      │ Recommended Follow-ups                                  │
│ Settings     │ ┌─────────────────────────────────────────────────────┐ │
│              │ │ Sequoia viewed financials 3x      Send follow-up   │ │
│              │ │ Acme proposal forwarded to 4 users Schedule call    │ │
│              │ │ LP A returned to Q4 report        Notify IR         │ │
│              │ └─────────────────────────────────────────────────────┘ │
│              │ Recent Activity                    Active Rooms         │
└──────────────┴─────────────────────────────────────────────────────┘
```

Mobile Web Viewer:

```text
┌─────────────────────────────┐
│ Acme Capital   Series A Deck│
├─────────────────────────────┤
│                             │
│       Document Page         │
│                             │
├─────────────────────────────┤
│  ‹     Page 4 / 16     ›    │
├─────────────────────────────┤
│ Outline   Download   Ask    │
└─────────────────────────────┘
```

### 6.3 States

| State | Trigger | Visual marker | User/system sees | Exit condition |
|---|---|---|---|---|
| Default | Page load / initial state | Skeleton cards, empty table | Dashboard with "Upload first document" CTA | User uploads a document or creates a link |
| Loading | Async operation starts | Spinner or skeleton after 200ms | Blurred placeholder with spinner | Operation completes or fails |
| Empty | No data condition | Empty illustration + copy | "No documents yet. Upload your first deck." | User uploads a document |
| Active | User interaction | Highlighted row, open panel | Selected document or link with actions | User navigates away or closes panel |
| Error | Failure condition | Red banner/toast with icon | Specific error message + retry CTA | User retries or dismisses |
| Revoked | Sender action | Red "Revoked" badge | Link status changed, viewer blocked | Sender reactivates (v1: recreate only) |
| Expired | Time passes | Gray "Expired" badge | Viewer shows "This link has expired" | Sender extends expiration |
| Hot | Score >= 70 | Flame icon + "Hot" badge | Recipient card highlighted in dashboard | Score drops below 70 |

### 6.4 Failure Paths

| Failure | Cause | User/system response | Recovery |
|---|---|---|---|
| Upload fails | Network timeout or unsupported file type | Toast: "Upload failed: [reason]" | Retry upload or convert file |
| Link expired | `expires_at` passed | Viewer shows "This link has expired" with contact sender option | Sender extends expiration in Link Detail |
| Access denied | Email not in allowlist or revoked | Viewer shows "You don't have access" with request-access form | Sender approves request or updates allowlist |
| Email verification failed | Wrong/expired OTP | Viewer shows "Verification failed, please try again" | Resend verification code |
| Analytics event lost | Network failure on recipient side | Event queued and retried; fallback heartbeat | System reconciles on next viewer ping |
| Score calculation stale | Worker backlog | Dashboard shows cached score with "Last updated" timestamp | Score refreshes when worker catches up |

### 6.5 Module Experience Contracts

#### Module A: Document Management

**Part a) Shape and Flow**
- Surface: Desktop Web Admin
- Interface: Documents list table + Document Detail tabs (Overview / Pages / Links / Versions / Settings)
- Normal flow:
  1. User clicks "Upload" or drags a file.
  2. System uploads to object storage and creates document + version records.
  3. Document appears in list with status (processing → ready).
  4. User clicks document to view detail and create Smart Links.
- Failure paths:
  - Failure: File type unsupported.
    - Response: Toast with supported formats.
    - Recovery: User selects a supported file.
  - Failure: Processing timeout.
    - Response: Status badge "Failed" with retry.
    - Recovery: User retries processing or re-uploads.

**Part b) States**

| State | Trigger | Visual marker | User/system sees | Exit condition |
|---|---|---|---|---|
| Uploading | File dropped | Progress bar | "Uploading Series A Deck.pdf 45%" | Upload completes |
| Processing | Upload done | Spinner + "Processing" badge | "Extracting pages..." | Processing succeeds/fails |
| Ready | Processing done | Green "Ready" badge | Document with thumbnail, links count | Archived or deleted |
| Failed | Processing error | Red "Failed" badge | Error message + retry | User retries |

**Part c) Data Dependencies**
- Reads: documents, document_versions, document_pages, smart_links counts, intent_scores aggregates.
- Writes: documents, document_versions, object storage.
- Data flow: Document metadata owns versions; versions own pages. Current version is denormalized on documents.

**Part d) Product Decisions**

| Decision | Safe default | What would change this |
|---|---|---|
| Support video files in v1? | Yes, but only storage + basic playback; no page-level analytics. | User research shows video is a core pitch format. |
| Auto-extract text for search? | Yes for PDF/PPT; no for scanned images in v1. | OCR service cost and accuracy data. |

**Part e) Boundary Cases**
- Empty workspace: show onboarding CTA, hide table.
- Very large PDF (>100 MB): stream upload, show progress, cap page extraction.
- Duplicate filename: allow duplicates, display version indicator.
- Deleted document: soft delete, asynchronously clean storage.

#### Module B: Smart Link Creation and Sharing

**Part a) Shape and Flow**
- Surface: Desktop Web Admin
- Interface: Create Smart Link form with presets (Fast Share / Balanced / High Security) + custom controls
- Normal flow:
  1. User selects document.
  2. Names link and enters recipient email.
  3. Chooses preset or custom access mode.
  4. Sets download, watermark, expiration.
  5. Reviews recipient friction level.
  6. Creates link and copies URL.
- Failure paths:
  - Failure: Recipient email domain blocked by workspace policy.
    - Response: Inline warning with policy explanation.
    - Recovery: User chooses allowed domain or contacts admin.
  - Failure: Expiration in the past.
    - Response: Form validation error.
    - Recovery: User picks a future date.

**Part b) States**

| State | Trigger | Visual marker | User/system sees | Exit condition |
|---|---|---|---|---|
| Draft | Form open | Default form | Preset selection + controls | User submits |
| Validating | Submit clicked | Loading spinner on Create button | "Creating link..." | Validation passes/fails |
| Created | API success | Modal/toast with copy button | Link URL + copy CTA | User closes/copies |
| Revoked | Revoke clicked | Red "Revoked" badge | Link no longer accessible | N/A in v1 |

**Part c) Data Dependencies**
- Reads: documents, document_versions, workspace default_security_preset.
- Writes: smart_links, smart_link_recipients, activity_events.
- Data flow: Link points to a document version; recipient record tracks intended viewer.

**Part d) Product Decisions**

| Decision | Safe default | What would change this |
|---|---|---|
| Default access mode | Email verification | Beta data shows public links have higher open rates and acceptable security. |
| Allow multiple links per document? | Yes | Users need investor-specific tracking. |

**Part e) Boundary Cases**
- Same recipient on multiple links: separate records, separate scores.
- Revoked link accessed: clear block page, no content leak.
- Password-protected link: hash verified server-side, never return hash.

#### Module C: Recipient Viewer

**Part a) Shape and Flow**
- Surface: Mobile + Desktop Web Viewer
- Interface: Compact top bar (sender brand + doc title), document canvas, bottom navigation, optional download/ask buttons
- Normal flow:
  1. Recipient clicks link.
  2. System resolves slug and checks status.
  3. Access mode enforced (public / email verify / password / allowlist).
  4. Viewer loads document pages.
  5. Recipient navigates pages; events recorded.
  6. Recipient downloads if allowed or asks a question.
- Failure paths:
  - Failure: Link revoked or expired.
    - Response: Block page with reason and "Contact sender" button.
    - Recovery: Recipient contacts sender; sender reactivates or extends.
  - Failure: Email not allowed.
    - Response: "This link is restricted" with request access form.
    - Recovery: Sender approves request.

**Part b) States**

| State | Trigger | Visual marker | User/system sees | Exit condition |
|---|---|---|---|---|
| Access check | Link opened | Spinner | "Checking access..." | Allowed / blocked |
| Verify email | Email verification required | Input field + send button | "Enter the code sent to your email" | Code verified |
| Loading document | Access granted | Skeleton page | "Loading document..." | Document rendered |
| Viewing | Document loaded | Page rendered with page indicator | Document page + navigation | User closes or navigates away |
| Blocked | Access denied | Lock icon + red copy | Reason + contact sender / request access | User contacts sender |

**Part c) Data Dependencies**
- Reads: smart_links, smart_link_recipients, access_grants, document_versions, document_pages, deal_room_files, deal_room_access_rules.
- Writes: view_sessions, activity_events, page_view_events, download_events, access_grants.
- Data flow: Viewer is read-heavy; events are append-only writes.

**Part d) Product Decisions**

| Decision | Safe default | What would change this |
|---|---|---|
| Require account for viewers? | No unless policy requires | Enterprise customers demand audit identity. |
| Show privacy notice | Yes, a short footer/disclosure | Regulatory feedback or user complaints. |

**Part e) Boundary Cases**
- Mobile viewport: bottom bar thumb-friendly, pinch zoom.
- Offline: cache current page, queue events.
- Screen reader: alt text for pages, keyboard navigation.
- Very large page image: lazy load, downsample.

#### Module D: Analytics Dashboard

**Part a) Shape and Flow**
- Surface: Desktop Web Admin + Mobile Management Lite
- Interface: Dashboard cards (Hot Signals / Opens / Risks), Recommended Follow-ups list, Recent Activity feed
- Normal flow:
  1. Sender opens Dashboard.
  2. System aggregates recent events and scores.
  3. Cards and recommendations render.
  4. Sender clicks a recommendation to open Link/Contact detail.
  5. Sender takes follow-up action.
- Failure paths:
  - Failure: Score worker lag.
    - Response: Show "Last updated X min ago" with refresh button.
    - Recovery: System catches up; user refreshes.
  - Failure: No data yet.
    - Response: Empty state with upload CTA.
    - Recovery: User uploads and shares first document.

**Part b) States**

| State | Trigger | Visual marker | User/system sees | Exit condition |
|---|---|---|---|---|
| Empty | No events | Empty illustration + upload CTA | "Upload your first deck to see signals" | User creates link and gets opens |
| Loading | Dashboard open | Skeleton cards | Shimmer placeholders | Data loaded |
| Hot signals | Recent hot events | Flame badges | List of hot recipients | Events age out |
| Risk alert | Blocked/expired/suspicious | Red alert card | Risk summary + review CTA | Risk resolved |

**Part c) Data Dependencies**
- Reads: activity_events, page_view_events, view_sessions, intent_scores, smart_links, contacts, recommendations.
- Writes: recommendations (action assistant), notifications.
- Data flow: Raw events → materialized scores/recommendations → dashboard reads.

**Part d) Product Decisions**

| Decision | Safe default | What would change this |
|---|---|---|
| Real-time vs batched alerts | Real-time for first open and hot score; daily digest optional later | Users complain about noise. |
| Score refresh frequency | On every significant event, debounced 30s | Performance issues. |

**Part e) Boundary Cases**
- Many events in one minute: aggregate, don't flood UI.
- Score ties: sort by recency.
- Suspicious access (unusual geo): surface risk card.

#### Module E: Deal Rooms

**Part a) Shape and Flow**
- Surface: Desktop Web Admin + Mobile Web Viewer
- Interface: Room list → Create from template → Room Detail tabs (Overview / Files / Recipients / Activity / Q&A / Settings)
- Normal flow:
  1. User creates room from template.
  2. System creates folders.
  3. User uploads/assigns documents to folders.
  4. User invites recipients.
  5. Recipients access room and view files.
  6. User tracks room activity.
- Failure paths:
  - Failure: Recipient tries to access a folder they don't have permission for.
    - Response: Folder hidden or disabled with tooltip.
    - Recovery: Sender updates access rules.
  - Failure: Room invitation email bounces.
    - Response: Bounce status on recipient row.
    - Recovery: Sender corrects email and re-invites.

**Part b) States**

| State | Trigger | Visual marker | User/system sees | Exit condition |
|---|---|---|---|---|
| Draft | Room created | "Draft" badge | Room not yet published | User publishes |
| Active | Room published | "Active" badge | Recipients can access | Archived / expired |
| Archived | Sender archives | "Archived" badge | Read-only historical view | Restored (future) |
| Pending access | Recipient requests access | Yellow "Pending" badge | Sender sees request in dashboard | Approved / denied |

**Part c) Data Dependencies**
- Reads: deal_rooms, deal_room_folders, deal_room_files, deal_room_members, deal_room_access_rules, documents, document_versions.
- Writes: deal_rooms, deal_room_folders, deal_room_files, deal_room_members, deal_room_access_rules, activity_events.
- Data flow: Room owns folders and members; files reference document versions; access rules evaluated per file/folder.

**Part d) Product Decisions**

| Decision | Safe default | What would change this |
|---|---|---|
| Q&A in v1? | Yes, simplified (question + answer, no threading) | Users demand threading and assignments. |
| Folder-level permissions | Yes | Users find room-level only too coarse. |

**Part e) Boundary Cases**
- Nested folders: support one level in v1, more later.
- Room with no files: show empty state and upload CTA.
- Member removed: revoke access grants immediately.

## 7. Data and Integration Contract

### 7.1 Core Data Objects

```jsonc
{
  "version": "1",
  "user": {
    "id": "uuid",
    "email": "user@example.com",
    "name": "Jane Doe",
    "avatar_url": "https://...",
    "created_at": "2026-01-01T00:00:00Z"
  },
  "workspace": {
    "id": "uuid",
    "name": "Acme Capital",
    "slug": "acme-capital",
    "mode": "founder",
    "default_security_preset": {},
    "created_at": "2026-01-01T00:00:00Z"
  },
  "document": {
    "id": "uuid",
    "workspace_id": "uuid",
    "name": "Series A Deck.pdf",
    "status": "ready",
    "current_version_id": "uuid",
    "metadata": {}
  },
  "document_version": {
    "id": "uuid",
    "document_id": "uuid",
    "version_number": 1,
    "storage_bucket": "dealsignal-files",
    "storage_key": "path/to/file.pdf",
    "mime_type": "application/pdf",
    "file_size_bytes": 1048576,
    "checksum_sha256": "abc123",
    "page_count": 12,
    "processing_status": "ready"
  },
  "smart_link": {
    "id": "uuid",
    "workspace_id": "uuid",
    "document_id": "uuid",
    "document_version_id": "uuid",
    "name": "Sequoia - Sarah Chen",
    "slug": "abc123xyz",
    "access_mode": "email_verification",
    "download_policy": "allowed",
    "watermark_enabled": true,
    "expires_at": "2026-02-01T00:00:00Z",
    "revoked_at": null,
    "status": "active"
  },
  "view_session": {
    "id": "uuid",
    "smart_link_id": "uuid",
    "contact_id": "uuid",
    "recipient_email": "sarah@sequoiacap.com",
    "ip_address": "1.2.3.4",
    "user_agent": "Mozilla/5.0",
    "started_at": "2026-01-01T12:00:00Z",
    "ended_at": null
  },
  "page_view_event": {
    "id": "uuid",
    "view_session_id": "uuid",
    "document_version_id": "uuid",
    "page_number": 8,
    "visible_started_at": "2026-01-01T12:05:00Z",
    "visible_ended_at": "2026-01-01T12:07:00Z",
    "duration_ms": 120000
  },
  "intent_score": {
    "id": "uuid",
    "score_type": "investor_intent",
    "score": 84,
    "label": "hot",
    "explanation": "Hot because this recipient viewed the pricing page 3 times...",
    "factors": {},
    "contact_id": "uuid",
    "calculated_at": "2026-01-01T12:10:00Z"
  }
}
```

### 7.2 External Interfaces

| Interface | Direction | Contract | Failure mode |
|---|---|---|---|
| Object storage (S3/R2/MinIO) | Outbound write + read | Upload via presigned URL or direct SDK; files served through app proxy | Upload retry; fallback to direct URL only if signed and short-lived |
| Email provider (SendGrid/Resend/AWS SES) | Outbound | Send verification codes, alerts, invites via SMTP/API | Queue and retry; alert admin on persistent failure |
| HubSpot API | Outbound | OAuth 2.0; write timeline events and tasks | Retry with exponential backoff; mark integration error |
| Salesforce API | Outbound | OAuth 2.0; write Task/Event objects | Retry with exponential backoff; mark integration error |
| Slack Web API | Outbound | OAuth 2.0; post messages to channel | Retry; log failure |
| GeoIP service | Outbound | Resolve IP to country/region/city/device | Degrade gracefully to unknown on failure |

### 7.3 Data Retention, Privacy, and Permissions

- Recipient IP addresses retained for 90 days by default; workspace admins can configure shorter periods in P2.
- Passwords and integration tokens stored hashed or encrypted at application level.
- Deleted documents are soft-deleted in DB; object storage cleanup is asynchronous.
- Analytics disclosure shown in recipient viewer.
- Workspace owners own first-party engagement data; DealSignal does not sell recipient data.
- GDPR deletion requests supported via admin action in P2.

### 7.4 Architecture, Package Sizes, and Replaceability

Layered architecture:

```
+--------------------------------------------------+
| Presentation: React web app, mobile viewer, email│
+--------------------------------------------------+
         |
+--------------------------------------------------+
| API Layer: REST/JSON, auth middleware, tenant filter│
+--------------------------------------------------+
         |
+--------------------------------------------------+
| Domain Services: upload, link, viewer, analytics,│
| scoring, notifications, rooms, integrations      │
+--------------------------------------------------+
         |
+--------------------------------------------------+
| Data Layer: PostgreSQL + object storage          │
+--------------------------------------------------+
```

Dependency table:

| Dependency / library | Purpose | Why better than alternative | Package size |
|---|---|---|---|
| PostgreSQL 15+ | Relational data, tenant isolation, JSONB | Schema already designed; ACID for audit events | N/A (service) |
| Object storage (S3/R2/MinIO) | File blobs | Decouples scaling | N/A (service) |
| React 18+ | Web UI | Team familiarity, ecosystem | ~40 kB runtime |
| pdf.js or react-pdf | PDF rendering in browser | Standard, well-maintained | ~200 kB |
| Tailwind CSS | Styling | Rapid UI iteration | ~0 kB runtime (purge) |
| BullMQ / pg-boss | Background jobs | Postgres-backed reliability | ~100 kB |
| SendGrid/Resend SDK | Email delivery | Reliable transactional email | ~50 kB |

No new runtime dependencies are mandatory beyond the existing stack assumptions. Use project-preferred libraries where they already exist.

Biggest architecture risk: Real-time intent scoring under high event volume. If scoring worker cannot keep up, dashboard data becomes stale and alerts delayed. Mitigation: score calculation is idempotent and can be batched; cache aggressively.

Replaceability:

| Decision | Recommended default | Acceptable substitute | Invariant that must not change | Risk if wrong |
|---|---|---|---|---|
| Object storage provider | S3-compatible (R2/MinIO/S3) | Any S3-compatible API | Files are referenced by bucket+key, never exposed directly | Migration cost and broken links |
| Email provider | Resend or SendGrid | AWS SES, Postmark | Transactional email queue + retry semantics | Lost alerts and verification codes |
| PDF renderer | pdf.js | Server-side image tiles | Page-level analytics events must still fire | Broken analytics or viewer experience |

### 7.5 Output and Delivery Contract

| Output form | Description | Who consumes it |
|---|---|---|
| Smart Link URL | Shareable controlled link | Sender distributes to recipients |
| Document viewer page | Branded recipient reading experience | Recipients |
| Dashboard view | Hot signals, recent activity, follow-ups | Senders |
| Link/Document/Room analytics | Timelines, page performance, intent scores | Senders |
| Email alert | First-open or hot-score notification | Senders |
| CSV export | Analytics data for offline reporting | Senders, admins |
| Deal Room portal | Grouped materials with permissions | Recipients, LPs, buyers |

## 8. Risks, Unknowns, and Open Decisions

### 8.1 Risks

| Risk | Impact | Mitigation | Owner |
|---|---|---|---|
| Users perceive tracking as creepy | Reputation damage; lower open rates | Transparent privacy disclosure; professional messaging; opt-out where possible | Product |
| Security promises overinterpreted | Legal exposure; customer churn | Explicitly state screenshots cannot be fully prevented; position watermark as deterrence | Legal / Product |
| Founder market is seasonal | Churn spikes between fundraises | Expand to investor updates and sales proposals | GTM |
| DocSend owns category awareness | Higher CAC; slower organic growth | SEO for "DocSend alternative"; founder-specific landing pages; lower-friction viewer | Marketing |
| Real-time scoring can't keep up | Stale dashboard; delayed alerts | Idempotent batch scoring; caching; worker autoscaling | Engineering |
| Enterprise security requirements slow adoption | Sales cycles extend | Start with founders/emerging funds before enterprise procurement | GTM |

### 8.2 Known Unknowns

| Unknown | Why it matters | Safe default until resolved |
|---|---|---|
| HubSpot/Salesforce API rate limits | Affects batching and retry strategy | Batch writes, respect 429s, exponential backoff |
| Dynamic watermarking feasibility on downloads | Affects P0 watermark scope | Implement viewer-layer watermark first; defer file-level embedding to P1 |
| Accurate page-level time measurement across devices | Affects scoring accuracy | Use visibility API + heartbeat; ignore background tabs |
| Email deliverability for verification codes | Affects viewer conversion | Use reputable provider; monitor bounce/spam rates |
| Optimal Founder Plan link limit | Affects conversion | Start with 10 active links; A/B test |
| Investor willingness to open tracked links | Affects core value proposition | Transparent disclosure; low-friction default; measure open rates |

### 8.3 Open Decisions

| Decision | Options | Default recommendation | Change signal |
|---|---|---|---|
| First wedge market | Founders vs sales | Founders | Sales teams show stronger activation in beta |
| Watermark as free or paid | Free (deterrence) vs paid conversion feature | Free in Founder Plan; advanced templates paid | Free hurts conversion or paid hurts adoption |
| Recipient privacy disclosure | Standardized vs configurable | Standardized with workspace name visible | Regulated customers demand customization |
| AI assistance depth | Action recommendations vs email drafts | Recommendations in v1; AI drafts in P2 | Users distrust or strongly demand AI drafts |
| Public no-auth links in regulated workspaces | Allowed vs blocked by policy | Workspace-level toggle, default off | Compliance feedback |

### 8.4 Product Metrics and Performance Targets

| Target | Goal value | Measurement method | Degradation threshold | Owner |
|---|---:|---|---:|---|
| Viewer first meaningful render | < 2.0 s | Chrome DevTools Lighthouse / web-vitals on typical 10-page PDF | > 3.0 s | Frontend |
| Page navigation after load | < 500 ms | Manual/browser automation timing | > 1.0 s | Frontend |
| Analytics event visible in dashboard | < 60 s | End-to-end test: open link, wait for event | > 2 min | Backend |
| Intent score refresh after key event | < 60 s | Backend test / dashboard observation | > 2 min | Backend |
| Link revocation propagation | < 5 s | Open revoked link immediately after revoke | > 10 s | Backend |
| Upload success rate | > 95% | Backend logs for supported file types | < 90% | Backend |
| Email verification delivery | > 98% | Email provider analytics | < 95% | Backend |
| Beta activation (create first link) | > 70% of signups | Product analytics | < 50% | Product |
| Beta return after first open | > 50% | Product analytics | < 30% | Product |

## 9. Verification Matrix

| ID | Requirement | Evidence | Check method | Required before ship? |
|---|---|---|---|---|
| US-001 | Upload supported files | Uploaded file appears in DB and storage | Unit test + manual browser upload | Yes |
| US-001 | Upload progress and errors | Screen recording / toast visible | Browser manual test | Yes |
| US-002 | Create Smart Link with settings | DB record + copyable URL | API test + browser test | Yes |
| US-002 | Recipient friction shown | Screenshot of form | Browser manual test | Yes |
| US-003 | Open link without account | Viewer loads | Browser manual test (incognito) | Yes |
| US-003 | Mobile viewer usable | Screenshot on mobile viewport | Browser dev tools + real device | Yes |
| US-003 | Expired/denied messages | Block page screenshot | Manual test with expired link | Yes |
| US-004 | First open recorded | activity_events row | DB query / API response | Yes |
| US-004 | Page view duration recorded | page_view_events row | DB query / API response | Yes |
| US-004 | Event latency < 60 s | Timestamp diff test | End-to-end test | Yes |
| US-005 | Score 0-100 with label | intent_scores row | DB query + dashboard screenshot | Yes |
| US-005 | Score updates on activity | Score changes after simulated events | Backend test | Yes |
| US-006 | Create Deal Room from template | DB records + UI | Browser manual test | Yes |
| US-007 | Watermark visible | Viewer screenshot | Browser manual test | Yes |
| US-008 | First-open email alert | Email inbox / notification log | Manual test + email provider logs | Yes |
| US-008 | Hot-score email alert | Email inbox after hot event | Manual test | Yes |
| HC-1 | Workspace isolation | Cross-workspace request returns 403 | Integration test | Yes |
| HC-2 | Access before content | Revoked link never returns bytes | Manual test + network trace | Yes |
| HC-3 | Append-only events | Update attempt fails or is blocked | Unit/integration test | Yes |
| FR-30 | CSV export | Downloaded CSV file with expected columns | Manual test | Yes |

## 10. Suggested Issue Mapping

### Issue 1: Project scaffold and database schema
- Source: HC-1, database-model.md, sql/schema.sql
- Type: infra
- Priority: high
- Dependencies: None
- Why this slice: Foundation for all other work.
- Acceptance Criteria:
  - [ ] PostgreSQL schema creates all P0 tables and indexes.
  - [ ] Migration tooling configured.
  - [ ] Lint/build passes.
- Validation:
  - [ ] DB contains users, workspaces, documents, smart_links tables.
- Loop-it notes:
  - Branch hint: feat/issue-1-scaffold
  - Risk class: build_failure

### Issue 2: Auth and workspace model
- Source: HC-1
- Type: backend
- Priority: high
- Dependencies: Issue 1
- Why this slice: Required for tenant isolation and user identity.
- Acceptance Criteria:
  - [ ] Register/login via email.
  - [ ] Workspace creation and switching.
  - [ ] Role-based membership.
- Validation:
  - [ ] Cross-workspace access returns 403.
- Loop-it notes:
  - Branch hint: feat/issue-2-auth
  - Risk class: test_failure

### Issue 3: Document upload and version backend
- Source: US-001, FR-1, FR-27
- Type: backend
- Priority: high
- Dependencies: Issue 1, Issue 2
- Why this slice: Core asset for all sharing features.
- Acceptance Criteria:
  - [ ] Upload supported files.
  - [ ] Store metadata and object storage keys.
  - [ ] Version history.
- Validation:
  - [ ] DB records created after upload.
- Loop-it notes:
  - Branch hint: feat/issue-3-upload
  - Risk class: build_failure

### Issue 4: Document processing pipeline
- Source: US-001, FR-12
- Type: backend
- Priority: high
- Dependencies: Issue 3
- Why this slice: Enables page-level analytics.
- Acceptance Criteria:
  - [ ] Extract pages from PDF.
  - [ ] Generate thumbnails and text excerpts.
  - [ ] Status machine: uploaded/processing/ready/failed.
- Validation:
  - [ ] 10-page PDF produces 10 document_pages rows.
- Loop-it notes:
  - Branch hint: feat/issue-4-processing
  - Risk class: unknown

### Issue 5: Documents list and detail UI
- Source: US-001, UI/page-prototypes.md Section 6-7
- Type: frontend
- Priority: high
- Dependencies: Issue 3, Issue 4
- Why this slice: Sender's primary document management surface.
- Acceptance Criteria:
  - [ ] List documents with status, links, opens.
  - [ ] Detail tabs: Overview, Pages, Links, Versions, Settings.
- Validation:
  - [ ] Browser test shows uploaded documents.
- Loop-it notes:
  - Branch hint: feat/issue-5-documents-ui
  - Risk class: test_failure

### Issue 6: Smart Link creation and permissions backend
- Source: US-002, FR-2~FR-10
- Type: backend
- Priority: high
- Dependencies: Issue 3
- Why this slice: Core sharing primitive.
- Acceptance Criteria:
  - [ ] Create unique slug links.
  - [ ] Access modes: public, email_verification, allowlist, password.
  - [ ] Expiration, revocation, download policy, watermark.
- Validation:
  - [ ] DB records created; expired/revoked links blocked.
- Loop-it notes:
  - Branch hint: feat/issue-6-smart-links
  - Risk class: build_failure

### Issue 7: Smart Link creation form UI
- Source: US-002, UI/page-prototypes.md Section 8
- Type: frontend
- Priority: high
- Dependencies: Issue 6
- Why this slice: Sender-facing sharing flow.
- Acceptance Criteria:
  - [ ] Security presets and custom controls.
  - [ ] Recipient friction indicator.
  - [ ] Copy link after creation.
- Validation:
  - [ ] Browser test creates link and copies URL.
- Loop-it notes:
  - Branch hint: feat/issue-7-link-form
  - Risk class: test_failure

### Issue 8: Link detail and management UI
- Source: US-002, UI/page-prototypes.md Section 9
- Type: frontend
- Priority: high
- Dependencies: Issue 6
- Why this slice: Sender operates existing links.
- Acceptance Criteria:
  - [ ] Show status, security, copy, revoke.
  - [ ] Activity summary.
- Validation:
  - [ ] Revoke action blocks viewer access.
- Loop-it notes:
  - Branch hint: feat/issue-8-link-detail
  - Risk class: test_failure

### Issue 9: Viewer access control and email verification
- Source: US-003, FR-6, FR-29
- Type: fullstack
- Priority: high
- Dependencies: Issue 6
- Why this slice: Security gate before content.
- Acceptance Criteria:
  - [ ] Resolve slug, check status, enforce access mode.
  - [ ] Email verification flow.
  - [ ] Clear block pages.
- Validation:
  - [ ] Revoked/expired links show block page without content.
- Loop-it notes:
  - Branch hint: feat/issue-9-viewer-access
  - Risk class: test_failure

### Issue 10: Document viewer rendering and navigation
- Source: US-003, UI/page-prototypes.md Section 10.2
- Type: frontend
- Priority: high
- Dependencies: Issue 4, Issue 9
- Why this slice: Recipient reading experience.
- Acceptance Criteria:
  - [ ] Render PDF pages, navigate, zoom.
  - [ ] Mobile responsive.
  - [ ] Download only when allowed.
- Validation:
  - [ ] Browser and mobile viewport test.
- Loop-it notes:
  - Branch hint: feat/issue-10-viewer
  - Risk class: test_failure

### Issue 11: Page-level analytics events backend
- Source: US-004, FR-11, FR-12
- Type: backend
- Priority: high
- Dependencies: Issue 9, Issue 10
- Why this slice: Core signal data.
- Acceptance Criteria:
  - [ ] view_sessions, page_view_events, activity_events.
  - [ ] Latency < 10s.
- Validation:
  - [ ] Events recorded after browsing.
- Loop-it notes:
  - Branch hint: feat/issue-11-events
  - Risk class: test_failure

### Issue 12: Download and access-denied event capture
- Source: US-004, FR-13, FR-29
- Type: backend
- Priority: high
- Dependencies: Issue 9
- Why this slice: Audit and analytics completeness.
- Acceptance Criteria:
  - [ ] download_events for allowed/blocked.
  - [ ] access_denied events.
- Validation:
  - [ ] Blocked download creates record.
- Loop-it notes:
  - Branch hint: feat/issue-12-download-events
  - Risk class: test_failure

### Issue 13: Recipient timeline and analytics UI
- Source: US-004, FR-14, FR-15
- Type: fullstack
- Priority: high
- Dependencies: Issue 11, Issue 12
- Why this slice: Sender sees the signal.
- Acceptance Criteria:
  - [ ] Activity timeline per link/recipient.
  - [ ] Page analytics.
  - [ ] Forward/new viewer detection.
- Validation:
  - [ ] Browser test shows timeline after events.
- Loop-it notes:
  - Branch hint: feat/issue-13-analytics-ui
  - Risk class: test_failure

### Issue 14: Intent score calculation and explanation
- Source: US-005, FR-16, FR-17
- Type: backend
- Priority: high
- Dependencies: Issue 11, Issue 12
- Why this slice: Converts data into prioritized action.
- Acceptance Criteria:
  - [ ] 0-100 score, Cold/Warm/Hot labels.
  - [ ] Natural language explanation.
  - [ ] Segment-specific types.
- Validation:
  - [ ] Simulated activity changes score.
- Loop-it notes:
  - Branch hint: feat/issue-14-scoring
  - Risk class: test_failure

### Issue 15: Dashboard and hot signals UI
- Source: US-005, US-008, UI/page-prototypes.md Section 5
- Type: frontend
- Priority: high
- Dependencies: Issue 13, Issue 14
- Why this slice: Daily operating surface for senders.
- Acceptance Criteria:
  - [ ] Hot signals, opens, risks cards.
  - [ ] Recommended follow-ups.
  - [ ] Segment variations.
- Validation:
  - [ ] Hot events appear on dashboard.
- Loop-it notes:
  - Branch hint: feat/issue-15-dashboard
  - Risk class: test_failure

### Issue 16: Basic dynamic watermark
- Source: US-007, FR-10
- Type: backend
- Priority: medium
- Dependencies: Issue 10
- Why this slice: Deterrence and traceability.
- Acceptance Criteria:
  - [ ] Viewer-layer watermark with email and timestamp.
  - [ ] Toggle in link settings.
- Validation:
  - [ ] Screenshot shows watermark.
- Loop-it notes:
  - Branch hint: feat/issue-16-watermark
  - Risk class: unknown

### Issue 17: Email alert system
- Source: US-008, FR-22
- Type: backend
- Priority: medium
- Dependencies: Issue 14
- Why this slice: Timely sender notifications.
- Acceptance Criteria:
  - [ ] First-open and hot-score alerts.
  - [ ] Queue and retry.
- Validation:
  - [ ] Email received after events.
- Loop-it notes:
  - Branch hint: feat/issue-17-alerts
  - Risk class: test_failure

### Issue 18: Basic Deal Room backend
- Source: US-006, FR-18~FR-21
- Type: backend
- Priority: medium
- Dependencies: Issue 3, Issue 2
- Why this slice: Multi-document sharing primitive.
- Acceptance Criteria:
  - [ ] Rooms, folders, files, members, access rules.
  - [ ] Activity logs.
- Validation:
  - [ ] DB records created; access rules enforced.
- Loop-it notes:
  - Branch hint: feat/issue-18-rooms
  - Risk class: build_failure

### Issue 19: Deal Room creation and management UI
- Source: US-006, UI/page-prototypes.md Section 12-14
- Type: frontend
- Priority: medium
- Dependencies: Issue 18
- Why this slice: Sender operates rooms.
- Acceptance Criteria:
  - [ ] Room list, create from template, detail tabs.
  - [ ] Files, recipients, activity, Q&A.
- Validation:
  - [ ] Browser test creates room and invites member.
- Loop-it notes:
  - Branch hint: feat/issue-19-rooms-ui
  - Risk class: test_failure

### Issue 20: CSV export
- Source: FR-30
- Type: backend
- Priority: medium
- Dependencies: Issue 13
- Why this slice: Offline reporting need.
- Acceptance Criteria:
  - [ ] Export link/document/room analytics as CSV.
  - [ ] Reasonable performance for thousands of rows.
- Validation:
  - [ ] Downloaded CSV has expected columns.
- Loop-it notes:
  - Branch hint: feat/issue-20-csv
  - Risk class: test_failure

### Issue 21: 高级水印模板
- Source: P1: Advanced watermark templates
- Type: backend
- Priority: medium
- Dependencies: Issue 16
- Why this slice: 扩展水印能力，支持自定义水印文本、位置、透明度、颜色，以及下载文件的水印嵌入。
- Acceptance Criteria:
  - [ ] 支持配置水印内容模板（邮箱、时间、IP、自定义文本）
  - [ ] 支持调整水印位置与样式
  - [ ] 下载 PDF 时可在文件上嵌入水印
  - [ ] 不同链接可使用不同水印模板
- Validation:
  - [ ] 配置自定义水印后 viewer 与下载文件均显示对应水印
- Loop-it notes:
  - Branch hint: feat/issue-21-高级水印模板
  - Risk class: unknown

### Issue 22: Slack 提醒集成
- Source: P1: Slack alerts, FR-23
- Type: backend
- Priority: medium
- Dependencies: Issue 17
- Why this slice: 连接 Slack workspace，将首次打开、Hot score、转发检测等事件发送到指定频道。
- Acceptance Criteria:
  - [ ] 用户可通过 OAuth 连接 Slack
  - [ ] 可配置提醒事件类型与目标频道
  - [ ] Hot score 事件触发 Slack 消息
  - [ ] 消息包含链接到 DealSignal 的按钮
- Validation:
  - [ ] 配置后模拟 Hot score 事件，Slack 频道收到消息
- Loop-it notes:
  - Branch hint: feat/issue-22-slack-提醒集成
  - Risk class: test_failure

### Issue 23: HubSpot / Salesforce 连接
- Source: US-009, FR-24
- Type: backend
- Priority: medium
- Dependencies: Issue 2
- Why this slice: 实现 CRM 集成连接，支持 OAuth 授权并存储 access token，建立 DealSignal 对象与 CRM 对象的映射。
- Acceptance Criteria:
  - [ ] 支持连接 HubSpot 与 Salesforce
  - [ ] 存储加密后的 integration credentials
  - [ ] 支持将 contact / account / smart_link / deal_room 映射到 CRM 对象
  - [ ] 连接状态可显示在设置页
- Validation:
  - [ ] 完成 OAuth 后 integrations 表生成 connected 记录
  - [ ] crm_mappings 可保存对象映射
- Loop-it notes:
  - Branch hint: feat/issue-23-hubspot---salesforce-连接
  - Risk class: test_failure

### Issue 24: CRM 活动同步
- Source: US-009, FR-25
- Type: backend
- Priority: medium
- Dependencies: Issue 23, Issue 11
- Why this slice: 将文档打开、高意图等事件写入 CRM timeline，并在启用时自动创建跟进任务。
- Acceptance Criteria:
  - [ ] Smart Link 可与 CRM deal / contact 关联
  - [ ] 文档打开事件写入关联 CRM 对象 timeline
  - [ ] Hot score 事件触发 CRM task 创建（可配置）
  - [ ] 失败同步进入重试队列
- Validation:
  - [ ] 模拟文档打开后，HubSpot/Salesforce timeline 出现对应事件
- Loop-it notes:
  - Branch hint: feat/issue-24-crm-活动同步
  - Risk class: test_failure

### Issue 25: 数据室模板
- Source: P1: Deal Room templates
- Type: fullstack
- Priority: medium
- Dependencies: Issue 18
- Why this slice: 为 Seed Fundraising、Series A、LP Update、M&A Diligence、Enterprise Sales 等场景预置数据室模板与默认文件夹。
- Acceptance Criteria:
  - [ ] 创建 room 时可选择模板
  - [ ] 模板自动创建默认文件夹结构
  - [ ] 模板附带推荐的默认权限与安全设置
  - [ ] 模板可在设置中维护
- Validation:
  - [ ] 选择 Seed Fundraising 模板后自动创建 Pitch/Financials/Legal 等文件夹
- Loop-it notes:
  - Branch hint: feat/issue-25-数据室模板
  - Risk class: test_failure

### Issue 26: 内容库后端
- Source: US-010, FR-26, FR-28
- Type: backend
- Priority: medium
- Dependencies: Issue 3
- Why this slice: 实现内容库的数据模型，支持文档状态 Draft / In Review / Approved / Archived、集合管理与使用统计。
- Acceptance Criteria:
  - [ ] library_collections 与 library_items 表可用
  - [ ] 文档状态可在 Draft / In Review / Approved / Archived 间切换
  - [ ] 支持将文档加入集合
  - [ ] 可追踪文档被使用的链接数与打开数
- Validation:
  - [ ] 标记文档为 Approved 后状态更新并记录审批人
- Loop-it notes:
  - Branch hint: feat/issue-26-内容库后端
  - Risk class: build_failure

### Issue 27: 内容库 UI
- Source: US-010, UI/page-prototypes.md Section 17
- Type: frontend
- Priority: medium
- Dependencies: Issue 26
- Why this slice: 实现 Content Library 页面，支持按 Approved / Drafts / Archived / Templates 分类查看、审批、归档与使用统计。
- Acceptance Criteria:
  - [ ] 内容库页面展示文档状态与集合
  - [ ] Admin 可审批或归档文档
  - [ ] 可配置仅允许从 Approved 内容创建 Smart Link
  - [ ] 展示文档内容表现（链接数、打开数、转化率）
- Validation:
  - [ ] 在浏览器中打开 Content Library 可看到文档列表
  - [ ] 审批后该文档状态变为 Approved
- Loop-it notes:
  - Branch hint: feat/issue-27-内容库-ui
  - Risk class: test_failure

### Issue 28: 行动助手推荐
- Source: P1: Action Assistant recommendations, PRD.md Section 7.6
- Type: backend
- Priority: medium
- Dependencies: Issue 14
- Why this slice: 基于意图评分与行为模式生成下一步行动建议（如跟进时机、推荐材料、建议会议），并展示在 Dashboard 与 Link Detail。
- Acceptance Criteria:
  - [ ] 检测高意图、停滞、异常访问等模式
  - [ ] 生成推荐标题、正文与建议动作
  - [ ] 推荐展示在 Dashboard 与 Link Detail
  - [ ] 用户可 dismiss 或 mark done
- Validation:
  - [ ] 模拟高意图行为后 Dashboard 出现跟进建议
  - [ ] 点击 mark done 后 recommendations 状态更新
- Loop-it notes:
  - Branch hint: feat/issue-28-行动助手推荐
  - Risk class: unknown

### Issue 29: 品牌化阅读器
- Source: P1: Branded viewer
- Type: frontend
- Priority: low
- Dependencies: Issue 10
- Why this slice: 允许工作区在文档 viewer 中展示自定义 logo、品牌色与发送方信息，提升专业形象。
- Acceptance Criteria:
  - [ ] 工作区可上传 logo 与设置主色
  - [ ] viewer 顶部栏展示工作区品牌
  - [ ] 品牌设置不遮挡文档内容
  - [ ] 移动端 viewer 同步展示品牌
- Validation:
  - [ ] 配置品牌后 viewer 页面显示自定义 logo
- Loop-it notes:
  - Branch hint: feat/issue-29-品牌化阅读器
  - Risk class: test_failure

### Issue 30: AI 跟进邮件草稿
- Source: P2: AI follow-up drafts
- Type: backend
- Priority: low
- Dependencies: Issue 28
- Why this slice: 根据收件人行为自动生成个性化跟进邮件草稿，供发送方一键复制或编辑后发送。
- Acceptance Criteria:
  - [ ] 基于行为摘要生成邮件主题与正文
  - [ ] 支持创始人/基金/销售三种语气
  - [ ] 用户可在 Link Detail 查看并复制草稿
  - [ ] 草稿明确标注为 AI 生成，需人工审核后发送
- Validation:
  - [ ] 高意图收件人详情页展示可用的跟进邮件草稿
- Loop-it notes:
  - Branch hint: feat/issue-30-ai-跟进邮件草稿
  - Risk class: unknown

### Issue 31: LP 门户
- Source: P2: LP Portal
- Type: fullstack
- Priority: low
- Dependencies: Issue 18, Issue 29
- Why this slice: 为投资机构提供品牌化 LP 门户，LP 可登录查看 fund deck、季度报告、税务文件等聚合材料。
- Acceptance Criteria:
  - [ ] 可创建 LP Update Room 类型的门户
  - [ ] LP 按账户/联系人权限看到不同内容
  - [ ] 门户首页展示最新报告与未读内容
  - [ ] 支持通知 LP 新内容上线
- Validation:
  - [ ] LP 登录门户后可见被授权的报告列表
- Loop-it notes:
  - Branch hint: feat/issue-31-lp-门户
  - Risk class: unknown

### Issue 32: 自定义域名
- Source: P2: Custom domain
- Type: infra
- Priority: low
- Dependencies: Issue 10, Issue 31
- Why this slice: 支持工作区绑定自定义域名（如 investor.fund.com），使阅读器与门户展示企业自有域名。
- Acceptance Criteria:
  - [ ] 工作区可配置自定义域名
  - [ ] 提供 DNS 验证指引
  - [ ] viewer 链接可通过自定义域名打开
  - [ ] HTTPS 证书自动申请或支持上传
- Validation:
  - [ ] 配置自定义域名后，Smart Link 可通过该域名访问
- Loop-it notes:
  - Branch hint: feat/issue-32-自定义域名
  - Risk class: unknown

### Issue 33: 高级审计导出
- Source: P2: Advanced audit export
- Type: backend
- Priority: low
- Dependencies: Issue 20
- Why this slice: 提供合规级审计导出，包含完整访问日志、IP、设备、下载记录、权限变更等，支持 PDF/CSV。
- Acceptance Criteria:
  - [ ] 可按时间范围导出完整审计日志
  - [ ] 导出包含 IP、设备、邮箱、事件类型、结果
  - [ ] 支持 tamper-evident 摘要或签名（可选）
  - [ ] 导出文件包含工作区与生成时间元数据
- Validation:
  - [ ] 导出审计日志后文件包含所有事件类型
- Loop-it notes:
  - Branch hint: feat/issue-33-高级审计导出
  - Risk class: test_failure

### Issue 34: SSO 单点登录
- Source: P2: SSO
- Type: backend
- Priority: low
- Dependencies: Issue 2
- Why this slice: 支持 SAML / OIDC 单点登录，满足企业客户对工作区成员统一身份管理的需求。
- Acceptance Criteria:
  - [ ] 支持 SAML 2.0 与 OIDC 身份提供商
  - [ ] 管理员可配置 SSO 元数据
  - [ ] SSO 用户首次登录自动加入工作区
  - [ ] 支持强制 SSO 登录
- Validation:
  - [ ] 通过 SSO 登录后成功进入工作区
- Loop-it notes:
  - Branch hint: feat/issue-34-sso-单点登录
  - Risk class: unknown

### Issue 35: SCIM 用户同步
- Source: P2: SCIM
- Type: backend
- Priority: low
- Dependencies: Issue 34
- Why this slice: 提供 SCIM 2.0 接口，允许企业通过身份提供商自动同步用户、分配角色、禁用账户。
- Acceptance Criteria:
  - [ ] 实现 SCIM /Users 与 /Groups 端点
  - [ ] 支持创建、更新、停用用户
  - [ ] 支持通过 group 映射工作区角色
  - [ ] 同步事件记录审计日志
- Validation:
  - [ ] 从 IdP 推送用户后 DealSignal 工作区出现对应成员
- Loop-it notes:
  - Branch hint: feat/issue-35-scim-用户同步
  - Risk class: unknown

### Issue 36: 数据保留策略
- Source: P2: Data retention policies
- Type: backend
- Priority: low
- Dependencies: Issue 11, Issue 12
- Why this slice: 允许企业工作区配置数据保留周期，自动清理过期事件、IP 地址、已删除文件等。
- Acceptance Criteria:
  - [ ] 管理员可设置文档、事件、IP 的保留期限
  - [ ] 系统按策略自动匿名化或删除过期数据
  - [ ] 保留策略变更前通知管理员
  - [ ] 支持 GDPR 删除请求工作流
- Validation:
  - [ ] 设置 30 天事件保留后，过期事件被清理
- Loop-it notes:
  - Branch hint: feat/issue-36-数据保留策略
  - Risk class: unknown

### Issue 37: 高级工作流自动化
- Source: P3: Advanced workflow automation
- Type: backend
- Priority: low
- Dependencies: Issue 28
- Why this slice: 支持用户自定义触发器与动作，如特定页面访问后自动发送邮件、进入数据室后创建 CRM 任务等。
- Acceptance Criteria:
  - [ ] 可视化或配置化规则编辑器
  - [ ] 支持事件触发器：打开、Hot score、下载、进入 room
  - [ ] 支持动作：发送邮件、创建任务、邀请成员、更新 CRM
  - [ ] 规则执行记录可查询
- Validation:
  - [ ] 配置规则后触发事件自动执行对应动作
- Loop-it notes:
  - Branch hint: feat/issue-37-高级工作流自动化
  - Risk class: unknown

### Issue 38: 数据驻留
- Source: P3: Data residency
- Type: infra
- Priority: low
- Dependencies: Issue 1
- Why this slice: 支持企业客户选择数据存储区域（如 US/EU/Asia），满足合规与本地化要求。
- Acceptance Criteria:
  - [ ] 企业工作区可选择数据驻留区域
  - [ ] 文档、事件、数据库按区域隔离
  - [ ] 跨区域访问遵循策略限制
- Validation:
  - [ ] 选择 EU 区域后，该工作区数据存储在 EU
- Loop-it notes:
  - Branch hint: feat/issue-38-数据驻留
  - Risk class: unknown

### Issue 39: 深度 BI 报表
- Source: P3: Deep BI reporting
- Type: backend
- Priority: low
- Dependencies: Issue 13, Issue 14
- Why this slice: 提供多维度 BI 报表：内容转化漏斗、团队表现、账户级 engagement、 cohort 分析等，支持导出与嵌入。
- Acceptance Criteria:
  - [ ] 提供漏斗、趋势、对比等报表视图
  - [ ] 支持按时间、segment、内容类型筛选
  - [ ] 支持导出报表为 CSV/PDF
  - [ ] 性能可支持百万级事件
- Validation:
  - [ ] 生成月度内容表现报表并导出
- Loop-it notes:
  - Branch hint: feat/issue-39-深度-bi-报表
  - Risk class: unknown

### Issue 40: SOC 2 支持工作流
- Source: P3: SOC 2 support workflows
- Type: docs
- Priority: low
- Dependencies: Issue 33, Issue 36
- Why this slice: 整理并实施 SOC 2 合规所需的政策、控制、证据收集与审计导出模板。
- Acceptance Criteria:
  - [ ] 制定访问控制、变更管理、事件响应等政策文档
  - [ ] 实现审计日志不可篡改与导出
  - [ ] 建立定期访问复核工作流
  - [ ] 提供审计师只读导出接口
- Validation:
  - [ ] 可生成 SOC 2 所需的审计证据包
- Loop-it notes:
  - Branch hint: feat/issue-40-soc-2-支持工作流
  - Risk class: unknown

### Issue 41: 企业 DLP 集成
- Source: P3: Enterprise DLP integrations
- Type: backend
- Priority: low
- Dependencies: Issue 36
- Why this slice: 与常见 DLP/CASB 方案集成，支持内容扫描、敏感数据检测、外发策略联动等企业安全需求。
- Acceptance Criteria:
  - [ ] 提供 API 或 webhook 供 DLP 系统查询/扫描内容
  - [ ] 支持上传前敏感信息扫描
  - [ ] 支持按 DLP 策略阻止下载或分享
  - [ ] 记录 DLP 相关审计事件
- Validation:
  - [ ] 上传含敏感信息文档时触发 DLP 策略
- Loop-it notes:
  - Branch hint: feat/issue-41-企业-dlp-集成
  - Risk class: unknown

### Issue 42: 移动端轻量管理后台（Mobile Web Management Lite）
- Source: UI/page-prototypes.md Section 11
- Type: frontend
- Priority: medium
- Dependencies: Issue 15, Issue 17
- Why this slice: 实现发送方在移动设备上的轻量管理界面，包括底部导航、Activity Feed、Hot Signals、Link/Room Summary、Access Requests 和通知设置。复杂的数据室搭建和文档上传仍保留在桌面端。
- Acceptance Criteria:
  - [ ] 底部导航包含 Activity / Hot / Links / Rooms / Me
  - [ ] Activity Feed 展示 first open、repeat open、hot score、forward、access request 等事件
  - [ ] Hot Signals 卡片展示收件人、评分、解释、建议动作
  - [ ] Link Summary 支持复制链接、发送跟进、撤销、打开桌面分析
  - [ ] Room Summary 支持批准访问、查看活跃收件人、打开桌面房间
  - [ ] Access Requests 支持一键批准/拒绝/批准域名
  - [ ] 在 iOS Safari 和 Chrome Android 上验证可用
- Validation:
  - [ ] 在移动端浏览器打开管理后台，Hot Signals 列表正常显示
  - [ ] 点击 Approve 后 access_grant 状态更新为 approved
- Loop-it notes:
  - Branch hint: feat/issue-42-移动端轻量管理后台mobile-web-management-lite
  - Risk class: test_failure

### Issue 43: 联系人管理（Contacts + Contact Detail）
- Source: UI/page-prototypes.md Section 15
- Type: frontend
- Priority: medium
- Dependencies: Issue 2, Issue 13
- Why this slice: 实现 Contacts 列表与 Contact Detail 页面，展示投资人/LP/客户/合伙人的互动历史、数据室访问记录、总体热度评分和推荐下一步动作。支持公司与账户级视图。
- Acceptance Criteria:
  - [ ] Contacts 列表展示姓名、邮箱、组织、细分标签、总体评分
  - [ ] 支持按 segment、组织、评分筛选
  - [ ] Contact Detail 展示个人资料、看过的文档、访问过的数据室、时间线
  - [ ] 展示 Overall engagement score 和 Recommended next action
  - [ ] Company/Account Detail 展示关联联系人、账户级评分、相关链接和房间
  - [ ] 支持与 CRM 映射状态联动（P1）
- Validation:
  - [ ] 在浏览器中打开 Contacts 页面可见联系人列表
  - [ ] 点击联系人进入 Detail 后时间线与评分加载正常
- Loop-it notes:
  - Branch hint: feat/issue-43-联系人管理contacts-+-contact-detail
  - Risk class: test_failure

### Issue 44: 洞察分析中心（Insights）
- Source: UI/page-prototypes.md Section 16
- Type: frontend
- Priority: medium
- Dependencies: Issue 13, Issue 14
- Why this slice: 实现 Insights 页面，包含 Intent Analytics、Content Performance、Page Performance、Team Performance 和 Risk & Audit 视图，帮助用户优化内容并识别机会与风险。
- Acceptance Criteria:
  - [ ] Intent Analytics 展示 Hot / Warm / Cold 收件人、停滞收件人、活跃度上升的账户
  - [ ] Content Performance 展示 Top converting documents、drop-off pages、最高/最低互动页面
  - [ ] Page Performance 展示每页平均停留时间、重读率、跳出率
  - [ ] Team Performance 展示成员活跃度、发送链接数、产生的高意图信号
  - [ ] Risk and Audit 展示被阻止访问、异常地区、下载事件、撤销/过期链接
  - [ ] 支持按时间范围和 segment 筛选
- Validation:
  - [ ] 打开 Insights 页面可见 Intent Analytics 卡片
  - [ ] 筛选时间范围后图表与表格数据更新
- Loop-it notes:
  - Branch hint: feat/issue-44-洞察分析中心insights
  - Risk class: test_failure

### Issue 45: 设置中心（Settings）
- Source: UI/page-prototypes.md Section 18
- Type: frontend
- Priority: medium
- Dependencies: Issue 2
- Why this slice: 实现 Settings 页面，支持工作区配置、成员管理、角色权限、品牌设置、安全默认值、集成连接、账单和数据隐私设置。
- Acceptance Criteria:
  - [ ] Workspace 设置：名称、slug、模式（founder/investment_firm/sales/mixed）
  - [ ] Members 设置：邀请成员、分配角色 owner/admin/member/viewer、移除成员
  - [ ] Branding 设置：上传 logo、设置主色、预览品牌化阅读器
  - [ ] Security defaults：默认访问模式、下载策略、水印策略
  - [ ] Integrations：连接/断开 Slack、HubSpot、Salesforce
  - [ ] Billing：展示当前计划与使用配额（可占位）
  - [ ] Data and privacy：数据保留、删除请求入口
- Validation:
  - [ ] 在浏览器中打开 Settings 可切换各子页面
  - [ ] 修改品牌设置后 viewer 顶部栏显示自定义 logo
- Loop-it notes:
  - Branch hint: feat/issue-45-设置中心settings
  - Risk class: test_failure

### Issue 46: 品牌化 LP 门户 UI（LP Portal）
- Source: P2: LP Portal, UI/page-prototypes.md Section 10.3 Mobile Room Viewer
- Type: frontend
- Priority: low
- Dependencies: Issue 18, Issue 29
- Why this slice: 为投资机构实现品牌化 LP 门户界面，LP 登录后可见 fund deck、季度报告、税务文件等聚合材料，支持按 LP 权限展示不同内容。
- Acceptance Criteria:
  - [ ] 门户首页展示工作区品牌、最新报告、未读内容
  - [ ] 按 LP 账户/联系人权限过滤可见房间和文件
  - [ ] 支持文件夹导航和文件搜索
  - [ ] 展示通知和新内容上线提醒
  - [ ] 响应式布局支持桌面和移动端
- Validation:
  - [ ] LP 登录门户后可见被授权的报告列表
  - [ ] 不同 LP 账户看到的内容按权限区分
- Loop-it notes:
  - Branch hint: feat/issue-46-品牌化-lp-门户-uilp-portal
  - Risk class: unknown

## 11. Downstream Handoff

### 11.1 For /prd-to-spec
- Run /prd-to-spec first if backend architecture, auth, multi-tenancy, object storage, or scoring worker design is unclear.
- Architecture decisions to preserve: PostgreSQL 15+; object storage for blobs; append-only events; workspace_id filtering everywhere.
- Technical questions that need resolution: PDF rendering strategy; scoring job queue; email provider; object storage provider.

### 11.2 For /to-issues
- Use Section 10 as primary source.
- Preserve Source, Dependencies, Acceptance Criteria, Validation, and Loop-it notes.
- Do not create issues from Creative Space unless user confirms.
- Local mode default path: `.autoresearch/issues`.

### 11.3 For /loop-it or /goal
- Build order: Issue 1 → 2 → 3 → 4 → 6 → 9 → 10 → 11 → 14 → 15, with UI issues interleaved once APIs exist.
- Do not reinterpret these hard constraints: workspace isolation, access-before-content, append-only events, no forced accounts, instant revocation.
- Safe implementation freedoms: UI copy, animation, exact colors, empty-state illustrations.
- Stop and ask if: a requirement changes the P0 loop; a security control would block legitimate recipient access; a new dependency is needed.

### 11.4 For /review-it
- Review must verify: all hard constraints have tests or manual evidence; P0 acceptance criteria pass; viewer does not leak content for revoked/expired links; analytics events are append-only.
- Findings that should be rejected as scope creep: adding AI drafts, SSO, custom domains, or BI reporting to P0; changing default access mode to require registration; removing watermark or privacy disclosure.

### 11.5 For /note-it and /ship-it
- Notes should capture: deviations from PRD and why; scoring algorithm weights chosen; performance benchmarks; security decisions.
- PR body must include: closes which issues; summary of user-visible changes; verification evidence (screenshots, test output); any new environment variables or migrations.

### 11.6 Acceptance Scripts

Acceptance Script 1: In a local dev environment, register a workspace, upload a 10-page PDF, create a Smart Link with email verification + watermark, open the link in an incognito browser, verify the email, navigate to page 5 for 30 seconds, and confirm that the Dashboard shows a page-view event within 60 seconds.

Acceptance Script 2: From the Link Detail page, click Revoke, then open the same Smart Link in another browser and verify that a block page is shown and no document bytes are loaded (check Network tab).

Acceptance Script 3: Create a Deal Room from the Seed Fundraising template, upload a pitch deck to the Pitch folder, invite an external email, open the room from that email, and verify the folder and file are visible and an activity event is recorded.

Acceptance Script 4: Simulate repeated opens and page-5 re-reads from the same email over 2 minutes, then verify the intent score updates from Cold to Warm/Hot and a hot-score email alert is queued or sent.

Acceptance Script 5: On a mobile viewport, open a valid Smart Link, verify the viewer loads, swipe through 3 pages, and confirm the bottom navigation and page indicator are usable without horizontal scroll.

## 12. Overdelivery Opportunities

| Opportunity | Effort | Why it matters | Guard |
|---|---|---|---|
| Keyboard shortcuts in viewer (arrow keys, Esc for outline) | Low | Power users navigate faster | Must not conflict with screen readers or OS shortcuts |
| Hot-signal count badge on Dashboard nav item | Low | Draws attention without interrupting | Badge must clear when viewed |
| One-click "Send follow-up" from Dashboard that copies suggested text | Low | Reduces friction between signal and action | Must not auto-send email; only copy draft |
| Empty-state copy that explains why no signals appear yet | Low | Reduces confusion for new users | Must link to upload/create-link CTA |
| Skeleton loading state for Link Detail analytics | Low | Perceived performance boost | Must not block actual content |
| Preferred segment preselection during onboarding | Medium | Makes dashboard language relevant immediately | Must be changeable in settings |

