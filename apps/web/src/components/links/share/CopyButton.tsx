import { useState } from "react";
import { Copy, Check } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { copyToClipboard } from "@/lib/clipboard";
import { cn } from "@/lib/utils";

interface CopyButtonProps {
  value: string;
  label: string;
  successLabel: string;
  variant?: "default" | "outline" | "ghost" | "secondary";
  size?: "default" | "sm" | "icon";
  className?: string;
  disabled?: boolean;
}

export function CopyButton({
  value,
  label,
  successLabel,
  variant = "outline",
  size = "default",
  className,
  disabled,
}: CopyButtonProps) {
  const [copied, setCopied] = useState(false);

  const handleClick = async () => {
    const ok = await copyToClipboard(value, successLabel);
    if (ok) {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  return (
    <Button
      type="button"
      variant={variant}
      size={size}
      onClick={handleClick}
      disabled={disabled || copied || !value}
      className={cn("gap-1.5 active:scale-95 transition-transform", className)}
    >
      {copied ? <Check size={16} /> : <Copy size={16} />}
      {copied ? successLabel : label}
    </Button>
  );
}
