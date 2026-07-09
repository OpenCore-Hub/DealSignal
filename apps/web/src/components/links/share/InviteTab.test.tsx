// @vitest-environment jsdom
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { InviteTab } from "./InviteTab";
import type { LinkInvitation } from "@/types";
import enLinkShare from "@/i18n/locales/en/linkShare.json";

const i18nInstance = i18n.createInstance();
i18nInstance.use(initReactI18next).init({
  lng: "en",
  resources: {
    en: {
      linkShare: enLinkShare,
      common: { loading: "Loading..." },
    },
  },
  interpolation: { escapeValue: false },
});

function Wrapper({ children }: { children: React.ReactNode }) {
  return <I18nextProvider i18n={i18nInstance}>{children}</I18nextProvider>;
}

vi.mock("@/lib/clipboard", () => ({ copyToClipboard: vi.fn(() => Promise.resolve(true)) }));

function renderInviteTab(props: Partial<Parameters<typeof InviteTab>[0]> = {}) {
  const setEmailsRaw = vi.fn();
  const onSend = vi.fn();
  const onResend = vi.fn();
  const onRevoke = vi.fn();

  const base: Parameters<typeof InviteTab>[0] = {
    linkId: "link-1",
    publicUrl: "http://localhost/l/abc123",
    emailsRaw: "",
    setEmailsRaw,
    invalid: [],
    sending: false,
    invitations: [],
    loading: false,
    onSend,
    onResend,
    onRevoke,
  };

  const { rerender } = render(
    <Wrapper>
      <InviteTab {...base} {...props} />
    </Wrapper>
  );
  return { setEmailsRaw, onSend, onResend, onRevoke, rerender, base };
}

const mockInvitations: LinkInvitation[] = [
  {
    id: "inv-1",
    email: "alice@vc.com",
    token: "tok-1",
    status: "pending",
    createdAt: new Date().toISOString(),
  },
  {
    id: "inv-2",
    email: "bob@vc.com",
    token: "tok-2",
    status: "opened",
    createdAt: new Date().toISOString(),
  },
] as LinkInvitation[];

describe("InviteTab", () => {
  it("shows create-link placeholder when no linkId", () => {
    renderInviteTab({ linkId: undefined });
    expect(screen.getByText(/save the link to start inviting viewers/i)).toBeInTheDocument();
  });

  it("updates email input and triggers send", () => {
    const { setEmailsRaw, onSend, rerender, base } = renderInviteTab();
    fireEvent.change(screen.getByPlaceholderText(/alice@vc\.com/i), {
      target: { value: "alice@vc.com, bob@vc.com" },
    });
    expect(setEmailsRaw).toHaveBeenCalledWith("alice@vc.com, bob@vc.com");

    rerender(
      <Wrapper>
        <InviteTab {...base} emailsRaw="alice@vc.com, bob@vc.com" />
      </Wrapper>
    );
    fireEvent.click(screen.getByRole("button", { name: /send invitations/i }));
    expect(onSend).toHaveBeenCalled();
  });

  it("disables send while sending", () => {
    renderInviteTab({ sending: true, emailsRaw: "alice@vc.com" });
    expect(screen.getByRole("button", { name: /sending/i })).toBeDisabled();
  });

  it("renders invitations with status badges", () => {
    renderInviteTab({ invitations: mockInvitations });
    expect(screen.getByText("alice@vc.com")).toBeInTheDocument();
    expect(screen.getByText("bob@vc.com")).toBeInTheDocument();
    expect(screen.getByText("Pending")).toBeInTheDocument();
    expect(screen.getByText("Opened")).toBeInTheDocument();
  });

  it("triggers resend and revoke actions", () => {
    const { onResend, onRevoke } = renderInviteTab({ invitations: mockInvitations });
    const buttons = screen.getAllByRole("button", { name: /actions/i });
    expect(buttons.length).toBe(mockInvitations.length);

    fireEvent.click(buttons[0]);
    fireEvent.click(screen.getByRole("menuitem", { name: /resend/i }));
    expect(onResend).toHaveBeenCalledWith("alice@vc.com");

    fireEvent.click(buttons[0]);
    fireEvent.click(screen.getByRole("menuitem", { name: /revoke/i }));
    expect(onRevoke).toHaveBeenCalledWith(mockInvitations[0]);
  });
});
