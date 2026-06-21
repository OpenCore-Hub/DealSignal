import { useEffect } from "react";
import { RouterProvider } from "react-router";
import { useTranslation } from "react-i18next";
import { router } from "@/router";
import { ThemeProvider } from "@/components/providers/ThemeProvider";
import { Toaster } from "@/components/ui/sonner";
import { I18nProvider } from "@/i18n/provider";
import { updateDocumentLanguage, updatePageTitle } from "@/i18n/utils";
import "@/i18n/config";

function AppContent() {
  const { i18n, t } = useTranslation("layout");

  useEffect(() => {
    const lng = i18n.language;
    updateDocumentLanguage(lng);
    updatePageTitle(t("appTitle", { defaultValue: "DealSignal" }));

    const handler = () => {
      updateDocumentLanguage(i18n.language);
      updatePageTitle(t("appTitle", { defaultValue: "DealSignal" }));
    };
    i18n.on("languageChanged", handler);
    return () => {
      i18n.off("languageChanged", handler);
    };
  }, [i18n, t]);

  return (
    <ThemeProvider>
      <RouterProvider router={router} />
      <Toaster position="top-right" richColors closeButton />
    </ThemeProvider>
  );
}

function App() {
  return (
    <I18nProvider>
      <AppContent />
    </I18nProvider>
  );
}

export default App;
