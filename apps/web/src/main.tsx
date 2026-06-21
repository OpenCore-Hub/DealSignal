import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import App from "@/App";
import "@/index.css";

async function prepare() {
  // MSW 用于无后端演示；拦截所有 /api 请求并返回 Mock 数据
  const { worker } = await import("@/lib/mocks/browser");
  return worker.start({
    onUnhandledRequest: "bypass",
  });
}

prepare().then(() => {
  createRoot(document.getElementById("root")!).render(
    <StrictMode>
      <App />
    </StrictMode>
  );
});
