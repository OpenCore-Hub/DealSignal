import { useNavigate, useSearchParams } from "react-router";
import { useTranslation } from "react-i18next";
import { AuthStage, AuthTextLink, railFromNamespace } from "@/components/auth/AuthStage";
import { ResendVerification } from "@/components/auth/ResendVerification";
import { inviteEmailFromSearchParams, isInviteAuthFlow, safeAuthRedirect } from "@/lib/inviteAuth";

export function CheckEmailPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { t } = useTranslation("auth");
  const email = inviteEmailFromSearchParams(searchParams) || (searchParams.get("email") ?? "").trim();

  return (
    <AuthStage
      rail={railFromNamespace(t, "stage")}
      kicker={t("checkEmail.kicker")}
      title={t("checkEmail.title")}
      lede={email ? t("checkEmail.bodyWithEmail", { email }) : t("checkEmail.body")}
      footer={
        <p>
          <AuthTextLink
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
          </AuthTextLink>
        </p>
      }
    >
      {email ? <div className="mt-8"><ResendVerification email={email} /></div> : null}
    </AuthStage>
  );
}
