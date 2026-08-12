import { Navigate, useParams } from "react-router";
import { useTranslation } from "react-i18next";
import { useWorkspaceAccess } from "@/hooks/useWorkspaceAccess";
import { Skeleton } from "@/components/ui/skeleton";

/** Blocks guests from write-only workspace routes (upload, create, edit). */
export function RequireWorkspaceWrite({ children }: { children: React.ReactNode }) {
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const { canWrite, loading } = useWorkspaceAccess(workspaceSlug);
  const { t } = useTranslation("common");

  if (loading) {
    return (
      <div className="space-y-4 p-6">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-40 w-full" />
      </div>
    );
  }

  if (!canWrite) {
    return (
      <Navigate
        to={`/${workspaceSlug}/dashboard`}
        replace
        state={{ notice: t("error.codes.insufficient_role") }}
      />
    );
  }

  return <>{children}</>;
}
