import { test, expect } from "@playwright/test";
import path from "path";
import { fileURLToPath } from "url";
import { setupAuthenticatedPage, WORKSPACE_SLUG, attachDebug } from "./helpers";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

test.describe("Deal room folder upload", () => {
  test("uploads a file into a folder and shows it in the folder tree", async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);

    await page.goto(`/${WORKSPACE_SLUG}/deal-rooms/room_1`);
    await expect(page.getByRole("heading", { name: "Seed Round Due Diligence" })).toBeVisible();

    // The folder tree should show at least one folder.
    const folderRow = page.getByRole("button", { name: /01 Pitch Deck/i });
    await expect(folderRow).toBeVisible();

    // Hover to reveal the upload icon.
    await folderRow.hover();
    const uploadButton = folderRow.locator("button[aria-label='Add file']");
    await expect(uploadButton).toBeVisible();

    // Intercept the upload request so we can verify it is actually sent.
    const uploadPromise = page.waitForRequest((req) =>
      req.url().includes(`/api/workspaces/${WORKSPACE_SLUG}/documents`) && req.method() === "POST"
    );
    const addToRoomPromise = page.waitForRequest((req) =>
      req.url().includes(`/api/workspaces/${WORKSPACE_SLUG}/deal-rooms/room_1/documents`) && req.method() === "POST"
    );

    // Click the upload icon and select a file.
    await uploadButton.click();
    const fileInput = folderRow.locator('[data-testid="folder-upload-input-/pitch"]');
    await fileInput.setInputFiles(path.join(__dirname, "fixtures", "sample.pdf"));

    // Wait for the network requests.
    const uploadReq = await uploadPromise;
    const addToRoomReq = await addToRoomPromise;

    expect(uploadReq).toBeTruthy();
    expect(addToRoomReq).toBeTruthy();

    // Verify the add-to-room payload targets the correct folder.
    const addBody = await addToRoomReq.postDataJSON();
    expect(addBody.folder_path).toBe("/pitch");

    // The upload dashboard should appear.
    await expect(page.getByText("Upload progress")).toBeVisible();

    // The folder tree should eventually list the uploaded document.
    await expect(page.getByText(/^sample\.pdf$/).first()).toBeVisible();
  });
});
