#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Enrich existing local issue files with SPEC references from tasks/spec-dealsignal-v1.md."""

from pathlib import Path

ISSUES_DIR = Path(__file__).parent / "issues"
SPEC_PATH = Path(__file__).parent.parent / "tasks" / "spec-dealsignal-v1.md"

SPEC_REFS = {
    1: {
        "sections": "SPEC Section 2.4 File Structure, 3.1 Schema Changes, 3.4 Migration Plan",
        "apis": "N/A — infrastructure",
        "data_model": "All P0 tables from sql/schema.sql; add document_page_tiles table",
        "tests": "Migration applies cleanly; lint/build passes"
    },
    2: {
        "sections": "SPEC Section 7.1 Authentication & Authorization, 9.1 Unit Tests",
        "apis": "POST /auth/register, POST /auth/login, GET /auth/me, POST /workspaces",
        "data_model": "users, workspaces, workspace_memberships",
        "tests": "Unit tests for workspace isolation middleware; cross-workspace returns 403"
    },
    3: {
        "sections": "SPEC Section 4.1 Documents, 5.1 Tile Pipeline (step 1), 7.3 Data Protection",
        "apis": "POST /documents, GET /documents, GET /documents/:id, POST /documents/:id/versions",
        "data_model": "documents, document_versions, R2 source bucket",
        "tests": "Integration: upload PDF, verify DB + R2 records"
    },
    4: {
        "sections": "SPEC Section 3.1 document_page_tiles, 5.1 Tile Pipeline (steps 2-5)",
        "apis": "Internal pg-boss processing job; no public API",
        "data_model": "document_pages, document_page_tiles",
        "tests": "10-page PDF produces 10 document_page_tiles rows"
    },
    5: {
        "sections": "SPEC Section 4.1 Documents, 9.4 Acceptance Mapping (US-001)",
        "apis": "GET /documents, GET /documents/:id",
        "data_model": "documents, document_versions, document_page_tiles (counts)",
        "tests": "Browser test shows uploaded documents and detail tabs"
    },
    6: {
        "sections": "SPEC Section 4.1 Smart Links, 5.2 Validation Rules, 5.3 State Machine",
        "apis": "POST /smart-links, GET /smart-links, GET /smart-links/:id, POST /smart-links/:id/revoke",
        "data_model": "smart_links, smart_link_recipients",
        "tests": "Create link with all access modes; verify slug + settings"
    },
    7: {
        "sections": "SPEC Section 4.2 POST /smart-links, 9.4 Acceptance Mapping (US-002)",
        "apis": "POST /smart-links",
        "data_model": "smart_links, documents",
        "tests": "Browser test creates link and copies URL; friction indicator accurate"
    },
    8: {
        "sections": "SPEC Section 4.1 Smart Links, 9.4 Acceptance Mapping (US-002)",
        "apis": "GET /smart-links/:id, POST /smart-links/:id/revoke",
        "data_model": "smart_links, activity_events",
        "tests": "Revoke action blocks viewer access"
    },
    9: {
        "sections": "SPEC Section 4.1 Viewer, 5.1 Access Resolution, 7.3 Data Protection, 9.4 (FR-29)",
        "apis": "GET /v/:slug, POST /v/:slug/verify, POST /v/:slug/password, POST /v/:slug/request-access",
        "data_model": "smart_links, smart_link_recipients, access_grants, view_sessions",
        "tests": "Revoked/expired links show block page without content; public link opens without account"
    },
    10: {
        "sections": "SPEC Section 2.1 System Context, 4.2 GET /v/:slug/manifest, 5.1 Viewer Rendering (Canvas 2D + OffscreenCanvas + Web Worker)",
        "apis": "GET /v/:slug/manifest, GET /v/:slug/tiles/:token",
        "data_model": "document_page_tiles, view_sessions",
        "tests": "Browser and mobile viewport test; tiles decrypt and assemble correctly"
    },
    11: {
        "sections": "SPEC Section 4.1 events, 5.1 Analytics flow, 9.4 Acceptance Mapping (US-004)",
        "apis": "POST /v/:slug/events (beacon)",
        "data_model": "view_sessions, page_view_events, activity_events",
        "tests": "Events recorded after browsing; latency < 60s"
    },
    12: {
        "sections": "SPEC Section 4.1 events, 5.4 Edge Cases",
        "apis": "POST /v/:slug/events",
        "data_model": "download_events, activity_events",
        "tests": "Blocked download creates record; access_denied events logged"
    },
    13: {
        "sections": "SPEC Section 4.1 Analytics, 9.4 Acceptance Mapping (US-004)",
        "apis": "GET /analytics/links/:id, GET /analytics/documents/:id",
        "data_model": "page_view_events, activity_events, view_sessions",
        "tests": "Browser test shows timeline after events"
    },
    14: {
        "sections": "SPEC Section 5.1 Intent Scoring, 8.2 Optimization Strategy, 9.4 Acceptance Mapping (US-005)",
        "apis": "Internal pg-boss scoring job; GET /analytics/dashboard reads scores",
        "data_model": "intent_scores, activity_events, page_view_events",
        "tests": "Simulated activity changes score from cold to warm/hot"
    },
    15: {
        "sections": "SPEC Section 4.1 Dashboard, 9.4 Acceptance Mapping (US-005, US-008)",
        "apis": "GET /analytics/dashboard",
        "data_model": "intent_scores, activity_events, recommendations",
        "tests": "Hot events appear on dashboard"
    },
    16: {
        "sections": "SPEC Section 5.1 Watermark, 7.3 Data Protection, 9.4 Acceptance Mapping (US-007)",
        "apis": "GET /v/:slug/manifest (watermarkText), viewer rendering",
        "data_model": "smart_links (watermark_enabled)",
        "tests": "Screenshot shows both server-baked and client-overlay watermarks"
    },
    17: {
        "sections": "SPEC Section 4.1 Alerts, 6.2 Retry Strategy, 9.4 Acceptance Mapping (US-008)",
        "apis": "Internal alert job triggered by analytics events",
        "data_model": "notifications, notification_preferences",
        "tests": "Email received after first-open and hot-score events"
    },
    18: {
        "sections": "SPEC Section 3.1 Rooms schema, 4.1 Rooms, 5.1 Room logic",
        "apis": "POST /rooms, GET /rooms, GET /rooms/:id, POST /rooms/:id/folders, POST /rooms/:id/files, POST /rooms/:id/members",
        "data_model": "deal_rooms, deal_room_folders, deal_room_files, deal_room_members, deal_room_access_rules",
        "tests": "DB records created; access rules enforced"
    },
    19: {
        "sections": "SPEC Section 4.1 Rooms, 9.4 Acceptance Mapping (US-006)",
        "apis": "All /rooms endpoints",
        "data_model": "deal_rooms, deal_room_folders, deal_room_files, deal_room_members",
        "tests": "Browser test creates room from template and invites member"
    },
    20: {
        "sections": "SPEC Section 4.1 Exports, 8.3 Database Considerations",
        "apis": "GET /exports/links/:id.csv, GET /exports/documents/:id.csv, GET /exports/rooms/:id.csv",
        "data_model": "activity_events, page_view_events, view_sessions",
        "tests": "Downloaded CSV has expected columns"
    }
}


def enrich_file(issue_id: int):
    files = list(ISSUES_DIR.glob(f"issue-{issue_id:03d}-*.md"))
    if not files:
        print(f"Issue {issue_id}: file not found")
        return
    path = files[0]
    content = path.read_text(encoding="utf-8")

    # Avoid double-appending SPEC reference
    if "## SPEC Reference" in content:
        print(f"Issue {issue_id}: already enriched, skipping")
        return

    ref = SPEC_REFS.get(issue_id)
    if not ref:
        print(f"Issue {issue_id}: no SPEC reference mapping")
        return

    section = f"""
## SPEC Reference

- SPEC file: `{SPEC_PATH.relative_to(Path(__file__).parent.parent)}`
- Relevant sections: {ref['sections']}
- API endpoints: {ref['apis']}
- Data model: {ref['data_model']}
- Testing guidance: {ref['tests']}
"""
    new_content = content.rstrip() + "\n" + section
    path.write_text(new_content, encoding="utf-8")
    print(f"Issue {issue_id}: enriched {path.name}")


def main():
    for i in range(1, 21):
        enrich_file(i)


if __name__ == "__main__":
    main()
