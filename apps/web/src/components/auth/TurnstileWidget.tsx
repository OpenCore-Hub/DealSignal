import { forwardRef, useEffect, useId, useImperativeHandle, useRef } from "react";
import { useTranslation } from "react-i18next";
import { getCurrentLanguage } from "@/i18n/utils";

const SCRIPT_SRC = "https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit";

type TurnstileAPI = {
  render: (container: HTMLElement, options: Record<string, unknown>) => string;
  reset: (widgetId?: string) => void;
  remove: (widgetId: string) => void;
};

declare global {
  interface Window {
    turnstile?: TurnstileAPI;
  }
}

let scriptPromise: Promise<TurnstileAPI> | null = null;

function loadTurnstile(): Promise<TurnstileAPI> {
  if (typeof window === "undefined") {
    return Promise.reject(new Error("turnstile requires a browser"));
  }
  if (window.turnstile) {
    return Promise.resolve(window.turnstile);
  }
  if (!scriptPromise) {
    scriptPromise = new Promise((resolve, reject) => {
      const existing = document.querySelector<HTMLScriptElement>(`script[src="${SCRIPT_SRC}"]`);
      if (existing) {
        existing.addEventListener("load", () => {
          if (window.turnstile) resolve(window.turnstile);
          else reject(new Error("turnstile missing after load"));
        });
        existing.addEventListener("error", () => reject(new Error("turnstile script failed")));
        return;
      }
      const script = document.createElement("script");
      script.src = SCRIPT_SRC;
      script.async = true;
      script.defer = true;
      script.onload = () => {
        if (window.turnstile) resolve(window.turnstile);
        else reject(new Error("turnstile missing after load"));
      };
      script.onerror = () => reject(new Error("turnstile script failed"));
      document.head.appendChild(script);
    });
  }
  return scriptPromise;
}

function widgetTheme(): "light" | "dark" {
  if (typeof document === "undefined") return "light";
  return document.documentElement.classList.contains("dark") ? "dark" : "light";
}

function widgetLanguage(): string {
  const lng = getCurrentLanguage();
  return lng === "zh-CN" ? "zh-cn" : "en";
}

export type TurnstileWidgetHandle = {
  reset: () => void;
};

export const TurnstileWidget = forwardRef<
  TurnstileWidgetHandle,
  {
    siteKey: string;
    action: string;
    hintKey?: string;
    onToken: (token: string) => void;
    onError?: () => void;
  }
>(function TurnstileWidget({ siteKey, action, hintKey = "register.captchaHint", onToken, onError }, ref) {
  const { t } = useTranslation("auth");
  const containerRef = useRef<HTMLDivElement>(null);
  const widgetIdRef = useRef<string | null>(null);
  const onTokenRef = useRef(onToken);
  const onErrorRef = useRef(onError);
  const reactId = useId();
  onTokenRef.current = onToken;
  onErrorRef.current = onError;

  useImperativeHandle(ref, () => ({
    reset: () => {
      onTokenRef.current("");
      const api = window.turnstile;
      const id = widgetIdRef.current;
      if (api && id) api.reset(id);
    },
  }));

  useEffect(() => {
    if (!siteKey || !containerRef.current) return;
    let cancelled = false;
    loadTurnstile()
      .then((api) => {
        if (cancelled || !containerRef.current) return;
        if (widgetIdRef.current) {
          api.remove(widgetIdRef.current);
          widgetIdRef.current = null;
        }
        widgetIdRef.current = api.render(containerRef.current, {
          sitekey: siteKey,
          action,
          theme: widgetTheme(),
          language: widgetLanguage(),
          appearance: "always",
          size: "flexible",
          callback: (token: string) => onTokenRef.current(token),
          "expired-callback": () => onTokenRef.current(""),
          "error-callback": () => {
            onTokenRef.current("");
            onErrorRef.current?.();
          },
        });
      })
      .catch(() => {
        if (!cancelled) onErrorRef.current?.();
      });
    return () => {
      cancelled = true;
      const api = window.turnstile;
      const id = widgetIdRef.current;
      if (api && id) {
        api.remove(id);
        widgetIdRef.current = null;
      }
    };
  }, [siteKey, action]);

  return (
    <div className="space-y-2">
      <p className="text-caption text-muted-foreground">{t(hintKey)}</p>
      <div
        ref={containerRef}
        id={`turnstile-${reactId}`}
        className="min-h-[65px]"
        aria-label={t("register.captchaLabel")}
      />
    </div>
  );
});
