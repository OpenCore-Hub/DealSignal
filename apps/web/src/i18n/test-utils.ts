import i18n from "i18next";
import { initReactI18next } from "react-i18next";

export async function createTestI18n(resources: Record<string, Record<string, string>> = {}) {
  const instance = i18n.createInstance();
  const namespaces = Array.from(new Set(["common", "layout", ...Object.keys(resources)]));
  const enResources: Record<string, Record<string, string>> = {};
  namespaces.forEach((ns) => {
    enResources[ns] = resources[ns] ?? {};
  });
  await instance.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    ns: namespaces,
    defaultNS: "common",
    resources: { en: enResources },
    interpolation: { escapeValue: false },
  });
  return instance;
}
