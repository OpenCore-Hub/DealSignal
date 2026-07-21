import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useParams, useNavigate, useSearchParams } from "react-router";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Prohibit, WarningCircle } from "@phosphor-icons/react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Textarea } from "@/components/ui/textarea";
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
  link: { id: string; name?: string; permissionType: string; downloadEnabled: boolean; watermarkEnabled: boolean; watermarkText?: string; screenshotProtectionEnabled?: boolean; aiCopilotEnabled: boolean; qaEnabled: boolean; fileRequestsEnabled: boolean; isBundle: boolean; dealRoomId?: string };
  documents: PublicDocumentSummary[];
  visitorId: string;
  requiresEmail: boolean;
  requiresEmailVerification: boolean;
  requiresPassword: boolean;
  requiresNda: boolean;
  sessionToken: string;
}

type NdaGatePhase = "sign" | "review" | "credentials" | "complete";

function isValidDeliveryEmail(value: string): boolean {
  const emailValue = value.trim();
  if (emailValue.length < 3 || emailValue.length > 254) return false;
  const at = emailValue.lastIndexOf("@");
  if (at <= 0 || at === emailValue.length - 1) return false;
  const domain = emailValue.slice(at + 1);
  return domain.includes(".") && !domain.startsWith(".") && !domain.endsWith(".");
}

interface NdaIntent {
  signerName: string;
  ndaDeliveryEmail: string;
  ndaAgreed: boolean;
  phase: NdaGatePhase;
  /**
   * Tab-scoped: visitor submitted an access request and is waiting for owner
   * approval. Survives refresh so we show "request submitted" until the email
   * is allowlisted (then credentials / new verification code).
   */
  accessRequestPending?: boolean;
  /** Server-issued NDA binding; post-sign phases require a match. */
  ndaTemplateId?: string;
  contentSha256?: string;
}

interface NdaTemplateBinding {
  ndaTemplateId: string;
  contentSha256: string;
}

function ndaIntentStorageKey(token: string) {
  return `nda-intent:${token}`;
}

function isPostSignNdaPhase(phase: NdaGatePhase | undefined): boolean {
  return phase === "credentials" || phase === "review" || phase === "complete";
}

function intentMatchesNdaBinding(
  intent: NdaIntent,
  binding: NdaTemplateBinding | null
): boolean {
  if (!intent.ndaTemplateId) return false;
  if (!binding) return false;
  if (intent.ndaTemplateId !== binding.ndaTemplateId) return false;
  // Prefer hash match when both sides have a hash; empty hash is treated as unknown.
  if (intent.contentSha256 && binding.contentSha256) {
    return intent.contentSha256 === binding.contentSha256;
  }
  return true;
}

function loadNdaIntent(token: string): NdaIntent | null {
  try {
    const raw = sessionStorage.getItem(ndaIntentStorageKey(token));
    if (!raw) return null;
    const parsed = JSON.parse(raw) as Partial<NdaIntent>;
    if (!parsed.ndaDeliveryEmail) return null;
    // Non-NDA access-request pending may omit signer name.
    if (!parsed.signerName && !parsed.accessRequestPending) return null;
    const phase = isPostSignNdaPhase(parsed.phase as NdaGatePhase | undefined)
      ? (parsed.phase as NdaGatePhase)
      : "sign";
    // Post-sign / pending NDA flows must carry a server-issued template binding.
    // Legacy intents without binding are invalidated so review cannot be skipped.
    if (
      !parsed.accessRequestPending &&
      isPostSignNdaPhase(phase) &&
      !parsed.ndaTemplateId
    ) {
      return null;
    }
    return {
      signerName: parsed.signerName ?? "",
      ndaDeliveryEmail: parsed.ndaDeliveryEmail,
      ndaAgreed: Boolean(parsed.ndaAgreed),
      phase,
      ...(parsed.accessRequestPending ? { accessRequestPending: true } : {}),
      ...(parsed.ndaTemplateId
        ? {
            ndaTemplateId: parsed.ndaTemplateId,
            ...(parsed.contentSha256 ? { contentSha256: parsed.contentSha256 } : {}),
          }
        : {}),
    };
  } catch {
    return null;
  }
}

function saveNdaIntent(token: string, intent: NdaIntent) {
  try {
    sessionStorage.setItem(ndaIntentStorageKey(token), JSON.stringify(intent));
  } catch {
    /* ignore quota errors */
  }
}

function clearNdaIntent(token: string) {
  try {
    sessionStorage.removeItem(ndaIntentStorageKey(token));
  } catch {
    /* ignore */
  }
}

function clearAccessRequestPending(token: string) {
  const existing = loadNdaIntent(token);
  if (!existing?.accessRequestPending) return;
  const { accessRequestPending: _pending, ...rest } = existing;
  saveNdaIntent(token, rest);
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
  const [ndaAgreed, setNdaAgreed] = useState(false);
  const [signerName, setSignerName] = useState("");
  const [ndaDeliveryEmail, setNdaDeliveryEmail] = useState(prefilledEmail);
  const [ndaPreviewPageUrls, setNdaPreviewPageUrls] = useState<string[]>([]);
  const [ndaDocumentUrl, setNdaDocumentUrl] = useState<string | null>(null);
  const [ndaPreviewZoomed, setNdaPreviewZoomed] = useState(false);
  const [ndaBinding, setNdaBinding] = useState<NdaTemplateBinding | null>(null);
  // NDA gate phases: sign → 30s signed review → clean email/code page.
  const [ndaGatePhase, setNdaGatePhase] = useState<NdaGatePhase>("sign");
  const [ndaCountdown, setNdaCountdown] = useState(30);
  const [ndaSignedAt, setNdaSignedAt] = useState<Date | null>(null);
  const ndaReviewFinishedRef = useRef(false);
  const [accessCredentials, setAccessCredentials] = useState<PublicLinkCredentials>({});
  const [security, setSecurity] = useState<{ email: boolean; emailVerification: boolean; password: boolean; nda: boolean; isDealRoom: boolean }>({
    email: false,
    emailVerification: false,
    password: false,
    nda: false,
    isDealRoom: false,
  });
  const [gateError, setGateError] = useState<string | null>(null);
  const [gateErrorCode, setGateErrorCode] = useState<string | null>(null);
  /** Top-of-screen floating tip with 10s visual countdown (email deny / mismatch). */
  const [floatingTip, setFloatingTip] = useState<{ message: string; id: number } | null>(null);
  const [floatingTipProgress, setFloatingTipProgress] = useState(1);
  const showFloatingGateTip = useCallback((message: string) => {
    setFloatingTip({ message, id: Date.now() });
    setFloatingTipProgress(1);
  }, []);
  const [showAccessRequest, setShowAccessRequest] = useState(false);
  const [accessRequestReason, setAccessRequestReason] = useState("");
  const [accessRequestSubmitting, setAccessRequestSubmitting] = useState(false);
  const [accessRequestSubmitted, setAccessRequestSubmitted] = useState(false);
  const [ndaEmailChecking, setNdaEmailChecking] = useState(false);
  const [linkErrorCode, setLinkErrorCode] = useState<string | null>(null);
  const [selectedDocIndex, setSelectedDocIndex] = useState(0);
  const [folderView, setFolderView] = useState(false);
  const [sidebarOpen, setSidebarOpen] = useState(false);
  // Monotonic id so in-flight Access responses are ignored after a newer
  // tryAccess starts (token/invite change or double-submit). Replaces a
  // boolean lock that could drop the newer request entirely.
  const accessRequestIdRef = useRef(0);
  // Tracks which token+invite bootstrap already ran for this mount. Must not be a
  // boolean: when tryAccess identity churns (e.g. i18n `t` changes), a boolean
  // "already checked" path previously fell through to Access WITHOUT the stored
  // session and re-prompted email/code on every refresh-related re-render.
  const bootstrappedKeyRef = useRef<string | null>(null);
  const ndaIntentRestoredRef = useRef<string | null>(null);
  // Prevent session-failure → NDA auto-retry from looping when Access still fails.
  const ndaSessionRecoveryRef = useRef(false);
  /**
   * Set while advancing from a pending access-request probe into Access.
   * If Access still returns not_allowed/blocked, restore the submitted UI so we
   * do not permanently drop the pending marker after a flaky allowlist check.
   */
  const accessRequestPendingResumeRef = useRef(false);
  const tryAccessRef = useRef<(gateParams?: {
    email?: string;
    emailCode?: string;
    password?: string;
    ndaAgreed?: boolean;
    signerName?: string;
    sessionToken?: string;
    inviteToken?: string;
  }) => Promise<void>>(async () => {});

  // Persisted session: on successful access, the sessionToken is stored so
  // that re-visits within the session lifetime skip the credential prompt.
  // Note: this uses sessionStorage (tab-scoped), not an HTTP cookie.
  const sessionKey = token ? `link-session:${token}` : null;

  const persistNdaIntent = useCallback((
    phase: NdaGatePhase,
    opts?: { accessRequestPending?: boolean }
  ) => {
    if (!token) return;
    const existing = loadNdaIntent(token);
    const pending =
      opts && Object.prototype.hasOwnProperty.call(opts, "accessRequestPending")
        ? Boolean(opts.accessRequestPending)
        : Boolean(existing?.accessRequestPending);
    const binding = ndaBinding ?? (
      existing?.ndaTemplateId
        ? {
            ndaTemplateId: existing.ndaTemplateId,
            contentSha256: existing.contentSha256 ?? "",
          }
        : null
    );
    saveNdaIntent(token, {
      signerName: signerName.trim(),
      ndaDeliveryEmail: ndaDeliveryEmail.trim(),
      ndaAgreed,
      phase,
      ...(pending ? { accessRequestPending: true } : {}),
      ...(binding?.ndaTemplateId
        ? {
            ndaTemplateId: binding.ndaTemplateId,
            ...(binding.contentSha256 ? { contentSha256: binding.contentSha256 } : {}),
          }
        : {}),
    });
  }, [token, signerName, ndaDeliveryEmail, ndaAgreed, ndaBinding]);

  const applyNdaPreview = useCallback((preview: {
    ndaTemplate?: { id?: string; contentSha256?: string };
    previewPageUrls?: string[];
    previewImageUrl?: string;
    documentUrl?: string;
    previewUrl?: string;
  }) => {
    const pages =
      preview.previewPageUrls && preview.previewPageUrls.length > 0
        ? preview.previewPageUrls
        : preview.previewImageUrl
          ? [preview.previewImageUrl]
          : [];
    setNdaPreviewPageUrls(pages);
    setNdaDocumentUrl(preview.documentUrl || preview.previewUrl || null);

    const templateId = preview.ndaTemplate?.id?.trim() ?? "";
    if (!templateId) return;
    const nextBinding: NdaTemplateBinding = {
      ndaTemplateId: templateId,
      contentSha256: preview.ndaTemplate?.contentSha256?.trim() ?? "",
    };
    setNdaBinding(nextBinding);

    if (!token) return;
    const intent = loadNdaIntent(token);
    if (!intent) return;
    // Access-request-only pending without NDA can omit template binding.
    if (intent.accessRequestPending && !intent.ndaAgreed && !intent.ndaTemplateId) {
      return;
    }
    if (isPostSignNdaPhase(intent.phase) || intent.accessRequestPending) {
      if (!intentMatchesNdaBinding(intent, nextBinding)) {
        clearNdaIntent(token);
        setAccessRequestSubmitted(false);
        setNdaGatePhase("sign");
        setNdaAgreed(false);
      }
    }
  }, [token]);

  const tryAccess = useCallback(async (gateParams?: { email?: string; emailCode?: string; password?: string; ndaAgreed?: boolean; signerName?: string; sessionToken?: string; inviteToken?: string }) => {
    if (!token) return;
    const requestId = ++accessRequestIdRef.current;
    const requestToken = token;
    const requestSessionKey = sessionKey;
    setLoading(true);
    setError(null);
    setGateError(null);
    setGateErrorCode(null);
    setLinkErrorCode(null);
    try {
      const res = await api.accessPublicLink(requestToken, gateParams);
      if (requestId !== accessRequestIdRef.current) return;
      // Persist session before touching NDA intent so a quota/storage glitch
      // cannot leave the visitor with neither session nor completion marker.
      const nextSession =
        typeof res.sessionToken === "string" && res.sessionToken.length > 0
          ? res.sessionToken
          : undefined;
      if (requestSessionKey && nextSession) {
        try {
          sessionStorage.setItem(requestSessionKey, nextSession);
        } catch { /* ignore quota errors */ }
      }
      // Keep a short-lived NDA completion marker so refresh does not bounce
      // back to the sign page when session reuse fails (expired/rotated).
      if (res.requiresNda) {
        const existing = loadNdaIntent(requestToken);
        const completeIntent: NdaIntent = {
          signerName: (gateParams?.signerName ?? existing?.signerName ?? signerName).trim(),
          ndaDeliveryEmail: (gateParams?.email ?? existing?.ndaDeliveryEmail ?? ndaDeliveryEmail).trim(),
          ndaAgreed: true,
          phase: "complete",
        };
        if (completeIntent.signerName && completeIntent.ndaDeliveryEmail) {
          saveNdaIntent(requestToken, completeIntent);
        }
        // Stop the 30s review timer if Access completed while still in review.
        setNdaGatePhase("credentials");
      } else {
        clearNdaIntent(requestToken);
      }
      ndaSessionRecoveryRef.current = false;
      accessRequestPendingResumeRef.current = false;
      setAccess(res);
      setSelectedDocIndex(0);
      setAccessCredentials({
        email: gateParams?.email,
        emailCode: gateParams?.emailCode,
        password: gateParams?.password,
        ndaAgreed: gateParams?.ndaAgreed,
        sessionToken: nextSession,
      });
      // The backend always returns the link's configured security gates.
      // Persist them so the UI does not flip between sequential prompts.
      setSecurity({
        email: res.requiresEmail,
        emailVerification: res.requiresEmailVerification,
        password: res.requiresPassword,
        nda: res.requiresNda,
        isDealRoom: Boolean(res.link.dealRoomId),
      });
      } catch (e) {
      if (requestId !== accessRequestIdRef.current) return;
      const err = e as ApiError;
      // Pending-access resume: allowlist check passed but Access still denies —
      // put the visitor back on the submitted screen instead of dropping pending.
      if (
        accessRequestPendingResumeRef.current &&
        (err.code === "not_allowed" || err.code === "blocked_email")
      ) {
        accessRequestPendingResumeRef.current = false;
        saveNdaIntent(requestToken, {
          signerName: (gateParams?.signerName ?? signerName).trim(),
          ndaDeliveryEmail: (gateParams?.email ?? ndaDeliveryEmail).trim(),
          ndaAgreed: true,
          phase: "credentials",
          accessRequestPending: true,
        });
        setAccessRequestSubmitted(true);
        setShowAccessRequest(false);
        setGateError(null);
        setGateErrorCode(null);
        setLoading(false);
        return;
      }
      accessRequestPendingResumeRef.current = false;
      // On session expiry or invalidity, clear stored token so the next
      // revisit falls through to the credential gate. Do not wipe on
      // transient network failures — that would force a full NDA re-sign.
      if (gateParams?.sessionToken && requestSessionKey && err.code !== "network_error") {
        try {
          sessionStorage.removeItem(requestSessionKey);
        } catch { /* ignore */ }
      }
      // The backend enriches gate errors with the link's full security config.
      // Render every configured control on the first response. When a later
      // error response omits those flags (e.g. an internal_error or an unknown
      // code from the edge), preserve the previously known gates so inputs do
      // not vanish from the visitor.
      setSecurity((prev) => ({
        email: err.requiresEmail ?? prev.email,
        emailVerification: err.requiresEmailVerification ?? prev.emailVerification,
        password: err.requiresPassword ?? prev.password,
        nda: err.requiresNda ?? prev.nda,
        isDealRoom: err.isDealRoom ?? prev.isDealRoom,
      }));

      // Session reuse failed after the visitor already signed: never bounce to
      // the NDA sign page. Recover silently when NDA is the only remaining gate;
      // otherwise land on the credentials step with restored intent fields.
      if (
        gateParams?.sessionToken &&
        !ndaSessionRecoveryRef.current &&
        err.code !== "network_error"
      ) {
        const intent = loadNdaIntent(requestToken);
        if (intent && isPostSignNdaPhase(intent.phase) && intent.ndaAgreed) {
          setSignerName(intent.signerName);
          setNdaDeliveryEmail(intent.ndaDeliveryEmail);
          setNdaAgreed(true);
          setNdaGatePhase("credentials");
          const needsCreds =
            Boolean(err.requiresEmailVerification) ||
            Boolean(err.requiresPassword) ||
            (Boolean(err.requiresEmail) && !err.requiresEmailVerification && !gateParams.inviteToken);
          if (!needsCreds) {
            ndaSessionRecoveryRef.current = true;
            void tryAccessRef.current({
              email: intent.ndaDeliveryEmail,
              ndaAgreed: true,
              signerName: intent.signerName,
              inviteToken: gateParams.inviteToken,
            });
            return;
          }
        }
      }

      const unavailableCodes = new Set([
        "link_not_found",
        "link_expired",
        "link_revoked",
        "link_disabled",
        "link_max_access_reached",
        "blocked_email",
        "invite_expired",
        "invite_revoked",
        "invite_already_used",
      ]);
      const gateErrorCodes = new Set([
        "invalid_password",
        "whitelist_denied",
        "invalid_email_code",
        "not_allowed",
        "email_mismatch",
      ]);
      if (unavailableCodes.has(err.code)) {
        setLinkErrorCode(err.code);
      } else if (gateErrorCodes.has(err.code)) {
        setGateErrorCode(err.code);
        if (err.code === "email_mismatch") {
          const msg = t("viewer.emailMismatch");
          setGateError(msg);
          showFloatingGateTip(msg);
        } else if (err.code === "not_allowed") {
          const msg = t("viewer.emailNotAllowed");
          setGateError(msg);
          showFloatingGateTip(msg);
        } else {
          setGateError(err.message ?? t("common:error.loadFailed"));
        }
      } else if (
        err.code === "requires_email" ||
        err.code === "requires_email_code" ||
        err.code === "nda_required" ||
        err.code === "invalid_signer_name" ||
        err.code === "requires_password"
      ) {
        // Normal gate prompts: keep the credential form visible but don't show
        // an error message on the first visit when the visitor hasn't typed yet.
        setGateErrorCode(err.code);
        if (err.code === "invalid_signer_name") {
          setGateError(err.message ?? t("viewer.signerNameRequired"));
        }
        if (err.requiresNda && token) {
          const previewEmail = isValidDeliveryEmail(ndaDeliveryEmail)
            ? ndaDeliveryEmail.trim()
            : undefined;
          void api.getPublicNDAPreview?.(token, previewEmail).then((preview) => {
            applyNdaPreview(preview);
          }).catch(() => { /* preview is best-effort */ });
        }
      } else {
        // Unknown error codes (e.g. internal_error / network_error) still need
        // a visible message instead of the generic "load failed" fallback.
        setGateError(err.message ?? t("common:error.loadFailed"));
      }
    } finally {
      if (requestId === accessRequestIdRef.current) {
        setLoading(false);
      }
    }
  }, [token, t, sessionKey, signerName, ndaDeliveryEmail, showFloatingGateTip, applyNdaPreview]);
  tryAccessRef.current = tryAccess;

  useEffect(() => {
    if (!token) return;
    const bootKey = `${token}|${inviteToken ?? ""}`;
    // Only bootstrap once per token/invite. Do NOT re-issue a credential-less
    // Access when tryAccess's identity changes (i18n), or a valid session is
    // ignored and the visitor is sent back to the email/code gate.
    if (bootstrappedKeyRef.current === bootKey) return;
    bootstrappedKeyRef.current = bootKey;

    // Invalidate any in-flight Access for the previous link before starting.
    accessRequestIdRef.current += 1;
    ndaSessionRecoveryRef.current = false;
    setAccess(null);
    setAccessCredentials({});
    setSelectedDocIndex(0);
    setFolderView(false);
    setNdaCountdown(30);
    setNdaSignedAt(null);
    ndaReviewFinishedRef.current = false;
    setShowAccessRequest(false);
    setGateError(null);
    setGateErrorCode(null);
    setFloatingTip(null);
    setLoading(true);

    // Restore short-stored NDA intent in the SAME effect that boots Access so
    // bootstrap cannot overwrite phase=credentials after approval refresh.
    // phase=complete (post-Access) must also skip the sign page on refresh.
    const intent = loadNdaIntent(token);
    ndaIntentRestoredRef.current = token;
    if (intent) {
      setSignerName(intent.signerName);
      setNdaDeliveryEmail(intent.ndaDeliveryEmail);
      setNdaAgreed(intent.ndaAgreed);
      if (isPostSignNdaPhase(intent.phase) || intent.accessRequestPending) {
        setNdaGatePhase("credentials");
      } else {
        setNdaGatePhase("sign");
      }
    } else {
      setNdaGatePhase("sign");
    }

    let storedSession: string | null = null;
    if (sessionKey) {
      try {
        storedSession = sessionStorage.getItem(sessionKey);
      } catch { /* ignore */ }
    }
    if (storedSession && storedSession !== "undefined") {
      setAccessRequestSubmitted(false);
      void tryAccess({ sessionToken: storedSession, inviteToken });
      return;
    }

    // Pending access request: keep "submitted" UI across refresh until the
    // reserved email is allowlisted (owner approved). Then open credentials
    // for the new verification code (contract F).
    if (intent?.accessRequestPending) {
      setAccessRequestSubmitted(true);
      setLoading(false);
      const pendingBootKey = bootKey;
      const pendingEmail = intent.ndaDeliveryEmail;
      const pendingSigner = intent.signerName;
      const pendingAgreed = intent.ndaAgreed;
      void (async () => {
        try {
          await api.checkPublicLinkEmail(token, pendingEmail);
        } catch (e) {
          if (bootstrappedKeyRef.current !== pendingBootKey) return;
          const err = e as ApiError;
          setSecurity((prev) => ({
            email: err.requiresEmail ?? prev.email,
            emailVerification: err.requiresEmailVerification ?? prev.emailVerification,
            password: err.requiresPassword ?? prev.password,
            nda: err.requiresNda ?? prev.nda,
            isDealRoom: err.isDealRoom ?? prev.isDealRoom,
          }));
          // Still pending / denied — stay on submitted screen.
          return;
        }
        // Stale bootstrap (token/invite changed) — ignore this probe result.
        if (bootstrappedKeyRef.current !== pendingBootKey) return;
        // Approved: clear pending marker and continue into credential gates.
        // If Access still denies allowlist, tryAccess restores pending UI.
        clearAccessRequestPending(token);
        accessRequestPendingResumeRef.current = true;
        setAccessRequestSubmitted(false);
        setNdaGatePhase("credentials");
        void tryAccessRef.current({
          email: pendingEmail,
          ndaAgreed: pendingAgreed,
          signerName: pendingSigner,
          inviteToken,
        });
      })();
      return;
    }

    setAccessRequestSubmitted(false);

    // No session: if the visitor already completed NDA in this tab, retry Access
    // with stored intent instead of bouncing to the sign page. Verification-gated
    // links will still prompt for a fresh code via requires_email_code.
    if (intent?.phase === "complete" && intent.ndaAgreed) {
      void tryAccess({
        email: intent.ndaDeliveryEmail,
        ndaAgreed: true,
        signerName: intent.signerName,
        inviteToken,
      });
      return;
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

  const needsCredentialGates =
    security.emailVerification ||
    security.password ||
    (security.email && !security.emailVerification && !inviteToken);

  const ndaSignReady =
    signerName.trim().length > 0 &&
    ndaAgreed &&
    isValidDeliveryEmail(ndaDeliveryEmail);

  // Load NDA preview once a delivery email is present so allowlisted links can
  // evaluate access rules (preview without email is denied when allow rules exist).
  useEffect(() => {
    if (!token || !security.nda) return;
    if (ndaGatePhase !== "sign" && ndaGatePhase !== "review") return;
    if (!isValidDeliveryEmail(ndaDeliveryEmail)) return;
    const email = ndaDeliveryEmail.trim();
    let cancelled = false;
    void api
      .getPublicNDAPreview?.(token, email)
      .then((preview) => {
        if (cancelled) return;
        applyNdaPreview(preview);
      })
      .catch(() => { /* preview is best-effort */ });
    return () => {
      cancelled = true;
    };
  }, [token, security.nda, ndaGatePhase, ndaDeliveryEmail, applyNdaPreview]);

  /** Pre-review allowlist check. On success enters NDA review; on deny keeps mismatch actions. */
  const checkAndEnterNdaReview = useCallback(async () => {
    if (!token || !ndaSignReady) return;
    const deliveryEmail = ndaDeliveryEmail.trim();
    setFloatingTip(null);
    setNdaEmailChecking(true);
    try {
      // Allowlist first — preview itself is gated and would fail for denied emails.
      await api.checkPublicLinkEmail(token, deliveryEmail);

      let binding = ndaBinding;
      if (!binding?.ndaTemplateId) {
        const preview = await api.getPublicNDAPreview(token, deliveryEmail);
        applyNdaPreview(preview);
        if (!preview.ndaTemplate?.id) {
          setGateError(t("viewer.ndaPreviewUnavailable"));
          return;
        }
        binding = {
          ndaTemplateId: preview.ndaTemplate.id,
          contentSha256: preview.ndaTemplate.contentSha256?.trim() ?? "",
        };
      }

      setGateError(null);
      setGateErrorCode(null);
      setNdaSignedAt(new Date());
      saveNdaIntent(token, {
        signerName: signerName.trim(),
        ndaDeliveryEmail: deliveryEmail,
        ndaAgreed,
        phase: "review",
        ndaTemplateId: binding.ndaTemplateId,
        ...(binding.contentSha256 ? { contentSha256: binding.contentSha256 } : {}),
      });
      setNdaGatePhase("review");
    } catch (e) {
      const err = e as ApiError;
      setSecurity((prev) => ({
        email: err.requiresEmail ?? prev.email,
        emailVerification: err.requiresEmailVerification ?? prev.emailVerification,
        password: err.requiresPassword ?? prev.password,
        nda: err.requiresNda ?? prev.nda,
        isDealRoom: err.isDealRoom ?? prev.isDealRoom,
      }));
      if (err.code === "not_allowed" || err.code === "blocked_email") {
        const tip =
          err.code === "blocked_email"
            ? (err.message ?? t("viewer.emailNotAllowed"))
            : t("viewer.emailNotAuthorized");
        setGateErrorCode(err.code === "blocked_email" ? "blocked_email" : "not_allowed");
        setGateError(tip);
        showFloatingGateTip(tip);
        persistNdaIntent("sign");
        return;
      }
      setGateError(err.message ?? t("common:error.loadFailed"));
    } finally {
      setNdaEmailChecking(false);
    }
  }, [
    token,
    ndaSignReady,
    ndaDeliveryEmail,
    ndaBinding,
    ndaAgreed,
    signerName,
    applyNdaPreview,
    persistNdaIntent,
    showFloatingGateTip,
    t,
  ]);

  // After Continue on the sign step: 30s review of the signed agreement, then
  // either a clean credential page or Access when NDA is the only gate.
  useEffect(() => {
    if (ndaGatePhase !== "review") return;
    ndaReviewFinishedRef.current = false;
    setNdaCountdown(30);
    const startedAt = Date.now();
    const timer = window.setInterval(() => {
      const left = Math.max(0, 30 - Math.floor((Date.now() - startedAt) / 1000));
      setNdaCountdown(left);
      if (left > 0 || ndaReviewFinishedRef.current) return;
      ndaReviewFinishedRef.current = true;
      persistNdaIntent("credentials");
      if (needsCredentialGates) {
        setNdaGatePhase("credentials");
        return;
      }
      void tryAccess({
        email: ndaDeliveryEmail.trim(),
        ndaAgreed: true,
        signerName: signerName.trim(),
        inviteToken,
      });
    }, 250);
    return () => window.clearInterval(timer);
  }, [ndaGatePhase, needsCredentialGates, tryAccess, signerName, ndaDeliveryEmail, inviteToken, persistNdaIntent]);

  const submitAccessRequest = useCallback(async () => {
    if (!token) return;
    const reason = accessRequestReason.trim();
    if (!reason) {
      setGateError(t("viewer.accessRequestReasonRequired"));
      return;
    }
    const requestEmail = (security.nda ? ndaDeliveryEmail : (email || ndaDeliveryEmail)).trim();
    if (!isValidDeliveryEmail(requestEmail)) {
      setGateError(t("viewer.ndaDeliveryEmailRequired"));
      return;
    }
    // Keep intent email in sync for pending restore (NDA and non-NDA).
    if (!security.nda) {
      setNdaDeliveryEmail(requestEmail);
      setEmail(requestEmail);
    }
    setAccessRequestSubmitting(true);
    setGateError(null);
    try {
      await api.requestPublicLinkAccess(token, {
        email: requestEmail,
        reason,
        signerName: signerName.trim() || undefined,
      });
      saveNdaIntent(token, {
        signerName: signerName.trim(),
        ndaDeliveryEmail: requestEmail,
        ndaAgreed: security.nda ? ndaAgreed : false,
        phase: security.nda ? "credentials" : "sign",
        accessRequestPending: true,
      });
      setAccessRequestSubmitted(true);
      setShowAccessRequest(false);
      setGateErrorCode(null);
    } catch (e) {
      const err = e as ApiError;
      if (err.code === "access_request_exists") {
        saveNdaIntent(token, {
          signerName: signerName.trim(),
          ndaDeliveryEmail: requestEmail,
          ndaAgreed: security.nda ? ndaAgreed : false,
          phase: security.nda ? "credentials" : "sign",
          accessRequestPending: true,
        });
        setAccessRequestSubmitted(true);
        setShowAccessRequest(false);
        setGateError(null);
      } else {
        setGateError(err.message ?? t("viewer.accessRequestFailed"));
      }
    } finally {
      setAccessRequestSubmitting(false);
    }
  }, [token, accessRequestReason, ndaDeliveryEmail, email, signerName, security.nda, ndaAgreed, t]);

  /** After owner approval: clear pending marker and open credential / code gates. */
  const continueAfterAccessRequestApproval = useCallback(async () => {
    if (!token) return;
    const deliveryEmail = (ndaDeliveryEmail || email).trim();
    if (!isValidDeliveryEmail(deliveryEmail)) {
      setGateError(t("viewer.ndaDeliveryEmailRequired"));
      return;
    }
    const expectedBootKey = bootstrappedKeyRef.current ?? `${token}|${inviteToken ?? ""}`;
    setAccessRequestSubmitting(true);
    setGateError(null);
    try {
      await api.checkPublicLinkEmail(token, deliveryEmail);
      if (bootstrappedKeyRef.current !== expectedBootKey) return;
      clearAccessRequestPending(token);
      accessRequestPendingResumeRef.current = true;
      if (security.nda) {
        persistNdaIntent("credentials", { accessRequestPending: false });
        setNdaGatePhase("credentials");
      } else {
        clearNdaIntent(token);
      }
      setAccessRequestSubmitted(false);
      void tryAccess({
        email: deliveryEmail,
        ndaAgreed: security.nda,
        signerName: signerName.trim() || undefined,
        inviteToken,
      });
    } catch (e) {
      if (bootstrappedKeyRef.current !== expectedBootKey) return;
      const err = e as ApiError;
      setSecurity((prev) => ({
        email: err.requiresEmail ?? prev.email,
        emailVerification: err.requiresEmailVerification ?? prev.emailVerification,
        password: err.requiresPassword ?? prev.password,
        nda: err.requiresNda ?? prev.nda,
        isDealRoom: err.isDealRoom ?? prev.isDealRoom,
      }));
      if (err.code === "not_allowed" || err.code === "blocked_email") {
        setGateError(t("viewer.accessRequestStillPending"));
        return;
      }
      setGateError(err.message ?? t("common:error.loadFailed"));
    } finally {
      setAccessRequestSubmitting(false);
    }
  }, [token, ndaDeliveryEmail, email, signerName, inviteToken, persistNdaIntent, tryAccess, security.nda, t]);

  const clearMismatchAndRetry = useCallback(() => {
    setFloatingTip(null);
    setFloatingTipProgress(1);
    setShowAccessRequest(false);

    // Sign-step deny (not_allowed / blocked): re-check current delivery email.
    // Do NOT wipe gateErrorCode → Continue; that felt like resetting to the start page.
    if (
      ndaGatePhase === "sign" &&
      (gateErrorCode === "not_allowed" || gateErrorCode === "blocked_email")
    ) {
      void checkAndEnterNdaReview();
      // Focus email so the visitor can correct it before / while retrying.
      window.setTimeout(() => {
        document.getElementById("nda-delivery-email")?.focus();
      }, 0);
      return;
    }

    // Credentials email_mismatch: return to sign so delivery email is editable.
    if (gateErrorCode === "email_mismatch" && security.nda) {
      setGateError(null);
      setGateErrorCode(null);
      setEmailCode("");
      setNdaGatePhase("sign");
      persistNdaIntent("sign", { accessRequestPending: false });
      window.setTimeout(() => {
        document.getElementById("nda-delivery-email")?.focus();
      }, 0);
      return;
    }

    setGateError(null);
    setGateErrorCode(null);
    setEmailCode("");
  }, [ndaGatePhase, gateErrorCode, checkAndEnterNdaReview, security.nda, persistNdaIntent]);

  // Floating tip: 10s visual countdown, then auto-dismiss (recovery buttons stay).
  useEffect(() => {
    if (!floatingTip) return;
    const durationMs = 10_000;
    const startedAt = Date.now();
    const frame = window.setInterval(() => {
      const remaining = Math.max(0, 1 - (Date.now() - startedAt) / durationMs);
      setFloatingTipProgress(remaining);
      if (remaining <= 0) {
        setFloatingTip(null);
        window.clearInterval(frame);
      }
    }, 50);
    return () => window.clearInterval(frame);
  }, [floatingTip?.id]);

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
            <Button variant="outline" className="w-full" onClick={() => navigate("/")}>
              {t("common:backToHome")}
            </Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  // Deal-room: folder browser when explicitly requested, or when scope yields
  // zero documents (empty allowlist) so visitors never land on a blank canvas.
  if (access?.link.dealRoomId && (folderView || access.documents.length === 0)) {
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
    const inNdaSign = security.nda && ndaGatePhase === "sign";
    const inNdaReview = security.nda && ndaGatePhase === "review";
    const showCredentialGates = !security.nda || ndaGatePhase === "credentials" || ndaGatePhase === "complete";
    const continueDisabled =
      (inNdaSign && !ndaSignReady)
      || inNdaReview
      || accessRequestSubmitting
      || ndaEmailChecking;
    const useNdaSquareWindow =
      (inNdaSign || inNdaReview) &&
      !showAccessRequest &&
      !accessRequestSubmitted;
    const ndaSquareSize = "min(calc(100dvw - 3rem), calc(100dvh - 3rem))";
    const showMismatchActions = gateErrorCode === "email_mismatch" || gateErrorCode === "not_allowed" || gateErrorCode === "blocked_email";
    const requestEmailCandidate = security.nda ? ndaDeliveryEmail : (email || ndaDeliveryEmail);
    const canRequestAccess =
      isValidDeliveryEmail(requestEmailCandidate) &&
      (gateErrorCode === "email_mismatch" || gateErrorCode === "not_allowed");

    const floatingTipSecondsLeft = Math.max(1, Math.ceil(floatingTipProgress * 10));
    const countdownRing = 2 * Math.PI * 18; // r=18 for 40x40 viewBox

    return (
      <div className="relative flex min-h-dvh items-center justify-center bg-muted/30 p-6">
        {floatingTip && (
          <>
            {/* Soft blur veil over the NDA sign page; clicks pass through to bottom actions. */}
            <div
              className="pointer-events-none absolute inset-0 z-40 bg-zinc-950/20 backdrop-blur-[8px] transition-opacity duration-300 animate-in fade-in"
              aria-hidden
            />
            <div
              className="pointer-events-none fixed inset-0 z-50 flex items-center justify-center p-6"
              role="alert"
              aria-live="assertive"
            >
              <div
                className="pointer-events-auto w-full max-w-[22rem] origin-center animate-in fade-in zoom-in-95 duration-300"
                style={{ animationTimingFunction: "cubic-bezier(0.22, 1, 0.36, 1)" }}
              >
                <div
                  className="relative overflow-hidden rounded-2xl border border-white/60 bg-white/90 shadow-[0_28px_80px_-24px_rgba(15,23,42,0.45),inset_0_1px_0_rgba(255,255,255,0.85)] backdrop-blur-xl dark:border-white/10 dark:bg-zinc-950/90"
                >
                  {/* Warm accent edge */}
                  <div
                    className="absolute inset-y-3 left-0 w-[2px] rounded-full bg-gradient-to-b from-rose-400/90 via-amber-500/70 to-rose-400/40"
                    aria-hidden
                  />
                  <div className="flex gap-3.5 px-5 pb-4 pt-5 pl-6">
                    <div className="relative mt-0.5 size-10 shrink-0">
                      <svg
                        className="absolute inset-0 -rotate-90"
                        viewBox="0 0 40 40"
                        aria-hidden
                      >
                        <circle
                          cx="20"
                          cy="20"
                          r="18"
                          fill="none"
                          className="stroke-zinc-200/80 dark:stroke-zinc-700/80"
                          strokeWidth="2"
                        />
                        <circle
                          cx="20"
                          cy="20"
                          r="18"
                          fill="none"
                          className="stroke-rose-500/80 transition-[stroke-dashoffset] duration-75 ease-linear"
                          strokeWidth="2"
                          strokeLinecap="round"
                          strokeDasharray={countdownRing}
                          strokeDashoffset={countdownRing * (1 - Math.max(0, floatingTipProgress))}
                        />
                      </svg>
                      <div className="absolute inset-0 flex items-center justify-center">
                        <WarningCircle
                          size={18}
                          weight="duotone"
                          className="text-rose-600/90 dark:text-rose-400"
                        />
                      </div>
                    </div>
                    <div className="min-w-0 flex-1 pr-1">
                      <p className="text-[11px] font-medium tracking-[0.14em] text-zinc-500 uppercase dark:text-zinc-400">
                        {t("viewer.accessNoticeTitle")}
                      </p>
                      <p className="mt-1.5 text-[14.5px] leading-relaxed text-zinc-800 dark:text-zinc-100">
                        {floatingTip.message}
                      </p>
                      <p className="mt-3 text-[11px] tabular-nums tracking-wide text-zinc-400 dark:text-zinc-500">
                        {t("viewer.tipAutoDismiss", { seconds: floatingTipSecondsLeft })}
                      </p>
                    </div>
                  </div>
                  {/* Hairline progress under the panel */}
                  <div className="h-px bg-zinc-200/70 dark:bg-zinc-800">
                    <div
                      className="h-full origin-left bg-gradient-to-r from-rose-500/80 to-amber-400/70 transition-transform duration-75 ease-linear"
                      style={{ transform: `scaleX(${Math.max(0, floatingTipProgress)})` }}
                    />
                  </div>
                </div>
              </div>
            </div>
          </>
        )}
        <Card
          className={[
            useNdaSquareWindow ? "flex flex-col overflow-hidden" : "w-full max-w-md",
            floatingTip
              ? "scale-[0.985] opacity-70 blur-[2px] transition-[filter,opacity,transform] duration-300"
              : "transition-[filter,opacity,transform] duration-300",
          ].join(" ")}
          style={
            useNdaSquareWindow
              ? { width: ndaSquareSize, height: ndaSquareSize, maxWidth: "100%" }
              : undefined
          }
        >
          <CardHeader className="shrink-0">
            <CardTitle>
              {accessRequestSubmitted
                ? t("viewer.accessRequestSubmittedTitle")
                : showAccessRequest
                  ? t("viewer.accessRequestTitle")
                  : inNdaReview
                    ? t("viewer.ndaReviewTitle")
                    : inNdaSign
                      ? t("viewer.ndaSignTitle")
                      : t("viewer.gateTitle")}
            </CardTitle>
          </CardHeader>
          <CardContent
            className={
              useNdaSquareWindow
                ? "flex min-h-0 flex-1 flex-col space-y-4 overflow-hidden"
                : "space-y-4"
            }
          >
            {accessRequestSubmitted ? (
              <div className="space-y-4">
                <p className="text-sm text-muted-foreground">
                  {t("viewer.accessRequestSubmittedDescription", {
                    email: (ndaDeliveryEmail || email).trim(),
                  })}
                </p>
                <p className="text-sm text-muted-foreground">
                  {t("viewer.accessRequestRefreshHint")}
                </p>
                {gateError && (
                  <p className="text-sm text-destructive" role="alert">{gateError}</p>
                )}
                <div className="flex items-center justify-center">
                  <Button
                    className="min-w-[6.5rem] px-5"
                    disabled={accessRequestSubmitting}
                    onClick={() => { void continueAfterAccessRequestApproval(); }}
                  >
                    {t("viewer.accessRequestContinueAfterApproval")}
                  </Button>
                </div>
              </div>
            ) : showAccessRequest ? (
              <div className="space-y-4">
                <p className="text-sm text-muted-foreground">{t("viewer.accessRequestHint")}</p>
                <div className="space-y-2">
                  <Label htmlFor="access-request-email">
                    {security.nda ? t("viewer.ndaDeliveryEmailLabel") : t("viewer.emailLabel")}
                  </Label>
                  <Input
                    id="access-request-email"
                    type="email"
                    value={security.nda ? ndaDeliveryEmail : (email || ndaDeliveryEmail)}
                    readOnly
                    className="bg-muted/50"
                  />
                </div>
                {security.nda && (
                <div className="space-y-2">
                  <Label htmlFor="access-request-signer">{t("viewer.signerNameLabel")}</Label>
                  <Input
                    id="access-request-signer"
                    value={signerName}
                    onChange={(e) => setSignerName(e.target.value)}
                    placeholder={t("viewer.signerNamePlaceholder")}
                  />
                </div>
                )}
                <div className="space-y-2">
                  <Label htmlFor="access-request-reason">{t("viewer.accessRequestReasonLabel")}</Label>
                  <Textarea
                    id="access-request-reason"
                    value={accessRequestReason}
                    onChange={(e) => setAccessRequestReason(e.target.value)}
                    placeholder={t("viewer.accessRequestReasonPlaceholder")}
                    rows={3}
                  />
                </div>
                {gateError && (
                  <p className="text-sm text-destructive" role="alert">{gateError}</p>
                )}
                <div className="flex shrink-0 items-center justify-center gap-4">
                  <Button
                    className="min-w-[6.5rem] px-5"
                    disabled={accessRequestSubmitting || !accessRequestReason.trim()}
                    onClick={() => { void submitAccessRequest(); }}
                  >
                    {t("viewer.accessRequestSubmit")}
                  </Button>
                  <Button
                    variant="outline"
                    className="min-w-[6.5rem] px-5"
                    disabled={accessRequestSubmitting}
                    onClick={() => {
                      setShowAccessRequest(false);
                      setGateError(null);
                    }}
                  >
                    {t("common:back")}
                  </Button>
                </div>
              </div>
            ) : (
              <>
            {hasGates && gateError && !inNdaReview && !showMismatchActions && (
              <p className="text-sm text-destructive" role="alert">
                {gateError}
              </p>
            )}
            {inviteToken && showCredentialGates && (
              <div className="rounded-md border border-border bg-muted/50 p-3 text-sm">
                {prefilledEmail
                  ? t("viewer.inviteVerificationFor", { email: prefilledEmail })
                  : t("viewer.inviteVerification")}
              </div>
            )}

            {inNdaReview && (
              <div className="flex min-h-0 flex-1 flex-col space-y-3">
                <div className="min-h-0 flex-1 overflow-hidden rounded-md border bg-white">
                  <div className="h-full overflow-y-auto overscroll-contain">
                    {ndaPreviewPageUrls.length > 0 ? (
                      ndaPreviewPageUrls.map((url, index) => (
                        <img
                          key={`${url}-${index}`}
                          src={url}
                          alt={t("viewer.ndaPreviewPage", { page: index + 1 })}
                          className="block h-auto w-full border-b border-border/40 last:border-b-0"
                        />
                      ))
                    ) : ndaDocumentUrl ? (
                      <iframe
                        title={t("viewer.ndaPreviewTitle")}
                        src={ndaDocumentUrl}
                        className="h-full min-h-[50%] w-full border-0"
                      />
                    ) : (
                      <div className="flex h-40 items-center justify-center px-4 text-center text-xs text-muted-foreground">
                        {t("viewer.ndaPreviewUnavailable")}
                      </div>
                    )}
                    <div className="border-t bg-muted/20 px-4 py-4">
                      <p className="text-xs font-medium text-muted-foreground">
                        {t("viewer.ndaAuditTrailTitle")}
                      </p>
                      <p className="mt-3 text-xs text-muted-foreground">{t("viewer.ndaSignedBy")}</p>
                      <p
                        className="mt-1 text-2xl text-foreground"
                        style={{ fontFamily: "\"Segoe Script\", \"Apple Chancery\", \"Bradley Hand\", cursive" }}
                      >
                        {signerName.trim()}
                      </p>
                      <p className="mt-3 text-xs text-muted-foreground">{t("viewer.ndaDeliveryEmailLabel")}</p>
                      <p className="text-sm text-foreground">{ndaDeliveryEmail.trim()}</p>
                      {ndaSignedAt && (
                        <p className="mt-3 text-xs text-muted-foreground">
                          {t("viewer.ndaSignedAt", { time: ndaSignedAt.toLocaleString() })}
                        </p>
                      )}
                    </div>
                  </div>
                </div>
                <p className="shrink-0 text-center text-sm text-muted-foreground" aria-live="polite">
                  {t("viewer.ndaReviewCountdown", { seconds: ndaCountdown })}
                </p>
                <div className="h-1.5 shrink-0 overflow-hidden rounded-full bg-muted">
                  <div
                    className="h-full bg-primary transition-[width] duration-300 ease-linear"
                    style={{ width: `${((30 - ndaCountdown) / 30) * 100}%` }}
                  />
                </div>
              </div>
            )}

            {showCredentialGates && security.emailVerification && (
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
            {showCredentialGates && security.email && !security.emailVerification && !inviteToken && !security.nda && (
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
            {showCredentialGates && security.password && (
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

            {inNdaSign && (
              <div className="flex min-h-0 flex-1 flex-col space-y-3">
                {(ndaPreviewPageUrls.length > 0 || ndaDocumentUrl) && (
                  <div className="min-h-0 flex-1 overflow-hidden rounded-md border bg-muted/30">
                    {ndaPreviewPageUrls.length > 0 ? (
                      <div
                        className="h-full cursor-zoom-in overflow-y-auto overscroll-contain bg-white"
                        onClick={() => setNdaPreviewZoomed(true)}
                        title={t("viewer.ndaPreviewZoomHint")}
                        role="button"
                        tabIndex={0}
                        aria-label={t("viewer.ndaPreviewZoomHint")}
                        onKeyDown={(e) => {
                          if (e.key === "Enter" || e.key === " ") {
                            e.preventDefault();
                            setNdaPreviewZoomed(true);
                          }
                        }}
                      >
                        {ndaPreviewPageUrls.map((url, index) => (
                          <img
                            key={`${url}-${index}`}
                            src={url}
                            alt={t("viewer.ndaPreviewPage", { page: index + 1 })}
                            className="block h-auto w-full select-none pointer-events-none border-b border-border/40 last:border-b-0"
                            draggable={false}
                          />
                        ))}
                      </div>
                    ) : (
                      <div className="flex h-full min-h-32 items-center justify-center px-4 text-center text-xs text-muted-foreground">
                        {t("viewer.ndaPreviewUnavailable")}
                      </div>
                    )}
                  </div>
                )}
                <Dialog open={ndaPreviewZoomed} onOpenChange={setNdaPreviewZoomed}>
                  <DialogContent className="max-h-[90vh] gap-0 overflow-hidden p-0 sm:max-w-3xl">
                    <DialogHeader className="border-b px-4 py-3">
                      <DialogTitle>{t("viewer.ndaPreviewTitle")}</DialogTitle>
                    </DialogHeader>
                    {ndaPreviewPageUrls.length > 0 && (
                      <div className="max-h-[min(80vh,720px)] overflow-y-auto bg-white">
                        {ndaPreviewPageUrls.map((url, index) => (
                          <img
                            key={`zoom-${url}-${index}`}
                            src={url}
                            alt={t("viewer.ndaPreviewPage", { page: index + 1 })}
                            className="block h-auto w-full border-b border-border/40 last:border-b-0"
                          />
                        ))}
                      </div>
                    )}
                  </DialogContent>
                </Dialog>
                <div className="shrink-0 space-y-2">
                  <Label htmlFor="nda-delivery-email">{t("viewer.ndaDeliveryEmailLabel")}</Label>
                  <Input
                    id="nda-delivery-email"
                    type="email"
                    value={ndaDeliveryEmail}
                    onChange={(e) => {
                      setNdaDeliveryEmail(e.target.value);
                      setEmail(e.target.value);
                    }}
                    placeholder={t("viewer.ndaDeliveryEmailPlaceholder")}
                    autoComplete="email"
                    readOnly={Boolean(inviteToken && prefilledEmail)}
                  />
                  <p className="text-xs text-muted-foreground">{t("viewer.ndaDeliveryEmailHint")}</p>
                </div>
                <div className="shrink-0 space-y-2">
                  <Label htmlFor="signer-name">{t("viewer.signerNameLabel")}</Label>
                  <Input
                    id="signer-name"
                    type="text"
                    value={signerName}
                    onChange={(e) => setSignerName(e.target.value)}
                    placeholder={t("viewer.signerNamePlaceholder")}
                    autoComplete="name"
                  />
                </div>
                <div className="flex shrink-0 items-center gap-2">
                  <Checkbox
                    id="nda"
                    checked={ndaAgreed}
                    onCheckedChange={(checked) => setNdaAgreed(checked === true)}
                  />
                  <Label htmlFor="nda">{t("viewer.ndaLabel")}</Label>
                </div>
              </div>
            )}

            {showMismatchActions ? (
              <div className="flex shrink-0 items-center justify-center gap-4">
                <Button
                  className="min-w-[6.5rem] px-5"
                  disabled={ndaEmailChecking}
                  onClick={clearMismatchAndRetry}
                >
                  {t("common:retry")}
                </Button>
                {canRequestAccess && (
                  <Button
                    variant="outline"
                    className="min-w-[6.5rem] px-5"
                    disabled={ndaEmailChecking || accessRequestSubmitting}
                    onClick={() => {
                      if (!security.nda) {
                        const candidate = (email || ndaDeliveryEmail).trim();
                        if (candidate) {
                          setNdaDeliveryEmail(candidate);
                          setEmail(candidate);
                        }
                      }
                      setShowAccessRequest(true);
                      setGateError(null);
                      setFloatingTip(null);
                    }}
                  >
                    {t("viewer.requestAuthorization")}
                  </Button>
                )}
              </div>
            ) : (
              !inNdaReview && (
              <Button
                className="w-full shrink-0"
                disabled={continueDisabled}
                onClick={() => {
                  setGateError(null);
                  setGateErrorCode(null);
                  setFloatingTip(null);
                  const accessEmail = prefilledEmail || email;

                  if (inNdaSign) {
                    if (!ndaSignReady || !token) return;
                    void checkAndEnterNdaReview();
                    return;
                  }

                  if (security.nda && !isValidDeliveryEmail(ndaDeliveryEmail)) {
                    setGateError(t("viewer.ndaDeliveryEmailRequired"));
                    return;
                  }
                  if (!security.nda && security.email && !security.emailVerification && !inviteToken && !accessEmail.trim()) {
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
                  if (security.nda) {
                    persistNdaIntent("credentials");
                  }
                  void tryAccess({
                    email: security.nda
                      ? ndaDeliveryEmail.trim()
                      : security.email && !security.emailVerification
                        ? accessEmail
                        : undefined,
                    emailCode: security.emailVerification ? emailCode : undefined,
                    password: security.password ? password : undefined,
                    ndaAgreed: security.nda ? ndaAgreed : undefined,
                    signerName: security.nda ? signerName.trim() : undefined,
                    inviteToken,
                  });
                }}
              >
                {gateErrorCode === "not_allowed" ? t("common:retry") : t("viewer.continue")}
              </Button>
              )
            )}
            {!hasGates && (
              <p className="text-sm text-muted-foreground">
                {gateError ?? t("common:error.loadFailed")}
              </p>
            )}
              </>
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
        watermark={access.link.watermarkEnabled ? { watermarkText: access.link.watermarkText } : null}
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
