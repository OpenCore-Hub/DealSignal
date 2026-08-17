// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { I18nextProvider } from "react-i18next";
import { ResetPasswordPage } from "./reset-password";
import { createTestI18n } from "@/i18n/test-utils";
import { ApiError } from "@/lib/apiClient";

const navigateMock = vi.fn();
const { resetPasswordMock } = vi.hoisted(() => ({
  resetPasswordMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    resetPassword: resetPasswordMock,
  },
}));

vi.mock("react-router", async () => {
  const actual = await vi.importActual<typeof import("react-router")>("react-router");
  return {
    ...actual,
    useNavigate: () => navigateMock,
  };
});

const authResources = {
  resetPassword: {
    title: "Choose a new password",
    password: "New password",
    passwordPlaceholder: "Create a new password",
    confirm: "Confirm password",
    confirmPlaceholder: "Re-enter your new password",
    submit: "Update password",
    submitting: "Updating…",
    errorMismatch: "Passwords do not match.",
    errorInvalidToken: "This reset link is invalid or has expired.",
    errorFailed: "Could not update.",
    requestNew: "Request a new reset link",
  },
  register: {
    passwordRules: "rules",
    errorPasswordMinLength: "too short",
    errorPasswordUppercase: "need upper",
    errorPasswordLowercase: "need lower",
    errorPasswordNumber: "need number",
    errorPasswordSpecial: "need special",
  },
};

async function renderPage(path = "/reset-password/tok-1") {
  const i18n = await createTestI18n({ auth: authResources });
  return render(
    <I18nextProvider i18n={i18n}>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path="/reset-password/:token" element={<ResetPasswordPage />} />
        </Routes>
      </MemoryRouter>
    </I18nextProvider>,
  );
}

describe("ResetPasswordPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    resetPasswordMock.mockResolvedValue({ code: "ok" });
  });

  it("rejects mismatched passwords without calling the API", async () => {
    await renderPage();
    fireEvent.change(screen.getByLabelText("New password"), { target: { value: "Password123!" } });
    fireEvent.change(screen.getByLabelText("Confirm password"), { target: { value: "Password123?" } });
    fireEvent.click(screen.getByRole("button", { name: "Update password" }));
    expect(await screen.findByText("Passwords do not match.")).toBeInTheDocument();
    expect(resetPasswordMock).not.toHaveBeenCalled();
  });

  it("posts the token then sends the user to sign in", async () => {
    await renderPage();
    fireEvent.change(screen.getByLabelText("New password"), { target: { value: "Password123!" } });
    fireEvent.change(screen.getByLabelText("Confirm password"), { target: { value: "Password123!" } });
    fireEvent.click(screen.getByRole("button", { name: "Update password" }));
    await waitFor(() => expect(resetPasswordMock).toHaveBeenCalledWith("tok-1", "Password123!"));
    expect(navigateMock).toHaveBeenCalledWith("/login?reset=true", { replace: true });
  });

  it("shows a generic expired-link error", async () => {
    resetPasswordMock.mockRejectedValue(
      new ApiError({
        status: 400,
        code: "invalid_or_expired_token",
        message: "reset link is invalid or has expired",
        requestId: "r1",
      }),
    );
    await renderPage();
    fireEvent.change(screen.getByLabelText("New password"), { target: { value: "Password123!" } });
    fireEvent.change(screen.getByLabelText("Confirm password"), { target: { value: "Password123!" } });
    fireEvent.click(screen.getByRole("button", { name: "Update password" }));
    expect(await screen.findByText(/invalid or has expired/i)).toBeInTheDocument();
    expect(navigateMock).not.toHaveBeenCalled();
  });
});
