/**
 * Deal Radar — six-product stress seed + Evidence rail UX acceptance (MSW).
 * Seeds via `/__e2e/reset` action `radar-stress` (72 rows by default).
 */
import { test, expect, type Page } from "@playwright/test";
import {
  attachDebug,
  waitForMsw,
  resetMockState,
  WORKSPACE_SLUG,
} from "./helpers";

const PRODUCTS = [
  "buying_window",
  "diligence_gate",
  "commitment_ask",
  "leak_watch",
  "access_decay",
  "abuse_guard",
] as const;

async function authenticate(page: Page) {
  await page.context().addCookies([
    {
      name: "auth_session",
      value: "1",
      url: "http://localhost:5175",
      sameSite: "Lax" as const,
    },
  ]);
}

async function seedRadarStress(page: Page, perProduct = 12) {
  await waitForMsw(page);
  const result = await page.evaluate(async (n) => {
    const r = await fetch("/__e2e/reset", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ action: "radar-stress", perProduct: n }),
    });
    const body = (await r.json().catch(() => ({}))) as {
      items?: number;
      counts?: Record<string, number>;
    };
    return { status: r.status, body };
  }, perProduct);
  if (result.status !== 200) {
    throw new Error(`radar-stress seed failed: HTTP ${result.status}`);
  }
  return result.body;
}

async function expandStrands(page: Page) {
  const strands = page.getByTestId("radar-strand");
  const count = await strands.count();
  for (let i = 0; i < count; i += 1) {
    const header = strands.nth(i).locator("button[aria-expanded]").first();
    if ((await header.count()) === 0) continue;
    if ((await header.getAttribute("aria-expanded")) === "false") {
      await header.click();
    }
  }
}

async function openStressedRadar(page: Page, perProduct = 12) {
  await resetMockState(page);
  await authenticate(page);
  const seeded = await seedRadarStress(page, perProduct);
  await page.goto(`/${WORKSPACE_SLUG}/dashboard`);
  await waitForMsw(page);
  await expect(page.getByTestId("radar-queue")).toBeVisible({ timeout: 15000 });
  return seeded;
}

test.describe("Radar six-product stress UX (MSW)", () => {
  test("loads dense feed: filters, strands, noise, keyboard, evidence by product", async ({
    page,
  }) => {
    attachDebug(page);
    const seeded = await openStressedRadar(page, 12);
    const total = seeded.items ?? 72;

    await expect(page.getByTestId("radar-filter-all").locator("span.tabular-nums")).toHaveText(
      String(total),
    );
    await expect(page.getByTestId("radar-noise-hints")).toBeVisible();
    await expect(page.getByTestId("radar-strands")).toBeVisible();
    await expect(page.getByTestId("radar-cleared-today")).toBeVisible();

    for (const p of PRODUCTS) {
      const chip = page.getByTestId(`radar-filter-${p}`);
      await expect(chip.locator("span.tabular-nums")).toHaveText("12");
    }

    // Walk each product: filter → expand → select first row → Evidence rail product shape.
    for (const p of PRODUCTS) {
      await page.getByTestId(`radar-filter-${p}`).click();
      await expandStrands(page);
      const rows = page.getByTestId("radar-row");
      await expect(rows).toHaveCount(12);
      await rows.first().click();
      const rail = page.getByTestId("radar-evidence-rail");
      await expect(rail).toBeVisible();

      if (p === "diligence_gate") {
        await expect(page.getByTestId("radar-evidence-access-request")).toBeVisible({
          timeout: 10000,
        });
        await expect(page.getByTestId("radar-evidence-applicant")).toBeVisible();
        await expect(page.getByTestId("radar-evidence-gate-timeline")).toBeVisible();
        await expect(page.getByTestId("radar-evidence-security-events")).toContainText("4×");
        await expect(page.getByTestId("radar-evidence-gate-no-opens")).toBeVisible();
        await expect(page.getByTestId("radar-evidence-metrics")).toHaveCount(0);
        await expect(page.getByTestId("radar-evidence-open")).toContainText(/Share|分享/i);
      } else {
        // Non-gate products should show engagement metrics (not the gate empty-state).
        await expect(page.getByTestId("radar-evidence-metrics")).toBeVisible({
          timeout: 10000,
        });
        await expect(page.getByTestId("radar-evidence-why-now")).toBeVisible();
        await expect(page.getByTestId("radar-evidence-access-request")).toHaveCount(0);
      }
    }

    await page.getByTestId("radar-filter-all").click();
    await expandStrands(page);
    await page.keyboard.press("j");
    await page.keyboard.press("k");
    await expect(page.getByTestId("radar-queue")).toBeVisible();
    await expect(page.getByTestId("radar-evidence-rail")).toBeVisible();
  });

  test("manual seed action returns balanced counts payload", async ({ page }) => {
    attachDebug(page);
    await resetMockState(page);
    await authenticate(page);
    const body = await seedRadarStress(page, 8);
    expect(body.items).toBe(48);
    for (const p of PRODUCTS) {
      expect(body.counts?.[p]).toBe(8);
    }
  });
});
