import { useCallback, useEffect, useRef, useState } from "react";
import { api, type PublicLinkCredentials } from "@/lib/api";

const MAX_CONCURRENT_THUMBNAIL_FETCHES = 4;

function orderPagesByProximity(pageNumbers: number[], currentPage: number): number[] {
  return [...pageNumbers].sort(
    (a, b) => Math.abs(a - currentPage) - Math.abs(b - currentPage),
  );
}

export function usePublicPageThumbnailUrls(args: {
  documentId?: string;
  publicToken?: string;
  pageNumbers: number[];
  currentPage: number;
  credentials?: PublicLinkCredentials;
  seedUrls?: Record<number, string>;
}): Record<number, string> {
  const { documentId, publicToken, pageNumbers, currentPage, credentials, seedUrls } = args;
  const [urls, setUrls] = useState<Record<number, string>>(() => ({ ...seedUrls }));
  const cacheRef = useRef<Record<number, string>>({ ...seedUrls });
  const inflightRef = useRef(new Map<number, Promise<void>>());
  const credentialsRef = useRef(credentials);
  const scopeRef = useRef(0);

  useEffect(() => {
    credentialsRef.current = credentials;
  }, [credentials]);

  useEffect(() => {
    if (!seedUrls) return;
    let changed = false;
    for (const [page, url] of Object.entries(seedUrls)) {
      const pageNum = Number(page);
      if (!url || cacheRef.current[pageNum] === url) continue;
      cacheRef.current[pageNum] = url;
      changed = true;
    }
    if (changed) setUrls({ ...cacheRef.current });
  }, [seedUrls]);

  useEffect(() => {
    scopeRef.current += 1;
    inflightRef.current.clear();
    const next = { ...(seedUrls ?? {}) };
    cacheRef.current = next;
    setUrls(next);
  }, [documentId, publicToken]);

  const fetchOne = useCallback(
    async (pageNumber: number): Promise<void> => {
      if (!documentId || !publicToken) return;
      if (cacheRef.current[pageNumber]) return;

      const existing = inflightRef.current.get(pageNumber);
      if (existing) {
        await existing;
        return;
      }

      const scope = scopeRef.current;
      const task = (async () => {
        try {
          const res = await api.getPublicPageSignedUrl(
            documentId,
            publicToken,
            pageNumber,
            credentialsRef.current,
          );
          if (scope !== scopeRef.current) return;
          if (!res.image_url) return;
          cacheRef.current[pageNumber] = res.image_url;
          setUrls({ ...cacheRef.current });
        } catch {
          // Best-effort thumbnails; a later effect pass can retry uncached pages.
        } finally {
          inflightRef.current.delete(pageNumber);
        }
      })();

      inflightRef.current.set(pageNumber, task);
      await task;
    },
    [documentId, publicToken],
  );

  useEffect(() => {
    if (!documentId || !publicToken || pageNumbers.length === 0) return;

    const missing = pageNumbers.filter((pageNumber) => !cacheRef.current[pageNumber]);
    if (missing.length === 0) return;

    const ordered = orderPagesByProximity(missing, currentPage);
    let cancelled = false;

    void (async () => {
      const queue = [...ordered];
      const workers = Array.from({ length: MAX_CONCURRENT_THUMBNAIL_FETCHES }, async () => {
        while (!cancelled) {
          const pageNumber = queue.shift();
          if (pageNumber === undefined) return;
          await fetchOne(pageNumber);
        }
      });
      await Promise.all(workers);

      if (cancelled) return;

      const stillMissing = pageNumbers.filter((pageNumber) => !cacheRef.current[pageNumber]);
      if (stillMissing.length === 0) return;

      const retryQueue = orderPagesByProximity(stillMissing, currentPage);
      await Promise.all(retryQueue.map((pageNumber) => fetchOne(pageNumber)));
    })();

    return () => {
      cancelled = true;
    };
  }, [documentId, publicToken, currentPage, pageNumbers.join(","), fetchOne]);

  return urls;
}
