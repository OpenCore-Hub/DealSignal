// @vitest-environment jsdom
import { describe, it, expect } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { CreateLinkSheet } from "./CreateLinkSheet";
import { Button } from "@/components/ui/button";
import enDealRooms from "@/i18n/locales/en/dealRooms.json";

const i18nInstance = i18n.createInstance();
i18nInstance.use(initReactI18next).init({
  lng: "en",
  resources: {
    en: {
      dealRooms: enDealRooms,
      common: { close: "Close" },
    },
  },
  interpolation: { escapeValue: false },
});

function Wrapper({ children }: { children: React.ReactNode }) {
  return <I18nextProvider i18n={i18nInstance}>{children}</I18nextProvider>;
}

describe("CreateLinkSheet", () => {
  it("renders collapsible section titles after open", () => {
    render(
      <Wrapper>
        <CreateLinkSheet dealRoomId="room-123">
          <Button>Open</Button>
        </CreateLinkSheet>
      </Wrapper>
    );

    fireEvent.click(screen.getByText("Open"));

    expect(screen.getByText("Security Controls")).toBeInTheDocument();
    expect(screen.getByText("Advanced Controls")).toBeInTheDocument();
  });
});
