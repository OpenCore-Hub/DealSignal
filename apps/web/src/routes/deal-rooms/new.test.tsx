// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, act } from "@testing-library/react";
import { MemoryRouter, Routes, Route, useLocation } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { NewDealRoomPage } from "./new";
import { toast } from "sonner";
import type { DealRoomTemplate, DealRoom } from "@/types";

const { getDealRoomTemplatesMock, createDealRoomMock } = vi.hoisted(() => ({
  getDealRoomTemplatesMock: vi.fn(),
  createDealRoomMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getDealRoomTemplates: getDealRoomTemplatesMock,
    createDealRoom: createDealRoomMock,
  },
}));

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

function LocationDisplay() {
  const location = useLocation();
  return <div data-testid="location">{location.pathname}</div>;
}

const mockTemplates: DealRoomTemplate[] = [
  {
    id: "tpl-seed",
    name: "Seed Round",
    description: "Early-stage due diligence room",
    scenario: "startup-fundraising",
    folderStructure: [{ name: "Pitch", description: "Pitch materials" }],
    recommendedFiles: ["Pitch deck", "Cap table"],
    defaultPermissionLevel: "public",
    ndaEnabled: false,
  },
  {
    id: "tpl-series-a",
    name: "Series A",
    description: "Growth-stage data room",
    scenario: "series-a-plus",
    folderStructure: [{ name: "Financials" }],
    recommendedFiles: ["Financial model"],
    defaultPermissionLevel: "standard",
    ndaEnabled: true,
  },
];

const createdRoom: DealRoom = {
  id: "room-1",
  name: "Seed Round Due Diligence",
  description: "Room for seed round",
  template: "startup-fundraising",
  documentCount: 0,
  memberCount: 0,
  pendingApprovals: 0,
  ndaEnabled: false,
  createdAt: "2026-06-24T00:00:00Z",
  status: "active",
};

const resources = {
  en: {
    dealRooms: {
      detail: { back: "Back to deal rooms", noTemplate: "No matching template structure." },
      new: {
        title: "New Deal Room",
        subtitle: "Choose a scenario template",
        basicInfo: "Basic info",
        name: "Name",
        namePlaceholder: "e.g. Seed Round Due Diligence",
        description: "Description",
        descriptionPlaceholder: "Describe the purpose",
        enableNda: "Enable NDA",
        enableNdaDescription: "Require NDA signature before access",
        folders: "Folder structure",
        recommendedFiles: "Recommended files",
        defaultPermission: "Default permission",
        cancel: "Cancel",
        create: "Create deal room",
        creating: "Creating...",
        created: "Deal room created",
        createFailed: "Failed to create deal room",
        folderCount: "{{count}} folders",
        folderCount_one: "{{count}} folder",
      },
      permission: {
        public: { label: "Public Distribution" },
        standard: { label: "Standard Due Diligence" },
        confidential: { label: "Confidential Data Room" },
        collaborative: { label: "Collaborative Review" },
      },
    },
    common: {
      retry: "Retry",
      error: { loadFailed: "Failed to load" },
    },
  },
};

async function initI18n() {
  const instance = i18n.createInstance();
  await instance.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    ns: ["dealRooms", "common"],
    defaultNS: "dealRooms",
    resources,
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
        <MemoryRouter initialEntries={["/acme/deal-rooms/new"]}>
          <Routes>
            <Route path=":workspaceSlug/deal-rooms/new" element={<NewDealRoomPage />} />
            <Route path="*" element={<LocationDisplay />} />
          </Routes>
        </MemoryRouter>
      </I18nextProvider>
    );
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
  return result;
}

describe("NewDealRoomPage", () => {
  beforeEach(() => {
    getDealRoomTemplatesMock.mockReset();
    createDealRoomMock.mockReset();
    vi.mocked(toast.success).mockClear();
    vi.mocked(toast.error).mockClear();
    getDealRoomTemplatesMock.mockResolvedValue({ data: mockTemplates });

    Object.defineProperty(window, "matchMedia", {
      writable: true,
      value: vi.fn().mockImplementation((query: string) => ({
        matches: false,
        media: query,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    });
  });

  it("renders loading skeletons", async () => {
    getDealRoomTemplatesMock.mockReturnValue(new Promise(() => {}));
    await renderPage();

    expect(document.querySelectorAll("[data-slot=\"skeleton\"]").length).toBeGreaterThan(0);
  });

  it("renders templates and pre-fills first template", async () => {
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("Seed Round")).toBeInTheDocument();
    });
    expect(screen.getByDisplayValue("Seed Round")).toBeInTheDocument();
    expect(screen.getByDisplayValue("Early-stage due diligence room")).toBeInTheDocument();
  });

  it("switches template selection and updates form defaults", async () => {
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("Seed Round")).toBeInTheDocument();
    });

    const nameInput = screen.getByPlaceholderText("e.g. Seed Round Due Diligence") as HTMLInputElement;
    const descriptionInput = screen.getByPlaceholderText("Describe the purpose") as HTMLInputElement;
    fireEvent.change(nameInput, { target: { value: "" } });
    fireEvent.change(descriptionInput, { target: { value: "" } });

    fireEvent.click(screen.getByText("Series A"));

    await waitFor(() => {
      expect(screen.getByDisplayValue("Series A")).toBeInTheDocument();
    });
    expect(screen.getByDisplayValue("Growth-stage data room")).toBeInTheDocument();
  });

  it("creates a deal room and navigates to detail", async () => {
    createDealRoomMock.mockResolvedValue(createdRoom);
    await renderPage();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /create deal room/i })).toBeEnabled();
    });

    fireEvent.click(screen.getByRole("button", { name: /create deal room/i }));

    await waitFor(() => {
      expect(createDealRoomMock).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "Seed Round",
          slug: "seed-round",
          description: "Early-stage due diligence room",
          template: "startup-fundraising",
          ndaEnabled: false,
        })
      );
    });

    expect(toast.success).toHaveBeenCalledWith("Deal room created");
    await waitFor(() => {
      expect(screen.getByTestId("location")).toHaveTextContent("/acme/deal-rooms/room-1");
    });
  });

  it("shows error and retries on failure", async () => {
    getDealRoomTemplatesMock.mockRejectedValue(new Error("network error"));
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("network error")).toBeInTheDocument();
    });

    getDealRoomTemplatesMock.mockResolvedValue({ data: mockTemplates });
    fireEvent.click(screen.getByRole("button", { name: /retry/i }));

    await waitFor(() => {
      expect(screen.getByText("Seed Round")).toBeInTheDocument();
    });
  });
});
