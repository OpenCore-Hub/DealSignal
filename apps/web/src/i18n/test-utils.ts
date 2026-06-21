import i18n from "i18next";
import { initReactI18next } from "react-i18next";

export async function createTestI18n(resources: Record<string, Record<string, string>> = {}) {
  const instance = i18n.createInstance();
  await instance.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    ns: ["common", "layout"],
    defaultNS: "common",
    resources: {
      en: {
        common: resources.common ?? {},
        layout: resources.layout ?? {},
      },
    },
    interpolation: { escapeValue: false },
  });
  return instance;
}
