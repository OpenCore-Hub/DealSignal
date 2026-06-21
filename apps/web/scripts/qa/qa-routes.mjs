import { chromium } from 'playwright';
import fs from 'fs';
import path from 'path';

const BASE = process.env.QA_BASE_URL || 'http://localhost:5173';
const WS = process.env.QA_WORKSPACE || 'acme-capital';
const OUT = process.env.QA_OUTPUT_DIR || path.resolve(import.meta.dirname, '../../qa-output');

if (!fs.existsSync(OUT)) fs.mkdirSync(OUT, { recursive: true });

const routes = [
  { name: 'workspaces', url: `${BASE}/` },
  { name: 'dashboard', url: `${BASE}/${WS}/dashboard` },
  { name: 'documents', url: `${BASE}/${WS}/documents` },
  { name: 'document-detail', url: `${BASE}/${WS}/documents/doc_1` },
  { name: 'document-upload', url: `${BASE}/${WS}/documents/upload` },
  { name: 'links', url: `${BASE}/${WS}/links` },
  { name: 'link-detail', url: `${BASE}/${WS}/links/link_1` },
  { name: 'deal-rooms', url: `${BASE}/${WS}/deal-rooms` },
  { name: 'deal-room-new', url: `${BASE}/${WS}/deal-rooms/new` },
  { name: 'deal-room-detail', url: `${BASE}/${WS}/deal-rooms/room_1` },
  { name: 'contacts', url: `${BASE}/${WS}/contacts` },
  { name: 'contact-detail', url: `${BASE}/${WS}/contacts/contact_1` },
  { name: 'insights-overview', url: `${BASE}/${WS}/insights` },
  { name: 'insights-pages', url: `${BASE}/${WS}/insights/pages` },
  { name: 'insights-suggestions', url: `${BASE}/${WS}/insights/suggestions` },
  { name: 'settings-general', url: `${BASE}/${WS}/settings` },
  { name: 'settings-language', url: `${BASE}/${WS}/settings/language` },
  { name: 'settings-brand', url: `${BASE}/${WS}/settings/brand` },
  { name: 'settings-billing', url: `${BASE}/${WS}/settings/billing` },
  { name: 'settings-security', url: `${BASE}/${WS}/settings/security` },
  { name: 'settings-members', url: `${BASE}/${WS}/settings/members` },
  { name: 'settings-integrations', url: `${BASE}/${WS}/settings/integrations` },
];

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

(async () => {
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  const page = await context.newPage();

  const results = [];

  for (const route of routes) {
    const errors = [];
    const pageErrorHandler = (err) => errors.push({ type: 'pageerror', message: err.message, stack: err.stack?.split('\n').slice(0, 3).join('\n') });
    const consoleHandler = (msg) => {
      const text = msg.text();
      if (msg.type() === 'error' && !shouldIgnore(text)) {
        errors.push({ type: 'console.error', text: text.slice(0, 500) });
      }
    };
    page.on('pageerror', pageErrorHandler);
    page.on('console', consoleHandler);

    try {
      await page.goto(route.url, { waitUntil: 'networkidle', timeout: 15000 });
      await page.waitForTimeout(1200);
      const screenshotPath = path.join(OUT, `${route.name}.png`);
      await page.screenshot({ path: screenshotPath, fullPage: false });
      results.push({ ...route, status: 'ok', errors, screenshot: screenshotPath });
    } catch (e) {
      results.push({ ...route, status: 'failed', error: e.message, errors, screenshot: null });
    } finally {
      page.off('pageerror', pageErrorHandler);
      page.off('console', consoleHandler);
    }
  }

  // Test AI assistant open on dashboard
  {
    const aiErrors = [];
    const pageErrorHandler = (err) => aiErrors.push({ type: 'pageerror', message: err.message });
    const consoleHandler = (msg) => {
      const text = msg.text();
      if (msg.type() === 'error' && !shouldIgnore(text)) aiErrors.push({ type: 'console.error', text: text.slice(0, 500) });
    };
    page.on('pageerror', pageErrorHandler);
    page.on('console', consoleHandler);
    try {
      await page.goto(`${BASE}/${WS}/dashboard`, { waitUntil: 'networkidle' });
      await page.waitForTimeout(1000);
      const aiBtn = page.locator('button[aria-label*="Open AI assistant"]').first();
      if (await aiBtn.isVisible().catch(() => false)) {
        await aiBtn.click();
        await page.waitForTimeout(800);
        await page.screenshot({ path: path.join(OUT, 'ai-assistant-open.png'), fullPage: false });
      }
      results.push({ name: 'ai-assistant-open', url: `${BASE}/${WS}/dashboard`, status: 'ok', errors: aiErrors, screenshot: path.join(OUT, 'ai-assistant-open.png') });
    } catch (e) {
      results.push({ name: 'ai-assistant-open', url: `${BASE}/${WS}/dashboard`, status: 'failed', error: e.message, errors: aiErrors, screenshot: null });
    } finally {
      page.off('pageerror', pageErrorHandler);
      page.off('console', consoleHandler);
    }
  }

  await browser.close();

  const reportPath = path.join(OUT, 'report.json');
  fs.writeFileSync(reportPath, JSON.stringify(results, null, 2));

  // Print summary
  const failed = results.filter((r) => r.status === 'failed' || r.errors.length > 0);
  console.log(`\n=== QA Summary ===`);
  console.log(`Total routes: ${results.length}`);
  console.log(`Clean: ${results.filter((r) => r.status === 'ok' && r.errors.length === 0).length}`);
  console.log(`With errors: ${failed.length}`);
  for (const r of failed) {
    console.log(`\n[${r.name}] ${r.status === 'failed' ? 'FAILED' : 'errors'}`);
    if (r.error) console.log('  Navigation error:', r.error);
    for (const err of r.errors) {
      console.log('  -', err.type + ':', (err.message || err.text || '').replace(/\n/g, ' '));
    }
  }
  console.log(`\nScreenshots saved to: ${OUT}`);
  console.log(`Report saved to: ${reportPath}`);
  if (failed.length > 0) process.exit(1);
})();
