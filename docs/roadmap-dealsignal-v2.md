# DealSignal v2 Commercial Roadmap and Issue Plan

> Canonical roadmap for commercial maturity, version tracking, priority, and GitHub issue creation. Generated from the v2 issue redesign after reviewing DocHub implementation and DealSignal planning assets.

## Strategic Decision


DealSignal is the single commercial product. DocHub remains a reference implementation for secure document sharing, access gates, basic analytics, and AI/RAG patterns, but DealSignal keeps its own Fastify + Drizzle + Vite architecture and richer commercial data model.

The first sellable wedge is not a full data-room platform. It is:

```text
Smart Link + Recipient Activity + Intent Score + Recommended Next Action
```

This lets the product prove that critical documents can become actionable deal signals.

## Implementation Quality Gates

All product implementation issues must preserve these gates unless explicitly marked not applicable:

- **Bilingual UX by default:** every user-facing UI string must use a translation key and ship with English (`en`) and Chinese (`zh-CN`) translations.
- **No hardcoded UI copy:** `pnpm i18n:check` fails when visible JSX strings bypass i18n.
- **Translation parity:** `en.json` and `zh-CN.json` must have identical key structures and non-empty values.
- **API localization contract:** user-facing API errors should expose stable `error.code` values so the web app can localize display messages.
- **CI enforcement:** GitHub Actions runs `pnpm i18n:check`, `pnpm -r typecheck`, and `pnpm -r lint` on pull requests and pushes.

## Version Roadmap

| Version | Name | Commercial Maturity | Business Goal | Sellability |
|---|---|---:|---|---|
| v0.1.0 | Private MVP — Smart Link usable loop | 20% | Prove the core controlled-document-link loop: upload document → create smart link → recipient opens → page events are captured. | Internal demo / design-partner validation; not yet a paid product. |
| v0.2.0 | Commercial MVP — Intent Signal | 45% | Turn raw document analytics into recipient activity, hot/warm/cold scoring, and recommended next actions. | Paid pilots with founders, small sales teams, and BD teams. |
| v0.3.0 | Deal Room v1 | 60% | Expand from single-document links into lightweight multi-document deal rooms with room-level engagement. | Chargeable founder fundraising / investor update / sales room workflows. |
| v0.4.0 | Deal Workflow — Actions and Insights | 72% | Move from passive analytics to workflow guidance: account engagement, AI follow-up drafts, insights, branded viewer, and mobile-lite management. | Differentiated subscription product; supports outbound sales narrative. |
| v0.5.0 | Team GTM Stack | 82% | Support team adoption through Slack/CRM integrations, content library, custom domains, and LP portal v1. | Team plans and expansion revenue. |
| v0.6.0 | Enterprise Trust | 90% | Unlock enterprise purchasing through SSO, SCIM, audit logs, retention policies, security defaults, and SOC 2 support workflows. | Enterprise readiness and higher ACV. |
| v0.7.0+ | AI / Enterprise Intelligence Layer | 95%+ | Create long-term defensibility with advanced workflow automation, AI indexing, deal-room Q&A, BI, data residency, and DLP. | Platform expansion and enterprise differentiation. |

## Issue Plan by Version

### v0.1.0 — Private MVP — Smart Link usable loop

**Business goal:** Prove the core controlled-document-link loop: upload document → create smart link → recipient opens → page events are captured.

| Seq | ID | Title | Type | Priority | Risk | Dependencies |
|---:|---|---|---|---|---|---|
| 1 | DS-001 | Project scaffold and schema baseline | infra | high | build_failure | None |
| 2 | DS-002 | Auth, sessions, and workspace memberships | backend | high | build_failure | DS-001 |
| 3 | DS-003 | Private object storage provider | infra | high | build_failure | DS-001 |
| 4 | DS-004 | Document upload and document_versions | backend | high | build_failure | DS-001, DS-002, DS-003 |
| 5 | DS-005 | Document processing worker | backend | high | build_failure | DS-004 |
| 6 | DS-006 | PDF page extraction and document_pages | backend | high | unknown | DS-005 |
| 7 | DS-007 | Document library and document detail | frontend | high | test_failure | DS-004, DS-006 |
| 8 | DS-008 | Smart link backend | backend | high | build_failure | DS-004 |
| 9 | DS-009 | Smart link creation UI | frontend | high | test_failure | DS-008 |
| 10 | DS-010 | Viewer access gate | fullstack | high | test_failure | DS-008 |
| 11 | DS-011 | Viewer session token security | backend | high | test_failure | DS-010 |
| 12 | DS-012 | PDF viewer v1 | frontend | high | test_failure | DS-006, DS-010, DS-011 |
| 13 | DS-013 | Page view event ingestion | backend | high | test_failure | DS-011, DS-012 |
| 65 | DS-065 | i18n foundation for English and Chinese | fullstack | high | test_failure | DS-001 |

### v0.2.0 — Commercial MVP — Intent Signal

**Business goal:** Turn raw document analytics into recipient activity, hot/warm/cold scoring, and recommended next actions.

| Seq | ID | Title | Type | Priority | Risk | Dependencies |
|---:|---|---|---|---|---|---|
| 14 | DS-014 | Activity event taxonomy | backend | high | test_failure | DS-013 |
| 15 | DS-015 | Link detail and management | frontend | high | test_failure | DS-008, DS-014 |
| 16 | DS-016 | Download and access-denied events | backend | high | test_failure | DS-010, DS-014 |
| 17 | DS-017 | Recipient activity timeline | fullstack | high | test_failure | DS-014, DS-016 |
| 18 | DS-018 | Intent score v1 rules | backend | high | test_failure | DS-013, DS-016, DS-017 |
| 19 | DS-019 | Hot signals dashboard | frontend | high | test_failure | DS-017, DS-018 |
| 20 | DS-020 | Basic dynamic watermark | backend | medium | unknown | DS-012 |
| 21 | DS-021 | Email alert system | backend | medium | test_failure | DS-018 |
| 22 | DS-022 | Action assistant recommendations | backend | high | unknown | DS-018 |
| 23 | DS-023 | Demo workspace and seed data | infra | medium | test_failure | DS-019 |

### v0.3.0 — Deal Room v1

**Business goal:** Expand from single-document links into lightweight multi-document deal rooms with room-level engagement.

| Seq | ID | Title | Type | Priority | Risk | Dependencies |
|---:|---|---|---|---|---|---|
| 24 | DS-024 | Deal room backend | backend | medium | build_failure | DS-004, DS-002 |
| 25 | DS-025 | Deal room management UI | frontend | medium | test_failure | DS-024 |
| 26 | DS-026 | Deal room viewer | fullstack | high | test_failure | DS-024, DS-012 |
| 27 | DS-027 | Deal room permission engine | backend | high | test_failure | DS-024 |
| 28 | DS-028 | Deal room templates | fullstack | medium | test_failure | DS-024 |
| 29 | DS-029 | Room engagement score | backend | medium | test_failure | DS-018, DS-026 |
| 30 | DS-030 | CSV export | backend | medium | test_failure | DS-017 |
| 31 | DS-031 | Contacts and contact detail | frontend | medium | test_failure | DS-002, DS-017 |
| 32 | DS-032 | Settings center | frontend | medium | test_failure | DS-002 |

### v0.4.0 — Deal Workflow — Actions and Insights

**Business goal:** Move from passive analytics to workflow guidance: account engagement, AI follow-up drafts, insights, branded viewer, and mobile-lite management.

| Seq | ID | Title | Type | Priority | Risk | Dependencies |
|---:|---|---|---|---|---|---|
| 33 | DS-033 | Account-level engagement | fullstack | medium | test_failure | DS-031, DS-018 |
| 34 | DS-034 | Advanced watermark templates | backend | medium | unknown | DS-020 |
| 35 | DS-035 | Branded viewer | frontend | low | test_failure | DS-012 |
| 36 | DS-036 | AI follow-up draft | backend | low | unknown | DS-022 |
| 37 | DS-037 | Follow-up draft prompt contract | backend | medium | unknown | DS-036 |
| 38 | DS-038 | Insights center | frontend | medium | test_failure | DS-017, DS-018 |
| 39 | DS-039 | Insight definitions v1 | backend | medium | test_failure | DS-038 |
| 40 | DS-040 | Mobile web management lite | frontend | medium | test_failure | DS-019, DS-021 |

### v0.5.0 — Team GTM Stack

**Business goal:** Support team adoption through Slack/CRM integrations, content library, custom domains, and LP portal v1.

| Seq | ID | Title | Type | Priority | Risk | Dependencies |
|---:|---|---|---|---|---|---|
| 41 | DS-041 | Slack alerts | backend | medium | test_failure | DS-021 |
| 42 | DS-042 | HubSpot / Salesforce connection | backend | medium | test_failure | DS-002 |
| 43 | DS-043 | CRM activity sync | backend | medium | test_failure | DS-042, DS-014 |
| 44 | DS-044 | CRM object mapping rules | backend | medium | test_failure | DS-042, DS-031, DS-033 |
| 45 | DS-045 | Content library backend | backend | medium | build_failure | DS-004 |
| 46 | DS-046 | Content library UI | frontend | medium | test_failure | DS-045 |
| 47 | DS-047 | Content performance v1 | backend | medium | test_failure | DS-045, DS-038 |
| 48 | DS-048 | Custom domain | infra | low | unknown | DS-012 |
| 49 | DS-049 | LP portal v1 | fullstack | low | unknown | DS-024, DS-035 |
| 50 | DS-050 | Notification rules | backend | medium | test_failure | DS-021, DS-041 |

### v0.6.0 — Enterprise Trust

**Business goal:** Unlock enterprise purchasing through SSO, SCIM, audit logs, retention policies, security defaults, and SOC 2 support workflows.

| Seq | ID | Title | Type | Priority | Risk | Dependencies |
|---:|---|---|---|---|---|---|
| 51 | DS-051 | Advanced audit export | backend | low | test_failure | DS-030 |
| 52 | DS-052 | Audit log persistence | backend | high | build_failure | DS-014 |
| 53 | DS-053 | SSO | backend | low | unknown | DS-002 |
| 54 | DS-054 | SCIM | backend | low | unknown | DS-053 |
| 55 | DS-055 | Data retention policies | backend | low | unknown | DS-013, DS-016 |
| 56 | DS-056 | Admin security policies | backend | medium | test_failure | DS-032, DS-052 |
| 57 | DS-057 | SOC 2 support workflow | docs | low | unknown | DS-051, DS-055 |

### v0.7.0+ — AI / Enterprise Intelligence Layer

**Business goal:** Create long-term defensibility with advanced workflow automation, AI indexing, deal-room Q&A, BI, data residency, and DLP.

| Seq | ID | Title | Type | Priority | Risk | Dependencies |
|---:|---|---|---|---|---|---|
| 58 | DS-058 | Advanced workflow automation | backend | low | unknown | DS-022 |
| 59 | DS-059 | Data residency | infra | low | unknown | DS-001 |
| 60 | DS-060 | Deep BI reporting | backend | low | unknown | DS-017, DS-018 |
| 61 | DS-061 | Enterprise DLP integrations | backend | low | unknown | DS-055 |
| 62 | DS-062 | Document AI indexing | backend | medium | unknown | DS-005, DS-006 |
| 63 | DS-063 | Deal room Q&A | fullstack | medium | unknown | DS-024, DS-027, DS-062 |
| 64 | DS-064 | AI risk and opportunity summary | backend | medium | unknown | DS-018, DS-039, DS-062 |

## Commercial Maturity Gates


| Gate | Required Evidence |
|---|---|
| v0.1.0 complete | A user can upload a document, create a smart link, open it as a recipient, and persist page-view events with duration. |
| v0.2.0 complete | Sender can see recipient timeline, hot/warm/cold score, and a recommended next action. |
| v0.3.0 complete | A lightweight deal room can be created, shared, viewed, permissioned, and scored. |
| v0.4.0 complete | Contact/account engagement, AI follow-up drafts, insights, branded viewer, and mobile-lite management are usable. |
| v0.5.0 complete | Slack/CRM/content library/custom domain/LP portal support team expansion. |
| v0.6.0 complete | SSO, SCIM, audit logs, retention, security defaults, and SOC 2 support unlock enterprise procurement. |
| v0.7.0+ complete | AI indexing, deal-room Q&A, advanced BI, DLP, data residency, and workflow automation create platform defensibility. |

## Source Files

- Issue manifest: `docs/tasks/issue-manifest-v2.json`
- Local issue files: `docs/tasks/issues-v2/`
- GitHub creation script: `scripts/create_github_issues_from_v2_manifest.py`
