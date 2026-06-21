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
import { useTranslation } from "react-i18next";

export function InsightsPagesPage() {
  const { t } = useTranslation("insights");
  const { t: tc } = useTranslation("common");
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
        <Button onClick={refetch}>{tc("retry")}</Button>
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
        title={t("pages.emptyTitle")}
        description={t("pages.emptyDescription")}
      />
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <h2 className="text-h2">{t("pages.title")}</h2>
        <Select value={selectedDocId} onValueChange={handleDocChange}>
          <SelectTrigger className="w-full sm:w-64">
            <SelectValue placeholder={t("pages.selectPlaceholder")} />
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
          title={t("pages.noAnalyticsTitle")}
          description={t("pages.noAnalyticsDescription")}
        />
      ) : (
        <DocumentAnalytics analytics={analytics ?? []} />
      )}
    </div>
  );
}
