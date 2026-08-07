import type { Page } from "@playwright/test";

export const WORKSPACE_SLUG = "acme-capital";

/** Poll until MSW handles `/__e2e/*` (worker.start is async after document load). */
export async function waitForMsw(page: Page) {
  const deadline = Date.now() + 15_000;
  while (Date.now() < deadline) {
    const status = await page.evaluate(async () => {
      try {
        const r = await fetch("/__e2e/reset", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ action: "ping" }),
        });
        return r.status;
      } catch {
        return 0;
      }
    });
    if (status === 204) return;
    await new Promise((r) => setTimeout(r, 50));
  }
  throw new Error("MSW not ready: /__e2e/reset ping never returned 204");
}

export async function resetMockState(page: Page) {
  await page.goto("/");
  await waitForMsw(page);
  const res = await page.evaluate(async () => {
    const r = await fetch("/__e2e/reset", { method: "POST" });
    return r.status;
  });
  if (res !== 204) {
    throw new Error(`resetMockState failed: HTTP ${res}`);
  }
}

async function authenticate(page: Page) {
  // App auth gate checks auth_session cookie (see router.tsx), not localStorage.
  await page.context().addCookies([
    {
      name: "auth_session",
      value: "1",
      url: "http://localhost:5175",
      sameSite: "Lax",
    },
  ]);
}

export async function setupAuthenticatedPage(page: Page) {
  await resetMockState(page);
  await authenticate(page);
  await page.goto(`/${WORKSPACE_SLUG}/dashboard`);
  // Full navigation reloads the document; wait until the worker controls fetch again
  // or evaluate() can hit Vite's SPA HTML fallback (200) instead of MSW.
  await waitForMsw(page);
}

/** Authenticate without wiping MSW mock state (e.g. after a visitor flow in another context). */
export async function authenticatePageOnly(page: Page) {
  await page.goto("/");
  await waitForMsw(page);
  await authenticate(page);
}

/** Navigate to a workspace route after auth, ensuring MSW controls API fetches. */
export async function gotoAuthenticated(page: Page, path: string) {
  await page.context().addCookies([
    {
      name: "auth_session",
      value: "1",
      url: "http://localhost:5175",
      sameSite: "Lax",
    },
  ]);
  await page.goto(`/${WORKSPACE_SLUG}/dashboard`);
  await waitForMsw(page);
  await page.goto(path);
  await waitForMsw(page);
}

/** Like gotoAuthenticated but waits for a GET API response (e.g. owner ask inbox). */
export async function gotoAuthenticatedWaitForApi(
  page: Page,
  path: string,
  urlIncludes: string,
) {
  await page.context().addCookies([
    {
      name: "auth_session",
      value: "1",
      url: "http://localhost:5175",
      sameSite: "Lax",
    },
  ]);
  await page.goto(`/${WORKSPACE_SLUG}/dashboard`);
  await waitForMsw(page);
  const responsePromise = page.waitForResponse(
    (res) =>
      res.request().method() === "GET" &&
      res.url().includes(urlIncludes) &&
      res.ok(),
    { timeout: 20000 },
  );
  await page.goto(path);
  await waitForMsw(page);
  await responsePromise;
}

/** Force MSW session ask to return JSON 429 before opening SSE (busy / RPM / quota). */
export async function setMockKnowledgeAskGate(
  page: Page,
  opts: { code: string; httpStatus?: number } | { clear: true },
) {
  await page.goto("/");
  await waitForMsw(page);
  const res = await page.evaluate(async (payload) => {
    const r = await fetch("/__e2e/reset", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ action: "knowledge-ask-gate", ...payload }),
    });
    return r.status;
  }, opts);
  if (res !== 204) {
    throw new Error(`setMockKnowledgeAskGate failed: HTTP ${res}`);
  }
}

/** Force MSW knowledge corpus status for a room (A5: building / not ready). */
export async function setMockKnowledgeCorpus(
  page: Page,
  roomId: string,
  opts: { status?: string; documentStatus?: string; jobStatus?: string } = {},
) {
  await page.goto("/");
  await waitForMsw(page);
  const res = await page.evaluate(
    async ({ roomId: id, status, documentStatus, jobStatus }) => {
      const r = await fetch("/__e2e/reset", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          action: "knowledge-corpus",
          roomId: id,
          status,
          documentStatus,
          jobStatus,
        }),
      });
      return r.status;
    },
    {
      roomId,
      status: opts.status ?? "syncing",
      documentStatus: opts.documentStatus ?? "syncing",
      jobStatus: opts.jobStatus ?? "running",
    },
  );
  if (res !== 204) {
    throw new Error(`setMockKnowledgeCorpus failed: HTTP ${res}`);
  }
}

export async function setMockLinkAskPolicy(
  page: Page,
  linkId: string,
  policy: { askAiEnabled?: boolean },
) {
  await page.goto("/");
  await waitForMsw(page);
  const res = await page.evaluate(
    async ({ linkId: id, askAiEnabled }) => {
      const r = await fetch("/__e2e/reset", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          action: "link-ask-policy",
          linkId: id,
          ...(typeof askAiEnabled === "boolean" ? { askAiEnabled } : {}),
        }),
      });
      return r.status;
    },
    { linkId, askAiEnabled: policy.askAiEnabled },
  );
  if (res !== 204) {
    throw new Error(`setMockLinkAskPolicy failed: HTTP ${res}`);
  }
}

export function attachDebug(page: Page) {
  page.on("console", (msg) => {
    console.log(`[browser ${msg.type()}] ${msg.text()}`);
  });
  page.on("pageerror", (err) => {
    console.log(`[browser error] ${err.message}`);
  });
}

/** Open the visitor Ask panel on a public link (unified Ask UI). */
export async function openVisitorAskPanel(page: Page, token: string) {
  await page.goto(`/l/${token}`);
  await page.locator("img[alt*='Page']").first().waitFor({ state: "visible", timeout: 15000 });

  const openSidebar = page.getByRole("button", { name: /Open sidebar/i });
  if (await openSidebar.isVisible().catch(() => false)) {
    await openSidebar.click();
  }

  const input = page.getByPlaceholder(/materials you can access|Ask the host/i);
  if (!(await input.isVisible().catch(() => false))) {
    const askTab = page.locator("button.rounded-full").filter({ hasText: /^Ask$/ });
    if (await askTab.isVisible().catch(() => false)) {
      await askTab.click();
    }
  }

  await input.waitFor({ state: "visible", timeout: 10000 });
  return input;
}
