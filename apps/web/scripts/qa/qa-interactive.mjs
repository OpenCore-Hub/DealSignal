import { chromium } from 'playwright';
import fs from 'fs';
import path from 'path';

const BASE = process.env.QA_BASE_URL || 'http://localhost:5173';
const WS = process.env.QA_WORKSPACE || 'acme-capital';
const OUT = process.env.QA_OUTPUT_DIR || path.resolve(import.meta.dirname, '../../qa-output');

if (!fs.existsSync(OUT)) fs.mkdirSync(OUT, { recursive: true });

const IGNORED_CONSOLE = [
  '[vite] connecting',
  '[vite] connected',
  'React DevTools',
  '[MSW]',
  'Documentation:',
  'Found an issue?',
  'Worker script URL',
  'Worker scope',
  'Client ID',
];

function shouldIgnore(text) {
  return IGNORED_CONSOLE.some((s) => text.includes(s));
}

async function run() {
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  const page = await context.newPage();
  const results = [];

  function attach() {
    const errors = [];
    const pe = (err) => errors.push({ type: 'pageerror', message: err.message });
    const ce = (msg) => {
      const text = msg.text();
      if (msg.type() === 'error' && !shouldIgnore(text)) errors.push({ type: 'console.error', text: text.slice(0, 500) });
    };
    page.on('pageerror', pe);
    page.on('console', ce);
    return { errors, detach: () => { page.off('pageerror', pe); page.off('console', ce); } };
  }

  // 1. Workspace switcher click
  {
    const { errors, detach } = attach();
    await page.goto(`${BASE}/${WS}/dashboard`, { waitUntil: 'networkidle' });
    await page.waitForTimeout(800);
    const switcher = page.locator('button:has-text("Acme")').first();
    if (await switcher.isVisible().catch(() => false)) {
      await switcher.click();
      await page.waitForTimeout(500);
      await page.screenshot({ path: path.join(OUT, 'interactive-workspace-switcher.png') });
    }
    results.push({ name: 'workspace-switcher-click', errors: [...errors] });
    detach();
  }

  // 2. Theme toggle click
  {
    const { errors, detach } = attach();
    const themeBtn = page.locator('button[aria-label*="Toggle theme"]').first();
    if (await themeBtn.isVisible().catch(() => false)) {
      await themeBtn.click();
      await page.waitForTimeout(500);
      await page.screenshot({ path: path.join(OUT, 'interactive-theme-menu.png') });
    }
    results.push({ name: 'theme-toggle-click', errors: [...errors] });
    detach();
  }

  // 3. AI assistant open
  {
    const { errors, detach } = attach();
    // Close theme menu first by pressing Escape
    await page.keyboard.press('Escape');
    await page.waitForTimeout(200);
    const aiBtn = page.locator('button[aria-label="Open AI assistant"]').first();
    if (await aiBtn.isVisible().catch(() => false)) {
      await aiBtn.click();
      await page.waitForTimeout(800);
      await page.screenshot({ path: path.join(OUT, 'interactive-ai-open.png') });
    }
    results.push({ name: 'ai-assistant-open', errors: [...errors] });
    detach();
  }

  // 4. Send a suggestion in AI assistant
  {
    const { errors, detach } = attach();
    const suggestion = page.locator('button:has-text("high-heat signals")').first();
    if (await suggestion.isVisible().catch(() => false)) {
      await suggestion.click();
      await page.waitForTimeout(1500);
      await page.screenshot({ path: path.join(OUT, 'interactive-ai-reply.png') });
    }
    results.push({ name: 'ai-assistant-suggestion', errors: [...errors] });
    detach();
  }

  // 5. Click Create Link on links page
  {
    const { errors, detach } = attach();
    await page.goto(`${BASE}/${WS}/links`, { waitUntil: 'networkidle' });
    await page.waitForTimeout(800);
    const createLink = page.locator('a[href*="/links/new"], button:has-text("Create Link")').first();
    if (await createLink.isVisible().catch(() => false)) {
      await createLink.click();
      await page.waitForTimeout(800);
      await page.screenshot({ path: path.join(OUT, 'interactive-create-link.png') });
    }
    results.push({ name: 'create-link-click', errors: [...errors] });
    detach();
  }

  // 6. Mobile viewport dashboard
  {
    const { errors, detach } = attach();
    await page.setViewportSize({ width: 375, height: 812 });
    await page.goto(`${BASE}/${WS}/dashboard`, { waitUntil: 'networkidle' });
    await page.waitForTimeout(800);
    await page.screenshot({ path: path.join(OUT, 'mobile-dashboard.png') });
    results.push({ name: 'mobile-dashboard', errors: [...errors] });
    detach();
  }

  await browser.close();

  const failed = results.filter((r) => r.errors.length > 0);
  console.log(`\n=== Interactive QA Summary ===`);
  console.log(`Total interactions: ${results.length}`);
  console.log(`Clean: ${results.length - failed.length}`);
  console.log(`With errors: ${failed.length}`);
  for (const r of failed) {
    console.log(`\n[${r.name}]`);
    for (const err of r.errors) console.log('  -', err.type + ':', err.message || err.text);
  }
}

run().catch((e) => { console.error(e); process.exit(1); });
