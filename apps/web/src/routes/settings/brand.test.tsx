// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, act } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { SettingsBrandPage, isAllowedLogoFile, toColorInputValue } from "./brand";
import { toast } from "sonner";

const {
  getWorkspaceSettingsMock,
  updateWorkspaceSettingsMock,
  uploadWorkspaceLogoMock,
  getWorkspaceViewerDomainMock,
  putWorkspaceViewerDomainMock,
  verifyWorkspaceViewerDomainMock,
  deleteWorkspaceViewerDomainMock,
} = vi.hoisted(() => ({
  getWorkspaceSettingsMock: vi.fn(),
  updateWorkspaceSettingsMock: vi.fn(),
  uploadWorkspaceLogoMock: vi.fn(),
  getWorkspaceViewerDomainMock: vi.fn(),
  putWorkspaceViewerDomainMock: vi.fn(),
  verifyWorkspaceViewerDomainMock: vi.fn(),
  deleteWorkspaceViewerDomainMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getWorkspaceSettings: getWorkspaceSettingsMock,
    updateWorkspaceSettings: updateWorkspaceSettingsMock,
    uploadWorkspaceLogo: uploadWorkspaceLogoMock,
    getWorkspaceViewerDomain: getWorkspaceViewerDomainMock,
    putWorkspaceViewerDomain: putWorkspaceViewerDomainMock,
    verifyWorkspaceViewerDomain: verifyWorkspaceViewerDomainMock,
    deleteWorkspaceViewerDomain: deleteWorkspaceViewerDomainMock,
  },
}));

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

const settingsResources = {
  en: {
    settings: {
      brand: {
        title: "Brand Customization",
        logo: "Logo",
        upload: "Upload Logo",
        uploading: "Uploading...",
        uploadSuccess: "Logo uploaded",
        uploadFailed: "Logo upload failed",
        invalidType: "Please select an image file",
        tooLarge: "Logo must be smaller than 5 MB",
        noLogo: "No logo uploaded",
        brandColor: "Brand Color",
        viewerDomain: "Custom Domain",
        viewerDomainPlaceholder: "invest.yourdomain.com",
        viewerDomainHint: "Share links use this hostname only after DNS is verified.",
        viewerDomainCname: "Create a CNAME record: {{host}} → {{target}}",
        viewerDomainPending: "Waiting for DNS",
        viewerDomainVerified: "Verified",
        viewerDomainAdd: "Add domain",
        viewerDomainAdding: "Adding...",
        viewerDomainVerify: "Verify DNS",
        viewerDomainVerifying: "Verifying...",
        viewerDomainRemove: "Remove",
        viewerDomainRemoving: "Removing...",
        viewerDomainAdded: "Domain saved. Point DNS, then verify.",
        viewerDomainVerifiedSuccess: "Custom domain verified",
        viewerDomainRemoved: "Custom domain removed",
        save: "Save",
        saving: "Saving...",
        saved: "Brand settings saved",
        hint: "Uploaded logo is saved to file storage first.",
      },
    },
    common: {
      error: {
        loadFailed: "Failed to load",
        saveFailed: "Failed to save",
        deleteFailed: "Failed to delete",
      },
      retry: "Retry",
      delete: "Delete",
    },
  },
};

async function initI18n() {
  const instance = i18n.createInstance();
  await instance.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    ns: ["settings", "common"],
    defaultNS: "settings",
    resources: settingsResources,
    interpolation: { escapeValue: false },
  });
  return instance;
}

async function renderPage(path = "/acme/settings/brand") {
  const i18nInstance = await initI18n();
  let result!: ReturnType<typeof render>;
  await act(async () => {
    result = render(
      <I18nextProvider i18n={i18nInstance}>
        <MemoryRouter initialEntries={[path]}>
          <Routes>
            <Route path=":slug/settings/brand" element={<SettingsBrandPage />} />
          </Routes>
        </MemoryRouter>
      </I18nextProvider>
    );
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
  return result;
}

describe("SettingsBrandPage", () => {
  beforeEach(() => {
    getWorkspaceSettingsMock.mockReset();
    updateWorkspaceSettingsMock.mockReset();
    uploadWorkspaceLogoMock.mockReset();
    getWorkspaceViewerDomainMock.mockReset();
    putWorkspaceViewerDomainMock.mockReset();
    verifyWorkspaceViewerDomainMock.mockReset();
    deleteWorkspaceViewerDomainMock.mockReset();
    vi.mocked(toast.success).mockClear();
    vi.mocked(toast.error).mockClear();

    getWorkspaceSettingsMock.mockResolvedValue({
      logoUrl: "https://cdn.example.com/old-logo.png",
      brandColor: "#0f172a",
      viewerDomain: "invest.example.com",
    });
    getWorkspaceViewerDomainMock.mockResolvedValue({
      hostname: "invest.example.com",
      status: "verified",
      cnameHost: "invest.example.com",
      cnameTarget: "cname.dealsignal.com",
      verifiedAt: "2026-01-01T00:00:00Z",
    });
  });

  it("renders workspace brand settings", async () => {
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("Brand Customization")).toBeInTheDocument();
    });
    expect(screen.getByRole("button", { name: "Upload Logo" })).toBeInTheDocument();
    expect(screen.getByRole("img", { name: /Logo/i })).toBeInTheDocument();
    const colorInput = screen.getByLabelText("Brand Color") as HTMLInputElement;
    expect(colorInput).toHaveAttribute("type", "color");
    expect(colorInput).toHaveValue("#0f172a");
    expect(screen.getByDisplayValue("invest.example.com")).toBeInTheDocument();
    expect(screen.getByText("Verified")).toBeInTheDocument();
  });

  it("uploads a new logo and replaces the preview with the server url", async () => {
    uploadWorkspaceLogoMock.mockResolvedValue({
      logoUrl: "https://cdn.example.com/new-logo.png",
    });

    await renderPage();
    await waitFor(() => {
      expect(screen.getByRole("img", { name: /Logo/i })).toBeInTheDocument();
    });

    const fileInput = screen.getByTestId("brand-logo-input") as HTMLInputElement;
    const file = new File(["pixels"], "logo.png", { type: "image/png" });
    fireEvent.change(fileInput, { target: { files: [file] } });

    await waitFor(() => {
      expect(uploadWorkspaceLogoMock).toHaveBeenCalledWith(file);
    });
    expect(screen.getByRole("img", { name: /Logo/i })).toHaveAttribute(
      "src",
      "https://cdn.example.com/new-logo.png"
    );
    expect(toast.success).toHaveBeenCalled();
  });

  it("reverts to the previous logo when upload fails", async () => {
    uploadWorkspaceLogoMock.mockRejectedValue(new Error("upload failed"));

    await renderPage();
    await waitFor(() => {
      expect(screen.getByRole("img", { name: /Logo/i })).toBeInTheDocument();
    });

    const fileInput = screen.getByTestId("brand-logo-input") as HTMLInputElement;
    const file = new File(["pixels"], "logo.png", { type: "image/png" });
    fireEvent.change(fileInput, { target: { files: [file] } });

    await waitFor(() => {
      expect(uploadWorkspaceLogoMock).toHaveBeenCalledWith(file);
    });
    expect(screen.getByRole("img", { name: /Logo/i })).toHaveAttribute(
      "src",
      "https://cdn.example.com/old-logo.png"
    );
    expect(toast.error).toHaveBeenCalled();
  });

  it("accepts svg logos", async () => {
    uploadWorkspaceLogoMock.mockResolvedValue({
      logoUrl: "https://cdn.example.com/logo.svg",
    });
    await renderPage();
    await waitFor(() => {
      expect(screen.getByTestId("brand-logo-input")).toBeInTheDocument();
    });
    const fileInput = screen.getByTestId("brand-logo-input") as HTMLInputElement;
    const file = new File(["<svg xmlns='http://www.w3.org/2000/svg'></svg>"], "logo.svg", {
      type: "image/svg+xml",
    });
    fireEvent.change(fileInput, { target: { files: [file] } });
    await waitFor(() => {
      expect(uploadWorkspaceLogoMock).toHaveBeenCalledWith(file);
    });
  });

  it("rejects non-image files", async () => {
    await renderPage();
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Upload Logo" })).toBeInTheDocument();
    });

    const fileInput = screen.getByTestId("brand-logo-input") as HTMLInputElement;
    const file = new File(["text"], "readme.txt", { type: "text/plain" });
    fireEvent.change(fileInput, { target: { files: [file] } });

    expect(uploadWorkspaceLogoMock).not.toHaveBeenCalled();
    expect(toast.error).toHaveBeenCalled();
  });

  it("saves brand settings", async () => {
    updateWorkspaceSettingsMock.mockResolvedValue({
      logoUrl: "https://cdn.example.com/old-logo.png",
      brandColor: "#3b82f6",
      viewerDomain: "view.example.com",
    });

    await renderPage();
    await waitFor(() => {
      expect(screen.getByLabelText("Brand Color")).toHaveValue("#0f172a");
    });

    const colorInput = screen.getByLabelText("Brand Color");
    fireEvent.change(colorInput, { target: { value: "#3b82f6" } });

    fireEvent.click(screen.getByRole("button", { name: /^Save$/i }));

    await waitFor(() => {
      expect(updateWorkspaceSettingsMock).toHaveBeenCalledWith(
        expect.objectContaining({ brandColor: "#3b82f6" })
      );
    });
    expect(toast.success).toHaveBeenCalled();
  });

  it("shows an empty logo tile that is the upload control", async () => {
    getWorkspaceSettingsMock.mockResolvedValue({
      logoUrl: "",
      brandColor: "#0055ff",
      viewerDomain: "",
    });
    getWorkspaceViewerDomainMock.mockResolvedValue({
      hostname: "",
      status: "",
      cnameHost: "",
      cnameTarget: "cname.dealsignal.com",
    });
    await renderPage();
    await waitFor(() => {
      expect(screen.getByText("No logo uploaded")).toBeInTheDocument();
    });
    expect(screen.queryByRole("img", { name: /Logo/i })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Upload Logo" })).toBeInTheDocument();
  });

  it("treats svg as an allowed logo type even without a mime type", () => {
    expect(isAllowedLogoFile(new File(["<svg />"], "logo.svg", { type: "" }))).toBe(true);
    expect(isAllowedLogoFile(new File(["x"], "readme.txt", { type: "text/plain" }))).toBe(false);
  });

  it("adds a pending viewer domain without saving brand settings", async () => {
    getWorkspaceSettingsMock.mockResolvedValue({
      logoUrl: "",
      brandColor: "#0055ff",
      viewerDomain: "",
    });
    getWorkspaceViewerDomainMock.mockResolvedValue({
      hostname: "",
      status: "",
      cnameHost: "",
      cnameTarget: "cname.dealsignal.com",
    });
    putWorkspaceViewerDomainMock.mockResolvedValue({
      hostname: "view.example.com",
      status: "pending",
      cnameHost: "view.example.com",
      cnameTarget: "cname.dealsignal.com",
    });

    await renderPage();
    await waitFor(() => {
      expect(screen.getByPlaceholderText("invest.yourdomain.com")).toBeInTheDocument();
    });
    fireEvent.change(screen.getByPlaceholderText("invest.yourdomain.com"), {
      target: { value: "view.example.com" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Add domain" }));

    await waitFor(() => {
      expect(putWorkspaceViewerDomainMock).toHaveBeenCalledWith("view.example.com");
    });
    expect(updateWorkspaceSettingsMock).not.toHaveBeenCalled();
    expect(screen.getByDisplayValue("view.example.com")).toBeInTheDocument();
    expect(screen.getByText("Waiting for DNS")).toBeInTheDocument();
    expect(screen.getByText(/Create a CNAME record/)).toBeInTheDocument();
  });

  it("normalizes stored hex for the native color input", () => {
    expect(toColorInputValue("#0055FF")).toBe("#0055ff");
    expect(toColorInputValue("3366ff")).toBe("#3366ff");
    expect(toColorInputValue("#0f1")).toBe("#00ff11");
    expect(toColorInputValue("")).toBe("#0055ff");
    expect(toColorInputValue("not-a-color")).toBe("#0055ff");
  });
});
