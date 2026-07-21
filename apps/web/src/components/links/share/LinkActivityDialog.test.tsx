// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { Button } from "@/components/ui/button";
import type { Link } from "@/types";
import { LinkActivityDialog } from "./LinkActivityDialog";
import enLinkShare from "@/i18n/locales/en/linkShare.json";

const i18nInstance = i18n.createInstance();
i18nInstance.use(initReactI18next).init({
  lng: "en",
  resources: {
    en: {
      linkShare: enLinkShare,
      common: { loading: "Loading...", close: "Close" },
    },
  },
  interpolation: { escapeValue: false },
});

function Wrapper({ children }: { children: React.ReactNode }) {
  return <I18nextProvider i18n={i18nInstance}>{children}</I18nextProvider>;
}

vi.mock("./AnalyticsTab", () => ({
  AnalyticsTab: () => <div>Analytics content</div>,
}));

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const baseLink = {
  id: "link-1",
  name: "Acme",
  requireEmailVerification: true,
} as unknown as Link;

describe("LinkActivityDialog window controls", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("supports shrink, enlarge, and fullscreen", () => {
    render(
      <Wrapper>
        <LinkActivityDialog link={baseLink} open onOpenChange={() => {}}>
          <Button>Open</Button>
        </LinkActivityDialog>
      </Wrapper>,
    );

    const title = screen.getByText("Link activity");
    expect(title).toHaveClass("sr-only");
    expect(screen.getByRole("toolbar", { name: "Window size" })).toBeInTheDocument();

    const shrink = screen.getByRole("button", { name: "Shrink window" });
    const enlarge = screen.getByRole("button", { name: "Enlarge window" });
    const fullscreen = screen.getByRole("button", { name: "Enter fullscreen" });

    // Default is md: can shrink and enlarge.
    expect(shrink).not.toBeDisabled();
    expect(enlarge).not.toBeDisabled();

    fireEvent.click(shrink);
    expect(shrink).toBeDisabled();
    expect(enlarge).not.toBeDisabled();

    fireEvent.click(enlarge);
    fireEvent.click(enlarge);
    expect(enlarge).toBeDisabled();
    expect(shrink).not.toBeDisabled();

    fireEvent.click(fullscreen);
    expect(screen.getByRole("button", { name: "Exit fullscreen" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Shrink window" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Enlarge window" })).toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: "Exit fullscreen" }));
    expect(screen.getByRole("button", { name: "Enter fullscreen" })).toBeInTheDocument();
  });
});
