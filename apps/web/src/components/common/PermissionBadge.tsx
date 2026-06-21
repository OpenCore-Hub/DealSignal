import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";

interface PermissionBadgeProps {
  type: "public" | "email" | "whitelist" | "password" | "nda";
  className?: string;
}

const config: Record<PermissionBadgeProps["type"], { label: string; variant: "default" | "secondary" | "outline" | "hot" | "warm" }> = {
  public: { label: "公开", variant: "secondary" },
  email: { label: "邮箱验证", variant: "outline" },
  whitelist: { label: "白名单", variant: "outline" },
  password: { label: "密码保护", variant: "outline" },
  nda: { label: "NDA", variant: "warm" },
};

export function PermissionBadge({ type, className }: PermissionBadgeProps) {
  const { label, variant } = config[type];
  return (
    <Badge variant={variant} className={cn("text-xs", className)}>
      {label}
    </Badge>
  );
}
