import { DocumentsTable } from "@/components/documents/DocumentsTable";

export function DocumentsPage() {
  return (
    <div className="space-y-6">
      <div className="space-y-1">
        <h1 className="text-h1">文档库</h1>
        <p className="text-body text-muted-foreground">
          管理所有已上传材料，追踪传播与热度。
        </p>
      </div>
      <DocumentsTable />
    </div>
  );
}
