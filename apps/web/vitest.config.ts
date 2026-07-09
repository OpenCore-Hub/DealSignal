import path from "path";
import { defineConfig } from "vitest/config";

export default defineConfig({
  resolve: { alias: { "@": path.resolve(__dirname, "./src") } },
  test: {
    globals: true,
    environment: "node",
    include: ["src/**/*.test.ts", "src/**/*.test.tsx"],
    setupFiles: ["src/test/setup.tsx"],
    coverage: {
      enabled: true,
      provider: "v8",
      reporter: ["text", "html"],
      include: [
        "src/lib/**/*.ts",
        "src/i18n/**/*.ts",
        "src/components/ai/AIAssistant.tsx",
        "src/components/common/ThemeToggle.tsx",
        "src/components/dashboard/ActionList.tsx",
        "src/components/layout/WorkspaceSwitcher.tsx",
        "src/components/viewer/CanvasViewer.tsx",
      ],
      exclude: [
        "src/lib/mocks/**",
        "src/lib/api.ts",
        "src/lib/clipboard.ts",
        "src/i18n/config.ts",
        "src/i18n/types.ts",
        "**/*.test.*",
      ],
      thresholds: {
        statements: 70,
        branches: 60,
        functions: 50,
        lines: 70,
      },
    },
  },
});
