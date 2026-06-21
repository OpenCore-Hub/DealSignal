import i18n, { isSupportedLanguage, type SupportedLanguage } from "./config";

export function getCurrentLanguage(): SupportedLanguage {
  const lng = i18n.language;
  return isSupportedLanguage(lng) ? lng : "en";
}

export function setLanguage(lng: SupportedLanguage) {
  void i18n.changeLanguage(lng);
}

export function toggleLanguage() {
  const next = getCurrentLanguage() === "zh-CN" ? "en" : "zh-CN";
  setLanguage(next);
}

export function updateDocumentLanguage(lng: string) {
  document.documentElement.lang = isSupportedLanguage(lng) ? lng : "zh-CN";
}

export function updatePageTitle(title?: string) {
  document.title = title || "DealSignal";
}
