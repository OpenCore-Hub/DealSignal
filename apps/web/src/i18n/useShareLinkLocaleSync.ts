import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { detectBrowserLanguage, isPublicShareLinkPath } from "./detectors";

/**
 * Re-sync locale when entering a share link inside an existing SPA session
 * (e.g. member app had English in localStorage but visitor browser is Chinese).
 */
export function useShareLinkLocaleSync() {
  const { i18n } = useTranslation();

  useEffect(() => {
    if (!isPublicShareLinkPath(window.location.pathname)) return;

    const queryLng = new URLSearchParams(window.location.search).get("lng");
    if (queryLng) return;

    const browserLng = detectBrowserLanguage();
    if (i18n.language !== browserLng) {
      void i18n.changeLanguage(browserLng);
    }
  }, [i18n]);
}
