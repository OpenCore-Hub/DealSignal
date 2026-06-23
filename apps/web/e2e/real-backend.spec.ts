import { test, expect } from "@playwright/test";
import fs from "fs";
import path from "path";
import { fileURLToPath } from "url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const API_BASE_URL = process.env.REAL_API_BASE_URL || "http://localhost:8080";
const FIXTURES_DIR = path.join(__dirname, "fixtures");

interface SeedResult {
  token: string;
  workspaceSlug: string;
}

async function apiFetch(input: string, init?: RequestInit): Promise<Response> {
  return fetch(`${API_BASE_URL}${input}`, {
    headers: { "Content-Type": "application/json", ...init?.headers },
    ...init,
  });
}

async function seedRealBackend(): Promise<SeedResult> {
  const timestamp = Date.now();
  const email = `e2e-${timestamp}@example.com`;
  const password = "Password123!";

  const registerRes = await apiFetch("/api/auth/register", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });
  if (!registerRes.ok) {
    throw new Error(`register failed: ${registerRes.status} ${await registerRes.text()}`);
  }
  const { token } = (await registerRes.json()) as { token: string };

  const workspaceSlug = `e2e-${timestamp}`;
  const workspaceRes = await apiFetch("/api/workspaces", {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
    body: JSON.stringify({ name: "E2E Workspace", slug: workspaceSlug, brand_color: "#0055ff" }),
  });
  if (!workspaceRes.ok) {
    throw new Error(`workspace create failed: ${workspaceRes.status} ${await workspaceRes.text()}`);
  }

  return { token, workspaceSlug };
}

let seed: SeedResult;

test.beforeAll(async () => {
  seed = await seedRealBackend();
});

async function authenticate(page: import("@playwright/test").Page, token: string) {
  await page.addInitScript((t: string) => {
    localStorage.setItem("access_token", t);
  }, token);
}

function attachDebug(page: import("@playwright/test").Page) {
  page.on("console", (msg) => {
    console.log(`[browser ${msg.type()}] ${msg.text()}`);
  });
  page.on("pageerror", (err) => {
    console.log(`[browser error] ${err.message}`);
  });
}

test.describe("real backend P0 flow", () => {
  test("workspace list loads the seeded workspace", async ({ page }) => {
    attachDebug(page);
    await authenticate(page, seed.token);
    await page.goto("/");

    // With a single workspace, the page auto-redirects to its dashboard.
    await expect(page).toHaveURL(new RegExp(`/${seed.workspaceSlug}/dashboard`), { timeout: 30000 });
  });

  test("upload page renders and accepts a file", async ({ page }) => {
    attachDebug(page);
    await authenticate(page, seed.token);
    await page.goto(`/${seed.workspaceSlug}/documents/upload`);
    await expect(page.getByRole("heading", { name: "Upload Document" })).toBeVisible({ timeout: 30000 });

    const fileInput = page.locator("input#file-upload");
    await fileInput.setInputFiles(path.join(FIXTURES_DIR, "sample.pdf"));

    await expect(page.getByText("sample.pdf")).toBeVisible();
    await expect(page.getByTestId("upload-success")).toBeVisible({ timeout: 30000 });
  });

  test("upload dialog from top nav uploads a file and document appears in the list", async ({ page }) => {
    attachDebug(page);
    await authenticate(page, seed.token);
    await page.goto(`/${seed.workspaceSlug}/dashboard`);

    await page.getByRole("button", { name: "Upload document" }).click();

    const dialog = page.getByRole("dialog", { name: "Upload Document" });
    await expect(dialog).toBeVisible({ timeout: 30000 });

    const fileInput = dialog.locator("input#file-upload");
    await fileInput.setInputFiles(path.join(FIXTURES_DIR, "sample.pdf"));

    await expect(dialog.getByText("sample.pdf")).toBeVisible();
    await expect(dialog.getByTestId("upload-success")).toBeVisible({ timeout: 30000 });

    await page.goto(`/${seed.workspaceSlug}/documents`);
    await expect(page.getByText("sample.pdf").first()).toBeVisible({ timeout: 30000 });
  });

  test("create a link for a document and open the public share URL", async ({ page }) => {
    attachDebug(page);
    await authenticate(page, seed.token);

    // Upload a document via UI so we have a ready document.
    await page.goto(`/${seed.workspaceSlug}/documents/upload`);
    await expect(page.getByRole("heading", { name: "Upload Document" })).toBeVisible({ timeout: 30000 });
    await page.locator("input#file-upload").setInputFiles(path.join(FIXTURES_DIR, "sample.pdf"));
    await expect(page.getByTestId("upload-success")).toBeVisible({ timeout: 30000 });

    await page.goto(`/${seed.workspaceSlug}/documents`);
    await page.getByText("sample.pdf").first().click();
    await expect(page.getByRole("button", { name: "Create link" })).toBeVisible({ timeout: 30000 });
    await page.getByRole("button", { name: "Create link" }).click();

    await expect(page).toHaveURL(new RegExp(`/${seed.workspaceSlug}/links/new`), { timeout: 30000 });
    await page.getByTestId("create-link-button").click();

    await expect(page.getByTestId("generated-link")).toBeVisible({ timeout: 30000 });
  });
});
