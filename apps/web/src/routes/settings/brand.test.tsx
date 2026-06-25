// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, act } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router";
import { I18nextProvider, initReactI18next } from "react-i18next";
import i18n from "i18next";
import { SettingsBrandPage } from "./brand";
import { toast } from "sonner";

const { getWorkspaceSettingsMock, updateWorkspaceSettingsMock, uploadWorkspaceLogoMock } = vi.hoisted(
  () => ({
    getWorkspaceSettingsMock: vi.fn(),
    updateWorkspaceSettingsMock: vi.fn(),
    uploadWorkspaceLogoMock: vi.fn(),
  })
);

vi.mock("@/lib/api", () => ({
  api: {
    getWorkspaceSettings: getWorkspaceSettingsMock,
    updateWorkspaceSettings: updateWorkspaceSettingsMock,
    uploadWorkspaceLogo: uploadWorkspaceLogoMock,
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
        brandColor: "Brand color",
        viewerDomain: "Custom viewer domain",
        save: "Save brand settings",
        saving: "Saving...",
        saved: "Brand settings saved",
        hint: "Uploaded logo is saved to file storage first.",
      },
    },
    common: {
      error: {
        loadFailed: "Failed to load",
        saveFailed: "Failed to save",
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
    vi.mocked(toast.success).mockClear();
    vi.mocked(toast.error).mockClear();

    getWorkspaceSettingsMock.mockResolvedValue({
      logoUrl: "https://cdn.example.com/old-logo.png",
      brandColor: "#0f172a",
      viewerDomain: "invest.example.com",
    });
  });

  it("renders workspace brand settings", async () => {
    await renderPage();

    await waitFor(() => {
      expect(screen.getByText("Brand Customization")).toBeInTheDocument();
    });
    expect(screen.getByDisplayValue("#0f172a")).toBeInTheDocument();
    expect(screen.getByDisplayValue("invest.example.com")).toBeInTheDocument();
  });

  it("uploads a new logo and replaces the preview with the server url", async () => {
    uploadWorkspaceLogoMock.mockResolvedValue({
      data: { logoUrl: "https://cdn.example.com/new-logo.png" },
    });

    await renderPage();
    await waitFor(() => {
      expect(screen.getByRole("img", { name: /Logo/i })).toBeInTheDocument();
    });

    const fileInput = screen.getByLabelText("Upload Logo") as HTMLInputElement;
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

    const fileInput = screen.getByLabelText(/Upload Logo/i) as HTMLInputElement;
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

  it("rejects non-image files", async () => {
    await renderPage();
    await waitFor(() => {
      expect(screen.getByLabelText("Upload Logo")).toBeInTheDocument();
    });

    const fileInput = screen.getByLabelText("Upload Logo") as HTMLInputElement;
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
      expect(screen.getByDisplayValue("#0f172a")).toBeInTheDocument();
    });

    const colorInput = screen.getByDisplayValue("#0f172a");
    fireEvent.change(colorInput, { target: { value: "#3b82f6" } });

    fireEvent.click(screen.getByRole("button", { name: /Save brand settings/i }));

    await waitFor(() => {
      expect(updateWorkspaceSettingsMock).toHaveBeenCalledWith(
        expect.objectContaining({ brandColor: "#3b82f6" })
      );
    });
    expect(toast.success).toHaveBeenCalled();
  });
});
