// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router";
import { VerifyEmailPage } from "./verify-email";

const { verifyEmailMock } = vi.hoisted(() => ({
  verifyEmailMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    verifyEmail: verifyEmailMock,
  },
}));

function renderPage(token: string) {
  return render(
    <MemoryRouter initialEntries={[`/verify-email/${token}`]}>
      <Routes>
        <Route path="/verify-email/:token" element={<VerifyEmailPage />} />
      </Routes>
    </MemoryRouter>
  );
}

describe("VerifyEmailPage", () => {
  beforeEach(() => {
    verifyEmailMock.mockReset();
  });

  it("shows success when verification succeeds", async () => {
    verifyEmailMock.mockResolvedValue({ code: "verified", message: "Email verified successfully" });
    renderPage("valid-token");

    expect(screen.getAllByText(/verifyEmail.verifying/i).length).toBeGreaterThan(0);

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /login.submit/i })).toBeInTheDocument();
    });
    expect(verifyEmailMock).toHaveBeenCalledWith("valid-token");
  });

  it("shows error when verification fails", async () => {
    verifyEmailMock.mockRejectedValue(new Error("Invalid or expired token"));
    renderPage("bad-token");

    await waitFor(() => {
      expect(screen.getByText(/invalid or expired token/i)).toBeInTheDocument();
    });
    expect(screen.getByRole("button", { name: /register.signIn/i })).toBeInTheDocument();
  });


});
