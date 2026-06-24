import { test, expect } from "@playwright/test";
import { setupAuthenticatedPage, resetMockState, attachDebug } from "./helpers";

test.describe("auth & workspace flows", () => {
  test.beforeEach(async ({ page }) => {
    attachDebug(page);
    await resetMockState(page);
  });

  test("register with validation and then log in", async ({ page }) => {
    const email = `e2e-${Date.now()}@example.com`;
    const password = "Password123!";

    await page.goto("/register");
    await expect(page.getByText("Create account").first()).toBeVisible();

    await page.getByLabel("Email").fill(email);
    await page.getByLabel("Password").fill("short");
    await page.getByRole("button", { name: "Create account" }).click();
    await expect(page.getByText("password must be at least 8 characters")).toBeVisible();

    await page.getByLabel("Password").fill(password);
    await page.getByRole("button", { name: "Create account" }).click();

    await expect(page).toHaveURL(/\/login\?registered=true/);
    await expect(page.getByText("Registration successful")).toBeVisible();

    await page.getByLabel("Email").fill(email);
    await page.getByLabel("Password").fill("WrongPassword1!");
    await page.getByRole("button", { name: "Sign in" }).click();
    await expect(page.getByText("invalid email or password")).toBeVisible();

    await page.getByLabel("Password").fill(password);
    await page.getByRole("button", { name: "Sign in" }).click();

    // login redirects to workspace selector; choose workspace to reach dashboard
    await expect(page).toHaveURL("/");
    await expect(page.getByRole("heading", { name: "Select workspace" })).toBeVisible();
    await page.getByTestId("workspace-card-acme-capital").click();
    await expect(page).toHaveURL(/\/acme-capital\/dashboard/);
  });

  test("log out returns to login", async ({ page }) => {
    await setupAuthenticatedPage(page);
    await expect(page.getByRole("heading", { name: "Deal Radar" })).toBeVisible();

    await page.getByLabel("Account menu").click();
    await page.getByRole("menuitem", { name: "Log out" }).click();

    await expect(page).toHaveURL("/login");
  });

  test("workspace list and creation", async ({ page }) => {
    await page.evaluate(() => localStorage.setItem("access_token", "mock_e2e_token"));
    await page.goto("/");

    await expect(page.getByRole("heading", { name: "Select workspace" })).toBeVisible();
    await expect(page.getByTestId("workspace-card-acme-capital")).toBeVisible();
    await expect(page.getByTestId("workspace-card-ventura-fund")).toBeVisible();

    await page.getByTestId("workspace-card-acme-capital").click();
    await expect(page).toHaveURL(/\/acme-capital\/dashboard/);

    await page.goto("/workspaces/new");
    await expect(page.getByText("Create workspace").first()).toBeVisible();

    await page.getByLabel("Workspace name").fill("E2E Workspace");
    await expect(page.getByLabel("Workspace slug")).toHaveValue("e2e-workspace");

    await page.getByRole("button", { name: "Create workspace" }).click();
    await expect(page).toHaveURL(/\/e2e-workspace\/dashboard/);
  });

  test("verify email page shows success", async ({ page }) => {
    await page.goto("/verify-email/mock-token");
    await expect(page.getByText("Email verified").first()).toBeVisible();
    await expect(page.getByText("email verified successfully")).toBeVisible();
  });
});
