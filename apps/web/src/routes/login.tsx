import { useState } from "react";
import { useNavigate, useSearchParams } from "react-router";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
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

  return (
    <div className="flex min-h-[100dvh] flex-col items-center justify-center bg-background p-6">
      <div className="w-full max-w-md">
        <Card>
          <CardHeader>
            <CardTitle className="text-h2">{t("login.title")}</CardTitle>
          </CardHeader>
          <CardContent>
            {registered && (
              <div className="mb-4 rounded-md bg-green-50 p-3 text-sm text-green-700 dark:bg-green-950 dark:text-green-300">
                {t("login.registeredSuccess")}
              </div>
            )}
            {resetDone && (
              <div className="mb-4 rounded-md bg-green-50 p-3 text-sm text-green-700 dark:bg-green-950 dark:text-green-300">
                {t("login.resetSuccess")}
              </div>
            )}
            {lockEmail ? (
              <div className="mb-4 rounded-md bg-muted p-3 text-sm text-muted-foreground">{t("login.inviteBanner")}</div>
            ) : null}
            <form onSubmit={handleSubmit} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="email">{t("login.email")}</Label>
                <Input
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
                {lockEmail ? <p className="text-caption text-muted-foreground">{t("login.emailLockedHint")}</p> : null}
              </div>
              <div className="space-y-2">
                <div className="flex items-center justify-between gap-3">
                  <Label htmlFor="password">{t("login.password")}</Label>
                  <Button
                    type="button"
                    variant="link"
                    className="h-auto p-0 text-sm"
                    onClick={() => {
                      const params = new URLSearchParams();
                      const current = (lockEmail ? invitedEmail : email).trim();
                      if (current.includes("@")) params.set("email", current);
                      const qs = params.toString();
                      navigate(qs ? `/forgot-password?${qs}` : "/forgot-password");
                    }}
                  >
                    {t("login.forgotPassword")}
                  </Button>
                </div>
                <Input
                  id="password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder={t("login.passwordPlaceholder")}
                  autoComplete="current-password"
                  required
                />
              </div>
              {error && <p className="text-sm text-error-500">{error}</p>}
              {unverified ? <ResendVerification email={(lockEmail ? invitedEmail : email).trim()} /> : null}
              <Button type="submit" className="w-full" disabled={loading}>
                {loading ? t("login.submitting") : t("login.submit")}
              </Button>
              <p className="text-center text-sm text-muted-foreground">
                {t("login.noAccount")}{" "}
                <Button
                  variant="link"
                  className="p-0"
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
                </Button>
              </p>
            </form>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
