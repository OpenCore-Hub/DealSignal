import { useNavigate, useParams } from "react-router";
import { Plus } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { LinksTable } from "@/components/links/LinksTable";

export function LinksPage() {
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="space-y-1">
          <h1 className="text-h1">分享链接</h1>
          <p className="text-body text-muted-foreground">
            管理所有可追踪链接，查看访问热度与安全设置。
          </p>
        </div>
        <Button onClick={() => navigate(`/${workspaceSlug}/links/new`)}>
          <Plus size={16} className="mr-2" />
          创建链接
        </Button>
      </div>
      <LinksTable />
    </div>
  );
}
