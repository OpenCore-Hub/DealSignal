import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useParams, useNavigate, useSearchParams } from "react-router";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Prohibit } from "@phosphor-icons/react";
import { toast } from "sonner";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { api, type PublicLinkCredentials } from "@/lib/api";
import { ApiError, setLinkSessionRefreshHandler } from "@/lib/apiClient";
import { CanvasViewer } from "./CanvasViewer";
import { RightSidebar } from "./RightSidebar";
import { PublicDealRoomLinkViewer } from "./PublicDealRoomLinkViewer";
import type { Document } from "@/types";

interface PublicDocumentSummary {
  id: string;
  title: string;
  pageCount: number;
  sourceType: string;
  folderPath?: string;
}

interface AccessResult {
  link: { id: string; name?: string; permissionType: string; downloadEnabled: boolean; watermarkEnabled: boolean; aiCopilotEnabled: boolean; qaEnabled: boolean; fileRequestsEnabled: boolean; isBundle: boolean; dealRoomId?: string };
  documents: PublicDocumentSummary[];
  visitorId: string;
  requiresEmail: boolean;
  requiresEmailVerification: boolean;
  requiresPassword: boolean;
  requiresNda: boolean;
  sessionToken: string;
}

export function PublicViewerPage() {
  const { token } = useParams<{ token: string }>();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { t } = useTranslation("documents");
  const [access, setAccess] = useState<AccessResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const prefilledEmail = searchParams.get("email") ?? "";
  const inviteToken = searchParams.get("inviteToken") ?? undefined;
  const [email, setEmail] = useState(prefilledEmail);
  const [emailCode, setEmailCode] = useState("");
  const [password, setPassword] = useState("");
  const [ndaAgreed, setNdaAgreed] = useState(true);
  const [accessCredentials, setAccessCredentials] = useState<PublicLinkCredentials>({});
  const [security, setSecurity] = useState<{ email: boolean; emailVerification: boolean; password: boolean; nda: boolean }>({
    email: false,
    emailVerification: false,
    password: false,
    nda: false,
  });
  const [gateError, setGateError] = useState<string | null>(null);
  const [linkErrorCode, setLinkErrorCode] = useState<string | null>(null);
  const [selectedDocIndex, setSelectedDocIndex] = useState(0);
  const [folderView, setFolderView] = useState(false);
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [requestEmail, setRequestEmail] = useState(prefilledEmail);
  const [requestReason, setRequestReason] = useState("");
  const [requestLoading, setRequestLoading] = useState(false);
  const [requestSubmitted, setRequestSubmitted] = useState(false);
  const [showAccessRequestForm, setShowAccessRequestForm] = useState(false);
  const [requestError, setRequestError] = useState<string | null>(null);
  const accessingRef = useRef(false);
  const sessionCheckedRef = useRef(false);

  // Persisted session: on successful access, the sessionToken is stored so
  // that re-visits within the session lifetime skip the credential prompt.
  const sessionKey = token ? `link-session:${token}` : null;

  const tryAccess = useCallback(async (gateParams?: { email?: string; emailCode?: string; password?: string; ndaAgreed?: boolean; sessionToken?: string; inviteToken?: string }) => {
    if (!token || accessingRef.current) return;
    accessingRef.current = true;
    setLoading(true);
    setError(null);
    setGateError(null);
    setLinkErrorCode(null);
    try {
      const res = await api.accessPublicLink(token, gateParams);
      setAccess(res);
      setSelectedDocIndex(0);
      setAccessCredentials({
        email: gateParams?.email,
        emailCode: gateParams?.emailCode,
        password: gateParams?.password,
        ndaAgreed: gateParams?.ndaAgreed,
        sessionToken: res.sessionToken,
      });
      // Persist session for re-visits within the session lifetime.
      if (sessionKey) {
        try {
          sessionStorage.setItem(sessionKey, res.sessionToken);
        } catch { /* ignore quota errors */ }
      }
      // The backend always returns the link's configured security gates.
      // Persist them so the UI does not flip between sequential prompts.
      setSecurity({
        email: res.requiresEmail,
        emailVerification: res.requiresEmailVerification,
        password: res.requiresPassword,
        nda: res.requiresNda,
      });
    } catch (e) {
      const err = e as ApiError;
      // On session expiry or invalidity, clear stored token so the next
      // revisit falls through to the credential gate.
      if (gateParams?.sessionToken && sessionKey) {
        try {
          sessionStorage.removeItem(sessionKey);
        } catch { /* ignore */ }
      }
      // The backend enriches gate errors with the link's full security config.
      // Render every configured control on the first response.
      setSecurity({
        email: err.requiresEmail ?? false,
        emailVerification: err.requiresEmailVerification ?? false,
        password: err.requiresPassword ?? false,
        nda: err.requiresNda ?? false,
      });
      const unavailableCodes = new Set([
        "link_not_found",
        "link_expired",
        "link_revoked",
        "link_disabled",
        "link_max_access_reached",
        "blocked_email",
        "blocked_domain",
        "not_allowed",
        "invite_expired",
        "invite_revoked",
      ]);
      if (unavailableCodes.has(err.code)) {
        setLinkErrorCode(err.code);
      } else if (
        err.code === "invalid_password" ||
        err.code === "whitelist_denied" ||
        err.code === "invalid_email_code"
      ) {
        // Only show raw backend messages for actual validation failures.
        // Missing required fields are handled by client-side checks on Continue.
        setGateError(err.message ?? t("common:error.loadFailed"));
      }
    } finally {
      accessingRef.current = false;
      setLoading(false);
    }
  }, [token, t, sessionKey]);

  useEffect(() => {
    // On mount, try the persisted session first (skip credentials on re-visit).
    // If the session is valid, the backend returns access immediately.
    // If it expired or was revoked, the error path clears the stored token
    // and we fall through to the empty tryAccess call below.
    if (sessionKey && !sessionCheckedRef.current) {
      sessionCheckedRef.current = true;
      let storedSession: string | null = null;
      try {
        storedSession = sessionStorage.getItem(sessionKey);
      } catch { /* ignore */ }
      if (storedSession) {
        void tryAccess({ sessionToken: storedSession, inviteToken });
        return;
      }
    }
     
    void tryAccess({ inviteToken });
  }, [token, tryAccess, sessionKey, inviteToken]);

  // Sliding session: when the backend returns X-Link-Session-Refresh on any
  // API response (page signed-URL, download-URL, etc.), update sessionStorage
  // and accessCredentials so the 15-min idle timeout keeps resetting while
  // the visitor is actively viewing pages.
  useEffect(() => {
    if (!sessionKey) return;
    setLinkSessionRefreshHandler((refreshedToken: string) => {
      try { sessionStorage.setItem(sessionKey, refreshedToken); } catch { /* ignore */ }
      setAccessCredentials(prev => prev ? { ...prev, sessionToken: refreshedToken } : prev);
    });
    return () => { setLinkSessionRefreshHandler(null); };
  }, [sessionKey]);

  const selectedDoc = useMemo(() => {
    return access?.documents[selectedDocIndex] ?? access?.documents[0];
  }, [access, selectedDocIndex]);

  const doc: Document = useMemo(() => {
    if (!selectedDoc) {
      return {
        id: "", title: "", sourceType: "pdf", fileName: "", fileType: "pdf",
        fileSize: 0, pageCount: 0, status: "ready", createdAt: "", updatedAt: "",
      } as Document;
    }
    return {
      id: selectedDoc.id,
      title: selectedDoc.title,
      sourceType: selectedDoc.sourceType.toLowerCase() as Document["sourceType"],
      fileName: selectedDoc.title,
      fileType: selectedDoc.sourceType.toLowerCase() as Document["fileType"],
      fileSize: 0,
      pageCount: selectedDoc.pageCount,
      status: "ready" as Document["status"],
      createdAt: "",
      updatedAt: "",
    };
  }, [selectedDoc]);

  const toggleSidebar = useCallback(() => setSidebarOpen((v) => !v), []);

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

  if (linkErrorCode) {
    const requestableErrorCodes = new Set(["blocked_email", "blocked_domain", "not_allowed"]);
    const canRequestAccess = requestableErrorCodes.has(linkErrorCode);
    return (
      <div className="flex min-h-screen items-center justify-center bg-muted/30 p-6">
        <Card className="w-full max-w-md">
          <CardHeader className="items-center space-y-3">
            <div className="flex h-12 w-12 items-center justify-center rounded-full bg-muted">
              <Prohibit size={24} className="text-muted-foreground" />
            </div>
            <CardTitle className="text-center">{t(`viewer.${linkErrorCode}Title`)}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4 text-center">
            <p className="text-muted-foreground">{t(`viewer.${linkErrorCode}Description`)}</p>

            {canRequestAccess && !requestSubmitted && !showAccessRequestForm && (
              <Button
                className="w-full"
                onClick={() => {
                  setShowAccessRequestForm(true);
                  setRequestError(null);
                }}
              >
                {t("viewer.requestAccess")}
              </Button>
            )}

            {canRequestAccess && showAccessRequestForm && !requestSubmitted && (
              <div className="space-y-4 text-left">
                <p className="text-sm text-muted-foreground">{t("viewer.requestAccessDescription")}</p>
                {requestError && (
                  <p className="text-sm text-destructive" role="alert">
                    {requestError}
                  </p>
                )}
                <div className="space-y-2">
                  <Label htmlFor="request-email">{t("viewer.requestAccessEmailLabel")}</Label>
                  <Input
                    id="request-email"
                    type="email"
                    value={requestEmail}
                    onChange={(e) => setRequestEmail(e.target.value)}
                    placeholder={t("viewer.requestAccessEmailPlaceholder")}
                    disabled={requestLoading}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="request-reason">{t("viewer.requestAccessReasonLabel")}</Label>
                  <Input
                    id="request-reason"
                    type="text"
                    value={requestReason}
                    onChange={(e) => setRequestReason(e.target.value)}
                    placeholder={t("viewer.requestAccessReasonPlaceholder")}
                    disabled={requestLoading}
                  />
                </div>
                <Button
                  className="w-full"
                  disabled={requestLoading}
                  onClick={() => {
                    setRequestError(null);
                    const trimmed = requestEmail.trim();
                    if (!trimmed || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(trimmed)) {
                      setRequestError(t("viewer.requestAccessEmailRequired"));
                      return;
                    }
                    if (!token) return;
                    setRequestLoading(true);
                    api
                      .createLinkAccessRequest(token, { email: trimmed, reason: requestReason.trim() || undefined })
                      .then(() => {
                        setRequestSubmitted(true);
                        setShowAccessRequestForm(false);
                        toast.success(t("viewer.requestAccessSubmitted"));
                      })
                      .catch((e: ApiError) => {
                        if (e.code === "access_request_exists") {
                          setRequestError(t("viewer.requestAccessExists"));
                        } else {
                          setRequestError(t("viewer.requestAccessFailed"));
                        }
                      })
                      .finally(() => setRequestLoading(false));
                  }}
                >
                  {requestLoading ? t("common:loading") : t("viewer.requestAccessSubmit")}
                </Button>
              </div>
            )}

            {canRequestAccess && requestSubmitted && (
              <div className="rounded-md border border-border bg-muted/50 p-3 text-sm text-muted-foreground">
                {t("viewer.requestAccessSubmitted")}
              </div>
            )}

            <Button variant="outline" className="w-full" onClick={() => navigate("/")}>
              {t("common:backToHome")}
            </Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  if (access?.link.dealRoomId && folderView) {
    return (
      <PublicDealRoomLinkViewer
        linkName={access.link.name}
        documents={access.documents}
        onViewDocument={(documentId) => {
          const idx = access.documents.findIndex((d) => d.id === documentId);
          if (idx >= 0) {
            setSelectedDocIndex(idx);
          }
          setFolderView(false);
        }}
      />
    );
  }

  if (!access) {
    const hasGates = security.email || security.emailVerification || security.password || security.nda;
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
            {inviteToken && (
              <div className="rounded-md border border-border bg-muted/50 p-3 text-sm">
                {prefilledEmail
                  ? t("viewer.inviteVerificationFor", { email: prefilledEmail })
                  : t("viewer.inviteVerification")}
              </div>
            )}
            {security.email && !inviteToken && (
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
            {security.emailVerification && (
              <div className="space-y-2">
                <Label htmlFor="email-code">{t("viewer.codeLabel")}</Label>
                <Input
                  id="email-code"
                  type="text"
                  inputMode="numeric"
                  value={emailCode}
                  onChange={(e) => setEmailCode(e.target.value)}
                  placeholder={t("viewer.codePlaceholder")}
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
                const accessEmail = prefilledEmail || email;
                if (security.email && !inviteToken && !accessEmail.trim()) {
                  setGateError(t("viewer.emailRequired"));
                  return;
                }
                if (security.emailVerification && !emailCode.trim()) {
                  setGateError(t("viewer.codeRequired"));
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
                  email: security.email ? accessEmail : undefined,
                  emailCode: security.emailVerification ? emailCode : undefined,
                  password: security.password ? password : undefined,
                  ndaAgreed: security.nda ? ndaAgreed : undefined,
                  inviteToken,
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

  return (
    <div className="flex h-[100dvh] flex-col overflow-hidden">
      {access.link.dealRoomId && (
        <div className="flex items-center gap-3 border-b bg-background px-4 py-2">
          <Button variant="ghost" size="sm" onClick={() => setFolderView(true)}>
            {t("common:back")}
          </Button>
          <span className="text-sm font-medium truncate">
            {selectedDoc?.title}
          </span>
        </div>
      )}
      <CanvasViewer
        publicToken={token}
        publicLink={access.link}
        publicDocument={doc}
        publicVisitorId={access.visitorId}
        publicAccessCredentials={accessCredentials}
        watermark={access.link.watermarkEnabled ? { email: accessCredentials.email || email || access.visitorId } : null}
        sidebarOpen={sidebarOpen}
        onToggleSidebar={toggleSidebar}
        sidebar={
          <RightSidebar
            open={sidebarOpen}
            onClose={() => setSidebarOpen(false)}
            documents={access.documents}
            selectedDocIndex={selectedDocIndex}
            onSelectDoc={setSelectedDocIndex}
            activeDocumentId={selectedDoc?.id}
            aiCopilotEnabled={access.link.aiCopilotEnabled}
            qaEnabled={access.link.qaEnabled}
            fileRequestsEnabled={access.link.fileRequestsEnabled}
            publicToken={token}
            publicSessionToken={accessCredentials.sessionToken}
          />
        }
      />
    </div>
  );
}
