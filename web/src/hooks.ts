import { useCallback, useEffect, useRef, useState } from "react";

export interface Loadable<T> {
  data?: T;
  error?: Error;
  loading: boolean;
  reload: () => void;
}

// useLoad fetches on mount and every `pollMs` (if set). Polling keeps
// the "in game" status live without websockets. When deps change (e.g. the
// chosen player) the old data stays until the new arrives, so the page does
// not blank; answers to requests made before the change are dropped.
export function useLoad<T>(fn: () => Promise<T>, deps: unknown[], pollMs?: number): Loadable<T> {
  const [state, setState] = useState<{ data?: T; error?: Error; loading: boolean }>({ loading: true });
  const fnRef = useRef(fn);
  fnRef.current = fn;
  const gen = useRef(0);

  const reload = useCallback(() => {
    const g = gen.current;
    fnRef.current().then(
      (data) => g === gen.current && setState({ data, loading: false }),
      (error: Error) => g === gen.current && setState((s) => ({ ...s, error, loading: false })),
    );
  }, []);

  useEffect(() => {
    gen.current++;
    setState((s) => ({ ...s, loading: true }));
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
