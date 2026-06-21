import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { FileText } from "@phosphor-icons/react";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { DocumentAnalytics } from "@/components/documents/DocumentAnalytics";
import { EmptyState } from "@/components/common/EmptyState";
import { useAsyncData } from "@/hooks/useAsyncData";
import { api } from "@/lib/api";

export function InsightsPagesPage() {
  const [selectedDocId, setSelectedDocId] = useState("");
  const {
    data: documents,
    loading: loadingDocs,
    error,
    refetch,
  } = useAsyncData(async () => {
    const res = await api.getDocuments();
    const docs = res.data;
    setSelectedDocId(docs[0]?.id || "");
    return docs;
  }, []);

  const {
    data: analytics,
    loading: loadingAnalytics,
  } = useAsyncData(
    async () => {
      if (!selectedDocId) return [];
      const res = await api.getPageAnalytics(selectedDocId);
      return res.data;
    },
    [selectedDocId]
  );

  const handleDocChange = (value: string | null) => {
    if (value) {
      setSelectedDocId(value);
    }
  };

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center gap-4 rounded-lg border border-border bg-card p-12 text-center">
        <p className="text-body text-muted-foreground">{error}</p>
        <Button onClick={refetch}>重试</Button>
      </div>
    );
  }

  if (loadingDocs) {
    return <Skeleton className="h-80" />;
  }

  if (!documents || documents.length === 0) {
    return (
      <EmptyState
        icon={<FileText size={48} />}
        title="暂无文档"
        description="上传文档后即可查看页面参与度分析。"
      />
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <h2 className="text-h2">页面参与度</h2>
        <Select value={selectedDocId} onValueChange={handleDocChange}>
          <SelectTrigger className="w-full sm:w-64">
            <SelectValue placeholder="选择文档" />
          </SelectTrigger>
          <SelectContent>
            {documents.map((doc) => (
              <SelectItem key={doc.id} value={doc.id}>
                {doc.title}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {loadingAnalytics ? (
        <Skeleton className="h-80" />
      ) : analytics?.length === 0 ? (
        <EmptyState
          icon={<FileText size={48} />}
          title="暂无页面分析数据"
          description="该文档暂无访问记录，分享链接后即可收集数据。"
        />
      ) : (
        <DocumentAnalytics analytics={analytics ?? []} />
      )}
    </div>
  );
}
