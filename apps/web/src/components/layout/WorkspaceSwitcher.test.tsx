// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter, Route, Routes } from "react-router";
import { WorkspaceSwitcher } from "./WorkspaceSwitcher";
import { createTestI18n } from "@/i18n/test-utils";
import type { Workspace } from "@/types";

const setCurrentWorkspaceMock = vi.fn();
const navigateMock = vi.fn();
const toastInfoMock = vi.hoisted(() => vi.fn());

const mockWorkspaces: Workspace[] = [
  { id: "ws_1", name: "mock.workspaces.acme.name", slug: "acme-capital" },
  { id: "ws_2", name: "mock.workspaces.ventura.name", slug: "ventura-fund" },
];

vi.mock("@/lib/api", () => ({
  api: {
    getWorkspaces: vi.fn(() => Promise.resolve({ data: mockWorkspaces })),
  },
}));

vi.mock("@/stores/uiStore", () => ({
  useUIStore: (selector?: (state: { currentWorkspace: Workspace | null; setCurrentWorkspace: (w: Workspace) => void }) => unknown) => {
    const state = {
      currentWorkspace: null,
      setCurrentWorkspace: setCurrentWorkspaceMock,
    };
    return selector ? selector(state) : state;
  },
}));

vi.mock("sonner", () => ({
  toast: {
    info: toastInfoMock,
  },
}));

vi.mock("react-router", async () => {
  const actual = await vi.importActual<typeof import("react-router")>("react-router");
  return {
    ...actual,
    useNavigate: () => navigateMock,
  };
});

describe("WorkspaceSwitcher", () => {
  beforeEach(() => {
    setCurrentWorkspaceMock.mockClear();
    navigateMock.mockClear();
    toastInfoMock.mockClear();
  });

  const renderWithProviders = async (initialRoute = "/acme-capital/dashboard") => {
    const i18n = await createTestI18n({
      common: {
        "mock.workspaces.acme.name": "Acme Capital",
        "mock.workspaces.ventura.name": "Ventura Fund",
        loading: "Loading...",
      },
      layout: {
        "workspaceSwitcher.switchWorkspace": "Switch workspace",
        "workspaceSwitcher.label": "Workspaces",
        "workspaceSwitcher.createWorkspace": "Create workspace",
        "workspaceSwitcher.createWorkspaceComingSoon": "Workspace creation requires backend support.",
      },
    });
    return render(
      <I18nextProvider i18n={i18n}>
        <MemoryRouter initialEntries={[initialRoute]}>
          <Routes>
            <Route path="/:workspaceSlug/*" element={<WorkspaceSwitcher />} />
          </Routes>
        </MemoryRouter>
      </I18nextProvider>
    );
  };

  it("renders active workspace name and switches workspace", async () => {
    await renderWithProviders();
    await waitFor(() => {
      expect(screen.getByText("Acme Capital")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole("button", { name: /Switch workspace/i }));
    await waitFor(() => {
      expect(screen.getByRole("menuitem", { name: /Ventura Fund/i })).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole("menuitem", { name: /Ventura Fund/i }));
    expect(setCurrentWorkspaceMock).toHaveBeenCalledWith(mockWorkspaces[1]);
    expect(navigateMock).toHaveBeenCalledWith("/ventura-fund/dashboard");
  });

  it("shows coming-soon toast when creating workspace", async () => {
    await renderWithProviders();
    await waitFor(() => {
      expect(screen.getByText("Acme Capital")).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole("button", { name: /Switch workspace/i }));
    await waitFor(() => {
      expect(screen.getByRole("menuitem", { name: /Create workspace/i })).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole("menuitem", { name: /Create workspace/i }));
    expect(toastInfoMock).toHaveBeenCalledWith("Workspace creation requires backend support.");
  });
});
