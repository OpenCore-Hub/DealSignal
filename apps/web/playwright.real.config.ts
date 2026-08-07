import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  testMatch: "**/*.spec.ts",
  // Dedicated gates in ./e2e-visitor-ask-real.sh — skip here to avoid duplicate runs.
  testIgnore: [
    "**/visitor-ask-real.spec.ts",
    "**/visitor-ask-owner-reply-real.spec.ts",
    "**/visitor-ask-dashboard-nav-real.spec.ts",
    "**/visitor-ask-ai-stream-real.spec.ts",
    "**/visitor-ask-ai-ui-real.spec.ts",
    "**/visitor-ask-engage-policy-real.spec.ts",
  ],
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: 1,
  reporter: "list",
  timeout: 60_000,
  expect: { timeout: 15_000 },
  use: {
    baseURL: "http://localhost:5173",
    trace: "on-first-retry",
    locale: "en-US",
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
  webServer: {
    command: "pnpm dev",
    env: {
      VITE_API_BASE_URL:
        process.env.VITE_API_BASE_URL ||
        process.env.REAL_API_BASE_URL ||
        "http://localhost:8080",
    },
    url: "http://localhost:5173",
    reuseExistingServer: false,
    timeout: 120 * 1000,
  },
});
