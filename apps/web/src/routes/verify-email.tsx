import { useEffect, useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router";
import { useTranslation } from "react-i18next";
import { AuthStage, AuthSubmit, railFromNamespace } from "@/components/auth/AuthStage";
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

  return (
    <AuthStage
      rail={railFromNamespace(t, "stage")}
      kicker={t("verifyEmail.kicker")}
      title={title}
      lede={status === "loading" ? message : undefined}
    >
      {status !== "loading" ? <p className={status === "error" ? "auth-error" : "auth-lede"}>{message}</p> : null}
      {status !== "loading" ? (
        <AuthSubmit type="button" onClick={onContinue}>
          {status === "success" ? t("verifyEmail.continue") : t("register.signIn")}
        </AuthSubmit>
      ) : null}
    </AuthStage>
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
