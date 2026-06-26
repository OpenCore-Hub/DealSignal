import { defineConfig, devices } from "@playwright/test";

const TEST_PORT = 5175;

export default defineConfig({
  testDir: "./e2e",
  testIgnore: ["**/real-backend.spec.ts"],
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
