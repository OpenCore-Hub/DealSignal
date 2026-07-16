// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, act } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter, Routes, Route } from "react-router";
import { VerifyEmailPage } from "./verify-email";
import { createTestI18n } from "@/i18n/test-utils";

const { verifyEmailMock } = vi.hoisted(() => ({
  verifyEmailMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    verifyEmail: verifyEmailMock,
  },
}));

async function renderPage(token: string) {
  const i18n = await createTestI18n();
  const view = render(
    <MemoryRouter initialEntries={[`/verify-email/${token}`]}>
      <I18nextProvider i18n={i18n}>
        <Routes>
          <Route path="/verify-email/:token" element={<VerifyEmailPage />} />
        </Routes>
      </I18nextProvider>
    </MemoryRouter>
  );
  // Flush async state updates from the verification effect.
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
  return view;
}

describe("VerifyEmailPage", () => {
  beforeEach(() => {
    verifyEmailMock.mockReset();
  });

  it("shows success when verification succeeds", async () => {
    verifyEmailMock.mockResolvedValue({ code: "verified", message: "Email verified successfully" });
    await renderPage("valid-token");

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /login.submit/i })).toBeInTheDocument();
    });
    expect(verifyEmailMock).toHaveBeenCalledWith("valid-token");
  });

  it("shows error when verification fails", async () => {
    verifyEmailMock.mockRejectedValue(new Error("Invalid or expired token"));
    await renderPage("bad-token");

    await waitFor(() => {
      expect(screen.getByText(/invalid or expired token/i)).toBeInTheDocument();
    });
    expect(screen.getByRole("button", { name: /register.signIn/i })).toBeInTheDocument();
  });


});
