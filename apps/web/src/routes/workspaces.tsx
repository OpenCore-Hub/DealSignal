import { useEffect, useState } from "react";
import { useNavigate } from "react-router";
import { Buildings } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { api } from "@/lib/api";
import type { Workspace } from "@/types";

export function WorkspacesPage() {
  const navigate = useNavigate();
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      try {
        setLoading(true);
        setError(null);
        const res = await api.getWorkspaces();
        if (!cancelled) setWorkspaces(res.data);
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : "加载失败");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    load();
    return () => {
      cancelled = true;
    };
  }, []);

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
        <Button onClick={() => window.location.reload()}>重试</Button>
      </div>
    );
  }

  if (workspaces.length === 1) {
    // 只有一个 workspace 时直接跳转，避免多余点击
    navigate(`/${workspaces[0].slug}/dashboard`, { replace: true });
    return null;
  }

  return (
    <div className="flex min-h-[100dvh] flex-col items-center justify-center bg-background p-6">
      <div className="w-full max-w-md space-y-6">
        <div className="text-center">
          <h1 className="text-display text-foreground">选择工作区</h1>
          <p className="mt-2 text-body text-muted-foreground">
            请选择要进入的工作区
          </p>
        </div>

        <div className="space-y-3">
          {workspaces.map((workspace) => (
            <Card
              key={workspace.id}
              className="cursor-pointer transition-colors hover:bg-muted/50"
              onClick={() => navigate(`/${workspace.slug}/dashboard`)}
            >
              <CardContent className="flex items-center gap-4 py-4">
                <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-md bg-primary text-primary-foreground text-lg font-bold">
                  {workspace.name.slice(0, 1)}
                </div>
                <div className="min-w-0">
                  <p className="font-medium text-foreground">{workspace.name}</p>
                  <p className="text-caption text-muted-foreground truncate">
                    {workspace.slug}
                  </p>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>

        <Button
          variant="outline"
          className="w-full gap-2"
          onClick={() => {}}
          disabled
          title="创建工作区需后端支持"
        >
          <Buildings size={18} />
          创建工作区
        </Button>
      </div>
    </div>
  );
}
