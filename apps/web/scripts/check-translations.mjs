#!/usr/bin/env node
import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const localeDir = resolve(root, 'src/i18n/locales');
const localeFiles = {
  en: resolve(localeDir, 'en.json'),
  'zh-CN': resolve(localeDir, 'zh-CN.json'),
};

function readJson(path) {
  return JSON.parse(readFileSync(path, 'utf8'));
}

function flatten(value, prefix = '') {
  const entries = [];
  for (const [key, child] of Object.entries(value)) {
    const path = prefix ? `${prefix}.${key}` : key;
    if (typeof child === 'string') {
      entries.push([path, child]);
    } else if (child && typeof child === 'object' && !Array.isArray(child)) {
      entries.push(...flatten(child, path));
    } else {
      throw new Error(`Invalid translation node at ${path}; expected string or object.`);
    }
  }
  return entries;
}

const locales = Object.fromEntries(
  Object.entries(localeFiles).map(([locale, path]) => [locale, Object.fromEntries(flatten(readJson(path)))])
);

const canonical = locales.en;
let failed = false;

for (const [locale, entries] of Object.entries(locales)) {
  const missing = Object.keys(canonical).filter((key) => !(key in entries));
  const extra = Object.keys(entries).filter((key) => !(key in canonical));
  const empty = Object.entries(entries).filter(([, value]) => !value.trim());
  const todos = Object.entries(entries).filter(([, value]) => /TODO|TRANSLATE_ME/i.test(value));

  if (missing.length || extra.length || empty.length || todos.length) {
    failed = true;
    console.error(`\n${locale} translation check failed:`);
    if (missing.length) console.error(`  Missing keys: ${missing.join(', ')}`);
    if (extra.length) console.error(`  Extra keys: ${extra.join(', ')}`);
    if (empty.length) console.error(`  Empty values: ${empty.map(([key]) => key).join(', ')}`);
    if (todos.length) console.error(`  Placeholder values: ${todos.map(([key]) => key).join(', ')}`);
  }
}

if (failed) {
  process.exit(1);
}

console.log(`i18n translation check passed for ${Object.keys(locales).join(', ')}.`);
