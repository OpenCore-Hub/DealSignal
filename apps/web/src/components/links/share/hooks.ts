import { useCallback } from "react";
import { useAsyncData } from "@/hooks/useAsyncData";
import { api } from "@/lib/api";
import type { AccessLog, AccessRule, Link, LinkInvitation } from "@/types";

export function useDealRoomLinks(roomId: string, enabled = true) {
  const { data, loading, error, refetch } = useAsyncData(
    () => (enabled ? api.getDealRoomLinks(roomId).then((res) => res.data) : Promise.resolve([])),
    [roomId, enabled]
  );
  return { links: data ?? [], loading, error, refetch };
}

export function useLinkAccessRules(linkId: string | undefined, enabled = true) {
  const { data, loading, error, refetch } = useAsyncData(
    () => (enabled && linkId ? api.getLinkAccessRules(linkId).then((res) => res.data) : Promise.resolve([])),
    [linkId, enabled]
  );
  return { rules: data ?? [], loading, error, refetch };
}

export function useLinkInvitations(linkId: string | undefined, enabled = true) {
  const { data, loading, error, refetch } = useAsyncData(
    () => (enabled && linkId ? api.getLinkInvitations(linkId).then((res) => res.data) : Promise.resolve([])),
    [linkId, enabled]
  );
  return { invitations: data ?? [], loading, error, refetch };
}

export function useAccessLogs(linkId: string | undefined, enabled = true) {
  const { data, loading, error, refetch } = useAsyncData(
    () => (enabled && linkId ? api.getAccessLogs(linkId).then((res) => res.data) : Promise.resolve([])),
    [linkId, enabled]
  );
  return { logs: data ?? [], loading, error, refetch };
}

export interface DealRoomShareDialogData {
  links: Link[];
  selectedLink: Link | null;
  rules: AccessRule[];
  invitations: LinkInvitation[];
  logs: AccessLog[];
}

export function useDealRoomShareDialogData(roomId: string, linkId: string | undefined, open: boolean) {
  const { links, loading: linksLoading, error: linksError, refetch: refetchLinks } = useDealRoomLinks(roomId, open);

  const selectedLink =
    links.find((l) => (linkId ? l.id === linkId : l.status === "active" || l.isActive)) || null;

  const selectedLinkId = selectedLink?.id;
  const { rules, loading: rulesLoading, error: rulesError, refetch: refetchRules } =
    useLinkAccessRules(selectedLinkId, open);
  const { invitations, loading: invitationsLoading, error: invitationsError, refetch: refetchInvitations } =
    useLinkInvitations(selectedLinkId, open);
  const { logs, loading: logsLoading, error: logsError, refetch: refetchLogs } =
    useAccessLogs(selectedLinkId, open);

  const refetch = useCallback(async () => {
    await Promise.all([refetchLinks(), refetchRules(), refetchInvitations(), refetchLogs()]);
  }, [refetchLinks, refetchRules, refetchInvitations, refetchLogs]);

  return {
    data: { links, selectedLink, rules, invitations, logs },
    loading: linksLoading || rulesLoading || invitationsLoading || logsLoading,
    error: linksError || rulesError || invitationsError || logsError,
    refetch,
  };
}

export interface LinkShareDialogData {
  link: Link | null;
  rules: AccessRule[];
  invitations: LinkInvitation[];
  logs: AccessLog[];
}

export function useLinkShareDialogData(linkId: string | undefined, open: boolean) {
  const { data: link, loading: linkLoading, error: linkError, refetch: refetchLink } = useAsyncData(
    () => (open && linkId ? api.getLinkById(linkId) : Promise.resolve(null)),
    [linkId, open]
  );

  const { rules, loading: rulesLoading, error: rulesError, refetch: refetchRules } =
    useLinkAccessRules(link?.id, open);
  const { invitations, loading: invitationsLoading, error: invitationsError, refetch: refetchInvitations } =
    useLinkInvitations(link?.id, open);
  const { logs, loading: logsLoading, error: logsError, refetch: refetchLogs } =
    useAccessLogs(link?.id, open);

  const refetch = useCallback(async () => {
    await Promise.all([refetchLink(), refetchRules(), refetchInvitations(), refetchLogs()]);
  }, [refetchLink, refetchRules, refetchInvitations, refetchLogs]);

  return {
    data: { link, rules, invitations, logs },
    loading: linkLoading || rulesLoading || invitationsLoading || logsLoading,
    error: linkError || rulesError || invitationsError || logsError,
    refetch,
  };
}
