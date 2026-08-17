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
    <div className="flex min-h-[100dvh] flex-col items-center justify-center bg-background p-6">
      <div className="w-full max-w-md">
        <Card>
          <CardHeader>
            <CardTitle className="text-h2">{t("forgotPassword.title")}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <p className="text-sm text-muted-foreground">{t("forgotPassword.body")}</p>
            {sent ? (
              <p className="text-sm text-muted-foreground">{t("forgotPassword.sent")}</p>
            ) : (
              <form onSubmit={handleSubmit} className="space-y-4">
                <div className="space-y-2">
                  <Label htmlFor="email">{t("forgotPassword.email")}</Label>
                  <Input
                    id="email"
                    type="email"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    placeholder={t("forgotPassword.emailPlaceholder")}
                    autoComplete="email"
                    required
                  />
                </div>
                {siteKey ? (
                  <TurnstileWidget
                    ref={captchaRef}
                    siteKey={siteKey}
                    action="register"
                    hintKey="forgotPassword.captchaHint"
                    onToken={setCaptchaToken}
                    onError={() => setError(t("register.errorCaptchaFailed"))}
                  />
                ) : null}
                {error ? <p className="text-sm text-error-500">{error}</p> : null}
                <Button
                  type="submit"
                  className="w-full"
                  disabled={loading || siteKey === null || Boolean(siteKey && !captchaToken)}
                >
                  {loading ? t("forgotPassword.submitting") : t("forgotPassword.submit")}
                </Button>
              </form>
            )}
            <Button variant="link" className="w-full" onClick={() => navigate("/login")}>
              {t("forgotPassword.backToLogin")}
            </Button>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
