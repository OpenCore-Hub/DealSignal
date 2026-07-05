import { useCallback, useEffect, useRef, useState } from "react";

export interface AsyncDataState<T> {
  data: T | null;
  loading: boolean;
  error: string | null;
  refetch: () => Promise<void>;
}

function isAbortError(e: unknown): boolean {
  return e instanceof Error && e.name === "AbortError";
}

/**
 * 统一异步数据加载 hook。
 * 自动管理 loading / error / data / cancel / refetch，替代组件内重复的 useEffect fetch 样板。
 * fetcher 可接收 AbortSignal，在依赖变化或组件卸载时自动 abort，避免幽灵请求和竞态。
 */
export function useAsyncData<T>(
  fetcher: (signal?: AbortSignal) => Promise<T>,
  deps: React.DependencyList = []
): AsyncDataState<T> {
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [tick, setTick] = useState(0);
  const cancelledRef = useRef(false);
  const pendingResolvesRef = useRef<((value: void | PromiseLike<void>) => void)[]>([]);

  const refetch = useCallback(() => {
    setTick((t) => t + 1);
    return new Promise<void>((resolve) => {
      pendingResolvesRef.current.push(resolve);
    });
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    cancelledRef.current = false;

    async function load() {
      setLoading(true);
      setError(null);
      try {
        const result = await fetcher(controller.signal);
        if (!cancelledRef.current && !controller.signal.aborted) {
          setData(result);
        }
      } catch (e) {
        if (cancelledRef.current || controller.signal.aborted || isAbortError(e)) {
          return;
        }
        setError(e instanceof Error ? e.message : "Failed to load");
      } finally {
        if (!cancelledRef.current && !controller.signal.aborted) {
          setLoading(false);
        }
        const resolves = pendingResolvesRef.current;
        pendingResolvesRef.current = [];
        for (const resolve of resolves) {
          resolve();
        }
      }
    }

    load();

    return () => {
      cancelledRef.current = true;
      controller.abort();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tick, ...deps]);

  return { data, loading, error, refetch };
}
