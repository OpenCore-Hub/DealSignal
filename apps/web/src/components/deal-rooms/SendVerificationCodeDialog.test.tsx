// @vitest-environment jsdom
import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { api } from "@/lib/api";
import type { AccessRule, Contact, Link } from "@/types";
import enDealRooms from "@/i18n/locales/en/dealRooms.json";
import {
  SendVerificationCodeDialog,
  buildAllowedVisitors,
} from "./SendVerificationCodeDialog";

const i18nInstance = i18n.createInstance();
i18nInstance.use(initReactI18next).init({
  lng: "en",
  resources: {
    en: {
      dealRooms: enDealRooms,
      common: {
        cancel: "Cancel",
        loading: "Loading...",
      },
    },
  },
  interpolation: { escapeValue: false },
});

function Wrapper({ children }: { children: React.ReactNode }) {
  return <I18nextProvider i18n={i18nInstance}>{children}</I18nextProvider>;
}

vi.mock("@/lib/api", () => ({
  api: {
    getLinkAccessRules: vi.fn(),
    getContactById: vi.fn(),
    getContacts: vi.fn(),
    sendEmailVerificationCode: vi.fn(),
  },
}));

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

const link: Link = {
  id: "link-1",
  shortUrl: "http://localhost:5173/l/token-abc",
  name: "测啊",
  requireEmailVerification: true,
  isActive: true,
  contactIds: ["c1"],
} as Link;

function renderDialog(open = true, linkOverride?: Link) {
  const onOpenChange = vi.fn();
  render(
    <Wrapper>
      <SendVerificationCodeDialog
        link={linkOverride ?? link}
        open={open}
        onOpenChange={onOpenChange}
      />
    </Wrapper>,
  );
  return { onOpenChange };
}

describe("buildAllowedVisitors", () => {
  it("membership comes only from allow rules; extra contacts never expand the list", () => {
    const rules: AccessRule[] = [
      { ruleType: "email", value: "alice@example.com", action: "allow" },
      { ruleType: "email", value: "bob@example.com", action: "allow" },
      { ruleType: "email", value: "leaker@bad.com", action: "block" },
      { ruleType: "email", value: "bob@example.com", action: "block" },
    ];
    const contacts: Contact[] = [
      {
        id: "c1",
        email: "alice@example.com",
        name: "Alice",
        heatLevel: "cold",
        score: 0,
        scoreHistory: [],
        totalVisits: 0,
        totalDurationSeconds: 0,
        viewedDocuments: [],
      },
      {
        id: "c2",
        email: "outsider@example.com",
        name: "Outsider",
        heatLevel: "cold",
        score: 0,
        scoreHistory: [],
        totalVisits: 0,
        totalDurationSeconds: 0,
        viewedDocuments: [],
      },
    ];

    expect(buildAllowedVisitors(rules, contacts)).toEqual([
      { email: "alice@example.com", name: "Alice" },
    ]);
  });

  it("accepts PascalCase rule fields from legacy API payloads", () => {
    const rules = [
      { RuleType: "email", Value: "yang@example.com", Action: "allow" },
    ] as unknown as AccessRule[];
    expect(buildAllowedVisitors(rules, [])).toEqual([
      { email: "yang@example.com", name: "" },
    ]);
  });
});

describe("SendVerificationCodeDialog", () => {
  beforeEach(() => {
    vi.mocked(api.getLinkAccessRules).mockReset();
    vi.mocked(api.getContactById).mockReset();
    vi.mocked(api.getContacts).mockReset();
    vi.mocked(api.sendEmailVerificationCode).mockReset();
  });

  it("lists only this link's allowed visitors and never loads workspace contacts", async () => {
    vi.mocked(api.getLinkAccessRules).mockResolvedValue({
      data: [
        { ruleType: "email", value: "alice@example.com", action: "allow" },
        { ruleType: "email", value: "bob@example.com", action: "allow" },
        { ruleType: "email", value: "leaker@bad.com", action: "block" },
      ],
    });
    vi.mocked(api.getContactById).mockResolvedValue({
      id: "c1",
      email: "alice@example.com",
      name: "杨生",
      heatLevel: "cold",
      score: 0,
      scoreHistory: [],
      totalVisits: 0,
      totalDurationSeconds: 0,
      viewedDocuments: [],
    });
    vi.mocked(api.sendEmailVerificationCode).mockResolvedValue(undefined);

    const { onOpenChange } = renderDialog();

    await waitFor(() => {
      expect(screen.getByText("杨生")).toBeInTheDocument();
    });
    expect(api.getContacts).not.toHaveBeenCalled();
    expect(api.getContactById).toHaveBeenCalledWith("c1");
    expect(api.getLinkAccessRules).toHaveBeenCalledWith("link-1");
    expect(screen.getByText("alice@example.com")).toBeInTheDocument();
    expect(screen.getByText("bob@example.com")).toBeInTheDocument();
    expect(screen.queryByText("leaker@bad.com")).not.toBeInTheDocument();

    fireEvent.click(screen.getByLabelText("Select bob@example.com"));
    fireEvent.click(screen.getByRole("button", { name: "Send code" }));

    await waitFor(() => {
      expect(api.sendEmailVerificationCode).toHaveBeenCalledTimes(1);
      expect(api.sendEmailVerificationCode).toHaveBeenCalledWith(
        "token-abc",
        "alice@example.com",
      );
      expect(onOpenChange).toHaveBeenCalledWith(false);
    });
  });

  it("shows empty state when there are no allowed visitors", async () => {
    vi.mocked(api.getLinkAccessRules).mockResolvedValue({
      data: [{ ruleType: "email", value: "leaker@bad.com", action: "block" }],
    });

    renderDialog(true, { ...link, contactIds: [] });

    await waitFor(() => {
      expect(screen.getByText(/No allowed visitors yet/i)).toBeInTheDocument();
    });
    expect(api.getContacts).not.toHaveBeenCalled();
    expect(api.getContactById).not.toHaveBeenCalled();
  });

  it("filters the list by search without expanding the allow scope", async () => {
    vi.mocked(api.getLinkAccessRules).mockResolvedValue({
      data: [
        { ruleType: "email", value: "alice@example.com", action: "allow" },
        { ruleType: "email", value: "bob@example.com", action: "allow" },
      ],
    });
    vi.mocked(api.getContactById).mockResolvedValue({
      id: "c1",
      email: "alice@example.com",
      name: "杨生",
      heatLevel: "cold",
      score: 0,
      scoreHistory: [],
      totalVisits: 0,
      totalDurationSeconds: 0,
      viewedDocuments: [],
    });

    renderDialog();

    await waitFor(() => {
      expect(screen.getByText("bob@example.com")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByPlaceholderText("Search"), {
      target: { value: "杨" },
    });

    expect(screen.getByText("杨生")).toBeInTheDocument();
    expect(screen.queryByText("bob@example.com")).not.toBeInTheDocument();
  });
});
