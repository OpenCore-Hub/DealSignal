import { useState } from "react";
import { useNavigate } from "react-router";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { api } from "@/lib/api";

export function RegisterPage() {
  const navigate = useNavigate();
  const { t } = useTranslation("auth");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    const trimmedEmail = email.trim();
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

    try {
      await api.register(trimmedEmail, password);
      navigate("/login?registered=true", { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : t("register.errorRegistrationFailed"));
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
            <form onSubmit={handleSubmit} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="email">{t("register.email")}</Label>
                <Input
                  id="email"
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder={t("register.emailPlaceholder")}
                  required
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="password">{t("register.password")}</Label>
                <Input
                  id="password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder={t("register.passwordPlaceholder")}
                  required
                />
                <p className="text-caption text-muted-foreground">
                  {t("register.passwordRules")}
                </p>
              </div>
              {error && <p className="text-sm text-error-500">{error}</p>}
              <Button type="submit" className="w-full" disabled={loading}>
                {loading ? t("register.submitting") : t("register.submit")}
              </Button>
              <p className="text-center text-sm text-muted-foreground">
                {t("register.hasAccount")}{" "}
                <Button variant="link" className="p-0" onClick={() => navigate("/login")}>
                  {t("register.signIn")}
                </Button>
              </p>
            </form>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
