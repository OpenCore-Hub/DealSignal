import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { defaultLocale, getTranslation, isSupportedLocale, resolveBrowserLocale } from './core.js';
import { I18nContext } from './context.js';
import type { Locale, TranslationKey } from './types.js';

const LOCALE_STORAGE_KEY = 'dealsignal.locale';

function getInitialLocale(): Locale {
  if (typeof window === 'undefined') return defaultLocale;

  const stored = window.localStorage.getItem(LOCALE_STORAGE_KEY);
  if (isSupportedLocale(stored)) return stored;

  return resolveBrowserLocale(window.navigator.language);
}

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>(getInitialLocale);

  const setLocale = useCallback((nextLocale: Locale) => {
    setLocaleState(nextLocale);
    window.localStorage.setItem(LOCALE_STORAGE_KEY, nextLocale);
    document.documentElement.lang = nextLocale;
  }, []);

  useEffect(() => {
    document.documentElement.lang = locale;
  }, [locale]);

  const t = useCallback((key: TranslationKey) => getTranslation(locale, key), [locale]);

  const value = useMemo(() => ({ locale, setLocale, t }), [locale, setLocale, t]);

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}
