#!/usr/bin/env node
import { readFileSync, readdirSync, statSync } from 'node:fs';
import { dirname, extname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const srcDir = resolve(root, 'src');
const allowedText = new Set(['DealSignal']);
const ignoredDirs = new Set(['i18n']);

function walk(dir) {
  const files = [];
  for (const entry of readdirSync(dir)) {
    if (ignoredDirs.has(entry)) continue;
    const path = join(dir, entry);
    const stat = statSync(path);
    if (stat.isDirectory()) files.push(...walk(path));
    else if (['.tsx', '.jsx'].includes(extname(path))) files.push(path);
  }
  return files;
}

function lineNumberAt(content, index) {
  return content.slice(0, index).split('\n').length;
}

const violations = [];
const textBetweenTags = />\s*([^<>{}\n][^<>{}]*)\s*</g;
const ariaLabel = /aria-label=["']([^"']+)["']/g;
const titleAttr = /title=["']([^"']+)["']/g;

for (const file of walk(srcDir)) {
  const content = readFileSync(file, 'utf8');
  if (content.includes('i18n-exempt-file')) continue;

  for (const pattern of [textBetweenTags, ariaLabel, titleAttr]) {
    pattern.lastIndex = 0;
    let match;
    while ((match = pattern.exec(content))) {
      const text = match[1].trim();
      if (!text || allowedText.has(text)) continue;
      if (/^[\s\d.,:;!?()\-_/|]+$/.test(text)) continue;
      if (text.startsWith('{') || text.includes('=>')) continue;
      const previousLineStart = content.lastIndexOf('\n', match.index) + 1;
      const previousLine = content.slice(previousLineStart, match.index);
      if (previousLine.includes('i18n-exempt')) continue;
      violations.push(`${file.replace(root + '/', '')}:${lineNumberAt(content, match.index)} "${text}"`);
    }
  }
}

if (violations.length) {
  console.error('Hardcoded user-facing UI strings found. Use t(\'translation.key\') instead:');
  for (const violation of violations) console.error(`  ${violation}`);
  process.exit(1);
}

console.log('No hardcoded user-facing UI strings found.');
