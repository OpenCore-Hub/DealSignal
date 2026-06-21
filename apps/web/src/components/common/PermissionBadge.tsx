import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";

interface PermissionBadgeProps {
  type: "public" | "email" | "whitelist" | "password" | "nda";
  className?: string;
}

const config: Record<PermissionBadgeProps["type"], { labelKey: string; variant: "default" | "secondary" | "outline" | "hot" | "warm" }> = {
  public: { labelKey: "permission.public", variant: "secondary" },
  email: { labelKey: "permission.email", variant: "outline" },
  whitelist: { labelKey: "permission.whitelist", variant: "outline" },
  password: { labelKey: "permission.password", variant: "outline" },
  nda: { labelKey: "permission.nda", variant: "warm" },
};

export function PermissionBadge({ type, className }: PermissionBadgeProps) {
  const { t } = useTranslation("common");
  const { labelKey, variant } = config[type];
  return (
    <Badge variant={variant} className={cn("text-xs", className)}>
      {t(labelKey)}
    </Badge>
  );
}
