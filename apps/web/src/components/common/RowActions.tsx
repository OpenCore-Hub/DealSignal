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
          "rounded-full text-muted-foreground/80",
          "opacity-70 transition-[opacity,background-color,color,transform] duration-200",
          "hover:bg-foreground/[0.06] hover:text-foreground hover:opacity-100",
          "active:scale-[0.94]",
          "data-[popup-open]:bg-foreground/[0.08] data-[popup-open]:text-foreground data-[popup-open]:opacity-100",
          "group-hover:opacity-100",
        )}
        aria-label={t("moreActions")}
        onClick={(e) => e.stopPropagation()}
      >
        <DotsThree size={18} weight="bold" />
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="end"
        sideOffset={8}
        className={cn(
          "min-w-[14rem] overflow-hidden rounded-xl border-border/50 p-1.5",
          "bg-popover/95 text-popover-foreground backdrop-blur-xl",
          "shadow-[0_18px_50px_-18px_rgba(15,23,42,0.35),0_0_0_1px_rgba(15,23,42,0.04)]",
          "dark:shadow-[0_18px_50px_-18px_rgba(0,0,0,0.55),0_0_0_1px_rgba(255,255,255,0.06)]",
          "duration-200 data-open:zoom-in-95 data-open:slide-in-from-top-1",
        )}
        onClick={(e) => e.stopPropagation()}
      >
        {actions.map((action, index) => {
          const prev = actions[index - 1];
          const showSeparator = Boolean(action.destructive && prev && !prev.destructive);
          return (
            <div
              key={`${action.label}-${index}`}
              className="animate-in fade-in-0 slide-in-from-top-1 fill-mode-both"
              style={{
                animationDuration: "220ms",
                animationDelay: `${Math.min(index, 6) * 28}ms`,
              }}
            >
              {showSeparator ? (
                <DropdownMenuSeparator className="mx-1 my-1.5 bg-border/70" />
              ) : null}
              <DropdownMenuItem
                onClick={action.onClick}
                disabled={action.disabled}
                title={action.title}
                variant={action.destructive ? "destructive" : "default"}
                className={cn(
                  "gap-2.5 rounded-lg px-2 py-2",
                  "transition-[background-color,transform] duration-150",
                  "focus:bg-foreground/[0.05]",
                  "data-[variant=destructive]:focus:bg-destructive/10",
                  "active:scale-[0.985]",
                )}
              >
                {action.icon ? (
                  <span
                    className={cn(
                      "flex size-7 shrink-0 items-center justify-center rounded-md",
                      "transition-colors duration-150",
                      action.destructive
                        ? "bg-destructive/10 text-destructive"
                        : "bg-muted/80 text-muted-foreground group-focus/dropdown-menu-item:bg-foreground/[0.06] group-focus/dropdown-menu-item:text-foreground",
                    )}
                  >
                    <span className="flex size-4 items-center justify-center [&_svg]:size-3.5">
                      {action.icon}
                    </span>
                  </span>
                ) : null}
                <span className="flex-1 truncate text-left text-[13px] font-medium tracking-[-0.01em]">
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
