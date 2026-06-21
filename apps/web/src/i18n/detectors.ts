const normalizeLanguage = (lng: string) => {
  if (lng.startsWith("zh")) return "zh-CN";
  if (lng.startsWith("en")) return "en";
  return "en";
};

export const customLanguageDetector = {
  type: "languageDetector" as const,
  name: "customLanguageDetector",

  async: false,

  init() {},

  detect() {
    const urlParams = new URLSearchParams(window.location.search);
    const queryLng = urlParams.get("lng");
    if (queryLng) return normalizeLanguage(queryLng);

    const stored = localStorage.getItem("i18nextLng");
    if (stored) return normalizeLanguage(stored);

    if (typeof navigator !== "undefined") {
      const navLng = navigator.language;
      if (navLng) return normalizeLanguage(navLng);
    }

    const htmlLang = document.documentElement.lang;
    if (htmlLang) return normalizeLanguage(htmlLang);

    return "en";
  },

  cacheUserLanguage(lng: string) {
    localStorage.setItem("i18nextLng", lng);
  },
};
