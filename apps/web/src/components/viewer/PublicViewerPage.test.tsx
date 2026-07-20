// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, act } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter, Routes, Route } from "react-router";
import { PublicViewerPage } from "./PublicViewerPage";
import { createTestI18n } from "@/i18n/test-utils";
import { ApiError } from "@/lib/apiClient";

const { accessPublicLinkMock } = vi.hoisted(() => ({
  accessPublicLinkMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    accessPublicLink: accessPublicLinkMock,
  },
}));

async function renderPage(token: string) {
  const i18n = await createTestI18n();
  const view = render(
    <MemoryRouter initialEntries={[`/l/${token}`]}>
      <I18nextProvider i18n={i18n}>
        <Routes>
          <Route path="/l/:token" element={<PublicViewerPage />} />
        </Routes>
      </I18nextProvider>
    </MemoryRouter>
  );
  // Flush async state updates from the access-public-link effect.
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
  return view;
}

describe("PublicViewerPage", () => {
  beforeEach(() => {
    accessPublicLinkMock.mockReset();
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
});
