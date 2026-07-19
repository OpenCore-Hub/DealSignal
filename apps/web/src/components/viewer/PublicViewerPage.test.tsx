// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, act, fireEvent } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter, Routes, Route } from "react-router";
import { PublicViewerPage } from "./PublicViewerPage";
import { createTestI18n } from "@/i18n/test-utils";
import { ApiError } from "@/lib/apiClient";

const { accessPublicLinkMock, sendEmailVerificationCodeMock } = vi.hoisted(() => ({
  accessPublicLinkMock: vi.fn(),
  sendEmailVerificationCodeMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    accessPublicLink: accessPublicLinkMock,
    sendEmailVerificationCode: sendEmailVerificationCodeMock,
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
    sendEmailVerificationCodeMock.mockReset();
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

  it("renders email, send code button, and access code for deal-room verification", async () => {
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
      expect(document.getElementById("email")).toBeInTheDocument();
    });
    expect(document.getElementById("email-code")).toBeInTheDocument();
    expect(screen.getByText("viewer.sendCode")).toBeInTheDocument();
  });

  it("sends verification code when deal-room visitor enters email and clicks send", async () => {
    accessPublicLinkMock.mockRejectedValue(
      new ApiError({
        status: 403,
        code: "requires_email_code",
        message: "email code required",
        requestId: "req-3",
        requiresEmail: false,
        requiresEmailVerification: true,
        requiresPassword: false,
        requiresNda: false,
        isDealRoom: true,
      })
    );
    sendEmailVerificationCodeMock.mockResolvedValue(undefined);

    await renderPage("dealroom-token");

    await waitFor(() => {
      expect(document.getElementById("email")).toBeInTheDocument();
    });

    await act(async () => {
      const emailInput = document.getElementById("email") as HTMLInputElement;
      fireEvent.change(emailInput, { target: { value: "visitor@example.com" } });
    });

    await act(async () => {
      screen.getByText("viewer.sendCode").click();
    });

    await waitFor(() => {
      expect(sendEmailVerificationCodeMock).toHaveBeenCalledWith("dealroom-token", "visitor@example.com");
    });
  });
});
