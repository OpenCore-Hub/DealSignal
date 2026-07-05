import type { Page } from "@playwright/test";

export const WORKSPACE_SLUG = "acme-capital";

export async function resetMockState(page: Page) {
  await page.goto("/");
  await page.evaluate(() =>
    fetch("/__e2e/reset", { method: "POST" })
  );
}

async function authenticate(page: Page) {
  await page.evaluate(() => {
    localStorage.setItem("access_token", "mock_e2e_token");
  });
}

export async function setupAuthenticatedPage(page: Page) {
  await resetMockState(page);
  await authenticate(page);
  await page.goto(`/${WORKSPACE_SLUG}/dashboard`);
}

export function attachDebug(page: Page) {
  page.on("console", (msg) => {
    console.log(`[browser ${msg.type()}] ${msg.text()}`);
  });
  page.on("pageerror", (err) => {
    console.log(`[browser error] ${err.message}`);
  });
}
