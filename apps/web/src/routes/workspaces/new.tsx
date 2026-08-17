import { useEffect, useState } from "react";
import { useNavigate } from "react-router";
import {
  AuthField,
  AuthInput,
  AuthNotice,
  AuthStage,
  AuthSubmit,
  AuthTextLink,
  railFromNamespace,
} from "@/components/auth/AuthStage";
import { api } from "@/lib/api";
import { ApiError } from "@/lib/apiClient";
import { apiErrorMessage } from "@/lib/apiErrors";
import { useUIStore } from "@/stores/uiStore";
import { useTranslation } from "react-i18next";

function slugify(name: string): string {
  return name
    .toLowerCase()
    .replace(/[^a-z0-9]/g, "")
    .slice(0, 50);
}

export function CreateWorkspacePage() {
  const { t } = useTranslation("common");
  const navigate = useNavigate();
  const setCurrentWorkspace = useUIStore((s) => s.setCurrentWorkspace);
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [brandColor, setBrandColor] = useState("#0055ff");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [slugError, setSlugError] = useState<string | null>(null);
  const [needsEmailVerify, setNeedsEmailVerify] = useState(false);

  useEffect(() => {
    let cancelled = false;
    api
      .getMe()
      .then((user) => {
        if (!cancelled) setNeedsEmailVerify(user.email_verified === false);
      })
      .catch(() => {
        /* create still works; trial copy is best-effort */
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const handleNameChange = (value: string) => {
    setName(value);
    setSlugError(null);
    setError(null);
    if (slug === slugify(name) || slug === "") {
      setSlug(slugify(value));
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);
    setSlugError(null);
    try {
      const workspace = await api.createWorkspace({
        name: name.trim(),
        slug: slug.trim(),
        brand_color: brandColor,
      });
      setCurrentWorkspace(workspace);
      navigate(`/${workspace.slug}/dashboard`, { replace: true });
    } catch (err) {
      if (
        err instanceof ApiError &&
        (err.code === "slug_conflict" || err.code === "duplicate_slug")
      ) {
        setSlugError(t("workspaceSlugConflict"));
        setError(null);
      } else {
        setError(apiErrorMessage(err, { fallback: "saveFailed" }));
        setSlugError(null);
      }
      setLoading(false);
    }
  };

  return (
    <AuthStage
      rail={railFromNamespace(t, "workspaceStage")}
      kicker={t("createWorkspaceKicker")}
      title={t("createWorkspace")}
      notice={needsEmailVerify ? <AuthNotice tone="invite">{t("verifyEmailForTrial")}</AuthNotice> : null}
      footer={
        <p>
          <AuthTextLink onClick={() => navigate(-1)}>{t("back")}</AuthTextLink>
        </p>
      }
    >
      <form onSubmit={handleSubmit}>
        <AuthField id="name" label={t("workspaceName")}>
          <AuthInput
            id="name"
            value={name}
            onChange={(e) => handleNameChange(e.target.value)}
            placeholder={t("workspaceNamePlaceholder")}
            required
          />
        </AuthField>
        <AuthField
          id="slug"
          label={t("workspaceSlug")}
          hint={
            slugError ? (
              <p id="workspace-slug-error" className="auth-error">
                {slugError}
              </p>
            ) : undefined
          }
        >
          <AuthInput
            id="slug"
            value={slug}
            onChange={(e) => {
              setSlug(slugify(e.target.value));
              setSlugError(null);
            }}
            placeholder={t("workspaceSlugPlaceholder")}
            required
            aria-invalid={Boolean(slugError)}
            aria-describedby={slugError ? "workspace-slug-error" : undefined}
          />
        </AuthField>
        <AuthField id="brandColor" label={t("brandColor")}>
          <div className="auth-color-row">
            <input
              id="brandColor"
              type="color"
              value={brandColor}
              onChange={(e) => setBrandColor(e.target.value)}
            />
            <span className="auth-color-hex">{brandColor}</span>
          </div>
        </AuthField>
        {error ? <p className="auth-error">{error}</p> : null}
        <AuthSubmit type="submit" disabled={loading || !name.trim() || !slug.trim()}>
          {loading ? t("creating") : t("createWorkspace")}
        </AuthSubmit>
      </form>
    </AuthStage>
  );
}
