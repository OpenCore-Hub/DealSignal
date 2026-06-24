import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { api } from "@/lib/api";

function VerifyEmailCard({ status, message }: { status: "loading" | "success" | "error"; message: string }) {
  const navigate = useNavigate();
  const title =
    status === "success" ? "Email verified" : status === "error" ? "Verification failed" : "Verifying email";
  const messageClass =
    status === "success" ? "text-green-600" : status === "error" ? "text-error-500" : "text-muted-foreground";

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
              <Button onClick={() => navigate("/login")} className="w-full">
                {status === "success" ? "Continue to sign in" : "Back to sign in"}
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
  const [status, setStatus] = useState<"loading" | "success" | "error">("loading");
  const [message, setMessage] = useState("Verifying your email address…");

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
        setMessage(res.message);
      })
      .catch((err) => {
        if (cancelled) return;
        setStatus("error");
        setMessage(err instanceof Error ? err.message : "Verification failed. The link may have expired.");
      });

    return () => {
      cancelled = true;
    };
  }, [token]);

  if (!token) {
    return <VerifyEmailCard status="error" message="Verification link is missing." />;
  }

  return <VerifyEmailCard status={status} message={message} />;
}
