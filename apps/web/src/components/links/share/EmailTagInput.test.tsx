// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { EmailTagInput } from "./EmailTagInput";

const i18nInstance = i18n.createInstance();
i18nInstance.use(initReactI18next).init({
  lng: "en",
  resources: {
    en: {
      linkShare: {
        emailTagInput: {
          invalid: "invalid email or domain",
          remove: "Remove {{value}}",
        },
      },
    },
  },
  interpolation: { escapeValue: false },
});

function Wrapper({ children }: { children: React.ReactNode }) {
  return <I18nextProvider i18n={i18nInstance}>{children}</I18nextProvider>;
}

describe("EmailTagInput", () => {
  it("adds valid emails as chips", () => {
    const onChange = vi.fn();
    render(
      <Wrapper>
        <EmailTagInput values={[]} onChange={onChange} />
      </Wrapper>
    );

    const input = screen.getByRole("textbox");
    fireEvent.change(input, { target: { value: "alice@vc.com, bob@vc.com" } });
    fireEvent.keyDown(input, { key: "Enter" });

    expect(onChange).toHaveBeenCalledWith(["alice@vc.com", "bob@vc.com"]);
  });

  it("supports domain wildcards", () => {
    const onChange = vi.fn();
    render(
      <Wrapper>
        <EmailTagInput values={[]} onChange={onChange} />
      </Wrapper>
    );

    const input = screen.getByRole("textbox");
    fireEvent.change(input, { target: { value: "*@vc.com" } });
    fireEvent.keyDown(input, { key: "Enter" });

    expect(onChange).toHaveBeenCalledWith(["*@vc.com"]);
  });

  it("does not add invalid emails", () => {
    const onChange = vi.fn();
    render(
      <Wrapper>
        <EmailTagInput values={[]} onChange={onChange} />
      </Wrapper>
    );

    const input = screen.getByRole("textbox");
    fireEvent.change(input, { target: { value: "not-an-email" } });
    fireEvent.keyDown(input, { key: "Enter" });

    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByText("not-an-email — invalid email or domain")).toBeInTheDocument();
  });

  it("removes a chip when clicking its remove button", () => {
    const onChange = vi.fn();
    render(
      <Wrapper>
        <EmailTagInput values={["alice@vc.com"]} onChange={onChange} />
      </Wrapper>
    );

    fireEvent.click(screen.getByLabelText("Remove alice@vc.com"));

    expect(onChange).toHaveBeenCalledWith([]);
  });

  it("does not call onChange when adding a duplicate", () => {
    const onChange = vi.fn();
    render(
      <Wrapper>
        <EmailTagInput values={["alice@vc.com"]} onChange={onChange} />
      </Wrapper>
    );

    const input = screen.getByRole("textbox");
    fireEvent.change(input, { target: { value: "alice@vc.com" } });
    fireEvent.keyDown(input, { key: "Enter" });

    expect(onChange).not.toHaveBeenCalled();
  });
});
