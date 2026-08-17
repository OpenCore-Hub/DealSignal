// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor, act } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router";
import { I18nextProvider } from "react-i18next";
import { CreateWorkspacePage } from "./new";
import { createTestI18n } from "@/i18n/test-utils";
import { ApiError } from "@/lib/apiClient";

const navigateMock = vi.fn();
const setCurrentWorkspaceMock = vi.fn();
const { createWorkspaceMock, getMeMock } = vi.hoisted(() => ({
  createWorkspaceMock: vi.fn(),
  getMeMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    createWorkspace: createWorkspaceMock,
    getMe: getMeMock,
  },
}));

vi.mock("@/stores/uiStore", () => ({
  useUIStore: (selector?: (state: { setCurrentWorkspace: (w: unknown) => void }) => unknown) => {
    const state = { setCurrentWorkspace: setCurrentWorkspaceMock };
    return selector ? selector(state) : state;
  },
}));

vi.mock("react-router", async () => {
  const actual = await vi.importActual<typeof import("react-router")>("react-router");
  return {
    ...actual,
    useNavigate: () => navigateMock,
  };
});

const commonResources = {
  back: "Back",
  createWorkspace: "Create workspace",
  creating: "Creating...",
  workspaceName: "Workspace name",
  workspaceNamePlaceholder: "My workspace",
  workspaceSlug: "Workspace slug",
  workspaceSlugPlaceholder: "myworkspace",
  workspaceSlugConflict: "This workspace slug is already taken. Please choose another one.",
  verifyEmailForTrial: "Verify your email to start the 14-day trial.",
  brandColor: "Brand Color",
  error: {
    saveFailed: "Failed to save",
    duplicateSlug: "This URL is already taken. Please choose a different name.",
  },
};

async function renderPage() {
  const i18n = await createTestI18n({ common: commonResources });
  let result!: ReturnType<typeof render>;
  await act(async () => {
    result = render(
      <I18nextProvider i18n={i18n}>
        <MemoryRouter initialEntries={["/workspaces/new"]}>
          <Routes>
            <Route path="/workspaces/new" element={<CreateWorkspacePage />} />
          </Routes>
        </MemoryRouter>
      </I18nextProvider>
    );
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
  return result;
}

describe("CreateWorkspacePage", () => {
  beforeEach(() => {
    navigateMock.mockClear();
    setCurrentWorkspaceMock.mockClear();
    createWorkspaceMock.mockReset();
    getMeMock.mockReset();
    getMeMock.mockResolvedValue({ id: "u1", email: "a@example.com", email_verified: true });
  });

  it("derives slug from English letters and digits only", async () => {
    await renderPage();

    const name = screen.getByLabelText("Workspace name");
    const slug = screen.getByLabelText("Workspace slug") as HTMLInputElement;

    fireEvent.change(name, { target: { value: "我的 Acme Capital 2026!" } });
    expect(slug.value).toBe("acmecapital2026");

    fireEvent.change(slug, { target: { value: "my-workspace 中文" } });
    expect(slug.value).toBe("myworkspace");
  });

  it("keeps a manually edited slug when the name changes", async () => {
    await renderPage();

    const name = screen.getByLabelText("Workspace name");
    const slug = screen.getByLabelText("Workspace slug") as HTMLInputElement;

    fireEvent.change(name, { target: { value: "Acme" } });
    expect(slug.value).toBe("acme");

    fireEvent.change(slug, { target: { value: "custom01" } });
    fireEvent.change(name, { target: { value: "Acme Two" } });
    expect(slug.value).toBe("custom01");
  });

  it("shows a friendly conflict message and keeps the form open", async () => {
    createWorkspaceMock.mockRejectedValue(
      new ApiError({
        status: 409,
        code: "slug_conflict",
        message: "a workspace with this URL already exists",
        requestId: "req-1",
      })
    );
    await renderPage();

    const name = screen.getByLabelText("Workspace name");
    const slug = screen.getByLabelText("Workspace slug") as HTMLInputElement;
    fireEvent.change(name, { target: { value: "Acme" } });
    expect(slug.value).toBe("acme");

    fireEvent.click(screen.getByRole("button", { name: "Create workspace" }));

    await waitFor(() => {
      expect(
        screen.getByText("This workspace slug is already taken. Please choose another one.")
      ).toBeInTheDocument();
    });
    expect(createWorkspaceMock).toHaveBeenCalledWith({
      name: "Acme",
      slug: "acme",
      brand_color: "#0055ff",
    });
    expect(navigateMock).not.toHaveBeenCalled();
  });

  it("saves the hex color selected by the brand color control", async () => {
    createWorkspaceMock.mockResolvedValue({
      id: "ws-1",
      slug: "acme",
      name: "Acme",
      brand_color: "#3366ff",
      created_at: "2026-08-12T00:00:00Z",
    });
    await renderPage();

    fireEvent.change(screen.getByLabelText("Workspace name"), {
      target: { value: "Acme" },
    });
    fireEvent.change(screen.getByLabelText("Brand Color"), {
      target: { value: "#3366ff" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Create workspace" }));

    await waitFor(() => {
      expect(createWorkspaceMock).toHaveBeenCalledWith({
        name: "Acme",
        slug: "acme",
        brand_color: "#3366ff",
      });
    });
  });

  it("navigates to the new workspace after successful creation", async () => {
    createWorkspaceMock.mockResolvedValue({
      id: "ws-1",
      slug: "acme",
      name: "Acme",
      brand_color: "#0055ff",
      created_at: "2026-08-12T00:00:00Z",
    });
    await renderPage();

    fireEvent.change(screen.getByLabelText("Workspace name"), {
      target: { value: "Acme" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Create workspace" }));

    await waitFor(() => {
      expect(setCurrentWorkspaceMock).toHaveBeenCalledWith({
        id: "ws-1",
        slug: "acme",
        name: "Acme",
        brand_color: "#0055ff",
        created_at: "2026-08-12T00:00:00Z",
      });
      expect(navigateMock).toHaveBeenCalledWith("/acme/dashboard", { replace: true });
    });
  });

  it("explains that trial starts after email verification", async () => {
    getMeMock.mockResolvedValue({ id: "u1", email: "a@example.com", email_verified: false });
    await renderPage();
    expect(await screen.findByText("Verify your email to start the 14-day trial.")).toBeInTheDocument();
  });
});
