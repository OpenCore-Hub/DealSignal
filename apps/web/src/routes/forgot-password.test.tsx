// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { I18nextProvider } from "react-i18next";
import { ForgotPasswordPage } from "./forgot-password";
import { createTestI18n } from "@/i18n/test-utils";

const navigateMock = vi.fn();
const { forgotPasswordMock, getCaptchaMock } = vi.hoisted(() => ({
  forgotPasswordMock: vi.fn(),
  getCaptchaMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    forgotPassword: forgotPasswordMock,
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
  forgotPassword: {
    title: "Reset your password",
    body: "Enter your email.",
    email: "Email",
    emailPlaceholder: "you@example.com",
    submit: "Send reset link",
    submitting: "Sending…",
    sent: "If that inbox has an activated account, we sent a reset link.",
    backToLogin: "Back to sign in",
    errorInvalidEmail: "invalid email",
    errorFailed: "Could not send.",
    captchaHint: "Complete verification",
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
      <MemoryRouter initialEntries={[search ? `/forgot-password${search}` : "/forgot-password"]}>
        <ForgotPasswordPage />
      </MemoryRouter>
    </I18nextProvider>,
  );
}

describe("ForgotPasswordPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getCaptchaMock.mockResolvedValue({ turnstile_site_key: "" });
    forgotPasswordMock.mockResolvedValue({ code: "ok" });
  });

  it("always shows the generic success copy after submit", async () => {
    await renderPage();
    await waitFor(() => expect(getCaptchaMock).toHaveBeenCalled());
    fireEvent.change(screen.getByLabelText("Email"), { target: { value: "a@example.com" } });
    fireEvent.click(screen.getByRole("button", { name: "Send reset link" }));
    await waitFor(() => expect(forgotPasswordMock).toHaveBeenCalledWith("a@example.com", undefined));
    expect(await screen.findByText("If that inbox has an activated account, we sent a reset link.")).toBeInTheDocument();
  });
});
