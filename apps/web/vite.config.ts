import path from "path";
import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { resolveDevApiProxyTarget } from "./src/lib/apiBaseUrl";

function getVendorChunk(id: string): string | undefined {
  if (!id.includes("node_modules")) return undefined;

  const segments = id.split("node_modules/").pop()?.split("/");
  if (!segments) return undefined;

  // Scoped packages: @scope/name
  const packageName = segments[0].startsWith("@")
    ? `${segments[0]}/${segments[1]}`
    : segments[0];

  if (["react", "react-dom", "react-router"].includes(packageName)) {
    return "vendor-react";
  }
  if (packageName === "motion") {
    return "vendor-motion";
  }
  if (packageName === "@tanstack/react-table") {
    return "vendor-table";
  }
  if (["@base-ui/react", "sonner"].includes(packageName)) {
    return "vendor-ui";
  }
  return undefined;
}

function apiDevProxy(env: Record<string, string>) {
  const target = resolveDevApiProxyTarget(
    process.env.VITE_API_BASE_URL || env.VITE_API_BASE_URL,
  );
  if (!target) return undefined;
  return {
    "/api": {
      target,
      changeOrigin: true,
      // Host-only cookies on the Vite origin (matches production nginx).
      cookieDomainRewrite: { "*": "" },
      // Knowledge SSE can exceed default proxy idle timeouts.
      timeout: 0,
      proxyTimeout: 0,
      ws: true,
    },
  };
}

// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, path.resolve(__dirname), "VITE_");
  const proxy = apiDevProxy(env);
  return {
    plugins: [react(), tailwindcss()],
    resolve: {
      alias: {
        "@": path.resolve(__dirname, "./src"),
      },
    },
    server: {
      watch: {
        ignored: ["**/coverage/**"],
      },
      proxy,
    },
    preview: {
      proxy,
    },
    // Newly added answer-markdown deps — keep prebundle fresh after installs.
    optimizeDeps: {
      include: ["react-markdown", "remark-breaks", "rehype-sanitize"],
    },
    build: {
      chunkSizeWarningLimit: 800,
      rollupOptions: {
        output: {
          manualChunks(id: string) {
            return getVendorChunk(id);
          },
        },
      },
    },
  };
});
