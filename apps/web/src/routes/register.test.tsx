// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { I18nextProvider } from "react-i18next";
import { RegisterPage } from "./register";
import { createTestI18n } from "@/i18n/test-utils";

const navigateMock = vi.fn();
const { registerMock, getCaptchaMock } = vi.hoisted(() => ({
  registerMock: vi.fn(),
  getCaptchaMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    register: registerMock,
    getCaptcha: getCaptchaMock,
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
  register: {
    title: "Create account",
    email: "Email",
    emailPlaceholder: "you@example.com",
    password: "Password",
    passwordPlaceholder: "Create a password",
    submit: "Create account",
    submitting: "Creating account…",
    hasAccount: "Already have an account?",
    signIn: "Sign in",
    passwordRules: "rules",
    errorInvalidEmail: "invalid email",
    errorPasswordMinLength: "too short",
    errorPasswordUppercase: "need upper",
    errorPasswordLowercase: "need lower",
    errorPasswordNumber: "need number",
    errorPasswordSpecial: "need special",
    errorCaptchaRequired: "complete captcha",
    errorCaptchaFailed: "captcha failed",
    errorCaptchaUnavailable: "captcha unavailable",
    captchaHint: "Complete verification",
    captchaLabel: "Human verification",
    errorRegistrationFailed: "Registration failed",
    forgotPassword: "Forgot password?",
  },
};

async function renderPage() {
  const i18n = await createTestI18n({ auth: authResources });
  return render(
    <I18nextProvider i18n={i18n}>
      <MemoryRouter>
        <RegisterPage />
      </MemoryRouter>
    </I18nextProvider>,
  );
}

describe("RegisterPage Turnstile", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getCaptchaMock.mockResolvedValue({ turnstile_site_key: "" });
    registerMock.mockResolvedValue({ user: { id: "u1", email: "a@example.com" }, verification_required: true });
  });

  it("registers without a token when captcha is off", async () => {
    await renderPage();
    await waitFor(() => expect(getCaptchaMock).toHaveBeenCalled());
    fireEvent.change(screen.getByLabelText("Email"), { target: { value: "a@example.com" } });
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: "Password123!" } });
    fireEvent.click(screen.getByRole("button", { name: "Create account" }));
    await waitFor(() => expect(registerMock).toHaveBeenCalledWith("a@example.com", "Password123!", undefined));
    expect(navigateMock).toHaveBeenCalledWith("/check-email?email=a%40example.com", { replace: true });
  });

  it("keeps the auto-verify path on login when a session is issued", async () => {
    registerMock.mockResolvedValue({ user: { id: "u1", email: "a@example.com" }, expires_in: 900 });
    await renderPage();
    await waitFor(() => expect(getCaptchaMock).toHaveBeenCalled());
    fireEvent.change(screen.getByLabelText("Email"), { target: { value: "a@example.com" } });
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: "Password123!" } });
    fireEvent.click(screen.getByRole("button", { name: "Create account" }));
    await waitFor(() =>
      expect(navigateMock).toHaveBeenCalledWith("/login?registered=true", { replace: true }),
    );
  });

  it("keeps submit disabled until captcha config loads", async () => {
    getCaptchaMock.mockImplementation(() => new Promise(() => {}));
    await renderPage();
    expect(screen.getByRole("button", { name: "Create account" })).toBeDisabled();
  });
});
