/**
 * 端至端模拟人工排列组合 100% 覆盖测试 —— 安全选项功能验收
 *
 * 覆盖范围:
 *   - 6 个布尔开关的 2^6 = 64 种笛卡尔积组合
 *   - 有效期(4 档) × 最大访问次数(4 档) = 16 种高级设置组合
 *   - 关键组合的完整创建链路验证 (UI → Mock API → 返回结果)
 *
 * 验收标准:
 *   1. 每个组合下 UI 开关状态与预期一致
 *   2. 跨选项约束正确生效 (whitelist/NDA 自动开启邮箱验证)
 *   3. Review 页功能标签与开关状态一一对应
 *   4. 提交到后端的 payload 字段正确
 */

import { test, expect, type Page } from "@playwright/test";
import { setupAuthenticatedPage, WORKSPACE_SLUG, attachDebug } from "./helpers";

// ----------------------------------------------------------------------------
// 常量定义
// ----------------------------------------------------------------------------

const BOOL_FIELDS = [
  { key: "requireEmailVerification", label: "Email verification code", testId: "security-switch-requireEmailVerification" },
  { key: "whitelistEnabled", label: "Whitelist emails/domains", testId: "security-switch-whitelistEnabled" },
  { key: "passwordEnabled", label: "Access password", testId: "security-switch-passwordEnabled" },
  { key: "ndaEnabled", label: "NDA agreement", testId: "security-switch-ndaEnabled" },
  { key: "allowDownload", label: "Allow download", testId: "security-switch-allowDownload" },
  { key: "watermarkEnabled", label: "Dynamic watermark", testId: "security-switch-watermarkEnabled" },
] as const;

type BoolKey = (typeof BOOL_FIELDS)[number]["key"];
type BoolCombo = Record<BoolKey, boolean>;

const EXPIRY_VALUES = [7, 30, 90, "custom"] as const;
const MAX_VIEWS_VALUES = ["unlimited", 10, 50, 100] as const;

const TEST_DOC_ID = "doc_1";
const TEST_CONTACT_ID = "contact_1";

// ----------------------------------------------------------------------------
// 辅助函数
// ----------------------------------------------------------------------------

function generateBoolCombinations(): BoolCombo[] {
  const total = 1 << BOOL_FIELDS.length;
  const results: BoolCombo[] = [];
  for (let i = 0; i < total; i++) {
    const combo = {} as BoolCombo;
    for (let j = 0; j < BOOL_FIELDS.length; j++) {
      combo[BOOL_FIELDS[j].key] = ((i >> j) & 1) === 1;
    }
    results.push(combo);
  }
  return results;
}

async function gotoSecurityStep(page: Page) {
  await page.goto(`/${WORKSPACE_SLUG}/links/new`);
  // 等待文档列表加载
  await page.waitForSelector(`[data-testid="bundle-doc-label-${TEST_DOC_ID}"]`);
  // 选择测试文档
  await page.locator(`[data-testid="bundle-doc-checkbox-${TEST_DOC_ID}"]`).click();
  // 进入 Security 步骤
  await page.locator('[data-testid="pipeline-nav-forward"]').click();
  await expect(page.getByText("Security Options")).toBeVisible();
}

async function setSwitch(page: Page, testId: string, targetChecked: boolean) {
  const locator = page.locator(`[data-testid="${testId}"]`).first();
  await locator.waitFor({ state: "visible" });
  const current = await locator.getAttribute("data-checked");
  const isChecked = current === "" || current === "true";
  const isDisabled = await locator.isDisabled();
  if (isChecked === targetChecked) return;
  if (isDisabled) {
    throw new Error(
      `Switch ${testId} is disabled but needs to change from ${isChecked} to ${targetChecked}`
    );
  }
  await locator.click();
}

async function getSwitchState(page: Page, testId: string): Promise<boolean> {
  const locator = page.locator(`[data-testid="${testId}"]`);
  const dataChecked = await locator.getAttribute("data-checked");
  return dataChecked === "" || dataChecked === "true";
}

async function setWhitelist(page: Page, value: string) {
  const input = page.locator('[data-testid="security-whitelist-input"]');
  await input.fill(value);
}

async function setPassword(page: Page, value: string) {
  const input = page.locator('[data-testid="security-password-input"]');
  await input.fill(value);
}

async function openAdvancedSettings(page: Page) {
  const toggle = page.locator('[data-testid="security-advanced-toggle"]');
  // 如果已经展开，aria-expanded 会是 true
  const expanded = await toggle.getAttribute("aria-expanded");
  if (expanded !== "true") {
    await toggle.click();
  }
  await expect(page.locator('[data-testid="security-expiry-select"]')).toBeVisible();
}

async function setSelectValue(page: Page, testId: string, value: string) {
  const trigger = page.locator(`[data-testid="${testId}"]`).first();
  await trigger.click();
  // 通过 option role + 文本来选择，兼容 Base UI Select
  const label = value === "custom"
    ? "Custom"
    : value === "unlimited"
      ? "Unlimited"
      : String(value);
  await page.getByRole("option", { name: new RegExp(label) }).first().click();
}

async function selectFirstContact(page: Page) {
  // 打开联系人 Combobox
  await page.getByText("Add contacts...").first().click();
  // 选择第一个联系人 (mockContacts[0])
  await page.locator(`[data-testid="contact-option-${TEST_CONTACT_ID}"]`).click();
}

async function gotoReviewStep(page: Page) {
  await page.locator('[data-testid="pipeline-nav-forward"]').click();
  await expect(page.locator('[data-testid="review-submit-button"]')).toBeVisible();
}

// ----------------------------------------------------------------------------
// 测试主体
// ----------------------------------------------------------------------------

test.describe("安全选项 100% 排列组合验收", () => {
  test.beforeEach(async ({ page }) => {
    attachDebug(page);
    await setupAuthenticatedPage(page);
  });

  test("64 种布尔开关笛卡尔积 — UI 状态与约束验收", async ({ page }) => {
    test.setTimeout(120000);
    await gotoSecurityStep(page);
    const combos = generateBoolCombinations();

    for (let idx = 0; idx < combos.length; idx++) {
      const combo = combos[idx];

      // 先重置所有开关为关闭 (注意顺序: 先关 whitelist/NDA, 再关 email)
      await setSwitch(page, "security-switch-whitelistEnabled", false);
      await setSwitch(page, "security-switch-ndaEnabled", false);
      await setSwitch(page, "security-switch-requireEmailVerification", false);
      await setSwitch(page, "security-switch-passwordEnabled", false);
      await setSwitch(page, "security-switch-allowDownload", false);
      await setSwitch(page, "security-switch-watermarkEnabled", false);

      // 期望的 email verification 状态 (受 whitelist/NDA 强制约束)
      const expectedEmailVerification =
        combo.requireEmailVerification || combo.whitelistEnabled || combo.ndaEnabled;

      // 先设置不会触发 disabled 的开关
      await setSwitch(page, "security-switch-passwordEnabled", combo.passwordEnabled);
      await setSwitch(page, "security-switch-allowDownload", combo.allowDownload);
      await setSwitch(page, "security-switch-watermarkEnabled", combo.watermarkEnabled);

      // 设置 whitelist/NDA (这会强制开启并 disable email verification)
      await setSwitch(page, "security-switch-whitelistEnabled", combo.whitelistEnabled);
      await setSwitch(page, "security-switch-ndaEnabled", combo.ndaEnabled);

      // 最后设置 email verification, 仅在未被强制开启时
      if (!combo.whitelistEnabled && !combo.ndaEnabled) {
        await setSwitch(page, "security-switch-requireEmailVerification", combo.requireEmailVerification);
      }

      // 填充依赖字段，避免 guard 拦截
      if (combo.whitelistEnabled) {
        await setWhitelist(page, "vip@example.com");
      }
      if (combo.passwordEnabled) {
        await setPassword(page, "TestPass123!");
      }
      if (expectedEmailVerification) {
        await selectFirstContact(page);
      }

      // 断言 UI 状态
      await expect(
        getSwitchState(page, "security-switch-requireEmailVerification"),
      ).resolves.toBe(expectedEmailVerification);
      await expect(
        getSwitchState(page, "security-switch-whitelistEnabled"),
      ).resolves.toBe(combo.whitelistEnabled);
      await expect(
        getSwitchState(page, "security-switch-passwordEnabled"),
      ).resolves.toBe(combo.passwordEnabled);
      await expect(
        getSwitchState(page, "security-switch-ndaEnabled"),
      ).resolves.toBe(combo.ndaEnabled);
      await expect(
        getSwitchState(page, "security-switch-allowDownload"),
      ).resolves.toBe(combo.allowDownload);
      await expect(
        getSwitchState(page, "security-switch-watermarkEnabled"),
      ).resolves.toBe(combo.watermarkEnabled);

      // 断言邮箱验证开关在 whitelist/NDA 开启时被禁用
      const emailSwitch = page.locator('[data-testid="security-switch-requireEmailVerification"]');
      const emailDisabled = await emailSwitch.isDisabled();
      expect(emailDisabled).toBe(combo.whitelistEnabled || combo.ndaEnabled);
    }
  });

  test("有效期 × 最大访问次数 16 种组合 — 高级设置验收", async ({ page }) => {
    await gotoSecurityStep(page);
    await openAdvancedSettings(page);

    for (const expiry of EXPIRY_VALUES) {
      for (const maxViews of MAX_VIEWS_VALUES) {
        await setSelectValue(page, "security-expiry-select", String(expiry));
        await setSelectValue(page, "security-max-views-select", String(maxViews));

        // 验证 trigger 上显示的值
        const expiryTrigger = page.locator('[data-testid="security-expiry-select"]').first();
        const maxViewsTrigger = page.locator('[data-testid="security-max-views-select"]').first();

        const expiryText = await expiryTrigger.textContent();
        const maxViewsText = await maxViewsTrigger.textContent();

        if (expiry === "custom") {
          expect(expiryText?.toLowerCase()).toContain("custom");
        } else {
          expect(expiryText).toContain(String(expiry));
        }

        if (maxViews === "unlimited") {
          expect(maxViewsText?.toLowerCase()).toContain("unlimited");
        } else {
          expect(maxViewsText).toContain(String(maxViews));
        }
      }
    }
  });

  test("关键组合完整链路 — UI → Review → API Payload", async ({ page }) => {
    await gotoSecurityStep(page);

    // 选择最高安全组合
    await setSwitch(page, "security-switch-whitelistEnabled", true);
    await setWhitelist(page, "ceo@corp.com, @partner.io");
    await setSwitch(page, "security-switch-passwordEnabled", true);
    await setPassword(page, "UltraSecure123!");
    await setSwitch(page, "security-switch-ndaEnabled", true);
    await setSwitch(page, "security-switch-allowDownload", false);
    await setSwitch(page, "security-switch-watermarkEnabled", true);

    // whitelist/NDA 会强制开启邮箱验证，需要选择联系人
    await selectFirstContact(page);

    // 设置高级选项
    await openAdvancedSettings(page);
    await setSelectValue(page, "security-expiry-select", "7");
    await setSelectValue(page, "security-max-views-select", "10");

    // 进入 Review 验证功能标签
    await gotoReviewStep(page);
    await expect(page.getByText("Email verification code")).toBeVisible();
    await expect(page.getByText("Whitelist")).toBeVisible();
    await expect(page.getByText("Access password")).toBeVisible();
    await expect(page.getByText("NDA signing")).toBeVisible();
    await expect(page.getByText("Dynamic watermark")).toBeVisible();
    await expect(page.getByText("Download disabled")).toBeVisible();

    // 拦截创建请求并验证 payload
    const createPromise = page.waitForRequest((req) =>
      req.url().includes(`/api/workspaces/${WORKSPACE_SLUG}/links`) && req.method() === "POST"
    );

    await page.locator('[data-testid="review-submit-button"]').click();

    const request = await createPromise;
    const payload = await request.postDataJSON();

    expect(payload.require_email_verification).toBe(true);
    expect(payload.require_password).toBe(true);
    expect(payload.require_nda).toBe(true);
    expect(payload.allowed_emails).toEqual(["ceo@corp.com"]);
    expect(payload.allowed_domains).toEqual(["@partner.io"]);
    expect(payload.password).toBe("UltraSecure123!");
    expect(payload.download_enabled).toBe(false);
    expect(payload.watermark_enabled).toBe(true);
    expect(payload.max_access_count).toBe(10);
    expect(payload.contact_ids).toContain(TEST_CONTACT_ID);
    expect(payload.permission_type).toBe("password");
    expect(payload.document_ids).toContain(TEST_DOC_ID);

    // 验证创建成功
    await expect(page.locator('[data-testid="review-success-card"]')).toBeVisible();
  });
});
