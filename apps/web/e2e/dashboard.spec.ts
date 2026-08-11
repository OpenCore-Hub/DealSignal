import { test, expect } from "@playwright/test";
import {
  seedRealBackend,
  seedDocument,
  authenticatePage,
  attachDebug,
} from "./real-helpers";

let workspaceSlug: string;

test.describe("Deal Radar (real backend)", () => {
  test.beforeAll(async () => {
    const seed = await seedRealBackend();
    workspaceSlug = seed.workspaceSlug;
  // Document seed exercises deal-room surfaces; deep radar contract is
  // apps/web/e2e/deal-radar-real.spec.ts (access-request → evidence → PATCH).
  await seedDocument(workspaceSlug);
  });

  test("renders Deal Radar shell and role lens", async ({ page }) => {
    attachDebug(page);
    await authenticatePage(page);
    await page.goto(`/${workspaceSlug}/dashboard`);

    await expect(
      page.getByRole("heading", { name: "Deal Radar" }),
    ).toBeVisible({ timeout: 10000 });

    await expect(page.getByTestId("radar-queue")).toBeVisible({
      timeout: 10000,
    });
    await expect(page.getByTestId("radar-lens-founder")).toBeVisible();
    await expect(page.getByTestId("radar-lens-investor_ir")).toBeVisible();
    await expect(page.getByTestId("radar-lens-sales")).toBeVisible();

    await expect(page.getByTestId("radar-cleared-today")).toBeVisible();
    await expect(page.getByTestId("radar-evidence-rail")).toBeVisible();
    await expect(page.getByTestId("radar-insights-link")).toBeVisible();
  });

  test("empty or loaded focus surface is real (not collage UI)", async ({
    page,
  }) => {
    attachDebug(page);
    await authenticatePage(page);
    await page.goto(`/${workspaceSlug}/dashboard`);

    await expect(
      page.getByRole("heading", { name: "Deal Radar" }),
    ).toBeVisible({ timeout: 10000 });

    // Removed collage dashboard must stay gone.
    await expect(page.getByText("Hot signals", { exact: true })).toHaveCount(0);
    await expect(page.getByText("Heat map", { exact: true })).toHaveCount(0);
    await expect(
      page.getByText("Recent documents", { exact: true }),
    ).toHaveCount(0);

    const emptyClear = page.getByText(/You're clear for now|Create data room/i);
    const focusHeading = page.getByText(/Today'?s focus/i);
    await expect(emptyClear.or(focusHeading).first()).toBeVisible({
      timeout: 10000,
    });
  });

  test("role lens updates circle query param", async ({ page }) => {
    attachDebug(page);
    await authenticatePage(page);
    await page.goto(`/${workspaceSlug}/dashboard`);

    await expect(
      page.getByRole("heading", { name: "Deal Radar" }),
    ).toBeVisible({ timeout: 10000 });

    await page.getByTestId("radar-lens-sales").click();
    await expect(page).toHaveURL(/circle=sales/, { timeout: 5000 });

    await page.getByTestId("radar-lens-investor_ir").click();
    await expect(page).toHaveURL(/circle=investor_ir/, { timeout: 5000 });
  });

  test("Analyze in Insights navigates to insights overview", async ({
    page,
  }) => {
    attachDebug(page);
    await authenticatePage(page);
    await page.goto(`/${workspaceSlug}/dashboard`);

    await expect(page.getByTestId("radar-insights-link")).toBeVisible({
      timeout: 10000,
    });
    await page.getByTestId("radar-insights-link").click();
    await expect(page).toHaveURL(
      new RegExp(`/${workspaceSlug}/insights/overview`),
      { timeout: 10000 },
    );
  });
});
