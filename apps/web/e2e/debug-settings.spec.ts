import { test, expect } from "@playwright/test";
import { seedRealBackend, authenticatePage } from "./real-helpers";

test("debug settings load", async ({ page }) => {
  const seed = await seedRealBackend();
  await authenticatePage(page);
  page.on("console", (msg) => console.log("[console]", msg.type(), msg.text()));
  page.on("pageerror", (err) => console.log("[pageerror]", err.message));
  await page.goto(`/${seed.workspaceSlug}/settings/general`);
  await page.waitForTimeout(5000);
  const html = await page.content();
  console.log("HTML snippet:", html.slice(0, 2000));
  await expect(page.getByText("Settings").first()).toBeVisible({ timeout: 30000 });
});
