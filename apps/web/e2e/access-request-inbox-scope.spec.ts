/**
 * Access-request inbox isolation (MSW).
 *
 * Document Library → Share must never show deal-room share applicants.
 * Deal Room → Access must never show document-library share applicants.
 * Room membership requests remain on the deal-room surface only.
 */
import { test, expect } from "@playwright/test";
import { setupAuthenticatedPage, attachDebug, WORKSPACE_SLUG } from "./helpers";

const DOC_APPLICANT = "doc-share-applicant@example.com";
const ROOM_LINK_APPLICANT = "room-share-applicant@example.com";
const ROOM_MEMBER_APPLICANT = "marcus@boldstart.vc";

test.describe("Access request inbox scope (MSW)", () => {
  test("document share inbox excludes deal-room share applicants", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/documents?tab=shared`);
    await expect(page.getByRole("tab", { name: /Shared/i })).toBeVisible({ timeout: 15000 });

    // Inbox may take a tick after links load.
    await expect(page.getByTestId("share-access-requests-panel")).toBeVisible({ timeout: 15000 });
    await expect(page.getByText(DOC_APPLICANT)).toBeVisible();
    await expect(page.getByText(ROOM_LINK_APPLICANT)).toHaveCount(0);
    await expect(page.getByText(ROOM_MEMBER_APPLICANT)).toHaveCount(0);

    // Network contract: document surface always asks for scope=document.
    const pending = await page.evaluate(async () => {
      const r = await fetch(
        "/api/workspaces/acme-capital/links/pending-access-requests?scope=document",
      );
      const body = (await r.json()) as { data: { email: string }[] };
      return { status: r.status, emails: body.data.map((x) => x.email) };
    });
    expect(pending.status).toBe(200);
    expect(pending.emails).toContain(DOC_APPLICANT);
    expect(pending.emails).not.toContain(ROOM_LINK_APPLICANT);
  });

  test("deal-room access inbox includes room + room-link applicants only", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/deal-rooms/room_1?tab=access`);
    await expect(page.getByTestId("deal-room-access-control-tab")).toBeVisible({ timeout: 15000 });
    await expect(page.getByTestId("deal-room-access-requests-panel")).toBeVisible({ timeout: 15000 });

    await expect(page.getByText(ROOM_LINK_APPLICANT)).toBeVisible();
    await expect(page.getByText(ROOM_MEMBER_APPLICANT)).toBeVisible();
    await expect(page.getByText(DOC_APPLICANT)).toHaveCount(0);

    // No duplicate per-link inbox on the access tab.
    await expect(page.getByTestId("link-access-requests")).toHaveCount(0);

    const pending = await page.evaluate(async () => {
      const r = await fetch(
        "/api/workspaces/acme-capital/links/pending-access-requests?scope=deal_room&deal_room_id=room_1",
      );
      const body = (await r.json()) as { data: { email: string }[] };
      return { status: r.status, emails: body.data.map((x) => x.email) };
    });
    expect(pending.status).toBe(200);
    expect(pending.emails).toContain(ROOM_LINK_APPLICANT);
    expect(pending.emails).not.toContain(DOC_APPLICANT);
  });

  test("pending API rejects invalid scope contracts", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    const results = await page.evaluate(async () => {
      const probe = async (qs: string) => {
        const r = await fetch(
          `/api/workspaces/acme-capital/links/pending-access-requests?${qs}`,
        );
        let code: string;
        try {
          const body = (await r.json()) as { code?: string };
          code = body.code ?? "";
        } catch {
          // keep empty code on non-JSON responses
        }
        return { status: r.status, code };
      };
      return {
        missingRoomId: await probe("scope=deal_room"),
        emptyRoomId: await probe("scope=deal_room&deal_room_id="),
        badScope: await probe("scope=all"),
      };
    });

    expect(results.missingRoomId).toEqual({ status: 400, code: "invalid_input" });
    expect(results.emptyRoomId).toEqual({ status: 400, code: "invalid_input" });
    expect(results.badScope).toEqual({ status: 400, code: "invalid_input" });
  });
});
