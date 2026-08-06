import { useEffect, useState } from "react";
import { useAsyncData } from "@/hooks/useAsyncData";
import { api } from "@/lib/api";
import type { AccessLog } from "@/types";
import {
  loadNdaPickerSources,
  type NdaDocumentOption,
  type NdaTemplateOption,
} from "./ndaPicker";

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

/** Shared NDA picker sources for document-library share and deal-room access. */
export function useNdaPickerSources(): {
  ndaTemplates: NdaTemplateOption[];
  agreementDocs: NdaDocumentOption[];
  loading: boolean;
} {
  const [ndaTemplates, setNdaTemplates] = useState<NdaTemplateOption[]>([]);
  const [agreementDocs, setAgreementDocs] = useState<NdaDocumentOption[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    void (async () => {
      const sources = await loadNdaPickerSources();
      if (cancelled) return;
      setNdaTemplates(sources.templates);
      setAgreementDocs(sources.agreementDocs);
      setLoading(false);
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  return { ndaTemplates, agreementDocs, loading };
}
