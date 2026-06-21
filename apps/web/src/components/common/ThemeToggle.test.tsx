// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { ThemeToggle } from "./ThemeToggle";
import { createTestI18n } from "@/i18n/test-utils";

const setThemeMock = vi.fn();

vi.mock("@/stores/uiStore", () => ({
  useUIStore: (selector?: (state: { theme: string; setTheme: (t: string) => void }) => unknown) => {
    const state = { theme: "system", setTheme: setThemeMock };
    return selector ? selector(state) : state;
  },
}));

describe("ThemeToggle", () => {
  beforeEach(() => {
    setThemeMock.mockClear();
  });

  it("renders theme toggle button", async () => {
    const i18n = await createTestI18n({
      common: {
        "theme.toggle": "Toggle theme",
        "theme.light": "Light",
        "theme.dark": "Dark",
        "theme.system": "System",
      },
    });
    render(
      <I18nextProvider i18n={i18n}>
        <ThemeToggle />
      </I18nextProvider>
    );
    expect(screen.getByRole("button", { name: /Toggle theme/i })).toBeInTheDocument();
  });

  it("switches to light theme", async () => {
    const i18n = await createTestI18n({
      common: {
        "theme.toggle": "Toggle theme",
        "theme.light": "Light",
        "theme.dark": "Dark",
        "theme.system": "System",
      },
    });
    render(
      <I18nextProvider i18n={i18n}>
        <ThemeToggle />
      </I18nextProvider>
    );
    fireEvent.click(screen.getByRole("button", { name: /Toggle theme/i }));
    await waitFor(() => {
      expect(screen.getByRole("menuitem", { name: /Light/i })).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole("menuitem", { name: /Light/i }));
    expect(setThemeMock).toHaveBeenCalledWith("light");
  });
});
