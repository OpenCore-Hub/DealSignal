/**
 * Deal Radar — six high-value product boundary E2E (MSW).
 * Seeds GET /radar via `/__e2e/reset` action `radar-feed` (Cache-backed).
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

type Product = (typeof PRODUCTS)[number];

function workItem(
  product: Product,
  over: Record<string, unknown> = {},
): Record<string, unknown> {
  const id = `radar_${product}_${String(over.idSuffix ?? "1")}`;
  return {
    id,
    product,
    headline: `Boundary ${product}`,
    subtitle: `why ${product}`,
    verb:
      product === "diligence_gate"
        ? "approve"
        : product === "commitment_ask"
          ? "reply"
          : product === "buying_window"
            ? "email"
            : product === "access_decay"
              ? "renew"
              : "review",
    priority: "high",
    slaDueAt: "2026-08-09T12:00:00Z",
    createdAt: "2026-08-08T12:00:00Z",
    dealKey: `deal:${product}`,
    dealName: `Deal ${product}`,
    actionId: id,
    navigatePath: `/${WORKSPACE_SLUG}/deal-rooms/room_1?tab=access`,
    evidence: [{ kind: "gate", count: 1 }],
    whyNowCode: product,
    state: "open",
    scenario: "startup-fundraising",
    ...over,
  };
}

function buildBoundaryFeed(extraItems: Record<string, unknown>[] = []) {
  const items = [...PRODUCTS.map((p) => workItem(p)), ...extraItems];
  const counts: Record<string, number> = { all: items.length };
  for (const p of PRODUCTS) {
    counts[p] = items.filter((i) => i.product === p).length;
  }
  return {
    nextUp: items[0],
    strands: [
      {
        dealKey: "boundary",
        dealName: "Boundary strand",
        scenario: "startup-fundraising",
        items,
      },
    ],
    items,
    clearedToday: 0,
    counts,
    lens: "founder",
    defaultLens: "founder",
    lensSource: "default",
    scenarioPack: {
      scenario: "startup-fundraising",
      defaultCircle: "founder",
      depth: "p0",
    },
    noiseHints: [
      {
        product: "leak_watch",
        falsePositiveRate: 0.4,
        sample: 5,
        demoteBoost: 2,
      },
    ],
  };
}

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

async function seedRadarFeed(page: Page, feed: ReturnType<typeof buildBoundaryFeed>) {
  await waitForMsw(page);
  const status = await page.evaluate(async (body) => {
    const r = await fetch("/__e2e/reset", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    return r.status;
  }, { action: "radar-feed", feed });
  if (status !== 204) {
    throw new Error(`seedRadarFeed failed: HTTP ${status}`);
  }
}

async function openRadarWithFeed(
  page: Page,
  feed: ReturnType<typeof buildBoundaryFeed>,
) {
  await resetMockState(page);
  await authenticate(page);
  await seedRadarFeed(page, feed);
  await page.goto(`/${WORKSPACE_SLUG}/dashboard`);
  await waitForMsw(page);
  await expect(page.getByTestId("radar-queue")).toBeVisible({ timeout: 15000 });
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

test.describe("Radar six-product boundaries (MSW)", () => {
  test("filter chips expose all six products with correct counts", async ({ page }) => {
    attachDebug(page);
    await openRadarWithFeed(page, buildBoundaryFeed());

    await expect(page.getByTestId("radar-filter-all")).toBeVisible();

    for (const p of PRODUCTS) {
      const chip = page.getByTestId(`radar-filter-${p}`);
      await expect(chip).toBeVisible();
      await expect(chip.locator("span.tabular-nums")).toHaveText("1");
    }

    await page.getByTestId("radar-filter-diligence_gate").click();
    await expandStrands(page);
    await expect(page.getByTestId("radar-row")).toHaveCount(1);
    await expect(page.getByTestId("radar-row")).toContainText(/Boundary diligence_gate/i);

    await page.getByTestId("radar-filter-abuse_guard").click();
    await expandStrands(page);
    await expect(page.getByTestId("radar-row")).toHaveCount(1);
    await expect(page.getByTestId("radar-row")).toContainText(/Boundary abuse_guard/i);
  });

  test("false-positive empty-actor NDA title is not injected by boundary fixture", async ({
    page,
  }) => {
    attachDebug(page);
    const feed = buildBoundaryFeed([
      workItem("diligence_gate", {
        idSuffix: "nda",
        // Evidence / subtitle must name the visitor — never "from  for".
        headline: "NDA signature required from lp@vc.com for Startup Fundraising",
        subtitle: "NDA signature required from lp@vc.com for Startup Fundraising",
        actor: "lp@vc.com",
        contactEmail: "lp@vc.com",
      }),
    ]);
    await openRadarWithFeed(page, feed);

    await page.getByTestId("radar-filter-diligence_gate").click();
    await expandStrands(page);

    await expect(page.getByText(/from\s+for/i)).toHaveCount(0);
    await expect(page.getByTestId("radar-row").filter({ hasText: /lp@vc\.com/i })).toHaveCount(1);
  });

  test("noise hint banner surfaces closed-loop demotion", async ({ page }) => {
    attachDebug(page);
    await openRadarWithFeed(page, buildBoundaryFeed());
    await expect(page.getByTestId("radar-noise-hints")).toBeVisible();
    await expect(page.getByTestId("radar-noise-hints")).toContainText(/leak/i);
  });

  test("sales lens re-ranks next-up to buying_window", async ({ page }) => {
    attachDebug(page);
    const feed = buildBoundaryFeed();
    const productOf = (i: Record<string, unknown>) => String(i.product ?? "");
    // Force gate-first order so sales lens must promote buying_window to next-up.
    const gate = feed.items.find((i) => productOf(i) === "diligence_gate");
    const buy = feed.items.find((i) => productOf(i) === "buying_window");
    if (!gate || !buy) throw new Error("boundary feed missing gate/buy items");
    const rest = feed.items.filter(
      (i) => productOf(i) !== "diligence_gate" && productOf(i) !== "buying_window",
    );
    const gateFirst = {
      ...feed,
      items: [gate, buy, ...rest],
      nextUp: gate,
      strands: [
        {
          dealKey: "boundary",
          dealName: "Boundary strand",
          scenario: "startup-fundraising",
          items: [gate, buy, ...rest],
        },
      ],
    };
    await openRadarWithFeed(page, gateFirst);

    await page.getByTestId("radar-lens-sales").click();
    await expect(page).toHaveURL(/circle=sales/, { timeout: 5000 });
    await expect(page.getByTestId("radar-next-up")).toContainText(
      /Boundary buying_window/i,
      { timeout: 10000 },
    );
  });

  test("default MSW feed emits abuse_guard from structured eventType", async ({
    page,
  }) => {
    attachDebug(page);
    await resetMockState(page);
    await authenticate(page);
    // No radar-feed override — use fixture-derived getMockRadarFeed (sig_6).
    await page.goto(`/${WORKSPACE_SLUG}/dashboard`);
    await waitForMsw(page);
    await expect(page.getByTestId("radar-queue")).toBeVisible({ timeout: 15000 });

    const abuseChip = page.getByTestId("radar-filter-abuse_guard");
    await expect(abuseChip).toBeVisible();
    const countText = await abuseChip.locator("span.tabular-nums").innerText();
    expect(Number(countText)).toBeGreaterThan(0);

    await abuseChip.click();
    await expandStrands(page);
    await expect(page.getByTestId("radar-row").first()).toBeVisible();
  });

  test("stress: 60 mixed rows keep filters and keyboard focus stable", async ({ page }) => {
    attachDebug(page);

    const extra: Record<string, unknown>[] = [];
    for (let i = 0; i < 54; i += 1) {
      const p = PRODUCTS[i % PRODUCTS.length];
      extra.push(
        workItem(p, {
          idSuffix: `stress_${i}`,
          dealKey: `stress:${i % 9}`,
          dealName: `Stress deal ${i % 9}`,
          headline: `Stress ${p} #${i}`,
        }),
      );
    }
    const feed = buildBoundaryFeed(extra);
    await openRadarWithFeed(page, feed);

    await expect(page.getByTestId("radar-filter-all").locator("span.tabular-nums")).toHaveText(
      String(feed.counts.all),
    );

    for (const p of PRODUCTS) {
      await page.getByTestId(`radar-filter-${p}`).click();
      await expandStrands(page);
      const n = await page.getByTestId("radar-row").count();
      expect(n).toBe(feed.counts[p]);
      expect(n).toBeGreaterThan(0);
    }

    await page.getByTestId("radar-filter-all").click();
    await expandStrands(page);
    await page.keyboard.press("j");
    await page.keyboard.press("k");
    await expect(page.getByTestId("radar-queue")).toBeVisible();
  });
});
