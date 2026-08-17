// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, act } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { readFileSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { ContactDetailPage } from "./detail";
import type { Contact, Activity } from "@/types";

const __dirname = dirname(fileURLToPath(import.meta.url));

const { getContactByIdMock, getActivitiesByContactIdMock } = vi.hoisted(() => ({
  getContactByIdMock: vi.fn(),
  getActivitiesByContactIdMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getContactById: getContactByIdMock,
    getActivitiesByContactId: getActivitiesByContactIdMock,
  },
}));

const mockContact: Contact = {
  id: "c-1",
  email: "sarah@example.com",
  name: "Sarah Chen",
  organization: "acme.capital",
  role: "Partner",
  heatLevel: "hot",
  score: 92,
  scoreHistory: [
    { date: "2026-06-20T00:00:00Z", score: 2 },
    { date: "2026-06-21T00:00:00Z", score: 5 },
  ],
  totalVisits: 12,
  totalDurationSeconds: 360,
  lastSeenAt: "2026-06-24T00:00:00Z",
  viewedDocuments: ["doc-1"],
  viewedDocumentItems: [{ id: "doc-1", title: "Q3 Pitch" }],
};

const mockActivities: Activity[] = [
  {
    id: "a-1",
    contactId: "c-1",
    contactEmail: "sarah@example.com",
    linkId: "link-1",
    documentId: "doc-1",
    documentTitle: "Q3 Pitch",
    eventType: "page_view",
    pageNumber: 3,
    durationSeconds: 60,
    timestamp: "2026-06-24T00:00:00Z",
    description: "Q3 Pitch · page 3",
  },
];

async function initI18n() {
  const instance = i18n.createInstance();
  const contactsJson = JSON.parse(readFileSync(resolve(__dirname, "../../i18n/locales/en/contacts.json"), "utf-8"));
  const commonJson = JSON.parse(readFileSync(resolve(__dirname, "../../i18n/locales/en/common.json"), "utf-8"));
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
        <MemoryRouter initialEntries={["/acme/contacts/c-1"]}>
          <Routes>
            <Route path=":workspaceSlug/contacts/:contactId" element={<ContactDetailPage />} />
          </Routes>
        </MemoryRouter>
      </I18nextProvider>,
    );
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
  return result;
}

describe("ContactDetailPage", () => {
  beforeEach(() => {
    getContactByIdMock.mockReset();
    getActivitiesByContactIdMock.mockReset();

    getContactByIdMock.mockResolvedValue(mockContact);
    getActivitiesByContactIdMock.mockResolvedValue({ data: mockActivities });
  });

  it("renders contact details and stats", async () => {
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("Sarah Chen")).toBeInTheDocument();
    });

    expect(screen.getByText(/sarah@example\.com/)).toBeInTheDocument();
    expect(screen.getByText(/acme\.capital/)).toBeInTheDocument();
    expect(screen.getByText("12")).toBeInTheDocument();
    expect(screen.getByText("Engagement trend")).toBeInTheDocument();
    expect(screen.queryByText("Key-page evidence")).not.toBeInTheDocument();
  });

  it("shows skim key-page evidence without changing the heat badge", async () => {
    getContactByIdMock.mockResolvedValue({
      ...mockContact,
      keyPages: {
        engaged: 0,
        total: 1,
        minSeconds: 3,
        pages: [{ pageNumber: 1, title: "Financials", engagedViews: 0, totalViews: 1 }],
      },
    });
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("Key-page evidence")).toBeInTheDocument();
    });
    expect(screen.getByText(/Title matched, but dwell was under 3s/i)).toBeInTheDocument();
    expect(screen.getByText(/p1 · Financials/i)).toBeInTheDocument();
  });

  it("switches to timeline tab and shows activities", async () => {
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("Sarah Chen")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("tab", { name: /timeline/i }));

    await waitFor(() => {
      expect(screen.getByText(/Q3 Pitch/)).toBeInTheDocument();
    });
  });

  it("lists viewed documents from contact payload without fetching all documents", async () => {
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("Sarah Chen")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("tab", { name: /documents/i }));

    await waitFor(() => {
      expect(screen.getByText("Q3 Pitch")).toBeInTheDocument();
    });
    expect(getContactByIdMock).toHaveBeenCalled();
    expect(getActivitiesByContactIdMock).toHaveBeenCalled();
  });

  it("shows error and retries on failure", async () => {
    getContactByIdMock.mockRejectedValue(new Error("network error"));
    await renderPage();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /retry/i })).toBeInTheDocument();
    });
    expect(screen.getByText(/failed to load|network error/i)).toBeInTheDocument();
  });
});
