import { useEffect } from "react";
import { useUIStore } from "@/stores/uiStore";

function applyTheme(theme: "light" | "dark" | "system") {
  const root = document.documentElement;
  const systemDark = window.matchMedia("(prefers-color-scheme: dark)").matches;
  const isDark = theme === "dark" || (theme === "system" && systemDark);
  root.classList.toggle("dark", isDark);
}

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const { theme } = useUIStore();

  useEffect(() => {
    applyTheme(theme);

    if (theme !== "system") return;
    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const handler = () => applyTheme(theme);
    media.addEventListener("change", handler);
    return () => media.removeEventListener("change", handler);
  }, [theme]);

  return <>{children}</>;
}
