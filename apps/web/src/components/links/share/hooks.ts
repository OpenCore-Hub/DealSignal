import { useAsyncData } from "@/hooks/useAsyncData";
import { api } from "@/lib/api";
import type { AccessLog } from "@/types";

export function useAccessLogs(linkId: string | undefined, enabled = true): {
  logs: AccessLog[];
  loading: boolean;
  error: string | null;
  refetch: () => Promise<void>;
} {
  const { data, loading, error, refetch } = useAsyncData(
    () => (enabled && linkId ? api.getAccessLogs(linkId).then((res) => res.data) : Promise.resolve([])),
    [linkId, enabled]
  );
  return { logs: data ?? [], loading, error, refetch };
}
