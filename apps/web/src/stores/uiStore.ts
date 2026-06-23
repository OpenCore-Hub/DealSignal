import { create } from "zustand";
import { persist } from "zustand/middleware";
import type { Workspace } from "@/types";

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

      reset: () => set({ currentWorkspace: null, uploadDialogOpen: false }),
    }),
    {
      name: "dealsignal-ui",
      partialize: (state) => ({ theme: state.theme, sidebarOpen: state.sidebarOpen }),
    }
  )
);
