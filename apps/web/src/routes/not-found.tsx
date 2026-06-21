import { useNavigate, useParams } from "react-router";
import { House } from "@phosphor-icons/react";
import { EmptyState } from "@/components/common/EmptyState";

export function NotFoundPage() {
  const navigate = useNavigate();
  const { workspaceSlug } = useParams<{ workspaceSlug: string }>();

  return (
    <div className="flex h-full items-center justify-center">
      <EmptyState
        icon={<House size={48} />}
        title="页面未找到"
        description="你访问的页面不存在，请返回 Dashboard。"
        action={{ label: "返回 Dashboard", onClick: () => navigate(`/${workspaceSlug}/dashboard`) }}
        size="large"
      />
    </div>
  );
}
