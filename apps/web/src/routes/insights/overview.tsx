import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { FileText, Link as LinkIcon, Users } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { StatCard } from "@/components/common/StatCard";
import { HeatBadge } from "@/components/common/HeatBadge";
import { TrendChart } from "@/components/common/TrendChart";
import { api, type InsightsOverview } from "@/lib/api";

export function InsightsOverviewPage() {
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();
  const [data, setData] = useState<InsightsOverview | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.getInsightsOverview().then((d) => {
      setData(d);
      setLoading(false);
    });
  }, []);

  if (loading || !data) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-8 w-64" />
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
          <Skeleton className="h-24" />
          <Skeleton className="h-24" />
          <Skeleton className="h-24" />
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard label="高热度" value={data.tierCounts.hot} icon={<HeatBadge level="hot" />} />
        <StatCard label="中热度" value={data.tierCounts.warm} icon={<HeatBadge level="warm" />} />
        <StatCard label="低热度" value={data.tierCounts.cold} icon={<HeatBadge level="cold" />} />
        <StatCard label="总热度分" value={data.tierCounts.hot * 3 + data.tierCounts.warm} />
      </div>

      <TrendChart title="近 7 天访问量趋势" />

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-h2 flex items-center gap-2">
              <FileText size={20} />
              热门文档
            </CardTitle>
          </CardHeader>
          <CardContent>
            <ul className="space-y-2">
              {data.topDocuments.map((doc) => (
                <li
                  key={doc.id}
                  className="flex cursor-pointer items-center justify-between rounded-md border border-border p-3 transition-colors hover:bg-muted"
                  onClick={() => navigate(`/${workspaceSlug}/documents/${doc.id}`)}
                >
                  <span className="truncate text-sm font-medium">{doc.title}</span>
                  <div className="flex items-center gap-3">
                    <span className="text-caption tabular-nums text-muted-foreground">{doc.views} views</span>
                    <HeatBadge level={doc.heatLevel} />
                  </div>
                </li>
              ))}
            </ul>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-h2 flex items-center gap-2">
              <LinkIcon size={20} />
              热门链接
            </CardTitle>
          </CardHeader>
          <CardContent>
            <ul className="space-y-2">
              {data.topLinks.map((link) => (
                <li
                  key={link.id}
                  className="flex cursor-pointer items-center justify-between rounded-md border border-border p-3 transition-colors hover:bg-muted"
                  onClick={() => navigate(`/${workspaceSlug}/links/${link.id}`)}
                >
                  <span className="truncate text-sm font-medium">{link.shortUrl}</span>
                  <div className="flex items-center gap-3">
                    <span className="text-caption tabular-nums text-muted-foreground">{link.views} views</span>
                    <HeatBadge level={link.heatLevel} />
                  </div>
                </li>
              ))}
            </ul>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-h2 flex items-center gap-2">
            <Users size={20} />
            高意向访问者
          </CardTitle>
        </CardHeader>
        <CardContent>
          <ul className="space-y-2">
            {data.topContacts.map((contact) => (
              <li
                key={contact.id}
                className="flex cursor-pointer items-center justify-between rounded-md border border-border p-3 transition-colors hover:bg-muted"
                onClick={() => navigate(`/${workspaceSlug}/contacts/${contact.id}`)}
              >
                <span className="truncate text-sm font-medium">{contact.email}</span>
                <div className="flex items-center gap-3">
                  <span className="text-caption tabular-nums text-muted-foreground">{contact.score} 分</span>
                  <HeatBadge level={contact.heatLevel} />
                </div>
              </li>
            ))}
          </ul>
        </CardContent>
      </Card>
    </div>
  );
}
