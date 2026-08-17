import { useNavigate, useSearchParams } from "react-router";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ResendVerification } from "@/components/auth/ResendVerification";
import { inviteEmailFromSearchParams, isInviteAuthFlow, safeAuthRedirect } from "@/lib/inviteAuth";

export function CheckEmailPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { t } = useTranslation("auth");
  const email = inviteEmailFromSearchParams(searchParams) || (searchParams.get("email") ?? "").trim();

  return (
    <div className="flex min-h-[100dvh] flex-col items-center justify-center bg-background p-6">
      <div className="w-full max-w-md">
        <Card>
          <CardHeader>
            <CardTitle className="text-h2">{t("checkEmail.title")}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <p className="text-sm text-muted-foreground">
              {email ? t("checkEmail.bodyWithEmail", { email }) : t("checkEmail.body")}
            </p>
            {email ? <ResendVerification email={email} /> : null}
            <Button
              variant="link"
              className="w-full"
              onClick={() => {
                const params = new URLSearchParams();
                if (email.includes("@")) params.set("email", email);
                const redirect = safeAuthRedirect(searchParams.get("redirect"));
                if (redirect) params.set("redirect", redirect);
                if (isInviteAuthFlow(searchParams)) params.set("invite", "1");
                const qs = params.toString();
                navigate(qs ? `/login?${qs}` : "/login");
              }}
            >
              {t("checkEmail.backToLogin")}
            </Button>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
