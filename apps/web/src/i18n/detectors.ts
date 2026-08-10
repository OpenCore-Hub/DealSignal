import type { SupportedLanguage } from "./config";

export const normalizeLanguage = (lng: string): SupportedLanguage => {
  if (lng.startsWith("zh")) return "zh-CN";
  if (lng.startsWith("en")) return "en";
  return "en";
};

/** Public share-link viewer routes (`/l/:token`). */
export function isPublicShareLinkPath(pathname: string): boolean {
  return pathname.startsWith("/l/");
}

/** First Chinese preference in browser languages, otherwise English. */
export function detectBrowserLanguage(): SupportedLanguage {
  if (typeof navigator === "undefined") return "en";

  const candidates = [...(navigator.languages ?? []), navigator.language].filter(Boolean);
  for (const lng of candidates) {
    if (lng.startsWith("zh")) return "zh-CN";
  }
  return "en";
}

export function resolveInitialLanguage(pathname: string, search: string): SupportedLanguage {
  const urlParams = new URLSearchParams(search);
  const queryLng = urlParams.get("lng");
  if (queryLng) return normalizeLanguage(queryLng);

  if (isPublicShareLinkPath(pathname)) {
    return detectBrowserLanguage();
  }

  if (typeof localStorage !== "undefined") {
    const stored = localStorage.getItem("i18nextLng");
    if (stored) return normalizeLanguage(stored);
  }

  if (typeof navigator !== "undefined") {
    const navLng = navigator.language;
    if (navLng) return normalizeLanguage(navLng);
  }

  if (typeof document !== "undefined") {
    const htmlLang = document.documentElement.lang;
    if (htmlLang) return normalizeLanguage(htmlLang);
  }

  return "en";
}

export const customLanguageDetector = {
  type: "languageDetector" as const,
  name: "customLanguageDetector",

  async: false,

  init() {},

  detect() {
    if (typeof window === "undefined") return "en";
    return resolveInitialLanguage(window.location.pathname, window.location.search);
  },

  cacheUserLanguage(lng: string) {
    if (typeof window === "undefined") return;
    // Do not persist visitor browser locale into the member preference key.
    if (isPublicShareLinkPath(window.location.pathname)) return;
    localStorage.setItem("i18nextLng", lng);
  },
};
