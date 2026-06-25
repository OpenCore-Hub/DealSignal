import path from "path";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

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

// https://vite.dev/config/
export default defineConfig({
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
});
