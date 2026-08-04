/**
 * Regression (MSW): bundle pipeline NDA matches deal-room share —
 * require explicit NDA document selection; never create without it.
 */
import { test, expect } from "@playwright/test";
import { setupAuthenticatedPage, attachDebug, WORKSPACE_SLUG } from "./helpers";

async function gotoSecurityStep(page: import("@playwright/test").Page) {
  await page.goto(`/${WORKSPACE_SLUG}/links/new`);
  const firstCheckbox = page.locator('[data-testid^="bundle-doc-checkbox-"]').first();
  await expect(firstCheckbox).toBeVisible({ timeout: 15000 });
  await firstCheckbox.click();
  await page.locator('[data-testid="pipeline-nav-forward"]').click();
  await expect(page.getByText(/Security Options/i).first()).toBeVisible({ timeout: 10000 });
}

test.describe("Bundle NDA guard (MSW)", () => {
  test("NDA on without document shows picker error and blocks create", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);
    await gotoSecurityStep(page);

    await page.locator('[data-testid="security-switch-ndaEnabled"]').click();
    await expect(page.getByTestId("security-nda-document-select")).toBeVisible({
      timeout: 5000,
    });
    await expect(page.getByTestId("security-nda-document-error")).toBeVisible();
    await expect(page.getByTestId("security-nda-document-error")).toContainText(
      /select an NDA|选择 NDA/i,
    );

    // Contact required when NDA forces email verification — pick one if present.
    const contactTrigger = page.locator('[data-testid="contact-selector-trigger"]');
    if (await contactTrigger.isVisible({ timeout: 3000 }).catch(() => false)) {
      await contactTrigger.click();
      const option = page.locator('[data-testid^="contact-option-"]').first();
      if (await option.isVisible({ timeout: 3000 }).catch(() => false)) {
        await option.click();
      }
    }

    await page.locator('[data-testid="pipeline-nav-forward"]').click();
    await expect(page.locator('[data-testid="review-submit-button"]')).toBeVisible({
      timeout: 10000,
    });
    await page.locator('[data-testid="review-submit-button"]').click();

    await expect(page.getByText(/select an NDA|选择 NDA/i).first()).toBeVisible({
      timeout: 5000,
    });
    await expect(page.locator('[data-testid="generated-link"]')).toHaveCount(0);
  });

  test("NDA with selected agreement document allows create", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);
    await gotoSecurityStep(page);

    await page.locator('[data-testid="security-switch-ndaEnabled"]').click();
    await expect(page.getByTestId("security-nda-document-select")).toBeVisible({
      timeout: 5000,
    });

    await page.getByTestId("security-nda-document-select").click();
    const option = page.getByRole("option").first();
    await expect(option).toBeVisible({ timeout: 5000 });
    const optionLabel = (await option.textContent())?.trim() ?? "";
    expect(optionLabel.length).toBeGreaterThan(0);
    await option.click();
    await expect(page.getByTestId("security-nda-document-error")).toHaveCount(0);

    const contactTrigger = page.locator('[data-testid="contact-selector-trigger"]');
    await expect(contactTrigger).toBeVisible({ timeout: 5000 });
    await contactTrigger.click();
    await page.locator('[data-testid^="contact-option-"]').first().click();

    await page.locator('[data-testid="pipeline-nav-forward"]').click();
    await expect(page.locator('[data-testid="review-submit-button"]')).toBeVisible({
      timeout: 10000,
    });

    const createReq = page.waitForRequest(
      (req) =>
        req.method() === "POST" &&
        /\/api\/workspaces\/[^/]+\/links$/.test(new URL(req.url()).pathname),
    );
    await page.locator('[data-testid="review-submit-button"]').click();
    const req = await createReq;
    const body = req.postDataJSON() as {
      require_nda?: boolean;
      nda_document_id?: string;
      nda_template_id?: string;
      document_ids?: string[];
    };
    expect(body.require_nda).toBe(true);
    expect(Boolean(body.nda_document_id || body.nda_template_id)).toBe(true);
    // Must not silently bind the shared content document as the NDA agreement.
    if (body.nda_document_id && body.document_ids?.length) {
      expect(body.document_ids).not.toContain(body.nda_document_id);
    }

    await expect(page.locator('[data-testid="generated-link"]')).toBeVisible({
      timeout: 15000,
    });
  });
});
