import { useState } from "react";
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
import { ResendVerification } from "@/components/auth/ResendVerification";
import { api } from "@/lib/api";
import { ApiError } from "@/lib/apiClient";
import { apiErrorMessage } from "@/lib/apiErrors";
import {
  buildInviteAuthPath,
  inviteEmailFromSearchParams,
  isInviteAuthFlow,
  safeAuthRedirect,
  workspaceInviteTokenFromRedirect,
} from "@/lib/inviteAuth";

export function LoginPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { t } = useTranslation("auth");
  const invitedEmail = inviteEmailFromSearchParams(searchParams);
  const lockEmail = isInviteAuthFlow(searchParams);
  const [email, setEmail] = useState(invitedEmail);
  const [password, setPassword] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [unverified, setUnverified] = useState(false);
  const registered = searchParams.get("registered") === "true";
  const resetDone = searchParams.get("reset") === "true";

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    const trimmedEmail = (lockEmail ? invitedEmail : email).trim();
    if (!trimmedEmail || !trimmedEmail.includes("@")) {
      setError(t("login.errorInvalidEmail"));
      return;
    }
    if (!password) {
      setError(t("login.errorEmptyPassword"));
      return;
    }

    setLoading(true);
    setError(null);
    setUnverified(false);
    try {
      await api.login(
        trimmedEmail,
        password,
        workspaceInviteTokenFromRedirect(searchParams.get("redirect")) || undefined,
      );
      const redirect = safeAuthRedirect(searchParams.get("redirect"));
      navigate(redirect ?? "/", { replace: true });
    } catch (err) {
      if (err instanceof ApiError && err.code === "email_not_verified") {
        setUnverified(true);
        setError(t("login.emailNotVerified"));
      } else {
        setError(apiErrorMessage(err, { context: "login", messageKey: "auth:login.errorLoginFailed" }));
      }
      setLoading(false);
    }
  };

  const goForgot = () => {
    const params = new URLSearchParams();
    const current = (lockEmail ? invitedEmail : email).trim();
    if (current.includes("@")) params.set("email", current);
    const qs = params.toString();
    navigate(qs ? `/forgot-password?${qs}` : "/forgot-password");
  };

  return (
    <AuthStage
      rail={railFromNamespace(t, "stage")}
      kicker={t("login.kicker")}
      title={t("login.title")}
      notice={
        <>
          {registered ? <AuthNotice tone="ok">{t("login.registeredSuccess")}</AuthNotice> : null}
          {resetDone ? <AuthNotice tone="ok">{t("login.resetSuccess")}</AuthNotice> : null}
          {lockEmail ? <AuthNotice tone="invite">{t("login.inviteBanner")}</AuthNotice> : null}
        </>
      }
      footer={
        <p>
          {t("login.noAccount")}{" "}
          <AuthTextLink
            onClick={() => {
              navigate(
                buildInviteAuthPath("register", {
                  redirect: searchParams.get("redirect"),
                  email: invitedEmail || undefined,
                }),
              );
            }}
          >
            {t("login.signUp")}
          </AuthTextLink>
        </p>
      }
    >
      <form onSubmit={handleSubmit}>
        <AuthField
          id="email"
          label={t("login.email")}
          hint={lockEmail ? t("login.emailLockedHint") : undefined}
        >
          <AuthInput
            id="email"
            type="email"
            value={email}
            onChange={(e) => {
              if (!lockEmail) setEmail(e.target.value);
            }}
            placeholder={t("login.emailPlaceholder")}
            autoComplete="email"
            readOnly={lockEmail}
            aria-readonly={lockEmail || undefined}
            required
          />
        </AuthField>
        <AuthField
          id="password"
          label={t("login.password")}
          action={<AuthTextLink onClick={goForgot}>{t("login.forgotPassword")}</AuthTextLink>}
        >
          <AuthInput
            id="password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder={t("login.passwordPlaceholder")}
            autoComplete="current-password"
            required
          />
        </AuthField>
        {error ? <p className="auth-error">{error}</p> : null}
        {unverified ? <ResendVerification email={(lockEmail ? invitedEmail : email).trim()} /> : null}
        <AuthSubmit type="submit" disabled={loading}>
          {loading ? t("login.submitting") : t("login.submit")}
        </AuthSubmit>
      </form>
    </AuthStage>
  );
}
