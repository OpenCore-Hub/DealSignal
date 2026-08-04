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

  /** Optional trailing crumb for nested pages (e.g. deal room name). */
  breadcrumbTail: BreadcrumbItem | null;
  setBreadcrumbTail: (item: BreadcrumbItem | null) => void;

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

      breadcrumbTail: null,
      setBreadcrumbTail: (item) => set({ breadcrumbTail: item }),

      reset: () =>
        set({
          currentWorkspace: null,
          uploadDialogOpen: false,
          breadcrumbs: [],
          breadcrumbTail: null,
        }),
    }),
    {
      name: "dealsignal-ui",
      // Persist workspace so /viewer (outside /:slug layout) and window.open
      // from Manage Docs keep authenticated API routing (ceiling Phase X).
      partialize: (state) => ({
        theme: state.theme,
        sidebarOpen: state.sidebarOpen,
        currentWorkspace: state.currentWorkspace,
      }),
    }
  )
);
