import { useMemo } from "react";
import { useParams } from "react-router";
import { api } from "@/lib/api";
import { useAsyncData } from "@/hooks/useAsyncData";
import type { WorkspaceMember } from "@/types";

export type WorkspaceRole = WorkspaceMember["role"];

export interface WorkspaceAccess {
  role: WorkspaceRole | null;
  loading: boolean;
  /** Read workspace content (all roles including guest). */
  canRead: boolean;
  /** Create/update/delete links, documents, rooms, contacts, etc. */
  canWrite: boolean;
  /** Settings, members, billing, integrations, brand, security. */
  canManage: boolean;
  isGuest: boolean;
}

function normalizeRole(role: string | undefined | null): WorkspaceRole | null {
  switch (role) {
    case "owner":
    case "admin":
    case "member":
    case "guest":
      return role;
    default:
      return null;
  }
}

/** Resolves the caller's role in the current (or provided) workspace slug. */
export function useWorkspaceAccess(workspaceSlugProp?: string): WorkspaceAccess {
  const { workspaceSlug: paramSlug } = useParams<{ workspaceSlug: string }>();
  const workspaceSlug = workspaceSlugProp ?? paramSlug;

  const { data, loading } = useAsyncData(async () => {
    const res = await api.getWorkspaces();
    return res.data ?? [];
  }, [workspaceSlug]);

  return useMemo(() => {
    const match = (data ?? []).find((ws) => ws.slug === workspaceSlug);
    const role = normalizeRole(match?.role);
    const canManage = role === "owner" || role === "admin";
    const canWrite = canManage || role === "member";
    const canRead = Boolean(role);
    return {
      role,
      loading,
      canRead,
      canWrite,
      canManage,
      isGuest: role === "guest",
    };
  }, [data, workspaceSlug, loading]);
}
