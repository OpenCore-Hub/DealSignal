"use client"

import { Toaster as Sonner, type ToasterProps } from "sonner"
import {
  CheckCircle,
  Info,
  Warning,
  X,
  Spinner,
} from "@phosphor-icons/react"
import { useUIStore } from "@/stores/uiStore"

const Toaster = ({ ...props }: ToasterProps) => {
  const { theme } = useUIStore()

  return (
    <Sonner
      theme={theme === "system" ? undefined : theme}
      className="toaster group"
      icons={{
        success: <CheckCircle className="size-4" weight="fill" />,
        info: <Info className="size-4" weight="fill" />,
        warning: <Warning className="size-4" weight="fill" />,
        error: <X className="size-4" weight="bold" />,
        loading: <Spinner className="size-4 animate-spin" />,
      }}
      style={
        {
          "--normal-bg": "var(--popover)",
          "--normal-text": "var(--popover-foreground)",
          "--normal-border": "var(--border)",
          "--border-radius": "var(--radius)",
        } as React.CSSProperties
      }
      toastOptions={{
        classNames: {
          toast: "cn-toast",
        },
      }}
      {...props}
    />
  )
}

export { Toaster }
