import { useEffect, useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { api } from "@/lib/api";
import { safeAuthRedirect } from "@/lib/inviteAuth";

function VerifyEmailCard({
  status,
  message,
  onContinue,
}: {
  status: "loading" | "success" | "error";
  message: string;
  onContinue: () => void;
}) {
  const { t } = useTranslation("auth");

  const title =
    status === "success"
      ? t("verifyEmail.success")
      : status === "error"
        ? t("verifyEmail.error")
        : t("verifyEmail.verifying");

  const messageClass =
    status === "success"
      ? "text-green-600 dark:text-green-400"
      : status === "error"
        ? "text-error-500"
        : "text-muted-foreground";

  return (
    <div className="flex min-h-[100dvh] flex-col items-center justify-center bg-background p-6">
      <div className="w-full max-w-md">
        <Card>
          <CardHeader>
            <CardTitle className="text-h2">{title}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-6">
            <p className={`text-sm ${messageClass}`}>{message}</p>
            {status !== "loading" && (
              <Button onClick={onContinue} className="w-full">
                {status === "success" ? t("verifyEmail.continue") : t("register.signIn")}
              </Button>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

export function VerifyEmailPage() {
  const { token } = useParams<{ token: string }>();
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const { t } = useTranslation("auth");
  const [status, setStatus] = useState<"loading" | "success" | "error">("loading");
  const [message, setMessage] = useState(t("verifyEmail.verifying"));
  const redirect = safeAuthRedirect(searchParams.get("redirect"));

  useEffect(() => {
    if (!token) {
      return;
    }
    let cancelled = false;
    api
      .verifyEmail(token)
      .then((res) => {
        if (cancelled) return;
        setStatus(res.code === "verified" ? "success" : "error");
        setMessage(res.code === "verified" ? t("verifyEmail.success") : t("verifyEmail.error"));
      })
      .catch(() => {
        if (cancelled) return;
        setStatus("error");
        setMessage(t("verifyEmail.error"));
      });

    return () => {
      cancelled = true;
    };
  }, [token, t]);

  const onContinue = () => {
    if (status === "success") {
      navigate(redirect ?? "/", { replace: true });
      return;
    }
    navigate("/login");
  };

  if (!token) {
    return <VerifyEmailCard status="error" message={t("verifyEmail.error")} onContinue={onContinue} />;
  }

  return <VerifyEmailCard status={status} message={message} onContinue={onContinue} />;
}
