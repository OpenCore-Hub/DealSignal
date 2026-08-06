# Release: Document Category Tri-State

Use this template when shipping migration **128** and the associated API/web changes.
Replace placeholders in `{{angle brackets}}` before publishing.

---

## Summary

Documents now use a three-way library partition: **`general`** (文档库), **`deal_room`** (数据室专属), and **`agreement`** (协议库). Category is the source of truth for list filtering and lifecycle; `deal_room_documents` still controls folder placement.

This release adds migration `128_document_category_deal_room`, API transition guards, UI badges, and CI/staging verification gates.

---

## What's included

### Database (migration 128)

- `documents.category` CHECK extended to `general | agreement | deal_room`
- Backfill: rows in any deal room with `category=general` → `deal_room`
- Historical agreements already in rooms **keep** `agreement` (new attaches blocked)

### API behavior

| Event | Category change |
|--------|-----------------|
| Upload to library | `general` (default) |
| Upload to agreements page | `agreement` |
| Upload into deal room | `general` on POST → `deal_room` after attach |
| Add library doc to room | `general` → `deal_room` |
| Remove from **last** room | `deal_room` → `general` |
| PATCH category | `general` ↔ `agreement` only; `deal_room` immutable via API |
| POST/PATCH `category=deal_room` | **400** `category_deal_room_via_api` |
| Add agreement to room | **400** `agreement_not_allowed_in_deal_room` |

### Frontend

- Document library lists `category=general`
- Share-content picker uses `category=general`
- Category badges on detail + table rows (non-general)
- Add-to-room hidden for `agreement` / `deal_room` docs

---

## Compatibility

| Client | Status |
|--------|--------|
| New web (this release) | Uses `category=general` / `category=agreement` |
| Old web with `exclude_deal_room` + `exclude_agreement` | **Still supported** — mapped server-side |
| Setting `category=deal_room` via PATCH | **Rejected** (400/409) — managed by room membership |

No action required for mobile/scripts that only read documents; update list calls to `category=` when convenient.

---

## Deploy steps

1. **Build & push** API + web images from `main` (CI `docker-push-*` on merge).
2. **Staging pull & restart:**
   ```bash
   cd {{STAGING_COMPOSE_PATH}}
   docker pull {{API_IMAGE}}
   docker pull {{WEB_IMAGE}}
   API_IMAGE={{API_IMAGE}} WEB_IMAGE={{WEB_IMAGE}} docker compose up -d
   ```
3. API runs migrations on startup — confirm `128_document_category_deal_room.up.sql` applied.
4. **Smoke verify** (from dev machine or staging bastion):

   ```bash
   # Quick category gate (~30s) — also proves migration behavior
   BASE_URL=https://{{STAGING_API_HOST}} ./apps/api/e2e-staging-verify.sh

   # Optional: full P0
   BASE_URL=https://{{STAGING_API_HOST}} ./apps/api/e2e-staging-verify.sh --full

   # Optional: on staging host with docker-compose access
   COMPOSE_DIR={{STAGING_COMPOSE_PATH}} BASE_URL=... ./apps/api/e2e-staging-verify.sh --migration-check
   ```

5. **Manual UI spot-check** (2 min):
   - Documents page: no deal-room uploads listed
   - Deal room upload → doc shows **Data room** badge on detail
   - Agreement doc: no “Add to Deal Room” action
   - NDA picker: agreement library only (no room docs)

6. Promote to production with same image tags after staging sign-off.

---

## Verification checklist

- [ ] `/healthz` returns OK on staging API
- [ ] `./e2e-staging-verify.sh` exits 0 (category gate)
- [ ] Migration 128 in `schema_migrations` (if checked on host)
- [ ] Library list `GET .../documents?category=general` excludes in-room docs
- [ ] Attach general doc to room → `category=deal_room`
- [ ] Agreement attach to room → 400
- [ ] Web deployed with matching tag

---

## Rollback

**If rollback before migration 128 applied:** deploy previous API image; no DB change needed.

**If rollback after migration 128 applied:**

1. Deploy previous API/web images.
2. Run down migration manually if reverting CHECK (optional — old code treats unknown categories as general in most paths):
   ```bash
   psql -f apps/api/internal/db/migrations/128_document_category_deal_room.down.sql
   ```
3. Down sets all `deal_room` → `general` and restores two-value CHECK.

**Data note:** Documents promoted to `deal_room` remain in `deal_room_documents`; down migration only resets category column, not room membership.

---

## Monitoring (first 24h)

Watch for:

- Spike in **400** `agreement_not_allowed_in_deal_room` (should be rare — indicates UI bypass or API client)
- **409** `category_immutable` / `category_while_in_room` on PATCH
- Support tickets: “document missing from library” → likely still attached to a room (`deal_room` partition)

---

## Related PRs / commits

- {{PR_LINK_OR_COMMIT_RANGE}}

## Test commands (local)

```bash
# Backend P0 + category
cd apps/api && BASE_URL=http://localhost:8090 PDF=../web/e2e/fixtures/sample.pdf ./e2e-test.sh

# Staging-style quick gate
BASE_URL=http://localhost:8090 ./e2e-staging-verify.sh

# Playwright API gate
cd apps/web && REAL_API_BASE_URL=http://localhost:8090 pnpm test:e2e:category-real
```
