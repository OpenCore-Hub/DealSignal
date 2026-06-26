import { test, expect } from "@playwright/test";

const API_BASE_URL = process.env.REAL_API_BASE_URL || "http://localhost:8080";

async function apiFetch(input: string, init?: RequestInit): Promise<Response> {
  return fetch(`${API_BASE_URL}${input}`, {
    headers: { "Content-Type": "application/json", ...init?.headers },
    ...init,
  });
}

async function seed() {
  const ts = Date.now();
  const email = `debug-${ts}@example.com`;
  const password = "Password123!";
  const reg = await apiFetch("/api/auth/register", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });
  const { access_token: token } = (await reg.json()) as { access_token: string };
  const slug = `debug-${ts}`;
  await apiFetch("/api/workspaces", {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
    body: JSON.stringify({ name: "Debug", slug, brand_color: "#0055ff" }),
  });
  return { token, slug };
}

test("debug settings load", async ({ page }) => {
  const s = await seed();
  await page.addInitScript((t: string) => {
    localStorage.setItem("access_token", t);
  }, s.token);
  page.on("console", (msg) => console.log("[console]", msg.type(), msg.text()));
  page.on("pageerror", (err) => console.log("[pageerror]", err.message));
  await page.goto(`/${s.slug}/settings/general`);
  await page.waitForTimeout(5000);
  const html = await page.content();
  console.log("HTML snippet:", html.slice(0, 2000));
  await expect(page.getByText("Settings").first()).toBeVisible({ timeout: 30000 });
});
