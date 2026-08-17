// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { I18nextProvider } from "react-i18next";
import { CheckEmailPage } from "./check-email";
import { createTestI18n } from "@/i18n/test-utils";

const navigateMock = vi.fn();
const { resendMock, getCaptchaMock } = vi.hoisted(() => ({
  resendMock: vi.fn(),
  getCaptchaMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    resendVerification: resendMock,
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
  checkEmail: {
    title: "Check your email",
    body: "We sent an activation link.",
    bodyWithEmail: "We sent an activation link to {{email}}.",
    resend: "Resend email",
    resending: "Sending…",
    resent: "If that inbox can receive mail, we sent another link.",
    backToLogin: "Back to sign in",
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

async function renderPage(search = "?email=a@example.com") {
  const i18n = await createTestI18n({ auth: authResources });
  return render(
    <I18nextProvider i18n={i18n}>
      <MemoryRouter initialEntries={[`/check-email${search}`]}>
        <Routes>
          <Route path="/check-email" element={<CheckEmailPage />} />
        </Routes>
      </MemoryRouter>
    </I18nextProvider>,
  );
}

describe("CheckEmailPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getCaptchaMock.mockResolvedValue({ turnstile_site_key: "" });
    resendMock.mockResolvedValue({ code: "ok" });
  });

  it("shows the mailbox and resends without captcha when it is off", async () => {
    await renderPage();
    expect(screen.getByText(/we sent an activation link to a@example.com/i)).toBeInTheDocument();
    await waitFor(() => expect(getCaptchaMock).toHaveBeenCalled());
    fireEvent.click(screen.getByRole("button", { name: "Resend email" }));
    await waitFor(() => expect(resendMock).toHaveBeenCalledWith("a@example.com", undefined));
    expect(await screen.findByText(/if that inbox can receive mail/i)).toBeInTheDocument();
  });

  it("returns to login", async () => {
    await renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Back to sign in" }));
    expect(navigateMock).toHaveBeenCalledWith("/login?email=a%40example.com");
  });
});
