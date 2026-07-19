import { create } from "zustand";
import { persist } from "zustand/middleware";
import type { Workspace } from "@/types";

export interface BreadcrumbItem {
  label: string;
  to?: string;
}

interface UIState {
  sidebarOpen: boolean;
  toggleSidebar: () => void;
  setSidebarOpen: (open: boolean) => void;

  theme: "light" | "dark" | "system";
  setTheme: (theme: "light" | "dark" | "system") => void;

  currentWorkspace: Workspace | null;
  setCurrentWorkspace: (workspace: Workspace | null) => void;

  uploadDialogOpen: boolean;
  setUploadDialogOpen: (open: boolean) => void;

  breadcrumbs: BreadcrumbItem[];
  setBreadcrumbs: (items: BreadcrumbItem[]) => void;

  reset: () => void;
}

export const useUIStore = create<UIState>()(
  persist(
    (set) => ({
      sidebarOpen: true,
      toggleSidebar: () => set((state) => ({ sidebarOpen: !state.sidebarOpen })),
      setSidebarOpen: (open) => set({ sidebarOpen: open }),

      theme: "system",
      setTheme: (theme) => set({ theme }),

      currentWorkspace: null,
      setCurrentWorkspace: (workspace) => set({ currentWorkspace: workspace }),

      uploadDialogOpen: false,
      setUploadDialogOpen: (open) => set({ uploadDialogOpen: open }),

      breadcrumbs: [],
      setBreadcrumbs: (items) => set({ breadcrumbs: items }),

      reset: () => set({ currentWorkspace: null, uploadDialogOpen: false, breadcrumbs: [] }),
    }),
    {
      name: "dealsignal-ui",
      partialize: (state) => ({ theme: state.theme, sidebarOpen: state.sidebarOpen }),
    }
  )
);
