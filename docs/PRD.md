# PRD: DealSignal

## 1. Overview

DealSignal is a secure document sharing, deal room, and intent analytics platform for three high-value user groups:

- Fundraising founders
- Investment firms, including VC, PE, fund IR, and M&A teams
- B2B sales and business development teams

The product turns sensitive business documents into controlled links and converts recipient behavior into actionable deal signals.

DealSignal is not positioned as a generic file sharing product. It is a transaction intelligence system for documents that influence fundraising, investment, and revenue outcomes.

## 2. Product Positioning

One-line positioning:

> Turn every critical document into a secure, trackable, deal-moving signal.

Chinese positioning:

> 把每一份关键文档变成可控、可追踪、可推进成交的交易信号系统。

Core product promise:

- Send sensitive materials with control.
- Know who is truly interested.
- Act on the right next step before the opportunity cools down.

## 3. First Principles

All three target groups share the same underlying problem:

> After sending important materials, users lose visibility, control, and timing.

The product must solve three fundamental jobs:

1. Reduce uncertainty: Show who is interested, what they read, and how engagement changes.
2. Reduce risk: Control access, expiration, downloads, watermarks, and revocation.
3. Increase transaction velocity: Recommend the next best action from recipient behavior.

## 4. Target Users

### 4.1 Fundraising Founders

Typical users:

- Seed and Series A founders
- CEOs
- CFOs
- Startup operators handling fundraising

Primary documents:

- Pitch decks
- Financial models
- Cap tables
- Customer lists
- Product demos
- Legal and diligence materials

Top three needs:

1. Identify real investor interest.
2. Control the fundraising narrative and reduce leakage.
3. Reduce diligence friction and appear professionally prepared.

Key product value:

> Know which investors are truly interested before sending the next follow-up.

### 4.2 Investment Firms

Typical users:

- VC partners and associates
- PE and growth equity teams
- Fund IR teams
- Corporate development teams
- Investment bankers and M&A advisors

Primary documents:

- Fund decks
- LP updates
- Quarterly reports
- Deal memos
- Diligence files
- M&A data room materials

Top three needs:

1. Maintain permission control and auditability for sensitive capital materials.
2. Identify LP, buyer, or co-investor engagement.
3. Present a professional, compliant, trustworthy information experience.

Key product value:

> Make sensitive capital information controlled, trackable, and accountable.

### 4.3 B2B Sales and BD Teams

Typical users:

- Account executives
- Sales managers
- Solution consultants
- BD and partnership teams
- RevOps teams

Primary documents:

- Proposals
- Pricing documents
- Business cases
- ROI reports
- Security packets
- Case studies
- Partnership decks

Top three needs:

1. Identify real buying intent.
2. Improve follow-up timing and messaging.
3. Keep customer-facing materials consistent, current, and trackable.

Key product value:

> Turn every proposal into a live buying-intent signal.

## 5. Goals

- Allow users to share sensitive documents through controlled links.
- Provide page-level and recipient-level engagement analytics.
- Generate intent scores tailored to founders, investors, and sales teams.
- Provide lightweight deal rooms for fundraising, LP communication, M&A, and enterprise sales.
- Recommend next actions based on recipient behavior.
- Provide a recipient experience that is lower-friction than legacy secure document tools.

## 6. Non-Goals

- DealSignal will not be a general-purpose cloud drive.
- DealSignal will not provide full legal-grade DRM guarantees against screenshots.
- DealSignal will not replace full enterprise document management suites in the MVP.
- DealSignal will not provide native email campaign automation in the MVP.
- DealSignal will not provide full e-signature workflows in the MVP.
- DealSignal will not automatically rewrite pitch decks or proposals in the MVP.

## 7. Core Product Modules

### 7.1 Smart Links

Smart Links are secure, trackable links generated for individual documents or collections.

Capabilities:

- Upload PDF, PPT, DOC, XLS, image, and video files.
- Generate one or more links per document.
- Name links by recipient, account, fund, or campaign.
- Update a document version while keeping the same link.
- Set link expiration.
- Revoke links instantly.
- Enable password access.
- Require verified email.
- Restrict access to email allowlists.
- Enable or disable downloads.
- Add dynamic watermarking.
- Require NDA before access.

Design principle:

Security settings must show their recipient-friction impact before the user sends the link.

Example:

- Low friction: anyone with link, downloads enabled.
- Balanced: email verification, downloads enabled, watermark enabled.
- High security: allowlist, NDA, no download, watermark, expiry.

### 7.2 Recipient Viewer

The recipient viewer is a core growth surface. Every recipient may become a future sender.

Capabilities:

- Open documents without creating an account when policy allows it.
- Render documents quickly on desktop and mobile.
- Show table of contents where available.
- Support page navigation.
- Support full-text search where possible.
- Support download when enabled.
- Show clear explanations when access is blocked, expired, or requires verification.
- Provide a "contact sender" action.
- Show transparent privacy notice when tracking is enabled.

Design principle:

The recipient should feel trusted, not trapped.

### 7.3 Intent Analytics

DealSignal captures behavior events and translates them into account, recipient, and document-level signals.

Tracked events:

- First open
- Repeat open
- Last open
- Total reading time
- Per-page time
- Page skips
- Page re-reads
- Downloads
- Email verification
- NDA completion
- Forward or new-recipient detection
- Data room entry
- Q&A activity
- Geographic and device metadata

Core analytics views:

- Document analytics
- Recipient analytics
- Account analytics
- Room analytics
- Page performance
- Activity timeline

### 7.4 Intent Scores

Each target segment gets a score model with segment-specific language.

Founder score:

- Investor Intent Score
- Inputs: deck opens, repeat reads, key-page reads, data room entry, partner forwarding, financial page engagement.

Investment firm score:

- LP / Buyer Engagement Score
- Inputs: fund deck opens, report engagement, repeated access, document depth, data room activity, Q&A.

Sales score:

- Deal Intent Score
- Inputs: proposal opens, pricing page views, security page views, multi-stakeholder activity, repeat reads, CRM deal stage.

Score output:

- 0-39: Cold
- 40-69: Warm
- 70-100: Hot

Each score must include explanation text.

Example:

> Hot because this recipient viewed the pricing page 3 times, forwarded the proposal to 4 people, and returned within 24 hours.

### 7.5 Deal Rooms

Deal Rooms allow users to share grouped sensitive materials with structured permissions.

Templates:

- Seed Fundraising Room
- Series A Fundraising Room
- LP Update Room
- M&A Diligence Room
- Enterprise Sales Room
- Partner Enablement Room

Capabilities:

- Create room from template.
- Upload folders and files.
- Set folder-level permissions.
- Require NDA.
- Enable dynamic watermark.
- Approve access requests.
- Track room activity.
- Add Q&A.
- Add requested-documents checklist.
- Export room activity.

### 7.6 Action Assistant

The Action Assistant turns analytics into suggested next steps.

Capabilities:

- Detect high-intent recipients.
- Detect stalled recipients.
- Detect suspicious or unusual access.
- Recommend follow-up timing.
- Generate follow-up talking points.
- Draft email copy.
- Recommend materials to send next.
- Suggest when to open a data room.

Example recommendations:

- Founder: "This investor viewed the financial model twice but did not enter the data room. Send a short note offering a finance walkthrough."
- Fund: "This LP returned to the track record section three times this week. IR should prioritize follow-up."
- Sales: "Three new stakeholders viewed security pages. Suggest scheduling a security review."

### 7.7 Content Library

The Content Library helps teams standardize and measure shared materials.

Capabilities:

- Store approved documents.
- Mark document status: Draft, Approved, Archived.
- Maintain version history.
- Lock approved templates.
- Track content performance.
- Show which pages convert or stall deals.
- Allow team members to create Smart Links only from approved assets when required.

### 7.8 Integrations

MVP integrations:

- Gmail
- Outlook
- Slack
- HubSpot
- Salesforce
- Google Drive
- Dropbox

Integration behaviors:

- Insert Smart Link from email composer.
- Send first-open and high-intent notifications.
- Sync activity to CRM timeline.
- Create CRM tasks for recommended follow-ups.
- Associate links with CRM contacts, accounts, and deals.

## 8. User Stories

### US-001: Upload a Document

Description: As a user, I want to upload a document so that I can create a secure, trackable link.

Acceptance criteria:

- User can upload supported file types.
- Upload progress is visible.
- Failed uploads show a specific error.
- Uploaded document appears in Documents.
- Typecheck and lint pass.

### US-002: Create a Smart Link

Description: As a user, I want to generate a link with access settings so that I can share a document safely.

Acceptance criteria:

- User can create a named link from a document.
- User can choose access mode.
- User can enable or disable downloads.
- User can set expiration.
- User can copy the created link.
- Verify in browser using dev-browser skill.

### US-003: View a Shared Document

Description: As a recipient, I want to open a shared document with minimal friction so that I can review it quickly.

Acceptance criteria:

- Recipient can open a valid link.
- Recipient sees a readable document viewer.
- Recipient can move between pages.
- Recipient sees a clear message if access is expired or denied.
- Verify desktop and mobile browser views.

### US-004: Track Recipient Activity

Description: As a sender, I want to see recipient activity so that I can understand interest.

Acceptance criteria:

- System records first open.
- System records page-level viewing.
- System records download event when download is enabled.
- Activity appears in link analytics.
- Event delay is under 10 seconds in normal conditions.

### US-005: Generate Intent Score

Description: As a sender, I want DealSignal to score engagement so that I can prioritize follow-up.

Acceptance criteria:

- System generates a 0-100 score per recipient.
- Score includes Cold, Warm, or Hot label.
- Score includes an explanation.
- Score updates when new activity occurs.

### US-006: Create a Deal Room

Description: As a user, I want to create a room from a template so that I can share multiple diligence materials quickly.

Acceptance criteria:

- User can choose a room template.
- Room contains default folders from the template.
- User can upload files into folders.
- User can invite recipients.
- User can view room activity.
- Verify in browser using dev-browser skill.

### US-007: Apply Dynamic Watermark

Description: As a user, I want to watermark documents with recipient information so that leaks are discouraged and traceable.

Acceptance criteria:

- User can enable watermark for a link or room.
- Viewer displays watermark with recipient email and timestamp.
- Downloaded files include watermark when download is enabled.
- Watermark setting is visible in link settings.

### US-008: Receive High-Intent Alerts

Description: As a user, I want to be alerted when a recipient shows strong interest so that I can follow up at the right time.

Acceptance criteria:

- User can configure email alerts.
- User can configure Slack alerts when Slack is connected.
- First open alert is sent.
- Hot score alert is sent.
- Alert links to the relevant analytics page.

### US-009: Sync Activity to CRM

Description: As a sales user, I want document activity synced to CRM so that the deal record stays current.

Acceptance criteria:

- User can connect HubSpot or Salesforce.
- User can associate a Smart Link with a CRM deal.
- System writes open and high-intent events to CRM timeline.
- System creates follow-up task for Hot score events when enabled.

### US-010: Manage Approved Sales Content

Description: As a sales manager, I want approved content in a shared library so that reps send the correct materials.

Acceptance criteria:

- Admin can mark a document as Approved.
- Admin can archive old content.
- Team members can filter by status.
- Admin can restrict Smart Link creation to approved content.

## 9. Functional Requirements

- FR-1: The system must allow users to upload supported document files.
- FR-2: The system must generate unique Smart Links for documents.
- FR-3: The system must allow multiple Smart Links per document.
- FR-4: The system must allow users to set link expiration.
- FR-5: The system must allow users to revoke a link.
- FR-6: The system must allow users to require recipient email verification.
- FR-7: The system must allow users to restrict access by email allowlist.
- FR-8: The system must allow users to enable password protection.
- FR-9: The system must allow users to enable or disable downloads.
- FR-10: The system must allow users to enable dynamic watermarking.
- FR-11: The system must record document open events.
- FR-12: The system must record page-level viewing events.
- FR-13: The system must record download events.
- FR-14: The system must display recipient-level analytics.
- FR-15: The system must display document-level analytics.
- FR-16: The system must generate a segment-specific intent score.
- FR-17: The system must explain why an intent score changed.
- FR-18: The system must allow users to create Deal Rooms.
- FR-19: The system must allow users to apply folder-level room permissions.
- FR-20: The system must allow users to invite recipients to a room.
- FR-21: The system must provide room activity logs.
- FR-22: The system must provide high-intent notifications.
- FR-23: The system must allow users to connect Slack.
- FR-24: The system must allow users to connect HubSpot or Salesforce.
- FR-25: The system must sync selected activity events to CRM.
- FR-26: The system must provide a content library.
- FR-27: The system must support document version history.
- FR-28: The system must allow admins to archive documents.
- FR-29: The system must show blocked, expired, and denied access pages.
- FR-30: The system must provide CSV export for analytics.

## 10. Non-Functional Requirements

Security:

- Documents must be encrypted in transit and at rest.
- Access logs must be retained according to workspace policy.
- Users must be able to delete documents and recipient data.
- Admins must be able to configure default security policies.

Performance:

- Viewer first meaningful render should be under 2 seconds for typical PDFs.
- Page navigation should be under 500ms after document load.
- Analytics events should appear within 10 seconds in normal conditions.

Reliability:

- Link revocation must take effect immediately.
- Access rules must be enforced before document content is loaded.
- Failed analytics writes must be retried.

Privacy:

- Recipient tracking must be disclosed in viewer privacy text.
- DealSignal must not sell recipient data.
- Workspace owners must own their first-party engagement data.

Usability:

- Recipients should not need an account unless the sender explicitly requires it.
- Blocked access states must explain what happened and how to request access.
- Security settings must clearly communicate recipient friction.

## 11. MVP Scope

P0:

- Document upload
- Smart Link creation
- Email verification
- Expiration
- Download control
- Revocation
- Basic watermark
- Page-level analytics
- Recipient timeline
- Intent score
- Basic Deal Room
- Email alerts
- CSV export

P1:

- Advanced watermark templates
- Slack alerts
- HubSpot and Salesforce sync
- Deal Room templates
- Content Library
- Action Assistant recommendations
- Branded viewer

P2:

- AI follow-up drafts
- LP Portal
- Custom domain
- Advanced audit export
- SSO
- SCIM
- Data retention policies

P3:

- Advanced workflow automation
- Data residency
- Deep BI reporting
- SOC 2 support workflows
- Enterprise DLP integrations

## 12. Pricing

Founder Plan:

- $19-29 per user per month
- Designed for fundraising founders
- Includes limited active links, basic analytics, watermark, and simple Deal Room

Deal Pro:

- $49-79 per user per month
- Designed for sales and BD
- Includes unlimited links, advanced analytics, CRM integration, Slack alerts, content library, and Deal Intent Score

Secure Room:

- Starts at $199-499 per month
- Designed for funds, M&A, and professional services
- Includes Deal Rooms, NDA, audit logs, SSO-ready controls, branded portals, and LP / Buyer Engagement Score

Enterprise:

- Custom pricing
- Includes SSO, SCIM, data retention, custom security review, legal terms, and dedicated support

## 13. Success Metrics

Activation:

- User uploads first document.
- User creates first Smart Link.
- First recipient opens a shared link.

Founder metrics:

- Deck open rate
- Investor repeat-open rate
- Data room entry rate
- Meeting conversion after Hot score

Investment firm metrics:

- LP report open rate
- LP repeat-engagement rate
- Deal Room active-recipient count
- Audit export usage

Sales metrics:

- Proposal open rate
- Pricing page view rate
- Multi-stakeholder engagement rate
- Follow-up response rate
- Win rate for deals with Hot score

Business metrics:

- Free-to-paid conversion
- 7-day retention
- Monthly active senders
- Average links per workspace
- CRM integration activation
- Net revenue retention

## 14. Product Principles

1. Reduce uncertainty before adding complexity.
2. Security must not kill deal velocity.
3. Analytics must lead to recommended action.
4. Segment-specific language matters.
5. Recipient experience is a growth loop.
6. Data ownership and privacy must be explicit.

## 15. Key Open Questions

- Should the first wedge be fundraising founders or B2B sales teams?
- Should free users get watermarking, or should watermarking be a paid conversion feature?
- Should recipient privacy disclosure be configurable or standardized?
- How much AI assistance is safe before users distrust recommendations?
- Should the product support public no-auth links in regulated workspaces?
