import { DotsThree } from "@phosphor-icons/react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { buttonVariants } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { useTranslation } from "react-i18next";

export interface RowAction {
  label: string;
  icon?: React.ReactNode;
  onClick: () => void;
  destructive?: boolean;
  pro?: boolean;
  disabled?: boolean;
  title?: string;
}

interface RowActionsProps {
  actions: RowAction[];
}

export function RowActions({ actions }: RowActionsProps) {
  const { t } = useTranslation("common");
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        className={buttonVariants({ variant: "ghost", size: "icon" })}
        aria-label={t("moreActions")}
        onClick={(e) => e.stopPropagation()}
      >
        <DotsThree size={18} />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-48" onClick={(e) => e.stopPropagation()}>
        {actions.map((action, index) => (
          <div key={action.label + index}>
            {index > 0 && action.destructive && <DropdownMenuSeparator />}
            <DropdownMenuItem
              onClick={action.onClick}
              disabled={action.disabled}
              title={action.title}
              className={action.destructive ? "text-error-500 focus:text-error-500" : ""}
            >
              {action.icon && <span className="mr-2">{action.icon}</span>}
              <span className="flex-1">{action.label}</span>
              {action.pro && (
                <Badge variant="outline" className="ml-2 text-caption">
                  PRO
                </Badge>
              )}
            </DropdownMenuItem>
          </div>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
