// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { TurnstileWidget } from "./TurnstileWidget";
import { createTestI18n } from "@/i18n/test-utils";

describe("TurnstileWidget", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    window.turnstile = {
      render: vi.fn((_el, options: Record<string, unknown>) => {
        const cb = options.callback as (token: string) => void;
        cb("tok-1");
        return "widget-1";
      }),
      reset: vi.fn(),
      remove: vi.fn(),
    };
  });

  it("renders with action register and emits token", async () => {
    const onToken = vi.fn();
    const i18n = await createTestI18n({
      auth: {
        register: {
          captchaHint: "Complete verification",
          captchaLabel: "Human verification",
        },
      },
    });
    render(
      <I18nextProvider i18n={i18n}>
        <TurnstileWidget siteKey="1x00000000000000000000AA" action="register" onToken={onToken} />
      </I18nextProvider>,
    );
    await waitFor(() => expect(onToken).toHaveBeenCalledWith("tok-1"));
    expect(window.turnstile?.render).toHaveBeenCalled();
    const opts = vi.mocked(window.turnstile!.render).mock.calls[0][1] as Record<string, unknown>;
    expect(opts.action).toBe("register");
    expect(opts.sitekey).toBe("1x00000000000000000000AA");
  });
});
