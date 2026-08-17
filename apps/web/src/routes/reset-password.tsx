import { useState } from "react";
import { useNavigate, useParams } from "react-router";
import { useTranslation } from "react-i18next";
import {
  AuthField,
  AuthInput,
  AuthStage,
  AuthSubmit,
  AuthTextLink,
  railFromNamespace,
} from "@/components/auth/AuthStage";
import { api } from "@/lib/api";
import { apiErrorMessage } from "@/lib/apiErrors";

function passwordErrorKey(pw: string): string | null {
  if (pw.length < 8) return "register.errorPasswordMinLength";
  if (!/[A-Z]/.test(pw)) return "register.errorPasswordUppercase";
  if (!/[a-z]/.test(pw)) return "register.errorPasswordLowercase";
  if (!/[0-9]/.test(pw)) return "register.errorPasswordNumber";
  if (!/[^A-Za-z0-9]/.test(pw)) return "register.errorPasswordSpecial";
  return null;
}

export function ResetPasswordPage() {
  const { token } = useParams<{ token: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation("auth");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!token) {
      setError(t("resetPassword.errorInvalidToken"));
      return;
    }
    const rule = passwordErrorKey(password);
    if (rule) {
      setError(t(rule));
      return;
    }
    if (password !== confirm) {
      setError(t("resetPassword.errorMismatch"));
      return;
    }
    setLoading(true);
    setError(null);
    try {
      await api.resetPassword(token, password);
      navigate("/login?reset=true", { replace: true });
    } catch (err) {
      setError(apiErrorMessage(err, { context: "resetPassword", messageKey: "auth:resetPassword.errorFailed" }));
      setLoading(false);
    }
  };

  if (!token) {
    return (
      <AuthStage
        rail={railFromNamespace(t, "stage")}
        kicker={t("resetPassword.kicker")}
        title={t("resetPassword.title")}
        lede={t("resetPassword.errorInvalidToken")}
        footer={
          <p>
            <AuthTextLink onClick={() => navigate("/forgot-password")}>
              {t("resetPassword.requestNew")}
            </AuthTextLink>
          </p>
        }
      >
        <AuthSubmit type="button" onClick={() => navigate("/forgot-password")}>
          {t("resetPassword.requestNew")}
        </AuthSubmit>
      </AuthStage>
    );
  }

  return (
    <AuthStage
      rail={railFromNamespace(t, "stage")}
      kicker={t("resetPassword.kicker")}
      title={t("resetPassword.title")}
      footer={
        <p>
          <AuthTextLink onClick={() => navigate("/forgot-password")}>
            {t("resetPassword.requestNew")}
          </AuthTextLink>
        </p>
      }
    >
      <form onSubmit={handleSubmit}>
        <AuthField id="password" label={t("resetPassword.password")} hint={t("register.passwordRules")}>
          <AuthInput
            id="password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder={t("resetPassword.passwordPlaceholder")}
            autoComplete="new-password"
            required
          />
        </AuthField>
        <AuthField id="confirm" label={t("resetPassword.confirm")}>
          <AuthInput
            id="confirm"
            type="password"
            value={confirm}
            onChange={(e) => setConfirm(e.target.value)}
            placeholder={t("resetPassword.confirmPlaceholder")}
            autoComplete="new-password"
            required
          />
        </AuthField>
        {error ? <p className="auth-error">{error}</p> : null}
        <AuthSubmit type="submit" disabled={loading}>
          {loading ? t("resetPassword.submitting") : t("resetPassword.submit")}
        </AuthSubmit>
      </form>
    </AuthStage>
  );
}
