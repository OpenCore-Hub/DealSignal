import { defineConfig } from "@playwright/test";

/** API-only real-backend specs — no Vite webServer (avoids port conflicts). */
export default defineConfig({
  testDir: "./e2e",
  testMatch: [
    "**/document-category-tristate-real.spec.ts",
    "**/visitor-ask-real.spec.ts",
    "**/visitor-ask-ai-stream-real.spec.ts",
  ],
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  reporter: "list",
  timeout: 120_000,
  use: {
    baseURL: process.env.PLAYWRIGHT_BASE_URL || "http://localhost:5173",
  },
  expect: { timeout: 30_000 },
});
