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
  documentId: string;
  linkId: string;
  publicToken: string;
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

  // 1. Register
  const registerRes = await apiFetch("/api/auth/register", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });
  if (!registerRes.ok) {
    throw new Error(`register failed: ${registerRes.status} ${await registerRes.text()}`);
  }
  const { token } = (await registerRes.json()) as { token: string };

  // 2. Create workspace
  const workspaceSlug = `e2e-${timestamp}`;
  const workspaceRes = await apiFetch("/api/workspaces", {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
    body: JSON.stringify({ name: "E2E Workspace", slug: workspaceSlug, brand_color: "#0055ff" }),
  });
  if (!workspaceRes.ok) {
    throw new Error(`workspace create failed: ${workspaceRes.status} ${await workspaceRes.text()}`);
  }

  // 3. Upload PDF
  const pdfPath = path.join(FIXTURES_DIR, "sample.pdf");
  if (!fs.existsSync(pdfPath)) {
    throw new Error(`PDF fixture not found: ${pdfPath}`);
  }
  const formData = new FormData();
  formData.append("file", new Blob([fs.readFileSync(pdfPath)]), "sample.pdf");

  const uploadRes = await fetch(`${API_BASE_URL}/api/workspaces/${workspaceSlug}/documents`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
    body: formData,
  });
  if (!uploadRes.ok) {
    throw new Error(`upload failed: ${uploadRes.status} ${await uploadRes.text()}`);
  }
  const uploadJson = (await uploadRes.json()) as { id: string };
  const documentId = uploadJson.id;

  // 4. Poll until ready
  for (let i = 0; i < 30; i++) {
    await new Promise((r) => setTimeout(r, 1000));
    const statusRes = await fetch(`${API_BASE_URL}/api/workspaces/${workspaceSlug}/documents/${documentId}/status`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    const statusJson = (await statusRes.json()) as { status: string };
    if (statusJson.status === "ready") break;
    if (statusJson.status === "failed") {
      throw new Error("document ingestion failed");
    }
  }

  // 5. Create link
  const linkRes = await apiFetch(`/api/workspaces/${workspaceSlug}/links`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
    body: JSON.stringify({ document_id: documentId, name: "E2E Link", permission_type: "public", download_enabled: true }),
  });
  if (!linkRes.ok) {
    throw new Error(`link create failed: ${linkRes.status} ${await linkRes.text()}`);
  }
  const linkJson = (await linkRes.json()) as { id: string; public_token: string };

  return {
    token,
    workspaceSlug,
    documentId,
    linkId: linkJson.id,
    publicToken: linkJson.public_token,
  };
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

test.describe("real backend P0 flow", () => {
  test("dashboard loads for seeded workspace", async ({ page }) => {
    await authenticate(page, seed.token);
    await page.goto(`/${seed.workspaceSlug}/dashboard`);
    await expect(page).toHaveURL(new RegExp(`/${seed.workspaceSlug}/dashboard`));
    await expect(page.getByRole("heading", { name: "Deal Radar" })).toBeVisible();
  });

  test("upload a document via UI", async ({ page }) => {
    await authenticate(page, seed.token);
    await page.goto(`/${seed.workspaceSlug}/documents/upload`);
    await expect(page.getByRole("heading", { name: "Upload Document" })).toBeVisible();

    const fileInput = page.locator("input#file-upload");
    await fileInput.setInputFiles(path.join(FIXTURES_DIR, "sample.pdf"));

    await expect(page.getByText("sample.pdf")).toBeVisible();
    await expect(page.getByTestId("upload-success")).toBeVisible({ timeout: 30000 });
  });

  test("create a smart link and view analytics", async ({ page }) => {
    await authenticate(page, seed.token);
    await page.goto(`/${seed.workspaceSlug}/links/new`);
    await expect(page.getByRole("heading", { name: "Smart Link Creator" })).toBeVisible();

    await expect(page.getByTestId("selected-document")).not.toHaveText("Not selected");

    await page.getByTestId("create-link-button").click();
    await expect(page.getByTestId("generated-link")).toBeVisible();

    const generatedLink = await page.getByTestId("generated-link").textContent();
    expect(generatedLink).toContain("http");

    await page.goto(`/${seed.workspaceSlug}/links`);
    await expect(page.getByRole("heading", { name: "Share Links" })).toBeVisible();

    await page.locator('[data-testid="links-table-row"]').first().click();
    await expect(page).toHaveURL(/\/links\//);

    await expect(page.getByText("Total Visits")).toBeVisible();
    await expect(page.getByText("Unique Visitors")).toBeVisible();
    await expect(page.getByText("Access Log")).toBeVisible();
  });
});
