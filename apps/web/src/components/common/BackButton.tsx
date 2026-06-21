import { Link } from "react-router";
import { ArrowLeft } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";

interface BackButtonProps {
  to: string;
  label?: string;
  className?: string;
}

export function BackButton({ to, label, className }: BackButtonProps) {
  const { t } = useTranslation("common");
  const backLabel = label ?? t("back");

  return (
    <Link
      to={to}
      className={cn(
        "inline-flex items-center gap-1 text-caption text-muted-foreground transition-colors hover:text-foreground focus-ring",
        className
      )}
    >
      <ArrowLeft size={14} />
      {backLabel}
    </Link>
  );
}
