import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { api } from "@/lib/api";
import { ApiError } from "@/lib/apiClient";
import { apiErrorMessage } from "@/lib/apiErrors";
import { buildInviteAuthPath } from "@/lib/inviteAuth";

type InvitationPreview = Awaited<ReturnType<typeof api.previewDealRoomInvitation>>;
type AcceptInvitationResponse = Awaited<ReturnType<typeof api.acceptDealRoomInvitation>>;

/** Deduplicate concurrent accepts (React Strict Mode remount / double effect). */
const acceptInflight = new Map<string, Promise<AcceptInvitationResponse>>();

function acceptRoomInvitationOnce(token: string): Promise<AcceptInvitationResponse> {
  const existing = acceptInflight.get(token);
  if (existing) return existing;
  const promise = api.acceptDealRoomInvitation(token).finally(() => {
    acceptInflight.delete(token);
  });
  acceptInflight.set(token, promise);
  return promise;
}

function hasAuthSession(): boolean {
  try {
    return document.cookie.split(";").some((c) => c.trim().startsWith("auth_session="));
  } catch {
    return false;
  }
}

function emailsMatch(a: string, b: string): boolean {
  return a.trim().toLowerCase() === b.trim().toLowerCase();
}

function inviteAuthPath(mode: "login" | "register", token: string, email: string): string {
  return buildInviteAuthPath(mode, {
    redirect: `/room-invitations/${encodeURIComponent(token)}/accept`,
    email,
  });
}

function isInvitationEmailMismatch(err: unknown): boolean {
  return (
    err instanceof ApiError &&
    (err.code === "invitation_email_mismatch" || err.code === "email_mismatch")
  );
}

function roomPath(workspaceSlug: string, roomId: string): string {
  return `/${workspaceSlug}/deal-rooms/${roomId}`;
}

export function AcceptRoomInvitationPage() {
  const { token } = useParams<{ token: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation("auth");
  const [status, setStatus] = useState<"loading" | "ready" | "accepting" | "success" | "error">("loading");
  const [message, setMessage] = useState(t("acceptRoomInvitation.loading"));
  const [preview, setPreview] = useState<InvitationPreview | null>(null);
  const [accepted, setAccepted] = useState<AcceptInvitationResponse | null>(null);
  const [emailMismatch, setEmailMismatch] = useState(false);
  const [signedInEmail, setSignedInEmail] = useState<string | null>(null);
  const [switching, setSwitching] = useState(false);

  useEffect(() => {
    if (!token) {
      setStatus("error");
      setMessage(t("acceptRoomInvitation.error"));
      return;
    }

    let active = true;

    const run = async () => {
      let nextPreview: InvitationPreview;
      try {
        nextPreview = await api.previewDealRoomInvitation(token);
      } catch (err) {
        if (!active) return;
        setStatus("error");
        setEmailMismatch(false);
        setMessage(
          apiErrorMessage(err, {
            context: "acceptRoomInvitation",
            messageKey: "auth:acceptRoomInvitation.error",
          }),
        );
        return;
      }
      if (!active) return;
      setPreview(nextPreview);

      if (!hasAuthSession()) {
        if (nextPreview.status === "used") {
          setStatus("error");
          setMessage(t("acceptRoomInvitation.used"));
          return;
        }
        setStatus("ready");
        setMessage(
          t("acceptRoomInvitation.readyUnauthenticated", {
            workspace: nextPreview.workspaceName || nextPreview.workspaceSlug,
            room: nextPreview.roomName,
            email: nextPreview.email,
          }),
        );
        return;
      }

      let meEmail: string;
      try {
        const me = await api.getMe();
        meEmail = me.email ?? "";
      } catch {
        if (!active) return;
        navigate(inviteAuthPath("login", token, nextPreview.email), { replace: true });
        return;
      }
      if (!active) return;
      setSignedInEmail(meEmail);

      if (!emailsMatch(meEmail, nextPreview.email)) {
        setEmailMismatch(true);
        setStatus("error");
        setMessage(
          t("acceptRoomInvitation.emailMismatchDetail", {
            signedIn: meEmail,
            invited: nextPreview.email,
          }),
        );
        return;
      }

      setStatus("accepting");
      setMessage(t("acceptRoomInvitation.accepting"));
      try {
        const res = await acceptRoomInvitationOnce(token);
        if (!active) return;
        setEmailMismatch(false);
        setAccepted(res);
        setStatus("success");
        setMessage(t("acceptRoomInvitation.success", { room: res.roomName || nextPreview.roomName }));
        window.setTimeout(() => {
          if (!active) return;
          navigate(roomPath(res.workspaceSlug, res.roomId), { replace: true });
        }, 800);
      } catch (err) {
        if (!active) return;
        if (
          nextPreview.status === "used" &&
          err instanceof ApiError &&
          err.code === "invitation_used"
        ) {
          setStatus("error");
          setMessage(t("acceptRoomInvitation.used"));
          return;
        }
        setStatus("error");
        setEmailMismatch(isInvitationEmailMismatch(err));
        setMessage(
          apiErrorMessage(err, {
            context: "acceptRoomInvitation",
            messageKey: "auth:acceptRoomInvitation.error",
          }),
        );
      }
    };

    void run();
    return () => {
      active = false;
    };
  }, [token, navigate, t]);

  const switchToInvitedAccount = async () => {
    if (!token || !preview || switching) return;
    setSwitching(true);
    try {
      await api.logout();
    } catch {
      // Continue even if logout fails.
    }
    navigate(inviteAuthPath("login", token, preview.email), { replace: true });
  };

  const title =
    status === "success"
      ? t("acceptRoomInvitation.successTitle")
      : status === "error"
        ? t("acceptRoomInvitation.errorTitle")
        : t("acceptRoomInvitation.title");

  return (
    <div className="flex min-h-[100dvh] flex-col items-center justify-center bg-background p-6">
      <div className="w-full max-w-md">
        <Card>
          <CardHeader>
            <CardTitle className="text-h2">{title}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-6">
            {preview && status !== "loading" ? (
              <div className="space-y-1 text-sm text-muted-foreground">
                <p>
                  {t("acceptRoomInvitation.workspaceLabel", {
                    workspace: preview.workspaceName || preview.workspaceSlug,
                  })}
                </p>
                <p>{t("acceptRoomInvitation.roomLabel", { room: preview.roomName })}</p>
                <p>{t("acceptRoomInvitation.emailLabel", { email: preview.email })}</p>
                <p>
                  {preview.status === "used"
                    ? t("acceptRoomInvitation.statusUsed")
                    : t("acceptRoomInvitation.statusPending")}
                </p>
                {signedInEmail && emailMismatch ? (
                  <p>{t("acceptRoomInvitation.signedInLabel", { email: signedInEmail })}</p>
                ) : null}
              </div>
            ) : null}
            <p
              className={
                status === "success"
                  ? "text-sm text-green-600 dark:text-green-400"
                  : status === "error"
                    ? "text-sm text-error-500"
                    : "text-sm text-muted-foreground"
              }
            >
              {message}
            </p>
            {status === "ready" && token && preview ? (
              <div className="flex flex-col gap-2">
                <Button
                  className="w-full"
                  onClick={() => navigate(inviteAuthPath("register", token, preview.email))}
                >
                  {t("acceptRoomInvitation.createAccount")}
                </Button>
                <Button
                  variant="outline"
                  className="w-full"
                  onClick={() => navigate(inviteAuthPath("login", token, preview.email))}
                >
                  {t("acceptRoomInvitation.signIn")}
                </Button>
              </div>
            ) : null}
            {status === "error" ? (
              <div className="flex flex-col gap-2">
                {emailMismatch && preview ? (
                  <Button onClick={() => void switchToInvitedAccount()} disabled={switching} className="w-full">
                    {t("acceptRoomInvitation.switchAccount")}
                  </Button>
                ) : null}
                <Button
                  onClick={() => navigate("/")}
                  variant={emailMismatch ? "outline" : "default"}
                  className="w-full"
                >
                  {t("acceptRoomInvitation.backHome")}
                </Button>
              </div>
            ) : null}
            {status === "success" && accepted ? (
              <Button
                onClick={() => navigate(roomPath(accepted.workspaceSlug, accepted.roomId))}
                className="w-full"
              >
                {t("acceptRoomInvitation.openRoom")}
              </Button>
            ) : null}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
