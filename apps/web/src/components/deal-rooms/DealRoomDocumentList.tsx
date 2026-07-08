import { useMemo } from "react";
import { FileText } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { EmptyState } from "@/components/common/EmptyState";
import { FileTypeIcon } from "@/components/common/FileTypeIcon";
import { useTranslation } from "react-i18next";
import { formatFileSize } from "@/lib/formatters";
import type { DealRoomFolderDocs, DealRoomFolder } from "@/types";

interface DealRoomDocumentListProps {
  folders: DealRoomFolder[];
  folderDocs: DealRoomFolderDocs[];
  selectedFolderPath?: string | null;
  onDocumentOpen?: (docId: string) => void;
}

export function DealRoomDocumentList({
  folders,
  folderDocs,
  selectedFolderPath,
  onDocumentOpen,
}: DealRoomDocumentListProps) {
  const { t } = useTranslation("dealRooms");
  const { t: tc } = useTranslation("common");

  const folderNameMap = useMemo(() => {
    const map = new Map<string, string>();
    for (const folder of folders) {
      map.set(folder.path, folder.name);
    }
    return map;
  }, [folders]);

  const documents = useMemo(() => {
    if (selectedFolderPath) {
      const group = folderDocs.find((fd) => fd.folder === selectedFolderPath);
      return group?.documents ?? [];
    }
    return folderDocs.flatMap((fd) => fd.documents).sort((a, b) => a.sort_order - b.sort_order);
  }, [folderDocs, selectedFolderPath]);

  const title = selectedFolderPath
    ? t("documentList.documentsInFolder", { name: folderNameMap.get(selectedFolderPath) ?? selectedFolderPath })
    : t("documentList.allDocuments");

  if (documents.length === 0) {
    return (
      <Card className="h-full">
        <CardHeader>
          <CardTitle className="text-h2">{title}</CardTitle>
        </CardHeader>
        <CardContent>
          <EmptyState
            icon={<FileText size={40} />}
            title={t("documentList.emptyTitle")}
            description={t("documentList.emptyDescription")}
          />
        </CardContent>
      </Card>
    );
  }

  return (
    <Card className="h-full">
      <CardHeader className="flex flex-row items-center justify-between gap-4">
        <CardTitle className="text-h2">
          {title}
          <span className="ml-2 text-sm font-normal text-muted-foreground">({documents.length})</span>
        </CardTitle>
      </CardHeader>
      <CardContent>
        <ul className="space-y-2">
          {documents.map((doc) => (
            <li
              key={doc.id}
              className="group flex items-center justify-between gap-3 rounded-lg border border-border bg-card p-3 transition-colors hover:bg-muted/50"
            >
              <div className="flex min-w-0 items-center gap-3">
                <FileTypeIcon type={doc.source_type} size={24} />
                <div className="min-w-0">
                  <p
                    className="cursor-pointer truncate text-sm font-medium hover:text-primary"
                    onClick={() => onDocumentOpen?.(doc.document_id)}
                  >
                    {doc.title}
                  </p>
                  <p className="text-caption text-muted-foreground">
                    {doc.page_count ? tc("pageCount", { count: doc.page_count }) : null}
                    {doc.page_count && doc.file_size ? " · " : null}
                    {doc.file_size ? formatFileSize(doc.file_size) : null}
                  </p>
                </div>
              </div>
            </li>
          ))}
        </ul>
      </CardContent>
    </Card>
  );
}
