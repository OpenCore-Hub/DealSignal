import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import App from "@/App";
import "@/index.css";

async function prepare() {
  // 当未配置真实后端地址时，在开发环境启用 MSW
  if (import.meta.env.DEV && !import.meta.env.VITE_API_BASE_URL) {
    const { worker } = await import("@/lib/mocks/browser");
    return worker.start({
      onUnhandledRequest: "bypass",
    });
  }
}

prepare().then(() => {
  createRoot(document.getElementById("root")!).render(
    <StrictMode>
      <App />
    </StrictMode>
  );
});
