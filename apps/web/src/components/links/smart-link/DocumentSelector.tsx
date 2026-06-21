import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { FileText } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import type { Document } from "@/types";

interface DocumentSelectorProps {
  documents: Document[];
  loading: boolean;
  selectedId: string;
  onSelect: (id: string) => void;
  onUpload: () => void;
}

export function DocumentSelector({
  documents,
  loading,
  selectedId,
  onSelect,
  onUpload,
}: DocumentSelectorProps) {
  const { t } = useTranslation("links");
  const { t: tc } = useTranslation("common");

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-h2 flex items-center gap-2">
          <FileText size={20} />
          {t("creator.selectDocument")}
        </CardTitle>
      </CardHeader>
      <CardContent>
        {loading ? (
          <Skeleton className="h-10" />
        ) : documents.length === 0 ? (
          <div className="rounded-md border border-dashed border-border p-6 text-center">
            <p className="text-sm text-muted-foreground">{t("creator.noDocuments")}</p>
            <Button className="mt-3" size="sm" onClick={onUpload}>
              {tc("upload")}
            </Button>
          </div>
        ) : (
          <Select value={selectedId} onValueChange={(value) => value && onSelect(value)}>
            <SelectTrigger>
              <SelectValue placeholder={t("creator.selectPlaceholder")} />
            </SelectTrigger>
            <SelectContent>
              {documents.map((doc) => (
                <SelectItem key={doc.id} value={doc.id}>
                  {doc.title}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}
      </CardContent>
    </Card>
  );
}
