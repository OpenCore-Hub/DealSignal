// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router";
import { PublicViewerPage } from "./PublicViewerPage";
import { ApiError } from "@/lib/apiClient";

const { accessPublicLinkMock } = vi.hoisted(() => ({
  accessPublicLinkMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    accessPublicLink: accessPublicLinkMock,
  },
}));

function renderPage(token: string) {
  return render(
    <MemoryRouter initialEntries={[`/l/${token}`]}>
      <Routes>
        <Route path="/l/:token" element={<PublicViewerPage />} />
      </Routes>
    </MemoryRouter>
  );
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

    renderPage("modern-token");

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

    renderPage("legacy-token");

    await waitFor(() => {
      expect(document.getElementById("email")).toBeInTheDocument();
    });
    expect(document.getElementById("email-code")).toBeInTheDocument();
  });
});
