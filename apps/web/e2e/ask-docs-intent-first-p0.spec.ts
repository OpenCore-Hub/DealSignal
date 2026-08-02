/**
 * Ask Docs Intent-first P0 (MSW) — acceptance cases from
 * docs/designs/plan/ask-docs-intent-first-clue-engine.md §8.1.
 */
import { test, expect } from "@playwright/test";
import { resetMockState, attachDebug, setupAuthenticatedPage, WORKSPACE_SLUG } from "./helpers";

const SMOKE_TOKEN = "AskSmoke1";

async function openAskSidebar(page: import("@playwright/test").Page) {
  await page.goto(`/l/${SMOKE_TOKEN}`);
  await expect(page.locator("img[alt*='Page']").first()).toBeVisible({ timeout: 15000 });
  const openSidebar = page.getByRole("button", { name: /Open sidebar/i });
  if (await openSidebar.isVisible().catch(() => false)) {
    await openSidebar.click();
  }
  const askTab = page.locator('button[type="button"]').filter({ hasText: /^Ask$/ });
  await expect(askTab).toBeVisible({ timeout: 10000 });
  await askTab.click();
  await expect(
    page.getByText(/Ask Docs first; switch to Ask Host if you need missing materials/i),
  ).toBeVisible({ timeout: 10000 });
}

async function askDocs(
  page: import("@playwright/test").Page,
  message: string,
): Promise<Record<string, unknown>> {
  const input = page.getByPlaceholder(/Ask about materials authorized for this link/i);
  await expect(input).toBeVisible({ timeout: 5000 });

  const respPromise = page.waitForResponse(
    (r) =>
      r.url().includes("/assistant/chat") &&
      r.request().method() === "POST" &&
      r.status() === 200,
  );
  await input.fill(message);
  await page.getByRole("button", { name: "Ask", exact: true }).and(page.locator('[type="submit"]')).click();
  const resp = await respPromise;
  return (await resp.json()) as Record<string, unknown>;
}

test.describe("Ask Docs Intent-first P0 (MSW)", () => {
  test("topic bare term returns extractive-style answer without definition phrasing", async ({
    page,
  }) => {
    attachDebug(page);
    await resetMockState(page);
    await openAskSidebar(page);

    const body = await askDocs(page, "财务数据");
    expect(body).not.toHaveProperty("doc_intent");
    expect(body).not.toHaveProperty("generation_mode");
    expect(body.result_status).toBe("success");
    expect(String(body.answer)).toMatch(/Found these related excerpts/i);
    expect(String(body.answer)).not.toMatch(/是指|定义为|means that|is defined as/i);

    await expect(page.getByText(/Found these related excerpts/i)).toBeVisible({ timeout: 10000 });
    await expect(page.getByText(/Revenue grew 3x year over year/i).first()).toBeVisible();
  });

  test("locate clause paste returns template locator + single evidence", async ({ page }) => {
    attachDebug(page);
    await resetMockState(page);
    await openAskSidebar(page);

    const body = await askDocs(page, "__intent_locate__ 请定位第 12 条转让限制");
    expect(body).not.toHaveProperty("doc_intent");
    expect(body.result_status).toBe("success");
    expect(String(body.answer)).toMatch(/Located the following excerpt/i);
    const evidence = body.evidence as unknown[];
    expect(Array.isArray(evidence) && evidence.length).toBe(1);

    await expect(page.getByText(/Located the following excerpt/i)).toBeVisible({ timeout: 10000 });
  });

  test("list intent returns controlled inventory answer", async ({ page }) => {
    attachDebug(page);
    await resetMockState(page);
    await openAskSidebar(page);

    const body = await askDocs(page, "有哪些财务指标");
    expect(body).not.toHaveProperty("doc_intent");
    expect(String(body.answer)).toMatch(/Revenue growth|Gross margin/i);
    await expect(page.getByText(/Revenue growth/i)).toBeVisible({ timeout: 10000 });
  });

  test("qa intent returns grounded judgment", async ({ page }) => {
    attachDebug(page);
    await resetMockState(page);
    await openAskSidebar(page);

    const body = await askDocs(page, "是否可转让");
    expect(body).not.toHaveProperty("doc_intent");
    expect(String(body.answer)).toMatch(/written consent/i);
    await expect(page.getByText(/written consent/i)).toBeVisible({ timeout: 10000 });
  });

  test("out_of_corpus refuse shows Host CTA for visitor", async ({ page }) => {
    attachDebug(page);
    await resetMockState(page);
    await openAskSidebar(page);

    const body = await askDocs(page, "__intent_refuse__ 市场惯例是什么");
    expect(body).not.toHaveProperty("doc_intent");
    expect(body.result_status).toBe("out_of_corpus");
    expect(body.suggest_ask_host).toBe(true);

    await expect(page.getByText(/outside what the authorized materials can support/i)).toBeVisible({
      timeout: 10000,
    });
    await expect(page.getByRole("button", { name: /Ask the host instead/i })).toBeVisible({
      timeout: 5000,
    });
  });

  test("no_evidence still offers Ask Host CTA", async ({ page }) => {
    attachDebug(page);
    await resetMockState(page);
    await openAskSidebar(page);

    const body = await askDocs(page, "__no_evidence__ Is there an SOC2 report?");
    expect(body.result_status).toBe("no_evidence");
    expect(body.suggest_ask_host).toBe(true);
    await expect(page.getByRole("button", { name: /Ask the host instead/i })).toBeVisible({
      timeout: 5000,
    });
  });

  test("absence miss returns not_found_in_scope with Ask Host CTA", async ({ page }) => {
    attachDebug(page);
    await resetMockState(page);
    await openAskSidebar(page);

    const body = await askDocs(page, "有没有竞业限制");
    expect(body).not.toHaveProperty("doc_intent");
    expect(body).not.toHaveProperty("absence");
    expect(body.result_status).toBe("not_found_in_scope");
    expect(body.suggest_ask_host).toBe(true);
    expect(String(body.answer)).toMatch(/could not find that clause or topic/i);
    await expect(page.getByRole("button", { name: /Ask the host instead/i })).toBeVisible({
      timeout: 5000,
    });
  });

  test("absence hit answers from materials without not_found status", async ({ page }) => {
    attachDebug(page);
    await resetMockState(page);
    await openAskSidebar(page);

    const body = await askDocs(page, "__absence_hit__ 有没有竞业限制");
    expect(body.result_status).toBe("success");
    expect(body.suggest_ask_host).toBe(false);
    expect(String(body.answer)).toMatch(/竞业限制|non-compete/i);
  });

  test("party list question succeeds without exposing party on chat API", async ({ page }) => {
    attachDebug(page);
    await resetMockState(page);
    await openAskSidebar(page);

    const body = await askDocs(page, "投资人有哪些权利");
    expect(body).not.toHaveProperty("doc_intent");
    expect(body).not.toHaveProperty("party");
    expect(body.result_status).toBe("success");
    expect(String(body.answer)).toMatch(/authorized materials|Revenue|margin|expenses/i);
  });

  test("locate without strong literal degrades to topic-style multi-excerpt answer", async ({
    page,
  }) => {
    attachDebug(page);
    await resetMockState(page);
    await openAskSidebar(page);

    const body = await askDocs(page, "__intent_locate_fallback__ pasted clause without literal hit");
    expect(body).not.toHaveProperty("doc_intent");
    expect(body.result_status).toBe("success");
    expect(String(body.answer)).toMatch(/Found these related excerpts/i);
    const evidence = body.evidence as unknown[];
    expect(Array.isArray(evidence) && evidence.length).toBeLessThanOrEqual(3);
    expect(Array.isArray(evidence) && evidence.length).toBeGreaterThan(1);

    await expect(page.getByText(/Found these related excerpts/i)).toBeVisible({ timeout: 10000 });
  });

  test("owner refuse is neutral with no Ask Host CTA (G1/G12)", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);
    await expect(page.getByText(/Dashboard|Overview|Documents/i).first()).toBeVisible({
      timeout: 15000,
    });

    // Hit a known MSW route first so the worker is confirmed active.
    const health = await page.evaluate(async (slug) => {
      const res = await fetch(`/api/workspaces/${slug}/members`);
      return res.status;
    }, WORKSPACE_SLUG);
    expect(health).toBe(200);

    const body = await page.evaluate(async (slug) => {
      const res = await fetch(`/api/workspaces/${slug}/assistant/chat`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ message: "请给投资建议" }),
      });
      const text = await res.text();
      let json: unknown = null;
      try {
        json = text ? JSON.parse(text) : null;
      } catch {
        json = { raw: text };
      }
      return { ok: res.ok, status: res.status, json };
    }, WORKSPACE_SLUG);
    expect(body.status, JSON.stringify(body)).toBe(200);
    const payload = body.json as Record<string, unknown>;
    expect(payload).not.toHaveProperty("doc_intent");
    expect(payload.result_status).toBe("out_of_corpus");
    expect(payload.suggest_ask_host).toBeFalsy();
    expect(String(payload.answer)).toMatch(/will not invent an answer/i);
    expect(String(payload.answer)).not.toMatch(/ask the host/i);
    expect(String(payload.answer)).toMatch(/human judgment|add materials/i);
  });

  test("owner topic path stays extractive-shaped and omits intent fields", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);
    await expect(page.getByText(/Dashboard|Overview|Documents/i).first()).toBeVisible({
      timeout: 15000,
    });

    const body = await page.evaluate(async (slug) => {
      const res = await fetch(`/api/workspaces/${slug}/assistant/chat`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ message: "财务数据" }),
      });
      const text = await res.text();
      let json: unknown = null;
      try {
        json = text ? JSON.parse(text) : null;
      } catch {
        json = { raw: text };
      }
      return { ok: res.ok, status: res.status, json };
    }, WORKSPACE_SLUG);
    expect(body.status, JSON.stringify(body)).toBe(200);
    const payload = body.json as Record<string, unknown>;
    expect(payload).not.toHaveProperty("doc_intent");
    expect(payload).not.toHaveProperty("generation_mode");
    expect(payload.result_status).toBe("success");
    expect(String(payload.answer)).toMatch(/Found these related excerpts/i);
    expect(String(payload.answer)).not.toMatch(/是指|定义为|is defined as/i);
  });
});
