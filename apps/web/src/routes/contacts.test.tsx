// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, act } from "@testing-library/react";
import { MemoryRouter, Routes, Route, useNavigate } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { readFileSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { ContactsPage } from "./contacts";
import type { Contact } from "@/types";

const __dirname = dirname(fileURLToPath(import.meta.url));

const { getContactsMock, sendMarketingBatchMock } = vi.hoisted(() => ({
  getContactsMock: vi.fn(),
  sendMarketingBatchMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getContacts: getContactsMock,
    sendMarketingBatch: sendMarketingBatchMock,
  },
}));

const mockContacts: Contact[] = [
  {
    id: "contact_1",
    email: "sarah.chen@horizon.vc",
    name: "Sarah Chen",
    organization: "Horizon VC",
    heatLevel: "hot",
    score: 92,
    scoreHistory: [],
    totalVisits: 5,
    totalDurationSeconds: 1112,
    lastSeenAt: "2026-06-20T18:42:00Z",
    viewedDocuments: ["doc_1"],
  },
  {
    id: "contact_2",
    email: "marcus@boldstart.vc",
    name: "Marcus Johnson",
    organization: "Boldstart",
    heatLevel: "warm",
    score: 64,
    scoreHistory: [],
    totalVisits: 2,
    totalDurationSeconds: 320,
    lastSeenAt: "2026-06-19T20:30:00Z",
    viewedDocuments: ["doc_2"],
  },
];

async function initI18n() {
  const instance = i18n.createInstance();
  const contactsJson = JSON.parse(
    readFileSync(resolve(__dirname, "../i18n/locales/en/contacts.json"), "utf-8"),
  );
  const commonJson = JSON.parse(
    readFileSync(resolve(__dirname, "../i18n/locales/en/common.json"), "utf-8"),
  );
  await instance.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    ns: ["contacts", "common"],
    defaultNS: "contacts",
    resources: { en: { contacts: contactsJson, common: commonJson } },
    interpolation: { escapeValue: false },
  });
  return instance;
}

async function renderPage() {
  const i18nInstance = await initI18n();
  let result!: ReturnType<typeof render>;
  await act(async () => {
    result = render(
      <I18nextProvider i18n={i18nInstance}>
        <MemoryRouter initialEntries={["/acme/contacts"]}>
          <Routes>
            <Route path=":workspaceSlug/contacts" element={<ContactsPage />} />
            <Route path=":workspaceSlug/contacts/:contactId" element={<div>detail</div>} />
          </Routes>
        </MemoryRouter>
      </I18nextProvider>,
    );
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
  return result;
}

describe("ContactsPage multi-select bulk email", () => {
  beforeEach(() => {
    getContactsMock.mockReset();
    sendMarketingBatchMock.mockReset();
    getContactsMock.mockResolvedValue({ data: mockContacts });
    sendMarketingBatchMock.mockResolvedValue({
      data: { sent: 1, failed: 0, log_ids: ["elog_1"], failed_recipients: [] },
    });
  });

  it("keeps bulk email disabled until a contact is selected", async () => {
    await renderPage();
    await waitFor(() => {
      expect(screen.getByText("Sarah Chen")).toBeInTheDocument();
    });
    expect(screen.getByTestId("contacts-bulk-email")).toBeDisabled();
  });

  it("enables bulk email for selected contacts only and sends those recipients", async () => {
    await renderPage();
    await waitFor(() => {
      expect(screen.getByText("Sarah Chen")).toBeInTheDocument();
    });
    expect(getContactsMock).toHaveBeenCalledWith("acme");

    fireEvent.click(screen.getByTestId("contact-select-contact_1"));
    await waitFor(() => {
      expect(screen.getByTestId("contacts-bulk-email")).not.toBeDisabled();
    });
    expect(screen.getByTestId("contacts-bulk-email")).toHaveTextContent("Bulk email (1)");

    fireEvent.click(screen.getByTestId("contacts-bulk-email"));
    await waitFor(() => {
      expect(screen.getByRole("dialog")).toBeInTheDocument();
    });
    expect(screen.getByText(/1 selected contacts/i)).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Subject"), {
      target: { value: "Follow up" },
    });
    fireEvent.click(screen.getByRole("button", { name: /send/i }));

    await waitFor(() => {
      expect(sendMarketingBatchMock).toHaveBeenCalledWith(
        {
          recipients: ["sarah.chen@horizon.vc"],
          subject: "Follow up",
          body: "",
          track_opens: true,
          track_clicks: true,
        },
        "acme",
      );
    });
  });

  it("selects all filtered contacts", async () => {
    await renderPage();
    await waitFor(() => {
      expect(screen.getByText("Sarah Chen")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("contacts-select-all"));
    await waitFor(() => {
      expect(screen.getByTestId("contacts-selected-count")).toHaveTextContent("2 selected");
    });
    expect(screen.getByTestId("contacts-bulk-email")).toHaveTextContent("Bulk email (2)");
  });

  it("reloads contacts when workspace slug changes (no cross-workspace leak)", async () => {
    getContactsMock
      .mockResolvedValueOnce({ data: mockContacts })
      .mockResolvedValue({
        data: [
          {
            ...mockContacts[0],
            id: "ws2_contact",
            name: "Other Workspace Contact",
            email: "other@ws2.test",
          },
        ],
      });

    const i18nInstance = await initI18n();
    function WorkspaceSwitchHarness() {
      const navigate = useNavigate();
      return (
        <div>
          <button type="button" onClick={() => navigate("/other-ws/contacts")}>
            switch-ws
          </button>
          <Routes>
            <Route path=":workspaceSlug/contacts" element={<ContactsPage />} />
          </Routes>
        </div>
      );
    }

    await act(async () => {
      render(
        <I18nextProvider i18n={i18nInstance}>
          <MemoryRouter initialEntries={["/acme/contacts"]}>
            <WorkspaceSwitchHarness />
          </MemoryRouter>
        </I18nextProvider>,
      );
      await new Promise((resolve) => setTimeout(resolve, 0));
    });

    await waitFor(() => {
      expect(screen.getByText("Sarah Chen")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: "switch-ws" }));

    await waitFor(() => {
      expect(screen.getByText("Other Workspace Contact")).toBeInTheDocument();
    });
    expect(screen.queryByText("Sarah Chen")).not.toBeInTheDocument();
    expect(getContactsMock).toHaveBeenCalledWith("acme");
    expect(getContactsMock).toHaveBeenCalledWith("other-ws");
    expect(getContactsMock.mock.calls.length).toBeGreaterThanOrEqual(2);
  });
});
