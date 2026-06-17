import en from './locales/en.json' with { type: 'json' };
import zhCN from './locales/zh-CN.json' with { type: 'json' };
import type { Locale, TranslationKey, TranslationTree } from './types.js';

export const defaultLocale: Locale = 'en';

export const translations: Record<Locale, TranslationTree> = {
  en,
  'zh-CN': zhCN,
};

export function isSupportedLocale(value: string | null | undefined): value is Locale {
  return value === 'en' || value === 'zh-CN';
}

export function resolveBrowserLocale(language: string | undefined): Locale {
  if (!language) return defaultLocale;
  const normalized = language.toLowerCase();
  if (normalized.startsWith('zh')) return 'zh-CN';
  return 'en';
}

export function getTranslation(locale: Locale, key: TranslationKey): string {
  const segments = key.split('.');
  let current: unknown = translations[locale];

  for (const segment of segments) {
    if (typeof current !== 'object' || current === null || !(segment in current)) {
      return key;
    }
    current = (current as Record<string, unknown>)[segment];
  }

  return typeof current === 'string' ? current : key;
}
