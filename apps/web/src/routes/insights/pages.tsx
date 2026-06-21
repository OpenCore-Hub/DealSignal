import { useEffect, useState } from "react";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { DocumentAnalytics } from "@/components/documents/DocumentAnalytics";
import { api } from "@/lib/api";
import { mockDocuments } from "@/lib/mocks/data";
import type { PageAnalytics } from "@/types";

export function InsightsPagesPage() {
  const [selectedDocId, setSelectedDocId] = useState(mockDocuments[0]?.id || "");
  const [analytics, setAnalytics] = useState<PageAnalytics[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!selectedDocId) return;
    api.getPageAnalytics(selectedDocId).then((res) => {
      setAnalytics(res.data);
      setLoading(false);
    });
  }, [selectedDocId]);

  const handleDocChange = (value: string | null) => {
    if (value) {
      setLoading(true);
      setSelectedDocId(value);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <h2 className="text-h2">页面参与度</h2>
        <Select value={selectedDocId} onValueChange={handleDocChange}>
          <SelectTrigger className="w-full sm:w-64">
            <SelectValue placeholder="选择文档" />
          </SelectTrigger>
          <SelectContent>
            {mockDocuments.map((doc) => (
              <SelectItem key={doc.id} value={doc.id}>
                {doc.title}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {loading ? (
        <Skeleton className="h-80" />
      ) : analytics.length === 0 ? (
        <Card>
          <CardContent className="py-12 text-center text-muted-foreground">
            暂无页面分析数据
          </CardContent>
        </Card>
      ) : (
        <DocumentAnalytics analytics={analytics} />
      )}
    </div>
  );
}
