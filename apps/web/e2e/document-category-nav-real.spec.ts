/**
 * Deal room document → detail → smart back navigation (real backend UI).
 */
import { test, expect } from "@playwright/test";
import path from "path";
import { fileURLToPath } from "url";
import fs from "fs";
import {
  seedRealBackend,
  seedDealRoom,
  attachDocumentToRoom,
  fetchDocument,
  authenticatePage,
  attachDebug,
  apiFetch,
} from "./real-helpers";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

test.describe("Deal room document detail navigation (real backend)", () => {
  test("opens document detail from room and returns via smart back", async ({ page }) => {
    attachDebug(page);
    const seed = await seedRealBackend();
    const room = await seedDealRoom(seed.workspaceSlug, {
      name: "Nav Test Room",
      templateType: "seed",
    });

    // Upload as general; attach promotes to deal_room (POST deal_room is rejected).
    const buffer = fs.readFileSync(path.join(__dirname, "fixtures", "sample.pdf"));
    const file = new File([buffer], "room-doc.pdf", { type: "application/pdf" });
    const form = new FormData();
    form.append("file", file);

    const uploadRes = await apiFetch(`/api/workspaces/${seed.workspaceSlug}/documents`, {
      method: "POST",
      body: form,
    });
    expect(uploadRes.ok).toBe(true);
    const uploaded = (await uploadRes.json()) as { id: string; title: string };
    expect((await fetchDocument(seed.workspaceSlug, uploaded.id)).category).toBe("general");

    // Wait for ready + attach to room
    for (let i = 0; i < 30; i++) {
      const doc = await fetchDocument(seed.workspaceSlug, uploaded.id);
      if (doc.status === "ready") break;
      await new Promise((r) => setTimeout(r, 1000));
    }

    const attachRes = await attachDocumentToRoom(seed.workspaceSlug, room.id, uploaded.id);
    expect([200, 201]).toContain(attachRes.status);

    await authenticatePage(page);
    await page.goto(`/${seed.workspaceSlug}/deal-rooms/${room.id}`);
    await expect(page.getByRole("heading", { name: "Nav Test Room" })).toBeVisible({ timeout: 15000 });

    const folderTree = page.locator('[data-testid="folder-tree"]');
    await expect(folderTree).toBeVisible({ timeout: 10000 });
    await expect(folderTree.getByText(uploaded.title, { exact: false })).toBeVisible({ timeout: 15000 });

    await folderTree.getByText(uploaded.title, { exact: false }).click();
    await expect(page).toHaveURL(new RegExp(`/documents/${uploaded.id}`), { timeout: 10000 });
    await expect(page.getByText("Data room")).toBeVisible({ timeout: 5000 });

    await page.getByRole("button", { name: /Back to data rooms/i }).click();
    await expect(page).toHaveURL(new RegExp(`/deal-rooms/${room.id}`), { timeout: 10000 });
    await expect(page.getByRole("heading", { name: "Nav Test Room" })).toBeVisible();
  });
});
