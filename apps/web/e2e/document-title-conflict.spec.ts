/**
 * Regression (MSW): live library same-title upload returns 409 document_exists
 * unless replace=true (aligned with upload CreateDocument conflict path).
 */
import { test, expect } from "@playwright/test";
import { setupAuthenticatedPage, attachDebug, WORKSPACE_SLUG } from "./helpers";

const TITLE = "regress-title-conflict.pdf";

async function uploadDoc(
  page: import("@playwright/test").Page,
  opts: { replace?: boolean } = {},
) {
  return page.evaluate(
    async ({ slug, title, replace }) => {
      const file = new File([new Uint8Array([0x25, 0x50, 0x44, 0x46])], title, {
        type: "application/pdf",
      });
      const form = new FormData();
      form.append("file", file);
      if (replace) form.append("replace", "true");
      const res = await fetch(`/api/workspaces/${slug}/documents`, {
        method: "POST",
        body: form,
      });
      const body = (await res.json()) as {
        id?: string;
        title?: string;
        code?: string;
        document?: { id?: string; title?: string };
      };
      return {
        status: res.status,
        code: body.code,
        id: body.id ?? body.document?.id,
        title: body.title ?? body.document?.title,
      };
    },
    { slug: WORKSPACE_SLUG, title: TITLE, replace: !!opts.replace },
  );
}

test.describe("Document title conflict (MSW)", () => {
  test("duplicate title without replace → 409 document_exists", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    const first = await uploadDoc(page);
    expect(first.status).toBe(201);
    expect(first.title).toBe(TITLE);

    const conflict = await uploadDoc(page);
    expect(conflict.status).toBe(409);
    expect(conflict.code).toBe("document_exists");
    expect(conflict.title).toBe(TITLE);
  });

  test("duplicate title with replace=true → 201 and keeps id/title", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    const first = await uploadDoc(page);
    expect([201, 409]).toContain(first.status);

    const replaced = await uploadDoc(page, { replace: true });
    expect(replaced.status).toBe(201);
    expect(replaced.title).toBe(TITLE);
    expect(replaced.id).toBeTruthy();
    if (first.status === 201 && first.id) {
      expect(replaced.id).toBe(first.id);
    }
  });
});
