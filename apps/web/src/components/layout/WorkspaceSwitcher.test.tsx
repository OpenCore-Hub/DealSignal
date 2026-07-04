// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor, act } from "@testing-library/react";
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

const { apiMock, resolveGetWorkspaces } = vi.hoisted(() => {
  let resolve: (value: { data: Workspace[] }) => void = () => {};
  return {
    apiMock: {
      getWorkspaces: vi.fn(
        () => new Promise<{ data: Workspace[] }>((r) => { resolve = r; })
      ),
    },
    resolveGetWorkspaces: (value: { data: Workspace[] }) => resolve(value),
  };
});

vi.mock("@/lib/api", () => ({ api: apiMock }));

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
    apiMock.getWorkspaces.mockClear();
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

  const loadWorkspaces = async () => {
    await act(async () => {
      resolveGetWorkspaces({ data: mockWorkspaces });
      await new Promise((resolve) => setTimeout(resolve, 0));
    });
  };

  it("renders active workspace name and switches workspace", async () => {
    await renderWithProviders();
    await loadWorkspaces();

    const button = await screen.findByRole("button", { name: /Switch workspace/i });
    expect(await screen.findByText("Acme Capital")).toBeInTheDocument();

    fireEvent.click(button);
    const venturaItem = await screen.findByRole("menuitem", { name: /Ventura Fund/i });
    fireEvent.click(venturaItem);

    await waitFor(() => {
      expect(setCurrentWorkspaceMock).toHaveBeenCalledWith(mockWorkspaces[1]);
    });
    expect(navigateMock).toHaveBeenCalledWith("/ventura-fund/dashboard");
  });

  it("navigates to create workspace page", async () => {
    await renderWithProviders();
    await loadWorkspaces();

    const button = await screen.findByRole("button", { name: /Switch workspace/i });
    expect(await screen.findByText("Acme Capital")).toBeInTheDocument();

    fireEvent.click(button);
    const createItem = await screen.findByRole("menuitem", { name: /Create workspace/i });
    fireEvent.click(createItem);

    await waitFor(() => {
      expect(navigateMock).toHaveBeenCalledWith("/workspaces/new");
    });
  });
});
