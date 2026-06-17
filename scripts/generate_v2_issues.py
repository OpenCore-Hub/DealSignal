#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Generate DealSignal v2 roadmap docs and issue source files."""

from __future__ import annotations

import json
import re
from pathlib import Path
from textwrap import dedent

ROOT = Path(__file__).resolve().parents[1]
DOCS = ROOT / "docs"
ISSUES_DIR = DOCS / "tasks" / "issues-v2"
MANIFEST = DOCS / "tasks" / "issue-manifest-v2.json"
ROADMAP = DOCS / "roadmap-dealsignal-v2.md"
GH_SCRIPT = ROOT / "scripts" / "create_github_issues_from_v2_manifest.py"

ISSUES_DIR.mkdir(parents=True, exist_ok=True)

versions = [
    {
        "version": "v0.1.0",
        "name": "Private MVP — Smart Link usable loop",
        "commercial_maturity": "20%",
        "business_goal": "Prove the core controlled-document-link loop: upload document → create smart link → recipient opens → page events are captured.",
        "sellability": "Internal demo / design-partner validation; not yet a paid product.",
    },
    {
        "version": "v0.2.0",
        "name": "Commercial MVP — Intent Signal",
        "commercial_maturity": "45%",
        "business_goal": "Turn raw document analytics into recipient activity, hot/warm/cold scoring, and recommended next actions.",
        "sellability": "Paid pilots with founders, small sales teams, and BD teams.",
    },
    {
        "version": "v0.3.0",
        "name": "Deal Room v1",
        "commercial_maturity": "60%",
        "business_goal": "Expand from single-document links into lightweight multi-document deal rooms with room-level engagement.",
        "sellability": "Chargeable founder fundraising / investor update / sales room workflows.",
    },
    {
        "version": "v0.4.0",
        "name": "Deal Workflow — Actions and Insights",
        "commercial_maturity": "72%",
        "business_goal": "Move from passive analytics to workflow guidance: account engagement, AI follow-up drafts, insights, branded viewer, and mobile-lite management.",
        "sellability": "Differentiated subscription product; supports outbound sales narrative.",
    },
    {
        "version": "v0.5.0",
        "name": "Team GTM Stack",
        "commercial_maturity": "82%",
        "business_goal": "Support team adoption through Slack/CRM integrations, content library, custom domains, and LP portal v1.",
        "sellability": "Team plans and expansion revenue.",
    },
    {
        "version": "v0.6.0",
        "name": "Enterprise Trust",
        "commercial_maturity": "90%",
        "business_goal": "Unlock enterprise purchasing through SSO, SCIM, audit logs, retention policies, security defaults, and SOC 2 support workflows.",
        "sellability": "Enterprise readiness and higher ACV.",
    },
    {
        "version": "v0.7.0+",
        "name": "AI / Enterprise Intelligence Layer",
        "commercial_maturity": "95%+",
        "business_goal": "Create long-term defensibility with advanced workflow automation, AI indexing, deal-room Q&A, BI, data residency, and DLP.",
        "sellability": "Platform expansion and enterprise differentiation.",
    },
]

issues = [
    # v0.1.0
    ("DS-001", "Project scaffold and schema baseline", "v0.1.0", "infra", "high", "build_failure", [], "Original #1", "Establish the monorepo, database schema, migration flow, and baseline checks required for all later work.", ["PostgreSQL schema can be created from migrations", "pnpm build/typecheck/lint baseline passes", "README documents local database and migration commands"], ["Run database migration in a clean environment", "Run pnpm -r typecheck"]),
    ("DS-002", "Auth, sessions, and workspace memberships", "v0.1.0", "backend", "high", "build_failure", ["DS-001"], "Original #2", "Implement users, login/session handling, workspace memberships, and role-based API guards.", ["Users can register and log in", "Users can create or join workspaces", "Workspace members have owner/admin/member/viewer roles", "Every protected API is workspace-scoped"], ["Cross-workspace resource access returns 403", "A workspace owner membership exists after signup"]),
    ("DS-003", "Private object storage provider", "v0.1.0", "infra", "high", "build_failure", ["DS-001"], "New", "Implement a private S3/R2-compatible storage abstraction for sensitive deal materials.", ["Files are stored by bucket/key, not public URL", "Provider supports put/getStream/delete/signed access or proxy access", "Checksum is recorded for uploaded files", "Storage errors surface actionable messages"], ["Upload and retrieve a test object through provider", "Verify no public object URL is stored in app data"]),
    ("DS-004", "Document upload and document_versions", "v0.1.0", "backend", "high", "build_failure", ["DS-001", "DS-002", "DS-003"], "Original #3", "Implement document upload API and version records backed by private object storage.", ["Users can upload supported document files to a workspace", "documents and document_versions rows are created transactionally", "Version numbers increment per document", "Upload failure rolls back metadata and best-effort deletes storage object"], ["Upload PDF creates document + version", "Second version increments version_number"]),
    ("DS-005", "Document processing worker", "v0.1.0", "backend", "high", "build_failure", ["DS-004"], "New", "Create a real background worker for document processing instead of only enqueueing jobs.", ["Upload enqueues a process_document_version job", "Worker consumes jobs with retry/backoff", "processing_status transitions uploaded → processing → ready/failed", "processing_error is stored on failure"], ["Seed a job and verify worker marks version ready", "Force parser failure and verify failed status + retry behavior"]),
    ("DS-006", "PDF page extraction and document_pages", "v0.1.0", "backend", "high", "unknown", ["DS-005"], "Original #4", "Extract page-level metadata, thumbnails or placeholders, and text excerpts from uploaded PDFs.", ["PDF processing writes one document_pages row per page", "page_count is persisted on document_versions", "Each page stores text_excerpt where extractable", "Scanned/empty pages fail gracefully or are marked low-text"], ["10-page PDF produces 10 document_pages rows", "Ready version exposes page_count"]),
    ("DS-007", "Document library and document detail", "v0.1.0", "frontend", "high", "test_failure", ["DS-004", "DS-006"], "Original #5", "Build the sender-side document library and basic document detail surface.", ["Documents list shows name/type/status/link count/open count/update time", "Users can filter by status/type/owner", "Document detail shows overview, pages, links, versions, settings placeholders", "Loading/empty/error states are handled"], ["Open Documents page and see uploaded documents", "Click document and load detail page"]),
    ("DS-008", "Smart link backend", "v0.1.0", "backend", "high", "build_failure", ["DS-004"], "Original #6", "Implement smart link creation, secure slug generation, access modes, expiration, revoke, download policy, and watermark settings.", ["Create one or more unique smart links per document", "Supports public/email_verification/allowlist/password/approval_required/nda_required modes", "Password mode requires password_hash", "Active/expired/revoked states are enforced"], ["Create smart link and verify DB row", "Expired link resolves as expired", "Revoked link blocks viewer access"]),
    ("DS-009", "Smart link creation UI", "v0.1.0", "frontend", "high", "test_failure", ["DS-008"], "Original #7", "Build the sender UI for creating smart links with security presets and recipient-friction messaging.", ["User can select Fast Share / Balanced / High Security presets", "Security controls show recipient friction impact", "User can configure email verification, download, watermark, NDA, expiration", "Created link is copyable"], ["Create link in browser and copy URL", "High Security preset shows high friction"]),
    ("DS-010", "Viewer access gate", "v0.1.0", "fullstack", "high", "test_failure", ["DS-008"], "Original #9", "Implement the public viewer gate that resolves smart links and enforces access before content loads.", ["Valid public link opens viewer", "Expired/revoked link shows clear block reason", "Email verification collects and verifies recipient email", "Password and allowlist modes block unauthorized viewers", "No document bytes are returned before access passes"], ["Revoked link shows blocked page without content", "Public link opens without account"]),
    ("DS-011", "Viewer session token security", "v0.1.0", "backend", "high", "test_failure", ["DS-010"], "New", "Issue server-bound viewer session tokens so event ingestion cannot be forged with only a session id.", ["Session start returns sessionId and opaque sessionToken", "Only hashed sessionToken is stored", "Page/download/heartbeat events require sessionToken", "Events are rejected if token/session/scope mismatch"], ["Forged event with sessionId but wrong token returns 401", "Valid session token accepts page event"]),
    ("DS-012", "PDF viewer v1", "v0.1.0", "frontend", "high", "test_failure", ["DS-006", "DS-010", "DS-011"], "Original #10", "Build a readable desktop/mobile PDF viewer with basic navigation and hooks for event tracking.", ["Viewer supports desktop and mobile layout", "Shows page number and previous/next navigation", "Supports download only when policy allows", "Emits page visibility hooks", "First readable page loads quickly for normal PDFs"], ["Open link on desktop and mobile viewport", "Blocked download is not exposed"]),
    ("DS-013", "Page view event ingestion", "v0.1.0", "backend", "high", "test_failure", ["DS-011", "DS-012"], "Original #11", "Persist page visibility events with reliable duration, idempotency, and workspace scoping.", ["page_view_events stores visibleStartedAt/visibleEndedAt/durationMs", "Events are tied to view_sessions, documents, and versions", "Duration is server-sanity-capped", "Duplicate bursts do not inflate duration excessively"], ["Browse pages and verify durationMs values", "Invalid page number or session token is rejected"]),
    # v0.2.0
    ("DS-014", "Activity event taxonomy", "v0.2.0", "backend", "high", "test_failure", ["DS-013"], "New", "Define and implement the canonical activity event taxonomy that powers timelines, scoring, alerts, and integrations.", ["Event types are documented and typed", "Core events write to activity_events", "Metadata shape is stable per event type", "Events are workspace-scoped and ordered by occurredAt"], ["Create link/open/page/download/access-denied events and verify activity feed rows"]),
    ("DS-015", "Link detail and management", "v0.2.0", "frontend", "high", "test_failure", ["DS-008", "DS-014"], "Original #8", "Build link detail and management UI for status, security, copy, revoke, and activity summaries.", ["Page header shows link name/document/status/security mode", "User can copy and revoke link", "Intent score placeholder or real score is visible", "Recent activity summary is visible", "Revoking immediately blocks viewer access"], ["Click Revoke and verify status revoked", "Link detail loads without browser errors"]),
    ("DS-016", "Download and access-denied events", "v0.2.0", "backend", "high", "test_failure", ["DS-010", "DS-014"], "Original #12", "Record allowed downloads, blocked downloads, and access denied attempts as first-class commercial signals.", ["Allowed downloads create download_events", "Blocked downloads create download_events with blockedReason", "Access denied events create activity_events", "Sender can distinguish security risk from normal engagement"], ["Blocked download creates row", "Denied access appears in activity timeline"]),
    ("DS-017", "Recipient activity timeline", "v0.2.0", "fullstack", "high", "test_failure", ["DS-014", "DS-016"], "Original #13", "Show sender a recipient-level chronological timeline of opens, page reads, downloads, and access issues.", ["Timeline groups events by recipient email/contact", "Events show readable labels and timestamps", "Page-level behavior is summarized", "Timeline can be filtered by link/document"], ["Generate viewer events and see them in timeline", "Timeline distinguishes open/page/download/denied"]),
    ("DS-018", "Intent score v1 rules", "v0.2.0", "backend", "high", "test_failure", ["DS-013", "DS-016", "DS-017"], "Original #14 + New", "Calculate explainable hot/warm/cold scores using deterministic v1 rules before introducing AI scoring.", ["Scores are 0-100", "Labels are cold/warm/hot", "Factors JSON records input signals", "Explanation text is human-readable", "Scores update after relevant activity"], ["Simulated activity changes score cold→warm→hot", "Explanation references actual factors"]),
    ("DS-019", "Hot signals dashboard", "v0.2.0", "frontend", "high", "test_failure", ["DS-017", "DS-018"], "Original #15", "Build the main dashboard around hot signals, recent activity, risks, and recommended follow-ups.", ["Dashboard shows hot/warm/cold recipients", "Shows recommended follow-ups", "Shows recent activity", "Shows risk/security events", "Supports founder/sales/investor-firm copy variants"], ["Open dashboard and see hot signal cards", "New hot score appears after simulated activity"]),
    ("DS-020", "Basic dynamic watermark", "v0.2.0", "backend", "medium", "unknown", ["DS-012"], "Original #16", "Add a basic dynamic watermark overlay for sensitive document viewing.", ["Watermark can include recipient email", "Watermark can include timestamp/link name", "Watermark displays when enabled", "Download policy respects watermark settings"], ["Screenshot viewer and verify watermark", "Disabled watermark does not render"]),
    ("DS-021", "Email alert system", "v0.2.0", "backend", "medium", "test_failure", ["DS-018"], "Original #17", "Send sender-side email alerts for first opens, hot scores, and access requests.", ["First-open alert can be sent", "Hot-score alert can be sent", "Preferences prevent unwanted alerts", "Failures are stored for retry/visibility"], ["Trigger first open and verify email queued/sent", "Disable preference and verify no email"]),
    ("DS-022", "Action assistant recommendations", "v0.2.0", "backend", "high", "unknown", ["DS-018"], "Original #28", "Generate concrete next-best-action recommendations from recipient behavior and score explanations.", ["Recommendations are created for hot or high-change activity", "Each recommendation has title/body/action/status", "Recommendation links to contact/link/room context", "Sender can dismiss or complete recommendations"], ["Simulated hot activity creates recommendation", "Dismissed recommendation no longer appears open"]),
    ("DS-023", "Demo workspace and seed data", "v0.2.0", "infra", "medium", "test_failure", ["DS-019"], "New", "Create deterministic demo workspaces for founder, investment-firm, and sales storylines.", ["Seed includes documents, links, recipients, events, scores, recommendations", "Demo data can be reset", "Demo supports screenshots and sales walkthroughs"], ["Run seed and open dashboard with hot/warm/cold examples"]),
    # v0.3.0
    ("DS-024", "Deal room backend", "v0.3.0", "backend", "medium", "build_failure", ["DS-004", "DS-002"], "Original #18", "Implement lightweight deal rooms for multi-document transaction spaces.", ["Rooms can be created/listed/updated", "Folders can be created", "Documents can be mounted into folders", "Room members can be invited", "Workspace isolation is enforced"], ["Create room/folder/file/member rows", "Access rules are enforced by API"]),
    ("DS-025", "Deal room management UI", "v0.3.0", "frontend", "medium", "test_failure", ["DS-024"], "Original #19", "Build sender-side UI for creating and managing deal rooms, folders, files, and members.", ["User can create room", "User can add folders/files", "User can invite members", "User can preview room structure", "Desktop UX handles realistic room size"], ["Create room from browser", "Invite member and verify room member row"]),
    ("DS-026", "Deal room viewer", "v0.3.0", "fullstack", "high", "test_failure", ["DS-024", "DS-012"], "New", "Build the external recipient viewer for deal rooms and room files.", ["Recipient can open authorized room", "Only authorized folders/files are visible", "Opening files creates view sessions and page events", "Blocked files show clear explanation"], ["Room viewer shows only permitted files", "Opening room file writes events"]),
    ("DS-027", "Deal room permission engine", "v0.3.0", "backend", "high", "test_failure", ["DS-024"], "New", "Resolve effective room permissions from member, contact, account, domain, folder, and document rules.", ["Supports contact/account/domain/role principals", "Supports folder/document scopes", "Resolves canView/canDownload", "Permission checks are shared by API and viewer"], ["Domain rule grants room access", "Document-specific deny/allow behaves correctly"]),
    ("DS-028", "Deal room templates", "v0.3.0", "fullstack", "medium", "test_failure", ["DS-024"], "Original #25", "Provide starter room templates for fundraising, LP updates, M&A diligence, and enterprise sales.", ["User can create room from template", "Template creates folders and checklist placeholders", "Templates are segment-aware", "User can edit generated structure"], ["Create fundraising room from template", "Template folders appear correctly"]),
    ("DS-029", "Room engagement score", "v0.3.0", "backend", "medium", "test_failure", ["DS-018", "DS-026"], "New", "Calculate room-level engagement scores for contacts/accounts based on room visits and file/page activity.", ["Room score uses file opens, depth, repeats, downloads, and questions", "Score has explanation and factors", "Room scores appear in dashboard/room detail"], ["Simulated room activity changes room score", "Explanation references room behavior"]),
    ("DS-030", "CSV export", "v0.3.0", "backend", "medium", "test_failure", ["DS-017"], "Original #20", "Export link/document/room analytics for sender reporting.", ["Exports support smart link analytics", "Exports include page-level activity", "Exports support room analytics where available", "CSV uses safe headers and no-store responses"], ["Download CSV and validate expected columns"]),
    ("DS-031", "Contacts and contact detail", "v0.3.0", "frontend", "medium", "test_failure", ["DS-002", "DS-017"], "Original #43", "Build contacts list and detail pages so DealSignal moves beyond anonymous email analytics.", ["Contacts list shows name/email/account/segment/score", "Contact detail shows timeline and viewed documents/rooms", "Filters by segment/account/score", "CRM mapping placeholder is visible"], ["Open contacts page and see list", "Open contact detail and see timeline"]),
    ("DS-032", "Settings center", "v0.3.0", "frontend", "medium", "test_failure", ["DS-002"], "Original #45", "Create a coherent settings center for workspace, members, branding, security defaults, integrations, billing placeholders, and data/privacy.", ["Settings has workspace/members/branding/security/integrations/billing/data sections", "Members can be invited and roles changed", "Security defaults can be viewed/edited", "Branding changes preview in viewer"], ["Open settings and switch subpages", "Brand setting updates viewer preview"]),
    # v0.4.0
    ("DS-033", "Account-level engagement", "v0.4.0", "fullstack", "medium", "test_failure", ["DS-031", "DS-018"], "New", "Aggregate engagement across contacts into account-level scores and timelines.", ["Account detail shows contacts and account timeline", "Account score uses contact/link/room activity", "Account-level recommendations can be generated", "Domain matching associates contacts to accounts"], ["Multiple contacts from same account roll up into account score"]),
    ("DS-034", "Advanced watermark templates", "v0.4.0", "backend", "medium", "unknown", ["DS-020"], "Original #21", "Support configurable watermark templates beyond the basic overlay.", ["Workspace can define watermark templates", "Templates support recipient/link/time fields", "Templates can be applied per link/room", "Preview shows rendered watermark"], ["Apply template and verify viewer watermark"]),
    ("DS-035", "Branded viewer", "v0.4.0", "frontend", "low", "test_failure", ["DS-012"], "Original #29", "Allow sender-controlled branding in recipient viewer.", ["Viewer can display workspace logo/theme", "Branding can be scoped by workspace or link", "Fallback branding is clean", "Branding does not bypass security messages"], ["Viewer shows custom logo/theme after settings update"]),
    ("DS-036", "AI follow-up draft", "v0.4.0", "backend", "low", "unknown", ["DS-022"], "Original #30", "Generate follow-up email drafts based on activity and recommendations.", ["Draft includes subject/body/CTA", "Draft uses recipient timeline and score factors", "Sender can copy draft", "No email is auto-sent in v1"], ["Generate draft for hot recipient and verify it references actual behavior"]),
    ("DS-037", "Follow-up draft prompt contract", "v0.4.0", "backend", "medium", "unknown", ["DS-036"], "New", "Define prompt inputs, outputs, tone variants, safety constraints, and evaluation fixtures for AI follow-up drafts.", ["Prompt supports founder/sales/investor-firm tone", "Output schema is stable", "Draft avoids fabricating activity", "Fixtures cover hot/warm/cold examples"], ["Run prompt fixtures and validate schema"]),
    ("DS-038", "Insights center", "v0.4.0", "frontend", "medium", "test_failure", ["DS-017", "DS-018"], "Original #44", "Build an insights surface for intent analytics, content performance, page performance, team performance, and risk/audit views.", ["Intent Analytics cards are visible", "Content/page performance views show top and drop-off pages", "Risk and audit views show security events", "Date range and segment filters work"], ["Open Insights and filter by date range"]),
    ("DS-039", "Insight definitions v1", "v0.4.0", "backend", "medium", "test_failure", ["DS-038"], "New", "Implement first-class definitions for recurring commercial insights.", ["Detect stalled recipient", "Detect returning hot contact", "Detect key-page spike", "Detect unexpected geography or blocked access risk", "Each insight has explanation and severity"], ["Seed events and verify expected insights are produced"]),
    ("DS-040", "Mobile web management lite", "v0.4.0", "frontend", "medium", "test_failure", ["DS-019", "DS-021"], "Original #42", "Build lightweight mobile management for activity, hot signals, links, rooms, access requests, and notification settings.", ["Bottom nav includes Activity/Hot/Links/Rooms/Me", "Hot signals are readable on mobile", "Link/room summaries support key actions", "Access requests can be approved/denied"], ["Open mobile viewport and see hot signals", "Approve access request from mobile"]),
    # v0.5.0
    ("DS-041", "Slack alerts", "v0.5.0", "backend", "medium", "test_failure", ["DS-021"], "Original #22", "Send hot signal and activity alerts to Slack.", ["Workspace can connect Slack", "Hot score and first-open alerts can be sent", "Messages link back to DealSignal", "Failures are recorded"], ["Trigger hot score and verify Slack notification in test/mocked adapter"]),
    ("DS-042", "HubSpot / Salesforce connection", "v0.5.0", "backend", "medium", "test_failure", ["DS-002"], "Original #23", "Connect CRM providers for later object mapping and activity sync.", ["User can connect HubSpot/Salesforce", "Credentials are encrypted", "Connection status is visible", "Disconnect revokes or disables sync"], ["Connect mocked CRM provider and verify integration row"]),
    ("DS-043", "CRM activity sync", "v0.5.0", "backend", "medium", "test_failure", ["DS-042", "DS-014"], "Original #24", "Sync selected activity events to CRM objects.", ["Open/download/hot-score events can sync", "Sync respects workspace settings", "Sync errors are logged", "Duplicate activity is not sent repeatedly"], ["Simulated page event creates CRM sync payload"]),
    ("DS-044", "CRM object mapping rules", "v0.5.0", "backend", "medium", "test_failure", ["DS-042", "DS-031", "DS-033"], "New", "Map local contacts/accounts/links/rooms/documents to external CRM objects.", ["Contacts match by email", "Accounts match by domain/name", "Mappings are stored in crm_mappings", "Ambiguous matches require user action"], ["Known contact maps to external CRM contact", "Ambiguous domain match is not auto-linked"]),
    ("DS-045", "Content library backend", "v0.5.0", "backend", "medium", "build_failure", ["DS-004"], "Original #26", "Implement managed sales/fundraising content collections with approval status.", ["Users can create collections", "Documents can be added as library items", "Items have draft/in_review/approved/archived status", "Approved version can be tracked"], ["Create approved content item and query library"]),
    ("DS-046", "Content library UI", "v0.5.0", "frontend", "medium", "test_failure", ["DS-045"], "Original #27", "Build UI for browsing, filtering, approving, and using content library assets.", ["Collections and items are visible", "Filters by status/type work", "Approved assets are distinguishable", "Users can create smart links from library documents"], ["Open content library and create link from approved document"]),
    ("DS-047", "Content performance v1", "v0.5.0", "backend", "medium", "test_failure", ["DS-045", "DS-038"], "New", "Measure which content drives opens, depth, hot scores, and drop-offs.", ["Content performance aggregates views/depth/scores", "Top converting documents are visible", "Drop-off pages are identified", "Results can feed Insights"], ["Seed activities and verify content performance rankings"]),
    ("DS-048", "Custom domain", "v0.5.0", "infra", "low", "unknown", ["DS-012"], "Original #32", "Support custom domains for branded viewer and portal experiences.", ["Workspace can add domain", "DNS verification shows required record", "Verified domain serves viewer/portal routes", "Remove/retry flows are supported"], ["Verify mocked DNS and open viewer URL on custom host"]),
    ("DS-049", "LP portal v1", "v0.5.0", "fullstack", "low", "unknown", ["DS-024", "DS-035"], "Original #31 + #46", "Build a branded LP portal experience for investment-firm use cases.", ["Portal homepage shows brand/latest reports/unread content", "LP permissions filter visible rooms/files", "Folder navigation and search work", "Desktop and mobile layouts are usable"], ["Different LP accounts see different authorized content"]),
    ("DS-050", "Notification rules", "v0.5.0", "backend", "medium", "test_failure", ["DS-021", "DS-041"], "New", "Give users configurable notification rules for email, Slack, CRM, and in-app channels.", ["Rules support first-open/hot-score/access-request/download-blocked", "Rules can be enabled/disabled per user/channel", "Notification preferences are enforced", "Queued notifications include related event context"], ["Disable hot-score Slack rule and verify no Slack notification"]),
    # v0.6.0
    ("DS-051", "Advanced audit export", "v0.6.0", "backend", "low", "test_failure", ["DS-030"], "Original #33", "Export richer audit and activity data for compliance and enterprise review.", ["Exports support date/user/action filters", "Exports include access/download/security events", "Exports can include room-level events", "Export files are access controlled"], ["Download filtered audit export and validate rows"]),
    ("DS-052", "Audit log persistence", "v0.6.0", "backend", "high", "build_failure", ["DS-014"], "New", "Persist immutable audit logs separately from user-facing activity events.", ["audit_logs records actor/action/target/before/after/ip/userAgent", "Critical admin/security actions write audit logs", "Audit logs cannot be edited through app APIs", "Audit reads are admin-only"], ["Change security setting and verify audit log row"]),
    ("DS-053", "SSO", "v0.6.0", "backend", "low", "unknown", ["DS-002"], "Original #34", "Add SAML/OIDC SSO for enterprise workspaces.", ["Workspace can configure SSO provider", "Users can sign in via SSO", "Domain restrictions can route to SSO", "Fallback/admin recovery path exists"], ["Mock SAML/OIDC login creates authenticated session"]),
    ("DS-054", "SCIM", "v0.6.0", "backend", "low", "unknown", ["DS-053"], "Original #35", "Support SCIM user and group provisioning.", ["SCIM can create/update/deactivate users", "Workspace memberships sync from SCIM", "SCIM tokens are scoped and revocable", "Errors are auditable"], ["SCIM deactivate removes workspace access"]),
    ("DS-055", "Data retention policies", "v0.6.0", "backend", "low", "unknown", ["DS-013", "DS-016"], "Original #36", "Allow admins to configure retention for activity, analytics, downloads, and document artifacts.", ["Workspace can define retention windows", "Cleanup job enforces retention", "Archived/deleted records behave predictably", "Policy changes are audited"], ["Run cleanup with test retention and verify old events handled"]),
    ("DS-056", "Admin security policies", "v0.6.0", "backend", "medium", "test_failure", ["DS-032", "DS-052"], "New", "Give admins default controls for secure sharing and viewer behavior.", ["Default access mode can be configured", "Default download policy can be configured", "Default watermark policy can be configured", "Allowed domain/session expiry defaults can be configured"], ["New smart link inherits workspace security defaults"]),
    ("DS-057", "SOC 2 support workflow", "v0.6.0", "docs", "low", "unknown", ["DS-051", "DS-055"], "Original #40", "Document and productize evidence workflows needed for SOC 2 readiness.", ["SOC 2 checklist exists", "Evidence export steps are documented", "Relevant controls map to product features", "Open gaps are visible"], ["Generate SOC 2 evidence checklist from docs"]),
    # v0.7.0+
    ("DS-058", "Advanced workflow automation", "v0.7.0+", "backend", "low", "unknown", ["DS-022"], "Original #37", "Automate multi-step follow-up and routing workflows after core recommendations prove value.", ["Users can define trigger/action workflows", "Workflows use activity and score events", "Executions are logged", "Failures can retry or be inspected"], ["Hot score triggers configured workflow execution"]),
    ("DS-059", "Data residency", "v0.7.0+", "infra", "low", "unknown", ["DS-001"], "Original #38", "Support region-aware data placement for enterprise customers that require residency controls.", ["Region metadata is represented", "Storage and DB strategy is documented", "Workspace can be assigned region", "Cross-region risks are documented"], ["Create workspace with region setting and verify metadata"]),
    ("DS-060", "Deep BI reporting", "v0.7.0+", "backend", "low", "unknown", ["DS-017", "DS-018"], "Original #39", "Add deeper BI reporting after sufficient event volume and customer demand exist.", ["Reports support funnels/cohorts/content performance", "Reports can be filtered by segment/account/user/date", "Exports are supported", "Heavy queries are optimized or pre-aggregated"], ["Generate report over seeded dataset"]),
    ("DS-061", "Enterprise DLP integrations", "v0.7.0+", "backend", "low", "unknown", ["DS-055"], "Original #41", "Integrate with enterprise DLP systems where customer demand justifies the complexity.", ["DLP provider configuration exists", "Document/share events can be evaluated", "Blocked actions record reason", "Failures fail safe where appropriate"], ["Mock DLP block prevents configured action"]),
    ("DS-062", "Document AI indexing", "v0.7.0+", "backend", "medium", "unknown", ["DS-005", "DS-006"], "New", "Add embeddings and document chunks for later AI Q&A and opportunity/risk summaries.", ["document_chunks schema exists", "Worker embeds chunks with controlled concurrency", "Embedding model/dimensions are recorded", "Vector search integration test passes"], ["Process document and retrieve expected chunk by semantic query"]),
    ("DS-063", "Deal room Q&A", "v0.7.0+", "fullstack", "medium", "unknown", ["DS-024", "DS-027", "DS-062"], "New", "Allow authorized recipients and senders to ask questions across deal-room documents with citations.", ["Q&A only searches authorized files", "Answers include document/page citations", "Unready files are reported not blocking whole room", "Conversation history is scoped correctly"], ["Ask room question and verify answer citations from authorized files only"]),
    ("DS-064", "AI risk and opportunity summary", "v0.7.0+", "backend", "medium", "unknown", ["DS-018", "DS-039", "DS-062"], "New", "Summarize account/contact/room activity into AI-generated risks, opportunities, and recommended next moves.", ["Summary uses actual events/scores only", "Risks and opportunities cite evidence", "Summary can be regenerated", "Output can create recommendations"], ["Generate summary for seeded hot account and verify evidence references"]),
]

def slugify(text: str) -> str:
    s = text.lower()
    s = re.sub(r"[^a-z0-9\u4e00-\u9fff]+", "-", s).strip("-")
    return s[:72] or "issue"

manifest = []
for idx, (local_id, title, version, typ, priority, risk, deps, source, description, ac, validation) in enumerate(issues, start=1):
    body = f"""# [{local_id}] {title}

## Description
{description}

## Source
{source}

## Version
{version}

## Hard Constraints
- Preserve workspace isolation for every persisted record and API query.
- Do not expose sensitive document bytes before access control passes.
- Keep activity/audit records append-only unless a later retention policy explicitly removes them.

## Acceptance Criteria
"""
    for item in ac:
        body += f"- [ ] {item}\n"
    body += "\n## Validation\n"
    for item in validation:
        body += f"- [ ] {item}\n"
    body += f"\n## Dependencies\n{', '.join(deps) if deps else 'None'}\n"
    body += f"\n## Type\n{typ}\n\n## Priority\n{priority}\n\n## Risk Class\n{risk}\n\n## PRD Reference\ndocs/PRD.md; docs/tasks/prd-dealsignal-v1.md; docs/tasks/spec-dealsignal-v1.md\n"
    body += f"\n## Loop-it Notes\n- Branch hint: feat/{local_id.lower()}-{slugify(title)[:42]}\n- Version: {version}\n- Priority: {priority}\n"

    filename = f"issue-{idx:03d}-{local_id.lower()}-{slugify(title)}.md"
    path = ISSUES_DIR / filename
    path.write_text(body, encoding="utf-8")
    manifest.append({
        "seq": idx,
        "local_id": local_id,
        "title": title,
        "version": version,
        "type": typ,
        "priority": priority,
        "risk_class": risk,
        "dependencies": deps,
        "source": source,
        "local_path": str(path.relative_to(ROOT)),
        "github_number": None,
        "github_url": None,
        "status": "planned",
    })

MANIFEST.write_text(json.dumps({"source": "DealSignal v2 commercial roadmap", "issue_count": len(manifest), "versions": versions, "issues": manifest}, ensure_ascii=False, indent=2), encoding="utf-8")

roadmap = "# DealSignal v2 Commercial Roadmap and Issue Plan\n\n"
roadmap += "> Canonical roadmap for commercial maturity, version tracking, priority, and GitHub issue creation. Generated from the v2 issue redesign after reviewing DocHub implementation and DealSignal planning assets.\n\n"
roadmap += "## Strategic Decision\n\n"
roadmap += dedent("""
DealSignal is the single commercial product. DocHub remains a reference implementation for secure document sharing, access gates, basic analytics, and AI/RAG patterns, but DealSignal keeps its own Fastify + Drizzle + Vite architecture and richer commercial data model.

The first sellable wedge is not a full data-room platform. It is:

```text
Smart Link + Recipient Activity + Intent Score + Recommended Next Action
```

This lets the product prove that critical documents can become actionable deal signals.
""")
roadmap += "\n## Version Roadmap\n\n"
roadmap += "| Version | Name | Commercial Maturity | Business Goal | Sellability |\n|---|---|---:|---|---|\n"
for v in versions:
    roadmap += f"| {v['version']} | {v['name']} | {v['commercial_maturity']} | {v['business_goal']} | {v['sellability']} |\n"

roadmap += "\n## Issue Plan by Version\n\n"
for v in versions:
    roadmap += f"### {v['version']} — {v['name']}\n\n"
    roadmap += f"**Business goal:** {v['business_goal']}\n\n"
    roadmap += "| Seq | ID | Title | Type | Priority | Risk | Dependencies |\n|---:|---|---|---|---|---|---|\n"
    for item in manifest:
        if item["version"] == v["version"]:
            deps = ", ".join(item["dependencies"]) if item["dependencies"] else "None"
            roadmap += f"| {item['seq']} | {item['local_id']} | {item['title']} | {item['type']} | {item['priority']} | {item['risk_class']} | {deps} |\n"
    roadmap += "\n"

roadmap += "## Commercial Maturity Gates\n\n"
roadmap += dedent("""
| Gate | Required Evidence |
|---|---|
| v0.1.0 complete | A user can upload a document, create a smart link, open it as a recipient, and persist page-view events with duration. |
| v0.2.0 complete | Sender can see recipient timeline, hot/warm/cold score, and a recommended next action. |
| v0.3.0 complete | A lightweight deal room can be created, shared, viewed, permissioned, and scored. |
| v0.4.0 complete | Contact/account engagement, AI follow-up drafts, insights, branded viewer, and mobile-lite management are usable. |
| v0.5.0 complete | Slack/CRM/content library/custom domain/LP portal support team expansion. |
| v0.6.0 complete | SSO, SCIM, audit logs, retention, security defaults, and SOC 2 support unlock enterprise procurement. |
| v0.7.0+ complete | AI indexing, deal-room Q&A, advanced BI, DLP, data residency, and workflow automation create platform defensibility. |
""")

roadmap += "\n## Source Files\n\n"
roadmap += f"- Issue manifest: `docs/tasks/issue-manifest-v2.json`\n"
roadmap += f"- Local issue files: `docs/tasks/issues-v2/`\n"
roadmap += f"- GitHub creation script: `scripts/create_github_issues_from_v2_manifest.py`\n"
ROADMAP.write_text(roadmap, encoding="utf-8")

GH_SCRIPT.write_text(dedent('''
#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Create GitHub issues from docs/tasks/issue-manifest-v2.json.

Idempotency: skips manifest entries that already have github_number.
"""

from __future__ import annotations

import json
import subprocess
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
MANIFEST = ROOT / "docs" / "tasks" / "issue-manifest-v2.json"

LABELS = {
    "type:backend": "5319e7",
    "type:frontend": "0e8a16",
    "type:fullstack": "1d76db",
    "type:infra": "fbca04",
    "type:docs": "0075ca",
    "priority:high": "d93f0b",
    "priority:medium": "fbca04",
    "priority:low": "0e8a16",
    "version:v0.1.0": "b60205",
    "version:v0.2.0": "b60205",
    "version:v0.3.0": "b60205",
    "version:v0.4.0": "b60205",
    "version:v0.5.0": "b60205",
    "version:v0.6.0": "b60205",
    "version:v0.7.0+": "b60205",
    "risk:build_failure": "d93f0b",
    "risk:test_failure": "fbca04",
    "risk:unknown": "cfd3d7",
}


def run(args: list[str], *, input_text: str | None = None) -> subprocess.CompletedProcess[str]:
    return subprocess.run(args, cwd=ROOT, input=input_text, text=True, capture_output=True, check=False)


def ensure_labels() -> None:
    existing_proc = run(["gh", "label", "list", "--limit", "500", "--json", "name"])
    existing = set()
    if existing_proc.returncode == 0:
        existing = {item["name"] for item in json.loads(existing_proc.stdout or "[]")}
    for name, color in LABELS.items():
        if name in existing:
            continue
        desc = name.replace(":", " ")
        proc = run(["gh", "label", "create", name, "--color", color, "--description", desc])
        if proc.returncode != 0 and "already exists" not in proc.stderr.lower():
            print(f"WARN: failed to create label {name}: {proc.stderr.strip()}")


def main() -> None:
    data = json.loads(MANIFEST.read_text(encoding="utf-8"))
    ensure_labels()
    created = []
    for issue in data["issues"]:
        if issue.get("github_number"):
            print(f"skip {issue['local_id']} already created as #{issue['github_number']}")
            continue
        body_path = ROOT / issue["local_path"]
        body = body_path.read_text(encoding="utf-8")
        title = f"[{issue['local_id']}] {issue['title']}"
        labels = [
            f"type:{issue['type']}",
            f"priority:{issue['priority']}",
            f"version:{issue['version']}",
            f"risk:{issue['risk_class']}",
        ]
        args = ["gh", "issue", "create", "--title", title, "--body", body]
        for label in labels:
            args.extend(["--label", label])
        proc = run(args)
        if proc.returncode != 0:
            print(f"ERROR creating {issue['local_id']}: {proc.stderr.strip()}")
            continue
        url = proc.stdout.strip().splitlines()[-1]
        number = url.rstrip("/").split("/")[-1]
        issue["github_number"] = int(number)
        issue["github_url"] = url
        issue["status"] = "open"
        created.append((issue["local_id"], number, url))
        MANIFEST.write_text(json.dumps(data, ensure_ascii=False, indent=2), encoding="utf-8")
        print(f"created {issue['local_id']} -> #{number} {url}")
    print(f"created_count={len(created)}")


if __name__ == "__main__":
    main()
''').lstrip(), encoding="utf-8")

print(f"Generated {len(manifest)} issues")
print(ROADMAP.relative_to(ROOT))
print(MANIFEST.relative_to(ROOT))
print(ISSUES_DIR.relative_to(ROOT))
print(GH_SCRIPT.relative_to(ROOT))
