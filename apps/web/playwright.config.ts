import { defineConfig, devices } from "@playwright/test";

const TEST_PORT = 5175;

export default defineConfig({
  testDir: "./e2e",
  // MSW mock e2e only — specs that hit a live API use playwright.real.config.ts.
  testIgnore: ["**/real-backend.spec.ts", "**/*-real.spec.ts"],
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: "list",
  use: {
    baseURL: `http://localhost:${TEST_PORT}`,
    trace: "on-first-retry",
    locale: "en-US",
  },
  projects: [
    {
      name: "chromium",
      testIgnore: [
        "**/real-backend.spec.ts",
        "**/*-real.spec.ts",
        "**/visitor-ask-owner-reply-loop.spec.ts",
        "**/visitor-ask-ai-stream.spec.ts",
        "**/visitor-ask-host-inbox.spec.ts",
        "**/visitor-ask-engage-policy.spec.ts",
      ],
      use: { ...devices["Desktop Chrome"] },
    },
    {
      name: "visitor-ask-serial",
      testMatch: [
        "**/visitor-ask-owner-reply-loop.spec.ts",
        "**/visitor-ask-ai-stream.spec.ts",
        "**/visitor-ask-host-inbox.spec.ts",
        "**/visitor-ask-engage-policy.spec.ts",
        "**/visitor-ask-repeat-routing.spec.ts",
      ],
      fullyParallel: false,
      workers: 1,
      use: { ...devices["Desktop Chrome"] },
    },
  ],
  webServer: {
    command: `pnpm dev --port ${TEST_PORT}`,
    env: { VITE_API_BASE_URL: "" },
    url: `http://localhost:${TEST_PORT}`,
    reuseExistingServer: false,
    timeout: 120 * 1000,
  },
});
