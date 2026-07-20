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
import { cn } from "@/lib/utils";
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
        className={cn(
          buttonVariants({ variant: "ghost", size: "icon-sm" }),
          "rounded-full text-muted-foreground hover:bg-muted hover:text-foreground data-[popup-open]:bg-muted data-[popup-open]:text-foreground",
        )}
        aria-label={t("moreActions")}
        onClick={(e) => e.stopPropagation()}
      >
        <DotsThree size={18} weight="bold" />
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="end"
        sideOffset={6}
        className="min-w-48 p-1.5 shadow-dropdown"
        onClick={(e) => e.stopPropagation()}
      >
        {actions.map((action, index) => {
          const prev = actions[index - 1];
          const showSeparator = Boolean(action.destructive && prev && !prev.destructive);
          return (
            <div key={`${action.label}-${index}`}>
              {showSeparator && <DropdownMenuSeparator className="my-1.5" />}
              <DropdownMenuItem
                onClick={action.onClick}
                disabled={action.disabled}
                title={action.title}
                variant={action.destructive ? "destructive" : "default"}
                className="gap-2.5 px-2.5 py-2"
              >
                {action.icon ? (
                  <span
                    className={cn(
                      "flex size-4 shrink-0 items-center justify-center [&_svg]:size-4",
                      action.destructive
                        ? "text-destructive"
                        : "text-muted-foreground group-focus/dropdown-menu-item:text-accent-foreground",
                    )}
                  >
                    {action.icon}
                  </span>
                ) : null}
                <span className="flex-1 truncate text-left font-medium tracking-tight">
                  {action.label}
                </span>
                {action.pro ? (
                  <Badge variant="outline" className="ml-1 text-caption">
                    PRO
                  </Badge>
                ) : null}
              </DropdownMenuItem>
            </div>
          );
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
