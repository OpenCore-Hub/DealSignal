import { useState, useMemo } from "react";
import { useNavigate, useParams } from "react-router";
import { Users, MagnifyingGlass } from "@phosphor-icons/react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { PageHeader } from "@/components/common/PageHeader";
import { HeatBadge } from "@/components/common/HeatBadge";
import { EmptyState } from "@/components/common/EmptyState";
import { api } from "@/lib/api";
import { formatDuration, formatRelativeTime } from "@/lib/formatters";
import { useAsyncData } from "@/hooks/useAsyncData";
import type { Contact } from "@/types";
// Contact 类型用于组件内的类型推断
export type { Contact };

export function ContactsPage() {
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const [query, setQuery] = useState("");
  const { data: contacts, loading, error, refetch } = useAsyncData(
    async () => {
      const res = await api.getContacts();
      return res.data;
    },
    []
  );

  const filtered = useMemo(() => {
    const list = contacts ?? [];
    const q = query.toLowerCase();
    return list.filter(
      (c) =>
        c.name.toLowerCase().includes(q) ||
        c.email.toLowerCase().includes(q) ||
        (c.organization?.toLowerCase().includes(q) ?? false)
    );
  }, [contacts, query]);

  return (
    <div className="space-y-6">
      <PageHeader title="访问者" description="追踪谁看过你的材料，识别高意向联系人。" />

      <div className="relative max-w-sm">
        <MagnifyingGlass
          size={18}
          className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground"
        />
        <Input
          placeholder="搜索联系人..."
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          className="pl-9"
        />
      </div>

      {error ? (
        <div className="flex flex-col items-center justify-center gap-4 rounded-lg border border-border bg-card p-12 text-center">
          <p className="text-body text-muted-foreground">{error}</p>
          <Button onClick={refetch}>重试</Button>
        </div>
      ) : loading ? (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
          <Skeleton className="h-28" />
          <Skeleton className="h-28" />
        </div>
      ) : filtered.length === 0 ? (
        <EmptyState
          icon={<Users size={48} />}
          title="暂无联系人"
          description="当有人通过分享链接访问文档时，会自动聚合为联系人。"
          size="large"
        />
      ) : (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
          {filtered.map((contact) => (
            <Card
              key={contact.id}
              role="link"
              tabIndex={0}
              className="cursor-pointer transition-colors hover:bg-muted/50"
              onClick={() => navigate(`/${workspaceSlug}/contacts/${contact.id}`)}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  navigate(`/${workspaceSlug}/contacts/${contact.id}`);
                }
              }}
            >
              <CardContent className="flex items-start justify-between p-5">
                <div>
                  <p className="text-h3">{contact.name}</p>
                  <p className="text-caption text-muted-foreground">{contact.email}</p>
                  <p className="mt-2 text-caption text-muted-foreground">
                    {contact.organization || "未知机构"} · {contact.totalVisits} 次访问 · 累计{" "}
                    {formatDuration(contact.totalDurationSeconds)}
                  </p>
                </div>
                <div className="text-right">
                  <HeatBadge level={contact.heatLevel} />
                  <p className="mt-1 text-caption text-muted-foreground">
                    最后 {contact.lastSeenAt ? formatRelativeTime(contact.lastSeenAt) : "-"}
                  </p>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
