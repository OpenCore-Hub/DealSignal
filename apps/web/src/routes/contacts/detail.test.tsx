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
import type { Contact, Activity, Document } from "@/types";

const __dirname = dirname(fileURLToPath(import.meta.url));

const { getContactByIdMock, getActivitiesByContactIdMock, getDocumentsMock } = vi.hoisted(() => ({
  getContactByIdMock: vi.fn(),
  getActivitiesByContactIdMock: vi.fn(),
  getDocumentsMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getContactById: getContactByIdMock,
    getActivitiesByContactId: getActivitiesByContactIdMock,
    getDocuments: getDocumentsMock,
  },
}));

const mockContact: Contact = {
  id: "c-1",
  email: "sarah@example.com",
  name: "Sarah Chen",
  organization: "Acme Capital",
  role: "Partner",
  heatLevel: "hot",
  score: 92,
  scoreHistory: [
    { date: "2026-06-20T00:00:00Z", score: 80 },
    { date: "2026-06-21T00:00:00Z", score: 92 },
  ],
  totalVisits: 12,
  totalDurationSeconds: 360,
  lastSeenAt: "2026-06-24T00:00:00Z",
  viewedDocuments: ["doc-1"],
};

const mockActivities: Activity[] = [
  {
    id: "a-1",
    contactId: "c-1",
    contactEmail: "sarah@example.com",
    linkId: "link-1",
    documentTitle: "Q3 Pitch",
    eventType: "page_view",
    pageNumber: 3,
    durationSeconds: 60,
    timestamp: "2026-06-24T00:00:00Z",
    description: "Viewed financial slide",
  },
];

const mockDocuments: Document[] = [
  {
    id: "doc-1",
    title: "Q3 Pitch",
    sourceType: "pdf",
    fileName: "Q3 Pitch.pdf",
    fileType: "pdf",
    fileSize: 1024 * 1024,
    pageCount: 10,
    status: "ready",
    createdAt: "2026-06-20T00:00:00Z",
    updatedAt: "2026-06-20T00:00:00Z",
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
      </I18nextProvider>
    );
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
  return result;
}

describe("ContactDetailPage", () => {
  beforeEach(() => {
    getContactByIdMock.mockReset();
    getActivitiesByContactIdMock.mockReset();
    getDocumentsMock.mockReset();

    getContactByIdMock.mockResolvedValue(mockContact);
    getActivitiesByContactIdMock.mockResolvedValue({ data: mockActivities });
    getDocumentsMock.mockResolvedValue({ data: mockDocuments });
  });

  it("renders contact details and stats", async () => {
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("Sarah Chen")).toBeInTheDocument();
    });

    expect(screen.getByText(/sarah@example\.com/)).toBeInTheDocument();
    expect(screen.getByText(/Acme Capital/)).toBeInTheDocument();
    expect(screen.getByText("12")).toBeInTheDocument();
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

  it("shows error and retries on failure", async () => {
    getContactByIdMock.mockRejectedValue(new Error("network error"));
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("network error")).toBeInTheDocument();
    });

    getContactByIdMock.mockResolvedValue(mockContact);
    fireEvent.click(screen.getByRole("button", { name: /retry/i }));

    await waitFor(() => {
      expect(screen.getByText("Sarah Chen")).toBeInTheDocument();
    });
  });
});
