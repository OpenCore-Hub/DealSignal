import { Buildings, Scales } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Badge } from "@/components/ui/badge";
import { normalizeCategoryForDisplay, shouldShowCategoryBadge } from "@/lib/documentCategory";
import type { DocumentCategory } from "@/types";

interface DocumentCategoryBadgeProps {
  category?: DocumentCategory | string | null;
  className?: string;
}

export function DocumentCategoryBadge({ category, className }: DocumentCategoryBadgeProps) {
  const { t } = useTranslation("documents");
  const normalized = normalizeCategoryForDisplay(category);

  if (!shouldShowCategoryBadge(normalized)) {
    return null;
  }

  if (normalized === "agreement") {
    return (
      <Badge
        variant="outline"
        className={
          className ??
          "gap-1 border-violet-200 bg-violet-50 text-violet-700 dark:border-violet-800 dark:bg-violet-950 dark:text-violet-300"
        }
      >
        <Scales className="size-3" />
        {t("category.agreement")}
      </Badge>
    );
  }

  return (
    <Badge
      variant="outline"
      className={
        className ??
        "gap-1 border-sky-200 bg-sky-50 text-sky-700 dark:border-sky-800 dark:bg-sky-950 dark:text-sky-300"
      }
    >
      <Buildings className="size-3" />
      {t("category.dealRoom")}
    </Badge>
  );
}
