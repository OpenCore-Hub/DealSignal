// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, act } from "@testing-library/react";
import { MemoryRouter, Routes, Route, useLocation } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { NewDealRoomPage } from "./new";
import { toast } from "sonner";
import { ApiError } from "@/lib/apiClient";
import { useUIStore } from "@/stores/uiStore";
import type { DealRoomTemplate, DealRoom } from "@/types";

const { getDealRoomTemplatesMock, createDealRoomMock, getBillingInfoMock } = vi.hoisted(() => ({
  getDealRoomTemplatesMock: vi.fn(),
  createDealRoomMock: vi.fn(),
  getBillingInfoMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getDealRoomTemplates: getDealRoomTemplatesMock,
    createDealRoom: createDealRoomMock,
    getBillingInfo: getBillingInfoMock,
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
      detail: { back: "Back to data rooms", noTemplate: "No matching template structure." },
      new: {
        title: "Choose a template",
        breadcrumb: "Data Room",
        subtitle: "The data room and its folders are created from your selection.",
        scenario: "Scenario",
        basicInfo: "Basic info",
        name: "Name",
        nameRequired: "Required",
        namePlaceholder: "e.g. Acme Series A",
        nameError: {
          empty: "Enter a name.",
          short: "Use at least 2 characters.",
          long: "Keep the name under 80 characters.",
          format: "Include letters or numbers, and do not use < or >.",
        },
        description: "Description",
        descriptionOptional: "Optional",
        descriptionPlaceholder: "Describe the purpose",
        enableNda: "Enable NDA",
        enableNdaDescription: "Require NDA signature before access",
        ndaFromTemplate: "Members sign before they enter.",
        ndaPlanRequired: "NDA is on Business and above.",
        scenarioGroupNda: "NDA",
        scenarioGroupOpen: "No NDA",
        roomLimitReached: "You've reached the data room limit for your plan. Upgrade to create more.",
        folders: "Folder structure",
        recommendedFiles: "Recommended files",
        defaultPermission: "Default permission",
        cancel: "Cancel",
        create: "Create data room",
        creating: "Creating...",
        created: "Data room created",
        createFailed: "Failed to create data room",
        folderCount: "{{count}} folders",
        folderCount_one: "{{count}} folder",
        customFolderMeta: "Build your own",
        customFoldersHint: "No folders yet. Add them after you create the room.",
        foldersDialog: {
          title: "Review folders",
          subtitle: "{{name}} — keep what you need, rename in the list, add a folder or one subfolder under it.",
          add: "Add folder",
          addSubfolder: "Add subfolder",
          addPlaceholder: "Folder name",
          rename: "Rename",
          select: "Keep {{name}}",
          empty: "Add a folder to continue.",
          create: "Create",
        },
      },
      permission: {
        public: { label: "Public Distribution" },
        standard: { label: "Standard Due Diligence" },
        confidential: { label: "Confidential Data Room" },
        collaborative: { label: "Collaborative Review" },
      },
      templates: {
        custom: {
          name: "Completely Custom",
          description: "Start with an empty room and add your own folders.",
        },
      },
    },
    common: {
      retry: "Retry",
      error: {
        loadFailed: "Failed to load",
        saveFailed: "Failed to save",
        codes: {
          plan_limit_rooms: "You've reached the data room limit for your plan. Upgrade to create more.",
          plan_feature_nda: "NDA requirements are not available on your plan. Upgrade to Business or higher.",
        },
      },
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

const namePlaceholder = "e.g. Acme Series A";

function fillRoomName(value = "Seed-Round") {
  fireEvent.change(screen.getByLabelText(/name/i), { target: { value } });
}

async function confirmFolderDialog() {
  await waitFor(() => {
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });
  fireEvent.click(screen.getByTestId("confirm-create-room"));
}

describe("NewDealRoomPage", () => {
  beforeEach(() => {
    getDealRoomTemplatesMock.mockReset();
    createDealRoomMock.mockReset();
    getBillingInfoMock.mockReset();
    useUIStore.getState().setBreadcrumbTail(null);
    vi.mocked(toast.success).mockClear();
    vi.mocked(toast.error).mockClear();
    getDealRoomTemplatesMock.mockResolvedValue({ data: mockTemplates });
    getBillingInfoMock.mockResolvedValue({
      plan: "trial",
      period: "monthly",
      trialExpired: false,
      storageUsed: 0,
      storageLimit: 0,
      linksUsed: 0,
      linksLimit: 0,
      roomsUsed: 0,
      roomsLimit: 0,
      seatsUsed: 1,
      seatsLimit: 10,
      customDomainEnabled: true,
      watermarkEnabled: true,
      ndaEnabled: true,
      visitorAskAiEnabled: true,
    });

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

  it("sets the workspace breadcrumb tail to Data Room", async () => {
    await renderPage();

    expect(useUIStore.getState().breadcrumbTail).toEqual({ label: "Data Room" });
  });

  it("omits the back link and returns to the list on cancel", async () => {
    await renderPage();

    expect(screen.queryByText("Back to data rooms")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /^cancel$/i }));
    expect(screen.getByTestId("location")).toHaveTextContent("/acme/deal-rooms");
  });

  it("renders templates and uses the first template as placeholders", async () => {
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("Seed Round")).toBeInTheDocument();
    });
    expect(screen.getByPlaceholderText(namePlaceholder)).toHaveValue("");
    expect(screen.getByPlaceholderText("Early-stage due diligence room")).toHaveValue("");
    expect(screen.getByRole("button", { name: /create data room/i })).toBeDisabled();
    expect(screen.getByRole("button", { name: /seed round/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /series a/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /completely custom/i })).toBeInTheDocument();
    expect(screen.getByRole("switch", { name: /enable nda/i })).toHaveAttribute(
      "aria-checked",
      "false",
    );
  });

  it("switches template placeholders without filling the form", async () => {
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("Seed Round")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Series A"));

    await waitFor(() => {
      expect(screen.getByPlaceholderText("Growth-stage data room")).toHaveValue("");
    });
    expect(screen.getByPlaceholderText(namePlaceholder)).toHaveValue("");
    expect(screen.getByRole("button", { name: /create data room/i })).toBeDisabled();
    expect(screen.getByRole("switch", { name: /enable nda/i })).toHaveAttribute(
      "aria-checked",
      "true",
    );
  });

  it("enables create only after the name passes validation", async () => {
    await renderPage();
    await waitFor(() => {
      expect(screen.getByPlaceholderText(namePlaceholder)).toBeInTheDocument();
    });
    const nameInput = screen.getByLabelText(/name/i);
    fireEvent.change(nameInput, { target: { value: "A < B" } });
    fireEvent.blur(nameInput);
    expect(
      screen.getByText("Include letters or numbers, and do not use < or >."),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /create data room/i })).toBeDisabled();
    fillRoomName("创业融资");
    expect(screen.getByRole("button", { name: /create data room/i })).toBeEnabled();
  });

  it("uses an opaque slug for CJK-only names instead of the template scenario", async () => {
    createDealRoomMock.mockResolvedValue(createdRoom);
    await renderPage();
    await waitFor(() => {
      expect(screen.getByPlaceholderText(namePlaceholder)).toBeInTheDocument();
    });
    fillRoomName("创业融资");
    fireEvent.click(screen.getByRole("button", { name: /create data room/i }));
    await confirmFolderDialog();
    await waitFor(() => {
      expect(createDealRoomMock).toHaveBeenCalled();
    });
    expect(createDealRoomMock).toHaveBeenCalledWith(
      expect.objectContaining({
        name: "创业融资",
        slug: expect.stringMatching(/^room-[0-9a-f]{10}$/),
      }),
    );
  });

  it("still shows completely custom when the API catalog omits it", async () => {
    await renderPage();
    await waitFor(() => {
      expect(screen.getByRole("button", { name: /completely custom/i })).toBeInTheDocument();
    });
    fireEvent.click(screen.getByText("Completely Custom"));
    expect(screen.getByRole("button", { name: /build your own/i })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(screen.getByRole("switch", { name: /enable nda/i })).toHaveAttribute(
      "aria-checked",
      "false",
    );
  });

  it("creates a deal room and navigates to detail", async () => {
    createDealRoomMock.mockResolvedValue(createdRoom);
    await renderPage();

    await waitFor(() => {
      expect(screen.getByPlaceholderText(namePlaceholder)).toBeInTheDocument();
    });
    fillRoomName("Seed-Round");
    expect(screen.getByRole("button", { name: /create data room/i })).toBeEnabled();

    fireEvent.click(screen.getByRole("button", { name: /create data room/i }));
    expect(createDealRoomMock).not.toHaveBeenCalled();
    await confirmFolderDialog();

    await waitFor(() => {
      expect(createDealRoomMock).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "Seed-Round",
          slug: "seed-round",
          description: "",
          template: "startup-fundraising",
          ndaEnabled: false,
          folders: [{ name: "Pitch", path: "/pitch", description: "Pitch materials", sort_order: 0 }],
        })
      );
    });

    expect(toast.success).toHaveBeenCalledWith("Data room created");
    await waitFor(() => {
      expect(screen.getByTestId("location")).toHaveTextContent("/acme/deal-rooms/room-1");
    });
  });

  it("creates the room from the folders edited in the dialog", async () => {
    createDealRoomMock.mockResolvedValue(createdRoom);
    await renderPage();

    await waitFor(() => {
      expect(screen.getByPlaceholderText(namePlaceholder)).toBeInTheDocument();
    });
    fillRoomName("Seed-Round");
    fireEvent.click(screen.getByRole("button", { name: /create data room/i }));
    await waitFor(() => {
      expect(screen.getByRole("dialog")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: /^rename$/i }));
    fireEvent.change(screen.getByLabelText("Folder name"), { target: { value: "Deck" } });
    fireEvent.blur(screen.getByLabelText("Folder name"));
    fireEvent.click(screen.getByRole("button", { name: /add subfolder/i }));
    fireEvent.change(screen.getByLabelText("Folder name"), { target: { value: "Annex" } });
    fireEvent.blur(screen.getByLabelText("Folder name"));

    fireEvent.click(screen.getByTestId("confirm-create-room"));
    await waitFor(() => {
      expect(createDealRoomMock).toHaveBeenCalledWith(
        expect.objectContaining({
          folders: [
            { name: "Deck", path: "/deck", sort_order: 0 },
            { name: "Annex", path: "/deck/annex", sort_order: 1 },
          ],
        }),
      );
    });
  });

  it("retries with a suffixed slug when the URL is already taken", async () => {
    createDealRoomMock
      .mockRejectedValueOnce(
        new ApiError({
          status: 409,
          code: "duplicate_slug",
          message: "a data room with this URL already exists",
          requestId: "req_slug",
        }),
      )
      .mockResolvedValueOnce(createdRoom);
    await renderPage();
    await waitFor(() => {
      expect(screen.getByPlaceholderText(namePlaceholder)).toBeInTheDocument();
    });
    fillRoomName("Seed-Round");
    fireEvent.click(screen.getByRole("button", { name: /create data room/i }));
    await confirmFolderDialog();
    await waitFor(() => {
      expect(createDealRoomMock).toHaveBeenCalledTimes(2);
    });
    expect(createDealRoomMock).toHaveBeenNthCalledWith(
      1,
      expect.objectContaining({ slug: "seed-round" }),
    );
    expect(createDealRoomMock).toHaveBeenNthCalledWith(
      2,
      expect.objectContaining({ slug: "seed-round-2" }),
    );
    expect(toast.success).toHaveBeenCalledWith("Data room created");
  });

  it("disables create and shows hint when room quota is exhausted", async () => {
    getBillingInfoMock.mockResolvedValue({
      plan: "free",
      period: "monthly",
      trialExpired: false,
      storageUsed: 0,
      storageLimit: 1,
      linksUsed: 0,
      linksLimit: 20,
      roomsUsed: 1,
      roomsLimit: 1,
      seatsUsed: 1,
      seatsLimit: 1,
      customDomainEnabled: false,
      watermarkEnabled: false,
      ndaEnabled: false,
      visitorAskAiEnabled: false,
    });
    await renderPage();
    await waitFor(() => {
      expect(screen.getByTestId("new-room-limit-hint")).toBeInTheDocument();
    });
    expect(screen.getByRole("button", { name: /create data room/i })).toBeDisabled();
  });

  it("turns NDA on for a confidential scenario, without requiring a manual toggle", async () => {
    createDealRoomMock.mockResolvedValue({ ...createdRoom, ndaEnabled: true });
    await renderPage();
    await waitFor(() => {
      expect(screen.getByText("Series A")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByText("Series A"));
    await waitFor(() => {
      expect(screen.getByRole("switch", { name: /enable nda/i })).toHaveAttribute(
        "aria-checked",
        "true",
      );
    });
    fillRoomName("Series-A-Room");
    fireEvent.click(screen.getByRole("button", { name: /create data room/i }));
    await confirmFolderDialog();
    await waitFor(() => {
      expect(createDealRoomMock).toHaveBeenCalledWith(
        expect.objectContaining({
          template: "series-a-plus",
          ndaEnabled: true,
        }),
      );
    });
  });

  it("forces NDA off when the plan lacks NDA, even for confidential scenarios", async () => {
    createDealRoomMock.mockResolvedValue(createdRoom);
    getBillingInfoMock.mockResolvedValue({
      plan: "free",
      period: "monthly",
      trialExpired: false,
      storageUsed: 0,
      storageLimit: 1,
      linksUsed: 0,
      linksLimit: 20,
      roomsUsed: 0,
      roomsLimit: 1,
      seatsUsed: 1,
      seatsLimit: 1,
      customDomainEnabled: false,
      watermarkEnabled: false,
      ndaEnabled: false,
      visitorAskAiEnabled: false,
    });
    await renderPage();
    await waitFor(() => {
      expect(screen.getByText("Series A")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByText("Series A"));
    await waitFor(() => {
      expect(screen.getByText(/nda is on business and above/i)).toBeInTheDocument();
    });
    expect(screen.getByRole("switch", { name: /enable nda/i })).toBeDisabled();
    expect(screen.getByRole("switch", { name: /enable nda/i })).toHaveAttribute(
      "aria-checked",
      "false",
    );
    fillRoomName("Series-A-Room");
    fireEvent.click(screen.getByRole("button", { name: /create data room/i }));
    await confirmFolderDialog();
    await waitFor(() => {
      expect(createDealRoomMock).toHaveBeenCalledWith(
        expect.objectContaining({ ndaEnabled: false }),
      );
    });
  });

  it("surfaces plan_limit_rooms from the API via toast", async () => {
    createDealRoomMock.mockRejectedValueOnce(
      new ApiError({
        status: 403,
        code: "plan_limit_rooms",
        message: "data room limit reached for this plan",
        requestId: "req_rooms",
      }),
    );
    await renderPage();
    await waitFor(() => {
      expect(screen.getByPlaceholderText(namePlaceholder)).toBeInTheDocument();
    });
    fillRoomName("Seed-Round");
    fireEvent.click(screen.getByRole("button", { name: /create data room/i }));
    await confirmFolderDialog();
    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith(
        "You've reached the data room limit for your plan. Upgrade to create more.",
      );
    });
  });

  it("shows error and retries on failure", async () => {
    getDealRoomTemplatesMock.mockRejectedValue(new Error("network error"));
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("Failed to load")).toBeInTheDocument();
    });

    getDealRoomTemplatesMock.mockResolvedValue({ data: mockTemplates });
    fireEvent.click(screen.getByRole("button", { name: /retry/i }));

    await waitFor(() => {
      expect(screen.getByText("Seed Round")).toBeInTheDocument();
    });
  });
});
