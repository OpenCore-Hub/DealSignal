import { useCallback, useEffect, useRef, useState } from "react";

export interface AsyncDataState<T> {
  data: T | null;
  loading: boolean;
  error: string | null;
  refetch: () => Promise<void>;
}

/**
 * 统一异步数据加载 hook。
 * 自动管理 loading / error / data / cancel / refetch，替代组件内重复的 useEffect fetch 样板。
 */
export function useAsyncData<T>(
  fetcher: () => Promise<T>,
  deps: React.DependencyList = []
): AsyncDataState<T> {
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [tick, setTick] = useState(0);
  const cancelledRef = useRef(false);
  const loadPromiseRef = useRef<Promise<void> | null>(null);

  const refetch = useCallback(() => {
    setTick((t) => t + 1);
    return loadPromiseRef.current ?? Promise.resolve();
  }, []);

  useEffect(() => {
    cancelledRef.current = false;

    async function load() {
      setLoading(true);
      setError(null);
      try {
        const result = await fetcher();
        if (!cancelledRef.current) {
          setData(result);
        }
      } catch (e) {
        if (!cancelledRef.current) {
          setError(e instanceof Error ? e.message : "Failed to load");
        }
      } finally {
        if (!cancelledRef.current) {
          setLoading(false);
        }
      }
    }

    const promise = load();
    loadPromiseRef.current = promise;

    return () => {
      cancelledRef.current = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tick, ...deps]);

  return { data, loading, error, refetch };
}
