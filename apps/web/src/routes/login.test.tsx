// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { I18nextProvider } from "react-i18next";
import { LoginPage } from "./login";
import { createTestI18n } from "@/i18n/test-utils";
import { ApiError } from "@/lib/apiClient";

const navigateMock = vi.fn();
const { loginMock, getCaptchaMock, resendMock } = vi.hoisted(() => ({
  loginMock: vi.fn(),
  getCaptchaMock: vi.fn(),
  resendMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    login: loginMock,
    getCaptcha: getCaptchaMock,
    resendVerification: resendMock,
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
  login: {
    title: "Sign in",
    email: "Email",
    emailPlaceholder: "you@example.com",
    password: "Password",
    passwordPlaceholder: "••••••••",
    submit: "Sign in",
    submitting: "Signing in…",
    noAccount: "Don't have an account?",
    signUp: "Sign up",
    errorInvalidEmail: "invalid email",
    errorEmptyPassword: "empty password",
    errorLoginFailed: "Login failed",
    errorInvalidCredentials: "bad credentials",
    emailNotVerified: "Verify your email before signing in.",
    forgotPassword: "Forgot password?",
    resetSuccess: "Password updated. Sign in with your new password.",
  },
  checkEmail: {
    resend: "Resend email",
    resending: "Sending…",
    resent: "If that inbox can receive mail, we sent another link.",
    errorResendFailed: "Could not resend.",
    captchaHint: "Complete verification to resend.",
  },
  register: {
    errorCaptchaRequired: "complete captcha",
    errorCaptchaFailed: "captcha failed",
    errorCaptchaUnavailable: "captcha unavailable",
    captchaHint: "Complete verification",
    captchaLabel: "Human verification",
  },
};

async function renderPage(search = "") {
  const i18n = await createTestI18n({ auth: authResources });
  return render(
    <I18nextProvider i18n={i18n}>
      <MemoryRouter initialEntries={[search ? `/login${search}` : "/login"]}>
        <LoginPage />
      </MemoryRouter>
    </I18nextProvider>,
  );
}

describe("LoginPage email verification", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getCaptchaMock.mockResolvedValue({ turnstile_site_key: "" });
    resendMock.mockResolvedValue({ code: "ok" });
  });

  it("prompts to verify and offers resend when the mailbox is unconfirmed", async () => {
    loginMock.mockRejectedValue(
      new ApiError({
        status: 403,
        code: "email_not_verified",
        message: "email not verified",
        requestId: "r1",
      }),
    );
    await renderPage();
    fireEvent.change(screen.getByLabelText("Email"), { target: { value: "a@example.com" } });
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: "Password123!" } });
    fireEvent.click(screen.getByRole("button", { name: "Sign in" }));
    expect(await screen.findByText("Verify your email before signing in.")).toBeInTheDocument();
    await waitFor(() => expect(getCaptchaMock).toHaveBeenCalled());
    fireEvent.click(screen.getByRole("button", { name: "Resend email" }));
    await waitFor(() => expect(resendMock).toHaveBeenCalledWith("a@example.com", undefined));
  });

  it("links to password reset from the sign-in form", async () => {
    await renderPage();
    fireEvent.change(screen.getByLabelText("Email"), { target: { value: "a@example.com" } });
    fireEvent.click(screen.getByRole("button", { name: "Forgot password?" }));
    expect(navigateMock).toHaveBeenCalledWith("/forgot-password?email=a%40example.com");
  });

  it("sends the workspace invite token from a safe accept redirect", async () => {
    loginMock.mockResolvedValue({ id: "u1", email: "a@example.com" });
    await renderPage("?redirect=/invitations/tok-1/accept");
    fireEvent.change(screen.getByLabelText("Email"), { target: { value: "a@example.com" } });
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: "Password123!" } });
    fireEvent.click(screen.getByRole("button", { name: "Sign in" }));
    await waitFor(() =>
      expect(loginMock).toHaveBeenCalledWith("a@example.com", "Password123!", "tok-1"),
    );
  });
});
