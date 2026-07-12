import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { FileText, Folder, CaretRight } from "@phosphor-icons/react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";

interface PublicDocumentSummary {
  id: string;
  title: string;
  pageCount: number;
  sourceType: string;
  folderPath?: string;
}

interface PublicDealRoomLinkViewerProps {
  linkName?: string;
  documents: PublicDocumentSummary[];
  onViewDocument: (documentId: string) => void;
}

interface FolderGroup {
  path: string;
  name: string;
  documents: PublicDocumentSummary[];
}

export function PublicDealRoomLinkViewer({
  linkName,
  documents,
  onViewDocument,
}: PublicDealRoomLinkViewerProps) {
  const { t } = useTranslation("dealRooms");
  const [expanded, setExpanded] = useState<Set<string>>(new Set());

  const folders: FolderGroup[] = useMemo(() => {
    const map = new Map<string, FolderGroup>();
    for (const doc of documents) {
      const path = doc.folderPath || "/";
      if (!map.has(path)) {
        const segments = path.split("/").filter(Boolean);
        const name =
          segments.length === 0
            ? t("detail.folders")
            : (segments[segments.length - 1] ?? path);
        map.set(path, { path, name, documents: [] });
      }
      map.get(path)!.documents.push(doc);
    }
    return Array.from(map.values()).sort((a, b) => a.path.localeCompare(b.path));
  }, [documents, t]);

  const toggleFolder = (path: string) => {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(path)) {
        next.delete(path);
      } else {
        next.add(path);
      }
      return next;
    });
  };

  return (
    <div className="min-h-screen bg-muted/30 p-6">
      <div className="mx-auto max-w-5xl space-y-6">
        <div>
          <h1 className="text-h1">{linkName || t("public.title")}</h1>
          <p className="text-body text-muted-foreground">
            {t("detail.documents")}
          </p>
        </div>

        {folders.length === 0 ? (
          <Card>
            <CardContent className="py-10 text-center text-sm text-muted-foreground">
              {t("public.noDocuments")}
            </CardContent>
          </Card>
        ) : (
          folders.map((folder) => {
            const isOpen = expanded.has(folder.path) || folders.length === 1;
            return (
              <Card key={folder.path}>
                <CardHeader className="pb-2">
                  <CardTitle className="text-h3 flex items-center gap-2">
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-auto gap-2 p-0 font-semibold"
                      onClick={() => toggleFolder(folder.path)}
                      aria-expanded={isOpen}
                    >
                      <Folder size={18} className="text-muted-foreground" />
                      {folder.name}
                      <CaretRight
                        size={16}
                        className={`text-muted-foreground transition-transform ${
                          isOpen ? "rotate-90" : ""
                        }`}
                      />
                    </Button>
                  </CardTitle>
                </CardHeader>
                {isOpen && (
                  <CardContent>
                    <ul className="space-y-1">
                      {folder.documents.map((doc) => (
                        <li key={doc.id}>
                          <Button
                            variant="ghost"
                            className="h-auto w-full justify-start gap-2 px-2 py-2 font-normal"
                            onClick={() => onViewDocument(doc.id)}
                          >
                            <FileText
                              size={16}
                              className="text-muted-foreground"
                            />
                            <span className="flex-1 text-left text-sm">
                              {doc.title}
                            </span>
                            <span className="text-xs text-muted-foreground">
                              {doc.pageCount > 0
                                ? t("documents:viewer.pageCountShort", { count: doc.pageCount })
                                : doc.sourceType.toUpperCase()}
                            </span>
                          </Button>
                        </li>
                      ))}
                    </ul>
                  </CardContent>
                )}
              </Card>
            );
          })
        )}
      </div>
    </div>
  );
}
