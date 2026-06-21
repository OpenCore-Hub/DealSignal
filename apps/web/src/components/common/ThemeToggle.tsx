import { Sun, Moon, Desktop } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useUIStore } from "@/stores/uiStore";

export function ThemeToggle() {
  const { theme, setTheme } = useUIStore();

  return (
    <DropdownMenu>
      <DropdownMenuTrigger>
        <Button size="icon" variant="ghost" aria-label="切换主题">
          {theme === "light" ? <Sun size={18} /> : theme === "dark" ? <Moon size={18} /> : <Desktop size={18} />}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem onClick={() => setTheme("light")} className="gap-2">
          <Sun size={16} /> 浅色
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => setTheme("dark")} className="gap-2">
          <Moon size={16} /> 深色
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => setTheme("system")} className="gap-2">
          <Desktop size={16} /> 跟随系统
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
