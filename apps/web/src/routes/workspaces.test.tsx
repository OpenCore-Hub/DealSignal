// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, act } from "@testing-library/react";
import { MemoryRouter, Routes, Route, useLocation } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { WorkspacesPage } from "./workspaces";
import type { Workspace } from "@/types";

const { getWorkspacesMock } = vi.hoisted(() => ({
  getWorkspacesMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getWorkspaces: getWorkspacesMock,
  },
}));

function LocationDisplay() {
  const location = useLocation();
  return <div data-testid="location">{location.pathname}</div>;
}

const mockWorkspaces: Workspace[] = [
  { id: "ws-1", slug: "acme", name: "Acme Capital" },
  { id: "ws-2", slug: "ventura", name: "Ventura Fund" },
];

const resources = {
  en: {
    common: {
      selectWorkspace: "Select workspace",
      selectWorkspaceDescription: "Choose a workspace to enter",
      createWorkspace: "Create workspace",
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
    ns: ["common"],
    defaultNS: "common",
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
        <MemoryRouter initialEntries={["/workspaces"]}>
          <Routes>
            <Route path="/workspaces" element={<WorkspacesPage />} />
            <Route path="*" element={<LocationDisplay />} />
          </Routes>
        </MemoryRouter>
      </I18nextProvider>
    );
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
  return result;
}

describe("WorkspacesPage", () => {
  beforeEach(() => {
    getWorkspacesMock.mockReset();
  });

  it("renders loading state", async () => {
    getWorkspacesMock.mockReturnValue(new Promise(() => {}));
    await renderPage();

    expect(document.querySelectorAll("[data-slot=\"skeleton\"]").length).toBeGreaterThan(0);
  });

  it("renders workspace cards", async () => {
    getWorkspacesMock.mockResolvedValue({ data: mockWorkspaces });
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("Select workspace")).toBeInTheDocument();
    });

    expect(screen.getByTestId("workspace-card-acme")).toBeInTheDocument();
    expect(screen.getByTestId("workspace-card-ventura")).toBeInTheDocument();
    expect(screen.getByText("Acme Capital")).toBeInTheDocument();
    expect(screen.getByText("Ventura Fund")).toBeInTheDocument();
  });

  it("redirects to dashboard when only one workspace exists", async () => {
    getWorkspacesMock.mockResolvedValue({ data: [mockWorkspaces[0]] });
    await renderPage();

    await waitFor(() => {
      expect(screen.getByTestId("location")).toHaveTextContent("/acme/dashboard");
    });
  });

  it("shows error and retries on failure", async () => {
    getWorkspacesMock.mockRejectedValue(new Error("network error"));
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("network error")).toBeInTheDocument();
    });

    getWorkspacesMock.mockResolvedValue({ data: mockWorkspaces });
    fireEvent.click(screen.getByRole("button", { name: /retry/i }));

    await waitFor(() => {
      expect(screen.getByText("Select workspace")).toBeInTheDocument();
    });
  });
});
