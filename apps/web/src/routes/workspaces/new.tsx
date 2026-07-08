import { useState } from "react";
import { useNavigate } from "react-router";
import { Buildings } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { api } from "@/lib/api";
import { ApiError } from "@/lib/apiClient";
import { useTranslation } from "react-i18next";

function slugify(name: string): string {
  return name
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 50);
}

export function CreateWorkspacePage() {
  const { t } = useTranslation("common");
  const navigate = useNavigate();
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [brandColor, setBrandColor] = useState("#0055ff");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleNameChange = (value: string) => {
    setName(value);
    if (slug === slugify(name) || slug === "") {
      setSlug(slugify(value));
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);
    try {
      const workspace = await api.createWorkspace({
        name: name.trim(),
        slug: slug.trim(),
        brand_color: brandColor,
      });
      navigate(`/${workspace.slug}/dashboard`, { replace: true });
    } catch (err) {
      let message = t("error.saveFailed");
      if (err instanceof ApiError) {
        if (err.code === "slug_conflict") {
          message = t("error.duplicateSlug");
        } else if (err.code === "invalid_slug") {
          message = t("error.invalidSlug");
        } else if (err.message) {
          message = err.message;
        }
      } else if (err instanceof Error && err.message) {
        message = err.message;
      }
      setError(message);
      setLoading(false);
    }
  };

  return (
    <div className="flex min-h-[100dvh] flex-col items-center justify-center bg-background p-6">
      <div className="w-full max-w-md">
        <Button variant="ghost" className="mb-4" onClick={() => navigate(-1)}>
          {t("back")}
        </Button>
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-h2">
              <Buildings size={24} />
              {t("createWorkspace")}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSubmit} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="name">{t("workspaceName")}</Label>
                <Input
                  id="name"
                  value={name}
                  onChange={(e) => handleNameChange(e.target.value)}
                  placeholder={t("workspaceNamePlaceholder")}
                  required
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="slug">{t("workspaceSlug")}</Label>
                <Input
                  id="slug"
                  value={slug}
                  onChange={(e) => setSlug(slugify(e.target.value))}
                  placeholder={t("workspaceSlugPlaceholder")}
                  required
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="brandColor">{t("brandColor")}</Label>
                <Input
                  id="brandColor"
                  type="color"
                  value={brandColor}
                  onChange={(e) => setBrandColor(e.target.value)}
                />
              </div>
              {error && <p className="text-caption text-error-500">{error}</p>}
              <Button type="submit" className="w-full" disabled={loading || !name.trim() || !slug.trim()}>
                {loading ? t("creating") : t("createWorkspace")}
              </Button>
            </form>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
