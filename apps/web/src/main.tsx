import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import App from "@/App";
import "@/index.css";

async function prepare() {
  // MSW 仅用于开发环境无后端演示
  if (import.meta.env.DEV) {
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
