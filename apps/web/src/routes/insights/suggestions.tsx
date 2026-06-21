import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { Lightning, Envelope, Lightbulb } from "@phosphor-icons/react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { HeatBadge } from "@/components/common/HeatBadge";
import { EmptyState } from "@/components/common/EmptyState";
import { api } from "@/lib/api";
import type { Suggestion } from "@/types";

export function InsightsSuggestionsPage() {
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const [suggestions, setSuggestions] = useState<Suggestion[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    async function load() {
      try {
        setLoading(true);
        setError(null);
        const res = await api.getSuggestions();
        setSuggestions(res.data);
      } catch (e) {
        setError(e instanceof Error ? e.message : "加载失败");
      } finally {
        setLoading(false);
      }
    }
    load();
  }, []);

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center gap-4 rounded-lg border border-border bg-card p-12 text-center">
        <p className="text-body text-muted-foreground">{error}</p>
        <Button onClick={() => window.location.reload()}>重试</Button>
      </div>
    );
  }

  if (loading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-32" />
        <Skeleton className="h-32" />
        <Skeleton className="h-32" />
      </div>
    );
  }

  if (suggestions.length === 0) {
    return (
      <EmptyState
        icon={<Lightbulb size={48} />}
        title="暂无跟进建议"
        description="当前没有待办建议，系统会在检测到新的热度信号时自动生成。"
        size="large"
      />
    );
  }

  return (
    <div className="space-y-6">
      <div className="grid gap-4">
        {suggestions.map((s) => (
          <Card key={s.id} className="transition-colors hover:bg-muted/50">
            <CardContent className="p-5">
              <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                <div className="space-y-2">
                  <div className="flex items-center gap-2">
                    <Lightning size={18} className="text-warning-500" />
                    <h3 className="text-h3">{s.action}</h3>
                  </div>
                  <p className="text-body text-muted-foreground">{s.reason}</p>
                  <div className="flex flex-wrap items-center gap-3 text-caption text-muted-foreground">
                    <span>{s.contactEmail}</span>
                    <span>·</span>
                    <span>{s.documentTitle}</span>
                    <span>·</span>
                    <HeatBadge level={s.heatLevel} />
                    <span className="tabular-nums">{s.score} 分</span>
                  </div>
                </div>
                <div className="flex shrink-0 gap-2">
                  <Button
                    variant="outline"
                    onClick={() => navigate(`/${workspaceSlug}/contacts/${s.contactId}`)}
                  >
                    查看联系人
                  </Button>
                  <Button className="gap-1.5" disabled title="邮件发送需后端支持">
                    <Envelope size={16} />
                    写跟进邮件
                  </Button>
                </div>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  );
}
