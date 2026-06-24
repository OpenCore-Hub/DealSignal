import { useEffect } from "react";
import { useNavigate } from "react-router";
import { Buildings } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { api } from "@/lib/api";
import { useAsyncData } from "@/hooks/useAsyncData";
import { useTranslation } from "react-i18next";

export function WorkspacesPage() {
  const { t } = useTranslation("common");
  const navigate = useNavigate();
  const {
    data: workspaces,
    loading,
    error,
    refetch,
  } = useAsyncData(async () => {
    const res = await api.getWorkspaces();
    return res.data;
  }, [t]);

  useEffect(() => {
    if (workspaces?.length === 1) {
      navigate(`/${workspaces[0].slug}/dashboard`, { replace: true });
    }
  }, [workspaces, navigate]);

  if (loading) {
    return (
      <div className="flex min-h-[100dvh] items-center justify-center p-6">
        <div className="w-full max-w-md space-y-4">
          <Skeleton className="h-8 w-1/3" />
          <Skeleton className="h-24 w-full" />
          <Skeleton className="h-24 w-full" />
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex min-h-[100dvh] flex-col items-center justify-center gap-4 p-6 text-center">
        <p className="text-muted-foreground">{error}</p>
        <Button onClick={refetch}>{t("retry")}</Button>
      </div>
    );
  }

  if (workspaces?.length === 1) {
    return null;
  }

  return (
    <div className="flex min-h-[100dvh] flex-col items-center justify-center bg-background p-6">
      <div className="w-full max-w-md space-y-6">
        <div className="text-center">
          <h1 className="text-display text-foreground">{t("selectWorkspace")}</h1>
          <p className="mt-2 text-body text-muted-foreground">
            {t("selectWorkspaceDescription")}
          </p>
        </div>

        <div className="space-y-3">
          {workspaces?.map((workspace) => {
            const displayName = t(workspace.name);
            return (
              <Card
                key={workspace.id}
                data-testid={`workspace-card-${workspace.slug}`}
                className="cursor-pointer transition-colors hover:bg-muted/50"
                onClick={() => navigate(`/${workspace.slug}/dashboard`)}
              >
                <CardContent className="flex items-center gap-4 py-4">
                  <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-md bg-primary text-primary-foreground text-lg font-bold">
                    {displayName.slice(0, 1)}
                  </div>
                  <div className="min-w-0">
                    <p className="font-medium text-foreground">{displayName}</p>
                    <p className="text-caption text-muted-foreground truncate">
                      {workspace.slug}
                    </p>
                  </div>
                </CardContent>
              </Card>
            );
          })}
        </div>

        <Button
          variant="outline"
          className="w-full gap-2"
          onClick={() => navigate("/workspaces/new")}
        >
          <Buildings size={18} />
          {t("createWorkspace")}
        </Button>
      </div>
    </div>
  );
}
