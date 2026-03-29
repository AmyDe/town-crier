import { useState, useEffect, useCallback, type DependencyList } from 'react';

interface FetchDataState<T> {
  data: T | null;
  isLoading: boolean;
  error: string | null;
}

interface FetchDataResult<T> extends FetchDataState<T> {
  refresh: () => void;
}

export function useFetchData<T>(
  fetcher: () => Promise<T>,
  deps: DependencyList,
): FetchDataResult<T> {
  const [state, setState] = useState<FetchDataState<T>>({
    data: null,
    isLoading: true,
    error: null,
  });

  const load = useCallback(async () => {
    setState(prev => ({ ...prev, isLoading: true, error: null }));
    try {
      const data = await fetcher();
      setState({ data, isLoading: false, error: null });
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'An error occurred';
      setState(prev => ({ ...prev, isLoading: false, error: message }));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const data = await fetcher();
        if (!cancelled) {
          setState({ data, isLoading: false, error: null });
        }
      } catch (err: unknown) {
        if (!cancelled) {
          const message = err instanceof Error ? err.message : 'An error occurred';
          setState(prev => ({ ...prev, isLoading: false, error: message }));
        }
      }
    })();
    return () => { cancelled = true; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);

  return { ...state, refresh: load };
}
