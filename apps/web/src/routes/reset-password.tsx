import { useState } from "react";
import { useNavigate, useParams } from "react-router";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
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
      <div className="flex min-h-[100dvh] flex-col items-center justify-center bg-background p-6">
        <div className="w-full max-w-md">
          <Card>
            <CardHeader>
              <CardTitle className="text-h2">{t("resetPassword.title")}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <p className="text-sm text-error-500">{t("resetPassword.errorInvalidToken")}</p>
              <Button className="w-full" onClick={() => navigate("/forgot-password")}>
                {t("resetPassword.requestNew")}
              </Button>
            </CardContent>
          </Card>
        </div>
      </div>
    );
  }

  return (
    <div className="flex min-h-[100dvh] flex-col items-center justify-center bg-background p-6">
      <div className="w-full max-w-md">
        <Card>
          <CardHeader>
            <CardTitle className="text-h2">{t("resetPassword.title")}</CardTitle>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSubmit} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="password">{t("resetPassword.password")}</Label>
                <Input
                  id="password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder={t("resetPassword.passwordPlaceholder")}
                  autoComplete="new-password"
                  required
                />
                <p className="text-caption text-muted-foreground">{t("register.passwordRules")}</p>
              </div>
              <div className="space-y-2">
                <Label htmlFor="confirm">{t("resetPassword.confirm")}</Label>
                <Input
                  id="confirm"
                  type="password"
                  value={confirm}
                  onChange={(e) => setConfirm(e.target.value)}
                  placeholder={t("resetPassword.confirmPlaceholder")}
                  autoComplete="new-password"
                  required
                />
              </div>
              {error ? <p className="text-sm text-error-500">{error}</p> : null}
              <Button type="submit" className="w-full" disabled={loading}>
                {loading ? t("resetPassword.submitting") : t("resetPassword.submit")}
              </Button>
              <Button
                type="button"
                variant="link"
                className="w-full"
                onClick={() => navigate("/forgot-password")}
              >
                {t("resetPassword.requestNew")}
              </Button>
            </form>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
