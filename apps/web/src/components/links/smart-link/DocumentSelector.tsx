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
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-h2 flex items-center gap-2">
          <FileText size={20} />
          选择文档
        </CardTitle>
      </CardHeader>
      <CardContent>
        {loading ? (
          <Skeleton className="h-10" />
        ) : documents.length === 0 ? (
          <div className="rounded-md border border-dashed border-border p-6 text-center">
            <p className="text-sm text-muted-foreground">暂无可用文档，请先上传。</p>
            <Button className="mt-3" size="sm" onClick={onUpload}>
              上传文档
            </Button>
          </div>
        ) : (
          <Select value={selectedId} onValueChange={(value) => value && onSelect(value)}>
            <SelectTrigger>
              <SelectValue placeholder="选择要分享的文档" />
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
