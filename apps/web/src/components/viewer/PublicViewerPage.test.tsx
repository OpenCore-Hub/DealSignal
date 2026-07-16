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

  it("renders only access code input for modern email verification (requiresEmail=false)", async () => {
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
      })
    );

    await renderPage("modern-token");

    await waitFor(() => {
      expect(document.getElementById("email-code")).toBeInTheDocument();
    });

    expect(document.getElementById("email")).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/password/i)).not.toBeInTheDocument();
  });

  it("renders email and access code for legacy email_required (requiresEmail=true)", async () => {
    accessPublicLinkMock.mockRejectedValue(
      new ApiError({
        status: 403,
        code: "requires_email",
        message: "email required",
        requestId: "req-2",
        requiresEmail: true,
        requiresEmailVerification: true,
        requiresPassword: false,
        requiresNda: false,
      })
    );

    await renderPage("legacy-token");

    await waitFor(() => {
      expect(document.getElementById("email")).toBeInTheDocument();
    });
    expect(document.getElementById("email-code")).toBeInTheDocument();
  });
});
