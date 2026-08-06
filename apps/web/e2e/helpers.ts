import type { Page } from "@playwright/test";

export const WORKSPACE_SLUG = "acme-capital";

/** Poll until MSW handles `/__e2e/*` (worker.start is async after document load). */
async function waitForMsw(page: Page) {
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

export function attachDebug(page: Page) {
  page.on("console", (msg) => {
    console.log(`[browser ${msg.type()}] ${msg.text()}`);
  });
  page.on("pageerror", (err) => {
    console.log(`[browser error] ${err.message}`);
  });
}
