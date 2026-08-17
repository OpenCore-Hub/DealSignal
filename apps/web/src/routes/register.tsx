import { useEffect, useRef, useState } from "react";
import { useNavigate, useSearchParams } from "react-router";
import { useTranslation } from "react-i18next";
import {
  AuthField,
  AuthInput,
  AuthNotice,
  AuthStage,
  AuthSubmit,
  AuthTextLink,
  railFromNamespace,
} from "@/components/auth/AuthStage";
import { TurnstileWidget, type TurnstileWidgetHandle } from "@/components/auth/TurnstileWidget";
import { api } from "@/lib/api";
import { apiErrorMessage } from "@/lib/apiErrors";
import {
  buildInviteAuthPath,
  inviteEmailFromSearchParams,
  isInviteAuthFlow,
  safeAuthRedirect,
} from "@/lib/inviteAuth";

export function RegisterPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { t } = useTranslation("auth");
  const invitedEmail = inviteEmailFromSearchParams(searchParams);
  const lockEmail = isInviteAuthFlow(searchParams);
  const [email, setEmail] = useState(invitedEmail);
  const [password, setPassword] = useState("");
  const [loading, setLoading] = useState(false);
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
        if (!cancelled) {
          setError(t("register.errorCaptchaUnavailable"));
        }
      });
    return () => {
      cancelled = true;
    };
  }, [t]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    const trimmedEmail = (lockEmail ? invitedEmail : email).trim();
    if (!trimmedEmail || !trimmedEmail.includes("@")) {
      setError(t("register.errorInvalidEmail"));
      return;
    }

    setLoading(true);
    setError(null);

    const pw = password;
    if (pw.length < 8) {
      setError(t("register.errorPasswordMinLength"));
      setLoading(false);
      return;
    }
    if (!/[A-Z]/.test(pw)) {
      setError(t("register.errorPasswordUppercase"));
      setLoading(false);
      return;
    }
    if (!/[a-z]/.test(pw)) {
      setError(t("register.errorPasswordLowercase"));
      setLoading(false);
      return;
    }
    if (!/[0-9]/.test(pw)) {
      setError(t("register.errorPasswordNumber"));
      setLoading(false);
      return;
    }
    if (!/[^A-Za-z0-9]/.test(pw)) {
      setError(t("register.errorPasswordSpecial"));
      setLoading(false);
      return;
    }

    if (siteKey && !captchaToken) {
      setError(t("register.errorCaptchaRequired"));
      setLoading(false);
      return;
    }

    try {
      const res = await api.register(trimmedEmail, password, captchaToken || undefined);
      const redirect = safeAuthRedirect(searchParams.get("redirect"));
      if (res.verification_required) {
        const params = new URLSearchParams({ email: trimmedEmail });
        if (redirect) params.set("redirect", redirect);
        if (lockEmail && invitedEmail) params.set("invite", "1");
        navigate(`/check-email?${params.toString()}`, { replace: true });
        return;
      }
      const params = new URLSearchParams({ registered: "true" });
      if (redirect) params.set("redirect", redirect);
      if (lockEmail && invitedEmail) {
        params.set("email", invitedEmail);
        params.set("invite", "1");
      }
      navigate(`/login?${params.toString()}`, { replace: true });
    } catch (err) {
      captchaRef.current?.reset();
      setCaptchaToken("");
      setError(apiErrorMessage(err, { context: "register", messageKey: "auth:register.errorRegistrationFailed" }));
      setLoading(false);
    }
  };

  const goForgot = () => {
    const current = (lockEmail ? invitedEmail : email).trim();
    const params = new URLSearchParams();
    if (current.includes("@")) params.set("email", current);
    const qs = params.toString();
    navigate(qs ? `/forgot-password?${qs}` : "/forgot-password");
  };

  return (
    <AuthStage
      rail={railFromNamespace(t, "stage")}
      kicker={t("register.kicker")}
      title={t("register.title")}
      notice={lockEmail ? <AuthNotice tone="invite">{t("register.inviteBanner")}</AuthNotice> : null}
      footer={
        <p>
          {t("register.hasAccount")}{" "}
          <AuthTextLink
            onClick={() => {
              navigate(
                buildInviteAuthPath("login", {
                  redirect: searchParams.get("redirect"),
                  email: invitedEmail || undefined,
                }),
              );
            }}
          >
            {t("register.signIn")}
          </AuthTextLink>
        </p>
      }
    >
      <form onSubmit={handleSubmit}>
        <AuthField
          id="email"
          label={t("register.email")}
          hint={lockEmail ? t("register.emailLockedHint") : undefined}
        >
          <AuthInput
            id="email"
            type="email"
            value={email}
            onChange={(e) => {
              if (!lockEmail) setEmail(e.target.value);
            }}
            placeholder={t("register.emailPlaceholder")}
            autoComplete="email"
            readOnly={lockEmail}
            aria-readonly={lockEmail || undefined}
            required
          />
        </AuthField>
        <AuthField
          id="password"
          label={t("register.password")}
          action={<AuthTextLink onClick={goForgot}>{t("register.forgotPassword")}</AuthTextLink>}
          hint={t("register.passwordRules")}
        >
          <AuthInput
            id="password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder={t("register.passwordPlaceholder")}
            autoComplete="new-password"
            required
          />
        </AuthField>
        {siteKey ? (
          <div className="mt-6">
            <TurnstileWidget
              ref={captchaRef}
              siteKey={siteKey}
              action="register"
              onToken={setCaptchaToken}
              onError={() => setError(t("register.errorCaptchaFailed"))}
            />
          </div>
        ) : null}
        {error ? <p className="auth-error">{error}</p> : null}
        <AuthSubmit type="submit" disabled={loading || siteKey === null || Boolean(siteKey && !captchaToken)}>
          {loading ? t("register.submitting") : t("register.submit")}
        </AuthSubmit>
      </form>
    </AuthStage>
  );
}
