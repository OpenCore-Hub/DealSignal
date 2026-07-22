# Spec: Visitor Ask, Deal-Room Knowledge Base, and Anti-Bypass Controls

> Source design: `docs/designs/plan/visitor-ask-knowledge-base.md` (v1.3)  
> Status: **V1 shipped** — dual-gen rebuild, embedder wiring, building-period Ask Docs, Redis fail-closed limits landed; residual P1/P2 in `docs/designs/plan/PLAN-visitor-ask-v1-debt.md`  
> Test seams (agreed): (1) Public visitor API (2) Deal-room owner + link save API (3) Owner audit read API

## Problem Statement

Owners configuring share links see two overlapping advanced toggles (“AI assistant” and “Q&A conversations”) without a clear mental model. Visitors who pass link gates can ask questions, but Ask Docs today is not gated with the same access path as viewing pages, so a valid or stale session can call the AI endpoint more loosely than Ask Host. Deal-room RAG readiness is opaque (upload may embed automatically), owners cannot explicitly choose what is indexed, cannot audit Ask Docs with proof of authorized scope, and cannot trust that visitors cannot bypass access control or scrape materials via long evidence quotes and unbounded AI calls.

## Solution

Unify visitor-facing AI and human Q&A into one **Visitor Ask / 沟通** capability with two channels: **Ask Docs** (instant, evidence-grounded answers over `KB ∩ link-authorized documents`) and **Ask Host** (async messages to the owner). Introduce an explicit **deal-room knowledge base** (create/rebuild with folder/document selection; embeddings only on create/rebuild). Make Ask Docs auditable for room members and workspace admins. Enforce anti-bypass: Ask Docs and Ask Host use the same public access resolution as page viewing, re-evaluate allow/block on every call, bind APIs to `publicToken`, hard rate-limit, truncate evidence quotes, and record high-risk security events.

## User Stories

1. As a **link owner**, I want a single **Visitor Ask** control with Ask Docs and Ask Host sub-options, so that I understand and configure visitor communication without two confusing switches.
2. As a **link owner**, I want advanced enabled-count to treat Visitor Ask as one capability, so that the UI does not inflate “features enabled.”
3. As a **link owner**, I want clear copy that Ask Docs is instant grounded Q&A and Ask Host is async human follow-up, so that I pick the right channel.
4. As a **link owner**, I want file requests to remain a separate owner-initiated collection tool, so that missing-materials feedback goes to Ask Host instead of a visitor-driven file-request flow.
5. As a **deal-room admin**, I want to **create a knowledge base** by selecting folders/documents (default none selected), so that only approved materials are embedded.
6. As a **deal-room admin**, I want folder selections to **follow the path** (new files mark KB stale; rebuild embeds them), so that scope stays intentional without re-picking every file.
7. As a **deal-room admin**, I want to **rebuild** the knowledge base, so that I can refresh embeddings after changes or model updates.
8. As a **deal-room admin**, I want Ask Docs to keep working on the **previous index while rebuild runs**, so that diligence is not interrupted.
9. As a **deal-room admin**, I want KB **stale** (soft) after room document changes without hard-stopping Ask Docs, so that availability is preserved while I am nudged to rebuild.
10. As a **deal-room admin**, I want deal-room uploads to produce preview pages/chunks **without auto-embedding**, so that embedding is an explicit trust action.
11. As a **link owner**, I want saving Ask Docs to **fail** if the room KB is not `ready` or `stale`, so that I cannot enable a broken visitor experience.
12. As a **link owner**, I want a **warning** when link authorization is not a subset of the KB selection, so that I know some authorized folders are not Ask Docs–searchable.
13. As a **link owner**, I want migration that **turns off Ask Docs** on links in rooms without a ready/stale KB at launch, so that we do not leave unsafe half-enabled links.
14. As a **visitor**, I want one **Ask / 沟通** sidebar entry, so that I do not hunt for separate AI and Q&A tabs.
15. As a **visitor**, I want Ask Docs / Ask Host mode toggle when both are enabled, defaulting to Ask Docs, so that I can choose instant search vs human follow-up.
16. As a **visitor**, I want Ask Docs answers grounded in **link-authorized ∩ KB** materials only, so that I never see out-of-scope content.
17. As a **visitor**, I want Ask Docs to search the **full link-authorized ∩ KB set by default**, with copy that answers are based on materials authorized for this link.
18. As a **visitor**, I want a clear **refusal** when no evidence is found, so that I am not misled by ungrounded answers.
19. As a **visitor**, I want a prompt to switch to Ask Host after refusal (if enabled), so that I can escalate missing or judgment questions.
20. As a **visitor**, I want evidence cards with **short quotes** and jump-to-page, so that I can verify answers without receiving downloadable dumps.
21. As a **visitor**, I want Ask Host for missing materials and human confirmation, so that the owner can reply asynchronously.
22. As a **visitor**, I want my own Ask Host thread status (e.g. awaiting reply), so that I know the owner has not answered yet.
23. As a **visitor who failed gates**, I want Ask Docs and Ask Host APIs to reject me the same way page access does, so that I cannot bypass NDA/email/password/session rules via chat.
24. As a **visitor who was removed from allowlist or blocked**, I want subsequent Ask calls to fail and my session invalidated, so that revocation is immediate.
25. As a **visitor**, I want rate limits on Ask Docs/Host, so that abuse is constrained (even if that limits me when I spam).
26. As a **room member**, I want to read Ask Docs **audit** of questions, answers, and evidence scope, so that I trust AI did not exceed authorization.
27. As a **workspace admin**, I want the same audit visibility, so that I can review compliance across rooms.
28. As a **room member**, I want default audit lists to show the last **90 days**, with older items **archived but searchable**, so that the UI stays clean without losing history.
29. As a **link owner**, I want Ask Docs audit next to Ask Host management on the link, so that I operate both channels in one place.
30. As a **room owner**, I want a room-level Ask Docs audit timeline (filterable by link), so that I can monitor the whole deal.
31. As a **link owner**, I want Ask Docs questions to still create **Signals** for intent, separate from the audit ledger, so that the radar remains useful.
32. As a **security-conscious owner**, I want high-risk events (block, scope violation, rate limit) visible, so that I know if someone is probing Ask APIs.
33. As a **deal-room viewer (non-admin)**, I want to be unable to create/rebuild KB, so that embedding scope stays admin-controlled.
34. As a **product operator**, I want single-document share links to keep working without room KB UI for now, so that we do not block existing flows before a V2 deprecation.
35. As a **frontend user (zh/en)**, I want all new strings internationalized, so that the product stays bilingual.
36. As a **visitor**, I want the inactive screenshot-blur overlay not to block the Ask sidebar, so that I can still communicate when the window is unfocused.
37. As an **implementer**, I want Ask Docs routed under `/public/links/:publicToken/...`, so that token binding matches Ask Host.
38. As an **implementer**, I want retrieval document IDs for Ask Docs to match Access document scope for deal-room links, so that folder allowlists cannot be bypassed via AI.
39. As an **owner**, I want empty retrieval scope to fail closed (no workspace-wide search), so that misconfiguration never leaks the corpus.
40. As a **visitor**, I want feature-disabled Ask Docs/Host to return clear 403s, so that disabled channels are not usable via raw API.

## Implementation Decisions

### Product / configuration
- Persist Ask Docs / Ask Host as existing booleans (`ai_copilot_enabled`, `qa_enabled`); UI master “Visitor Ask” is OR of the two.
- Advanced count: Visitor Ask counts as **1** when either sub-channel is on.
- Deal-room link save: reject enabling Ask Docs unless room KB status is `ready` or `stale`.
- On save, warn (do not hard-block) when link-authorized documents/folders are not covered by KB selection.
- Launch migration: for rooms without ready/stale KB, force `ai_copilot_enabled=false` on their links.
- File requests remain separate; do not deep-link visitors from Ask empty states into file requests.
- Single-document links: no room KB UI; may keep upload-time embedding until V2 deprecation consideration.

### Knowledge base (deal room)
- One KB per deal room; create/rebuild restricted to room owner/admin.
- Create wizard: default **no** selection; user must opt in folders/documents.
- Folder path follow: new docs under selected paths mark KB `stale`; embed only on rebuild.
- Upload/ingestion for deal-room path: pages/chunks for preview only; **no** automatic embeddings.
- Rebuild: stage new embeddings in `chunk_embedding_builds` while Ask Docs keeps searching live `chunks.embedding` for `ActiveDocumentIds`; promote into live vectors then switch metadata atomically; discard staging on failure.
- Soft stale: Ask Docs remains available when `stale`. During `building`, Ask Docs uses current `ActiveDocumentIds` ∩ Access (previous generation).

### Ask Docs retrieval & answering
- Retrieval set = KB selected & embedded documents ∩ link Access-authorized documents (same scope function as Access document listing for deal rooms).
- Default search across that full set (not current-open document only).
- Never call workspace-wide search for public Ask Docs.
- No evidence → refuse with fixed copy; audit `no_evidence`; optional Host switch CTA.
- Evidence quotes returned to visitors truncated to **320** characters; page jump remains. The same cap is applied when persisting assistant evidence and when projecting Ask Docs audit detail (historical long quotes truncated on read).
- Post-filter evidence document IDs; log `scope_violation` on any drop.

### Anti-bypass / access
- Ask Docs and Ask Host must use the same public access resolution path as page assets (session, security version, gates).
- Every Ask call: re-evaluate allow/block for session email; on failure invalidate session and 403.
- Ask Docs HTTP route must include `publicToken` and require session public token match.
- Rate limits (per visitor+link): Ask Docs 20/10min and 200/day; Ask Host 30/day; **429 `rate_limit_exceeded`** when exceeded (and high-risk security event). Redis/limiter errors **fail closed** with **503 `limiter_unavailable`** (deny, not labeled as visitor rate abuse; no rate_limit security event). Unset limiter skips enforcement (must not happen in production). Visitor UI maps these codes to distinct i18n strings (Ask Docs chat + Ask Host submit).
- Product security events for high-risk only: block, scope_violation, rate limit (not every 401).

### Audit & signals
- V1 audit projection over public assistant sessions/messages plus authorized-scope snapshot and result status; reserve dedicated append-only table later.
- Visibility: room members ∪ workspace admins; full Q&A text.
- Hot window 90 days; then archive (not delete); admins can still open archived items.
- UI: link management audit + room-level timeline; V1 may ship link-side first.
- Keep async question→Signal creation alongside audit; separate UX labels.

### Frontend
- Sidebar tab naming: 沟通 / Ask; modes: 问文档 / 问发起方 (Ask Docs / Ask Host).
- Access tab: single Visitor Ask card with two sub-toggles; i18n en + zh-CN.
- Deal-room documents page: Create KB / Rebuild KB + status strip.

### Phased delivery (release gate)
- Gate-0 (anti-bypass) + Sec-0 (scope alignment) + Audit-1 + Ingest-1 + KB-1 + Mig-1 — **done in code**; remaining UX/test debt in `PLAN-visitor-ask-v1-debt.md` (B3–B7 naming, MSW, security-events UI).
- UX phases (naming/card, visitor polish, room audit summary) + V1.5 channel hint + smoke e2e + SPEC #36 blur — **done**.
- Still open (not this epic / OOS): dedicated append-only audit table; single-document KB product / V2 deprecation.

## Testing Decisions

### What makes a good test
- Assert **external behavior** at HTTP (or documented public client) boundaries: status codes, response bodies, headers, and persisted audit/security side effects visible through APIs.
- Do **not** assert internal RAG pipelines, prompt strings, React component structure, or DB column names unless exposed via API contract.
- Prefer integration tests with real Postgres where the repo already does for link/deal-room scope.

### Agreed seams (highest level)

1. **Public visitor API** — Access session → Ask Docs / Ask Host / page access parity: gates, allow/block revocation, token binding, 429 limits, refusal, quote length, retrieval confined to authorized∩KB, no workspace-wide leak.
2. **Deal-room owner + link save API** — KB create/rebuild/selection, admin authz, no auto-embed on upload, soft stale / rebuild availability, save rejects Ask Docs without ready/stale KB, warn on auth⊄KB.
3. **Owner audit read API** — list/detail visibility, 90-day default vs archive retrieval, separation from Signal surfaces.

Seam 1 is the release-blocking core; 2 and 3 are required for the full product promise.

### Modules under test (behavioral)
- Public link access + public Ask Docs/Host handlers/services.
- Deal-room knowledge-base write APIs and link update validation for Ask Docs.
- Owner audit list/detail APIs (link and/or room).

### Prior art in this repo
- Link deal-room scope integration tests (folder allowlist, Access document listing).
- Public assistant service tests (disabled copilot, SearchInDocuments usage, signals).
- Public handler session tests; `resolvePublicAccess` / session gate unit tests.
- Frontend: UnifiedQAPanel and AccessTab tests for UI contracts after API exists.
- Prefer extending these styles over new unit-only mocks of retrieval internals.

## Out of Scope

- Merging file requests into Visitor Ask.
- Per-link physical vector indexes.
- Fully automatic intent routing without an explicit Ask Docs / Ask Host control (V1.5 maybe).
- Expanding visible link scope via KB.
- Explicit KB product for single-document links (V2 deprecation track).
- V1 dedicated append-only audit table (projection first).
- Training models on audit text.
- Changing NDA/password/email product rules beyond applying the same gates to Ask APIs.

## Further Notes

- Design authority: `docs/designs/plan/visitor-ask-knowledge-base.md` v1.3 (grilling Q1–Q25; V1 shipped with debt).
- Implementation debt / progress: `docs/designs/plan/PLAN-visitor-ask-v1-debt.md`.
- Domain language: Deal Room, Link, Access, Visitor Ask / 沟通, Ask Docs / 问文档, Ask Host / 问发起方, Knowledge Base, folder allowlist, Link session, Signal, security event.
- Hide “RAG/knowledge base” jargon from visitors; owners see KB controls on the deal-room documents page.
- i18n mandatory for all new user-facing strings (en + zh-CN).
