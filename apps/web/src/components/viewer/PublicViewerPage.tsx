import { useCallback, useEffect, useState } from "react";
import { useParams } from "react-router";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { api } from "@/lib/api";
import { CanvasViewer } from "./CanvasViewer";
import type { Document } from "@/types";

interface AccessResult {
  link: { id: string; name?: string; documentId: string; permissionType: string; downloadEnabled: boolean; watermarkEnabled: boolean };
  document: { id: string; title: string; pageCount: number; status: string; sourceType: string; fileSize: number };
  visitorId: string;
  requiresEmail: boolean;
  requiresPassword: boolean;
  requiresNda: boolean;
}

export function PublicViewerPage() {
  const { token } = useParams<{ token: string }>();
  const { t } = useTranslation("documents");
  const [access, setAccess] = useState<AccessResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [ndaAgreed, setNdaAgreed] = useState(false);
  const [gate, setGate] = useState<{ email: boolean; password: boolean; nda: boolean }>({
    email: false,
    password: false,
    nda: false,
  });

  const tryAccess = useCallback(async (gateParams?: { email?: string; password?: string; ndaAgreed?: boolean }) => {
    if (!token) return;
    setLoading(true);
    setError(null);
    try {
      const res = await api.accessPublicLink(token, gateParams);
      setAccess(res);
      setGate({
        email: res.requiresEmail && !gateParams?.email,
        password: res.requiresPassword && !gateParams?.password,
        nda: res.requiresNda && !gateParams?.ndaAgreed,
      });
    } catch (e) {
      const err = e as { code?: string; message?: string };
      if (err.code === "requires_email" || err.code === "requires_password" || err.code === "nda_required") {
        setGate({
          email: err.code === "requires_email",
          password: err.code === "requires_password",
          nda: err.code === "nda_required",
        });
      } else {
        setError(err.message ?? t("viewer.loadFailed"));
      }
    } finally {
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

  if (!access || gate.email || gate.password || gate.nda) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-muted/30 p-6">
        <Card className="w-full max-w-md">
          <CardHeader>
            <CardTitle>{t("viewer.gateTitle")}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {gate.email && (
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
            {gate.password && (
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
            {gate.nda && (
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
              onClick={() =>
                void tryAccess({
                  email: gate.email ? email : undefined,
                  password: gate.password ? password : undefined,
                  ndaAgreed: gate.nda ? ndaAgreed : undefined,
                })
              }
            >
              {t("viewer.continue")}
            </Button>
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
    <div className="flex min-h-screen flex-col">
      <CanvasViewer
        publicToken={token}
        publicLink={access.link}
        publicDocument={doc}
        publicVisitorId={access.visitorId}
        watermark={access.link.watermarkEnabled ? { email: email || access.visitorId } : null}
      />
    </div>
  );
}
