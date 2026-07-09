import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import resourcesToBackend from "i18next-resources-to-backend";
import { customLanguageDetector } from "./detectors";

const supportedLngs = ["en", "zh-CN"] as const;
export type SupportedLanguage = (typeof supportedLngs)[number];

export const isSupportedLanguage = (lng: string): lng is SupportedLanguage =>
  supportedLngs.includes(lng as SupportedLanguage);

i18n
  .use(customLanguageDetector)
  .use(initReactI18next)
  .use(resourcesToBackend((language: string, namespace: string) => import(`./locales/${language}/${namespace}.json`)))
  .init({
    fallbackLng: "en",
    supportedLngs: [...supportedLngs],
    ns: ["common", "layout", "dashboard", "documents", "agreementDocuments", "links", "contacts", "insights", "settings", "dealRooms", "linkShare", "ai", "auth", "formatters"],
    defaultNS: "common",
    interpolation: {
      escapeValue: false,
    },
    detection: {
      order: ["customLanguageDetector"],
      caches: ["localStorage"],
      lookupLocalStorage: "i18nextLng",
    },
  });

export default i18n;
