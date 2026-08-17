import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { TurnstileWidget, type TurnstileWidgetHandle } from "@/components/auth/TurnstileWidget";
import { api } from "@/lib/api";
import { apiErrorMessage } from "@/lib/apiErrors";

export function ResendVerification({ email }: { email: string }) {
  const { t } = useTranslation("auth");
  const [siteKey, setSiteKey] = useState<string | null>(null);
  const [captchaToken, setCaptchaToken] = useState("");
  const [loading, setLoading] = useState(false);
  const [sent, setSent] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const captchaRef = useRef<TurnstileWidgetHandle>(null);

  useEffect(() => {
    let cancelled = false;
    api
      .getCaptcha()
      .then((cfg) => {
        if (!cancelled) setSiteKey(cfg.turnstile_site_key?.trim() ?? "");
      })
      .catch(() => {
        if (!cancelled) setError(t("register.errorCaptchaUnavailable"));
      });
    return () => {
      cancelled = true;
    };
  }, [t]);

  const handleResend = async () => {
    if (!email.includes("@")) {
      return;
    }
    if (siteKey && !captchaToken) {
      setError(t("register.errorCaptchaRequired"));
      return;
    }
    setLoading(true);
    setError(null);
    try {
      await api.resendVerification(email, captchaToken || undefined);
      setSent(true);
    } catch (err) {
      captchaRef.current?.reset();
      setCaptchaToken("");
      setError(apiErrorMessage(err, { messageKey: "auth:checkEmail.errorResendFailed" }));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="space-y-3">
      {siteKey ? (
        <TurnstileWidget
          ref={captchaRef}
          siteKey={siteKey}
          action="register"
          hintKey="checkEmail.captchaHint"
          onToken={setCaptchaToken}
          onError={() => setError(t("register.errorCaptchaFailed"))}
        />
      ) : null}
      {error ? <p className="text-sm text-error-500">{error}</p> : null}
      {sent ? <p className="text-sm text-muted-foreground">{t("checkEmail.resent")}</p> : null}
      <Button
        type="button"
        variant="outline"
        className="w-full"
        disabled={loading || siteKey === null || Boolean(siteKey && !captchaToken)}
        onClick={() => void handleResend()}
      >
        {loading ? t("checkEmail.resending") : t("checkEmail.resend")}
      </Button>
    </div>
  );
}
