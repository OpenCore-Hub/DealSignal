// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, act, fireEvent } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter, Routes, Route, createMemoryRouter, RouterProvider } from "react-router";
import { PublicViewerPage } from "./PublicViewerPage";
import { createTestI18n } from "@/i18n/test-utils";
import { ApiError } from "@/lib/apiClient";

const { accessPublicLinkMock, getPublicNDAPreviewMock, requestPublicLinkAccessMock, checkPublicLinkEmailMock } = vi.hoisted(() => ({
  accessPublicLinkMock: vi.fn(),
  getPublicNDAPreviewMock: vi.fn(),
  requestPublicLinkAccessMock: vi.fn(),
  checkPublicLinkEmailMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    accessPublicLink: accessPublicLinkMock,
    getPublicNDAPreview: getPublicNDAPreviewMock,
    requestPublicLinkAccess: requestPublicLinkAccessMock,
    checkPublicLinkEmail: checkPublicLinkEmailMock,
  },
}));

function installMemorySessionStorage() {
  const store = new Map<string, string>();
  const memory = {
    getItem: (key: string) => store.get(key) ?? null,
    setItem: (key: string, value: string) => {
      store.set(key, String(value));
    },
    removeItem: (key: string) => {
      store.delete(key);
    },
    clear: () => store.clear(),
    get length() {
      return store.size;
    },
    key: (index: number) => [...store.keys()][index] ?? null,
  };
  Object.defineProperty(window, "sessionStorage", {
    configurable: true,
    writable: true,
    value: memory,
  });
  return store;
}

const successAccess = {
  link: {
    id: "link-1",
    permissionType: "email",
    downloadEnabled: false,
    watermarkEnabled: false,
    aiCopilotEnabled: false,
    qaEnabled: false,
    fileRequestsEnabled: false,
    isBundle: false,
  },
  documents: [{ id: "doc-1", title: "Deck", pageCount: 1, sourceType: "pdf" }],
  visitorId: "v1",
  requiresEmail: false,
  requiresEmailVerification: true,
  requiresPassword: false,
  requiresNda: false,
  sessionToken: "session-after-access",
};

async function renderPage(token: string, i18n?: Awaited<ReturnType<typeof createTestI18n>>) {
  const i18nInstance = i18n ?? (await createTestI18n());
  const view = render(
    <MemoryRouter initialEntries={[`/l/${token}`]}>
      <I18nextProvider i18n={i18nInstance}>
        <Routes>
          <Route path="/l/:token" element={<PublicViewerPage />} />
        </Routes>
      </I18nextProvider>
    </MemoryRouter>
  );
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
  return { ...view, i18n: i18nInstance };
}

describe("PublicViewerPage", () => {
  beforeEach(() => {
    vi.useRealTimers();
    accessPublicLinkMock.mockReset();
    getPublicNDAPreviewMock.mockReset();
    requestPublicLinkAccessMock.mockReset();
    checkPublicLinkEmailMock.mockReset();
    getPublicNDAPreviewMock.mockResolvedValue({
      ndaTemplate: {
        id: "tpl-1",
        name: "NDA",
        requireSignerName: true,
        sourceDocumentId: "doc-nda",
        contentSha256: "hash-1",
      },
      document: { id: "doc-nda", title: "NDA", pageCount: 1, sourceType: "pdf" },
      previewImageUrl: "https://example.test/nda-page-1.png",
      previewPageUrls: ["https://example.test/nda-page-1.png"],
      documentUrl: "https://example.test/nda.pdf",
      previewUrl: "https://example.test/nda-page-1.png",
      expiresAt: "2099-01-01T00:00:00Z",
    });
    checkPublicLinkEmailMock.mockResolvedValue({ ok: true });
    installMemorySessionStorage();
  });

  it("shows the backend error message instead of generic load failed for unknown error codes", async () => {
    accessPublicLinkMock.mockRejectedValue(
      new ApiError({
        status: 500,
        code: "internal_error",
        message: "server exploded",
        requestId: "req-internal",
      })
    );

    await renderPage("internal-token");

    await waitFor(() => {
      expect(screen.getByText("server exploded")).toBeInTheDocument();
    });
    expect(screen.getByText("viewer.gateTitle")).toBeInTheDocument();
    expect(screen.queryByText("common:error.loadFailed")).not.toBeInTheDocument();
  });

  it("renders only access code input for modern document link verification (requiresEmail=false)", async () => {
    accessPublicLinkMock.mockRejectedValue(
      new ApiError({
        status: 403,
        code: "requires_email_code",
        message: "email code required",
        requestId: "req-1",
        requiresEmail: false,
        requiresEmailVerification: true,
        requiresPassword: false,
        requiresNda: false,
        isDealRoom: false,
      })
    );

    await renderPage("modern-token");

    await waitFor(() => {
      expect(document.getElementById("email-code")).toBeInTheDocument();
    });

    expect(document.getElementById("email")).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/password/i)).not.toBeInTheDocument();
  });

  it("renders email gate without error on first visit when email is required", async () => {
    accessPublicLinkMock.mockRejectedValue(
      new ApiError({
        status: 403,
        code: "requires_email",
        message: "email required",
        requestId: "req-email",
        requiresEmail: true,
        requiresEmailVerification: false,
        requiresPassword: false,
        requiresNda: false,
        isDealRoom: false,
      })
    );

    await renderPage("requires-email-token");

    await waitFor(() => {
      expect(document.getElementById("email")).toBeInTheDocument();
    });

    expect(screen.getByText("viewer.continue")).toBeInTheDocument();
    expect(screen.queryByText("email required")).not.toBeInTheDocument();
    expect(screen.queryByText("viewer.emailNotAllowed")).not.toBeInTheDocument();
    expect(screen.queryByText("retry")).not.toBeInTheDocument();
  });

  it("renders only email gate when email is not allowed", async () => {
    accessPublicLinkMock.mockRejectedValue(
      new ApiError({
        status: 403,
        code: "not_allowed",
        message: "email is not allowed",
        requestId: "req-4",
        requiresEmail: true,
        requiresEmailVerification: false,
        requiresPassword: false,
        requiresNda: false,
        isDealRoom: false,
      })
    );

    await renderPage("restricted-token");

    await waitFor(() => {
      expect(document.getElementById("email")).toBeInTheDocument();
    });

    expect(screen.getByText("viewer.emailNotAllowed")).toBeInTheDocument();
    expect(screen.getByText("retry")).toBeInTheDocument();
    expect(screen.queryByText("email is not allowed")).not.toBeInTheDocument();
  });

  it("renders only email verification code for deal-room verification (no visitor send-code UI)", async () => {
    accessPublicLinkMock.mockRejectedValue(
      new ApiError({
        status: 403,
        code: "requires_email_code",
        message: "email code required",
        requestId: "req-2",
        requiresEmail: false,
        requiresEmailVerification: true,
        requiresPassword: false,
        requiresNda: false,
        isDealRoom: true,
      })
    );

    await renderPage("dealroom-token");

    await waitFor(() => {
      expect(document.getElementById("email-code")).toBeInTheDocument();
    });
    expect(document.getElementById("email")).not.toBeInTheDocument();
    expect(screen.queryByText("viewer.sendCode")).not.toBeInTheDocument();
    expect(screen.getByText("viewer.codeLabel")).toBeInTheDocument();
  });

  it("defaults NDA checkbox to unchecked when NDA is required", async () => {
    accessPublicLinkMock.mockRejectedValue(
      new ApiError({
        status: 403,
        code: "nda_required",
        message: "nda required",
        requestId: "req-nda",
        requiresEmail: false,
        requiresEmailVerification: false,
        requiresPassword: false,
        requiresNda: true,
        isDealRoom: false,
      })
    );

    await renderPage("nda-token");

    await waitFor(() => {
      expect(document.getElementById("nda")).toBeInTheDocument();
    });
    expect(document.getElementById("nda")).not.toBeChecked();
    expect(screen.queryByText("nda required")).not.toBeInTheDocument();
  });

  it("scrolls NDA preview and opens zoom dialog on click", async () => {
    accessPublicLinkMock.mockRejectedValue(
      new ApiError({
        status: 403,
        code: "nda_required",
        message: "nda required",
        requestId: "req-nda-preview",
        requiresEmail: false,
        requiresEmailVerification: false,
        requiresPassword: false,
        requiresNda: true,
        isDealRoom: false,
      })
    );
    getPublicNDAPreviewMock.mockResolvedValue({
      ndaTemplate: {
        id: "tpl-1",
        name: "练习册.pdf",
        requireSignerName: true,
        sourceDocumentId: "doc-nda",
        contentSha256: "hash-1",
      },
      document: { id: "doc-nda", title: "练习册.pdf", pageCount: 2, sourceType: "pdf" },
      previewImageUrl: "https://example.test/nda-page-1.png",
      previewPageUrls: [
        "https://example.test/nda-page-1.png",
        "https://example.test/nda-page-2.png",
      ],
      documentUrl: "https://example.test/nda.pdf",
      previewUrl: "https://example.test/nda-page-1.png",
      expiresAt: "2099-01-01T00:00:00Z",
    });

    await renderPage("nda-preview-token");

    const preview = await waitFor(() =>
      screen.getByRole("button", { name: "viewer.ndaPreviewZoomHint" })
    );
    expect(preview.className).toMatch(/overflow-y-auto/);
    fireEvent.click(preview);
    await waitFor(() => {
      expect(screen.getByRole("dialog")).toBeInTheDocument();
    });
    expect(screen.getAllByRole("img").length).toBeGreaterThanOrEqual(2);
  });

  it("disables Continue until NDA email, name and agreement are complete", async () => {
    accessPublicLinkMock.mockRejectedValue(
      new ApiError({
        status: 403,
        code: "nda_required",
        message: "nda required",
        requestId: "req-nda-disabled",
        requiresEmail: false,
        requiresEmailVerification: true,
        requiresPassword: false,
        requiresNda: true,
        isDealRoom: false,
      })
    );

    await renderPage("nda-disabled-token");

    await waitFor(() => {
      expect(document.getElementById("nda")).toBeInTheDocument();
    });
    const continueBtn = screen.getByRole("button", { name: "viewer.continue" });
    expect(continueBtn).toBeDisabled();
    expect(document.getElementById("email-code")).not.toBeInTheDocument();

    fireEvent.change(document.getElementById("signer-name")!, { target: { value: "Alice Zhang" } });
    expect(continueBtn).toBeDisabled();

    fireEvent.click(document.getElementById("nda")!);
    expect(continueBtn).toBeDisabled();

    fireEvent.change(document.getElementById("nda-delivery-email")!, {
      target: { value: "alice@example.com" },
    });
    expect(continueBtn).not.toBeDisabled();
  });

  it("shows signed NDA review countdown then a clean email-code page", async () => {
    accessPublicLinkMock.mockRejectedValue(
      new ApiError({
        status: 403,
        code: "nda_required",
        message: "nda required",
        requestId: "req-nda-review",
        requiresEmail: false,
        requiresEmailVerification: true,
        requiresPassword: false,
        requiresNda: true,
        isDealRoom: false,
      })
    );
    getPublicNDAPreviewMock.mockResolvedValue({
      ndaTemplate: {
        id: "tpl-1",
        name: "练习册.pdf",
        requireSignerName: true,
        sourceDocumentId: "doc-nda",
        contentSha256: "hash-1",
      },
      document: { id: "doc-nda", title: "练习册.pdf", pageCount: 2, sourceType: "pdf" },
      previewImageUrl: "https://example.test/nda-page-1.png",
      previewPageUrls: [
        "https://example.test/nda-page-1.png",
        "https://example.test/nda-page-2.png",
      ],
      documentUrl: "https://example.test/nda.pdf",
      previewUrl: "https://example.test/nda-page-1.png",
      expiresAt: "2099-01-01T00:00:00Z",
    });

    await renderPage("nda-review-token");

    await waitFor(() => {
      expect(document.getElementById("nda")).toBeInTheDocument();
    });

    fireEvent.change(document.getElementById("nda-delivery-email")!, {
      target: { value: "alice@example.com" },
    });
    fireEvent.change(document.getElementById("signer-name")!, { target: { value: "Alice Zhang" } });
    fireEvent.click(document.getElementById("nda")!);

    vi.useFakeTimers();
    try {
      fireEvent.click(screen.getByRole("button", { name: "viewer.continue" }));
      await act(async () => {
        await Promise.resolve();
        await Promise.resolve();
      });
      expect(checkPublicLinkEmailMock).toHaveBeenCalled();
      expect(screen.getByText("viewer.ndaReviewTitle")).toBeInTheDocument();
      expect(screen.getByText("viewer.ndaAuditTrailTitle")).toBeInTheDocument();
      expect(document.getElementById("email-code")).not.toBeInTheDocument();
      expect(document.getElementById("signer-name")).not.toBeInTheDocument();
      expect(screen.getByText("Alice Zhang")).toBeInTheDocument();
      expect(screen.getByText("alice@example.com")).toBeInTheDocument();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(30_000);
      });

      expect(document.getElementById("email-code")).toBeInTheDocument();
      expect(document.getElementById("signer-name")).not.toBeInTheDocument();
      expect(screen.queryByText("viewer.ndaReviewTitle")).not.toBeInTheDocument();
      expect(accessPublicLinkMock).toHaveBeenCalledTimes(1);
    } finally {
      vi.useRealTimers();
    }
  });

  it("checks email before NDA review and stays on sign with retry + authorization when not allowed", async () => {
    accessPublicLinkMock.mockRejectedValue(
      new ApiError({
        status: 403,
        code: "nda_required",
        message: "nda required",
        requestId: "req-check-boot",
        requiresEmail: false,
        requiresEmailVerification: true,
        requiresPassword: false,
        requiresNda: true,
        isDealRoom: false,
      })
    );
    checkPublicLinkEmailMock.mockRejectedValue(
      new ApiError({
        status: 403,
        code: "not_allowed",
        message: "email is not allowed",
        requestId: "req-check-deny",
        requiresEmail: false,
        requiresEmailVerification: true,
        requiresPassword: false,
        requiresNda: true,
        isDealRoom: false,
      })
    );

    await renderPage("nda-check-email-token");

    await waitFor(() => {
      expect(document.getElementById("nda")).toBeInTheDocument();
    });
    fireEvent.change(document.getElementById("nda-delivery-email")!, {
      target: { value: "partner@example.com" },
    });
    fireEvent.change(document.getElementById("signer-name")!, { target: { value: "Partner User" } });
    fireEvent.click(document.getElementById("nda")!);
    fireEvent.click(screen.getByRole("button", { name: "viewer.continue" }));

    await waitFor(() => {
      expect(checkPublicLinkEmailMock).toHaveBeenCalledWith("nda-check-email-token", "partner@example.com");
    });
    await waitFor(() => {
      expect(screen.getByText("viewer.emailNotAuthorized")).toBeInTheDocument();
    });
    expect(screen.getByText("viewer.ndaSignTitle")).toBeInTheDocument();
    expect(screen.queryByText("viewer.ndaReviewTitle")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "retry" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "viewer.requestAuthorization" })).toBeInTheDocument();
    // Must not fall back to the initial Continue CTA while still denied.
    expect(screen.queryByRole("button", { name: "viewer.continue" })).not.toBeInTheDocument();
  });

  it("retry after not_allowed re-checks email without resetting to Continue", async () => {
    accessPublicLinkMock.mockRejectedValue(
      new ApiError({
        status: 403,
        code: "nda_required",
        message: "nda required",
        requestId: "req-retry-check-boot",
        requiresEmail: false,
        requiresEmailVerification: true,
        requiresPassword: false,
        requiresNda: true,
        isDealRoom: false,
      })
    );
    checkPublicLinkEmailMock
      .mockRejectedValueOnce(
        new ApiError({
          status: 403,
          code: "not_allowed",
          message: "email is not allowed",
          requestId: "req-retry-check-deny",
          requiresEmail: false,
          requiresEmailVerification: true,
          requiresPassword: false,
          requiresNda: true,
          isDealRoom: false,
        })
      )
      .mockResolvedValueOnce({ ok: true });

    await renderPage("nda-retry-check-token");

    await waitFor(() => {
      expect(document.getElementById("nda")).toBeInTheDocument();
    });
    fireEvent.change(document.getElementById("nda-delivery-email")!, {
      target: { value: "wrong@example.com" },
    });
    fireEvent.change(document.getElementById("signer-name")!, { target: { value: "Partner User" } });
    fireEvent.click(document.getElementById("nda")!);
    fireEvent.click(screen.getByRole("button", { name: "viewer.continue" }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "retry" })).toBeInTheDocument();
    });
    expect(screen.queryByRole("button", { name: "viewer.continue" })).not.toBeInTheDocument();

    fireEvent.change(document.getElementById("nda-delivery-email")!, {
      target: { value: "alice@example.com" },
    });
    fireEvent.click(screen.getByRole("button", { name: "retry" }));

    await waitFor(() => {
      expect(checkPublicLinkEmailMock).toHaveBeenLastCalledWith("nda-retry-check-token", "alice@example.com");
    });
    await waitFor(() => {
      expect(screen.getByText("viewer.ndaReviewTitle")).toBeInTheDocument();
    });
  });

  it("enters NDA review only after email check passes", async () => {
    accessPublicLinkMock.mockRejectedValue(
      new ApiError({
        status: 403,
        code: "nda_required",
        message: "nda required",
        requestId: "req-review-boot",
        requiresEmail: false,
        requiresEmailVerification: true,
        requiresPassword: false,
        requiresNda: true,
        isDealRoom: false,
      })
    );
    checkPublicLinkEmailMock.mockResolvedValue({ ok: true });

    await renderPage("nda-check-pass-token");

    await waitFor(() => {
      expect(document.getElementById("nda")).toBeInTheDocument();
    });
    fireEvent.change(document.getElementById("nda-delivery-email")!, {
      target: { value: "alice@example.com" },
    });
    fireEvent.change(document.getElementById("signer-name")!, { target: { value: "Alice" } });
    fireEvent.click(document.getElementById("nda")!);
    fireEvent.click(screen.getByRole("button", { name: "viewer.continue" }));

    await waitFor(() => {
      expect(checkPublicLinkEmailMock).toHaveBeenCalledWith("nda-check-pass-token", "alice@example.com");
    });
    await waitFor(() => {
      expect(screen.getByText("viewer.ndaReviewTitle")).toBeInTheDocument();
    });
  });

  it("on email_mismatch shows friendly tip with retry and authorization request (no email leak)", async () => {
    accessPublicLinkMock.mockImplementation(async (_tok: string, opts?: { emailCode?: string }) => {
      if (opts?.emailCode) {
        throw new ApiError({
          status: 403,
          code: "email_mismatch",
          message: "delivery email does not match verified email",
          requestId: "req-mismatch",
          requiresEmail: false,
          requiresEmailVerification: true,
          requiresPassword: false,
          requiresNda: true,
          isDealRoom: false,
        });
      }
      throw new ApiError({
        status: 403,
        code: "nda_required",
        message: "nda required",
        requestId: "req-mismatch-boot",
        requiresEmail: false,
        requiresEmailVerification: true,
        requiresPassword: false,
        requiresNda: true,
        isDealRoom: false,
      });
    });
    requestPublicLinkAccessMock.mockResolvedValue({
      id: "ar-1",
      email: "partner@example.com",
      status: "pending",
    });

    await renderPage("nda-mismatch-token");

    await waitFor(() => {
      expect(document.getElementById("nda")).toBeInTheDocument();
    });

    fireEvent.change(document.getElementById("nda-delivery-email")!, {
      target: { value: "partner@example.com" },
    });
    fireEvent.change(document.getElementById("signer-name")!, { target: { value: "Partner User" } });
    fireEvent.click(document.getElementById("nda")!);

    vi.useFakeTimers();
    try {
      fireEvent.click(screen.getByRole("button", { name: "viewer.continue" }));
      await act(async () => {
        await Promise.resolve();
        await Promise.resolve();
      });
      expect(screen.getByText("viewer.ndaReviewTitle")).toBeInTheDocument();
      await act(async () => {
        await vi.advanceTimersByTimeAsync(30_000);
      });
    } finally {
      vi.useRealTimers();
    }

    await waitFor(() => {
      expect(document.getElementById("email-code")).toBeInTheDocument();
    });
    fireEvent.change(document.getElementById("email-code")!, { target: { value: "123456" } });
    fireEvent.click(screen.getByRole("button", { name: "viewer.continue" }));

    await waitFor(() => {
      expect(screen.getByText("viewer.emailMismatch")).toBeInTheDocument();
    });
    expect(screen.queryByText(/alice@example.com/i)).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "retry" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "viewer.requestAuthorization" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "viewer.editDeliveryEmail" })).not.toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "viewer.requestAuthorization" })
    );

    await waitFor(() => {
      expect(screen.getByText("viewer.accessRequestTitle")).toBeInTheDocument();
    });
    expect(document.getElementById("access-request-email")).toHaveValue("partner@example.com");
    expect(document.getElementById("access-request-email")).toHaveAttribute("readonly");

    fireEvent.change(document.getElementById("access-request-reason")!, {
      target: { value: "Partner needs access" },
    });
    fireEvent.click(screen.getByRole("button", { name: "viewer.accessRequestSubmit" }));

    await waitFor(() => {
      expect(requestPublicLinkAccessMock).toHaveBeenCalledWith(
        "nda-mismatch-token",
        expect.objectContaining({
          email: "partner@example.com",
          reason: "Partner needs access",
          signerName: "Partner User",
        })
      );
    });
    expect(screen.getByText("viewer.accessRequestSubmittedTitle")).toBeInTheDocument();
    const stored = JSON.parse(window.sessionStorage.getItem("nda-intent:nda-mismatch-token")!);
    expect(stored.accessRequestPending).toBe(true);
    expect(stored.phase).toBe("credentials");
  });

  it("refresh while access request pending restores submitted UI (not verification code)", async () => {
    const token = "nda-pending-refresh-token";
    window.sessionStorage.setItem(
      `nda-intent:${token}`,
      JSON.stringify({
        signerName: "Partner User",
        ndaDeliveryEmail: "partner@example.com",
        ndaAgreed: true,
        ndaTemplateId: "tpl-1",
        contentSha256: "hash-1",
        phase: "credentials",
        accessRequestPending: true,
      })
    );
    checkPublicLinkEmailMock.mockRejectedValue(
      new ApiError({
        status: 403,
        code: "not_allowed",
        message: "email is not allowed",
        requestId: "req-pending-still",
        requiresEmail: false,
        requiresEmailVerification: true,
        requiresPassword: false,
        requiresNda: true,
        isDealRoom: false,
      })
    );
    accessPublicLinkMock.mockRejectedValue(
      new ApiError({
        status: 403,
        code: "requires_email_code",
        message: "should not reach access",
        requestId: "req-pending-should-not",
        requiresEmail: false,
        requiresEmailVerification: true,
        requiresPassword: false,
        requiresNda: true,
        isDealRoom: false,
      })
    );

    await renderPage(token);

    await waitFor(() => {
      expect(screen.getByText("viewer.accessRequestSubmittedTitle")).toBeInTheDocument();
    });
    expect(checkPublicLinkEmailMock).toHaveBeenCalledWith(token, "partner@example.com");
    expect(accessPublicLinkMock).not.toHaveBeenCalled();
    expect(document.getElementById("email-code")).not.toBeInTheDocument();
    expect(screen.queryByText("viewer.gateTitle")).not.toBeInTheDocument();
  });

  it("refresh after access request approved opens credentials for new verification code", async () => {
    const token = "nda-approved-refresh-token";
    window.sessionStorage.setItem(
      `nda-intent:${token}`,
      JSON.stringify({
        signerName: "Partner User",
        ndaDeliveryEmail: "partner@example.com",
        ndaAgreed: true,
        ndaTemplateId: "tpl-1",
        contentSha256: "hash-1",
        phase: "credentials",
        accessRequestPending: true,
      })
    );
    checkPublicLinkEmailMock.mockResolvedValue({ ok: true });
    accessPublicLinkMock.mockRejectedValue(
      new ApiError({
        status: 403,
        code: "requires_email_code",
        message: "email code required",
        requestId: "req-approved-code",
        requiresEmail: false,
        requiresEmailVerification: true,
        requiresPassword: false,
        requiresNda: true,
        isDealRoom: false,
      })
    );

    await renderPage(token);

    await waitFor(() => {
      expect(document.getElementById("email-code")).toBeInTheDocument();
    });
    expect(checkPublicLinkEmailMock).toHaveBeenCalledWith(token, "partner@example.com");
    expect(accessPublicLinkMock).toHaveBeenCalled();
    expect(screen.getByText("viewer.gateTitle")).toBeInTheDocument();
    expect(screen.queryByText("viewer.accessRequestSubmittedTitle")).not.toBeInTheDocument();
    const stored = JSON.parse(window.sessionStorage.getItem(`nda-intent:${token}`)!);
    expect(stored.accessRequestPending).toBeUndefined();
  });

  it("restores submitted UI if Access still denies after allowlist probe", async () => {
    const token = "nda-probe-deny-token";
    window.sessionStorage.setItem(
      `nda-intent:${token}`,
      JSON.stringify({
        signerName: "Partner User",
        ndaDeliveryEmail: "partner@example.com",
        ndaAgreed: true,
        ndaTemplateId: "tpl-1",
        contentSha256: "hash-1",
        phase: "credentials",
        accessRequestPending: true,
      })
    );
    checkPublicLinkEmailMock.mockResolvedValue({ ok: true });
    accessPublicLinkMock.mockRejectedValue(
      new ApiError({
        status: 403,
        code: "not_allowed",
        message: "email is not allowed",
        requestId: "req-probe-deny",
        requiresEmail: false,
        requiresEmailVerification: true,
        requiresPassword: false,
        requiresNda: true,
        isDealRoom: false,
      })
    );

    await renderPage(token);

    await waitFor(() => {
      expect(accessPublicLinkMock).toHaveBeenCalled();
    });
    await waitFor(() => {
      expect(screen.getByText("viewer.accessRequestSubmittedTitle")).toBeInTheDocument();
    });
    expect(document.getElementById("email-code")).not.toBeInTheDocument();
    const stored = JSON.parse(window.sessionStorage.getItem(`nda-intent:${token}`)!);
    expect(stored.accessRequestPending).toBe(true);
  });

  it("email_mismatch retry returns to NDA sign so delivery email is editable", async () => {
    accessPublicLinkMock.mockImplementation(async (_tok: string, opts?: { emailCode?: string }) => {
      if (opts?.emailCode) {
        throw new ApiError({
          status: 403,
          code: "email_mismatch",
          message: "delivery email does not match verified email",
          requestId: "req-retry-mismatch",
          requiresEmail: false,
          requiresEmailVerification: true,
          requiresPassword: false,
          requiresNda: true,
          isDealRoom: false,
        });
      }
      throw new ApiError({
        status: 403,
        code: "nda_required",
        message: "nda required",
        requestId: "req-retry-boot",
        requiresEmail: false,
        requiresEmailVerification: true,
        requiresPassword: false,
        requiresNda: true,
        isDealRoom: false,
      });
    });
    checkPublicLinkEmailMock.mockResolvedValue({ ok: true });

    await renderPage("nda-mismatch-retry-token");

    await waitFor(() => {
      expect(document.getElementById("nda")).toBeInTheDocument();
    });
    fireEvent.change(document.getElementById("nda-delivery-email")!, {
      target: { value: "partner@example.com" },
    });
    fireEvent.change(document.getElementById("signer-name")!, { target: { value: "Partner User" } });
    fireEvent.click(document.getElementById("nda")!);

    vi.useFakeTimers();
    try {
      fireEvent.click(screen.getByRole("button", { name: "viewer.continue" }));
      await act(async () => {
        await Promise.resolve();
        await Promise.resolve();
      });
      expect(screen.getByText("viewer.ndaReviewTitle")).toBeInTheDocument();
      await act(async () => {
        await vi.advanceTimersByTimeAsync(30_000);
      });
    } finally {
      vi.useRealTimers();
    }

    await waitFor(() => {
      expect(document.getElementById("email-code")).toBeInTheDocument();
    });
    fireEvent.change(document.getElementById("email-code")!, { target: { value: "123456" } });
    fireEvent.click(screen.getByRole("button", { name: "viewer.continue" }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "retry" })).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole("button", { name: "retry" }));

    await waitFor(() => {
      expect(screen.getByText("viewer.ndaSignTitle")).toBeInTheDocument();
    });
    expect(document.getElementById("nda-delivery-email")).toBeInTheDocument();
    expect(document.getElementById("email-code")).not.toBeInTheDocument();
  });

  it("restores NDA intent into credentials phase on refresh (not sign)", async () => {
    const token = "nda-intent-refresh-token";
    window.sessionStorage.setItem(
      `nda-intent:${token}`,
      JSON.stringify({
        signerName: "Partner User",
        ndaDeliveryEmail: "partner@example.com",
        ndaAgreed: true,
        ndaTemplateId: "tpl-1",
        contentSha256: "hash-1",
        phase: "credentials",
      })
    );
    accessPublicLinkMock.mockRejectedValue(
      new ApiError({
        status: 403,
        code: "requires_email_code",
        message: "email code required",
        requestId: "req-intent-refresh",
        requiresEmail: false,
        requiresEmailVerification: true,
        requiresPassword: false,
        requiresNda: true,
        isDealRoom: false,
      })
    );

    await renderPage(token);

    await waitFor(() => {
      expect(document.getElementById("email-code")).toBeInTheDocument();
    });
    expect(document.getElementById("signer-name")).not.toBeInTheDocument();
    expect(screen.queryByText("viewer.ndaSignTitle")).not.toBeInTheDocument();
    expect(screen.getByText("viewer.gateTitle")).toBeInTheDocument();
  });

  it("reuses persisted session on refresh and does not re-prompt after tryAccess identity changes", async () => {
    const token = "session-refresh-token";
    window.sessionStorage.setItem(`link-session:${token}`, "stored-session");

    accessPublicLinkMock.mockImplementation(async (_tok: string, opts?: { sessionToken?: string }) => {
      if (opts?.sessionToken === "stored-session" || opts?.sessionToken === "session-after-access") {
        return successAccess;
      }
      throw new ApiError({
        status: 403,
        code: "requires_email_code",
        message: "email code required",
        requestId: "req-gate",
        requiresEmail: false,
        requiresEmailVerification: true,
        requiresPassword: false,
        requiresNda: false,
        isDealRoom: true,
      });
    });

    const { i18n } = await renderPage(token);

    await waitFor(() => {
      expect(accessPublicLinkMock).toHaveBeenCalledWith(
        token,
        expect.objectContaining({ sessionToken: "stored-session" })
      );
    });

    // Simulate i18n/t identity churn (common after hydration / language ready).
    await act(async () => {
      await i18n.changeLanguage("zh-CN");
      await new Promise((resolve) => setTimeout(resolve, 0));
    });

    const callsWithoutSession = accessPublicLinkMock.mock.calls.filter(
      ([, opts]) => !opts?.sessionToken
    );
    expect(callsWithoutSession).toHaveLength(0);
    expect(document.getElementById("email-code")).not.toBeInTheDocument();
  });

  it("does not return to NDA sign page on refresh when session reuses for an NDA link", async () => {
    const token = "nda-session-refresh-token";
    window.sessionStorage.setItem(`link-session:${token}`, "stored-session");
    window.sessionStorage.setItem(
      `nda-intent:${token}`,
      JSON.stringify({
        signerName: "Alice Zhang",
        ndaDeliveryEmail: "alice@example.com",
        ndaAgreed: true,
        ndaTemplateId: "tpl-1",
        contentSha256: "hash-1",
        phase: "complete",
      })
    );

    accessPublicLinkMock.mockImplementation(async (_tok: string, opts?: { sessionToken?: string }) => {
      if (opts?.sessionToken === "stored-session") {
        return {
          ...successAccess,
          requiresEmailVerification: false,
          requiresNda: true,
          sessionToken: "session-after-refresh",
        };
      }
      throw new ApiError({
        status: 403,
        code: "nda_required",
        message: "nda required",
        requestId: "req-nda",
        requiresEmail: false,
        requiresEmailVerification: false,
        requiresPassword: false,
        requiresNda: true,
        isDealRoom: false,
      });
    });

    await renderPage(token);

    await waitFor(() => {
      expect(accessPublicLinkMock).toHaveBeenCalledWith(
        token,
        expect.objectContaining({ sessionToken: "stored-session" })
      );
    });
    expect(screen.queryByText("viewer.ndaSignTitle")).not.toBeInTheDocument();
    expect(document.getElementById("signer-name")).not.toBeInTheDocument();
    expect(window.sessionStorage.getItem(`link-session:${token}`)).toBe("session-after-refresh");
  });

  it("skips NDA sign page on refresh when session is gone but complete intent remains", async () => {
    const token = "nda-complete-intent-token";
    window.sessionStorage.setItem(
      `nda-intent:${token}`,
      JSON.stringify({
        signerName: "Alice Zhang",
        ndaDeliveryEmail: "alice@example.com",
        ndaAgreed: true,
        ndaTemplateId: "tpl-1",
        contentSha256: "hash-1",
        phase: "complete",
      })
    );

    accessPublicLinkMock.mockImplementation(async (_tok: string, opts?: { sessionToken?: string; ndaAgreed?: boolean }) => {
      if (opts?.sessionToken) {
        throw new ApiError({
          status: 403,
          code: "nda_required",
          message: "nda required",
          requestId: "req-session-dead",
          requiresEmail: false,
          requiresEmailVerification: false,
          requiresPassword: false,
          requiresNda: true,
          isDealRoom: false,
        });
      }
      if (opts?.ndaAgreed) {
        return {
          ...successAccess,
          requiresEmailVerification: false,
          requiresNda: true,
          sessionToken: "session-recovered",
        };
      }
      throw new ApiError({
        status: 403,
        code: "nda_required",
        message: "nda required",
        requestId: "req-nda",
        requiresEmail: false,
        requiresEmailVerification: false,
        requiresPassword: false,
        requiresNda: true,
        isDealRoom: false,
      });
    });

    await renderPage(token);

    await waitFor(() => {
      expect(accessPublicLinkMock).toHaveBeenCalledWith(
        token,
        expect.objectContaining({
          ndaAgreed: true,
          signerName: "Alice Zhang",
          email: "alice@example.com",
        })
      );
    });
    expect(screen.queryByText("viewer.ndaSignTitle")).not.toBeInTheDocument();
    expect(document.getElementById("signer-name")).not.toBeInTheDocument();
    expect(window.sessionStorage.getItem(`link-session:${token}`)).toBe("session-recovered");
  });

  it("lands on credentials (not sign) when session fails but NDA+verification intent is complete", async () => {
    const token = "nda-session-fail-credentials-token";
    window.sessionStorage.setItem(`link-session:${token}`, "dead-session");
    window.sessionStorage.setItem(
      `nda-intent:${token}`,
      JSON.stringify({
        signerName: "Alice Zhang",
        ndaDeliveryEmail: "alice@example.com",
        ndaAgreed: true,
        ndaTemplateId: "tpl-1",
        contentSha256: "hash-1",
        phase: "complete",
      })
    );

    accessPublicLinkMock.mockImplementation(async (_tok: string, opts?: { sessionToken?: string }) => {
      if (opts?.sessionToken === "dead-session") {
        throw new ApiError({
          status: 403,
          code: "nda_required",
          message: "nda required",
          requestId: "req-dead-session",
          requiresEmail: false,
          requiresEmailVerification: true,
          requiresPassword: false,
          requiresNda: true,
          isDealRoom: false,
        });
      }
      throw new ApiError({
        status: 403,
        code: "requires_email_code",
        message: "email code required",
        requestId: "req-code",
        requiresEmail: false,
        requiresEmailVerification: true,
        requiresPassword: false,
        requiresNda: true,
        isDealRoom: false,
      });
    });

    await renderPage(token);

    await waitFor(() => {
      expect(document.getElementById("email-code")).toBeInTheDocument();
    });
    expect(screen.queryByText("viewer.ndaSignTitle")).not.toBeInTheDocument();
    expect(document.getElementById("signer-name")).not.toBeInTheDocument();
    expect(window.sessionStorage.getItem(`link-session:${token}`)).toBeNull();
  });

  it("discards in-flight Access when the link token changes", async () => {
    let resolveA!: (value: unknown) => void;
    const pendingA = new Promise((resolve) => {
      resolveA = resolve;
    });

    accessPublicLinkMock.mockImplementation(async (tok: string) => {
      if (tok === "token-a") return pendingA;
      throw new ApiError({
        status: 403,
        code: "requires_email_code",
        message: "email code required",
        requestId: "req-b",
        requiresEmail: false,
        requiresEmailVerification: true,
        requiresPassword: false,
        requiresNda: false,
        isDealRoom: false,
      });
    });

    const i18n = await createTestI18n();
    const router = createMemoryRouter(
      [{ path: "/l/:token", element: <PublicViewerPage /> }],
      { initialEntries: ["/l/token-a"] }
    );

    render(
      <I18nextProvider i18n={i18n}>
        <RouterProvider router={router} />
      </I18nextProvider>
    );

    await waitFor(() => {
      expect(accessPublicLinkMock).toHaveBeenCalledWith("token-a", expect.anything());
    });

    await act(async () => {
      await router.navigate("/l/token-b");
    });

    await waitFor(() => {
      expect(document.getElementById("email-code")).toBeInTheDocument();
    });

    await act(async () => {
      resolveA(successAccess);
      await new Promise((resolve) => setTimeout(resolve, 20));
    });

    // Stale token-a success must not overwrite the newer token-b gate.
    expect(document.getElementById("email-code")).toBeInTheDocument();
    expect(screen.queryByText("Deck")).not.toBeInTheDocument();
  });
});
