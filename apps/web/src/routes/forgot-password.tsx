import { useEffect, useRef, useState } from "react";
import { useNavigate, useSearchParams } from "react-router";
import { useTranslation } from "react-i18next";
import {
  AuthField,
  AuthInput,
  AuthStage,
  AuthSubmit,
  AuthTextLink,
  railFromNamespace,
} from "@/components/auth/AuthStage";
import { TurnstileWidget, type TurnstileWidgetHandle } from "@/components/auth/TurnstileWidget";
import { api } from "@/lib/api";
import { apiErrorMessage } from "@/lib/apiErrors";

export function ForgotPasswordPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { t } = useTranslation("auth");
  const presetEmail = (searchParams.get("email") ?? "").trim();
  const [email, setEmail] = useState(presetEmail);
  const [loading, setLoading] = useState(false);
  const [sent, setSent] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [siteKey, setSiteKey] = useState<string | null>(null);
  const [captchaToken, setCaptchaToken] = useState("");
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

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const trimmed = email.trim();
    if (!trimmed || !trimmed.includes("@")) {
      setError(t("forgotPassword.errorInvalidEmail"));
      return;
    }
    if (siteKey && !captchaToken) {
      setError(t("register.errorCaptchaRequired"));
      return;
    }
    setLoading(true);
    setError(null);
    try {
      await api.forgotPassword(trimmed, captchaToken || undefined);
      setSent(true);
    } catch (err) {
      captchaRef.current?.reset();
      setCaptchaToken("");
      setError(apiErrorMessage(err, { context: "forgotPassword", messageKey: "auth:forgotPassword.errorFailed" }));
    } finally {
      setLoading(false);
    }
  };

  return (
    <AuthStage
      rail={railFromNamespace(t, "stage")}
      kicker={t("forgotPassword.kicker")}
      title={t("forgotPassword.title")}
      lede={sent ? t("forgotPassword.sent") : t("forgotPassword.body")}
      footer={
        <p>
          <AuthTextLink onClick={() => navigate("/login")}>{t("forgotPassword.backToLogin")}</AuthTextLink>
        </p>
      }
    >
      {sent ? null : (
        <form onSubmit={handleSubmit}>
          <AuthField id="email" label={t("forgotPassword.email")}>
            <AuthInput
              id="email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder={t("forgotPassword.emailPlaceholder")}
              autoComplete="email"
              required
            />
          </AuthField>
          {siteKey ? (
            <div className="mt-6">
              <TurnstileWidget
                ref={captchaRef}
                siteKey={siteKey}
                action="register"
                hintKey="forgotPassword.captchaHint"
                onToken={setCaptchaToken}
                onError={() => setError(t("register.errorCaptchaFailed"))}
              />
            </div>
          ) : null}
          {error ? <p className="auth-error">{error}</p> : null}
          <AuthSubmit type="submit" disabled={loading || siteKey === null || Boolean(siteKey && !captchaToken)}>
            {loading ? t("forgotPassword.submitting") : t("forgotPassword.submit")}
          </AuthSubmit>
        </form>
      )}
    </AuthStage>
  );
}
