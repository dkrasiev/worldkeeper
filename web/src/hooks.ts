import { useCallback, useEffect, useRef, useState } from "react";

export interface Loadable<T> {
  data?: T;
  error?: Error;
  loading: boolean;
  reload: () => void;
}

// useLoad fetches on mount and every `pollMs` (if set). Polling keeps
// the "in game" status live without websockets.
export function useLoad<T>(fn: () => Promise<T>, deps: unknown[], pollMs?: number): Loadable<T> {
  const [state, setState] = useState<{ data?: T; error?: Error; loading: boolean }>({ loading: true });
  const fnRef = useRef(fn);
  fnRef.current = fn;

  const reload = useCallback(() => {
    fnRef.current().then(
      (data) => setState({ data, loading: false }),
      (error: Error) => setState((s) => ({ ...s, error, loading: false })),
    );
  }, []);

  useEffect(() => {
    setState({ loading: true });
    reload();
    if (!pollMs) return;
    const t = setInterval(reload, pollMs);
    return () => clearInterval(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);

  return { ...state, reload };
}

export function useHashRoute(): [string[], (path: string) => void] {
  const parse = () =>
    location.hash
      .replace(/^#\/?/, "")
      .split("/")
      .filter(Boolean)
      .map(decodeURIComponent);
  const [parts, setParts] = useState(parse);
  useEffect(() => {
    const on = () => setParts(parse());
    addEventListener("hashchange", on);
    return () => removeEventListener("hashchange", on);
  }, []);
  const go = (path: string) => {
    location.hash = path;
  };
  return [parts, go];
}
