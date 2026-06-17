export type { Locale, TranslationKey, TranslationTree } from './types.js';
export {
  defaultLocale,
  getTranslation,
  isSupportedLocale,
  resolveBrowserLocale,
  translations,
} from './core.js';
export { I18nProvider } from './I18nProvider.js';
export { useI18n } from './useI18n.js';
