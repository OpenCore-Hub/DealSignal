import { useEffect, useRef, useState } from "react";
import { useNavigate, useSearchParams } from "react-router";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
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

  return (
    <div className="flex min-h-[100dvh] flex-col items-center justify-center bg-background p-6">
      <div className="w-full max-w-md">
        <Card>
          <CardHeader>
            <CardTitle className="text-h2">{t("register.title")}</CardTitle>
          </CardHeader>
          <CardContent>
            {lockEmail ? (
              <div className="mb-4 rounded-md bg-muted p-3 text-sm text-muted-foreground">{t("register.inviteBanner")}</div>
            ) : null}
            <form onSubmit={handleSubmit} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="email">{t("register.email")}</Label>
                <Input
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
                {lockEmail ? <p className="text-caption text-muted-foreground">{t("register.emailLockedHint")}</p> : null}
              </div>
              <div className="space-y-2">
                <Label htmlFor="password">{t("register.password")}</Label>
                <Input
                  id="password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder={t("register.passwordPlaceholder")}
                  autoComplete="new-password"
                  required
                />
                <p className="text-caption text-muted-foreground">{t("register.passwordRules")}</p>
              </div>
              {siteKey ? (
                <TurnstileWidget
                  ref={captchaRef}
                  siteKey={siteKey}
                  action="register"
                  onToken={setCaptchaToken}
                  onError={() => setError(t("register.errorCaptchaFailed"))}
                />
              ) : null}
              {error && <p className="text-sm text-error-500">{error}</p>}
              <Button type="submit" className="w-full" disabled={loading || siteKey === null || Boolean(siteKey && !captchaToken)}>
                {loading ? t("register.submitting") : t("register.submit")}
              </Button>
              <p className="text-center text-sm text-muted-foreground">
                {t("register.hasAccount")}{" "}
                <Button
                  variant="link"
                  className="p-0"
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
                </Button>
              </p>
              <p className="text-center text-sm text-muted-foreground">
                <Button
                  type="button"
                  variant="link"
                  className="p-0"
                  onClick={() => {
                    const current = (lockEmail ? invitedEmail : email).trim();
                    const params = new URLSearchParams();
                    if (current.includes("@")) params.set("email", current);
                    const qs = params.toString();
                    navigate(qs ? `/forgot-password?${qs}` : "/forgot-password");
                  }}
                >
                  {t("register.forgotPassword")}
                </Button>
              </p>
            </form>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
