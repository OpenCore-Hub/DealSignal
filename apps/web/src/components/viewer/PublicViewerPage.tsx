import { useCallback, useEffect, useRef, useState } from "react";
import { useParams } from "react-router";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { api, type PublicLinkCredentials } from "@/lib/api";
import { ApiError } from "@/lib/apiClient";
import { CanvasViewer } from "./CanvasViewer";
import type { Document } from "@/types";

interface AccessResult {
  link: { id: string; name?: string; documentId: string; permissionType: string; downloadEnabled: boolean; watermarkEnabled: boolean };
  document: { id: string; title: string; pageCount: number; status: string; sourceType: string; fileSize: number };
  visitorId: string;
  requiresEmail: boolean;
  requiresPassword: boolean;
  requiresNda: boolean;
  sessionToken: string;
}

export function PublicViewerPage() {
  const { token } = useParams<{ token: string }>();
  const { t } = useTranslation("documents");
  const [access, setAccess] = useState<AccessResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [ndaAgreed, setNdaAgreed] = useState(true);
  const [accessCredentials, setAccessCredentials] = useState<PublicLinkCredentials>({});
  const [security, setSecurity] = useState<{ email: boolean; password: boolean; nda: boolean }>({
    email: false,
    password: false,
    nda: false,
  });
  const [gateError, setGateError] = useState<string | null>(null);
  const accessingRef = useRef(false);

  const tryAccess = useCallback(async (gateParams?: { email?: string; password?: string; ndaAgreed?: boolean }) => {
    if (!token || accessingRef.current) return;
    accessingRef.current = true;
    setLoading(true);
    setError(null);
    setGateError(null);
    try {
      const res = await api.accessPublicLink(token, gateParams);
      setAccess(res);
      setAccessCredentials({
        email: gateParams?.email,
        password: gateParams?.password,
        ndaAgreed: gateParams?.ndaAgreed,
        sessionToken: res.sessionToken,
      });
      // The backend always returns the link's configured security gates.
      // Persist them so the UI does not flip between sequential prompts.
      setSecurity({
        email: res.requiresEmail,
        password: res.requiresPassword,
        nda: res.requiresNda,
      });
    } catch (e) {
      const err = e as ApiError;
      // The backend enriches gate errors with the link's full security config.
      // Render every configured control on the first response.
      setSecurity({
        email: err.requiresEmail ?? false,
        password: err.requiresPassword ?? false,
        nda: err.requiresNda ?? false,
      });
      // Only show raw backend messages for actual validation failures.
      // Missing required fields are handled by client-side checks on Continue.
      if (err.code === "invalid_password" || err.code === "whitelist_denied") {
        setGateError(err.message ?? t("common:error.loadFailed"));
      }
    } finally {
      accessingRef.current = false;
      setLoading(false);
    }
  }, [token, t]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void tryAccess();
  }, [token, tryAccess]);

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p className="text-muted-foreground">{t("viewer.loading")}</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center gap-4 p-6">
        <p className="text-destructive">{error}</p>
      </div>
    );
  }

  if (!access) {
    const hasGates = security.email || security.password || security.nda;
    return (
      <div className="flex min-h-screen items-center justify-center bg-muted/30 p-6">
        <Card className="w-full max-w-md">
          <CardHeader>
            <CardTitle>{t("viewer.gateTitle")}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {gateError && (
              <p className="text-sm text-destructive" role="alert">
                {gateError}
              </p>
            )}
            {security.email && (
              <div className="space-y-2">
                <Label htmlFor="email">{t("viewer.emailLabel")}</Label>
                <Input
                  id="email"
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder={t("viewer.emailPlaceholder")}
                />
              </div>
            )}
            {security.password && (
              <div className="space-y-2">
                <Label htmlFor="password">{t("viewer.passwordLabel")}</Label>
                <Input
                  id="password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder={t("viewer.passwordPlaceholder")}
                />
              </div>
            )}
            {security.nda && (
              <div className="flex items-center gap-2">
                <Checkbox
                  id="nda"
                  checked={ndaAgreed}
                  onCheckedChange={(checked) => setNdaAgreed(checked === true)}
                />
                <Label htmlFor="nda">{t("viewer.ndaLabel")}</Label>
              </div>
            )}
            <Button
              className="w-full"
              onClick={() => {
                setGateError(null);
                if (security.email && !email.trim()) {
                  setGateError(t("viewer.emailRequired"));
                  return;
                }
                if (security.password && !password) {
                  setGateError(t("viewer.passwordRequired"));
                  return;
                }
                if (security.nda && !ndaAgreed) {
                  setGateError(t("viewer.ndaRequired"));
                  return;
                }
                void tryAccess({
                  email: security.email ? email : undefined,
                  password: security.password ? password : undefined,
                  ndaAgreed: security.nda ? ndaAgreed : undefined,
                });
              }}
            >
              {t("viewer.continue")}
            </Button>
            {!hasGates && (
              <p className="text-sm text-muted-foreground">{t("common:error.loadFailed")}</p>
            )}
          </CardContent>
        </Card>
      </div>
    );
  }

  const doc: Document = {
    id: access.document.id,
    title: access.document.title,
    sourceType: access.document.sourceType.toLowerCase() as Document["sourceType"],
    fileName: access.document.title,
    fileType: access.document.sourceType.toLowerCase() as Document["fileType"],
    fileSize: access.document.fileSize,
    pageCount: access.document.pageCount,
    status: access.document.status as Document["status"],
    createdAt: "",
    updatedAt: "",
  };

  return (
    <div className="flex h-[100dvh] flex-col overflow-hidden">
      <CanvasViewer
        publicToken={token}
        publicLink={access.link}
        publicDocument={doc}
        publicVisitorId={access.visitorId}
        publicAccessCredentials={accessCredentials}
        watermark={access.link.watermarkEnabled ? { email: accessCredentials.email || email || access.visitorId } : null}
      />
    </div>
  );
}
