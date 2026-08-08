/**
 * Reproduce screenshot flow: Ask before AI → enable AI → new question uses AI →
 * re-ask same question must route to Ask Docs (not duplicate Host-only behavior).
 *
 * MSW root cause (fixed): POST /ask ignored ask_ai_enabled unless question had __ai__.
 */
import { test, expect } from "@playwright/test";
import {
  resetMockState,
  attachDebug,
  openVisitorAskPanel,
  setMockLinkAskPolicy,
} from "./helpers";

const SMOKE_TOKEN = "AskSmoke1";
const SMOKE_LINK_ID = "link_visitor_ask_smoke";
const Q_NET_PROFIT_2025 = "2025年净利润多少";
const Q_FORECAST_2027 = "预测2027年净利润多少";
const AI_ANSWER_SNIPPET = /revenue grew 12%/i;

async function submitAskAndParseLane(
  page: import("@playwright/test").Page,
  question: string,
): Promise<{ lane: string; status: string; route_reason?: string; id: string }> {
  const input = page.getByPlaceholder(/materials you can access|Ask the host/i);
  await input.fill(question);
  const postPromise = page.waitForResponse(
    (res) =>
      res.request().method() === "POST" &&
      res.url().includes(`/public/links/${SMOKE_TOKEN}/ask`) &&
      res.status() === 201,
  );
  await page
    .getByRole("button", { name: /Ask/i })
    .and(page.locator('[type="submit"]'))
    .click();
  const res = await postPromise;
  const body = (await res.json()) as {
    data: { id: string; lane: string; status: string; route_reason?: string };
  };
  return body.data;
}

test.describe("Visitor Ask repeat question routing (MSW)", () => {
  test("re-ask same question after enabling AI routes to Ask Docs", async ({ page }) => {
    attachDebug(page);
    await resetMockState(page);
    await setMockLinkAskPolicy(page, SMOKE_LINK_ID, { askAiEnabled: false });
    await openVisitorAskPanel(page, SMOKE_TOKEN);

    const hostFirst = await submitAskAndParseLane(page, Q_NET_PROFIT_2025);
    expect(hostFirst.lane, "AI off should create host lane").toBe("host");

    await setMockLinkAskPolicy(page, SMOKE_LINK_ID, { askAiEnabled: true });
    await openVisitorAskPanel(page, SMOKE_TOKEN);

    const second = await submitAskAndParseLane(page, Q_FORECAST_2027);
    expect(second.lane, `route_reason=${second.route_reason ?? "none"}`).toBe("ai");
    expect(second.status).toBe("ai_streaming");
    await expect(page.getByText(AI_ANSWER_SNIPPET)).toBeVisible({ timeout: 15000 });

    const third = await submitAskAndParseLane(page, Q_NET_PROFIT_2025);
    expect(third.lane, `route_reason=${third.route_reason ?? "none"}`).toBe("ai");
    expect(third.status).toBe("ai_streaming");
    expect(third.id).not.toBe(hostFirst.id);

    const listRes = await page.evaluate(async (token) => {
      const r = await fetch(`/api/v1/public/links/${token}/ask/me`);
      return r.json();
    }, SMOKE_TOKEN);
    const turns = (listRes as { data: Array<{ id: string; question: string; lane: string }> }).data ?? [];
    const profitTurns = turns.filter((t) => t.question === Q_NET_PROFIT_2025);
    expect(profitTurns).toHaveLength(2);
    expect(profitTurns.filter((t) => t.lane === "host")).toHaveLength(1);
    expect(profitTurns.filter((t) => t.lane === "ai")).toHaveLength(1);
  });

  test("with AI already enabled, repeat question always routes to AI", async ({ page }) => {
    attachDebug(page);
    await resetMockState(page);
    await setMockLinkAskPolicy(page, SMOKE_LINK_ID, { askAiEnabled: true });

    await openVisitorAskPanel(page, SMOKE_TOKEN);

    const first = await submitAskAndParseLane(page, Q_NET_PROFIT_2025);
    expect(first.lane).toBe("ai");
    await expect(page.getByText(AI_ANSWER_SNIPPET)).toBeVisible({ timeout: 15000 });

    const second = await submitAskAndParseLane(page, Q_FORECAST_2027);
    expect(second.lane).toBe("ai");

    const third = await submitAskAndParseLane(page, Q_NET_PROFIT_2025);
    expect(third.lane).toBe("ai");
    expect(third.id).not.toBe(first.id);
  });
});
